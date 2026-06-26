"""Webhook handler для push-уведомлений от Manus.

Features:
  - Auto-registration при старте (MANUS_WEBHOOK_URL)
  - RSA signature verification (существующий код сохранён)
  - Redis storage для task_id → status (TTL 1h)
  - Event routing: task_created, task_completed, task_failed
  - GET /webhook/status — статус сервера
  - Attachment download при task_completed
"""

import asyncio
import base64
import hashlib
import json
import logging
import os
import time
from datetime import datetime, timezone
from typing import Any, Dict, Optional

import httpx
import redis.asyncio as redis
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.exceptions import InvalidSignature
from fastapi import FastAPI, Request, HTTPException
from fastapi.responses import JSONResponse

# ── Imports from sibling modules ──────────────────────────────────────────────
import sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from manus_client import ManusClient, get_result, extract_attachments  # noqa: E402

logger = logging.getLogger(__name__)

# ── App ───────────────────────────────────────────────────────────────────────
app = FastAPI(title="Manus Webhook Handler", version="2.0.0")

# ── Configuration from environment ────────────────────────────────────────────
MANUS_API_KEY = os.environ.get("MANUS_API_KEY", "")
MANUS_WEBHOOK_URL = os.environ.get("MANUS_WEBHOOK_URL", "")
REDIS_URL = os.environ.get("REDIS_URL", "redis://localhost:6379/0")
OUTPUT_DIR = os.environ.get("OUTPUT_DIR", "/root/LabDoctorM/projects/free-api-hunter/output")
WEBHOOK_PORT = int(os.environ.get("WEBHOOK_PORT", "8090"))

# ── Runtime state ────────────────────────────────────────────────────────────
_startup_time: float = time.time()
_webhook_id: Optional[str] = None
_redis_client: Optional[redis.Redis] = None
_manus_client: Optional[ManusClient] = None

# ── Public key cache (existing logic preserved) ──────────────────────────────
_public_key_cache: Optional[bytes] = None
_last_key_fetch: float = 0
KEY_CACHE_DURATION = 3600  # 1 hour

PUBLIC_KEY_URL = "https://api.manus.ai/v2/webhook/publicKey"

# ── Redis constants ──────────────────────────────────────────────────────────
REDIS_KEY_PREFIX = "manus:webhook:task"
REDIS_TTL = 3600  # 1 hour


# ═══════════════════════════════════════════════════════════════════════════════
# Public Key Fetching (existing logic, preserved)
# ═══════════════════════════════════════════════════════════════════════════════

async def _fetch_public_key() -> bytes:
    """Получить публичный ключ Manus для проверки подписи."""
    global _public_key_cache, _last_key_fetch

    now = time.time()
    if _public_key_cache and (now - _last_key_fetch < KEY_CACHE_DURATION):
        return _public_key_cache

    async with httpx.AsyncClient() as client:
        resp = await client.get(PUBLIC_KEY_URL)
        resp.raise_for_status()
        data = resp.json()

    # Берём первый ключ
    if isinstance(data, list) and len(data) > 0:
        key_pem = data[0].get("key", "")
    elif isinstance(data, dict):
        keys = data.get("keys", [])
        key_pem = keys[0].get("key", "") if keys else ""
    else:
        raise ValueError(f"Unexpected public key format: {type(data)}")

    _public_key_cache = key_pem.encode()
    _last_key_fetch = now
    logger.info("Fetched and cached Manus public key")
    return _public_key_cache


def _verify_signature(public_key_pem: bytes, url: str, body: bytes, signature_b64: str, timestamp: str) -> bool:
    """Проверить подпись webhook от Manus. (Существующая логика, сохранена)"""
    try:
        # Проверка актуальности метки времени (окно 5 минут)
        if abs(int(time.time()) - int(timestamp)) > 300:
            logger.warning(f"Webhook timestamp too old: {timestamp}")
            return False

        # Восстановление подписанного содержимого
        body_hash = hashlib.sha256(body).hexdigest()
        signed_content = f"{timestamp}.{url}.{body_hash}".encode()

        # Загрузка публичного ключа
        key = serialization.load_pem_public_key(public_key_pem)

        # Проверка подписи
        signature = base64.b64decode(signature_b64)
        key.verify(
            signature,
            signed_content,
            padding.PKCS1v15(),
            hashes.SHA256(),
        )
        return True

    except InvalidSignature:
        logger.warning("Invalid webhook signature")
        return False
    except Exception as e:
        logger.error(f"Signature verification error: {e}")
        return False


# ═══════════════════════════════════════════════════════════════════════════════
# Redis helpers
# ═══════════════════════════════════════════════════════════════════════════════

async def _get_redis() -> redis.Redis:
    """Get or create Redis connection."""
    global _redis_client
    if _redis_client is None:
        _redis_client = redis.Redis.from_url(REDIS_URL, decode_responses=True)
    return _redis_client


