#!/usr/bin/env python3
"""Choose-a-X / Name-a-X / Player-chooses extensions.

This module owns the **CHOOSE-A-X EFFECTS** family: oracle-text shapes where
the controller (or a named player) makes a discrete choice during resolution
and downstream text references "the chosen …" / "that creature" / etc.

Scope (per comp rules §701 "Keyword Actions" and assorted choose-prefixed
clauses):

    Controller choose-a-thing (resolution-time):
        choose a color
        choose a creature type
        choose an artifact type / card type / permanent type
        choose a basic land type
        choose a number
        choose a player / choose an opponent
        choose left or right
        choose a kingdom / clan / guild / shard / wedge         (faction-flavor)

    Name-a-card family (Pithing Needle / Cabal Therapy / Meddling Mage):
        name a card
        name a nonland card
        name a nonartifact, nonland card
        name a creature card
        name a card other than a basic land card

    Back-reference (static riders keyed on a prior choice):
        ~ is the chosen color
        the chosen color / the chosen type / the chosen name
        creatures of the chosen type get +1/+1
        creatures you control of the chosen type get +1/+1
        spells of the chosen type cost {1} less
        spells with the chosen name can't be cast
        activated abilities of sources with the chosen name can't be activated
        enchanted land is the chosen type
        ~ has protection from the chosen color
        creatures you control gain protection from the chosen color
        destroy each permanent/creature chosen this way
        destroy the chosen creatures

    Opponent / target-player chooses (Vendilion Clique-class):
        target opponent chooses a card from it
        target player chooses a creature you control
        an opponent chooses one of those piles
        an opponent chooses one of those cards
        an opponent chooses one of them
        you choose a card from it / you choose one of them

    Each-player-chooses (group-slug / Braids-style):
        each player chooses a creature they control
        each opponent chooses a creature they control
        each player chooses a number of lands they control equal to …
        each player chooses any number of creatures they control …

    Bulk-choose-N-permanents (Aether Gale / Cyclonic Rift-ish sweepers with
    a target-player picking):
        [player] chooses [N] permanents [filter]. ~ does X to those permanents

    Multiplayer-only choices:
        you choose [N] of your opponents
        for each opponent, choose up to one target creature that player controls

    Divide-as-you-choose (Fireball / Cone of Flame-class):
        ~ deals N damage divided as you choose among [filter] targets
        ~ deals N damage divided as you choose among any number of targets

The AST we emit for these is intentionally thin — we don't have dedicated
"Choose-a-value" or "OpponentChooses" AST node types, so we lean on
``UnknownEffect(raw_text='choose:…')`` and ``Static(modification=Modification(
kind='chosen_back_ref', args=(…,)))`` to keep the structural fingerprint
stable while leaving the specific choice type introspectable downstream.

Where a real back-reference rider applies a Buff or Restriction, we emit the
most-specific AST we can (Buff / Modification) so score-keeping / clustering
can bin these with their functional cousins.

This is a *pattern-extension* module — it does NOT import from parser.py nor
modify it. Integration is automatic via ``parser.load_extensions()``.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

# scripts/ on sys.path so `from mtg_ast import …` works
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Buff, Condition, Conditional, Damage, Effect, Filter, Modification,
    Sacrifice, Static, TARGET_ANY, TARGET_CREATURE, Trigger, Triggered,
    UnknownEffect,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}


def _num(token: Optional[str]):
    if token is None:
        return 1
    t = token.strip().lower()
    if t.isdigit():
        return int(t)
    return _NUM_WORDS.get(t, t)


# A canonical "the taxonomic category being chosen". These words appear both
# in the choose- and chosen-phrases and drive back-reference matching.
_CHOOSABLE = (
    r"creature|artifact|planeswalker|enchantment|instant|sorcery|land|"
    r"basic land|card|permanent"
)
_CHOOSABLE_TYPE = rf"(?:{_CHOOSABLE}) type"


# ---------------------------------------------------------------------------
# EFFECT_RULES — body effects that appear as stand-alone clauses.
#
# The parser consults EFFECT_RULES from parse_effect() BEFORE falling through.
# Every rule below returns an Effect AST node. Matches must (roughly) consume
# the full clause (parse_effect checks m.end() >= len(text) - 2).
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Controller chooses a discrete value ------------------------------------

@_eff(rf"^choose a ({_CHOOSABLE})(?: type)?(?:\s+other than [^.]+)?$")
def _choose_type_bare(m):
    kind = m.group(1).strip().lower()
    return UnknownEffect(raw_text=f"choose:type:{kind}")


@_eff(r"^choose a color(?:\s+other than [^.]+)?$")
def _choose_color(m):
    return UnknownEffect(raw_text="choose:color")


@_eff(r"^choose a basic land type(?:\s+at random)?$")
def _choose_basic_land_type(m):
    return UnknownEffect(raw_text="choose:basic_land_type")


@_eff(r"^choose a number(?:\s+(?:between \d+ and \d+|greater than \d+|"
      r"that isn'?t \d+))?$")
def _choose_number(m):
    return UnknownEffect(raw_text="choose:number")


@_eff(r"^choose a player$")
def _choose_player(m):
    return UnknownEffect(raw_text="choose:player")


@_eff(r"^choose an opponent$")
def _choose_opponent(m):
    return UnknownEffect(raw_text="choose:opponent")


@_eff(r"^choose left or right$")
def _choose_direction(m):
    return UnknownEffect(raw_text="choose:direction")


@_eff(r"^choose any target$")
def _choose_any_target(m):
    return UnknownEffect(raw_text="choose:any_target")


# Faction-flavor buckets (Tarkir / Khans / Ravnica / MH3)
@_eff(r"^choose a (kingdom|clan|guild|shard|wedge|house|family|"
      r"college|background)$")
def _choose_faction(m):
    return UnknownEffect(raw_text=f"choose:faction:{m.group(1).lower()}")


# --- Name-a-card family -----------------------------------------------------

@_eff(r"^choose a(?:\s+nonland)? card name"
      r"(?:\s*,\s*other than a basic land card name)?$")
def _choose_card_name(m):
    return UnknownEffect(raw_text="choose:card_name")


@_eff(r"^name a (?:nonland |nonartifact,?\s+nonland |creature |nontoken )?card"
      r"(?:\s+other than a basic land card)?$")
def _name_card(m):
    return UnknownEffect(raw_text="choose:card_name")


@_eff(r"^name a card(?:\s*,\s*then target player mills a card)?$")
def _name_card_mill(m):
    return UnknownEffect(raw_text="choose:card_name_then_mill")


# --- Multiplayer "you choose N of your opponents" ---------------------------

@_eff(r"^you choose (a|an|one|two|three|\d+) of your opponents?$")
def _choose_n_opponents(m):
    return UnknownEffect(raw_text=f"choose:opponents:{_num(m.group(1))}")


# --- Opponent / target-player chooses (Vendilion Clique-class) --------------
# Note: intentionally small — we just tag the shape; the Effect body that
# follows is handled by parse_effect downstream where applicable.

@_eff(r"^(?:target opponent|an opponent|target player|that player) "
      r"chooses a (?:card|creature|permanent)(?:[^.]*?)(?:from [^.]+)?$")
def _opp_chooses_thing(m):
    return UnknownEffect(raw_text=f"opp_choose:{m.group(0).lower()}")


@_eff(r"^(?:an opponent|target opponent) chooses one of (?:them|those piles|"
      r"those cards)$")
def _opp_chooses_pile(m):
    return UnknownEffect(raw_text="opp_choose:pile")


@_eff(r"^you choose (?:a|one|one of) [^.]+? (?:card|pile|creature|"
      r"permanent|them)(?:\s+from [^.]+)?$")
def _you_choose_thing(m):
    return UnknownEffect(raw_text=f"you_choose:{m.group(0).lower()}")


# --- Each-player / each-opponent chooses -----------------------------------

@_eff(r"^each (player|opponent) chooses (a|an|one|two|three|any number of|"
      r"a number of|\d+) ([^.]+?)(?:\.|$)")
def _each_player_chooses(m):
    who = m.group(1).lower()
    n = _num(m.group(2))
    what = m.group(3).strip()
    return UnknownEffect(raw_text=f"each_{who}_chooses:{n}:{what}")


# --- Divide-as-you-choose damage (Fireball-class) --------------------------

@_eff(r"^~ deals (\d+|x) damage divided as you choose "
      r"(?:among|to) (?:any number of|one(?:,? two)?(?:,? or three)? )?"
      r"target(?:s)?(?:\s+[^.]+?)?$")
def _divided_damage(m):
    amt = m.group(1)
    amt = int(amt) if amt.isdigit() else "x"
    return Damage(amount=amt, target=TARGET_ANY, divided=True)


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — ETB choose riders beyond what conditional_etb.py covers.
# These match the trigger PREFIX so the remaining text is re-entered into
# parse_effect. We keep them narrow — conditional_etb already owns "as ~
# enters, …"; these are for variants it doesn't catch.
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # "as ~ enters the battlefield, ..." — catch alternate spellings that
    # conditional_etb may miss (this creature / this vehicle / this equipment).
    (re.compile(r"^as this vehicle enters(?: the battlefield)?", re.I),
     "etb_as", "self"),
    (re.compile(r"^as this equipment becomes attached to a creature", re.I),
     "attached_as", "self"),
    (re.compile(r"^as this creature transforms(?: into [^,]+)?", re.I),
     "transform_as", "self"),
    (re.compile(r"^as this (?:creature|aura) is turned face up", re.I),
     "face_up_as", "self"),
]


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — back-references to "the chosen X" and choose-riders.
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _stat(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ~ is the chosen color / ~ is the chosen type
@_stat(r"^(?:~|this creature|this permanent|enchanted (?:land|creature|permanent)) "
       r"is the chosen (color|type|name)(?:\.|$)")
def _self_is_chosen(m, raw):
    return Static(
        modification=Modification(kind="self_is_chosen",
                                  args=(m.group(1).lower(),)),
        raw=raw,
    )


# enchanted land is the chosen type  (Spreading Seas-class)
@_stat(r"^enchanted land is the chosen (?:land )?type(?:\.|$)")
def _enchanted_is_chosen_type(m, raw):
    return Static(
        modification=Modification(kind="enchanted_is_chosen_type"),
        raw=raw,
    )


# "(other )?creatures (you control )?of the chosen (type|color) get +N/+N"
@_stat(r"^(other )?creatures(?: you control)?(?: of the chosen (?:type|color))"
       r" get \+(\d+)/\+(\d+)(?:[^.]*)?(?:\.|$)")
def _chosen_anthem(m, raw):
    other = m.group(1) is not None
    p, t = int(m.group(2)), int(m.group(3))
    kind = "tribal_anthem_chosen" if other else "anthem_chosen"
    return Static(
        modification=Modification(kind=kind, args=(p, t)),
        raw=raw,
    )


# "creatures of the chosen type get +N/+N"  (no "you control")
@_stat(r"^creatures of the chosen (?:type|color) get \+(\d+)/\+(\d+)"
       r"(?:\s+and [^.]+?)?(?:\.|$)")
def _chosen_global_anthem(m, raw):
    p, t = int(m.group(1)), int(m.group(2))
    return Static(
        modification=Modification(kind="global_anthem_chosen", args=(p, t)),
        raw=raw,
    )


# "spells (you cast )?of the chosen type cost {N} less to cast"
@_stat(r"^(?:spells(?: you cast)?|creature spells) of the chosen (?:type|color)"
       r" cost \{(\d+)\} less to cast(?:\.|$)")
def _chosen_cost_reduction(m, raw):
    n = int(m.group(1))
    return Static(
        modification=Modification(kind="cost_reduction_chosen", args=(n,)),
        raw=raw,
    )


# "spells with the chosen name can't be cast"
@_stat(r"^spells with the chosen name can'?t be cast(?:\.|$)")
def _chosen_name_uncastable(m, raw):
    return Static(
        modification=Modification(kind="chosen_name_uncastable"),
        raw=raw,
    )


# "your opponents can't cast spells with the chosen name"
@_stat(r"^your opponents can'?t cast spells with the chosen name(?:\.|$)")
def _opps_cant_cast_chosen(m, raw):
    return Static(
        modification=Modification(kind="opps_cant_cast_chosen_name"),
        raw=raw,
    )


# "activated abilities of sources with the chosen name can't be activated
#  [unless they're mana abilities]"
@_stat(r"^activated abilities of sources with the chosen name can'?t be "
       r"activated(?:\s+unless they'?re mana abilities)?(?:\.|$)")
def _pithing_needle(m, raw):
    return Static(
        modification=Modification(kind="pithing_needle_chosen"),
        raw=raw,
    )


# "creatures you control gain protection from the chosen color until end of turn"
@_stat(r"^creatures you control gain protection from the chosen color"
       r"(?:\s+until end of turn)?(?:\.|$)")
def _team_protection_chosen(m, raw):
    return Static(
        modification=Modification(kind="team_protection_from_chosen"),
        raw=raw,
    )


# "~ has protection from the chosen color"
@_stat(r"^(?:~|this creature|this permanent) has protection from the chosen "
       r"(color|type)(?:\.|$)")
def _self_protection_chosen(m, raw):
    return Static(
        modification=Modification(kind="self_protection_from_chosen",
                                  args=(m.group(1).lower(),)),
        raw=raw,
    )


# "destroy each permanent chosen this way" / "destroy the chosen creatures"
@_stat(r"^destroy (?:each (?:permanent|creature) chosen this way|"
       r"the chosen (?:permanents?|creatures?))(?:\.|$)")
def _destroy_chosen(m, raw):
    # Emit a static "back-reference destroy" — downstream resolvers look at
    # the preceding choose-clause for the target set.
    return Static(
        modification=Modification(kind="destroy_chosen_set"),
        raw=raw,
    )


# "exile each permanent chosen this way" / "exile the chosen creatures"
@_stat(r"^exile (?:each (?:permanent|creature|card) chosen this way|"
       r"the chosen (?:permanents?|creatures?|cards?))(?:\.|$)")
def _exile_chosen(m, raw):
    return Static(
        modification=Modification(kind="exile_chosen_set"),
        raw=raw,
    )


# "put the chosen cards into your hand and the rest on the bottom of your library"
@_stat(r"^put the chosen cards? into your hand and the rest"
       r"(?:\s+on the bottom of your library(?:\s+in a random order)?|"
       r"\s+into your graveyard|\s+on top of your library)?(?:\.|$)")
def _chosen_to_hand_rest(m, raw):
    return Static(
        modification=Modification(kind="chosen_to_hand_rest_elsewhere"),
        raw=raw,
    )


# "put all cards of the chosen type revealed this way into your hand and the
#  rest on the bottom of your library / into your graveyard"
@_stat(r"^put all cards of the chosen (?:type|color) revealed this way "
       r"into your hand and the rest"
       r"(?:\s+on the bottom of your library|\s+into your graveyard)?"
       r"(?:\.|$)")
def _chosen_type_reveal_sort(m, raw):
    return Static(
        modification=Modification(kind="chosen_type_reveal_sort"),
        raw=raw,
    )


# "each player chooses a creature type" (as an orphan static line — some
# oracle texts emit this alone)
@_stat(r"^each (player|opponent) chooses a "
       rf"(?:({_CHOOSABLE_TYPE})|color|number)(?:\.|$)")
def _each_player_choose_static(m, raw):
    who = m.group(1).lower()
    what = (m.group(2) or "color_or_number").lower().replace(" ", "_")
    return Static(
        modification=Modification(kind="each_player_choose",
                                  args=(who, what)),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Public dispatcher helpers (optional; parser auto-collects EFFECT_RULES,
# STATIC_PATTERNS, TRIGGER_PATTERNS via load_extensions()).
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
    "TRIGGER_PATTERNS",
    "try_static_extensions",
]
