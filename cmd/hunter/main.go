package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"free-api-hunter/internal/alerter"
	"free-api-hunter/internal/api"
	"free-api-hunter/internal/cf"
	"free-api-hunter/internal/filter"
	"free-api-hunter/internal/keydrop"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/notify"
	"free-api-hunter/internal/ocr"
	"free-api-hunter/internal/output"
	"free-api-hunter/internal/pollinations"
	"free-api-hunter/internal/scraper"
	"free-api-hunter/internal/storage"
	"free-api-hunter/internal/tts"
	"free-api-hunter/internal/verifier"
	"free-api-hunter/internal/validator"
)

var logger = log.New(os.Stderr, "[hunter] ", log.LstdFlags)

// Version — устанавливается через ldflags при сборке
// go build -ldflags "-X main.Version=$(git describe --tags --always)" -o hunter cmd/hunter/main.go
var Version = "dev"

// defaultPendingReviewPath — контрактный файл, который Мангуст читает в heartbeat.
// Не содержит секретов (только URL источников).
const defaultPendingReviewPath = "/root/LabDoctorM/projects/free-api-hunter/data/pending_review.json"

// Config — загруженная конфигурация источников
type Config struct {
	Sources   []scraper.SourceConfig `json:"sources"`
	Providers []ProviderConfig       `json:"provider_pages"`
}

// FilterConfig — конфигурация фильтров из filters.json
type FilterConfig struct {
	ExcludedProviders []string         `json:"excluded_providers"`
	SpamFilters       SpamFilterConfig `json:"spam_filters"`
	QualityThreshold  QualityConfig    `json:"quality_threshold"`
	Dedup             DedupConfig      `json:"dedup"`
}

// SpamFilterConfig — спам-фильтры
type SpamFilterConfig struct {
	ExcludeDomains      []string `json:"exclude_domains"`
	ExcludeKeywords     []string `json:"exclude_keywords"`
	ExcludeCreditCard   bool     `json:"exclude_credit_card_required"`
	ExcludeTrashSources []string `json:"exclude_trash_sources"`
}

// QualityConfig — пороги качества
type QualityConfig struct {
	MinDescLength  int  `json:"min_description_length"`
	RequireURL     bool `json:"require_url"`
	ExcludeExpired bool `json:"exclude_expired"`
	MaxAgeDays     int  `json:"max_age_days"`
}

