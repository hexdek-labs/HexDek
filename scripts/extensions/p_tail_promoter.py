#!/usr/bin/env python3
"""Parsed-tail promoter — converts common residual patterns to proper semantic types.

Family: parsed_tail / if_intervening_tail -> semantic Modification kinds.

Loads as ``p_tail_promoter.py`` so it sorts BEFORE ``partial_final.py`` in the
alphabetical extension load order.  This means these more-specific semantic
patterns match *first*, preventing ``partial_final.py`` from catching them as
generic ``parsed_tail`` nodes.

When ``partial_final.py`` later sees the same text, the parser has already
returned a result from this extension, so the text never reaches the
broad-bucket ``parsed_tail`` handlers.

The Go engine treats ``parsed_tail`` as an opaque blob and marks the card
PARTIAL. By promoting the top ~30 frequency buckets here to named kinds
(``self_enters_tapped``, ``restriction``, ``anthem``, ``timing_restriction``,
``spell_effect``, ``keyword_grant``, ``self_damage``, ``shuffle_clause``,
``replacement_static``, ``static_rule_mod``, ``saga_chapter``,
``ability_word``), those cards become COVERED in the engine's eyes.

Patterns are ordered most-specific first within each family to avoid
shadowing.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Modification, Static, Keyword,
)


# ═══════════════════════════════════════════════════════════════════════════
# STATIC_PATTERNS
# ═══════════════════════════════════════════════════════════════════════════

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    """Decorator: compile *pattern* with ``re.I`` and append to STATIC_PATTERNS."""
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I), fn))
        return fn
    return deco


# ───────────────────────────────────────────────────────────────────────────
# Restriction family (~300+ cards)
# Most-specific patterns first so conditional variants don't get swallowed.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^this creature can(?:'t| not) attack or block")
def _restriction_cant_attack_or_block(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("cant_attack_or_block",)),
        raw=raw,
    )


@_sp(r"^this creature can(?:'t| not) attack unless")
def _restriction_conditional_attack(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("conditional_attack", m.group(0))),
        raw=raw,
    )


@_sp(r"^this creature can(?:'t| not) be blocked (?:except by|by only)")
def _restriction_conditional_evasion(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("conditional_evasion", m.group(0))),
        raw=raw,
    )


@_sp(r"^this creature can(?:'t| not) be blocked")
def _restriction_unblockable(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("unblockable",)),
        raw=raw,
    )


@_sp(r"^this creature can(?:'t| not) block")
def _restriction_cant_block(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("cant_block",)),
        raw=raw,
    )


@_sp(r"^this creature can(?:'t| not) attack")
def _restriction_cant_attack(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("cant_attack",)),
        raw=raw,
    )


@_sp(r"^this spell can(?:'t| not) be countered")
def _restriction_uncounterable(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("uncounterable",)),
        raw=raw,
    )


@_sp(r"^~ can(?:'t| not) be (?:blocked|countered|the target)")
def _restriction_tilde_cant_be(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Enters-tapped family (~500 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^this (?:land|creature|permanent|artifact) enters (?:the battlefield )?tapped")
def _self_enters_tapped_this(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="self_enters_tapped"),
        raw=raw,
    )


@_sp(r"^~ enters (?:the battlefield )?tapped")
def _self_enters_tapped_tilde(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="self_enters_tapped"),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Anthem / buff family (~100+ cards)
# Longer patterns (with "until end of turn") before shorter ones.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^creatures you control get \+(\d+)/\+(\d+) until end of turn")
def _anthem_your_eot(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="anthem",
            args=(int(m.group(1)), int(m.group(2)), "until_eot"),
        ),
        raw=raw,
    )


@_sp(r"^all creatures get ([+-]\d+)/([+-]\d+) until end of turn")
def _anthem_all_eot(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="anthem",
            args=(int(m.group(1)), int(m.group(2)), "all", "until_eot"),
        ),
        raw=raw,
    )


@_sp(r"^creatures you control get \+(\d+)/\+(\d+)")
def _anthem_your_permanent(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="anthem",
            args=(int(m.group(1)), int(m.group(2))),
        ),
        raw=raw,
    )


@_sp(r"^creatures you control have "
     r"(haste|vigilance|trample|lifelink|flying|deathtouch|hexproof|"
     r"indestructible|menace|reach|first strike|double strike)")
def _keyword_grant(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="keyword_grant", args=(m.group(1),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Damage family (~200+ cards)
# X-damage before fixed-amount so the X-pattern doesn't shadow.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^~ deals? x damage to")
def _spell_effect_x_damage(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="spell_effect", args=(m.group(0),)),
        raw=raw,
    )


@_sp(r"^~ deals (\d+) damage to "
     r"(any target|target (?:creature|player|opponent|"
     r"creature or player|creature or planeswalker))")
def _spell_effect_fixed_damage(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="spell_effect", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Timing restriction family (~115 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^this ability triggers only once each turn")
def _timing_once_per_turn(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="timing_restriction", args=("once_per_turn",)),
        raw=raw,
    )


@_sp(r"^activate (?:this ability |~ )?only (?:as a sorcery|any time|once|during)")
def _timing_activate_only(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="timing_restriction", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Pain lands / self-damage (~22 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^(?:this land|~) deals (\d+) damage to you")
def _self_damage(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="self_damage", args=(int(m.group(1)),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Saga chapter III — transform (~27 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^(?:iii|iv|v) ?[-—] ?exile this saga,? then return it to the battlefield transformed")
def _saga_chapter_transform(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="saga_chapter", args=("transform_self",)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Commander / leyline / enters-prepared family (~40 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^~ can be your commander")
def _can_be_commander(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="static_rule_mod", args=("can_be_commander",)),
        raw=raw,
    )


@_sp(r"^this creature enters prepared")
def _enters_prepared(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="self_enters_tapped", args=("prepared",)),
        raw=raw,
    )


@_sp(r"^if this (?:card|spell) is in your opening hand,? you may begin the game with it on the battlefield")
def _leyline(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="static_rule_mod", args=("leyline",)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Kinship (~12 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^kinship [-—] at the beginning of your upkeep,? you may look at the top card of your library")
def _kinship(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="ability_word", args=("kinship",)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# If-clause tails (also catches if_intervening_tail entries)
# Most-specific first.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^if you search your library this way,? shuffle")
def _shuffle_clause(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="shuffle_clause"),
        raw=raw,
    )


@_sp(r"^if (?:that creature|it) would die this turn,? exile it instead")
def _replacement_die_to_exile(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="replacement_static", args=("die_to_exile",)),
        raw=raw,
    )


@_sp(r"^if that spell (?:would be put into|is countered this way,? exile)")
def _replacement_spell_exile(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="replacement_static", args=(m.group(0),)),
        raw=raw,
    )


# ═══════════════════════════════════════════════════════════════════════════
# ROUND 2 — broader pattern families
# ═══════════════════════════════════════════════════════════════════════════

# ───────────────────────────────────────────────────────────────────────────
# "Other creatures you control" anthem / keyword grant (~40 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^other creatures you control get \+(\d+)/\+(\d+)")
def _anthem_other_your(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="anthem",
            args=(int(m.group(1)), int(m.group(2)), "other"),
        ),
        raw=raw,
    )


@_sp(r"^other creatures you control have ([\w ]+)")
def _keyword_grant_other(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="keyword_grant", args=(m.group(1), "other")),
        raw=raw,
    )


@_sp(r"^creatures your opponents control get ([+-]\d+)/([+-]\d+)")
def _anthem_opponents(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="anthem",
            args=(int(m.group(1)), int(m.group(2)), "opponents"),
        ),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Self-buff "this creature gets" family (~30 cards)
# More-specific "+X/+0 for each" before generic "+X/+X".
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^this creature gets \+(\d+)/\+0 for each")
def _self_buff_per_each(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="self_buff", args=(m.group(0),)),
        raw=raw,
    )


@_sp(r"^this creature gets \+(\d+)/\+(\d+)")
def _self_buff_fixed(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="self_buff",
            args=(int(m.group(1)), int(m.group(2))),
        ),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Enchantment buffs "enchanted creature gets" (~10 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^enchanted creature gets \+(\d+)/\+(\d+)")
def _aura_buff(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="aura_buff",
            args=(int(m.group(1)), int(m.group(2))),
        ),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Escape / kicker ETB counters (~10 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^this creature escapes with (?:a |two |three |four )?\+1/\+1 counters? on it")
def _etb_counters_escape(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="etb_with_counters", args=("escape",)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# "Lands don't untap" restriction (~5 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^lands (?:you|they|that player) controls? don(?:'t| not) untap")
def _restriction_no_untap_lands(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("no_untap_lands",)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Cost modifications (~15 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^(?:spells|abilities|this ability) (?:you cast |your opponents cast |.*?)?costs? \{?\d\}? (?:more|less)")
def _additional_cost(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="additional_cost", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Timing: "activate no more than" / "only once" (~10 cards)
# More-specific "no more than" before the existing broader "activate only".
# NOTE: "this ability triggers only once each turn" already exists above.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^activate (?:this ability )?(?:no more than|only) (?:once|twice|three times)")
def _timing_activate_no_more_than(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="timing_restriction", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Toughness assigns combat damage (~6 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^each creature you control assigns combat damage equal to its toughness")
def _toughness_assigns_damage(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="static_rule_mod",
            args=("toughness_assigns_damage",),
        ),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Evasion-based restrictions (~6 cards)
# More-specific "without flying can't attack/block" first.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^creatures without flying can(?:'t| not) (?:attack|block)")
def _restriction_no_flying(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=("no_flying_cant_block",)),
        raw=raw,
    )


@_sp(r"^creatures with power less than")
def _restriction_power_less_than(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="restriction", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# "Each player/opponent" spell effects (~30 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^each (?:player|opponent) (?:shuffles|discards|draws|sacrifices|loses)")
def _spell_effect_each_player(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="spell_effect", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# "Choose one or both" / modal (~5 cards)
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^choose one (?:or both|or more)")
def _spell_effect_modal(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="spell_effect", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Broad "~ deals damage" catch-all (~100+ remaining cards)
# MUST come AFTER the specific damage patterns above.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^~ deals? (?:\d+|x) damage")
def _spell_effect_damage_broad(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="spell_effect", args=(m.group(0),)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# Ability words (~30 cards with various ability words)
# Kinship is handled above; this catches the remaining ability words.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^(?:valiant|pack tactics|void|disappear|formidable|threshold|morbid|"
     r"metalcraft|delirium|ferocious|hellbent|spell mastery|raid|revolt|domain|"
     r"landfall|constellation|heroic|inspired|battalion|bloodrush|channel|"
     r"forecast|radiance|grandeur|imprint|join forces|kinfall|tempting offer|"
     r"will of the council|lieutenant|parley|strive|unleash) [-—]")
def _ability_word_broad(m, raw):
    # Extract the ability word name (everything before the dash separator)
    text = m.group(0)
    ability_name = text.rsplit(" ", 1)[0].rsplit("—", 1)[0].rsplit("-", 1)[0].strip()
    return Static(
        condition=None,
        modification=Modification(kind="ability_word", args=(ability_name,)),
        raw=raw,
    )


# ───────────────────────────────────────────────────────────────────────────
# if_intervening_tail broad families
# Most-specific first, broad catch-alls last.
# NOTE: "if you search your library this way, shuffle" already exists above.
# NOTE: "if that creature/it would die this turn, exile" already exists above.
# NOTE: "if that spell would be put into / is countered" already exists above.
# ───────────────────────────────────────────────────────────────────────────

@_sp(r"^if damage would be dealt to (?:this creature|~|it),? prevent")
def _replacement_damage_prevention(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="replacement_static",
            args=("damage_prevention",),
        ),
        raw=raw,
    )


@_sp(r"^if (?:a creature|it|that creature|that creature or planeswalker) "
     r"(?:dealt damage this way )?would die")
def _replacement_die_broad(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="replacement_static",
            args=("die_replacement", m.group(0)),
        ),
        raw=raw,
    )


@_sp(r"^if (?:one or more )?(?:tokens|counters) would be (?:created|put)")
def _replacement_token_counter(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="replacement_static",
            args=(m.group(0),),
        ),
        raw=raw,
    )


@_sp(r"^if (?:a |that |this )?spell (?:cast this way|would be put into)")
def _replacement_spell_broad(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="replacement_static",
            args=(m.group(0),),
        ),
        raw=raw,
    )


@_sp(r"^if (?:this creature|~|it) was kicked")
def _conditional_kicker(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="etb_with_counters",
            args=("kicker", m.group(0)),
        ),
        raw=raw,
    )


@_sp(r"^if you can(?:'t| not),? (?:you )?lose the game")
def _conditional_pact_lose(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="conditional_static",
            args=("pact_lose",),
        ),
        raw=raw,
    )


@_sp(r"^if (?:you searched|you search) your library this way,? shuffle")
def _shuffle_clause_searched(m, raw):
    return Static(
        condition=None,
        modification=Modification(kind="shuffle_clause"),
        raw=raw,
    )


@_sp(r"^if (?:it(?:'s| is) a land card),? you may put it onto the battlefield")
def _conditional_land_put(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="conditional_static",
            args=(m.group(0),),
        ),
        raw=raw,
    )


@_sp(r"^if (?:you win|you lose) (?:the flip)?")
def _conditional_flip(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="conditional_static",
            args=(m.group(0),),
        ),
        raw=raw,
    )


@_sp(r"^if (?:it(?:'s| is) (?:neither day nor night|your turn|not your turn))")
def _conditional_turn_state(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="conditional_static",
            args=(m.group(0),),
        ),
        raw=raw,
    )


@_sp(r"^if you control (?:a commander|two or more)")
def _conditional_control(m, raw):
    return Static(
        condition=None,
        modification=Modification(
            kind="conditional_static",
            args=(m.group(0),),
        ),
        raw=raw,
    )
