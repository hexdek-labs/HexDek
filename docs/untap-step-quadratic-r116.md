# r116 — the untap step was O(n²), found by the detector the r112 contract lacked

r112 fixed an O(n²) state-based action and wrote the **SBA SCAN CONTRACT**:
no state-based action may rescan the battlefield per permanent. Nothing
enforced it. This round builds the enforcement, runs it, and it immediately
finds a second instance of the same shape on a hotter path.

**`UntapAll` called `auraHoldsDownUntap` once per permanent, ungated, and that
function walked every seat's entire battlefield.** The untap step was
quadratic in board size, on every turn of every game, and paid in full on the
overwhelming majority of boards that contain no such Aura at all.

## 1. The detector

A go/ast walker (lesson 37 — a detector you write is code), scoped per
innermost function (r102), in two passes:

- **DIRECT**: a `for … range X.Battlefield` whose body transitively contains
  another `for … range Y.Battlefield`. This is r112's literal shape.
- **INDIRECT**: a `range X.Battlefield` loop whose body **calls** one of the
  794 functions that themselves walk the battlefield. This is the shape that
  would hide from the first pass — and it is where the find came from.

```
functions that walk the battlefield: 794

DIRECT nested battlefield walks: 3
  combat.go                  propagandaTaxFor              outer L2936  inner L2947
  cost_modifiers.go          ScanCostModifiersWithContext  outer L520   inner L660
  per_card/chishiro…         chishiroEndStep               outer L65    inner L76

INDIRECT (battlefield loop calling a battlefield-walker): 5
  cost_modifiers.go          ScanCostModifiersWithContext  L830   calls CountCreaturesOnBattlefield()
  keywords_p0.go             CheckBestowFalloffs           L1029  calls permanentOnBattlefield()
  phases.go                  PhaseInAll                    L815   calls PhaseIn()
  phases.go                  UntapAll                      L994   calls auraHoldsDownUntap()
  sba.go                     sba704_5n                     L1171  calls permanentOnBattlefield()
```

**All three DIRECT hits are benign and were triaged by reading them**, not
dismissed: each inner walk sits inside a `case "<single card name>":` arm —
Sphere of Safety in `propagandaTaxFor`, Magnus the Red in
`ScanCostModifiersWithContext`. Bounded by the number of copies of one card,
not by board size. Recorded so nobody re-triages them.

`sba704_5n` and `CheckBestowFalloffs` call `permanentOnBattlefield` (a linear
membership test) from inside a battlefield loop, but both are gated: 5n only
reaches it for permanents that are Equipment/Fortification **and** attached.
Bounded in practice by the equipment count. Noted, not fixed, not measured as
hot.

**`UntapAll` is gated by nothing.**

## 2. Measured before touching it

Board spread across all four seats, because that is the production shape —
`UntapAll` walks one seat (~n/4) while the inner scan walks all of them (~n).
Concentrating on seat 0 would overstate it.

```
                      n=40    n=80   n=160   n=320   n=640  n=1280  n=2560
UntapAll             10.9µs  34.2µs  44.2µs  123µs   353µs  2.87ms  7.43ms
auraHoldsDownUntap   123ns   308ns   696ns   1.45µs  2.89µs 5.71µs  12.3µs
```

The predicate alone is **cleanly linear** — which is the point: it is correct
in isolation and ruinous called n times. `UntapAll` over 320→2560 (8x) grows
60x, **exponent ≈1.97**. Allocations are linear (33 → 2082 for a 64x board),
so the cost is CPU in the scan, not garbage. At n=2560 that is **7.4ms per
untap step**; a 55-turn game spends ~0.4s there.

## 3. The fix

Same shape as r112: build the set once instead of asking per permanent.
`collectAuraNoUntapHosts(gs)` does one pass over the battlefields and returns
the set of hosts held down, or **nil when no such Aura exists** — the common
case, and a nil map reads as empty so the caller needs no special case.
`auraHoldsDownUntap` had exactly one caller and is deleted rather than left
dead (open issue (4) is a dead-code ledger; this round does not add to it).

```
                      n=40    n=80   n=160   n=320   n=640  n=1280  n=2560
before               10.9µs  34.2µs  44.2µs  123µs   353µs  2.87ms  7.43ms
after                10.2µs  17.6µs  40.8µs  71.5µs  143µs  381µs   818µs
                                                             7.5x    9.1x
exponent 1.97 -> 1.17
collect… (the index)  119ns   200ns   631ns  1.34µs  2.78µs 5.88µs  13.0µs   0 B/op
```

Allocations unchanged (2082 → 2081), and the index itself allocates nothing
when the board holds no such Aura.

## 4. Verification

The existing `scaffold_aura_no_untap_r63_test.go` already covers all four
directions — stays tapped, lock lifts on detach, control untaps normally,
and **the cross-seat case** (an opponent's Waterknot on my creature), which
is precisely what the "walk every seat" loop existed for and the direction an
indexing bug would break. All four green, so no new semantic test was needed.

Semantics are unchanged, so those guards pass before *and* after. Per lesson
56 each was **mutation-checked** instead:

- index never populated → `KeepsCreatureTapped` red.
- index keyed on the Aura instead of its host → `KeepsCreatureTapped` red.

The benchmark delta is this change's real fails-pre-change artifact.

Eleven packages green including crcite and paritycheck; hexapi only its three
known deterministic reds.

## 5. What this does and does not claim

It does **not** claim to explain r115's worker-capture episodes. Those games
ran 610.7s and 828.2s at 11.5 and 14.8 s/turn; 7.4ms of untap per turn at
n=2560 is ~0.4s over a 55-turn game, which is real but is not ten minutes.
This is one more term off the per-turn cost, not the whole of it.

What it does establish is that **r112 was not a one-off**. The same shape
existed on a hotter path, and the detector that found it is now committed:
`scripts/detect_nested_battlefield_walks_r116.go.txt`. Run it after adding
any per-permanent scan.

## Lesson 68

**Write the detector for the contract you just wrote.** r112 ended with a
contract — "no state-based action may rescan the battlefield per permanent" —
and no enforcement, which is instance thirty-two of this lane's single most
common finding, authored by me. The detector took one file and found a second
instance on a hotter path in its first run. **And the find came from the
INDIRECT pass**: the direct pass only reproduces the shape you already know,
so the pass worth writing is the one for the shape one call-hop away.
