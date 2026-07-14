package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlePrometheusMetrics(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	IncAPIRequests("GET", "/health", 200)
	SetProvidersTotal(5)
	SetFindingsTotal(3)
	ObserveScanDuration(0.05)
	ObserveScanDuration(100) // exceeds top bucket → +Inf

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain, got %s", ct)
	}
	body := w.Body.String()
	for _, want := range []string{"api_requests_total", "providers_total 5", "findings_total 3", "scan_duration_seconds_bucket", "hunter_uptime_seconds"} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}

func TestHandlePrometheusMetricsMethodNotAllowed(t *testing.T) {
	dir := setupTestDir(t)
	defer cleanupDB(t)
	s := NewServerWithDir("127.0.0.1:0", dir)

	req := httptest.NewRequest("POST", "/metrics", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestMetricsHelpers(t *testing.T) {
	Inc("test_counter_a")
	SetGauge("test_gauge_a", 1.5)

	m := GetMetrics()
	counters, ok := m["counters"].(map[string]int64)
	if !ok {
		t.Fatal("counters not present")
	}
	if counters["test_counter_a"] != 1 {
		t.Errorf("expected test_counter_a=1, got %d", counters["test_counter_a"])
	}
	gauges, ok := m["gauges"].(map[string]float64)
	if !ok {
		t.Fatal("gauges not present")
	}
	if gauges["test_gauge_a"] != 1.5 {
		t.Errorf("expected test_gauge_a=1.5, got %v", gauges["test_gauge_a"])
	}
}

func TestObserveScanDuration(t *testing.T) {
	// Exercising both small and large observations.
	ObserveScanDuration(0.01)
	ObserveScanDuration(0.5)
	ObserveScanDuration(999)
}

func TestFormatFloatV2(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{1, "1.0"},
		{1.5, "1.5"},
		{100, "100.0"},
	}
	for _, c := range cases {
		if got := formatFloatV2(c.in); got != c.want {
			t.Errorf("formatFloatV2(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHandleLegacyMetrics(t *testing.T) {
	setupTestDir(t)
	defer cleanupDB(t)

	Inc("legacy_test_counter")
	w := httptest.NewRecorder()
	handleLegacyMetrics(w, httptest.NewRequest("GET", "/x", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "legacy_test_counter") {
		t.Errorf("legacy metrics missing counter: %s", w.Body.String())
	}
}

func TestHandleLegacyMetricsPrometheus(t *testing.T) {
	setupTestDir(t)
	defer cleanupDB(t)

	Inc("legacy_prom_counter")
	w := httptest.NewRecorder()
	handleLegacyMetricsPrometheus(w, httptest.NewRequest("GET", "/x", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "legacy_prom_counter") {
		t.Errorf("legacy prometheus metrics missing counter: %s", w.Body.String())
	}
}
