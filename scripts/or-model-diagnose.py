#!/usr/bin/env python3
"""
OpenRouter Free Model Diagnostics
Сканирует все бесплатные модели OpenRouter, тестирует их,
и выводит ранжированный отчёт.

Использование:
  python3 or-model-diagnose.py                  # полное сканирование
  python3 or-model-diagnose.py --json           # JSON-вывод
  python3 or-model-diagnose.py --quick          # быстрый тест (только известные)
  python3 or-model-diagnose.py --dry-run        # только список моделей

Автор: raven (Ворон), 2026-06-30
Документация: ../docs/openrouter-model-diagnostics.md
"""

import json
import os
import sys
import time
import argparse
import urllib.request
import urllib.error
from datetime import datetime, timezone

# ─── Конфигурация ───────────────────────────────────────────────

OPENROUTER_API_URL = "https://openrouter.ai/api/v1"
MODELS_ENDPOINT = f"{OPENROUTER_API_URL}/models"
CHAT_ENDPOINT = f"{OPENROUTER_API_URL}/chat/completions"

# Тестовый промпт среднего размера
TEST_PROMPT = """You are a system administrator. Analyze the following server log summary and provide 3 specific recommendations:

Server: Ubuntu 22.04, 4 CPU, 8GB RAM
Services: 9 active microservices
Issues observed:
1. 5 cascade timeouts of primary LLM model in 1 hour
2. 343 SSH brute-force attempts from 30 unique IPs in 4 hours
3. Gateway process using 1.39GB RAM (17.1%)
4. One external IP scanning /error.log paths

Provide exactly 3 actionable recommendations, one sentence each."""

REQUEST_TIMEOUT = 45  # секунд
REQUEST_DELAY = 2     # пауза между запросами (чтобы не ловить 429)
MAX_TOKENS = 200

# Известные проблемные модели (не тестировать)
KNOWN_BROKEN = {
    "google/lyria-3-pro-preview": "402 insufficient credits",
    "google/lyria-3-clip-preview": "402 insufficient credits",
    "nvidia/nemotron-3.5-content-safety:free": "content-safety only, not for general use",
    "liquid/lfm-2.5-1.2b-instruct:free": "too small (1.2B), low quality",
}

# ─── Получение API-ключа ─────────────────────────────────────────

def get_api_key():
    """Читает OPENROUTER_API_KEY из окружения или .env файла."""
    key = os.environ.get("OPENROUTER_API_KEY", "")
    if key:
        return key

    env_path = os.path.expanduser("~/.openclaw/.env")
    if os.path.exists(env_path):
        with open(env_path) as f:
            for line in f:
                if line.startswith("OPENROUTER_API_KEY="):
                    return line.strip().split("=", 1)[1].strip("'\"")

    print("ERROR: OPENROUTER_API_KEY not found in environment or ~/.openclaw/.env", file=sys.stderr)
    sys.exit(1)

# ─── API-запросы ─────────────────────────────────────────────────

def fetch_free_models(api_key):
    """Получает список всех бесплатных моделей из OpenRouter."""
    req = urllib.request.Request(
        MODELS_ENDPOINT,
        headers={"Authorization": f"Bearer {api_key}"}
    )
    try:
        resp = urllib.request.urlopen(req, timeout=15)
        data = json.loads(resp.read().decode())
    except Exception as e:
        print(f"ERROR: failed to fetch models: {e}", file=sys.stderr)
        sys.exit(1)

    free = []
    for m in data.get("data", []):
        pricing = m.get("pricing", {})
        if pricing.get("prompt") == "0" and pricing.get("completion") == "0":
            free.append({
                "id": m["id"],
                "name": m.get("name", ""),
                "context_length": m.get("context_length", 0),
                "max_tokens": m.get("top_provider", {}).get("max_completion_tokens", 0),
            })

    return free


