# Stub Hunt — cmd/ Tool Suite (R46)

Date: 2026-05-20
Branch: `dev/stub-hunt-tools-r46`
Scope: every `cmd/*/` entry point — 24 binaries totaling ~17 KLOC of `main.go` +
helpers (excluding the per-card generator's tooling, which is by-design partial).

## Methodology

1. `grep -rn -E "(TODO|FIXME|XXX|HACK|stub|not yet|coming soon|placeholder|WIP|unimplemented)"` over `cmd/`.
2. `grep` for `panic("not …")` / `return nil // …` / `_ = …` patterns.
3. Per-tool `flag.{String,Bool,Int,Duration,Float}` audit: declare-vs-reference
   count (only one tool had a declared-but-unread flag — see #1 below).
4. Manual read of every tool's `main.go` that's under 1 KLOC, plus targeted
   sweeps in the larger Thor / Freya / Loki tools.
5. Cross-checked against the existing `gen-handlers` `emitPartial` markers
   (already documented as by-design — see "Not a stub" section).

## Severity legend

- **HIGH** — feature is advertised (flag, docstring) but does nothing.
- **MED** — partial / lenient implementation that can mask real bugs or panic on
  empty input.
- **LOW** — documented limitation or unreachable code path; cosmetic.
- **INFO** — by-design partial markers worth knowing about, not bugs.

---

## Findings

### 1. `hexdek-artfetch`: `--workers` flag is dead — fetch is sequential

- **Path:** `cmd/hexdek-artfetch/main.go:36`
- **Pattern:** unused CLI flag (category 5)
- **Severity:** **HIGH**
- **What:** `workers = flag.Int("workers", 1, …)` is declared at module scope and
  parsed, but `*workers` is never read anywhere in the file. The Scryfall fetch
  loop at `main.go:94` (`for i, name := range uncached { fetchArt(...) }`) is
  fully sequential. The flag is documented in the package preamble.
- **Impact:** users passing `-workers=4` get the same single-threaded behavior;
  this is the kind of silent no-op that erodes trust. Since Scryfall asks for
  50–100 ms between requests anyway, the right move is either (a) wire a real
  worker pool with rate-limit-aware throttling, or (b) drop the flag.
- **Fix in this PR:** wire a worker pool with a shared rate-limit ticker so the
  flag actually does what it says, defaulting to 1.

### 2. `hexdek-server`: `handleTopN` panics on empty library

- **Path:** `cmd/hexdek-server/main.go:286`
- **Pattern:** missing bounds check before zero-indexed read (category 4 sibling)
- **Severity:** **MED**
- **What:** the JSON response includes `"yuriko_damage_if_top_revealed":
  s.library[0].CMC`, which dereferences index 0 unconditionally. The guard above
  it only clamps `n > len(s.library)` to `len(s.library)` — when the library is
  empty, `n` is clamped to 0 but `s.library[0]` is still read.
- **Impact:** any request to `/game/test/library/top/N` against an
  empty/uninitialized deck panics. Reachable today only if the deck file loads
  with zero mainboard cards (theoretical), but cheap to harden.
- **Fix in this PR:** guard the index, emit 0 / omit the field when empty.

### 3. `hexdek-server`: `init()` sanity check is silently wrong-way-round

- **Path:** `cmd/hexdek-server/main.go:348-352`
- **Pattern:** dead diagnostic + flag bypassed (category 9)
- **Severity:** **LOW–MED**
- **What:** the package `init()` does
  `os.Stat("data/decks/yuriko_v1.json"); err != nil && !os.IsNotExist(err)` —
  it warns only on errors *other than* "file missing", which is the one case
  operators actually care about. It also hardcodes the path rather than reading
  the `--deck` flag (which `init()` can't, because flags haven't parsed yet).
- **Impact:** the sanity check never fires for the case it's named after, and is
  silent when the path is changed via flag.
- **Fix in this PR:** move the check into `main()` after `flag.Parse()`, stat
  `*deckPath`, and warn on the missing-file case (since `LoadDeckFromFile`
  fatals on it a few lines later, we just promote it to a one-line operator
  hint).

### 4. `dump_drift`: file/decode errors silently dropped on startup

- **Path:** `cmd/dump_drift/main.go:17-31`
- **Pattern:** errors discarded with `_` (category 7 sibling)
- **Severity:** **MED**
- **What:** `corpus, _ := astload.Load(...)` then `f, _ := os.Open(...)` then
  `json.NewDecoder(f).Decode(...)`. If the AST corpus fails to load, `corpus`
  is nil and the first `corpus.Get(e.Name)` call panics. If the oracle file is
  missing, `f` is nil and `Decode` panics.
- **Impact:** developer-facing tool but the nil-deref panic is unfriendly. A
  `log.Fatalf` with the path makes the failure mode immediately obvious.
- **Fix in this PR:** check both errors and `log.Fatalf` with the path.

### 5. `reset-elo`: all DB errors silently dropped

- **Path:** `cmd/reset-elo/main.go:36-56`
- **Pattern:** errors discarded (category 7 sibling)
- **Severity:** **MED**
- **What:** every `db.QueryRow().Scan(&cnt)` ignores the error, so missing tables
  silently report 0 rows. Every `db.Exec("DELETE …")` ignores the error, so a
  failed delete still prints "ELO reset complete".
- **Impact:** a corrupted/migrated DB looks like a no-op success. For a tool
  that *is* a destructive operation, that's the wrong default.
- **Fix in this PR:** check Scan errors (downgrade table-missing to a warning
  like the existing `card_stats` branch), and check Exec errors as fatals.

### 6. `hexdek-thor` coverage_depth: `stubEffects` map is empty — STUB_EFFECT bucket unreachable

- **Path:** `cmd/hexdek-thor/coverage_depth.go:107`
- **Pattern:** empty placeholder data structure (category 4)
- **Severity:** **LOW**
- **What:** `var stubEffects = map[string]bool{}` is referenced at line 330 in
  `classifyEffect`, but the map has zero entries. Effects either hit
  `dispatchedEffects` → COVERED, or fall through → PARSED_NO_DISPATCH.
  STUB_EFFECT is only reachable via `classifyModKind` for unknown mod-kinds.
- **Impact:** the audit's STUB_EFFECT category exists in the data model but
  never gets populated for top-level effects. Not a runtime bug; just a
  half-built feature that should either get its dictionary populated or get
  collapsed into PARSED_NO_DISPATCH.
- **Not fixing in this PR:** populating the dictionary requires knowing which
  effects the engine logs-but-doesn't-resolve, which is a separate audit. Logged
  for follow-up.

### 7. `hexdek-thor` corpus_audit: actors silently accepted without dispatch

- **Path:** `cmd/hexdek-thor/corpus_audit.go:813-815`
- **Pattern:** lenient verifier — `return "" // actor not yet handled by engine`
- **Severity:** **LOW**
- **What:** the Sacrifice effect verifier short-circuits with success when the
  actor is `defending_player_choice` or `each_other_player`, because those
  actors aren't dispatched by the engine yet. The comment is honest, but it
  means the audit pretends to cover effects it isn't actually verifying.
- **Impact:** audit completeness gap, not a correctness bug.
- **Not fixing in this PR:** real fix is in the engine's sacrifice dispatch.

### 8. `hexdek-thor` conditional_setup: mutate scaffold logs `stub:true` when srcPerm is nil

- **Path:** `cmd/hexdek-thor/conditional_setup.go:2645-2654`
- **Pattern:** half-wired scaffold (category 9)
- **Severity:** **LOW**
- **What:** `condScaffoldMutates` only stamps `Flags["mutated"] = 1` when
  `srcPerm != nil`. The else branch logs a `mutate` event with
  `details: { "stub": true }`, which any downstream consumer can use as a marker
  that the scaffold didn't fully prime. This is the only `"stub": true`
  log-event marker in the tree.
- **Impact:** scaffold completeness; tests that depend on srcPerm presence are
  fine, others see a half-primed state.

### 9. `hexdek-thor` advanced_mechanics: lenient "accept for now" branch

- **Path:** `cmd/hexdek-thor/advanced_mechanics.go:1393-1396`
- **Pattern:** test accepts known-incorrect behavior with comment (category 9)
- **Severity:** **LOW**
- **What:** `Steal_Then_Sacrifice_Goes_To_Owner_GY` returns nil success even
  when the stolen card lands in the *controller's* graveyard — which is
  technically wrong per CR §400.7 (owner-zone), but matches current engine
  behavior. Comment: `// Accept for now — zone_change uses owner.`
- **Impact:** a real bug in the engine's zone-change-after-control-swap would
  be masked by this assertion. Worth converting to a fail or at least an
  invariant violation log once the engine catches up.

### 10. `gen-handlers`: `emitPartial` markers in 5,500+ generated handlers

- **Path:** `cmd/gen-handlers/main.go:1030, 1042, 1214, 1315, 1347`
- **Pattern:** by-design partial code generation (category 9, INFO)
- **Severity:** **INFO**
- **What:** the gen-handlers binary emits `emitPartial(gs, slug, name, "auto-gen:
  ETB effect not parsed from oracle text")`-style markers when it can't parse a
  card's oracle text into structured effects. The resulting generated files in
  `internal/gameengine/per_card/gen_*.go` are valid Go, register their ETB hook,
  and log the partial — they don't fail compile.
- **Impact:** this is the architecture, not a stub. The engine resolves most
  effects via the data-driven AST; per_card handlers exist to register
  trigger/ETB hooks and to allow hand-written upgrades (see
  `internal/gameengine/per_card/custom_*.go`). Tracking is via the
  `oracle_compliance` audit's `partial` counter.

---

## False positives discovered

- `cmd/hexdek-import/main.go:7,15,49` — `XXXXX` strings are docstring deck-ID
  examples, not stubs.
- `cmd/hexdek-contrib/main.go:175-184` — the `stub := &hexapi.ContribResult{…}`
  is a deliberate empty-result envelope sent back to the dispatcher when an
  assignment fails, clearing the in-flight slot. Production-correct; just
  shares the lexeme.
- Various `// no-op` / `// fallthrough` / `// simplified` comments in
  hexdek-thor — every one I read was a documented narrow-scope choice, not an
  unfinished feature.
- Every other tool's `flag.*` declarations are read.

---

## Top-5 fixes landing in this PR

1. **#1 artfetch `--workers`** — wire a real worker pool with shared
   rate-limit ticker.
2. **#2 server `handleTopN`** — bounds-check `s.library[0]`.
3. **#3 server `init()` sanity check** — move into `main()` post-flag-parse,
   warn on missing file (the case operators actually care about).
4. **#4 dump_drift** — check + fatal on AST/oracle load errors.
5. **#5 reset-elo** — check Scan + Exec errors, downgrade missing-table to a
   warning (matching the existing `showmatch_card_stats` branch), fatal on a
   real delete failure.

## Follow-up (not in this PR)

- Populate `coverage_depth.go:stubEffects` once the engine's effect log gives a
  signal for "parsed and logged but not resolved" cases (#6).
- Engine work to dispatch `defending_player_choice` / `each_other_player`
  Sacrifice actors so the corpus audit can stop short-circuiting (#7).
- Engine work on §400.7 owner-zone semantics so `advanced_mechanics.go:1395`
  can flip from lenient to strict (#9).
