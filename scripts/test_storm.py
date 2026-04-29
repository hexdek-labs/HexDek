#!/usr/bin/env python3
"""Standalone verification test for Storm keyword + cast-count infrastructure.

Covers CR §702.40 (Storm) + §700.4 (spells cast this turn) + cast-trigger
observers (Storm-Kiln Artist, Young Pyromancer, Birgi, Runaway Steam-Kin).

Run:
    python3 scripts/test_storm.py

Exit 0 = all assertions passed.

Tests:
  1. Increment semantics: casting a single non-storm spell bumps
     game.spells_cast_this_turn and seat.spells_cast_this_turn by 1.
  2. Storm copy count: with 2 prior spells cast this turn, casting
     Grapeshot produces 1 original + 2 copies = 3 damage resolutions.
  3. Turn reset: counters zero out at the active seat's untap step.
  4. Runaway Steam-Kin counter accumulation: 3 red spell casts put 3
     +1/+1 counters (floored at 3).
  5. Birgi mana gen: 5 spell casts add 5 mana to controller's pool.
  6. Young Pyromancer token gen: 3 instant/sorcery casts produce
     3 Elemental tokens.
  7. Storm copies don't re-trigger Storm (§706.10).
  8. Storm copies don't fire cast-trigger observers (§706.10).
"""

from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from mtg_ast import (  # type: ignore  # noqa: E402
    CardAST, Damage, Filter, Keyword, Modification, Static, TARGET_ANY,
)
import playloop as pl  # type: ignore  # noqa: E402


# ---------------------------------------------------------------------------
# Synthetic card factories
# ---------------------------------------------------------------------------

def make_grapeshot() -> pl.CardEntry:
    """Grapeshot: '~ deals 1 damage to any target. Storm.'"""
    damage_effect = Damage(amount=1, target=TARGET_ANY)
    spell_effect = Static(
        modification=Modification(kind="spell_effect", args=(damage_effect,)),
        raw="~ deals 1 damage to any target",
    )
    storm_kw = Keyword(name="storm", raw="storm")
    ast = CardAST(
        name="Grapeshot",
        abilities=(spell_effect, storm_kw),
        parse_errors=(),
        fully_parsed=True,
    )
    return pl.CardEntry(
        name="Grapeshot",
        mana_cost="{1}{R}",
        cmc=2,
        type_line="Sorcery",
        oracle_text="Grapeshot deals 1 damage to any target.\nStorm",
        power=None,
        toughness=None,
        ast=ast,
        colors=("R",),
    )


def make_filler_sorcery(name: str = "Filler Sorcery",
                         cost: int = 1) -> pl.CardEntry:
    """A cmc-1 sorcery with no effect. Used to juice the cast counter."""
    ast = CardAST(name=name, abilities=(), parse_errors=(), fully_parsed=True)
    return pl.CardEntry(
        name=name,
        mana_cost=f"{{{cost}}}" if cost > 0 else "",
        cmc=cost,
        type_line="Sorcery",
        oracle_text="",
        power=None,
        toughness=None,
        ast=ast,
        colors=(),
    )


def make_red_filler_sorcery(name: str) -> pl.CardEntry:
    """A cmc-1 red sorcery used to exercise red-spell cast triggers."""
    ast = CardAST(name=name, abilities=(), parse_errors=(), fully_parsed=True)
    return pl.CardEntry(
        name=name,
        mana_cost="{R}",
        cmc=1,
        type_line="Sorcery",
        oracle_text="",
        power=None,
        toughness=None,
        ast=ast,
        colors=("R",),
    )


def make_vanilla_permanent(name: str, power: int = 1, toughness: int = 1,
                            colors: tuple = ()) -> pl.CardEntry:
    """A vanilla creature — used to stand up 'Storm-Kiln Artist' etc. on
    the battlefield without relying on the parser to carry its oracle text
    into the AST. We rely on name-based dispatch in
    _fire_cast_trigger_observers."""
    ast = CardAST(name=name, abilities=(), parse_errors=(), fully_parsed=True)
    return pl.CardEntry(
        name=name,
        mana_cost="{R}",
        cmc=1,
        type_line="Creature",
        oracle_text="",
        power=power,
        toughness=toughness,
        ast=ast,
        colors=colors,
    )


