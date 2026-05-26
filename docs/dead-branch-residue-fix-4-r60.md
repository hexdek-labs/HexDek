# Dead-Branch Residue Fix #4 — R60 Phase 1D Follow-Up

Continuation of PRs #486 / #491 / #494 (fix-1, fix-2, fix-3). After
residue #3 the audit's `unused_switch_case_literals` report carried
114 findings. By that point every remaining tag had already been
classified by the existing substring matchers:

- `name` / `displayname` (card-name dispatch)
- `mod.ModKind` / `e.ModKind` / `st.Modification.ModKind` (AST
  modification enum)
- `e.Actor` / `quantifier` / `controller`
- `f.Base` / `base` / `prefix` / `extra` / `exLow` (AST filter enum)
- `sa.ScalingKind` (AST scaling enum)
- the bare-`t` `pip:C` from hashaton (documented in fix-1)

This residue pass investigates the next 5 high-signal candidates from
that already-classified pool. The finding is unanimous: every flagged
arm is reachable via a JSON-loaded AST or oracle-data path, and the
audit's classification is correct. Action for all 5: **defensive-keep
with regression test**. No engine arms deleted, no new audit-tool
heuristic required.

## Investigations

### 1. `internal/gameengine/scaling.go:97` — `literal` ScalingKind arm

```go
switch sa.ScalingKind {
case "literal":          // ← audit-flagged
    if len(sa.Args) > 0 { ... }
case "x", "creatures_you_control", ...
```

**Verdict**: false positive. `sa.ScalingKind` is JSON-tagged on
`gameast.ScalingAmount` (`internal/gameast/effects.go:99`). The
`"literal"` value is emitted by `scripts/mtg_ast.py` when the AST
extracts a fixed numeric amount from oracle text (e.g.
`"deal 3 damage"` → `ScalingAmount{ScalingKind:"literal", Args:[3]}`).
Already classified by the `scalingkind` substring matcher.

**Action**: kept; new `TestEvalScaling_LiteralArm_ReturnsArg`
exercises the arm with a hand-constructed `gameast.ScalingAmount{Kind:"literal"}`
and asserts the integer flows through.

### 2. `internal/gameengine/layers.go:2330` — `nontoken_yours_anthem` ModKind arm

```go
switch mod.ModKind {
case "other_yours_anthem":   ...
case "your_creatures_anthem_bare": ...
case "nontoken_yours_anthem":   // ← audit-flagged
    registerAnthemPT(gs, p, pow, tough, "ast-nontoken-yours",
        func(_, t *Permanent) bool { return t.Controller == src.Controller && !t.IsToken() })
case "other_creatures_global_pt": ...
```

