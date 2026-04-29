# MTG Parser Test Suite

Pytest-based regression tests for `scripts/parser.py` and its ~50 extensions
in `scripts/extensions/`.

## What's here

```
tests/
├── card_selection.py          # 200 curated card names grouped by family
├── generate_golden.py         # Re-creates the golden snapshots
├── conftest.py                # Adds scripts/ to sys.path + loads extensions
├── pytest.ini                 # Registers the `slow` marker
├── test_parser_regression.py  # 201 golden-file regression tests
├── test_parser_coverage.py    # 2 whole-corpus sanity tests (slow)
└── golden/                    # 200 JSON snapshots, one per curated card
```

## Running

From the repo root (`sandbox/mtgsquad/`):

```bash
# Full suite (~60s — the coverage test parses all 31,639 cards)
python3 -m pytest tests/

# Fast regression-only loop (~1s)
python3 -m pytest tests/ -m "not slow"

# One family at a time
python3 -m pytest tests/test_parser_regression.py -k cedh
python3 -m pytest tests/test_parser_regression.py -k "sol_ring or doomsday"

# Verbose failure output
python3 -m pytest tests/ -v --tb=long
```

## How it works

**Golden-file regression tests** (`test_parser_regression.py`):

1. `card_selection.py` lists 200 cards grouped into 8 mechanic families:

   | Family    | Count | Purpose                                         |
   |-----------|------:|-------------------------------------------------|
   | `cedh`    |    40 | Format staples — ANY regression here is P0      |
   | `triggers`|    30 | ETB / death / attack / upkeep / cast / etc.     |
   | `keywords`|    30 | Evergreen + deciduous (Flying, Cycling, Echo...)|
   | `layer7`  |    30 | P/T anthems, counters, base-P/T-sets            |
   | `modal`   |    20 | Charms + commands (Cryptic, Kolaghan's, ...)    |
   | `ramp`    |    20 | Fetch-a-land + mana rocks                       |
   | `removal` |    20 | Destroy / Exile / Bounce w/ various filters     |
   | `vanilla` |    10 | Sanity check — zero abilities                   |
   | **total** |**200**|                                                 |

2. `generate_golden.py` parses each card and writes a compact JSON signature:

   ```json
   {
     "name": "Sol Ring",
     "group": "cedh",
     "ability_count": 1,
     "fully_parsed": true,
     "parse_error_count": 0,
     "abilities": [
       {"index": 0, "kind": "activated", "first5": ["act:T", "add_mana"]}
     ]
   }
   ```

   The signature captures: top-level counts, each ability's kind, and the
   first 5 kinds DFS-walked from the effect tree. Enough to detect real
   structural regressions; robust to trivial refactors (doesn't hash raw
   text or full sub-trees).

3. `test_parser_regression.py` parametrizes over every file in `golden/` —
   one test per card. A regression lights up exactly the card(s) that
   changed, not the whole suite.

**Whole-corpus coverage** (`test_parser_coverage.py`, marked `slow`):

- `test_full_corpus_100_percent_green` — parses all 31,639 real cards and
  asserts `green_count == 31639`. Fails loudly if anyone drops coverage.
- `test_full_corpus_no_exceptions` — every card must parse without raising.

## Regenerating goldens (intentional parser changes)

When you deliberately change the parser and accept the new AST shape:

```bash
python3 tests/generate_golden.py              # overwrite goldens
git diff tests/golden/                        # review the diff before committing
python3 -m pytest tests/                      # confirm green
```

To verify goldens are up to date without modifying them (CI-style):

```bash
python3 tests/generate_golden.py --check      # exits non-zero if stale
```

## Adding / swapping cards

Edit `card_selection.py` — add a name to the appropriate group list, re-run
`generate_golden.py`. If a card doesn't exist in `data/rules/oracle-cards.json`,
the generator prints a `MISSING` list and skips it.
