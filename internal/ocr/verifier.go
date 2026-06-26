package ocr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"free-api-hunter/internal/vault"
)

var logger = log.New(os.Stderr, "[ocr-verifier] ", log.LstdFlags)

// OCR API endpoints
const (
	OCRBaseURL      = "https://api.ocr.space"
	OCRParseEndpoint = "/parse/image"
)

// HTTPClient — настраиваемый HTTP клиент для OCR
var HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// OCRVerifyResult — результат верификации OCR-провайдера
type OCRVerifyResult struct {
	IsActive      bool     `json:"is_active"`
	StatusCode    int      `json:"status_code"`
	Error         string   `json:"error,omitempty"`
	EngineUsed    int      `json:"engine_used"`
	Language      string   `json:"language"`
	ProcessingMs  string   `json:"processing_ms"`
	RecognizedText string  `json:"recognized_text,omitempty"`
	CheckedAt     string   `json:"checked_at"`
}

// OCRTestResult — результат тестирования OCR с конкретным изображением
type OCRTestResult struct {
	Engine        int    `json:"engine"`
	Language      string `json:"language"`
	Success       bool   `json:"success"`
	ExitCode      int    `json:"exit_code"`
	Text          string `json:"text"`
	ProcessingMs  string `json:"processing_ms"`
	Error         string `json:"error,omitempty"`
}

// VerifyOCRKey — проверить что OCR API ключ рабочий
// Отправляет тестовое изображение и проверяет что распознавание сработало
func VerifyOCRKey(providerName string, engine int, language string) *OCRVerifyResult {
	result := &OCRVerifyResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		EngineUsed: engine,
		Language:   language,
	}

	// Получаем ключ из vault (providerName = "free-api-hunter/ocr-space")
	key, err := vault.GetDefaultKey(providerName)
	if err != nil {
		result.Error = "vault_error: " + err.Error()
		return result
	}

	// Создаём тестовое изображение
	testImage := createTestImage()

	// Отправляем запрос
	ocrResult, err := sendOCRRequest(key, testImage, engine, language)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.StatusCode = 200
	result.ProcessingMs = ocrResult.ProcessingMs
	result.RecognizedText = ocrResult.Text

	if ocrResult.Success {
		result.IsActive = true
		logger.Printf("OCR key for %s is ACTIVE (engine=%d, lang=%s, %sms)",
			providerName, engine, language, ocrResult.ProcessingMs)
	} else {
		result.IsActive = false
		result.Error = ocrResult.Error
		logger.Printf("OCR key for %s FAILED: %s", providerName, ocrResult.Error)
	}

	return result
}

// TestOCREngine — протестировать конкретный OCR-движок
func TestOCREngine(apiKey string, engine int, language string, imageData []byte) *OCRTestResult {
	result := &OCRTestResult{
		Engine:   engine,
		Language: language,
	}

	ocrResult, err := sendOCRRequest(apiKey, imageData, engine, language)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = ocrResult.Success
	result.ExitCode = ocrResult.ExitCode
	result.Text = ocrResult.Text
	result.ProcessingMs = ocrResult.ProcessingMs
	result.Error = ocrResult.Error

	return result
}

// TestAllEngines — протестировать все доступные OCR-движки
func TestAllEngines(apiKey string, language string) []*OCRTestResult {
	engines := []int{1, 2, 3}
	var results []*OCRTestResult

	testImage := createTestImage()

	for _, engine := range engines {
		result := TestOCREngine(apiKey, engine, language, testImage)
		results = append(results, result)
	}

	return results
}

// ocrAPIResponse — структура ответа OCR.space API
type ocrAPIResponse struct {
	ParsedResults []struct {
		TextOverlay struct {
			Lines []struct {
				LineText string `json:"LineText"`
				Words   []struct {
					WordText string  `json:"WordText"`
					Left     float64 `json:"Left"`
					Top      float64 `json:"Top"`
					Height   float64 `json:"Height"`
					Width    float64 `json:"Width"`
				} `json:"Words"`
				MaxHeight float64 `json:"MaxHeight"`
				MinTop    float64 `json:"MinTop"`
			} `json:"Lines"`
			HasOverlay bool   `json:"HasOverlay"`
			Message    string `json:"Message"`
		} `json:"TextOverlay"`
		TextOrientation    string `json:"TextOrientation"`
		FileParseExitCode  int    `json:"FileParseExitCode"`
		ParsedText         string `json:"ParsedText"`
		ErrorMessage       string `json:"ErrorMessage"`
		ErrorDetails       string `json:"ErrorDetails"`
	} `json:"ParsedResults"`
	OCRExitCode            int      `json:"OCRExitCode"`
	IsErroredOnProcessing  bool     `json:"IsErroredOnProcessing"`
	ErrorMessage           []string `json:"ErrorMessage"`
	ProcessingTimeInMilliseconds string `json:"ProcessingTimeInMilliseconds"`
	SearchablePDFURL       string `json:"SearchablePDFURL"`
}

// ocrInternalResult — внутренний результат парсинга
type ocrInternalResult struct {
	Success      bool
	ExitCode     int
	Text         string
	ProcessingMs string
	Error        string
}

