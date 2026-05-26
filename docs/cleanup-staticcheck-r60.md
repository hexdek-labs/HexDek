# Static-Check Cleanup — R60 Phase 2A

Findings + fixes from running `staticcheck ./...` (v0.7.0) and `go vet ./...`
across the repo. Every fix is locally justifiable; no auto-mass-replace.

## Before / after counts

| Metric | Before | After | Δ |
|---|---:|---:|---:|
| `staticcheck` total | **182** | **150** | **−32** |
| `go vet` total | **1** | **0** | **−1** |

## By category

| Code | Description | Before | After | Δ | Notes |
|---|---|---:|---:|---:|---|
| SA1019 | deprecated APIs | 51 | 51 | 0 | Skipped — `strings.Title` family needs `golang.org/x/text/cases` migration, deferred to a focused PR |
| U1000 | unused identifiers | 48 | 41 | **−7** | Six unexported funcs + one unexported var removed; verified zero refs via `grep -rn` |
| S1039 | unnecessary `fmt.Sprintf` | 28 | 18 | **−10** | All single-literal calls — `fmt.Sprintf("foo")` → `"foo"` |
| S1016 | redundant struct literal | 12 | 3 | **−9** | Converted to `T(x)` where both structs share field shape; preserved 2 literal sites in `report.go` where the literal intentionally omits cuttable-tier-only fields |
| S1009 | nil check before `len()` | 3 | 0 | **−3** | `if x != nil && len(x) != 0` → `if len(x) != 0` in three test files |
| S1031 | nil check around `range` | 1 | 0 | **−1** | `range` over nil map is already a no-op |
| SA4000 | identical exprs on both sides of operator | 1 | 0 | **−1** | Refactored a determinism check to keep its intent without tripping the lint |
| go vet | `append` with no values | 1 | 0 | **−1** | Dead loop replaced with an actual `sort.Strings` call — the comment said "Stable order so tests don't flake," the sort was missing |
| (others — SA4006, S1008, S1011, SA4011, etc.) | various | 33 | 33 | 0 | Out of scope for "cleanest 20-30" — needs case-by-case review |

**Total fixes shipped: 32 individual edits across 6 categories.**

## Fix-by-fix detail

### SA4000 — `internal/hexapi/spectate_rooms_test.go:218`

`TestShortHash_Stable` had `if shortHash("foo") != shortHash("foo")` —
staticcheck flagged "identical expressions on both sides of `!=`," but
the intent was a determinism check (same input, two calls, same output).
Refactored to bind each call to a temp var, preserving the intent and
silencing the lint without an annotation.

### go vet — `internal/tournament/rivalries_test.go:207`

`for _, k := range append(append([]string{}, names...))` was a no-op
loop iterating a copy with `_ = k` inside. The comment promised "Stable
order so tests don't flake" but the sort never happened. Replaced with
`sort.Strings(names)` — the obvious intent.

### S1031 — `cmd/hexdek-freya/main.go:279`

`if cardQtys != nil { for k, v := range cardQtys { ... } }` — `range`
over a nil map is a zero-iteration no-op, so the guard added nothing.
Dropped the wrapper and re-indented via `gofmt`.

### S1009 — three test files

- `internal/db/card_stats_extended_test.go:73`
- `internal/db/summary_archive_r60_test.go:230`
- `internal/gameengine/per_card/percard_stub_batch_r42b_test.go:153`

Pattern: `if x != nil && len(x) != 0` → `if len(x) != 0`. `len(nil)` is
defined as 0 in Go, so the nil guard is redundant.

### S1016 — `cmd/hexdek-freya/report.go` (×9)

`RampCard` / `AltBuildSuggestion` / `MetaAdvantage` / `CardPowerLevel` /
`PetCard` / `CoachingTip` / `BracketSignal` / `ComboResult` plus a
selective conversion on `CuttableCards`-only `jsonCardQuality`: every
field-by-field literal where the source and destination structs share
identical field names+types became a `T(x)` conversion.

**Preserved as struct literals** (2 sites): `StarCards` and `SolidCards`
loops over `CardQuality` intentionally OMIT the cuttable-tier rationale
fields (`Detected`, `WhyCut`, `Effect`, `Suggested`). A `T(x)` conversion
would copy them via `omitempty`-controlled JSON, which would surface
unintended fields if any star/solid CardQuality ever had non-zero rationale
state. Tight enough that the lint warning is the correct trade.

### S1039 — `cmd/gen-handlers/main.go` (×8) + `cmd/hexdek-freya/legality.go` (×2)

`fmt.Sprintf("literal with no formatting verbs")` → bare string literal.

### U1000 — 7 unexported declarations removed

Each verified with `grep -rn` across the whole repo (no references anywhere):

| Identifier | File | Kind |
|---|---|---|
| `effectKindSimple` | `cmd/gen-handlers/main.go` | unused 14-entry map literal |
| `countPermsOnSeat` | `cmd/hexdek-thor/advanced_mechanics.go` | unused helper |
| `countInGraveyard` | `cmd/hexdek-thor/advanced_mechanics.go` | unused helper |
| `claimInvariantFail` | `cmd/hexdek-thor/claim_verifier.go` | unused helper |
| `flattenEffect` | `cmd/hexdek-thor/corpus_audit.go` | superseded by `flattenEffectEx` (callers all migrated); only self-recursion left |
| `reKeywordLine` | `cmd/hexdek-thor/ast_fidelity.go` | unused regex var |
| `kwArgStr` | `cmd/hexdek-thor/goldilocks.go` | unused arg-extraction helper |

## What I did NOT fix (and why)

- **SA1019 (51 findings)** — all deprecated-API warnings, dominated by
  `strings.Title` (deprecated since Go 1.18). Replacement is
  `golang.org/x/text/cases` which requires adding a dependency. Deferred
  to a focused migration PR so the dependency addition is reviewable on
  its own.
- **U1000 (41 remaining)** — many are struct fields whose unused state
  might be intentional (forward-compat scaffolding, JSON-deserialization
  destination fields read via reflection, or methods on an interface
  that's invoked only via an indirection staticcheck can't follow).
  Each needs individual verification. The seven removed here are
  conservative picks (unexported, file-local, zero `grep` hits).
- **SA4006 (6 ineffective assignments)** — mostly in test files where
  the assignment is followed by a re-assignment in the same scope; the
  fixes need to verify the dropped first-write is actually dead, not
  accidentally important for the test's setup.
- **S1011 / S1008 / SA4011 / S1001 / SA6005** — each fix is
  individually obvious but adds up to <10 findings combined. Out of
  scope for "cleanest 20-30," queued for a follow-up sweep.

## Verification

- `go build ./...` clean.
- Touched-package tests pass: `go test ./cmd/hexdek-freya/... ./internal/db/... ./internal/tournament/... ./internal/gameengine/per_card/...` clean.
- `TestShortHash_Stable` (the SA4000 fix) and `TestComputeRivalries*`
  (the go vet fix) explicitly verified.
- Pre-existing `internal/hexapi` OpenAPI-spec test failures (`yaml:
  unmarshal errors: mapping key "/api/meta/archetype-vs-archetype" already
  defined`) reproduce on stashed `origin/main` — unrelated to this PR.

## Reproducing the baseline

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./... | wc -l   # total finding count
staticcheck ./... | grep -oE '[A-Z]+[0-9]+' | sort | uniq -c | sort -rn
go vet ./...
```
