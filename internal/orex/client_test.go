package orex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient — создать клиент с тестовым сервером
func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	ts := httptest.NewServer(handler)
	return NewClient(ts.URL), ts
}

func TestNewClient_DefaultURL(t *testing.T) {
	c := NewClient("")
	if c.baseURL != defaultBaseURL {
		t.Fatalf("expected %s, got %s", defaultBaseURL, c.baseURL)
	}
}

func TestNewClient_CustomURL(t *testing.T) {
	c := NewClient("http://localhost:9999")
	if c.baseURL != "http://localhost:9999" {
		t.Fatalf("expected custom URL, got %s", c.baseURL)
	}
}

func TestGetModels(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		resp := ModelsResponse{
			Models: []OrexModel{
				{
					ID:            "openrouter/gpt-oss-120b",
					Name:          "gpt-oss-120b",
					Description:   "Test model",
					ContextLength: 8192,
					Pricing:       OrexPricing{Prompt: "0", Completion: "0"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	result, err := client.GetModels()
	if err != nil {
		t.Fatalf("GetModels error: %v", err)
	}
	if len(result.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(result.Models))
	}
	if result.Models[0].Name != "gpt-oss-120b" {
		t.Fatalf("unexpected model name: %s", result.Models[0].Name)
	}
}

func TestGetFreeModels(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("pricing_free") != "true" {
			t.Fatal("expected pricing_free=true query param")
		}
		resp := ModelsResponse{
			Models: []OrexModel{
				{
					ID:            "openrouter/free-model",
					Name:          "free-model",
					ContextLength: 4096,
					Pricing:       OrexPricing{Prompt: "0", Completion: "0"},
				},
				{
					ID:            "openrouter/paid-model",
					Name:          "paid-model",
					ContextLength: 8192,
					Pricing:       OrexPricing{Prompt: "0.001", Completion: "0.002"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	models, err := client.GetFreeModels()
	if err != nil {
		t.Fatalf("GetFreeModels error: %v", err)
	}
	// GetFreeModels returns all models from the endpoint (filtering is done by Orex)
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

func TestSync(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sync" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SyncResponse{
			Status: "ok",
			Models: 100,
		})
	})
	defer ts.Close()

	resp, err := client.Sync()
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	if resp.Models != 100 {
		t.Fatalf("expected 100 models, got %d", resp.Models)
	}
}

func TestGetAlerts(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/alerts" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(AlertsResponse{
			Alerts: []OrexAlert{
				{
					Type:      "new_model",
					Model:     "gpt-oss-120b",
					Message:   "New free model available",
					Timestamp: "2026-06-18T12:00:00Z",
				},
			},
		})
	})
	defer ts.Close()

	resp, err := client.GetAlerts()
	if err != nil {
		t.Fatalf("GetAlerts error: %v", err)
	}
	if len(resp.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(resp.Alerts))
	}
	if resp.Alerts[0].Type != "new_model" {
		t.Fatalf("unexpected alert type: %s", resp.Alerts[0].Type)
	}
}

func TestToFreeModels(t *testing.T) {
	input := []OrexModel{
		{ID: "openrouter/free-1", Name: "free-1", ContextLength: 4096, Pricing: OrexPricing{Prompt: "0", Completion: "0"}},
		{ID: "openrouter/paid-1", Name: "paid-1", ContextLength: 8192, Pricing: OrexPricing{Prompt: "0.001", Completion: "0.002"}},
		{ID: "cerebras/free-2", Name: "free-2", ContextLength: 16384, Pricing: OrexPricing{Prompt: "0", Completion: "0"}},
	}

	result := ToFreeModels(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 free models, got %d", len(result))
	}

	// Check first free model
	if result[0].Provider != "openrouter" {
		t.Fatalf("expected provider openrouter, got %s", result[0].Provider)
	}
	if !result[0].IsFree {
		t.Fatal("expected IsFree=true")
	}
	if result[0].ContextLength != 4096 {
		t.Fatalf("expected context 4096, got %d", result[0].ContextLength)
	}

	// Check second free model
	if result[1].Provider != "cerebras" {
		t.Fatalf("expected provider cerebras, got %s", result[1].Provider)
	}
}

func TestToFreeModels_Empty(t *testing.T) {
	result := ToFreeModels([]OrexModel{})
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}
}

func TestSplitProvider(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openrouter/gpt-oss-120b", "openrouter"},
		{"cerebras/llama-3.1-8b", "cerebras"},
		{"noprovidermodel", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := splitProvider(tt.input)
		if got != tt.expected {
			t.Fatalf("splitProvider(%q): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestDoRequest_ServerError(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	})
	defer ts.Close()

	var resp ModelsResponse
	err := client.doRequest("/api/models", &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDoRequest_InvalidJSON(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	defer ts.Close()

	var resp ModelsResponse
	err := client.doRequest("/api/models", &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
