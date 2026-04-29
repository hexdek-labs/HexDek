#!/usr/bin/env python3
"""4-player EDH Commander Gauntlet runner for mtgsquad.

Standalone harness that:
  1. Parses Moxfield-style .txt deck files from data/decks/personal/
  2. Runs N games of Commander (4-player free-for-all if engine supports it,
     otherwise falls back to 2-player round-robin: 6 matchups × games/6 each)
  3. Collects winrate, elimination order, audit violations, parser-gap stats
  4. Writes data/rules/gauntlet_report.md

Contract with playloop.py:
  - We import playloop module members only; we DO NOT monkey-patch.
  - If playloop's Game / play_game isn't yet N-player aware, we run 2-player
    round-robin and note the fallback in the report.
  - All game execution is wrapped in try/except; one crashed game never
    halts the gauntlet.

Usage:
    python3 scripts/gauntlet.py                          # 100 games, seed 42
    python3 scripts/gauntlet.py --games 5 --seed 42      # quick smoke test
    python3 scripts/gauntlet.py --decks-dir path/to/dir  # custom deck dir
"""

from __future__ import annotations

import argparse
import glob
import json
import random
import re
import os
import sys
import traceback
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

# Import playloop + parser. These are hard dependencies; if they fail to
# import we abort — nothing sensible we can do.
import parser as mtg_parser  # noqa: E402
import playloop  # noqa: E402
from playloop import (  # noqa: E402
    CardEntry, Deck, Game, Seat,
    load_card_by_name, play_game,
)

ROOT = HERE.parent
ORACLE_DUMP = ROOT / "data" / "rules" / "oracle-cards.json"
REPORT_PATH = ROOT / "data" / "rules" / "gauntlet_report.md"
AUDIT_EVENTS_PATH = ROOT / "data" / "rules" / "gauntlet_audit_events.jsonl"

COMMANDER_DECK_SIZE = 100  # 99 + commander
# Default N_SEATS; overridable via MTGSQUAD_N_SEATS env var or --seats CLI flag.
N_SEATS = int(os.environ.get("MTGSQUAD_N_SEATS", "4"))


# ============================================================================
# Deck parsing
# ============================================================================

# Moxfield line: "1 CardName (SET) NUMBER" or "1 CardName". Trailing newline
# + "COMMANDER: Name" footer. Commander line may or may not appear in the
# main list as a "1 X" entry (both patterns appear in the personal decks).
_LINE_RE = re.compile(
    r"^\s*(\d+)\s+"                      # count
    r"(.+?)"                             # name (non-greedy)
    r"(?:\s+\([0-9A-Za-z-]+\)\s+\S+)?"   # optional (SET) NUMBER suffix
    r"\s*$"
)
_COMMANDER_RE = re.compile(r"^\s*COMMANDER\s*:\s*(.+?)\s*$", re.IGNORECASE)
# Partner line — CR §702.124 / §903.3c. Decks with a partner pair list
# the second commander on this line:
#     COMMANDER: Kraum, Ludevic's Opus
#     PARTNER: Tymna the Weaver
# We don't enforce partner legality at parse time; that's a runtime check.
_PARTNER_RE = re.compile(r"^\s*PARTNER\s*:\s*(.+?)\s*$", re.IGNORECASE)


@dataclass
class DeckParseResult:
    """Result of parsing a .txt decklist file."""
    deck: Optional[Deck]
    commander_name: Optional[str]
    total_lines: int                     # non-empty, non-commander lines
    cards_requested: int                 # sum of count * line
    cards_found: int                     # cards found in oracle pool
    missing: list[str] = field(default_factory=list)  # names not found
    basics_padded: int = 0               # extra basics added to hit 99
    path: str = ""


