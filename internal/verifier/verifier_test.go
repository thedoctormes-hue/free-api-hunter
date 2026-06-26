package verifier

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"free-api-hunter/internal/models"
)

// allowLocalhost is a test helper that lets IsValidOutboundURL accept
// http://localhost and http://127.0.0.1 URLs (for httptest servers).
func allowLocalhost(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return u, nil
	}
	return nil, fmt.Errorf("scheme %q not allowed", u.Scheme)
}

// installTestClient replaces HTTPClient + ValidateOutboundURL for the duration
// of a test, restoring both on cleanup. Returns the cleanup function.
func installTestClient(server *httptest.Server) func() {
	origClient := HTTPClient
	origValidate := ValidateOutboundURL

	HTTPClient = server.Client()
	HTTPClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	ValidateOutboundURL = allowLocalhost

	return func() {
		HTTPClient = origClient
		ValidateOutboundURL = origValidate
	}
}

func TestCheckURLAive(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cleanup := installTestClient(server)
	defer cleanup()

	if !CheckURLAive(server.URL) {
		t.Error("CheckURLAive returned false for live URL")
	}

	if CheckURLAive("http://localhost:19999") {
		t.Error("CheckURLAive returned true for dead URL")
	}
}

func TestVerifyProviderPage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Free tier available! No credit card required.</body></html>"))
	}))
	defer server.Close()

	cleanup := installTestClient(server)
	defer cleanup()

	provider := &models.Provider{
		Name: "Test Provider",
		URL:  server.URL,
	}

	result := VerifyProviderPage(provider)

	if !result.URLAlive {
		t.Error("URLAlive should be true")
	}
	if !result.FreeTierMentioned {
		t.Error("FreeTierMentioned should be true")
	}
	if result.CreditCardReq == nil || *result.CreditCardReq {
		t.Error("CreditCardReq should be false")
	}
}

func TestVerifyProviderPageNoFreeTier(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Premium plans only. Credit card required.</body></html>"))
	}))
	defer server.Close()

	cleanup := installTestClient(server)
	defer cleanup()

	provider := &models.Provider{
		Name: "Premium Provider",
		URL:  server.URL,
	}

	result := VerifyProviderPage(provider)

	if !result.URLAlive {
		t.Error("URLAlive should be true")
	}
	if result.FreeTierMentioned {
		t.Error("FreeTierMentioned should be false")
	}
	if result.CreditCardReq == nil || !*result.CreditCardReq {
		t.Error("CreditCardReq should be true")
	}
}

func TestVerifyProviderPageDead(t *testing.T) {
	provider := &models.Provider{
		Name: "Dead Provider",
		URL:  "http://localhost:19999",
	}

	result := VerifyProviderPage(provider)

	if result.URLAlive {
		t.Error("URLAlive should be false for dead URL")
	}
}

func TestExtractKeyInfo(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"id":"model-1","context_length":128000,"pricing":"free"},
				{"id":"model-2","context_length":256000,"pricing":"paid"}
			]
		}`))
	}))
	defer server.Close()

	cleanup := installTestClient(server)
	defer cleanup()

	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "sk-test-key-12345",
		Endpoint:     server.URL,
	}

	info := ExtractKeyInfo(key)

	modelsList, ok := info["models"].([]string)
	if !ok || len(modelsList) != 2 {
		t.Errorf("expected 2 models, got %v", info["models"])
	}

	contexts, ok := info["contexts"].(map[string]int)
	if !ok {
		t.Error("contexts should be map[string]int")
	} else {
		if contexts["model-1"] != 128000 {
			t.Errorf("model-1 context should be 128000, got %d", contexts["model-1"])
		}
	}
}

func TestExtractKeyInfoError(t *testing.T) {
	key := &models.APIKey{
		ProviderName: "test",
		KeyLocation:  "test-key",
		Endpoint:     "http://localhost:19999",
	}

	info := ExtractKeyInfo(key)

	// При ошибке должен вернуть пустой список моделей
	modelsList, ok := info["models"].([]string)
	if !ok || len(modelsList) != 0 {
		t.Errorf("expected empty models list, got %v", info["models"])
	}
}
