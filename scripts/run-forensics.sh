#!/usr/bin/env bash
# scripts/run-forensics.sh — CI integration for the InstanceID
# mint-bypass forensics pipeline.
#
# Workflow:
#   1. Run hexdek-loki (small game count) to surface ZoneConservation
#      violations + emit a violations TSV.
#   2. Synthesize a Replay JSON from the TSV + a stub event log.
#      ##### TODO: when hexdek-loki ships `--replay-stream-out`
#      ##### (see cmd/hexdek-forensics/watch.go package comment), replace
#      ##### the synthesis block with a direct file read of Loki's output.
#   3. Run hexdek-forensics --replay <json> --patterns to cluster the
#      fabrications by mint-bypass code path.
#   4. Diff the resulting cluster keys against scripts/forensics-baseline.txt.
#      Any NEW PatternKey not in the baseline fails the run (exit 1).
#
# Baseline policy:
#   - The shipped baseline is the EMPTY SET — a clean 50-game / seed=42
#     Loki run on the current engine surfaces no fabrications (per the
#     2026-05-25 r60 canonical-final closure, see CLAUDE.md kanban),
#     so any new cluster appearing in CI is a regression signal.
#   - For local bootstrapping / development against the bundled
#     testdata fixtures, the SAME empty baseline catches both stored
#     fixtures as "new clusters" — exactly what the deterministic test
#     in run-forensics_test.sh exercises.
#   - When a cluster IS expected (e.g. an intentional new fabrication
#     surface during a refactor — extremely rare), regenerate with
#     `--update-baseline` AFTER reviewing the new entries by hand.
#
# Flags:
#   --update-baseline       overwrite scripts/forensics-baseline.txt with
#                           the current run's cluster keys
#   --games N               override Loki game count (default 50)
#   --seed N                override Loki seed (default 42)
#   --use-stored-replay P   skip Loki, analyze stored Replay JSON at P
#                           (used by run-forensics_test.sh for determinism)
#   --help                  print usage
#
# Exit codes:
#   0  clean — no new cluster keys vs baseline
#   1  one or more new cluster keys surfaced (or runtime error)
#   2  bad args

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." &> /dev/null && pwd)"
BASELINE_FILE="${SCRIPT_DIR}/forensics-baseline.txt"

GAMES=50
SEED=42
UPDATE_BASELINE=0
STORED_REPLAY=""

usage() {
    sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --update-baseline) UPDATE_BASELINE=1; shift ;;
        --games) GAMES="$2"; shift 2 ;;
        --seed) SEED="$2"; shift 2 ;;
        --use-stored-replay) STORED_REPLAY="$2"; shift 2 ;;
        --help|-h) usage; exit 0 ;;
        *) echo "run-forensics.sh: unknown flag: $1" >&2; usage >&2; exit 2 ;;
    esac
done

log() { printf '[run-forensics] %s\n' "$*"; }
die() { printf '[run-forensics] ERROR: %s\n' "$*" >&2; exit 1; }

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/hexdek-forensics-ci.XXXXXX")"
trap 'rm -rf "${WORK_DIR}"' EXIT

cd "${REPO_ROOT}"

# Ensure hexdek-forensics is built. Cheap on a warm cache.
FORENSICS_BIN="${WORK_DIR}/hexdek-forensics"
log "building hexdek-forensics → ${FORENSICS_BIN}"
go build -o "${FORENSICS_BIN}" ./cmd/hexdek-forensics/ || die "go build hexdek-forensics failed"

REPLAY_JSON=""

if [[ -n "${STORED_REPLAY}" ]]; then
    # --- Stored-replay path (deterministic / test mode) ---------------
    [[ -f "${STORED_REPLAY}" ]] || die "stored replay not found: ${STORED_REPLAY}"
    log "using stored replay (skipping Loki): ${STORED_REPLAY}"
    REPLAY_JSON="${STORED_REPLAY}"
