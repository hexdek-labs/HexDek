#!/usr/bin/env python3
"""Regression tests for rule_auditor.py zone_conservation + legal_targeting.

These tests were added after the 100-game 4p EDH gauntlet surfaced 84
zone_conservation and 39 legal_targeting false positives.

    zone_conservation: the old rule computed baseline from the first state
        snapshot's card counts (99 for commander decks) but never accounted
        for the command-zone card (which the snapshot omits). When a player
        cast their commander from the command zone, it moved to the
        battlefield (which IS in the snapshot), bumping the total to 100
        without any matching zone being accounted for.

        Also: treasure tokens created during play aren't emitted as
        create_token audit events, so the token-allowance counter always
        read 0, even when tokens sat on the final battlefield.

    legal_targeting: the old rule built the seat-battlefield Counter purely
        from `enter_battlefield` / `play_land` events. Cards that hit the
        battlefield via reanimation, tutor-to-play, token creation, or
        direct replacement effects (e.g., Aftermath Analyst) never emit an
        enter_battlefield event, so damage to them was flagged as targeting
        a non-existent creature.

Run:
    python3 scripts/test_zone_conservation.py
"""

from __future__ import annotations

import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from rule_auditor import Violations, audit_game, _is_token_name


def _make_commander_setup(seats: int = 4) -> dict:
    return {
        "seq": 0, "turn": 1, "phase": "beginning", "seat": 0,
        "type": "commander_setup",
        "seats": [
            {"seat": i, "commander_names": [f"Cmd{i}"],
             "starting_life": 40, "command_zone_size": 1}
            for i in range(seats)
        ],
        "_matchup": "test", "_game": 0,
    }


def _make_state(seq: int, seat_data: list[dict], turn: int = 1,
                phase: str = "beginning") -> dict:
    return {
        "seq": seq, "turn": turn, "phase": phase, "seat": 0,
        "type": "state",
        "seats": seat_data,
        "_matchup": "test", "_game": 0,
    }


def _seat_state(idx: int, hand: int = 7, library: int = 92,
                graveyard=None, exile=None, battlefield=None,
                lost: bool = False) -> dict:
    return {
        "idx": idx, "life": 40,
        "hand": hand, "library": library,
        "graveyard": graveyard or [],
        "exile": exile or [],
        "battlefield": battlefield or [],
        "mana_pool": 0, "lost": lost,
    }


def _bf_card(name: str, power: int = 1, toughness: int = 1) -> dict:
    return {
        "name": name, "tapped": False, "summoning_sick": False,
        "power": power, "toughness": toughness, "damage": 0,
    }


def test_token_name_detection():
    assert _is_token_name("treasure artifact token Token (1/1)")
    assert _is_token_name("food artifact token Token (1/1)")
    assert _is_token_name("soldier creature token Token (1/1)")
    # Non-tokens — real card names that might coincidentally share a word
    assert not _is_token_name("Token Admiral")  # hypothetical; no P/T suffix
    assert not _is_token_name("Ragost, Deft Gastronaut")
    assert not _is_token_name("")
    # Explicit token-with-pt — matches
    assert _is_token_name("zombie Token (2/2)")
    print("  test_token_name_detection PASS")