// sendOCRRequest — отправить запрос в OCR.space API
func sendOCRRequest(apiKey string, imageData []byte, engine int, language string) (*ocrInternalResult, error) {
	result := &ocrInternalResult{}

	// Формируем multipart/form-data
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Поля формы
	writer.WriteField("apikey", apiKey)
	writer.WriteField("OCREngine", fmt.Sprintf("%d", engine))
	writer.WriteField("language", language)

	// Файл изображения
	part, err := writer.CreateFormFile("file", "test.png")
	if err != nil {
		return result, fmt.Errorf("failed to create form file: %w", err)
	}
	part.Write(imageData)

	writer.Close()

	// Отправляем POST-запрос
	url := OCRBaseURL + OCRParseEndpoint
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "FreeAPIHunter-OCR/0.1")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to read response: %w", err)
	}

	// Парсим JSON
	var ocrResp ocrAPIResponse
	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		// Может быть ошибка в формате JSON (например, searchable PDF возвращает бинарник)
		return result, fmt.Errorf("failed to parse response: %w", err)
	}

	result.ProcessingMs = ocrResp.ProcessingTimeInMilliseconds
	result.ExitCode = ocrResp.OCRExitCode

	// OCRExitCode: 1 = success, 2 = partial, 3+ = error
	if ocrResp.OCRExitCode == 1 || ocrResp.OCRExitCode == 2 {
		result.Success = true
		if len(ocrResp.ParsedResults) > 0 {
			result.Text = strings.TrimSpace(ocrResp.ParsedResults[0].ParsedText)
		}
	} else {
		result.Success = false
		if ocrResp.IsErroredOnProcessing && len(ocrResp.ErrorMessage) > 0 {
			result.Error = strings.Join(ocrResp.ErrorMessage, "; ")
		} else if len(ocrResp.ParsedResults) > 0 && ocrResp.ParsedResults[0].ErrorMessage != "" {
			result.Error = ocrResp.ParsedResults[0].ErrorMessage
		} else {
			result.Error = fmt.Sprintf("OCRExitCode: %d", ocrResp.OCRExitCode)
		}
	}

	return result, nil
}

// createTestImage — создать минимальное тестовое изображение (1x1 белый пиксель PNG)
// Используется для проверки валидности ключа (API вернёт ошибку распознавания, но не ошибку ключа)
func createTestImage() []byte {
	// Минимальный валидный PNG: 1x1 белый пиксель
	// Это не для распознавания текста, а для проверки что ключ принимается
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, // 8-bit RGB
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54, // IDAT chunk
		0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00, 0x00, // compressed data
		0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC, 0x33, // checksum
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, // IEND chunk
		0xAE, 0x42, 0x60, 0x82, // IEND CRC
	}
}

// createTestImageWithText — создать тестовое изображение с текстом "HELLO"
// Используется для полноценного тестирования OCR
func createTestImageWithText() []byte {
	// Возвращаем тот же минимальный PNG — для реального теста
	// нужно генерировать изображение с текстом (требует внешних зависимостей)
	// Для базовой проверки ключа достаточно минимального PNG
	return createTestImage()
}

// CheckOCRKeySimple — упрощённая проверка ключа (без отправки изображения)
// Отправляет запрос с невалидным файлом — если ключ невалидный, получим E555
func CheckOCRKeySimple(providerName string) *OCRVerifyResult {
	result := &OCRVerifyResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	key, err := vault.GetDefaultKey(providerName)
	if err != nil {
		result.Error = "vault_error: " + err.Error()
		return result
	}

	// Отправляем запрос с пустым base64 — проверяем только ключ
	url := OCRBaseURL + OCRParseEndpoint
	payload := fmt.Sprintf(`{"apikey":"%s","base64Image":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==","language":"eng"}`, key)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FreeAPIHunter-OCR/0.1")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		result.Error = "request_failed: " + err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	respBody, _ := io.ReadAll(resp.Body)
	bodyStr := string(respBody)

	// Проверяем на ошибку ключа
	if strings.Contains(bodyStr, "E555") || strings.Contains(bodyStr, "API key not valid") {
		result.Error = "invalid_key"
		result.IsActive = false
		logger.Printf("OCR key for %s: INVALID", providerName)
		return result
	}

	// Если получили ошибку обработки (E501, E216, E3 и т.д.) — ключ рабочий
	// E501 = Not an image (ключ принят, но файл невалидный)
	// E216 = Unable to detect file type (ключ принят)
	// OCRExitCode = распознавание завершено (успешно или нет)
	if strings.Contains(bodyStr, "E501") || strings.Contains(bodyStr, "E216") ||
		strings.Contains(bodyStr, "OCRExitCode") || strings.Contains(bodyStr, "error") {
		result.IsActive = true
		logger.Printf("OCR key for %s: ACTIVE (key accepted, processing error: %s)", providerName, truncate(bodyStr, 100))
		return result
	}

	// Непредвиденный ответ
	result.Error = fmt.Sprintf("unexpected_response: %s", truncate(bodyStr, 200))
	result.IsActive = false
	return result
}

// truncate — обрезать строку
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// EnsureVaultDir — создать директорию в vault для OCR-провайдера
func EnsureVaultDir(providerName string) error {
	dir := filepath.Join(vault.VaultPath, "free-api-hunter", providerName)
	return os.MkdirAll(dir, 0700)
}