def parse_deck_file(path: str,
                    cards_by_name: dict) -> DeckParseResult:
    """Parse a Moxfield-style .txt decklist into a Deck for Commander play.

    Args:
        path: Absolute path to the .txt file.
        cards_by_name: dict keyed by lowercase card name (matches
                       playloop.load_card_by_name's expectation).

    Returns:
        DeckParseResult with a populated `deck` field on success. On parse
        failure (no commander found / zero cards matched), `deck` is None
        and the caller should skip this deck.

    Notes on Commander setup:
        - Per §903.6 the commander card is physically moved to command_zone
          at setup time. setup_commander_game() does this work: it looks
          first in deck.commander_cards, then in seat.library, pulling the
          card out. We build deck.cards to contain the 99-card remainder
          (excluding the commander) so there's no double-counting.
        - If the decklist includes the commander as "1 CardName" in the
          main list AND names it in `COMMANDER: CardName`, we strip one
          copy (leaving 99 cards). If the main list excludes the commander
          already (Ragost-style), we leave it alone.
    """
    p = Path(path)
    text = p.read_text(errors="replace")
    lines = text.splitlines()

    commander_name: Optional[str] = None
    partner_name: Optional[str] = None
    raw_lines: list[tuple[int, str]] = []   # (count, name)
    missing: list[str] = []

    for raw in lines:
        line = raw.strip()
        if not line:
            continue
        cm = _COMMANDER_RE.match(line)
        if cm:
            commander_name = cm.group(1).strip()
            continue
        pm = _PARTNER_RE.match(line)
        if pm:
            partner_name = pm.group(1).strip()
            continue
        m = _LINE_RE.match(line)
        if not m:
            # Garbage line; log and skip
            continue
        count = int(m.group(1))
        name = m.group(2).strip()
        raw_lines.append((count, name))

    cards_requested = sum(c for c, _ in raw_lines)

    # Build CardEntry list. cards_by_name is case-insensitive (lowercase keys).
    card_entries: list[CardEntry] = []
    commander_entry: Optional[CardEntry] = None
    partner_entry: Optional[CardEntry] = None

    # Helper: look up by name, cache.
    _lookup_cache: dict[str, Optional[CardEntry]] = {}

    def _lookup(name: str) -> Optional[CardEntry]:
        key = name.lower()
        if key in _lookup_cache:
            return _lookup_cache[key]
        ce = None
        try:
            ce = load_card_by_name(cards_by_name, name)
        except Exception:
            # parse_card inside load_card_by_name can throw on weird cards.
            ce = None
        # Fallback: try face-matching DFC "A // B" entries.
        if ce is None:
            for lcase, c in cards_by_name.items():
                full = c.get("name") or ""
                if " // " in full:
                    faces = [f.strip().lower() for f in full.split(" // ")]
                    if key in faces:
                        try:
                            ce = load_card_by_name(cards_by_name, full)
                        except Exception:
                            ce = None
                        break
        _lookup_cache[key] = ce
        return ce

    for count, name in raw_lines:
        ce = _lookup(name)
        if ce is None:
            missing.append(name)
            continue
        if commander_name and name.lower() == commander_name.lower():
            # This line IS the commander copy; remember it and don't put
            # its copies in the main library.
            commander_entry = ce
            continue
        if partner_name and name.lower() == partner_name.lower():
            # Line IS the partner copy. Singleton, don't library.
            partner_entry = ce
            continue
        for _ in range(count):
            card_entries.append(ce)

    # If commander wasn't in the main list, resolve it standalone.
    if commander_name and commander_entry is None:
        commander_entry = _lookup(commander_name)
        if commander_entry is None:
            missing.append(commander_name)
    # Same for partner — CR §702.124 / §903.3c.
    if partner_name and partner_entry is None:
        partner_entry = _lookup(partner_name)
        if partner_entry is None:
            missing.append(partner_name)

    # Validate: must have a commander, must have cards.
    if not commander_name or commander_entry is None:
        return DeckParseResult(
            deck=None, commander_name=commander_name,
            total_lines=len(raw_lines),
            cards_requested=cards_requested,
            cards_found=len(card_entries),
            missing=missing, basics_padded=0,
            path=str(p),
        )

    if not card_entries:
        return DeckParseResult(
            deck=None, commander_name=commander_name,
            total_lines=len(raw_lines),
            cards_requested=cards_requested,
            cards_found=0,
            missing=missing, basics_padded=0,
            path=str(p),
        )

    # Pad with basic lands to reach the deck target. For single-commander
    # decks that's 99 (100 - commander); for partner decks it's 98
    # (100 - two commanders). CR §903.5b: Commander deck is always
    # exactly 100 cards including all commander(s).
    commander_slots = 1 + (1 if partner_entry is not None else 0)
    target = COMMANDER_DECK_SIZE - commander_slots
    basics_padded = 0
    if len(card_entries) < target:
        basic_count: Counter = Counter()
        for e in card_entries:
            if e.type_line.lower().startswith("basic land"):
                basic_count[e.name] += 1
        if basic_count:
            fill = basic_count.most_common(1)[0][0]
        else:
            color_to_basic = {
                "W": "Plains", "U": "Island", "B": "Swamp",
                "R": "Mountain", "G": "Forest",
            }
            if commander_entry and commander_entry.colors:
                fill = color_to_basic.get(
                    commander_entry.colors[0], "Wastes")
            else:
                fill = "Wastes"
        fill_ce = _lookup(fill)
        if fill_ce is None:
            fill_ce = _lookup("Wastes")
        while fill_ce is not None and len(card_entries) < target:
            card_entries.append(fill_ce)
            basics_padded += 1

    # Trim excess (Commander is singleton but decks can overshoot if
    # commander was duplicated etc.)
    if len(card_entries) > target:
        card_entries = card_entries[:target]

    # Build commander_cards + commander_names for setup_commander_game.
    # Partner decks get BOTH cards in the command zone (CR §903.6).
    commander_cards_list = [commander_entry]
    commander_names_list = [commander_name]
    if partner_entry is not None and partner_name is not None:
        commander_cards_list.append(partner_entry)
        commander_names_list.append(partner_name)

    deck = Deck(
        cards=card_entries,
        commander_name=commander_name,
        commander_names=commander_names_list,
    )
    # setup_commander_game() consults `commander_cards` to place the
    # commander(s) into the command zone. We provide it so we don't have
    # to monkey-patch seat.library.
    deck.commander_cards = commander_cards_list

    return DeckParseResult(
        deck=deck, commander_name=commander_name,
        total_lines=len(raw_lines),
        cards_requested=cards_requested,
        cards_found=len(card_entries),
        missing=missing, basics_padded=basics_padded,
        path=str(p),
    )


# ============================================================================
# Multiplayer capability detection
# ============================================================================

