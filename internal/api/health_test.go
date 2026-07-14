package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"free-api-hunter/internal/vault"
)

func TestHandleLiveness(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/health/live", nil)
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
	if data["status"] != "alive" {
		t.Errorf("expected status=alive, got %v", data["status"])
	}
}

func TestHandleLivenessMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/health/live", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleReadinessOK(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir) // DataDir writable, DB initialized

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp.Data.(map[string]interface{})
	if data["sqlite"] != "ok" {
		t.Errorf("expected sqlite=ok, got %v", data["sqlite"])
	}
	if data["data_dir"] != "ok" {
		t.Errorf("expected data_dir=ok, got %v", data["data_dir"])
	}
}

func TestHandleReadinessNoDataDir(t *testing.T) {
	setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", "") // DataDir empty

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp.Data.(map[string]interface{})
	if data["data_dir"] != "not_configured" {
		t.Errorf("expected data_dir=not_configured, got %v", data["data_dir"])
	}
}

func TestHandleDeepHealth(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	// providers.json present so storage/last_scan checks resolve
	if err := os.WriteFile(filepath.Join(dir, "providers.json"), []byte(`{"providers":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("GET", "/health/deep", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// 200 (ok/warn) or 500 (error) — both exercise the handler
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d", w.Code)
	}
	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data == nil {
		t.Error("expected deep health data")
	}
}

func TestHandleDeepHealthMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/health/deep", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleHealth(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
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
	data, _ := resp.Data.(map[string]interface{})
	if data["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", data["status"])
	}
}

func TestHandleHealthExtended(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	if err := os.WriteFile(filepath.Join(dir, "providers.json"), []byte(`{"providers":[{"name":"x"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "findings.json"), []byte(`{"findings":[{"title":"y"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewServerWithDir("127.0.0.1:0", dir)

	// handleHealthExtended is dead code (not registered in routes) — invoke directly for coverage.
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.handleHealthExtended(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	dataBytes, _ := json.Marshal(resp.Data)
	var hs HealthStatus
	if err := json.Unmarshal(dataBytes, &hs); err != nil {
		t.Fatal(err)
	}
	if hs.ProvidersCount != 1 {
		t.Errorf("expected ProvidersCount=1, got %d", hs.ProvidersCount)
	}
	if hs.FindingsCount != 1 {
		t.Errorf("expected FindingsCount=1, got %d", hs.FindingsCount)
	}
}

func TestCountItems(t *testing.T) {
	dir := t.TempDir()

	p := filepath.Join(dir, "providers.json")
	if err := os.WriteFile(p, []byte(`{"providers":[{"a":1},{"b":2}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := countItems(p); got != 2 {
		t.Errorf("expected 2 providers, got %d", got)
	}

	f := filepath.Join(dir, "findings.json")
	if err := os.WriteFile(f, []byte(`{"findings":[{"x":1},{"y":2},{"z":3}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := countItems(f); got != 3 {
		t.Errorf("expected 3 findings, got %d", got)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`not json`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := countItems(bad); got != 0 {
		t.Errorf("expected 0 on invalid json, got %d", got)
	}

	if got := countItems(filepath.Join(dir, "nope.json")); got != 0 {
		t.Errorf("expected 0 on missing file, got %d", got)
	}
}

func TestCheckVault(t *testing.T) {
	old := vault.VaultPath
	defer func() { vault.VaultPath = old }()

	vault.VaultPath = ""
	if c := checkVault(); c.Status != "warn" {
		t.Errorf("expected warn for empty path, got %q", c.Status)
	}

	vault.VaultPath = filepath.Join(t.TempDir(), "does-not-exist")
	if c := checkVault(); c.Status != "warn" {
		t.Errorf("expected warn for missing dir, got %q", c.Status)
	}
}

func TestCheckStorageJSON(t *testing.T) {
	if c := checkStorageJSON(""); c.Status != "warn" {
		t.Errorf("expected warn for empty dir, got %q", c.Status)
	}
	dir := t.TempDir()
	if c := checkStorageJSON(dir); c.Status != "warn" {
		t.Errorf("expected warn for missing providers.json, got %q", c.Status)
	}
	if err := os.WriteFile(filepath.Join(dir, "providers.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if c := checkStorageJSON(dir); c.Status != "ok" {
		t.Errorf("expected ok, got %q", c.Status)
	}
}

func TestCheckSQLite(t *testing.T) {
	setupTestDir(t)
	defer cleanupDB(t)
	if c := checkSQLite(); c.Status != "ok" {
		t.Errorf("expected ok, got %q", c.Status)
	}
}

func TestCheckLastScan(t *testing.T) {
	dir := t.TempDir()
	if c := checkLastScan(dir); c.Status != "warn" {
		t.Errorf("expected warn for missing providers.json, got %q", c.Status)
	}
	if err := os.WriteFile(filepath.Join(dir, "providers.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if c := checkLastScan(dir); c.Status == "" {
		t.Error("expected non-empty status")
	}
}

func TestCheckOrexProxy(t *testing.T) {
	// Network call may succeed or fail; we only verify it returns a populated check.
	c := checkOrexProxy()
	if c.Status == "" {
		t.Error("expected non-empty status")
	}
}
