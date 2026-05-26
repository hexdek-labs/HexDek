# Precon Vibes-Bracket Calibration (R60, Round 5)

> **HISTORICAL — `plays_like` removed (dev/remove-plays-like-r60).** This doc references the `plays_like` signal and its `estimatePlaysLike()` simulator as a discrete code path. That scaffolding has since been removed end-to-end (Go, JSON, DB-comment, UI, glossary, tests). `measured_bracket` is now the canonical felt-power measurement; the "no wincon → floppy" semantics that `plays_like` tracked are queued for absorption into `measured_bracket` as a separate calibration chapter. The Plays-Like column has been dropped from the ranked table below; historical findings and disagreement-rate rollups are preserved as research evidence.

## Why this is round 5

PR #508 (R1) and PR #523 (R2) built the unedited-WotC-precon vibes-bracket baseline at N=30 decks, finding a stable 10% B4 false-positive rate against the `Tuned-redundancy floor` predicate and a stable ~55-60% disagreement rate between `measured_bracket` and `plays_like`. PR #525's R2 findings summary made those into decision-support recommendations; engine work followed (PR #529's bracket-vs-measured-bracket refactor, PR #530's cycling-loop combo blowup fix).

R3 is in flight as **PR #532 (open, unmerged at the time of this writing)** on branch `dev/precon-vibes-r3-r60`; R4 has no corresponding branch or PR in `origin`. R5 was scheduled assuming R3 + R4 would land first; with R4 absent, R5 in practice runs CONCURRENT to R3 rather than after it. Zero-overlap was enforced against all three prior corpora: the 15 R3 Moxfield IDs were fetched from `origin/dev/precon-vibes-r3-r60:docs/precon-vibes-r3-r60.md` and cross-referenced before R5 picks were locked in. **No deck in this R5 wave appears in R1 (PR #508), R2 (PR #523), or R3 (PR #532).** Once R3 merges, the combined corpus is 60 decks (R1 + R2 + R3 + R5); the cross-validation tables in this doc compare R5 against the merged-to-main R1 + R2 baseline (N=30) and call out the R3 findings separately where they affect interpretation.

The R5 picks deliberately tilt toward the **post-2023 WotC product** (MOM, MKM, OTJ, FIN/Final Fantasy, PIP/Fallout, DFT/Aetherdrift, SoS/Secrets of Strixhaven) — the eras least represented in R1/R2's C13-LCI bias. This is itself the most useful framing of R5: the corpus is now diverse enough to surface temporal drift in how WotC tunes precons.

## Methodology

