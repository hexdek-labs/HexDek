# Seat-Bias Expanded Study — Scope Proposal

**Date:** 2026-05-25
**Branch:** `dev/seat-bias-expanded-study-r60`
**Status:** **SCOPE PROPOSAL — awaiting green-light before execution**
**Predecessor:** PR #322 (`docs/seat-bias-meta-study-r60.md`) — the
37,500-game meta-study that found QUARK behavior in the 2 archetypes
that had cross-composition data, and UNDETERMINED for the other 16.

## What this study would answer

The meta-study's headline limitation: only 2 of 18 archetypes had
multiple compositions, so only 2 could be classified ELECTRON vs
QUARK. Both classified QUARK, but we don't know if that's a property
of THOSE specific archetypes/decks (Reanimator-via-Meren-or-Karador,
LandsMatter-via-Windgrace-or-Aesi) or a systemic property of the
hat across all archetypes.

The expanded study is designed to answer **two distinct questions** the
meta-study could not:

1. **Archetype-vs-deck separation.** Is the within-archetype seat
   preference consistent across DIFFERENT decks of that archetype?
   (e.g., does Phenax-Mill prefer seat 0 because it's Mill, or
   because it's Phenax specifically?) — answered by running 3-4
   different decks of each archetype.
2. **ELECTRON-vs-QUARK for non-singleton archetypes.** With each
   archetype's decks appearing in multiple compositions, can we
   classify each one as ELECTRON or QUARK directly (not by
   analogy from Reanimator/LandsMatter)?

Without both: we can't tell whether to use a per-deck, per-archetype,
or per-composition correction in any future TrueSkill prior.

## Deck inventory (data/decks/moxfield/)

Per-archetype counts of decks available for the study:

| Archetype | Available decks | Sufficient for 3-deck study? |
|---|---:|---|
| Mill | 3 (Phenax, Bruvac, Anowon variants) | ✅ exactly enough |
| Voltron | 3 (Wyleth, Uril, Skullbriar) | ✅ exactly enough |
| Spellslinger / Storm | 23 (Kalamax, Mizzix, Krark, Jhoira, Niv-Mizzet variants) | ✅ ample |
| LandsMatter | 12 (Windgrace, Aesi, Omnath, Titania, Yarok, Gitrog) | ✅ ample |
| Aristocrats | 11 (Korvold, Teysa, Mazirek, Judith, Meren) | ✅ ample |
| Reanimator | 13 (Karador, Meren, Muldrotha, Sefris, Chainer, Hogaak) | ✅ ample |
| Tribal | 18 (Edgar Markov, Krenko, Slimefoot, Lathril, Atla Palani) | ✅ ample |
| Lifegain | 5 (Heliod, Karlov, Oloro, Trelasarra) | ✅ enough |
| Selfmill | 6 (Sidisi, Varolz, Thrasios, Hermit Druid) | ✅ enough |
| Artifacts | 13 (Urza, Breya, Osgir, Daretti, Jhoira-Weatherlight) | ✅ ample |
| Blink | 6 (Aminatou, Brago, Bilbo) | ✅ enough |
| Stax | 5 (Drannith, Talion, Tergrid) | ✅ enough |
| ExtraCombats | 3 (Najeela, Purphoros, Kediss) | ✅ exactly enough |
| GroupHug | 4 (Zedruu, Kynaios, Phelddagrif, Selvala-Explorer) | ✅ enough |
| CountersMatter | 4 (Ezuri, Pir, Marwyn, Hamza) | ✅ enough |
| Enchantress | 1 (Sythis only) | ❌ **insufficient** — can't separate deck vs arch |
| Superfriends | 1 (Atraxa-Praetors only) | ❌ **insufficient** |
| Aurelia/Moraug aggro | 1 | ❌ **insufficient** |
| Aggro (pure) | ? (no clean inventory) | ❓ unknown |
| Control (pure) | ? (no clean inventory) | ❓ unknown |
| Ramp (pure) | ? | ❓ unknown |
| Combo (generic) | many in Kenrith/Thrasios pool, hard to isolate | ❓ ambiguous |

**Coverable: 15 archetypes** with ≥3 decks each. **Not coverable: 7**
without sourcing more decks (would need Moxfield imports).

## Study designs (three options)

### Option A — Minimal (focused on the 6 high-priority archetypes)

The 6 archetypes that showed the most interesting seat patterns in the
meta-study, with 3 decks each, each appearing in 2 compositions.

| Parameter | Value |
|---|---|
| Archetypes | 6 (Mill, Voltron, Spellslinger, LandsMatter, Aristocrats, Reanimator) |
| Decks per archetype | 3 |
| Compositions per (archetype × deck) | 2 |
| Compositions total | 6 arch × 3 decks × 2 comps / 4 seats ≈ 9 compositions |
| Seeds per composition | 3 (vs meta-study's 5) |
| Games per (composition × seed) | 1500 |
| **Total games** | **9 × 3 × 1500 = 40,500** |
| **Wall time estimate** | **~50 min** (matches meta-study density) |

This barely expands on the meta-study but adds the deck-vs-archetype
separation for 6 archetypes. **Acceptable risk: this is the cheapest
defensible upgrade.**

### Option B — Recommended (15 coverable archetypes, 3 decks each)

| Parameter | Value |
|---|---|
| Archetypes | 15 (all archetypes with ≥3 deck inventory) |
| Decks per archetype | 3 |
| Compositions per (archetype × deck) | 2 |
| Compositions total | 15 arch × 3 decks × 2 comps / 4 seats ≈ 23 compositions |
| Seeds per composition | 3 |
| Games per (composition × seed) | 1500 |
| **Total games** | **23 × 3 × 1500 ≈ 103,500** |
| **Wall time estimate** | **~2h 5m** (3× the meta-study's wall) |

This is the user-mentioned "100K+" target. Covers 15 of 22
archetypes with deck-level resolution. Each (archetype × deck) cell
gets 3 comps × 3 seeds × 1500g = 13,500 games — stderr ~0.4pp per
cell, well below the 0.5pp target.

### Option C — Full (15 archetypes × 4 decks where available)

| Parameter | Value |
|---|---|
| Archetypes | 15 |
| Decks per archetype | 4 where inventory allows (else 3) |
| Compositions per (archetype × deck) | 3 |
| Compositions total | ~45 |
| Seeds per composition | 5 |
| Games per (composition × seed) | 1500 |
| **Total games** | **~337,500** |
| **Wall time estimate** | **~6 h 30 min** |

Diminishing returns vs Option B. Mostly worth it if we want
ELECTRON-vs-QUARK certainty per (archetype, deck) rather than just
per archetype.

## Recommended scope: Option B

- 103,500 games, ~2h wall (5× the meta-study cost for ~8× the
  archetype coverage and full deck-vs-archetype separation).
- Within Option B's budget the per-cell stderr (~0.4pp) clears the
  ≤0.5pp target the meta-study set.
- Doesn't require sourcing new decks — uses what's in
  `data/decks/moxfield/` today.
- The 7 archetypes excluded (Enchantress, Superfriends, etc., with
  only 1 deck each) get noted as "deck-inventory-blocked" — a
  separate cycle could import more decks for them.

## What we'd learn

For each archetype, three answers:
1. **Per-deck seat winrate** — is Phenax's seat-0 advantage (PR #258's
   70.2%) something Bruvac and Anowon share, or is it a Phenax-only
   quirk?
2. **Per-(archetype, seat) verdict** — ELECTRON if the same seat
   pattern holds across 3 decks of the same archetype AND 2
   compositions; QUARK if not.
3. **Per-archetype across-deck variance** — directly measures
   "deck-level effects" the meta-study couldn't separate from
   "composition-level effects."

## Risks + caveats

1. **15 archetypes × 3 decks × 2 comps = 90 deck-comp cells.** That's
   complex study bookkeeping — needs a deck-selection script + an
   aggregation script that handles the 3-deck-per-archetype shape
   (PR #322's aggregator assumes 1-deck-per-archetype, would need
   extension).
2. **~2h wall is long enough to wedge on overnight machine sleep.**
   Recommend running on a dedicated machine, or chunking into 4-5
   composition batches with checkpointed output.
3. **Some archetypes (Mill, Voltron, ExtraCombats) have exactly 3
   decks** — no spare for deck-variance backup. If one deck file is
   broken (parse error, missing oracle data) the study has to fall
   back to 2 decks for that archetype.
4. **Self-play limitation persists.** All 4 seats use identical
   YggdrasilHat. Live human + bot mixed play might show different
   per-deck seat patterns. The expanded study would still be
   silent on that.
5. **Per-(archetype, deck) cell stderr ≈ 0.4pp at Option B sizing**
   gives 95%-CI ±0.8pp per cell. ELECTRON-vs-QUARK classification
   at 1.5pp / 2pp thresholds needs the variance signal to clear
   ~3× the noise floor, which it should.

## Green-light criteria

Please confirm before execution:

1. **Scope choice.** Option A (focused, 40K games, ~50min) / Option
   B (recommended, 100K games, ~2h) / Option C (full, 340K games,
   ~6.5h) — or a custom variant.
2. **Compute window.** Whether to launch immediately or schedule
   for a known-idle window.
3. **Aggregator extension.** Option B requires extending the
   meta-study's aggregator to handle multi-deck-per-archetype data
   layout (mostly straightforward — group by `(archetype, deck,
   seat)` instead of `(commander, seat)` and add an across-decks
   variance computation per archetype).
4. **Reporting cadence.** Per-composition progress (matches the
   meta-study's pattern) vs. per-archetype-completion summary.

Once the user picks a scope variant, the execution PR can branch off
this one and run.

## What this PR does NOT do

- No tournament runs executed.
- No code changes (aggregator extension is part of the execution PR).
- Just the design + cost/scope tradeoff doc.
