# Mistral API Integration

## Обзор

Mistral AI — европейский (французский) провайдер LLM с моделями высокого качества. **Mistral имеет Free Tier** с бесплатными лимитами для evaluation и prototyping.

- **Официальный сайт:** https://mistral.ai
- **API документация:** https://docs.mistral.ai/api
- **Платформа:** https://console.mistral.ai
- **Базовый URL:** `https://api.mistral.ai/v1`
- **Free Tier Docs:** https://docs.mistral.ai/admin/user-management-finops/tier
- **Pricing:** https://mistral.ai/pricing/

## Ключи

Хранятся в vault: `/root/LabDoctorM/vault/free-api-hunter/mistral/`

| Файл | Назначение | Статус |
|------|-----------|--------|
| `api_key_primary.key` | Основной ключ | ✅ Активен (протестирован 2026-06-24) |
| `api_key_secondary.key` | Резервный ключ | ✅ Активен (протестирован 2026-06-24) |

Оба ключа имеют полный доступ к API (chat, models, embeddings, files).

**Добавлено:** 2026-06-24

## Эндпоинты

### Проверенные (live тесты от 2026-06-24)

| Метод | Путь | Статус | Описание |
|-------|------|--------|----------|
| `GET` | `/v1/models` | ✅ 200 | Список всех моделей (75 шт.) |
| `GET` | `/v1/files` | ✅ 200 | Список загруженных файлов |
| `POST` | `/v1/chat/completions` | ✅ 200 | Chat completions (основной) |
| `POST` | `/v1/embeddings` | ✅ 200 | Embeddings (mistral-embed, 1024d) |

### Существующие (422/400 = требуют правильный payload)

| Метод | Путь | Статус | Описание |
|-------|------|--------|----------|
| `POST` | `/v1/agents` | 422 | Agents API |
| `POST` | `/v1/agents/completions` | 422 | Agent completions |
| `POST` | `/v1/conversations` | 422 | Conversations |
| `POST` | `/v1/ocr` | 422 | OCR распознавание |
| `POST` | `/v1/files` | 422 | Загрузка/управление файлами |
| `POST` | `/v1/completions` | 422 | Text completions (legacy) |

### Несуществующие (404)

| Метод | Путь | Примечание |
|-------|------|-----------|
| `POST` | `/v1/fine-tunes` | Не поддерживается |
| `POST` | `/v1/fine-tuning/jobs` | Не поддерживается |
| `POST` | `/v1/jobs` | Не поддерживается |

## Доступные модели (75 шт.)

### Топ моделей по контексту

| Модель | Контекст | Особенности |
|--------|----------|-------------|
| mistral-medium-2508 | 131K | Последняя версия Medium, vision ✅ |
| mistral-medium-2505 | 131K | Предыдущая версия, vision ✅ |
| open-mistral-nemo | 128K | Открытая, эффективная |
| mistral-large-latest | 128K | Самая мощная |
| mistral-small-latest | 32K | Быстрая, недорогая |
| mistral-7b | 32K | Базовая |
| mistral-7b-instruct | 32K | Instruct-версия |
| mistral-embed | - | Embeddings (1024 dimensions) |

### Категории моделей

- **Базовые:** mistral-7b, mistral-7b-instruct
- **Средние:** mistral-small-latest, open-mistral-nemo
- **Крупные:** mistral-medium-2505, mistral-medium-2508, mistral-large-latest
- **Специализированные:** mistral-embed (embeddings)
- **Multimodal:** vision поддержка в mistral-medium-2505/2508

### Полный список

```bash
curl -s https://api.mistral.ai/v1/models \
  -H "Authorization: Bearer $(cat /root/LabDoctorM/vault/free-api-hunter/mistral/api_key_primary.key)" \
  | python3 -m json.tool
```

## Примеры запросов

### Chat Completion
```bash
curl -s https://api.mistral.ai/v1/chat/completions \
  -X POST \
  -H "Authorization: Bearer $MISTRAL_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mistral-small-latest",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 100
  }'
```

### Embeddings
```bash
curl -s https://api.mistral.ai/v1/embeddings \
  -X POST \
  -H "Authorization: Bearer $MISTRAL_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mistral-embed",
    "input": ["Hello world"]
  }'
```

