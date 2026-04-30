#!/usr/bin/env bash
# muninn-daily-report.sh — Daily Muninn telemetry report to Discord
# Runs via crontab on DARKSTAR. Posts to HexDek Official #system-status.
#
# Setup:
#   1. Set MUNINN_DISCORD_WEBHOOK in environment or .env
#   2. crontab -e → 17 8 * * * /home/josh/hexdek/muninn-daily-report.sh
#
set -euo pipefail

HEXDEK_DIR="${HEXDEK_DIR:-/home/josh/hexdek}"
MUNINN_BIN="${HEXDEK_DIR}/hexdek-muninn"
WEBHOOK_URL="${MUNINN_DISCORD_WEBHOOK:?Set MUNINN_DISCORD_WEBHOOK}"

DATE=$(date +%Y-%m-%d)

if [ ! -x "$MUNINN_BIN" ]; then
    echo "muninn binary not found at $MUNINN_BIN" >&2
    exit 1
fi

RAW=$(cd "$HEXDEK_DIR" && "$MUNINN_BIN" --all --top 5 2>&1) || true

GAPS=$(echo "$RAW" | grep -oP 'PARSER GAPS \(top \d+ by frequency, \K\d+' 2>/dev/null || echo "0")
if echo "$RAW" | grep -q "PARSER GAPS: (none)"; then GAPS="0"; fi

CRASHES=$(echo "$RAW" | grep -oP 'RECURRING CRASHES \(top \d+ by recency, \K\d+' 2>/dev/null || echo "0")
if echo "$RAW" | grep -q "RECURRING CRASHES: (none)"; then CRASHES="0"; fi

TRIGGERS=$(echo "$RAW" | grep -oP 'DEAD TRIGGERS \(top \d+ by frequency, \K\d+' 2>/dev/null || echo "0")
if echo "$RAW" | grep -q "DEAD TRIGGERS: (none)"; then TRIGGERS="0"; fi

if [ "$GAPS" = "0" ] && [ "$CRASHES" = "0" ] && [ "$TRIGGERS" = "0" ]; then
    STATUS="CLEAN — no parser gaps, no crashes, no dead triggers"
else
    PARTS=()
    [ "$GAPS" != "0" ] && PARTS+=("${GAPS} parser gaps")
    [ "$CRASHES" != "0" ] && PARTS+=("${CRASHES} crashes")
    [ "$TRIGGERS" != "0" ] && PARTS+=("${TRIGGERS} dead triggers")
    STATUS=$(IFS=', '; echo "${PARTS[*]}")
fi

TRIGGER_DETAIL=""
if [ "$TRIGGERS" != "0" ]; then
    TRIGGER_DETAIL=$(echo "$RAW" | sed -n '/DEAD TRIGGERS/,/^$/p' | grep '^\s*[0-9]' | head -5 | while read -r line; do
        card=$(echo "$line" | grep -oP 'card="\K[^"]+')
        count=$(echo "$line" | grep -oP 'count=\K[0-9]+')
        games=$(echo "$line" | grep -oP 'games=\K[0-9]+')
        echo "  ${card} — ${count} fires / ${games} games"
    done)
fi

GAP_DETAIL=""
if [ "$GAPS" != "0" ]; then
    GAP_DETAIL=$(echo "$RAW" | sed -n '/PARSER GAPS/,/^$/p' | grep '^\s*[0-9]' | head -5 | while read -r line; do
        snippet=$(echo "$line" | grep -oP '"\K[^"]+' | head -1)
        count=$(echo "$line" | grep -oP 'count=\K[0-9]+')
        echo "  \"${snippet}\" — ${count}x"
    done)
fi

REPORT="\`\`\`"
REPORT+=$'\n'"══════════════════════════════════════════"
REPORT+=$'\n'"  MUNINN DAILY REPORT — ${DATE}"
REPORT+=$'\n'"══════════════════════════════════════════"
REPORT+=$'\n'
REPORT+=$'\n'"  PARSER GAPS ............ ${GAPS}"
REPORT+=$'\n'"  RECURRING CRASHES ...... ${CRASHES}"
REPORT+=$'\n'"  DEAD TRIGGERS .......... ${TRIGGERS}"

if [ -n "$TRIGGER_DETAIL" ]; then
    REPORT+=$'\n'
    REPORT+=$'\n'"  TOP DEAD TRIGGERS:"
    REPORT+=$'\n'"${TRIGGER_DETAIL}"
fi

if [ -n "$GAP_DETAIL" ]; then
    REPORT+=$'\n'
    REPORT+=$'\n'"  TOP PARSER GAPS:"
    REPORT+=$'\n'"${GAP_DETAIL}"
fi

REPORT+=$'\n'
REPORT+=$'\n'"  STATUS: ${STATUS}"
REPORT+=$'\n'"══════════════════════════════════════════"
REPORT+=$'\n'"\`\`\`"

PAYLOAD=$(jq -n --arg content "$REPORT" '{content: $content, username: "Muninn", avatar_url: "https://hexdek.dev/favicon.svg"}')

curl -s -H "Content-Type: application/json" -d "$PAYLOAD" "$WEBHOOK_URL"
