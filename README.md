# mtgsquad

A Magic: the Gathering platform-in-progress whose design goal is that AI agents and humans sit at the same table as first-class peers, against a game engine driven by a typed AST compiled from every printed card.

---

## What this repository is

This is a **parser and engine scaffold**, not a playable client. It consists of three concentric rings:

1. **An oracle-text parser** (`scripts/parser.py`, ~98K lines including extensions). It ingests every printed Magic card from a Scryfall bulk dump and emits a typed AST per comprehensive rules §113 (Static / Activated / Triggered / Keyword).
2. **A layer harness** (`scripts/layer_harness.py`). It walks every card's AST and tags every static-ability `Modification` with its §613 continuous-effects layer so a future runtime can resolve effects in the correct order without re-deriving.
3. **A runtime engine** (`internal/game/` in Go). Today this handles turn structure, priority, mana, casting, and simple actions. It is **not** yet capable of consuming the AST end-to-end, and it is **not** yet AI-playable.

The cage around all three is an HTTP/WebSocket server (`cmd/mtgsquad-server/`) with SQLite for ephemeral game state, Moxfield-format deck import, and `crypto/rand`-backed Fisher-Yates shuffle with a commit-reveal option for trustless shuffle attestation.

---

## Status (honest)

| Layer | Status |
|---|---|
| Parser syntactic coverage | **100%** on 31,639 real cards — every card returns an AST with no parse errors. |
| AST engine-executability | **~24%** — cards whose AST is fully typed leaf nodes a runtime can execute from node kinds alone. See `data/rules/coverage_honest.md` for the live number. |
| AST stub coverage | **~76%** — cards parse, but some or all abilities carry `Modification(kind="custom", args=(slug,))` placeholders that will need hand-coded resolvers in the runtime layer. |
| Per-card handlers | 1,079 snowflake cards (`scripts/extensions/per_card.py`) — intentional stubs, a work queue for the runtime. |
| Layer tagging | Every card's abilities tagged against §613 layers 1–7e. |
| Python self-play loop | 3-matchup (Burn / Control / Creatures), full turn structure including combat (`scripts/playloop.py`). |
| Regression test suite | 203 pytest tests — 201 golden card snapshots + 2 full-corpus coverage asserts. |
| Go runtime engine | Basic: turn structure, mana, casting, simple actions. No stack-based resolution against the AST yet. Not AI-playable. |
| Client UI | Stub (`web/`) + static replay viewer (`web/replay/`). |

Read `data/rules/coverage_honest.md` for the full dual-metric breakdown. The short version: "we parsed every printed Magic card" is true and load-bearing; "we can play every printed Magic card" is not yet true.

For architectural detail on how the parser, AST, extensions, layer harness, and runtime fit together, see **[ARCHITECTURE.md](./ARCHITECTURE.md)**.

---

## Quick start

The parser and harness are pure Python 3.11+ and have no runtime dependencies beyond stdlib.

```bash
# From sandbox/mtgsquad/
cd sandbox/mtgsquad

# 1. Parse every card, report coverage, regenerate data/rules/parser_coverage.md
python3 scripts/parser.py

# 2. Parse one card and dump its AST
python3 scripts/parser.py --card "Snapcaster Mage"

# 3. Produce the honest dual-metric coverage report
#    Writes data/rules/coverage_honest.md
python3 scripts/coverage_honest.py

# 4. Run the §613 layer harness across every card
#    Writes data/rules/layer_harness.md
python3 scripts/layer_harness.py
```

The Go runtime and HTTP server:

```bash
# Build and run the server (listens on :8080)
go run ./cmd/mtgsquad-server

# Example endpoint (test deck, top-N of a freshly shuffled library)
curl http://localhost:8080/game/test/library/top-3
```

The server is scaffolding — do not treat it as a playable product.

---

## Self-play loop

