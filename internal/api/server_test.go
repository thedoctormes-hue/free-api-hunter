package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/storage"
	_ "modernc.org/sqlite"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	storage.DataDir = dir
	// Init SQLite in the same dir
	if err := database.Init(dir); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	return dir
}

func writeTestData(t *testing.T, dir string) {
	t.Helper()

	providers := []*models.Provider{
		{Name: "Cohere", Status: models.StatusVerified, CreditCard: false, Models: []string{"command-r"}},
		{Name: "Groq", Status: models.StatusVerified, CreditCard: false, Models: []string{"llama-3.3-70b"}},
		{Name: "PaidAPI", Status: models.StatusClaimed, CreditCard: true, Models: []string{"gpt-4"}},
	}
	if err := storage.SaveProviders(providers, ""); err != nil {
		t.Fatal(err)
	}

	findings := []*models.Finding{
		{Title: "Free API 1", SourceID: "hackernews", QualityScore: 0.8},
		{Title: "Free API 2", SourceID: "github", QualityScore: 0.6},
		{Title: "Free API 3", SourceID: "hackernews", QualityScore: 0.4},
	}
	if err := storage.SaveFindings(findings, ""); err != nil {
		t.Fatal(err)
	}
}

func cleanupDB(t *testing.T) {
	t.Helper()
	database.Close()
}

func TestHealth(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/health", nil)
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
		t.Error("expected success=true")
	}
}

func TestListProviders(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
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
	if resp.Meta.Count != 3 {
		t.Errorf("expected 3 providers, got %d", resp.Meta.Count)
	}
}

func TestFilterProvidersByStatus(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers?status=verified", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 2 {
		t.Errorf("expected 2 verified providers, got %d", resp.Meta.Count)
	}
}

func TestFilterProvidersNoCreditCard(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers?credit_card=false", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 2 {
		t.Errorf("expected 2 no-cc providers, got %d", resp.Meta.Count)
	}
}

func TestGetProviderByID(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers/Cohere", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp.Data.(map[string]interface{})
	if data["name"] != "Cohere" {
		t.Errorf("expected name=Cohere, got %v", data["name"])
	}
}

func TestGetProviderNotFound(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/providers/nonexistent", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListFindings(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)
	defer cleanupDB(t)
	writeTestData(t, dir)
	_ = writeTestData

	req := httptest.NewRequest("GET", "/api/v1/findings", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 3 {
		t.Errorf("expected 3 findings, got %d", resp.Meta.Count)
	}
}

func TestFilterFindingsBySource(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings?source=hackernews", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 2 {
		t.Errorf("expected 2 hackernews findings, got %d", resp.Meta.Count)
	}
	// Verify data is array
	arr, ok := resp.Data.([]interface{})
	if !ok || len(arr) != 2 {
		t.Errorf("expected array of 2, got %v", resp.Data)
	}
}

func TestFindingsLimit(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings?limit=1", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Meta.Count != 1 {
		t.Errorf("expected 1 finding, got %d", resp.Meta.Count)
	}
}

func TestStats(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
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
	if data["providers_total"].(float64) != 3 {
		t.Errorf("expected 3 providers_total, got %v", data["providers_total"])
	}
	if data["findings_total"].(float64) != 3 {
		t.Errorf("expected 3 findings_total, got %v", data["findings_total"])
	}
}

func TestMethodNotAllowed(t *testing.T) {
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

func TestNotFound(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestIndexPage(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html, got %s", ct)
	}
}

// Suppress unused import warning
var _ = os.Remove
