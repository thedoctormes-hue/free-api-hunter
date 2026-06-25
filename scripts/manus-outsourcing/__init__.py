"""Manus Outsourcing — аутсорсинг задач через Manus API v2."""

__version__ = "1.0.0"

from manus_client import ManusClient, ManusError
from account_manager import AccountManager
from task_manager import TaskManager
from config import ManusConfig, ManusAccount

__all__ = [
    "ManusClient",
    "ManusError",
    "AccountManager",
    "TaskManager",
    "ManusConfig",
    "ManusAccount",
]
