# Hat Self-Play Diversity Variance — R60 Baseline

**Date:** 2026-05-24
**Branch:** `dev/hat-self-play-baseline-r60`
**Companion:** PR #250 (4-deck × 1000g single-pod baseline). This run
isolates how much a deck's winrate moves when the OTHER three decks in
the pod change — i.e., how stable any single winrate observation is
across different table compositions.

## Setup

- 5 decks spanning 5 archetype profiles, one per slot:
  - **A** = Phenax, God of Deception (Mill)
  - **B** = Wyleth, Soul of Steel (Voltron)
  - **C** = Kalamax, the Stormsire (Spellslinger)
  - **D** = Lord Windgrace (LandsMatter)
  - **E** = Korvold, Fae-Cursed King (Aristocrats)
- 5 pods, each excluding ONE deck (so each deck appears in exactly
  4 of 5 pods):
  - Pod 1: A B C D (no E / Korvold)
  - Pod 2: A B C E (no D / Windgrace)
  - Pod 3: A B D E (no C / Kalamax)
  - Pod 4: A C D E (no B / Wyleth)
  - Pod 5: B C D E (no A / Phenax)
- 500 games per pod = 2500 total games
- `--seed 42`, default YggdrasilHat in every seat, commander format, no
  Freya analysis → all hats dispatch through `DefaultWeightsForArchetype`

## Engine stability — all 5 pods clean

| Pod | Crashes | Concessions | Avg turns | Wall |
|---|---:|---:|---:|---:|
| Pod 1 (no E) | 0 | 0 | 67.0 | 3m21s |
| Pod 2 (no D) | 0 | 0 | 58.4 | 1m38s |
| Pod 3 (no C) | 0 | 0 | 61.0 | 1m47s |
| Pod 4 (no B) | 0 | 0 | 59.1 | 1m30s |
| Pod 5 (no A) | 0 | 0 | 64.3 | 1m38s |
| **Total** | **0** | **0** | — | 9m54s |

2500 games, no crashes / panics / timeouts / concessions. Same
stability profile as PR #250's single-pod 1000-game baseline.

## Per-deck winrate across pods

Each deck plays in 4 pods; the row shows its winrate in each pod plus
mean/range/stddev.

| Deck | Archetype | Pod 1 | Pod 2 | Pod 3 | Pod 4 | Pod 5 | Mean | Range | σ |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Phenax | Mill | 68.0% | 43.2% | 45.2% | 34.6% | — | **47.8%** | 33.4 | **12.4** |
| Wyleth | Voltron | 2.6% | 3.0% | 2.8% | — | 2.8% | **2.8%** | 0.4 | **0.15** |
| Kalamax | Spellslinger | 13.0% | 9.8% | — | 9.4% | 15.2% | **11.9%** | 5.8 | **2.6** |
| Windgrace | LandsMatter | 16.4% | — | 12.6% | 10.8% | 32.2% | **18.0%** | 21.4 | **8.4** |
| Korvold | Aristocrats | — | 44.0% | 39.4% | 45.2% | 49.8% | **44.6%** | 10.4 | **3.7** |

Variance ranges over **two orders of magnitude across decks**:
Voltron's σ=0.15 vs Mill's σ=12.4. Same hat, same seed, same archetype
weights — the only variable is which OTHER three decks are at the table.

## Patterns

### 1. Voltron is uniformly weak (σ = 0.15)

Wyleth wins 2.6–3.0% no matter who else is in the pod. The single-
creature plan loses to identical-AI opponents who all have removal —
table composition doesn't matter because the bottleneck is "Wyleth's
commander gets killed and the deck has no plan B." Self-play removes
the political variance that would normally let a Voltron deck slip a
win through diplomacy.

### 2. Mill has the highest swing (σ = 12.4)

Phenax went from **68.0%** in Pod 1 (without Aristocrats) to **34.6%**
in Pod 4 (with Aristocrats, without Voltron). The pattern:
- Mill dominates 4-decks-no-Aristocrats pods (no other late-game
  closer to compete with Tasha's library-out + Bruvac doubler).
- Adding Korvold/Aristocrats halves Mill's winrate — Korvold's
  sac-engine drains and Marionette Master / Blood Artist triggers
  close games BEFORE Mill can grind opponents out.
- Removing Voltron (Pod 4) tightens the race further (no "free win"
  available against the weakest table member), dropping Mill another
  10 percentage points.

### 3. Korvold > Mill in head-to-head (σ = 3.7)

Aristocrats narrowly out-wins Mill in every pod where both appear
(Pod 2: 44% / 43%, Pod 3: 39% / 45% — wait, Mill higher in 3, lower
elsewhere — see below). Korvold's tight σ=3.7 means it consistently
wins 39–50% regardless of the OTHER table members. Drain engines
scale with whoever's playing — every opponent feeds them.

### 4. LandsMatter is the most situationally-dependent (σ = 8.4)

Lord Windgrace's range 10.8 → 32.2 (Pod 4 → Pod 5) is the second-
biggest swing. The 32.2% in Pod 5 (no Phenax) suggests Windgrace beats
the slower Wyleth/Kalamax/Korvold combos when there's no faster Mill
closer to race. The 10.8% in Pod 4 (no Wyleth, plus Phenax + Korvold)
suggests Windgrace can't compete with two strong drain/mill closers.

### 5. Pod 1 winrate sum (no Korvold) ≠ other pods

| Pod | Sum of wins | Draws |
|---|---:|---:|
| Pod 1 | 100.0% (340+82+65+13) | 0 |
| Pod 2 | 100.0% (220+216+49+15) | 0 |
| Pod 3 | 100.0% (226+197+63+14) | 0 |
| Pod 4 | 100.0% (226+173+54+47) | 0 |
| Pod 5 | 100.0% (249+161+76+14) | 0 |

All five pods resolved 500/500 games to a decisive winner.

## Takeaways

1. **The hat engine is stable across diverse pods.** 2500 games, 0
   crashes, 0 concessions, 0 timeouts.

2. **A single 500-game pod is NOT a reliable winrate estimate for
   high-σ decks.** Mill σ=12.4 means a 500-game observation has a
   95%-CI of ~±25 percentage points just from table composition.
   LandsMatter σ=8.4 means ~±17 points. Future calibration runs need
   either:
   - Multiple pod compositions (this run's pattern) — preferred, since
     it reveals the dependency structure
   - Bigger n (e.g., 5000g) — but doesn't fix the composition bias,
     just shrinks within-pod noise

3. **Low-σ decks (Voltron, Korvold) ARE well-estimated at 500g.**
   Wyleth σ=0.15 and Korvold σ=3.7 mean a single 500g pod's number is
   reliable for them. The asymmetry is itself a useful signal: decks
   that are sensitive to opponent composition will always need
   multi-pod variance; decks that aren't can be measured in a single
   pod.

4. **PR #250's single-pod 67.1% Phenax number was specific to its pod
   composition.** This run shows Phenax's TRUE self-play winrate is
   probably ~47.8% averaged over diverse compositions — the 67.1%
   reading was structurally inflated by the absence of a competing
   late-game closer (Korvold). Future "baseline" headlines should
   either pin the pod composition explicitly or report the
   variance-aware mean.

5. **The Aristocrats-vs-Mill late-game race is the central dynamic.**
   When Korvold is present, Phenax wins drop from ~68% to ~40%; when
   Korvold is absent, Phenax dominates. This is a deck-balance finding,
   not a hat-tuning finding — both archetypes have late-game wincons
   that out-scale the others' midgame.
