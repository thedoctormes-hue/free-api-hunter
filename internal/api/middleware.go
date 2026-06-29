// Package api provides HTTP API server with security middleware
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Rate Limiting: 100 requests per minute per IP
// ============================================================

// rateLimiter tracks request counts per IP
// Maps: IP -> [bucketStartTime, count]
type rateLimiter struct {
	mu                   sync.Mutex
	requestCounts        map[string][2]interface{} // [time.Time, int]
	windowDuration       time.Duration
	maxRequestsPerWindow int
}

var globalRateLimiter = &rateLimiter{
	requestCounts:        make(map[string][2]interface{}),
	windowDuration:       1 * time.Minute,
	maxRequestsPerWindow: 100,
}

// RateLimitMiddleware enforces 100 requests per minute per IP
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		globalRateLimiter.mu.Lock()
		defer globalRateLimiter.mu.Unlock()

		now := time.Now().UTC()
		entry, exists := globalRateLimiter.requestCounts[ip]

		if exists {
			lastTime, ok1 := entry[0].(time.Time)
			count, ok2 := entry[1].(int)
			if !ok1 || !ok2 {
				count = 0
			}

			if now.Sub(lastTime) > globalRateLimiter.windowDuration {
				count = 0
			}

			if count >= globalRateLimiter.maxRequestsPerWindow {
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", globalRateLimiter.windowDuration.Seconds()))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				Inc("rate_limit_hit")
				return
			}

			globalRateLimiter.requestCounts[ip] = [2]interface{}{now, count + 1}
			SetGauge("rate_limit_concurrent", float64(count+1))
		} else {
			globalRateLimiter.requestCounts[ip] = [2]interface{}{now, 1}
			SetGauge("rate_limit_concurrent", 1)
		}

		next.ServeHTTP(w, r)
	})
}

// realIP extracts real client IP from headers or RemoteAddr
func realIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ss := strings.Split(fwd, ",")
		if len(ss) > 0 && net.ParseIP(ss[0]) != nil {
			return ss[0]
		}
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		if net.ParseIP(realIP) != nil {
			return realIP
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if host != "" {
		return host
	}
	return r.RemoteAddr
}

// ============================================================
// CORS Middleware: Only allow localhost origins
// ============================================================

var AllowedOrigins = []string{"localhost", "127.0.0.1", "::1"}

// CORSMiddleware validates Origin header against safelist
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		allowed := false
		host := originHost(origin)
		for _, allowedHost := range AllowedOrigins {
			if strings.EqualFold(host, allowedHost) {
				allowed = true
				break
			}
		}

		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if r.Method == "OPTIONS" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
				w.WriteHeader(http.StatusOK)
				Inc("cors_preflight")
				return
			}
		} else {
			if r.Method == "OPTIONS" {
				http.Error(w, "CORS origin not allowed", http.StatusForbidden)
				Inc("cors_forbidden")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func originHost(origin string) string {
	if strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://") {
		if u, err := url.Parse(origin); err == nil {
			u.Scheme = ""
			u.User = nil
			return strings.ToLower(u.String())
		}
	}
	return strings.ToLower(origin)
}

// ============================================================
// Request Size Limit: 1MB
// ============================================================

const MaxRequestSize = 1 << 20 // 1 MB

// MaxSizeMiddleware enforces 1MB request size limit
func MaxSizeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.ContentLength > MaxRequestSize {
			http.Error(w, "Request entity too large", http.StatusRequestEntityTooLarge)
			Inc("request_size_limited")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)
		next.ServeHTTP(w, r)
	})
}

// testMode allows skipping authentication in tests (set via TEST_MODE=true env)
var testMode = os.Getenv("TEST_MODE") == "true"

var allowedAPIKeys = []string{os.Getenv("FREE_API_HUNTER_API_KEY")}

var apiKeyLock sync.Mutex

// SetAPIKeys updates allowed API keys
func SetAPIKeys(keys []string) {
	apiKeyLock.Lock()
	defer apiKeyLock.Unlock()
	allowedAPIKeys = append(allowedAPIKeys, keys...)
}

