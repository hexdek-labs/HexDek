# Contributing to HexDek

Practical guide for working on the engine, hat, tournament runner,
and per-card handlers. For domain orientation start with the
top-level `CLAUDE.md`; this doc covers the mechanics.

## Local development setup

```bash
git clone https://github.com/hexdek-labs/HexDek.git
cd HexDek
go build ./...
```

You need Go 1.22+ (the repo uses generics + the `mux.HandleFunc("GET
/path", h)` routing syntax from net/http 1.22). No external services
to install — SQLite is in-process via `modernc.org/sqlite`.

### Required data files

These are gitignored because of their size. Fetch before running the
server, tournament binary, or any test that loads the corpus:

```bash
scripts/fetch-oracle.sh   # downloads ~163MB of Scryfall oracle data
```

The fetcher writes:
- `data/rules/oracle-cards.json` (~163 MB)
- `data/rules/ast_dataset.jsonl` (~46 MB)

A symlinked worktree (used by the dev agents) inherits these via
`../../../../data/rules/`.

### Running the server

```bash
go run ./cmd/hexdek-server/
```

Default port: 8090. With WireGuard up the production server at DARKSTAR
serves on `192.168.1.207:8090`. See `cmd/hexdek-server/main.go` for
all CLI flags.

### Running the frontend

```bash
cd hexdek
npm install
npm run dev   # vite dev server on 5173
```

Production build:

```bash
cd hexdek && VITE_API_URL="" npx vite build
```

Frontend source is `hexdek/` (Vite + React). Do not deploy from any
other directory — there used to be a duplicate `frontend/` that's now
disused.

## Running tests

Per-package, fast:

```bash
go test ./internal/gameengine/...     -count=1 -timeout 300s
go test ./internal/hat/...            -count=1
go test ./internal/tournament/...     -count=1
go test ./internal/hexapi/...         -count=1
go test ./internal/gameengine/per_card/... -count=1
```

`-count=1` disables Go's test cache so you actually re-run the
assertions. `-timeout 300s` (5 min) is needed for the gameengine
suite — many integration tests spin up small tournaments.

Whole-repo sweep before pushing:

```bash
go build ./... && go test ./... -count=1 -timeout 600s
```

Known pre-existing failures live in `internal/hexapi/openapi_validate_test.go`
(duplicate YAML keys in the spec) — flagged for cleanup but unrelated to
most PRs.

### Running a single test

```bash
go test ./internal/hat/ -run "TestCompositionPrior" -count=1 -v
```

`-v` prints PASS/FAIL per case so you can see which sub-tests fired.

### Tournament-style integration smoke

To verify the engine end-to-end (no test framework, just runs games
to completion):

```bash
go run ./cmd/hexdek-tournament/ --decks data/decks/moxfield/ --games 100 --seed 42
```

For a smaller pool (4-deck rotate):

```bash
go run ./cmd/hexdek-tournament/ \
  --decks data/decks/moxfield/phenax_*.txt,data/decks/moxfield/wyleth_*.txt,data/decks/moxfield/kalamax_*.txt,data/decks/moxfield/lord_windgrace_*.txt \
  --games 500 --seed 42
```

## Parallel-worker patterns

Several workflows in this repo benefit from running many tasks in
parallel:

- **Fuzz testing** via `cmd/hexdek-loki/` — random-deck games at
  scale to surface engine crashes. `--games 5000 --seed 41` is the
  canonical regression sweep referenced throughout `CLAUDE.md`'s
  Issue Log.
- **Goldilocks audits** via `cmd/hexdek-thor/` — corpus-wide keyword /
  ability scaffolding checks.
- **Tournament round-robin** via `cmd/hexdek-tournament/
  --round-robin` — every C(N, seats) combination of decks plays the
  configured game count.

When iterating on per-card fixes that fan out across many cards
(e.g. a sweeping nil-deref forensic), the practical pattern is:

1. Land the engine-level fix on `main`.
2. Run Loki against the offending seed(s) to confirm the bug class
   is gone.
3. Open follow-up PRs for sibling cards that share the anti-pattern
   (see CLAUDE.md's "Issue Log" Resolved entries for examples — the
   Abdel Adrian nil-deref forensic flagged 6 sibling handlers carrying
   the same pattern).

`CLAUDE.md` references "dev agents" and "BlueFrog hex-worker parallel
pattern" — those are project-coordination patterns for spawning
multiple Claude sessions against the same repo. They live in the
user's personal-workflow memory rather than the repo; if you're
working solo in the codebase, the standard pattern is "land + audit
siblings serially" described above.

## Adding a new card handler

Per-card behavior lives in `internal/gameengine/per_card/`. There are
two file conventions:

- `custom_<commander_or_card>.go` — hand-written handlers for
  high-traffic cards. Owns the canonical implementation.
