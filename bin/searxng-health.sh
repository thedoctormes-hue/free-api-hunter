#!/bin/sh
# SearXNG health-check: проверяет не HTTP 200, а что probe-запрос
# возвращает >0 результатов. HTTP 200 при CAPTCHA отдаёт пустой results —
# это тихий отказ, который ловит только проверка results>0.
#
# Основной чек general идёт через engines=yandex; доп. сигналы — exa/tinyfish/olostep.
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
  # Основной чек: general через конкретный рабочий движок (engines=yandex,
  # без API-ключа), чтобы не маскировался ответами wikipedia/wikidata.
  c=$(count_results "${URL}?q=OpenClaw&format=json&engines=yandex")
  if [ "${c:-0}" -gt 0 ]; then
    echo "OK general (yandex: $c)"
  else
    echo "DEGRADED general (yandex: $c)"
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
# Доп. сигналы free-тира: exa/tinyfish/olostep (каждый в отдельном probe).
for e in exa tinyfish olostep; do
  check_engine "$e"
done

if [ "$FAILED" -eq 0 ]; then
  exit 0
else
  exit 1
fi
