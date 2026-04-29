#!/usr/bin/env python3
"""Conditional ETB / 'may have ~ enter' extensions.

Oracle text has a cluster of ETB ("enters the battlefield") shapes that all
revolve around **decoration at entry time**: the creature or permanent arrives
with counters, as a copy, tapped under conditions, or with an optional choice
made by its controller. The shapes in scope here are:

    Optional-copy ETBs (replacement on self-ETB):
        you may have this creature enter as a copy of any creature on the battlefield
        you may have this land enter tapped as a copy of any land card in a graveyard
        you may have ~ enter as a copy of another creature you control, except ...

    ETB-with-counters (replacement on self-ETB, with counters as a rider):
        this creature enters with three +1/+1 counters on it
        this artifact enters with x charge counters on it
        ~ enters with four +1/+1 counters on it
        this creature enters with a -1/-1 counter on it                 (singular)
        this creature enters with a shield counter on it                (non-p1p1)
        this creature enters with two +1/+1 counters on it for each creature ...
        this creature enters with a +1/+1 counter on it if you attacked this turn
        this creature enters with x +1/+1 counters on it, where x is [...]

    "As ~ enters" (comp rules §603.6f — these are REPLACEMENT effects
    that fire simultaneously with the ETB, not triggered abilities, but we
    model them here as ETB-event Triggered nodes so parse_triggered consumes
    them; the effect body carries the choice/name/exile-select action):
        as ~ enters, choose a creature type
        as ~ enters, choose a color
        as ~ enters, choose a basic land type
        as ~ enters, choose left or right
        as ~ enters, choose a number
        as ~ enters, choose a card type (other than creature or land)
        as ~ enters, name a card
        as ~ enters, you may pay 3 life. if you don't, it enters tapped
        as ~ enters the battlefield, choose a color

    ETB tapped riders (static/replacement):
        ~ enters tapped and doesn't untap during your next untap step
        ~ enters tapped unless you control a Plains
        ~ enters tapped and attacking

    Intervening-if on ETB triggers (already partially handled by
    parse_triggered — we just normalize the shape):
        when ~ enters, if you control a swamp, draw a card
        if [condition] when ~ enters, ...

The AST types we target:
  - Triggered(Trigger(event="etb_as"), effect=<choice/name/selection>)
    for the "as ~ enters" shape.
  - Static(Modification(kind="etb_with_counters", args=(n, counter_kind, [cond]))
    for "~ enters with N <kind> counters on it".
  - Static(Modification(kind="etb_may_copy", args=(filter_raw,))
    for "you may have ~ enter as a copy of ...".
  - Static(Modification(kind="etb_tapped_unless", args=(cond,)))
    for "enters tapped unless ...".
  - Static(Modification(kind="etb_tapped_and_no_untap"))
    for the Icy-Manipulator-style "enters tapped, doesn't untap next turn".
  - Triggered wrapped with Conditional(...) body for intervening-if ETB.

The parser does not import this file yet; wiring would look like:

    # in parser.parse_static, BEFORE the `as long as / if` fallback:
    from extensions.conditional_etb import STATIC_PATTERNS as _CETB_STATIC
    for pat, builder in _CETB_STATIC:
        m = pat.match(s)
        if m:
            out = builder(m, text)
            if out is not None:
                return out

    # in parser.parse_triggered, BEFORE the existing _TRIGGER_PATTERNS loop:
    from extensions.conditional_etb import TRIGGER_PATTERNS as _CETB_TRIG
    for pat, event, scope in _CETB_TRIG:
        ...

    # parse_effect already exposes EFFECT_RULES via @rule — to reuse this
    # module's EFFECT_RULES you'd either call them directly from parse_effect
    # or iterate them the same way as the in-file rules.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mtg_ast import (  # noqa: E402
    Condition, Conditional, CounterMod, CreateToken, Effect, Filter,
    Modification, Replacement, Static, Trigger, Triggered, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
    "that many": "var", "a number of": "var",
}


def _num(token: str):
    """Coerce a spelled/numeric count word to int|str."""
    t = token.strip().lower()
    if t.isdigit():
        return int(t)
    if t in _NUM_WORDS:
        n = _NUM_WORDS[t]
        if isinstance(n, str) and n.isdigit():
            return int(n)
        return n
    return t


_SELF = Filter(base="self", targeted=False)


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — "as ~ enters" (comp rules §603.6f)
# These are technically replacement effects on self-ETB. Modeling them as
# Triggered nodes with event="etb_as" keeps them in the triggered pipeline
# so the downstream effect body gets parsed.  The parser's existing
# _TRIGGER_PATTERNS don't handle "as ~ enters" at all, so we list them here
# as an ADD to the trigger table (integration: prepend to _TRIGGER_PATTERNS).
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # "as ~ enters the battlefield, ..." / "as ~ enters, ..."
    (re.compile(r"^as (?:~|this creature|this permanent|this artifact|"
                r"this enchantment|this land) enters(?: the battlefield)?", re.I),
     "etb_as", "self"),
]


# ---------------------------------------------------------------------------
# EFFECT_RULES — body effects that appear inside "as ~ enters, <X>"
# These are consulted only for the text following the trigger comma, e.g.
# "choose a creature type" or "name a card". Each returns an Effect.
#
# Integration: parse_effect() should consult these BEFORE falling through.
# They're written as (compiled_regex, builder) tuples so parser.rule() isn't
# required.
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


@_eff(r"^choose a (creature|land|card|artifact|planeswalker|basic land|enchantment|instant|sorcery) type"
      r"(?: other than [^.]+)?(?:\.|$)")
def _choose_type(m):
    return UnknownEffect(raw_text=f"etb_choose_type:{m.group(1)}")


@_eff(r"^choose a color(?: other than [^.]+)?(?:\.|$)")
def _choose_color(m):
    return UnknownEffect(raw_text="etb_choose_color")


@_eff(r"^choose a basic land type(?: at random)?(?:\.|$)")
def _choose_basic(m):
    return UnknownEffect(raw_text="etb_choose_basic_land_type")


@_eff(r"^choose a player(?:\.|$)")
def _choose_player(m):
    return UnknownEffect(raw_text="etb_choose_player")


@_eff(r"^choose left or right(?:\.|$)")
def _choose_dir(m):
    return UnknownEffect(raw_text="etb_choose_direction")


@_eff(r"^choose a number(?: between \d+ and \d+)?(?:\.|$)")
def _choose_num(m):
    return UnknownEffect(raw_text="etb_choose_number")


@_eff(r"^name a (?:nonland )?card(?:\.|$)")
def _name_card(m):
    return UnknownEffect(raw_text="etb_name_card")


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — ETB-with-counters, may-copy, tapped-unless, etc.
# Each entry is (compiled_regex, builder). The builder signature is
# (match, original_text) → Static | Triggered | None.
#
# Integration: parse_static() should consult these BEFORE the generic
# "as long as / if" conditional_static fallback but AFTER specific shapes.
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _stat(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# -- "you may have ~ enter as a copy of <filter>" -----------------------------

@_stat(r"^you may have (?:~|this (?:creature|land|artifact|enchantment|"
       r"permanent|equipment|vehicle)) enter(?:s)?"
       r"(?:\s+tapped)?\s+as a (?:~|copy) of ([^.]+?)(?:,\s*except [^.]+)?(?:\.|$)")
def _may_copy(m, raw):
    source_desc = m.group(1).strip()
    return Static(
        modification=Modification(kind="etb_may_copy",
                                  args=(source_desc,)),
        raw=raw,
    )


# -- "~ enters with X/N <kind> counter(s) on it [for each ...] [if ...] [,where x is ...]" -

@_stat(r"^(?:~|this (?:creature|artifact|enchantment|permanent|land)) "
       r"enters with "
       r"(a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) "
       r"([a-z+/0-9\- ]+?) counters? on it"
       r"(?:\s*,?\s*where x is ([^.]+?))?"
       r"(?:\s+for each ([^.]+?))?"
       r"(?:\s+if ([^.]+?))?"
       r"(?:\.|$)")
def _etb_with_counters(m, raw):
    count = _num(m.group(1))
    kind = m.group(2).strip()
    where_x = m.group(3)
    for_each = m.group(4)
    if_cond = m.group(5)

    target = _SELF
    mod_args: tuple = (count, kind)
    cond: Optional[Condition] = None

    if where_x is not None:
        count = "x"
        mod_args = ("x", kind, "where_x:" + where_x.strip())
    if for_each is not None:
        count = "var"
        mod_args = ("var", kind, "for_each:" + for_each.strip())
    if if_cond is not None:
        cond = Condition(kind="etb_if", args=(if_cond.strip(),))

    return Static(
        condition=cond,
        modification=Modification(kind="etb_with_counters", args=mod_args),
        raw=raw,
    )


# -- "~ enters with a number of +1/+1 counters on it equal to <expr>" ----------

@_stat(r"^(?:~|this (?:creature|artifact|permanent)) enters with "
       r"a number of ([a-z+/0-9\- ]+?) counters? on it equal to ([^.]+?)(?:\.|$)")
def _etb_counters_equal(m, raw):
    kind = m.group(1).strip()
    expr = m.group(2).strip()
    return Static(
        modification=Modification(
            kind="etb_with_counters",
            args=("var", kind, "equal_to:" + expr),
        ),
        raw=raw,
    )


# -- "~ enters tapped unless <cond>" -------------------------------------------

@_stat(r"^(?:~|this (?:creature|land|artifact|permanent)) enters tapped unless "
       r"([^.]+?)(?:\.|$)")
def _etb_tapped_unless(m, raw):
    cond = m.group(1).strip()
    return Static(
        condition=Condition(kind="etb_tapped_unless", args=(cond,)),
        modification=Modification(kind="etb_tapped_unless", args=(cond,)),
        raw=raw,
    )


# -- "~ enters tapped and doesn't untap during your next untap step" -----------

@_stat(r"^(?:~|this (?:creature|land|artifact|permanent)) enters tapped "
       r"and doesn'?t untap during (?:your|its controller'?s?) next untap step"
       r"(?:\.|$)")
def _etb_tapped_no_untap(m, raw):
    return Static(
        modification=Modification(kind="etb_tapped_and_no_untap"),
        raw=raw,
    )


# -- "~ enters tapped and attacking" -------------------------------------------

@_stat(r"^(?:~|this (?:creature|permanent)) enters tapped and attacking(?:\.|$)")
def _etb_tapped_attacking(m, raw):
    return Static(
        modification=Modification(kind="etb_tapped_and_attacking"),
        raw=raw,
    )


# -- "when ~ enters, if <cond>, <effect>" --------------------------------------
# Modeled as a Triggered node whose effect is a Conditional wrapping an
# UnknownEffect (body text left raw; downstream parse_effect can enrich).

@_stat(r"^when (?:~|this creature|this permanent) enters(?: the battlefield)?, "
       r"if ([^,]+), ([^.]+?)(?:\.|$)")
def _etb_intervening_if(m, raw):
    cond_text = m.group(1).strip()
    body_text = m.group(2).strip()
    return Triggered(
        trigger=Trigger(event="etb"),
        intervening_if=Condition(kind="intervening_if", args=(cond_text,)),
        effect=Conditional(
            condition=Condition(kind="intervening_if", args=(cond_text,)),
            body=UnknownEffect(raw_text=body_text),
        ),
        raw=raw,
    )


# -- "if <cond> when ~ enters, <effect>" (flipped intervening-if) --------------

@_stat(r"^if ([^,]+?) when (?:~|this creature|this permanent) enters"
       r"(?: the battlefield)?, ([^.]+?)(?:\.|$)")
def _etb_intervening_if_flipped(m, raw):
    cond_text = m.group(1).strip()
    body_text = m.group(2).strip()
    return Triggered(
        trigger=Trigger(event="etb"),
        intervening_if=Condition(kind="intervening_if", args=(cond_text,)),
        effect=Conditional(
            condition=Condition(kind="intervening_if", args=(cond_text,)),
            body=UnknownEffect(raw_text=body_text),
        ),
        raw=raw,
    )


# -- "when ~ enters, you may <effect>" — optional ETB --------------------------
# We still emit a Triggered node, but wrap the body in a Replacement-less
# Conditional to mark optionality. Real wiring should use Optional_ wrapping.

@_stat(r"^when (?:~|this creature|this permanent|this artifact|this enchantment) "
       r"enters(?: the battlefield)?, you may ([^.]+?)(?:\.|$)")
def _etb_may(m, raw):
    body_text = m.group(1).strip()
    return Triggered(
        trigger=Trigger(event="etb"),
        effect=Conditional(
            condition=Condition(kind="may_choose", args=()),
            body=UnknownEffect(raw_text=body_text),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Public entry point for a unified try-static dispatcher (mirrors
# ability_words.try_static_extensions pattern).
# ---------------------------------------------------------------------------

def try_static_extensions(text: str):
    """Run STATIC_PATTERNS against `text`. Returns the first matching node
    (Static or Triggered) or None."""
    s = text.strip().rstrip(".").lower()
    if not s:
        return None
    for pat, builder in STATIC_PATTERNS:
        m = pat.match(s)
        if not m:
            continue
        try:
            out = builder(m, text)
        except Exception:
            continue
        if out is not None:
            return out
    return None


def try_trigger_extensions(text: str):
    """Match "as ~ enters, <body>". Returns (Trigger, event_name, rest_text)
    for the caller (parser.parse_triggered) to then parse the body. Returns
    None if no pattern matched."""
    s = text.strip().rstrip(".").lower()
    if not s:
        return None
    for pat, event, scope in TRIGGER_PATTERNS:
        m = pat.match(s)
        if not m:
            continue
        rest = s[m.end():].lstrip(" ,")
        return (Trigger(event=event), rest)
    return None


__all__ = [
    "EFFECT_RULES",
    "STATIC_PATTERNS",
    "TRIGGER_PATTERNS",
    "try_static_extensions",
    "try_trigger_extensions",
]
