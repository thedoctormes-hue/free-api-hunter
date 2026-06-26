package ocr

import (
	"strings"
	"testing"
)

func TestFormatOCRKeyStatusActive(t *testing.T) {
	result := &OCRVerifyResult{
		IsActive:      true,
		EngineUsed:    1,
		Language:      "eng",
		ProcessingMs:  "312",
		RecognizedText: "HELLO WORLD",
	}

	formatted := FormatOCRKeyStatus(result, "OCR.space")

	if !strings.Contains(formatted, "✅") {
		t.Error("Expected ✅ for active key")
	}
	if !strings.Contains(formatted, "OCR.space") {
		t.Error("Expected provider name in output")
	}
	if !strings.Contains(formatted, "312") {
		t.Error("Expected processing time in output")
	}
	if !strings.Contains(formatted, "HELLO WORLD") {
		t.Error("Expected recognized text in output")
	}
}

func TestFormatOCRKeyStatusInactive(t *testing.T) {
	result := &OCRVerifyResult{
		IsActive:   false,
		EngineUsed: 1,
		Error:      "invalid_key",
	}

	formatted := FormatOCRKeyStatus(result, "OCR.space")

	if !strings.Contains(formatted, "❌") {
		t.Error("Expected ❌ for inactive key")
	}
	if !strings.Contains(formatted, "invalid_key") {
		t.Error("Expected error message in output")
	}
}

func TestFormatOCRScanReport(t *testing.T) {
	results := []*OCRVerifyResult{
		{IsActive: true, EngineUsed: 1, Language: "eng", ProcessingMs: "312"},
		{IsActive: false, EngineUsed: 2, Language: "eng", ProcessingMs: "0"},
	}

	formatted := FormatOCRScanReport(results, 1, 2)

	if !strings.Contains(formatted, "1 / 2") {
		t.Error("Expected active/total count in output")
	}
	if !strings.Contains(formatted, "Engine 1") {
		t.Error("Expected engine info in output")
	}
}

func TestFormatOCRAllEnginesReport(t *testing.T) {
	results := []*OCRTestResult{
		{Engine: 1, Language: "eng", Success: true, ProcessingMs: "312", Text: "HELLO"},
		{Engine: 2, Language: "eng", Success: false, Error: "test image too small"},
		{Engine: 3, Language: "eng", Success: true, ProcessingMs: "987", Text: "WORLD"},
	}

	formatted := FormatOCRAllEnginesReport("OCR.space", results)

	if !strings.Contains(formatted, "Engine 1") {
		t.Error("Expected Engine 1 in output")
	}
	if !strings.Contains(formatted, "Engine 2") {
		t.Error("Expected Engine 2 in output")
	}
	if !strings.Contains(formatted, "Engine 3") {
		t.Error("Expected Engine 3 in output")
	}
	if !strings.Contains(formatted, "HELLO") {
		t.Error("Expected recognized text in output")
	}
}

func TestFormatOCRScoreReport(t *testing.T) {
	score := &OCRScore{
		ProviderName: "OCR.space",
		OverallScore: 0.85,
		SpeedScore:   0.8,
		QualityScore: 0.9,
		FeatureScore: 1.0,
		ValueScore:   0.7,
		HasFreeTier:  true,
		FreeQuota:    "25,000/month",
	}

	formatted := FormatOCRScoreReport(score)

	if !strings.Contains(formatted, "85%") {
		t.Error("Expected overall score percentage")
	}
	if !strings.Contains(formatted, "OCR.space") {
		t.Error("Expected provider name")
	}
	if !strings.Contains(formatted, "25,000/month") {
		t.Error("Expected free quota info")
	}
}

func TestNewOCRAlertEvent(t *testing.T) {
	event := NewOCRAlertEvent(AlertOCRKeyActive, "OCR.space", "active", "key verified")

	if event.Type != AlertOCRKeyActive {
		t.Errorf("Expected type=%s, got %s", AlertOCRKeyActive, event.Type)
	}
	if event.Provider != "OCR.space" {
		t.Errorf("Expected provider=OCR.space, got %s", event.Provider)
	}
	if event.Status != "active" {
		t.Errorf("Expected status=active, got %s", event.Status)
	}
}

func TestOCRAlertEventToJSON(t *testing.T) {
	event := NewOCRAlertEvent(AlertOCRKeyInactive, "OCR.space", "inactive", "key expired")
	json := event.ToJSON()

	if !strings.Contains(json, `"event":"ocr_key_inactive"`) {
		t.Error("Expected event type in JSON")
	}
	if !strings.Contains(json, `"provider":"OCR.space"`) {
		t.Error("Expected provider in JSON")
	}
	if !strings.Contains(json, `"status":"inactive"`) {
		t.Error("Expected status in JSON")
	}
}
