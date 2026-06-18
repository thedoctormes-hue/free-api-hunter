package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var logger = log.New(os.Stderr, "[alerter] ", log.LstdFlags)

// TelegramConfig — конфиг Telegram бота
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// LoadConfig — загрузить конфиг алертов
func LoadConfig(path string) (*TelegramConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg TelegramConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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
