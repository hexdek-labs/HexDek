#!/usr/bin/env python3
"""Generate golden files for the parser regression suite.

For each card in card_selection.py:
  1. Look up the card in oracle-cards.json.
  2. Run parser.parse_card(card) to get an AST.
  3. Compute a compact "signature": ability count + first 5 kind-tags for each
     ability's effect tree.
  4. Save as JSON in tests/golden/<safe_name>.json.

This is idempotent — re-running with no parser changes produces byte-identical
golden files. When the parser is modified, review the diff before committing.

Usage:
    python3 tests/generate_golden.py            # (re)generate all golden files
    python3 tests/generate_golden.py --check    # fail if any file would change
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
sys.path.insert(0, str(SCRIPTS))
sys.path.insert(0, str(Path(__file__).resolve().parent))

import parser as _parser_module  # noqa: E402

_parser_module.load_extensions()  # register all ~50 extensions
from parser import parse_card  # noqa: E402
from mtg_ast import (  # noqa: E402
    Activated,
    Choice,
    Conditional,
    Keyword,
    Optional_,
    Sequence,
    Static,
    Triggered,
)
from card_selection import all_cards, GROUPS  # noqa: E402

ORACLE = ROOT / "data" / "rules" / "oracle-cards.json"
GOLDEN_DIR = Path(__file__).resolve().parent / "golden"


def _flatten_effect_kinds(e, out: list[str]) -> None:
    """Walk an effect tree and append kinds in DFS order."""
    if e is None:
        return
    if isinstance(e, Sequence):
        out.append("sequence")
        for item in e.items:
            _flatten_effect_kinds(item, out)
        return
    if isinstance(e, Choice):
        out.append(f"choice:{e.pick}")
        for opt in e.options:
            _flatten_effect_kinds(opt, out)
        return
    if isinstance(e, Optional_):
        out.append("optional")
        _flatten_effect_kinds(e.body, out)
        return
    if isinstance(e, Conditional):
        out.append("conditional")
        _flatten_effect_kinds(e.body, out)
        return
    # Leaf effect
    out.append(getattr(e, "kind", e.__class__.__name__.lower()))


def _ability_kinds(ab) -> list[str]:
    """Return a flat list of kinds representing this ability's shape."""
    kinds: list[str] = []
    if isinstance(ab, Keyword):
        kinds.append(f"keyword:{ab.name}")
    elif isinstance(ab, Static):
        mod = ab.modification.kind if ab.modification else "_"
        cond = ab.condition.kind if ab.condition else "_"
        kinds.append(f"static:{mod}:{cond}")
    elif isinstance(ab, Activated):
        cost_tags = []
        c = ab.cost
        if c.tap: cost_tags.append("T")
        if c.untap: cost_tags.append("Q")
        if c.mana: cost_tags.append(f"mana{c.mana.cmc}")
        if c.sacrifice: cost_tags.append("sac")
        if c.pay_life: cost_tags.append(f"life{c.pay_life}")
        if c.discard: cost_tags.append("discard")
        if c.exile_self: cost_tags.append("exile_self")
        kinds.append("act:" + "+".join(cost_tags) if cost_tags else "act:free")
        _flatten_effect_kinds(ab.effect, kinds)
    elif isinstance(ab, Triggered):
        kinds.append(f"trig:{ab.trigger.event}")
        _flatten_effect_kinds(ab.effect, kinds)
    else:
        kinds.append(ab.kind if hasattr(ab, "kind") else ab.__class__.__name__.lower())
    return kinds


def compute_signature(ast) -> dict:
    """Build the golden-file payload for a parsed card.

    Shape:
        {
          "name": "Sol Ring",
          "fully_parsed": true,
          "ability_count": 1,
          "parse_error_count": 0,
          "abilities": [
              {"index": 0, "kind": "activated",
               "first5": ["act:T", "add_mana"]}
          ]
        }
    """
    out = {
        "name": ast.name,
        "fully_parsed": ast.fully_parsed,
        "ability_count": len(ast.abilities),
        "parse_error_count": len(ast.parse_errors),
        "abilities": [],
    }
    # Abilities in canonical (source) order — NOT sorted, because order can be
    # semantically meaningful (e.g. Static 1 vs Static 2 buffs stacking).
    for idx, ab in enumerate(ast.abilities):
        kinds = _ability_kinds(ab)
        out["abilities"].append({
            "index": idx,
            "kind": ab.kind,
            "first5": kinds[:5],
        })
    return out


def safe_filename(name: str) -> str:
    s = name.lower()
    s = re.sub(r"[^a-z0-9]+", "_", s).strip("_")
    return s


def load_cards() -> dict[str, dict]:
    print(f"Loading oracle dump from {ORACLE} ...", file=sys.stderr)
    with open(ORACLE, encoding="utf-8") as f:
        cards = json.load(f)
    return {c["name"]: c for c in cards}


def group_for(name: str) -> str:
    for g, names in GROUPS.items():
        if name in names:
            return g
    return "misc"


def generate(check: bool = False) -> int:
    by_name = load_cards()
    GOLDEN_DIR.mkdir(parents=True, exist_ok=True)

    selection = all_cards()
    missing: list[str] = []
    written = 0
    unchanged = 0
    changed: list[str] = []

    for group, card_name in selection:
        card = by_name.get(card_name)
        if card is None:
            missing.append(card_name)
            continue
        ast = parse_card(card)
        payload = compute_signature(ast)
        payload["group"] = group

        fname = GOLDEN_DIR / f"{safe_filename(card_name)}.json"
        new_text = json.dumps(payload, indent=2, sort_keys=True) + "\n"
        if fname.exists():
            old_text = fname.read_text(encoding="utf-8")
            if old_text == new_text:
                unchanged += 1
                continue
            changed.append(card_name)
        if check:
            continue
        fname.write_text(new_text, encoding="utf-8")
        written += 1

    total = len(selection) - len(missing)
    print(f"selected={len(selection)} resolved={total} "
          f"written={written} unchanged={unchanged} "
          f"missing={len(missing)}", file=sys.stderr)
    if missing:
        print("MISSING cards (not in oracle dump):", file=sys.stderr)
        for m in missing:
            print(f"  - {m}", file=sys.stderr)

    if check and (written or changed):
        print(f"CHECK FAILED — {len(changed)} goldens out of date:",
              file=sys.stderr)
        for c in changed:
            print(f"  - {c}", file=sys.stderr)
        return 1
    return 0


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true",
                    help="Exit non-zero if any golden file needs updating.")
    args = ap.parse_args()
    return generate(check=args.check)


if __name__ == "__main__":
    sys.exit(main())
