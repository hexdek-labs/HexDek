# Dead-Branch Residue Fix #3 — R60 Phase 1D Follow-Up

Continuation of PRs #486 and #491. After residue #2 the audit's
`unused_switch_case_literals` report had 125 findings. Investigated the
next 5 higher-signal residue cases — all 5 traced to the same root
cause this round: local-variable names holding AST-emitted content
that the audit-tool can't detect via direct field-name pattern.

## Common root cause: AST-derived locals

The Phase 1D audit's `tagInterpretation` substring-matcher recognizes
direct AST field references (`mod.ModKind`, `e.Actor`, `t.Controller`)
as expected false positives. But the engine often **derives** local
variables from those fields before switching:

```go
exLow := strings.ToLower(ex)   // ex := f.Extra[i]  (gameast.Filter.Extra)
switch exLow {
case "nonenchantment", "non-enchantment": ...
```

```go
prefix := base[:idx]           // base := strings.ToLower(f.Base)
switch prefix {
case "nonland creature", "noncommander creature": ...
```

The audit's static AST scan sees only the switch tag (`exLow` /
`prefix`), not the data flow back to `Filter.Extra` / `Filter.Base`.
Result: false positives in the report.

## Investigations

### 1. `internal/gameengine/targets.go:573` `prefix` switch (**FALSE POSITIVE — audit-tool fix**)

```go
if idx := strings.Index(base, " with "); idx > 0 {
    prefix := base[:idx]
    switch prefix {
    case "creature", "artifact", ...,
         "nonland creature", "nonland permanent",   // ← flagged
         "noncommander creature",                    // ← flagged
         "noncreature permanent":                    // ← flagged
        base = prefix
    ...
```

`prefix` is sliced from `base`, which is the lowercased
`gameast.Filter.Base` (parser-emitted). The compound bases
("nonland creature", "noncommander creature") are real values the
AST parser emits for filtered targets like "nonland permanent with
mana value 3 or less". Removing any arm would drop the prefix-normalization
shortcut for that compound, returning a parser-mangled base further
down the function.

### 2. `internal/gameengine/targets.go:710` `exLow` switch (**FALSE POSITIVE — audit-tool fix**)

```go
for _, ex := range f.Extra {
    exLow := strings.ToLower(ex)
    switch exLow {
    case "nonenchantment", "non-enchantment":   // ← flagged
        ...
    case "nontoken", "non-token":               // ← flagged
        ...
    case "nonbasic", "non-basic":               // ← flagged
        ...
    case "nonlegendary", "non-legendary":       // ← flagged
        ...
```

`ex` comes from `gameast.Filter.Extra` (JSON-tagged slice). The
flagged arms are alternate hyphen/non-hyphen spellings of canonical
"non-X" filter modifiers. Forward-compat for parser variants.

### 3. `internal/gameengine/tutor_resolve.go:470` `exLow` switch (**FALSE POSITIVE — same pattern**)

Same `for ex := range f.Extra` shape as targets.go, here driving the
library-side tutor filter. The flagged arms — `non-token`, `non-basic`,
`nonlegendary`, `non-legendary` — are spelling variants of canonical
filter modifiers parser-emitted via `Filter.Extra`.

### 4. `internal/gameengine/tutor_resolve.go:497` `exLow` `historic` arm (**FALSE POSITIVE — same switch**)

Sibling arm in the same `for ex := range f.Extra` switch. `historic`
is the supertype-disjunction-shortcut from Dominaria onwards
(legendary OR artifact OR saga). The parser would emit it for oracle
text like "search your library for a historic card." Without the
arm, the tutor filter falls through and silently fails to match
historic cards.

### 5. `internal/gameengine/per_card/hashaton.go:87` `pip:C` arm (**ALREADY DOCUMENTED — confirmed false positive**)

`for t := range token.Types { switch t { case "pip:C": ... } }` —
the audit can't see `Card.Types` JSON content. Already covered by
PR #486's documentation; no change needed this pass.

## Actions

**Audit-tool fix** (`cmd/audit-engine-dead/main.go`): extended
`tagInterpretation`'s expected-false-positive substring matcher to
include `exlow`, `prefix`, and `extra` — the three canonical
local-variable names that hold AST-derived filter content. Documented
the pattern with reference to which AST field each name normally
derives from.

**Engine code**: zero changes — all 5 investigations were false
positives whose case arms are real parser-emitted forward-compat
shapes. Removing any would silently fail a parser-emitted filter
that should match.

## Tests

5 regressions in `internal/gameengine/dead_branch_residue_3_r60_test.go`:

- `TestMatchesPermanent_NonEnchantmentExtra` — both spellings on
  matchesPermanent.
- `TestMatchesPermanent_NonLegendaryExtra` — both spellings.
- `TestMatchesPermanent_NonBasicExtra` — both spellings.
- `TestCardMatchesTutorFilter_NonTokenLibrarySemantics` — pins the
  library-side no-op semantics (library cards are never tokens, so
  the filter passes through).
- `TestCardMatchesTutorFilter_HistoricArm` — drives each of the
  three qualifying types (legendary creature, artifact, saga) plus
  two negatives (plain creature, non-saga enchantment).

Plus the audit-tool test updates (`tag_interpretation_test.go`) now
include `exLow`, `prefix`, and `f.Extra` in the AST-enum-bucket
coverage list.

## Verification

- `go build ./...` clean.
- `internal/gameengine` + `cmd/audit-engine-dead` suites clean.
- Re-running the audit: 125 → 114 unused case findings (−11). The
  reduction tracks new test files adding case-arm literals to the
  emitter pool. No engine arms deleted; the audit-tool reclassification
  doesn't change the case-count directly but it does reduce the
  "verify the emitter" residue in the By-switch-tag table.

## Summary

| Finding | Action |
|---|---|
| targets.go:573 `prefix` switch (3 compound-base arms) | Audit-tool false-positive — extended `tagInterpretation` |
| targets.go:710 `exLow` switch (~6 spelling-variant arms) | Same audit-tool fix |
| tutor_resolve.go:470 `exLow` switch (~5 arms) | Same audit-tool fix |
| tutor_resolve.go:497 `historic` arm | Same audit-tool fix |
| hashaton.go:87 `pip:C` | Already documented (PR #486) |

The recurring pattern: AST-derived local variables. After this PR
the audit-tool covers three canonical names (`exLow` / `prefix` /
`extra`) plus the original direct-field names (`ModKind`, `Actor`,
`Controller`, `Quantifier`, `Base`, `ScalingKind`). The remaining
~114 findings continue to dominate in these classes; future audits
that hit yet-another-local-variable-name pattern should add it here.
