package alerter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/orex"
)

func TestLoadConfigNonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	// Vault empty + file doesn't exist → error
	_, err := LoadConfig(filepath.Join(tmpDir, "nonexistent.json"))
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	cfgFile := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(cfgFile, []byte("not json"), 0644)

	_, err := LoadConfig(cfgFile)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadConfigPlaceholderToken(t *testing.T) {
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	cfgFile := filepath.Join(t.TempDir(), "alerter.json")
	os.WriteFile(cfgFile, []byte(`{"bot_token":"YOUR_BOT_TOKEN","chat_id":"real-chat"}`), 0644)

	cfg, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil for placeholder token")
	}
}

func TestLoadConfigPlaceholderChatID(t *testing.T) {
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	cfgFile := filepath.Join(t.TempDir(), "alerter.json")
	os.WriteFile(cfgFile, []byte(`{"bot_token":"real-token","chat_id":"YOUR_CHAT_ID"}`), 0644)

	cfg, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil for placeholder chat_id")
	}
}

func TestLoadConfigVaultPartialTokenOnly(t *testing.T) {
	tmpDir := t.TempDir()
	origPath := vaultPath
	vaultPath = tmpDir
	defer func() { vaultPath = origPath }()

	// Only token in vault, no chat_id → fallback to file
	os.WriteFile(filepath.Join(tmpDir, "telegram_bot_token.key"), []byte("vault-token"), 0600)

	cfgFile := filepath.Join(t.TempDir(), "alerter.json")
	os.WriteFile(cfgFile, []byte(`{"bot_token":"file-token","chat_id":"file-chat"}`), 0644)

	cfg, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || cfg.BotToken != "file-token" {
		t.Error("should fallback to file when vault is incomplete")
	}
}

func TestFormatScanReportEmptyNewProviders(t *testing.T) {
	report := FormatScanReport(10, 5, []string{})
	if !strings.Contains(report, "10") {
		t.Error("should contain raw count")
	}
	if strings.Contains(report, "New providers") {
		t.Error("should not contain 'New providers' for empty list")
	}
}

func TestFormatScanReportManyNewProviders(t *testing.T) {
	providers := make([]string, 20)
	for i := range providers {
		providers[i] = "Provider" + string(rune('A'+i))
	}
	report := FormatScanReport(200, 100, providers)
	if !strings.Contains(report, "ProviderA") {
		t.Error("should contain provider names")
	}
}

func TestFormatKeyStatusEmpty(t *testing.T) {
	status := FormatKeyStatus("TestProv", nil, "")
	if !strings.Contains(status, "TestProv") {
		t.Error("should contain provider name")
	}
}

func TestFormatKeyStatusWithModels(t *testing.T) {
	status := FormatKeyStatus("Groq", []string{"llama-3", "mixtral"}, "100 RPM")
	if !strings.Contains(status, "llama-3") {
		t.Error("should contain model names")
	}
	if !strings.Contains(status, "100 RPM") {
		t.Error("should contain limits")
	}
}

func TestFormatOrexNewModelAlertEmpty(t *testing.T) {
	report := FormatOrexNewModelAlert(nil, 0)
	if !strings.Contains(report, "0") {
		t.Error("should contain count 0")
	}
}

func TestFormatOrexNewModelAlertManyModels(t *testing.T) {
	models := make([]orex.FreeModel, 15)
	for i := range models {
		models[i] = orex.FreeModel{
			Name:          "model-" + string(rune('a'+i)),
			Provider:      "test-provider",
			ContextLength: 4096,
		}
	}
	report := FormatOrexNewModelAlert(models, 15)
	if !strings.Contains(report, "15") {
		t.Error("should contain count")
	}
	// Should show first 10 and "... and N more"
	if !strings.Contains(report, "more") {
		t.Error("should indicate there are more models")
	}
}

