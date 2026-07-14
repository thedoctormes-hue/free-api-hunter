package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// resetScanLimiter fully clears scan state shared across tests.
// scan_limiter.reset() only nils current; lastScanAt persists and would make
// subsequent scans hit the 5-minute rate limit within a single test run.
func resetScanLimiter() {
	scan_limiter.reset()
	scan_limiter.lastScanAt = time.Time{}
}

func TestMetricsMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	h := metricsMiddleware(next)

	req := httptest.NewRequest("GET", "/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", w.Body.String())
	}
}

func TestLoggingMiddlewareCacheControl(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := LoggingMiddleware(next)

	req := httptest.NewRequest("GET", "/api/v1/providers", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Cache-Control") == "" {
		t.Error("expected Cache-Control header for /api/v1/providers")
	}
}

func TestHandleFindingsPagination(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings?limit=10&offset=0&source=foo", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleScanHistoryLimit(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/scan-history?limit=5", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleScanAlreadyRunning(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	resetScanLimiter()
	defer resetScanLimiter()
	s := NewServerWithDir("127.0.0.1:0", dir)

	// First trigger starts a scan (background goroutine keeps current="running").
	req1 := httptest.NewRequest("POST", "/api/v1/scan", nil)
	req1.Header.Set("X-API-Key", "test-key")
	w1 := httptest.NewRecorder()
	s.mux.ServeHTTP(w1, req1)
	if w1.Code != http.StatusAccepted {
		t.Fatalf("expected 202 on first trigger, got %d", w1.Code)
	}

	// Second trigger while still running -> 409 (deterministic within the 2s window).
	req2 := httptest.NewRequest("POST", "/api/v1/scan", nil)
	req2.Header.Set("X-API-Key", "test-key")
	w2 := httptest.NewRecorder()
	s.mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 (already running), got %d", w2.Code)
	}
}
