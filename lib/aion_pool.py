#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Aion Labs key pool — ротация 5 ключей с трекингом дневного истощения.

Aion Free-тир: 15 RPM + 20k TPM + 20k ток/день на ключ.
При 429 "Daily token limit exceeded" ключ помечается истощённым на текущие UTC-сутки
и пропускается до сброса (следующий UTC-день). Невалидный ключ (401) тоже исключается.

Состояние истощения персистится на диск (env AION_POOL_STATE, по умолчанию
/tmp/aion_pool_exhausted.json), чтобы переживать перезапуск бота внутри суток.
При загрузке оставляются только записи за текущие UTC-сутки (старые сбрасываются).

Использование:
    from aion_pool import AionKeyPool
    pool = AionKeyPool()           # читает /root/.aion_keys.json
    key = pool.acquire()           # round-robin по живым ключам
    pool.mark_exhausted(key)       # при 429/401
"""
import json
import os
import threading
import time


class AionKeyPool:
    def __init__(self, path: str = None, keys: list = None, state_path: str = None):
        self.path = path or os.getenv("AION_KEYS_FILE", "/root/.aion_keys.json")
        self.state_path = state_path or os.getenv(
            "AION_POOL_STATE", "/tmp/aion_pool_exhausted.json"
        )
        self._lock = threading.Lock()
        self.keys = keys if keys is not None else self._load()
        self.exhausted: dict = self._load_state()  # key -> UTC-day-bucket
        self._cursor = 0

    def _load(self) -> list:
        try:
            with open(self.path, "r", encoding="utf-8") as f:
                data = json.load(f)
            if isinstance(data, list):
                return [k for k in data if isinstance(k, str) and k]
        except Exception:
            pass
        return []

    @staticmethod
    def _day_bucket(ts: float = None) -> float:
        ts = time.time() if ts is None else ts
        return ts - (ts % 86400)  # секунды с начала UTC-суток (epoch выровнен по UTC)

    def _load_state(self) -> dict:
        today = self._day_bucket()
        try:
            with open(self.state_path, "r", encoding="utf-8") as f:
                raw = json.load(f)
            if isinstance(raw, dict):
                # оставляем только записи за сегодняшние сутки
                return {k: d for k, d in raw.items() if d == today}
        except Exception:
            pass
        return {}

    def _save_state(self):
        try:
            with open(self.state_path, "w", encoding="utf-8") as f:
                json.dump(self.exhausted, f)
        except Exception:
            pass

    def available(self) -> list:
        today = self._day_bucket()
        return [k for k in self.keys if self.exhausted.get(k) != today]

    def acquire(self):
        """Возвращает следующий доступный ключ (round-robin) или None, если все истощены."""
        with self._lock:
            avail = self.available()
            if not avail:
                return None
            n = len(self.keys)
            for i in range(n):
                idx = (self._cursor + i) % n
                k = self.keys[idx]
                if k in avail:
                    self._cursor = (idx + 1) % n
                    return k
            return None

    def mark_exhausted(self, key: str):
        with self._lock:
            self.exhausted[key] = self._day_bucket()
            self._save_state()

    def status(self) -> dict:
        today = self._day_bucket()
        return {
            "total": len(self.keys),
            "available": len(self.available()),
            "exhausted_today": [k[-4:] for k, d in self.exhausted.items() if d == today],
        }
