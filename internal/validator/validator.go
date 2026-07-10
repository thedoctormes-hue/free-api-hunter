// Package validator — KRV-пайплайн: живая валидация API-ключей из vault.
//
// Spike-прототип. Итерирует ключи в vault
// (/root/LabDoctorM/vault/free-api-hunter/<provider>/api.key*), для каждого
// делает живую пробу провайдера (переиспользуя verifier.VerifyAPIKey /
// verifier.ExtractKeyInfo) и пишет live_status в SQLite-таблицу "keys".
//
// live_status ∈ {valid, expired, rate_limited, unknown} по StatusCode:
//   200 -> valid, 401/403 -> expired, 429 -> rate_limited, else -> unknown.
//
// PAT-005: если endpoint провайдера неизвестен — пишем unknown, не угадываем.
package validator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/verifier"
)

// vaultBase — корень vault для free-api-hunter (ключи лежат в подкаталогах провайдеров).
const vaultBase = "/root/LabDoctorM/vault/free-api-hunter"

var logger = log.New(os.Stderr, "[validator] ", log.LstdFlags)

// EndpointConfig — как живо проверить провайдера.
// verified:true  => endpoint подтверждён оператором (доверяем live_status).
// verified:false => ГИПОТЕЗА, требует подтверждения перед доверием к live_status.
type EndpointConfig struct {
	BaseURL      string `json:"base_url"`      // API base URL, пусто => unknown
	AuthType     string `json:"auth_type"`     // bearer | query | unknown
	ModelsPath   string `json:"models_path"`   // путь к /models (может быть "")
	Method       string `json:"method"`        // GET | POST
	Verified     bool   `json:"verified"`
	RegistryName string `json:"registry_name"` // имя провайдера в providers.json
	Instructions string `json:"instructions"`  // человекочитаемая подсказка
}

// EndpointMap — dir-имя провайдера в vault -> конфиг.
type EndpointMap struct {
	Providers map[string]EndpointConfig `json:"providers"`
}

// LoadEndpointMap — загрузить карту endpoint'ов (config/validator_endpoints.json).
// Отсутствие файла не фатально: все ключи пойдут как unknown.
func LoadEndpointMap(path string) (*EndpointMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m EndpointMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Providers == nil {
		m.Providers = map[string]EndpointConfig{}
	}
	return &m, nil
}

// ListProviderKeyDirs — список подкаталогов-провайдеров в vault/free-api-hunter.
func ListProviderKeyDirs() ([]string, error) {
	entries, err := os.ReadDir(vaultBase)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// ListKeyFiles — ВСЕ файлы-ключи провайдера (не только api.key*).
// Важно: vault неконсистентен — встречаются api.keys, api_key_primary.key и т.д.
func ListKeyFiles(provider string) ([]string, error) {
	dir := filepath.Join(vaultBase, provider)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out = append(out, e.Name())
	}
	return out, nil
}

// KeyRecord — одна строка таблицы "keys".
type KeyRecord struct {
	Provider       string
	KeyID          string // provider + "/" + filename (PRIMARY KEY)
	VaultPath      string
	RegistryStatus string // статический статус из providers.json
	LiveStatus     string // valid|expired|rate_limited|unknown
	LastValidated  string // ISO
	Models         []string
	AuthType       string
	BaseURL        string
	Instructions   string
	AddedBy        string
	AddedAt        string
}

// liveStatusFromResult — маппинг результата верификатора в live_status.
func liveStatusFromResult(res *verifier.KeyVerifyResult) string {
	switch res.StatusCode {
	case 200:
		return "valid"
	case 401, 403:
		return "expired"
	case 429:
		return "rate_limited"
	default:
		return "unknown"
	}
}

