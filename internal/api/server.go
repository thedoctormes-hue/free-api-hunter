package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/notify"
	"free-api-hunter/internal/storage"
)

var logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})).With("service", "api")

// validVerdictsWeb — белый список вердиктов, принимаемых веб-триажем.
// Совпадает с notify.validVerdicts (алиасы not_confirmed|not_working_rf → rejected).
var validVerdictsWeb = map[string]bool{
	"confirmed":      true,
	"rejected":       true,
	"backlog":        true,
	"already_in_use": true,
	"not_confirmed":  true, // alias -> rejected
	"not_working_rf": true, // alias -> rejected
}

//scanState represents the current scan run status.
type scanState struct {
	Status    string `json:"status"`     // idle, running
	ScanID    string `json:"scan_id"`    // current run id
	StartedAt string `json:"started_at"` // RFC3339
	FinishedAt string `json:"finished_at,omitempty"`
	RawCount   int    `json:"raw_count,omitempty"`
	FilteredCount int `json:"filtered_count,omitempty"`
	ProvidersTotal int `json:"providers_total,omitempty"`
	NewFindings int `json:"new_findings,omitempty"`
	Message    string `json:"message,omitempty"`
}

// scanLimiter tracks scan runs with in-memory state + SQLite persistence.
// Uses sync.Map for lock-free rate-limit check and current state.
type scanLimiter struct {
	mu         sync.Mutex
	current    *scanState
	lastScanAt time.Time
	runs       sync.Map // scan_id -> *scanState (recent history)
}

var scan_limiter = &scanLimiter{}

const scanRateLimit = 5 * time.Minute

const defaultScanHistoryLimit = 20 // max in-memory run entries kept

func (l *scanLimiter) tryStart(scanID string) (*scanState, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current != nil && l.current.Status == "running" {
		return nil, false // already running
	}

	now := time.Now()
	if !l.lastScanAt.IsZero() && now.Sub(l.lastScanAt) < scanRateLimit {
		return nil, false // rate limited
	}

	state := &scanState{
		Status:    "running",
		ScanID:    scanID,
		StartedAt: now.UTC().Format(time.RFC3339),
	}
	l.current = state
	l.lastScanAt = now
	l.runs.Store(scanID, state)
	return state, true
}

func (l *scanLimiter) finish(scanID string, rawCount, filteredCount, providersTotal, newFindings int, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current != nil && l.current.ScanID == scanID {
		now := time.Now().UTC()
		l.current.Status = "completed"
		l.current.FinishedAt = now.Format(time.RFC3339)
		l.current.RawCount = rawCount
		l.current.FilteredCount = filteredCount
		l.current.ProvidersTotal = providersTotal
		l.current.NewFindings = newFindings
		l.current.Message = msg
	}
}

func (l *scanLimiter) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.current = nil
}

func (l *scanLimiter) getStatus() *scanState {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current == nil {
		return &scanState{Status: "idle"}
	}
	// Return a copy to avoid races
	copy := *l.current
	return &copy
}

func (l *scanLimiter) nextAvailable() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastScanAt.IsZero() {
		return ""
	}
	next := l.lastScanAt.Add(scanRateLimit)
	now := time.Now()
	if now.After(next) {
		return ""
	}
	return next.UTC().Format(time.RFC3339)
}

// Server — HTTP API сервер
type Server struct {
	Addr    string
	DataDir string
	mux     *http.ServeMux
}

// NewServer — создать новый API сервер
func NewServer(addr string) *Server {
	// Initialize database for API endpoints
	if err := storage.InitDB(""); err != nil {
		logger.Error("failed to init database", "error", err)
	}

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
		logger.Info("API server starting", "addr", s.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		logger.Info("received signal, shutting down", "signal", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown error", "error", err)
			return err
		}
		logger.Info("API server stopped gracefully")
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
	// Health (public — no auth)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/health/live", s.handleLiveness)
	s.mux.HandleFunc("/health/ready", s.handleReadiness)
	s.mux.HandleFunc("/health/deep", s.handleDeepHealth)
	// Prometheus metrics (public — no auth)
	s.mux.HandleFunc("/metrics", s.handlePrometheusMetrics)
	s.mux.HandleFunc("/", s.handleIndex)

	// Create handler with protections
	buildHandler := func(h http.HandlerFunc, public ...string) http.Handler {
		handler := s.wrapHandler(h)
		return ProtectedMiddleware(RateLimitMiddleware(CORSMiddleware(MaxSizeMiddleware(handler))), public...)
	}

	// LLM providers (protected)
	s.mux.Handle("/api/v1/providers", buildHandler(s.handleProviders))
	s.mux.Handle("/api/v1/providers/", buildHandler(s.handleProviderByID))
	s.mux.Handle("/api/v1/findings", buildHandler(s.handleFindings))
	s.mux.Handle("/api/v1/findings/verdict", buildHandler(s.handleSetVerdict))
	s.mux.Handle("/api/v1/stats", buildHandler(s.handleStats))
	// Scan history & scan trigger (protected)
	s.mux.Handle("/api/v1/scan-history", buildHandler(s.handleScanHistory))
	s.mux.Handle("/api/v1/scan", buildHandler(s.handleScanCombined))
	// TTS providers (protected)
	s.mux.Handle("/api/v1/tts/providers", buildHandler(s.handleTTSProviders))
	s.mux.Handle("/api/v1/tts/providers/", buildHandler(s.handleTTSProviderByID))
	s.mux.Handle("/api/v1/tts/stats", buildHandler(s.handleTTSStats))
}

// ListenAndServe — запустить сервер
func (s *Server) wrapHandler(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Inc("api_call_total")
		t1 := time.Now()
		defer func() {
			since := time.Since(t1).Seconds()
			SetGauge("api_response_time", since)
			if w.Header().Get("Status") >= "400" {
				Inc("api_errors")
			}
		}()
		h(w, r)
	})
}

