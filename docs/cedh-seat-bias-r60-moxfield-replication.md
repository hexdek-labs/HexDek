# cEDH Seat-Bias Gauntlet — Moxfield-Pool Replication (r60)

> **Companion study to [`docs/cedh-seat-bias-r60.md`](cedh-seat-bias-r60.md).** That study used 8 internal `data/decks/test/cedh_*.txt` fixtures (Kraum, Ral, Kinnan, Azula, Y'shtola, Muldrotha, Ardenn, Yarok) and verdicted **QUARK** — no stable seat bias at the cEDH bracket. This run replicates the methodology on a **different** 8-deck pool — the 8 community-built B5 decks filed in `data/decks/moxfield_300/*_b5_*.txt` — and reaches the same verdict at p ≈ 0.72. It also surfaces a load-bearing caveat the original study undersells: at the engine's current play depth, cEDH games run 47–50 turns, which is well outside the turn-1-to-3 regime in which the early-seat-advantage hypothesis would actually operate.

**Verdict:** **Hypothesis refuted at observed game length; not falsifiable in the regime the hypothesis assumes.** χ² = 1.354 across the 4 seats (df = 3, p ≈ 0.72); all four seat winrates' 95% Wilson CIs cover 25.00%.

## Method

- **Pool:** All 8 decks tagged `_b5_` under `data/decks/moxfield_300/`. Freya's archetype classifier was run on each one; every one came back `bracket = cEDH` and was kept in the pool.
- **Mode:** Two rotate-mode sub-gauntlets of 4 decks × 1250 games each (= 2500 total). `cmd/hexdek-tournament`'s default rotate path (`runner.go:105`) takes the first NSeats decks and rotates them each game, so every deck plays every seat ~312 times.
- **Why two sub-gauntlets, not one big pool/round-robin:** only the rotate-mode aggregator (`aggregate.go:21–95`) populates `WinsBySeat` and `WinsByCommanderBySeat`. Pool mode randomizes seat assignment per game and discards seat indices in aggregation; round-robin's aggregator drops seat data entirely. Two balanced rotate pods preserve "each deck plays each seat equal games" across the 8-deck corpus at the cost of cross-pod head-to-head matchups (acceptable: the question asked is seat bias, not matchup matrix).
- **Pod composition:** balanced for archetype mix so neither sub-gauntlet is dominated by one strategy.
  - **Batch A** (seed 42): Etali (Midrange), Francisco (Combo), Rograkh-ocellblau (Storm), Tymna-ezinho (Stax)
  - **Batch B** (seed 43): Tymna-quincyhicks (Midrange), Vial Smasher (Storm), Tayam (Stax), Rograkh-valtchuz (Stax)
- **Engine:** r60-clean (post #427: Loki 100K chaos + 100K nightmare = 0 violations per `docs/loki-r60-canonical-final.md`).
- **Hat:** default `yggdrasil`, budget 50, σ = 0.2 — same configuration the original `cedh-seat-bias-r60.md` ran. No per-seat hat variation.
- **Max turns:** 80; per-game timeout: 120 s. **0 crashes, 0 concessions, 0 timeouts** across all 2500 games.

## Result 1 — Combined seat-position winrate (n = 2500)

| Seat | Wins | Winrate | 95% Wilson CI | Δ vs 25.0% expected |
|------|------|---------|---------------|---------------------|
| 0    | 641  | 25.64%  | [23.97%, 27.39%] | +0.64 pp |
| 1    | 611  | 24.44%  | [22.80%, 26.16%] | −0.56 pp |
| 2    | 610  | 24.40%  | [22.76%, 26.12%] | −0.60 pp |
| 3    | 638  | 25.52%  | [23.85%, 27.27%] | +0.52 pp |

**χ² = 1.354, df = 3, p ≈ 0.72.** All four 95% CIs cover 25.00%. The mild seat-0 / seat-3 uptick (the "start and end of round" pattern) is smaller than the per-cell sampling noise and reverses between batches: Batch A favors seat 2 (+11.5 wins above expected); Batch B favors seats 0 and 3 (+22.5 and +13.5). Aggregate is essentially flat.

## Result 2 — Per-archetype absolute winrate by seat

Decks classified by Freya's primary archetype tag. Absolute winrate is per-batch (decks within the same batch share opponents), so cross-archetype absolute comparison is biased by who shared the pod.

| Archetype  | # Decks | seat 0 | seat 1 | seat 2 | seat 3 |
|------------|---------|--------|--------|--------|--------|
| Combo      | 1       | 38.3%  | 34.8%  | 37.8%  | 34.6%  |
| Storm      | 2       | 5.6%   | 5.0%   | 6.7%   | 5.0%   |
| Stax       | 3       | 31.7%  | 30.4%  | 27.6%  | 31.4%  |
| Midrange   | 2       | 30.2%  | 29.8%  | 30.6%  | 32.6%  |

## Result 3 — Per-archetype seat-sensitivity Δ (relative to deck mean)

Δ = (per-seat winrate) − (per-deck mean winrate), then averaged across decks of the same archetype. This controls for absolute deck strength so seat-position effects are comparable across archetypes.

| Archetype  | # Decks | seat 0  | seat 1  | seat 2  | seat 3  |
|------------|---------|---------|---------|---------|---------|
| Combo      | 1       | +1.92pp | −1.58pp | +1.42pp | −1.78pp |
| Storm      | 2       | +0.05pp | −0.60pp | +1.15pp | −0.60pp |
| Stax       | 3       | +1.39pp | +0.09pp | −2.64pp | +1.16pp |
| Midrange   | 2       | −0.60pp | −1.00pp | −0.20pp | +1.80pp |

**Read.** Combo (Francisco only — n = 1, treat as directional) leans toward even seats (0 and 2), not early seats. Storm is the flattest profile. Stax shows a real seat-2 dip of −2.64pp, plausibly explained by Stax being least able to recover when the seat-3 player has table information before their first turn. Midrange tilts modestly to seat 3 (+1.80pp). No archetype shows the early-seat-advantage signature the hypothesis predicts.

## Result 4 — Per-deck × per-seat winrate with CIs (n ≈ 312 per cell)

| Deck | Archetype | seat 0 (CI) | seat 1 (CI) | seat 2 (CI) | seat 3 (CI) | mean |
|------|-----------|-------------|-------------|-------------|-------------|------|
| Etali, Primal Conqueror              | Midrange | 45.7% [40.4, 51.4] | 47.8% [42.3, 53.3] | 49.7% [44.2, 55.2] | 47.9% [42.6, 53.6] | 47.8% |
| Francisco, Fowl Marauder             | Combo    | 38.3% [33.2, 44.0] | 34.8% [29.9, 40.4] | 37.8% [32.6, 43.3] | 34.6% [29.6, 40.1] | 36.4% |
| Rograkh (ocellblau)                  | Storm    | 6.1%  [3.9, 9.3]   | 6.4%  [4.2, 9.7]   | 8.9%  [6.3, 12.7]  | 6.7%  [4.4, 10.1]  | 7.0%  |
| Tymna the Weaver (ezinho)            | Stax     | 7.7%  [5.2, 11.2]  | 9.6%  [6.8, 13.4]  | 7.3%  [5.0, 10.8]  | 10.5% [7.6, 14.5]  | 8.8%  |
| Rograkh (valtchuz)                   | Stax     | 35.5% [30.5, 41.0] | 31.7% [26.8, 37.1] | 29.8% [25.0, 35.1] | 31.9% [27.1, 37.4] | 32.2% |
| Tayam, Luminous Enigma               | Stax     | 51.8% [46.4, 57.4] | 49.8% [44.5, 55.5] | 45.8% [40.4, 51.4] | 51.9% [46.4, 57.4] | 49.8% |
| Tymna the Weaver (quincyhicks)       | Midrange | 14.7% [11.2, 19.1] | 11.8% [8.7, 15.9]  | 11.5% [8.5, 15.6]  | 17.3% [13.5, 21.9] | 13.8% |
| Vial Smasher the Fierce              | Storm    | 5.1%  [3.2, 8.2]   | 3.5%  [2.0, 6.2]   | 4.5%  [2.7, 7.4]   | 3.2%  [1.8, 5.8]   | 4.1%  |

Every per-deck per-seat 95% Wilson CI overlaps that deck's other-seat CIs. No deck shows seat-position dependence beyond per-cell sampling noise.

## The load-bearing caveat the original study undersells

Average game length in this gauntlet: **46.8 turns (Batch B) – 50.0 turns (Batch A)**. 99% of games landed in the 21+ turn bucket. The hypothesis under test — "cEDH turn 1-to-3 combo metas favor early seats" — describes a regime where games end before seat 3 gets a turn. At 47-turn average game length, seat 3 has played ~47 turns by game-end. The structural mechanism the hypothesis names cannot operate here.

This does not mean the engine is broken (Loki is clean). It means YggdrasilHat at budget 50 plays B5/cEDH decks as midrange value engines rather than as combo-assembly priorities. The B5 decks in this pool contain Thassa's Oracle + Demonic Consultation, Underworld Breach lines, Worldgorger Dragon + Animate Dead, Dramatic Reversal + Isochron Scepter — and the hat does not assemble them at the turn-2-to-4 lethal speed those packages support in real cEDH play. Until that gap closes, **no seat-bias gauntlet in this engine can answer the question the hypothesis is actually asking**.

The internal-pool study reached the same verdict at the same game-length regime; the moxfield-pool replication confirms the verdict is pool-independent. Both verdicts should be read as "*there is no seat-position bias at HexDek's current play depth*", not "*there is no seat-position bias in cEDH metas*". The first claim is supported; the second is not testable until the hat plays cEDH at lethal-line speed.

## Recommended follow-up

- **Hat fix first, not gauntlet expansion.** Until YggdrasilHat assembles Thoracle / Breach / Worldgorger by turn 4–6 on the B5 decks, this measurement is structurally underpowered. Suggested wedge: an archetype-conditional MCTS weight that boosts combo-assembly when the deck's Freya strategy bridge reports `bracket = cEDH AND finishers ≥ 8`.
- **Re-run at the same 2500-game depth after the hat fix lands.** If seat-0 winrate climbs above the upper 95% CI bound (>27.4% sustained), the original hypothesis is confirmed. If it remains in [23.5%, 27.5%], the seat-position effect at HexDek's eventual cEDH-speed fidelity is genuinely below detection.
- **Add `--hat-budget-per-seat`.** Asymmetric skill — the seat-3 deck has the most information, the seat-0 deck has the least defensive prep time — is plausibly the dominant seat-bias mechanism in real cEDH and is not currently testable in the runner.

## Reproduction

```bash
mkdir -p /tmp/cedh-seat-bias/{batch_a,batch_b}

# Batch A — Midrange + Combo + Storm + Stax mix
cp data/decks/moxfield_300/etali_primal_conqueror_etali_primal_sickness_b5_*.txt /tmp/cedh-seat-bias/batch_a/
cp data/decks/moxfield_300/francisco_fowl_marauder_b5_*.txt                       /tmp/cedh-seat-bias/batch_a/
cp data/decks/moxfield_300/rograkh_son_of_rohgahh_b5_ocellblau_*.txt              /tmp/cedh-seat-bias/batch_a/
cp data/decks/moxfield_300/tymna_the_weaver_b5_ezinho_*.txt                       /tmp/cedh-seat-bias/batch_a/

# Batch B — Midrange + Storm + Stax + Stax mix
cp data/decks/moxfield_300/tymna_the_weaver_b5_quincyhicks_*.txt                  /tmp/cedh-seat-bias/batch_b/
cp data/decks/moxfield_300/vial_smasher_the_fierce_b5_*.txt                       /tmp/cedh-seat-bias/batch_b/
cp data/decks/moxfield_300/tayam_luminous_enigma_b5_*.txt                         /tmp/cedh-seat-bias/batch_b/
cp data/decks/moxfield_300/rograkh_son_of_rohgahh_b5_valtchuz_*.txt               /tmp/cedh-seat-bias/batch_b/

go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_a --games 1250 --seed 42 \
   --report docs/cedh-seat-bias-r60-data/batch_a_report.md
go run ./cmd/hexdek-tournament --decks /tmp/cedh-seat-bias/batch_b --games 1250 --seed 43 \
   --report docs/cedh-seat-bias-r60-data/batch_b_report.md
```

Per-batch markdown reports — including the dashboard's `SEAT-POSITION BIAS` and `WINRATE BY (COMMANDER, SEAT)` blocks — are committed at `docs/cedh-seat-bias-r60-data/batch_a_report.md` and `docs/cedh-seat-bias-r60-data/batch_b_report.md`.

## See also

- [`docs/cedh-seat-bias-r60.md`](cedh-seat-bias-r60.md) — the original internal-pool study (Comp A: Kraum/Ral/Kinnan/Azula; Comp B: Y'shtola/Muldrotha/Ardenn/Yarok) that this replication independently corroborates.
- [`docs/seat-bias-meta-study-r60.md`](seat-bias-meta-study-r60.md) — the 37,500-game multi-archetype QUARK verdict.
- [`docs/composition-prior-validation.md`](composition-prior-validation.md) — the live `CompositionPrior` validation that replaced the abandoned per-seat prior.
