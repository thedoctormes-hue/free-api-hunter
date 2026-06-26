package tts

import (
	"strings"
	"time"

	"free-api-hunter/internal/models"
)

// ScoreTTSProvider — оценить TTS-провайдера по 4 метрикам:
// - FreeTierScore: наличие и щедрость бесплатного тира
// - FeatureScore: уникальные фичи (audio_tags, voice_cloning, realtime, etc.)
// - LanguageScore: количество языков, наличие русского
// - LatencyScore: скорость (v3 = медленно, Flash = быстро, Realtime = мгновенно)
func ScoreTTSProvider(provider *models.TTSProvider, hasActiveKey bool) *models.TTSScore {
	score := &models.TTSScore{
		ProviderName: provider.Name,
		ScoredAt:     time.Now().UTC().Format(time.RFC3339),
		HasFreeTier:  provider.FreeTier != nil,
	}

	// 1. Free Tier Score
	score.FreeTierScore = calculateFreeTierScore(provider, hasActiveKey)

	// 2. Feature Score
	score.FeatureScore = calculateFeatureScore(provider)

	// 3. Language Score
	score.LanguageScore = calculateLanguageScore(provider)

	// 4. Latency Score
	score.LatencyScore = calculateLatencyScore(provider)

	// Overall — взвешенное среднее
	score.OverallScore =
		score.FreeTierScore*0.30 +
			score.FeatureScore*0.30 +
			score.LanguageScore*0.15 +
			score.LatencyScore*0.25

	if provider.FreeTier != nil {
		score.CharLimit = provider.FreeTier.CharLimit
	}

	return score
}

func calculateFreeTierScore(provider *models.TTSProvider, hasActiveKey bool) float64 {
	if !hasActiveKey {
		// Нет ключа — оцениваем только по заявленному фритиру
		if provider.FreeTier == nil {
			return 0.1
		}
		return scoreCharLimit(provider.FreeTier.CharLimit)
	}

	// Есть ключ и верификация прошла
	if provider.FreeTier != nil && provider.FreeTier.CharLimit > 0 {
		base := scoreCharLimit(provider.FreeTier.CharLimit)
		// Бонус за активных ключ
		return min(base+0.1, 1.0)
	}

	return 0.1
}

func scoreCharLimit(chars int) float64 {
	switch {
	case chars >= 50000:
		return 1.0
	case chars >= 10000:
		return 0.8
	case chars >= 1000:
		return 0.5
	case chars > 0:
		return 0.3
	default:
		return 0.0
	}
}

func calculateFeatureScore(provider *models.TTSProvider) float64 {
	if len(provider.Features) == 0 {
		return 0.3 // нет данных — средняя оценка
	}

	// Уникальные фичи с весами
	weights := map[string]float64{
		"audio_tags":         0.25, // уникально для ElevenLabs
		"voice_cloning":      0.20, // сложно реализовать
		"pronunciation_dicts": 0.15, // редко у конкурентов
		"multi_speaker":      0.15, // редко
		"realtime":           0.15, // важно для агентов
		"multilingual":       0.05, // почти у всех
		"streaming":          0.05, // почти у всех
	}

	score := 0.0
	for _, feature := range provider.Features {
		if w, ok := weights[feature]; ok {
			score += w
		} else {
			score += 0.01 // неизвестная фича
		}
	}

	return min(score, 1.0)
}

func calculateLanguageScore(provider *models.TTSProvider) float64 {
	if len(provider.Languages) == 0 {
		return 0.2
	}

	// Бонус за русский язык
	hasRussian := false
	for _, lang := range provider.Languages {
		lower := strings.ToLower(lang)
		if lower == "ru" || lower == "russian" || lower == "rh" || strings.Contains(lower, "русск") {
			hasRussian = true
			break
		}
	}

	// Бонус за большое количество языков
	count := len(provider.Languages)

	switch {
	case hasRussian && count >= 20:
		return 1.0
	case hasRussian && count >= 5:
		return 0.8
	case hasRussian:
		return 0.6
	case count >= 20:
		return 0.7
	case count >= 10:
		return 0.5
	case count >= 3:
		return 0.3
	default:
		return 0.2
	}
}

func calculateLatencyScore(provider *models.TTSProvider) float64 {
	// Определяем latency по списку моделей
	if len(provider.Models) == 0 {
		return 0.5
	}

	// Если есть real-time модель — максимум
	for _, m := range provider.Models {
		lower := strings.ToLower(m)
		if strings.Contains(lower, "realtime") {
			return 1.0
		}
	}

	// Flash/Turbo = быстрые
	for _, m := range provider.Models {
		lower := strings.ToLower(m)
		if strings.Contains(lower, "flash") || strings.Contains(lower, "turbo") {
			return 0.9
		}
	}

	// Multilingual v2 = средние
	hasMultilingual := false
	hasV3 := false
	for _, m := range provider.Models {
		lower := strings.ToLower(m)
		if strings.Contains(lower, "multilingual") {
			hasMultilingual = true
		}
		if strings.Contains(lower, "v3") || strings.Contains(lower, "eleven_v3") {
			hasV3 = true
		}
	}

	if hasMultilingual {
		return 0.6
	}
	if hasV3 {
		return 0.4
	}

	return 0.5
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