func TestFormatOrexAlertEventFull(t *testing.T) {
	alert := orex.OrexAlert{
		Type:      "deprecation",
		Model:     "gpt-3.5-turbo",
		Message:   "Model deprecated",
		Timestamp: "2026-06-26T00:00:00Z",
	}
	report := FormatOrexAlertEvent(alert)
	if !strings.Contains(report, "DEPRECATION") {
		t.Error("should contain uppercase type")
	}
	if !strings.Contains(report, "gpt-3.5-turbo") {
		t.Error("should contain model name")
	}
	if !strings.Contains(report, "Model deprecated") {
		t.Error("should contain message")
	}
}

func TestFormatOrexSyncReportFull(t *testing.T) {
	report := FormatOrexSyncReport(50, 5, 2)
	if !strings.Contains(report, "50") {
		t.Error("should contain total free count")
	}
	if !strings.Contains(report, "5") {
		t.Error("should contain new models count")
	}
	if !strings.Contains(report, "2") {
		t.Error("should contain alerts count")
	}
}

func TestFormatTTSReportEmpty(t *testing.T) {
	report := FormatTTSReport(nil, nil, nil)
	if report == "" {
		t.Error("should return non-empty report even with nil inputs")
	}
}

func TestFormatTTSReportWithResults(t *testing.T) {
	results := []*models.TTSVerifyResult{
		{IsActive: true, Plan: "free", CharLimit: 10000, Voices: []string{"voice1", "voice2"}},
		{IsActive: false, Error: "API key invalid"},
	}
	scores := []*models.TTSScore{
		{OverallScore: 0.85, ProviderName: "TestTTS"},
	}
	providers := []*models.TTSProvider{
		{Name: "TestTTS"},
		{Name: "FailedTTS"},
	}
	report := FormatTTSReport(results, scores, providers)
	if !strings.Contains(report, "TestTTS") {
		t.Error("should contain provider name")
	}
	if !strings.Contains(report, "API key invalid") {
		t.Error("should contain error for inactive provider")
	}
}

func TestFormatTTSKeyStatusActive(t *testing.T) {
	result := &models.TTSVerifyResult{
		IsActive:  true,
		Plan:      "free",
		CharLimit: 5000,
		Voices:    []string{"v1", "v2", "v3"},
	}
	report := FormatTTSKeyStatus(result, "ElevenLabs")
	if !strings.Contains(report, "ElevenLabs") {
		t.Error("should contain provider name")
	}
	if !strings.Contains(report, "5000") {
		t.Error("should contain char limit")
	}
}

func TestFormatTTSKeyStatusInactive(t *testing.T) {
	result := &models.TTSVerifyResult{
		IsActive: false,
		Error:    "401 Unauthorized",
	}
	report := FormatTTSKeyStatus(result, "BadProvider")
	if !strings.Contains(report, "BadProvider") {
		t.Error("should contain provider name")
	}
	if !strings.Contains(report, "401 Unauthorized") {
		t.Error("should contain error")
	}
}

func TestFormatTTSScoreReportFull(t *testing.T) {
	score := &models.TTSScore{
		ProviderName:  "TestTTS",
		OverallScore:  0.92,
		FreeTierScore: 0.8,
		FeatureScore:  0.95,
		LanguageScore: 0.88,
		LatencyScore:  0.9,
		HasFreeTier:   true,
		CharLimit:     10000,
	}
	report := FormatTTSScoreReport(score)
	if !strings.Contains(report, "TestTTS") {
		t.Error("should contain provider name")
	}
	if !strings.Contains(report, "10000") {
		t.Error("should contain char limit")
	}
}

func TestOrexAlertEventToJSON(t *testing.T) {
	event := NewOrexAlertEvent("test", "prov", "model", "active", "details")
	json := event.ToJSON()
	if !strings.Contains(json, "\"event\":\"test\"") {
		t.Error("JSON should contain event field")
	}
	if !strings.Contains(json, "\"provider\":\"prov\"") {
		t.Error("JSON should contain provider field")
	}
}

func TestSendTelegramEmptyChatID(t *testing.T) {
	cfg := &TelegramConfig{BotToken: "token", ChatID: ""}
	err := SendTelegram(cfg, "test")
	if err != nil {
		t.Errorf("SendTelegram with empty chat_id should return nil, got %v", err)
	}
}
