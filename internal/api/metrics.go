package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Prometheus Metrics ───

// prometheusMetrics is the global Prometheus-compatible metrics registry.
type prometheusMetrics struct {
	mu sync.RWMutex

	// api_requests_total counter with labels method, path, status
	apiRequestsTotal map[string]float64 // key: "METHOD:path:status"

	// scan_duration_seconds histogram
	scanDurationHistogram struct {
		buckets  []float64
		counts   []uint64
		sum      float64
		totalCnt uint64
	}

	// Gauges
	providersTotal float64
	findingsTotal  float64

	StartTime time.Time
}

var prometheus = &prometheusMetrics{
	apiRequestsTotal: make(map[string]float64),
	StartTime:        time.Now(),
}

// Histogram buckets (seconds): 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
var defaultBuckets = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func newPrometheusMetrics() *prometheusMetrics {
	m := &prometheusMetrics{
		apiRequestsTotal: make(map[string]float64),
		StartTime:        time.Now(),
	}
	m.scanDurationHistogram.buckets = defaultBuckets
	m.scanDurationHistogram.counts = make([]uint64, len(defaultBuckets)+1)
	return m
}

func init() {
	prometheus = newPrometheusMetrics()
}

// IncAPIRequests — increment api_requests_total counter
func IncAPIRequests(method, path string, status int) {
	key := fmt.Sprintf("%s:%s:%d", method, path, status)
	prometheus.mu.Lock()
	defer prometheus.mu.Unlock()
	prometheus.apiRequestsTotal[key]++
}

// SetProvidersTotal — set providers_total gauge
func SetProvidersTotal(n float64) {
	prometheus.mu.Lock()
	defer prometheus.mu.Unlock()
	prometheus.providersTotal = n
}

// SetFindingsTotal — set findings_total gauge
func SetFindingsTotal(n float64) {
	prometheus.mu.Lock()
	defer prometheus.mu.Unlock()
	prometheus.findingsTotal = n
}


// bucketBoundaries defines the upper bounds for the scan_duration histogram.
var bucketBoundaries = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300}

// ObserveScanDuration — record scan_duration_seconds observation.
// Uses cumulative histogram semantics: all buckets whose upper bound
// is >= the observation are incremented. +Inf (last bucket) always increments.
func ObserveScanDuration(seconds float64) {
	h := &prometheus.scanDurationHistogram
	h.sum += seconds
	h.totalCnt++

	// Ensure counts slice is large enough
	for len(h.counts) <= len(bucketBoundaries) {
		h.counts = append(h.counts, 0)
	}

	for i, bound := range bucketBoundaries {
		if seconds <= bound {
			h.counts[i]++
		}
	}
	h.counts[len(bucketBoundaries)]++ // +Inf
}

// ─── Prometheus Text Exposition ───

