# Precon Vibes-Bracket Calibration (R60, Round 7)

## Round status — R6 absent

7174n1c's instruction set this up as "wait for R6 to merge, then read `data/decks/wizards/` for non-overlap (90 precons by then)." **R6 was never staffed:**

```
$ git branch -a | grep precon-vibes-r6     # empty
$ gh pr list --state all --search "precon-vibes-r6"   # empty
```

R5 was the last round that added decks (PR #533, merged 2026-05-26). The current `data/decks/wizards/` count at the start of this branch was **60 decks (R1+R2+R3+R5, 15 each)** — 30 short of 7174n1c's expected 90 because both R4 and R6 happened to be analysis-only / unstaffed waves.

The user explicitly authorized "ship with whatever you find and explicitly mark coverage exhaustion." This R7 round took the option literally: imported 15 fresh precons from gaps in the Commander Precons namespace that R1-R5 didn't touch. Combined corpus is now **75 decks (R1+R2+R3+R5+R7)**. Coverage is NOT yet exhausted at the WotC catalog level — see the [§Catalog gap analysis](#catalog-gap-analysis) section for what remains — but the WotC product is rate-of-decay slowing enough that R7 reasonably IS the final calibration round for now, modulo new product (Foundations Commander 2026, etc.) WotC ships after this date.

## Methodology

Identical to R5 (PR #533):

- **Corpus:** 15 NEW Moxfield community uploads from the canonical "Commander Precons" namespace. Zero overlap with R1 (PR #508), R2 (PR #523), R3 (PR #532), or R5 (PR #533). Every Moxfield ID was cross-referenced against the prior round's source tables.
- **Pipeline:** `go run ./cmd/hexdek-import/ --moxfield <url> --owner wizards`. Freya auto-runs on import; per-deck output at `data/decks/wizards/freya/<slug>.{strategy,profile}.json`.
- **Naming convention** (post-PR #529): `bracket` is the declared / rubber-stamp bracket (defaults to B2 under `data/decks/wizards/`). `measured_bracket` is what Freya's signal estimator computes. Δ column = `declared − measured`; negative = engine HOTTER than declared, positive = engine COOLER.
- **Metrics:** identical to R1/R2/R5 — measured_bracket, plays_like_label, commander_synergy×100, win_line_count, combo_density (= `len(combo_notes)`), value_chains[].depth (avg/max), recursion ratio (= fraction of chains with `recursion_depth` ∈ {deep, infinite}), game_changer_count, power_percentile, mana_base_grade.
- **Declared-bracket rule:** B1 if `power_pct < 25 AND no win_lines AND no combos`; B3 lift if `power_pct ≥ 60 OR combo_density ≥ 4 OR gc ≥ 2`; B4 lift if `power_pct ≥ 80 OR gc ≥ 4`; otherwise B2.
- **Reproducer:** `python3 scripts/build_precon_r7_table.py` rebuilds the ranked table from the sidecars; reads strategy.json + profile.json deterministically.

## Era coverage of R7 picks

| Era | Decks |
|---|---|
| EoE (Edge of Eternities) | World Shaper, Counter Intelligence |
| VOW (Innistrad: Crimson Vow) | Vampiric Bloodline, Spirit Squadron |
| BRO (Brothers' War) | Mishra's Burnished Banner |
| C21 Secrets of Strixhaven | Silverquill Influence |
| SL Commander 2025 (Secret Lair) | Everyone's Invited! |
| SNC (Streets of New Capenna) | Bedecked Brokers, Cabaretti Cacophony, Maestros Massacre, Obscura Operation, Riveteers Rampage |
| ONE (Phyrexia: All Will Be One) | Rebellion Rising |
| BLB (Bloomburrow) | Peace Offering |
| MH3 (Modern Horizons 3) | Graveyard Overdrive |

R7 deliberately fills the **all-five-SNC-decks** gap that R1-R5 left wide open, completes Crimson Vow's 2-deck precon line (no prior coverage), and adds the **brand-new Edge of Eternities** product (released after R5 wrote its doc; not in any earlier corpus). Plus one-off gap fills (BRO sibling Mishra's Burnished Banner, ONE Rebellion Rising, BLB Peace Offering, MH3 Graveyard Overdrive).

## Ranked Table

Sorted chronologically by era, then by intra-era cluster.

| # | Era | Precon | Commander | Archetype | Mch | Plays-Like | Cmdr Syn % | Win Lines | Combo Dens | Chain Depth (avg/max) | Recursion Ratio | GC | Power % | Mana | **Declared Brkt** | Δ |
|---|-----|--------|-----------|-----------|:---:|:----------:|:----------:|:---------:|:----------:|:---------------------:|:---------------:|:--:|:-------:|:----:|:--------------:|:-:|
|  1 | EoE     | World Shaper          | Hearthhull, the Worldseed     | midrange     | **4 Optimized** | Upgraded   | 80.7 |  47 | 1 | 2.80 / 3 | 0.60 | 0 | 60 | B | **3** | −1 |
|  2 | EoE     | Counter Intelligence  | Inspirit, Flagship Vessel     | artifacts    | 2 Core         | Core       | 98.3 |   9 | 1 | 2.50 / 3 | 0.50 | 0 | 63 | C | **3** | +1 |
|  3 | VOW     | Vampiric Bloodline    | Strefan, Maurer Progenitor    | counters     | 2 Core         | Core       | 37.1 | 137 | 1 | 2.50 / 3 | 1.00 | 0 | 63 | B | **3** | +1 |
|  4 | VOW     | Spirit Squadron       | Millicent, Restless Revenant  | midrange     | 2 Core         | Exhibition | 48.4 |  11 | 2 | 3.00 / 3 | 0.00 | 0 | 63 | A | **3** | +1 |
|  5 | BRO     | Mishra's Burnished Banner | Mishra, Eminent One       | artifacts    | 2 Core         | Core       | 90.3 |  11 | 1 | 3.00 / 3 | 0.67 | 0 | 63 | C | **3** | +1 |
|  6 | C21 SoS | Silverquill Influence | Killian, Decisive Mentor      | enchantress  | 1 Exhibition   | Exhibition | 82.3 |   3 | 3 | 2.40 / 3 | 0.40 | 0 | 63 | B | **3** | **+2** |
|  7 | SL2025  | Everyone's Invited!   | Morophon, the Boundless       | tribal       | 2 Core         | Exhibition | 48.4 |   7 | 2 | — / —    | 0.00 | 0 | 55 | C | **2** | ✓ |
|  8 | SNC     | Bedecked Brokers      | Perrie, the Pulverizer        | midrange     | 2 Core         | Exhibition | 73.8 |   6 | 1 | 2.00 / 2 | 0.50 | 0 | 50 | C | **2** | ✓ |
|  9 | SNC     | Cabaretti Cacophony   | Kitt Kanto, Mayhem Diva       | midrange     | **4 Optimized** | Exhibition | 50.8 |  59 | 1 | 2.00 / 2 | 1.00 | 0 | 63 | B | **3** | −1 |
| 10 | SNC     | Maestros Massacre     | Anhelo, the Painter           | storm        | 2 Core         | Exhibition | 71.0 |   1 | 1 | 3.00 / 3 | 0.33 | 0 | 43 | B | **2** | ✓ |
| 11 | SNC     | Obscura Operation     | Kamiz, Obscura Oculus         | midrange     | 2 Core         | Core       | 59.0 |  10 | 1 | 2.00 / 2 | 0.50 | 0 | 58 | B | **2** | ✓ |
| 12 | SNC     | Riveteers Rampage     | Henzie "Toolbox" Torre        | midrange     | 2 Core         | Exhibition | 38.3 |  10 | 3 | 2.67 / 3 | 0.33 | 0 | 53 | B | **2** | ✓ |
| 13 | ONE     | Rebellion Rising      | Neyali, Suns' Vanguard        | artifacts    | **4 Optimized** | Exhibition | 70.5 |  23 | 1 | 2.50 / 3 | 0.00 | 0 | 71 | B | **3** | −1 |
| 14 | BLB     | Peace Offering        | Ms. Bumbleflower              | midrange     | 2 Core         | Core       | 80.3 |   7 | 1 | 2.00 / 2 | 1.00 | 0 | 81 | A | **4** | **+2** |
| 15 | MH3     | Graveyard Overdrive   | Disa the Restless             | combo        | **4 Optimized** | Core       | 63.3 |  32 | 2 | 2.57 / 3 | 0.43 | 0 | 60 | B | **3** | −1 |

**Aggregate:** 5/15 exact, 13/15 within ±1, 2/15 off by ±2. **4/15 B4 false-positives** (World Shaper, Cabaretti Cacophony, Rebellion Rising, Graveyard Overdrive) — the highest single-round B4 FP rate in the entire calibration series.

## Cross-validation vs prior rounds

| Metric | R1 (508) | R2 (523) | R3 (532) | R5 (533) | R7 (this) | Combined (75 decks) |
|---|:-:|:-:|:-:|:-:|:-:|:-:|
| Exact match | 11/15 (73%) | 12/15 (80%) | 9/15 (60%) | 4/15 (27%) | 5/15 (33%) | **41/75 (55%)** |
| Within ±1 | 13/15 (87%) | 14/15 (93%) | 14/15 (93%) | 13/15 (87%) | 13/15 (87%) | **67/75 (89%)** |
| B4 false-positives | 2/15 (13%) | 1/15 (7%) | 3/15 (20%) | 2/15 (13%) | **4/15 (27%)** | **12/75 (16%)** |
| `measured_bracket` vs `plays_like` disagree | 9/15 | 8/15 | ~8/15 | 8/15 | 9/15 | **~42/75 (56%)** |
| Δ ≤ −1 (engine hotter) | 2/15 | 1/15 | ~3/15 | 3/15 | **4/15** | **~13/75 (17%)** |
| Δ ≥ +1 (engine cooler) | 2/15 | 1/15 | ~3/15 | 8/15 | 6/15 | **~20/75 (27%)** |

R3 numbers approximate per the R5 doc's cross-validation table (PR #532's branch surfaced 6/45 B4 FPs across R1+R2+R3, of which R1 contributed 2 and R2 contributed 1, leaving R3 at 3).

### What replicates and what doesn't

- **B4 false-positive rate is TRENDING UP.** R1/R2 sat at 7-13%; R3-R7 sit at 13-27%; the 5-round combined rate is 12/75 = 16%, up from the 5-deck-baseline 10% noted in R2's findings. **Two distinct predicates now drive B4 FPs**: the longstanding `Tuned-redundancy floor` (R1-R5 documented at length; fires on Rebellion Rising raw 4 → B4 + Graveyard Overdrive raw 7 → B4 in R7) AND a `Winning-combo floor` (raw label "WotC carveout") that hadn't appeared in earlier rounds but fires on **2/4 R7 B4 FPs** (World Shaper, Cabaretti Cacophony). See [§Findings](#findings) for traces.
- **Exact-match rate stays low post-R3** at 27-33%, consistent with R5's "modern WotC product trends higher `power_percentile` than older sets" finding. R7's SNC sub-corpus (5 decks) sits right at the 50-63% power band that lifts the declared rule to B3 while leaving `measured_bracket` at B2; 4 of 5 SNC decks hit Δ ∈ {0, ✓} confirming the declared rule is reasonable for that band, but the engine's GC=0 ceiling continues to under-rate decks that are reasonably calibrated as Core.
- **`measured_bracket` vs `plays_like` disagreement is stable** at 56% (R7: 9/15). Same direction across all rounds: `plays_like` calls Exhibition where `measured_bracket` calls Core for midrange/artifact precons without a clean win condition. `estimatePlaysLike()` calibration recommendation from R2's findings still stands.

## Findings

### 1. **`Winning-combo floor` is a NEW B4 FP surface — 2/15 in R7** (severity: HIGH, **new in R7**)

R1-R5 documented the `Tuned-redundancy floor` predicate as the primary B4 FP driver. R7 surfaces a second floor predicate, labeled `Winning-combo floor` in the rationale with the comment "WotC carveout" — the same floor PR #517-era engine work added to honor the WotC bracket rules' explicit "2-card categorical-win combo = B4 minimum" rule.

The problem: **Freya's combo-line detector is over-firing**, flagging cluster pairs as 2-card categorical-win combos when they aren't.

**World Shaper (EoE Hearthhull)** — raw score 7 → B4:
```
[+3] Tutor density (12%+): 20% of nonlands
[+3] Combo lines (5+): 35 true-infinite/determined loops
[-1] Average CMC (heavy (>3.5)): 3.5 avg
[+2] Finisher density (8+): 10 distinct finisher lines
[floor] Winning-combo floor: lifted to B4: 2-card categorical-win combo present (was B3) — WotC carveout

Primary win line: Scouring Swarm + Rampaging Baloths
```

35 "true-infinite/determined loops" on a stock landfall precon is clearly a false-positive cluster of the shape R3 PR #532 already partly addressed (consume-once mechanics over-counted as loops). The Scouring Swarm + Rampaging Baloths "categorical-win" combo flag is the same overfit at finer grain — both cards trigger off lands entering, but they don't form a closed loop without external enablers.

**Cabaretti Cacophony (SNC Kitt Kanto)** — raw score 5 → B4:
```
[+1] Tutor density (4-7%): 6% of nonlands
[+1] Combo lines (1): 1 true-infinite/determined loop
[+1] Fast mana (3-5): 4 sub-2-CMC mana producers
[+2] Finisher density (8+): 56 distinct finisher lines
[floor] Winning-combo floor: lifted to B4: 2-card categorical-win combo present (was B3) — WotC carveout

Primary win line: Rose Room Treasurer + Scute Swarm
```

Same pattern: Scute Swarm requires lands to enter, not arbitrary engines. Rose Room Treasurer + Scute Swarm isn't a closed 2-card combo. 56 "distinct finisher lines" on a 100-card precon is also a finisher-detector blowup.

**Recommended fix surface widens.** The PR #513 fix (AND-chain) currently only gates the `Tuned-redundancy floor`. The same gate logic needs to apply to `Winning-combo floor`: don't lift to B4 on a 2-card combo detection unless EITHER (a) the combo is in the curated `KnownCombos` list (Thassa's Oracle, Demonic Consultation, Worldgorger Dragon, etc.) OR (b) corroborating signals exist (GC ≥ 1, true-infinite class label, or tutor density ≥ 0.08).

### 2. **`Tuned-redundancy floor` still firing, now on R7's Rebellion Rising + Graveyard Overdrive** (severity: HIGH, **pattern stable across 6 corpora**)

The exact predicate documented in R1 (Urza, Blast), R2 (Family Matters), R3 (Forces of Imperium, Creative Energy, Squirreled Away), R5 (Blame Game, Science!), and now R7:

**Rebellion Rising (ONE Neyali)** — raw 4 → B4:
```
[+2] Fast mana (6-9): 6 sub-2-CMC mana producers
[+2] Finisher density (8+): 21 distinct finisher lines
[floor] Tuned-redundancy floor: lifted to B4: 21 finishers + 6 fast-mana pieces (was B2)
```

**Graveyard Overdrive (MH3 Disa)** — raw 7 → ceiling B2 → floor B4:
```
[+2] Tutor density (8-11%): 9% of nonlands
[+2] Combo lines (2-4): 4 true-infinite/determined loops
[-1] Average CMC (heavy (>3.5)): 3.7 avg
[+2] Fast mana (6-9): 6 sub-2-CMC mana producers
[+2] Finisher density (8+): 26 distinct finisher lines
[ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
[floor] Tuned-redundancy floor: lifted to B4: 26 finishers + 6 fast-mana pieces (was B2)
```

Graveyard Overdrive's trace is the canonical ceiling-then-floor override pattern. The Disa precon has zero GC and zero true-infinite, the ceiling correctly caps at B2, then the floor immediately overrides back to B4 on the finisher+fast-mana redundancy signal. **Now 7/75 of all precons surveyed exhibit this exact override.** The PR #513 fix (gate the floor on GC/true-infinite/tutor density) would correctly demote Graveyard Overdrive but NOT Rebellion Rising (GC=0, true-infinite=0, tutor density not surfaced in rationale — likely below 4%). R3's α/β refinement (raise GC threshold OR require corroborating signals) would catch Rebellion Rising too. R7 confirms the fix is needed across BOTH floor predicates.

### 3. **Peace Offering (BLB Ms. Bumbleflower) is the second 81%+ power outlier** (severity: MEDIUM, **mirror to R5's Most Wanted finding**)

R5 flagged Most Wanted (OTJ Olivia) as the loudest "engine cooler than declared" miss at `power_percentile=81 / mana=A / measured B2 / declared B4`. R7's Peace Offering (BLB Ms. Bumbleflower) reproduces the exact same shape:

| Signal | Value |
|---|---|
| `power_percentile` | **81** (tied with R5's Most Wanted for highest in 75-deck corpus) |
| `mana_base_grade` | A |
| `commander_synergy` | 80.3% |
| `archetype` | midrange |
| `measured_bracket` | 2 (Core) |
| `declared_bracket` (per rule) | 4 (Optimized) |
| `Δ` | **+2** |

Same conclusion as R5: **either the declared-bracket heuristic's `power_pct ≥ 80 → B4` threshold is too aggressive for modern precons, OR the engine's GC=0 ceiling is under-rating tuned BLB-era product.** Most Wanted + Peace Offering being two independent BLB-era data points suggests the BLB / OTJ tuning cycle specifically is calibrated tighter than the engine's signal ladder accounts for. The next round (if Foundations C26 / EoE follow-up product warrants R8) should track whether this 81%-power-plus-A-mana cluster keeps showing up in WotC's flagship-era output.

### 4. **Silverquill Influence (C21 SoS Killian) Δ=+2 inverse-direction outlier** (severity: LOW)

`measured_bracket = 1 Exhibition` is the strongest "engine cooler than declared" miss in R7 — the only B1 measured call across all 75 decks in the combined corpus. Declared rule lifts to B3 on `power_pct=63 OR combo_density=3`. Engine reads: 3 win lines, 3 combo notes, but `power_percentile=63`, `mana_grade=B`, and the engine's archetype classifier calls it "enchantress" (a low-power-floor archetype). The measured-B1 call appears to be the engine's archetype-aware power tier knocking the deck below the B2 floor — possibly a `enchantress + low win-line count` edge case where the signal ladder doesn't have enough corroborating evidence to lift past B1.

Not a B4 FP (the other direction), but flagged here as a Δ=+2 outlier the engine should look at: an enchantress precon with 3 combo lines and 63% power shouldn't measure as B1 Exhibition.

## Cumulative findings across R1-R7 (75 decks)

| Predicate / observation | Cumulative count | Notes |
|---|:-:|---|
| `Tuned-redundancy floor` B4 FPs | 7/75 (9%) | Urza, Blast, Family Matters, Forces of the Imperium, Creative Energy, Squirreled Away, Blame Game, Science!, Rebellion Rising, Graveyard Overdrive (counts overlap with combo-floor FPs on Graveyard Overdrive) |
| `Winning-combo floor` B4 FPs (NEW in R7) | 2/75 (3%) | World Shaper, Cabaretti Cacophony |
| Total B4 false-positives | 12/75 (16%) | Stable upward trend R1 → R7 |
| 81%+ power_pct with measured B2 | 2/75 (3%) | Most Wanted (R5), Peace Offering (R7) — both BLB-era, suggests modern-precon tuning shift |
| `measured_bracket` vs `plays_like` disagree | ~42/75 (56%) | Stable across all 5 rounds; `estimatePlaysLike()` calibration recommendation from R2 still pending |

## Catalog gap analysis

R7's 15 picks knocked out the most-glaring gaps (Crimson Vow, SNC, Edge of Eternities, Brothers' War sibling) but the WotC precon catalog still has uncovered product. Best-effort inventory of what remains accessible on Moxfield in the "Commander Precons" namespace, sorted by likely R8 priority:

| Era / set | Missing precons | Note |
|---|---|---|
| C15 (Commander 2015) | All 5 (Plunder the Graves, Wade into Battle, Call the Spirits, Seize Control, Swell the Host) | Entire set untouched |
| C14 (Commander 2014) | Forged in Stone, Sworn to Darkness, Peer Through Time, Guided by Nature | 1/5 covered |
| C16 (Commander 2016) | Open Hostility, Stalwart Unity, Entropic Uprising, Invent Superiority | 1/5 covered |
| C17 (Commander 2017) | Feline Ferocity, Wizard's Mischief, Draconic Domination | 1/4 covered |
| C18, C19, C20 | 1-2 each missing | Mostly covered |
| AFR (Forgotten Realms) | Aura of Courage | 3/4 covered |
| LCI (Lost Caverns of Ixalan) | Ahoy Mateys, Blood Rites, Explorers of the Deep | 1/4 covered |
| MOM (March of the Machine) | Call for Backup, Tinker Time | Set had 5; we have 3 |
| MKM (Karlov Manor) | Revenant Recon | 3/4 covered |
| DMU (Dominaria United) | Legends' Legacy | 1/2 covered |
| FIN (Final Fantasy) | Counter Blast, Scions and Spellcraft, Revival Trance | 1/4 covered |
| DFT (Aetherdrift) | Endless Engines, Tempo Shift, Fast and Furious | 1/4 covered |
| PIP (Fallout) | Mutant Menace, Hail Caesar, Scrappy Survivors | 1/4 covered |
| DSK (Duskmourn) | Mind Your Manners | 3/4 covered |
| FDN (Foundations) | All — set may not have flagship precons; needs research | |
| EoE (Edge of Eternities) | Possibly more (only 2 picked in R7) | New product; partial WotC info |
| Secret Lair Commander 2026 | Goblin Storm + others surfaced in search | Speculative; product still rolling out |

**Estimated remaining accessible inventory: ~30-40 precons.** Coverage is NOT yet exhausted at the WotC catalog level. R8 could conservatively target another 15 picks at C14/C15/C16/C17 (the older sets where WotC's precon designers were less aggressive on the tuned-redundancy axis — useful for stress-testing whether the B4 FP rate trends UP with newer product as R1-R7 suggests it does).

## R7 picks — source URLs

15 source URLs (all `https://moxfield.com/decks/<id>` in the `Commander Precons` namespace, none overlapping R1, R2, R3, or R5):

| Precon | Moxfield ID |
|--------|-------------|
| World Shaper (EoE) | `z4iIQoHd4ECI0GNv5H1u3g` |
| Counter Intelligence (EoE) | `K_R2ARDl_0W6Bs-mVi-vCA` |
| Vampiric Bloodline (VOW) | `lava2X1Op0aSeUHhPC5cfQ` |
| Spirit Squadron (VOW) | `zoNqxklcjEyz2NZ9MI6jpA` |
| Mishra's Burnished Banner (BRO) | `2SN7rtbtuEy6rydE2U55gA` |
| Silverquill Influence (C21 SoS) | `zQLesJzHJEyCaNMw-GUT7w` |
| Everyone's Invited! (SL 2025) | `4LyEuAZJA0WFp-H1P-85VA` |
| Bedecked Brokers (SNC) | `LMu_N7hy3UWNP-HBkaw_xQ` |
| Cabaretti Cacophony (SNC) | `ONFl58ai-U-S3u539zDJQg` |
| Maestros Massacre (SNC) | `flLw5YvIOE2tBprZ_GTsmQ` |
| Obscura Operation (SNC) | `flG8SlpN50qt-L-rQ6VftQ` |
| Riveteers Rampage (SNC) | `fTgFMxk5cESAhOVIks00kA` |
| Rebellion Rising (ONE) | `RHTNEgHYMUigpM8XFwqUeQ` |
| Peace Offering (BLB) | `bTMR5Ab1PU-5UzXaQ_OsgQ` |
| Graveyard Overdrive (MH3) | `p9lI8QQGH0eEeJmaPX0KVQ` |

Re-import any deck via `go run ./cmd/hexdek-import/ --moxfield https://moxfield.com/decks/<id> --owner wizards` (Moxfield JSON is cached at `~/.cache/hexdek/moxfield`).

## Recommendations

1. **Extend the PR #513 fix to also gate the `Winning-combo floor` predicate.** Currently the fix decision surface only covers `Tuned-redundancy floor`. R7 surfaced 2 additional FPs from the combo-floor predicate (World Shaper, Cabaretti Cacophony) — same shape, same need for AND-chain gating on either KnownCombos membership OR corroborating GC/tutor signals.
2. **Re-investigate the combo-line detector's "true-infinite/determined loop" classifier.** World Shaper reports 35 such loops on a stock landfall precon; Cabaretti Cacophony reports 56 finisher lines. These are detector blowups of the same shape PR #530 partially addressed for cycling-loop combos. Land-based payoff cards (Scute Swarm, Rampaging Baloths, Scouring Swarm, Rose Room Treasurer) appear to be the next consume-once / external-enabler-dependent surface that needs to be coalesced out of the combo set.
3. **R8 (if scheduled) should bias toward C14/C15/C16/C17.** The 4 oldest sets (C13 has 2/5; C14 has 1/5; C15 has 0/5; C16 has 1/5; C17 has 1/4) are the corpus's weakest representation. R7's R5-comparison suggests modern precons are tuned tighter than older ones; sampling more of the older bracket would confirm whether the B4 FP rate uptrend is REAL or a sampling artifact.
4. **Investigate Most Wanted + Peace Offering as a BLB-era cluster.** Two BLB-era 81%+ power_pct A-mana midrange precons reading as measured B2 is unlikely to be coincidence. Either WotC's modern-precon tuning has shifted (deserving a calibration update) or the engine's GC=0 ceiling is too aggressive against modern tuned-but-no-GC product.
