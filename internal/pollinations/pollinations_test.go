package pollinations

import (
	"testing"
)

func TestIsPaidOnlyModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"claude-opus-4.6", true},
		{"gpt-5.4", true},
		{"flux", true},
		{"elevenlabs", true},
		{"openai", false},
		{"llama", false},
		{"deepseek", false},
		{"grok", false},
		{"mistral", false},
		{"qwen-coder", false},
		{"kimi", false},
		{"gemma", false},
		{"gemini-3-flash", false}, // может быть платной, но не в списке paid
		{"whisper", true},
		{"cohere-embed-v4", true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := isPaidOnlyModel(tt.model)
			if got != tt.expected {
				t.Errorf("isPaidOnlyModel(%q) = %v, want %v", tt.model, got, tt.expected)
			}
		})
	}
}

func TestIsNonTextModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"openai-audio", true},
		{"elevenmusic", true},
		{"whisper", true},
		{"flux", true},
		{"veo", true},
		{"cohere-embed-v4", true},
		{"openai", false},
		{"llama", false},
		{"deepseek", false},
		{"gpt-5.4-mini", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := isNonTextModel(tt.model)
			if got != tt.expected {
				t.Errorf("isNonTextModel(%q) = %v, want %v", tt.model, got, tt.expected)
			}
		})
	}
}

func TestToProvider(t *testing.T) {
	info := &ProviderInfo{
		Name:       "Pollinations",
		URL:        GenBaseURL,
		APIKeyURL:  GenBaseURL,
		CreditCard: false,
		Status:     "verified",
		Models:     []string{"openai", "llama", "deepseek"},
		Limits:     "3 free models",
		Notes:      "Test provider",
		VerifiedAt: "2026-06-26T06:00:00Z",
	}

	p := ToProvider(info)
	if p.Name != "Pollinations" {
		t.Errorf("Name = %q, want Pollinations", p.Name)
	}
	if p.CreditCard != false {
		t.Error("CreditCard should be false")
	}
	if len(p.Models) != 3 {
		t.Errorf("Models count = %d, want 3", len(p.Models))
	}
	if p.Status != "verified" {
		t.Errorf("Status = %q, want verified", p.Status)
	}
}

func TestModelTestResult(t *testing.T) {
	r := ModelTestResult{
		ModelID:      "openai",
		IsFree:       true,
		IsWorking:    true,
		ResponseTime: 150,
		ActualModel:  "gpt-5.4-nano",
	}
	if !r.IsFree {
		t.Error("IsFree should be true")
	}
	if !r.IsWorking {
		t.Error("IsWorking should be true")
	}
}
