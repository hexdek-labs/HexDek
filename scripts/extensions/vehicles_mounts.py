"""Vehicle / Mount / Spacecraft / Modular Artifact mechanics.

Family: VEHICLES / SADDLE / MOUNT / STATION + PROTOTYPE / RECONFIGURE.

Covers the modern "artifact becomes creature" mechanic family:

Keyword shapes (parsed as Keyword via STATIC_PATTERNS, since the base
parser's ``KEYWORD_RE`` only recognises ``crew N``):
  - ``crew N`` / ``crew N {cost}``            (Kaladesh Vehicles; Kamigawa alt-cost)
  - ``saddle N``                              (Outlaws of Thunder Junction Mounts)
  - ``station``                               (Aetherdrift Spacecraft - bare keyword)
  - ``prototype {cost} - N/N``                (Brothers' War Prototype)
  - ``reconfigure {cost}``                    (Kamigawa: NON Equipment reconfigure)

Trigger shapes (emitted as Triggered with structured event names):
  - ``whenever this vehicle becomes crewed[...]``     -> ``becomes_crewed``
  - ``whenever this creature/mount becomes saddled``  -> ``becomes_saddled``
  - ``whenever ~ is saddled by N or more creatures``  -> ``saddled_by_n``
  - ``whenever a creature crews ~``                   -> ``crewed_by_actor``
  - ``whenever a creature saddles ~``                 -> ``saddled_by_actor``
  - ``whenever you crew ~``                           -> ``you_crew``
  - ``whenever this creature crews a vehicle``        -> ``self_crews``
  - ``whenever this creature saddles a mount``        -> ``self_saddles``
  - ``whenever a mount/vehicle you control enters``   -> ``ally_mount_etb`` /
                                                        ``ally_vehicle_etb``

Static shapes (anthems / global grants / combat restrictions specific to
the vehicle/mount type line plus the Station "N+ |" level gate):
  - ``mounts and vehicles you control have <kw>``
  - ``vehicles you control have crew N``              (rules-gift anthem)
  - ``vehicles you control have <keyword>``
  - ``vehicles you control become artifact creatures until end of turn``
  - ``this creature crews vehicles as though its power were N greater``
  - ``this creature saddles mounts and crews vehicles as though ...``
  - ``this vehicle can't be blocked by creatures with power N or less``
  - ``N+ | <ability>``                                 (Station level gate)

Ordering: most-specific first. "attack while saddled" already lives in
``combat_triggers.py`` so we don't duplicate it here; we only cover the
remaining shapes.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

# Allow the extension to import mtg_ast whether loaded via importlib.util
# (by parser.load_extensions) or by direct import path.
_HERE = Path(__file__).resolve().parent
if str(_HERE.parent) not in sys.path:
    sys.path.insert(0, str(_HERE.parent))

from mtg_ast import (  # noqa: E402
    Keyword, Static, Triggered, Trigger, Modification, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Self / subject aliases
# ---------------------------------------------------------------------------
_SELF = (
    r"(?:~|this creature|this permanent|this vehicle|this mount|"
    r"this spacecraft|this artifact|this card)"
)


# ===========================================================================
# STATIC_PATTERNS
# Each entry: (compiled_regex, builder(match, raw_text) -> Static|Keyword|None)
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---- Keyword-shape lines: saddle N, station, prototype, reconfigure --------
#
# These sit above other static patterns so that a line that is JUST the
# keyword resolves as a Keyword node rather than falling through to the
# generic "unknown static" fallback.

@_sp(r"^saddle\s+(\d+)\s*$")
def _kw_saddle(m, raw):
    return Keyword(name="saddle", args=(int(m.group(1)),), raw=raw)


@_sp(r"^station\s*$")
def _kw_station(m, raw):
    return Keyword(name="station", args=(), raw=raw)


@_sp(r"^crew\s+(\d+)\s*$")
def _kw_crew(m, raw):
    # parser.KEYWORD_RE already matches "crew N", but some splitter paths
    # hand the Crew line to parse_static directly. Re-assert it here.
    return Keyword(name="crew", args=(int(m.group(1)),), raw=raw)


@_sp(r"^crew\s+(\d+)\s*-\s*(\{[^}]+(?:\}\{[^}]+)*\})\s*$")
def _kw_crew_alt_cost(m, raw):
    # Kamigawa: "Crew 2—{1}{U}" alternate-mana crew (rare)
    return Keyword(name="crew", args=(int(m.group(1)), m.group(2)),
                   raw=raw)


@_sp(r"^reconfigure\s+(\{[^}]+(?:\}\{[^}]+)*\})\s*$")
def _kw_reconfigure(m, raw):
    return Keyword(name="reconfigure", args=(m.group(1),), raw=raw)


@_sp(r"^prototype\s+(\{[^}]+(?:\}\{[^}]+)*\})\s*-\s*(\d+)/(\d+)\s*$")
def _kw_prototype(m, raw):
    return Keyword(
        name="prototype",
        args=(m.group(1), int(m.group(2)), int(m.group(3))),
        raw=raw,
    )


# ---- Anthem / global modifiers for Vehicles & Mounts -----------------------

@_sp(r"^mounts and vehicles you control have ([^.]+?)\s*$")
def _mounts_and_vehicles_have(m, raw):
    return Static(
        modification=Modification(
            kind="tribal_anthem_multi",
            args=(("mount", "vehicle"), m.group(1).strip()),
        ),
        raw=raw,
    )


@_sp(r"^vehicles you control have crew\s+(\d+)\s*$")
def _vehicles_have_crew_n(m, raw):
    return Static(
        modification=Modification(
            kind="tribal_crew_reduction",
            args=("vehicle", int(m.group(1))),
        ),
        raw=raw,
    )


@_sp(r"^vehicles you control have ([^.]+?)\s*$")
def _vehicles_have_kw(m, raw):
    return Static(
        modification=Modification(
            kind="tribal_anthem",
            args=("vehicle", m.group(1).strip()),
        ),
        raw=raw,
    )


@_sp(r"^mounts you control have ([^.]+?)\s*$")
def _mounts_have_kw(m, raw):
    return Static(
        modification=Modification(
            kind="tribal_anthem",
            args=("mount", m.group(1).strip()),
        ),
        raw=raw,
    )


@_sp(r"^vehicles you control become artifact creatures until end of turn\s*$")
def _vehicles_become_creatures(m, raw):
    return Static(
        modification=Modification(kind="mass_animate",
                                  args=("vehicle", "until_end_of_turn")),
        raw=raw,
    )


# ---- "Crews vehicles as though its power were N greater" (pilot anthems) ---

@_sp(rf"^{_SELF} crews vehicles as though (?:its|their) power were\s+(\d+)\s+greater\s*$")
def _self_crews_as_though(m, raw):
    return Static(
        modification=Modification(kind="crew_power_bonus",
                                  args=(int(m.group(1)),)),
        raw=raw,
    )


@_sp(rf"^{_SELF} saddles mounts as though (?:its|their) power were\s+(\d+)\s+greater\s*$")
def _self_saddles_as_though(m, raw):
    return Static(
        modification=Modification(kind="saddle_power_bonus",
                                  args=(int(m.group(1)),)),
        raw=raw,
    )


@_sp(
    rf"^{_SELF} saddles mounts and crews vehicles as though "
    r"(?:its|their) power were\s+(\d+)\s+greater\s*$"
)
def _self_saddle_and_crew(m, raw):
    n = int(m.group(1))
    return Static(
        modification=Modification(kind="saddle_and_crew_power_bonus",
                                  args=(n,)),
        raw=raw,
    )


# ---- Vehicle combat restrictions ------------------------------------------

@_sp(
    rf"^{_SELF} can'?t be blocked by creatures with power\s+(\d+)\s+or less\s*$"
)
def _cant_be_blocked_by_small(m, raw):
    return Static(
        modification=Modification(kind="unblockable_by_power_le",
                                  args=(int(m.group(1)),)),
        raw=raw,
    )


# ---- Station level-gated abilities: "N+ | <ability>" -----------------------
#
# Station spacecraft list tiered abilities keyed to charge-counter counts.
# We capture the threshold and the body text; the body will often itself be
# a keyword list or anthem that downstream rules can further decompose.

@_sp(r"^(\d+)\+\s*\|\s*(.+)$")
def _station_level(m, raw):
    threshold = int(m.group(1))
    body = m.group(2).strip()
    return Static(
        modification=Modification(kind="station_level_ability",
                                  args=(threshold, body)),
        raw=raw,
    )


# ---- "During your turn, mounts and vehicles you control have <kw>" --------

@_sp(
    r"^during your turn,\s+mounts and vehicles you control have ([^.]+?)\s*$"
)
def _your_turn_mounts_vehicles(m, raw):
    return Static(
        modification=Modification(
            kind="tribal_anthem_conditional",
            args=(("mount", "vehicle"), m.group(1).strip(), "your_turn"),
        ),
        raw=raw,
    )


@_sp(r"^during your turn,\s+vehicles you control have ([^.]+?)\s*$")
def _your_turn_vehicles(m, raw):
    return Static(
        modification=Modification(
            kind="tribal_anthem_conditional",
            args=(("vehicle",), m.group(1).strip(), "your_turn"),
        ),
        raw=raw,
    )


# ===========================================================================
# TRIGGER_PATTERNS
# Matches parser._TRIGGER_PATTERNS shape: (regex, event_name, scope).
# Scope "self" = the card itself triggers; "actor" = group(1) is the actor;
# "all" = global event (no specific actor captured here).
# ===========================================================================

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [

    # ---- "becomes crewed" (Vehicles) --------------------------------------
    # Specializations first: the "for the first time each turn" variant.
    (re.compile(rf"^whenever {_SELF} becomes crewed for the first time each turn", re.I),
     "becomes_crewed_first", "self"),
    (re.compile(rf"^whenever {_SELF} becomes crewed", re.I),
     "becomes_crewed", "self"),

    # ---- "becomes saddled" (Mounts) ---------------------------------------
    (re.compile(rf"^whenever {_SELF} becomes saddled for the first time each turn", re.I),
     "becomes_saddled_first", "self"),
    (re.compile(rf"^whenever {_SELF} becomes saddled", re.I),
     "becomes_saddled", "self"),
    # "When this Mount becomes saddled" variant (one-shot "when")
    (re.compile(rf"^when {_SELF} becomes saddled", re.I),
     "becomes_saddled", "self"),

    # ---- "is crewed / saddled by N or more creatures" ---------------------
    (re.compile(
        rf"^whenever {_SELF} is crewed by (\d+) or more creatures?", re.I),
     "crewed_by_n_or_more", "self"),
    (re.compile(
        rf"^whenever {_SELF} is saddled by (\d+) or more creatures?", re.I),
     "saddled_by_n_or_more", "self"),

    # ---- Actor-side: "a creature crews/saddles ~" -------------------------
    (re.compile(r"^whenever a creature crews ~", re.I),
     "crewed_by_actor", "self"),
    (re.compile(r"^whenever a creature saddles ~", re.I),
     "saddled_by_actor", "self"),

    # ---- "Whenever you crew <vehicle>" / "whenever you saddle <mount>" ----
    (re.compile(rf"^whenever you crew {_SELF}", re.I),
     "you_crew_self", "self"),
    (re.compile(r"^whenever you crew a vehicle", re.I),
     "you_crew_any", "self"),
    (re.compile(rf"^whenever you saddle {_SELF}", re.I),
     "you_saddle_self", "self"),
    (re.compile(r"^whenever you saddle a mount", re.I),
     "you_saddle_any", "self"),

    # ---- "Whenever this creature crews a Vehicle" -------------------------
    (re.compile(rf"^whenever {_SELF} crews a vehicle", re.I),
     "self_crews_vehicle", "self"),
    (re.compile(rf"^whenever {_SELF} saddles a mount", re.I),
     "self_saddles_mount", "self"),

    # ---- Ally-ETB variants: "whenever a Mount/Vehicle you control enters"
    (re.compile(r"^whenever a mount you control enters(?: the battlefield)?", re.I),
     "ally_mount_etb", "self"),
    (re.compile(r"^whenever a vehicle you control enters(?: the battlefield)?", re.I),
     "ally_vehicle_etb", "self"),
    (re.compile(r"^whenever a spacecraft you control enters(?: the battlefield)?", re.I),
     "ally_spacecraft_etb", "self"),

    # ---- Station threshold triggers: "When ~ has N or more charge counters"
    (re.compile(
        rf"^when {_SELF} has (\d+) or more charge counters on it", re.I),
     "station_threshold", "self"),
]


__all__ = ["STATIC_PATTERNS", "TRIGGER_PATTERNS", "EFFECT_RULES"]

# No EFFECT_RULES for this family — vehicles/mounts speak entirely through
# triggers and statics; their "effects" (tap, become a creature, attach)
# are handled by reminder text that the normalizer strips before we see it.
EFFECT_RULES: list = []
