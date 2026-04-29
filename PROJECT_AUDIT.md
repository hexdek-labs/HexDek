# mtgsquad — Project Audit (2026-04-15)

A single-snapshot audit of the mtgsquad codebase at the end of the
parser-hardening / self-play push. This file is **generated**, not
hand-maintained — it is the artifact of a "final polish" pass and is expected
to go stale as the parser and engine evolve. Regenerate by re-running the
sanity commands in the "Verification log" section.

---

## What exists

File tree grouped by purpose. Sizes / line counts are as of 2026-04-15.

### Entry-point docs

| File | Purpose |
|---|---|
| `README.md` | High-level overview, quickstart, status table, repo layout |
| `ARCHITECTURE.md` | Parser → AST → extensions → harness → playloop → runtime map |
| `PROJECT_AUDIT.md` | **This file.** Snapshot of project state as of 2026-04-15 |

### Parser core

| File | Purpose |
|---|---|
| `scripts/parser.py` | Recursive-descent oracle-text parser, ~106k bytes |
| `scripts/mtg_ast.py` | Frozen-dataclass AST schema per comp rules §113 |
| `scripts/extensions/` | ~50 grammar-extension modules + `per_card.py` (1,079 handlers) |
| `scripts/extensions/CHANGELOG.md` | Extension evolution notes |
| `scripts/patterns/` | Additional grammar pattern tables |

### Parser-adjacent tooling

| File | Purpose |
|---|---|
| `scripts/coverage_honest.py` | Dual-metric coverage reporter (structural / mixed / stub / vanilla) |
| `scripts/rules_coverage.py` | Detailed parser diagnostics |
| `scripts/layer_harness.py` | §613 continuous-effects layer tagger |
| `scripts/cluster_priority.py` | Prioritize stub work by EDHREC usage frequency |
| `scripts/semantic_clusters.py` | Cluster cards by AST signature |

### New surface area (this push)

| File | Purpose |
|---|---|
| `scripts/combo_detector.py` | Static-analysis infinite-combo scanner (6 families, 2-card pair search) |
| `scripts/playloop.py` | Python self-play loop w/ full turn structure including combat (~1,630 LOC) |
| `scripts/export_ast_dataset.py` | Emit typed AST to JSONL for finetuning / analysis |
| `scripts/export_finetune_pairs.py` | Alpaca-style oracle-text → AST instruction pairs |
| `web/replay/` | Zero-dependency static browser viewer for playloop JSONL logs |
| `tests/` | 203-test pytest suite (201 goldens + 2 whole-corpus asserts) |

### Generated artifacts (`data/rules/`)

| File | Produced by | What it is |
|---|---|---|
| `parser_coverage.md` | `parser.py` | Parser coverage summary |
| `coverage_honest.md` | `coverage_honest.py` | Dual-metric (structural / mixed / stub / vanilla) |
| `layer_harness.md` | `layer_harness.py` | §613 layer distribution |
| `combo_detections.md` / `.json` | `combo_detector.py` | Infinite-combo candidates |
| `playloop_report.md` | `playloop.py` | 3-matchup self-play results |
| `playloop_sample_log.txt` | `playloop.py` | JSONL event stream for replay viewer |
| `ast_dataset.jsonl` | `export_ast_dataset.py` | 31,639 rows, ~39 MiB |
| `ast_dataset_README.md` | `export_ast_dataset.py` | HF-ready dataset card |
| `finetune_pairs.jsonl` | `export_finetune_pairs.py` | Instruction-tuning pairs |
| `cluster_priority.md` | `cluster_priority.py` | Stub work ranked by usage |
| `semantic_clusters.md` | `semantic_clusters.py` | AST-signature clusters |
| `partial_diagnostic.md` | (historical) | Pre-100%-GREEN diagnostic |

### Go runtime (scaffold only)

| File | Purpose |
|---|---|
| `cmd/mtgsquad-server/` | HTTP/WebSocket entry point |
| `internal/game/` | Turn structure, mana, casting, simple actions |
| `internal/shuffle/` | `crypto/rand` Fisher-Yates, commit-reveal |
| `internal/moxfield/` | Moxfield-format deck import |
| `internal/oracle/` | Scryfall card lookup |
| `internal/db/` | SQLite ephemeral game state |
| `internal/ws/` | WebSocket hub |
| `internal/ai/`, `auth/`, `mana/`, `party/`, `rules/` | Stub packages |

