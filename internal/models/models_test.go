package models

import (
	"testing"
)

func TestFindingFingerprint(t *testing.T) {
	f1 := Finding{
		Title: "Test Provider",
		URL:   "https://example.com",
	}
	f2 := Finding{
		Title: "Test Provider",
		URL:   "https://example.com",
	}
	f3 := Finding{
		Title: "Different Provider",
		URL:   "https://other.com",
	}

	fp1 := f1.Fingerprint()
	fp2 := f2.Fingerprint()
	fp3 := f3.Fingerprint()

	if fp1 != fp2 {
		t.Errorf("Same findings should have same fingerprint: %s != %s", fp1, fp2)
	}
	if fp1 == fp3 {
		t.Errorf("Different findings should have different fingerprint: %s == %s", fp1, fp3)
	}
}

func TestFindingFingerprintWithProviderName(t *testing.T) {
	name := "Groq"
	f1 := Finding{
		Title:        "Some title",
		URL:          "https://example.com",
		ProviderName: &name,
	}
	f2 := Finding{
		Title:        "Different title",
		URL:          "https://example.com",
		ProviderName: &name,
	}

	fp1 := f1.Fingerprint()
	fp2 := f2.Fingerprint()

	if fp1 != fp2 {
		t.Errorf("Same provider+url should have same fingerprint: %s != %s", fp1, fp2)
	}
}

func TestProviderStatus(t *testing.T) {
	tests := []struct {
		status ProviderStatus
		want   string
	}{
		{StatusVerified, "verified"},
		{StatusConfirmed, "confirmed"},
		{StatusClaimed, "claimed"},
		{StatusExpired, "expired"},
		{StatusUnverified, "unverified"},
		{StatusDeprioritized, "deprioritized"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("ProviderStatus: got %s, want %s", tt.status, tt.want)
		}
	}
}

func TestPriority(t *testing.T) {
	if PriorityHigh != 1 {
		t.Errorf("PriorityHigh should be 1, got %d", PriorityHigh)
	}
	if PrioritySkip != 9 {
		t.Errorf("PrioritySkip should be 9, got %d", PrioritySkip)
	}
}

func TestNow(t *testing.T) {
	n := Now()
	if n == "" {
		t.Error("Now() should return non-empty string")
	}
}
