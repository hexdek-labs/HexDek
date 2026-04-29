#!/usr/bin/env python3
"""Layer-system verification test: Humility + Opalescence.

This is the flagship test for the §613 layer registry. It drops a
Humility, an Opalescence, and two sample creatures onto a shared
battlefield, then queries ``get_effective_characteristics`` on each and
verifies the stable state:

  * Humility is an Enchantment. Opalescence's layer-4 effect says
    "each OTHER non-Aura enchantment is a creature..." — self-exclusion
    in the post-2017 oracle means Opalescence DOES NOT turn itself into
    a creature, so we skip Opalescence in layer 4.
  * Humility's layer 6 strips abilities from every creature.
  * Humility's layer 7b sets base P/T to 1/1 for every creature.
  * Opalescence's layer 4 turns Humility (enchantment, non-Aura, non-self)
    INTO a creature. So by the time we reach layer 6/7b, Humility is a
    creature, gets stripped, gets set to 1/1.
  * Opalescence's layer 7b sets each creature-it-turned-into-a-creature
    to P/T = CMC. But Humility's layer 7b ALSO runs (its own timestamp
    differs). The later-timestamped layer-7b effect wins per §613.7.
    In our canonical setup, Humility comes down first (older timestamp),
    Opalescence second (newer timestamp). Wait — actually per CR §613.7:
    effects are applied in timestamp ORDER, so Opalescence's 7b runs
    AFTER Humility's 7b. But Opalescence's 7b only affects permanents
    Opalescence itself turned into creatures; Humility IS one of those,
    so Opalescence would set Humility to (Humility's CMC=4) → 4/4.
    BUT Humility's 7b set it to 1/1 first. Order: Humility_7b → 1/1,
    Opalescence_7b → 4/4 (since Opalescence was registered LATER).
    Result: Humility is a 4/4 creature (Opalescence set P/T last).
    The sample creatures were NOT turned into creatures by Opalescence,
    so Opalescence's 7b predicate doesn't match them; they stay at 1/1
    from Humility's 7b. Abilities were stripped by Humility's L6 for
    all creatures (including the freshly-turned-creature Humility).

Expected stable state:
  * Humility: types=[enchantment, creature] (L4 added), abilities=[]
    (L6 stripped), power=4 toughness=4 (L7b Opalescence set to CMC=4
    since Humility's CMC=4).
  * Opalescence: types=[enchantment] (self-excluded from L4),
    abilities=[...its own static] (unchanged at L6 since NOT a creature),
    power=None toughness=None.
  * Sample creatures (Goblin Guide, Bear): types=[creature],
    abilities=[] (stripped), power=1 toughness=1.

This test is the "§613 layer framework proves itself" test.

Run standalone:
    python3 scripts/test_layer_humility.py
"""

from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from mtg_ast import CardAST, Keyword  # noqa: E402
import playloop as pl  # noqa: E402


def _card(name: str, type_line: str, cmc: int = 0,
          power=None, toughness=None, abilities=()) -> pl.CardEntry:
    ast = CardAST(name=name, abilities=tuple(abilities),
                  parse_errors=(), fully_parsed=True)
    return pl.CardEntry(
        name=name, mana_cost=f"{{{cmc}}}", cmc=cmc,
        type_line=type_line, oracle_text="",
        power=power, toughness=toughness, ast=ast, colors=(),
    )


def _place(game: pl.Game, seat_idx: int, card: pl.CardEntry) -> pl.Permanent:
    perm = pl.Permanent(card=card, controller=seat_idx,
                        tapped=False, summoning_sick=False)
    perm.timestamp = game.next_timestamp()
    game.seats[seat_idx].battlefield.append(perm)
    return perm


def _check(desc: str, cond: bool, detail: str = "") -> tuple:
    return (desc, cond, detail)


