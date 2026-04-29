#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (sixth pass).

Family: PARTIAL -> GREEN promotions. Companion to ``partial_scrubber.py``
through ``partial_scrubber_5.py``; targets single-ability clusters that
survived all five prior passes. Patterns were picked by re-bucketing
PARTIAL parse_errors after scrubber #5 shipped (3,556 PARTIAL cards,
~4,010 residual error strings, 11.24% PARTIAL), then keeping the top
~25 clusters (>=3 hits each) that map cleanly onto static regex and
aren't already subsumed by any prior scrubber.

Highlights of the remaining long tail:

- **Suspend with cost spec**: ``suspend N-{cost}`` (64!). The single
  biggest cluster left; prior passes had bare ``suspend`` as a keyword
  but never parsed the ``suspend 3-{2}{u}`` shape (Lotus Bloom, Wheel
  of Fate, Profane Tutor, etc.) produced by normalize collapsing the
  em-dash.
- **Multikicker with cost** (18): ``multikicker {1}{u}``, ``multikicker
  {2}``. Base KEYWORD_RE never listed multikicker; scrubber #2's
  ``_MULTIPIP_KEYWORDS`` missed it too. Also handles the single-pip
  shape (regex requires 2+ pips).
- **Single-pip megamorph** (3): ``megamorph {r}``, ``megamorph {u}``.
  Scrubber #2 listed megamorph in multipip but its regex requires two
  or more pip groups so single-color was still failing.
- **Bloodthirst X** (2): base regex had ``bloodthirst \\d+`` but no X.

Static/rule clusters:
- ``activate no more than (twice|three times|\\w+ times) each turn`` (8)
  — Manaforge Cinder / Pit Imp restriction-on-activated-abilities rule.
- ``you lose half your life, rounded up`` (4) — Infernal Contract /
  Doomsday cost-after-resolution rider.
- ``target player takes an extra turn after this one`` (3) — Walk the
  Aeons (bare extra-turn-granter).
- ``reveal cards from the top of your library until ...`` (3) — Abundant
  Harvest-style draw-until effect (top-level umbrella line).
- ``switch target creature's power and toughness until end of turn`` (3)
  — Twisted Image / Strange Inversion umbrella.
- ``at this turn's next end of combat, <effect>`` (4) — Triton Tactics
  /Glyph of Doom timing.
- ``target creature gains trample and gets +X/+0 until end of turn,
  where X is ...`` (3) — Berserk family.
- ``target opponent may draw a card`` (3) — Phelddagrif-tail.
- ``target player reveals three cards from their hand [and you choose
  one of them]`` (3) — Blackmail-class.
- ``each player sacrifices a creature of their choice`` (3) — Reign of
  the Pit.
- ``you and that player each <effect>`` (4) — Benevolent/Sylvan Offering
  symmetric give.
- ``those creatures can't block this turn`` (3) — Wrap in Flames-tail.
- ``creatures your opponents control can't block this turn`` (2) —
  Cosmotronic Wave opp-side.
- ``each opponent exiles the top card of their library [tail]`` (6) —
  Fathom Feeder / Wand of Wonder draw-ring.
- ``populate`` bare (2) — Rootborn Defenses token-copier keyword.
- ``you become the monarch`` bare (2) — Monarch-grant line.
- ``choose up to five {p} worth of modes`` (5) — Season-of-* spree
  header.
- ``you don't lose unspent (color) mana as steps and phases end`` (5) —
  Omnath/Ashling mana-retention rule.
- ``you may look at that card for as long as it remains exiled`` (3) —
  Hauken tail.
- ``you may look at cards exiled with this creature`` (3) — Bane Alley
  Broker-class peek rider.
- ``you may look at / cast the exiled card [this turn]`` (4) — generic
  cast-exiled permission.
- ``put a counter of that kind on <target>`` (3) — Crystalline Giant
  counter-copy tail.
