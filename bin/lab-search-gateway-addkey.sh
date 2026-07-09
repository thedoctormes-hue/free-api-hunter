#!/usr/bin/env bash
# lab-search-gateway-addkey.sh — deploy search provider key(s) + engine(s) into SearXNG.
#
# Usage:
#   # one key (serper/serpapi/olostep, or first exa):
#   bin/lab-search-gateway-addkey.sh <exa|serper|serpapi|olostep> <API_KEY>
#
#   # Exa POOL for routing (multiple keys -> exa, exa2, exa3, ...):
#   bin/lab-search-gateway-addkey.sh exa <KEY1> <KEY2> <KEY3> ...
#
# What it does:
#   1. Stores key(s) in /root/.openclaw/.api-keys.json (chmod 600, gitignored) as an array.
#   2. Copies engine module(s) into the running searxng container.
#   3. For Exa pool: creates exa / exa2 / exa3 ... engines (one per key) in settings.yml.
#      For single providers: injects/ appends one engine block.
#   4. Restarts searxng.
#   5. Runs the healthcheck.
set -euo pipefail

KEYSTORE=/root/.openclaw/.api-keys.json
ENGINES_DIR=/root/LabDoctorM/projects/free-api-hunter/searxng/engines
COMPOSE=/root/LabDoctorM/projects/free-api-hunter/docker-compose.searxng.yml
HEALTH=/root/LabDoctorM/projects/free-api-hunter/bin/searxng-health.sh
SETTINGS=/root/LabDoctorM/projects/free-api-hunter/searxng/settings.yml

PROVIDER="${1:-}"
shift || true
KEYS=("$@")

usage() { echo "usage: $0 <exa|serper|serpapi|olostep> <API_KEY> [KEY2 KEY3 ...]"; exit 1; }
[ -z "$PROVIDER" ] && usage
[ ${#KEYS[@]} -eq 0 ] && usage

case "$PROVIDER" in
  exa|serper|serpapi|olostep) ;;
  *) echo "error: unknown provider '$PROVIDER'"; usage ;;
esac

# 1) store key(s) as array in central keystore (chmod 600)
mkdir -p "$(dirname "$KEYSTORE")"
[ -f "$KEYSTORE" ] || echo '{}' > "$KEYSTORE"
chmod 600 "$KEYSTORE"
TMP=$(mktemp)
jq --argjson ks "$(printf '%s\n' "${KEYS[@]}" | jq -R . | jq -s .)" ".\"$PROVIDER\" = \$ks" "$KEYSTORE" > "$TMP" && mv "$TMP" "$KEYSTORE"
echo "stored ${#KEYS[@]} $PROVIDER key(s) in $KEYSTORE"

# 2) copy ALL engine modules into container (folder NOT mounted, to avoid
#    shadowing SearXNG's built-in engines). docker cp writes into the
#    container layer; survives `restart`, lost on `up -d` recreate (re-run).
for f in "$ENGINES_DIR"/*.py; do
  [ -f "$f" ] || continue
  docker cp "$f" searxng:/usr/local/searxng/searx/engines/"$(basename "$f")"
done
echo "copied engine modules -> searxng container"

# 3) inject engine block(s) into settings.yml
if [ "$PROVIDER" = "exa" ] && [ ${#KEYS[@]} -gt 1 ]; then
  # POOL: exa, exa2, exa3 ... (one engine per key)
  i=1
  for k in "${KEYS[@]}"; do
    ENG=$([ "$i" -eq 1 ] && echo "exa" || echo "exa$i")
    if ! grep -q "name: $ENG$" "$SETTINGS"; then
      cat >> "$SETTINGS" <<EOF

  - name: $ENG
    engine: $ENG
    api_key: $k
    categories: [general, web, ai]
EOF
      echo "appended $ENG engine block (key #$i)"
    else
      # update existing key inline
      sed -i "/name: $ENG$/,/api_key:/ s|api_key:.*|api_key: $k|" "$SETTINGS"
      echo "updated $ENG key inline"
    fi
    i=$((i + 1))
  done
else
  # SINGLE engine (first key)
  k="${KEYS[0]}"
  if grep -q "name: $PROVIDER$" "$SETTINGS"; then
    sed -i "/name: $PROVIDER$/,/api_key:/ s|api_key:.*|api_key: $k|" "$SETTINGS"
    echo "updated $PROVIDER key inline"
  else
    cat >> "$SETTINGS" <<EOF

  - name: $PROVIDER
    engine: $PROVIDER
    api_key: $k
    categories: [general, web]
EOF
    echo "appended $PROVIDER engine block"
  fi
fi

# 4) restart searxng to load engine(s) + key(s)
docker compose -f "$COMPOSE" restart searxng
echo "restarted searxng"

# 5) healthcheck
sleep 5
bash "$HEALTH" || echo "WARN: healthcheck failed - check 'docker logs searxng'"
