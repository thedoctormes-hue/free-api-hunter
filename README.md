---
description: "free-api-hunter — README"
type: readme
last_reviewed: 2026-06-21
last_code_change: 2026-06-21
status: active
---

# Free API Hunter

> **Владелец:** DoctorM&Ai | **Статус:** active

## Описание

Поиск и мониторинг бесплатных API — автоматический сбор и проверка доступности бесплатных LLM и других API. Ищет бесплатные эндпоинты, проверяет их работоспособность и каталогизирует.

## Быстрый старт

```bash
cd projects/free-api-hunter

# Установка
pip install -r requirements.txt

# Запуск поиска
python3 src/hunter.py

# Проверка доступных API
python3 src/checker.py
```

## Архитектура

**Стек:** Python 3.10+, requests, asyncio

**Компоненты:**
- `hunter.py` — поиск бесплатных API по каталогам и источникам
- `checker.py` — проверка работоспособности найденных эндпоинтов
- `catalog.json` — каталог найденных API

## Разработка

```bash
# Тесты
python3 -m pytest tests/ -v

# Линтер
ruff check src/
```

## Деплой

```bash
# Запуск по расписанию (cron)
python3 src/hunter.py --schedule
```

## Документация

- [CHANGELOG](CHANGELOG.md)
- [Тесты](tests/)
