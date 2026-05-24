# Layer 613 Audit — R60

Date: 2026-05-24
Scope: `internal/gameengine/layers.go` continuous-effect application,
focused on CR §613.4 sublayer ordering within layer 7 (P/T).

## TL;DR

Found one concrete ordering bug and fixed it: counters and
`Permanent.Modifications` were applied as a post-pass *after* layer 7d
switch, but CR §613.4c places them *in* 7c (alongside anthems), which
must apply *before* 7d switch. For symmetric pumps (+1/+1 counters,
+3/+3 Brute Force) the ordering is mathematically irrelevant. For
asymmetric pumps (Berserk-style +X/+0) combined with switch effects
(Hands of Binding, About Face, Inside Out, Twisted Image), the wrong
P/T is produced.

## CR §613.4 reference

> 613.4. Within layer 7 the following sublayers apply in the order
> described below.
>
> 613.4a Layer 7a: Characteristic-defining abilities that define
>   power and/or toughness are applied.
>
> 613.4b Layer 7b: Effects that set power and/or toughness to a
>   specific number or value are applied. Effects that refer to the
>   base power and/or toughness of a creature apply in this layer.
>
> 613.4c Layer 7c: Effects and counters that modify power and/or
>   toughness (but don't set power and/or toughness to a specific
>   number or value) are applied. (See rule 122, "Counters.")
>
> 613.4d Layer 7d: Effects that switch a creature's power and
>   toughness are applied.

The official CR has four PT sublayers (a/b/c/d). The HexDek engine
declares five (a/b/c/d/e) but `Sublayer = "e"` is never assigned by any
registered effect — harmless cruft, called out below.

## Pre-R60 engine order

```
GetEffectiveCharacteristics:
  applyLayer(perm, LayerCopy,    "")
  applyLayer(perm, LayerCopy,    "a")
  applyLayer(perm, LayerCopy,    "b")
  applyLayer(perm, LayerControl, "")
  applyLayer(perm, LayerText,    "")
  applyLayer(perm, LayerType,    "")
  applyLayer(perm, LayerColor,   "")
  applyLayer(perm, LayerAbility, "")
  applyLayer(perm, LayerPT,      "a")   ← CDA
  applyLayer(perm, LayerPT,      "b")   ← set
  applyLayer(perm, LayerPT,      "c")   ← modify (anthems only)
  applyLayer(perm, LayerPT,      "d")   ← switch
  applyLayer(perm, LayerPT,      "e")   ← (unused)
  applyCountersAndMods(perm, chars)     ← counters + Modifications POST-PASS
```

The post-pass adds `Counters["+1/+1"] − Counters["-1/-1"]` to both
power and toughness, then walks `Permanent.Modifications` summing
`Power` and `Toughness` deltas. Both contributions belong in CR's 7c,
but the engine applied them after 7d switch.

## Worked failure case

Card under test: a 3/2 creature.

- Apply Berserk-style "double power" pump (from `resolve_helpers.go:263`,
  which builds `Modification{Power: pow, Toughness: 0}` for the
  "double_power" branch) — uneven +3/+0.
- Apply Hands-of-Binding-style P/T switch (`RegisterPTSwitch`,
  sublayer "d").

CR-correct order:

```
base 3/2 → 7c +3/+0 → 6/2 → 7d switch → 2/6
```

Pre-R60 engine order:

```
base 3/2 → 7d switch → 2/3 → post-pass +3/+0 → 5/3   ✗
```

Cards in the corpus exposed to this:

- **Berserk** (legacy) — `Power: pow, Toughness: 0` modification path.
- **About Face**, **Inside Out**, **Twisted Image** — `RegisterPTSwitch`
  sublayer "d" interactions with any uneven `Permanent.Modifications`
  the target carries.
- **Doran the Siege Tower** — modeled as a global 7d switch on creatures
  (`RegisterDoranSiegeTower`, see `layers.go:2193+`); creatures with
  uneven pumps (Berserk, Furor of the Bitten if it landed as a
  modification, level-up brackets with asymmetric P/T deltas) read
  wrong combat damage under the pre-R60 ordering.
- **Level-up brackets** with asymmetric power/toughness (e.g. Joraga
  Treespeaker's bracket pumps) on a switched creature.

## Fix

`internal/gameengine/layers.go:GetEffectiveCharacteristics`:

```
  applyLayer(perm, LayerPT, "c")
- applyLayer(perm, LayerPT, "d")
- applyLayer(perm, LayerPT, "e")
- applyCountersAndMods(perm, chars)
+ applyCountersAndMods(perm, chars)   // counters + mods are 7c per §613.4c
+ applyLayer(perm, LayerPT, "d")
+ applyLayer(perm, LayerPT, "e")
```

`applyCountersAndMods` now executes as the canonical 7c counter/mod
step, before 7d switch. The fix is a strict re-order — no new fields,
no behavior change for the common +1/+1-counters-plus-anthem cases.

Three pinning regression tests in
`internal/gameengine/layers_r60_switch_test.go`:

- `TestLayer7c_AsymmetricModBeforeSwitch_R60` — the failing case above,
  passes post-fix with the CR-correct 2/6.
- `TestLayer7c_PlusOneCounterCommutesWithSwitch_R60` — pins that
  symmetric counters commute with switch (no regression).
- `TestLayer7b_HumilityPlusOneOneCounter_R60` — pins Humility's set 1/1
  followed by a +1/+1 counter still reads 2/2 (the test the existing
  post-pass comment specifically called out).

## Audit-only findings (no fix)

- `Sublayer = "e"` is never assigned by any registered continuous
  effect (`grep -nE 'Sublayer.*"e"' layers.go` returns nothing). The
  `applyLayer(..., LayerPT, "e")` call in
  `GetEffectiveCharacteristics` is a no-op. Safe to leave; remove in a
  follow-up cleanup if desired.
- The §613.8 dependency-detector in `effectDependsOn` (line 556+) has
  a placeholder branch at line 569–589 that runs `b.Predicate(gs,
  a.SourcePerm)` for type-changing-effect-on-same-layer-4 but never
  returns `true` based on the result. Worked example: Opalescence (L4
  type-adder) and another L4 effect on the same enchantment-creature
  target would skip the dependency. Not exercised by any current
  per-card handler, so no concrete failing case in the corpus today.
  Documented as a §613.8 follow-up.
- `LayerCopy` is applied three times (`""`, `"a"`, `"b"`) but only
  `""` is in use. Same cruft pattern as `LayerPT "e"`. Leaving for
  follow-up.

## Validation

```
go test ./internal/gameengine/... -count=1 -timeout 300s
ok  github.com/hexdek/hexdek/internal/gameengine          3.4s
ok  github.com/hexdek/hexdek/internal/gameengine/per_card 1.5s
```

All pre-existing layer tests (`layers_test.go`, `layers_r38_test.go`)
remain green.
