# Free API Hunter — Полная документация

## Обзор

Free API Hunter — система для обнаружения, верификации и мониторинга бесплатных LLM API.

**Версия:** v0.6.0
**Разработчик:** Штрейкбрехер
**Репозиторий:** https://github.com/thedoctormes-hue/free-api-hunter

## Архитектура

```
cmd/hunter/main.go          — CLI точка входа, оркестрация
internal/
  models/models.go           — модели данных (Provider, Finding, APIKey)
  scraper/scraper.go         — сбор данных из источников
  filter/filter.go           — фильтрация находок, дедуп, скоринг
  verifier/verifier.go       — верификация ключей и провайдеров
  storage/storage.go         — JSON хранилище
  vault/vault_client.go      — безопасное хранение ключей (файловый доступ)
  alerter/alerter.go         — Telegram алерты (vault-first конфигурация)
  orex/client.go             — Orex API client (OpenRouter Expert)
config/
  sources.json               — источники + провайдеры (source of truth для статусов)
  filters.json               — фильтры (keywords, domains, thresholds, scoring)
  alerter.json               — Telegram config (fallback, placeholder)
  crontab.txt                — cron-задачи
data/
  providers.json             — база провайдеров (runtime cache)
  findings.json              — находки сканера
  key_pool.json              — пул ключей
  orex_cache.json            — кэш Orex
```

## Поток данных

```
1. loadConfig(sources.json)     → Config{Sources, Providers}
2. scraper.RunScraper(Sources)  → []Finding (сырые)
3. filter.FilterFindings([])    → []Finding (очищенные)
4. loadInitialProviders(Config) → []*Providers (мерж config + runtime)
5. verifier.VerifyProvider()    → обновление статусов
6. storage.Save()               → data/providers.json, data/findings.json
7. alerter.SendTelegram()       → Telegram алерт (если настроен)
```

### Мерж статусов (loadInitialProviders)

При загрузке провайдеров происходит мерж двух источников:

1. **config/sources.json** — source of truth для статусов (verified/confirmed/claimed)
2. **data/providers.json** — runtime cache для моделей, URL, лимитов

Приоритет: runtime-данные (модели, URL) + config-статусы. Провайдеры только из config добавляются как новые. Провайдеры только из runtime (из Orex и т.д.) сохраняются как есть.

## Хранение ключей

### Файловый vault

Путь: `/root/LabDoctorM/vault/free-api-hunter/`

```
vault/free-api-hunter/
├── cohere/
│   ├── api.key               — основной ключ
│   ├── api.key.2             — запасной ключ 1
│   ├── api.key.3             — запасной ключ 2
│   └── api.key.4             — запасной ключ 3
└── telegram_bot_token.key    — не настроен (placeholder в alerter.json)
```

**Принципы:**
- Ключи НЕ хранятся в коде
- Ключи НЕ коммитятся в git
- Права 600 на файлах
- Формат: plaintext, одно значение на файл

### lab-vault (HTTP API)

Отдельный сервис на `127.0.0.1:8301`. Проект: `/root/LabDoctorM/projects/lab-vault/`

Получение секрета:
```bash
curl -s http://127.0.0.1:8301/access/{token}
```

Токены хранятся в `config.yaml` lab-vault с TTL (720 часов по умолчанию).

## Работа с ключами в коде

### Загрузка ключа

```go
import "free-api-hunter/internal/vault"

key, err := vault.GetDefaultKey("cohere")
if err != nil {
    log.Fatal(err)
}
// key.Value содержит API ключ
```

### Список провайдеров

```go
providers, err := vault.ListProviders()
// []string{"cohere", ...}
```

### Проверка наличия ключа

```go
if vault.HasKey("cohere", "api") {
    // ключ существует
}
```

## Alerter — Telegram интеграция

### Конфигурация (vault-first)

`LoadConfig()` проверяет источники в порядке:
1. **Vault**: `/root/LabDoctorM/vault/free-api-hunter/telegram_bot_token.key` + `telegram_chat_id.key`
2. **Config file**: `config/alerter.json` (fallback)
3. **Placeholder detection**: если значения начинаются с `YOUR_` — warning + nil config

### Формат алерта

