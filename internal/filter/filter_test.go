package filter

import (
	"testing"

	"free-api-hunter/internal/models"
)

func TestFilterDedup(t *testing.T) {
	engine := NewEngine()

	f1 := models.Finding{
		SourceID:    "test1",
		Title:       "Test",
		URL:         "https://example.com",
		Description: "A sufficiently long description that passes the minimum length filter check",
		RawText:     "test content",
	}
	f2 := models.Finding{
		SourceID:    "test2",
		Title:       "Test",
		URL:         "https://example.com",
		Description: "B sufficiently long description that passes the minimum length filter check",
		RawText:     "test content",
	}

	results := engine.FilterFindings([]models.Finding{f1, f2})
	if len(results) != 1 {
		t.Errorf("Expected 1 result after dedup, got %d", len(results))
	}
}

func TestFilterSpam(t *testing.T) {
	engine := NewEngine()

	f := models.Finding{
		SourceID:    "test",
		Title:       "Купи сейчас со скидкой!",
		URL:         "https://example.com",
		Description: "This is a sufficiently long description that would normally pass the filter but contains spam keywords that should trigger the spam filter pattern",
		RawText:     "Купи сейчас со скидкой! Специальное предложение!",
	}

	results := engine.FilterFindings([]models.Finding{f})
	if len(results) != 0 {
		t.Errorf("Expected 0 results after spam filter, got %d", len(results))
	}
}

func TestFilterTooShort(t *testing.T) {
	engine := NewEngine()

	f := models.Finding{
		SourceID:    "test",
		Title:       "Short",
		URL:         "https://example.com",
		Description: "Too short",
		RawText:     "short",
	}

	results := engine.FilterFindings([]models.Finding{f})
	if len(results) != 0 {
		t.Errorf("Expected 0 results for short description, got %d", len(results))
	}
}

func TestFilterExcludedDomain(t *testing.T) {
	engine := NewEngine()

	f := models.Finding{
		SourceID:    "test",
		Title:       "Medium article",
		URL:         "https://medium.com/some-article",
		Description: "A sufficiently long description that passes the minimum length filter check for testing",
		RawText:     "test content",
	}

	results := engine.FilterFindings([]models.Finding{f})
	if len(results) != 0 {
		t.Errorf("Expected 0 results for excluded domain, got %d", len(results))
	}
}

func TestFilterQualityScore(t *testing.T) {
	engine := NewEngine()

	highQuality := models.Finding{
		SourceID:    "test",
		Title:       "Free API key for GPT-4 with 1000 RPM",
		URL:         "https://api.example.com/docs",
		Description: "Get free access to GPT-4 model with 1000 requests per minute limit. No credit card required for basic tier.",
		RawText:     "Free API key for GPT-4 with 1000 RPM",
	}

	lowQuality := models.Finding{
		SourceID:    "test2",
		Title:       "Something",
		URL:         "https://example.com",
		Description: "A sufficiently long description that passes the minimum length filter check for testing purposes here",
		RawText:     "short",
	}

	// Pass slice so engine modifies the elements
	input := []models.Finding{highQuality, lowQuality}
	results := engine.FilterFindings(input)

	// Both should pass filter
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// High quality should have higher score
	if input[0].QualityScore <= input[1].QualityScore {
		t.Errorf("High quality score (%.2f) should be > low quality score (%.2f)",
			input[0].QualityScore, input[1].QualityScore)
	}
}

func TestAssignPriority(t *testing.T) {
	tests := []struct {
		name     string
		provider models.Provider
		want     models.Priority
	}{
		{
			name:     "verified no card",
			provider: models.Provider{Status: models.StatusVerified, CreditCard: false},
			want:     models.PriorityHigh,
		},
		{
			name:     "confirmed no card",
			provider: models.Provider{Status: models.StatusConfirmed, CreditCard: false},
			want:     models.PriorityHigh,
		},
		{
			name:     "claimed no card",
			provider: models.Provider{Status: models.StatusClaimed, CreditCard: false},
			want:     models.PriorityMed,
		},
		{
			name:     "unverified",
			provider: models.Provider{Status: models.StatusUnverified, CreditCard: false},
			want:     models.PriorityLow,
		},
		{
			name:     "deprioritized",
			provider: models.Provider{Status: models.StatusDeprioritized, CreditCard: false},
			want:     models.PriorityLow,
		},
		{
			name:     "credit card required",
			provider: models.Provider{Status: models.StatusConfirmed, CreditCard: true},
			want:     models.PrioritySkip,
		},
		{
			name:     "verified but credit card",
			provider: models.Provider{Status: models.StatusVerified, CreditCard: true},
			want:     models.PrioritySkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssignPriority(&tt.provider)
			if got != tt.want {
				t.Errorf("AssignPriority: got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFilterMultipleUnique(t *testing.T) {
	engine := NewEngine()

	findings := []models.Finding{
		{SourceID: "1", Title: "A", URL: "https://a.com", Description: "Description A that is long enough to pass the filter", RawText: "text A"},
		{SourceID: "2", Title: "B", URL: "https://b.com", Description: "Description B that is long enough to pass the filter", RawText: "text B"},
		{SourceID: "3", Title: "C", URL: "https://c.com", Description: "Description C that is long enough to pass the filter", RawText: "text C"},
	}

	results := engine.FilterFindings(findings)
	if len(results) != 3 {
		t.Errorf("Expected 3 unique results, got %d", len(results))
	}
}
