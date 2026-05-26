# Cleanup — Removed-Residue Sweep (R60 Versailles Phase 2D)

**Date:** 2026-05-25
**Branch:** `dev/cleanup-removed-residue-r60`
**Scope:** repo-wide grep for `// removed` comments, `_ = unused` shims,
half-finished commit residue, `_var = unused` patterns, and stubbed types
with no callers. Delete completely where confirmed dead.

## Method

1. Grep for `^\s*//\s*(removed|deleted|obsolete|deprecated)\b` in `*.go`.
2. Grep for `^\s*_\s*=\s*<identifier>$` in `*.go` (199 hits).
3. Cross-reference `docs/half-finished-features-r48.md` for items still
   flagged that have since been completed.
4. Confirm each candidate is genuinely dead (no callers, no side-effect
   reliance) before deleting. `_ = makePerm(...)` and `_ = f.Close()` are
   intentional side-effect-only calls — left alone.

## Category 1 — Trailing dead `// removed` block

Single removal, biggest single payoff.

| File | Lines | Note |
|---|---|---|
| `internal/gameengine/per_card/percard_stub_batch_r43_test.go` | 362-374 | 13-line header + commentary for "TestIroh_* legacy assertions deleted in R60" with **no test body below**. The commentary documented a long-completed refactor; replacement test coverage already lives in `iroh_grand_lotus_test.go`. Pure git-history material. |

## Category 2 — `// removed` inline migration commentary

| File | Lines | Note |
|---|---|---|
| `internal/gameengine/keywords_p1p2.go` | 306-312 → 306-309 | Collapsed the 7-line "the earlier AdditionalCost-style stub … has been removed; the canonical hook is …" block to a 4-line forward-pointer ("Buyback lives in keywords_buyback.go … canonical hook is ShouldReturnToHandOnResolve"). The removed-symbol enumeration belongs in git history. |

## Category 3 — `_ = unused` shims around dead local assignments

These were `X := compute(...)` lines where `X` was never read, with a
trailing `_ = X` added to silence the Go compiler. Each is a half-finished
intent — the author allocated a variable name but never wired it through.
Replaced with either:
- `_ = compute(...)` when only the side effect matters, OR
- Removed entirely when `compute(...)` itself was pure.

| File | Symbol | Resolution |
|---|---|---|
| `cmd/hexdek-thor/spell_resolve.go:68-76` | `var eff interface{}` + AST loop + `_ = eff` (9-line dead block) | Whole block deleted. `eff` was never inserted into the `StackItem`. |
| `cmd/hexdek-thor/oracle_diff.go:90,100,198` | `var hasTrigger bool` + assignment + `_ = hasTrigger` | Entire variable removed (assignment inside the type-switch deleted too). |
| `cmd/hexdek-thor/oracle_diff.go:199` | `_ = hasActivated` | Silencer deleted. `hasActivated` is genuinely used at the activated-ability check below. |
| `cmd/hexdek-thor/oracle_compliance.go:311,413` | `expectedEffects := inferExpectedEffects(...)` + `_ = expectedEffects` | Variable deleted. Cascading: `inferExpectedEffects` had no other callers, so the function (33 lines) and its 20 supporting regex `MustCompile` declarations (~22 lines) were deleted too. **Net -55 lines.** |
| `cmd/hexdek-freya/analysis.go:2211-2212` | `ot := strings.ToLower(p.Name)` + `_ = ot` (inside finisher loop) | Both lines deleted. `ot` was never referenced. |
| `cmd/hexdek-thor/combo_demo.go:110,128,164,168` | `oracle :=`, `artist :=`, `big :=`, `small :=` followed by `_ = ...` | `_ = X` silencers deleted; `oracle`/`big`/`small` are actually used by later lines, `artist` collapsed to `_ = addDemoPerm(...)`. |
| `cmd/hexdek-thor/deep_rules.go:594,691,1294,2734` | 4 sites of `X := makePerm(...) / _ = X` or `_ = creature` after the var was used | 2 collapsed to `_ = makePerm(...)` form; 1 to `_ = makeToken(...)`; 1 standalone silencer deleted (the var was already used elsewhere). |

## Category 4 — Untouched on purpose

Genuine side-effect-only `_ = X` shims kept as-is:

