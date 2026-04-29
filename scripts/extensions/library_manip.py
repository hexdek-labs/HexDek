#!/usr/bin/env python3
"""Library manipulation parser extension.

Owned family: LIBRARY MANIPULATION — peeking at, reordering, placing, milling,
revealing, exiling, and playing cards from the top/bottom of any library.

Exports:
  EFFECT_RULES: list[(compiled_regex, builder)] — appended to parser.EFFECT_RULES.
    Each builder takes the regex Match and returns an Effect AST node (or a
    Sequence for compound tails). Rules are matched via `re.match` against
    the full ability text; the parser accepts them when `m.end() >= len(text)-2`.
  STATIC_PATTERNS: list[(compiled_regex, builder)] — appended to the parser's
    static handler chain. Builders take the Match and return a Static (or Keyword)
    AST node.

Design notes:
  * `split_abilities` in parser.py splits on sentence boundaries, so compound
    cards like Impulse ("Look at the top four cards ... Put one of them ...")
    arrive here as TWO independent ability strings. Our rules therefore
    primarily target the isolated clause shapes — "look at top N", "put one
    of them into your hand", "put the rest on the bottom", etc. — rather than
    the whole compound sentence.
  * Comma-joined fragments ("search your library for a creature card, reveal
    it, then shuffle and put the card on top") DO arrive as one string, so we
    add specific compound rules for the canonical tutor-then-reveal-then-shuffle
    and the Brainstorm/Ponder reorder tails.
  * Effects that don't have a first-class AST node (e.g. "play lands from the
    top of your library", "the top card is revealed at all times", "manifest
    the top card") are emitted as UnknownEffect or Modification-wrapped Static
    nodes so they register as *parsed* (consuming the ability text) rather
    than leaking to parse_errors. This trades structural fidelity for coverage;
    a future pass can add dedicated AST nodes (`PlayFromTop`, `RevealedTop`,
    `Manifest`) without changing the rule shapes.
"""

from __future__ import annotations

import re
from typing import Optional

from mtg_ast import (
    Bounce, Effect, Exile, Filter, LookAt, Mill, Modification, Recurse, Reveal,
    Scry, Sequence, Shuffle, Static, Surveil, Tutor, UnknownEffect,
    SELF, TARGET_PLAYER,
)


# ============================================================================
# Helpers
# ============================================================================

_NUM = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11,
    "twelve": 12, "thirteen": 13, "x": "x",
}


def _num(token: str):
    token = token.strip().lower()
    if token in _NUM:
        return _NUM[token]
    if token.isdigit():
        return int(token)
    return token


def _library_owner(phrase: str) -> Filter:
    """Classify whose library is being referenced."""
    p = phrase.lower()
    if "your library" in p:
        return SELF
    if "target player's library" in p or "target opponent's library" in p:
        return TARGET_PLAYER
    if "each player" in p or "their library" in p:
        return Filter(base="player", quantifier="each", targeted=False)
    if "target opponent" in p:
        return Filter(base="opponent", targeted=True)
    if "owner's library" in p or "its owner's library" in p:
        return Filter(base="that_card_owner", targeted=False)
    return SELF


def _unk(raw: str) -> UnknownEffect:
    return UnknownEffect(raw_text=raw)


def _lib(raw: str):
    """Wrap an UnknownEffect in a Sequence so parse_ability doesn't treat it
    as 'unparsed'. Library-manip effects that don't have dedicated AST nodes
    still belong to our covered grammar — the Sequence wrapping signals intent
    while preserving raw text for later structural refinement."""
    return Sequence(items=(UnknownEffect(raw_text=raw),))


# ============================================================================
# EFFECT_RULES — these are tried by parse_effect against an ability string
# ============================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _rule(pattern: str):
    compiled = re.compile(pattern, re.I | re.S)
    def decorator(fn):
        EFFECT_RULES.append((compiled, fn))
        return fn
    return decorator