# ---------------------------------------------------------------------------
# Test scaffolding
# ---------------------------------------------------------------------------

def make_blank_seat(idx: int) -> pl.Seat:
    s = pl.Seat(idx=idx, life=20)
    s.mana_pool = 100  # give every test enough to cast
    return s


def make_game(n: int = 2) -> pl.Game:
    seats = [make_blank_seat(i) for i in range(n)]
    g = pl.Game(seats=seats, active=0, turn=1)
    return g


def put_on_battlefield(game: pl.Game, seat_idx: int,
                       card: pl.CardEntry) -> pl.Permanent:
    perm = pl.Permanent(card=card, controller=seat_idx,
                        tapped=False, summoning_sick=False)
    game.seats[seat_idx].battlefield.append(perm)
    return perm


# ---------------------------------------------------------------------------
# Checks
# ---------------------------------------------------------------------------

FAILS: list[str] = []


def check(cond: bool, label: str, detail: str = "") -> None:
    status = "PASS" if cond else "FAIL"
    print(f"[{status}] {label}" + (f" — {detail}" if detail else ""))
    if not cond:
        FAILS.append(label)


# ---------------------------------------------------------------------------
# Test 1: increment semantics
# ---------------------------------------------------------------------------

def test_cast_count_increment() -> None:
    print("\n=== Test 1: cast-count increment ===")
    g = make_game(2)
    filler = make_filler_sorcery()
    g.seats[0].hand.append(filler)
    # Baseline
    check(g.spells_cast_this_turn == 0, "baseline game counter == 0")
    check(g.seats[0].spells_cast_this_turn == 0, "baseline seat counter == 0")
    # Cast one spell
    pl.cast_spell(g, filler)
    check(g.spells_cast_this_turn == 1, "after 1 cast: game counter == 1",
          f"got {g.spells_cast_this_turn}")
    check(g.seats[0].spells_cast_this_turn == 1,
          "after 1 cast: seat[0] counter == 1",
          f"got {g.seats[0].spells_cast_this_turn}")
    check(g.seats[1].spells_cast_this_turn == 0,
          "after 1 cast: seat[1] counter unchanged")


# ---------------------------------------------------------------------------
# Test 2: Storm copy count + damage total
# ---------------------------------------------------------------------------

def test_storm_copy_damage() -> None:
    print("\n=== Test 2: Storm copies on Grapeshot ===")
    g = make_game(2)
    # Pre-cast 2 filler spells this turn, then cast Grapeshot at seat 1.
    for i in range(2):
        f = make_filler_sorcery(f"Filler {i}")
        g.seats[0].hand.append(f)
        pl.cast_spell(g, f)
    check(g.spells_cast_this_turn == 2,
          "after 2 fillers: game counter == 2",
          f"got {g.spells_cast_this_turn}")
    # Record seat 1's life before Grapeshot
    life_before = g.seats[1].life
    grape = make_grapeshot()
    g.seats[0].hand.append(grape)
    pl.cast_spell(g, grape)
    # Grapeshot is the 3rd cast. spells_cast_this_turn becomes 3. Storm
    # makes 3 - 1 = 2 copies. Damage = 3 (original + 2 copies) × 1 = 3.
    check(g.spells_cast_this_turn == 3,
          "after Grapeshot: game counter == 3",
          f"got {g.spells_cast_this_turn}")
    damage_dealt = life_before - g.seats[1].life
    check(damage_dealt == 3,
          "Grapeshot deals 3 total damage (1 original + 2 storm copies)",
          f"life went {life_before} -> {g.seats[1].life} (delta {damage_dealt})")
    # Confirm a storm_trigger event fired with copies=2
    storm_events = [e for e in g.events if e.get("type") == "storm_trigger"]
    check(len(storm_events) == 1,
          "exactly one storm_trigger event fired",
          f"got {len(storm_events)}: {storm_events}")
    if storm_events:
        check(storm_events[0].get("copies") == 2,
              "storm_trigger reported copies=2",
              f"got {storm_events[0].get('copies')}")


