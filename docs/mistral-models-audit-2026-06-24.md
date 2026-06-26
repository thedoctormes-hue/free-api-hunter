# Mistral Models Audit

**Дата:** 2026-06-24
**Источник:** `GET /v1/models` (live API)
**Ключ:** Free Tier (primary)
**Всего моделей:** 75 (с дубликатами latest) → 36 уникальных

## Сводка по модальностям

| Модальность | Уникальных моделей | Топ-1 |
|------------|-------------------|-------|
| Reasoning | 5 | magistral-medium-latest (131K ctx, score 240) |
| Vision | 5 | magistral-medium-latest (совмещает reasoning+vision+tools) |
| Tools (function calling) | 5 | magistral-medium-latest |
| Chat | 5 | magistral-medium-latest |
| Fine-tuning | 5 | magistral-medium-latest |
| OCR | 1 | mistral-ocr-latest (16K ctx) |
| Audio | 2 | voxtral-small-latest (32K ctx) |
| TTS | 1 | voxtral-mini-tts-latest (4K ctx) |
| FIM (code) | 1 | codestral-latest (256K ctx) |
| Classification | 1 | mistral-moderation-latest (8K ctx) |
| Moderation | 1 | mistral-moderation-latest (8K ctx) |

## Топ-5 по модальности

### Reasoning (модели с поддержкой рассуждений)

1. **magistral-medium-latest** — 131K ctx | reasoning + vision + tools + finetune | Score: 240
2. **mistral-medium-latest** — 262K ctx | reasoning + vision + tools | Score: 230
3. **mistral-small-latest** — 262K ctx | reasoning + vision + tools | Score: 230
4. **mistral-vibe-cli-fast** — 262K ctx | reasoning + vision + tools | Score: 230
5. **magistral-small-latest** — 262K ctx | reasoning + vision + tools | Score: 230

### Vision (мультимодальные с поддержкой изображений)

1. **magistral-medium-latest** — 131K ctx | reasoning + vision + tools + finetune | Score: 240
2. **mistral-medium-latest** — 262K ctx | reasoning + vision + tools | Score: 230
3. **mistral-small-latest** — 262K ctx | reasoning + vision + tools | Score: 230
4. **mistral-vibe-cli-fast** — 262K ctx | reasoning + vision + tools | Score: 230
5. **magistral-small-latest** — 262K ctx | reasoning + vision + tools | Score: 230

### Tools (function calling)

1. **magistral-medium-latest** — 131K ctx | reasoning + vision + tools + finetune | Score: 240
2. **mistral-medium-latest** — 262K ctx | reasoning + vision + tools | Score: 230
3. **mistral-small-latest** — 262K ctx | reasoning + vision + tools | Score: 230
4. **mistral-vibe-cli-fast** — 262K ctx | reasoning + vision + tools | Score: 230
5. **magistral-small-latest** — 262K ctx | reasoning + vision + tools | Score: 230

### Chat (текстовый чат)

1. **magistral-medium-latest** — 131K ctx | reasoning + vision + tools + finetune | Score: 240
2. **mistral-medium-latest** — 262K ctx | reasoning + vision + tools | Score: 230
3. **mistral-small-latest** — 262K ctx | reasoning + vision + tools | Score: 230
4. **mistral-vibe-cli-fast** — 262K ctx | reasoning + vision + tools | Score: 230
5. **magistral-small-latest** — 262K ctx | reasoning + vision + tools | Score: 230

### Fine-tuning

1. **magistral-medium-latest** — 131K ctx | reasoning + vision + tools + finetune | Score: 240
2. **mistral-large-latest** — 262K ctx | finetune + vision + tools | Score: 150
3. **ministral-8b-latest** — 262K ctx | finetune + vision + tools | Score: 150
4. **ministral-14b-latest** — 262K ctx | finetune + vision + tools | Score: 150
5. **ministral-3b-latest** — 131K ctx | finetune + vision + tools | Score: 140

### OCR

1. **mistral-ocr-latest** — 16K ctx | ocr + vision + tools | Score: 105

