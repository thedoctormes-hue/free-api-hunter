# Cloudflare Workers AI — Исследование эндпоинтов

**Дата:** 2026-06-25
**Агент:** Ворон (Researcher)
**Ключ:** cfut_gkT*** (53 chars, stored in vault)
**Account ID:** a35fc6ed5da4e21b2f492a5ede04eb73

---

## Базовый URL

```
https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/{model}
```

**Авторизация:** `Authorization: Bearer {api_key}`
**Content-Type:** application/json

---

## Работающие модели (5/10)

| Модель | Статус | Latency | Пример ответа |
|--------|--------|---------|---------------|
| @cf/meta/llama-3.1-8b-instruct | ✅ 200 | 1.4s | "I think it's hello..." |
| @cf/meta/llama-3.3-70b-instruct-fp8-fast | ✅ 200 | 6.0s | Формат threaded |
| @cf/meta/llama-4-scout-17b-16e-instruct | ✅ 200 | 0.5s | "Hello!" |
| @cf/openai/gpt-oss-120b | ✅ 200 | 1.7s | (empty) |
| @cf/moonshotai/kimi-k2.7-code | ✅ 200 | 11.8s | (code response) |

## Неработающие модели (5/10)

| Модель | Статус | Причина |
|--------|--------|---------|
| @cf/deepseek/deepseek-r1-distill-qwen-32b | ❌ 400 | No route for that URI |
| @cf/qwen/qwen3-32b | ❌ 400 | No route for that URI |
| @cf/zhipuai/glm-4.7-flash | ❌ 400 | No route for that URI |
| @cf/google/gemma-3-27b-it | ❌ 400 | No route for that URI |
| @cf/mistral/mistral-7b-instruct-v0.3 | ❌ 400 | No route for that URI |

---

## Формат запроса

```json
POST /client/v4/accounts/{account_id}/ai/run/{model}
{
  "prompt": "Your prompt here"
}
```

## Формат ответа (200)

```json
{
  "result": {
    "response": "Model response text"
  },
  "success": true,
  "errors": [],
  "messages": []
}
```

## Формат ответа (400)

```json
{
  "result": null,
  "success": false,
  "errors": [
    {
      "code": 1000,
      "message": "No route for that URI"
    }
  ],
  "messages": []
}
```

---

## Тарификация

- **Бесплатный tier:** 10,000 neurons/день
- **Neurons:** ~1 neuron за 1 token (примерно)
- **Итого:** ~10K tokens/день бесплатно

---

## Рекомендации для лаборатории

1. **llama-4-scout** — лучшая latency (0.5s), подходит для быстрых задач
2. **llama-3.1-8b** — баланс скорости и качества (1.4s)
3. **llama-3.3-70b** — качество выше, но медленнее (6s)
4. **gpt-oss-120b** — нестабильный (пустые ответы), не использовать
5. **kimi-k2.7-code** — медленный (12s), только для специфических задач

---

## Приоритет использования

Cloudflare Workers AI = **fallback для быстрого инференса** когда основные провайдеры недоступны.

- Быстрый ответ (0.5-1.5s) → llama-4-scout или llama-3.1-8b
- Качественный ответ (6s) → llama-3.3-70b
