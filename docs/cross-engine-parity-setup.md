# Cross-Engine Parity Setup (Verification Phase 4 prep)

Status: **scaffolding only**. Test runs require a JVM-resident xmage server which is intentionally out of scope for this PR. The goal here is to leave a future-Phase-4 worker with: a) an honest picture of what exists, b) the concrete gaps blocking cross-engine runs, c) a 17-scenario seed corpus that exercises the load-bearing engine surfaces, d) a CI entry point that returns the right exit code without xmage and surfaces "skipped" honestly.

## 1. What exists today

`internal/paritycheck/paritycheck.go` is fully built out for **Go ↔ Python** parity:

- `Event` — canonical normalized event (Seq, Turn, Phase, Step, Seat, Kind, Source, Target, Amount, Rule)
- `Outcome` — final per-game summary (Winner, WinnerName, Turns, EndReason)
- `ReplayData` — events + outcome wrapper
- `Divergence` — single reported difference between two engines
- `ParityReport` — aggregate output with category counts, markdown writer
- `RecordGoGame(nSeats, decks, seed, gameIdx)` → `*ReplayData` runs one Go game and normalizes events to the canonical form
- `RunPython(cfg, gameIdx, seed)` → `*ReplayData` shells out to a Python harness
- `Diff(gameIdx, goReplay, pyReplay)` → `[]Divergence` walks paired streams
- `Run(Config)` → `*ParityReport` end-to-end orchestrator
- `WriteMarkdown(report, path)` writes the report to disk

CLI driver at `cmd/hexdek-parity/main.go` accepts `--decks`, `--games`, `--seed`, `--seats`, `--python-harness`, `--python-bin`, `--report`. Has 4 unit tests in `internal/paritycheck/paritycheck_test.go`.

## 2. What is missing (gap inventory)

The infrastructure is shaped for Python target — switching to xmage needs four discrete pieces:

### 2.1 The Python harness itself

`paritycheck.RunPython` shells out to `scripts/parity_harness.py`. **That file does not exist in the repo.** When missing, `paritycheck.Run` correctly degrades to `pythonAvailable: false` and emits Go-only outcomes — *no false-positive parity claims*. Good. But it means there is no working reference target today; the Go ↔ Python promise has never actually been driven.

For xmage we don't need this file — we need a sibling Java adapter. But the gap is informative: cross-engine parity has been "infrastructure ready, target missing" for a while.

### 2.2 An xmage adapter

xmage is a Java application built on a custom rules engine + Mage-style replay infrastructure. For parity we need a thin shim that:

1. Boots an xmage `GameImpl` headless (no UI)
2. Loads the same 4 deck files (xmage parses `.dck` / `.txt` deck formats)
3. Runs the game with a deterministic RNG seed
4. Exports the event stream in a parseable form
5. Outputs the same canonical `Event` JSON shape `parsePythonReplay` already understands

**Replay export format**: xmage emits per-game log files (`/data/saved/` under the xmage data dir) but they are human-readable game logs, not structured event traces. The xmage `GameView` API exposes per-event hooks via `GameImpl.fireEvent` and the underlying `EventDispatcher` — that's the path a Java-side adapter would tap. The Phase-4 worker should expect to write a small Java program (~200 LOC) that subscribes to xmage's `GameEvent` stream and writes a canonical `ReplayData` JSON. Reference points: `mage.game.events.GameEvent` (the event type enum), `mage.game.GameImpl.fireEvent` (the dispatcher entry point), `mage.game.match.MatchImpl` (the multi-game wrapper).

**Determinism**: xmage uses `java.util.Random` seeded from `MatchOptions`. The seed surface is `MatchOptions.setMatchType(...)` + a custom test harness; the existing `xmage-tests` module (`/Mage.Tests/`) demonstrates the pattern.

**RNG identity is impossible**: even with seed parity, Go's `math/rand` and Java's `java.util.Random` produce different sequences. Parity comparisons must be **outcome-level** (winner, turn count, life totals at checkpoint) and **event-stream structure** (same kind of events in same order modulo policy drift), not bit-for-bit RNG. The existing `paritycheck.outcomesEqual` already handles this — `EventStreamMatches` is a softer signal than `OutcomeMatches`.

### 2.3 Game-state JSON ingest

The current `RecordGoGame` entry point takes `(nSeats, decks, seed, gameIdx)` and plays a full game from shuffle. For **scenario-driven** parity (e.g. "given board state X, what happens when Y resolves?"), we need a separate entry point:

