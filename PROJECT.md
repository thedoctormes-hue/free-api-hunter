---
name: Free API Hunter
owner: DoctorM&Ai
type: devtools
status: active
last_reviewed: 2026-07-12
last_code_change: 2026-07-12
priority: medium
stack: [Go]
version: "0.5.0"
---

# Free API Hunter

Поиск и мониторинг бесплатных API (LLM + OCR).

## Модули

- `internal/models/` — доменные модели (APIKey, KeyRecord, EndpointConfig)
- `internal/vault/` — безопасное хранение API-ключей (1 файл = 1 ключ; мульти-ключ файлы split построчно)
- `internal/verifier/` — верификация LLM-ключей (per-provider probe: bearer/query/header) и страниц провайдеров
- `internal/validator/` — KRV-Validator: живая валидация ключей из vault → `live_status` (valid/unknown/expired/rate_limited); мульти-ключ (`key_id#N`); PAT-005 гейтинг неизвестных эндпоинтов как `unknown`
- `internal/scraper/` — сбор находок из источников (GitHub, HN, Reddit, web)
- `internal/ocr/` — верификация, скоринг и алерты OCR-провайдеров (OCR.space)
- `internal/filter/` — фильтрация мусора и дедупликация
- `internal/alerter/` — Telegram-уведомления
- `internal/storage/` — сохранение результатов (SQLite)
- `internal/database/` — слой доступа к БД
- `internal/keydrop/` — drop-zone: сырые ключи → vault → Registry (Yandex Disk)
- `internal/notify/` — ежедневные уведомления (discovery → pending_review.json)
- `internal/api/` — HTTP API сервер (флаг `-api`)
- `internal/cf/` — Cloudflare Workers AI аккаунты (флаг `-cf-verify`)
- `internal/pollinations/` — Pollinations провайдер
- `internal/orex/` — вспомогательная оркестрация провайдеров
- `internal/output/` — форматирование вывода
- `internal/securego/` — безопасная обёртка исходящих HTTP-запросов
- `internal/tts/` — TTS провайдер
- `internal/webhooks/` — webhook-уведомления
- `cmd/hunter/` — CLI entrypoint: `scan`, `validate-keys`, `notify`, `triage-*`

## OCR Pipeline

OCR.space интегрирован как отдельный провайдер:
- Верификация ключа через `/parse/image` API
- Скоринг: speed, quality, features, value
- 3 движка распознавания, 30+ языков
- Бесплатный тир: 25,000 запросов/мес
