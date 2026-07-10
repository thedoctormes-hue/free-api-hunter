# Unified Search Gateway (SearXNG)

Self-hosted SearXNG как единая точка входа для веб-поиска лаборатории.
Агенты получают нативный поиск через кастомный MCP (`lab-search`), когда он будет разрешён.

## Архитектура

- **SearXNG** (image `searxng/searxng:latest`) на порту 8889.
- **5 платных пулов** (free-tier аккаунты), каждый ротирует ключи внутри одного движка:
  - `exa` (shortcut `exa`) — нейропоиск
  - `tavily` (`tav`) — LLM-ориентированный поиск
  - `firecrawl` (`fc`) — scrape + search
  - `tinyfish` (`tf`) — search API
  - `olostep` (`os`) — Web Data API (search/scrape/crawl)
- **Бесплатные built-in движки** (wiby, wikipedia, bing, seznam, mojeek, ...) — фоллбэк без ключей.

Всего в конфиге ~274 движка (built-in + 5 кастомных), реально рабочих по healthcheck ~26.

## Ротация ключей

Каждый engine-модуль (`searxng/engines/*.py`) читает `api_keys: [...]` из settings.yml
и выбирает следующий ключ при каждом запросе (round-robin через `threading.Lock`).
Лимиты всех аккаунтов провайдера суммируются и балансируются.

## Добавление ключей

```bash
bin/lab-search-gateway-addkey.sh add-key exa KEY1 KEY2 KEY3 KEY4 KEY5
```

Сохраняет ключи в `/root/.openclaw/.api-keys.json` (chmod 600) и обновляет
`api_keys:` в `searxng/settings.yml`.

## Деплой модулей

**Кастомные движки запечены в образ** (`Dockerfile.searxng` → `lab-searxng:local`),
поэтому они переживают `(re)создание контейнера` (`compose up -d`), pull образа
и перезагрузку хоста. Это основной, воспроизводимый путь.

После правки модуля движка — пересобрать образ:

```bash
docker compose -f docker-compose.searxng.yml build
docker compose -f docker-compose.searxng.yml up -d
```

`bin/lab-search-gateway-addkey.sh deploy` (hot-patch через `docker cp` + restart)
оставлен как быстрый путь без пересборки, НО он НЕ персистентен — при следующем
`up -d` правки исчезнут. Используйте только для экспериментов.

## Порт

SearXNG опубликован на **8889** (`0.0.0.0:8889→8080`). Порт 8080 уже занят
другим сервисом лаборатории (СнабЛаб / nginx) — НЕ используйте 8080 для SearXNG.
Каноничный URL для агентов/MCP: `http://localhost:8889/search` (env `SEARXNG_URL`).

## Healthcheck

```bash
bin/searxng-health.sh
```

Проверяет general + per-engine (exa/tavily/firecrawl/tinyfish/olostep) — каждый должен вернуть >0 результатов.

## Добавление нового провайдера

1. Написать `searxng/engines/<name>.py` по паттерну существующих:
   - `about` dict + `init()` + `request()` + `response()`
   - `api_keys: list[str] = []`, `_next_key()` — round-robin
   - `from searx.result_types import EngineResults`; `res.add(res.types.MainResult(...))`
2. Добавить блок в `settings.yml` (`name`, `engine`, `shortcut`, `api_keys: [...]`, `categories`).
3. Пересобрать образ и пересоздать контейнер:
   ```bash
   docker compose -f docker-compose.searxng.yml build
   docker compose -f docker-compose.searxng.yml up -d
   ```
   (либо быстрый hot-patch: `bin/lab-search-gateway-addkey.sh deploy` — не персистентно)
4. Проверить: `curl "http://localhost:8889/search?q=test&format=json&engines=<name>"`

## Секреты

`searxng/settings.yml` содержит ключи API → **gitignored**. Не коммитить.
Центральный keystore: `/root/.openclaw/.api-keys.json` (chmod 600).

## Статус

- Коммиты: `48d7a03` (пулы), `9558b48` (ротация внутри движка), `010e86b` (olostep),
  `e943748` (fix healthcheck: curl→python3 в образе нет), `655b16d` (персистентный
  деплой: движки запечены в образ через Dockerfile).
- Движки персистентны (больше не теряются при `up -d`).
- MCP `lab-search` НЕ зарегистрирован (ждёт разрешения ЗавЛаба).
- **Известный дефект (не закрыт):** в движках нет failover по ключам — один
  протухший ключ suspend-ит весь движок. Требует доработки (double-request pattern).
