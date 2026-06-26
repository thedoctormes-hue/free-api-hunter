package tts

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/vault"
)

var logger = log.New(log.Writer(), "[tts-verifier] ", log.LstdFlags)

// HTTPClient — настраиваемый HTTP клиент
var HTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// VerifyTTSKey — проверить ключ TTS-провайдера
// Делает последовательные запросы:
// 1. GET /v1/user/subscription — проверка ключа и получение плана
// 2. GET /v1/voices — список доступных голосов
func VerifyTTSKey(provider *models.TTSProvider) *models.TTSVerifyResult {
	result := &models.TTSVerifyResult{
		CheckedAt: models.Now(),
	}

	// Получаем реальный ключ из vault
	realKey, err := getVaultKey(provider.Name)
	if err != nil {
		// Ключ ещё не в vault — пробуем использовать имя провайдера
		// (может быть переменная окружения или ключ передан иначе)
		result.Error = "vault_key_not_found"

		// Fallback: пробуем запрос без ключа
		// Для ElevenLabs — /v1/voices без ключа вернёт 401
		// Для других провайдеров может работать
		checkURL := provider.URL + "/v1/voices"
		req, _ := http.NewRequest("GET", checkURL, nil)
		req.Header.Set("User-Agent", "FreeAPIHunter-TTS/0.1")
		resp, err := HTTPClient.Do(req)
		if err != nil {
			result.Error = "no_key_and_request_failed: " + err.Error()
			return result
		}
		result.StatusCode = resp.StatusCode
		resp.Body.Close()
		if resp.StatusCode == 401 {
			result.Error = "no_valid_key"
		}
		return result
	}

	// Шаг 1: Проверяем subscription
	subURL := strings.TrimRight(provider.URL, "/") + "/v1/user/subscription"
	subResult, err := checkSubscription(subURL, realKey)
	if err != nil {
		result.Error = fmt.Sprintf("subscription_check_failed: %v", err)
		return result
	}

	result.StatusCode = subResult.StatusCode
	result.Plan = subResult.Plan
	result.CharLimit = subResult.CharLimit
	result.IsActive = subResult.IsActive

	if !result.IsActive {
		result.Error = subResult.Error
		return result
	}

	// Шаг 2: Получаем список голосов
	voicesURL := strings.TrimRight(provider.URL, "/") + "/v1/voices"
	voices, err := getVoices(voicesURL, realKey)
	if err == nil {
		result.Voices = voices
	}

	// Шаг 3: Пробуем TTS с коротким текстом (только если ключ активен)
	if result.IsActive {
		ttsURL := strings.TrimRight(provider.URL, "/") + "/v1/text-to-speech"
		// Используем первый доступный голос
		if len(result.Voices) > 0 {
			ttsURL += "/" + result.Voices[0]
		}
		ttsOK, ttsCode, ttsErr := testTTS(ttsURL, realKey)
		if !ttsOK {
			result.Error = fmt.Sprintf("tts_test_failed: HTTP %d: %v", ttsCode, ttsErr)
		}
	}

	return result
}

// subResult — результат проверки подписки
type subResult struct {
	StatusCode int
	Plan       string
	CharLimit  int
	IsActive   bool
	Error      string
}

// checkSubscription — GET /v1/user/subscription для ElevenLabs
func checkSubscription(url, apiKey string) (*subResult, error) {
	result := &subResult{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("User-Agent", "FreeAPIHunter-TTS/0.1")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		return result, nil
	}

	// Парсим ответ ElevenLabs
	var subResp struct {
		Detail struct {
			Status string `json:"status"`
		} `json:"detail,omitempty"`
		Tier   string `json:"tier,omitempty"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	// Для ElevenLabs ответ может содержать ошибку
	if json.Unmarshal(body, &subResp) == nil && subResp.Detail.Status == "" {
		result.IsActive = true
		result.Plan = subResp.Tier
		// CharLimit зависит от плана
		switch subResp.Tier {
		case "free":
			result.CharLimit = 10000
		case "starter":
			result.CharLimit = 30000
		case "creator":
			result.CharLimit = 121000
		}
	} else {
		// Не ElevenLabs — просто проверяем что 200 OK
		result.IsActive = true
		result.Plan = "unknown"
	}

	return result, nil
}

// getVoices — GET /v1/voices для ElevenLabs
func getVoices(url, apiKey string) ([]string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("User-Agent", "FreeAPIHunter-TTS/0.1")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var voicesResp struct {
		Voices []struct {
			Name    string `json:"name"`
			VoiceID string `json:"voice_id"`
		} `json:"voices"`
	}

	if err := json.Unmarshal(body, &voicesResp); err != nil {
		return nil, err
	}

	var voices []string
	for _, v := range voicesResp.Voices {
		voices = append(voices, v.Name)
	}

	return voices, nil
}

// testTTS — POST /v1/text-to-speech/{voice_id}
func testTTS(url, apiKey string) (bool, int, error) {
	payload := map[string]interface{}{
		"text":     "API test",
		"model_id": "eleven_multilingual_v2",
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FreeAPIHunter-TTS/0.1")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// Проверяем что получили аудио (не HTML/JSON ошибку)
		contentType := resp.Header.Get("Content-Type")
		return strings.Contains(contentType, "audio") || strings.Contains(contentType, "octet-stream"), resp.StatusCode, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return false, resp.StatusCode, fmt.Errorf("TTS failed %d: %s", resp.StatusCode, truncate(string(respBody), 200))
}

// getVaultKey — получить ключ из vault
func getVaultKey(providerName string) (string, error) {
	providerPath := "free-api-hunter/" + normalizeName(providerName)
	return vault.GetDefaultKey(providerPath)
}

// normalizeName — нормализовать имя провайдера для пути vault
func normalizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "(", "")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, "/", "")
	return name
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
