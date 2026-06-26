package models

import (
	"fmt"
	"time"
)

// ProviderStatus — статус провайдера
type ProviderStatus string

const (
	StatusVerified   ProviderStatus = "verified"
	StatusConfirmed  ProviderStatus = "confirmed"
	StatusClaimed    ProviderStatus = "claimed"
	StatusExpired    ProviderStatus = "expired"
	StatusUnverified ProviderStatus = "unverified"
	StatusDeprioritized ProviderStatus = "deprioritized"
)

// Priority — приоритет
type Priority int

const (
	PriorityHigh Priority = 1
	PriorityMed  Priority = 2
	PriorityLow  Priority = 3
	PrioritySkip Priority = 9
)

// Provider — LLM API провайдер
type Provider struct {
	Name         string         `json:"name"`
	URL          string         `json:"url"`
	APIKeyURL    string         `json:"api_key_url"`
	CreditCard   bool           `json:"credit_card"`
	Status       ProviderStatus `json:"status"`
	Models       []string       `json:"models"`
	Limits       string         `json:"limits"`
	Notes        string         `json:"notes"`
	Source       string         `json:"source"`
	Priority     Priority       `json:"priority"`
	DiscoveredAt string         `json:"discovered_at"`
	LastVerified *string        `json:"last_verified,omitempty"`
}

// Finding — сырая находка из источника
type Finding struct {
	SourceID     string  `json:"source_id"`
	Title        string  `json:"title"`
	URL          string  `json:"url"`
	Description  string  `json:"description"`
	RawText      string  `json:"raw_text"`
	DiscoveredAt string  `json:"discovered_at"`
	ProviderName *string `json:"provider_name,omitempty"`
	IsDuplicate  bool    `json:"is_duplicate"`
	QualityScore float64 `json:"quality_score"`
	FilteredOut  bool    `json:"filtered_out"`
	FilterReason string  `json:"filter_reason"`
}

// Fingerprint — отпечаток для дедупликации
func (f *Finding) Fingerprint() string {
	key := f.Title + ":" + f.URL
	if f.ProviderName != nil {
		key = *f.ProviderName + ":" + f.URL
	}
	// FNV-1a like hash
	h := uint64(14695981039346656037)
	for _, c := range key {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}

// APIKey — рабочий API ключ
type APIKey struct {
	ProviderName string            `json:"provider_name"`
	KeyLocation  string            `json:"key_location"` // vault path, не сам ключ
	Endpoint     string            `json:"endpoint"`
	Models       []string          `json:"models"`
	Limits       map[string]string `json:"limits"`
	IsActive     bool              `json:"is_active"`
	LastChecked  *string           `json:"last_checked,omitempty"`
	CreatedAt    string            `json:"created_at"`
	Notes        string            `json:"notes"`
}

// OCRProvider — OCR-провайдер (отдельный тип от LLM-провайдеров)
type OCRProvider struct {
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	APIKeyURL       string   `json:"api_key_url"`
	CreditCard      bool     `json:"credit_card"`
	Status          ProviderStatus `json:"status"`
	Engines         []int    `json:"engines"`         // доступные движки (1, 2, 3)
	Languages       []string `json:"languages"`       // поддерживаемые языки
	FreeQuota       string   `json:"free_quota"`      // бесплатная квота
	MaxFileSize     string   `json:"max_file_size"`   // макс размер файла
	HasOverlay      bool     `json:"has_overlay"`     // bounding box overlay
	HasTableMode    bool     `json:"has_table_mode"`   // табличный режим
	HasSearchablePDF bool    `json:"has_searchable_pdf"`
	Notes           string   `json:"notes"`
	Source          string   `json:"source"`
	DiscoveredAt    string   `json:"discovered_at"`
	LastVerified    *string  `json:"last_verified,omitempty"`
}

// OCRKey — рабочий OCR API ключ
type OCRKey struct {
	ProviderName string            `json:"provider_name"`
	KeyLocation  string            `json:"key_location"` // vault path
	Endpoint     string            `json:"endpoint"`
	Engines      []int             `json:"engines"`
	Languages    []string          `json:"languages"`
	Limits       map[string]string `json:"limits"`
	IsActive     bool              `json:"is_active"`
	LastChecked  *string           `json:"last_checked,omitempty"`
	CreatedAt    string            `json:"created_at"`
	Notes        string            `json:"notes"`
}

// Now — текущее время в ISO формате
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
