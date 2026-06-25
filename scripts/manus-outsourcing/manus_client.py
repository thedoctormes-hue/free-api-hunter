"""Низкоуровневый клиент для Manus API v2."""

import asyncio
import json
import logging
from typing import Any, Dict, List, Optional

import aiohttp

logger = logging.getLogger(__name__)

BASE_URL = "https://api.manus.ai/v2/"


class ManusError(Exception):
    def __init__(self, code: str, message: str, status: int = 0):
        self.code = code
        self.message = message
        self.status = status
        super().__init__(f"[{code}] {message}")


class ManusClient:
    def __init__(self, api_key: str, base_url: str = BASE_URL):
        self.api_key = api_key
        self.base_url = base_url.rstrip("/") + "/"
        self._session: Optional[aiohttp.ClientSession] = None

    async def _get_session(self) -> aiohttp.ClientSession:
        if self._session is None or self._session.closed:
            self._session = aiohttp.ClientSession(
                headers={
                    "x-manus-api-key": self.api_key,
                    "Content-Type": "application/json",
                },
                timeout=aiohttp.ClientTimeout(total=30),
            )
        return self._session

    async def _request(
        self,
        method: str,
        endpoint: str,
        json_data: Optional[Dict] = None,
        params: Optional[Dict] = None,
        raw_data: Optional[bytes] = None,
    ) -> Dict[str, Any]:
        sess = await self._get_session()
        url = f"{self.base_url}{endpoint}"

        try:
            if raw_data is not None:
                async with sess.request(method, url, data=raw_data, params=params) as resp:
                    return await self._handle_response(resp)
            else:
                async with sess.request(method, url, json=json_data, params=params) as resp:
                    return await self._handle_response(resp)
        except aiohttp.ClientError as e:
            raise ManusError("connection_error", str(e))

    async def _handle_response(self, resp: aiohttp.ClientResponse) -> Dict[str, Any]:
        data = await resp.json()

        if resp.status == 200:
            if data.get("ok") is False:
                raise ManusError(
                    data.get("error", {}).get("code", "unknown"),
                    data.get("error", {}).get("message", "Unknown error"),
                    resp.status,
                )
            return data

        if resp.status == 429:
            raise ManusError("rate_limited", "Rate limit exceeded", 429)
        if resp.status == 401:
            raise ManusError("unauthorized", "Invalid API key", 401)
        if resp.status == 404:
            raise ManusError("not_found", "Resource not found", 404)

        raise ManusError("http_error", f"HTTP {resp.status}", resp.status)

    async def create_task(
        self,
        message: str,
        structured_output_schema: Optional[Dict] = None,
        agent_profile: Optional[str] = None,
        locale: Optional[str] = None,
        project_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        payload: Dict[str, Any] = {"message": {"content": message}}

        if structured_output_schema:
            payload["structured_output_schema"] = structured_output_schema
        if agent_profile:
            payload["agent_profile"] = agent_profile
        if locale:
            payload["locale"] = locale
        if project_id:
            payload["project_id"] = project_id

        return await self._request("POST", "task.create", json_data=payload)

    async def list_messages(
        self,
        task_id: str,
        order: str = "desc",
        limit: int = 20,
    ) -> Dict[str, Any]:
        return await self._request(
            "GET",
            "task.listMessages",
            params={"task_id": task_id, "order": order, "limit": limit},
        )

    async def send_message(
        self,
        task_id: str,
        message: str,
        structured_output_schema: Optional[Dict] = None,
    ) -> Dict[str, Any]:
        payload: Dict[str, Any] = {
            "task_id": task_id,
            "message": {"content": message},
        }
        if structured_output_schema:
            payload["structured_output_schema"] = structured_output_schema
        return await self._request("POST", "task.sendMessage", json_data=payload)

    async def stop_task(self, task_id: str) -> Dict[str, Any]:
        return await self._request("POST", "task.stop", json_data={"task_id": task_id})

    async def upload_file_init(self, filename: str) -> Dict[str, Any]:
        return await self._request("POST", "file.upload", json_data={"filename": filename})

    async def upload_file_content(self, upload_url: str, content: bytes) -> bool:
        sess = await self._get_session()
        async with sess.put(upload_url, data=content) as resp:
            return resp.status == 200

    async def get_credits(self) -> Dict[str, Any]:
        return await self._request("GET", "usage.availableCredits")

    async def get_task_detail(self, task_id: str) -> Dict[str, Any]:
        return await self._request("GET", "task.detail", params={"task_id": task_id})

    def extract_messages(self, response: Dict[str, Any]) -> List[Dict[str, Any]]:
        return response.get("messages", [])

    def extract_attachments(self, response: Dict[str, Any]) -> List[Dict[str, Any]]:
        attachments = []
        for msg in response.get("messages", []):
            if msg.get("type") == "assistant_message":
                am = msg.get("assistant_message", {})
                attachments.extend(am.get("attachments", []))
        return attachments

    def extract_status(self, response: Dict[str, Any]) -> Optional[str]:
        for msg in reversed(response.get("messages", [])):
            if msg.get("type") == "status_update":
                return msg.get("status_update", {}).get("agent_status")
        return None

    def extract_structured_output(self, response: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        for msg in response.get("messages", []):
            if msg.get("type") == "assistant_message":
                am = msg.get("assistant_message", {})
                if "structured_output_result" in am:
                    return am["structured_output_result"]
        return None

    async def download_file(self, url: str) -> bytes:
        sess = await self._get_session()
        async with sess.get(url) as resp:
            resp.raise_for_status()
            return await resp.read()

    async def close(self):
        if self._session and not self._session.closed:
            await self._session.close()

    async def __aenter__(self):
        return self

    async def __aexit__(self, *args):
        await self.close()
