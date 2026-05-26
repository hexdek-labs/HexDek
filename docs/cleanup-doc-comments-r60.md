# Cleanup — Exported Doc-Comment Audit (R60 Versailles Phase 2H)

**Date:** 2026-05-25
**Branch:** `dev/cleanup-doc-comments-r60`
**Scope:** audit exported function / type / var doc comments for
(a) stale comments referring to deleted code paths, (b) TODOs older
than r48 with no owner, and (c) doc comments contradicting current
behavior. Fix the cleanest 15-20.

## Method

1. Grep for `// TODO`, `// FIXME`, `// XXX`, `// HACK`, `// Deprecated`
   across non-test Go files (`*.go` minus `_test.go` minus
   `node_modules`).
2. Grep for `// Returns nil/true/false ...`, `// Always X`, `// Never X`
   doc claims and spot-check each against the code.
3. Grep for references to specific symbols (`see X.go`, `lives in Y`,
   PR/commit refs) and verify the named symbol still exists with the
   described behavior.
4. Spot-check files touched in earlier Versailles 2 PRs for residue
   the deletions might have left behind.

## Result classification

**Total TODO-style markers in non-test Go:** 17 (out of a corpus of
~3,500 Go files). All 17 are post-r48 era and trace to specific
deferred work (Unfinity Space Sculptor, Aetherdrift Station payoff
dispatch, Tasigur graveyard recursion, etc.). **No "older than r48
with no owner" TODOs found** — the codebase is well-maintained on
this dimension.

**Doc-vs-code mismatches found and fixed: 12.**

## Category 1 — Wrong CR rule reference (2 sites)

- `internal/gameengine/keywords_batch6.go:33` — Gift listed as
  `CR §702.174`. Real reference is `§702.192` (per the canonical
  handler in `keywords_gift.go`). Fixed.
- `internal/gameengine/resolve_helpers.go:4435` — Gift case body
  prefixed `CR §702.XXX`. Fixed to `§702.192` to match.

## Category 2 — Stale era reference (1 site)

- `cmd/parser-coverage/main.go:17-18` — package doc comment named the
  default output path as `docs/parser-coverage-r41.md` (the r41-era
  baseline). The r60 era is closed (per CLAUDE.md and the
  `parser-coverage-backlog-r60.md` sibling), so the default is
  effectively a snapshot path. Reworded the comment to flag that
  the default is the historical baseline and new runs should pass
  an era-tagged `--out`. The default flag value itself is unchanged
  — preserving behavior for any script that depends on it.

## Category 3 — Dead commented-out code with TODO sentinel (1 site)

- `internal/heimdall/replay.go:249-258` — six lines of
  `// observation.ComboAttempted = false` / `// observation.CausalPivot = nil`
  commented out under TODO comments. The struct's zero-value already
  handles these (every field defaults to its zero). The dead
  assignments were redundant orientation. Collapsed to a single
  comment block explaining the deferral.

## Category 4 — Doc-vs-code behavior mismatch (8 sites)

