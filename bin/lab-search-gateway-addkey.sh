#!/usr/bin/env bash
# lab-search-gateway-addkey.sh — Unified Search Gateway management.
#
# Architecture:
#   SearXNG + 5 premium API pools (exa/tavily/firecrawl/tinyfish/olostep),
#   each rotating its API keys internally (round-robin). Plus free built-in
#   engines (wiby/wikipedia/bing/seznam/mojeek/...).
#
# Key rotation: each engine module reads `api_keys: [...]` from settings.yml
#   and picks the next key per request (threading.Lock + global index).
#
# Usage:
#   bin/lab-search-gateway-addkey.sh deploy
#       Copy all engine modules into the container, restart, healthcheck.
#       Run after `docker compose up -d` recreates the image (modules live
#       in the container layer, lost on recreate).
#
#   bin/lab-search-gateway-addkey.sh add-key <provider> <KEY1> [KEY2 ...]
#       Store key(s) in central keystore + update `api_keys:` list in settings.yml.
#       provider ∈ {exa,tavily,firecrawl,tinyfish,olostep}
#
#   bin/lab-search-gateway-addkey.sh health
#       Per-engine + general healthcheck.

set -euo pipefail

KEYSTORE=/root/.openclaw/.api-keys.json
ENGINES_DIR=/root/LabDoctorM/projects/free-api-hunter/searxng/engines
COMPOSE=/root/LabDoctorM/projects/free-api-hunter/docker-compose.searxng.yml
HEALTH=/root/LabDoctorM/projects/free-api-hunter/bin/searxng-health.sh
SETTINGS=/root/LabDoctorM/projects/free-api-hunter/searxng/settings.yml

# Engine modules actually deployed (free-tier premium providers).
MODULES="exa tavily firecrawl tinyfish olostep"

usage() {
  echo "usage:"
  echo "  $0 deploy"
  echo "  $0 add-key <exa|tavily|firecrawl|tinyfish|olostep> <KEY1> [KEY2 ...]"
  echo "  $0 health"
  exit 1
}

cmd="${1:-}"; shift || true

case "$cmd" in
  deploy)
    for m in $MODULES; do
      docker cp "$ENGINES_DIR/$m.py" "searxng:/usr/local/searxng/searx/engines/$m.py"
    done
    echo "copied modules: $MODULES"
    docker compose -f "$COMPOSE" restart searxng
    sleep 5
    bash "$HEALTH"
    ;;
  add-key)
    PROVIDER="${1:-}"; shift || true
    KEYS=("$@")
    [ -z "$PROVIDER" ] && usage
    [ ${#KEYS[@]} -eq 0 ] && usage
    case "$PROVIDER" in
      exa|tavily|firecrawl|tinyfish|olostep) ;;
      *) echo "error: unknown provider '$PROVIDER'"; usage ;;
    esac
    # 1) store in keystore as array
    mkdir -p "$(dirname "$KEYSTORE")"
    [ -f "$KEYSTORE" ] || echo '{}' > "$KEYSTORE"
    chmod 600 "$KEYSTORE"
    TMP=$(mktemp)
    jq --argjson ks "$(printf '%s\n' "${KEYS[@]}" | jq -R . | jq -s .)" ".\"$PROVIDER\" = \$ks" "$KEYSTORE" > "$TMP" && mv "$TMP" "$KEYSTORE"
    echo "stored ${#KEYS[@]} $PROVIDER key(s) in $KEYSTORE"
    # 2) update api_keys: list in settings.yml (python+yaml preserves structure)
    python3 - "$SETTINGS" "$PROVIDER" "${KEYS[@]}" <<'PY'
import sys, yaml
path, provider = sys.argv[1], sys.argv[2]
keys = sys.argv[3:]
with open(path) as f:
    cfg = yaml.safe_load(f)
for e in cfg.get("engines", []):
    if e.get("name") == provider:
        e["api_keys"] = keys
        print(f"updated api_keys for {provider} ({len(keys)} keys)")
        break
else:
    cfg.setdefault("engines", []).append({
        "name": provider, "engine": provider, "shortcut": provider[:2],
        "api_keys": keys, "categories": ["general", "web", "ai"],
    })
    print(f"appended {provider} engine block")
with open(path, "w") as f:
    yaml.safe_dump(cfg, f, sort_keys=False, default_flow_style=False)
PY
    # 3) restart + healthcheck
    docker compose -f "$COMPOSE" restart searxng
    sleep 5
    bash "$HEALTH"
    ;;
  health)
    bash "$HEALTH"
    ;;
  *) usage ;;
esac
