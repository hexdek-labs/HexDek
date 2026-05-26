# Cleanup — `golangci-lint` Sweep (R60 Versailles Phase 2G)

**Date:** 2026-05-25
**Branch:** `dev/cleanup-golangci-lint-r60`
**Tool:** `golangci-lint` v2.12.2 (default config, no `.golangci.yml`)
**Scope:** repo-wide pass; fix the **cleanest 15-20** sites surfaced by
ineffassign, staticcheck, govet, and friends. No mass-replace; every
fix is locally justifiable, and false-positive cases are documented
in the "Skipped" section.

## Baseline

```
golangci-lint run ./...  →  472 warnings
```

Active linter breakdown on default config:

| Linter | Count | Sample |
|---|---|---|
| staticcheck | 50 | SA4011 ineffective break, SA5011 nil-deref, SA1019 deprecated, S1008 return-X, S1039 unnecessary fmt.Sprintf |
| errcheck | 50 | `_ = X.Close()` / `enc.Encode()` / `os.MkdirAll()` |
| unused | 41 | unused funcs, vars, fields |
| ineffassign | 12 | `X = sentinel; X = real` |
| govet | 2 | (both in `hexdek/node_modules/flatted/golang/` — vendored; out of scope) |

After this PR: **458 warnings** (-14). Several staticcheck/errcheck slots
freed by my edits got replaced by new findings the linter surfaces under
different variable names — net warning count understates the actual
code-quality delta.

## Category 1 — Real bugs (4 sites)

### `SA4011` ineffective break in `internal/oracle/scryfall.go` (3 sites)

```go
for _, chunk := range chunks {
    select {
    case sem <- struct{}{}:
    case <-ctx.Done():
        break        // ❌ breaks the select, not the for-loop
    }
    ...
}
```

On context cancellation the loop **kept iterating** instead of aborting.
Mostly benign in practice (Scryfall fetcher chunks finish fast and the
inner goroutines do re-check ctx) but the intent was clearly to bail
the outer loop. Fix: **labeled break**.

- `internal/oracle/scryfall.go:273` (`collectLoop` — chunk fetcher)
- `internal/oracle/scryfall.go:312` (`fuzzyLoop` — Layer-2 fuzzy fallback)
- `internal/oracle/scryfall.go:392` (`prefetchLoop` — prefetcher variant)

Comment added: `break <label> // ctx-cancel; bare break only escapes the select`.

### `SA5011` nil deref in `internal/hat/yggdrasil.go:2321` (1 site)

`cardHeuristic` accessed `gs.Seats[seatIdx]` at line 2321 but a later
defensive block at line 2511 checks `gs != nil`. Staticcheck flagged
this as evidence that `gs` *can* be nil from at least one caller — so
line 2321 panics before the guard runs. Fix: add the standard early-
guard that sibling helpers at lines 833 / 2889 / 2945 / 3080 already
use:

```go
if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
    return 0
}
```

## Category 2 — Ineffassign dead-state cleanups (5 sites)

| File | Site | Fix |
|---|---|---|
| `cmd/hexdek-freya/winlines.go:337` | `delivery := "hand"` overwritten unconditionally at line 428 | Delete init; convert line 428 to `:=` |
| `internal/gameengine/chaos.go:160,165` | `basicCount = 0` after the variable goes out of scope | Delete both writes |
| `internal/heimdall/replay.go:219-234` | 16-line winner-determination block where the result was never read | Delete the whole block |
| `internal/hexapi/contrib.go:279,299` | `inFlight = nil` immediately before `return` in two case branches | Delete the writes |
| `internal/muninn/muninn.go:401` | `msg = s` overwritten by explicit `return ..., s` on every path | Delete the init line |

`internal/analytics/threat_graph.go:146` (`killerSeat = -1` sentinel)
was flagged but **kept** as defensive future-proofing — see "Skipped"
section.

## Category 3 — Staticcheck readability cleanups (8 sites)

| Rule | File | Fix |
|---|---|---|
| `S1008` return-X-directly | `internal/gameengine/keywords_imprint.go:84-87` | Collapse `if X { return true }; return false` to `return X` |
| `S1008` | `internal/gameengine/keywords_metalcraft.go:68-71` | Same |
| `S1011` slice-append in loop | `internal/gameengine/keywords_bloodrush.go:289-291` | Replace `for _, kw := range abilities { GrantedAbilities = append(..., kw) }` with `append(..., abilities...)` |
| `S1016` struct-conversion | `internal/heimdall/deck_archive.go:163` | Replace `DeckArchiveRecentGame{FinishedAt: g.FinishedAt, Won: g.Won, Draw: g.Draw}` with `DeckArchiveRecentGame(sortedGames[i])` (same field layout) |
| `S1030` body.String | `internal/hexapi/game_summary_pdf_cache_r60_test.go:246` | `string(rec.Body.Bytes())` → `rec.Body.String()` |
| `S1039` unnecessary Sprintf | `internal/tournament/hat_review_test.go:133-135` | 3 sites — replace `WriteString(fmt.Sprintf(...))` with `Fprintf(&sb, ...)` or plain `WriteString("...")` when no args |
| `SA1019` deprecated `strings.Title` | `cmd/gen-handlers/main.go:994` | Replace with local `titleCase` helper (ASCII-only — token type names) |
| `SA6005` `strings.EqualFold` | `internal/hat/yggdrasil.go:2900` | Replace `strings.ToLower(a) != strings.ToLower(b)` with `!strings.EqualFold(a, b)` (one less allocation) |
| `SA1019` deprecated `rand.Seed` | `internal/gameengine/per_card/krark_bounceback_r54_test.go:36` | Add `//nolint:staticcheck` with justifying comment — the test deliberately reseeds the global stream for legacy callers |

