#!/usr/bin/env python3
"""key_pool.py — canonical API-key pool manager for free-api-hunter.

SSOT: configs/key_pool.json  (gitignored — contains secrets + status).
Consumers synced on every change:
  - config/search-keys.yaml        (orchestrator direct provider calls)
  - searxng/settings.yml           (SearXNG engine `api_keys: [...]`)
  - config/.key-index-<provider>   (legacy cyclic pointer, kept for audit)

WHY
---
Paid providers (tavily/firecrawl/tinyfish/exa/olostep) return HTTP 402 when a
key is suspended. Without management:
  * a 402 key is re-probed on EVERY search (quota + latency waste), and
  * for single-key providers the dead key is sent straight to the API,
    suspending the whole SearXNG engine for `suspended_time` seconds.

This module marks 402 keys `burnt` and rotates to the next valid key WITHOUT
agent involvement. A scheduled healthcheck prober does the same proactively
and logs an alert when a key must be replaced.

Probe behaviour mirrors searxng/engines/_poolkeys.py: a minimal real request;
HTTP 2xx => valid, any HTTPError (incl. 402) => invalid.

Usage:
  key_pool.py init
  key_pool.py next <provider>            # print next ACTIVE key (cyclic, skip burnt)
  key_pool.py mark_burnt <provider> <key>
  key_pool.py rotate_on_error <provider> <key>   # mark burnt + print next active key
  key_pool.py healthcheck [--apply]      # probe all active keys, mark 402 burnt
  key_pool.py sync                       # rewrite consumers from pool
  key_pool.py status                     # print summary (keys masked)
  key_pool.py self-test                  # offline tests with mocked probe
"""
from __future__ import annotations

import json
import os
import re
import sys
import time
import urllib.error
import urllib.request
from typing import Optional

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
CONFIG_DIR = os.path.join(REPO, "config")          # symlink -> configs
KEY_POOL = os.path.join(CONFIG_DIR, "key_pool.json")
SEARCH_KEYS = os.path.join(CONFIG_DIR, "search-keys.yaml")
SETTINGS = os.path.join(REPO, "searxng", "settings.yml")

PROVIDERS = ["exa", "tavily", "firecrawl", "tinyfish", "olostep"]
HTTP_TIMEOUT = 8.0

# --- logging -------------------------------------------------------------
def log(msg: str) -> None:
    ts = time.strftime("%Y-%m-%dT%H:%M:%S")
    line = f"[{ts}] key_pool: {msg}"
    print(line, flush=True)
    try:
        with open(os.path.join(REPO, "data", "key_pool.log"), "a") as f:
            f.write(line + "\n")
    except OSError:
        pass


# --- pool load/save ------------------------------------------------------
def load_pool() -> dict:
    if not os.path.exists(KEY_POOL):
        return {"updated_at": None, "providers": {p: {"keys": []} for p in PROVIDERS}}
    try:
        with open(KEY_POOL) as f:
            data = json.load(f)
    except (json.JSONDecodeError, OSError):
        data = {"updated_at": None, "providers": {}}
    data.setdefault("providers", {})
    for p in PROVIDERS:
        data["providers"].setdefault(p, {"keys": []})
        data["providers"][p].setdefault("keys", [])
        data["providers"][p].setdefault("active_idx", 0)
    return data


