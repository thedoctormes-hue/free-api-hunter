package orex

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSelectModel(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/select" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req SelectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.TaskType != "summarize" {
			t.Fatalf("unexpected task_type: %s", req.TaskType)
		}
		if req.MinContextLen != 32000 {
			t.Fatalf("unexpected min_context_length: %d", req.MinContextLen)
		}
		resp := SelectResponse{
			Task: "summarize",
			Results: []SelectResult{
				{Model: "openrouter/gpt-oss-120b", Score: 0.95, Reason: "Best fit", PricePer1M: 0, ContextLen: 131072, IsFree: true},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer ts.Close()

	req := SelectRequest{TaskType: "summarize", MinContextLen: 32000, RequireFree: true}
	resp, err := client.SelectModel(req)
	if err != nil {
		t.Fatalf("SelectModel error: %v", err)
	}
	if resp.Task != "summarize" {
		t.Fatalf("unexpected task: %s", resp.Task)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Model != "openrouter/gpt-oss-120b" {
		t.Fatalf("unexpected model: %s", resp.Results[0].Model)
	}
	if resp.Results[0].Score != 0.95 {
		t.Fatalf("unexpected score: %f", resp.Results[0].Score)
	}
}

func TestSelectModelServerError(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})
	defer ts.Close()

	req := SelectRequest{TaskType: "test"}
	_, err := client.SelectModel(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSelectModelSimple(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/select" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("task_type") != "coding" {
			t.Fatalf("unexpected task_type query: %s", r.URL.Query().Get("task_type"))
		}
		json.NewEncoder(w).Encode(SelectResponse{
			Task: "coding",
			Results: []SelectResult{
				{Model: "openrouter/deepseek-coder", Score: 0.9},
			},
		})
	})
	defer ts.Close()

	resp, err := client.SelectModelSimple("coding")
	if err != nil {
		t.Fatalf("SelectModelSimple error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
}

func TestGetCost(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pricing/cost" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("model") != "gpt-4" {
			t.Fatalf("unexpected model: %s", r.URL.Query().Get("model"))
		}
		json.NewEncoder(w).Encode(CostResponse{
			Model:   "gpt-4",
			CostUSD: 0.03,
		})
	})
	defer ts.Close()

	resp, err := client.GetCost("gpt-4", 1000, 500)
	if err != nil {
		t.Fatalf("GetCost error: %v", err)
	}
	if resp.CostUSD != 0.03 {
		t.Fatalf("unexpected cost: %f", resp.CostUSD)
	}
}

func TestGetAlertsSince(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/alerts" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		since := r.URL.Query().Get("since")
		if since != "2026-06-01T00:00:00Z" {
			t.Fatalf("unexpected since: %s", since)
		}
		json.NewEncoder(w).Encode(AlertsResponse{
			Alerts: []OrexAlert{
				{Type: "new_model", Model: "gpt-5", Message: "New model!", Timestamp: "2026-06-15T00:00:00Z"},
			},
		})
	})
	defer ts.Close()

	resp, err := client.GetAlertsSince("2026-06-01T00:00:00Z")
	if err != nil {
		t.Fatalf("GetAlertsSince error: %v", err)
	}
	if len(resp.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(resp.Alerts))
	}
	if resp.Alerts[0].Model != "gpt-5" {
		t.Fatalf("unexpected model: %s", resp.Alerts[0].Model)
	}
}

func TestGetModelsByProvider(t *testing.T) {
	client, ts := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("provider") != "cerebras" {
			t.Fatalf("unexpected provider: %s", r.URL.Query().Get("provider"))
		}
		json.NewEncoder(w).Encode(ModelsResponse{
			Models: []OrexModel{
				{ID: "cerebras/llama-3.1-8b", Name: "llama-3.1-8b", ContextLength: 8192},
			},
		})
	})
	defer ts.Close()

	models, err := client.GetModelsByProvider("cerebras")
	if err != nil {
		t.Fatalf("GetModelsByProvider error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Name != "llama-3.1-8b" {
		t.Fatalf("unexpected model name: %s", models[0].Name)
	}
}
