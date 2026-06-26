---
name: Free API Hunter
owner: DoctorM&Ai
type: devtools
status: active
last_reviewed: 2026-06-26
last_code_change: 2026-06-26
priority: medium
stack: [Go]
version: "0.5.0"
---

# Free API Hunter

Поиск и мониторинг бесплатных API (LLM + OCR).

## Модули

- `internal/scraper/` — сбор находок из источников (GitHub, HN, Reddit, web)
- `internal/verifier/` — верификация LLM-ключей и страниц провайдеров
- `internal/ocr/` — верификация, скоринг и алерты OCR-провайдеров
- `internal/filter/` — фильтрация мусора и дедупликация
- `internal/alerter/` — Telegram-уведомления
- `internal/storage/` — сохранение результатов
- `internal/vault/` — безопасное хранение API-ключей

## OCR Pipeline

OCR.space интегрирован как отдельный провайдер:
- Верификация ключа через `/parse/image` API
- Скоринг: speed, quality, features, value
- 3 движка распознавания, 30+ языков
- Бесплатный тир: 25,000 запросов/мес