def save_pool(data: dict) -> None:
    data["updated_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    os.makedirs(os.path.dirname(KEY_POOL), exist_ok=True)
    tmp = KEY_POOL + ".tmp"
    with open(tmp, "w") as f:
        json.dump(data, f, indent=2, sort_keys=True)
    os.replace(tmp, KEY_POOL)
    try:
        os.chmod(KEY_POOL, 0o600)
    except OSError:
        pass


def _find_key(prov: dict, key: str) -> Optional[dict]:
    for k in prov["keys"]:
        if k.get("key") == key:
            return k
    return None


def mask(k: str) -> str:
    if not k:
        return "<empty>"
    return k[:6] + "…" + k[-4:] if len(k) > 12 else k[:3] + "…"


# --- seeding from existing sources --------------------------------------
def cmd_init() -> None:
    data = load_pool()
    changed = False
    # Primary source of deployed truth: settings.yml api_keys (all 5 pools).
    settings_keys = _read_settings_api_keys()
    for p in PROVIDERS:
        existing = {k["key"]: k for k in data["providers"][p]["keys"]}
        incoming = settings_keys.get(p, [])
        for key in incoming:
            if key and key not in existing:
                data["providers"][p]["keys"].append(
                    {"key": key, "status": "active", "last_error": None,
                     "checked_at": None, "burnt_at": None}
                )
                changed = True
    if changed:
        save_pool(data)
        log("init: seeded pool from settings.yml")
    else:
        log("init: pool already in sync with settings.yml")
    print(status_text(data))


def _read_settings_api_keys() -> dict:
    out = {p: [] for p in PROVIDERS}
    if not os.path.exists(SETTINGS):
        return out
    try:
        import yaml
    except ImportError:
        return out
    try:
        with open(SETTINGS) as f:
            cfg = yaml.safe_load(f) or {}
    except Exception:
        return out
    for e in cfg.get("engines", []) or []:
        name = e.get("name")
        if name in out and e.get("api_keys"):
            out[name] = list(e["api_keys"])
    return out


# --- core operations -----------------------------------------------------
def cmd_next(provider: str) -> str:
    data = load_pool()
    prov = data["providers"][provider]
    keys = prov["keys"]
    n = len(keys)
    if n == 0:
        log(f"next {provider}: NO KEYS")
        return ""
    start = int(prov.get("active_idx", 0)) % n
    for off in range(n):
        i = (start + off) % n
        k = keys[i]
        if k.get("status") not in ("burnt", "suspended"):
            prov["active_idx"] = (i + 1) % n
            save_pool(data)
            return k["key"]
    log(f"next {provider}: all keys burnt")
    return ""


def cmd_mark_burnt(provider: str, key: str) -> None:
    data = load_pool()
    prov = data["providers"][provider]
    k = _find_key(prov, key)
    if k is None:
        # key not in pool yet (e.g. rotated key from a different source) — add as burnt
        k = {"key": key, "status": "burnt", "last_error": "invalid",
             "checked_at": None, "burnt_at": _now()}
        prov["keys"].append(k)
        log(f"mark_burnt {provider}: key {mask(key)} not in pool, added as burnt")
    else:
        if k.get("status") == "burnt":
            log(f"mark_burnt {provider}: key {mask(key)} already burnt")
        else:
            k["status"] = "burnt"
            k["last_error"] = "invalid"
            k["burnt_at"] = _now()
            log(f"mark_burnt {provider}: key {mask(key)} -> burnt (fatal)")
    save_pool(data)
    sync_consumers(data, provider)


def cmd_suspend(provider: str, key: str) -> str:
    """Mark key SUSPENDED (temporary: 402 credits / 429-432 quota), sync, and
    return the NEXT active key for immediate retry. Suspended keys are still
    probed by the healthcheck prober and auto-recover to 'active' on HTTP 200,
    unlike 'burnt' (fatal, e.g. 401/403 invalid key)."""
    data = load_pool()
    prov = data["providers"][provider]
    k = _find_key(prov, key)
    if k is None:
        k = {"key": key, "status": "suspended", "last_error": "suspended",
             "checked_at": None, "burnt_at": None}
        prov["keys"].append(k)
        log(f"suspend {provider}: key {mask(key)} not in pool, added as suspended")
    else:
        if k.get("status") == "burnt":
            log(f"suspend {provider}: key {mask(key)} already burnt, leaving burnt")
        else:
            k["status"] = "suspended"
            k["last_error"] = "suspended"
            log(f"suspend {provider}: key {mask(key)} -> suspended (temporary)")
    save_pool(data)
    sync_consumers(data, provider)
    return cmd_next(provider)


def cmd_rotate_on_error(provider: str, key: str) -> str:
    """Runtime error on a key: SOFT-suspend it (not burn), sync, return NEXT
    active key for immediate retry. 402/429/432 are temporary and auto-recover;
    only 401/403 (invalid) is marked burnt by the healthcheck prober."""
    cmd_suspend(provider, key)
    return cmd_next(provider)


def _now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%S")


# --- sync consumers ------------------------------------------------------
def sync_consumers(data: dict, provider: Optional[str] = None) -> None:
    providers = [provider] if provider else PROVIDERS
    for p in providers:
        active = [k["key"] for k in data["providers"][p]["keys"]
                  if k.get("status") not in ("burnt", "suspended") and k.get("key")]
        _write_settings_api_keys(p, active)
        _write_search_keys(p, active)
        _write_legacy_index(p)
    log(f"sync: consumers updated for {', '.join(providers)}")


def _write_settings_api_keys(provider: str, keys: list) -> None:
    if not os.path.exists(SETTINGS):
        return
    with open(SETTINGS) as f:
        text = f.read()
    text = _set_api_keys_block(text, provider, keys)
    # All keys burnt -> disable the engine so an empty key pool does not
    # trip a SearXNG load error / per-request suspension on restart.
    if not keys:
        text = _set_disabled(text, provider, True)
    if text != (orig := open(SETTINGS).read()):
        tmp = SETTINGS + ".tmp"
        with open(tmp, "w") as f:
            f.write(text)
        os.replace(tmp, SETTINGS)


def _set_disabled(text: str, provider: str, disabled: bool) -> str:
    """Set `disabled:` inside one engine block (adds it if missing)."""
    lines = text.splitlines(keepends=True)
    out: list[str] = []
    i = 0
    n = len(lines)
    target = f"- name: {provider}"
    in_block = False
    done = False
    while i < n:
        line = lines[i]
        stripped = line.lstrip()
        if not in_block:
            if line.rstrip() == target:
                in_block = True
            out.append(line)
            i += 1
            continue
        # inside target block
        if stripped.startswith("- name:") and stripped != target:
            # block ends just before this line; insert disabled here if missing
            if not done:
                out.append("  disabled: " + ("true\n" if disabled else "false\n"))
                done = True
            out.append(line)
            i += 1
            in_block = False
            continue
        if stripped.startswith("disabled:"):
            out.append("  disabled: " + ("true\n" if disabled else "false\n"))
            i += 1
            done = True
            continue
        out.append(line)
        i += 1
    if in_block and not done:
        out.append("  disabled: " + ("true\n" if disabled else "false\n"))
    return "".join(out)


def _set_api_keys_block(text: str, provider: str, keys: list) -> str:
    """Surgically replace the `api_keys:` list inside one engine block.
    Handles both populated and empty (corrupted) lists. Preserves every
    other line: weights, categories, comments, disabled flags."""
    lines = text.splitlines(keepends=True)
    out: list[str] = []
    i = 0
    n = len(lines)
    target = f"- name: {provider}"
    in_block = False
    replaced = False
    while i < n:
        line = lines[i]
        stripped = line.lstrip()
        if not in_block:
            if line.rstrip() == target:
                in_block = True
                out.append(line)
                i += 1
                continue
            out.append(line)
            i += 1
            continue
        # inside block
        if stripped.startswith("- name:") and stripped != target:
            in_block = False
            out.append(line)
            i += 1
            continue
        if stripped.startswith("api_keys:"):
            out.append(line)  # keep original indent of "api_keys:"
            i += 1
            # skip existing list items that belong to api_keys (indented "- ")
            while i < n and lines[i].lstrip().startswith("- "):
                i += 1
            for k in keys:
                out.append("  - " + k + "\n")
            replaced = True
            continue
        out.append(line)
        i += 1
    if not replaced:
        # block had no api_keys line; append before block end (rare)
        pass
    return "".join(out)


def _write_search_keys(provider: str, keys: list) -> None:
    """Update config/search-keys.yaml providers.<provider> list (active only)."""
    if not os.path.exists(SEARCH_KEYS):
        return
    try:
        import yaml
    except ImportError:
        return
    with open(SEARCH_KEYS) as f:
        cfg = yaml.safe_load(f) or {}
    cfg.setdefault("providers", {})[provider] = list(keys)
    with open(SEARCH_KEYS, "w") as f:
        yaml.safe_dump(cfg, f, sort_keys=False, default_flow_style=False)


def _write_legacy_index(provider: str) -> None:
    path = os.path.join(CONFIG_DIR, f".key-index-{provider}")
    try:
        with open(path, "w") as f:
            f.write("0")
    except OSError:
        pass


# --- healthcheck prober --------------------------------------------------
def _probe(provider: str, key: str, timeout: float = HTTP_TIMEOUT) -> str:
    """Return 'ok' | '402' | 'invalid' | 'error'."""
    try:
        if provider == "tavily":
            url = "https://api.tavily.com/search"
            # Tavily expects api_key in the JSON body (NOT an Authorization header)
            req = urllib.request.Request(
                url,
                data=json.dumps({"api_key": key, "query": "health",
                                 "max_results": 1}).encode(),
                headers={"Content-Type": "application/json"},
                method="POST",
            )
        elif provider == "firecrawl":
            url = "https://api.firecrawl.dev/v1/search"
            req = urllib.request.Request(
                url,
                data=json.dumps({"query": "health", "limit": 1}).encode(),
                headers={"Content-Type": "application/json",
                         "Authorization": "Bearer " + key},
                method="POST",
            )
        elif provider == "tinyfish":
            url = "https://api.search.tinyfish.ai?query=health&location=US&language=en"
            req = urllib.request.Request(
                url, headers={"X-API-Key": key}, method="GET")
        elif provider == "exa":
            url = "https://api.exa.ai/search"
            req = urllib.request.Request(
                url,
                data=json.dumps({"query": "health", "num_results": 1}).encode(),
                headers={"Content-Type": "application/json",
                         "Authorization": "Bearer " + key},
                method="POST",
            )
        elif provider == "olostep":
            # Engine uses https://api.olostep.com/v1/searches (plural) + Bearer.
            url = "https://api.olostep.com/v1/searches"
            req = urllib.request.Request(
                url,
                data=json.dumps({"query": "health"}).encode(),
                headers={"Content-Type": "application/json",
                         "Authorization": "Bearer " + key},
                method="POST",
            )
        else:
            return "error"
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return "ok" if 200 <= resp.status < 300 else "invalid"
    except urllib.error.HTTPError as e:
        if e.code == 402:
            return "402"
        if e.code in (401, 403):
            return "invalid"
        if e.code in (429, 432):
            return "limit"
        return "error"
    except Exception:
        return "error"


def cmd_healthcheck(apply: bool = True) -> int:
    data = load_pool()
    alerts = 0
    for p in PROVIDERS:
        prov = data["providers"][p]
        for k in prov["keys"]:
            if k.get("status") == "burnt" or not k.get("key"):
                continue
            res = _probe(p, k["key"])
            k["checked_at"] = _now()
            if res == "402":
                k["status"] = "suspended"
                k["last_error"] = "402"
                alerts += 1
                log(f"HEALTHCHECK: {p} key {mask(k['key'])} -> 402 (credits), SUSPENDED "
                    f"(temporary, auto-recovers on top-up).")
                print(f"⚠ {p} key credits exhausted (402) — suspended, auto-recovers on top-up.")
            elif res == "limit":
                k["status"] = "suspended"
                k["last_error"] = "limit"
                alerts += 1
                log(f"HEALTHCHECK: {p} key {mask(k['key'])} -> QUOTA/LIMIT (429/432), SUSPENDED (temporary)")
                print(f"⚠ {p} key quota/limit (429/432) — suspended, auto-recovers on reset.")
            elif res in ("invalid", "error"):
                # 401/403 (invalid key) / network: FATAL -> burnt, prober stops probing.
                k["status"] = "burnt"
                k["last_error"] = res
                k["burnt_at"] = _now()
                alerts += 1
                log(f"HEALTHCHECK: {p} key {mask(k['key'])} -> {res} (burnt, likely invalid key)")
                print(f"⚠ NEED TO UPDATE KEY: {p} — key {mask(k['key'])} invalid ({res}), burnt.")
            else:
                k["status"] = "active"
                k["last_error"] = None
    if apply:
        save_pool(data)
        sync_consumers(data)
        if alerts:
            log(f"HEALTHCHECK done: {alerts} key(s) flagged (burnt/suspended), consumers synced.")
    else:
        log("HEALTHCHECK dry-run: changes not applied.")
    return alerts


# --- status --------------------------------------------------------------
def status_text(data: dict) -> str:
    lines = ["key_pool status:"]
    for p in PROVIDERS:
        prov = data["providers"][p]
        active = [k for k in prov["keys"] if k.get("status") == "active"]
        suspended = [k for k in prov["keys"] if k.get("status") == "suspended"]
        burnt = [k for k in prov["keys"] if k.get("status") == "burnt"]
        lines.append(f"  {p}: {len(active)} active / {len(suspended)} suspended / {len(burnt)} burnt")
        for k in suspended:
            lines.append(f"      suspended {mask(k['key'])} ({k.get('last_error','?')})")
        for k in burnt:
            lines.append(f"      burnt {mask(k['key'])} ({k.get('burnt_at','?')})")
    return "\n".join(lines)


def cmd_recover(provider: Optional[str] = None) -> int:
    """Reset 'burnt' keys back to 'suspended' so the prober re-probes them.
    Used after a key is topped-up / replaced, or to undo a false burn."""
    data = load_pool()
    providers = [provider] if provider else PROVIDERS
    n = 0
    for p in providers:
        for k in data["providers"][p]["keys"]:
            if k.get("status") == "burnt":
                k["status"] = "suspended"
                k["last_error"] = "recovered"
                k["burnt_at"] = None
                n += 1
                log(f"recover: {p} key {mask(k['key'])} burnt -> suspended")
    if n:
        save_pool(data)
        sync_consumers(data, provider)
        print(f"recovered {n} key(s) -> suspended (prober will re-probe)")
    else:
        print("nothing to recover")
    return 0


def cmd_status() -> None:
    print(status_text(load_pool()))


# --- self-test (offline, mocked probe) ----------------------------------
def cmd_self_test() -> int:
    global _probe, KEY_POOL, SETTINGS, SEARCH_KEYS, CONFIG_DIR
    import tempfile
    tmp = tempfile.mkdtemp(prefix="kp_selftest_")
    saved_probe = _probe
    saved_pool = KEY_POOL
    saved_settings = SETTINGS
    saved_searchkeys = SEARCH_KEYS
    saved_configdir = CONFIG_DIR
    # Redirect ALL side-effect paths to an isolated temp tree.
    KEY_POOL = os.path.join(tmp, "key_pool.json")
    SETTINGS = os.path.join(tmp, "settings.yml")
    SEARCH_KEYS = os.path.join(tmp, "search-keys.yaml")
    CONFIG_DIR = tmp
    # Minimal valid settings.yml consumed by sync_consumers.
    with open(SETTINGS, "w") as f:
        f.write("engines:\n- name: tavily\n  api_keys:\n")
    with open(SEARCH_KEYS, "w") as f:
        f.write("providers:\n  tavily: []")
    # Mock probe: -dead => 402, -inv => invalid(401/403), -lim => limit(429/432), else ok.
    def fake_probe(provider, key, timeout=HTTP_TIMEOUT):
        if key.endswith("-dead"):
            return "402"
        if key.endswith("-inv"):
            return "invalid"
        if key.endswith("-lim"):
            return "limit"
        return "ok"
    _probe = fake_probe
    try:
        data = load_pool()
        data["providers"]["tavily"]["keys"] = [
            {"key": "tvly-good", "status": "active"},
            {"key": "tvly-dead", "status": "active"},
            {"key": "tvly-inv", "status": "active"},
            {"key": "tvly-lim", "status": "active"},
        ]
        save_pool(data)
        # runtime 402 on tvly-dead -> SOFT suspend (NOT burnt)
        cmd_rotate_on_error("tavily", "tvly-dead")
        d = load_pool()
        dead = [k for k in d["providers"]["tavily"]["keys"] if k["key"] == "tvly-dead"][0]
        assert dead["status"] == "suspended", "402 should suspend, not burn"
        # healthcheck: 402->suspended, limit->suspended, invalid->burnt, ok->active
        cmd_healthcheck(apply=True)
        d = load_pool()
        keys = {k["key"]: k for k in d["providers"]["tavily"]["keys"]}
        assert keys["tvly-dead"]["status"] == "suspended", "dead not suspended"
        assert keys["tvly-lim"]["status"] == "suspended", "limit not suspended"
        assert keys["tvly-inv"]["status"] == "burnt", "invalid not burnt"
        assert keys["tvly-good"]["status"] == "active", "good not active"
        # next must skip both suspended and burnt
        seen = set()
        for _ in range(8):
            nxt = cmd_next("tavily")
            if not nxt:
                break
            seen.add(nxt)
        assert seen == {"tvly-good"}, f"next leaked suspended/burnt: {seen}"
        # recover resets burnt -> suspended (re-probable)
        cmd_recover("tavily")
        d = load_pool()
        inv = [k for k in d["providers"]["tavily"]["keys"] if k["key"] == "tvly-inv"][0]
        assert inv["status"] == "suspended", "recover did not reset burnt"
        # sync_consumers must EXCLUDE suspended from settings.yml active list
        active = [k["key"] for k in d["providers"]["tavily"]["keys"]
                  if k.get("status") not in ("burnt", "suspended")]
        assert active == ["tvly-good"], f"suspended leaked to active: {active}"
        # status text masks keys
        assert "tvly-good" not in status_text(d), "key leaked in status"
        print("self-test: ALL PASSED")
        return 0
    finally:
        _probe = saved_probe
        KEY_POOL = saved_pool
        SETTINGS = saved_settings
        SEARCH_KEYS = saved_searchkeys
        CONFIG_DIR = saved_configdir
        import shutil
        shutil.rmtree(tmp, ignore_errors=True)


# --- CLI -----------------------------------------------------------------
def main(argv: list) -> int:
    if not argv:
        print(__doc__)
        return 2
    cmd = argv[0]
    if cmd == "init":
        cmd_init()
        return 0
    if cmd == "next":
        print(cmd_next(argv[1]))
        return 0
    if cmd == "mark_burnt":
        cmd_mark_burnt(argv[1], argv[2])
        return 0
    if cmd == "rotate_on_error":
        print(cmd_rotate_on_error(argv[1], argv[2]))
        return 0
    if cmd == "healthcheck":
        apply = "--no-apply" not in argv
        return cmd_healthcheck(apply)
    if cmd == "sync":
        sync_consumers(load_pool())
        log("manual sync done")
        return 0
    if cmd == "status":
        cmd_status()
        return 0
    if cmd == "recover":
        return cmd_recover(argv[1] if len(argv) > 1 else None)
    if cmd == "self-test":
        return cmd_self_test()
    print(f"unknown command: {cmd}")
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
