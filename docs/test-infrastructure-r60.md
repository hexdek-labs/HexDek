# Test Infrastructure (r60)

`scripts/run-tests.sh` is a flake-resilient wrapper around `go test -json`.
It exists so transient flaky tests don't false-fail the engine suite or
push devs toward `t.Skip(...)` shortcuts.

## Usage

```
scripts/run-tests.sh [--retries N] [--flake-out PATH] [--timing-out PATH] [PACKAGES...]
```

Defaults:
- `--retries 3`
- `--flake-out test-flakes.log`
- `--timing-out test-timing.log`
- Packages: `./...`

Run from the repo root before pushing:

```
scripts/run-tests.sh
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0`  | All tests passed (possibly with flakes — see flake-list) |
| `1`  | At least one test failed all retries (real failure) |
| `2`  | Build / compile error (no retries attempted) |

## How retries work

1. First pass: `go test -json -count=1 <PACKAGES>` runs the full suite,
   live-streaming a humanized view to stderr.
2. The JSON stream is parsed; every leaf test that recorded `Action=fail`
   is queued for retry.
3. Each failed leaf is re-run in isolation via
   `go test -json -count=1 -run "^TestName$" <PACKAGE>`, up to N times.
4. Classification:
   - **PASS**: passed first try.
   - **FLAKE**: failed initially, passed on retry. Logged with
     `retries_used` count (1-N).
   - **FAIL**: failed every retry. Treated as a real failure.

## Output files

### `test-flakes.log` (TSV)

```
package    test    retries_used    final_status
github.com/hexdek/hexdek/internal/tournament    TestUntapDrain_OmnathRetainsGreenOnly    1    FLAKE
github.com/hexdek/hexdek/internal/foo    TestSomethingBroken    3    FAIL
```

Empty (header only) when no tests failed initially. Header is always
written so downstream tooling can diff cleanly.

### `test-timing.log` (TSV)

```
package    test    duration_seconds
github.com/hexdek/hexdek/internal/gameengine    TestSlowFuzz    42.103
github.com/hexdek/hexdek/internal/hat    TestMidPath    3.512
...
```

Sorted descending by duration. Useful for spotting tests that have crept
above the suite's per-test budget.

## Recommended dev workflow

- Run `scripts/run-tests.sh` before pushing. Flakes won't block you;
  real failures will.
- If the flake-list grows, file an issue with the offending
  `(package, test)` pair. Don't `t.Skip` — that hides the signal.
- For a fast iteration loop on a single package:
  `scripts/run-tests.sh ./internal/gameengine/...`

## Known flaky tests this targets

These motivated the runner; see the originating PRs for context.

| Test | Package | Surfaced in |
|------|---------|-------------|
| `TestUntapDrain_OmnathRetainsGreenOnly` | `internal/tournament/mana_pool_untap_drain_r60_test.go` | PR #816 review |
| `TestFindStale_*` | `internal/moxfield/imports_r60_test.go` | PR #796 |

Fixing the underlying flakes is tracked separately. This runner is
infrastructure to make their flakes non-blocking in the meantime.

## Dependencies

- `bash` (3.2+ compatible — macOS ships this)
- `go`
- `python3` (stdlib only — no `jq`, no third-party packages)

The Python helper at `scripts/run-tests-parse.py` does all JSON parsing.
