package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"free-api-hunter/internal/alerter"
	"free-api-hunter/internal/api"
	"free-api-hunter/internal/filter"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/ocr"
	"free-api-hunter/internal/pollinations"
	"free-api-hunter/internal/scraper"
	"free-api-hunter/internal/storage"
	"free-api-hunter/internal/tts"
	"free-api-hunter/internal/verifier"
)

var logger = log.New(os.Stderr, "[hunter] ", log.LstdFlags)

// Version — устанавливается через ldflags при сборке
// go build -ldflags "-X main.Version=$(git describe --tags --always)" -o hunter cmd/hunter/main.go
var Version = "dev"

// Config — загруженная конфигурация источников
type Config struct {
	Sources   []scraper.SourceConfig `json:"sources"`
	Providers []ProviderConfig       `json:"provider_pages"`
}

// FilterConfig — конфигурация фильтров из filters.json
type FilterConfig struct {
	ExcludedProviders []string          `json:"excluded_providers"`
	SpamFilters       SpamFilterConfig  `json:"spam_filters"`
	QualityThreshold  QualityConfig     `json:"quality_threshold"`
	Dedup             DedupConfig       `json:"dedup"`
}

// SpamFilterConfig — спам-фильтры
type SpamFilterConfig struct {
	ExcludeDomains        []string `json:"exclude_domains"`
	ExcludeKeywords       []string `json:"exclude_keywords"`
	ExcludeCreditCard     bool     `json:"exclude_credit_card_required"`
	ExcludeTrashSources   []string `json:"exclude_trash_sources"`
}

// QualityConfig — пороги качества
type QualityConfig struct {
	MinDescLength    int  `json:"min_description_length"`
	RequireURL       bool `json:"require_url"`
	ExcludeExpired   bool `json:"exclude_expired"`
	MaxAgeDays       int  `json:"max_age_days"`
}

// DedupConfig — настройки дедупликации
type DedupConfig struct {
	Enabled           bool     `json:"enabled"`
	TTLHours          int      `json:"ttl_hours"`
	CheckURLUnique    bool     `json:"check_url_uniqueness"`
}

// ProviderConfig — конфигурация страницы провайдера
type ProviderConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	CreditCard bool   `json:"credit_card"`
	Status     string `json:"status"`
}

