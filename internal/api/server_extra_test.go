package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/storage"
	_ "modernc.org/sqlite"
)

func setupExtraTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	storage.DataDir = dir
	if err := database.Init(dir); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	return dir
}

func writeExtraTestData(t *testing.T, dir string) {
	t.Helper()
	providers := []*models.Provider{
		{Name: "Cohere", Status: models.StatusVerified, CreditCard: false, Models: []string{"command-r"}},
	}
	if err := storage.SaveProviders(providers, ""); err != nil {
		t.Fatal(err)
	}

	findings := []*models.Finding{
		{Title: "Free API 1", SourceID: "hackernews", QualityScore: 0.8},
	}
	if err := storage.SaveFindings(findings, ""); err != nil {
		t.Fatal(err)
	}
}

func TestAPIProvidersEmpty(t *testing.T) {
	dir := setupExtraTestDir(t)
	defer database.Close()
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
	dir := setupExtraTestDir(t)
	defer database.Close()
	writeExtraTestData(t, dir)
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
	dir := setupExtraTestDir(t)
	defer database.Close()
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
	dir := setupExtraTestDir(t)
	defer database.Close()
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
	dir := setupExtraTestDir(t)
	defer database.Close()
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("DELETE", "/api/v1/findings", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestAPIMethodNotAllowedStats(t *testing.T) {
	dir := setupExtraTestDir(t)
	defer database.Close()
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("PUT", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestAPIProviderByIDEmpty(t *testing.T) {
	dir := setupExtraTestDir(t)
	defer database.Close()
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers/", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPIFindingsLimitZero(t *testing.T) {
	dir := setupExtraTestDir(t)
	defer database.Close()
	writeExtraTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings?limit=0", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	// limit=0 means no limit, should return all
	if resp.Meta.Count != 1 {
		t.Errorf("expected 1 finding with limit=0, got %d", resp.Meta.Count)
	}
}

func TestAPIFindingsMultipleFilters(t *testing.T) {
	dir := setupExtraTestDir(t)
	defer database.Close()
	writeExtraTestData(t, dir)
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

func TestAPIScanHistory(t *testing.T) {
	dir := setupExtraTestDir(t)
	defer database.Close()

	// Insert scan history
	if err := storage.SaveScanHistory(5, 3, 2, 1); err != nil {
		t.Fatal(err)
	}

	s := NewServerWithDir("127.0.0.1:0", dir)
	req := httptest.NewRequest("GET", "/api/v1/scan-history", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 1 {
		t.Errorf("expected 1 scan history entry, got %d", resp.Meta.Count)
	}
}
