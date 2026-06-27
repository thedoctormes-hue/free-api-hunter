package cf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTimeout = 15 * time.Second

// Client — HTTP-клиент для Cloudflare Workers AI API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient — создать клиент Cloudflare Workers AI
func NewClient() *Client {
	return &Client{
		baseURL: "https://api.cloudflare.com/client/v4",
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// doRequest — выполнить HTTP-запрос и распарсить JSON-ответ
func (c *Client) doRequest(method, path string, body []byte, token string, target interface{}) error {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("cf request creation failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cf request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cf HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// Chat - execute chat completions request
// accountID is the Cloudflare user ID (= Account ID for Workers AI)
func (c *Client) Chat(accountID string, req ChatRequest, token string) (*ChatResponse, error) {
	path := fmt.Sprintf("/accounts/%s/ai/v1/chat/completions", accountID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("cf chat marshal failed: %w", err)
	}

	var rawResp map[string]interface{}
	if err := c.doRequest(http.MethodPost, path, body, token, &rawResp); err != nil {
		// Return empty response with success=false instead of nil
		return &ChatResponse{Success: false}, nil
	}

	// /ai/v1/chat/completions returns OpenAI-format response (flat, no 'success' wrapper)
	resp := &ChatResponse{
		Success: false,
	}

	// Check for error in response
	if _, ok := rawResp["error"].([]interface{}); ok {
		return resp, nil
	}

	// Check for choices (success)
	if choices, ok := rawResp["choices"].([]interface{}); ok && len(choices) > 0 {
		resp.Success = true
	}

	// Parse usage
	if usage, ok := rawResp["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			resp.Result.Usage.PromptTokens = int(pt)
		}
		if ct, ok := usage["completion_tokens"].(float64); ok {
			resp.Result.Usage.CompletionTokens = int(ct)
		}
		if tt, ok := usage["total_tokens"].(float64); ok {
			resp.Result.Usage.TotalTokens = int(tt)
		}
	}

	return resp, nil
}

// VerifyToken — проверить валидность токена и получить список моделей
// Пробуем простой запрос к @cf/nvidia/nemotron-3-120b-a12b — самой дешёвой модели
func (c *Client) VerifyToken(accountID, token string) (*VerifyResult, error) {
	result := &VerifyResult{
		AccountID: accountID,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	req := ChatRequest{
		Model: "@cf/nvidia/nemotron-3-120b-a12b",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}

	resp, err := c.Chat(accountID, req, token)
	if err != nil {
		result.Error = err.Error()
		return result, nil // не фатальная ошибка — просто ключ не работает
	}

	result.Active = resp.Success
	if resp.Success {
		result.ModelsCount = 80 // значение из документации
		result.NeuronLimit = 10000
		result.Models = []string{
			"@cf/zai-org/glm-5.2",
			"@cf/moonshotai/kimi-k2.7-code",
			"@cf/openai/gpt-oss-120b",
			"@cf/nvidia/nemotron-3-120b-a12b",
			"@cf/deepseek-ai/deepseek-r1-distill-qwen-32b",
			"@cf/meta/llama-3.1-70b-instruct-fp8-fast",
			"@cf/mistralai/mistral-small-3.1-24b-instruct",
			"@cf/meta/llama-4-scout-17b-16e-instruct",
			"@cf/google/gemma-4-26b-a4b-it",
			"@cf/qwen/qwen3-30b-a3b-fp8",
		}
	}

	return result, nil
}

// GetModels — получить список моделей (из документации, т.к. /ai/models не работает с этими ключами)
func GetModels() []Model {
	return []Model{
		// Топ модели
		{ID: "@cf/zai-org/glm-5.2", Name: "GLM-5.2", Author: "zai-org", TaskType: "text", ContextLength: 262000, NeuronsInput: 127273, NeuronsOutput: 400000, IsFree: true},
		{ID: "@cf/moonshotai/kimi-k2.7-code", Name: "Kimi K2.7 Code", Author: "moonshotai", TaskType: "text", ContextLength: 262000, NeuronsInput: 86364, NeuronsOutput: 363636, IsFree: true},
		{ID: "@cf/openai/gpt-oss-120b", Name: "GPT-oss-120B", Author: "openai", TaskType: "text", ContextLength: 131000, NeuronsInput: 31818, NeuronsOutput: 68182, IsFree: true},
		{ID: "@cf/nvidia/nemotron-3-120b-a12b", Name: "Nemotron-3-120B", Author: "nvidia", TaskType: "text", ContextLength: 131000, NeuronsInput: 45455, NeuronsOutput: 136364, IsFree: true},
		{ID: "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b", Name: "DeepSeek R1 Qwen-32B", Author: "deepseek-ai", TaskType: "text", ContextLength: 131000, NeuronsInput: 45170, NeuronsOutput: 443756, IsFree: true},
		{ID: "@cf/meta/llama-3.1-70b-instruct-fp8-fast", Name: "Llama 3.1 70B", Author: "meta", TaskType: "text", ContextLength: 131000, NeuronsInput: 26668, NeuronsOutput: 204805, IsFree: true},
		{ID: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", Name: "Llama 3.3 70B", Author: "meta", TaskType: "text", ContextLength: 131000, NeuronsInput: 26668, NeuronsOutput: 204805, IsFree: true},
		{ID: "@cf/mistralai/mistral-small-3.1-24b-instruct", Name: "Mistral Small 3.1 24B", Author: "mistralai", TaskType: "text", ContextLength: 131000, NeuronsInput: 31876, NeuronsOutput: 50488, IsFree: true},
		{ID: "@cf/meta/llama-4-scout-17b-16e-instruct", Name: "Llama 4 Scout 17B", Author: "meta", TaskType: "text", ContextLength: 131000, NeuronsInput: 24545, NeuronsOutput: 77273, IsFree: true},
		{ID: "@cf/google/gemma-4-26b-a4b-it", Name: "Gemma 4 26B", Author: "google", TaskType: "text", ContextLength: 131000, NeuronsInput: 9091, NeuronsOutput: 27273, IsFree: true},
		// Средние модели
		{ID: "@cf/qwen/qwen3-30b-a3b-fp8", Name: "Qwen3-30B-A3B", Author: "qwen", TaskType: "text", ContextLength: 131000, NeuronsInput: 4625, NeuronsOutput: 30475, IsFree: true},
		{ID: "@cf/zai-org/glm-4.7-flash", Name: "GLM-4.7 Flash", Author: "zai-org", TaskType: "text", ContextLength: 131000, NeuronsInput: 5500, NeuronsOutput: 36400, IsFree: true},
		{ID: "@cf/meta/llama-3.1-8b-instruct-fp8-fast", Name: "Llama 3.1 8B FP8", Author: "meta", TaskType: "text", ContextLength: 131000, NeuronsInput: 4119, NeuronsOutput: 34868, IsFree: true},
		{ID: "@cf/meta/llama-3.1-8b-instruct-fp8", Name: "Llama 3.1 8B FP8 (alt)", Author: "meta", TaskType: "text", ContextLength: 131000, NeuronsInput: 13778, NeuronsOutput: 26128, IsFree: true},
		{ID: "@cf/openai/gpt-oss-20b", Name: "GPT-oss-20B", Author: "openai", TaskType: "text", ContextLength: 131000, NeuronsInput: 18182, NeuronsOutput: 27273, IsFree: true},
		{ID: "@cf/google/gemma-3-12b-it", Name: "Gemma 3 12B", Author: "google", TaskType: "text", ContextLength: 131000, NeuronsInput: 31371, NeuronsOutput: 50560, IsFree: true},
		{ID: "@cf/aisingapore/gemma-sea-lion-v4-27b-it", Name: "SEA-LION V4 27B", Author: "aisingapore", TaskType: "text", ContextLength: 131000, NeuronsInput: 31876, NeuronsOutput: 50488, IsFree: true},
		{ID: "@cf/mistral/mistral-7b-instruct-v0.1", Name: "Mistral 7B", Author: "mistral", TaskType: "text", ContextLength: 131000, NeuronsInput: 10000, NeuronsOutput: 17300, IsFree: true},
		{ID: "@cf/ibm-granite/granite-4.0-h-micro", Name: "Granite 4.0 H Micro", Author: "ibm-granite", TaskType: "text", ContextLength: 131000, NeuronsInput: 1542, NeuronsOutput: 10158, IsFree: true},
		// Лёгкие модели
		{ID: "@cf/meta/llama-3.2-3b-instruct", Name: "Llama 3.2 3B", Author: "meta", TaskType: "text", ContextLength: 131000, NeuronsInput: 4625, NeuronsOutput: 30475, IsFree: true},
		{ID: "@cf/meta/llama-3.2-1b-instruct", Name: "Llama 3.2 1B", Author: "meta", TaskType: "text", ContextLength: 131000, NeuronsInput: 2457, NeuronsOutput: 18252, IsFree: true},
		{ID: "@cf/meta/llama-3.2-11b-vision-instruct", Name: "Llama 3.2 11B Vision", Author: "meta", TaskType: "text", ContextLength: 131000, NeuronsInput: 4410, NeuronsOutput: 61493, IsFree: true},
		{ID: "@cf/qwen/qwen2.5-coder-32b-instruct", Name: "Qwen2.5 Coder 32B", Author: "qwen", TaskType: "text", ContextLength: 131000, NeuronsInput: 60000, NeuronsOutput: 90909, IsFree: true},
		{ID: "@cf/qwen/qwq-32b", Name: "QwQ-32B", Author: "qwen", TaskType: "text", ContextLength: 131000, NeuronsInput: 60000, NeuronsOutput: 90909, IsFree: true},
		{ID: "@cf/moonshotai/kimi-k2.5", Name: "Kimi K2.5", Author: "moonshotai", TaskType: "text", ContextLength: 262000, NeuronsInput: 54545, NeuronsOutput: 272727, IsFree: true},
		{ID: "@cf/moonshotai/kimi-k2.6", Name: "Kimi K2.6", Author: "moonshotai", TaskType: "text", ContextLength: 262000, NeuronsInput: 86364, NeuronsOutput: 363636, IsFree: true},
		// Embeddings
		{ID: "@cf/baai/bge-m3", Name: "BGE-M3", Author: "baai", TaskType: "embeddings", NeuronsInput: 1075, IsFree: true},
		{ID: "@cf/qwen/qwen3-embedding-0.6b", Name: "Qwen3-Embedding-0.6B", Author: "qwen", TaskType: "embeddings", NeuronsInput: 1075, IsFree: true},
		{ID: "@cf/baai/bge-small-en-v1.5", Name: "BGE-Small", Author: "baai", TaskType: "embeddings", NeuronsInput: 1841, IsFree: true},
		{ID: "@cf/baai/bge-base-en-v1.5", Name: "BGE-Base", Author: "baai", TaskType: "embeddings", NeuronsInput: 6058, IsFree: true},
		{ID: "@cf/baai/bge-large-en-v1.5", Name: "BGE-Large", Author: "baai", TaskType: "embeddings", NeuronsInput: 18582, IsFree: true},
		{ID: "@cf/pfnet/plamo-embedding-1b", Name: "Plamo-Embedding-1B", Author: "pfnet", TaskType: "embeddings", NeuronsInput: 1689, IsFree: true},
		// Image
		{ID: "@cf/black-forest-labs/flux-1-schnell", Name: "FLUX.1 Schnell", Author: "black-forest-labs", TaskType: "image", IsFree: true},
		{ID: "@cf/black-forest-labs/flux-2-dev", Name: "FLUX.2 Dev", Author: "black-forest-labs", TaskType: "image", IsFree: true},
		{ID: "@cf/black-forest-labs/flux-2-klein-4b", Name: "FLUX.2 Klein 4B", Author: "black-forest-labs", TaskType: "image", IsFree: true},
		{ID: "@cf/black-forest-labs/flux-2-klein-9b", Name: "FLUX.2 Klein 9B", Author: "black-forest-labs", TaskType: "image", IsFree: true},
		{ID: "@cf/leonardo/lucid-origin", Name: "Lucid Origin", Author: "leonardo", TaskType: "image", IsFree: true},
		{ID: "@cf/leonardo/phoenix-1.0", Name: "Phoenix 1.0", Author: "leonardo", TaskType: "image", IsFree: true},
		// Audio TTS
		{ID: "@cf/deepgram/aura-1", Name: "Aura-1", Author: "deepgram", TaskType: "audio-tts", IsFree: true},
		{ID: "@cf/deepgram/aura-2-en", Name: "Aura-2 EN", Author: "deepgram", TaskType: "audio-tts", IsFree: true},
		{ID: "@cf/myshell-ai/melotts", Name: "Melotts", Author: "myshell-ai", TaskType: "audio-tts", IsFree: true},
		// Audio ASR
		{ID: "@cf/openai/whisper", Name: "Whisper", Author: "openai", TaskType: "audio-asr", IsFree: true},
		{ID: "@cf/openai/whisper-large-v3-turbo", Name: "Whisper Large V3 Turbo", Author: "openai", TaskType: "audio-asr", IsFree: true},
		{ID: "@cf/deepgram/nova-3", Name: "Nova-3", Author: "deepgram", TaskType: "audio-asr", IsFree: true},
	}
}

// EstimateNeurons — оценить расход Neurons для запроса
// Возвращает примерное количество Neurons для input + output
func EstimateNeurons(m Model, inputTokens, outputTokens int) int {
	inputNeurons := 0
	if m.NeuronsInput > 0 {
		inputNeurons = (inputTokens * m.NeuronsInput) / 1000000
	}
	outputNeurons := 0
	if m.NeuronsOutput > 0 {
		outputNeurons = (outputTokens * m.NeuronsOutput) / 1000000
	}
	return inputNeurons + outputNeurons
}
