#!/usr/bin/env bash
# scripts/run-forensics_test.sh — deterministic smoke test for run-forensics.sh.
#
# Exercises the --use-stored-replay path so the test doesn't depend on
# Loki runtime (~minute per game count) and stays reproducible across
# machines. Verifies:
#
#   1. With the SHIPPED EMPTY baseline + the multi-cluster fixture
#      (3 distinct prov/first/match keys) → script EXITS 1 with new-cluster
#      diagnostics in stderr.
#   2. After --update-baseline against the multi-cluster fixture, re-running
#      the same fixture → EXITS 0.
#   3. With the just-rebuilt baseline still in place, swapping in the
#      synthetic-fabrication fixture (1 cluster — a SUBSET of the multi
#      fixture's 3) → still EXITS 0 (subset is fine; the rule is "no NEW
#      keys"). Defends against over-strict equality checks.
#   4. The reverse direction — baseline=synthetic-only, current=multi —
#      EXITS 1 because multi adds keys synthetic didn't have.
#
# Test isolation: the script saves + restores the real baseline file so
# CI doesn't drift after this test runs.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." &> /dev/null && pwd)"
SCRIPT="${SCRIPT_DIR}/run-forensics.sh"
BASELINE="${SCRIPT_DIR}/forensics-baseline.txt"
FIXTURE_DIR="${REPO_ROOT}/cmd/hexdek-forensics/testdata"
MULTI="${FIXTURE_DIR}/replay-multi-cluster.json"
SYNTHETIC="${FIXTURE_DIR}/replay-synthetic-fabrication.json"

PASS=0
FAIL=0

pass() { printf '  PASS: %s\n' "$*"; PASS=$((PASS + 1)); }
fail() { printf '  FAIL: %s\n' "$*"; FAIL=$((FAIL + 1)); }
note() { printf '[test] %s\n' "$*"; }

[[ -x "${SCRIPT}" ]] || { echo "FAIL: ${SCRIPT} not executable"; exit 1; }
[[ -f "${MULTI}" ]]   || { echo "FAIL: fixture missing: ${MULTI}";   exit 1; }
[[ -f "${SYNTHETIC}" ]] || { echo "FAIL: fixture missing: ${SYNTHETIC}"; exit 1; }

# Save + restore the real baseline so CI re-runs aren't poisoned.
BASELINE_BACKUP="$(mktemp "${TMPDIR:-/tmp}/forensics-baseline-backup.XXXXXX")"
cp "${BASELINE}" "${BASELINE_BACKUP}"
trap 'cp "${BASELINE_BACKUP}" "${BASELINE}"; rm -f "${BASELINE_BACKUP}"' EXIT

run_script() {
    # Captures both stdout+stderr; returns the exit code via $?.
    local out; local rc
    out="$("${SCRIPT}" "$@" 2>&1)" && rc=0 || rc=$?
    LAST_OUTPUT="${out}"
    LAST_RC="${rc}"
}

# --- Test 1: empty baseline + multi-cluster fixture → must EXIT 1 -----
note "test 1: empty baseline + multi-cluster fixture (expect EXIT 1)"
run_script --use-stored-replay "${MULTI}" || true
if [[ "${LAST_RC}" -eq 1 ]] && grep -q 'new mint-bypass cluster surfaced' <<<"${LAST_OUTPUT}"; then
    pass "rejected the new-cluster set against empty baseline"
else
    fail "expected EXIT 1 with new-cluster diagnostic, got EXIT ${LAST_RC}"
    printf '%s\n' "${LAST_OUTPUT}" | sed 's/^/    > /'
fi

# --- Test 2: --update-baseline against multi → must EXIT 0 ------------
note "test 2: --update-baseline against multi (expect EXIT 0)"
run_script --use-stored-replay "${MULTI}" --update-baseline
if [[ "${LAST_RC}" -eq 0 ]] && [[ $(grep -cE '^prov=' "${BASELINE}") -eq 3 ]]; then
    pass "baseline regenerated with 3 cluster keys"
else
    fail "expected EXIT 0 + 3 baseline keys, got EXIT ${LAST_RC} + $(grep -cE '^prov=' "${BASELINE}") keys"
    printf '%s\n' "${LAST_OUTPUT}" | sed 's/^/    > /'
fi

# --- Test 3: rebuilt baseline + same fixture → must EXIT 0 ------------
note "test 3: rebuilt baseline + same multi fixture (expect EXIT 0)"
run_script --use-stored-replay "${MULTI}"
if [[ "${LAST_RC}" -eq 0 ]] && grep -q 'no new cluster keys' <<<"${LAST_OUTPUT}"; then
    pass "clean run against matching baseline"
else
    fail "expected EXIT 0 with clean message, got EXIT ${LAST_RC}"
    printf '%s\n' "${LAST_OUTPUT}" | sed 's/^/    > /'
fi

# --- Test 4: rebuilt baseline (3 keys) + synthetic subset (1 key) → 0 -
note "test 4: baseline=multi, current=synthetic-subset (expect EXIT 0)"
run_script --use-stored-replay "${SYNTHETIC}"
if [[ "${LAST_RC}" -eq 0 ]]; then
    pass "subset accepted (current keys ⊆ baseline keys)"
else
    fail "expected EXIT 0 for subset, got EXIT ${LAST_RC}"
    printf '%s\n' "${LAST_OUTPUT}" | sed 's/^/    > /'
fi

# --- Test 5: reverse — baseline=synthetic, current=multi → must EXIT 1 -
note "test 5: rebuild baseline from synthetic, run against multi (expect EXIT 1)"
run_script --use-stored-replay "${SYNTHETIC}" --update-baseline
if [[ "${LAST_RC}" -ne 0 ]]; then
    fail "test 5 setup: rebuilding baseline from synthetic failed (EXIT ${LAST_RC})"
    printf '%s\n' "${LAST_OUTPUT}" | sed 's/^/    > /'
else
    run_script --use-stored-replay "${MULTI}" || true
    if [[ "${LAST_RC}" -eq 1 ]] && grep -q 'new mint-bypass cluster surfaced' <<<"${LAST_OUTPUT}"; then
        pass "rejected the multi-cluster superset against synthetic baseline"
    else
        fail "expected EXIT 1 with new-cluster diagnostic, got EXIT ${LAST_RC}"
        printf '%s\n' "${LAST_OUTPUT}" | sed 's/^/    > /'
    fi
fi

# --- Summary ----------------------------------------------------------
printf '\n[test] %d PASS / %d FAIL\n' "${PASS}" "${FAIL}"
if [[ "${FAIL}" -eq 0 ]]; then
    printf '[test] OVERALL: PASS\n'
    exit 0
fi
printf '[test] OVERALL: FAIL\n' >&2
exit 1
