package verifier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"free-api-hunter/internal/models"
)

func TestCheckURLAive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if !CheckURLAive(server.URL) {
		t.Error("CheckURLAive returned false for live URL")
	}

	if CheckURLAive("http://localhost:19999") {
		t.Error("CheckURLAive returned true for dead URL")
	}
}

func TestVerifyProviderPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Free tier available! No credit card required.</body></html>"))
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Premium plans only. Credit card required.</body></html>"))
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{"id":"model-1","context_length":128000,"pricing":"free"},
				{"id":"model-2","context_length":256000,"pricing":"paid"}
			]
		}`))
	}))
	defer server.Close()

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
