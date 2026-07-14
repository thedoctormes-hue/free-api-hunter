package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleIndex(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected html content-type, got %s", w.Header().Get("Content-Type"))
	}
}

func TestHandleIndexNotFound(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/nope", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleProvidersEmpty(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got error=%q", resp.Error)
	}
}

func TestHandleProvidersMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/providers", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleProviderByIDNotFound(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers/nope", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleProviderByIDMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/providers/nope", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleFindingsEmpty(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleStats(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data, _ := resp.Data.(map[string]interface{})
	if data["server_time"] == nil {
		t.Error("expected server_time in stats")
	}
}

func TestHandleScanHistory(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/scan-history", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHandleScanStatus(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	resetScanLimiter() // isolate from other tests' scan state
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/scan", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data, _ := resp.Data.(map[string]interface{})
	if data["status"] != "idle" {
		t.Errorf("expected status=idle, got %v", data["status"])
	}
}

func TestHandleScanTrigger(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	resetScanLimiter() // isolate from other tests' scan state
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/scan", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestHandleTTSProvidersNotFound(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleTTSProviderByIDNotFound(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/providers/nope", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleTTSStatsNotFound(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/tts/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleSetVerdictInvalidBody(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/findings/verdict", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetVerdictMissingFields(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/findings/verdict", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetVerdictInvalidVerdict(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	body := `{"source":"https://example.com/x","verdict":"bogus"}`
	req := httptest.NewRequest("POST", "/api/v1/findings/verdict", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetVerdictMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings/verdict", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleSetProviderStatusInvalidBody(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/provider-status", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetProviderStatusMissingFields(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/provider-status", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetProviderStatusInvalidStatus(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	body := `{"name":"x","status":"bogus"}`
	req := httptest.NewRequest("POST", "/api/v1/provider-status", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetProviderStatusMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/provider-status", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleScanMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("DELETE", "/api/v1/scan", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleStatsMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
