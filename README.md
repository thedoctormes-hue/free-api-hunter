# Free API Hunter 🐦‍⬛

Автоматизированный мониторинг бесплатных LLM API-ключей и кредитов.
Находит халяву, фильтрует мусор, приносит профит.

**Разработка:** Штрейкбрехер (Go) + Ворон (рекон/аналитика)

## Что делает

- Сканирует источники: Reddit, GitHub awesome-листы, Hacker News, официальные страницы провайдеров
- Фильтрует мусор: дедупликация, порог полезности, spam-фильтр, changelog-фильтр
- Верифицирует провайдеров: проверяет работоспособность ссылок и наличие бесплатного тира
- Хранит базу провайдеров и пул рабочих ключей в JSON
- Алерты в Telegram при новых находках (планируется)

## Технологии

- **Язык:** Go
- **Хранение:** JSON-файлы (SQLite в планах)
- **Планировщик:** systemd timers / cron
- **Алерты:** Telegram Bot API
- **Прокси:** yandex.sh для обхода блокировок

## Структура

```
projects/
├── README.md                    # этот файл
├── go.mod                       # Go модуль
├── hunter                       # собранный бинарник
├── config/
│   ├── sources.json             # источники для сканирования (v0.3.0)
│   └── filters.json             # настройки фильтрации
├── data/
│   ├── providers.json           # база провайдеров (v0.4.0, 17 провайдеров)
│   ├── findings.json            # находки после фильтрации (генерируется)
│   └── key_pool.json            # пул рабочих ключей (генерируется)
├── cmd/hunter/
│   └── main.go                  # точка входа, CLI
└── internal/
    ├── models/
    │   ├── models.go            # Provider, Finding, APIKey, Priority, Status
    │   └── models_test.go       # тесты моделей
    ├── scraper/
    │   ├── scraper.go           # сбор данных (Reddit, GitHub, HN, web)
    │   └── scraper_test.go      # тесты скрапера
    ├── filter/
    │   ├── filter.go            # фильтрация, дедуп, скоринг
    │   └── filter_test.go       # тесты фильтра
    ├── verifier/
    │   ├── verifier.go          # верификация провайдеров
    │   └── verifier_test.go     # тесты верификатора
    ├── storage/
    │   ├── storage.go           # хранение в JSON
    │   └── storage_test.go      # тесты хранилища
    └── vault/
        ├── vault.go             # интеграция с lab-vault
        └── vault_test.go        # тесты vault
```

## Запуск

```bash
# Сборка
go build -o hunter ./cmd/hunter

# Сухой прогон (без сохранения)
./hunter --dry-run

# Полный цикл
./hunter

# С верификацией
./hunter --verify

# Конкретный источник
./hunter --source hackernews
```

## Статус провайдеров

| Статус | Значение | Кол-во |
|--------|----------|--------|
| verified | ЗавЛаб лично проверил | 8 |
| confirmed | Подтверждён сканером | 6 |
| deprioritized | Низкий приоритет | 3 |

### Верифицированные (✅ работают)

- **OpenRouter** — 27 бесплатных моделей, 200 req/day
- **Groq** — 5 моделей, 14400 req/day
- **Cloudflare Workers AI** — edge inference, 10K neurons/day
- **Manus** — ключ активен
- **Cohere** — 1000 calls/month
- **Google AI Studio** — ключ валидный, квота исчерпана
- **Cerebras** — 5 RPM / 30K tokens/min
- **Mistral** — 50 RPM / 500K tokens/min

### Подтверждённые сканером

- **Z.ai (GLM)** — 3 бесплатные Flash-модели, 1000 req/day
- **GitHub Models** — нужен GitHub аккаунт
- **Kilo Gateway** — анонимный доступ
- **Pollinations** — GPT-OSS 20B
- **OVH AI Endpoints** — Qwen3.5 397B
- **OpenCode Zen** — DeepSeek V4 Flash

### Деприоритизированные

- **NVIDIA NIM** — ToS: evaluation only
- **HuggingFace** — бесплатный тир урезан
- **Together AI** — только Apriel бесплатно

## Источники сканирования

| Источник | Тип | Статус | Находки |
|----------|-----|--------|---------|
| Reddit r/LocalLLaMA | reddit | ⚠️ 403 | — |
| Reddit r/MachineLearning | reddit | ⚠️ 403 | — |
| Hacker News | hackernews | ✅ | 7 |
| awesome-free-llm-apis | github_raw | ✅ | 42 |
| free-llm-api-keys | github_raw | ✅ | 77 |
| CostGoat OpenRouter | web_page | ✅ | 2 |
| GetAIPerks Blog | web_page | ✅ | 2 |

**Reddit 403:** Reddit блокирует без User-Agent/прокси. Решения:
1. Использовать `HTTP_PROXY` (yandex.sh)
2. Reddit OAuth (в планах)

## Модель данных

### ProviderStatus

```
verified       — ЗавЛаб лично проверил, ключ работает
confirmed      — Подтверждён сканером/документацией
claimed        — Найден, но не проверен
expired        — Не работает / тир закончился
deprioritized  — Работает, но низкий приоритет
unverified     — Статус неизвестен
```

### Priority

```
PriorityHigh (1)  — verified/confirmed, максимальный приоритет
PriorityMed  (2)  — claimed, средний
PriorityLow  (3)  — deprioritized/unverified
PrioritySkip (9)  — credit card required, пропускать
```

## Orex Integration

Orex (OpenRouter Expert) — внешний сервис `http://127.0.0.1:8710`, предоставляющий каталог моделей, цены и алерты.

### Использование

```bash
# Синхронизация с Orex перед сканированием
./hunter --orex-sync

# Полный цикл с Orex + верификация
./hunter --orex-sync --verify
```

### Что делает --orex-sync

1. Вызывает `Orex.Sync()` — обновляет базу моделей
2. Загружает бесплатные модели (`Orex.GetFreeModels()`)
3. Сохраняет кэш в `data/orex_cache.json`
4. Объединяет данные с локальными провайдерами (`MergeOrexProviders`)
5. Отправляет алерт о новых моделях (если настроен Telegram)

### Быстрая верификация

Если Orex подтверждает что модель бесплатная (`pricing.prompt=0, pricing.completion=0`), верификатор пропускает тяжёлый HTML-парсинг страницы провайдера и сразу ставит статус `confirmed`.

### Структура данных

```
data/
├── providers.json      # локальные провайдеры
├── findings.json       # находки сканера
├── key_pool.json       # пул ключей
└── orex_cache.json     # кэш Orex (бесплатные модели)
```

## План развития

### v0.5 (текущая итерация)
- [ ] Исправить Reddit 403 (OAuth или прокси)
- [ ] Дете GitHub README парсинг (убрать changelog-мусор)
- [ ] Добавить Twitter/X источник
- [ ] Верификация всех confirmed провайдеров

### v0.6 (продакшен)
- [ ] Интеграция с lab-vault для хранения ключей
- [ ] Алерты в Telegram
- [ ] systemd timer для автоматического сканирования
- [ ] Бенчмарк скорости бесплатных моделей

### v0.7 (интеллектуальная система)
- [ ] Подбор модели под задачу
- [ ] Мониторинг изменений в бесплатных тирах
- [ ] Автоматическая ротация ключей
- [ ] Дашборд мониторинга

## Лицензия

Для внутреннего использования лаборатории.
