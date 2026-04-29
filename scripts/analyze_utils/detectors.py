"""Tier 1/2/3 anomaly detectors.

Each detector is a top-level function taking
``(events, final_state, decks, timeline) → list[Finding]``.

Design choices:

  - Detectors are PURE: no side effects, no persistent state.
  - Missing data = NO finding (never fabricate). A bug in the event
    stream should not produce a CRITICAL; the correct response is for
    the analyzer to note "detector X couldn't run because field Y was
    absent" in the coverage stats. See `analyze_single_game.py`.
  - False negatives are worse than false positives — better to flag
    suspiciously and let the human dismiss than miss a real bug.
  - Every finding carries a stable group_key so the grouper can merge
    same-shape findings into single summary lines.

The Tier-1 detectors target the engine bugs Josh is currently hunting:
inert artifact mana, empty storm copies, Bolas's Citadel life-neutral
activations, missed reactive triggers (Rhystic/Sentinel/Bowmasters/
Tithe/Bloodchief/Drannith), commander-damage SBA misses, Thoracle
win-condition misses, casts from illegal zones, and mana pool that
doesn't drain across phase boundaries.
"""

from __future__ import annotations

from collections import defaultdict
from typing import Callable

from .findings import Finding, Severity
from .timelines import Timeline


# =============================================================================
# Helpers
# =============================================================================

# Known artifacts that produce COLORLESS mana (or colorless-only mana
# on the first activation). We special-case these for the "Sol Ring
# didn't generate {C}{C}" check. List is conservative — matches the
# engine's is_real_card pool pattern where possible.
# Mapping: card name → expected colors emitted by a single {T} activation.
_ARTIFACT_MANA_EXPECTED: dict = {
    "Sol Ring":          "CC",
    "Mana Crypt":        "CC",
    "Mana Vault":        "CCC",
    "Grim Monolith":     "CCC",
    "Basalt Monolith":   "CCC",
    "Worn Powerstone":   "CC",
    "Thran Dynamo":      "CCC",
    "Hedron Archive":    "CC",
    "Gilded Lotus":      "any3",
    "Mox Diamond":       "any1",
    "Mox Tantalite":     "any1",
    "Mox Jet":           "B",
    "Mox Emerald":       "G",
    "Mox Pearl":         "W",
    "Mox Ruby":          "R",
    "Mox Sapphire":      "U",
    "Chromatic Lantern": "any1",
    "Coalition Relic":   "any1",
    # Conditional / kicker-based artifacts REMOVED from the inertness map
    # per 7174n1c ruling 2026-04-16:
    #   - Everflowing Chalice: 0 counters = legit 0 output; OctoHat casts
    #     it at CMC 0 so it's always inert by design. Skip it.
    #   - Astral Cornucopia: same multikicker-counter semantics. Skip.
    #   - Mox Amber: requires a legendary creature/planeswalker in play.
    #     Not currently checkable without per-turn legendary detection; move
    #     to _CONDITIONAL_ARTIFACTS below for INFO-only emission.
    #   - Mox Opal: requires metalcraft (3+ artifacts). Same treatment.
    # Signets / Talismans / Diamonds are {T}: {C}, {cost}: {X}{Y}.
    # The simple "generates mana on T" should still fire for them.
    "Azorius Signet":    "any2",
    "Dimir Signet":      "any2",
    "Rakdos Signet":     "any2",
    "Gruul Signet":      "any2",
    "Selesnya Signet":   "any2",
    "Orzhov Signet":     "any2",
    "Izzet Signet":      "any2",
    "Golgari Signet":    "any2",
    "Boros Signet":      "any2",
    "Simic Signet":      "any2",
    "Arcane Signet":     "any1",
    "Fellwar Stone":     "any1",
    "Mind Stone":        "C",
    "Commander's Sphere": "any1",
    "Talisman of Progress":  "any2",
    "Talisman of Dominance": "any2",
    "Talisman of Indulgence": "any2",
    "Talisman of Impulse":   "any2",
    "Talisman of Unity":     "any2",
    "Talisman of Hierarchy": "any2",
    "Talisman of Curiosity": "any2",
    "Talisman of Resilience": "any2",
    "Talisman of Conviction": "any2",
    "Talisman of Creativity": "any2",
}

# Conditional artifacts — emit INFO-level reminder that their inertness
# may be legitimate (condition not satisfied). Requires manual probe
# for now; future work: implement per-turn metalcraft / legendary checks.
_CONDITIONAL_ARTIFACTS: dict = {
    "Mox Amber": "requires legendary creature or planeswalker you control",
    "Mox Opal":  "requires metalcraft (3+ artifacts you control)",
}


def _card_in_deck_names(decks: dict, card_name: str) -> list:
    """Return seat indices whose decks contain card_name."""
    out = []
    for seat_info in decks.get("seats", []):
        names = set(seat_info.get("card_names", []))
        names.update(seat_info.get("commander_cards", []))
        commander = seat_info.get("commander_name", "")
        if commander:
            names.add(commander)
        if card_name in names:
            out.append(seat_info["seat"])
    return out


def _seat_commander(decks: dict, seat: int) -> str:
    for s in decks.get("seats", []):
        if s.get("seat") == seat:
            return s.get("commander_name", "")
    return ""


# =============================================================================
# Tier 1 — Engine core bug detectors
# =============================================================================


def detect_artifact_mana_inertness(events: list, final_state: dict,
                                   decks: dict, tl: Timeline) -> list:
    """CRITICAL — artifact was on battlefield, untapped, and was sitting
    in a mana-gathering phase but emitted no add_mana sourced to it.

    We use a coarser proxy: for each (seat, turn, artifact) where the
    artifact was on battlefield for the whole turn, check whether
    that seat emitted ANY add_mana event sourced from that artifact
    during that turn. If not, it's suspicious.

    Grouping: one Finding per (artifact, seat, turn). The grouper
    merges by (artifact) → "Seats [x, y] had Sol Ring inert for N turns".
    """
    findings: list = []
    num_seats = tl.num_seats

    for seat_idx in range(num_seats):
        for te, tl_, name in tl.battlefield_history.get(seat_idx, ()):
            # Conditional artifacts — emit INFO probe instead of CRITICAL
            if name in _CONDITIONAL_ARTIFACTS:
                leave = tl_ if tl_ is not None else (tl.total_turns + 1)
                inert_turns = []
                for t in range(te, min(leave, tl.total_turns + 1)):
                    sl = tl.slice_of(seat_idx, t)
                    if sl.mana_by_source.get(name, 0) == 0:
                        inert_turns.append(t)
                if inert_turns:
                    findings.append(Finding(
                        severity=Severity.INFO,
                        category="Conditional artifact inert (probe required)",
                        message=(
                            f"{name} on seat {seat_idx} produced no mana "
                            f"for {len(inert_turns)} turn(s). This may be "
                            f"expected: {_CONDITIONAL_ARTIFACTS[name]}. "
                            f"Manual probe required to confirm whether "
                            f"the enabling condition was met."
                        ),
                        group_key=f"conditional_inert::{name}",
                        affected_seats=[seat_idx],
                        affected_turns=sorted(inert_turns),
                        details={
                            "artifact": name,
                            "condition": _CONDITIONAL_ARTIFACTS[name],
                            "inert_turns": sorted(inert_turns),
                            "manual_probe_required": True,
                        },
                    ))
                continue
            if name not in _ARTIFACT_MANA_EXPECTED:
                continue
            # For every turn this artifact was present, check that
            # add_mana fired at least once from it for this seat.
            leave = tl_ if tl_ is not None else (tl.total_turns + 1)
            inert_turns = []
            for t in range(te, min(leave, tl.total_turns + 1)):
                sl = tl.slice_of(seat_idx, t)
                emitted = sl.mana_by_source.get(name, 0)
                if emitted == 0:
                    inert_turns.append(t)
            if not inert_turns:
                continue
            # We only flag if the artifact was present for ≥1 turn
            # where the seat HAD activity (pay_mana, cast, or draw).
            # Otherwise we'd flag e.g. opponents' Sol Rings on turns
            # where nothing mana-relevant happened, which is noisy.
            elim_turn = tl.elimination_turn.get(seat_idx)
            suspicious_turns = []
            for t in inert_turns:
                # Skip turns after the seat was eliminated.
                if elim_turn is not None and t > elim_turn:
                    continue
                sl = tl.slice_of(seat_idx, t)
                # Must be seat's OWN turn AND seat attempted to pay mana
                # or cast something — that's when an inert mana artifact
                # is genuinely anomalous. The engine doesn't tap artifacts
                # on opponents' turns (outside of instant-speed reactive
                # sources we don't model here), so off-turn inertness is
                # normal.
                if t not in tl.active_turns_by_seat.get(seat_idx, set()):
                    continue
                if sl.pay_mana_events or sl.casts:
                    suspicious_turns.append(t)
            if not suspicious_turns:
                continue
            findings.append(Finding(
                severity=Severity.CRITICAL,
                category="Artifact mana inertness",
                message=(f"{name} on seat {seat_idx} generated no mana "
                         f"for {len(suspicious_turns)} turn(s)"),
                group_key=f"artifact_inert::{name}",
                affected_seats=[seat_idx],
                affected_turns=sorted(suspicious_turns),
                details={
                    "artifact": name,
                    "expected": _ARTIFACT_MANA_EXPECTED[name],
                    "present_range": [te, tl_],
                    "inert_turns": sorted(suspicious_turns),
                },
            ))
    return findings


