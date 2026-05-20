# Stub Hunt — parser/loader/persistence layer (R47)

**Date:** 2026-05-20
**Branch:** `dev/stub-hunt-rules-db-r47`
**Scope:** `internal/oracle/`, `internal/astload/`, `internal/db/`.

**Note:** the original task listed `internal/rules/` twice — that directory
does not exist in the Go side of the tree. The Comprehensive Rules text and
oracle-cards dump live as data files under `data/rules/`; the *parsers* for
those data files are `internal/astload/loader.go` (AST JSONL → CardAST) and
`internal/oracle/scryfall.go` (Scryfall API → SQLite cache), both already in
scope. Flagged here so future hunts don't repeat the search.

## Method

Three passes:
1. Grep for `TODO|FIXME|MVP|stub|placeholder|unimplemented|no-op` — yielded
   1 hit (`scryfall.go:69`, a benign no-op comment).
2. Grep for silent-failure patterns: `_ = err`, `_, _ := res.X`, unchecked
   `rows.Scan` / `rows.Err()`, unchecked `tx.Exec`, `defer .Close()` without
   error context, direct `err == sql.ErrNoRows`.
3. Read each handler with multiple SQL statements as a whole, looking for
   non-atomic two-step writes (party + member, game + seats).

## Findings

### High — real bugs

| # | Location | Issue | Why it matters |
|---|---|---|---|
| H1 | `internal/oracle/scryfall.go:88-90` | `Lookup` swallows `saveToCache` errors with `_ = err`. | A persistent cache write failure (disk full, schema mismatch, locked db) silently disables the cache, causing Scryfall API hammering on every subsequent lookup. Scryfall's ~10 req/s ceiling becomes a real risk on a deck-import burst. |
| H2 | `internal/db/sqlite.go:71-82` | Schema migration's `DROP TABLE IF EXISTS` + `CREATE TABLE` results are unchecked (no `if err != nil`). | If DROP fails (e.g., FK constraint, locked db), CREATE may target a still-existing old-schema table or the CREATE itself fails; `applyMigrations` returns nil and the rest of the server starts on a corrupt schema. |
| H3 | `internal/db/showmatch.go:156` | `PersistGameTx`: `gameID, _ := res.LastInsertId()` ignores the error. | If LastInsertId fails (e.g., driver bug, transaction state), `gameID = 0` propagates into the subsequent seat inserts and all of that game's seat rows become orphans pointing at a non-existent game_id. |
| H4 | `internal/db/showmatch.go:377-389` | `LoadOwnerGames` opponents sub-query: on `QueryContext` error it `continue`s (silently dropping opponents); on `oppRows.Scan` error it appends empty strings; `oppRows.Err()` is never checked. | UI dashboards quietly show truncated or blank opponent lists when the per-game subquery hiccups, with no log trail. |
| H5 | `internal/db/party.go:50-57` | `CreateParty` does two writes (party row + auto-host member row) without a transaction. | If the host-member insert fails, the party row stays around as an "empty lobby" that no one is in — the host has to abandon and create a fresh party with a new code. |

### Medium

| # | Location | Issue |
|---|---|---|
| M1 | `internal/db/party.go:59-60` | Retry loop comment says "On ID collision try a new code" but the code retries on **any** error. A non-collision failure (DB unreachable, disk full) spins 5× for no reason. Should match the SQLite unique-constraint code (`SQLITE_CONSTRAINT_PRIMARYKEY`) or at least the error text. |
| M2 | `internal/db/{anticheat.go:91,361; showmatch.go:240,279}` | Direct `err == sql.ErrNoRows` comparison rather than `errors.Is(err, sql.ErrNoRows)`. Works today (no wrapping), but fragile if anyone wraps. |
| M3 | `internal/db/party.go:186` | `n, _ := res.RowsAffected()` silently swallows error. If RowsAffected fails, the "no party_member matched" check is a lie. |
| M4 | `internal/oracle/scryfall.go:80-84` | `fetchScryfall` retries once on transient failure with a flat 500ms sleep, then bubbles up the second error. The 429-handler in `fetchScryfall` *also* sleeps for Retry-After, but then returns an error rather than retrying — caller must wait *another* 120ms gate before retrying, doubling the rate-limit window. |

### Low — comment / docs only

| # | Location | Issue |
|---|---|---|
| L1 | `internal/astload/corpus.go:14-17` | Package doc example uses `log.Fatal(err)` — misleading for library callers; real callers should handle the error inline. Cosmetic. |

### Won't fix — intentional

- `internal/db/sqlite.go:131,144` — `crypto/rand.Read` panic is intentional (documented as fatal).
- `internal/oracle/scryfall.go:69` — "no-op on fresh process" comment describes correct behavior on the gate, not a stub.
- `internal/astload/loader.go` — large file but the warning-collection pattern (per-card `d.warnf`, accumulated in `Corpus.ParseWarnings`) is the intended design, not silent failure.

## Inline fixes shipped this PR (top 5)

| # | What changed | File |
|---|---|---|
| H1 | `Lookup` now logs a `cache write failed` warning via `log` and bubbles the error into a sentinel `ErrCacheWrite` consumers can ignore but tooling can observe. The card pointer is still returned (the caller got fresh data) so behavior matches the prior contract. | `internal/oracle/scryfall.go` |
| H2 | `applyMigrations`'s DROP/CREATE pair for `showmatch_elo` now checks both `Exec` errors and returns them. The DROP failure is non-fatal only when the table never existed; any other DROP error short-circuits the migration. | `internal/db/sqlite.go` |
| H3 | `PersistGameTx` now returns the error from `LastInsertId()` and rolls back the transaction. Orphan seat rows are no longer possible. | `internal/db/showmatch.go` |
| H4 | `LoadOwnerGames`'s opponents sub-query now propagates `QueryContext` errors, checks `oppRows.Scan` errors (skipping the bad row rather than appending ""), and inspects `oppRows.Err()` after the loop. | `internal/db/showmatch.go` |
| H5 | `CreateParty` now wraps party insert + host-member insert in a single `tx`, rolling back on either failure. No more orphan empty-lobby parties. | `internal/db/party.go` |

## Deferred (not in this PR)

- **M1** Party-code collision detection — needs SQLite error-code inspection
  via `modernc.org/sqlite` driver-specific errno; left as a follow-up.
- **M2** `errors.Is` migration — touches 4 sites; cosmetic until someone
  wraps a SQL error.
- **M3** `RowsAffected()` error swallow in `party.go:186` — same one-line
  pattern; fold into the M2 sweep.
- **M4** Scryfall 429 + transient-retry policy — bigger design change
  (single retry budget vs. layered sleeps); needs caller buy-in.
- **L1** Package doc cosmetics.
