#!/usr/bin/env bash
#
# self-tune-loop.sh — Heimdall → hat self-tune workflow (single iteration).
#
# Architecture (see docs/heimdall-hat-self-tune-r60.md for the long-form
# write-up):
#
#   gauntlet A  ──Heimdall.ComputeWeightDeltas──>  HeimdallFeedback JSON
#                                                          │
#                                                          ▼
#                                              hat.SetActiveFeedback
#                                                          │
#                                                          ▼
#   gauntlet B  ──> result_b.json ── diff vs ── result_a.json
#
# If gauntlet B's win-rate distribution or avg-turns shifted from
# gauntlet A, the closed loop is wired end-to-end and the attribution
# model has signal. If they're identical (within noise), the loop wires
# but the attribution model is either still stubbed (PR #955 rebasing)
# or producing zero-delta feedback — the cure is on the attribution
# side, not the integration side.
#
# Usage:
#   scripts/self-tune-loop.sh [--decks PATH] [--games N] [--seed N] [--seats N] [--out DIR]
#
# Defaults:
#   --decks   data/decks/cage_match
#   --games   500
#   --seed    42
#   --seats   4
#   --out     /tmp/heimdall-self-tune
#
# Single-iteration only; convergence study is a follow-up.

set -euo pipefail

DECKS=data/decks/cage_match
GAMES=500
SEED=42
SEATS=4
OUT=/tmp/heimdall-self-tune

while [[ $# -gt 0 ]]; do
  case "$1" in
    --decks) DECKS="$2"; shift 2 ;;
    --games) GAMES="$2"; shift 2 ;;
    --seed) SEED="$2"; shift 2 ;;
    --seats) SEATS="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    -h|--help)
      grep '^#' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *) echo "self-tune-loop: unknown flag $1" >&2; exit 2 ;;
  esac
done

mkdir -p "$OUT"

echo "=== Heimdall → hat self-tune (single iteration) ==="
echo "  decks: $DECKS"
echo "  games per gauntlet: $GAMES"
echo "  seed: $SEED"
echo "  seats: $SEATS"
echo "  output dir: $OUT"
echo

echo "→ Building hexdek-tournament..."
go build -o "$OUT/hexdek-tournament" ./cmd/hexdek-tournament

# -----------------------------------------------------------------------------
# Gauntlet A — baseline run, no feedback applied, emit feedback for B.
# -----------------------------------------------------------------------------
echo
echo "=== GAUNTLET A (baseline) ==="
"$OUT/hexdek-tournament" \
    --decks "$DECKS" \
    --games "$GAMES" \
    --seed "$SEED" \
    --seats "$SEATS" \
    --feedback-out "$OUT" \
    --result-json "$OUT/result_a.json" \
    --report "$OUT/report_a.md"

if [[ ! -f "$OUT/heimdall_feedback.json" ]]; then
  echo "ERROR: gauntlet A did not emit heimdall_feedback.json" >&2
  exit 1
fi
if [[ ! -f "$OUT/result_a.json" ]]; then
  echo "ERROR: gauntlet A did not emit result_a.json" >&2
  exit 1
fi

# -----------------------------------------------------------------------------
# Gauntlet B — same deck pool, same seed, with feedback applied.
# -----------------------------------------------------------------------------
echo
echo "=== GAUNTLET B (feedback applied) ==="
"$OUT/hexdek-tournament" \
    --decks "$DECKS" \
    --games "$GAMES" \
    --seed "$SEED" \
    --seats "$SEATS" \
    --apply-feedback "$OUT/heimdall_feedback.json" \
    --result-json "$OUT/result_b.json" \
    --report "$OUT/report_b.md"

if [[ ! -f "$OUT/result_b.json" ]]; then
  echo "ERROR: gauntlet B did not emit result_b.json" >&2
  exit 1
fi

# -----------------------------------------------------------------------------
# Compare. Lightweight summary: avg_turns delta + per-commander winrate
# delta. The full distribution diff is a follow-up; this is enough to
# answer "did anything shift at all?"
# -----------------------------------------------------------------------------
echo
echo "=== LOOP STATUS ==="
python3 - "$OUT/result_a.json" "$OUT/result_b.json" "$OUT/heimdall_feedback.json" <<'PY'
import json, sys
a, b, fb_path = sys.argv[1], sys.argv[2], sys.argv[3]
A = json.load(open(a))
B = json.load(open(b))
FB = json.load(open(fb_path))

print(f"  games: {A['games']} (A) → {B['games']} (B)")
print(f"  draws: {A['draws']} (A) → {B['draws']} (B)")
print(f"  avg turns: {A['avg_turns']:.2f} (A) → {B['avg_turns']:.2f} (B)  Δ={B['avg_turns']-A['avg_turns']:+.2f}")
print()
print("  Per-commander wins (A → B):")
all_cmdrs = sorted(set(A.get('wins_by_commander', {}).keys()) | set(B.get('wins_by_commander', {}).keys()))
total_shift = 0
for c in all_cmdrs:
    wa = A.get('wins_by_commander', {}).get(c, 0)
    wb = B.get('wins_by_commander', {}).get(c, 0)
    delta = wb - wa
    total_shift += abs(delta)
    marker = ' '
    if delta > 0: marker = '↑'
    elif delta < 0: marker = '↓'
    print(f"    {marker} {c:40s}  {wa:4d} → {wb:4d}  (Δ {delta:+d})")
print()
print(f"  Total |Δ wins| across commanders: {total_shift}")
print()
print(f"  Feedback metadata:")
print(f"    schema_version: {FB.get('schema_version')}")
print(f"    provenance:     {FB.get('provenance')}")
print(f"    gauntlet_games: {FB.get('gauntlet_games')}")
print(f"    archetypes:     {len(FB.get('archetypes', []))} with deltas")
print()
if not FB.get('archetypes'):
    print("  VERDICT: feedback file empty (attribution model is still STUBBED).")
    print("           Apply path no-ops on empty feedback — gauntlet B == gauntlet A is EXPECTED.")
    print("           Closed-loop INTEGRATION verified; ATTRIBUTION needs PR #955 to land.")
elif total_shift == 0 and abs(B['avg_turns']-A['avg_turns']) < 0.01:
    print("  VERDICT: feedback computed but produced no observable shift.")
    print("           Either the deltas were too small to bite (cap at MaxDimensionDelta=2.0,")
    print("           floor at MinSampleSize=50 per archetype) or the gauntlet is too small.")
elif total_shift > 0 or abs(B['avg_turns']-A['avg_turns']) >= 0.01:
    print("  VERDICT: gauntlet B differs from A — the closed loop has BITE.")
    print("           Attribution model produced non-trivial deltas; hat consumed them;")
    print("           gameplay outcomes shifted. Single iteration complete.")
PY

echo
echo "Artifacts written to $OUT:"
ls -la "$OUT" | tail -n +2