### Inputs (large, some gitignored)

| File | Size | What it is |
|---|---|---|
| `data/rules/oracle-cards.json` | ~163 MiB | Scryfall bulk dump (source of truth for card text) |
| `data/rules/MagicCompRules-20260227.txt` | 942 KB | Comprehensive Rules PDF-extracted text |

---

## What works today

**Parser (stable, load-bearing).**

- 100% GREEN syntactic coverage on all 31,639 real printed Magic cards — every card returns an AST with no parse errors.
- Recursive-descent grammar with ~50 extension modules, registered at import time via `load_extensions()`. New set mechanics land as a new extension, not a core parser patch.
- Typed AST: every ability is a `Static` / `Activated` / `Triggered` / `Keyword`, and effect bodies are dispatched through ~40 typed Effect nodes.

**Typed AST as a dataset.**

- Every card's AST is serializable (`scripts/export_ast_dataset.py`) to a JSONL dataset with `__ast_type__` tags on every dataclass node. 31,639 rows, ~39 MiB.
- An Alpaca-style instruction-tuning pair variant (`scripts/export_finetune_pairs.py`) is available for oracle-text → AST finetuning.
- Both outputs come with a ready-to-publish HF-format README (`ast_dataset_README.md`).

**Coverage accounting.**

- "Honest coverage" separates *syntactic* coverage (100% parsed) from *engine-executable* coverage (structural AST only, no stubs). Currently: **~24.15%** engine-executable, **~75.85%** engine work owed. See `data/rules/coverage_honest.md`.

**§613 layer tagging.**

- Every static ability's `Modification` is tagged with its continuous-effects layer (1, 2, 4, 6, 7a-7e). Pre-tagging avoids the runtime having to derive layers on the fly.

**Regression test suite.**

- 203 pytest tests: 201 golden-file card snapshots across 8 mechanic families + 2 whole-corpus coverage asserts. One regression = one red test, not the whole suite.

**Python self-play loop.**

- Three matchups (Burn / Control / Creatures), 20 games each at seed=42: 90% / 100% / 85% win rates for the expected dominant deck. Full turn structure including a real combat phase.
- Combat keyword handling: flying, reach, first strike, double strike, trample, deathtouch, lifelink, vigilance, menace (2+ blockers), defender, haste.
- Dispatch on Effect node kinds: Damage, Draw, Discard, Destroy, Buff, CreateToken, AddMana, GainLife, LoseLife, CounterMod, Tap/Untap, Sequence/Choice/Optional/Conditional control flow, ETB triggers, upkeep triggers, Tutor (basic-land), Reanimate.

**Replay viewer.**

- Zero-dependency static HTML/CSS/JS at `web/replay/`. Drag-and-drop a JSONL event log, scrub / step / autoplay through events against a two-seat board. No game logic runs in the viewer.

**Combo detector.**

- Static-analysis scanner with six families: mana-positive engines (1,239 candidates), untap triggers (213), storm engines (1,508), iterative draw engines (280), doublers (37), activated-untap permanents (58). 2-card pair search yields 375,805 candidate pairs with top-N ranked by heuristic confidence. Human-review candidates, not solved combos.

**Go HTTP/WebSocket server (scaffold).**

- `cmd/mtgsquad-server/` serves basic REST endpoints against an in-memory game. Turn structure, mana, casting, simple actions work. SQLite ephemeral state. WebSocket hub. `crypto/rand` Fisher-Yates shuffle with commit-reveal option. Moxfield deck import. This is not yet AI-playable or AST-driven.

---

## What's stubbed / known limitations

Honest list of what doesn't work yet.

**Engine (Go) side:**

