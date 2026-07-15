#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
echo "[$(date -Iseconds)] key_pool_healthcheck start"
SETTINGS=searxng/settings.yml
before=$(md5sum "$SETTINGS" 2>/dev/null | awk '{print $1}')
python3 scripts/key_pool.py healthcheck
after=$(md5sum "$SETTINGS" 2>/dev/null | awk '{print $1}')
if [ "${before}" != "${after}" ]; then
  echo "[$(date -Iseconds)] settings.yml changed (key state shifted) -> restarting searxng to pick up new keys"
  docker compose -f docker-compose.searxng.yml restart searxng
else
  echo "[$(date -Iseconds)] no settings change, skip searxng restart"
fi
echo "[$(date -Iseconds)] key_pool_healthcheck done"
