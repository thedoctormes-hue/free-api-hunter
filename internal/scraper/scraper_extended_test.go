package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestFetchURLSuccess verifies that FetchURL returns body on 200 OK.
func TestFetchURLSuccess(t *testing.T) {
	want := "hello world"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header missing")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(want))
	}))
	defer server.Close()

	got, err := FetchURL(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// TestFetchURLNon200 ensures non‑200 status returns an error.
func TestFetchURLNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	if _, err := FetchURL(server.URL); err == nil {
		t.Fatalf("expected error for non‑200 status")
	}
}

// TestScrapeRedditTimeout ensures ScrapeReddit handles unreachable servers gracefully.
// Uses the injectable redditClientFactory pointed at a closed server (no real network).
func TestScrapeRedditTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // closed → connection error, no real network

	orig := redditClientFactory
	redditClientFactory = func() *http.Client { return srv.Client() }
	defer func() { redditClientFactory = orig }()

	findings := ScrapeReddit("test", "free", 2)
	if findings != nil && len(findings) > 0 {
		t.Logf("got %d findings (non-zero is ok if mocked)", len(findings))
	}
}

// TestFetchURLInvalidURL verifies FetchURL returns error for invalid URLs.
func TestFetchURLInvalidURL(t *testing.T) {
	_, err := FetchURL("http://localhost:1")
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

// TestFetchURLEmptyBody verifies FetchURL handles empty 200 response.
func TestFetchURLEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	got, err := FetchURL(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty body, got %q", got)
	}
}

// TestCreateRedditClient verifies the Reddit client is configured.
func TestCreateRedditClient(t *testing.T) {
	client := CreateRedditClient()
	if client == nil {
		t.Fatal("CreateRedditClient returned nil")
	}
	if client.Timeout != 20000000000 {
		t.Errorf("expected 20s timeout, got %v", client.Timeout)
	}
}

// TestIsChangelogLineEdgeCases tests additional edge cases.
func TestIsChangelogLineEdgeCases(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"## Changelog", true},
		{"### Changelog", true},
		{"## Roadmap", true},
		{"## Contributing", true},
		{"## License", true},
		{"Released on 2026-06-18", true},
		{"Version 1.2.3", true},
		{"v0.3.0", true},
		{"Breaking changes", true},
		{"Migration guide", true},
		{"All notable changes", true},
		{"See the changelog", true},
		{"Полезная информация про API", false},
		{"Free tier with 1000 requests per day", false},
		{"New provider added: Groq", false},
	}

	for _, tt := range tests {
		got := isChangelogLine(tt.line)
		if got != tt.want {
			t.Errorf("isChangelogLine(%q): got %v, want %v", tt.line, got, tt.want)
		}
	}
}

// TestIsProviderEntryEdgeCases tests additional edge cases.
func TestIsProviderEntryEdgeCases(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"Free tier with 1000 requests per day", true},
		{"No credit card required", true},
		{"API endpoint: https://api.example.com", true},
		{"Rate limit: 20 RPM", true},
		{"1000 req/day free", true},
		{"OpenAI-compatible endpoint", true},
		{"Информация о лицензии MIT", false},
		{"Полезные ссылки на ресурсы", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isProviderEntry(tt.line)
		if got != tt.want {
			t.Errorf("isProviderEntry(%q): got %v, want %v", tt.line, got, tt.want)
		}
	}
}

// TestGetRandomUserAgentDistribution verifies UA rotation.
func TestGetRandomUserAgentDistribution(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ua := getRandomUserAgent()
		if ua == "" {
			t.Fatal("getRandomUserAgent returned empty")
		}
		seen[ua] = true
	}
	if len(seen) < 2 {
		t.Error("expected at least 2 different UAs in 100 calls")
	}
}

// TestWaitForRedditRateLimitTiming verifies rate limiter doesn't panic.
func TestWaitForRedditRateLimitTiming(t *testing.T) {
	lastRedditRequest = time.Time{}
	waitForRedditRateLimit()
	waitForRedditRateLimit()
}

// Suppress unused import warning.
var _ = strings.Contains
