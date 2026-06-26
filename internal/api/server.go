package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/storage"
)

var logger = log.New(log.Writer(), "[api] ", log.LstdFlags)

// Server — HTTP API сервер
type Server struct {
	Addr    string
	DataDir string
	mux     *http.ServeMux
}

// NewServer — создать новый API сервер
func NewServer(addr string) *Server {
	s := &Server{
		Addr:    addr,
		DataDir: storage.DataDir,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

// ListenAndServeGraceful — запустить сервер с graceful shutdown.
// Сервер завершается при получении SIGINT или SIGTERM.
func (s *Server) ListenAndServeGraceful() error {
	srv := &http.Server{
		Addr:    s.Addr,
		Handler: s.mux,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		logger.Printf("API server starting on %s", s.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		logger.Printf("Received signal %v, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Printf("Graceful shutdown error: %v", err)
			return err
		}
		logger.Println("API server stopped gracefully")
		return nil
	}
}

// NewServerWithDir — создать сервер с кастомной директорией данных
func NewServerWithDir(addr, dataDir string) *Server {
	s := &Server{
		Addr:    addr,
		DataDir: dataDir,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// LLM providers
	s.mux.HandleFunc("/api/v1/providers", s.handleProviders)
	s.mux.HandleFunc("/api/v1/providers/", s.handleProviderByID)
	s.mux.HandleFunc("/api/v1/findings", s.handleFindings)
	s.mux.HandleFunc("/api/v1/stats", s.handleStats)
	// Scan history & scan trigger
	s.mux.HandleFunc("/api/v1/scan-history", s.handleScanHistory)
	s.mux.HandleFunc("/api/v1/scan", s.handleScan)
	// TTS providers
	s.mux.HandleFunc("/api/v1/tts/providers", s.handleTTSProviders)
	s.mux.HandleFunc("/api/v1/tts/providers/", s.handleTTSProviderByID)
	s.mux.HandleFunc("/api/v1/tts/stats", s.handleTTSStats)
	// Health & index
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/", s.handleIndex)
}

// ListenAndServe — запустить сервер
func (s *Server) ListenAndServe() error {
	logger.Printf("API server starting on %s", s.Addr)
	return http.ListenAndServe(s.Addr, s.mux)
}

// response — стандартный JSON ответ
type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *meta       `json:"meta,omitempty"`
}

type meta struct {
	Count   int    `json:"count"`
	Version string `json:"version"`
}

func (s *Server) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	enc.Encode(data)
}

func (s *Server) jsonOK(w http.ResponseWriter, data interface{}, count int) {
	s.json(w, http.StatusOK, response{
		Success: true,
		Data:    data,
		Meta: &meta{
			Count:   count,
			Version: "0.1.0",
		},
	})
}

func (s *Server) jsonErr(w http.ResponseWriter, status int, msg string) {
	s.json(w, status, response{
		Success: false,
		Error:   msg,
	})
}

// handleProviders — GET /api/v1/providers — список провайдеров
func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Параметры фильтрации
	statusFilter := r.URL.Query().Get("status")
	creditCardFilter := r.URL.Query().Get("credit_card")

	providers, err := storage.LoadProviders("")
	if err != nil {
		s.jsonErr(w, http.StatusInternalServerError, fmt.Sprintf("load error: %v", err))
		return
	}

	// Фильтрация
	var filtered []*models.Provider
	for _, p := range providers {
		if statusFilter != "" && string(p.Status) != statusFilter {
			continue
		}
		if creditCardFilter == "false" && p.CreditCard {
			continue
		}
		if creditCardFilter == "true" && !p.CreditCard {
			continue
		}
		filtered = append(filtered, p)
	}

	s.jsonOK(w, filtered, len(filtered))
}

// handleProviderByID — GET /api/v1/providers/{id} — провайдер по ID
func (s *Server) handleProviderByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Извлечь ID из пути: /api/v1/providers/{id}
	prefix := "/api/v1/providers/"
	id := strings.TrimPrefix(r.URL.Path, prefix)
	if id == "" {
		s.jsonErr(w, http.StatusBadRequest, "provider id required")
		return
	}

	providers, err := storage.LoadProviders("")
	if err != nil {
		s.jsonErr(w, http.StatusInternalServerError, fmt.Sprintf("load error: %v", err))
		return
	}

	for _, p := range providers {
		if p.Name == id {
			s.jsonOK(w, p, 1)
			return
		}
	}

	s.jsonErr(w, http.StatusNotFound, "provider not found")
}