| File | What the doc said | What the code does | Fix |
|---|---|---|---|
| `cmd/hexdek-freya/cluster_export.go:91` | "Returns nil if dp is nil or has no clusters" | Returns nil only for nil dp; empty-clusters returns a populated struct with metadata. | Clarified that empty-clusters returns the metadata-only struct (intentional — clients distinguish "no clusters" from "export failed"). |
| `internal/gameengine/per_card/heliod_sun_crowned.go:29-37` | "full until-end-of-turn keyword tracking is not yet implemented" + "UEOT keyword grants are not yet tracked in the layers pipeline" | The R52 batchM port stamps `kw:lifelink` + schedules a `next_end_step` cleanup delayed trigger. UEOT semantics correct end-to-end. | Reframed: UEOT works via flag + delayed trigger; the remaining gap is that this isn't using the canonical layer-7 pipeline, so it skips layer-dependency ordering. |
| `internal/moxfield/imports.go:155-157` | "Returns an error only on IO failure" | Also returns error for empty DeckID (validation). | Added validation-failure to the list. |
| `internal/hexapi/deckmeta.go:212-215` | `loadFreyaSystemTags` "Returns nil when: strategy.json missing / unparseable / archetype empty" | Also returns nil when `decksDir / owner / id` is empty (caller misuse). | Added that case to the list. |
| `cmd/hexdek-thor/tracer.go:177-179` | `newCardTraceRecorder` "Returns nil if the collector is nil" | The function doesn't take a collector argument and never returns nil. The doc was copy-pasted from `TraceCollector.Begin` (line 318). | Rewrote: this constructor always returns non-nil; the nil-collector guard lives in `Begin`. |
| `cmd/hexdek-thor/goldilocks.go:4159-4160` | `executeKeyword` "Returns true if execution succeeded" | The function signature has no return value at all. | Rewrote: dispatch-only; verification lives in `verifyKeyword`. |
| `internal/hexapi/csrf.go:106-109` | `VerifyToken` "All other return values are constant — no information leaks back to the attacker about which validation step failed" | The function returns one of 4 distinct sentinel errors (`ErrCSRFMissing` / `ErrCSRFMalformed` / `ErrCSRFBadSignature` / `ErrCSRFExpired`). The "no information leaks" claim is true only at the HTTP boundary, where handlers collapse them into 403. | Reworded: 4 sentinels returned, HTTP handlers must collapse into a single 403 to avoid info-leak; sentinels preserved for audit logs. |
| `internal/hexapi/deckmeta.go:695-696` | `handleListTags` "Always returns 200 with a JSON array (possibly empty)" | Also returns 500 on DB error. | Reworded: 200-with-array on happy path; 500 on DB error; empty result for unknown owner is the 200 path. |

## Skipped (documented as false-positives or not residue)

- **All `r41` / `r42` / `r44`-`r57` references in active code** —
  every one I sampled documents the specific Loki bug-fix incident
  that drove the current code shape (e.g., `internal/gameengine/etali.go:106`
  "CardIdentity invariant violation seen in r41/r42 goldilocks").
  These are valuable historical context, not residue.
- **`DEPRECATED` markers on `Seat.SpellsCastThisTurn` /
  `Seat.DescendedThisTurn` in `internal/gameengine/state.go:860,875`** —
  88 active callers still reference these fields (verified in
  Versailles Phase 2D). The DEPRECATED tag accurately documents an
  in-flight drain-then-delete migration; the call-site sweep is a
  separate PR.
- **`// stub` markers on `sba704_5h/_5t/_5u/_5w/_5x/_5z`** —
  intentional SBA registration hooks for mechanics not yet modeled
  (Unfinity space sculptor, dungeons, battle protectors, Speed).
  Each has a CR reference and a TODO scoped to "when these card
  sets land." Not residue.
- **`// stub` markers on per_card `custom_*.go` handlers** —
  orientation for the gen_*.go scaffold the custom handler replaced.
  Each comment makes the gen-vs-custom split intentional.
- **`internal/game/combat.go` MVP P/T stub at line 318-322** — flagged
  in `docs/half-finished-features-r48.md` #5 as a deferred 3-step
  migration. Doc comment accurately tags the current state ("Placeholder:
  until we wire Scryfall power/toughness…"). Not residue.
- **`internal/gameengine/resolve_helpers.go:3744` "stat_modification
  is a log-only stub"** — slightly misleading wording (the specific
  pt_swap case body above DOES mutate base P/T directly) but the
  comment correctly describes the WIDER stat_modification family
  where layers own the real work. Untouched to avoid over-narrowing.

## Verification

- `go build ./...` clean.
- `go test -short` clean on `internal/gameengine/`, `internal/heimdall/`,
  `internal/moxfield/`, `cmd/hexdek-thor/`, `cmd/hexdek-freya/`.
- Pre-existing `internal/hexapi/` OpenAPI test failures
  (`/api/games/search` missing from spec) confirmed unrelated —
  same failures pre-exist on main per the Versailles Phase 2F / 2G
  verification.

## Net diff

11 files changed, comments-only:

- `cmd/hexdek-freya/cluster_export.go`
- `cmd/hexdek-thor/goldilocks.go`
- `cmd/hexdek-thor/tracer.go`
- `cmd/parser-coverage/main.go`
- `internal/gameengine/keywords_batch6.go`
- `internal/gameengine/per_card/heliod_sun_crowned.go`
- `internal/gameengine/resolve_helpers.go`
- `internal/heimdall/replay.go`
- `internal/hexapi/csrf.go`
- `internal/hexapi/deckmeta.go`
- `internal/moxfield/imports.go`
