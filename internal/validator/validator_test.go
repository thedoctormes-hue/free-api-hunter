package validator

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"free-api-hunter/internal/verifier"
)

// probeCapture фиксирует, как именно на пробу пришёл запрос (KRV-E2).
type probeCapture struct {
	method      string
	auth        string // Authorization header
	headerKey   string // непустое имя кастомного заголовка (header-адаптер)
	headerValue string
	queryKey    string // значение apikey query-параметра (query-адаптер)
}

// newProbeServer поднимает httptest-сервер, возвращающий 200 + список моделей,
// и фиксирует форму аутентификации. Также отключает SSRF-гейт в тестах
// (переопределяет verifier.ValidateOutboundURL) и подменяет HTTPClient.
// Используются ТОЛЬКО фейковые секреты и loopback — живых сетевых проб нет.
func newProbeServer(t *testing.T) (string, *probeCapture) {
	t.Helper()
	cap := &probeCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.method = r.Method
		cap.auth = r.Header.Get("Authorization")
		for _, h := range []string{"xi-api-key", "x-manus-api-key"} {
			if v := r.Header.Get(h); v != "" {
				cap.headerKey = h
				cap.headerValue = v
			}
		}
		cap.queryKey = r.URL.Query().Get("apikey")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"m1"},{"id":"m2"}]}`))
	}))
	t.Cleanup(srv.Close)

	origClient := verifier.HTTPClient
	origValidate := verifier.ValidateOutboundURL
	verifier.HTTPClient = srv.Client()
	verifier.ValidateOutboundURL = func(rawURL string) (*url.URL, error) {
		return url.Parse(rawURL)
	}
	t.Cleanup(func() {
		verifier.HTTPClient = origClient
		verifier.ValidateOutboundURL = origValidate
	})

	return srv.URL, cap
}

// TestProbeAdapterBearer — KRV-E2: bearer => "Authorization: Bearer <secret>" + GET.
func TestProbeAdapterBearer(t *testing.T) {
	srv, cap := newProbeServer(t)
	spec := verifier.ProbeSpec{AuthType: "bearer", Method: "GET", URL: srv + "/models"}
	res := verifier.VerifyAPIKeyWithSecretSpec("openrouter", spec, "FAKE-SECRET")
	if !res.IsActive {
		t.Fatalf("expected active, got error=%q status=%d", res.Error, res.StatusCode)
	}
	if cap.auth != "Bearer FAKE-SECRET" {
		t.Fatalf("bearer auth wrong: got %q", cap.auth)
	}
	if len(res.Models) != 2 || res.Models[0] != "m1" {
		t.Fatalf("models not parsed: %v", res.Models)
	}
}

// TestProbeAdapterQuery — KRV-E2: query => GET/POST с ?apikey=<secret>.
func TestProbeAdapterQuery(t *testing.T) {
	srv, cap := newProbeServer(t)
	spec := verifier.ProbeSpec{AuthType: "query", Method: "GET", URL: srv, QueryParam: "apikey"}
	res := verifier.VerifyAPIKeyWithSecretSpec("ocr-space", spec, "FAKE-QUERY-SECRET")
	if !res.IsActive {
		t.Fatalf("expected active, got error=%q status=%d", res.Error, res.StatusCode)
	}
	if cap.queryKey != "FAKE-QUERY-SECRET" {
		t.Fatalf("query param wrong: got %q", cap.queryKey)
	}
	if cap.auth != "" {
		t.Fatalf("query adapter must NOT set Authorization header, got %q", cap.auth)
	}
}

// TestProbeAdapterHeader — KRV-E2: header => кастомный заголовок (xi-api-key).
func TestProbeAdapterHeader(t *testing.T) {
	srv, cap := newProbeServer(t)
	spec := verifier.ProbeSpec{AuthType: "header", Method: "GET", URL: srv, AuthHeader: "xi-api-key"}
	res := verifier.VerifyAPIKeyWithSecretSpec("elevenlabs", spec, "FAKE-HEADER-SECRET")
	if !res.IsActive {
		t.Fatalf("expected active, got error=%q status=%d", res.Error, res.StatusCode)
	}
	if cap.headerKey != "xi-api-key" || cap.headerValue != "FAKE-HEADER-SECRET" {
		t.Fatalf("header adapter wrong: key=%q value=%q", cap.headerKey, cap.headerValue)
	}
}

// TestProbeAdapterHeaderManus — KRV-E2: header => x-manus-api-key.
func TestProbeAdapterHeaderManus(t *testing.T) {
	srv, cap := newProbeServer(t)
	spec := verifier.ProbeSpec{AuthType: "header", Method: "GET", URL: srv, AuthHeader: "x-manus-api-key"}
	res := verifier.VerifyAPIKeyWithSecretSpec("manus", spec, "FAKE-MANUS-SECRET")
	if !res.IsActive {
		t.Fatalf("expected active, got error=%q status=%d", res.Error, res.StatusCode)
	}
	if cap.headerKey != "x-manus-api-key" || cap.headerValue != "FAKE-MANUS-SECRET" {
		t.Fatalf("manus header adapter wrong: key=%q value=%q", cap.headerKey, cap.headerValue)
	}
}

// TestLiveStatusFromResult — маппинг статус-кодов в live_status.
func TestLiveStatusFromResult(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{200, "valid"},
		{401, "expired"},
		{403, "expired"},
		{429, "rate_limited"},
		{500, "unknown"},
		{0, "unknown"},
	}
	for _, c := range cases {
		got := liveStatusFromResult(&verifier.KeyVerifyResult{StatusCode: c.code})
		if got != c.want {
			t.Errorf("status %d -> got %q, want %q", c.code, got, c.want)
		}
	}
}

// TestSplitKeys — KRV-E3: каждая непустая строка = ключ; пустые игнорируются.
func TestSplitKeys(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"k1\nk2\nk3\n", []string{"k1", "k2", "k3"}},
		{"k1\n\nk2\n", []string{"k1", "k2"}},
		{"  k1  \n\tk2\t\n", []string{"k1", "k2"}},
		{"\r\nk1\r\nk2\r\n", []string{"k1", "k2"}},
		{"\n\n\n", nil},
		{"", nil},
	}
	for i, c := range cases {
		got := splitKeys(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("case %d: got %v, want %v", i, got, c.want)
		}
		for j := range c.want {
			if got[j] != c.want[j] {
				t.Fatalf("case %d idx %d: got %q want %q", i, j, got[j], c.want[j])
			}
		}
	}
}

// writeVaultFile создаёт провайдера и файл ключа во временном vault.
func writeVaultFile(t *testing.T, provider, file, content string) {
	t.Helper()
	dir := filepath.Join(vaultBase, provider)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestValidateKeySingleKeyFile — один ключ => key_id "provider/file", valid при пробе.
func TestValidateKeySingleKeyFile(t *testing.T) {
	vaultBase = t.TempDir()
	srv, _ := newProbeServer(t)
	writeVaultFile(t, "openrouter", "api.key", "sk-fake-single\n")

	cfg := EndpointConfig{BaseURL: srv, AuthType: "bearer", ModelsPath: "/models", Verified: true}
	recs, err := ValidateKey("openrouter", "api.key", cfg, true, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].KeyID != "openrouter/api.key" {
		t.Fatalf("key_id = %q, want openrouter/api.key", recs[0].KeyID)
	}
	if recs[0].LiveStatus != "valid" {
		t.Fatalf("live_status = %q, want valid", recs[0].LiveStatus)
	}
	if len(recs[0].Models) != 2 {
		t.Fatalf("models = %v", recs[0].Models)
	}
}

// TestValidateKeyMultiKeyFile — KRV-E3: 5 ключей в файле => key_id "provider/file#N".
func TestValidateKeyMultiKeyFile(t *testing.T) {
	vaultBase = t.TempDir()
	srv, _ := newProbeServer(t)
	writeVaultFile(t, "cerebras", "api.keys", "c1\nc2\nc3\nc4\nc5\n")

	cfg := EndpointConfig{BaseURL: srv, AuthType: "bearer", ModelsPath: "/models", Verified: true}
	recs, err := ValidateKey("cerebras", "api.keys", cfg, true, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if len(recs) != 5 {
		t.Fatalf("expected 5 records (multi-key file), got %d", len(recs))
	}
	for i, r := range recs {
		want := fmt.Sprintf("cerebras/api.keys#%d", i+1)
		if r.KeyID != want {
			t.Fatalf("record %d key_id = %q, want %q", i, r.KeyID, want)
		}
		if r.LiveStatus != "valid" {
			t.Fatalf("record %d live_status = %q, want valid", i, r.LiveStatus)
		}
	}
}

// TestValidateKeyDryRun — сухой прогон: сети нет, live_status="unknown".
func TestValidateKeyDryRun(t *testing.T) {
	vaultBase = t.TempDir()
	// Намеренно НЕ поднимаем сервер: в dry-run сети быть не должно.
	writeVaultFile(t, "openrouter", "api.key", "sk-fake\n")

	cfg := EndpointConfig{BaseURL: "http://127.0.0.1:9", AuthType: "bearer", ModelsPath: "/models", Verified: true}
	recs, err := ValidateKey("openrouter", "api.key", cfg, false, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].LiveStatus != "unknown" {
		t.Fatalf("dry-run live_status = %q, want unknown", recs[0].LiveStatus)
	}
	if !strings.Contains(recs[0].Instructions, "dry-run") {
		t.Fatalf("instructions missing dry-run marker: %q", recs[0].Instructions)
	}
}

// TestValidateKeyUnknownEndpoint — PAT-005: unknown endpoint => не пробим, unknown.
func TestValidateKeyUnknownEndpoint(t *testing.T) {
	vaultBase = t.TempDir()
	// Даже если бы сервер был — не должен дёрнуться (endpoint unknown).
	writeVaultFile(t, "manus", "api.key", "mk-fake\n")

	cfg := EndpointConfig{BaseURL: "", AuthType: "unknown", Verified: false}
	recs, err := ValidateKey("manus", "api.key", cfg, true, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].LiveStatus != "unknown" {
		t.Fatalf("live_status = %q, want unknown", recs[0].LiveStatus)
	}
	if !strings.Contains(recs[0].Instructions, "not probed") {
		t.Fatalf("instructions missing not-probed marker: %q", recs[0].Instructions)
	}
}

// TestValidateKeyQueryAdapter — KRV-E2 на уровне validator: query-провайдер
// строит probe с ?apikey= и не ставит Bearer.
func TestValidateKeyQueryAdapter(t *testing.T) {
	vaultBase = t.TempDir()
	srv, cap := newProbeServer(t)
	writeVaultFile(t, "ocr-space", "api.key", "ok-fake\n")

	cfg := EndpointConfig{BaseURL: srv, AuthType: "query", ModelsPath: "", Method: "POST", Verified: false}
	recs, err := ValidateKey("ocr-space", "api.key", cfg, true, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	// ocr-space не имеет /models; адаптер всё равно строит запрос с
	// ?apikey= и не ставит Bearer. (Тестовый сервер отвечает 200, так что
	// live_status здесь valid — проверяем именно ФОРМУ адаптера.)
	if cap.queryKey != "ok-fake" {
		t.Fatalf("query adapter not applied: apikey=%q", cap.queryKey)
	}
	if cap.auth != "" {
		t.Fatalf("query adapter must not set Bearer, got %q", cap.auth)
	}
}
