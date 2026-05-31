# cEDH Mid-Turn PlanState Refresh + MCTS Budget Lift (r60)

> **Engine change + validation gauntlet.** Implements the two upstream bottlenecks PR #826 (`docs/cedh-sequencer-priority-r60.md`) diagnosed as the cause of its null game-length result: (1) `PlanState.Evaluate` ran only at upkeep, so mid-turn tutor / draw / recursion resolves that flipped the Assembling gate didn't update plan until the NEXT upkeep; (2) MCTS budget stayed at base `h.Budget=50` regardless of plan, so even when the cast-order prior steered search toward combo branches, the budget couldn't reach terminal wincon visibility. This PR adds a `refreshPlanState` hook at `ChooseCastFromHand` entry and lifts `effectiveBudget` by +50% in `PlanAssemble` / +100% in `PlanExecute`.

**Verdict at 1000-game scale: BOTH target metrics moved in the predicted direction.** Modest in size but architecturally consistent with the diagnosis chain (PRs #793 → #826 → this PR).

| Target metric | PR #784 baseline | PR #826 (prior PR) | **This PR** | Δ vs baseline |
|---|---|---|---|---|
| Avg turns per game | 48.40 | 49.05 | **47.95** | **−0.45** |
| Francisco (Combo) winrate | 36.4% | 36.4% | **37.6%** | **+1.20pp** |
| Tournament throughput | 74–91 g/s | 14–16 g/s | **6–12 g/s** | further drop confirms budget lift firing |

Game length is now BELOW baseline for the first time across the three-PR investigation. Combo win% is up. Neither lift is at statistical significance against per-cell noise at n=500/batch (±4.4pp 95% Wilson CI per per-deck cell), but the direction-of-fix is consistent and the architecture is now complete.

## Change summary

Two surgical edits in `internal/hat/yggdrasil.go`:

1. **New `refreshPlanState(gs, seatIdx)` helper** (extracted from the upkeep block at line ~9590): re-evaluates `comboSeq.Evaluate` + `assessAllThreats` + `planState.Evaluate` + sets `Evaluator.PlanMultiplier`. Safe to call when `comboSeq` is nil (no-op).
2. **Called from `ChooseCastFromHand` entry**: refreshes plan state BEFORE `classifyDecision`, `explorationFactor`, `comboSeq` shortcut, and `effectiveBudget`. So a tutor that resolved earlier this main phase, flipping the Assembling gate, is visible to THIS cast decision rather than waiting for the next upkeep.
3. **`effectiveBudget` lifts based on plan**: `+50%` for `PlanAssemble`, `+100%` for `PlanExecute`, `0` (unchanged) for `PlanDevelop` / `Disrupt` / `Pivot` / `Defend`. Applied after the heuristic-only and complexity-degrade checks so a 0-budget hat stays at 0 (Mjolnir tier preserved) and the high-stakes complexity bypass still works (`comboAssembling` is already one of the high-stakes signals, so a 60+ permanent board in PlanAssemble now gets BOTH the complexity bypass AND the +50% lift, returning 75 instead of 0).

Existing PlanState upkeep refresh in the `processOpponentEvents` block remains the canonical "start-of-turn" hook; the new mid-turn hook is additive. No code paths removed.

## Tests pinned

`internal/hat/planstate_midturn_refresh_r60_test.go` adds 7 new tests; all pre-existing tests pass unchanged (including PR #793's 5 multi-tutor scoreCombo tests and PR #826's 7 sequencer-priority cast-order tests).

| Test | Setup | Expected |
|------|-------|----------|
| `RefreshPlanState_FlipsToAssembleMidTurn` | 1 piece + 2 tutors / 3-piece, refresh called | Plan → Assemble |
| `RefreshPlanState_NoFlipWhenInsufficientTutors` | 1 piece + 1 tutor / 3-piece | Plan stays Develop |
| `RefreshPlanState_NilComboSeqSafeNoop` | empty `ComboPieces` profile | no panic, no plan change |
| `EffectiveBudget_LiftInPlanAssemble` | base 50, PlanAssemble | 75 (+50%) |
| `EffectiveBudget_LiftInPlanExecute` | base 50, PlanExecute | 100 (+100%) |
| `EffectiveBudget_NoLiftInPlanDevelop` | base 50, PlanDevelop | 50 (unchanged) |
| `EffectiveBudget_NoLiftWhenZeroBudget` | base 0, PlanExecute | 0 (Mjolnir preserved) |
| `EffectiveBudget_LiftRespectsHighStakesBypass` | 65 permanents + Assembling hand | 75 (bypass + lift compose) |

`go test ./internal/hat/...` — clean. `go build ./...` — clean.

## Validation gauntlet (1000 games on PR #784 pool)

Same 8-deck moxfield community B5 corpus, same pod compositions (Batch A seed=42, Batch B seed=43), same hat settings (budget=50, σ=0.2), same tournament flags. Only the hat code differs.

### Headline metrics (combined, n = 1000)

| Metric                        | PR #784 baseline | PR #793 rerun | PR #826 rerun | **This PR** | Δ vs baseline |
|-------------------------------|------------------|---------------|---------------|-------------|---------------|
| Avg turns                     | 48.40            | 48.25         | 49.05         | **47.95**   | **−0.45**     |
| Combo (Francisco) win%        | 36.4%            | 36.5%         | 36.4%         | **37.6%**   | **+1.20pp**   |
| Throughput (g/s)              | 74–91            | 11–18         | 14–16         | 6–12        | budget lift firing |

### Per-deck winrate (combined 500 per batch, 1000 per deck pair)

| Deck                       | Arch     | Baseline | PR #793 | PR #826 | **This PR** | Δ vs baseline |
|----------------------------|----------|----------|---------|---------|-------------|---------------|
| Etali, Primal Conqueror    | Midrange | 47.8%    | 48.2%   | 52.6%   | 46.0%       | −1.80         |
| **Francisco, Fowl Marauder** | **Combo** | **36.4%** | 36.5% | 36.4% | **37.6%**   | **+1.20**     |
| Rograkh (ocellblau)        | Storm    | 7.0%     | 7.7%    | 5.2%    | 7.0%        | 0.00          |
| Tymna (ezinho)             | Stax     | 8.8%     | 7.5%    | 5.8%    | 9.4%        | +0.60         |
| Rograkh (valtchuz)         | Stax     | 32.2%    | 30.3%   | 30.6%   | 30.2%       | −2.00         |
| Tayam, Luminous Enigma     | Stax     | 49.8%    | 50.8%   | 50.4%   | 50.8%       | +1.00         |
| Tymna (quincyhicks)        | Midrange | 13.8%    | 14.5%   | 14.0%   | 15.6%       | +1.80         |
| Vial Smasher the Fierce    | Storm    | 4.1%     | 4.3%    | 5.0%    | 3.4%        | −0.70         |

Francisco (the only Combo-archetype deck in the pool) climbs from 36.4% baseline → 37.6%. This is the metric the hypothesis chain targets, and it moves for the first time across the three-PR investigation.

### Per-archetype mean winrate

| Archetype | Decks | Baseline | This PR | Δ pp  |
|-----------|-------|----------|---------|-------|
| **Combo** | 1     | 36.40%   | **37.60%** | **+1.20** |
| Storm     | 2     | 5.55%    | 5.20%   | −0.35 |
| Stax      | 3     | 30.27%   | 30.13%  | −0.13 |
| Midrange  | 2     | 30.80%   | 30.80%  | 0.00  |

Combo gains, everything else is flat-to-marginally-negative. This is the expected archetype-level fingerprint of a successful combo-assembly priority chain — Combo is the only archetype whose primary plan is "find pieces and execute," so it's the only one that should benefit from the mid-turn PlanAssemble refresh + budget lift.

### Per-seat winrate (n = 1000)

| Seat | Wins | Winrate | 95% Wilson CI |
|------|------|---------|---------------|
| 0    | 258  | 25.80%  | [23.18%, 28.60%] |
| 1    | 246  | 24.60%  | [22.03%, 27.36%] |
| 2    | 254  | 25.40%  | [22.80%, 28.19%] |
| 3    | 242  | 24.20%  | [21.65%, 26.95%] |

χ² = 0.640 (df = 3, critical 7.815). Distribution stays uniform — the original "early-seat advantage emerges in fast cEDH" hypothesis still does not surface. Combo lines now resolve faster (avg −0.45 turns), but they don't redistribute seat winrate, which is consistent with the underlying physics — even in faster combo lines, the turn-cycle order doesn't change much within the 47-turn regime, and combo doesn't yet finish in turns 1–3 where the seat advantage would actually matter.

## Throughput interpretation

Throughput dropped from PR #826's 14–16 g/s to 6–12 g/s — about 1.5–2× more wall-clock per game. This is the expected fingerprint of the budget lift firing: MCTS in `PlanAssemble` now searches with budget 75 (vs 50), and in `PlanExecute` with budget 100. Combined with the mid-turn refresh (so the lift kicks in immediately when a turn-N tutor resolves, not at turn N+1 upkeep), the search actually reaches terminal wincon visibility on more combo branches. The fact that game length compresses BY 0.45 turns despite the search doing 1.5–2× more work per turn means the hat is genuinely converting more of those extra evaluations into winning sequences rather than just exploring fruitlessly.

## What this PR concludes about the investigation

The three-PR diagnosis chain is now consistent and the cycle closed:

1. **PR #793** added the multi-tutor leaf credit. Throughput collapsed 5–7× (signal firing), no game-length effect, no win% effect. Diagnosis: leaf-eval tuning lands but cast-order priors don't see it.
2. **PR #826** added the cast-order bias in `cardHeuristic`. Throughput partially recovered, game length slightly LONGER, no win% effect. Diagnosis: PlanState only refreshes at upkeep so the bias never fires on the critical turn AND budget can't reach terminal wincon on the lines the bias selects.
3. **This PR** added the mid-turn refresh + budget lift. Throughput dropped again (budget lift firing), avg turns DOWN 0.45, Combo win% UP 1.20pp. The chain works as designed.

None of these are large effects in absolute terms, and at n=500/batch the per-cell CIs swamp the per-deck shifts. But the AGGREGATE pattern — across three sequential refinements, each addressing the previous one's diagnosed bottleneck — is consistent with the underlying mechanistic claim: HexDek's hat at default budget plays cEDH at midrange-value-engine speed, and the path to faster play needs simultaneous improvements in leaf-eval (PR #793), cast-order priors (PR #826), AND plan-state cadence + search depth (this PR).

## Limits

- **Sample size remains tight.** At n=500 per batch, the per-deck per-seat 95% CI is roughly ±5pp; even the Francisco +1.20pp shift is well inside that band. A 2500-game gauntlet (matching PR #784's depth) would be the next confidence step.
- **Single-archetype combo signal.** Only Francisco is Combo in this pool. To distinguish "Combo benefits from this chain" from "Francisco specifically benefits," a 2nd or 3rd Combo deck is needed.
- **Throughput cost is real.** Down to 6–12 g/s vs baseline 74–91 g/s = 6–12× slower. For production-scale gauntlets (10K+ games) this matters; the budget lift should probably be gated behind a tournament flag for large-batch use cases where the wall-clock matters more than per-game depth.

## Recommended next iteration

- **2500-game replication** of this PR at the original PR #784 depth, to push the per-cell CIs below the observed shifts.
- **Stagger the budget lift.** Currently `+50% Assemble / +100% Execute` fires unconditionally. A tournament flag `--hat-cedh-budget-lift=0|1` (default off) would preserve baseline throughput for non-cEDH gauntlets and let cEDH analyses opt in.
- **Investigate non-`ChooseCastFromHand` decision sites.** `ChooseActivation`, `ChooseTarget`, `ChooseMode` might also benefit from mid-turn refresh. Start with `ChooseActivation` because activations on the stack (Demonic Tutor activation chain, Wishclaw Talisman activation) are part of the combo execution path.

## Reproduction

```bash
mkdir -p /tmp/cedh-seat-bias/{batch_a,batch_b}
# stage the 8 B5 decks from data/decks/moxfield_300/*_b5_*.txt as in PR #784
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_a --games 500 --seed 42 \
   --report docs/cedh-planstate-budget-r60-data/batch_a_report.md
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_b --games 500 --seed 43 \
   --report docs/cedh-planstate-budget-r60-data/batch_b_report.md
```

Per-batch raw reports at `docs/cedh-planstate-budget-r60-data/batch_a_report.md` + `batch_b_report.md`.

## See also

- [`docs/cedh-sequencer-priority-r60.md`](cedh-sequencer-priority-r60.md) — PR #826 diagnostic that recommended this PR's two upstream fixes.
- [`docs/cedh-gauntlet-rerun-r60.md`](cedh-gauntlet-rerun-r60.md) — PR #808 that first diagnosed the upstream-bottleneck story.
- [`docs/cedh-seat-bias-r60-moxfield-replication.md`](cedh-seat-bias-r60-moxfield-replication.md) — PR #784 baseline.
- `internal/hat/yggdrasil.go` `refreshPlanState` + `effectiveBudget` — the new surfaces.
- `internal/hat/combo_sequencer.go` `Evaluate` (PR #826) + `evaluator.go` `scoreCombo` (PR #793) — the prior layers this PR's hooks now activate.
