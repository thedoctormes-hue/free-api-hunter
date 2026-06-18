package scraper

import (
	"testing"
	"time"
)

func TestIsChangelogLine(t *testing.T) {
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

func TestIsProviderEntry(t *testing.T) {
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

func TestGetRandomUserAgent(t *testing.T) {
	// Проверяем что функция возвращает непустой результат
	for i := 0; i < 10; i++ {
		ua := getRandomUserAgent()
		if ua == "" {
			t.Error("getRandomUserAgent() returned empty string")
		}
	}
}

func TestWaitForRedditRateLimit(t *testing.T) {
	// Проверяем что функция не паникует
	lastRedditRequest = time.Now()

	// Первый вызов — должен подождать ~2s
	waitForRedditRateLimit()

	// Второй вызов сразу — тоже должен подождать
	waitForRedditRateLimit()
}