def supports_n_player(n: int) -> bool:
    """Return True iff playloop.play_game supports an N-seat game.

    Heuristic: check the signature of play_game for an `n_seats` parameter
    OR accept a `decks` list. Right now playloop takes (deck_a, deck_b).
    The parallel agent is extending; we detect at import time.
    """
    import inspect
    try:
        sig = inspect.signature(play_game)
    except (TypeError, ValueError):
        return False
    params = set(sig.parameters.keys())
    # If any of these are present, the extension has landed.
    return ("n_seats" in params or
            "decks" in params or
            "seats" in params)


# ============================================================================
# Game result normalization
# ============================================================================

@dataclass
class GameResult:
    """Normalized outcome of a single game (2p or 4p)."""
    winner_seat: Optional[int]              # None for draw/TLE
    seat_to_commander: dict                 # seat_idx → commander_name
    turns: int
    end_reason: str
    elimination_order: list                 # [(seat, turn_died), ...]
    final_battlefield: dict                 # seat_idx → [card_name, ...]
    events: list                            # full audit event stream
    unknown_nodes: Counter                  # parser gaps
    crashed: bool = False
    crash_message: str = ""


def _elimination_order_from_events(events: list) -> list:
    """Scan audit events for seat-loss sequence."""
    order: list[tuple[int, int]] = []
    seen_losers: set = set()
    for e in events:
        t = e.get("type") or ""
        if t in ("seat_loss", "player_loss", "game_loss",
                 "lose_game", "sba_704_5a", "sba_704_5b", "sba_704_5c",
                 "sba_704_6c"):
            seat = e.get("seat") if "seat" in e else e.get("loser")
            if seat is None:
                # Try nested
                for k in ("target_seat", "player", "loser_seat"):
                    if k in e:
                        seat = e[k]
                        break
            if seat is not None and seat not in seen_losers:
                seen_losers.add(seat)
                order.append((seat, e.get("turn", 0)))
    return order


def _final_battlefield_from_game(game: Game) -> dict:
    """Snapshot each seat's battlefield at end-of-game."""
    out: dict = {}
    try:
        for s in game.seats:
            out[s.idx] = [p.card.name for p in s.battlefield]
    except Exception:
        pass
    return out


def _run_one_2p_game(decks_by_seat: dict,
                    rng: random.Random) -> GameResult:
    """Run a single 2-player Commander game.

    decks_by_seat: {0: Deck, 1: Deck}.
    Returns a GameResult regardless of success (crashed=True on failure).
    """
    deck_a = decks_by_seat[0]
    deck_b = decks_by_seat[1]
    seat_to_commander = {
        0: deck_a.commander_name,
        1: deck_b.commander_name,
    }
    try:
        # IMPORTANT: play_game shuffles the deck.cards into seat.library in
        # Commander mode; it mutates the Deck's own cards list. We pass a
        # shallow-copied Deck so the original template stays untouched
        # between games.
        da = Deck(cards=list(deck_a.cards),
                  commander_name=deck_a.commander_name)
        da.commander_cards = list(getattr(deck_a, "commander_cards", []))
        db = Deck(cards=list(deck_b.cards),
                  commander_name=deck_b.commander_name)
        db.commander_cards = list(getattr(deck_b, "commander_cards", []))

        game = play_game(da, db, rng, verbose=False, commander_format=True)

        winner = game.winner
        turns = game.turn
        end_reason = game.end_reason or ""
        elim = _elimination_order_from_events(game.events)
        # Ensure loser is in elim if game ended with a winner and nobody
        # logged a loss event.
        if winner is not None and not any(s == (1 - winner) for s, _ in elim):
            elim.append((1 - winner, turns))
        final_bf = _final_battlefield_from_game(game)
        return GameResult(
            winner_seat=winner,
            seat_to_commander=seat_to_commander,
            turns=turns,
            end_reason=end_reason,
            elimination_order=elim,
            final_battlefield=final_bf,
            events=list(game.events),
            unknown_nodes=Counter(game.unknown_nodes),
        )
    except Exception as e:
        return GameResult(
            winner_seat=None,
            seat_to_commander=seat_to_commander,
            turns=0, end_reason="crashed",
            elimination_order=[],
            final_battlefield={},
            events=[],
            unknown_nodes=Counter(),
            crashed=True,
            crash_message=f"{type(e).__name__}: {e}\n{traceback.format_exc()}",
        )


_LAST_RUN_GAME = {"game": None, "decks": None}


