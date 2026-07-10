package notify

import (
	"fmt"
	"html"
	"log"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"free-api-hunter/internal/models"
)

// yandexShPath is the single entry point for all Yandex Calendar operations.
// PAT-005: we MUST go through this wrapper (it logs usage and supplies the
// app-password itself) — never curl CalDAV directly.
const yandexShPath = "/root/LabDoctorM/projects/DoctorM_and_Ai/bin/yandex.sh"

// Calendar names resolved by yandex.sh cal_resolve (verified against the
// yandex-suite SKILL.md defaults for the moscowskiymichi@yandex.ru account).
const (
	yandexTasksCal = "Не забыть"   // VTODO task list ("Не забыть")
	yandexEventsCal = "Мои события" // VEVENT grid ("Мои события")
)

// validVerdicts are the verdicts a human reviewer may set on a finding.
// not_confirmed / not_working_rf are aliases of rejected (see applyVerdict).
var validVerdicts = map[string]bool{
	"rejected":       true,
	"backlog":        true,
	"already_in_use": true,
	"confirmed":      true,
	"not_confirmed":  true, // alias -> rejected
	"not_working_rf": true, // alias -> rejected
}

// rejectedVerdicts is the set of verdicts that push a source onto the denylist.
var rejectedVerdicts = map[string]bool{
	"rejected":       true,
	"not_confirmed":  true,
	"not_working_rf": true,
}

// TriageSet records a human verdict on a pending finding WITHOUT touching
// Yandex. The actual execution (denylist / calendar sync) happens later in
// TriageApply (run by the nightly systemd timer).
//
// Exactly one of index (1-based) or source must be supplied to locate the item.
func TriageSet(dataDir string, index int, verdict, source string) error {
	if !validVerdicts[verdict] {
		return fmt.Errorf("invalid verdict %q (allowed: rejected|backlog|already_in_use|confirmed, aliases not_confirmed|not_working_rf)", verdict)
	}
	if index == 0 && source == "" {
		return fmt.Errorf("triage-set: supply either --index (1-based) or --source")
	}

	pendingPath := filepath.Join(dataDir, "pending_review.json")
	pr, err := LoadPendingReview(pendingPath)
	if err != nil {
		return err
	}
	if len(pr.Pending) == 0 {
		return fmt.Errorf("triage-set: no pending items in %s", pendingPath)
	}

	var idx int
	if source != "" {
		found := -1
		for i, it := range pr.Pending {
			if it.Source == source {
				found = i
				break
			}
		}
		if found < 0 {
			return fmt.Errorf("triage-set: no pending item with source %q", source)
		}
		idx = found
	} else {
		if index < 1 || index > len(pr.Pending) {
			return fmt.Errorf("triage-set: index %d out of range (1..%d)", index, len(pr.Pending))
		}
		idx = index - 1
	}

	it := &pr.Pending[idx]
	it.Verdict = verdict
	it.Reviewed = true
	it.ReviewedAt = models.Now()

	if err := writeAtomic(pendingPath, pr); err != nil {
		return err
	}
	log.Printf("[triage] set verdict=%q on #%d source=%s", verdict, idx+1, it.Source)
	return nil
}

