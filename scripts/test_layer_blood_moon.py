#!/usr/bin/env python3
"""Layer-system verification test: Blood Moon / Magus of the Moon.

Tests:
  1. Blood Moon on a standard non-basic land (Underground Sea) converts
     its subtypes to ["Mountain"], preserves the Land type.
  2. Blood Moon does NOT affect Basic lands.
  3. Dryad Arbor (CR §305.7 edge case): "Land Creature — Forest Dryad"
     becomes "Land Creature — Mountain Dryad" — Forest is a land subtype
     that's REPLACED; Dryad is a creature subtype that's PRESERVED.
  4. Blood Moon + Magus of the Moon stack without conflict (both apply
     layer 4; idempotent composition).
  5. Blood Moon leaves the battlefield → effects unregister, the non-
     basic lands revert to their printed subtypes.

Run standalone:
    python3 scripts/test_layer_blood_moon.py
"""

from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from mtg_ast import CardAST  # noqa: E402
import playloop as pl  # noqa: E402


def _card(name: str, type_line: str, cmc: int = 0,
          power=None, toughness=None) -> pl.CardEntry:
    ast = CardAST(name=name, abilities=(), parse_errors=(),
                  fully_parsed=True)
    return pl.CardEntry(
        name=name, mana_cost=f"{{{cmc}}}", cmc=cmc,
        type_line=type_line, oracle_text="",
        power=power, toughness=toughness, ast=ast, colors=(),
    )


def _place(game: pl.Game, seat_idx: int,
           card: pl.CardEntry) -> pl.Permanent:
    perm = pl.Permanent(card=card, controller=seat_idx,
                        tapped=False, summoning_sick=False)
    perm.timestamp = game.next_timestamp()
    game.seats[seat_idx].battlefield.append(perm)
    return perm


def _check(desc: str, cond: bool, detail: str = "") -> tuple:
    return (desc, cond, detail)


def test_blood_moon_on_dual_land() -> list:
    """Blood Moon + Underground Sea → Mountain (typeline subtype swap)."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    usea = _card("Underground Sea", "Land — Island Swamp")
    bmoon = _card("Blood Moon", "Enchantment", cmc=3)
    p_usea = _place(g, 1, usea)
    p_bmoon = _place(g, 0, bmoon)

    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, bmoon, "blood_moon_layer_4",
                    {"permanent": p_bmoon})

    g.invalidate_characteristics_cache()
    chars = pl.get_effective_characteristics(g, p_usea)
    out.append(_check("Blood Moon registered 1 L4 effect",
                      len(g.continuous_effects) == 1,
                      f"count={len(g.continuous_effects)}"))
    out.append(_check("Underground Sea is now a Mountain",
                      "mountain" in [s.lower() for s in chars.get("subtypes", [])],
                      f"subtypes={chars.get('subtypes')}"))
    out.append(_check("Underground Sea no longer Island",
                      "island" not in [s.lower() for s in chars.get("subtypes", [])],
                      f"subtypes={chars.get('subtypes')}"))
    out.append(_check("Underground Sea no longer Swamp",
                      "swamp" not in [s.lower() for s in chars.get("subtypes", [])],
                      f"subtypes={chars.get('subtypes')}"))
    out.append(_check("Underground Sea still has Land type",
                      "land" in chars.get("types", []),
                      f"types={chars.get('types')}"))
    return out


def test_blood_moon_ignores_basics() -> list:
    """Basic lands are NOT affected by Blood Moon (predicate gates)."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    forest = _card("Forest", "Basic Land — Forest")
    bmoon = _card("Blood Moon", "Enchantment", cmc=3)
    p_forest = _place(g, 1, forest)
    p_bmoon = _place(g, 0, bmoon)

    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, bmoon, "blood_moon_layer_4",
                    {"permanent": p_bmoon})

    g.invalidate_characteristics_cache()
    chars = pl.get_effective_characteristics(g, p_forest)
    out.append(_check("Basic Forest still has Forest subtype",
                      "forest" in [s.lower() for s in chars.get("subtypes", [])],
                      f"subtypes={chars.get('subtypes')}"))
    out.append(_check("Basic Forest is NOT a Mountain",
                      "mountain" not in [s.lower() for s in chars.get("subtypes", [])],
                      f"subtypes={chars.get('subtypes')}"))
    return out


