#!/usr/bin/env bash
# scripts/run-tests_test.sh — verifies scripts/run-tests.sh against 4
# synthetic Go packages (pass / fail / flake / compile-err).
#
# Style mirrors run-forensics_test.sh: PASS/FAIL counters + trapped tmpdir.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
SCRIPT="${SCRIPT_DIR}/run-tests.sh"

[[ -x "${SCRIPT}" ]] || { echo "FAIL: ${SCRIPT} not executable"; exit 1; }
command -v go >/dev/null 2>&1 || { echo "FAIL: go not on PATH"; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "FAIL: python3 not on PATH"; exit 1; }

PASS=0
FAIL=0
pass() { printf '  PASS: %s\n' "$*"; PASS=$((PASS + 1)); }
fail() { printf '  FAIL: %s\n' "$*"; FAIL=$((FAIL + 1)); }
note() { printf '[test] %s\n' "$*"; }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/run-tests-test.XXXXXX")"
trap 'rm -rf "${WORK}"' EXIT

# Bootstrap a synthetic Go module with four packages.
mkdir -p "${WORK}/pkg_pass" "${WORK}/pkg_fail" "${WORK}/pkg_flake" "${WORK}/pkg_compile_err"
cat > "${WORK}/go.mod" <<'EOF'
module runtests_synth

go 1.21
EOF

cat > "${WORK}/pkg_pass/pkg_pass_test.go" <<'EOF'
package pkg_pass

import "testing"

func TestAlwaysPass(t *testing.T) {}
EOF

cat > "${WORK}/pkg_fail/pkg_fail_test.go" <<'EOF'
package pkg_fail

import "testing"

func TestAlwaysFail(t *testing.T) {
    t.Fatal("intentional permanent failure")
}
EOF

# Flake: read+increment counter file in the test's own package dir.
# First invocation creates the file with "1" → fails. Subsequent calls
# see existing file → pass. The runner's retry naturally hits the
# subsequent-call branch. Counter lives under the package directory so
# it's isolated per test-run (each top-level `run-tests.sh` invocation
# uses a fresh WORK dir).
cat > "${WORK}/pkg_flake/pkg_flake_test.go" <<'EOF'
package pkg_flake

import (
    "os"
    "path/filepath"
    "runtime"
    "testing"
)

func TestFlakesOnce(t *testing.T) {
    _, thisFile, _, _ := runtime.Caller(0)
    counterPath := filepath.Join(filepath.Dir(thisFile), ".flake_counter")
    if _, err := os.Stat(counterPath); os.IsNotExist(err) {
        if werr := os.WriteFile(counterPath, []byte("1"), 0o644); werr != nil {
            t.Fatalf("could not write counter: %v", werr)
        }
        t.Fatal("flaking on first invocation (counter primed)")
    }
}
EOF

cat > "${WORK}/pkg_compile_err/pkg_compile_err.go" <<'EOF'
package pkg_compile_err

// This file references an undefined symbol so the package does not compile.
var _ = undefinedSymbol
EOF
cat > "${WORK}/pkg_compile_err/pkg_compile_err_test.go" <<'EOF'
package pkg_compile_err

import "testing"

func TestNeverRuns(t *testing.T) {}
EOF

run_case() {
    # run_case <subdir> <pkg-pattern> -> echoes exit code
    local subdir="$1"
    local pkg="$2"
    pushd "${WORK}/${subdir}" >/dev/null 2>&1 || { pushd "${WORK}" >/dev/null; }
    local rc=0
    set +e
    "${SCRIPT}" --retries 3 \
        --flake-out "${WORK}/flake-${subdir}.log" \
        --timing-out "${WORK}/timing-${subdir}.log" \
        "${pkg}" >"${WORK}/stdout-${subdir}.log" 2>"${WORK}/stderr-${subdir}.log"
    rc=$?
    set -e
    popd >/dev/null
    echo "${rc}"
}

# --- Case 1: pkg_pass --------------------------------------------------------
note "case 1: pkg_pass (expect exit 0, no FAIL/FLAKE entries)"
pushd "${WORK}" >/dev/null
set +e
"${SCRIPT}" --retries 3 \
    --flake-out "${WORK}/flake-pass.log" \
    --timing-out "${WORK}/timing-pass.log" \
    ./pkg_pass >"${WORK}/stdout-pass.log" 2>"${WORK}/stderr-pass.log"
RC=$?
set -e
popd >/dev/null
if [[ ${RC} -eq 0 ]]; then pass "pkg_pass exited 0"; else fail "pkg_pass exited ${RC} (want 0)"; fi
# flake log should have header line only (no data rows).
ROWS=$(($(wc -l < "${WORK}/flake-pass.log") - 1))
if [[ ${ROWS} -eq 0 ]]; then pass "pkg_pass flake-list has no data rows"; else fail "pkg_pass flake-list has ${ROWS} rows (want 0)"; fi

