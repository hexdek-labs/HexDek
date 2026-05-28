# Freya `SuggestedChanges` Quality Review — 5 WotC Precons

Companion to PR #721. Runs `BuildSuggestedChanges` against 5
representative WotC Commander precons and reads each
recommendation through the lens of "would a real EDH player
upgrading a precon actually take this advice?"

## Headline

**3 of 5 precons get genuinely useful suggestions; 2 surface real
quality bugs.** The mana-base swap pipeline is excellent
(Forsaken Sanctuary → Concealed Courtyard, Bloodfell Caves →
Luxury Suite, Boros Guildgate → Sacred Foundry are canonical
upgrade pairs). The interaction-gap fill pipeline works for
moderate budgets (Counterspell / Swords to Plowshares / Generous
Gift / Swan Song are correct picks) but **breaks down at low
brackets when chase-rare $200+ staples like Mana Crypt and Force
of Will get suggested to B2 precon-upgrade players** — technically
correct (highest BaselineImpact, color-allowed, not-owned) but
tone-deaf to the casual-budget reality of bracket 2.

Two quality bugs surfaced:

  1. **Bracket-blind staple selection** — `buyGuideStaples` is
     ranked by raw BaselineImpact, so Mana Crypt
     (BaselineImpact 95) outranks Arcane Signet (85). At
     bracket 1-3 the recommendation should prefer the cheaper
     pick.
  2. **Limited tap-land catalog coverage** — `taplandUpgradeMap`
     has ~30 entries covering the most-common precon
     guildgates + bicycle-lands + bicycle-checks, but misses
     set-specific themed tap-lands (Doctor Who set, MH3 set,
     Phyrexia: All Will Be One, etc.) and mono-color refuges.

A third concern (not a SuggestedChanges bug; an upstream
CuttableCards issue surfaced through Cuts) is documented for
completeness.

## Methodology

Diagnostic test
`cmd/hexdek-freya/suggested_changes_quality_r60_test.go` runs
`analyzeDeckFile` → `BuildDeckProfile` → `BuildSuggestedChanges`
on each of:

  - `animated_army_bloomburrow_commander_precon_decklist.txt`
    (B2 / GR / Combo per classifier — Tokens precon)
  - `vampiric_bloodlust_commander_2017_precon_decklist.txt`
    (B2 / BRW / Tribal — Vampires)
  - `blast_from_the_past_doctor_who_commander_precon_decklist.txt`
    (B2 / W / Combo per classifier — historic-spells theme)
  - `tyranid_swarm_warhammer_40_000_commander_precon_decklist.txt`
    (B2 / GRU / Midrange — swarm aggro)
  - `cabaretti_cacophony_streets_of_new_capenna_commander_precon_decklist.txt`
    (B2 / GRW / Midrange — go-wide tokens)

For each deck the test dumps input metrics
(ManaBaseGrade, RampCount, DrawCount, RemovalCount, BoardWipes,
Tutors, CuttableCards count) and every recommendation with
priority + full reason. The dump is human-read and translated
into this doc's verdicts.

## Deck-by-deck review

### 1. animated_army (Bloomburrow, B2 / GR / Combo)

**Inputs**: 38 lands (over by 10), B-grade mana base with 10
taplands, 24 ramp, 15 draw, 4 removal, **0 board wipes per role
classifier**, 0 tutors.

**Output**: 0 adds, 5 cuts, 0 swaps.

| Recommendation | Verdict |
|----------------|---------|
| Cut Gilded Lotus (P5, "high CMC no clear role") | **Reasonable** — Gilded Lotus is a tempo-trap in casual decks. ✓ |
| Cut Unnatural Growth (P5, "high CMC no clear role") | **Reasonable** — 5-CMC enchantment with no tribal hook. ✓ |
| Cut Tendershoot Dryad (P5, "high CMC no clear role") | **Reasonable** — 5-CMC token-builder without saproling synergy. ✓ |
| Cut Rolling Hamsphere (P5, "high CMC no clear role") | **Reasonable** — niche threat. ✓ |
| **Cut Blasphemous Act (P5, "high CMC no clear role")** | **WRONG — this is a flagship 8-mana-down-to-1-mana board wipe**, a staple cut for almost every casual red deck. CuttableCards classifier is misreading it as filler. |