### Python SDK
```python
from mistralai.client import Mistral

with Mistral(api_key=os.getenv("MISTRAL_API_KEY", "")) as mistral:
    res = mistral.chat.complete(
        model="mistral-large-latest",
        messages=[{"role": "user", "content": "Hello"}],
    )
    print(res)
```

### Node.js SDK
```javascript
import { Mistral } from "@mistralai/mistralai";
const mistral = new Mistral({ apiKey: "MISTRAL_API_KEY" });
const result = await mistral.chat.complete({
  model: "mistral-small-latest",
  messages: [{ content: "Hello", role: "user" }],
});
```

## Free Tier

Mistral предоставляет бесплатный уровень (Free Tier) для evaluation и prototyping.

### Лимиты Free Tier

- **Rate limit:** 1 запрос в секунду
- **Модели:** ограниченный набор (mistral-small-latest и базовые)
- **Назначение:** evaluation, prototyping, разработка

### Trial Credits

- **Новые аккаунты:** $25-$50 trial credits
- **Стартап-программа:** до $30K free credits для квалифицирующихся компаний
- **Freelance Stack:** до $30K credits (промокод)

### Бесплатные модели

Некоторые модели доступны бесплатно:
- **Devstral Small 2505** — полностью бесплатна через API

### Источники

- [Rate limits and usage tiers](https://docs.mistral.ai/admin/user-management-finops/tier)
- [Mistral Pricing](https://mistral.ai/pricing/)
- [Free Tier analysis (Reddit)](https://www.reddit.com/r/MistralAI/comments/1rc8rwf/mistral_api_quota_and_rate_limits_pools_analysis/)
- [Mistral Free Tokens Guide](https://www.getaiperks.com/en/ai/mistral-ai-free-credits-2026)

## Ценообразование (Pay-as-you-go)

⚠️ **Pay-as-you-go модели платные.** Актуальные цены на https://mistral.ai/pricing

Примерные цены (за 1M tokens, могут быть неактуальными):

| Модель | Prompt | Completion |
|--------|--------|------------|
| mistral-small-latest | $0.20 | $0.60 |
| mistral-medium | $2.75 | $8.10 |
| mistral-large-latest | $4.00 | $12.00 |

**Важно:** Перед использованием в продакшене проверяйте актуальные цены на официальном сайте. Поле `pricing` в `/v1/models` показывает цену за 1M tokens, но **не отражает наличие бесплатного лимита** на аккаунте.

## Интеграция с Free API Hunter

Mistral добавлен в `config/sources.json` как провайдер со статусом `confirmed`.

**Важно:** Mistral — платный провайдер, не подходит для бесплатного сканирования. Используется для:
- Платного fallback инференса
- Тестирования и верификации
- Сравнения качества моделей

### Верификация ключей

```bash
# Тест первичного ключа
curl -s -o /dev/null -w "%{http_code}" \
  "https://api.mistral.ai/v1/models" \
  -H "Authorization: Bearer $(cat /root/LabDoctorM/vault/free-api-hunter/mistral/api_key_primary.key)"

# Тест вторичного ключа
curl -s -o /dev/null -w "%{http_code}" \
  "https://api.mistral.ai/v1/models" \
  -H "Authorization: Bearer $(cat /root/LabDoctorM/vault/free-api-hunter/mistral/api_key_secondary.key)"

# Тест chat
curl -s -o /dev/null -w "%{http_code}" \
  -X POST "https://api.mistral.ai/v1/chat/completions" \
  -H "Authorization: Bearer $(cat /root/LabDoctorM/vault/free-api-hunter/mistral/api_key_primary.key)" \
  -H "Content-Type: application/json" \
  -d '{"model":"mistral-small-latest","messages":[{"role":"user","content":"hi"}],"max_tokens":5}'
```

## Безопасность

- Ключи хранятся в vault с правами `600`
- Директория vault: `700`
- Audit log: каждое чтение ключа логируется
- Ротация: через `rotate-vault-keys.sh` (раз в 90 дней)
- Права на ключи: `chmod 600` при ротации

## Ссылки

- [Mistral AI](https://mistral.ai)
- [API Documentation](https://docs.mistral.ai/api)
- [Pricing](https://mistral.ai/pricing)
- [Console](https://console.mistral.ai)
- [Python SDK](https://github.com/mistralai/mistralai-python)
- [Node.js SDK](https://github.com/mistralai/mistralai-client-js)
