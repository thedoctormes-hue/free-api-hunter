package filter

import (
	"log"
	"regexp"
	"strings"
	"time"

	"free-api-hunter/internal/models"
)

var logger = log.New(log.Writer(), "[filter] ", log.LstdFlags)

// Engine — фильтр мусора с дедупликацией
type Engine struct {
	SpamPattern         *regexp.Regexp
	ExcludeDomains      map[string]bool
	ExcludedProviders   map[string]bool
	ExcludeKeywords     []string
	ExcludeTrashSources map[string]bool
	MinDescLength       int
	RequireURL          bool
	ExcludeExpired      bool
	MaxAgeDays          int
	seenFingerprints    map[string]bool
	seenURLs            map[string]bool
}

// FilterConfigData — данные конфигурации фильтров для ApplyConfig
type FilterConfigData struct {
	ExcludedProviders []string
	SpamDomains       []string
	SpamKeywords      []string
	TrashSources      []string
	MinDescLength     int
	RequireURL        bool
	ExcludeExpired    bool
	MaxAgeDays        int
	CheckURLUnique    bool
}

// ApplyConfig — применить конфигурацию из filters.json
func (e *Engine) ApplyConfig(cfg FilterConfigData) {
	if len(cfg.ExcludedProviders) > 0 {
		e.ExcludedProviders = make(map[string]bool)
		for _, p := range cfg.ExcludedProviders {
			e.ExcludedProviders[strings.ToLower(p)] = true
		}
	}

	for _, d := range cfg.SpamDomains {
		e.ExcludeDomains[strings.ToLower(d)] = true
	}

	if len(cfg.SpamKeywords) > 0 {
		e.ExcludeKeywords = cfg.SpamKeywords
	}

	if len(cfg.TrashSources) > 0 {
		e.ExcludeTrashSources = make(map[string]bool)
		for _, s := range cfg.TrashSources {
			e.ExcludeTrashSources[strings.ToLower(s)] = true
		}
	}

	if cfg.MinDescLength > 0 {
		e.MinDescLength = cfg.MinDescLength
	}
	if cfg.RequireURL {
		e.RequireURL = true
	}
	if cfg.ExcludeExpired {
		e.ExcludeExpired = true
	}
	if cfg.MaxAgeDays > 0 {
		e.MaxAgeDays = cfg.MaxAgeDays
	}
	if cfg.CheckURLUnique {
		e.seenURLs = make(map[string]bool)
	}
}

// NewEngine — создать фильтр с настройками по умолчанию
func NewEngine() *Engine {
	// Спам-паттерн: явный спам и рекламный текст
	pattern := regexp.MustCompile(`(?i)купить\s+сейчас|продать\s+|скидка\s+\d+%|рефералка|affiliate\s+link|click\s+here\s+to\s+buy|купить сейчас|скидка!|специальное предложение`)

	return &Engine{
		SpamPattern: pattern,
		ExcludeDomains: map[string]bool{
			"medium.com":   true,
			"substack.com": true,
			"linkedin.com": true,
		},
		ExcludedProviders: map[string]bool{
			"kilo gateway": true,
			"kilochat":     true,
			"kilo":         true,
		},
		MinDescLength:    30,
		RequireURL:       false,
		seenFingerprints: make(map[string]bool),
	}
}

// FilterFindings — прогнать все находки через цепочку фильтров
func (e *Engine) FilterFindings(findings []models.Finding) []models.Finding {
	var results []models.Finding

	for i := range findings {
		e.applyFilters(&findings[i])
		if !findings[i].FilteredOut {
			results = append(results, findings[i])
		}
		_ = i
	}

	logger.Printf("FilterFindings: %d in, %d out, %d filtered",
		len(findings), len(results), len(findings)-len(results))
	return results
}

