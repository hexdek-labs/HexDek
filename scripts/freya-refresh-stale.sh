#!/usr/bin/env bash
#
# freya-refresh-stale.sh — re-analyze decks whose stored analysis was
# produced by an older Freya than the one installed now.
#
# WHY THIS EXISTS
#
# A deck's analysis is a file on disk. It is written when the deck is
# imported, edited, cloned, or explicitly refreshed — and at no other
# time. When Freya's logic changes, every existing file keeps the old
# conclusions until something rewrites it.
#
# The deck page now shows a "this analysis is out of date — Refresh"
# banner, which fixes this for decks somebody visits. It does nothing
# for the rest, and the rest is most of them. This script is the other
# half: after a deploy that changes Freya's output, run it once and the
# whole corpus catches up.
#
# It is deliberately a manual, serial, off-peak tool rather than
# something the server does on its own. Re-analysis on page view was
# considered and rejected: runFreya is serialized behind a single
# process-wide mutex with an unbounded queue, so a version bump — which
# marks every deck stale at once — would put the entire corpus behind
# that one lock the moment traffic arrived.
#
# COST MODEL (measured 2026-09-15, not estimated)
#
#   one deck, fresh process   18.7s
#   four decks, one process   20.6s
#
# Almost all of that is loading the 212 MB oracle corpus, once per
# process. Per-deck analysis is roughly half a second. So the naive
# "loop over decks, invoke Freya each time" shape costs ~18s per deck
# and would take about NINE HOURS on the current 1,707-deck corpus,
# while Freya's own --all-decks mode loads the corpus once and does the
# same work in about fifteen minutes.
#
# This script therefore batches through --all-decks by owner directory
# and only falls back to per-deck invocation for --limit runs, where
# the point is a quick canary rather than throughput.
#
# USAGE
#
#   scripts/freya-refresh-stale.sh [--decks DIR] [--dry-run] [--all]
#                                  [--limit N] [--jobs N]
#
#   --decks DIR   deck root (default: data/decks)
#   --dry-run     list what would be re-analyzed, change nothing
#   --all         re-analyze every deck, not just the stale ones
#   --limit N     stop after N decks (useful for a canary run)
#   --jobs N      parallel Freya processes (default 1; see NOTE below)
#
# NOTE ON --jobs: Freya loads the oracle corpus per process, so each
# job carries a real memory cost. The default is 1 because this is
# meant to run unattended off-peak, where finishing slowly is free and
# competing with the live server for RAM is not.

set -uo pipefail

DECKS_DIR="data/decks"
DRY_RUN=0
FORCE_ALL=0
LIMIT=0
JOBS=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --decks)   DECKS_DIR="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --all)     FORCE_ALL=1; shift ;;
    --limit)   LIMIT="$2"; shift 2 ;;
    --jobs)    JOBS="$2"; shift 2 ;;
    -h|--help) sed -n '2,56p' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

# Locate the Freya binary the same way the server does: PATH first,
# then the working directory.
FREYA="$(command -v hexdek-freya || true)"
if [[ -z "$FREYA" && -x "./hexdek-freya" ]]; then
  FREYA="./hexdek-freya"
fi
if [[ -z "$FREYA" ]]; then
  echo "error: hexdek-freya not found on PATH or in $(pwd)" >&2
  echo "       build it first: go build -o hexdek-freya ./cmd/hexdek-freya/" >&2
  exit 1
fi

if [[ ! -d "$DECKS_DIR" ]]; then
  echo "error: deck directory not found: $DECKS_DIR" >&2
  exit 1
fi

CURRENT_VERSION="$("$FREYA" --version 2>/dev/null | head -1 | awk '{print $NF}')"
if [[ -z "$CURRENT_VERSION" ]]; then
  echo "error: could not read a version from '$FREYA --version'" >&2
  exit 1
fi

echo "freya binary   : $FREYA"
echo "current version: $CURRENT_VERSION"
echo "deck root      : $DECKS_DIR"
[[ "$DRY_RUN" -eq 1 ]] && echo "mode           : DRY RUN (nothing will be written)"
[[ "$FORCE_ALL" -eq 1 ]] && echo "mode           : ALL decks (staleness check skipped)"
echo

