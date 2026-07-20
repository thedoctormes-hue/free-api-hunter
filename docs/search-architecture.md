# Архитектура веб-поиска для лаборатории

**Дата:** 2026-06-21
**Версия:** 1.1
**Статус:** протестировано, 8/8 тестов пройдено
**Авторы:** Бестия (Operator), консенсус агентов лаборатории

> ⚠️ **КАНОНИЧНЫЙ ПУТЬ ВЕБ-ПОИСКА / DEEP RESEARCH ДЛЯ АГЕНТОВ — MCP-СЕРВЕР `searxng-gateway`.**
> Агент использует ТОЛЬКО тулы `searxng-gateway__search_web` и `searxng-gateway__deep_research`.
> Запрещено:
> - запускать shell-скрипты (`search-orchestrator.sh`, `search-parallel.sh`, `search-check-keys.sh`) через `exec`;
> - прямой вызов Tavily/SearXNG по API (curl/http, порты 8081/8082/8087/8100);
> - нативный `web_search` / `memory_search` OpenClaw;
> - отменённый MCP `lab-search` и `scripts/lab_search.py`.
> Скрипты-оркестраторы ниже — это внутренний backend MCP-сервера `searxng-gateway`, агенту недоступны напрямую.

## Обзор

Система веб-поиска для лаборатории LabDoctorM. Использует 4 провайдера с 15 API-ключами для максимальной пропускной способности и надёжности.

## Архитектура

```
Запрос → Классификатор типа задачи
    │
    ├── factual (факты, новости) → Tavily (ротация 5 ключей)
    ├── content (контент страницы) → Firecrawl scrape (ротация 5 ключей)
    ├── dynamic (JS/SPA сайты) → TinyFish fetch (ротация 5 ключей)
    ├── broad (метапоиск) → SearXNG (бесконечный)
    ├── deep_research → ВСЕ 4 провайдера параллельно + дедуп
    └── fallback → SearXNG (если всё упало)
```

## Провайдеры

### Tavily (5 ключей)
- **Сила:** AI-synthesized ответы, структурированные данные
- **Лимит:** 1000 кредитов/мес на ключ = 5000 всего
- **Rate limit:** 100 req/min
- **Когда использовать:** факты, новости, быстрые ответы
- **Auth:** `api_key` в JSON body

### Firecrawl (5 ключей)
- **Сила:** Полный скрапинг страниц, markdown, batch, GitHub search
- **Лимит:** 1000 кредитов/мес на ключ = 5000 всего
- **Rate limit:** 50 req/min
- **Когда использовать:** глубокий контент, статьи, документация
- **Auth:** `Authorization: Bearer`

### TinyFish (5 ключей)
- **Сила:** JS-рендеринг, бот-обход, SPA
- **Лимит:** бесплатно (Search + Fetch)
- **Rate limit:** 60 req/min (search), 300 url/min (fetch)
- **Когда использовать:** JS-тяжёлые сайты, динамический контент
- **Auth:** `X-API-Key` header

### SearXNG (локальный)
- **Сила:** Метапоиск 70+ движков, без лимитов
- **Лимит:** бесконечный
- **Rate limit:** нет
- **Когда использовать:** широкий поиск, fallback, кросс-проверка
- **Auth:** не нужна

## Ротация ключей

**Цклическая per-провайдер** — каждый новый запрос использует следующий ключ по кругу.

**Зачем:** маскировка мультиаккаунта. Провайдер видит запросы с разных ключей, не может связать их в один аккаунт.

**Алгоритм:**
1. Каждый запрос → `get_next_key()` → следующий ключ по кругу (0→1→2→3→4→0→...)
2. При 429/ошибке → пропустить ключ, попробовать следующий (до 5 попыток)
3. Все 5 ключей исчерпаны → fallback на SearXNG
4. State хранится в `config/.key-index-{provider}` (только числовой индекс)

**Пример (Tavily, 5 ключей):**
```
Запрос 1 → key[0]  ✓
Запрос 2 → key[1]  ✓
Запрос 3 → key[2]  ✓
Запрос 4 → key[3]  ✓
Запрос 5 → key[4]  ✓
Запрос 6 → key[0]  ✓  (цикл)
```

## Структура файлов