// trackResponseWriter wraps http.ResponseWriter to capture status code.
type trackResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *trackResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *trackResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *trackResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// metricsMiddleware records api_requests_total for every request that passes through it.
// Labeled by method, route pattern (r.URL.Path), and response status code.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tw := &trackResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(tw, r)
		IncAPIRequests(r.Method, r.URL.Path, tw.status)
	})
}

func (s *Server) jsonErrWrapper(w http.ResponseWriter, r *http.Request) {
	// Reject unknown methods for wrapped handlers
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		Inc("api_method_not_allowed")
		return
	}
	// Use the original JSON error flow based on route logic
	// This wrapper ensures metrics and timing are captured
	switch r.URL.Path {
	case "/api/v1/providers":
		s.handleProviders(w, r)
	case "/api/v1/providers/":
		s.handleProviderByID(w, r)
	case "/api/v1/findings":
		s.handleFindings(w, r)
	case "/api/v1/stats":
		s.handleStats(w, r)
	case "/api/v1/scan-history":
		s.handleScanHistory(w, r)
	case "/api/v1/scan":
		if r.Method == "POST" {
			s.handleScan(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case "/api/v1/tts/providers":
		s.handleTTSProviders(w, r)
	case "/api/v1/tts/providers/":
		s.handleTTSProviderByID(w, r)
	case "/api/v1/tts/stats":
		s.handleTTSStats(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
		Inc("api_not_found")
	}
}

func (s *Server) ListenAndServe() error {
	logger.Info("API server starting", "addr", s.Addr)
	handler := CORSMiddleware(RateLimitMiddleware(metricsMiddleware(s.mux)))
	return http.ListenAndServe(s.Addr, handler)
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

// handleFindings — GET /api/v1/findings — список находок с pagination.
// Query params: limit (default 50, max 200), offset (default 0), source (filter).
func (s *Server) handleFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Pagination params
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
		if offset < 0 {
			offset = 0
		}
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

	total := len(filtered)

	// Apply offset
	if offset > 0 {
		if offset >= total {
			filtered = nil
		} else {
			filtered = filtered[offset:]
		}
	}

	// Apply limit
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	// Return with pagination metadata
	s.json(w, http.StatusOK, response{
		Success: true,
		Data:    filtered,
		Meta: &meta{
			Count:   total,
			Version: "0.1.0",
		},
	})
}

// handleSetVerdict — POST /api/v1/findings/verdict — веб-триаж находки.
// Тело JSON: {"source":"<url>","verdict":"confirmed|rejected|backlog|already_in_use"}.
// Матч по source (URL), НЕ по индексу (индекс нестабилен между ре-бриджами).
// Переиспользует notify.TriageSet — логику не дублируем.
func (s *Server) handleSetVerdict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed; only POST is supported")
		return
	}

	var req struct {
		Source  string `json:"source"`
		Verdict string `json:"verdict"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}

	if req.Source == "" || req.Verdict == "" {
		s.jsonErr(w, http.StatusBadRequest, "source and verdict are required")
		return
	}

	if !validVerdictsWeb[req.Verdict] {
		s.jsonErr(w, http.StatusBadRequest, "invalid verdict (allowed: confirmed|rejected|backlog|already_in_use)")
		return
	}

	// index=0 → TriageSet матчит по source URL.
	if err := notify.TriageSet(s.DataDir, 0, req.Verdict, req.Source); err != nil {
			msg := err.Error()
			switch {
			case strings.Contains(msg, "no pending item with source"):
				s.jsonErr(w, http.StatusNotFound, msg)
			case strings.Contains(msg, "invalid verdict"):
				s.jsonErr(w, http.StatusBadRequest, msg)
			default:
				s.jsonErr(w, http.StatusInternalServerError, fmt.Sprintf("triage failed: %v", err))
			}
		return
	}

	s.json(w, http.StatusOK, response{Success: true, Meta: &meta{Version: "0.1.0"}})
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

// handleScanCombined — routes /api/v1/scan by method:
//   POST → trigger a new scan (202 Accepted, non-blocking)
//   GET  → current scan status + timing info
func (s *Server) handleScanCombined(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleScan(w, r)
	case http.MethodGet:
		s.handleScanStatus(w, r)
	default:
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed; only GET and POST are supported")
	}
}

// handleScan — POST /api/v1/scan — trigger a scan asynchronously.
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed; use POST")
		return
	}

	scanID := fmt.Sprintf("scan-%d", time.Now().UnixNano())

	state, ok := scan_limiter.tryStart(scanID)
	if !ok {
		status := http.StatusConflict
		msg := "scan already running"
		if scan_limiter.current == nil || scan_limiter.current.Status != "running" {
			status = http.StatusTooManyRequests
			msg = fmt.Sprintf("rate limited: max 1 scan per %s", scanRateLimit)
		}
		if next := scan_limiter.nextAvailable(); next != "" {
			w.Header().Set("Retry-After", scanRateLimit.String())
		}
		logger.Warn("scan rejected", "scan_id", scanID, "reason", msg)
		s.json(w, status, response{Success: false, Error: msg, Meta: &meta{Version: "0.1.0"}})
		return
	}

	logger.Info("scan accepted", "scan_id", scanID)
	go s.runBackgroundScan(scanID)

	s.json(w, http.StatusAccepted, response{
		Success: true,
		Data: map[string]interface{}{
			"scan_id":    scanID,
			"status":     state.Status,
			"started_at": state.StartedAt,
		},
		Meta: &meta{Version: "0.1.0"},
	})
}

// handleScanStatus — GET /api/v1/scan — current scan status.
func (s *Server) handleScanStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed; use GET")
		return
	}

	state := scan_limiter.getStatus()
	resp := map[string]interface{}{"status": state.Status}

	if state.Status == "running" {
		resp["scan_id"] = state.ScanID
		resp["started_at"] = state.StartedAt
	} else {
		if state.ScanID != "" {
			resp["last_scan_id"] = state.ScanID
			resp["last_started_at"] = state.StartedAt
			resp["last_finished_at"] = state.FinishedAt
			resp["last_raw_count"] = state.RawCount
			resp["last_filtered_count"] = state.FilteredCount
			resp["last_providers_total"] = state.ProvidersTotal
			resp["last_new_findings"] = state.NewFindings
			resp["last_message"] = state.Message
		}
		if next := scan_limiter.nextAvailable(); next != "" {
			resp["next_available_at"] = next
		}
	}

	s.jsonOK(w, resp, 0)
}

// runBackgroundScan executes the real scan work asynchronously.
func (s *Server) runBackgroundScan(scanID string) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Error("scan panic", "scan_id", scanID, "recover", rec)
			scan_limiter.reset()
		}
	}()

	logger.Info("scan running", "scan_id", scanID)
	time.Sleep(2 * time.Second) // placeholder

	if err := storage.SaveScanHistory(0, 0, 0, 0); err != nil {
		logger.Error("save scan history failed", "scan_id", scanID, "error", err)
	}

	scan_limiter.finish(scanID, 0, 0, 0, 0, "Scan pipeline not wired. Placeholder completed.")
	logger.Info("scan completed", "scan_id", scanID)
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
<div class="endpoint"><span class="method">POST</span><span class="path">/api/v1/findings/verdict</span><span>— Веб-триаж (вердикт по source URL)</span></div>
<div class="endpoint"><span class="method">GET</span><span class="path"><a href="/api/v1/stats">/api/v1/stats</a></span><span>— Статистика</span></div>
<div class="endpoint"><span class="method">GET</span><span class="path"><a href="/api/v1/scan-history">/api/v1/scan-history</a></span><span>— История сканирований</span></div>
<div class="endpoint"><span class="method">POST</span><span class="path">/api/v1/scan</span><span>— Запустить сканирование</span></div>

<h2>Параметры фильтрации</h2>
<p><code>?status=verified</code> — фильтр по статусу (verified, confirmed, claimed, unverified)</p>
<p><code>?credit_card=false</code> — только без кредитной карты</p>
<p><code>?source=hackernews</code> — фильтр находок по источнику</p>
<p><code>?limit=10</code> — лимит результатов</p>
<p><code>POST /api/v1/findings/verdict</code> — тело <code>{"source":"&lt;url&gt;","verdict":"confirmed|rejected|backlog|already_in_use"}</code></p>

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
