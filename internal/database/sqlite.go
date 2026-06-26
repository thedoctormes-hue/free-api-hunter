package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	mu sync.Mutex
	db *sql.DB
)

// DB — глобальный пул соединений к SQLite
func DB() *sql.DB {
	return db
}

// Init — инициализировать SQLite БД и выполнить миграции.
// При первом запуске автоматически мигрирует данные из JSON.
func Init(dataDir string) error {
	mu.Lock()
	defer mu.Unlock()

	if db != nil {
		// Already initialized — check if still alive
		if err := db.Ping(); err == nil {
			return nil
		}
		_ = Close()
		db = nil
	}

	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "free-api-hunter.db")
	// WAL mode для concurrent reads, foreign keys для integrity
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000", filepath.ToSlash(dbPath))

	var err error
	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	// Проверяем что БД жива
	if err = db.Ping(); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}

	// Миграции схемы
	if err = runMigrations(db); err != nil {
		return err
	}

	// Автомиграция из JSON (только если таблицы пустые)
	return autoMigrateJSON(db, dataDir)
}

// Close — закрыть соединение с БД
func Close() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}