def _run_one_4p_game(decks_by_seat: dict,
                    rng: random.Random) -> GameResult:
    """Run a single 4-player Commander free-for-all game.

    Attempts to use play_game's N-player support. If the signature doesn't
    support it, raises RuntimeError — callers should use the 2p fallback.
    """
    import inspect
    sig = inspect.signature(play_game)
    params = sig.parameters
    seat_to_commander = {
        i: decks_by_seat[i].commander_name for i in range(N_SEATS)
    }

    # Prepare fresh Deck copies so play_game can mutate library safely.
    fresh_decks = []
    for i in range(N_SEATS):
        d = decks_by_seat[i]
        nd = Deck(cards=list(d.cards), commander_name=d.commander_name)
        nd.commander_cards = list(getattr(d, "commander_cards", []))
        fresh_decks.append(nd)

    try:
        # Try the most-likely API shapes. Order of preference:
        # 1. play_game(decks=[...], rng=..., commander_format=True, n_seats=4)
        # 2. play_game(decks=[...], rng=..., commander_format=True)
        # 3. play_game(*fresh_decks, rng=..., commander_format=True, n_seats=4)
        kwargs = {"rng": rng, "commander_format": True}
        if "n_seats" in params:
            kwargs["n_seats"] = N_SEATS
        if "decks" in params:
            game = play_game(decks=fresh_decks, **kwargs)
        elif "seats" in params:
            game = play_game(seats=fresh_decks, **kwargs)
        else:
            # Last-resort positional — unlikely to work, but try
            game = play_game(*fresh_decks, **kwargs)
    except Exception as e:
        return GameResult(
            winner_seat=None,
            seat_to_commander=seat_to_commander,
            turns=0, end_reason="crashed",
            elimination_order=[], final_battlefield={},
            events=[], unknown_nodes=Counter(),
            crashed=True,
            crash_message=f"{type(e).__name__}: {e}\n{traceback.format_exc()}",
        )

    # Stash the live Game object for forensic --emit-* dumps. The
    # gauntlet driver below reads _LAST_RUN_GAME after each game and
    # writes JSON if the caller asked for dumps. This keeps the
    # GameResult dataclass pure/serializable (old consumers unchanged)
    # while giving the forensic path access to live battlefield state.
    _LAST_RUN_GAME["game"] = game
    _LAST_RUN_GAME["decks"] = fresh_decks
    _LAST_RUN_GAME["seat_to_commander"] = seat_to_commander

    return GameResult(
        winner_seat=game.winner,
        seat_to_commander=seat_to_commander,
        turns=game.turn,
        end_reason=game.end_reason or "",
        elimination_order=_elimination_order_from_events(game.events),
        final_battlefield=_final_battlefield_from_game(game),
        events=list(game.events),
        unknown_nodes=Counter(game.unknown_nodes),
    )


# ============================================================================
# Gauntlet runner
# ============================================================================

@dataclass
class GauntletReport:
    decks: list                              # list of DeckParseResult
    n_games: int
    mode: str                                # "4p-free-for-all" | "2p-round-robin"
    games_run: int
    games_crashed: int
    winner_by_commander: Counter             # commander_name → wins
    games_by_commander: Counter              # commander_name → games played
    elimination_positions: dict              # commander_name → list[position]
    turns: list                              # list[int]
    turn_cap_hits: int
    top_violations: list                     # [(rule, count, example), ...]
    violations_total: int
    unknown_nodes: Counter                   # union across all games
    winner_battlefield_appearance: Counter   # card_name → games-won-with
    crashes: list                            # list[str] of crash messages (first 10)


def _aggregate(results: list[GameResult],
               audit_violations: Counter,
               audit_examples: dict) -> dict:
    """Collapse a list of GameResult into aggregate stats."""
    wins_by_cmd: Counter = Counter()
    games_by_cmd: Counter = Counter()
    elim_positions: dict = defaultdict(list)
    turns = []
    turn_cap_hits = 0
    unknown_nodes: Counter = Counter()
    winner_bf: Counter = Counter()
    crashes: list = []
    games_run = 0
    games_crashed = 0

    for r in results:
        if r.crashed:
            games_crashed += 1
            crashes.append(r.crash_message.splitlines()[0])
            continue
        games_run += 1
        turns.append(r.turns)
        if "turn cap" in (r.end_reason or "").lower():
            turn_cap_hits += 1
        # Count each seat's commander as having played this game
        for seat_idx, cmd in r.seat_to_commander.items():
            games_by_cmd[cmd] += 1
        # Winner bookkeeping
        if r.winner_seat is not None:
            winner_cmd = r.seat_to_commander.get(r.winner_seat)
            if winner_cmd:
                wins_by_cmd[winner_cmd] += 1
                # Battlefield at win-time
                for cname in r.final_battlefield.get(r.winner_seat, []):
                    winner_bf[cname] += 1
        # Elimination positions: for each loser, position = their order index
        for pos, (seat, _turn) in enumerate(r.elimination_order):
            cmd = r.seat_to_commander.get(seat)
            if cmd:
                elim_positions[cmd].append(pos)
        # Winner is at the last position (if there was one)
        if r.winner_seat is not None:
            winner_cmd = r.seat_to_commander.get(r.winner_seat)
            if winner_cmd:
                elim_positions[winner_cmd].append(
                    len(r.seat_to_commander) - 1)
        # Parser-gap union
        for k, v in r.unknown_nodes.items():
            unknown_nodes[k] += v

    # Top violations
    top_viol: list = []
    for rule, count in audit_violations.most_common(20):
        ex = audit_examples.get(rule, "")
        top_viol.append((rule, count, ex))

    return {
        "wins_by_cmd": wins_by_cmd,
        "games_by_cmd": games_by_cmd,
        "elim_positions": dict(elim_positions),
        "turns": turns,
        "turn_cap_hits": turn_cap_hits,
        "unknown_nodes": unknown_nodes,
        "winner_bf": winner_bf,
        "crashes": crashes,
        "games_run": games_run,
        "games_crashed": games_crashed,
        "top_violations": top_viol,
        "violations_total": sum(audit_violations.values()),
    }


