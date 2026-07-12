package scraper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestScrapeSearchMock — основной тест: мок SearXNG JSON API, проверка маппинга.
func TestScrapeSearchMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// запрос должен уходить к JSON API
		if got := r.URL.Query().Get("format"); got != "json" {
			t.Errorf("expected format=json, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := searxngResponse{
			Results: []searxngResult{
				{
					Title:   "Free LLM API credits on Reddit",
					URL:     "https://www.reddit.com/r/LocalLLaMA/comments/abc/free_llm_api/",
					Content: "Found a way to get free API credits for LLM inference. Offer includes OpenRouter tokens.",
				},
				{
					Title:   "GitHub awesome free LLM list",
					URL:     "https://github.com/mnfst/awesome-free-llm-apis",
					Content: "Curated list of free LLM APIs and credits.",
				},
				{
					// без content — должен подхватить snippet
					Title:   "Reddit giveaway of Groq credits",
					URL:     "https://www.reddit.com/r/MachineLearning/comments/def/groq_credits/",
					Snippet: "Giveaway: free Groq API credits, no credit card.",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origBase := searxngBaseURL
	origClient := searchClient
	searxngBaseURL = server.URL
	searchClient = server.Client()
	defer func() {
		searxngBaseURL = origBase
		searchClient = origClient
	}()

	findings := ScrapeSearch("site:reddit.com free LLM API credits", 10)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding from mock searxng")
	}
	if len(findings) != 3 {
		t.Errorf("expected 3 findings, got %d", len(findings))
	}

	redditFound := false
	githubFound := false
	snippetFallback := false
	for _, f := range findings {
		if strings.Contains(f.URL, "reddit.com") {
			redditFound = true
			if !strings.HasPrefix(f.SourceID, "search_reddit_") {
				t.Errorf("expected reddit category in SourceID, got %q", f.SourceID)
			}
		}
		if strings.Contains(f.URL, "github.com") {
			githubFound = true
			if !strings.HasPrefix(f.SourceID, "search_github_") {
				t.Errorf("expected github category in SourceID, got %q", f.SourceID)
			}
		}
		// snippet fallback: groq credits post не имеет content
		if strings.Contains(f.URL, "groq_credits") {
			snippetFallback = strings.Contains(f.Description, "Giveaway") ||
				strings.Contains(f.Description, "Groq")
		}
	}
	if !redditFound {
		t.Error("expected a reddit finding")
	}
	if !githubFound {
		t.Error("expected a github finding")
	}
	if !snippetFallback {
		t.Error("expected snippet fallback for result without content")
	}
}

// TestScrapeSearchEmptyResults — пустой results[] → 0 находок.
func TestScrapeSearchEmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searxngResponse{Results: []searxngResult{}})
	}))
	defer server.Close()

	origBase := searxngBaseURL
	origClient := searchClient
	searxngBaseURL = server.URL
	searchClient = server.Client()
	defer func() {
		searxngBaseURL = origBase
		searchClient = origClient
	}()

	findings := ScrapeSearch("site:reddit.com free LLM API credits", 10)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty results, got %d", len(findings))
	}
}

// TestScrapeSearchInvalidJSON — не-JSON ответ → 0 находок (без паники).
func TestScrapeSearchInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("<html>not json</html>"))
	}))
	defer server.Close()

	origBase := searxngBaseURL
	origClient := searchClient
	searxngBaseURL = server.URL
	searchClient = server.Client()
	defer func() {
		searxngBaseURL = origBase
		searchClient = origClient
	}()

	findings := ScrapeSearch("site:reddit.com free LLM API credits", 10)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for invalid JSON, got %d", len(findings))
	}
}

// TestScrapeSearchServerError — HTTP 500 → 0 находок.
func TestScrapeSearchServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	origBase := searxngBaseURL
	origClient := searchClient
	searxngBaseURL = server.URL
	searchClient = server.Client()
	defer func() {
		searxngBaseURL = origBase
		searchClient = origClient
	}()

	findings := ScrapeSearch("site:reddit.com free LLM API credits", 10)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for server error, got %d", len(findings))
	}
}

// TestScrapeSearchNetworkError — сеть недоступна (закрытый сервер) → 0 находок.
func TestScrapeSearchNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := server.URL
	server.Close()

	origBase := searxngBaseURL
	origClient := searchClient
	searxngBaseURL = deadURL
	searchClient = &http.Client{}
	defer func() {
		searxngBaseURL = origBase
		searchClient = origClient
	}()

	findings := ScrapeSearch("site:reddit.com free LLM API credits", 10)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for network error, got %d", len(findings))
	}
}

// TestScrapeSearchSkipsMalformedURLs — битые/пустые URL пропускаются.
func TestScrapeSearchSkipsMalformedURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := searxngResponse{
			Results: []searxngResult{
				{Title: "valid", URL: "https://www.reddit.com/r/LocalLLaMA/comments/x/free", Content: "free API credits"},
				{Title: "bad", URL: "://not-a-valid-url", Content: "free API credits"},
				{Title: "no-scheme", URL: "reddit.com/r/x", Content: "free API credits"},
				{Title: "empty", URL: "", Content: "free API credits"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origBase := searxngBaseURL
	origClient := searchClient
	searxngBaseURL = server.URL
	searchClient = server.Client()
	defer func() {
		searxngBaseURL = origBase
		searchClient = origClient
	}()

	findings := ScrapeSearch("site:reddit.com free LLM API credits", 10)
	if len(findings) != 1 {
		t.Errorf("expected 1 valid finding (malformed URLs skipped), got %d", len(findings))
	}
}

// TestScrapeSearchEmptyQuery — пустой query не делает сетевой запрос.
func TestScrapeSearchEmptyQuery(t *testing.T) {
	findings := ScrapeSearch("", 10)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty query, got %d", len(findings))
	}
}

func TestDetectResultType(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.reddit.com/r/x", "reddit"},
		{"https://old.reddit.com/r/x", "reddit"},
		{"https://redd.it/abc", "reddit"},
		{"https://github.com/foo/bar", "github"},
		{"https://raw.githubusercontent.com/foo/bar/main/README.md", "github"},
		{"https://news.ycombinator.com/item?id=1", "hackernews"},
		{"https://example.com/page", "web"},
	}
	for _, tt := range tests {
		if got := detectResultType(tt.url); got != tt.want {
			t.Errorf("detectResultType(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"site:reddit.com free LLM API credits", "site_reddit_com_free_llm_api_credits"},
		{"Free GPT-4 API!", "free_gpt_4_api"},
		{"  multiple   spaces  ", "multiple_spaces"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := slugify(tt.in); got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