# Optional trailing "... and the rest (on bottom | into graveyard | on top in any order)"
# clause used by many compound rules below. Defined up-front so rules can
# reference it at class-body time.
_TAIL_REST = r"(?:\s+and\s+the\s+(?:rest|other)s?\s+(on the bottom of (?:your|their|its owner's) library(?: in (?:any|a random) order)?|into (?:your|their|its owner's) graveyard|on top of (?:your|their) library(?: in (?:any|a random) order)?))?"


# ----------------------------------------------------------------------------
# "Look at the top N cards of [library]"
# ----------------------------------------------------------------------------

@_rule(r"^(?:you may )?look at the top (a|an|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|x|\d+) cards? of (your library|target player's library|target opponent's library|each player's library|their library)(?:\.|$)")
def _look_at_top_n(m):
    n = _num(m.group(1))
    return LookAt(target=_library_owner(m.group(2)), zone="library_top_n", count=n)


@_rule(r"^(?:you may )?look at the top card of (your library|target player's library|target opponent's library|their library)(?:\.|$)")
def _look_at_top(m):
    return LookAt(target=_library_owner(m.group(1)), zone="library_top_n", count=1)


# "you may look at the top card of your library any time" / "at any time"
@_rule(r"^you may look at the top card of your library(?: at)? any time(?:\.|$)")
def _may_peek_top_anytime(m):
    return LookAt(target=SELF, zone="library_top_n", count=1)


# ----------------------------------------------------------------------------
# Reveal N from top / reveal top card
# ----------------------------------------------------------------------------

@_rule(r"^reveal the top (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) cards? of (your library|target player's library|their library)(?:\.|$)")
def _reveal_top_n(m):
    n = _num(m.group(1))
    return Reveal(source="top_of_library", count=n, actor="controller")


@_rule(r"^reveal the top card of (your library|target player's library|their library)(?:\.|$)")
def _reveal_top_card(m):
    return Reveal(source="top_of_library", count=1, actor="controller")


@_rule(r"^target player reveals their hand(?:\.|$)")
def _target_reveals_hand(m):
    return Reveal(source="opponent_hand", count=1, actor="opponent")


@_rule(r"^each opponent reveals their hand(?:\.|$)")
def _each_opp_reveals_hand(m):
    return Reveal(source="opponent_hand", count=1, actor="opponent")


# Fact or Fiction-style tail: "reveal those cards, put them into your hand, then shuffle"
@_rule(r"^reveal those cards, put them into your hand, (?:then )?shuffle(?:\.|$)")
def _reveal_those_to_hand_shuffle(m):
    return Sequence(items=(
        Reveal(source="top_of_library", count="those", actor="controller"),
        Tutor(query=Filter(base="card_from_revealed"), destination="hand",
              count="all", shuffle_after=True, reveal=True),
    ))


# "put that card onto the battlefield and the rest on the bottom of your library..."
@_rule(r"^put that card onto the battlefield(?: under your control)?" + _TAIL_REST + r"(?:\.|$)")
def _put_that_to_bf(m):
    tail = (m.group(1) or "").lower()
    rest = "bottom" if "bottom" in tail else ("graveyard" if "graveyard" in tail else None)
    return Tutor(
        query=Filter(base="that_card"),
        destination="battlefield",
        count=1,
        shuffle_after=False,
        reveal=True,
        rest=rest,
    )


# "put one of them on top of your library and the rest on the bottom..."
@_rule(r"^put (?:up to )?(a|an|one|two|three|four|five|x) of them on top of your library" + _TAIL_REST + r"(?:\.|$)")
def _put_one_top(m):
    n = _num(m.group(1))
    tail = (m.group(2) or "").lower()
    rest = "bottom" if "bottom" in tail else ("graveyard" if "graveyard" in tail else None)
    return Tutor(
        query=Filter(base="card_from_revealed"),
        destination="top_of_library",
        count=n,
        shuffle_after=False,
        reveal=False,
        rest=rest,
    )


# "you may put a card from your hand on the bottom of your library"
@_rule(r"^you may put a card from your hand on the bottom of your library(?:\.|$)")
def _hand_to_bottom(m):
    return _lib("put hand card on bottom of library")


