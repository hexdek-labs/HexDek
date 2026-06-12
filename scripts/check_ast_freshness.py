#!/usr/bin/env python3
"""Freshness gate for data/rules/ast_dataset.jsonl.

The export script (export_ast_dataset.py) stamps the dataset's first line
with a DatasetManifest carrying `parser_source_hash` — a SHA-256 over
parser.py + mtg_ast.py + scripts/extensions/*.py at export time. This
script recomputes that hash from the live tree and compares.

Exit codes:
  0 — dataset is fresh (hashes match)
  1 — dataset is STALE (parser changed since export) or has no manifest
  2 — dataset file missing

Run manually, from CI, or as a pre-commit hook:

    python3 scripts/check_ast_freshness.py [path/to/ast_dataset.jsonl]

Background: r61 review (report 07, C6) found the shipped dataset was
generated 2026-05-16 while parser.py was last edited 2026-05-22 — live
drift with no detection. Every parser fix made after an export is absent
from the data the engine actually runs until someone remembers to
regenerate; this gate makes forgetting loud.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

SCRIPTS_DIR = Path(__file__).resolve().parent
ROOT = SCRIPTS_DIR.parent
DEFAULT_DATASET = ROOT / "data" / "rules" / "ast_dataset.jsonl"

sys.path.insert(0, str(SCRIPTS_DIR))


def main() -> int:
    dataset = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_DATASET
    if not dataset.exists():
        print(f"STALE-CHECK: dataset not found: {dataset}", file=sys.stderr)
        return 2

    with dataset.open("r", encoding="utf-8") as f:
        first = f.readline()
    try:
        head = json.loads(first)
    except json.JSONDecodeError:
        head = {}

    if head.get("__ast_type__") != "DatasetManifest":
        print(
            "STALE: dataset has no DatasetManifest header line — it predates "
            "the freshness stamp. Regenerate with "
            "`python3 scripts/export_ast_dataset.py`.",
            file=sys.stderr,
        )
        return 1

    from export_ast_dataset import _parser_source_hash  # noqa: E402

    live = _parser_source_hash()
    stamped = head.get("parser_source_hash", "")
    if live != stamped:
        print(
            "STALE: parser source tree has changed since the dataset was "
            f"exported on {head.get('export_date', '?')}.\n"
            f"  stamped: {stamped[:16]}…\n"
            f"  live:    {live[:16]}…\n"
            "Regenerate with `python3 scripts/export_ast_dataset.py`.",
            file=sys.stderr,
        )
        return 1

    print(
        f"FRESH: dataset exported {head.get('export_date', '?')} "
        f"({head.get('entry_count', '?')} entries) matches live parser "
        f"source hash {live[:16]}…"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