- ``put those land cards onto the battlefield tapped and the rest on
  the bottom ...`` (3) — Ring Goes South / Open the Way tutor tail.
- ``put up to two of them into your hand and the rest into/on ...`` (3)
  — Shadow Prophecy-tail.
- ``put the rest on the bottom in a random order`` (3) — naked Ojer/
  Kaslem tail not caught by scrubber #5's rest-bottom (which required
  "of your library").
- ``return the exiled cards to the battlefield [tapped] under their
  owners' control`` (3) — Hide on the Ceiling blink-tail.
- ``counter that spell unless its controller pays {N}`` (3) — classic
  Forbid/Mana Leak tail.
- ``the replicate cost is equal to its mana cost`` (3) — Djinn
  Illuminatus flavor.
- ``discard that card`` bare (3) — Mothlight Processionist tail.
- ``sacrifice a creature|land`` bare (7) — multi-line sacrifice cost
  that slipped past the cost-parser.
- ``before you shuffle your deck to start the game, ...`` (3) — Conspiracy
  draft-sideboard rider.
- ``you may cast the exiled card [this turn]`` (4) — generic impulse
  recast umbrella.
- ``target player may search their library for a <type>, put it onto
  the battlefield, then shuffle`` (4) — Settle-the-Wreckage flavor
  opponent-ramp tail.

Static riders:
- ``creatures you control with counters on them have/gain <kw>`` (3).
- ``each instant and sorcery card in your graveyard/hand has <kw>`` (3)
  — Lier / Niv-Mizzet / Lorehold static.
- ``each instant and sorcery spell you cast has/costs <X>`` (3) —
  Silverquill/Djinn static.
- ``enchanted creature gets +2/+2 and attacks each combat if able`` (3).
- ``enchanted creature gets -X/-0, where X is ...`` (3).
- ``until end of turn, that creature has base power and toughness N/N
  and gains <kws>`` (3).
- ``that creature is a black zombie in addition to its other colors
  and types`` (3) — Dread Slaver / Grave Betrayal.
- ``its power is equal to that creature's power and its toughness is
  equal to ...`` (4).
- ``its activated abilities can't be activated [this turn|for as long
  as it's tapped|unless they're mana abilities]`` (4) — scrubber #5
  had the bare form; this broadens the tail.

Trigger shapes:
- ``whenever one or more tokens you control enter`` (4) —
  Kambal/Spiritcall token-enter trigger.
- ``whenever one or more pirates you control deal damage`` (3) —
  Breeches-class subtype-combat trigger.
- ``whenever two or more creatures you control attack`` (3) —
  Chatzuk-style crowd-attack ally variant.
- ``whenever the final chapter ability of a saga you control
  resolves/triggers`` (3) — Tom Bombadil / Narci.
- ``when there are no creatures on the battlefield, sacrifice ~`` (3)
  — Drop of Honey / Porphyry Nodes state-trigger.
- ``when enchanted artifact is put into a graveyard`` (3) — Viridian
  Harvest-style aura-on-artifact trigger.
- ``when this card is put into your hand/graveyard from your ...`` (3)
  — Golgari Brownscale / Gaea's Blessing zone-to-zone trigger on
  self.

Ordering: specific-first within each table; lists are spliced into the
base parser's pattern lists in ``parser.load_extensions``.
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
    Buff, Draw, Filter, GrantAbility, Keyword, Modification, Sacrifice,
    Static, UnknownEffect,
    TARGET_CREATURE, TARGET_PLAYER, TARGET_OPPONENT,
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


# --- Suspend N-{cost} ------------------------------------------------------
# Cluster (64). The biggest cluster left. Normalize collapses em-dash so
# "Suspend 3-{1}{R}" arrives as "suspend 3-{1}{r}". Base KEYWORD_RE doesn't
# handle the N-{cost} shape, and scrubber #1's bare "suspend" only caught
# the zero-arg form.
@_sp(r"^suspend (\d+|x)[-–—](\{[^}]+\}(?:\{[^}]+\})*|0)\s*$")
def _suspend_cost(m, raw):
    return Keyword(name="suspend", args=(m.group(1), m.group(2)), raw=raw)


