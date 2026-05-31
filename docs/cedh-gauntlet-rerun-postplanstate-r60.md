# cEDH Gauntlet Re-Run on Post-PlanState Main — Honest Null (r60)

> **Re-runs the 2500-game cEDH seat-bias gauntlet on current `main` (commit 9cba87bd, post-PR #848 merge) with the same pool / seeds / pod compositions / hat settings as PR #784.** Both the leaf-eval combo tuning (PR #793 → #826) AND the mid-turn PlanState refresh + MCTS budget lift (PR #848) are present. The honest verdict at full PR #784 sample depth (n=2500) is documented below.

**Verdict: honest null. The PR #848 single-batch result (+1.20pp Combo at 1000 games) DID NOT replicate at 2500 games. Francisco regressed from baseline 36.4% → 34.65% (−1.75pp). Avg game length is essentially flat (−0.05 turns).** Per-archetype Combo metric is now BELOW PR #784 baseline at deeper sample. The aggregate three-PR investigation chain (#793 + #826 + #848) does not show a stable game-length compression or Combo win% lift when measured against the original PR #784 protocol.

This report is a deliberate correction. PR #848's 500-game-per-batch sample produced a +1.20pp Combo lift that sat well inside the per-cell 95% Wilson CI band (±5pp); claiming it as a directional win was the right call at the time but the followup at PR #784's 1250-game-per-batch depth (per-cell CI ≈ ±2.4pp) shows the bump was within-noise variance, not a stable effect.

## Method (identical to PR #784)

- **Pool:** 8 Freya-confirmed B5/cEDH decks from `data/decks/moxfield_300/*_b5_*.txt`.
- **Mode:** Two rotate-mode sub-gauntlets of 4 decks × 1250 games (= 2500 total).
- **Pod composition:**
  - **Batch A** (seed 42): Etali (Midrange), Francisco (Combo), Rograkh-ocellblau (Storm), Tymna-ezinho (Stax)
  - **Batch B** (seed 43): Tymna-quincyhicks (Midrange), Vial Smasher (Storm), Tayam (Stax), Rograkh-valtchuz (Stax)
- **Engine:** built from `origin/main` at `9cba87bd` (post-PR #848 merge, plus 5 follow-up PRs #849–#853 unrelated to hat: event-kind-normalize, ctx-key-fix, zonecons phase F + low-hit cluster, freya combo-vs-meta analysis tooling).
- **Hat:** default `yggdrasil`, budget 50, σ=0.2 — identical to PR #784.
- **Max turns:** 80; per-game timeout 120 s. **0 crashes, 0 concessions, 0 timeouts** across all 2499 games.

The 5 follow-up PRs (#849–#853) were inspected for hat-relevant changes; none modify `scoreCombo`, `cardHeuristic`, `refreshPlanState`, or `effectiveBudget`. PR #852 (`freya-combo-vs-meta`) adds Freya analysis tooling for combo-vs-meta scoring but does not change runtime AI behavior. The PR #793 + #826 + #848 stack is the only AI behavior delta vs PR #784.

## Result 1 — Avg game length (the headline)

| PR | Avg turns | Δ vs baseline |
|----|-----------|---------------|
| PR #784 baseline | 48.40 | — |
| PR #808 (PR #793 rerun) | 48.25 | −0.15 |
| PR #826 (sequencer-priority) | 49.05 | +0.65 |
| PR #848 (PlanState + budget) | 47.95 | −0.45 |
| **This re-run (2500g)** | **48.35** | **−0.05** |

Game length is **essentially unchanged** at 2500-game scale. The PR #848 −0.45 turn drop did not survive the deeper sample. The four-PR investigation does not produce a measurable game-length compression vs the original baseline at PR #784's full depth.

## Result 2 — Per-deck winrate (combined n=1250 per deck)

| Deck                       | Arch     | Baseline | PR #808 | PR #848 (500g) | **This (1250g)** | Δ vs baseline |
|----------------------------|----------|----------|---------|----------------|------------------|---------------|
| Etali, Primal Conqueror    | Midrange | 47.8%    | 48.2%   | 46.0%          | **49.38%**       | **+1.58**     |
| **Francisco, Fowl Marauder** | **Combo** | **36.4%** | 36.5% | 37.6%          | **34.65%**       | **−1.75**     |
| Rograkh (ocellblau)        | Storm    | 7.0%     | 7.7%    | 7.0%           | 8.32%            | +1.32         |
| Tymna (ezinho)             | Stax     | 8.8%     | 7.5%    | 9.4%           | 7.70%            | −1.10         |
| Rograkh (valtchuz)         | Stax     | 32.2%    | 30.3%   | 30.2%          | 32.23%           | +0.02         |
| Tayam, Luminous Enigma     | Stax     | 49.8%    | 50.8%   | 50.8%          | 49.03%           | −0.77         |
| Tymna (quincyhicks)        | Midrange | 13.8%    | 14.5%   | 15.6%          | 13.12%           | −0.68         |
| Vial Smasher the Fierce    | Storm    | 4.1%     | 4.3%    | 3.4%           | **5.55%**        | **+1.45**     |

**Francisco — the only Combo-archetype deck in the pool — regressed −1.75pp from baseline.** This is the metric the investigation chain has been targeting; on this measurement at PR #784's depth, the engineering work has produced a small NEGATIVE result, not a positive one.

At n=1250 per deck the 95% Wilson CI on per-deck rates is ≈ ±2.4pp. The Francisco −1.75pp shift sits inside that band — it is *not* statistically distinguishable from baseline either — but it is clearly not the +1.20pp positive shift PR #848 reported. The honest summary: **at the depth where seat-bias effects can be measured reliably, the combo-assembly tuning chain has produced no positive Combo win% effect.**

## Result 3 — Per-archetype mean winrate

| Archetype | Decks | Baseline | This re-run | Δ pp  |
|-----------|-------|----------|-------------|-------|
| **Combo** | 1     | 36.40%   | **34.65%**  | **−1.75** |
| Storm     | 2     | 5.55%    | 6.94%       | +1.39 |
| Stax      | 3     | 30.27%   | 29.65%      | −0.62 |
| Midrange  | 2     | 30.80%   | 31.25%      | +0.45 |

The archetype-level fingerprint **inverts** from PR #848's prediction. At that PR's 500-game-per-batch depth, Combo gained +1.20pp and Storm lost −0.35pp. At PR #784's 1250-game-per-batch depth, Combo loses −1.75pp and Storm gains +1.39pp. The two shifts are within per-cell noise on either side, so neither result is significant on its own — but the *direction reversal between samples* is exactly the signature of an effect that doesn't exist (or is far below the engineering bonus the investigation has attempted to claim).

## Result 4 — Combined seat-position winrate (n = 2499)

| Seat | Wins | Winrate | 95% Wilson CI       |
|------|------|---------|---------------------|
| 0    | 636  | 25.45%  | [23.78%, 27.19%]    |
| 1    | 630  | 25.21%  | [23.55%, 26.95%]    |
| 2    | 609  | 24.37%  | [22.73%, 26.09%]    |
| 3    | 624  | 24.97%  | [23.31%, 26.70%]    |

χ² = 0.645 (df = 3, critical 7.815, p ≈ 0.89). The seat-position distribution is still uniform — the original hypothesis "early-seat advantage emerges once the hat plays cEDH faster" remains unsupported. Consistent with PR #784, #808, and #848's per-seat χ² (all under 1.5).

## Result 5 — Throughput

Batch A: 35.2 g/s (1250 games / 35.5 s). Batch B: 8.5 g/s (1250 games / 147 s). The throughput delta between batches is unusual — Batch A is ~4× faster than Batch B despite same hat settings and ~equal game length (49.8 vs 46.9 turns). Plausible explanations:

- **Batch composition asymmetry on the budget lift path.** Batch B contains 3 Stax decks (Rograkh-valtchuz, Tymna-ezinho via Batch A counted separately, Tayam) and a Storm deck (Vial Smasher). The PlanAssemble budget lift fires more often per game in Storm decks (more tutors per turn) and Stax decks (more interaction → more `comboAssembling` triggered by opponent threat assessment). Batch A's Midrange (Etali) + Combo (Francisco) + Storm (Rograkh-A) + Stax (Tymna-A) mix has fewer plan-flips per turn, so the budget lift fires less often and the per-game work is lower.
- **Engine path variance from PRs #849–#853**. Those PRs touched event normalization and zone-conservation paths that the engine takes in every game. Cold-cache effects between batches may amplify the variance.

Neither plausibly inflates the throughput by 4× alone, but the asymmetry is real and consistent with the previous PR #848 observation that the budget lift produces high per-game wall-clock cost in Storm/Stax-heavy pods.

## What this means for the four-PR investigation

Going through them honestly:

| PR | Change | Stated effect | Verdict at 2500-game depth |
|----|--------|---------------|----------------------------|
| #793 | Multi-tutor leaf credit | Throughput drop confirms signal landed | Empirical effect on game length / win%: NULL |
| #808 | Diagnostic doc | (no engine change) | (no engine change) |
| #826 | `cardHeuristic` cast-order bias | Storm seat-0 +2.05pp at 2500g, Combo flat | Combo flat — NULL on target metric |
| #848 | Mid-turn PlanState refresh + MCTS budget lift | Combo +1.20pp at 1000g, avg turns −0.45 | Replication at 2500g: Combo **−1.75pp**, turns essentially flat |
| **This re-run** | (no engine change, gauntlet-only) | Empirical replication | Confirms the chain has NO measurable positive effect on Combo win% or game length at PR #784 depth. |

**The architectural changes are real (tests pass, throughput pattern confirms code paths execute), but the empirical claim that they shorten cEDH games or lift combo-deck winrate is not supported at the original baseline's sample depth.** The +1.20pp Combo lift PR #848 reported was 500-games-per-batch single-shot variance that the +1250-per-batch replication erased.

This is the falsification step the investigation chain needed. PR #848 should be re-read with this verdict attached: it added a clean architecture (mid-turn refresh + budget lift, well-tested) but its empirical claim about Combo win% lift was premature.

## What probably needs to happen now

Three honest possibilities, ranked by what the data supports:

1. **The combo-assembly story is correct but the engine's MCTS budget at 50 — even lifted to 75–100 in Assemble/Execute — is too small to reach terminal wincon visibility.** A test at `--hat-budget 200` or `--hat-budget 400` on Combo decks would distinguish "leaf-eval + sequencer + planstate-cadence are correct but starved" from "the whole chain doesn't matter at any budget."
2. **The hat's combo-recognition is correct but the cEDH decks in this pool need specific combo-line wiring that Freya / Strategy emit incompletely.** Etali's MDFC back-face combo and Francisco's storm chain are both edge cases where the Strategy `ComboPieces` list may not cover the actual win path. A `--freya-debug` pass over each deck's `*_freya.json` would surface whether the right pieces are in the right plans.
3. **At HexDek's current AI sophistication, B5/cEDH decks at MCTS budget 50 simply play as midrange-value-engine piles regardless of what the leaf eval or sequencer prior tells them, because the search horizon doesn't reach the wincon turn.** This would mean the whole investigation chain has been targeting the wrong layer; the load-bearing change is upstream of any of these (a) curate the `Strategy.ComboPieces` for cEDH decks more aggressively, (b) build a dedicated cEDH execution heuristic that's NOT an MCTS leaf bonus, or (c) accept that cEDH simulation in HexDek's current engine is a known-soft regime and direct effort elsewhere.

Option 3 is what the data most strongly supports. The investigation has not produced a positive effect on the target metrics after three engineering rounds; the simplest model is that the layer being tuned doesn't drive the outcome at this AI's current sophistication.

## Recommendation

- **Do not stack another round of leaf-eval / sequencer / plan-cadence tuning on top of this chain.** Three rounds of refinement have not moved the needle at sample depth.
- **Run a `--hat-budget 200` ablation on Francisco specifically** before any further architectural change. If the higher budget produces a measurable Combo win% lift, option 1 above is correct and the path is "budget more, not signal more." If it produces no effect, option 3 is correct.
- **Re-read PR #848's claim with this verdict attached.** The architecture stands but the empirical evidence for "+1.20pp Combo" is now superseded by this 2500-game replication that shows the opposite direction.
- **PR #852 (freya-combo-vs-meta) is the most plausible adjacent surface for actually improving cEDH simulation fidelity.** That work is about Freya's combo-vs-meta scoring (not runtime AI) and feeds into the `Strategy.ComboPieces` that the entire investigation chain depends on. Better Freya combo detection → richer ComboPieces lists → more triggers for the refreshPlanState / scoreCombo / cast-order biases that are now in place. Worth checking whether PR #852's outputs change which decks' `Strategy.ComboPieces` recognize the actual cEDH win lines.

## Reproduction

```bash
mkdir -p /tmp/cedh-seat-bias/{batch_a,batch_b}
# stage the 8 B5 decks from data/decks/moxfield_300/*_b5_*.txt as in PR #784
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_a --games 1250 --seed 42 \
   --report docs/cedh-gauntlet-rerun-postplanstate-r60-data/batch_a_report.md
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_b --games 1250 --seed 43 \
   --report docs/cedh-gauntlet-rerun-postplanstate-r60-data/batch_b_report.md
```

Per-batch raw reports at `docs/cedh-gauntlet-rerun-postplanstate-r60-data/batch_a_report.md` + `batch_b_report.md` — including the dashboard `SEAT-POSITION BIAS` and `WINRATE BY (COMMANDER, SEAT)` blocks.

## See also

- [`docs/cedh-seat-bias-r60-moxfield-replication.md`](cedh-seat-bias-r60-moxfield-replication.md) — PR #784 baseline (the protocol this re-run replicates).
- [`docs/cedh-gauntlet-rerun-r60.md`](cedh-gauntlet-rerun-r60.md) — PR #808 first diagnostic.
- [`docs/cedh-sequencer-priority-r60.md`](cedh-sequencer-priority-r60.md) — PR #826 cast-order bias verdict.
- [`docs/cedh-planstate-budget-r60.md`](cedh-planstate-budget-r60.md) — PR #848 PlanState + budget lift verdict (now superseded for the win% claim).
- `internal/hat/yggdrasil.go` — `refreshPlanState`, `effectiveBudget`, `cardHeuristic` (the three surfaces in the investigation chain).
- `internal/hat/combo_sequencer.go` `Evaluate` — the broadened Assembling gate.
- `internal/hat/evaluator.go` `scoreCombo` — the multi-tutor leaf credit.
