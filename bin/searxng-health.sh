#!/bin/sh
# SearXNG health-check: проверяет не HTTP 200, а что probe-запрос
# возвращает >0 результатов. HTTP 200 при CAPTCHA отдаёт пустой results —
# это тихий отказ, который ловит только проверка results>0.
#
# Проверяет general search + каждый premium пул (exa/tavily/firecrawl/tinyfish/olostep).
set -u
URL="${SEARXNG_URL:-http://localhost:8889/search}"
FAILED=0

count_results() {
  curl -s --max-time 20 "$1" 2>/dev/null | python3 -c "import sys,json
try:
    d=json.load(sys.stdin); print(len(d.get('results',[])))
except Exception:
    print(0)" 2>/dev/null
}

check_general() {
  c=$(count_results "${URL}?q=OpenClaw&format=json&categories=general")
  if [ "${c:-0}" -gt 0 ]; then
    echo "OK general ($c)"
  else
    echo "DEGRADED general ($c)"
    FAILED=$((FAILED + 1))
  fi
}

check_engine() {
  e="$1"
  c=$(count_results "${URL}?q=OpenClaw&format=json&engines=$e")
  if [ "${c:-0}" -gt 0 ]; then
    echo "OK $e ($c)"
  else
    echo "DEGRADED $e ($c)"
    FAILED=$((FAILED + 1))
  fi
}

check_general
for e in exa tavily firecrawl tinyfish olostep; do
  check_engine "$e"
done

if [ "$FAILED" -eq 0 ]; then
  exit 0
else
  exit 1
fi
