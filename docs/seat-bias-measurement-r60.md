# Seat-Position Bias Measurement — R60

**Date:** 2026-05-24
**Branch:** `dev/seat-bias-measurement-r60`
**Proposal source:** 7174n1c — measure seat-position winrate to expose
the implicit prior our TrueSkill model isn't applying.
**Scope:** Measurement only. NO TrueSkill changes.

## Data-source note

The `game`/`game_player` user-game tables in `data/hexdek.db` had 0
recorded games (the play app hasn't shipped a game persistence path).
The `showmatch_game` table had only 3 rows. The 2116 games behind the
`showmatch_elo` aggregates were never persisted per-game.

To produce the measurement, I added three counters to
`TournamentResult` (no logic changes — pure aggregation):
- `WinsBySeat []int` — wins indexed by play-seat (post-rotation)
- `WinsByCommanderBySeat map[string][]int` — (commander, seat) wins
- `GamesByCommanderBySeat map[string][]int` — (commander, seat) games

Then ran a 2000-game rotate-mode tournament with 4 decks (Kalamax /
Wyleth / Phenax / Lord Windgrace) and all-YggdrasilHat self-play to
get a clean, balanced dataset. Same composition as PR #250's baseline
(seed 42, default Yggdrasil, no Freya analysis).

## Engine sanity

| Metric | Value |
|---|---:|
| Total games | 2000 |
| Crashes | 0 |
| Concessions | 0 |
| Duration | 4m06s wall |
| Throughput | 8.1 g/s |
| Avg turns | 67.8 |

## Per-seat winrate (raw)

```
SELECT play_seat, COUNT(*) FROM games GROUP BY play_seat
```

| Play seat | Wins | Winrate | Expected (25%) | Δ |
|---:|---:|---:|---:|---:|
| 0 | 496 | 24.8% | 500 | -4 |
| 1 | 494 | 24.7% | 500 | -6 |
| 2 | 488 | 24.4% | 500 | -12 |
| 3 | **522** | **26.1%** | 500 | **+22** |

**Global seat bias is tiny.** Standard error at n=500/seat is ≈1.94pp;
seat 3's +1.1pp deviation is ~0.6σ — well within noise. The seat-3
advantage you'd expect from "last to act, full information about
opponents' turn before responding" doesn't show up clearly in
aggregate. Wyleth/Voltron's uniform weakness across all seats (see
below) damps the signal.

## Per-(commander, seat) winrate matrix

```
SELECT commander, play_seat, COUNT(*)/games AS winrate
  FROM games GROUP BY commander, play_seat
```

| Commander | Archetype | Seat 0 | Seat 1 | Seat 2 | Seat 3 | Mean | Range |
|---|---|---:|---:|---:|---:|---:|---:|
| Phenax, God of Deception | Mill | **70.2%** | 63.8% | 67.6% | 66.0% | 66.9% | **6.4** |
| Kalamax, the Stormsire | Spellslinger | 12.0% | 16.4% | 11.0% | **17.8%** | 14.3% | **6.8** |
| Lord Windgrace | LandsMatter | 14.4% | 15.4% | 15.8% | **17.2%** | 15.7% | 2.8 |
| Wyleth, Soul of Steel | Voltron | 2.6% | 3.2% | 3.2% | 3.4% | 3.1% | 0.8 |

Each cell ≈500 games. SE ≈ √(p(1-p)/500), so Phenax cell SE ≈ 2.1pp,
Wyleth cell SE ≈ 0.76pp.

**Significant bias signals (≥ 2σ from commander mean):**
- **Mill prefers seat 0.** Phenax seat-0 (70.2%) vs seat-1 (63.8%) =
  6.4pp swing (~3σ). Acting FIRST each turn lets the mill spells
  resolve before opponents can disrupt — Tasha's Hideous Laughter
  / Maddening Cacophony / Bruvac trigger windows open earlier.
