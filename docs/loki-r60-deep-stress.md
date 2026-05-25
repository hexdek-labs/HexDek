# Loki R60 Deep Stress

Date: 2026-05-24
Branch: `dev/loki-r60-deep-stress-r60`
Goal: take the 10-seed wide-seed gauntlet from 2,000 chaos games per
seed up to **5,000 chaos games per seed** (5x sample depth) to push
the "engine officially clean" claim to a tighter statistical bound.
Same 10 seeds previously validated at 2K each in `docs/loki-r60-
final-confirm.md` (all 0/0).

## TL;DR

**7 of 10 seeds returned 0/0 at 5K games. 3 seeds surfaced new
residuals visible only at the deeper sample.**

| Seed | Chaos (5K games) | Nightmare (10K boards) | Verdict |
|:----:|:----------------:|:----------------------:|:-------:|
| 42 | **0** | **0** | ✅ clean |
| 43 | **0** | **0** | ✅ clean |
| 99 | **0** | **0** | ✅ clean |
| 7 | **0** | **0** | ✅ clean |
| 1337 | **0** | **0** | ✅ clean |
| 2024 | **24 in 1 game** | **0** | ⚠️ Athreos cross-seat race |
| 2025 | **1 in 1 game** | **0** | ⚠️ Charix +X/-X stacking |
| 31415 (π) | **0** | **0** | ✅ clean |
| 271828 (e) | **1 in 1 game** | **0** | ⚠️ Rest in Peace replacement skipped |
| 161803 (φ) | **0** | **0** | ✅ clean |

Aggregate at the deeper sample:

- **50,000 chaos games + 100,000 nightmare boards** total
- **26 violations across 3 games** (3 distinct signatures, each
  concentrated on a single game)
- **0 nightmare violations across all 10 seeds × 10K boards each**
- **0 crashes / 0 panics**

Per-game stochastic violation rate at 5K depth: **0.006%** chaos
games (3 in 50,000), **0%** nightmare boards. Compare against the
r41 baseline of 33% per game and the r60 final confirm rate of
0.0% at 2K depth — the deeper sample surfaces three rare-window
bugs that the 2K sample missed.

## Isolated residuals

### Residual 1 (seed 2024 game 2798) — Athreos cross-seat race

**Invariant**: CardIdentity (24 hits, all on game 2798 turn 36 cleanup)
**Signature**: `Woodland Liege` AND `Athreos, Shroud-Veiled` both
appear simultaneously on seat 2 AND seat 3 battlefields — same
`*Card` pointer in two zones.

Event trail (excerpt):

```
[1449] per_card_handler seat=0 source=Athreos, Shroud-Veiled
[1450] trigger_evaluated seat=2 source=Athreos, Shroud-Veiled
[1451..1456] seat 2's Athreos trigger pushes + resolves
[1457] per_card_failed seat=0 source=Athreos, Shroud-Veiled
[1458..1464] seat 3's Athreos trigger pushes + resolves
[1465] per_card_failed seat=0 source=Athreos, Shroud-Veiled
```

Classic r41-era Adric / Oketra / Abuelo pattern: Athreos,
Shroud-Veiled is "When a creature with a coin counter dies, return
it to the battlefield under your control." When BOTH seat 2 and
seat 3 control an Athreos, and both have placed coin counters on
the same target, the dying creature triggers BOTH handlers — both
try to claim the *Card. The per_card handler at
`internal/gameengine/per_card/athreos_shroud_veiled.go` apparently
doesn't validate that the card is still in graveyard before
claiming (same gap fixed for Gisa / The Reaper / etc. in earlier
r60 waves).

The 24 hits = invariant fires once per turn × 2 violations per
(SBACompleteness + LifeConsistency on each Athreos/Liege) ×
multiple turns the stuck state persists. Single signature, single
game, deterministic.

### Residual 2 (seed 2025 game 3180) — Charix +X/-X mod stacking

**Invariant**: SBACompleteness x1 (turn 59 end_of_combat)
**Signature**: `seat 0 has creature "Charix, the Raging Isle" on
battlefield with toughness=-7 (layer=-7) — SBA 704.5f missed
(base=0/17, counters=map[], mods={P:+8 T:-8 dur:until_end_of_turn}
x3)`

