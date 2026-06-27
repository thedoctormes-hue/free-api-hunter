# Cloudflare Workers AI — Исследование

**Дата:** 2026-06-27
**Автор:** КотОлизатор
**Статус:** Завершено

---

## 1. Аккаунты и ключи

Всего **6 аккаунтов**, каждый со своим Cloudflare API token (`cfut_`).

| # | Ключ (начало) | User ID (Account ID) | Статус |
|---|---|---|---|
| 1 | cfut_ZqD7Ep01... | 207736d5b210b443550eed21bd09502b | ✅ Полный доступ |
| 2 | cfut_Iu9uJVMq... | a35fc6ed5da4e21b2f492a5ede04eb73 | ✅ Workers AI |
| 3 | cfut_slr6nK0K... | 62f67a9debc67c45a49a6b8eacef2352 | ✅ Workers AI |
| 4 | cfut_IBlWwZcr... | 34ba88610ade55bc07b1244274acfebc | ✅ Workers AI |
| 5 | cfut_LDnU1TzK... | 9799b87f0d905886e01fda1a19c3d9cb | ✅ Workers AI |
| 6 | cfut_QvnMZBO8... | f6db2cf91561290f288e70b9ffa9cca5 | ✅ Workers AI |

**Критически важно:** `user ID = Account ID` для Cloudflare Workers AI REST API.

---

## 2. Архитектура доступа

### Рабочие эндпоинты:

```
POST https://api.cloudflare.com/client/v4/accounts/{USER_ID}/ai/v1/chat/completions
Headers: Authorization: Bearer {API_TOKEN}
Body: OpenAI chat completions format
```

### НЕ работают:
- `/ai/run` — возвращает 400/401
- `/ai/models` — возвращает 401 (нужен AI Gateway permission)

### Лимиты:
- **10,000 Neurons/день** на аккаунт (бесплатно)
- 6 аккаунтов = **60,000 Neurons/день** при ротации
- Сверх лимита: **$0.011 / 1,000 Neurons**
- Лимиты сбрасываются в 00:00 UTC

---

## 3. Что такое Neurons

Neurons — единица измерения GPU-вычислений в Cloudflare Workers AI. Аналог «стоимости вызова GPU». Каждая модель имеет свой коэффициент — чем больше/сложнее модель, тем больше Neurons сжигается за токен.

**Примеры стоимости (output):**
- Gemma 4 26B: 27K neurons/M output (самая дешёвая)
- Granite 4.0 H Micro: 10K neurons/M output
- Llama 3.1 70B: 204K neurons/M output
- GLM-5.2: 400K neurons/M output (самая дорогая)

---

## 4. Модальности

### 4.1 Text Generation (33+ моделей)

Все бесплатны в рамках 10K Neurons/день.

**Топ по мощности:**
1. **GLM-5.2** — Z.ai флагман, 262K ctx, agentic coding, reasoning
2. **Kimi K2.7 Code** — 1T params, 262K ctx, tool calling, vision
3. **GPT-oss-120B** — OpenAI open-weight, reasoning, agentic
4. **Nemotron-3-120B** — NVIDIA, хороший баланс цена/качество
5. **DeepSeek R1 Distill Qwen-32B** — reasoning
6. **Llama 3.1 70B / 3.3 70B** — мощные, но дорогие
7. **Mistral Small 3.1 24B** — vision, 128K ctx
8. **Llama 4 Scout 17B** — MoE, multimodal (text+image)
9. **Gemma 4 26B** — быстрая, дешёвая
10. **Qwen3-30B-A3B** — MoE, хорошее соотношение

**Полный список моделей:**
- Llama: 3.2-1B, 3.2-3B, 3.1-8B (fp8/awq), 3.1-11B-vision, 3.1-70B, 3.3-70B, 3-8B, 4-Scout-17B
- OpenAI: gpt-oss-20B, gpt-oss-120B
- Moonshot: kimi-k2.5, kimi-k2.6, kimi-k2.7-code
- Zhipu: glm-4.7-flash, glm-5.2
- Google: gemma-3-12b-it, gemma-4-26b-a4b-it
- NVIDIA: nemotron-3-120b-a12b
- DeepSeek: r1-distill-qwen-32b
- Mistral: mistral-7b-instruct-v0.1, mistral-small-3.1-24b-instruct
- Qwen: qwen3-30b-a3b-fp8, qwen2.5-coder-32b-instruct, qwq-32b
- IBM: granite-4.0-h-micro
- Other: llama-guard-3-8b, sea-lion-v4-27b-it

### 4.2 Text Embeddings (6 моделей)
- bge-small-en-v1.5, bge-base-en-v1.5, bge-large-en-v1.5, bge-m3
- plamo-embedding-1b, qwen3-embedding-0.6b

### 4.3 Text-to-Image (5 моделей)
- FLUX.2 dev, FLUX.2-klein-4b, FLUX.2-klein-9b
- Lucid Origin, Phoenix 1.0

### 4.4 Audio
- TTS: Aura-1, Aura-2-en, Aura-2-es, Melotts
- ASR: Whisper, Whisper-large-v3-turbo, Nova-3, Flux
- VAD: smart-turn-v2

---

## 5. Сравнение с OpenRouter

| Параметр | Cloudflare Workers AI | OpenRouter |
|---|---|---|
| Моделей | 80 | 20+ free |
| Лимит | 10K Neurons/день × 6 аккаунтов | Зависит от провайдера |
| AI Gateway | Встроенный (cache, rate limit, logging) | Нет |
| Ценообразование | Neurons (GPU compute) | Токены |
| Формат | /ai/v1/chat/completions | OpenAI-compatible |
| Модальности | LLM + Embeddings + Image + Audio | Преимущественно LLM |

---

## 6. Уроки и находки

1. **user ID = Account ID** — не путать с Account ID из /accounts endpoint
2. **Использовать /ai/v1/chat/completions** — /ai/run не работает
3. **Rate limiting** — Cloudflare блокирует при >3 запросах подряд, нужен интервал 5-30 сек
4. **/ai/models не работает** с этими ключами — список моделей только из документации
5. **Бенчмарк требует аккуратности** — 6×10×3 запросов нельзя гонять без пауз

---

## 7. Рекомендации для free-api-hunter

- Добавить Cloudflare Workers AI как источник бесплатных API
- Ротация по 6 аккаунтам для 60K Neurons/день
- Модели-кандидаты для мониторинга: GLM-5.2, Kimi K2.7 Code, GPT-oss-120B, Nemotron-3-120B
- Формат хранения: `cfut_` ключ + user ID как account_id
