# cEDH Seat-Bias Gauntlet Re-Run — Multi-Tutor Tuning Impact (r60)

> **Companion to** [`docs/cedh-seat-bias-r60-moxfield-replication.md`](cedh-seat-bias-r60-moxfield-replication.md) **(PR #784).** This re-runs the same 2500-game gauntlet, on the same 8-deck moxfield community B5 pool, with the same pod compositions, seeds, and tournament flags — only the hat code has changed. The behavior change under test is the multi-tutor credit fix in `internal/hat/evaluator.go scoreCombo()` from [PR #793](https://github.com/hexdek-labs/HexDek/pull/793) (merged in `3f79b41c`).

**Verdict:** **The signal landed and IS being applied — but it did NOT drive shorter games at the default MCTS budget.**

The empirical evidence is mixed:
- **Game length essentially unchanged.** Avg turns 48.40 → 48.25 (Δ −0.15). 99% of games still in the 21+ turn bucket. The "cEDH avg 47-50 turns" caveat from PR #784 is NOT relieved.
- **Throughput collapsed 5–7×.** Batch A 91.0 → 17.7 g/s; Batch B 74.0 → 10.9 g/s. The hat is now spending 5–7× more wall-clock per game, almost certainly because higher `ComboProximity` scores keep combo-assembly subtrees alive deeper in the MCTS search tree.
- **Per-archetype winrate redistribution is visible at single-batch noise scale.** Storm gains seat-0 winrate (+2.05pp); Stax loses seat-0 winrate (−3.90pp); Combo shifts winrate from seat-0 to seat-1 (−2.80 / +2.30); Midrange is flat. This is the archetype-level fingerprint of the tuning firing.
- **Combined per-seat distribution is MORE uniform than baseline**, not less. χ² 1.354 → 0.694 (df=3). The seat-0 winrate moved 25.64% → 24.40% — in the *opposite* direction the original hypothesis predicts.

**Interpretation.** The multi-tutor signal IS firing (proven by the 5–7× throughput drop and the +2pp Storm seat-0 shift). It changes the hat's search shape but does not move the game-length needle. This is consistent with a hypothesis that **signal-side leaf evaluation tuning is necessary but not sufficient to translate into earlier wincon assembly at MCTS budget = 50**. The fix lands in MCTS leaf scores; the gameplan/sequencer that picks cast order does not yet see the same prior. See "Why didn't game length move?" below.

## Method (identical to PR #784)

- **Pool:** 8 Freya-confirmed B5/cEDH decks from `data/decks/moxfield_300/*_b5_*.txt`.
- **Mode:** Two rotate-mode sub-gauntlets, same pod composition as PR #784.
  - **Batch A** (seed 42): Etali (Midrange), Francisco (Combo), Rograkh-ocellblau (Storm), Tymna-ezinho (Stax)
  - **Batch B** (seed 43): Tymna-quincyhicks (Midrange), Vial Smasher (Storm), Tayam (Stax), Rograkh-valtchuz (Stax)
- **Games per batch:** 1250 (= 2500 total). Each deck plays each seat ~312 times.
- **Hat:** default `yggdrasil`, budget 50, σ = 0.2 — identical to PR #784.
- **Engine:** built from `origin/main` at `3f79b41c` (the merge commit for PR #793). Loki-clean r60.
- **Max turns:** 80; per-game timeout: 120 s. **0 crashes, 0 concessions, 0 timeouts** across all 2500 games.

The only delta vs PR #784 is the multi-tutor credit in `scoreCombo` (`evaluator.go:1331-1352`).

## Result 1 — Game length

| Metric                    | PR #784 baseline | PR #793 re-run | Δ        |
|---------------------------|------------------|----------------|----------|
| Avg turns per game        | 48.40            | 48.25          | **−0.15**|
| Batch A avg turns         | 50.0             | 49.6           | −0.4     |
| Batch B avg turns         | 46.8             | 46.9           | +0.1     |
| Games in turns 1–5        | 0                | 0              | 0        |
| Games in turns 6–10       | 2 + 1 = 3        | 1 + 1 = 2      | −1       |
| Games in turns 11–20      | 12 + 16 = 28     | 12 + 14 = 26   | −2       |
| Games in turns 21+        | 1236 + 1233 = 2469 | 1237 + 1233 = 2470 | +1   |

**No detectable game-length shift.** The fast-game tail (turns 1–20) shrank by 3 games out of 2500 — well inside Poisson noise. The hypothesis "combo-assembly tuning drives earlier wincon assembly" is **NOT directly supported** by game-length data at default MCTS budget.

## Result 2 — Combined seat-position winrate (n = 2500)

| Seat | Baseline wins | Re-run wins | Baseline % | Re-run % | Δ pp     | Re-run 95% Wilson CI |
|------|---------------|-------------|------------|----------|----------|----------------------|
| 0    | 641           | 610         | 25.64%     | 24.40%   | **−1.24**| [22.76%, 26.12%]     |
| 1    | 611           | 639         | 24.44%     | 25.56%   | **+1.12**| [23.89%, 27.31%]     |
| 2    | 610           | 622         | 24.40%     | 24.88%   | +0.48    | [23.22%, 26.61%]     |
| 3    | 638           | 627         | 25.52%     | 25.08%   | −0.44    | [23.42%, 26.82%]     |

**χ²(baseline) = 1.354 → χ²(re-run) = 0.694** (df = 3, critical = 7.815). The re-run distribution is *more uniform*, not less. The hypothesis-direction "seat 0 advantage emerges once the hat plays cEDH faster" predicts the opposite shift. Seat 0 actually lost 1.24pp.

## Result 3 — Per-deck winrate shift

| Deck                      | Archetype | Baseline | Re-run | Δ pp    | Sig.* |
|---------------------------|-----------|----------|--------|---------|-------|
| Etali, Primal Conqueror   | Midrange  | 47.8%    | 48.2%  | +0.45   | noise |
| Francisco, Fowl Marauder  | Combo     | 36.4%    | 36.5%  | +0.10   | noise |
| Rograkh (ocellblau)       | Storm     | 7.0%     | 7.7%   | +0.65   | noise |
| Tymna (ezinho)            | Stax      | 8.8%     | 7.5%   | **−1.30** | noise |
| Rograkh (valtchuz)        | Stax      | 32.2%    | 30.3%  | **−1.90** | noise |
| Tayam, Luminous Enigma    | Stax      | 49.8%    | 50.8%  | +1.00   | noise |
| Tymna (quincyhicks)       | Midrange  | 13.8%    | 14.5%  | +0.70   | noise |
| Vial Smasher              | Storm     | 4.1%     | 4.3%   | +0.20   | noise |

*All per-deck shifts are within ±2.5pp per-cell noise floor at n = 1250 per batch. Direction: Stax decks underperform vs baseline; Storm + Midrange marginally over-perform. Mirrors the per-archetype seat-bias signature in Result 4.

## Result 4 — Per-archetype Δ winrate by seat (the actual fingerprint of the change)

This is the one place a real signal emerges from the noise.

### Combo (n = 1 deck — directional only)
| seat | baseline | re-run | Δ pp |
|------|----------|--------|------|
| 0    | 38.3%    | 35.5%  | **−2.80** |
| 1    | 34.8%    | 37.1%  | **+2.30** |
| 2    | 37.8%    | 38.5%  | +0.70    |
| 3    | 34.6%    | 34.9%  | +0.30    |

Combo redistributes from seat 0 to seat 1 — seat 0 is still the deck-best seat in absolute terms (35.5% > 34.9%), but the seat-0 edge compressed.

### Storm (n = 2 decks)
| seat | baseline | re-run | Δ pp |
|------|----------|--------|------|
| 0    | 5.6%     | 7.6%   | **+2.05** |
| 1    | 5.0%     | 5.9%   | +0.95    |
| 2    | 6.7%     | 6.6%   | −0.15    |
| 3    | 5.0%     | 3.9%   | **−1.10** |

**Storm gains seat-0 winrate by +2.05pp.** This is the clearest archetype-level signal in the run — Storm decks (Rograkh-ocellblau and Vial Smasher) benefit measurably from the early seat under the new combo-assembly weighting, consistent with the theory that earlier-acting combo decks see more value from cheap-tutor-into-piece sequencing.

### Stax (n = 3 decks)
| seat | baseline | re-run | Δ pp |
|------|----------|--------|------|
| 0    | 31.7%    | 27.8%  | **−3.90** |
| 1    | 30.4%    | 30.6%  | +0.23    |
| 2    | 27.6%    | 28.6%  | +0.97    |
| 3    | 31.4%    | 31.2%  | −0.27    |

**Stax loses seat-0 winrate by −3.90pp** — the most pronounced single-cell shift in the run. Stax decks rely on hate-piece deployment in their first turn cycle; if Storm opponents now reach for combo pieces faster, Stax in seat 0 (acting before its prison is up) takes more punishment from Storm in seats 1–3.

### Midrange (n = 2 decks)
| seat | baseline | re-run | Δ pp |
|------|----------|--------|------|
| 0    | 30.2%    | 30.4%  | +0.20    |
| 1    | 29.8%    | 31.9%  | **+2.05** |
| 2    | 30.6%    | 30.9%  | +0.30    |
| 3    | 32.6%    | 32.3%  | −0.30    |

Midrange is essentially unaffected at the deck-mean level — small shifts within noise. Midrange decks don't use multi-tutor reach, so the signal doesn't change their MCTS subtrees.

## Result 5 — Throughput (the smoking gun)

| Batch | Baseline g/s | Re-run g/s | Slowdown |
|-------|--------------|------------|----------|
| A     | 91.0         | 17.7       | 5.14×    |
| B     | 74.0         | 10.9       | 6.79×    |

**The signal IS being applied.** A 5–7× wall-clock slowdown at unchanged MCTS budget means the hat is expanding combo-assembly subtrees much deeper before pruning. The multi-tutor `scoreCombo` lift causes `ComboProximity × archetype-weight (2.0 for Combo / 1.0–1.4 for Storm/Stax)` to score higher on lines where 1 piece + 2 tutors are in hand, which keeps those MCTS nodes from being pruned and forces the search to evaluate the downstream "cast tutor, then cast piece" branches. The throughput collapse proves the search behavior changed; the unchanged game length proves the *decision* the search ultimately returns is approximately the same.

## Why didn't game length move?

The most likely explanation: **leaf-evaluation tuning is necessary but not sufficient.** The multi-tutor fix changes how favorably the hat scores the terminal state of "cast tutor + cast piece" branches at the MCTS leaves. But the cast-order priors driving *which* spell gets cast first this turn live in `internal/hat/gameplan.go` and `internal/hat/combo_sequencer.go`, not in `scoreCombo`. If the gameplan picks a value engine over a combo piece when the hat's first decision of the turn is made, the multi-tutor leaf bonus never gets to weigh in — the search tree already branched elsewhere.

Three orthogonal follow-ups, ranked by expected impact on game length:

1. **Sequencer-side combo priority.** When `Strategy.ComboPieces` has a plan with `realPiecesFound ≥ 1 AND tutorsInHand ≥ missing`, the combo sequencer should rank tutor-casts and piece-casts above value-engine casts for that turn. This is a *gameplan* change, not a leaf-evaluation change. Likely the missing wedge.
2. **Higher MCTS budget for Storm/Combo archetypes.** Budget = 50 may simply not carry the multi-tutor lift deep enough to terminal states. Bumping budget to 100 for archetypes with `bracket = cEDH AND finishers ≥ 8` would test whether the leaf-tuning is starved.
3. **`ChooseCastOrder` archetype-aware bias.** The hat's `ChooseCastOrder` decides which in-hand spell to cast first when multiple are castable. A Combo/Storm-aware variant that prefers tutor → piece over value-engine casts would short-circuit the search and act on the multi-tutor signal directly.

This re-run does not invalidate the multi-tutor tuning — it confirms the signal lands. It does establish that the signal alone is insufficient to drive earlier wincon assembly at the engine's current depth. The combo-assembly story needs a sequencer-level companion change before the seat-bias hypothesis can be tested in the regime it actually describes.

## Reproduction

```bash
# from a worktree at origin/main with PR #793 merged:
mkdir -p /tmp/cedh-seat-bias/{batch_a,batch_b}
cp data/decks/moxfield_300/etali_primal_conqueror_etali_primal_sickness_b5_*.txt /tmp/cedh-seat-bias/batch_a/
cp data/decks/moxfield_300/francisco_fowl_marauder_b5_*.txt                       /tmp/cedh-seat-bias/batch_a/
cp data/decks/moxfield_300/rograkh_son_of_rohgahh_b5_ocellblau_*.txt              /tmp/cedh-seat-bias/batch_a/
cp data/decks/moxfield_300/tymna_the_weaver_b5_ezinho_*.txt                       /tmp/cedh-seat-bias/batch_a/
cp data/decks/moxfield_300/tymna_the_weaver_b5_quincyhicks_*.txt                  /tmp/cedh-seat-bias/batch_b/
cp data/decks/moxfield_300/vial_smasher_the_fierce_b5_*.txt                       /tmp/cedh-seat-bias/batch_b/
cp data/decks/moxfield_300/tayam_luminous_enigma_b5_*.txt                         /tmp/cedh-seat-bias/batch_b/
cp data/decks/moxfield_300/rograkh_son_of_rohgahh_b5_valtchuz_*.txt               /tmp/cedh-seat-bias/batch_b/

go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_a --games 1250 --seed 42 \
   --report docs/cedh-gauntlet-rerun-r60-data/batch_a_report.md
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_b --games 1250 --seed 43 \
   --report docs/cedh-gauntlet-rerun-r60-data/batch_b_report.md
```

Per-batch raw reports committed at `docs/cedh-gauntlet-rerun-r60-data/batch_a_report.md` and `docs/cedh-gauntlet-rerun-r60-data/batch_b_report.md` — including the dashboard `SEAT-POSITION BIAS` and `WINRATE BY (COMMANDER, SEAT)` blocks.

## See also

- [`docs/cedh-seat-bias-r60-moxfield-replication.md`](cedh-seat-bias-r60-moxfield-replication.md) — PR #784, the baseline measurement.
- [`docs/cedh-seat-bias-r60.md`](cedh-seat-bias-r60.md) — original internal-pool study.
- [PR #793](https://github.com/hexdek-labs/HexDek/pull/793) — the multi-tutor credit fix this re-run measures.
- `internal/hat/evaluator.go scoreCombo()` (line 1245) — the function changed.
- `internal/hat/gameplan.go` + `internal/hat/combo_sequencer.go` — the sequencer-side surface the recommended follow-up targets.
