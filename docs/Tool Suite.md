# HexDek Tool Suite — Norse Pantheon

> Last updated: 2026-04-29
> Location: `sandbox/mtgsquad/`

Norse-named tool suite around the [[Engine Architecture|HexDek engine]]. Each tool is a single-binary entry point under `cmd/`; shared engine logic lives in `internal/`. Tools split testing, simulation, analysis, and serving into separate processes so each can be parallelized independently and run overnight on DARKSTAR.

## Tools

| Tool | Purpose | Binary |
|---|---|---|
| [[Tool - Thor\|Thor]] | Per-card stress tester (oracle-text-aware effect verification) | `cmd/mtgsquad-thor/` |
| [[Tool - Odin\|Odin]] | 20-invariant property fuzzer | `cmd/mtgsquad-odin/` |
| [[Tool - Loki\|Loki]] | Random-deck chaos gauntlet + nightmare boards | `cmd/mtgsquad-loki/` |
| [[Tool - Heimdall\|Heimdall]] | Spectator + post-game analytics, missed-combo detection | `cmd/mtgsquad-heimdall/` |
| [[Tool - Freya\|Freya]] | Static deck analyzer, archetype + win lines, drives StrategyProfile | `cmd/mtgsquad-freya/` |
| [[Tool - Valkyrie\|Valkyrie]] | Deck regression runner over `data/decks/` | `cmd/mtgsquad-valkyrie/` |
| [[Tool - Judge\|Judge]] | Interactive REPL for adversarial rules testing | `cmd/mtgsquad-judge/` |
| [[Tool - Tournament\|Tournament]] | Parallel tournament runner (workhorse) | `cmd/mtgsquad-tournament/` |
| [[Tool - Server\|Server]] | WebSocket game server for `hexdek.bluefroganalytics.com` | `cmd/mtgsquad-server/` |
| [[Tool - Import\|Import]] | Single Moxfield/Archidekt URL → `.txt` deck | `cmd/mtgsquad-import/` |
| [[Tool - Parity\|Parity]] | Go ↔ Python engine parity verifier | `cmd/mtgsquad-parity/` |
| [[Tool - Stack Trace\|Stack Trace]] | CR-compliance audit logger (in-engine, not a binary) | `internal/gameengine/stack_trace.go` |

## How They Fit Together

```mermaid
flowchart TD
    Scry[Scryfall corpus] --> Thor
    Scry --> Loki
    Import --> Decks[(data/decks)]
    Decks --> Freya
    Decks --> Valkyrie
    Decks --> Tournament
    Decks --> Heimdall
    Freya --> Strategy[strategy.json]
    Strategy --> Tournament
    Tournament --> Outcomes[Outcomes + event logs]
    Outcomes --> Heimdall
    Loki --> Invariants[[[Invariants Odin|20 invariants]]]
    Odin --> Invariants
    Tournament -.--audit.-> StackTrace[Stack Trace]
    Judge --> Engine[Engine REPL]
    Server --> Engine
    Parity --> Engine
    Parity --> Python[Python reference]
```

- [[Tool - Thor|Thor]] verifies cards before they enter chaos play
- [[Tool - Loki|Loki]] + [[Tool - Odin|Odin]] both lean on [[Invariants Odin|the same 20 invariants]] — Loki for random-deck variety, Odin for overnight fuzz with violation aggregation
- [[Tool - Freya|Freya]] writes `strategy.json` consumed by [[YggdrasilHat]] inside [[Tool - Tournament|Tournament]]
- [[Tool - Heimdall|Heimdall]] reads tournament event logs for analytics + missed-combo detection
- [[Tool - Valkyrie|Valkyrie]] is the regression smoke test against the curated portfolio
- [[Tool - Judge|Judge]] reproduces bugs Thor/Loki/Valkyrie surface
- [[Tool - Server|Server]] hosts the live web frontend
- [[Tool - Import|Import]] feeds `data/decks/`; [[Moxfield Import Pipeline]] handles bulk corpus pulls
- [[Tool - Parity|Parity]] keeps the Go engine pinned to the Python reference

## Verification Status

```
Thor:   793,826 tests across 36,083 cards — ZERO failures
Loki:   10,000 games + 50,000 nightmare boards — ZERO violations
Odin:   20 invariants checked after every game action
CR Audit: 15/15 identified issues FIXED
```

## Related

- [[Engine Overview]]
- [[Engine Architecture]]
- [[Tournament Runner]]
- [[Hat AI System]]
