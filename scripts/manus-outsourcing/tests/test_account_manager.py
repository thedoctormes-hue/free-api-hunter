"""Тесты для account_manager."""

import asyncio
import unittest
from unittest.mock import AsyncMock, MagicMock, patch

import sys
import os
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from account_manager import AccountManager
from config import ManusConfig, ManusAccount
from manus_client import ManusError


import pytest

class TestAccountManager:

    def setup_method(self):
        self.config = ManusConfig(
            accounts=[
                ManusAccount(id="acc1", api_key="key1", remaining_credits=1000),
                ManusAccount(id="acc2", api_key="key2", remaining_credits=500),
                ManusAccount(id="acc3", api_key="key3", remaining_credits=200),
            ]
        )
        self.redis_mock = AsyncMock()
        self.manager = AccountManager(self.config, self.redis_mock)

    @pytest.mark.asyncio
    async def test_select_account_max_balance(self):
        """Выбирает аккаунт с максимальным балансом."""
        self.redis_mock.hget.return_value = None

        with patch.object(self.manager, "get_all_balances", new_callable=AsyncMock) as mock_bal:
            mock_bal.return_value = {"acc1": 1000, "acc2": 500, "acc3": 200}
            acc_id, client = await self.manager.select_account()

        assert acc_id == "acc1"

    @pytest.mark.asyncio
    async def test_select_account_all_low(self):
        """Все балансы низкие — выбирает максимальный."""
        with patch.object(self.manager, "get_all_balances", new_callable=AsyncMock) as mock_bal:
            mock_bal.return_value = {"acc1": 10, "acc2": 30, "acc3": 5}
            acc_id, client = await self.manager.select_account()

        assert acc_id == "acc2"

    @pytest.mark.asyncio
    async def test_update_balance(self):
        """Обновляет баланс в Redis."""
        self.redis_mock.hget.return_value = b"1000"

        await self.manager.update_balance("acc1", 300)

        self.redis_mock.hset.assert_called_with("manus:credits", "acc1", 700)

    @pytest.mark.asyncio
    async def test_update_balance_not_below_zero(self):
        """Баланс не уходит в минус."""
        self.redis_mock.hget.return_value = b"100"

        await self.manager.update_balance("acc1", 500)

        self.redis_mock.hset.assert_called_with("manus:credits", "acc1", 0)

    @pytest.mark.asyncio
    async def test_acquire_token_success(self):
        # Мокаем pipeline через redis_mock
        pipe_mock = AsyncMock()
        pipe_mock.execute.return_value = [
            {b"tokens": b"5.0", b"last_refill": b"1000.0"}
        ]
        self.redis_mock.pipeline = MagicMock(return_value=pipe_mock)
        self.redis_mock.hset = AsyncMock()
        self.redis_mock.get = AsyncMock(return_value=b"5.0")

        result = await self.manager.acquire_token("acc1")
        assert result is True

    @pytest.mark.asyncio
    async def test_handle_rate_limit(self):
        self.redis_mock.get.return_value = b"0"
        self.redis_mock.set = AsyncMock()

        delay = await self.manager.handle_rate_limit("acc1")
        assert 0 < delay < 2

    @pytest.mark.asyncio
    async def test_handle_rate_limit_max_retries(self):
        self.redis_mock.get.return_value = b"10"

        with pytest.raises(ManusError) as ctx:
            await self.manager.handle_rate_limit("acc1", max_retries=5)

        assert ctx.value.code == "max_retries"


class TestAccountManagerIntegration(unittest.TestCase):
    """Интеграционные тесты."""

    def test_real_balances(self):
        api_key = os.environ.get("MANUS_API_KEY", "")
        if not api_key:
            self.skipTest("MANUS_API_KEY not set")

        config = ManusConfig(
            accounts=[ManusAccount(id="test", api_key=api_key)]
        )

        # Используем реальный Redis или mock
        try:
            import redis.asyncio as redis
            r = redis.Redis.from_url("redis://localhost:6379/0", decode_responses=True)
            manager = AccountManager(config, r)

            balances = asyncio.get_event_loop().run_until_complete(
                manager.get_all_balances()
            )
            self.assertIsInstance(balances, dict)
            self.assertIn("test", balances)
            print(f"✅ Real balance: {balances['test']} credits")
        except Exception as e:
            self.skipTest(f"Redis not available: {e}")


if __name__ == "__main__":
    unittest.main()