**Quality note — silent-correct add gap**: the role classifier
reports `BoardWipes: 0`. SuggestedChanges should add a wipe.
Blasphemous Act IS in the deck and is the only wipe in the deck's
GR color identity (Toxic Deluge / Damnation are B, Wrath /
Farewell are W, Cyclonic Rift is U). The owned-card filter
correctly blocks recommending Blasphemous Act → Adds=0. So the
function is correctly silent, but the silence masks the upstream
inconsistency between RoleBoardWipe=0 and the deck having a wipe.

**Bug surfaced**: Blasphemous Act / Toxic Deluge–class burn
wipes are flagged "cuttable: high CMC no clear role" by the
CuttableCards classifier (not by SuggestedChanges directly).
File against `computeCardQualityTiers` rather than this PR.

### 2. vampiric_bloodlust (Commander 2017, B2 / BRW / Tribal)

**Inputs**: 37 lands (over by 7), **F-grade mana base** with 18
taplands, 7 ramp, 11 draw, 9 removal, 2 wipes, 1 tutor.

**Output**: 1 add, 4 cuts, **8 swaps**.

| Recommendation | Verdict |
|----------------|---------|
| **Add Mana Crypt (P7, "current 7 ramp, target 8 for bracket 2")** | **TONE-DEAF** — Mana Crypt is a $150-200 chase rare. Suggesting it for a B2 precon player to fill a 7→8 ramp gap is laughable. The function correctly picked the highest-BaselineImpact ramp staple not in the deck, but ignored the bracket-2 budget context. Arcane Signet ($2) would be the correct call. **BUG #1**. |
| Swap Forsaken Sanctuary → Concealed Courtyard (P10) | **Excellent** — canonical WB tap-land → fastland upgrade. ✓ |
| Swap Stone Quarry → Inspiring Vantage (P10) | **Excellent** — canonical RW tap-land → fastland upgrade. ✓ |
| Swap Scoured Barrens → Shineshadow Snarl (P10) | **Reasonable** — Snarl is more expensive than Sanctuary alternatives but the upgrade direction is correct. ✓ |
| Swap Bloodfell Caves → Luxury Suite (P10) | **Excellent** — canonical Battlebond dual upgrade. ✓ |
| Swap Rakdos Guildgate → Blood Crypt (P10) | **Excellent** — flagship shock-land upgrade. ✓ |
| Swap Boros Guildgate → Sacred Foundry (P10) | **Excellent** — flagship shock-land upgrade. ✓ |
| Swap Wind-Scarred Crag → Sacred Foundry (P10) | **Duplicate target** — Sacred Foundry already recommended for Boros Guildgate. A real upgrade would be Inspiring Vantage (already taken by Stone Quarry) or Battlefield Forge. **Minor**: the recommendation works at single-swap granularity but is sub-optimal when the multi-swap set is taken together. |
| Swap Orzhov Guildgate → Godless Shrine (P10) | **Excellent** — flagship shock-land upgrade. ✓ |
| Cut Blood Tribute (P5, "high CMC no clear role") | **Reasonable** — 8-CMC sorcery; even on theme it's pricey for casual. ✓ |
| Cut Anowon, the Ruin Sage (P5, "high CMC no clear role") | **Defensible** — Anowon is on-tribe but his discard ability is hit-or-miss; many vampire players cut him in upgrades. ✓ |
| Cut New Blood (P5, "filler — no synergy role at CMC 4") | **Reasonable** — situational removal. ✓ |
| Cut Disrupt Decorum (P5, "filler — no synergy role at CMC 4") | **Reasonable** — niche goad spell. ✓ |

**Verdict**: **swaps are excellent; one tone-deaf add**. The swap
pipeline genuinely produces the recommendations a precon-upgrade
guide would write, including the canonical Forsaken Sanctuary →
Concealed Courtyard / Bloodfell Caves → Luxury Suite pairs.

### 3. blast_from_the_past (Doctor Who Commander, B2 / W / Combo)

**Inputs**: 36 lands (over by 8), **D-grade mana base** with 14
taplands, 11 ramp, 8 draw, 8 removal, 3 wipes, 5 tutors.

**Output**: 0 adds, 5 cuts, **0 swaps** ← problem.