def _forensic_dump_game(game, decks, seat_to_commander: dict,
                        emit_events: str = None,
                        emit_final_state: str = None,
                        emit_decks: str = None,
                        policy_name: str = "default") -> None:
    """Mirror of gauntlet_poker._dump_* — shared dump helpers for the
    regular gauntlet. Kept inline to avoid a cross-script import (these
    two scripts already have a brittle coupling that Josh wants to
    unblock, not tighten)."""
    from pathlib import Path as _P

    def _jsonable(v):
        if v is None or isinstance(v, (bool, int, float, str)):
            return v
        if isinstance(v, (list, tuple)):
            return [_jsonable(x) for x in v]
        if isinstance(v, dict):
            return {str(k): _jsonable(x) for k, x in v.items()}
        if isinstance(v, set):
            return sorted(_jsonable(x) for x in v)
        return repr(v)

    if emit_events:
        _P(emit_events).parent.mkdir(parents=True, exist_ok=True)
        with open(emit_events, "w") as f:
            for e in game.events:
                f.write(json.dumps(_jsonable(e)) + "\n")
        print(f"  [forensic] wrote {len(game.events)} events → "
              f"{emit_events}")

    if emit_final_state:
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
        _P(emit_final_state).parent.mkdir(parents=True, exist_ok=True)
        _P(emit_final_state).write_text(json.dumps(payload, indent=2))
        print(f"  [forensic] wrote final state → {emit_final_state}")

    if emit_decks:
        payload = {
            "seats": [
                {
                    "seat": i,
                    "commander_name": seat_to_commander.get(i, ""),
                    "policy": policy_name,
                    "card_count": len(d.cards),
                    "card_names": [c.name for c in d.cards],
                    "commander_cards": [
                        c.name for c in getattr(d, "commander_cards", [])
                    ],
                }
                for i, d in enumerate(decks)
            ],
        }
        _P(emit_decks).parent.mkdir(parents=True, exist_ok=True)
        _P(emit_decks).write_text(json.dumps(payload, indent=2))
        print(f"  [forensic] wrote deck metadata → {emit_decks}")


