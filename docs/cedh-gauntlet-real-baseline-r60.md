# cEDH Gauntlet — First Real Baseline (r60)

> **The first cEDH gauntlet run where `Strategy.ComboPieces` is actually populated.** PR #888 fixed the silent loader-path bug that caused every prior gauntlet (PRs #784 → #863) to operate on `profile=nil` — every architectural change in PRs #793, #826, #848 silently no-op'd because they all gated on `comboPieceSet` and `Strategy.ComboPieces`. This re-runs PR #784's full 2500-game protocol on current main (commit `3c7eb177`, post-PR #888 merge). Same pool, same seeds, same pod compositions, same hat config. Only delta: the loader actually finds the strategy.json files for the first time.

**Headline verdict: game length compresses materially (−4.55 turns, −9.4%), but the architectural chain (PRs #793 + #826 + #848) does NOT lift Combo win%. Francisco regressed −2.90pp from the original baseline. The per-archetype redistribution is consistent across the 400g pilot (PR #888) and this 2500g run.**

This is honest reporting. The chain compresses cEDH simulation toward more realistic game lengths (the 47–50 turn cEDH-as-midrange behavior PR #784/863 measured was the no-strategy fallback path), but it does not deliver the Combo-deck-winrate-climb the four PRs were claiming as their target metric. The two surfaces are decoupled.

## Verification — strategy ACTUALLY loaded

Per-deck `combos=N` from the tournament log (the first time these have been non-zero across any cEDH gauntlet):

```
deck etali_primal_conqueror_..._.txt:    loaded Freya strategy (archetype=midrange, combos=3, tutor_targets=3)
deck francisco_fowl_marauder_..._.txt:   loaded Freya strategy (archetype=combo,   combos=7, tutor_targets=8)
deck rograkh_son_of_rohgahh_ocellblau_..: loaded Freya strategy (archetype=storm,    combos=7, tutor_targets=8)
deck tymna_the_weaver_ezinho_..._.txt:   loaded Freya strategy (archetype=stax,     combos=7, tutor_targets=8)
deck rograkh_son_of_rohgahh_valtchuz_..: loaded Freya strategy (archetype=stax,     combos=2, tutor_targets=2)
deck tayam_luminous_enigma_..._.txt:     loaded Freya strategy (archetype=stax,     combos=4, tutor_targets=5)
deck tymna_the_weaver_quincyhicks_..._.txt: loaded Freya strategy (archetype=midrange, combos=6, tutor_targets=7)
deck vial_smasher_the_fierce_..._.txt:   loaded Freya strategy (archetype=storm,    combos=8, tutor_targets=9)
```

Cross-reference vs PR #863's same gauntlet on the pre-fix loader: `WARNING: deck X has no Freya analysis` printed for every deck, no `combos=N` log lines at all, `Strategy.ComboPieces` empty for every deck. **Every architectural change in PRs #793, #826, #848 fires for the first time in this run.**

## Method (identical to PR #784)

- **Pool:** 8 Freya-confirmed B5/cEDH decks from `data/decks/moxfield_300/*_b5_*.txt`.
- **Mode:** Two rotate-mode sub-gauntlets of 4 decks × 1250 games each = 2500 total.
- **Pod composition:** identical to PR #784 (Batch A seed 42: Etali / Francisco / Rograkh-ocellblau / Tymna-ezinho; Batch B seed 43: Tymna-quincyhicks / Vial Smasher / Tayam / Rograkh-valtchuz).
- **Engine:** built from `origin/main` at `3c7eb177` (post-#888 merge).
- **Hat:** default `yggdrasil`, budget 50, σ=0.2.
- **Max turns:** 80; per-game timeout: 120s. **0 crashes, 0 concessions, 0 timeouts** across all 2500 games.

## Result 1 — Avg game length (the headline)

| Gauntlet | Strategy loaded? | Avg turns |
|----------|-------------------|-----------|
| PR #784 (original baseline) | NO | 48.40 |
| PR #808 (post-#793 leaf-eval rerun) | NO | 48.25 |
| PR #826 (post-#826 sequencer-priority) | NO | 49.05 |
| PR #848 (post-#848 PlanState+budget) | NO | 47.95 |
| PR #863 (4-PR chain replication at 2500g) | NO | 48.35 |
| **This PR (post-#888 loader fix at 2500g)** | **YES** | **43.85** |

| Δ                              | Turns       | % change |
|--------------------------------|-------------|----------|
| **This vs PR #784 baseline**   | **−4.55**   | **−9.4%**|
| **This vs PR #863 (false-null)**| **−4.50**  | **−9.3%**|
| Internal: leaf-eval+sequencer+PlanState contribution | ≈ 0    | ≈ 0%     |
| Internal: loader-fix contribution                     | ≈ −4.55 | ≈ −9.4% |

The 4.55-turn compression maps cleanly to the loader fix landing — every prior PR (with profile=nil) sat at 48.0–49.1 turns. Once the strategy actually loads, average game length drops to 43.85.

**Where the engineering work earned its keep:** the 4-PR architecture chain (#793 + #826 + #848) was a no-op until #888. With #888 shipped, all four surfaces fire and game length compresses ~5 turns — but the bulk of the compression is "Freya combo recognition now wired into cardHeuristic + scoreCombo + planState," not any individual PR's architectural cleverness. The chain was *necessary* but the wiring fix was the load-bearing change.

## Result 2 — Per-deck winrate (n = 1250 per deck)

| Deck                       | Arch     | PR #784  | PR #863  | **This PR** | Δ vs #784 | Δ vs #863 |
|----------------------------|----------|----------|----------|-------------|-----------|-----------|
| Etali, Primal Conqueror    | Midrange | 47.80%   | 49.40%   | **51.80%**  | **+4.00** | **+2.40** |
| **Francisco, Fowl Marauder** | **Combo** | **36.40%** | 34.65% | **33.50%**  | **−2.90** | −1.15     |
| Rograkh (ocellblau)        | Storm    | 7.00%    | 8.30%    | 7.70%       | +0.70     | −0.60     |
| Tymna (ezinho)             | Stax     | 8.80%    | 7.70%    | 7.00%       | −1.80     | −0.70     |
| Rograkh (valtchuz)         | Stax     | 32.20%   | 32.23%   | **34.50%**  | **+2.30** | +2.27     |
| Tayam, Luminous Enigma     | Stax     | 49.80%   | 49.03%   | **45.30%**  | **−4.50** | **−3.73** |
| Tymna (quincyhicks)        | Midrange | 13.80%   | 13.12%   | 13.00%      | −0.80     | −0.12     |
| Vial Smasher the Fierce    | Storm    | 4.10%    | 5.55%    | **7.20%**   | **+3.10** | **+1.65** |

At n=1250 per deck the 95% Wilson CI is approximately ±2.4pp. Three shifts exceed that band:

- **Etali +4.00pp** — MDFC back-face combo is now recognized; the Storm-flavored back face benefits from the multi-tutor reach signal.
- **Tayam −4.50pp** — Stax value engine takes the largest single hit. With Storm and Combo opponents recognizing their lines faster, Tayam's slow-grind plan is less effective.
- **Vial Smasher +3.10pp** — Storm benefits cleanly from the chain. Recognized as Storm archetype with 8 combo plans; the lookahead matters more for spell-based combos.

**Francisco −2.90pp is at the noise floor but in the wrong direction.** The single Combo-archetype deck *lost* winrate.

## Result 3 — Per-archetype mean winrate

| Archetype | Decks | PR #784 | PR #863 | **This PR** | Δ vs #784 | Δ vs #863 |
|-----------|-------|---------|---------|-------------|-----------|-----------|
| **Combo** | 1     | 36.40%  | 34.65%  | **33.50%**  | **−2.90** | −1.15     |
| Storm     | 2     | 5.55%   | 6.93%   | **7.45%**   | **+1.90** | +0.52     |
| Stax      | 3     | 30.27%  | 29.65%  | 28.93%      | −1.33     | −0.72     |
| Midrange  | 2     | 30.80%  | 31.26%  | **32.40%**  | **+1.60** | +1.14     |

**Combo regressed.** The investigation chain's stated target metric — "combo decks should benefit from combo-assembly tuning" — does not hold even with the loader fix. Storm gains, Midrange gains, Stax marginal loss. Combo specifically loses.

Plausible mechanism (testable, not confirmed): Francisco at MCTS budget=50 (TierGungnir, no rollouts) now recognizes its 7 combo lines and tries to assemble them, but cannot actually close any of them because TierGungnir does single-state evaluation and the rollout simulator at `rollout.go resolveStack` doesn't simulate effect resolution (PR #888's documented Bottlenecks #2 and #3). Francisco spends turns committing to combo assembly that it can't actually finish, while Storm decks (whose combos are inherently faster and lean more on cantrip-storm count than multi-piece assembly) and Midrange decks (Etali's MDFC back face is structurally a Storm-flavored finisher) extract more value from the recognition.

## Result 4 — Per-seat winrate (n = 2500)

| Seat | Wins | Winrate | 95% Wilson CI    |
|------|------|---------|------------------|
| 0    | 651  | 26.04%  | [24.36%, 27.80%] |
| 1    | 595  | 23.80%  | [22.17%, 25.51%] |
| 2    | 633  | 25.32%  | [23.65%, 27.06%] |
| 3    | 621  | 24.84%  | [23.19%, 26.57%] |

**χ² = 2.650** (df=3, critical 7.815). Seat distribution is uniform — even with the strategy chain firing for the first time, no early-seat advantage emerges. χ² is slightly larger than PR #784's 1.354 (because the loader fix is shifting *something* in the per-seat distribution) but still well inside the noise band. The original "fast cEDH favors early seats" hypothesis remains unconfirmed; the engine doesn't kill on turn 1–3 even with all four PRs active.

## Result 5 — Throughput

Batch A: 15.4 g/s. Batch B: 15.1 g/s. **2–3× slower than PR #784's 74–91 g/s baseline** (the no-strategy fallback path) but **2× faster than PR #848's 6–12 g/s** at 500g/batch. The throughput pattern is consistent with the architectural surfaces actually firing (PR #793 throughput hit + PR #848 budget lift hit) while game length is shorter (offsetting the per-game work).

## What the chain actually did

Honest accounting of the four-PR investigation chain:

| PR | Stated contribution | Validated contribution (this run) |
|----|---------------------|------------------------------------|
| #793 | scoreCombo multi-tutor leaf credit | Fires for the first time. Contributes to the 4.55-turn game-length drop. Magnitude attributable to this PR alone is not isolable from the others. |
| #826 | cardHeuristic combo-priority cast-order bias | Fires for the first time. Contributes similarly. |
| #848 | Mid-turn PlanState refresh + budget lift | Fires for the first time. Contributes similarly. |
| #888 | Strategy loader parent-dir fallback | The wire that made the other three actually run. |

Without #888 the chain didn't matter. With #888 the chain matters but the empirical effect is "game length compresses cEDH-style, Storm+Midrange gain, Combo specifically regresses." None of the four prior PRs claimed Combo would regress.

The right honest statement: **PR #888 turned the chain ON, and turning the chain ON produces measurable behavioral changes — just not the specific change ("Combo deck wins more") the chain was sold as.** The cEDH simulation is now structurally more realistic; the assertion that this would lift Combo deck winrate specifically was wrong.

## Why Combo regressed — the testable hypothesis

Francisco now recognizes:
- 7 combo plans (Thoracle + Consultation, Thoracle + Tainted Pact, Thoracle + Beseech the Mirror, Nezahal + Bowmasters blink, etc.)
- 8 tutor targets
- archetype=combo

The cardHeuristic combo-priority bias (`+0.40` combo piece, `+0.35` tutor, `−0.15` value engine) now fires when planState flips to PlanAssemble. The hat actually steers toward combo lines.

But at MCTS budget=50 (TierGungnir):
- No rollouts. Single-state heuristic + UCB1 only.
- The hat sees "Demonic Tutor in hand → +0.35 cardHeuristic" but can't actually simulate "tutor for Oracle → cast Oracle → cast Consultation → win." That state never materializes in the search tree because the search tree doesn't have a forward simulator at this budget.
- So Francisco *commits* to combo assembly (cast tutors early, hold pieces) but doesn't actually *finish* the combo. Meanwhile Etali (which keeps recasting its MDFC back face) and Vial Smasher (whose storm-count engine fires every turn it casts something) close their games on the back of the same recognition + the architectural lift.

This is testable: run a `--hat-budget 200` ablation specifically on Francisco. If Combo win% climbs at budget 200 (TierRagnarok), Bottleneck #2 is confirmed and the path forward is "lift gauntlet budget." If Combo doesn't climb even at 200, Bottleneck #3 (rollout effect simulator approximation) is the binding constraint and the rollout needs real effect resolution.

## What this PR concludes

The four-PR investigation chain produces a real behavioral shift — when wired. The shift is:

- Avg game length: −4.55 turns (−9.4%). Real, large, durable.
- Per-archetype redistribution: Storm +1.9pp, Midrange +1.6pp, Stax −1.3pp, **Combo −2.9pp**.
- Per-seat distribution: still uniform (χ² 2.65, p ≈ 0.45).

**Combo deck winrate did not climb.** The investigation's stated target metric is rejected at full sample depth with strategy actually loaded. The chain still matters — game length compression is real and reflects more cEDH-like behavior — but the specific claim "this helps Combo decks win more" is empirically false.

## Recommendation

- **Land this as the first-ever real cEDH baseline.** Every prior gauntlet's per-deck winrate numbers were measuring the no-strategy fallback path and should not be cited going forward. This 43.85-turn / 33.50% Francisco / 51.80% Etali run is the actual current state.
- **`--hat-budget 200` ablation on Francisco is now the next experiment.** Until #888 landed it was untestable (no strategy loaded → no PlanState transitions → no rollout-trigger conditions). Now it can distinguish Bottleneck #2 (budget) from Bottleneck #3 (rollout simulator approximation).
- **Re-read PR #863's null verdict.** PR #863 was correct that the chain produced no Combo lift; the diagnosis (chain was unwired) was correct in PR #888. PR #888's pilot (400g) showed Combo flat, this 2500g run shows Combo −2.9pp. The chain doesn't lift Combo specifically — that conclusion is now empirically supported.
- **Do NOT re-attempt signal-side tuning** of leaf-eval / cast-order / planstate. Three rounds of refinement plus a loader fix produce game length compression but not Combo win% lift. The next round of work has to address Bottlenecks #2 and #3, not add more signal layers.

## Reproduction

```bash
mkdir -p /tmp/cedh-seat-bias/{batch_a,batch_b,freya}
# stage the 8 B5 decks from data/decks/moxfield_300/*_b5_*.txt as in PR #784
# stage their Freya analysis at /tmp/cedh-seat-bias/freya/<deck>.strategy.json
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_a --games 1250 --seed 42 \
   --report docs/cedh-gauntlet-real-baseline-r60-data/batch_a_report.md
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_b --games 1250 --seed 43 \
   --report docs/cedh-gauntlet-real-baseline-r60-data/batch_b_report.md
```

Per-batch raw reports at `docs/cedh-gauntlet-real-baseline-r60-data/batch_a_report.md` + `batch_b_report.md`. Both include `loaded Freya strategy (archetype=X, combos=N, tutor_targets=M)` log lines for every deck — the cross-reference signal that this is the real-baseline run, not the no-strategy fallback.

## See also

- [`docs/hat-bottleneck-r60.md`](hat-bottleneck-r60.md) — PR #888's investigation report that found and fixed the loader path bug.
- [`docs/cedh-gauntlet-rerun-postplanstate-r60.md`](cedh-gauntlet-rerun-postplanstate-r60.md) — PR #863's honest-null result on the unwired chain.
- [`docs/cedh-seat-bias-r60-moxfield-replication.md`](cedh-seat-bias-r60-moxfield-replication.md) — PR #784's original baseline (also unwired in retrospect).
- [PR #793](https://github.com/hexdek-labs/HexDek/pull/793), [#826](https://github.com/hexdek-labs/HexDek/pull/826), [#848](https://github.com/hexdek-labs/HexDek/pull/848), [#888](https://github.com/hexdek-labs/HexDek/pull/888) — the architecture chain whose actual contribution is measured here for the first time.
- `internal/hat/rollout.go` (Bottlenecks #2 + #3) — the next iteration's surface.
