# Dead-Test Cleanup — R60 Phase 2B

Sweep of `*_test.go` files across the repo for:

1. `t.Skip()` calls with no justification comment.
2. Tests with no assertions (no `t.Error*` / `t.Fatal*` / `require.*` / `assert.*` / `panic` / helper-delegation).
3. Commented-out test files.
4. `_test.go` files that test only deleted functions.

Each finding either revived with a justifying comment or deleted with a reason.

## 1. Unjustified `t.Skip()` — 0 findings

55 `t.Skip*` calls exist across the repo. **All 55 carry a string
explaining the reason** (fixture-missing, integration-mode gating,
env-flag gating, short-mode skip, etc.). No bare `t.Skip()` calls.

```bash
grep -rn 't\.Skip\b' --include='*_test.go' | wc -l
# 55  (all carry a justification string)
grep -rnE 't\.Skip\(\)|t\.SkipNow\(\)' --include='*_test.go'
# (empty)
```

**Action**: nothing to do.

## 2. No-assertion tests — 13 found, 13 actioned

Implemented a small Go scanner (`/tmp/noassert/main.go`, not checked in)
that uses `go/ast` to walk every `func Test*(t *testing.T)` and flags
any whose body contains none of:

- `t.Error` / `t.Errorf` / `t.Fatal` / `t.Fatalf` / `t.FailNow` / `t.Skip*`
- testify-style `require.*` / `assert.*` / `Equal` / `Nil` / `NoError` etc.
- `panic(...)`
- Custom helpers whose name starts with `assert` / `require` / `check` /
  `expect` / `verify` / `ensure` / `must` / `want` / `validate`
- **Helper-delegation**: any function call passing `t` as the first
  arg (catches custom helpers like `containsActionTitled(t, ...)` that
  call `t.Errorf` internally — without this filter the scanner
  produced 61 candidates, mostly false positives)

Final scan after improvements: **13 candidates**. All actioned.

### Pattern 1: "Doesn't panic" tests — 10 actioned

These tests verify a function accepts nil/edge inputs without
panicking. The contract is real but the assertion was implicit: the
test passed by reaching the closing brace. Converted each to an
explicit `defer recover()` block that fails the test cleanly if a
panic occurs.

| Test | File:Line | Action |
|---|---|---|
| `TestEnrichOpponentSeat_NilSafety` | `cmd/hexdek-thor/opponent_detect_test.go:410` | Added `defer recover()` |
| `TestResolveParadigmCopies_NilSafe` | `internal/gameengine/adventure_prepared_paradigm_test.go:238` | Added `defer recover()` |
| `TestCombatKeywords_NilSafety` | `internal/gameengine/keywords_combat_test.go:1348` | Added `defer recover()` |
| `TestClearVisitFlags_NilSafe` | `internal/gameengine/keywords_visit_test.go:192` | Added `defer recover()` |
| `TestR60_MarkSeatLost_NilSafety` | `internal/gameengine/sba_lost_resource_drain_r60_test.go:133` | Added `defer recover()` |
| `TestEmitDecisionEvent_NilGameStateIsNoop` | `internal/hat/decision_logging_test.go:111` | Added `defer recover()` |
| `TestObserver_NilSinks` | `internal/heimdall/observer_test.go:211` | Added `defer recover()` |
| `TestObserver_RecordSnapshot_NoSinkWiredIsNoOp` | `internal/heimdall/snapshot_sink_r60_test.go:149` | Added `defer recover()` |
| `TestRoomManager_TeardownRoom_Idempotent` | `internal/hexapi/spectate_rooms_test.go:110` | Added `defer recover()` — double-close-stopCh would have panicked here without an assertion |
| `TestAwardAchievementsNilTrackerIsNoOp` | `internal/tournament/achievements_test.go:57` | Added `defer recover()` |

The pattern:

```go
func TestX_NilSafe(t *testing.T) {
    defer func() {
        if r := recover(); r != nil {
            t.Fatalf("X(nil, ...) must not panic; got %v", r)
        }
    }()
    X(nil)
}
```

