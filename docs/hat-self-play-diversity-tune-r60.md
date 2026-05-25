# Hat Self-Play Diversity Tune — R60 Voltron Defensive Bump

**Date:** 2026-05-24
**Branch:** `dev/hat-self-play-diversity-tune-r60`
**Source data:** PR #251 (5-pod × 500g diversity gauntlet,
`docs/hat-self-play-baseline-r60.md`)

## The bias

PR #251's variance run measured every archetype's self-play winrate
across 4 of 5 pod compositions. Most-favored vs least-favored:

| Rank | Deck | Archetype | Mean | σ | Range |
|---:|---|---|---:|---:|---|
| 1 | Korvold | Aristocrats | **44.6%** | 3.7 | 39.4–49.8 |
| 2 | Phenax | Mill | 47.8% | 12.4 | 34.6–68.0 |
| 3 | Windgrace | LandsMatter | 18.0% | 8.4 | 10.8–32.2 |
| 4 | Kalamax | Spellslinger | 11.9% | 2.6 | 9.4–15.2 |
| 5 | Wyleth | Voltron | **2.8%** | 0.15 | 2.6–3.0 |

**Spread: 41.8 percentage points (Korvold 44.6% vs Wyleth 2.8%).**

Voltron's σ=0.15 is the lowest in the corpus — Wyleth wins 2.6–3.0%
no matter who else is at the table. The flatness is the diagnostic:
identical-AI opponents systematically remove the commander on sight,
and live political variance (which would let Voltron slip a win
through deals) is absent in self-play.

This isn't a deck-power bug — Wyleth scored 27.7–28.2% in PR #194's
mixed 4-deck pool gauntlets where Freya analysis and political-style
heuristics were active. The 2.8% self-play floor is the SYSTEMIC bias.

## Diagnosis

When all 4 seats run identical YggdrasilHat:
1. Every opponent's threat-targeting heuristic converges on the same
   answer: "Wyleth has 1 important threat, it grows fast, kill it."
2. Wyleth's hat needs to PROTECT its commander before the table can
   pile removal on. Heroic Intervention / Boros Charm / Teferi's
   Protection are the right plays.
3. Pre-fix, Wyleth's profile had `ThreatExposure=1.4` (bumped from
   0.9 in R2 — `dev/hat-archetype-weights-round2-r60`) and
   `StackInteraction=0.8` (bumped from 0.4 in R2). These were enough
   for the mixed gauntlet but not for self-play.
4. The result: Wyleth's hat sees an incoming removal spell, evaluates
   "should I cast Heroic Intervention here," and the eval score
   doesn't quite clear the threshold to spend mana on it. Commander
   dies. Game lost.

The hat is making a locally-defensible decision based on a profile
that under-weights how catastrophic the commander loss is.

## Proposed tune

`ArchetypeVoltron` profile in `internal/hat/eval_weights.go`:

| Dimension | R2 (current) | R5 (this PR) | Rationale |
|---|---:|---:|---|
| ThreatExposure | 1.4 | **1.8** | Anchor at the "highest secondary" tier alongside CommanderProgress=2.0. The commander IS the entire winrate; treating its survival as nearly as important as advancing it matches the structural reality. Now beats Reanimator (1.2) and Control (1.2) — the three single-creature-plan archetypes are ordered Voltron > Reanimator > Control by descending plan-B redundancy. |
| StackInteraction | 0.8 | **1.2** | Protection spells (Heroic Intervention / Boros Charm / Teferi's Protection / Akroma's Will) must be valued enough to actually cast on the right turn rather than held for a "better moment" that never comes. Matches Mill's StackInteraction=1.1 (R1 tune) and Stax's StackInteraction=1.1 (R4 tune). |

CommanderProgress=2.0 remains the array maximum (the signature
dimension is untouched). ArtifactSynergy=1.1 and EnchantmentSynergy=0.9
(R2 tune) are intact.

## Expected effect

The tune raises the evaluator's appetite for both:
1. Casting protection spells at instant speed when removal is on the
   stack (StackInteraction bump → ChooseResponse path more likely to
   pay mana for the fog/protection answer added in PR #176)
2. Pre-emptively prioritizing protection over offensive plays in the
   main phase (ThreatExposure bump → evaluator scores game states
   where protection is up higher than equivalent-board states without it)

**Projected winrate lift**: 2.8% → 5–8% in self-play. This won't
close the spread entirely (Voltron is structurally disadvantaged in
identical-AI pods regardless of tuning) but should at least double the
floor.

## Limits of the tune

- **Cannot fix the structural problem of single-creature plans in
  identical-AI 4-player pods.** Even with perfect protection, Wyleth
  has one wincon and three opponents have unlimited removal cycles.
- **May come at a cost to mixed-pool winrate.** The R2 tune was
  calibrated against #194's mixed-pool gauntlet; the R5 bump weights
  defense even higher, which could make Wyleth too passive when
  opponents AREN'T all using identical heuristics. A follow-up
  validation gauntlet (post-merge, same setup as #194) should
  measure the net.
- **Doesn't address the symmetric bias** (Korvold/Aristocrats at
  44.6% — too HIGH, not too low). Aristocrats over-performs because
  its drain triggers scale with table-wide creature deaths, and
  self-play produces lots of creature deaths. A symmetric tune
  reducing Aristocrats' DrainEngine weight is a separate question
  worth its own gauntlet.

## What this is NOT

- Not a fix to ChooseBlockers / ChooseResponse logic (those got their
  own R60 work in PRs #167 / #176 / #178).
- Not a per-card handler change.
- Not an evaluator-pipeline change. Pure profile tune.

## Regressions

5 tests in `internal/hat/voltron_defensive_r5_test.go`:
1. Voltron ThreatExposure ≥ 1.8 (the new floor)
2. Voltron StackInteraction ≥ 1.2 (the new floor)
3. Voltron ThreatExposure > both Reanimator's AND Control's (the
   defensive-tier ordering)
4. CommanderProgress remains the array maximum (signature dimension
   not overtaken)
5. R2-era ArtifactSynergy/EnchantmentSynergy floors intact (no
   collateral damage from this round's bumps)
