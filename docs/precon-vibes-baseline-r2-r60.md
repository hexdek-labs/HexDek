# Precon Vibes-Bracket Calibration Baseline (R60, Round 2)

## Why this is round 2

PR #508 imported a 15-deck unedited-WotC-precon corpus and built the first vibes-bracket baseline against them, finding **2/15 B4 false-positives** and tracing both (PR #513) to a single predicate at `cmd/hexdek-freya/archetype.go:1229` (the "Tuned-redundancy floor"). A reasonable open question: was that result driven by the specific 15 precons sampled, or does it reflect a systemic over-firing of the floor across all WotC product?

This round runs the same exercise on a **completely disjoint** 15-deck precon corpus (no overlap with R1) to answer that question.

## Methodology

- **Corpus:** 15 NEW Moxfield community uploads of stock WotC precon decklists, in `data/decks/wizards/` (joining the R1 set; no R1 deck overlaps). Same 5-era distribution (3 per era). All from the canonical "Commander Precons" Moxfield namespace.
- **Pipeline:** identical to R1 — `go run ./cmd/hexdek-import/ --moxfield <url> --owner wizards`, Freya auto-runs.
- **Metrics & vibes-bracket rule:** identical to R1 (see `docs/precon-vibes-baseline-r60.md` §Methodology for full definitions). Same prediction rule: WotC default B2, lift to B3 on `power_pct ≥ 60 OR combo_density ≥ 4 OR gc ≥ 2`, lift to B4 on `power_pct ≥ 80 OR gc ≥ 4`, drop to B1 on `power_pct < 25 AND no win_lines AND no combos`.

## Ranked Table

| # | Era | Precon | Commander | Archetype | Mech Brkt | Plays-Like | Cmdr Syn % | Win Lines | Combo Dens | Chain Depth (avg/max) | Recursion Ratio | GC | Power % | Mana | **Vibes Brkt** | Δ |
|---|-----|--------|-----------|-----------|:---------:|:----------:|:----------:|:---------:|:----------:|:---------------------:|:---------------:|:--:|:-------:|:----:|:--------------:|:-:|
| 1 | C11 | Heavenly Inferno | Kaalia of the Vast | tribal | 2 Core | Exhibition | 11.5 | 16 | 1 | 3.00 / 3 | 0.00 | 0 | 32 | B | **2** | ✓ |
| 2 | C17 | Vampiric Bloodlust | Edgar Markov | tribal | 2 Core | Exhibition | 58.1 | 12 | 2 | 3.00 / 3 | 0.00 | 1 | 40 | F | **2** | ✓ |
| 3 | C18 | Nature's Vengeance | Lord Windgrace | counters | 2 Core | Core | 44.6 | 5 | 2 | 2.75 / 3 | 0.25 | 0 | 30 | D | **2** | ✓ |
| 4 | C19 | Primal Genesis | Ghired, Conclave Exile | midrange | 2 Core | Exhibition | 70.0 | 10 | 1 | 3.00 / 3 | 1.00 | 0 | 58 | B | **2** | ✓ |
| 5 | C20 | Timeless Wisdom | Gavi, Nest Warden | midrange | 3 Upgraded | Upgraded | 61.7 | **27,879** | 1 | 2.20 / 3 | 0.40 | 1 | 48 | D | **2** | +1 |
| 6 | C21 | Witherbloom Witchcraft | Willowdusk, Essence Seer | tribal | 2 Core | Core | 33.9 | 36 | 3 | 2.50 / 3 | 0.50 | 0 | 45 | B | **2** | ✓ |
| 7 | AFR | Dungeons of Death | Sefris of the Hidden Ways | midrange | 2 Core | Core | 43.3 | 10 | 2 | 2.33 / 3 | 0.50 | 0 | 58 | B | **2** | ✓ |
| 8 | MID | Coven Counters | Leinore, Autumn Sovereign | tribal | 2 Core | Core | 41.9 | 9 | 2 | 2.20 / 3 | 0.40 | 0 | 52 | A | **2** | ✓ |
| 9 | LCI | Veloci-Ramp-Tor | Pantlaza, Sun-Favored | midrange | 2 Core | Exhibition | 11.7 | 14 | 2 | 2.00 / 2 | 0.00 | 0 | 35 | C | **2** | ✓ |
| 10 | 40K | Tyranid Swarm | Magus Lucea Kane | midrange | 2 Core | Core | 58.3 | 4 | 1 | 2.33 / 3 | 0.33 | 0 | 58 | B | **2** | ✓ |
| 11 | 40K | The Ruinous Powers | Abaddon the Despoiler | tribal | 2 Core | Exhibition | 0.0 | 14 | 1 | 2.50 / 3 | 0.00 | 0 | 58 | A | **2** | ✓ |
| 12 | LTR | Elven Council | Galadriel, Elven-Queen | combo | 2 Core | Core | 37.7 | 15 | 1 | 2.25 / 3 | 0.50 | 0 | 63 | A | **3** | −1 |
| 13 | MH3 | Tricky Terrain | Omo, Queen of Vesuva | midrange | 2 Core | Exhibition | 16.4 | 14 | 2 | 2.60 / 3 | 0.20 | 0 | 43 | F | **2** | ✓ |
| 14 | BLB | Family Matters | Zinnia, Valley's Voice | midrange | **4 Optimized** | Core | 31.1 | 27 | 1 | 2.33 / 3 | 0.50 | 0 | 58 | B | **2** | **+2** |
| 15 | DSK | Jump Scare | Zimone, Mystery Unraveler | lands | 2 Core | Exhibition | 24.6 | 14 | 1 | 2.25 / 3 | 0.50 | 0 | 53 | B | **2** | ✓ |

