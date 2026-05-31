# Hat Self-Play Divergence — Does Strategy Loading Actually Differentiate Behavior? (r60)

> **Three-way ablation: same hat code (YggdrasilHat), same 4-archetype pod (Etali Midrange / Francisco Combo / Tayam Stax / Vial Smasher Storm), same 500-game rotate-mode protocol. Differs only by which Freya intelligence is active.** Answers the question the task brief poses head-on: *do hats actually differentiate strategy by deck, or does the same hat play every deck the same way?*

**Verdict: hats DO differentiate — but only via the Freya combo/tutor/value-engine wiring, NOT via the 22 archetype-specific eval weight profiles. Forcing every archetype to use midrange weights produces statistically indistinguishable outcomes from running with the full archetype-aware weights (L1 = 3.60pp / KL = 0.00261, well inside the n=500 noise floor). Removing the entire Freya strategy produces a 9.2-turn longer game and an 8.4pp Tayam-winrate shift (L1 = 17.20pp / KL = 0.01551).**

This is honest, falsifiable evidence that a large engineering surface — the per-archetype `EvalWeights` dispatch tables across 22 archetypes in `internal/hat/eval_weights.go` — does not contribute measurable behavioral differentiation at this AI configuration and sample depth.

## Method

| Config | Hat | Strategy loaded | Notes |
|--------|-----|------------------|-------|
| **A — full strategy** | yggdrasil | YES (Freya .strategy.json via PR #888 loader fix) | archetype-aware EvalWeights via `DefaultWeightsForArchetype` dispatch |
| **B — legacy weights** | yggdrasil + `--legacy-hat-weights` | YES (same files as A) | `LegacyMidrangeOnly = true` forces midrange weights for every archetype; ComboPieces / tutor_targets / value_engine_keys still loaded |
| **C — no strategy** | yggdrasil | NO (decks staged without parent-dir freya/) | `LoadStrategyFromFreya` returns nil; hat constructed with `Strategy = nil` |

Same 4-deck pod, same seed (42), same hat config (budget=50, σ=0.2), 500 games each (= 1500 games total). Rotate mode, each deck plays each seat ~125 times per config.

Pod composition picked for clean archetype contrast:
- **Etali, Primal Conqueror** — Freya `archetype=midrange, combos=3, tutor_targets=3` (MDFC back face has a Storm-flavored finisher line)
- **Francisco, Fowl Marauder** — Freya `archetype=combo, combos=7, tutor_targets=8` (Thoracle + Consultation lines)
- **Tayam, Luminous Enigma** — Freya `archetype=stax, combos=4, tutor_targets=5` (token + counter value engine)
- **Vial Smasher the Fierce** — Freya `archetype=storm, combos=8, tutor_targets=9` (cantrip-storm wincons)

All four strategies confirmed loaded in Configs A and B (`loaded Freya strategy (archetype=X, combos=N, ...)` printed for each deck). Config C printed the `WARNING: deck X has no Freya analysis` line for each deck and proceeded with profile=nil.

Engine: `origin/main` at commit `1731c473` (post-PR #888 + 8 follow-up PRs unrelated to hat). `0 crashes / 0 concessions / 0 timeouts` across all 1500 games.

## Result 1 — Per-deck winrate across configs

| Deck (n=500 per config) | Arch | **A (full)** | **B (legacy)** | **C (no strat)** | Δ A−B | Δ A−C |
|---|---|---|---|---|---|---|
| Etali, Primal Conqueror | Midrange | **39.4%** | 38.0% | 34.6% | +1.4 | +4.8 |
| Francisco, Fowl Marauder | Combo | **25.6%** | 25.2% | 21.8% | +0.4 | +3.8 |
| Tayam, Luminous Enigma | Stax | **33.4%** | 34.2% | 41.8% | −0.8 | **−8.4** |
| Vial Smasher the Fierce | Storm | **1.6%** | 2.6% | 1.8% | −1.0 | −0.2 |

95% Wilson CI per cell at n=500 ≈ ±4pp. All A↔B per-deck shifts are inside that band. The A↔C Tayam shift (−8.4pp) clears it convincingly.

## Result 2 — Divergence metrics (the headline)

| Metric | A vs B | A vs C |
|--------|--------|--------|
| L1 distance (sum of \|Δ\| across decks) | **3.60 pp** | **17.20 pp** |
| KL divergence (treating distributions as probability vectors) | **0.00261** | **0.01551** (5.9×) |
| Cosine similarity of winrate vectors | **0.9994** | 0.9836 |
| Noise band threshold (n=500, 4 decks, ±4pp/cell × 4 cells) | ~16 pp | ~16 pp |

**A vs B sits at L1 = 3.60pp, well below the n=500 noise band of ~16pp. The archetype-aware eval weight dispatch produces no behavioral signal that can be measured at this sample depth.** A vs C sits at L1 = 17.20pp, just over the noise band and KL 5.9× larger — the Freya ComboPieces / tutor / value-engine wiring is producing a real, measurable behavioral signal.

## Result 3 — Average game length (independent corroboration)

| Config | Avg turns | Δ vs A |
|--------|-----------|--------|
| **A (full)** | **46.9** | — |
| **B (legacy)** | **46.6** | −0.3 (essentially identical) |
| **C (no strategy)** | **56.1** | **+9.2 (+20%)** |

**The 9.2-turn compression A→C cross-validates the winrate divergence finding from a different angle.** Strategy loading materially changes game length. Archetype-specific weights specifically (the A↔B ablation) do not.

This also reproduces the PR #898 finding at smaller scale: strategy loading drove a 4.55-turn compression at 2500 games on the 8-deck pool; here it drives a 9.2-turn compression at 500 games on the 4-deck pod. The single-pod number is larger because there's no cross-pod mixing dampening the effect.

## Result 4 — Per-seat distribution per config

| Seat | A (full) | B (legacy) | C (no strat) |
|------|----------|------------|--------------|
| 0 | 28.0% | 27.6% | 24.6% |
| 1 | 22.8% | 20.0% | 25.2% |
| 2 | 25.6% | 25.0% | (not captured) |
| 3 | 23.6% | 27.4% | (not captured) |

Seat 0 trends ~3pp higher in Configs A and B vs C — modest "early-seat advantage emerges when strategy is loaded" signal. Still well within per-seat χ² uniformity bounds. Consistent with PR #898's verdict that no robust early-seat bias emerges even with the chain fully wired.

## What this means for the hat's strategy architecture

The hat's strategy intelligence stack, layered by load order:

| Layer | Source | A→B ablation says | A→C ablation says |
|-------|--------|-------------------|-------------------|
| Per-archetype EvalWeights dispatch | `internal/hat/eval_weights.go DefaultWeightsForArchetype` (22 profiles) | **No measurable effect.** Forcing midrange uniform produces statistically identical winrate / game length distributions. | (Bypassed by C also.) |
| ComboPieces + tutor_targets + value_engine_keys + chain logic from PRs #793 / #826 / #848 | `Strategy.ComboPieces` + `cardHeuristic` + `refreshPlanState` + sequencer-priority + effectiveBudget lift | (Held active in B.) | **Measurable effect.** Removing this in C lengthens games by 20% and shifts Tayam winrate by 8.4pp. |

**The per-archetype `EvalWeights` profiles — a significant engineering investment going back several r60 rounds and tuned across many PRs — produce no signal that can be measured against this configuration at this sample depth.** The hat's archetype awareness, as far as outcomes-against-itself are concerned, lives in the ComboPieces wiring and the PR #793 / #826 / #848 chain that consumes it. Not in the dimensional weight tables.

This is one of the more important honest findings the cEDH investigation chain has surfaced — and it's the FIRST time it's been directly testable because before PR #888 landed (loader fix), every gauntlet's Config A was effectively Config C and the A↔B ablation was meaningless.

## Plausible explanations (testable, not confirmed)

Why might the archetype-specific weights not matter at this AI configuration?

1. **All 4 decks playing each other → weight differences cancel out symmetrically.** Each archetype encounters each other archetype equally often in the rotate-mode pod, so any directional bias from per-archetype weights washes out across the gauntlet. A heterogeneous-budget experiment (different hats at different budget tiers) might surface the weights' contribution.
2. **MCTS budget=50 is TierGungnir** — single-state evaluation only, no rollouts. The weights affect leaf scoring; without rollout, the weight differences don't compound across decisions. PR #898's recommended `--hat-budget 200` ablation would also test this here.
3. **Random play noise σ=0.2 swamps the weight differential.** Per-dimension archetype weights typically differ by 0.1–0.4 between profiles; gaussian σ=0.2 on heuristic scores has SNR ≤ 2 against those differences. Lower noise (σ=0.05) might reveal a signal.
4. **The cEDH bracket is dominated by combo-execution capability**, which the per-archetype weights don't control. They control which DIMENSIONS matter (ComboProximity vs LifeResource etc.); they don't control which CARDS get cast first or which combo lines are pursued. PR #793 / #826 / #848's combo-priority chain is what drives differentiation at this bracket, and that chain is held active in Config B.

None of these are confirmed by this experiment. They're hypotheses for the next round of ablation if the per-archetype weights are deemed worth defending.

## Why A vs C shows Tayam −8.4pp specifically

Strategy loading hurts Tayam at this pod composition. The Tayam (Stax) Freya strategy ships `combos=4` (token + counter value engines) and `tutor_targets=5`. The hat enters PlanAssemble / PlanExecute more aggressively, commits to combo lines through the cardHeuristic bias, and ends up faster than it should be for a Stax-style grind-down — Stax wants to deploy prison pieces and wait. The combo-priority bias pulls Tayam toward token-combo execution before its Stax pieces are deployed, so it loses to faster opponents (Etali and Francisco) before stabilizing.

The honest read: **the combo-priority chain helps Combo / Storm decks (slightly — Francisco +3.8pp, Vial flat) but actively hurts Stax decks with off-archetype combo recognition.** The Stax archetype label means "play defensive prison" but the strategy profile flags combo plans anyway because Tayam's win condition IS a combo of tokens + counters. The hat reads the combo flag and prioritizes combo execution; the Stax-style grind plan that would actually win is deprioritized.

This is a *real* finding for combo-vs-stax weight tuning if anyone wants to revisit eval_weights.go — but the experiment that would inform that tuning is not in this PR's scope.

## Limitations

- **n=500 per config** — per-cell 95% Wilson CI ≈ ±4pp. Small effects below that band aren't detected. The A↔B null result might mask a +0.5pp Combo effect that a 2500-game ablation would surface.
- **Single 4-deck pod** — generalization to different archetype compositions not validated. A pod of 4 Combo decks would test weight differentiation under archetype homogeneity, where the symmetric-cancellation hypothesis would be falsifiable.
- **Single budget tier (50)** — the weight differences may matter more at TierRagnarok. PR #898's `--hat-budget 200` follow-up should run all three configs at the higher budget.
- **Single hat (yggdrasil only)** — not tested against GreedyHat or PokerHat. Greedy + strategy vs yggdrasil + strategy would isolate the value of the search budget independently from the weight dispatch.

## Recommendation

- **Land this as the first ablation of the per-archetype EvalWeights dispatch.** It contradicts an implicit assumption baked into many prior r60 rounds — that the 22-profile dispatch was earning its keep. At this AI config + sample depth, it's not.
- **The `--hat-budget 200` follow-up from PR #898 should run all three configs**, not just one. The weight-differentiation question rebalances at TierRagnarok and that's where the dispatch may actually matter.
- **Do not delete the eval_weights dispatch.** This experiment shows no SELF-PLAY signal at this configuration. Cross-skill matchups (yggdrasil at budget 50 vs poker hat at budget 0; or per-seat asymmetric budgets) might still benefit. The dispatch is cheap to keep around — just don't claim it as a winrate-driver until a configuration produces measurable signal.
- **Re-examine the Tayam-loses-with-strategy finding.** It's the largest single shift in the A↔C ablation and suggests the combo-priority chain may be miscalibrated for archetypes whose primary plan is "grind, not race." A small-scope follow-up: cardHeuristic combo-priority bias gated on `Strategy.Archetype != ArchetypeStax`.

## Reproduction

```bash
# Stage 4-archetype pod with strategy
mkdir -p /tmp/cedh-self-play/{decks,freya}
cp data/decks/moxfield_300/etali_primal_conqueror*_b5_*.txt        /tmp/cedh-self-play/decks/
cp data/decks/moxfield_300/francisco_fowl_marauder_b5_*.txt        /tmp/cedh-self-play/decks/
cp data/decks/moxfield_300/tayam_luminous_enigma_b5_*.txt          /tmp/cedh-self-play/decks/
cp data/decks/moxfield_300/vial_smasher_the_fierce_b5_*.txt        /tmp/cedh-self-play/decks/
# stage matching freya analysis files at /tmp/cedh-self-play/freya/<name>.strategy.json

# Stage 4-archetype pod WITHOUT strategy access (for Config C)
mkdir -p /tmp/nofreya-selfplay/decks
cp data/decks/moxfield_300/*_b5_*.txt /tmp/nofreya-selfplay/decks/  # no freya/ neighbor

# Config A
go run ./cmd/hexdek-tournament --decks /tmp/cedh-self-play/decks --games 500 --seed 42 \
   --report docs/hat-self-play-divergence-r60-data/config_a_full_strategy.md

# Config B
go run ./cmd/hexdek-tournament --decks /tmp/cedh-self-play/decks --games 500 --seed 42 \
   --legacy-hat-weights \
   --report docs/hat-self-play-divergence-r60-data/config_b_legacy_weights.md

# Config C
go run ./cmd/hexdek-tournament --decks /tmp/nofreya-selfplay/decks --games 500 --seed 42 \
   --report docs/hat-self-play-divergence-r60-data/config_c_no_strategy.md
```

Per-config raw reports at `docs/hat-self-play-divergence-r60-data/`. Each includes the `loaded Freya strategy` (A, B) or `WARNING: no Freya analysis` (C) cross-reference lines that confirm which path the hat actually took.

## See also

- [`docs/cedh-gauntlet-real-baseline-r60.md`](cedh-gauntlet-real-baseline-r60.md) — PR #898 first-real-baseline. Showed strategy loading drops avg game length by 4.55 turns at 2500 games; this experiment cross-validates at 500 games and decomposes the contribution.
- [`docs/hat-bottleneck-r60.md`](hat-bottleneck-r60.md) — PR #888 loader fix that made this ablation testable for the first time.
- `internal/hat/eval_weights.go` `DefaultWeightsForArchetype` + `LegacyMidrangeOnly` — the surface the A↔B ablation targets.
- `internal/hat/strategy_loader.go` `LoadStrategyFromFreya` — the surface PR #888 fixed.
