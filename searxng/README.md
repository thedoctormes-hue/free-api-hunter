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

При `(re)создании контейнера` (`compose up -d`) кастомные модули теряются — они в слое
контейнера, не в volume. Восстановить:

```bash
bin/lab-search-gateway-addkey.sh deploy
```

Копирует `searxng/engines/{exa,tavily,firecrawl,tinyfish,olostep}.py` в контейнер,
рестартит SearXNG, прогоняет healthcheck.

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
3. `bin/lab-search-gateway-addkey.sh deploy`
4. Проверить: `curl "http://localhost:8889/search?q=test&format=json&engines=<name>"`

## Секреты

`searxng/settings.yml` содержит ключи API → **gitignored**. Не коммитить.
Центральный keystore: `/root/.openclaw/.api-keys.json` (chmod 600).

## Статус

- Коммиты: `48d7a03` (пулы), `9558b48` (ротация внутри движка), `010e86b` (olostep).
- MCP `lab-search` НЕ зарегистрирован (ждёт разрешения ЗавЛаба).
