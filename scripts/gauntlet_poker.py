#!/usr/bin/env python3
"""Policy-swap 4-player EDH gauntlet demo.

Sibling to ``scripts/gauntlet.py`` — same deck pool, same game count,
same RNG seeding contract — but drives the engine with
:class:`PokerHat` instead of the default :class:`GreedyHat` on
every seat. The winrate distribution this script reports should differ
from the baseline gauntlet's, proving that the pluggable hat layer
actually changes gameplay outcomes (the "no spaghetti" signal).

This script reuses ``gauntlet.py``'s deck parser and 4p runner where
possible; it only injects the hat. That reuse is the architectural
point: swapping AI hat is a ONE-LINE operation — we don't need a
parallel engine, a parallel game loop, or any other infrastructure.
Grep this file for ``seat.policy = PokerHat()`` to see the full
extent of the change.

Run::

    python3 scripts/gauntlet_poker.py --games 20 --seed 42
"""

from __future__ import annotations

import argparse
import glob
import json
import random
import sys
import traceback
from collections import Counter, defaultdict
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import parser as mtg_parser  # noqa: E402
import playloop as pl  # noqa: E402
from playloop import (  # noqa: E402
    Deck, Game, Seat, play_game,
)
from extensions.policies import GreedyHat, PokerHat  # noqa: E402

# Re-use the deck-parsing bits from gauntlet.py to keep behavior in
# lockstep with the baseline. No need to duplicate the Moxfield parser.
from gauntlet import (  # noqa: E402
    parse_deck_file,
    DeckParseResult,
    N_SEATS,
    ORACLE_DUMP,
    ROOT,
)


def _run_one_game_with_policy(decks_by_seat: dict,
                              rng: random.Random,
                              policy_factory,
                              game_id: str = None,
                              return_game: bool = False,
                              commander_names_by_seat: dict = None) -> dict:
    """Run one 4-player Commander game with the supplied policy
    applied to every seat. Returns a simple summary dict —
    {winner_seat, turns, end_reason, mode_changes}.

    When ``return_game=True`` the returned dict also carries a ``game``
    key with the raw :class:`Game` instance, which the forensic analyzer
    uses to dump event stream + final state.
    """
    # Mirror gauntlet._run_one_4p_game's deck prep.
    fresh_decks = []
    for i in range(N_SEATS):
        d = decks_by_seat[i]
        nd = Deck(cards=list(d.cards), commander_name=d.commander_name)
        nd.commander_cards = list(getattr(d, "commander_cards", []))
        fresh_decks.append(nd)

    # Build seats manually with our chosen policy. This is the
    # demonstration — one line of "plugin" per seat.
    seats = [
        Seat(idx=i, library=list(fresh_decks[i].cards),
             policy=policy_factory())
        for i in range(N_SEATS)
    ]
    game = Game(seats=seats, rng=rng)
    if game_id is not None:
        game.game_id = game_id
    for s in seats:
        rng.shuffle(s.library)
    pl.setup_commander_game(game, *fresh_decks)
    for s in seats:
        pl.draw_cards_no_lose(s, pl.STARTING_HAND)
    game.active = rng.randint(0, N_SEATS - 1)
    game.emit(
        f"policy-gauntlet start — {N_SEATS} seats, "
        f"seat {game.active} on the play")
    game.ev("game_start", on_the_play=game.active,
            n_seats=N_SEATS, commander_format=True)
    game.snapshot()

    turn_cap = pl.MAX_TURNS_MULTIPLAYER
    while not game.ended and game.turn <= turn_cap:
        pl.take_turn(game)
        if game.ended:
            break
        pl.swap_active(game)

    if not game.ended:
        living = [s for s in game.seats if not s.lost]
        if living:
            living.sort(key=lambda s: s.life, reverse=True)
            top_life = living[0].life
            leaders = [s for s in living if s.life == top_life]
            if len(leaders) == 1:
                game.winner = leaders[0].idx
                game.end_reason = f"turn cap; seat {game.winner} ahead"
            else:
                game.winner = None
                game.end_reason = "turn cap; tie"
        game.ended = True

    mode_changes = sum(
        1 for e in game.events if e.get("type") == "player_mode_change"
    )
    result = {
        "winner_seat": game.winner,
        "turns": game.turn,
        "end_reason": game.end_reason,
        "mode_changes": mode_changes,
    }
    if return_game:
        result["game"] = game
        result["decks"] = fresh_decks
        result["commander_names"] = commander_names_by_seat or {}
    return result


