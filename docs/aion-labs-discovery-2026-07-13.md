# Aion Labs — Discovery Note (2026-07-13)

Validated live on 2026-07-13 by agent `streikbrecher` using a working Aion Labs API key
(Bearer auth). This note is the versioned record of the discovery; the actual registry
edits live in `data/free-api-hunter.db` + `data/providers.json` (both gitignored under
`data/`).

## Registry status
- Provider `Aion Labs` set to `verified` in KRV Registry (`free-api-hunter.db`,
  table `providers`) and mirrored to `providers.json`.
- Caveat: KRV `validate-keys` timer may recompute `live_status` unless a key is present
  in the vault. This note is the durable record; the DB row can drift on re-validation.

## API
- Base: `https://api.aionlabs.ai/v1` — OpenAI-compatible.
- Auth: `Authorization: Bearer <key>` (also accepts `Api-Key <key>`).
- Public: `GET /v1/models` (no key required).

## Endpoints (verified by live probe)
- `GET /v1/models` — public, lists models.
- `POST /v1/chat/completions` — chat completions, works (HTTP 200).
- `POST /v1/responses` — **UNDOCUMENTED** OpenAI Responses API. Verified working:
  - basic `input` → 200, `object: response`, `status: completed`.
  - streaming (`stream:true`) → 200, SSE `data:` chunks with `delta` / `delta_type: reasoning`.
  - tools: schema accepted (200), but Aion roleplay models (e.g. `aion-2.0`) emit plain text
    instead of a structured `function_call` even with `tool_choice:"required"`. Tool-use is
    supported at the API level but limited by roleplay-model behavior.
- NOT implemented (probed, 404): `/v1/completions`, `/v1/embeddings`, `/v1/moderations`,
  `/v1/images/*`, `/v1/audio/*`, `/v1/files`, `/v1/fine_tuning/jobs`, `/v1/batches`,
  `/v1/rerank`, `/v1/usage`, `/v1/user`, `/v1/organization`, `/v1/dashboard/billing/usage`,
  `/v1/vector_stores`.

## Models (active)
- `aion-labs/aion-2.0` — DeepSeek V3.2 variant, RP/storytelling, 128K ctx, 32K max out,
  reasoning. $0.80 / $0.20 cached / $1.60 per 1M tokens.
- `aion-labs/aion-2.5` — refined RP, released 2026-03-10. $1.00 / $0.35 / $3.00.
- `aion-labs/aion-3.0` — multi-model system on GLM family, 2026-05-05. $3.00 / $0.75 / $6.00.
- `aion-labs/aion-3.0-mini` — multi-model on DeepSeek family, 2026-05-14. $0.70 / $0.18 / $1.40.
- `aion-labs/aion-rp-llama-3.1-8b` — Llama 3.1 8B uncensored RP, 32K ctx.
- Expired: `aion-1.0`, `aion-1.0-mini` (sunset 2026-07-04, replaced by aion-2.0).

## Limits (from official docs)
- Free: 15 RPM, 20k TPM, 20k tokens/day. Tiers 1–5 by lifetime top-up ($100–$1000).

## Pricing rationale (official, from /docs/models/)
Why the same 128K context + uncensored stance still spans $0.70→$3.00 in/out:
- **Base upstream model** is the primary cost driver: GLM family (3.0) > DeepSeek V3.2
  (2.0/2.5) > Llama 3.1 8B (8B). Aion is a router; upstream cost passes through.
- **Multi-model collaborative generation** for 3.0 & 3.0-mini: "multiple specialized models
  contribute to each response" = N inferences per request. 3.0 (GLM ensemble) is priciest;
  3.0-mini (DeepSeek ensemble) is CHEAPER than single 2.0 — proving base matters more
  than the "multi-model" label.
- **Reasoning**: 2.0/2.5/3.0/3.0-mini are reasoning models (hidden reasoning tokens
  ≈2x token burn); 8B is NOT (no reasoning, no cached discount).
- **Positioning**: 2.5 = "more natural/human-like + faster" step-up from 2.0 (justifies $1/$3);
  8B = budget RP niche at 32K ctx, priced parity with 2.0 despite tiny size.