A Python-only structural self-play simulator that proves the parser AST can drive
an end-to-end game loop. It runs three matchups (Burn / Control / Creatures),
with a real turn structure including a combat phase (declare-attackers,
declare-blockers, first-strike step, regular damage, state-based actions,
keyword handling for flying, reach, first strike, double strike, trample,
deathtouch, lifelink, vigilance, menace, defender, haste).

```bash
# 3 matchups × 20 games each, deterministic seed
python3 scripts/playloop.py --games 20 --seed 42

# More games, with per-game log spill
python3 scripts/playloop.py --games 100
```

Report: `data/rules/playloop_report.md`.
Sample game logs: `data/rules/playloop_sample_log.txt`.

---

## Replay viewer

A zero-dependency static browser viewer for playloop logs. Open
`web/replay/index.html` in a browser and drag-and-drop a `.jsonl` log (or use
the bundled `web/replay/sample.jsonl`). Scrub / step / autoplay through events;
no game logic runs in the viewer — it only plays back logged state deltas.

---

## Combo detector

Static-analysis scanner that surfaces infinite-combo and engine candidates from
the parsed AST (mana-positive engines, untap triggers, storm engines, iterative
draw engines, doublers, and 2-card pair candidates). Heuristic — every entry is
a candidate for human review, not a solved combo.

```bash
python3 scripts/combo_detector.py
```

Writes `data/rules/combo_detections.md` (human-readable) and
`data/rules/combo_detections.json` (programmatic).

---

## AST dataset export

For downstream finetuning / analysis, every card's typed AST is exported as a
JSONL dataset with `__ast_type__` tags on every dataclass node.

```bash
# Full dataset — ~40 MiB, 31,639 rows
python3 scripts/export_ast_dataset.py

# Alpaca-style instruction pairs (oracle text → AST JSON)
python3 scripts/export_finetune_pairs.py
```

Outputs:
- `data/rules/ast_dataset.jsonl`
- `data/rules/ast_dataset_README.md` (Hugging Face-ready card)
- `data/rules/finetune_pairs.jsonl`

---

## Tests

Pytest suite of 201 golden-file regressions (one test per curated card across
8 mechanic families) plus 2 slow whole-corpus asserts.

```bash
# Fast regression-only tier
python3 -m pytest tests/ -m "not slow"

# Full suite (parses all 31,639 cards)
python3 -m pytest tests/
```

See `tests/README.md` for the breakdown and regeneration workflow.

---

## Repository layout

```
mtgsquad/
├── README.md                   # this file
├── ARCHITECTURE.md             # parser → AST → extensions → harness → runtime
├── go.mod / go.sum
├── cmd/
│   └── mtgsquad-server/        # HTTP/WebSocket entry point
├── internal/
│   ├── game/                   # Go runtime engine (basic; consumes AST in future)
│   ├── shuffle/                # crypto/rand Fisher-Yates
│   ├── moxfield/               # deck import
│   ├── oracle/                 # card lookup / Scryfall integration
│   ├── db/                     # SQLite ephemeral game state
│   ├── ws/                     # WebSocket hub
│   └── ai/, auth/, mana/, party/, rules/
├── scripts/
│   ├── parser.py               # oracle text → CardAST
│   ├── mtg_ast.py              # typed frozen-dataclass schema (§113 abilities)
│   ├── layer_harness.py        # §613 layer tagging
│   ├── coverage_honest.py      # dual-metric reporter
│   ├── rules_coverage.py       # detailed parser diagnostics
│   ├── cluster_priority.py     # prioritize stub work by card-usage frequency
│   ├── semantic_clusters.py    # AST-signature clustering
│   ├── combo_detector.py       # static-analysis infinite-combo scanner
│   ├── playloop.py             # Python self-play loop (3 matchups, combat)
│   ├── export_ast_dataset.py   # dumps typed AST to JSONL
│   ├── export_finetune_pairs.py# Alpaca-style instruction pairs
│   ├── extensions/             # ~50 parser-extension modules
│   └── patterns/               # additional grammar patterns
├── tests/                      # pytest suite (203 tests)
│   ├── card_selection.py       # 200 curated cards grouped by family
│   ├── generate_golden.py      # regenerates golden/*.json
│   ├── test_parser_regression.py  # 201 golden-file snapshots
│   ├── test_parser_coverage.py    # 2 whole-corpus asserts (slow)
│   └── golden/                 # 200 JSON signatures
├── data/
│   ├── rules/
│   │   ├── MagicCompRules-20260227.txt   # source of truth
│   │   ├── oracle-cards.json             # Scryfall bulk dump (gitignored)
│   │   ├── parser_coverage.md            # produced by parser.py
│   │   ├── coverage_honest.md            # produced by coverage_honest.py
│   │   ├── layer_harness.md              # produced by layer_harness.py
│   │   ├── combo_detections.{md,json}    # produced by combo_detector.py
│   │   ├── ast_dataset.jsonl             # produced by export_ast_dataset.py
│   │   ├── ast_dataset_README.md         # dataset card (HF-ready)
│   │   ├── finetune_pairs.jsonl          # produced by export_finetune_pairs.py
│   │   ├── playloop_report.md            # produced by playloop.py
│   │   ├── playloop_sample_log.txt
│   │   ├── cluster_priority.md
│   │   ├── semantic_clusters.md
│   │   └── partial_diagnostic.md
│   ├── decks/                            # example decks (Moxfield JSON)
│   └── mtgsquad.db                       # SQLite; ephemeral game state
└── web/                                  # client stub (not yet wired)
    └── replay/                           # static browser viewer for playloop logs
```

