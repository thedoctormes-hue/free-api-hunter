# SQLite Migration Plan for free-api-hunter

## Executive Summary

Документ содержит результаты исследования и план миграции проекта free-api-hunter с JSON-хранилища файлов на SQLite. 

**Мотивация:** Текущая схема хранения данных в JSON-файлах (`data/*.json`) не масштабируется. Отсутствуют индексы, нет транзакционной целостности (атомарности), и выполнение сложных запросов (агрегация, фильтрация) требует вычитки и десериализации всего файла целиком, что приведет к деградации производительности при росте количества находок и провайдеров.

---

## 1. Исследование вариантов SQLite для Go

| Библиотека | Тип | Плюсы | Минусы | Рекомендация |
| :--- | :--- | :--- | :--- | :--- |
| **`modernc.org/sqlite`** | **Pure Go** | **CGO_ENABLED=0**. Идеально для Docker (Alpine) и кросс-компиляции. Не требует gcc. Работает через стандартный `database/sql`. | Чуть медленнее нативных решений. Бинарник больше. | **Выбор №1** |
| **`github.com/mattn/go-sqlite3`** | **CGO** | Максимальная производительность. Самый популярный драйвер. | **Требует CGO_ENABLED=1**. Усложняет Docker-сборку (нужен gcc/musl-dev) и кросс-компиляцию. | Не рекомендуется |
| **`zombiezen.com/go/sqlite`** | **Pure Go** | Очень удобный API, отличная поддержка FTS5 (поиск по тексту). | Требует переписывания слоя storage под свой API (не `database/sql`). | Для сложного поиска |

**Рекомендация для проекта:** `modernc.org/sqlite`.
Для лабораторного проекта критически важна простота сборки. Текущий Dockerfile настроен на `CGO_ENABLED=0`. Переход на `modernc` позволит внедрить БД без изменения пайплайна CI/CD и Docker-образа.

---

## 2. Проектирование схемы БД

Для обеспечения нормализации и производительности предлагается следующая структура:

### Основные таблицы
- **`providers`**: `name` (PK), `url`, `api_key_url`, `credit_card` (bool), `status`, `source`, `priority`, `discovered_at`, `last_verified`.
- **`provider_models`**: `id`, `provider_name` (FK), `model_name`. (Выносим список моделей из JSON в 1:N таблицу).
- **`findings`**: `id`, `fingerprint` (UNIQUE), `source_id`, `title`, `url`, `description`, `raw_text`, `discovered_at`, `provider_name`, `quality_score`, `is_duplicate`, `filtered_out`.
- **`api_keys`**: `id`, `provider_name`, `key_location` (vault path), `endpoint`, `limits_json` (TEXT), `is_active`, `last_checked`, `created_at`.
- **`key_usage`**: `key_id` (FK), `chars_used`, `chars_limit`, `last_used`.
- **`orex_cache`**: `id`, `model_id`, `model_name`, `provider`, `context_length`, `is_free`, `synced_at`.
- **`orex_alerts`**: `id`, `type`, `model`, `message`, `timestamp`.
- **`tts_providers`**: `name` (PK), `url`, `api_key_url`, `credit_card`, `status`, `limits`, `features_json`, `languages_json`, `free_tier_json`, `discovered_at`.
- **`tts_results`**: `id`, `provider_name` (FK), `is_active`, `status_code`, `error`, `models_json`, `voices_json`, `plan`, `char_limit`, `checked_at`.
- **`scan_history`**: `id`, `scan_type`, `started_at`, `completed_at`, `status`, `metrics_json`.

### Индексы
- `idx_providers_status` ON `providers(status)`
- `idx_findings_source` ON `findings(source_id)`
- `idx_findings_discovered` ON `findings(discovered_at)`
- `idx_findings_fingerprint` ON `findings(fingerprint)`
- `idx_api_keys_active` ON `api_keys(is_active)`

---

## 3. Обновление кода и обратная совместимость

### Архитектурный подход
Чтобы не сломать REST API, мы применим паттерн **Repository**. Текущий `internal/storage` будет заменен на реализацию, которая возвращает существующие структуры из `internal/models`.

**Цепочка вызовов:**
`API Server` $\rightarrow$ `Storage Interface` $\rightarrow$ `SQLite Implementation` $\rightarrow$ `models.Struct`

### Изменения в файлах
1. **`internal/storage/storage.go`**: Изменить сигнатуры функций (добавить `context.Context`) и заменить логику работы с файлами на SQL-запросы.
2. **`internal/storage/db.go` (new)**: Инициализация DB, миграции (схемы).
3. **`internal/storage/migrate.go` (new)**: Скрипт одноразового переноса данных из JSON в SQLite.
4. **`internal/models/models.go`**: Без изменений (используем как DTO).
5. **`internal/api/server.go`**: Минимальные правки для передачи контекста в методы storage.
6. **Все `*_test.go`**: Переписать тесты на использование DB вместо временных JSON-файлов.

### Инструментарий
Использовать стандартный **`database/sql`**. 
`sqlx` может быть полезен для упрощения маппинга строк в структуры, но для минимизации зависимостей и сложности в рамках лабораторного проекта достаточно стандартной библиотеки.

---

## 4. План реализации

### Шаг 1: Инфраструктура (Схема)
- Добавить `modernc.org/sqlite` в `go.mod`.
- Реализовать `internal/storage/db.go` с функцией `InitDB(path string)`.
- Написать SQL-скрипт инициализации (создание всех таблиц и индексов).

### Шаг 2: Миграция данных
- Написать скрипт `internal/storage/migrate.go`.
- Алгоритм:
    1. Открыть текущие JSON-файлы.
    2. Начать транзакцию в SQLite.
    3. Пройтись по массивам (providers, findings и т.д.) и выполнить `INSERT`.
    4. Переименовать старые JSON файлы в `*.json.bak` для безопасности.
    5. Зафиксировать транзакцию.

### Шаг 3: Замена Storage
- Реализовать методы `Load/Save` через SQL.
- Обеспечить точное соответствие возвращаемых типов (`[]*models.Provider` и т.д.), чтобы API не заметило подмены.
- Интегрировать в `cmd/hunter/main.go`.

### Шаг 4: Тестирование
- Запуск тестов со встроенной БД `:memory:`.
- Проверка целостности данных после миграции.
- Тест REST API запросов на соответствие формату ответов.

### Оценка объёма работ
- **Файлы:** ~8 новых/измененных файлов.
- **Код:** ~800-1200 строк Go (включая SQL-запросы и маппинг).
- **Сложность:** Средняя (основной риск — корректный маппинг вложенных структур типа `Models []string` в отдельные таблицы).
