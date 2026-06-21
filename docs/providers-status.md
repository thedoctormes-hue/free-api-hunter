# Статус провайдеров — 2026-06-21

## Cohere

**Статус:** ✅ Всё работает
**Ключей:** 4 (eaNrIYAqRw..., 5OkipXnQ0R..., JVU0mq9jtG..., 7cGV7Zre9Q...)
**Лимиты:** 1000 calls/month на ключ, trial 100 calls. Итого 4000 calls/month.

**Топ-уровень (Reasoning):**
- command-a-plus-05-2026 — лучшая модель, 436K ctx, reasoning-first, max_tokens≥200
- command-a-reasoning-08-2025 — отличная, 288K ctx, reasoning-first, max_tokens≥250

**Основные (Chat):**
- command-a-03-2025 — основная рабочая, 288K ctx
- command-r-plus-08-2024 — проверенная, 128K ctx
- c4ai-aya-expanse-32b — мультиязычная, 128K ctx
- command-r-08-2024 — стабильная, 128K ctx
- command-r7b-arabic-02-2025 — арабский язык, 128K ctx

**Лёгкие:**
- command-r7b-12-2024 — быстрая и дешёвая, 132K ctx

**Специализированные:**
- command-a-vision-07-2025 — зрение, 128K ctx
- c4ai-aya-vision-32b — зрение, 16K ctx
- command-a-translate-08-2025 — перевод, 9K ctx
- embed-multilingual-v3.0 — embedding, 1024 dims
- rerank-multilingual-v3.0 — rerank

**Рекомендации:**
- Сложные задачи → command-a-plus-05-2026
- Быстрые ответы → command-r7b-12-2024
- Зрение → command-a-vision-07-2025
- Перевод → command-a-translate-08-2025
- Embedding → embed-multilingual-v3.0

**Эндпоинты:**
- /v2/chat — работает (основной)
- /v1/chat — deprecated, требует миграцию на v2
- /v1/generate — удалён 15.09.2025
- /v1/embed — работает
- /v1/rerank — работает
- /v1/models — работает, 20 моделей

## Cerebras

**Статус:** ✅ Всё работает
**Ключей:** 5 (csk-xd…5tp4, csk-6e…4vk, csk-35…ntf, csk-96…nrh, csk-rp…h62)
**Лимиты:** 5 RPM / 30K tokens/min / 2400 req/day

**Модели:**
- gpt-oss-120b — reasoning-first, быстрая
- zai-glm-4.7 — reasoning-first, мультиязычная

## Инциденты

**INC-20260621:** Штрейкбрехер не отвечал на прямые вопросы ЗавЛаба. Корень: вместо краткого ответа да/нет — длинные объяснения. Решение: отвечать на прямые вопросы кратко, контекст добавлять после.

**lab-vault перезапуск:** Старый snapshot.enc удалён, новый создан с паролем Iam@31415926. Бот @labvaultbot работает. Секреты загружаются через бота КотОлизатором.
