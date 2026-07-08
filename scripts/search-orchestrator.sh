#!/bin/bash
# Search Orchestrator — маршрутизация по типу задачи + циклическая ротация ключей
# Usage: ./search-orchestrator.sh <query> [type] [count]
#
# Types: factual | content | dynamic | broad | deep_research
# Default: factual
#
# Ротация: каждый новый запрос использует следующий ключ по кругу.
# При 429 — пропускает ключ, идёт к следующему.
# Это маскирует мультиаккаунт: запросы идут с разных ключей равномерно.

set -uo pipefail

QUERY="${1:?Usage: $0 <query> [type] [count]}"
TYPE="${2:-factual}"
COUNT="${3:-5}"
CONFIG_FILE="$(cd "$(dirname "$0")" && pwd)/../config/search-keys.yaml"
LOG_FILE="$(cd "$(dirname "$0")" && pwd)/../logs/search-orchestrator.log"

mkdir -p "$(dirname "$LOG_FILE")"

log() {
    echo "[$(date -u '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE" 2>/dev/null
}

log "QUERY='$QUERY' TYPE='$TYPE' COUNT='$COUNT'"

# ─── Key Rotation (циклическая) ─────────────────────────────────
# Каждый вызов возвращает СЛЕДУЮЩИЙ ключ по кругу.
# State хранится в config/.key-index-{provider}

get_next_key() {
    local provider="$1"
    local state_file
    state_file="$(cd "$(dirname "$0")" && pwd)/../config/.key-index-${provider}"

    local idx=0
    [[ -f "$state_file" ]] && idx=$(cat "$state_file" 2>/dev/null || echo 0)

    local keys
    keys=$(grep "^  ${provider}:" -A 6 "$CONFIG_FILE" 2>/dev/null | grep "^    - " | sed 's/^    - //')
    local total
    total=$(echo "$keys" | wc -l)

    local key
    key=$(echo "$keys" | sed -n "$((idx + 1))p")

    # Advance to next (cyclic)
    idx=$(( (idx + 1) % total ))
    echo "$idx" > "$state_file"

    echo "$key"
}

# ─── Provider: Tavily ───────────────────────────────────────────

search_tavily() {
    local query="$1"
    local count="$2"
    local max_retries=5
    local i

    for ((i = 0; i < max_retries; i++)); do
        local key
        key=$(get_next_key tavily)

        local response
        response=$(curl -s -X POST "https://api.tavily.com/search" \
            -H "Content-Type: application/json" \
            -d "{\"api_key\":\"${key}\",\"query\":\"${query}\",\"max_results\":${count},\"include_answer\":true}" \
            --max-time 30 2>/dev/null)

        if echo "$response" | grep -q '"status":429\|"code":429\|rate.limit'; then
            log "Tavily: key $((i+1)) → 429, skip to next"
            continue
        fi

        # Success — check we got results
        if echo "$response" | grep -q '"results"'; then
            log "Tavily: key $((i+1)) → OK"
            echo "$response"
            return 0
        fi

        log "Tavily: key $((i+1)) → empty/error, skip"
    done

    log "Tavily: all keys exhausted"
    echo '{"error":"all_keys_exhausted","provider":"tavily"}'
    return 1
}

# ─── Provider: Firecrawl ────────────────────────────────────────

