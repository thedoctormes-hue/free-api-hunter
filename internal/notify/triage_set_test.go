package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writePending serializes a PendingReview to pending_review.json in dir.
func writePending(t *testing.T, dir string, pr PendingReview) {
	t.Helper()
	b, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pending_review.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
}

// TestTriageSetInvalidVerdict rejects unknown verdicts.
func TestTriageSetInvalidVerdict(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{{Source: "s"}}})
	if err := TriageSet(dir, 1, "bogus", ""); err == nil {
		t.Errorf("expected error for invalid verdict")
	}
}

// TestTriageSetNoLocator requires either --index or --source.
func TestTriageSetNoLocator(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{{Source: "s"}}})
	if err := TriageSet(dir, 0, "rejected", ""); err == nil {
		t.Errorf("expected error when no index/source supplied")
	}
}

// TestTriageSetEmptyPending errors when there is nothing to triage.
func TestTriageSetEmptyPending(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{}})
	if err := TriageSet(dir, 1, "rejected", ""); err == nil {
		t.Errorf("expected error for empty pending")
	}
}

// TestTriageSetSourceNotFound errors when --source is unknown.
func TestTriageSetSourceNotFound(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{{Source: "s"}}})
	if err := TriageSet(dir, 0, "rejected", "other"); err == nil {
		t.Errorf("expected error for missing source")
	}
}

// TestTriageSetIndexOutOfRange rejects 1-based indices outside the slice.
func TestTriageSetIndexOutOfRange(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{{Source: "s"}}})
	if err := TriageSet(dir, 5, "rejected", ""); err == nil {
		t.Errorf("expected out-of-range error")
	}
	if err := TriageSet(dir, -1, "rejected", ""); err == nil {
		t.Errorf("expected out-of-range error for negative index")
	}
}

// TestTriageSetByIndex records a verdict by 1-based index.
func TestTriageSetByIndex(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{
		{Source: "a"},
		{Source: "b"},
	}})
	if err := TriageSet(dir, 2, "confirmed", ""); err != nil {
		t.Fatal(err)
	}
	pr, err := LoadPendingReview(filepath.Join(dir, "pending_review.json"))
	if err != nil {
		t.Fatal(err)
	}
	it := pr.Pending[1]
	if it.Verdict != "confirmed" || !it.Reviewed || it.ReviewedAt == "" {
		t.Errorf("by-index triage not recorded: %+v", it)
	}
	// the other entry must be untouched
	if pr.Pending[0].Reviewed {
		t.Errorf("unrelated entry was mutated")
	}
}

// TestTriageSetLoadError verifies a broken pending_review.json surfaces an error.
func TestTriageSetLoadError(t *testing.T) {
	dir := t.TempDir()
	// pending_review.json as a directory -> LoadPendingReview fails.
	if err := os.Mkdir(filepath.Join(dir, "pending_review.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := TriageSet(dir, 1, "rejected", ""); err == nil {
		t.Errorf("expected error from unreadable pending_review.json")
	}
}
func TestTriageSetBySource(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{
		{Source: "a"},
		{Source: "b"},
	}})
	if err := TriageSet(dir, 0, "backlog", "b"); err != nil {
		t.Fatal(err)
	}
	pr, err := LoadPendingReview(filepath.Join(dir, "pending_review.json"))
	if err != nil {
		t.Fatal(err)
	}
	it := pr.Pending[1]
	if it.Verdict != "backlog" || !it.Reviewed || it.ReviewedAt == "" {
		t.Errorf("by-source triage not recorded: %+v", it)
	}
}
