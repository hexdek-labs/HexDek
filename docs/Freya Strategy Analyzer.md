# Freya Strategy Analyzer

> Last updated: 2026-04-29
> Source: `cmd/mtgsquad-freya/`
> Output: `<deck>.strategy.json` + `<deck>_freya.md`

Static deck analyzer. Reads oracle text, classifies effects, builds resource graph, finds combos and synergies. Output drives [[YggdrasilHat]] strategy via [[Eval Weights and Archetypes|StrategyProfile]].

## Pipeline

```mermaid
flowchart TD
    Deck[decklist .txt] --> Oracle[Resolve oracle text<br/>from Scryfall dump]
    Oracle --> Classify[Classify each card:<br/>PRODUCES /<br/>CONSUMES /<br/>TRIGGERS]
    Classify --> Graph[Build resource graph<br/>edges = produce → consume]
    Graph --> Cycles[Find cycles<br/>= combo loops]
    Graph --> P1[Phase 1: statistics<br/>mana curve, color sources]
    Graph --> P2[Phase 2: roles<br/>12 role tags per card]
    Graph --> P3[Phase 3: archetype<br/>10-archetype Euclidean match]
    Graph --> P4[Phase 4: win lines<br/>tutor chains, redundancy]
    Graph --> P5[Phase 5: profile<br/>unified DeckProfile struct]
    P1 --> Out[Output report]
    P2 --> Out
    P3 --> Out
    P4 --> Out
    P5 --> Out
    Out --> JSON[strategy.json]
    Out --> MD[_freya.md]
    JSON --> Hat[[[YggdrasilHat]] loads<br/>via LoadStrategyFromFreya]
```

## Output Categories

- 🔴 **True Infinites** — loop never terminates, needs outlet
- 🟢 **Determined Loops** — finite, terminates at opponent life total
- 🟡 **Game Finishers** — win buttons, mass pump, X-burn, infect
- 🔵 **Synergies** — strong 2-card interactions, not infinite

## Phases (5)

| Phase | File | Output |
|---|---|---|
| 1 | `statistics.go` | mana curve, pip demand by CMC bracket, color sources/gaps, Frank Karsten land eval, ramp/draw classification |
| 2 | `roles.go` | 12 tags: Ramp, Draw, Removal, BoardWipe, Counterspell, Tutor, Threat, Combo, Protection, Stax, Utility, Land. Multi-role + balance warnings |
| 3 | `archetype.go` | 10 archetypes: Combo, Stax, Control, Voltron, Aristocrats, Spellslinger, Tribal, Reanimator, Aggro, Midrange. Hybrid detection + bracket estimation (1-5) |
| 4 | `winlines.go` | Win lines with 80+ known tutors, type restrictions, tutor chains, backup plans, single points of failure |
| 5 | `deckprofile.go` | Unified `DeckProfile`: identity + stats + roles + archetype + win lines + auto-derived strengths/weaknesses + gameplan summary |

## ComputeEvalWeights

Derives deck-specific [[Eval Weights and Archetypes|EvalWeights]] from analysis. Combo decks get higher `ComboProximity`, control decks higher `CardAdvantage`, etc. Beats archetype defaults when wired.

## Known False-Positive Pattern

Audit 2026 found ~20/28 detected loops in 7174n1c's decks were false positives. Causes:
- Self-exile not modeled — "exile this" loop pieces never come back
- Hand vs battlefield confusion — cards needed in hand counted as on-battlefield
- Attack-trigger dependency missed — combos that need an attack to fire treated as priority-speed
- Randomness — Cascade/coin-flip loops treated as deterministic

Tracked separately; fixes ongoing.

## Usage

```bash
go run ./cmd/mtgsquad-freya --deck data/decks/benched/ragost.txt
go run ./cmd/mtgsquad-freya --all-decks data/decks/lyon/ --format markdown
go run ./cmd/mtgsquad-freya --deck my_deck.txt --format json > out.json
```

## Related

- [[Tool - Freya]]
- [[Hat AI System]]
- [[YggdrasilHat]]
- [[Eval Weights and Archetypes]]
