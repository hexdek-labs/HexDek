# r157 — the three per-Card caches that never saw a transform

## What happened

r156 removed the characteristics-cache map probe. The fresh CPU profile
taken on the r156 binary to pick the next target found something else
entirely: **`strings.Contains` is 20.69% of all CPU**, almost all of it
the AI evaluator re-scanning oracle text.

The obvious round was to memoize those predicates per `*Card`. Before
building it I applied r149's rule — *prove the key is immutable* — and
the key is not immutable. Worse, the memoization that already exists is
**already stale**.

## The defect

`Card` carries three lazily-computed caches, each guarded by a "have we
computed this yet" bool:

| cache | guard | derived from |
|---|---|---|
| `OracleTextCache` | `oracleTextReady` | `Card.AST` |
| `TypeLineLowerCache` | `typeLineLowerReady` | `Card.TypeLine` |
| `ProducedColorsMask` | `ProducedColorsReady` | both of the above |

A double-faced card transform **reassigns both sources in place**:

- `dfc.go:153/159` — `TransformPermanent`
- `dfc.go:384` — `RevertTransformedDFCToFrontFace`
- `keywords_p1p2.go:388` — `ApplyDisturbETB`
- `dfc.go:489`, `stack.go:2676` — `Card.TypeLine = Card.BackFaceTypeLine`

Every one of those correctly calls `gs.InvalidateCharacteristicsCache()`.
**None of them clears the three text caches** — a bool that means
"computed" cannot represent "computed from something that has since been
replaced."

So after any transform, every reader sees the **front face for the rest
of the game**.

Probed, not reasoned about:

```
BEFORE transform: "front face: creatures you control get +1/+1. "
AFTER  transform: "front face: creatures you control get +1/+1. "
Card.AST is back face: true
Card.Name: "Probe Wolf, Howling"
```

The AST and the name followed the transform. The text did not.

## Blast radius

**44 files** read `OracleTextLower`, 141 call sites in `internal/hat`
alone. It is not only AI perception — the rules layer reads it too:

- `legality.go:923` — the **block-count cap**. A creature whose back
  face reads "can block any number of creatures" stayed capped at one.
- `sba.go:1090`, `:1117` — an aura's enchant-restriction clause
  (CR §704.5m parsing).
- `cost_modifiers.go:1354`, `:1398`, `:1118` — cost modification.

`ProducedColorsMask` is the sharpest case because the two faces differ
in kind: **Search for Azcanta** transforms *into* Azcanta, the Sunken
Ruin, a land that taps for `{U}`. The mask computed on the front-face
enchantment is `0`, and it survived.

## Exposure — live decks, not corpus

```
transform/meld cards in corpus:            823
decks scanned:                           1,727
decks containing >=1 transform card:       657  (38.0%)
total transform-card slots:              1,080
per-affected-deck median 1, max 20
```

## The fix

Key each cache on the value it was derived from rather than on a
readiness flag. This is the same move as r156's `charGS`: the guard
stops being a thing someone has to remember to clear and becomes a
property of the data.

- `oracleTextAST *gameast.CardAST` replaces `oracleTextReady` — serve
  the cache only when `card.oracleTextAST == card.AST`.
- `typeLineLowerSrc string` (+ a `typeLineComputed` bool so an empty
  type line still caches) replaces `typeLineLowerReady`. The compare is
  O(1) in practice: it is the same string header, so Go's `memequal`
  short-circuits on equal data pointers.
- `ProducedColorsValid()` / `MarkProducedColors()` record both sources.
  Exported because `internal/hat` carries its own mirror of the same
  computation (`landProducesColorsMask`) and the two must agree.

A future site that swaps a face invalidates these **by construction**.
No audit of transform sites is required, now or later.

## Controls

**5 of 5 went red**, each naming a different test:

| control | reds |
|---|---|
| A — `OracleTextLower` keyed on "computed yet" (the original bug) | 3 tests |
| B — `TypeLineLower` keyed on "computed yet" | 1 |
| C — mask keyed on `ProducedColorsReady` alone | 1 |
| D — validity drops the TypeLine half | the derived gate |
| E — validity drops the AST half | the derived gate |

Control A reds three because the mask is *derived*: breaking the oracle
text breaks it downstream, which is the coupling the round is about.

r73's lesson applied again: the existing tests that primed the cache via
`card.oracleTextReady = true` were **ported, not deleted** — 8 sites
across 4 files now set `card.oracleTextAST = card.AST`.

## Prediction

**Throughput and allocation unchanged.** The hot path swaps a bool test
for a pointer compare; recomputation happens only after a transform,
which is rare. This is a correctness round, not a performance one — the
`strings.Contains` 20.69% that led here is **untouched and still open**.

## ELO

**Before and after are incomparable.** This changes what the rules layer
and the AI perceive for transformed permanents in 38% of decks —
block caps, aura restrictions, cost modification and evaluator scoring
all move. Game outcomes will differ.
