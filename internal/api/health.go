package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/orex"
	"free-api-hunter/internal/vault"
)

// ─── Existing (backward compat) ───

// HealthStatus — расширенный health check
type HealthStatus struct {
	Status         string `json:"status"`
	Time           string `json:"time"`
	Version        string `json:"version"`
	LastScanTime   string `json:"last_scan_time,omitempty"`
	ProvidersCount int    `json:"providers_count,omitempty"`
	FindingsCount  int    `json:"findings_count,omitempty"`
	ScanLogOk      bool   `json:"scan_log_ok"`
	OrexProxyOk    bool   `json:"orex_proxy_ok"`
}

// handleHealthExtended — legacy extended health check (registered as /health via handleHealth)
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

	scanLogPath := "/var/log/free-api-hunter/scan.log"
	if fi, err := os.Stat(scanLogPath); err == nil {
		status.ScanLogOk = time.Since(fi.ModTime()) < 12*time.Hour
	}

	orexClient := orex.NewClient("")
	if _, err := orexClient.GetModels(); err == nil {
		status.OrexProxyOk = true
	}

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

// ─── New Health Endpoints ───

// handleLiveness — GET /health/live
// Returns 200 if the process is running. No dependency checks.
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s.json(w, http.StatusOK, response{
		Success: true,
		Data: map[string]string{
			"status": "alive",
			"time":   time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// handleReadiness — GET /health/ready
// Checks SQLite connectivity and data directory writability.
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	checks := map[string]string{}
	allOK := true

	// 1. SQLite ping
	if db := database.DB(); db != nil {
		if err := db.Ping(); err != nil {
			checks["sqlite"] = "error: " + err.Error()
			allOK = false
		} else {
			checks["sqlite"] = "ok"
		}
	} else {
		checks["sqlite"] = "not_initialized"
	}

	// 2. Data directory writable
	if s.DataDir != "" {
		testFile := filepath.Join(s.DataDir, ".write_test")
		if f, err := os.OpenFile(testFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644); err == nil {
			f.Close()
			os.Remove(testFile)
			checks["data_dir"] = "ok"
		} else {
			checks["data_dir"] = "error: " + err.Error()
			allOK = false
		}
	} else {
		checks["data_dir"] = "not_configured"
	}

	if !allOK {
		s.json(w, http.StatusServiceUnavailable, response{
			Success: false,
			Error:   "not ready",
			Data:    checks,
		})
		return
	}
	s.json(w, http.StatusOK, response{
		Success: true,
		Data:    checks,
	})
}

// handleDeepHealth — GET /health/deep
// Checks every subsystem: vault, storage, SQLite, last scan time, Orex proxy.
func (s *Server) handleDeepHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	result := deepHealthResult{
		Status:  "ok",
		Time:    time.Now().UTC().Format(time.RFC3339),
		Version: "0.1.0",
		Checks:  make(map[string]deepCheck),
	}

	result.Checks["vault"] = checkVault()
	result.Checks["storage"] = checkStorageJSON(s.DataDir)
	result.Checks["sqlite"] = checkSQLite()
	result.Checks["last_scan"] = checkLastScan(s.DataDir)
	result.Checks["orex_proxy"] = checkOrexProxy()

	// Determine overall status
	for _, c := range result.Checks {
		if c.Status == "error" {
			result.Status = "error"
			break
		}
		if c.Status == "warn" && result.Status != "error" {
			result.Status = "warn"
		}
	}

	if result.Status == "error" {
		s.json(w, http.StatusInternalServerError, response{
			Success: false,
			Error:   "one or more subsystems unhealthy",
			Data:    result,
		})
		return
	}
	s.json(w, http.StatusOK, response{
		Success: true,
		Data:    result,
	})
}

// ─── deep health types & helpers ───

type deepHealthResult struct {
	Status  string               `json:"status"`
	Time    string               `json:"time"`
	Version string               `json:"version"`
	Checks  map[string]deepCheck `json:"checks"`
}

type deepCheck struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func checkVault() deepCheck {
	if vault.VaultPath == "" {
		return deepCheck{Status: "warn", Detail: "vault path not configured"}
	}
	if _, err := os.Stat(vault.VaultPath); err != nil {
		return deepCheck{Status: "warn", Detail: "vault dir not found: " + err.Error()}
	}
	providers, err := vault.ListProviders()
	if err != nil {
		return deepCheck{Status: "warn", Detail: "list providers error: " + err.Error()}
	}
	return deepCheck{Status: "ok", Detail: fmt.Sprintf("%d providers", len(providers))}
}

func checkStorageJSON(dataDir string) deepCheck {
	if dataDir == "" {
		return deepCheck{Status: "warn", Detail: "data dir not configured"}
	}
	p := filepath.Join(dataDir, "providers.json")
	if _, err := os.Stat(p); err != nil {
		return deepCheck{Status: "warn", Detail: "providers.json not found"}
	}
	f, err := os.Open(p)
	if err != nil {
		return deepCheck{Status: "error", Detail: "cannot open providers.json: " + err.Error()}
	}
	defer f.Close()
	return deepCheck{Status: "ok"}
}

func checkSQLite() deepCheck {
	db := database.DB()
	if db == nil {
		return deepCheck{Status: "warn", Detail: "db not initialized (legacy JSON mode)"}
	}
	if err := db.Ping(); err != nil {
		return deepCheck{Status: "error", Detail: "ping failed: " + err.Error()}
	}
	var version string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		return deepCheck{Status: "warn", Detail: "version query failed: " + err.Error()}
	}
	return deepCheck{Status: "ok", Detail: "sqlite " + version}
}

func checkLastScan(dataDir string) deepCheck {
	p := filepath.Join(dataDir, "providers.json")
	fi, err := os.Stat(p)
	if err != nil {
		return deepCheck{Status: "warn", Detail: "providers.json not found"}
	}
	age := time.Since(fi.ModTime())
	detail := fmt.Sprintf("age: %s", age.Round(time.Minute))
	if age > 24*time.Hour {
		return deepCheck{Status: "warn", Detail: detail}
	}
	return deepCheck{Status: "ok", Detail: detail}
}

func checkOrexProxy() deepCheck {
	orexClient := orex.NewClient("")
	if _, err := orexClient.GetModels(); err != nil {
		return deepCheck{Status: "warn", Detail: "orex proxy unreachable: " + err.Error()}
	}
	return deepCheck{Status: "ok"}
}

// ─── ensure sql import is used ───
var _ *sql.DB
