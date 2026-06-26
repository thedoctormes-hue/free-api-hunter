"""Manus Outsourcing — аутсорсинг задач через Manus API v2."""

__version__ = "2.0.0"

from manus_client import ManusClient, ManusError, get_result, extract_attachments
from manus_client import TaskResult, TaskStatus, ManusTaskError, ManusTimeoutError
from account_manager import AccountManager
from task_manager import TaskManager
from config import ManusConfig, ManusAccount

__all__ = [
    "ManusClient",
    "ManusError",
    "get_result",
    "extract_attachments",
    "TaskResult",
    "TaskStatus",
    "ManusTaskError",
    "ManusTimeoutError",
    "AccountManager",
    "TaskManager",
    "ManusConfig",
    "ManusAccount",
]