// TriageApply reads the recorded verdicts and acts on them:
//   - rejected / not_confirmed / not_working_rf  -> add Source to rejected.json denylist
//   - backlog                                     -> create a Yandex task + grid event (with link)
//   - already_in_use / confirmed                  -> no automated action
//
// Backlog items are de-duplicated via YandexSynced; rejected items via
// RejectedMarked, so re-running is safe.
//
// When dryRun is true, nothing is written and Yandex is never called; the
// function only logs what WOULD be created (title/link/desc) for each backlog
// item.
func TriageApply(dataDir string, dryRun bool) error {
	pendingPath := filepath.Join(dataDir, "pending_review.json")
	pr, err := LoadPendingReview(pendingPath)
	if err != nil {
		return err
	}
	if len(pr.Pending) == 0 {
		log.Printf("[triage] apply: no pending items")
		return nil
	}

	rejPath := rejectedFilePath(dataDir)
	rejected, err := loadRejected(rejPath)
	if err != nil {
		return err
	}

	// 7-day horizon for the backlog task/event (RFC3339 with Z — the wrapper
	// converts VEVENT to Europe/Moscow wall-clock; VTODO DUE keeps Z).
	now := time.Now().UTC()
	startT := now.Add(7 * 24 * time.Hour)
	endT := startT.Add(1 * time.Hour)
	startZ := startT.Format(time.RFC3339)
	endZ := endT.Format(time.RFC3339)
	dueZ := startZ

	changed := false
	createdLinks := map[string][2]string{} // canonical link -> [taskUID, eventUID] (dedup)

	// throttle ensures ≥30s between Yandex calls (rate limit is 1 req / 30s).
	firstCall := true
	callYandex := func(args ...string) (string, error) {
		if !firstCall {
			time.Sleep(31 * time.Second)
		}
		firstCall = false
		return runYandex(args...)
	}

	for i := range pr.Pending {
		it := &pr.Pending[i]
		if it.Verdict == "" {
			continue
		}

		if rejectedVerdicts[it.Verdict] {
			if it.RejectedMarked {
				continue
			}
			if dryRun {
				log.Printf("[triage][dry-run] #%d would add %s to denylist (verdict=%s)", i+1, it.Source, it.Verdict)
				continue
			}
			rejected[it.Source] = true
			it.RejectedMarked = true
			changed = true
			log.Printf("[triage] #%d marked rejected -> denylist: %s", i+1, it.Source)
			continue
		}

		if it.Verdict == "backlog" {
			if it.YandexSynced {
				continue
			}
			link := extractLink(it)

			// De-duplicate backlog items that resolve to the same canonical
			// link (e.g. several badge/playground URLs of one GitHub repo).
			if uids, ok := createdLinks[link]; ok {
				it.YandexTaskUID = uids[0]
				it.YandexEventUID = uids[1]
				it.YandexSynced = true
				changed = true
				log.Printf("[triage] #%d backlog deduped to existing Yandex entry (link=%s)", i+1, link)
				continue
			}

			title := deriveTitle(it)
			desc := cleanText(it.WhyFree) + "\n\nСсылка: " + link

			if dryRun {
				fmt.Printf("[triage][dry-run] backlog #%d WOULD create Yandex task+event:\n", i+1)
				fmt.Printf("  title: %s\n", title)
				fmt.Printf("  link:  %s\n", link)
				fmt.Printf("  desc:  %s\n", desc)
				createdLinks[link] = [2]string{"<dry-run>", "<dry-run>"}
				continue
			}

			taskUID, err := callYandex("cal", "task", yandexTasksCal, "add", title, dueZ, desc)
			if err != nil {
				return fmt.Errorf("create yandex task for %s: %w", it.Source, err)
			}
			it.YandexTaskUID = taskUID

			eventUID, err := callYandex("cal", "add", yandexEventsCal, startZ, endZ, title)
			if err != nil {
				return fmt.Errorf("create yandex event for %s: %w", it.Source, err)
			}
			it.YandexEventUID = eventUID
			it.YandexSynced = true
			changed = true
			createdLinks[link] = [2]string{taskUID, eventUID}
			log.Printf("[triage] #%d backlog synced to Yandex (task=%s event=%s)", i+1, taskUID, eventUID)
			continue
		}

		// already_in_use / confirmed -> no automated action, leave as-is.
		log.Printf("[triage] #%d verdict=%s: no automated action", i+1, it.Verdict)
	}

	if !dryRun {
		if err := saveRejected(rejPath, rejected); err != nil {
			return err
		}
		if changed {
			if err := writeAtomic(pendingPath, pr); err != nil {
				return err
			}
		}
	}
	return nil
}

// runYandex executes the yandex.sh wrapper and returns the UID parsed from its
// stdout (a line of the form "UID: <value>").
func runYandex(args ...string) (string, error) {
	cmd := exec.Command(yandexShPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yandex.sh %v failed: %w\noutput:\n%s", args, err, string(out))
	}
	uid := parseUID(string(out))
	if uid == "" {
		return "", fmt.Errorf("yandex.sh %v did not return a UID\noutput:\n%s", args, string(out))
	}
	return uid, nil
}

var uidRe = regexp.MustCompile(`(?m)^UID:\s*(\S+)`)

// parseUID extracts the "UID: <value>" token from yandex.sh stdout.
func parseUID(out string) string {
	m := uidRe.FindStringSubmatch(out)
	if m == nil {
		return ""
	}
	return m[1]
}

// deriveTitle produces a human-friendly calendar title from a finding:
//   - github.com/o/r (canonical link) -> "o/r"
//   - modelscope.cn                  -> "modelscope API-Inference"
//   - else                           -> host of the link
func deriveTitle(it *PendingItem) string {
	link := extractLink(it)
	if strings.Contains(link, "modelscope.cn") {
		return "modelscope API-Inference"
	}
	if m := githubRe.FindStringSubmatch(link); m != nil {
		return m[1] + "/" + m[2]
	}
	if u, err := url.Parse(link); err == nil && u.Host != "" {
		return u.Host
	}
	return link
}

