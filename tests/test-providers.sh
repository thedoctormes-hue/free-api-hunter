#!/bin/bash
# Test suite for search-orchestrator.sh (free-api-hunter)
# Адаптировано из api-hub/tests/test-providers.sh + добавлены
# детерминированные тесты fallback / verify / meta-тегов.
# Usage: ./tests/test-providers.sh

set -uo pipefail

SCRIPTS_DIR="$(cd "$(dirname "$0")/../scripts" && pwd)"
ORC="$SCRIPTS_DIR/search-orchestrator.sh"
PASS=0; FAIL=0; WARN=0

echo "=== Search Orchestrator Tests ==="
echo "Date: $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
echo ""

# Helper: создать temp-копию оркестратора с заглушёнными провайдерами
# $1 = список имён функций (через пробел) -> всегда возвращают error
stub_providers() {
    local tmp; tmp=$(mktemp "$SCRIPTS_DIR/.stub.XXXXXX")
    cp "$ORC" "$tmp"
    python3 - "$tmp" "$1" <<'PY'
import sys
tmp=sys.argv[1]; names=sys.argv[2].split()
s=open(tmp).read()
for fn in names:
    s=s.replace(fn+'() {\n', fn+'() {\n    echo \'{"error":"all_keys_exhausted","provider":"%s"}\'; return 1\n' % fn, 1)
open(tmp,"w").write(s)
PY
    echo "$tmp"
}

# ─── Test 1: Syntax ─────────────────────────────────────────────
echo "--- Test 1: Syntax (bash -n) ---"
if bash -n "$ORC" 2>/dev/null; then
    echo "  ✅ syntax OK"; PASS=$((PASS+1))
else
    echo "  ❌ syntax error"; FAIL=$((FAIL+1))
fi

# ─── Test 2: Fallback traversal (all paid stubbed → SearXNG) ────
echo "--- Test 2: Fallback traverses to SearXNG ---"
T=$(stub_providers "search_tavily search_firecrawl search_tinyfish")
r=$(bash "$T" "fallback traversal" factual 3 2>/dev/null); rm -f "$T"
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); m=d.get('_meta',{}); sys.exit(0 if m.get('provider_used')=='searxng' and m.get('fell_back') else 1)" 2>/dev/null; then
    echo "  ✅ fallback Tavily→Firecrawl→TinyFish→SearXNG works"; PASS=$((PASS+1))
else
    echo "  ❌ fallback did not reach SearXNG"; FAIL=$((FAIL+1))
fi

# ─── Test 3: Fallback engages when primary fails ────────────────
echo "--- Test 3: Fallback engages (Tavily down) ---"
T=$(stub_providers "search_tavily")
r=$(bash "$T" "fallback engage" factual 3 2>/dev/null); rm -f "$T"
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); m=d.get('_meta',{}); sys.exit(0 if (m.get('fell_back') and m.get('provider_used')!='tavily') else 1)" 2>/dev/null; then
    echo "  ✅ fell_back=True, primary not used"; PASS=$((PASS+1))
else
    echo "  ❌ fallback did not engage"; FAIL=$((FAIL+1))
fi

# ─── Test 4: Meta tagging (via local SearXNG) ───────────────────
echo "--- Test 4: _meta.provider_used tagged ---"
r=$(bash "$ORC" "meta tag" broad 3 2>/dev/null)
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get('_meta',{}).get('provider_used') else 1)" 2>/dev/null; then
    echo "  ✅ result tagged with _meta.provider_used"; PASS=$((PASS+1))
else
    echo "  ⚠️  no _meta tag (SearXNG local down?)"; WARN=$((WARN+1))
fi

# ─── Test 5: Verify mode always emits valid JSON ────────────────
echo "--- Test 5: verify mode graceful on empty Tavily ---"
T=$(stub_providers "search_tavily")
r=$(bash "$T" "verify empty" verify 3 2>/dev/null); rm -f "$T"
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); v=d.get('_meta',{}).get('verification',{}); sys.exit(0 if v and 'verified' in v else 1)" 2>/dev/null; then
    echo "  ✅ verify emits valid JSON + verification meta"; PASS=$((PASS+1))
else
    echo "  ❌ verify produced invalid/empty output"; FAIL=$((FAIL+1))
fi

# ─── Test 6: Verify mode (live) ─────────────────────────────────
echo "--- Test 6: verify mode (live Tavily × SearXNG) ---"
r=$(bash "$ORC" "verify live" verify 3 2>/dev/null)
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get('_meta',{}).get('verification') else 1)" 2>/dev/null; then
    echo "  ✅ verify returns verification meta"; PASS=$((PASS+1))
else
    echo "  ⚠️  verify live failed (quota/rate-limit?)"; WARN=$((WARN+1))
fi

# ─── Test 7: Live factual (Tavily) ──────────────────────────────
echo "--- Test 7: Live factual (Tavily) ---"
r=$(bash "$ORC" "live factual" factual 3 2>/dev/null)
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if 'error' not in d else 1)" 2>/dev/null; then
    echo "  ✅ Tavily factual works"; PASS=$((PASS+1))
else
    echo "  ⚠️  Tavily failed (rate-limit?)"; WARN=$((WARN+1))
fi

# ─── Test 8: Live content (Firecrawl) ───────────────────────────
echo "--- Test 8: Live content (Firecrawl) ---"
r=$(bash "$ORC" "live content" content 3 2>/dev/null)
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if ('error' not in d) else 1)" 2>/dev/null; then
    echo "  ✅ Firecrawl content works"; PASS=$((PASS+1))
else
    echo "  ⚠️  Firecrawl failed (rate-limit?)"; WARN=$((WARN+1))