// handleFindings — GET /api/v1/findings — список находок
func (s *Server) handleFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	sourceFilter := r.URL.Query().Get("source")

	findings, err := storage.LoadFindings("")
	if err != nil {
		s.jsonErr(w, http.StatusInternalServerError, fmt.Sprintf("load error: %v", err))
		return
	}

	// Фильтрация по источнику (SourceID)
	var filtered []*models.Finding
	for _, f := range findings {
		if sourceFilter != "" && f.SourceID != sourceFilter {
			continue
		}
		filtered = append(filtered, f)
	}

	// Лимит
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	s.jsonOK(w, filtered, len(filtered))
}

// handleStats — GET /api/v1/stats — статистика
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	providers, _ := storage.LoadProviders("")
	findings, _ := storage.LoadFindings("")

	stats := map[string]interface{}{
		"providers_total":     0,
		"providers_by_status": map[string]int{},
		"providers_no_cc":     0,
		"findings_total":      0,
		"findings_by_source":  map[string]int{},
		"models_total":        0,
		"server_time":         time.Now().UTC().Format(time.RFC3339),
	}

	if providers != nil {
		stats["providers_total"] = len(providers)
		byStatus := map[string]int{}
		noCC := 0
		modelsTotal := 0
		for _, p := range providers {
			byStatus[string(p.Status)]++
			if !p.CreditCard {
				noCC++
			}
			modelsTotal += len(p.Models)
		}
		stats["providers_by_status"] = byStatus
		stats["providers_no_cc"] = noCC
		stats["models_total"] = modelsTotal
	}

	if findings != nil {
		stats["findings_total"] = len(findings)
		bySource := map[string]int{}
		for _, f := range findings {
			bySource[f.SourceID]++
		}
		stats["findings_by_source"] = bySource
	}

	s.jsonOK(w, stats, 0)
}

// handleScanHistory — GET /api/v1/scan-history — история сканирований
func (s *Server) handleScanHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	history, err := storage.LoadScanHistory(limit)
	if err != nil {
		s.jsonErr(w, http.StatusInternalServerError, fmt.Sprintf("load error: %v", err))
		return
	}

	s.jsonOK(w, history, len(history))
}

// handleScan — POST /api/v1/scan — запустить сканирование (заглушка)
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.jsonOK(w, map[string]string{
		"status":  "not_implemented",
		"message": "Scan trigger via API is not yet implemented. Use CLI mode.",
	}, 0)
}

// handleHealth — GET /health — health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonOK(w, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}, 0)
}