func main() {
	dryRun := flag.Bool("dry-run", false, "Не сохранять результаты")
	source := flag.String("source", "", "Сканировать только конкретный источник")
	verify := flag.Bool("verify", false, "Верифицировать провайдеров")
	limit := flag.Int("limit", 10, "Лимит находок для вывода")
	noAlerts := flag.Bool("no-alerts", false, "Не отправлять алерты в Telegram")
	alertConfigPath := flag.String("alert-config", "config/alerter.json", "Путь к конфигу алертеров")
	showVersion := flag.Bool("version", false, "Показать версию и выйти")
	apiAddr := flag.String("api", "", "Запустить HTTP API сервер на указанном адресе (напр. :8080)")
	flag.Parse()

	// API сервер
	if *apiAddr != "" {
		go func() {
			server := api.NewServer(*apiAddr)
			if err := server.ListenAndServe(); err != nil {
				logger.Fatalf("API server error: %v", err)
			}
		}()
		logger.Printf("API server started on %s", *apiAddr)
		// Блокируем main чтобы сервер не завершился
		select {}
	}

	if *showVersion {
		fmt.Printf("Free API Hunter %s\n", Version)
		os.Exit(0)
	}

	logger.Printf("Free API Hunter %s starting...", Version)

	// 1. Загружаем конфиг источников
	config, err := loadConfig("config/sources.json")
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// 2. Сканируем источники
	sources := config.Sources
	if *source != "" {
		var filtered []scraper.SourceConfig
		for _, s := range sources {
			if s.ID == *source {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			logger.Fatalf("Source %s not found or disabled", *source)
		}
		sources = filtered
	}

	logger.Println("Running scraper...")
	rawFindings := scraper.RunScraper(sources)

	// 3. Загружаем фильтры
	filterConfig := loadFilterConfig("config/filters.json")

	// 4. Фильтруем мусор
	engine := filter.NewEngine()
	engine.ApplyConfig(filter.FilterConfigData{
		ExcludedProviders: filterConfig.ExcludedProviders,
		SpamDomains:       filterConfig.SpamFilters.ExcludeDomains,
		SpamKeywords:      filterConfig.SpamFilters.ExcludeKeywords,
		TrashSources:      filterConfig.SpamFilters.ExcludeTrashSources,
		MinDescLength:     filterConfig.QualityThreshold.MinDescLength,
		RequireURL:        filterConfig.QualityThreshold.RequireURL,
		ExcludeExpired:    filterConfig.QualityThreshold.ExcludeExpired,
		MaxAgeDays:        filterConfig.QualityThreshold.MaxAgeDays,
		CheckURLUnique:    filterConfig.Dedup.CheckURLUnique,
	})
	findings := engine.FilterFindings(rawFindings)

	// 5. Загружаем/верифицируем провайдеров
	providers := loadInitialProviders(config)
	if *verify {
		logger.Printf("Verifying %d providers...", len(providers))
		for _, p := range providers {
			result := verifier.VerifyProviderPage(p)
			p.LastVerified = &result.CheckedAt
			if result.URLAlive && result.FreeTierMentioned && (result.CreditCardReq == nil || !*result.CreditCardReq) {
				p.Status = models.StatusConfirmed
			} else if result.URLAlive {
				p.Status = models.StatusClaimed
			} else {
				p.Status = models.StatusExpired
			}
		}
	}

	// 6. Выводим результаты
	printResults(rawFindings, findings, providers, *limit)

	// 6b. Pollinations pipeline — верификация и обогащение моделей
	if *verify {
		logger.Println("Testing Pollinations models...")
		pollInfo, pollResults := pollinations.TestAllModels()
		if pollInfo != nil {
			pollProvider := pollinations.ToProvider(pollInfo)
			// Обновляем или добавляем провайдера
			updated := false
			for i, p := range providers {
				if p.Name == pollProvider.Name {
					providers[i] = pollProvider
					updated = true
					break
				}
			}
			if !updated {
				providers = append(providers, pollProvider)
			}
			freeCount := len(pollInfo.ModelsFree)
			paidCount := len(pollInfo.ModelsPaid)
			logger.Printf("Pollinations: %d free, %d paid models", freeCount, paidCount)
			_ = pollResults
		}

		// Image generation test
		ok, msg := pollinations.VerifyImageGeneration()
		if ok {
			logger.Println("Pollinations image generation: ✅")
		} else {
			logger.Printf("Pollinations image generation: %s", msg)
		}
	}

	// 6c. Сохраняем результаты (включая Pollinations)
	if !*dryRun {
		logger.Println("Saving results...")
		if err := storage.SaveProviders(providers, ""); err != nil {
			logger.Printf("Failed to save providers: %v", err)
		}
		var findingsPtr []*models.Finding
		for i := range findings {
			findingsPtr = append(findingsPtr, &findings[i])
		}
		if err := storage.SaveFindings(findingsPtr, ""); err != nil {
			logger.Printf("Failed to save findings: %v", err)
		}
		logger.Println("Results saved to data/")
	} else {
		logger.Println("Dry run — results not saved")
	}

	logger.Println("Scan completed.")

	// 7. OCR pipeline — верификация и скоринг OCR-провайдеров
	if *verify {
		logger.Println("Running OCR pipeline...")
		runOCRPipeline(*noAlerts, *alertConfigPath)
	}

	// 8. TTS pipeline — верификация и скоринг TTS-провайдеров
	if *verify {
		logger.Println("Running TTS pipeline...")
		runTTSPipeline(*noAlerts, *alertConfigPath)
	}

	// 9. Отправляем алерт (если не отключён)
	if !*noAlerts {
		alertCfg, err := alerter.LoadConfig(*alertConfigPath)
		if err != nil {
			logger.Printf("Alert config not found (%v), skipping alerts", err)
		} else {
			report := alerter.FormatScanReport(len(rawFindings), len(findings), nil)
			if err := alerter.SendTelegram(alertCfg, report); err != nil {
				logger.Printf("Failed to send alert: %v", err)
			} else {
				logger.Println("Alert sent to Telegram")
			}
		}
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadFilterConfig(path string) FilterConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Printf("Filter config not found (%v), using defaults", err)
		return FilterConfig{}
	}
	var cfg FilterConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		logger.Printf("Failed to parse filter config (%v), using defaults", err)
		return FilterConfig{}
	}
	return cfg
}

