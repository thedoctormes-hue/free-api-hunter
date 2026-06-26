package storage

import (
	"path/filepath"
	"testing"

	"free-api-hunter/internal/models"
)

func BenchmarkSaveProviders(b *testing.B) {
	dir := b.TempDir()
	origDir := DataDir
	DataDir = dir
	defer func() { DataDir = origDir }()

	providers := make([]*models.Provider, 50)
	for i := range providers {
		providers[i] = &models.Provider{
			Name:      "Provider" + string(rune('A'+i%26)),
			URL:       "https://example.com",
			APIKeyURL: "https://example.com/keys",
			Status:    models.StatusVerified,
			Models:    []string{"model-1", "model-2", "model-3"},
			Limits:    "100 RPM",
			Source:    "bench",
			Priority:  models.PriorityHigh,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SaveProviders(providers, filepath.Join(dir, "bench_providers.json"))
	}
}

func BenchmarkLoadProviders(b *testing.B) {
	dir := b.TempDir()
	origDir := DataDir
	DataDir = dir
	defer func() { DataDir = origDir }()

	providers := make([]*models.Provider, 50)
	for i := range providers {
		providers[i] = &models.Provider{
			Name:   "Provider" + string(rune('A'+i%26)),
			URL:    "https://example.com",
			Status: models.StatusVerified,
			Models: []string{"model-1", "model-2"},
		}
	}
	SaveProviders(providers, filepath.Join(dir, "bench_providers.json"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadProviders(filepath.Join(dir, "bench_providers.json"))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSaveFindings(b *testing.B) {
	dir := b.TempDir()
	origDir := DataDir
	DataDir = dir
	defer func() { DataDir = origDir }()

	findings := make([]*models.Finding, 100)
	for i := range findings {
		findings[i] = &models.Finding{
			SourceID:     "bench_" + string(rune('a'+i%26)),
			Title:        "Free API Finding",
			URL:          "https://example.com/api",
			Description:  "A test finding description that is long enough",
			QualityScore: 0.75,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SaveFindings(findings, filepath.Join(dir, "bench_findings.json"))
	}
}

func BenchmarkLoadFindings(b *testing.B) {
	dir := b.TempDir()
	origDir := DataDir
	DataDir = dir
	defer func() { DataDir = origDir }()

	findings := make([]*models.Finding, 100)
	for i := range findings {
		findings[i] = &models.Finding{
			SourceID:     "bench_" + string(rune('a'+i%26)),
			Title:        "Free API Finding",
			URL:          "https://example.com/api",
			Description:  "A test finding description",
			QualityScore: 0.75,
		}
	}
	SaveFindings(findings, filepath.Join(dir, "bench_findings.json"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadFindings(filepath.Join(dir, "bench_findings.json"))
		if err != nil {
			b.Fatal(err)
		}
	}
}
