#!/usr/bin/env python3
"""Mana cost dynamics extension.

Covers the family of rules centered on how spells are *paid for* — beyond the
plain-vanilla "pay the mana cost" path. Specifically:

- Alternative casts (``you may cast ~ for its <name> cost of {X}``; bare
  keyword-cost lines like ``evoke {R}`` / ``freerunning {1}{B}`` /
  ``foretell {2}{U}`` when they aren't swallowed by the global KEYWORD_RE).
- X-spell scaffolding (``X can't be <bound>``, ``X spells you cast cost {1}
  less to cast``, ``for each {1} spent on X, …``).
- Conditional and positional cost reduction (``spells you cast from X cost
  {N} less``, ``the first/second [filter] spell you cast each turn costs
  {N} less``, ``this spell costs {X} less to cast this way, where X is …``).
- Activated-ability cost reduction (``activated abilities of [filter] you
  control cost {N} less to activate``, ``equip/boast/ninjutsu abilities you
  activate cost {N} less to activate``).
- Snow-specific mana math (``scry X, where X is the amount of {S} spent to
  cast this spell``, ``add {C} for each {S} spent to cast this spell``, and
  the handful of snow-type state statics KEYWORD_RE misses such as
  ``enchanted land is snow`` / ``snow forestwalk``).
- Mana-source restrictions + mana-retention riders (``spend this mana only
  on costs that contain {X}``, ``you can't spend this mana to cast spells``,
  ``this mana doesn't empty as steps and phases end``).
- Mana refunds / ramp-via-payback (``add one mana of that color`` after a
  rider, ``whenever you discard one or more artifact cards, add {R}{R}``,
  ``add {R} for each [filter]``).
- Phyrexian-mana textual payment option (``you may pay 2 life rather than
  pay {W/P}``) — kept as a Static rider even though the ``{P}`` tokens
  themselves are already handled by ``parse_mana_cost``.
- ``rather than pay`` alternative-cost bodies (``pay 2 life rather than pay
  {W/P}``, ``tap three untapped dwarves you control rather than pay this
  spell's mana cost``).
- ``only <player> may activate this ability`` activation-rights gating,
  which comes up alongside alt-cost mana use (e.g. Propaganda-class
  oppositional activation).

Exports the usual triplet:
- ``STATIC_PATTERNS``: ability-shape statics.
- ``EFFECT_RULES``: body-of-effect phrasings used inside spells / abilities.
- ``TRIGGER_PATTERNS``: the one or two trigger starters that live here (e.g.
  "whenever you foretell a card").

Collision notes:
  * ``parser.py`` already handles:
      - ``this spell costs {N} less to cast`` (bare kicker-style).
      - ``this spell costs {x} less to cast, where x is …`` (variable).
      - ``spend this mana only to …`` (partial — we add ``…on costs that
        contain {X}`` and the ``can't spend this mana`` inverse).
      - ``X spells you cast cost {N} less`` (bare tribal shape).
      - ``until end of turn, you don't lose this mana`` (mana retention).
      - ``activate only as a sorcery`` / ``only once each turn``.
      - ``you may cast (~|this spell|that card|it) without paying its mana
        cost`` (via replacements.py).
      - ``this ability costs {N} less to activate for [X]``.
    We intentionally do NOT re-cover these — we pick up the adjacent
    phrasings the base parser leaves as parse errors.
  * ``color_devotion.py`` owns ``multicolored/monocolored spells cost {N}
    less to cast``. ``tribal_anthems`` (via parser core) owns bare tribal
    cost reduction. ``choices.py`` owns ``spells of the chosen type cost
    {N} less``. We intentionally don't shadow those.
  * ``partial_scrubber_2.py`` owns ``when you spend this mana to <X>``
    (triggered form) — we only add the *static* ``spend this mana only
    on costs that contain {X}`` direction.
  * ``KEYWORD_RE`` in parser.py matches ``evoke {X}`` / ``foretell {X}`` /
    ``boast {X}`` already, but *misses* ``freerunning {X}`` and the
    ``evoke-exile a <color> card from your hand`` body shape; we cover
    those here.
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
    Keyword, Modification, Static, Trigger, Triggered, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Shared regex fragments
# ---------------------------------------------------------------------------

# Any mana-symbol sequence: {1}, {X}, {W}, {W/P}, {2/G}, etc. Kept permissive.
_MC = r"(?:\{[^}]+\})+"

# Filter fragments used in many "spells you cast" / "abilities of" shapes.
_SPELL_FILT = r"[a-z0-9 ,/'\-]+?"


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# === Alternative-cast lines =================================================

# Bare keyword-cost lines KEYWORD_RE misses.
#   freerunning {1}{b}
#   freerunning {x}{r}
@_sp(rf"^freerunning\s+(?P<cost>{_MC})\s*$")
def _freerunning_cost(m, raw):
    return Keyword(name="freerunning", args=(m.group("cost"),), raw=raw)


# "freerunning - <body>" (Murders at Karlov Manor rider explaining the alt
# cost body rather than the cost itself).
@_sp(r"^freerunning\s*[-—]\s*(?P<body>.+?)\s*$")
def _freerunning_body(m, raw):
    return Keyword(
        name="freerunning",
        args=(m.group("body").strip(),),
        raw=raw,
    )


# "evoke - <body>" / "evoke-exile a <color> card from your hand" (Eventide)
@_sp(
    r"^evoke\s*[-—]\s*(?P<body>exile an? [a-z ]+? card from your hand|"
    r".+?)\s*$"
)
def _evoke_body(m, raw):
    return Keyword(
        name="evoke",
        args=(m.group("body").strip(),),
        raw=raw,
    )


# Bare "evoke {cost}" fallback (should usually be handled by KEYWORD_RE,
# but we catch fragments that land here as parse errors).
@_sp(rf"^evoke\s+(?P<cost>{_MC})\s*$")
def _evoke_cost(m, raw):
    return Keyword(name="evoke", args=(m.group("cost"),), raw=raw)


# "you may cast ~ for its <name> cost of {X}" — prowl/surge/foretell-cast
# "you may cast this card from exile for its foretell cost"
@_sp(
    r"^you may cast (?:~|this card|this spell|the exiled card) "
    r"(?:from (?:your hand|exile|your graveyard|anywhere)\s+)?"
    r"(?:for|by paying|using) its "
    r"(?P<name>[a-z ]+?)\s+cost"
    r"(?:\s+(?:of|\s+)\s*(?P<cost>" + _MC + r"))?\s*$"
)
def _cast_for_named_cost(m, raw):
    return Static(
        modification=Modification(
            kind="alt_cast_named_cost",
            args=(m.group("name").strip(), (m.group("cost") or "").strip()),
        ),
        raw=raw,
    )


# "its foretell cost is equal to its mana cost reduced by {N}"
# "its foretell cost is its mana cost reduced by {N}"
@_sp(
    r"^its (?P<name>foretell|flashback|dash|embalm|aftermath|kicker|overload)"
    r"\s+cost is(?: equal to)? its mana cost reduced by "
    r"(?P<amt>\{[^}]+\})\s*$"
)
def _named_cost_eq_mc_reduced(m, raw):
    return Static(
        modification=Modification(
            kind="named_cost_eq_mc_reduced",
            args=(m.group("name"), m.group("amt")),
        ),
        raw=raw,
    )


# "the flashback cost is equal to its mana cost"
# "the flashback cost is equal to that card's mana cost"
@_sp(
    r"^the (?P<name>flashback|foretell|dash|embalm|aftermath|kicker|overload|"
    r"unearth|madness|retrace|jump-start) cost is (?:equal to )?"
    r"(?P<ref>its|that card'?s?|this card'?s?) mana cost\s*$"
)
def _named_cost_eq_mc(m, raw):
    return Static(
        modification=Modification(
            kind="named_cost_eq_mc",
            args=(m.group("name"), m.group("ref").strip()),
        ),
        raw=raw,
    )


# "the next <filter> spell you cast (this turn) costs {N} less to cast"
@_sp(
    r"^the next (?P<filt>" + _SPELL_FILT + r")? ?spells? (?:you cast )?"
    r"(?:this turn )?costs? (?P<amt>\{[^}]+\}) less to cast"
    r"(?:\s+for each (?P<per>[^.]+?))?\s*$"
)
def _next_spell_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="next_spell_cost_reduce",
            args=(
                (m.group("filt") or "").strip(),
                m.group("amt"),
                (m.group("per") or "").strip(),
            ),
        ),
        raw=raw,
    )


# "that copy costs {N} less to cast"
@_sp(
    r"^that (?:copy|card) costs (?P<amt>\{[^}]+\}) less to cast\s*$"
)
def _that_copy_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="that_copy_cost_reduce",
            args=(m.group("amt"),),
        ),
        raw=raw,
    )


# "it also costs {N} less to cast if <cond>"
@_sp(
    r"^it also costs (?P<amt>\{[^}]+\}) less to cast "
    r"(?P<cond>if [^.]+|for each [^.]+)\s*$"
)
def _it_also_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="it_also_cost_reduce",
            args=(m.group("amt"), m.group("cond").strip()),
        ),
        raw=raw,
    )


# "this spell costs {U}{U}{U} less to cast if <cond>" / "costs {U} less for
# each …" — colored reduction with conditional rider.
@_sp(
    r"^this spell costs (?P<amt>(?:\{[wubrgcx0-9/]+\})+) less to cast "
    r"(?P<cond>if [^.]+|for each [^.]+)\s*$"
)
def _this_spell_colored_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="this_spell_colored_cost_reduce",
            args=(m.group("amt"), m.group("cond").strip()),
        ),
        raw=raw,
    )


# "foretelling cards from your hand costs {1} less and can be done on any
# player's turn"
@_sp(
    r"^foretelling cards from your hand costs (?P<amt>\{\d+\}) less"
    r"(?P<rest>[^.]*)?\s*$"
)
def _foretell_cheaper(m, raw):
    return Static(
        modification=Modification(
            kind="foretell_discount",
            args=(m.group("amt"), (m.group("rest") or "").strip()),
        ),
        raw=raw,
    )


# "each nonland card in your hand without foretell has foretell"
@_sp(
    r"^each (?P<scope>nonland card|card) in your hand(?: without foretell)?"
    r" has foretell\s*$"
)
def _grant_foretell(m, raw):
    return Static(
        modification=Modification(
            kind="grant_foretell_in_hand",
            args=(m.group("scope"),),
        ),
        raw=raw,
    )


# "the first card you foretell each turn costs {0} to foretell"
@_sp(
    r"^the first card you foretell each turn costs (?P<amt>\{[^}]+\}) "
    r"to foretell\s*$"
)
def _first_foretell_cost(m, raw):
    return Static(
        modification=Modification(
            kind="first_foretell_cost",
            args=(m.group("amt"),),
        ),
        raw=raw,
    )


# === X-spell scaffolding =====================================================

# "X can't be <bound>" — upper bounds and "greater than …" riders beyond
# the parser-native "X can't be 0".
@_sp(
    r"^x can'?t be "
    r"(?P<body>(?:greater than |less than |more than )?[^.]+)\s*$"
)
def _x_bound(m, raw):
    body = m.group("body").strip()
    return Static(
        modification=Modification(kind="x_bound", args=(body,)),
        raw=raw,
    )


# "for each {1} spent on X, <body>" / "for each {1} spent to cast ~, <body>"
@_sp(
    r"^for each (?P<atom>\{\d+\}|\{[a-z]\}|\{s\}) spent "
    r"(?:on x|to cast (?:~|this spell|it)),\s*(?P<body>.+?)\s*$"
)
def _for_each_unit_spent(m, raw):
    return Static(
        modification=Modification(
            kind="for_each_unit_spent",
            args=(m.group("atom"), m.group("body").strip()),
        ),
        raw=raw,
    )


# === Cost reduction — positional / conditional ==============================

# "the first/second/third <filter> spell you cast each turn costs {N} less
# to cast"   (supports "your" / "you" variants and "<filter> spell[s]")
@_sp(
    r"^the (?P<which>first|second|third|next) "
    r"(?P<filt>" + _SPELL_FILT + r")?\s*"
    r"spells? (?:you cast |each player casts )?each turn "
    r"(?:cost|costs) (?P<amt>\{[^}]+\}) less to cast\s*$"
)
def _positional_spell_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="positional_spell_cost_reduce",
            args=(
                m.group("which"),
                (m.group("filt") or "").strip(),
                m.group("amt"),
            ),
        ),
        raw=raw,
    )


# "the first/second <filter> spell you cast each turn has <keyword>"
#   (same positional family but granting an ability instead of discount)
@_sp(
    r"^the (?P<which>first|second|third) "
    r"(?P<filt>" + _SPELL_FILT + r")?\s*"
    r"spells? you cast each turn has (?P<kw>[a-z0-9 ,{}/]+?)\s*$"
)
def _positional_spell_grant(m, raw):
    return Static(
        modification=Modification(
            kind="positional_spell_grant",
            args=(
                m.group("which"),
                (m.group("filt") or "").strip(),
                m.group("kw").strip(),
            ),
        ),
        raw=raw,
    )


# "spells you cast from <zone> cost {N} less to cast"
#   zones covered: your graveyard, exile, anywhere other than your hand,
#   your graveyard or from exile, etc.
@_sp(
    r"^(?P<filt>" + _SPELL_FILT + r"\s*)?spells you cast "
    r"from (?P<zone>your graveyard|exile|your graveyard or from exile|"
    r"anywhere other than your hand|your graveyard or exile) "
    r"(?:cost|costs) (?P<amt>\{[^}]+\}) less to cast\s*$"
)
def _zone_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="zone_cost_reduce",
            args=(
                (m.group("filt") or "").strip(),
                m.group("zone").strip(),
                m.group("amt"),
            ),
        ),
        raw=raw,
    )


# "<filter> spells you cast <rider> cost {N} less to cast"
#   (covers "creature spells with flying you cast cost {1} less",
#   "permanent spells you cast that have an adventure cost {1} less",
#   "spells you cast with mana value 5 or greater cost {1} less", etc.)
# Keep the left-hand side narrow so we don't swallow the base parser's
# "X spells you cast cost {N} less" — require either a "with", "that have",
# or "with mana value" rider in between.
@_sp(
    r"^(?P<filt>[^.]+?)\s+(?:with |that have |that target )"
    r"(?P<rider>[^.]+?)\s+(?:cost|costs) (?P<amt>\{[^}]+\}) less to cast\s*$"
)
def _filtered_rider_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="filtered_rider_cost_reduce",
            args=(
                m.group("filt").strip(),
                m.group("rider").strip(),
                m.group("amt"),
            ),
        ),
        raw=raw,
    )


# "this spell costs {X} less to cast this way, where X is …"
#   (companion-style / commander-tax-alt)
@_sp(
    r"^this spell costs (?P<amt>\{x\}|\{\d+\}) less to cast this way"
    r"(?:, where x is (?P<where>[^.]+))?\s*$"
)
def _this_way_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="this_way_cost_reduce",
            args=(m.group("amt"), (m.group("where") or "").strip()),
        ),
        raw=raw,
    )


# "~ spell costs {N} less to cast if it targets …" — name-prefixed form.
@_sp(
    r"^~\s+spell costs (?P<amt>\{[^}]+\}) less to cast "
    r"(?P<cond>if [^.]+|for each [^.]+)\s*$"
)
def _self_spell_cost_reduce_cond(m, raw):
    return Static(
        modification=Modification(
            kind="self_spell_cost_reduce_cond",
            args=(m.group("amt"), m.group("cond").strip()),
        ),
        raw=raw,
    )


# "spells that target this creature cost {N} less to cast" (Shadowspear-ish)
@_sp(
    r"^spells that target (?P<tgt>this creature|this permanent|~|"
    r"enchanted creature|equipped creature) cost (?P<amt>\{[^}]+\}) "
    r"less to cast\s*$"
)
def _spells_targeting_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="spells_targeting_cost_reduce",
            args=(m.group("tgt"), m.group("amt")),
        ),
        raw=raw,
    )


# "minotaur spells you cast cost {B}{R} less to cast" — colored-mana reduction
# (Didgeridoo-class is keyword-free, but the colored reduction doesn't fit
# the integer-only parser-native "X spells you cast cost {N} less" regex).
@_sp(
    r"^(?P<filt>[a-z][a-z ,]+?) spells you cast cost "
    r"(?P<amt>(?:\{[wubrgc0-9x/]+\})+) less to cast\s*$"
)
def _colored_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="colored_cost_reduce",
            args=(m.group("filt").strip(), m.group("amt")),
        ),
        raw=raw,
    )


# === Activated-ability cost reduction ========================================

# "activated abilities of <filter> you control cost {N} less to activate"
@_sp(
    r"^activated abilities of (?P<filt>[^.]+?) you control "
    r"cost (?P<amt>\{[^}]+\}) less to activate"
    r"(?:\.|$|\s*$)"
)
def _activated_abilities_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="activated_abilities_cost_reduce",
            args=(m.group("filt").strip(), m.group("amt")),
        ),
        raw=raw,
    )


# "<named> abilities you activate cost {N} less to activate"
#   covers equip, boast, ninjutsu, cycling, channel, morph, etc.
@_sp(
    r"^(?P<kind>equip|boast|ninjutsu|cycling|channel|morph|unearth|flashback|"
    r"disturb|commander ninjutsu|level up|fortify|reconfigure|crew|"
    r"activated) abilities you activate "
    r"(?:that target (?P<tgt>[^.]+?) )?"
    r"cost (?P<amt>\{[^}]+\}) less to activate"
    r"(?:\s+for each (?P<per>[^.]+?))?\s*$"
)
def _named_ability_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="named_ability_cost_reduce",
            args=(
                m.group("kind"),
                (m.group("tgt") or "").strip(),
                m.group("amt"),
                (m.group("per") or "").strip(),
            ),
        ),
        raw=raw,
    )


# "this equipment's equip ability costs {N} less to activate if it targets X"
@_sp(
    r"^this (?:equipment's|creature's|permanent's) "
    r"(?P<kind>equip|boast|cycling|channel|morph|activated) ability "
    r"costs (?P<amt>\{[^}]+\}) less to activate "
    r"(?P<cond>if [^.]+|for each [^.]+)\s*$"
)
def _self_ability_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="self_ability_cost_reduce",
            args=(m.group("kind"), m.group("amt"), m.group("cond").strip()),
        ),
        raw=raw,
    )


# "this ability costs {N} less to activate if <cond>"
@_sp(
    r"^this ability costs (?P<amt>\{[^}]+\}) less to activate "
    r"(?P<cond>if [^.]+)\s*$"
)
def _this_ability_cost_reduce_if(m, raw):
    return Static(
        modification=Modification(
            kind="this_ability_cost_reduce_if",
            args=(m.group("amt"), m.group("cond").strip()),
        ),
        raw=raw,
    )


# "this ability costs {X} less to activate, where X is <expr>"
@_sp(
    r"^this ability costs (?P<amt>\{x\}|\{\d+\}) less to activate,\s*"
    r"where x is (?P<where>[^.]+)\s*$"
)
def _this_ability_cost_reduce_var(m, raw):
    return Static(
        modification=Modification(
            kind="this_ability_cost_reduce_var",
            args=(m.group("amt"), m.group("where").strip()),
        ),
        raw=raw,
    )


# === Affinity / convoke / improvise granting =================================

# "<filter> spells you cast have affinity for <X>"
@_sp(
    r"^(?P<filt>[^.]+? )?spells you cast "
    r"have affinity for (?P<rhs>[^.]+?)\s*$"
)
def _grant_affinity(m, raw):
    return Static(
        modification=Modification(
            kind="grant_affinity",
            args=((m.group("filt") or "").strip(), m.group("rhs").strip()),
        ),
        raw=raw,
    )


# "<filter> spells you cast have convoke"
@_sp(
    r"^(?P<filt>[^.]+? )?spells you cast "
    r"have (?P<kw>convoke|improvise|delve|undaunted|cascade|discover \d+)\s*$"
)
def _grant_cost_helper(m, raw):
    return Static(
        modification=Modification(
            kind="grant_cost_helper",
            args=((m.group("filt") or "").strip(), m.group("kw").strip()),
        ),
        raw=raw,
    )


# "the first <filter> spell you cast each turn has convoke"
# (redundant-safe — positional_spell_grant above also matches, but this is
# the more targeted "cost helper" case so we parse it first for clarity.)
# [No new regex — _positional_spell_grant already covers it.]


# "spells you cast have freerunning {cost}" — (rare — e.g. an anthem that
# grants alternative-cost keywords).
@_sp(
    r"^(?P<filt>[^.]+? )?spells you cast "
    r"have (?P<kw>freerunning|foretell|evoke|kicker|overload|dash|retrace|"
    r"flashback|cascade|prowl|surge|embalm|jump-start|madness) "
    r"(?P<cost>" + _MC + r")\s*$"
)
def _grant_alt_cost_keyword(m, raw):
    return Static(
        modification=Modification(
            kind="grant_alt_cost_keyword",
            args=(
                (m.group("filt") or "").strip(),
                m.group("kw"),
                m.group("cost"),
            ),
        ),
        raw=raw,
    )


# === Snow-type statics =======================================================

# "snow forestwalk" / "snow plainswalk" etc. — the "snow" adjective makes it
# miss the plain landwalk keyword regex in most runs.
@_sp(
    r"^snow (?P<w>forestwalk|plainswalk|islandwalk|swampwalk|mountainwalk|"
    r"landwalk)\s*$"
)
def _snow_landwalk(m, raw):
    return Keyword(name="snow " + m.group("w"), raw=raw)


# "<subject> is snow" / "~ is snow" / "enchanted land is snow"
@_sp(
    r"^(?P<subj>~|this creature|this land|this permanent|enchanted land|"
    r"enchanted creature|target land|permanents? with [a-z ]+ counters? on "
    r"(?:it|them)) (?:is|are) snow\s*$"
)
def _is_snow(m, raw):
    return Static(
        modification=Modification(
            kind="becomes_snow",
            args=(m.group("subj").strip(),),
        ),
        raw=raw,
    )


# "all lands are no longer snow" / "no permanent is snow"
@_sp(r"^all lands are no longer snow\s*$")
def _no_longer_snow(m, raw):
    return Static(
        modification=Modification(kind="no_longer_snow"),
        raw=raw,
    )


# "snow lands your opponents control enter tapped"
@_sp(
    r"^snow (?P<kind>lands|creatures|permanents) your opponents control "
    r"enter tapped\s*$"
)
def _opp_snow_etb_tapped(m, raw):
    return Static(
        modification=Modification(
            kind="opp_snow_etb_tapped",
            args=(m.group("kind"),),
        ),
        raw=raw,
    )


# === Mana-source / retention / restriction statics ==========================

# "spend this mana only on costs that contain {X}"
# Trailing punctuation (e.g. stray `"`) is stripped.
@_sp(
    r"^spend this mana only on costs that contain (?P<what>[^.\"']+)\s*[\"']?\s*$"
)
def _spend_on_costs_with(m, raw):
    return Static(
        modification=Modification(
            kind="spend_only_on_costs_with",
            args=(m.group("what").strip(),),
        ),
        raw=raw,
    )


# "you can't spend this mana to cast spells"
@_sp(r"^you can'?t spend this mana to cast (?P<what>[^.]+?)\s*$")
def _cant_spend_for(m, raw):
    return Static(
        modification=Modification(
            kind="cant_spend_this_mana_for",
            args=(m.group("what").strip(),),
        ),
        raw=raw,
    )


# "this mana doesn't empty from your [mana] pool as steps and phases end"
@_sp(
    r"^(?:this mana|the mana|that mana) doesn'?t empty "
    r"(?:from your (?:mana )?pool )?as (?:steps and phases|phases and steps) end\s*$"
)
def _mana_doesnt_empty(m, raw):
    return Static(
        modification=Modification(kind="mana_doesnt_empty"),
        raw=raw,
    )


# === Activation-rights (sit alongside mana-source gating) ===================

# "only your opponents may activate this ability [and only as a sorcery]"
# "only the player this creature is attacking may activate this ability …"
# "only <subject> may activate this ability …"
@_sp(
    r"^only (?P<who>[^.]+?) may activate this ability"
    r"(?:\s+and only (?P<timing>[^.]+?))?\s*$"
)
def _only_who_activate(m, raw):
    return Static(
        modification=Modification(
            kind="only_who_activate",
            args=(
                m.group("who").strip(),
                (m.group("timing") or "").strip(),
            ),
        ),
        raw=raw,
    )


# "you may also activate this ability while ~ is in your graveyard"
# "you may also activate this ability if ~ is in your graveyard"
@_sp(
    r"^you may also activate this ability "
    r"(?:while|if) (?P<cond>[^.]+?)\s*$"
)
def _may_also_activate(m, raw):
    return Static(
        modification=Modification(
            kind="may_also_activate",
            args=(m.group("cond").strip(),),
        ),
        raw=raw,
    )


# "you can't activate this ability during combat" / "…during an opponent's turn"
@_sp(
    r"^you can'?t activate this ability (?P<when>during [^.]+?)\s*$"
)
def _cant_activate_when(m, raw):
    return Static(
        modification=Modification(
            kind="cant_activate_when",
            args=(m.group("when").strip(),),
        ),
        raw=raw,
    )


# === Rather-than-pay alternative costs =======================================

# "you may pay X rather than pay [this spell's mana cost|the <kw> cost of …]"
@_sp(
    r"^you may pay (?P<alt>[^.]+?) rather than pay "
    r"(?P<target>this spell'?s mana cost|its mana cost|the \w+ cost[^.]+?|"
    r"that mana|the equip cost[^.]+)\s*$"
)
def _rather_than_pay(m, raw):
    return Static(
        modification=Modification(
            kind="rather_than_pay",
            args=(m.group("alt").strip(), m.group("target").strip()),
        ),
        raw=raw,
    )


# "you may tap three untapped dwarves you control rather than pay this
# spell's mana cost" — "tap Xs" alt-cost form.
@_sp(
    r"^you may tap (?P<what>[^.]+?) rather than pay "
    r"(?P<target>this spell'?s mana cost|its mana cost|the \w+ cost[^.]+)\s*$"
)
def _rather_than_pay_tap(m, raw):
    return Static(
        modification=Modification(
            kind="rather_than_pay_tap",
            args=(m.group("what").strip(), m.group("target").strip()),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- X = amount of {S} spent / amount of {C} spent (snow & colorless X) ----

# "scry X, where X is the amount of {S} spent to cast this spell, then draw …"
@_er(
    r"^scry x,\s*where x is the amount of \{s\} spent to cast "
    r"(?:this spell|~|it)(?:,\s*.+?)?(?:\.|$)"
)
def _scry_x_eq_snow(m):
    return UnknownEffect(raw_text="scry_x_eq_snow_spent")


# "distribute X +1/+1 counters among any number of creatures you control,
# where X is the amount of {S} spent to cast this spell"
@_er(
    r"^distribute x \+1/\+1 counters among [^.]+,\s*where x is "
    r"the amount of \{s\} spent to cast (?:this spell|~|it)(?:\.|$)"
)
def _distribute_x_eq_snow(m):
    return UnknownEffect(raw_text="distribute_x_eq_snow_spent")


# "return a … card with mana value X or less … where X is the amount of
# {S} spent to cast this spell"  (generic "where X is the amount of {S}" tail)
@_er(
    r"^(?:then )?(?:return|put|exile|draw) [^.]+?,\s*"
    r"where x is the amount of \{s\} spent to cast (?:this spell|~|it)"
    r"(?:\.|$)"
)
def _generic_x_eq_snow(m):
    return UnknownEffect(raw_text="effect_x_eq_snow_spent")


# "add {C} for each {S} spent to cast this spell"
@_er(
    r"^add (?P<atom>\{[wubrgc]\}(?:\{[wubrgc]\})*) for each \{s\} spent "
    r"to cast (?:this spell|~|it)(?:\.|$)"
)
def _add_per_snow_spent(m):
    return UnknownEffect(
        raw_text=f"add_{m.group('atom')}_per_snow_spent"
    )


# --- Generic "add <atom> for each <X>" refund bodies -----------------------

# "add {R} for each card in your hand"
# "add {B} for each creature card in your graveyard"
# "add {R} for each goblin on the battlefield"
@_er(
    r"^add (?P<atom>\{[wubrgc]\}(?:\{[wubrgc]\})*) for each "
    r"(?P<what>[^.]+?)(?:\.|$)"
)
def _add_per_count(m):
    return UnknownEffect(
        raw_text=f"add_{m.group('atom')}_per:{m.group('what').strip()}"
    )


# "add one mana of that color" (dangling ramp tail)
# "add one mana of that color for each charge counter on this artifact"
# "add one mana of that color unless any player pays {1}"
@_er(
    r"^add one mana of that color"
    r"(?:\s+for each (?P<per>[^.]+?))?"
    r"(?:\s+unless (?P<unless>[^.]+?))?\s*(?:\.|$)"
)
def _add_one_of_that_color(m):
    return UnknownEffect(
        raw_text=(
            "add_one_of_that_color"
            + (f":per={m.group('per').strip()}" if m.group("per") else "")
            + (f":unless={m.group('unless').strip()}" if m.group("unless") else "")
        )
    )


# "add two mana of any one color" / "add two mana in any combination of colors"
@_er(
    r"^add (?P<n>two|three|four|five|x|\d+) mana "
    r"(?P<body>of any one color|in any combination of colors|"
    r"of any colors?|of one color of your choice|"
    r"of any combination of \{w\}[^.]+)(?:\.|$)"
)
def _add_n_mana_any(m):
    return UnknownEffect(
        raw_text=f"add_{m.group('n')}_{m.group('body').strip()}"
    )


# --- Phyrexian-mana textual payment option ---------------------------------

# "you may pay 2 life rather than pay {W/P}" (the parser catches the symbol,
# but the whole clause as a static body is unparsed in some places).
@_er(
    r"^you may pay (?P<amt>\d+) life rather than pay "
    r"(?P<atom>\{[wubrg]/p\}(?:\{[wubrg]/p\})*)(?:\.|$)"
)
def _pay_life_rather_than_p_mana(m):
    return UnknownEffect(
        raw_text=(
            f"pay_life_rather_than_phyrexian:"
            f"{m.group('amt')}:{m.group('atom')}"
        )
    )


# --- "Rather than pay <...>" as an effect body ------------------------------

# "rather than pay {2} for each previous time you've cast this spell from the
# command zone this game, pay 2 life that many times"
@_er(
    r"^rather than pay (?P<orig>\{[^}]+\}(?:\{[^}]+\})*) for each "
    r"(?P<per>[^,]+?),\s*(?P<sub>pay [^.]+?)(?:\.|$)"
)
def _rather_than_pay_per_times(m):
    return UnknownEffect(
        raw_text=(
            f"rather_than_pay_{m.group('orig')}_per:"
            f"{m.group('per').strip()}|sub={m.group('sub').strip()}"
        )
    )


# "you may cast that card by paying life equal to the spell's mana value
# rather than paying its mana cost"
@_er(
    r"^you may cast (?P<what>that card|it|the exiled card) by paying "
    r"(?P<alt>[^.]+?) rather than paying its mana cost(?:\.|$)"
)
def _cast_by_paying_alt(m):
    return UnknownEffect(
        raw_text=(
            f"cast_by_paying_alt:{m.group('what').strip()}|"
            f"alt={m.group('alt').strip()}"
        )
    )


# --- Ramp-via-payback: "add {R}{R}" after a trigger body --------------------

# Catches the bare "add {...}" form when it lands as its own sentence
# (Storm Cauldron / Mana Reflection class).
@_er(
    r"^add (?P<atom>\{[wubrgcsx0-9/]+\}(?:\{[wubrgcsx0-9/]+\})*)(?:\.|$)"
)
def _bare_add_atoms(m):
    return UnknownEffect(raw_text=f"add_{m.group('atom')}")


# --- Eldrazi-scion-style baked mana: "sacrifice this creature: add {C}" ----
# Already handled structurally by the activated-ability parser; we add a
# body-shape rule so any leftover "…: add {C}" body-fragment lands as a
# recognized shape rather than a parse error.
@_er(
    r"^sacrifice (?:this creature|~|this permanent):\s*add "
    r"(?P<atom>\{[wubrgc]\})(?:\.|$)"
)
def _sac_for_mana(m):
    return UnknownEffect(
        raw_text=f"sac_self_for_{m.group('atom')}"
    )


# --- Mana-ability conversion (Mana Flare-ish riders) ------------------------
# "until end of turn, any time you could activate a mana ability, you may
# pay 1 life. if you do, add {C}"
@_er(
    r"^until end of turn,\s*any time you could activate a mana ability,\s*"
    r"you may pay (?P<alt>[^.]+?)\.\s*if you do, add "
    r"(?P<atom>\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)"
)
def _anytime_mana_ability(m):
    return UnknownEffect(
        raw_text=(
            f"anytime_mana_ability:alt={m.group('alt').strip()}|"
            f"add={m.group('atom')}"
        )
    )


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # "whenever you foretell a card"
    (re.compile(r"^whenever you foretell a card", re.I),
     "foretell_card", "self"),
    # "whenever you cast a spell with <alternative cost>" (prowl/surge-cast)
    (re.compile(
        r"^whenever you cast a spell (?:for|with|using) its "
        r"(?:prowl|surge|foretell|evoke|flashback|dash|madness|"
        r"overload|spectacle|escape|adventure|cleave) cost",
        re.I,
     ), "cast_spell_alt_cost", "self"),
    # "whenever you cast a spell with {X} in its mana cost"
    (re.compile(
        r"^whenever you cast a spell with \{x\} in its mana cost",
        re.I,
     ), "cast_x_spell", "self"),
    # "whenever you spend one or more {S} to cast a spell" — snow payoff
    (re.compile(
        r"^whenever you spend one or more \{s\} to cast a spell",
        re.I,
     ), "snow_spent_cast", "self"),
    # "whenever you discard one or more artifact cards, add {R}{R}"
    # (this is actually a triggered mana ability; registering the starter
    # so the trigger body gets a typed Trigger event.)
    (re.compile(
        r"^whenever you discard one or more (?P<filt>[a-z ]+?) cards?",
        re.I,
     ), "discard_filter_cards", "self"),
]


__all__ = [
    "STATIC_PATTERNS",
    "EFFECT_RULES",
    "TRIGGER_PATTERNS",
]
