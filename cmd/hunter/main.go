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
	"free-api-hunter/internal/filter"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/scraper"
	"free-api-hunter/internal/storage"
	"free-api-hunter/internal/verifier"
)

var logger = log.New(os.Stderr, "[hunter] ", log.LstdFlags)

// Config — загруженная конфигурация источников
type Config struct {
	Sources   []scraper.SourceConfig `json:"sources"`
	Providers []ProviderConfig       `json:"provider_pages"`
}

// FilterConfig — конфигурация фильтров из filters.json
type FilterConfig struct {
	ExcludedProviders []string `json:"excluded_providers"`
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
	alertConfigPath := flag.String("alert-config", "config/alerter.json", "Путь к конфигу алертов")
	flag.Parse()

	logger.Println("Free API Hunter v0.1.0 starting...")

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
	if len(filterConfig.ExcludedProviders) > 0 {
		engine.ExcludedProviders = make(map[string]bool)
		for _, p := range filterConfig.ExcludedProviders {
			engine.ExcludedProviders[strings.ToLower(p)] = true
		}
	}
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

	// 7. Сохраняем (если не dry-run)
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

	// 7. Отправляем алерт (если не отключён)
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
	// Пробуем загрузить из файла
	providers, err := storage.LoadProviders("")
	if err == nil && len(providers) > 0 {
		logger.Printf("Loaded %d providers from data/providers.json", len(providers))
		return providers
	}

	// Иначе из конфига
	var result []*models.Provider
	for _, p := range config.Providers {
		status := models.StatusClaimed
		if p.Status == "confirmed" {
			status = models.StatusConfirmed
		}
		result = append(result, &models.Provider{
			Name:         p.Name,
			URL:          p.URL,
			APIKeyURL:    p.URL,
			CreditCard:   p.CreditCard,
			Status:       status,
			DiscoveredAt: models.Now(),
		})
	}
	logger.Printf("Loaded %d providers from initial config", len(result))
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
