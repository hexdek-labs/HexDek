# r107 — the TYPE axis: a superset that was wrong three times out of four

**Fixed.** Date: 2026-10-14. Source: open item (E).

## The question, and why the obvious answer was a trap

(E) asked whether `gs.IsCreatureOf` is a strict superset of the layer-blind
`(*Permanent).IsCreature`, with r65's warning attached: "these two agree" from
READING is not verification. A probe over four type cases measured:

```
(0) plain creature             blind=true   layered=true    agree
(1) impending    §702.176      blind=false  layered=true    DIVERGE
(2) bestowed     §702.103a     blind=false  layered=true    DIVERGE
(3) reconfigured §702.151b     blind=false  layered=true    DIVERGE
(4) layer-4 type-add (Ensoul)  blind=false  layered=true    DIVERGE

blind-true-only = 0    layered-true-only = 4
```

So it **was** a strict superset — and that is exactly the trap. A superset is
only a safety property if the extra members are CORRECT, and **three of those
four extra trues were rules-wrong**:

| rule | keyword | what the CR says |
|---|---|---|
| §702.103a | bestow | while attached as an Aura it is an enchantment — Aura, **not a creature** |
| §702.151b | reconfigure | while attached it is just an Equipment and **stops being a creature** |
| §702.176 | impending | while time counters remain it is **not a creature** |

Only case (4), the Ensoul-style type-add, was a correct extra true.

`IsCreature` implemented all three suppressions inline. `IsCreatureOf` reads
`GetEffectiveCharacteristics`, and **nothing in `layers.go` had ever heard of
the three flags** — grep put every read at `state.go:2023/2030/2039`, inside
the layer-blind predicate alone.

Had the item been worked the way its framing invited — "the layer-aware one is
a superset, migrate the 687 call sites to it" — it would have introduced three
§702 violations to fix one.

## It reached production

`sba704_5f` (§704.5f toughness), `sba704_5g` (§704.5g lethal damage) and
`sba704_5h` (§704.5h deathtouch) all gate on `gs.IsCreatureOf`. Driving the
real `StateBasedActions` entry point, a bestowed, a reconfigured and an
impending permanent at lethal damage were **each destroyed**.

Meanwhile `IsCreature`'s own doc comment for the reconfigure case promised the
permanent "isn't seen by creature SBAs". **The contract was written down in the
predicate the SBAs do not call** — the recurring finding, instance 28.

## The fix

Not a call-site migration. One shared predicate so the two readers cannot drift
again:

- `permSuppressesCreatureType(p)` in `creature_type_suppression_r107.go` holds
  the three CR rules.
- `(*Permanent).IsCreature` delegates to it (its three inline checks removed).
- `applyCreatureTypeSuppression` strips the creature type from the computed
  characteristics at the **end of the §613.1d layer-4 pass** — these are
  type-changing effects, so they belong in that layer, and closing the layer
  with them means an Ensoul-style type-add cannot re-grant a type the keyword
  has suppressed.

After the fix the only remaining divergence is case (4), where layer-aware is
the correct answer — so the superset is now a *correct* superset, and migrating
a `.IsCreature()` site to `IsCreatureOf` is defensible for the first time.

## Fixture notes, each of which cost a wrong run

Two of these are old lessons biting again, recorded so the next reader skips them:

- **Reading the flags AFTER `StateBasedActions`** reports the wrong thing:
  `destroyPermSBA` removes the permanent and `BestowFalloff` clears the flag.
  Read predicates before the entry point runs (lesson 29).
- **A bestow/reconfigure subject needs its attacher type.** Without
  `enchantment`/`aura` (or `artifact`/`equipment`) on the card, `sba.go:1232`
  detaches it as "attached but not an aura/equipment", falloff then restores the
  creature type, and the subject dies for a reason unrelated to the type axis.
  A fixture that omits a field the real path always sets is not a fixture of
  that thing (r100).
- An **unattached** bestow subject is binned by §704.5m before the type axis is
  ever consulted.

## Verification

Three SBA regressions fail individually pre-change (semantics-only revert: the
single `applyCreatureTypeSuppression` call removed) and pass after. The probe
carries a positive control — a plain creature must read true on both — so an
all-false run cannot masquerade as a result (lesson 53).

Eleven packages green including crcite and paritycheck; hexapi shows exactly its
three known deterministic reds and no new ones.

**Lesson 59: a superset is not a safety property.** "B returns true everywhere A
does" reads like proof B is the safe choice to migrate to, and says nothing about
whether B's extra trues are right. Check the extras against the rule, one by one,
before letting the word "superset" license a migration.
