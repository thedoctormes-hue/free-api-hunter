# ADR-0005: Мigrация с JSON-хранилища на SQLite

**Дата:** 2026-06-26
**Автор:** Штрейкбрехер (Developer / Technical Lead)
**Статус:** Proposed

---

## 1. Status

**Proposed** — черновик, ожидает исследования и согласования. Не применять до принятия.

---

## 2. Context

Текущее хранилище free-api-hunter — JSON-файлы в `data/`:

| Файл | Содержимое | Размер | Кол-во записей |
|---|---|---|---|
| `providers.json` | LLM-провайдеры | ~9 KB | 21 |
| `findings.json` | Находки из источников | ~47 KB | 9 |
| `tts_providers.json` | TTS/STT-провайдеры | ~3 KB | ~5 |
| `key_pool.json` | API-ключи | ~1 KB | ~3 |
| `orex_cache.json` | Кэш моделей Orex | <1 KB | ~30 моделей |

Проблемы текущего подхода:
- **Нет конкурентного доступа** — два процесса одновременно пишут в один файл = race condition
- **Нет индексов** — поиск по всем данным = полный перебор при каждом запросе
- **Не масштабируется** — при росте числа провайдеров и находок файлы будут раздуваться
- **Сериализация/десериализация** — каждый Load/Save = маршалинг всего массива, даже для одной записи
- **Нет SQL-подоб запросов** — фильтрация, сортировка, агрегация только в коде

Потенциальные точки роста:
- Бэклог содержит «MCP server» и «Telegram alerter» — оба пишут данные
- Пользовательские источники — новые потоки данных
- Changelog провайдеров — исторические записи, которые накапливаются

---

## 3. Decision

**Выбран вариант: SQLite через `modernc.org/sqlite`**

### 3.1 Почему modernc.org/sqlile

- **Pure Go** — нет CGO, нет зависимостей от libsqlite3, статическая линковка
- **Поддержка `database/sql`** — стандартный Go-интерфейс, можно заменить бэкенд
- **WAL mode** — concurrent reads + single writer без блокировки
- **`_journal_mode=wal` + `_busy_timeout`** — решает проблему «database is locked»
- **Активное развитие** — форк mattn/sqlite3, совместим с SQLite 3.46+
- **Размер бинарника** — +~3 MB, приемлемо для локального инструмента

### 3.2 Архитектура

```
internal/storage/
├── storage.go        ← интерфейс (сохраняем текущий API)
├── json_store.go     ← текущая реализация (fallback)
├── sqlite_store.go   ← новая реализация
└── sqlite_schema.go  ← DDL и миграции
```

Интерфейс `storage.Store`:
```go
type Store interface {
    // Providers
    SaveProviders(providers []*models.Provider) error
    LoadProviders() ([]*models.Provider, error)
    // Findings
    SaveFindings(findings []*models.Finding) error
    LoadFindings() ([]*models.Finding, error)
    // KeyPool
    SaveKeyPool(keys []*models.APIKey) error
    LoadKeyPool() ([]*models.APIKey, error)
    // Orex cache
    SaveOrexCache(cache *OrexCache) error
    LoadOrexCache() (*OrexCache, error)
    // TTS providers
    SaveTTSProviders(providers []*models.TTSProvider) error
    LoadTTSProviders() ([]*models.TTSProvider, error)
}
```

### 3.3 DDL (схема БД)