func loadInitialProviders(config *Config) []*models.Provider {
	// 1. Загружаем config-провайдеров как baseline (source of truth для статусов)
	configProviders := make(map[string]*models.Provider)
	for _, p := range config.Providers {
		status := models.ProviderStatus(p.Status)
		if status == "" {
			status = models.StatusClaimed
		}
		configProviders[p.Name] = &models.Provider{
			Name:       p.Name,
			URL:        p.URL,
			APIKeyURL:  p.URL,
			CreditCard: p.CreditCard,
			Status:     status,
		}
	}

	// 2. Пробуем загрузить runtime-данные из файла
	runtimeProviders, err := storage.LoadProviders("")
	if err != nil || len(runtimeProviders) == 0 {
		// Нет runtime-данных — возвращаем config как есть
		var result []*models.Provider
		for _, p := range configProviders {
			if p.DiscoveredAt == "" {
				p.DiscoveredAt = models.Now()
			}
			result = append(result, p)
		}
		logger.Printf("Loaded %d providers from config (no runtime data)", len(result))
		return result
	}

	// 3. Мержим: runtime-данные (URL, модели, лимиты) + config-статусы
	var result []*models.Provider
	seen := make(map[string]bool)

	for _, rp := range runtimeProviders {
		seen[rp.Name] = true
		if cp, ok := configProviders[rp.Name]; ok {
			// Провайдер есть и в config, и в runtime
			// Статус из config (source of truth), остальное из runtime
			rp.Status = cp.Status
			rp.CreditCard = cp.CreditCard
			// URL из config как более актуальный, если runtime пустой
			if rp.URL == "" {
				rp.URL = cp.URL
			}
			if rp.APIKeyURL == "" {
				rp.APIKeyURL = cp.URL
			}
		} else {
			// Провайдер есть только в runtime (новый из Orex и т.д.)
			// Оставляем как есть
		}
		result = append(result, rp)
	}

	// 4. Добавляем провайдеров из config, которых нет в runtime
	for name, cp := range configProviders {
		if !seen[name] {
			if cp.DiscoveredAt == "" {
				cp.DiscoveredAt = models.Now()
			}
			result = append(result, cp)
		}
	}

	logger.Printf("Loaded %d providers (%d runtime + %d config-only, %d merged)",
		len(result), len(runtimeProviders), len(result)-len(runtimeProviders),
		len(runtimeProviders)+len(configProviders)-len(result))
	return result
}

func printResults(raw []models.Finding, filtered []models.Finding, providers []*models.Provider, limit int) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("FREE API HUNTER — РЕЗУЛЬТАТЫ")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Сырых находок: %d\n", len(raw))
	fmt.Printf("После фильтра: %d\n", len(filtered))
	fmt.Printf("Провайдеров в базе: %d\n", len(providers))
	fmt.Println(strings.Repeat("-", 60))

	// Топ находок
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].QualityScore > filtered[j].QualityScore
	})

	topN := limit
	if topN > len(filtered) {
		topN = len(filtered)
	}
	fmt.Printf("\nТоп %d находок:\n", topN)
	for i := 0; i < topN; i++ {
		f := filtered[i]
		fmt.Printf("%d. [%.2f] %s\n", i+1, f.QualityScore, f.Title)
		fmt.Printf("   Источник: %s\n", f.SourceID)
		fmt.Printf("   URL: %s\n", f.URL)
		desc := f.Description
		if len(desc) > 150 {
			desc = desc[:150] + "..."
		}
		fmt.Printf("   Описание: %s\n", desc)
		fmt.Println()
	}

	// Провайдеры по приоритету (verified + confirmed, без credit card)
	var highPri []*models.Provider
	for _, p := range providers {
		if (p.Status == models.StatusVerified || p.Status == models.StatusConfirmed) && !p.CreditCard {
			highPri = append(highPri, p)
		}
	}
	sort.Slice(highPri, func(i, j int) bool {
		return highPri[i].Name < highPri[j].Name
	})

	fmt.Printf("\nПодтверждённых бесплатных провайдеров: %d\n", len(highPri))
	if len(highPri) == 0 {
		fmt.Println("  Нет подтверждённых провайдеров.")
	}
	for _, p := range highPri {
		modelsStr := "N/A"
		if len(p.Models) > 0 {
			modelsStr = strings.Join(p.Models, ", ")
		}
		limitsStr := p.Limits
		if limitsStr == "" {
			limitsStr = "не указаны"
		}
		fmt.Printf("  • %s\n", p.Name)
		fmt.Printf("    Модели: %s\n", modelsStr)
		fmt.Printf("    Лимиты: %s\n", limitsStr)
		fmt.Printf("    URL: %s\n", p.URL)
		fmt.Println()
	}
}

