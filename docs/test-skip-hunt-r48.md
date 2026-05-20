# Test-Skip Hunt (R48)

Date: 2026-05-20
Branch: `dev/test-skip-cleanup-r48`
Scope: every `*_test.go` in the tree — t.Skip / b.Skip calls, t.Log("TODO"),
commented-out test cases, `//nolint` suppressions.

## Methodology

1. `grep -rnE "t\.(Skip|SkipNow|Skipf)"` → **57 hits across 14 files**.
2. `grep -rnE "(b|tb)\.Skip"` → 7 additional hits in benchmarks / shared
   helpers (covered by total).
3. `grep -rn "nolint"` → **1 hit total** (`internal/credits/credits.go:202`,
   documented `defer tx.Rollback() //nolint:errcheck — committed on success
   below`; legitimate, not a test file).
4. `grep -rnE "TODO|FIXME|XXX|HACK" --include="*_test.go"` → **0 hits**.
5. `awk` block-comment scan for hidden test bodies → **0 hits**.
6. `grep -rnE "^\s*//\s*(t\.Errorf|t\.Fatalf|require\.|assert\.)"` →
   **0 hits** (no commented-out assertions).

Then triaged each Skip by category to find the ones masking real issues.

## Category distribution (57 total)

| Category                                       | Count | Verdict     |
|------------------------------------------------|------:|-------------|
| `-short` mode gate                             |     9 | legitimate  |
| "need ≥N decks" after `findDecks` (partial)    |    16 | legitimate  |
| Inside `findAllDecks*` / `findDecks` helpers   |     4 | legitimate  |
| AST corpus / oracle data unavailable           |     5 | legitimate  |
| Env-var opt-in (`HEXDEK_CONVICTION_HARVEST`)   |     1 | legitimate  |
| moxfield / specific-deck-file missing          |     4 | legitimate  |
| **Dead post-`findAllDecks` len() check**       |     3 | **DELETE**  |
| **Dead RNG-retry preamble**                    |     1 | **DELETE**  |
| **Dead "need 2 seats" guard (fixed fixture)**  |     1 | **DELETE**  |
| **Defensive skip on now-passing assertion**    |     2 | **FATAL**   |
| **"test was vacuous" skip on missing signal**  |     1 | **ERRORF**  |
| Total actionable                               |     8 |             |

Hunting harder didn't produce a clean 10. The remaining 49 skips are
legitimate environment guards — they ensure `go test ./...` doesn't fail
in environments where AST corpora / moxfield decks / GBs of data aren't
present. Forcing them off would just make CI flaky.

## Severity legend

- **HIGH** — skip is masking a regression that would surface today if
  removed.
- **MED** — skip is dead code (predicate is impossible) so removing it
  is purely cleanup, but it leaves test intent clearer.
- **LOW** — skip is a stylistic smell; predicate is possible but
  vanishingly rare.

---

## Findings (top 10 examined, 8 acted on)

### 1. `internal/anticheat/anticheat_test.go:124-156` — RNG-retry preamble + flaky skip

- **Severity:** **MED** (dead but worse: it makes the test flaky at
  ~3e-5 odds; the real assertion is the manual-enqueue block after)
- **What:** `TestScheduler_SelectAndEnqueue_PerSeatRows` rolls a Bernoulli
  at p=0.10 against a 1-element population, retries up to 100 seeds, and
  if none hit (prob ≈ 0.9^100 ≈ 2.6e-5) the whole test skips. The
  `queueIDs` returned by the subsequent `SelectAndEnqueue` is `_ =`'d and
  the actual assertion (lines 158-178) manually enqueues per-seat rows
  and checks the fan-out — that part doesn't need the preamble at all.
- **Fix:** delete lines 129-156 (the entire dice-rolling preamble). The
  manual-enqueue block does the real work; deterministic `Select`
  behavior already has its own coverage in
  `TestScheduler_Select_DeterministicWithSeed` (line 106).

### 2. `internal/gameengine/keywords_misc_test.go:1705-1708` — dead "need 2 seats" guard

- **Severity:** **MED** (dead code; predicate is never true)
- **What:** `if len(gs.Seats) < 2 { t.Skip("need 2 seats") }` after a
  call to `newMiscGame(t)`. `newMiscGame` always returns
  `NewGameState(2, ...)` (line 14-18). The guard is unreachable.
- **Fix:** delete lines 1705-1708.

