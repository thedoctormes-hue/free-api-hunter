package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// runMigrations — создать таблицы если не существуют
func runMigrations(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS providers (
			name TEXT PRIMARY KEY,
			url TEXT NOT NULL DEFAULT '',
			api_key_url TEXT NOT NULL DEFAULT '',
			credit_card INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'unverified',
			models TEXT NOT NULL DEFAULT '[]',
			limits TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			priority INTEGER NOT NULL DEFAULT 2,
			discovered_at TEXT NOT NULL DEFAULT '',
			last_verified TEXT,
			updated_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS findings (
			fingerprint TEXT PRIMARY KEY,
			source_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			raw_text TEXT NOT NULL DEFAULT '',
			provider_name TEXT,
			discovered_at TEXT NOT NULL DEFAULT '',
			is_duplicate INTEGER NOT NULL DEFAULT 0,
			quality_score REAL NOT NULL DEFAULT 0.0,
			filtered_out INTEGER NOT NULL DEFAULT 0,
			filter_reason TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_name TEXT NOT NULL DEFAULT '',
			key_location TEXT NOT NULL DEFAULT '',
			endpoint TEXT NOT NULL DEFAULT '',
			models TEXT NOT NULL DEFAULT '[]',
			limits TEXT NOT NULL DEFAULT '{}',
			is_active INTEGER NOT NULL DEFAULT 1,
			last_checked TEXT,
			created_at TEXT NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT ''
		)`,
		// KRV-Validator: живая валидация ключей из vault (spike/krv-validator).
		// Отличается от api_keys: хранит именно LIVE-статус конкретного файла ключа.
		`CREATE TABLE IF NOT EXISTS "keys" (
			provider TEXT NOT NULL DEFAULT '',
			key_id TEXT NOT NULL PRIMARY KEY,
			vault_path TEXT NOT NULL DEFAULT '',
			registry_status TEXT NOT NULL DEFAULT 'unknown',
			live_status TEXT NOT NULL DEFAULT 'unknown',
			last_validated TEXT,
			models TEXT NOT NULL DEFAULT '[]',
			auth_type TEXT NOT NULL DEFAULT 'unknown',
			base_url TEXT NOT NULL DEFAULT '',
			instructions TEXT NOT NULL DEFAULT '',
			added_by TEXT NOT NULL DEFAULT '',
			added_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS key_pool (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_hash TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			chars_used INTEGER NOT NULL DEFAULT 0,
			chars_limit INTEGER NOT NULL DEFAULT 0,
			active INTEGER NOT NULL DEFAULT 1,
			last_used TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS orex_cache (
			id INTEGER PRIMARY KEY,
			free_models TEXT NOT NULL DEFAULT '[]',
			alerts TEXT NOT NULL DEFAULT '[]',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS scan_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scan_time TEXT NOT NULL DEFAULT '',
			raw_count INTEGER NOT NULL DEFAULT 0,
			filtered_count INTEGER NOT NULL DEFAULT 0,
			providers_total INTEGER NOT NULL DEFAULT 0,
			new_findings INTEGER NOT NULL DEFAULT 0
		)`,
		// Индексы для частых запросов
		`CREATE INDEX IF NOT EXISTS idx_providers_status ON providers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_findings_source ON findings(source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_findings_filtered ON findings(filtered_out)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_provider ON api_keys(provider_name)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_keys_provider ON "keys"(provider)`,
		`CREATE INDEX IF NOT EXISTS idx_keys_live ON "keys"(live_status)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %v\n%s", err, q)
		}
	}
	// Идемпотентное добавление колонки provenance (живая БД уже существует;
	// SQLite не поддерживает ADD COLUMN IF NOT EXISTS — игнорируем duplicate column).
	if _, err := db.Exec(`ALTER TABLE "keys" ADD COLUMN provenance TEXT NOT NULL DEFAULT 'unknown'`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migration provenance: %v", err)
		}
	}
	return nil
}
