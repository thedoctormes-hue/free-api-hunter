package scraper

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// allowAllURL overrides the SSRF guard so httptest (localhost) URLs are permitted.
func allowAllURL(t *testing.T) {
	t.Helper()
	orig := validateOutboundURL
	validateOutboundURL = func(rawURL string) (*url.URL, error) {
		u, _ := url.Parse(rawURL)
		return u, nil
	}
	t.Cleanup(func() { validateOutboundURL = orig })
}

// hostRewritingTransport routes every request to the target httptest server,
// regardless of the original host (used for hardcoded external URLs like HN).
type hostRewritingTransport struct {
	target *url.URL
	rt     http.RoundTripper
}

func (t *hostRewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = t.target.Scheme
	req2.URL.Host = t.target.Host
	req2.Host = t.target.Host
	return t.rt.RoundTrip(req2)
}

// mockClientTo returns an HTTP client that redirects all requests to srv.
func mockClientTo(t *testing.T, srv *httptest.Server) *http.Client {
	u, _ := url.Parse(srv.URL)
	return &http.Client{
		Transport: &hostRewritingTransport{target: u, rt: srv.Client().Transport},
	}
}

func withHTTPClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	SetHTTPClient(mockClientTo(t, srv))
	t.Cleanup(func() { SetHTTPClient(nil) })
}

func TestSetHTTPClientReset(t *testing.T) {
	// nil resets to default client (must not panic on use).
	SetHTTPClient(nil)
	if HTTPClient == nil {
		t.Fatal("HTTPClient should not be nil after reset")
	}
}

func TestScrapeProviderPage(t *testing.T) {
	allowAllURL(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<html>free api tier</html>")
	}))
	defer srv.Close()
	withHTTPClient(t, srv)

	body, err := ScrapeProviderPage(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "free api") {
		t.Fatalf("got %q", body)
	}
}

func TestScrapeHackerNewsMockExternal(t *testing.T) {
	allowAllURL(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"hits":[{"title":"Free API for LLM","url":"https://x.com","objectID":"123","points":10,"created_at":"2024"}]}`)
	}))
	defer srv.Close()
	withHTTPClient(t, srv)

	findings := ScrapeHackerNews("free api", 25)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding from mocked HN")
	}
}

func TestScrapeHackerNewsInvalidJSON(t *testing.T) {
	allowAllURL(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "not json")
	}))
	defer srv.Close()
	withHTTPClient(t, srv)

	findings := ScrapeHackerNews("q", 5)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on invalid JSON, got %d", len(findings))
	}
}

func TestScrapeProviderPageRejected(t *testing.T) {
	orig := validateOutboundURL
	validateOutboundURL = func(rawURL string) (*url.URL, error) {
		return nil, fmt.Errorf("blocked by policy")
	}
	t.Cleanup(func() { validateOutboundURL = orig })

	_, err := ScrapeProviderPage("http://evil.example.com")
	if err == nil {
		t.Fatal("expected rejection error")
	}
}

func TestRunScraperAllTypes(t *testing.T) {
	allowAllURL(t)
	ghContent := "# README\n## Free APIs\n- [Groq](https://groq.com) free api key without card\n"
	webContent := "we provide a free tier for self-hosting\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "search") {
			io.WriteString(w, `{"results":[{"title":"Free LLM","url":"https://together.ai","content":"free llm"}]}`)
			return
		}
		if strings.Contains(r.URL.Path, "readme") || strings.Contains(r.URL.Path, "raw") {
			io.WriteString(w, ghContent)
			return
		}
		io.WriteString(w, webContent)
	}))
	defer srv.Close()
	withHTTPClient(t, srv)

	// Reddit source via injectable factory (no real network).
	redditSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":{"children":[{"data":{"title":"Free API","selftext":"free api for llm","permalink":"/r/x/1","id":"1"}}]}}`)
	}))
	defer redditSrv.Close()
	origFactory := redditClientFactory
	redditClientFactory = func() *http.Client { return mockClientTo(t, redditSrv) }
	t.Cleanup(func() { redditClientFactory = origFactory })

	origURL := searxngBaseURL
	searxngBaseURL = srv.URL
	t.Cleanup(func() { searxngBaseURL = origURL })

	origInterval := defaultRateLimiter.interval
	defaultRateLimiter.interval = 0
	t.Cleanup(func() { defaultRateLimiter.interval = origInterval })

	sources := []SourceConfig{
		{ID: "reddit_r", Type: "reddit", Enabled: true},
		{ID: "github_x", Type: "github", URL: srv.URL + "/readme", Enabled: true},
		{ID: "web_y", Type: "web_page", URL: srv.URL + "/page", Enabled: true},
		{ID: "search_z", Type: "search", Query: "free llm", Limit: 10, Enabled: true},
		{ID: "unknown", Type: "weird", Enabled: true},
		{ID: "disabled", Type: "github", Enabled: false},
	}
	findings := RunScraper(sources)
	if len(findings) == 0 {
		t.Fatal("expected findings from all enabled sources")
	}
}

func TestRunScraperUnknownType(t *testing.T) {
	sources := []SourceConfig{
		{ID: "weird", Type: "unknown_source", Enabled: true},
	}
	findings := RunScraper(sources)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for unknown type, got %d", len(findings))
	}
}

func TestScrapeRedditMock(t *testing.T) {
	allowAllURL(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":{"children":[{"data":{"title":"Free GPT-4 API alternative","selftext":"I found a free API for LLM inference","permalink":"/r/test/comments/abc123","id":"abc123"}}]}}`)
	}))
	defer srv.Close()

	orig := redditClientFactory
	redditClientFactory = func() *http.Client { return mockClientTo(t, srv) }
	defer func() { redditClientFactory = orig }()

	findings := ScrapeReddit("test", "free api", 25)
	if len(findings) == 0 {
		t.Fatal("expected findings from mocked reddit")
	}
}

func TestScrapeRedditMockNon200(t *testing.T) {
	allowAllURL(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	orig := redditClientFactory
	redditClientFactory = func() *http.Client { return mockClientTo(t, srv) }
	defer func() { redditClientFactory = orig }()

	findings := ScrapeReddit("test", "free api", 25)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings on non-200, got %d", len(findings))
	}
}
