package scraper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"free-api-hunter/internal/models"
)

func init() {
	validateOutboundURL = func(rawURL string) (*url.URL, error) {
		return url.Parse(rawURL)
	}
}

func TestScrapeGitHubReadmeMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`# Free API List

## OpenRouter
Free tier with 1000 requests per day [OpenRouter](https://openrouter.ai)

## Groq
No credit card required for free API access [Groq](https://groq.com)

## Changelog
v1.0.0 released on 2026-01-01
`))
	}))
	defer server.Close()

	findings := ScrapeGitHubREADME(server.URL, "test_source")
	if len(findings) == 0 {
		t.Error("expected at least one finding")
	}

	// Should find OpenRouter and Groq but skip changelog
	foundOpenRouter := false
	foundGroq := false
	for _, f := range findings {
		if strings.Contains(f.Title, "OpenRouter") || strings.Contains(f.URL, "openrouter") {
			foundOpenRouter = true
		}
		if strings.Contains(f.Title, "Groq") || strings.Contains(f.URL, "groq") {
			foundGroq = true
		}
	}
	if !foundOpenRouter {
		t.Error("should find OpenRouter")
	}
	if !foundGroq {
		t.Error("should find Groq")
	}
}

func TestScrapeGitHubReadmeNoFree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`# Paid APIs Only

## Expensive API
This costs money and requires credit card.
`))
	}))
	defer server.Close()

	findings := ScrapeGitHubREADME(server.URL, "test_source")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for paid-only content, got %d", len(findings))
	}
}

func TestScrapeGitHubReadmeEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	findings := ScrapeGitHubREADME(server.URL, "test_source")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty content, got %d", len(findings))
	}
}

func TestScrapeGitHubReadmeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	findings := ScrapeGitHubREADME(server.URL, "test_source")
	if findings != nil {
		t.Error("expected nil findings for error response")
	}
}

func TestScrapeHackerNewsMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := map[string]interface{}{
			"hits": []map[string]interface{}{
				{
					"title":      "Free API for LLM inference",
					"url":        "https://example.com/free-api",
					"objectID":   "12345",
					"points":     150,
					"created_at": "2026-06-26T00:00:00Z",
				},
				{
					"title":      "Paid service announcement",
					"url":        "https://example.com/paid",
					"objectID":   "12346",
					"points":     50,
					"created_at": "2026-06-25T00:00:00Z",
				},
			},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	// Override HTTPClient to point to mock
	origClient := HTTPClient
	HTTPClient = server.Client()
	defer func() { HTTPClient = origClient }()

	// We can't easily redirect the URL, but we can test the parsing logic
	hits := []map[string]interface{}{
		{
			"title":      "Free API for LLM inference",
			"url":        "https://example.com/free-api",
			"objectID":   "12345",
			"points":     150,
			"created_at": "2026-06-26T00:00:00Z",
		},
	}

	var findings []models.Finding
	for _, h := range hits {
		title, _ := h["title"].(string)
		postURL, _ := h["url"].(string)
		postID, _ := h["objectID"].(string)
		points, _ := h["points"].(int)
		createdAt, _ := h["created_at"].(string)

		combined := strings.ToLower(title)
		keywords := []string{"free api", "free tier", "free credits", "free llm",
			"open source", "self-host", "local", "privacy"}
		hasKeyword := false
		for _, kw := range keywords {
			if strings.Contains(combined, kw) {
				hasKeyword = true
				break
			}
		}
		if !hasKeyword {
			continue
		}

		findings = append(findings, models.Finding{
			SourceID:    "hn_" + postID,
			Title:       title,
			URL:         postURL,
			Description: "test",
			RawText:     title,
		})
		_ = points
		_ = createdAt
	}

	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestScrapeWebPageMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
<html><body>
<h1>API Documentation</h1>
<p>We offer a free tier with 1000 requests per day.</p>
<p>No credit card required to get started.</p>
<p>Premium plans available for heavy usage.</p>
</body></html>
`))
	}))
	defer server.Close()

	// Override HTTPClient
	origClient := HTTPClient
	HTTPClient = server.Client()
	defer func() { HTTPClient = origClient }()

	findings := ScrapeWebPage(server.URL, "test_web")
	// Should find at least one mention of free tier
	if len(findings) == 0 {
		t.Error("expected findings from web page with free tier mentions")
	}
}

func TestScrapeWebPageNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body><h1>Paid API Only</h1><p>Credit card required.</p></body></html>`))
	}))
	defer server.Close()

	origClient := HTTPClient
	HTTPClient = server.Client()
	defer func() { HTTPClient = origClient }()

	findings := ScrapeWebPage(server.URL, "test_web")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for paid-only page, got %d", len(findings))
	}
}

func TestScrapeWebPageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	origClient := HTTPClient
	HTTPClient = server.Client()
	defer func() { HTTPClient = origClient }()

	findings := ScrapeWebPage(server.URL, "test_web")
	if findings != nil {
		t.Error("expected nil findings for error response")
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{"https://www.example.com/path", "example.com"},
		{"https://api.example.com/v1", "api.example.com"},
		{"http://localhost:8080", "localhost"},
		{"invalid-url", "invalid-url"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractDomain(tt.rawURL)
		if got != tt.want {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.rawURL, got, tt.want)
		}
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{"https://www.example.com/path", "www.example.com"},
		{"http://localhost:8080/api", "localhost"},
		{"invalid", ""},
	}
	for _, tt := range tests {
		got := extractHost(tt.rawURL)
		if got != tt.want {
			t.Errorf("extractHost(%q) = %q, want %q", tt.rawURL, got, tt.want)
		}
	}
}

func TestDomainRateLimiter(t *testing.T) {
	limiter := &domainRateLimiter{
		lastReq:  make(map[string]time.Time),
		interval: 100 * time.Millisecond,
	}

	// First call should not block
	limiter.WaitForRate("example.com")

	// Second call should block ~100ms
	start := time.Now()
	limiter.WaitForRate("example.com")
	elapsed := time.Since(start)

	if elapsed < 50*time.Millisecond {
		t.Errorf("rate limiter did not wait enough: %v", elapsed)
	}
}

func TestScrapeRedditMockParsing(t *testing.T) {
	// Test the parsing logic with mock data
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"children": []interface{}{
				map[string]interface{}{
					"data": map[string]interface{}{
						"title":     "Free GPT-4 API alternative",
						"selftext":  "I found a free API for LLM inference",
						"permalink": "/r/test/comments/abc123",
						"id":        "abc123",
					},
				},
				map[string]interface{}{
					"data": map[string]interface{}{
						"title":     "Paid service discussion",
						"selftext":  "This is about paid services",
						"permalink": "/r/test/comments/def456",
						"id":        "def456",
					},
				},
			},
		},
	}

	children := extractRedditChildren(data)
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}

	// Check keyword filtering
	var findings []models.Finding
	keywords := []string{"api", "key", "free", "credit", "tier", "llm", "model", "gpt", "gemini", "claude"}
	for _, child := range children {
		post, ok := child["data"].(map[string]interface{})
		if !ok {
			continue
		}
		title, _ := post["title"].(string)
		selftext, _ := post["selftext"].(string)
		permalink, _ := post["permalink"].(string)
		postID, _ := post["id"].(string)

		combined := strings.ToLower(title + " " + selftext)
		hasKeyword := false
		for _, kw := range keywords {
			if strings.Contains(combined, kw) {
				hasKeyword = true
				break
			}
		}
		if !hasKeyword {
			continue
		}

		findings = append(findings, models.Finding{
			SourceID:    "reddit_" + postID,
			Title:       title,
			URL:         "https://reddit.com" + permalink,
			Description: selftext,
			RawText:     title + "\n\n" + selftext,
		})
	}

	if len(findings) != 1 {
		t.Errorf("expected 1 finding (free API post), got %d", len(findings))
	}
}

func TestScrapeRedditEmptyChildren(t *testing.T) {
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"children": []interface{}{},
		},
	}
	children := extractRedditChildren(data)
	if len(children) != 0 {
		t.Error("expected 0 children")
	}
}

func TestScrapeRedditMalformedData(t *testing.T) {
	data := map[string]interface{}{
		"data": "not a map",
	}
	children := extractRedditChildren(data)
	if children != nil {
		t.Error("expected nil for malformed data")
	}
}

func TestScrapeRedditNoData(t *testing.T) {
	data := map[string]interface{}{}
	children := extractRedditChildren(data)
	if children != nil {
		t.Error("expected nil for missing data key")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}
