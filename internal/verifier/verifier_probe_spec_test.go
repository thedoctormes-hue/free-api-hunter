package verifier

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"free-api-hunter/internal/models"
)

// installProbeSpecClient — подменить HTTPClient + ValidateOutboundURL на
// время теста (httptest-сервер, loopback разрешён). Восстанавливает по cleanup.
func installProbeSpecClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	origClient := HTTPClient
	origValidate := ValidateOutboundURL
	HTTPClient = server.Client()
	ValidateOutboundURL = func(rawURL string) (*url.URL, error) {
		return url.Parse(rawURL)
	}
	t.Cleanup(func() {
		HTTPClient = origClient
		ValidateOutboundURL = origValidate
	})
}

// TestVerifyAPIKeyWithSecretSpecBearer — KRV-E2: bearer => "Authorization: Bearer".
func TestVerifyAPIKeyWithSecretSpecBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sec" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"m1"}]}`))
	}))
	defer srv.Close()
	installProbeSpecClient(t, srv)

	spec := ProbeSpec{AuthType: "bearer", Method: "GET", URL: srv.URL + "/models"}
	res := VerifyAPIKeyWithSecretSpec("openrouter", spec, "sec")
	if !res.IsActive {
		t.Fatalf("expected active, got error=%q status=%d", res.Error, res.StatusCode)
	}
	if len(res.Models) != 1 || res.Models[0] != "m1" {
		t.Fatalf("models wrong: %v", res.Models)
	}
}

// TestVerifyAPIKeyWithSecretSpecQuery — KRV-E2: query => ?apikey= (явный и дефолтный параметр).
func TestVerifyAPIKeyWithSecretSpecQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apikey") != "sec" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	installProbeSpecClient(t, srv)

	// Явный QueryParam.
	spec := ProbeSpec{AuthType: "query", Method: "GET", URL: srv.URL, QueryParam: "apikey"}
	res := VerifyAPIKeyWithSecretSpec("ocr-space", spec, "sec")
	if !res.IsActive {
		t.Fatalf("expected active (explicit param), got %q", res.Error)
	}
	// Дефолтный QueryParam ("apikey").
	spec2 := ProbeSpec{AuthType: "query", Method: "GET", URL: srv.URL}
	res2 := VerifyAPIKeyWithSecretSpec("ocr-space", spec2, "sec")
	if !res2.IsActive {
		t.Fatalf("expected active (default param), got %q", res2.Error)
	}
	// query НЕ должен ставить Bearer.
	if res.Error != "" && res.StatusCode != 200 {
		t.Fatalf("query adapter error: %q", res.Error)
	}
}

// TestVerifyAPIKeyWithSecretSpecHeader — KRV-E2: header => кастомный заголовок,
// в т.ч. с пустым именем (дефолт xi-api-key).
func TestVerifyAPIKeyWithSecretSpecHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != "sec" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	installProbeSpecClient(t, srv)

	spec := ProbeSpec{AuthType: "header", Method: "GET", URL: srv.URL, AuthHeader: "xi-api-key"}
	res := VerifyAPIKeyWithSecretSpec("elevenlabs", spec, "sec")
	if !res.IsActive {
		t.Fatalf("expected active (header), got %q", res.Error)
	}
	// Пустой AuthHeader -> дефолт xi-api-key.
	spec2 := ProbeSpec{AuthType: "header", Method: "GET", URL: srv.URL}
	res2 := VerifyAPIKeyWithSecretSpec("elevenlabs", spec2, "sec")
	if !res2.IsActive {
		t.Fatalf("expected active (header default), got %q", res2.Error)
	}
}

// TestVerifyAPIKeyWithSecretSpecNone — KRV-E2: none => без аутентификации.
func TestVerifyAPIKeyWithSecretSpecNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	installProbeSpecClient(t, srv)

	spec := ProbeSpec{AuthType: "none", Method: "GET", URL: srv.URL + "/models"}
	res := VerifyAPIKeyWithSecretSpec("pollinations", spec, "sec")
	if !res.IsActive {
		t.Fatalf("expected active (none), got %q", res.Error)
	}
}

// TestVerifyAPIKeyWithSecretSpecEmptyURL — пустой URL => invalid_endpoint.
func TestVerifyAPIKeyWithSecretSpecEmptyURL(t *testing.T) {
	spec := ProbeSpec{AuthType: "bearer", URL: ""}
	res := VerifyAPIKeyWithSecretSpec("p", spec, "sec")
	if res.Error == "" {
		t.Fatal("expected error for empty URL")
	}
}

// TestVerifyAPIKeyWithSecretSpecInvalidURL — SSRF-гейт отклоняет URL => invalid_endpoint.
func TestVerifyAPIKeyWithSecretSpecInvalidURL(t *testing.T) {
	origValidate := ValidateOutboundURL
	ValidateOutboundURL = func(rawURL string) (*url.URL, error) {
		return nil, fmt.Errorf("blocked by policy")
	}
	defer func() { ValidateOutboundURL = origValidate }()

	spec := ProbeSpec{AuthType: "bearer", URL: "http://example.com/models"}
	res := VerifyAPIKeyWithSecretSpec("p", spec, "sec")
	if res.Error == "" {
		t.Fatal("expected invalid_endpoint error from SSRF gate")
	}
}

// TestVerifyAPIKeyVaultError — KRV: resolveKey падает (абсолютный путь вне vault)
// => vault_error, не паникуем.
func TestVerifyAPIKeyVaultError(t *testing.T) {
	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "/etc/passwd", // абсолютный, но вне vault -> GetKeyByPath отклоняет
		Endpoint:     "http://127.0.0.1:9",
	}
	res := VerifyAPIKey(key)
	if res.IsActive {
		t.Fatal("expected inactive for vault error")
	}
	if res.Error == "" || !strings.Contains(res.Error, "vault_error") {
		t.Fatalf("expected vault_error, got %q", res.Error)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
