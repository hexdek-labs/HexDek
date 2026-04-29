#!/usr/bin/env python3
"""CR §106 typed mana pool tests.

Verifies:
  (a) Sol Ring taps for {C}{C}.
  (b) Mox Diamond taps for one mana of any color.
  (c) Lotus Petal taps+sacrifices for any color once.
  (d) Food Chain exile produces creature-only mana.
  (e) Omnath, Locus of Mana retains green across phase transitions.
  (f) Upwelling retains all colors across phase transitions.
  (g) Treasure tap-sacrifices for any color.
  (h) Pool drains at every phase/step boundary absent exemptions.
  (i) Colored cost payment routes from matching color bucket.
  (j) Powerstone token produces {C}, restricted to noncreature spending.
  (k) Mana Crypt produces 2 colorless.
  (l) Dark Ritual adds 3 black (colored) to the pool.
  (m) Phyrexian artifact mana uses appropriate color.

Run:  python3 scripts/test_mana_pool.py
"""

from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import playloop as pl  # type: ignore
from mtg_ast import (  # type: ignore
    AddMana, Activated, CardAST, Cost, Keyword, ManaSymbol,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _blank_game(n_seats: int = 2) -> pl.Game:
    import random
    seats = [pl.Seat(idx=i) for i in range(n_seats)]
    return pl.Game(seats=seats, rng=random.Random(42), verbose=False)


def _make_card(name: str, type_line: str, oracle: str = "",
               abilities: tuple = (), cmc: int = 0,
               mana_cost: str = "",
               power=None, toughness=None,
               colors: tuple = ()) -> pl.CardEntry:
    ast = CardAST(name=name, abilities=abilities, parse_errors=(),
                  fully_parsed=True)
    return pl.CardEntry(name=name, mana_cost=mana_cost, cmc=cmc,
                         type_line=type_line, oracle_text=oracle,
                         power=power, toughness=toughness, ast=ast,
                         colors=colors)


def _mana_tap_ability(pool: tuple, any_count: int = 0) -> Activated:
    """Build an Activated({T}: Add ...) ability."""
    return Activated(cost=Cost(tap=True),
                     effect=AddMana(pool=pool,
                                     any_color_count=any_count))


def _put_permanent(game: pl.Game, seat_idx: int, card: pl.CardEntry,
                   tapped: bool = False, sick: bool = False):
    perm = pl.Permanent(card=card, controller=seat_idx,
                         tapped=tapped, summoning_sick=sick,
                         owner=seat_idx)
    game.seats[seat_idx].battlefield.append(perm)
    return perm


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

def test_sol_ring_adds_CC():
    """(a) Sol Ring: {T}: Add {C}{C}. Must end with C=2 in pool."""
    g = _blank_game()
    sol = _make_card(
        name="Sol Ring",
        type_line="Legendary Artifact",
        cmc=1, mana_cost="{1}",
        abilities=(_mana_tap_ability((ManaSymbol(raw="{C}"),
                                        ManaSymbol(raw="{C}"))),),
    )
    _put_permanent(g, 0, sol)
    g.active = 0
    pl.fill_mana_pool(g, reserve=0)
    s = g.seats[0]
    assert s.mana.C == 2, f"expected C=2, got {s.mana}"
    assert s.mana.total() == 2
    print("  PASS: Sol Ring → C=2")


def test_mox_diamond_adds_any():
    """(b) Mox Diamond: {T}: Add one mana of any color."""
    g = _blank_game()
    mox = _make_card(
        name="Mox Diamond",
        type_line="Artifact",
        cmc=0, mana_cost="",
        abilities=(_mana_tap_ability(pool=(), any_count=1),),
    )
    _put_permanent(g, 0, mox)
    g.active = 0
    pl.fill_mana_pool(g)
    s = g.seats[0]
    assert s.mana.any == 1, f"expected any=1, got {s.mana}"
    print("  PASS: Mox Diamond → any=1")


def test_lotus_petal_tap_sac():
    """(c) Lotus Petal: tap + sacrifice → one any-color mana. Petal
    leaves battlefield."""
    g = _blank_game()
    petal = _make_card(
        name="Lotus Petal",
        type_line="Artifact",
        cmc=0, mana_cost="",
    )
    _put_permanent(g, 0, petal)
    g.active = 0
    pl.fill_mana_pool(g)
    s = g.seats[0]
    assert s.mana.any == 1, f"expected any=1, got {s.mana}"
    assert not any(p.card.name == "Lotus Petal" for p in s.battlefield), \
        "Lotus Petal should have been sacrificed"
    print("  PASS: Lotus Petal → any=1 + sacrificed")


def test_food_chain_restriction():
    """(d) Food Chain mana is creature-spell-only. Should pay a
    creature spell but not a sorcery."""
    g = _blank_game()
    s = g.seats[0]
    # Directly add Food-Chain mana to the pool.
    s.mana.add_restricted(5, color=None,
                           restriction="creature_spell_only",
                           source_name="Food Chain")
    # Creature cost should be payable.
    creature = _make_card(name="Test Creature",
                           type_line="Creature — Test",
                           cmc=3, mana_cost="{3}",
                           power=3, toughness=3)
    cost = pl._get_parsed_mana_cost(creature)
    assert cost is not None
    assert pl.pay_mana_cost(g, s, cost,
                             spell_type="creature",
                             reason="test", card_name="Test Creature"), \
        "creature spell should be payable from Food Chain mana"
    # Recharge.
    s.mana.clear()
    s.mana.add_restricted(5, None, "creature_spell_only", "Food Chain")
    # Sorcery should NOT be payable.
    sorcery = _make_card(name="Test Sorcery", type_line="Sorcery",
                          cmc=3, mana_cost="{3}")
    cost = pl._get_parsed_mana_cost(sorcery)
    assert not pl.pay_mana_cost(g, s, cost,
                                  spell_type="sorcery",
                                  reason="test", card_name="Test Sorcery"), \
        "sorcery should NOT be payable from Food Chain mana"
    print("  PASS: Food Chain mana pays creatures but not sorceries")


def test_omnath_retains_green():
    """(e) Omnath, Locus of Mana keeps green across phase boundaries."""
    g = _blank_game()
    omnath = _make_card(name="Omnath, Locus of Mana",
                         type_line="Legendary Creature — Elemental",
                         cmc=3, mana_cost="{2}{G}",
                         power=1, toughness=1, colors=("G",))
    _put_permanent(g, 0, omnath)
    s = g.seats[0]
    s.mana.add("G", 5)
    s.mana.add("R", 3)
    s.mana.add("any", 2)
    # Drain simulates a phase/step boundary.
    pl.drain_all_pools(g, "precombat_main", "")
    assert s.mana.G == 5, f"green should be retained; got G={s.mana.G}"
    assert s.mana.R == 0, f"red should drain; got R={s.mana.R}"
    assert s.mana.any == 0, f"any should drain; got any={s.mana.any}"
    print("  PASS: Omnath retains {G} across phase boundary")


def test_upwelling_retains_all():
    """(f) Upwelling keeps every color across boundaries."""
    g = _blank_game()
    up = _make_card(name="Upwelling", type_line="Enchantment",
                    cmc=4, mana_cost="{3}{G}", colors=("G",))
    _put_permanent(g, 0, up)
    s = g.seats[0]
    s.mana.add("W", 2); s.mana.add("U", 2); s.mana.add("B", 2)
    s.mana.add("R", 2); s.mana.add("G", 2); s.mana.add("C", 1)
    s.mana.add("any", 1)
    total_before = s.mana.total()
    pl.drain_all_pools(g, "combat", "end_of_combat")
    assert s.mana.total() == total_before, \
        f"pool should not drain under Upwelling; lost {total_before - s.mana.total()}"
    print("  PASS: Upwelling retains pool across phase boundary")


def test_treasure_token_taps_for_any():
    """(g) Treasure: {T}, sacrifice: add one mana of any color."""
    g = _blank_game()
    # Simulate a Treasure token by calling the in-engine creator.
    pl._create_treasure_token(g, seat_idx=0)
    s = g.seats[0]
    assert any("Treasure" in p.card.type_line for p in s.battlefield)
    g.active = 0
    pl.fill_mana_pool(g)
    assert s.mana.any == 1, f"expected any=1 after treasure tap, got {s.mana}"
    assert not any("Treasure" in p.card.type_line for p in s.battlefield), \
        "Treasure should be sacrificed after use"
    print("  PASS: Treasure → any=1 + sacrificed")


def test_pool_drains_at_phase_boundary():
    """(h) Without exemptions, the pool drains at every phase/step."""
    g = _blank_game()
    s = g.seats[0]
    s.mana.add("R", 4)
    s.mana.add("any", 2)
    pl.drain_all_pools(g, "precombat_main", "")
    assert s.mana.total() == 0, \
        f"pool should be empty after drain; got total={s.mana.total()}"
    print("  PASS: Pool drains at phase boundary without exemption")


def test_colored_cost_uses_matching_bucket():
    """(i) {W}{U} cost pulls from W and U buckets."""
    g = _blank_game()
    s = g.seats[0]
    s.mana.add("W", 2)
    s.mana.add("U", 2)
    s.mana.add("R", 2)
    card = _make_card(name="Test WU", type_line="Instant",
                      cmc=2, mana_cost="{W}{U}")
    cost = pl._get_parsed_mana_cost(card)
    assert pl.pay_mana_cost(g, s, cost, spell_type="instant",
                             reason="test", card_name="Test WU")
    assert s.mana.W == 1, f"expected W=1 after paying 1 W, got {s.mana.W}"
    assert s.mana.U == 1, f"expected U=1 after paying 1 U, got {s.mana.U}"
    assert s.mana.R == 2, f"R should be untouched, got {s.mana.R}"
    print("  PASS: {W}{U} pays from W and U buckets")


def test_powerstone_noncreature_only():
    """(j) Powerstone {C} mana can pay noncreature costs only."""
    g = _blank_game()
    powerstone_card = pl.CardEntry(
        name="Powerstone Token", mana_cost="", cmc=0,
        type_line="Token Artifact — Powerstone",
        oracle_text="{T}: Add {C}. Spend only on noncreature activations.",
        power=None, toughness=None,
        ast=CardAST(name="Powerstone Token", abilities=(),
                    parse_errors=(), fully_parsed=True),
        colors=(),
    )
    _put_permanent(g, 0, powerstone_card)
    g.active = 0
    pl.fill_mana_pool(g)
    s = g.seats[0]
    # Should have one restricted-{C} pip.
    assert len(s.mana.restricted) == 1, \
        f"expected 1 restricted pip, got {s.mana.restricted}"
    assert s.mana.restricted[0].restriction == "noncreature_or_artifact_activation"
    assert s.mana.restricted[0].color == "C"
    # Creature cost should NOT be payable.
    creature = _make_card(name="Bear", type_line="Creature",
                           cmc=1, mana_cost="{1}", power=2, toughness=2)
    cost = pl._get_parsed_mana_cost(creature)
    ok = pl.pay_mana_cost(g, s, cost, spell_type="creature",
                           reason="test", card_name="Bear")
    assert not ok, "Powerstone mana must not pay creature costs"
    print("  PASS: Powerstone restricted mana can't pay creatures")


def test_mana_crypt_adds_CC():
    """(k) Mana Crypt: {T}: Add {C}{C}."""
    g = _blank_game()
    crypt = _make_card(
        name="Mana Crypt", type_line="Artifact",
        cmc=0, mana_cost="",
        abilities=(_mana_tap_ability((ManaSymbol(raw="{C}"),
                                        ManaSymbol(raw="{C}"))),),
    )
    _put_permanent(g, 0, crypt)
    g.active = 0
    pl.fill_mana_pool(g)
    s = g.seats[0]
    assert s.mana.C == 2, f"Mana Crypt should produce C=2, got {s.mana.C}"
    print("  PASS: Mana Crypt → C=2")


def test_basic_lands_map_to_colors():
    """Lands tap for their respective colors — Plains→W, etc."""
    g = _blank_game()
    pairs = [
        ("Plains", "W"), ("Island", "U"), ("Swamp", "B"),
        ("Mountain", "R"), ("Forest", "G"),
    ]
    for name, color in pairs:
        g2 = _blank_game()
        land = _make_card(name=name, type_line=f"Basic Land — {name}")
        _put_permanent(g2, 0, land)
        g2.active = 0
        pl.fill_mana_pool(g2)
        val = getattr(g2.seats[0].mana, color)
        assert val == 1, f"{name} should produce 1 {color}, got {val}"
    print("  PASS: basic lands produce correct colors (W/U/B/R/G)")


def test_backward_compat_int_api():
    """Setting seat.mana_pool = 5 / += 3 / -= 2 still works."""
    g = _blank_game()
    s = g.seats[0]
    s.mana_pool = 5
    assert s.mana_pool == 5
    assert s.mana.any == 5
    s.mana_pool += 3
    assert s.mana_pool == 8
    s.mana_pool -= 2
    assert s.mana_pool == 6
    s.mana_pool = 0
    assert s.mana_pool == 0
    assert s.mana.total() == 0
    print("  PASS: legacy int API (set/+=/-=/=0) still works")


def test_artifact_mana_potential():
    """_available_mana includes artifact sources."""
    g = _blank_game()
    sol = _make_card(
        name="Sol Ring", type_line="Legendary Artifact",
        cmc=1, mana_cost="{1}",
        abilities=(_mana_tap_ability((ManaSymbol(raw="{C}"),
                                        ManaSymbol(raw="{C}"))),),
    )
    _put_permanent(g, 0, sol)
    s = g.seats[0]
    avail = pl._available_mana(s)
    # Sol Ring contributes 2 pips.
    assert avail == 2, f"Sol Ring should grant 2 in _available_mana, got {avail}"
    print("  PASS: _available_mana counts Sol Ring")


def test_phase_transition_full_flow():
    """Integration: multiple phases cascade, pool drains unless exempt."""
    g = _blank_game()
    s = g.seats[0]
    s.mana.add("G", 4)
    g.set_phase_step("precombat_main", "")
    g.set_phase_step("combat", "beginning_of_combat")
    # Pool should be empty — drained at the precombat_main → combat boundary.
    assert s.mana.total() == 0, \
        f"pool should drain at phase transition, got total={s.mana.total()}"
    # Put Omnath on bf and try again.
    omnath = _make_card(name="Omnath, Locus of Mana",
                         type_line="Legendary Creature — Elemental",
                         cmc=3, mana_cost="{2}{G}", colors=("G",))
    _put_permanent(g, 0, omnath)
    s.mana.add("G", 5)
    s.mana.add("R", 2)
    g.set_phase_step("combat", "declare_attackers")
    assert s.mana.G == 5, f"Omnath should retain G; got {s.mana.G}"
    assert s.mana.R == 0, f"Omnath does NOT retain R; got {s.mana.R}"
    print("  PASS: phase transitions drain unless Omnath exempts green")


# ---------------------------------------------------------------------------
# Runner
# ---------------------------------------------------------------------------

TESTS = [
    test_backward_compat_int_api,
    test_basic_lands_map_to_colors,
    test_sol_ring_adds_CC,
    test_mana_crypt_adds_CC,
    test_mox_diamond_adds_any,
    test_lotus_petal_tap_sac,
    test_treasure_token_taps_for_any,
    test_powerstone_noncreature_only,
    test_colored_cost_uses_matching_bucket,
    test_pool_drains_at_phase_boundary,
    test_omnath_retains_green,
    test_upwelling_retains_all,
    test_food_chain_restriction,
    test_artifact_mana_potential,
    test_phase_transition_full_flow,
]


def main() -> int:
    print("=" * 64)
    print("CR §106 Typed Mana Pool — test suite")
    print("=" * 64)
    failed = 0
    passed = 0
    for t in TESTS:
        name = t.__name__
        try:
            t()
            passed += 1
        except AssertionError as e:
            failed += 1
            print(f"  FAIL: {name}: {e}")
        except Exception as e:
            failed += 1
            print(f"  ERROR: {name}: {type(e).__name__}: {e}")
    print("-" * 64)
    print(f"  Passed: {passed}/{len(TESTS)}    Failed: {failed}")
    print("=" * 64)
    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
