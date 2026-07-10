package keydrop

import (
	"strings"
	"testing"
)

func TestParseMarkdown_FilenameProvider(t *testing.T) {
	md := "sk-abcd1234efgh5678ijkl9012mnop3456\ngsk-zzz999888777666555444333222111\n# notes\nfree tier, 50/day\n"
	provider, keys, notes, err := ParseMarkdown(md, "alistaitsacle")
	if err != nil {
		t.Fatal(err)
	}
	if provider != "alistaitsacle" {
		t.Errorf("provider = %q, want alistaitsacle", provider)
	}
	if len(keys) != 2 {
		t.Errorf("keys = %d, want 2: %v", len(keys), keys)
	}
	if !strings.Contains(notes, "free tier") {
		t.Errorf("notes missing human info: %q", notes)
	}
}

func TestParseMarkdown_HeadingProvider(t *testing.T) {
	md := "# OpenRouter\n\nkey: sk-heading00000000000000000000000000\n"
	provider, keys, _, err := ParseMarkdown(md, "defaultname")
	if err != nil {
		t.Fatal(err)
	}
	if provider != "openrouter" {
		t.Errorf("provider = %q, want openrouter", provider)
	}
	if len(keys) != 1 || !strings.HasPrefix(keys[0], "sk-heading") {
		t.Errorf("keys wrong: %v", keys)
	}
}

func TestParseMarkdown_FrontmatterProvider(t *testing.T) {
	md := "---\nprovider: my-custom-Provider\n---\n\napi_key: `rk_abc123def456ghi789`\n"
	provider, keys, _, err := ParseMarkdown(md, "defaultname")
	if err != nil {
		t.Fatal(err)
	}
	if provider != "my-custom-provider" {
		t.Errorf("provider = %q, want my-custom-provider", provider)
	}
	if len(keys) != 1 || !strings.HasPrefix(keys[0], "rk_") {
		t.Errorf("keys wrong: %v", keys)
	}
}

func TestParseMarkdown_ManyKeysAndNotes(t *testing.T) {
	md := `# pollinations

## keys
- sk-one0000000000000000000000000000
- sk-two0000000000000000000000000000
pk_live_abc123DEF456ghi789

## notes
endpoint: https://api.example.com
quota: 1000/day, expires 2026-08-01
это тестовый провайдер
`
	provider, keys, notes, err := ParseMarkdown(md, "pollinations")
	if err != nil {
		t.Fatal(err)
	}
	if provider != "pollinations" {
		t.Errorf("provider = %q", provider)
	}
	if len(keys) != 3 {
		t.Errorf("keys = %d, want 3: %v", len(keys), keys)
	}
	if !strings.Contains(notes, "endpoint: https://api.example.com") {
		t.Errorf("notes lost endpoint: %q", notes)
	}
	if !strings.Contains(notes, "quota: 1000/day") {
		t.Errorf("notes lost quota: %q", notes)
	}
	if !strings.Contains(notes, "тестовый провайдер") {
		t.Errorf("notes lost cyrillic note: %q", notes)
	}
	// URL shouldn't be captured as a key
	for _, k := range keys {
		if strings.Contains(k, "http") {
			t.Errorf("URL captured as key: %q", k)
		}
	}
}

func TestParseMarkdown_NoKeys(t *testing.T) {
	md := "# nothing\njust a note, no keys here\n"
	_, keys, _, _ := ParseMarkdown(md, "x")
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d: %v", len(keys), keys)
	}
}
