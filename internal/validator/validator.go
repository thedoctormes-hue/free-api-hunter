// Package validator — KRV-пайплайн: живая валидация API-ключей из vault.
//
// Spike-прототип. Итерирует ключи в vault
// (/root/LabDoctorM/vault/free-api-hunter/<provider>/api.key*), для каждого
// делает живую пробу провайдера (переиспользуя verifier.VerifyAPIKeyWithSecret)
// и пишет live_status в SQLite-таблицу "keys".
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
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/models"
	"free-api-hunter/internal/verifier"
)

// vaultBase — корень vault для free-api-hunter (ключи лежат в подкаталогах провайдеров).
// var (а не const), чтобы тесты могли переопределить на t.TempDir().
var vaultBase = "/root/LabDoctorM/vault/free-api-hunter"

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
	Provenance     string // источник instructions: endpoint_map | deep_research | hypothesis | unknown
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

// probeSpecFromConfig — построить per-provider probe-адаптер (KRV-E2) из
// конфигурации endpoint'а. Возвращает ok=false, если адаптер невозможно
// построить (шаблонированный base_url с {account_id}, неизвестный auth_type),
// — тогда вызывающий пишет live_status=unknown (PAT-005, не угадываем).
func probeSpecFromConfig(cfg EndpointConfig) (verifier.ProbeSpec, bool) {
	if cfg.BaseURL == "" || cfg.AuthType == "unknown" {
		return verifier.ProbeSpec{}, false
	}
	// Шаблонированный base_url (напр. cloudflare {account_id}) пока не поддерживаем.
	if strings.Contains(cfg.BaseURL, "{") {
		return verifier.ProbeSpec{}, false
	}
	spec := verifier.ProbeSpec{Method: cfg.Method}
	if spec.Method == "" {
		spec.Method = http.MethodGet
	}
	spec.URL = strings.TrimRight(cfg.BaseURL, "/") + cfg.ModelsPath
	switch {
	case strings.HasPrefix(cfg.AuthType, "bearer"):
		spec.AuthType = "bearer"
	case strings.HasPrefix(cfg.AuthType, "query"):
		spec.AuthType = "query"
		spec.QueryParam = "apikey"
	case strings.HasPrefix(cfg.AuthType, "header"):
		spec.AuthType = "header"
		// имя заголовка может быть задано после ':' (напр. "header:xi-api-key")
		name := strings.TrimSpace(strings.TrimPrefix(cfg.AuthType, "header"))
		name = strings.TrimPrefix(name, ":")
		if name == "" {
			name = "xi-api-key"
		}
		spec.AuthHeader = name
	case cfg.AuthType == "none":
		spec.AuthType = "none"
	default:
		return verifier.ProbeSpec{}, false
	}
	return spec, true
}

