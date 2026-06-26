package pollinations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"free-api-hunter/internal/models"
)

func TestIsPaidOnlyModelEdgeCases(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		// Prefix matches
		{"claude-opus-4.6", true},
		{"claude-large-v2", true},
		{"perplexity-sonar", true},
		{"gemini-search-pro", true},
		{"flux-redux", true},
		{"kontext-dev", true},
		{"seedream-3", true},
		{"sana-2", true},
		{"gptimage-v2", true},
		{"veo-2", true},
		{"seedance-1", true},
		{"wan-2", true},
		{"elevenlabs-v2", true},
		{"elevenflash-v1", true},
		{"elevenmusic-v1", true},
		{"whisper-v3", true},
		{"scribe-v1", true},
		{"universal-v1", true},
		{"nova-canvas-v1", true},
		{"nova-reel-v1", true},
		{"grok-imagine-v1", true},
		{"grok-video-v1", true},
		{"klein-v1", true},
		{"ltx-13b", true},
		{"p-image-v1", true},
		{"p-video-v1", true},
		{"acestep-v1", true},
		{"stable-audio-v1", true},
		{"qwen-tts-v1", true},
		{"openai-3-large", true},
		// Free models
		{"openai", false},
		{"llama", false},
		{"deepseek", false},
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

func TestIsNonTextModelEdgeCases(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"cohere-embed-v4", true},
		{"cohere-embed-v3", true},
		{"realtime-openai", true},
		// Free text
		{"mistral", false},
		{"gemma", false},
		{"phi-3", false},
		{"command-r", false},
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

func TestToProviderEmptyModels(t *testing.T) {
	info := &ProviderInfo{
		Name:       "TestProv",
		URL:        "https://test.com",
		APIKeyURL:  "https://test.com/keys",
		CreditCard: false,
		Status:     "verified",
		Models:     nil,
		VerifiedAt: "2026-06-26T00:00:00Z",
	}
	p := ToProvider(info)
	// When ModelsFree is empty and Models is nil, Models should be nil
	if len(p.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(p.Models))
	}
	if p.Name != "TestProv" {
		t.Errorf("name = %q, want TestProv", p.Name)
	}
}

func TestToProviderFallbackModels(t *testing.T) {
	info := &ProviderInfo{
		Name:        "TestProv",
		URL:         "https://test.com",
		APIKeyURL:   "https://test.com/keys",
		CreditCard:  false,
		Status:      "verified",
		Models:      []string{"m1", "m2"},
		ModelsFree:  nil,
		VerifiedAt: "2026-06-26T00:00:00Z",
	}
	p := ToProvider(info)
	// When ModelsFree is empty, should fallback to Models
	if len(p.Models) != 2 {
		t.Errorf("expected 2 fallback models, got %d", len(p.Models))
	}
}

func TestToProviderSource(t *testing.T) {
	info := &ProviderInfo{
		Name:       "Pollinations",
		URL:        GenBaseURL,
		APIKeyURL:  GenBaseURL,
		CreditCard: false,
		Status:     "verified",
		VerifiedAt: "2026-06-26T00:00:00Z",
	}
	p := ToProvider(info)
	if p.Source != "raven" {
		t.Errorf("source = %q, want raven", p.Source)
	}
	if p.DiscoveredAt != info.VerifiedAt {
		t.Errorf("discovered_at = %q, want %q", p.DiscoveredAt, info.VerifiedAt)
	}
	if p.LastVerified == nil || *p.LastVerified != info.VerifiedAt {
		t.Error("last_verified should be set")
	}
}

func TestModelTestResultError(t *testing.T) {
	r := ModelTestResult{
		ModelID:   "bad-model",
		IsFree:    false,
		IsWorking: false,
		Error:     "model_not_found",
	}
	if r.IsFree {
		t.Error("should not be free")
	}
	if r.Error != "model_not_found" {
		t.Errorf("error = %q", r.Error)
	}
}

