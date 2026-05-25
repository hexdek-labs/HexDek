# cEDH Seat-Bias Gauntlet — Hypothesis Rejected

> **TL;DR — cEDH seat-bias is QUARK-shaped, same as the all-archetype baseline.**
> A 2,500-game gauntlet across 8 B5/cEDH decks in 2 distinct 4-deck compositions finds **no stable early-seat advantage at the cEDH bracket**. The two compositions yield seat-bias patterns pointing in **opposite directions** — Comp A trends early (seat 0+1 = 52.0%), Comp B trends late (seat 0+1 = 46.0%). Aggregated, the per-seat winrate is essentially uniform (24.0 / 25.0 / 25.7 / 25.2). The hypothesis "cEDH turn 1-3 combo metas favor seat 0 (>27%) and disadvantage seat 3 (<22%)" is **rejected**.

| | |
|---|---|
| **Study branch** | `dev/cedh-seat-bias-r60` |
| **Run date** | 2026-05-25 |
| **Engine release** | r60 (canonical-clean post #427) |
| **Total games** | 2,500 (1,250 × 2 compositions) |
| **Deck pool** | 8 B5/cEDH decks from `data/decks/test/cedh_*.txt` |
| **Hat config** | YggdrasilHat budget 50, noise σ=0.2, no Freya intelligence (no `.freya.json` for these decks) |
| **Engine throughput** | Comp A 25.1 g/s, Comp B 33.7 g/s |
| **Crashes / Concessions** | 0 / 0 across both compositions |
| **Companion** | [`docs/seat-bias-meta-study-r60.md`](seat-bias-meta-study-r60.md) — the 37,500-game multi-archetype QUARK verdict this study replicates at cEDH bracket |
| **Live outcome** | reinforces the case for `CompositionPrior` ([#403](../../pulls/403)) over per-seat priors at every bracket level |

---

## Why we ran this

The all-archetype seat-bias meta-study (`docs/seat-bias-meta-study-r60.md`) tested 5 mixed-archetype compositions and verdicted **QUARK** — seat bias swings with composition rather than tracking the seat position itself. The follow-up question: does **cEDH specifically** behave differently?

The intuition is plausible. cEDH metas resolve in turns 1-3 via mana-tutor-counter-win lines. If the win condition fires before seat 3 even gets a turn, you'd expect an early-seat advantage: seat 0 plays first, seat 3 plays last, and at turn-3-win speed there's no time for late seats to assemble.

**Specific hypothesis being tested:**
- seat 0 winrate > 27%
- seat 3 winrate < 22%
- effect must replicate across distinct cEDH compositions (otherwise it's still QUARK)

If both conditions hold across both compositions, ship a per-seat penalty in the B5 bracket only — the `CompositionPrior` retains the all-archetype baseline but a seat-position term lifts B5-bracket games.

---

## Methodology

**2 compositions × 1,250 games each = 2,500 games**, default rotate mode (CR §103.1-style — each deck plays each seat exactly games/4 times). YggdrasilHat budget=50, noise σ=0.2. Wall time **86.9s** aggregate. Zero crashes, zero concessions across both runs.

| Comp | Decks (commanders) | Strategic flavor |
|:----|:----|:---|
| **A — Combo-heavy fast** | Kraum (Ludevic) / Ral, Monsoon Mage / Kinnan, Bonder Prodigy / Fire Lord Azula | Storm, turbo, partner-combo, combat-cast combo — the kind of pod where every deck wants to assemble its win in turns 2-4. |
| **B — Varied B5** | Y'shtola, Night's Blessed / Muldrotha, the Gravetide / Ardenn, Intrepid Archaeologist / Yarok, the Desecrated | Control, value/mill, voltron, blink-ETB — slower B5 decks that play more reactive midrange than turbo. |

The 9th deck (`cedh_good_luck_b5_lumra.txt`) was held out — 8 decks fit cleanly into 2 compositions of 4.

All cEDH decks lacked pre-computed Freya analysis at gauntlet time, so the Hat played without strategy intelligence (matching the seat-bias meta study's setup for comparability). This is conservative: WITH Freya intelligence cEDH decks play sharper, but the seat-bias question is about the engine's structural behavior, not the AI's archetype tuning.

---

## Headline finding: **cEDH bracket replicates the QUARK verdict**

| Seat | Comp A wins | Comp A rate | Comp B wins | Comp B rate | Aggregate rate |
|:----:|---:|---:|---:|---:|---:|
| 0 | 318 | **25.4%** | 283 | **22.6%** | **24.0%** |
| 1 | 333 | **26.6%** | 292 | **23.4%** | **25.0%** |
| 2 | 298 | **23.8%** | 345 | **27.6%** | **25.7%** |
| 3 | 301 | **24.1%** | 330 | **26.4%** | **25.2%** |
| total | 1,250 | 100% | 1,250 | 100% | 100% |

Standard error per cell at n=2,500 × p=0.25: ≈ 0.87pp.

The 4 seats' aggregate rates fall within **±1.7pp of uniform 25%** — well within sampling noise once both compositions are pooled. The within-composition spreads are higher (Comp A: 2.8pp; Comp B: 5.0pp) but **point in opposite directions** — the textbook QUARK signature.

### ASCII chart: seats vs uniform 25%

```
                       20%       22%       24%   25%   26%       28%       30%
                        ├─────────┼─────────┼─────┼─────┼─────────┼─────────┤
Comp A seat 0  ────────────────────#######25.4%
Comp A seat 1  ───────────────────────####26.6%      ← Comp A peak
Comp A seat 2  ───────────────#####23.8%                  ← Comp A floor
Comp A seat 3  ────────────────#####24.1%

Comp B seat 0  ─────────────#######22.6%                  ← Comp B floor
Comp B seat 1  ──────────────######23.4%
Comp B seat 2  ─────────────────────────####27.6%     ← Comp B peak
Comp B seat 3  ─────────────────────────###26.4%

Aggregate s0   ────────────────####24.0%
Aggregate s1   ─────────────────####25.0%
Aggregate s2   ──────────────────####25.7%
Aggregate s3   ─────────────────####25.2%
```

The visual is unambiguous: Comp A peaks early, Comp B peaks late, and the aggregate sits flat at ~uniform.

---

## Per-archetype × seat winrate (within composition)

Stderr per cell at games-per-cell ≈ 312 is ~2.4pp.

### Comp A (combo-heavy fast)

| Deck | seat 0 | seat 1 | seat 2 | seat 3 | within-seat range |
|:----|---:|---:|---:|---:|---:|
| Kraum, Ludevic's Opus | 18.5% | 14.4% | 17.3% | 17.3% | 4.1pp |
| Ral, Monsoon Mage | 8.3% | 15.3% | 10.3% | 7.7% | **7.6pp** |
| Kinnan, Bonder Prodigy | 58.7% | 60.4% | 53.7% | 59.0% | 6.7pp |
| Fire Lord Azula | 16.3% | 16.3% | 14.1% | 12.5% | 3.8pp |

Kinnan dominates this pod (~58% winrate across seats) — a turbo-ramp commander wins decisively when paired with three other combo decks. Kraum and Azula are roughly even role players. Ral underperforms — its storm engine wants more time than the pod allows.

### Comp B (varied B5)

| Deck | seat 0 | seat 1 | seat 2 | seat 3 | within-seat range |
|:----|---:|---:|---:|---:|---:|
| Y'shtola, Night's Blessed | 19.5% | 23.4% | 22.4% | 23.0% | 3.9pp |
| Muldrotha, the Gravetide | 7.0% | 8.6% | 9.0% | 9.9% | 2.9pp |
| Ardenn, Intrepid Archaeologist | 44.6% | 38.3% | 47.3% | 48.1% | **9.8pp** |
| Yarok, the Desecrated | 19.6% | 23.1% | 31.6% | 24.6% | **12.0pp** |

Ardenn (Voltron — equip-stacked beats) and Yarok (blink-ETB value) carry this pod. Muldrotha underperforms in every seat (~9%) — the slow value engine doesn't keep up with B5-bracket pressure even from "varied B5" opponents. Y'shtola sits around break-even (23%).

The per-deck × seat ranges within compositions (max 12pp for Yarok in Comp B) **swamp** the per-deck cross-composition seat-shift — there's no archetype here that classifies as ELECTRON (stable seat pattern across compositions). All 8 decks classify UNDETERMINED or QUARK, consistent with the meta-study's earlier finding.

---

## Statistical interpretation

Pooled across both compositions:
- Mean per-seat winrate: **25.0%** (exactly uniform expected value)
- Standard deviation across the 4 seat means: **0.62pp**
- Max-min spread: **1.7pp** (seat 0 24.0% → seat 2 25.7%)
- 95% confidence band per cell: **±1.71pp** (1.96 × 0.87pp stderr)

The aggregate per-seat rates **all four sit within the per-cell confidence band of 25%**. None of the 4 seats hits the hypothesized > 27% threshold, and none of the 4 seats sits below the hypothesized < 22% threshold.

Within each composition:
- Comp A seat 0 = 25.4% (below 27% hypothesis floor)
- Comp A seat 3 = 24.1% (above 22% hypothesis floor)
- Comp B seat 0 = 22.6% (well below 27% — opposite direction)
- Comp B seat 3 = 26.4% (well above 22% — opposite direction)

The Comp B numbers don't just fail the hypothesis — they **invert it**. Late seats outperform early seats by ~4pp.

---

## Verdict

**Hypothesis rejected.** cEDH seat-bias is QUARK-shaped, identical in character to the all-archetype seat-bias meta study. Composition flips the direction of the seat preference; aggregate per-seat winrate is uniform within sampling noise.

Concrete implications:

1. **Do not ship a per-seat penalty for B5 games.** The signal isn't there.
2. **`CompositionPrior` is the correct rating prior at every bracket level** — including B5. The cEDH bracket doesn't need a separate priors layer; it gets the same composition-aware treatment from the matchup matrix.
3. The Hat's archetype-specific weight profiles (each tuned with no seat awareness) are doing the right thing. cEDH archetypes that race (Kinnan, Ardenn) win equally well from any seat; cEDH archetypes that grind (Muldrotha) lose equally badly from any seat. The clock determines the outcome, not the seat.

The single robust prior is the composition prior. Seat position is noise at every measured bracket.

---

## Reproducibility

```bash
# Comp A — combo-heavy fast
go run ./cmd/hexdek-tournament \
  --decks data/decks/test/cedh_combo_partner_b5_kraum_tymna.txt,data/decks/test/cedh_stormoff_b5_ral.txt,data/decks/test/cedh_turbo_b5_kinnan.txt,data/decks/test/cedh_combat_cast_combo_b5_azula.txt \
  --games 1250 --seed 42

# Comp B — varied B5
go run ./cmd/hexdek-tournament \
  --decks data/decks/test/cedh_control_b5_yshtola.txt,data/decks/test/cedh_mullie_b5_muldrotha.txt,data/decks/test/cedh_big_stick_b5_ardenn_rograkh.txt,data/decks/test/cedh_blink_b5_yarok.txt \
  --games 1250 --seed 42
```

Both runs emit a `SEAT-POSITION BIAS:` section in the dashboard with the per-seat win counts that drive the tables above.

---

## See also

- [`docs/seat-bias-meta-study-r60.md`](seat-bias-meta-study-r60.md) — the 37,500-game multi-archetype QUARK verdict (5 compositions × 5 seeds × 1500 games each, including the original 1500-game single-pod baseline this whole line of investigation pivoted from).
- [`docs/composition-prior-validation.md`](composition-prior-validation.md) — the +1.4pp accuracy / +0.036 log-loss live validation of the CompositionPrior that replaced the abandoned per-seat prior.
- [PR #403](../../pulls/403) / [#408](../../pulls/408) / [#415](../../pulls/415) — `CompositionPrior` implementation, TrueSkill wire-in, showmatch live integration.
- [`docs/release-notes-r60.md`](release-notes-r60.md) — the broader r60 release context.
