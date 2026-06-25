# Руководство по ключам провайдеров LLM

**Дата:** 2026-06-24
**Автор:** Мангуст (mangust)
**Контекст:** Free API Hunter — сканирование и верификация бесплатных LLM API

---

## Общая информация

Все ключи хранятся в vault: `/root/LabDoctorM/vault/free-api-hunter/`

Структура пути: `vault/free-api-hunter/<провайдер>/<имя_ключа>.key`

Права: директории `700`, файлы ключей `600`.

---

## 1. Mistral AI

### Хранилище

Путь: `vault/free-api-hunter/mistral/`

- `api_key_primary.key` — основной ключ (32 символа)
- `api_key_secondary.key` — резервный ключ (32 символа)

### Тип аккаунта

**Free Tier** — оба ключа работают на бесплатном плане.

Лимиты Free Tier:
- 1 запрос в секунду
- Базовые модели (mistral-small-latest, medium, large)
- Некоторые модели (Devstral Small 2505) полностью бесплатны
- Назначение: evaluation и prototyping

Бонусы для новых аккаунтов: $25-$50 trial credits.
Стартап-программа: до $30K credits.

### Использование

```bash
# Базовый URL
export MISTRAL_BASE_URL="https://api.mistral.ai/v1"

# Основной ключ
export MISTRAL_API_KEY=$(cat /root/LabDoctorM/vault/free-api-hunter/mistral/api_key_primary.key)

# Список моделей
curl -s "${MISTRAL_BASE_URL}/models" \
  -H "Authorization: Bearer ${MISTRAL_API_KEY}" | python3 -m json.tool

# Chat completion
curl -s -X POST "${MISTRAL_BASE_URL}/chat/completions" \
  -H "Authorization: Bearer ${MISTRAL_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mistral-small-latest",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 100
  }' | python3 -m json.tool

# Embeddings
curl -s -X POST "${MISTRAL_BASE_URL}/embeddings" \
  -H "Authorization: Bearer ${MISTRAL_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"model": "mistral-embed", "input": ["Hello world"]}' | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(f'Dimensions: {len(d[\"data\"][0][\"embedding\"])}')
"

# Files API
curl -s "${MISTRAL_BASE_URL}/files" \
  -H "Authorization: Bearer ${MISTRAL_API_KEY}" | python3 -m json.tool
```

### Доступные эндпоинты

| Метод | Путь | Статус |
|-------|------|--------|
| GET | /v1/models | ✅ |
| GET | /v1/files | ✅ |
| POST | /v1/chat/completions | ✅ |
| POST | /v1/embeddings | ✅ |

### Регион

`api.mistral.ai` — один глобальный endpoint, без региональных вариантов.

### Примечания

- Поле `pricing` в `/v1/models` показывает цену за 1M tokens, но **не отражает наличие бесплатного лимита** на аккаунте
- Для проверки баланса используйте test chat-запрос (успешный ответ = есть лимит)
- Free Tier ключи могут использовать все основные модели

---

## 2. Cohere

### Хранилище

Путь: `vault/free-api-hunter/cohere/`

- `api.key` — основной ключ

Дополнительные ключи в `secrets-backup.json`:
- `cohere/apiKey` (дублирует api.key)
- `cohere/apiKey.2` — второй ключ
- `cohere/apiKey.3` — третий ключ
- `cohere/apiKey.4` — четвёртый ключ

Все 4 ключа рабочие (протестированы 2026-06-24).

### Тип аккаунта

**Pay-as-you-go с депозитом** — ключи возвращают ответы (биллинг активирован). Точный баланс неизвестен (Cohere не возвращает баланс в `/v1/models`).

### Использование

```bash
# Базовый URL
export COHERE_BASE_URL="https://api.cohere.com/v1"

# Основной ключ
export COHERE_API_KEY=$(cat /root/LabDoctorM/vault/free-api-hunter/cohere/api.key)

# Список моделей
curl -s "${COHERE_BASE_URL}/models" \
  -H "Authorization: Bearer ${COHERE_API_KEY}" | python3 -m json.tool

# Chat completion (формат Cohere — НЕ OpenAI-совместимый!)
curl -s -X POST "${COHERE_BASE_URL}/chat" \
  -H "Authorization: Bearer ${COHERE_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "command-r-plus-08-2024",
    "message": "Hello",
    "max_tokens": 100
  }' | python3 -m json.tool

# Альтернатива: через v1/chat/completions (OpenAI-совместимый)
curl -s -X POST "${COHERE_BASE_URL}/chat/completions" \
  -H "Authorization: Bearer ${COHERE_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "command-r-plus-08-2024",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 100
  }' | python3 -m json.tool

# Embeddings
curl -s -X POST "${COHERE_BASE_URL}/embed" \
  -H "Authorization: Bearer ${COHERE_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "embed-multilingual-v3.0",
    "texts": ["Hello world"]
  }' | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(f'Embeddings: {len(d[\"embeddings\"])} vectors, dim={len(d[\"embeddings\"][0])}')
"
```

### Доступные эндпоинты

| Метод | Путь | Статус | Формат |
|-------|------|--------|--------|
| GET | /v1/models | ✅ | — |
| POST | /v1/chat | ✅ | Native Cohere |
| POST | /v1/chat/completions | ✅ | OpenAI-совместимый |
| POST | /v1/embed | ✅ | Native Cohere |

### Особенности Cohere

- **Три формата API** (важно!):
  - `/v2/chat` — **основной формат** (работает для всех моделей, включая command-a-plus)
  - `/v1/chat` — native Cohere (НЕ работает для command-a-plus и reasoning)
  - `/v1/chat/completions` — OpenAI-совместимый (НЕ работает для новых моделей)
