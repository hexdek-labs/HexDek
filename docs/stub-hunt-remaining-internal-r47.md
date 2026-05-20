# Stub Hunt — Remaining internal/ Packages (R47)

Date: 2026-05-20
Branch: `dev/stub-hunt-remaining-r47`
Scope: `internal/auth/`, `internal/deckid/`, `internal/paritycheck/`,
`internal/game/`, `internal/mana/`, `internal/shuffle/`, `internal/trueskill/`,
`internal/ai/` — ~3.6 KLOC of non-test Go across 19 files.

Note: `internal/game/` is the MVP "live multiplayer" runtime that backs
hexdek-server's WebSocket play, NOT the full `internal/gameengine` rules
engine. Several findings call out as "MVP simplification" rather than bugs —
those are flagged here but most are not in scope for r47 fixes.

## Methodology

1. `grep -rn` for `TODO`, `FIXME`, `XXX`, `HACK`, `stub`, `not yet`,
   `coming soon`, `placeholder`, `WIP`, `unimplemented`, `not implemented`.
2. Soft-stub sweep for `simplified`, `naive`, `approximat*`, `for now`,
   `currently`, `temporary`, `hacky`, `hard-coded`, `MVP`.
3. Discarded-error sweep (`_ = `, `_, _ = `) across non-test files.
4. Read of every file under 1 KLOC plus targeted reads in
   `internal/game/{combat,engine}.go`.

## Severity legend

- **HIGH** — load-bearing functionality is stubbed; users get visibly wrong
  results.
- **MED** — partial / lenient implementation that can mask real bugs or
  blocks valid inputs.
- **LOW** — documented limitation or minor smell.
- **INFO** — by-design choice worth knowing about.

---

## Findings

### 1. `internal/game/combat.go` — `creaturePower` / `creatureToughness` hardcoded to 1

- **Path:** `internal/game/combat.go:318-327`
- **Pattern:** explicit stub (`// For MVP we use a stub: return 1 …`)
- **Severity:** **HIGH** (for the live-game MVP)
- **What:** every creature is a 1/1 in combat regardless of card data. The
  `Card` struct has no `Power`/`Toughness` fields (see
  `internal/game/types.go:47-61`), so the stub is forced. Comment justifies
  it for Yuriko ninja-tribal play where combat damage is irrelevant
  vs. the reveal trigger.
- **Impact:** combat math is wrong for every non-1/1 creature. A 6/6
  Reanimated Griselbrand kills exactly one 0/1 token blocker (well, no — it
  kills one because `attackerPower(=1) >= bt(=1)`) and dies if `1` 1/1
  blocker exists. This is the load-bearing finding of this audit but
  fixing it requires (a) adding `Power,Toughness int` to `Card`, (b)
  threading them through `CreateGameCard`/`ListCardsInZone` storage, and
  (c) loading P/T from the Moxfield deck JSON. Out of scope for an inline
  r47 fix; logged for a dedicated PR.

### 2. `internal/deckid/hash.go` — `CardDelta` collapses duplicates

- **Path:** `internal/deckid/hash.go:68-90`
- **Pattern:** silent correctness bug
- **Severity:** **MED-HIGH** (TrueSkill rating inheritance)
- **What:** `CardDelta` builds `map[string]bool` sets from the input lists.
  Both `ComputeHash` and `CardList` emit one `"1x <name>"` line per copy
  (so the joined hash payload preserves quantity), but `CardDelta`
  collapses them to a set — two decks differing only in basic-land counts
  (30 Plains vs 35 Plains) compute delta = 0.
- **Impact:** `trueskill.InheritRating(parent, cardDelta)` inflates sigma
  proportional to `cardDelta`. When delta is undercounted, the child
  inherits too much confidence from the parent. The bug is most acute for
  basics tuning (the one place Commander allows duplicates) but also
  affects any deck representation that ever has duplicate entries.
