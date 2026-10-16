#!/usr/bin/env python3
"""r135 regression — Adventurer / Omen halves (CR §715.2, §720.2).

Before r135 parser.normalize() joined card_faces[*].oracle_text with "\n"
and parse_card() parsed the concatenation as ONE card. Two consequences,
both measured on the live 38,754-card corpus:

  1. The half's instant/sorcery text became top-level abilities of the
     PARENT. Faerie Guidemother (a 1/1 flier) carried "target creature
     gets +2/+1 and gains flying until end of turn" as its own ability.
     171 such abilities across 153 cards.

  2. No Keyword(name="adventure"|"omen") was ever emitted, because those
     words appear ONLY in the half's TYPE LINE — never in oracle text.
     gameengine.HasAdventure / HasOmen gate on exactly that node, so both
     were false for every card in the corpus (0 and 0), which in turn made
     CastAdventure and CastOmen unreachable: neither has a production
     caller to this day.

Run:  python3 -I scripts/test_adventure_faces_r135.py
Exits non-zero on failure. No corpus needed — fixtures are inline.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import parser as P  # noqa: E402

FAILURES: list[str] = []


def check(cond: bool, msg: str) -> None:
    if not cond:
        FAILURES.append(msg)


# Scryfall shape, trimmed to the fields the parser reads. Face 0 is the
# card proper and face 1 the half — verified 170/170 on the live corpus.
ADVENTURE_CARD = {
    "name": "Faerie Guidemother // Gift of the Fae",
    "layout": "adventure",
    "type_line": "Creature — Faerie // Sorcery — Adventure",
    "mana_cost": "",
    "card_faces": [
        {
            "name": "Faerie Guidemother",
            "type_line": "Creature — Faerie",
            "mana_cost": "{W}",
            "oracle_text": "Flying",
        },
        {
            "name": "Gift of the Fae",
            "type_line": "Sorcery — Adventure",
            "mana_cost": "{1}{W}",
            "oracle_text": "Target creature gets +2/+1 and gains flying "
                           "until end of turn.",
        },
    ],
}

OMEN_CARD = {
    "name": "Marang River Regent // Coil and Catch",
    "layout": "adventure",
    "type_line": "Creature — Dragon // Instant — Omen",
    "mana_cost": "",
    "card_faces": [
        {
            "name": "Marang River Regent",
            "type_line": "Creature — Dragon",
            "mana_cost": "{4}{U}{U}",
            "oracle_text": "Flying",
        },
        {
            "name": "Coil and Catch",
            "type_line": "Instant — Omen",
            "mana_cost": "{3}{U}",
            "oracle_text": "Draw three cards, then discard a card.",
        },
    ],
}

# A transform DFC: r135 deliberately does NOT touch these. Its faces are
# still concatenated and it must gain no `faces` and no keyword.
TRANSFORM_CARD = {
    "name": "Delver of Secrets // Insectile Aberration",
    "layout": "transform",
    "type_line": "Creature — Human Wizard",
    "card_faces": [
        {"name": "Delver of Secrets", "type_line": "Creature — Human Wizard",
         "mana_cost": "{U}",
         "oracle_text": "At the beginning of your upkeep, look at the top "
                        "card of your library."},
        {"name": "Insectile Aberration", "type_line": "Creature — Human Insect",
         "mana_cost": "", "oracle_text": "Flying"},
    ],
}


def keywords(ast) -> dict:
    return {a.name: a for a in ast.abilities if type(a).__name__ == "Keyword"}


def main() -> int:
    P.load_extensions()

    # --- Adventure -------------------------------------------------------
    ast = P.parse_card(ADVENTURE_CARD)
    kws = keywords(ast)

    check("adventure" in kws,
          "no Keyword(name='adventure') emitted — HasAdventure stays false "
          "and CastAdventure stays unreachable")
    if "adventure" in kws:
        check(kws["adventure"].args == ("{1}{W}",),
              f"adventure keyword should carry the half's mana cost, "
              f"got {kws['adventure'].args!r}")

    check(len(ast.faces) == 1,
          f"the Adventure half should be its own CardAST, got "
          f"{len(ast.faces)} faces")
    if ast.faces:
        check(ast.faces[0].name == "Gift of the Fae",
              f"half name: got {ast.faces[0].name!r}")
        check(len(ast.faces[0].abilities) >= 1,
              "the half parsed to zero abilities — its text was dropped "
              "rather than relocated")

    # The other direction: the parent must NOT have absorbed the half.
    parent_raw = " ".join(getattr(a, "raw", "") for a in ast.abilities).lower()
    check("+2/+1" not in parent_raw,
          "the parent creature still carries the half's buff text — this is "
          "the pre-r135 concatenation bug (171 abilities across 153 cards)")
    check("flying" in kws,
          "the parent lost its OWN printed keyword 'flying' — "
          "de-concatenation took too much")

    # --- Omen ------------------------------------------------------------
    omen_ast = P.parse_card(OMEN_CARD)
    omen_kws = keywords(omen_ast)
    check("omen" in omen_kws,
          "no Keyword(name='omen') emitted — HasOmen stays false for all "
          "13 omen cards and CastOmen stays unreachable")
    if "omen" in omen_kws:
        check(omen_kws["omen"].args == ("{3}{U}",),
              f"omen keyword should carry the half's mana cost for OmenCost, "
              f"got {omen_kws['omen'].args!r}")
    check(len(omen_ast.faces) == 1,
          f"the Omen half should be its own CardAST, got "
          f"{len(omen_ast.faces)} faces")

    # --- Must-never: other multi-face layouts are untouched ---------------
    tr = P.parse_card(TRANSFORM_CARD)
    check(len(tr.faces) == 0,
          "a transform DFC gained faces — r135 is scoped to "
          "layout='adventure' only; transform/modal_dfc/split/flip each "
          "need their own pass")
    check("adventure" not in keywords(tr) and "omen" not in keywords(tr),
          "a transform DFC gained an adventure/omen keyword")

    if FAILURES:
        print(f"FAIL ({len(FAILURES)}):")
        for f in FAILURES:
            print("  -", f)
        return 1
    print("PASS — adventure + omen halves parse separately, keywords emitted, "
          "other layouts untouched")
    return 0


if __name__ == "__main__":
    sys.exit(main())
