package storage

import (
	"path/filepath"
	"testing"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/models"
	_ "modernc.org/sqlite"
)

func initTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	DataDir = tmpDir
	if err := database.Init(tmpDir); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
}

func cleanupDB(t *testing.T) {
	t.Helper()
	database.Close()
}

func TestSaveAndLoadProviders(t *testing.T) {
	initTestDB(t)
	defer cleanupDB(t)

	providers := []*models.Provider{
		{
			Name:       "Test Provider",
			URL:        "https://example.com",
			APIKeyURL:  "https://example.com/keys",
			Status:     models.StatusConfirmed,
			Models:     []string{"model-1", "model-2"},
			Limits:     "100 RPM",
		},
	}

	if err := SaveProviders(providers, ""); err != nil {
		t.Fatalf("SaveProviders failed: %v", err)
	}

	loaded, err := LoadProviders("")
	if err != nil {
		t.Fatalf("LoadProviders failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(loaded))
	}
	if loaded[0].Name != "Test Provider" {
		t.Errorf("Expected name 'Test Provider', got %q", loaded[0].Name)
	}
	if loaded[0].Status != models.StatusConfirmed {
		t.Errorf("Expected status 'confirmed', got %q", loaded[0].Status)
	}
}

func TestSaveAndLoadFindings(t *testing.T) {
	initTestDB(t)
	defer cleanupDB(t)

	findings := []*models.Finding{
		{
			SourceID:     "test-1",
			Title:        "Test Finding",
			URL:          "https://example.com",
			Description:  "A test finding description",
			QualityScore: 0.8,
		},
	}

	if err := SaveFindings(findings, ""); err != nil {
		t.Fatalf("SaveFindings failed: %v", err)
	}

	loaded, err := LoadFindings("")
	if err != nil {
		t.Fatalf("LoadFindings failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Expected 1 finding, got %d", len(loaded))
	}
	if loaded[0].Title != "Test Finding" {
		t.Errorf("Expected title 'Test Finding', got %q", loaded[0].Title)
	}
}

func TestSaveAndLoadKeyPool(t *testing.T) {
	initTestDB(t)
	defer cleanupDB(t)

	keys := []*models.APIKey{
		{
			ProviderName: "test",
			KeyLocation:  "vault/test/default.key",
			Endpoint:     "https://api.example.com/v1",
			IsActive:     true,
		},
	}

	if err := SaveKeyPool(keys, ""); err != nil {
		t.Fatalf("SaveKeyPool failed: %v", err)
	}

	loaded, err := LoadKeyPool("")
	if err != nil {
		t.Fatalf("LoadKeyPool failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(loaded))
	}
	if !loaded[0].IsActive {
		t.Error("Key should be active")
	}
}

func TestLoadNonexistent(t *testing.T) {
	initTestDB(t)
	defer cleanupDB(t)

	providers, err := LoadProviders("")
	if err != nil {
		t.Fatalf("LoadProviders failed: %v", err)
	}
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers, got %d", len(providers))
	}

	findings, err := LoadFindings("")
	if err != nil {
		t.Fatalf("LoadFindings failed: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(findings))
	}
}

func TestEnsureDir(t *testing.T) {
	DataDir = filepath.Join(t.TempDir(), "subdir")
	err := EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
}

func TestScanHistory(t *testing.T) {
	initTestDB(t)
	defer cleanupDB(t)

	if err := SaveScanHistory(10, 5, 3, 2); err != nil {
		t.Fatalf("SaveScanHistory failed: %v", err)
	}
	if err := SaveScanHistory(20, 8, 4, 3); err != nil {
		t.Fatalf("SaveScanHistory failed: %v", err)
	}

	history, err := LoadScanHistory(10)
	if err != nil {
		t.Fatalf("LoadScanHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("Expected 2 history entries, got %d", len(history))
	}

	// Most recent first
	if history[0]["raw_count"].(int) != 20 {
		t.Errorf("Expected raw_count=20, got %v", history[0]["raw_count"])
	}
}

func TestProviderModelsRoundTrip(t *testing.T) {
	initTestDB(t)
	defer cleanupDB(t)

	providers := []*models.Provider{
		{
			Name:   "TestWithModels",
			Models: []string{"gpt-4", "claude-3"},
			Status: models.StatusVerified,
		},
	}

	if err := SaveProviders(providers, ""); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadProviders("")
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded[0].Models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(loaded[0].Models))
	}
}

func TestProviderLastVerified(t *testing.T) {
	initTestDB(t)
	defer cleanupDB(t)

	now := "2026-06-26T00:00:00Z"
	providers := []*models.Provider{
		{
			Name:         "WithVerified",
			LastVerified: &now,
		},
	}

	if err := SaveProviders(providers, ""); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadProviders("")
	if err != nil {
		t.Fatal(err)
	}

	if loaded[0].LastVerified == nil {
		t.Fatal("LastVerified should not be nil")
	}
	if *loaded[0].LastVerified != now {
		t.Errorf("Expected last_verified=%s, got %s", now, *loaded[0].LastVerified)
	}
}
