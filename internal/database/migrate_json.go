package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"free-api-hunter/internal/models"
)

// autoMigrateJSON — если SQLite БД пустая, мигрирует данные из JSON файлов.
// Безопасно многократно вызывать — если уже есть данные, просто выходит.
func autoMigrateJSON(db *sql.DB, dataDir string) error {
	// Проверяем, есть ли уже провайдеры
	count, err := dbCount(db, "providers")
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // уже есть данные — не мигрируем
	}

	// Проверяем наличие JSON файлов
	jsonProviders := filepath.Join(dataDir, "providers.json")
	jsonFindings := filepath.Join(dataDir, "findings.json")

	if _, err := os.Stat(jsonProviders); err != nil {
		return nil // нет JSON файлов — чистая база, нечего мигрировать
	}

	if err := migrateProvidersFromJSON(db, jsonProviders); err != nil {
		return fmt.Errorf("migrate providers: %w", err)
	}

	if err := migrateFindingsFromJSON(db, jsonFindings); err != nil {
		return fmt.Errorf("migrate findings: %w", err)
	}

	if err := migrateKeyPoolFromJSON(db, filepath.Join(dataDir, "key_pool.json")); err != nil {
		return fmt.Errorf("migrate key_pool: %w", err)
	}

	return nil
}

func dbCount(db *sql.DB, table string) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
	return count, err
}

// migrateProvidersFromJSON — мигрировать провайдеров из JSON
func migrateProvidersFromJSON(db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var wrapper struct {
		Providers []*models.Provider `json:"providers"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse providers json: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO providers 
		(name, url, api_key_url, credit_card, status, models, limits, notes, source, priority, discovered_at, last_verified, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range wrapper.Providers {
		modelsJSON, _ := json.Marshal(p.Models)
		lastVerified := ""
		if p.LastVerified != nil {
			lastVerified = *p.LastVerified
		}
		if p.DiscoveredAt == "" {
			p.DiscoveredAt = now
		}

		_, err := stmt.Exec(
			p.Name, p.URL, p.APIKeyURL,
			boolToInt(p.CreditCard), string(p.Status),
			modelsJSON, p.Limits, p.Notes, p.Source,
			int(p.Priority), p.DiscoveredAt,
			lastVerified, now,
		)
		if err != nil {
			return fmt.Errorf("insert provider %s: %w", p.Name, err)
		}
	}

	return tx.Commit()
}

// migrateFindingsFromJSON — мигрировать находки из JSON
func migrateFindingsFromJSON(db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var wrapper struct {
		Findings []*models.Finding `json:"findings"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse findings json: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO findings
		(fingerprint, source_id, title, url, description, raw_text, provider_name, discovered_at, is_duplicate, quality_score, filtered_out, filter_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range wrapper.Findings {
		if f.DiscoveredAt == "" {
			f.DiscoveredAt = time.Now().UTC().Format(time.RFC3339)
		}
		if f.Fingerprint() == "0000000000000000" {
			continue // скипаем невалидные fingerprint
		}
		providerName := ""
		if f.ProviderName != nil {
			providerName = *f.ProviderName
		}
		_, err := stmt.Exec(
			f.Fingerprint(), f.SourceID, f.Title, f.URL,
			f.Description, f.RawText, providerName,
			f.DiscoveredAt, boolToInt(f.IsDuplicate),
			f.QualityScore, boolToInt(f.FilteredOut), f.FilterReason,
		)
		if err != nil {
			return fmt.Errorf("insert finding %s: %w", f.Title, err)
		}
	}

	return tx.Commit()
}

// migrateKeyPoolFromJSON — мигрировать пул API ключей из JSON
func migrateKeyPoolFromJSON(db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var wrapper struct {
		Keys []*models.APIKey `json:"keys"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse key_pool json: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO api_keys 
		(provider_name, key_location, endpoint, models, limits, is_active, last_checked, created_at, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, k := range wrapper.Keys {
		modelsJSON, _ := json.Marshal(k.Models)
		limitsJSON, _ := json.Marshal(k.Limits)
		lastChecked := ""
		if k.LastChecked != nil {
			lastChecked = *k.LastChecked
		}
		if k.CreatedAt == "" {
			k.CreatedAt = now
		}
		_, err := stmt.Exec(
			k.ProviderName, k.KeyLocation, k.Endpoint,
			modelsJSON, limitsJSON,
			boolToInt(k.IsActive), lastChecked,
			k.CreatedAt, k.Notes,
		)
		if err != nil {
			return fmt.Errorf("insert api key %s: %w", k.ProviderName, err)
		}
	}

	return tx.Commit()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