> Все перечисленные скрипты — **внутренний backend** MCP-сервера `searxng-gateway`.
> Агент их НЕ вызывает напрямую (см. раздел «Использование»).

```
free-api-hunter/
├── scripts/
│   ├── search-orchestrator.sh    # Основной оркестратор (backend)
│   ├── search-parallel.sh        # Параллельный поиск (Deep Research, backend)
│   └── search-check-keys.sh      # Проверка всех 15 ключей (backend)
├── config/
│   ├── search-keys.yaml          # Конфигурация ключей (chmod 600)
│   └── .key-index-*              # State files для ротации
├── docs/
│   └── search-architecture.md    # Этот файл
├── tests/
│   └── test-providers.sh         # Тесты провайдеров (backend)
└── logs/
    ├── search-orchestrator.log
    └── search-parallel.log
```

## Использование (каноничный путь: MCP `searxng-gateway`)

> Агент вызывает MCP-тулы сервера `searxng-gateway`. Никаких `exec` к скриптам,
> никакого прямого Tavily/SearXNG API. Маршрутизация по провайдерам, ротация ключей
> и fallback реализованы внутри gateway/бэкенда.

### Быстрый веб-поиск (factual / broad / content / dynamic)
```
searxng-gateway__search_web(query="latest AI news", max_results=5)
```

### Deep Research (веб + синтез + семантическая память лабы)
```
searxng-gateway__deep_research(query="OpenClaw architecture", count=10)
```

### Проверка доступности поиска (только алерт/мониторинг инфраструктуры)
Осуществляется скриптом `bin/searxng-health.sh`, а НЕ агентом. Из агента
вызывать не нужно.

## Интеграция с агентами

Агенты вызывают MCP-тулы `searxng-gateway__search_web` / `searxng-gateway__deep_research`
(сервер `searxng-gateway`). Пример вызова из агента:

```json
{ "tool": "searxng-gateway__search_web", "arguments": { "query": "latest AI news", "max_results": 5 } }
```

Профили агентов (из deep-research-lab) отражают ТОЛЬКО предпочтительный ТИП запроса,
но ВСЕ они идут через один каноничный MCP `searxng-gateway`:
- **raven** (Researcher): deep_research — все провайдеры
- **dominika** (Scout): content + dynamic
- **mangust** (Analyst): factual + content
- **streikbrecher** (Dev): factual + content
- **antcat** (Builder): factual + broad
- **kotolizator** (Orch): factual
- **bestia** (Operator): factual
- **owl** (Auditor): factual + content

> Запрещено: вызывать `search-orchestrator.sh`/`search-parallel.sh`/`search-check-keys.sh`
> через `exec`, использовать нативный `web_search`/`memory_search`, отменённый MCP `lab-search`,
> `scripts/lab_search.py` или прямой Tavily/SearXNG API. Все эти пути — dead/отменены.

## Отказоустойчивость

**Единый каноничный путь — MCP `searxng-gateway`.** Маршрутизация по провайдерам,
ротация ключей и fallback (Tavily→Firecrawl→TinyFish→SearXNG) реализованы ВНУТРИ
backend-а gateway. Агент об этом не знает и не управляет им.

```
Уровень 1 (каноничный): MCP searxng-gateway → search_web / deep_research
    ↓ при ошибке (degraded) — gateway сам делает fallback по провайдерам
Уровень 2 (аварийный): gateway возвращает {degraded:true} / ошибку →
    агент сообщает пользователю, что поиск недоступен, и НЕ использует
    неканоничные обходные пути (прямой API, нативный web_search, lab-search).
```

- Агент ВСЕГДА начинает с MCP `searxng-gateway`.
- При `degraded:true` — результат всё ещё полезен (частичные данные).
- При полной ошибке — сообщить пользователю «Поиск недоступен», без обходных путей.
- Мониторинг здоровья поиска: `bin/searxng-health.sh` (инфраструктура, не агент).
- При проблемах — алерт в Telegram.

## Безопасность

- `search-keys.yaml` — chmod 600, в .gitignore
- Ключи НЕ передаются в промпты LLM
- Логи не содержат полных ключей (только номера и статусы)
- State files (`.key-index-*`) — только числовые индексы, в .gitignore
- Шаблон конфига: `search-keys.yaml.template` (без реальных ключей)