### Pattern 2: Inverted pin assertion — 1 actioned

`TestRoles_YawgmothsWill_CastFromGraveyardPattern` (`cmd/hexdek-freya/roles_recursion_r60_test.go:272`)
documented (in comments) a "known narrow detection that should broaden
later" but asserted nothing. Converted to an **inverted pin**: the
test now ASSERTS the narrow current behaviour (`!hasRole(...
RoleRecursion)`) so any future broadening of the detection logic
trips the test, forcing the maintainer to update it.

```go
// Before:
_ = roles  // (no assertion — narrow detection documented but not pinned)

// After:
if hasRole(roles, RoleRecursion) {
    t.Errorf("Yawgmoth's Will recursion detection broadened — update this test (got %v)", roles)
}
```

### Pattern 3: Compile-time interface checks — 2 actioned (kept, documented)

| Test | File:Line | Action |
|---|---|---|
| `TestMCTSHat_InterfaceSatisfaction` | `internal/hat/mcts_test.go:10` | Added a function-level doc comment explaining the compile-time check pattern |
| `TestMCTSHat_PruningFieldsInterfaceStable` | `internal/hat/mcts_pruning_r60_test.go:305` | Expanded the existing comment |

These use the canonical Go pattern `var _ Iface = (*T)(nil)` which is
a **compile-time** assertion. If the type stops satisfying the
interface, `go test` fails before any test runs because the file
won't compile. The body has no runtime assertion **by design**; my
scanner correctly flags it, but the function-level comment now makes
the intent explicit so future audits don't re-flag.

## 3. Commented-out test files — 0 findings

```bash
# Files where >70% of lines are //-comments AND >20 lines total
for f in $(find . -name '*_test.go' -not -path './.git/*'); do
  total=$(wc -l < "$f")
  if [ "$total" -lt 5 ]; then continue; fi
  commented=$(grep -cE "^\s*//" "$f")
  if [ "$total" -gt 20 ] && [ "$commented" -gt $((total * 7 / 10)) ]; then
    echo "$f: $commented/$total"
  fi
done
# (empty)
```

**Action**: nothing to do.

## 4. Test files testing only deleted functions — 0 findings (full sweep deferred)

Four `*_test.go` files contain no `func Test*` declarations:

- `internal/hexapi/game_replay_bench_r60_test.go`
- `internal/hexapi/game_summary_pdf_bench_r60_test.go`
- `internal/hexapi/game_summary_bench_r60_test.go`
- `internal/hat/bench_test.go`

All four contain only `func Benchmark*` — legitimate bench-only files,
not dead.

A full sweep for "test files that exercise only deleted functions"
needs the cross-reference machinery from Phase 1D (`cmd/audit-engine-dead/`).
That tool's `exported_but_test_only` category (495 findings on
`internal/gameengine`) is the **inverse** of this question — it finds
production funcs called only by tests, which after deletion would
leave the test orphaned. The forward direction (test-side functions
calling deleted prod funcs) reduces to "test fails to build" once
the prod func is gone, so a healthy `go build ./...` is the gate.

`go build ./...` is clean across the working tree.

**Action**: nothing to do.

## Verification

- `go build ./...` clean.
- All 13 modified tests pass: `go test ./cmd/hexdek-freya/ ./cmd/hexdek-thor/ ./internal/gameengine/ ./internal/heimdall/ ./internal/hat/ ./internal/hexapi/ ./internal/tournament/` clean (touched-package subset).
- Re-running the scanner reports **2 remaining findings**, both compile-time interface checks documented as intentional.

## Summary

| Category | Found | Actioned |
|---|---:|---:|
| Unjustified `t.Skip()` | 0 | 0 |
| No-assertion tests | 13 | 13 |
| Commented-out test files | 0 | 0 |
| Tests of deleted funcs | 0 | 0 |
| **Total** | **13** | **13** |

10 tests upgraded to explicit `defer recover()` assertions, 1 converted
to an inverted-pin assertion, 2 kept as compile-time interface checks
with clarifying comments.