// canonicalRepo finds a GitHub repository slug anywhere in the finding's
// source or why_free and returns the canonical https://github.com/<o>/<r> URL.
// This collapses badge / github.io / star-history / raw URLs that all point at
// the same repository into ONE canonical link (so the backlog groups cleanly).
func canonicalRepo(it *PendingItem) string {
	text := it.Source + " " + it.WhyFree
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`github\.com/([\w.-]+)/([\w.-]+)`),
		regexp.MustCompile(`([\w.-]+)\.github\.io/([\w.-]+)`),
		regexp.MustCompile(`repos=([\w.-]+)/([\w.-]+)`),
		regexp.MustCompile(`raw\.githubusercontent\.com/([\w.-]+)/([\w.-]+)/`),
	}
	for _, re := range patterns {
		if m := re.FindStringSubmatch(text); m != nil {
			owner, repo := m[1], m[2]
			repo = strings.TrimSuffix(repo, ".git")
			return "https://github.com/" + owner + "/" + repo
		}
	}
	return ""
}

// extractLink returns the most useful URL for a finding:
//   - if a GitHub repo slug is present anywhere (source or why_free) ->
//     the canonical https://github.com/<owner>/<repo> URL;
//   - else if Source is itself a "useful" URL (not a badge / star-history
//     image) -> Source verbatim;
//   - else parse WhyFree for a github.com/<owner>/<repo> link and return it;
//   - else Source.
func extractLink(it *PendingItem) string {
	if repo := canonicalRepo(it); repo != "" {
		return repo
	}
	if !isUselessSource(it.Source) {
		return it.Source
	}
	if r := extractGitHubRepo(it.WhyFree); r != "" {
		return r
	}
	return it.Source
}

// isUselessSource reports whether a source URL carries no real navigation value
// (shields.io badges, star-history charts).
func isUselessSource(src string) bool {
	for _, d := range []string{"img.shields.io", "star-history.com", "api.star-history.com"} {
		if strings.Contains(src, d) {
			return true
		}
	}
	return false
}

// extractGitHubRepo finds the first github.com/<owner>/<repo> link inside text
// (markdown [..](url), html href="url", or a raw URL) and returns the canonical
// repo URL with trailing /stargazers,/commits,/blob,/tree segments stripped.
func extractGitHubRepo(text string) string {
	candidates := []string{}
	for _, m := range mdURLRe.FindAllStringSubmatch(text, -1) {
		candidates = append(candidates, m[1])
	}
	for _, m := range hrefURLRe.FindAllStringSubmatch(text, -1) {
		candidates = append(candidates, m[1])
	}
	for _, m := range rawGhRe.FindAllStringSubmatch(text, -1) {
		candidates = append(candidates, m[0])
	}
	for _, c := range candidates {
		if repo := canonicalGitHubRepo(c); repo != "" {
			return repo
		}
	}
	return ""
}

// canonicalGitHubRepo returns "https://github.com/<owner>/<repo>" for a URL that
// contains a github repo path, stripping known suffixes and .git.
func canonicalGitHubRepo(u string) string {
	m := ghRepoRe.FindStringSubmatch(u)
	if m == nil {
		return ""
	}
	owner, repo := m[1], m[2]
	repo = strings.TrimSuffix(repo, ".git")
	return "https://github.com/" + owner + "/" + repo
}

// cleanText normalizes WhyFree into plain, calendar-friendly text:
//   - [text](url) markdown  -> "text (url)"
//   - HTML entities decoded
//   - HTML tags stripped
//   - runs of whitespace/newlines collapsed to a single space
func cleanText(s string) string {
	s = mdLinkRe.ReplaceAllString(s, "$1 ($2)")
	s = html.UnescapeString(s)
	s = tagRe.ReplaceAllString(s, "")
	s = wsRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

var (
	githubRe  = regexp.MustCompile(`github\.com/([^/\s)"'>]+)/([^/\s)"'>]+)`)
	mdURLRe   = regexp.MustCompile(`\]\((\s*https?://[^)\s]+)\)`)
	hrefURLRe = regexp.MustCompile(`href=["']([^"']+)["']`)
	rawGhRe   = regexp.MustCompile(`https?://github\.com/[^\s)<>"']+`)
	ghRepoRe  = regexp.MustCompile(`github\.com/([^/\s)"'>?#.]+)/([^/\s)"'?#.]+)`)
	mdLinkRe  = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	tagRe     = regexp.MustCompile(`(?s)<[^>]+>`)
	wsRe      = regexp.MustCompile(`[ \t\r\n\f\v]+`)
)
