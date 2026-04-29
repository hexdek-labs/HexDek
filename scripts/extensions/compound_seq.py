#!/usr/bin/env python3
"""Compound conditional sequencers.

This module owns multi-clause oracle text whose semantics chain through
"do A. if you did/do, B. if you don't/otherwise, C. then D." constructions.
Each of those connectives binds the resolution of one clause to the outcome
of an earlier clause, so the right AST shape is a `Conditional` (or stack of
them) whose `condition` is a back-reference to the prior step's success.

Scope (real cards in parens):

    Optional-pay chains (very common — ~1.4K cards):
        you may discard a card. if you do, draw two cards.            (Witch's Mark)
        you may pay {2}. if you do, [effect].                          (Minion Reflector)
        you may sacrifice ~. if you do, [effect].                      (Awaken the Sky Tyrant)

    Optional-pay with both branches:
        you may pay {3}. if you do, A. if you don't, B.                (Reckoner Shakedown)
        you may cast it. if you don't, create a treasure token.        (Vaan, Street Thief)

    Then-chains (sequence with a state check):
        do A. then if [cond], do B.                                    (Yorvo, Lord of Garenbrig)
        do A. then [unconditional B].                                  (Brainstorm, Gluntch)

    Conditional/otherwise dispatch:
        if [cond], do A. otherwise, do B.                              (Search for Survivors)
        do A if [cond]. otherwise, do B.                               (Zurgo Stormrender)

    Reveal-then-categorize (explore-style):
        reveal the top card. put it into [zone] if [type-cond].
        otherwise, [other effect].                                     (River Herald Scout / explore)

    Repeat loops:
        repeat this process any number of times.                       (Forbidden Ritual)
        do this N times.                                               (Ire of Kaminari, Crackle with Power)

    For-each scaling (loop over a set producing per-iteration effect):
        for each X, [effect].                                          (Pyretic Charge, Pir's Whim)
        for each card discarded this way, [effect].                    (Seasoned Pyromancer)

    Multikicker tail:
        you may pay {cost} any number of times.                        (Spell Contortion, Everflowing Chalice)
        for each time you paid, [effect].

    "When you do" sub-trigger (resolves after a player completes a may-action):
        when you do, [effect].                                         (Valentin / Lisette)

The split_abilities() pass in parser.py already glues "if you do/don't",
"otherwise", and "then" continuations onto the previous sentence, so by the
time we see the text it's a single combined clause. Rather than decomposing
the inner clauses (which would require recursing into parse_effect from a
foreign module — fragile), we capture the SHAPE as a `Conditional` AST node
whose `body` and `else_body` carry `UnknownEffect(raw_text=...)` placeholders.
The structural fingerprint then signals the conditional shape correctly,
which is what semantic clustering and the AST equivalence helpers care about.

Two conventions:

  * `EFFECT_RULES` — for spell-effect bodies (instants/sorceries) and the
    bodies of triggered abilities. parse_effect() iterates these alongside
    the in-file rules; the FIRST whose regex consumes (almost) the whole
    text wins.

  * `STATIC_PATTERNS` — for tail riders that survive splitting as their own
    "ability" line, e.g. orphan "for each ..." or "when you do, ..." clauses
    that the parser would otherwise leave UNPARSED.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Condition, Conditional, Effect, Filter, Modification, Optional_,
    Sequence, Static, Trigger, Triggered, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _u(text: str) -> Effect:
    """Wrap a clause as a typed effect if possible, else UnknownEffect.

    Originally this always returned an UnknownEffect placeholder, on the
    assumption that downstream tools would re-resolve the raw text later. With
    the conjunction trial-split in ``parser.parse_effect``, though, any clause
    that survives here after the may/if/otherwise shape has been extracted is
    typically a complete, self-contained effect body (e.g. "draw a card",
    "this creature gets +1/+1 and gains trample until end of turn"). Calling
    parse_effect lazily lets those bodies land as typed nodes instead of
    opaque UnknownEffect blobs — which is exactly what downstream dispatch
    expects. Falls back to UnknownEffect when parse_effect declines (so the
    original placeholder-shape guarantee is preserved for unrecognised text).
    """
    clean = text.strip().rstrip(".").strip()
    if not clean:
        return UnknownEffect(raw_text="")
    try:
        # Lazy import — avoids the circular parser ↔ extensions bootstrap.
        from parser import parse_effect  # type: ignore
        parsed = parse_effect(clean)
    except Exception:
        parsed = None
    if parsed is not None and not isinstance(parsed, UnknownEffect):
        return parsed
    # parse_effect may itself return an UnknownEffect wrapper; keep the raw text
    # verbatim so existing cluster/signature machinery behaves unchanged.
    return UnknownEffect(raw_text=clean)


def _seq(*items: Effect) -> Effect:
    """Sequence helper that flattens singletons."""
    flat = tuple(i for i in items if i is not None)
    if not flat:
        return _u("")
    if len(flat) == 1:
        return flat[0]
    return Sequence(items=flat)


def _cond(kind: str, *args) -> Condition:
    return Condition(kind=kind, args=tuple(args))


# ---------------------------------------------------------------------------
# EFFECT_RULES — spell/triggered-effect bodies
# ---------------------------------------------------------------------------
# Each entry: (compiled_regex, builder(match) -> Effect).  parse_effect()
# accepts a match only if the regex consumes ~all of the text, so we anchor
# with `^` and end with optional period.

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# -- "you may [pay/discard/sacrifice/...]. if you do, A. if you don't, B." ----
#
# Three-clause branching optional. We bind both branches.
@_eff(
    r"^you may ([^.]+?)\.\s*"
    r"if you do,?\s+([^.]+?)\.\s*"
    r"if you don'?t,?\s+([^.]+?)"
    r"\.?\s*$"
)
def _may_if_do_else(m):
    cost_clause, do_clause, else_clause = m.group(1), m.group(2), m.group(3)
    return Conditional(
        condition=_cond("paid_optional_cost", cost_clause.strip()),
        body=_seq(_u(do_clause)),
        else_body=_seq(_u(else_clause)),
    )


# -- "you may [X]. if you do, A." (no else branch) ---------------------------
@_eff(
    r"^you may ([^.]+?)\.\s*"
    r"if you do,?\s+([^.]+?)"
    r"\.?\s*$"
)
def _may_if_do(m):
    cost_clause, do_clause = m.group(1), m.group(2)
    body = _u(do_clause)
    inner = Conditional(
        condition=_cond("paid_optional_cost", cost_clause.strip()),
        body=body,
    )
    # The whole effect IS the optional + payoff; wrap in Optional_ so callers
    # can see "the entire effect is opt-in."
    return Optional_(body=inner)


# -- "you may [X]. if you don't, B." ------------------------------------------
# Vaan-style: optional with an else-only payoff.
@_eff(
    r"^you may ([^.]+?)\.\s*"
    r"if you don'?t,?\s+([^.]+?)"
    r"\.?\s*$"
)
def _may_if_dont(m):
    cost_clause, else_clause = m.group(1), m.group(2)
    return Conditional(
        condition=_cond("paid_optional_cost", cost_clause.strip()),
        body=_u(cost_clause),         # body = the may-action itself
        else_body=_u(else_clause),
    )


# -- "do A. if you do, B." (non-may form — A is itself a cost-action like
#    "sacrifice ~", "discard a card", "exile target card") --------------------
@_eff(
    r"^([^.]+?)\.\s*if you do,?\s+([^.]+?)\.?\s*$"
)
def _do_if_do(m):
    head, payoff = m.group(1), m.group(2)
    # Reject if head is itself a "you may ..." (handled above with higher
    # specificity) — guard prevents double-match.
    if re.match(r"^you may\b", head.strip(), re.I):
        return None
    return Sequence(items=(
        _u(head),
        Conditional(
            condition=_cond("did_prior_action"),
            body=_u(payoff),
        ),
    ))


# -- "if [cond], A. otherwise, B." -------------------------------------------
@_eff(
    r"^if ([^,.]+?),\s+([^.]+?)\.\s*otherwise,?\s+([^.]+?)\.?\s*$"
)
def _if_otherwise(m):
    cond_text, then_clause, else_clause = m.group(1), m.group(2), m.group(3)
    return Conditional(
        condition=_cond("if", cond_text.strip()),
        body=_u(then_clause),
        else_body=_u(else_clause),
    )


# -- "do A if [cond]. otherwise, B." (clause-trailing-if dispatch) ------------
@_eff(
    r"^([^.]+?)\s+if ([^.]+?)\.\s*otherwise,?\s+([^.]+?)\.?\s*$"
)
def _do_if_else(m):
    body_clause, cond_text, else_clause = m.group(1), m.group(2), m.group(3)
    # Avoid swallowing "you may pay X if you do" forms (handled above).
    if re.search(r"\byou\s+(?:do|don'?t)\b", cond_text, re.I):
        return None
    return Conditional(
        condition=_cond("if", cond_text.strip()),
        body=_u(body_clause),
        else_body=_u(else_clause),
    )


# -- "do A. then if [cond], B." (Yorvo-style state-check chain) --------------
@_eff(
    r"^([^.]+?)\.\s*then if ([^,]+),\s+([^.]+?)\.?\s*$"
)
def _then_if(m):
    head, cond_text, tail = m.group(1), m.group(2), m.group(3)
    return Sequence(items=(
        _u(head),
        Conditional(
            condition=_cond("if", cond_text.strip()),
            body=_u(tail),
        ),
    ))


# -- "do A. then B." (unconditional sequence with an explicit "then" join) ---
# Only fires if neither side looks like an "if/otherwise/may" construct so
# the more specific rules above get first shot.
@_eff(
    r"^([^.]+?)\.\s*then ([^.]+?)\.?\s*$"
)
def _then_chain(m):
    head, tail = m.group(1), m.group(2)
    if re.match(r"^if\b", tail.strip(), re.I):
        return None
    if re.search(r"\b(?:if you do|if you don'?t|otherwise|may)\b", head + " " + tail, re.I):
        # Defer to the parse_effect splitters (which can recursively handle
        # "may" via the dedicated may rules) by returning None. With the
        # parse_effect None-fall-through fix in place, this no longer aborts
        # the whole call.
        # As a last-resort safety net, if the splitter wouldn't catch this
        # (e.g. no whitespace after the period), provide a coarse Sequence
        # of placeholders so the card lands as PARTIAL rather than UNPARSED.
        if not re.search(r"\.\s+then\s+", m.string, re.I):
            return Sequence(items=(_u(head), _u(tail)))
        return None
    return Sequence(items=(_u(head), _u(tail)))


# -- "do A N times." / "repeat this process N times" -------------------------
_TIMES_NUM = {
    "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
    "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}


@_eff(
    r"^([^.]+?)\s+(two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+times"
    r"\.?\s*$"
)
def _do_n_times(m):
    body_clause, n_token = m.group(1), m.group(2)
    n = _TIMES_NUM.get(n_token.lower(), int(n_token) if n_token.isdigit() else n_token)
    return Conditional(
        condition=_cond("repeat_n", n),
        body=_u(body_clause),
    )


# -- "you may repeat this process any number of times." ----------------------
@_eff(
    r"^you may repeat this process any number of times\.?\s*$"
)
def _may_repeat_any(m):
    return Conditional(
        condition=_cond("repeat_any_optional"),
        body=_u("repeat process"),
    )


# -- "for each X, [effect]." (generic per-X loop) ----------------------------
# Anchors only when "for each" is at clause start AND the body is a complete
# sub-effect (period-ended). Existing in-file `for each`-as-modifier suffixes
# are preserved by the early-anchor.
@_eff(
    r"^for each ([^,]+?),\s+([^.]+?)\.?\s*$"
)
def _for_each(m):
    set_clause, body_clause = m.group(1), m.group(2)
    return Conditional(
        condition=_cond("for_each", set_clause.strip()),
        body=_u(body_clause),
    )


# -- "[player] reveals N cards. if [type] is among them, A. then put the rest
#    [destination]." (Mulldrifter-style reveal-categorize) ------------------
@_eff(
    r"^([^.]+?reveals? [^.]+?)\.\s*"
    r"if (?:a |an |any )?([^.]+?) (?:is|are) among them,?\s+([^.]+?)\."
    r"\s*(?:then )?put the rest ([^.]+?)\.?\s*$"
)
def _reveal_among_rest(m):
    reveal_clause = m.group(1)
    among_filter = m.group(2)
    payoff = m.group(3)
    rest_dest = m.group(4)
    return Sequence(items=(
        _u(reveal_clause),
        Conditional(
            condition=_cond("reveal_includes", among_filter.strip()),
            body=_u(payoff),
        ),
        _u("put the rest " + rest_dest),
    ))


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — orphan tail riders that survive split_abilities() as
# their own line and would otherwise be UNPARSED.
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _stat(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# -- "when you do, <effect>." — sub-trigger that fires after the controller
# completes a prior may-action (Valentin / Lisette / Primal Adversary).
@_stat(
    r"^when you do,\s+([^.]+?)\.?\s*$"
)
def _when_you_do(m, raw):
    body = m.group(1).strip()
    return Triggered(
        trigger=Trigger(event="when_you_do"),
        effect=_u(body),
        raw=raw,
    )


# -- "for each time you [paid|did] ...,  <effect>." — Multikicker tail -------
@_stat(
    r"^for each time you (paid [^,.]+|did [^,.]+|kicked [^,.]+|cast [^,.]+),"
    r"\s+([^.]+?)\.?\s*$"
)
def _for_each_time_you(m, raw):
    pred = m.group(1).strip()
    body = m.group(2).strip()
    return Static(
        condition=_cond("for_each_time_you", pred),
        modification=Modification(
            kind="for_each_time_payoff",
            args=(pred, body),
        ),
        raw=raw,
    )


# -- "then if [cond], <effect>." — orphan trailing then-if rider -------------
# (Verifies the existing partial-coverage assumption: such trailing clauses
# can survive splitting when the prior sentence terminated cleanly.)
@_stat(
    r"^then if ([^,]+),\s+([^.]+?)\.?\s*$"
)
def _trailing_then_if(m, raw):
    cond_text = m.group(1).strip()
    body = m.group(2).strip()
    return Static(
        condition=_cond("then_if", cond_text),
        modification=Modification(
            kind="then_if_rider",
            args=(cond_text, body),
        ),
        raw=raw,
    )


# -- "otherwise, <effect>." — orphan else-branch rider -----------------------
@_stat(
    r"^otherwise,\s+([^.]+?)\.?\s*$"
)
def _trailing_otherwise(m, raw):
    body = m.group(1).strip()
    return Static(
        condition=_cond("otherwise"),
        modification=Modification(
            kind="otherwise_rider",
            args=(body,),
        ),
        raw=raw,
    )


# -- "if you do, <effect>." — orphan true-branch rider -----------------------
@_stat(
    r"^if you do,\s+([^.]+?)\.?\s*$"
)
def _trailing_if_you_do(m, raw):
    body = m.group(1).strip()
    return Static(
        condition=_cond("if_you_did"),
        modification=Modification(
            kind="if_you_do_rider",
            args=(body,),
        ),
        raw=raw,
    )


# -- "if you don't, <effect>." — orphan false-branch rider -------------------
@_stat(
    r"^if you don'?t,\s+([^.]+?)\.?\s*$"
)
def _trailing_if_you_dont(m, raw):
    body = m.group(1).strip()
    return Static(
        condition=_cond("if_you_didnt"),
        modification=Modification(
            kind="if_you_dont_rider",
            args=(body,),
        ),
        raw=raw,
    )


# -- "for each <set>, <effect>." — orphan per-X rider ------------------------
@_stat(
    r"^for each ([^,]+?),\s+([^.]+?)\.?\s*$"
)
def _trailing_for_each(m, raw):
    set_clause = m.group(1).strip()
    body = m.group(2).strip()
    return Static(
        condition=_cond("for_each", set_clause),
        modification=Modification(
            kind="for_each_rider",
            args=(set_clause, body),
        ),
        raw=raw,
    )


# -- "you may pay {cost} any number of times." — Multikicker-self rider ------
@_stat(
    r"^you may pay (\{[^}]+\}(?:\{[^}]+\})*) any number of times"
    r"(?: as you cast this spell)?\.?\s*$"
)
def _multikicker(m, raw):
    cost_text = m.group(1)
    return Static(
        modification=Modification(
            kind="multikicker_pay_any",
            args=(cost_text,),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Public entry point (mirrors conditional_etb.try_static_extensions).
# ---------------------------------------------------------------------------

def try_static_extensions(text: str):
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


__all__ = [
    "EFFECT_RULES",
    "STATIC_PATTERNS",
    "try_static_extensions",
]