async def _store_task_status(task_id: str, status: str, extra: Optional[Dict] = None):
    """Store task status in Redis with TTL."""
    r = await _get_redis()
    key = f"{REDIS_KEY_PREFIX}:{task_id}"
    data = {"status": status, "updated_at": datetime.now(timezone.utc).isoformat()}
    if extra:
        data.update(extra)
    await r.setex(key, REDIS_TTL, json.dumps(data))
    logger.debug(f"Redis: stored {task_id} → {status} (TTL {REDIS_TTL}s)")


async def _get_task_status(task_id: str) -> Optional[Dict]:
    """Get task status from Redis."""
    r = await _get_redis()
    key = f"{REDIS_KEY_PREFIX}:{task_id}"
    raw = await r.get(key)
    if raw:
        return json.loads(raw)
    return None


# ═══════════════════════════════════════════════════════════════════════════════
# Event Handlers
# ═══════════════════════════════════════════════════════════════════════════════

async def _handle_task_created(task_detail: Dict[str, Any]):
    """Обработка события task_created: логируем, сохраняем в Redis."""
    task_id = task_detail.get("task_id", "unknown")
    task_title = task_detail.get("task_title", "")
    logger.info(f"Task created: {task_id} — {task_title}")

    await _store_task_status(task_id, "created", {
        "task_title": task_title,
    })


async def _handle_task_completed(task_detail: Dict[str, Any]):
    """Обработка события task_completed: получаем результат, скачиваем файлы."""
    task_id = task_detail.get("task_id", "unknown")
    logger.info(f"Task completed: {task_id}, fetching result...")

    if not _manus_client:
        logger.error("ManusClient not initialized, cannot fetch result")
        await _store_task_status(task_id, "completed_no_client")
        return

    try:
        # Fetch full result via ManusClient.get_result()
        result = await _manus_client.get_result(task_id)
        status = result.get("status", "unknown")
        attachments = result.get("attachments", [])

        # Store result in Redis
        await _store_task_status(task_id, status, {
            "content_length": len(result.get("structured_output", {}) or {}),
            "attachment_count": len(attachments),
        })

        # Save result to file
        os.makedirs(OUTPUT_DIR, exist_ok=True)
        result_file = os.path.join(OUTPUT_DIR, f"webhook_result_{task_id}.json")
        with open(result_file, "w", encoding="utf-8") as f:
            json.dump(result, f, indent=2, ensure_ascii=False, default=str)
        logger.info(f"Result saved to {result_file}")

        # Download attachments
        if attachments:
            logger.info(f"Downloading {len(attachments)} attachments for {task_id}...")
            for att in attachments:
                url = att.get("url", "")
                filename = att.get("filename", f"file_{task_id}")
                if url:
                    try:
                        content = await _manus_client.download_file(url)
                        att_path = os.path.join(OUTPUT_DIR, f"{task_id}_{filename}")
                        with open(att_path, "wb") as f:
                            f.write(content)
                        logger.info(f"  ✅ {filename} → {att_path} ({len(content)} bytes)")
                    except Exception as e:
                        logger.warning(f"  ❌ Failed to download {filename}: {e}")

    except Exception as e:
        logger.error(f"Error handling task_completed for {task_id}: {e}")
        await _store_task_status(task_id, "completed_error", {"error": str(e)})


async def _handle_task_failed(task_detail: Dict[str, Any]):
    """Обработка события task_failed: логируем ошибку."""
    task_id = task_detail.get("task_id", "unknown")
    error_message = task_detail.get("error_message", task_detail.get("message", ""))
    logger.error(f"Task failed: {task_id} — {error_message}")

    await _store_task_status(task_id, "failed", {
        "error_message": error_message,
    })


async def _handle_task_stopped(task_detail: Dict[str, Any]):
    """Обработка события task_stopped (legacy)."""
    task_id = task_detail.get("task_id")
    stop_reason = task_detail.get("stop_reason", "")
    attachments = task_detail.get("attachments", [])

    logger.info(f"Task stopped: {task_id} reason={stop_reason}")
    await _store_task_status(task_id, "stopped", {"stop_reason": stop_reason})

    for att in attachments:
        logger.info(f"  Attachment: {att.get('filename')} ({att.get('size_bytes')} bytes)")


# ═══════════════════════════════════════════════════════════════════════════════
# Auto-registration
# ═══════════════════════════════════════════════════════════════════════════════

async def _auto_register_webhook():
    """Auto-register webhook on startup if MANUS_WEBHOOK_URL is set."""
    global _webhook_id, _manus_client

    if not MANUS_WEBHOOK_URL:
        logger.info("MANUS_WEBHOOK_URL not set, skipping auto-registration")
        return

    if not MANUS_API_KEY:
        logger.warning("MANUS_API_KEY not set, cannot register webhook")
        return

    _manus_client = ManusClient(api_key=MANUS_API_KEY)

    try:
        result = await _manus_client.register_webhook(
            url=MANUS_WEBHOOK_URL,
            events=["task_created", "task_completed", "task_failed"],
        )
        _webhook_id = result.get("webhook_id", result.get("id", "unknown"))
        logger.info(f"✅ Webhook auto-registered: id={_webhook_id}, url={MANUS_WEBHOOK_URL}")
    except Exception as e:
        logger.error(f"❌ Webhook auto-registration failed: {e}")


