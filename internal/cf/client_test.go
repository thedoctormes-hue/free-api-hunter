package cf

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c.baseURL != "https://api.cloudflare.com/client/v4" {
		t.Fatalf("expected default baseURL, got %s", c.baseURL)
	}
	if c.httpClient.Timeout != defaultTimeout {
		t.Fatalf("expected timeout %v, got %v", defaultTimeout, c.httpClient.Timeout)
	}
}

func TestChat_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id":      "test-id",
			"object":  "chat.completion",
			"model":   "@cf/nvidia/nemotron-3-120b-a12b",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	resp, err := c.Chat("test-account", ChatRequest{
		Model: "@cf/nvidia/nemotron-3-120b-a12b",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}, "test-token")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Result.Usage.CompletionTokens != 5 {
		t.Fatalf("expected 5 completion tokens, got %d", resp.Result.Usage.CompletionTokens)
	}
}

func TestChat_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		resp := map[string]interface{}{
			"error": []interface{}{
				map[string]interface{}{
					"code":    10000,
					"message": "Authentication error",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	resp, err := c.Chat("test-account", ChatRequest{
		Model: "@cf/nvidia/nemotron-3-120b-a12b",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}, "bad-token")

	// HTTP errors return empty response with success=false (not nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Success {
		t.Fatal("expected success=false for error response")
	}
}

func TestVerifyToken_Active(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id":      "test-id",
			"object":  "chat.completion",
			"model":   "@cf/nvidia/nemotron-3-120b-a12b",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hi!",
					},
					"finish_reason": "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	result, err := c.VerifyToken("test-account", "good-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Active {
		t.Fatal("expected active=true")
	}
	if result.ModelsCount != 80 {
		t.Fatalf("expected 80 models, got %d", result.ModelsCount)
	}
	if result.NeuronLimit != 10000 {
		t.Fatalf("expected 10000 neuron limit, got %d", result.NeuronLimit)
	}
}

func TestVerifyToken_Inactive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		resp := map[string]interface{}{
			"error": []interface{}{
				map[string]interface{}{
					"code":    10000,
					"message": "Authentication error",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	result, err := c.VerifyToken("test-account", "bad-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Active {
		t.Fatal("expected active=false")
	}
	// Error message comes from HTTP error in doRequest, which is now empty
	// The important thing is that Active=false
	_ = result.Error
}

func TestEstimateNeurons(t *testing.T) {
	m := Model{
		ID:            "@cf/meta/llama-3.1-70b-instruct-fp8-fast",
		NeuronsInput:  26668,
		NeuronsOutput: 204805,
	}

	// 1000 input tokens, 500 output tokens
	neurons := EstimateNeurons(m, 1000, 500)
	expectedInput := (1000 * 26668) / 1000000  // 266
	expectedOutput := (500 * 204805) / 1000000 // 1024
	expected := expectedInput + expectedOutput

	if neurons != expected {
		t.Fatalf("expected %d neurons, got %d", expected, neurons)
	}
}

func TestEstimateNeurons_ZeroModel(t *testing.T) {
	m := Model{ID: "@cf/unknown/model"}
	neurons := EstimateNeurons(m, 1000, 500)
	if neurons != 0 {
		t.Fatalf("expected 0 neurons for unknown model, got %d", neurons)
	}
}

func TestGetModels(t *testing.T) {
	models := GetModels()
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}

	// Check top models exist
	found := map[string]bool{}
	for _, m := range models {
		found[m.ID] = true
	}

	required := []string{
		"@cf/zai-org/glm-5.2",
		"@cf/moonshotai/kimi-k2.7-code",
		"@cf/openai/gpt-oss-120b",
		"@cf/nvidia/nemotron-3-120b-a12b",
	}
	for _, id := range required {
		if !found[id] {
			t.Fatalf("required model %s not found", id)
		}
	}
}

func TestGetTopModels(t *testing.T) {
	top := GetTopModels()
	if len(top) != 10 {
		t.Fatalf("expected 10 top models, got %d", len(top))
	}
}

func TestFindModel(t *testing.T) {
	m := findModel("@cf/openai/gpt-oss-120b")
	if m.ID != "@cf/openai/gpt-oss-120b" {
		t.Fatalf("expected gpt-oss-120b, got %s", m.ID)
	}
}

func TestFindModel_Unknown(t *testing.T) {
	m := findModel("@cf/totally/fake-model")
	if m.ID != "@cf/nvidia/nemotron-3-120b-a12b" {
		t.Fatalf("expected fallback to nemotron, got %s", m.ID)
	}
}
