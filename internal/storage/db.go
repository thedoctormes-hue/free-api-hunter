package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"free-api-hunter/internal/database"
	"free-api-hunter/internal/models"
)

// InitDB — инициализировать SQLite БД
func InitDB(dataDir string) error {
	return database.Init(dataDir)
}

// CloseDB — закрыть БД
func CloseDB() error {
	return database.Close()
}

// ============================================================
// Providers
// ============================================================

// SaveProviders — сохранить провайдеров (UPSERT)
func SaveProviders(providers []*models.Provider, path string) error {
	db := database.DB()
	if db == nil {
		return fmt.Errorf("database not initialized")
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

	for _, p := range providers {
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
			return fmt.Errorf("save provider %s: %w", p.Name, err)
		}
	}

	return tx.Commit()
}

// LoadProviders — загрузить всех провайдеров
func LoadProviders(path string) ([]*models.Provider, error) {
	db := database.DB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`SELECT name, url, api_key_url, credit_card, status, models, limits, notes, source, priority, discovered_at, last_verified
		FROM providers ORDER BY priority, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*models.Provider
	for rows.Next() {
		var p models.Provider
		var modelsJSON string
		var creditCard int
		var lastVerified sql.NullString

		if err := rows.Scan(&p.Name, &p.URL, &p.APIKeyURL, &creditCard, &p.Status,
			&modelsJSON, &p.Limits, &p.Notes, &p.Source, &p.Priority,
			&p.DiscoveredAt, &lastVerified); err != nil {
			return nil, err
		}

		p.CreditCard = creditCard != 0
		if lastVerified.Valid && lastVerified.String != "" {
			p.LastVerified = &lastVerified.String
		}

		if modelsJSON != "" && modelsJSON != "null" {
			_ = json.Unmarshal([]byte(modelsJSON), &p.Models)
		}

		providers = append(providers, &p)
	}

	return providers, rows.Err()
}

// ============================================================
// Findings
// ============================================================

// SaveFindings — сохранить находки (UPSERT по fingerprint)
func SaveFindings(findings []*models.Finding, path string) error {
	db := database.DB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	now := time.Now().UTC().Format(time.RFC3339)
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

	for _, f := range findings {
		if f.DiscoveredAt == "" {
			f.DiscoveredAt = now
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
			return fmt.Errorf("save finding %s: %w", f.Title, err)
		}
	}

	return tx.Commit()
}

// LoadFindings — загрузить все находки
func LoadFindings(path string) ([]*models.Finding, error) {
	db := database.DB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`SELECT fingerprint, source_id, title, url, description, raw_text, provider_name, discovered_at, is_duplicate, quality_score, filtered_out, filter_reason FROM findings ORDER BY quality_score DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []*models.Finding
	for rows.Next() {
		var f models.Finding
		var isDup, filteredOut int
		var providerName sql.NullString
		var fingerprint string

		if err := rows.Scan(&fingerprint, &f.SourceID, &f.Title, &f.URL,
			&f.Description, &f.RawText, &providerName, &f.DiscoveredAt,
			&isDup, &f.QualityScore, &filteredOut, &f.FilterReason); err != nil {
			return nil, err
		}

		f.IsDuplicate = isDup != 0
		f.FilteredOut = filteredOut != 0
		if providerName.Valid {
			f.ProviderName = &providerName.String
		}

		findings = append(findings, &f)
	}

	return findings, rows.Err()
}

// ============================================================
// API Keys
// ============================================================

// SaveKeyPool — сохранить пул API ключей
func SaveKeyPool(keys []*models.APIKey, path string) error {
	db := database.DB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Очищаем и вставляем заново
	if _, err := tx.Exec("DELETE FROM api_keys"); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO api_keys
		(provider_name, key_location, endpoint, models, limits, is_active, last_checked, created_at, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, k := range keys {
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
			return fmt.Errorf("save api key %s: %w", k.ProviderName, err)
		}
	}

	return tx.Commit()
}

// LoadKeyPool — загрузить пул API ключей
func LoadKeyPool(path string) ([]*models.APIKey, error) {
	db := database.DB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`SELECT provider_name, key_location, endpoint, models, limits, is_active, last_checked, created_at, notes
		FROM api_keys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		var modelsJSON, limitsJSON string
		var isActive int
		var lastChecked sql.NullString

		if err := rows.Scan(&k.ProviderName, &k.KeyLocation, &k.Endpoint,
			&modelsJSON, &limitsJSON, &isActive, &lastChecked,
			&k.CreatedAt, &k.Notes); err != nil {
			return nil, err
		}

		k.IsActive = isActive != 0
		if lastChecked.Valid && lastChecked.String != "" {
			k.LastChecked = &lastChecked.String
		}

		if modelsJSON != "" && modelsJSON != "null" {
			_ = json.Unmarshal([]byte(modelsJSON), &k.Models)
		}
		if limitsJSON != "" && limitsJSON != "null" {
			_ = json.Unmarshal([]byte(limitsJSON), &k.Limits)
		}

		keys = append(keys, &k)
	}

	return keys, rows.Err()
}

// ============================================================
// Orex Cache
// ============================================================

// SaveOrexCache — сохранить кэш Orex
func SaveOrexCache(cache *OrexCache, path string) error {
	db := database.DB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	freeModelsJSON, _ := json.Marshal(cache.FreeModels)
	alertsJSON, _ := json.Marshal(cache.Alerts)

	_, err := db.Exec(`INSERT OR REPLACE INTO orex_cache (id, free_models, alerts, updated_at)
		VALUES (1, ?, ?, ?)`, freeModelsJSON, alertsJSON, now)
	return err
}

// LoadOrexCache — загрузить кэш Orex
func LoadOrexCache(path string) (*OrexCache, error) {
	db := database.DB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var cache OrexCache
	var freeModelsJSON, alertsJSON string

	err := db.QueryRow("SELECT free_models, alerts, updated_at FROM orex_cache WHERE id = 1").
		Scan(&freeModelsJSON, &alertsJSON, &cache.Meta.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(freeModelsJSON), &cache.FreeModels)
	_ = json.Unmarshal([]byte(alertsJSON), &cache.Alerts)
	cache.Meta.Version = "0.1.0"
	// Meta.UpdatedAt already set from Scan
	cache.Meta.Count = len(cache.FreeModels)

	return &cache, nil
}

// ============================================================
// Scan History
// ============================================================

// SaveScanHistory — сохранить историю сканирования
func SaveScanHistory(rawCount, filteredCount, providersTotal, newFindings int) error {
	db := database.DB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO scan_history (scan_time, raw_count, filtered_count, providers_total, new_findings)
		VALUES (?, ?, ?, ?, ?)`, now, rawCount, filteredCount, providersTotal, newFindings)
	return err
}

// LoadScanHistory — загрузить историю сканирований
func LoadScanHistory(limit int) ([]map[string]interface{}, error) {
	db := database.DB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(`SELECT id, scan_time, raw_count, filtered_count, providers_total, new_findings
		FROM scan_history ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var id, rawCount, filteredCount, providersTotal, newFindings int
		var scanTime string
		if err := rows.Scan(&id, &scanTime, &rawCount, &filteredCount, &providersTotal, &newFindings); err != nil {
			return nil, err
		}
		history = append(history, map[string]interface{}{
			"id":              id,
			"scan_time":       scanTime,
			"raw_count":       rawCount,
			"filtered_count":  filteredCount,
			"providers_total": providersTotal,
			"new_findings":    newFindings,
		})
	}
	return history, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