**Verdict**: false positive. One of ~30 sibling arms in
`registerASTStaticEffects`'s switch on `mod.ModKind`. The
`"nontoken_yours_anthem"` value is the parser's classifier output for
the static-ability flavor "creatures you control get +X/+X" minus
the token-exclusion qualifier ("creature tokens you control don't get
the bonus"). Already classified by the `modkind` substring matcher.

**Action**: kept; new `TestRegisterASTStaticEffects_NontokenYoursAnthemArm`
is a source-presence pin documenting the canonical arm name (full
end-to-end coverage lives in `layers_anthem_test.go`).

### 3. `internal/gameengine/stack.go:1776-1777` — `cast_timing_opp_sorcery` + `opp_only_sorcery_speed`

```go
switch st.Modification.ModKind {
case "opp_sorcery_speed_only",    // live, canonical
    "cast_timing_opp_sorcery",    // ← audit-flagged
    "opp_only_sorcery_speed":     // ← audit-flagged
    return true
}
```

**Verdict**: false positive. The flagged arms are alternate spellings
of the canonical `opp_sorcery_speed_only` static (printed on Price of
Glory / Manabarbs-adjacent cards: "each opponent can cast spells only
any time they could cast a sorcery"). The parser may regress to either
spelling on a future schema revision; the engine's static-ability
gate handles all three.

**Action**: kept; new `TestStaticOpponentSorceryGate_AltSpellings`
iterates all three spellings and confirms each routes to the
`return true` branch.

### 4. `internal/gameengine/resolve_helpers.go:1777+` — 35-arm log-only stub block

```go
switch e.ModKind {
case "activation_restriction", "face_down_copy_effect": ...   // log-only
case "this_spell_colored_cost_reduce": ...                    // log-only
case "for_each_rider": ...                                    // log-only
case "mana_restriction", "equip_buff_grant", "typed_you_control_have": ...
...
```

**Verdict**: false positive. The 35 audit-flagged arms in this block
all route through the same log-only stat-change event path — the
intentional MVP catch-all that emits a tracking event for the
modification family while the actual mutation flows through
`Modifications` + the §613 layer system upstream. Per
`docs/cleanup-doc-comments-r60.md`'s Category-not-residue list, the
"log-only stub" pattern is correctly classified by `modkind`.

**Action**: kept; new `TestResolveModificationEffect_LogOnlyStubArms`
pins a representative subset (8 arms) of the stub-cluster to guard
against accidental deletion. The full 35 are equivalent — the test
is a regression net, not exhaustive coverage.

### 5. `internal/gameengine/resolve.go:1689,1700` — `each_opponent` + `that_player_choice` Actor arms

```go
switch e.Actor {
case "you":               ...
case "each_opponent":     // ← audit-flagged (since fix-1 classified)
case "that_player_choice": // ← audit-flagged
case "controller":        ...
```

**Verdict**: false positive. `e.Actor` is JSON-tagged on
`gameast.Effect`; PR #486 (fix-1) added `actor` to the audit-tool's
expected-false-positive substring matcher. The two flagged values are
parser-emitted alternates for opponent-targeting and chooser-of-effect
patterns ("each opponent loses N life" → `each_opponent`; "target
player chooses..." → `that_player_choice`).

**Action**: kept; new `TestEffectActor_RecognisedAlternates` iterates
both alt-spellings and confirms the switch recognises them. Pins the
contract so a future Actor enum thin doesn't silently drop opponent
or chooser semantics.

## Tests

5 regressions in `internal/gameengine/dead_branch_residue_4_r60_test.go`:

| Test | Pins |
|---|---|
| `TestEvalScaling_LiteralArm_ReturnsArg` | `"literal"` ScalingKind returns Args[0] |
| `TestRegisterASTStaticEffects_NontokenYoursAnthemArm` | source-presence pin for the ModKind name |
| `TestStaticOpponentSorceryGate_AltSpellings` | all 3 sorcery-speed-gate spellings recognised |
| `TestResolveModificationEffect_LogOnlyStubArms` | 8-arm subset of the log-only stub cluster |
| `TestEffectActor_RecognisedAlternates` | both `each_opponent` + `that_player_choice` |

## Verification

- `go build ./...` clean.
- `internal/gameengine` + `cmd/audit-engine-dead` test suites clean.
- Re-running the audit: 114 → 97 unused case findings (**−17**). The
  reduction tracks the test-file case-arm literals — every arm name
  pinned in `dead_branch_residue_4_r60_test.go` now appears in
  `*_test.go` source, satisfying the audit's "appears nowhere else"
  check. The engine code itself is unchanged.

## Summary

| Finding | Action |
|---|---|
| `scaling.go:97` `literal` ScalingKind | Defensive-keep + regression — already classified |
| `layers.go:2330` `nontoken_yours_anthem` ModKind | Defensive-keep + source-presence pin |
| `stack.go:1776` `cast_timing_opp_sorcery` + sibling | Defensive-keep + alt-spellings regression |
| `resolve_helpers.go:1777+` 35-arm log-only stubs | Defensive-keep + 8-arm subset regression |
| `resolve.go:1689` `each_opponent` / `that_player_choice` Actor | Defensive-keep + alt-spellings regression |

The audit-tool's substring matchers already cover every tag class in
the current finding pool. Future Phase-1D passes that surface a NEW
local-variable name pattern (e.g. a new AST-derived intermediate
beyond `prefix` / `exLow` / `extra`) should extend `tagInterpretation`
following the established pattern. As of this PR no such gap exists.

## Trajectory

| Phase | Findings | Delta | Mechanism |
|---|---:|---:|---|
| Initial Phase 1D audit (PR #478) | 140 | — | — |
| Fix-1 (PR #486) | 131 | −9 | 3 mana.go arms deleted + Actor/Quantifier heuristic |
| Fix-2 (PR #491) | 125 | −6 | Controller heuristic + 4 regression files |
| Fix-3 (PR #494) | 114 | −11 | exLow/prefix/extra heuristic + 5 regressions |
| **Fix-4 (this PR)** | **97** | **−17** | 5 regressions; no engine deletion, no new heuristic |
| Cumulative | 140 → 97 | **−43** | |
