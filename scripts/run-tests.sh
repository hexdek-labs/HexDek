#!/usr/bin/env bash
# scripts/run-tests.sh — flake-resilient test runner for HexDek.
#
# Wraps `go test -json` so transient flakes (e.g.
# TestUntapDrain_OmnathRetainsGreenOnly, TestFindStale_*) don't false-fail
# a clean engine suite. Each failed leaf test is retried up to N times in
# isolation; tests that eventually pass are classified as FLAKE rather
# than FAIL.
#
# See docs/test-infrastructure-r60.md for the full rationale and dev
# workflow recommendations.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." &> /dev/null && pwd)"
PARSER="${SCRIPT_DIR}/run-tests-parse.py"

RETRIES=3
FLAKE_OUT="test-flakes.log"
TIMING_OUT="test-timing.log"
PACKAGES=()

usage() {
    cat <<'EOF'
Usage: scripts/run-tests.sh [--retries N] [--flake-out PATH] [--timing-out PATH] [PACKAGES...]

Runs `go test -json` against PACKAGES (default ./...), retrying any failed
leaf test up to N times (default 3) to distinguish FLAKE from real FAIL.

Outputs:
  --flake-out PATH    TSV: package, test, retries_used, final_status (default test-flakes.log)
  --timing-out PATH   TSV: package, test, duration_seconds (sorted desc) (default test-timing.log)

Exit codes:
  0  all tests passed (possibly with flakes)
  1  at least one test failed all retries (real failure)
  2  build / compile error

See docs/test-infrastructure-r60.md for details.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --retries)      RETRIES="$2"; shift 2 ;;
        --flake-out)    FLAKE_OUT="$2"; shift 2 ;;
        --timing-out)   TIMING_OUT="$2"; shift 2 ;;
        -h|--help)      usage; exit 0 ;;
        --)             shift; PACKAGES+=("$@"); break ;;
        -*)             echo "unknown flag: $1" >&2; usage >&2; exit 2 ;;
        *)              PACKAGES+=("$1"); shift ;;
    esac
done

