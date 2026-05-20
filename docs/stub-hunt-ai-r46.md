# Stub Hunt R46 — `internal/hat/`, `internal/analytics/`, `internal/tournament/`

**Date:** 2026-05-20
**Branch:** `dev/stub-hunt-ai-r46`
**Scope:** AI / analytics / tournament packages (≈45K LoC, ~70 source files).

## Methodology

1. `grep` for `TODO|FIXME|XXX|HACK|stub|placeholder|unimplemented|panic("not impl…")` markers across all three packages (excluding `_test.go`).
2. `awk` scan for functions whose body is ≤3 statements — candidate stubs.
3. `grep` for `_ = ` discards, dropped errors (`json.Unmarshal(..., &out)` without checking), and single-line `return 0/nil/false` bodies.
4. Hand-inspection of every short-bodied AI decision method (`MCTSHat.*`, `OctoHat.*`, `PokerHat.*`, `GreedyHat.*`) to separate **intentional delegation** from **actual stubs**.
5. Spot-checks of the largest files (`yggdrasil.go` 29K LoC, `turn.go` 1.9K LoC, `analyzer.go`, `report.go`).

## Key result up front

**There are very few classic "TODO" stubs.** The packages are mature; most thin-bodied functions are either thin getters or **deliberate delegations** to a base hat (most of `MCTSHat.Choose*` delegates to `Inner.PokerHat.Choose*` under a "Low-impact decisions — delegate to inner hat" comment block).

What I did find are **silent-failure bugs and dead-code residue** from past refactors. Those are higher-value to fix than chasing token TODO markers.

## Findings

Rated by severity:

| # | Sev | File:line | Issue | Fix in this PR? |
|---|---|---|---|---|
| 1 | Med | `internal/analytics/rivalry.go:170` + `threat_graph.go:394` | `json.Unmarshal` errors are silently dropped — corrupt files return `(nil, nil)` from public `LoadRivalries` / `LoadThreatGraph` | **Yes** |
| 2 | Low | `internal/hat/feynman.go:254-258` | `turnCapped` computed then `_ = turnCapped` — dead residue from a downgrade-rule refactor; comment says "No downgrade needed" but the predicate is still being evaluated | **Yes** |
| 3 | Low | `internal/hat/yggdrasil.go:6864-6867` | `linkedExilesAgainstUs(seatIdx int)` accepts `seatIdx` and discards it (`_ = seatIdx`). Function is never called from anywhere — dead public accessor | **Yes** |
| 4 | Low | `internal/tournament/elo.go:82-99` | `Snapshot()` uses an O(n²) bubble sort instead of `sort.Slice`. Functionally correct, just hand-rolled | **Yes** |
| 5 | Med | `internal/tournament/aggregate.go:182` | `_ = nGames // seen should == nGames; we don't enforce` — silently drops mismatch between expected and received game count | **Yes** |
| 6 | Info | `internal/analytics/combos.go` `KnownCombos` (10 entries) | Heimdall's missed-combo database is stale relative to Freya's (58 entries per CLAUDE.md). `DetectMissedCombos` only detects 10 of 58 known combos | Deferred — large data port, schema differs |
| 7 | Info | `internal/analytics/resource.go` `EventResources` | Covers 12 event kinds; doesn't model mill / exile / discard / scry / counter — limiting co-trigger causal-link coverage in `cooccurrence.go` | Deferred — coupled to cooccurrence event-kind switch |
| 8 | Info | `internal/tournament/runner.go:285` | `defaultHatFactory` returns `&hat.GreedyHat{}` — a sensible baseline, but masks the fact that tournament code never wires a YggdrasilHat by default. Documented design choice, not a stub | Skip |
| 9 | Info | `internal/hat/conviction.go` | `ShouldConcede` returning `false` across all 4 hats is **intentional** — Round 15 (`docs/conviction-reassessment-2026-05-17.md`) put concession in diagnostic-only mode pending data collection. Not a stub | Skip |

## What I deliberately did **not** flag

- **`MCTSHat.Choose*` thin delegations to `Inner.PokerHat`** — explicitly documented as "Low-impact decisions — delegate to inner hat". Working as designed.
- **`GreedyHat.ShouldCastCommander → true`** and **`ShouldRedirectCommanderZone → true`** — these are documented "greedy baseline" choices, with rationale comments inline.
- **`PokerHat.ChooseSurveil` / `ChoosePutBack` delegating to GreedyHat** — explicit fallback design.
- **`Evaluate(...)` → `EvaluateDetailed(...).Score`** in `evaluator.go` — thin extractor on a documented pair.
- **`anomaly.go` doc-reference to `docs/HexDek TODO Board.md`** — just a citation in the file's header comment block, not an in-code TODO.

## Fixes shipped in this PR

### 1. `loadRivalries` / `loadThreatGraph` — surface unmarshal errors

**Before:** `json.Unmarshal(data, &out)` — error dropped. Public `LoadRivalries` / `LoadThreatGraph` always returned `(slice, nil)`.

**After:** Inner loaders return `([]T, error)`. Public Load functions propagate the unmarshal error. Missing-file path still returns `(nil, nil)` — the existing `TestLoadRivalries_MissingFile` contract is preserved. New behavior: corrupt JSON returns the parse error so callers (`hexapi/handler.go`) can log it instead of silently rendering an empty page.

### 2. `feynman.go` — delete dead `turnCapped`

The branch that computes `turnCapped` exists, threads through `gs.Turn >= 80` and `gs.Flags["turn_capped"]`, then assigns `_ = turnCapped`. The comment block immediately above says turn-cap games already resolve via seat-order tiebreak and **need no downgrade**, so the variable has no purpose. Deleted the dead computation; kept the explanatory comment so a future reader knows why turn-cap doesn't downgrade severity.

### 3. `yggdrasil.go` — delete dead `linkedExilesAgainstUs`

Public accessor with no callers in the repo. The underlying field `linkedExilesByOpponent` is still maintained for internal use by `OrderTriggers` / `ChooseTarget` logic. Removed the dead accessor.

### 4. `elo.go` — replace bubble sort with `sort.Slice`

O(n²) → O(n log n), and matches the sort idiom used everywhere else in `analytics/` / `tournament/`.

### 5. `aggregate.go` — enforce `seen == nGames`

Mismatch now appends to `r.CrashLogs` (the existing channel for tournament-level diagnostics). The function still completes — partial-result behavior is preserved — but the discrepancy is no longer silently swallowed.

## Test impact

- `internal/analytics`: `TestLoadRivalries_MissingFile` and `TestPersistRivalries_*` still pass — missing-file → `(nil, nil)` contract preserved.
- `internal/tournament`: no test currently exercises `aggregate` with a deliberately-short channel; the new diagnostic path is additive.
- `go build ./...` clean.

## Open follow-ups (not done in this PR)

- **Sync `analytics.KnownCombos` with Freya's 58-entry `KnownCombo` database.** Schema port required (Freya tracks tutor paths; analytics tracks mana floor + win-type). Would meaningfully improve Heimdall's "missed combo" coverage.
- **Expand `analytics/resource.go EventResources` to cover mill / exile / discard / scry / tap / counter.** Requires parallel expansion of the event-kind allowlist switch in `cooccurrence.go:160-167`.
- **Tournament `defaultHatFactory` → YggdrasilHat?** Currently defaults to GreedyHat, which is a strictly weaker baseline. Confirm with maintainer whether this is intentional.