// splitKeys — разбить содержимое vault-файла на отдельные ключи (KRV-E3).
// Каждая непустая (после TrimSpace) строка — отдельный ключ. Пустые строки
// (включая завершающий перевод) игнорируются; порядок сохраняется.
func splitKeys(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	raw := strings.Split(normalized, "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// ValidateKey — спроверить ключи из ОДНОГО vault-файла и вернуть записи
// (без записи в БД). KRV-E3: файл может содержать НЕСКОЛЬКО ключей (по строкам).
//   - один ключ  -> key_id = "provider/file" (обратная совместимость)
//   - N ключей   -> key_id = "provider/file#N" (N с 1)
//
// probe=false => сухой прогон, сети нет, live_status="unknown".
// registryStatus — статический статус провайдера из providers.json.
func ValidateKey(provider, keyFile string, cfg EndpointConfig, probe bool, registryStatus string) ([]KeyRecord, error) {
	vaultPath := filepath.Join(vaultBase, provider, keyFile)

	secret, err := os.ReadFile(vaultPath)
	if err != nil {
		rec := validateRawSecret(provider, keyFile, provider+"/"+keyFile, vaultPath, "", cfg, probe, registryStatus)
		rec.LiveStatus = "unknown"
		rec.Instructions = strings.TrimSpace(rec.Instructions + " | read_error: " + err.Error())
		return []KeyRecord{rec}, nil
	}

	// KRV-E3: каждая непустая строка файла — отдельный ключ.
	keys := splitKeys(string(secret))
	if len(keys) == 0 {
		rec := validateRawSecret(provider, keyFile, provider+"/"+keyFile, vaultPath, "", cfg, probe, registryStatus)
		rec.LiveStatus = "unknown"
		rec.Instructions = strings.TrimSpace(rec.Instructions + " | empty file: no keys")
		return []KeyRecord{rec}, nil
	}

	records := make([]KeyRecord, 0, len(keys))
	for i, raw := range keys {
		keyID := provider + "/" + keyFile
		if len(keys) > 1 {
			keyID = fmt.Sprintf("%s#%d", keyID, i+1)
		}
		rec := validateRawSecret(provider, keyFile, keyID, vaultPath, raw, cfg, probe, registryStatus)
		records = append(records, rec)
	}
	return records, nil
}

// validateRawSecret — проверить один сырой секрет (одну строку файла) и
// построить KeyRecord. Выделено из ValidateKey, чтобы не дублировать логику
// для мульти-ключевых файлов (KRV-E3) и не дублировать E1 (используем
// verifier.VerifyAPIKeyWithSecretSpec, а не VerifyRawKey).
func validateRawSecret(provider, keyFile, keyID, vaultPath, rawSecret string, cfg EndpointConfig, probe bool, registryStatus string) KeyRecord {
	rec := KeyRecord{
		Provider:       provider,
		KeyID:          keyID,
		VaultPath:      vaultPath,
		RegistryStatus: orUnknown(registryStatus),
		AuthType:       cfg.AuthType,
		BaseURL:        cfg.BaseURL,
		Instructions:   cfg.Instructions,
		AddedAt:        models.Now(),
	}

	// PAT-005: endpoint неизвестен (или шаблонирован) — не угадываем, не стучимся.
	if cfg.BaseURL == "" || cfg.AuthType == "unknown" || strings.Contains(cfg.BaseURL, "{") {
		rec.LiveStatus = "unknown"
		rec.Instructions = strings.TrimSpace(rec.Instructions + " [endpoint unknown/templated: not probed]")
		return rec
	}

	// Сухой прогон без сети.
	if !probe {
		rec.LiveStatus = "unknown"
		rec.Instructions = strings.TrimSpace(rec.Instructions + " [dry-run: not probed]")
		return rec
	}

	// Строим per-provider адаптер пробы (KRV-E2).
	spec, ok := probeSpecFromConfig(cfg)
	if !ok {
		rec.LiveStatus = "unknown"
		rec.Instructions = strings.TrimSpace(rec.Instructions + " [probe adapter unsupported: not probed]")
		return rec
	}

	// Живая проба КОНКРЕТНОГО секрета (без vault, без default-ключа провайдера).
	// Для bearer переиспользуем E1-функцию main VerifyAPIKeyWithSecret (она уже
	// пробует /models и /v1/models с Bearer) — E1 НЕ дублируем. Для query/header/
	// none используем новый per-provider адаптер VerifyAPIKeyWithSecretSpec (KRV-E2).
	var res *verifier.KeyVerifyResult
	if spec.AuthType == "bearer" {
		res = verifier.VerifyAPIKeyWithSecret(provider, strings.TrimRight(cfg.BaseURL, "/"), rawSecret)
	} else {
		res = verifier.VerifyAPIKeyWithSecretSpec(provider, spec, rawSecret)
	}
	rec.LiveStatus = liveStatusFromResult(res)
	rec.LastValidated = res.CheckedAt
	if res.Error != "" {
		rec.Instructions = strings.TrimSpace(rec.Instructions + " | probe_error: " + res.Error)
	}
	rec.Models = res.Models
	if len(res.Limits) > 0 {
		for k, v := range res.Limits {
			rec.Instructions = strings.TrimSpace(rec.Instructions + fmt.Sprintf(" | limit:%s=%s", k, v))
		}
	}
	return rec
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
	if rec.Provenance == "" {
		rec.Provenance = "unknown"
	}
	modelsJSON, err := json.Marshal(rec.Models)
	if err != nil {
		modelsJSON = []byte("[]")
	}
	_, err = db.Exec(`
		INSERT INTO "keys" (
			provider, key_id, vault_path, registry_status, live_status,
			last_validated, models, auth_type, base_url, instructions, added_by, added_at,
			provenance
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
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
			added_at=excluded.added_at,
			provenance=excluded.provenance
	`, rec.Provider, rec.KeyID, rec.VaultPath, rec.RegistryStatus, rec.LiveStatus,
		rec.LastValidated, string(modelsJSON), rec.AuthType, rec.BaseURL, rec.Instructions,
		rec.AddedBy, rec.AddedAt, rec.Provenance)
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
			status := statuses[ecfg.RegistryName]
			if status == "" {
				status = "unknown"
			}
			recs, err := ValidateKey(p, f, ecfg, !cfg.DryRun, status)
			if err != nil {
				logger.Printf("error %s/%s: %v", p, f, err)
				unknown++
				continue
			}
			for _, rec := range recs {
				total++
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
					if uerr := UpsertKey(database.DB(), &rec); uerr != nil {
						logger.Printf("upsert failed %s: %v", rec.KeyID, uerr)
					} else {
						logger.Printf("[ok] %s -> live=%s", rec.KeyID, rec.LiveStatus)
					}
				}
			}
		}
	}

	logger.Printf("summary: keys=%d valid=%d expired=%d rate_limited=%d unknown=%d (dry-run=%v)",
		total, valid, expired, rateLimited, unknown, cfg.DryRun)
	return nil
}
