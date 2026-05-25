# R60 Hat Rebalance Gauntlet — PR #191 Cross-Cutting Validation

**Date:** 2026-05-24
**Branch:** `dev/hat-eval-gauntlet-validation-r60`
**Methodology:** 2000-game pool gauntlet (4 random decks per game from a
12-deck pool), `seed=42`, default Yggdrasil hat (budget=50), same RNG
under both binaries so deck-slot assignments are identical and per-deck
win counts are directly subtractable.

The "baseline" binary was built from `evaluator.go` with the two PR #191
adjustments reverted (`w.LifeResource *= 1.0 + lateFactor*0.15` and
`w.BoardPresence *= 1.0 + aheadFactor*0.2`); the "post-#191" binary is
current `main`. Both share rounds 1-4 archetype-specific tuning, so the
gauntlet isolates the cross-cutting rebalance.

## Pool composition (12 decks)

| Deck | Archetype | Tuning round |
|---|---|---|
| Kalamax, the Stormsire | Spellslinger | R3 (#185) |
| Anowon, the Ruin Thief | Mill | R1 (#179) |
| Phenax, God of Deception | Mill | R1 (#179) |
| Uril, the Miststalker | Voltron | R2 (#181) |
| Wyleth, Soul of Steel | Voltron | R2 (#181) |
| Korvold, Fae-Cursed King | Aristocrats | R2 (#181) |
| Karador, Ghost Chieftain | Reanimator | R3 (#185) |
| Meren of Clan Nel Toth | Reanimator | R3 (#185) |
| Edgar Markov | Tribal | R3 (#185) |
| Lord Windgrace | LandsMatter | R4 (#187) |
| Aminatou, Veil Piercer | Blink | R4 (#187) |
| Atraxa, Grand Unifier | (none — midrange fallback) | — |

## Run parameters

- 2000 games each, `--pool` mode, `--seats 4`, `--seed 42`
- Same AST corpus + oracle data on both binaries
- Baseline: 3m52s wall, 8.6 g/s, 0 crashes, avg 55.7 turns
- Post-#191: 3m51s wall, 8.7 g/s, 0 crashes, avg 56.1 turns

## Per-deck winrate delta

Games-played per deck is identical between runs (same seed → same
deck-slot assignments). Δwins = post − baseline; Δrate = Δwins / games.

| Deck | Archetype | Games | Baseline wins | Post-#191 wins | Δ wins | Δ rate |
|---|---|---:|---:|---:|---:|---:|
| Atraxa, Grand Unifier | midrange fallback | 647 | 149 (23.0%) | 175 (27.0%) | **+26** | **+4.0%** |
| Anowon, the Ruin Thief | Mill | 657 | 165 (25.1%) | 174 (26.5%) | +9 | +1.4% |
| Meren of Clan Nel Toth | Reanimator | 648 | 180 (27.8%) | 187 (28.9%) | +7 | +1.1% |
| Kalamax, the Stormsire | Spellslinger | 681 | 156 (22.9%) | 162 (23.8%) | +6 | +0.9% |
| Wyleth, Soul of Steel | Voltron | 656 | 182 (27.7%) | 185 (28.2%) | +3 | +0.5% |
| Korvold, Fae-Cursed King | Aristocrats | 672 | 164 (24.4%) | 165 (24.6%) | +1 | +0.1% |
| Edgar Markov | Tribal | 641 | 169 (26.4%) | 165 (25.7%) | -4 | -0.6% |
| Aminatou, Veil Piercer | Blink | 699 | 178 (25.5%) | 174 (24.9%) | -4 | -0.6% |
| Uril, the Miststalker | Voltron | 697 | 174 (25.0%) | 168 (24.1%) | -6 | -0.9% |
| Karador, Ghost Chieftain | Reanimator | 681 | 155 (22.8%) | 148 (21.7%) | -7 | -1.0% |
| Lord Windgrace | LandsMatter | 669 | 155 (23.2%) | 146 (21.8%) | -9 | -1.3% |
| Phenax, God of Deception | Mill | 652 | 173 (26.5%) | 149 (22.9%) | **-24** | **-3.7%** |

Standard error at n≈660 is ≈1.7%, so anything within ±1.5% is noise.
Two deltas clear the 2σ threshold: **Atraxa +4.0% (p≈0.02)** and
**Phenax -3.7% (p≈0.04)**.

## By archetype (paired decks summed)

| Archetype | Decks | Baseline wins | Post-#191 wins | Δ wins | Direction |
|---|---|---:|---:|---:|---|
| midrange fallback | Atraxa | 149 | 175 | **+26** | ✅ gain |
| Mill | Anowon + Phenax | 338 | 323 | -15 | ⚠️ loss |
| Reanimator | Meren + Karador | 335 | 335 | 0 | ⏸ wash |
| Voltron | Wyleth + Uril | 356 | 353 | -3 | ⏸ wash |
| Spellslinger | Kalamax | 156 | 162 | +6 | small gain |
| Aristocrats | Korvold | 164 | 165 | +1 | wash |
| Tribal | Edgar | 169 | 165 | -4 | small loss |
| Blink | Aminatou | 178 | 174 | -4 | small loss |
| LandsMatter | Lord Windgrace | 155 | 146 | -9 | small loss |

## Interpretation

**The headline result is what the rebalance was DESIGNED to do.** PR
#191's stated intent was "close cross-cutting gaps that apply
uniformly across every archetype, including the midrange fallback used
by every unknown deck." Atraxa Grand Unifier is the only deck in the
pool that hits the midrange fallback (no Freya archetype constant
matches "grand unifier"), and it gained +4.0% — the cleanest possible
confirmation that the late-game LifeResource and ahead-branch
BoardPresence bumps actually do help midrange decks.

**Mill regressed, and Phenax specifically lost 3.7%.** This is the
strongest negative signal. The likely mechanism: the late-game
LifeResource bump teaches the hat to value life retention more in long
games, but Mill decks are happy to trade life for mill progress (they
WIN by emptying opponent libraries, not by surviving). The R1 Mill
profile boosted CardAdvantage / StackInteraction / DrainEngine /
ToolboxBreadth to match Mill's control-variant gameplan, but didn't
explicitly dampen LifeResource — the global cross-cutting bump now
applies on top of Mill's already-low LifeResource (0.5) and shifts
decisions toward life preservation that Mill shouldn't care about. Worth
investigating in a follow-up — either reducing Mill's base LifeResource
to 0.3 to absorb the global bump, or making the late-game LifeResource
multiplier archetype-aware (skip for Mill / Aristocrats / Storm where
life is intentionally tradeable).

**The rest of the pool is statistical noise** (everything between
-1.5% and +1.5% on n≈660 is well within 1σ). Reanimator and Voltron are
flat in aggregate (one deck up, one deck down by similar magnitude).
Spellslinger, Aristocrats, Edgar Markov, Aminatou, and Lord Windgrace
all moved less than 1.5%.

## Verdict

Cross-cutting rebalance lands as designed for the midrange fallback
(+4.0% on the one untuned deck) and is winrate-neutral for 9 of 12
tuned archetypes. The Mill regression is real and points at a specific
interaction worth a follow-up: either dampen Mill's LifeResource
baseline to neutralize the cross-cutting bump, or make the late-game
LifeResource multiplier opt-out per archetype.

No reason to revert PR #191. The Atraxa gain alone is worth the change,
and the Mill regression is fixable via a targeted Mill-profile tweak in
a follow-up rather than rolling back the global rebalance.
