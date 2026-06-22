package storage

import (
	"path/filepath"
	"testing"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/orex"
)

func TestSaveAndLoadOrexCache(t *testing.T) {
	dir := t.TempDir()
	DataDir = dir

	cache := &OrexCache{
		FreeModels: []orex.FreeModel{
			{ID: "openrouter/free-1", Name: "free-1", Provider: "openrouter", ContextLength: 4096, IsFree: true},
			{ID: "cerebras/free-2", Name: "free-2", Provider: "cerebras", ContextLength: 8192, IsFree: true},
		},
		Alerts: []orex.OrexAlert{
			{Type: "new_model", Model: "free-1", Message: "New free model", Timestamp: "2026-06-22T00:00:00Z"},
		},
	}

	if err := SaveOrexCache(cache, filepath.Join(dir, "orex_cache.json")); err != nil {
		t.Fatalf("SaveOrexCache error: %v", err)
	}

	loaded, err := LoadOrexCache(filepath.Join(dir, "orex_cache.json"))
	if err != nil {
		t.Fatalf("LoadOrexCache error: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadOrexCache returned nil")
	}
	if len(loaded.FreeModels) != 2 {
		t.Fatalf("expected 2 free models, got %d", len(loaded.FreeModels))
	}
	if len(loaded.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(loaded.Alerts))
	}
	if loaded.Meta.Count != 2 {
		t.Fatalf("expected meta count 2, got %d", loaded.Meta.Count)
	}
}

func TestLoadOrexCacheNonexistent(t *testing.T) {
	dir := t.TempDir()
	DataDir = dir

	loaded, err := LoadOrexCache(filepath.Join(dir, "nonexistent.json"))
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil for nonexistent file")
	}
}

func TestMergeOrexProviders_NewProvider(t *testing.T) {
	existing := []*models.Provider{
		{Name: "OpenRouter", Status: models.StatusVerified, Models: []string{"gpt-4"}},
	}

	freeModels := []orex.FreeModel{
		{ID: "openrouter/gpt-oss-120b", Name: "gpt-oss-120b", Provider: "OpenRouter", ContextLength: 131072, IsFree: true},
		{ID: "cerebras/llama-3.1-8b", Name: "llama-3.1-8b", Provider: "Cerebras", ContextLength: 8192, IsFree: true},
	}

	result := MergeOrexProviders(existing, freeModels)

	// Should have 2 providers now (OpenRouter + Cerebras)
	if len(result) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(result))
	}

	// OpenRouter should have new model added
	for _, p := range result {
		if p.Name == "OpenRouter" {
			hasModel := false
			for _, m := range p.Models {
				if m == "gpt-oss-120b" {
					hasModel = true
				}
			}
			if !hasModel {
				t.Error("OpenRouter should have gpt-oss-120b model")
			}
		}
	}
}

func TestMergeOrexProviders_ExistingModel(t *testing.T) {
	existing := []*models.Provider{
		{Name: "OpenRouter", Status: models.StatusVerified, Models: []string{"gpt-4", "gpt-oss-120b"}},
	}

	freeModels := []orex.FreeModel{
		{ID: "openrouter/gpt-oss-120b", Name: "gpt-oss-120b", Provider: "OpenRouter", ContextLength: 131072, IsFree: true},
	}

	result := MergeOrexProviders(existing, freeModels)

	// Should still have 1 provider with 2 models (no duplicate)
	if len(result) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result))
	}
	if len(result[0].Models) != 2 {
		t.Fatalf("expected 2 models (no duplicate), got %d", len(result[0].Models))
	}
}

func TestMergeOrexProviders_Empty(t *testing.T) {
	// Empty existing — new provider added
	result := MergeOrexProviders(nil, []orex.FreeModel{
		{ID: "new/model", Name: "model", Provider: "NewProvider", IsFree: true},
	})
	if len(result) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result))
	}
	if result[0].Name != "NewProvider" {
		t.Fatalf("expected name NewProvider, got %s", result[0].Name)
	}

	// Empty free models
	existing := []*models.Provider{{Name: "Existing"}}
	result = MergeOrexProviders(existing, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result))
	}
}

func TestMergeOrexProviders_UnverifiedBecomesClaimed(t *testing.T) {
	existing := []*models.Provider{
		{Name: "OpenRouter", Status: models.StatusUnverified, Models: []string{"old-model"}},
	}

	freeModels := []orex.FreeModel{
		{ID: "openrouter/new-model", Name: "new-model", Provider: "OpenRouter", IsFree: true},
	}

	result := MergeOrexProviders(existing, freeModels)
	if result[0].Status != models.StatusClaimed {
		t.Fatalf("expected status claimed, got %s", result[0].Status)
	}
}