func TestVerifyImageGenerationMockServer(t *testing.T) {
	// Mock server returning a valid response with b64 data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := ImageResponse{
			Created: 1234567890,
			Data: []struct {
				URL           string `json:"url,omitempty"`
				B64JSON       string `json:"b64_json,omitempty"`
				RevisedPrompt string `json:"revised_prompt,omitempty"`
			}{
				{B64JSON: "aGVsbG8gd29ybGQ="},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// We can't easily override the client, but we can test the parsing logic
	// by simulating the response handling
	body := `{"created":1234567890,"data":[{"b64_json":"aGVsbG8gd29ybGQ="}]}`
	var imgResp ImageResponse
	if err := json.Unmarshal([]byte(body), &imgResp); err != nil {
		t.Fatal(err)
	}
	if len(imgResp.Data) == 0 {
		t.Error("expected data")
	}
	hasImage := imgResp.Data[0].URL != "" || imgResp.Data[0].B64JSON != ""
	if !hasImage {
		t.Error("should have image")
	}
}

func TestVerifyImageGenerationNoImages(t *testing.T) {
	body := `{"created":1234567890,"data":[]}`
	var imgResp ImageResponse
	json.Unmarshal([]byte(body), &imgResp)
	if len(imgResp.Data) != 0 {
		t.Error("expected no images")
	}
}

func TestChatResponseError(t *testing.T) {
	body := `{"error":{"message":"Insufficient balance","code":"payment_required"}}`
	var chatResp ChatResponse
	if err := json.Unmarshal([]byte(body), &chatResp); err != nil {
		t.Fatal(err)
	}
	if chatResp.Error == nil {
		t.Fatal("expected error in response")
	}
	if !strings.Contains(chatResp.Error.Message, "balance") {
		t.Errorf("error message = %q", chatResp.Error.Message)
	}
}

func TestModelsResponseEmpty(t *testing.T) {
	body := `{"object":"list","data":[]}`
	var resp ModelsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 0 {
		t.Error("expected empty data")
	}
}

func TestModelsResponseWithModels(t *testing.T) {
	body := `{"object":"list","data":[{"id":"openai","name":"OpenAI","tier":"free"}]}`
	var resp ModelsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 model, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "openai" {
		t.Errorf("model id = %q", resp.Data[0].ID)
	}
}

func TestProviderInfoJSON(t *testing.T) {
	info := &ProviderInfo{
		Name:       "Pollinations",
		URL:        GenBaseURL,
		APIKeyURL:  GenBaseURL,
		CreditCard: false,
		Status:     "verified",
		Models:     []string{"openai", "llama"},
		VerifiedAt: "2026-06-26T00:00:00Z",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ProviderInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Name != info.Name {
		t.Errorf("name = %q, want %q", decoded.Name, info.Name)
	}
	if len(decoded.Models) != 2 {
		t.Errorf("models count = %d, want 2", len(decoded.Models))
	}
}

func TestChatRequestJSON(t *testing.T) {
	req := ChatRequest{
		Model: "openai",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 10,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\"model\":\"openai\"") {
		t.Errorf("JSON missing model: %s", string(data))
	}
}

func TestImageResponseWithURL(t *testing.T) {
	body := `{"created":1234567890,"data":[{"url":"https://example.com/img.png"}]}`
	var resp ImageResponse
	json.Unmarshal([]byte(body), &resp)
	if len(resp.Data) != 1 || resp.Data[0].URL == "" {
		t.Error("expected URL in response")
	}
}

func TestPollinationsModelFields(t *testing.T) {
	m := PollinationsModel{
		ID:        "test",
		Name:      "Test",
		Reasoning: true,
		Tier:      "free",
		Vision:    true,
		Tools:     true,
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var decoded PollinationsModel
	json.Unmarshal(data, &decoded)
	if !decoded.Reasoning {
		t.Error("Reasoning should be true")
	}
	if decoded.Tier != "free" {
		t.Errorf("Tier = %q", decoded.Tier)
	}
}

func TestModelTestResultAllFields(t *testing.T) {
	r := ModelTestResult{
		ModelID:      "test-model",
		ModelAlias:   "test-alias",
		IsFree:       true,
		IsWorking:    true,
		ResponseTime: 250,
		ActualModel:  "actual-model-v2",
		SampleOutput: "Hi!",
	}
	if !r.IsFree || !r.IsWorking {
		t.Error("should be free and working")
	}
	if r.ResponseTime != 250 {
		t.Errorf("response time = %d", r.ResponseTime)
	}
	_ = models.ProviderStatus("verified") // ensure models import is used
}

func TestModelsResponseLegacyFormat(t *testing.T) {
	// Legacy format is a plain array
	body := `[{"id":"old-model","name":"Old Model"}]`
	var legacy []PollinationsModel
	if err := json.Unmarshal([]byte(body), &legacy); err != nil {
		t.Fatal(err)
	}
	if len(legacy) != 1 || legacy[0].ID != "old-model" {
		t.Error("legacy parse failed")
	}
}
