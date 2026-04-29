"""OctoHat smoke test.

Runs a small 5-game 4-seat OctoHat-vs-OctoHat gauntlet to verify the
policy doesn't crash and measure event volume per game.

Based on gauntlet_poker._run_one_game_with_policy pattern.
"""
from __future__ import annotations

import os
import random
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import playloop as pl  # noqa: E402
from playloop import Game, Seat, Deck  # noqa: E402
from extensions.policies import OctoHat, GreedyHat  # noqa: E402

DECK_DIR = os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    "..",
    "data",
    "decks",
    "benched",
)

TEST_DECKS = [
    "oloro_lifegain_b2_ageless_ascetic.txt",
    "disguise_precon_b2_kaust.txt",
    "coram_value_b3_the_undertaker.txt",
    "varina_tribal_widetall_b3_lich_queen.txt",
]

N_SEATS = 4
N_GAMES = 5
TURN_CAP = 60


_CARDS_BY_NAME = None


def _get_cards_by_name():
    global _CARDS_BY_NAME
    if _CARDS_BY_NAME is None:
        import json
        from pathlib import Path
        import parser as mtg_parser  # noqa: F401
        ORACLE = os.path.join(
            os.path.dirname(os.path.abspath(__file__)),
            "..", "data", "rules", "oracle-cards.json",
        )
        mtg_parser.load_extensions()
        cards = json.loads(Path(ORACLE).read_text())
        _CARDS_BY_NAME = {c["name"].lower(): c for c in cards
                          if mtg_parser.is_real_card(c)}
    return _CARDS_BY_NAME


def _parse_deck_file(path: str) -> Deck:
    """Reuse gauntlet.py's deck parsing."""
    from gauntlet import parse_deck_file
    result = parse_deck_file(path, _get_cards_by_name())
    if result.deck is None:
        raise RuntimeError(f"Failed to parse {path}: {result.missing}")
    return result.deck


def run_one_game(policy_factory, seed: int):
    rng = random.Random(seed)
    decks = [_parse_deck_file(os.path.join(DECK_DIR, d)) for d in TEST_DECKS]

    seats = [
        Seat(idx=i, library=list(decks[i].cards), policy=policy_factory())
        for i in range(N_SEATS)
    ]
    game = Game(seats=seats, rng=rng)
    for s in seats:
        rng.shuffle(s.library)
    pl.setup_commander_game(game, *decks)
    for s in seats:
        pl.draw_cards_no_lose(s, pl.STARTING_HAND)
    game.active = rng.randint(0, N_SEATS - 1)
    game.ev("game_start", on_the_play=game.active,
            n_seats=N_SEATS, commander_format=True)

    while not game.ended and game.turn <= TURN_CAP:
        pl.take_turn(game)
        if game.ended:
            break
        pl.swap_active(game)

    return game


def main():
    print(f"=== OctoHat smoke test: {N_GAMES} games × {N_SEATS} seats ===")
    print(f"Decks: {TEST_DECKS}")
    print(f"Turn cap: {TURN_CAP}")
    print()

    start = time.time()
    totals = {"events": 0, "casts": 0, "storm_trigger": 0,
              "cast_trigger_observer": 0, "draw": 0, "damage": 0}
    winners = []

    for i in range(N_GAMES):
        try:
            g = run_one_game(OctoHat, seed=42 + i)
        except Exception as e:
            print(f"Game {i}: CRASH — {type(e).__name__}: {e}")
            import traceback
            traceback.print_exc()
            continue

        n_events = len(g.events)
        casts = sum(1 for e in g.events if e.get("type") == "cast")
        storm = sum(1 for e in g.events if e.get("type") == "storm_trigger")
        obs = sum(1 for e in g.events
                  if e.get("type") == "cast_trigger_observer")
        draws = sum(1 for e in g.events if e.get("type") == "draw")
        dmg = sum(1 for e in g.events if e.get("type") == "damage")
        totals["events"] += n_events
        totals["casts"] += casts
        totals["storm_trigger"] += storm
        totals["cast_trigger_observer"] += obs
        totals["draw"] += draws
        totals["damage"] += dmg

        winner = g.winner if g.winner is not None else "DRAW"
        winners.append(winner)
        print(f"Game {i:>2}: {n_events:>5} events, {casts:>3} casts, "
              f"{storm:>2} storm, {obs:>3} obs, {draws:>3} draws, "
              f"{dmg:>3} dmg, winner={winner}, turn={g.turn}, "
              f"end={g.end_reason}")

    elapsed = time.time() - start
    print()
    print(f"=== Completed in {elapsed:.1f}s ===")
    print(f"Total events: {totals['events']} "
          f"({totals['events'] // max(1, N_GAMES)}/game)")
    print(f"Total casts: {totals['casts']}")
    print(f"Storm triggers: {totals['storm_trigger']}")
    print(f"Cast-trigger observers: {totals['cast_trigger_observer']}")
    print(f"Draw events: {totals['draw']}")
    print(f"Damage events: {totals['damage']}")
    print(f"Winners: {winners}")


if __name__ == "__main__":
    main()