```go
// PROPOSED — NOT YET IMPLEMENTED
func RecordGoScenario(scenarioPath string) (*ReplayData, error)
```

This would require a `gameengine.GameState` JSON marshaler/unmarshaler that captures the load-bearing state surface:
- All seat states (life, hand, library, graveyard, exile, battlefield slices)
- Each `*Permanent` (Card ref, Controller, Owner, Timestamp, Counters, Flags, Tapped, AttachedTo)
- Active seat, turn number, current phase/step
- Stack contents
- Replacement effects + continuous effects registry
- Pending triggers

`internal/game/json_helpers.go` has thin `jsonMarshal`/`jsonUnmarshal` wrappers but no `MarshalGameState`/`UnmarshalGameState` entry points. There is no documented round-trip path. **This is the biggest engine-side gap blocking scenario parity.**

**Phase-4 sequencing**: bootstrap with `mode: "deck_seed"` scenarios (works against existing infrastructure today); land `mode: "state_inject"` after a separate PR adds GameState JSON round-tripping.

### 2.4 CI-friendly target

No `Makefile` exists in the repo. CI parity-test entry needs a shell script or `make` target that:

- Builds `hexdek-parity` if not cached
- Runs the scenario corpus
- Returns exit code 0 (parity), 1 (divergence), or 77 (skipped — xmage adapter not present)
- Emits a JUnit-style XML or JSON summary for CI consumption

The "skipped" code 77 is the standard autotools convention used by `prove`, `automake`, etc. — CI tooling already understands it as "test was unable to run, don't count as failure".

This PR ships `scripts/parity-test.sh` as the entry point. Switching to `make parity-test` is one line if/when a Makefile is added.

## 3. The 17-scenario seed corpus

`data/parity-scenarios/*.json` — 17 scenario skeletons covering the load-bearing engine surfaces. Each one is `mode: "deck_seed"` and uses decks that already live in `data/decks/test/` (the calibration corpus). Reproducibility is the design goal: same deck pool + same seed + same engine version → identical outcome.

The scenarios are organized by tag for selective CI runs:

| Tag | Count | Surface exercised |
|---|---|---|
| `combat-fundamentals` | 3 | Plain attack, combat damage, lethal trample |
| `mass-effects` | 2 | Board wipes, mass exile, SBA cascades |
| `etb-cascade` | 2 | ETB doublers, blink loops, observer triggers |
| `combo-wincons` | 3 | Thassa's Oracle, Walking Ballista X-storm, infinite-mana payoffs |
| `replacement-effects` | 2 | Platinum Angel cancel, Rest in Peace exile-redirect |
| `stack-interactions` | 2 | Counter chains, copy-spell §707.10 cease |
| `multiplayer-dynamics` | 2 | Commander damage (CR §704.5j), §800.4a seat elimination |
| `edge-cases` | 1 | Empty library / max hand size |

Each scenario JSON has the schema:

```json
{
  "name": "combat_fundamentals_alpha_strike",
  "description": "Pure aggro: voja_wolf_elf attacks unblocked into nazgul_tribal for lethal commander damage by turn 8.",
  "mode": "deck_seed",
  "deck_pool": ["data/decks/test/voja_wolf_elf_tribal_b4_jaws_of_the_conclave.txt", ...],
  "n_seats": 4,
  "seed": 42,
  "max_turns": 30,
  "expected_outcome": {
    "winner_seat_in": [0, 1, 2, 3],
    "min_turns": 4,
    "max_turns": 25,
    "end_reason_in": ["life_zero", "commander_damage", "concede", "library_empty"]
  },
  "tags": ["combat-fundamentals", "phase4-bootstrap"]
}
```

`expected_outcome` uses *range constraints* rather than exact-match values because:
1. Pre-xmage, the Go side is the only source of truth; range constraints catch the obvious Go-side regressions (a scenario that used to end at turn 8 now ending at turn 1 = bug) without forcing a brittle exact match.
2. Post-xmage, the cross-engine diff path is `paritycheck.Diff` — that's where strict outcome equality lives. The range here is a **floor** check against the Go side.

A separate `mode: "state_inject"` field is reserved for the post-GameState-JSON-roundtrip future. Schema spec doc in §4.

## 4. Scenario file schema

Validation is pinned by `internal/paritycheck/scenario_schema_test.go` — see §5.1.

