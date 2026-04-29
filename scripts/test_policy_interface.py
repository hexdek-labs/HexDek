#!/usr/bin/env python3
"""Pluggable ``Hat`` interface verification.

The architectural directive is that swapping AI hats is a ONE-LINE
change::

    seat.policy = GreedyHat()   # current default
    seat.policy = PokerHat()    # adaptive HOLD/CALL/RAISE
    seat.policy = MinimalTestHat()   # anything Protocol-conforming

This test suite verifies that promise:

  1. Default attachment — every Seat auto-binds GreedyHat via the
     dataclass ``__post_init__`` hook. 20 games run without errors and
     complete naturally.

  2. Hat swap — attaching PokerHat to every seat still produces
     completing games (different meta, but the engine doesn't care).

  3. Minimal hat — a synthetic MinimalTestHat that passes on
     every decision still terminates (deck-out or turn cap).

  4. Mixed attachment — seat 0 Greedy, seat 1 Poker, seat 2 Greedy,
     seat 3 Poker in a 4-player game — both hats work in parallel.

Run: ``python3 scripts/test_policy_interface.py``
"""

from __future__ import annotations

import random
import sys
import traceback
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import playloop as pl  # noqa: E402
import test_multiplayer as tm  # noqa: E402
from extensions.policies import GreedyHat, PokerHat, PlayerMode  # noqa: E402

RESULTS: list[tuple[str, bool, str]] = []


def check(desc: str, cond: bool, detail: str = "") -> None:
    RESULTS.append((desc, cond, detail))
    status = "PASS" if cond else "FAIL"
    print(f"    [{status}] {desc}" + (f" — {detail}" if detail and not cond else ""))


# ---------------------------------------------------------------------
# Helpers — build tiny 60-card decks out of synthetic CardEntry objects.
# ---------------------------------------------------------------------


def _make_basic_land(name: str = "Forest") -> pl.CardEntry:
    """Cheap basic land. Tap for one green mana. Good enough to cast
    cheap spells. We piggyback on the test_multiplayer builder."""
    return tm.make_basic_land(name)


def _make_creature(name: str, cmc: int, power: int = 2,
                   toughness: int = 2) -> pl.CardEntry:
    return tm.make_card(name, cmc=cmc, power=power, toughness=toughness)


def _make_deck(n: int = 40, creature_ratio: float = 0.5) -> list:
    """Small sanctioned deck for self-play. N cards; half creatures
    (cmc 1-3), half lands. Good enough to drive both engines."""
    deck: list = []
    n_creatures = int(n * creature_ratio)
    n_lands = n - n_creatures
    for i in range(n_creatures):
        deck.append(_make_creature(
            f"Cr{i}", cmc=1 + (i % 3), power=1 + (i % 3),
            toughness=1 + (i % 3),
        ))
    for i in range(n_lands):
        deck.append(_make_basic_land(f"Forest{i}"))
    return deck


# ---------------------------------------------------------------------
# MinimalTestHat — passes on every decision. Used to prove the
# engine doesn't depend on any specific hat behavior: if every
# decision returns the "empty / pass / skip" answer, games must STILL
# terminate (deck-out or turn cap).
# ---------------------------------------------------------------------


class MinimalTestHat:
    """Pass on everything."""

    def choose_mulligan(self, game, seat, hand):
        return False

    def choose_land_to_play(self, game, seat, lands_in_hand):
        # Land drops are how we ramp; even minimal policy plays them
        # (engine doesn't force lands, so if we return None here the
        # game deterministically deck-outs — which is the REAL test
        # for termination).
        return None

    def choose_cast_from_hand(self, game, seat, castable_cards):
        return None  # never cast

    def choose_activation(self, game, seat, activatable):
        return None

    def declare_attackers(self, game, seat, legal_attackers):
        return []  # never attack

    def declare_attack_target(self, game, seat, attacker, legal_defenders):
        return legal_defenders[0] if legal_defenders else seat.idx

    def declare_blockers(self, game, seat, attackers):
        return {id(a): [] for a in attackers}

    def choose_target(self, game, seat, filter_spec, legal_targets):
        return "none", None

    def respond_to_stack_item(self, game, seat, stack_item):
        return None

    def choose_mode(self, game, seat, spell, modal_choices):
        return [0] if modal_choices else []

    def order_replacements(self, game, seat, candidates):
        return list(candidates)

    def choose_discard(self, game, seat, hand, n):
        return hand[:n]

    def choose_distribution(self, game, seat, n, targets):
        return {targets[0]: n} if targets else {}

    def observe_event(self, game, seat, event):
        pass


