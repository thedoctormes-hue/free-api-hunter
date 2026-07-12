package validator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"free-api-hunter/internal/database"
)

// writeFakeKey — создать фейковый vault-файл ключа во временном vault.
func writeFakeKey(t *testing.T, provider, file, content string) {
	t.Helper()
	dir := filepath.Join(vaultBase, provider)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0600); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
}

// writeEndpointMapFile — сериализовать EndpointMap во временный JSON-файл.
func writeEndpointMapFile(t *testing.T, em *EndpointMap) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "validator_endpoints.json")
	data, err := json.Marshal(em)
	if err != nil {
		t.Fatalf("marshal endpoint map: %v", err)
	}
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatalf("write endpoint map: %v", err)
	}
	return p
}

// TestRunIntegrationDryRun — KRV: Run в сухом прогоне (DryRun=true) НЕ лезет в
// сеть, не падает, проходит по всем провайдерам/файлам. Покрывает оркестрацию
// Run (ветка dry-run), loadProviderStatuses (пусто), ListProviderKeyDirs,
// ListKeyFiles, ValidateKey, validateRawSecret (dry-run + unknown).
func TestRunIntegrationDryRun(t *testing.T) {
	vaultBase = t.TempDir()

	writeFakeKey(t, "openrouter", "api.key", "sk-test123\n")
	writeFakeKey(t, "cerebras", "api.keys", "csk-test1\ncsk-test2\ncsk-test3\n")
	// провайдер с неизвестным endpoint (нет в карте) -> live_status unknown.
	writeFakeKey(t, "manus", "api.key", "mk-fake\n")

	em := &EndpointMap{Providers: map[string]EndpointConfig{
		"openrouter": {BaseURL: "http://127.0.0.1:9", AuthType: "bearer", ModelsPath: "/models", Method: "GET", Verified: true},
		"cerebras":   {BaseURL: "http://127.0.0.1:9", AuthType: "bearer", ModelsPath: "/models", Method: "GET", Verified: true},
	}}
	emPath := writeEndpointMapFile(t, em)

	dataDir := t.TempDir()
	if err := Run(Config{DataDir: dataDir, EndpointsPath: emPath, DryRun: true}); err != nil {
		t.Fatalf("Run (dry-run) returned error: %v", err)
	}
	// Dry-run НЕ пишет в БД — проверяем, что таблица пуста (нет записей).
	if err := database.Init(dataDir); err != nil {
		t.Fatalf("db init for check: %v", err)
	}
	defer database.Close()
	var n int
	if err := database.DB().QueryRow(`SELECT COUNT(*) FROM "keys"`).Scan(&n); err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if n != 0 {
		t.Fatalf("dry-run must not write to DB, got %d rows", n)
	}
}