// runTTSPipeline — полный pipeline для TTS-провайдеров: верификация → скоринг → алерт
func runTTSPipeline(noAlerts bool, alertConfigPath string) {
	// 0. Инициализируем пул ключей с ротацией
	if err := tts.InitKeyPool("config/tts_sources.json"); err != nil {
		logger.Printf("TTS keypool init failed (%v), skipping TTS pipeline", err)
		return
	}

	// 1. Загружаем конфиг TTS-провайдеров
	ttsProviders, err := tts.LoadTTSSources("config/tts_sources.json")
	if err != nil {
		logger.Printf("TTS config not found (%v), skipping TTS pipeline", err)
		return
	}
	logger.Printf("TTS: loaded %d providers from config", len(ttsProviders))

	// 2. Верифицируем каждого провайдера
	var verifyResults []*models.TTSVerifyResult
	var scores []*models.TTSScore
	for _, p := range ttsProviders {
		logger.Printf("TTS: verifying %s...", p.Name)
		result := tts.VerifyTTSKey(p)
		verifyResults = append(verifyResults, result)

		if result.IsActive {
			logger.Printf("TTS: %s ✅ active (plan: %s, chars: %d)",
				p.Name, result.Plan, result.CharLimit)
		} else {
			logger.Printf("TTS: %s ❌ inactive: %s", p.Name, result.Error)
		}

		// 3. Скоринг
		score := tts.ScoreTTSProvider(p, result.IsActive)
		scores = append(scores, score)
		logger.Printf("TTS: %s score: %.0f%% (free:%.0f feat:%.0f lang:%.0f latency:%.0f)",
			p.Name, score.OverallScore*100, score.FreeTierScore*100,
			score.FeatureScore*100, score.LanguageScore*100, score.LatencyScore*100)
	}

	// 4. Сохраняем данные и состояние пула
	if len(ttsProviders) > 0 {
		saveTTSData(ttsProviders, verifyResults, scores)
	}
	if kp, ok := tts.GetKeyPool(); ok {
		kp.SaveState("data/tts_keypool_state.json")
		stats := kp.Stats()
		logger.Printf("TTS keypool: %d active, %d exhausted", stats["active"], stats["exhausted"])
	}

	// 5. Вывод в stdout
	fmt.Println()
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println("TTS PROVIDERS")
	fmt.Println(strings.Repeat("─", 40))
	for i, r := range verifyResults {
		status := "❌"
		if r.IsActive {
			status = "✅"
		}
		p := ttsProviders[i]
		fmt.Printf("%s %s — %s\n", status, p.Name, r.Plan)
		if r.IsActive {
			fmt.Printf("   Voices: %d | Chars: %d/month\n", len(r.Voices), r.CharLimit)
			if i < len(scores) {
				fmt.Printf("   Score: %.0f%%\n", scores[i].OverallScore*100)
			}
		} else if r.Error != "" {
			fmt.Printf("   Error: %s\n", r.Error)
		}
	}
	fmt.Println(strings.Repeat("─", 40))

	// 6. Алерт
	if !noAlerts && len(scores) > 0 {
		alertCfg, err := alerter.LoadConfig(alertConfigPath)
		if err != nil {
			logger.Printf("TTS alert: config not found (%v), skipping", err)
		} else {
			report := alerter.FormatTTSReport(verifyResults, scores, ttsProviders)
			if err := alerter.SendTelegram(alertCfg, report); err != nil {
				logger.Printf("TTS alert failed: %v", err)
			} else {
				logger.Println("TTS report alert sent")
			}
		}
	}
}

