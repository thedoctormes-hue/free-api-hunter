"""Тесты для webhook_handler.py — auto-registration, event handling, Redis, endpoints."""

import asyncio
import json
import os
import sys
import time
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from webhook_handler import (
    app,
    _handle_task_created,
    _handle_task_completed,
    _handle_task_failed,
    _handle_task_stopped,
    _store_task_status,
    _get_task_status,
    _fetch_public_key,
    _verify_signature,
    REDIS_KEY_PREFIX,
)


# ── Fixtures ─────────────────────────────────────────────────────────────────

@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture(autouse=True)
def clear_redis_mock():
    """Ensure Redis mock is clean between tests."""
    from webhook_handler import _redis_client
    _redis_client = None


# ── _handle_task_created ─────────────────────────────────────────────────────

class TestHandleTaskCreated:

    @pytest.mark.asyncio
    async def test_stores_status_in_redis(self):
        """task_created stores 'created' status in Redis."""
        with patch("webhook_handler._store_task_status", new_callable=AsyncMock) as mock_store:
            await _handle_task_created({
                "task_id": "task-001",
                "task_title": "Test task",
            })
            mock_store.assert_called_once_with("task-001", "created", {"task_title": "Test task"})

    @pytest.mark.asyncio
    async def test_logs_task_info(self):
        """task_created logs task_id and title."""
        with patch("webhook_handler._store_task_status", new_callable=AsyncMock), \
             patch("webhook_handler.logger") as mock_logger:
            await _handle_task_created({
                "task_id": "task-002",
                "task_title": "Research AI",
            })
            # Verify logger.info was called with task details
            assert mock_logger.info.called
            log_msg = str(mock_logger.info.call_args)
            assert "task-002" in log_msg or "Research AI" in log_msg


# ── _handle_task_completed ───────────────────────────────────────────────────

class TestHandleTaskCompleted:

    @pytest.mark.asyncio
    async def test_stores_completed_status(self):
        """task_completed stores status in Redis."""
        mock_client = AsyncMock()
        mock_client.get_result.return_value = {
            "task_id": "task-003",
            "text": "Done",
            "attachments": [],
            "structured_output": None,
            "status": "stopped",
        }
        with patch("webhook_handler._manus_client", mock_client), \
             patch("webhook_handler._store_task_status", new_callable=AsyncMock) as mock_store, \
             patch("builtins.open", MagicMock()):
            await _handle_task_completed({"task_id": "task-003"})

        # Should store completed status
        call_args = mock_store.call_args
        assert call_args[0][0] == "task-003"
        assert call_args[0][1] == "stopped"

    @pytest.mark.asyncio
    async def test_downloads_attachments(self):
        """task_completed downloads attachments."""
        mock_client = AsyncMock()
        mock_client.get_result.return_value = {
            "task_id": "task-004",
            "text": "Done",
            "attachments": [
                {"url": "https://example.com/file.pdf", "filename": "file.pdf"}
            ],
            "structured_output": None,
            "status": "stopped",
        }
        mock_client.download_file.return_value = b"PDF content"
        with patch("webhook_handler._manus_client", mock_client), \
             patch("webhook_handler._store_task_status", new_callable=AsyncMock), \
             patch("builtins.open", MagicMock()):
            await _handle_task_completed({"task_id": "task-004"})
        mock_client.download_file.assert_called_once_with("https://example.com/file.pdf")

    @pytest.mark.asyncio
    async def test_handles_no_client_gracefully(self):
        """task_completed handles missing ManusClient gracefully."""
        with patch("webhook_handler._manus_client", None), \
             patch("webhook_handler._store_task_status", new_callable=AsyncMock) as mock_store:
            await _handle_task_completed({"task_id": "task-005"})
        mock_store.assert_called_once_with("task-005", "completed_no_client")


# ── _handle_task_failed ──────────────────────────────────────────────────────

class TestHandleTaskFailed:

    @pytest.mark.asyncio
    async def test_stores_failed_status(self):
        """task_failed stores 'failed' status in Redis."""
        with patch("webhook_handler._store_task_status", new_callable=AsyncMock) as mock_store:
            await _handle_task_failed({
                "task_id": "task-006",
                "error_message": "Out of credits",
            })
            mock_store.assert_called_once()
            call_args = mock_store.call_args
            assert call_args[0][0] == "task-006"
            assert call_args[0][1] == "failed"
            assert call_args[0][2]["error_message"] == "Out of credits"


# ── _handle_task_stopped (legacy) ────────────────────────────────────────────

