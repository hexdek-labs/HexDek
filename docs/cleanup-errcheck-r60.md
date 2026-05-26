# Cleanup — `errcheck` Sweep (R60 Versailles Phase 2F)

**Date:** 2026-05-25
**Branch:** `dev/cleanup-errcheck-r60`
**Tool:** `errcheck` v1.20.0 (`go install github.com/kisielk/errcheck@latest`)
**Scope:** repo-wide pass; fix the **cleanest 15-20** sites where an error
return was silently dropped. Each fix must be locally justifiable (log,
handle, or `_ =` with a comment) and not change observable behavior on
the happy path.

## Baseline

```
errcheck ./...  →  1423 findings
```

After this PR: **1396 findings** (-27).

Of the remaining 1396, the vast majority are intentional:
- `fmt.Fprintf(w, ...)` to writers (~410 sites) — Fprintf returns
  `(int, error)`; convention is to drop unless writing to a network
  socket. Out of scope.
- `defer f.Close()` on read-only `os.Open` (~200 sites) — Close after
  read is a Go idiom; close-error doesn't affect the data already read.
- `_test.go` files (~600 sites) — `json.Unmarshal(...)` on hard-coded
  fixtures where the parse cannot fail. Out of scope.

The 27 fixed in this PR all fall into one of four high-signal patterns:
**silently dropped data writes** (where a failure leaves an empty or
truncated file), **silently dropped state mutations** (where the game
state diverges from disk), **rotation operations** (where a rename
failure leaves the file in the wrong place), and **`Close()` on the
error path** (where the close-error is real but the original error is
the one the caller needs).

## Category 1 — Drop-the-error-on-data-supplement (3 sites)

`*MetaDB.SupplementWithOracleJSON` enriches deck metadata with
Scryfall-side P/T and type-line corrections. Silently dropping the
error means downstream analytics quietly under-report — same pattern
documented as `half-finished-features-r48.md` #3, where one site was
patched and three siblings escaped the original sweep.

| Site | Fix |
|---|---|
| `cmd/hexdek-heimdall/main.go:128` | `log.Printf("warn: oracle supplement failed (%s): %v — metadata may be incomplete", ...)` |
| `cmd/hexdek-heimdall/main.go:532` | same wording (sibling site in the post-game replay path) |
| `cmd/hexdek-valkyrie/main.go:96` | same wording (the ranker driver) |

## Category 2 — Drop-the-error-on-mutation (1 site)

`internal/ai/autopilot.go:128` — the autopilot's `tapAllLands` loop
called `game.TapLandForMana(...)` and discarded its `(string, error)`
return. A failed tap (e.g. DB write conflict on the SQLite write
path) would silently leave the land untapped, the mana pool unfilled,
and the next `castSpells` step to fail at a higher level with no
breadcrumb. Now logs the failing instance + game + seat.

## Category 3 — Drop-the-error-on-file-write (5 sites)

Silent file-write failures leave **empty files** that overwrite the
prior good state without warning. Worst case: a long-running training
loop's checkpoint silently stops persisting and the user discovers it
only when reload returns no data.

| Site | Fix |
|---|---|
| `internal/heimdall/observer.go:194` (`os.Rename` rotation) | `log.Printf("heimdall: rotate %s -> .prev failed: %v (will append instead)", ...)` — append-instead is safe behavior; warn but continue |
| `internal/heimdall/observer.go:203` (`enc.Encode(s)` loop) | propagate by `return`-ing inside the loop after `log.Printf` |
| `internal/heimdall/seed_binary.go:136` (`os.WriteFile` for deck-index) | log + leave `dirty` flag set so a future flush retries |
| `internal/hat/micronet.go:373` (`SaveMicroNet` after Train) | log; in-memory net stays active |
| `internal/hat/selfplay.go:151` (`os.Rename` rotation) | log + fall through to append-to-existing-file path |

## Category 4 — Drop-the-error-on-MkdirAll (3 sites)

Each was followed by a `WriteFile` / `OpenFile` that would surface a
"no such directory" error anyway, but the original signal (permission
denied vs read-only mount vs disk-full) is more informative than the
downstream `ENOENT`. Each now wraps the mkdir error and short-circuits.

