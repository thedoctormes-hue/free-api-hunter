package verifier

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/vault"
)

// setupSpecificKeyVault создаёт vault с ДВУМЯ ключами одного провайдера.
// GetDefaultKey вернёт ПЕРВЫЙ (co-a), а мы хотим валидировать ВТОРОЙ (co-b),
// чтобы доказать: проба идёт с конкретным секретом, а не с дефолтным.
func setupSpecificKeyVault(t *testing.T) (provider, coA, coB string) {
	t.Helper()
	tmp := t.TempDir()
	orig := vault.VaultPath
	vault.VaultPath = tmp
	t.Cleanup(func() { vault.VaultPath = orig })

	provider = "cohere"
	dir := filepath.Join(tmp, provider)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	coA = "co-SECRET-A" // вернётся из GetDefaultKey (первый файл)
	coB = "co-SECRET-B" // конкретный файл, который хотим проверить
	if err := os.WriteFile(filepath.Join(dir, "co-a.key"), []byte(coA), 0600); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "co-b.key"), []byte(coB), 0600); err != nil {
		t.Fatalf("write b: %v", err)
	}
	return provider, coA, coB
}

// newProbeServer возвращает httptest-сервер, который отвечает 200 на /models
// и записывает переданный Bearer-токен. Также отключает SSRF-гейт в тестах.
func newProbeServer(t *testing.T) (string, *string) {
	t.Helper()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"test-model-1"}]}`))
	}))
	t.Cleanup(srv.Close)

	// Отключаем HTTPS/loopback-проверку ТОЛЬКО в тесте.
	origValidate := ValidateOutboundURL
	ValidateOutboundURL = func(rawURL string) (*url.URL, error) {
		return url.Parse(rawURL)
	}
	t.Cleanup(func() { ValidateOutboundURL = origValidate })

	return srv.URL, &gotAuth
}

func TestVerifyAPIKeyUsesSpecificVaultFile_NotDefault(t *testing.T) {
	provider, coA, coB := setupSpecificKeyVault(t)
	// Дефолтный ключ (для контраста) — первый файл.
	def, err := vault.GetDefaultKey(provider)
	if err != nil {
		t.Fatalf("GetDefaultKey: %v", err)
	}
	if def != coA {
		t.Fatalf("precondition: default key should be co-a, got %q", def)
	}

	endpoint, gotAuth := newProbeServer(t)
	specificPath := filepath.Join(vault.VaultPath, provider, "co-b.key")

	key := &models.APIKey{
		ProviderName: provider,
		KeyLocation:  specificPath, // абсолютный путь к КОНКРЕТНОМУ файлу
		Endpoint:     endpoint,
	}

	result := VerifyAPIKey(key)

	if !result.IsActive {
		t.Fatalf("expected active key, got error=%q status=%d", result.Error, result.StatusCode)
	}
	if *gotAuth != "Bearer "+coB {
		t.Fatalf("probe used WRONG secret: got %q, want %q (specific co-b)", *gotAuth, "Bearer "+coB)
	}
	if *gotAuth == "Bearer "+coA {
		t.Fatalf("BUG reproduced: probe used the DEFAULT (co-a) secret instead of specific co-b")
	}
	if len(result.Models) == 0 || result.Models[0] != "test-model-1" {
		t.Fatalf("expected models parsed, got %v", result.Models)
	}
	// Ключ должен быть помечен активным.
	if !key.IsActive {
		t.Fatalf("expected key.IsActive true after verify")
	}
}

func TestVerifyAPIKeyWithSecret_UsesGivenSecret(t *testing.T) {
	// Даже если в vault есть дефолтный ключ, WithSecret берёт переданный секрет.
	provider, _, coB := setupSpecificKeyVault(t)
	endpoint, gotAuth := newProbeServer(t)

	result := VerifyAPIKeyWithSecret(provider, endpoint, coB)
	if !result.IsActive {
		t.Fatalf("expected active, got error=%q", result.Error)
	}
	if *gotAuth != "Bearer "+coB {
		t.Fatalf("WithSecret used wrong secret: got %q want %q", *gotAuth, "Bearer "+coB)
	}
	if result.Provider != provider {
		t.Fatalf("expected Provider=%q, got %q", provider, result.Provider)
	}
}

func TestResolveKey_ConcretePathIgnoresDefault(t *testing.T) {
	provider, coA, coB := setupSpecificKeyVault(t)
	specificPath := filepath.Join(vault.VaultPath, provider, "co-b.key")

	got, err := resolveKey(&models.APIKey{ProviderName: provider, KeyLocation: specificPath})
	if err != nil {
		t.Fatalf("resolveKey error: %v", err)
	}
	if got != coB {
		t.Fatalf("resolveKey returned %q, want specific co-b %q (not default co-a)", got, coB)
	}
	if got == coA {
		t.Fatalf("resolveKey fell back to default co-a")
	}
}

func TestVaultGetKeyByPath(t *testing.T) {
	provider, _, coB := setupSpecificKeyVault(t)
	specificPath := filepath.Join(vault.VaultPath, provider, "co-b.key")

	got, err := vault.GetKeyByPath(specificPath)
	if err != nil {
		t.Fatalf("GetKeyByPath: %v", err)
	}
	if got != coB {
		t.Fatalf("GetKeyByPath returned %q, want co-b", got)
	}

	// Вне vault — должно быть отклонено (path traversal protection).
	if _, err := vault.GetKeyByPath("/etc/passwd"); err == nil {
		t.Fatalf("GetKeyByPath should reject paths outside vault")
	}
	// Относительный путь — отклонён.
	if _, err := vault.GetKeyByPath("co-b.key"); err == nil {
		t.Fatalf("GetKeyByPath should reject relative paths")
	}
}

// TestVerifyAPIKeySkPrefixStillUsedAsSecret — регрессия: sk- префикс используется
// как сам секрет, vault не трогается.
func TestVerifyAPIKeySkPrefixStillUsedAsSecret(t *testing.T) {
	provider, _, _ := setupSpecificKeyVault(t)
	skSecret := "sk-abc123"
	endpoint, gotAuth := newProbeServer(t)

	key := &models.APIKey{ProviderName: provider, KeyLocation: skSecret, Endpoint: endpoint}
	result := VerifyAPIKey(key)
	if !result.IsActive {
		t.Fatalf("expected active, got %q", result.Error)
	}
	if !strings.Contains(*gotAuth, skSecret) {
		t.Fatalf("sk- secret not used: %q", *gotAuth)
	}
}