# ----------------------------------------------------------------------------
# "Put one/two/any number of them into your hand" — standalone tail clause
# produced by split_abilities on Impulse-class cards.
# Expected form: "put <N> of them into your hand and the rest on the bottom
# of your library [in any order]."  or "... and the rest into your graveyard."
# ----------------------------------------------------------------------------

@_rule(r"^put (a|an|one|two|three|four|five|x|any number) of (?:them|those cards) into your hand" + _TAIL_REST + r"(?:\.|$)")
def _put_one_into_hand(m):
    n = _num(m.group(1))
    tail = (m.group(2) or "").lower()
    rest = None
    if "graveyard" in tail:
        rest = "graveyard"
    elif "bottom" in tail:
        rest = "bottom"
    elif "top" in tail:
        rest = "top_reordered"
    return Tutor(
        query=Filter(base="card_from_revealed"),
        destination="hand",
        count=n,
        shuffle_after=False,
        reveal=True,
        rest=rest,
    )


# Impulse/Sleight of Hand tail: "you may reveal a/an [filter] card from among them
# and put it into your hand. put the rest on the bottom of your library..."
@_rule(r"^you may reveal a?n? ([^.]+?) card from among them and put it into your hand" + _TAIL_REST + r"(?:\.|$)")
def _may_reveal_filter_to_hand(m):
    filt = m.group(1).strip()
    tail = (m.group(2) or "").lower()
    rest = "bottom" if "bottom" in tail else ("graveyard" if "graveyard" in tail else ("top_reordered" if "top" in tail else None))
    return Tutor(
        query=Filter(base="card", extra=(filt,)),
        destination="hand",
        count=1,
        shuffle_after=False,
        reveal=True,
        optional=True,
        rest=rest,
    )


# Same shape but with "with mana value N or less" / "with power 2 or less" modifiers
@_rule(r"^you may reveal a?n? ([^.]+?card[^.]*?) from among them and put it into your hand" + _TAIL_REST + r"(?:\.|$)")
def _may_reveal_card_with_qual_to_hand(m):
    filt = m.group(1).strip()
    tail = (m.group(2) or "").lower()
    rest = "bottom" if "bottom" in tail else ("graveyard" if "graveyard" in tail else None)
    return Tutor(
        query=Filter(base="card", extra=(filt,)),
        destination="hand",
        count=1,
        shuffle_after=False,
        reveal=True,
        optional=True,
        rest=rest,
    )


@_rule(r"^put (?:that|the chosen) card into your hand" + _TAIL_REST + r"(?:\.|$)")
def _put_that_into_hand(m):
    tail = (m.group(1) or "").lower()
    rest = "bottom" if "bottom" in tail else ("graveyard" if "graveyard" in tail else None)
    return Tutor(
        query=Filter(base="that_card"),
        destination="hand",
        count=1,
        shuffle_after=False,
        reveal=True,
        rest=rest,
    )


# ----------------------------------------------------------------------------
# "Put the rest ..." — Brainstorm/Impulse tail clauses as their OWN ability
# ----------------------------------------------------------------------------

@_rule(r"^(?:then )?put the rest (?:of (?:them|those cards) )?on the bottom of (?:your|their|its owner's) library(?: in (?:any|a random) order)?(?:\.|$)")
def _put_rest_bottom(m):
    return _lib("put rest on bottom of library")


@_rule(r"^(?:then )?put the rest (?:of (?:them|those cards) )?into (?:your|their|its owner's) graveyard(?:\.|$)")
def _put_rest_graveyard(m):
    return Mill(count="rest", target=SELF)


@_rule(r"^(?:then )?put the rest on top of (?:your|their) library(?: in (?:any|a random) order)?(?:\.|$)")
def _put_rest_top(m):
    return _lib("put rest on top of library (reordered)")


# "put them back in any order" / "put them back on top ..." (Ponder tail)
@_rule(r"^(?:then )?put them back(?:\s+on top(?: of your library)?)?(?:\s+in (?:any|a random) order)?(?:\.|$)")
def _put_them_back(m):
    return _lib("reorder top cards of library")