- External latency does NOT track price (live probe: 3.0 fastest at 6.2s yet priciest;
  2.5 slowest at 13.6s at mid price) — confirms cost is internal (upstream+ensemble), not
  observable latency.

## Endpoints confirmation
- Official docs list ONLY `POST /v1/chat/completions` + `GET /v1/models`.
  `POST /v1/responses` is ABSENT from all public Aion Labs docs/ToS → our discovery
  of a working, undocumented Responses endpoint is corroborated.

## Context / reputation
- `aionlabs.ai` is an LLM aggregator/router (per ToS it routes to upstream providers). Do NOT
  confuse with `aionlabs.com` (a separate pharma venture studio).
- Popular in SillyTavern RP communities; Aion-RP 1.0 (8B) tops the RPBench-Auto character eval.
  Described by users as "the drummers but on DeepSeek V3.2".
- The undocumented `/v1/responses` is absent from all public docs/ToS/aggregator listings —
  confirmed novel at test time.

## Sources
- https://www.aionlabs.ai/docs/ , /docs/models/ , /docs/rate-limits/
- OpenRouter (aion-labs), Puter Developer, Krater.ai, Hugging Face (Aion-RP-Llama-3.1-8B)
- Reddit r/SillyTavernAI

## Реальный бенчмарк: MSK Gastro Digest Bot (2026-07-13)
Цель: проверить модели на РЕАЛЬНОЙ прод-задаче бота — литературный дайджест московских
ресторанов из живых постов Telegram-каналов. Бенчмарк НЕ спамит канал публикации
(`@moscovskiiest`): скрипт импортирует только read-only конфиг бота (`VOICE_DNA`,
`VERDICTS`, `DEFAULT_CHANNELS`) и никогда не вызывает `tg_send`. Данные — живой scrape
`t.me/s/<channel>` (без токена, read-only).

Метод:
- Вход: 63 уникальных живых поста из 6 каналов (raidedrests, restaurantmoscow, vkusonomika,
  sysoevfm, thesaltmagazine, michelinwontgive).
- Промпт: точный `VOICE_DNA.format(date=, verdicts=, news=)` бота (лит-голос).
- Параметры: `temperature=0.75` (GENERATION_TEMPERATURE бота), `max_tokens=800` (cap видимого текста).
- По одному прогону на каждую из 5 моделей на одном и том же входе → честное сравнение.
- Артефакты: файлы по моделям (`aion-2.0.md` и т.д.), сводка `index.md`, вход `_META.md`
  (в `/tmp/bench_aion/`, выгружены на Яндекс.Диск по запросу ЗавЛаба).

Результаты (прочитано глазами):
- **aion-2.0** ($1.60 out): ✅ полный лит-дайджест, верный голос, синтез 6+ источников.
  Хук «Москва меняет меню быстрее, чем гардероб.» Чуть проще подача.
- **aion-2.5** ($3.00 out): ✅ лучший синтез (поймал инсайд соусов Miratorg «для фудкоста»,
  «постколониальная гастрономика спальных районов»), самый «живой» тон.
  Хук «Италия, Корея и возврат к истокам».
- **aion-3.0** ($6.00 out): ✅ самый отполированный, богатая образность
  («достойным отдельного гастрономического государства 🍷»). Хук «Москва меняет вывески
  быстрее, чем привыкает.» Полир > суть.
- **aion-3.0-mini** ($1.40 out): ✅ на уровне старших, разговорный голос
  («И знаете, что-то мне подсказывает…»). Хук «Где итальянцы, а где шаурма 🍷🥩». Самый выгодный.
- **aion-rp-llama-3.1-8b** ($1.60 out): ❌ ПРОВАЛ. 95 токенов — погодно-бродская заглушка
  («13 июля, Москва, 27–30°C… дайджест новостей»), без синтеза, без вердикта, без ресторанов.
  Непригоден к задаче.