// TestRunIntegrationLive — KRV: Run с реальной записью в SQLite (DryRun=false)
// против httptest-сервера (БЕЗ внешней сети). Покрывает Run (ветка записи),
// UpsertKey, validateRawSecret (probe-ветки bearer/query/header/none),
// liveStatusFromResult, probeSpecFromConfig (все ветки адаптера).
func TestRunIntegrationLive(t *testing.T) {
	vaultBase = t.TempDir()
	srv, _ := newProbeServer(t)

	// openrouter: один bearer-ключ.
	writeFakeKey(t, "openrouter", "api.key", "sk-test123\n")
	// cerebras: мульти-ключ (3 строки) -> 3 KeyRecord.
	writeFakeKey(t, "cerebras", "api.keys", "csk-test1\ncsk-test2\ncsk-test3\n")
	// ocr-space: query-адаптер.
	writeFakeKey(t, "ocr-space", "api.key", "ok-fake\n")
	// elevenlabs: header-адаптер (KRV-E2, header:xi-api-key).
	writeFakeKey(t, "elevenlabs", "api.key", "el-fake\n")
	// pollinations: none-адаптер.
	writeFakeKey(t, "pollinations", "api.key", "pl-fake\n")
	// manus: НЕТ в карте -> endpoint unknown -> live_status=unknown (PAT-005).
	writeFakeKey(t, "manus", "api.key", "mk-fake\n")

	em := &EndpointMap{Providers: map[string]EndpointConfig{
		"openrouter":   {BaseURL: srv, AuthType: "bearer", ModelsPath: "/models", Method: "GET", Verified: true, RegistryName: "OpenRouter"},
		"cerebras":     {BaseURL: srv, AuthType: "bearer", ModelsPath: "/models", Method: "GET", Verified: true, RegistryName: "Cerebras"},
		"ocr-space":    {BaseURL: srv, AuthType: "query", ModelsPath: "", Method: "POST", Verified: false, RegistryName: "OCR.space"},
		"elevenlabs":   {BaseURL: srv, AuthType: "header:xi-api-key", ModelsPath: "/models", Method: "GET", Verified: false, RegistryName: "unknown"},
		"pollinations": {BaseURL: srv, AuthType: "none", ModelsPath: "/models", Method: "GET", Verified: false, RegistryName: "Pollinations"},
	}}
	emPath := writeEndpointMapFile(t, em)

	dataDir := t.TempDir()
	if err := Run(Config{DataDir: dataDir, EndpointsPath: emPath, DryRun: false, AddedBy: "test"}); err != nil {
		t.Fatalf("Run (live) returned error: %v", err)
	}

	// Run закрыл БД через defer — переоткрываем для проверки записанного.
	if err := database.Init(dataDir); err != nil {
		t.Fatalf("db init for check: %v", err)
	}
	defer database.Close()
	db := database.DB()

	// Общее число записей: openrouter 1 + cerebras 3 + ocr-space 1 + elevenlabs 1
	// + pollinations 1 + manus 1 = 8.
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "keys"`).Scan(&total); err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if total != 8 {
		t.Fatalf("expected 8 key records, got %d", total)
	}

	// cerebras мульти-ключ разбит на 3 KeyRecord (key_id …#1/#2/#3).
	var cerebrasCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "keys" WHERE provider='cerebras'`).Scan(&cerebrasCount); err != nil {
		t.Fatalf("count cerebras: %v", err)
	}
	if cerebrasCount != 3 {
		t.Fatalf("expected 3 cerebras records (multi-key split), got %d", cerebrasCount)
	}
	for i := 1; i <= 3; i++ {
		var kid string
		if err := db.QueryRow(`SELECT key_id FROM "keys" WHERE key_id=?`,
			filepath.ToSlash("cerebras/api.keys#"+itoa(i))).Scan(&kid); err != nil {
			t.Fatalf("cerebras record #%d missing: %v", i, err)
		}
	}

	// unknown-провайдер (manus) -> live_status=unknown, не пробивался.
	var manusStatus string
	if err := db.QueryRow(`SELECT live_status FROM "keys" WHERE provider='manus'`).Scan(&manusStatus); err != nil {
		t.Fatalf("manus query: %v", err)
	}
	if manusStatus != "unknown" {
		t.Fatalf("manus live_status = %q, want unknown", manusStatus)
	}

	// header-адаптер (elevenlabs) реально пробился через httptest -> valid.
	var elStatus string
	if err := db.QueryRow(`SELECT live_status FROM "keys" WHERE provider='elevenlabs'`).Scan(&elStatus); err != nil {
		t.Fatalf("elevenlabs query: %v", err)
	}
	if elStatus != "valid" {
		t.Fatalf("elevenlabs live_status = %q, want valid (header adapter probed)", elStatus)
	}

	// openrouter bearer -> valid.
	var orStatus string
	if err := db.QueryRow(`SELECT live_status FROM "keys" WHERE provider='openrouter'`).Scan(&orStatus); err != nil {
		t.Fatalf("openrouter query: %v", err)
	}
	if orStatus != "valid" {
		t.Fatalf("openrouter live_status = %q, want valid", orStatus)
	}
}

func itoa(i int) string {
	return string(rune('0' + i))
}

// TestUpsertKey — прямая запись/чтение KeyRecord во временную SQLite БД.
// Покрывает UpsertKey (INSERT + ON CONFLICT UPDATE) и round-trip через БД.
func TestUpsertKey(t *testing.T) {
	dataDir := t.TempDir()
	if err := database.Init(dataDir); err != nil {
		t.Fatalf("db init: %v", err)
	}
	defer database.Close()
	db := database.DB()

	rec := &KeyRecord{
		Provider:       "openrouter",
		KeyID:          "openrouter/api.key",
		VaultPath:      "/vault/openrouter/api.key",
		RegistryStatus: "verified",
		LiveStatus:     "valid",
		LastValidated:  "2026-07-12T00:00:00Z",
		Models:         []string{"m1", "m2"},
		AuthType:       "bearer",
		BaseURL:        "https://openrouter.ai/api/v1",
		Instructions:   "probe ok",
		AddedBy:        "test",
		AddedAt:        "2026-07-12T00:00:00Z",
	}
	if err := UpsertKey(db, rec); err != nil {
		t.Fatalf("UpsertKey: %v", err)
	}
	// Обновление того же key_id (ON CONFLICT) — меняем live_status.
	rec.LiveStatus = "expired"
	if err := UpsertKey(db, rec); err != nil {
		t.Fatalf("UpsertKey update: %v", err)
	}

	var (
		provider, keyID, live, models, addedBy string
	)
	row := db.QueryRow(`SELECT provider, key_id, live_status, models, added_by FROM "keys" WHERE key_id='openrouter/api.key'`)
	if err := row.Scan(&provider, &keyID, &live, &models, &addedBy); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if provider != "openrouter" || keyID != "openrouter/api.key" {
		t.Fatalf("identity mismatch: %q/%q", provider, keyID)
	}
	if live != "expired" {
		t.Fatalf("ON CONFLICT update not applied: live_status=%q", live)
	}
	if models == "" || addedBy != "test" {
		t.Fatalf("models=%q addedBy=%q (unexpected)", models, addedBy)
	}
}