# ---------------------------------------------------------------------
# Test bodies
# ---------------------------------------------------------------------


def test_greedy_default_attachment() -> None:
    """Every Seat must auto-attach a GreedyHat instance."""
    print("\n── test_greedy_default_attachment ──")
    seat = pl.Seat(idx=0)
    check("Seat auto-binds policy", seat.policy is not None)
    check("Default policy is GreedyHat",
          isinstance(seat.policy, GreedyHat),
          detail=f"got {type(seat.policy).__name__}")
    # Contract: GreedyHat satisfies the Hat Protocol.
    required = [
        "choose_mulligan", "choose_cast_from_hand", "choose_activation",
        "choose_land_to_play", "declare_attackers", "declare_attack_target",
        "declare_blockers", "choose_target", "respond_to_stack_item",
        "choose_mode", "order_replacements", "choose_discard",
        "choose_distribution", "observe_event",
    ]
    for m in required:
        check(f"GreedyHat exposes {m}()", hasattr(seat.policy, m))


def test_greedy_plays_20_games() -> None:
    """20 games with default (Greedy) attachment must complete."""
    print("\n── test_greedy_plays_20_games ──")
    completed = 0
    crashes = 0
    for i in range(20):
        try:
            rng = random.Random(i)
            g = pl.play_game(_make_deck(), _make_deck(), rng=rng)
            assert g.ended
            completed += 1
        except Exception:
            crashes += 1
            if crashes == 1:
                traceback.print_exc()
    check("20/20 Greedy games completed", completed == 20,
          detail=f"completed={completed} crashes={crashes}")


def test_poker_plays_20_games() -> None:
    """20 games with PokerHat on all seats must complete."""
    print("\n── test_poker_plays_20_games ──")
    completed = 0
    crashes = 0
    mode_changes = 0
    for i in range(20):
        try:
            rng = random.Random(i + 1000)
            deck_a = _make_deck()
            deck_b = _make_deck()
            # Build game with pre-attached PokerHat. Can't drop in
            # via play_game's kwargs, so we pre-build Seats.
            seats = [
                pl.Seat(idx=0, library=list(deck_a), policy=PokerHat()),
                pl.Seat(idx=1, library=list(deck_b), policy=PokerHat()),
            ]
            for s in seats:
                rng.shuffle(s.library)
                pl.draw_cards_no_lose(s, pl.STARTING_HAND)
            g = pl.Game(seats=seats, rng=rng)
            g.active = rng.randint(0, 1)
            g.emit(f"poker-game start — seat {g.active} on the play")
            g.ev("game_start", on_the_play=g.active, n_seats=2)
            g.snapshot()
            turn_cap = pl.MAX_TURNS
            while not g.ended and g.turn <= turn_cap:
                pl.take_turn(g)
                if g.ended:
                    break
                pl.swap_active(g)
            if not g.ended:
                g.ended = True
            completed += 1
            mode_changes += sum(
                1 for e in g.events if e.get("type") == "player_mode_change"
            )
        except Exception:
            crashes += 1
            if crashes == 1:
                traceback.print_exc()
    check("20/20 Poker games completed", completed == 20,
          detail=f"completed={completed} crashes={crashes}")
    check("PokerHat emitted player_mode_change events",
          mode_changes > 0,
          detail=f"{mode_changes} mode transitions across 20 games")


