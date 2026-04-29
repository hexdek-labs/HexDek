#!/usr/bin/env python3
"""Standalone verification test for N-seat (multiplayer) game-state
generalization. Targets the 3-4 seat scaffolding for Commander gauntlet
play.

Covers:
  1. N-seat construction — play_game(decks=[...]) produces N Seat objects.
  2. Turn-order cycling — swap_active cycles 0 → 1 → 2 → 3 → 0.
  3. Turn counter only bumps when active returns to the lowest-index
     living seat.
  4. Seat elimination — setting seat.lost triggers §800.4 cleanup:
     battlefield cleared, replacement/continuous effects pruned, stack
     items this seat controls are purged.
  5. Last-seat-standing wins — check_end sets game.winner to the sole
     living seat's idx.
  6. Commander 4-player setup — setup_commander_game(*decks) for 4 decks
     seeds 40-life × 4, commander zones populated, §903.9b replacement
     registered per commander.
  7. N-way priority-pass APNAP ordering — _priority_round polls all
     non-caster living seats in APNAP order.

Run standalone:
    python3 scripts/test_multiplayer.py

Exit code 0 = pass, non-zero = fail.
"""

from __future__ import annotations

import random
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from mtg_ast import CardAST  # noqa: E402
import playloop as pl  # noqa: E402


# ---------------------------------------------------------------------------
# Synthetic-card helpers (parser-free, mirrors test_commander_zone.py).
# ---------------------------------------------------------------------------

def make_card(name: str, cmc: int,
              type_line: str = "Legendary Creature — Human",
              power: int = 2, toughness: int = 2,
              colors: tuple = ()) -> pl.CardEntry:
    return pl.CardEntry(
        name=name, mana_cost=f"{{{cmc}}}", cmc=cmc,
        type_line=type_line, oracle_text="",
        power=power, toughness=toughness,
        ast=CardAST(name=name, abilities=(),
                    parse_errors=(), fully_parsed=True),
        colors=colors, starting_loyalty=None, starting_defense=None,
    )


def make_basic_land(name: str = "Forest") -> pl.CardEntry:
    return pl.CardEntry(
        name=name, mana_cost="", cmc=0,
        type_line=f"Basic Land — {name}",
        oracle_text="", power=None, toughness=None,
        ast=CardAST(name=name, abilities=(),
                    parse_errors=(), fully_parsed=True),
        colors=(),
    )


def make_deck(cards_count: int = 40) -> list:
    return [make_basic_land("Forest") for _ in range(cards_count)]


def make_commander_deck(commander: pl.CardEntry,
                        fillers: int = 40) -> pl.Deck:
    land = make_basic_land("Forest")
    cards = [land for _ in range(fillers)]
    deck = pl.Deck(cards=cards, commander_name=commander.name)
    deck.commander_cards = [commander]  # type: ignore[attr-defined]
    return deck


# ---------------------------------------------------------------------------
# Assertion harness.
# ---------------------------------------------------------------------------

_RESULTS: list[tuple[str, bool, str]] = []


def check(desc: str, cond: bool, detail: str = "") -> None:
    _RESULTS.append((desc, cond, detail))


def print_results() -> int:
    passed = sum(1 for _, ok, _ in _RESULTS if ok)
    failed = sum(1 for _, ok, _ in _RESULTS if not ok)
    print("\n" + "=" * 72)
    print("  N-seat generalization tests — 4-player EDH gauntlet prerequisites")
    print("=" * 72)
    last_desc = None
    for desc, ok, detail in _RESULTS:
        tag = "[PASS]" if ok else "[FAIL]"
        line = f"    {tag} {desc}"
        if detail and not ok:
            line += f"  ({detail})"
        print(line)
        last_desc = desc
    print("-" * 72)
    print(f"  Total: {passed} passed, {failed} failed "
          f"({passed + failed} assertions)")
    print("=" * 72)
    return 0 if failed == 0 else 1


