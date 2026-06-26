---
description: "free-api-hunter — README"
type: readme
last_reviewed: 2026-06-26
last_code_change: 2026-06-26
status: active
---

# Free API Hunter

[![CI](https://github.com/DoctorM-Ai/free-api-hunter/actions/workflows/ci.yml/badge.svg)](https://github.com/DoctorM-Ai/free-api-hunter/actions/workflows/ci.yml)
[![Release](https://github.com/DoctorM-Ai/free-api-hunter/actions/workflows/release.yml/badge.svg)](https://github.com/DoctorM-Ai/free-api-hunter/actions/workflows/release.yml)

> **Владелец:** DoctorM&Ai | **Статус:** active | **Версия:** v0.7.0

## Веб-интерфейс

Дашборд доступен на **https://freeapihunter.shtab-ai.ru**

- Dashboard — обзор статистики, графики, последние находки
- Providers — каталог провайдеров с фильтрацией и поиском
- Findings — лента обнаруженных находок
- Statistics — детальная аналитика

Исходный код фронтенда: `web/` (React + TypeScript + Tailwind CSS)

## Описание

Поиск и мониторинг бесплатных LLM API — автоматический сбор, верификация и каталогизация бесплатных LLM эндпоинтов. Ищет бесплатные эндпоинты, проверяет их работоспособность, скорит качество находок.

## Быстрый старт

```bash
cd projects/free-api-hunter

# Сборка (через Makefile)
make build

# Запуск поиска (dry-run)
./hunter --dry-run --limit 20

# Верификация провайдеров
./hunter --verify

# Полный цикл
./hunter

# Конкретный источник
./hunter --source hackernews

# Без алертов
./hunter --no-alerts
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

## TTS/STT провайдеры

Free API Hunter поддерживает не только LLM, но и TTS/STT провайдеров (ElevenLabs, и др.).

**Отдельная модель данных:** `TTSProvider` в `internal/models/models.go` — метрики (chars, voice clones, audio tags) отличаются от LLM-метрик (RPM, TPM).

**Key Pool с ротацией:** `internal/tts/keypool.go` — round-robin между несколькими ключами, отслеживание расхода символов, автопометка исчерпанных.

```bash
# Ключи хранятся в vault:
echo -n "sk_ключ_1" > /root/LabDoctorM/vault/free-api-hunter/elevenlabs/api.key
echo -n "sk_ключ_2" > /root/LabDoctorM/vault/free-api-hunter/elevenlabs/api.key.1

# Верификация через пул (автоматическая ротация)
./bin/hunter --verify
```

**API:**
- `GET /api/v1/tts/providers` — список TTS-провайдеров
- `GET /api/v1/tts/providers/{id}` — провайдер по ID
- `GET /api/v1/tts/stats` — статистика

**Фронтенд:** страница `/tts` в дашборде.

## Разработка

```bash
# Тесты (через Makefile)
make test

# Линтер
make lint

# Вручную (без Makefile)
go test ./... -count=1 -v
go test ./... -cover
go test ./internal/filter/ -v
go vet ./...
```

## CI/CD

### GitHub Actions

| Workflow | Описание |
|----------|----------|
| **CI** (`.github/workflows/ci.yml`) | Запускается на push/PR: lint (golangci-lint), test (race + coverage), build, Docker build |
| **Release** (`.github/workflows/release.yml`) | Запускается на tag `v*`: сборка для linux/amd64, linux/arm64, darwin/amd64, загрузка в GitHub Release |

### Локальная разработка

```bash
# Сборка
make build

# Тесты с coverage
make test

# Линтер
make lint

# Docker образ
make docker

# Запуск API
make run

# Очистка
make clean
```

### Docker

```bash
# Сборка образа
docker build -t free-api-hunter:latest .

# Запуск контейнера
docker run -d -p 8090:8090 -v ./data:/app/data free-api-hunter:latest

# Или через docker-compose
docker compose up -d
```

## Деплой

### Backend (Go API)

```bash
# Сборка бинарника
go build -ldflags "-X main.Version=$(git describe --tags --always)" -o bin/hunter cmd/hunter/main.go

# Запуск API сервера
./bin/hunter --api :8090

# Запуск по расписанию (cron)
# См. configs/crontab.txt
```

### Frontend (React SPA)

```bash
cd web/
npm install
npm run build          # Сборка в dist/
cp -r dist/* /var/www/freeapihunter/
```

Nginx конфиг: `/etc/nginx/sites-enabled/freeapihunter.shtab-ai.ru`
- HTTPS на порту 8443 (Let's Encrypt)
- `/api/` проксируется на Go backend
- SPA роутинг через `try_files $uri $uri/ /index.html`

## Документация

- [CHANGELOG](CHANGELOG.md)
- [PROJECT](PROJECT.md)
- [Полная документация](DOCUMENTATION.md)
- [Статус провайдеров](docs/providers-status.md)
- [Архитектура секретов](docs/secrets-architecture.md)
