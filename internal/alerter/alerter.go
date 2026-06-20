package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"free-api-hunter/internal/orex"
)

var logger = log.New(os.Stderr, "[alerter] ", log.LstdFlags)

// TelegramConfig — конфиг Telegram бота
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// vaultPath — путь к vault для Telegram credentials
var vaultPath = "/root/LabDoctorM/vault/free-api-hunter"

// LoadConfig — загрузить конфиг алертов.
// Приоритет: vault > config file.
// Если токен = "YOUR_*" — выводит warning и возвращает nil.
func LoadConfig(path string) (*TelegramConfig, error) {
	cfg := &TelegramConfig{}

	// 1. Пробуем загрузить из vault
	vaultTokenFile := filepath.Join(vaultPath, "telegram_bot_token.key")
	vaultChatFile := filepath.Join(vaultPath, "telegram_chat_id.key")

	token, tokenErr := os.ReadFile(vaultTokenFile)
	chat, chatErr := os.ReadFile(vaultChatFile)

	if tokenErr == nil && chatErr == nil {
		cfg.BotToken = strings.TrimSpace(string(token))
		cfg.ChatID = strings.TrimSpace(string(chat))
		if cfg.BotToken != "" && cfg.ChatID != "" {
			logger.Println("Telegram config loaded from vault")
			return cfg, nil
		}
	}

	// 2. Fallback: config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// 3. Проверяем на placeholder values
	if strings.HasPrefix(cfg.BotToken, "YOUR_") || strings.HasPrefix(cfg.ChatID, "YOUR_") {
		logger.Println("⚠️  TELEGRAM NOT CONFIGURED: bot_token or chat_id is a placeholder.")
		logger.Println("   Create vault files or update config/alerter.json:")
		logger.Printf("   echo 'YOUR_BOT_TOKEN' > %s", vaultTokenFile)
		logger.Printf("   echo 'YOUR_CHAT_ID' > %s", vaultChatFile)
		return nil, nil
	}

	logger.Println("Telegram config loaded from config file")
	return cfg, nil
}

// SendTelegram — отправить сообщение в Telegram
func SendTelegram(cfg *TelegramConfig, text string) error {
	if cfg == nil || cfg.BotToken == "" || cfg.ChatID == "" {
		logger.Println("Telegram config not set, skipping alert")
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	payload := map[string]string{
		"chat_id":    cfg.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram HTTP %d", resp.StatusCode)
	}
	return nil
}

// FormatScanReport — форматировать отчёт о сканировании
func FormatScanReport(rawCount, filteredCount int, newProviders []string) string {
	var b strings.Builder
	b.WriteString("🔍 <b>Free API Hunter — Scan Report</b>\n\n")
	b.WriteString(fmt.Sprintf("📊 Raw findings: <b>%d</b>\n", rawCount))
	b.WriteString(fmt.Sprintf("✅ After filter: <b>%d</b>\n", filteredCount))

	if len(newProviders) > 0 {
		b.WriteString("\n🆕 <b>New providers:</b>\n")
		for _, p := range newProviders {
			b.WriteString(fmt.Sprintf("  • %s\n", p))
		}
	}

	b.WriteString(fmt.Sprintf("\n⏰ %s UTC", time.Now().Format("2006-01-02 15:04")))
	return b.String()
}

// FormatKeyStatus — статус ключей провайдера
func FormatKeyStatus(provider string, models []string, limits string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔑 <b>%s</b>\n", provider))
	if len(models) > 0 {
		b.WriteString(fmt.Sprintf("   Models: %s\n", strings.Join(models, ", ")))
	}
	if limits != "" {
		b.WriteString(fmt.Sprintf("   Limits: %s\n", limits))
	}
	return b.String()
}

// FormatKeyPoolReport — отчёт о пуле ключей
func FormatKeyPoolReport(active, total int, providers []string) string {
	var b strings.Builder
	b.WriteString("🏦 <b>Free API Hunter — Key Pool Report</b>\n\n")
	b.WriteString(fmt.Sprintf("🔑 Active keys: <b>%d / %d</b>\n\n", active, total))

	if len(providers) > 0 {
		b.WriteString("<b>Providers with active keys:</b>\n")
		for _, p := range providers {
			b.WriteString(fmt.Sprintf("  ✅ %s\n", p))
		}
	}

	b.WriteString(fmt.Sprintf("\n⏰ %s UTC", time.Now().Format("2006-01-02 15:04")))
	return b.String()
}

// ============================================================
// Orex alerts
// ============================================================

// FormatOrexNewModelAlert — алерт о новой бесплатной модели из Orex
func FormatOrexNewModelAlert(freeModels []orex.FreeModel, newCount int) string {
	var b strings.Builder
	b.WriteString("🆕 <b>Orex — New Free Models</b>\n\n")
	b.WriteString(fmt.Sprintf("Found <b>%d</b> new free models:\n\n", newCount))

	// Показываем первые 10
	limit := 10
	if len(freeModels) < limit {
		limit = len(freeModels)
	}
	for i := 0; i < limit; i++ {
		fm := freeModels[i]
		b.WriteString(fmt.Sprintf("  • <b>%s</b> (%s) — ctx: %d\n", fm.Name, fm.Provider, fm.ContextLength))
	}
	if len(freeModels) > limit {
		b.WriteString(fmt.Sprintf("\n  ... and %d more", len(freeModels)-limit))
	}

	b.WriteString(fmt.Sprintf("\n⏰ %s UTC", time.Now().Format("2006-01-02 15:04")))
	return b.String()
}

// FormatOrexAlertEvent — алерт о событии из Orex
func FormatOrexAlertEvent(alert orex.OrexAlert) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("⚠️ <b>Orex Alert — %s</b>\n\n", strings.ToUpper(alert.Type)))
	b.WriteString(fmt.Sprintf("Model: <b>%s</b>\n", alert.Model))
	b.WriteString(fmt.Sprintf("Message: %s\n", alert.Message))
	b.WriteString(fmt.Sprintf("\n⏰ %s", alert.Timestamp))
	return b.String()
}

// FormatOrexSyncReport — отчёт о синхронизации с Orex
func FormatOrexSyncReport(totalFree, newModels, alertsCount int) string {
	var b strings.Builder
	b.WriteString("🔄 <b>Orex Sync Report</b>\n\n")
	b.WriteString(fmt.Sprintf("Free models: <b>%d</b>\n", totalFree))
	b.WriteString(fmt.Sprintf("New models: <b>%d</b>\n", newModels))
	b.WriteString(fmt.Sprintf("Alerts: <b>%d</b>\n", alertsCount))
	b.WriteString(fmt.Sprintf("\n⏰ %s UTC", time.Now().Format("2006-01-02 15:04")))
	return b.String()
}

// OrexAlertEvent — событие для отправки через alerter (JSON для Бестии)
type OrexAlertEvent struct {
	Event     string `json:"event"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Status    string `json:"status"`
	Details   string `json:"details"`
	Timestamp string `json:"timestamp"`
}

// NewOrexAlertEvent — создать событие Orex
func NewOrexAlertEvent(eventType, provider, model, status, details string) *OrexAlertEvent {
	return &OrexAlertEvent{
		Event:     eventType,
		Provider:  provider,
		Model:     model,
		Status:    status,
		Details:   details,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// ToJSON — сериализовать событие в JSON
func (e *OrexAlertEvent) ToJSON() string {
	b, _ := json.Marshal(e)
	return string(b)
}