def _jsonable(value):
    """Coerce any remaining non-JSON-safe objects (tuples, sets, dataclasses,
    CardEntry, Permanent refs) into plain primitives for dump().

    game.events is already composed of simple dict/str/int/list values —
    but we keep this defensive helper for the final-state dumper which
    walks live Seat/Permanent objects.
    """
    if value is None or isinstance(value, (bool, int, float, str)):
        return value
    if isinstance(value, (list, tuple)):
        return [_jsonable(v) for v in value]
    if isinstance(value, dict):
        return {str(k): _jsonable(v) for k, v in value.items()}
    if isinstance(value, set):
        return sorted(_jsonable(v) for v in value)
    # Fallback — string repr is enough for a forensic dump.
    return repr(value)


def _dump_final_state(game, out_path: str) -> None:
    """Write the final Game snapshot as JSON. Structure is:

        {
          "game_id": "...",
          "winner": int | null,
          "end_reason": "...",
          "total_turns": int,
          "total_events": int,
          "commander_format": bool,
          "day_night": "...",
          "seats": [
            {
              "idx": int,
              "life": int,
              "lost": bool,
              "loss_reason": "...",
              "library_count": int,
              "hand": [card_name, ...],
              "graveyard": [card_name, ...],
              "exile": [card_name, ...],
              "command_zone": [card_name, ...],
              "commander_names": [...],
              "commander_tax": {name: int},
              "commander_damage": {dealer_seat: {name: int}},
              "mana_pool_total": int,
              "battlefield": [
                {"name": ..., "tapped": ..., "summoning_sick": ...,
                 "damage_marked": ..., "power": ..., "toughness": ...,
                 "counters": {...}, "controller": int}
              ]
            },
            ...
          ]
        }
    """
    from pathlib import Path as _P

    def _seat_state(s):
        return {
            "idx": s.idx,
            "life": s.life,
            "lost": bool(s.lost),
            "loss_reason": str(getattr(s, "loss_reason", "")),
            "library_count": len(s.library),
            "hand": [c.name for c in s.hand],
            "graveyard": [c.name for c in s.graveyard],
            "exile": [c.name for c in s.exile],
            "command_zone": [c.name for c in s.command_zone],
            "commander_names": list(s.commander_names),
            "commander_tax": _jsonable(dict(s.commander_tax)),
            "commander_damage": _jsonable(dict(s.commander_damage)),
            "mana_pool_total": s.mana.total(),
            "poison_counters": int(getattr(s, "poison_counters", 0)),
            "spells_cast_this_turn": int(
                getattr(s, "spells_cast_this_turn", 0)),
            "battlefield": [
                {
                    "name": p.card.name,
                    "tapped": bool(p.tapped),
                    "summoning_sick": bool(p.summoning_sick),
                    "damage_marked": int(p.damage_marked),
                    "power": p.power,
                    "toughness": p.toughness,
                    "counters": _jsonable(dict(p.counters)),
                    "controller": int(p.controller),
                    "is_creature": p.is_creature,
                    "is_land": p.is_land,
                    "is_artifact": p.is_artifact,
                    "is_token": p.is_token,
                }
                for p in s.battlefield
            ],
        }

    payload = {
        "game_id": getattr(game, "game_id", None),
        "winner": game.winner,
        "end_reason": game.end_reason,
        "total_turns": game.turn,
        "total_events": len(game.events),
        "commander_format": bool(game.commander_format),
        "day_night": getattr(game, "day_night", "neither"),
        "seats": [_seat_state(s) for s in game.seats],
    }
    _P(out_path).parent.mkdir(parents=True, exist_ok=True)
    _P(out_path).write_text(json.dumps(payload, indent=2))


