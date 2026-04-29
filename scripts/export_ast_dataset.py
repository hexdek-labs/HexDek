#!/usr/bin/env python3
"""Export the typed CardAST for every real MTG card to a JSONL dataset.

Writes one JSON object per line:
  {
    "name": str,
    "oracle_text": str,
    "type_line": str,
    "mana_cost": str,
    "cmc": float,
    "colors": list[str],
    "ast": nested dict of the CardAST (with __ast_type__ tags per node)
  }

The AST is serialized via `dataclasses.asdict` plus an added `__ast_type__`
field on every node so the consumer can discriminate between otherwise
structurally-similar nodes (e.g. Static vs Keyword vs Activated inside the
`abilities` list, or Damage vs Draw inside `effect`).

Usage:
    python3 scripts/export_ast_dataset.py

Idempotent. Re-running overwrites the output file.
"""

from __future__ import annotations

import dataclasses
import json
import sys
import time
from pathlib import Path

SCRIPTS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS_DIR))

import parser as P  # noqa: E402


ROOT = SCRIPTS_DIR.parent
OUT_JSONL = ROOT / "data" / "rules" / "ast_dataset.jsonl"
OUT_README = ROOT / "data" / "rules" / "ast_dataset_README.md"


def _tag_ast_types(node):
    """Recursively walk a dataclasses.asdict() output and tag each dict
    that originated from a dataclass with its class name under
    `__ast_type__`. We achieve this by walking the original object in
    parallel rather than re-traversing the dict blindly."""
    if dataclasses.is_dataclass(node) and not isinstance(node, type):
        out = {"__ast_type__": type(node).__name__}
        for f in dataclasses.fields(node):
            out[f.name] = _tag_ast_types(getattr(node, f.name))
        return out
    if isinstance(node, (list, tuple)):
        return [_tag_ast_types(x) for x in node]
    if isinstance(node, dict):
        return {k: _tag_ast_types(v) for k, v in node.items()}
    return node


def card_row(card: dict) -> dict:
    ast = P.parse_card(card)
    # Oracle text: prefer top-level, fall back to concatenated faces.
    oracle = card.get("oracle_text") or ""
    if not oracle and card.get("card_faces"):
        oracle = "\n".join(
            (f.get("oracle_text") or "") for f in card["card_faces"]
        ).strip()
    colors = card.get("colors")
    if not colors and card.get("card_faces"):
        seen: list[str] = []
        for f in card["card_faces"]:
            for c in (f.get("colors") or []):
                if c not in seen:
                    seen.append(c)
        colors = seen or []
    return {
        "name": card.get("name", ""),
        "oracle_text": oracle,
        "type_line": card.get("type_line") or "",
        "mana_cost": card.get("mana_cost") or "",
        "cmc": card.get("cmc"),
        "colors": colors or [],
        "ast": _tag_ast_types(ast),
    }


