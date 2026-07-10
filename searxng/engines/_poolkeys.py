# SPDX-License-Identifier: AGPL-3.0-or-later
"""Shared API-key pool with live validation (failover) for paid SearXNG engines.

WHY THIS EXISTS
----------------
SearXNG suspends the *entire* engine when a request raises (e.g. a dead or
rate-limited API key -> HTTP 401/429). Plain round-robin does NOT prevent that:
the request carrying the bad key still fails and suspends the engine for that
call (and briefly afterwards, by `suspend_time`).

This pool validates each key with a *minimal* real API call BEFORE handing a
working key to the framework, so the framework's own request always uses a
valid key -> zero key-induced outages.

COST
-----
One extra (minimal) API call per search -> roughly 2x quota usage.
Mitigated by result caching at the orchestrator / MCP layer.
"""

import json
import threading
import urllib.error
import urllib.request

from searx.exceptions import SearxEngineAPIException


class KeyPool:
    """Pick the next key that passes ``validate(key, timeout) -> bool``."""

    def __init__(self, keys, validate):
        self.keys = list(keys) if keys else []
        self.validate = validate
        self.idx = 0
        self.lock = threading.Lock()

    def pick(self, timeout: float = 5.0) -> str:
        n = len(self.keys)
        if n == 0:
            raise SearxEngineAPIException("No API keys configured for engine")
        # Single key: nothing to fail over to, so skip the probe entirely.
        # (Probing a slow 1-key API only doubles latency and can trip the
        #  framework request timeout.)
        if n == 1:
            return self.keys[0]
        with self.lock:
            start = self.idx
        last_err = None
        for off in range(n):
            i = (start + off) % n
            key = self.keys[i]
            try:
                if self.validate(key, timeout):
                    with self.lock:
                        self.idx = (i + 1) % n
                    return key
            except Exception as exc:  # noqa: BLE001 - any failure => key unusable
                last_err = exc
                continue
        raise SearxEngineAPIException(
            f"All {n} API keys failed validation (last error: {last_err!r})"
        )


def probe_post_json(url: str, headers: dict, body: dict, timeout: float = 8.0) -> bool:
    """Minimal POST probe. True iff the API accepted the key (HTTP 2xx)."""
    req = urllib.request.Request(
        url,
        data=json.dumps(body).encode(),
        headers={**headers, "Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return 200 <= resp.status < 300
    except urllib.error.HTTPError:
        return False
    except Exception:
        return False


def probe_get(url: str, headers: dict, timeout: float = 8.0) -> bool:
    """Minimal GET probe. True iff the API accepted the key (HTTP 2xx)."""
    req = urllib.request.Request(url, headers=headers, method="GET")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return 200 <= resp.status < 300
    except urllib.error.HTTPError:
        return False
    except Exception:
        return False
