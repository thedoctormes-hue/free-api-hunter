package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/orex"
)

// DataDir — директория для данных
var DataDir = "data"

// EnsureDir — создать директорию если не существует
func EnsureDir() error {
	return os.MkdirAll(DataDir, 0755)
}

// meta — общая структура метаданных
type meta struct {
	Version string `json:"version"`
	Updated string `json:"updated"`
	Count   int    `json:"count"`
}

// SaveProviders — сохранить провайдеров в JSON
func SaveProviders(providers []*models.Provider, path string) error {
	if err := EnsureDir(); err != nil {
		return err
	}
	if path == "" {
		path = filepath.Join(DataDir, "providers.json")
	}

	data := struct {
		Meta      meta                `json:"_meta"`
		Providers []*models.Provider  `json:"providers"`
	}{
		Meta: meta{
			Version: "0.1.0",
			Updated: time.Now().UTC().Format(time.RFC3339),
			Count:   len(providers),
		},
		Providers: providers,
	}

	return writeJSON(path, data)
}

// LoadProviders — загрузить провайдеров из JSON
func LoadProviders(path string) ([]*models.Provider, error) {
	if path == "" {
		path = filepath.Join(DataDir, "providers.json")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var wrapper struct {
		Providers []*models.Provider `json:"providers"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Providers, nil
}

// SaveFindings — сохранить находки в JSON
func SaveFindings(findings []*models.Finding, path string) error {
	if err := EnsureDir(); err != nil {
		return err
	}
	if path == "" {
		path = filepath.Join(DataDir, "findings.json")
	}

	data := struct {
		Meta     meta              `json:"_meta"`
		Findings []*models.Finding `json:"findings"`
	}{
		Meta: meta{
			Version: "0.1.0",
			Updated: time.Now().UTC().Format(time.RFC3339),
			Count:   len(findings),
		},
		Findings: findings,
	}

	return writeJSON(path, data)
}

// LoadFindings — загрузить находки из JSON
func LoadFindings(path string) ([]*models.Finding, error) {
	if path == "" {
		path = filepath.Join(DataDir, "findings.json")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var wrapper struct {
		Findings []*models.Finding `json:"findings"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Findings, nil
}

// SaveKeyPool — сохранить пул ключей в JSON
func SaveKeyPool(keys []*models.APIKey, path string) error {
	if err := EnsureDir(); err != nil {
		return err
	}
	if path == "" {
		path = filepath.Join(DataDir, "key_pool.json")
	}

	activeCount := 0
	for _, k := range keys {
		if k.IsActive {
			activeCount++
		}
	}

	data := struct {
		Meta       meta             `json:"_meta"`
		Keys       []*models.APIKey `json:"keys"`
	}{
		Meta: meta{
			Version:      "0.1.0",
			Updated:      time.Now().UTC().Format(time.RFC3339),
			Count:        len(keys),
		},
		Keys: keys,
	}
	_ = activeCount

	return writeJSON(path, data)
}

// LoadKeyPool — загрузить пул ключей из JSON
func LoadKeyPool(path string) ([]*models.APIKey, error) {
	if path == "" {
		path = filepath.Join(DataDir, "key_pool.json")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var wrapper struct {
		Keys []*models.APIKey `json:"keys"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Keys, nil
}

func writeJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(data)
}

// ============================================================
// Orex integration
// ============================================================

// OrexCache — кэш данных от Orex
type OrexCache struct {
	Meta       meta              `json:"_meta"`
	FreeModels []orex.FreeModel  `json:"free_models"`
	Alerts     []orex.OrexAlert  `json:"alerts"`
}

// SaveOrexCache — сохранить кэш Orex
func SaveOrexCache(cache *OrexCache, path string) error {
	if err := EnsureDir(); err != nil {
		return err
	}
	if path == "" {
		path = filepath.Join(DataDir, "orex_cache.json")
	}
	cache.Meta = meta{
		Version: "0.1.0",
		Updated: time.Now().UTC().Format(time.RFC3339),
		Count:   len(cache.FreeModels),
	}
	return writeJSON(path, cache)
}

// LoadOrexCache — загрузить кэш Orex
func LoadOrexCache(path string) (*OrexCache, error) {
	if path == "" {
		path = filepath.Join(DataDir, "orex_cache.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache OrexCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// MergeOrexProviders — объединить бесплатные модели Orex с локальными провайдерами
func MergeOrexProviders(existing []*models.Provider, freeModels []orex.FreeModel) []*models.Provider {
	index := make(map[string]int)
	for i, p := range existing {
		index[p.Name] = i
	}

	now := models.Now()

	for _, fm := range freeModels {
		if idx, ok := index[fm.Provider]; ok {
			// Провайдер уже есть — добавляем модель если новая
			p := existing[idx]
			modelExists := false
			for _, m := range p.Models {
				if m == fm.Name {
					modelExists = true
					break
				}
			}
			if !modelExists {
				p.Models = append(p.Models, fm.Name)
			}
			if p.Status == models.StatusUnverified {
				p.Status = models.StatusClaimed
			}
		} else {
			// Новый провайдер из Orex
			existing = append(existing, &models.Provider{
				Name:         fm.Provider,
				URL:          "https://openrouter.ai",
				APIKeyURL:    "https://openrouter.ai/keys",
				CreditCard:   false,
				Status:       models.StatusClaimed,
				Models:       []string{fm.Name},
				Source:       "orex",
				Priority:     models.PriorityMed,
				DiscoveredAt: now,
			})
			index[fm.Provider] = len(existing) - 1
		}
	}

	return existing
}
