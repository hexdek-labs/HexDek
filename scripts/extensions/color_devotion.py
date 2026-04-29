#!/usr/bin/env python3
"""Color-identity-aware statics extension.

This module owns the grammar productions that depend on *colors* —
devotion, multicolor/monocolor tribal statics, "for each color" counters
and cost reductions, color-changing effects, color-hosing protection
variants, and the big family of "you may spend mana as though it were
mana of any color" riders.

Exports the standard triplet consumed by ``parser.load_extensions``:

- ``STATIC_PATTERNS``: ability-shape static patterns.
- ``EFFECT_RULES``: body-of-effect patterns (used inside spell /
  activated / triggered ability bodies).
- ``TRIGGER_PATTERNS``: color-specific triggered-ability starters.

Collision notes (verified against existing extensions):
  * ``choices.py`` already handles the narrow "chosen color" cases
    (``~ has protection from the chosen color`` / ``~ is the chosen
    color`` / team-grant of protection-from-chosen). We intentionally
    *do not* shadow those — we pick up the adjacent phrasings
    ("color of your choice", "color OR colors of your choice", "each
    color", "colorless or from the color of your choice", etc.).
  * ``ability_words.py`` owns ``domain`` as an ability word. We extend
    the non-ability-word shape: "this cost is reduced by {N} for each
    basic land type among lands you control".
  * ``equipment_aura.py`` handles ``<subject> has protection from X``
    on equipped/enchanted bodies — we don't duplicate that here.
  * ``KEYWORD_RE`` in ``parser.py`` already matches bare
    ``protection from <X>``. We don't step on it; our patterns anchor
    on explicit subjects (``this creature``, ``target ...``, ``creatures
    you control``) or on exotic right-hand sides (``each color``,
    ``each of your opponents``, ``colors of permanents you control``).

Design:
  All builders return ``Static`` (or, for a few trigger shapes,
  ``Triggered``) with a stable ``kind`` string, so downstream analyzers
  can slice the dataset by mechanic family.
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
    Filter, Keyword, Modification, Static, Trigger, Triggered,
    UnknownEffect,
)


# ---------------------------------------------------------------------------
# Shared regex fragments
# ---------------------------------------------------------------------------

# Single color word (excludes the phrase "colorless" from matching where we
# don't want it — each rule names this explicitly).
_COLOR = r"(?:white|blue|black|red|green)"
_COLOR_OR_LESS = r"(?:white|blue|black|red|green|colorless)"

# "<color> and <color>" / "<color>, <color>, and <color>" — up to 5.
_COLOR_LIST = (
    rf"{_COLOR}(?:(?:,\s*|\s+and\s+|\s+and\s+from\s+|\s+or\s+from\s+){_COLOR}){{0,4}}"
)


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Devotion-on-self (Theros gods, Heliod pantheon): "~ has devotion to X" -
@_sp(
    rf"^~ has devotion to (?P<c1>{_COLOR})"
    rf"(?: and (?:to )?(?P<c2>{_COLOR}))?\s*$"
)
def _self_devotion(m, raw):
    cols = [m.group("c1")]
    if m.group("c2"):
        cols.append(m.group("c2"))
    return Static(
        modification=Modification(kind="self_has_devotion", args=tuple(cols)),
        raw=raw,
    )


# --- Theros god indestructibility rider:
#     "as long as your devotion to <color> is less than [N], ~ isn't a creature"
@_sp(
    rf"^as long as your devotion to (?P<c1>{_COLOR})"
    rf"(?: and (?:to )?(?P<c2>{_COLOR}))? is less than "
    r"(?P<n>\w+), ~ isn'?t a creature\s*$"
)
def _god_not_creature(m, raw):
    cols = [m.group("c1")]
    if m.group("c2"):
        cols.append(m.group("c2"))
    return Static(
        modification=Modification(
            kind="devotion_gated_not_creature",
            args=(tuple(cols), m.group("n")),
        ),
        raw=raw,
    )


# --- Monocolored / multicolored tribal anthem:
#     "multicolored creatures you control get +N/+N"
#     "monocolored creatures you control have flying" etc.
@_sp(
    r"^(?P<class>multicolored|monocolored) creatures you control "
    r"get \+(?P<p>\d+)/\+(?P<t>\d+)\s*$"
)
def _color_class_anthem(m, raw):
    return Static(
        modification=Modification(
            kind="color_class_anthem",
            args=(m.group("class"), int(m.group("p")), int(m.group("t"))),
        ),
        raw=raw,
    )


@_sp(
    r"^(?:other )?(?P<class>multicolored|monocolored) creatures you control "
    r"(?:have|gain) (?P<body>[a-z ,{}0-9]+?)\s*$"
)
def _color_class_grant(m, raw):
    return Static(
        modification=Modification(
            kind="color_class_grant",
            args=(m.group("class"), m.group("body").strip()),
        ),
        raw=raw,
    )


# --- "multicolored spells cost {N} less to cast" / "... have convoke"
@_sp(
    r"^(?P<class>multicolored|monocolored) spells (?:you cast )?"
    r"cost \{(?P<n>\d+)\} less to cast\s*$"
)
def _color_class_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="color_class_cost_reduce",
            args=(m.group("class"), int(m.group("n"))),
        ),
        raw=raw,
    )


@_sp(
    r"^(?P<class>multicolored|monocolored) spells you cast have (?P<kw>[a-z ,]+?)\s*$"
)
def _color_class_spells_have(m, raw):
    return Static(
        modification=Modification(
            kind="color_class_spells_have",
            args=(m.group("class"), m.group("kw").strip()),
        ),
        raw=raw,
    )


# --- "monocolored creatures can't block this turn" / combat lockouts -------
@_sp(
    r"^(?P<class>multicolored|monocolored) creatures can'?t "
    r"(?P<what>attack|block)(?: this turn)?\s*$"
)
def _color_class_combat_lock(m, raw):
    return Static(
        modification=Modification(
            kind="color_class_combat_lock",
            args=(m.group("class"), m.group("what")),
        ),
        raw=raw,
    )


# --- "hexproof from multicolored" / "hexproof from monocolored" -----------
# KEYWORD_RE doesn't cover these right-hand sides; emit as a Keyword so the
# card counts as keyword-parsed.
@_sp(r"^hexproof from (?P<class>multicolored|monocolored)\s*$")
def _hexproof_from_class(m, raw):
    return Keyword(name="hexproof from", args=(m.group("class"),), raw=raw)


# --- Bare "hexproof from <rhs>" — KEYWORD_RE covers "protection from …"
# but not "hexproof from …". Mirror it here with a permissive RHS.
@_sp(
    r"^hexproof from (?P<rhs>"
    rf"{_COLOR_OR_LESS}(?:\s+and\s+(?:from\s+)?{_COLOR_OR_LESS})*"
    r"|artifacts|creatures|enchantments|planeswalkers"
    r"|artifacts and enchantments"
    r"|artifacts, creatures, and enchantments"
    r"|activated and triggered abilities"
    r"|each color|each of its colors|its colors|that color"
    r"|ring-bearers|non-human creatures"
    r")\s*$"
)
def _hexproof_from_any(m, raw):
    return Keyword(name="hexproof from", args=(m.group("rhs").strip(),), raw=raw)


# --- "flying, lifelink, hexproof from X" — multi-keyword lead-in
@_sp(
    r"^(?P<lead>[a-z ,]+?), hexproof from (?P<rhs>"
    rf"{_COLOR_OR_LESS}(?:\s+and\s+(?:from\s+)?{_COLOR_OR_LESS})*"
    r"|artifacts|creatures|enchantments|planeswalkers"
    r"|artifacts and enchantments"
    r"|artifacts, creatures, and enchantments"
    r"|activated and triggered abilities"
    r"|each color|each of its colors|its colors|that color"
    r")\s*$"
)
def _kw_lead_hexproof_any(m, raw):
    return Static(
        modification=Modification(
            kind="keywords_plus_hexproof",
            args=(m.group("lead").strip(), m.group("rhs").strip()),
        ),
        raw=raw,
    )


# --- "flying, hexproof from multicolored" — multi-keyword with color-class
@_sp(
    r"^(?P<lead>[a-z ,]+?), hexproof from (?P<class>multicolored|monocolored)\s*$"
)
def _kw_lead_hexproof_class(m, raw):
    return Static(
        modification=Modification(
            kind="keywords_plus_hexproof_class",
            args=(m.group("lead").strip(), m.group("class")),
        ),
        raw=raw,
    )


# --- "for each color among permanents you control" buffs ------------------
#
# "this creature gets +1/+1 for each color among permanents you control"
# "equipped creature gets +1/+1 for each color among permanents you control"
# "~ gets +1/+0 for each color among <filter>"
@_sp(
    r"^(?P<subj>~|this creature|equipped creature|enchanted creature) "
    r"gets \+(?P<p>\d+)/\+(?P<t>\d+) for each color among "
    r"(?P<scope>[^.]+?)\s*$"
)
def _buff_per_color_among(m, raw):
    return Static(
        modification=Modification(
            kind="buff_per_color_among",
            args=(m.group("subj"), int(m.group("p")), int(m.group("t")),
                  m.group("scope").strip()),
        ),
        raw=raw,
    )


# --- Vivid-style cost reduction: "this spell costs {N} less to cast for
#     each color among permanents you control" (ability-word prefix ok)
@_sp(
    r"^(?:[a-z ]+?-\s*)?this spell costs \{(?P<n>\d+)\} less to cast "
    r"for each color among (?P<scope>[^.]+?)\s*$"
)
def _cost_reduce_per_color_among(m, raw):
    return Static(
        modification=Modification(
            kind="cost_reduce_per_color_among",
            args=(int(m.group("n")), m.group("scope").strip()),
        ),
        raw=raw,
    )


# --- Converge: "converge - <body> for each color of mana spent to cast [it|this spell|~]"
# Treated as a Static so the whole ability counts as parsed; body retained
# verbatim for downstream typing.
@_sp(
    r"^converge\s*[-—]\s*(?P<body>.+?) for each color of mana spent to cast "
    r"(?:it|this spell|this creature|this enchantment|~)\s*$"
)
def _converge(m, raw):
    return Static(
        modification=Modification(
            kind="converge",
            args=(m.group("body").strip(),),
        ),
        raw=raw,
    )


# --- Sunburst rider on ETB: "~ enters with a +1/+1 counter on it for each
#     color of mana spent to cast it" (the word "sunburst" isn't in the text,
#     but the expanded rider is — parser treats it as a distinct shape).
@_sp(
    r"^(?:~|this creature|this artifact|this enchantment) enters with "
    r"(?P<n>\w+) (?P<kind>\+1/\+1|charge|crystal|[a-z]+) counters? on it "
    r"for each color of mana spent to cast (?:it|this spell|~)\s*$"
)
def _sunburst_etb(m, raw):
    return Static(
        modification=Modification(
            kind="sunburst_etb_counters",
            args=(m.group("n"), m.group("kind")),
        ),
        raw=raw,
    )


# --- Domain cost reduction (non-ability-word shape):
#     "this cost is reduced by {N} for each basic land type among lands you control"
@_sp(
    r"^this cost is reduced by \{(?P<n>\d+)\} for each basic land type "
    r"among lands you control\s*$"
)
def _domain_cost_reduce_explicit(m, raw):
    return Static(
        modification=Modification(
            kind="domain_cost_reduce",
            args=(int(m.group("n")),),
        ),
        raw=raw,
    )


# --- "all nonland permanents are the chosen color" ------------------------
@_sp(r"^all nonland permanents are the chosen color\s*$")
def _all_nonland_chosen(m, raw):
    return Static(
        modification=Modification(kind="all_nonland_chosen_color"),
        raw=raw,
    )


# --- "this artifact becomes the chosen color" -----------------------------
@_sp(
    r"^(?:this artifact|this creature|this permanent|~) becomes "
    r"the chosen color\s*$"
)
def _self_becomes_chosen(m, raw):
    return Static(
        modification=Modification(kind="self_becomes_chosen_color"),
        raw=raw,
    )


# --- "your opponents can't cast spells of the chosen color" ---------------
@_sp(r"^your opponents can'?t cast spells of the chosen color\s*$")
def _opp_cant_cast_chosen(m, raw):
    return Static(
        modification=Modification(kind="opp_cant_cast_chosen_color"),
        raw=raw,
    )


# --- "spend only mana of the chosen color to activate this ability" ------
@_sp(r"^spend only mana of the chosen color to activate this ability\s*$")
def _spend_only_chosen(m, raw):
    return Static(
        modification=Modification(kind="spend_only_chosen_color"),
        raw=raw,
    )


# --- "when you control no permanents of the chosen color, sacrifice ~" ---
# Triggered (state-based-esque); emit as a Triggered so the parser accepts it.
@_sp(
    r"^when you control no permanents of the chosen color, "
    r"sacrifice (?:this creature|~)\s*$"
)
def _no_chosen_sac(m, raw):
    return Triggered(
        trigger=Trigger(event="no_chosen_color_permanents"),
        effect=UnknownEffect(raw_text="sacrifice self"),
        raw=raw,
    )


# --- Spend-mana-any-color statics -----------------------------------------
# "you may spend mana as though it were mana of any color to cast ~ spells"
# "you may spend mana as though it were mana of any color to activate those abilities"
@_sp(
    r"^you may spend mana as though it were mana of any color "
    r"to (?P<what>cast [^.]+|activate those abilities|cast them)\s*$"
)
def _spend_any_color_static(m, raw):
    return Static(
        modification=Modification(
            kind="spend_any_color",
            args=(m.group("what").strip(),),
        ),
        raw=raw,
    )


# Bare form: "you may spend mana as though it were mana of any color"
@_sp(r"^you may spend mana as though it were mana of any color\s*$")
def _spend_any_color_bare(m, raw):
    return Static(
        modification=Modification(kind="spend_any_color_bare"),
        raw=raw,
    )


# --- "<subject> is the color of your choice" / "is the chosen color" -----
# choices.py already handles "~ is the chosen color"; we add "of your choice".
@_sp(
    r"^(?:~|this creature|this permanent|target permanent|that permanent|that spell) "
    r"(?:is|becomes) the colors? of your choice(?:[^.]*)?\s*$"
)
def _is_color_of_choice(m, raw):
    return Static(
        modification=Modification(kind="is_color_of_choice"),
        raw=raw,
    )


# --- Color-hosing protection — broad right-hand sides ---------------------
# "target creature has/gains protection from the color of your choice …"
@_sp(
    r"^(?P<subj>~|this creature|this permanent|target [a-z ]+?|"
    r"(?:creatures|permanents|artifacts|all creatures) you control|all creatures) "
    r"(?:has|have|gains?|gain) protection from "
    r"(?P<rhs>the color of your choice"
    r"|colorless or from the color of your choice"
    r"|artifacts or from the color of your choice"
    r"|the colors of permanents you control"
    r"|each color(?: with the most votes[^.]*)?"
    r"|each of your opponents"
    r"|each mana value among [^.]+"
    r"|its colors"
    r"|each of its colors"
    r"|that color"
    r")(?: until end of turn)?\s*$"
)
def _protection_broad(m, raw):
    return Static(
        modification=Modification(
            kind="protection_from_scope",
            args=(m.group("subj").strip(), m.group("rhs").strip()),
        ),
        raw=raw,
    )


# --- "all creatures have protection from <color>" -------------------------
@_sp(
    rf"^all creatures have protection from (?P<c>{_COLOR_OR_LESS})\s*$"
)
def _all_creatures_protection(m, raw):
    return Static(
        modification=Modification(
            kind="all_creatures_protection",
            args=(m.group("c"),),
        ),
        raw=raw,
    )


# --- "<colored>-creatures you control have protection from <color>" ------
# "white creatures you control have protection from black" (Circle-of-Protection
# tribal variant).
@_sp(
    rf"^(?P<own>{_COLOR_OR_LESS}) creatures you control "
    rf"have protection from (?P<other>{_COLOR_OR_LESS})\s*$"
)
def _own_color_creatures_prot(m, raw):
    return Static(
        modification=Modification(
            kind="own_color_creatures_protection",
            args=(m.group("own"), m.group("other")),
        ),
        raw=raw,
    )


# --- "protection from <color>, from <color>, and from <color>" — multi
# Used as a standalone ability body (Sliver Overlord–style riders).
@_sp(
    rf"^protection from (?P<a>{_COLOR}|vampires|werewolves|zombies)"
    rf"(?:, from (?P<b>{_COLOR}|vampires|werewolves|zombies))"
    rf"(?:, (?:and )?from (?P<c>{_COLOR}|vampires|werewolves|zombies))?\s*$"
)
def _protection_chain(m, raw):
    cols = [m.group("a"), m.group("b")]
    if m.group("c"):
        cols.append(m.group("c"))
    return Static(
        modification=Modification(
            kind="protection_chain",
            args=tuple(cols),
        ),
        raw=raw,
    )


# --- "the same is true for first strike, trample, and protection from any
#     color" — Odyssey-era rider on a prior clause. Emit as a structural
#     rider static so the ability parses rather than staying raw.
@_sp(
    r"^the same is true for (?P<list>[a-z ,]+?(?:and protection from any color)?)\s*$"
)
def _same_is_true_rider(m, raw):
    return Static(
        modification=Modification(
            kind="same_is_true_rider",
            args=(m.group("list").strip(),),
        ),
        raw=raw,
    )


# --- "each creature has protection from its colors" (Sliver Legion-ish)
@_sp(r"^each creature has protection from its colors\s*$")
def _each_creature_from_its_colors(m, raw):
    return Static(
        modification=Modification(kind="each_creature_protection_own_colors"),
        raw=raw,
    )


# --- "~ has protection from the chosen player" ----------------------------
@_sp(
    r"^(?:~|this creature|this permanent) has protection from "
    r"the chosen player\s*$"
)
def _self_protection_chosen_player(m, raw):
    return Static(
        modification=Modification(kind="self_protection_from_chosen_player"),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# EFFECT_RULES
# Body-of-effect patterns — used inside spells / activated abilities whose
# bodies reference color variables.
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "add an amount of mana of that color equal to your devotion to that color"
@_er(
    r"^add an amount of mana of that color equal to your devotion "
    r"to that color(?:\.|$)"
)
def _add_devotion_that_color(m):
    return UnknownEffect(raw_text="add_mana_eq_devotion_that_color")


# --- "where X is your devotion to <color>" rider on a preceding effect
@_er(
    rf"^(?:you gain|you lose|you draw|you mill) x [a-z ]+?, where x is "
    rf"your devotion to (?P<c>{_COLOR})(?:\.|$)"
)
def _x_eq_devotion(m):
    return UnknownEffect(raw_text=f"x_equals_devotion:{m.group('c')}")


# --- "creatures you control get +x/+x until end of turn, where x is your
#     devotion to <color>" (Nylea / Pharika-style team pump)
@_er(
    rf"^creatures you control get \+x/\+x until end of turn,"
    rf" where x is your devotion to (?P<c>{_COLOR})(?:\.|$)"
)
def _team_pump_eq_devotion(m):
    return UnknownEffect(
        raw_text=f"team_pump_x_equals_devotion:{m.group('c')}"
    )


# --- "~ deals damage to <filter> equal to your devotion to <color>"
@_er(
    rf"^~ deals damage to (?P<who>[^.]+?) equal to your devotion "
    rf"to (?P<c>{_COLOR})(?:\.|$)"
)
def _damage_eq_devotion(m):
    return UnknownEffect(
        raw_text=f"damage_eq_devotion:{m.group('c')}:{m.group('who').strip()}"
    )


# --- "you may spend mana as though it were mana of any color to cast <X>"
# as an activated/triggered body rider (distinct from the static form above).
@_er(
    r"^you may spend mana as though it were mana of any color "
    r"to (?P<what>cast [^.]+|cast it|cast that spell|cast them|"
    r"activate those abilities)(?:\.|$)"
)
def _spend_any_color_effect(m):
    return UnknownEffect(
        raw_text=f"spend_any_color:{m.group('what').strip()}"
    )


# --- "add one mana of any color for each <thing>" -------------------------
@_er(
    r"^add one mana of any color for each (?P<what>[^.]+?)(?:\.|$)"
)
def _add_any_color_per(m):
    return UnknownEffect(
        raw_text=f"add_any_color_per:{m.group('what').strip()}"
    )


# --- "target creature/permanent becomes the color[s] of your choice …" ---
@_er(
    r"^(?P<subj>target [a-z ]+?|any number of target creatures) "
    r"becomes? the colors? of your choice(?: until end of turn)?(?:\.|$)"
)
def _target_becomes_color(m):
    return UnknownEffect(
        raw_text=f"target_becomes_color_of_choice:{m.group('subj').strip()}"
    )


# --- "return all permanents of the color of your choice to their owners' hands"
@_er(
    r"^return all permanents of the color of your choice to "
    r"their owners' hands(?:\.|$)"
)
def _bounce_all_chosen_color(m):
    return UnknownEffect(raw_text="bounce_all_of_color_of_choice")


# --- "for each color, return up to one target [filter] card of that color
#     from your graveyard to your hand"
@_er(
    r"^for each color, return up to one target (?P<what>[a-z ]+?) card of "
    r"that color from your graveyard to your hand(?:\.|$)"
)
def _for_each_color_return(m):
    return UnknownEffect(
        raw_text=f"per_color_reanimate_to_hand:{m.group('what').strip()}"
    )


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — color-specific trigger starters
# ---------------------------------------------------------------------------
#
# NB: parser._TRIGGER_PATTERNS expects entries of the shape
# ``(compiled_regex, event_name, actor_kind)``. We add a handful of starters
# that the main list misses, all tied to color-cast / mana-produced events.

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # "whenever you cast a <color> spell"
    (re.compile(
        rf"^whenever you cast an? (?P<c>{_COLOR}|multicolored|monocolored) spell",
        re.I,
     ), "cast_color_spell", "self"),
    # "whenever an opponent casts a <color>/multicolored spell"
    (re.compile(
        rf"^whenever an opponent casts an? (?P<c>{_COLOR}|multicolored|monocolored) spell",
        re.I,
     ), "opp_cast_color_spell", "all"),
    # "whenever a basic land is tapped for mana of the chosen color"
    (re.compile(
        r"^whenever a basic land is tapped for mana of the chosen color",
        re.I,
     ), "chosen_color_mana_tapped", "all"),
    # "whenever a land's ability causes you to add one or more mana of the
    #  chosen color"
    (re.compile(
        r"^whenever a land'?s? ability causes you to add one or more mana "
        r"of the chosen color",
        re.I,
     ), "chosen_color_mana_added", "self"),
    # "whenever a permanent you control transforms into a phyrexian"
    (re.compile(
        r"^whenever a permanent you control transforms into a phyrexian",
        re.I,
     ), "transform_into_phyrexian", "self"),
    # "whenever a phyrexian you control dies"
    (re.compile(
        r"^whenever a phyrexian you control dies",
        re.I,
     ), "phyrexian_dies", "self"),
]


__all__ = [
    "STATIC_PATTERNS",
    "EFFECT_RULES",
    "TRIGGER_PATTERNS",
]