# storedVersion <deck-path> — echoes the freya_version recorded in this
# deck's strategy.json, or nothing when there is no analysis or no
# version stamp. A file with no stamp predates version tracking and is
# therefore older than any current build, so it counts as stale.
storedVersion() {
  local deck="$1"
  local dir base strat
  dir="$(dirname "$deck")"
  base="$(basename "$deck")"
  base="${base%.*}"
  strat="$dir/freya/$base.strategy.json"
  [[ -f "$strat" ]] || return 0
  # Deliberately not `jq` — this runs on deploy hosts where jq may not
  # be installed. python3 ships with both our server platforms.
  python3 -c '
import json,sys
try:
    with open(sys.argv[1]) as fh:
        print(json.load(fh).get("freya_version") or "")
except Exception:
    print("")
' "$strat" 2>/dev/null
}

mapfile -t ALL_DECKS < <(find "$DECKS_DIR" -mindepth 2 -maxdepth 2 \
  \( -name '*.txt' -o -name '*.json' \) \
  -not -path '*/freya/*' -not -path '*/.versions/*' | sort)

STALE=()
for deck in "${ALL_DECKS[@]}"; do
  if [[ "$FORCE_ALL" -eq 1 ]]; then
    STALE+=("$deck")
  else
    v="$(storedVersion "$deck")"
    if [[ "$v" != "$CURRENT_VERSION" ]]; then
      STALE+=("$deck")
    fi
  fi
  if [[ "$LIMIT" -gt 0 && "${#STALE[@]}" -ge "$LIMIT" ]]; then
    break
  fi
done

echo "decks found    : ${#ALL_DECKS[@]}"
echo "to re-analyze  : ${#STALE[@]}"
echo

if [[ "${#STALE[@]}" -eq 0 ]]; then
  echo "Nothing to do — every analysis is current."
  exit 0
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
  printf '%s\n' "${STALE[@]}"
  exit 0
fi

OK=0
FAILED=0
FAILED_LIST=()
START="$(date +%s)"

# Owner directories containing at least one stale deck. --all-decks
# operates on a directory, and re-running a deck that was already
# current is nearly free (it hits the report cache), so batching by
# owner trades a little redundant work for a ~40x saving on process
# startup.
stale_owner_dirs() {
  local d
  for d in "${STALE[@]}"; do
    dirname "$d"
  done | sort -u
}

analyze_one() {
  # --no-cache is essential here. Without it a deck whose CONTENTS are
  # unchanged hits the report cache and this script rewrites the same
  # stale conclusions while reporting success — the exact failure this
  # whole change set exists to remove.
  "$FREYA" --deck "$1" --format json --no-cache >/dev/null 2>&1
}