def test_dryad_arbor_cr_305_7() -> list:
    """CR §305.7 edge case — Dryad Arbor is "Land Creature — Forest
    Dryad". Blood Moon removes Forest, adds Mountain; preserves
    Creature type and Dryad subtype. Result: "Land Creature —
    Mountain Dryad"."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    arbor = _card("Dryad Arbor", "Land Creature — Forest Dryad",
                  cmc=0, power=1, toughness=1)
    bmoon = _card("Blood Moon", "Enchantment", cmc=3)
    p_arbor = _place(g, 1, arbor)
    p_bmoon = _place(g, 0, bmoon)

    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, bmoon, "blood_moon_layer_4",
                    {"permanent": p_bmoon})

    g.invalidate_characteristics_cache()
    chars = pl.get_effective_characteristics(g, p_arbor)
    subs = [s.lower() for s in chars.get("subtypes", [])]
    out.append(_check("Dryad Arbor still a Land",
                      "land" in chars.get("types", []),
                      f"types={chars.get('types')}"))
    out.append(_check("Dryad Arbor still a Creature",
                      "creature" in chars.get("types", []),
                      f"types={chars.get('types')}"))
    out.append(_check("Dryad Arbor became a Mountain",
                      "mountain" in subs,
                      f"subtypes={chars.get('subtypes')}"))
    out.append(_check("Dryad Arbor's Forest subtype removed (§305.7)",
                      "forest" not in subs,
                      f"subtypes={chars.get('subtypes')}"))
    out.append(_check("Dryad Arbor preserves Dryad creature subtype "
                      "(§305.7)",
                      "dryad" in subs,
                      f"subtypes={chars.get('subtypes')}"))
    # P/T preserved (no L7b effect).
    out.append(_check("Dryad Arbor P/T unchanged 1/1",
                      chars["power"] == 1 and chars["toughness"] == 1,
                      f"got {chars['power']}/{chars['toughness']}"))
    return out


def test_blood_moon_plus_magus() -> list:
    """Blood Moon + Magus of the Moon: both layer-4, timestamp-ordered.
    Idempotent composition — subtypes end up [Mountain]."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    usea = _card("Volcanic Island", "Land — Island Mountain")
    bmoon = _card("Blood Moon", "Enchantment", cmc=3)
    magus = _card("Magus of the Moon", "Creature — Human Wizard",
                  cmc=3, power=2, toughness=2)
    p_usea = _place(g, 1, usea)
    p_bmoon = _place(g, 0, bmoon)
    p_magus = _place(g, 0, magus)

    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, bmoon, "blood_moon_layer_4",
                    {"permanent": p_bmoon})
    dispatch_custom(g, 0, magus, "magus_of_the_moon_layer_4",
                    {"permanent": p_magus})

    g.invalidate_characteristics_cache()
    chars = pl.get_effective_characteristics(g, p_usea)
    subs = [s.lower() for s in chars.get("subtypes", [])]
    out.append(_check("2 L4 continuous effects registered",
                      len([ce for ce in g.continuous_effects
                           if ce.layer == "4"]) == 2,
                      f"effects={len(g.continuous_effects)}"))
    out.append(_check("Volcanic Island → [Mountain] only",
                      subs == ["mountain"],
                      f"subtypes={chars.get('subtypes')}"))
    return out


def test_blood_moon_leaves_battlefield() -> list:
    """Blood Moon leaves the battlefield → unregister → subtypes revert."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    usea = _card("Underground Sea", "Land — Island Swamp")
    bmoon = _card("Blood Moon", "Enchantment", cmc=3)
    p_usea = _place(g, 1, usea)
    p_bmoon = _place(g, 0, bmoon)

    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, bmoon, "blood_moon_layer_4",
                    {"permanent": p_bmoon})
    g.invalidate_characteristics_cache()
    chars_on = pl.get_effective_characteristics(g, p_usea)
    out.append(_check("While Blood Moon is in play, Underground Sea is "
                      "Mountain",
                      "mountain" in [s.lower() for s in chars_on.get("subtypes", [])],
                      f"subtypes={chars_on.get('subtypes')}"))

    removed = pl.unregister_continuous_effects_for_permanent(g, p_bmoon)
    out.append(_check("Unregister returned 1 removed effect",
                      removed == 1,
                      f"removed={removed}"))

    chars_off = pl.get_effective_characteristics(g, p_usea)
    subs = [s.lower() for s in chars_off.get("subtypes", [])]
    out.append(_check("After unregister: Underground Sea back to Island/Swamp",
                      "island" in subs and "swamp" in subs,
                      f"subtypes={chars_off.get('subtypes')}"))
    out.append(_check("After unregister: NOT a Mountain",
                      "mountain" not in subs,
                      f"subtypes={chars_off.get('subtypes')}"))
    return out


def run_all() -> int:
    suites = [
        ("Blood Moon on dual land", test_blood_moon_on_dual_land),
        ("Blood Moon ignores basics", test_blood_moon_ignores_basics),
        ("Dryad Arbor (§305.7)", test_dryad_arbor_cr_305_7),
        ("Blood Moon + Magus of the Moon stacking",
         test_blood_moon_plus_magus),
        ("Blood Moon leaves battlefield → revert",
         test_blood_moon_leaves_battlefield),
    ]
    total_pass = 0
    total_fail = 0
    print("═" * 72)
    print("  §613 Blood Moon / Magus of the Moon layer resolution — "
          "verification tests")
    print("═" * 72)
    for name, fn in suites:
        print(f"\n  Suite: {name}")
        try:
            results = fn()
        except Exception as exc:
            print(f"    ! CRASH: {type(exc).__name__}: {exc}")
            import traceback
            traceback.print_exc()
            total_fail += 1
            continue
        for desc, ok, detail in results:
            flag = "PASS" if ok else "FAIL"
            tail = f"  ({detail})" if detail and not ok else ""
            print(f"    [{flag}] {desc}{tail}")
            if ok:
                total_pass += 1
            else:
                total_fail += 1
    print("\n" + "═" * 72)
    print(f"  Total: {total_pass} passed, {total_fail} failed")
    print("═" * 72)
    return 0 if total_fail == 0 else 1


if __name__ == "__main__":
    sys.exit(run_all())
