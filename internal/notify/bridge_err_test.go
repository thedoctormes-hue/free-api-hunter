package notify

import (
	"os"
	"path/filepath"
	"testing"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/storage"
)

// TestComputeBridgeRejectedReadError forces loadRejected to fail (by shadowing
// rejected.json with a directory) so ComputeBridge's error path is exercised.
func TestComputeBridgeRejectedReadError(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{{Source: "s"}}})
	if err := os.Mkdir(filepath.Join(dir, "rejected.json"), 0755); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "pending_review.json")
	if _, _, err := ComputeBridge(outPath); err == nil {
		t.Errorf("expected ComputeBridge error from unreadable rejected.json")
	}
}

// TestComputeBridgeLoadFindingsError forces storage.LoadFindings to fail (no
// initialized DB) so ComputeBridge's DB-error path is exercised.
func TestComputeBridgeLoadFindingsError(t *testing.T) {
	storage.CloseDB() // ensure no DB so LoadFindings returns an error
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{{Source: "s"}}})
	outPath := filepath.Join(dir, "pending_review.json")
	if _, _, err := ComputeBridge(outPath); err == nil {
		t.Errorf("expected ComputeBridge error when DB unavailable")
	}
}

// TestComputeBridgeDenylistFilters verifies KRV П.1: a finding whose source URL
// is already in rejected.json is dropped before being offered for review.
func TestComputeBridgeDenylistFilters(t *testing.T) {
	dir := t.TempDir()
	if err := storage.InitDB(dir); err != nil {
		t.Fatal(err)
	}
	defer storage.CloseDB()
	f := &models.Finding{SourceID: "s1", URL: "https://deny.example.com", Description: "d", DiscoveredAt: models.Now()}
	if err := storage.SaveFindings([]*models.Finding{f}, ""); err != nil {
		t.Fatal(err)
	}
	if err := saveRejected(filepath.Join(dir, "rejected.json"), map[string]bool{"https://deny.example.com": true}); err != nil {
		t.Fatal(err)
	}
	added, _, err := ComputeBridge(filepath.Join(dir, "pending_review.json"))
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Errorf("denylisted finding should be skipped, got added=%d", added)
	}
}

// TestComputeBridgeLoadError verifies ComputeBridge surfaces a broken
// pending_review.json (here a directory) as a LoadPendingReview error.
func TestComputeBridgeLoadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "pending_review.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ComputeBridge(filepath.Join(dir, "pending_review.json")); err == nil {
		t.Errorf("expected ComputeBridge error from unreadable pending_review.json")
	}
}

// TestBridgePendingReviewLoadError verifies BridgePendingReview propagates a
// LoadPendingReview failure from ComputeBridge.
func TestBridgePendingReviewLoadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "pending_review.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := BridgePendingReview(filepath.Join(dir, "pending_review.json")); err == nil {
		t.Errorf("expected BridgePendingReview error from unreadable pending_review.json")
	}
}
func TestBridgePendingReviewError(t *testing.T) {
	dir := t.TempDir()
	writePending(t, dir, PendingReview{Pending: []PendingItem{{Source: "s"}}})
	if err := os.Mkdir(filepath.Join(dir, "rejected.json"), 0755); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "pending_review.json")
	if _, err := BridgePendingReview(outPath); err == nil {
		t.Errorf("expected BridgePendingReview error from unreadable rejected.json")
	}
}
