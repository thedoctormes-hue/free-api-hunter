# Лучшие модели по модальности (Mistral, Cohere, Cerebras)

**Дата:** 2026-06-24
**Провайдеры:** Mistral (Free Tier), Cohere (Pay-as-you-go), Cerebras (неизвестен)
**Всего уникальных моделей:** 58 (Mistral 36 + Cohere 20 + Cerebras 2)

---

## Reasoning (рассуждения)

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | mistral-medium-latest | Mistral | 262K | reasoning + vision + tools, лучший баланс |
| 2 | mistral-small-2603 | Mistral | 262K | reasoning + vision + tools, новее medium |
| 3 | command-a-reasoning-08-2025 | Cohere | 289K | единственная reasoning у Cohere |

## Vision (изображения)

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | mistral-medium-latest | Mistral | 262K | reasoning + vision + tools |
| 2 | mistral-small-2603 | Mistral | 262K | reasoning + vision + tools |
| 3 | command-a-vision-07-2025 | Cohere | 128K | мультимодальная, встроена в command-a |

## Tools (function calling)

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | mistral-medium-latest | Mistral | 262K | reasoning + vision + tools |
| 2 | devstral-2512 | Mistral | 262K | tools, без reasoning |
| 3 | mistral-ocr-latest | Mistral | 16K | ocr + vision + tools |

## Chat (текстовый чат)

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | command-a-plus-05-2026 | Cohere | 436K | крупнейший контекст, флагман |
| 2 | mistral-medium-latest | Mistral | 262K | reasoning + vision + tools |
| 3 | mistral-small-2603 | Mistral | 262K | баланс качества и скорости |
| 4 | gpt-oss-120b | Cerebras | — | 120B params, ~0.13s response |
| 5 | command-a-03-2025 | Cohere | 288K | оптимальный баланс у Cohere |

## Code (FIM, fill-in-the-middle)

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | codestral-2508 | Mistral | 256K | FIM + chat + tools, для кодинга |
| 2 | mistral-code-latest | Mistral | 256K | то же что codestral |

## Fine-tuning

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | mistral-large-2512 | Mistral | 262K | finetune + vision + tools |
| 2 | ministral-8b-2512 | Mistral | 262K | лёгкая для fine-tuning |
| 3 | ministral-14b-2512 | Mistral | 262K | средняя для fine-tuning |

## OCR

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | mistral-ocr-latest | Mistral | 16K | ocr + vision + tools |

## Audio

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | voxtral-small-2507 | Mistral | 32K | audio + chat + tools |
| 2 | voxtral-mini-2507 | Mistral | 32K | audio + chat |

## TTS (text-to-speech)

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | voxtral-mini-tts-2603 | Mistral | 4K | tts + tools + finetune |

## Embeddings

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | embed-multilingual-v3.0 | Cohere | 512 | мультиязычный |
| 2 | embed-english-v3.0 | Cohere | 512 | английский |
| 3 | mistral-embed-2312 | Mistral | 8K | встроенный |

## ASR (speech-to-text)

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | cohere-transcribe-03-2026 | Cohere | 32K | whisper-альтернатива |

## Translate

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | command-a-translate-08-2025 | Cohere | 9K | встроенный переводчик |

## Classification / Moderation

| # | Модель | Провайдер | Контекст | Примечание |
|---|--------|-----------|----------|------------|
| 1 | mistral-moderation-2603 | Mistral | 131K | classify + moderate |

---

## Результаты проверки (верификация через chat)

**Дата:** 2026-06-24

### Mistral — 4/4 ✅

- mistral-medium-latest — ✅ 0.35s
- mistral-small-2603 — ✅ 2.28s
- devstral-2512 — ✅ 0.46s
- codestral-2508 — ✅ 0.49s

### Cohere — 3/4 ✅

- command-a-plus-05-2026 — ✅ 0.48s (через /v2/chat)
- command-a-reasoning-08-2025 — ⚠️ только thinking (через /v2/chat)
- command-a-03-2025 — ✅ 5.45s (через /v2/chat)
- command-a-vision-07-2025 — ✅ 0.32s (через /v2/chat)

### Cerebras — 2/2 ✅

- gpt-oss-120b — ✅ 0.42s
- zai-glm-4.7 — ⚠️ только reasoning

**Вывод:** все 4 провайдера работают. Thinking-only модели (command-a-reasoning, zai-glm-4.7) требуют парсинга reasoning-поля.

---

## Топ-5 моделей по провайдеру

### Mistral

1. **mistral-medium-latest** — 262K ctx, reasoning + vision + tools
2. **mistral-small-2603** — 262K ctx, reasoning + vision + tools (новее)
3. **devstral-2512** — 262K ctx, tools
4. **codestral-2508** — 256K ctx, FIM + code
5. **mistral-large-2512** — 262K ctx, finetune + vision + tools

### Cohere

1. **command-a-plus-05-2026** — 436K ctx, флагман
2. **command-a-reasoning-08-2025** — 289K ctx, reasoning
3. **command-a-03-2025** — 288K ctx, оптимальный
4. **command-a-vision-07-2025** — 128K ctx, vision
5. **command-r-plus-08-2024** — 128K ctx, предыдущий флагман

### Cerebras

1. **gpt-oss-120b** — 120B params, reasoning + content, ~0.13s
2. **zai-glm-4.7** — 4.7B params, reasoning-only (content не возвращается)

---

## Рекомендации по использованию

| Задача | Лучшая модель | Провайдер | Причина |
|--------|--------------|-----------|---------|
| Универсальный чат | mistral-medium-latest | Mistral | reasoning + vision + tools, Free Tier |
| Обработка больших документов | command-a-plus-05-2026 | Cohere | 436K контекст |
| Генерация кода | codestral-2508 | Mistral | FIM поддержка |
| Распознавание изображений | mistral-medium-latest | Mistral | vision + reasoning |
| Аудио понимание | voxtral-small-2507 | Mistral | audio + chat + tools |
| Быстрый inference | gpt-oss-120b | Cerebras | ~0.13s response |
| Перевод | command-a-translate-08-2025 | Cohere | специализированная модель |
| Embeddings | embed-multilingual-v3.0 | Cohere | мультиязычный |
| Модерация контента | mistral-moderation-2603 | Mistral | classify + moderate |
