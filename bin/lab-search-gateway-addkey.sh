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

# 2) copy engine module into container
docker cp "$ENGINES_DIR/$PROVIDER.py" searxng:/usr/local/searxng/searx/engines/$PROVIDER.py
echo "deployed engine $PROVIDER.py -> searxng container"

# 3) write env var for container
touch "$ENVFILE"
grep -v "^${ENVNAME}=" "$ENVFILE" > "$ENVFILE.tmp" || true
echo "$ENVNAME=$APIKEY" >> "$ENVFILE.tmp"
mv "$ENVFILE.tmp" "$ENVFILE"
chmod 600 "$ENVFILE"
echo "env $ENVNAME written to $ENVFILE"

# 4) restart searxng to load engine + env
docker compose -f "$COMPOSE" restart searxng
echo "restarted searxng"

# 5) healthcheck
sleep 5
bash "$HEALTH" || echo "WARN: healthcheck failed - check 'docker logs searxng'"
