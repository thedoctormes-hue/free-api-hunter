package tts

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"free-api-hunter/internal/vault"
)

// KeyEntry — один ключ в пуле
type KeyEntry struct {
	Key       string    `json:"key"`
	Provider  string    `json:"provider"`
	CharsUsed int       `json:"chars_used"`  // сколько символов потрачено
	CharsLimit int      `json:"chars_limit"` // месячный лимит
	Active    bool      `json:"active"`       // false = исчерпан/заблокирован
	LastUsed  string    `json:"last_used"`    // ISO timestamp
}

// KeyPool — потокобезопасный пул ключей с round-robin ротацией
type KeyPool struct {
	mu      sync.Mutex
	keys    []*KeyEntry
	current int // индекс текущего ключа
	loaded  string // когда загружен (для инвалидации)
}

// NewKeyPool — создать пул из конфига
func NewKeyPool(configPath string) (*KeyPool, error) {
	pool := &KeyPool{}
	if err := pool.load(configPath); err != nil {
		return nil, err
	}
	return pool, nil
}

// load — загрузить ключи из конфига + vault
func (p *KeyPool) load(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("keypool: cannot read config %s: %w", configPath, err)
	}

	var cfg struct {
		Providers []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			CharLimit int    `json:"char_limit"`
			// Ключи могут быть в конфиге (для совместимости)
			// или в vault (рекомендуется)
			Keys []string `json:"keys,omitempty"`
		} `json:"tts_providers"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("keypool: cannot parse config: %w", err)
	}

	p.keys = nil
	now := time.Now().UTC().Format(time.RFC3339)

	for _, prov := range cfg.Providers {
		// 1. Сначала из vault
		vaultKeys, err := loadVaultKeys(prov.ID)
		if err == nil && len(vaultKeys) > 0 {
			for _, k := range vaultKeys {
				p.keys = append(p.keys, &KeyEntry{
					Key:        k,
					Provider:   prov.ID,
					CharsUsed:  0,
					CharsLimit: prov.CharLimit,
					Active:     true,
					LastUsed:   now,
				})
			}
		}

		// 2. Затем из конфига (если есть)
		for _, k := range prov.Keys {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			p.keys = append(p.keys, &KeyEntry{
				Key:        k,
				Provider:   prov.ID,
				CharsUsed:  0,
				CharsLimit: prov.CharLimit,
				Active:     true,
				LastUsed:   now,
			})
		}
	}

	p.loaded = now
	p.current = 0
	return nil
}

// loadVaultKeys — загрузить все ключи провайдера из vault
// Ищет файлы: vault/free-api-hunter/<provider>/api.key, api.key.1, api.key.2, ...
func loadVaultKeys(providerID string) ([]string, error) {
	vaultDir := vault.VaultPath + "/free-api-hunter/" + providerID
	entries, err := os.ReadDir(vaultDir)
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// api.key, api.key.1, api.key.2, etc.
		if name == "api.key" || strings.HasPrefix(name, "api.key.") {
			data, err := os.ReadFile(vaultDir + "/" + name)
			if err != nil {
				continue
			}
			k := strings.TrimSpace(string(data))
			if k != "" {
				keys = append(keys, k)
			}
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys found in vault for %s", providerID)
	}
	return keys, nil
}

// Next — получить следующий активный ключ (round-robin)
// Если все ключи исчерпаны — возвращает ошибку
func (p *KeyPool) Next() (*KeyEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return nil, fmt.Errorf("keypool: no keys available")
	}

	// Ищем следующий активный ключ
	attempts := 0
	for attempts < len(p.keys) {
		entry := p.keys[p.current]
		p.current = (p.current + 1) % len(p.keys)

		if entry.Active {
			entry.LastUsed = time.Now().UTC().Format(time.RFC3339)
			return entry, nil
		}
		attempts++
	}

	return nil, fmt.Errorf("keypool: all keys exhausted for all providers")
}

// NextForProvider — получить ключ для конкретного провайдера
func (p *KeyPool) NextForProvider(providerID string) (*KeyEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return nil, fmt.Errorf("keypool: no keys available")
	}

	// Ищем активный ключ для провайдера
	for i := 0; i < len(p.keys); i++ {
		idx := (p.current + i) % len(p.keys)
		entry := p.keys[idx]
		if entry.Provider == providerID && entry.Active {
			p.current = (idx + 1) % len(p.keys)
			entry.LastUsed = time.Now().UTC().Format(time.RFC3339)
			return entry, nil
		}
	}

	return nil, fmt.Errorf("keypool: no active keys for provider %s", providerID)
}

// ReportUsage — отчитаться о расходе символов для ключа
func (p *KeyPool) ReportUsage(key string, chars int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, entry := range p.keys {
		if entry.Key == key {
			entry.CharsUsed += chars
			if entry.CharsLimit > 0 && entry.CharsUsed >= entry.CharsLimit {
				entry.Active = false
				keyPrefix := key
				if len(keyPrefix) > 8 {
					keyPrefix = keyPrefix[:8]
				}
				logger.Printf("keypool: key %s... exhausted (%d/%d chars)",
					keyPrefix, entry.CharsUsed, entry.CharsLimit)
			}
			return
		}
	}
}

// ReportError — пометить ключ как неактивный (ошибка 401, 403, etc.)
func (p *KeyPool) ReportError(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, entry := range p.keys {
		if entry.Key == key {
			entry.Active = false
			keyPrefix := key
			if len(keyPrefix) > 8 {
				keyPrefix = keyPrefix[:8]
			}
			logger.Printf("keypool: key %s... marked inactive (API error)", keyPrefix)
			return
		}
	}
}

// Stats — статистика пула
func (p *KeyPool) Stats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	active := 0
	exhausted := 0
	byProvider := map[string]map[string]int{}

	for _, entry := range p.keys {
		if entry.Active {
			active++
		} else {
			exhausted++
		}

		if _, ok := byProvider[entry.Provider]; !ok {
			byProvider[entry.Provider] = map[string]int{"active": 0, "exhausted": 0, "total_chars": 0}
		}
		byProvider[entry.Provider]["total_chars"] += entry.CharsUsed
		if entry.Active {
			byProvider[entry.Provider]["active"]++
		} else {
			byProvider[entry.Provider]["exhausted"]++
		}
	}

	return map[string]interface{}{
		"total_keys":   len(p.keys),
		"active":       active,
		"exhausted":    exhausted,
		"by_provider":  byProvider,
		"loaded_at":    p.loaded,
	}
}

// SaveState — сохранить состояние пула (использование) в файл
func (p *KeyPool) SaveState(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := json.MarshalIndent(p.keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Reload — перезагрузить пул из конфига (для добавления новых ключей)
func (p *KeyPool) Reload(configPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Сохраняем текущее использование
	usageMap := make(map[string]*KeyEntry)
	for _, entry := range p.keys {
		usageMap[entry.Key] = entry
	}

	if err := p.load(configPath); err != nil {
		return err
	}

	// Восстанавливаем использование для существующих ключей
	for _, entry := range p.keys {
		if prev, ok := usageMap[entry.Key]; ok {
			entry.CharsUsed = prev.CharsUsed
			entry.Active = prev.Active
		}
	}

	return nil
}