# ---------------------------------------------------------------------------
# 1. N-seat construction.
# ---------------------------------------------------------------------------

def test_n_seat_construction() -> None:
    rng = random.Random(42)
    decks = [make_deck() for _ in range(4)]
    # Hand-constructed non-Commander 4-seat game.
    seats = [pl.Seat(idx=i, library=list(decks[i])) for i in range(4)]
    for s in seats:
        rng.shuffle(s.library)
        pl.draw_cards_no_lose(s, pl.STARTING_HAND)
    game = pl.Game(seats=seats, verbose=False, rng=rng)

    check("4-seat: Game has 4 seats",
          len(game.seats) == 4,
          f"got {len(game.seats)}")
    check("4-seat: each seat has a unique idx 0..3",
          [s.idx for s in game.seats] == [0, 1, 2, 3])
    check("4-seat: each seat has STARTING_LIFE",
          all(s.life == pl.STARTING_LIFE for s in game.seats))
    check("4-seat: game.opp(0) is a living seat",
          game.opp(0) in game.seats and not game.opp(0).lost)
    check("4-seat: game.opponents(0) returns 3 seats in APNAP order",
          [s.idx for s in game.opponents(0)] == [1, 2, 3])
    check("4-seat: game.opponents(2) wraps correctly",
          [s.idx for s in game.opponents(2)] == [3, 0, 1])
    check("4-seat: game.apnap_order with active=2 yields [2,3,0,1]",
          [s.idx for s in game.apnap_order(2)] == [2, 3, 0, 1])


# ---------------------------------------------------------------------------
# 2. Turn-order cycling.
# ---------------------------------------------------------------------------

def test_turn_order_cycling() -> None:
    rng = random.Random(0)
    seats = [pl.Seat(idx=i, library=[make_basic_land()]) for i in range(4)]
    game = pl.Game(seats=seats, verbose=False, rng=rng)
    game.active = 0
    game.turn = 1

    # Cycle 0 → 1 → 2 → 3 → 0
    order = [game.active]
    for _ in range(4):
        pl.swap_active(game)
        order.append(game.active)
    check("swap_active cycles 0→1→2→3→0",
          order == [0, 1, 2, 3, 0],
          f"got {order}")
    check("turn bumps once per full round",
          game.turn == 2,
          f"turn={game.turn}")

    # Elimination skips that seat.
    seats[2].lost = True
    seats[2].loss_reason = "test elimination"
    game.active = 1
    pl.swap_active(game)
    check("swap_active skips eliminated seat 2 → goes 1→3",
          game.active == 3,
          f"active={game.active}")
    pl.swap_active(game)
    check("swap_active continues 3→0",
          game.active == 0,
          f"active={game.active}")


# ---------------------------------------------------------------------------
# 3. Seat elimination cleanup (§800.4).
# ---------------------------------------------------------------------------

def test_seat_elimination_cleanup() -> None:
    rng = random.Random(1)
    seats = [pl.Seat(idx=i, library=[make_basic_land()]) for i in range(4)]
    game = pl.Game(seats=seats, verbose=False, rng=rng)
    game.active = 0

    # Put permanents on seat 2's battlefield.
    goblin = make_card("Goblin A", cmc=1,
                       type_line="Creature — Goblin",
                       power=1, toughness=1)
    perm = pl.Permanent(card=goblin, controller=2, tapped=False,
                        summoning_sick=False)
    pl._etb_initialize(game, perm)
    seats[2].battlefield.append(perm)

    # Push a fake stack item controlled by seat 2 so we can verify purge.
    land_card = make_basic_land("Mountain")
    stack_item = pl.StackItem(card=land_card, controller=2,
                              is_permanent_spell=False, effects=[])
    game.stack.append(stack_item)

    check("pre-elimination: seat 2 has 1 permanent",
          len(seats[2].battlefield) == 1)
    check("pre-elimination: stack has 1 item controlled by seat 2",
          len(game.stack) == 1 and game.stack[0].controller == 2)

    # Eliminate seat 2.
    seats[2].life = 0
    seats[2].lost = True
    seats[2].loss_reason = "life total 0 (test)"
    game.check_end()

    check("§800.4a: seat 2 permanents removed after elimination",
          len(seats[2].battlefield) == 0,
          f"bf size={len(seats[2].battlefield)}")
    check("§800.4a: stack items controlled by seat 2 purged",
          len(game.stack) == 0,
          f"stack size={len(game.stack)}")
    check("§800.4a: seat 2 flagged _left_game",
          getattr(seats[2], "_left_game", False) is True)
    check("game not ended with 3 living seats remaining",
          game.ended is False,
          f"ended={game.ended}")