Выводы (объективный чейнджлог КАЧЕСТВА на реальной задаче):
- Цена НЕ линейна к качеству.
- Все reasoning-модели (2.0 / 2.5 / 3.0 / 3.0-mini) закрывают задачу; различия — в полировке
  и голосе, не в способности.
- 3.0 за $6 — лишь полир + образность поверх 2.0; в ×3.75 дороже, но НЕ в ×3.75 качественнее.
  Переплата на этом кейсе не обоснована.
- 8B за $1.60 (= цена 2.0) — жёсткий провал. Доказывает: дело в reasoning + размере, а не в
  ценнике. Дешёвый БЕЗ reasoning ≠ дешёвый С reasoning.
- Лучший value: **3.0-mini** ($1.40 out) или **2.5** ($3.00 out — за лучший синтез).

Методологическая оговорка: `max_tokens=800` подрезал хвост у verbose-моделей (вердикт обрезан
на 2.0 / 3.0 / 3.0-mini), тело целое. Reasoning-токены (~800 hidden) считаются в
`completion_tokens` сверх видимого лимита, поэтому «out tok» в логах = ~800 hidden + ~800 visible.

## Update 2026-07-13 (evening) — Rate limits, 5-key pool, msk-gastro integration

### Official rate limits (aionlabs.ai/docs/rate-limits/, verified live)
- Tiers by lifetime credit top-up: Free (default) / 1 (any top-up) / 2 ($100) / 3 ($250) / 4 ($500) / 5 ($1000). Tier never decreases.
- Free tier: 15 RPM, 20,000 TPM, **20,000 tokens/day** per key.
- Tier 1+: 50 RPM, 1,000,000 TPM, **UNLIMITED tokens/day** (scales to Tier 5: 1000 RPM, 20M TPM).
- `GET /v1/models` is PUBLIC (no key). Auth accepts `Bearer` OR `Api-Key`.

### 5-key pool (5 accounts, tested 2026-07-13)
- Pool file: `/root/.aion_keys.json` (chmod 600), 5 keys. Live-tested each via `/v1/chat/completions` with model `aion-labs/aion-3.0-mini`.
- Result: **2 valid (200)**, **3 rate-limited (429 "Daily token limit exceeded")** on test day.
  - Valid: `...p1ss` (alv2_p8Tv…), `...DXak` (alv2_xCTV…).
  - Daily-limited: `...x-o4` (alv2_0xSF5…), `...x3xc` (alv2_ScSl…, the previously-exhausted key), `...BaWc` (alv2_x3MRP… — burned by earlier benchmark/verify runs this session).
- Implication: Free-tier daily cap is real and per-key. Effective daily capacity fluctuates (≤100k across 5 keys, often less). A pool MUST track per-key daily exhaustion, rotate, and detect UTC-day reset. If any account is topped up (Tier 1+), that key becomes effectively unlimited.
- NOTE: one key (alv2_x3MRP…) was exhausted by this agent's earlier benchmark/verify calls (~22k tokens) — disclosed honestly.

### msk-gastro-digest-bot integration
- New module `aion_pool.py`: round-robin key acquisition + per-key daily-exhaustion tracking (UTC-day bucket). No secrets in code; keys read from `/root/.aion_keys.json` at runtime.
- `config.py`: `AION_ENABLED`, `AION_URL`, `AION_KEYS_FILE`, `AION_MODELS` (4 reasoning models; excludes `aion-rp-llama-3.1-8b` which failed the creative benchmark), `AION_DEFAULT_MODEL = aion-labs/aion-3.0-mini`.
- `main.py`: `generate()` gained `backend` param. Aion is PRIMARY backend (pool keys); OpenRouter `:free` is fallback. On Aion 429/401 the key is marked exhausted and the next candidate is tried.
- Verified live: `generate()` through the pool returned a proper-voice Moscow gastro digest (model `aion-labs/aion-3.0-mini`, pt=592/ct=120). Aion reasoning models are slow (tens of seconds per full digest) — `GENERATION_TIMEOUT=180` covers it.
- Committed on branch `antcat/restore-env-and-docs` (agent edits auto-commit in this environment).
