package verifier

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"free-api-hunter/internal/models"
	"free-api-hunter/internal/orex"
	"free-api-hunter/internal/securego"
	"free-api-hunter/internal/vault"
)

// ensure orex import is used
var _ = orex.FreeModel{}

var logger = log.New(log.Writer(), "[verifier] ", log.LstdFlags)

// ValidateOutboundURL is a variable for overriding IsValidOutboundURL in tests.
// Defaults to the real implementation.
var ValidateOutboundURL = securego.IsValidOutboundURL

// HTTPClient — настраиваемый HTTP клиент
var HTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// CheckURLAive — проверить что URL отвечает 200
func CheckURLAive(rawURL string) bool {
	_, err := ValidateOutboundURL(rawURL)
	if err != nil {
		logger.Printf("CheckURLAive: URL rejected: %v", err)
		return false
	}

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

	// Валидация URL для SSRF protection
	_, err := ValidateOutboundURL(provider.URL)
	if err != nil {
		logger.Printf("VerifyProvider: URL rejected for %s: %v", provider.Name, err)
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
	Provider   string            `json:"provider,omitempty"` // кто верифицировался (для удобства Validator)
	IsActive   bool              `json:"is_active"`
	StatusCode int               `json:"status_code"`
	Error      string            `json:"error,omitempty"`
	Models     []string          `json:"models"`
	Limits     map[string]string `json:"limits"`
	CheckedAt  string            `json:"checked_at"`
}

// resolveKey — честно резолвит реальный секрет для APIKey:
//   - KeyLocation с префиксом sk-/gsk-/csk-/cfut_ — это сам секрет, используем как есть
//   - KeyLocation — абсолютный путь внутри vault — читаем КОНКРЕТНЫЙ файл (GetKeyByPath)
//   - иначе (относительный/неизвестный location) — legacy fallback на дефолтный ключ провайдера
//
// Раньше ветка «не префикс» всегда звала vault.GetDefaultKey, то есть брала ПЕРВЫЙ
// файл провайдера, а не тот, что нужен. Теперь конкретный путь читается точно.
func resolveKey(key *models.APIKey) (string, error) {
	loc := key.KeyLocation
	if strings.HasPrefix(loc, "sk-") || strings.HasPrefix(loc, "gsk_") ||
		strings.HasPrefix(loc, "csk-") || strings.HasPrefix(loc, "cfut_") {
		return loc, nil
	}
	if filepath.IsAbs(loc) {
		secret, err := vault.GetKeyByPath(loc)
		if err != nil {
			return "", err
		}
		return secret, nil
	}
	// Legacy: относительный/неизвестный location — дефолтный ключ провайдера.
	return vault.GetDefaultKey(key.ProviderName)
}

// probeKey — живая проба конкретного секрета (GET /models + Bearer).
// НЕ трогает vault и НЕ мутирует models.APIKey. Общий хелпер для
// VerifyAPIKey и VerifyAPIKeyWithSecret, чтобы не дублировать логику пробы.
func probeKey(endpoint, secret string) *KeyVerifyResult {
	result := &KeyVerifyResult{
		CheckedAt: models.Now(),
		Limits:    make(map[string]string),
	}

	// Validate endpoint URL (SSRF protection). В тестах переопределяется.
	_, valErr := ValidateOutboundURL(endpoint)
	if valErr != nil {
		result.Error = "invalid_endpoint: " + valErr.Error()
		return result
	}

	testURLs := []string{
		strings.TrimRight(endpoint, "/") + "/models",
		strings.TrimRight(endpoint, "/") + "/v1/models",
	}

	for _, testURL := range testURLs {
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+secret)
		req.Header.Set("User-Agent", "FreeAPIHunter/0.1")

		resp, err := HTTPClient.Do(req)
		if err != nil {
			result.Error = err.Error()
			continue
		}
		func() {
			defer resp.Body.Close()
			result.StatusCode = resp.StatusCode

			if resp.StatusCode == 200 {
				result.IsActive = true
				body, err := io.ReadAll(resp.Body)
				if err == nil {
					parseModels(body, result)
				}
			} else if resp.StatusCode == 401 {
				result.Error = "invalid_key"
			} else if resp.StatusCode == 403 {
				result.Error = "forbidden"
			} else if resp.StatusCode == 429 {
				result.Error = "rate_limited"
			} else {
				result.Error = fmt.Sprintf("http_%d", resp.StatusCode)
			}
		}()

		if result.IsActive {
			break
		}
	}

	return result
}

