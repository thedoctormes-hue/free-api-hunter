package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPublicFindingsNoKey — GET /api/v1/findings БЕЗ X-API-Key должен вернуть 200.
func TestPublicFindingsNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without API key, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestPublicStatsNoKey — GET /api/v1/stats БЕЗ X-API-Key должен вернуть 200.
func TestPublicStatsNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without API key, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestPublicProvidersNoKey — GET /api/v1/providers БЕЗ X-API-Key должен вернуть 200.
func TestPublicProvidersNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without API key, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestPublicScanHistoryNoKey — GET /api/v1/scan-history БЕЗ X-API-Key должен вернуть 200.
func TestPublicScanHistoryNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/scan-history", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without API key, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestPublicVerdictNoKey — POST /api/v1/findings/verdict БЕЗ X-API-Key НЕ должен вернуть 401.
// Auth должен быть пропущен (префикс /api/v1/findings). Ожидаем 200/400/404 — но не 401.
func TestPublicVerdictNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	body := `{"source":"https://example.com/find1","verdict":"confirmed"}`
	req := httptest.NewRequest("POST", "/api/v1/findings/verdict", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Fatalf("expected NOT 401 (auth should be skipped for public verdict), got 401 body=%s", w.Body.String())
	}
}

// TestProtectedScanNoKey — POST /api/v1/scan БЕЗ X-API-Key должен остаться 401 "Missing API Key".
// Контроль: мутирующий эндпоинт защищён.
func TestProtectedScanNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/scan", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected scan without key, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Missing API Key") {
		t.Errorf("expected body to contain 'Missing API Key', got %q", w.Body.String())
	}
}

// TestProtectedTTSNoKey — GET /api/v1/tts/providers БЕЗ X-API-Key возвращает 200 (TTS каталог публичен).
// Проблема (eca25ec): я сделала TTS endpoints публичными для страницы TTS без ключа.
func TestProtectedTTSNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for public TTS catalog without key, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestProviderStatusNoKey — POST /api/v1/provider-status без X-API-Key возвращает 200 (верификация публична).
func TestProviderStatusNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	body, _ := json.Marshal(map[string]string{"name": "Cohere", "status": "confirmed"})
	req := httptest.NewRequest("POST", "/api/v1/provider-status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for public provider-status without key, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestProviderStatusInvalidStatus — POST /api/v1/provider-status с недопустимым status возвращает 400.
func TestProviderStatusInvalidStatus(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	body, _ := json.Marshal(map[string]string{"name": "Cohere", "status": "invalid"})
	req := httptest.NewRequest("POST", "/api/v1/provider-status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid status") {
		t.Errorf("expected body to contain 'invalid status', got %q", w.Body.String())
	}
}

// TestProviderStatusNotFound — POST /api/v1/provider-status с несуществующим name возвращает 500 из-за backend error.
func TestProviderStatusNotFound(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	body, _ := json.Marshal(map[string]string{"name": "NonExistentProvider", "status": "unverified"})
	req := httptest.NewRequest("POST", "/api/v1/provider-status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for not-found provider, got %d body=%s", w.Code, w.Body.String())
	}
}
