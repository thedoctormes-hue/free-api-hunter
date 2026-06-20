package alerter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigFromVault(t *testing.T) {
	// Создаём временный vault
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	// Записываем тестовые ключи
	tokenFile := filepath.Join(tmpDir, "telegram_bot_token.key")
	chatFile := filepath.Join(tmpDir, "telegram_chat_id.key")
	os.WriteFile(tokenFile, []byte("test-token-123"), 0600)
	os.WriteFile(chatFile, []byte("test-chat-456"), 0600)

	cfg, err := LoadConfig("nonexistent.json")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected non-nil config from vault")
	}
	if cfg.BotToken != "test-token-123" {
		t.Errorf("Expected token 'test-token-123', got %q", cfg.BotToken)
	}
	if cfg.ChatID != "test-chat-456" {
		t.Errorf("Expected chat_id 'test-chat-456', got %q", cfg.ChatID)
	}
}

func TestLoadConfigPlaceholder(t *testing.T) {
	// Vault пустой → fallback на config с placeholder → должен вернуть nil
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	// Config file с placeholder
	cfgFile := filepath.Join(t.TempDir(), "alerter.json")
	os.WriteFile(cfgFile, []byte(`{"bot_token":"YOUR_BOT_TOKEN_HERE","chat_id":"YOUR_CHAT_ID_HERE"}`), 0644)

	cfg, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg != nil {
		t.Error("Expected nil config for placeholder values")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Vault пустой → fallback на config с реальными значениями
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	cfgFile := filepath.Join(t.TempDir(), "alerter.json")
	os.WriteFile(cfgFile, []byte(`{"bot_token":"real-token","chat_id":"real-chat"}`), 0644)

	cfg, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}
	if cfg.BotToken != "real-token" {
		t.Errorf("Expected 'real-token', got %q", cfg.BotToken)
	}
}

func TestLoadConfigVaultEmpty(t *testing.T) {
	// Vault существует, но файлы пустые → fallback на config
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	os.WriteFile(filepath.Join(tmpDir, "telegram_bot_token.key"), []byte(""), 0600)
	os.WriteFile(filepath.Join(tmpDir, "telegram_chat_id.key"), []byte(""), 0600)

	cfgFile := filepath.Join(t.TempDir(), "alerter.json")
	os.WriteFile(cfgFile, []byte(`{"bot_token":"file-token","chat_id":"file-chat"}`), 0644)

	cfg, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected non-nil config from file fallback")
	}
	if cfg.BotToken != "file-token" {
		t.Errorf("Expected 'file-token', got %q", cfg.BotToken)
	}
}

func TestFormatScanReport(t *testing.T) {
	report := FormatScanReport(100, 42, []string{"OpenRouter", "Groq"})

	if !strings.Contains(report, "100") {
		t.Error("Report should contain raw count")
	}
	if !strings.Contains(report, "42") {
		t.Error("Report should contain filtered count")
	}
	if !strings.Contains(report, "OpenRouter") {
		t.Error("Report should contain new providers")
	}
	if !strings.Contains(report, "Groq") {
		t.Error("Report should contain new providers")
	}
}

func TestFormatScanReportNoNew(t *testing.T) {
	report := FormatScanReport(50, 20, nil)

	if !strings.Contains(report, "50") {
		t.Error("Report should contain raw count")
	}
	if strings.Contains(report, "New providers") {
		t.Error("Report should not contain 'New providers' when list is empty")
	}
}

func TestFormatKeyStatus(t *testing.T) {
	status := FormatKeyStatus("OpenRouter", []string{"gpt-4", "claude-3"}, "20 RPM")

	if !strings.Contains(status, "OpenRouter") {
		t.Error("Key status should contain provider name")
	}
	if !strings.Contains(status, "gpt-4") {
		t.Error("Key status should contain models")
	}
	if !strings.Contains(status, "20 RPM") {
		t.Error("Key status should contain limits")
	}
}

func TestFormatKeyPoolReport(t *testing.T) {
	report := FormatKeyPoolReport(3, 5, []string{"OpenRouter", "Groq", "Mistral"})

	if !strings.Contains(report, "3 / 5") {
		t.Error("Report should contain active/total ratio")
	}
	if !strings.Contains(report, "OpenRouter") {
		t.Error("Report should list active providers")
	}
}

func TestSendTelegramNilConfig(t *testing.T) {
	// Nil config не должен паниковать
	err := SendTelegram(nil, "test")
	if err != nil {
		t.Errorf("SendTelegram(nil) should return nil, got %v", err)
	}
}

func TestSendTelegramEmptyToken(t *testing.T) {
	// Пустой токен не должен паниковать
	cfg := &TelegramConfig{BotToken: "", ChatID: "123"}
	err := SendTelegram(cfg, "test")
	if err != nil {
		t.Errorf("SendTelegram(empty token) should return nil, got %v", err)
	}
}

func TestFormatOrexNewModelAlert(t *testing.T) {
	// Проверяем что функция не падает с пустым списком
	report := FormatOrexNewModelAlert(nil, 0)
	if !strings.Contains(report, "0") {
		t.Error("Alert should contain count")
	}
}

func TestFormatOrexAlertEvent(t *testing.T) {
	alert := OrexAlertEvent{
		Event:     "test_event",
		Provider:  "openrouter",
		Model:     "gpt-4",
		Status:    "new",
		Details:   "Test details",
		Timestamp: "2026-01-01T00:00:00Z",
	}
	json := alert.ToJSON()
	if !strings.Contains(json, "test_event") {
		t.Error("JSON should contain event type")
	}
	if !strings.Contains(json, "gpt-4") {
		t.Error("JSON should contain model")
	}
}

func TestNewOrexAlertEvent(t *testing.T) {
	event := NewOrexAlertEvent("new_model", "openrouter", "gpt-4", "active", "test")
	if event.Event != "new_model" {
		t.Errorf("Expected 'new_model', got %q", event.Event)
	}
	if event.Provider != "openrouter" {
		t.Errorf("Expected 'openrouter', got %q", event.Provider)
	}
	if event.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}