```sql
CREATE TABLE IF NOT EXISTS providers (
    name         TEXT PRIMARY KEY,
    url          TEXT NOT NULL,
    api_key_url  TEXT NOT NULL DEFAULT '',
    credit_card  INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'unverified',
    models       TEXT NOT NULL DEFAULT '[]',  -- JSON array
    limits       TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL DEFAULT '',
    priority     INTEGER NOT NULL DEFAULT 2,
    discovered_at TEXT NOT NULL,
    last_verified TEXT,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS findings (
    fingerprint    TEXT PRIMARY KEY,          -- FNV-1a hash
    source_id      TEXT NOT NULL,
    title          TEXT NOT NULL,
    url            TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    raw_text       TEXT NOT NULL DEFAULT '',
    discovered_at  TEXT NOT NULL,
    provider_name  TEXT,
    is_duplicate   INTEGER NOT NULL DEFAULT 0,
    quality_score  REAL NOT NULL DEFAULT 0.0,
    filtered_out   INTEGER NOT NULL DEFAULT 0,
    filter_reason  TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS key_pool (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_name TEXT NOT NULL,
    key_location  TEXT NOT NULL,
    endpoint     TEXT NOT NULL DEFAULT '',
    models        TEXT NOT NULL DEFAULT '[]',  -- JSON array
    limits        TEXT NOT NULL DEFAULT '{}',  -- JSON object
    is_active     INTEGER NOT NULL DEFAULT 1,
    last_checked  TEXT,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    notes        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS orex_cache (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider    TEXT NOT NULL,
    model_name  TEXT NOT NULL,
    context_length INTEGER,
    is_moderated INTEGER DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(provider, model_name)
);

CREATE TABLE IF NOT EXISTS tts_providers (
    name          TEXT PRIMARY KEY,
    url           TEXT NOT NULL,
    api_key_url   TEXT NOT NULL DEFAULT '',
    credit_card   INTEGER NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'unverified',
    models        TEXT NOT NULL DEFAULT '[]',
    limits        TEXT NOT NULL DEFAULT '',
    free_tier     TEXT,                        -- JSON object
    features      TEXT NOT NULL DEFAULT '[]',
    languages     TEXT NOT NULL DEFAULT '[]',
    source        TEXT NOT NULL DEFAULT '',
    priority      INTEGER NOT NULL DEFAULT 2,
    discovered_at TEXT NOT NULL,
    last_verified TEXT,
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_providers_status ON providers(status);
CREATE INDEX IF NOT EXISTS idx_providers_priority ON providers(priority);
CREATE INDEX IF NOT EXISTS idx_findings_source ON findings(source_id);
CREATE INDEX IF NOT EXISTS idx_findings_duplicate ON findings(is_duplicate);
CREATE INDEX IF NOT EXISTS idx_findings_filtered ON findings(filtered_out);
CREATE INDEX IF NOT EXISTS idx_key_pool_active ON key_pool(is_active);
CREATE INDEX IF NOT EXISTS idx_orex_provider ON orex_cache(provider);
CREATE INDEX IF NOT EXISTS idx_tts_status ON tts_providers(status);

-- Миграция (версионирование схемы)
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### 3.4 Стратегия миграции

**Фаза 1: Интерфейс + SQLite-реализация**
- Извлечь интерфейс `Store` из текущего `storage.go`
- Реализовать `SQLiteStore` с полным набором методов
- Тесты: паритет с JSON-реализацией (golden tests)

**Фаза 2: Импорт существующих данных**
- При первом запуске: если `data/` содержит JSON — импортировать в SQLite
- Флаг `--migrate-once` или автоматическое определение
- После успешного импорта: переименовать `data/*.json` в `data/*.json.bak`

**Фаза 3: Удаление JSON-реализации**
- Убрать `json_store.go` после стабилизации
- Сохранить обратную совместимость API (сигнатуры функций не меняются)

### 3.5 Обратная совместимость API

Текущие вызывающие коды (handlers, фильтры, верификаторы) используют функции-обёртки:
```go
storage.SaveProviders(providers)  // сигнатура не меняется
storage.LoadProviders()           // возврат тот же тип
```

Интерфейс `Store` сохраняет эти сигнатуры. Внутренняя реализация меняется прозрачно.

---

## 4. Alternatives

### 4.1 PostgreSQL
- **Плюсы:** полноценный реляционный СУБД, мощный SQL, конкурентный доступ
- **Минусы:** избыточно для локального инструмента; требует отдельного сервера; настройка пользователей, бэкапов; нет встроенного full-text search по умолчанию
- **Вердикт:** перебор для проекта с 21 провайдером

### 4.2 BoltDB
- **Плюсы:** встроенный key-value, быстрый, Go-native
- **Минусы:** нет реляционной модели; нет SQL; сложно делать запросы по полям; нет индексов кроме первичного ключа
- **Вердикт:** не подходит — данные реляционные (providers → models, findings → providers)

### 4.3 Оставить JSON
- **Плюсы:** ничего не меняется, просто, читаемый формат
- **Минусы:** не решает проблемы масштабирования, конкурентного доступа, индексации
- **Вердикт:** технический долг будет расти

### 4.4 BadgerDB
- **Плюсы:** Go-native, LSM-tree, быстрый
- **Минусы:** key-value, нет SQL, нет реляционной модели
- **Вердикт:** аналогично BoltDB — не подходит для реляционных данных

---

## 5. Consequences

### 5.1 Что нужно изменить в коде

- `internal/storage/storage.go` — извлечь интерфейс, добавить SQLite-реализацию
- `internal/storage/sqlite_store.go` — новый файл (основная реализация)
- `internal/storage/sqlite_schema.go` — DDL и миграции
- `cmd/` — добавить флаг `--db-path` (по умолчанию `data/app.db`)
- `go.mod` — добавить зависимость `modernc.org/sqlite`
- Тесты — переписать на table-driven tests с обоими бэкендами

### 5.2 Что становится лучше

- Конкурентный доступ — WAL mode позволяет параллельное чтение
- Индексация — поиск по status, priority, source_id вместо полного перебора
- Масштабируемость — десятки тысяч записей не проблема
- Фильтрация в SQL — `WHERE status='verified' AND priority=1` вместо циклов в Go
- Атомарность — транзакции при обновлении нескольких таблиц

### 5.3 Риски

- **Размер бинарника** — +3 MB от SQLite приемлемо
- **WAL-файлы** — нужно добавить `data/app.db-wal` и `data/app.db-shm` в бэкапы
- **Одновременный доступ** — WAL mode решает, но нужно тестировать при нагрузке
- **Миграция схемы** — простая стратегия (ALTER TABLE) достаточна для текущего объёма данных
- **Совместимость** — `modernc.org/sqlite` поддерживает Go 1.21+, проверить версию в `go.mod`

---

## 6. Open Questions

- [ ] Нужен ли full-text search по `findings.title` / `findings.description`? (FTS5 в SQLite — хороший вариант)
- [ ] Сохранять ли JSON-файлы для экспорта/бэкапа, или SQLite — единственный источник?
- [ ] Нужна ли поддержка миграций назад (rollback)?
- [ ] Порядок внедрения: сначала MCP server / Telegram alerter, потом миграция? Или миграция как пререквизит?

---

## 7. References

- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure Go SQLite driver
- [SQLite WAL mode](https://www.sqlite.org/wal.html) — concurrency documentation
- [database/sql](https://pkg.go.dev/database/sql) — standard Go interface