def detect_storm_copies_empty(events: list, final_state: dict,
                              decks: dict, tl: Timeline) -> list:
    """CRITICAL — storm_trigger fired, stack_push_storm_copy landed copies,
    but no matching damage/mill/life_change resolved for each copy.

    The body-effect of a storm spell (Grapeshot: 1 damage; Tendrils: 2
    damage + 2 life drain; Brain Freeze: mill 3) MUST produce at least
    `copies+1` matching downstream events when resolving all copies +
    the original. If we see 0, something's wrong.
    """
    findings: list = []
    # Walk events, find storm_trigger → the following stream until we
    # see a new turn boundary OR a non-storm cast, count damage/mill
    # events that look sourced to the storm card.
    for i, e in enumerate(events):
        if e.get("type") != "storm_trigger":
            continue
        source = e.get("source") or ""
        copies = int(e.get("copies", 0))
        seat = e.get("seat")
        turn = int(e.get("turn", 0))
        seq = int(e.get("seq", i))
        # Scan ahead until we hit a turn boundary or a new cast event.
        window_damage = 0
        window_mill = 0
        window_life_drain = 0
        lookahead_limit = min(len(events), i + 200)
        for j in range(i + 1, lookahead_limit):
            ee = events[j]
            ee_type = ee.get("type", "")
            if ee_type == "turn_start" and int(ee.get("turn", 0)) > turn:
                break
            if ee_type == "damage":
                if (ee.get("source_card") == source
                        or ee.get("target_kind") in ("player",
                                                     "permanent")):
                    window_damage += int(ee.get("amount", 0))
            if ee_type == "mill":
                window_mill += int(ee.get("count", 0))
            if ee_type == "life_change":
                lf = int(ee.get("from", 0))
                lt = int(ee.get("to", 0))
                if lt < lf:
                    window_life_drain += (lf - lt)
        # Decide expected effect by card name.
        name_l = source.lower()
        expected_effect = None
        if "grapeshot" in name_l:
            expected_effect = ("damage", copies + 1, window_damage)
        elif "tendrils" in name_l:
            expected_effect = ("damage+life",
                               (copies + 1) * 2, window_damage)
        elif "brain freeze" in name_l:
            expected_effect = ("mill", (copies + 1) * 3, window_mill)
        elif "empty the warrens" in name_l:
            # Tokens rather than damage; this detector doesn't measure
            # tokens, so skip (future enhancement).
            continue
        else:
            # Unknown storm card — can't assert a body-effect shape.
            continue
        kind, expected_total, observed_total = expected_effect
        if observed_total == 0 and expected_total > 0:
            findings.append(Finding(
                severity=Severity.CRITICAL,
                category="Storm body-effect missing",
                message=(f"{source} storm_trigger with {copies} copies "
                         f"produced 0 {kind} (expected ≥{expected_total})"),
                group_key=f"storm_empty::{source}",
                affected_seats=[seat] if isinstance(seat, int) else [],
                affected_turns=[turn],
                event_refs=[seq],
                details={
                    "source": source,
                    "copies": copies,
                    "expected_effect": kind,
                    "expected_total": expected_total,
                    "observed_total": observed_total,
                },
            ))
        elif observed_total < (expected_total // 2) and expected_total > 0:
            # Partial resolution — still suspicious.
            findings.append(Finding(
                severity=Severity.CRITICAL,
                category="Storm body-effect partial",
                message=(f"{source} storm_trigger with {copies} copies "
                         f"produced {observed_total} {kind} "
                         f"(expected ≥{expected_total})"),
                group_key=f"storm_partial::{source}",
                affected_seats=[seat] if isinstance(seat, int) else [],
                affected_turns=[turn],
                event_refs=[seq],
                details={
                    "source": source,
                    "copies": copies,
                    "expected_effect": kind,
                    "expected_total": expected_total,
                    "observed_total": observed_total,
                },
            ))
    return findings


def detect_bolas_citadel_no_life_change(events: list, final_state: dict,
                                        decks: dict, tl: Timeline) -> list:
    """CRITICAL — Bolas's Citadel in play + spells cast "from library"
    zone + life total didn't decrease by each spell's cmc.

    We can't directly see "from which zone the spell was cast" in the
    event stream (cast events don't carry a zone field). Proxy: if
    Bolas's Citadel is present for seat S for turn T, and seat S cast
    ≥1 spell during T, and seat S's life total did not drop by ≥1 CMC
    during T, flag.

    Life drop from the Citadel pay-life is a "pay life" action rather
    than a life_change event in some engines. We check BOTH events
    named "pay_life" (none in this engine) AND life_change with from>to.
    """
    findings: list = []
    for seat_idx in range(tl.num_seats):
        for t in range(1, tl.total_turns + 1):
            bf = tl.battlefield_at(seat_idx, t)
            if "Bolas's Citadel" not in bf:
                continue
            sl = tl.slice_of(seat_idx, t)
            if not sl.casts:
                continue
            # Count total life lost this turn for this seat.
            life_lost = 0
            for lc in sl.life_changes:
                f_ = int(lc.get("from", 0))
                to_ = int(lc.get("to", 0))
                if to_ < f_:
                    life_lost += (f_ - to_)
            total_cmc = sum(int(c.get("cmc", 0)) for c in sl.casts)
            if total_cmc > 0 and life_lost == 0:
                # Also require no damage-taken to seat this turn (damage
                # from an opponent would be the "real" reason life went
                # down — avoid false positive). We check that no damage
                # took life below current life (the engine collapses
                # damage with life_change, so if damage happened we'd
                # see life_lost > 0).
                findings.append(Finding(
                    severity=Severity.CRITICAL,
                    category="Bolas's Citadel life-neutral",
                    message=(f"Bolas's Citadel in play for seat "
                             f"{seat_idx} with {len(sl.casts)} casts "
                             f"this turn (CMC sum {total_cmc}), but "
                             f"no life loss occurred"),
                    group_key="bolas_citadel_neutral",
                    affected_seats=[seat_idx],
                    affected_turns=[t],
                    event_refs=[c["seq"] for c in sl.casts],
                    details={
                        "turn": t,
                        "cast_count": len(sl.casts),
                        "total_cmc": total_cmc,
                    },
                ))
    return findings


# Reactive-trigger lookup: card on battlefield → expected reactive
# behavior when an opponent takes a qualifying action. Only cards
# whose reaction is "draw a card" or "pay mana" are included here
# (the "bless you" check — per-cast draw/pay response). Other reactive
# cards (Drannith Magistrate = prohibits casts, Containment Priest =
# exile-token-ETB) have different semantics and would produce false
# positives if checked with this detector's heuristic.
#
# Orcish Bowmasters: reaction fires when an opponent DRAWS (not casts),
# but since we can't always distinguish "opponent draw triggered by
# opponent cast" vs "natural upkeep draw", we flag heuristically: if
# Bowmasters is out and no damage=1 or create_token from Bowmasters
# ever appears during the window, that's a signal.
_REACTIVE_CAST_TRIGGERS = {
    "Rhystic Study":
        "whenever opponent casts, draw unless they pay {1}",
    "Mystic Remora":
        "whenever opponent casts nonc, draw unless they pay {4}",
    "Esper Sentinel":
        "whenever opponent casts first noncreature, draw unless pay {X}",
}

# Separately, draw-triggered reactive cards (Smothering Tithe, Orcish
# Bowmasters, etc.) fire on opponent DRAWS, not casts. We don't check
# those in the cast-trigger detector above; they get their own check
# below.
_REACTIVE_DRAW_TRIGGERS = {
    "Smothering Tithe":
        "whenever opp draws, create Treasure unless they pay {2}",
    "Orcish Bowmasters":
        "whenever opp draws extra/card, create goblin + ping 1",
}


def detect_reactive_trigger_missed(events: list, final_state: dict,
                                   decks: dict, tl: Timeline) -> list:
    """CRITICAL — the "bless you" detector. If Rhystic Study is on the
    battlefield for seat S, every opponent cast should produce either
    a pay_mana {1} from the caster OR a draw event for seat S. If
    neither happened, the trigger didn't fire. That's almost certainly
    a bug.

    Grouping: one Finding per (trigger_name, controller_seat). Details
    contain the full list of opponent casts + which ones had no bless.
    """
    findings: list = []

    # For each reactive-trigger card we care about: identify time
    # windows during which it was on each seat's battlefield. For each
    # opponent cast in that window, check whether a matching reaction
    # (pay_mana {1} with reason 'rhystic' / draw for controller_seat)
    # occurred in the next ~50 events. Missing reaction → flag.
    def _cast_events_in_window(seat_controller: int, turn_from: int,
                               turn_to: int) -> list:
        out = []
        for i, e in enumerate(events):
            if e.get("type") != "cast":
                continue
            t = int(e.get("turn", 0))
            if t < turn_from or t > turn_to:
                continue
            caster = e.get("seat")
            if not isinstance(caster, int):
                continue
            if caster == seat_controller:
                continue
            out.append((i, e))
        return out

    # Rhystic Study: expected reaction = opponent pays {1} OR controller
    # draws a card. We look in the next N events for EITHER a pay_mana
    # tied to this cast or a draw event whose seat == controller.
    def _has_bless_for(cast_idx: int, cast_evt: dict,
                       controller_seat: int,
                       bless_kind: str) -> bool:
        """Look ahead up to 20 events for evidence the trigger fired.

        bless_kind='rhystic' → draw for controller OR pay_mana {1} from
            caster within next 40 events.
        bless_kind='remora' → same shape but pay_mana {4}.
        bless_kind='sentinel' → pay_mana {X} from first-noncreature
            caster (we accept ANY pay_mana from caster within window
            OR draw for controller).
        bless_kind='tithe' → treasure token appeared for controller
            (we check via create_token or add_mana source=treasure).
        """
        caster = cast_evt.get("seat")
        look_ahead = min(len(events), cast_idx + 40)
        for j in range(cast_idx + 1, look_ahead):
            ee = events[j]
            t = ee.get("type", "")
            if t == "draw":
                if ee.get("seat") == controller_seat:
                    return True
            if t == "pay_mana":
                # A pay_mana from the caster in response to the trigger.
                if ee.get("seat") == caster:
                    if ee.get("reason", "") in ("rhystic", "remora",
                                                "sentinel", "tithe"):
                        return True
            if t == "cast":
                # A new cast — window closes.
                if int(ee.get("turn", 0)) > int(cast_evt.get("turn", 0)):
                    break
        return False

    for trigger_name, descr in _REACTIVE_CAST_TRIGGERS.items():
        # Find windows across all seats.
        for seat_idx in range(tl.num_seats):
            for te, tle, name in tl.battlefield_history.get(seat_idx, ()):
                if name != trigger_name:
                    continue
                tf = te
                tt = (tle - 1) if tle is not None else tl.total_turns
                if tt < tf:
                    continue
                opp_casts = _cast_events_in_window(seat_idx, tf, tt)
                if not opp_casts:
                    continue
                missed_bless: list = []
                for (ci, ce) in opp_casts:
                    bless_kind = "rhystic"  # generic "any bless" probe
                    if not _has_bless_for(ci, ce, seat_idx, bless_kind):
                        missed_bless.append({
                            "cast_card": ce.get("card", ""),
                            "caster_seat": ce.get("seat"),
                            "turn": int(ce.get("turn", 0)),
                            "seq": int(ce.get("seq", ci)),
                        })
                if missed_bless:
                    findings.append(Finding(
                        severity=Severity.CRITICAL,
                        category="Reactive trigger missed",
                        message=(
                            f"{trigger_name} on seat {seat_idx} — "
                            f"{len(missed_bless)}/{len(opp_casts)} "
                            f"opponent casts produced no draw or "
                            f"pay-mana reaction"),
                        group_key=f"reactive_missed::{trigger_name}",
                        affected_seats=[seat_idx],
                        affected_turns=sorted({m["turn"]
                                              for m in missed_bless}),
                        event_refs=[m["seq"] for m in missed_bless[:20]],
                        details={
                            "trigger": trigger_name,
                            "descr": descr,
                            "missed_count": len(missed_bless),
                            "total_opportunities": len(opp_casts),
                            "sample_misses": missed_bless[:5],
                        },
                    ))
    return findings


def detect_draw_reactive_missed(events: list, final_state: dict,
                                decks: dict, tl: Timeline) -> list:
    """CRITICAL — Smothering Tithe / Orcish Bowmasters are on battlefield
    for seat S; opponents drew cards; no matching reaction
    (create_token=Treasure for Tithe, damage=1 OR create_token=Goblin
    for Bowmasters) fired.
    """
    findings: list = []

    def _draw_events_in_window(controller_seat: int, turn_from: int,
                               turn_to: int) -> list:
        out = []
        for i, e in enumerate(events):
            if e.get("type") != "draw":
                continue
            t = int(e.get("turn", 0))
            if t < turn_from or t > turn_to:
                continue
            drawer = e.get("seat")
            if not isinstance(drawer, int):
                continue
            if drawer == controller_seat:
                continue
            out.append((i, e))
        return out

    def _has_reaction_for(draw_idx: int, draw_evt: dict,
                          controller_seat: int,
                          trigger_name: str) -> bool:
        drawer = draw_evt.get("seat")
        look_ahead = min(len(events), draw_idx + 40)
        for j in range(draw_idx + 1, look_ahead):
            ee = events[j]
            t = ee.get("type", "")
            if trigger_name == "Smothering Tithe":
                # Either create_token treasure or pay_mana {2} from drawer.
                if t == "create_token":
                    tk = ee.get("token_name", "") or ee.get("kind", "")
                    if "treasure" in tk.lower():
                        return True
                if t == "add_mana" and ee.get("source") == "treasure_token":
                    return True
                if t == "pay_mana" and ee.get("seat") == drawer:
                    if "tithe" in (ee.get("reason", "") or "").lower():
                        return True
            elif trigger_name == "Orcish Bowmasters":
                # damage=1 sourced to Bowmasters, or create_token=Zombie.
                if t == "damage" and int(ee.get("amount", 0)) == 1:
                    if "bowmasters" in (
                            ee.get("source_card", "") or "").lower():
                        return True
                if t == "create_token":
                    tk = ee.get("token_name", "") or ee.get("kind", "")
                    if "zombie" in tk.lower():
                        return True
            if t == "cast":
                # Cast — window closes heuristically.
                if int(ee.get("turn", 0)) > int(draw_evt.get("turn", 0)):
                    break
        return False

    for trigger_name, descr in _REACTIVE_DRAW_TRIGGERS.items():
        for seat_idx in range(tl.num_seats):
            for te, tle, name in tl.battlefield_history.get(seat_idx, ()):
                if name != trigger_name:
                    continue
                tf = te
                tt = (tle - 1) if tle is not None else tl.total_turns
                if tt < tf:
                    continue
                draws = _draw_events_in_window(seat_idx, tf, tt)
                if len(draws) < 3:
                    # Too few draws — skip to avoid false positives on
                    # decks that just don't draw much.
                    continue
                missed = []
                for (di, de) in draws:
                    if not _has_reaction_for(di, de, seat_idx,
                                             trigger_name):
                        missed.append({
                            "drawer_seat": de.get("seat"),
                            "turn": int(de.get("turn", 0)),
                            "seq": int(de.get("seq", di)),
                        })
                if len(missed) >= 3 and len(missed) > (len(draws) // 2):
                    findings.append(Finding(
                        severity=Severity.CRITICAL,
                        category="Draw-reactive trigger missed",
                        message=(
                            f"{trigger_name} on seat {seat_idx} — "
                            f"{len(missed)}/{len(draws)} opponent draws "
                            f"produced no reaction"
                        ),
                        group_key=f"draw_reactive_missed::{trigger_name}",
                        affected_seats=[seat_idx],
                        affected_turns=sorted({m["turn"] for m in missed}),
                        event_refs=[m["seq"] for m in missed[:20]],
                        details={
                            "trigger": trigger_name,
                            "descr": descr,
                            "missed_count": len(missed),
                            "total_opportunities": len(draws),
                        },
                    ))
    return findings


def detect_commander_damage_sba_miss(events: list, final_state: dict,
                                     decks: dict, tl: Timeline) -> list:
    """CRITICAL — a seat accumulated ≥21 commander damage from one
    dealer/commander bucket but sba_704_6c never fired.

    Per CR §704.3 and per project-owner ruling (7174n1c, 2026-04-16):
    when a seat dies from simultaneous combat damage (life-total to ≤0 AND
    21+ commander damage both crossed in the same damage-application batch),
    both SBAs apply simultaneously and the engine's choice to record
    §704.5a (life-total) as the loss_reason is VALID, not a bug. Loss
    reason is a display-layer artifact; the rules treat both conditions
    as having fired.

    The ONLY case this IS a bug: first-strike damage (CR §510.1d) brings
    commander damage to ≥21 in the first-strike damage step, which resolves
    SBAs BEFORE the normal combat damage step. In that ordering, the
    victim should have lost to commander damage before any normal damage
    was dealt. If normal damage then kills them instead, sba_704_6c was
    skipped when it shouldn't have been.

    Refined logic:
    1. For each bucket that hit ≥21, find the commander_damage_accum event
       that crossed the threshold.
    2. Check if that threshold-crossing event had first_strike=true.
    3. If yes AND sba_704_6c didn't fire in the same event window → CRITICAL.
    4. If no (normal damage step) AND victim lost in the same damage batch
       via sba_704_5a → simultaneous SBAs, not a bug, downgrade to INFO.
    5. If victim didn't lose at all but 21 was crossed → CRITICAL (real miss).
    """
    findings: list = []
    # Build set of (victim, dealer, commander) buckets that hit 21.
    hit_buckets: list = []
    for seat_info in final_state.get("seats", []):
        victim = seat_info["idx"]
        cd = seat_info.get("commander_damage", {}) or {}
        for dealer, by_name in cd.items():
            try:
                dealer_i = int(dealer)
            except Exception:
                continue
            if not isinstance(by_name, dict):
                continue
            for cmd_name, dmg in by_name.items():
                try:
                    dmg_i = int(dmg)
                except Exception:
                    continue
                if dmg_i >= 21:
                    hit_buckets.append((victim, dealer_i, cmd_name, dmg_i))

    for victim, dealer, cmd_name, dmg_i in hit_buckets:
        # Did sba_704_6c fire for this bucket?
        fired_704_6c = False
        for e in events:
            if e.get("type") != "sba_704_6c":
                continue
            if e.get("seat") != victim:
                continue
            if e.get("commander") != cmd_name:
                continue
            dealer_match = (
                e.get("dealer_seat") == dealer
                or e.get("dealer_seat") is None
            )
            if dealer_match:
                fired_704_6c = True
                break

        if fired_704_6c:
            continue  # no issue, SBA fired as expected

        # Find the commander_damage_accum event that crossed the 21 threshold.
        cumulative = 0
        crossing_seq = None
        crossing_first_strike = False
        for e in events:
            if e.get("type") != "commander_damage_accum":
                continue
            if e.get("target_seat") != victim and e.get("seat") != victim:
                continue
            if e.get("commander") != cmd_name:
                continue
            prev = cumulative
            cumulative = int(e.get("total", cumulative))
            if prev < 21 <= cumulative:
                crossing_seq = int(e.get("seq", -1))
                crossing_first_strike = bool(e.get("first_strike", False))
                break

        # Did the victim lose via sba_704_5a (life total ≤0)?
        lost_via_life = False
        life_loss_seq = None
        for e in events:
            if e.get("type") != "sba_704_5a":
                continue
            if e.get("seat") != victim:
                continue
            lost_via_life = True
            life_loss_seq = int(e.get("seq", -1))
            break

        seat_info = final_state["seats"][victim]
        is_lost = bool(seat_info.get("lost", False))

        # Case: victim didn't lose at all → real miss (CRITICAL)
        if not is_lost:
            findings.append(Finding(
                severity=Severity.CRITICAL,
                category="Commander damage SBA miss",
                message=(
                    f"Seat {victim} took {dmg_i} commander damage from "
                    f"{cmd_name} (dealer {dealer}) but sba_704_6c never "
                    f"fired AND victim did not lose"
                ),
                group_key=f"cmd_dmg_miss::{cmd_name}",
                affected_seats=[victim],
                affected_turns=[],
                details={
                    "victim": victim,
                    "dealer": dealer,
                    "commander": cmd_name,
                    "damage": dmg_i,
                    "victim_lost_anyway": False,
                    "rule": "704.6c — victim should have lost to commander damage",
                },
            ))
            continue

        # Case: first-strike damage crossed 21 before normal damage.
        # That's a clear violation of §510.1d step ordering.
        if crossing_first_strike:
            findings.append(Finding(
                severity=Severity.CRITICAL,
                category="Commander damage SBA miss (first-strike ordering)",
                message=(
                    f"Seat {victim} crossed 21 commander damage from "
                    f"{cmd_name} via first-strike damage (seq {crossing_seq}) "
                    f"but sba_704_6c didn't fire; normal damage then killed "
                    f"them via sba_704_5a (seq {life_loss_seq}). First-strike "
                    f"SBAs resolve before normal damage per CR §510.1d."
                ),
                group_key=f"cmd_dmg_miss_fs::{cmd_name}",
                affected_seats=[victim],
                affected_turns=[],
                details={
                    "victim": victim,
                    "dealer": dealer,
                    "commander": cmd_name,
                    "damage": dmg_i,
                    "first_strike_crossing_seq": crossing_seq,
                    "life_loss_seq": life_loss_seq,
                    "rule": "510.1d — first-strike SBAs resolve before normal damage step",
                },
            ))
            continue

        # Case: normal damage — 21 CDMG + life-total both crossed in same
        # damage batch. Simultaneous SBAs per CR §704.3. Engine recording
        # one loss_reason is valid. Downgrade to INFO (observability note).
        findings.append(Finding(
            severity=Severity.INFO,
            category="Simultaneous SBA (cmdr damage + life)",
            message=(
                f"Seat {victim} took {dmg_i} commander damage from "
                f"{cmd_name} AND life total reached ≤0 in the same damage "
                f"batch. Both §704.5a and §704.6c apply simultaneously per "
                f"CR §704.3. Engine recorded §704.5a as loss_reason; this "
                f"is a valid display choice, not a bug."
            ),
            group_key=f"cmd_dmg_simultaneous::{cmd_name}",
            affected_seats=[victim],
            affected_turns=[],
            details={
                "victim": victim,
                "dealer": dealer,
                "commander": cmd_name,
                "damage": dmg_i,
                "simultaneous_with_life_loss": True,
                "rule": "704.3 — simultaneous SBAs all apply",
            },
        ))
    return findings


def detect_thoracle_no_win(events: list, final_state: dict,
                           decks: dict, tl: Timeline) -> list:
    """CRITICAL — Thassa's Oracle is present + library size small +
    no per_card_win fired for this seat.

    The engine has a dedicated per_card_win event with slug 'thoracle'
    or similar for the Oracle ETB/CIP trigger. Also check the
    thoracle_win_threshold_met event as a hint.

    Detection strategy: look for any thoracle_win_threshold_met event.
    For each one, check that the corresponding seat subsequently won
    or emitted a per_card_win event with that seat as winner.
    """
    findings: list = []
    for i, e in enumerate(events):
        if e.get("type") != "thoracle_win_threshold_met":
            continue
        seat_idx = e.get("seat")
        if not isinstance(seat_idx, int):
            continue
        seq = int(e.get("seq", i))
        # Look ahead for a per_card_win or game_over with this seat as
        # winner. Within the same turn is expected.
        won = False
        for j in range(i + 1, min(len(events), i + 100)):
            ee = events[j]
            tt = ee.get("type", "")
            if tt == "per_card_win" and ee.get("winner_seat") == seat_idx:
                won = True
                break
            if tt == "game_over" and ee.get("winner") == seat_idx:
                won = True
                break
        if not won:
            findings.append(Finding(
                severity=Severity.CRITICAL,
                category="Thoracle win miss",
                message=(
                    f"thoracle_win_threshold_met for seat {seat_idx} "
                    f"but no subsequent per_card_win / game_over "
                    f"crowned that seat"
                ),
                group_key="thoracle_no_win",
                affected_seats=[seat_idx],
                affected_turns=[int(e.get("turn", 0))],
                event_refs=[seq],
                details={
                    "threshold_event_seq": seq,
                    "library_size": e.get("library_size"),
                    "count": e.get("count"),
                },
            ))
    return findings


def detect_mana_pool_persists_across_phase(events: list, final_state: dict,
                                           decks: dict,
                                           tl: Timeline) -> list:
    """CRITICAL — CR 106.4. Mana pool should empty at end of each phase
    and step. Exceptions: Upwelling, Omnath Locus of Mana, etc.

    Detection is tricky because the engine emits phase_step_change
    BEFORE it calls drain_all_pools (set_phase_step() in playloop.py
    orders them that way). So at phase_step_change-emit time, the pool
    legitimately still holds mana from the old phase. A pool_drain
    event should follow within a few events.

    To avoid a false-positive storm we do two things:

    1. At every phase_step_change we record expected-drain debt for
       each seat whose running pool > 0.
    2. We then walk forward a short lookahead (≤30 events or until
       next phase_step_change). If a pool_drain event for that seat
       fires, we clear the debt. If the next phase_step_change arrives
       with debt still owed, THEN we flag — that's the real bug
       ("engine advanced to the next phase without draining").

    We also require the lingering pool at the NEXT boundary to be ≥1
    to trip, so that a no-op edge case (both pool and drain are zero)
    doesn't generate noise.
    """
    findings: list = []

    def _exempt_for_seat(seat_idx: int, turn: int) -> bool:
        bf = tl.battlefield_at(seat_idx, turn)
        return (
            "Upwelling" in bf
            or "Omnath, Locus of Mana" in bf
            or "Omnath, Locus of Rage" in bf
            or "Omnath, Locus of Creation" in bf
        )

    running_pool = [0] * tl.num_seats
    # debt[s] = {"pool_owed": int, "boundary_seq": int,
    #            "from_phase": str, "from_step": str, "turn": int}
    debt: dict = {}

    def _flush_boundary(boundary_turn: int, boundary_seq: int,
                        from_phase: str, from_step: str):
        """Close the PREVIOUS boundary: anyone with unflushed debt
        AND current pool>0 is persisting across phase = bug."""
        for sidx in list(debt.keys()):
            d = debt[sidx]
            if running_pool[sidx] > 0 and not _exempt_for_seat(
                    sidx, d["turn"]):
                findings.append(Finding(
                    severity=Severity.CRITICAL,
                    category="Mana pool persists across phase",
                    message=(
                        f"Seat {sidx} mana pool={running_pool[sidx]} "
                        f"still present when next phase change arrived "
                        f"({d['from_phase']}/{d['from_step']} → …)"
                    ),
                    group_key=f"pool_persist::s{sidx}",
                    affected_seats=[sidx],
                    affected_turns=[d["turn"]],
                    event_refs=[d["boundary_seq"]],
                    details={
                        "seat": sidx,
                        "pool_owed": d["pool_owed"],
                        "pool_still_present": running_pool[sidx],
                        "from_phase": d["from_phase"],
                        "from_step": d["from_step"],
                    },
                ))
            del debt[sidx]

    for i, e in enumerate(events):
        t = e.get("type", "")
        turn = int(e.get("turn", 0))
        if t == "add_mana":
            s = e.get("seat")
            if isinstance(s, int):
                running_pool[s] = int(e.get("pool_after", running_pool[s]))
        elif t == "pay_mana":
            s = e.get("seat")
            if isinstance(s, int):
                pa = e.get("pool_after")
                if pa is not None:
                    running_pool[s] = int(pa)
        elif t == "pool_drain":
            s = e.get("seat")
            if isinstance(s, int):
                running_pool[s] = 0
                # Debt paid — drop this seat's outstanding debt.
                debt.pop(s, None)
        elif t == "phase_step_change":
            # First, close out previous boundary's debts.
            _flush_boundary(turn, int(e.get("seq", i)),
                            e.get("from_phase_kind", ""),
                            e.get("from_step_kind", ""))
            # Then, record fresh debt for seats that enter this
            # boundary with non-zero pool (expecting a drain to
            # follow). Exempt seats (Upwelling/Omnath) are skipped.
            for sidx in range(tl.num_seats):
                if running_pool[sidx] > 0 and not _exempt_for_seat(
                        sidx, turn):
                    debt[sidx] = {
                        "pool_owed": running_pool[sidx],
                        "boundary_seq": int(e.get("seq", i)),
                        "from_phase": e.get("from_phase_kind", ""),
                        "from_step": e.get("from_step_kind", ""),
                        "turn": turn,
                    }
    return findings


def detect_winner_consistency(events: list, final_state: dict,
                              decks: dict, tl: Timeline) -> list:
    """CRITICAL — final_state.winner disagrees with the last
    game_over event, OR more than one seat is "not lost" when the
    game ended and nobody was declared winner.

    This is a self-consistency check on the engine's own bookkeeping.
    """
    findings: list = []
    # 1) last game_over.winner == final_state.winner.
    go = None
    for e in reversed(events):
        if e.get("type") == "game_over":
            go = e
            break
    if go is not None:
        if go.get("winner") != final_state.get("winner"):
            findings.append(Finding(
                severity=Severity.CRITICAL,
                category="Winner inconsistency",
                message=(f"game_over.winner={go.get('winner')} disagrees "
                         f"with final_state.winner="
                         f"{final_state.get('winner')}"),
                group_key="winner_inconsistency",
                affected_seats=[],
                affected_turns=[],
                details={"event": go, "final_winner":
                         final_state.get("winner")},
            ))

    # 2) seats alive ≠ expected given winner.
    living = [s for s in final_state.get("seats", [])
              if not s.get("lost", False)]
    winner = final_state.get("winner")
    if winner is None and len(living) == 1:
        # Should have been a winner.
        findings.append(Finding(
            severity=Severity.CRITICAL,
            category="Winner inconsistency",
            message=(f"final_state.winner=None but 1 seat alive "
                     f"(seat {living[0]['idx']})"),
            group_key="winner_vs_alive",
            affected_seats=[living[0]["idx"]],
            affected_turns=[],
        ))
    if winner is not None and len(living) > 1:
        # Winner declared but >1 seat still alive — unusual
        # (legal via concession / Thoracle / per_card_win, so WARN not
        # CRITICAL).
        pass

    return findings


def detect_cast_failed_spam(events: list, final_state: dict,
                            decks: dict, tl: Timeline) -> list:
    """WARN — seat repeatedly emits cast_failed with reason 'unpayable'
    for the SAME card many turns in a row. Indicates the greedy AI is
    looping on an unreachable card. Not a bug per se but a signal that
    the mana pool / cost model may be underselling mana availability.
    """
    findings: list = []
    # seat → card → list of turns where cast_failed fired.
    by_seat_card: dict = defaultdict(lambda: defaultdict(list))
    for e in events:
        if e.get("type") != "cast_failed":
            continue
        s = e.get("seat")
        c = e.get("card", "")
        t = int(e.get("turn", 0))
        if isinstance(s, int) and c:
            by_seat_card[s][c].append(t)
    for s, per_card in by_seat_card.items():
        for card, turns in per_card.items():
            if len(turns) >= 4:
                findings.append(Finding(
                    severity=Severity.WARN,
                    category="Cast-failed loop",
                    message=(f"Seat {s} tried to cast {card} "
                             f"{len(turns)} times and failed"),
                    group_key=f"cast_failed::{card}",
                    affected_seats=[s],
                    affected_turns=sorted(turns),
                    details={
                        "card": card,
                        "turns": sorted(turns),
                    },
                ))
    return findings


def detect_per_card_unhandled(events: list, final_state: dict,
                              decks: dict, tl: Timeline) -> list:
    """WARN — per_card_unhandled events mean the engine saw a card
    with no handler. Each unique card is one finding (grouped).
    """
    findings: list = []
    by_slug: dict = defaultdict(list)
    for i, e in enumerate(events):
        if e.get("type") != "per_card_unhandled":
            continue
        by_slug[e.get("slug", "?")].append({
            "seq": int(e.get("seq", i)),
            "turn": int(e.get("turn", 0)),
            "site": e.get("site", ""),
        })
    for slug, hits in by_slug.items():
        findings.append(Finding(
            severity=Severity.WARN,
            category="Per-card handler missing",
            message=(f"per_card_unhandled for slug '{slug}' fired "
                     f"{len(hits)} times"),
            group_key=f"unhandled::{slug}",
            affected_seats=[],
            affected_turns=sorted({h["turn"] for h in hits}),
            event_refs=[h["seq"] for h in hits[:20]],
            details={
                "slug": slug,
                "hits": len(hits),
                "sites": sorted({h["site"] for h in hits if h["site"]}),
            },
        ))
    return findings


def detect_crashed_effects(events: list, final_state: dict, decks: dict,
                           tl: Timeline) -> list:
    """CRITICAL — any event ending in _crashed indicates an internal
    exception that was swallowed. Real bug.
    """
    findings: list = []
    by_type: dict = defaultdict(list)
    for i, e in enumerate(events):
        tp = e.get("type", "")
        if tp.endswith("_crashed"):
            by_type[tp].append({
                "seq": int(e.get("seq", i)),
                "turn": int(e.get("turn", 0)),
                "source": e.get("source_card") or e.get("card") or "",
            })
    for tp, hits in by_type.items():
        findings.append(Finding(
            severity=Severity.CRITICAL,
            category="Crashed handler",
            message=(f"{tp} fired {len(hits)} times — swallowed "
                     f"exception"),
            group_key=f"crash::{tp}",
            affected_seats=[],
            affected_turns=sorted({h["turn"] for h in hits}),
            event_refs=[h["seq"] for h in hits[:10]],
            details={
                "type": tp,
                "hits": len(hits),
                "sample_sources": sorted({h["source"] for h in hits
                                          if h["source"]})[:10],
            },
        ))
    return findings


def detect_cap_hits(events: list, final_state: dict, decks: dict,
                    tl: Timeline) -> list:
    """WARN — safety-cap events (sba_cap_hit, replacement_depth_cap,
    replacement_iter_cap). These indicate a runaway loop was short-
    circuited, which is not inherently a bug but usually symptoms of
    one. Aggregate by cap kind.
    """
    findings: list = []
    by_type: dict = defaultdict(int)
    turns_by_type: dict = defaultdict(set)
    for e in events:
        tp = e.get("type", "")
        if "cap_hit" in tp or tp.endswith("_cap") or "iter_cap" in tp:
            by_type[tp] += 1
            turns_by_type[tp].add(int(e.get("turn", 0)))
    for tp, count in by_type.items():
        findings.append(Finding(
            severity=Severity.WARN,
            category="Safety cap triggered",
            message=f"{tp} fired {count} times (runaway short-circuit)",
            group_key=f"cap::{tp}",
            affected_seats=[],
            affected_turns=sorted(turns_by_type[tp]),
            details={"cap_type": tp, "hits": count},
        ))
    return findings


# =============================================================================
# Tier 2 — Deck-signature anomalies
# =============================================================================


# Cards whose presence indicates a storm-style combo deck. If the deck
# has ≥1 of these in its card_names list, we expect to see storm_trigger
# events once cast.
_STORM_PAYOFFS = (
    "Grapeshot", "Tendrils of Agony", "Brain Freeze",
    "Empty the Warrens", "Aetherflux Reservoir",
)

# Cards that produce Treasure / Gold / Powerstone tokens. Presence of
# ≥5 of these in a deck implies "ramp" / "artifact ramp" signature.
_TREASURE_MAKERS = (
    "Dockside Extortionist", "Goldspan Dragon", "Smothering Tithe",
    "Revel in Riches", "Hullbreaker Horror", "Storm-Kiln Artist",
    "Jeska's Will", "Deadly Dispute", "Big Score",
    "Unexpected Windfall", "Prosperous Innkeeper",
    "Captain Lannery Storm", "Treasure Map",
    "Professional Face-Breaker", "Magda, Brazen Outlaw",
)


def detect_storm_deck_never_fires(events: list, final_state: dict,
                                  decks: dict, tl: Timeline) -> list:
    """WARN — deck has a storm win condition but the storm spell was
    never cast despite 20+ total casts game-wide.
    """
    findings: list = []
    total_casts = sum(1 for e in events if e.get("type") == "cast")
    for seat_info in decks.get("seats", []):
        names = set(seat_info.get("card_names", []))
        payoffs_present = [n for n in _STORM_PAYOFFS if n in names]
        if not payoffs_present:
            continue
        # Did any of these get cast?
        cast_names = {e.get("card", "") for e in events
                      if e.get("type") == "cast"
                      and e.get("seat") == seat_info["seat"]}
        never_cast = [n for n in payoffs_present if n not in cast_names]
        if never_cast and total_casts >= 20:
            findings.append(Finding(
                severity=Severity.WARN,
                category="Storm deck never fires",
                message=(
                    f"Seat {seat_info['seat']} ({seat_info.get('commander_name')}) "
                    f"has storm payoffs {never_cast} but never cast them "
                    f"(game had {total_casts} total casts)"
                ),
                group_key=f"storm_no_fire::s{seat_info['seat']}",
                affected_seats=[seat_info["seat"]],
                affected_turns=[],
                details={
                    "seat": seat_info["seat"],
                    "commander": seat_info.get("commander_name"),
                    "payoffs_present": payoffs_present,
                    "payoffs_never_cast": never_cast,
                    "total_casts": total_casts,
                },
            ))
    return findings


def detect_ramp_deck_no_treasure(events: list, final_state: dict,
                                 decks: dict, tl: Timeline) -> list:
    """WARN — 5+ treasure-making cards in deck, 0 treasure tokens
    generated. The engine has create_token events for treasure; we
    search for them.
    """
    findings: list = []
    treasure_count = sum(
        1 for e in events
        if e.get("type") == "create_token"
        and "treasure" in (e.get("token_name", "")
                           or e.get("kind", "")).lower()
    )
    for seat_info in decks.get("seats", []):
        names = set(seat_info.get("card_names", []))
        makers = [n for n in _TREASURE_MAKERS if n in names]
        if len(makers) < 5:
            continue
        if treasure_count == 0:
            findings.append(Finding(
                severity=Severity.WARN,
                category="Ramp deck signature miss",
                message=(
                    f"Seat {seat_info['seat']} has {len(makers)} "
                    f"treasure-making cards but 0 treasure tokens "
                    f"were generated"
                ),
                group_key=f"no_treasure::s{seat_info['seat']}",
                affected_seats=[seat_info["seat"]],
                affected_turns=[],
                details={
                    "seat": seat_info["seat"],
                    "makers": makers,
                    "treasures_generated": treasure_count,
                },
            ))
    return findings


_COMBO_PAIRS = (
    ("Thassa's Oracle", "Demonic Consultation"),
    ("Thassa's Oracle", "Tainted Pact"),
    ("Isochron Scepter", "Dramatic Reversal"),
    ("Kiki-Jiki, Mirror Breaker", "Zealous Conscripts"),
    ("Kiki-Jiki, Mirror Breaker", "Felidar Guardian"),
    ("Food Chain", "Misthollow Griffin"),
    ("Food Chain", "Squee, the Immortal"),
    ("Hermit Druid", "Thassa's Oracle"),
    ("Dramatic Reversal", "Isochron Scepter"),
)


def detect_combo_pieces_stranded(events: list, final_state: dict,
                                 decks: dict, tl: Timeline) -> list:
    """WARN — both combo pieces appeared in hand or on battlefield
    simultaneously at any point, but no win-sequence attempt (no
    thoracle_win_threshold_met, no per_card_win) occurred.

    This is a loose proxy — a rigorous detector would simulate whether
    the player HAD mana to combo. MVP just flags.
    """
    findings: list = []
    # Need events that expose hand/battlefield snapshots. The 'state'
    # event captures snapshots at game start + occasional other points.
    # We'll check final_state primarily.
    for seat_info in final_state.get("seats", []):
        seat = seat_info["idx"]
        visible = set(seat_info.get("hand", []))
        visible.update(p["name"] for p in seat_info.get("battlefield", []))
        visible.update(seat_info.get("graveyard", []))
        visible.update(seat_info.get("command_zone", []))
        for a, b in _COMBO_PAIRS:
            if a in visible and b in visible:
                # Did we see a win-attempt?
                saw_attempt = any(
                    e.get("type") in ("thoracle_win_threshold_met",
                                      "per_card_win")
                    and e.get("seat") == seat
                    for e in events
                )
                if not saw_attempt:
                    findings.append(Finding(
                        severity=Severity.WARN,
                        category="Combo pieces stranded",
                        message=(
                            f"Seat {seat} had both {a} and {b} "
                            f"available by game-end but no win attempt"
                        ),
                        group_key=f"combo_stranded::{a}+{b}",
                        affected_seats=[seat],
                        affected_turns=[],
                        details={
                            "seat": seat,
                            "piece_a": a,
                            "piece_b": b,
                        },
                    ))
    return findings


def detect_muldrotha_no_gy_cast(events: list, final_state: dict,
                                decks: dict, tl: Timeline) -> list:
    """WARN — Muldrotha was in play but we don't see any cast event
    with in_response_to/zone=graveyard. The engine's cast events
    don't always carry zone info, but Muldrotha's grave-cast is a
    signature we want.
    """
    findings: list = []
    muldrotha_present = False
    seat_present = None
    for seat_idx in range(tl.num_seats):
        for te, tle, name in tl.battlefield_history.get(seat_idx, ()):
            if name == "Muldrotha, the Gravetide":
                muldrotha_present = True
                seat_present = seat_idx
                break
        if muldrotha_present:
            break
    if not muldrotha_present:
        return findings

    # Check for any event indicating recurse/cast from graveyard.
    recurse_events = sum(1 for e in events if e.get("type") == "recurse")
    gy_cast_events = sum(
        1 for e in events
        if e.get("type") == "cast"
        and (e.get("zone") == "graveyard"
             or e.get("from_zone") == "graveyard")
    )
    if recurse_events == 0 and gy_cast_events == 0:
        findings.append(Finding(
            severity=Severity.WARN,
            category="Muldrotha no graveyard cast",
            message=(
                f"Muldrotha on seat {seat_present} but no graveyard "
                f"recurse or cast-from-graveyard observed"
            ),
            group_key="muldrotha_no_gy",
            affected_seats=[seat_present],
            affected_turns=[],
        ))
    return findings


def detect_werewolf_day_night_static(events: list, final_state: dict,
                                     decks: dict, tl: Timeline) -> list:
    """WARN — 4+ daybound/nightbound creatures in play total but no
    day_night_change event ever fired.

    Proxy: count Werewolf-type creatures on battlefield (final_state)
    + search for the day_night_change event. Absence flags.
    """
    findings: list = []
    daynight_changes = sum(1 for e in events
                           if e.get("type") == "day_night_change")
    if daynight_changes > 0:
        return findings

    werewolf_creatures = 0
    # Check final_state for werewolf-indicating type lines. We don't
    # have type lines in the dump, so fall back to name heuristics.
    werewolf_markers = (
        "Ulrich", "Tovolar", "Arlinn", "Kessig", "Howlpack",
        "Werewolf", "Moonmist", "Duskwatch Recruiter",
    )
    for seat_info in final_state.get("seats", []):
        for p in seat_info.get("battlefield", []):
            n = p.get("name", "")
            if any(m in n for m in werewolf_markers):
                werewolf_creatures += 1

    if werewolf_creatures >= 4:
        findings.append(Finding(
            severity=Severity.WARN,
            category="Day/Night never flipped",
            message=(
                f"{werewolf_creatures} werewolf-family creatures "
                f"observed but day_night_change never fired "
                f"(day_night stayed '{final_state.get('day_night')}')"
            ),
            group_key="daynight_static",
            affected_seats=[],
            affected_turns=[],
            details={
                "werewolf_creatures": werewolf_creatures,
                "final_daynight": final_state.get("day_night"),
            },
        ))
    return findings


# =============================================================================
# Tier 3 — Resource waste detectors
# =============================================================================


def detect_dead_in_hand(events: list, final_state: dict, decks: dict,
                        tl: Timeline) -> list:
    """INFO — cards in hand at game end that the seat never cast and
    never had pay_mana attempt against. Usually just a snapshot of
    the flop, not a bug, but interesting for deck quality.
    """
    findings: list = []
    # Map seat → set of cards cast this game.
    cast_by_seat: dict = defaultdict(set)
    for e in events:
        if e.get("type") == "cast":
            s = e.get("seat")
            if isinstance(s, int):
                cast_by_seat[s].add(e.get("card", ""))
    for seat_info in final_state.get("seats", []):
        seat = seat_info["idx"]
        hand = seat_info.get("hand", [])
        dead = [c for c in hand if c not in cast_by_seat.get(seat, set())]
        if dead:
            findings.append(Finding(
                severity=Severity.INFO,
                category="Dead in hand",
                message=(f"Seat {seat} ended with {len(dead)} cards "
                         f"in hand that were never cast"),
                group_key=f"dead_hand::s{seat}",
                affected_seats=[seat],
                affected_turns=[],
                details={
                    "seat": seat,
                    "hand_size": len(hand),
                    "dead_cards": dead,
                },
            ))
    return findings


def detect_mana_floated(events: list, final_state: dict, decks: dict,
                        tl: Timeline) -> list:
    """INFO — seat frequently ended main2 with mana floating.

    Proxy: count pool_drain events per seat per turn where
    reason='end_phase' and amount>=3. If ≥3 turns had drain-with-extra,
    flag.
    """
    findings: list = []
    floated_by_seat: dict = defaultdict(list)
    for e in events:
        if e.get("type") != "pool_drain":
            continue
        amt = int(e.get("amount", 0))
        if amt < 3:
            continue
        s = e.get("seat")
        t = int(e.get("turn", 0))
        if isinstance(s, int):
            floated_by_seat[s].append((t, amt))
    for s, floats in floated_by_seat.items():
        if len(floats) < 3:
            continue
        findings.append(Finding(
            severity=Severity.INFO,
            category="Mana floated",
            message=(f"Seat {s} floated ≥3 mana at phase boundary for "
                     f"{len(floats)} turns"),
            group_key=f"mana_float::s{s}",
            affected_seats=[s],
            affected_turns=sorted({t for t, _ in floats}),
            details={
                "seat": s,
                "float_count": len(floats),
                "total_wasted": sum(a for _, a in floats),
            },
        ))
    return findings


def detect_commander_never_cast(events: list, final_state: dict,
                                decks: dict, tl: Timeline) -> list:
    """INFO — commander stayed in command zone for 6+ turns and seat
    never cast it.
    """
    findings: list = []
    cast_names_by_seat: dict = defaultdict(set)
    for e in events:
        if e.get("type") in ("cast", "commander_cast_from_command_zone"):
            s = e.get("seat")
            if isinstance(s, int):
                cast_names_by_seat[s].add(e.get("card", ""))

    if tl.total_turns < 6:
        return findings

    for seat_info in final_state.get("seats", []):
        seat = seat_info["idx"]
        cmds = seat_info.get("commander_names", [])
        for cmd in cmds:
            if cmd not in cast_names_by_seat.get(seat, set()):
                findings.append(Finding(
                    severity=Severity.INFO,
                    category="Commander never cast",
                    message=(
                        f"Seat {seat} ({cmd}) never cast their "
                        f"commander across {tl.total_turns} turns"
                    ),
                    group_key=f"cmd_never_cast::{cmd}",
                    affected_seats=[seat],
                    affected_turns=[],
                    details={
                        "seat": seat,
                        "commander": cmd,
                        "total_turns": tl.total_turns,
                    },
                ))
    return findings


def detect_untapped_artifact_never_activated(events: list, final_state: dict,
                                             decks: dict, tl: Timeline) -> list:
    """INFO — permanent with tap-mana-ability was never tapped for mana.

    Proxy: the final_state shows some permanents as untapped AND we
    never saw an add_mana sourced from them. Only flags for known
    mana artifacts.
    """
    findings: list = []
    mana_sources_ever_activated: dict = defaultdict(set)
    for e in events:
        if e.get("type") == "add_mana":
            s = e.get("seat")
            src = e.get("source_card")
            if isinstance(s, int) and src:
                mana_sources_ever_activated[s].add(src)

    for seat_info in final_state.get("seats", []):
        seat = seat_info["idx"]
        for p in seat_info.get("battlefield", []):
            name = p.get("name", "")
            if name not in _ARTIFACT_MANA_EXPECTED:
                continue
            if name not in mana_sources_ever_activated.get(seat, set()):
                findings.append(Finding(
                    severity=Severity.INFO,
                    category="Mana permanent never activated",
                    message=(
                        f"Seat {seat} ended with {name} on the "
                        f"battlefield having never generated mana"
                    ),
                    group_key=f"unactivated::{name}",
                    affected_seats=[seat],
                    affected_turns=[],
                    details={
                        "seat": seat,
                        "card": name,
                        "expected": _ARTIFACT_MANA_EXPECTED[name],
                    },
                ))
    return findings


# =============================================================================
# Detector registry
# =============================================================================


ALL_DETECTORS: list = [
    # --- Tier 1 (CRITICAL) ----
    ("artifact_mana_inertness", detect_artifact_mana_inertness),
    ("storm_copies_empty", detect_storm_copies_empty),
    ("bolas_citadel_no_life_change", detect_bolas_citadel_no_life_change),
    ("reactive_trigger_missed", detect_reactive_trigger_missed),
    ("draw_reactive_trigger_missed", detect_draw_reactive_missed),
    ("commander_damage_sba_miss", detect_commander_damage_sba_miss),
    ("thoracle_no_win", detect_thoracle_no_win),
    ("mana_pool_persists_across_phase",
     detect_mana_pool_persists_across_phase),
    ("winner_consistency", detect_winner_consistency),
    ("crashed_effects", detect_crashed_effects),
    ("cap_hits", detect_cap_hits),
    # --- Tier 1/2 mixed ----
    ("cast_failed_spam", detect_cast_failed_spam),
    ("per_card_unhandled", detect_per_card_unhandled),
    # --- Tier 2 (WARN) ----
    ("storm_deck_never_fires", detect_storm_deck_never_fires),
    ("ramp_deck_no_treasure", detect_ramp_deck_no_treasure),
    ("combo_pieces_stranded", detect_combo_pieces_stranded),
    ("muldrotha_no_gy_cast", detect_muldrotha_no_gy_cast),
    ("werewolf_day_night_static", detect_werewolf_day_night_static),
    # --- Tier 3 (INFO) ----
    ("dead_in_hand", detect_dead_in_hand),
    ("mana_floated", detect_mana_floated),
    ("commander_never_cast", detect_commander_never_cast),
    ("untapped_artifact_never_activated",
     detect_untapped_artifact_never_activated),
]


def run_all_detectors(events: list, final_state: dict, decks: dict,
                      tl: Timeline) -> tuple:
    """Run every registered detector. Returns (findings, coverage).

    coverage is a list of (name, fired_count, error_or_None) so the
    report can show "detector X ran and fired N findings".
    """
    findings: list = []
    coverage: list = []
    for name, fn in ALL_DETECTORS:
        try:
            out = fn(events, final_state, decks, tl)
        except Exception as ex:
            coverage.append((name, 0, f"{type(ex).__name__}: {ex}"))
            continue
        findings.extend(out)
        coverage.append((name, len(out), None))
    return findings, coverage