def test_commander_baseline_accounts_for_cz():
    """A 4p EDH snapshot showing commander on battlefield should NOT flag
    zone_conservation — commander+battlefield together = initial CZ + 99.
    """
    events = [
        _make_commander_setup(4),
        {"seq": 1, "turn": 1, "phase": "beginning", "seat": 0,
         "type": "game_start", "n_seats": 4, "commander_format": True,
         "_matchup": "test", "_game": 0},
        _make_state(2, [_seat_state(i) for i in range(4)]),
        # Final snapshot: seat 2 cast their commander + has 2 treasure tokens.
        _make_state(100, [
            _seat_state(0, hand=0, library=85, graveyard=[], battlefield=[]),
            _seat_state(1, hand=0, library=85, graveyard=[], battlefield=[]),
            _seat_state(2, hand=5, library=80, graveyard=["Foo"] * 3,
                        battlefield=[
                            _bf_card("Cmd2"),
                            _bf_card("treasure artifact token Token (1/1)"),
                            _bf_card("treasure artifact token Token (1/1)"),
                            _bf_card("Plains"),
                            _bf_card("Plains"),
                            _bf_card("Sol Ring"),
                            _bf_card("Arcane Signet"),
                            _bf_card("Ichor Wellspring"),
                            _bf_card("Iron Myr"),
                            _bf_card("Reckless Fireweaver"),
                            _bf_card("Academy Manufactor"),
                        ]),
            _seat_state(3, hand=0, library=85, graveyard=[], battlefield=[]),
        ]),
        {"seq": 101, "type": "game_over", "winner": 2,
         "_matchup": "test", "_game": 0},
    ]
    # seat 2: hand=5 + library=80 + gy=3 + bf=11 = 99. Baseline 99 + CZ 1
    # + tokens 2 = 102. Actual total = 99. No violation.
    viol = Violations()
    audit_game(events, viol)
    zc = [v for v in viol.items if v["rule"] == "zone_conservation"]
    assert not zc, f"unexpected zone_conservation: {zc}"
    print("  test_commander_baseline_accounts_for_cz PASS")


def test_token_overcount_is_tolerated():
    """Snapshot total > baseline+cz is OK if the overage is tokens on bf."""
    events = [
        _make_commander_setup(4),
        {"seq": 1, "type": "game_start", "n_seats": 4,
         "commander_format": True, "_matchup": "test", "_game": 0},
        _make_state(2, [_seat_state(i) for i in range(4)]),
        _make_state(100, [
            _seat_state(0, hand=7, library=92, battlefield=[]),
            _seat_state(1, hand=7, library=92, battlefield=[]),
            # seat 2 total = 7 + 85 + 10 (battlefield with Cmd + 5 tokens +
            # 4 real cards) = 102. Baseline 99, CZ 1, tokens 5. Allowed 105.
            _seat_state(2, hand=7, library=85, battlefield=[
                _bf_card("Cmd2"),
                _bf_card("treasure artifact token Token (1/1)"),
                _bf_card("treasure artifact token Token (1/1)"),
                _bf_card("treasure artifact token Token (1/1)"),
                _bf_card("treasure artifact token Token (1/1)"),
                _bf_card("treasure artifact token Token (1/1)"),
                _bf_card("Sol Ring"),
                _bf_card("Arcane Signet"),
                _bf_card("Iron Myr"),
                _bf_card("Plains"),
            ]),
            _seat_state(3, hand=7, library=92, battlefield=[]),
        ]),
        {"seq": 101, "type": "game_over", "winner": 2},
    ]
    viol = Violations()
    audit_game(events, viol)
    zc = [v for v in viol.items if v["rule"] == "zone_conservation"]
    assert not zc, f"unexpected zone_conservation: {zc}"
    print("  test_token_overcount_is_tolerated PASS")


def test_phantom_duplication_still_flagged():
    """A real bug — cards appearing from nowhere (not tokens, not commander)
    — should still trigger zone_conservation.
    """
    events = [
        _make_commander_setup(4),
        {"seq": 1, "type": "game_start", "n_seats": 4,
         "commander_format": True, "_matchup": "test", "_game": 0},
        _make_state(2, [_seat_state(i) for i in range(4)]),
        _make_state(100, [
            _seat_state(0, hand=7, library=92, battlefield=[]),
            _seat_state(1, hand=7, library=92, battlefield=[]),
            # seat 2 total = 7 + 92 + 10 = 109, baseline 99, no tokens, cz 1,
            # allowed 100. 109 > 100 → violation.
            _seat_state(2, hand=7, library=92, battlefield=[
                _bf_card("Sol Ring"),
                _bf_card("Arcane Signet"),
                _bf_card("Iron Myr"),
                _bf_card("Plains"),
                _bf_card("Mountain"),
                _bf_card("Forest"),
                _bf_card("Island"),
                _bf_card("Swamp"),
                _bf_card("Command Tower"),
                _bf_card("Bojuka Bog"),
            ]),
            _seat_state(3, hand=7, library=92, battlefield=[]),
        ]),
        {"seq": 101, "type": "game_over", "winner": 2},
    ]
    viol = Violations()
    audit_game(events, viol)
    zc = [v for v in viol.items if v["rule"] == "zone_conservation"]
    assert zc and zc[0]["seat"] == 2, f"expected zc violation for seat 2, got {zc}"
    print("  test_phantom_duplication_still_flagged PASS")