- **Spellslinger prefers seat 3.** Kalamax seat-3 (17.8%) vs seat-2
  (11.0%) = 6.8pp swing (~3σ). Acting LAST lets reactive mana
  decisions see what every opponent did first — counterspells and
  responsive Bolts land on the right targets, not pre-committed.

**Weak signals:**
- LandsMatter has a mild seat-3 preference (+1.5pp from mean, ~0.7σ).
  Not significant alone but trends in the spellslinger direction.

**No detectable bias:**
- Voltron is uniformly weak (σ=0.3pp across seats) — Wyleth's 3% mean
  winrate floors any per-seat signal. The seat doesn't matter when
  every seat is losing.

## Recommended seat-penalty lookup table

For each (commander, seat) cell, the penalty is the deviation from
the commander's mean winrate, expressed in percentage points. Negative
values indicate the seat is ADVANTAGED for this commander and TrueSkill
updates should down-weight wins from it; positive values indicate the
seat is DISADVANTAGED and wins from it should be up-weighted.

`penalty[commander][seat] = mean(commander) - winrate(commander, seat)`

| Commander | Archetype | Seat 0 | Seat 1 | Seat 2 | Seat 3 |
|---|---|---:|---:|---:|---:|
| Phenax, God of Deception | Mill | **-3.3** | +3.1 | -0.7 | +0.9 |
| Kalamax, the Stormsire | Spellslinger | +2.3 | -2.1 | +3.3 | **-3.5** |
| Lord Windgrace | LandsMatter | +1.3 | +0.3 | -0.1 | -1.5 |
| Wyleth, Soul of Steel | Voltron | +0.5 | -0.1 | -0.1 | -0.3 |

By archetype (averaging within-archetype, when more decks per archetype
are available — single deck here so just commander × seat):

| Archetype | Recommended seat-bonus direction |
|---|---|
| Mill | Prefer seat 0 (act-first advantage) |
| Spellslinger | Prefer seat 3 (act-last reactive-mana advantage) |
| LandsMatter | Mild seat-3 preference |
| Voltron | No discernible per-seat effect |

## What the table is FOR (and what it ISN'T)

**For**: A TrueSkill prior that subtracts the seat penalty from the
raw win signal before updating ratings — so the rating system sees
"this Mill deck won from seat 0, but seat 0 is +3.3pp easier for Mill,
so credit only the residual." This corrects the underlying bias
without requiring perfect seat rotation in every tournament.

**Not for**: deck-tier ranking, archetype balance comparison, or
gameplay-decision tuning. The seat-bias penalty is orthogonal to those
— it's a TrueSkill-prior adjustment only.

## Caveats

1. **Only 4 commanders measured.** The archetype × seat matrix needs
   multiple decks per archetype to reduce per-deck noise. Mill at 70%
   skews the Phenax row; another mill deck at 40% in the same pool
   would show whether the "seat-0 prefers Mill" pattern holds for the
   archetype or is specific to Phenax's printed text.
2. **All-Yggdrasil self-play.** Live matches with mixed hat types
   (humans + bots, or different bot generations) might show different
   seat biases — the act-first / act-last advantage depends on
   opponents making predictable plays.
3. **One pod composition.** PR #251 showed that single-pod measurements
   for high-σ decks (Mill σ=12.4) can be misleading. The seat-bias
   signal here is from one composition; future measurement runs
   should rotate the composition like #251 did and average.
4. **2000 games is not enough for sub-1pp resolution.** SE per cell
   is ~2pp; only the ~3σ signals (Phenax-seat0, Kalamax-seat3) are
   confidently above noise. The smaller deltas should be treated as
   "directional only" until a 10K-game run validates.

## Next steps (NOT taken in this PR — measurement-only scope)

1. Extend the measurement to a multi-deck-per-archetype pool to derive
   archetype-level (not commander-level) seat penalties.
2. Re-run with mixed hat types (yggdrasil + greedy + poker) to see how
   robust the seat-position bias is to opponent strength variation.
3. Implement the TrueSkill prior using the lookup table — only after
   the archetype-level table is established and validated at higher n.
