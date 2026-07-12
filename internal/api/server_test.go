package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/notify"
	"free-api-hunter/internal/storage"
	_ "modernc.org/sqlite"
)

func init() {
	os.Setenv("FREE_API_HUNTER_API_KEY", "test-key")
	SetAPIKeys([]string{"test-key"})
}

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
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Meta.Count is total count; check data has exactly 1 item
	data, ok := resp.Data.([]interface{})
	if !ok || len(data) != 1 {
		t.Errorf("expected 1 finding in data, got %d", len(data))
	}
}

func TestStats(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	writeTestData(t, dir)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
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
	req.Header.Set("X-API-Key", "test-key")
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

func TestSetVerdict(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	// Seed pending_review.json with one pending item.
	seed := notify.PendingReview{Pending: []notify.PendingItem{
		{Provider: "", Source: "https://example.com/find1", WhyFree: "free tier", FoundAt: models.Now(), Reviewed: false, Verdict: ""},
	}}
	b, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pending_review.json"), b, 0644); err != nil {
		t.Fatal(err)
	}

	body := `{"source":"https://example.com/find1","verdict":"confirmed"}`
	req := httptest.NewRequest("POST", "/api/v1/findings/verdict", strings.NewReader(body))
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp)
	}

	// Verify the verdict was persisted via notify.TriageSet.
	pr, err := notify.LoadPendingReview(filepath.Join(dir, "pending_review.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Pending) != 1 {
		t.Fatalf("expected 1 pending item, got %d", len(pr.Pending))
	}
	if !pr.Pending[0].Reviewed || pr.Pending[0].Verdict != "confirmed" {
		t.Errorf("expected reviewed=true verdict=confirmed, got reviewed=%v verdict=%q", pr.Pending[0].Reviewed, pr.Pending[0].Verdict)
	}
}

func TestSetVerdictBadVerdict(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	body := `{"source":"https://example.com/find1","verdict":"bogus"}`
	req := httptest.NewRequest("POST", "/api/v1/findings/verdict", strings.NewReader(body))
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Success {
		t.Errorf("expected success=false")
	}
}

func TestSetVerdictEmptyBody(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	body := `{"source":"","verdict":""}`
	req := httptest.NewRequest("POST", "/api/v1/findings/verdict", strings.NewReader(body))
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSetVerdictMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/api/v1/findings/verdict", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d body=%s", w.Code, w.Body.String())
	}
}

// Suppress unused import warning
var _ = os.Remove
