# Free API Hunter 🐦‍⬛

Автоматизированный мониторинг бесплатных LLM API-ключей и кредитов.
Находит халяву, фильтрует мусор, приносит профит.

**Разработка:** Штрейкбрехер (Go) + Ворон (рекон/аналитика)
**Версия:** v0.6.0

## Что делает

- Сканирует источники: GitHub awesome-листы, Hacker News, официальные страницы провайдеров
- Фильтрует мусор: дедупликация (fingerprint + URL), спам-ключевые слова, trash-источники, порог качества, возраст находок
- Верифицирует провайдеров: проверяет работоспособность ссылок и наличие бесплатного тира
- Хранит базу провайдеров и пул рабочих ключей в JSON
- Алерты в Telegram через vault-конфигурацию
- Мерж статусов: config (source of truth) + runtime data

## Технологии

- **Язык:** Go 1.23 (stdlib only, 0 внешних зависимостей)
- **Хранение:** JSON-файлы + lab-vault для секретов
- **Планировщик:** cron (crontab.txt)
- **Алерты:** Telegram Bot API (конфиг через vault)
- **Vault:** lab-vault (`/root/LabDoctorM/vault/free-api-hunter/`)

## Структура

```
projects/
├── README.md                    # этот файл
├── DOCUMENTATION.md             # полная документация
├── go.mod / go.sum              # Go модуль
├── bin/hunter                   # собранный бинарник
├── config/
│   ├── sources.json             # источники + провайдеры (source of truth)
│   ├── filters.json             # фильтры (keywords, domains, thresholds)
│   ├── alerter.json             # Telegram config (fallback)
│   └── crontab.txt              # cron-задачи
├── data/
│   ├── providers.json           # база провайдеров (runtime cache)
│   ├── findings.json            # находки после фильтрации
│   └── key_pool.json            # пул рабочих ключей
├── cmd/hunter/
│   └── main.go                  # точка входа, CLI, оркестрация
└── internal/
    ├── models/                  # Provider, Finding, APIKey, Priority, Status
    ├── scraper/                 # сбор данных (GitHub, HN, web)
    ├── filter/                  # фильтрация, дедуп, скоринг
    ├── verifier/                # верификация провайдеров
    ├── storage/                 # хранение в JSON
    ├── vault/                   # интеграция с lab-vault
    ├── alerter/                 # Telegram алерты (vault-first)
    └── orex/                    # Orex integration (OpenRouter Expert)
```

## Запуск

```bash
# Сборка с версионированием
go build -ldflags "-X main.Version=$(git describe --tags --always)" -o bin/hunter cmd/hunter/main.go

# Версия
./bin/hunter --version

# Сухой прогон (без сохранения)
./bin/hunter --dry-run

# Полный цикл
./bin/hunter

# С верификацией
./bin/hunter --verify

# Конкретный источник
./bin/hunter --source hackernews

# Без алертов
./bin/hunter --no-alerts

# Orion Scan — ежедневная проверка бесплатных моделей OpenRouter
./orion-scan.sh
```

## CLI Flags

| Flag | Default | Описание |
|------|---------|----------|
| `--dry-run` | false | Не сохранять результаты |
| `--source` | "" | Сканировать только конкретный источник |
| `--verify` | false | Верифицировать провайдеров |
| `--limit` | 10 | Лимит находок для вывода |
| `--no-alerts` | false | Не отправлять алерты в Telegram |
| `--alert-config` | config/alerter.json | Путь к конфигу алертов |
| `--version` | false | Показать версию и выйти |

## Статус провайдеров

| Статус | Значение | Кол-во |
|--------|----------|--------|
| verified | ЗавЛаб лично проверил | 1 |
| confirmed | Подтверждён сканером | 1 |
| claimed | Найден, не проверен | 12 |
| expired | Не работает | 3 |

### Активные провайдеры (14)

| Провайдер | Модели | Лимиты | Статус |
|-----------|--------|--------|--------|
| OpenRouter | 10 бесплатных | 50 req/day (без кредитов), 20 RPM | claimed |
| Groq | 5 моделей | 14400 req/day, 30 RPM | claimed |
| Cloudflare Workers AI | 4 модели | 10K neurons/day | claimed |
| Cohere | 20 моделей | 1000 calls/month | claimed |
| Google AI Studio (Gemini) | 3+ модели | 1500 RPM, 1500 RPD, 1M TPM | claimed |
| Cerebras | 2 модели | 5 RPM / 30K tokens/min | claimed |
| Z.ai (GLM) | 8 моделей | 1000 req/day, 3 RPM | claimed |
| GitHub Models | — | — | claimed |
| Kilo Gateway | — | — | confirmed |
| Pollinations | — | — | claimed |
| OpenCode Zen | — | — | claimed |
| NVIDIA NIM | — | — | claimed |
| Together AI | — | — | claimed |
| Manus | — | — | claimed |