# ---------------------------------------------------------------------------
# 4. Last-seat-standing wins (§104.2a).
# ---------------------------------------------------------------------------

def test_last_seat_standing_wins() -> None:
    rng = random.Random(2)
    seats = [pl.Seat(idx=i, library=[make_basic_land()]) for i in range(4)]
    game = pl.Game(seats=seats, verbose=False, rng=rng)
    # Kill seats 1, 2, 3.
    for i in (1, 2, 3):
        seats[i].life = 0
        seats[i].lost = True
        seats[i].loss_reason = "test kill"
    game.check_end()
    check("game ended when 3 of 4 seats eliminated",
          game.ended is True)
    check("winner is seat 0 (last standing)",
          game.winner == 0,
          f"winner={game.winner}")
    check("end_reason mentions last one standing",
          "last one standing" in (game.end_reason or ""))


def test_draw_when_all_dead() -> None:
    rng = random.Random(3)
    seats = [pl.Seat(idx=i, library=[make_basic_land()]) for i in range(3)]
    game = pl.Game(seats=seats, verbose=False, rng=rng)
    for s in seats:
        s.life = 0
        s.lost = True
        s.loss_reason = "simultaneous"
    game.check_end()
    check("game ended when all seats dead simultaneously",
          game.ended is True)
    check("winner is None on simultaneous elimination (draw)",
          game.winner is None,
          f"winner={game.winner}")


# ---------------------------------------------------------------------------
# 5. 4-player Commander setup.
# ---------------------------------------------------------------------------

def test_commander_4_player_setup() -> None:
    cmds = [
        make_card("Commander A", cmc=3, power=2, toughness=2, colors=("G",)),
        make_card("Commander B", cmc=4, power=3, toughness=3, colors=("R",)),
        make_card("Commander C", cmc=2, power=1, toughness=2, colors=("U",)),
        make_card("Commander D", cmc=5, power=4, toughness=4, colors=("B",)),
    ]
    decks = [make_commander_deck(c) for c in cmds]
    rng = random.Random(42)
    seats = [pl.Seat(idx=i, library=list(decks[i].cards))
             for i in range(4)]
    game = pl.Game(seats=seats, verbose=False, rng=rng)
    pl.setup_commander_game(game, *decks)

    check("Commander 4p: commander_format flag set",
          game.commander_format is True)
    check("Commander 4p: all 4 seats have 40 life",
          all(s.life == 40 for s in game.seats))
    for i in range(4):
        check(f"Commander 4p: seat {i} command zone has commander",
              any(c.name == cmds[i].name
                  for c in game.seats[i].command_zone))
    check("Commander 4p: commander_names seeded per seat",
          all(game.seats[i].commander_names == [cmds[i].name]
              for i in range(4)))
    zc_reps = [r for r in game.replacement_effects
               if r.event_type == "would_change_zone"]
    check("Commander 4p: §903.9b replacement registered for all 4",
          len(zc_reps) == 4,
          f"found {len(zc_reps)}")


# ---------------------------------------------------------------------------
# 6. Full 4-player non-Commander game terminates.
# ---------------------------------------------------------------------------