def test_humility_alone() -> list:
    """Humility + two vanilla creatures: creatures → 1/1 no abilities."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    bear = _card("Grizzly Bears", "Creature — Bear", cmc=2,
                 power=2, toughness=2,
                 abilities=(Keyword(name="haste"),))
    goblin = _card("Goblin Guide", "Creature — Goblin", cmc=1,
                   power=2, toughness=2,
                   abilities=(Keyword(name="haste"),))
    humility = _card("Humility", "Enchantment", cmc=4,
                     power=None, toughness=None)
    p_bear = _place(g, 0, bear)
    p_goblin = _place(g, 1, goblin)
    p_humility = _place(g, 0, humility)

    # Fire Humility's ETB handler via the per_card_runtime dispatch.
    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, humility, "humility_strip_abilities",
                    {"permanent": p_humility})

    g.invalidate_characteristics_cache()
    bear_chars = pl.get_effective_characteristics(g, p_bear)
    goblin_chars = pl.get_effective_characteristics(g, p_goblin)
    humility_chars = pl.get_effective_characteristics(g, p_humility)

    out.append(_check("Humility ETB registered 2 continuous effects",
                      len(g.continuous_effects) == 2,
                      f"count={len(g.continuous_effects)}"))
    out.append(_check("Bear reduced to 1/1",
                      bear_chars["power"] == 1 and bear_chars["toughness"] == 1,
                      f"got {bear_chars['power']}/{bear_chars['toughness']}"))
    out.append(_check("Goblin reduced to 1/1",
                      goblin_chars["power"] == 1
                      and goblin_chars["toughness"] == 1,
                      f"got {goblin_chars['power']}/{goblin_chars['toughness']}"))
    out.append(_check("Bear abilities stripped",
                      bear_chars["abilities"] == [],
                      f"got {bear_chars['abilities']}"))
    out.append(_check("Goblin abilities stripped",
                      goblin_chars["abilities"] == [],
                      f"got {goblin_chars['abilities']}"))
    out.append(_check("Humility is NOT a creature (not turned into one)",
                      "creature" not in humility_chars.get("types", []),
                      f"got types={humility_chars.get('types')}"))
    # Humility itself should retain its (None,None) base P/T.
    out.append(_check("Humility base P/T unchanged (not a creature)",
                      humility_chars["power"] is None,
                      f"got {humility_chars['power']}"))
    return out


def test_humility_plus_opalescence() -> list:
    """THE flagship test. Humility + Opalescence + two sample creatures.

    Expected stable state:
      - Humility: creature+enchantment (L4 Opalescence), abilities=[]
        (L6 Humility), P/T = CMC from Opalescence's L7b which fires
        AFTER Humility's L7b (later timestamp).
      - Opalescence: enchantment only (L4 self-exclusion).
      - Vanilla creatures: 1/1 no abilities.
    """
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    bear = _card("Grizzly Bears", "Creature — Bear", cmc=2,
                 power=2, toughness=2,
                 abilities=(Keyword(name="haste"),))
    goblin = _card("Goblin Guide", "Creature — Goblin", cmc=1,
                   power=2, toughness=2,
                   abilities=(Keyword(name="haste"),))
    humility = _card("Humility", "Enchantment", cmc=4,
                     power=None, toughness=None)
    opalescence = _card("Opalescence", "Enchantment", cmc=5,
                        power=None, toughness=None)
    p_bear = _place(g, 0, bear)
    p_goblin = _place(g, 1, goblin)
    p_humility = _place(g, 0, humility)
    p_opalescence = _place(g, 0, opalescence)

    # Fire both handlers. Order: Humility first (older timestamp),
    # Opalescence second (newer timestamp).
    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, humility, "humility_strip_abilities",
                    {"permanent": p_humility})
    dispatch_custom(g, 0, opalescence, "opalescence_layer_effects",
                    {"permanent": p_opalescence})

    g.invalidate_characteristics_cache()
    bear_chars = pl.get_effective_characteristics(g, p_bear)
    goblin_chars = pl.get_effective_characteristics(g, p_goblin)
    humility_chars = pl.get_effective_characteristics(g, p_humility)
    opal_chars = pl.get_effective_characteristics(g, p_opalescence)

    out.append(_check("4 continuous effects registered (2 Humility + 2 Opal)",
                      len(g.continuous_effects) == 4,
                      f"count={len(g.continuous_effects)}"))
    # Opalescence should NOT be a creature (self-exclusion).
    out.append(_check("Opalescence self-excluded (NOT a creature)",
                      "creature" not in opal_chars.get("types", []),
                      f"got types={opal_chars.get('types')}"))
    # Humility SHOULD become a creature (Opalescence's L4 added it).
    out.append(_check("Humility became a creature (L4 Opalescence)",
                      "creature" in humility_chars.get("types", []),
                      f"got types={humility_chars.get('types')}"))
    # Humility's abilities should be stripped (L6 Humility).
    out.append(_check("Humility abilities stripped (L6)",
                      humility_chars["abilities"] == [],
                      f"got {humility_chars['abilities']}"))
    # Humility's P/T: Opalescence's 7b runs AFTER Humility's 7b (newer
    # timestamp). Opalescence sets P/T to CMC=4 for Humility.
    out.append(_check("Humility P/T = 4/4 (Opal L7b last, CMC=4)",
                      humility_chars["power"] == 4
                      and humility_chars["toughness"] == 4,
                      f"got {humility_chars['power']}/{humility_chars['toughness']}"))
    # Sample creatures: Opalescence's 7b doesn't apply (they weren't
    # turned into creatures by Opalescence), so Humility's 7b wins → 1/1.
    out.append(_check("Bear: 1/1 no abilities",
                      bear_chars["power"] == 1
                      and bear_chars["toughness"] == 1
                      and bear_chars["abilities"] == [],
                      f"got P/T={bear_chars['power']}/"
                      f"{bear_chars['toughness']}, "
                      f"abilities={bear_chars['abilities']}"))
    out.append(_check("Goblin: 1/1 no abilities",
                      goblin_chars["power"] == 1
                      and goblin_chars["toughness"] == 1
                      and goblin_chars["abilities"] == [],
                      f"got P/T={goblin_chars['power']}/"
                      f"{goblin_chars['toughness']}, "
                      f"abilities={goblin_chars['abilities']}"))
    # Opalescence should not be a creature → P/T unchanged.
    out.append(_check("Opalescence P/T unchanged (None)",
                      opal_chars["power"] is None,
                      f"got {opal_chars['power']}"))
    return out


def test_humility_cache_invalidation() -> list:
    """Re-register then re-query — cache invalidates correctly."""
    out = []
    g = pl.Game(seats=[pl.Seat(idx=0), pl.Seat(idx=1)])
    bear = _card("Bear", "Creature — Bear", cmc=2, power=2, toughness=2)
    p_bear = _place(g, 0, bear)

    # Baseline — no layer effects.
    c1 = pl.get_effective_characteristics(g, p_bear)
    out.append(_check("Baseline Bear = 2/2",
                      c1["power"] == 2 and c1["toughness"] == 2))

    # Register Humility.
    humility = _card("Humility", "Enchantment", cmc=4)
    p_humility = _place(g, 0, humility)
    from extensions.per_card_runtime import dispatch_custom
    dispatch_custom(g, 0, humility, "humility_strip_abilities",
                    {"permanent": p_humility})

    c2 = pl.get_effective_characteristics(g, p_bear)
    out.append(_check("After Humility register: Bear = 1/1 "
                      "(cache invalidated)",
                      c2["power"] == 1 and c2["toughness"] == 1,
                      f"got {c2['power']}/{c2['toughness']}"))

    # Unregister Humility (simulating it leaves battlefield).
    pl.unregister_continuous_effects_for_permanent(g, p_humility)

    c3 = pl.get_effective_characteristics(g, p_bear)
    out.append(_check("After Humility unregister: Bear back to 2/2",
                      c3["power"] == 2 and c3["toughness"] == 2,
                      f"got {c3['power']}/{c3['toughness']}"))
    return out


def run_all() -> int:
    suites = [
        ("Humility alone", test_humility_alone),
        ("Humility + Opalescence (flagship)", test_humility_plus_opalescence),
        ("Humility cache invalidation + unregister", test_humility_cache_invalidation),
    ]
    total_pass = 0
    total_fail = 0
    print("═" * 72)
    print("  §613 Humility + Opalescence layer resolution — "
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
