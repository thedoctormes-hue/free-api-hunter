# Free API Hunter — Документация

## Обзор

Free API Hunter — система для обнаружения, верификации и мониторинга бесплатных LLM API.

**Версия:** v0.5.0 (Orex integration)
**Разработчик:** Штрейкбрехер
**Репозиторий:** https://github.com/thedoctormes-hue/free-api-hunter

## Архитектура

```
cmd/hunter/main.go          — CLI точка входа
internal/
  models/models.go           — модели данных (Provider, Finding, APIKey)
  scraper/scraper.go         — сбор данных из источников
  filter/filter.go           — фильтрация находок
  verifier/verifier.go       — верификация ключей и провайдеров
  storage/storage.go         — JSON хранилище
  vault/vault_client.go      — безопасное хранение ключей
config/
  sources.json               — источники + провайдеры
  filters.json               — фильтры
data/
  providers.json             — база провайдеров
  findings.json              — находки
  key_pool.json              — пул ключей
```

## Хранение ключей

Ключи хранятся в **lab-vault** — `/root/LabDoctorM/vault/free-api-hunter/`

Структура:
```
vault/free-api-hunter/
  cerebras/api.key
  mistral/primary.key
  mistral/secondary.key
  cloudflare/api.key
  cohere/api.key
  gemini/api.key
  zai/api.key
  manus/api.key
  tavily/api.key
```

**Принципы:**
- Ключи НЕ хранятся в коде
- Ключи НЕ коммитятся в git
- Доступ только через vault API
- Права 600 на файлах

## Работа с ключами

### Загрузка ключа

```go
import "free-api-hunter/internal/vault"

key, err := vault.GetDefaultKey("cerebras")
if err != nil {
    log.Fatal(err)
}
// key.Value содержит API ключ
```

### Список провайдеров

```go
providers, err := vault.ListProviders()
// []string{"cerebras", "mistral", "cloudflare", ...}
```

### Проверка наличия ключа

```go
if vault.HasKey("mistral", "primary") {
    // ключ существует
}
```

## Orion Scan — ежедневный сканер бесплатных моделей OpenRouter

Скрипт `orion-scan.sh` — это инструмент для автоматического сканирования и верификации бесплатных моделей OpenRouter с контекстом ≥128K токенов. Запускается ежедневно по крону.

### Как работает

1. **Получает список всех моделей** из OpenRouter API
2. **Фильтрует** бесплатные модели (prompt = 0, completion = 0) с контекстом ≥128K
3. **Проверяет каждую модель** на живучесть, отправляя тестовый запрос
4. **Формирует отчёт** с группировкой по размеру контекста и статусом reasoning

### Формат вывода

Отчёт содержит:
- 📊 Общее количество найденных/работающих/не работающих моделей
- 🥇 Гигант-модели (1M+ контекст)
- 🥈 Средние (256K-512K контекст)  
- 🥉 Компактные (128K-256K контекст)
- ✅/❌ Статус каждой модели с указанием причины ошибки

### Тесты

Скрипт покрыт bash-тестами (`orion-scan_test.sh`), которые проверяют:
- Парсинг JSON ответа OpenRouter
- Фильтрацию по цене и контексту
- Группировку по размеру контекста
- Определение наличия reasoning
- Форматирование вывода
- Обработку边界 случаев

Запуск тестов: `bash orion-scan_test.sh`

## Проверенные провайдеры

### Работающие (27 моделей)

| Провайдер | Модели | Лимиты | Статус |
|-----------|--------|--------|--------|
| Cerebras | gpt-oss-120b, zai-glm-4.7 | 5 RPM / 30K tok/min | ✅ |
| Mistral | 11 моделей | 50 RPM / 500K tok/min | ✅ |
| Cloudflare | 4 модели | 10K neurons/день | ✅ |
| Cohere | 6 моделей | 1000 calls/мес, 20/день | ✅ |
| Gemini | gemini-2.5-flash | 1500 RPD / 1M ctx / 65K out | ⚠️ квота |
| Manus | task API | agent platform | ✅ |
| Tavily | search API | search engine | ✅ |

### Неработающие

| Провайдер | Причина |
|-----------|---------|
| Groq | Ключ заблокирован (401) |
| Z.AI | Баланс пуст (нужен бесплатный tier) |

## Rate Limits (из заголовков)

### Mistral
```
x-ratelimit-limit-req-minute: 50
x-ratelimit-remaining-req-minute: 49
x-ratelimit-limit-tokens-minute: 50000
x-ratelimit-remaining-tokens-minute: 49979
```

