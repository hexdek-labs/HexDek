#!/usr/bin/env python3
"""Villainous-choice cascades and forced-opponent choice trees.

This module owns the **VILLAINOUS CHOICE CASCADES** family: oracle-text
clauses where a (usually opposing) player is forced to pick one of 2-3
self-contained sub-effects, each with its own internal compound body.

Comp rules §701 introduces "villainous choice" as a templated keyword
action: the affected player chooses exactly one of the listed options and
that option's effect resolves. Functionally it's a `Choice(pick=1)` AST
node whose options are themselves arbitrary effects, with the wrinkle
that the *chooser* is an opponent rather than the controller.

Scope (real cards in parens):

    Per-opponent villainous choice (group-slug):
        each opponent faces a villainous choice - <A>, or <B>.
            (Ensnared by the Mara, Sycorax Commander, Davros / Dalek Creator,
             Missy, Dr. Eggman, The Dalek Emperor)

    Targeted villainous choice (single-target):
        target opponent faces a villainous choice - <A>, or <B>.
            (Great Intelligence's Plan, Genesis of the Daleks ch. IV)

    Pronoun-back-referenced villainous choice (post-prior-clause):
        ..., then faces a villainous choice - <A>, or <B>.
        ..., that creature's controller faces a villainous choice - <A>,
            or <B>.
        ..., that player faces a villainous choice - <A>, or <B>.
            (This Is How It Ends, Hunted by The Family,
             The Master Gallifrey's End, Midnight Crusader Shuttle,
             defending player on attack triggers)

    Generic "chooses one - bullet body" forms (legacy templating that
    Wizards still occasionally uses):
        target opponent chooses one - <A>; or <B>.
            (rare; covered as a fallback so existing modal cards don't
             collide with our regex.)

The whole text is wrapped as a `Static(modification=Modification(
kind='villainous_choice', args=(chooser, ChoiceAST))` carrying the
embedded `Choice(options=(branch_a, branch_b), pick=1)` so downstream
clustering can treat villainous choice as its own structural family
while still introspecting the branch effects.

This is a *pattern-extension* module — it does not import from parser.py
at module-load time (only lazily inside builders). Integration is
automatic via `parser.load_extensions()`.

Notes on input shape (post-`normalize()` in parser.py):

  * em-dash "—" has already been folded to "-".
  * text is lowercased.
  * card name has been replaced with "~".
  * `split_abilities()` will NOT cut the body further because villainous
    choice clauses contain no internal sentence-final period until the
    end (the `, or` join is a soft conjunction, not a sentence break).

Output AST shape:

    Static(
      modification=Modification(
        kind='villainous_choice',
        args=(chooser_token, Choice(
          options=(branch_a_effect, branch_b_effect),
          pick=1,
        )),
      ),
      raw=<full clause>,
    )

Where `chooser_token` is one of: 'each_opponent', 'target_opponent',
'defending_player', 'that_player', 'that_creature_controller',
'that_creature_owner', 'pronoun_chain'.
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
    Choice, Effect, Modification, Static, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _u(text: str) -> UnknownEffect:
    """Wrap a raw branch as an UnknownEffect placeholder when parse_effect
    declines (which is common for villainous-choice branches because they
    routinely contain compound 'then'/'and' sequences that aren't a tight
    structural fit for any single rule)."""
    return UnknownEffect(raw_text=text.strip().rstrip(".").strip())


def _parse_branch(text: str) -> Effect:
    """Try to parse a branch via the full parser, falling back to
    UnknownEffect. Lazy-imports parser to dodge circular-import."""
    body = text.strip().rstrip(",.;").strip()
    if not body:
        return _u("")
    try:
        from parser import parse_effect  # lazy
    except Exception:
        return _u(body)
    parsed = None
    try:
        parsed = parse_effect(body)
    except Exception:
        parsed = None
    return parsed if parsed is not None else _u(body)


# Pronoun starters that reliably begin a villainous-choice branch B.
# Used to find the rightmost ", or <starter>" split point in a text that
# may itself have intra-branch ", or" tokens (e.g. compound sub-clauses).
_BRANCH_B_STARTERS = (
    r"they|that player|that opponent|that creature'?s? (?:controller|owner)|"
    r"that creature|each (?:of your )?opponents?|the chosen (?:player|opponent)|"
    r"you may|you create|you draw|you|"
    r"~|this creature|this vehicle|this artifact|"
    r"each artifact|each creature"
)
_BRANCH_B_RE = re.compile(rf",\s+or\s+(?={_BRANCH_B_STARTERS}\b)", re.I)

# Three-branch (rare; bullet-templated): "- • A • B • C" or
# "- A; or B; or C" form — the post-normalize bullet glyph survives as
# • or · because normalize() only folds dashes, not bullets.
_BULLET_SPLIT_RE = re.compile(r"\s*[•·]\s*")


def _split_branches(body: str) -> list[str]:
    """Split a villainous-choice body into its option branches.

    Strategy:
      1. If the body uses bullets (• / ·), split on those.
      2. Otherwise prefer the *rightmost* ", or <pronoun-starter>" split
         so intra-branch ", or"s in branch A don't get eaten.
      3. As a last resort, split on the first ", or ".
    """
    s = body.strip()
    if "•" in s or "·" in s:
        parts = [p.strip(" ;.,") for p in _BULLET_SPLIT_RE.split(s) if p.strip(" ;.,")]
        if len(parts) >= 2:
            return parts

    # Find every ", or <starter>" boundary; keep the rightmost one (so
    # earlier intra-branch ", or"s stay inside branch A).
    matches = list(_BRANCH_B_RE.finditer(s))
    if matches:
        cut = matches[-1]
        return [s[:cut.start()].strip(" ;.,"),
                s[cut.end():].strip(" ;.,")]

    # Fallback: any ", or " (less precise — last resort).
    fallback = list(re.finditer(r",\s+or\s+", s, re.I))
    if fallback:
        cut = fallback[-1]
        return [s[:cut.start()].strip(" ;.,"),
                s[cut.end():].strip(" ;.,")]

    # Nothing to split — return as a single (degenerate) branch.
    return [s]


def _build_choice(chooser: str, body: str, raw: str) -> Static:
    branches = _split_branches(body)
    options = tuple(_parse_branch(b) for b in branches if b)
    if not options:
        options = (_u(body),)
    choice = Choice(options=options, pick=1)
    return Static(
        modification=Modification(
            kind="villainous_choice",
            args=(chooser, choice),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# EFFECT_RULES — full-clause spell-effect bodies
# ---------------------------------------------------------------------------
# Triggered-ability bodies (e.g. "at the beginning of your end step, each
# opponent faces a villainous choice - ...") arrive here AFTER the trigger
# header has been stripped by parse_triggered, so we just need to match the
# body. Spell texts (Ensnared by the Mara, Great Intelligence's Plan, This
# Is How It Ends) arrive as the entire ability.

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "each opponent faces a villainous choice - <body>" ----------------------
@_eff(
    r"^each (?:of your )?opponents? faces a villainous choice\s*[-:]\s*"
    r"(.+?)\.?\s*$"
)
def _each_opp_villainous(m):
    body = m.group(1)
    return _build_choice("each_opponent", body, m.group(0))


# --- "target opponent faces a villainous choice - <body>" --------------------
@_eff(
    r"^target opponent faces a villainous choice\s*[-:]\s*"
    r"(.+?)\.?\s*$"
)
def _target_opp_villainous(m):
    body = m.group(1)
    return _build_choice("target_opponent", body, m.group(0))


# --- "<pronoun-chain> faces a villainous choice - <body>" --------------------
# Catches: "they faces ...", "that player faces ...",
# "that creature's owner faces ...", "defending player faces ...",
# "that creature's controller faces ...", "the chosen player faces ...",
# and variants like "each opponent who lost 3 or more life this turn".
@_eff(
    r"^("
    r"they|that player|that opponent|defending player|"
    r"that creature'?s? (?:controller|owner)|"
    r"the chosen (?:player|opponent)|"
    r"each opponent who [^.]+?|"
    r"each player who [^.]+?"
    r") faces a villainous choice\s*[-:]\s*"
    r"(.+?)\.?\s*$"
)
def _pronoun_villainous(m):
    chooser_text = m.group(1).strip()
    body = m.group(2)
    chooser = _normalize_chooser(chooser_text)
    return _build_choice(chooser, body, m.group(0))


# --- "<head clause>. then <chooser> faces a villainous choice - <body>" -----
# Sequence-join variant: an unconditional head effect, then a fresh
# villainous-choice clause keyed on a different (or same) chooser.
# Triggers like "draw a card. then each opponent faces a villainous choice
# - ..." (Dr. Eggman) and "draw three cards. then target opponent faces
# ..." (Great Intelligence's Plan) land here.
@_eff(
    r"^(.+?)\.\s*then ("
    r"each (?:of your )?opponents?|target opponent|that player|"
    r"defending player|the chosen (?:player|opponent)|"
    r"each opponent who [^.]+?|each player who [^.]+?"
    r") faces a villainous choice\s*[-:]\s*"
    r"(.+?)\.?\s*$"
)
def _then_chooser_villainous(m):
    head = m.group(1).strip()
    chooser_text = m.group(2).strip()
    body = m.group(3)
    head_effect = _parse_branch(head)
    chooser = _normalize_chooser(chooser_text) if chooser_text not in {
        "target opponent", "each opponent", "each of your opponents",
        "each opponents",
    } else (
        "target_opponent" if chooser_text == "target opponent" else "each_opponent"
    )
    base = _build_choice(chooser, body, m.group(0))
    return Static(
        modification=Modification(
            kind="villainous_choice_after",
            args=(head_effect, base.modification),
        ),
        raw=m.group(0),
    )


# --- "<head clause>, then faces a villainous choice - <body>" ---------------
# Pronoun-back-reference variant where the trigger sentence first does
# something (e.g. "target creature's owner shuffles it into their library")
# and THEN attaches a villainous choice that the same actor faces.
@_eff(
    r"^(.+?),\s*then faces a villainous choice\s*[-:]\s*"
    r"(.+?)\.?\s*$"
)
def _then_faces_villainous(m):
    head = m.group(1).strip()
    body = m.group(2)
    head_effect = _parse_branch(head)
    choice_static = _build_choice("pronoun_chain", body, m.group(0))
    # Encode as a single Static carrying both the head and the choice via
    # the Modification args so downstream code keeps the structural shape.
    return Static(
        modification=Modification(
            kind="villainous_choice_after",
            args=(head_effect, choice_static.modification),
        ),
        raw=m.group(0),
    )


# --- "for each of them, <controller-phrase> faces a villainous choice ..." ---
# Hunted by The Family templating: a per-target loop where each target's
# controller faces a villainous choice. We emit a per-iteration Choice and
# tag it with kind='villainous_choice_for_each' so the loop semantics stay
# inspectable.
@_eff(
    r"^for each of them,\s+(.+?) faces a villainous choice\s*[-:]\s*"
    r"(.+?)\.?\s*$"
)
def _for_each_faces_villainous(m):
    chooser_text = m.group(1).strip()
    body = m.group(2)
    chooser = _normalize_chooser(chooser_text)
    base = _build_choice(chooser, body, m.group(0))
    return Static(
        modification=Modification(
            kind="villainous_choice_for_each",
            args=(chooser, base.modification.args[1]),
        ),
        raw=m.group(0),
    )


# --- Generic "target opponent chooses one - • A • B" (legacy templating) -----
# Some pre-villainous cards used "chooses one -" with bulleted bodies. We
# only handle the OPPONENT-as-chooser case here so we don't collide with
# the controller-side modal "choose one -" rule already in parser.py.
@_eff(
    r"^(?:target opponent|each opponent|that player|defending player) "
    r"chooses one\s*[-:]\s*(.+?)\.?\s*$"
)
def _opp_chooses_one_modal(m):
    body = m.group(1)
    # Force bullet-or-semicolon split so we don't accidentally swallow an
    # intra-branch ", or".
    if "•" in body or "·" in body:
        parts = [p.strip(" ;.,") for p in _BULLET_SPLIT_RE.split(body) if p.strip(" ;.,")]
    else:
        parts = [p.strip(" ;.,") for p in re.split(r";\s*", body) if p.strip(" ;.,")]
    if len(parts) < 2:
        # Fall through to villainous-style branching as a safety net.
        parts = _split_branches(body)
    options = tuple(_parse_branch(p) for p in parts if p)
    if not options:
        return None
    choice = Choice(options=options, pick=1)
    return Static(
        modification=Modification(
            kind="opponent_modal_choice",
            args=("opponent", choice),
        ),
        raw=m.group(0),
    )


# ---------------------------------------------------------------------------
# Chooser normalization
# ---------------------------------------------------------------------------

def _normalize_chooser(text: str) -> str:
    """Map the natural-language chooser phrase to a stable token."""
    t = text.strip().lower()
    if t in {"they", "that player", "that opponent"}:
        return "that_player"
    if t == "defending player":
        return "defending_player"
    if t.startswith("that creature"):
        if "controller" in t:
            return "that_creature_controller"
        if "owner" in t:
            return "that_creature_owner"
        return "that_creature_controller"
    if t.startswith("the chosen"):
        return "chosen_player"
    if t.startswith("each opponent who"):
        return "each_opponent_filtered"
    if t.startswith("each player who"):
        return "each_player_filtered"
    return "pronoun_chain"


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — orphan villainous-choice clauses that survive splitting
# (e.g. saga chapter bodies like "iv - target opponent faces a villainous
# choice - ...", or trailing clauses that couldn't be claimed as effects).
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _stat(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# Saga chapter prefix: "iv - target opponent faces a villainous choice - ..."
# Genesis of the Daleks templating. The saga extension may already strip
# the prefix; this is a belt-and-suspenders catch.
@_stat(
    r"^[ivx]+(?:\s*,\s*[ivx]+)*\s*[-]\s*"
    r"(?:target opponent|each (?:of your )?opponents?|that player|"
    r"defending player|the chosen (?:player|opponent)) "
    r"faces a villainous choice\s*[-:]\s*(.+?)\.?\s*$"
)
def _saga_chapter_villainous(m, raw):
    body = m.group(1)
    branches = _split_branches(body)
    options = tuple(_parse_branch(b) for b in branches if b)
    if not options:
        options = (_u(body),)
    choice = Choice(options=options, pick=1)
    return Static(
        modification=Modification(
            kind="saga_villainous_choice",
            args=("saga_chapter", choice),
        ),
        raw=raw,
    )


# "for each of them, <chooser> faces a villainous choice - ..." — Hunted by
# The Family templating reaching us as a static-shaped sentence (because
# compound_seq's `_trailing_for_each` would otherwise flatten it).
@_stat(
    r"^for each of them,\s+(.+?) faces a villainous choice\s*[-:]\s*"
    r"(.+?)\.?\s*$"
)
def _orphan_for_each_villainous(m, raw):
    chooser_text = m.group(1).strip()
    body = m.group(2)
    chooser = _normalize_chooser(chooser_text)
    branches = _split_branches(body)
    options = tuple(_parse_branch(b) for b in branches if b)
    if not options:
        options = (_u(body),)
    choice = Choice(options=options, pick=1)
    return Static(
        modification=Modification(
            kind="villainous_choice_for_each",
            args=(chooser, choice),
        ),
        raw=raw,
    )


# Orphan "X faces a villainous choice - ..." line that didn't get caught
# as an effect (e.g. because it survived splitting attached to another
# fragment). Last-resort static rider.
@_stat(
    r"^(?:they|that player|that opponent|target opponent|defending player|"
    r"each (?:of your )?opponents?|that creature'?s? (?:controller|owner)|"
    r"the chosen (?:player|opponent)) "
    r"faces a villainous choice\s*[-:]\s*(.+?)\.?\s*$"
)
def _orphan_faces_villainous(m, raw):
    # Re-derive the chooser from the leading slice of raw.
    head = raw[: raw.lower().index("faces a villainous choice")].strip()
    chooser = _normalize_chooser(head)
    body = m.group(1)
    branches = _split_branches(body)
    options = tuple(_parse_branch(b) for b in branches if b)
    if not options:
        options = (_u(body),)
    choice = Choice(options=options, pick=1)
    return Static(
        modification=Modification(
            kind="villainous_choice",
            args=(chooser, choice),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Public dispatcher (mirrors compound_seq.try_static_extensions).
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


# ---------------------------------------------------------------------------
# Priority bump: parse_effect iterates EFFECT_RULES in append order and
# returns on the first match. compound_seq.py's `_then_chain` ("do A. then
# B.") loads earlier (alphabetical module order) and would steal cards like
# Davros / Dr. Eggman / Great Intelligence's Plan whose villainous-choice
# clause is glued to a head clause via a leading "then". To win priority,
# we splice our most-collision-prone rules into the FRONT of the live
# `parser.EFFECT_RULES` registry at module-load time. The normal post-exec
# append in `load_extensions()` will duplicate them at the tail but the
# front copy fires first, so the duplicate is harmless.
def _prepend_priority_rules() -> None:
    try:
        import parser as _parser  # type: ignore
    except Exception:
        return
    if not hasattr(_parser, "EFFECT_RULES"):
        return
    # Only the rules whose body starts with "<head>. then <chooser> faces"
    # or "<head>, then faces" need to outrun compound_seq. Pure
    # "<chooser> faces a villainous choice - ..." rules will win on their
    # own because no earlier rule consumes that prefix.
    priority_patterns = (
        _then_chooser_villainous,
        _then_faces_villainous,
        _for_each_faces_villainous,
    )
    priority_rules = [r for r in EFFECT_RULES if r[1] in priority_patterns]
    for rule in reversed(priority_rules):
        _parser.EFFECT_RULES.insert(0, rule)
    # Same priority concern for static patterns: compound_seq's
    # `_trailing_for_each` claims "for each ..., ..." sentences before our
    # `_for_each_faces_villainous` static fallback can fire. parse_static
    # walks EXT_STATIC_PATTERNS top-down, so prepend ours.
    if hasattr(_parser, "EXT_STATIC_PATTERNS"):
        for rule in reversed(STATIC_PATTERNS):
            _parser.EXT_STATIC_PATTERNS.insert(0, rule)


_prepend_priority_rules()


__all__ = [
    "EFFECT_RULES",
    "STATIC_PATTERNS",
    "try_static_extensions",
]