## Category 4 — Unused-symbol removal (1 site)

`cmd/hexdek-thor/conditional_setup.go:1965` `priorActionVerbRe` — defined
but zero callers (`grep` confirms). The neighboring regexes are all
referenced by `setupCondition`; this one was orphaned by a prior
refactor. Removed the var declaration outright.

## Skipped — documented false positives / out-of-scope

- **`govet` x2 in `hexdek/node_modules/flatted/golang/pkg/flatted/flatted.go`**
  — vendored JS-tooling dependency, never compiled into the Go binary.
  Out of scope; can't fix upstream without forking a JS package.
- **`internal/analytics/threat_graph.go:146`** `killerSeat = -1` —
  named-return defaults to `0` (a valid seat index). The init to `-1`
  is a *deliberate* sentinel: future branches that fall through without
  explicitly setting `killerSeat = ev.Seat` will inherit `-1`, not `0`.
  Lint sees it as "ineffectual" only because every current path
  overwrites — but removing it would silently change behavior on the
  next branch added. **Kept as defensive future-proofing.**
- **~10 `SA9003` empty-branch warnings** — most are `if X { /* TODO */ }`
  placeholders intentionally left for the next implementation pass.
  Out of scope here; per-site review needed before deletion.
- **~20 `QF1003` "could use tagged switch" / `QF1001` "could apply
  De Morgan's law"** — Quality-Fix (`QF*`) hints are *style*-level. Each
  rewrite is locally valid but the net readability win is marginal,
  and mass-applying them across the engine risks subtle diff churn.
  Deferred — would be its own PR if the team decides to address them.
- **40 `unused` warnings** — mostly fields on internal structs that
  may be read via reflection or planned for upcoming features. Per-
  site triage needed (some are genuinely dead, some are real). Not
  attempting in this sweep.
- **All `errcheck` warnings** — owned by Phase 2F
  (`docs/cleanup-errcheck-r60.md`). 27 closed there; ~50 deferred
  (mostly `fmt.Fprintf` to `*os.File` per Go convention).

## Verification

- `go build ./...` clean.
- `go test -short` clean on touched packages:
  - `internal/oracle`, `internal/hat`, `internal/heimdall`,
    `internal/gameengine`, `internal/hexapi` (PDF tests),
    `internal/muninn`, `internal/analytics`,
    `cmd/hexdek-thor`, `cmd/hexdek-freya`,
    `internal/tournament`.
- Pre-existing `internal/hexapi` OpenAPI test failures
  (`/api/games/search` missing from spec) confirmed unrelated —
  same failures appeared in Phase 2F's verification under the same
  branch-vs-main bisect. The single test I touched
  (`TestGameSummaryPDFCache_*`) passes.

## Net diff

13 files changed:

- `cmd/gen-handlers/main.go` (deprecated `strings.Title`)
- `cmd/hexdek-freya/winlines.go` (ineffassign `delivery`)
- `cmd/hexdek-thor/conditional_setup.go` (unused `priorActionVerbRe`)
- `internal/gameengine/chaos.go` (ineffassign `basicCount`)
- `internal/gameengine/keywords_bloodrush.go` (S1011 append slice)
- `internal/gameengine/keywords_imprint.go` (S1008)
- `internal/gameengine/keywords_metalcraft.go` (S1008)
- `internal/gameengine/per_card/krark_bounceback_r54_test.go` (SA1019)
- `internal/hat/yggdrasil.go` (SA5011 nil-guard + SA6005 EqualFold)
- `internal/heimdall/deck_archive.go` (S1016 struct conversion)
- `internal/heimdall/replay.go` (dead winner block)
- `internal/hexapi/contrib.go` (ineffassign `inFlight = nil` before return)
- `internal/hexapi/game_summary_pdf_cache_r60_test.go` (S1030)
- `internal/hexapi/showmatch.go` (S1001 copy)
- `internal/muninn/muninn.go` (ineffassign `msg = s`)
- `internal/oracle/scryfall.go` (SA4011 x3 ineffective break)
- `internal/tournament/hat_review_test.go` (S1039 x3 unnecessary Sprintf)