# Batch mode is the default and the fast path. --limit runs stay
# per-deck: they exist to sanity-check a handful of decks quickly, and
# batching a directory to re-analyze 2 decks would defeat that.
if [[ "$LIMIT" -eq 0 && "$JOBS" -eq 1 ]]; then
  mapfile -t OWNER_DIRS < <(stale_owner_dirs)
  echo "batching ${#STALE[@]} stale deck(s) across ${#OWNER_DIRS[@]} owner director(ies)"
  echo "(corpus loads once per directory rather than once per deck)"
  echo
  di=0
  for dir in "${OWNER_DIRS[@]}"; do
    di=$((di + 1))
    printf '[%d/%d] %s ... ' "$di" "${#OWNER_DIRS[@]}" "$dir"
    # --no-cache even in batch mode. If Freya's CODE changed but its
    # version token did not — which is exactly what happens when the
    # binary was built somewhere the Go toolchain stamps no VCS
    # revision — a cached run would rewrite the same stale conclusions
    # and this script would report success. The corpus load dominates
    # the runtime either way (~18s fixed vs ~0.5s per deck), so
    # recomputing costs almost nothing and removes the one way this
    # tool could lie.
    if "$FREYA" --all-decks "$dir" --format json --no-cache >/dev/null 2>&1; then
      echo "done"
    else
      echo "FREYA EXITED NONZERO"
    fi
  done
  echo
  # Outcome is derived per deck from what landed on disk, never from
  # Freya's exit code — a run can exit 0 having written nothing useful,
  # and the stamp is what consumers actually read.
  #
  # This check is not a formality. On the first full run it caught 23
  # decks the batch had silently skipped, because Freya's --all-decks
  # walker (listDeckFiles, cmd/hexdek-freya/main.go:1684) hard-skips the
  # `benched` and `test` directories and only globs *.txt — so .json
  # decks are invisible to it. Exit code was 0 throughout. Without
  # verifying the artifact this script would have reported a clean sweep
  # over a corpus with 23 stale analyses still in it.
  MISSED=()
  for deck in "${STALE[@]}"; do
    if [[ "$(storedVersion "$deck")" == "$CURRENT_VERSION" ]]; then
      OK=$((OK + 1))
    else
      MISSED+=("$deck")
    fi
  done

  # Second pass: anything the batch walker didn't reach gets a direct
  # per-deck invocation, which has no such filter. Slow per deck (~18s
  # of corpus load each) but the set is small by construction — it is
  # only the decks --all-decks structurally cannot see.
  if [[ "${#MISSED[@]}" -gt 0 ]]; then
    echo "batch walker skipped ${#MISSED[@]} deck(s) — retrying those individually"
    mi=0
    for deck in "${MISSED[@]}"; do
      mi=$((mi + 1))
      printf '  [%d/%d] %s ... ' "$mi" "${#MISSED[@]}" "$(basename "$deck")"
      analyze_one "$deck"
      if [[ "$(storedVersion "$deck")" == "$CURRENT_VERSION" ]]; then
        echo "ok"
        OK=$((OK + 1))
      else
        echo "STILL NOT STAMPED"
        FAILED=$((FAILED + 1))
        FAILED_LIST+=("$deck")
      fi
    done
    echo
  fi
elif [[ "$JOBS" -gt 1 ]]; then
  export -f analyze_one
  export FREYA
  printf '%s\0' "${STALE[@]}" \
    | xargs -0 -P "$JOBS" -I{} bash -c 'analyze_one "$@"' _ {}
  # xargs gives one aggregate status, so re-derive per-deck outcomes
  # from what actually landed on disk rather than trusting the count.
  for deck in "${STALE[@]}"; do
    if [[ "$(storedVersion "$deck")" == "$CURRENT_VERSION" ]]; then
      OK=$((OK + 1))
    else
      FAILED=$((FAILED + 1))
      FAILED_LIST+=("$deck")
    fi
  done
else
  i=0
  for deck in "${STALE[@]}"; do
    i=$((i + 1))
    printf '[%d/%d] %s ... ' "$i" "${#STALE[@]}" "$(basename "$deck")"
    if analyze_one "$deck"; then
      # Trust the artifact, not the exit code: Freya can exit 0 having
      # written nothing useful, and the stamp is the thing consumers
      # actually read.
      if [[ "$(storedVersion "$deck")" == "$CURRENT_VERSION" ]]; then
        echo "ok"
        OK=$((OK + 1))
      else
        echo "RAN BUT DID NOT STAMP"
        FAILED=$((FAILED + 1))
        FAILED_LIST+=("$deck")
      fi
    else
      echo "FAILED"
      FAILED=$((FAILED + 1))
      FAILED_LIST+=("$deck")
    fi
  done
fi

ELAPSED=$(( $(date +%s) - START ))
echo
echo "re-analyzed : $OK"
echo "failed      : $FAILED"
echo "elapsed     : ${ELAPSED}s"

if [[ "$FAILED" -gt 0 ]]; then
  echo
  echo "Decks that did not end up stamped with $CURRENT_VERSION:"
  printf '  %s\n' "${FAILED_LIST[@]}"
  exit 1
fi
