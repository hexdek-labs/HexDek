"""Pytest configuration — makes sibling `scripts/` importable AND loads all
parser extensions before any test parses a card.

The parser + AST live in `scripts/` and expect to be imported bare
(``from parser import parse_card``). Tests live in `tests/`, so we inject
`scripts/` onto sys.path before collection.

CRITICAL: ``parser.load_extensions()`` must run before parse_card, otherwise
the ~50 extensions in scripts/extensions/ are not registered and coverage
drops from 100% to ~41%. The production CLI calls this in its main(); we have
to replicate it here.
"""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
TESTS = Path(__file__).resolve().parent

for p in (str(SCRIPTS), str(TESTS)):
    if p not in sys.path:
        sys.path.insert(0, p)

# Load extensions once, at import time, so every test sees the fully-wired
# parser. Safe to call more than once (the registry lists just re-append
# duplicate rules, which is a no-op for correctness).
import parser as _parser  # noqa: E402

_parser.load_extensions()