else
    # --- Live Loki run --------------------------------------------------
    LOKI_REPORT="${WORK_DIR}/loki-report.md"
    LOKI_VIOLATIONS="${WORK_DIR}/loki-violations.tsv"
    log "running hexdek-loki --games ${GAMES} --seed ${SEED} (this may take a minute)"
    if ! go run ./cmd/hexdek-loki \
        --games "${GAMES}" \
        --seed "${SEED}" \
        --report "${LOKI_REPORT}" \
        --violations-dump "${LOKI_VIOLATIONS}" \
        > "${WORK_DIR}/loki-stdout.log" 2>&1; then
        log "Loki exited non-zero — tail of stdout:"
        tail -40 "${WORK_DIR}/loki-stdout.log" >&2 || true
        die "hexdek-loki failed; see ${WORK_DIR}/loki-stdout.log"
    fi

    # ##### TODO: replace this synthesis with a real --replay-stream-out
    # ##### read once hexdek-loki ships that flag (watch.go pkg comment).
    # ##### Today: synthesize a minimal Replay JSON from the violations
    # ##### TSV + an empty event log. Forensics output for the fabrication
    # ##### path will report <not_found> first-events (no event log to
    # ##### walk), but the PROVENANCE key still classifies correctly,
    # ##### so the baseline-diff signal remains useful — any net-new
    # ##### bypass surface shows up as a new (provenance, <not_found>,
    # ##### <none>) cluster row.
    REPLAY_JSON="${WORK_DIR}/synthesized-replay.json"
    log "synthesizing replay JSON from violations TSV → ${REPLAY_JSON}"
    SYNTH_TMP="${WORK_DIR}/violations.synth.json"
    if [[ -f "${LOKI_VIOLATIONS}" ]]; then
        # TSV columns: game-idx<TAB>turn<TAB>invariant<TAB>message
        awk -F '\t' '
            BEGIN { print "["; first = 1 }
            NF >= 4 {
                if (!first) printf ",\n"; first = 0
                # JSON-escape the message: backslash + double-quote.
                msg = $4
                gsub(/\\/, "\\\\", msg)
                gsub(/"/, "\\\"", msg)
                printf "  {\"turn\": %d, \"invariant\": \"%s\", \"message\": \"%s\"}",
                    $2 + 0, $3, msg
            }
            END { print "\n]" }
        ' "${LOKI_VIOLATIONS}" > "${SYNTH_TMP}"
    else
        printf '[]\n' > "${SYNTH_TMP}"
    fi
    # Wrap into the full Replay schema (events empty, card_index empty —
    # see TODO above; this is the lossy bit of the synthesis workaround).
    {
        printf '{\n'
        printf '  "game_idx": 0,\n'
        printf '  "seed": %d,\n' "${SEED}"
        printf '  "deck_seed": 0,\n'
        printf '  "commanders": [],\n'
        printf '  "events": [],\n'
        printf '  "violations": '
        cat "${SYNTH_TMP}"
        printf ',\n'
        printf '  "card_index": {}\n'
        printf '}\n'
    } > "${REPLAY_JSON}"
fi

# --- Forensics pattern analysis ---------------------------------------
PATTERNS_OUT="${WORK_DIR}/patterns.txt"
log "running hexdek-forensics --replay ${REPLAY_JSON} --patterns"
"${FORENSICS_BIN}" --replay "${REPLAY_JSON}" --patterns > "${PATTERNS_OUT}" || \
    die "hexdek-forensics failed; output:\n$(cat "${PATTERNS_OUT}" 2>/dev/null || true)"

# Extract one PatternKey per line. RenderPatternSummary emits rows like:
#   #1  hits=2  unique_ids=1  prov=OG first=enter_battlefield match=instance_id
# We grep the "prov=..." substring (stable / sortable / matches PatternKey.String()).
CURRENT_KEYS="${WORK_DIR}/current-keys.txt"
grep -oE 'prov=[^[:space:]]+ first=[^[:space:]]+ match=[^[:space:]]+' \
    "${PATTERNS_OUT}" \
    | LC_ALL=C sort -u > "${CURRENT_KEYS}" || true

log "current cluster keys ($(wc -l < "${CURRENT_KEYS}" | tr -d ' ')):"
if [[ -s "${CURRENT_KEYS}" ]]; then
    sed 's/^/  /' "${CURRENT_KEYS}"
else
    log "  (none — clean run)"
fi

# --- Baseline update path ---------------------------------------------
if [[ "${UPDATE_BASELINE}" -eq 1 ]]; then
    log "updating baseline → ${BASELINE_FILE}"
    {
        printf '# scripts/forensics-baseline.txt — known-good PatternKeys.\n'
        printf '# One PatternKey per line, format: prov=X first=Y match=Z.\n'
        printf '# Regenerate with: scripts/run-forensics.sh --update-baseline\n'
        printf '# Lines starting with "#" and blank lines are ignored.\n'
        cat "${CURRENT_KEYS}"
    } > "${BASELINE_FILE}"
    log "baseline updated with $(wc -l < "${CURRENT_KEYS}" | tr -d ' ') key(s)"
    exit 0
fi

# --- Baseline diff -----------------------------------------------------
[[ -f "${BASELINE_FILE}" ]] || die "baseline file missing: ${BASELINE_FILE} (run with --update-baseline to seed it)"

BASELINE_KEYS="${WORK_DIR}/baseline-keys.txt"
grep -vE '^\s*(#|$)' "${BASELINE_FILE}" | LC_ALL=C sort -u > "${BASELINE_KEYS}" || true

# Set diff: lines in CURRENT_KEYS not in BASELINE_KEYS = new clusters.
NEW_KEYS="${WORK_DIR}/new-keys.txt"
LC_ALL=C comm -23 "${CURRENT_KEYS}" "${BASELINE_KEYS}" > "${NEW_KEYS}" || true

if [[ -s "${NEW_KEYS}" ]]; then
    log "NEW cluster keys not in baseline:"
    sed 's/^/  + /' "${NEW_KEYS}" >&2
    log "full patterns output below:"
    sed 's/^/  | /' "${PATTERNS_OUT}" >&2
    die "new mint-bypass cluster surfaced — review the patterns table above, then either fix the bug or rerun with --update-baseline if intentional"
fi

log "no new cluster keys — clean against baseline ($(wc -l < "${BASELINE_KEYS}" | tr -d ' ') baseline key(s))"
exit 0