if [[ ${#PACKAGES[@]} -eq 0 ]]; then
    PACKAGES=("./...")
fi

[[ -f "${PARSER}" ]] || { echo "missing parser: ${PARSER}" >&2; exit 2; }

TMPDIR_RUN="$(mktemp -d "${TMPDIR:-/tmp}/run-tests.XXXXXX")"
trap 'rm -rf "${TMPDIR_RUN}"' EXIT

FAILS_FILE="${TMPDIR_RUN}/fails.tsv"
TIMING_FILE="${TMPDIR_RUN}/timing.tsv"
SUMMARY_FILE="${TMPDIR_RUN}/summary.txt"
FIRST_PASS_JSON="${TMPDIR_RUN}/first-pass.json"

echo "[run-tests] first pass: go test -json -count=1 ${PACKAGES[*]}" >&2

# First pass: capture JSON to a file (for parsing) AND stream readable
# output to stderr live. We strip JSON to plain "go test"-style lines via
# Python so devs see progress without scrolling through {Action:...} noise.
set +e
go test -json -count=1 "${PACKAGES[@]}" > "${FIRST_PASS_JSON}" 2>"${TMPDIR_RUN}/first-pass.stderr"
FIRST_EXIT=$?
set -e

# Print stderr from go (build errors, etc.) so devs see them.
if [[ -s "${TMPDIR_RUN}/first-pass.stderr" ]]; then
    cat "${TMPDIR_RUN}/first-pass.stderr" >&2
fi

# Stream a humanized view of the JSON to stderr.
python3 - "${FIRST_PASS_JSON}" >&2 <<'PYEOF'
import json, sys
path = sys.argv[1]
with open(path) as f:
    for line in f:
        line = line.rstrip("\n")
        if not line:
            continue
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            continue
        action = ev.get("Action", "")
        if action == "output":
            sys.stderr.write(ev.get("Output", ""))
        elif action in ("fail", "pass", "skip") and ev.get("Test"):
            tag = action.upper()
            elapsed = ev.get("Elapsed", 0.0)
            sys.stderr.write(f"--- {tag}: {ev['Test']} ({elapsed:.2f}s)  [{ev.get('Package','')}]\n")
PYEOF

# Parse first-pass results.
python3 "${PARSER}" \
    --fails-out "${FAILS_FILE}" \
    --timing-out "${TIMING_FILE}" \
    < "${FIRST_PASS_JSON}" \
    > "${SUMMARY_FILE}"

# SUMMARY <tests_run> <fail_count> <build_fail:0|1>
read -r _tag TESTS_RUN FAIL_COUNT BUILD_FAIL < "${SUMMARY_FILE}"

if [[ "${BUILD_FAIL}" == "1" ]]; then
    echo "[run-tests] build failure detected — aborting (exit 2)" >&2
    exit 2
fi

# Belt-and-suspenders: if go exited non-zero with no parseable failures
# AND no tests ran, treat as build error (e.g. package-load failure with
# no JSON events).
if [[ "${FIRST_EXIT}" -ne 0 && "${FAIL_COUNT}" -eq 0 && "${TESTS_RUN}" -eq 0 ]]; then
    echo "[run-tests] go test exited ${FIRST_EXIT} with no test events — treating as build error (exit 2)" >&2
    exit 2
fi

# Move timing log to final destination immediately (independent of retries).
mv "${TIMING_FILE}" "${TIMING_OUT}"

# Write flake-list header.
printf 'package\ttest\tretries_used\tfinal_status\n' > "${FLAKE_OUT}"

FLAKE_COUNT=0
HARD_FAIL_COUNT=0

if [[ "${FAIL_COUNT}" -gt 0 ]]; then
    echo "[run-tests] ${FAIL_COUNT} failed test(s); retrying each up to ${RETRIES} time(s)" >&2

    while IFS=$'\t' read -r PKG TEST; do
        [[ -z "${PKG}" || -z "${TEST}" ]] && continue
        RETRY_USED=0
        STATUS="FAIL"
        for ((i=1; i<=RETRIES; i++)); do
            echo "[run-tests] RETRY[${i}/${RETRIES}]: ${PKG} ${TEST}" >&2
            set +e
            RETRY_OUT="${TMPDIR_RUN}/retry-${i}-$$.json"
            go test -json -count=1 -run "^${TEST}$" "${PKG}" > "${RETRY_OUT}" 2>/dev/null
            RETRY_EXIT=$?
            set -e
            RETRY_USED=$i
            if [[ "${RETRY_EXIT}" -eq 0 ]]; then
                # Double-check via JSON: ensure the specific test recorded "pass".
                if python3 - "${RETRY_OUT}" "${TEST}" <<'PYEOF'
import json, sys
path, target = sys.argv[1], sys.argv[2]
with open(path) as f:
    for line in f:
        try:
            ev = json.loads(line)
        except Exception:
            continue
        if ev.get("Test") == target and ev.get("Action") == "pass":
            sys.exit(0)
sys.exit(1)
PYEOF
                then
                    STATUS="FLAKE"
                    break
                fi
            fi
        done

        if [[ "${STATUS}" == "FLAKE" ]]; then
            FLAKE_COUNT=$((FLAKE_COUNT + 1))
        else
            HARD_FAIL_COUNT=$((HARD_FAIL_COUNT + 1))
        fi
        printf '%s\t%s\t%d\t%s\n' "${PKG}" "${TEST}" "${RETRY_USED}" "${STATUS}" >> "${FLAKE_OUT}"
    done < "${FAILS_FILE}"
fi

PASS_COUNT=$((TESTS_RUN - FAIL_COUNT))

echo "" >&2
echo "==================== run-tests summary ====================" >&2
printf '  passed (first try): %d\n' "${PASS_COUNT}" >&2
printf '  flaked (passed on retry): %d\n' "${FLAKE_COUNT}" >&2
printf '  failed (all retries): %d\n' "${HARD_FAIL_COUNT}" >&2
printf '  flake-list: %s\n' "${FLAKE_OUT}" >&2
printf '  timing log: %s\n' "${TIMING_OUT}" >&2
echo "===========================================================" >&2

if [[ "${HARD_FAIL_COUNT}" -gt 0 ]]; then
    exit 1
fi
exit 0