Charix has an activated ability that grants `+X/-X` where X equals
its current toughness. Each activation snapshots `+8/-8` (when
toughness was 8 at activation time), but with 3 sequential
activations the modifiers stack additively: net `+24/-24` on
base `0/17` → P=24, T=-7.

SBA 704.5f correctly detects toughness ≤ 0 → destroy. The
"missed" diagnosis suggests EITHER the modifier-application
order has the toughness reading the post-mod value (incorrect
self-reference) OR the SBA pass ran before the third mod was
applied. Probably the former. Either way: at most one of the
three activations should result in -8 toughness on a 0/17
creature, not three stacked.

Single signature, single game.

### Residual 3 (seed 271828 game 4773) — Rest in Peace skip

**Invariant**: ReplacementCompleteness x1 (turn 21 cleanup)
**Signature**: `card "Rapier Wit" entered graveyard (event 585)
while graveyard-exile effect is on battlefield — replacement
effect skipped`

Seat 0 controls Rest in Peace ("If a card or token would be put
into a graveyard from anywhere, exile it instead. All graveyards
are exiled."). When Rapier Wit went to graveyard, Rest in Peace
should have redirected to exile. The replacement was skipped.

This is the same family as the multiple Rest in Peace fixes earlier
in r60 (Firemane Commando + Gerrard, the destroyPermSBA emitter at
sba.go:1889 / 1950 stamping `to_zone` in Details). The skipped
replacement may be a non-creature-death zone-change path that
doesn't call `FireGraveyardEvent` (the §614 dispatcher) — likely
something like discard, sacrifice-non-permanent, or a tutor that
puts directly into graveyard.

Single signature, single game.

## Statistical interpretation

At 2K games per seed, all 10 seeds returned 0 violations. At 5K games
per seed (2.5x deeper), 3 of 10 seeds surfaced new bugs.

This is **not** a regression — those bugs have always existed; the
2K sample just wasn't deep enough to roll them out. The 5K sample
gives a much tighter bound:

- Per-game rate at 2K: 0/20,000 → upper-95%-confidence bound ≈ 0.015%
- Per-game rate at 5K: 3/50,000 → empirical rate 0.006%, upper-95%
  bound ≈ 0.017%

Both estimates are consistent. The 5K sample doesn't reveal a worse
engine — it reveals more *of the same long-tail surface* the 2K
couldn't see.

## Conclusion

The engine is **clean at the wide-seed level but has three identified
long-tail rare-window bugs** that the 5x sample surfaced. All three
are bounded to a single game out of 5,000 per affected seed, single
signature each, and isolated to specific cards / interactions:

1. **Athreos, Shroud-Veiled cross-seat reanimate race** — needs the
   Adric/Oketra-style "validate card still in graveyard before
   claiming" defensive check in
   `athreos_shroud_veiled.go`.
2. **Charix, the Raging Isle self-referencing +X/-X mod stacking** —
   needs `+X/-X` evaluation to snapshot once at resolve, not stack
   linearly across activations, OR an SBA 704.5f recheck after each
   mod application.
3. **Rest in Peace replacement skipped on Rapier Wit graveyard entry**
   — needs to identify the specific zone-change path that didn't
   call `FireGraveyardEvent`. Likely discard or sacrifice-non-perm
   route.

The r60 era's previous "officially clean at the wide-seed level" claim
holds at 2K depth. At 5K depth it weakens to "clean across 7 of 10
seeds with 3 isolated long-tail residuals." Each residual has a
deterministic reproducer (the exact game-id is captured in the
report).

## How to reproduce

```bash
for seed in 42 43 99 7 1337 2024 2025 31415 271828 161803; do
  go run ./cmd/hexdek-loki --games 5000 --seed "$seed"
done
```

Narrow per-seed:

```bash
go run ./cmd/hexdek-loki --games 2800 --seed 2024     # Athreos race in game 2798
go run ./cmd/hexdek-loki --games 3200 --seed 2025     # Charix in game 3180
go run ./cmd/hexdek-loki --games 4775 --seed 271828   # RIP skip in game 4773
```