fi

# ─── Test 9: Live dynamic (TinyFish) ────────────────────────────
echo "--- Test 9: Live dynamic (TinyFish) ---"
r=$(bash "$ORC" "live dynamic" dynamic 3 2>/dev/null)
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if 'error' not in d else 1)" 2>/dev/null; then
    echo "  ✅ TinyFish dynamic works"; PASS=$((PASS+1))
else
    echo "  ⚠️  TinyFish failed (rate-limit?)"; WARN=$((WARN+1))
fi

# ─── Test 10: Deep Research (parallel) ──────────────────────────
echo "--- Test 10: Deep Research (parallel) ---"
r=$(bash "$ORC" "deep" deep_research 3 2>/dev/null)
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); p=d.get('providers',{}); sys.exit(0 if len(p)>=3 else 1)" 2>/dev/null; then
    echo "  ✅ Deep Research works (>=3 providers)"; PASS=$((PASS+1))
else
    echo "  ⚠️  Deep Research partial/failed"; WARN=$((WARN+1))
fi

# ─── Test 11: Invalid type rejected ─────────────────────────────
echo "--- Test 11: Invalid type rejected ---"
if ! bash "$ORC" "x" invalid_type 2>/dev/null; then
    echo "  ✅ invalid type rejected"; PASS=$((PASS+1))
else
    echo "  ❌ invalid type not rejected"; FAIL=$((FAIL+1))
fi

# ─── Summary ────────────────────────────────────────────────────
echo ""
# ─── New feature tests (process.py deterministic + live) ────────
LIB="$SCRIPTS_DIR/lib/process.py"

# Test 12: merge + dedup across providers
echo "--- Test 12: process.py merge/dedup (deterministic) ---"
SAMPLE_MERGE='{"query":"q","type":"deep_research","providers":{"tavily":{"status":"ok","data":{"results":[{"title":"A","url":"https://x.com/a","content":"2024 release"}]}},"firecrawl":{"status":"ok","data":{"data":[{"url":"https://x.com/a","title":"A","content":"2024 release v2"}]}}}}'
if echo "$SAMPLE_MERGE" | python3 "$LIB" merge | python3 -c "import sys,json; d=json.load(sys.stdin); r=d['results']; assert len(r)==1, 'dedup'; assert r[0]['provider_count']==2, 'pc'; assert 'freshness_score' in r[0]['_meta'], 'fr'; print('ok')" 2>/dev/null; then
    echo "  ✅ merge dedups + provider_count + freshness"; PASS=$((PASS+1))
else
    echo "  ❌ merge/dedup broken"; FAIL=$((FAIL+1))
fi

# Test 13: decomposition
echo "--- Test 13: process.py decompose (deterministic) ---"
if echo '{"query":"react vs vue compared to svelte"}' | python3 "$LIB" decompose | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d['subqueries'])==3, d; print('ok')" 2>/dev/null; then
    echo "  ✅ decompose → 3 subqueries"; PASS=$((PASS+1))
else
    echo "  ❌ decompose broken"; FAIL=$((FAIL+1))
fi

# Test 14: freshness
echo "--- Test 14: process.py freshness (deterministic) ---"
if echo '{"results":[{"title":"x","url":"https://x.com","content":"published 2024-01-15"}]}' | python3 "$LIB" freshness | python3 -c "import sys,json; d=json.load(sys.stdin); f=d['results'][0]['_meta']['freshness_score']; assert 0<f<=1, f; print('ok')" 2>/dev/null; then
    echo "  ✅ freshness_score in (0,1]"; PASS=$((PASS+1))
else
    echo "  ❌ freshness broken"; FAIL=$((FAIL+1))
fi

# Test 15: cache HIT (live, free SearXNG)
echo "--- Test 15: cache hit on repeat ---"
Q="cache probe $(date +%s%N)"
bash "$ORC" "$Q" broad 3 >/dev/null 2>&1
r2=$(bash "$ORC" "$Q" broad 3 2>/dev/null)
if echo "$r2" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get('_meta',{}).get('cached') else 1)" 2>/dev/null; then
    echo "  ✅ repeat query served from cache"; PASS=$((PASS+1))
else
    echo "  ⚠️  cache miss (SearXNG local down?)"; WARN=$((WARN+1))
fi

# Test 16: adaptive stats recorded (live)
echo "--- Test 16: adaptive routing stats recorded ---"
bash "$ORC" "adaptive probe" broad 3 >/dev/null 2>&1
if [[ -f "$SCRIPTS_DIR/../config/.provider-stats.json" ]]; then
    echo "  ✅ provider-stats.json created"; PASS=$((PASS+1))
else
    echo "  ⚠️  stats not written"; WARN=$((WARN+1))
fi

# Test 17: deep_research decomposition (live)
echo "--- Test 17: deep_research decomposition (live) ---"
r=$(bash "$ORC" "compare postgres vs mysql for analytics" deep_research 3 2>/dev/null)
if echo "$r" | python3 -c "import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get('_meta',{}).get('decomposed') else 1)" 2>/dev/null; then
    echo "  ✅ deep_research decomposed compound query"; PASS=$((PASS+1))
else
    echo "  ⚠️  decomposition not triggered (live fail?)"; WARN=$((WARN+1))
fi

echo "=== Test Summary ==="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo "  Warnings: $WARN"
echo "  Total: $((PASS+FAIL+WARN))"
if [[ "$FAIL" -gt 0 ]]; then
    echo ""; echo "❌ HARD FAILURES PRESENT"; exit 1
else
    echo ""; echo "✅ NO HARD FAILURES (warnings = environmental/quota)"; exit 0
fi
