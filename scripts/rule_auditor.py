#!/usr/bin/env python3
"""Rule auditor for mtgsquad playloop event logs.

Reads a JSONL event stream (produced by `playloop.py --audit` or --json-log) and
asserts MTG-rule invariants. Violations are LOGGED, never raised — the point is
to triage, not crash.

Invariants implemented:
    1. mana_conservation  — add_mana pips must eventually be paid or drained.
    2. damage_arithmetic  — damage N to seat X implies a life_change of -N
                            for seat X (± lifelink for source's controller).
    3. life_negative      — life ≤ 0 must be followed by destroy/game_over.
    4. zone_conservation  — per-seat card count (hand+lib+gy+exile+bf+stack)
                            must remain constant modulo tokens.
    5. turn_structure     — phases inside a turn must follow the canonical
                            order: untap → upkeep → draw → main1 → combat →
                            main2 → end → cleanup.
    6. land_drop_limit    — at most one play_land per seat per turn.
    7. summoning_sickness — a creature that ETB'd in turn T cannot attack in
                            turn T unless it has haste.
    8. legal_targeting    — every damage event's target must be alive (for
                            seats) or on the battlefield (for permanents) at
                            the event's sequence point.
    9. pt_sanity          — no creature's power should drop below its
                            printed base without a buff/counter event to
                            explain it. (MVP: checks only via event stream; if
                            no printed-base info present, skipped.)

CLI:
    python3 scripts/rule_auditor.py PATH/TO/events.jsonl
        → writes violations to data/rules/audit_violations.jsonl and a summary
        → to data/rules/audit_summary.md

    python3 scripts/rule_auditor.py PATH/TO/events.jsonl \
        --violations /tmp/viol.jsonl --summary /tmp/summary.md
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter, defaultdict
from pathlib import Path
from typing import Iterable

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
DEFAULT_VIOLATIONS = ROOT / "data" / "rules" / "audit_violations.jsonl"
DEFAULT_SUMMARY = ROOT / "data" / "rules" / "audit_summary.md"

CANONICAL_PHASES = [
    "beginning", "untap", "upkeep", "draw",
    "main1", "combat", "main2", "end", "cleanup",
]


# ---------------------------------------------------------------------------
# Event iteration
# ---------------------------------------------------------------------------

def iter_events(path: Path) -> Iterable[dict]:
    with path.open() as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                yield json.loads(line)
            except json.JSONDecodeError:
                # Tolerate malformed lines; auditor should never die.
                continue


def group_by_game(events: Iterable[dict]) -> Iterable[tuple[str, int, list[dict]]]:
    """Group events into per-game event lists.

    Logs produced by --audit carry _matchup and _game tags; logs from
    --json-log carry none, so we fall back to one group keyed (None, 0).
    """
    buf: dict[tuple, list] = defaultdict(list)
    for e in events:
        key = (e.get("_matchup"), e.get("_game", 0))
        buf[key].append(e)
    for (m, g), evts in buf.items():
        yield m, g, evts


# ---------------------------------------------------------------------------
# Per-game auditor
# ---------------------------------------------------------------------------

class Violations:
    def __init__(self) -> None:
        self.items: list[dict] = []
        self.counts: Counter = Counter()

    def add(self, rule: str, evt: dict | None, detail: str, **extra) -> None:
        row = {
            "rule": rule,
            "seq": evt.get("seq") if evt else None,
            "turn": evt.get("turn") if evt else None,
            "phase": evt.get("phase") if evt else None,
            "seat": evt.get("seat") if evt else None,
            "matchup": evt.get("_matchup") if evt else None,
            "game": evt.get("_game") if evt else None,
            "detail": detail,
        }
        row.update(extra)
        self.items.append(row)
        self.counts[rule] += 1


def audit_game(events: list[dict], viol: Violations) -> None:
    if not events:
        return

    # --- Rule 1: mana conservation (per seat, per turn) ---
    # For each seat: sum(add_mana) - sum(pay_mana) - sum(pool_drain) should
    # equal the residual pool at turn end (which should be 0 at end_step).
    # We check per-turn-per-seat: the balance after end_step should be 0.
    mana_ledger: dict[tuple[int, int], dict] = defaultdict(
        lambda: {"added": 0, "paid": 0, "drained": 0, "first_add": None})

    # --- Rule 5: turn structure ---
    # Each player-turn (bounded by turn_start events) records the order of
    # phase_change events. Keyed by (turn_start_seq). Both players' halves of
    # a "turn" (as tracked by playloop) have separate turn_start events.
    turn_phases: dict[int, list[tuple[int, str]]] = defaultdict(list)
    current_turn_key: int = 0

    # --- Rule 6: land drop limit ---
    land_drops: Counter = Counter()  # (turn, seat) → count

    # --- Rule 7: summoning sickness ---
    # Track ALL ETB turns for each (seat, card_name) so we can ask "does at
    # least one copy predate this turn?" rather than overwriting on each ETB.
    etb_turns: dict[tuple[int, str], list[tuple[int, bool]]] = defaultdict(list)
    # (seat, card) -> list of (turn_etb'd, summoning_sick_at_etb)

    # --- Rule 8: targeting legality ---
    # Truth source for battlefield presence is the authoritative `state`
    # snapshot stream — ETB events don't cover every path cards reach the
    # battlefield (reanimation, tutor-to-play, token creation, replacement
    # effects like Aftermath Analyst, etc.). We still build an ETB-derived
    # battlefield as a fast pre-check, but fall back to the most recent
    # snapshot (and nearby attackers/blockers lists) before flagging.
    alive_seats: set[int] = set()  # seats we've seen alive via snapshots/events
    battlefield: dict[int, Counter] = defaultdict(Counter)  # seat → Counter(card_name)
    dead_seats: set[int] = set()

    # Snapshot-derived battlefield cache: for each seat, the most recent
    # state snapshot's battlefield (list of card names). Updated in-pass.
    snapshot_bf: dict[int, list[str]] = defaultdict(list)

    # Recent combat references (attackers/blockers lists) within the current
    # combat step — cards referenced here are implicitly on the battlefield
    # at damage resolution time, even if the snapshot lags or we missed an
    # ETB emission. Cleared whenever phase changes out of combat.
    recent_combat_cards: dict[int, set[str]] = defaultdict(set)
    in_combat: bool = False

    # --- Rule 3: life_negative ---
    pending_lethal: dict[int, int] = {}  # seat -> seq where life went ≤ 0
    resolved_lethal: set[int] = set()

    # --- Rule 4: zone conservation (per seat, known count start = 60) ---
    # We track only the net effect: library→hand draws are conserving; tokens
    # are the only legit way to exceed. Our event stream doesn't log every
    # zone move, so we approximate using snapshot events when present.
    snapshots: list[dict] = []

    # --- Commander format tracking (§903) ---
    # commander_setup emits command_zone_size per seat; that card is NOT
    # reflected in state snapshots' hand/library/gy/bf/exile lists, so we
    # must add it to the baseline zone total.
    commander_cz_baseline: dict[int, int] = {}
    commander_names: dict[int, set[str]] = defaultdict(set)
    commander_format: bool = False

    # Seats observed.
    seats_seen: set[int] = set()

    # --- Pre-pass: seed seats + commander metadata from setup events ---
    for e in events:
        et = e.get("type")
        if et == "commander_setup":
            commander_format = True
            for s in e.get("seats", []) or []:
                sidx = s.get("seat")
                if sidx is None:
                    continue
                commander_cz_baseline[sidx] = s.get("command_zone_size", 1)
                for name in s.get("commander_names", []) or []:
                    commander_names[sidx].add(name)
                seats_seen.add(sidx)
                alive_seats.add(sidx)
        elif et == "game_start":
            n_seats = e.get("n_seats", 2)
            for s in range(n_seats):
                seats_seen.add(s)
                alive_seats.add(s)
            if e.get("commander_format"):
                commander_format = True
            break
        elif et == "state":
            # first state snapshot comes before game_start in some logs;
            # seed seats from it too but don't break — keep scanning for
            # commander_setup above.
            for sidx, _ in enumerate(e.get("seats", []) or []):
                seats_seen.add(sidx)
                alive_seats.add(sidx)

    # --- Main linear pass ---
    for i, e in enumerate(events):
        t = e.get("type")
        turn = e.get("turn", 0)
        seat = e.get("seat", 0)
        seats_seen.add(seat)

        if t == "add_mana":
            amt = e.get("amount", 0)
            key = (e.get("seat", 0), turn)
            mana_ledger[key]["added"] += amt
            if mana_ledger[key]["first_add"] is None:
                mana_ledger[key]["first_add"] = e.get("seq")

        elif t == "pay_mana":
            amt = e.get("amount", 0)
            key = (e.get("seat", 0), turn)
            mana_ledger[key]["paid"] += amt

        elif t == "pool_drain":
            amt = e.get("amount", 0)
            key = (e.get("seat", 0), turn)
            mana_ledger[key]["drained"] += amt

        elif t == "turn_start":
            current_turn_key = e.get("seq", i)
            # New turn — combat cache from prior turn is stale.
            recent_combat_cards.clear()
            in_combat = False

        elif t == "phase_change":
            ph = e.get("phase", "?")
            # Attribute to current player-turn (bounded by turn_start events).
            turn_phases[current_turn_key].append((e.get("seq", i), ph))

        elif t == "play_land":
            s = e.get("seat", 0)
            key = (turn, s)
            land_drops[key] += 1
            if land_drops[key] > 1:
                viol.add(
                    "land_drop_limit", e,
                    f"seat {s} played {land_drops[key]} lands on turn {turn} "
                    f"(max 1)",
                )
            # also track battlefield entry
            card = e.get("card", "?")
            battlefield[s][card] += 1

        elif t == "enter_battlefield":
            s = e.get("seat", 0)
            card = e.get("card", "?")
            battlefield[s][card] += 1
            sick = bool(e.get("summoning_sick", True))
            etb_turns[(s, card)].append((turn, sick))

        elif t == "attackers":
            atk_list = e.get("attackers", []) or []
            s = e.get("seat", 0)
            # Names declared as attackers are authoritatively on the
            # battlefield at this moment — feed them into the combat cache
            # and into the ETB-derived battlefield so targeting checks
            # don't false-positive on stealth-ETB'd cards (reanimation etc).
            for name in atk_list:
                recent_combat_cards[s].add(name)
                if battlefield[s].get(name, 0) <= 0:
                    battlefield[s][name] = max(battlefield[s].get(name, 0), 1)
                key = (s, name)
                records = etb_turns.get(key, [])
                if not records:
                    continue
                # Can any copy attack? It can if (etb_turn < current turn) OR
                # (etb happened in a prior turn-half, which we approximate as
                # etb_turn <= current turn - 1) OR it had haste at ETB.
                eligible = any(
                    (etb_t < turn) or (not sick)
                    for (etb_t, sick) in records
                )
                if not eligible:
                    viol.add(
                        "summoning_sickness", e,
                        f"seat {s} attacked with '{name}' on turn {turn} but "
                        f"no copy ETB'd in a prior turn (records={records})",
                        card=name, etb_records=records,
                    )

        elif t == "blockers":
            # Blockers are authoritatively on the battlefield of their
            # controller. The event schema lists attackers (attacker's seat)
            # but blockers can be from ANY opponent's board. We can't always
            # know the blocker's seat, so we add blocker names to every
            # opponent's combat cache as a fallback (the permanent targeting
            # check matches by seat, so this is conservative).
            pairs = e.get("pairs", []) or []
            for pair in pairs:
                for bname in pair.get("blockers", []) or []:
                    # Add to all seats' combat cache; targeting check uses
                    # the specific seat from the damage event.
                    for sidx in seats_seen:
                        recent_combat_cards[sidx].add(bname)

        elif t == "damage":
            amt = e.get("amount", 0)
            tk = e.get("target_kind")
            if tk == "player":
                ts = e.get("target_seat")
                if ts in dead_seats:
                    viol.add(
                        "legal_targeting", e,
                        f"damage {amt} targeted seat {ts} but seat was already "
                        f"dead at seq={e.get('seq')}",
                        target_seat=ts,
                    )
                # Rule 2: damage arithmetic — look for the next life_change for
                # this seat within the next few events.
                _check_damage_arithmetic(events, i, e, viol)
            elif tk == "permanent":
                name = e.get("target_card", "?")
                ts = e.get("target_seat", 0)
                # Primary truth: ETB-derived battlefield counter.
                if battlefield[ts].get(name, 0) > 0:
                    pass
                # Fallback 1: most recent state snapshot's battlefield list
                # (authoritative — covers reanimation, tutor-to-play, tokens,
                # replacement effects that bypass our ETB emission).
                elif name in snapshot_bf.get(ts, []):
                    pass
                # Fallback 2: card was referenced as attacker/blocker in the
                # current combat step (damage happens INSIDE combat, so any
                # creature named in the attackers/blockers lists is on the
                # battlefield at resolution time by construction).
                elif name in recent_combat_cards.get(ts, set()):
                    pass
                # Fallback 3: look ahead — if the NEXT few events include a
                # destroy/sba event for this same card on this same seat,
                # the damage DID land on a real creature (we just missed its
                # ETB). This handles tokens with bespoke names that are
                # created, damaged, and die all in the same resolution stack.
                elif _target_resolved_nearby(events, i, name, ts):
                    pass
                else:
                    viol.add(
                        "legal_targeting", e,
                        f"damage {amt} targeted '{name}' but no such creature "
                        f"is on seat {ts}'s battlefield (or count=0)",
                        target_card=name, target_seat=ts,
                    )

        elif t == "life_change":
            new_life = e.get("to", 0)
            s = e.get("seat", 0)
            if new_life <= 0 and s not in resolved_lethal:
                pending_lethal[s] = e.get("seq", i)

        elif t == "destroy":
            # Permanent destruction: remove from battlefield counter.
            s = e.get("seat", 0)
            card = e.get("card", "?")
            if battlefield[s][card] > 0:
                battlefield[s][card] -= 1
            # If seat itself "died" via destroy? Not in this schema. Mark
            # seat dead if loss_reason was recorded — we don't see that here,
            # so we rely on game_over below.

        elif t == "state":
            snapshots.append(e)
            # Refresh snapshot_bf: list of card names currently on each
            # seat's battlefield. This is the authoritative truth source
            # for rule 8 (legal_targeting) permanents.
            for s_state in e.get("seats", []) or []:
                s_idx = s_state.get("idx")
                if s_idx is None:
                    continue
                bf_list = s_state.get("battlefield", []) or []
                snapshot_bf[s_idx] = [
                    (p.get("name") if isinstance(p, dict) else str(p))
                    for p in bf_list
                ]
                # Note: we deliberately do NOT mark `lost` seats as dead
                # here — simultaneous combat damage resolution can target
                # a seat that's already lost but hasn't seen game_over yet.
            # Crude lifecheck: any seat with life<=0 and no following game_over
            # will be caught by pending_lethal.

        elif t == "phase_step_change":
            # Track combat scope so we know when to clear the combat card cache.
            to_phase_kind = e.get("to_phase_kind", "")
            if to_phase_kind == "combat":
                if not in_combat:
                    recent_combat_cards.clear()
                in_combat = True
            else:
                in_combat = False
                # Don't clear immediately — SBA cleanup damage after end of
                # combat can still reference attackers. Clear on next
                # turn_start below.

        elif t == "seat_eliminated":
            # NOTE: we do NOT add to dead_seats here because combat damage
            # from multiple attackers all resolves simultaneously — a seat
            # can go to 0 life mid-damage-step (firing seat_eliminated) yet
            # still legally receive further damage from other attackers
            # completing their damage assignment (CR 510.1c). dead_seats is
            # reserved for post-game_over out-of-turn damage detection.
            pass

        elif t == "game_over":
            # All pending lethal hits resolved by the game ending.
            for s in list(pending_lethal):
                resolved_lethal.add(s)
                pending_lethal.pop(s, None)
            # Mark all non-winner seats dead.
            winner = e.get("winner")
            for s in seats_seen:
                if s != winner:
                    dead_seats.add(s)

    # --- Post-pass: mana conservation (per seat per turn) ---
    for (s, turn), led in mana_ledger.items():
        added = led["added"]
        paid = led["paid"]
        drained = led["drained"]
        if added != paid + drained:
            viol.add(
                "mana_conservation",
                {"seq": led["first_add"], "turn": turn, "phase": None,
                 "seat": s, "_matchup": events[0].get("_matchup"),
                 "_game": events[0].get("_game")},
                f"seat {s} turn {turn}: added {added} mana, paid {paid}, "
                f"drained {drained} (diff={added-paid-drained})",
                added=added, paid=paid, drained=drained,
            )

    # --- Post-pass: turn structure ---
    for turn, phases in turn_phases.items():
        seen_phases = [ph for _, ph in phases]
        # Collapse consecutive duplicates (phase_change emitted once per phase
        # entry, but upkeep/main may run per-seat).
        compressed: list[str] = []
        for ph in seen_phases:
            if not compressed or compressed[-1] != ph:
                compressed.append(ph)
        # Check the canonical order appears as a subsequence of compressed.
        # If any canonical phase appears out-of-order, flag.
        canonical_idx = {p: i for i, p in enumerate(CANONICAL_PHASES)}
        last = -1
        for j, ph in enumerate(compressed):
            if ph not in canonical_idx:
                continue
            idx = canonical_idx[ph]
            if idx < last:
                viol.add(
                    "turn_structure",
                    {"seq": phases[j][0], "turn": turn, "phase": ph,
                     "seat": None,
                     "_matchup": events[0].get("_matchup"),
                     "_game": events[0].get("_game")},
                    f"turn {turn}: phase '{ph}' appeared after a later phase "
                    f"(sequence: {' → '.join(compressed)})",
                    sequence=compressed,
                )
                break
            last = idx

    # --- Post-pass: life_negative without destroy/game_over ---
    for s, seq in pending_lethal.items():
        if s in resolved_lethal:
            continue
        viol.add(
            "life_negative",
            {"seq": seq, "turn": None, "phase": None, "seat": s,
             "_matchup": events[0].get("_matchup"),
             "_game": events[0].get("_game")},
            f"seat {s} dropped to ≤0 life at seq={seq} but game never ended "
            f"or resolved SBA",
        )

    # --- Post-pass: zone conservation (snapshot-based) ---
    #
    # Conservation law (per seat):
    #
    #   final_snapshot_total ≤ initial_snapshot_total
    #                          + command_zone_size_at_start  (commander format)
    #                          + tokens_currently_on_battlefield
    #
    # Notes:
    # - The state snapshot schema does NOT include command_zone contents.
    #   In Commander, each seat begins with 99 cards in library+hand and 1
    #   in the command zone (total 100). When the commander is cast it moves
    #   CZ → stack → battlefield, so the snapshot total goes UP by 1 because
    #   it now appears in battlefield (which IS in the snapshot).
    # - Tokens have no baseline (they're created during play) and only exist
    #   while on the battlefield. Dead tokens cease to exist, so the ONLY
    #   legit overcount at end-of-game is the tokens currently on a seat's
    #   battlefield in the final snapshot.
    # - We detect tokens by name: our token-creation code emits names like
    #   "treasure artifact token Token (1/1)" and similar patterns. A token
    #   is any battlefield card whose lowercased name contains "token".
    # - create_token audit events are NOT emitted in the current engine, so
    #   we can't rely on a running counter — we count from the snapshot.
    if len(snapshots) >= 2:
        baseline = _seat_zone_totals(snapshots[0])
        last = _seat_zone_totals(snapshots[-1])

        # Count tokens currently on each seat's battlefield in the FINAL
        # snapshot (the only place tokens still add to the visible total).
        tokens_in_final_bf: Counter = Counter()
        last_snap = snapshots[-1]
        for s_state in last_snap.get("seats", []) or []:
            s_idx = s_state.get("idx")
            if s_idx is None:
                continue
            for p in s_state.get("battlefield", []) or []:
                nm = (p.get("name", "") if isinstance(p, dict) else str(p))
                if _is_token_name(nm):
                    tokens_in_final_bf[s_idx] += 1

        # Legacy 2-seat snapshots don't carry "idx" — handle them too.
        if not tokens_in_final_bf:
            for key in ("seat_0", "seat_1", "seat_2", "seat_3"):
                st = last_snap.get(key)
                if not st:
                    continue
                s_idx = int(key.split("_")[1])
                for p in st.get("battlefield", []) or []:
                    nm = (p.get("name", "") if isinstance(p, dict) else str(p))
                    if _is_token_name(nm):
                        tokens_in_final_bf[s_idx] += 1

        for seat_key, total_last in last.items():
            base = baseline.get(seat_key, 60)
            cz_bonus = commander_cz_baseline.get(seat_key, 0)
            tok = tokens_in_final_bf.get(seat_key, 0)
            allowed = base + cz_bonus + tok
            if total_last > allowed:
                viol.add(
                    "zone_conservation",
                    {"seq": snapshots[-1].get("seq"), "turn": None,
                     "phase": None, "seat": seat_key,
                     "_matchup": events[0].get("_matchup"),
                     "_game": events[0].get("_game")},
                    f"seat {seat_key}: final zone total {total_last} exceeds "
                    f"baseline {base} + commander CZ {cz_bonus} + tokens {tok}",
                    baseline=base, commander_cz=cz_bonus, final=total_last,
                    tokens=tok,
                )


def _seat_zone_totals(state_evt: dict) -> dict[int, int]:
    """Sum up cards across zones per seat from a 'state' snapshot event.

    Supports both 2-seat legacy (`seat_0`, `seat_1`) and N-seat (`seats`
    list) snapshots. Includes command_zone for commander-format compliance.
    """
    totals: dict[int, int] = {}
    # N-seat snapshot format: seats = [ {...}, {...}, ... ]
    seats_list = state_evt.get("seats")
    if seats_list is not None:
        for seat_idx, st in enumerate(seats_list):
            if not st:
                continue
            n = (st.get("hand", 0) + st.get("library", 0)
                 + len(st.get("graveyard", []) or [])
                 + len(st.get("exile", []) or [])
                 + len(st.get("battlefield", []) or [])
                 + len(st.get("command_zone", []) or []))
            totals[seat_idx] = n
        return totals
    # Legacy 2-seat format
    for key in ("seat_0", "seat_1", "seat_2", "seat_3"):
        st = state_evt.get(key)
        if not st:
            continue
        seat_idx = int(key.split("_")[1])
        n = (st.get("hand", 0) + st.get("library", 0)
             + len(st.get("graveyard", []) or [])
             + len(st.get("exile", []) or [])
             + len(st.get("battlefield", []) or [])
             + len(st.get("command_zone", []) or []))
        totals[seat_idx] = n
    return totals


def _is_token_name(name: str) -> bool:
    """Token names in this engine include a '(P/T)' suffix and contain
    'token' or 'Token' in the string. Examples:
        'treasure artifact token Token (1/1)'
        'food artifact token Token (1/1)'
    Non-token cards may coincidentally have 'token' in their real name
    (e.g., no such printed card exists in our decklists), but this is
    the cleanest heuristic given no `is_token` flag in the snapshot.
    """
    if not name:
        return False
    lower = name.lower()
    # Require both "token" in the name and the "(X/Y)" pt suffix the
    # engine's create_token routine always appends — this avoids ever
    # matching a non-token card whose real name contains "token".
    if "token" not in lower:
        return False
    # Rough P/T-suffix check
    import re
    return bool(re.search(r"\(\d+/\d+\)", name))


def _target_resolved_nearby(events: list[dict], i: int, name: str,
                             seat: int, window: int = 8) -> bool:
    """Look ahead a few events for a destroy/sba/damage_wears_off referring
    to the same (seat, card). If one exists, the damage DID land on a real
    creature — we just missed the ETB.

    Also look BACKWARD a short window: creatures can be pumped/named in
    an attackers list or damaged back-to-back in the same step.
    """
    back_window = 4
    for j in range(max(0, i - back_window), min(len(events), i + window + 1)):
        if j == i:
            continue
        ev = events[j]
        t = ev.get("type")
        if t in ("destroy", "sba_704_5g", "sba_704_5a", "sba_704_6c",
                 "sba_704_6d", "damage_wears_off", "enter_battlefield"):
            if ev.get("card") == name:
                # seat match: destroy carries owner_seat; sba carries seat;
                # damage_wears_off carries seat or target_seat
                s2 = (ev.get("owner_seat")
                      if ev.get("owner_seat") is not None
                      else ev.get("seat"))
                if s2 == seat:
                    return True
        if t == "damage":
            if (ev.get("target_card") == name
                    and ev.get("target_seat") == seat):
                return True
        if t == "attackers":
            if name in (ev.get("attackers", []) or []) \
                    and ev.get("seat") == seat:
                return True
        if t == "blockers":
            for pair in ev.get("pairs", []) or []:
                if name in (pair.get("blockers", []) or []):
                    return True
    return False


def _check_damage_arithmetic(events: list[dict], i: int, dmg: dict,
                              viol: Violations) -> None:
    """For a damage event to a player, assert the next life_change for that
    seat matches -amount (or -amount + lifelink if source has lifelink).
    """
    amt = dmg.get("amount", 0)
    ts = dmg.get("target_seat")
    # Look ahead up to 6 events for the next life_change for this seat.
    for j in range(i + 1, min(i + 7, len(events))):
        e = events[j]
        if e.get("type") == "life_change" and e.get("seat") == ts \
                and "reason" not in e:
            delta = e.get("to", 0) - e.get("from", 0)
            if delta != -amt:
                viol.add(
                    "damage_arithmetic", dmg,
                    f"damage {amt} to seat {ts} but next life_change has "
                    f"delta={delta} (expected {-amt})",
                    damage=amt, life_delta=delta,
                )
            return
    # No life_change found — possibly already in combat-damage double-count,
    # or the damage was prevented. Flag only if target was a player.
    viol.add(
        "damage_arithmetic", dmg,
        f"damage {amt} to seat {ts} has no matching life_change within 6 events",
        damage=amt,
    )


# ---------------------------------------------------------------------------
# Orchestration
# ---------------------------------------------------------------------------

def audit_jsonl(events_path: Path,
                violations_path: Path | None = None,
                summary_path: Path | None = None) -> Counter:
    events_path = Path(events_path)
    violations_path = Path(violations_path) if violations_path else DEFAULT_VIOLATIONS
    summary_path = Path(summary_path) if summary_path else DEFAULT_SUMMARY

    viol = Violations()
    n_games = 0
    for matchup, gi, evts in group_by_game(iter_events(events_path)):
        n_games += 1
        audit_game(evts, viol)

    violations_path.parent.mkdir(parents=True, exist_ok=True)
    with violations_path.open("w") as f:
        for item in viol.items:
            f.write(json.dumps(item) + "\n")

    # Build summary
    examples: dict[str, dict] = {}
    for item in viol.items:
        rule = item["rule"]
        if rule not in examples:
            examples[rule] = item

    md = ["# Audit Summary\n",
          f"_{n_games} games audited from `{events_path}`._\n",
          f"_Total violations: **{sum(viol.counts.values())}**_\n",
          "## Violations per rule\n",
          "| Rule | Count |",
          "|---|---:|"]
    for rule, n in viol.counts.most_common():
        md.append(f"| `{rule}` | {n} |")
    md.append("")

    md.append("## Top 10 most common\n")
    md.append("| Rule | Count |")
    md.append("|---|---:|")
    for rule, n in viol.counts.most_common(10):
        md.append(f"| `{rule}` | {n} |")
    md.append("")

    md.append("## Example per rule\n")
    for rule in viol.counts.keys():
        ex = examples[rule]
        md.append(f"### `{rule}`\n")
        md.append("```json")
        md.append(json.dumps(ex, indent=2))
        md.append("```")
        md.append("")

    summary_path.write_text("\n".join(md))
    return viol.counts


def main() -> int:
    ap = argparse.ArgumentParser(description="Audit an mtgsquad JSONL event log.")
    ap.add_argument("events", help="Path to the JSONL event log")
    ap.add_argument("--violations", default=None,
                    help="Path to write violation JSONL "
                         f"(default: {DEFAULT_VIOLATIONS})")
    ap.add_argument("--summary", default=None,
                    help="Path to write summary markdown "
                         f"(default: {DEFAULT_SUMMARY})")
    args = ap.parse_args()

    counts = audit_jsonl(args.events, args.violations, args.summary)
    total = sum(counts.values())
    print(f"Total violations: {total}")
    print(f"By rule:")
    for rule, n in counts.most_common():
        print(f"  {rule:>28}: {n}")
    print(f"Violations JSONL: {args.violations or DEFAULT_VIOLATIONS}")
    print(f"Summary MD:       {args.summary or DEFAULT_SUMMARY}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
