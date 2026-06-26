package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const testAPIKey = "test-key"

func writeTTSData(t *testing.T, dir string) {
	t.Helper()
	data := `{
		"providers": [
			{"name": "ElevenLabs", "url": "https://elevenlabs.io", "free_tier": {"char_limit": 10000}},
			{"name": "OpenAI-TTS", "url": "https://openai.com"}
		],
		"verify_results": [
			{"is_active": true, "voices": ["voice1", "voice2"], "char_limit": 10000, "plan": "free"},
			{"is_active": false, "error": "no key"}
		],
		"scores": [
			{"provider_name": "ElevenLabs", "overall_score": 0.9}
		],
		"updated_at": "2026-06-26T00:00:00Z"
	}`
	path := filepath.Join(dir, "tts_providers.json")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestTTSProvidersEndpoint(t *testing.T) {
	dir := setupTestDir(t)
	writeTTSData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Meta.Count != 2 {
		t.Errorf("expected 2 TTS providers, got %d", resp.Meta.Count)
	}
}

func TestTTSProvidersNotFound(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTTSProviderByID(t *testing.T) {
	dir := setupTestDir(t)
	writeTTSData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers/ElevenLabs", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp.Data.(map[string]interface{})
	if data["name"] != "ElevenLabs" {
		t.Errorf("expected name=ElevenLabs, got %v", data["name"])
	}
}

func TestTTSProviderByIDNotFound(t *testing.T) {
	dir := setupTestDir(t)
	writeTTSData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers/nonexistent", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTTSProviderByIDEmpty(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers/", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTTSStats(t *testing.T) {
	dir := setupTestDir(t)
	writeTTSData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/stats", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp.Data.(map[string]interface{})
	if data["providers_total"].(float64) != 2 {
		t.Errorf("expected 2 providers_total, got %v", data["providers_total"])
	}
	if data["active_count"].(float64) != 1 {
		t.Errorf("expected 1 active_count, got %v", data["active_count"])
	}
}

func TestTTSStatsNotFound(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/stats", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTTSProvidersMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/tts/providers", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestTTSStatsMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/tts/stats", nil)
	req.Header.Set("X-API-Key", testAPIKey)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestProvidersFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_providers.json")
	data := `{"providers": [{"name": "Test", "url": "https://test.com"}]}`
	os.WriteFile(path, []byte(data), 0644)

	providers, err := ProvidersFromFile(path)
	if err != nil {
		t.Fatalf("ProvidersFromFile failed: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Name != "Test" {
		t.Errorf("expected 'Test', got %q", providers[0].Name)
	}
}

func TestProvidersFromFileNonexistent(t *testing.T) {
	_, err := ProvidersFromFile("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestProvidersFromFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := ProvidersFromFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
