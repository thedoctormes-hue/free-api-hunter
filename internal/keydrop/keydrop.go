// Package keydrop — Фаза 3' KRV: drop-zone ключей на Яндекс Диске.
//
// Человек (IRL) кладёт один .md-файл на провайдера в папку на Яндекс Диске.
// Файл может содержать СКОЛЬКО УГОДНО ключей (любым видом) + любую
// «доп информацию». Агент опрашивает папку, гибко парсит .md, зеркалит
// ключи в локальный vault, верифицирует (validator) и регистрирует в
// Registry (таблица keys), после чего удаляет .md с Диска.
package keydrop

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"encoding/json"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/validator"
)

var logger = log.New(os.Stderr, "[keydrop] ", log.LstdFlags)

// vaultBase — корень vault для free-api-hunter (совпадает с validator.vaultBase).
// Это var (не const), чтобы тесты могли переопределить.
var vaultBase = "/root/LabDoctorM/vault/free-api-hunter"

const yandexRate = 31 * time.Second // Yandex: <=1 запрос / 30с
const defaultYandexBin = "/root/LabDoctorM/projects/DoctorM_and_Ai/bin/yandex.sh"
const defaultDiskDir = "free-api-hunter/keys"
const defaultEndpoints = "/root/LabDoctorM/projects/free-api-hunter/config/validator_endpoints.json"

// Options — параметры прогона keydrop.
type Options struct {
	DataDir       string // каталог с free-api-hunter.db (SQLite)
	DiskDir       string // папка на Яндекс Диске (отн. корня Диска)
	YandexBin     string // путь к yandex.sh
	EndpointsPath string // карта endpoint'ов для верификации
	AddedBy              string // кто добавил (аудит)
	Keep                 bool   // не удалять .md с Диска после обработки
	PendingValidationPath string // куда писать сигнал для агентской валидации
}

// Run — опросить папку на Диске, распарсить .md, зеркалить ключи в vault,
// верифицировать и зарегистрировать в Registry, удалить .md с Диска.
func Run(opts Options) error {
	if opts.YandexBin == "" {
		opts.YandexBin = defaultYandexBin
	}
	if opts.DiskDir == "" {
		opts.DiskDir = defaultDiskDir
	}
	if opts.EndpointsPath == "" {
		opts.EndpointsPath = defaultEndpoints
	}
	if opts.AddedBy == "" {
		opts.AddedBy = "keydrop"
	}
	if opts.PendingValidationPath == "" {
		opts.PendingValidationPath = filepath.Join(opts.DataDir, "pending_validation.json")
	}

	if err := database.Init(opts.DataDir); err != nil {
		return fmt.Errorf("db init: %w", err)
	}
	defer database.Close()
	db := database.DB()

	em, err := validator.LoadEndpointMap(opts.EndpointsPath)
	if err != nil {
		logger.Printf("endpoint map load failed (%v); using empty (all unknown)", err)
		em = &validator.EndpointMap{Providers: map[string]validator.EndpointConfig{}}
	}

	files, err := yandexDiskLs(opts.YandexBin, opts.DiskDir)
	if err != nil {
		return fmt.Errorf("disk ls %s: %w", opts.DiskDir, err)
	}

	processed := 0
	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f), ".md") {
			continue
		}
		remote := strings.TrimRight(opts.DiskDir, "/") + "/" + f
		local, gerr := yandexDiskGet(opts.YandexBin, remote)
		if gerr != nil {
			logger.Printf("skip %s: get failed: %v", f, gerr)
			continue
		}
		content, _ := os.ReadFile(local)

		provider, keys, notes, perr := ParseMarkdown(string(content), strings.TrimSuffix(f, filepath.Ext(f)))
		os.Remove(local)
		if perr != nil {
			logger.Printf("skip %s: parse: %v", f, perr)
			continue
		}
		if len(keys) == 0 {
			logger.Printf("skip %s: no keys found (provider=%s) — оставлено на Диске для исправления", f, provider)
			continue
		}

		vdir := filepath.Join(vaultBase, provider)
		if merr := os.MkdirAll(vdir, 0700); merr != nil {
			logger.Printf("skip %s: mkdir vault %s: %v", f, vdir, merr)
			continue
		}

		ecfg := em.Providers[provider]
		for i, k := range keys {
			name := "api.key"
			if i > 0 {
				name = fmt.Sprintf("api.key.%d", i)
			}
			vp := filepath.Join(vdir, name)
			if werr := os.WriteFile(vp, []byte(k), 0600); werr != nil {
				logger.Printf("write key %s: %v", vp, werr)
				continue
			}
			rec, verr := validator.ValidateKey(provider, name, ecfg, true, "")
			if verr != nil {
				logger.Printf("validate %s/%s: %v", provider, name, verr)
				continue
			}
			if notes != "" {
				rec.Instructions = strings.TrimSpace(rec.Instructions + "\n\nHuman notes: " + notes)
			}
			rec.AddedBy = opts.AddedBy
			if uerr := validator.UpsertKey(db, rec); uerr != nil {
				logger.Printf("upsert %s/%s: %v", provider, name, uerr)
				continue
			}
			logger.Printf("registered %s/%s live=%s", provider, name, rec.LiveStatus)

			// Сигнал для агентской валидации (Manus), если механика не справилась
			if rec.LiveStatus != "valid" {
				pe := PendingEntry{
					KeyID:      provider + "/" + name,
					Provider:   provider,
					VaultPath:  vp,
					LiveStatus: rec.LiveStatus,
					ReceivedAt: time.Now().UTC().Format(time.RFC3339),
				}
				if serr := AppendPendingValidation(opts.PendingValidationPath, pe); serr != nil {
					logger.Printf("pending_validation emit %s/%s: %v", provider, name, serr)
				}
			}
		}

		if !opts.Keep {
			if derr := yandexDiskDel(opts.YandexBin, remote); derr != nil {
				logger.Printf("del %s: %v (ключ уже в vault — можно удалить вручную)", remote, derr)
			}
		}
		processed++
	}
	logger.Printf("keydrop: обработано %d .md-файл(ов)", processed)
	return nil
}

