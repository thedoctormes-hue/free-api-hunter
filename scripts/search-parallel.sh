#!/bin/bash
# Parallel Search — запрос ко всем провайдерам с агрегацией и дедупом
# Usage: ./search-parallel.sh <query> [count]

set -uo pipefail

QUERY="${1:?Usage: $0 <query> [count]}"
COUNT="${2:-5}"
CONFIG_FILE="$(cd "$(dirname "$0")" && pwd)/../config/search-keys.yaml"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

mkdir -p "$SCRIPT_DIR/../logs"

# Cyclic key rotation — each call returns next key in round-robin
get_next_key() {
    local provider="$1"
    local state_file="${SCRIPT_DIR}/../config/.key-index-${provider}"

    local idx=0
    [[ -f "$state_file" ]] && idx=$(cat "$state_file" 2>/dev/null || echo 0)

    local keys
    keys=$(grep "^  ${provider}:" -A 6 "$CONFIG_FILE" 2>/dev/null | grep "^    - " | sed 's/^    - //')
    local total
    total=$(echo "$keys" | wc -l)

    local key
    key=$(echo "$keys" | sed -n "$((idx + 1))p")

    idx=$(( (idx + 1) % total ))
    echo "$idx" > "$state_file"

    echo "$key"
}

TAVILY_KEY=$(get_next_key tavily)
FIRECRAWL_KEY=$(get_next_key firecrawl)
TINYFISH_KEY=$(get_next_key tinyfish)

ENCODED_QUERY=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$QUERY'))" 2>/dev/null || echo "$QUERY")

TMPDIR=$(mktemp -d)

# Query all providers sequentially (reliable, no orphaned processes)
curl -s -X POST "https://api.tavily.com/search" \
    -H "Content-Type: application/json" \
    -d "{\"api_key\":\"${TAVILY_KEY}\",\"query\":\"${QUERY}\",\"max_results\":${COUNT},\"include_answer\":true}" \
    --max-time 30 > "$TMPDIR/tavily.json" 2>/dev/null

curl -s -X POST "https://api.firecrawl.dev/v1/search" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${FIRECRAWL_KEY}" \
    -d "{\"query\":\"${QUERY}\",\"limit\":${COUNT}}" \
    --max-time 30 > "$TMPDIR/firecrawl.json" 2>/dev/null

curl -s "https://api.search.tinyfish.ai?query=${ENCODED_QUERY}&location=US&language=en" \
    -H "X-API-Key: ${TINYFISH_KEY}" \
    --max-time 30 > "$TMPDIR/tinyfish.json" 2>/dev/null

curl -s "http://localhost:8889/search?q=${ENCODED_QUERY}&format=json&categories=general" \
    --max-time 15 > "$TMPDIR/searxng.json" 2>/dev/null

# Aggregate via python3 with env vars (no heredoc, no $ expansion issues)
export PF_QUERY="$QUERY"
export PF_TMPDIR="$TMPDIR"

python3 -c '
import json, os, sys

tmpdir = os.environ["PF_TMPDIR"]
query = os.environ["PF_QUERY"]

results = {
    "query": query,
    "providers": {},
    "aggregated_urls": [],
    "total_results": 0
}

def extract_urls(data, provider):
    urls = []
    if provider == "tavily" and isinstance(data, dict):
        for r in data.get("results", []):
            urls.append({"url": r.get("url",""), "title": r.get("title",""), "snippet": r.get("content","")[:200]})
    elif provider == "firecrawl" and isinstance(data, dict):
        for r in data.get("data", []):
            urls.append({"url": r.get("url","") or r.get("metadata",{}).get("url",""),
                         "title": r.get("title","") or r.get("metadata",{}).get("title",""),
                         "snippet": (r.get("markdown","") or "")[:200]})
    elif provider == "tinyfish" and isinstance(data, dict):
        for r in data.get("results", []):
            urls.append({"url": r.get("url",""), "title": r.get("title",""), "snippet": r.get("snippet","")[:200]})
    elif provider == "searxng" and isinstance(data, dict):
        for r in data.get("results", []):
            urls.append({"url": r.get("url",""), "title": r.get("title",""), "snippet": r.get("content","")[:200]})
    return urls

seen_urls = set()

for provider in ["tavily", "firecrawl", "tinyfish", "searxng"]:
    filepath = os.path.join(tmpdir, provider + ".json")
    try:
        with open(filepath) as f:
            data = json.load(f)
        urls = extract_urls(data, provider)
        unique = []
        for u in urls:
            if u["url"] and u["url"] not in seen_urls:
                seen_urls.add(u["url"])
                unique.append(u)
        results["providers"][provider] = {"status": "ok", "count": len(unique), "results": unique}
        results["total_results"] += len(unique)
        results["aggregated_urls"].extend(unique)
    except Exception as e:
        results["providers"][provider] = {"status": "error", "error": str(e)}

print(json.dumps(results, indent=2, ensure_ascii=False))
'

rm -rf "$TMPDIR"
