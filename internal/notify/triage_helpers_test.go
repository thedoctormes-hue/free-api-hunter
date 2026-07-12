package notify

import "testing"

// TestDeriveTitle exercises the calendar-title heuristic:
//   - modelscope.cn host -> fixed label
//   - github.com/<o>/<r> -> "<o>/<r>"
//   - any other host -> URL host
//   - unparseable link -> link verbatim
func TestDeriveTitle(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"modelscope", "https://modelscope.cn/models/foo", "modelscope API-Inference"},
		{"github", "https://github.com/owner/repo", "owner/repo"},
		{"host", "https://example.com/page", "example.com"},
		{"nohost", "foo", "foo"},
	}
	for _, c := range cases {
		it := &PendingItem{Source: c.src}
		if got := deriveTitle(it); got != c.want {
			t.Errorf("%s: deriveTitle=%q want %q", c.name, got, c.want)
		}
	}
}

// TestCanonicalRepo collapses various GitHub URL shapes into one canonical repo URL.
func TestCanonicalRepo(t *testing.T) {
	cases := []struct {
		name string
		src  string
		why  string
		want string
	}{
		{"gh-source", "https://github.com/a/b", "", "https://github.com/a/b"},
		{"gh-io-why", "", "see owner.github.io/repo docs", "https://github.com/owner/repo"},
		{"repos-param", "", "trending?repos=x/y", "https://github.com/x/y"},
		{"raw", "", "file at raw.githubusercontent.com/p/q/master/a.go", "https://github.com/p/q"},
		{"none", "https://example.com/x", "no repo here", ""},
	}
	for _, c := range cases {
		it := &PendingItem{Source: c.src, WhyFree: c.why}
		if got := canonicalRepo(it); got != c.want {
			t.Errorf("%s: canonicalRepo=%q want %q", c.name, got, c.want)
		}
	}
}

// TestExtractLink checks the URL-selection priority:
//  1. canonical github repo (source or why_free)
//  2. otherwise a non-useless source verbatim
//  3. otherwise a github link parsed from why_free
//  4. otherwise the (possibly useless) source
func TestExtractLink(t *testing.T) {
	cases := []struct {
		name string
		src  string
		why  string
		want string
	}{
		{"gh", "https://github.com/a/b", "", "https://github.com/a/b"},
		{"source-ok", "https://example.com/x", "", "https://example.com/x"},
		{"useless-source-gh-in-why", "https://img.shields.io/badge/foo", "go to https://github.com/c/d for more", "https://github.com/c/d"},
		{"useless-source-no-gh", "https://img.shields.io/badge/foo", "nothing useful", "https://img.shields.io/badge/foo"},
	}
	for _, c := range cases {
		it := &PendingItem{Source: c.src, WhyFree: c.why}
		if got := extractLink(it); got != c.want {
			t.Errorf("%s: extractLink=%q want %q", c.name, got, c.want)
		}
	}
}

func TestIsUselessSource(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"https://img.shields.io/badge/x", true},
		{"https://star-history.com/#a/b", true},
		{"https://api.star-history.com/svg", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isUselessSource(c.src); got != c.want {
			t.Errorf("isUselessSource(%q)=%v want %v", c.src, got, c.want)
		}
	}
}

// TestExtractGitHubRepo parses a repo link out of markdown, html href, or raw text.
func TestExtractGitHubRepo(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"md", "[repo](https://github.com/a/b)", "https://github.com/a/b"},
		{"href", `<a href="https://github.com/c/d">x</a>`, "https://github.com/c/d"},
		{"raw", "see https://github.com/e/f here", "https://github.com/e/f"},
		{"none", "no link", ""},
	}
	for _, c := range cases {
		if got := extractGitHubRepo(c.text); got != c.want {
			t.Errorf("%s: extractGitHubRepo=%q want %q", c.name, got, c.want)
		}
	}
}

func TestCanonicalGitHubRepo(t *testing.T) {
	cases := []struct {
		name string
		u    string
		want string
	}{
		{"basic", "https://github.com/a/b", "https://github.com/a/b"},
		{"suffix", "https://github.com/a/b/stargazers", "https://github.com/a/b"},
		{"dotgit", "https://github.com/a/b.git", "https://github.com/a/b"},
		{"nongh", "https://example.com/a/b", ""},
	}
	for _, c := range cases {
		if got := canonicalGitHubRepo(c.u); got != c.want {
			t.Errorf("%s: canonicalGitHubRepo(%q)=%q want %q", c.name, c.u, got, c.want)
		}
	}
}

// TestCleanText normalizes markdown links, HTML, and whitespace.
func TestCleanText(t *testing.T) {
	in := "[click](https://example.com/x) &amp; <b>bold</b>\n\ttab   spaced"
	want := "click (https://example.com/x) & bold tab spaced"
	if got := cleanText(in); got != want {
		t.Errorf("cleanText=%q want %q", got, want)
	}
}

// TestParseUID extracts the "UID: <value>" token from yandex.sh stdout.
func TestParseUID(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want string
	}{
		{"basic", "UID: abc123\n", "abc123"},
		{"none", "no uid here", ""},
		{"multiline", "resp ok\nUID: xyz_789\nend", "xyz_789"},
	}
	for _, c := range cases {
		if got := parseUID(c.out); got != c.want {
			t.Errorf("%s: parseUID(%q)=%q want %q", c.name, c.out, got, c.want)
		}
	}
}