// saveTTSData — сохранить TTS-провайдеров в JSON
func saveTTSData(providers []*models.TTSProvider, results []*models.TTSVerifyResult, scores []*models.TTSScore) {
	type ttsData struct {
		Providers []*models.TTSProvider     `json:"providers"`
		Results   []*models.TTSVerifyResult `json:"verify_results"`
		Scores    []*models.TTSScore        `json:"scores"`
		UpdatedAt string                `json:"updated_at"`
	}

	data := ttsData{
		Providers: providers,
		Results:   results,
		Scores:    scores,
		UpdatedAt: models.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.Printf("TTS: failed to marshal: %v", err)
		return
	}

	if err := os.WriteFile("data/tts_providers.json", jsonData, 0644); err != nil {
		logger.Printf("TTS: failed to save: %v", err)
	} else {
		logger.Println("TTS: saved to data/tts_providers.json")
	}
}

// runOCRPipeline — полный pipeline для OCR-провайдеров: верификация → скоринг → алерт
func runOCRPipeline(noAlerts bool, alertConfigPath string) {
	ocrProviders := []struct {
		name   string
		engine int
		lang   string
	}{
		{"free-api-hunter/ocr-space", 1, "eng"},
		{"free-api-hunter/ocr-space", 2, "eng"},
		{"free-api-hunter/ocr-space", 3, "eng"},
		{"free-api-hunter/ocr-space", 1, "rus"},
	}

	var verifyResults []*ocr.OCRVerifyResult
	var testResults []*ocr.OCRTestResult
	activeCount := 0

	// Шаг 1: Верификация ключа
	logger.Println("Step 1: Verifying OCR keys...")
	for _, p := range ocrProviders {
		// Сначала быстрая проверка ключа
		simpleResult := ocr.CheckOCRKeySimple(p.name)
		if !simpleResult.IsActive {
			logger.Printf("OCR key for %s (engine %d, %s): INACTIVE — %s",
				p.name, p.engine, p.lang, simpleResult.Error)
			verifyResults = append(verifyResults, simpleResult)
			continue
		}

		// Полная верификация с тестовым изображением
		result := ocr.VerifyOCRKey(p.name, p.engine, p.lang)
		verifyResults = append(verifyResults, result)
		if result.IsActive {
			activeCount++
			testResults = append(testResults, &ocr.OCRTestResult{
				Engine:       p.engine,
				Language:     p.lang,
				Success:      true,
				Text:         result.RecognizedText,
				ProcessingMs: result.ProcessingMs,
			})
		}
	}

	// Шаг 2: Скоринг
	logger.Printf("Step 2: Scoring OCR providers... (%d/%d active)", activeCount, len(ocrProviders))
	score := ocr.ScoreOCRProvider("OCR.space", testResults, "25,000 requests/month, 500/day/IP")
	logger.Printf("OCR Score for OCR.space: %.0f%%", score.OverallScore*100)

	// Вывод в stdout
	fmt.Println()
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println("OCR PROVIDERS")
	fmt.Println(strings.Repeat("─", 40))
	for _, r := range verifyResults {
		status := "❌"
		if r.IsActive {
			status = "✅"
		}
		fmt.Printf("%s Engine %d (%s) — %s\n",
			status, r.EngineUsed, r.Language, r.ProcessingMs)
	}
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("Overall Score: %.0f%%\n", score.OverallScore*100)

	// Шаг 3: Алерт
	if !noAlerts {
		alertCfg, err := alerter.LoadConfig(alertConfigPath)
		if err != nil {
			logger.Printf("OCR alert: config not found (%v), skipping", err)
		} else {
			scoreReport := ocr.FormatOCRScoreReport(score)
			if err := alerter.SendTelegram(alertCfg, scoreReport); err != nil {
				logger.Printf("OCR alert failed: %v", err)
			} else {
				logger.Println("OCR score alert sent")
			}
		}
	}
}
