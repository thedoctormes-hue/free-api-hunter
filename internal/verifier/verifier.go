package verifier

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/orex"
	"free-api-hunter/internal/vault"
)

// ensure orex import is used
var _ = orex.FreeModel{}

var logger = log.New(log.Writer(), "[verifier] ", log.LstdFlags)

// HTTPClient — настраиваемый HTTP клиент
var HTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// CheckURLAive — проверить что URL отвечает 200
func CheckURLAive(rawURL string) bool {
	req, err := http.NewRequest("HEAD", rawURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "FreeAPIHunter/0.1")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// VerifyResult — результат верификации провайдера
type VerifyResult struct {
	URLAlive          bool     `json:"url_alive"`
	FreeTierMentioned bool     `json:"free_tier_mentioned"`
	CreditCardReq     *bool    `json:"credit_card_required,omitempty"`
	ModelsFound       []string `json:"models_found"`
	LimitsFound       string   `json:"limits_found"`
	CheckedAt         string   `json:"checked_at"`
}

// VerifyProviderPage — проверить страницу провайдера
func VerifyProviderPage(provider *models.Provider) *VerifyResult {
	result := &VerifyResult{
		CheckedAt: models.Now(),
	}

	result.URLAlive = CheckURLAive(provider.URL)
	if !result.URLAlive {
		logger.Printf("VerifyProvider: %s URL dead", provider.Name)
		return result
	}

	// Загружаем страницу
	req, err := http.NewRequest("GET", provider.URL, nil)
	if err != nil {
		return result
	}
	req.Header.Set("User-Agent", "FreeAPIHunter/0.1")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result
	}
	text := strings.ToLower(string(body))

	// Проверяем упоминание бесплатного тира
	freeKeywords := []string{"free tier", "free plan", "free credit", "no credit card",
		"without credit card", "бесплатн", "без карты"}
	for _, kw := range freeKeywords {
		if strings.Contains(text, kw) {
			result.FreeTierMentioned = true
			break
		}
	}

	// Проверяем требование карты
	// Сначала проверяем явные отрицания ("no credit card", "without credit card")
	noCardPatterns := []string{"no credit card", "without credit card", "no payment",
		"без карты", "без оплаты"}
	hasNoCard := false
	for _, p := range noCardPatterns {
		if strings.Contains(text, p) {
			hasNoCard = true
			break
		}
	}

	cardFound := false
	if !hasNoCard {
		cardKeywords := []string{"credit card required", "add payment method", "billing info",
			"кредитная карта", "способ оплаты", "enter your card"}
		for _, kw := range cardKeywords {
			if strings.Contains(text, kw) {
				cardFound = true
				break
			}
		}
	}
	result.CreditCardReq = &cardFound

	return result
}

// KeyVerifyResult — результат проверки API ключа
type KeyVerifyResult struct {
	IsActive    bool     `json:"is_active"`
	StatusCode  int      `json:"status_code"`
	Error       string   `json:"error,omitempty"`
	Models      []string `json:"models"`
	Limits      map[string]string `json:"limits"`
	CheckedAt   string   `json:"checked_at"`
}

// VerifyAPIKey — проверить что API ключ рабочий (ключ берётся из vault)
func VerifyAPIKey(key *models.APIKey) *KeyVerifyResult {
	result := &KeyVerifyResult{
		CheckedAt: models.Now(),
		Limits:    make(map[string]string),
	}

	// Получаем реальный ключ из vault
	realKey := key.KeyLocation
	if !strings.HasPrefix(realKey, "sk-") && !strings.HasPrefix(realKey, "gsk_") &&
		!strings.HasPrefix(realKey, "csk-") && !strings.HasPrefix(realKey, "cfut_") {
		// Это путь в vault — читаем
		var err error
		realKey, err = vault.GetDefaultKey(key.ProviderName)
		if err != nil {
			result.Error = "vault_error: " + err.Error()
			return result
		}
	}

	testURLs := []string{
		strings.TrimRight(key.Endpoint, "/") + "/models",
		strings.TrimRight(key.Endpoint, "/") + "/v1/models",
	}

	for _, testURL := range testURLs {
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+realKey)
		req.Header.Set("User-Agent", "FreeAPIHunter/0.1")

		resp, err := HTTPClient.Do(req)
		if err != nil {
			result.Error = err.Error()
			continue
		}
		defer resp.Body.Close()

		result.StatusCode = resp.StatusCode

		if resp.StatusCode == 200 {
			result.IsActive = true
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				var data map[string]interface{}
				if json.Unmarshal(body, &data) == nil {
					if d, ok := data["data"].([]interface{}); ok {
						for _, m := range d {
							if mi, ok := m.(map[string]interface{}); ok {
								if id, ok := mi["id"].(string); ok {
									result.Models = append(result.Models, id)
								}
							}
						}
					}
				}
			}
			break
		} else if resp.StatusCode == 401 {
			result.Error = "invalid_key"
		} else if resp.StatusCode == 403 {
			result.Error = "forbidden"
		} else if resp.StatusCode == 429 {
			result.Error = "rate_limited"
		} else {
			result.Error = fmt.Sprintf("http_%d", resp.StatusCode)
		}
	}

	key.IsActive = result.IsActive
	key.LastChecked = &result.CheckedAt
	if len(result.Models) > 0 {
		key.Models = result.Models
	}

	return result
}

// ExtractKeyInfo — извлечь информацию с рабочего ключа
func ExtractKeyInfo(key *models.APIKey) map[string]interface{} {
	info := map[string]interface{}{
		"provider":  key.ProviderName,
		"endpoint":  key.Endpoint,
		"models":    []string{},
		"limits":    map[string]string{},
		"contexts":  map[string]int{},
		"pricing":   map[string]string{},
	}

	testURL := strings.TrimRight(key.Endpoint, "/") + "/models"
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return info
	}
	req.Header.Set("Authorization", "Bearer "+key.KeyLocation)
	req.Header.Set("User-Agent", "FreeAPIHunter/0.1")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return info
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return info
	}

	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		return info
	}

	models := []string{}
	contexts := map[string]int{}
	pricing := map[string]string{}

	if d, ok := data["data"].([]interface{}); ok {
		for _, m := range d {
			if mi, ok := m.(map[string]interface{}); ok {
				if id, ok := mi["id"].(string); ok {
					models = append(models, id)
				}
				if ctx, ok := mi["context_length"].(float64); ok {
					if id, ok := mi["id"].(string); ok {
						contexts[id] = int(ctx)
					}
				}
				if p, ok := mi["pricing"].(string); ok {
					if id, ok := mi["id"].(string); ok {
						pricing[id] = p
					}
				}
			}
		}
	}

	info["models"] = models
	info["contexts"] = contexts
	info["pricing"] = pricing

	return info
}
