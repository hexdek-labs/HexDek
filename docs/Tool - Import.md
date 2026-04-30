# Tool - Import

> Last updated: 2026-04-29
> Source: `cmd/mtgsquad-import/`, `internal/moxfield/`

Deck importer for Moxfield and Archidekt URLs. Writes a `.txt` deck in canonical deckparser format.

## Flow

```mermaid
flowchart LR
    URL[Moxfield URL] --> Match[regex extract deck ID]
    Match --> API[Moxfield v3 API<br/>api.moxfield.com/v3/decks]
    API --> JSON[JSON response<br/>name, format,<br/>mainboard, commanders,<br/>sideboard, companions]
    JSON --> Resolve[Resolve oracle text]
    Resolve --> TXT[Write .txt:<br/># Deck Name<br/># Source: URL<br/>COMMANDER: Name<br/>1 Card Name<br/>...]
    TXT --> Disk[data/decks/[folder]/]
```

## Format Output

```
# Deck Name
# Source: https://moxfield.com/decks/XXXXX
COMMANDER: Commander Name
1 Card Name
1 Another Card
...
```

Consumed by [[Decklist to Game Pipeline|deckparser]] in every other tool.

## Usage

```bash
mtgsquad-import --moxfield https://moxfield.com/decks/XXXXX
mtgsquad-import --archidekt https://archidekt.com/decks/12345/name
mtgsquad-import --moxfield URL --output data/decks/josh
```

## Bulk Import

For mass corpus pulls (e.g. 5K-decks coevolution), see [[Moxfield Import Pipeline]].

## Known Limit

Moxfield API returns 403 on search endpoints. Bulk discovery requires either the share-link CSV pathway or scraping. Direct deck-fetch (URL → JSON) works fine.

## Related

- [[Decklist to Game Pipeline]]
- [[Moxfield Import Pipeline]]
- [[Tool - Tournament]]