### Audio

1. **voxtral-small-latest** — 32K ctx | audio + chat + tools | Score: 75
2. **voxtral-mini-latest** — 32K ctx | audio + chat | Score: 45

### TTS (text-to-speech)

1. **voxtral-mini-tts-latest** — 4K ctx | tts + tools + finetune | Score: 80

### FIM (fill-in-the-middle, code)

1. **codestral-latest** — 256K ctx | fim + chat + tools | Score: 95

### Classification / Moderation

1. **mistral-moderation-latest** — 8K ctx | classify + moderate | Score: 30

## Полный список (36 уникальных, по убыванию качества)

| # | Модель | Context | Modalities | Score |
|---|--------|---------|------------|-------|
| 1 | magistral-medium-latest | 131K | chat, finetune, reasoning, tools, vision | 240 |
| 2 | mistral-medium-latest | 262K | chat, reasoning, tools, vision | 230 |
| 3 | mistral-small-latest | 262K | chat, reasoning, tools, vision | 230 |
| 4 | mistral-vibe-cli-fast | 262K | chat, reasoning, tools, vision | 230 |
| 5 | magistral-small-latest | 262K | chat, reasoning, tools, vision | 230 |
| 6 | mistral-medium-3-5 | 262K | chat, reasoning, tools, vision | 230 |
| 7 | mistral-medium-3.5 | 262K | chat, reasoning, tools, vision | 230 |
| 8 | mistral-medium-3 | 262K | chat, reasoning, tools, vision | 230 |
| 9 | mistral-vibe-cli-latest | 262K | chat, reasoning, tools, vision | 230 |
| 10 | mistral-vibe-cli-with-tools | 262K | chat, reasoning, tools, vision | 230 |
| 11 | labs-leanstral-2603 | 197K | chat, reasoning, tools, vision | 220 |
| 12 | mistral-large-latest | 262K | chat, finetune, tools, vision | 150 |
| 13 | ministral-8b-latest | 262K | chat, finetune, tools, vision | 150 |
| 14 | ministral-14b-latest | 262K | chat, finetune, tools, vision | 150 |
| 15 | ministral-3b-latest | 131K | chat, finetune, tools, vision | 140 |
| 16 | mistral-ocr-latest | 16K | ocr, tools, vision | 105 |
| 17 | codestral-latest | 256K | chat, fim, tools | 95 |
| 18 | open-mistral-nemo-2407 | 131K | chat, finetune, tools | 90 |
| 19 | mistral-tiny-latest | 131K | chat, finetune, tools | 90 |
| 20 | devstral-latest | 262K | chat, tools | 80 |
| 21 | devstral-medium-latest | 262K | chat, tools | 80 |
| 22 | mistral-code-agent-latest | 262K | chat, tools | 80 |
| 23 | voxtral-mini-tts-latest | 4K | finetune, tools, tts | 80 |
| 24 | voxtral-small-latest | 32K | audio, chat, tools | 75 |
| 25 | voxtral-mini-latest | 32K | audio, chat | 45 |
| 26 | mistral-moderation-latest | 8K | classify, moderate | 30 |
| 27 | voxtral-mini-transcribe-realtime-2602 | 32K | unknown | 20 |
| 28 | voxtral-mini-realtime-latest | 32K | unknown | 20 |
| 29 | mistral-embed-2312 | 8K | unknown | 10 |
| 30 | codestral-embed-2505 | 8K | unknown | 10 |

*Примечание: 6 моделей с неопределёнными модальностями (unknown) — вероятно embeddings или специализированные.*

## Примечания

- **Pricing:** не возвращается API для Free Tier ключа (все `?`). Для получения pricing нужен Scale plan или запрос к /v1/models с платного ключа.
- **Дедупликация:** `latest` и конкретные версии (например, `mistral-medium-2508` и `mistral-medium-latest`) — одна и та же модель. В таблице приведена последняя версия.
- **Free Tier:** не все модели доступны на Free Tier. Точный список требует проверки через chat completions.