// ---------- Yandex Disk helpers (с соблюдением rate-limit 1/30s) ----------

func runYandex(bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	time.Sleep(yandexRate) // пауза ПОСЛЕ каждого вызова (respect 1 req/30s)
	if err != nil {
		return string(out), fmt.Errorf("%v: %s", err, string(out))
	}
	return string(out), nil
}

func yandexDiskLs(bin, dir string) ([]string, error) {
	out, err := runYandex(bin, "disk", "ls", dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, filepath.Base(line))
	}
	return files, nil
}

func yandexDiskGet(bin, remote string) (string, error) {
	local := filepath.Join(os.TempDir(), fmt.Sprintf("keydrop-%d.md", time.Now().UnixNano()))
	if _, err := runYandex(bin, "disk", "get", remote, local); err != nil {
		return "", err
	}
	return local, nil
}

func yandexDiskDel(bin, remote string) error {
	_, err := runYandex(bin, "disk", "del", remote)
	return err
}

// ---------- Гибкий парсер .md ----------

var (
	headingRe = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	keyLabelRe = regexp.MustCompile(`(?i)\b(?:api[_-]?key|apikey|key|token|secret|password|access[_-]?token)\b\s*[:=]\s*["']?([^\s"',]+)["']?`)
	backtickRe = regexp.MustCompile("`([^`\n]{8,})`")
	prefixRe   = regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{16,}|gsk-[A-Za-z0-9_-]{16,}|csk-[A-Za-z0-9_-]{16,}|cfut_[A-Za-z0-9_.-]+|AIza[0-9A-Za-z_-]{30,}|pk_live_[0-9A-Za-z]+|pk_test_[0-9A-Za-z]+|rk_[0-9A-Za-z]+|xox[baprs]-[0-9A-Za-z-]+|ya\.[A-Za-z0-9_-]{10,})\b`)
	genericRe  = regexp.MustCompile(`\b[A-Za-z0-9_\-]{28,}\b`)
	wordRe     = regexp.MustCompile(`^[a-z]+$`)
)

// ParseMarkdown — гибко извлечь (provider, keys, notes) из .md.
//   - provider: frontmatter provider: > первый # заголовок > defaultProvider (имя файла)
//   - keys: любые key-подобные строки (префиксы, помеченные key:/token:, в ``, длинные токены)
//   - notes: весь не-key текст (описания, эндпоинты, квоты, заметки)
func ParseMarkdown(content, defaultProvider string) (provider string, keys []string, notes string, err error) {
	provider = defaultProvider

	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "---") {
		if end := strings.Index(trimmed[3:], "---"); end >= 0 {
			fm := trimmed[3 : 3+end]
			if p := fieldValue(fm, "provider"); p != "" {
				provider = p
			}
		}
	}
	if provider == defaultProvider {
		if fl := firstNonEmptyLine(content); headingRe.MatchString(fl) {
			if m := headingRe.FindStringSubmatch(fl); m != nil {
				provider = strings.TrimSpace(m[1])
			}
		}
	}
	provider = sanitizeProvider(provider)

	keys = extractKeys(content)
	notes = extractNotes(content, keys)
	return provider, keys, notes, nil
}

func fieldValue(fm, key string) string {
	re := regexp.MustCompile(`(?im)^\s*` + regexp.QuoteMeta(key) + `\s*:\s*(.+?)\s*$`)
	if m := re.FindStringSubmatch(fm); m != nil {
		return strings.Trim(m[1], "`\"'")
	}
	return ""
}

