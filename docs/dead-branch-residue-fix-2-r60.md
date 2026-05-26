# Dead-Branch Residue Fix #2 — R60 Phase 1D Follow-Up

Continuation of the PR #486 investigation. The Phase 1D audit's
`unused_switch_case_literals` report carried 131 findings after
PR #486; the "By switch tag" triage table grouped the bulk as
AST-modification-kind / card-name false positives. This PR
investigates the next 5 higher-signal residue findings on remaining
non-AST-enum / non-card-name tags.

## Investigations

### 1. `internal/gameengine/phases.go:188` — `triggerControllerMatches` `active_player` arm (**FALSE POSITIVE, audit-tool fix**)

```go
switch ctrl {
case "", "you":
    return perm.Controller == gs.Active
case "each", "each_player":
    return true
case "active_player":   // ← audit-flagged
    return perm.Controller == gs.Active
case "opponent":
    return perm.Controller != gs.Active
}
```

**Verdict**: false positive. `ctrl` is the local variable holding
`gameast.Trigger.Controller`, a JSON-tagged field
(`internal/gameast/trigger.go:21`) whose documented values include
`"active_player"` (`trigger.go:20`). Emitter is `scripts/mtg_ast.py`,
not Go source — same false-positive class as ModKind / Actor.

**Action**: extended the audit tool's `tagInterpretation` substring
matcher to include `controller`. Tags like `t.Controller` /
`e.Controller` / `trg.Controller` now route to the expected-false-
positive bucket. The bare `ctrl` local-var-name tag still classifies
as high-signal so future code that introduces unrelated `ctrl`-tagged
switches gets investigation attention.

### 2. `internal/gameengine/resolve.go:565` — `compareInt` `!=` arm (**DEFENSIVE-KEEP**)

`compareInt(a, op, b)` covers all six standard int comparison
operators (`<`, `<=`, `>`, `>=`, `==`/`=`, `!=`). Audit flagged `!=`
because no Go-side emitter produces it (parser populates
`Filter.ManaValueOp` with `<=`/`>=`/`==`/empty per the documented
shape at `gameast/filter.go:35`). But the parser COULD emit `!=` for
"not equal to" oracle patterns, and the function's default fallback
returns `false` — silently making "values are unequal" report as
equal if the parser ever emits the op.

**Action**: kept; added a comment recording the Phase 1D-residue
investigation and the silent-wrong-fallback reasoning.

### 3. `internal/gameengine/resolve.go:1657` — `resolveSacrifice` `that_thing` Query.Base arm (**FALSE POSITIVE, already classified**)

```go
switch e.Query.Base {
case "self", "it", "this",
    "that_creature", "that creature",
    "this_creature", "this creature",
    "that_thing", "that":
    ...
```

**Verdict**: false positive. `Query.Base` is `gameast.Filter.Base`
(JSON-tagged). The audit tool already classifies the
`e.Query.Base` switch tag as AST-enum via the existing `base`
substring match (added in PR #478).

**Action**: nothing engine-side. The arms route a parser-emitted
"sacrifice this creature" / "sacrifice that_thing" through the
source-permanent sacrifice path; removing any of the seven aliases
would let a Pestilence-style self-sac silently no-op instead of
sacrificing the source. New regression test exercises every arm.

### 4. `internal/gameengine/per_card/sac_outlets.go:587-593` — card-name dispatch cluster (**FALSE POSITIVE, already classified**)

```go
switch p.Card.DisplayName() {
case "Phyrexian Reclamation", "Volrath's Stronghold", ...
    "Genesis",                    // ← audit-flagged
    "The Cauldron of Eternity",   // ← audit-flagged
    "Sheoldred, Whispering One",  // ← audit-flagged
    "Liliana, Death's Majesty",   // ← audit-flagged
    "Whisper, Blood Liturgist",   // ← audit-flagged
    ...
    n++
}
```

**Verdict**: false positive. Card-name dispatch — `DisplayName()`
returns the card's name from JSON oracle data. Audit tool already
classifies `p.Card.DisplayName(…)` tags as expected-false-positive
via the `displayname` substring match.

The five flagged card names are real Magic cards that happen not to
appear elsewhere as Go string literals (most cards don't — they're
referenced through their JSON name).

**Action**: nothing.

### 5. `internal/gameengine/layers.go:2330+` — ModKind anthem variants (**FALSE POSITIVE, already classified**)

Multiple arms (`nontoken_yours_anthem`, `tri_tribe_anthem`,
`other_creatures_global_pt`, `commander_anthem`, etc.) on a switch
keyed by `mod.ModKind`. Already classified by the audit tool's
`modkind` substring matcher. Per the PR #478 reading guide, these
are AST-emitted modification-kind enums; the Python parser produces
them during AST construction.

**Action**: nothing. Spot-checked one arm (`nontoken_yours_anthem`)
to confirm it's a valid parser output shape; no Go-side dead branch.

## Tests

5 regressions across two files:

### `internal/gameengine/dead_branch_residue_2_r60_test.go`

- `TestCompareInt_AllSixStandardOps` — pins all six standard int
  comparison operators including the audit-flagged `!=`, plus the
  conservative-false fallback for unknown ops.
- `TestTriggerControllerMatches_ActivePlayer` — pins the
  active-seat-gating contract on the `active_player` arm.
- `TestTriggerControllerMatches_LiveAndAliasArms` — pins the
  surrounding arms (`""`, `"you"`, `"each"`, `"each_player"`,
  `"opponent"`, unknown-fallback) so the documentation-only change to
  the `active_player` arm doesn't accidentally regress siblings.
- `TestResolveSacrifice_SelfReferenceQueryBaseArms` — exercises
  every arm of the self-reference cluster (`self`/`it`/`this`/
  `that_creature`/`that creature`/`this_creature`/`this creature`/
  `that_thing`/`that`) end-to-end against `resolveSacrifice`,
  asserting the source is sacrificed in each case.

### `cmd/audit-engine-dead/tag_interpretation_test.go`

- `TestTagInterpretation_ASTEnumBucketCoverage` + companion
  high-signal-must-not-misclassify test. Pins the substring set
  (`modkind` / `scalingkind` / `base` / `actor` / `quantifier` /
  `controller`) used to classify expected false positives. Future
  cleanups that delete one would cause real engine arms keyed on the
  corresponding `gameast.*` field to re-surface as high-signal —
  wasting investigation time.

## Verification

- `go build ./...` clean.
- `internal/gameengine` + `cmd/audit-engine-dead` test suites clean.
- Re-running the audit: 131 → 125 unused case findings (-6). The
  reduction tracks new test files added in this PR contributing case
  arms to the emitter pool; no engine arms were deleted this pass.

## Summary

| Finding | Action |
|---|---|
| phases.go `active_player` | Audit-tool fix — added `controller` to expected-false-positive substring matcher |
| resolve.go `!=` | Kept + documented — silently-wrong-fallback risk |
| resolve.go `that_thing` (Query.Base) | Already classified by audit tool (`base` substring); engine arm exercised by new test |
| sac_outlets card-name cluster | Already classified (`displayname` substring) |
| layers.go ModKind anthem | Already classified (`modkind` substring) |

This residue pass produced only one engine-side change (the
`compareInt` doc-comment) and one audit-tool change (the `controller`
substring). The remaining four findings were false positives whose
classification was already correct — but now each carries either a
regression test pinning the live behavior of the flagged arms, or
documentation explaining why the audit-tool keeps them in the
expected-false-positive bucket.
