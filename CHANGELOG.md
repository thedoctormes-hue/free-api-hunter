---
description: "free-api-hunter — история изменений"
type: changelog
last_reviewed: 2026-06-26
last_code_change: 2026-06-26
status: active
---

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **SQLite storage backend** — New `internal/database/` package with SQLite (modernc.org/sqlite) support, WAL mode, foreign keys, and auto-migration from JSON. JSON storage preserved as fallback.
- **TTS provider support** — New `TTSProvider` model with key pool and round-robin rotation (`internal/tts/keypool.go`). TTS API endpoints: `GET /api/v1/tts/providers`, `GET /api/v1/tts/providers/{id}`, `GET /api/v1/tts/stats`.
- **OCR.space provider** — Full OCR pipeline with mock HTTP tests (90.8% coverage).
- **Pollinations.ai integration** — Added as a provider.
- **Frontend theme toggle** — Light/dark mode via `web/src/contexts/theme.tsx`.
- **Dashboard charts** — Recharts-based visualizations: providers chart, findings-by-source chart, stats cards.
- **Provider filters, search, and export** — Frontend filtering by status/source, text search, and CSV export.
- **TTS keypool visualization** — TTS cards and stats components in web dashboard.
- **Error boundaries and skeletons** — React error boundaries and loading skeletons for better UX.
- **CI/CD via GitHub Actions** — Two workflows: CI (lint, test with race+coverage, build, Docker build) and Release (multi-platform builds on tag `v*`).
- **Docker support** — Dockerfile and docker-compose for containerized deployment.
- **Git hygiene standards** — gitleaks, pre-commit hooks, CONTRIBUTING.md, .gitignore updates.
- **API index page** — Root endpoint `/` now serves API documentation.

### Changed

- **Storage layer refactored** — `internal/storage/` now supports both JSON and SQLite backends. SQLite is the default when `internal/database` is initialized.
- **Frontend upgraded** — React 19, Tailwind CSS 4, modern stack with @tanstack/react-query, recharts, framer-motion.
- **Test coverage improved** — Key packages: filter 99.2%, models 100%, verifier 87.6%, vault 85.7%, storage 82.3%, orex 86.1%, alerter 92.3%, ocr 90.8%, tts 80.6%.

### Fixed

- **Vault permissions** — Changed from 0666 to 0600 for secret files.
- **IsDuplicate fix** — Corrected duplicate detection logic.
- **ParseFloat fix** — Fixed float parsing edge cases.
- **Model ID fallback** — Added fallback for missing model_id in provider data.
- **API key masking** — Secrets masked in all documentation and reports.

### Security

- Vault file permissions hardened (0600).
- API keys redacted from research docs and reports.
- gitleaks pre-commit hook prevents accidental secret commits.
- `.gitignore` updated to exclude sensitive files.

## [0.7.0] - 2026-06-21

### Added

- Full React frontend — dashboard, providers, findings, statistics pages.
- Web UI deployed at https://freeapihunter.shtab-ai.ru.
- SSL + nginx config with HTTPS on port 8443.
- TTS/STT provider support with key pool.
- Manus API integration and research.
- Provider status documentation and missing keys report.

### Changed

- Project overhaul — structure, security, tests, API, infra.

## [0.1.0] - 2026-06-14

### Added

- Initial release — Go CLI for free LLM API discovery.
- Scraper, filter, verifier, storage, alerter modules.
- JSON-based provider and finding storage.
- Telegram alert integration.
