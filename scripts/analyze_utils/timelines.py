"""Extract per-seat per-turn resource timelines from the event stream.

Downstream detectors need to ask questions like "was Sol Ring on the
battlefield for seat 0 during turn 3?" or "how much mana did seat 2
generate in turn 5?" This module walks the events once and builds
data structures indexed by (seat, turn) so detectors can query
cheaply.

All timeline extraction is additive; we don't simulate the engine, we
just remember what it emitted.
"""

from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass, field


# Events that put a permanent onto a seat's battlefield. The canonical
# path is enter_battlefield; token_created_attacking and
# create_token also signal appearance. We treat all three as "permanent
# became present on this seat's battlefield" for the purpose of
# detector queries. commander_cast_from_command_zone doesn't carry
# the battlefield-landing signal directly (the enter_battlefield
# trigger fires separately), so we don't need to handle it here.
_BATTLEFIELD_ENTER_EVENTS = (
    "enter_battlefield",
    "token_created_attacking",
    "create_token",
    "manifest_done",
    "cloak_done",
    "manifest_dread_manifest",
)
# Events that remove from battlefield. We treat commander_zone_return,
# destroy, sacrifice, wrath (group) as leave signals. Exile effects
# would ideally appear as a separate event but don't currently, so
# we may miss some leave transitions. That's OK for MVP — the detectors
# can always fall back to final_state.
_BATTLEFIELD_LEAVE_EVENTS = (
    "destroy",
    "sacrifice",
    "commander_zone_return",
    "die_replaced",
)


@dataclass
class TurnSlice:
    """One seat's activity during one turn."""
    seat: int
    turn: int
    # Mana generated this turn, keyed (source_card → total_pips).
    mana_by_source: dict = field(default_factory=lambda: defaultdict(int))
    # Spells cast this turn by this seat. Each entry: {"card": name,
    # "event_seq": int, "cmc": int}.
    casts: list = field(default_factory=list)
    # pay_mana events this seat emitted (used by "unless they pay 1"
    # detection).
    pay_mana_events: list = field(default_factory=list)
    # Draw events for this seat this turn.
    draws: list = field(default_factory=list)
    # Cards milled this turn (from any source).
    milled: list = field(default_factory=list)
    # Damage dealt TO this seat this turn.
    damage_taken: list = field(default_factory=list)
    # Life changes for this seat this turn.
    life_changes: list = field(default_factory=list)
    # Cast-trigger-observer firings for this seat this turn.
    cast_observer_fires: list = field(default_factory=list)
    # Reactive trigger fires (Rhystic Study, Esper Sentinel, etc.)
    # that we detected by proxy — the add/draw that followed.
    # Used for cross-reference in detectors.
    rhystic_draws: int = 0
    # Cast events WHERE seat is active but cast is by another seat
    # (opponent casts — used for reactive trigger analysis).
    opponent_casts_while_i_have_trigger: list = field(default_factory=list)


@dataclass
class Timeline:
    """Whole-game timeline.

    Top-level index is (seat_idx, turn_num). Queries:
        tl.slice_of(seat, turn) → TurnSlice
        tl.battlefield_at(seat, turn) → set[card_name]
        tl.all_turns() → sorted list of turn numbers
    """
    num_seats: int
    total_turns: int = 0
    # seat → turn → TurnSlice
    slices: dict = field(default_factory=dict)
    # seat → list[(turn_enter, turn_leave_or_None, card_name)]
    # "leave" is None while still on battlefield at end of game.
    battlefield_history: dict = field(default_factory=dict)
    # Global: ordered list of (turn, seat, "phase/step") transitions.
    phase_transitions: list = field(default_factory=list)
    # seat → list of turns in which that seat was active.
    active_turns_by_seat: dict = field(default_factory=dict)
    # game_start event seq (for priority-window detection).
    game_start_seq: int = -1
    # seat → turn this seat lost (or None).
    elimination_turn: dict = field(default_factory=dict)

    def slice_of(self, seat: int, turn: int) -> TurnSlice:
        t = self.slices.setdefault(seat, {})
        return t.setdefault(turn, TurnSlice(seat=seat, turn=turn))

    def battlefield_at(self, seat: int, turn: int) -> set:
        """Names of cards present on `seat`'s battlefield during `turn`.

        A card counts as "present during turn T" if it entered at turn
        Te ≤ T and left at turn Tl > T (or never left). This is coarse
        (doesn't care about within-turn timing), but detectors mostly
        ask "was Sol Ring there at all during T?" — which is what this
        returns.
        """
        out = set()
        for te, tl, name in self.battlefield_history.get(seat, ()):
            if te <= turn and (tl is None or tl > turn):
                out.add(name)
        return out

    def all_turns(self) -> list:
        return list(range(1, self.total_turns + 1))


