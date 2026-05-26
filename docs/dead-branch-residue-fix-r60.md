# Dead-Branch Residue Fix — R60 Phase 1D Follow-Up

Per the Phase 1D audit (PR #478), the `unused_switch_case_literals`
report flagged 140 case arms, of which the report's "By switch tag"
triage table grouped ~93 as AST-modification-kind enums (parser-
emitted) and ~23 as card-name dispatches (data-driven). That left
~24 higher-signal residue findings — case arms on tags like
`e.Actor`, `r`, `op`, `f`, `ctrl`, `Permanent`, etc. — that warranted
manual investigation.

This PR investigates the top 5 of those residue findings and either
deletes confirmed-dead branches or documents intentional-defensive
arms whose absence would silently corrupt behavior.

## Investigations

### 1. `internal/gameengine/mana.go:212-218` — RestrictionAllows switch (**DELETED 3 arms**)

```go
case "noncreature_or_artifact_activation",
    "non_creature_activation_only",          // ← audit-flagged
    "noncreature_activation_only":            // ← audit-flagged
    return spellType == "noncreature" || ...
case "instant_or_sorcery_only":               // ← audit-flagged
    return spellType == "instant" || ...
```

**Verdict**: confirmed dead. `grep -rn` across the whole repo found no
emitter for any of the three flagged spellings:

- `non_creature_activation_only` / `noncreature_activation_only`:
  alternate spellings of the live `noncreature_or_artifact_activation`
  canonical (which IS emitted by `mana_artifacts.go` for Powerstone-style
  tokens). Variant-spelling defensiveness from an old parser revision.
- `instant_or_sorcery_only`: only appears as a partial-emit
  human-readable note in `per_card/custom_galazeth_prismari.go`
  explaining that the restriction is "tracked via seat flag untyped
  pool" — i.e., not actually implemented via the typed mana pool.

**Action**: deleted all three arms. The function's documented fallback
(`Unknown restriction → allow (conservative)`) covers any future
emitter that uses one of the deleted spellings — they'll allow the
payment instead of mis-routing. Added a comment recording the cleanup
history and confirming the safe-fallback shape.

### 2. `internal/gameengine/counter_resolve.go:167,256` — matchesCounterFilter switch (**KEPT, documented**)

Two flagged arms, both grouped with live siblings:

- `case "activated", "activated_ability":` — `activated_ability` is a
  defensive alias for the canonical `activated` spelling. Removing it
  would silently turn a Stifle-style counter into a no-op if the
  parser ever regresses to the longer form.
- `case "non", "other", "or":` — parser-edge-case fallback for
  malformed filter bases; treats them as "any spell" so the Extra
  slice's real restrictions still drive matching.

**Verdict**: intentional defensive forward-compat. Removing either arm
fails-silently (counter doesn't fire) instead of fails-loud.

**Action**: kept both. Expanded the existing comments with explicit
"Phase-1D audit flagged this as unreachable but kept because..."
notes so future audits don't re-flag for re-investigation.

### 3. `internal/gameengine/resolve.go:1681,1692` — Actor switch (**AUDIT-TOOL FIX**)

Two flagged arms (`each_opponent`, `that_player_choice`) on a switch
keyed by `e.Actor`. Investigation showed `Actor` is a JSON-tagged
field on `gameast.Effect` (see `internal/gameast/effects.go:307`)
populated from the AST dataset — i.e., emitter side is
`scripts/mtg_ast.py`, not Go source. Same false-positive class as
ModKind / ScalingKind but missed by the audit-tool's filter.

**Verdict**: not a Phase-1D engine issue — an audit-tool gap.

**Action**: updated `cmd/audit-engine-dead/main.go`'s `tagInterpretation`
to include `actor` and `quantifier` substring matches in the
"AST enum from `scripts/mtg_ast.py`" expected-false-positive bucket.
The residue list shrinks accordingly on the next audit run.

### 4. `internal/gameengine/costs.go:549` — findSacrificeCandidate `green creature` (**KEPT, documented**)

Forward-compat for AST-emitted color-filtered sacrifice costs. No
current `SacrificeFilter` emitter sets `"green creature"`, but the
function's docstring at line 143 explicitly lists it as supported,
and the AST parser COULD emit it for any "sacrifice a green creature"
oracle text. The default-fallback (`try-any-creature`) would accept
off-color sacrifices, breaking the color filter if the parser ever
does emit the value.

**Verdict**: defensive forward-compat with provable
silently-wrong-fallback if removed.

**Action**: kept. Added a comment explaining why the arm matters even
though no current per_card emitter uses it.

### 5. `internal/gameengine/per_card/hashaton.go:87` — pip-stripping `pip:C` (**KEPT, documented**)

```go
case "pip:W", "pip:U", "pip:R", "pip:G", "pip:C":
    continue   // Drop original color pips — token is black.
```

Hashaton reanimates a discarded creature as a black 4/4 zombie token,
stripping the original creature's color-pip type markers. `pip:C`
(colorless) catches Eldrazi reanimations (Emrakul, Ulamog, Kozilek):
their `Card.Types` arrays carry the colorless pip and need stripping
just like the colored ones.

**Verdict**: pip markers come from `Card.Types` JSON data, not Go
literals — the static scanner can't see emitter coverage there.
Removing the arm would leave a `pip:C` stub on the resulting token,
contradicting Hashaton's mono-black contract.

**Action**: kept. Added a comment explaining why the colorless pip
matters and why the audit can't see its emitter.

## Tests

5 regressions in `internal/gameengine/dead_branch_residue_r60_test.go`:

- `TestRestrictionAllows_LiveTagsStillWork` — pins all live restriction
  tags (`creature_spell_only`, `noncreature_or_artifact_activation`,
  `artifact_only`) against the simplified switch, plus the three
  deleted alias spellings (now route to the conservative-allow
  fallback).
- `TestRestrictionAllows_UnknownTagConservativeAllow` — pins the
  documented "unknown → allow" semantics that make deleting the
  alias arms safe.
- `TestMatchesCounterFilter_ActivatedAbilityAlias` — pins the
  defensive `activated_ability` arm. A regression that drops the
  alias would make Stifle-style counters silently fail on the
  alternate spelling.
- `TestMatchesCounterFilter_EdgeCaseBaseValues` — pins `non`/`other`/`or`
  routing to broad-match-true.
- `TestFindSacrificeCandidate_GreenCreatureFilter` — pins the
  forward-compat color-filtered sacrifice arm: black creature
  rejected, green creature accepted.

## Verification

- `go build ./...` clean.
- `internal/gameengine` test suite clean (all 5 new tests pass).
- Re-running the audit: 139 → 131 case findings (−8); the three deleted
  mana.go arms are gone, and the audit-tool fix re-classifies the two
  resolve.go Actor arms as expected-false-positive in the triage
  table.

## Summary

| Finding | Action |
|---|---|
| mana.go restriction switch (3 arms) | Deleted — confirmed dead, fallback covers re-emission |
| counter_resolve.go (2 arms) | Kept + documented — defensive forward-compat |
| resolve.go Actor arms (2) | Audit-tool false-positive — updated tagInterpretation |
| costs.go `green creature` | Kept + documented — silent-wrong-fallback risk |
| hashaton.go `pip:C` | Kept + documented — pip markers in Card.Types JSON, invisible to scanner |
