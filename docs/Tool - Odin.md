# Tool - Odin

> Last updated: 2026-04-29
> Source: `cmd/mtgsquad-odin/` (binary), `internal/gameengine/invariants.go` (predicates)

Property-based fuzzer. Wraps every GameState mutation with [[Invariants Odin|the 20 invariants]]. Designed to run overnight on DARKSTAR.

## Fuzz Loop

```mermaid
flowchart LR
    Start[Odin start<br/>--games N --seed X] --> Setup[Setup random pod<br/>from --decks dir]
    Setup --> Game[Game runs]
    Game --> Mut[Every mutation<br/>cast / resolve / SBA /<br/>combat / trigger]
    Mut --> Check[Run all 20 invariants]
    Check -- pass --> Mut
    Check -- violation --> Capture[Capture full state +<br/>event history +<br/>invariant name + game ID]
    Capture --> Log[Append to violations list]
    Log --> Game
    Game -- end --> Next[Next game]
    Next --> Setup
    Setup -- N done --> Report[Markdown report<br/>sorted by invariant]
```

## Why It's a Separate Binary

[[Tool - Loki|Loki]] runs invariants too, but Odin's specialty is overnight fuzz runs — its violation aggregator collects per-game and writes a clean markdown report at the end, designed for the next-morning triage workflow.

## Usage

```bash
go run ./cmd/mtgsquad-odin \
  --games 10000 \
  --seed 42 \
  --decks data/decks/cage_match/ \
  --report data/rules/FUZZ_REPORT.md
```

## Predicate Source

The 20 predicates live in `internal/gameengine/invariants.go` and are documented in [[Invariants Odin]]. Both Odin and [[Tool - Loki|Loki]] import the same predicate set — single source of truth.

## Related

- [[Invariants Odin]]
- [[Tool - Loki]]
- [[Tool - Thor]]