// DedupConfig — настройки дедупликации
type DedupConfig struct {
	Enabled        bool `json:"enabled"`
	TTLHours       int  `json:"ttl_hours"`
	CheckURLUnique bool `json:"check_url_uniqueness"`
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
	// Subcommand dispatch — must run before flag.Parse so the leading
	// subcommand token is stripped before the default flag set is parsed.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "notify":
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
			runNotify()
			return
		case "triage-set":
			runTriageSet(os.Args[2:])
			return
		case "triage-apply":
			runTriageApply(os.Args[2:])
			return
		case "validate-keys":
			runValidateKeys(os.Args[2:])
			return
		case "keydrop":
			runKeyDrop(os.Args[2:])
			return
		case "scan":
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		case "help", "-h", "--help":
			printUsage()
			return
		default:
			// Unknown first token: treat the rest as flags for the scan pipeline.
		}
	}

	dryRun := flag.Bool("dry-run", false, "Не сохранять результаты")
	source := flag.String("source", "", "Сканировать только конкретный источник")
	verify := flag.Bool("verify", false, "Верифицировать провайдеров")
	limit := flag.Int("limit", 10, "Лимит находок для вывода")
	noAlerts := flag.Bool("no-alerts", false, "Не отправлять алерты в Telegram")
	alertConfigPath := flag.String("alert-config", "config/alerter.json", "Путь к конфигу алертеров")
	showVersion := flag.Bool("version", false, "Показать версию и выйти")
	apiAddr := flag.String("api", "", "Запустить HTTP API сервер на указанном адресе (напр. :8080)")
	cfVerify := flag.Bool("cf-verify", false, "Верифицировать Cloudflare Workers AI аккаунты")
	cfConfigPath := flag.String("cf-config", "config/cf_accounts.json", "Путь к конфигу CF аккаунтов")
	dataDir := flag.String("data-dir", "data", "Каталог с free-api-hunter.db (SQLite)")
	bridge := flag.Bool("bridge", false, "После скана дописать новые находки в pending_review.json")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Free API Hunter %s\n", Version)
		os.Exit(0)
	}

	// API сервер (graceful shutdown)
	if *apiAddr != "" {
		server := api.NewServer(*apiAddr)
		logger.Printf("API server starting on %s", *apiAddr)
		if err := server.ListenAndServeGraceful(); err != nil {
			logger.Fatalf("API server error: %v", err)
		}
		return
	}

	logger.Printf("Free API Hunter %s starting...", Version)

	// 0. Init database
	if err := storage.InitDB(*dataDir); err != nil {
		logger.Fatalf("Database init failed: %v", err)
	}
	defer storage.CloseDB()

	// 1. Load all config
	config, filterConfig, err := loadAllConfig(*source)
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// 2. Run scrape + filter pipeline
	rawFindings, findings, err := runScrapePipeline(config, filterConfig)
	if err != nil {
		logger.Fatalf("Pipeline error: %v", err)
	}

	// 3. Load/verify providers
	providers := loadInitialProviders(config)
	if *verify {
		providers = verifyProviders(providers)
	}

	// 4. Print results
	output.PrintResults(rawFindings, findings, providers, *limit)

	// 5. Pollinations pipeline
	if *verify {
		providers = runPollinationsPipeline(providers)
	}

	// 6. Save results
	if !*dryRun {
		saveResults(providers, findings)
	} else {
		logger.Println("Dry run — results not saved")
	}

	logger.Println("Scan completed.")

	// 6a. Bridge new findings into pending_review.json for the Mongoose agent.
	if *bridge {
		added, err := notify.BridgePendingReview(defaultPendingReviewPath)
		if err != nil {
			logger.Printf("Bridge to pending_review.json failed: %v", err)
		} else {
			logger.Printf("Bridge: added %d new finding(s) to %s", added, defaultPendingReviewPath)
		}
	}

	// Save scan history
	if !*dryRun {
		if err := storage.SaveScanHistory(len(rawFindings), len(findings), len(providers), 0); err != nil {
			logger.Printf("Failed to save scan history: %v", err)
		}
	}

	// 6b. Cloudflare Workers AI pipeline
	if *cfVerify {
		logger.Println("Running Cloudflare Workers AI pipeline...")
		runCFPipeline(*cfConfigPath, *noAlerts, *alertConfigPath)
	}

	// 7. OCR pipeline
	if *verify {
		logger.Println("Running OCR pipeline...")
		runOCRPipeline(*noAlerts, *alertConfigPath)
	}

	// 8. TTS pipeline
	if *verify {
		logger.Println("Running TTS pipeline...")
		runTTSPipeline(*noAlerts, *alertConfigPath)
	}

	// 9. Send alert
	if !*noAlerts {
		sendAlert(*alertConfigPath, len(rawFindings), len(findings))
	}
}

