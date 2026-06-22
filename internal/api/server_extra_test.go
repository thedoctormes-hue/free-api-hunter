package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIProvidersEmpty(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 0 {
		t.Errorf("expected 0 providers, got %d", resp.Meta.Count)
	}
}

func TestAPIProviderByIDCaseSensitive(t *testing.T) {
	dir := setupTestDir(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	// Wrong case should not match
	req := httptest.NewRequest("GET", "/api/v1/providers/cohere", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong case, got %d", w.Code)
	}
}

func TestAPIFindingsEmpty(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 0 {
		t.Errorf("expected 0 findings, got %d", resp.Meta.Count)
	}
}

func TestAPIStatsEmpty(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp.Data.(map[string]interface{})
	if data["providers_total"].(float64) != 0 {
		t.Errorf("expected 0 providers_total, got %v", data["providers_total"])
	}
}

func TestAPIMethodNotAllowedFindings(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("DELETE", "/api/v1/findings", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestAPIMethodNotAllowedStats(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("PUT", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestAPIProviderByIDEmpty(t *testing.T) {
	dir := setupTestDir(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers/", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPIFindingsLimitZero(t *testing.T) {
	dir := setupTestDir(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings?limit=0", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	// limit=0 means no limit, should return all
	if resp.Meta.Count != 3 {
		t.Errorf("expected 3 findings with limit=0, got %d", resp.Meta.Count)
	}
}

func TestAPIFindingsMultipleFilters(t *testing.T) {
	dir := setupTestDir(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings?source=hackernews&limit=1", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 1 {
		t.Errorf("expected 1 finding, got %d", resp.Meta.Count)
	}
}