// handleIndex — GET / — корневая страница с документацией API
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.jsonErr(w, http.StatusNotFound, "not found")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="utf-8">
<title>Free API Hunter — API</title>
<style>
body{font-family:system-ui,sans-serif;max-width:800px;margin:40px auto;padding:0 20px;background:#0d1117;color:#c9d1d9}
h1{color:#58a6ff} h2{color:#79c0ff;border-bottom:1px solid #21262d;padding-bottom:8px}
code{background:#161b22;padding:2px 6px;border-radius:4px;font-size:14px}
pre{background:#161b22;padding:16px;border-radius:8px;overflow-x:auto}
.endpoint{display:flex;gap:12px;align-items:center;margin:8px 0}
.method{color:#3fb950;font-weight:bold;min-width:60px}
.path{color:#d2a8ff}
a{color:#58a6ff}
</style>
</head>
<body>
<h1>&#128269; Free API Hunter</h1>
<p>API для доступа к каталогу бесплатных LLM API.</p>

<h2>Endpoints</h2>
<div class="endpoint"><span class="method">GET</span><span class="path"><a href="/health">/health</a></span><span>— Health check</span></div>
<div class="endpoint"><span class="method">GET</span><span class="path"><a href="/api/v1/providers">/api/v1/providers</a></span><span>— Список провайдеров</span></div>
<div class="endpoint"><span class="method">GET</span><span class="path">/api/v1/providers/{id}</span><span>— Провайдер по ID</span></div>
<div class="endpoint"><span class="method">GET</span><span class="path"><a href="/api/v1/findings">/api/v1/findings</a></span><span>— Список находок</span></div>
<div class="endpoint"><span class="method">GET</span><span class="path"><a href="/api/v1/stats">/api/v1/stats</a></span><span>— Статистика</span></div>
<div class="endpoint"><span class="method">GET</span><span class="path"><a href="/api/v1/scan-history">/api/v1/scan-history</a></span><span>— История сканирований</span></div>
<div class="endpoint"><span class="method">POST</span><span class="path">/api/v1/scan</span><span>— Запустить сканирование</span></div>

<h2>Параметры фильтрации</h2>
<p><code>?status=verified</code> — фильтр по статусу (verified, confirmed, claimed, unverified)</p>
<p><code>?credit_card=false</code> — только без кредитной карты</p>
<p><code>?source=hackernews</code> — фильтр находок по источнику</p>
<p><code>?limit=10</code> — лимит результатов</p>

<h2>Пример</h2>
<pre>curl http://localhost:8080/api/v1/providers?status=verified&credit_card=false</pre>
</body>
</html>`))
}

// ─── TTS Providers ───

// ttsData — структура TTS-данных из файла
type ttsData struct {
	Providers []*models.TTSProvider     `json:"providers"`
	Results   []*models.TTSVerifyResult `json:"verify_results"`
	Scores    []*models.TTSScore        `json:"scores"`
	UpdatedAt string                    `json:"updated_at"`
}

func (s *Server) loadTTSData() *ttsData {
	path := filepath.Join(s.DataDir, "tts_providers.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var d ttsData
	if err := json.Unmarshal(data, &d); err != nil {
		return nil
	}
	return &d
}

// handleTTSProviders — GET /api/v1/tts/providers
func (s *Server) handleTTSProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	data := s.loadTTSData()
	if data == nil {
		s.jsonErr(w, http.StatusNotFound, "TTS data not found. Run with --verify first.")
		return
	}

	s.jsonOK(w, data.Providers, len(data.Providers))
}

// handleTTSProviderByID — GET /api/v1/tts/providers/{id}
func (s *Server) handleTTSProviderByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	prefix := "/api/v1/tts/providers/"
	id := strings.TrimPrefix(r.URL.Path, prefix)
	if id == "" {
		s.jsonErr(w, http.StatusBadRequest, "provider id required")
		return
	}

	data := s.loadTTSData()
	if data == nil {
		s.jsonErr(w, http.StatusNotFound, "TTS data not found.")
		return
	}

	for _, p := range data.Providers {
		if p.Name == id {
			s.jsonOK(w, p, 1)
			return
		}
	}

	s.jsonErr(w, http.StatusNotFound, "TTS provider not found")
	return
}

// handleTTSStats — GET /api/v1/tts/stats
func (s *Server) handleTTSStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	data := s.loadTTSData()
	if data == nil {
		s.jsonErr(w, http.StatusNotFound, "TTS data not found.")
		return
	}

	stats := map[string]interface{}{
		"providers_total": len(data.Providers),
		"active_count":    0,
		"free_tier_count": 0,
		"total_voices":    0,
		"updated_at":      data.UpdatedAt,
	}

	for _, r := range data.Results {
		if r.IsActive {
			stats["active_count"] = stats["active_count"].(int) + 1
		}
		stats["total_voices"] = stats["total_voices"].(int) + len(r.Voices)
	}

	for _, p := range data.Providers {
		if p.FreeTier != nil && p.FreeTier.CharLimit > 0 {
			stats["free_tier_count"] = stats["free_tier_count"].(int) + 1
		}
	}

	s.jsonOK(w, stats, 0)
}

// ProvidersFromFile — загрузить провайдеров из файла (для тестов)
func ProvidersFromFile(path string) ([]*models.Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Providers []*models.Provider `json:"providers"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Providers, nil
}