// TestProbeSpecFromConfig — покрывает ВСЕ ветки probeSpecFromConfig (KRV-E2):
// unknown endpoint, templated base_url, bearer, query, header:NAME, none.
func TestProbeSpecFromConfig(t *testing.T) {
	// unknown auth_type -> адаптер не строится (PAT-005).
	if _, ok := probeSpecFromConfig(EndpointConfig{AuthType: "unknown", BaseURL: "https://x"}); ok {
		t.Fatal("unknown auth_type must not build adapter")
	}
	// пустой base_url -> не строится.
	if _, ok := probeSpecFromConfig(EndpointConfig{AuthType: "bearer", BaseURL: ""}); ok {
		t.Fatal("empty base_url must not build adapter")
	}
	// шаблонированный base_url ({account_id}) -> не строится.
	if _, ok := probeSpecFromConfig(EndpointConfig{AuthType: "bearer", BaseURL: "https://x/{account_id}/v1"}); ok {
		t.Fatal("templated base_url must not build adapter")
	}
	// bearer.
	spec, ok := probeSpecFromConfig(EndpointConfig{AuthType: "bearer", BaseURL: "https://x", ModelsPath: "/models", Method: "GET"})
	if !ok || spec.AuthType != "bearer" || spec.URL != "https://x/models" {
		t.Fatalf("bearer spec wrong: ok=%v spec=%+v", ok, spec)
	}
	// query -> QueryParam по умолчанию "apikey".
	spec, ok = probeSpecFromConfig(EndpointConfig{AuthType: "query", BaseURL: "https://x", ModelsPath: "", Method: "POST"})
	if !ok || spec.AuthType != "query" || spec.QueryParam != "apikey" {
		t.Fatalf("query spec wrong: ok=%v spec=%+v", ok, spec)
	}
	// header:NAME -> AuthHeader=NAME.
	spec, ok = probeSpecFromConfig(EndpointConfig{AuthType: "header:xi-api-key", BaseURL: "https://x", ModelsPath: "/models", Method: "GET"})
	if !ok || spec.AuthType != "header" || spec.AuthHeader != "xi-api-key" {
		t.Fatalf("header:NAME spec wrong: ok=%v spec=%+v", ok, spec)
	}
	// header без имени -> дефолт xi-api-key.
	spec, ok = probeSpecFromConfig(EndpointConfig{AuthType: "header", BaseURL: "https://x", ModelsPath: "/models"})
	if !ok || spec.AuthType != "header" || spec.AuthHeader != "xi-api-key" {
		t.Fatalf("header default spec wrong: ok=%v spec=%+v", ok, spec)
	}
	// none.
	spec, ok = probeSpecFromConfig(EndpointConfig{AuthType: "none", BaseURL: "https://x", ModelsPath: "/models", Method: "GET"})
	if !ok || spec.AuthType != "none" || spec.URL != "https://x/models" {
		t.Fatalf("none spec wrong: ok=%v spec=%+v", ok, spec)
	}
	// неизвестный auth_type -> не строится.
	if _, ok := probeSpecFromConfig(EndpointConfig{AuthType: "weird", BaseURL: "https://x"}); ok {
		t.Fatal("unknown auth_type 'weird' must not build adapter")
	}
}

