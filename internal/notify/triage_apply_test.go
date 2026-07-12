package notify

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTriageApplyEmpty verifies an empty pending list is a no-op (dry or not).
func TestTriageApplyEmpty(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{}})
	if err := TriageApply(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := TriageApply(dir, false); err != nil {
		t.Fatal(err)
	}
}

// TestTriageApplyDryRunVerdicts exercises the dry-run branches across verdicts
// without ever touching Yandex or mutating the pending file.
func TestTriageApplyDryRunVerdicts(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{
		{Source: "https://a.com", Verdict: "rejected"},
		{Source: "https://b.com", Verdict: "backlog"},
		{Source: "https://c.com", Verdict: "confirmed"},
		{Source: "https://d.com", Verdict: "already_in_use"},
		{Source: "https://e.com", Verdict: ""}, // skipped, no verdict
	}})
	if err := TriageApply(dir, true); err != nil {
		t.Fatal(err)
	}
	// dry-run must not mutate the pending file nor write rejected.json.
	pr, err := LoadPendingReview(filepath.Join(dir, "pending_review.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range pr.Pending {
		if it.RejectedMarked || it.YandexSynced {
			t.Errorf("dry-run mutated item %s", it.Source)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "rejected.json")); err == nil {
		t.Errorf("dry-run must not write rejected.json")
	}
}

// TestTriageApplyRejectedNonDryRun verifies rejected/alias verdicts push to the
// denylist (rejected.json) and mark items, while confirmed/already_in_use take
// no automated action. No Yandex call happens for rejected verdicts.
func TestTriageApplyRejectedNonDryRun(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{
		{Source: "https://a.com", Verdict: "rejected"},
		{Source: "https://b.com", Verdict: "not_confirmed"}, // alias -> rejected
		{Source: "https://c.com", Verdict: "already_in_use"},
	}})
	if err := TriageApply(dir, false); err != nil {
		t.Fatal(err)
	}
	rej, err := loadRejected(filepath.Join(dir, "rejected.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !rej["https://a.com"] || !rej["https://b.com"] {
		t.Errorf("denylist missing entries: %v", rej)
	}
	if rej["https://c.com"] {
		t.Errorf("already_in_use must not enter denylist")
	}
	pr, err := LoadPendingReview(filepath.Join(dir, "pending_review.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !pr.Pending[0].RejectedMarked || !pr.Pending[1].RejectedMarked {
		t.Errorf("rejected items not marked: %+v", pr.Pending)
	}
}

// TestTriageApplyRejectedIdempotent verifies an already-marked item is skipped
// on re-run (safe repeat) while it remains in the denylist that was persisted
// by the previous run.
func TestTriageApplyRejectedIdempotent(t *testing.T) {
	dir := t.TempDir()
	// Simulate the previous run having already written the denylist.
	if err := saveRejected(filepath.Join(dir, "rejected.json"), map[string]bool{"https://a.com": true}); err != nil {
		t.Fatal(err)
	}
	writePending(t, dir, PendingReview{Pending: []PendingItem{
		{Source: "https://a.com", Verdict: "rejected", RejectedMarked: true},
	}})
	if err := TriageApply(dir, false); err != nil {
		t.Fatal(err)
	}
	rej, err := loadRejected(filepath.Join(dir, "rejected.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !rej["https://a.com"] {
		t.Errorf("item should remain in denylist")
	}
}

// TestTriageApplyDryRunBacklogDedup exercises the backlog dry-run path, the
// same-canonical-link dedup, and the already-synced skip. No Yandex call fires.
func TestTriageApplyDryRunBacklogDedup(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{
		{Source: "https://github.com/o/r/issues/1", Verdict: "backlog"},
		{Source: "https://github.com/o/r", Verdict: "backlog"},           // same canonical -> dedup
		{Source: "https://github.com/o/r", Verdict: "backlog", YandexSynced: true}, // skip
	}})
	if err := TriageApply(dir, true); err != nil {
		t.Fatal(err)
	}
}

// TestTriageApplyLoadError verifies a broken pending_review.json surfaces an error.
func TestTriageApplyLoadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "pending_review.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := TriageApply(dir, false); err == nil {
		t.Errorf("expected error from unreadable pending_review.json")
	}
}
// error rather than silently proceeding.
func TestTriageApplyRejectedReadError(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{
		{Source: "https://a.com", Verdict: "rejected"},
	}})
	// Force loadRejected to fail by shadowing it with a directory.
	if err := os.Mkdir(filepath.Join(dir, "rejected.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := TriageApply(dir, false); err == nil {
		t.Errorf("expected error from unreadable rejected.json")
	}
}
