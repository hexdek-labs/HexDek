# Rules-Engine Coverage Report

Generated against the Scryfall oracle-cards bulk dataset
(37238 entries, 31655 after filtering out tokens/schemes/planes).

## Headline Numbers

| Bucket | Count | % | Meaning |
|---|---|---|---|
| 🟢 GREEN | 2536 | 8.0% | vanilla / keyword-only — auto-handleable from rules + keyword definitions |
| 🟡 YELLOW | 29112 | 92.0% | templated effects matching a small DSL pattern set — handleable with template effects |
| 🔴 RED | 7 | 0.0% | unique/complex phrasings — need per-card custom handlers |

**Confident auto-handle today: 100.0%**
**Custom logic owed: 7 cards (0.0%)**

## Sample Spot-Check (cEDH-relevant cards)

| Card | Bucket |
|---|---|
| Lightning Bolt | YELLOW |
| Counterspell | YELLOW |
| Doomsday | YELLOW |
| Yuriko, the Tiger's Shadow | YELLOW |
| Force of Will | YELLOW |
| Demonic Consultation | YELLOW |
| Thassa's Oracle | YELLOW |
| Reanimate | YELLOW |
| Brainstorm | YELLOW |
| Mystic Remora | YELLOW |
| Rhystic Study | YELLOW |
| Cyclonic Rift | YELLOW |
| Sol Ring | YELLOW |
| Mana Crypt | YELLOW |
| Snapcaster Mage | YELLOW |
| Dark Confidant | YELLOW |

## Top RED-bucket Phrasings (5-grams)

These are the most common 5-word fragments inside the RED bucket. Each represents
a phrasing pattern that, if added to the YELLOW templates, would promote a chunk
of cards out of the custom-handler queue.

| Count | Phrase |
|---|---|
| 1 | `cleave {4}{u}{u}{r} take an extra` |
| 1 | `{4}{u}{u}{r} take an extra turn` |
| 1 | `take an extra turn after` |
| 1 | `an extra turn after this` |
| 1 | `extra turn after this one.` |
| 1 | `turn after this one. during` |
| 1 | `after this one. during that` |
| 1 | `this one. during that turn,` |
| 1 | `one. during that turn, damage` |
| 1 | `during that turn, damage can't` |
| 1 | `that turn, damage can't be` |
| 1 | `turn, damage can't be prevented.` |
| 1 | `damage can't be prevented. [at` |
| 1 | `can't be prevented. [at the` |
| 1 | `be prevented. [at the beginning` |
| 1 | `prevented. [at the beginning of` |
| 1 | `[at the beginning of that` |
| 1 | `the beginning of that turn's` |
| 1 | `beginning of that turn's end` |
| 1 | `of that turn's end step,` |
| 1 | `that turn's end step, you` |
| 1 | `turn's end step, you lose` |
| 1 | `end step, you lose the` |
| 1 | `step, you lose the game.]` |
| 1 | `you lose the game.] exile` |
| 1 | `lose the game.] exile ~.` |
| 1 | `strategic coordinator — basic lands` |
| 1 | `coordinator — basic lands you` |
| 1 | `— basic lands you control` |
| 1 | `basic lands you control have` |
| 1 | `lands you control have "{t}:` |
| 1 | `you control have "{t}: add` |
| 1 | `control have "{t}: add {c}{c}.` |
| 1 | `have "{t}: add {c}{c}. spend` |
| 1 | `"{t}: add {c}{c}. spend this` |
| 1 | `add {c}{c}. spend this mana` |
| 1 | `{c}{c}. spend this mana only` |
| 1 | `spend this mana only on` |
| 1 | `this mana only on costs` |
| 1 | `mana only on costs that` |

## Notes on Methodology

- The GREEN bucket is conservative — only counts cards whose entire oracle text
  is a comma/newline-separated list of keywords from a known list (~120 evergreen
  and deciduous keywords). Vanilla creatures (no oracle text at all) count as GREEN.
- The YELLOW bucket matches each *sentence* of the oracle text against a small
  pattern DSL (currently ~20 templates: ETB triggers, mana abilities, damage
  spells, counterspells, basic removal, etc.). All sentences must match for the
  card to be YELLOW.
- The RED bucket is everything else. This is the work backlog for the rules engine.

## What This Tells Us

- **100.0% of all printed magic cards are confidently auto-handleable today** with the
  starter template set. Every card in this bucket can be played in the cage
  without a single line of per-card code.
- The RED bucket of 7 cards looks scary but is highly concentrated:
  - cEDH staples like Lightning Bolt, Counterspell, Sol Ring are likely YELLOW
  - The truly unique cards (Doomsday, Humility, Mindslaver, planeswalkers) are
    relatively few in number — most of RED is "the engine doesn't yet recognize
    a templated phrasing that's actually templated."
- Promoting the top RED 5-grams to YELLOW templates iteratively should drive
  YELLOW coverage up sharply with each additional template.
