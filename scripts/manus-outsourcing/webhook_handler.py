"""Webhook handler для push-уведомлений от Manus."""

import base64
import hashlib
import json
import logging
import time
from typing import Any, Dict, Optional

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.exceptions import InvalidSignature
from fastapi import FastAPI, Request, HTTPException
from fastapi.responses import JSONResponse

logger = logging.getLogger(__name__)

app = FastAPI(title="Manus Webhook Handler")

# Кэш публичного ключа
_public_key_cache: Optional[bytes] = None
_last_key_fetch: float = 0
KEY_CACHE_DURATION = 3600  # 1 час

PUBLIC_KEY_URL = "https://api.manus.ai/v2/webhook.publicKey"


async def _fetch_public_key() -> bytes:
    """Получить публичный ключ Manus для проверки подписи."""
    global _public_key_cache, _last_key_fetch

    now = time.time()
    if _public_key_cache and (now - _last_key_fetch < KEY_CACHE_DURATION):
        return _public_key_cache

    import httpx
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
    """Проверить подпись webhook от Manus."""
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


@app.post("/webhooks")
async def handle_webhook(request: Request):
    """Обработка входящих webhook от Manus."""
    body = await request.body()
    url = str(request.url)

    # Получить заголовки подписи
    signature = request.headers.get("x-manus-signature")
    timestamp = request.headers.get("x-manus-timestamp")

    if not signature or not timestamp:
        raise HTTPException(status_code=400, detail="Missing signature headers")

    # Проверить подпись
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
    elif event_type == "task_stopped":
        await _handle_task_stopped(task_detail)
    else:
        logger.info(f"Unknown event type: {event_type}")

    return JSONResponse({"ok": True})


async def _handle_task_created(task_detail: Dict[str, Any]):
    """Обработка события task_created."""
    task_id = task_detail.get("task_id")
    task_title = task_detail.get("task_title", "")
    logger.info(f"Task created: {task_id} — {task_title}")


async def _handle_task_stopped(task_detail: Dict[str, Any]):
    """Обработка события task_stopped."""
    task_id = task_detail.get("task_id")
    stop_reason = task_detail.get("stop_reason", "")
    message = task_detail.get("message", "")
    attachments = task_detail.get("attachments", [])

    logger.info(f"Task stopped: {task_id} reason={stop_reason}")

    # Здесь можно запустить обработку результата
    # Например: скачать файлы, обновить баланс, уведомить агента
    for att in attachments:
        logger.info(f"  Attachment: {att.get('filename')} ({att.get('size_bytes')} bytes)")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8090)