search_firecrawl() {
    local query="$1"
    local count="$2"
    local max_retries=5
    local i

    for ((i = 0; i < max_retries; i++)); do
        local key
        key=$(get_next_key firecrawl)

        local response
        response=$(curl -s -X POST "https://api.firecrawl.dev/v1/search" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${key}" \
            -d "{\"query\":\"${query}\",\"limit\":${count}}" \
            --max-time 30 2>/dev/null)

        if echo "$response" | grep -q '"status":429\|"code":429\|rate.limit'; then
            log "Firecrawl: key $((i+1)) → 429, skip to next"
            continue
        fi

        if echo "$response" | grep -q '"success":true\|"data"'; then
            log "Firecrawl: key $((i+1)) → OK"
            echo "$response"
            return 0
        fi

        log "Firecrawl: key $((i+1)) → empty/error, skip"
    done

    log "Firecrawl: all keys exhausted"
    echo '{"error":"all_keys_exhausted","provider":"firecrawl"}'
    return 1
}

# ─── Provider: TinyFish ─────────────────────────────────────────

search_tinyfish() {
    local query="$1"
    local count="$2"
    local max_retries=5
    local i

    local encoded_query
    encoded_query=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$query'))" 2>/dev/null || echo "$query")

    for ((i = 0; i < max_retries; i++)); do
        local key
        key=$(get_next_key tinyfish)

        local response
        response=$(curl -s "https://api.search.tinyfish.ai?query=${encoded_query}&location=US&language=en" \
            -H "X-API-Key: ${key}" \
            --max-time 30 2>/dev/null)

        if echo "$response" | grep -q '"code":"MISSING_API_KEY"\|"error"\|429'; then
            log "TinyFish: key $((i+1)) → error/429, skip to next"
            continue
        fi

        if echo "$response" | grep -q '"results"'; then
            log "TinyFish: key $((i+1)) → OK"
            echo "$response"
            return 0
        fi

        log "TinyFish: key $((i+1)) → empty, skip"
    done

    log "TinyFish: all keys exhausted"
    echo '{"error":"all_keys_exhausted","provider":"tinyfish"}'
    return 1
}

# ─── Provider: SearXNG ──────────────────────────────────────────

search_searxng() {
    local query="$1"

    log "SearXNG: querying local instance"

    local encoded_query
    encoded_query=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$query'))" 2>/dev/null || echo "$query")

    curl -s "http://localhost:8889/search?q=${encoded_query}&format=json&categories=general" \
        --max-time 15 2>/dev/null
}

# ─── Scrape: Firecrawl (full page content) ─────────────────────

scrape_firecrawl() {
    local url="$1"
    local max_retries=5
    local i

    for ((i = 0; i < max_retries; i++)); do
        local key
        key=$(get_next_key firecrawl)

        local response
        response=$(curl -s -X POST "https://api.firecrawl.dev/v2/scrape" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${key}" \
            -d "{\"url\":\"${url}\",\"formats\":[\"markdown\"],\"onlyMainContent\":true}" \
            --max-time 60 2>/dev/null)

        if echo "$response" | grep -q '"status":429\|"code":429\|rate.limit'; then
            log "Firecrawl scrape: key $((i+1)) → 429, skip"
            continue
        fi

        if echo "$response" | grep -q '"success":true\|"markdown"'; then
            log "Firecrawl scrape: key $((i+1)) → OK"
            echo "$response"
            return 0
        fi
    done

    echo '{"error":"all_keys_exhausted","provider":"firecrawl_scrape"}'
    return 1
}

# ─── Scrape: TinyFish (JS rendering) ────────────────────────────

scrape_tinyfish() {
    local url="$1"
    local max_retries=5
    local i

    for ((i = 0; i < max_retries; i++)); do
        local key
        key=$(get_next_key tinyfish)

        local response
        response=$(curl -s "https://api.fetch.tinyfish.ai?url=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${url}'))")" \
            -H "X-API-Key: ${key}" \
            --max-time 60 2>/dev/null)

        if echo "$response" | grep -q '"error"\|429'; then
            log "TinyFish fetch: key $((i+1)) → error, skip"
            continue
        fi

        if echo "$response" | grep -q '"content"\|"html"\|"markdown"'; then
            log "TinyFish fetch: key $((i+1)) → OK"
            echo "$response"
            return 0
        fi
    done

    echo '{"error":"all_keys_exhausted","provider":"tinyfish_fetch"}'
    return 1
}

# ─── Deep Research: Parallel search ─────────────────────────────

deep_research() {
    local query="$1"
    local count="$2"
    local tmpdir
    tmpdir=$(mktemp -d)

    log "Deep Research: parallel search"

    # Run all 4 providers in parallel, write to temp files
    search_tavily "$query" "$count" > "$tmpdir/tavily.json" 2>/dev/null &
    search_firecrawl "$query" "$count" > "$tmpdir/firecrawl.json" 2>/dev/null &
    search_tinyfish "$query" "$count" > "$tmpdir/tinyfish.json" 2>/dev/null &
    search_searxng "$query" > "$tmpdir/searxng.json" 2>/dev/null &

    wait

    # Output aggregated JSON
    export DR_TMPDIR="$tmpdir"
    export DR_QUERY="$query"
    python3 -c '
import json, os

tmpdir = os.environ["DR_TMPDIR"]
query = os.environ["DR_QUERY"]

results = {
    "query": query,
    "type": "deep_research",
    "providers": {}
}

for name in ["tavily", "firecrawl", "tinyfish", "searxng"]:
    filepath = os.path.join(tmpdir, name + ".json")
    try:
        with open(filepath) as f:
            raw = f.read()
        data = json.loads(raw)
        if "error" in data:
            results["providers"][name] = {"status": "error", "error": data["error"]}
        else:
            results["providers"][name] = {"status": "ok", "data": data}
    except Exception as e:
        results["providers"][name] = {"status": "error", "error": str(e)}

print(json.dumps(results, indent=2, ensure_ascii=False))
'

    rm -rf "$tmpdir"
}

# ─── Router ─────────────────────────────────────────────────────

case "$TYPE" in
    factual)
        log "Route: factual → Tavily"
        search_tavily "$QUERY" "$COUNT"
        ;;
    content)
        log "Route: content → Firecrawl"
        if [[ "$QUERY" == http* ]]; then
            scrape_firecrawl "$QUERY"
        else
            search_firecrawl "$QUERY" "$COUNT"
        fi
        ;;
    dynamic)
        log "Route: dynamic → TinyFish"
        if [[ "$QUERY" == http* ]]; then
            scrape_tinyfish "$QUERY"
        else
            search_tinyfish "$QUERY" "$COUNT"
        fi
        ;;
    broad)
        log "Route: broad → SearXNG"
        search_searxng "$QUERY"
        ;;
    deep_research)
        log "Route: deep_research → ALL providers"
        deep_research "$QUERY" "$COUNT"
        ;;
    *)
        echo "Unknown type: $TYPE" >&2
        echo "Valid types: factual | content | dynamic | broad | deep_research" >&2
        exit 2
        ;;
esac

log "Done."