- **Fix in this PR:** count-aware delta via `map[string]int`; sum of
  positive differences across both directions.

### 3. `internal/mana/parser.go` — hybrid / phyrexian / snow symbols rejected

- **Path:** `internal/mana/parser.go:118-124`
- **Pattern:** explicit `not yet implemented` error
- **Severity:** **MED**
- **What:** `applyToken` returns
  `unsupported mana symbol "X" (hybrid/phyrexian/snow not yet implemented)`
  for anything that's not generic / `WUBRGCX`. Any card with `{W/U}`,
  `{2/W}`, `{W/P}`, or `{S}` in its mana cost fails `mana.Parse`, which in
  the AI autopilot (`internal/ai/autopilot.go:162-164`) silently `continue`s
  past the spell. Cards like Boros Reckoner ({R/W}{R/W}{R/W}),
  Birthing Pod ({3}{G/P}), Arcum's Astrolabe ({S}), and any of the ~140
  snow lands become uncastable.
- **Fix in this PR:** accept `{S}` as generic mana (the simplest faithful
  treatment for an engine that doesn't model the snow supertype on land
  sources); hybrid / phyrexian still rejected with the same error
  (separate scope — needs Pool.CanPay logic to handle "either color
  satisfies this pip").

### 4. `internal/ai/autopilot.go` — `var _ = db.Now` dead reference

- **Path:** `internal/ai/autopilot.go:15, 234`
- **Pattern:** unused-import-keeper hack
- **Severity:** **LOW** (cleanup)
- **What:** the `db` import is held alive solely by `var _ = db.Now` at
  the bottom of the file. Nothing else in the file references the `db`
  package. Either dead from a removed-but-not-cleaned refactor, or a
  scaffold for future code that never landed.
- **Fix in this PR:** drop the dead reference and remove the unused
  import.

### 5. `internal/paritycheck/paritycheck.go` — oracle supplement error swallowed

- **Path:** `internal/paritycheck/paritycheck.go:192`
- **Pattern:** `_ = meta.SupplementWithOracleJSON(cfg.OraclePath)` — error
  discarded silently
- **Severity:** **MED**
- **What:** when the parity report can't load `oracle-cards.json` (wrong
  path, malformed JSON, permission error), the comparison runs without
  P/T supplements and the divergence report quietly misses fidelity. The
  earlier AST/meta loads at lines 184–190 do surface errors; this one
  doesn't, for no obvious reason.
- **Fix in this PR:** log a warning with the path so operators see the
  degradation; keep going (matches the existing
  `pythonAvailable` graceful-degradation pattern at lines 204–209).

### 6. `internal/auth/middleware.go` — raw error string leaks to 401 body

- **Path:** `internal/auth/middleware.go:47, 60`
- **Pattern:** `http.Error(w, "unauthorized: "+err.Error(), 401)`
- **Severity:** **LOW-MED** (information leak)
- **What:** the 401 response body echoes `err.Error()`. For sentinel
  errors (`invalid or unknown session token`, `session token expired`)
  this is harmless. For wrapped DB errors (e.g.,
  `validate session: <SQL driver message>`), an attacker probing
  endpoints with bad tokens gets information about the persistence
  layer. Best practice is to return a generic 401 body and log details
  server-side.
- **Fix in this PR:** distinguish sentinels (`ErrInvalidToken`,
  `ErrSessionExpired`) from internal errors — return the friendly
  reason for sentinels, log + return generic "unauthorized" for the
  rest.

### 7. `internal/game/engine.go` — DrawCards silently clamps on empty library

- **Path:** `internal/game/engine.go:257-261`
- **Pattern:** silent partial-success (`for MVP we just draw what's
  available`)
- **Severity:** **MED**
- **What:** if the caller requests 3 cards but only 1 remains, `DrawCards`
  draws 1 and returns nil error. The caller has to count the returned
  slice length to detect the empty-library state. CR 119.5 says an attempt
  to draw from empty library doesn't itself fail — it sets a flag that
  makes the next SBA check eliminate the player. The MVP doesn't model
  that flag, and the silent clamp makes the empty state hard to detect.