// loadAllConfig loads sources.json and filters.json, optionally filtering by source ID.
func loadAllConfig(sourceID string) (*Config, FilterConfig, error) {
	config, err := loadConfig("config/sources.json")
	if err != nil {
		return nil, FilterConfig{}, err
	}

	if sourceID != "" {
		var filtered []scraper.SourceConfig
		for _, s := range config.Sources {
			if s.ID == sourceID {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			return nil, FilterConfig{}, fmt.Errorf("source %s not found or disabled", sourceID)
		}
		config.Sources = filtered
	}

	filterConfig := loadFilterConfig("config/filters.json")
	return config, filterConfig, nil
}

// runScrapePipeline runs the scraper and filter engine on the given sources.
func runScrapePipeline(config *Config, filterConfig FilterConfig) ([]models.Finding, []models.Finding, error) {
	logger.Println("Running scraper...")
	rawFindings := scraper.RunScraper(config.Sources)

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
	return rawFindings, findings, nil
}

// verifyProviders verifies each provider's page and updates status.
func verifyProviders(providers []*models.Provider) []*models.Provider {
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
	return providers
}

// runPollinationsPipeline tests Pollinations models and adds/updates the provider.
func runPollinationsPipeline(providers []*models.Provider) []*models.Provider {
	logger.Println("Testing Pollinations models...")
	pollInfo, pollResults := pollinations.TestAllModels()
	if pollInfo != nil {
		pollProvider := pollinations.ToProvider(pollInfo)
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

	ok, msg := pollinations.VerifyImageGeneration()
	if ok {
		logger.Println("Pollinations image generation: ✅")
	} else {
		logger.Printf("Pollinations image generation: %s", msg)
	}
	return providers
}

// saveResults persists providers and findings to disk.
func saveResults(providers []*models.Provider, findings []models.Finding) {
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
}

// runNotify — подкоманда `hunter notify`: мостит новые находки из БД в
// pending_review.json (контракт Мангуста). Существующие записи, включая
// reviewed:true, сохраняются; добавляются только новые (по source URL).
func runNotify() {
	fs := flag.NewFlagSet("notify", flag.ExitOnError)
	dataDir := fs.String("data-dir", "data", "Каталог с free-api-hunter.db (SQLite)")
	outPath := fs.String("out", defaultPendingReviewPath, "Путь к pending_review.json")
	dryRun := fs.Bool("dry-run", false, "Не записывать, только сообщить сколько добавится")
	_ = fs.Parse(os.Args[1:])

	if err := storage.InitDB(*dataDir); err != nil {
		logger.Fatalf("Database init failed: %v", err)
	}
	defer storage.CloseDB()

	if *dryRun {
		added, _, err := notify.ComputeBridge(*outPath)
		if err != nil {
			logger.Fatalf("Compute bridge failed: %v", err)
		}
		logger.Printf("Dry run: %d new finding(s) would be added to %s", added, *outPath)
		return
	}

	added, err := notify.BridgePendingReview(*outPath)
	if err != nil {
		logger.Fatalf("Bridge failed: %v", err)
	}
	logger.Printf("Added %d new finding(s) to %s", added, *outPath)
}

// runTriageSet — подкоманда `hunter triage-set`: записывает вердикт человека
// в pending_review.json (КRV П.1/П.2). НЕ вызывает Яндекс — исполнение
// откладывается до ночного `hunter triage-apply` (systemd-таймер).
func runTriageSet(args []string) {
	fs := flag.NewFlagSet("triage-set", flag.ExitOnError)
	dataDir := fs.String("data-dir", "/root/LabDoctorM/projects/free-api-hunter/data", "Каталог данных (data/)")
	index := fs.Int("index", 0, "1-based индекс элемента в pending_review.json")
	verdict := fs.String("verdict", "", "verdict: rejected|backlog|already_in_use|confirmed (алиасы not_confirmed|not_working_rf → rejected)")
	source := fs.String("source", "", "матч по Source вместо --index")
	fs.Parse(args)

	if *verdict == "" {
		logger.Fatalf("triage-set: --verdict обязателен")
	}
	if err := notify.TriageSet(*dataDir, *index, *verdict, *source); err != nil {
		logger.Fatalf("triage-set failed: %v", err)
	}
	logger.Printf("triage-set: OK (index=%d source=%q verdict=%s)", *index, *source, *verdict)
}

// runTriageApply — подкоманда `hunter triage-apply`: исполняет записанные
// вердикты (КRV П.1/П.2): rejected → denylist (rejected.json), backlog →
// Яндекс Календарь (задача + событие). Предназначен для ночного systemd-таймера.
func runTriageApply(args []string) {
	fs := flag.NewFlagSet("triage-apply", flag.ExitOnError)
	dataDir := fs.String("data-dir", "/root/LabDoctorM/projects/free-api-hunter/data", "Каталог данных (data/)")
	dryRun := fs.Bool("dry-run", false, "Не вызывать yandex.sh, только логировать что создалось бы")
	fs.Parse(args)

	if err := notify.TriageApply(*dataDir, *dryRun); err != nil {
		logger.Fatalf("triage-apply failed: %v", err)
	}
}

// runKeyDrop — подкоманда `hunter keydrop`: забирает .md с ключами
// из папки на Яндекс Диске, зеркалит в vault, верифицирует и регистрирует
// в Registry (таблица keys). См. internal/keydrop.
func runKeyDrop(args []string) {
	fs := flag.NewFlagSet("keydrop", flag.ExitOnError)
	dataDir := fs.String("data-dir", "data", "Каталог с free-api-hunter.db (SQLite)")
	diskDir := fs.String("disk-dir", "free-api-hunter/keys", "Папка на Яндекс Диске (отн. корня Диска)")
	yandexBin := fs.String("yandex", "/root/LabDoctorM/projects/DoctorM_and_Ai/bin/yandex.sh", "Путь к yandex.sh")
	endpoints := fs.String("endpoints", "config/validator_endpoints.json", "Карта endpoint'ов для верификации")
	keep := fs.Bool("keep", false, "Не удалять .md с Диска после обработки")
	addedBy := fs.String("added-by", "keydrop", "Кто добавил ключ (аудит)")
	_ = fs.Parse(args)

	opts := keydrop.Options{
		DataDir:       *dataDir,
		DiskDir:       *diskDir,
		YandexBin:     *yandexBin,
		EndpointsPath: *endpoints,
		AddedBy:       *addedBy,
		Keep:          *keep,
	}
	if err := keydrop.Run(opts); err != nil {
		logger.Fatalf("keydrop failed: %v", err)
	}
}

// подкоманда `hunter validate-keys`: живая валидация
// API-ключей из vault (spike/krv-validator). Пишет live_status в таблицу "keys".
func runValidateKeys(args []string) {
	fs := flag.NewFlagSet("validate-keys", flag.ExitOnError)
	check := fs.Bool("check", false, "Dry-run: проверить конфиг/схему БД без сетевых вызовов")
	dryRun := fs.Bool("dry-run", false, "Не делать live-пробы и не писать в БД")
	dataDir := fs.String("data-dir", "data", "Директория данных/БД")
	endpoints := fs.String("endpoints", "configs/validator_endpoints.json", "Путь к карте endpoint'ов")
	addedBy := fs.String("added-by", "krv-validator", "Кто добавил записи")
	fs.Parse(args)

	if *check {
		*dryRun = true
	}
	if err := validator.Run(validator.Config{
		DataDir:       *dataDir,
		EndpointsPath: *endpoints,
		DryRun:        *dryRun,
		AddedBy:       *addedBy,
	}); err != nil {
		logger.Fatalf("validate-keys failed: %v", err)
	}
}

// printUsage — справка по подкомандам.
func printUsage() {
	fmt.Fprintln(os.Stderr, "Free API Hunter "+Version)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  hunter [flags] scan     Run discovery scan (default)")
	fmt.Fprintln(os.Stderr, "  hunter notify [-data-dir D] [-out P] [-dry-run]   Bridge findings to pending_review.json")
	fmt.Fprintln(os.Stderr, "  hunter triage-set [-data-dir D] -index N|-source S -verdict V   Record a human verdict (writes file only)")
	fmt.Fprintln(os.Stderr, "  hunter triage-apply [-data-dir D] [-dry-run]   Execute recorded verdicts (denylist + Yandex)")
	fmt.Fprintln(os.Stderr, "  hunter validate-keys [-data-dir D] [-endpoints P] [-dry-run] [-check]   Live-validate vault keys")
	fmt.Fprintln(os.Stderr, "  hunter help            Show this help")
}

// sendAlert loads alert config and sends a Telegram report.
func sendAlert(alertConfigPath string, rawCount, filteredCount int) {
	alertCfg, err := alerter.LoadConfig(alertConfigPath)
	if err != nil {
		logger.Printf("Alert config not found (%v), skipping alerts", err)
		return
	}
	report := alerter.FormatScanReport(rawCount, filteredCount, nil)
	if err := alerter.SendTelegram(alertCfg, report); err != nil {
		logger.Printf("Failed to send alert: %v", err)
	} else {
		logger.Println("Alert sent to Telegram")
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
		UpdatedAt string                    `json:"updated_at"`
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

// runCFPipeline — полный pipeline для Cloudflare Workers AI: верификация → скоринг → алерт
func runCFPipeline(cfConfigPath string, noAlerts bool, alertConfigPath string) {
	// 0. Инициализируем пул аккаунтов
	if err := cf.InitKeyPool(cfConfigPath); err != nil {
		logger.Printf("CF keypool init failed (%v), skipping CF pipeline", err)
		return
	}

	// 1. Верифицируем все аккаунты
	logger.Println("CF: verifying accounts...")
	verifyResults := cf.VerifyAll()
	if verifyResults == nil {
		logger.Println("CF: no accounts in pool")
		return
	}

	activeCount := 0
	var details []map[string]interface{}
	for _, r := range verifyResults {
		if r.Active {
			activeCount++
		}
		details = append(details, map[string]interface{}{
			"account_id":   r.AccountID,
			"active":       r.Active,
			"models_count": r.ModelsCount,
			"neuron_limit": r.NeuronLimit,
			"error":        r.Error,
		})
		status := "❌"
		if r.Active {
			status = "✅"
		}
		logger.Printf("CF: %s %s — %d models, %d neurons/day%s",
			status, r.AccountID[:12], r.ModelsCount, r.NeuronLimit,
			map[bool]string{true: "", false: " — " + r.Error}[r.Active])
	}

	// 2. Скоринг
	logger.Printf("CF: scoring... (%d/%d active)", activeCount, len(verifyResults))
	score := activeCount * 100 / len(verifyResults)

	// 3. Вывод в stdout
	fmt.Println()
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println("CLOUDFLARE WORKERS AI")
	fmt.Println(strings.Repeat("─", 40))
	for _, d := range details {
		status := "❌"
		if d["active"].(bool) {
			status = "✅"
		}
		fmt.Printf("[%s] %s\n", status, d["account_id"].(string)[:12])
	}
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("Active: %d/%d | Score: %d%%\n", activeCount, len(verifyResults), score)
	fmt.Printf("Total free Neurons/day: %d\n", activeCount*10000)
	fmt.Println(strings.Repeat("─", 40))

	// 4. Сохраняем данные
	saveCFData(verifyResults)

	// 5. Алерт
	if !noAlerts && activeCount > 0 {
		alertCfg, err := alerter.LoadConfig(alertConfigPath)
		if err != nil {
			logger.Printf("CF alert: config not found (%v), skipping", err)
		} else {
			report := alerter.FormatCFReport(activeCount, len(verifyResults), score)
			if err := alerter.SendTelegram(alertCfg, report); err != nil {
				logger.Printf("CF alert failed: %v", err)
			} else {
				logger.Println("CF report alert sent")
			}
		}
	}
}

// saveCFData — сохранить CF данные в JSON
func saveCFData(results []*cf.VerifyResult) {
	type cfData struct {
		Results   []*cf.VerifyResult `json:"results"`
		UpdatedAt string             `json:"updated_at"`
	}

	data := cfData{
		Results:   results,
		UpdatedAt: models.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.Printf("CF: failed to marshal: %v", err)
		return
	}

	if err := os.WriteFile("data/cf_accounts.json", jsonData, 0644); err != nil {
		logger.Printf("CF: failed to save: %v", err)
	} else {
		logger.Println("CF: saved to data/cf_accounts.json")
	}
}