def test_model(api_key, model_id, prompt=TEST_PROMPT):
    """Отправляет тестовый запрос в модель и возвращает результат."""
    try:
        req = urllib.request.Request(
            CHAT_ENDPOINT,
            data=json.dumps({
                "model": model_id,
                "messages": [{"role": "user", "content": prompt}],
                "max_tokens": MAX_TOKENS,
                "temperature": 0.3,
            }).encode(),
            headers={
                "Authorization": f"Bearer {api_key}",
                "Content-Type": "application/json",
            },
        )

        start = time.time()
        resp = urllib.request.urlopen(req, timeout=REQUEST_TIMEOUT)
        elapsed_ms = (time.time() - start) * 1000

        data = json.loads(resp.read().decode())

        if "choices" in data and data["choices"]:
            content = data["choices"][0].get("message", {}).get("content", "")
            usage = data.get("usage", {})
            return {
                "status": "OK",
                "time_ms": round(elapsed_ms, 0),
                "total_tokens": usage.get("total_tokens", 0),
                "completion_tokens": usage.get("completion_tokens", 0),
                "response": content[:300],
            }
        else:
            return {
                "status": "EMPTY",
                "time_ms": round(elapsed_ms, 0),
                "total_tokens": 0,
                "completion_tokens": 0,
                "response": str(data)[:200],
            }

    except urllib.error.HTTPError as e:
        elapsed_ms = (time.time() - start) * 1000
        body = ""
        try:
            body = e.read().decode()[:200]
        except:
            pass
        error_msg = ""
        try:
            err_data = json.loads(body)
            error_msg = err_data.get("error", {}).get("message", "")[:100]
        except:
            error_msg = body[:100]

        return {
            "status": f"HTTP_{e.code}",
            "time_ms": round(elapsed_ms, 0),
            "total_tokens": 0,
            "completion_tokens": 0,
            "response": error_msg,
        }

    except Exception as e:
        elapsed_ms = (time.time() - start) * 1000
        return {
            "status": "ERROR",
            "time_ms": round(elapsed_ms, 0),
            "total_tokens": 0,
            "completion_tokens": 0,
            "response": str(e)[:200],
        }


# ─── Ранжирование ────────────────────────────────────────────────

def classify_tier(result):
    """Определяет тир модели по результатам теста."""
    if result["status"] != "OK":
        return "broken"

    ms = result["time_ms"]
    tok = result["completion_tokens"]

    if ms < 1000 and tok >= 100:
        return "tier1"
    elif ms < 2500 and tok >= 100:
        return "tier2"
    elif tok >= 50:
        return "tier3"
    else:
        return "exclude"


def generate_report(results, free_models_count):
    """Генерирует текстовый отчёт."""
    ok = [r for r in results if r["status"] == "OK"]
    failed = [r for r in results if r["status"] != "OK"]
    skipped = [r for r in results if r["status"] == "SKIPPED"]

    tiers = {"tier1": [], "tier2": [], "tier3": [], "exclude": []}
    for r in ok:
        tier = classify_tier(r)
        tiers[tier].append(r)

    lines = []
    lines.append("=" * 80)
    lines.append("OPENROUTER FREE MODEL DIAGNOSTICS REPORT")
    lines.append(f"Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')}")
    lines.append("=" * 80)
    lines.append("")
    lines.append(f"Total free models in catalog: {free_models_count}")
    lines.append(f"Tested: {len(results) - len(skipped)}")
    lines.append(f"Skipped (known broken): {len(skipped)}")
    lines.append(f"Working: {len(ok)}")
    lines.append(f"Failed: {len(failed)}")
    lines.append("")

    # Tier 1
    if tiers["tier1"]:
        lines.append("TIER 1 — PRIMARY FALLBACK (fast + high quality):")
        lines.append("-" * 80)
        for r in sorted(tiers["tier1"], key=lambda x: x["time_ms"]):
            lines.append(f"  {r['model']:<55} {r['time_ms']:6.0f}ms  {r['completion_tokens']:3d}tok")
            lines.append(f"    {r['response'][:80]}")
        lines.append("")

    # Tier 2
    if tiers["tier2"]:
        lines.append("TIER 2 — BACKUP (quality but slower):")
        lines.append("-" * 80)
        for r in sorted(tiers["tier2"], key=lambda x: x["time_ms"]):
            lines.append(f"  {r['model']:<55} {r['time_ms']:6.0f}ms  {r['completion_tokens']:3d}tok")
            lines.append(f"    {r['response'][:80]}")
        lines.append("")

    # Tier 3
    if tiers["tier3"]:
        lines.append("TIER 3 — MINIMAL (short responses):")
        lines.append("-" * 80)
        for r in sorted(tiers["tier3"], key=lambda x: x["time_ms"]):
            lines.append(f"  {r['model']:<55} {r['time_ms']:6.0f}ms  {r['completion_tokens']:3d}tok")
        lines.append("")

    # Failed
    if failed:
        lines.append("FAILED MODELS:")
        lines.append("-" * 80)
        # Группируем по типу ошибки
        by_error = {}
        for r in failed:
            status = r["status"]
            by_error.setdefault(status, []).append(r)

        for status, items in sorted(by_error.items()):
            lines.append(f"  [{status}] ({len(items)} models):")
            for r in items:
                lines.append(f"    {r['model']:<55} {r['response'][:70]}")
        lines.append("")

    # Skipped
    if skipped:
        lines.append("SKIPPED (known broken):")
        lines.append("-" * 80)
        for r in skipped:
            lines.append(f"  {r['model']:<55} {r['response']}")
        lines.append("")

    # Рекомендуемый fallback-список
    lines.append("=" * 80)
    lines.append("RECOMMENDED FALLBACK ORDER:")
    lines.append("=" * 80)
    fallback_num = 0
    for tier_name in ["tier1", "tier2", "tier3"]:
        for r in sorted(tiers[tier_name], key=lambda x: x["time_ms"]):
            fallback_num += 1
            lines.append(f"  {fallback_num}. {r['model']} ({r['time_ms']:.0f}ms, {r['completion_tokens']}tok)")
    lines.append("")

    return "\n".join(lines)


