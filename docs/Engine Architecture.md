# Engine Architecture

> Last updated: 2026-04-29
> Source: `sandbox/mtgsquad/internal/gameengine/`
> Language: Go

Top-level dataflow for the [[HexDek]] rules engine: oracle text → AST → game state → turn loop → analytics.

## Overview Diagram

```mermaid
flowchart LR
    SCRY[Scryfall<br/>oracle-cards.json<br/>37,384 cards] --> PARSER[Python parser<br/>scripts/parser.py]
    PARSER --> AST[ast_dataset.jsonl<br/>31,965 ASTs]
    AST --> ASTLOAD[[[Card AST and Parser|astload]]]
    DECK[decklist .txt] --> DECKPARSER[deckparser]
    ASTLOAD --> CARDS[Card pool]
    DECKPARSER --> CARDS
    CARDS --> GS[GameState<br/>state.go]
    HAT[[[Hat AI System]]] -.decisions.-> GS
    GS --> TURN[[[Tournament Runner|TakeTurn]]]
    TURN --> CAST[[[Stack and Priority|CastSpell]]]
    TURN --> COMBAT[[[Combat Phases|CombatPhase]]]
    CAST --> RESOLVE[ResolveStackTop]
    RESOLVE --> ZONE[[[Zone Changes|MoveCard]]]
    ZONE --> REPL[[[Replacement Effects|FireEvent]]]
    REPL --> TRIG[[[Trigger Dispatch|FireZoneChangeTriggers]]]
    TRIG --> SBA[[[State-Based Actions]]]
    SBA --> INV[[[Invariants Odin|Odin invariants]]]
    INV --> EVENTS[Event log]
    EVENTS --> HEIM[[[Tool - Heimdall|Heimdall analytics]]]
```

## Layered Pipeline

- **Layer 0 — Data:** Scryfall bulk dump + parser produce typed AST. See [[Card AST and Parser]].
- **Layer 1 — Static:** [[Layer System]] computes effective characteristics (§613).
- **Layer 2 — Action:** [[Stack and Priority]], [[Combat Phases]], [[Mana System]] mutate state.
- **Layer 3 — Reactive:** [[Replacement Effects]], [[Trigger Dispatch]], [[Zone Changes]] modify or fan out events.
- **Layer 4 — Stabilize:** [[State-Based Actions]] loop until quiescent.
- **Layer 5 — Verify:** [[Invariants Odin]] predicates after every action.
- **Layer 6 — Decide:** [[Hat AI System]] picks targets, attackers, mulligans, responses.
- **Layer 7 — Observe:** Event log feeds [[Tool - Heimdall]] analytics.

## Key Files

- `state.go` — `GameState`, `Seat`, `Permanent`, `Card`, `StackItem` types
- `stack.go` — cast pipeline, priority, resolution, `DrainStack`
- `combat.go` — 5-step combat phase
- `sba.go` — §704 state-based action loop
- `zone_move.go` — universal `MoveCard` entry point
- `zone_change.go` — destroy/exile/sacrifice/bounce + zone-change triggers
- `replacement.go` — `FireEvent` dispatcher (§614/§616)
- `triggers.go` — APNAP trigger ordering
- `layers.go` — §613 continuous-effect layer system
- `mana.go` — typed colored mana pool
- `multiplayer.go` — N-seat / §800 / APNAP helpers
- `invariants.go` — 20 Odin predicates

## Throughput

532 games/sec on DARKSTAR, 32 workers, v10d binary. 50K-game tournament finishes in 1m34s with 2 timeouts (0.004%).

## Related

- [[Engine Overview]] — MOC
- [[Hat AI System]]
- [[Tool Suite]]
