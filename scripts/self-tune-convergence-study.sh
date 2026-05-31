#!/usr/bin/env bash
#
# self-tune-convergence-study.sh — Multi-iteration Heimdall → hat
# self-tune loop. PR #965 closed the single-iteration loop; this
# orchestrates 5 iterations and captures per-iteration artifacts so we
# can answer the convergence question: does the loop converge, diverge,
# or oscillate?
#
# Each iteration:
#   1. Run a gauntlet with the prior iteration's feedback applied
#      (iteration 0 runs baseline, no feedback).
#   2. Emit a new feedback file from the gauntlet's analytics.
#   3. Snapshot result_N.json and feedback_N.json.
#
# After all iterations, scripts/self-tune-convergence-analyze.py
# computes per-iteration win-rate by commander, avg game length, and
# feedback-vector L2 norms; emits an ASCII trajectory plot and a
# convergence verdict.
#
# Usage:
#   scripts/self-tune-convergence-study.sh \
#     [--decks PATH] [--games N] [--seed N] [--seats N] [--iterations N] [--out DIR]
#
# Defaults:
#   --decks       data/decks/test          (16 decks, large enough pool)
#   --games       500
#   --seed        42
#   --seats       4
#   --iterations  5
#   --out         /tmp/heimdall-convergence

set -euo pipefail

DECKS=data/decks/test
GAMES=500
SEED=42
SEATS=4
ITERATIONS=5
OUT=/tmp/heimdall-convergence

while [[ $# -gt 0 ]]; do
  case "$1" in
    --decks) DECKS="$2"; shift 2 ;;
    --games) GAMES="$2"; shift 2 ;;
    --seed) SEED="$2"; shift 2 ;;
    --seats) SEATS="$2"; shift 2 ;;
    --iterations) ITERATIONS="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \?//'; exit 0 ;;
    *) echo "self-tune-convergence-study: unknown flag $1" >&2; exit 2 ;;
  esac
done

mkdir -p "$OUT"

echo "=== Heimdall → hat self-tune CONVERGENCE STUDY ($ITERATIONS iterations) ==="
echo "  decks: $DECKS"
echo "  games per iteration: $GAMES"
echo "  seed: $SEED  (held constant across iterations)"
echo "  seats: $SEATS"
echo "  output dir: $OUT"
echo

echo "→ Building hexdek-tournament..."
go build -o "$OUT/hexdek-tournament" ./cmd/hexdek-tournament

# Per-iteration feedback file path. Iteration 0 has none (baseline);
# iteration N+1 consumes iteration N's emitted feedback.
prev_feedback_arg=""

for ((i = 0; i < ITERATIONS; i++)); do
  echo
  echo "=== ITERATION $i ==="
  start_ts=$(date +%s)

  # Per-iteration output paths.
  iter_dir="$OUT/iter_$i"
  mkdir -p "$iter_dir"

  if [[ -n "$prev_feedback_arg" ]]; then
    echo "  consuming feedback from: $prev_feedback_arg"
    "$OUT/hexdek-tournament" \
        --decks "$DECKS" \
        --games "$GAMES" \
        --seed "$SEED" \
        --seats "$SEATS" \
        --apply-feedback "$prev_feedback_arg" \
        --feedback-out "$iter_dir" \
        --result-json "$iter_dir/result.json" \
        --report "$iter_dir/report.md" \
        2>&1 | tail -3
  else
    echo "  (baseline — no feedback applied)"
    "$OUT/hexdek-tournament" \
        --decks "$DECKS" \
        --games "$GAMES" \
        --seed "$SEED" \
        --seats "$SEATS" \
        --feedback-out "$iter_dir" \
        --result-json "$iter_dir/result.json" \
        --report "$iter_dir/report.md" \
        2>&1 | tail -3
  fi

  if [[ ! -f "$iter_dir/result.json" ]]; then
    echo "ERROR: iteration $i did not emit result.json" >&2
    exit 1
  fi
  if [[ ! -f "$iter_dir/heimdall_feedback.json" ]]; then
    echo "ERROR: iteration $i did not emit heimdall_feedback.json" >&2
    exit 1
  fi

  prev_feedback_arg="$iter_dir/heimdall_feedback.json"
  elapsed=$(( $(date +%s) - start_ts ))
  echo "  iteration $i complete in ${elapsed}s"
done

echo
echo "=== ALL $ITERATIONS ITERATIONS COMPLETE ==="
echo "  Artifacts: $OUT/iter_{0..$((ITERATIONS-1))}/{result.json,heimdall_feedback.json,report.md}"
echo
echo "→ Running analysis..."
python3 scripts/self-tune-convergence-analyze.py "$OUT" "$ITERATIONS"