### Cerebras
```
x-ratelimit-limit-requests-minute: 5
x-ratelimit-remaining-requests-minute: 4
x-ratelimit-limit-requests-hour: 150
x-ratelimit-remaining-requests-hour: 149
x-ratelimit-limit-requests-day: 2400
x-ratelimit-remaining-requests-day: 2399
x-ratelimit-limit-tokens-minute: 30000
x-ratelimit-remaining-tokens-minute: 29995
```

### Cohere
```
x-endpoint-monthly-call-limit: 1000
x-trial-endpoint-call-limit: 20
x-trial-endpoint-call-remaining: 18
```

### Cloudflare
```
cf-ai-neurons: 1.98
```

## Использование CLI

### Проверка всех ключей
```bash
./hunter_check.sh check
```

### Просмотр лимитов
```bash
./hunter_check.sh limits
```

### Просмотр моделей
```bash
./hunter_check.sh models
```

### Полная проверка
```bash
./hunter_check.sh all
```

## Запуск сканера

```bash
# Сбор данных (dry-run)
./hunter --dry-run --limit 20

# Верификация провайдеров
./hunter --verify

# Полный цикл
./hunter --all
```

## Формат вывода (v0.2.0)

Сканер выводит человекочитаемый отчёт:
- Общее количество сырых находок и после фильтра
- Топ-N находок с описанием, источником, URL
- Список подтверждённых бесплатных провайдеров с моделями, лимитами и URL
- Если подтверждённых провайдеров нет — явное сообщение

## Исключённые провайдеры

В `config/filters.json` добавлен список `excluded_providers` — провайдеры, которые фильтруются из результатов по имени (case-insensitive, частичное совпадение).

Текущий список:
- kilo gateway
- kilochat
- kilo

Фильтр работает по `ProviderName` находки — если имя содержит любую из строк исключения, находка отфильтрована с причиной `excluded_provider:<имя>`.

## Cron-задачи

Системный crontab (пользователь `root`):

```bash
# Полный скан с верификацией каждые 6 часов
0 */6 * * * cd /root/LabDoctorM/projects/free-api-hunter && ./hunter --verify >> /var/log/free-api-hunter/scan.log 2>&1

# Ежедневный dry-run для алерта (08:00 UTC)
0 8 * * * cd /root/LabDoctorM/projects/free-api-hunter && ./hunter --dry-run >> /var/log/free-api-hunter/report.log 2>&1

# Быстрый скан без верификации каждые 3 часа
0 */3 * * * cd /root/LabDoctorM/projects/free-api-hunter && ./hunter >> /var/log/free-api-hunter/quick.log 2>&1
```

## Orex Integration

Orex (OpenRouter Expert) — внешний сервис `http://127.0.0.1:8710`, предоставляющий каталог моделей, цены и алерты.

### Эндпоинты

| Эндпоинт | Метод | Описание |
|----------|-------|----------|
| `/api/models` | GET | Список моделей |
| `/api/models?pricing_free=true` | GET | Только бесплатные модели |
| `/api/models?provider=X` | GET | Модели конкретного провайдера |
| `/api/select?task_type=X` | GET | Подбор модели под задачу |
| `/api/pricing/cost` | GET | Расчёт стоимости |
| `/api/sync` | GET | Синхронизация базы |
| `/api/alerts` | GET | Алерты о новых моделях |
| `/api/alerts?since=T` | GET | Алерты с времени T |

### Схема событий для алертера

```json
{
  "event": "new_orex_model",
  "provider": "cerebras",
  "model": "gpt-oss-120b",
  "status": "ok",
  "details": "New free model available",
  "timestamp": "2026-06-18T12:00:00Z"
}
```

### Файлы данных

```
data/
├── providers.json      # локальные провайдеры
├── findings.json       # находки сканера
├── key_pool.json       # пул ключей
└── orex_cache.json     # кэш Orex (бесплатные модели)
```

## Добавление нового провайдера

1. Создать директорию в vault: `mkdir -p /root/LabDoctorM/vault/free-api-hunter/{provider}`
2. Записать ключ: `echo -n "API_KEY" > /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key`
3. Установить права: `chmod 600 /root/LabDoctorM/vault/free-api-hunter/{provider}/api.key`
4. Добавить провайдера в `config/sources.json`
5. Запустить верификацию: `./hunter --verify`

## Безопасность

- **Никогда** не коммитьте ключи в git
- **Никогда** не логируйте значения ключей
- Используйте `KeyLocation` вместо самого ключа в коде
- При утечке — немедленно отозвать ключ и сгенерировать новый
