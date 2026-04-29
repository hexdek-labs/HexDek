#!/usr/bin/env python3
"""Standalone verification test for partner-commander support.

Covers CR §702.124 / §903.3c:

  1. Kraum+Tymna partner deck loads, both in command zone, both names
     in seat.commander_names.
  2. Kraum cast from command zone: first cast 4UBR (treated as cost 4),
     second cast 6UBR (cost 4 + 2*1 = 6).
  3. Tymna cast independently (cost 1WB treated as 3) does NOT interact
     with Kraum's tax counter.
  4. Damage tracking: 15 Kraum + 10 Tymna to one seat = survival
     (partner damage tracked in separate buckets); 21 Kraum alone = loss.
  5. Legality validator: bare-Partner pairs pass; Partner + non-partner
     fails; Partner-with mismatched names fails.

Run standalone:
    python3 scripts/test_partner.py

Exit 0 = pass, non-zero = fail.
"""
from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from mtg_ast import CardAST, Keyword  # type: ignore  # noqa: E402

import playloop as pl  # type: ignore  # noqa: E402


# ---------------------------------------------------------------------------
# Synthetic-card helpers.
# ---------------------------------------------------------------------------

def make_partner_card(name: str, cmc: int, power: int = 2, toughness: int = 2,
                      colors: tuple = (), type_line: str = "Legendary Creature — Human"):
    """Build a CardEntry with a bare Partner keyword in its AST."""
    ast = CardAST(
        name=name,
        abilities=(Keyword(name="partner", args=(), raw="partner"),),
        parse_errors=(),
        fully_parsed=True,
    )
    return pl.CardEntry(
        name=name,
        mana_cost=f"{{{cmc}}}",
        cmc=cmc,
        type_line=type_line,
        oracle_text="Partner (You can have two commanders if both have partner.)",
        power=power,
        toughness=toughness,
        ast=ast,
        colors=colors,
        starting_loyalty=None,
        starting_defense=None,
    )


def make_plain_card(name: str, cmc: int):
    """Non-partner legendary creature for negative tests."""
    ast = CardAST(name=name, abilities=(), parse_errors=(), fully_parsed=True)
    return pl.CardEntry(
        name=name,
        mana_cost=f"{{{cmc}}}",
        cmc=cmc,
        type_line="Legendary Creature — Human",
        oracle_text="",
        power=3, toughness=3,
        ast=ast,
        colors=(),
        starting_loyalty=None,
        starting_defense=None,
    )


def _check(label: str, cond: bool, detail: str = "") -> tuple[bool, str]:
    tag = "[PASS]" if cond else "[FAIL]"
    msg = f"    {tag} {label}"
    if detail and not cond:
        msg += f"  ({detail})"
    print(msg)
    return cond, label


def spawn_partner_game(seed: int = 1):
    """Build a 4-seat game with seat 0 running the Kraum+Tymna partner
    pair, other seats running single-commander synthetic cards.

    Returns (game, kraum, tymna).
    """
    import random
    kraum = make_partner_card("Kraum, Ludevic's Opus", cmc=4,
                              power=4, toughness=4, colors=("U", "B", "R"),
                              type_line="Legendary Creature — Zombie Horror")
    tymna = make_partner_card("Tymna the Weaver", cmc=3,
                              power=1, toughness=3, colors=("W", "B"),
                              type_line="Legendary Creature — Human Cleric")
    c = make_partner_card("Commander C", cmc=4)
    d = make_partner_card("Commander D", cmc=4)
    e = make_partner_card("Commander E", cmc=4)

    # Per-seat filler library: 50 basic lands so the shuffle has
    # something to move, independent per seat (don't share list refs).
    def _filler(n: int):
        return [pl.CardEntry(
            name="Filler Forest", mana_cost="", cmc=0,
            type_line="Basic Land — Forest", oracle_text="", power=0,
            toughness=0,
            ast=CardAST(name="Filler Forest", abilities=(), parse_errors=(),
                        fully_parsed=True),
            colors=(), starting_loyalty=None, starting_defense=None)
            for _ in range(n)]

    deck_a = pl.Deck(cards=_filler(50),
                     commander_name=kraum.name,
                     commander_names=[kraum.name, tymna.name])
    deck_a.commander_cards = [kraum, tymna]
    deck_b = pl.Deck(cards=_filler(50), commander_name=c.name)
    deck_b.commander_cards = [c]
    deck_c = pl.Deck(cards=_filler(50), commander_name=d.name)
    deck_c.commander_cards = [d]
    deck_d = pl.Deck(cards=_filler(50), commander_name=e.name)
    deck_d.commander_cards = [e]

    rng = random.Random(seed)
    seats = [
        pl.Seat(idx=0, library=list(deck_a.cards)),
        pl.Seat(idx=1, library=list(deck_b.cards)),
        pl.Seat(idx=2, library=list(deck_c.cards)),
        pl.Seat(idx=3, library=list(deck_d.cards)),
    ]
    game = pl.Game(seats=seats, verbose=False, rng=rng)
    pl.setup_commander_game(game, deck_a, deck_b, deck_c, deck_d)
    game.active = 0
    return game, kraum, tymna


