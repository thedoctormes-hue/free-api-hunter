#!/bin/sh
# searxng-alert.sh — wrapper for searxng-monitor.service (systemd timer, 30 min).
#
# Runs bin/searxng-health.sh (quorum-aware results>0 probe) and, based on the
# marker emitted by health.sh, notifies via the lab's gk_notify.py mechanism:
#   OK    -> silence (no Telegram, no alert-log entry; collapse-to-green).
#   WARN  -> Telegram WARNING, dedup TTL 3600s (at most one per hour while degraded).
#   ALERT -> Telegram CRITICAL, short TTL 600s (re-alert each run while down).
#
# Alert chain (verified):
#   * systemd searxng-monitor.timer (30 min) -> searxng-monitor.service (oneshot)
#     -> THIS script -> health.sh -> (on non-OK) gk_notify.py -> Telegram (ЗавЛаб).
#   * gateway-cron raven-searxng-hourly-check (1 h) is an INDEPENDENT AI agentTurn
#     that does its own read-only curl checks and returns text to Telegram. It does
#     NOT call these scripts, so there is no script-level duplicate Telegram alert.
#
# This wrapper ALWAYS exits 0 after processing, so the oneshot service / timer
# never "falls" and never double-reports a failure as its own error.
set -u

BIN_DIR="/root/LabDoctorM/projects/free-api-hunter/bin"
HEALTH="$BIN_DIR/searxng-health.sh"
GK_NOTIFY="/root/LabDoctorM/projects/mcp-tools/mcp-gatekeeper/audit/gk_notify.py"
ALERT_LOG="/var/log/searxng-alert.log"
SEARXNG_URL="${SEARXNG_URL:-http://localhost:8889/search}"

TS="$(date '+%Y-%m-%d %H:%M:%S %z')"

# Run health check, capture combined output + exit code.
OUT="$($HEALTH 2>&1)"
RC=$?

if [ "$RC" -eq 0 ]; then
    # Healthy: keep journal clean (debug only), no Telegram, no alert-log entry.
    logger -t searxng-monitor -p daemon.debug "OK: SearXNG health passed"
    echo "$OUT"
    exit 0
fi

# ---- Non-OK path: classify by the marker health.sh emitted. ----
# health.sh emits exactly one of: OK quorum / WARN quorum / ALERT quorum / ALERT general=0
# (per-engine DEGRADED lines never contain the WARN/ALERT keywords).
LEVEL="ALERT"
case "$OUT" in
    *WARN*)  LEVEL="WARN" ;;
    *ALERT*) LEVEL="ALERT" ;;
    *)       LEVEL="ALERT" ;;   # unknown non-zero (script error) -> treat critical
esac

if [ "$LEVEL" = "WARN" ]; then
    PREFIX="SearXNG WARN (search-gateway quorum degraded)"
    TTL=3600
    LPRI="daemon.warning"
else
    PREFIX="SearXNG ALERT (search-gateway quorum)"
    TTL=600
    LPRI="daemon.crit"
fi

# Stable text (no per-run timestamp) so gk_notify.py dedup (TTL) works as designed.
TG_MSG="${PREFIX}
${OUT}"

# 1) Telegram via lab mechanism (best-effort; never fails the caller).
if [ -f "$GK_NOTIFY" ]; then
    GK_NOTIFY_TTL="$TTL" GK_TG_ACCOUNT="${GK_TG_ACCOUNT:-raven}" python3 "$GK_NOTIFY" "$TG_MSG" || true
fi

# 2) Always record to journald + alert log (audit trail / fallback if TG unavailable).
logger -t searxng-monitor -p "$LPRI" "${LEVEL} @ ${TS}: ${OUT}"
{
    echo "[$TS] ${LEVEL}"
    echo "$OUT"
    echo "----"
} >> "$ALERT_LOG" 2>/dev/null || true

echo "$OUT"
exit 0