- Does not consume the Python AST. AST export to a Go-readable form (JSON/protobuf handoff) is not wired.
- No stack. Spells resolve at cast time in the Python playloop; the Go engine has no stack object either.
- No colored mana — single generic pool, CMC-only cost check.
- No continuous-effects resolver. §613 layer tags exist in the AST but are not yet applied at runtime.
- No custom-slug or per-card-handler resolver registry. Stub modifications are recognized but do not execute.
- No AI-playable surface.

**Python playloop side:**

- No instants during opponent's turn (requires a stack).
- No combat tricks / pump spells during combat (same reason).
- No planeswalker attackers; attacks always target player.
- No protection / hexproof / shroud / indestructible / ward.
- No damage-prevention / replacement effects.
- Scry / Surveil / LookAt / Reveal are no-ops (library not reordered).
- `Choice` always picks option 1. `Conditional` always executes body.
- `ExtraTurn` and `GainControl` are no-ops.
- 52 unique `UnknownEffect` node slugs observed across the 3 matchups — these are valid AST nodes that the playloop doesn't yet dispatch.
- 289 `UnknownEffect` conjunction patterns pending in the parser side (the parallel "conjunction-fix" pass is still shrinking this number).

**Parser side (remaining edges):**

- 10,831 cards (34.23%) still carry all-stub Modifications. Parser sees them; runtime has nothing to execute.
- 13,167 cards (41.62%) are "mixed" — some abilities typed, some stubbed. Playable but incomplete.
- `per_card.py` handlers emit stub `Modification(kind="custom", args=(slug,))` for 1,079 snowflakes. The runtime needs a slug → resolver map.

**Dataset side:**

- Cluster priority and semantic cluster reports were generated against an older 31,655-card snapshot and have not been regenerated since the final-set pass dropped the pool to 31,639. Non-blocking; regenerate when convenient.

**Test suite side:**

- 3 golden-file tests (`basking_rootwalla`, `shivan_dragon`, `thorn_lieutenant`) are currently failing because the parser's effect-leaf resolution is now emitting more-specific ops (`buff`, `create_token`) where the golden expects `unknown`. This is parser progress, not a regression. **Action:** regenerate goldens via `python3 tests/generate_golden.py` once the parallel conjunction-fix pass is done. Do NOT regenerate mid-pass — it will lock in half-finished parser state.

---

## What's next

Recommended work order for the next push. Tier numbering follows the pre-existing planning doc.

### Tier 2 — Runtime engine consumes AST (highest leverage)

