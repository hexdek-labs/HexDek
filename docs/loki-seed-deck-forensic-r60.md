# Loki r60 — Seed-Deck Pointer-Aliasing Forensic (Probe C)

## Verdict

**HARNESS IS CLEAN.** The Loki chaos deck-build path allocates a fresh
`*gameengine.Card` per deck-insertion. The 3324 `CardIdentity` invariant
hits surfaced by the 25k-sweep are **genuine engine bugs**, not fixture
artifacts, and require root-cause analysis in the engine.

## What we tested

The hypothesis: does the `--seed-cards` / `--seed-cards-all-seats`
injection path (and the broader chaos deck-build loop in `runChaosGame`)
accidentally alias one `*Card` pointer across multiple seats' decks?
If yes, every CardIdentity hit ("same `*Card` in two zones") would be a
fixture bug — the engine would be correctly tracking a card that the
HARNESS had silently placed in two seats' libraries.

## What we found

`buildCardFromName` (`cmd/hexdek-loki/main.go:1102`) constructs
`&gameengine.Card{...}` fresh on every call. There is no memoization
cache, no shared-Card pool, and no shortcut path that returns a
previously-built Card. The `AST` pointer it stamps onto each Card is
shared across calls — but that is **correct and intentional**: the AST
is read-only oracle data and the engine treats it as immutable. The
`Card` struct itself, which is the unit the `CardIdentity` invariant
compares with `==`, gets a new heap allocation per call.

The deck-build loop at `runChaosGame` (`main.go:812-868`) calls
`buildCardFromName` once per `(seat, card_name)` pair. With 4 seats and
99 library cards each, the loop produces 4 * 99 = 396 distinct
allocations. No aliasing path exists in the harness.

The seed-cards flag itself stores names (strings), not Card pointers
(`main.go:348-360`), so seed injection cannot leak pointers across
games — the injection writes a string into `chaosDecks[0].Cards`, and
the per-seat allocator builds a fresh Card from that string later.

## How we tested

Two unit tests in `cmd/hexdek-loki/seed_deck_alloc_r60_test.go`:

1. **`TestSeedDeckBuild_BloodMoonPointersAreUnique`** — calls
   `buildCardFromName("Blood Moon", ...)` four times, simulating four
   seats receiving the same seed-card. Asserts all 4 returned `*Card`
   pointers are distinct heap addresses via pairwise `==`. Also verifies
   each Card's `Owner` stamp survives per-seat mutation (would fail if
   the Card struct were shared).

2. **`TestSeedDeckBuild_FullLibrariesAreDisjoint`** — mirrors the full
   `runChaosGame` per-seat library-build loop end-to-end. Builds 4
   libraries of 6 cards each (same name set per seat — the worst-case
   stress shape that `--seed-cards-all-seats` exercises) and asserts
   all 24 `*Card` pointers are unique by flattening into a `map[*Card]`
   and detecting collisions.

Both tests pass. The harness does not alias pointers.

```
=== RUN   TestSeedDeckBuild_BloodMoonPointersAreUnique
--- PASS: TestSeedDeckBuild_BloodMoonPointersAreUnique (0.00s)
=== RUN   TestSeedDeckBuild_FullLibrariesAreDisjoint
--- PASS: TestSeedDeckBuild_FullLibrariesAreDisjoint (0.00s)
PASS
```

## Implications for the engine bug surface

The 25k-sweep's 3324 `CardIdentity` violations stand. They reflect real
zone-tracking bugs where the engine relocates or copies a `*Card` into
a second zone without removing it from the first. The r60 issue log
already records the class of fixes that close these (Adric / Oketra /
Dread / Gisa / Athreos / The Reaper §704.6d cross-seat race; Zidane EOT
return-to-stale-target; God-Eternal Oketra tuck-from-graveyard;
shuffle-into-owner-library not falling through to non-battlefield
zones; the Cerulean Sphinx + paradigm copy clusters), and the residual
3324 hits at 25k depth are the next layer down the same architectural
class.

Recommended next steps:

1. Aggregate the 3324 hits by card-name + zone-pair signature to
   identify the dominant offender clusters at this depth (same approach
   that found Adric / Oketra / Cerulean Sphinx in earlier sweeps).
2. For each dominant cluster, bisect to the per-card handler or engine
   primitive that moves the `*Card` between zones without unregistering
   the source zone, and apply the established fix pattern (fall-through
   zone scan + canonical-API call instead of raw `removePermanent` +
   `moveCardBetweenZones`).
3. Do NOT chase this as a harness problem.

## Files changed

- `cmd/hexdek-loki/seed_deck_alloc_r60_test.go` (new)
- `docs/loki-seed-deck-forensic-r60.md` (this file)

No engine or production code touched.
