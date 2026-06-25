"""Конфигурация pytest для async тестов."""

import asyncio
import pytest


@pytest.fixture(scope="session")
def event_loop():
    """Создать event loop для всех тестов."""
    loop = asyncio.new_event_loop()
    yield loop
    loop.close()


def pytest_configure(config):
    config.addinivalue_line("markers", "asyncio: mark test as async")
