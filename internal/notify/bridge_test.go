package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/storage"
)

func strptr(s string) *string { return &s }

// TestBridgePreservesReviewed verifies:
//   - existing reviewed:true entries are kept (not overwritten, not duplicated)
//   - new findings are appended with reviewed:false
//   - duplicate findings and those without a URL are skipped
//   - re-running the bridge does not create duplicates
func TestBridgePreservesReviewed(t *testing.T) {
	tmpDir := t.TempDir()
	tmpOut := filepath.Join(tmpDir, "pending_review.json")

	if err := storage.InitDB(tmpDir); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer storage.CloseDB()

	// Seed findings into the DB.
	findings := []*models.Finding{
		{
			SourceID:     "src_a",
			Title:        "Free model A",
			URL:          "https://example.com/a",
			Description:  "free tier mentioned",
			DiscoveredAt: "2026-07-10T00:00:00Z",
			ProviderName: nil, // unknown provider
		},
		{
			SourceID:     "src_b",
			Title:        "Free model B",
			URL:          "https://example.com/b",
			Description:  "", // empty -> "unknown"
			DiscoveredAt: "2026-07-10T01:00:00Z",
		},
		{
			SourceID:     "src_c",
			Title:        "Dup model C",
			URL:          "https://example.com/c",
			Description:  "duplicate",
			DiscoveredAt: "2026-07-10T02:00:00Z",
			IsDuplicate:  true,
		},
	}
	if err := storage.SaveFindings(findings, ""); err != nil {
		t.Fatalf("save findings: %v", err)
	}

	// Pre-existing pending_review.json with a reviewed:true entry that matches
	// finding A's URL. The bridge must PRESERVE it and NOT re-add A.
	existing := PendingReview{Pending: []PendingItem{
		{
			Provider: "already-reviewed-provider",
			Source:   "https://example.com/a",
			WhyFree:  "previously reviewed note",
			FoundAt:  "2026-07-09T00:00:00Z",
			Reviewed: true,
		},
	}}
	existingBytes, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(tmpOut, existingBytes, 0644); err != nil {
		t.Fatalf("seed pending: %v", err)
	}

	// First bridge run: expect 1 new (B); A skipped (already present, reviewed),
	// C skipped (duplicate).
	added, err := BridgePendingReview(tmpOut)
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}
	if added != 1 {
		t.Fatalf("expected 1 added, got %d", added)
	}

	got, err := LoadPendingReview(tmpOut)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(got.Pending) != 2 {
		t.Fatalf("expected 2 pending entries, got %d: %+v", len(got.Pending), got.Pending)
	}

	// Entry 0 must be the preserved reviewed:true record (verbatim).
	if !got.Pending[0].Reviewed {
		t.Errorf("existing reviewed entry was not preserved as reviewed:true")
	}
	if got.Pending[0].Source != "https://example.com/a" || got.Pending[0].Provider != "already-reviewed-provider" {
		t.Errorf("existing entry content changed: %+v", got.Pending[0])
	}

	// Entry 1 must be the newly appended B with reviewed:false.
	b := got.Pending[1]
	if b.Source != "https://example.com/b" {
		t.Errorf("expected new entry for B, got %+v", b)
	}
	if b.Reviewed {
		t.Errorf("new entry must be reviewed:false")
	}
	if b.WhyFree != "unknown" {
		t.Errorf("empty description should map to 'unknown', got %q", b.WhyFree)
	}
	if b.Provider != "" {
		t.Errorf("provider should be empty (unknown), got %q", b.Provider)
	}

	// Second bridge run: nothing new should be added, no duplicates.
	added2, err := BridgePendingReview(tmpOut)
	if err != nil {
		t.Fatalf("bridge2: %v", err)
	}
	if added2 != 0 {
		t.Fatalf("expected 0 added on re-run, got %d", added2)
	}
	got2, _ := LoadPendingReview(tmpOut)
	if len(got2.Pending) != 2 {
		t.Fatalf("re-run duplicated entries: got %d", len(got2.Pending))
	}
}

// TestAppendNewFindingsPure checks the pure function in isolation.
func TestAppendNewFindingsPure(t *testing.T) {
	existing := PendingReview{Pending: []PendingItem{
		{Source: "https://example.com/x", Reviewed: true},
	}}
	findings := []*models.Finding{
		{URL: "https://example.com/x", ProviderName: strptr("p1"), Description: "d"}, // dup by URL
		{URL: "https://example.com/y", ProviderName: strptr("p2"), Description: "free"},
		{URL: "", Description: "no url"}, // skipped
	}
	out, added := AppendNewFindings(existing, findings)
	if added != 1 {
		t.Fatalf("expected 1 added, got %d", added)
	}
	if len(out.Pending) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(out.Pending))
	}
	if !out.Pending[0].Reviewed {
		t.Errorf("existing reviewed entry lost")
	}
	if out.Pending[1].Source != "https://example.com/y" || out.Pending[1].Reviewed {
		t.Errorf("new entry wrong: %+v", out.Pending[1])
	}
	if out.Pending[1].WhyFree != "free" {
		t.Errorf("why_free wrong: %q", out.Pending[1].WhyFree)
	}
}

// TestTruncateWhyFree ensures long descriptions are clipped rune-safely.
func TestTruncateWhyFree(t *testing.T) {
	f := &models.Finding{URL: "https://example.com/z", Description: repeatRune('x', 600)}
	out, added := AppendNewFindings(PendingReview{Pending: []PendingItem{}}, []*models.Finding{f})
	if added != 1 {
		t.Fatalf("expected 1 added, got %d", added)
	}
	wf := out.Pending[0].WhyFree
	runes := []rune(wf)
	if len(runes) != maxWhyFreeLen+1 { // truncated to max + ellipsis
		t.Fatalf("why_free not truncated as expected: %d runes", len(runes))
	}
	if runes[len(runes)-1] != '…' {
		t.Errorf("expected ellipsis when truncated, got %q", string(runes[len(runes)-1]))
	}
	if !utf8.ValidString(wf) {
		t.Errorf("why_free is not valid UTF-8: %q", wf)
	}
}

func repeatRune(r rune, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = r
	}
	return string(b)
}
