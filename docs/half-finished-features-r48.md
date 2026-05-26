# Half-Finished Feature Hunt (R48)

**Date:** 2026-05-20 · **Last reconciled:** 2026-05-26 (R60 close-out pass — second sweep)
**Branch:** `dev/half-finished-r48` (original audit) ·
`dev/half-finished-features-r48-close-r60` (first reconciliation) ·
`dev/half-finished-bisect-remaining-r60` (this sweep)
**Scope:** repo-wide scan for features that are *shaped* but missing parts —
structs with unused fields, interfaces with one impl and a documented
"not yet wired" extension, feature flags that are always off, audit docs
with deferred bullets that never landed, frontend screens whose JSX is
local-only/skeleton, generated stubs with effect wired but cost not
gated.

## R60 reconciliation summary

Bisected every deferred item against current `main`:

| # | Original verdict | Current state |
|---|---|---|
| 1-3 | Shipped in original r48 PR | **Resolved** |
| 4 | WardPayer non-generic ward not wired | **Resolved** — new `internal/gameengine/ward_alt_payment.go` handles `ward_alt_kind` Permanent.Flags via `tryPayAltWardCost` (extends `CheckWardOnTargeting`); Sauron / Saruman / Auntie Ool ETBs stamp the flags and the `*_alt_payment_unimplemented` partials are gone |
| 5 | Live-game MVP combat P/T returns 1 | **Resolved** — `Power int` / `Toughness int` added to `internal/game/types.go::Card` + `internal/moxfield/parser.go::Card`; `creaturePower` / `creatureToughness` now read directly with 1/1 fallback. R60 Versailles; see "Closed since" #5. |
| 6 | DrawCards empty-library flag | **Resolved** — `Player.AttemptedEmptyDraw` field + schema migration; `DrawCards` sets the flag on attempted-draw against empty library (CR §119.5 semantics); `CheckGameEnd` eliminates flagged seats. R60 Versailles; see "Closed since" #6. |
| 7 | Tasigur recursion + delve | **Still open** — `gen_tasigur_the_golden_fang.go:43-48` mills 2 only; no return-from-graveyard, no delve. |
| 8 | Profile.jsx backend sync | **Still open** — `hexdek/src/screens/Profile.jsx:17-23` still localStorage-only; line 75 still says "Preferences are stored locally in your browser only." |
| 9 | r47 M1/M2/M3 janitor sweep | **Partial** — **M2 stealth-resolved** since the previous reconciliation (zero production `err == sql.ErrNoRows` sites; `internal/lint/errors_is_sql_errnorows_test.go` lint test enforces; `internal/db/errnorows_sentinels_r60_test.go` pins the migrated semantics). M1 + M3 still open: `internal/db/party.go:62-63` follow-up comment intact; `internal/db/party.go:202` `n, _ := res.RowsAffected()` swallow unchanged. |
| 10 | Auth middleware 401 sentinel-aware | **Resolved** — `authErrorMessage` now sentinel-checks `ErrInvalidToken` / `ErrSessionExpired` / `ErrInvalid/Expired/RevokedAPIKey`; non-sentinel errors log server-side and surface generic "unauthorized". Landed in R47 stub-hunt commit `17b2cb3`, extended for API-key sentinels in PR #388. |

## Method

Five passes:
1. Read `docs/HexDek TODO Board.md` for explicit `[ ]` items in the
   Open / Tracking sections.
2. `git ls-tree` the `docs/stub-hunt-*.md` series and pull "Deferred"
   sections — each one is a curated list of features the author started
   but didn't ship.
3. Grep for `type * interface` across `internal/`, count concrete
   implementations and look for "OPTIONAL extension" / "not yet wired"
   comments.
4. Grep for `os.Getenv(...) == "1"` / `if false {` / `Enabled bool` /
   "feature flag" patterns to find dormant switches.
5. Read frontend screens under 100 lines for "local-only storage,
   never synced" / "TODO" / "Coming soon" shapes; check generated stubs
   under `internal/gameengine/per_card/` against their custom siblings.

## Top 10 half-finished features

### Stub / shape inventory

