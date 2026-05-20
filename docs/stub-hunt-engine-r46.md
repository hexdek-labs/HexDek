# Stub Hunt — internal/gameengine/ core (R46)

**Date:** 2026-05-20
**Branch:** `dev/stub-hunt-engine-r46`
**Scope:** `internal/gameengine/` *excluding* `internal/gameengine/per_card/` and `_test.go` files.

## Method

Grep across the engine core for `TODO`, `FIXME`, `XXX`, `HACK`, `placeholder`,
`not implemented`, `unimplemented`, `MVP`, `stub`, `no-op`, `log only`.
Filter to non-test, non-per_card files. Read the surrounding handler body to
classify whether the marker is a real gap (function does no observable work)
or merely a stale comment on top of a real implementation.

`grep` raw hits:

- `TODO|FIXME|XXX|HACK|placeholder|not implemented|unimplemented` in core: **7** lines
- `MVP` markers: **160** lines (most are intentional simplification notes)
- `log-only|log only` in core: **7** lines

## Findings (core only — per_card excluded by request)

### Real stubs (handler logs/no-ops without doing the work)

| # | Location | Stub | Severity | Notes |
|---|---|---|---|---|
| S1 | `resolve_helpers.go:4275` — `case "gift":` | Logs `gift` event but never dispatches the nested gift/reward effects encoded in `Args`. | High | Bloomburrow gift mechanic. The comment even predicts nested ModificationEffects "if present" — they are, and nothing iterates them. |
| S2 | `resolve_helpers.go:2843` — `case "populate":` | Sets `kaResolved = false` and falls through to log only. | High | §701.30 — create a copy of a creature token you control. Per_card `trostani_selesnyas_voice.go` already documents this is a log-only stub. |
| S3 | `resolve_helpers.go:2764` — `case "explore":` | Just `drawOne`. | Med | §701.40: reveal top; if land → hand; else +1/+1 counter on source. Drawing is a wrong observable: it's a "card advantage if non-land" mechanic, not a draw. |
| S4 | `resolve_helpers.go:2774` — `case "proliferate":` | Only walks the proliferating seat's battlefield; only touches creatures with `+1/+1` counters; ignores poison/charge/loyalty/level/age/etc. and all opposing perms/players. | High | §701.27 — "any number of permanents and/or players that already have a counter on them. For each of them, choose a counter on that permanent or player. Add another of that kind." Proliferate matters for poison wins, planeswalker ults, Inexorable Tide combos. |
| S5 | `resolve_helpers.go:4068` — `case "reorder_top_of_library":` | Shuffles a hard-coded top 3 cards. Ignores any N encoded in `e.Args`. | Med | Scry/surveil/look-at-N effects parsed into this generic shape lose their N. |
| S6 | `resolve.go:2099` — `case "move":` (counter mod) | Pure no-op. | Med | §122.5 move-counters. AST gap: `gameast.CounterMod` only carries one `Target` (no source/dest split). Real fix needs an AST change; flagged here for the parser team. |
| S7 | `resolve_helpers.go:3633` — `stat_modification` default arm | Log-only. | Low | The switch above it already handles `set_pt`, `switch_pt`, etc. The remaining fall-through is rare modkinds; layers system covers the common cases. |
| S8 | `sba.go:1290` — `sba704_5u` (Space Sculptor) | Returns `false`. | Won't fix | Comment is explicit: Unfinity is out of tournament scope. Leave documented; revisit if Unfinity ever enters. |
| S9 | `resolve_helpers.go:2806` — `case "venture":` | `kaResolved = false`, falls through. | Won't fix | Requires modeling dungeon state. Logged as a known gap; no Commander corpus impact today. |

### Stale `MVP`/`stub` comments where the code is actually implemented

These are comment debt, not behavior bugs. Worth cleaning up in a separate
pass; out of scope for this batch.

- `resolve.go:2424` — `// Replaces the old Phase 3 MVP "mark top as countered" stub` — describes the historical state; current handler is full filter-aware counterspell resolution.
- `resolve_helpers.go:3245` — `discover` MVP comment, but the handler below it (3245–3296) implements the full exile-cast/bottom-shuffle loop.
- `resolve_helpers.go:4055` — `reveal_effect` comment but the case dispatches to `&gameast.Reveal{}`.
- `keywords_station.go:159` — TODO about Aetherdrift cards; actual dispatch surface is intentionally in `per_card` per architecture.
- `keywords_p1p2.go:310` — Just a removal note for a defunct hook.
- `keywords_tempting_offer.go:157` — Describes a thin shim card type, not a behavior gap.

## Inline fixes shipped this PR (top 5)

| # | What changed | File |
|---|---|---|
| S1 | `case "gift":` now walks `e.Args` for any `gameast.Effect` and dispatches each via `ResolveEffect`, mirroring `choose_one_of_them`. | `resolve_helpers.go` |
| S2 | `keyword_action`'s `case "populate":` now delegates to the canonical `case "populate":` ModKind resolver (which already mints a strongest-token copy with the full ETB cascade). The orphaned `kaResolved = false` branch was the only path that left populate observable as a no-op. | `resolve_helpers.go` |
| S3 | `keyword_action`'s `case "explore":` now calls `PerformExplore` (reveal top → land-to-hand or `+1/+1` counter on source). The previous `gs.drawOne` was producing a wrong observable that broke proliferate/explore archetypes. | `resolve_helpers.go` |
| S4 | `keyword_action`'s `case "proliferate":` now delegates to the canonical `case "proliferate":` ModKind resolver (full GreedyHat policy across every counter kind on every eligible perm + the four player-counter pools). The previous inline path only touched friendly creatures with `+1/+1` counters. | `resolve_helpers.go` |
| S5 | `case "reorder_top_of_library":` now reads N from `e.Args[0]` if present, capped at library size; falls back to 3 only when no N is encoded. | `resolve_helpers.go` |

## Deferred (logged but not fixed)

- **S6** `move` counters — needs `gameast.CounterMod` to carry a second target slot. Filed as parser-team work.
- **S7** stale `stat_modification` comment — pure comment cleanup.
- **S8** Unfinity Space Sculptor — intentional out-of-scope.
- **S9** Venture/Dungeon — intentional out-of-scope until dungeon state lands.
- **Stale MVP comments** — cleanup pass, not behavior.
