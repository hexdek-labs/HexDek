# Decklist to Game Pipeline

> Last updated: 2026-04-29
> Source: `internal/deckparser/`, `internal/astload/`, `internal/shuffle/`

How a `.txt` deck file becomes a playable [[Engine Architecture|GameState]].

## Pipeline

```mermaid
flowchart TD
    TXT[deck.txt<br/>COMMANDER: X<br/>1 Card Name] --> DP[deckparser.Parse]
    DP --> TD[TournamentDeck<br/>commander + 99-card list]
    AST[ast_dataset.jsonl] --> Loader[astload.LoadCorpus]
    Loader --> Pool[Card pool<br/>map name → CardAST]
    TD --> Resolve[Resolve names<br/>against pool]
    Pool --> Resolve
    Resolve --> Cards[*Card array]
    Cards --> Shuffle[shuffle.Shuffle<br/>Fisher-Yates +<br/>crypto/rand]
    Shuffle --> Lib[seat.Library]
    Cards --> Cmd[seat.CommandZone]
    Lib --> Draw[Draw 7 → seat.Hand]
    Draw --> GS[GameState ready]
    GS --> Mull[Hat ChooseMulligan]
    Mull -- mulligan --> Shuffle
    Mull -- keep --> Play[Game starts]
```

## deckparser Format

```
# Deck Name
# Source: https://moxfield.com/decks/XXXXX
COMMANDER: Commander Name
1 Card Name
1 Other Card
...
```

Lines starting with `#` are comments. `COMMANDER:` is special. Quantity-prefixed cards are mainboard. Single-line per card; quantity replication handled in parser.

## AST Pool

`astload.LoadCorpus("data/rules/ast_dataset.jsonl")` returns `map[string]*gameast.CardAST`. Resolution by exact card name. Missing cards logged but the deck is still playable with stub cards.

## Shuffle (§103.1)

`internal/shuffle/fisher_yates.go` — Fisher-Yates with `crypto/rand` entropy. Returns error only if entropy source fails. For multi-party trustless shuffle, commit-reveal happens at the game layer (see [[Tool - Server]]).

## Mulligan (§103.4)

Engine offers `ChooseMulligan` to [[Hat AI System|Hat]]. Greedy: mulligan if 0-1 lands. Yggdrasil: archetype-tuned thresholds. London mulligan rules (draw 7, scry to bottom on each restart).

## Commander Setup

Commander goes to command zone, NOT library. Cast via `ShouldCastCommander` Hat decision with [[Mana System|tax]] applied per §903.8.

## Related

- [[Tool - Import]]
- [[Tournament Runner]]
- [[Engine Architecture]]
