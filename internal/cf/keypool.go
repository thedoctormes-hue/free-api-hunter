package cf

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"free-api-hunter/internal/vault"
)

// AccountEntry — один аккаунт в пуле
type AccountEntry struct {
	Account
	NeuronsUsed  int    `json:"neurons_used"`  // сколько Neurons потрачено сегодня
	NeuronsLimit int    `json:"neurons_limit"` // дневной лимит (10000 для free)
	LastUsed     string `json:"last_used"`     // ISO timestamp последнего использования
}

// KeyPool — потокобезопасный пул аккаунтов с round-robin ротацией
type KeyPool struct {
	mu       sync.Mutex
	accounts []*AccountEntry
	current  int
	loaded   string
}

// NewKeyPool — создать пул из конфига
func NewKeyPool(configPath string) (*KeyPool, error) {
	pool := &KeyPool{}
	if err := pool.load(configPath); err != nil {
		return nil, err
	}
	return pool, nil
}

// load — загрузить аккаунты из конфига + vault
func (p *KeyPool) load(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("cf keypool: cannot read config %s: %w", configPath, err)
	}

	var cfg struct {
		Accounts []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			AccountID string `json:"account_id"` // user ID
			Limit   int    `json:"limit"`        // дневной лимит Neurons
		} `json:"cf_accounts"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("cf keypool: cannot parse config: %w", err)
	}

	p.accounts = nil
	now := time.Now().UTC().Format(time.RFC3339)

	for _, acc := range cfg.Accounts {
		// Загружаем ключ из vault
		token, err := vault.GetKey("cloudflare", acc.AccountID)
		if err != nil {
			continue // пропускаем если ключа нет в vault
		}

		limit := acc.Limit
		if limit <= 0 {
			limit = 10000 // дефолтный бесплатный лимит
		}

		p.accounts = append(p.accounts, &AccountEntry{
			Account: Account{
				ID:     acc.AccountID,
				Name:   acc.Name,
				Token:  token,
				Active: true,
			},
			NeuronsUsed:  0,
			NeuronsLimit: limit,
			LastUsed:     now,
		})
	}

	p.loaded = now
	return nil
}

// Next — получить следующий активный аккаунт (round-robin)
func (p *KeyPool) Next() (*AccountEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.accounts) == 0 {
		return nil, fmt.Errorf("cf keypool: no accounts configured")
	}

	attempts := 0
	for attempts < len(p.accounts) {
		entry := p.accounts[p.current]
		p.current = (p.current + 1) % len(p.accounts)

		if entry.Active && entry.NeuronsUsed < entry.NeuronsLimit {
			entry.LastUsed = time.Now().UTC().Format(time.RFC3339)
			return entry, nil
		}
		attempts++
	}

	return nil, fmt.Errorf("cf keypool: all accounts exhausted")
}

// NextForModel — получить аккаунт для конкретной модели
// Учитывает NeuronBudget — если на аккаунте не хватает Neurons, берёт следующий
func (p *KeyPool) NextForModel(model Model, inputTokens, outputTokens int) (*AccountEntry, error) {
	estimated := EstimateNeurons(model, inputTokens, outputTokens)

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.accounts) == 0 {
		return nil, fmt.Errorf("cf keypool: no accounts configured")
	}

	attempts := 0
	for attempts < len(p.accounts) {
		entry := p.accounts[p.current]
		p.current = (p.current + 1) % len(p.accounts)

		if entry.Active && (entry.NeuronsUsed+estimated) <= entry.NeuronsLimit {
			entry.LastUsed = time.Now().UTC().Format(time.RFC3339)
			return entry, nil
		}
		attempts++
	}

	return nil, fmt.Errorf("cf keypool: no account with enough Neurons (need %d)", estimated)
}

// ReportUsage — отчитаться о расходе Neurons
func (p *KeyPool) ReportUsage(accountID string, neurons int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, acc := range p.accounts {
		if acc.ID == accountID {
			acc.NeuronsUsed += neurons
			if acc.NeuronsUsed >= acc.NeuronsLimit {
				acc.Active = false // исчерпан
			}
			return
		}
	}
}

// ResetDaily — сбросить дневные счётчики (вызывается из cron)
func (p *KeyPool) ResetDaily() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, acc := range p.accounts {
		acc.NeuronsUsed = 0
		acc.Active = true
	}
}

// Stats — статистика пула
func (p *KeyPool) Stats() []map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	var stats []map[string]interface{}
	for _, acc := range p.accounts {
		stats = append(stats, map[string]interface{}{
			"id":            acc.ID,
			"name":          acc.Name,
			"active":        acc.Active,
			"neurons_used":  acc.NeuronsUsed,
			"neurons_limit": acc.NeuronsLimit,
			"last_used":     acc.LastUsed,
		})
	}
	return stats
}

// loadVaultKeys — загрузить ключи из vault для провайдера
// (заглушка — vault уже имеет готовые методы)
func loadVaultKeys(provider string) ([]string, error) {
	dir := vault.VaultPath + "/" + provider
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			continue
		}
		keys = append(keys, strings.TrimSpace(string(data)))
	}
	return keys, nil
}
