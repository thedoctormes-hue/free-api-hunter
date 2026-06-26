// Package api provides HTTP API server with security middleware
package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
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

// testMode allows skipping authentication in tests
var testMode = false

// SetTestMode enables test mode (no auth required)
func SetTestMode() {
	testMode = true
}

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
