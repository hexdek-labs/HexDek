#!/usr/bin/env python3
# scripts/run-tests-parse.py — JSON parser helper for run-tests.sh.
#
# Reads `go test -json` event stream on stdin and emits two TSV files:
#   --fails-out PATH : one row per failing leaf test (package<TAB>test)
#   --timing-out PATH: one row per test (package<TAB>test<TAB>elapsed) sorted desc
#
# Also prints a summary line to stdout:
#   SUMMARY <tests_run> <fail_count> <build_fail:0|1>
#
# A "leaf" test is one that has no children (we drop parent rows when a
# subtest with the same prefix exists). Parents still get retried fine via
# -run "^Name$", but for the flake list we want the actual failing subtest.

import argparse
import json
import sys


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--fails-out", required=True)
    ap.add_argument("--timing-out", required=True)
    args = ap.parse_args()

    # (pkg, test) -> elapsed seconds (last pass/fail event wins)
    timing: dict[tuple[str, str], float] = {}
    # (pkg, test) -> "pass" | "fail" | "skip"
    status: dict[tuple[str, str], str] = {}
    build_fail = False
    tests_run = 0

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            # go test -json sometimes leaks non-JSON output on stderr-merge;
            # ignore non-JSON lines defensively.
            continue

        action = ev.get("Action", "")
        pkg = ev.get("Package", "")
        test = ev.get("Test", "")

        if action == "build-fail":
            build_fail = True
            continue

        # Package-level events have no Test field; skip for per-test tracking.
        if not test:
            continue

        if action == "run":
            tests_run += 1
        elif action in ("pass", "fail", "skip"):
            status[(pkg, test)] = action
            elapsed = ev.get("Elapsed", 0.0)
            try:
                timing[(pkg, test)] = float(elapsed)
            except (TypeError, ValueError):
                timing[(pkg, test)] = 0.0

    # Identify leaf tests: a test is a leaf if no other test name starts with
    # "<test>/". We only emit leaves to the fails file so retry targets the
    # actual failing subtest, not its parent.
    all_test_names_by_pkg: dict[str, set[str]] = {}
    for (pkg, test) in status.keys():
        all_test_names_by_pkg.setdefault(pkg, set()).add(test)

    def is_leaf(pkg: str, test: str) -> bool:
        prefix = test + "/"
        for other in all_test_names_by_pkg.get(pkg, ()):
            if other.startswith(prefix):
                return False
        return True

    fails = [(p, t) for (p, t), s in status.items() if s == "fail" and is_leaf(p, t)]
    fails.sort()

    with open(args.fails_out, "w") as f:
        for pkg, test in fails:
            f.write(f"{pkg}\t{test}\n")

    # Timing: emit all tests (leaves and parents both useful for slow-test
    # diagnosis), sorted by elapsed descending.
    rows = sorted(
        ((pkg, test, dur) for (pkg, test), dur in timing.items()),
        key=lambda r: r[2],
        reverse=True,
    )
    with open(args.timing_out, "w") as f:
        for pkg, test, dur in rows:
            f.write(f"{pkg}\t{test}\t{dur:.3f}\n")

    # Summary line on stdout. Count is leaf-only to match retry universe.
    print(f"SUMMARY {tests_run} {len(fails)} {1 if build_fail else 0}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
