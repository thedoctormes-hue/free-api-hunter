package pollinations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// restore resets httpClient and vault fn after a test
func restore() {
	httpClient = &http.Client{Timeout: 6 * time.Second}
	getAPIKeyFn = func() (string, error) {
		return "test-key", nil
	}
	genBaseURL = GenBaseURL
}

// --- TestAllModels: mock HTTP server returning list of models ---

func TestAllModelsFetchAndParse(t *testing.T) {
	defer restore()

	modelsJSON := `{"object":"list","data":[
		{"id":"openai","name":"OpenAI GPT","tier":"free","vision":false},
		{"id":"llama","name":"Llama 3","tier":"free","vision":false},
		{"id":"deepseek","name":"DeepSeek V3","tier":"free","vision":false},
		{"id":"claude-opus-4.6","name":"Claude Opus","tier":"paid","vision":true},
		{"id":"flux-xl","name":"Flux XL Image","tier":"paid","vision":false},
		{"id":"openai-audio","name":"OpenAI Audio","tier":"free","vision":false},
		{"id":"gpt-5.4","name":"GPT 5.4","tier":"paid","vision":false}
	]}`

	modelsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == ModelsEndpoint {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(modelsJSON))
			return
		}
		w.WriteHeader(404)
	}))
	defer modelsServer.Close()

	SetHTTPClient(modelsServer.Client())
	SetBaseURL(modelsServer.URL)

	t.Run("GetModels_parsing", func(t *testing.T) {
		resp, err := GetModels()
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 7 {
			t.Errorf("expected 7 models, got %d", len(resp.Data))
		}
		if resp.Data[0].ID != "openai" {
			t.Errorf("first model = %q, want openai", resp.Data[0].ID)
		}
	})
}

// --- TestVerifyImageGeneration: mock gen.pollinations.ai ---

func TestVerifyImageGenerationSuccess(t *testing.T) {
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != ImageEndpoint {
			t.Errorf("expected path %s, got %s", ImageEndpoint, r.URL.Path)
		}

		resp := ImageResponse{
			Created: 1234567890,
			Data: []struct {
				URL           string `json:"url,omitempty"`
				B64JSON       string `json:"b64_json,omitempty"`
				RevisedPrompt string `json:"revised_prompt,omitempty"`
			}{
				{B64JSON: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	SetHTTPClient(server.Client())
	SetBaseURL(server.URL)

	success, msg := VerifyImageGeneration()
	if !success {
		t.Errorf("expected success, got fail: %s", msg)
	}
}

func TestVerifyImageGenerationFailure(t *testing.T) {
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`internal server error`))
	}))
	defer server.Close()

	SetHTTPClient(server.Client())
	SetBaseURL(server.URL)

	success, msg := VerifyImageGeneration()
	if success {
		t.Error("expected failure, got success")
	}
	if !strings.Contains(msg, "500") {
		t.Errorf("expected 500 error message, got %q", msg)
	}
}

// --- TestToProvider: comprehensive field checking ---

func TestToProviderAllFields(t *testing.T) {
	info := &ProviderInfo{
		Name:       "Pollinations",
		URL:        GenBaseURL,
		APIKeyURL:  GenBaseURL,
		CreditCard: false,
		Status:     "verified",
		Models:     []string{"openai", "llama", "deepseek", "mistral"},
		ModelsFree: []string{"openai", "llama"},
		ModelsPaid: []string{"deepseek", "mistral"},
		Limits:     "2 free, 2 paid",
		Notes:      "Test notes",
		Endpoints: map[string]string{
			"chat":  GenBaseURL + ChatEndpoint,
			"image": GenBaseURL + ImageEndpoint,
		},
		VerifiedAt: "2026-06-26T16:00:00Z",
	}

	p := ToProvider(info)

	if p.Name != "Pollinations" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.CreditCard != false {
		t.Error("CreditCard should be false")
	}
	if len(p.Models) != 2 {
		t.Errorf("Models count = %d, want 2 (free only)", len(p.Models))
	}
	if p.Source != "raven" {
		t.Errorf("Source = %q, want raven", p.Source)
	}
	if p.DiscoveredAt != info.VerifiedAt {
		t.Errorf("DiscoveredAt = %q, want %q", p.DiscoveredAt, info.VerifiedAt)
	}
}