# --- Case 2: pkg_fail --------------------------------------------------------
note "case 2: pkg_fail (expect exit 1, FAIL entry in flake-list)"
pushd "${WORK}" >/dev/null
set +e
"${SCRIPT}" --retries 2 \
    --flake-out "${WORK}/flake-fail.log" \
    --timing-out "${WORK}/timing-fail.log" \
    ./pkg_fail >"${WORK}/stdout-fail.log" 2>"${WORK}/stderr-fail.log"
RC=$?
set -e
popd >/dev/null
if [[ ${RC} -eq 1 ]]; then pass "pkg_fail exited 1"; else fail "pkg_fail exited ${RC} (want 1)"; fi
if grep -q $'\tFAIL$' "${WORK}/flake-fail.log"; then
    pass "pkg_fail flake-list has FAIL entry"
else
    fail "pkg_fail flake-list missing FAIL entry; contents:"
    cat "${WORK}/flake-fail.log" >&2
fi

# --- Case 3: pkg_flake -------------------------------------------------------
note "case 3: pkg_flake (expect exit 0, FLAKE entry with retries_used >= 1)"
pushd "${WORK}" >/dev/null
# Clear counter so first invocation is the priming "fail" pass.
rm -f "${WORK}/pkg_flake/.flake_counter"
set +e
"${SCRIPT}" --retries 3 \
    --flake-out "${WORK}/flake-flake.log" \
    --timing-out "${WORK}/timing-flake.log" \
    ./pkg_flake >"${WORK}/stdout-flake.log" 2>"${WORK}/stderr-flake.log"
RC=$?
set -e
popd >/dev/null
if [[ ${RC} -eq 0 ]]; then pass "pkg_flake exited 0"; else fail "pkg_flake exited ${RC} (want 0); stderr:"; cat "${WORK}/stderr-flake.log" >&2; fi
if grep -q $'\tFLAKE$' "${WORK}/flake-flake.log"; then
    pass "pkg_flake flake-list has FLAKE entry"
    # Verify retries_used column is >= 1.
    RETRIES_USED=$(awk -F'\t' '$4 == "FLAKE" { print $3 }' "${WORK}/flake-flake.log" | head -1)
    if [[ -n "${RETRIES_USED}" && "${RETRIES_USED}" -ge 1 ]]; then
        pass "pkg_flake retries_used=${RETRIES_USED} (>=1)"
    else
        fail "pkg_flake retries_used='${RETRIES_USED}' (want >=1)"
    fi
else
    fail "pkg_flake flake-list missing FLAKE entry; contents:"
    cat "${WORK}/flake-flake.log" >&2
fi

# --- Case 4: pkg_compile_err -------------------------------------------------
note "case 4: pkg_compile_err (expect exit 2)"
pushd "${WORK}" >/dev/null
set +e
"${SCRIPT}" --retries 3 \
    --flake-out "${WORK}/flake-cerr.log" \
    --timing-out "${WORK}/timing-cerr.log" \
    ./pkg_compile_err >"${WORK}/stdout-cerr.log" 2>"${WORK}/stderr-cerr.log"
RC=$?
set -e
popd >/dev/null
if [[ ${RC} -eq 2 ]]; then pass "pkg_compile_err exited 2"; else fail "pkg_compile_err exited ${RC} (want 2); stderr:"; cat "${WORK}/stderr-cerr.log" >&2; fi

# --- Case 5: timing log populated + sorted desc ------------------------------
note "case 5: timing log populated + sorted descending"
# Use a small multi-test mix (pkg_pass + pkg_flake post-priming so both pass cleanly).
pushd "${WORK}" >/dev/null
set +e
"${SCRIPT}" --retries 3 \
    --flake-out "${WORK}/flake-timing.log" \
    --timing-out "${WORK}/timing-mix.log" \
    ./pkg_pass ./pkg_flake >"${WORK}/stdout-timing.log" 2>"${WORK}/stderr-timing.log"
set -e
popd >/dev/null
TIMING_ROWS=$(wc -l < "${WORK}/timing-mix.log" | tr -d ' ')
if [[ "${TIMING_ROWS}" -ge 1 ]]; then
    pass "timing log has ${TIMING_ROWS} row(s)"
else
    fail "timing log empty"
fi
# Sort check: extract column 3 and verify it's non-increasing.
SORTED=$(awk -F'\t' '{print $3}' "${WORK}/timing-mix.log")
PREV=""
SORT_OK=1
while IFS= read -r dur; do
    [[ -z "${dur}" ]] && continue
    if [[ -n "${PREV}" ]]; then
        # bash float compare via awk.
        if awk -v a="${PREV}" -v b="${dur}" 'BEGIN { exit !(a+0 >= b+0) }'; then
            :
        else
            SORT_OK=0
            break
        fi
    fi
    PREV="${dur}"
done <<< "${SORTED}"
if [[ ${SORT_OK} -eq 1 ]]; then
    pass "timing log sorted descending"
else
    fail "timing log not sorted descending"
fi

echo ""
echo "==================== run-tests_test summary ===================="
printf '  PASS: %d\n' "${PASS}"
printf '  FAIL: %d\n' "${FAIL}"
echo "================================================================"

if [[ ${FAIL} -gt 0 ]]; then
    exit 1
fi
exit 0
