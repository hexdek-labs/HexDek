# r108 — (N5) Vivi: the real defect is a 25-card mana class, not a missing card

**Measured and filed. Probe committed; fix deferred to its own round.** Date: 2026-10-14.

## The card, from the corpus rather than from memory

r98 removed a Vivi handler that implemented a card which does not exist, so the
first job was to read the real one.

```
Vivi Ornitier   {1}{U}{R}   Legendary Creature — Wizard
  {0}: Add X mana in any combination of {U} and/or {R}, where X is Vivi
       Ornitier's power. Activate only during your turn and only once each turn.
  Whenever you cast a noncreature spell, put a +1/+1 counter on Vivi Ornitier
  and it deals 1 damage to each opponent.

A-Vivi Ornitier  — the Alchemy rebalance, {T} instead of {0} and no
                   once-per-turn clause. Also in the corpus, also unimplemented.
```

**The data reaches the code** (r103's check): both entries are in the AST dataset,
`fully_parsed=True`, with an `Activated{cost {0}} -> Modification{add_mana_effect}`,
a `Static{timing_restriction}` and a `Triggered{cast_filtered}`. All three
modification kinds have engine handlers. There is **no** Vivi code anywhere in the
tree, confirming r98's removal.

## What actually happens, measured at the entry point

`resolveAddManaEffect` bails out on variable quantities:

```go
// modkind_add_mana_effect.go:35
func addManaEffectVariable(text string) bool {
    for _, marker := range []string{"x mana", "any combination", "any number",
                                    "for each", "equal to"} { ... }
}
```

and the caller's fallback (`resolve_helpers.go:1568`) adds **one generic mana**.

Driving the real `ActivateAbility` with the exporter's own arg text, and a
positive control so an all-wrong run cannot be mistaken for a dead probe:

```
CONTROL  add {u}{r}       -> ManaPool 0 -> 2   typed U:1 R:1    correct
Vivi {0} (power 4)        -> ManaPool 0 -> 1   typed Any:1      want 4 in {U}/{R}
Saltcrusted Steppe        -> ManaPool 0 -> 1   typed Any:1      want X in {G}/{W}
Grand Warlord Radha       -> ManaPool 0 -> 1   typed Any:1      want N in {R}/{G}
Axebane Guardian          -> ManaPool 0 -> 1   typed Any:1      want X in any colors
```

## The finding: it is a class, and the two halves are independent

**25 of the 88 corpus cards carrying an `add_mana_effect` node (28%) hit this
bailout** and produce one colorless mana instead of what they print:

```
marker 'any combination'   16   Grand Warlord Radha, Omnath Locus of All,
                                Leyline Immersion, ...
marker 'x mana'            12   Saltcrusted Steppe, Vivi Ornitier,
                                Axebane Guardian, ...
marker 'for each'           8   Megatron, Blue/Red Mana Battery, ...
marker 'equal to'           1   Elemental Resonance
```
(markers overlap; 25 distinct cards.)

The important structural point: **the count and the colors are independent, and
only the count is genuinely hard.** The colors are written plainly in the text and
`resolveAddManaEffect` already extracts them correctly — the bailout discards that
work too, which is why Vivi yields `Any:1` rather than at least `U:1`.

So the repair decomposes:

1. **Colors, cheap and near-risk-free.** When the quantity is variable, still emit
   the extracted colors rather than generic. Fixes the colour half for the 16
   "any combination of {A} and/or {B}" cards immediately.
2. **Count, per shape.** `where X is <self>'s power` (Vivi), `the number of
   creatures you control` (Axebane), `that much` (Radha — needs the preceding
   clause's binding), charge counters removed (Batteries), `equal to enchanted
   permanent's mana cost` (Elemental Resonance). Several distinct resolvers.
3. **Vivi's remaining two clauses** are separate questions: whether
   `timing_restriction "activate only during"` actually enforces once-per-turn and
   your-turn-only, and whether the `cast_filtered` trigger preserves the
   **noncreature** filter — its `target_filter` is `null` in the AST, which is a
   live suspicion that it fires on every spell, not a measured fact.

## Why filed rather than fixed in this round

The fix is a multi-shape change to mana production touching 25 cards, including
combo-relevant ones (Axebane Guardian). Mana output changes what decks can cast,
so it needs the full five-step discipline — per-shape regressions, a revert
verification, and a throughput read after deploy — not the tail end of a tick that
had already spent its budget on measurement. Rushing exactly this kind of change is
how the defects this lane keeps finding got written.

**Ranked first for the next round**, in the order above: colors first (cheap,
independently verifiable), then the count resolvers one shape at a time, then
Vivi's trigger and timing clauses as their own probes.

**Lesson 60: when a predicate bails, check what it discards on the way out.**
`addManaEffectVariable` answers "is the quantity variable?" and the caller treats
that as "is anything here recoverable?" — throwing away a colour extraction that had
already succeeded. A guard that conflates "I can't do all of this" with "I can't do
any of this" turns a partial answer into a wrong one.
