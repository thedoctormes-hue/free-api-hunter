#!/bin/bash
# Search Orchestrator — маршрутизация по типу задачи + циклическая ротация ключей
# Usage: ./search-orchestrator.sh <query> [type] [count]
#
# Types: factual | content | dynamic | broad | verify | deep_research
# Default: factual
# verify: cross-checks Tavily against SearXNG, flags answer as
#         UNVERIFIED_SYNTHESIS when URL overlap < threshold.
# Fallback: single modes auto-failover across providers
#         (Tavily→Firecrawl→TinyFish→SearXNG) via run_chain.
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

# ─── Processing library + cache + adaptive routing ─────────────
LIB="$(cd "$(dirname "$0")" && pwd)/lib/process.py"

# Кэш результатов (файловый, по mtime-age)
CACHE_DIR="$(cd "$(dirname "$0")" && pwd)/../data/cache"
init_cache() { mkdir -p "$CACHE_DIR"; }
init_cache
ttl_for() {
    if echo "$1" | grep -qiE 'news|cve|version|release|latest|today|202[0-9]|update|security|v[0-9]'; then
        echo 3600
    else
        echo 86400
    fi
}
cache_key() {
    python3 -c "import hashlib,sys;print(hashlib.sha256((':'.join(sys.argv[1:])).encode()).hexdigest())" "$1" "$2" "$3"
}
cache_get() {
    local f="$CACHE_DIR/$1.json"
    [[ -f "$f" ]] || return 1
    local age; age=$(( $(date +%s) - $(stat -c %Y "$f") ))
    (( age < $2 )) && { cat "$f"; return 0; }
    return 1
}
cache_put() {
    printf '%s' "$3" > "$CACHE_DIR/$1.json"
}

# Адаптивная маршрутизация: статистика успехов провайдеров по типу
STATS_FILE="$(cd "$(dirname "$0")" && pwd)/../config/.provider-stats.json"
record_outcome() {
    local prov="$1" type="$2" ok="$3"
    python3 - "$STATS_FILE" "$prov" "$type" "$ok" <<'PY'
import sys, json, os
f, p, typ, ok = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
d = {}
if os.path.exists(f):
    try:
        d = json.load(open(f))
    except Exception:
        d = {}
d.setdefault(p, {}).setdefault(typ, {"ok": 0, "total": 0})
d[p][typ]["total"] += 1
if ok == "1":
    d[p][typ]["ok"] += 1
json.dump(d, open(f, "w"))
PY
}
adaptive_order() {
    local type="$1"
    local order
    case "$type" in
        factual)  order="search_tavily search_firecrawl search_tinyfish search_searxng" ;;
        content)  order="search_firecrawl search_tavily search_tinyfish search_searxng" ;;
        dynamic)  order="search_tinyfish search_firecrawl search_tavily search_searxng" ;;
        broad)    order="search_searxng search_tavily search_firecrawl search_tinyfish" ;;
        *)        order="search_tavily search_firecrawl search_tinyfish search_searxng" ;;
    esac
    python3 - "$STATS_FILE" "$type" $order <<'PY'
import sys, json, os
f, typ = sys.argv[1], sys.argv[2]
provs = sys.argv[3:]
try:
    d = json.load(open(f))
except Exception:
    d = {}
def score(p):
    st = d.get(p.replace("search_", ""), {}).get(typ)
    if not st or st.get("total", 0) == 0:
        return 0.5
    return st["ok"] / st["total"]
provs.sort(key=score, reverse=True)
print(" ".join(provs))
PY
}

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
    query="$query" encoded_query=$(python3 -c "import urllib.parse,os; print(urllib.parse.quote(os.environ.get('query','')))" 2>/dev/null || echo "$query")

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
    query="$query" encoded_query=$(python3 -c "import urllib.parse,os; print(urllib.parse.quote(os.environ.get('query','')))" 2>/dev/null || echo "$query")

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
        url="$url" response=$(curl -s "https://api.fetch.tinyfish.ai?url=$(python3 -c "import urllib.parse,os; print(urllib.parse.quote(os.environ.get('url','')))")" \
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

# deep_research_v2: базовый параллельный поиск по всем провайдерам +
# процессинг (мерж/дедуп/фрешнес/противоречия) + опциональная
# декомпозиция сложного запроса на подвопросы.
deep_research_v2() {
    local query="$1" count="$2"
    local base
    base=$(deep_research "$query" "$count" | python3 "$LIB" merge)

    local subs_json
    subs_json=$(echo "{\"query\":\"$query\"}" | python3 "$LIB" decompose)
    local n
    n=$(echo "$subs_json" | python3 -c "import sys,json;print(len(json.load(sys.stdin).get('subqueries',[])))")

    if (( n > 1 )); then
        log "Decomposition: $n subqueries"
        local tmp; tmp=$(mktemp -d)
        echo "$subs_json" | python3 -c "import sys,json;[print(q) for q in json.load(sys.stdin)['subqueries']]" | \
        while read -r sq; do
            CHAIN_TYPE=factual r=$(run_chain "$sq" "$count" $(adaptive_order factual) | python3 "$LIB" freshness)
            echo "$r" | python3 -c "import sys,json;d=json.loads(sys.stdin.read());print(json.dumps(d.get('results',[]),ensure_ascii=False))" >> "$tmp/subs.txt"
        done
        if [[ -s "$tmp/subs.txt" ]]; then
            base=$(echo "$base" | python3 "$LIB" merge_subs "$tmp/subs.txt")
        fi
        rm -rf "$tmp"
    fi
    echo "$base"
}

# ─── Fallback chain + Verification ─────────────────────────────
# run_chain: tries providers in order, returns first non-error result,
# tags _meta.provider_used / _meta.fell_back. Real cross-provider
# fallback (previously documented only).
# provider_ok: per-provider success detection (schemas differ).
provider_ok() {
    local fn="$1" result="$2"
    case "$fn" in
        search_firecrawl|scrape_firecrawl)
            echo "$result" | grep -q '"success":true\|"data"' ;;
        *)
            echo "$result" | grep -q '"results"' ;;
    esac
}
run_chain() {
    local query="$1" count="$2"; shift 2
    local primary="$1"
    local fn result answered=""
    local ctype="${CHAIN_TYPE:-$TYPE}"
    for fn in "$@"; do
        log "Chain: trying $fn"
        local short="${fn/search_/}"
        record_outcome "$short" "$ctype" "0"
        result=$("$fn" "$query" "$count")
        if [[ -n "$result" ]] && provider_ok "$fn" "$result"; then
            record_outcome "$short" "$ctype" "1"
            answered="$fn"
            break
        fi
    done
    if [[ -z "$answered" ]]; then
        echo '{"error":"all_providers_exhausted"}'
        return 1
    fi
    echo "$result" | python3 -c "import sys,json
raw=sys.stdin.read()
try:
    d=json.loads(raw)
except Exception:
    print(raw); sys.exit(0)
used='${answered/search_/}'; prim='${primary/search_/}'
d.setdefault('_meta',{})
d['_meta']['provider_used']=used
d['_meta']['fell_back']=(used!=prim)
print(json.dumps(d, ensure_ascii=False))"
}