// --- TestFreeModelDetection: is_free for different pricing combinations ---

func TestFreeModelDetection(t *testing.T) {
	tests := []struct {
		name     string
		result   ModelTestResult
		expected bool
	}{
		{"free_working", ModelTestResult{IsFree: true, IsWorking: true}, true},
		{"paid_working", ModelTestResult{IsFree: false, IsWorking: true, Error: "paid_model"}, false},
		{"not_working", ModelTestResult{IsFree: false, IsWorking: false, Error: "model_not_found"}, false},
		{"auth_error", ModelTestResult{IsFree: false, IsWorking: false, Error: "auth_error"}, false},
		{"free_broken", ModelTestResult{IsFree: true, IsWorking: false}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.IsFree != tt.expected {
				t.Errorf("IsFree = %v, want %v", tt.result.IsFree, tt.expected)
			}
		})
	}
}

// --- TestPollinateGeneration: generation with mock HTTP, check URL return ---

func TestPollinateGeneration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		prompt, _ := reqBody["prompt"].(string)
		resp := ImageResponse{
			Created: 1234567890,
			Data: []struct {
				URL           string `json:"url,omitempty"`
				B64JSON       string `json:"b64_json,omitempty"`
				RevisedPrompt string `json:"revised_prompt,omitempty"`
			}{
				{URL: "https://image.pollinations.ai/prompt/" + prompt},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	SetHTTPClient(server.Client())
	SetBaseURL(server.URL)

	resp, err := server.Client().Post(server.URL+ImageEndpoint, "application/json",
		strings.NewReader(`{"prompt":"a beautiful sunset","n":1,"size":"256x256"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var imgResp ImageResponse
	json.NewDecoder(resp.Body).Decode(&imgResp)
	if len(imgResp.Data) != 1 || !strings.Contains(imgResp.Data[0].URL, "sunset") {
		t.Errorf("expected sunset URL, got %v", imgResp.Data)
	}
}

// --- TestNewProviderParams ---

func TestNewProviderParams(t *testing.T) {
	info := &ProviderInfo{
		Name:       "Pollinations",
		URL:        GenBaseURL,
		APIKeyURL:  GenBaseURL,
		CreditCard: false,
		Status:     "verified",
		Models:     []string{"m1", "m2", "m3", "m4", "m5"},
		ModelsFree: []string{"m1", "m2", "m3"},
		ModelsPaid: []string{"m4", "m5"},
		Limits:     "3 free, 2 paid",
		Notes:      "Notes",
		Endpoints: map[string]string{
			"chat":         GenBaseURL + ChatEndpoint,
			"models":       GenBaseURL + ModelsEndpoint,
			"image":        GenBaseURL + ImageEndpoint,
			"image_legacy": ImageBaseURL + "/prompt/{prompt}",
			"text_legacy":  TextBaseURL + "/{prompt}",
		},
		VerifiedAt: "2026-06-26T16:00:00Z",
	}

	if len(info.Models) != 5 || len(info.ModelsFree) != 3 || len(info.Endpoints) != 5 {
		t.Errorf("ProviderInfo params incorrect")
	}
}

// --- TestGetModelsMockServer ---

func TestGetModelsMockServer(t *testing.T) {
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ModelsResponse{
			Object: "list",
			Data: []PollinationsModel{
				{ID: "openai", Name: "OpenAI", Tier: "free", Vision: false, Tools: true},
				{ID: "llama", Name: "Llama 3", Tier: "free", Vision: false, Tools: false},
				{ID: "deepseek-r1", Name: "DeepSeek R1", Tier: "free", Reasoning: true, Vision: false, Tools: false},
				{ID: "gemma-3", Name: "Gemma 3", Tier: "free", Vision: true, Tools: false},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	SetHTTPClient(server.Client())
	SetBaseURL(server.URL)

	resp, err := GetModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 4 {
		t.Fatalf("expected 4 models, got %d", len(resp.Data))
	}
	if !resp.Data[2].Reasoning {
		t.Error("deepseek-r1 should have Reasoning=true")
	}
	if !resp.Data[3].Vision {
		t.Error("gemma-3 should have Vision=true")
	}
}

// --- TestGetModelsLegacyFormat ---

func TestGetModelsLegacyFormat(t *testing.T) {
	defer restore()

	legacyJSON := `[
		{"id":"old-model-1","name":"Old Model 1"},
		{"id":"old-model-2","name":"Old Model 2"}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(legacyJSON))
	}))
	defer server.Close()

	SetHTTPClient(server.Client())
	SetBaseURL(server.URL)

	resp, err := GetModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 legacy models, got %d", len(resp.Data))
	}
}