```
🆓 Free API Hunter — Отчёт сканирования
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📅 2026-06-21 01:00 UTC
📊 Сырых находок: 7
📊 После фильтра: 0
📊 Провайдеров: 17

🆕 Новые провайдеры:
  • OpenRouter
  • Groq
```

## Фильтрация

Все настройки в `config/filters.json`. Применяется через `engine.ApplyConfig()`.

### Цепочка фильтров (applyFilters)

1. **Дедупликация** — fingerprint (provider_name + models + limits)
2. **Мин. длина описания** — по умолчанию 30, настраивается в `quality_threshold.min_description_length`
3. **Проверка URL** — если `require_url: true`, отбрасывает без http
4. **Исключённые домены** — medium.com, substack.com, linkedin.com + из `spam_filters.exclude_domains`
5. **Исключённые провайдеры** — kilo gateway, kilochat, kilo + из `excluded_providers`
6. **Trash-источники** — pastebin.com, ghostbin.com, rentry.co + из `spam_filters.exclude_trash_sources`
7. **URL uniqueness** — дедупликация по URL (если `dedup.check_url_uniqueness: true`)
8. **Спам-паттерн** — regex: `купить сейчас|продать|скидка \d+%|рефералка|affiliate link`
9. **Exclude keywords** — из `spam_filters.exclude_keywords` (купить, продать, скидка, referral, affiliate, course, tutorial, seo, marketing, ebook, telegram канал, etc.)
10. **Возраст находки** — если `exclude_expired: true` и `max_age_days > 0`, отбрасывает старые
11. **Скоринг качества** — по длине описания, моделям, лимитам, URL документации

### Скоринг (scoreQuality)

- Длинное описание (>200 chars): +0.3
- Упоминание моделей (gpt, claude, gemini, llama, mistral, etc.): +0.1 каждое, max +0.3
- Упоминание лимитов (rpm, tpm, rpd, free, tier, credit, limit, quota): +0.1 каждое, max +0.3
- URL с docs/api: +0.1
- Максимум: 1.0

## Проверенные провайдеры (актуально на 2026-06-21)

### Cohere ✅

**Ключи:** 4 ключа в vault, все рабочие

**Эндпоинты:**

| Эндпоинт | Статус | Примечание |
|----------|--------|------------|
| `/v1/models` | ✅ 200 | 20 моделей |
| `/v2/chat` | ✅ 200 | Основной chat API |
| `/v1/chat` | ⚠️ Deprecated | Требует миграцию на v2 |
| `/v1/generate` | ❌ Удалён | CoGenerate удалён 15.09.2025 |
| `/v1/embed` | ✅ 200 | 1024 dimensions |
| `/v1/rerank` | ✅ 200 | Rerank API |

**Rate limits (из заголовков):**
```
x-endpoint-monthly-call-limit: 1000
x-trial-endpoint-call-limit: 100
x-trial-endpoint-call-remaining: 98
```

**Модели (20):**
- c4ai-aya-expanse-32b, c4ai-aya-vision-32b
- cohere-transcribe-03-2026
- command-a-03-2025, command-a-plus-05-2026, command-a-reasoning-08-2025
- command-a-translate-08-2025, command-a-vision-07-2025
- command-r-08-2024, command-r-plus-08-2024, command-r7b-12-2024, command-r7b-arabic-02-2025
- embed-english-light-v3.0, embed-english-v3.0, embed-multilingual-light-v3.0, embed-multilingual-v3.0 (+ image variants)

### Cerebras ✅

- **Модели:** gpt-oss-120b, zai-glm-4.7
- **Лимиты:** 5 RPM / 30K tokens/min / 2400 req/day
- **Rate limit headers:** `x-ratelimit-limit-requests-minute: 5`, `x-ratelimit-limit-tokens-minute: 30000`

### Mistral ✅

- **Модели:** mistral-small-latest, mistral-large-latest, open-mistral-nemo (+1)
- **Лимиты:** 50 RPM / 500K tokens/min / 1 RPS
- **Rate limit headers:** `x-ratelimit-limit-req-minute: 50`, `x-ratelimit-limit-tokens-minute: 50000`

### Cloudflare Workers AI ✅