- `/v2/chat` возвращает `message.content` как **array** с типами `text` и `thinking`
- reasoning-модели возвращают ответ только в `thinking` (поле `text` пустое)
- Embeddings: `/v1/embed` с `texts` (array)
- Classification: `/v1/classify`

### Правильный формат запроса Cohere

```json
POST https://api.cohere.com/v2/chat
{
  "model": "command-a-plus-05-2026",
  "messages": [{"role": "user", "content": "Hello"}],
  "max_tokens": 100
}
```

Ответ:
```json
{
  "message": {
    "role": "assistant",
    "content": [
      {"type": "thinking", "thinking": "..."},
      {"type": "text", "text": "4"}
    ]
  }
}
```

Для reasoning-моделей (command-a-reasoning) ответ только в `thinking`, `text` пустой.

### Регион

`api.cohere.com` — один глобальный endpoint.

---

## 3. Cerebras

### Хранилище

Путь: `vault/free-api-hunter/secrets-backup.json`

Ключи (все 5 рабочие, протестированы 2026-06-24):
- `cerebras/apiKey` — основной
- `cerebras/apiKey.2` — резервный
- `cerebras/apiKey.3` — резервный (rate limited на момент теста)
- `cerebras/apiKey.4` — резервный (rate limited на момент теста)
- `cerebras/apiKey.5` — резервный

### Тип аккаунта

Неизвестен (ключи работают, баланс не проверен).

### Использование

```bash
# Базовый URL (из документации)
export CEREBRA_BASE_URL="https://api.cerebras.ai/v1"

# Основной ключ
export CEREBRA_API_KEY=$(python3 -c "
import json
with open('/root/LabDoctorM/vault/free-api-hunter/secrets-backup.json') as f:
    d = json.load(f)
print(d['cerebras/apiKey'])
")

# Список моделей (OpenAI-совместимый формат)
curl -s "${CEREBRA_BASE_URL}/models" \
  -H "Authorization: Bearer ${CEREBRA_API_KEY}" | python3 -m json.tool

# Chat completion (OpenAI-совместимый формат — используйте этот!)
curl -s -X POST "${CEREBRA_BASE_URL}/chat/completions" \
  -H "Authorization: Bearer ${CEREBRA_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-120b",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 100
  }' | python3 -m json.tool
```

### Доступные эндпоинты

| Метод | Путь | Статус |
|-------|------|--------|
| GET | /v1/models | ✅ (OpenAI-совместимый) |
| POST | /v1/chat/completions | ✅ (OpenAI-совместимый) |

### Важные замечания

- **Используйте OpenAI-совместимый формат** (`/v1/chat/completions` с `messages`), а не нативный
- **Моделей всего 2**: `gpt-oss-120b` (флагман) и `zai-glm-4.7` (reasoning-only)
- **zai-glm-4.7 возвращает ответ только в поле `reasoning`**, поле `content` пустое — это особенность модели
- **Высокая частота запросов** — ключи 3 и 4 получили rate limit (`too_many_requests_error`) при тестировании. Соблюдайте интервалы между запросами
- **Время ответа ~0.13s** для gpt-oss-120b — Cerebras позиционируется как «самый быстрый inference»

### Регион

`api.cerebras.ai` — глобальный endpoint через Cloudflare.
Региональных вариантов нет.

### Пример работы с zai-glm-4.7 (reasoning-only)

```bash
curl -s -X POST "${CEREBRA_BASE_URL}/chat/completions" \
  -H "Authorization: Bearer ${CEREBRA_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "zai-glm-4.7",
    "messages": [{"role": "user", "content": "What is 2+2?"}],
    "max_tokens": 100
  }' | python3 -c "
import sys,json
d=json.load(sys.stdin)
msg=d['choices'][0]['message']
# content пустой, ответ в reasoning
answer = msg.get('content') or msg.get('reasoning', '')
print(f'Answer: {answer}')
"
```

---

## Сводка по провайдерам

| Параметр | Mistral | Cohere | Cerebras |
|----------|---------|--------|---------|
| Базовый URL | api.mistral.ai/v1 | api.cohere.com/v1 | api.cerebras.ai/v1 |
| Формат API | OpenAI-совместимый | Native + OpenAI | OpenAI-совместимый |
| Ключей рабочих | 2/2 | 4/4 | 5/5 |
| Моделей (уникальных) | 36 | 20 | 2 |
| Free Tier | ✅ (1 req/s) | Ограниченный | Неизвестен |
| Ценовой план | Free | Pay-as-you-go | Неизвестен |
| Регион | Глобальный | Глобальный | Глобальный (Cloudflare) |

## Типичные ошибки

### Mistral
- Ошибочный вывод о платности только по полю `pricing` → всегда делайте test chat-запрос
- Reddit банит без прокси (403/429)

### Cohere
- Использование нативного формата вместо OpenAI — оба работают, но структура запроса отличается

### Cerebras
- **Не используйте нативный формат** — только OpenAI-совместимый `/v1/chat/completions` с `messages`
- **zai-glm-4.7 не возвращает content** — ответ только в `reasoning`
- Rate limit при частых запросах — соблюдайте интервалы

---

## Ссылки

- [Mistral API Docs](https://docs.mistral.ai/api)
- [Mistral Free Tier](https://docs.mistral.ai/admin/user-management-finops/tier)
- [Cohere API Docs](https://docs.cohere.com/reference/about)
- [Cerebras Inference Docs](https://inference-docs.cerebras.ai/introduction)
