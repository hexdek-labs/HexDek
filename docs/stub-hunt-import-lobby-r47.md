# Stub Hunt R47 — `internal/{deckparser,moxfield,hub,party,ws}/`

**Date:** 2026-05-20
**Branch:** `dev/stub-hunt-import-lobby-r47`
**Scope:** Deck-import + lobby + WebSocket transport layer (≈3.4K LoC, 9 source files).

## Methodology

1. `grep` for explicit stub markers (`TODO|FIXME|stub|placeholder|not implemented`) — **zero hits** across this scope.
2. `grep` for silent-error patterns — `_ = err`, `_ = json.Unmarshal(...)`, `_, _ = db.Exec(...)`, dropped `os.WriteFile` returns.
3. `awk` scan for short function bodies (≤3 statements) — separated thin getters from real stubs.
4. Hand-read every file end-to-end; the largest is `party/handler.go` (740 LoC) and `ws/handler.go` (593 LoC).
5. Cross-referenced `formatDecklist` (moxfield) against its test fixtures to verify nondeterminism wasn't already pinned.

## Findings, by severity

| # | Sev | File:line | Issue | In this PR |
|---|---|---|---|---|
| 1 | **High** | `moxfield/moxfield.go:198-224` | `formatDecklist` iterates `data.commanders()` / `data.mainboard()` / `sideboard()` / `companions()` — Go map iteration is random. For **partner decks** (2 commanders), the `COMMANDER:` line order flips between fetches; whoever appears first becomes the primary commander in the downstream deckparser. Same deck, different "primary" each import. | **Yes** |
| 2 | **High** | `party/handler.go:551-552` | `_, _ = h.DB.ExecContext(ctx, "UPDATE party SET state='playing' WHERE id=?", partyID)` — error dropped. If the UPDATE fails (busy DB, conflict, schema mismatch), `gameengine.StartGame` already succeeded so a game exists, but the party row still says `state='lobby'`. Subsequent `getParty` misreports, the host could call `startGame` again and create a second game record. | **Yes** |
| 3 | **Med** | `ws/handler.go:165` | `_ = json.Unmarshal(env.Payload, &msg)` in the `chat` dispatcher — malformed payload yields a chat broadcast with empty `text`. Other clients see a ghost message. Should reject and send an `error` envelope to the sender, matching the `default` case and the pattern used by every `game.*` handler. | **Yes** |
| 4 | **Med** | `moxfield/textlist.go:135-136` | `cleanName` calls `regexp.MustCompile(...)` twice per invocation. With 99-card decklists, that's 198 unnecessary compilations per `ParseDecklist`. Hoist to package vars (same pattern this file already uses at line 25 for `lineRE`). | **Yes** |
| 5 | **Low** | `deckparser/deckparser.go:151-202` | `SupplementWithOracleJSON` increments `merged++` on every successful P/T merge — then `return nil` and throws the counter away. The function signature is `(...) error`, so the caller has no way to know if it merged zero cards (e.g., wrong file format) versus thousands. Surface the count via an additional return, or drop the dead counter. | **Yes (drop counter)** |
| 6 | Low | `party/handler.go:740` | `var _ = context.Background` — labeled "context-helper for tests" but it's just a no-op reference to a function value. Dead. | Yes (drop) |
| 7 | Low | `hub/hub.go:117-129` | `PartyDeviceIDs` returns map keys in random Go-iteration order. Used by lobby UI ("Useful for diagnostics and lobby UI"), so players visibly shuffle between refreshes. Sort for stable display. | Yes (sort) |
| 8 | Info | `party/handler.go:297-299` | `_ = os.MkdirAll(importDir, 0755); _ = os.WriteFile(txtPath, ...)` after a successful Moxfield fetch. Soft cache: failure means next import re-hits the API. Could log; not data-loss. | Skip |
| 9 | Info | `ws/handler.go:92-94` | `websocket.AcceptOptions{InsecureSkipVerify: true}` accepts any Origin. Documented MVP choice; revisit before public deploy. | Skip |
| 10 | Info | `hub/hub.go:57` | `Register` calls `existing.Close()` while holding the hub's write lock — `Close()` performs a WS handshake-close write which can stall on slow clients. Move outside the lock or run in a goroutine. Latency hazard, not correctness. | Skip |
| 11 | Info | `moxfield/parser.go:4-5` | Doc comment says "Future work: integrate with the actual Moxfield public API (api.moxfield.com/v3/decks/all/{id})." This has actually shipped in `moxfield.go` — the comment is stale, the work is done. | Skip (doc only) |
| 12 | Info | `deckparser/deckparser.go:294,301-302` | `LoadMetaFromJSONL` `continues` on per-row unmarshal errors and on bad P/T strings without counting skips. Probably fine — JSONL is line-oriented and one bad row shouldn't kill the load — but a skip count would help diagnose dataset rot. | Skip |

