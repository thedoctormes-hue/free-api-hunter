#!/usr/bin/env bash
# lab-search-gateway-addkey.sh — deploy a search provider key + engine into SearXNG.
#
# Usage:
#   bin/lab-search-gateway-addkey.sh <exa|serper|serpapi|olostep> <API_KEY>
#
# What it does:
#   1. Stores the key in /root/.openclaw/.api-keys.json (chmod 600, gitignored).
#   2. Copies the engine module into the running searxng container.
#   3. Writes the env var (EXA_API_KEY / SERPER_API_KEY / ...) into .env.searxng.
#   4. Restarts searxng so the engine + env are picked up.
#   5. Runs the healthcheck.
set -euo pipefail

KEYSTORE=/root/.openclaw/.api-keys.json
ENGINES_DIR=/root/LabDoctorM/projects/free-api-hunter/searxng/engines
COMPOSE=/root/LabDoctorM/projects/free-api-hunter/docker-compose.searxng.yml
ENVFILE=/root/LabDoctorM/projects/free-api-hunter/.env.searxng
HEALTH=/root/LabDoctorM/projects/free-api-hunter/bin/searxng-health.sh

PROVIDER="${1:-}"
APIKEY="${2:-}"

usage() { echo "usage: $0 <exa|serper|serpapi|olostep> <api_key>"; exit 1; }
[ -z "$PROVIDER" ] && usage
[ -z "$APIKEY" ] && usage

case "$PROVIDER" in
  exa|serper|serpapi|olostep) ;;
  *) echo "error: unknown provider '$PROVIDER'"; usage ;;
esac

ENVNAME=$(echo "$PROVIDER" | tr '[:lower:]' '[:upper:]')_API_KEY

# 1) store key (chmod 600)
mkdir -p "$(dirname "$KEYSTORE")"
[ -f "$KEYSTORE" ] || echo '{}' > "$KEYSTORE"
chmod 600 "$KEYSTORE"
TMP=$(mktemp)
jq --arg k "$APIKEY" ".\"$PROVIDER\" = [\$k]" "$KEYSTORE" > "$TMP" && mv "$TMP" "$KEYSTORE"
echo "stored $PROVIDER key in $KEYSTORE"

# 2) copy ALL engine modules into container (folder NOT mounted, to avoid
#    shadowing SearXNG's built-in engines). docker cp writes into the
#    container layer; survives `restart`, lost on `up -d` recreate (re-run).
for f in "$ENGINES_DIR"/*.py; do
  [ -f "$f" ] || continue
  docker cp "$f" searxng:/usr/local/searxng/searx/engines/"$(basename "$f")"
done
echo "copied engine modules -> searxng container"

# 3) inject key into settings.yml (placeholder <ENVNAME> or append engine block)
SETTINGS=/root/LabDoctorM/projects/free-api-hunter/searxng/settings.yml
if grep -q "api_key: <${ENVNAME}>" "$SETTINGS"; then
  sed -i "s|api_key: <${ENVNAME}>|api_key: $APIKEY|" "$SETTINGS"
  echo "injected $PROVIDER key into settings.yml (placeholder)"
elif grep -q "name: $PROVIDER$" "$SETTINGS"; then
  echo "engine $PROVIDER already present in settings.yml (key expected inline)"
else
  cat >> "$SETTINGS" <<EOF

  - name: $PROVIDER
    engine: $PROVIDER
    api_key: $APIKEY
    categories: [general, web]
EOF
  echo "appended $PROVIDER engine block to settings.yml"
fi

# 4) restart searxng to load engine + key
docker compose -f "$COMPOSE" restart searxng
echo "restarted searxng"

# 5) healthcheck
sleep 5
bash "$HEALTH" || echo "WARN: healthcheck failed - check 'docker logs searxng'"
