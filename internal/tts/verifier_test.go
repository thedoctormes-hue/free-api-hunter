package tts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

	// Инициализируем pool
	poolConfig := tmpDir + "/pool_config.json"
	os.WriteFile(poolConfig, []byte(`{
		"tts_providers": [{
			"id": "elevenlabs-test",
			"name": "ElevenLabs Test",
			"char_limit": 10000
		}]
	}`), 0644)
	if err := InitKeyPool(poolConfig); err != nil {
		t.Fatalf("InitKeyPool failed: %v", err)
	}

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

	// Инициализируем pool с ключом для этого теста
	tmpDir := t.TempDir()
	poolConfig := tmpDir + "/pool.json"
	os.WriteFile(poolConfig, []byte(`{"tts_providers":[{"id":"elevenlabs-invalid","name":"Test","char_limit":10000}]}`), 0644)
	InitKeyPool(poolConfig)

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
	// Провайдер без ключа в vault — pool не найдёт ключ
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer server.Close()

	// Инициализируем pool без ключей
	tmpDir := t.TempDir()
	poolConfig := tmpDir + "/pool.json"
	os.WriteFile(poolConfig, []byte(`{"tts_providers":[{"id":"test-no-key","name":"Test","char_limit":10000}]}`), 0644)
	InitKeyPool(poolConfig)

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

	if !strings.Contains(result.Error, "no_active_keys") && !strings.Contains(result.Error, "no_valid_key") && !strings.Contains(result.Error, "vault_key_not_found") {
		t.Errorf("expected error about missing keys, got: %s", result.Error)
	}
}

func TestKeyPoolRoundRobin(t *testing.T) {
	tmpDir := t.TempDir()
	vaultDir := tmpDir + "/free-api-hunter/test-provider"
	os.MkdirAll(vaultDir, 0755)
	os.WriteFile(vaultDir+"/api.key", []byte("key-1"), 0600)
	os.WriteFile(vaultDir+"/api.key.1", []byte("key-2"), 0600)
	os.WriteFile(vaultDir+"/api.key.2", []byte("key-3"), 0600)

	// Подменяем VaultPath для keypool
	origVaultPath := vault.VaultPath
	vault.VaultPath = tmpDir
	defer func() { vault.VaultPath = origVaultPath }()

	poolConfig := tmpDir + "/pool.json"
	os.WriteFile(poolConfig, []byte(`{"tts_providers":[{"id":"test-provider","name":"Test","char_limit":10000}]}`), 0644)

	p, err := NewKeyPool(poolConfig)
	if err != nil {
		t.Fatalf("NewKeyPool failed: %v", err)
	}

	// Round-robin: 3 ключа должны меняться
	keys := make([]string, 3)
	for i := 0; i < 3; i++ {
		entry, err := p.Next()
		if err != nil {
			t.Fatalf("Next() failed: %v", err)
		}
		keys[i] = entry.Key
	}

	if keys[0] == keys[1] || keys[1] == keys[2] {
		t.Errorf("expected round-robin rotation, got: %v", keys)
	}
}

func TestKeyPoolExhaustion(t *testing.T) {
	tmpDir := t.TempDir()
	vaultDir := tmpDir + "/free-api-hunter/test-provider"
	os.MkdirAll(vaultDir, 0755)
	os.WriteFile(vaultDir+"/api.key", []byte("key-1"), 0600)

	// Подменяем VaultPath для keypool
	origVaultPath := vault.VaultPath
	vault.VaultPath = tmpDir
	defer func() { vault.VaultPath = origVaultPath }()

	poolConfig := tmpDir + "/pool.json"
	os.WriteFile(poolConfig, []byte(`{"tts_providers":[{"id":"test-provider","name":"Test","char_limit":100}]}`), 0644)

	p, err := NewKeyPool(poolConfig)
	if err != nil {
		t.Fatalf("NewKeyPool failed: %v", err)
	}

	// Исчерпаем ключ
	p.ReportUsage("key-1", 100)

	// Теперь пул должен быть пуст
	_, err = p.Next()
	if err == nil {
		t.Error("expected error when all keys exhausted")
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
