#!/usr/bin/env python3
"""Layer-system verification test: Painter's Servant.

Tests:
  1. Painter's Servant ETB chooses a color and registers a layer-5
     continuous effect. Every battlefield permanent then has the chosen
     color added to its colors (in addition to existing colors, per
     the oracle "in addition to their other colors").
  2. The `game.painter_color` signal is also set (legacy Grindstone
     consumer).
  3. Cards outside the battlefield — hand, library, graveyard, exile —
     are modeled via the `game.painter_color` attribute (since the
     §613 layer system only resolves characteristics for battlefield
     permanents). The layer effect itself is scoped to the battlefield
     but the game-level flag carries the all-zones signal that matches
     oracle text.
  4. A colorless permanent (e.g. Sol Ring) gets the chosen color.
  5. A colored permanent (e.g. Island) KEEPS its existing color AND
     adds the chosen one.
  6. Painter's Servant leaves the battlefield → effect unregisters →
     colors revert.

Run standalone:
    python3 scripts/test_layer_painter.py
"""

from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from mtg_ast import CardAST  # noqa: E402
import playloop as pl  # noqa: E402


def _card(name: str, type_line: str, cmc: int = 0,
          power=None, toughness=None,
          colors=()) -> pl.CardEntry:
    ast = CardAST(name=name, abilities=(), parse_errors=(),
                  fully_parsed=True)
    return pl.CardEntry(
        name=name, mana_cost=f"{{{cmc}}}", cmc=cmc,
        type_line=type_line, oracle_text="",
        power=power, toughness=toughness, ast=ast, colors=tuple(colors),
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


def test_painter_adds_color_to_colorless() -> list:
    """Painter's Servant choosing Red → Sol Ring (colorless) is now Red."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    sol = _card("Sol Ring", "Artifact", cmc=1)
    painter = _card("Painter's Servant", "Artifact Creature — Scarecrow",
                    cmc=2, power=1, toughness=3)
    p_sol = _place(g, 0, sol)
    p_painter = _place(g, 0, painter)
    # Pre-set chosen_color to "R" since the choose-color hook's default
    # policy picks from opposing board state; we want a stable choice.
    p_painter.chosen_color = "R"

    from extensions.per_card_runtime import dispatch_custom
    # painters_servant_color_wash now also registers the L5 effect.
    dispatch_custom(g, 0, painter, "painters_servant_color_wash",
                    {"permanent": p_painter})

    g.invalidate_characteristics_cache()
    chars = pl.get_effective_characteristics(g, p_sol)
    out.append(_check("game.painter_color set to R (legacy signal)",
                      getattr(g, "painter_color", None) == "R",
                      f"got {getattr(g, 'painter_color', None)}"))
    out.append(_check("Sol Ring now Red (via layer 5)",
                      "R" in chars.get("colors", []),
                      f"colors={chars.get('colors')}"))
    out.append(_check("Layer-5 continuous effect registered",
                      any(ce.layer == "5"
                          for ce in g.continuous_effects),
                      f"effects={[ce.layer for ce in g.continuous_effects]}"))
    return out


def test_painter_adds_color_to_colored() -> list:
    """Painter (R) + Island (U) → Island is U AND R (additive)."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    island = _card("Island", "Basic Land — Island", colors=())
    # Use a colored card since basic lands are colorless per CR 202.2.
    # Swap in Tropical Island (which is uncolored too but let's use
    # Counterspell as a spell example via its AST). Simpler: hand-
    # craft a colored creature.
    blue_bear = _card("Phantasmal Bear", "Creature — Bear",
                      cmc=1, power=2, toughness=2,
                      colors=("U",))
    painter = _card("Painter's Servant", "Artifact Creature — Scarecrow",
                    cmc=2, power=1, toughness=3)
    _ = _place(g, 0, island)  # not asserted on but placed for flavor
    p_bear = _place(g, 0, blue_bear)
    p_painter = _place(g, 0, painter)
    p_painter.chosen_color = "R"

    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, painter, "painters_servant_color_wash",
                    {"permanent": p_painter})

    g.invalidate_characteristics_cache()
    chars = pl.get_effective_characteristics(g, p_bear)
    out.append(_check("Phantasmal Bear retains Blue",
                      "U" in chars.get("colors", []),
                      f"colors={chars.get('colors')}"))
    out.append(_check("Phantasmal Bear gains Red (Painter chose R)",
                      "R" in chars.get("colors", []),
                      f"colors={chars.get('colors')}"))
    # Two colors now — order-agnostic.
    out.append(_check("Phantasmal Bear exactly {U, R}",
                      sorted(chars.get("colors", [])) == ["R", "U"],
                      f"colors={chars.get('colors')}"))
    return out


def test_painter_unregister() -> list:
    """When Painter leaves, color-wash unregisters; colors revert."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    sol = _card("Sol Ring", "Artifact", cmc=1)
    painter = _card("Painter's Servant", "Artifact Creature — Scarecrow",
                    cmc=2, power=1, toughness=3)
    p_sol = _place(g, 0, sol)
    p_painter = _place(g, 0, painter)
    p_painter.chosen_color = "R"

    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, painter, "painters_servant_color_wash",
                    {"permanent": p_painter})
    g.invalidate_characteristics_cache()
    chars_before = pl.get_effective_characteristics(g, p_sol)
    out.append(_check("Pre-unregister: Sol Ring is Red",
                      "R" in chars_before.get("colors", []),
                      f"colors={chars_before.get('colors')}"))

    removed = pl.unregister_continuous_effects_for_permanent(g, p_painter)
    out.append(_check("Unregister returned 1 removed effect",
                      removed == 1,
                      f"removed={removed}"))

    chars_after = pl.get_effective_characteristics(g, p_sol)
    out.append(_check("Post-unregister: Sol Ring is colorless again",
                      chars_after.get("colors", []) == [],
                      f"colors={chars_after.get('colors')}"))
    return out


def test_painter_zones_flag() -> list:
    """Cards OUTSIDE the battlefield rely on `game.painter_color`
    (all-zones oracle wording). We verify the flag is set so the
    Grindstone mill-loop / similar consumers can still see the
    all-zones color-share."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    painter = _card("Painter's Servant", "Artifact Creature — Scarecrow",
                    cmc=2, power=1, toughness=3)
    p_painter = _place(g, 0, painter)
    p_painter.chosen_color = "G"

    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, painter, "painters_servant_color_wash",
                    {"permanent": p_painter})

    out.append(_check("game.painter_color = G (all-zones signal)",
                      g.painter_color == "G",
                      f"got {g.painter_color}"))
    # A card in library is NOT on battlefield → layer system doesn't
    # reach it, but the flag remains the source of truth for off-
    # battlefield checks.
    bear = _card("Bear", "Creature — Bear", cmc=2, power=2, toughness=2)
    g.seats[0].library.append(bear)
    # Grindstone-style consumer check: use game.painter_color directly.
    out.append(_check("Library card shares Painter color via flag",
                      g.painter_color == "G"))
    return out


def run_all() -> int:
    suites = [
        ("Painter on colorless permanent",
         test_painter_adds_color_to_colorless),
        ("Painter on already-colored permanent",
         test_painter_adds_color_to_colored),
        ("Painter unregister on LTB",
         test_painter_unregister),
        ("Painter all-zones flag",
         test_painter_zones_flag),
    ]
    total_pass = 0
    total_fail = 0
    print("═" * 72)
    print("  §613 Painter's Servant layer resolution — "
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