# ═══════════════════════════════════════════════════════════════════════════════
# Startup / Shutdown
# ═══════════════════════════════════════════════════════════════════════════════

@app.on_event("startup")
async def startup_event():
    """Auto-register webhook and initialize Redis on startup."""
    global _startup_time
    _startup_time = time.time()
    logger.info("Webhook handler starting...")
    await _auto_register_webhook()
    logger.info(f"Webhook handler ready on port {WEBHOOK_PORT}")


@app.on_event("shutdown")
async def shutdown_event():
    """Cleanup on shutdown."""
    global _redis_client, _manus_client
    if _redis_client:
        await _redis_client.close()
    if _manus_client:
        await _manus_client.close()
    logger.info("Webhook handler stopped")


# ═══════════════════════════════════════════════════════════════════════════════
# Endpoints
# ═══════════════════════════════════════════════════════════════════════════════

@app.post("/webhooks")
async def handle_webhook(request: Request):
    """Обработка входящих webhook от Manus.

    Preserves existing signature verification.
    Adds: task_completed, task_failed event handling + Redis storage.
    """
    body = await request.body()
    url = str(request.url)

    # Получить заголовки подписи
    signature = request.headers.get("x-manus-signature")
    timestamp = request.headers.get("x-manus-timestamp")

    if not signature or not timestamp:
        raise HTTPException(status_code=400, detail="Missing signature headers")

    # Проверить подпись (existing logic preserved)
    public_key = await _fetch_public_key()
    if not _verify_signature(public_key, url, body, signature, timestamp):
        raise HTTPException(status_code=401, detail="Invalid signature")

    # Обработать событие
    data = json.loads(body)
    event_type = data.get("event_type", "unknown")
    task_detail = data.get("task_detail", {})
    task_id = task_detail.get("task_id", "unknown")

    logger.info(f"Webhook: {event_type} for task {task_id}")

    # Маршрутизация по типу события
    if event_type == "task_created":
        await _handle_task_created(task_detail)
    elif event_type == "task_completed":
        await _handle_task_completed(task_detail)
    elif event_type == "task_failed":
        await _handle_task_failed(task_detail)
    elif event_type == "task_stopped":
        await _handle_task_stopped(task_detail)
    else:
        logger.info(f"Unknown event type: {event_type}")

    return JSONResponse({"ok": True})


@app.get("/webhook/health")
async def health_check():
    """Health check endpoint."""
    redis_ok = False
    try:
        r = await _get_redis()
        await r.ping()
        redis_ok = True
    except Exception:
        pass

    return JSONResponse({
        "status": "healthy" if redis_ok else "degraded",
        "redis": "connected" if redis_ok else "disconnected",
        "webhook_registered": _webhook_id is not None,
        "webhook_id": _webhook_id,
        "uptime_seconds": int(time.time() - _startup_time),
        "timestamp": datetime.now(timezone.utc).isoformat(),
    })


@app.get("/webhook/status")
async def status_check():
    """Detailed status: registered webhook_id, uptime, redis connectivity."""
    r = await _get_redis()

    # Count tracked tasks
    task_count = 0
    try:
        keys = await r.keys(f"{REDIS_KEY_PREFIX}:*")
        task_count = len(keys)
    except Exception:
        pass

    return JSONResponse({
        "webhook_id": _webhook_id,
        "webhook_url": MANUS_WEBHOOK_URL or "(not set)",
        "uptime_seconds": int(time.time() - _startup_time),
        "uptime_human": _format_uptime(time.time() - _startup_time),
        "tracked_tasks": task_count,
        "redis_url": REDIS_URL,
        "version": "2.0.0",
        "timestamp": datetime.now(timezone.utc).isoformat(),
    })


@app.get("/webhook/task/{task_id}")
async def get_task_status_endpoint(task_id: str):
    """Get status of a specific task from Redis."""
    status = await _get_task_status(task_id)
    if status is None:
        raise HTTPException(status_code=404, detail=f"Task {task_id} not found")
    return JSONResponse({"task_id": task_id, **status})


# ═══════════════════════════════════════════════════════════════════════════════
# Helpers
# ═══════════════════════════════════════════════════════════════════════════════

def _format_uptime(seconds: float) -> str:
    """Format uptime as human-readable string."""
    h, rem = divmod(int(seconds), 3600)
    m, s = divmod(rem, 60)
    return f"{h}h {m}m {s}s"


# ═══════════════════════════════════════════════════════════════════════════════
# Entry point
# ═══════════════════════════════════════════════════════════════════════════════

if __name__ == "__main__":
    import uvicorn
    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
    uvicorn.run(app, host="0.0.0.0", port=WEBHOOK_PORT)