# ---------------------------------------------------------------------------
# Test 3: turn reset
# ---------------------------------------------------------------------------

def test_turn_reset() -> None:
    print("\n=== Test 3: turn-start counter reset ===")
    g = make_game(2)
    for i in range(3):
        f = make_filler_sorcery(f"Filler {i}")
        g.seats[0].hand.append(f)
        pl.cast_spell(g, f)
    check(g.spells_cast_this_turn == 3,
          "pre-reset: game counter == 3",
          f"got {g.spells_cast_this_turn}")
    check(g.seats[0].spells_cast_this_turn == 3,
          "pre-reset: seat[0] counter == 3",
          f"got {g.seats[0].spells_cast_this_turn}")
    # Simulate start of a new turn for seat 0 (bypass the full turn loop)
    g.active = 0
    pl.untap_step(g)
    check(g.spells_cast_this_turn == 0,
          "post-untap: game counter reset to 0",
          f"got {g.spells_cast_this_turn}")
    check(g.seats[0].spells_cast_this_turn == 0,
          "post-untap: active seat[0] counter reset to 0",
          f"got {g.seats[0].spells_cast_this_turn}")
    check(g.seats[0].spells_cast_last_turn == 3,
          "post-untap: seat[0] spells_cast_last_turn == 3 (snapshot)",
          f"got {g.seats[0].spells_cast_last_turn}")


# ---------------------------------------------------------------------------
# Test 4: Runaway Steam-Kin
# ---------------------------------------------------------------------------

def test_runaway_steam_kin() -> None:
    print("\n=== Test 4: Runaway Steam-Kin counter accumulation ===")
    g = make_game(2)
    steam_kin_card = make_vanilla_permanent("Runaway Steam-Kin",
                                             power=1, toughness=1,
                                             colors=("R",))
    perm = put_on_battlefield(g, 0, steam_kin_card)
    # Initial: 0 counters
    initial = perm.counters.get("+1/+1", 0) if hasattr(perm, "counters") and perm.counters else 0
    check(initial == 0, "initial +1/+1 counters == 0")
    # Cast 3 red spells
    for i in range(3):
        red = make_red_filler_sorcery(f"Red Spell {i}")
        g.seats[0].hand.append(red)
        pl.cast_spell(g, red)
    post = perm.counters.get("+1/+1", 0) if perm.counters else 0
    check(post == 3,
          "after 3 red casts: +1/+1 counters == 3",
          f"got {post}")
    # Fourth red cast must NOT push past 3 (oracle says "if ... fewer than
    # three +1/+1 counters")
    red4 = make_red_filler_sorcery("Red Spell 4")
    g.seats[0].hand.append(red4)
    pl.cast_spell(g, red4)
    post2 = perm.counters.get("+1/+1", 0) if perm.counters else 0
    check(post2 == 3,
          "after 4th red cast: +1/+1 counters still == 3 (capped)",
          f"got {post2}")


# ---------------------------------------------------------------------------
# Test 5: Birgi mana generation
# ---------------------------------------------------------------------------

def test_birgi_mana() -> None:
    print("\n=== Test 5: Birgi, God of Storytelling mana gen ===")
    g = make_game(2)
    birgi_card = make_vanilla_permanent("Birgi, God of Storytelling",
                                         power=3, toughness=3,
                                         colors=("R",))
    put_on_battlefield(g, 0, birgi_card)
    # Record mana pool before 5 casts (we give 100 to start; after paying
    # cmc for each filler, pool drops)
    pool_before = g.seats[0].mana_pool
    cost_total = 0
    for i in range(5):
        filler = make_filler_sorcery(f"Filler {i}", cost=1)
        g.seats[0].hand.append(filler)
        pl.cast_spell(g, filler)
        cost_total += 1
    pool_after = g.seats[0].mana_pool
    # Each cast: -1 for cost, +1 from Birgi's trigger. Net 0 change per cast.
    # Over 5 casts, pool should equal pool_before.
    check(pool_after == pool_before,
          "after 5 casts with Birgi: net mana change == 0 "
          "(5 cost - 5 Birgi refunds)",
          f"went {pool_before} -> {pool_after} (cost {cost_total})")