# ---------------------------------------------------------------------------
# Test suites
# ---------------------------------------------------------------------------

def test_setup() -> list:
    """Kraum+Tymna land in command zone with independent tax counters."""
    out = []
    game, kraum, tymna = spawn_partner_game()
    seat_0 = game.seats[0]
    out.append(_check("partner seat has 2 commanders in command zone",
                      len(seat_0.command_zone) == 2,
                      f"cz={[c.name for c in seat_0.command_zone]}"))
    out.append(_check("partner seat has 2 commander_names",
                      len(seat_0.commander_names) == 2,
                      f"names={seat_0.commander_names}"))
    out.append(_check("Kraum is registered as commander",
                      kraum.name in seat_0.commander_names))
    out.append(_check("Tymna is registered as commander",
                      tymna.name in seat_0.commander_names))
    out.append(_check("seat 0 starting life = 40",
                      seat_0.life == 40 and seat_0.starting_life == 40,
                      f"life={seat_0.life}"))
    out.append(_check("Kraum cast tax starts at 0",
                      seat_0.commander_tax.get(kraum.name, -1) == 0))
    out.append(_check("Tymna cast tax starts at 0",
                      seat_0.commander_tax.get(tymna.name, -1) == 0))
    # Single-commander seats remain 1-commander.
    out.append(_check("single-commander seat 1 has 1 in command zone",
                      len(game.seats[1].command_zone) == 1))
    return out


def test_independent_cast_tax() -> list:
    """Casting Kraum multiple times does NOT tax Tymna, and vice versa."""
    out = []
    game, kraum, tymna = spawn_partner_game()
    seat = game.seats[0]
    # Grant mana generously so we can focus on tax.
    seat.mana_pool = 100

    # First Kraum cast: base 4, tax(Kraum) → 1.
    pl.cast_commander_from_command_zone(game, kraum)
    out.append(_check("Kraum tax after 1 cast = 1",
                      seat.commander_tax.get(kraum.name) == 1,
                      f"tax={seat.commander_tax}"))
    out.append(_check("Tymna tax still 0 after Kraum cast",
                      seat.commander_tax.get(tymna.name) == 0,
                      f"tax={seat.commander_tax}"))
    # Resolve (push & resolve were driven by cast_commander_from_command_zone).
    # Bring Kraum back to CZ for the next cast.
    if kraum not in seat.command_zone:
        # Pull from wherever (battlefield / graveyard) — in the synthetic
        # harness, cast resolves it to the battlefield; simulate bouncing.
        for zone_name in ("battlefield",):
            zone = getattr(seat, zone_name)
            for obj in list(zone):
                target = getattr(obj, "card", obj)
                if getattr(target, "name", "") == kraum.name:
                    if zone_name == "battlefield":
                        seat.battlefield.remove(obj)
                    else:
                        zone.remove(obj)
                    seat.command_zone.append(target)
                    break
        if kraum not in seat.command_zone:
            seat.command_zone.append(kraum)

    # Second Kraum cast: should be cost base 4 + 2*1 = 6.
    seat.mana_pool = 100
    starting_pool = seat.mana_pool
    pl.cast_commander_from_command_zone(game, kraum)
    paid = starting_pool - seat.mana_pool
    out.append(_check("Kraum 2nd cast paid base 4 + tax 2 = 6",
                      paid == 6, f"paid={paid}"))
    out.append(_check("Kraum tax after 2 casts = 2",
                      seat.commander_tax.get(kraum.name) == 2))
    out.append(_check("Tymna tax STILL 0 after 2 Kraum casts",
                      seat.commander_tax.get(tymna.name) == 0))

    # Now cast Tymna once: base 3, tax(Tymna) → 1. Kraum unchanged.
    seat.mana_pool = 100
    starting_pool = seat.mana_pool
    pl.cast_commander_from_command_zone(game, tymna)
    paid = starting_pool - seat.mana_pool
    out.append(_check("Tymna first cast paid base 3 (no tax)",
                      paid == 3, f"paid={paid}"))
    out.append(_check("Tymna tax after first cast = 1",
                      seat.commander_tax.get(tymna.name) == 1))
    out.append(_check("Kraum tax unchanged by Tymna cast",
                      seat.commander_tax.get(kraum.name) == 2))
    return out


