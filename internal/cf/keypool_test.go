package cf

import (
	"testing"
)

func TestKeyPool_Next_Exhausted(t *testing.T) {
	pool := &KeyPool{
		accounts: []*AccountEntry{
			{
				Account:     Account{ID: "test-1", Name: "Test", Token: "key", Active: true},
				NeuronsUsed: 10000,
				NeuronsLimit: 10000,
			},
		},
	}

	// Account is already exhausted
	pool.accounts[0].Active = false

	_, err := pool.Next()
	if err == nil {
		t.Fatal("expected error when all accounts exhausted")
	}
}

func TestKeyPool_Next_Success(t *testing.T) {
	pool := &KeyPool{
		accounts: []*AccountEntry{
			{
				Account:     Account{ID: "test-1", Name: "Test", Token: "key1", Active: true},
				NeuronsUsed: 0,
				NeuronsLimit: 10000,
			},
			{
				Account:     Account{ID: "test-2", Name: "Test2", Token: "key2", Active: true},
				NeuronsUsed: 0,
				NeuronsLimit: 10000,
			},
		},
	}

	entry, err := pool.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID != "test-1" {
		t.Fatalf("expected test-1, got %s", entry.ID)
	}

	// Next call should return second account
	entry, err = pool.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID != "test-2" {
		t.Fatalf("expected test-2, got %s", entry.ID)
	}
}

func TestKeyPool_NextForModel(t *testing.T) {
	pool := &KeyPool{
		accounts: []*AccountEntry{
			{
				Account:     Account{ID: "acc-1", Name: "Acc1", Token: "key1", Active: true},
				NeuronsUsed: 9955,
				NeuronsLimit: 10000,
			},
			{
				Account:     Account{ID: "acc-2", Name: "Acc2", Token: "key2", Active: true},
				NeuronsUsed: 0,
				NeuronsLimit: 10000,
			},
		},
	}

	// Model that costs >50 neurons for this request (acc-1 has only 45 left)
	// 1000 input * 45455 / 1M = 45 neurons, 500 output * 136364 / 1M = 68 neurons
	// Total = 113 neurons > 45 remaining
	m := Model{NeuronsInput: 45455, NeuronsOutput: 136364}

	// First account only has 45 neurons left, should skip to second
	entry, err := pool.NextForModel(m, 1000, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID != "acc-2" {
		t.Fatalf("expected acc-2 (acc-1 has insufficient budget), got %s", entry.ID)
	}
}

func TestKeyPool_ResetDaily(t *testing.T) {
	pool := &KeyPool{
		accounts: []*AccountEntry{
			{
				Account:     Account{ID: "test", Active: false},
				NeuronsUsed: 5000,
				NeuronsLimit: 10000,
			},
		},
	}

	pool.ResetDaily()

	if !pool.accounts[0].Active {
		t.Fatal("expected account to be active after reset")
	}
	if pool.accounts[0].NeuronsUsed != 0 {
		t.Fatal("expected neurons used to be 0 after reset")
	}
}

func TestKeyPool_Stats(t *testing.T) {
	pool := &KeyPool{
		accounts: []*AccountEntry{
			{
				Account:     Account{ID: "test1", Name: "Test 1", Active: true},
				NeuronsUsed: 1000,
				NeuronsLimit: 10000,
			},
			{
				Account:     Account{ID: "test2", Name: "Test 2", Active: false},
				NeuronsUsed: 10000,
				NeuronsLimit: 10000,
			},
		},
	}

	stats := pool.Stats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}
	if stats[0]["active"] != true {
		t.Fatal("expected first account active")
	}
	if stats[1]["active"] != false {
		t.Fatal("expected second account inactive")
	}
}

func TestKeyPool_ReportUsage(t *testing.T) {
	pool := &KeyPool{
		accounts: []*AccountEntry{
			{
				Account:     Account{ID: "test", Active: true},
				NeuronsUsed: 0,
				NeuronsLimit: 10000,
			},
		},
	}

	pool.ReportUsage("test", 5000)
	if pool.accounts[0].NeuronsUsed != 5000 {
		t.Fatalf("expected 5000 neurons used, got %d", pool.accounts[0].NeuronsUsed)
	}
	if !pool.accounts[0].Active {
		t.Fatal("expected account still active")
	}

	// Exhaust the account
	pool.ReportUsage("test", 6000)
	if pool.accounts[0].Active {
		t.Fatal("expected account to be inactive after exceeding limit")
	}
}

func TestKeyPool_ReportUsage_UnknownAccount(t *testing.T) {
	pool := &KeyPool{
		accounts: []*AccountEntry{
			{
				Account:     Account{ID: "test", Active: true},
				NeuronsUsed: 0,
				NeuronsLimit: 10000,
			},
		},
	}

	// Should not panic for unknown account
	pool.ReportUsage("unknown", 5000)
	if pool.accounts[0].NeuronsUsed != 0 {
		t.Fatal("expected no change for unknown account")
	}
}
