package verifier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"free-api-hunter/internal/models"
)

func TestVerifyAPIKey_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-test-valid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[{"id":"model-1","context_length":8192}]}`))
	}))
	defer server.Close()

	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-test-valid-key",
		Endpoint:     server.URL,
	}

	result := VerifyAPIKey(key)
	if !result.IsActive {
		t.Fatalf("expected IsActive=true, got false (error: %s)", result.Error)
	}
	if result.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", result.StatusCode)
	}
	if len(result.Models) != 1 || result.Models[0] != "model-1" {
		t.Fatalf("expected [model-1], got %v", result.Models)
	}
}

func TestVerifyAPIKey_InvalidKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-invalid-key",
		Endpoint:     server.URL,
	}

	result := VerifyAPIKey(key)
	if result.IsActive {
		t.Error("expected IsActive=false for 401")
	}
	if result.Error != "invalid_key" {
		t.Fatalf("expected error=invalid_key, got %s", result.Error)
	}
}

func TestVerifyAPIKey_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-rate-limited",
		Endpoint:     server.URL,
	}

	result := VerifyAPIKey(key)
	if result.IsActive {
		t.Error("expected IsActive=false for 429")
	}
	if result.Error != "rate_limited" {
		t.Fatalf("expected error=rate_limited, got %s", result.Error)
	}
}

func TestVerifyAPIKey_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-forbidden",
		Endpoint:     server.URL,
	}

	result := VerifyAPIKey(key)
	if result.IsActive {
		t.Error("expected IsActive=false for 403")
	}
	if result.Error != "forbidden" {
		t.Fatalf("expected error=forbidden, got %s", result.Error)
	}
}

func TestVerifyAPIKey_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-server-error",
		Endpoint:     server.URL,
	}

	result := VerifyAPIKey(key)
	if result.IsActive {
		t.Error("expected IsActive=false for 500")
	}
	if result.Error != "http_500" {
		t.Fatalf("expected error=http_500, got %s", result.Error)
	}
}

func TestVerifyAPIKey_ConnectionError(t *testing.T) {
	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-conn-error",
		Endpoint:     "http://localhost:1",
	}

	result := VerifyAPIKey(key)
	if result.IsActive {
		t.Error("expected IsActive=false for connection error")
	}
	if result.Error == "" {
		t.Error("expected non-empty error for connection failure")
	}
}

func TestVerifyAPIKey_FallbackToV1Models(t *testing.T) {
	// First /models returns 404, fallback to /v1/models should work
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path == "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[{"id":"v1-model"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-fallback",
		Endpoint:     server.URL,
	}

	result := VerifyAPIKey(key)
	if !result.IsActive {
		t.Fatalf("expected IsActive=true via v1 fallback, got false (error: %s)", result.Error)
	}
	if requestCount < 2 {
		t.Error("expected at least 2 requests (models + v1/models)")
	}
}

func TestCheckURL_InvalidURL(t *testing.T) {
	if CheckURLAive("://invalid-url") {
		t.Error("expected false for invalid URL")
	}
}

func TestVerifyProviderPage_EmptyURL(t *testing.T) {
	provider := &models.Provider{Name: "Empty", URL: ""}
	result := VerifyProviderPage(provider)
	if result.URLAlive {
		t.Error("expected URLAlive=false for empty URL")
	}
}

func TestExtractKeyInfo_Pricing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"id":"free-model","context_length":4096,"pricing":"free"},
				{"id":"paid-model","context_length":8192,"pricing":"0.001"}
			]
		}`))
	}))
	defer server.Close()

	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-pricing-test",
		Endpoint:     server.URL,
	}

	info := ExtractKeyInfo(key)

	pricing, ok := info["pricing"].(map[string]string)
	if !ok {
		t.Fatal("pricing should be map[string]string")
	}
	if pricing["free-model"] != "free" {
		t.Fatalf("expected free pricing, got %s", pricing["free-model"])
	}
	if pricing["paid-model"] != "0.001" {
		t.Fatalf("expected 0.001 pricing, got %s", pricing["paid-model"])
	}
}