func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return t
		}
	}
	return ""
}

func sanitizeProvider(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, " ", "-")
	return strings.ToLower(p)
}

func extractKeys(content string) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(k string) {
		k = strings.TrimSpace(k)
		k = strings.Trim(k, "`\"'")
		k = strings.TrimSpace(k)
		if k == "" || seen[strings.ToLower(k)] {
			return
		}
		if len(k) < 8 {
			return
		}
		seen[strings.ToLower(k)] = true
		keys = append(keys, k)
	}
	for _, m := range keyLabelRe.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}
	for _, m := range backtickRe.FindAllStringSubmatch(content, -1) {
		add(m[1])
	}
	for _, m := range prefixRe.FindAllStringSubmatch(content, -1) {
		add(m[0])
	}
	for _, m := range genericRe.FindAllStringSubmatch(content, -1) {
		tok := m[0]
		if strings.Contains(tok, "/") || strings.Contains(tok, ".") {
			continue // не URL/пути
		}
		if len(tok) < 40 && wordRe.MatchString(tok) {
			continue // чисто буквенное слово, не ключ
		}
		add(tok)
	}
	return keys
}

func extractNotes(content string, keys []string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "---") {
		if end := strings.Index(trimmed[3:], "---"); end >= 0 {
			content = trimmed[3+end+3:]
		}
	}
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[strings.ToLower(k)] = true
	}
	var kept []string
	for _, ln := range strings.Split(content, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if headingRe.MatchString(ln) {
			continue
		}
		if containsKey(t, keySet) {
			continue
		}
		kept = append(kept, t)
	}
	return strings.Join(kept, "\n")
}

func containsKey(line string, keySet map[string]bool) bool {
	low := strings.ToLower(line)
	for k := range keySet {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// PendingEntry — одна запись сигнала для агентской валидации (Manus).
type PendingEntry struct {
	KeyID      string `json:"key_id"`
	Provider   string `json:"provider"`
	VaultPath  string `json:"vault_path"`
	LiveStatus string `json:"live_status"`
	ReceivedAt string `json:"received_at"`
}

// AppendPendingValidation — дописать запись в pending_validation.json (массив).
func AppendPendingValidation(path string, e PendingEntry) error {
	var arr []PendingEntry
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		if jerr := json.Unmarshal(b, &arr); jerr != nil {
			arr = nil
		}
	}
	arr = append(arr, e)
	out, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}
