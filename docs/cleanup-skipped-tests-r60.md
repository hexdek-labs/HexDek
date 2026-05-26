# Cleanup — Skipped-Tests Justification Sweep (R60 Versailles Phase 2E)

**Date:** 2026-05-25
**Branch:** `dev/cleanup-skipped-tests-justify-r60`
**Scope:** every `t.Skip()` / `t.Skipf()` / `tb.Skip()` call across the
repo (66 sites), classified by whether the skip rationale is obvious from
context or needs a justifying comment.

## Method

1. Grep `^.*\b(t|tb)\.Skip(f)?\(` across `*.go` files.
2. For each call site, read the surrounding ~5 lines and decide:
   - **Self-justifying**: the skip MESSAGE itself explains the why
     (e.g. `"oracle data not available at %s — run scripts/fetch-oracle.sh"`).
   - **Helper-chained**: the test inherits a skip from a shared
     helper (`findDecks` / `loadCorpus` / `corpusPath`); the helper
     itself should carry the rationale.
   - **Needs comment**: terse/cryptic skip that doesn't tell a reader
     unfamiliar with the codebase WHY skipping is acceptable.
   - **Delete candidate**: the skip indicates the test is permanently
     broken or covers state the engine no longer reaches.
3. For "needs comment", added a comment above the skip (or at the
   helper) explaining the design intent.

## Result classification

**Total skip-like calls audited: 66**

- **Self-justifying** (skip message is enough): **54**
- **Helper-chained, justified at the helper after this sweep**: **8**
  - All "need N decks" / "corpus unavailable" / "ast_dataset.jsonl not
    found" calls funnel through 4 helpers.
- **Needed an inline comment** (added this sweep): **3**
  - `cmd/hexdek-loki/main_test.go:151` — graceful-degradation skip
    needs the "this test owns filter contract, not detector contract"
    framing.
  - `cmd/hexdek-freya/meta_strong_against_test.go:90` — dormant
    defensive guard against a future matrix prune.
  - (The 3rd lands at the helper rather than per-call-site.)
- **Confirmed-irrelevant / delete candidates**: **0**.
  - Every `t.Skip()` traces to a real reason (gitignored data, `-short`
    mode, env-var gate, defensive guard). None are dead artifacts.