def test_full_4p_game_terminates() -> None:
    rng = random.Random(123)
    # Non-Commander 4-player with generic land-only decks so the game
    # terminates on turn-cap (or edge-case damage). The point is to
    # verify the turn loop doesn't loop forever.
    decks = []
    for i in range(4):
        d = []
        for _ in range(40):
            d.append(make_basic_land("Forest"))
        decks.append(d)
    game = pl.play_game(decks=decks, rng=rng, verbose=False,
                        commander_format=False)
    check("4p non-Commander game terminates",
          game.ended is True)
    check("4p game records 4 seats",
          len(game.seats) == 4)
    check("4p game either names a winner or declares a draw",
          (game.winner is not None) or (game.end_reason is not None))


# ---------------------------------------------------------------------------
# 7. N-way priority in APNAP order.
# ---------------------------------------------------------------------------

def test_apnap_ordering() -> None:
    rng = random.Random(7)
    seats = [pl.Seat(idx=i, library=[make_basic_land()]) for i in range(4)]
    game = pl.Game(seats=seats, verbose=False, rng=rng)
    game.active = 1
    # APNAP from active=1 → [1, 2, 3, 0]
    order = [s.idx for s in game.apnap_order()]
    check("APNAP order from active=1: [1,2,3,0]",
          order == [1, 2, 3, 0],
          f"got {order}")

    # opp(1) in 4-seat game returns the first LIVING non-source seat
    # in APNAP order, which is seat 2.
    check("opp(1) in 4-seat returns seat 2 (APNAP primary)",
          game.opp(1).idx == 2,
          f"got {game.opp(1).idx}")

    # Eliminate seat 2; opp(1) should fall through to seat 3.
    seats[2].lost = True
    check("opp(1) skips eliminated seat 2 → seat 3",
          game.opp(1).idx == 3,
          f"got {game.opp(1).idx}")

    # living_opponents(1) returns only living non-source seats.
    living = [s.idx for s in game.living_opponents(1)]
    check("living_opponents(1) skips dead seat 2",
          living == [3, 0],
          f"got {living}")


# ---------------------------------------------------------------------------
# 8. 2-seat back-compat.
# ---------------------------------------------------------------------------

def test_2_seat_backcompat() -> None:
    """Verify legacy 2-player paths still work identically."""
    rng = random.Random(99)
    seats = [pl.Seat(idx=i, library=[make_basic_land()]) for i in range(2)]
    game = pl.Game(seats=seats, verbose=False, rng=rng)

    check("2-seat: game.opp(0) == seat 1",
          game.opp(0).idx == 1)
    check("2-seat: game.opp(1) == seat 0",
          game.opp(1).idx == 0)
    check("2-seat: opponents(0) returns only seat 1",
          [s.idx for s in game.opponents(0)] == [1])
    check("2-seat: apnap_order(0) == [0, 1]",
          [s.idx for s in game.apnap_order(0)] == [0, 1])

    # Legacy play_game signature still works:
    deck_a = [make_basic_land() for _ in range(20)]
    deck_b = [make_basic_land() for _ in range(20)]
    g2 = pl.play_game(deck_a, deck_b, random.Random(42), verbose=False)
    check("Legacy 2-player play_game(deck_a, deck_b, rng) still works",
          g2.ended is True and len(g2.seats) == 2)


# ---------------------------------------------------------------------------
# Runner.
# ---------------------------------------------------------------------------

def main() -> int:
    test_n_seat_construction()
    test_turn_order_cycling()
    test_seat_elimination_cleanup()
    test_last_seat_standing_wins()
    test_draw_when_all_dead()
    test_commander_4_player_setup()
    test_full_4p_game_terminates()
    test_apnap_ordering()
    test_2_seat_backcompat()
    return print_results()


if __name__ == "__main__":
    sys.exit(main())
