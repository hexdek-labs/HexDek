# Moxfield Import Pipeline

> Last updated: 2026-04-29
> Source: `internal/moxfield/`, `cmd/mtgsquad-import/`

Bulk decklist ingestion path for the 5K+-deck pool tournaments. Two-stage: share-link CSV → details NDJSON → engine ingestion.

## Two-Stage Flow

```mermaid
flowchart LR
    Share[Share-link CSV<br/>or scraped index] --> Stage1[Stage 1: deck IDs]
    Stage1 --> API[Moxfield v3 API<br/>per-deck fetch]
    API --> Details[details.ndjson<br/>1 line per deck]
    Details --> Stage2[Stage 2: parse]
    Stage2 --> TXT[.txt files<br/>data/decks/imported/]
    TXT --> Tournament[[[Tournament Runner|--lazy-pool]]]
```

## Why Two Stages

Moxfield API returns 403 on search endpoints — bulk discovery requires either:
- Share-link CSV (manual export from a curated list)
- HTML scraping (more brittle but covers public listings)

Per-deck fetch is reliable; only the discovery side is tricky.

## Stage 1: Deck IDs

Input is a list of deck IDs or full URLs. CSV scraper or share-link export populates this. Output is the canonical ID list.

## Stage 2: Per-Deck Fetch

For each deck ID:
1. `moxfield.FetchDeck(url)` calls `api.moxfield.com/v3/decks/all/{id}`
2. JSON parsed via `apiResponse` struct (name, format, mainboard, commanders, sideboard, companions)
3. Oracle text resolved via Scryfall lookup
4. Written as canonical `.txt` deck — see [[Decklist to Game Pipeline]] format

## Recent Run

Per memory: a recent corpus pull produced 1481 unique decks. Used in steel-man pool tournaments, surfaces commanders/cards never seen in the curated 32-deck portfolio.

## Caching

`details.ndjson` keeps each fetched deck so re-runs don't hit the API again. NDJSON-line format: `{deck_id, name, commander, mainboard:[...], fetched_at}`.

## Rate Limiting

Moxfield API has no published rate limits. Be polite — sleep between requests, cap to single-digit RPS. No production issues observed at conservative pacing.

## Related

- [[Tool - Import]]
- [[Decklist to Game Pipeline]]
- [[Tournament Runner]]
