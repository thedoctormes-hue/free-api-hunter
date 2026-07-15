#!/usr/bin/env bash
# key_pool_healthcheck.sh — scheduled prober for paid API keys.
#
# Probes every ACTIVE key in the pool. A 402 (suspended) key is marked
# 'burnt' in config/key_pool.json and removed from the consumers
# (searxng/settings.yml api_keys + config/search-keys.yaml). Logs an alert
# when a key needs replacement.
#
# Intended schedule: hourly (see configs/crontab.txt + deploy/kp-healthcheck.*).
# Safe to run any time: it does NOT restart SearXNG; the burnt state takes
# effect on the next planned gateway restart (see sprint-3 integration test).
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
KP="$HERE/key_pool.py"
LOG_DIR="${FREE_API_HUNTER_LOG:-/var/log/free-api-hunter}"
mkdir -p "$LOG_DIR" 2>/dev/null || LOG_DIR="$HERE/../data"

echo "=== key_pool healthcheck: $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
python3 "$KP" healthcheck --apply
echo "--- pool status ---"
python3 "$KP" status
echo "=== done ==="