// parseModels — извлечь список моделей из ответа /models.
func parseModels(body []byte, result *KeyVerifyResult) {
	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		return
	}
	if d, ok := data["data"].([]interface{}); ok {
		for _, m := range d {
			if mi, ok := m.(map[string]interface{}); ok {
				if id, ok := mi["id"].(string); ok {
					result.Models = append(result.Models, id)
				} else if modelID, ok := mi["model_id"].(string); ok {
					result.Models = append(result.Models, modelID)
				}
			}
		}
	}
}

// VerifyAPIKey — проверить что API ключ рабочий (ключ берётся из vault
// или из самого KeyLocation, см. resolveKey).
func VerifyAPIKey(key *models.APIKey) *KeyVerifyResult {
	realKey, err := resolveKey(key)
	if err != nil {
		result := &KeyVerifyResult{
			Provider:  key.ProviderName,
			CheckedAt: models.Now(),
			Limits:    make(map[string]string),
		}
		result.Error = "vault_error: " + err.Error()
		return result
	}

	result := probeKey(key.Endpoint, realKey)
	result.Provider = key.ProviderName

	key.IsActive = result.IsActive
	key.LastChecked = &result.CheckedAt
	if len(result.Models) > 0 {
		key.Models = result.Models
	}

	return result
}

// VerifyAPIKeyWithSecret — живая проба КОНКРЕТНОГО секрета провайдера,
// без обращения к vault. Полезно, когда секрет уже известен (например,
// Validator получил его из конкретного vault-файла или пробросил напрямую).
//
// ПРИМЕЧАНИЕ: живая проба GET /models требует целевой endpoint, а в кодовой
// базе нет реестра provider→endpoint (Endpoint хранится на models.APIKey и в
// БД пула ключей). Поэтому по сравнению с запрошенной сигнатурой (provider, secret)
// добавлен обязательный параметр endpoint — иначе проба невозможна. Validator
// уже держит APIKey.Endpoint и передаёт его.
func VerifyAPIKeyWithSecret(provider, endpoint, secret string) *KeyVerifyResult {
	result := probeKey(endpoint, secret)
	result.Provider = provider
	return result
}

// ExtractKeyInfo — извлечь информацию с рабочего ключа (ключ берётся из vault)
func ExtractKeyInfo(key *models.APIKey) map[string]interface{} {
	info := map[string]interface{}{
		"provider": key.ProviderName,
		"endpoint": key.Endpoint,
		"models":   []string{},
		"limits":   map[string]string{},
		"contexts": map[string]int{},
		"pricing":  map[string]string{},
	}

	// Получаем реальный ключ из vault
	realKey := key.KeyLocation
	if !strings.HasPrefix(realKey, "sk-") && !strings.HasPrefix(realKey, "gsk_") &&
		!strings.HasPrefix(realKey, "csk-") && !strings.HasPrefix(realKey, "cfut_") {
		var err error
		realKey, err = vault.GetDefaultKey(key.ProviderName)
		if err != nil {
			logger.Printf("ExtractKeyInfo: vault error for %s: %v", key.ProviderName, err)
			return info
		}
	}

	// Validate endpoint URL
	_, valErr := ValidateOutboundURL(key.Endpoint)
	if valErr != nil {
		logger.Printf("ExtractKeyInfo: invalid endpoint for %s: %v", key.ProviderName, valErr)
		return info
	}

	testURL := strings.TrimRight(key.Endpoint, "/") + "/models"
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return info
	}
	req.Header.Set("Authorization", "Bearer "+realKey)
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
				} else if modelID, ok := mi["model_id"].(string); ok {
					models = append(models, modelID)
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