- `gen_<card>.go` — auto-generated stubs from the AST parser. Used
  for cards whose AST shape covers all behavior; the gen file is a
  thin `Register…(r *Registry)` wiring around scaffolding primitives.

### Anatomy of a custom handler

```go
// custom_lier_disciple_of_the_drowned.go
package per_card

import "github.com/hexdek/hexdek/internal/gameengine"

func registerLierDiscipleOfTheDrownedCustom(r *Registry) {
    r.OnTrigger("Lier, Disciple of the Drowned", "instant_or_sorcery_cast",
        lierUncounterableMark)
    r.OnETB("Lier, Disciple of the Drowned", lierGraveyardFlashbackGrantETB)
    r.OnTrigger("Lier, Disciple of the Drowned", "permanent_ltb",
        lierGraveyardFlashbackGrantLTB)
}

func lierGraveyardFlashbackGrantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
    grant := &gameengine.GraveyardFlashbackGrant{
        Controller:      perm.Controller,
        SourceTimestamp: perm.Timestamp,
        SourceName:      perm.Card.DisplayName(),
        OnlyActiveTurn:  false,
        CostFor:         gameengine.PrintedMassFlashbackCost,
    }
    gameengine.RegisterGraveyardFlashbackGrant(gs, grant)
    emit(gs, "lier_flashback_grant", perm.Card.DisplayName(), nil)
}
```

Three steps for a new custom handler:

1. **Create the file.** Put it in `internal/gameengine/per_card/`
   with the `custom_` prefix.
2. **Register via init() + AddResetHook.** The registry has a
   per-test `Reset()` hook that wipes runtime state; your init must
   re-attach via `AddResetHook(registerYourCard)` so tests that call
   `Reset()` don't permanently lose your handler:

```go
func init() {
    registerYourCard(Global())
    AddResetHook(registerYourCard)
}
```

3. **Test against a Loki regression seed.** Most card-fix PRs in the
   Issue Log were validated this way:

```bash
go test ./internal/gameengine/per_card/ -run "TestYourCard" -count=1 -v
go run ./cmd/hexdek-loki/ --games 500 --seed 41   # spot-check the broader corpus
```

### When to write a custom file vs. extend a gen file

- **Custom** for cards whose behavior the parser can't express
  (multi-step replacement effects, stateful counters, exotic
  zone-change patterns).
- **Extend the gen file** (in-place) for cards whose AST is close to
  right but needs one runtime tweak — e.g. an LTB cleanup the AST
  parser doesn't emit. See the R60 stub-batch sweeps (PR #167's batch
  G, #185 round 3) for canonical examples.
- **Neuter the gen file** when a custom handler already owns the
  card; the gen body should be a no-op `register…` function so it
  doesn't fight the custom path. See `percard_stub_batch_g_r50_test.go`
  for the regression pattern that catches gen/custom collisions.

### Pre-flight checks for every new card

1. **Search for similar patterns first.** Don't whack-a-mole — if a
   bug appears in one card, grep `internal/gameengine/per_card/` for
   the same anti-pattern and fix the family. Memory entry "card-fix
   dispatches auto-search siblings" pins this.
2. **Run the full per_card suite.** Even a one-card change can break
   sibling tests via the registry:

```bash
go test ./internal/gameengine/per_card/ -count=1
```

3. **Verify against Loki at the affected seed.** If the bug surfaced
   in a fuzz run, re-run the same seed and confirm the count drops
   to zero.

## Logging issues

Every bug surfaced by tests / Loki / Goldilocks / manual investigation
gets logged in `CLAUDE.md`'s **Issue Log** (Open / Resolved tables).
The rule lives at the top of that section — applies whether or not
you have a fix ready. Open issues drive next-cycle work; Resolved
issues stay logged with the resolution PR link so future investigators
don't re-investigate fixed problems.

## Commit + PR conventions

- Branch naming: `dev/<topic>-r60` (R60 is the current release era;
  bump for the next cycle). Recent examples: `dev/composition-elo-
  implementation-r60`, `dev/hat-priority-defensive-r60`.
- Commit message: `<scope>(<area>): <imperative summary>` (e.g.
  `feat(hat): wire CompositionPrior into Update path`,
  `fix(per_card): Athreos must not race a sibling claim`). Body
  explains WHY + lists the regressions added.
- All assistant-authored commits sign with the `Co-Authored-By:
  Claude Opus 4.7 (1M context) <noreply@anthropic.com>` trailer.

## Where to look when you're stuck

- `CLAUDE.md` — top-level domain orientation + tool suite + common
  commands + the full Issue Log
- `docs/API.md` — REST / GraphQL / WebSocket surface index with
  quick-start curl/wscat examples
- `docs/composition-elo.md` — TrueSkill + composition prior reference
- `docs/composition-elo-blog.md` — research-story companion
- `docs/freya-api.md` — deck-analysis REST surface
- `docs/hexapi-openapi.yaml` — machine-readable spec for every REST
  endpoint
