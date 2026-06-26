package tts

import (
	"testing"

	"free-api-hunter/internal/models"
)

func TestScoreTTSProvider_FreeTier(t *testing.T) {
	provider := &models.TTSProvider{
		Name: "ElevenLabs",
		FreeTier: &models.FreeTierInfo{
			CharLimit:   10000,
			VoiceClones: 3,
			ResetPeriod: "monthly",
		},
		Features:  []string{"audio_tags", "voice_cloning", "multilingual"},
		Languages: []string{"en", "ru", "70+"},
		Models:    []string{"eleven_v3", "eleven_flash_v2_5"},
	}

	score := ScoreTTSProvider(provider, true)

	if score.OverallScore <= 0 || score.OverallScore > 1 {
		t.Errorf("OverallScore out of range: %f", score.OverallScore)
	}

	if !score.HasFreeTier {
		t.Error("expected HasFreeTier=true")
	}

	if score.CharLimit != 10000 {
		t.Errorf("expected CharLimit=10000, got %d", score.CharLimit)
	}

	// Скоринг должен быть выше 0.5 для ElevenLabs с ключом
	if score.OverallScore < 0.5 {
		t.Errorf("expected OverallScore >= 0.5 for active ElevenLabs, got %f", score.OverallScore)
	}
}

func TestScoreTTSProvider_NoKey(t *testing.T) {
	provider := &models.TTSProvider{
		Name: "TestProvider",
		FreeTier: &models.FreeTierInfo{
			CharLimit: 1000,
		},
		Features:  []string{"multilingual"},
		Languages: []string{"en"},
		Models:    []string{"basic"},
	}

	score := ScoreTTSProvider(provider, false)

	// Без ключа скор должен быть ниже
	if score.FreeTierScore > 0.5 {
		t.Errorf("expected FreeTierScore <= 0.5 without key, got %f", score.FreeTierScore)
	}
}

func TestScoreTTSProvider_NoFreeTier(t *testing.T) {
	provider := &models.TTSProvider{
		Name:      "PaidOnly",
		FreeTier:  nil,
		Features:  []string{},
		Languages: []string{"en"},
		Models:    []string{"v1"},
	}

	score := ScoreTTSProvider(provider, true)

	if score.HasFreeTier {
		t.Error("expected HasFreeTier=false")
	}

	if score.FreeTierScore != 0.1 {
		t.Errorf("expected FreeTierScore=0.1 for no free tier, got %f", score.FreeTierScore)
	}
}

func TestScoreTTSProvider_FullFeatures(t *testing.T) {
	provider := &models.TTSProvider{
		Name: "FullFeatured",
		FreeTier: &models.FreeTierInfo{
			CharLimit:   50000,
			VoiceClones: 10,
			ResetPeriod: "monthly",
		},
		Features: []string{
			"audio_tags",
			"voice_cloning",
			"pronunciation_dicts",
			"multi_speaker",
			"realtime",
			"multilingual",
			"streaming",
		},
		Languages: []string{"en", "ru", "de", "fr", "es", "ja", "ko", "zh", "ar", "hi",
			"pt", "it", "nl", "pl", "tr", "uk", "sv", "da", "no", "fi"},
		Models: []string{"realtime", "flash", "v3"},
	}

	score := ScoreTTSProvider(provider, true)

	// С полным набором фич и ключом скор должен быть высоким
	if score.OverallScore < 0.8 {
		t.Errorf("expected OverallScore >= 0.8 for full-featured provider, got %f", score.OverallScore)
	}

	if score.FeatureScore < 0.8 {
		t.Errorf("expected FeatureScore >= 0.8, got %f", score.FeatureScore)
	}
}

func TestScoreCharLimit(t *testing.T) {
	tests := []struct {
		chars    int
		expected float64
	}{
		{0, 0.0},
		{500, 0.3},
		{5000, 0.5},
		{10000, 0.8},
		{50000, 1.0},
		{100000, 1.0},
	}

	for _, tt := range tests {
		got := scoreCharLimit(tt.chars)
		if got != tt.expected {
			t.Errorf("scoreCharLimit(%d) = %f, want %f", tt.chars, got, tt.expected)
		}
	}
}

func TestCalculateLanguageScore(t *testing.T) {
	tests := []struct {
		name     string
		langs    []string
		expected float64
	}{
		{"Russian+20", []string{"ru", "en", "de", "fr", "es", "it", "pt", "nl", "pl", "tr",
			"uk", "sv", "da", "no", "fi", "cs", "sk", "hu", "ro", "bg"}, 1.0},
		{"Russian only", []string{"ru"}, 0.6},
		{"No Russian, 5 langs", []string{"en", "de", "fr", "es", "it"}, 0.3},
		{"Empty", nil, 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &models.TTSProvider{Languages: tt.langs}
			got := calculateLanguageScore(provider)
			if got != tt.expected {
				t.Errorf("calculateLanguageScore(%v) = %f, want %f", tt.langs, got, tt.expected)
			}
		})
	}
}