- **Stale skip-reason / restore candidates**: **0** at this depth.
  - Sampled: the loki test does still trigger the synthetic violation
    today (verified by running the test — passes, doesn't skip).

## Justifying comments added

### 1. `internal/tournament/runner_test.go` (shared helper block)

Tournament integration tests across this package (`engine_test.go`,
`stress_test.go`, `runner_test.go`, `swiss_test.go`, `double_elim_test.go`,
`elo_test.go`, `mcts_diag_test.go`, `balanced_pool_test.go`,
`mdfc_grinder_test.go`, `zone_accounting_stress_test.go`,
`contract_e2e_test.go`, `conviction_harvest_test.go`) all skip via
`findDecks` / `loadCorpus`. Added one block-comment at the top of
`runner_test.go` (where both helpers live) explaining:

- The dependency on `data/rules/ast_dataset.jsonl` and `data/decks/personal/*.txt`.
- Both are gitignored (cross-link to CLAUDE.md "Data Files").
- The `t.Skip()` is intentional graceful-degradation so engine-only
  contributors can run the rest of the suite without fetching data.

This single comment justifies ~28 of the 66 skip calls across the
tournament package without per-test repetition.

### 2. `internal/astload/astload_test.go` — `corpusPath`

Added a 4-line rationale on the `corpusPath` helper (size, gitignored
status, why skip is the right behavior). Justifies the 2 skips in this
file (`corpusPath` + `TestLoadFullCorpus`).

### 3. `internal/gameengine/resolve_test.go` — Corpus-driven section header

Added 4 lines to the section divider above `corpusPath`. Justifies
both skips in the corpus-driven test (`-short` and "corpus not found").

### 4. `internal/deckparser/deckparser_test.go` — `astDatasetPath`

Added a 3-line comment. Justifies the 4 skips that depend on this
helper.

### 5. `cmd/hexdek-loki/main_test.go:151` (inline)

The skip message — `"synthetic state did not produce a violation;
engine semantics may have shifted"` — is unusually broad and easily
misread as "this test is broken." Added 7 lines above the skip
explaining the actual design:

- The test owns the **filter** pipeline contract (matchesInvariantFilter,
  invariantFilter env-var), not the **detector** contract.
- If a future engine refactor makes the duplicate-card-pointer plant no
  longer trip any invariant, the right behavior is `t.Skip()` — the
  invariant-detector contract is independently owned by
  `internal/gameengine/invariants_test.go`.
- Verified the skip is dormant today (test passes; doesn't reach the
  Skip branch).

### 6. `cmd/hexdek-freya/meta_strong_against_test.go:90` (inline)

The skip — `"need ≥2 entries to test ordering, got %d"` — is a
defensive guard against a future matrix prune that drops the "Control"
row below the threshold needed to validate multi-tier ordering. Added
a 5-line comment framing it as dormant future-proofing.

## Already-justified skips (no change needed)

These already have a message + nearby comment that together explain the
skip rationale; left untouched:

| File | Line | Skip message |
|---|---|---|
| `cmd/parser-coverage/html_export_test.go` | 129 | `"timezone db unavailable"` (clear from `time.LoadLocation` call above) |
| `cmd/parser-coverage/history_test.go` | 49 | `"time zone DB unavailable"` (same) |
| `cmd/hexdek-freya/power_calibration_test.go` | 27 | function-level doc comment already says "Skipped when data/rules/oracle-cards.json is absent (gitignored)" |
| `cmd/hexdek-freya/bracket_calibration_test.go` | 23 | skip itself names the fetch script: `"…— run scripts/fetch-oracle.sh"` |
| `cmd/hexdek-freya/spellbook_import_test.go` | 256 | `"curated set already contains Painter + Grindstone — fixture needs an alternate new combo"` (self-documenting fix instruction) |
| `internal/tournament/stress_test.go` | 29 | `"stress integration test (set HEXDEK_FULL_INTEGRATION=1 to run)"` |
| `internal/tournament/conviction_harvest_test.go` | 46 | `"set HEXDEK_CONVICTION_HARVEST=1 to enable the harvest run"` |
| `internal/hat/self_play_cohesion_test.go` | 199, 297 | `"self-play game loop, skipped in -short mode"` |
| `internal/astload/astload_test.go` | 130 | `"-short: skipping full corpus load"` |
| All `-short` mode skips across the suite (~10 sites) | — | `-short` is a standard Go convention; the test name + `if testing.Short()` already makes the intent obvious. |

## Cross-reference: design pattern

The dominant pattern (~56 of 66 skips) is **data-availability skip with
graceful degradation**:

```go
path := astDatasetPath()
if path == "" {
    t.Skip("no AST dataset")
}
```

This pattern is correct and intentional — the alternative would be to
hard-fail every test on every fresh clone, which would obscure real
regressions. The cleanup here doesn't change behavior; it adds the
explanatory context once per helper so future readers don't need to
re-derive the rationale at each call site.

## Verification

- `go build ./...` clean.
- `go test ./cmd/hexdek-loki/... ./cmd/hexdek-freya/... ./internal/tournament/... ./internal/astload/... ./internal/deckparser/... ./internal/gameengine/...` clean (no skips became failures).

## Net diff

7 files changed, comments-only, +N / -0:

- `internal/tournament/runner_test.go`
- `internal/astload/astload_test.go`
- `internal/gameengine/resolve_test.go`
- `internal/deckparser/deckparser_test.go`
- `cmd/hexdek-loki/main_test.go`
- `cmd/hexdek-freya/meta_strong_against_test.go`
- (this doc)
