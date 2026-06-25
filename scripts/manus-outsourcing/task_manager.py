"""Высокоуровневый менеджер задач Manus."""

import asyncio
import json
import logging
import os
import time
from datetime import datetime
from typing import Any, Dict, List, Optional

import redis.asyncio as redis

from account_manager import AccountManager
from config import ManusConfig
from manus_client import ManusClient, ManusError

logger = logging.getLogger(__name__)


class TaskManager:
    def __init__(self, config: ManusConfig, redis_client: redis.Redis):
        self.config = config
        self.redis = redis_client
        self.account_manager = AccountManager(config, redis_client)

    async def send_task(
        self,
        message: str,
        structured_output_schema: Optional[Dict] = None,
        agent_profile: Optional[str] = None,
        locale: Optional[str] = None,
        project_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Отправить задачу Manus. Выбирает аккаунт, rate limits, отправляет."""
        # Выбрать аккаунт
        account_id, client = await self.account_manager.select_account()

        # Ждать rate limit token
        await self.account_manager.wait_for_token(account_id)

        # Отправить задачу
        try:
            result = await client.create_task(
                message=message,
                structured_output_schema=structured_output_schema,
                agent_profile=agent_profile or self.config.default_agent_profile,
                locale=locale or self.config.default_locale,
                project_id=project_id,
            )
            self.account_manager.reset_backoff(account_id)

            return {
                "task_id": result["task_id"],
                "task_url": result.get("task_url", ""),
                "task_title": result.get("task_title", ""),
                "account_id": account_id,
                "status": "created",
            }

        except ManusError as e:
            if e.code == "rate_limited":
                delay = await self.account_manager.handle_rate_limit(account_id)
                await asyncio.sleep(delay)
                return await self.send_task(
                    message, structured_output_schema, agent_profile, locale, project_id
                )
            if e.code == "unauthorized":
                logger.warning(f"Account {account_id} unauthorized, trying next")
                return await self.send_task(
                    message, structured_output_schema, agent_profile, locale, project_id
                )
            raise

    async def get_result(
        self,
        task_id: str,
        account_id: str,
        wait: bool = True,
        timeout: int = 600,
        poll_interval: int = 5,
    ) -> Dict[str, Any]:
        """Получить результат задачи. Опционально ждать завершения."""
        client = self.account_manager.get_client(account_id)
        start_time = time.time()

        while True:
            try:
                response = await client.list_messages(task_id)
                status = client.extract_status(response)

                if status == "stopped":
                    return self._build_result(task_id, account_id, response)

                if not wait:
                    return {
                        "task_id": task_id,
                        "account_id": account_id,
                        "status": status or "unknown",
                        "finished": False,
                    }

                elapsed = time.time() - start_time
                if elapsed >= timeout:
                    return {
                        "task_id": task_id,
                        "account_id": account_id,
                        "status": "timeout",
                        "finished": False,
                        "error": f"Timeout after {timeout}s",
                    }

                logger.debug(f"Task {task_id}: {status}, waiting {poll_interval}s...")
                await asyncio.sleep(poll_interval)

            except ManusError as e:
                if e.code == "rate_limited":
                    delay = await self.account_manager.handle_rate_limit(account_id)
                    await asyncio.sleep(delay)
                    continue
                raise

    def _build_result(
        self, task_id: str, account_id: str, response: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Собрать результат из ответа API."""
        attachments = client.extract_attachments(response) if 'client' in dir() else []
        # Извлечь attachments из messages
        for msg in response.get("messages", []):
            if msg.get("type") == "assistant_message":
                am = msg.get("assistant_message", {})
                attachments.extend(am.get("attachments", []))

        structured_output = None
        for msg in response.get("messages", []):
            if msg.get("type") == "assistant_message":
                am = msg.get("assistant_message", {})
                if "structured_output_result" in am:
                    structured_output = am["structured_output_result"]

        # Извлечь текстовый ответ
        content = ""
        for msg in reversed(response.get("messages", [])):
            if msg.get("type") == "assistant_message":
                am = msg.get("assistant_message", {})
                if am.get("content") and len(am["content"]) > 100:
                    content = am["content"]
                    break

        return {
            "task_id": task_id,
            "account_id": account_id,
            "status": "finished",
            "finished": True,
            "content": content,
            "attachments": attachments,
            "structured_output": structured_output,
        }

    async def send_followup(
        self, task_id: str, account_id: str, message: str
    ) -> Dict[str, Any]:
        """Отправить follow-up сообщение в существующую задачу."""
        client = self.account_manager.get_client(account_id)
        await self.account_manager.wait_for_token(account_id)

        try:
            result = await client.send_message(task_id, message)
            return {"ok": True, "request_id": result.get("request_id", "")}
        except ManusError as e:
            if e.code == "rate_limited":
                delay = await self.account_manager.handle_rate_limit(account_id)
                await asyncio.sleep(delay)
                return await self.send_followup(task_id, account_id, message)
            raise

    async def download_attachments(
        self,
        task_id: str,
        account_id: str,
        output_dir: Optional[str] = None,
    ) -> List[Dict[str, str]]:
        """Скачать все вложения из результата задачи."""
        client = self.account_manager.get_client(account_id)
        output_dir = output_dir or self.config.output_dir
        os.makedirs(output_dir, exist_ok=True)

        response = await client.list_messages(task_id)
        downloaded = []

        for msg in response.get("messages", []):
            if msg.get("type") == "assistant_message":
                am = msg.get("assistant_message", {})
                for att in am.get("attachments", []):
                    url = att.get("url", "")
                    filename = att.get("filename", f"file_{len(downloaded)}")

                    if url:
                        try:
                            content = await client.download_file(url)
                            local_path = os.path.join(output_dir, filename)
                            with open(local_path, "wb") as f:
                                f.write(content)
                            downloaded.append({
                                "filename": filename,
                                "local_path": local_path,
                                "size_bytes": len(content),
                                "source_url": url,
                            })
                            logger.info(f"Downloaded: {filename} ({len(content)} bytes)")
                        except Exception as e:
                            logger.warning(f"Failed to download {filename}: {e}")

        return downloaded

    async def get_credits(self, account_id: Optional[str] = None) -> Dict[str, Any]:
        """Получить баланс кредитов."""
        if account_id:
            balance = await self.account_manager.get_balance(account_id)
            return {"account_id": account_id, "balance": balance}
        return await self.account_manager.get_all_balances()
