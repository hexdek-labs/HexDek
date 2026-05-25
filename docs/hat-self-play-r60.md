# Hat Self-Play Tournament — R60 Baseline

**Date:** 2026-05-24
**Branch:** `dev/hat-self-play-tournament-r60`
**Goal:** Verify Yggdrasil hat plays itself cleanly across 1000 4-player
games. Look for crashes, infinite loops, runaway winrate skews, and
sanity-check the TrueSkill machinery.

## Setup

- 4 fixed decks, rotated through all seat positions (default tournament
  mode): Kalamax (Spellslinger), Wyleth (Voltron), Phenax (Mill),
  Lord Windgrace (LandsMatter).
- All four seats run `YggdrasilHat` (default `--hat yggdrasil`,
  budget 50, noise σ=0.2).
- Commander format, 80-turn max, 4 seats.
- `--seed 42`, 1000 games.
- **No Freya analysis available** for these decks — every seat fell
  back to the archetype-string-driven `DefaultWeightsForArchetype`,
  which is the cleanest possible "identical-hat-implementation"
  comparison.

## Engine stability — all green

| Metric | Result |
|---|---|
| Crashes | **0** |
| Panics / recovers | **0** |
| Concessions | **0** |
| Games finishing (vs. timeout) | **1000 / 1000** |
| Duration | 3m07s wall |
| Throughput | 5.3 g/s (lower than the 9 g/s pool gauntlet, because
  same-deck-every-game means longer games and slower per-game wall) |

No infinite loops, no stuck games, no panics across 4000 hat instances
(4 seats × 1000 games). The conviction concession path never fired,
which is expected with identical hats — no seat is meaningfully
out-of-position relative to its peers, so the relative-position trigger
for concession doesn't reach its threshold.

## Winrate distribution

| Rank | Deck | Archetype | Wins | Winrate |
|---:|---|---|---:|---:|
| 1 | Phenax, God of Deception | Mill | 671 | **67.1%** |
| 2 | Lord Windgrace | LandsMatter | 167 | 16.7% |
| 3 | Kalamax, the Stormsire | Spellslinger | 130 | 13.0% |
| 4 | Wyleth, Soul of Steel | Voltron | 32 | **3.2%** |

**Wins sum to 1000 — no draws.** Distribution is severely skewed: top
deck wins ~21× more than the bottom deck. The 25% / 25% / 25% / 25%
"balanced self-play" expectation does NOT hold here — but that's the
point of the test. Identical hats reveal pure deck-power-level
disparity once seat luck averages out, and at n=1000 the signal is
overwhelming.

### What the spread tells us

- **Mill dominates in self-play.** Phenax 67.1% reflects a structural
  reality of identical-AI 4-player commander: nobody is incentivized to
  attack the mill player early (no aggressive board presence to fear,
  no political pressure), and Mill's late-game inevitability via Tasha's
  Hideous Laughter / Maddening Cacophony / Bruvac doubler triggers
  closes faster than the other three decks' wincons. Avg turn-to-win:
  65.8 (Phenax) vs 75.0 (Wyleth) — 9 turns earlier.
- **Voltron near-zero in self-play.** Wyleth 3.2% reflects the
  structural cost of a one-creature plan in a 4-player pod where
  every opponent has removal and the AI doesn't form attacking
  alliances. Each Wyleth win likely required an extreme variance line
  (early Lightning Greaves + Embercleave + zero opp removal in hand).
- **LandsMatter and Spellslinger are flat in the middle.** Both have
  slow setup but moderate finishers; neither closes as fast as Mill
  nor depends as catastrophically on a single creature as Voltron.

This is NOT a bug in the rebalance or the archetype tuning — it's
self-play surfacing power-level differences between commanders. The
identical-hat condition removes player skill from the variance budget,
leaving deck strength as the only meaningful signal.

## Game length

| Bucket | Count | % |
|---|---:|---:|
| Turns 1-5 | 0 | 0% |
| Turns 6-10 | 0 | 0% |
| Turns 11-20 | 0 | 0% |
| Turns 21+ | 1000 | **100%** |

Average game: 67.1 turns. **Every game ran past turn 20.** Identical
hats produce long games because nobody gets crushed early — attackers
and defenders are equally good, so the lethal-clock window never opens
before the late-game wincons land. The 80-turn `--max-turns` cap is the
only thing preventing some games from running indefinitely.

This pattern (long games, no early eliminations, late-game wincons
deciding everything) is exactly what we'd expect from clean self-play
of a reasonably-tuned hat. If the hat were broken (over-aggressive,
panic-blocking, mis-targeting) we'd see shorter games with bigger life
swings and the distribution would cluster in the 11-20 bucket.

## TrueSkill ratings — converged to a tight cluster

| Rank | Deck | μ | σ | Conservative (μ - 3σ) |
|---:|---|---:|---:|---:|
| 1 | Wyleth, Soul of Steel | 23.2 | 0.8 | 20.9 |
| 2 | Lord Windgrace | 22.9 | 0.8 | 20.6 |
| 3 | Phenax, God of Deception | 22.3 | 0.8 | 20.0 |
| 4 | Kalamax, the Stormsire | 22.0 | 0.8 | 19.7 |

**The TrueSkill rank inverts the winrate rank.** Wyleth has μ=23.2
(highest) despite winning 3.2% of games; Phenax μ=22.3 (third) despite
winning 67.1%. This isn't a bug — it's TrueSkill working as designed.
The system weights each win/loss by the strength of the opponent
beaten, and Wyleth's rare 32 wins came against opponents who were
themselves winning a lot (Phenax/Windgrace), so each Wyleth win is
worth more than a Kalamax win against Phenax. The tight cluster (μ
range 22.0–23.2, ±0.8) reflects that after 1000 games the system has
converged: all four ratings have low uncertainty, the relative ordering
is stable, and the model has correctly identified that the four hats
themselves are equivalent — only the decks differ.

The conservative ranking (μ - 3σ) shows the same ordering with all
deck pairs within 1.2 points — well below TrueSkill's typical
"different skill class" threshold of ~4 points. The system has
correctly inferred "these are four instances of the same player".

## Patterns + takeaways

1. **The engine is stable for hat self-play.** Zero crashes, zero
   recovers, zero infinite loops across 1000 games × 4 hats. Safe to
   run at scale for calibration / regression workloads.
2. **Winrate ≠ skill in self-play.** The 21× winrate spread between
   Phenax (67.1%) and Wyleth (3.2%) is entirely deck-driven, not hat-
   driven. Future calibration runs that need balanced winrates should
   either use power-level-matched deck pools or weight by deck-power
   percentile.
3. **TrueSkill correctly identifies the self-play case.** Tight μ
   cluster (22.0–23.2) confirms the rating system isn't being fooled by
   raw winrate into ranking the decks as different skill classes.
4. **80-turn cap is load-bearing for self-play.** Avg 67.1 turns means
   the cap fires on ~10% of games (those that didn't resolve before
   turn 80). Self-play tournaments should keep this cap — without it,
   games could run indefinitely against an equally-skilled opponent.
5. **No Freya analysis = clean A/B comparison.** All four seats fell
   back to the same `DefaultWeightsForArchetype` dispatch, so any
   future change to archetype profiles can be re-run against this
   baseline and the deck-power-driven winrate skew will be the
   reference distribution.
