"""Низкоуровневый клиент для Manus API v2."""

import asyncio
import json
import logging
import os
from dataclasses import dataclass, field
from enum import Enum
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


class TaskStatus(str, Enum):
    """Статусы задачи Manus."""
    RUNNING = "running"
    STOPPED = "stopped"
    WAITING = "waiting"
    ERROR = "error"


@dataclass
class TaskResult:
    """Результат выполненной задачи."""
    text: str
    attachments: List[Dict[str, Any]] = field(default_factory=list)
    structured_output: Optional[Dict[str, Any]] = None


class ManusTaskError(ManusError):
    """Ошибка выполнения задачи."""
    def __init__(self, task_id: str, status: str, message: str):
        super().__init__(f"task_{status}", message)
        self.task_id = task_id
        self.status = status


class ManusTimeoutError(ManusError):
    """Таймаут ожидания завершения задачи."""
    def __init__(self, task_id: str, timeout: int):
        super().__init__("task_timeout", f"Task {task_id} did not complete in {timeout}s")
        self.task_id = task_id
        self.timeout = timeout


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

    async def register_webhook(
        self,
        url: str,
        events: Optional[List[str]] = None,
    ) -> Dict[str, Any]:
        """Register webhook for real-time task notifications."""
        if events is None:
            events = ["task_created", "task_completed", "task_failed"]
        return await self._request("POST", "webhook.create", json_data={
            "url": url,
            "events": events,
        })

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

    # -----------------------------------------------------------
    # New API v2 methods
    # -----------------------------------------------------------

    async def wait_for_completion(
        self,
        task_id: str,
        timeout: int = 300,
        poll_interval: int = 3,
    ) -> List[Dict[str, Any]]:
        """Poll task status via listMessages until stopped/error/timeout.

        Uses task.listMessages (NOT task.detail) for polling.
        Implements exponential backoff after 30 seconds:
        3 → 6 → 12 seconds.

        Returns the full message stream (list of message dicts).
        Raises ManusTimeoutError on timeout.
        Raises ManusTaskError on error or waiting (if no event type).
        """
        start = asyncio.get_event_loop().time()
        current_interval = poll_interval

        while True:
            elapsed = asyncio.get_event_loop().time() - start
            if elapsed >= timeout:
                raise ManusTimeoutError(task_id, timeout)

            messages_resp = await self.list_messages(task_id, order="desc", limit=20)
            messages = messages_resp.get("messages", [])

            # Check status_update events
            status = None
            waiting_for_event = None
            for msg in reversed(messages):
                if msg.get("type") == "status_update":
                    su = msg.get("status_update", {})
                    status = su.get("agent_status")
                    waiting_for_event = su.get("waiting_for_event_type")
                    break

            if status == "stopped":
                return messages

            if status == "running":
                pass  # keep polling

            elif status == "waiting":
                if waiting_for_event:
                    logger.info(
                        "Task %s waiting for: %s — will keep polling",
                        task_id,
                        waiting_for_event,
                    )
                else:
                    raise ManusTaskError(
                        task_id, "waiting", "Task is waiting for user input"
                    )

            elif status == "error":
                # Try to extract error message from messages
                error_msg = "Task ended with error"
                for msg in messages:
                    if msg.get("type") == "error_message":
                        error_msg = msg.get("error_message", {}).get("message", error_msg)
                        break
                raise ManusTaskError(task_id, "error", error_msg)

            await asyncio.sleep(current_interval)

            # Exponential backoff after 30s
            if elapsed > 30:
                if current_interval == 3:
                    current_interval = 6
                elif current_interval == 6:
                    current_interval = 12

    async def get_result(self, task_id: str) -> TaskResult:
        """Extract text result and attachments from a completed task.

        Returns a TaskResult dataclass with text, attachments,
        and optional structured_output.
        """
        messages_resp = await self.list_messages(task_id, order="desc", limit=20)
        messages = messages_resp.get("messages", [])

        text = ""
        attachments: List[Dict[str, Any]] = []
        structured_output: Optional[Dict[str, Any]] = None

        for msg in messages:
            if msg.get("type") == "assistant_message":
                am = msg.get("assistant_message", {})
                # Extract text from content blocks
                for c in am.get("content", []):
                    if c.get("type") == "output_text":
                        text = c.get("text", text)
                # Collect attachments
                attachments.extend(am.get("attachments", []))
                # Extract structured output if present
                if "structured_output_result" in am:
                    structured_output = am["structured_output_result"]

        return TaskResult(
            text=text,
            attachments=attachments,
            structured_output=structured_output,
        )

    async def download_attachments(
        self, task_id: str, output_dir: str = "output"
    ) -> List[str]:
        """Download all attachments from a completed task result.

        Saves files to <output_dir>/manus-output/<task_id>/.
        Returns list of local file paths.
        """
        result = await self.get_result(task_id)
        if not result.attachments:
            return []

        save_dir = os.path.join(output_dir, "manus-output", task_id)
        os.makedirs(save_dir, exist_ok=True)

        local_paths: List[str] = []
        for att in result.attachments:
            url = att.get("url")
            if not url:
                continue
            filename = att.get("filename", "")
            if not filename:
                # Fallback: extract from URL
                filename = url.rsplit("/", 1)[-1].split("?")[0]
            if not filename:
                filename = f"attachment_{len(local_paths)}"

            content = await self.download_file(url)
            filepath = os.path.join(save_dir, filename)
            with open(filepath, "wb") as f:
                f.write(content)
            local_paths.append(filepath)
            logger.info("Downloaded attachment: %s", filepath)

        return local_paths

    async def create_project(
        self, name: str, instruction: str = ""
    ) -> Dict[str, Any]:
        """Create a new Manus project.

        POST /v2/project.create
        Returns the project object (including id).
        """
        payload: Dict[str, Any] = {"name": name}
        if instruction:
            payload["instruction"] = instruction
        return await self._request("POST", "project.create", json_data=payload)

    async def upload_file(self, filepath: str) -> str:
        """Upload a file to Manus for use as task context.

        Steps:
        1. POST /v2/file.upload with filename → get upload_url
        2. PUT bytes to upload_url
        3. Return file_id
        """
        filename = os.path.basename(filepath)
        init = await self.upload_file_init(filename)
        upload_url = init.get("upload_url")
        if not upload_url:
            raise ManusError("upload_error", "No upload_url in response")

        with open(filepath, "rb") as f:
            content = f.read()

        success = await self.upload_file_content(upload_url, content)
        if not success:
            raise ManusError("upload_error", "File upload PUT failed")

        return init.get("file_id", "")

    async def confirm_action(self, task_id: str) -> Dict[str, Any]:
        """Confirm a pending action on a waiting task.

        POST /v2/task.confirmAction
        Used when task is in 'waiting' status.
        """
        return await self._request(
            "POST", "task.confirmAction", json_data={"task_id": task_id}
        )

    async def close(self):
        if self._session and not self._session.closed:
            await self._session.close()

    async def __aenter__(self):
        return self

    async def __aexit__(self, *args):
        await self.close()


# ── Standalone helper functions (for import convenience) ─────────────────────

async def get_result(api_key: str, task_id: str) -> Dict[str, Any]:
    """Convenience: get full result for a task without instantiating ManusClient manually."""
    client = ManusClient(api_key=api_key)
    try:
        result = await client.get_result(task_id)
        # Convert TaskResult dataclass to dict for callers expecting dict
        return {
            "task_id": task_id,
            "text": result.text,
            "attachments": result.attachments,
            "structured_output": result.structured_output,
        }
    finally:
        await client.close()


def extract_attachments(response: Dict[str, Any]) -> List[Dict[str, Any]]:
    """Convenience: extract attachments from a messages response dict."""
    attachments = []
    for msg in response.get("messages", []):
        if msg.get("type") == "assistant_message":
            am = msg.get("assistant_message", {})
            attachments.extend(am.get("attachments", []))
    return attachments
