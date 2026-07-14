package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddlewareAllowed(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := CORSMiddleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "localhost")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "localhost" {
		t.Errorf("expected allow-origin=localhost, got %q", got)
	}
}

func TestCORSMiddlewarePreflightAllowed(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := CORSMiddleware(next)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "127.0.0.1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected allow-methods header")
	}
}

func TestCORSMiddlewareDisallowedOptions(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := CORSMiddleware(next)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCORSMiddlewareDisallowedNonOptions(t *testing.T) {
	// Disallowed origin + non-OPTIONS → request passes through to next.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) })
	h := CORSMiddleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected passthrough 201, got %d", w.Code)
	}
}

func TestCORSMiddlewareNoOrigin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := CORSMiddleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestOriginHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://Example.com/foo", "//example.com/foo"},
		{"http://localhost", "//localhost"},
		{"PlainHost", "plainhost"},
	}
	for _, c := range cases {
		if got := originHost(c.in); got != c.want {
			t.Errorf("originHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaxSizeMiddlewareTooLarge(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := MaxSizeMiddleware(next)

	req := httptest.NewRequest("POST", "/", nil)
	req.ContentLength = MaxRequestSize + 1
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", w.Code)
	}
}

func TestMaxSizeMiddlewareOK(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := MaxSizeMiddleware(next)

	req := httptest.NewRequest("POST", "/", nil)
	req.ContentLength = 100
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestProtectedMiddlewarePublic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := ProtectedMiddleware(next, "/api/v1/providers")

	req := httptest.NewRequest("GET", "/api/v1/providers", nil) // public, no key
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 for public route, got %d", w.Code)
	}
}

func TestProtectedMiddlewareMissingKey(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := ProtectedMiddleware(next, "/api/v1/providers")

	req := httptest.NewRequest("GET", "/api/v1/scan", nil) // not public, no key
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestProtectedMiddlewareInvalidKey(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := ProtectedMiddleware(next, "/api/v1/providers")

	req := httptest.NewRequest("GET", "/api/v1/scan", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestProtectedMiddlewareValidKey(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := ProtectedMiddleware(next, "/api/v1/providers")

	req := httptest.NewRequest("GET", "/api/v1/scan", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCmdCompare(t *testing.T) {
	if !cmdCompare("abc", "abc") {
		t.Error("expected equal strings to match")
	}
	if cmdCompare("abc", "abd") {
		t.Error("expected different strings to not match")
	}
	if cmdCompare("abc", "ab") {
		t.Error("expected different-length strings to not match")
	}
}

func TestRequestIDFromContext(t *testing.T) {
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("expected empty for missing key, got %q", got)
	}
	ctx := context.WithValue(context.Background(), requestIDKey, "req-xyz")
	if got := RequestIDFromContext(ctx); got != "req-xyz" {
		t.Errorf("expected req-xyz, got %q", got)
	}
}

func TestNewResponseWriterWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)
	rw.WriteHeader(404)
	rw.WriteHeader(200) // second write must be ignored
	if rec.Code != 404 {
		t.Errorf("expected 404 (first write wins), got %d", rec.Code)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202) })
	h := LoggingMiddleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 202 {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header")
	}
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header")
	}
}

func TestLevelForStatus(t *testing.T) {
	cases := []struct {
		status int
		want   slog.Level
	}{
		{500, slog.LevelError},
		{404, slog.LevelWarn},
		{301, slog.LevelInfo},
		{200, slog.LevelInfo},
	}
	for _, c := range cases {
		if got := levelForStatus(c.status); got != c.want {
			t.Errorf("levelForStatus(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 70.41.3.18")
	if got := realIP(req); got != "203.0.113.5" {
		t.Errorf("expected 203.0.113.5, got %q", got)
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-Real-IP", "198.51.100.7")
	if got := realIP(req2); got != "198.51.100.7" {
		t.Errorf("expected 198.51.100.7, got %q", got)
	}

	req3 := httptest.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "192.0.2.1:1234"
	if got := realIP(req3); got != "192.0.2.1" {
		t.Errorf("expected 192.0.2.1, got %q", got)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := RateLimitMiddleware(next)

	// Override the (TestMain-disabled) limit locally so this test still
	// exercises the real throttling path independently of other tests.
	old := globalRateLimiter.maxRequestsPerWindow
	globalRateLimiter.maxRequestsPerWindow = 100
	defer func() { globalRateLimiter.maxRequestsPerWindow = old }()

	// Unique IP to avoid interfering with other tests' rate-limit state.
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.20.30.40:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("request %d unexpected status %d", i, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.20.30.40:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after 100 requests, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Retry-After"), "60") {
		t.Errorf("expected Retry-After header, got %q", w.Header().Get("Retry-After"))
	}
}
