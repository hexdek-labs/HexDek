# Cross-Package Duplicate Cleanup — R60 Phase 2C

Sweep of duplicate top-level declarations (functions, types, constants)
across the repo using a small `go/ast` scanner that hashes normalized
declaration bodies and groups cross-package matches.

**Baseline scan**: 25 cross-package duplicate groups.

## Consolidated (5)

Each consolidation preserves all callers; the redundant copy was either
deleted or reduced to a forwarder pointing at the canonical exported
version.

### 1. `nextLivingSeat` — 3 packages → 1 canonical

| Before | After |
|---|---|
| `cmd/hexdek-heimdall/main.go:891` | (deleted) |
| `cmd/hexdek-loki/main.go:1212` | (deleted) |
| `cmd/hexdek-odin/main.go:313` | (deleted) |
|   | **`gameengine.NextLivingSeat`** (new export in `internal/gameengine/multiplayer.go`) |

Three identical 9-line copies in three command-line tools. Belongs in
`gameengine` next to the existing `LivingSeats` helper (and used by
all three tools to step a finished game state through the turn order
without invoking the engine's turn-loop). All three callers updated
to call `gameengine.NextLivingSeat(gs)` directly.

### 2. `cardHasKeyword` — engine private + per_card → engine exported

The engine had `cardHasKeyword` private in `stack.go`; the
`internal/gameengine/per_card/` subpackage had a verbatim copy. The
per_card copy carried the comment "mirrors the engine's internal
helper" — i.e., a known sync risk. Consolidated:

- `internal/gameengine/stack.go`: renamed to **`CardHasKeyword`** (exported), kept a lowercase `cardHasKeyword` forwarder for the ~dozen in-package callers (saves churn).
- `internal/gameengine/per_card/helpers.go`: replaced the body with a one-line forwarder to `gameengine.CardHasKeyword`. The `gameast` import that the local body relied on is no longer needed and was removed.

### 3. `cardHasType` (strict) — per_card + hat → engine exported

`internal/gameengine/per_card/helpers.go` and `internal/hat/opponent_profile.go`
both carried byte-identical 9-line `cardHasType(c *gameengine.Card, t string) bool`
helpers. They differed semantically from the engine-internal
`cardHasType` in `cost_modifiers.go` (which also does a TypeLine
substring match — broader behavior).

Added a new exported **`gameengine.CardHasTypeExact`** with the strict
Types-only semantics that the per_card and hat callers actually need.
Replaced both verbatim copies with one-line forwarders. Did NOT touch
the engine's internal broader `cardHasType` — that's a separate
intentional asymmetry documented below.

### 4. `normalizeName` + `foldAccent` — deckid + deckparser → deckparser canonical

Both `internal/deckid/hash.go` and `internal/deckparser/deckparser.go`
carried byte-identical copies of `normalizeName` (24 lines) and its
`foldAccent` helper (29 lines). The deckid copy carried a stale
comment "Duplicated from deckparser to avoid circular imports" — but
deckid already imports deckparser, so there's no cycle to avoid going
that direction.

- `internal/deckparser/deckparser.go`: added a **`NormalizeName`** export wrapping the existing private `normalizeName`.
- `internal/deckid/hash.go`: deleted the local 50+ line normalizer + accent-fold helper; replaced with a one-line forwarder `deckparser.NormalizeName(name)`. Removed the now-unused `unicode` import.

### 5. `max1` — 3 packages → built-in `max`

`cmd/hexdek-heimdall/main.go:510`, `cmd/hexdek-parity/main.go:109`,
and `internal/analytics/report.go:785` each defined `max1(v)` (two
int, one float64) that returned `max(v, 1)`. Repo is on Go 1.25
which has built-in `max`; replaced all 7 call sites with
`max(v, 1)` and deleted all three definitions.

## Skipped — intentional package isolation (8)

### `escapeCell` × 2 — `cmd/audit-ast-oracle/` + `cmd/audit-engine-dead/`

Each is a 5-line markdown-cell escaper in two `cmd/` tools I built
for earlier Versailles phases. Both are `package main` so they cannot
share private helpers directly. Creating a new `internal/auditutil`
package for 5 lines is overhead beyond the cleanup value.

### `pluralS` × 2 — `cmd/parser-coverage/` + `cmd/hexdek-freya/`

5-line "return 's' if n != 1 else '' " helper duplicated across two
unrelated `cmd/main` packages. Same reasoning as `escapeCell`.

### `shortDate` × 2 — `cmd/hexdek-huginn/` + `cmd/hexdek-muninn/`

6-line ISO-date prefix extractor. Same reasoning.

### `toSet` × 2 — `cmd/dump_drift/` + `cmd/hexdek-thor/`

6-line `[]string → map[string]bool` set constructor. Same reasoning.

### `boolToInt` × 2 — `internal/game/storage.go` + `internal/gameengine/per_card/jodah_the_unifier.go`

5-line ternary helper. Each call site is local enough that inlining
`if b { 1 } else { 0 }` is the alternative; creating a new util
package is heavier than the duplication.

### `writeJSON` × 3 — `internal/credits/` + `internal/friends/` + `internal/userprofile/`

4-line HTTP-handler JSON-writer. Could share via a new
`internal/httpjson` package, but `internal/hexapi` (the natural home)
already imports all three of these packages to register their routes
— consolidating into hexapi would create import cycles. Marginal
value below the new-package threshold.

### `SetAPIBaseForTest` and `FetchDeckByID` — `internal/archidekt/` + `internal/moxfield/`

Identical-looking bodies, but each modifies a different
**package-scoped** `apiBaseVar`. Each deck-importer is a self-contained
package representing a distinct external API; the look-alike test
helpers operate on different state. Consolidating would either require
generic plumbing or merging the two packages, neither of which is in
Phase 2C scope.

### Zone constants — `internal/game/types.go` vs `internal/gameengine/zone_cast.go`

Both define `ZoneLibrary` / `ZoneHand` / `ZoneGraveyard` / `ZoneExile`.
But the engine's `ZoneCommandZone = "command_zone"` (underscore) doesn't
match game's `ZoneCommand = "command"` (no underscore), and `internal/game`
uses a typed `Zone` while `internal/gameengine` uses untyped string.
The overlap is genuine but the type+value mismatch on the command-zone
constant means a naive merge would change serialization shape.
Out of scope.

### Resource constants — `cmd/hexdek-freya/` + `internal/analytics/`

`ResMana` / `ResCard` / `ResLife` / `ResToken` / `ResLand` / `ResDamage`
appear in both packages. `internal/analytics/resource.go` carries an
explicit comment: *"Resource types aligned with Freya's ResourceType
constants (which live in cmd/hexdek-freya and cannot be imported)."*
Go forbids importing main packages. The proper fix is extracting
freya's analysis core to `internal/freya`, which is a larger refactor
outside Phase 2C.

### `showmatchSeats` / `showmatchMaxTurn` — `cmd/hexdek-ceiling/` + `internal/hexapi/showmatch.go`

Both define `showmatchSeats = 4` and `showmatchMaxTurn = 80`. Each is
unexported in its own package — they're independent magic-number
declarations for two binaries that happen to share the same gauntlet
shape. Exporting from hexapi would couple cmd/hexdek-ceiling to a
much larger package just for 2 ints.

## Verification

- `go build ./...` clean across all changes.
- Touched-package tests clean: `internal/deckid`, `internal/deckparser`,
  `internal/gameengine`, `internal/gameengine/per_card`, `internal/hat`,
  `internal/analytics`, `cmd/hexdek-loki`.
- The 4 `cmd/` packages I touched (`heimdall`, `loki`, `odin`,
  `parity`) have no test files of their own, but `go build` exercises
  the source.

## Summary

| Action | Count |
|---|---:|
| Cross-package duplicate groups found | 25 |
| Consolidated | 5 (~80 lines deleted) |
| Skipped (intentional isolation / out of scope) | 8 categories covering remaining groups |

Five consolidations is the "cleanest" subset where the canonical home
is obvious, the import direction is feasible (no new cycles), and the
behavior change is provably nil (forwarder pattern) or upgrading to a
language built-in (`max1` → `max`).