# verify_research: cross-checks Tavily (search-first) vs SearXNG
# (free local metasearch). URL intersection = strong reliability
# signal. Flags answer UNVERIFIED_SYNTHESIS when overlap < threshold.
verify_research() {
    local query="$1" count="$2" threshold="${3:-2}"
    local tj; tj=$(search_tavily "$query" "$count")
    local fj; fj=$(search_searxng "$query")
    echo "$tj" | python3 - "$fj" "$threshold" <<'PY'
import sys, json
tj_raw=sys.stdin.read(); fj_raw=sys.argv[1]
try:
    thr=int(sys.argv[2])
except Exception:
    thr=2
try:
    d=json.loads(tj_raw)
except Exception:
    d={"error":"tavily_unavailable_or_invalid"}
def urls(blob):
    try:
        x=json.loads(blob)
    except Exception:
        return []
    if isinstance(x,dict) and "results" in x:
        return [r.get("url","") for r in x.get("results",[]) if r.get("url")]
    return []
t=urls(tj_raw); s=urls(fj_raw); inter=set(t)&set(s)
verified=len(inter)>=max(1,thr-1)
d.setdefault("_meta",{})
d["_meta"]["verification"]={
  "cross_checked_with":["tavily","searxng"],
  "tavily_urls":len(t),"searxng_urls":len(s),
  "overlapping_urls":sorted(inter),"overlap_count":len(inter),
  "threshold":thr,"verified":verified,
  "answer_status":("verified" if verified else "unverified_synthesis")
}
if not verified and "answer" in d:
    d["answer"]="[UNVERIFIED_SYNTHESIS] "+str(d["answer"])
print(json.dumps(d, ensure_ascii=False))
PY
}

# ─── Router ─────────────────────────────────────────────────────

case "$TYPE" in
    factual|content|dynamic|broad)
        log "Route: $TYPE (cache + adaptive fallback)"
        KEY=$(cache_key "$QUERY" "$TYPE" "$COUNT")
        TTL=$(ttl_for "$QUERY")
        CACHED_JSON=$(cache_get "$KEY" "$TTL")
        if [[ -n "$CACHED_JSON" ]]; then
            echo "$CACHED_JSON" | python3 -c 'import sys,json
try:
 d=json.loads(sys.stdin.read()); d.setdefault("_meta",{}); d["_meta"]["cached"]=True; print(json.dumps(d,ensure_ascii=False))
except Exception:
 print(sys.stdin.read())'
            log "Cache HIT → served"
            exit 0
        fi
        if [[ "$QUERY" == http* && ( "$TYPE" == "content" || "$TYPE" == "dynamic" ) ]]; then
            if [[ "$TYPE" == "content" ]]; then ORDER="scrape_firecrawl scrape_tinyfish search_searxng"
            else ORDER="scrape_tinyfish scrape_firecrawl search_searxng"; fi
            CHAIN_TYPE="$TYPE" OUT=$(run_chain "$QUERY" "$COUNT" $ORDER)
        else
            ORDER=$(adaptive_order "$TYPE")
            OUT=$(run_chain "$QUERY" "$COUNT" $ORDER)
        fi
        OUT=$(echo "$OUT" | python3 "$LIB" freshness)
        if ! echo "$OUT" | grep -q '"error"'; then
            cache_put "$KEY" "$TTL" "$OUT"
        fi
        echo "$OUT"
        ;;
    verify)
        log "Route: verify → Tavily × SearXNG cross-check"
        verify_research "$QUERY" "$COUNT" "${4:-2}"
        ;;
    deep_research)
        log "Route: deep_research → ALL providers + decomposition"
        deep_research_v2 "$QUERY" "$COUNT"
        ;;
    *)
        echo "Unknown type: $TYPE" >&2
        echo "Valid types: factual | content | dynamic | broad | verify | deep_research" >&2
        exit 2
        ;;
esac

log "Done."
