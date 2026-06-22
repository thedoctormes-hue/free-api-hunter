package alerter

import (
	"strings"
	"testing"

	"free-api-hunter/internal/orex"
)

func TestFormatScanReportLargeNumbers(t *testing.T) {
	report := FormatScanReport(99999, 50000, []string{"P1", "P2", "P3", "P4", "P5"})
	if !strings.Contains(report, "99999") {
		t.Error("should contain raw count")
	}
	if !strings.Contains(report, "50000") {
		t.Error("should contain filtered count")
	}
	for _, p := range []string{"P1", "P2", "P3"} {
		if !strings.Contains(report, p) {
			t.Errorf("should contain provider %s", p)
		}
	}
}

func TestFormatKeyStatusEmptyModels(t *testing.T) {
	status := FormatKeyStatus("TestProvider", nil, "")
	if !strings.Contains(status, "TestProvider") {
		t.Error("should contain provider name")
	}
}

func TestFormatKeyPoolReportAllActive(t *testing.T) {
	report := FormatKeyPoolReport(5, 5, []string{"A", "B", "C", "D", "E"})
	if !strings.Contains(report, "5 / 5") {
		t.Error("should show all active")
	}
}

func TestFormatKeyPoolReportNoneActive(t *testing.T) {
	report := FormatKeyPoolReport(0, 5, nil)
	if !strings.Contains(report, "0 / 5") {
		t.Error("should show none active")
	}
}

func TestFormatOrexNewModelAlertWithModels(t *testing.T) {
	models := []orex.FreeModel{{Name: "gpt-4"}, {Name: "claude-3"}, {Name: "llama-3"}}
	report := FormatOrexNewModelAlert(models, 3)
	if !strings.Contains(report, "3") {
		t.Error("should contain count")
	}
	if !strings.Contains(report, "gpt-4") {
		t.Error("should contain model names")
	}
}

func TestOrexAlertEventToJSONEmpty(t *testing.T) {
	event := OrexAlertEvent{}
	json := event.ToJSON()
	if json == "" {
		t.Error("ToJSON should return non-empty string even for empty event")
	}
}
