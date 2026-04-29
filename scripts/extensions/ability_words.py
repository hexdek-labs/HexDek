#!/usr/bin/env python3
"""Ability-word inline-trigger extensions.

An "ability word" (comp rules §207.2c) is a flavor label that prefixes an
ability. It carries no rules meaning itself — it exists to group mechanically
related abilities. On Scryfall oracle text, the shape is:

    <ability_word> — <body>

After our normalizer (see parser.normalize), the em-dash is rewritten to a
plain hyphen and text is lowercased:

    landfall - whenever a land you control enters, draw a card.
    threshold - as long as there are seven or more cards in your graveyard, ~ has flying.
    spell mastery - if there are two or more instant and/or sorcery cards in your graveyard, ~ gets +2/+2.
    metalcraft - ~ gets +2/+2 as long as you control three or more artifacts.
    strive - this spell costs {2}{r} more to cast for each target beyond the first.

`parser.parse_static` already has a generic ability-word handler that matches
the prefix and re-runs `parse_triggered` on the body. That works for the
triggered shape (landfall, heroic, constellation, magecraft, raid, revolt on
triggered builds, undergrowth, eerie, addendum, radiance, bloodrush, plot,
discover, inspired, haunt, champion) but MISSES the static-body shapes
(threshold, hellbent, delirium, ferocious, formidable, metalcraft, domain,
fateful hour, lieutenant, spell mastery, coven) and the specialised shapes
(strive, will of the council, tempting offer, council's dilemma, parley,
bolster, cleave, splice, gravestorm, imprint, survival, tempting offer,
morbid conditional riders).

This module exports:

    EFFECT_RULES         — new effect productions (e.g. strive cost rider,
                            vote/parley council bodies)
    STATIC_PATTERNS      — [(re, builder)] new static shapes matched inside
                            parse_static. Each builder returns a Static node
                            tagged with Modification(kind="ability_word",
                            args=(name, sub_kind, ...)) and, when possible,
                            an embedded parsed sub-effect/condition.
    TRIGGER_PATTERNS     — [(re, event, scope)] new trigger shapes (none
                            needed right now — the generic `when`/`whenever`/
                            `at the beginning` handlers cover triggered
                            bodies once the ability-word prefix is stripped).

The parser does not import this file yet; the existing generic handler plus
the ability_words STATIC_PATTERNS below is how a future integration would
wire the extensions in. Until then, calling `try_static_extensions(text)` at
the end of parser.parse_static (before the `conditional_static` fallback)
would shift ~500 ability-word cards from PARTIAL to GREEN.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

# Parser and AST sit one directory up
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mtg_ast import (  # noqa: E402
    Buff, Condition, Conditional, CounterMod, Effect, Filter, Modification,
    Sequence, Static, TARGET_CREATURE, UnknownEffect,
)

# Keep the authoritative list in one place. Ordering is stable — longest
# multi-word names are placed first so they bind before their prefixes (e.g.
# "spell mastery" before "spell", "will of the council" before "will").
ABILITY_WORDS: tuple[str, ...] = (
    "will of the council",
    "council's dilemma",
    "tempting offer",
    "spell mastery",
    "fateful hour",
    "landfall",
    "heroic",
    "constellation",
    "magecraft",
    "coven",
    "ferocious",
    "hellbent",
    "metalcraft",
    "threshold",
    "delirium",
    "morbid",
    "battalion",
    "domain",
    "formidable",
    "imprint",
    "raid",
    "revolt",
    "undergrowth",
    "survival",
    "inspired",
    "eerie",
    "addendum",
    "lieutenant",
    "parley",
    "radiance",
    "bloodrush",
    "bolster",
    "strive",
    "gravestorm",
    "champion",
    "haunt",
    "splice",
    "cleave",
    "plot",
    "discover",
)

# Ability-word prefix: <name> - <body>.  Body is everything after the dash.
_AW_ALT = "|".join(re.escape(w) for w in ABILITY_WORDS)
_AW_PREFIX = re.compile(rf"^({_AW_ALT})\s*-\s*(.+)$", re.I | re.S)


def _split_aw(text: str) -> Optional[tuple[str, str]]:
    """If text begins with `<ability_word> - ...`, return (name, body).
    Else None. Case-insensitive; returns lowercased name."""
    m = _AW_PREFIX.match(text.strip())
    if not m:
        return None
    return m.group(1).lower(), m.group(2).strip()


# ---------------------------------------------------------------------------
# Condition parsing helpers — ability-word bodies very often carry an
# "intervening-if" or "as long as" clause that should become a Condition node.
# ---------------------------------------------------------------------------

_COND_PATTERNS: list[tuple[re.Pattern, str]] = [
    # threshold
    (re.compile(r"there are seven or more cards in your graveyard", re.I),
     "threshold"),
    # delirium
    (re.compile(r"there are four or more card types among cards in your graveyard", re.I),
     "delirium"),
    # spell mastery
    (re.compile(r"there are two or more instant and/or sorcery cards in your graveyard", re.I),
     "spell_mastery"),
    # ferocious
    (re.compile(r"you control a creature with power 4 or greater", re.I),
     "ferocious"),
    # formidable
    (re.compile(r"creatures you control have total power 8 or greater", re.I),
     "formidable"),
    # fateful hour
    (re.compile(r"you have 5 or less life", re.I),
     "fateful_hour"),
    # hellbent
    (re.compile(r"you have no cards in hand", re.I),
     "hellbent"),
    # metalcraft
    (re.compile(r"you control three or more artifacts", re.I),
     "metalcraft"),
    # coven
    (re.compile(r"you control three or more creatures with different powers", re.I),
     "coven"),
    # lieutenant
    (re.compile(r"you control your commander", re.I),
     "lieutenant"),
    # morbid (rider form)
    (re.compile(r"a creature died this turn", re.I),
     "morbid"),
    # battalion
    (re.compile(r"~ and at least two other creatures attack", re.I),
     "battalion"),
    # raid (rider form)
    (re.compile(r"you attacked (?:with a creature |this turn)", re.I),
     "raid"),
    # revolt (rider form)
    (re.compile(r"a permanent you controlled left the battlefield this turn", re.I),
     "revolt"),
    # landfall (rider form)
    (re.compile(r"you had a land enter the battlefield under your control this turn", re.I),
     "landfall"),
    # domain — "for each basic land type among lands you control"
    (re.compile(r"for each basic land type (?:among lands you control|you control)", re.I),
     "domain"),
]


def _detect_condition(body: str) -> Optional[Condition]:
    for pat, kind in _COND_PATTERNS:
        if pat.search(body):
            return Condition(kind=kind, args=())
    return None


# ---------------------------------------------------------------------------
# Body-shape parsers — each returns an Effect (or None).
# These are intentionally narrow; we want the dominant shapes, not everything.
# ---------------------------------------------------------------------------

_BUFF_SELF = re.compile(
    r"^(?:~|this creature) gets \+(\d+)/\+(\d+)\s*(?:and has ([a-z, ]+?))?\s*as long as",
    re.I,
)
_BUFF_SELF_STATIC = re.compile(
    r"^(?:~|this creature) gets \+(\d+)/\+(\d+)\b", re.I,
)
_HAS_SELF = re.compile(r"^(?:~|this creature) has ([a-z, ]+?)(?:\s+as long as|\.|$)", re.I)
_ETB_COUNTERS = re.compile(
    r"^(?:~|this creature) enters (?:the battlefield )?with (\w+) \+1/\+1 counters? on it",
    re.I,
)
_COST_REDUCE_DOMAIN = re.compile(
    r"^this spell costs \{(\d+)\} less to cast for each", re.I,
)
_STRIVE_RIDER = re.compile(
    r"^this spell costs (\{[^}]+\}(?:\{[^}]+\})*) more to cast for each target beyond the first",
    re.I,
)


_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "x": "x",
}


def _parse_body_effect(body: str) -> Optional[Effect]:
    """Try to extract a typed Effect from an ability-word body. Returns None
    when the body is a pure condition/state modifier (handled separately)."""
    b = body.strip().rstrip(".")
    m = _BUFF_SELF.match(b) or _BUFF_SELF_STATIC.match(b)
    if m:
        return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                    target=Filter(base="self", targeted=False),
                    duration="permanent")
    m = _ETB_COUNTERS.match(b)
    if m:
        token = m.group(1).lower()
        n = _NUM_WORDS.get(token, token)
        if isinstance(n, str) and n.isdigit():
            n = int(n)
        return CounterMod(op="put", count=n, counter_kind="+1/+1",
                          target=Filter(base="self", targeted=False))
    m = _STRIVE_RIDER.match(b)
    if m:
        # Represent as an UnknownEffect with a structured tag — real cost-rider
        # nodes don't exist in the AST yet.
        return UnknownEffect(raw_text=f"strive_rider:{m.group(1)}")
    m = _COST_REDUCE_DOMAIN.match(b)
    if m:
        return UnknownEffect(raw_text=f"cost_reduce_per_domain:{m.group(1)}")
    return None


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — each entry is (compiled_regex, builder). The builder
# receives the match object and returns a Static node (or None to decline).
#
# These are intended to be consulted by parse_static() BEFORE its final
# `conditional_static` catch-all but AFTER the existing specific shapes.
# The first pattern here swallows every ability-word-prefixed ability and
# dispatches to the right sub-shape.
# ---------------------------------------------------------------------------


def _build_ability_word(m: re.Match) -> Optional[Static]:
    """Universal builder for `<ability_word> - <body>`.

    Tries, in order:
      1. Triggered body (when / whenever / at the beginning)     — already
         handled upstream, but we re-check to keep this module self-contained.
      2. Static body with detectable Condition + typed sub-effect.
      3. Static body with only a typed sub-effect (no condition).
      4. Static body with only a condition (bare "as long as" / "if").
      5. Fallback: record the ability word and raw body.
    """
    parsed = _split_aw(m.group(0))
    if parsed is None:
        return None
    name, body = parsed

    # 1) Triggered delegation — defer to parser.parse_triggered if the body
    #    starts with a trigger word. Import here to avoid a circular import
    #    at module-load time.
    if re.match(r"^(when|whenever|at the beginning)\b", body, re.I):
        try:
            from parser import parse_triggered  # type: ignore
        except Exception:
            parse_triggered = None  # type: ignore
        if parse_triggered is not None:
            t = parse_triggered(body)
            if t is not None:
                # Re-wrap as a Static that preserves the ability-word label AND
                # the nested triggered AST inside Modification.args.
                return Static(
                    modification=Modification(
                        kind="ability_word",
                        args=(name, "triggered", t),
                    ),
                    raw=m.group(0),
                )

    # 2/3/4) Static body.
    cond = _detect_condition(body)
    sub = _parse_body_effect(body)

    if cond is not None and sub is not None:
        return Static(
            condition=cond,
            modification=Modification(
                kind="ability_word",
                args=(name, "conditional_effect", sub),
            ),
            raw=m.group(0),
        )
    if sub is not None:
        return Static(
            modification=Modification(
                kind="ability_word",
                args=(name, "effect", sub),
            ),
            raw=m.group(0),
        )
    if cond is not None:
        return Static(
            condition=cond,
            modification=Modification(
                kind="ability_word",
                args=(name, "condition_only", body),
            ),
            raw=m.group(0),
        )

    # 5) Last-resort: label and keep the body text as an arg so the cluster
    #    signature still distinguishes this from a truly unparsed static.
    return Static(
        modification=Modification(
            kind="ability_word",
            args=(name, "raw", body),
        ),
        raw=m.group(0),
    )


STATIC_PATTERNS: list[tuple[re.Pattern, object]] = [
    (_AW_PREFIX, _build_ability_word),
]


# ---------------------------------------------------------------------------
# EFFECT_RULES — effects we want to expose to the parser's effect grammar.
# Currently we only add the strive cost-rider because it appears at top level
# in some oracle texts (Clutch of Currents, Wing Shards, etc.) even outside
# an ability-word prefix.
# ---------------------------------------------------------------------------


def _strive_effect_builder(m: re.Match) -> Effect:
    return UnknownEffect(raw_text=f"strive_rider:{m.group(1)}")


def _domain_cost_builder(m: re.Match) -> Effect:
    return UnknownEffect(raw_text=f"cost_reduce_per_domain:{m.group(1)}")


EFFECT_RULES: list[tuple[re.Pattern, object]] = [
    (re.compile(
        r"^this spell costs (\{[^}]+\}(?:\{[^}]+\})*) more to cast for each target beyond the first\.?$",
        re.I),
     _strive_effect_builder),
    (re.compile(
        r"^this spell costs \{(\d+)\} less to cast for each basic land type (?:among lands you control|you control)\.?$",
        re.I),
     _domain_cost_builder),
]


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — matches the parser._TRIGGER_PATTERNS tuple shape exactly.
# Parley and will-of-the-council style votes begin with "starting with you,
# each player votes for ..." which isn't a conventional trigger, so we
# synthesize a pseudo-trigger event "vote" to let parse_triggered recognize
# them once the ability-word prefix is stripped.
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    (re.compile(r"^starting with you, each player votes for", re.I), "vote", "all"),
    (re.compile(r"^each opponent may ", re.I), "tempting_offer", "all"),
]


# ---------------------------------------------------------------------------
# Public helper — a single call parse_static can make at the end.
# ---------------------------------------------------------------------------


def try_static_extensions(text: str) -> Optional[Static]:
    """Run STATIC_PATTERNS against `text` (lowercased, dash-normalized) and
    return the first successful Static, or None."""
    s = text.strip().rstrip(".").lower()
    for pat, builder in STATIC_PATTERNS:
        m = pat.match(s)
        if m:
            out = builder(m)
            if out is not None:
                return out
    return None


__all__ = [
    "ABILITY_WORDS",
    "EFFECT_RULES",
    "STATIC_PATTERNS",
    "TRIGGER_PATTERNS",
    "try_static_extensions",
]