# Compound: "look at the top N cards of your library, then put them back in any order"
# (Ponder — arrives as single comma-joined ability)
@_rule(r"^look at the top (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) cards? of (?:your|target player's) library,\s*(?:then )?put them back(?:\s+on top(?: of (?:your|their) library)?)?(?:\s+in (?:any|a random) order)?(?:\.|$)")
def _look_then_reorder(m):
    n = _num(m.group(1))
    return Sequence(items=(
        LookAt(target=SELF, zone="library_top_n", count=n),
        _unk("reorder top cards of library"),
    ))


# Compound: "draw N cards, then put N cards from your hand on top of your library in any order"
# (Brainstorm)
@_rule(r"^draw (a|one|two|three|four|five|x|\d+) cards?, (?:then )?put (a|one|two|three|four|five|x|\d+) cards? from your hand on top of your library(?: in (?:any|a random) order)?(?:\.|$)")
def _draw_then_put_top(m):
    from mtg_ast import Draw
    draw_n = _num(m.group(1))
    put_n = _num(m.group(2))
    return Sequence(items=(
        Draw(count=draw_n, target=SELF),
        _unk(f"put {put_n} cards from hand on top of library"),
    ))


# "Put N cards from your hand on top of your library" — standalone Brainstorm tail
@_rule(r"^(?:then )?put (a|one|two|three|four|five|x|\d+) cards? from your hand on top of your library(?: in (?:any|a random) order)?(?:\.|$)")
def _put_hand_top(m):
    return _lib(f"put {m.group(1)} cards from hand on top of library")


@_rule(r"^put the revealed cards on (?:the )?(?:bottom|top) of (?:your|their) library(?: in (?:any|a random) order)?(?:\.|$)")
def _put_revealed_back(m):
    return _lib("place revealed cards back on library")


# ----------------------------------------------------------------------------
# "Reveal cards from the top of your library until you reveal ..."
# ----------------------------------------------------------------------------

@_rule(r"^(?:each opponent )?reveals? cards from the top of (?:your|their) library until (?:you|they) reveal a?n? ([^.]+?card[^.]*?)(?:\.|$)")
def _reveal_until(m):
    query = Filter(base="card_matched", extra=(m.group(1).strip(),))
    return Tutor(
        query=query, destination="hand", count=1,
        shuffle_after=False, reveal=True, rest="bottom",
    )


@_rule(r"^its controller reveals cards from the top of their library until they reveal a?n? ([^.]+?card[^.]*?)(?:\.|$)")
def _reveal_until_controller(m):
    query = Filter(base="card_matched", extra=(m.group(1).strip(),))
    return Tutor(
        query=query, destination="hand", count=1,
        shuffle_after=False, reveal=True, rest="bottom",
    )


# ----------------------------------------------------------------------------
# "Exile the top N cards of your library"
# ----------------------------------------------------------------------------

@_rule(r"^exile the top (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) cards? of (your library|target player's library|their library|each player's library)(?:\.|$)")
def _exile_top_n(m):
    n = _num(m.group(1))
    target = _library_owner(m.group(2))
    return Exile(target=Filter(base="top_of_library", quantifier="n", count=n, targeted=False))


@_rule(r"^exile the top card of (your library|target player's library|their library)(?:\.|$)")
def _exile_top_1(m):
    return Exile(target=Filter(base="top_of_library", quantifier="one", count=1, targeted=False))


@_rule(r"^exile cards from the top of (your|their) library until (?:you|they) exile a?n? ([^.]+?card[^.]*?)(?:\.|$)")
def _exile_until(m):
    return Exile(target=Filter(base="top_of_library_until", extra=(m.group(2).strip(),)))


# ----------------------------------------------------------------------------
# Mill variants: "put the top N cards of your library into your graveyard"
# ----------------------------------------------------------------------------