// ProtectedMiddleware authenticates protected endpoints with X-API-Key header
func ProtectedMiddleware(next http.Handler, publicPrefixes ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		isPublic := false
		for _, prefix := range publicPrefixes {
			if strings.HasPrefix(path, prefix) {
				isPublic = true
				break
			}
		}

		if isPublic {
			next.ServeHTTP(w, r)
			return
		}

		providedKey := r.Header.Get("X-API-Key")
		if providedKey == "" {
			http.Error(w, "Missing API Key", http.StatusUnauthorized)
			Inc("auth_failure_api_missing")
			return
		}

		apiKeyLock.Lock()
		hasAccess := false
		for _, key := range allowedAPIKeys {
			if key != "" && cmdCompare(providedKey, key) {
				hasAccess = true
				break
			}
		}
		apiKeyLock.Unlock()

		if !hasAccess {
			http.Error(w, "Invalid API Key", http.StatusForbidden)
			Inc("auth_failure_api_invalid")
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), "auth", "api-key"))
		next.ServeHTTP(w, r)
	})
}

// cmdCompare is constant-time comparison to resist timing attacks
func cmdCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	result := 0
	for i := range a {
		result |= int(a[i] ^ b[i])
	}
	return result == 0
}

// ============================================================
// Structured JSON Logger + Response Headers
// ============================================================

// jsonLogger is the structured logger instance (JSON handler, level from env)
var jsonLogger *slog.Logger

func init() {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	jsonLogger = slog.New(handler)
}

// contextKey stores the request ID in context
type contextKey string

const requestIDKey contextKey = "requestID"

// RequestIDFromContext extracts the request ID from context
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// LoggingMiddleware adds request ID, response headers and structured JSON logging.
// It does NOT modify any existing handlers or business logic.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate request ID (UUID v4)
		reqID := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		r = r.WithContext(ctx)

		// Set X-Request-ID header immediately so it's present even on errors
		w.Header().Set("X-Request-ID", reqID)

		// Set X-RateLimit headers (from global state for visibility)
		ip := realIP(r)
		globalRateLimiter.mu.Lock()
		entry, exists := globalRateLimiter.requestCounts[ip]
		var currentCount int
		var windowStart time.Time
		if exists {
			if t, ok := entry[0].(time.Time); ok {
				windowStart = t
			}
			if c, ok := entry[1].(int); ok {
				currentCount = c
			}
		}
		globalRateLimiter.mu.Unlock()

		resetSeconds := 0
		if !windowStart.IsZero() {
			resetDur := globalRateLimiter.windowDuration - time.Since(windowStart)
			if resetDur > 0 {
				resetSeconds = int(resetDur.Seconds())
			}
		}
		remaining := globalRateLimiter.maxRequestsPerWindow - currentCount
		if remaining < 0 {
			remaining = 0
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", globalRateLimiter.maxRequestsPerWindow))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetSeconds))

		// Wrap response writer to capture status
		rw := newResponseWriter(w)

		// Call next handler
		next.ServeHTTP(rw, r)

		// Calculate response time
		duration := time.Since(start)
		w.Header().Set("X-Response-Time", fmt.Sprintf("%dms", duration.Milliseconds()))

		// Cache-Control for cacheable endpoints
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/v1/providers") || strings.HasPrefix(path, "/api/v1/findings") {
			w.Header().Set("Cache-Control", "max-age=60, stale-while-revalidate=300")
		}

		// Structured JSON log entry
		jsonLogger.LogAttrs(r.Context(), levelForStatus(rw.status),
			"http_request",
			slog.String("request_id", reqID),
			slog.String("method", r.Method),
			slog.String("path", path),
			slog.String("ip", ip),
			slog.String("user_agent", r.UserAgent()),
			slog.Int("status", rw.status),
			slog.Duration("duration", duration),
			slog.Int("rate_limit_remaining", remaining),
		)
	})
}

// levelForStatus returns the appropriate log level based on HTTP status code
func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	case status >= 300:
		return slog.LevelInfo
	default:
		if jsonLogger.Enabled(context.Background(), slog.LevelDebug) {
			return slog.LevelDebug
		}
		return slog.LevelInfo
	}
}
