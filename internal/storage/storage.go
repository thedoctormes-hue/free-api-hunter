package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"free-api-hunter/internal/models"
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
