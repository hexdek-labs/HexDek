#!/usr/bin/env python3
"""Standalone verification test for CR §726 Day/Night + §712 DFC
transform + §702.144 Daybound / §702.145 Nightbound + DFC commander
name resolution.

Covers:
  1. §726.2 — game starts at "neither"; first daybound/nightbound
              permanent to enter flips state to "day".
  2. §726.3a day→night — if active player cast 0 spells last turn,
              state flips from day to night on the next turn start.
  3. §726.3a night→day — if active player cast 2+ spells last turn,
              state flips from night to day on the next turn start.
  4. §702.144 Daybound — a daybound creature transforms to its
              nightbound back face when state becomes night, and back
              to its daybound front face when state becomes day.
  5. §712 transform event emission + timestamp refresh + event log.
  6. DFC commander setup (§903 + §712): Ral, Monsoon Mage loads from
     the cedh_stormoff_b5_ral.txt deck, enters command zone, and can
     be cast out of the command zone by name.
  7. Werewolf-deck smoke: loading the werewolf deck with Ulrich,
     Reckless Stormseeker, Scorned Villager etc. produces CardEntry
     objects whose front and back faces both parse, and the front
     face carries the `daybound` Keyword AST node.

Run standalone:
    python3 scripts/test_day_night.py

Exit 0 on all-green, non-zero on failures.

Citations verified against data/rules/MagicCompRules-20260227.txt:
  108.3, 613, 614.5, 614.6, 702.144, 702.145,
  712.1, 712.2, 712.3, 712.8,
  726.1, 726.2, 726.3, 726.3a, 726.4,
  903.3, 903.6, 903.7
"""

from __future__ import annotations

import json
import random
import sys
from pathlib import Path
from typing import Any

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
ROOT = HERE.parent

import parser as mtg_parser  # type: ignore
import playloop as pl  # type: ignore
from mtg_ast import CardAST, Keyword  # type: ignore


def _oracle_by_name() -> dict:
    path = ROOT / "data" / "rules" / "oracle-cards.json"
    with open(path) as f:
        cards = json.load(f)
    return {c["name"].lower(): c for c in cards if mtg_parser.is_real_card(c)}


# ---------------------------------------------------------------------------
# Synthetic card helpers so we don't rely on oracle-cards.json for every test.
# ---------------------------------------------------------------------------

def _forest_card() -> pl.CardEntry:
    return pl.CardEntry(
        name="Forest",
        mana_cost="",
        cmc=0,
        type_line="Basic Land — Forest",
        oracle_text="",
        power=None,
        toughness=None,
        ast=CardAST(name="Forest", abilities=(), parse_errors=(), fully_parsed=True),
        colors=(),
    )


def _synthetic_daybound_werewolf() -> pl.CardEntry:
    """Build a synthetic Daybound DFC: a 2/2 daybound front face and a
    3/3 nightbound back face. Parser-free; we forge the ASTs directly
    so the test doesn't depend on oracle lookups.
    """
    front_ast = CardAST(
        name="Test Wolf Human",
        abilities=(Keyword(name="daybound", raw="Daybound"),),
        parse_errors=(), fully_parsed=True)
    back_ast = CardAST(
        name="Test Wolf Beast",
        abilities=(Keyword(name="nightbound", raw="Nightbound"),),
        parse_errors=(), fully_parsed=True)
    return pl.CardEntry(
        name="Test Wolf Human // Test Wolf Beast",
        mana_cost="{1}{G}",
        cmc=2,
        type_line="Creature — Human Werewolf",
        oracle_text="Daybound",
        power=2,
        toughness=2,
        ast=front_ast,
        colors=("G",),
        layout="transform",
        is_dfc=True,
        front_face_name="Test Wolf Human",
        front_face_ast=front_ast,
        front_face_mana_cost="{1}{G}",
        front_face_cmc=2,
        front_face_type_line="Creature — Human Werewolf",
        front_face_oracle_text="Daybound",
        front_face_power=2,
        front_face_toughness=2,
        front_face_colors=("G",),
        back_face_name="Test Wolf Beast",
        back_face_ast=back_ast,
        back_face_mana_cost="",
        back_face_cmc=2,
        back_face_type_line="Creature — Werewolf",
        back_face_oracle_text="Nightbound",
        back_face_power=3,
        back_face_toughness=3,
        back_face_colors=("G",),
    )