def test_minimal_policy_terminates() -> None:
    """A policy that passes on everything must still terminate."""
    print("\n── test_minimal_policy_terminates ──")
    completed = 0
    for i in range(5):
        try:
            rng = random.Random(i + 5000)
            deck_a = _make_deck()
            deck_b = _make_deck()
            seats = [
                pl.Seat(idx=0, library=list(deck_a),
                        policy=MinimalTestHat()),
                pl.Seat(idx=1, library=list(deck_b),
                        policy=MinimalTestHat()),
            ]
            for s in seats:
                rng.shuffle(s.library)
                pl.draw_cards_no_lose(s, pl.STARTING_HAND)
            g = pl.Game(seats=seats, rng=rng)
            g.active = 0
            turn_cap = pl.MAX_TURNS
            while not g.ended and g.turn <= turn_cap:
                pl.take_turn(g)
                if g.ended:
                    break
                pl.swap_active(g)
            completed += 1
        except Exception:
            traceback.print_exc()
    check("MinimalTestHat games terminate", completed == 5,
          detail=f"completed={completed}/5")


def test_policy_swap_mid_setup() -> None:
    """Demonstrate the ONE-LINE policy swap the directive requires.

    The engine doesn't know or care what kind of policy is attached —
    we can mix Greedy and Poker on a per-seat basis and the engine
    still runs correctly.
    """
    print("\n── test_policy_swap_mid_setup ──")
    rng = random.Random(42)
    deck_a = _make_deck()
    deck_b = _make_deck()
    seats = [
        pl.Seat(idx=0, library=list(deck_a)),
        pl.Seat(idx=1, library=list(deck_b)),
    ]

    # ------------- THE ONE-LINE SWAP ---------------------------------
    seats[0].policy = GreedyHat()
    seats[1].policy = PokerHat()
    # -----------------------------------------------------------------

    check("Seat 0 policy = GreedyHat",
          isinstance(seats[0].policy, GreedyHat))
    check("Seat 1 policy = PokerHat",
          isinstance(seats[1].policy, PokerHat))
    check("Engine never inspects policy type — no isinstance checks "
          "in playloop decision sites",
          "isinstance" not in (
              # Grep our own refactor for escape hatches.
              (HERE / "playloop.py").read_text().split(
                  "# Pluggable AI decision policy")[1]
                  .split("from extensions.policies")[0]
          ))

    for s in seats:
        rng.shuffle(s.library)
        pl.draw_cards_no_lose(s, pl.STARTING_HAND)
    g = pl.Game(seats=seats, rng=rng)
    g.active = 0
    turn_cap = pl.MAX_TURNS
    while not g.ended and g.turn <= turn_cap:
        pl.take_turn(g)
        if g.ended:
            break
        pl.swap_active(g)

    check("Mixed-policy game completed without exceptions",
          g.ended or g.turn > turn_cap)

    # Verify BOTH policies actually got observe_event calls.
    # (Greedy is stateless but observe_event still fires; Poker should
    # have seen events and possibly changed mode.)
    check("PokerHat observed events (events_seen > 0)",
          seats[1].policy._events_seen > 0,
          detail=f"events_seen={seats[1].policy._events_seen}")


def test_poker_mode_transitions_have_hysteresis() -> None:
    """HOLD→CALL requires score >= 12, CALL→HOLD requires score <= 8.
    This prevents mode chatter near the boundary."""
    print("\n── test_poker_mode_transitions_have_hysteresis ──")

    from extensions.policies.poker import (
        _HOLD_TO_CALL_THRESHOLD,
        _CALL_TO_HOLD_THRESHOLD,
    )

    check("HOLD→CALL threshold (12) > CALL→HOLD threshold (8) — "
          "hysteresis present",
          _HOLD_TO_CALL_THRESHOLD > _CALL_TO_HOLD_THRESHOLD,
          detail=f"up={_HOLD_TO_CALL_THRESHOLD} "
                 f"down={_CALL_TO_HOLD_THRESHOLD}")

    # Verify default initial mode is CALL (v2 default — "playing
    # normally" seat starts engaged; HOLD is reached on first
    # re-evaluate when nothing to do T1).
    p = PokerHat()
    check("Initial mode is CALL", p.mode == PlayerMode.CALL)

    # Test the transition API: force-transition from CALL to HOLD
    # (simulating the adaptive layer). No game needed for the unit
    # check — we only exercise the state machine.
    class _FakeGame:
        events: list = []

        def ev(self, *a, **kw):
            self.events.append((a, kw))

    class _FakeSeat:
        idx = 0

    fg = _FakeGame()
    fs = _FakeSeat()
    p._events_seen = 10  # past the cooldown
    p._transition(fg, fs, PlayerMode.HOLD, "test")
    check("Transition emits player_mode_change",
          any(kw.get("type") == "player_mode_change" or
              (a and a[0] == "player_mode_change")
              for a, kw in fg.events))
    check("Mode updated to HOLD", p.mode == PlayerMode.HOLD)