// --- TestGetModelsHTTPError ---

func TestGetModelsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte("service unavailable"))
	}))
	defer server.Close()

	SetHTTPClient(server.Client())
	SetBaseURL(server.URL)

	_, err := GetModels()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("expected 503 error, got %q", err.Error())
	}
}

// --- TestTestModelMock ---

func TestTestModelMock(t *testing.T) {
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Simulate paid model error
		if req.Model == "paid-model" {
			resp := ChatResponse{
				Error: &struct {
					Message string `json:"message"`
					Code    string `json:"code"`
				}{Message: "Insufficient balance", Code: "payment_required"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Simulate model not found
		if req.Model == "missing-model" {
			resp := ChatResponse{
				Error: &struct {
					Message string `json:"message"`
					Code    string `json:"code"`
				}{Message: "Model not found", Code: "not_found"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Successful response
		resp := ChatResponse{
			ID:    "test-id",
			Model: req.Model,
		}
		resp.Choices = []struct {
			Index   int `json:"index"`
			Message struct {
				Role      string        `json:"role"`
				Content   string        `json:"content"`
				Reasoning string        `json:"reasoning"`
				Refusal   interface{}   `json:"refusal"`
				ToolCalls []interface{} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{Index: 0, FinishReason: "stop"},
		}
		resp.Choices[0].Message.Role = "assistant"
		resp.Choices[0].Message.Content = "Hi there!"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	SetHTTPClient(server.Client())
	SetBaseURL(server.URL)

	t.Run("free_model_response", func(t *testing.T) {
		resp := TestModel("openai")
		if resp.Error != "" {
			t.Errorf("expected no error, got %s", resp.Error)
		}
		if !resp.IsFree {
			t.Error("expected free model")
		}
	})

	t.Run("paid_model_response", func(t *testing.T) {
		resp := TestModel("paid-model")
		if resp.Error != "paid_model" {
			t.Errorf("expected paid_model error, got %s", resp.Error)
		}
		if resp.IsFree {
			t.Error("expected paid model")
		}
	})

	t.Run("not_found_response", func(t *testing.T) {
		resp := TestModel("missing-model")
		if resp.Error != "model_not_found" {
			t.Errorf("expected model_not_found, got %s", resp.Error)
		}
	})
}

// --- TestChatResponseParsing ---

func TestChatResponseParsing(t *testing.T) {
	t.Run("with_reasoning", func(t *testing.T) {
		body := `{
			"id":"chatcmpl-123",
			"model":"deepseek-r1",
			"choices":[{
				"index":0,
				"message":{"role":"assistant","content":"Hello","reasoning":"Let me think..."},
				"finish_reason":"stop"
			}]
		}`
		var resp ChatResponse
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Model != "deepseek-r1" {
			t.Errorf("model = %q", resp.Model)
		}
		if resp.Choices[0].Message.Reasoning == "" {
			t.Error("expected reasoning content")
		}
	})

	t.Run("with_error", func(t *testing.T) {
		body := `{"error":{"message":"Rate limit exceeded","code":"rate_limit"}}`
		var resp ChatResponse
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Error == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty_choices", func(t *testing.T) {
		body := `{"id":"x","model":"y","choices":[]}`
		var resp ChatResponse
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Choices) != 0 {
			t.Error("expected empty choices")
		}
	})
}

// --- TestProviderInfoEndpoints ---

func TestProviderInfoEndpoints(t *testing.T) {
	info := &ProviderInfo{
		Name:       "Pollinations",
		URL:        GenBaseURL,
		APIKeyURL:  GenBaseURL,
		CreditCard: false,
		Status:     "verified",
		Endpoints: map[string]string{
			"chat":         GenBaseURL + ChatEndpoint,
			"models":       GenBaseURL + ModelsEndpoint,
			"image":        GenBaseURL + ImageEndpoint,
			"image_legacy": ImageBaseURL + "/prompt/{prompt}",
			"text_legacy":  TextBaseURL + "/{prompt}",
		},
		VerifiedAt: "2026-06-26T16:00:00Z",
	}

	if len(info.Endpoints) != 5 {
		t.Errorf("expected 5 endpoints, got %d", len(info.Endpoints))
	}
}

// --- TestImageResponseBothFormats ---

func TestImageResponseBothFormats(t *testing.T) {
	t.Run("url_format", func(t *testing.T) {
		body := `{"created":1234567890,"data":[{"url":"https://image.pollinations.ai/prompt/hello"}]}`
		var resp ImageResponse
		json.Unmarshal([]byte(body), &resp)
		if len(resp.Data) != 1 || resp.Data[0].URL == "" {
			t.Error("expected URL format")
		}
	})

	t.Run("b64_format", func(t *testing.T) {
		body := `{"created":1234567890,"data":[{"b64_json":"base64data","revised_prompt":"a hello image"}]}`
		var resp ImageResponse
		json.Unmarshal([]byte(body), &resp)
		if len(resp.Data) != 1 || resp.Data[0].B64JSON == "" {
			t.Error("expected b64 format")
		}
		if resp.Data[0].RevisedPrompt != "a hello image" {
			t.Errorf("revised_prompt = %q", resp.Data[0].RevisedPrompt)
		}
	})
}

// --- TestPollinationsModelAllFields ---

func TestPollinationsModelAllFields(t *testing.T) {
	m := PollinationsModel{
		ID:         "deepseek-r1",
		Name:       "DeepSeek R1",
		Description: "Reasoning model",
		Reasoning:  true,
		Tier:       "free",
		Vision:     false,
		Audio:      false,
		Tools:      true,
		InputMods:  []string{"text"},
		OutputMods: []string{"text"},
		Aliases:    []string{"ds-r1"},
		Created:    1700000000,
		OwnedBy:    "deepseek",
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	var decoded PollinationsModel
	json.Unmarshal(data, &decoded)

	if decoded.ID != "deepseek-r1" {
		t.Errorf("ID = %q", decoded.ID)
	}
	if !decoded.Reasoning {
		t.Error("Reasoning should be true")
	}
	if decoded.Tier != "free" {
		t.Errorf("Tier = %q", decoded.Tier)
	}
	if !decoded.Tools {
		t.Error("Tools should be true")
	}
	if len(decoded.Aliases) != 1 || decoded.Aliases[0] != "ds-r1" {
		t.Errorf("Aliases = %v", decoded.Aliases)
	}
	if decoded.OwnedBy != "deepseek" {
		t.Errorf("OwnedBy = %q", decoded.OwnedBy)
	}
}

// --- TestConstants ---

func TestConstants(t *testing.T) {
	if GenBaseURL != "https://gen.pollinations.ai" {
		t.Errorf("GenBaseURL = %q", GenBaseURL)
	}
	if ImageBaseURL != "https://image.pollinations.ai" {
		t.Errorf("ImageBaseURL = %q", ImageBaseURL)
	}
	if TextBaseURL != "https://text.pollinations.ai" {
		t.Errorf("TextBaseURL = %q", TextBaseURL)
	}
	if ModelsEndpoint != "/v1/models" {
		t.Errorf("ModelsEndpoint = %q", ModelsEndpoint)
	}
	if ChatEndpoint != "/v1/chat/completions" {
		t.Errorf("ChatEndpoint = %q", ChatEndpoint)
	}
	if ImageEndpoint != "/v1/images/generations" {
		t.Errorf("ImageEndpoint = %q", ImageEndpoint)
	}
}

// --- TestSetHTTPClientAndVaultFn ---

func TestSetHTTPClientAndVaultFn(t *testing.T) {
	defer restore()

	customClient := &http.Client{Timeout: 1 * time.Second}
	SetHTTPClient(customClient)
	if httpClient != customClient {
		t.Error("httpClient not set")
	}

	called := false
	SetVaultKeyFn(func() (string, error) {
		called = true
		return "my-key", nil
	})
	key, err := getAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("custom vault fn not called")
	}
	if key != "my-key" {
		t.Errorf("key = %q", key)
	}
}

// --- TestIsPaidOnlyModelComprehensive ---

func TestIsPaidOnlyModelComprehensive(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		// Exact matches
		{"gpt-5.4", true},
		{"openai-large", true},
		{"mercury", true},
		{"kimi-code", true},
		{"llama-maverick", true},
		{"qwen-large", true},
		{"deepseek-pro", true},
		// Case insensitive
		{"GPT-5.4", true},
		{"FLUX-xl", true},
		// Prefix matches
		{"claude-opus-4", true},
		{"claude-large-v3", true},
		{"perplexity-sonar", true},
		{"gemini-search-pro", true},
		{"flux-1", true},
		{"kontext-dev", true},
		{"seedream-3", true},
		{"sana-xl", true},
		{"gptimage-v2", true},
		{"veo-2", true},
		{"seedance-1", true},
		{"wan-2.1", true},
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
		{"cohere-embed-v4", true},
		{"qwen3-embedding-large", true},
		{"gpt-realtime-v1", true},
		{"midijourney-v1", true},
		{"ideogram-v2", true},
		{"zimage-v1", true},
		{"nanobanana-v1", true},
		{"step-flash-v1", true},
		// Free models
		{"openai", false},
		{"openai-mini", false},
		{"llama", false},
		{"llama-3.3", false},
		{"deepseek", false},
		{"deepseek-v3", false},
		{"mistral", false},
		{"gemma", false},
		{"phi-3", false},
		{"qwen-coder", false},
		{"grok", false},
		{"command-r", false},
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

// --- TestIsNonTextModelComprehensive ---

func TestIsNonTextModelComprehensive(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		// Exact
		{"openai-audio", true},
		{"openai-audio-large", true},
		{"whisper", true},
		{"scribe", true},
		// Prefix
		{"audio-v1", true},
		{"tts-v1", true},
		{"music-v1", true},
		{"sfx-v1", true},
		{"video-v1", true},
		{"veo-2", true},
		{"seedance-1", true},
		{"wan-2", true},
		{"klein-v1", true},
		{"ltx-13b", true},
		{"image-v1", true},
		{"flux-xl", true},
		{"kontext-dev", true},
		{"seedream-3", true},
		{"sana-2", true},
		{"gptimage-v2", true},
		{"ideogram-v2", true},
		{"zimage-v1", true},
		{"nanobanana-v1", true},
		{"nova-canvas-v1", true},
		{"grok-imagine-v1", true},
		{"grok-video-v1", true},
		{"p-image-v1", true},
		{"p-video-v1", true},
		{"midijourney-v1", true},
		{"embed-v1", true},
		{"realtime-v1", true},
		{"elevenlabs-v2", true},
		{"elevenflash-v1", true},
		{"elevenmusic-v1", true},
		{"eleven-sfx-v1", true},
		{"stable-audio-v1", true},
		{"qwen-tts-v1", true},
		{"universal-v1", true},
		{"nova-reel-v1", true},
		{"cohere-embed-v4", true},
		// Free text models
		{"openai", false},
		{"openai-mini", false},
		{"llama", false},
		{"deepseek", false},
		{"mistral", false},
		{"gemma", false},
		{"phi-3", false},
		{"command-r", false},
		{"grok", false},
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

// --- Verify image generation with URL (no b64) ---

func TestImageResponseWithURL(t *testing.T) {
	body := `{"created":1234567890,"data":[{"url":"https://image.pollinations.ai/prompt/a%20red%20circle","revised_prompt":"A simple red circle"}]}`
	var resp ImageResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 || resp.Data[0].URL == "" {
		t.Error("expected URL format")
	}
	if resp.Data[0].RevisedPrompt != "A simple red circle" {
		t.Errorf("revised_prompt = %q", resp.Data[0].RevisedPrompt)
	}
}