1. Port the Python playloop's Effect-node dispatcher to Go. The dispatch table is small (~40 leaf kinds); the hard part is plumbing it through `internal/game/`.
2. Emit the AST as JSON/protobuf on Go startup (re-use `export_ast_dataset.py`'s shape). Go engine loads once, builds `map[kind]Handler` and `map[slug]Resolver`.
3. Implement the stack. Unblocks instants, combat tricks, counterspells, triggered-ability ordering.
4. Colored mana. Unblocks Ancestral Recall, Counterspell, Lightning Bolt at the color-cost level.
5. §613 continuous-effects resolver. AST layer tags already exist; hook them into the state-based-actions pass.

### Tier 3 — Fill in the stubs

1. Walk `data/rules/cluster_priority.md` top-down. Each entry is a custom-modification slug ranked by how often it appears in real decklists. Implementing the top-100 slugs covers a disproportionate share of real-game cards.
2. Work through `per_card.py` in the same priority order.

### Tier 4 — Combat polish

1. Planeswalker attackers / redirected damage.
2. Protection / hexproof / shroud / indestructible / ward.
3. Damage-prevention and replacement effects (handoff to §613 layer system).
4. Combat tricks during declare-blockers / first-strike (requires stack).
5. MCTS or heuristic policy for `Choice` and `Conditional` branches.

### Tier 5 — Presentation

1. Resolve the 3 failing golden-file tests by regenerating after the conjunction-fix pass lands.
2. Regenerate `cluster_priority.md` and `semantic_clusters.md` against the 31,639 pool.
3. Shrink the UnknownEffect tail in `playloop.py` by implementing dispatchers for the 52 observed slugs.
4. Wire `web/replay/` against a live game (WebSocket stream from the Go engine) rather than only replaying static JSONL.

---

## Verification log

Commands run on 2026-04-15 as part of this audit pass. All invoked from
`sandbox/mtgsquad/` with system Python 3.11.

### 1. Parser coverage

```bash
python3 scripts/parser.py
```

Result:

```
GREEN:    31,639 (100.00%)
PARTIAL:       0  (0.00%)
UNPARSED:      0  (0.00%)
```

**PASS.**

### 2. Full pytest suite

```bash
python3 -m pytest tests/ -q
```

Result: **200 passed, 3 failed** in ~77s. Failures are the three goldens listed under "What's stubbed" — they reflect parser progress (more-specific effect-leaf resolution) outrunning the golden snapshots, not a regression. Regenerate via `python3 tests/generate_golden.py` when the conjunction-fix pass lands.

### 3. Self-play loop

```bash
python3 scripts/playloop.py --games 20 --seed 42
```

Matchup results (20 games each):

| Matchup | Winner | Win rate | Avg turns |
|---|---|---:|---:|
| Burn vs Control | Burn | 90% (18/20) | 7.65 |
| Creatures vs Control | Creatures | 100% (20/20) | 5.35 |
| Creatures vs Burn | Creatures | 85% (17/20) | 5.35 |

All three decks' dominant matchup win rates track expectation. **PASS.**

### 4. Combo detector

```bash
python3 scripts/combo_detector.py
```

Result (no errors):

| Family | Count |
|---|---:|
| Mana-positive engines | 1,239 |
| Untap triggers | 213 |
| Untap-on-activation | 58 |
| Storm engines | 1,508 |
| Iterative draw engines | 280 |
| Doublers | 37 |

2-card pair candidates: **375,805**. Top-20 pairs printed to stdout; reports written to `data/rules/combo_detections.md` and `.json`. **PASS.**

### 5. Honest coverage

```bash
python3 scripts/coverage_honest.py
```

Result (on 31,639 cards):

| Category | Count | % |
|---|---:|---:|
| Structural | 7,280 | 23.01% |
| Mixed | 13,167 | 41.62% |
| Stub | 10,831 | 34.23% |
| Vanilla | 361 | 1.14% |

Engine-executable today: **7,641 (24.15%)**. Engine work owed: **23,998 (75.85%)**. Per-card handlers in `per_card.py`: **1,079**. **PASS.**

### 6. §613 layer harness

```bash
python3 scripts/layer_harness.py
```

Cards touching ≥1 layer: **11,475 (36.27%)**. Cards touching ≥2 layers: **1,354 (4.28%)**. Vanilla / spell-only: **20,164 (63.73%)**.

Layer distribution:

| Layer | Cards |
|---|---:|
| 1 (copy) | 6 |
| 2 (control-change) | 76 |
| 3 (text-change) | 0 |
| 4 (type/subtype) | 166 |
| 5 (color) | 1 |
| 6 (abilities) | 8,751 |
| 7a (CDA P/T) | 188 |
| 7b (P/T set) | 47 |
| 7c (P/T modify) | 2,100 |
| 7d (counters) | 1,511 |
| 7e (P/T switch) | 0 |

**PASS.**

---

## Notes for the next agent

- `parser.py`, `mtg_ast.py`, and `extensions/` are **the load-bearing surface**. They are stable enough to be treated as a published interface. The parser is also being actively hardened by a parallel conjunction-fix agent at the time of this audit — expect the structural / mixed / stub numbers to drift upward (more structural, less stub) in the next few hours.
- `playloop.py` is deliberately duplicative of the Go runtime. It exists so new effect-kind dispatchers can be exercised against live game state the same day they land in the parser, without waiting on the Go port. Don't delete the duplication until the Go engine consumes the AST end-to-end.
- `web/replay/` is a glorified JSONL viewer with zero game logic. Keep it that way. If a gameplay question requires interpreting events, implement it in `playloop.py` first, log the result, and let the viewer stay dumb.
- The 3 failing goldens are expected and should be regenerated, not "fixed." Do not try to roll the parser back to re-emit `unknown` — that would be regressing real progress.
- `data/rules/oracle-cards.json` is the ground truth. If a card appears missing, the first check is always that the dump is up-to-date with Scryfall's latest bulk.
