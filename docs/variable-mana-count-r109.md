# r109 — the variable-mana bailout: the count comes first, not the colours

Issue 17, opened by r108. r108 ranked the work "colours first, the count is the
hard half, the colour half is cheap and near-risk-free". **That ordering was
wrong, and shipping it would have made the affected cards worse.** This round
establishes why, then fixes the part that is genuinely an improvement.

## 1. Why colours-first is a nerf

`resolveAddManaEffect` bails on any args text matching `addManaEffectVariable`,
and the caller adds one mana via the legacy `Seat.ManaPool++`. `EnsureTypedPool`
bridges that int into the typed pool's **`Any`** bucket.

`Any` is a **universal wildcard**. Measured in
`TestAnyManaWildcardProbe_r109`, against both the predicate and the spend path,
with an empty-pool positive control:

```
pip {W}:  Any:1 pays=true   U:1 pays=false
pip {U}:  Any:1 pays=true   U:1 pays=true
pip {B}:  Any:1 pays=true   U:1 pays=false
pip {R}:  Any:1 pays=true   U:1 pays=false
pip {G}:  Any:1 pays=true   U:1 pays=false
pip {C}:  Any:1 pays=true   U:1 pays=false
PayColoredPip({W}) against Any:1 -> "any"      (… and for every other pip)
CONTROL empty pool: cannot pay {U}  (predicate and spend both refuse)
```

So emitting Vivi's printed `{U}`/`{R}` while leaving the quantity at 1 replaces
a mana that pays *anything* with one that pays *one colour*. **Strictly worse
than the bug.** The colour is only an improvement when it arrives with the
count, because only then does the extra quantity outweigh the lost flexibility.

r108's framing — "the count and the colours are independent and only the count
is hard" — was half right. They are independent as *defects*; they are **not
independent as a fix**, and the fix order is the opposite of the one filed.

### A separate defect, measured and NOT fixed here

That same table shows `Any` paying a `{C}` pip. **CR §107.4c**: the `{C}` symbol
"represents a cost that can be paid **only with one colorless mana**", and
§106.1a makes "any colour" one of the five colours. §118.14 confirms the
contrast — spending mana "as though it were colorless" is a special permission
some effects grant, not the default.

Blast radius measured against the real corpus: **31 of 38,754 cards** carry a
`{C}` pip. Narrow, but the `Any` bucket is what Treasures, Gold, Moxes and the
whole legacy `ManaPool` bridge produce, so tightening it changes castability for
every Eldrazi-style cost. **Filed, not fixed** — it is a mana-core change that
needs its own round and its own throughput read, not the tail of this one.

## 2. The classification r108 did not have

r108 reported the bailout as overlapping marker counts ("any combination 16,
x mana 12, for each 8, equal to 1"). Classifying the same 25 cards by
**shape × distinct printed colours** is far more useful:

```
any_combination  0 colours   6     equal_to  0 colours  1
any_combination  2 colours   9     for_each  0 colours  1
any_combination  3 colours   1     for_each  1 colour   6
                                   for_each  2 colours  1
```

The sharpest finding: **`"any combination"` is a COLOUR phrase, not a quantity
phrase**, and it sits in the marker list of a predicate named
`addManaEffectVariable`. Relic of Sauron prints *"Add **two** mana in any
combination of {U}, {B}, and/or {R}"* — a literal, fixed count — and was
bailing to one generic mana purely because of the colour phrase. Three of the
25 carry a literal count this way.

This is lesson 60 again from the other side: r108 found a guard that discards
what it already knows; this is a guard **answering a different question than
its name claims**.

## 3. What shipped

In `modkind_add_mana_effect.go`, a count resolver that runs *before* any colour
is emitted. The fixed-pip filter-land path is byte-identical — the new branch
only runs when `addManaEffectVariable` is true.

Shapes resolved:

| shape | basis | cards |
|---|---|---|
| literal count | `add <N> mana in any combination of …` | Relic of Sauron |
| `where X is <self>'s power` | `gs.PowerOf(src)`, layer-aware | Vivi Ornitier, A-Vivi Ornitier |
| `where X is the number of creatures you control` | `countCreaturesControlled` | Xenagos, the Reveler |

Guards, each for a reason that was measured rather than assumed:

- **Explicit pips required.** "any combination of *colors*" (Axebane, Selvala,
  Klauth, Prismatic Geoscope, Omnath) names no colours, so there is nothing to
  distribute; those keep the fallback.
- **`spend this mana only` declines.** CR §106.6 restrictions are not modelled
  on this path, so Leyline Immersion's literal five would have minted five
  *unrestricted* mana — more wrong than the fallback's one. Applied only to the
  new branch, to leave the existing path's blast radius at zero.
- **The self-possessive must name this permanent.** "the greatest power among
  creatures you control" (Selvala) and "the total power of attacking creatures"
  (Klauth) are different bases and must not match `'s power`.
- **The Alchemy `a-` prefix is stripped.** A-Vivi Ornitier's own oracle text
  says "where X is **Vivi Ornitier's** power". See §4.

Count is distributed round-robin across the distinct printed colours — always a
legal choice for an "any combination" clause.

## 4. The ledger caught a reasoning error

`TestAddManaBailoutCorpusCoverage_r109` drives the **real** resolver over the
**real** args[0] of all 25 bailing corpus entries, dumped verbatim from
`ast_dataset.jsonl` on .206. I predicted 4 of 25 covered. It reported 3:

```
fallback A-Vivi Ornitier     Any:1
```

A-Vivi is an Alchemy rebalance and keeps the *original* card's name in its own
text, so the self-possessive check rejected it. One `TrimPrefix` later:

```
RESOLVED A-Vivi Ornitier     U:2 R:1
RESOLVED Relic of Sauron     U:1 B:1
RESOLVED Vivi Ornitier       U:2 R:1
RESOLVED Xenagos             R:2 G:1
r109 coverage: 4 of 25 bailout cards now produce typed mana
```

**4 of 25.** The remaining 21 are listed above with the reason each still
declines. The ledger is committed so the next round starts from a measurement
rather than from this document's memory.

## 5. Verification

- `TestAddManaVariableCount_r109` — five cases incl. a fixed-pip positive
  control that must not move, asserting total, per-colour, **and `Any == 0`**
  (without that stamp the 1-mana Vivi case would pass vacuously on the
  fallback).
- `TestAddManaVariableCount_r109_StillBails` — the other direction: storage
  lands, "that much", and the spend-restricted card must keep `Any:1`.
- Revert verification, semantics only, tests run individually: the fix test
  fails at HEAD, the bails test and the wildcard probe pass at HEAD (correct —
  they describe behaviour this change preserves).
- Eleven packages green including crcite and paritycheck; hexapi shows only its
  known deterministic reds.
- `text_lock.txt` +1 line for the new §106.6 citation; diff read, one rule, text
  matches the CR.

## Lesson 61

**Before emitting a more precise value, check whether the imprecise one was
WIDER.** r108 read `Any:1` as "the colour information was thrown away" and
ranked restoring it as the cheap, safe half. But the engine's `Any` is a
wildcard, so the imprecise value was *more permissive*, and restoring precision
alone would have cost the player more than the bug did. A fix that makes an
output more correct in one dimension can make it worse overall when the
dimensions trade against each other — so resolve the dimension that *adds*
capability first, and let the narrowing ride along with it.