### Истёкшие (3)

| Провайдер | Причина |
|-----------|---------|
| Mistral | Ключ истёк |
| OVH AI Endpoints | Сервис закрыт |
| HuggingFace Inference API | Бесплатный тир урезан |

## Источники сканирования

| Источник | Тип | Статус | Находки |
|----------|-----|--------|---------|
| Hacker News | hackernews | ✅ | 7 |
| awesome-free-llm-apis | github_raw | ✅ | 42 |
| free-llm-api-keys | github_raw | ✅ | 77 |
| CostGoat OpenRouter | web_page | ✅ | 2 |
| GetAIPerks Blog | web_page | ✅ | 2 |
| Reddit r/LocalLLaMA | reddit | ❌ Отключён (403) | — |
| Reddit r/MachineLearning | reddit | ❌ Отключён (403) | — |

**Reddit 403:** Reddit блокирует скрейперы без прокси. Решения: HTTP_PROXY или Reddit OAuth.

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

## Фильтрация

Все настройки фильтрации в `config/filters.json`:

- **Дедупликация:** fingerprint + URL uniqueness
- **Спам-фильтры:** ключевые слова, исключённые домены, trash-источники
- **Пороги качества:** мин. длина описания, требование URL, макс. возраст
- **Исключённые провайдеры:** kilo gateway, kilochat, kilo
- **Скоринг:** по длине описания, упоминанию моделей, лимитов, URL документации

## Хранение ключей

Ключи хранятся в **lab-vault**: `/root/LabDoctorM/vault/free-api-hunter/`

```
vault/free-api-hunter/
├── cohere/
│   ├── api.key
│   ├── api.key.2
│   ├── api.key.3
│   └── api.key.4
└── telegram_bot_token.key    (не настроен)
```

**Принципы:**
- Ключи НЕ хранятся в коде
- Ключи НЕ коммитятся в git
- Права 600 на файлах
- Alerter загружает из vault-first, fallback на config/alerter.json

## Добавление нового провайдера

1. Создать директорию в vault: `mkdir -p /root/LabDoctorM/vault/free-api-hunter/{provider}`
2. Записать ключ: `echo -n "API_KEY" > /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key`
3. Установить права: `chmod 600 /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key`
4. Добавить провайдера в `config/sources.json`
5. Запустить верификацию: `./bin/hunter --verify`

## Cron-задачи

```bash
# Полный скан с верификацией каждые 6 часов
0 */6 * * * cd /root/LabDoctorM/projects/free-api-hunter && ./bin/hunter --verify --no-alerts >> /var/log/free-api-hunter/scan.log 2>&1

# Ежедневный dry-run для алерта (08:00 UTC)
0 8 * * * cd /root/LabDoctorM/projects/free-api-hunter && ./bin/hunter --dry-run >> /var/log/free-api-hunter/report.log 2>&1

# Быстрый скан без верификации каждые 3 часа
0 */3 * * * cd /root/LabDoctorM/projects/free-api-hunter && ./bin/hunter --no-alerts >> /var/log/free-api-hunter/quick-scan.log 2>&1
```

## Тестирование

```bash
# Все тесты
go test ./... -count=1

# С покрытием
go test ./... -cover

# Конкретный пакет
go test ./internal/filter/ -v
```

## План развития

### v0.7 (следующая)
- [ ] Reddit OAuth / прокси для разблокировки
- [ ] Миграция Cohere на /v2/chat (v1 deprecated)
- [ ] Интеграция с lab-vault API (HTTP) вместо файлового доступа
- [ ] Тесты для cmd/hunter (integration)

### v0.8 (продакшен)
- [ ] Алерты в Telegram (настройка через vault)
- [ ] Автоматическая ротация ключей
- [ ] Мониторинг изменений в бесплатных тирах

### v0.9 (интеллект)
- [ ] Подбор модели под задачу
- [ ] Бенчмарк скорости бесплатных моделей
- [ ] Дашборд мониторинга

## Лицензия

Для внутреннего использования лаборатории.