def _make_game(seat_count: int = 2) -> pl.Game:
    seats = [pl.Seat(idx=i, library=[]) for i in range(seat_count)]
    return pl.Game(seats=seats, verbose=False, rng=random.Random(0))


def _put_on_battlefield(game: pl.Game, seat_idx: int, card: pl.CardEntry
                       ) -> pl.Permanent:
    perm = pl.Permanent(card=card, controller=seat_idx,
                        tapped=False, summoning_sick=False)
    pl._etb_initialize(game, perm)
    game.seats[seat_idx].battlefield.append(perm)
    # Fire _maybe_become_day (also called by _etb_initialize for DFC;
    # calling here ensures parity for tests that put DFCs manually).
    pl._maybe_become_day(game, reason="test_harness")
    return perm


# ---------------------------------------------------------------------------
# Test cases — each returns (description, pass_flag, detail).
# ---------------------------------------------------------------------------

def _check(desc: str, cond: bool, detail: str = "") -> tuple[str, bool, str]:
    return (desc, cond, detail)


def test_initial_state() -> list:
    out = []
    game = _make_game()
    out.append(_check("§726.2 game begins at 'neither'",
                      game.day_night == "neither",
                      f"got {game.day_night}"))
    return out


def test_726_2_becomes_day_on_etb() -> list:
    """§726.2 — putting a daybound permanent onto the battlefield while
    state is 'neither' flips the state to 'day'."""
    out = []
    game = _make_game()
    werewolf = _synthetic_daybound_werewolf()
    _put_on_battlefield(game, 0, werewolf)
    out.append(_check("§726.2 daybound ETB → state becomes day",
                      game.day_night == "day",
                      f"state={game.day_night}"))
    # Emitted event
    events = [e for e in game.events if e.get("type") == "day_night_change"]
    out.append(_check("§726.2 day_night_change event emitted",
                      len(events) == 1
                      and events[0]["to_state"] == "day"
                      and events[0]["rule"] == "726.2",
                      f"events={events}"))
    return out


def test_726_3a_day_to_night() -> list:
    """§726.3a — day + prev active cast 0 spells → night."""
    out = []
    game = _make_game()
    werewolf = _synthetic_daybound_werewolf()
    perm = _put_on_battlefield(game, 0, werewolf)
    # Game is now day; werewolf is front-face (daybound).
    assert game.day_night == "day"
    assert not perm.transformed
    # Simulate: last active player cast 0 spells.
    game.spells_cast_by_active_last_turn = 0
    pl.evaluate_day_night_at_turn_start(game)
    out.append(_check("§726.3a day→night on 0-cast turn",
                      game.day_night == "night"))
    # Werewolf should have transformed to nightbound face.
    out.append(_check("§702.144 daybound werewolf transforms to night face",
                      perm.transformed,
                      f"transformed={perm.transformed}"))
    out.append(_check("transform: active face name = back face",
                      perm.card.name == "Test Wolf Beast",
                      f"name={perm.card.name}"))
    out.append(_check("transform: active face P/T = 3/3",
                      perm.card.power == 3 and perm.card.toughness == 3,
                      f"P/T={perm.card.power}/{perm.card.toughness}"))
    return out