def build_timeline(events: list, num_seats: int) -> Timeline:
    """Walk the events once; produce a Timeline."""
    tl = Timeline(num_seats=num_seats)
    for i in range(num_seats):
        tl.battlefield_history[i] = []
        tl.active_turns_by_seat[i] = set()
        tl.elimination_turn[i] = None

    # Track open battlefield entries so we can stamp a leave turn.
    # key = (seat, card_name) → list of open entry turns.
    open_entries: dict = defaultdict(list)

    for idx, e in enumerate(events):
        turn = int(e.get("turn", 0))
        seat = e.get("seat")
        type_ = e.get("type", "")
        if turn > tl.total_turns:
            tl.total_turns = turn

        # Active-seat tracking — `seat` in the event is the active seat
        # at emission time. Doesn't mean THIS event was caused by that
        # seat (opponents can cast during your phase), but useful for
        # "who took turn N" queries.
        if isinstance(seat, int):
            tl.active_turns_by_seat[seat].add(turn)

        # ---- Battlefield enter/leave tracking -----------------------
        if type_ in _BATTLEFIELD_ENTER_EVENTS:
            # `seat` in these events is (usually) the controller seat.
            # enter_battlefield carries explicit `seat=source_seat`.
            owner = e.get("seat")
            name = e.get("card") or e.get("name") or ""
            if isinstance(owner, int) and name:
                tl.battlefield_history[owner].append([turn, None, name])
                open_entries[(owner, name)].append(
                    len(tl.battlefield_history[owner]) - 1)
        elif type_ == "destroy" or type_ == "sacrifice":
            # target_card or card identifies what left.
            name = e.get("target_card") or e.get("card") or ""
            owner = e.get("target_seat")
            if owner is None:
                owner = e.get("seat")
            if isinstance(owner, int) and name:
                queue = open_entries.get((owner, name))
                if queue:
                    entry_idx = queue.pop(0)
                    tl.battlefield_history[owner][entry_idx][1] = turn
        elif type_ == "commander_zone_return":
            name = e.get("commander") or e.get("card") or ""
            owner = e.get("owner_seat")
            if owner is None:
                owner = e.get("seat")
            if isinstance(owner, int) and name:
                queue = open_entries.get((owner, name))
                if queue:
                    entry_idx = queue.pop(0)
                    tl.battlefield_history[owner][entry_idx][1] = turn
        elif type_ == "wrath":
            # Group clear — close all open entries for this seat's
            # creatures. We don't know which specific cards the wrath
            # destroyed (the event only has a count), so we close every
            # open creature entry for every seat.
            # This is lossy; detectors that need precise leave timing
            # on wrathed boards should add their own heuristic.
            pass

        # ---- Per-turn slice fill-in ---------------------------------
        if type_ == "add_mana":
            src_seat = e.get("seat")
            if isinstance(src_seat, int):
                sl = tl.slice_of(src_seat, turn)
                src_card = e.get("source_card") or e.get("source") or ""
                amt = int(e.get("amount", 0))
                sl.mana_by_source[src_card] += amt

        elif type_ == "cast":
            caster = e.get("seat")
            if isinstance(caster, int):
                sl = tl.slice_of(caster, turn)
                sl.casts.append({
                    "card": e.get("card", ""),
                    "cmc": int(e.get("cmc", 0)),
                    "seq": int(e.get("seq", idx)),
                    "in_response_to": e.get("in_response_to"),
                })

        elif type_ == "pay_mana":
            payer = e.get("seat")
            if isinstance(payer, int):
                sl = tl.slice_of(payer, turn)
                sl.pay_mana_events.append({
                    "amount": int(e.get("amount", 0)),
                    "reason": e.get("reason", ""),
                    "card": e.get("card", ""),
                    "seq": int(e.get("seq", idx)),
                })

        elif type_ == "draw":
            drawer = e.get("seat")
            if isinstance(drawer, int):
                sl = tl.slice_of(drawer, turn)
                sl.draws.append({
                    "count": int(e.get("count", 1)),
                    "hand_size": int(e.get("hand_size", 0)),
                    "seq": int(e.get("seq", idx)),
                })

        elif type_ == "mill":
            who = e.get("seat")
            if isinstance(who, int):
                sl = tl.slice_of(who, turn)
                sl.milled.extend(e.get("cards", []))

        elif type_ == "damage":
            tk = e.get("target_kind")
            if tk == "player":
                tgt = e.get("target_seat")
                if isinstance(tgt, int):
                    sl = tl.slice_of(tgt, turn)
                    sl.damage_taken.append({
                        "amount": int(e.get("amount", 0)),
                        "source_card": e.get("source_card", ""),
                        "source_seat": e.get("source_seat"),
                        "seq": int(e.get("seq", idx)),
                    })

        elif type_ == "life_change":
            who = e.get("seat")
            if isinstance(who, int):
                sl = tl.slice_of(who, turn)
                sl.life_changes.append({
                    "from": e.get("from", 0),
                    "to": e.get("to", 0),
                    "seq": int(e.get("seq", idx)),
                })

        elif type_ == "cast_trigger_observer":
            who = e.get("seat")
            if isinstance(who, int):
                sl = tl.slice_of(who, turn)
                sl.cast_observer_fires.append({
                    "source": e.get("source", ""),
                    "cast": e.get("cast", ""),
                    "effect": e.get("effect", ""),
                    "seq": int(e.get("seq", idx)),
                })

        elif type_ == "seat_eliminated":
            who = e.get("seat")
            if isinstance(who, int):
                if tl.elimination_turn.get(who) is None:
                    tl.elimination_turn[who] = turn

        elif type_ == "phase_step_change":
            tl.phase_transitions.append({
                "turn": turn,
                "seat": seat,
                "from_phase": e.get("from_phase_kind", ""),
                "from_step": e.get("from_step_kind", ""),
                "to_phase": e.get("to_phase_kind", ""),
                "to_step": e.get("to_step_kind", ""),
                "seq": int(e.get("seq", idx)),
            })

        elif type_ == "game_start":
            tl.game_start_seq = int(e.get("seq", idx))

    return tl
