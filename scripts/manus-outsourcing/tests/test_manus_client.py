"""Тесты для manus_client."""

import asyncio
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

import sys
import os
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from manus_client import ManusClient, ManusError


class TestManusClient:

    def setup_method(self):
        self.client = ManusClient(api_key="test-key")

    def teardown_method(self):
        if self.client._session and not self.client._session.closed:
            asyncio.get_event_loop().run_until_complete(self.client.close())

    @pytest.mark.asyncio
    @patch("manus_client.ManusClient._request")
    async def test_create_task(self, mock_request):
        mock_request.return_value = {
            "ok": True,
            "task_id": "test-task-123",
            "task_url": "https://manus.im/app/test-task-123",
        }

        result = await self.client.create_task("Test message")
        assert result["task_id"] == "test-task-123"

    @pytest.mark.asyncio
    @patch("manus_client.ManusClient._request")
    async def test_create_task_with_schema(self, mock_request):
        mock_request.return_value = {"ok": True, "task_id": "t1"}

        schema = {
            "type": "object",
            "properties": {"url": {"type": "string"}},
            "required": ["url"],
        }

        result = await self.client.create_task("Test", structured_output_schema=schema)
        assert result["task_id"] == "t1"

    def test_extract_status(self):
        response = {
            "messages": [
                {"type": "status_update", "status_update": {"agent_status": "stopped"}}
            ]
        }
        status = self.client.extract_status(response)
        assert status == "stopped"

    def test_extract_attachments(self):
        response = {
            "messages": [
                {
                    "type": "assistant_message",
                    "assistant_message": {
                        "content": "Done",
                        "attachments": [
                            {"filename": "test.pptx", "url": "https://example.com/test.pptx"}
                        ],
                    },
                }
            ]
        }
        attachments = self.client.extract_attachments(response)
        assert len(attachments) == 1
        assert attachments[0]["filename"] == "test.pptx"

    def test_extract_structured_output(self):
        response = {
            "messages": [
                {
                    "type": "assistant_message",
                    "assistant_message": {
                        "content": "Done",
                        "structured_output_result": {
                            "success": True,
                            "value": {"url": "https://example.com"},
                        },
                    },
                }
            ]
        }
        result = self.client.extract_structured_output(response)
        assert result is not None
        assert result["success"] is True

    def test_extract_structured_output_failure(self):
        response = {
            "messages": [
                {
                    "type": "assistant_message",
                    "assistant_message": {
                        "content": "Failed",
                        "structured_output_result": {
                            "success": False,
                            "error": "Schema mismatch",
                            "value": None,
                        },
                    },
                }
            ]
        }
        result = self.client.extract_structured_output(response)
        assert result is not None
        assert result["success"] is False

    def test_manus_error(self):
        error = ManusError("rate_limited", "Too many requests", 429)
        assert error.code == "rate_limited"
        assert error.status == 429
        assert "Too many requests" in str(error)


class TestManusClientAPI:
    """Интеграционные тесты (требуют реального API ключа)."""

    def setup_method(self):
        self.api_key = os.environ.get("MANUS_API_KEY", "")

    @pytest.mark.asyncio
    async def test_real_credits(self):
        if not self.api_key:
            pytest.skip("MANUS_API_KEY not set")
        client = ManusClient(api_key=self.api_key)
        try:
            result = await client.get_credits()
            assert result.get("ok")
            assert "total_credits" in result
            print(f"✅ Balance: {result['total_credits']} credits")
        finally:
            await client.close()

    @pytest.mark.asyncio
    async def test_real_create_task(self):
        if not self.api_key:
            pytest.skip("MANUS_API_KEY not set")
        client = ManusClient(api_key=self.api_key)
        try:
            result = await client.create_task("Say hello in one sentence")
            assert result.get("ok")
            assert "task_id" in result
            print(f"✅ Task created: {result['task_id']}")
        finally:
            await client.close()


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
