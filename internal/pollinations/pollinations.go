package pollinations

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/vault"
)

var logger = log.New(log.Writer(), "[pollinations] ", log.LstdFlags)

const (
	// Base URLs
	GenBaseURL   = "https://gen.pollinations.ai"
	ImageBaseURL = "https://image.pollinations.ai"
	TextBaseURL  = "https://text.pollinations.ai"

	// Endpoints
	ModelsEndpoint = "/v1/models"
	ChatEndpoint   = "/v1/chat/completions"
	ImageEndpoint  = "/v1/images/generations"

	// Vault path
	VaultPath = "pollinations"
)

// PollinationsModel — модель из Pollinations API
type PollinationsModel struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Reasoning   bool     `json:"reasoning"`
	Tier        string   `json:"tier"`
	Vision      bool     `json:"vision"`
	Audio       bool     `json:"audio"`
	Tools       bool     `json:"tools"`
	InputMods   []string `json:"input_modalities"`
	OutputMods  []string `json:"output_modalities"`
	Aliases     []string `json:"aliases"`
	Created     int64    `json:"created"`
	OwnedBy     string   `json:"owned_by"`
}

// ModelsResponse — ответ /v1/models
type ModelsResponse struct {
	Object string              `json:"object"`
	Data   []PollinationsModel `json:"data"`
}

// ChatRequest — запрос к chat completions
type ChatRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