Required fields:

- `name` (string, kebab-case, unique within `data/parity-scenarios/`)
- `description` (string, 1-2 sentences for what behavior is exercised)
- `mode` (string, one of `"deck_seed"` / `"state_inject"`)
- `tags` (string[], at least one tag from the §3 table)

For `mode: "deck_seed"`:

- `deck_pool` (string[], paths relative to repo root; must have at least `n_seats` entries)
- `n_seats` (int, 2-4)
- `seed` (int64)
- `max_turns` (int, > 0; caps the simulation to prevent runaway games)
- `expected_outcome` (object — see schema below)

For `mode: "state_inject"` (reserved, not yet usable):

- `state_path` (string, path to GameState JSON; not yet usable until engine ships round-trip)
- `events_to_execute` (string[], the player actions to drive after state load)
- `expected_outcome` (same shape as deck_seed)

`expected_outcome`:

- `winner_seat_in` (int[], allowed winner seats; `-1` represents draw)
- `min_turns` / `max_turns` (int, inclusive range)
- `end_reason_in` (string[], allowed end reasons: `life_zero` / `commander_damage` / `poison` / `concede` / `library_empty` / `mandatory_loop_draw` / `loss_effect`)

## 5. Test scaffolding

### 5.1 Schema validation test

`internal/paritycheck/scenario_schema_test.go` loads every `*.json` under `data/parity-scenarios/`, decodes it, and asserts schema conformance:

- All required fields present + non-empty
- `mode` is one of the allowed values
- `n_seats` in `[2, 4]`
- `deck_pool` length >= `n_seats`
- `max_turns` > 0
- Every `deck_pool` path resolves to an existing file
- `tags` contains at least one entry
- `name` matches the file basename (sans `.json`)
- No duplicate `name` across the corpus

This test runs in CI without xmage and without playing any games. Catches scenario-file mistakes during PR review.

### 5.2 CI entry point: `scripts/parity-test.sh`

Drives the scenario corpus through `hexdek-parity` (the existing CLI). Modes:

- `./scripts/parity-test.sh` — runs the full corpus, exits 0/1/77
- `./scripts/parity-test.sh --tag combat-fundamentals` — runs scenarios tagged `combat-fundamentals` only
- `./scripts/parity-test.sh --skipped-as-fail` — for stricter CI (treat missing xmage as failure)

Without an xmage adapter present (the current state), every run exits **77 (skipped)** with an honest message: "xmage adapter not configured; Go-only baseline recorded". The Go baseline DOES get written to `data/parity-scenarios/.baseline/<scenario>.json` so future xmage runs can diff against it.

## 6. Phase 4 worker handoff

The work that needs to land next (in order):

1. **Write `scripts/parity_harness.py`** — even if we go straight to xmage, the Python harness is the structural template for the Java adapter. Reading paritycheck.parsePythonReplay (`paritycheck.go:384-413`) shows the expected output schema.
2. **Write the xmage Java adapter** — ~200 LOC subscribing to `mage.game.events.GameEvent` and emitting canonical `ReplayData` JSON. Section 2.2 has the API references.
3. **Add `--xmage-harness` flag** to `cmd/hexdek-parity/main.go` mirroring `--python-harness`. Implement `paritycheck.RunXmage` mirroring `RunPython`.
4. **Land `GameState` JSON round-trip** in a separate engine PR. Unblocks `mode: "state_inject"` scenarios.
5. **Wire `scripts/parity-test.sh` to a real CI job** — replace the `77` exit code with actual cross-engine runs.

Steps 1-3 are tightly coupled (a single Phase-4 PR). Step 4 is a separate engine PR with broad surface area (the GameState struct has 50+ fields; round-tripping each one is real work). Step 5 is downstream of all of them.

## 7. Why this PR ships scaffolding without runs

The cleanest separation: this PR makes future xmage-adapter work cheaper (the scenario corpus, the CI entry point, the schema test, the doc spelling out the gap inventory). It does not pretend to deliver cross-engine validation we cannot run today. The next worker walking in finds:

- A doc that orients them in <5 minutes
- 17 scenarios already shaped for the canonical schema
- A CI script that does the right thing today (skip-with-77) AND will do the right thing tomorrow (run, diff, report) with no script-side changes
- A schema test that catches scenario-file regressions during PR review
- Explicit handoff in §6 of what to land next

Phase 4 verification is the destination; this PR is the path-laying.
