#!/usr/bin/env python3
"""Sagas, Adventures, MDFCs, Transform, Class, Battle — multi-zone / multi-form.

Family: MULTI-SHAPE MECHANICS — abilities whose oracle text depends on zone,
face, or lore-counter level. Covers:

  - Saga chapter abilities:      "I - effect", "II - effect", "I, II - effect",
                                 "III - exile this saga, then return it..."
                                 (incl. rider-flavor "I - gungnir - destroy ...")
  - Class level abilities:       "Level 2-6", "Level 7+" header lines that mark
                                 the level band for the following triggered /
                                 static body (Class enchantments — AFR+)
  - Class becomes-level trigger: "When this Class becomes level N, ..."
  - Transform triggers:          "When this creature transforms into ~, ..."
                                 "As this permanent transforms into ~, ..."
                                 day/nightbound face-flip triggers
  - Battle (Siege) triggers:     "When this Battle's last defense counter is
                                 removed, exile it..." — currently only on a
                                 handful of custom / future-planned sieges.
  - MDFC seam:                   parser.normalize already concatenates both
                                 faces; we just guard against stray "Reverse
                                 Side" / "// Face 2" header lines that some
                                 dumps leave behind.

The AST encoding for Saga chapters is intentionally structural — per the
spec, `Modification(kind="saga_chapter", args=(roman_numeral, body_text))`
so two sagas with the same chapter-shape (e.g. "III - exile this saga, then
return it to the battlefield transformed") cluster identically regardless
of surface-level wording differences further out.

Combined chapters "I, II - effect" are emitted with a tuple-roman, e.g.
args=(("I","II"), body_text). Solo chapters emit args=("III", body_text).

Exported:

    STATIC_PATTERNS   — chapter / level header statics
    TRIGGER_PATTERNS  — transform / battle-defeated / class-levelup triggers
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

# Parser and AST sit one directory up.
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mtg_ast import (  # noqa: E402
    Modification, Static,
)


# ---------------------------------------------------------------------------
# Roman-numeral chapter helpers
# ---------------------------------------------------------------------------

# Lowercased roman numerals as they appear after parser.normalize().
# Sagas published today top out at VI (Long List of the Ents) so we keep the
# grammar tight rather than accept arbitrary i/v/x sequences — strict match
# reduces false positives on tokens like "ix" that might appear elsewhere.
_ROMAN_ALT = r"i{1,3}v?|iv|v|vi|vii|viii|ix|x"

# Single chapter:         "iii - exile this saga, ..."
# Combined chapters:      "i, ii - ...",  "i, ii, iii - ..."
# Flavor-subtitle riders: "i - gungnir - destroy target creature ..."
#
# The dash is a normal hyphen because parser.normalize() rewrites em/en dashes
# to ASCII "-" before we see the text.
_CHAPTER_PREFIX = re.compile(
    rf"^(?P<nums>(?:{_ROMAN_ALT})(?:\s*,\s*(?:{_ROMAN_ALT}))*)\s*-\s*(?P<body>.+)$",
    re.I | re.S,
)


def _canon_roman(tok: str) -> str:
    """Uppercase and strip a single roman token for canonical storage."""
    return tok.strip().upper()


def _build_saga_chapter(m: re.Match, raw: str) -> Optional[Static]:
    nums_raw = m.group("nums")
    body = m.group("body").strip().rstrip(".")
    if not body:
        return None
    parts = tuple(_canon_roman(p) for p in nums_raw.split(","))
    # Some sagas use a flavor subtitle: "i - gungnir - destroy target ...".
    # We keep that subtitle attached to body — it's distinctive oracle text
    # and should participate in the cluster signature, not be thrown away.
    roman_arg: object = parts[0] if len(parts) == 1 else parts
    return Static(
        modification=Modification(
            kind="saga_chapter",
            args=(roman_arg, body),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Class level header — "Level 2-6", "Level 7+"
# ---------------------------------------------------------------------------

# Classes (AFR, MH3 Talents) use a stand-alone band header followed by the
# static/triggered body on subsequent lines. We treat the header itself as a
# Static that carries the band. The body lines already parse on their own via
# the triggered / keyword / activated pathways.
_CLASS_LEVEL_HEADER = re.compile(
    r"^level\s+(?P<lo>\d+)(?:\s*(?P<op>[-+])\s*(?P<hi>\d+)?)?$",
    re.I,
)


def _build_class_level(m: re.Match, raw: str) -> Optional[Static]:
    lo = int(m.group("lo"))
    op = m.group("op")
    hi = m.group("hi")
    if op == "+" and hi is None:
        band: tuple = (lo, None)  # open-ended top band, "Level 7+"
    elif op == "-" and hi is not None:
        band = (lo, int(hi))
    else:
        band = (lo, lo)
    return Static(
        modification=Modification(kind="class_level_band", args=band),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Adventure / MDFC seam hygiene
# ---------------------------------------------------------------------------

# Some oracle dumps leave a synthetic "reverse side" / "// name" header on
# the second face. parser.normalize strips parens but not bare headers, so
# we absorb them here as an empty type_add-ish static.
_FACE_HEADER = re.compile(
    r"^(?:reverse side|face \d|//\s*[^.\n]+|[-—]{2,})$",
    re.I,
)


def _build_face_header(m: re.Match, raw: str) -> Optional[Static]:
    return Static(
        modification=Modification(kind="face_header", args=(raw.strip(),)),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Day/Night keywords that stand alone on a face
# ---------------------------------------------------------------------------

# "daybound" / "nightbound" sometimes appear as bare keyword lines after the
# reminder-text strip (werewolf DFCs). parser's keyword expander handles them
# when they're inline, but a bare line occasionally slips through. Emit as a
# Static tagged as self_keyword for stable clustering.
_DAYNIGHT = re.compile(r"^(daybound|nightbound)\b\s*$", re.I)


def _build_daynight(m: re.Match, raw: str) -> Optional[Static]:
    return Static(
        modification=Modification(
            kind="self_keyword", args=(m.group(1).lower(),)
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — consulted by parser.parse_static before its own fallback.
# Ordering: most specific first. Saga chapters match a very distinctive shape
# (leading roman numeral + dash), so they go first; class-level headers are
# similarly distinctive; the MDFC seam header and bare day/nightbound come
# last because they're rare.
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, object]] = [
    (_CHAPTER_PREFIX, _build_saga_chapter),
    (_CLASS_LEVEL_HEADER, _build_class_level),
    (_DAYNIGHT, _build_daynight),
    (_FACE_HEADER, _build_face_header),
]


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — matches parser._TRIGGER_PATTERNS 3-tuple shape.
# ---------------------------------------------------------------------------
#
# Transform triggers come in two flavors:
#   - event-type "when this creature transforms into ~" — a triggered ability
#     that fires on the face flip (Avacyn, Ulrich, etc.)
#   - "as" replacement "as this permanent transforms into ~, ..." — this is
#     technically a replacement effect, but the parser models it cleanly as a
#     trigger-shaped node, so we register it here for coverage.
#
# Battle "last defense counter removed" is the defeated trigger on Sieges.
# After a siege is defeated it transforms into its back face and the back
# face's text takes over — but the front face still carries the trigger.
#
# Class "becomes level N" fires when a Class gains a level counter.

_SELF = (
    r"(?:~|this creature|this permanent|this class|this battle|this siege|"
    r"this card|this artifact|this enchantment|this token)"
)


TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # ---- Transform ----
    # "when this creature transforms into ~, ..."
    (re.compile(rf"^when {_SELF} transforms into ~", re.I),
     "transform_into_self", "self"),
    # "whenever this creature transforms into ~, ..."
    (re.compile(rf"^whenever {_SELF} transforms into ~", re.I),
     "transform_into_self", "self"),
    # "whenever this creature enters or transforms into ~, ..."
    (re.compile(rf"^whenever {_SELF} enters or transforms into ~", re.I),
     "etb_or_transform", "self"),
    # "as this permanent transforms into ~, ..." — replacement-shaped trigger
    (re.compile(rf"^as {_SELF} transforms into ~", re.I),
     "as_transform", "self"),
    # Plain "transform ~" as an effect cue at start of trigger body — rare but
    # appears on cards like Henrika back face in bullet modes.
    # (Not registered as a trigger; the effect grammar handles it.)

    # ---- Class level-up ----
    # "when this class becomes level N, ..."
    (re.compile(rf"^when {_SELF} becomes level (\d+)", re.I),
     "class_becomes_level", "named"),

    # ---- Battle defeated ----
    # "when this battle's last defense counter is removed, ..."
    (re.compile(rf"^when {_SELF}'s last defense counter is removed", re.I),
     "battle_defeated", "self"),
    # Generic fallback with apostrophe after the noun phrase.
    (re.compile(r"^when [^,.]+?'s last defense counter is removed", re.I),
     "battle_defeated", "actor"),

    # ---- Day / Night shift ----
    # "at the beginning of each upkeep, if ... , it becomes day/night"
    # is a phase trigger and already handled by the base grammar; we add the
    # narrower "it becomes day" / "it becomes night" event-shaped triggers
    # that some cards carry on the back face.
    (re.compile(r"^when it becomes (day|night)", re.I),
     "day_night_flip", "named"),
]


__all__ = ["STATIC_PATTERNS", "TRIGGER_PATTERNS"]