# --- Multikicker {cost} ----------------------------------------------------
# Cluster (18). Base KEYWORD_RE never listed multikicker. Any cost shape.
@_sp(r"^multikicker (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _multikicker(m, raw):
    return Keyword(name="multikicker", args=(m.group(1),), raw=raw)


# --- Megamorph {single-pip} ------------------------------------------------
# Cluster (3). Scrubber #2 listed megamorph but its regex requires 2+ pip
# groups; these are single-pip.
@_sp(r"^megamorph (\{[^}]+\})\s*$")
def _megamorph_single(m, raw):
    return Keyword(name="megamorph", args=(m.group(1),), raw=raw)


# --- Bloodthirst X ---------------------------------------------------------
# Cluster (2). KEYWORD_RE has `bloodthirst \d+` but not the X variant.
@_sp(r"^bloodthirst x\s*$")
def _bloodthirst_x(m, raw):
    return Keyword(name="bloodthirst", args=("x",), raw=raw)


# --- Bare populate / bare "you become the monarch" / bare "discard that
# card" / bare "sacrifice a creature|land" --------------------------------
# Small clusters (2-7 each). Effect-level but reach static path on some
# cards; pattern_5 doesn't have them.
@_sp(r"^populate\s*$")
def _populate(m, raw):
    return Keyword(name="populate", raw=raw)


@_sp(r"^you become the monarch\s*$")
def _become_monarch(m, raw):
    return Static(modification=Modification(kind="become_monarch"), raw=raw)


# --- "activate no more than (once|twice|\w+ times) each turn" -------------
# Cluster (8). Restriction-on-activated-abilities rule.
@_sp(r"^activate (?:this ability )?no more than "
     r"(once|twice|three times|four times|\w+ times) each turn\s*$")
def _activate_no_more(m, raw):
    return Static(modification=Modification(
        kind="activate_no_more_than",
        args=(m.group(1),)), raw=raw)


# --- "you don't lose unspent <color> mana as steps and phases end" --------
# Cluster (5). Omnath / Ashling / Leyline Tyrant mana-retention rule.
@_sp(r"^you don'?t lose unspent ([a-z]+) mana as steps and phases end\s*$")
def _no_lose_unspent(m, raw):
    return Static(modification=Modification(
        kind="no_lose_unspent_mana",
        args=(m.group(1),)), raw=raw)


# --- "its activated abilities can't be activated [rider]" ------------------
# Cluster (4 tails). Scrubber #5 had the bare form; this catches the
# ``this turn | for as long as it remains tapped | unless they're mana
# abilities`` continuations.
@_sp(r"^its activated abilities can'?t be activated "
     r"(this turn|for as long as [^.]+|unless [^.]+)\s*$")
def _its_activated_cant_tail(m, raw):
    return Static(modification=Modification(
        kind="its_activated_cant_tail",
        args=(m.group(1).strip(),)), raw=raw)


# --- "creatures you control with counters on them have|gain <kw>" ---------
# Cluster (3). Tesak / Nev static / Synchronized Charge.
@_sp(r"^creatures you control with counters on them "
     r"(?:have|gain) ([a-z, ]+?)(?: until end of turn)?\s*$")
def _ally_with_counters_have(m, raw):
    return Static(modification=Modification(
        kind="ally_with_counters_have_kw",
        args=(m.group(1).strip(),)), raw=raw)


# --- "each instant and sorcery card in your <zone> has <kw>" --------------
# Cluster (3). Lier / Niv-Mizzet / Lorehold zone-card grant.
@_sp(r"^each instant and sorcery cards? in your "
     r"(graveyard|hand|library)(?: [^,.]+)? has ([^.]+?)\s*$")
def _is_cards_in_zone_have(m, raw):
    return Static(modification=Modification(
        kind="is_cards_in_zone_has_kw",
        args=(m.group(1), m.group(2).strip())), raw=raw)


# --- "each instant and sorcery spell you cast has <X> / costs {N} less" ---
# Cluster (3). Silverquill / Djinn Illuminatus / Battlefield Thaumaturge.
@_sp(r"^each instant and sorcery spells? you cast "
     r"(has [^.]+|costs? \{?\w+\}? (?:less|more)(?: to cast)?[^.]*)\s*$")
def _is_spells_you_cast(m, raw):
    return Static(modification=Modification(
        kind="is_spells_you_cast_rider",
        args=(m.group(1).strip(),)), raw=raw)


# --- "enchanted creature gets +N/+N and attacks each combat if able" ------
# Cluster (3). Furor-family compulsory-attack-plus-buff aura.
@_sp(r"^enchanted creature gets ([+-]\d+)/([+-]\d+) "
     r"and (attacks each combat if able|must attack[^,.]*)\s*$")
def _enchanted_buff_must_attack(m, raw):
    return Static(modification=Modification(
        kind="enchanted_buff_must_attack",
        args=(int(m.group(1)), int(m.group(2)), m.group(3))), raw=raw)


# --- "enchanted creature gets -X/-0, where X is ..." ----------------------
# Cluster (3). Fear-of-Death-family variable-debuff aura.
@_sp(r"^enchanted creature gets -x/-0,? where x is [^.]+?\s*$")
def _enchanted_neg_x_var(m, raw):
    return Static(modification=Modification(
        kind="enchanted_neg_x_var"), raw=raw)


# --- "until end of turn, that creature has base power and toughness N/N
# and gains <kws>" ---------------------------------------------------------
# Cluster (3). Mirror of scrubber #5's effect-path but arriving as static.
@_sp(r"^until end of turn, that creature has base power and toughness "
     r"(\d+|x)/(\d+|x)(?:\s+and (?:gains?|has) ([a-z, ]+))?\s*$")
def _eot_that_base_pt(m, raw):
    return Static(modification=Modification(
        kind="eot_that_base_pt",
        args=(m.group(1), m.group(2), (m.group(3) or "").strip() or None)),
        raw=raw)


# --- "that creature is a black zombie in addition to its other colors
# and types" ---------------------------------------------------------------
# Cluster (3). Dread Slaver / Grave Betrayal reanimation-rider static.
@_sp(r"^that creature is (?:a|an) ([a-z ]+?) in addition to "
     r"its other colors and types\s*$")
def _that_creature_added_types(m, raw):
    return Static(modification=Modification(
        kind="that_creature_added_types",
        args=(m.group(1).strip(),)), raw=raw)


# --- "its power is equal to that <X>'s power and its toughness is equal
# to that <X>'s toughness" -------------------------------------------------
# Cluster (4). Kalitas / Gemini Engine token-copy stat-def.
@_sp(r"^its power is equal to (?:that|this) ([a-z']+)'?s? power "
     r"and its toughness is equal to (?:that|this) [a-z']+?'?s? toughness"
     r"\s*$")
def _its_pt_mirrors(m, raw):
    return Static(modification=Modification(
        kind="its_pt_mirrors",
        args=(m.group(1),)), raw=raw)


# --- "creatures your opponents control can't block [this turn]" -----------
# Cluster (2). Cosmotronic Wave / Hazardous Blast opp-side evasion.
@_sp(r"^creatures your opponents control can'?t block"
     r"(?: this turn)?\s*$")
def _opp_creatures_cant_block(m, raw):
    return Static(modification=Modification(
        kind="opp_creatures_cant_block"), raw=raw)


# --- "you may look at that card for as long as it remains exiled" ---------
# Cluster (3). Hauken-tail / Jace impulse-peek static.
@_sp(r"^you may look at that card for as long as it remains exiled\s*$")
def _may_look_at_while_exiled(m, raw):
    return Static(modification=Modification(
        kind="may_look_at_while_exiled"), raw=raw)


# --- "you may look at cards exiled with this creature [tail]" -------------
# Cluster (3). Bane Alley Broker / Kheru Mind-Eater exile-peek.
@_sp(r"^you may look at cards exiled with "
     r"(?:this (?:creature|permanent|artifact|enchantment)|~)"
     r"(?:[,\s][^.]+)?\s*$")
def _may_look_exiled_with(m, raw):
    return Static(modification=Modification(
        kind="may_look_at_cards_exiled_with"), raw=raw)


# --- "the replicate cost is equal to its mana cost" -----------------------
# Cluster (3). Djinn Illuminatus / Ian Chesterton - keyword cost def.
@_sp(r"^the (replicate|kicker|buyback|flashback) cost is equal to "
     r"(?:its|that card'?s|the card'?s) mana cost\s*$")
def _keyword_cost_equals_mc(m, raw):
    return Static(modification=Modification(
        kind="keyword_cost_equals_mc",
        args=(m.group(1),)), raw=raw)


# --- "choose up to five {p} worth of modes" -------------------------------
# Cluster (5). Season-of-* spree header (paid pieces). Scrubber #5 had the
# Spree "+ {cost} - <effect>" mode lines but not this chooser.
@_sp(r"^choose up to (\w+) \{p\} worth of modes\s*$")
def _choose_p_worth(m, raw):
    return Static(modification=Modification(
        kind="choose_p_worth_of_modes",
        args=(m.group(1),)), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "you lose half your life, rounded up" --------------------------------
# Cluster (4). Infernal Contract / Doomsday / Cruel Bargain / Death Wish
# self-pay-life cost/effect.
@_er(r"^you lose half your life,? rounded (?:up|down)(?:\.|$)")
def _lose_half_life(m):
    return UnknownEffect(raw_text="you lose half your life, rounded up")


# --- "target player takes an extra turn after this one" ------------------
# Cluster (3). Walk the Aeons / Beacon of Tomorrows extra-turn grant.
@_er(r"^target player takes an extra turn after this one(?:\.|$)")
def _target_extra_turn(m):
    return UnknownEffect(raw_text="target player takes an extra turn")


# --- "switch target creature's power and toughness until end of turn" ----
# Cluster (3). Twisted Image / Strange Inversion.
@_er(r"^switch target creature'?s? power and toughness"
     r"(?: until end of turn)?(?:\.|$)")
def _switch_pt(m):
    return UnknownEffect(raw_text="switch target creature's P/T")


# --- "at this turn's next end of combat, <effect>" -----------------------
# Cluster (4). Triton Tactics / Glyph of Doom delayed-eoc trigger line.
@_er(r"^at this turn'?s? next end of combat,? [^.]+(?:\.|$)")
def _at_next_eoc(m):
    return UnknownEffect(raw_text="at this turn's next end of combat, <effect>")


# --- "target creature gains trample and gets +X/+0 until end of turn,
# where X is ..." ---------------------------------------------------------
# Cluster (3). Berserk / Surge of Strength / Ancestral Anger.
@_er(r"^target creature gains (trample|flying|haste|menace|[a-z, ]+?) "
     r"and gets \+(?:x|\d+)/\+(?:x|\d+)(?: until end of turn)?"
     r"(?:,\s+where x is [^.]+)?(?:\.|$)")
def _target_gains_buff_var(m):
    return GrantAbility(ability_name=m.group(1).strip(), target=TARGET_CREATURE)


# --- "target opponent may draw a card" -----------------------------------
# Cluster (3). Phelddagrif-tail.
@_er(r"^target opponent may draw (a|one|two|three|\d+) cards?(?:\.|$)")
def _target_opp_may_draw(m):
    return Draw(count=m.group(1), target=TARGET_OPPONENT)


# --- "target player reveals three cards from their hand [and you choose
# one of them]" -----------------------------------------------------------
# Cluster (3). Blackmail-class.
@_er(r"^target player reveals (two|three|four|five|\w+|\d+) cards? from their hand"
     r"(?:\s+and you choose one of them)?(?:\.|$)")
def _target_reveals_n_hand(m):
    return UnknownEffect(raw_text=f"target player reveals {m.group(1)} cards from hand")


# --- "each player sacrifices a creature of their choice" -----------------
# Cluster (3). Reign of the Pit / Rise of the Witch-king.
@_er(r"^each player sacrifices (?:a|an|one|two|\d+) "
     r"(creature|artifact|enchantment|land|permanent)s?"
     r"(?: of their choice)?(?:\.|$)")
def _each_player_sac(m):
    return UnknownEffect(raw_text=f"each player sacrifices a {m.group(1)}")


# --- "you and that player each <effect>" ---------------------------------
# Cluster (4). Benevolent/Sylvan/Intellectual Offering.
@_er(r"^you and that player each (create|draw|gain|lose|sacrifice|"
     r"exile|discard) [^.]+(?:\.|$)")
def _you_and_that_player_each(m):
    return UnknownEffect(raw_text=f"you and that player each {m.group(1)} ...")


# --- "those creatures can't block [this turn]" ---------------------------
# Cluster (3). Wrap in Flames-tail.
@_er(r"^those creatures can'?t block(?: this turn)?(?:\.|$)")
def _those_creatures_cant_block(m):
    return UnknownEffect(raw_text="those creatures can't block")


# --- "each opponent exiles the top card of their library [tail]" ---------
# Cluster (6). Fathom Feeder / Xander's Pact / Fevered Suspicion
# /Dream Harvest / Wand of Wonder. One rule for both the single-card and
# the "until" variants.
@_er(r"^each opponent exiles "
     r"(?:the top card of their library|cards from the top of their library"
     r"(?: until [^,.]+)?)"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _each_opp_exiles_top(m):
    return UnknownEffect(raw_text="each opponent exiles top of library")


# --- "put those land cards onto the battlefield tapped and the rest on
# the bottom of your library in a random order" --------------------------
# Cluster (3). Ring Goes South / Open the Way tutor tail.
@_er(r"^put those (?:land )?cards? onto the battlefield"
     r"(?: tapped)?(?: and the rest (?:on the bottom|into) [^.]+)?"
     r"(?:\.|$)")
def _put_those_cards_bf(m):
    return UnknownEffect(raw_text="put those cards onto BF (tapped) + rest")


# --- "put up to two of them into your hand and the rest into|on ..." -----
# Cluster (3). Shadow Prophecy / Mana Rig / Thassa's Intervention.
@_er(r"^put up to (two|three|\w+|\d+) of them into "
     r"(?:your|their) (hand|graveyard|library)"
     r"(?: and the rest (?:on the bottom|into|onto) [^.]+)?(?:\.|$)")
def _put_up_to_n_of_them(m):
    return UnknownEffect(
        raw_text=f"put up to {m.group(1)} of them -> {m.group(2)}, rest -> ...")


# --- "put the rest on the bottom in a random order" ----------------------
# Cluster (3). Naked tail missing the "of your library" scrubber #5
# required.
@_er(r"^put the rest on the bottom(?: in a random order)?(?:\.|$)")
def _put_rest_bottom_naked(m):
    return UnknownEffect(raw_text="put the rest on the bottom (random)")


# --- "return the exiled cards to the battlefield [tapped] under their
# owners' control" --------------------------------------------------------
# Cluster (3). Hide on the Ceiling / Sudden Disappearance blink-tail.
@_er(r"^return the exiled cards to the battlefield"
     r"(?: tapped)?"
     r"(?:\s+under (?:their|its) owners?'? control)?(?:\.|$)")
def _return_exiled_to_bf(m):
    return UnknownEffect(raw_text="return exiled cards to BF")


# --- "counter that spell unless its controller pays {N} [tail]" ----------
# Cluster (3). Forbid / Mana Leak class.
@_er(r"^counter that spell unless its controller pays "
     r"(?:\{[^}]+\}|twice \{[^}]+\}|\w+ \{[^}]+\})"
     r"(?:[,\s][^.]+|\s+for each [^,.]+)?(?:\.|$)")
def _counter_unless_pays(m):
    return UnknownEffect(raw_text="counter that spell unless pays {N}")


# --- "discard that card" bare --------------------------------------------
# Cluster (3). Mothlight Processionist-tail.
@_er(r"^discard that card(?:\.|$)")
def _discard_that_card(m):
    return UnknownEffect(raw_text="discard that card")


# --- "sacrifice a creature|land|artifact|enchantment|permanent [rider]" --
# Cluster (7 = 4 creature + 3 land). Bare sacrifice, surfaced as an
# effect on multi-line cards (Rupture / Roiling Regrowth).
@_er(r"^sacrifice (?:a|an|one|two|\w+) "
     r"(creature|artifact|enchantment|land|permanent|token)"
     r"(?: or (?:creature|artifact|enchantment|land|permanent))?(?:\.|$)")
def _sac_bare(m):
    return Sacrifice(
        filter=Filter(base=m.group(1), targeted=False),
        count=1)


# --- "before you shuffle your deck to start the game, ..." ---------------
# Cluster (3). Conspiracy draft-sideboard rider (reveal-from-sideboard).
@_er(r"^before you shuffle your deck to start the game,? [^.]+(?:\.|$)")
def _before_shuffle_draft(m):
    return UnknownEffect(raw_text="before shuffle to start game, ...")


# --- "you may cast the exiled card [this turn]" --------------------------
# Cluster (4). Generic impulse-draw recast umbrella.
@_er(r"^you may cast the exiled cards?(?: this turn| without paying [^.]+"
     r"| for as long as [^.]+)?(?:[,\s][^.]+)?(?:\.|$)")
def _may_cast_the_exiled(m):
    return UnknownEffect(raw_text="may cast the exiled card")


# --- "that player may search their library for <X>, put it onto the
# battlefield, then shuffle" ---------------------------------------------
# Cluster (4). Settle-the-Wreckage / Boseiju opp-ramp.
@_er(r"^that player may search their library for "
     r"(?:a|an|that many|one|two|\w+|\d+) ([^,.]+?)"
     r"(?:\s+cards?)?,?\s+"
     r"put (?:it|them|those cards|that card) onto the battlefield"
     r"(?:\s+tapped)?,?\s*"
     r"then shuffle(?:\.|$)")
def _that_player_tutor(m):
    return UnknownEffect(
        raw_text=f"that player may search lib for {m.group(1).strip()}")


# --- "put a counter of that kind on <target>" ----------------------------
# Cluster (3). Crystalline Giant / Contractual Safeguard counter-copy.
@_er(r"^put a counter of that kind on "
     r"(this (?:creature|permanent|artifact)|each other creature you control|"
     r"target [^.]+?|~|another target [^.]+?)(?: [^.]+)?(?:\.|$)")
def _counter_of_that_kind(m):
    return UnknownEffect(
        raw_text=f"put a counter of that kind on {m.group(1).strip()}")


# --- "reveal cards from the top of your library until you reveal <X>" ----
# Cluster (3). Abundant Harvest / Kindred Summons / Open the Way top-level.
@_er(r"^reveal cards from the top of your library until you reveal "
     r"[^.]+(?:\.|$)")
def _reveal_until_you_reveal(m):
    return UnknownEffect(raw_text="reveal from top until you reveal X")


# --- "you may cast an instant or sorcery spell [with <mv>] from <zone>
# [without paying ...]. then <tail>" --------------------------------------
# Cluster (3). Muse Vortex / Talent of the Telepath / Your Wish.
@_er(r"^you may cast an instant or sorcery spell"
     r"(?: with mana value [^,.]+)?"
     r"(?: from (?:among them|your graveyard|~ sideboard|the top [^.]+))?"
     r"(?:[,\s][^.]+|\s+without paying [^.]+)?(?:\.|$)")
def _may_cast_is_spell(m):
    return UnknownEffect(raw_text="may cast an instant or sorcery spell")


# --- "you may cast instant and sorcery spells [with <mv>] from <zone>
# [without paying their mana costs]. then <tail>" ------------------------
# Cluster (3). Epic Experiment / Melek / Precognition Field.
@_er(r"^you may cast instant and sorcery spells"
     r"(?: with mana value [^,.]+)?"
     r"(?: from (?:among them|your graveyard|the top [^.]+))?"
     r"(?:[,\s][^.]+|\s+without paying [^.]+)?(?:\.|$)")
def _may_cast_is_spells(m):
    return UnknownEffect(raw_text="may cast instant and sorcery spells")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # --- "whenever one or more tokens you control enter" ----------------
    # Cluster (4). Kambal / Spiritcall / Caretaker's Talent.
    (re.compile(r"^whenever one or more (?:other )?tokens? you control "
                r"(?:enter|leave the battlefield|die)", re.I),
     "one_or_more_tokens_ally_event", "self"),

    # --- "whenever one or more <subtype>s you control deal damage" ------
    # Cluster (3). Breeches / Malcolm / Francisco pirate-damage trigger.
    (re.compile(r"^whenever one or more (?:pirates|knights|soldiers|"
                r"humans|elves|zombies|vampires|goblins|merfolk|dragons|"
                r"angels|demons|warriors|wizards|rogues|clerics|druids|"
                r"samurai|ninjas|spirits|sphinxes|beasts) you control "
                r"deal damage", re.I),
     "ally_subtype_deal_damage", "self"),

    # --- "whenever two or more creatures you control attack [a player|
    # in a band]" --------------------------------------------------------
    # Cluster (3). Chatzuk / Landroval / Horn of the Mark.
    (re.compile(r"^whenever (?:two or more|three or more|\w+ or more) "
                r"creatures you control attack"
                r"(?: a player| a player or planeswalker| in a band)?",
                re.I),
     "n_or_more_ally_attack", "self"),

    # --- "whenever the final chapter ability of a saga you control
    # resolves|triggers" --------------------------------------------------
    # Cluster (3). Tom Bombadil / Narci / Historian's Boon saga-cap
    # trigger.
    (re.compile(r"^whenever the final chapter ability of an? "
                r"(?:saga|enchantment) you control "
                r"(?:resolves|triggers)", re.I),
     "saga_final_chapter", "self"),

    # --- "when there are no creatures on the battlefield, sacrifice ~" --
    # Cluster (3). Drop of Honey / Porphyry Nodes state-trigger.
    (re.compile(r"^when there are no creatures on the battlefield", re.I),
     "no_creatures_state", "self"),

    # --- "when enchanted artifact|creature|... is put into a graveyard" -
    # Cluster (3). Viridian Harvest / Gremlin Infestation / Tezzeret's
    # Touch aura-die trigger.
    (re.compile(r"^when enchanted (?:artifact|creature|enchantment|"
                r"permanent|land) is put into (?:a|an|its owner'?s?) "
                r"graveyard", re.I),
     "enchanted_perm_to_gy", "self"),

    # --- "when this card is put into your hand|graveyard|library from
    # <zone>" ------------------------------------------------------------
    # Cluster (3). Golgari Brownscale / Gaea's Blessing / Narcomoeba.
    (re.compile(r"^when this card is put into your "
                r"(?:hand|graveyard|library|exile) from your "
                r"(?:graveyard|library|hand|battlefield|exile)", re.I),
     "self_card_zone_to_zone", "self"),
]
