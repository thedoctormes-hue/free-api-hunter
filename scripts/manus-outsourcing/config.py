"""Конфигурация скилла manus-outsourcing."""

import json
import os
from dataclasses import dataclass, field
from typing import List, Optional


@dataclass
class ManusAccount:
    id: str
    api_key: str
    credit_limit_per_day: int = 300
    remaining_credits: int = 300
    last_used: Optional[str] = None


@dataclass
class ManusConfig:
    base_url: str = "https://api.manus.ai/v2/"
    accounts: List[ManusAccount] = field(default_factory=list)
    redis_url: str = "redis://localhost:6379/0"
    polling_interval_sec: int = 5
    polling_timeout_sec: int = 600
    webhook_secret: str = ""
    default_agent_profile: str = "manus-1.6"
    default_locale: str = "ru"
    output_dir: str = "/root/LabDoctorM/projects/free-api-hunter/output"
    yandex_disk_enabled: bool = True
    yandex_disk_webdav: str = "https://webdav.yandex.ru"
    yandex_disk_user: str = "moscowskiymichi@yandex.ru"
    yandex_disk_pass_file: str = "~/.config/yandex/.disk-pass"
    yandex_disk_colony_path: str = "/colony/shared"

    @classmethod
    def from_file(cls, path: str) -> "ManusConfig":
        with open(path, "r") as f:
            data = json.load(f)

        accounts = []
        for acc in data.get("accounts", []):
            accounts.append(ManusAccount(
                id=acc["id"],
                api_key=acc["key"],
                credit_limit_per_day=acc.get("credit_limit", 300),
                remaining_credits=acc.get("balance", 300),
            ))

        return cls(
            base_url=data.get("base_url", cls.base_url),
            accounts=accounts,
            redis_url=data.get("redis_url", cls.redis_url),
            polling_interval_sec=data.get("polling_interval_sec", cls.polling_interval_sec),
            polling_timeout_sec=data.get("polling_timeout_sec", cls.polling_timeout_sec),
            webhook_secret=data.get("webhook_secret", ""),
            default_agent_profile=data.get("default_agent_profile", cls.default_agent_profile),
            default_locale=data.get("default_locale", cls.default_locale),
            output_dir=data.get("output_dir", cls.output_dir),
            yandex_disk_enabled=data.get("yandex_disk_enabled", True),
            yandex_disk_webdav=data.get("yandex_disk_webdav", cls.yandex_disk_webdav),
            yandex_disk_user=data.get("yandex_disk_user", cls.yandex_disk_user),
            yandex_disk_pass_file=data.get("yandex_disk_pass_file", cls.yandex_disk_pass_file),
            yandex_disk_colony_path=data.get("yandex_disk_colony_path", cls.yandex_disk_colony_path),
        )

    def get_yandex_disk_pass(self) -> str:
        pass_path = os.path.expanduser(self.yandex_disk_pass_file)
        with open(pass_path, "r") as f:
            return f.read().strip()
