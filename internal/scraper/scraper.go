package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/securego"
)

var logger = log.New(log.Writer(), "[scraper] ", log.LstdFlags)

// HTTPClient — базовый HTTP клиент
var HTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

// userAgents — список для ротации чтобы избегать простых блокировок
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36",
	"FreeAPIHunter/0.1 (LabDoctorM research project; +https://github.com/LabDoctorM/free-api-hunter)",
	"curl/7.88.0",
	"Wget/1.21.4",
}

// getRandomUserAgent возвращает случайный User-Agent из списка
func getRandomUserAgent() string {
	if len(userAgents) == 0 {
		return "FreeAPIHunter/0.1"
	}
	return userAgents[time.Now().UnixNano()%int64(len(userAgents))]
}

// rateLimitReddit — простая задержка для Reddit запросов чтобы избежать 429/403
var lastRedditRequest time.Time

const redditMinInterval = 2 * time.Second

// waitForRedditRateLimit ждёт если нужно соблюсти интервал между запросами к Reddit
func waitForRedditRateLimit() {
	elapsed := time.Since(lastRedditRequest)
	if elapsed < redditMinInterval {
		time.Sleep(redditMinInterval - elapsed)
	}
	lastRedditRequest = time.Now()
}

// ─── Domain Rate Limiter ───

type domainRateLimiter struct {
	mu       sync.Mutex
	lastReq  map[string]time.Time
	interval time.Duration
}

var defaultRateLimiter = &domainRateLimiter{
	lastReq:  make(map[string]time.Time),
	interval: 1 * time.Second, // max 1 request per second per domain
}

// WaitForRate blocks until the minimum interval for the given domain has passed.
// If customClient is non-nil and implements the http.RoundTripper protocol,
// it is used instead of HTTPClient for the actual fetch.
func (r *domainRateLimiter) WaitForRate(domain string) {
	r.mu.Lock()
	last, ok := r.lastReq[domain]
	r.mu.Unlock()

	if ok {
		elapsed := time.Since(last)
		if elapsed < r.interval {
			time.Sleep(r.interval - elapsed)
		}
	}

	r.mu.Lock()
	r.lastReq[domain] = time.Now()
	r.mu.Unlock()
}

// extractDomain parses a URL and returns its host domain.
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := u.Hostname()
	if host == "" {
		return rawURL
	}
	// Stripwww. prefix for consistency
	if strings.HasPrefix(host, "www.") {
		host = host[4:]
	}
	return host
}

// SetHTTPClient allows tests to inject a custom HTTP client.
// Pass nil to reset to the default.
func SetHTTPClient(client *http.Client) {
	mu.Lock()
	defer mu.Unlock()
	if client == nil {
		HTTPClient = &http.Client{Timeout: 15 * time.Second}
	} else {
		HTTPClient = client
	}
}

var mu sync.Mutex // protects HTTPClient replacements

// extractHost returns the host:port from a URL string (used for rate limiting).
func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	_, port, _ := net.SplitHostPort(u.Host)
	_ = port
	return u.Hostname()
}

// CreateRedditClient создаёт HTTP клиент оптимизированный для Reddit
// с ротацией User-Agent и базовыми заголовками
func CreateRedditClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			// Используем системный прокси если установлен (HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
			Proxy: http.ProxyFromEnvironment,
		},
	}
}