// TestValidateKeyReadError — KRV: файл ключа недоступен -> rec с read_error,
// live_status=unknown (не падаем).
func TestValidateKeyReadError(t *testing.T) {
	vaultBase = t.TempDir()
	cfg := EndpointConfig{BaseURL: "https://x", AuthType: "bearer", ModelsPath: "/models", Verified: true}
	recs, err := ValidateKey("openrouter", "missing.key", cfg, true, "")
	if err != nil {
		t.Fatalf("ValidateKey read error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].LiveStatus != "unknown" {
		t.Fatalf("live_status = %q, want unknown", recs[0].LiveStatus)
	}
	if recs[0].RegistryStatus != "unknown" {
		t.Fatalf("registry_status = %q, want unknown (orUnknown empty)", recs[0].RegistryStatus)
	}
	if !contains(recs[0].Instructions, "read_error") {
		t.Fatalf("instructions missing read_error: %q", recs[0].Instructions)
	}
}

// TestValidateKeyEmptyFile — KRV: пустой файл ключа -> rec с empty file marker.
func TestValidateKeyEmptyFile(t *testing.T) {
	vaultBase = t.TempDir()
	writeFakeKey(t, "openrouter", "api.key", "")
	cfg := EndpointConfig{BaseURL: "https://x", AuthType: "bearer", ModelsPath: "/models", Verified: true}
	recs, err := ValidateKey("openrouter", "api.key", cfg, true, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey empty file: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].LiveStatus != "unknown" {
		t.Fatalf("live_status = %q, want unknown", recs[0].LiveStatus)
	}
	if !contains(recs[0].Instructions, "empty file") {
		t.Fatalf("instructions missing empty file marker: %q", recs[0].Instructions)
	}
}

// TestValidateKeyProbeAdapterUnsupported — KRV-E2: адаптер не собрался
// (неизвестный auth_type, отличный от буквального "unknown") -> не пробим,
// live_status=unknown, пометка "probe adapter unsupported".
func TestValidateKeyProbeAdapterUnsupported(t *testing.T) {
	vaultBase = t.TempDir()
	writeFakeKey(t, "weirdprovider", "api.key", "w-fake\n")
	cfg := EndpointConfig{BaseURL: "https://x/v1", AuthType: "weird", ModelsPath: "/models", Verified: false}
	recs, err := ValidateKey("weirdprovider", "api.key", cfg, true, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].LiveStatus != "unknown" {
		t.Fatalf("live_status = %q, want unknown (adapter unsupported)", recs[0].LiveStatus)
	}
	if !contains(recs[0].Instructions, "probe adapter unsupported") {
		t.Fatalf("instructions missing 'probe adapter unsupported': %q", recs[0].Instructions)
	}
}

// TestValidateKeyTemplatedEndpoint — KRV: шаблонированный base_url
// ({account_id}) ловится раньше probeSpecFromConfig -> unknown, пометка
// "endpoint unknown/templated".
func TestValidateKeyTemplatedEndpoint(t *testing.T) {
	vaultBase = t.TempDir()
	writeFakeKey(t, "cloudflare", "api.key", "cf-fake\n")
	cfg := EndpointConfig{BaseURL: "https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/v1", AuthType: "bearer", ModelsPath: "/models", Verified: false}
	recs, err := ValidateKey("cloudflare", "api.key", cfg, true, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].LiveStatus != "unknown" {
		t.Fatalf("live_status = %q, want unknown (templated)", recs[0].LiveStatus)
	}
	if !contains(recs[0].Instructions, "endpoint unknown/templated") {
		t.Fatalf("instructions missing 'endpoint unknown/templated': %q", recs[0].Instructions)
	}
}

// TestProbeAdapterHeaderXiApiKey — KRV-E2 (Task 2): auth_type "header:xi-api-key"
// парсится probeSpecFromConfig -> AuthHeader="xi-api-key", и реальная проба
// ставит этот заголовок (проверяем через httptest-сервер).
func TestProbeAdapterHeaderXiApiKey(t *testing.T) {
	vaultBase = t.TempDir()
	srv, cap := newProbeServer(t)
	writeFakeKey(t, "elevenlabs", "api.key", "el-fake\n")

	cfg := EndpointConfig{BaseURL: srv, AuthType: "header:xi-api-key", ModelsPath: "/models", Method: "GET", Verified: false}
	recs, err := ValidateKey("elevenlabs", "api.key", cfg, true, "unknown")
	if err != nil {
		t.Fatalf("ValidateKey header xi-api-key: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].LiveStatus != "valid" {
		t.Fatalf("live_status = %q, want valid", recs[0].LiveStatus)
	}
	if cap.headerKey != "xi-api-key" || cap.headerValue != "el-fake" {
		t.Fatalf("header adapter wrong: key=%q value=%q", cap.headerKey, cap.headerValue)
	}
}

// TestLoadProviderStatuses — покрывает успешный путь loadProviderStatuses
// (чтение providers.json, разбор, цикл заполнения карты) и пустой результат
// при отсутствии файла.
func TestLoadProviderStatuses(t *testing.T) {
	dataDir := t.TempDir()
	pj := filepath.Join(dataDir, "providers.json")
	content := `{"providers":[{"name":"OpenRouter","status":"verified"},{"name":"Cerebras","status":"unverified"}]}`
	if err := os.WriteFile(pj, []byte(content), 0600); err != nil {
		t.Fatalf("write providers.json: %v", err)
	}
	m := loadProviderStatuses(dataDir)
	if m["OpenRouter"] != "verified" {
		t.Fatalf("OpenRouter status = %q, want verified", m["OpenRouter"])
	}
	if m["Cerebras"] != "unverified" {
		t.Fatalf("Cerebras status = %q, want unverified", m["Cerebras"])
	}
	// отсутствие файла -> пустая карта.
	empty := loadProviderStatuses(t.TempDir())
	if len(empty) != 0 {
		t.Fatalf("expected empty map for missing providers.json, got %d", len(empty))
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