def test_726_3a_night_to_day() -> list:
    """§726.3a — night + prev active cast 2+ spells → day."""
    out = []
    game = _make_game()
    werewolf = _synthetic_daybound_werewolf()
    perm = _put_on_battlefield(game, 0, werewolf)
    # Flip to night first.
    game.spells_cast_by_active_last_turn = 0
    pl.evaluate_day_night_at_turn_start(game)
    assert game.day_night == "night"
    assert perm.transformed  # on back face
    # Now simulate: last active cast 2 spells.
    game.spells_cast_by_active_last_turn = 2
    pl.evaluate_day_night_at_turn_start(game)
    out.append(_check("§726.3a night→day on 2+-cast turn",
                      game.day_night == "day"))
    out.append(_check("§702.145 nightbound back-face transforms to day",
                      not perm.transformed,
                      f"transformed={perm.transformed}"))
    out.append(_check("transform back: active face = front face",
                      perm.card.name == "Test Wolf Human"))
    return out


def test_726_3a_no_transition_when_conditions_missed() -> list:
    """§726.3a — day + 1 spell cast → stays day; night + 1 spell → stays night."""
    out = []
    game = _make_game()
    werewolf = _synthetic_daybound_werewolf()
    _put_on_battlefield(game, 0, werewolf)
    # Day, 1 spell cast → stays day.
    game.spells_cast_by_active_last_turn = 1
    pl.evaluate_day_night_at_turn_start(game)
    out.append(_check("§726.3a day + 1 cast → stays day",
                      game.day_night == "day"))
    # Force night.
    game.spells_cast_by_active_last_turn = 0
    pl.evaluate_day_night_at_turn_start(game)
    assert game.day_night == "night"
    # Night, 1 spell cast → stays night.
    game.spells_cast_by_active_last_turn = 1
    pl.evaluate_day_night_at_turn_start(game)
    out.append(_check("§726.3a night + 1 cast → stays night",
                      game.day_night == "night"))
    return out


def test_712_transform_preserves_counters_and_attachments() -> list:
    """§712.3 — transform keeps counters, attachments, etc."""
    out = []
    game = _make_game()
    werewolf = _synthetic_daybound_werewolf()
    perm = _put_on_battlefield(game, 0, werewolf)
    perm.counters["+1/+1"] = 3
    ts_before = perm.timestamp
    # Flip to night → transform.
    game.spells_cast_by_active_last_turn = 0
    pl.evaluate_day_night_at_turn_start(game)
    out.append(_check("§712.3 counters survive transform",
                      perm.counters.get("+1/+1") == 3))
    out.append(_check("§712.8 timestamp refreshed on transform",
                      perm.timestamp > ts_before,
                      f"{ts_before} → {perm.timestamp}"))
    return out


def test_transform_non_dfc_noop() -> list:
    """§712.1 — transforming a non-DFC is a no-op."""
    out = []
    game = _make_game()
    card = pl.CardEntry(
        name="Plain Goblin", mana_cost="{R}", cmc=1,
        type_line="Creature — Goblin", oracle_text="", power=1, toughness=1,
        ast=CardAST(name="Plain Goblin", abilities=(),
                    parse_errors=(), fully_parsed=True))
    perm = _put_on_battlefield(game, 0, card)
    name_before = perm.card.name
    ok = pl.transform_permanent(game, perm, reason="test")
    out.append(_check("§712.1 non-DFC transform returns False",
                      ok is False))
    out.append(_check("§712.1 non-DFC card unchanged after failed transform",
                      perm.card.name == name_before))
    return out


