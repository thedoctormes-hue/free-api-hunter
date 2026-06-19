package filter

import (
	"log"
	"regexp"
	"strings"

	"free-api-hunter/internal/models"
)

var logger = log.New(log.Writer(), "[filter] ", log.LstdFlags)

// Engine — фильтр мусора с дедупликацией
type Engine struct {
	SpamPattern        *regexp.Regexp
	ExcludeDomains     map[string]bool
	ExcludedProviders  map[string]bool
	MinDescLength      int
	RequireURL         bool
	seenFingerprints   map[string]bool
}

// NewEngine — создать фильтр с настройками по умолчанию
func NewEngine() *Engine {
	// Спам-паттерн: явный спам и рекламный текст
	pattern := regexp.MustCompile(`(?i)купить\s+сейчас|продать\s+|скидка\s+\d+%|рефералка|affiliate\s+link|click\s+here\s+to\s+buy|купить сейчас|скидка!|специальное предложение`)

	return &Engine{
		SpamPattern:    pattern,
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
		MinDescLength: 30,
		RequireURL:    false,
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

	// 5. Спам-паттерн
	if e.SpamPattern.MatchString(f.RawText) {
		f.FilteredOut = true
		f.FilterReason = "spam_pattern"
		return
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