### 3. `internal/gameengine/resolve_test.go:981-984` — Lightning Bolt parsed_tail defensive skip

- **Severity:** **HIGH** (was defensive against a parser gap; the gap
  has since closed, the skip is dead AND masks regression)
- **What:** `TestWithCorpus_LightningBolt` finds Lightning Bolt's Damage
  AST node through a Static/parsed_tail wrapper. If the parser ever
  re-wraps Bolt incorrectly, `dmg == nil` and the test skips. Running
  today shows the test PASSES — parser handles Bolt correctly.
- **Fix:** convert `t.Skip(...)` to `t.Fatalf("Lightning Bolt Damage
  no longer structured in corpus — parser regression?")` so a future
  drift back to parsed_tail surfaces as a real failure.

### 4. `internal/deckparser/deckparser_test.go:106-110` — Tergrid parse-error skip

- **Severity:** **HIGH** (skip never trips today, masks regression)
- **What:** `TestParseDeckReader` parses a 4-line text deck with Tergrid
  as DFC commander. The comment says "Tergrid is a legendary card that
  might or might not be in the meta. Don't fail hard." Running shows
  Tergrid IS in the corpus and the test passes.
- **Fix:** convert `t.Skipf("parse: %v", err)` to `t.Fatalf("ParseDeckReader:
  %v", err)`. If Tergrid disappears from the corpus, that itself is a
  regression worth surfacing.

### 5. `internal/tournament/mdfc_grinder_test.go:181-184` — "test was vacuous" skip

- **Severity:** **HIGH** (defensive on a known-flaky observation; the
  right reaction is to fail loud, not skip)
- **What:** after running N games and looking for permanent_types
  violations on FF-MDFC cards, the test asks: "did any FF MDFC card
  actually reach a battlefield?" If no, skip ("test was vacuous").
  This is a false-pass: the integration test claims green when it
  observed nothing.
- **Fix:** convert `t.Skip(...)` to `t.Errorf(...)`. If the grinder
  produces zero observations of the thing being tested, that's a
  test-quality regression — increase `numGames`/`maxTurns` or update
  the deck pool.

### 6, 7, 8. `internal/tournament/mcts_diag_test.go:67-69, 187-189, 319-321` — dead post-`findAllDecks` skip checks

- **Severity:** **MED** (each is dead-redundant after `findAllDecks`,
  which is fatal-flow on undersized result)
- **What:** every caller of `findAllDecks(t, N)` does
  ```go
  paths := findAllDecks(t, N)
  if len(paths) < N {
      t.Skipf("need at least %d decks, found %d", N, len(paths))
  }
  ```
  `findAllDecks` either returns ≥ N paths or itself fires `t.Skipf` at
  line 312 (which `runtime.Goexit`s the test). The follow-up check
  never runs. Verified by reading the helper: the only `return` with
  fewer than `minN` is unreachable because the function only returns
  early on `len(all) >= minN`, and otherwise falls through to the
  fatal `t.Skipf` after the loop.
- **Fix:** delete the three `if len(paths) < … { t.Skipf(...) }`
  blocks. `stress_test.go`'s callers of `findAllDecks` either check a
  *different* threshold (e.g., line 50: `< 8` after `findAllDecks(t, 4)`)
  or skip the redundant check entirely — both are correct and don't
  need editing.

### 9. (would have been mdfc_grinder_test.go vacuous; same as #5)

### 10. (no further finding; see "Category distribution" — the remaining 49 skips are all legitimate)

---

## Top-10 fixes landing in this PR

Eight items above; numbered 1-8 in the order they appear in code.
Combined diff: delete 4 dead blocks (≈ 30 lines), convert 3 defensive
skips to fatal/errorf, delete one flaky preamble.

## Follow-up (not in this PR)

- The `internal/anticheat` tests have a separate
  `TestScheduler_Select_DeterministicWithSeed` (line 106) that does
  what the deleted preamble was trying to do. If future tests need
  Bernoulli coverage with a known-hit seed, the right pattern is to
  iterate seeds in a *constructor* and pin the result — not roll
  inside the test body.
- The remaining 49 legitimate environment-gate skips form a healthy
  pattern. If we ever want CI to *require* the data files to be
  present (current default: skip if missing), the right place is
  `TestMain` with an explicit `os.Exit(0)` opt-out, not per-test
  skip churn.