def test_legal_targeting_uses_snapshot_bf():
    """Sami never emits enter_battlefield (reanimation) but appears in a
    state snapshot before the damage. Damage to her should be legal.
    """
    events = [
        _make_commander_setup(4),
        {"seq": 1, "type": "game_start", "n_seats": 4,
         "commander_format": True, "_matchup": "test", "_game": 0},
        _make_state(2, [_seat_state(i) for i in range(4)]),
        # Sami just reanimated to seat 2's battlefield (no ETB event).
        _make_state(50, [
            _seat_state(0, hand=5, library=80),
            _seat_state(1, hand=5, library=80),
            _seat_state(2, hand=5, library=80, battlefield=[
                _bf_card("Sami, Wildcat Captain", power=4, toughness=4)]),
            _seat_state(3, hand=5, library=80),
        ], turn=10, phase="combat"),
        # Now deal damage to Sami at seq 55.
        {"seq": 55, "turn": 10, "phase": "combat", "seat": 1,
         "type": "damage", "amount": 4,
         "target_kind": "permanent", "target_card": "Sami, Wildcat Captain",
         "target_seat": 2,
         "_matchup": "test", "_game": 0},
        _make_state(100, [_seat_state(i) for i in range(4)]),
        {"seq": 101, "type": "game_over", "winner": 0},
    ]
    viol = Violations()
    audit_game(events, viol)
    lt = [v for v in viol.items if v["rule"] == "legal_targeting"]
    assert not lt, f"unexpected legal_targeting: {lt}"
    print("  test_legal_targeting_uses_snapshot_bf PASS")


def test_legal_targeting_uses_combat_attackers():
    """Even without a snapshot having Sami yet, if she's listed as an
    attacker in the same combat, damage to her is legal.
    """
    events = [
        _make_commander_setup(4),
        {"seq": 1, "type": "game_start", "n_seats": 4,
         "commander_format": True, "_matchup": "test", "_game": 0},
        _make_state(2, [_seat_state(i) for i in range(4)]),
        {"seq": 20, "turn": 5, "phase": "combat", "seat": 2,
         "type": "phase_step_change",
         "to_phase_kind": "combat", "to_step_kind": "declare_attackers",
         "_matchup": "test", "_game": 0},
        {"seq": 21, "turn": 5, "phase": "combat", "seat": 2,
         "type": "attackers", "attackers": ["Sami, Wildcat Captain"],
         "_matchup": "test", "_game": 0},
        {"seq": 22, "turn": 5, "phase": "combat", "seat": 2,
         "type": "blockers",
         "pairs": [{"attacker": "Sami, Wildcat Captain",
                    "blockers": ["Wall of Thorns"]}],
         "_matchup": "test", "_game": 0},
        {"seq": 23, "turn": 5, "phase": "combat", "seat": 0,
         "type": "damage", "amount": 4,
         "target_kind": "permanent", "target_card": "Sami, Wildcat Captain",
         "target_seat": 2,
         "_matchup": "test", "_game": 0},
        _make_state(100, [_seat_state(i) for i in range(4)]),
        {"seq": 101, "type": "game_over", "winner": 0},
    ]
    viol = Violations()
    audit_game(events, viol)
    lt = [v for v in viol.items if v["rule"] == "legal_targeting"]
    assert not lt, f"unexpected legal_targeting: {lt}"
    print("  test_legal_targeting_uses_combat_attackers PASS")


