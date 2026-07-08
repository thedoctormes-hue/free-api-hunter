#!/bin/bash
# Check all API keys — проверка валидности и баланса всех 15 ключей
# Usage: ./search-check-keys.sh

set -euo pipefail

CONFIG_FILE="$(dirname "$0")/../config/search-keys.yaml"

echo "=== Search Provider Key Check ==="
echo "Date: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo ""

# ─── Tavily ─────────────────────────────────────────────────────

echo "--- Tavily (5 keys) ---"
tavily_keys=$(grep "^  tavily:" -A 6 "$CONFIG_FILE" | grep "^    - " | sed 's/^    - //')
i=1
while IFS= read -r key; do
    [[ -z "$key" ]] && continue
    result=$(curl -s -X POST "https://api.tavily.com/search" \
        -H "Content-Type: application/json" \
        -d "{\"api_key\":\"${key}\",\"query\":\"test\",\"max_results\":1}" \
        --max-time 10 2>/dev/null)
    if echo "$result" | grep -q '"results"'; then
        echo "  Key $i: ✅ OK"
    elif echo "$result" | grep -q '429\|rate.limit'; then
        echo "  Key $i: ⚠️  RATE LIMITED"
    elif echo "$result" | grep -q 'Unauthorized\|invalid'; then
        echo "  Key $i: ❌ INVALID"
    else
        echo "  Key $i: ❓ UNKNOWN ($(echo "$result" | head -c 100))"
    fi
    i=$((i + 1))
done <<< "$tavily_keys"

echo ""

# ─── Firecrawl ──────────────────────────────────────────────────

echo "--- Firecrawl (5 keys) ---"
fc_keys=$(grep "^  firecrawl:" -A 6 "$CONFIG_FILE" | grep "^    - " | sed 's/^    - //')
i=1
while IFS= read -r key; do
    [[ -z "$key" ]] && continue
    result=$(curl -s -X POST "https://api.firecrawl.dev/v1/search" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${key}" \
        -d '{"query":"test","limit":1}' \
        --max-time 10 2>/dev/null)
    if echo "$result" | grep -q '"success":true'; then
        echo "  Key $i: ✅ OK"
    elif echo "$result" | grep -q '429\|rate.limit'; then
        echo "  Key $i: ⚠️  RATE LIMITED"
    elif echo "$result" | grep -q 'Unauthorized\|Invalid'; then
        echo "  Key $i: ❌ INVALID"
    else
        echo "  Key $i: ❓ UNKNOWN ($(echo "$result" | head -c 100))"
    fi
    i=$((i + 1))
done <<< "$fc_keys"

echo ""

# ─── TinyFish ───────────────────────────────────────────────────

echo "--- TinyFish (5 keys) ---"
tf_keys=$(grep "^  tinyfish:" -A 6 "$CONFIG_FILE" | grep "^    - " | sed 's/^    - //')
i=1
while IFS= read -r key; do
    [[ -z "$key" ]] && continue
    result=$(curl -s "https://api.search.tinyfish.ai?query=test&location=US&language=en" \
        -H "X-API-Key: ${key}" \
        --max-time 10 2>/dev/null)
    if echo "$result" | grep -q '"results"'; then
        count=$(echo "$result" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('results',[])))" 2>/dev/null || echo "?")
        echo "  Key $i: ✅ OK ($count results)"
    elif echo "$result" | grep -q 'MISSING_API_KEY\|Unauthorized'; then
        echo "  Key $i: ❌ INVALID"
    else
        echo "  Key $i: ❓ UNKNOWN ($(echo "$result" | head -c 100))"
    fi
    i=$((i + 1))
done <<< "$tf_keys"

echo ""

# ─── SearXNG ────────────────────────────────────────────────────

echo "--- SearXNG (local) ---"
result=$(curl -s "http://localhost:8889/search?q=test&format=json" --max-time 5 2>/dev/null)
if echo "$result" | grep -q '"results"'; then
    count=$(echo "$result" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('results',[])))" 2>/dev/null || echo "?")
    echo "  Local: ✅ OK ($count results)"
else
    echo "  Local: ❌ UNAVAILABLE"
fi

echo ""
echo "=== Done ==="