class TestHandleTaskStopped:

    @pytest.mark.asyncio
    async def test_stores_stopped_status(self):
        """task_stopped stores 'stopped' status in Redis."""
        with patch("webhook_handler._store_task_status", new_callable=AsyncMock) as mock_store:
            await _handle_task_stopped({
                "task_id": "task-007",
                "stop_reason": "user_cancelled",
                "attachments": [{"filename": "partial.pdf", "size_bytes": 1024}],
            })
            mock_store.assert_called_once()
            call_args = mock_store.call_args
            assert call_args[0][0] == "task-007"
            assert call_args[0][1] == "stopped"


# ── Signature verification (existing logic preserved) ────────────────────────

class TestSignatureVerification:

    def test_rejects_old_timestamp(self):
        """Signature verification rejects timestamps older than 5 minutes."""
        import time
        old_ts = str(int(time.time()) - 600)  # 10 minutes ago
        result = _verify_signature(b"fake-key", "http://test", b"body", "sig", old_ts)
        assert result is False

    def test_invalid_signature_returns_false(self):
        """Invalid signature returns False, not exception."""
        result = _verify_signature(b"fake-key", "http://test", b"body", "invalid-base64!!!", str(int(time.time())))
        assert result is False


# ── Public key cache ─────────────────────────────────────────────────────────

class TestPublicKeyCache:

    @pytest.mark.asyncio
    async def test_uses_cache_within_ttl(self):
        """Within TTL, cached key is returned without fetching."""
        import webhook_handler as wh
        old_cache = wh._public_key_cache
        old_fetch = wh._last_key_fetch

        wh._public_key_cache = b"cached-key"
        wh._last_key_fetch = time.time()

        result = await wh._fetch_public_key()
        assert result == b"cached-key"

        wh._public_key_cache = old_cache
        wh._last_key_fetch = old_fetch

    @pytest.mark.asyncio
    async def test_refreshes_after_ttl(self):
        """After TTL expires, key is re-fetched."""
        from webhook_handler import _public_key_cache, _last_key_fetch, KEY_CACHE_DURATION
        _public_key_cache = b"old-key"
        _last_key_fetch = time.time() - KEY_CACHE_DURATION - 1

        mock_resp = MagicMock()
        mock_resp.json.return_value = [{"key": "new-key-pem"}]
        mock_resp.raise_for_status = MagicMock()

        mock_client = AsyncMock()
        mock_client.get.return_value = mock_resp

        with patch("httpx.AsyncClient") as mock_httpx:
            mock_httpx.return_value.__aenter__ = AsyncMock(return_value=mock_client)
            mock_httpx.return_value.__aexit__ = AsyncMock(return_value=False)
            result = await _fetch_public_key()

        assert result == b"new-key-pem"


# ── Redis helpers ────────────────────────────────────────────────────────────

class TestRedisHelpers:

    @pytest.mark.asyncio
    async def test_store_and_get_task_status(self):
        """Store and retrieve task status via Redis mock."""
        mock_redis = AsyncMock()
        mock_redis.setex = AsyncMock()
        mock_redis.get.return_value = json.dumps({
            "status": "created",
            "updated_at": "2026-06-25T22:00:00+00:00",
        })

        with patch("webhook_handler._get_redis", new_callable=AsyncMock) as mock_get_redis:
            mock_get_redis.return_value = mock_redis

            await _store_task_status("task-100", "created", {"task_title": "Test"})
            mock_redis.setex.assert_called_once()

            result = await _get_task_status("task-100")
            assert result is not None
            assert result["status"] == "created"

    @pytest.mark.asyncio
    async def test_get_missing_task_returns_none(self):
        """Getting a non-existent task returns None."""
        mock_redis = AsyncMock()
        mock_redis.get.return_value = None

        with patch("webhook_handler._get_redis", new_callable=AsyncMock) as mock_get_redis:
            mock_get_redis.return_value = mock_redis
            result = await _get_task_status("nonexistent")
            assert result is None


# ── App routes ───────────────────────────────────────────────────────────────

class TestAppRoutes:

    def test_webhook_endpoint_exists(self):
        """POST /webhooks route is registered."""
        routes = [r.path for r in app.routes]
        assert "/webhooks" in routes

    def test_health_endpoint_exists(self):
        """GET /webhook/health route is registered."""
        routes = [r.path for r in app.routes]
        assert "/webhook/health" in routes

    def test_status_endpoint_exists(self):
        """GET /webhook/status route is registered."""
        routes = [r.path for r in app.routes]
        assert "/webhook/status" in routes

    def test_task_status_endpoint_exists(self):
        """GET /webhook/task/{task_id} route is registered."""
        routes = [r.path for r in app.routes]
        assert "/webhook/task/{task_id}" in routes


# ── Version / metadata ───────────────────────────────────────────────────────

class TestMetadata:

    def test_app_version(self):
        """App version is 2.0.0."""
        assert app.version == "2.0.0"

    def test_app_title(self):
        """App title contains 'Manus'."""
        assert "Manus" in app.title


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