| Site | Fix |
|---|---|
| `internal/hat/curse.go:673` (`SavePool`) | `return fmt.Errorf("savepool: mkdir %s: %w", dir, err)` |
| `internal/hat/curse.go:699` (`SaveAllPools`) | same |
| `internal/hat/distillation.go:259` (`AppendDNAEnrichedSamples`) | same shape |
| `internal/hat/distillation.go:569` (`SaveDistillationManifest`) | same shape |

## Category 5 — `Close()` on the error-already-set path (8 sites)

When `Open(...)` succeeds but a subsequent operation fails, the `Close()`
in the cleanup must be best-effort — the original error is the one the
caller needs. Standard Go idiom is `_ = X.Close()` with a comment
documenting that the caller can't use the close error.

| Site | Fix |
|---|---|
| `internal/anticheat/spotcheck.go:254,263` (`rows.Close`) | `_ = rows.Close() // best-effort cleanup; scan error / rows.Err below is the reportable one` |
| `internal/db/sqlite.go:34,38,42` (`db.Close` after ping/migrate failure) | `_ = db.Close() // best-effort cleanup before returning the originating error` (replace_all hit all 3) |
| `internal/heimdall/observer.go:200` (`defer f.Close`) | `defer f.Close() //nolint:errcheck — append-only audit log; partial writes survive close errors.` |
| `internal/hat/distillation.go:264` (`defer f.Close`) | `defer f.Close() //nolint:errcheck — append-only JSONL; encode-loop errors propagate via the return below.` |
| `internal/hexapi/artcache.go:49` (`w.Write` on `http.ResponseWriter`) | `_, _ = w.Write(data) // ResponseWriter.Write error means client closed the connection — nothing to do.` |

## Category 6 — pprof/profile file lifecycle (5 sites)

`cmd/hexdek-thor/main.go:557` and `cmd/hexdek-tournament/main.go:208,
209, 377, 378, 383, 384, 372` — each profile write was a sequence of
`WriteHeapProfile(f) ; f.Close() ; log.Println("written")` where any
failure silently logged a success message even when the file was bad.
Now each gates the log message on a successful close and surfaces both
write- and close-errors.

## Pattern not chosen this PR

These were tempting but deferred:

- **All ~410 `fmt.Fprintf(w, ...)` sites** in `*_test.go` and `cmd/*/main.go`
  report files — stripping the error returns adds noise without changing
  behavior. The Go community convention is to drop Fprintf to a `*os.File`
  obtained via `os.Create` where the error path is "log the file path
  and move on."
- **Test-file `json.Unmarshal` of hard-coded fixtures** — the parse
  cannot fail; the error return is a compile-time tax. Out of scope.
- **`defer tx.Rollback()`** without a nolint comment — the team
  convention in `internal/credits/credits.go:202` is
  `defer tx.Rollback() //nolint:errcheck — committed on success below.`
  Sweep is worthwhile but mechanical (~12 sites in `internal/db/*.go`,
  `internal/friends/*.go`) and best done as its own PR.

## Verification

- `go build ./...` clean.
- `go test -short ./internal/hat/... ./internal/heimdall/... ./internal/ai/... ./internal/anticheat/... ./internal/db/... ./cmd/hexdek-thor/...` all green.
- `./internal/hexapi/...` test failure (TestOpenAPICoverage_AllRegisteredRoutesAreDocumented missing `/api/games/search`) **pre-exists on main**, confirmed by `git stash && go test ...`. Unrelated to this PR.

## Net diff

14 files changed, mostly +N inserts:

- `cmd/hexdek-heimdall/main.go`
- `cmd/hexdek-thor/main.go`
- `cmd/hexdek-tournament/main.go`
- `cmd/hexdek-valkyrie/main.go`
- `internal/ai/autopilot.go`
- `internal/anticheat/spotcheck.go`
- `internal/db/sqlite.go`
- `internal/hat/curse.go`
- `internal/hat/distillation.go`
- `internal/hat/micronet.go`
- `internal/hat/selfplay.go`
- `internal/heimdall/observer.go`
- `internal/heimdall/seed_binary.go`
- `internal/hexapi/artcache.go`
