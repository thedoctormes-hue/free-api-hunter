#!/bin/sh
# searxng-health.sh — quorum-aware SearXNG health check.
#
# Goal: detect not just "is there any light" (general > 0) but "do I see with
# ALL my general eyes". A single working engine (e.g. yandex) can keep
# general > 0 while 4 of 5 engines are dead -> the old check stayed green but
# Raven was 80% blind. This script probes every general-category engine
# individually (results > 0) and computes the live quorum.
#
# Emits machine-parseable markers (alert.sh parses these):
#   OK quorum N/T                 -> all N general eyes alive        (exit 0)
#   WARN quorum N/T degraded: X   -> exactly ONE eye degraded        (exit 1)
#   ALERT quorum N/T              -> TWO OR MORE eyes degraded       (exit 1)
#   ALERT general=0               -> bare general search is blind    (exit 1)
#
# Source of the general-engine list: the LIVE SearXNG /config endpoint. This is
# the resolved form of searxng/settings.yml. A static parse of settings.yml is
# NOT reliable: engines without an explicit `categories:` field inherit their
# engine-module built-in default category (e.g. github -> repos), so a naive
# "no categories => general" assumption would wrongly include dozens of
# non-general engines. /config reflects the true runtime categories.
#
# wikipedia is excluded from the quorum (QUORUM_EXCLUDE): it is a general-category
# encyclopedic reference engine that returns 0 results for arbitrary queries like
# OpenClaw, so counting it would produce constant false degradation.
#
# Per-engine exa/tinyfish/olostep probes are kept as free-tier informational
# signals (they do NOT affect the general quorum).
set -u

URL="${SEARXNG_URL:-http://localhost:8889/search}"
# Engines excluded from the quorum (encyclopedic reference engines that never
# answer arbitrary web queries). Comma-separated; empty disables exclusion.
QUORUM_EXCLUDE="${QUORUM_EXCLUDE:-wikipedia}"

count_results() {
  # $1 = full URL. Prints number of results (0 on any failure).
  curl -s --max-time 20 "$1" 2>/dev/null | python3 -c "import sys,json
try:
    d=json.load(sys.stdin); print(len(d.get('results',[])))
except Exception:
    print(0)" 2>/dev/null
}

# Probe one general engine; retry once on 0 to absorb transient blips.
probe_engine() {
  e="$1"
  c=$(count_results "${URL}?q=OpenClaw&format=json&engines=$e")
  if [ "${c:-0}" -gt 0 ]; then
    echo "$c"; return
  fi
  sleep 2
  c=$(count_results "${URL}?q=OpenClaw&format=json&engines=$e")
  echo "${c:-0}"
}

discover_general_engines() {
  # Returns space-separated list of enabled general engines (excl. QUORUM_EXCLUDE)
  # from the live /config endpoint. Empty on failure (SearXNG down / no config).
  QUORUM_EXCLUDE="$QUORUM_EXCLUDE" python3 - "$URL" <<'PY'
import sys, os, json, urllib.request
base = sys.argv[1].rsplit('/search', 1)[0]
exclude = set(x for x in (os.environ.get('QUORUM_EXCLUDE') or '').split(',') if x)
try:
    with urllib.request.urlopen(base + '/config', timeout=20) as r:
        d = json.load(r)
except Exception:
    sys.exit(0)
out = []
for e in d.get('engines', []):
    if not e.get('enabled', True):
        continue
    if 'general' not in e.get('categories', []):
        continue
    n = e.get('name', '')
    if n in exclude:
        continue
    out.append(n)
print(' '.join(out))
PY
}

# ---- Build the general-engine list ----
# GENERAL_ENGINES env override (comma-separated) replaces discovery. Used by
# simulations (Step 4) to inject fake engine lists without touching prod.
if [ -n "${GENERAL_ENGINES:-}" ]; then
  GENERAL_LIST=$(echo "$GENERAL_ENGINES" | tr ',' ' ')
else
  GENERAL_LIST=$(discover_general_engines)
fi

# ---- Per-engine quorum probe ----
alive=0
total=0
degraded_list=""
for e in $GENERAL_LIST; do
  total=$((total + 1))
  c=$(probe_engine "$e")
  if [ "${c:-0}" -gt 0 ]; then
    echo "OK $e ($c)"
    alive=$((alive + 1))
  else
    echo "DEGRADED $e ($c)"
    if [ -z "$degraded_list" ]; then
      degraded_list="$e"
    else
      degraded_list="$degraded_list,$e"
    fi
  fi
done

# ---- Control check: bare general search (no engines) must return > 0 ----
# This preserves the original behaviour: full blindness (general == 0) is always
# an ALERT regardless of the per-engine quorum maths.
gc=$(count_results "${URL}?q=OpenClaw&format=json")
if [ "${gc:-0}" -gt 0 ]; then
  echo "OK general (all: $gc)"
else
  echo "ALERT general=0"
fi

# ---- Free-tier per-engine signals (informational, NON-quorum) ----
for e in exa tinyfish olostep; do
  c=$(count_results "${URL}?q=OpenClaw&format=json&engines=$e")
  if [ "${c:-0}" -gt 0 ]; then
    echo "OK $e ($c)"
  else
    echo "DEGRADED $e ($c)"
  fi
done

# ---- Quorum decision ----
if [ "${gc:-0}" -eq 0 ]; then
  # Full blindness: general search returns nothing.
  if [ "$total" -gt 0 ]; then
    echo "ALERT quorum ${alive}/${total}"
  else
    echo "ALERT quorum 0/0 (general blind, no engines discovered)"
  fi
  exit 1
fi

if [ "$total" -eq 0 ]; then
  # Could not discover engines (SearXNG up but /config unavailable). General
  # control check passed, so we do not false-alert; just report no quorum data.
  echo "OK general (all: $gc) [quorum: no engines discovered]"
  exit 0
fi

if [ "$alive" -eq "$total" ]; then
  echo "OK quorum ${alive}/${total}"
  exit 0
fi

down=$((total - alive))
if [ "$down" -eq 1 ]; then
  # Exactly one eye degraded: degraded but not blind. WARN (non-critical).
  echo "WARN quorum ${alive}/${total} degraded: $degraded_list"
  exit 1
fi

# Two or more eyes degraded: materially blind. ALERT (critical).
echo "ALERT quorum ${alive}/${total}"
exit 1