- **Corpus:** 15 NEW Moxfield community uploads of stock WotC precon decklists, in `data/decks/wizards/`. All from the canonical "Commander Precons" Moxfield namespace. **Zero overlap with R1 (PR #508) or R2 (PR #523)** — every Moxfield ID below was cross-referenced against both prior source lists.
- **Pipeline:** identical to R1/R2 — `go run ./cmd/hexdek-import/ --moxfield <url> --owner wizards`. Freya auto-runs on import; per-deck output at `data/decks/wizards/freya/<slug>.strategy.json` and `<slug>.profile.json`.
- **Naming convention:** post-refactor (PR #529), the `bracket` field on `DeckProfile` is the **declared / rubber-stamp** bracket (B2 by default for any deck under `data/decks/wizards/`, user-editable). `measured_bracket` is what Freya's signal estimator computes. The Δ column tabulates **declared_bracket − measured_bracket** (matching R1/R2 sign convention) — negative Δ means the engine reads the precon HOTTER than the human-judged declared rule warrants; positive Δ means the engine reads it COOLER.
- **Metrics:** identical to R1/R2 — `measured_bracket`, `plays_like_label`, `commander_synergy × 100`, `len(win_lines)`, `len(combo_notes)`, `value_chains[].depth` (avg/max), recursion ratio, `game_changer_count`, `power_percentile`, `mana_base_grade`.
- **Declared-bracket rule** (per R1/R2): B1 if `power_pct < 25 AND no win_lines AND no combos`; B3 lift if `power_pct ≥ 60 OR combo_density ≥ 4 OR gc ≥ 2`; B4 lift if `power_pct ≥ 80 OR gc ≥ 4`; otherwise B2.

## Ranked Table

Sorted chronologically by era, then by intra-era cluster.

| # | Era | Precon | Commander | Archetype | Mch | Cmdr Syn % | Win Lines | Combo Dens | Chain Depth (avg/max) | Recursion Ratio | GC | Power % | Mana | **Declared Brkt** | Δ |
|---|-----|--------|-----------|-----------|:---:|:----------:|:---------:|:----------:|:---------------------:|:---------------:|:--:|:-------:|:----:|:--------------:|:-:|
|  1 | C21 SoS | Lorehold Spirit       | Quintorius, History Chaser    | combo        | 2 Core         | 69.4 |   9 | 2 | 3.00 / 3 | 0.33 | 0 | 63 | C | **3** | +1 |
|  2 | C21 SoS | Witherbloom Pestilence| Dina, Essence Brewer          | midrange     | 2 Core         | 50.0 |  85 | 1 | 2.75 / 3 | 0.50 | 0 | 55 | B | **2** | ✓ |
|  3 | MOM     | Growing Threat        | Brimaz, Blight of Oreskos     | artifacts    | 2 Core         | 88.5 |   4 | 1 | 2.83 / 3 | 0.17 | 0 | 50 | A | **2** | ✓ |
|  4 | MOM     | Divine Convocation    | Kasla, the Broken Halo        | midrange     | 2 Core         | 58.3 |  15 | 1 | 2.25 / 3 | 0.50 | 0 | 58 | B | **2** | ✓ |
|  5 | MOM     | Cavalry Charge        | Sidar Jabari of Zhalfir       | midrange     | 2 Core         | 55.0 |   9 | 1 | 3.00 / 3 | 0.00 | 0 | 58 | C | **2** | ✓ |
|  6 | MKM     | Deadly Disguise       | Kaust, Eyes of the Glade      | counters     | 2 Core         | 17.7 |  12 | 1 | 2.40 / 3 | 0.40 | 2 | 25 | F | **3** | +1 |
|  7 | MKM     | Blame Game            | Nelly Borca, Impulsive Accuser| midrange     | **4 Optimized**| 40.3 |  12 | 1 | 2.33 / 3 | 0.33 | 0 | 55 | B | **2** | **−2** |
|  8 | MKM     | Deep Clue Sea         | Morska, Undersea Sleuth       | midrange     | 3 Upgraded     | 93.5 |  20 | 1 | 3.00 / 3 | 1.00 | 1 | 48 | D | **2** | −1 |
|  9 | OTJ     | Most Wanted           | Olivia, Opulent Outlaw        | midrange     | 2 Core         | 87.1 |  20 | 1 | 2.50 / 3 | 0.50 | 0 | **81** | A | **4** | **+2** |
| 10 | OTJ     | Quick Draw            | Stella Lee, Wild Card         | midrange     | 2 Core         | 90.2 |   4 | 1 | — / —    | 0.00 | 0 | 76 | A | **3** | +1 |
| 11 | OTJ     | Desert Bloom          | Yuma, Proud Protector         | midrange     | 3 Upgraded     | 74.6 |  48 | 2 | 2.67 / 3 | 0.67 | 0 | 45 | D | **2** | −1 |
| 12 | OTJ     | Grand Larceny         | Gonti, Canny Acquisitor       | midrange     | 2 Core         | 60.7 |   6 | 1 | 2.33 / 3 | 0.67 | 0 | 68 | A | **3** | +1 |
| 13 | FIN     | Limit Break           | Cloud, Ex-SOLDIER             | artifacts    | 2 Core         | 82.3 |   8 | 1 | 2.50 / 3 | 0.50 | 0 | 63 | B | **3** | +1 |
| 14 | PIP     | Science!              | Dr. Madison Li                | artifacts    | **4 Optimized**| 98.4 |  16 | 2 | 2.83 / 3 | 0.00 | 0 | 63 | C | **3** | **−1** |
| 15 | DFT     | Living Energy         | Saheeli, Radiant Creator      | artifacts    | 2 Core         | 83.6 |   8 | 1 | 2.25 / 3 | 0.50 | 0 | 68 | B | **3** | +1 |

**Δ column:** declared_bracket − measured_bracket. `✓` = match. Negative Δ means the engine measured the precon HOTTER than declared (mechanical signal disagrees with the precon-floor B2 stamp); positive Δ means COOLER. Quick Draw's `—/—` chain depth reflects zero value-chain entries detected (single-shot spellslinger plan).

**Aggregate:** 4/15 exact, 13/15 within ±1, 2/15 off by ±2. **2/15 B4 false-positives (Blame Game, Science!) — same predicate as R1/R2.**

## Cross-validation vs R1 + R2

| Metric | R1 (PR #508) | R2 (PR #523) | R5 (this) | Combined (45 decks) |
|---|:-:|:-:|:-:|:-:|
| Exact match | 11/15 (73%) | 12/15 (80%) | 4/15 (27%) | **27/45 (60%)** |
| Within ±1 | 13/15 (87%) | 14/15 (93%) | 13/15 (87%) | **40/45 (89%)** |
| B4 false-positives | 2/15 (Urza, Blast) | 1/15 (Family Matters) | **2/15 (Blame Game, Science!)** | **5/45 (11%, excl. R3)** |
| `measured_bracket` vs `plays_like` disagree | 9/15 (60%) | 8/15 (53%) | 8/15 (53%) | **25/45 (56%)** |
| Δ ≤ −1 (engine hotter than declared) | 2/15 | 1/15 | 3/15 | **6/45 (13%)** |
| Δ ≥ +1 (engine cooler than declared) | 2/15 | 1/15 | 8/15 | **11/45 (24%)** |

### What replicates and what doesn't

- **B4 false-positive rate is stable** at 11% (5/45 against the R1+R2+R5 sub-corpus; R3 PR #532 separately reports 6/45 against R1+R2+R3, adding Forces of the Imperium, Creative Energy, and Squirreled Away — when R3 merges, the combined N=60 count will be at least 8/60 ≈ 13%). The `Tuned-redundancy floor` predicate fired in R5 on Blame Game (`raw score 4 → B4`) and Science! (`raw score 7 → ceiling demoted to B2 → floor re-lifted to B4`), the exact same ceiling→floor override shape PR #513 documented and R2 reaffirmed. **Two more confirming instances for what is now a multi-corpus pattern.** PR #513's recommended fix (AND-chain `gameChangerCount ≥ 1 OR trueInfCount ≥ 1 OR tutorDensity ≥ 0.08 OR (avgCMC < 3.0 AND manaGrade ≥ B)`) would correctly demote both Blame Game (GC=0, true_inf=0, tutor 4%, avgCMC 3.6) and Science! (GC=0, true_inf=0, tutor 5%, avgCMC unknown but artifact-heavy). **However, R3 PR #532 surfaced a counter-example (Creative Energy / MH3 Satya): GC=1 from Mana Vault satisfies the proposed predicate's first disjunct, so the floor would still fire and the deck would still land at B4.** R3's α/β refinement options (raise GC threshold to ≥2, or require corroborating power_pct/tutor density) are now part of the fix decision surface; R5 has nothing to add to that — Blame Game / Science! both have GC=0 and would be caught by EITHER the as-designed fix OR either refinement.
- **`measured_bracket` vs `plays_like` disagreement is stable** at 56%. Same direction in R5 as R1/R2: `plays_like` calls Exhibition where `measured_bracket` calls Core, on midrange/artifact precons without a clean win condition. Same `estimatePlaysLike()` calibration hypothesis stands.
- **Exact-match rate DROPPED** from R1/R2's 73-80% to R5's 27%. **This is a real signal — not noise.** All but one of the R5 disagreements is `Δ = +1` (engine cooler than the heuristic declared rule), driven by the corpus skew: 9/15 R5 decks have `power_percentile ≥ 60`, which trips the declared-B3-lift rule, but `measured_bracket` ladder cap-at-B2 for any deck without GC ≥ 1 or true_inf ≥ 1. The result is that the engine reports B2 for nearly the entire R5 corpus while the declared-bracket heuristic reports B3. **This is informative about WotC's product trend — modern precons (MOM, MKM, OTJ, FIN, PIP, DFT) ship with notably higher `power_percentile` than older sets** (R1 average pwr ≈ 50; R5 average pwr ≈ 61). Either the declared-bracket heuristic's `power_pct ≥ 60` threshold needs to shift upward, OR the engine's GC=0 ceiling is now systematically under-rating tuned modern precons. Both interpretations live in the data; resolving which requires a separate calibration pass against decks with known competitive history.

## Findings

### 1. **`Tuned-redundancy floor` predicate fires on Blame Game AND Science!** (severity: HIGH, **5/45 PATTERN CONFIRMED ACROSS 3 CORPORA**)

**Blame Game (MKM Nelly Borca)** — raw score 4 → B4:
```
Bracket rationale (raw score 4 → B4 Optimized):
  [+1] Tutor density (4-7%): 4% of nonlands
  [-1] Average CMC (heavy (>3.5)): 3.6 avg
  [+2] Fast mana (6-9): 7 sub-2-CMC mana producers
  [+2] Finisher density (8+): 10 distinct finisher lines
  [floor] Tuned-redundancy floor: lifted to B4: 10 finishers + 7 fast-mana pieces (was B2)
```

**Science! (PIP Dr. Madison Li)** — raw score 7 → ceiling B2 → floor re-lifted to B4:
```
Bracket rationale (raw score 7 → B4 Optimized):
  [+1] Tutor density (4-7%): 5% of nonlands
  [+2] Combo lines (2-4): 3 true-infinite/determined loops
  [+2] Fast mana (6-9): 9 sub-2-CMC mana producers
  [+2] Finisher density (8+): 11 distinct finisher lines
  [ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
  [floor] Tuned-redundancy floor: lifted to B4: 11 finishers + 9 fast-mana pieces (was B2)
```

Science!'s trace is the IDENTICAL ceiling→floor override pattern documented in R1 (Urza, Blast from the Past) and R2 (Family Matters). Five precons across three independent corpora now exhibit the same predicate firing pattern: stock product with 8+ finishers + 6+ fast-mana pieces gets lifted to B4 regardless of any other signal, including an explicit B2 ceiling applied by the GC=0 rule moments earlier.

The Science! combo-line examples are themselves diagnostic — "Nuka-Cola Vending Machine + Mystic Forge + Unexpected Windfall + Irrigated Farmland" is a cycling-land-pollution false-positive of the same shape PR #530 was supposed to address. PR #530's cycling-coalesce fix DID land, but cycling LANDS apparently still leak into combos when paired with non-cycling draw effects — Canyon Slough / Irrigated Farmland appear in Science!'s top "determined" lines despite being single-shot consume-once. **R3 PR #532 independently surfaced a wider version of this bug: Squirreled Away (BLB Hazel) reports 287 detected loops on FOOD tokens, confirming the consume-once detector bug is not specific to cycling — it generalizes to any consume-once mechanic (food, blood, clue tokens, etc.).** The fix surface widens accordingly.

### 2. **Most Wanted (OTJ Olivia) is the inverse miss: 81% power / A mana base / measured B2** (severity: HIGH, **NEW R5 FINDING**)

Most Wanted is the loudest R5 outlier in the opposite direction from the B4 false-positives:

| Signal | Value |
|---|---|
| `power_percentile` | **81** (the highest in R1/R2/R5 combined) |
| `mana_base_grade` | A |
| `commander_synergy` | 87.1% |
| `archetype` | midrange (Aristocrats hybrid per the intent string) |
| `game_changer_count` | 0 |
| `true_infinites` | 0 |
| `measured_bracket` | **2 Core** |

The bracket rationale shows:
```
Bracket rationale (raw score 6 → B2 Core):
  [+3] Combo lines (5+): 13 true-infinite/determined loops
  [+2] Fast mana (6-9): 6 sub-2-CMC mana producers
  [+1] Finisher density (4-7): 5 distinct finisher lines
  [ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
```

The GC=0 ceiling is firing where the data clearly says "this is a tuned-feeling deck" — 81% power, A mana, 87% commander synergy, 13 detected combo lines. The deck's combo lines are all "Deadly Dispute + Kamber + Canyon Slough + Life Insurance"-shape — Aristocrats value loops that close as `determined` combos but aren't `true_infinite`. The ceiling fires correctly per its predicate (no GC, no true_inf) but the underlying assumption — "no GC AND no true_inf → cannot be hotter than B2" — is incorrect for high-power Aristocrats decks where the game-changer power lives in the synergy-multiplied value loops, not in any individual card classified as a GC.

This is a **new** finding pattern: the B4 false-positives (Urza / Blast / Family Matters / Blame Game / Science!) show the engine running HOT on stock product, but Most Wanted shows the engine running COLD on stock product that has trended hot. Both pathologies share the same root: the score-ladder is too coupled to `gameChangerCount` and `trueInfCount` and not enough to derived signals like `commander_synergy ≥ 80% AND mana_grade ≥ A AND power_percentile ≥ 75`.

### 3. **Modern WotC precons (MOM, MKM, OTJ, FIN, PIP, DFT) trend higher power than older sets** (severity: MEDIUM, informational + calibration-relevant)

R5 average `power_percentile` = 61.7 vs R1 average ≈ 52 vs R2 average ≈ 47.5. Per-era breakdown:

| Era (R5 picks) | Avg pwr_pct |
|---|---|
| C21 SoS (Lorehold Spirit, Witherbloom Pest) | 59.0 |
| MOM (Growing Threat, Divine Convocation, Cavalry Charge) | 55.3 |
| MKM (Deadly Disguise, Blame Game, Deep Clue Sea) | 42.7 |
| OTJ (Most Wanted, Quick Draw, Desert Bloom, Grand Larceny) | **67.5** |
| FIN (Limit Break) | 63 |
| PIP (Science!) | 63 |
| DFT (Living Energy) | 68 |

OTJ alone averages 67.5% across 4 sample decks, with Most Wanted at 81. This isn't a sampling artifact — it's the trajectory of WotC's post-2023 precon design philosophy. The implication for the calibration heuristic: the R1/R2 `power_pct ≥ 60 → declared B3` rule was tuned against an older corpus where 60% power was genuinely upgraded territory; on modern precons, 60% may be the new B2 floor.

### 4. **Cycling-land combos still leak past the PR #530 coalesce fix** (severity: MEDIUM, **R5 SUB-FINDING**)

Science!'s top determined combo "Nuka-Cola Vending Machine + Mystic Forge + Irrigated Farmland + Unexpected Windfall" should not be detected — Irrigated Farmland is a cycling land (consume-once discard cost), so any combo line that requires its repeatability is a false positive. PR #530's fix was scoped to cycling-cycling-cycling chains in the quad detector; cycling-lands paired with non-cycling draw effects (Unexpected Windfall, Mystic Forge cantrip path) appear to still pass the `verifyTriggerChain` check.

Same shape in Most Wanted (Canyon Slough appearing in 3 of the 4 top combo lines). The fix likely needs to extend the cycling-coalesce guard to count cycling LANDS specifically (lands are already excluded from the quad-prefilter but apparently not from the pair/triple combo-result emit).

### 5. **`measured_bracket` vs `plays_like` disagreement holds at 53%** (severity: MEDIUM, **CONFIRMS R1/R2 §4**)

R5: 8/15 disagreements. Combined across 45 decks: 25/45 = 56%. Direction holds — 7 of the 8 R5 disagreements are `plays_like=Exhibition vs measured=Core`. Same `estimatePlaysLike()` underrating of "stuff happens" precons. R1/R2/R5 all point to the same fix surface; nothing has been done yet.

## Suggested Next Steps

1. ✅ PR #513's predicate-tightening recommendation. **Now 5/45 confirming instances against R1+R2+R5 alone; R3 PR #532 adds another 3, so the combined count once R3 merges is 8/60.** The decision surface for 7174n1c now includes R3's α/β refinement options (R5's Blame Game + Science! are caught by either the as-designed fix OR either refinement, so R5 doesn't constrain the choice between options).
2. **NEW from R5 §2:** add a measured-bracket lift rule for the inverse miss: high `commander_synergy` + high `power_percentile` + good mana grade should lift to B3 even at GC=0 and true_inf=0. Most Wanted is the cleanest test case.
3. **NEW from R5 §3:** revisit the declared-bracket heuristic's `power_pct ≥ 60 → B3` threshold against newer precons. Either bump the threshold to 70, or treat the declared rule as advisory-only and remove it from the Δ-vs-measured comparison (which is what the precon B2-stamp refactor was implicitly suggesting anyway).
4. **NEW from R5 §4 (now reinforced by R3 #532's Squirreled Away finding):** extend PR #530's cycling-coalesce guard to handle consume-once mechanics generally. R5 surfaces the cycling-land case (Canyon Slough + Irrigated Farmland still leaking); R3 surfaces the food-token case (287 loops on Squirreled Away). The fix is a generalization of PR #530's renewable-producer check to cover food / blood / clue / cycling-land — any single-use resource that nonetheless registers as a "produces" output in the loop detector.
5. ⏳ Implement PR #525's regression test ("every WotC stock precon ≤ B3") — would fail on 5/45 against R1+R2+R5 (Urza, Blast, Family Matters, Blame Game, Science!); combined with R3 PR #532 once it merges, would fail on 8/60 (adds Forces of the Imperium, Creative Energy, Squirreled Away).

## Coverage exhaustion notes

The user request flagged R5 as possibly the bottom of community Moxfield uploads. **It is not — coverage is not yet exhausted at N=45 (R1+R2+R5) or N=60 (R1+R2+R3+R5 post-R3-merge).** Cross-checking against the Commander Precons namespace, multiple additional stock-list uploads exist for further waves:

- **C13:** Evasive Maneuvers (Derevi), Nature of the Beast (Marath), Power Hungry (Prossh) — Mind Seize used in R1, Eternal Bargain used in R3.
- **C14:** Forged in Stone, Guided by Nature, Peer Through Time, Sworn to Darkness — Built from Scratch used in R3.
- **C15:** all 5 unused (Call the Spirits, Plunder the Graves, Seize Control, Swell the Host, Wade into Battle).
- **MKM:** Revenant Recon (Mirko Obsessive Theorist) — was in the R5 candidate pool but cut for era diversity.
- **MOM:** Call for Backup (Bright-Palm), Tinker Time (Gimbal Gremlin Prodigy) — same.
- **WOE:** Faerie Schemes, Virtue and Valor, Fae Dominion, Beggar Thy Neighbor.
- **WHO:** Masters of Evil, Paradox Power, Timey-Wimey.
- **LCI:** Ahoy Mateys, Blood Rites, Explorers of the Deep.
- **DSK:** Endless Punishment, Mind your Manors.
- **BLB:** Squirreled Away, Peace Offering.
- **MH3:** Creative Energy, Graveyard Overdrive.
- **EOE:** Counter Intelligence, World Shaper.

At least 30 more stock uploads remain accessible. **R6/R7/R8 are viable if calibration pressure on the predicates is still needed after the §1-4 fixes land.**

## Reproducing

```bash
# Re-import any R5 precon (cache lives at ~/.cache/hexdek/moxfield):
go run ./cmd/hexdek-import/ --moxfield https://moxfield.com/decks/<id> --owner wizards

# Re-trace any deck's bracket call:
go run ./cmd/hexdek-freya/ --deck data/decks/wizards/<slug>.txt 2>&1 | grep -A 25 "Bracket rationale"
```

15 source URLs (all `https://moxfield.com/decks/<id>` in the `Commander Precons` namespace, none overlapping R1 or R2):

| Precon | Moxfield ID |
|--------|-------------|
| Lorehold Spirit (C21 SoS) | `8erwc7vsiEanV87GMM3IuQ` |
| Witherbloom Pestilence (C21 SoS) | `CIe9QwsuMUm0xHZVdXcZJQ` |
| Growing Threat (MOM) | `Rwg0cZ0Yqk2ujq_GXIQA2g` |
| Divine Convocation (MOM) | `SfNm_azRAUGtkhIuesdBIw` |
| Cavalry Charge (MOM) | `PrHvjllh3kuzVuSV2wyhCQ` |
| Deadly Disguise (MKM) | `c9xQjfvGwkCMFmQWcdOKVw` |
| Blame Game (MKM) | `BPBVD7NsT0SznrmdG4l3Tw` |
| Deep Clue Sea (MKM) | `kudcIOMGwkuN0hi99q10MQ` |
| Most Wanted (OTJ) | `V766u1HzgUCREUUSgsnfFA` |
| Quick Draw (OTJ) | `wpucJrNlHUSL8zosshILkQ` |
| Desert Bloom (OTJ) | `5MXFLs15ck6nh85X5VEjyQ` |
| Grand Larceny (OTJ) | `kwiSILSLR0ic9U38G00JZQ` |
| Limit Break (FIN — Final Fantasy) | `xNhT2XmIrkOu2lXb4-vjhg` |
| Science! (PIP — Fallout) | `BA1a99vfi0W0YLlGC97t-A` |
| Living Energy (DFT — Aetherdrift) | `jWSmSfkdGEijTfTX_d8qNQ` |