**Aggregate:** 12/15 exact, 14/15 within ±1. **1/15 B4 false-positive** (Family Matters).

## Cross-validation vs R1

| Metric | R1 (PR #508) | R2 (this) | Combined (30 decks) |
|---|:-:|:-:|:-:|
| Exact match | 11/15 (73%) | 12/15 (80%) | 23/30 (77%) |
| Within ±1 | 13/15 (87%) | 14/15 (93%) | 27/30 (90%) |
| B4 false-positives | 2/15 (Urza, Blast) | 1/15 (Family Matters) | **3/30 (10%)** |
| `mechanical_bracket` vs `plays_like` disagree | 9/15 (60%) | 8/15 (53%) | 17/30 (57%) |
| Vibes-cooler-than-mech | 2/15 | 1/15 | 3/30 |
| Vibes-hotter-than-mech | 2/15 | 1/15 | 3/30 |

**Both top-line findings replicate.** The B4 false-positive rate (10% across 30 stock precons) is stable across the two waves. The `plays_like` vs `mechanical_bracket` disagreement is also stable at ~55-60% across both waves — consistent with the hypothesis from R1 §4 that one of those two code paths is systematically miscalibrated for the bottom of the distribution.

## Findings

### 1. **The B4 false-positive predicate fires on a third deck — confirmed not a one-off** (severity: HIGH, **CONFIRMS R1 ROOT CAUSE**)

**Family Matters (BLB Zinnia)** — third precon to trip the `Tuned-redundancy floor`. Trace:

```
Bracket rationale (raw score 7 → B4 Optimized):
  [+1] Tutor density (4-7%): 4% of nonlands
  [+3] Combo lines (5+): 10 true-infinite/determined loops
  [-1] Average CMC (heavy (>3.5)): 3.6 avg
  [+2] Fast mana (6-9): 8 sub-2-CMC mana producers
  [+2] Finisher density (8+): 15 distinct finisher lines
  [ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
  [floor]   Tuned-redundancy floor: lifted to B4: 15 finishers + 8 fast-mana pieces (was B2)
```

This is the EXACT same ceiling→floor override pattern documented in PR #513 for Blast from the Past: the `GC=0 ceiling` explicitly demotes the deck to B2 with reason *"no Game Changers and no true-infinite combo"*, and the tuned-redundancy floor runs immediately after and re-lifts to B4. Family Matters has GC=0, true_infinites=0, power_pct=58, mana grade B — none of the WotC-defined B4 markers — yet it lands at B4 because it ships with 8 fast-mana pieces (Bloomburrow's stock ramp suite: Sol Ring, signets, mana rocks) and the classifier counts 15 anthem-flavor finisher lines (Bello-style "tribal anthems make the board lethal" with each anthem counted separately).

3-for-3 on the predicate hit across two independent corpora removes any "specific deck quirk" interpretation. The PR #513 recommendation (tighten predicate to AND-chain `gameChangerCount ≥ 1 OR trueInfCount ≥ 1 OR tutorDensity ≥ 0.08 OR (avgCMC < 3.0 AND manaGrade ≥ B)` AND respect ceiling-applied reasons) would also catch Family Matters: GC=0, true_inf=0, tutor_density=4% (below 8%), avgCMC=3.6 (above 3.0) → none of the conjuncts satisfied → floor wouldn't fire.

### 2. **Cycling-loop combo-detector blowup — Timeless Wisdom shows 27,879 win lines** (severity: HIGH, **SECOND DECK CONFIRMING THE R1 SUSPICION**)

PR #513 §1B noted that Blast from the Past's "4 combo lines" were false positives — the combo detector treating cycling lands (Scattered Groves + Irrigated Farmland) as renewable loops when each cycle consumes the land permanently. R2 surfaces a far more extreme version of the same bug:

**Timeless Wisdom (C20 Gavi cycling commander)** reports `27,879 win_lines`. Breakdown by type:
- 27,323 of type `determined` — every cycling-card pair, every cycling-card triple, etc. (combinatorial blowup of `C(n, 2) + C(n, 3) + ...` across the ~25 cycling cards in the deck)
- 550 of type `infinite` — sample: `"Ominous Seas produces card for Tectonic Reformation, Tectonic Reformation produces card back | NO OUTLET -- draws the game"`. Tectonic Reformation gives lands cycling, Ominous Seas makes a Kraken on card draws — neither produces a renewable resource, and there's no actual "outlet" — the win-line classifier itself acknowledges in the description that there's no outlet but still counts it as a combo line.

Real win lines in Gavi cycling precon: probably 2-3 (cycle into card advantage, swing with Kraken/Eternal Scourge, commander damage from Gavi). The bracket score is fortunately NOT inflated by 27k (the score table caps at "5+ combo lines = +3"), but:
- the headline win_line count is meaningless and confuses any downstream user-facing report
- the underlying false-positive pattern (consume-once cards treated as loop pieces) is the same bug behind Blast from the Past's +2 score contribution

This is now confirmed across at least two decks, both featuring cycling. A targeted fix: the loop detector should require pieces whose triggers are reusable (cycling, kicker, suspend, encore — all single-shot — should NOT count as recurring producers).

### 3. **`mechanical_bracket` vs `plays_like` disagreement rate stable at ~55-60%** (severity: MEDIUM, **CONFIRMS R1 §4**)

R1 reported 60% (9/15) disagreement; R2 reports 53% (8/15). The two rates are statistically indistinguishable across N=15 samples. Combined across 30 decks: 57%.

Both waves show the same DIRECTION: `plays_like` consistently calls Exhibition (B1) where `bracket` calls Core (B2). 7 of the 8 R2 disagreements are `plays_like=Exhibition vs bracket=Core`; 8 of 9 R1 disagreements were the same shape. This is now strong evidence that the `plays_like` simulator UNDER-rates "stuff happens" precons — decks without a clean win condition that nevertheless function as B2 product. The fix surface is in `estimatePlaysLike()` at `cmd/hexdek-freya/archetype.go:1246`, not `estimateBracket()`.

### 4. **Edgar Markov tribal precon lands at C-grade mana base = F** (severity: LOW — possible grader miscalibration)

Vampiric Bloodlust (C17 Edgar Markov) reports mana grade F despite being an aggro tribal precon with what looks like a reasonable 36-land suite (most C17 precons ship the Cycle 5 dual-cycle lands plus utility). F-grade reads harsh for a precon a human would call C-tier. Worth a separate look at the mana grader on aggro 3-color tribal decks; might be over-penalizing taplands on a deck whose curve genuinely doesn't care about an untapped T2. Not blocking, but logged.

### 5. **Veloci-Ramp-Tor (LCI Pantlaza) commander_synergy 11.7%** (severity: LOW — informational)

Two-deck pattern emerging: Vrondiss (R1) at 11.7% and now Pantlaza (R2) at 11.7%. Both are "creature-type-matters" commanders (Dragon tribal / Dinosaur tribal). The synergy scorer likely doesn't recognize tribal-payoff-on-commander as commander synergy because the commander's text isn't itself a tribal payoff — Vrondiss makes Elementals from damage, Pantlaza gives discover. The commander synergy heuristic seems to score "commander as engine" but not "commander as tribal anchor with anthem support in the 99". Two data points isn't enough to call it a bug, but it's worth tracking.

## Suggested Next Steps (unchanged from R1)

1. ✅ R1 §1 — already resolved by tracing PR #513. **Reaffirmed by R2 §1.**
2. ⏳ Implement PR #513's recommended fix (engine work, awaiting 7174n1c decision).
3. **NEW from R2 §2:** fix the cycling-loop combo-detector false-positive. Loop pieces should require renewable triggers; cycling/kicker/suspend/encore-class triggers should be excluded.
4. ⏳ Add `data/decks/wizards/` regression test — corpus is now 30 decks; the assertion "every WotC stock precon ≤ B3" would currently fail on 3/30 (10%).
5. **NEW from R2 §3:** focused study on `estimatePlaysLike()` — the 17/30 plays-like-under-mech disagreements are consistently in one direction, which is a fixable systematic bias.

## Reproducing

```bash
# Run the full R2 import (15 URLs in source list below; identical command form to R1):
go run ./cmd/hexdek-import/ --moxfield https://moxfield.com/decks/<id> --owner wizards

# Re-trace any deck's bracket call:
go run ./cmd/hexdek-freya/ --deck data/decks/wizards/<slug>.txt 2>&1 | grep -A 15 "Bracket rationale"
```

15 source URLs (all `https://moxfield.com/decks/<id>` in the `Commander Precons` namespace, none overlapping R1):

| Precon | Moxfield ID |
|--------|-------------|
| Heavenly Inferno (C11) | `RTMMomJxi0iK-r6ZDzBXeQ` |
| Vampiric Bloodlust (C17) | `dne9B6FNGk2I86ppJ3iK5Q` |
| Nature's Vengeance (C18) | `m2GByoxwLU-kOIAWK3nFkw` |
| Primal Genesis (C19) | `647BlyGrEUCXfNB-PTvCxQ` |
| Timeless Wisdom (C20) | `FoYllHo-K0WwAeO4uX5uLg` |
| Witherbloom Witchcraft (C21) | `6WeWU_rriEaCGPmJ2l1e1g` |
| Dungeons of Death (AFR) | `Ym001xPF0E2HZl91qKhcwQ` |
| Coven Counters (MID) | `dj8xqpaSZUat_G4t1vXcSg` |
| Veloci-Ramp-Tor (LCI) | `viXU1nCwOkmTU5FGQu8MFw` |
| Tyranid Swarm (40K) | `3mvusZqSGESodaok8c5q7g` |
| The Ruinous Powers (40K) | `Co8GiCNRIUqzCJBzpdRaAg` |
| Elven Council (LTR) | `4aKNkLrXmU-zsytziZ6JnQ` |
| Tricky Terrain (MH3) | `GBX3VBGJH0ezo5sOEy53aQ` |
| Family Matters (BLB) | `PzY-rAZ3SEiLi_fuR2XBhQ` |
| Jump Scare (DSK) | `jZhx0oSi70aGEvOkBuSMHA` |