- **Not fixing in this PR:** the proper fix needs a per-player
  `attempted_draw_from_empty` flag wired into `CheckGameEnd`. Logged for
  the same dedicated PR as #1.

### 8. `internal/game/combat.go` — `ResolveCombat` blocker damage spillover stubbed

- **Path:** `internal/game/combat.go:247-254`
- **Pattern:** MVP simplification
- **Severity:** **MED** (when combined with #1, becomes LOW-impact since
  every creature is a 1/1 anyway)
- **What:** comment says "remaining damage spills to next blocker. For
  MVP simplification, just check if attacker.power >= blocker.toughness
  for each." With #1 in place, this is moot. Once P/T is real, blocker
  damage assignment order (per CR 510.1c) needs to be implemented.

### 9. `internal/game/combat.go` — `CheckGameEnd` library-zero is intentionally inert

- **Path:** `internal/game/combat.go:296-302`
- **Pattern:** dead branch with explanatory comment
- **Severity:** **INFO** (documented design choice)
- **What:** `if libCount == 0 {}` runs only the comment block — keeps the
  seat in `aliveList`. This is the correct CR §704.5b behavior in
  isolation (library 0 is not itself elimination), but combined with #7
  it means players can never be eliminated by mill in the MVP engine.

### 10. `internal/auth/session.go` — `last_used_at` UPDATE error swallowed

- **Path:** `internal/auth/session.go:75`
- **Pattern:** documented best-effort
- **Severity:** **INFO**
- **What:** `_, _ = database.ExecContext(...)` is annotated
  `// touch last_used_at (best-effort, not transactional)`. Acceptable
  given the read-then-write pattern; left as-is.

### 11. `internal/game/{combat,engine}.go` — broad `_ = ` discards on logging / cleanup

- **Path:** ~20 call sites in `combat.go` (lines 91, 128, 143, 183, 202,
  215, 251, 258, 264-265, 268, 271) and `engine.go` (158, 210, 230, 286,
  348, 352, 430, 515, 551, 595-602, 633-634, 644)
- **Severity:** **INFO**
- **What:** most are `_ = AppendActionLog(...)` (best-effort audit log)
  and `_ = MoveCard(...)` inside cleanup loops. Action-log discards are
  defensible — log writes shouldn't fail a state mutation. The MoveCard
  ones (combat.go:251, 258) are more dubious — a failed move-to-graveyard
  during damage resolution leaves the game in an inconsistent state. Not
  fixed in this PR; logged for the same dedicated MVP-engine cleanup PR.

---

## Top-5 fixes landing in this PR

1. **#2** `internal/deckid` — count-aware `CardDelta`.
2. **#3** `internal/mana` — accept `{S}` (snow) as generic mana.
3. **#4** `internal/ai/autopilot.go` — drop the dead `var _ = db.Now`.
4. **#5** `internal/paritycheck` — log warning on oracle-supplement
   failure instead of silent swallow.
5. **#6** `internal/auth/middleware.go` — distinguish sentinel auth
   errors from internal ones; return generic 401 body for the latter.

## Follow-up (not in this PR)

- **MVP game P/T scope (#1, #7, #8, #9, #11)** — single dedicated PR to
  add `Power, Toughness int` to `game.Card`, thread them through storage,
  load from Moxfield JSON, implement proper combat damage assignment,
  wire `DrawCards` empty-library detection into `CheckGameEnd`, and
  audit the `MoveCard`-discard call sites for the ones that matter.
- **Mana hybrid / phyrexian** — extend `mana.Cost` + `Pool.CanPay` to
  represent "this pip is satisfied by W OR U" pip requirements. Phyrexian
  also needs the "pay 2 life instead" branch wired into the cast path.