// ValidateKey — спроверить один ключ из vault и вернуть запись (без записи в БД).
// probe=false => сухой прогон, сети нет, live_status="unknown".
// registryStatus — статический статус провайдера из providers.json.
func ValidateKey(provider, keyFile string, cfg EndpointConfig, probe bool, registryStatus string) (*KeyRecord, error) {
	vaultPath := filepath.Join(vaultBase, provider, keyFile)
	keyID := provider + "/" + keyFile

	rec := &KeyRecord{
		Provider:       provider,
		KeyID:          keyID,
		VaultPath:      vaultPath,
		RegistryStatus: orUnknown(registryStatus),
		AuthType:       cfg.AuthType,
		BaseURL:        cfg.BaseURL,
		Instructions:   cfg.Instructions,
		AddedAt:        models.Now(),
	}

	// PAT-005: endpoint неизвестен — не угадываем, не стучимся.
	if cfg.BaseURL == "" || cfg.AuthType == "unknown" {
		rec.LiveStatus = "unknown"
		rec.Instructions = strings.TrimSpace(rec.Instructions + " [endpoint unknown: not probed]")
		return rec, nil
	}

	// Сухой прогон без сети.
	if !probe {
		rec.LiveStatus = "unknown"
		rec.Instructions = strings.TrimSpace(rec.Instructions + " [dry-run: not probed]")
		return rec, nil
	}

	// Читаем КОНКРЕТНЫЙ файл ключа и шлём живую пробу именно ЭТИМ секретом.
	// VerifyAPIKeyWithSecret проверяет переданный secret напрямую (без vault,
	// без default-ключа провайдера) — тем самым валидируется этот файл vault.
	secret, err := os.ReadFile(vaultPath)
	if err != nil {
		rec.LiveStatus = "unknown"
		rec.Instructions = "read_error: " + err.Error()
		return rec, nil
	}
	realKey := strings.TrimSpace(string(secret))
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + cfg.ModelsPath

	// Живая проба КОНКРЕТНОГО секрета из этого файла (PAT-005: не default-ключ).
	res := verifier.VerifyAPIKeyWithSecret(provider, endpoint, realKey)
	rec.LiveStatus = liveStatusFromResult(res)
	rec.LastValidated = res.CheckedAt
	if res.Error != "" {
		rec.Instructions = strings.TrimSpace(rec.Instructions + " | probe_error: " + res.Error)
	}

	// Модели приходят прямо из пробы; копируем также лимиты в инструкции.
	rec.Models = res.Models
	if len(res.Limits) > 0 {
		for k, v := range res.Limits {
			rec.Instructions = strings.TrimSpace(rec.Instructions + fmt.Sprintf(" | limit:%s=%s", k, v))
		}
	}
	return rec, nil
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// loadProviderStatuses — статический status провайдеров из providers.json
// (registry_status в терминах ADR-KRV). Ключ — имя провайдера.
func loadProviderStatuses(dataDir string) map[string]string {
	m := map[string]string{}
	path := filepath.Join(dataDir, "providers.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	var doc struct {
		Providers []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return m
	}
	for _, p := range doc.Providers {
		m[p.Name] = p.Status
	}
	return m
}

// UpsertKey — вставить/обновить запись в таблице "keys".
func UpsertKey(db *sql.DB, rec *KeyRecord) error {
	modelsJSON, err := json.Marshal(rec.Models)
	if err != nil {
		modelsJSON = []byte("[]")
	}
	_, err = db.Exec(`
		INSERT INTO "keys" (
			provider, key_id, vault_path, registry_status, live_status,
			last_validated, models, auth_type, base_url, instructions, added_by, added_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(key_id) DO UPDATE SET
			provider=excluded.provider,
			vault_path=excluded.vault_path,
			registry_status=excluded.registry_status,
			live_status=excluded.live_status,
			last_validated=excluded.last_validated,
			models=excluded.models,
			auth_type=excluded.auth_type,
			base_url=excluded.base_url,
			instructions=excluded.instructions,
			added_at=excluded.added_at
	`, rec.Provider, rec.KeyID, rec.VaultPath, rec.RegistryStatus, rec.LiveStatus,
		rec.LastValidated, string(modelsJSON), rec.AuthType, rec.BaseURL, rec.Instructions,
		rec.AddedBy, rec.AddedAt)
	return err
}

// Config — параметры прогона валидатора.
type Config struct {
	DataDir       string
	EndpointsPath string
	DryRun        bool
	AddedBy       string
}

// Run — основной цикл: перебрать провайдеров/ключи, проверить, записать.
func Run(cfg Config) error {
	if cfg.AddedBy == "" {
		cfg.AddedBy = "krv-validator"
	}

	em, err := LoadEndpointMap(cfg.EndpointsPath)
	if err != nil {
		logger.Printf("endpoint map load failed (%v); using empty map (all unknown)", err)
		em = &EndpointMap{Providers: map[string]EndpointConfig{}}
	}

	// Инициализируем БД (создаст таблицу "keys" через миграции).
	if err := database.Init(cfg.DataDir); err != nil {
		return fmt.Errorf("db init: %w", err)
	}
	defer database.Close()

	// Справочник статических статусов провайдеров (registry_status).
	statuses := loadProviderStatuses(cfg.DataDir)

	providers, err := ListProviderKeyDirs()
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}

	total, valid, expired, rateLimited, unknown := 0, 0, 0, 0, 0
	for _, p := range providers {
		ecfg, ok := em.Providers[p]
		if !ok {
			ecfg = EndpointConfig{AuthType: "unknown", RegistryName: "unknown"}
		}
		files, err := ListKeyFiles(p)
		if err != nil {
			logger.Printf("skip %s: %v", p, err)
			continue
		}
		for _, f := range files {
			total++
			status := statuses[ecfg.RegistryName]
			if status == "" {
				status = "unknown"
			}
			rec, err := ValidateKey(p, f, ecfg, !cfg.DryRun, status)
			if err != nil {
				logger.Printf("error %s/%s: %v", p, f, err)
				unknown++
				continue
			}
			rec.AddedBy = cfg.AddedBy
			switch rec.LiveStatus {
			case "valid":
				valid++
			case "expired":
				expired++
			case "rate_limited":
				rateLimited++
			default:
				unknown++
			}
			if cfg.DryRun {
				logger.Printf("[dry-run] %s -> live=%s registry=%s models=%d",
					rec.KeyID, rec.LiveStatus, rec.RegistryStatus, len(rec.Models))
			} else {
				if err := UpsertKey(database.DB(), rec); err != nil {
					logger.Printf("upsert failed %s: %v", rec.KeyID, err)
				} else {
					logger.Printf("[ok] %s -> live=%s", rec.KeyID, rec.LiveStatus)
				}
			}
		}
	}

	logger.Printf("summary: keys=%d valid=%d expired=%d rate_limited=%d unknown=%d (dry-run=%v)",
		total, valid, expired, rateLimited, unknown, cfg.DryRun)
	return nil
}
