package ocr

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

var alertLogger = log.New(os.Stderr, "[ocr-alerter] ", log.LstdFlags)

// OCRAlertType — тип OCR-алерта
type OCRAlertType string

const (
	AlertOCRKeyActive   OCRAlertType = "ocr_key_active"
	AlertOCRKeyInactive OCRAlertType = "ocr_key_inactive"
	AlertOCRKeyExpiring OCRAlertType = "ocr_key_expiring"
	AlertOCRNewProvider OCRAlertType = "ocr_new_provider"
	AlertOCRScanResult  OCRAlertType = "ocr_scan_result"
)

// OCRAlertEvent — событие OCR для отправки через Telegram
type OCRAlertEvent struct {
	Type      OCRAlertType `json:"type"`
	Provider  string       `json:"provider"`
	Status    string       `json:"status"`
	Details   string       `json:"details"`
	Timestamp string       `json:"timestamp"`
}

// NewOCRAlertEvent — создать OCR-событие
func NewOCRAlertEvent(alertType OCRAlertType, provider, status, details string) *OCRAlertEvent {
	return &OCRAlertEvent{
		Type:      alertType,
		Provider:  provider,
		Status:    status,
		Details:   details,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// FormatOCRKeyStatus — форматировать статус OCR-ключа для Telegram
func FormatOCRKeyStatus(result *OCRVerifyResult, providerName string) string {
	var b strings.Builder

	if result.IsActive {
		b.WriteString("✅ <b>OCR Key Active</b>\n\n")
		b.WriteString(fmt.Sprintf("Provider: <b>%s</b>\n", providerName))
		b.WriteString(fmt.Sprintf("Engine: %d\n", result.EngineUsed))
		b.WriteString(fmt.Sprintf("Language: %s\n", result.Language))
		b.WriteString(fmt.Sprintf("Processing time: %sms\n", result.ProcessingMs))
		if result.RecognizedText != "" {
			text := truncate(result.RecognizedText, 100)
			b.WriteString(fmt.Sprintf("Sample text: <code>%s</code>\n", text))
		}
	} else {
		b.WriteString("❌ <b>OCR Key Failed</b>\n\n")
		b.WriteString(fmt.Sprintf("Provider: <b>%s</b>\n", providerName))
		if result.Error != "" {
			b.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		}
	}

	b.WriteString(fmt.Sprintf("\n⏰ %s UTC", time.Now().Format("2006-01-02 15:04")))
	return b.String()
}

// FormatOCRScanReport — отчёт о сканировании OCR-провайдеров
func FormatOCRScanReport(results []*OCRVerifyResult, activeCount, totalCount int) string {
	var b strings.Builder

	b.WriteString("🔍 <b>OCR Provider Scan Report</b>\n\n")
	b.WriteString(fmt.Sprintf("Active keys: <b>%d / %d</b>\n\n", activeCount, totalCount))

	for _, r := range results {
		status := "❌"
		if r.IsActive {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s Engine %d: %s — %s\n",
			status, r.EngineUsed, r.Language, r.ProcessingMs))
	}

	b.WriteString(fmt.Sprintf("\n⏰ %s UTC", time.Now().Format("2006-01-02 15:04")))
	return b.String()
}

// FormatOCRAllEnginesReport — отчёт по всем движкам OCR-провайдера
func FormatOCRAllEnginesReport(providerName string, results []*OCRTestResult) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("🧪 <b>OCR Engines — %s</b>\n\n", providerName))

	for _, r := range results {
		status := "❌"
		if r.Success {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("<b>Engine %d</b> (%s) — %s — %sms\n",
			r.Engine, r.Language, status, r.ProcessingMs))
		if r.Success && r.Text != "" {
			text := truncate(r.Text, 80)
			b.WriteString(fmt.Sprintf("  Text: <code>%s</code>\n", text))
		}
		if r.Error != "" && !r.Success {
			b.WriteString(fmt.Sprintf("  Error: %s\n", r.Error))
		}
	}

	b.WriteString(fmt.Sprintf("\n⏰ %s UTC", time.Now().Format("2006-01-02 15:04")))
	return b.String()
}

// FormatOCRScoreReport — отчёт с оценкой OCR-провайдера
func FormatOCRScoreReport(score *OCRScore) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("📊 <b>OCR Score — %s</b>\n\n", score.ProviderName))
	b.WriteString(fmt.Sprintf("Overall: <b>%.0f%%</b>\n", score.OverallScore*100))
	b.WriteString(fmt.Sprintf("├ Speed: %.0f%%\n", score.SpeedScore*100))
	b.WriteString(fmt.Sprintf("├ Quality: %.0f%%\n", score.QualityScore*100))
	b.WriteString(fmt.Sprintf("├ Features: %.0f%%\n", score.FeatureScore*100))
	b.WriteString(fmt.Sprintf("└ Value: %.0f%%\n", score.ValueScore*100))

	if score.HasFreeTier {
		b.WriteString(fmt.Sprintf("\n🆓 Free tier: %s\n", score.FreeQuota))
	}

	b.WriteString(fmt.Sprintf("\n⏰ %s UTC", time.Now().Format("2006-01-02 15:04")))
	return b.String()
}

// ToJSON — сериализовать событие в JSON
func (e *OCRAlertEvent) ToJSON() string {
	parts := []string{
		fmt.Sprintf(`"event":"%s"`, e.Type),
		fmt.Sprintf(`"provider":"%s"`, e.Provider),
		fmt.Sprintf(`"status":"%s"`, e.Status),
		fmt.Sprintf(`"details":"%s"`, e.Details),
		fmt.Sprintf(`"timestamp":"%s"`, e.Timestamp),
	}
	return "{" + strings.Join(parts, ",") + "}"
}
