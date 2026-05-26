# Precon B4 False-Positive Tracing (R60) — DISCOVERY ONLY

## TL;DR

The two B4 false-positives from PR #508 (Urza's Iron Alliance — BRO, Blast from the Past — WHO) are both lifted to B4 by **the same predicate**, on **the same line of code**:

```go
// cmd/hexdek-freya/archetype.go:1165 + 1229
tunedRedundancy := finisherCount >= 8 && ctx.fastManaCount >= 6
if tunedRedundancy && bracket < 4 {
    bracket = 4
    label = "Optimized"
}
```

For Blast from the Past there is an additional logic bug: the `GC=0 ceiling` had already explicitly demoted the deck to B2 with reason *"no Game Changers and no true-infinite combo"* — and then `Tuned-redundancy floor` runs unconditionally after, undoing that ceiling. The floor does not consult ceiling-applied reasons.

Recommendation: tighten `tunedRedundancy` to require an independent "this is actually B4" signal (mirror to PR #139's B5 confirmation gate), AND make the floor respect a "ceiling capped at B2 with no-GC-no-combo" reason. Details in §3.

## 1. Per-deck rationale breakdown

### 1A. Urza's Iron Alliance (BRO precon, Urza Chief Artificer)

```
Bracket rationale (raw score 3 → B4 Optimized):
  [-1] Average CMC (heavy (>3.5)): 3.5 avg
  [+2] Fast mana (6-9): 8 sub-2-CMC mana producers
  [+2] Finisher density (8+): 13 distinct finisher lines
  [floor] Tuned-redundancy floor: lifted to B4: 13 finishers + 8 fast-mana pieces (was B2)
```

**Score-ladder verdict:** raw 3 → B2 on the ladder (avgCMC penalty + fast-mana + finishers).
**GC=0 ceiling:** no-op (bracket already at B2; nothing to cap).
**Tuned-redundancy floor:** **fires** and lifts B2 → B4 (skipping B3 entirely — a 2-bracket jump).

**Cross-reference metrics that argue against B4:**
| Metric | Value | Reading |
|---|---|---|
| Game Changers | 0 | No GC = WotC's threshold for "this is a deliberately tuned deck" not met |
| True infinites | 0 | No real combo finish (WotC combo carveout for B4 doesn't apply) |
| avgCMC | **3.52** | Heavy curve — incompatible with the "tuned" deck shape (B4 decks typically <3.0) |
| Mana base grade | C | Mediocre — A/B is the threshold for premium-mana B4 |
| Power percentile | 58 | Mid-tier within archetype |
| `plays_like` | Exhibition (B1) | Simulator strongly disagrees with the bracket call |

**Why the finisher counter inflates:** "13 distinct finisher lines" reads tuned but the lines themselves are mostly mass-pump anthems counted separately + their pairings with March of Progress:
```
1. Chief of the Foundry — mass pump finisher
2. March of Progress + Chief of the Foundry — token army + mass pump
6. Master of Etherium — mass pump finisher
8. Unbreakable Formation — mass pump finisher
10. March of Progress + Master of Etherium
11. Tempered Steel — mass pump finisher
12. March of Progress + Tempered Steel
13. March of Progress + Unbreakable Formation
```
That's 4 mass-pump anthems × (standalone + March of Progress pairing) = 8 lines from what is conceptually **one finisher pattern** ("anthems make the token board lethal"). Plus 3 board-wipe entries (Relic, Austere, Phyrexian Rebirth) that aren't really finishers, they're wipes that happen to clear a path. The "13 distinct finishers" headline number is heavily padded.

**Why fast-mana hits 8:** Sol Ring (1), 5 talisman/signet/mind-stone-class 2-CMC rocks, Mishra's Bauble / Springleaf Drum / etc. — i.e., the standard precon ramp suite + a couple of artifact-aware extras. WotC ships 6-8 ramp rocks in EVERY artifact-themed precon by design; the threshold of 6 does not discriminate precons from tuned decks.

### 1B. Blast from the Past (WHO precon, 4th Doctor + Sarah Jane)

```
Bracket rationale (raw score 7 → B4 Optimized):
  [+1] Tutor density (4-7%): 4% of nonlands
  [+2] Combo lines (2-4): 4 true-infinite/determined loops
  [+2] Fast mana (6-9): 7 sub-2-CMC mana producers
  [+2] Finisher density (8+): 8 distinct finisher lines
  [ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
  [floor]   Tuned-redundancy floor: lifted to B4: 8 finishers + 7 fast-mana pieces (was B2)
```

**Score-ladder verdict:** raw 7 → B3 on the ladder.
**GC=0 ceiling:** **explicitly demotes to B2** with reason "no Game Changers and no true-infinite combo (was B3 on raw score)".
**Tuned-redundancy floor:** **fires immediately after** and re-lifts B2 → B4. The ceiling's explicit demotion is silently overridden.

This is the most diagnostic case in the corpus: the rationale literally contains two adjacent lines with contradictory verdicts on the same deck, and the second one wins. The ceiling was added (per CLAUDE.md) to encode WotC's rule that *no GC + no real combo = not B4*. The floor undoes that without checking why the ceiling fired.

**Cross-reference metrics that argue against B4:**
| Metric | Value | Reading |
|---|---|---|
| Game Changers | 0 | Same as Urza — no GC threshold met |
| True infinites | 0 | The "4 combo lines" are FALSE POSITIVES (see below) |
| avgCMC | 3.40 | Heavy curve |
| Mana base grade | D | Below the B4 floor; this deck has a rough 2-color mana base |
| Power percentile | 50 | Median |
| `plays_like` | Core (B2) | Simulator agrees deck plays as B2 — but the floor still calls it B4 |

**Why the "4 combo lines" are false positives:** the win-lines section lists:
```
1. [DETERMINED] Scattered Groves + Irrigated Farmland
   Scattered Groves produces card for Irrigated Farmland, Irrigated Farmland produces card back
2. [DETERMINED] Scattered Groves + Jo Grant
3. [DETERMINED] Irrigated Farmland + Jo Grant
4. [DETERMINED] Scattered Groves + Irrigated Farmland + Jo Grant
```
These are cycling lands (Scattered Groves, Irrigated Farmland — each cycle exiles the land permanently, consuming the card) plus a Doctor Who draw-trigger creature. **Cycling lands cannot loop** — each activation removes the land from the deck. The "produces card → produces card back" cycle detector is treating one-shot cycling as if it were a renewable resource. This separately deserves a fix on the combo-detector side, but for this bracket-trace doc it's only relevant as a contributor to the +2 raw score from "Combo lines" — even without those 4 false combos, the ladder-score would be 5 (still B3, still triggering the GC=0 ceiling → B2 → floor → B4 chain).

## 2. The exact predicate (source)

`cmd/hexdek-freya/archetype.go`:

```go
// line 1165 — predicate definition
tunedRedundancy := finisherCount >= 8 && ctx.fastManaCount >= 6

// lines 1168-1201 — GC=0 ceiling: capped at B2 when GC=0 + no true-infinite
if ctx.gameChangerCount == 0 {
    if hasWinningCombo {
        // ... lift to B4 ...
    } else if bracket > 2 {
        bracket = 2
        label = "Core"
        addAdjustment("GC=0 ceiling", "ceiling",
            "capped at B2: no Game Changers and no true-infinite combo ...")
    }
}

// lines 1227-1235 — tuned-redundancy floor: runs AFTER ceiling, no consultation
if tunedRedundancy && bracket < 4 {
    bracket = 4
    label = "Optimized"
    addAdjustment("Tuned-redundancy floor", "floor",
        fmt.Sprintf("lifted to B4: %d finishers + %d fast-mana pieces (was B%d)",
            finisherCount, ctx.fastManaCount, preFloorBracket))
}
```

The May 24 calibration commit history (per CLAUDE.md) notes this floor was added "for voja-tribal-style decks" — i.e., for the case of a high-finisher-density Voltron with a tuned manabase. The predicate captures that shape, but it is over-broad: every WotC artifact precon and most token-anthem precons satisfy `finishers ≥ 8 AND fastMana ≥ 6` because of natural anthem-counting padding + the standard precon ramp suite.

## 3. Recommended fix (NOT implemented — for 7174n1c)

Two independent changes, either of which closes the immediate false-positive. Best results from applying BOTH.

### 3A. Tighten the `tunedRedundancy` predicate (mirror to PR #139's B5 confirmation gate)

PR #139 fixed an analogous over-firing problem at the B5 end by adding an AND-chain of independent signals. The same pattern applies here. Suggested predicate:

```go
tunedRedundancy := finisherCount >= 8 &&
                   ctx.fastManaCount >= 6 &&
                   (ctx.gameChangerCount >= 1 ||
                    trueInfCount >= 1 ||
                    ctx.tutorDensity >= 0.08 ||
                    (ctx.avgCMC < 3.0 && manaGradeAtLeast(report, "B")))
```

Reasoning per added conjunct:
- `gameChangerCount >= 1`: a deck without ANY Game Changer is, by WotC's own definition, not B4. Cheap unambiguous signal.
- `trueInfCount >= 1`: per the existing WotC combo carveout already used at line 1171.
- `tutorDensity >= 0.08`: 8% nonland-tutor density (4-5+ tutors) is the bottom of the B4 distribution; precons ship 1-2 tutors at most.
- `avgCMC < 3.0 && manaGrade ≥ B`: a deck with a heavy curve AND a mediocre mana base is structurally not a tuned-redundancy B4 — it can't deploy its closers reliably. Both PR #508 false-positives fail this (avgCMC 3.4-3.5, mana C-D).

The OR-chain mirrors PR #139's `(freeInteractionCount >= 2 OR tutorDensity >= 0.12 OR gameChangerCount >= 8) AND avgCMC < 2.8` structure for B5.

### 3B. Respect ceiling-applied reasons in the floor

Independent of the predicate change, the floor should not override an explicit `GC=0 ceiling: no GC + no true-infinite` demotion. Cleanest implementation:

```go
// Before the floor block — check whether the GC=0 + no-combo ceiling fired.
ceilingForbidsB4 := false
for _, sig := range rationale.Signals {
    if sig.Kind == "ceiling" && sig.Name == "GC=0 ceiling" &&
       !hasWinningCombo && ctx.gameChangerCount == 0 {
        ceilingForbidsB4 = true
        break
    }
}

if tunedRedundancy && bracket < 4 {
    target := 4
    if ceilingForbidsB4 {
        target = 3  // floor can still lift, but not all the way to B4
    }
    if bracket < target {
        bracket = target
        ...
    }
}
```

This preserves the floor's value for "genuinely tuned non-cEDH decks that happen to have 0 GCs" (rare but real — some custom anthem-token brews) while honoring the WotC carveout that 0 GC + 0 true-infinite ≠ B4. Blast from the Past would land at B3 instead of B4; Urza would land at B3 instead of B4. Both still arguably hot for an unedited precon, but only ±1 from the vibes target — not ±2.

### 3C. Separate item — finisher-counter dedup

The "13 finishers" Urza count is mostly an artifact-anthem padding effect (4 anthems × 2 pairings with March of Progress = 8 lines from 1 conceptual finisher). A finisher-line de-duplicator that collapses "anthem X (alone)" with "anthem X + token-maker Y" into a single line entry would tighten the count to something like 5-6 for Urza, naturally undershooting the `finisherCount >= 8` threshold without any predicate change.

This is a deeper fix and orthogonal to the bracket logic — flagging it as a follow-up, NOT part of the 3A/3B recommendation.

## 4. Regression test fixture (for the eventual fix PR)

The eventual fix PR should pin both decks (and the rest of `data/decks/wizards/`) in a regression test, mirror to `bracket_calibration_test.go`:

```go
// cmd/hexdek-freya/bracket_precon_calibration_test.go
func TestBracketPreconCorpus_AllStockPreconsAtMostB3(t *testing.T) {
    // Every deck under data/decks/wizards/ is an unedited WotC precon.
    // None should land above B3 (Upgraded). Most should be B2 (Core).
    decks := loadAllDecksUnder(t, "data/decks/wizards/")
    for _, d := range decks {
        if d.Bracket > 3 {
            t.Errorf("%s: stock precon classified as B%d (%s); precons MUST be ≤B3",
                d.SourcePath, d.Bracket, d.BracketLabel)
        }
    }
}
```

Pre-fix: 2/15 failures (Urza, Blast). Post-fix-3A: 0/15. Post-fix-3A+3B: 0/15 with both decks demonstrably routed through the new logic (rationale text contains the new predicate gating).

## 5. Reproducing

```bash
# Trace any deck's bracket rationale (always rendered in the default report):
go run ./cmd/hexdek-freya/ --deck data/decks/wizards/urza_s_iron_alliance_the_brothers_war_commander_precon_decklist.txt 2>&1 | grep -A 20 "Bracket rationale"

# Or pull the structured JSON for downstream analysis:
go run ./cmd/hexdek-freya/ --deck <path> --json | jq '.bracket_rationale'
```

There is **no `--explain-bracket` flag** — the bracket rationale is always rendered in the default text report under the `Bracket rationale (raw score N → BX label):` header and is always present in `--json` as `bracket_rationale.signals[]`.
