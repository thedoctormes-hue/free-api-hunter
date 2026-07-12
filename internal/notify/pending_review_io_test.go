package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadPendingReviewMissing verifies a missing file is treated as empty.
func TestLoadPendingReviewMissing(t *testing.T) {
	pr, err := LoadPendingReview(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pr.Pending) != 0 {
		t.Errorf("expected empty pending, got %d", len(pr.Pending))
	}
}

// TestLoadPendingReviewInvalidJSON verifies a malformed file yields a parse error.
func TestLoadPendingReviewInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pending_review.json")
	if err := os.WriteFile(p, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPendingReview(p); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got %v", err)
	}
}

// TestLoadPendingReviewReadError verifies a non-NotExist read failure propagates.
func TestLoadPendingReviewReadError(t *testing.T) {
	// A directory path is not a regular file -> ReadFile error (not IsNotExist).
	if _, err := LoadPendingReview(t.TempDir()); err == nil {
		t.Errorf("expected read error for directory path")
	}
}

// TestLoadPendingReviewNilPending verifies explicit null stays an empty slice.
func TestLoadPendingReviewNilPending(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pending_review.json")
	if err := os.WriteFile(p, []byte(`{"pending":null}`), 0644); err != nil {
		t.Fatal(err)
	}
	pr, err := LoadPendingReview(p)
	if err != nil {
		t.Fatal(err)
	}
	if pr.Pending == nil {
		t.Errorf("expected non-nil slice")
	}
	if len(pr.Pending) != 0 {
		t.Errorf("expected empty pending, got %d", len(pr.Pending))
	}
}

// TestLoadRejectedMissing verifies a missing denylist is an empty set.
func TestLoadRejectedMissing(t *testing.T) {
	set, err := loadRejected(filepath.Join(t.TempDir(), "rejected.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set) != 0 {
		t.Errorf("expected empty set, got %d", len(set))
	}
}

// TestLoadRejectedValid verifies entries load and empty strings are skipped.
func TestLoadRejectedValid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rejected.json")
	if err := os.WriteFile(p, []byte(`["https://a.com","https://b.com",""]`), 0644); err != nil {
		t.Fatal(err)
	}
	set, err := loadRejected(p)
	if err != nil {
		t.Fatal(err)
	}
	if !set["https://a.com"] || !set["https://b.com"] {
		t.Errorf("missing entries: %v", set)
	}
	if set[""] {
		t.Errorf("empty string should be skipped")
	}
}

// TestLoadRejectedInvalidJSON verifies a malformed denylist yields a parse error.
func TestLoadRejectedInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rejected.json")
	if err := os.WriteFile(p, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRejected(p); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got %v", err)
	}
}

// TestLoadRejectedReadError verifies a non-NotExist read failure propagates.
func TestLoadRejectedReadError(t *testing.T) {
	if _, err := loadRejected(t.TempDir()); err == nil {
		t.Errorf("expected read error for directory path")
	}
}

// TestSaveRejectedRoundTrip verifies the denylist is written and read back
// deterministically (sorted, diff-friendly).
func TestSaveRejectedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rejected.json")
	set := map[string]bool{"https://b.com": true, "https://a.com": true}
	if err := saveRejected(p, set); err != nil {
		t.Fatal(err)
	}
	got, err := loadRejected(p)
	if err != nil {
		t.Fatal(err)
	}
	if !got["https://a.com"] || !got["https://b.com"] {
		t.Errorf("round-trip lost entries: %v", got)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "https://a.com") || !strings.Contains(string(data), "https://b.com") {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

// TestSaveRejectedEmpty verifies an empty set serializes to "[]".
func TestSaveRejectedEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rejected.json")
	if err := saveRejected(p, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "[]" {
		t.Errorf("expected empty array, got %q", string(data))
	}
}

// TestWriteAtomicRoundTrip verifies the atomic writer round-trips content.
func TestWriteAtomicMkdirError(t *testing.T) {
	// Make a parent component a regular file so MkdirAll cannot create the dir.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(blocker, "sub", "pending_review.json")
	if err := writeAtomic(outPath, PendingReview{Pending: []PendingItem{{Source: "s"}}}); err == nil {
		t.Errorf("expected writeAtomic error when a parent path component is a file")
	}
}

func TestSaveRejectedMkdirError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(blocker, "rejected.json")
	if err := saveRejected(p, map[string]bool{"https://a.com": true}); err == nil {
		t.Errorf("expected saveRejected error when a parent path component is a file")
	}
}

// TestWriteAtomicRoundTrip verifies the atomic writer round-trips content.
func TestWriteAtomicRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pending_review.json")
	pr := PendingReview{Pending: []PendingItem{{Source: "https://x.com", WhyFree: "hi"}}}
	if err := writeAtomic(p, pr); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPendingReview(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pending) != 1 || got.Pending[0].Source != "https://x.com" {
		t.Errorf("round-trip failed: %+v", got)
	}
}
