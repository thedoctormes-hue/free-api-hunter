package storage

import (
	"os"
	"path/filepath"
	"testing"

	"free-api-hunter/internal/models"
)

func TestSaveAndLoadProviders(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := DataDir
	DataDir = tmpDir
	defer func() { DataDir = origDir }()

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

	err := SaveProviders(providers, "")
	if err != nil {
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
	tmpDir := t.TempDir()
	origDir := DataDir
	DataDir = tmpDir
	defer func() { DataDir = origDir }()

	findings := []*models.Finding{
		{
			SourceID:     "test-1",
			Title:        "Test Finding",
			URL:          "https://example.com",
			Description:  "A test finding description",
			QualityScore: 0.8,
		},
	}

	err := SaveFindings(findings, "")
	if err != nil {
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
	tmpDir := t.TempDir()
	origDir := DataDir
	DataDir = tmpDir
	defer func() { DataDir = origDir }()

	keys := []*models.APIKey{
		{
			ProviderName: "test",
			KeyLocation:  "vault/test/default.key",
			Endpoint:     "https://api.example.com/v1",
			IsActive:     true,
		},
	}

	err := SaveKeyPool(keys, "")
	if err != nil {
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
	tmpDir := t.TempDir()
	origDir := DataDir
	DataDir = tmpDir
	defer func() { DataDir = origDir }()

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
	tmpDir := filepath.Join(t.TempDir(), "subdir")
	origDir := DataDir
	DataDir = tmpDir
	defer func() { DataDir = origDir }()

	err := EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("EnsureDir should create a directory")
	}
}
