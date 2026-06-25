# Сводный отчёт по провайдерам LLM

**Дата:** 2026-06-24
**Методология:** ADR-0052 (скоринг по tier + context + capabilities)

---

## 1. Mistral AI

**Статус:** ✅ Доступен | **Ключей:** 2 (оба рабочие, Free Tier) | **Моделей:** 75 (36 уникальных)

**Ключевые модели:**

- **magistral-medium-latest** (131K ctx, score 240) — флагман, единственная модель с reasoning + vision + tools + finetune одновременно
- **mistral-medium-latest** (262K ctx, score 230) — оптимальный баланс
- **mistral-small-latest** (262K ctx, score 230) — лучшая для задач средней сложности
- **codestral-latest** (256K ctx, score 95) — кодинг с FIM
- **voxtral-small-latest** (32K ctx, score 75) — audio understanding + tools

**Модальности:** reasoning (5), vision (5), tools (5), chat (5), finetune (5), ocr (1), audio (2), tts (1), fim (1), moderate (1)

---

## 2. Cohere

**Статус:** ✅ Доступен | **Ключей:** 4 (все рабочие, платный с депозитом) | **Моделей:** 20 уникальных

**Ключевые модели:**

- **command-a-plus-05-2026** (436K ctx, score 150) — флагман, крупнейший контекст у Cohere
- **command-a-reasoning-08-2025** (289K ctx, score 145) — единственная reasoning модель
- **command-a-03-2025** (288K ctx, score 120) — оптимальный баланс
- **command-a-vision-07-2025** (128K ctx, score 120) — мультимодальная
- **c4ai-aya-expanse-32b** (128K ctx, score 70) — 32B параметров, конкурент DeepSeek

**Модальности:** chat (9+), reasoning (1), vision (2), embed (8), asr (1), translate (1)

---

## 3. Cerebras

**Статус:** ❌ Заблокирован | **Ключей:** 5 (все нерабочие) | **Моделей:** невозможно определить

**Причина:** TLS handshake failure при подключении к api.cerebras.com. Провайдер заблокирован из региона сервера (EU/США IP не проходит через Cloudflare/CDN Cerebras).

**Уроки:** При недоступности провайдера по TLS/сети — фиксировать причину сразу и переходить к следующему провайдеру. Не тратить время на повторные попытки одного и того же endpoint.

---

## Сравнение провайдеров

| Параметр | Mistral | Cohere | Cerebras |
|----------|---------|--------|---------|
| Доступность | ✅ | ✅ | ❌ |
| Моделей (уникальных) | 36 | 20 | — |
| Топ score | 240 | 150 | — |
| Ключей рабочих | 2/2 | 4/4 | 0/5 |
| Free Tier | ✅ | Ограниченный | — |
| Флагман | magistral-medium | command-a-plus-05 | — |
| Флагман score | 240 | 150 | — |
| Макс контекст | 262K | 436K | — |
| Главный вывод | Лучший score, Free Tier | Крупнейший контекст | Заблокирован |

**Winner by score:** Mistral (magistral-medium-latest, 240)
**Winner by context:** Cohere (command-a-plus-05-2026, 436K)
**Winner by availability:** Mistral + Cohere (оба ✅)
**Cerebras:** недоступен из-за блокировки региона сервера

---

## Ключевые выводы

1. **Cohere имеет наибольший контекст** (436K) — лучший для обработки больших документов
2. **Mistral имеет наивысший score** (240) —  richest capabilities (reasoning + vision + tools + finetune в одной модели)
3. **Cohere предлагает больше моделей** для специализированных задач (ASR, Translate, Embed)
4. **Cerebras недоступен** — требуется зеркало или прокси для API. Ключи в vault бесполезны без сетевого доступа.
5. **Mistral Free Tier** — единственный полностью бесплатный доступ (с лимитами 1 req/sec)
