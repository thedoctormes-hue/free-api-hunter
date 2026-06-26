package tts

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func writeKeyPoolConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "keypool.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewKeyPool(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "elevenlabs", "name": "ElevenLabs", "char_limit": 10000, "keys": ["key1", "key2"]},
			{"id": "openai", "name": "OpenAI", "char_limit": 50000, "keys": ["key3"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatalf("NewKeyPool failed: %v", err)
	}
	if pool == nil {
		t.Fatal("pool is nil")
	}
}

func TestNewKeyPoolFileNotFound(t *testing.T) {
	_, err := NewKeyPool("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestNewKeyPoolInvalidJSON(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `not json`)
	_, err := NewKeyPool(cfg)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNewKeyPoolEmptyProviders(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{"tts_providers": []}`)
	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatalf("NewKeyPool failed: %v", err)
	}
	if _, err := pool.Next(); err == nil {
		t.Error("expected error for empty pool")
	}
}

func TestNextRoundRobin(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 1000, "keys": ["a", "b", "c"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should cycle through keys in order
	seen := make(map[string]int)
	for i := 0; i < 9; i++ {
		entry, err := pool.Next()
		if err != nil {
			t.Fatalf("Next() failed: %v", err)
		}
		seen[entry.Key]++
	}

	for _, k := range []string{"a", "b", "c"} {
		if seen[k] != 3 {
			t.Errorf("key %s seen %d times, want 3", k, seen[k])
		}
	}
}

func TestNextSkipsExhausted(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100, "keys": ["k1", "k2"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Exhaust k1
	pool.ReportUsage("k1", 100)

	// Next should return k2 (skip exhausted k1)
	entry, err := pool.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if entry.Key != "k2" {
		t.Errorf("expected k2, got %s", entry.Key)
	}
}

func TestNextAllExhausted(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 50, "keys": ["x", "y"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pool.ReportUsage("x", 50)
	pool.ReportUsage("y", 50)

	_, err = pool.Next()
	if err == nil {
		t.Error("expected error when all keys exhausted")
	}
}

func TestNextForProvider(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 1000, "keys": ["p1key1", "p1key2"]},
			{"id": "p2", "name": "P2", "char_limit": 2000, "keys": ["p2key1"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	entry, err := pool.NextForProvider("p2")
	if err != nil {
		t.Fatalf("NextForProvider failed: %v", err)
	}
	if entry.Provider != "p2" {
		t.Errorf("expected provider p2, got %s", entry.Provider)
	}
	if entry.Key != "p2key1" {
		t.Errorf("expected p2key1, got %s", entry.Key)
	}
}

func TestNextForProviderNotFound(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 1000, "keys": ["k1"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.NextForProvider("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestNextForProviderAllExhausted(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100, "keys": ["k1"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pool.ReportUsage("k1", 100)
	_, err = pool.NextForProvider("p1")
	if err == nil {
		t.Error("expected error when all keys for provider exhausted")
	}
}

func TestReportUsage(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100, "keys": ["k1"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Partial usage — key still active
	pool.ReportUsage("k1", 50)
	entry, err := pool.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !entry.Active {
		t.Error("key should still be active at 50/100")
	}

	// Exceed limit
	pool.ReportUsage("k1", 60)
	entry, err = pool.Next()
	if err == nil {
		t.Error("expected error after exhausting key")
	}
	_ = entry
}

func TestReportUsageUnknownKey(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100, "keys": ["k1"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic
	pool.ReportUsage("nonexistent", 999)
}

func TestReportError(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 1000, "keys": ["k1", "k2"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pool.ReportError("k1")

	// k1 should be skipped, only k2 returned
	for i := 0; i < 4; i++ {
		entry, err := pool.Next()
		if err != nil {
			t.Fatalf("Next() failed: %v", err)
		}
		if entry.Key == "k1" {
			t.Error("k1 should be inactive after ReportError")
		}
	}
}

func TestReportErrorUnknownKey(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 1000, "keys": ["k1"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic
	pool.ReportError("nonexistent")
}

func TestStats(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100, "keys": ["a", "b"]},
			{"id": "p2", "name": "P2", "char_limit": 200, "keys": ["c"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pool.ReportUsage("a", 100) // exhaust a
	pool.ReportError("c")      // deactivate c

	stats := pool.Stats()
	if stats["total_keys"].(int) != 3 {
		t.Errorf("total_keys = %v, want 3", stats["total_keys"])
	}
	if stats["active"].(int) != 1 {
		t.Errorf("active = %v, want 1", stats["active"])
	}
	if stats["exhausted"].(int) != 2 {
		t.Errorf("exhausted = %v, want 2", stats["exhausted"])
	}

	byProvider := stats["by_provider"].(map[string]map[string]int)
	if byProvider["p1"]["active"] != 1 {
		t.Errorf("p1 active = %v, want 1", byProvider["p1"]["active"])
	}
	if byProvider["p2"]["exhausted"] != 1 {
		t.Errorf("p2 exhausted = %v, want 1", byProvider["p2"]["exhausted"])
	}
}

func TestSaveState(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100, "keys": ["k1"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pool.ReportUsage("k1", 42)

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := pool.SaveState(statePath); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("state file is empty")
	}
}

func TestReload(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100, "keys": ["k1"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pool.ReportUsage("k1", 50)
	pool.Reload(cfg)

	// After reload, usage should be preserved
	entry, err := pool.Next()
	if err != nil {
		t.Fatal(err)
	}
	if entry.CharsUsed != 50 {
		t.Errorf("CharsUsed = %d, want 50 (should be preserved after reload)", entry.CharsUsed)
	}
}

func TestReloadPreservesActive(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100, "keys": ["k1", "k2"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pool.ReportError("k1")
	pool.Reload(cfg)

	// k1 should still be inactive after reload
	for i := 0; i < 4; i++ {
		entry, err := pool.Next()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Key == "k1" {
			t.Error("k1 should remain inactive after reload")
		}
	}
}

// ─── Race condition tests ───

func TestConcurrentNext(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100000, "keys": ["a", "b", "c", "d", "e"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = pool.Next()
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentReportUsage(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100000, "keys": ["a", "b", "c"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				pool.ReportUsage("a", 1)
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentNextForProvider(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100000, "keys": ["a1", "a2"]},
			{"id": "p2", "name": "P2", "char_limit": 100000, "keys": ["b1", "b2"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = pool.NextForProvider("p1")
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = pool.NextForProvider("p2")
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentMixed(t *testing.T) {
	cfg := writeKeyPoolConfig(t, `{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 100000, "keys": ["x", "y", "z"]}
		]
	}`)

	pool, err := NewKeyPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = pool.Next()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				pool.ReportUsage("x", 1)
			}
		}()
		go func() {
			defer wg.Done()
			_ = pool.Stats()
		}()
	}
	wg.Wait()
}

// ─── Benchmark ───

func BenchmarkNext(b *testing.B) {
	// Use a temp file approach for benchmark
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.json")
	os.WriteFile(path, []byte(`{
		"tts_providers": [
			{"id": "p1", "name": "P1", "char_limit": 1000000, "keys": ["a", "b", "c", "d", "e"]}
		]
	}`), 0644)

	pool, err := NewKeyPool(path)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.Next()
		}
	})
}