// FetchURL — загрузить URL, вернуть тело или ошибку
func FetchURL(rawURL string) (string, error) {
	_, err := securego.IsValidOutboundURL(rawURL)
	if err != nil {
		return "", fmt.Errorf("URL rejected: %w", err)
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "FreeAPIHunter/0.1 (LabDoctorM research project)")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ScrapeReddit — сканировать Reddit через JSON API с обходом блокировок
func ScrapeReddit(subreddit, query string, limit int) []models.Finding {
	waitForRedditRateLimit()

	encodedQuery := url.QueryEscape(query)
	rawURL := fmt.Sprintf("https://www.reddit.com/r/%s/search.json?q=%s&sort=new&limit=%d", subreddit, encodedQuery, limit)

	// Создаём специализированный клиент для Reddit
	client := CreateRedditClient()

	// Формируем запрос с ротируемым User-Agent
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("ScrapeReddit %s failed: %v", subreddit, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Printf("ScrapeReddit %s: HTTP %d", subreddit, resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	raw := string(body)

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		logger.Printf("ScrapeReddit %s: invalid JSON", subreddit)
		return nil
	}

	children := extractRedditChildren(data)
	var findings []models.Finding

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
		keywords := []string{"api", "key", "free", "credit", "tier", "llm", "model", "gpt", "gemini", "claude"}
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

		fullURL := "https://reddit.com" + permalink
		findings = append(findings, models.Finding{
			SourceID:     "reddit_" + postID,
			Title:        title,
			URL:          fullURL,
			Description:  truncate(selftext, 500),
			RawText:      title + "\n\n" + selftext,
			DiscoveredAt: models.Now(),
		})
	}

	logger.Printf("ScrapeReddit %s: %d findings", subreddit, len(findings))
	return findings
}

func extractRedditChildren(data map[string]interface{}) []map[string]interface{} {
	d, ok := data["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	children, ok := d["children"].([]interface{})
	if !ok {
		return nil
	}

	var result []map[string]interface{}
	for _, c := range children {
		if m, ok := c.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// isChangelogLine — определяет является ли строка changelog-мусором
func isChangelogLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	changelogPatterns := []string{
		"## changelog", "### changelog", "## roadmap", "### roadmap",
		"## contributing", "### contributing", "## license", "### license",
		"## acknowledgments", "### acknowledgments",
		"released on", "version ", "v0.", "v1.", "v2.",
		"breaking change", "deprecated", "migration guide",
		"all notable changes", "see the changelog",
	}
	for _, p := range changelogPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// isProviderEntry — определяет содержит ли строка информацию о провайдере
func isProviderEntry(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	providerKeywords := []string{
		"api", "endpoint", "free tier", "free plan", "free model",
		"rate limit", "rpm", "tpm", "rpd", "req/day", "requests per",
		"no credit", "without card", "бесплатн", "без карты",
		"inference", "llm", "gpt", "claude", "gemini", "llama",
		"mistral", "qwen", "deepseek", "openrouter", "groq",
		"cerebras", "cohere", "perplexity", "together",
	}
	for _, kw := range providerKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ScrapeGitHubREADME — сканировать GitHub README в поисках упоминаний бесплатных API
func ScrapeGitHubREADME(rawURL, sourceID string) []models.Finding {
	raw, err := FetchURL(rawURL)
	if err != nil {
		logger.Printf("ScrapeGitHubREADME failed for %s: %v", rawURL, err)
		return nil
	}

	lines := strings.Split(raw, "\n")
	var findings []models.Finding
	currentSection := ""
	linkPattern := regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)]+)\)`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Пропускаем changelog-мусор
		if isChangelogLine(trimmed) {
			continue
		}

		// Отслеживаем секции
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			currentSection = strings.TrimLeft(trimmed, "# ")
			continue
		}

		lower := strings.ToLower(trimmed)

		// Ищем строки с упоминанием бесплатных API
		freeKeywords := []string{"free", "api key", "no credit", "without card", "free tier",
			"бесплатн", "без карты", "no payment", "without payment"}
		hasFree := false
		for _, kw := range freeKeywords {
			if strings.Contains(lower, kw) {
				hasFree = true
				break
			}
		}
		if !hasFree {
			continue
		}

		// Дополнительная проверка: это реально про API, а не просто упоминание
		if !isProviderEntry(trimmed) {
			continue
		}

		// Извлекаем ссылки из строки
		links := linkPattern.FindAllStringSubmatch(trimmed, -1)
		var url string
		var title string
		if len(links) > 0 {
			title = links[0][1]
			url = links[0][2]
		} else {
			title = currentSection
			url = rawURL
		}

		findings = append(findings, models.Finding{
			SourceID:     sourceID,
			Title:        title,
			URL:          url,
			Description:  trimmed,
			RawText:      trimmed,
			DiscoveredAt: models.Now(),
		})
	}

	logger.Printf("ScrapeGitHubREADME: %d findings from %s", len(findings), rawURL)
	return findings
}

// ScrapeProviderPage — загрузить страницу провайдера для верификации
func ScrapeProviderPage(rawURL string) (string, error) {
	_, err := securego.IsValidOutboundURL(rawURL)
	if err != nil {
		return "", fmt.Errorf("URL rejected: %w", err)
	}
	return FetchURL(rawURL)
}

// ScrapeWebPage — сканировать веб-страницу в поисках упоминаний бесплатных API
func ScrapeWebPage(rawURL, sourceID string) []models.Finding {
	// Rate limit: max 1 req/sec per domain
	host := extractHost(rawURL)
	if host != "" {
		defaultRateLimiter.WaitForRate(host)
	}

	raw, err := FetchURL(rawURL)
	if err != nil {
		logger.Printf("ScrapeWebPage failed for %s: %v", rawURL, err)
		return nil
	}

	var findings []models.Finding
	text := strings.ToLower(raw)

	// Ищем секции с упоминанием бесплатных API
	freeKeywords := []string{"free tier", "free plan", "free credit", "no credit card",
		"without credit card", "free api", "бесплатн", "без карты"}

	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lowerLine := strings.ToLower(line)
		for _, kw := range freeKeywords {
			if strings.Contains(lowerLine, kw) {
				// Берём контекст: текущая строка + 2 до и 2 после
				start := i - 2
				if start < 0 {
					start = 0
				}
				end := i + 3
				if end > len(lines) {
					end = len(lines)
				}
				context := strings.Join(lines[start:end], "\n")

				findings = append(findings, models.Finding{
					SourceID:     sourceID,
					Title:        "Web page mention",
					URL:          rawURL,
					Description:  strings.TrimSpace(context),
					RawText:      context,
					DiscoveredAt: models.Now(),
				})
				break
			}
		}
	}

	_ = text
	logger.Printf("ScrapeWebPage: %d findings from %s", len(findings), rawURL)
	return findings
}

// ScrapeHackerNews — сканировать HN на упоминания бесплатных API
func ScrapeHackerNews(query string, limit int) []models.Finding {
	encodedQuery := url.QueryEscape(query)
	rawURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search?query=%s&tags=story&hitsPerPage=%d", encodedQuery, limit)

	raw, err := FetchURL(rawURL)
	if err != nil {
		logger.Printf("ScrapeHackerNews failed: %v", err)
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		logger.Printf("ScrapeHackerNews: invalid JSON")
		return nil
	}

	hits, ok := data["hits"].([]interface{})
	if !ok {
		return nil
	}

	var findings []models.Finding
	for _, h := range hits {
		post, ok := h.(map[string]interface{})
		if !ok {
			continue
		}

		title, _ := post["title"].(string)
		postURL, _ := post["url"].(string)
		postID, _ := post["objectID"].(string)
		points, _ := post["points"].(int)
		createdAt, _ := post["created_at"].(string)

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

		// HN API не возвращает тело поста, используем title как description
		findings = append(findings, models.Finding{
			SourceID:     "hn_" + postID,
			Title:        title,
			URL:          postURL,
			Description:  fmt.Sprintf("%d points, %s", points, createdAt),
			RawText:      title,
			DiscoveredAt: models.Now(),
		})
	}

	logger.Printf("ScrapeHackerNews: %d findings", len(findings))
	return findings
}

// RunScraper — запустить все включённые источники
func RunScraper(sources []SourceConfig) []models.Finding {
	var allFindings []models.Finding

	for _, src := range sources {
		if !src.Enabled {
			continue
		}

		switch src.Type {
		case "reddit":
			subreddit := strings.TrimPrefix(src.ID, "reddit_")
			findings := ScrapeReddit(subreddit, "free API", 25)
			allFindings = append(allFindings, findings...)
		case "github":
			findings := ScrapeGitHubREADME(src.URL, src.ID)
			allFindings = append(allFindings, findings...)
		case "github_raw":
			findings := ScrapeGitHubREADME(src.URL, src.ID)
			allFindings = append(allFindings, findings...)
		case "web_page":
			findings := ScrapeWebPage(src.URL, src.ID)
			allFindings = append(allFindings, findings...)
		case "hackernews":
			findings := ScrapeHackerNews(src.URL, 25)
			allFindings = append(allFindings, findings...)
		default:
			logger.Printf("Unknown source type: %s", src.Type)
		}
	}

	logger.Printf("RunScraper: total %d raw findings", len(allFindings))
	return allFindings
}

// SourceConfig — конфигурация источника
type SourceConfig struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