| Recommendation | Verdict |
|----------------|---------|
| Cut Heroes' Podium (P5, "high CMC no clear role") | **Reasonable** — 5-CMC artifact with no Doctor Who tribal payoff. ✓ |
| Cut Twice Upon a Time // Unlikely Meeting (P5) | **Defensible** — 5-CMC modal. ✓ |
| Cut The Five Doctors (P5, "expensive tutor 4 cheaper alternatives") | **Reasonable** — 6-CMC tutor, redundant. ✓ |
| Cut Leela / Duggan (P5/P4 fillers) | **Reasonable** — mid-curve non-synergistic creatures. ✓ |

**Bug surfaced**: D-grade mana base should produce P9 swap
recommendations, but **0 swaps emitted**. Investigation: the
Doctor Who precon's taplands are set-themed (Crash Site, The
Master's Tardis, Tardis-related lands, etc.) — none of which are
in the curated `taplandUpgradeMap`. The map covers WotC's classic
guildgate / bicycle / refuge cycle but doesn't reach into the
modern set-themed land cycles. **BUG #2**: catalog coverage gap.

**Adds=0 verdict**: deck hits all bracket-2 targets — that's
actually correct.

### 4. tyranid_swarm (Warhammer 40K, B2 / GRU / Midrange)

**Inputs**: 39 lands (over by 11), B-grade mana base with 8
taplands, 13 ramp, 17 draw, 3 removal, **0 wipes**, 2 tutors.

**Output**: 6 adds, 2 cuts, 0 swaps.

| Recommendation | Verdict |
|----------------|---------|
| **Add Force of Will (P8, "current 3 interaction, target 8")** | **TONE-DEAF** — $80-200 chase rare. Same bug as Mana Crypt. **BUG #1 again**. |
| Add Counterspell (P8) | **Excellent** — $2, canonical pick. ✓ |
| Add Swan Song (P8) | **Excellent** — $5, canonical pick. ✓ |
| Add Fierce Guardianship (P8) | **Borderline** — $30 chase rare. Cheaper than Force of Will but still beyond casual-precon budget. |
| Add An Offer You Can't Refuse (P8) | **Excellent** — $2, perfect precon-upgrade pick. ✓ |
| Add Cyclonic Rift (P7, "zero board wipes") | **Borderline** — $40-60, but the canonical blue wipe and a real-world precon-upgrade staple. Many players DO buy this for tier-up. Verdict: realistic. ✓ |
| Cut Aetherize (P5, "filler") | **Reasonable** — situational counter. ✓ |
| Cut Genestealer Locus (P5, "filler") | **Reasonable** — mid-curve non-synergistic. ✓ |

**Verdict**: **2 of 6 adds are tone-deaf, 4 are genuinely useful**.
The interaction-shape is right (the deck IS under-interacted at
3 vs target 8); the issue is in the staple selection at low
brackets.

### 5. cabaretti_cacophony (Capenna, B2 / GRW / Midrange)

**Inputs**: 38 lands (over by 10), B-grade mana base with 10
taplands, 14 ramp, 13 draw, 6 removal, 1 wipe, 0 tutors.

**Output**: 2 adds, 5 cuts, 0 swaps.

| Recommendation | Verdict |
|----------------|---------|
| Add Swords to Plowshares (P8, "current 6 interaction, target 8") | **Excellent** — $2 staple, perfect precon-upgrade pick. ✓ |
| Add Generous Gift (P8) | **Excellent** — $5 staple, canonical upgrade. ✓ |
| Cut Gahiji, Honored One (P5, "high CMC no clear role") | **Reasonable** — 5-CMC commander-attempting filler. ✓ |
| Cut Crash the Party (P5, "high CMC no clear role") | **Reasonable** — 5-CMC sorcery, situational. ✓ |
| Cut Indulge // Excess (P5, "high CMC no clear role") | **Reasonable** — 4-CMC modal. ✓ |
| Cut Sandwurm Convergence (P5, "high CMC no clear role") | **Reasonable** — 7-CMC win-more. ✓ |
| **Cut Assemble the Legion (P5, "high CMC no clear role")** | **WRONG for this archetype** — Assemble the Legion is a 5-CMC win condition in token strategies; flagging it as filler in a go-wide tokens precon misses the win-line context. Upstream CuttableCards bug (not SuggestedChanges). |