README_TEMPLATE = """---
license: mit
task_categories:
- text2text-generation
- text-classification
language:
- en
tags:
- magic-the-gathering
- mtg
- abstract-syntax-tree
- ast
- games
- structured-generation
size_categories:
- 10K<n<100K
pretty_name: MTG Typed AST Dataset
---

# MTG Oracle-Text Typed AST Dataset

A finetuning-grade corpus of every printed Magic: The Gathering card
(post-filtering: __ROWS__ cards) paired with its **typed abstract syntax tree**
as produced by the [mtgsquad](https://github.com/) oracle-text parser.

Each AST is emitted per the MTG Comprehensive Rules §113 ability taxonomy:

- `Static` — always-on rules text (e.g. *"Flying"*, *"Creatures you control get +1/+1"*)
- `Activated` — cost/effect abilities (e.g. *"{T}: Add {G}"*)
- `Triggered` — When/Whenever/At-the-beginning-of events
- `Keyword` — named shorthand (Flying, Trample, Flashback, ...)

...and ~40 typed effect nodes (`Damage`, `Draw`, `Destroy`, `Exile`, `Bounce`,
`Buff`, `CreateToken`, `AddMana`, `Tutor`, `Reanimate`, `GainLife`, `LoseLife`,
`Discard`, `Mill`, `CounterMod`, `Sequence`, `Choice`, `Optional_`,
`Conditional`, `UnknownEffect`, and more).

Parser syntactic coverage: **100% GREEN** on all __ROWS__ real cards.
A majority of cards carry at least one `Modification(kind="custom", ...)`
stub intended for the runtime layer; that information is preserved in this
dataset so downstream consumers can filter on it. See
`data/rules/coverage_honest.md` in the mtgsquad repository for live
structural / mixed / stub / vanilla breakdown.

## File

- `ast_dataset.jsonl` — one JSON object per card, newline-delimited.

## Schema

```jsonc
{
  "name": "Lightning Bolt",
  "oracle_text": "Lightning Bolt deals 3 damage to any target.",
  "type_line": "Instant",
  "mana_cost": "{R}",
  "cmc": 1,
  "colors": ["R"],
  "ast": {
    "__ast_type__": "CardAST",
    "name": "Lightning Bolt",
    "abilities": [ /* typed ability nodes */ ],
    "parse_errors": [],
    "fully_parsed": true
  }
}
```

Every dataclass-backed AST node carries an `__ast_type__` field naming its
Python class (e.g. `Static`, `Activated`, `Triggered`, `Keyword`, `Damage`,
`Sequence`, `Choice`, ...) so consumers can discriminate structurally-similar
nodes without hand-rolled schema inference.

See `scripts/mtg_ast.py` in the mtgsquad repository for the full type
definitions (frozen dataclasses).

## Example row (verbatim first line of the JSONL)

```json
__EXAMPLE__
```

## Use cases

- **LoRA / full finetuning** of an LLM to emit a typed AST from oracle text
  (use `finetune_pairs.jsonl` in this folder for the instruction-format
  variant).
- **Retrieval augmentation** — cluster cards by AST signature.
- **Rules-engine validation** — round-trip AST → text → AST to measure
  semantic stability of a generated model.
- **Ability classification** — supervised targets are the `__ast_type__`
  tags on each ability node.

## License

Code (parser, AST schema, serializer): **MIT**.

Oracle text: Magic: The Gathering oracle text is a property of Wizards of the
Coast. This dataset is distributed for research / educational purposes under
Wizards' Fan Content Policy. Card text originates from the [Scryfall bulk
data API](https://scryfall.com/docs/api/bulk-data) (oracle-cards dump).
Please credit Scryfall when redistributing:

> Unofficial Fan Content permitted under the Fan Content Policy. Not
> approved/endorsed by Wizards. Portions of the materials used are property of
> Wizards of the Coast. ©Wizards of the Coast LLC.

## Citation

```bibtex
@misc{mtgsquad_ast_dataset,
  title  = {MTG Oracle-Text Typed AST Dataset},
  author = {mtgsquad contributors},
  year   = {2026},
  note   = {Typed AST per MTG Comprehensive Rules §113,
            sourced from Scryfall oracle-cards bulk data.}
}
```

Regenerate with:

```bash
python3 scripts/export_ast_dataset.py
```
"""


def main() -> None:
    t0 = time.time()
    P.load_extensions()
    cards = json.loads(P.ORACLE_DUMP.read_text())
    real = [c for c in cards if P.is_real_card(c)]

    OUT_JSONL.parent.mkdir(parents=True, exist_ok=True)
    first_rows: list[dict] = []
    n = 0
    with OUT_JSONL.open("w", encoding="utf-8") as f:
        for c in real:
            try:
                row = card_row(c)
            except Exception as e:  # defensive; parser is stable but be safe
                print(f"  skip {c.get('name')!r}: {e}", file=sys.stderr)
                continue
            f.write(json.dumps(row, ensure_ascii=False))
            f.write("\n")
            if n < 3:
                first_rows.append(row)
            n += 1

    size = OUT_JSONL.stat().st_size
    example = json.dumps(first_rows[0], ensure_ascii=False) if first_rows else "{}"
    rendered = (
        README_TEMPLATE
        .replace("__ROWS__", f"{n:,}")
        .replace("__EXAMPLE__", example)
    )
    OUT_README.write_text(rendered, encoding="utf-8")

    print(f"wrote {n:,} rows to {OUT_JSONL}")
    print(f"file size: {size:,} bytes ({size / 1024 / 1024:.1f} MiB)")
    print(f"readme:    {OUT_README}")
    print(f"elapsed:   {time.time() - t0:.1f}s")
    print("\n=== first 3 rows (pretty) ===")
    for r in first_rows:
        print(json.dumps(r, ensure_ascii=False, indent=2))
        print("---")


if __name__ == "__main__":
    main()
