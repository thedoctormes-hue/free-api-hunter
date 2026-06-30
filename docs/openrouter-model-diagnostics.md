# OpenRouter Model Diagnostics — методика диагностики

**Версия:** 1.0.0
**Дата:** 2026-06-30
**Автор:** raven (Ворон)
**Связанный инцидент:** INC-20260630-121200-5ce045

## Назначение

Методика автоматической диагностики доступности и качества бесплатных моделей OpenRouter для использования в качестве fallback-моделей агентов OpenClaw.

## Проблема

OpenRouter предоставляет список "бесплатных" моделей (pricing: prompt=0, completion=0), но на практике:

- **~35%** моделей работают стабильно
- **~25%** возвращают 429 (upstream rate limiting провайдера)
- **~20%** возвращают 404 (стали платными, free-тир убран)
- **~15%** возвращают 402 (требуют кредиты, хотя помечены как free)
- **~5%** возвращают NoneType (отключены, но висят в каталоге)

## Типы ошибок и их причины

### HTTP 429 — Rate Limited

**Причина:** провайдер (NVIDIA, Meta, Google, OpenAI) ограничивает количество бесплатных запросов. Это НЕ проблема ключа OpenRouter.

**Решение:** не использовать модель как основную fallback. Можно держать в конце списка как запасную (вдруг лимиты сбросятся).

**Пострадавшие модели (30.06.2026):**
- qwen/qwen3-coder:free
- google/gemma-4-31b-it:free
- qwen/qwen3-next-80b-a3b-instruct:free
- openai/gpt-oss-120b:free
- meta-llama/llama-3.3-70b-instruct:free
- meta-llama/llama-3.2-3b-instruct:free
- nousresearch/hermes-3-llama-3.1-405b:free

### HTTP 404 — Not Found

**Причина:** модель стала платной, free-версия удалена из каталога OpenRouter, но кэш обновляется с задержкой.

**Решение:** исключить из fallback-списка.

**Пострадавшие модели (30.06.2026):**
- meta-llama/llama-3.1-8b-instruct:free
- meta-llama/llama-3.1-70b-instruct:free
- anthropic/claude-3-haiku:free
- nex-agi/nex-n2-pro:free

### HTTP 402 — Insufficient Credits

**Причина:** модель помечена как free (pricing=0), но провайдер требует кредиты. Баг OpenRouter.

**Решение:** исключить из fallback-списка.

**Пострадавшие модели (30.06.2026):**
- google/lyria-3-pro-preview
- google/lyria-3-clip-preview

### NoneType Error — Отключённые модели

**Причина:** модель отключена провайдером, но не удалена из каталога. API возвращает `null` вместо ошибки.

**Решение:** исключить из fallback-списка.

**Пострадавшие модели (30.06.2026):**
- poolside/laguna-xs.2:free
- poolside/laguna-m.1:free
- cohere/north-mini-code:free
- openrouter/free
- openai/gpt-oss-20b:free
- nvidia/nemotron-nano-9b-v2:free
- liquid/lfm-2.5-1.2b-thinking:free

## Методика диагностики

### Шаг 1 — Получение списка бесплатных моделей

```
GET https://openrouter.ai/api/v1/models
Authorization: Bearer <ключ>
```

Фильтр: `pricing.prompt == "0" AND pricing.completion == "0"`

### Шаг 2 — Тестовый запрос в каждую модель

```
POST https://openrouter.ai/api/v1/chat/completions
Authorization: Bearer <ключ>
Content-Type: application/json

{
  "model": "<id>",
  "messages": [{"role": "user", "content": "<тестовый промпт>"}],
  "max_tokens": 200,
  "temperature": 0.3
}
```

**Тестовый промпт** должен быть среднего размера (~200 слов), требовать конкретного ответа (не "2+2"), чтобы проверить качество генерации.

### Шаг 3 — Классификация результатов

| Критерий | OK | 429 | 404 | 402 | NoneType |
|---|---|---|---|---|---|
| HTTP статус | 200 | 429 | 404 | 402 | 200 (null) |
| choices | массив | — | — | — | null |
| Действие | ✅ fallback | ⚠️ запасная | ❌ исключить | ❌ исключить | ❌ исключить |

### Шаг 4 — Ранжирование работающих моделей

**Tier 1 (основные fallback):** latency < 1000ms, tokens >= 100
**Tier 2 (запасные):** latency < 2500ms, tokens >= 100
**Tier 3 (минимальные):** tokens < 100, но работают
**Исключить:** content-safety, < 2B параметров

### Шаг 5 — Обновление openclaw.json

```json
{
  "agents": {
    "defaults": {
      "model": {
        "primary": "openrouter/owl-alpha",
        "fallbacks": [
          "nvidia/nemotron-3-nano-30b-a3b:free",
          "nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free",
          "nvidia/nemotron-3-ultra-550b-a55b:free",
          "nvidia/nemotron-3-super-120b-a12b:free",
          "google/gemma-4-26b-a4b-it:free"
        ]
      }
    }
  }
}
```

## Рекомендуемый порядок fallback

1. **nvidia/nemotron-3-nano-30b-a3b:free** — 537ms, 184tok, быстрая и качественная
2. **nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free** — 472ms, 200tok, reasoning
3. **nvidia/nemotron-3-ultra-550b-a55b:free** — 868ms, 200tok, большая модель
4. **nvidia/nemotron-3-super-120b-a12b:free** — 904ms, 200tok, запасная
5. **google/gemma-4-26b-a4b-it:free** — 1910ms, 200tok, медленная но качественная

## Автоматизация

Скрипт: `scripts/or-model-diagnose.py`
Крон: `raven-openrouter-diagnostics` (ежедневно 08:00 MSK)

## История изменений

- 1.0.0 (2026-06-30) — первая версия, сканирование 26 моделей, 9 работают
