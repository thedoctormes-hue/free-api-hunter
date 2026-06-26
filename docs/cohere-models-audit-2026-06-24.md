# Cohere Models Audit

**Дата:** 2026-06-24
**Источник:** `GET /v1/models` (live API)
**Ключ:** `cohere/apiKey` (primary)
**Всего моделей:** 20 (уникальных)

## Сводка по модальностям

| Модальность | Уникальных моделей | Топ-1 |
|------------|-------------------|-------|
| Chat | 9 | command-a-plus-05-2026 (436K ctx, score 150) |
| Reasoning | 1 | command-a-reasoning-08-2025 (289K ctx, score 145) |
| Vision | 2 | command-a-vision-07-2025 (128K ctx, score 120) |
| Embed | 8 | embed-multilingual-v3.0-image (score 40) |
| ASR | 1 | cohere-transcribe-03-2026 (33K ctx, score 20) |
| Translate | 1 | command-a-translate-08-2025 (9K ctx, score 90) |

## Топ-5 по модальности

### Reasoning
1. **command-a-reasoning-08-2025** — 289K ctx | chat+reasoning | Score: 145

### Chat
1. **command-a-plus-05-2026** — 436K ctx | chat | Score: 150
2. **command-a-reasoning-08-2025** — 289K ctx | chat+reasoning | Score: 145
3. **command-a-03-2025** — 288K ctx | chat | Score: 120
4. **command-a-vision-07-2025** — 128K ctx | chat+vision | Score: 120
5. **command-r-plus-08-2024** — 128K ctx | chat | Score: 100

### Vision
1. **command-a-vision-07-2025** — 128K ctx | chat+vision | Score: 120
2. **c4ai-aya-vision-32b** — 16K ctx | chat+vision | Score: 65

### Embed
1. **embed-multilingual-v3.0-image** — image embed | Score: 40
2. **embed-english-v3.0-image** — image embed | Score: 38
3. **embed-multilingual-v3.0** — 512 ctx | Score: 35
4. **embed-multilingual-light-v3.0** — 512 ctx | Score: 35
5. **embed-multilingual-light-v3.0-image** — image embed | Score: 35

### ASR
1. **cohere-transcribe-03-2026** — 33K ctx | asr | Score: 20

### Translate
1. **command-a-translate-08-2025** — 9K ctx | translate | Score: 90

## Полный список (20 моделей, по убыванию качества)

| # | Модель | Context | Modalities | Score |
|---|--------|---------|------------|-------|
| 1 | command-a-plus-05-2026 | 436K | chat | 150 |
| 2 | command-a-reasoning-08-2025 | 289K | chat, reasoning | 145 |
| 3 | command-a-03-2025 | 288K | chat | 120 |
| 4 | command-a-vision-07-2025 | 128K | chat, vision | 120 |
| 5 | command-r-plus-08-2024 | 128K | chat | 100 |
| 6 | command-a-translate-08-2025 | 9K | translate | 90 |
| 7 | command-r7b-12-2024 | 132K | chat | 90 |
| 8 | command-r7b-arabic-02-2025 | 128K | chat | 90 |
| 9 | command-r-08-2024 | 128K | chat | 80 |
| 10 | c4ai-aya-expanse-32b | 128K | chat | 70 |
| 11 | c4ai-aya-vision-32b | 16K | chat, vision | 65 |
| 12 | embed-multilingual-v3.0-image | - | embed | 40 |
| 13 | embed-english-v3.0-image | - | embed | 38 |
| 14 | embed-multilingual-v3.0 | 512 | embed | 35 |
| 15 | embed-multilingual-light-v3.0 | 512 | embed | 35 |
| 16 | embed-multilingual-light-v3.0-image | - | embed | 35 |
| 17 | embed-english-v3.0 | 512 | embed | 30 |
| 18 | embed-english-light-v3.0 | 512 | embed | 30 |
| 19 | embed-english-light-v3.0-image | - | embed | 30 |
| 20 | cohere-transcribe-03-2026 | 33K | asr | 20 |

## Ключевые выводы

- **command-a-plus-05-2026** — флагман с 436K контекста (крупнейший у Cohere)
- **command-a-reasoning-08-2025** — единственная reasoning модель
- **command-a-03-2025** — оптимальный баланс (288K ctx, score 120)
- **command-a-vision-07-2025** — мультимодальная с 128K
- **c4ai-aya-expanse-32b** — 32B параметра, 128K ctx, конкурент DeepSeek
- **c4ai-aya-vision-32b** — vision версия Aya
- **embed-multilingual-v3.0** — мультиязычный embedding
- **cohere-transcribe-03-2026** — ASR (whisper-альтернатива)

## Проверка через Chat (v2/chat)

**Дата проверки:** 2026-06-24

**Endpoint:** `POST /v2/chat` (основной формат для новых моделей Cohere)

| Модель | Статус | Время | Ответ |
|--------|--------|-------|-------|
| command-a-plus-05-2026 | ✅ OK | 0.48s | answer="4" |
| command-a-reasoning-08-2025 | ⚠️ thinking-only | 0.85s | thinking_only, text пустой |
| command-a-03-2025 | ✅ OK | 5.45s | answer="4" |
| command-a-vision-07-2025 | ✅ OK | 0.32s | answer="4" |

**Важно:** `/v1/chat` НЕ работает для command-a-plus и reasoning-моделей. Используйте `/v2/chat`.

## Примечания

- **Pricing:** не возвращается API. Ключ работает (chat вернул ответ) — значит есть активный баланс/депозит
- **Free Tier:** Cohere предлагает бесплатный тариф для разработки (ограниченные запросы)
- **Регион:** api.cohere.com (US/EU)
- **Формат ответа:** `message.content` — массив с объектами `{"type": "text", "text": "..."}` и `{"type": "thinking", "thinking": "..."}`
