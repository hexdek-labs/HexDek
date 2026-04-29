#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (eleventh pass).

Family: PARTIAL -> GREEN promotions. Companion to ``partial_scrubber.py``
through ``partial_scrubber_10.py``. Targets remaining single-ability
clusters after ten prior passes (1,708 PARTIAL cards at 5.40% on a
31,639-card pool).

Re-bucketing the remaining parse_errors after scrubber #10 reveals the
distribution has flattened: most 6-word prefixes hit only 1-2 cards now.
But broader 3-4 word "shape" prefixes still cluster richly — many
remaining PARTIAL cards fail on similar verb-led shapes that prior
scrubbers didn't quite cover (different filter, different rider,
different counter type).

Cluster set targeted in this eleventh pass — selected because each one
has 3+ examples AND the prior scrubbers don't already match the shape:

EFFECT_RULES clusters:
- ``enchanted creature gets <±P/±T> and <rider>`` (10) — generic two-
  clause aura buff body. Prior scrubbers caught specific riders
  (``and attacks each combat``, ``as long as it's <X>``, ``and assigns
  combat damage equal to its toughness``). Open: ``and is <color>``,
  ``and can't block``, ``and loses <kw>``, ``and its activated abilities
  can't be activated``, ``and gains <kws>``. Cluster covers
  Viper's Kiss / Cast into Darkness / Street Savvy / Craving of Yeenoghu
  / Ghoulflesh / Grave Servitude / Tightening Coils / Aspect of Wolf /
  Fresh Start / Sicken-style.
- ``enchanted creature is a <type> [with base PT P/T] [and ...]`` (5) —
  Lignify-family lobotomy. Spider-Man No More / Lignify / Fog on the
  Barrow-Downs / Eye of Nidhogg / Retro-Mutation.
- ``enchanted creature can't <verb> <tail>`` (4) — Spectral Grasp /
  Brainwash / Hobble / Ironclaw Curse aura debuff.
- ``creatures you control with <quality> <verb> <tail>`` (9) — anthem
  filtered by sub-population. Prior scrubbers cover specific shapes
  (with kw + get +N/+N, with counters, with mana value <N>). Open:
  ``with <X> have/can't be/are`` for arbitrary qualities. Jasmine
  Boreal / Delney / Kwende / Runadi / Rally of Wings / Jubilant
  Skybonder / The Flesh Is Weak / Proud Wildbonder / Hero of the Dunes.
- ``each creature you control <verb> <tail>`` (10) — ``has ward``,
  ``deals 1 damage``, ``becomes that type``, ``crews vehicles``,
  ``has vigilance if it's white``. Distinct from
  ``enters with additional counter`` / ``can't be blocked`` /
  ``assigns combat damage`` already in scrubbers 7-10.
- ``other creatures you control with <X> have <kw>`` (3+) — Cavalry
  Master / Sephara / Roshan-style sub-population grant. Distinct from
  scrubber #2 which catches ``other creatures you control with <X>
  get +N/+N``.
- ``creatures your opponents control [<filter>] <verb> <tail>`` (4) —
  Glaring Spotlight / Sidar Kondo / Baeloth Barrityl / Blightbeetle /
  Karlov Watchdog inverse anthem.
- ``creatures target player controls <verb> <tail>`` (3) — Shields of
  Velis Vel / Ego Erasure / Ember Gale targeted-mass-buff/debuff.
- ``nontoken creatures you control are/enter <tail>`` (3) — Ashaya /
  Infinite Reflection / Gorma the Gullet.
- ``[type] you control get/have/are/deal/enter <tail>`` (22) — generic
  tribal anthem with a non-creature noun. Distinct from existing
  ``creatures you control get +N/+N`` rules. Wurms / wizards /
  spiders / squirrels / spirits / vehicles / treasures / vampires /
  zombies / etc — open-ended type word + verb.
- ``spells your opponents cast [<filter>] cost <{N}|N life> <more|less>
  to cast`` (4) — Paladin Class / Hinata / Aven Interrupter / Terror
  of the Peaks variants.
- ``[type] spells you cast cost <{X}> <less|more> to cast`` (4) —
  Ezzaroot / Herald's Horn / Zinnia / Samut. Prior scrubbers cover
  ``each spell you cast costs {N} less`` and ``creature spells you
  cast cost {N} less if mutate``, but not the open-ended typed cost
  with arbitrary trailing where-clause.
- ``each spell you cast that's <pred>`` (3) — Seal of the Guildpact /
  Ancient Cellarspawn / Threefold Signal — predicate-qualified self-
  spell rider.
- ``the first player may <verb> <tail>`` (5) — Oath cycle (Oath of
  Mages / Oath of Lieges / Oath of Ghouls / Oath of Druids / Oath of
  Scholars). Each fails because the first-player conditional opens an
  arbitrary inner effect.
- ``each player who <pred> <verb> <tail>`` (7) — Plaguecrafter family.
  Cirdan / Momentum Breaker / Plaguecrafter / Infernal Offering /
  Fandaniel / Step Between Worlds / Scythe Specter / The Second Doctor.
- ``the owner of target <type> shuffles|puts|exiles <tail>`` (3) —
  Chaos Warp / Lost Days / Audacious Swap.
- ``shuffle your library, then <verb> [<tail>]`` (3) — Unexpected
  Results / Hazoret's Undying Fury / Creative Technique. Beyond the
  scrubber #10 ``then reveal the top card`` variant.
- ``this <aura|enchantment|sorcery|artifact> deals <amt> damage to
  <target>`` (8) — Errant Minion / Power Leak / Planeswalker's Fury /
  Heretic's Punishment / Everything Pizza / Goblin Charbelcher /
  Angel's Trumpet / Mephit's / Molten Impact source-as-card variant
  (the scrubber #10 catches the un-qualified ``this aura deals N
  damage`` and ``this sorcery deals N damage``; this one catches the
  ``deals damage equal to`` and ``deals N damage to <complex target>``
  shapes).
- ``you may put a <type> card from <X> [into|onto] <Y>`` (4+3=7) —
  Rosheen / Kinnan / Death Wish / Leyline Dowser / Szarekh / Grasping
  Tentacles. Distinct from scrubber #10's ``put a card that <pred>
  from X to Y``.
- ``you may reveal an? <type> card <tail>`` (13) — Wish cycle (Burning
  / Glittering / Cunning / Living / Fae / Golden / Coax) plus
  multi-type wishes (Kaalia / Harper Recruiter / Invasion of Ravnica)
  variant of ``you may reveal a card from outside the game``.
