package filter

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"free-api-hunter/internal/models"
)

func TestFilterFuzzFindings(t *testing.T) {
	engine := NewEngine()

	// Generate random findings
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .-_/:&?="

	for i := 0; i < 100; i++ {
		titleLen := rng.Intn(50) + 5
		descLen := rng.Intn(200) + 10
		title := make([]byte, titleLen)
		desc := make([]byte, descLen)
		for j := range title {
			title[j] = chars[rng.Intn(len(chars))]
		}
		for j := range desc {
			desc[j] = chars[rng.Intn(len(chars))]
		}

		f := models.Finding{
			SourceID:    "fuzz_" + string(rune('a'+i%26)),
			Title:       string(title),
			URL:         "https://example" + string(rune('a'+i%26)) + ".com",
			Description: string(desc),
			RawText:     string(title) + " " + string(desc),
		}

		// Should not panic
		engine.FilterFindings([]models.Finding{f})
	}
}

func TestFilterFuzzSpamPattern(t *testing.T) {
	patterns := []string{
		"купить сейчас",
		"продать скидка 50%",
		"рефералка",
		"affiliate link here",
		"click here to buy",
		"скидка! специальное предложение",
		"normal text without spam",
		"free API access with no spam",
		"",
		strings.Repeat("x", 1000),
	}

	engine := NewEngine()
	for _, p := range patterns {
		// Should not panic
		_ = engine.SpamPattern.MatchString(p)
	}
}

func TestFilterFuzzRawText(t *testing.T) {
	engine := NewEngine()
	engine.ExcludeKeywords = []string{"spam", "scam"}

	// Test with various random texts
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		textLen := rng.Intn(500)
		text := make([]byte, textLen)
		for j := range text {
			text[j] = byte(rng.Intn(128))
		}

		f := models.Finding{
			SourceID:    "fuzz",
			Title:       "Test",
			URL:         "https://test.com",
			Description: "A sufficiently long description that passes the minimum length filter check for testing",
			RawText:     string(text),
		}

		// Should not panic
		engine.FilterFindings([]models.Finding{f})
	}
}

func TestFilterExpiredAge(t *testing.T) {
	engine := NewEngine()
	engine.ExcludeExpired = true
	engine.MaxAgeDays = 30

	// Old finding
	oldTime := time.Now().Add(-60 * 24 * time.Hour).Format("2006-01-02T15:04:05Z07:00")
	f := models.Finding{
		SourceID:     "old",
		Title:        "Old API",
		URL:          "https://old.com",
		Description:  "A sufficiently long description that passes the minimum length filter check for testing",
		RawText:      "old content",
		DiscoveredAt: oldTime,
	}

	results := engine.FilterFindings([]models.Finding{f})
	if len(results) != 0 {
		t.Error("expected old finding to be filtered out")
	}
}

func TestFilterRequireURL(t *testing.T) {
	engine := NewEngine()
	engine.RequireURL = true

	f := models.Finding{
		SourceID:    "test",
		Title:       "No URL",
		URL:         "not-a-url",
		Description: "A sufficiently long description that passes the minimum length filter check for testing",
		RawText:     "content",
	}

	results := engine.FilterFindings([]models.Finding{f})
	if len(results) != 0 {
		t.Error("expected finding without valid URL to be filtered")
	}
}

func TestFilterScoreQualityEdgeCases(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name string
		f    models.Finding
	}{
		{
			name: "empty description",
			f: models.Finding{
				Description: "A sufficiently long description that passes the minimum length filter check for testing",
				RawText:     "",
				URL:         "https://example.com",
			},
		},
		{
			name: "all model keywords",
			f: models.Finding{
				Description: "A sufficiently long description that passes the minimum length filter check for testing",
				RawText:     "gpt claude gemini llama mistral mixtral qwen deepseek command gemma",
				URL:         "https://docs.api.com",
			},
		},
		{
			name: "all limit keywords",
			f: models.Finding{
				Description: "A sufficiently long description that passes the minimum length filter check for testing",
				RawText:     "rpm tpm rpd free tier credit limit quota",
				URL:         "https://api.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.f
			f.SourceID = "test"
			f.Title = "Test"
			input := []models.Finding{f}
			engine.FilterFindings(input)
			// Should not panic and score should be between 0 and 1
			if input[0].QualityScore < 0 || input[0].QualityScore > 1.0 {
				t.Errorf("QualityScore out of range: %f", input[0].QualityScore)
			}
		})
	}
}

// ─── Benchmarks ───

func BenchmarkFilterFindings(b *testing.B) {
	// Prepare test data
	findings := make([]models.Finding, 100)
	for i := range findings {
		findings[i] = models.Finding{
			SourceID:    "bench_" + string(rune('a'+i%26)),
			Title:       "Free API for testing",
			URL:         "https://example" + string(rune('a'+i%26)) + ".com",
			Description: "A sufficiently long description that passes the minimum length filter check for benchmarking",
			RawText:     "Free API access with GPT-4 and Claude models, 1000 RPM limit",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create fresh engine each time to avoid dedup skew
		e := NewEngine()
		e.FilterFindings(findings)
	}
}

func BenchmarkFilterSpamPattern(b *testing.B) {
	engine := NewEngine()
	texts := []string{
		"Normal text about free API access",
		"Купи сейчас со скидкой 50%!",
		"Free tier with 1000 RPM and no credit card",
		"affiliate link click here to buy",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range texts {
			engine.SpamPattern.MatchString(t)
		}
	}
}

func BenchmarkAssignPriority(b *testing.B) {
	providers := []*models.Provider{
		{Status: models.StatusVerified, CreditCard: false},
		{Status: models.StatusConfirmed, CreditCard: false},
		{Status: models.StatusClaimed, CreditCard: false},
		{Status: models.StatusUnverified, CreditCard: false},
		{Status: models.StatusVerified, CreditCard: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range providers {
			AssignPriority(p)
		}
	}
}
