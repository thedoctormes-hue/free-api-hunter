# Manus API v2 — VERIFIED reference (fact-checked)

**Compiled:** 2026-07-11 by Сова (owl), free-api-hunter KRV pipeline.
**Provenance (fact-check):**
- `[LIVE]` — verified by direct API call during this session (Сова).
- `[DOCS]` — verified against official Manus docs `open.manus.ai/docs/v2` (web-verify subagent).

**Why this exists:** 7 AI-generated Manus guides were harvested from the Manus agent layer
(KRV `validate-pending` path). Audit found 6/7 are near-duplicates of one core; the only
hard factual error across them (`refresh_credits:1500`) is corrected below. All other endpoint
claims were web-verified against official docs.

---

## 1. Base URL & authentication  `[LIVE]` `[DOCS]`
- **Base URL:** `https://api.manus.ai`
- **Auth:** header `x-manus-api-key: <your_api_key>`
- (OAuth2 `Authorization: Bearer <token>` also supported per docs.)

## 2. Validate key & check credits  `[LIVE]` `[DOCS]`
- **GET** `/v2/usage.availableCredits`
- Response shape: `{ "ok": true, "data": { "total_credits": <int>, "free_credits": <int>, "periodic_credits": <int>, "addon_credits": <int>, ... } }`
- **LIVE-observed fields:** `total_credits` (seen `300` when fresh, then **negative** `-1/-7/-7/-3/-4` when exhausted), and `next_refresh_time` (unix timestamp, e.g. `1783803600`).
- ⚠️ **CORRECTION:** a harvested guide claimed `refresh_credits: 1500` / `refresh_interval: "daily"`. **FALSE** — not present in live responses nor in official docs. Credits can go negative; there is no 1500 daily refill buffer.

## 3. Create a task  `[LIVE]` `[DOCS]`
- **POST** `/v2/task.create`
- Body:
  ```json
  {
    "message": { "content": [ { "type": "text", "text": "..." } ] },
    "agent_profile": "manus-1.6-lite"
  }
  ```
- `message.content`: canonical form is an **array** of `{type:"text", text:"..."}` objects `[DOCS]`. A plain string `"..."` **also works** `[LIVE]` — our dispatcher used a string and tasks were created successfully.
- `agent_profile` (optional): `"manus-1.6"` (standard) | `"manus-1.6-lite"` (fast) | `"manus-1.6-max"` (max capability) `[DOCS]`.
- Other optional params `[DOCS]`: `interactive_mode` (bool, default false), `hide_in_task_list` (bool, default false).
- Returns a `task_id`.

## 4. Poll / retrieve results  `[LIVE]` `[DOCS]`
- **GET** `/v2/task.list` — list tasks; paginated via `has_more` + `next_cursor` `[LIVE]`.
- **GET** `/v2/task.listMessages?task_id=<id>&order=asc|desc&limit=<1-200>&cursor=<cursor>` — event stream `[LIVE]`.
  - Task result text → `assistant_message.content`.
  - Generated files → `assistant_message.attachments[]` (pre-signed URLs, downloadable; retrieval is **free**, does not consume credits).
- **GET** `/v2/task.detail?task_id=<id>` — high-level status `[DOCS]`.
- Status values: `running` | `stopped` (done) | `waiting` (needs input → reply via `task.sendMessage`/`task.confirmAction`) | `error`.

## 5. Extended endpoints  `[DOCS]`
All web-verified against official Manus docs:
- **POST** `/v2/task.sendMessage` — continue/reply to a task.
- **POST** `/v2/task.confirmAction` — approve a `waiting` action.
- **GET** `/v2/agent.list` — list agent profiles.
- **POST** `/v2/webhook.create` , **GET** `/v2/webhook.publicKey` — webhook notifications (prefer over polling in production).
- **POST** `/v2/file.upload` (2-step presigned URL) , **GET** `/v2/file.detail`.
- **POST** `/v2/project.create` , **GET** `/v2/project.list`.
- **GET** `/v2/skill.list` , **GET** `/v2/connector.list`.

## 6. Rate limits (requests / minute)  `[LIVE partial]` `[DOCS]`
| Endpoint | Limit/min |
| :--- | :--- |
| `task.create` | 10 |
| `task.sendMessage` | 10 |
| `task.listMessages` | 100 |
| `task.detail` | 100 |
| `file.upload` | 40 |
| `project.create` | 40 |
| `webhook.create` | 40 |
| `task.confirmAction` | 40 |

- `[LIVE]`: we hit HTTP `429` (`error.code: "rate_limited"`) on `task.create` during rapid dispatch → the 10/min ceiling is real.
- All 8 values match official docs exactly.
- On `429`: exponential backoff with jitter; prefer webhooks over `task.listMessages` polling (consumes the 100/min budget fast).

## 7. Fact-check corrections (removed from raw material)
1. **`refresh_credits:1500` / `refresh_interval:"daily"`** — FABRICATED in harvested guide `ZPJj…`; absent from live data and official docs. Removed.
2. **`task.sendMessage` → fixed `task_id: "agent-default-main_task"`** — the *endpoint* is real, but a fixed "main agent" id is **not documented**; treat as unverified. Our task ids are random (e.g. `ZPJjAPjyaABsMvKq8ysdtS`).

---
*Source artifacts (7 raw Manus-generated guides) were audited, fact-checked, and deleted per KRV cleanup. This file is the sole retained, verified artifact.*