- ``put any number of <pred> cards <from among> <X> onto/into <Y>``
  (6) — Genesis Ultimatum / Ao / Gishath / Aurora Awakener / Valakut
  Awakening — the multi-piece distributive token-recur shape.
- ``put up to <N> <pred> cards <tail>`` (5) — Smelting Vat / Pieces
  of the Puzzle / United Battlefront / Thassa's Intervention.
- ``put all <type> cards <pred> <onto|into> <X>`` (8) — Tazri /
  Mass Polymorph / Brass Herald — mass-pile distributor.
- ``return any number of <type> cards <pred> from your graveyard
  <tail>`` (3) — Legion's Chant / Reunion / Seasons Past.
- ``any number of target <type> [you control] <verb>`` (3) — Clever
  Concealment / Wheel and Deal / Gallifrey Falls.
- ``you may cast spells <pred>`` (5) — Errant and Giada / Meeting of
  the Five / Abandoned Sarcophagus / Draugr Necromancer / Borne Upon
  a Wind. Distinct from scrubber #10's ``you may cast spells from
  the top of your library by <alt-cost>``.
- ``you may look at <X>`` (5) — Revealing Wind / Found Footage /
  Colfenor's Plans / Cogwork Spy / Enhanced Surveillance.
- ``you may play <X>`` (8) — generic ``you may play lands/cards/the
  top card`` variants.
- ``[you] gain control of <demonstrative> <type> [<tail>]`` (9) —
  Aura Graft / Stolen Uniform / Wellspring / Crown of Empires / Fumble
  / Murderous Spoils / Brand / Broadcast Takeover / Rags-Riches.
  Distinct from scrubber #10's ``you gain control of that creature``.
- ``until end of turn, <subject> <verb> <tail>`` (16) — generic UEOT
  prefix on miscellaneous subjects: Xanathar (target player) / Hall
  of Gemstone (lands) / Flare of Fortitude (life total + permanents)
  / Temporal Aperture / Majestic Metamorphosis (target artifact) /
  Sparkshaper Visionary (they). Distinct from scrubber #2/9/10 which
  cover ``target creature``, ``that creature``, ``it``, ``that
  creature's controller``, ``up to N target creatures``, ``you may``,
  ``it has base PT``.
- ``draw a card if <pred>`` (4) — Lyzolda / Roadside Reliquary /
  Eagle of Deliverance / Oblivion's Hunger conditional draw tail.
- ``each opponent who <pred> <verb> <tail>`` (3+ already partly
  covered above) — Momentum Breaker / Skull Storm / Entropic
  Battlecruiser. The "each opponent who can't" specific shape.
- ``target creature can't be <verb> <tail>`` (3) — Gravebind / The
  Black Gate / Vines of Vastwood targeted protection rider.
- ``this creature enters with <amt> <kind> counters on it [<rider>]``
  (3) — Banquet Guests / Steel Exemplar / Hotheaded Giant variant
  beyond plain ETB-with-counter.
- ``when you control no <noun>, sacrifice <X>`` (3) — Phylactery
  Lich / Skeleton Ship / Island Fish Jasconius static/triggered tail
  (treated as effect for orphan parser).
- ``when you play a card <tail>`` (4) — Voltaic Visionary / Juju
  Bubble / Fires of Mount Doom variant.
- ``whenever one or more <type> [<filter>] enter|die|are put|...``
  (6) — Expedition Supplier / City in a Bottle / Seer of Stolen
  Sight / Zone of Flame / Elvish Warmaster / Cloudsculpt Armorer.
  Distinct from scrubber #10's ``whenever one or more other <type>``.
- ``for as long as <pred>, <verb> <tail>`` (4) — Dream-Thief's
  Bandana / Opportunistic Dragon / Divine Purge / Dimensional Breach
  exile-loop tail.
- ``gain control of all <type> [<filter>] [<rider>]`` (3) — Fumble /
  Broadcast Takeover / Brand mass take-control.
- ``return any number of <type> cards <pred> from your graveyard
  to <Y>`` (3) — already covered above; notable shape.

