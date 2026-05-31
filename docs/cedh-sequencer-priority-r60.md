# cEDH Sequencer-Priority Cast-Order Bias (r60)

> **Engine change + validation gauntlet.** Implements the sequencer-side wedge recommended by [`docs/cedh-gauntlet-rerun-r60.md`](cedh-gauntlet-rerun-r60.md) (PR #808): broaden `ComboAssessment.Assembling` to fire whenever `realPiecesFound ≥ 1 AND tutorsInHand ≥ missing`, and add a cast-order bias in `cardHeuristic` so combo pieces and tutors are picked ahead of value engines when the plan is `PlanAssemble` or `PlanExecute`. Validates with a 1000-game cEDH gauntlet (2 × 500 games on the PR #784 8-deck moxfield B5 pool).

**Verdict at 1000-game scale: signal lands architecturally, does NOT drive game-length compression.** This is an honest null result — the change is structurally correct and pushes the cast queue toward wincon assembly when the broadened gate fires, but at MCTS budget = 50 and default noise σ = 0.2 it is not large enough to measurably shrink avg turn count or lift combo-deck winrate above per-cell sampling noise on a 500-games-per-batch sample.

## Change summary

Two surgical edits:

1. **`internal/hat/combo_sequencer.go` `Evaluate`** — `Assembling` gate broadened from `missing == 1 AND hasTutorInHand` to `realPiecesFound ≥ 1 AND missing ≥ 1 AND tutorsInHand ≥ missing`. The pre-tuning gate only caught the boundary case where exactly one piece was missing; the broadened gate fires on the canonical cEDH multi-tutor reach pattern (1 anchor + 2 tutors / 3-piece combo). The anchor-required guard stays intact (a tutor-only hand never claims Assembling).
2. **`internal/hat/yggdrasil.go` `cardHeuristic`** — appended cast-order bias block: when `planState.Current ∈ {PlanAssemble, PlanExecute}`, combo pieces score `+0.40`, tutors `+0.35`, non-combo value engines `−0.15`. Sizes tuned so a combo piece outranks a same-CMC value engine by ~0.55 — strictly larger than any single archetype/category bonus elsewhere in the function — so the bias is decisive when the broadened Assembling gate is firing. Tutor detection mirrors `hasTutorInHand`: `isTutor(c) OR oracle-text contains "search your library"`.

Both changes are scoped to the existing PlanAssemble / PlanExecute flow; PlanDevelop (the default) is unaffected.

## Test contracts pinned

`internal/hat/combo_sequencer_priority_r60_test.go` adds 7 new tests; all pre-existing tests pass unchanged.

| Test | Setup | Expected | Why |
|------|-------|----------|-----|
| `AssemblingBroadenedToMultiTutorReach` | 1 piece + 2 tutors / 3-piece combo | `Assembling = true` | Headline gate change. Pre-tuning would set false (missing == 2). |
| `NotAssemblingTutorsBelowMissing` | 1 piece + 1 tutor / 3-piece combo | `Assembling = false` | Defends against over-broadening — tutors must cover all missing slots. |
| `NotAssemblingZeroPieces` | 0 pieces + 3 tutors / 3-piece combo | `Assembling = false` | Anchor-required guard intact. |
| `ComboPriorityBiasInPlanAssemble` | Combo piece vs value engine in PlanAssemble | `combo > value`, gap ≥ 0.40 | Cast-order bias active. |
| `TutorPriorityBiasInPlanAssemble` | Tutor vs value engine in PlanAssemble | `tutor > value` | Tutors rank above demoted value engines. |
| `ComboPriorityBiasNotInPlanDevelop` | Combo piece vs value engine in PlanDevelop | gap < 0.40 | Bias correctly gated by plan state. |
| `ComboPieceOverridesValueEngine` | Card listed as BOTH combo piece AND value engine | scores at combo +0.40, not value −0.15 | Switch-order semantics — combo-piece branch fires first. |

Existing scoreCombo tests (5 zone+tutor + 5 multi-tutor + sequencer Assembling boundary + NotAssembling no-tutor) all pass without modification.

## Validation gauntlet (1000 games on PR #784 pool)

Pool, seeds, batches, and tournament flags are identical to PR #784 / PR #808. Only the hat code differs.

### Result 1 — Game length

| Metric                  | PR #784 baseline | PR #793 rerun | This PR (1000 games) | Δ vs baseline |
|-------------------------|------------------|---------------|----------------------|---------------|
| Avg turns               | 48.40            | 48.25         | **49.05**            | **+0.65**     |
| Batch A avg turns       | 50.0             | 49.6          | 51.1                 | +1.1          |
| Batch B avg turns       | 46.8             | 46.9          | 47.0                 | +0.2          |

**Avg game length did not compress.** Batch A drifted slightly longer; Batch B is essentially unchanged. The sequencer-priority bias is firing (tests confirm), and throughput is back up to 14.5–16.3 g/s (vs PR #793's 11–18 — MCTS is no longer expanding combo subtrees as deeply because the heuristic prior steers the search to lines that resolve and prune cleanly). But the actual decision the hat returns on most turns is not shifting enough to shorten games.

### Result 2 — Per-deck winrate (n = 500 per batch)

| Deck                       | Arch     | Baseline | This PR | Δ pp     |
|----------------------------|----------|----------|---------|----------|
| Etali, Primal Conqueror    | Midrange | 47.8%    | 52.6%   | **+4.80**|
| Francisco, Fowl Marauder   | Combo    | 36.4%    | 36.4%   | 0.00     |
| Rograkh (ocellblau)        | Storm    | 7.0%     | 5.2%    | −1.80    |
| Tymna (ezinho)             | Stax     | 8.8%     | 5.8%    | −3.00    |
| Rograkh (valtchuz)         | Stax     | 32.2%    | 30.6%   | −1.60    |
| Tayam, Luminous Enigma     | Stax     | 49.8%    | 50.4%   | +0.60    |
| Tymna (quincyhicks)        | Midrange | 13.8%    | 14.0%   | +0.20    |
| Vial Smasher the Fierce    | Storm    | 4.1%     | 5.0%    | +0.90    |

**Combo (Francisco) is exactly flat at 36.4%.** The hypothesis — "combo deck winrate climbs" — is NOT confirmed.

At n = 500 per cell the 95% Wilson CI on individual per-deck cells is roughly ±4.4pp for p ≈ 0.25 — so the Etali +4.80pp and Tymna-ezinho −3.00pp shifts are at the noise floor. Everything else is well inside it. There is no archetype where the change produces a statistically distinguishable gain.

### Result 3 — Per-archetype mean winrate

| Archetype | Decks | Baseline | This PR | Δ pp  |
|-----------|-------|----------|---------|-------|
| Combo     | 1     | 36.4%    | 36.4%   | 0.00  |
| Storm     | 2     | 5.6%     | 5.1%    | −0.45 |
| Stax      | 3     | 30.3%    | 28.9%   | −1.37 |
| Midrange  | 2     | 30.8%    | 33.3%   | +2.50 |

The most pronounced shift is **Midrange +2.50pp**, not Combo. Plausibly because Etali (`Etali, Primal Conqueror // Etali, Primal Sickness`) carries a combo plan in its Strategy profile (the MDFC back face's "exile-cast top six and play them" enables Storm-flavored chains) and the broadened Assembling gate fires on Etali's draws more often than baseline. Combo (Francisco) is flat, and Storm + Stax drift down slightly. **The sequencer-priority bias is not selectively helping the archetypes the hypothesis predicted it would help.**

### Result 4 — Seat-position winrate

| Seat | Baseline (n=2500) | This PR (n=1000) | Δ pp     |
|------|-------------------|------------------|----------|
| 0    | 25.64%            | 25.60%           | −0.04    |
| 1    | 24.44%            | 23.50%           | −0.94    |
| 2    | 24.40%            | 24.60%           | +0.20    |
| 3    | 25.52%            | 26.30%           | +0.78    |

χ² ≈ 1.51 (df = 3), essentially the same as PR #784's 1.354 — distribution still indistinguishable from uniform at p ≈ 0.68. No early-seat bias emerged. The original hypothesis (seat 0 advantage in fast cEDH) remains unsupported in this engine.

## Diagnosis — why is the architectural change not moving the empirical needle?

Three plausible upstream bottlenecks, ranked by likelihood:

1. **PlanState transitions happen once per turn at upkeep.** `PlanState.Evaluate` is called from the main MCTS entry (`yggdrasil.go:9553-9554`), not on every cast-decision sub-call. Within a single turn the hat may have 5–10 decisions but the plan is locked at upkeep. If `Assembling` doesn't fire on upkeep (e.g., the multi-tutor reach hand arrives mid-turn after a draw or recursion), the cast-order bias never activates for that turn.
2. **MCTS budget = 50 is too small to convert leaf-eval shifts into terminal wincon visibility.** PR #808 reported a 5–7× throughput drop confirming the search was expanding combo subtrees deeper. This PR has throughput back near baseline (14–16 g/s vs baseline 74–91), suggesting MCTS is *terminating* combo branches faster now — but possibly because the cast-order prior steers it to lines that the budget = 50 search cannot actually finish to a terminal win state.
3. **Bias size (+0.40 / −0.15) vs noise σ = 0.2.** SNR ≈ 2.0 per decision; with 5–10 sequential cast decisions per turn, the bias accumulates but individual sub-decisions still flip on noise. A larger bias (+0.80 / −0.30) might convert more single-turn outcomes but also risks over-prioritization in mixed-archetype decks like Etali (which already drifted +4.8pp on the smaller bias).

The architecturally-clean follow-up — what would *actually* compress game length:

- **Drive `PlanState.Evaluate` from cast-decision callbacks**, not just upkeep. When `Assembling` flips mid-turn (post-draw, post-recursion, post-tutor-resolve), the next cast decision should reflect that immediately. Currently a turn-N tutor that fetches the missing piece doesn't update plan state until turn N+1 upkeep.
- **Budget-aware combo assembly.** When `Assembling` is true, temporarily lift MCTS budget by 50–100% for cast decisions until the combo executes or the plan times out. This trades search time for actually finding the wincon.
- **Direct sequencer integration in `ChooseSpellToCast`**, not via `cardHeuristic`. The current path uses `cardHeuristic` as a prior on MCTS expansion; a more aggressive integration would have `Assembling`'s `MissingPiece` and `NextAction` directly preempt the search when the line is castable.

## Recommendation

Land the architectural change (Assembling gate + cast-order bias) because both are clean, well-tested, and consistent with the cEDH model. **Do NOT claim it ships the game-length compression goal** — the 1000-game gauntlet establishes that at default MCTS budget the empirical effect is null. The next iteration of this investigation should target the upstream bottlenecks (plan-state cadence + MCTS budget) before attempting another signal-side tweak.

## Reproduction

```bash
mkdir -p /tmp/cedh-seat-bias/{batch_a,batch_b}
# stage the 8 B5 decks from data/decks/moxfield_300/*_b5_*.txt as in PR #784
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_a --games 500 --seed 42 \
   --report docs/cedh-sequencer-priority-r60-data/batch_a_report.md
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_b --games 500 --seed 43 \
   --report docs/cedh-sequencer-priority-r60-data/batch_b_report.md
```

Per-batch raw reports at `docs/cedh-sequencer-priority-r60-data/batch_a_report.md` + `batch_b_report.md`.

## See also

- [`docs/cedh-gauntlet-rerun-r60.md`](cedh-gauntlet-rerun-r60.md) — PR #808 diagnostic that recommended this work.
- [`docs/cedh-seat-bias-r60-moxfield-replication.md`](cedh-seat-bias-r60-moxfield-replication.md) — PR #784 baseline.
- [PR #793](https://github.com/hexdek-labs/HexDek/pull/793) — leaf-eval multi-tutor credit (companion to this PR's sequencer-side change).
- `internal/hat/combo_sequencer.go` `Evaluate` — the Assembling gate.
- `internal/hat/yggdrasil.go` `cardHeuristic` — the cast-order bias.
- `internal/hat/gameplan.go` `PlanState.Evaluate` — the once-per-turn upstream bottleneck.
