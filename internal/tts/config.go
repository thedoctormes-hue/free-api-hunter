package tts

import (
	"encoding/json"
	"fmt"
	"os"

	"free-api-hunter/internal/models"
)

// TTSSourceConfig — конфиг источника TTS-провайдеров из JSON
type TTSSourceConfig struct {
	Providers []TTSProviderConfig `json:"tts_providers"`
}

// TTSProviderConfig — один провайдер в конфиге
type TTSProviderConfig struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	APIKeyURL  string    `json:"api_key_url"`
	CreditCard bool      `json:"credit_card"`
	Status     string    `json:"status"`
	Models     []string  `json:"models"`
	Limits     string    `json:"limits"`
	FreeTier   *FreeTier `json:"free_tier,omitempty"`
	Features   []string  `json:"features"`
	Languages  []string  `json:"languages"`
	Source     string    `json:"source"`
	Priority   int       `json:"priority"`
	Notes      string    `json:"notes"`
}

type FreeTier struct {
	CharLimit   int    `json:"char_limit"`
	VoiceClones int    `json:"voice_clones"`
	ResetPeriod string `json:"reset_period"`
}

// LoadTTSSources — загрузить TTS-провайдеров из конфига
func LoadTTSSources(path string) ([]*models.TTSProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tts: cannot read config %s: %w", path, err)
	}

	var cfg TTSSourceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("tts: cannot parse config: %w", err)
	}

	var providers []*models.TTSProvider
	for _, pc := range cfg.Providers {
		p := &models.TTSProvider{
			Name:       pc.Name,
			URL:        pc.URL,
			APIKeyURL:  pc.APIKeyURL,
			CreditCard: pc.CreditCard,
			Status:     models.ProviderStatus(pc.Status),
			Models:     pc.Models,
			Limits:     pc.Limits,
			Features:   pc.Features,
			Languages:  pc.Languages,
			Source:     pc.Source,
			Priority:   models.Priority(pc.Priority),
			Notes:      pc.Notes,
		}

		if pc.FreeTier != nil {
			p.FreeTier = &models.FreeTierInfo{
				CharLimit:   pc.FreeTier.CharLimit,
				VoiceClones: pc.FreeTier.VoiceClones,
				ResetPeriod: pc.FreeTier.ResetPeriod,
			}
		}

		providers = append(providers, p)
	}

	return providers, nil
}