- `_ = f.Close()` / `_ = os.Remove(tmp)` in `cmd/hexdek-oracle-sync/main.go` and similar — Go's "I know I'm dropping an error here, mark it deliberately" pattern.
- `_ = makePerm(gs, ...)` / `_ = makeToken(gs, ...)` in goldilocks scaffold tests where the return is irrelevant but the function appends to the battlefield. ~80 instances across `cmd/hexdek-thor/{advanced_mechanics,deep_rules,claim_verifier,negative_legality,chaos_games,multiplayer_chaos,clock_pressure}.go` — these are scaffold idioms, not residue.
- `// DEPRECATED` markers on `Seat.SpellsCastThisTurn` and `Seat.DescendedThisTurn` in `internal/gameengine/state.go:860,875` — 88 active call sites still reference them, so the deprecation tags are still load-bearing migration markers (intent: drain callers first, then delete). Left alone for a future migration sweep.
- `// stub` comments on `sba704_5h / _5t / _5u / _5w / _5x / _5z` in `internal/gameengine/sba.go` — these are real placeholder hooks for mechanics not yet modeled (deathtouch tracking, dungeons, space sculptor, battle protectors, siege protector reset, Speed). The hooks fire at the right SBA points; the bodies are intentional no-ops mirroring the Python stub set. Removing them would lose the SBA registration callsite.
- `// stub` annotations on per_card custom_*.go handlers (Yenna, Quandrix, Araumi, Inalla, Kalamax, Asmoranomardicadaistinaculdacar) — these all reference the gen_*.go scaffold they replaced. Comments document why the custom handler exists alongside the auto-generated stub; useful orientation, not residue.

## Category 5 — Cross-reference: `docs/half-finished-features-r48.md`

The r48 audit listed 10 top items. Status update as of 2026-05-25:

| # | Item | r48 status | Now | Note |
|---|---|---|---|---|
| 1 | `trigger_resolve` event distinction | shipped inline in r48 | **still shipped** | `internal/gameengine/stack.go:1050` emits `trigger_resolve` for triggered-ability resolve. |
| 2 | Captain America `{3}` cost gate | shipped inline in r48 | **still shipped** | `gen_captain_america_first_avenger.go:82` uses `PayGenericCost` + `emitFail("insufficient_mana")`. |
| 3 | `paritycheck` `SupplementWithOracleJSON` error log | inline-collapsed during rebase to no-op | **shipped** (via main's concurrent worker) | The audit row in r48 doc still documents the historical half-finished shape; the actual fix lives in main. |
| 4 | `WardPayer` non-generic ward (Pay life / Sacrifice / Discard) | deferred | **still deferred** | Per-card `ward_*_alt_payment_unimplemented` partials still present in `auntie_ool.go`, `saruman_of_many_colors.go`, `sauron_dark_lord.go`. |
| 5 | Live-game MVP combat P/T (`internal/game/combat.go`) | deferred | **still deferred** | `creaturePower` / `creatureToughness` still return 1 unconditionally. |
| 6 | `DrawCards` empty-library SBA wiring | deferred | **still deferred** | `internal/game/engine.go:257-261` still silently clamps; `combat.go:298-302` still has the dead `if libCount == 0 {}` branch. |
| 7 | Tasigur graveyard-return + delve | deferred | **still deferred** | `gen_tasigur_the_golden_fang.go` mills 2 but doesn't return-from-graveyard; delve unmodeled. |
| 8 | Profile.jsx → `/api/me` backend sync | deferred | **still deferred** | Still localStorage-only. |
| 9 | r47 M1/M2/M3 janitor sweep (party-code collision, ErrNoRows, RowsAffected) | deferred | **partially closed** | `internal/db/party.go:202` now uses `RowsAffected()` (`n, _ := res.RowsAffected(); if n == 0 { return ... }`) — value used; error still swallowed by `_`. M1 + M2 status unverified by this sweep. |
| 10 | Auth middleware sentinel-aware 401 messages | deferred | **shipped** | `internal/auth/middleware.go` now has `authErrorMessage(err error)` that maps `ErrInvalidToken` / `ErrSessionExpired` / `ErrInvalidAPIKey` family / default → safe client message + server-side log. Info-leak risk closed. |

**Net change since r48:** #10 closed end-to-end (sentinel-aware 401); #9 M3 half-closed (value used, error still discarded). Items #4-#8 remain deferred and continue to be tracked.

## Verification

- `go build ./...` clean.
- `go test ./cmd/hexdek-thor/... ./internal/gameengine/... ./cmd/hexdek-freya/...` clean (3.176s, no regressions).

## Net diff

8 files changed, **+8 / -107 lines**:

- `cmd/hexdek-freya/analysis.go` — -2
- `cmd/hexdek-thor/combo_demo.go` — -3
- `cmd/hexdek-thor/deep_rules.go` — -5
- `cmd/hexdek-thor/oracle_compliance.go` — -55 (includes function + 20-regex block)
- `cmd/hexdek-thor/oracle_diff.go` — -4
- `cmd/hexdek-thor/spell_resolve.go` — -10
- `internal/gameengine/keywords_p1p2.go` — net -3 (collapsed commentary)
- `internal/gameengine/per_card/percard_stub_batch_r43_test.go` — -13
