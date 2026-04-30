# Tool - Tournament

> Last updated: 2026-04-29
> Source: `cmd/mtgsquad-tournament/`, `internal/tournament/`

Parallel tournament runner. Workhorse for the 5K/10K/50K-game tests Josh + 7174n1c run on DARKSTAR. See [[Tournament Runner]] for runtime architecture.

## CLI Flow

```mermaid
flowchart TD
    Args[CLI flags<br/>--decks --games --seed --hat] --> Load[Load decks via deckparser]
    Load --> Mox{Moxfield URL?}
    Mox -- yes --> Fetch[moxfield.FetchDeck]
    Mox -- no --> Files[Read .txt files]
    Fetch --> Decks
    Files --> Decks[TournamentDeck array]
    Decks --> Mode{Pool mode?}
    Mode -- pool --> Pool[Each game samples<br/>NSeats random decks]
    Mode -- round-robin --> RR[Every C(N, seats)<br/>combo plays K games]
    Mode -- fixed --> Fixed[Same seats every game]
    Pool --> Workers[Goroutine pool]
    RR --> Workers
    Fixed --> Workers
    Workers --> Game[Run game<br/>via TakeTurn loop]
    Game --> Capture[Capture outcome<br/>+ event log]
    Capture --> Aggregate[Aggregate by commander]
    Aggregate --> Report[Markdown report<br/>+ matchup matrix]
```

## Run Modes

| Mode | Behavior |
|---|---|
| Fixed | Same N decks, every game |
| Pool | Random N decks sampled from full deck pool per game |
| Lazy pool | Pool mode + load decks on demand (memory ceiling) |
| Round-robin | Every C(N, seats) combination plays K games |

## Hat Selection

`--hat greedy|poker|yggdrasil` — uniform per seat. Yggdrasil is current; greedy/poker retained for parity.

## Audit Mode

`--audit` captures the full event stream for post-game rule auditing. Used to feed [[Tool - Stack Trace]] for CR compliance verification.

## Output

- Per-commander winrate
- Matchup matrix (commander × commander winrate when paired)
- Optional Markdown report (`--report path.md`)
- Per-game event logs (when `--audit`)

## Production Run (50K Games)

```bash
mtgsquad-tournament \
  --lazy-pool \
  --decks data/decks/all \
  --games 50000 \
  --seats 4 \
  --workers 32 \
  --hat yggdrasil \
  --hat-budget 50 \
  --turn-budget 100 \
  --report /tmp/50k.md
```

DARKSTAR v10d binary: 1m34s, 532 g/s, 2 timeouts (0.004%), 654/654 commanders.

## Related

- [[Tournament Runner]]
- [[YggdrasilHat]]
- [[Tool - Heimdall]]