# ─── Главная функция ─────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="OpenRouter Free Model Diagnostics")
    parser.add_argument("--json", action="store_true", help="Output as JSON")
    parser.add_argument("--quick", action="store_true", help="Test only known working models")
    parser.add_argument("--dry-run", action="store_true", help="List models without testing")
    parser.add_argument("--delay", type=float, default=REQUEST_DELAY, help="Delay between requests (seconds)")
    args = parser.parse_args()

    api_key = get_api_key()

    print("Fetching free models from OpenRouter...", file=sys.stderr)
    free_models = fetch_free_models(api_key)
    print(f"Found {len(free_models)} free models", file=sys.stderr)

    if args.dry_run:
        for m in free_models:
            print(f"  {m['id']:<55} ctx={m['context_length']:,}")
        return

    # Фильтр для --quick
    if args.quick:
        known_working = [
            "nvidia/nemotron-3-nano-30b-a3b:free",
            "nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free",
            "nvidia/nemotron-3-ultra-550b-a55b:free",
            "nvidia/nemotron-3-super-120b-a12b:free",
            "google/gemma-4-26b-a4b-it:free",
            "openrouter/owl-alpha",
        ]
        free_models = [m for m in free_models if m["id"] in known_working]
        print(f"Quick mode: testing {len(free_models)} known models", file=sys.stderr)

    # Тестирование
    results = []
    for i, m in enumerate(free_models):
        model_id = m["id"]

        # Пропускаем известно сломанные
        if model_id in KNOWN_BROKEN:
            results.append({
                "model": model_id,
                "status": "SKIPPED",
                "time_ms": 0,
                "total_tokens": 0,
                "completion_tokens": 0,
                "response": KNOWN_BROKEN[model_id],
            })
            print(f"[{i+1}/{len(free_models)}] SKIP {model_id}", file=sys.stderr)
            continue

        print(f"[{i+1}/{len(free_models)}] Testing {model_id}...", file=sys.stderr)
        result = test_model(api_key, model_id)
        result["model"] = model_id
        results.append(result)

        status_icon = "✅" if result["status"] == "OK" else "❌"
        print(f"  {status_icon} {result['status']} ({result['time_ms']:.0f}ms)", file=sys.stderr)

        if i < len(free_models) - 1:
            time.sleep(args.delay)

    # Вывод
    if args.json:
        output = {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "total_free_models": len(free_models),
            "tested": len([r for r in results if r["status"] != "SKIPPED"]),
            "working": len([r for r in results if r["status"] == "OK"]),
            "failed": len([r for r in results if r["status"] not in ("OK", "SKIPPED")]),
            "results": results,
        }
        print(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        print(generate_report(results, len(free_models)))


if __name__ == "__main__":
    main()
