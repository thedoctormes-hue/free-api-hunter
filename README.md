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
- TTS — мониторинг TTS провайдеров и статистики ключей

Исходный код фронтенда: `web/` (React + TypeScript + Tailwind CSS)

## Описание

Поиск и мониторинг бесплатных LLM API — автоматический сбор, верификация и каталогизация бесплатных LLM эндпоинтов. Ищет бесплатные эндпоинты, проверяет их работоспособность, скорит качество находок.

## Быстрый старт

```bash
cd projects/free-api-hunter

# Сборка проекта
make build

# Запуск сервера (API + Index)
make run

# Запуск поиска (dry-run)
./hunter --dry-run --limit 20

# Верификация провайдеров
./hunter --verify

# Верификация Cloudflare Workers AI
./hunter --cf-verify --cf-config config/cf_accounts.json

# Полный цикл
./hunter
```

## Архитектура

**Стек:** Go 1.23, SQLite (modernc.org/sqlite), стандартная библиотека + JSON-хранилище

**Структура:**
```
cmd/hunter/main.go          — CLI точка входа, оркестрация
internal/
  models/models.go           — модели данных (Provider, Finding, APIKey, TTSProvider)
  scraper/scraper.go         — сбор данных из источников
  filter/filter.go           — фильтрация находок, дедуп, скоринг
  verifier/verifier.go       — верификация ключей и провайдеров
  storage/storage.go         — хранилище (SQLite + JSON fallback)
  database/sqlite.go         — SQLite backend & migrations
  vault/vault.go             — безопасное хранение ключей (0600)
  orex/client.go             — Orex API client (OpenRouter Expert)
  alerter/alerter.go         — Telegram алерты
  tts/keypool.go             — ротация ключей для TTS
  cf/                        — Cloudflare Workers AI модуль
    client.go                — HTTP клиент для CF API
    keypool.go               — round-robin ротация аккаунтов
    verifier.go              — верификация ключей CF
    models.go                — модели данных
    client_test.go           — тесты клиента
    keypool_test.go          — тесты пула
configs/
  sources.json               — источники + провайдеры
  filters.json               — фильтры и скоринг
  alerter.json               — Telegram config (fallback)
data/                        — runtime cache (gitignored)
  free-api-hunter.db         — SQLite база данных
  providers.json             — JSON хранилище (legacy/fallback)
  findings.json              — находки сканера
```

**Поток данных:**
```
sources.json → scraper → []Finding (сырые)
    → filter (дедуп, спам, скоринг) → []Finding (очищенные)
    → verifier (проверка URL, free tier, credit card)
    → storage (SQLite / JSON)
    → alerter (Telegram, если настроен)
```

## Хранение ключей

Ключи хранятся в vault:
- LLM: `/root/LabDoctorM/vault/free-api-hunter/`
- Cloudflare: `/root/LabDoctorM/vault/cloudflare/{account_id}`

Формат: plaintext файлы с правами 600, одно значение на файл.
Ключи **никогда** не коммитятся в git.

```bash
# Добавить ключ
echo -n "API_KEY" > /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key
chmod 600 /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key
```

## Cloudflare Workers AI

Free API Hunter поддерживает 6 аккаунтов Cloudflare Workers AI с бесплатным лимитом 10,000 Neurons/день на аккаунт (60,000 Neurons/день при ротации).

**Архитектура:**
- `internal/cf/client.go` — HTTP-клиент для Cloudflare API (`/ai/v1/chat/completions`)
- `internal/cf/keypool.go` — round-robin ротация с учётом NeuronBudget
- `internal/cf/verifier.go` — верификация аккаунтов через тестовый запрос
- `internal/cf/models.go` — модели данных (Account, Model, NeuronBudget, ChatResponse)
- `config/cf_accounts.json` — конфигурация аккаунтов
- Ключи хранятся в `vault/cloudflare/{account_id}` (0600)

**80 доступных моделей** (4 модальности):
- Text Generation: GLM-5.2, Kimi K2.7 Code, GPT-oss-120B, Nemotron-3-120B, Llama 4 Scout, и др.
- Text Embeddings: BGE-M3, BGE-Small/Base/Large, Qwen3-Embedding-0.6B
- Text-to-Image: FLUX.2 Dev/Klein-4B/9B, Lucid Origin, Phoenix 1.0
- Audio TTS/ASR: Aura-1/2, Melotts, Whisper, Nova-3

```bash
# Верификация CF аккаунтов
./hunter --cf-verify --cf-config config/cf_accounts.json

# С алертом в Telegram
./hunter --cf-verify --cf-config config/cf_accounts.json --alert-config config/alerter.json
```

**Важно:** user ID = Account ID для Cloudflare Workers AI API.

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
- [API Reference](docs/API.md)
