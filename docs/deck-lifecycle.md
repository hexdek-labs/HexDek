# The Life of a Deck — HexDek submission lifecycle

*What sees a deck, when, and where. Code-grounded trace, 2026-06-12. Source line refs are into `internal/hexapi/handler.go`, `showmatch.go`, `cmd/hexdek-freya`, `internal/huginn` unless noted.*

## Flowchart (mermaid)

```mermaid
flowchart TD
    subgraph SUBMIT["1 · SUBMIT — hexdek/src/components/ImportModal.jsx"]
        P[Paste decklist] --> EP1[POST /api/decks/import]
        F[Upload .txt/.dec] --> EP1
        M[Moxfield URL] --> EP2[POST /api/import/moxfield]
    end

    EP1 --> PARSE
    EP2 --> MOX[fetch api2.moxfield.com/v3<br/>normalize to COMMANDER: + N Card]
    MOX --> PARSE

    subgraph PARSE["2 · PARSE + VALIDATE — handler.go:984 / :1111"]
        V[validate cards via /api/cards/search<br/>sanitize owner/name · detect commander]
    end

    PARSE --> STORE
    subgraph STORE["3 · STORE"]
        D1[(data/decks/&#123;owner&#125;/&#123;id&#125;.txt<br/>the decklist — plain text)]
        D2[(data/decks/.versions/ — lineage DAG)]
        D3[(SQLite — custom name, tags, import_log)]
    end

    STORE --> FREYA
    subgraph FREYA["4 · ANALYZE — cmd/hexdek-freya (auto, async go runFreya)"]
        FR[reads data/rules/oracle-cards.json] --> FOUT[writes &#123;id&#125;.strategy.json<br/>+ &#123;id&#125;_freya.md]
        FOUT --> SSE[SSE freya_complete → frontend]
    end

    FREYA --> DISPLAY
    subgraph DISPLAY["5 · DISPLAY — handler.go"]
        L[list /api/decks — enriched from strategy.json]
        DT[detail /api/decks/&#123;o&#125;/&#123;id&#125; — oracle text, mana, types]
        AN[panel /…/analysis — archetype, win lines, roles]
    end

    DISPLAY --> PLAY
    subgraph PLAY["6 · PLAY — showmatch.go (Gauntlet)"]
        G[POST /api/gauntlet → RunGauntlet → N games<br/>each: Yggdrasil hat + strategy profile + Curse DNA]
    end

    PLAY --> LEARN
    subgraph LEARN["7 · RATE + LEARN — post-game"]
        E[ELO: TrueSkill + HexELO → sm.elo]
        C[Curse evolution → hat.SavePool]
        H[Heimdall snapshots → analytics]
        HU[Huginn → data/huginn/raw_observations.json<br/>→ tier3_for_freya.json]
    end

    LEARN --> RESULTS
    subgraph RESULTS["8 · RESULTS DISPLAY"]
        R1[/…/elo-history]
        R2[/…/matchups]
        R3[/…/card-stats]
    end

    HU -.->|learning loop — NOT closed today| FREYA

    classDef gap stroke-dasharray: 5 5,stroke:#e74c3c;
    class HU gap
```

## Known broken / external arrows (honest)

- **⚠ Huginn → Freya loop is NOT closed.** `data/huginn/tier3_for_freya.json` is written (`internal/huginn/huginn.go:48-49`) but Freya never re-reads it on the analyze path — `huginn.Ingest()` is called from nowhere. The dotted red arrow above is aspirational. (This is exactly cleanup-round-2's "close the Huginn loop.")
- **⚠ Backup is external infra, not in this flow.** The `deck-backups` git branch, DARKSTAR `E:` daily mirror, and MISTY 6h cron run outside the Go code (scheduled tasks / cron), so they don't appear as code hops.
- **⚠ Freya auto-trigger** fires on import (`go h.runFreya` at handler.go:1084/1233) and lazily on first `/analysis` request, but some UI paths still surface "analyzing…" until a detail-page visit nudges it.

## Stage-by-stage (file:line)

| # | Stage | Actor / file:line | Reads → Writes |
|---|-------|-------------------|----------------|
| 1 | Submit | ImportModal.jsx:190-437 | user input → POST /api/decks/import or /api/import/moxfield |
| 2 | Parse/validate | handler.go:984 (paste/file), :1111 (moxfield) | Moxfield fetch api2.moxfield.com:1135 → normalize :1179-1193 |
| 3 | Store | handler.go:1037/1227 · :2059-2072 (versioning) · :1065-1074 (DB) | → `{owner}/{id}.txt`, `.versions/` DAG, SQLite import_log |
| 4 | Analyze (Freya) | runFreya handler.go:823 → cmd/hexdek-freya/main.go:93 | reads oracle-cards.json → `{id}.strategy.json` + `_freya.md` → SSE |
| 5 | Display | handleListDecks:202 · handleGetDeck:316 · handleGetAnalysis:775 | reads strategy.json + decklist + oracle metadata |
| 6 | Play | handleStartGauntlet showmatch.go:2739 → RunGauntlet:721 | strategy profile + Curse DNA + hat → game sim |
| 7 | Rate/learn | updateELO:915 · Curse:894-907 · Huginn huginn.go:112-146 | → sm.elo, SavePool, raw_observations.json, tier3_for_freya.json |
| 8 | Results | handleDeckEloHistory:137 · handleDeckMatchups:135 · card-stats:153-174 | reads gauntlet history → elo-history, matchups, card-stats |
