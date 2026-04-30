# Tool - Heimdall

> Last updated: 2026-04-29
> Source: `cmd/mtgsquad-heimdall/`, `internal/analytics/`

Two modes: **spectator** (live single-game stream with pause-on-anomaly) and **analytics** (post-tournament deep report).

## Two-Mode Flow

```mermaid
flowchart TD
    Heim[Heimdall] --> Mode{Mode}
    Mode -- default<br/>spectator --> S1[Run single game]
    S1 --> Stream[Print every event<br/>with full context]
    Stream --> Inv{Anomaly<br/>or invariant<br/>violation?}
    Inv -- yes --> Pause[PAUSE — print<br/>full game state,<br/>wait for input]
    Inv -- no --> Stream
    Mode -- --analyze --> A1[Run N games]
    A1 --> Events[Collect all event logs]
    Events --> Analyze[AnalyzeGame<br/>per game]
    Analyze --> Per[Per-card performance<br/>per-player stats<br/>per-game timeline]
    Per --> Combos[Missed-combo<br/>detection]
    Per --> Kills[Kill-shot attribution]
    Combos --> Report[Markdown report]
    Kills --> Report
```

## Tracks

- Per-card performance (times played, kills attributed, triggers fired)
- Win-condition detection (lethal-threshold kill-shot)
- Mana efficiency (wasted mana from pool drains)
- Dead-card analysis (cards stuck in hand all game)
- Matchup matrices (commander vs commander winrates)
- **Missed combo detection** — flags when a known combo was live but not executed

## Known Combos Tracked (10)

`internal/analytics/combos.go::KnownCombos`:

1. Thoracle + Consultation/Pact
2. Ragost Strongbull Loop
3. Sanguine Bond + Exquisite Blood
4. Basalt Monolith + Kinnan
5. Isochron Scepter + Dramatic Reversal
6. Walking Ballista + Heliod
7. Aetherflux Reservoir Storm
8. Dockside + Temur Sabertooth
9. Food Chain + Eternal Creature
10. Phyrexian Altar + Gravecrawler

## Usage

```bash
# Spectator
go run ./cmd/mtgsquad-heimdall \
  --decks data/decks/cage_match \
  --seed 42 \
  --pause-on-anomaly

# Analytics
go run ./cmd/mtgsquad-heimdall \
  --analyze --games 50 --decks data/decks/cage_match \
  --hat poker --report data/rules/HEIMDALL_ANALYSIS.md
```

## Discord Showmatch Hook

[[Tournament Runner|TakeTurnWithHook]] gives Heimdall a per-phase snapshot callback. Used by the showmatch spectator loop for paced Discord narration.

## Related

- [[Tool - Tournament]]
- [[Tool - Freya]]
- [[Engine Architecture]]
