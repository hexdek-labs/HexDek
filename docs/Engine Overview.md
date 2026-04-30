# HexDek — MTG Rules Engine

> Last updated: 2026-04-29
> Location: `sandbox/mtgsquad/`
> Language: Go

Judge-grade Magic: The Gathering rules engine for multiplayer Commander. Pure rules engine, not a client. Built because no existing simulator gets EDH right at the scale needed for meaningful data — XMage has 1000+ rules-bug issues, Cockatrice doesn't enforce rules, MTGO/Arena have no API and no Commander. HexDek targets correct multiplayer Commander simulation at thousands of games per second so the resulting data feeds card evaluation, combo detection, and strategy work.

## High-Level Flow

```mermaid
flowchart LR
    Deck[Decklist] --> Pipe[[[Decklist to Game Pipeline]]]
    Pipe --> Engine[[[Engine Architecture|Game Engine]]]
    AI[[[Hat AI System|Hat AI]]] -.decisions.-> Engine
    Engine --> Events[Event log]
    Events --> Tools[[[Tool Suite|Norse Tool Suite]]]
    Tools --> Reports[Reports / analytics]
    Reports -.feed back.-> AI
```

## Engine Internals

- [[Engine Architecture]] — top-level dataflow + key files
- [[Stack and Priority]]
- [[Combat Phases]]
- [[State-Based Actions]]
- [[Mana System]]
- [[Replacement Effects]]
- [[Trigger Dispatch]]
- [[Zone Changes]]
- [[Layer System]]
- [[Card AST and Parser]]
- [[Per-Card Handlers]]
- [[Invariants Odin]]
- [[APNAP]]

## AI / Decision-Making

- [[Hat AI System]] — pluggable player-decision protocol
- [[YggdrasilHat]] — current unified brain
- [[MCTS and Yggdrasil]] — budget dial + rollout architecture
- [[Eval Weights and Archetypes]]
- [[Greedy Hat]] (deprecated, retained for parity)
- [[Poker Hat]] (deprecated)
- [[Freya Strategy Analyzer]]

## Pipelines

- [[Decklist to Game Pipeline]]
- [[Tournament Runner]]
- [[Moxfield Import Pipeline]]

## Tools

- [[Tool Suite]] — Norse pantheon MOC (Thor, Odin, Loki, Heimdall, Freya, Valkyrie, Judge, Tournament, Server, Import, Parity, Stack Trace)

## Verification Status

```
Thor:   793,826 tests across 36,083 cards — ZERO failures
Loki:   10,000 games + 50,000 nightmare boards — ZERO violations
Odin:   20 invariants checked after every game action
CR Audit: 15/15 identified issues FIXED
Go Tests: 14/14 packages passing
```

50K-game tournament on DARKSTAR (v10d, 2026-04-28): 532 g/s, 2 timeouts (0.004%), 654/654 unique commanders covered.

## Known Gaps

- **§613.8 layer dependency ordering** — uses timestamp, not true dependency graph (affects Humility + Opalescence, Blood Moon + Urborg)
- **Mutate (§702.140)** — not implemented
- **Companion (§702.139)** — not implemented
- **Meld (§701.42)** — stub only
- **Phyrexian mana** — no life-payment alternative
- **Hybrid mana** — color choice not fully resolved
- **Combo win conditions** — engine doesn't recognize assembled combos as wins; combo decks beatdown only (90%+ wins are combat damage). Primary remaining ceiling on combo archetype.
- **Hat gaps** — combo recognition, decklist awareness, mulligan intelligence, alliance/betrayal politics

## Decks

Active deck corpus: ~1500 unique decks via [[Moxfield Import Pipeline]]; 32 hand-curated test decks in `data/decks/`.

## Related

- [[HexDek TODO Board]]
- [[Tool Suite]]
