# Архитектура веб-поиска для лаборатории

**Дата:** 2026-06-21
**Версия:** 1.1
**Статус:** протестировано, 8/8 тестов пройдено
**Авторы:** Бестия (Operator), консенсус агентов лаборатории

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

```
api-hub/
├── scripts/
│   ├── search-orchestrator.sh    # Основной оркестратор
│   ├── search-parallel.sh        # Параллельный поиск (Deep Research)
│   └── search-check-keys.sh      # Проверка всех 15 ключей
├── config/
│   ├── search-keys.yaml          # Конфигурация ключей (chmod 600)
│   └── .key-index-*              # State files для ротации
├── docs/
│   └── search-architecture.md    # Этот файл
├── tests/
│   └── test-providers.sh         # Тесты провайдеров
└── logs/
    ├── search-orchestrator.log
    └── search-parallel.log
```

## Использование

### Быстрый поиск (factual)
```bash
./scripts/search-orchestrator.sh "latest AI news" factual 5
```

### Скрапинг страницы
```bash
./scripts/search-orchestrator.sh "https://example.com" content
```

### JS-тяжёлый сайт
```bash
./scripts/search-orchestrator.sh "https://spa-app.com" dynamic
```

### Deep Research
```bash
./scripts/search-orchestrator.sh "OpenClaw architecture" deep_research 10
```

### Параллельный поиск
```bash
./scripts/search-parallel.sh "AI agent frameworks" 5
```

### Проверка ключей
```bash
./scripts/search-check-keys.sh
```

## Интеграция с агентами

Агенты вызывают скрипты через `exec`:

```bash
# Внутри агента:
exec("bash /root/LabDoctorM/projects/api-hub/scripts/search-orchestrator.sh 'query' factual 5")
```

Профили агентов (из deep-research-lab):
- **raven** (Researcher): deep_research — все провайдеры
- **dominika** (Scout): content + dynamic — Firecrawl + TinyFish
- **mangust** (Analyst): factual + content — Tavily + Firecrawl + SearXNG
- **streikbrecher** (Dev): factual + content — Tavily + Firecrawl GitHub
- **antcat** (Builder): factual + broad — Tavily + SearXNG
- **kotolizator** (Orch): factual — Tavily + SearXNG
- **bestia** (Operator): factual — Tavily + SearXNG
- **owl** (Auditor): factual + content — Tavily + SearXNG + Firecrawl

## Отказоустойчивость

**3 уровня надёжности:**

```
Уровень 1 (основной): api-hub оркестратор → 15 ключей, 4 провайдера, ротация
    ↓ при ошибке
Уровень 2 (fallback): нативный web_search (OpenClaw) → Tavily, 1 ключ
    ↓ при ошибке
Уровень 3 (аварийный): сообщить пользователю "Поиск недоступен"
```

- Агенты ВСЕГДА начинают с api-hub оркестратора
- Нативный web_search — только при явной ошибке оркестратора
- Мониторинг: cron-задача каждые 6 часов проверяет все 15 ключей
- При проблемах — алерт в Telegram

## Безопасность

- `search-keys.yaml` — chmod 600, в .gitignore
- Ключи НЕ передаются в промпты LLM
- Логи не содержат полных ключей (только номера и статусы)
- State files (`.key-index-*`) — только числовые индексы, в .gitignore
- Шаблон конфига: `search-keys.yaml.template` (без реальных ключей)