def test_legal_targeting_phantom_still_flagged():
    """A creature that never appears anywhere — no snapshot, no attackers,
    no destroy — should still flag legal_targeting.
    """
    events = [
        _make_commander_setup(4),
        {"seq": 1, "type": "game_start", "n_seats": 4,
         "commander_format": True, "_matchup": "test", "_game": 0},
        _make_state(2, [_seat_state(i) for i in range(4)]),
        {"seq": 50, "turn": 10, "phase": "main1", "seat": 1,
         "type": "damage", "amount": 3,
         "target_kind": "permanent",
         "target_card": "Nonexistent Phantom Creature",
         "target_seat": 2,
         "_matchup": "test", "_game": 0},
        _make_state(100, [_seat_state(i) for i in range(4)]),
        {"seq": 101, "type": "game_over", "winner": 0},
    ]
    viol = Violations()
    audit_game(events, viol)
    lt = [v for v in viol.items if v["rule"] == "legal_targeting"]
    assert lt and lt[0]["target_card"] == "Nonexistent Phantom Creature", \
        f"expected legal_targeting violation, got {lt}"
    print("  test_legal_targeting_phantom_still_flagged PASS")


def test_damage_after_seat_eliminated_is_legal():
    """Simultaneous combat damage: multiple attackers can drop a seat to 0
    mid-resolution. The seat_eliminated event fires, then MORE damage lands
    from other attackers in the same damage step. This is legal per CR
    510.1c (combat damage is assigned simultaneously).
    """
    events = [
        _make_commander_setup(4),
        {"seq": 1, "type": "game_start", "n_seats": 4,
         "commander_format": True, "_matchup": "test", "_game": 0},
        _make_state(2, [_seat_state(i) for i in range(4)]),
        # Damage drops seat 3 to 0.
        {"seq": 10, "turn": 5, "phase": "combat", "seat": 2,
         "type": "damage", "amount": 40, "target_kind": "player",
         "target_seat": 3, "_matchup": "test", "_game": 0},
        {"seq": 11, "turn": 5, "phase": "combat", "seat": 3,
         "type": "life_change", "from": 40, "to": 0,
         "_matchup": "test", "_game": 0},
        {"seq": 12, "turn": 5, "phase": "combat", "seat": 3,
         "type": "seat_eliminated",
         "_matchup": "test", "_game": 0},
        # More damage from another attacker — should NOT be flagged.
        {"seq": 13, "turn": 5, "phase": "combat", "seat": 2,
         "type": "damage", "amount": 5, "target_kind": "player",
         "target_seat": 3, "_matchup": "test", "_game": 0},
        {"seq": 14, "turn": 5, "phase": "combat", "seat": 3,
         "type": "life_change", "from": 0, "to": -5,
         "_matchup": "test", "_game": 0},
        _make_state(100, [_seat_state(i) for i in range(4)]),
        {"seq": 101, "type": "game_over", "winner": 2},
    ]
    viol = Violations()
    audit_game(events, viol)
    # Only damage_arithmetic (perhaps) would appear; legal_targeting for
    # the post-elimination damage should NOT fire.
    lt = [v for v in viol.items
          if v["rule"] == "legal_targeting"
          and v.get("target_seat") == 3 and v["seq"] == 13]
    assert not lt, f"damage at seq 13 should not flag legal_targeting: {lt}"
    print("  test_damage_after_seat_eliminated_is_legal PASS")


def main():
    print("Running rule_auditor regression tests …")
    test_token_name_detection()
    test_commander_baseline_accounts_for_cz()
    test_token_overcount_is_tolerated()
    test_phantom_duplication_still_flagged()
    test_legal_targeting_uses_snapshot_bf()
    test_legal_targeting_uses_combat_attackers()
    test_legal_targeting_phantom_still_flagged()
    test_damage_after_seat_eliminated_is_legal()
    print("\nAll rule_auditor tests PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
