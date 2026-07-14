package api

import (
	"os"
	"testing"
)

// TestMain disables the per-IP rate limit for the whole test run.
// Without this, the many subtests issuing requests from the same default
// RemoteAddr would exhaust the 100 req/min budget and start returning 429,
// making the suite order-dependent and flaky. TestRateLimitMiddleware
// overrides the limit locally to still exercise the throttling path.
func TestMain(m *testing.M) {
	globalRateLimiter.maxRequestsPerWindow = 1_000_000
	os.Exit(m.Run())
}
