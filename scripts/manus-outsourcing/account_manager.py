"""Управление аккаунтами Manus: балансы, маршрутизация, rate limiting."""

import json
import logging
import os
import time
from typing import Dict, List, Optional, Tuple

import redis.asyncio as redis

from config import ManusAccount, ManusConfig
from manus_client import ManusClient, ManusError

logger = logging.getLogger(__name__)

REDIS_KEY_BALANCES = "manus:credits"
REDIS_KEY_LAST_USED = "manus:last_used"
REDIS_KEY_TOKEN_BUCKET = "manus:tokens"
REDIS_KEY_BACKOFF = "manus:backoff"


class AccountManager:
    def __init__(self, config: ManusConfig, redis_client: redis.Redis):
        self.config = config
        self.redis = redis_client
        self.clients: Dict[str, ManusClient] = {}
        self._init_clients()

    def _init_clients(self):
        for acc in self.config.accounts:
            self.clients[acc.id] = ManusClient(api_key=acc.api_key, base_url=self.config.base_url)

    def get_client(self, account_id: str) -> ManusClient:
        if account_id not in self.clients:
            raise ValueError(f"Unknown account: {account_id}")
        return self.clients[account_id]

    async def refresh_balances(self):
        """Обновить балансы всех аккаунтов через API."""
        for acc_id, client in self.clients.items():
            try:
                result = await client.get_credits()
                if result.get("ok"):
                    credits = result.get("total_credits", 0)
                    await self.redis.hset(REDIS_KEY_BALANCES, acc_id, credits)
                    logger.info(f"Account {acc_id}: {credits} credits")
            except ManusError as e:
                logger.warning(f"Failed to get balance for {acc_id}: {e}")

    async def get_balance(self, account_id: str) -> int:
        """Получить баланс аккаунта (из Redis или API)."""
        cached = await self.redis.hget(REDIS_KEY_BALANCES, account_id)
        if cached is not None:
            return int(cached)

        # Fallback: запросить из API
        client = self.get_client(account_id)
        result = await client.get_credits()
        return result.get("total_credits", 0)

    async def get_all_balances(self) -> Dict[str, int]:
        """Получить балансы всех аккаунтов."""
        balances = {}
        for acc_id in self.clients:
            balances[acc_id] = await self.get_balance(acc_id)
        return balances

    async def select_account(self) -> Tuple[str, ManusClient]:
        """Выбрать аккаунт с наибольшим балансом (least-used)."""
        balances = await self.get_all_balances()

        if not balances:
            raise RuntimeError("No accounts available")

        # Выбрать аккаунт с максимальным балансом
        best_id = max(balances, key=balances.get)
        best_balance = balances[best_id]

        if best_balance < 50:
            logger.warning(f"Lowest balance: {best_id}={best_balance}. All accounts low.")

        await self.redis.hset(REDIS_KEY_LAST_USED, best_id, str(time.time()))
        logger.info(f"Selected account: {best_id} ({best_balance} credits)")

        return best_id, self.get_client(best_id)

    async def update_balance(self, account_id: str, credits_used: int):
        """Обновить баланс после выполнения задачи."""
        current = await self.get_balance(account_id)
        new_balance = max(0, current - credits_used)
        await self.redis.hset(REDIS_KEY_BALANCES, account_id, new_balance)
        logger.info(f"Updated {account_id}: {current} → {new_balance} (used {credits_used})")

    # === Rate Limiting (Token Bucket) ===

    async def acquire_token(self, account_id: str, max_rpm: int = 10) -> bool:
        """Получить токен для запроса. Вернёт True если можно делать запрос."""
        key = f"{REDIS_KEY_TOKEN_BUCKET}:{account_id}"
        now = time.time()

        pipe = self.redis.pipeline()

        # Получить текущее состояние
        pipe.hgetall(key)
        results = await pipe.execute()

        state = results[0] if results else {}
        tokens = float(state.get(b"tokens", max_rpm))
        last_refill = float(state.get(b"last_refill", now))

        # Пополнить токены
        elapsed = now - last_refill
        tokens_to_add = elapsed * (max_rpm / 60.0)
        tokens = min(max_rpm, tokens + tokens_to_add)

        if tokens >= 1:
            tokens -= 1
            await self.redis.hset(key, mapping={
                "tokens": str(tokens),
                "last_refill": str(now),
            })
            return True

        # Токенов нет — рассчитать время ожидания
        wait_time = (1 - tokens) / (max_rpm / 60.0)
        logger.warning(f"Rate limit for {account_id}: wait {wait_time:.1f}s")
        return False

    async def wait_for_token(self, account_id: str, max_rpm: int = 10, timeout: float = 30.0):
        """Ждать пока появится токен."""
        start = time.time()
        while time.time() - start < timeout:
            if await self.acquire_token(account_id, max_rpm):
                return True
            await asyncio.sleep(0.5)
        raise TimeoutError(f"Rate limit timeout for {account_id}")

    # === Exponential Backoff ===

    async def handle_rate_limit(self, account_id: str, max_retries: int = 5) -> float:
        """Обработать 429 ошибку. Вернуть задержку перед повтором."""
        key = f"{REDIS_KEY_BACKOFF}:{account_id}"

        retries_bytes = await self.redis.get(key)
        retries = int(retries_bytes) if retries_bytes else 0

        if retries >= max_retries:
            raise ManusError("max_retries", f"Max retries ({max_retries}) exceeded for {account_id}")

        retries += 1
        await self.redis.set(key, str(retries), ex=60)

        import random
        delay = (2 ** (retries - 1)) + random.uniform(0, 0.5)
        logger.warning(f"429 for {account_id}: retry {retries}/{max_retries} in {delay:.1f}s")
        return delay

    def reset_backoff(self, account_id: str):
        """Сбросить backoff после успешного запроса."""
        key = f"{REDIS_KEY_BACKOFF}:{account_id}"
        self.redis.delete(key)
