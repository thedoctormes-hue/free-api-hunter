package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Metrics — простой in-memory сборщик метрик
type Metrics struct {
	mu        sync.RWMutex
	counters  map[string]int64
	gauges    map[string]float64
	startTime time.Time
}

var metrics = &Metrics{
	counters:  make(map[string]int64),
	gauges:    make(map[string]float64),
	startTime: time.Now(),
}

// Inc — увеличить счётчик
func Inc(name string) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.counters[name]++
}

// SetGauge — установить значение gauge
func SetGauge(name string, value float64) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.gauges[name] = value
}

// GetMetrics — получить все метрики
func GetMetrics() map[string]interface{} {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	result := map[string]interface{}{
		"uptime_seconds":   time.Since(metrics.startTime).Seconds(),
		"server_time":      time.Now().UTC().Format(time.RFC3339),
	}

	counters := make(map[string]int64)
	for k, v := range metrics.counters {
		counters[k] = v
	}
	result["counters"] = counters

	gauges := make(map[string]float64)
	for k, v := range metrics.gauges {
		gauges[k] = v
	}
	result["gauges"] = gauges

	return result
}

// HandleMetrics — HTTP handler для /metrics (JSON формат)
func HandleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	enc.Encode(GetMetrics())
}

// HandleMetricsPrometheus — HTTP handler для /metrics/prometheus (Prometheus text format)
func HandleMetricsPrometheus(w http.ResponseWriter, r *http.Request) {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Uptime
	uptime := time.Since(metrics.startTime).Seconds()
	w.Write([]byte("# HELP hunter_uptime_seconds Server uptime in seconds\n"))
	w.Write([]byte("# TYPE hunter_uptime_seconds gauge\n"))
	w.Write([]byte("hunter_uptime_seconds " + formatFloat(uptime) + "\n"))

	// Counters
	for name, value := range metrics.counters {
		w.Write([]byte("# HELP hunter_" + name + " Counter\n"))
		w.Write([]byte("# TYPE hunter_" + name + " counter\n"))
		w.Write([]byte("hunter_" + name + " " + formatFloat(float64(value)) + "\n"))
	}

	// Gauges
	for name, value := range metrics.gauges {
		w.Write([]byte("# HELP hunter_" + name + " Gauge\n"))
		w.Write([]byte("# TYPE hunter_" + name + " gauge\n"))
		w.Write([]byte("hunter_" + name + " " + formatFloat(value) + "\n"))
	}
}

func formatFloat(f float64) string {
	return json.Number(floatToString(f)).String()
}

func floatToString(f float64) string {
	s := ""
	if f == float64(int64(f)) {
		s = "0"
		return "0"
	}
	// Простое форматирование
	buf := make([]byte, 0, 20)
	neg := f < 0
	if neg {
		f = -f
	}
	intPart := int64(f)
	fracPart := f - float64(intPart)
	buf = append(buf, byte('0'+intPart%10))
	fracPart *= 10
	for i := 0; i < 6; i++ {
		d := int(fracPart)
		buf = append(buf, byte('0'+d))
		fracPart = (fracPart - float64(d)) * 10
	}
	_ = neg
	_ = s
	return string(buf)
}
