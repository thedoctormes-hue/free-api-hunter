# Free API Hunter — Документация

## Обзор

Free API Hunter — система для обнаружения, верификации и мониторинга бесплатных LLM API.

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
# Сбор данных
./hunter --limit 20

# Верификация провайдеров
./hunter --verify

# Полный цикл
./hunter --all
```

## Cron-задачи

```bash
# Health-check каждые 30 минут
*/30 * * * * /root/LabDoctorM/workspaces/streikbrecher/projects/hunter_check.sh check

# Полный скан каждые 6 часов
0 */6 * * * cd /root/LabDoctorM/workspaces/streikbrecher/projects && ./hunter --verify

# Ежедневный отчёт
0 8 * * * cd /root/LabDoctorM/workspaces/streikbrecher/projects && ./hunter --all
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