## Deliberately not flagged

- **`party/handler.go:573,645`** (`_ = json.NewDecoder(r.Body).Decode(&req)` in `addAI` / `saveAsPremade`) — comments confirm body is optional. Working as intended.
- **`party/handler.go:729-731`** `_ = err` after `enc.Encode` — header already sent, nothing useful to do. Standard pattern.
- **`moxfield.go:139,179`** `defer resp.Body.Close()` without checking the error — standard idiom; close errors on a reader are not actionable.
- **`hub/hub.go`** — single mutex, snapshot-then-broadcast pattern. Well-designed; nothing to fix.
- **`deckparser/deckparser.go` MDFC face-match loops** (lines 595-611, 619-633) — repetitive but each is a small, separate concern; refactoring into a single helper would couple corpus and meta lookups.

## Fixes shipped

### 1. `moxfield.formatDecklist` — deterministic order

Sort the keys of `commanders / mainboard / sideboard / companions` before emitting. Partner decks (2 commanders) now emit the same `COMMANDER:` lines in the same order on every fetch.

### 2. `party.startGame` — surface UPDATE error

The `UPDATE party SET state='playing'` failure-mode would leave the system in an inconsistent state. If the UPDATE errors, log it and return a 500 — the game record exists in `gameengine.StartGame`, but the caller knows the lobby↔game transition didn't fully complete.

Trade-off: the original code chose "always return 201" so the client can advance regardless. The new behavior errors only on actual DB failure (the happy path is unchanged), so well-behaved deploys see no difference; only the failure case becomes diagnosable.

### 3. `ws.dispatch` chat — validate JSON before broadcasting

Apply the same `if err := json.Unmarshal(...); err != nil { sendErr; return }` pattern used by every `game.*` handler. Malformed chat payloads now get a targeted error envelope instead of a ghost broadcast.

### 4. `moxfield.cleanName` — hoist regexps

Two `regexp.MustCompile` calls moved from inside `cleanName` to package-level `var foilMarkerRE`, `var bracketTagRE`. Same matching behavior, ~190× fewer compilations per import.

### 5. `deckparser.SupplementWithOracleJSON` — drop dead `merged` counter

The counter was incremented but never returned and never logged. Removing it keeps the loop simpler. (Considered surfacing the count via a return; declined because every existing caller already ignores the `error` return and adding a `(int, error)` signature would ripple through callers for no observed need.)

### Bonus tweaks (zero-risk)

- **`party.handler.go`** — deleted `var _ = context.Background` dead helper.
- **`hub.PartyDeviceIDs`** — sorted output for stable lobby UI.

## Tests

- `go test ./internal/{deckparser,moxfield,hub,party,ws}/... -count=1` — all green.
- `formatDecklist` test (`TestFormatDecklist`) didn't pin order, so the determinism fix is backward-compatible.
- `go build ./...` — clean.

## Open follow-ups

- WS Origin verification (`InsecureSkipVerify: true`) needs revisit before broad deploy.
- Hub `Register` should drop the old-conn `Close()` outside the write lock.
- Stale doc comment in `moxfield/parser.go` (top-of-file "Future work" line that's already shipped).
- Dataset-rot visibility: count and surface skipped JSONL rows in `LoadMetaFromJSONL`.
