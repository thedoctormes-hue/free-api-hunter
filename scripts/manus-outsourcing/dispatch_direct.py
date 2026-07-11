#!/usr/bin/env python3
"""
dispatch_direct.py — тонкий диспатч задачи в Manus API напрямую (без локального
TaskManager/redis/воркера). Используется KRV-пайплайном (hunter validate-pending)
как «агентный LLM-слой»: получает промпт, создаёт задачу в Manus, ждёт
завершения и печатает текстовый результат в stdout.

Аргументы:
  $1 — промпт (текст задачи)
  $2 — путь к configs/manus-keys.json (опц.)

Ключи берутся из configs/manus-keys.json (все аккаунты) и ротируются, чтобы
обходить лимит task.create (10/мин на аккаунт). Сырой ключ НЕ передаётся в промпт.
"""
import asyncio
import json
import os
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
if SCRIPT_DIR not in sys.path:
    sys.path.insert(0, SCRIPT_DIR)

from manus_client import (  # noqa: E402
    ManusClient,
    ManusError,
    ManusTaskError,
    ManusTimeoutError,
)

DEFAULT_KEYS = "/root/LabDoctorM/projects/free-api-hunter/configs/manus-keys.json"


def load_keys(keys_path):
    try:
        cfg = json.load(open(keys_path))
    except Exception:
        return [], "https://api.manus.ai/v2"
    keys = []
    for acc in cfg.get("accounts", []):
        k = acc.get("api_key") or acc.get("apiKey") or acc.get("key")
        if k:
            keys.append(k)
    return keys, cfg.get("base_url", "https://api.manus.ai/v2")


async def main():
    if len(sys.argv) < 2:
        print("usage: dispatch_direct.py <prompt> [keys.json]", file=sys.stderr)
        sys.exit(2)
    prompt = sys.argv[1]
    keys_path = sys.argv[2] if len(sys.argv) > 2 else DEFAULT_KEYS

    keys, base_url = load_keys(keys_path)
    if not keys:
        env_key = os.environ.get("MANUS_API_KEY")
        if env_key:
            keys = [env_key]
    if not keys:
        print("NO_KEY_AVAILABLE", file=sys.stderr)
        sys.exit(2)

    last_err = None
    # Ротация по аккаунтам + ограниченные повторы при rate-limit.
    for attempt in range(len(keys) * 3):
        key = keys[attempt % len(keys)]
        client = ManusClient(api_key=key, base_url=base_url)
        try:
            res = await client.create_task(message=prompt)
            task_id = (
                res.get("task_id")
                or (res.get("data") or {}).get("task_id")
                or res.get("id")
            )
            if not task_id:
                last_err = "NO_TASK_ID:" + json.dumps(res)[:300]
                await client.close()
                continue
            await client.wait_for_completion(task_id, timeout=280)
            result = await client.get_result(task_id)
            print(result.text or "")
            await client.close()
            return
        except ManusTimeoutError as e:
            last_err = "MANUS_TIMEOUT:" + str(e)
        except ManusTaskError as e:
            last_err = "MANUS_TASK_ERROR:" + str(e)
        except ManusError as e:
            last_err = "MANUS_ERROR[%s]:%s" % (e.code, e.message)
            if e.code == "rate_limited":
                await asyncio.sleep(15)  # передышка перед ротацией ключа
        except Exception as e:  # noqa: BLE001
            last_err = "UNEXPECTED:" + repr(e)
        finally:
            try:
                await client.close()
            except Exception:
                pass

    print("DISPATCH_FAILED: " + str(last_err), file=sys.stderr)
    sys.exit(6)


if __name__ == "__main__":
    asyncio.run(main())
