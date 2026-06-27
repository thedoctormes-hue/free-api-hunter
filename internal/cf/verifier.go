package cf

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"free-api-hunter/internal/models"
)

var logger = log.New(log.Writer(), "[cf-verifier] ", log.LstdFlags)

// HTTPClient — настраиваемый HTTP клиент
var HTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// pool — глобальный пул аккаунтов (инициализируется через InitKeyPool)
var pool *KeyPool

// InitKeyPool — инициализировать пул аккаунтов из конфига
func InitKeyPool(configPath string) error {
	p, err := NewKeyPool(configPath)
	if err != nil {
		return err
	}
	pool = p
	return nil
}

// GetKeyPool — получить текущий пул (для сохранения состояния)
func GetKeyPool() (*KeyPool, bool) {
	if pool == nil {
		return nil, false
	}
	return pool, true
}

// VerifyAccount — проверить один аккаунт Cloudflare Workers AI
// Делает запрос к самой дешёвой модели (nemotron-3-120b-a12b)
func VerifyAccount(account *Account) *VerifyResult {
	result := &VerifyResult{
		AccountID: account.ID,
		CheckedAt: models.Now(),
	}

	c := NewClient()
	resp, err := c.Chat(account.ID, ChatRequest{
		Model: "@cf/nvidia/nemotron-3-120b-a12b",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}, account.Token)

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Active = resp.Success
	if resp.Success {
		result.ModelsCount = 80
		result.NeuronLimit = 10000
		result.Models = GetTopModels()
	}

	return result
}

// VerifyAll — проверить все аккаунты в пуле
func VerifyAll() []*VerifyResult {
	if pool == nil {
		return nil
	}

	var results []*VerifyResult
	for _, acc := range pool.accounts {
		results = append(results, VerifyAccount(&acc.Account))
	}
	return results
}

// ChatWithPool — выполнить chat запрос через пул аккаунтов (с ротацией)
// Автоматически выбирает аккаунт с достаточным NeuronBudget
func ChatWithPool(model string, messages []ChatMessage) (*ChatResponse, string, error) {
	if pool == nil {
		return nil, "", fmt.Errorf("cf pool not initialized")
	}

	m := findModel(model)
	entry, err := pool.NextForPool(m, estimateTokens(messages), 256)
	if err != nil {
		return nil, "", err
	}

	c := NewClient()
	resp, err := c.Chat(entry.ID, ChatRequest{
		Model:    model,
		Messages: messages,
	}, entry.Token)

	if err != nil {
		entry.Active = false // помечаем как неактивный
		return nil, "", err
	}

	// Отчитываем примерный расход
	if resp.Success {
		used := EstimateNeurons(m, estimateTokens(messages), resp.Result.Usage.CompletionTokens)
		pool.ReportUsage(entry.ID, used)
	}

	return resp, entry.ID, nil
}

// NextForPool — обёртка над NextForModel для совместимости
func (p *KeyPool) NextForPool(m Model, inputTokens, outputTokens int) (*AccountEntry, error) {
	return p.NextForModel(m, inputTokens, outputTokens)
}

// GetTopModels — получить список топ-10 моделей
func GetTopModels() []string {
	models := GetModels()
	var top []string
	for i, m := range models {
		if i >= 10 {
			break
		}
		if m.TaskType == "text" {
			top = append(top, m.ID)
		}
	}
	return top
}

// findModel — найти модель по ID
func findModel(id string) Model {
	for _, m := range GetModels() {
		if m.ID == id {
			return m
		}
	}
	// Если модель не найдена — возвращаем дефолтную (самую дешёвую)
	return Model{
		ID:            "@cf/nvidia/nemotron-3-120b-a12b",
		Name:          "Nemotron-3-120B",
		TaskType:      "text",
		NeuronsInput:  45455,
		NeuronsOutput: 136364,
		IsFree:        true,
	}
}

// estimateTokens — грубая оценка количества токенов в сообщениях
func estimateTokens(messages []ChatMessage) int {
	total := 0
	for _, msg := range messages {
		// ~4 chars per token для английского, ~2 для русского
		total += len(msg.Content) / 3
	}
	return total
}
