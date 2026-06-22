# Статус провайдеров — 2026-06-22

## Активные (4)

### Cerebras
- **Статус:** ✅ Работает
- **Модели:** zai-glm-4.7, gpt-oss-120b
- **Лимиты:** 5 RPM / 30K tokens/min
- **Бесплатно:** Да (регистрация)

### Mistral
- **Статус:** ✅ Работает (2 ключа)
- **Модели:** mistral-small-latest, mistral-large-latest, open-mistral-nemo, codestral-latest
- **Лимиты:** 50 RPM / 50K tokens/min
- **Бесплатно:** Experiment tier

### Cloudflare Workers AI
- **Статус:** ✅ Работает
- **Модели:** @cf/meta/llama-3.3-70b, @cf/meta/llama-4-scout, @cf/openai/gpt-oss-120b, @cf/moonshotai/kimi-k2.7-code
- **Лимиты:** 10K neurons/day
- **Бесплатно:** Да (регистрация)

### Manus
- **Статус:** ✅ Работает
- **Тип:** Agent platform (не чистый LLM API)
- **Лимиты:** 300 credits/day (free plan)
- **Бесплатно:** Да (регистрация)

## Мёртвые (3)

### Groq — ❌ Ключ blocked (401)
### Cohere — ❌ Ключ blocked (401)
### Gemini — ⚠️ Quota exhausted (403), сбрасывается daily

## Живые НЕ в базе ключей

Эти провайдеры подтверждены исследованием, но ключей нет:

- **OpenRouter** (27 free моделей, key в openclaw.json)
- **ModelScope** (2000 req/день, 900+ моделей, китайский)
- **Ollama Cloud** (free tier, но требует регистрации)
- **Pollinations.ai** (unlimited, no key)
- **OpenCode Zen** (free tier для coding agents)
- **GitHub Models** (free tier, есть rate limits)
- **OVH AI Endpoints** (free tier, rate limits)
- **Z.ai / GLM** (был ключ в cerebras как zai-glm-4.7)