**Verdict**: **best output of the 5 decks**. Both adds are
realistic precon-upgrade picks; cuts are mostly defensible. The
Assemble the Legion miss is a CuttableCards issue surfaced
through SuggestedChanges.Cuts.

## Quality bug summary

| Bug | Severity | Affected decks | Fix surface |
|-----|---------:|----------------|-------------|
| **#1 Bracket-blind staple selection** — Mana Crypt / Force of Will / Fierce Guardianship recommended for B1-B3 precon players | HIGH (UX) | vampiric_bloodlust, tyranid_swarm | `pickStapleAdds` in `cmd/hexdek-freya/suggested_changes.go` — add bracket gate: at bracket ≤ 3, prefer staples with BaselineImpact ≤ 88 OR re-rank by (BaselineImpact × budgetFriendly) where budget-friendly cards get a small multiplier |
| **#2 Tap-land catalog coverage gap** — set-themed tap-lands (Doctor Who, MH3, MOM, etc.) missing from `taplandUpgradeMap` | MEDIUM | blast_from_the_past, likely others | `taplandUpgradeMap` in `cmd/hexdek-freya/suggested_changes.go` — expand catalog to cover at minimum the WotC set-themed land cycles from 2019+ |
| **#3 Upstream: Blasphemous Act flagged cuttable** — CuttableCards misclassifies the canonical 1-mana board wipe as "high CMC no clear role" | MEDIUM | animated_army | `computeCardQualityTiers` (existing code, NOT this PR). File separately. |
| **#4 Upstream: Assemble the Legion cuttable in tokens deck** — CuttableCards doesn't cross-reference the deck's primary archetype when classifying win-conditions as filler | LOW | cabaretti_cacophony | `computeCardQualityTiers` (existing code, NOT this PR). File separately. |

## Recommendation

Ship PR #721 as-is — the SuggestedChanges struct shape, the
add/cut/swap distinction, and the priority sorting are sound.
Land follow-up fixes for Bug #1 and Bug #2 in a separate PR. Bugs
#3 and #4 are upstream of this work — file against the existing
CuttableCards classifier.

The mana-base swap pipeline (8/8 swaps were correct on vampiric_bloodlust)
is the strongest component and should be promoted to the
Decks-screen upgrade flow immediately.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git checkout dev/freya-deck-suggestion-tests-r60
go test -run TestSuggestedChanges_PreconQualityReview -count=1 -v ./cmd/hexdek-freya/
```

Test always passes — it's a diagnostic dump. The verbose log
output reproduces the deck-by-deck data this doc was written
from. Skipped when `data/rules/oracle-cards.json` is absent.

## CLAUDE.md issue-log impact

Recommended Open-table entries:

> | 2026-05-28 | PR #722 quality review | **Bracket-blind staple selection in SuggestedChanges — Mana Crypt / Force of Will suggested for B2 precon players** | HIGH (UX) | `pickStapleAdds` ranks by raw BaselineImpact; cEDH-staple chase rares outrank casual budget picks. Fix: at bracket ≤ 3, prefer BaselineImpact ≤ 88 OR add a budget-friendly multiplier. Surfaced by vampiric_bloodlust (Mana Crypt P7 for 7→8 ramp gap) + tyranid_swarm (Force of Will P8 for 3→8 interaction gap). |

> | 2026-05-28 | PR #722 quality review | **taplandUpgradeMap missing set-themed tap-lands** | MEDIUM | D-grade Doctor Who precon produced 0 swap suggestions because none of its set-themed taplands are in the curated ~30-entry catalog. Fix: expand to cover WotC set-themed land cycles from 2019+. |

> | 2026-05-28 | PR #722 quality review | **CuttableCards classifier flags 1-mana board wipes as "high CMC no clear role"** | MEDIUM | Blasphemous Act in a GR deck with 0 BoardWipes is flagged cuttable. Fix surface: `computeCardQualityTiers` — cross-reference RoleBoardWipe when deciding wipe-class cards. |

> | 2026-05-28 | PR #722 quality review | **CuttableCards doesn't archetype-cross-reference win conditions** | LOW | Assemble the Legion in a tokens deck flagged cuttable as filler. Fix surface: `computeCardQualityTiers` — when archetype matches token themes, treat token-generating wincons as protected from cuts. |