- **Модели:** @cf/meta/llama-3.3-70b, @cf/meta/llama-4-scout, @cf/openai/gpt-oss-120b (+1)
- **Лимиты:** 10K neurons/day
- **Rate limit headers:** `cf-ai-neurons: 1.98`

### Google AI Studio (Gemini) ⚠️

- **Модели:** gemini-2.0-flash, gemini-2.5-flash, gemini-2.5-pro
- **Лимиты:** 1500 RPM, 1500 RPD, 1M TPM
- **Статус:** ключ валидный, квота исчерпана

### OpenRouter ✅

- **Модели:** 10 бесплатных (deepseek-r1, qwen3-8b/14b, gpt-oss-120b, etc.)
- **Лимиты:** 50 req/day (без кредитов), 1000 req/day (с кредитами), 20 RPM

### Groq ✅

- **Модели:** openai/gpt-oss-20b/120b, llama-3.3-70b-versatile (+2)
- **Лимиты:** 14400 req/day, 6000 tokens/min, 30 RPM

## Orion Scan — ежедневный сканер бесплатных моделей OpenRouter

Скрипт `orion-scan.sh` — автоматическое сканирование и верификация бесплатных моделей OpenRouter с контекстом ≥128K.

### Как работает

1. Получает список всех моделей из OpenRouter API
2. Фильтрует бесплатные (prompt=0, completion=0) с контекстом ≥128K
3. Проверяет каждую модель на живучесть (тестовый запрос)
4. Формирует отчёт с группировкой по размеру контекста

### Группировка

- 🥇 Гигант-модели (1M+ контекст)
- 🥈 Средние (256K-512K)
- 🥉 Компактные (128K-256K)

## Orex Integration

Orex (OpenRouter Expert) — внешний сервис `http://127.0.0.1:8710`.

### Эндпоинты

| Эндпоинт | Метод | Описание |
|----------|-------|----------|
| `/api/models` | GET | Список моделей |
| `/api/models?pricing_free=true` | GET | Только бесплатные |
| `/api/models?provider=X` | GET | По провайдеру |
| `/api/select?task_type=X` | GET | Подбор модели под задачу |
| `/api/pricing/cost` | GET | Расчёт стоимости |
| `/api/sync` | GET | Синхронизация базы |
| `/api/alerts` | GET | Алерты о новых моделях |

### Схема событий

```json
{
  "event": "new_orex_model",
  "provider": "cerebras",
  "model": "gpt-oss-120b",
  "status": "ok",
  "details": "New free model available",
  "timestamp": "2026-06-18T12:00:00Z"
}
```

## Использование CLI

```bash
# Версия
./bin/hunter --version

# Сбор данных (dry-run)
./bin/hunter --dry-run --limit 20

# Верификация провайдеров
./bin/hunter --verify

# Полный цикл
./bin/hunter

# Конкретный источник
./bin/hunter --source hackernews

# Без алертов
./bin/hunter --no-alerts
```

## Тестирование

```bash
# Все тесты
go test ./... -count=1

# С подробностями
go test ./... -v

# С покрытием
go test ./... -cover

# Конкретный пакет
go test ./internal/filter/ -v
go test ./internal/alerter/ -v
```

### Покрытие тестами

| Пакет | Тесты | Покрытие |
|-------|-------|----------|
| models | 4 | ~85% |
| scraper | 6 | ~70% |
| filter | 24 | ~80% |
| verifier | 5 | ~75% |
| storage | 5 | ~85% |
| vault | 9 | ~90% |
| orex | 8 | ~80% |
| alerter | 13 | ~85% |
| cmd/hunter | 0 | integration only |

## Добавление нового провайдера

1. `mkdir -p /root/LabDoctorM/vault/free-api-hunter/{provider}`
2. `echo -n "API_KEY" > /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key`
3. `chmod 600 /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key`
4. Добавить провайдера в `config/sources.json`
5. `./bin/hunter --verify`

## Безопасность

- **Никогда** не коммитьте ключи в git
- **Никогда** не логируйте значения ключей
- Используйте vault вместо hardcoded значений
- При утечке — немедленно отозвать ключ и сгенерировать новый
- Не отправляйте ключи в чат (даже зашифрованные URL)