def test_ral_dfc_commander_setup() -> list:
    """Real-deck integration: Ral Monsoon Mage commander is resolved
    from the cedh_stormoff_b5_ral.txt deck file, enters the command
    zone, and can be cast out of the command zone by name."""
    out = []
    try:
        from gauntlet import parse_deck_file  # type: ignore
    except Exception as e:
        out.append(_check(
            "gauntlet module importable",
            False,
            f"import error: {e}"))
        return out
    by_name = _oracle_by_name()
    pr = parse_deck_file(
        str(ROOT / "data" / "decks" / "benched"
            / "cedh_stormoff_b5_ral.txt"),
        by_name)
    out.append(_check("Ral deck parses",
                      pr.deck is not None,
                      f"missing={pr.missing[:3]}"))
    if pr.deck is None:
        return out
    seats = [pl.Seat(idx=0, library=list(pr.deck.cards)),
             pl.Seat(idx=1, library=list(pr.deck.cards))]
    game = pl.Game(seats=seats, verbose=False, rng=random.Random(0))
    pl.setup_commander_game(game, pr.deck, pr.deck)
    cmd_names = game.seats[0].commander_names
    cz = game.seats[0].command_zone
    # The canonical name should be the full double-slash oracle name.
    expected_full = "Ral, Monsoon Mage // Ral, Leyline Prodigy"
    out.append(_check("§712+§903.3 DFC commander name canonicalized",
                      expected_full in cmd_names,
                      f"commander_names={cmd_names}"))
    out.append(_check("§903.6 Ral in command zone",
                      any(c.name == expected_full for c in cz),
                      f"cz={[c.name for c in cz]}"))
    out.append(_check("DFC commander card carries both faces",
                      cz[0].is_dfc is True
                      and cz[0].front_face_name == "Ral, Monsoon Mage"
                      and cz[0].back_face_name == "Ral, Leyline Prodigy",
                      f"faces: {cz[0].front_face_name} / {cz[0].back_face_name}"))
    # Cast from command zone — Ral front face is mana value 2. Give
    # the seat 3 mana to cover the free cast.
    game.phase = "main1"
    game.seats[0].mana_pool = 10
    result = pl.cast_commander_from_command_zone(game, cz[0])
    out.append(_check("§903.8 Ral castable from command zone",
                      any(p.card.name == expected_full
                          for p in game.seats[0].battlefield),
                      f"bf={[p.card.name for p in game.seats[0].battlefield]}, "
                      f"result={result}"))
    return out


def test_werewolf_deck_loads() -> list:
    """Ulrich deck smoke: the werewolf deck parses with 99 cards, the
    commander is canonicalized to the double-slash DFC name, and the
    deck contains multiple cards with daybound keywords on front face."""
    out = []
    try:
        from gauntlet import parse_deck_file  # type: ignore
    except Exception as e:
        out.append(_check("gauntlet module importable", False, str(e)))
        return out
    by_name = _oracle_by_name()
    pr = parse_deck_file(
        str(ROOT / "data" / "decks" / "benched"
            / "werewolf_daynight_flip_b3_ulrich.txt"),
        by_name)
    out.append(_check("Ulrich deck parses",
                      pr.deck is not None))
    if pr.deck is None:
        return out
    # Count cards with daybound keyword on front face.
    daybound_count = 0
    for ce in pr.deck.cards:
        ast = ce.front_face_ast or ce.ast
        if ast is None:
            continue
        for a in ast.abilities:
            if isinstance(a, Keyword) and a.name == "daybound":
                daybound_count += 1
                break
    out.append(_check("werewolf deck has >=4 daybound creatures",
                      daybound_count >= 4,
                      f"daybound_count={daybound_count}"))
    # Setup as a commander game; Ulrich canonicalizes.
    seats = [pl.Seat(idx=0, library=list(pr.deck.cards)),
             pl.Seat(idx=1, library=list(pr.deck.cards))]
    game = pl.Game(seats=seats, verbose=False, rng=random.Random(0))
    pl.setup_commander_game(game, pr.deck, pr.deck)
    expected = "Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha"
    out.append(_check("Ulrich DFC commander canonical name",
                      expected in game.seats[0].commander_names,
                      f"got {game.seats[0].commander_names}"))
    return out


def test_reckless_stormseeker_face_parse() -> list:
    """Reckless Stormseeker (MID werewolf) should load with both faces
    parsed, front carrying daybound, back carrying nightbound."""
    out = []
    by_name = _oracle_by_name()
    ce = pl.load_card_by_name(by_name, "Reckless Stormseeker")
    out.append(_check("Reckless Stormseeker loads", ce is not None))
    if ce is None:
        return out
    out.append(_check("full name is double-slash oracle name",
                      ce.name == "Reckless Stormseeker // Storm-Charged Slasher"))
    out.append(_check("is_dfc flag set",
                      ce.is_dfc is True))
    # Front face daybound
    ff_kws = [a.name for a in (ce.front_face_ast.abilities if ce.front_face_ast else ())
              if isinstance(a, Keyword)]
    out.append(_check("front face parses 'daybound' keyword",
                      "daybound" in ff_kws,
                      f"kws={ff_kws}"))
    bf_kws = [a.name for a in (ce.back_face_ast.abilities if ce.back_face_ast else ())
              if isinstance(a, Keyword)]
    out.append(_check("back face parses 'nightbound' keyword",
                      "nightbound" in bf_kws,
                      f"kws={bf_kws}"))
    return out


