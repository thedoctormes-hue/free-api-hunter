package cf

// Account — один Cloudflare аккаунт с API токеном
type Account struct {
	ID      string `json:"id"`       // user ID = Account ID
	Name    string `json:"name"`     // человекочитаемое имя
	Token   string `json:"token"`    // cfut_ ключ (загружается из vault, не хранится в конфиге)
	Active  bool   `json:"active"`   // false = исчерпан / отключён
}

// Model — Cloudflare Workers AI модель
type Model struct {
	ID            string `json:"id"`             // @cf/author/model
	Name          string `json:"name"`           // человекочитаемое имя
	Author        string `json:"author"`         // провайдер (meta, openai, moonshotai, etc.)
	TaskType      string `json:"task_type"`      // text, embeddings, image, audio
	ContextLength int    `json:"context_length"` // 0 = неизвестно
	MaxTokens     int    `json:"max_tokens"`     // 0 = неизвестно
	IsFree        bool   `json:"is_free"`        // в рамках бесплатного лимита
	NeuronsInput  int    `json:"neurons_input"`  // neurons per M input tokens
	NeuronsOutput int    `json:"neurons_output"` // neurons per M output tokens
}

// NeuronBudget — бюджет Neurons для аккаунта
type NeuronBudget struct {
	AccountID  string `json:"account_id"`
	Limit      int    `json:"limit"`       // дневной лимит (10000 для free)
	Used       int    `json:"used"`        // использовано сегодня
	ResetAt    string `json:"reset_at"`    // ISO время сброса
}

// VerifyResult — результат верификации CF ключа
type VerifyResult struct {
	AccountID    string   `json:"account_id"`
	Active       bool     `json:"active"`
	ModelsCount  int      `json:"models_count"`
	NeuronLimit  int      `json:"neuron_limit"`
	Models       []string `json:"models"`
	Error        string   `json:"error,omitempty"`
	CheckedAt    string   `json:"checked_at"`
}

// ChatRequest — запрос к chat completions
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage — сообщение в чате
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse — ответ от chat completions
type ChatResponse struct {
	Success bool `json:"success"`
	Result  struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	} `json:"result"`
	Error []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