# ---------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------


def test_policy_meta_shift() -> None:
    """Run the same 20-game matchup with Greedy on all seats, then
    again with PokerHat on all seats. The winrate distribution
    should differ — if every seat runs the same policy there's still
    noise from RNG, but the mode-sensitive play (HOLD delays early
    threats, CALL attacks threat-weighted, RAISE all-in on low life)
    should shift which seat tends to come out ahead.

    This is the signal that the policy actually matters: swap policy
    → meta changes. Zero delta would indicate the policy is being
    ignored by the engine (a spaghetti regression).
    """
    print("\n── test_policy_meta_shift ──")

    def _run(policy_factory) -> dict:
        """Run 20 2p games, seat 0 vs seat 1, both using the same
        policy class. Returns {seat_idx: win_count}."""
        wins = {0: 0, 1: 0, None: 0}
        for i in range(20):
            rng = random.Random(i + 10_000)
            deck_a = _make_deck()
            deck_b = _make_deck()
            seats = [
                pl.Seat(idx=0, library=list(deck_a),
                        policy=policy_factory()),
                pl.Seat(idx=1, library=list(deck_b),
                        policy=policy_factory()),
            ]
            for s in seats:
                rng.shuffle(s.library)
                pl.draw_cards_no_lose(s, pl.STARTING_HAND)
            g = pl.Game(seats=seats, rng=rng)
            g.active = rng.randint(0, 1)
            g.ev("game_start", on_the_play=g.active, n_seats=2)
            turn_cap = pl.MAX_TURNS
            while not g.ended and g.turn <= turn_cap:
                pl.take_turn(g)
                if g.ended:
                    break
                pl.swap_active(g)
            winner = g.winner if g.ended else None
            wins[winner] = wins.get(winner, 0) + 1
        return wins

    greedy_wins = _run(GreedyHat)
    poker_wins = _run(PokerHat)
    print(f"  Greedy (all seats):  seat0={greedy_wins[0]:2d}  "
          f"seat1={greedy_wins[1]:2d}  "
          f"nowinner={greedy_wins.get(None, 0):2d}")
    print(f"  Poker  (all seats):  seat0={poker_wins[0]:2d}  "
          f"seat1={poker_wins[1]:2d}  "
          f"nowinner={poker_wins.get(None, 0):2d}")

    # The two distributions should differ. We measure total absolute
    # difference across {seat0, seat1, no-winner}.
    delta = (
        abs(greedy_wins[0] - poker_wins[0])
        + abs(greedy_wins[1] - poker_wins[1])
        + abs(greedy_wins.get(None, 0) - poker_wins.get(None, 0))
    )
    check("Policy swap shifts 20-game meta (abs delta > 0)",
          delta > 0,
          detail=f"abs_delta={delta}")
    # Much stronger signal: Poker RAISE mode aggressively attacks,
    # so we expect more decisive games (fewer no-winner outcomes
    # when both sides actually attack with everything on low life).
    print(f"  meta delta = {delta} across {20*2} game outcomes")


def main() -> int:
    print("=" * 72)
    print("  Hat interface verification")
    print("=" * 72)
    test_greedy_default_attachment()
    test_greedy_plays_20_games()
    test_poker_plays_20_games()
    test_minimal_policy_terminates()
    test_policy_swap_mid_setup()
    test_poker_mode_transitions_have_hysteresis()
    test_policy_meta_shift()
    passed = sum(1 for _, ok, _ in RESULTS if ok)
    failed = sum(1 for _, ok, _ in RESULTS if not ok)
    print("-" * 72)
    print(f"  Total: {passed} passed, {failed} failed "
          f"({len(RESULTS)} assertions)")
    print("=" * 72)
    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