def run_gauntlet(deck_paths: list,
                 n_games: int,
                 seed: int,
                 cards_by_name: dict,
                 verbose: bool = False,
                 forensic: dict = None) -> GauntletReport:
    """Run the gauntlet: N games of EDH with the given deck files.

    If 4-player is supported by play_game, runs free-for-all; otherwise
    falls back to 2-player round-robin (every deck vs every other, games
    distributed across pairings).
    """
    # Parse all decks
    parse_results = []
    for path in deck_paths:
        pr = parse_deck_file(path, cards_by_name)
        parse_results.append(pr)
        if pr.deck:
            print(f"  {Path(path).name}: commander={pr.commander_name}, "
                  f"{pr.cards_found} cards matched "
                  f"({len(pr.missing)} missing, "
                  f"{pr.basics_padded} basics padded)")
        else:
            print(f"  SKIP {Path(path).name}: parse failed "
                  f"(commander={pr.commander_name}, "
                  f"missing={pr.missing[:5]})")

    valid = [pr for pr in parse_results if pr.deck is not None]
    if len(valid) < 2:
        print("ERROR: need at least 2 valid decks for any gauntlet mode")
        return GauntletReport(
            decks=parse_results, n_games=n_games,
            mode="none", games_run=0, games_crashed=0,
            winner_by_commander=Counter(), games_by_commander=Counter(),
            elimination_positions={}, turns=[], turn_cap_hits=0,
            top_violations=[], violations_total=0,
            unknown_nodes=Counter(), winner_battlefield_appearance=Counter(),
            crashes=[],
        )

    # Choose mode
    can_4p = (len(valid) >= N_SEATS) and supports_n_player(N_SEATS)
    mode = "4p-free-for-all" if can_4p else "2p-round-robin"
    print(f"\nGauntlet mode: {mode}")
    if not can_4p:
        reason = ("insufficient valid decks" if len(valid) < N_SEATS
                  else "playloop.play_game does not yet support n_seats "
                       "(N-player extension hasn't landed)")
        print(f"  (fallback reason: {reason})")

    # Collect GameResult list; write events to audit JSONL for post-hoc
    # auditor run.
    all_results: list[GameResult] = []
    AUDIT_EVENTS_PATH.parent.mkdir(parents=True, exist_ok=True)
    audit_fh = AUDIT_EVENTS_PATH.open("w")

    def _flush_events(match_label: str, game_idx: int, events: list) -> None:
        for evt in events:
            rec = dict(evt)
            rec["_matchup"] = match_label
            rec["_game"] = game_idx
            audit_fh.write(json.dumps(rec) + "\n")

    try:
        if can_4p:
            decks_list = [pr.deck for pr in valid[:N_SEATS]]
            for g in range(n_games):
                # Rotate seat assignment by game so each commander visits
                # every seat equally often over N_SEATS games.
                rot = g % N_SEATS
                decks_by_seat = {
                    i: decks_list[(i + rot) % N_SEATS]
                    for i in range(N_SEATS)
                }
                rng = random.Random(seed + g)
                r = _run_one_4p_game(decks_by_seat, rng)
                label = "4p_ffa"
                _flush_events(label, g, r.events)
                all_results.append(r)
                if verbose or g < 3 or r.crashed:
                    status = "CRASH" if r.crashed else (
                        f"seat {r.winner_seat} "
                        f"({r.seat_to_commander.get(r.winner_seat, '—')})"
                        if r.winner_seat is not None else "draw")
                    print(f"  game {g}: turn {r.turns}, winner={status}")
                # Forensic single-game dump target — fire exactly once
                # for the caller-specified game index. The live Game is
                # stashed in _LAST_RUN_GAME by _run_one_4p_game above.
                if (forensic and g == forensic.get("game_index", 0)
                        and _LAST_RUN_GAME["game"] is not None):
                    gm = _LAST_RUN_GAME["game"]
                    if forensic.get("game_id"):
                        gm.game_id = forensic["game_id"]
                    _forensic_dump_game(
                        gm,
                        _LAST_RUN_GAME["decks"],
                        _LAST_RUN_GAME["seat_to_commander"],
                        emit_events=forensic.get("emit_events"),
                        emit_final_state=forensic.get("emit_final_state"),
                        emit_decks=forensic.get("emit_decks"),
                        policy_name="default",
                    )
        else:
            # 2-player round-robin. Produce all pairs (i, j) with i<j,
            # distribute n_games evenly across pairings — any remainder is
            # spread across the first few pairs (so 5 games × 3 pairs →
            # 2/2/1 instead of 1/1/1). Caller gets ≥ n_games total games.
            pairs = []
            for i in range(len(valid)):
                for j in range(i + 1, len(valid)):
                    pairs.append((i, j))
            base = max(1, n_games // len(pairs))
            remainder = max(0, n_games - base * len(pairs))
            pair_counts = [base + (1 if idx < remainder else 0)
                           for idx in range(len(pairs))]
            total_games = sum(pair_counts)
            print(f"  {len(pairs)} pairings, {total_games} total games "
                  f"(distribution: {pair_counts})")
            game_idx = 0
            for (i, j), per_pair in zip(pairs, pair_counts):
                label = (f"{valid[i].commander_name} vs "
                         f"{valid[j].commander_name}")
                for g in range(per_pair):
                    decks_by_seat = {0: valid[i].deck, 1: valid[j].deck}
                    rng = random.Random(seed + game_idx)
                    r = _run_one_2p_game(decks_by_seat, rng)
                    _flush_events(label, g, r.events)
                    all_results.append(r)
                    if verbose or g < 1 or r.crashed:
                        status = "CRASH" if r.crashed else (
                            f"seat {r.winner_seat} "
                            f"({r.seat_to_commander.get(r.winner_seat, '—')})"
                            if r.winner_seat is not None else "draw")
                        print(f"    {label} [{g}]: turn {r.turns}, "
                              f"winner={status}")
                    game_idx += 1
    finally:
        audit_fh.close()

    # Run the rule auditor on the event log to surface violations.
    audit_viol_counts: Counter = Counter()
    audit_examples: dict = {}
    try:
        import rule_auditor
        viol = rule_auditor.Violations()
        for _match, _gid, evts in rule_auditor.group_by_game(
                rule_auditor.iter_events(AUDIT_EVENTS_PATH)):
            try:
                rule_auditor.audit_game(evts, viol)
            except Exception as e:
                print(f"  auditor crashed on one game: {e}")
        audit_viol_counts = viol.counts
        # Build example map: first occurrence's detail per rule
        for item in viol.items:
            rule = item["rule"]
            if rule not in audit_examples:
                audit_examples[rule] = item.get("detail", "")
    except Exception as e:
        print(f"  auditor unavailable: {e}")

    agg = _aggregate(all_results, audit_viol_counts, audit_examples)

    return GauntletReport(
        decks=parse_results,
        n_games=n_games,
        mode=mode,
        games_run=agg["games_run"],
        games_crashed=agg["games_crashed"],
        winner_by_commander=agg["wins_by_cmd"],
        games_by_commander=agg["games_by_cmd"],
        elimination_positions=agg["elim_positions"],
        turns=agg["turns"],
        turn_cap_hits=agg["turn_cap_hits"],
        top_violations=agg["top_violations"],
        violations_total=agg["violations_total"],
        unknown_nodes=agg["unknown_nodes"],
        winner_battlefield_appearance=agg["winner_bf"],
        crashes=agg["crashes"][:10],
    )


# ============================================================================
# Report writer
# ============================================================================

def write_report(report: GauntletReport, path: Path,
                 args: argparse.Namespace) -> None:
    """Render the gauntlet report as Markdown."""
    lines: list[str] = []
    ap = lines.append

    ap(f"# 4-Player EDH Gauntlet — Round 1")
    ap("")
    deck_names = [(pr.commander_name or Path(pr.path).stem)
                  for pr in report.decks if pr.deck is not None]
    bad = [Path(pr.path).stem for pr in report.decks if pr.deck is None]
    ap(f"**Decks:** {', '.join(deck_names) if deck_names else '—'}. "
       f"{report.n_games} games requested, seed {args.seed}. "
       f"Mode: **{report.mode}**. "
       f"Standard EDH (40 life, 100-card singleton, commander damage 21, "
       f"free-for-all).")
    if bad:
        ap(f"**Unparseable decks (skipped):** {', '.join(bad)}")
    ap("")

    # --- Deck parse summary ------------------------------------------------
    ap("## Deck parse summary")
    ap("")
    ap("| Deck | Commander | Cards requested | Cards matched | Missing | "
       "Basics padded |")
    ap("|---|---|---:|---:|---:|---:|")
    for pr in report.decks:
        name = Path(pr.path).stem
        cmd = pr.commander_name or "—"
        miss_n = len(pr.missing)
        found = pr.cards_found
        ap(f"| {name} | {cmd} | {pr.cards_requested} | {found} | "
           f"{miss_n} | {pr.basics_padded} |")
    ap("")

    # Detailed missing list, if any
    any_missing = any(pr.missing for pr in report.decks)
    if any_missing:
        ap("### Missing cards (per deck)")
        for pr in report.decks:
            if pr.missing:
                ap(f"- **{Path(pr.path).stem}** ({len(pr.missing)} missing): "
                   f"{', '.join(pr.missing[:10])}"
                   + (" …" if len(pr.missing) > 10 else ""))
        ap("")

    # --- Run summary -------------------------------------------------------
    ap("## Run summary")
    ap("")
    ap(f"- Games completed: **{report.games_run}**")
    ap(f"- Games crashed:   **{report.games_crashed}**")
    ap(f"- Turn-cap hits:   **{report.turn_cap_hits}**")
    if report.turns:
        avg = sum(report.turns) / len(report.turns)
        med = sorted(report.turns)[len(report.turns) // 2]
        ap(f"- Avg turns / median / min / max: "
           f"**{avg:.1f}** / **{med}** / {min(report.turns)} / "
           f"{max(report.turns)}")
    else:
        ap("- No games completed — check crashes below.")
    ap(f"- Audit events written to: `{AUDIT_EVENTS_PATH.relative_to(ROOT) if ROOT in AUDIT_EVENTS_PATH.parents else AUDIT_EVENTS_PATH}`")
    ap("")

    # --- Winrate matrix ----------------------------------------------------
    ap("## Winrate matrix")
    ap("")
    if report.games_run:
        ap("| Commander | Games | Wins | Win % | Avg elim position* |")
        ap("|---|---:|---:|---:|---:|")
        for cmd in sorted(report.games_by_commander.keys()):
            games = report.games_by_commander[cmd]
            wins = report.winner_by_commander[cmd]
            pct = (100.0 * wins / games) if games else 0.0
            positions = report.elimination_positions.get(cmd, [])
            avg_pos = (sum(positions) / len(positions)) if positions else 0.0
            ap(f"| {cmd} | {games} | {wins} | {pct:.1f}% | {avg_pos:.2f} |")
        ap("")
        ap("_\\* position 0 = first out; higher = stayed alive longer. "
           "Winner gets position N-1._")
    else:
        ap("_No games completed; no winrate data._")
    ap("")

    # --- Elimination order -------------------------------------------------
    ap("## Elimination order (frequency by slot)")
    ap("")
    if report.games_run and report.elimination_positions:
        max_pos = 0
        for positions in report.elimination_positions.values():
            if positions:
                max_pos = max(max_pos, max(positions))
        header = ["Commander"] + [f"Slot {i}" for i in range(max_pos + 1)]
        ap("| " + " | ".join(header) + " |")
        ap("|" + "|".join(["---"] + [":-:"] * (len(header) - 1)) + "|")
        for cmd in sorted(report.elimination_positions.keys()):
            positions = report.elimination_positions[cmd]
            bucket = Counter(positions)
            row = [cmd] + [str(bucket.get(i, 0)) for i in range(max_pos + 1)]
            ap("| " + " | ".join(row) + " |")
    else:
        ap("_No elimination data._")
    ap("")

    # --- Audit violations --------------------------------------------------
    ap("## Audit violations (top 20 by frequency)")
    ap("")
    ap(f"_Total violations: **{report.violations_total}**_")
    ap("")
    if report.top_violations:
        ap("| Rule | Count | Example |")
        ap("|---|---:|---|")
        for rule, cnt, ex in report.top_violations:
            # Truncate long example strings and escape pipe.
            ex_short = (ex or "").replace("|", "\\|").replace("\n", " ")
            if len(ex_short) > 120:
                ex_short = ex_short[:117] + "…"
            ap(f"| {rule} | {cnt} | {ex_short} |")
    else:
        ap("_No violations recorded (either auditor unavailable or the "
           "engine stayed inside the rules — unlikely on personal decks)._")
    ap("")

    # --- Parser gaps -------------------------------------------------------
    ap("## Parser gaps encountered")
    ap("")
    if report.unknown_nodes:
        ap("Engine-level unknown-node fallbacks seen during play. Each row "
           "is a chunk of oracle text the parser couldn't lower to a typed "
           "effect. These are candidates for new extensions.")
        ap("")
        ap("| Unknown node snippet | Count |")
        ap("|---|---:|")
        for snippet, cnt in report.unknown_nodes.most_common(30):
            s = str(snippet).replace("|", "\\|").replace("\n", " ")
            if len(s) > 100:
                s = s[:97] + "…"
            ap(f"| `{s}` | {cnt} |")
    else:
        ap("_No UnknownEffect fallbacks were logged in this run. Either "
           "coverage is great or the games were too short for rare text "
           "to fire._")
    ap("")

    # --- Winning battlefield cards ----------------------------------------
    ap("## Cards present on the winner's battlefield at game end (top 30)")
    ap("")
    if report.winner_battlefield_appearance:
        ap("| Card | Times on winner's battlefield |")
        ap("|---|---:|")
        for card, cnt in report.winner_battlefield_appearance.most_common(30):
            ap(f"| {card} | {cnt} |")
    else:
        ap("_No completed games with winners recorded._")
    ap("")

    # --- Crashes ----------------------------------------------------------
    if report.crashes:
        ap("## Crashes")
        ap("")
        ap(f"First {len(report.crashes)} crash messages (one per line):")
        ap("")
        for msg in report.crashes:
            ap(f"- `{msg}`")
        ap("")

    # --- Biggest surprises placeholder ------------------------------------
    ap("## Biggest surprises")
    ap("")
    if report.mode == "4p-free-for-all":
        ap("_(Fill in by hand after reviewing the numbers above. Look for "
           "winrate outliers, unexpected elimination patterns, audit "
           "violations concentrated on specific cards.)_")
    else:
        ap("_(Running in 2-player fallback mode — results are not a true "
           "4-player FFA proxy. When the N-player extension lands, re-run "
           "with the same seed to get the real gauntlet.)_")
    ap("")

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(lines))
    print(f"\n→ wrote report: {path}")


# ============================================================================
# CLI
# ============================================================================

def main() -> int:
    ap = argparse.ArgumentParser(
        description="4-player EDH gauntlet runner for mtgsquad")
    ap.add_argument("--games", type=int, default=100,
                    help="number of games to run (default: 100)")
    ap.add_argument("--seed", type=int, default=42,
                    help="master RNG seed")
    ap.add_argument("--decks-dir", type=str,
                    default=str(ROOT / "data" / "decks" / "personal"),
                    help="directory containing *.txt deck files")
    ap.add_argument("--decks", type=str, nargs="*", default=None,
                    help="optional explicit list of deck paths "
                         "(overrides --decks-dir)")
    ap.add_argument("--verbose", action="store_true",
                    help="print per-game winners")
    ap.add_argument("--output", type=str, default=str(REPORT_PATH),
                    help="output markdown path")
    # --- Forensic single-game dump flags (for analyze_single_game.py) ---
    # The gauntlet run normally aggregates stats. When debugging a
    # specific bug, the owner wants a full per-event dump of ONE game
    # so the analyzer can grade each action for anomalies.
    ap.add_argument("--emit-events", type=str, default=None,
                    metavar="PATH",
                    help="JSONL event-stream dump for the target game")
    ap.add_argument("--emit-final-state", type=str, default=None,
                    metavar="PATH",
                    help="JSON final Game state dump for the target game")
    ap.add_argument("--emit-decks", type=str, default=None,
                    metavar="PATH",
                    help="JSON deck metadata dump for the target game")
    ap.add_argument("--emit-game-index", type=int, default=0,
                    help="which game index to dump (default 0)")
    ap.add_argument("--game-id", type=str, default=None,
                    help="override game_id on the dumped game")
    args = ap.parse_args()

    # 1. Load extensions + oracle.
    print("Loading parser extensions + oracle card pool…")
    mtg_parser.load_extensions()
    cards = json.loads(Path(ORACLE_DUMP).read_text())
    cards_by_name = {c["name"].lower(): c for c in cards
                     if mtg_parser.is_real_card(c)}
    print(f"  oracle: {len(cards_by_name)} real cards")

    # 2. Resolve deck paths.
    if args.decks:
        deck_paths = args.decks
    else:
        deck_paths = sorted(glob.glob(
            str(Path(args.decks_dir) / "*.txt")))
    if not deck_paths:
        print(f"ERROR: no deck files found in {args.decks_dir}",
              file=sys.stderr)
        return 1
    print(f"Deck files ({len(deck_paths)}):")
    for p in deck_paths:
        print(f"  - {p}")
    print("")

    # 3. Run the gauntlet.
    print(f"Running gauntlet: {args.games} games, seed {args.seed}")
    forensic = None
    if (args.emit_events or args.emit_final_state or args.emit_decks):
        forensic = {
            "game_index": args.emit_game_index,
            "emit_events": args.emit_events,
            "emit_final_state": args.emit_final_state,
            "emit_decks": args.emit_decks,
            "game_id": args.game_id,
        }
    report = run_gauntlet(
        deck_paths=deck_paths,
        n_games=args.games,
        seed=args.seed,
        cards_by_name=cards_by_name,
        verbose=args.verbose,
        forensic=forensic,
    )

    # 4. Write report.
    write_report(report, Path(args.output), args)

    # 5. Console summary.
    print("")
    print("=" * 68)
    print(f"  Gauntlet complete — mode: {report.mode}")
    print(f"  Games run: {report.games_run}  Crashed: {report.games_crashed}")
    if report.turns:
        print(f"  Avg turns: {sum(report.turns) / len(report.turns):.1f}")
    print(f"  Violations logged: {report.violations_total}")
    print(f"  Parser-gap unique snippets: {len(report.unknown_nodes)}")
    print("=" * 68)

    return 0


if __name__ == "__main__":
    sys.exit(main())