---

## Design principles

1. **AI is a first-class player.** The same authoritative server, the same fairness constraints. AI connects via a JSON API scoped by session token; humans connect via a UI that speaks the same protocol. Neither can query the other's hidden zones, because the token doesn't authorize that scope.
2. **Typed AST, not string matching.** Every card's oracle text compiles to a typed tree the runtime can pattern-match against. No regex-in-the-hot-path.
3. **Honest coverage reporting.** "Parsed" and "executable" are separate metrics. See `data/rules/coverage_honest.md`.
4. **Structural cheating is impossible.** Session-scoped data access; no client ever touches another player's private state.
5. **True-RNG shuffle with cryptographic provenance.** `crypto/rand` Fisher-Yates. Optional commit-reveal for trustless attestation.
6. **Free decks.** Moxfield import, proxy-friendly. No card cost.

---

## Legal posture

This project is open source. It ships no card images and no copyrighted card text in this repository — oracle text is pulled at runtime from Scryfall's bulk-data dump, which is the same pattern every major MTG tool (Scryfall, Moxfield, EDHREC, Archidekt) has operated under for years without incident. The runtime engine implements the comprehensive rules as a distributed system, not as Wizards IP.

We do not host paid play. We do not sell a client. We do not bundle artwork.

Wizards of the Coast, *Magic: the Gathering*, card names, and card artwork are property of Wizards of the Coast LLC. This project is not affiliated with or endorsed by Wizards.

---

## Monetization

Donations only. Planned channels: GitHub Sponsors, Patreon, OpenCollective. **No advertising, no paid features, no premium tiers, no data monetization.** If the project needs money to run infrastructure, it will ask directly.

---

## Prior art and credits

- **[Scryfall](https://scryfall.com/)** — bulk card data and the operational precedent that an open MTG data tool is viable.
- **[Magic: the Gathering Comprehensive Rules](https://magic.wizards.com/en/rules)** — the spec this project compiles against (`data/rules/MagicCompRules-20260227.txt`).
- **[XMage](https://github.com/magefree/mage)** and **[Forge](https://github.com/Card-Forge/forge)** — long-running FOSS MTG engines in Java. Both proved that "implement every card" is a tractable problem at the right level of abstraction. mtgsquad's bet is that a typed AST plus a smaller custom-resolver surface beats a per-card class hierarchy for long-term maintainability and AI agent integration.

---

## License

MIT.