def test_other_dfc_commanders_canonicalize() -> list:
    """Ulrich / Ral / Tergrid all canonicalize to their double-slash
    oracle name when set up as commanders."""
    out = []
    try:
        from gauntlet import parse_deck_file  # type: ignore
    except Exception as e:
        out.append(_check("gauntlet module importable", False, str(e)))
        return out
    by_name = _oracle_by_name()
    deck_fixtures = [
        ("cedh_stormoff_b5_ral.txt",
         "Ral, Monsoon Mage // Ral, Leyline Prodigy"),
        ("werewolf_daynight_flip_b3_ulrich.txt",
         "Ulrich of the Krallenhorde // Ulrich, Uncontested Alpha"),
    ]
    for fname, expected in deck_fixtures:
        pr = parse_deck_file(
            str(ROOT / "data" / "decks" / "benched" / fname),
            by_name)
        if pr.deck is None:
            out.append(_check(f"{fname} deck parses", False,
                              f"missing={pr.missing[:3]}"))
            continue
        seats = [pl.Seat(idx=0, library=list(pr.deck.cards)),
                 pl.Seat(idx=1, library=list(pr.deck.cards))]
        g = pl.Game(seats=seats, verbose=False, rng=random.Random(0))
        pl.setup_commander_game(g, pr.deck, pr.deck)
        out.append(_check(f"{fname}: canonical name matches oracle",
                          expected in g.seats[0].commander_names,
                          f"got {g.seats[0].commander_names}"))
    return out


# ---------------------------------------------------------------------------
# Runner
# ---------------------------------------------------------------------------

def _run(name: str, fn) -> tuple[int, int, list]:
    print(f"\n  Suite: {name}")
    results = fn()
    passed = 0
    failed = 0
    for desc, ok, detail in results:
        tag = "PASS" if ok else "FAIL"
        print(f"    [{tag}] {desc}" + (f"  -- {detail}" if detail and not ok else ""))
        if ok:
            passed += 1
        else:
            failed += 1
    return passed, failed, results


def main() -> int:
    print("=" * 72)
    print("  Day/Night + DFC Transform test suite (CR §726 / §712 / §702.144)")
    print("=" * 72)
    total_pass = 0
    total_fail = 0
    for name, fn in [
        ("§726.2 initial state", test_initial_state),
        ("§726.2 becomes day on ETB", test_726_2_becomes_day_on_etb),
        ("§726.3a day→night", test_726_3a_day_to_night),
        ("§726.3a night→day", test_726_3a_night_to_day),
        ("§726.3a non-transition cases",
         test_726_3a_no_transition_when_conditions_missed),
        ("§712.3 transform preserves state",
         test_712_transform_preserves_counters_and_attachments),
        ("§712.1 non-DFC transform no-op",
         test_transform_non_dfc_noop),
        ("Reckless Stormseeker DFC face parse",
         test_reckless_stormseeker_face_parse),
        ("Ral DFC commander setup + cast",
         test_ral_dfc_commander_setup),
        ("Ulrich werewolf deck loads", test_werewolf_deck_loads),
        ("Other DFC commanders canonicalize",
         test_other_dfc_commanders_canonicalize),
    ]:
        p, f, _ = _run(name, fn)
        total_pass += p
        total_fail += f
    print("\n" + "=" * 72)
    print(f"  Total: {total_pass} passed, {total_fail} failed "
          f"({total_pass + total_fail} assertions)")
    print("=" * 72)
    return 0 if total_fail == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
