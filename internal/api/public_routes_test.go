package api

import (
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

// TestProtectedTTSNoKey — GET /api/v1/tts/providers БЕЗ X-API-Key должен остаться 401.
// Контроль: TTS-эндпоинты защищены.
func TestProtectedTTSNoKey(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected TTS without key, got %d body=%s", w.Code, w.Body.String())
	}
}
