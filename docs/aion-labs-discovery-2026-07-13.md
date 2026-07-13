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
