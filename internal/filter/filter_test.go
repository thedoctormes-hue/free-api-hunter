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

func TestFilterExcludedProvider(t *testing.T) {
	engine := NewEngine()

	kilo := "Kilo Gateway"
	f := models.Finding{
		SourceID:    "test",
		Title:       "Kilo Gateway free API",
		URL:         "https://kilo.example.com",
		Description: "A sufficiently long description that passes the minimum length filter check for testing",
		RawText:     "Kilo Gateway offers free API access",
		ProviderName: &kilo,
	}

	results := engine.FilterFindings([]models.Finding{f})
	if len(results) != 0 {
		t.Errorf("Expected 0 results for excluded provider, got %d", len(results))
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

func TestFilterExcludeKeywords(t *testing.T) {
	engine := NewEngine()
	engine.ExcludeKeywords = []string{"скидка", "affiliate"}

	spamFinding := models.Finding{
		SourceID:    "test",
		Title:       "Get скидка on API",
		URL:         "https://example.com",
		Description: "A sufficiently long description that passes the minimum length filter check for testing",
		RawText:     "Скидка на подписку! Успейте купить!",
	}

	cleanFinding := models.Finding{
		SourceID:    "test2",
		Title:       "Free API access",
		URL:         "https://example2.com",
		Description: "A sufficiently long description that passes the minimum length filter check for testing",
		RawText:     "Free tier available",
	}

	results := engine.FilterFindings([]models.Finding{spamFinding, cleanFinding})
	if len(results) != 1 {
		t.Errorf("Expected 1 result after keyword filter, got %d", len(results))
	}
}

func TestFilterTrashSources(t *testing.T) {
	engine := NewEngine()
	engine.ExcludeTrashSources = map[string]bool{"pastebin.com": true}

	trashFinding := models.Finding{
		SourceID:    "test",
		Title:       "Pastebin API",
		URL:         "https://pastebin.com/abc123",
		Description: "A sufficiently long description that passes the minimum length filter check for testing",
		RawText:     "Free API keys on pastebin",
	}

	results := engine.FilterFindings([]models.Finding{trashFinding})
	if len(results) != 0 {
		t.Errorf("Expected 0 results for trash source, got %d", len(results))
	}
}

func TestFilterURLUniqueness(t *testing.T) {
	engine := NewEngine()
	engine.seenURLs = make(map[string]bool)

	f1 := models.Finding{
		SourceID:    "test1",
		Title:       "Same URL",
		URL:         "https://example.com/api",
		Description: "First finding with this URL that is long enough to pass",
		RawText:     "content",
	}
	f2 := models.Finding{
		SourceID:    "test2",
		Title:       "Same URL duplicate",
		URL:         "https://example.com/api",
		Description: "Second finding with same URL that is long enough to pass",
		RawText:     "more content",
	}

	results := engine.FilterFindings([]models.Finding{f1, f2})
	if len(results) != 1 {
		t.Errorf("Expected 1 result after URL dedup, got %d", len(results))
	}
}

func TestApplyConfig(t *testing.T) {
	engine := NewEngine()

	cfg := FilterConfigData{
		ExcludedProviders: []string{"Bad Provider"},
		SpamDomains:       []string{"facebook.com"},
		SpamKeywords:      []string{"купить", "referral"},
		TrashSources:      []string{"pastebin.com"},
		MinDescLength:     50,
		RequireURL:        true,
		ExcludeExpired:    true,
		MaxAgeDays:        30,
		CheckURLUnique:    true,
	}

	engine.ApplyConfig(cfg)

	if !engine.ExcludedProviders["bad provider"] {
		t.Error("Expected 'bad provider' in ExcludedProviders")
	}
	if !engine.ExcludeDomains["facebook.com"] {
		t.Error("Expected facebook.com in ExcludeDomains")
	}
	if engine.MinDescLength != 50 {
		t.Errorf("Expected MinDescLength 50, got %d", engine.MinDescLength)
	}
	if !engine.RequireURL {
		t.Error("Expected RequireURL true")
	}
	if !engine.ExcludeExpired {
		t.Error("Expected ExcludeExpired true")
	}
	if engine.MaxAgeDays != 30 {
		t.Errorf("Expected MaxAgeDays 30, got %d", engine.MaxAgeDays)
	}
	if engine.seenURLs == nil {
		t.Error("Expected seenURLs initialized")
	}
	if len(engine.ExcludeKeywords) != 2 {
		t.Errorf("Expected 2 exclude keywords, got %d", len(engine.ExcludeKeywords))
	}
}
