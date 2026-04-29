#!/usr/bin/env python3
"""Emit instruction-tuning pairs for the MTG oracle-text → typed-AST task.

Output format (one JSON object per line at
`data/rules/finetune_pairs.jsonl`):

    {
      "instruction": "Parse the oracle text of this Magic: The Gathering
                      card into a typed abstract syntax tree.",
      "input":  "<name> — <type_line> — <mana_cost>\\n<oracle_text>",
      "output": "<JSON of the AST>"
    }

Suitable for LoRA / full-parameter instruction tuning (Alpaca-style).

Idempotent. Re-running overwrites the output file.

The underlying dataset is the same 31,639 real cards covered by
`export_ast_dataset.py` — this script is a schema-restatement, not a
re-parse. (It re-parses for simplicity; total runtime ~17s.)
"""

from __future__ import annotations

import json
import sys
import time
from pathlib import Path

SCRIPTS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS_DIR))

import parser as P  # noqa: E402
from export_ast_dataset import _tag_ast_types  # noqa: E402


ROOT = SCRIPTS_DIR.parent
OUT_JSONL = ROOT / "data" / "rules" / "finetune_pairs.jsonl"

INSTRUCTION = (
    "Parse the oracle text of this Magic: The Gathering card into a "
    "typed abstract syntax tree. The AST follows the ability taxonomy of "
    "the MTG Comprehensive Rules §113 (Static, Activated, Triggered, "
    "Keyword) with typed effect leaves (Damage, Draw, Destroy, Exile, "
    "Bounce, Buff, CreateToken, AddMana, Tutor, Reanimate, GainLife, "
    "LoseLife, Discard, Mill, CounterMod, Sequence, Choice, Optional_, "
    "Conditional, UnknownEffect, and related). Emit compact JSON; every "
    "node includes an __ast_type__ field naming its dataclass."
)


def _oracle(card: dict) -> str:
    text = card.get("oracle_text") or ""
    if not text and card.get("card_faces"):
        text = "\n".join(
            (f.get("oracle_text") or "") for f in card["card_faces"]
        ).strip()
    return text


def build_row(card: dict) -> dict | None:
    ast = P.parse_card(card)
    tagged = _tag_ast_types(ast)

    name = card.get("name") or ""
    type_line = card.get("type_line") or ""
    mana_cost = card.get("mana_cost") or ""
    if not mana_cost and card.get("card_faces"):
        mana_cost = " // ".join(
            (f.get("mana_cost") or "") for f in card["card_faces"]
        ).strip(" /")
    oracle = _oracle(card)

    header_parts = [name]
    if type_line:
        header_parts.append(type_line)
    if mana_cost:
        header_parts.append(mana_cost)
    header = " — ".join(header_parts)
    input_text = f"{header}\n{oracle}".strip()

    return {
        "instruction": INSTRUCTION,
        "input": input_text,
        "output": json.dumps(tagged, ensure_ascii=False, sort_keys=False),
    }


def main() -> None:
    t0 = time.time()
    P.load_extensions()
    cards = json.loads(P.ORACLE_DUMP.read_text())
    real = [c for c in cards if P.is_real_card(c)]

    # Deterministic ordering: name-sort the cards so the JSONL diff is
    # stable across runs even if Scryfall changes its bulk-dump order.
    real.sort(key=lambda c: (c.get("name") or ""))

    OUT_JSONL.parent.mkdir(parents=True, exist_ok=True)
    first_rows: list[dict] = []
    n = 0
    with OUT_JSONL.open("w", encoding="utf-8") as f:
        for c in real:
            try:
                row = build_row(c)
            except Exception as e:
                print(f"  skip {c.get('name')!r}: {e}", file=sys.stderr)
                continue
            if row is None:
                continue
            f.write(json.dumps(row, ensure_ascii=False))
            f.write("\n")
            if n < 3:
                first_rows.append(row)
            n += 1

    size = OUT_JSONL.stat().st_size
    print(f"wrote {n:,} instruction pairs to {OUT_JSONL}")
    print(f"file size: {size:,} bytes ({size / 1024 / 1024:.1f} MiB)")
    print(f"elapsed:   {time.time() - t0:.1f}s")
    print("\n=== first 3 rows (pretty) ===")
    for r in first_rows:
        print(json.dumps(r, ensure_ascii=False, indent=2))
        print("---")


if __name__ == "__main__":
    main()