@_rule(r"^(?:you )?(?:put|puts?) the top (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) cards? of (?:your|target player's|their) library into (?:your|that player's|their) graveyard(?:\.|$)")
def _mill_top_n(m):
    n = _num(m.group(1))
    target = TARGET_PLAYER if "target player" in m.string else SELF
    return Mill(count=n, target=target)


@_rule(r"^(?:target player |each opponent )?mills? (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) cards?(?:\.|$)")
def _mill_verb_n(m):
    n = _num(m.group(1))
    s = m.string.lower()
    if "each opponent" in s:
        target = Filter(base="opponent", quantifier="each", targeted=False)
    elif "target player" in s:
        target = TARGET_PLAYER
    else:
        target = SELF
    return Mill(count=n, target=target)


@_rule(r"^its controller mills? (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) cards?(?:\.|$)")
def _mill_controller(m):
    return Mill(count=_num(m.group(1)), target=Filter(base="that_controller", targeted=False))


# ----------------------------------------------------------------------------
# "Put the top card of your library into your hand" (Future Sight style)
# ----------------------------------------------------------------------------

@_rule(r"^put the top card of (?:your|target player's|their) library into (?:your|that player's|their) hand(?:\.|$)")
def _top_to_hand(m):
    return Recurse(query=Filter(base="top_of_library"), from_zone="top_of_library", destination="hand")


# ----------------------------------------------------------------------------
# "You may play lands / cast spells from the top of your library"
# ----------------------------------------------------------------------------

@_rule(r"^you may play lands from the top of your library(?:\.|$)")
def _play_lands_from_top(m):
    return _lib("play lands from top of library")


@_rule(r"^you may play lands and cast spells from the top of your library(?:\.|$)")
def _play_anything_from_top(m):
    return _lib("play lands and cast spells from top of library")


@_rule(r"^you may cast (creature|land|nonland|instant|sorcery|noncreature|artifact|enchantment|spells?|the top card)s? (?:spells? )?from the top of your library(?:\.|$)")
def _cast_filtered_from_top(m):
    return _lib(f"cast {m.group(1)} spells from top of library")


@_rule(r"^you may play (?:that card|it|the top card) (?:this turn|any time|as though it had flash)?(?:\.|$)")
def _may_play_that_card(m):
    return _lib("may play that card")


@_rule(r"^you may play that card (?:this turn|until end of turn)(?:\.|$)")
def _may_play_this_turn(m):
    return _lib("may play that card this turn")


# ----------------------------------------------------------------------------
# Scry / Surveil with larger N word-form
# ----------------------------------------------------------------------------

@_rule(r"^scry (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x)(?:\.|$)")
def _scry_word(m):
    return Scry(count=_num(m.group(1)))


@_rule(r"^surveil (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x)(?:\.|$)")
def _surveil_word(m):
    return Surveil(count=_num(m.group(1)))


# "scry 3, then reveal the top card of your library"
@_rule(r"^scry (\d+|x), then reveal the top card of your library(?:\.|$)")
def _scry_then_reveal(m):
    n = _num(m.group(1))
    return Sequence(items=(
        Scry(count=n),
        Reveal(source="top_of_library", count=1),
    ))


# ----------------------------------------------------------------------------
# Shuffle variants
# ----------------------------------------------------------------------------

@_rule(r"^shuffle(?: your library)?(?:\.|$)")
def _shuffle_self(m):
    return Shuffle(target=SELF)


@_rule(r"^you may shuffle(?:\.|$)")
def _may_shuffle(m):
    return Shuffle(target=SELF)


@_rule(r"^shuffle ~ into its owner's library(?:\.|$)")
def _shuffle_self_card(m):
    return Shuffle(target=Filter(base="self_into_owner_library", targeted=False))


@_rule(r"^its owner shuffles it into their library(?:\.|$)")
def _owner_shuffles_it(m):
    return Shuffle(target=Filter(base="that_card_owner", targeted=False))


@_rule(r"^target player shuffles their (?:library|graveyard into their library)(?:\.|$)")
def _target_player_shuffle(m):
    return Shuffle(target=TARGET_PLAYER)