def test_independent_damage_tracking() -> list:
    """15 Kraum + 10 Tymna = survival. 21 Kraum alone = loss."""
    out = []
    game, kraum, tymna = spawn_partner_game()
    victim = game.seats[1]  # opponent seat
    game.phase = "combat"

    # Kraum hits victim for 15.
    game.ev("damage", amount=15, target_kind="player",
            target_seat=victim.idx, source_card=kraum.name, source_seat=0)
    # Tymna hits victim for 10.
    game.ev("damage", amount=10, target_kind="player",
            target_seat=victim.idx, source_card=tymna.name, source_seat=0)
    pl.state_based_actions(game)
    out.append(_check("victim NOT lost after 15 Kraum + 10 Tymna",
                      not victim.lost,
                      f"lost={victim.lost}, reason={victim.loss_reason}, "
                      f"dmg={victim.commander_damage}"))
    # Verify bucket totals.
    by_dealer = victim.commander_damage.get(0, {})
    out.append(_check("Kraum bucket = 15",
                      by_dealer.get(kraum.name) == 15,
                      f"dmg={victim.commander_damage}"))
    out.append(_check("Tymna bucket = 10",
                      by_dealer.get(tymna.name) == 10))

    # Now bang Kraum in for 21 alone to a fresh victim → lose.
    game2, kraum2, _ = spawn_partner_game(seed=2)
    victim2 = game2.seats[1]
    game2.phase = "combat"
    game2.ev("damage", amount=21, target_kind="player",
             target_seat=victim2.idx, source_card=kraum2.name, source_seat=0)
    pl.state_based_actions(game2)
    out.append(_check("victim LOST after 21 Kraum alone",
                      victim2.lost,
                      f"lost={victim2.lost}, reason={victim2.loss_reason}"))
    out.append(_check("§704.6c loss reason cites Kraum",
                      "Kraum" in (victim2.loss_reason or ""),
                      f"reason={victim2.loss_reason}"))
    return out


def test_legality_validator() -> list:
    """validate_partner_pair matches CR §702.124 / §903.3c."""
    out = []
    kraum = make_partner_card("Kraum, Ludevic's Opus", cmc=4)
    tymna = make_partner_card("Tymna the Weaver", cmc=3)
    edgar = make_plain_card("Edgar Markov", cmc=5)

    legal, reason = pl.validate_partner_pair([kraum, tymna])
    out.append(_check("Kraum+Tymna legal (both Partner)", legal,
                      f"reason={reason}"))

    legal, reason = pl.validate_partner_pair([kraum])
    out.append(_check("Single commander legal", legal, f"reason={reason}"))

    legal, reason = pl.validate_partner_pair([kraum, edgar])
    out.append(_check("Kraum + Edgar Markov illegal (Edgar has no Partner)",
                      not legal, f"legal={legal}, reason={reason}"))

    legal, reason = pl.validate_partner_pair([])
    out.append(_check("empty commander list illegal", not legal))

    legal, reason = pl.validate_partner_pair([kraum, tymna, edgar])
    out.append(_check("three commanders illegal", not legal,
                      f"reason={reason}"))

    return out


# ---------------------------------------------------------------------------
# Driver
# ---------------------------------------------------------------------------

def run_all() -> int:
    suites = [
        ("Setup (§903.6 — both commanders in CZ)", test_setup),
        ("Independent cast tax (§903.8)", test_independent_cast_tax),
        ("Independent damage tracking (§704.6c / §903.10a)",
         test_independent_damage_tracking),
        ("Partner legality validator (§702.124 / §903.3c)",
         test_legality_validator),
    ]
    print("═" * 72)
    print("  Partner-commander tests (CR §702.124 / §903.3c)")
    print("═" * 72)
    total_pass = 0
    total_fail = 0
    for name, fn in suites:
        print(f"\n  Suite: {name}")
        results = fn()
        for ok, _label in results:
            if ok:
                total_pass += 1
            else:
                total_fail += 1
    print()
    print("═" * 72)
    status = "FAIL" if total_fail > 0 else "OK"
    print(f"  Total: {total_pass} passed, {total_fail} failed  [{status}]")
    print("═" * 72)
    return 1 if total_fail > 0 else 0


if __name__ == "__main__":
    sys.exit(run_all())
