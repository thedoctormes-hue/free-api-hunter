package orex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "http://127.0.0.1:8710"
const defaultTimeout = 15 * time.Second

// Client — HTTP-клиент для Orex (OpenRouter Expert) API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient — создать клиент Orex
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// doRequest — выполнить HTTP-запрос и распарсить JSON-ответ
func (c *Client) doRequest(path string, target interface{}) error {
	url := c.baseURL + path
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("orex request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("orex HTTP %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// ============================================================
// Models
// ============================================================

// OrexModel — модель из Orex API
type OrexModel struct {
	ID            string       `json:"id"`
	CanonicalSlug string       `json:"canonical_slug"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	ContextLength int          `json:"context_length"`
	Pricing       OrexPricing  `json:"pricing"`
	TopProvider   OrexProvider `json:"top_provider"`
	Architecture  OrexArch     `json:"architecture"`
	Created       int64        `json:"created"`
}

// OrexPricing — ценообразование модели
type OrexPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	WebSearch  string `json:"web_search"`
}

// OrexProvider — провайдер модели
type OrexProvider struct {
	ContextLength  int    `json:"context_length"`
	MaxCompletion  int    `json:"max_completion_tokens,omitempty"`
	IsModerated    bool   `json:"is_moderated,omitempty"`
}

// OrexArch — архитектура модели
type OrexArch struct {
	Modality         string   `json:"modality"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Tokenizer        string   `json:"tokenizer"`
}

// ModelsResponse — ответ /api/models
type ModelsResponse struct {
	Models []OrexModel `json:"models"`
}

// GetModels — получить список моделей из Orex
func (c *Client) GetModels() (*ModelsResponse, error) {
	var resp ModelsResponse
	if err := c.doRequest("/api/models", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFreeModels — получить только бесплатные модели
func (c *Client) GetFreeModels() ([]OrexModel, error) {
	var resp ModelsResponse
	path := "/api/models?pricing_free=true"
	if err := c.doRequest(path, &resp); err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// GetModelsByProvider — получить модели конкретного провайдера
func (c *Client) GetModelsByProvider(provider string) ([]OrexModel, error) {
	var resp ModelsResponse
	path := fmt.Sprintf("/api/models?provider=%s", provider)
	if err := c.doRequest(path, &resp); err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// ============================================================
// Select
// ============================================================

// SelectRequest — запрос на подбор модели под задачу
type SelectRequest struct {
	TaskType      string  `json:"task_type"`
	MaxPricePer1M float64 `json:"max_price_per_1m,omitempty"`
	MinContextLen int     `json:"min_context_length,omitempty"`
	RequireFree   bool    `json:"require_free,omitempty"`
}

// SelectResult — результат подбора модели
type SelectResult struct {
	Model      string  `json:"model"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
	PricePer1M float64 `json:"price_per_1m"`
	ContextLen int     `json:"context_length"`
	IsFree     bool    `json:"is_free"`
}

// SelectResponse — ответ /api/select
type SelectResponse struct {
	Task    string         `json:"task"`
	Results []SelectResult `json:"results"`
}

// SelectModel — подобрать модель под задачу (POST)
func (c *Client) SelectModel(req SelectRequest) (*SelectResponse, error) {
	url := c.baseURL + "/api/select"
	body, _ := json.Marshal(req)
	resp, err := c.httpClient.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("orex select failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("orex select HTTP %d: %s", resp.StatusCode, string(b))
	}

	var result SelectResponse
	return &result, json.NewDecoder(resp.Body).Decode(&result)
}

// SelectModelSimple — подобрать модель (GET)
func (c *Client) SelectModelSimple(taskType string) (*SelectResponse, error) {
	var resp SelectResponse
	path := fmt.Sprintf("/api/select?task_type=%s", taskType)
	if err := c.doRequest(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================
// Pricing
// ============================================================

// CostRequest — запрос на расчёт стоимости
type CostRequest struct {
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
}

// CostResponse — ответ на расчёт стоимости
type CostResponse struct {
	Model   string  `json:"model"`
	CostUSD float64 `json:"cost_usd"`
}

// GetCost — рассчитать стоимость запроса
func (c *Client) GetCost(model string, promptTokens, completionTokens int) (*CostResponse, error) {
	var resp CostResponse
	path := fmt.Sprintf("/api/pricing/cost?model=%s&prompt_tokens=%d&completion_tokens=%d",
		model, promptTokens, completionTokens)
	if err := c.doRequest(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================
// Sync
// ============================================================

// SyncResponse — ответ /api/sync
type SyncResponse struct {
	Status  string `json:"status"`
	Models  int    `json:"models_count"`
	Message string `json:"message"`
}

// Sync — синхронизировать базу моделей с OpenRouter
func (c *Client) Sync() (*SyncResponse, error) {
	var resp SyncResponse
	if err := c.doRequest("/api/sync", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================
// Alerts
// ============================================================

// OrexAlert — алерт от Orex
type OrexAlert struct {
	Type      string `json:"type"`
	Model     string `json:"model"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// AlertsResponse — ответ /api/alerts
type AlertsResponse struct {
	Alerts []OrexAlert `json:"alerts"`
}

// GetAlerts — получить алерты
func (c *Client) GetAlerts() (*AlertsResponse, error) {
	var resp AlertsResponse
	if err := c.doRequest("/api/alerts", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAlertsSince — получить алерты с определённого времени
func (c *Client) GetAlertsSince(since string) (*AlertsResponse, error) {
	var resp AlertsResponse
	path := fmt.Sprintf("/api/alerts?since=%s", since)
	if err := c.doRequest(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ============================================================
// FreeModelsProvider — отфильтровать бесплатные модели
// ============================================================

// FreeModel — упрощённая структура бесплатной модели
type FreeModel struct {
	ID            string
	Name          string
	Provider      string
	ContextLength int
	IsFree        bool
	Description   string
}

// ToFreeModels — преобразовать OrexModel в FreeModel (только бесплатные)
func ToFreeModels(models []OrexModel) []FreeModel {
	var result []FreeModel
	for _, m := range models {
		if m.Pricing.Prompt == "0" && m.Pricing.Completion == "0" {
			provider := splitProvider(m.ID)
			result = append(result, FreeModel{
				ID:            m.ID,
				Name:          m.Name,
				Provider:      provider,
				ContextLength: m.ContextLength,
				IsFree:        true,
				Description:   m.Description,
			})
		}
	}
	return result
}

func splitProvider(id string) string {
	for i, c := range id {
		if c == '/' {
			return id[:i]
		}
	}
	return ""
}
