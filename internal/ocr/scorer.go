package ocr

import (
	"log"
	"os"
	"strings"
	"time"
)

var scorerLogger = log.New(os.Stderr, "[ocr-scorer] ", log.LstdFlags)

// OCRScore — оценка качества OCR-провайдера
type OCRScore struct {
	ProviderName   string  `json:"provider_name"`
	OverallScore   float64 `json:"overall_score"` // 0.0 - 1.0
	SpeedScore     float64 `json:"speed_score"`   // на основе времени обработки
	QualityScore   float64 `json:"quality_score"` // на основе точности распознавания
	FeatureScore   float64 `json:"feature_score"` // на основе поддерживаемых фич
	ValueScore     float64 `json:"value_score"`   // на основе бесплатного тира
	EnginesCount   int     `json:"engines_count"`
	LanguagesCount int     `json:"languages_count"`
	HasFreeTier    bool    `json:"has_free_tier"`
	FreeQuota      string  `json:"free_quota,omitempty"`
	ScoredAt       string  `json:"scored_at"`
}

// ScoreOCRProvider — оценить OCR-провайдера по результатам тестирования
func ScoreOCRProvider(providerName string, testResults []*OCRTestResult, freeQuota string) *OCRScore {
	score := &OCRScore{
		ProviderName: providerName,
		ScoredAt:     time.Now().UTC().Format(time.RFC3339),
		HasFreeTier:  freeQuota != "",
		FreeQuota:    freeQuota,
	}

	// Подсчёт успешных движков
	successCount := 0
	var totalMs int64
	var successMs int64

	for _, tr := range testResults {
		if tr.Success {
			successCount++
			// Парсим время (формат "312" мс)
		}
		_ = totalMs
		_ = successMs
	}

	score.EnginesCount = successCount

	// Speed score: быстрые ответы = высокий балл
	score.SpeedScore = calculateSpeedScore(testResults)

	// Quality score: процент успешных распознаваний
	if len(testResults) > 0 {
		score.QualityScore = float64(successCount) / float64(len(testResults))
	}

	// Feature score: наличие фич (overlay, table, searchable PDF, multi-language)
	score.FeatureScore = calculateFeatureScore(providerName)

	// Value score: щедрость бесплатного тира
	score.ValueScore = calculateValueScore(freeQuota)

	// Overall score — взвешенное среднее
	score.OverallScore = score.SpeedScore*0.25 +
		score.QualityScore*0.30 +
		score.FeatureScore*0.20 +
		score.ValueScore*0.25

	scorerLogger.Printf("OCR Score for %s: overall=%.2f (speed=%.2f, quality=%.2f, features=%.2f, value=%.2f)",
		providerName, score.OverallScore, score.SpeedScore, score.QualityScore, score.FeatureScore, score.ValueScore)

	return score
}

// calculateSpeedScore — оценка скорости на основе результатов тестов
func calculateSpeedScore(results []*OCRTestResult) float64 {
	if len(results) == 0 {
		return 0.0
	}

	var totalMs float64
	var count int

	for _, r := range results {
		if r.Success && r.ProcessingMs != "" {
			var ms int
			if err := parseInt(r.ProcessingMs, &ms); err == nil {
				totalMs += float64(ms)
				count++
			}
		}
	}

	if count == 0 {
		return 0.5 // нет данных — средняя оценка
	}

	avgMs := totalMs / float64(count)

	// Менее 500мс = 1.0, 500-1000мс = 0.8, 1000-2000мс = 0.5, >2000мс = 0.2
	switch {
	case avgMs < 500:
		return 1.0
	case avgMs < 1000:
		return 0.8
	case avgMs < 2000:
		return 0.5
	default:
		return 0.2
	}
}

// calculateFeatureScore — оценка функциональности OCR-провайдера
func calculateFeatureScore(providerName string) float64 {
	score := 0.0
	nameLower := strings.ToLower(providerName)

	// OCR.space — известные фичи
	if strings.Contains(nameLower, "ocr.space") || strings.Contains(nameLower, "a9t9") {
		// 3 движка распознавания
		score += 0.3
		// 30+ языков
		score += 0.2
		// Bounding box overlay
		score += 0.1
		// Table mode
		score += 0.1
		// Searchable PDF
		score += 0.1
		// Auto orientation detection
		score += 0.1
		// Auto language detection (Engine 2/3)
		score += 0.1
	}

	return minFloat(score, 1.0)
}

// calculateValueScore — оценка ценности бесплатного тира
func calculateValueScore(freeQuota string) float64 {
	if freeQuota == "" {
		return 0.0
	}

	lower := strings.ToLower(freeQuota)

	// 25,000/мес — очень щедро
	if strings.Contains(lower, "25,000") || strings.Contains(lower, "25000") {
		return 1.0
	}
	// 10,000-24,999
	if strings.Contains(lower, "10,000") || strings.Contains(lower, "10000") {
		return 0.8
	}
	// 1,000-9,999
	if strings.Contains(lower, "1,000") || strings.Contains(lower, "1000") {
		return 0.5
	}
	// Менее 1,000
	if strings.Contains(lower, "free") {
		return 0.3
	}

	return 0.5
}

// parseInt — парсить строку в int (упрощённый)
func parseInt(s string, out *int) error {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	*out = result
	return nil
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
