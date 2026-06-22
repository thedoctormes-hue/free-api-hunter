---
description: "free-api-hunter — README"
type: readme
last_reviewed: 2026-06-22
last_code_change: 2026-06-22
status: active
---

# Free API Hunter

> **Владелец:** DoctorM&Ai | **Статус:** active | **Версия:** v0.6.0

## Описание

Поиск и мониторинг бесплатных LLM API — автоматический сбор, верификация и каталогизация бесплатных LLM эндпоинтов. Ищет бесплатные эндпоинты, проверяет их работоспособность, скорит качество находок.

## Быстрый старт

```bash
cd projects/free-api-hunter

# Сборка
go build -ldflags "-X main.Version=$(git describe --tags --always)" -o bin/hunter cmd/hunter/main.go

# Запуск поиска (dry-run)
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

## Архитектура

**Стек:** Go 1.23, стандартная библиотека + JSON-хранилище

**Структура:**
```
cmd/hunter/main.go          — CLI точка входа, оркестрация
internal/
  models/models.go           — модели данных (Provider, Finding, APIKey)
  scraper/scraper.go         — сбор данных из источников
  filter/filter.go           — фильтрация находок, дедуп, скоринг
  verifier/verifier.go       — верификация ключей и провайдеров
  storage/storage.go         — JSON хранилище
  vault/vault.go             — безопасное хранение ключей
  orex/client.go             — Orex API client (OpenRouter Expert)
  alerter/alerter.go         — Telegram алерты
configs/
  sources.json               — источники + провайдеры
  filters.json               — фильтры и скоринг
  alerter.json               — Telegram config (fallback)
data/                        — runtime cache (gitignored)
  providers.json             — база провайдеров
  findings.json              — находки сканера
```

**Поток данных:**
```
sources.json → scraper → []Finding (сырые)
    → filter (дедуп, спам, скоринг) → []Finding (очищенные)
    → verifier (проверка URL, free tier, credit card)
    → storage (data/providers.json, data/findings.json)
    → alerter (Telegram, если настроен)
```

## Хранение ключей

Ключи хранятся в vault: `/root/LabDoctorM/vault/free-api-hunter/`

Формат: plaintext файлы с правами 600, одно значение на файл.
Ключи **никогда** не коммитятся в git.

```bash
# Добавить ключ
echo -n "API_KEY" > /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key
chmod 600 /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key
```

## Разработка

```bash
# Тесты
go test ./... -count=1 -v

# С покрытием
go test ./... -cover

# Конкретный пакет
go test ./internal/filter/ -v

# Линтер
go vet ./...
```

## Деплой

```bash
# Сборка бинарника
go build -ldflags "-X main.Version=$(git describe --tags --always)" -o bin/hunter cmd/hunter/main.go

# Запуск по расписанию (cron)
# См. configs/crontab.txt
```

## Документация

- [CHANGELOG](CHANGELOG.md)
- [PROJECT](PROJECT.md)
- [Полная документация](DOCUMENTATION.md)
- [Статус провайдеров](docs/providers-status.md)
- [Архитектура секретов](docs/secrets-architecture.md)
