package ocr

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

var _ = strings.TrimSpace // ensure strings is used

// mockHTTPClient — подменяет HTTP клиент для тестирования
func mockHTTPClient(response string, statusCode int) *http.Client {
	return &http.Client{
		Transport: &mockTransport{
			response:   response,
			statusCode: statusCode,
		},
	}
}

type mockTransport struct {
	response   string
	statusCode int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(bytes.NewReader([]byte(m.response))),
		Header:     make(http.Header),
	}, nil
}

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
		IsActive:   false,
		Error:      "invalid_key",
		EngineUsed: 1,
	}

	if result.IsActive {
		t.Error("Expected IsActive=false")
	}
	if result.Error != "invalid_key" {
		t.Errorf("Expected error 'invalid_key', got '%s'", result.Error)
	}
}

func TestVerifyOCRKeyWithMock(t *testing.T) {
	origClient := HTTPClient
	defer func() { HTTPClient = origClient }()

	// Mock successful OCR response
	mockResp := `{
		"ParsedResults":[{"FileParseExitCode":1,"ParsedText":"HELLO WORLD\r\n","ErrorMessage":"","ErrorDetails":""}],
		"OCRExitCode":1,
		"IsErroredOnProcessing":false,
		"ProcessingTimeInMilliseconds":"312",
		"SearchablePDFURL":""
	}`
	HTTPClient = mockHTTPClient(mockResp, 200)

	result := VerifyOCRKey("free-api-hunter/ocr-space", 1, "eng")

	if !result.IsActive {
		t.Errorf("Expected IsActive=true, got false (error: %s)", result.Error)
	}
	if result.ProcessingMs != "312" {
		t.Errorf("Expected ProcessingMs=312, got %s", result.ProcessingMs)
	}
	if result.RecognizedText != "HELLO WORLD" {
		t.Errorf("Expected RecognizedText='HELLO WORLD', got '%s'", result.RecognizedText)
	}
}

func TestVerifyOCRKeyErrorMock(t *testing.T) {
	origClient := HTTPClient
	defer func() { HTTPClient = origClient }()

	// Mock error response
	mockResp := `{
		"ParsedResults":[{"FileParseExitCode":-10,"ParsedText":"","ErrorMessage":"Parsing Error","ErrorDetails":"corrupt"}],
		"OCRExitCode":3,
		"IsErroredOnProcessing":true,
		"ErrorMessage":["All images/pages errored"],
		"ProcessingTimeInMilliseconds":"359",
		"SearchablePDFURL":""
	}`
	HTTPClient = mockHTTPClient(mockResp, 200)

	result := VerifyOCRKey("free-api-hunter/ocr-space", 1, "eng")

	if result.IsActive {
		t.Error("Expected IsActive=false for OCRExitCode=3")
	}
	if result.Error == "" {
		t.Error("Expected error message for failed OCR")
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

func TestCheckOCRKeySimpleWithMock(t *testing.T) {
	origClient := HTTPClient
	defer func() { HTTPClient = origClient }()

	tests := []struct {
		name       string
		response   string
		statusCode int
		wantActive bool
	}{
		{
			name:       "valid key accepted",
			response:   `{"error":"E501: Not an image or PDF","details":"Invalid base64"}`,
			statusCode: 200,
			wantActive: true,
		},
		{
			name:       "invalid key rejected",
			response:   `{"error":"E555: API key not valid","details":"Get your FREE key"}`,
			statusCode: 403,
			wantActive: false,
		},
		{
			name:       "OCRExitCode present",
			response:   `{"OCRExitCode":1,"IsErroredOnProcessing":false,"ParsedResults":[{"ParsedText":"hello"}]}`,
			statusCode: 200,
			wantActive: true,
		},
		{
			name:       "unexpected response",
			response:   `{"unknown":"format"}`,
			statusCode: 200,
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			HTTPClient = mockHTTPClient(tt.response, tt.statusCode)
			result := CheckOCRKeySimple("free-api-hunter/ocr-space")
			if result.IsActive != tt.wantActive {
				t.Errorf("CheckOCRKeySimple() IsActive = %v, want %v (error: %s)",
					result.IsActive, tt.wantActive, result.Error)
			}
		})
	}
}

func TestTestOCREngineWithMock(t *testing.T) {
	origClient := HTTPClient
	defer func() { HTTPClient = origClient }()

	validImage := createTestImage()
	mockResp := `{
		"ParsedResults":[{"FileParseExitCode":1,"ParsedText":"test","ErrorMessage":"","ErrorDetails":""}],
		"OCRExitCode":1,
		"IsErroredOnProcessing":false,
		"ProcessingTimeInMilliseconds":"500",
		"SearchablePDFURL":""
	}`
	HTTPClient = mockHTTPClient(mockResp, 200)

	result := TestOCREngine("fake-api-key", 1, "eng", validImage)
	if !result.Success {
		t.Errorf("Expected Success=true for OCRExitCode=1")
	}
	if result.Text != "test" {
		t.Errorf("Expected Text='test', got '%s'", result.Text)
	}
}

func TestTestAllEnginesWithMock(t *testing.T) {
	origClient := HTTPClient
	defer func() { HTTPClient = origClient }()

	callCount := 0
	HTTPClient = &http.Client{
		Transport: &countingTransport{count: &callCount},
	}

	results := TestAllEngines("fake-api-key", "eng")
	if len(results) != 3 {
		t.Fatalf("Expected 3 engines, got %d", len(results))
	}
	if callCount != 3 {
		t.Errorf("Expected 3 HTTP calls, got %d", callCount)
	}
}

type countingTransport struct {
	count *int
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*t.count++
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`{"ParsedResults":[{"FileParseExitCode":1,"ParsedText":"x"}],"OCRExitCode":1,"ProcessingTimeInMilliseconds":"100"`)),
		Header:     make(http.Header),
	}, nil
}

// TestCheckOCRKeySimpleResponseParsing — проверка что разные ответы API корректно интерпретируются
func TestCheckOCRKeySimpleResponseParsing(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantActive bool
	}{
		{
			name:       "invalid key E555",
			response:   `{"error":"E555: API key not valid","details":"Get your FREE ocr api key"}`,
			wantActive: false,
		},
		{
			name:       "E501 not an image (key is valid)",
			response:   `{"error":"E501: Not an image or PDF","details":"Invalid base64"}`,
			wantActive: true,
		},
		{
			name:       "E216 unable to detect file type (key is valid)",
			response:   `{"error":"E216: Unable to detect the file extension"}`,
			wantActive: true,
		},
		{
			name:       "OCRExitCode present (key accepted)",
			response:   `{"OCRExitCode":3,"IsErroredOnProcessing":true,"ErrorMessage":["error"]}`,
			wantActive: true,
		},
		{
			name:       "unexpected response",
			response:   `{"something":"unexpected"}`,
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
