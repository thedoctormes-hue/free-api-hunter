package storage

import (
	"os"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/orex"
)

// DataDir — директория для данных (для обратной совместимости)
var DataDir = "data"

// EnsureDir — создать директорию если не существует
func EnsureDir() error {
	return os.MkdirAll(DataDir, 0755)
}

// OrexCache — кэш данных от Orex
type OrexCache struct {
	Meta       meta              `json:"_meta"`
	FreeModels []orex.FreeModel  `json:"free_models"`
	Alerts     []orex.OrexAlert  `json:"alerts"`
}

// meta — общая структура метаданных
type meta struct {
	Version   string `json:"version"`
	Updated   string `json:"updated"`
	UpdatedAt string `json:"updated_at"`
	Count     int    `json:"count"`
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

