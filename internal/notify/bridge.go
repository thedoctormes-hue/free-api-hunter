// Package notify bridges discovery findings from the SQLite DB into the
// pending_review.json contract that the Mongoose agent consumes in its
// heartbeat loop.
//
// Contract (fixed, do NOT change the schema):
//
//	{ "pending": [ { "provider": str, "source": str, "why_free": str, "found_at": ISO, "reviewed": bool, "verdict": str } ] }
//
// The writer only APPENDS new findings (matched by source URL) and never
// rewrites existing entries, so the Mongoose agent's reviewed:true updates are
// preserved across bridge runs. Writes are atomic (temp file + rename).
package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/storage"
)

// PendingReview is the top-level structure of pending_review.json.
type PendingReview struct {
	Pending []PendingItem `json:"pending"`
}

// PendingItem is one candidate free-key finding awaiting human review by the
// Mongoose agent.
type PendingItem struct {
	Provider string `json:"provider"`
	Source   string `json:"source"`
	WhyFree  string `json:"why_free"`
	FoundAt  string `json:"found_at"`
	Reviewed bool   `json:"reviewed"`
	Verdict  string `json:"verdict"`
}

// LoadPendingReview reads pending_review.json. A missing file is treated as an
// empty structure (the bridge will then create it).
func LoadPendingReview(path string) (PendingReview, error) {
	pr := PendingReview{Pending: []PendingItem{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pr, nil
		}
		return pr, fmt.Errorf("read pending_review.json: %w", err)
	}
	if err := json.Unmarshal(data, &pr); err != nil {
		return pr, fmt.Errorf("parse pending_review.json: %w", err)
	}
	if pr.Pending == nil {
		pr.Pending = []PendingItem{}
	}
	return pr, nil
}

// maxWhyFreeLen caps the why_free field so we don't dump huge raw HTML blobs
// (the findings.description is the raw scraped snippet) into the review file
// that an LLM agent reads in its heartbeat. Truncation is not "inventing" a
// value — it's the same real description, just clipped for readability.
const maxWhyFreeLen = 500

// truncate clips s to at most max runes, appending an ellipsis when clipped.
// Runes (not bytes) are used so multi-byte UTF-8 is never split.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// AppendNewFindings returns a copy of existing with new findings appended.
// A finding is appended only if its source URL is not already present in
// existing (dedup by source URL). All existing entries — including those with
// reviewed:true — are preserved verbatim. Duplicates (is_duplicate) and
// findings without a source URL are skipped.
//
// It is a pure function (no I/O) so it can be unit-tested without a database.
func AppendNewFindings(existing PendingReview, findings []*models.Finding) (PendingReview, int) {
	seen := make(map[string]bool, len(existing.Pending))
	for _, it := range existing.Pending {
		if it.Source != "" {
			seen[it.Source] = true
		}
	}

	// Copy existing entries (value type) so we never mutate the caller's slice.
	out := PendingReview{Pending: append([]PendingItem{}, existing.Pending...)}

	added := 0
	for _, f := range findings {
		if f.IsDuplicate {
			continue // already-seen duplicates add noise; skip
		}
		if f.URL == "" {
			continue // cannot dedupe without a source URL
		}
		if seen[f.URL] {
			continue // already surfaced previously
		}
		seen[f.URL] = true

		// PAT-005: do not invent facts. provider_name is rarely populated in
		// the findings table, so it stays "" when unknown.
		provider := ""
		if f.ProviderName != nil {
			provider = *f.ProviderName
		}
		whyFree := f.Description
		if whyFree == "" {
			whyFree = "unknown"
		} else {
			whyFree = truncate(whyFree, maxWhyFreeLen)
		}
		foundAt := f.DiscoveredAt
		if foundAt == "" {
			foundAt = models.Now()
		}

		out.Pending = append(out.Pending, PendingItem{
			Provider: provider,
			Source:   f.URL,
			WhyFree:  whyFree,
			FoundAt:  foundAt,
			Reviewed: false,
			Verdict:  "",
		})
		added++
	}
	return out, added
}

// ComputeBridge loads the existing pending_review.json and the findings from
// the (already-initialized) DB, and returns the merged structure plus the
// number of new items — WITHOUT writing to disk. Useful for dry runs.
func ComputeBridge(outPath string) (int, PendingReview, error) {
	existing, err := LoadPendingReview(outPath)
	if err != nil {
		return 0, existing, err
	}
	findings, err := storage.LoadFindings("")
	if err != nil {
		return 0, existing, fmt.Errorf("load findings from DB: %w", err)
	}
	updated, added := AppendNewFindings(existing, findings)
	return added, updated, nil
}

// BridgePendingReview loads existing pending_review.json, appends new findings
// from the DB (matched by source URL), and writes the result back atomically.
// Returns the number of newly added items.
func BridgePendingReview(outPath string) (int, error) {
	added, updated, err := ComputeBridge(outPath)
	if err != nil {
		return added, err
	}
	if added == 0 {
		return 0, nil
	}
	if err := writeAtomic(outPath, updated); err != nil {
		return added, err
	}
	return added, nil
}

// writeAtomic serializes data and replaces outPath atomically via a temp file
// in the same directory + rename.
func writeAtomic(outPath string, data PendingReview) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pending_review.json: %w", err)
	}
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir for pending_review.json: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".pending_review_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup: if rename succeeds the file is gone; if not, this
	// removes the orphan temp file.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, outPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
