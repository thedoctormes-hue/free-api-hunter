# Free API Hunter - REST API Reference

This document describes the REST API endpoints provided by the Free API Hunter backend.

## Base URL

`http://<host>:8090/api/v1`

## Endpoints

### Providers

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/providers` | List all providers (LLM and TTS) |
| `GET` | `/providers/{id}` | Get details for a specific provider |
| `POST` | `/provider-status` | Update provider verification status (web triage). **Public** — no `X-API-Key` required. Body: `{"name": "<provider>", "status": "<verified|confirmed|claimed|unverified|expired|deprioritized|blocked>"}`. Returns `{"success": true}` on success; `400` for unknown provider name or invalid status. |

> **Auth note:** read endpoints (`/providers`, `/findings`, `/stats`, `/scan-history`, `/tts/providers`, `/tts/stats`) and the verification write endpoints (`/findings/verdict`, `/provider-status`) are public. Only the mutating `/scan` trigger requires `X-API-Key`.

### Findings

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/findings` | List all discovered API findings |

### Statistics

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/stats` | General statistics (total findings, providers, etc.) |
| `GET` | `/tts/stats` | Detailed statistics for TTS providers (chars, rotation) |

### TTS Specific

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/tts/providers` | List all TTS providers |
| `GET` | `/tts/providers/{id}` | Get details for a specific TTS provider |

### Scanning

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/scan` | Manually trigger a new scan (stub) |
| `GET` | `/scan-history` | Get history of recent scans |

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Extended health check |

## Response Formats

All responses are returned in **JSON** format.

### Example: `GET /api/v1/providers`

**Success Response (200 OK):**
```json
[
  {
    "id": "mistral",
    "name": "Mistral",
    "type": "llm",
    "status": "active",
    "models": ["mistral-small-latest", "mistral-large-latest"],
    "limits": {
      "rpm": 50,
      "tpm": 50000
    }
  }
]
```

### Error Formats

Errors are returned with an appropriate HTTP status code and a JSON body.

**Example: 404 Not Found**
```json
{
  "error": "provider not found"
}
```
