package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"free-api-hunter/internal/orex"
)

// HealthStatus — расширенный health check
type HealthStatus struct {
	Status        string `json:"status"`
	Time          string `json:"time"`
	Version       string `json:"version"`
	LastScanTime  string `json:"last_scan_time,omitempty"`
	ProvidersCount int   `json:"providers_count,omitempty"`
	FindingsCount  int   `json:"findings_count,omitempty"`
	ScanLogOk     bool   `json:"scan_log_ok"`
	OrexProxyOk   bool   `json:"orex_proxy_ok"`
}

// handleHealthExtended — GET /health — расширенный health check
func (s *Server) handleHealthExtended(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := HealthStatus{
		Status:  "ok",
		Time:    time.Now().UTC().Format(time.RFC3339),
		Version: "0.1.0",
	}

	// Проверяем что данные существуют и свежие
	providersPath := filepath.Join(s.DataDir, "providers.json")
	findingsPath := filepath.Join(s.DataDir, "findings.json")

	if provs, err := os.Stat(providersPath); err == nil {
		status.ProvidersCount = countItems(providersPath)
		if time.Since(provs.ModTime()) < 12*time.Hour {
			status.LastScanTime = provs.ModTime().UTC().Format(time.RFC3339)
		}
	}

	if _, err := os.Stat(findingsPath); err == nil {
		status.FindingsCount = countItems(findingsPath)
	}

	// Проверяем scan log
	scanLogPath := "/var/log/free-api-hunter/scan.log"
	if fi, err := os.Stat(scanLogPath); err == nil {
		status.ScanLogOk = time.Since(fi.ModTime()) < 12*time.Hour
	}

	// Проверяем Orex proxy
	orexClient := orex.NewClient("")
	if _, err := orexClient.GetModels(); err == nil {
		status.OrexProxyOk = true
	}

	// Если данные старше 12 часов — degraded
	if status.LastScanTime == "" {
		status.Status = "degraded"
	}

	s.jsonOK(w, status, 0)
}

func countItems(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	// Грубая оценка — количество JSON объектов
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return 0
	}
	if arr, ok := wrapper["providers"].([]interface{}); ok {
		return len(arr)
	}
	if arr, ok := wrapper["findings"].([]interface{}); ok {
		return len(arr)
	}
	return 0
}
