package ocr

import (
	"testing"
)

func TestScoreOCRProvider(t *testing.T) {
	testResults := []*OCRTestResult{
		{Engine: 1, Language: "eng", Success: true, ProcessingMs: "312"},
		{Engine: 2, Language: "eng", Success: false, Error: "test image too small"},
		{Engine: 3, Language: "eng", Success: true, ProcessingMs: "987"},
	}

	score := ScoreOCRProvider("OCR.space", testResults, "25,000 requests/month")

	if score.ProviderName != "OCR.space" {
		t.Errorf("Expected ProviderName=OCR.space, got %s", score.ProviderName)
	}

	// Два из трёх движков успешны
	if score.EnginesCount != 2 {
		t.Errorf("Expected EnginesCount=2, got %d", score.EnginesCount)
	}

	// Speed score должен быть высоким (312ms и 987ms — среднее < 1000ms)
	if score.SpeedScore < 0.7 {
		t.Errorf("Expected SpeedScore >= 0.7, got %.2f", score.SpeedScore)
	}

	// Quality score = успешные / всего = 2/3
	if score.QualityScore < 0.6 || score.QualityScore > 0.7 {
		t.Errorf("Expected QualityScore ~0.67, got %.2f", score.QualityScore)
	}

	// Feature score для OCR.space должен быть 1.0
	if score.FeatureScore < 0.99 {
		t.Errorf("Expected FeatureScore~1.0 for OCR.space, got %.2f", score.FeatureScore)
	}

	// Value score с квотой 25,000 должен быть 1.0
	if score.ValueScore != 1.0 {
		t.Errorf("Expected ValueScore=1.0 for 25K quota, got %.2f", score.ValueScore)
	}

	// Overall score должен быть между 0 и 1
	if score.OverallScore < 0 || score.OverallScore > 1 {
		t.Errorf("OverallScore out of range: %.2f", score.OverallScore)
	}
}

func TestScoreOCRProviderNoResults(t *testing.T) {
	score := ScoreOCRProvider("OCR.space", nil, "25,000 requests/month")

	if score.SpeedScore != 0.0 {
		t.Errorf("Expected SpeedScore=0.0 for no results, got %.2f", score.SpeedScore)
	}

	if score.QualityScore != 0.0 {
		t.Errorf("Expected QualityScore=0.0 for no results, got %.2f", score.QualityScore)
	}
}

func TestCalculateSpeedScore(t *testing.T) {
	tests := []struct {
		name     string
		results  []*OCRTestResult
		expected float64
	}{
		{
			name:     "no results",
			results:  nil,
			expected: 0.0,
		},
		{
			name: "fast (< 500ms)",
			results: []*OCRTestResult{
				{Success: true, ProcessingMs: "300"},
			},
			expected: 1.0,
		},
		{
			name: "medium (500-1000ms)",
			results: []*OCRTestResult{
				{Success: true, ProcessingMs: "700"},
			},
			expected: 0.8,
		},
		{
			name: "slow (1000-2000ms)",
			results: []*OCRTestResult{
				{Success: true, ProcessingMs: "1500"},
			},
			expected: 0.5,
		},
		{
			name: "very slow (> 2000ms)",
			results: []*OCRTestResult{
				{Success: true, ProcessingMs: "2500"},
			},
			expected: 0.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateSpeedScore(tt.results)
			if score != tt.expected {
				t.Errorf("calculateSpeedScore() = %.1f, want %.1f", score, tt.expected)
			}
		})
	}
}

func TestCalculateValueScore(t *testing.T) {
	tests := []struct {
		quota    string
		expected float64
	}{
		{"", 0.0},
		{"25,000 requests/month", 1.0},
		{"10,000/month", 0.8},
		{"1,000/month", 0.5},
		{"free tier", 0.3},
		{"unknown", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.quota, func(t *testing.T) {
			score := calculateValueScore(tt.quota)
			if score != tt.expected {
				t.Errorf("calculateValueScore(%q) = %.1f, want %.1f", tt.quota, score, tt.expected)
			}
		})
	}
}

func TestCalculateFeatureScore(t *testing.T) {
	tests := []struct {
		provider string
		expected float64
	}{
		{"OCR.space", 0.99},
		{"a9t9 OCR", 0.99},
		{"unknown provider", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			score := calculateFeatureScore(tt.provider)
			if score < tt.expected {
				t.Errorf("calculateFeatureScore(%q) = %.2f, want >= %.2f", tt.provider, score, tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"312", 312},
		{"1500", 1500},
		{"0", 0},
		{"abc", 0},
		{"123abc", 123},
	}

	for _, tt := range tests {
		var result int
		err := parseInt(tt.input, &result)
		if err != nil {
			t.Errorf("parseInt(%q) returned error: %v", tt.input, err)
		}
		if result != tt.expected {
			t.Errorf("parseInt(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}