// handlePrometheusMetrics — GET /metrics (Prometheus text format)
func (s *Server) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	prometheus.mu.RLock()
	defer prometheus.mu.RUnlock()

	// Sort keys for deterministic output
	var keys []string
	for k := range prometheus.apiRequestsTotal {
		keys = append(keys, k)
	}
	// Simple sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	fmt.Fprintln(w, "# HELP api_requests_total Total number of API requests")
	fmt.Fprintln(w, "# TYPE api_requests_total counter")
	for _, key := range keys {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) == 3 {
			fmt.Fprintf(w, "api_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %s\n",
				parts[0], parts[1], parts[2], formatFloatV2(prometheus.apiRequestsTotal[key]))
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "# HELP providers_total Total number of API providers")
	fmt.Fprintln(w, "# TYPE providers_total gauge")
	fmt.Fprintf(w, "providers_total %s\n", formatFloatV2(prometheus.providersTotal))

	fmt.Fprintln(w)
	fmt.Fprintln(w, "# HELP findings_total Total number of findings")
	fmt.Fprintln(w, "# TYPE findings_total gauge")
	fmt.Fprintf(w, "findings_total %s\n", formatFloatV2(prometheus.findingsTotal))

	fmt.Fprintln(w)
	fmt.Fprintln(w, "# HELP scan_duration_seconds Duration of scan operations")
	fmt.Fprintln(w, "# TYPE scan_duration_seconds histogram")

	h := &prometheus.scanDurationHistogram
	bounds := defaultBuckets
	if len(h.counts) > 1 {
		// Determine which bucket set we're using
		if len(h.counts) == len(bucketBoundaries)+1 {
			bounds = bucketBoundaries
		}
	}

	for i, bound := range bounds {
		var count uint64
		if i < len(h.counts) {
			count = h.counts[i]
		}
		fmt.Fprintf(w, "scan_duration_seconds_bucket{le=\"%s\"} %d\n",
			formatFloatV2(bound), count)
	}
	fmt.Fprintf(w, "scan_duration_seconds_bucket{le=\"+Inf\"} %d\n", h.totalCnt)
	fmt.Fprintf(w, "scan_duration_seconds_sum %s\n", formatFloatV2(h.sum))
	fmt.Fprintf(w, "scan_duration_seconds_count %d\n", h.totalCnt)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "# HELP hunter_uptime_seconds Server uptime in seconds")
	fmt.Fprintln(w, "# TYPE hunter_uptime_seconds gauge")
	fmt.Fprintf(w, "hunter_uptime_seconds %s\n", formatFloatV2(time.Since(prometheus.StartTime).Seconds()))
}

func formatFloatV2(f float64) string {
	if f == 0 {
		return "0"
	}
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10) + ".0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ─── Legacy in-memory metrics (kept for backward compat with middleware.go) ───

type legacyMetrics struct {
	mu        sync.RWMutex
	counters  map[string]int64
	gauges    map[string]float64
	startTime time.Time
}

var legacy = &legacyMetrics{
	counters:  make(map[string]int64),
	gauges:    make(map[string]float64),
	startTime: time.Now(),
}

func Inc(name string) {
	legacy.mu.Lock()
	defer legacy.mu.Unlock()
	legacy.counters[name]++
}

func SetGauge(name string, value float64) {
	legacy.mu.Lock()
	defer legacy.mu.Unlock()
	legacy.gauges[name] = value
}

func GetMetrics() map[string]interface{} {
	legacy.mu.RLock()
	defer legacy.mu.RUnlock()
	result := map[string]interface{}{
		"uptime_seconds": time.Since(legacy.startTime).Seconds(),
		"server_time":    time.Now().UTC().Format(time.RFC3339),
	}
	counters := make(map[string]int64)
	for k, v := range legacy.counters {
		counters[k] = v
	}
	result["counters"] = counters
	gauges := make(map[string]float64)
	for k, v := range legacy.gauges {
		gauges[k] = v
	}
	result["gauges"] = gauges
	return result
}

func handleLegacyMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(GetMetrics())
}

func handleLegacyMetricsPrometheus(w http.ResponseWriter, r *http.Request) {
	legacy.mu.RLock()
	defer legacy.mu.RUnlock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	uptime := time.Since(legacy.startTime).Seconds()
	fmt.Fprintln(w, "# HELP hunter_uptime_seconds Server uptime in seconds")
	fmt.Fprintln(w, "# TYPE hunter_uptime_seconds gauge")
	fmt.Fprintf(w, "hunter_uptime_seconds %s\n", formatFloatV2(uptime))
	for name, value := range legacy.counters {
		fmt.Fprintf(w, "# HELP hunter_%s Counter\n", name)
		fmt.Fprintf(w, "# TYPE hunter_%s counter\n", name)
		fmt.Fprintf(w, "hunter_%s %s\n", name, formatFloatV2(float64(value)))
	}
	for name, value := range legacy.gauges {
		fmt.Fprintf(w, "# HELP hunter_%s Gauge\n", name)
		fmt.Fprintf(w, "# TYPE hunter_%s gauge\n", name)
		fmt.Fprintf(w, "hunter_%s %s\n", name, formatFloatV2(value))
	}
}