// Message — сообщение в чате
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse — ответ от chat completions
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string        `json:"role"`
			Content   string        `json:"content"`
			Reasoning string        `json:"reasoning"`
			Refusal   interface{}   `json:"refusal"`
			ToolCalls []interface{} `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ImageResponse — ответ от image generation
type ImageResponse struct {
	Created int `json:"created"`
	Data    []struct {
		URL           string `json:"url,omitempty"`
		B64JSON       string `json:"b64_json,omitempty"`
		RevisedPrompt string `json:"revised_prompt,omitempty"`
	} `json:"data"`
}

// ModelTestResult — результат тестирования модели
type ModelTestResult struct {
	ModelID      string `json:"model_id"`
	ModelAlias   string `json:"model_alias"`
	IsFree       bool   `json:"is_free"`
	IsWorking    bool   `json:"is_working"`
	Error        string `json:"error,omitempty"`
	ResponseTime int64  `json:"response_time_ms"`
	ActualModel  string `json:"actual_model,omitempty"`
	SampleOutput string `json:"sample_output,omitempty"`
}

// ProviderInfo — полная информация о Pollinations как провайдере
type ProviderInfo struct {
	Name       string            `json:"name"`
	URL        string            `json:"url"`
	APIKeyURL  string            `json:"api_key_url"`
	CreditCard bool              `json:"credit_card"`
	Status     string            `json:"status"`
	Models     []string          `json:"models"`
	ModelsFree []string          `json:"models_free"`
	ModelsPaid []string          `json:"models_paid"`
	Limits     string            `json:"limits"`
	Notes      string            `json:"notes"`
	Endpoints  map[string]string `json:"endpoints"`
	VerifiedAt string            `json:"verified_at"`
}

var httpClient = &http.Client{
	Timeout: 6 * time.Second,
}

// getAPIKey — получить ключ из vault
var getAPIKeyFn = func() (string, error) {
	return vault.GetDefaultKey("free-api-hunter/pollinations")
}

func getAPIKey() (string, error) {
	return getAPIKeyFn()
}

// SetHTTPClient allows tests to inject a custom HTTP client.
func SetHTTPClient(client *http.Client) {
	httpClient = client
}

// SetVaultKeyFn allows tests to inject a custom key function.
func SetVaultKeyFn(fn func() (string, error)) {
	getAPIKeyFn = fn
}

// GetModels — получить список всех моделей из Pollinations API
func GetModels() (*ModelsResponse, error) {
	resp, err := httpClient.Get(GenBaseURL + ModelsEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Попробовать как массив (legacy format)
		var legacy []PollinationsModel
		body, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(strings.NewReader(string(body)))
		if err := json.Unmarshal(body, &legacy); err != nil {
			return nil, fmt.Errorf("failed to parse models: %w", err)
		}
		result.Object = "list"
		result.Data = legacy
	}

	return &result, nil
}

// TestModel — протестировать одну модель (бесплатная или платная)
func TestModel(modelID string) *ModelTestResult {
	result := &ModelTestResult{
		ModelID:    modelID,
		ModelAlias: modelID,
	}

	// Получаем ключ
	key, err := getAPIKey()
	if err != nil {
		result.Error = "no_api_key: " + err.Error()
		return result
	}

	// Формируем запрос
	reqBody := ChatRequest{
		Model: modelID,
		Messages: []Message{
			{Role: "user", Content: "Say hi in one word"},
		},
		MaxTokens: 5,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", GenBaseURL+ChatEndpoint, strings.NewReader(string(body)))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	start := time.Now()
	resp, err := httpClient.Do(req)
	result.ResponseTime = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = "request_failed: " + err.Error()
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		result.Error = "parse_error"
		return result
	}

	// Проверяем ошибку
	if chatResp.Error != nil {
		msg := chatResp.Error.Message
		if strings.Contains(msg, "balance") || strings.Contains(msg, "pollen") || strings.Contains(msg, "Insufficient") {
			result.IsFree = false
			result.IsWorking = true
			result.Error = "paid_model"
		} else if strings.Contains(msg, "Authentication") {
			result.Error = "auth_error"
		} else if strings.Contains(msg, "not found") {
			result.Error = "model_not_found"
		} else {
			result.Error = msg
		}
		return result
	}

	// Успешный ответ
	if len(chatResp.Choices) > 0 {
		result.IsFree = true
		result.IsWorking = true
		result.ActualModel = chatResp.Model
		result.SampleOutput = chatResp.Choices[0].Message.Content
		return result
	}

	result.Error = "no_choices"
	return result
}

// TestAllModels — протестировать все модели и разделить на free/paid
func TestAllModels() (*ProviderInfo, []ModelTestResult) {
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
		VerifiedAt: models.Now(),
	}

	// Получаем список моделей
	modelsResp, err := GetModels()
	if err != nil {
		logger.Printf("Failed to get models: %v", err)
		info.Notes = "Failed to fetch models: " + err.Error()
		return info, nil
	}

	var results []ModelTestResult
	var mu sync.Mutex
	freeModels := []string{}
	paidModels := []string{}
	allModels := []string{}

	// Собираем список моделей для тестирования (только текстовые, не платные)
	testList := []PollinationsModel{}
	for _, m := range modelsResp.Data {
		modelID := m.ID
		if modelID == "" {
			modelID = m.Name
		}
		if modelID == "" {
			continue
		}
		allModels = append(allModels, modelID)

		if isPaidOnlyModel(modelID) {
			paidModels = append(paidModels, modelID)
			mu.Lock()
			results = append(results, ModelTestResult{
				ModelID:   modelID,
				IsFree:    false,
				IsWorking: false,
				Error:     "paid_only",
			})
			mu.Unlock()
			continue
		}

		if isNonTextModel(modelID) {
			continue
		}

		testList = append(testList, m)
	}

	logger.Printf("Testing %d Pollinations models (parallel, 10 workers)...", len(testList))

	// Параллельное тестирование с worker pool
	const workers = 10
	jobs := make(chan PollinationsModel, len(testList))
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range jobs {
				modelID := m.ID
				if modelID == "" {
					modelID = m.Name
				}
				result := TestModel(modelID)

				mu.Lock()
				results = append(results, *result)
				if result.IsFree {
					freeModels = append(freeModels, modelID)
					logger.Printf("  ✅ %s → %s (free, %dms)", modelID, result.ActualModel, result.ResponseTime)
				} else if result.Error == "paid_model" {
					paidModels = append(paidModels, modelID)
					logger.Printf("  💰 %s (paid)", modelID)
				} else {
					logger.Printf("  ⚠️ %s: %s", modelID, result.Error)
				}
				mu.Unlock()
			}
		}()
	}

	for _, m := range testList {
		jobs <- m
	}
	close(jobs)
	wg.Wait()

	info.Models = allModels
	info.ModelsFree = freeModels
	info.ModelsPaid = paidModels
	info.Limits = fmt.Sprintf("%d free models, %d paid models. Free tier requires API key but no credits.", len(freeModels), len(paidModels))
	info.Notes = fmt.Sprintf("OpenAI-compatible API. %d total models, %d free for chat. Key in vault/free-api-hunter/pollinations/. Image gen via /v1/images/generations (b64) or image.pollinations.ai (direct).", len(allModels), len(freeModels))

	return info, results
}

// isPaidOnlyModel — модель только за плату (известные)
func isPaidOnlyModel(modelID string) bool {
	// Точные совпадения (полное имя модели)
	paidExact := map[string]bool{
		"gpt-5.4": true, "openai-large": true, "mercury": true,
		"kimi-code": true, "llama-maverick": true, "qwen-large": true,
		"deepseek-pro": true,
	}
	if paidExact[strings.ToLower(modelID)] {
		return true
	}

	// Префиксные совпадения (startsWith)
	paidPrefixes := []string{
		"claude-opus", "claude-large",
		"perplexity", "gemini-search",
		"flux", "kontext", "seedream", "sana", "gptimage",
		"veo", "seedance", "wan", "elevenlabs", "elevenflash",
		"elevenmusic", "whisper", "scribe", "universal",
		"nova-canvas", "nova-reel", "grok-imagine", "grok-video",
		"klein", "ltx-", "p-image", "p-video", "acestep",
		"stable-audio", "qwen-tts", "openai-3-",
		"cohere-embed", "qwen3-embedding", "gpt-realtime",
		"midijourney", "ideogram", "zimage", "nanobanana",
		"step-flash",
	}
	lower := strings.ToLower(modelID)
	for _, p := range paidPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// isNonTextModel — модель не для текста (аудио, видео, изображения)
func isNonTextModel(modelID string) bool {
	lower := strings.ToLower(modelID)

	// Точные совпадения
	nonTextExact := map[string]bool{
		"openai-audio": true, "openai-audio-large": true,
		"whisper": true, "scribe": true,
	}
	if nonTextExact[lower] {
		return true
	}

	// Префиксные совпадения
	nonTextPrefixes := []string{
		"audio", "tts", "music", "sfx",
		"video", "veo", "seedance", "wan", "klein", "ltx",
		"image", "flux", "kontext", "seedream", "sana", "gptimage",
		"ideogram", "zimage", "nanobanana", "nova-canvas", "grok-imagine",
		"grok-video", "p-image", "p-video", "midijourney",
		"embed", "realtime", "elevenlabs", "elevenflash", "elevenmusic",
		"eleven-sfx", "stable-audio", "qwen-tts", "universal",
		"nova-reel", "cohere-",
	}
	for _, p := range nonTextPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// ToProvider — преобразовать ProviderInfo в models.Provider
func ToProvider(info *ProviderInfo) *models.Provider {
	providerModels := info.ModelsFree
	if len(providerModels) == 0 {
		providerModels = info.Models
	}
	return &models.Provider{
		Name:         info.Name,
		URL:          info.URL,
		APIKeyURL:    info.APIKeyURL,
		CreditCard:   info.CreditCard,
		Status:       models.ProviderStatus(info.Status),
		Models:       providerModels,
		Limits:       info.Limits,
		Notes:        info.Notes,
		Source:       "raven",
		DiscoveredAt: info.VerifiedAt,
		LastVerified: &info.VerifiedAt,
	}
}

// VerifyImageGeneration — проверить генерацию изображений
func VerifyImageGeneration() (bool, string) {
	key, err := getAPIKey()
	if err != nil {
		return false, "no_key: " + err.Error()
	}

	reqBody := map[string]interface{}{
		"prompt": "a red circle",
		"n":      1,
		"size":   "64x64",
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", GenBaseURL+ImageEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return false, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, "request_failed: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	var imgResp ImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&imgResp); err != nil {
		return false, "parse_error: " + err.Error()
	}

	if len(imgResp.Data) == 0 {
		return false, "no_images"
	}

	hasImage := imgResp.Data[0].URL != "" || imgResp.Data[0].B64JSON != ""
	return hasImage, fmt.Sprintf("image_generated (b64: %d chars)", len(imgResp.Data[0].B64JSON))
}