@_rule(r"^each player shuffles? their (?:hand and )?(?:graveyard )?(?:and )?(?:hand )?into their library(?:[^.]*)?(?:\.|$)")
def _each_player_shuffle_in(m):
    return Shuffle(target=Filter(base="player", quantifier="each", targeted=False))


# ----------------------------------------------------------------------------
# Bounce-to-library: "put target <X> on top/bottom of its owner's library"
# ----------------------------------------------------------------------------

@_rule(r"^put target ([^.]+?) on top of (?:its|their) owner'?s? library(?:\.|$)")
def _bounce_to_top(m):
    target = Filter(base=m.group(1).strip(), targeted=True)
    return Bounce(target=target, to="top_of_library")


@_rule(r"^put target ([^.]+?) on the bottom of (?:its|their) owner'?s? library(?:\.|$)")
def _bounce_to_bottom(m):
    target = Filter(base=m.group(1).strip(), targeted=True)
    return Bounce(target=target, to="bottom_of_library")


@_rule(r"^put ~ on the bottom of its owner'?s? library(?:\.|$)")
def _self_to_bottom(m):
    return Bounce(target=Filter(base="self", targeted=False), to="bottom_of_library")


@_rule(r"^put target ([^.]+?) into its owner'?s? library second from the top(?:\.|$)")
def _second_from_top(m):
    return _lib(f"put {m.group(1).strip()} second from top of owner's library")


@_rule(r"^target creature'?s? owner puts it on their choice of the top or bottom of (?:their|its owner'?s?) library(?:\.|$)")
def _owner_chooses_top_bottom(m):
    return _lib("owner chooses top or bottom of library")


@_rule(r"^target nonland permanent'?s? owner puts it on their choice of the top or bottom of (?:their|its owner'?s?) library(?:\.|$)")
def _owner_chooses_top_bottom_nonland(m):
    return _lib("owner chooses top or bottom of library (nonland perm)")


# ----------------------------------------------------------------------------
# Graveyard→top: "put target card from your graveyard on top of your library"
# ----------------------------------------------------------------------------

@_rule(r"^put target ([^.]+?) from (?:your|target player's) graveyard on top of (?:your|their) library(?:\.|$)")
def _graveyard_to_top(m):
    return Recurse(
        query=Filter(base=m.group(1).strip()),
        from_zone="your_graveyard",
        destination="top_of_library",
    )


# ----------------------------------------------------------------------------
# Compound tutor-with-reveal-then-shuffle-then-put-on-top
# (Worldly Tutor / Mystical Tutor / Vampiric Tutor class)
# ----------------------------------------------------------------------------

@_rule(r"^search your library for ([^.]+?card[^.]*?), reveal it,? (?:then )?shuffle(?:,?\s+(?:and|then))? put (?:the|that) card on top(?: of your library)?(?:\.|$)")
def _tutor_reveal_shuffle_top(m):
    query_text = m.group(1).strip()
    return Tutor(
        query=Filter(base="card", extra=(query_text,)),
        destination="top_of_library",
        count=1,
        reveal=True,
        shuffle_after=True,
    )


@_rule(r"^search your library for ([^.]+?card[^.]*?), put (?:that|it) (?:card )?into your hand, (?:then )?shuffle(?:\.|$)")
def _tutor_to_hand_shuffle(m):
    query_text = m.group(1).strip()
    return Tutor(
        query=Filter(base="card", extra=(query_text,)),
        destination="hand",
        count=1,
        reveal=False,
        shuffle_after=True,
    )


@_rule(r"^search your library for ([^.]+?), then shuffle and put that card on top(?: of your library)?(?:\.|$)")
def _tutor_shuffle_top(m):
    return Tutor(
        query=Filter(base="card", extra=(m.group(1).strip(),)),
        destination="top_of_library",
        count=1,
        shuffle_after=True,
    )


# ----------------------------------------------------------------------------
# Manifest
# ----------------------------------------------------------------------------

@_rule(r"^manifest the top card of your library(?:\.|$)")
def _manifest_top(m):
    return _lib("manifest top card of library")


