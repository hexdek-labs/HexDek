#!/usr/bin/env python3
"""Parity harness — runs ONE game in the Python reference engine and
emits a JSONL event stream for the Go paritycheck package to diff against.

Usage::

    python3 scripts/parity_harness.py \\
        --decks d1.txt,d2.txt,d3.txt,d4.txt \\
        --seed 123456 \\
        --game-idx 0 \\
        --seats 4 \\
        --output /tmp/py_events.jsonl

Output format (JSONL): every event produced by playloop.Game.ev() on one
line, followed by a final line containing the outcome summary::

    {"_outcome": {"winner": 2, "turns": 19, "end_reason": "last_seat_standing",
                  "life_totals": [0, 0, 14, 0], "lost_by_seat": [true, true, false, true]}}

Seed contract: uses `random.Random(seed)` directly. Go passes
seed = base_seed + game_idx*1000 + 1 so the Go and Python seeds match
1:1. Shuffle deterministic if the Python `random.Random` and Go
`math/rand` happen to produce the same sequence — which they WON'T
because the two language RNGs are different algorithms. The harness
uses Python's native RNG and lets the parity tester flag the resulting
board-state divergences as expected drift.

This harness deliberately reuses ``scripts/gauntlet.parse_deck_file``
so the Python deck parsing matches what the Python gauntlet already
does — no separate code path to drift.
"""

from __future__ import annotations

import argparse
import json
import random
import sys
import traceback
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
sys.path.insert(0, str(HERE))

import parser as mtg_parser  # noqa: E402
import playloop as pl  # noqa: E402
from playloop import Game, Seat  # noqa: E402
from gauntlet import parse_deck_file  # noqa: E402


ORACLE_DUMP = ROOT / "data" / "rules" / "oracle-cards.json"
STARTING_HAND = 7
MAX_TURNS_MULTIPLAYER = 80


def _build_seats(decks, rng):
    seats = []
    for i, d in enumerate(decks):
        seats.append(Seat(idx=i, library=list(d.cards), policy=None))
    return seats


def run_one(decks, seed, game_idx, n_seats):
    """Run a single game and return (events, outcome)."""
    rng = random.Random(seed)
    seats = _build_seats(decks[:n_seats], rng)
    game = Game(seats=seats, rng=rng)
    for s in seats:
        rng.shuffle(s.library)
    pl.setup_commander_game(game, *decks[:n_seats])
    for s in seats:
        pl.draw_cards_no_lose(s, STARTING_HAND)
    game.active = rng.randint(0, n_seats - 1)
    game.ev("game_start", on_the_play=game.active,
            n_seats=n_seats, commander_format=True, game_idx=game_idx)

    turn = 1
    while not game.ended and turn <= MAX_TURNS_MULTIPLAYER:
        game.turn = turn
        try:
            pl.take_turn(game)
        except Exception as exc:
            game.ev("python_harness_error",
                    error=f"{type(exc).__name__}: {exc}",
                    traceback=traceback.format_exc()[:1024])
            break
        if game.ended:
            break
        pl.swap_active(game)
        turn += 1

    # Build outcome.
    winner = -1
    end_reason = ""
    if game.ended:
        if hasattr(game, "winner") and game.winner is not None:
            winner = game.winner
        end_reason = getattr(game, "end_reason", "") or "last_seat_standing"
    else:
        living = [s for s in game.seats if not s.lost]
        if len(living) == 1:
            winner = living[0].idx
            end_reason = "last_seat_standing"
        elif len(living) > 1:
            living.sort(key=lambda s: s.life, reverse=True)
            top_life = living[0].life
            leaders = [s for s in living if s.life == top_life]
            if len(leaders) == 1:
                winner = leaders[0].idx
                end_reason = "turn_cap_leader"
            else:
                end_reason = "turn_cap_tie"
        else:
            end_reason = "turn_cap_all_dead"

    outcome = {
        "winner": winner,
        "winner_name": (decks[winner].commander_name if 0 <= winner < len(decks) else ""),
        "turns": game.turn,
        "end_reason": end_reason,
        "life_totals": [s.life for s in game.seats],
        "lost_by_seat": [bool(s.lost) for s in game.seats],
    }
    return game.events, outcome


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--decks", required=True,
                    help="comma-separated deck paths")
    ap.add_argument("--seed", type=int, required=True)
    ap.add_argument("--game-idx", type=int, default=0)
    ap.add_argument("--seats", type=int, default=4)
    ap.add_argument("--output", required=True,
                    help="JSONL output file")
    args = ap.parse_args()

    deck_paths = [p.strip() for p in args.decks.split(",") if p.strip()]
    if len(deck_paths) < args.seats:
        print(f"error: need {args.seats} decks, got {len(deck_paths)}",
              file=sys.stderr)
        return 1

    # Load oracle once.
    mtg_parser.load_extensions()
    cards = json.loads(ORACLE_DUMP.read_text())
    cards_by_name = {c["name"].lower(): c for c in cards
                     if mtg_parser.is_real_card(c)}

    decks = []
    for p in deck_paths[:args.seats]:
        res = parse_deck_file(p, cards_by_name)
        if res.deck is None:
            print(f"error: could not parse deck {p}: {res.missing}",
                  file=sys.stderr)
            return 1
        decks.append(res.deck)

    events, outcome = run_one(decks, args.seed, args.game_idx, args.seats)

    with open(args.output, "w") as f:
        for evt in events:
            f.write(json.dumps(evt) + "\n")
        f.write(json.dumps({"_outcome": outcome}) + "\n")

    return 0


if __name__ == "__main__":
    sys.exit(main())