def _dump_events(game, out_path: str) -> None:
    """Write the event stream as JSONL (one event per line)."""
    from pathlib import Path as _P
    _P(out_path).parent.mkdir(parents=True, exist_ok=True)
    with open(out_path, "w") as f:
        for e in game.events:
            f.write(json.dumps(_jsonable(e)) + "\n")


def _dump_decks(result: dict, out_path: str, policy_name: str,
                commander_names_by_seat: dict) -> None:
    """Write deck metadata — per-seat commander name, policy hat, and
    card-count — as JSON. Consumed by the forensic analyzer's
    deck-signature detectors (storm, ramp, combo, recursion, werewolf)
    which key off commander name and gross cardlist."""
    from pathlib import Path as _P
    fresh_decks = result.get("decks") or []
    payload = {
        "seats": [
            {
                "seat": i,
                "commander_name": commander_names_by_seat.get(i, ""),
                "policy": policy_name,
                "card_count": len(d.cards),
                "card_names": [c.name for c in d.cards],
                "commander_cards": [
                    c.name for c in getattr(d, "commander_cards", [])
                ],
            }
            for i, d in enumerate(fresh_decks)
        ],
    }
    _P(out_path).parent.mkdir(parents=True, exist_ok=True)
    _P(out_path).write_text(json.dumps(payload, indent=2))


