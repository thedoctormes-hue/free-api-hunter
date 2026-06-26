# Manus Webhook Server — Architecture v2.0.0

## Auto-Registration Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        STARTUP SEQUENCE                             │
│                                                                     │
│  webhook_handler.py                                                 │
│       │                                                             │
│       ├─ ENV: MANUS_API_KEY set? ──No──► Skip registration          │
│       │       │                                                     │
│       │      Yes                                                    │
│       │       │                                                     │
│       ├─ ENV: MANUS_WEBHOOK_URL set? ──No──► Skip registration      │
│       │       │                                                     │
│       │      Yes                                                    │
│       │       │                                                     │
│       ├─ ManusClient.register_webhook(url, events)                  │
│       │       │                                                     │
│       │       ▼                                                     │
│       │  POST /v2/webhook.create                                    │
│       │       │  {url, events: [task_created, task_completed,        │
│       │       │                        task_failed]}                 │
│       │       │                                                     │
│       │       ▼                                                     │
│       │  {webhook_id: "wh_xxx"}                                     │
│       │       │                                                     │
│       │       ▼                                                     │
│       │  Store _webhook_id in memory                                │
│       │  Initialize _manus_client for later use                     │
│       │                                                             │
│       ▼                                                             │
│  FastAPI app ready on :8090                                          │
└─────────────────────────────────────────────────────────────────────┘
```

## Event Processing Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                     WEBHOOK EVENT PROCESSING                         │
│                                                                     │
│  Manus ──POST──► /webhooks (port 8090)                              │
│                      │                                              │
│                      ▼                                              │
│              ┌─ Signature Verification (RSA, preserved) ─┐           │
│              │   x-manus-signature + x-manus-timestamp   │           │
│              │   ±5min window, SHA256 PKCS1v15           │           │
│              └───────────────┬────────────────────────────┘           │
│                              │                                      │
│                              ▼                                      │
│              ┌─ Parse body: event_type + task_detail ──┐            │
│              └───────────────┬──────────────────────────┘            │
│                              │                                      │
│              ┌───────────────┼───────────────┐                      │
│              ▼               ▼               ▼                      │
│     task_created     task_completed    task_failed                  │
│         │                │                │                         │
│         ▼                ▼                ▼                         │
│    ┌─────────┐    ┌────────────┐    ┌──────────┐                   │
│    │ Log it  │    │ get_result │    │ Log err  │                   │
│    │ Redis   │    │ + download │    │ Redis    │                   │
│    │ status= │    │ attachments│    │ status=  │                   │
│    │ created │    │ save file  │    │ failed   │                   │
│    └─────────┘    │ Redis      │    └──────────┘                   │
│                   │ status=    │                                    │
│                   │ stopped    │                                    │
│                   └────────────┘                                    │
└─────────────────────────────────────────────────────────────────────┘
```

## Redis Storage Schema

```
Key: manus:webhook:task:{task_id}
Value (JSON):
  {
    "status": "created|stopped|failed",
    "updated_at": "2026-06-25T22:00:00+00:00",
    "task_title": "...",           // optional
    "error_message": "...",        // for failed
    "attachment_count": 3          // for completed
  }
TTL: 3600s (1 hour)
```

## API Endpoints

```
POST /webhooks          — Webhook receiver (signature verified)
GET  /webhook/health    — Health check (redis ping, uptime)
GET  /webhook/status    — Detailed status (webhook_id, tracked tasks)
GET  /webhook/task/{id} — Task status from Redis
```

## Component Diagram

```
┌──────────────────────────────────────────────────────────┐
│                   webhook_handler.py                       │
│                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │ FastAPI  │  │  Redis   │  │  Manus   │               │
│  │ Routes   │  │  Client  │  │  Client  │               │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘               │
│       │              │              │                     │
│  ┌────┴──────────────┴──────────────┴────┐               │
│  │         Event Router                   │               │
│  │  task_created → _handle_task_created   │               │
│  │  task_completed → _handle_task_completed│              │
│  │  task_failed → _handle_task_failed     │               │
│  │  task_stopped → _handle_task_stopped   │               │
│  └────────────────────────────────────────┘               │
│                                                          │
│  ┌────────────────────────────────────────┐              │
│  │  Startup: _auto_register_webhook()     │              │
│  │  Shutdown: close redis + manus client  │              │
│  └────────────────────────────────────────┘              │
└──────────────────────────────────────────────────────────┘
         │              │              │
         ▼              ▼              ▼
    ┌─────────┐  ┌──────────┐  ┌──────────────┐
    │ :8090   │  │  Redis   │  │ api.manus.ai │
    │         │  │  :6379   │  │  (v2 API)    │
    └─────────┘  └──────────┘  └──────────────┘
```

## Key Design Decisions

1. **Signature verification preserved** — existing RSA verification untouched
2. **Auto-registration is optional** — only when both MANUS_API_KEY and MANUS_WEBHOOK_URL are set
3. **Redis TTL 1h** — tasks auto-expire, no manual cleanup
4. **Graceful degradation** — if Redis is down, webhook still responds, health shows "degraded"
5. **Standalone helpers** — `get_result()` and `extract_attachments()` available as module-level functions for easy importing