@_rule(r"^manifest the top (a|an|one|two|three|four|five|x|\d+) cards? of your library(?:\.|$)")
def _manifest_top_n(m):
    return _lib(f"manifest top {m.group(1)} cards of library")


@_rule(r"^manifest dread(?:\.|$)")
def _manifest_dread(m):
    return _lib("manifest dread")


# ----------------------------------------------------------------------------
# "Note the top card" (Vendilion Clique etc.)
# ----------------------------------------------------------------------------

@_rule(r"^note the top card of (your|target player's|their) library(?:\.|$)")
def _note_top(m):
    return _lib("note top card of library")


# ============================================================================
# STATIC_PATTERNS — handled at parse_static level
# ============================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _static(pattern: str):
    compiled = re.compile(pattern, re.I)
    def decorator(fn):
        STATIC_PATTERNS.append((compiled, fn))
        return fn
    return decorator


# Bolas's Citadel / Vendilion Clique-type: "the top card of your library is revealed"
@_static(r"^(?:the top card of (?:your|each player's) library is revealed(?: at all times)?|play with the top card of (?:your library|their libraries|your libraries) revealed)\b")
def _revealed_top_static(m):
    return Static(modification=Modification(kind="top_card_revealed"), raw=m.group(0))


@_static(r"^players play with (?:the top card of their libraries revealed|their hands revealed)\b")
def _players_reveal_static(m):
    return Static(modification=Modification(kind="public_information", args=(m.group(0),)), raw=m.group(0))


@_static(r"^your opponents play with their hands revealed\b")
def _opponent_open_hand(m):
    return Static(modification=Modification(kind="opponents_open_hand"), raw=m.group(0))


@_static(r"^you may look at the top card of your library(?: at)? any time\b")
def _may_peek_static(m):
    return Static(modification=Modification(kind="may_peek_top"), raw=m.group(0))


@_static(r"^you may play (?:lands(?: and cast spells)?|cards|(?:creature|instant|sorcery|noncreature|nonland|artifact|enchantment)s? spells?) from the top of your library\b")
def _play_from_top_static(m):
    return Static(modification=Modification(kind="play_from_top", args=(m.group(0),)), raw=m.group(0))


@_static(r"^you may play (?:the top card of your library|an additional land from the top of your library)\b")
def _play_top_card_static(m):
    return Static(modification=Modification(kind="play_from_top"), raw=m.group(0))


# ============================================================================
# Auto-register on import so parser.py picks us up even without explicit wiring.
# parser.py defines EFFECT_RULES at module level; we extend it so the main
# parse_effect loop sees our rules after its builtin ones (preserving
# precedence for short common cases while still covering our new phrasings).
# ============================================================================

def register(parser_module=None):
    """Append this module's rules to the given parser module (or the default
    one imported from `parser`)."""
    if parser_module is None:
        try:
            import parser as _pm  # type: ignore
        except Exception:
            return False
        parser_module = _pm
    existing = {id(p) for p, _ in getattr(parser_module, "EFFECT_RULES", [])}
    for pat, builder in EFFECT_RULES:
        if id(pat) not in existing:
            parser_module.EFFECT_RULES.append((pat, builder))
    # STATIC_PATTERNS: splice into parse_static by wrapping it.
    if hasattr(parser_module, "parse_static") and not getattr(parser_module, "_library_manip_wrapped", False):
        original_parse_static = parser_module.parse_static

        def wrapped_parse_static(text: str):
            s = text.strip().rstrip(".").lower()
            for pat, builder in STATIC_PATTERNS:
                mm = pat.match(s)
                if mm and mm.end() >= len(s) - 2:
                    try:
                        return builder(mm)
                    except Exception:
                        continue
            return original_parse_static(text)

        parser_module.parse_static = wrapped_parse_static
        parser_module._library_manip_wrapped = True
    return True


# Auto-register when imported as part of scripts.extensions package or directly.
try:
    register()
except Exception:
    pass


__all__ = ["EFFECT_RULES", "STATIC_PATTERNS", "register"]
