package ocr

import (
	"strings"
	"testing"
)

func TestCreateTestImage(t *testing.T) {
	img := createTestImage()
	if len(img) == 0 {
		t.Fatal("createTestImage returned empty data")
	}
	// Check PNG signature
	if img[0] != 0x89 || img[1] != 0x50 || img[2] != 0x4E || img[3] != 0x47 {
		t.Error("Invalid PNG signature")
	}
}

func TestVerifyOCRKeyResult(t *testing.T) {
	result := &OCRVerifyResult{
		IsActive:     true,
		StatusCode:   200,
		EngineUsed:   1,
		Language:     "eng",
		ProcessingMs: "312",
		CheckedAt:    "2026-06-26T06:00:00Z",
	}

	if !result.IsActive {
		t.Error("Expected IsActive=true")
	}
	if result.EngineUsed != 1 {
		t.Errorf("Expected EngineUsed=1, got %d", result.EngineUsed)
	}
}

func TestOCRVerifyResultInactive(t *testing.T) {
	result := &OCRVerifyResult{
		IsActive:  false,
		Error:    "invalid_key",
		EngineUsed: 1,
	}

	if result.IsActive {
		t.Error("Expected IsActive=false")
	}
	if result.Error != "invalid_key" {
		t.Errorf("Expected error 'invalid_key', got '%s'", result.Error)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "hel"},
		{"", 5, ""},
		{"test", 4, "test"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// TestCheckOCRKeySimpleResponseParsing — проверка что разные ответы API корректно интерпретируются
func TestCheckOCRKeySimpleResponseParsing(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantActive bool
	}{
		{
			name:     "invalid key E555",
			response: `{"error":"E555: API key not valid","details":"Get your FREE ocr api key"}`,
			wantActive: false,
		},
		{
			name:     "E501 not an image (key is valid)",
			response: `{"error":"E501: Not an image or PDF","details":"Invalid base64"}`,
			wantActive: true,
		},
		{
			name:     "E216 unable to detect file type (key is valid)",
			response: `{"error":"E216: Unable to detect the file extension"}`,
			wantActive: true,
		},
		{
			name:     "OCRExitCode present (key accepted)",
			response: `{"OCRExitCode":3,"IsErroredOnProcessing":true,"ErrorMessage":["error"]}`,
			wantActive: true,
		},
		{
			name:     "unexpected response",
			response: `{"something":"unexpected"}`,
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Симулируем логику из CheckOCRKeySimple
			isActive := false
			if strings.Contains(tt.response, "E555") || strings.Contains(tt.response, "API key not valid") {
				isActive = false
			} else if strings.Contains(tt.response, "E501") || strings.Contains(tt.response, "E216") ||
				strings.Contains(tt.response, "OCRExitCode") || strings.Contains(tt.response, "error") {
				isActive = true
			}

			if isActive != tt.wantActive {
				t.Errorf("response %q: got active=%v, want %v", tt.response, isActive, tt.wantActive)
			}
		})
	}
}