| # | What's there | What's missing |
|---|---|---|
| 1 | `internal/gameengine/stack_trace.go:20` documents **6 action kinds** (`push`, `resolve`, `priority_pass`, `sba_check`, `trigger_push`, `trigger_resolve`). | Only 5 emit. `trigger_resolve` is documented but no call site ever emits it — `ResolveStackTop` logs `resolve` for both spells and triggered abilities, so CR §608.2 audit reports can't distinguish the two. **Fix #1 below.** |
| 2 | `internal/gameengine/per_card/gen_captain_america_first_avenger.go:73-76` "Throw" activation pays `{3}` via a "defensive cost top-up" (`if seat.ManaPool >= 3 { seat.ManaPool -= 3 }`). | The gate is one-sided: if mana is insufficient the activation proceeds anyway, just without paying. Compare Erebos / Tasigur / Yenna — they all `emitFail("insufficient_mana")` and return. Free `{3}` activation for any caller that bypasses the engine's `ActivateAbility` mana check. **Fix #2 below.** |
| 3 | `internal/paritycheck/paritycheck.go:192` calls `meta.SupplementWithOracleJSON(cfg.OraclePath)` and discards the error with `_ =`. | The two `astload.Load` + `LoadMetaFromJSONL` calls above it surface errors. This one swallows. When the oracle path is wrong or the file is malformed, parity reports run *without P/T supplements* and quietly under-report divergence — defeats the purpose of the parity check. **Fix #3 below.** |
| 4 | ~~`internal/gameengine/hat.go:379-386` defines a `WardPayer` interface with one method. The doc comment ends with: *"non-generic ward (Ward—Pay life, Ward—Sacrifice) is not yet wired at the engine level so wardCost is always >= 0 here."*~~ | ~~Ward—Pay life / Ward—Sacrifice / Ward—Discard variants exist on real cards (Sauron, Dark Lord; Saruman of Many Colors; Auntie Ool). All three have per_card `emitPartial(..., "ward_..._alt_payment_unimplemented")` entries. The engine never threads non-generic ward through the cost-pay pipeline.~~ **RESOLVED in R60.** New `internal/gameengine/ward_alt_payment.go` introduces `WardAltKindSacrificeLegendary` / `WardAltKindDiscardInstSorcEnch` / `WardAltKindBlight`; `CheckWardOnTargeting` reads `perm.Flags["ward_alt_kind"]` + `ward_alt_filter` and dispatches to `tryPayAltWardCost`, which applies the sacrifice / discard / counter-placement or counters the spell per CR §702.21c. Sauron / Saruman / Auntie Ool ETBs now stamp the flags and the 3 `*_alt_payment_unimplemented` partials are removed. WardPayer interface doc-comment refreshed to describe the new alt-payment path. |
| 5 | `internal/game/combat.go:318-323` — `creaturePower` / `creatureToughness` return **1** for every creature regardless of card data. | The live-game MVP engine (powering hexdek-server's WebSocket `/game/test/*` endpoints) treats every creature as a 1/1 in combat. The `internal/game/types.go:47-65` `Card` struct has no `Power`/`Toughness` fields, so the stub is forced. Fixing this is a 3-step migration (add fields → thread through CreateGameCard → load from Moxfield deck JSON). Flagged in r47, deferred. |
| 6 | `internal/game/engine.go:257-261` — `DrawCards` silently clamps `n` to library size when the library is short. | CR §119.5 says drawing from an empty library doesn't fail per se but flags the seat for elimination at the next SBA check. The live-game MVP `Player` struct has no `AttemptedEmptyDraw` field, and `CheckGameEnd` (`internal/game/combat.go:296-302`) has an explicit dead branch (`if libCount == 0 {}`) where the elimination logic was supposed to go. |
| 7 | `internal/gameengine/per_card/gen_tasigur_the_golden_fang.go` activates `{2}{G/U}{G/U}` and mills 2 cards. | The ability text reads: *"Mill two cards, **then return a nonland card of an opponent's choice from your graveyard to your hand**."* The return-from-graveyard half (the entire reason to play Tasigur) is unimplemented. The delve cost reduction (exile cards from your graveyard to pay generic mana) is also not modeled. |
| 8 | `hexdek/src/screens/Profile.jsx` — Display Name / Owner Name inputs persist via `localStorage.setItem(...)` and the page explicitly tells users *"Preferences are stored locally in your browser only."* | The rest of the platform has a real auth model (`AuthContext`, `/api/me`, server-side device/deck ownership). The profile preferences ride on top of localStorage with no backend sync, no cross-device portability, and no schema. Either commit to the local-only design (and remove the "owner name" field, which is load-bearing for deck visibility) or wire it through `/api/me`. |
| 9 | `internal/anticheat/scheduler.go` / `cauterize.go` / `worker.go` Phase-2 anti-cheat lists **3 follow-ups** (M1 party-code collision via SQLite errno, M2 `errors.Is(err, sql.ErrNoRows)` migration across 4 sites, M3 `RowsAffected` swallow in `db/party.go:186`) in `docs/stub-hunt-rules-db-r47.md`. | All three are documented "Deferred (not in this PR)" — the audit shipped without them. They're trivial drive-bys waiting for a janitor PR. |
| 10 | ~~`internal/auth/middleware.go:47,60` returns 401 responses with `"unauthorized: "+err.Error()` — sentinel-friendly errors leak straight through.~~ | ~~`docs/stub-hunt-remaining-internal-r47.md` finding #6 (MED-severity) flagged this as an info-leak risk for wrapped DB errors (`validate session: <SQL driver message>`) being echoed back.~~ **RESOLVED.** New `authErrorMessage` helper sentinel-checks `ErrInvalidToken` / `ErrSessionExpired` / `ErrInvalidAPIKey` / `ErrAPIKeyExpired` / `ErrAPIKeyRevoked` and falls through to `log.Printf` + generic `"unauthorized"` so wrapped DB / driver messages are no longer echoed in the response body. Initial sentinel-check landed in commit `17b2cb3` (R47 stub hunt); API-key sentinel set added in PR #388 (API key system). |

### Honorable mentions (looked into, not in top 10)

- TODO Board's "Open — Era 1 cost-unenforced" list (Giada, Aminatou, Azami, Phenax, Erebos, Bilbo, etc.) — **mostly stale**: spot-checking Erebos, Azami, Phenax, Yenna shows all four ship with proper cost gates today. The board hasn't been reconciled since 2026-05-09. Recommend a reconciliation sweep before treating these as active gaps.
- `internal/heimdall/sinks.go` has three "OPTIONAL extension" sink interfaces (`HuginnZoneCastSink`, `HuginnExileLinkSink`, `MuninnExileLinkSink`). All three have at least one implementation today, so they're not actually half-finished — just over-architected.
- `gameast.CounterMod` "move" op — flagged in r46 as needing a second target slot on the AST. Real but the fix is parser-team work.
- `internal/mana/parser.go` rejects hybrid/phyrexian symbols — r47 fixed snow but left hybrid/phyrexian explicitly out of scope.

## Inline fixes shipped this PR (top 3)

| # | What changed | File |
|---|---|---|
| 1 | `ResolveStackTop` now emits `trigger_resolve` for triggered-ability stack items (`item.Kind == "triggered"` or `item.Source != nil`) and `resolve` only for actual spells. Matches the documented event set in `stack_trace.go`. | `internal/gameengine/stack.go` |
| 2 | Captain America's `Throw` activation now real-gates `{3}`: if `seat.ManaPool < 3` it `emitFail("insufficient_mana")` and returns. The equipment lookup also moves *before* the cost debit so we don't burn mana for an activation we couldn't complete. | `internal/gameengine/per_card/gen_captain_america_first_avenger.go` |
| 3 | `paritycheck.NewRunner` now logs a warning when `SupplementWithOracleJSON` fails (matching the existing `pythonAvailable` graceful-degradation pattern), instead of silently swallowing the error. **Note:** during the rebase onto `main` this turned out to have been concurrently fixed by another worker with essentially the same wording — the rebase resolved by taking `main`'s version. The audit row above still documents the historical half-finished shape; the inline change collapsed to a no-op on merge. | `internal/paritycheck/paritycheck.go` |

## Deferred — still open as of 2026-05-26 R60 reconciliation

- ~~**#4 WardPayer non-generic ward**~~ — **RESOLVED in R60** (see "Closed since the original audit" below).
- ~~**#6 DrawCards empty-library flag**~~ — **RESOLVED in R60** (see "Closed since the original audit" below).
- **#7 Tasigur recursion + delve** — `internal/gameengine/per_card/gen_tasigur_the_golden_fang.go:43-48` still mills 2 cards (verified 2026-05-26: handler body unchanged since original audit). The activation's second clause (*"then return a nonland card of an opponent's choice from your graveyard to your hand"*) is unimplemented; the Delve keyword (exile cards from graveyard to pay for {1}) is also not modeled at the engine level. Needs graveyard-cost framework support + an opponent-choice prompt surface.
- **#8 Profile.jsx backend sync** — `hexdek/src/screens/Profile.jsx:17-23` still uses `localStorage.getItem/setItem` for display name + owner name; line 75 still tells users "Preferences are stored locally in your browser only." (Verified 2026-05-26.) Needs `/api/me` preferences endpoint + schema decision.
- **#9 r47 M1/M2/M3 janitor sweep** (partial — M2 stealth-resolved):
  - **M1** still open: `internal/db/party.go:62-63` carries the explicit comment "Distinguishing constraint vs. transport errors via driver-specific errno is M1 in the audit — follow-up work." (Verified 2026-05-26.)
  - ~~**M2** still open AND has grown: 6 sites still use `err == sql.ErrNoRows`~~ — **STEALTH-RESOLVED.** Bisect on 2026-05-26 reports **zero** production `err == sql.ErrNoRows` sites. `internal/lint/errors_is_sql_errnorows_test.go` is a tree-wide lint test that fails CI on any new `err == sql.ErrNoRows` / `err != sql.ErrNoRows` occurrence. `internal/db/errnorows_sentinels_r60_test.go` pins the migrated semantics at the 4 originally-flagged db-package call sites (`ClaimNextVerification`, `ActiveBan`, etc.) — they now return `(nil, nil)` / `("", nil)` / sentinel-zero values on empty-row instead of leaking `sql.ErrNoRows`. The migration's other consumers (`db/showmatch.go`, `friends/friends.go`, `userprofile/userprofile.go`) are covered by the lint test rather than per-site regression coverage.
  - **M3** still open: `internal/db/party.go:202` keeps `n, _ := res.RowsAffected()` (error swallowed; value used). Verified 2026-05-26: unchanged. Sibling `anticheat/auditor.go:486` does the same shape but checks both — `M3` was specifically the party.go site.

## Closed since the original audit

- **#1-#3** shipped in the original r48 PR (commit `ccb9a9e`).
- **#5 live-game MVP combat P/T** — landed in R60 Versailles. Added `Power int` / `Toughness int` to `internal/moxfield/parser.go::Card` and `internal/game/types.go::Card`; threaded through `marshalCardData` / `hydrateCardData` (storage layer, no schema migration — `card_data` is already a JSON blob); copied at the two `CreateGameCard` call sites in `engine.go` (commander + library); `creaturePower` / `creatureToughness` now read `Card.Power` / `Card.Toughness` directly, with a 1/1 fallback when both are zero (= deck JSON omitted the metadata) to preserve pre-r60 MVP behavior for cards that haven't been re-imported with full P/T. First test coverage for `internal/game/` lands alongside (10 tests in `combat_power_r60_test.go`).
- **#6 DrawCards empty-library flag** — landed in R60 Versailles. The dead branch at `internal/game/combat.go:298` (`if libCount == 0 { /* keep alive */ }`) was **semantically wrong** — having 0 cards is not itself a loss per CR §119.5; only ATTEMPTING to draw from an empty library triggers the loss "the next time a player would receive priority." Fix: added `AttemptedEmptyDraw bool` to `Player` (with schema migration `game_player.attempted_empty_draw INTEGER NOT NULL DEFAULT 0` + updates to `CreateGamePlayer` / `GetGamePlayer` / `ListGamePlayers` / `UpdateGamePlayer`); `DrawCards` sets the flag when called with `n > 0` against an already-empty library (NOT when drawing the last card — the next draw attempt is what triggers); `CheckGameEnd` now eliminates flagged seats instead of running the dead `libCount == 0` check. 7 tests in `draw_empty_library_r60_test.go` pin: empty-draw sets flag; last-card-draw does NOT set flag; second-draw-after-last does set flag; zero-count draw is a no-op; CheckGameEnd eliminates flagged seats; CheckGameEnd no-op when no loss condition; full DrawCards → DrawCards → CheckGameEnd end-to-end flow.
- **#10** auth middleware sentinel-aware 401 — landed across two commits: initial sentinel-check in `17b2cb3` (R47 stub hunt), API-key sentinels added in PR #388. The current `authErrorMessage` matches the audit's recommended shape exactly.
- **#4 WardPayer non-generic ward** — landed in R60. New `internal/gameengine/ward_alt_payment.go` adds `WardAltKindSacrificeLegendary` / `WardAltKindDiscardInstSorcEnch` / `WardAltKindBlight`; `CheckWardOnTargeting` reads `perm.Flags["ward_alt_kind"]` + `ward_alt_filter` and dispatches to `tryPayAltWardCost`. Sauron / Saruman / Auntie Ool ETBs stamp the flags (replacing the prior `*_alt_payment_unimplemented` emitPartials). Engine policy: "pay if affordable" with cheapest-legal-pick heuristics (lowest-power legendary for sacrifice, highest-CMC matching card for discard, lowest-toughness creature for blight); spell is countered per CR §702.21c when no legal payment exists. 7 regression tests in `ward_alt_payment_test.go` cover paid + countered paths for each kind plus a regression guard for the mana-ward fall-through. `WardPayer` interface doc-comment refreshed to describe the new alt-payment path.
