package tts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/vault"
)

func TestVerifyTTSKey_Success(t *testing.T) {
	// Mock ElevenLabs API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/user/subscription":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tier": "free",
			})
		case "/v1/voices":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"voices": []map[string]string{
					{"name": "Adam", "voice_id": "pNInz6obpgDQGcFmaJgB"},
					{"name": "Antoni", "voice_id": "ErXwobaYiN019PkySvjV"},
				},
			})
		default:
			// TTS endpoint
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(200)
			w.Write([]byte("fake-audio-data"))
		}
	}))
	defer server.Close()

	// Создаём временный vault с ключом
	tmpDir := t.TempDir()
	vaultPath := tmpDir + "/free-api-hunter/elevenlabs-test"
	os.MkdirAll(vaultPath, 0755)
	os.WriteFile(vaultPath+"/api.key", []byte("test-key-123"), 0600)

	// Подменяем VaultPath на временный
	origVaultPath := vault.VaultPath
	vault.VaultPath = tmpDir
	defer func() { vault.VaultPath = origVaultPath }()

	provider := &models.TTSProvider{
		Name:       "elevenlabs-test",
		URL:        server.URL,
		APIKeyURL:  server.URL + "/v1/user/subscription",
		CreditCard: false,
		Models:     []string{"eleven_v3"},
	}

	result := VerifyTTSKey(provider)

	// Проверяем что результат не nil
	if result == nil {
		t.Fatal("VerifyTTSKey returned nil")
	}

	// Проверяем что ключ активен (mock сервер возвращает 200)
	if !result.IsActive {
		t.Errorf("expected IsActive=true, got false (error: %s)", result.Error)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected StatusCode=200, got %d", result.StatusCode)
	}
}

func TestVerifyTTSKey_InvalidKey(t *testing.T) {
	// Mock сервер возвращающий 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"detail":{"status":"unauthorized"}}`))
	}))
	defer server.Close()

	provider := &models.TTSProvider{
		Name:      "elevenlabs-invalid",
		URL:       server.URL,
		APIKeyURL: server.URL + "/v1/user/subscription",
	}

	result := VerifyTTSKey(provider)

	if result == nil {
		t.Fatal("VerifyTTSKey returned nil")
	}

	if result.IsActive {
		t.Error("expected IsActive=false for invalid key")
	}
}

func TestVerifyTTSKey_NoVaultKey(t *testing.T) {
	// Провайдер без ключа в vault — fallback на запрос без ключа
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer server.Close()

	provider := &models.TTSProvider{
		Name:      "test-no-key",
		URL:       server.URL,
		APIKeyURL: server.URL + "/v1/user/subscription",
	}

	result := VerifyTTSKey(provider)

	if result == nil {
		t.Fatal("VerifyTTSKey returned nil")
	}

	if result.IsActive {
		t.Error("expected IsActive=false without key")
	}

	if result.Error != "no_valid_key" && result.Error != "vault_key_not_found" {
		t.Errorf("expected 'no_valid_key' or 'vault_key_not_found', got: %s", result.Error)
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ElevenLabs", "elevenlabs"},
		{"Google Cloud", "google-cloud"},
		{"OpenAI (ChatGPT)", "openai-chatgpt"},
		{"test/provider", "testprovider"},
	}

	for _, tt := range tests {
		got := normalizeName(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