STATIC_PATTERNS clusters:
- ``[type] you control [<filter>] get <±P/±T> [<rider>]`` (22) — non-
  creature-noun anthem. Same body shape as creature anthem but the
  noun is wurms/spirits/wizards/spiders/squirrels/etc. Prior scrubbers
  enumerate specific tribes (in scrubber #4's typelist anthem); this
  one is a generic single-noun anthem.
- ``creatures you control with <quality> have <kw>`` (5) — Kwende /
  Jubilant Skybonder / Proud Wildbonder.
- ``each creature you control has <kw>[ <rider>]`` (3) — Chaplain of
  Alms / Cathedral Acolyte / Arcades the Strategist.
- ``creatures target player controls get <±P/±T> and <rider>`` (3) —
  Shields of Velis Vel / Ego Erasure / Ember Gale.
- ``creatures your opponents control with <quality> <verb>`` (3) —
  Glaring Spotlight / Sidar Kondo / Baeloth.
- ``other creatures you control [with <X>] have <kw>`` (3) — Cavalry
  Master / Sephara / Roshan.
- ``[type] spells you cast cost {N|X} less to cast [where|of|that]``
  (4) — typed-cost-reduction with arbitrary tail.
- ``spells your opponents cast [<filter>] cost <X> more to cast``
  (4) — inverse cost rider.

TRIGGER_PATTERNS clusters:
- ``at the beginning of <ordinal> <step>`` (4) — ``of the next upkeep``
  / ``of your first upkeep``. Distinct from base ``at the beginning
  of your upkeep``.
- ``when you play a <type> <tail>`` (4) — already enumerated.
- ``when you control no <noun>`` (3) — sacrifice trigger header.

Ordering: specific-first within each table; lists are spliced into
the base parser's pattern lists in ``parser.load_extensions``.
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
    Modification, Static, UnknownEffect,
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


# --- "<type> you control get ±P/±T [and have/gain <kw>] [for each X]" -----
# Cluster (22). Generic single-tribe anthem. Examples: wurms / wizards
# / spiders / squirrels / spirits / vehicles / vampires / zombies /
# treasures / artifacts / clerics / soldiers / dwarves. Distinct from
# the existing ``creatures you control get`` rules and from the typelist
# anthem in scrubber #9 (which requires "[A], [B], and [C] you control").
@_sp(r"^(?:non[a-z]+ )?[a-z]+s? you control "
     r"get ([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"(?:\s+(?:and (?:have|gain|are|can'?t) [^.]+?|"
     r"for each [^.]+?|"
     r"as long as [^.]+?))?\s*$")
def _typed_anthem_get(m, raw):
    return Static(modification=Modification(
        kind="typed_anthem_get",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "<type> you control have <kw[, kw]>" --------------------------------
# Sister rule. Anthem-by-grant rather than +N/+N.
@_sp(r"^(?:non[a-z]+ )?[a-z]+s? you control "
     r"(?:have|gain) "
     r"(flying|deathtouch|trample|haste|first strike|double strike|"
     r"vigilance|reach|menace|defender|lifelink|hexproof|ward|indestructible|"
     r"protection from [a-z]+|shroud|flash)"
     r"(?:,\s+(?:flying|deathtouch|trample|haste|first strike|double strike|"
     r"vigilance|reach|menace|defender|lifelink|hexproof|ward|indestructible|"
     r"shroud|flash))*"
     r"(?:,?\s+and (?:flying|deathtouch|trample|haste|first strike|"
     r"double strike|vigilance|reach|menace|defender|lifelink|hexproof|"
     r"ward|indestructible|shroud|flash))?\s*$")
def _typed_anthem_have_kw(m, raw):
    return Static(modification=Modification(
        kind="typed_anthem_have_kw",
        args=(m.group(1),)), raw=raw)


# --- "<type> you control are <type-list> in addition to ..." -------------
# Roshan, Hidden Magister / Conspiracy / Ashaya / etc.
@_sp(r"^(?:non[a-z]+ )?[a-z]+s? you control "
     r"are [a-z]+(?:\s+(?:and|or|/)\s+[a-z]+)?(?:\s+lands?)? "
     r"in addition to (?:their|its) other (?:types?|colors? and types?)"
     r"\s*$")
def _typed_are_added_types(m, raw):
    return Static(modification=Modification(
        kind="typed_are_added_types"), raw=raw)


# --- "creatures you control with <quality> have <kw[, kw, ...]>" ---------
# Kwende / Jubilant Skybonder / Proud Wildbonder / Cavalry Master.
# Distinct from scrubber #2's ``other creatures you control with <X>
# get +N/+N`` (this is have-keyword, not get-±P/±T).
@_sp(r"^(?:other )?creatures you control with [^,]{2,80}? "
     r"(?:have|gain) "
     r"(flying|deathtouch|trample|haste|first strike|double strike|"
     r"vigilance|reach|menace|defender|lifelink|hexproof|ward|indestructible|"
     r"flanking|shroud|flash|protection from [a-z]+)"
     r"(?:[,\s]+(?:and )?[a-z ]+?)?\s*$")
def _cu_with_have_kw(m, raw):
    return Static(modification=Modification(
        kind="cu_with_quality_have_kw",
        args=(m.group(1),)), raw=raw)


# --- "creatures you control with <quality> can't <verb> <tail>" ----------
# Jasmine Boreal / Delney / Hero of the Dunes mid-anthem rider.
@_sp(r"^(?:other )?creatures you control with [^,]{2,80}? "
     r"can'?t (?:be (?:blocked|targeted|countered)|attack|block|be the target) "
     r"[^.]+?\s*$")
def _cu_with_quality_cant(m, raw):
    return Static(modification=Modification(
        kind="cu_with_quality_cant"), raw=raw)


# --- "creatures you control with <quality> are <type> in addition ..." ---
# The Flesh Is Weak. Sub-population type-add.
@_sp(r"^creatures you control with [^,]{2,80}? "
     r"are [a-z]+(?:\s+(?:and|or)\s+[a-z]+)? "
     r"in addition to (?:their|its) other types?\s*$")
def _cu_with_quality_added_types(m, raw):
    return Static(modification=Modification(
        kind="cu_with_quality_added_types"), raw=raw)


# --- "creatures you control with <quality> get ±P/±T" --------------------
# Hero of the Dunes / Rally of Wings (when un-targeted) — broader filter
# than scrubber #5's `[a-z ]+?` body.
@_sp(r"^creatures you control with [^,]{2,80}? "
     r"get ([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"(?:\s+until end of turn)?\s*$")
def _cu_with_quality_get(m, raw):
    return Static(modification=Modification(
        kind="cu_with_quality_get",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "each creature you control has <kw>[ <rider>]" ----------------------
# Chaplain of Alms / Cathedral Acolyte / Arcades. Note scrubber #4's
# typed-quality variant is more specific.
@_sp(r"^each creature you control "
     r"(?:has|gains?) "
     r"(flying|deathtouch|trample|haste|first strike|double strike|"
     r"vigilance|reach|menace|defender|lifelink|hexproof|ward(?:\s+\{[^}]+\})?|"
     r"indestructible|shroud|flash|protection from [a-z]+)"
     r"(?:\s+(?:if|as long as|while|when) [^.]+?)?\s*$")
def _each_cu_has_kw(m, raw):
    return Static(modification=Modification(
        kind="each_cu_has_kw",
        args=(m.group(1),)), raw=raw)


# --- "creatures target player controls get ±P/±T [and <rider>]" ----------
# Shields of Velis Vel / Ego Erasure / Ember Gale targeted-mass-buff.
@_sp(r"^creatures target (?:player|opponent) controls "
     r"(?:get ([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"(?:\s+and (?:gain |lose |have |are |can'?t )[^.]+?)?|"
     r"can'?t (?:block|attack)[^.]*?|"
     r"gain [a-z, ]+?)"
     r"(?:\s+until end of turn)?\s*$")
def _ctp_controls_buff(m, raw):
    return Static(modification=Modification(
        kind="ctp_controls_buff"), raw=raw)


# --- "creatures your opponents control with <quality> <verb> <tail>" -----
# Glaring Spotlight / Sidar Kondo / Baeloth Barrityl / Blightbeetle.
@_sp(r"^creatures your opponents control "
     r"(?:with [^,]{2,80}? )?"
     r"(?:can'?t (?:block|attack|be (?:turned face up|regenerated)|"
     r"have [^.]+?|gain [^.]+?)|"
     r"are goaded|"
     r"get ([+-]\d+|[+-]x)/([+-]\d+|[+-]x))"
     r"(?:[^.]+?)?\s*$")
def _opp_cu_with_quality(m, raw):
    return Static(modification=Modification(
        kind="opp_cu_with_quality"), raw=raw)


# --- "permanents your opponents control can't <verb> [<rider>]" ---------
# Karlov Watchdog / similar broad inverse anthem.
@_sp(r"^(?:non[a-z]+ )?(?:permanents?|creatures?|artifacts?|enchantments?|"
     r"lands?|planeswalkers?) your opponents control "
     r"can'?t [a-z ]+?(?:\s+during your turn|\s+this turn|\s+each turn)?\s*$")
def _opp_perm_cant(m, raw):
    return Static(modification=Modification(
        kind="opp_perm_cant"), raw=raw)


# --- "<type> spells you cast cost {N|X} <less|more> to cast [tail]" ------
# Ezzaroot / Herald's Horn / Zinnia / Samut. Distinct from existing
# ``each spell you cast costs {N} less`` and the cycling-cost rules.
@_sp(r"^(?:nontoken |non[a-z]+ )?"
     r"(?:creature|instant|sorcery|noncreature|artifact|enchantment|"
     r"planeswalker|tribal|land|permanent|legendary)"
     r"(?:\s+(?:and|or)\s+(?:creature|instant|sorcery|artifact|enchantment))?"
     r"\s+spells you cast "
     r"(?:of (?:the chosen type|each chosen type|[a-z]+ types?) )?"
     r"cost (?:\{[^}]+\}|\{x\}|x|\d+) "
     r"(?:less|more|fewer|greater) to cast"
     r"(?:[,.]\s+where x is [^.]+?)?"
     r"(?:\s+(?:if|when|as long as) [^.]+?)?\s*$")
def _typed_spells_you_cast_cost(m, raw):
    return Static(modification=Modification(
        kind="typed_spells_you_cast_cost"), raw=raw)


# --- "spells your opponents cast [<filter>] cost <X> more to cast [tail]"
# Paladin Class / Hinata / Aven Interrupter / Terror of the Peaks.
@_sp(r"^spells your opponents cast "
     r"(?:during (?:your|their) turn|"
     r"from (?:graveyards?|exile|graveyards? or from exile)|"
     r"that target [^,]+?|"
     r"with [^,]+?)?"
     r"\s*cost "
     r"(?:\{[^}]+\}|\{x\}|x|\d+|an additional \d+ life) "
     r"(?:less|more) to cast"
     r"(?:\s+for each (?:target|[a-z ]+?))?"
     r"(?:\s+if [^.]+?)?\s*$")
def _opp_spells_cost(m, raw):
    return Static(modification=Modification(
        kind="opp_spells_cost"), raw=raw)


# --- "each spell you cast that's <pred> <verb> [<tail>]" -----------------
# Seal of the Guildpact / Ancient Cellarspawn / Threefold Signal.
@_sp(r"^each spell you cast "
     r"(?:that'?s [^,]{2,80}|of (?:the chosen type|exactly [a-z]+ colors?)) "
     r"(?:has|gains?|costs?) [^.]+?\s*$")
def _each_spell_cast_pred(m, raw):
    return Static(modification=Modification(
        kind="each_spell_cast_pred"), raw=raw)


# --- "creature spells you cast gain <kw>[ as you cast them]" -------------
# Zinnia, Valley's Voice. Sister to typed-spells-cost.
@_sp(r"^(?:creature|instant|sorcery|noncreature|artifact|enchantment) "
     r"spells you cast "
     r"(?:gain|have) "
     r"(offspring|cascade|storm|conspire|kicker|flashback|buyback|"
     r"rebound|jump-start|escape|encore|disturb|specialize|"
     r"squad|prowl|miracle)"
     r"(?:\s+\{[^}]+\})?"
     r"(?:\s+as you cast them)?\s*$")
def _typed_spells_gain_kw(m, raw):
    return Static(modification=Modification(
        kind="typed_spells_gain_kw",
        args=(m.group(1),)), raw=raw)


# --- "this creature enters with <amount> <kind> counters on it [unless ..]"
# Banquet Guests / Steel Exemplar / Hotheaded Giant. Allows variable
# count and a trailing ``unless`` clause.
@_sp(r"^this creature enters with "
     r"(?:(?:twice |half )?x|one|two|three|four|five|six|seven|\d+) "
     r"(?:[+-]?\d+/[+-]?\d+|[a-z]+) counters on it"
     r"(?:\s+unless [^.]+?|\s+for each [^.]+?)?\s*$")
def _this_creature_enters_with_counters(m, raw):
    return Static(modification=Modification(
        kind="this_creature_enters_with_counters"), raw=raw)


# --- "nontoken creatures you control are/enter <tail>" -------------------
# Ashaya / Infinite Reflection / Gorma the Gullet.
@_sp(r"^nontoken creatures you control "
     r"(?:are [a-z]+ (?:lands?|in addition to [^.]+?)|"
     r"enter (?:as a copy of [^.]+?|with [^.]+?))"
     r"\s*$")
def _nontoken_cu_are_enter(m, raw):
    return Static(modification=Modification(
        kind="nontoken_cu_are_enter"), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "enchanted creature gets ±P/±T and <rider>" -------------------------
# Cluster (10). Generic two-clause aura buff. Riders observed:
# ``and is black`` / ``and can't block`` / ``and loses flying`` /
# ``and is a black zombie ...`` / ``and gains haste, attacks each combat
# if able`` / ``and loses all abilities`` / ``and its activated abilities
# can't be activated``. Distinct from the prior scrubber rules that
# target a specific named rider.
@_er(r"^enchanted creature gets ([+-]\d+|[+-]x)/([+-]\d+|[+-]y|[+-]x|[+-]\d+)"
     r"(?:,?\s+(?:and|but) [^.]+?)+(?:\.|$)")
def _enchanted_gets_and_rider(m):
    return UnknownEffect(
        raw_text=f"enchanted creature gets {m.group(1)}/{m.group(2)} and rider")


# --- "enchanted creature is a <type> [with base PT P/T] [and ...]" -------
# Cluster (5). Lignify / Spider-Man No More / Fog on the Barrow-Downs /
# Eye of Nidhogg / Retro-Mutation.
@_er(r"^enchanted creature is "
     r"(?:a|an) [a-z]+(?:\s+[a-z]+)?"
     r"(?:\s+with base power and toughness \d+/\d+)?"
     r"(?:,?\s+(?:and|has|gains?|can'?t|loses?|is) [^.]+?)*(?:\.|$)")
def _enchanted_is_a_type(m):
    return UnknownEffect(raw_text="enchanted creature is a <type> ...")


# --- "enchanted creature can't <verb> <tail>" ----------------------------
# Cluster (4). Spectral Grasp / Brainwash / Hobble / Ironclaw Curse.
@_er(r"^enchanted creature can'?t "
     r"(?:block|attack|be (?:blocked|regenerated|the target [^.]+?)|"
     r"have [^.]+?|gain [^.]+?)"
     r"(?:\s+(?:creatures? [^.]+?|unless [^.]+?|if [^.]+?|with [^.]+?))?"
     r"(?:\.|$)")
def _enchanted_cant(m):
    return UnknownEffect(raw_text="enchanted creature can't <verb>")


# --- "each creature you control <verb> <tail>" ---------------------------
# Cluster (10). ``deals 1 damage`` / ``becomes that type`` / ``crews
# vehicles`` / ``deals damage equal to`` / ``has vigilance if`` /
# ``has ward {N}``. Generic catch-all for orphan single-sentence anthem
# bodies that aren't already caught by scrubber 4/5/7/9/10's specific
# shapes.
@_er(r"^each creature you control "
     r"(?:deals?|becomes?|crews?|stations?|has|gains?|enters?|assigns?|"
     r"can'?t|gets?|attacks?|blocks?) [^.]+?(?:\.|$)")
def _each_cu_verb(m):
    return UnknownEffect(raw_text="each creature you control <verb>")


# --- "other creatures you control [with X] have <kw[, kw]>" --------------
# Cavalry Master / Sephara / Roshan. Distinct from scrubber #2's get
# +N/+N variant.
@_er(r"^other creatures you control "
     r"(?:with [^,]+? )?"
     r"(?:have|gain|are) [^.]+?(?:\.|$)")
def _other_cu_have(m):
    return UnknownEffect(raw_text="other creatures you control have")


# --- "spirits/wurms/wizards/...-typed anthem ``get|have|gain|are``" ------
# Cluster (22). Same body as the static rule above but as effect-rule
# fallback for sentences that didn't make it into the static lane.
@_er(r"^(?:non[a-z]+ )?[a-z]+s? you control "
     r"(?:get [+-]\d+/[+-]\d+(?:\s+(?:and|for each|as long as) [^.]+?)?|"
     r"(?:have|gain) (?:flying|trample|haste|vigilance|menace|deathtouch|"
     r"first strike|double strike|reach|lifelink|hexproof|ward(?:\s+\{[^}]+\})?|"
     r"indestructible|flash|defender|protection from [a-z]+)"
     r"(?:[,\s]+(?:and )?[a-z ]+?)?|"
     r"are [a-z]+(?:\s+(?:and|or)\s+[a-z]+)?\s+in addition to [^.]+?)"
     r"(?:\.|$)")
def _typed_anthem_effect(m):
    return UnknownEffect(raw_text="<type> you control anthem")


# --- "creatures target player controls <verb> <tail>" --------------------
# Shields of Velis Vel / Ego Erasure / Ember Gale.
@_er(r"^creatures target (?:player|opponent) controls "
     r"(?:get [+-]\d+/[+-]\d+(?:\s+and [^.]+?)?|"
     r"can'?t [a-z ]+?(?:\s+this turn)?|"
     r"gain [^.]+?|"
     r"lose [^.]+?)"
     r"(?:\s+until end of turn)?(?:\.|$)")
def _ctp_controls_effect(m):
    return UnknownEffect(raw_text="creatures target player controls")


# --- "this <enchantment|aura|sorcery|artifact> deals <amt> damage to <X>"
# Cluster (8). Errant Minion / Power Leak (aura source) / Planeswalker's
# Fury / Heretic's Punishment / Everything Pizza / Goblin Charbelcher /
# Angel's Trumpet. Distinct from scrubber #10 which catches only
# ``this aura/sorcery deals N damage to <single target>``.
@_er(r"^this (?:enchantment|aura|sorcery|artifact|creature) "
     r"deals (?:\d+|x|damage equal to [^.]+?) damage to "
     r"(?:any target|that (?:player|creature|permanent|opponent)|"
     r"the player|target [^.]+?|each opponent|each player|"
     r"that permanent or player|that creature or player)"
     r"(?:\s+(?:equal to [^.]+?|for each [^.]+?))?(?:\.|$)")
def _this_card_deals_damage(m):
    return UnknownEffect(raw_text="this <card> deals damage to <X>")


# --- "you may put a <type> card from <X> [into|onto] <Y>" ----------------
# Rosheen / Kinnan / Death Wish / Leyline Dowser / Szarekh / Grasping
# Tentacles. Distinct from scrubber #10 (``put a card that <pred> from
# X to Y``).
@_er(r"^you may put an? "
     r"(?:[+-]?\d+/[+-]?\d+\s+)?"
     r"(?:[a-z]+\s+){1,4}card "
     r"(?:that [^,]+? )?"
     r"(?:milled|exiled|revealed) (?:this way )?"
     r"(?:from [^.]+? )?"
     r"(?:into (?:your hand|your graveyard|the battlefield)|"
     r"onto the battlefield(?:\s+(?:tapped|under your control|"
     r"under [^.]+?))?)"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_put_typed_milled_to(m):
    return UnknownEffect(raw_text="you may put a <type> card milled/exiled to <Y>")


# --- "you may put a <type> card from among them onto/into <Y>" ----------
# Sister rule for the from-among-them shape.
@_er(r"^you may put an? "
     r"(?:non-?(?:human|creature|land|artifact))?\s*"
     r"(?:[a-z]+(?:\s+with [^,.]+?)?\s+)?card "
     r"(?:from (?:among them|that player'?s? graveyard|exile|your graveyard))"
     r"\s*(?:onto the battlefield|into your hand|into the battlefield)"
     r"(?:\s+(?:tapped|under your control|under [^.]+?))?"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_put_typed_card_from_to(m):
    return UnknownEffect(raw_text="you may put a <type> card from among them to <Y>")


# --- "you may reveal an? <pred> card you own from outside the game ..." --
# Cluster (13). Wish cycle and multi-type wishes. Catches Burning /
# Glittering / Cunning / Living / Fae / Golden / Coax / Kaalia / Harper
# Recruiter variants. Distinct from scrubber #10's plain
# ``you may reveal a <type> card from among them``.
@_er(r"^you may reveal an? "
     r"(?:[a-z]+(?:\s+(?:and|or|and/or)\s+[a-z]+)?)\s+card"
     r"(?:[,\s]+(?:an?\s+[a-z]+(?:\s+and/or\s+[a-z]+)?\s+card[,\s]*)+)?"
     r"(?:\s+(?:and/or\s+an?\s+[a-z]+\s+card))?"
     r"(?:\s+(?:that'?s [^,]+?|named [^,]+?|with [^,]+?))?"
     r"\s+(?:from (?:outside the game|among them|exile|your graveyard|"
     r"your sideboard))"
     r"(?:\s+(?:and put (?:it|them|those cards) into your hand|"
     r"or choose [^.]+?))?"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_reveal_typed_from_outside(m):
    return UnknownEffect(raw_text="you may reveal a <type> card from <zone>")


# --- "put any number of <pred> cards <from among> <X> onto/into <Y>" -----
# Cluster (6). Genesis Ultimatum / Ao the Dawn Sky / Gishath / Aurora
# Awakener / Valakut Awakening.
@_er(r"^put any number of "
     r"(?:them\s+|those\s+(?:nonland\s+)?(?:permanent\s+)?cards?|"
     r"(?:[a-z]+(?:\s+and/or\s+[a-z]+)?\s+(?:permanent\s+)?cards?)"
     r"(?:\s+with (?:total mana value|mana value|total power) [^,]+?)?)"
     r"(?:\s+from (?:among them|your hand))?"
     r"\s+(?:on(?:to)? the battlefield|into your hand|"
     r"on the bottom of [^,]+?|on top of [^,]+?)"
     r"(?:[,.]\s+(?:and )?(?:the rest|put the rest) [^.]+?)?(?:\.|$)")
def _put_any_number_distrib(m):
    return UnknownEffect(raw_text="put any number of <type> cards onto/into Y")


# --- "put up to <N> <pred> cards <tail>" ---------------------------------
# Cluster (5). Smelting Vat / Pieces of the Puzzle / United Battlefront /
# Thassa's Intervention.
@_er(r"^put up to (?:one|two|three|four|five|six|seven|\d+) "
     r"(?:of them|"
     r"(?:[a-z]+(?:\s+and/or\s+[a-z]+)?\s+)*"
     r"(?:noncreature(?:,?\s+nonland)?\s+)?(?:permanent\s+)?cards?"
     r"(?:\s+with (?:total mana value|mana value)[^,.]+?)?)"
     r"(?:\s+from among them)?"
     r"\s+(?:on(?:to)? the battlefield|into your hand|"
     r"on the bottom of [^,]+?|on top of [^,]+?)"
     r"(?:\s+(?:tapped|and the rest [^.]+?))?"
     r"(?:[,.]\s+[^.]+?)?(?:\.|$)")
def _put_up_to_n_cards(m):
    return UnknownEffect(raw_text="put up to N <type> cards <tail>")


# --- "put all <type> cards <pred> into|onto <X>" -------------------------
# Cluster (8). Tazri / Mass Polymorph / Brass Herald multi-pile mass
# distributor.
@_er(r"^put all "
     r"(?:[a-z]+(?:\s+and/or\s+[a-z]+)?\s+(?:creature|permanent|land|"
     r"noncreature|nonland|artifact|enchantment)\s+cards?"
     r"|creature cards"
     r"|(?:noncreature|nonland|nonartifact)\s+cards?)"
     r"(?:\s+with [^,]+?)?"
     r"(?:\s+(?:revealed|milled|exiled)\s+this way)?"
     r"(?:\s+of the chosen type)?"
     r"(?:\s+from (?:among (?:them|the [^,.]+?)|your graveyard))?"
     r"\s+(?:on(?:to)? the battlefield|into your hand|into your graveyard|"
     r"on the bottom of [^,]+?|on top of [^,]+?)"
     r"(?:[,.]\s+(?:then|and) [^.]+?)?(?:\.|$)")
def _put_all_typed_cards(m):
    return UnknownEffect(raw_text="put all <type> cards <pred> to <Y>")


# --- "return any number of <type> cards <pred> from your graveyard <Y>" --
# Cluster (3). Legion's Chant / Reunion / Seasons Past.
@_er(r"^return any number of "
     r"(?:[a-z]+(?:\s+and/or\s+[a-z]+)?\s+)?cards?"
     r"(?:\s+with [^,]+?|"
     r"\s+with different mana values|"
     r"\s+with total (?:mana value|power)[^,]+?)?"
     r"\s+from your graveyard "
     r"(?:to your hand|to the battlefield)"
     r"(?:[,.]\s+where x is [^.]+?)?(?:\.|$)")
def _return_any_number_from_gy(m):
    return UnknownEffect(raw_text="return any number of <type> cards from gy")


# --- "any number of target <type> [you control] <verb>" -----------------
# Cluster (3). Clever Concealment / Wheel and Deal / Gallifrey Falls.
@_er(r"^any number of target "
     r"(?:[a-z]+(?:\s+(?:and|or)\s+[a-z]+)?\s+)+"
     r"(?:you control|an opponent controls)?"
     r"\s*(?:phase out|each [a-z ]+?|gain [^.]+?|get [^.]+?)"
     r"(?:[^.]+?)?(?:\.|$)")
def _any_number_target_typed(m):
    return UnknownEffect(raw_text="any number of target <type> <verb>")


# --- "you may cast spells <pred>" ----------------------------------------
# Cluster (5). Errant and Giada / Meeting of the Five / Abandoned
# Sarcophagus / Draugr Necromancer / Borne Upon a Wind. Distinct from
# scrubber #10's ``you may cast spells from the top of your library by
# <alt-cost>``.
@_er(r"^you may cast spells "
     r"(?:with [^,]+?|"
     r"that have [^,]+?|"
     r"from (?:among cards in exile [^,]+?|your graveyard|the top of [^,]+?)|"
     r"this turn as though they had flash|"
     r"with exactly [^,]+?(?:\s+from among them)?(?:\s+this turn)?)"
     r"(?:[,.]\s+[^.]+?)?(?:\.|$)")
def _may_cast_spells_pred(m):
    return UnknownEffect(raw_text="you may cast spells <pred>")


# --- "you may look at <X>" -----------------------------------------------
# Cluster (5). Revealing Wind / Found Footage / Colfenor's Plans /
# Cogwork Spy / Enhanced Surveillance. Distinct from the existing
# ``you may look at the top card of <library>`` rules.
@_er(r"^you may look at "
     r"(?:each face-down [^.]+?|"
     r"face-down (?:creature|permanent)s? [^.]+?|"
     r"the cards exiled with [^.]+?|"
     r"the next card drafted [^.]+?|"
     r"an additional [^.]+?|"
     r"and play that card [^.]+?)"
     r"(?:[,.]\s+(?:and|then) [^.]+?)?(?:\.|$)")
def _may_look_at_x(m):
    return UnknownEffect(raw_text="you may look at <X>")


# --- "you may play <X>" --------------------------------------------------
# Cluster (8). ``you may play lands from your graveyard`` etc. The
# scrubber #10 ``you may play lands from among <X>`` is one variant;
# this catches the broader noun-pred shape.
@_er(r"^you may play "
     r"(?:an additional land [^.]+?|"
     r"the top card of [^.]+?|"
     r"that card [^.]+?|"
     r"cards? exiled with [^.]+?|"
     r"lands? from (?:your hand|your graveyard|exile)[^.]*?|"
     r"any number of [^.]+?)"
     r"(?:[,.]\s+[^.]+?)?(?:\.|$)")
def _may_play_x(m):
    return UnknownEffect(raw_text="you may play <X>")


# --- "[you] gain control of <demonstrative> <type> [<rider>]" ------------
# Cluster (9). Aura Graft / Stolen Uniform / Wellspring / Crown of
# Empires / Fumble / Murderous Spoils / Brand / Broadcast Takeover /
# Rags-Riches.
@_er(r"^(?:you )?gain control of "
     r"(?:that|those|all|the|each) "
     r"(?:[a-z]+(?:\s+and/or\s+[a-z]+)?(?:\s+permanents?|\s+creatures?|"
     r"\s+lands?|\s+artifacts?|\s+enchantments?|\s+planeswalkers?|"
     r"\s+equipment|\s+auras?|\s+vehicles?)?"
     r"(?:s)?)"
     r"(?:\s+(?:you own|your opponents control|that were attached to [^,.]+?|"
     r"chosen this way))?"
     r"(?:\s+(?:until end of turn|this turn|until your next turn|"
     r"instead if [^.]+?))?"
     r"(?:[,.]\s+(?:then|and) [^.]+?)?(?:\.|$)")
def _gain_control_of_demonstrative(m):
    return UnknownEffect(raw_text="gain control of <demonstrative> <type>")


# --- "until end of turn, <subject> <verb> <tail>" ------------------------
# Cluster (16). Generic UEOT prefix on misc subjects. Distinct from
# all prior scrubber UEOT rules. Examples: ``target player can't cast``,
# ``lands tapped for mana produce``, ``your life total can't change``,
# ``permanents you control gain X``, ``target artifact or creature
# becomes Y``, ``they become 3/3 birds``.
@_er(r"^until end of turn,?\s+"
     r"(?:target [a-z]+(?:\s+(?:or|and)\s+[a-z]+)?|"
     r"lands?|"
     r"your life total|"
     r"permanents you control|"
     r"your opponents|"
     r"each (?:player|opponent|creature)|"
     r"they|"
     r"all (?:creatures?|permanents?))"
     r"\s+(?:can'?t|gains?|gets?|has|have|deals?|become[s]?|are|is|"
     r"produce|may|loses?|can|enters?)"
     r"\s+[^.]+?(?:\.|$)")
def _ueot_misc_subject(m):
    return UnknownEffect(raw_text="until end of turn, <subject> <verb>")


# --- "draw a card if <pred>" ---------------------------------------------
# Cluster (4). Lyzolda / Roadside Reliquary / Eagle of Deliverance /
# Oblivion's Hunger conditional draw tail (a prior trigger header has
# already consumed the ``when X happens, ...`` portion).
@_er(r"^draw a card "
     r"(?:if|when|unless|where) [^.]+?(?:\.|$)")
def _draw_a_card_if(m):
    return UnknownEffect(raw_text="draw a card if <pred>")


# --- "target creature can't be <verb> <tail>" ----------------------------
# Cluster (3). Gravebind / The Black Gate / Vines of Vastwood targeted
# protection rider.
@_er(r"^target creature can'?t be "
     r"(?:regenerated|blocked|the target [^.]+?|countered|"
     r"sacrificed|targeted)"
     r"(?:\s+(?:by [^.]+?|of (?:spells? or abilities?) [^.]+?|"
     r"this turn))?"
     r"(?:\s+this turn)?(?:\.|$)")
def _target_creature_cant_be(m):
    return UnknownEffect(raw_text="target creature can't be <X>")


# --- "the first player may <verb> <tail>" --------------------------------
# Cluster (5). Oath cycle (Oath of Mages / Lieges / Ghouls / Druids /
# Scholars). The "first player" subject is the trigger of an upkeep
# trigger that already parsed; this is the orphan tail.
@_er(r"^the first player "
     r"(?:may|may have|may search|may discard|may reveal|may return) "
     r"[^.]+?(?:\.|$)")
def _first_player_may(m):
    return UnknownEffect(raw_text="the first player may <X>")


# --- "each player who <pred> <verb> <tail>" ------------------------------
# Cluster (7). Cirdan / Plaguecrafter / Infernal Offering / Fandaniel /
# Step Between Worlds / Scythe Specter / The Second Doctor / Skull
# Storm / Momentum Breaker / Entropic Battlecruiser. Each-player-who-X
# distributive.
@_er(r"^each (?:player|opponent) who "
     r"(?:can'?t|does(?:n'?t)?|received|sacrificed|discarded|shuffled|"
     r"controls?|has|doesn'?t)\s+[^,]*? "
     r"(?:may [^.]+?|"
     r"discards? [^.]+?|"
     r"loses? [^.]+?|"
     r"draws? [^.]+?|"
     r"reveals? [^.]+?|"
     r"returns? [^.]+?|"
     r"puts? [^.]+?|"
     r"can'?t [^.]+?|"
     r"gains? [^.]+?|"
     r"sacrifices? [^.]+?)(?:\.|$)")
def _each_player_who(m):
    return UnknownEffect(raw_text="each player who <pred> <verb>")


# --- Shorter "each opponent who can't <verb>" / "each player who does"
# --- bare-tail variants ------------------------------------------------
@_er(r"^each (?:player|opponent) who "
     r"(?:can'?t|does|doesn'?t|did)"
     r"\s+(?:may|discards?|loses?|draws?|reveals?|returns?|puts?|"
     r"can'?t|gains?|sacrifices?|attacks?|blocks?)\s+[^.]+?(?:\.|$)")
def _each_player_who_short(m):
    return UnknownEffect(raw_text="each player who <does> <verb>")


# --- "the owner of target <type> shuffles|puts|exiles <tail>" ------------
# Cluster (3). Chaos Warp / Lost Days / Audacious Swap.
@_er(r"^the owner of target "
     r"(?:[a-z]+(?:\s+(?:or|and)\s+[a-z]+)?|permanent|nonenchantment\s+permanent|"
     r"creature|enchantment|artifact)"
     r"\s+(?:shuffles?|puts?|exiles?|reveals?|returns?)"
     r"\s+[^.]+?(?:\.|$)")
def _owner_of_target(m):
    return UnknownEffect(raw_text="the owner of target <type> <verb>")


# --- "shuffle your library, then <verb> <tail>" --------------------------
# Cluster (3). Unexpected Results / Hazoret's Undying Fury / Creative
# Technique. Beyond scrubber #10's ``shuffle your library, then reveal
# the top card``.
@_er(r"^shuffle your library,?\s+then "
     r"(?:reveal|exile|put|draw|search|return) "
     r"[^.]+?(?:\.|$)")
def _shuffle_then_verb(m):
    return UnknownEffect(raw_text="shuffle your library, then <verb>")


# --- "for as long as <pred>, <verb> <tail>" ------------------------------
# Cluster (4). Dream-Thief's Bandana / Opportunistic Dragon / Divine
# Purge / Dimensional Breach exile-loop tail.
@_er(r"^for as long as "
     r"(?:it remains exiled|this creature remains on the battlefield|"
     r"you control [^,]+?|each of them remain exiled|"
     r"any of those cards remain exiled)[,\s]+"
     r"[^.]+?(?:\.|$)")
def _for_as_long_as(m):
    return UnknownEffect(raw_text="for as long as <pred>, <verb>")


# --- "when you play a card <tail>" / "when you play a card this way" -----
# Cluster (4). Voltaic Visionary / Juju Bubble / Fires of Mount Doom /
# Vanguard variant.
@_er(r"^when you play a card "
     r"(?:exiled with this creature|this way|this turn|from [^,.]+?)?"
     r"[,\s]+[^.]+?(?:\.|$)")
def _when_you_play_a_card_tail(m):
    return UnknownEffect(raw_text="when you play a card <tail>")


# --- "when you control no <noun>, sacrifice <X>" -------------------------
# Cluster (3). Phylactery Lich / Skeleton Ship / Island Fish Jasconius
# orphan static-trigger tail.
@_er(r"^when you control no "
     r"(?:[a-z]+s?(?:\s+with [^,]+?)?|"
     r"~s?|"
     r"permanents with [^,]+?),\s+"
     r"sacrifice [^.]+?(?:\.|$)")
def _when_you_control_no_noun(m):
    return UnknownEffect(raw_text="when you control no <noun>, sacrifice <X>")


# --- "whenever one or more <type> [<filter>] enter|die|are put" ----------
# Cluster (6). Expedition Supplier / City in a Bottle / Seer of Stolen
# Sight / Zone of Flame / Elvish Warmaster / Cloudsculpt Armorer.
# Distinct from scrubber #10's ``one or more OTHER <type>`` shape (this
# matches ``whenever one or more <type>`` without "other").
@_er(r"^whenever one or more "
     r"(?:nontoken\s+)?"
     r"(?:[a-z]+(?:s)?(?:\s+(?:and/or|and|or)\s+[a-z]+(?:s)?)*|"
     r"counters|cards|artifacts|creatures|permanents|enchantments|"
     r"lands|planeswalkers|tokens)"
     r"(?:\s+(?:you control|an opponent controls|with [^,]+?|"
     r"named [^,]+?))?"
     r"\s+(?:enter|die|leave|attack|block|are (?:put|placed|dealt)|"
     r"become|is dealt|are removed)"
     r"[^.]*?(?:\.|$)")
def _whenever_one_or_more_typed(m):
    return UnknownEffect(raw_text="whenever one or more <type> <verb>")


# --- "[you] gain control of all <type> <pred>" ---------------------------
# Cluster (3). Fumble / Broadcast Takeover / Brand mass take-control.
# Already partly covered by _gain_control_of_demonstrative; this is the
# ``all`` shape with a more permissive predicate tail.
@_er(r"^(?:you )?gain control of all "
     r"(?:[a-z]+(?:\s+and (?:equipment|auras?|artifacts?|enchantments?))?"
     r"(?:s)?)"
     r"(?:\s+(?:that were attached to [^,.]+?|"
     r"your opponents control|"
     r"you own))"
     r"(?:\s+until end of turn)?"
     r"(?:[,.]\s+(?:then|and) [^.]+?)?(?:\.|$)")
def _gain_control_all_typed(m):
    return UnknownEffect(raw_text="gain control of all <type>")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [

    # --- "at the beginning of your first upkeep" ---------------------------
    # Cluster (4). Sphinx of Foresight / Firion / similar. The base
    # ``at the beginning of your upkeep`` doesn't accept the ``first``
    # / ``next`` ordinal modifier.
    (re.compile(
        r"^at the beginning of (?:your |the )?(first|next|second|third) "
        r"(?:upkeep|end step|combat|main phase)",
        re.I),
     "beginning_of_ordinal_step", "self"),

    # --- "when you play a card exiled with this creature" ------------------
    # Cluster (2). Voltaic Visionary // Volt-Charged Berserker. The body
    # is "transform this creature".
    (re.compile(
        r"^when you play a card exiled with this creature",
        re.I),
     "you_play_exiled_card", "self"),

    # --- "when you control no <noun>, sacrifice <X>" -----------------------
    # Cluster (3). Phylactery Lich / Skeleton Ship / Island Fish Jasconius.
    (re.compile(
        r"^when you control no "
        r"(?:[a-z]+s?(?:\s+with [^,]+?)?|"
        r"~s?|"
        r"permanents with [^,]+? counters? on them)",
        re.I),
     "you_control_no_noun", "self"),

    # --- "whenever one or more <type> [<filter>] enter|die|are put" --------
    # Cluster (6). Generic typed-tribe trigger without "other" qualifier.
    (re.compile(
        r"^whenever one or more "
        r"(?:nontoken\s+)?"
        r"[a-z]+(?:s)?(?:\s+(?:and/or|and|or)\s+[a-z]+(?:s)?)*"
        r"(?:\s+(?:you control|an opponent controls|with [^,]+?))?"
        r"\s+(?:enter|die|leave|attack|block|are (?:put|placed|dealt))",
        re.I),
     "one_or_more_typed_event", "self"),

    # --- "whenever you play a <type> <tail>" -------------------------------
    # Generic when-play trigger header.
    (re.compile(
        r"^when you play a (?:land|spell|card|creature|artifact|"
        r"enchantment|planeswalker)"
        r"(?:\s+(?:this way|from your graveyard|spell|card|"
        r"named [^,]+?))?",
        re.I),
     "you_play_typed", "self"),
]