# ---------------------------------------------------------------------------
# Test 6: Young Pyromancer tokens
# ---------------------------------------------------------------------------

def test_young_pyromancer() -> None:
    print("\n=== Test 6: Young Pyromancer token gen ===")
    g = make_game(2)
    yp_card = make_vanilla_permanent("Young Pyromancer",
                                      power=2, toughness=1,
                                      colors=("R",))
    put_on_battlefield(g, 0, yp_card)
    # Count battlefield before
    bf_before = len(g.seats[0].battlefield)
    for i in range(3):
        filler = make_filler_sorcery(f"Filler {i}")
        g.seats[0].hand.append(filler)
        pl.cast_spell(g, filler)
    bf_after = len(g.seats[0].battlefield)
    check(bf_after - bf_before == 3,
          "3 instant/sorcery casts → 3 Elemental tokens",
          f"battlefield went {bf_before} -> {bf_after}")
    # Validate token names
    new_perms = g.seats[0].battlefield[bf_before:]
    elemental_count = sum(1 for p in new_perms
                          if p.card.name == "Elemental Token")
    check(elemental_count == 3,
          "all 3 new permanents are named 'Elemental Token'",
          f"got {elemental_count}")


# ---------------------------------------------------------------------------
# Test 7: Storm copies don't re-trigger Storm
# ---------------------------------------------------------------------------

def test_storm_copies_no_recursion() -> None:
    print("\n=== Test 7: Storm copies don't re-trigger Storm ===")
    g = make_game(2)
    # Cast 1 filler, then Grapeshot. Grapeshot is the 2nd spell; it makes
    # 1 copy. The copy must NOT trigger Storm again (which would produce
    # recursive copies).
    f = make_filler_sorcery("Filler")
    g.seats[0].hand.append(f)
    pl.cast_spell(g, f)
    grape = make_grapeshot()
    g.seats[0].hand.append(grape)
    pl.cast_spell(g, grape)
    storm_events = [e for e in g.events if e.get("type") == "storm_trigger"]
    check(len(storm_events) == 1,
          "exactly ONE storm_trigger event (copies don't recurse)",
          f"got {len(storm_events)}")
    # Game counter should be 2 (filler + Grapeshot), not 3 (no
    # copy increment).
    check(g.spells_cast_this_turn == 2,
          "spells_cast_this_turn == 2 (copies don't increment)",
          f"got {g.spells_cast_this_turn}")


# ---------------------------------------------------------------------------
# Test 8: Storm copies don't fire cast-trigger observers
# ---------------------------------------------------------------------------

def test_storm_copies_no_observer_trigger() -> None:
    print("\n=== Test 8: Storm copies don't fire cast observers ===")
    g = make_game(2)
    yp = make_vanilla_permanent("Young Pyromancer",
                                 power=2, toughness=1, colors=("R",))
    put_on_battlefield(g, 0, yp)
    bf_before = len(g.seats[0].battlefield)
    # Cast 1 filler then Grapeshot. Young Pyromancer should make:
    #   - 1 token for filler
    #   - 1 token for Grapeshot (original cast)
    #   - 0 tokens for the 1 storm copy (copies don't trigger)
    # Total = 2 tokens, NOT 3.
    f = make_filler_sorcery("Filler")
    g.seats[0].hand.append(f)
    pl.cast_spell(g, f)
    grape = make_grapeshot()
    g.seats[0].hand.append(grape)
    pl.cast_spell(g, grape)
    bf_after = len(g.seats[0].battlefield)
    new_tokens = bf_after - bf_before
    check(new_tokens == 2,
          "Young Pyromancer fires only on casts, not storm copies "
          "(expected 2 tokens, not 3)",
          f"got {new_tokens}")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    test_cast_count_increment()
    test_storm_copy_damage()
    test_turn_reset()
    test_runaway_steam_kin()
    test_birgi_mana()
    test_young_pyromancer()
    test_storm_copies_no_recursion()
    test_storm_copies_no_observer_trigger()
    print(f"\n=== Summary: {len(FAILS)} failures ===")
    if FAILS:
        for f in FAILS:
            print(f"  - {f}")
        sys.exit(1)
    print("All storm + cast-count tests passed.")
    sys.exit(0)