def _dump_forensic_artifacts(result: dict, args, commander_names_by_seat: dict,
                             policy_name: str) -> None:
    """Driver — fires the three dumpers based on CLI flags."""
    game = result["game"]
    if args.emit_events:
        _dump_events(game, args.emit_events)
        print(f"  [forensic] wrote {len(game.events)} events → "
              f"{args.emit_events}")
    if args.emit_final_state:
        _dump_final_state(game, args.emit_final_state)
        print(f"  [forensic] wrote final state → {args.emit_final_state}")
    if args.emit_decks:
        _dump_decks(result, args.emit_decks, policy_name,
                    commander_names_by_seat)
        print(f"  [forensic] wrote deck metadata → {args.emit_decks}")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--games", type=int, default=20)
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--decks-dir", type=str,
                    default=str(ROOT / "data" / "decks" / "personal"))
    ap.add_argument("--policy", type=str, default="poker",
                    choices=["greedy", "poker", "octo"],
                    help="policy to attach to every seat")
    # --- Forensic analysis hooks -------------------------------------
    # Single-game dump flags consumed by scripts/analyze_single_game.py.
    # The owner-facing methodology is "test → data collection → analysis
    # → patch → repeat"; the analysis step needs the raw event stream
    # and final state. These flags are opt-in so normal gauntlet runs
    # aren't slowed down by disk I/O.
    ap.add_argument("--emit-events", type=str, default=None,
                    metavar="PATH",
                    help="dump the first game's full game.events list as "
                         "JSONL to PATH (one event per line). Used by "
                         "scripts/analyze_single_game.py.")
    ap.add_argument("--emit-final-state", type=str, default=None,
                    metavar="PATH",
                    help="dump the first game's final Game state (seats, "
                         "battlefield, life totals, commander damage, "
                         "winner, end_reason) as JSON to PATH.")
    ap.add_argument("--emit-decks", type=str, default=None,
                    metavar="PATH",
                    help="dump the loaded decks (commander names, policy "
                         "per seat) as JSON to PATH.")
    ap.add_argument("--emit-game-index", type=int, default=0,
                    help="when --games > 1, choose which game's dump to "
                         "emit (0-indexed). Default 0 (first game).")
    ap.add_argument("--game-id", type=str, default=None,
                    help="stable game_id attached to every event via "
                         "game.game_id (for cross-ref).")
    args = ap.parse_args()

    from extensions.policies import OctoHat  # noqa: E402
    policy_factory = {
        "greedy": GreedyHat,
        "poker": PokerHat,
        "octo": OctoHat,
    }[args.policy]

    print(f"Loading parser extensions + oracle card pool…")
    mtg_parser.load_extensions()
    cards = json.loads(Path(ORACLE_DUMP).read_text())
    cards_by_name = {c["name"].lower(): c for c in cards
                     if mtg_parser.is_real_card(c)}
    print(f"  oracle: {len(cards_by_name)} real cards")

    deck_paths = sorted(glob.glob(str(Path(args.decks_dir) / "*.txt")))
    print(f"Deck files ({len(deck_paths)}):")
    for p in deck_paths:
        print(f"  - {p}")

    # Parse into Deck objects.
    decks: list = []
    commander_names: list = []
    for dpath in deck_paths:
        res: DeckParseResult = parse_deck_file(dpath, cards_by_name)
        if res.deck is None:
            print(f"  SKIP {dpath}: parse failure")
            continue
        decks.append(res.deck)
        commander_names.append(res.commander_name)

    if len(decks) < N_SEATS:
        print(f"ERROR: need {N_SEATS} decks, got {len(decks)}",
              file=sys.stderr)
        return 1

    # Take the first 4 (matches gauntlet.py's default behavior).
    decks = decks[:N_SEATS]
    commander_names = commander_names[:N_SEATS]

    print(f"\nRunning {args.games} games with policy = {args.policy}")
    master_rng = random.Random(args.seed)

    seat_wins: dict = defaultdict(int)
    elim_positions: dict = defaultdict(list)
    total_mode_changes = 0
    crashes = 0
    total_turns = 0

    # When any forensic --emit-* flag is set we need the Game object
    # back from _run_one_game_with_policy. Only keep it for the target
    # game index to avoid holding giant game states in memory for long
    # gauntlet runs.
    want_dump = bool(args.emit_events or args.emit_final_state
                     or args.emit_decks)
    dump_target_idx = args.emit_game_index if want_dump else -1

    for gi in range(args.games):
        # Rotate seating so no commander is stuck at seat 0.
        rot = gi % N_SEATS
        rotated = {i: decks[(i + rot) % N_SEATS] for i in range(N_SEATS)}
        # Build commander-name map for THIS rotation, for seat→cmdr in dump.
        rotated_cmdrs = {
            i: commander_names[(i + rot) % N_SEATS]
            for i in range(N_SEATS)
        }
        per_game_rng = random.Random(master_rng.randint(0, 2**31))
        return_game = (gi == dump_target_idx)
        game_id = args.game_id if return_game else None
        if return_game and game_id is None:
            game_id = f"poker-{args.seed}-g{gi}"
        try:
            result = _run_one_game_with_policy(
                rotated, per_game_rng, policy_factory,
                game_id=game_id,
                return_game=return_game,
                commander_names_by_seat=rotated_cmdrs,
            )
        except Exception:
            crashes += 1
            traceback.print_exc()
            continue
        winner = result["winner_seat"]
        if winner is not None:
            # Map winner back to original commander index via the
            # rotation.
            orig_idx = (winner + rot) % N_SEATS
            seat_wins[orig_idx] += 1
        else:
            seat_wins[None] += 1
        total_mode_changes += result["mode_changes"]
        total_turns += result["turns"]
        print(f"  game {gi}: turn {result['turns']}, winner="
              f"{winner} ({commander_names[(winner + rot) % N_SEATS] if winner is not None else 'DRAW'})"
              f"  mode_changes={result['mode_changes']}")

        # Forensic dumps — fire ONLY for the targeted game index.
        if gi == dump_target_idx and "game" in result:
            _dump_forensic_artifacts(
                result, args, rotated_cmdrs, args.policy,
            )

    print(f"\n── Results ({args.policy} policy on all seats) ──")
    print(f"  games run:    {args.games - crashes}")
    print(f"  crashes:      {crashes}")
    print(f"  avg turns:    {total_turns / max(1, args.games):.1f}")
    print(f"  total mode-change events:  {total_mode_changes}")
    print(f"  avg mode-changes per game: "
          f"{total_mode_changes / max(1, args.games):.1f}")
    print(f"\n  Wins by commander:")
    for i, name in enumerate(commander_names):
        pct = 100 * seat_wins[i] / max(1, args.games)
        print(f"    {name}: {seat_wins[i]}/{args.games} ({pct:.0f}%)")
    if seat_wins[None] > 0:
        print(f"    (draws: {seat_wins[None]})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