func (e *Engine) applyFilters(f *models.Finding) {
	// 1. Дедупликация
	fp := f.Fingerprint()
	if e.seenFingerprints[fp] {
		f.IsDuplicate = true
		f.FilteredOut = true
		f.FilterReason = "duplicate"
		return
	}
	e.seenFingerprints[fp] = true

	// 2. Минимальная длина описания
	if len(strings.TrimSpace(f.Description)) < e.MinDescLength {
		f.FilteredOut = true
		f.FilterReason = "too_short"
		return
	}

	// 3. Проверка URL
	if e.RequireURL && !strings.HasPrefix(f.URL, "http") {
		f.FilteredOut = true
		f.FilterReason = "no_url"
		return
	}

	// 4. Исключённые домены
	for domain := range e.ExcludeDomains {
		if strings.Contains(f.URL, domain) {
			f.FilteredOut = true
			f.FilterReason = "excluded_domain:" + domain
			return
		}
	}

	// 4a. Исключённые провайдеры
	if f.ProviderName != nil {
		nameLower := strings.ToLower(*f.ProviderName)
		for excluded := range e.ExcludedProviders {
			if strings.Contains(nameLower, excluded) {
				f.FilteredOut = true
				f.FilterReason = "excluded_provider:" + excluded
				return
			}
		}
	}

	// 4b. Исключённые trash-источники (pastebin, ghostbin, etc.)
	for source := range e.ExcludeTrashSources {
		if strings.Contains(f.URL, source) {
			f.FilteredOut = true
			f.FilterReason = "trash_source:" + source
			return
		}
	}

	// 4c. URL uniqueness check
	if e.seenURLs != nil {
		if e.seenURLs[f.URL] {
			f.FilteredOut = true
			f.FilterReason = "duplicate_url"
			return
		}
		e.seenURLs[f.URL] = true
	}

	// 5. Спам-паттерн
	if e.SpamPattern.MatchString(f.RawText) {
		f.FilteredOut = true
		f.FilterReason = "spam_pattern"
		return
	}

	// 5a. Exclude keywords (из filters.json spam_filters.exclude_keywords)
	if len(e.ExcludeKeywords) > 0 {
		textLower := strings.ToLower(f.RawText)
		for _, kw := range e.ExcludeKeywords {
			if strings.Contains(textLower, strings.ToLower(kw)) {
				f.FilteredOut = true
				f.FilterReason = "spam_keyword:" + kw
				return
			}
		}
	}

	// 5b. Expired age check
	if e.ExcludeExpired && e.MaxAgeDays > 0 && f.DiscoveredAt != "" {
		if t, err := time.Parse(time.RFC3339, f.DiscoveredAt); err == nil {
			if time.Since(t) > time.Duration(e.MaxAgeDays)*24*time.Hour {
				f.FilteredOut = true
				f.FilterReason = "expired_age"
				return
			}
		}
	}

	// 6. Оценка качества
	f.QualityScore = e.scoreQuality(f)
}

func (e *Engine) scoreQuality(f *models.Finding) float64 {
	score := 0.0

	// Длинное описание = больше информации
	descLen := len(f.Description)
	if descLen > 200 {
		score += 0.3
	} else if descLen > 100 {
		score += 0.2
	} else {
		score += 0.1
	}

	// Упоминание конкретных моделей
	modelKeywords := []string{"gpt", "claude", "gemini", "llama", "mistral", "mixtral",
		"qwen", "deepseek", "command", "gemma"}
	textLower := strings.ToLower(f.RawText)
	modelMatches := 0
	for _, m := range modelKeywords {
		if strings.Contains(textLower, m) {
			modelMatches++
		}
	}
	score += float64(min(modelMatches, 3)) * 0.1

	// Упоминание лимитов/условий
	limitKeywords := []string{"rpm", "tpm", "rpd", "free", "tier", "credit", "limit", "quota"}
	limitMatches := 0
	for _, k := range limitKeywords {
		if strings.Contains(textLower, k) {
			limitMatches++
		}
	}
	score += float64(min(limitMatches, 3)) * 0.1

	// Наличие URL на API-документацию
	if strings.Contains(f.URL, "docs") || strings.Contains(f.URL, "api") {
		score += 0.1
	}

	if score > 1.0 {
		return 1.0
	}
	return score
}

// AssignPriority — определить приоритет провайдера
func AssignPriority(p *models.Provider) models.Priority {
	if p.CreditCard {
		return models.PrioritySkip
	}
	switch p.Status {
	case models.StatusVerified:
		return models.PriorityHigh
	case models.StatusConfirmed:
		return models.PriorityHigh
	case models.StatusClaimed:
		return models.PriorityMed
	case models.StatusDeprioritized:
		return models.PriorityLow
	default:
		return models.PriorityLow
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
