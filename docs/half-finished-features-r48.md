# Half-Finished Feature Hunt (R48)

**Date:** 2026-05-20
**Branch:** `dev/half-finished-r48`
**Scope:** repo-wide scan for features that are *shaped* but missing parts —
structs with unused fields, interfaces with one impl and a documented
"not yet wired" extension, feature flags that are always off, audit docs
with deferred bullets that never landed, frontend screens whose JSX is
local-only/skeleton, generated stubs with effect wired but cost not
gated.

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
| 4 | `internal/gameengine/hat.go:379-386` defines a `WardPayer` interface with one method. The doc comment ends with: *"non-generic ward (Ward—Pay life, Ward—Sacrifice) is not yet wired at the engine level so wardCost is always >= 0 here."* | Ward—Pay life / Ward—Sacrifice / Ward—Discard variants exist on real cards (Sauron, Dark Lord; Saruman of Many Colors; Auntie Ool). All three have per_card `emitPartial(..., "ward_..._alt_payment_unimplemented")` entries. The engine never threads non-generic ward through the cost-pay pipeline. |
| 5 | `internal/game/combat.go:318-323` — `creaturePower` / `creatureToughness` return **1** for every creature regardless of card data. | The live-game MVP engine (powering hexdek-server's WebSocket `/game/test/*` endpoints) treats every creature as a 1/1 in combat. The `internal/game/types.go:47-65` `Card` struct has no `Power`/`Toughness` fields, so the stub is forced. Fixing this is a 3-step migration (add fields → thread through CreateGameCard → load from Moxfield deck JSON). Flagged in r47, deferred. |
| 6 | `internal/game/engine.go:257-261` — `DrawCards` silently clamps `n` to library size when the library is short. | CR §119.5 says drawing from an empty library doesn't fail per se but flags the seat for elimination at the next SBA check. The live-game MVP `Player` struct has no `AttemptedEmptyDraw` field, and `CheckGameEnd` (`internal/game/combat.go:296-302`) has an explicit dead branch (`if libCount == 0 {}`) where the elimination logic was supposed to go. |
| 7 | `internal/gameengine/per_card/gen_tasigur_the_golden_fang.go` activates `{2}{G/U}{G/U}` and mills 2 cards. | The ability text reads: *"Mill two cards, **then return a nonland card of an opponent's choice from your graveyard to your hand**."* The return-from-graveyard half (the entire reason to play Tasigur) is unimplemented. The delve cost reduction (exile cards from your graveyard to pay generic mana) is also not modeled. |
| 8 | `hexdek/src/screens/Profile.jsx` — Display Name / Owner Name inputs persist via `localStorage.setItem(...)` and the page explicitly tells users *"Preferences are stored locally in your browser only."* | The rest of the platform has a real auth model (`AuthContext`, `/api/me`, server-side device/deck ownership). The profile preferences ride on top of localStorage with no backend sync, no cross-device portability, and no schema. Either commit to the local-only design (and remove the "owner name" field, which is load-bearing for deck visibility) or wire it through `/api/me`. |
| 9 | `internal/anticheat/scheduler.go` / `cauterize.go` / `worker.go` Phase-2 anti-cheat lists **3 follow-ups** (M1 party-code collision via SQLite errno, M2 `errors.Is(err, sql.ErrNoRows)` migration across 4 sites, M3 `RowsAffected` swallow in `db/party.go:186`) in `docs/stub-hunt-rules-db-r47.md`. | All three are documented "Deferred (not in this PR)" — the audit shipped without them. They're trivial drive-bys waiting for a janitor PR. |
| 10 | `internal/auth/middleware.go:47,60` returns 401 responses with `"unauthorized: "+err.Error()` — sentinel-friendly errors leak straight through. | `docs/stub-hunt-remaining-internal-r47.md` finding #6 (MED-severity) flagged this as an info-leak risk for wrapped DB errors (`validate session: <SQL driver message>`) being echoed back. Deferred from r47. |

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

## Deferred / not in this PR

- #4 WardPayer non-generic ward — touches the cost-pay pipeline and 3+ per_card handlers; bigger than an inline fix.
- #5 live-game MVP combat P/T — 3-step migration (struct fields → storage → deck JSON).
- #6 DrawCards empty-library flag — needs new Player field + CheckGameEnd wiring + DB migration.
- #7 Tasigur recursion + delve — needs graveyard-cost framework support.
- #8 Profile.jsx backend sync — needs `/api/me` preferences endpoint + schema decision.
- #9 r47 M1/M2/M3 janitor sweep — folding into a future cleanup PR.
- #10 auth middleware sentinel-aware 401 — security review on the exact response shape recommended before shipping.
