#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (tenth pass).

Family: PARTIAL -> GREEN promotions. Companion to ``partial_scrubber.py``
through ``partial_scrubber_9.py``. Targets remaining single-ability
clusters after nine prior passes.

Re-bucketing the PARTIAL parse_errors after scrubber #9 (1,986 PARTIAL
cards at 6.28% on a 31,639-card pool, 29,652 GREEN, ~2,151 fragments
across ~78 6-word prefixes with count >= 2) surfaced a long tail of
2-5 hit clusters that map cleanly onto a static regex.

Highlights of the tenth-pass cluster set:

Effect-rule clusters:
- ``you lose N life for each <noun> [verb-this-way]`` (5) - Reign of
  Terror / Rain of Daggers / Revival Experiment / Blex // Search for
  Blex / Coerced Confession self-punisher tail.
- ``put one of those cards into your hand, one ...`` (3) - Telling
  Time / Maestros Charm / Moment of Truth distributive recur tail
  variants beyond scrubber #9.
- ``choose up to four target creature cards in your graveyard ...``
  (3) - mass-recur grave pick.
- ``until end of turn, it has base power and toughness <P/T> ...``
  (3) - polymorph / overlay set-PT body.
- ``you may reveal a card that shares <X> from among them ...`` (3)
  - Tajuru / Radagast / Memory variant of "may reveal".
- ``this effect doesn't remove auras [already attached to those ...]``
  (3) - turn-into-creature gotcha-rider.
- ``you may cast spells from the top of your library by ...`` (3) -
  Falco Spara / Into the Pit / Aluren-style alt-cost rider.
- ``whenever one or more other [type] [verb] ...`` (4) - generic
  whenever-tribe variant, distinct from "whenever an X you control".
- ``each player may draw up to N cards`` (2) - Truce / Temporary
  Truce shared-draw.
- ``each player may play an additional land on each of their turns``
  (2) - Rites of Flourishing / Ghirapur Orrery shared-extra-land.
- ``each player discards their hand, then ...`` (2) - Juggle the
  Performance / Ill-Gotten Gains wheel-tail.
- ``you may play lands from among <X>`` (2) - Cosima // Omenkeel /
  Hedonist's Trove play-from-exile.
- ``you may put a card that <pred> from <X> into <Y>`` (2) - Memory
  Theft / Tajuru Paragon variant.
- ``put the rest of the revealed cards <destination>`` (2) - Thought
  Dissector / Sibylline Soothsayer reveal-tail.
- ``starting with the next opponent in turn order, ...`` (2) -
  Sadistic Shell Game / Manifold Insights table-around.
- ``add x mana of any one color, where x is ...`` (2) - Empowered
  Autogenerator / Metamorphosis variable-mana.
- ``copy target instant or sorcery spell <pred>`` (2) - Increasing
  Vengeance / Expansion // Explosion variant copy.
- ``shuffle your library, then reveal ...`` (2) - shuffle-reveal head.
- ``you may cast this card from exile`` (2) - Misthollow Griffin /
  Eternal Scourge static cast-from-exile.
- ``you draw cards equal to that creature's power`` (2) - Twisted
  Justice / Vanish into Memory.
- ``until your next end step, each player may ...`` (2) - delayed
  duration cluster.
- ``you may cast creature spells with <mv|power> by paying ...`` (2) -
  Primal Prayers / Thundermane Dragon alt-cost cast.
- ``return that card to the battlefield <tail>`` (2) - Thunderkin
  Awakener / Shepherd of the Clouds.
- ``each of those creatures doesn't untap during ...`` (2) - Juvenile
  Mist Dragon / Dread Wight stall.
- ``you may activate each of those abilities only once each turn``
  (2) - Mairsil / Enigma Jewel borrowed-abilities.
- ``you may look at and play that card <duration>`` (2) - Thought-
  String Analyst / Headliner Scarlett.
- ``you gain control of that creature [if|until] ...`` (2) - Debt of
  Loyalty / Nipton Lottery.
- ``return that card to the battlefield tapped|instead`` (2).
- ``each player may attack only the nearest opponent ...`` (2) -
  Mystic Barrier / Pramikon turn-direction.
- ``the player to your right chooses a color, ...`` (2) - Paliano /
  Regicide three-color-pick.
- ``draw two cards. then you may discard <noun>`` (2) - Ill-Timed
  Explosion / Hypothesizzle compound draw-discard. (compound case;
  caught by sentence split sometimes).
- ``the player discards that card`` (2) - Doomsday Specter /
  Leshrac's Sigil terse "the player" pronoun-tail.
- ``you may put that card onto the battlefield. then ...`` (2) - Hei
  Bai / Genesis Storm post-reveal flip.
- ``you may reveal that card`` (2) - Runo Stromkirk / Delver flip
  trigger tail.
- ``you may reveal a creature card from among them and put it ...``
  (2) - Memorial to Unity / Radagast.
- ``shuffle your library, then reveal the top card`` (2) - misc.
- ``put the rest of the revealed cards into <X>`` (2) - already done.
- ``reveal the top x plus one cards of your library`` (2) - Green
  Sun's Twilight / Epiphany x+1 reveal.
- ``target opponent reveals each nonland card in their hand ...`` (2)
  - Phantasmal Extraction / Thought Rattle.
- ``search its controller's graveyard, hand, and library for ...``
  (2) - The End / Test of Talents 3-zone tutor-and-exile.

Static / rule clusters:
- ``flying, firebending N`` (2) - Avatar Aang / Ran and Shaw
  comma-keyword chain (firebending not in base KEYWORD_RE).
- ``jump - during your turn, ~ has flying`` (2) - Kain / Freya
  conditional-flying keyword body.
- ``equip-discard a card`` (2) - Pact Weapon / Murderer's Axe
  alt-cost-equip.
- ``the scavenge cost is equal to its mana cost`` (2) - Varolz /
  Young Deathclaws.
- ``spend only mana produced by treasures to cast it this way``
  (2) - A-Security Rhox / Security Rhox.
- ``you may spend (mana|blue mana|...) as though it were mana of any
  color to <activate|pay|cast>`` (4) - Manascape / Agatha /
  Quicksilver / Grell Philosopher.
- ``cast this spell only before the combat damage step`` (2) -
  Berserk / Blood Frenzy.
- ``you may cast this card from exile`` (2) - Misthollow / Eternal
  Scourge.
- ``damage isn't removed from creatures your opponents control during
  cleanup steps`` (2) - Uthgardt Fury / Patient Zero.
- ``round down each time`` (2) - Hydroid Krasis / Pox Plague
  fractional-down rider.
- ``players can't gain life this turn`` (2) - Skullcrack / Call In
  a Professional.
- ``other creatures they control can't block this turn`` (2) -
  Goblin War Cry / Eunuchs' Intrigues. Tail of "until end of turn"
  body that splits off.
- ``creatures your opponents control attack each combat if able``
  (2) - Angler Turtle / Fumiko the Lowblood.
- ``all creatures attack enchanted creature's controller each combat
  if able`` (2) - Public Enemy / A-Public Enemy.
- ``creatures with flying your opponents control get -N/-N`` (2) -
  One-Eyed Scarecrow / Smog Elemental anti-flying anthem.
- ``each creature you control that's a wolf or a werewolf gets/deals
  ...`` (2) - Howlpack Resurgence / Moonlight Hunt tribal-OR anthem.
- ``each other creature that player controls gets/becomes ...`` (2)
  - Public Execution / Craterous Stomp.
- ``creatures you control that are enchanted [or equipped] have ...``
  (2) - Halvar / Infiltrator's Magemark equipped/enchanted anthem.
- ``creatures you control but don't own get/are ...`` (2) - Garland /
  Laughing Jasper Flint stolen-anthem.
- ``creatures you control also get +1/+0 [and have <kw>] as long
  as ...`` (2) - Jetmir, Nexus of Revels conditional secondary anthem.
- ``skeletons, vampires, and zombies you control get +1/+1`` (2) -
  Death-Priest of Myrkul / A-Death-Priest of Myrkul typelist anthem.
- ``enchanted creature gets +0/+2 and assigns combat damage equal to
  its toughness rather than its power`` (2) - Gauntlets of Light /
  Treefolk Umbra.
- ``enchanted creature gets +N/+N as long as it's <pred>. otherwise,
  ...`` (2) - Bonds of Faith / Burden of Proof conditional aura body.
- ``enchanted (creature|permanent) can't attack, block, or <verb>``
  (2+2) - Revoke Privileges / Bound by Moonsilver / Bound in Gold /
  Intercessor's Arrest.
- ``it gets an additional +0/+2 and has <kw> as long as ...`` (2) -
  Bride's Gown / Groom's Finery cross-equipment.
- ``each creature you control enters with an additional +1/+1 counter
  on it for each ...`` (2) - Bioengineered Future / Coin of Mastery.
- ``other creatures you control enter with an additional +1/+1
  counter on them for each ...`` (2) - Kalain / Gev.
- ``you create a 1/1 colorless eldrazi scion creature token`` (2) -
  Grave Birthing / Abstruse Interference (without the typical sac
  ability tail).
- ``create a black zombie creature token [with ...]`` (2) - Ritual
  of the Returned / Soul Separator.
- ``you may cast spells from the top of your library by ...`` (2) -
  Falco Spara / Into the Pit alt-cost.
- ``the top card of your library has plot|is a food token`` (2) -
  Fblthp / All-You-Can-Eat Buffet self-replacement.
- ``this artifact has all activated abilities of <X>`` (2) - Territory
  Forge / Manascape Refractor borrowed-AAs.
- ``for as long as you control ~, you may <X>`` (2) - Kotose / Hama
  command-zone-friendly cast.
- ``the first time you would create one or more tokens [each turn|
  during each of your turns], you may instead ...`` (2) - Esix /
  Moonlit Meditation token-substitute.
- ``they're black zombies in addition to their other colors and types
  ...`` (2) - Grimoire of the Dead / Ghouls' Night Out.
- ``each creature spell you cast costs {N} less if it has mutate``
  (2) - Vadrik cousin.
- ``this aura deals N damage to that player`` (2) - Errant Minion /
  Power Leak aura-as-source damage.
- ``this sorcery deals N damage to target creature or planeswalker``
  (2) - Mephit's Enthusiasm / Molten Impact sorcery-source.
- ``each player may draw up to two cards`` (2) - Truce / Temporary
  Truce.

Trigger-pattern clusters:
- ``when ~ leaves the battlefield`` (2) - Stangg / Mysterio orphan
  LTB header (body lives elsewhere).
- ``when you attack with three or more creatures, ...`` (2) - Ruby
  Collector / Adanto.
- ``when you draw your third card in a turn, ...`` (2) - Emerald
  Collector / Sneaky Snacker.
- ``whenever you conjure one or more (other) cards, ...`` (2) -
  Thayan Evokers / Third Little Pig.
- ``at the beginning of this turn`` (2) - Mindstorm Crown / Power
  Surge orphan trigger header (body lives elsewhere via continuation).
- ``whenever one or more other <type> ...`` (4) - generic tribe
  "whenever one or more other".

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


# --- "flying, firebending N" / similar comma-keyword chain ----------------
# Cluster (2). Avatar Aang // Aang, Master of Elements / Ran and Shaw.
# firebending / earthbending / waterbending / airbending are Avatar:TLA
# bending keywords not in base KEYWORD_RE.
@_sp(r"^(flying|deathtouch|trample|haste|first strike|double strike|"
     r"vigilance|reach|menace|defender|lifelink|hexproof|ward),\s+"
     r"(firebending|earthbending|waterbending|airbending|energybending) "
     r"(\d+|x)\s*$")
def _kw_plus_bending(m, raw):
    return Static(modification=Modification(
        kind="kw_plus_bending",
        args=(m.group(1), m.group(2), m.group(3))), raw=raw)


# --- "jump - during your turn, ~ has flying" -----------------------------
# Cluster (2). Kain, Traitorous Dragoon / Freya Crescent. Em-dash
# normalized. Jump is a Final Fantasy keyword whose body is a
# conditional-flying static.
@_sp(r"^jump\s*[-–—]\s*during your turn,?\s+~\s+has "
     r"([a-z, ]+?)\s*$")
def _jump_keyword(m, raw):
    return Static(modification=Modification(
        kind="keyword_jump",
        args=(m.group(1).strip(),)), raw=raw)


# --- "equip-discard a card" ----------------------------------------------
# Cluster (2). Pact Weapon / Murderer's Axe. Em-dash normalized to dash.
@_sp(r"^equip\s*[-–—]\s*discard a card\s*$")
def _equip_discard(m, raw):
    return Static(modification=Modification(
        kind="keyword_equip_discard"), raw=raw)


# --- "the scavenge cost is equal to its mana cost" -----------------------
# Cluster (2). Varolz, the Scar-Striped / Young Deathclaws.
@_sp(r"^the scavenge cost is equal to its mana cost\s*$")
def _scavenge_cost_eq_mc(m, raw):
    return Static(modification=Modification(
        kind="scavenge_cost_eq_mc"), raw=raw)


# --- "spend only mana produced by treasures to cast it this way" --------
# Cluster (2). A-Security Rhox / Security Rhox.
@_sp(r"^spend only mana produced by "
     r"(treasures|treasure tokens|artifacts|creatures|elves) "
     r"to cast (?:it|this spell) this way\s*$")
def _spend_only_token_mana(m, raw):
    return Static(modification=Modification(
        kind="spend_only_token_mana",
        args=(m.group(1),)), raw=raw)


# --- "you may spend (X) mana as though it were mana of any color to ..." -
# Cluster (4). Manascape Refractor / Agatha's Soul Cauldron / Quicksilver
# Elemental / Grell Philosopher.
@_sp(r"^you may spend "
     r"(blue|black|red|white|green|colorless|any|any-color)?\s*mana "
     r"as though it were mana of any color "
     r"to (?:pay the activation costs of "
     r"(?:this artifact'?s?|this creature'?s?|those|"
     r"a (?:creature|artifact|enchantment|planeswalker)) abilities|"
     r"activate (?:abilities|those abilities) of "
     r"(?:creatures|artifacts) you control|"
     r"cast [^.]+?)\s*$")
def _may_spend_X_mana_any_color(m, raw):
    return Static(modification=Modification(
        kind="may_spend_typed_mana_any_color",
        args=(m.group(1) or "any",)), raw=raw)


# --- "cast this spell only before the combat damage step" ---------------
# Cluster (2). Berserk / Blood Frenzy timing-restriction.
@_sp(r"^cast this spell only before "
     r"(?:the combat damage step|combat|attackers are declared|"
     r"blockers are declared|damage|the [a-z ]+? step)\s*$")
def _cast_only_before_step(m, raw):
    return Static(modification=Modification(
        kind="cast_only_before_step"), raw=raw)


# --- "you may cast this card from exile" --------------------------------
# Cluster (2). Misthollow Griffin / Eternal Scourge.
@_sp(r"^you may cast this card from exile\s*$")
def _may_cast_this_from_exile(m, raw):
    return Static(modification=Modification(
        kind="may_cast_this_from_exile"), raw=raw)


# --- "damage isn't removed from creatures your opponents control during
#      cleanup steps" --------------------------------------------------
# Cluster (2). Uthgardt Fury / Patient Zero.
@_sp(r"^damage isn'?t removed from creatures your opponents control "
     r"during cleanup steps\s*$")
def _damage_not_removed_opp(m, raw):
    return Static(modification=Modification(
        kind="damage_not_removed_opp_cleanup"), raw=raw)


# --- "round down each time" / "round up each time" ----------------------
# Cluster (2). Hydroid Krasis / Pox Plague.
@_sp(r"^round (down|up) each time\s*$")
def _round_each_time(m, raw):
    return Static(modification=Modification(
        kind="round_each_time",
        args=(m.group(1),)), raw=raw)


# --- "players can't gain life this turn" --------------------------------
# Cluster (2). Skullcrack / Call In a Professional.
@_sp(r"^players can'?t gain life this turn\s*$")
def _players_cant_gain_life(m, raw):
    return Static(modification=Modification(
        kind="players_cant_gain_life_this_turn"), raw=raw)


# --- "other creatures they control can't block this turn" ---------------
# Cluster (2). Goblin War Cry / Eunuchs' Intrigues.
@_sp(r"^other creatures they control can'?t block this turn\s*$")
def _other_creatures_they_cant_block(m, raw):
    return Static(modification=Modification(
        kind="other_creatures_they_cant_block_eot"), raw=raw)


# --- "creatures your opponents control attack each combat if able" ------
# Cluster (2). Angler Turtle / Fumiko the Lowblood.
@_sp(r"^creatures your opponents control attack each combat if able\s*$")
def _opp_creatures_attack_each(m, raw):
    return Static(modification=Modification(
        kind="opp_creatures_attack_each_combat"), raw=raw)


# --- "all creatures attack enchanted creature's controller each combat
#      if able" ----------------------------------------------------------
# Cluster (2). Public Enemy / A-Public Enemy.
@_sp(r"^all creatures attack enchanted creature'?s? controller "
     r"each combat if able\s*$")
def _all_attack_enchanted_controller(m, raw):
    return Static(modification=Modification(
        kind="all_attack_enchanted_controller"), raw=raw)


# --- "creatures with flying your opponents control get -N/-N" -----------
# Cluster (2). One-Eyed Scarecrow / Smog Elemental.
@_sp(r"^creatures with (flying|reach|trample|menace|haste) "
     r"your opponents control get "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x)\s*$")
def _creatures_with_kw_opp_get(m, raw):
    return Static(modification=Modification(
        kind="creatures_with_kw_opp_anthem",
        args=(m.group(1), m.group(2), m.group(3))), raw=raw)


# --- "each creature you control that's a <typeA> or a <typeB> gets ±N/±N
#      [and has <kw>]" --------------------------------------------------
# Cluster (2). Howlpack Resurgence (wolf/werewolf) / Moonlight Hunt
# variant. Distinguish from scrubber #9's typed-anthem (which is "each
# OTHER creature you control that's a typelist").
@_sp(r"^each creature you control that'?s "
     r"(?:a |an )?[a-z]+ or (?:a |an )?[a-z]+ "
     r"gets ([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"(?:\s+and has [a-z, ]+?)?\s*$")
def _each_typed_or_anthem(m, raw):
    return Static(modification=Modification(
        kind="each_typed_or_anthem",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "each other creature that player controls <verb> ..." ---------------
# Cluster (2). Public Execution / Craterous Stomp targeted-rider.
@_sp(r"^each other creature that player controls "
     r"(?:gets|becomes|has|gains|can'?t|deals?) [^.]+?\s*$")
def _each_other_creature_that_player(m, raw):
    return Static(modification=Modification(
        kind="each_other_creature_that_player_does"), raw=raw)


# --- "creatures you control that are enchanted [or equipped] have <kw>" -
# Cluster (2). Halvar // Sword of the Realms / Infiltrator's Magemark.
@_sp(r"^creatures you control that are enchanted"
     r"(?:\s+or equipped)?\s+"
     r"(?:have|get|gain) [^.]+?\s*$")
def _creatures_enchanted_have(m, raw):
    return Static(modification=Modification(
        kind="creatures_enchanted_have"), raw=raw)


# --- "creatures you control but don't own get|are ..." -------------------
# Cluster (2). Garland, Royal Kidnapper / Laughing Jasper Flint.
@_sp(r"^creatures you control but don'?t own "
     r"(?:get|are|have|gain) [^.]+?\s*$")
def _creatures_you_control_dont_own(m, raw):
    return Static(modification=Modification(
        kind="creatures_you_control_dont_own"), raw=raw)


# --- "creatures you control also get +1/+0 [and have <kw>] as long
#      as ..." ----------------------------------------------------------
# Cluster (2). Jetmir, Nexus of Revels secondary-anthem.
@_sp(r"^creatures you control also get "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"(?:\s+and (?:have|gain) [a-z, ]+?)?"
     r"\s+as long as [^.]+?\s*$")
def _creatures_also_get(m, raw):
    return Static(modification=Modification(
        kind="creatures_also_get",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "<typeA>, <typeB>, and <typeC> you control get +N/+N" ---------------
# Cluster (2). Death-Priest of Myrkul / A-Death-Priest typelist anthem.
@_sp(r"^[a-z]+,\s+[a-z]+,?\s+and [a-z]+ you control get "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x)\s*$")
def _typelist_anthem(m, raw):
    return Static(modification=Modification(
        kind="typelist_anthem",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "enchanted creature gets +0/+2 and assigns combat damage equal to
#      its toughness rather than its power" ----------------------------
# Cluster (2). Gauntlets of Light / Treefolk Umbra.
@_sp(r"^enchanted creature gets "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"\s+and assigns combat damage equal to its toughness "
     r"rather than its power\s*$")
def _enchanted_assign_by_toughness(m, raw):
    return Static(modification=Modification(
        kind="enchanted_assign_by_toughness",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "enchanted creature gets +N/+N as long as it's <pred>.
#      otherwise, ..." ---------------------------------------------------
# Cluster (2). Bonds of Faith / Burden of Proof conditional aura.
@_sp(r"^enchanted creature gets "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"\s+as long as it'?s [^.]+?\.\s+"
     r"otherwise,?\s+[^.]+?\s*$")
def _enchanted_gets_aslongas_otherwise(m, raw):
    return Static(modification=Modification(
        kind="enchanted_gets_aslongas_otherwise",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "enchanted (creature|permanent) can't attack, block, or <verb> ..." -
# Cluster (4). Revoke Privileges / Bound by Moonsilver / Bound in Gold /
# Intercessor's Arrest.
@_sp(r"^enchanted (creature|permanent) can'?t attack,\s+block,\s+or "
     r"(?:crew vehicles?|transform|"
     r"crew vehicles?,?\s+and its activated abilities can'?t be "
     r"activated unless they'?re mana abilities)\s*$")
def _enchanted_cant_aboc(m, raw):
    return Static(modification=Modification(
        kind="enchanted_cant_attack_block_or_X",
        args=(m.group(1),)), raw=raw)


# --- "it gets an additional +0/+2 and has <kw> as long as ..." ----------
# Cluster (2). Bride's Gown / Groom's Finery cross-equipment rider.
@_sp(r"^it gets an additional "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"\s+and has [a-z, ]+?\s+as long as [^.]+?\s*$")
def _it_gets_additional_aslongas(m, raw):
    return Static(modification=Modification(
        kind="it_gets_additional_aslongas",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "each creature you control enters with an additional +1/+1 counter
#      on it for each ..." ----------------------------------------------
# Cluster (2). Bioengineered Future / Coin of Mastery.
@_sp(r"^each creature you control enters with an additional "
     r"([+-]\d+)/([+-]\d+) counter on it for each [^.]+?\s*$")
def _each_creature_etb_addl(m, raw):
    return Static(modification=Modification(
        kind="each_creature_etb_additional_counter",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "other creatures you control enter with an additional +1/+1 counter
#      on them for each ..." --------------------------------------------
# Cluster (2). Kalain, Reclusive Painter / Gev, Scaled Scorch.
@_sp(r"^other creatures you control enter with an additional "
     r"([+-]\d+)/([+-]\d+) counter on them for each [^.]+?\s*$")
def _other_creatures_etb_addl(m, raw):
    return Static(modification=Modification(
        kind="other_creatures_etb_additional_counter",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "the top card of your library has plot|is a food token" -----------
# Cluster (2). Fblthp, Lost on the Range / All-You-Can-Eat Buffet
# self-replacement static.
@_sp(r"^the top card of your library "
     r"(?:has [a-z]+|is an? [a-z]+ token|is an? [a-z ]+ card)\s*$")
def _top_card_library_is(m, raw):
    return Static(modification=Modification(
        kind="top_card_library_is_or_has"), raw=raw)


# --- "this artifact has all activated abilities of <X>" -----------------
# Cluster (2). Territory Forge / Manascape Refractor.
@_sp(r"^this artifact has all activated abilities of "
     r"(?:the exiled card|all lands on the battlefield|"
     r"each [^.]+?|all [^.]+?|target [^.]+?)\s*$")
def _artifact_has_all_aas_of(m, raw):
    return Static(modification=Modification(
        kind="artifact_has_all_aas_of"), raw=raw)


# --- "for as long as you control ~, you may <X>" ------------------------
# Cluster (2). Kotose, the Silent Spider / Hama, the Bloodbender.
@_sp(r"^for as long as you control ~,?\s+you may [^.]+?\s*$")
def _for_as_long_as_you_control_self_may(m, raw):
    return Static(modification=Modification(
        kind="for_as_long_as_you_control_self_may"), raw=raw)


# --- "the first time you would create one or more tokens [each turn|
#      during each of your turns], you may instead ..." -----------------
# Cluster (2). Esix, Fractal Bloom / Moonlit Meditation.
@_sp(r"^the first time you would create one or more tokens "
     r"(?:each turn|during each of your turns|this turn),?\s+"
     r"you may instead [^.]+?\s*$")
def _first_time_create_tokens_instead(m, raw):
    return Static(modification=Modification(
        kind="first_time_create_tokens_instead"), raw=raw)


# --- "they're black zombies in addition to their other colors and types
#      [and they gain decayed]" ----------------------------------------
# Cluster (2). Grimoire of the Dead / Ghouls' Night Out.
@_sp(r"^they'?re ([a-z]+) ([a-z]+)s? in addition to their other "
     r"colors and types"
     r"(?:\s+and they gain [a-z]+)?\s*$")
def _theyre_color_type_addition(m, raw):
    return Static(modification=Modification(
        kind="theyre_color_type_in_addition",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "each creature spell you cast costs {N} less if it has mutate" -----
# Cluster (2). Vadrik / Auspicious Starrix cousin.
@_sp(r"^each (creature|instant|sorcery|artifact|enchantment) spell you cast "
     r"costs (\{[^}]+\}|\d+) less to cast "
     r"if it has [a-z]+\s*$")
def _each_typed_spell_costs_less_if(m, raw):
    return Static(modification=Modification(
        kind="each_typed_spell_costs_less_if",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "this aura deals N damage to that player" --------------------------
# Cluster (2). Errant Minion / Power Leak (aura-as-source punisher).
@_sp(r"^this aura deals (one|two|three|four|five|six|seven|eight|nine|"
     r"ten|x|\d+) damage to "
     r"(?:that player|enchanted creature'?s? controller)\s*$")
def _this_aura_deals(m, raw):
    return Static(modification=Modification(
        kind="this_aura_deals",
        args=(m.group(1),)), raw=raw)


# --- "this sorcery deals N damage to target creature or planeswalker" ---
# Cluster (2). Mephit's Enthusiasm / Molten Impact.
@_sp(r"^this sorcery deals (one|two|three|four|five|six|seven|eight|nine|"
     r"ten|x|\d+) damage to "
     r"(?:target [^.]+?)\s*$")
def _this_sorcery_deals(m, raw):
    return Static(modification=Modification(
        kind="this_sorcery_deals",
        args=(m.group(1),)), raw=raw)


# --- "create a black zombie creature token [with ...]" ------------------
# Cluster (2). Ritual of the Returned / Soul Separator. Without P/T
# (token spec is computed from card data) — a static-style token recipe.
@_sp(r"^create a (black|white|blue|red|green|colorless) "
     r"([a-z]+) creature token"
     r"(?:\s+with [^.]+?)?\s*$")
def _create_color_typed_token_no_pt(m, raw):
    return Static(modification=Modification(
        kind="create_color_typed_token_no_pt",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "you create a 1/1 colorless eldrazi scion creature token" -----------
# Cluster (2). Grave Birthing / Abstruse Interference (orphan recipe).
@_sp(r"^you create a (\d+)/(\d+) colorless "
     r"(eldrazi scion|eldrazi spawn|construct|servo|thopter) "
     r"(?:artifact )?creature token\s*$")
def _create_eldrazi_scion_token(m, raw):
    return Static(modification=Modification(
        kind="create_colorless_typed_token",
        args=(m.group(1), m.group(2), m.group(3))), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "you lose N life for each <noun> [verb] this way" ------------------
# Cluster (5). Reign of Terror / Rain of Daggers / Revival Experiment /
# Blex // Search for Blex / Coerced Confession self-punisher tail.
@_er(r"^you lose (one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) "
     r"life for each [^.]+?(?:\.|$)")
def _you_lose_n_life_for_each(m):
    return UnknownEffect(
        raw_text=f"you lose {m.group(1)} life for each X")


# --- "put one of those cards into your hand, one [into|on] ..." ---------
# Cluster (3). Telling Time / Maestros Charm / Moment of Truth distrib.
# Extends scrubber #9 which only catches the simpler variant.
@_er(r"^put one of those cards into "
     r"(?:your hand|that player'?s? graveyard|your graveyard|"
     r"the bottom of your library|on top of your library),?\s+"
     r"(?:one|and the rest|the rest) [^.]+?(?:\.|$)")
def _put_one_those_distributive(m):
    return UnknownEffect(
        raw_text="put one of those cards distributive")


# --- "choose up to four target <type> cards in your graveyard ..." ------
# Cluster (3). Mass-recur grave-pick.
@_er(r"^choose up to (one|two|three|four|five|six|seven|\d+) target "
     r"[a-z]+(?:\s+and/or\s+[a-z]+)?\s+cards "
     r"in your graveyard"
     r"(?:[^.]+?)?(?:\.|$)")
def _choose_up_to_n_cards_in_gy(m):
    return UnknownEffect(
        raw_text=f"choose up to {m.group(1)} target cards in your graveyard")


# --- "until end of turn, it has base power and toughness P/T ..." -------
# Cluster (3). Polymorph / overlay set-PT body.
@_er(r"^until end of turn,?\s+it has base power and toughness "
     r"(\d+|x)/(\d+|x)"
     r"(?:\s+and gains [^.]+?)?(?:\.|$)")
def _until_eot_it_has_base_pt(m):
    return UnknownEffect(
        raw_text=f"until eot, it has base PT {m.group(1)}/{m.group(2)}")


# --- "you may reveal a card that <pred> from among them ..." ------------
# Cluster (3). Tajuru Paragon / Radagast / Memory Theft variant.
@_er(r"^you may reveal a card that [^,]+? from among "
     r"(?:them|the (?:exiled|milled) cards|those cards)"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_reveal_card_that_from_among(m):
    return UnknownEffect(
        raw_text="you may reveal a card that <pred> from among them")


# --- "this effect doesn't remove auras [already attached to those ...]" -
# Cluster (3). Turn-into-creature gotcha-rider (Liquimetal Coating
# family).
@_er(r"^this effect doesn'?t remove auras"
     r"(?:\s+already attached to [^.]+?)?(?:\.|$)")
def _this_effect_doesnt_remove_auras(m):
    return UnknownEffect(raw_text="this effect doesn't remove auras")


# --- "you may cast spells from the top of your library by <alt-cost> in
#      addition to paying their other costs" ----------------------------
# Cluster (3). Falco Spara / Into the Pit / Aluren-style alt-cost.
@_er(r"^you may cast spells from the top of your library "
     r"by [^,]+?\s+in addition to paying their other costs(?:\.|$)")
def _may_cast_from_top_alt_cost(m):
    return UnknownEffect(
        raw_text="you may cast spells from top by alt cost")


# --- "you may cast creature spells with mana value N or less by paying
#      <alt-cost>" / "...with power N or greater from the top ..." -----
# Cluster (2). Primal Prayers / Thundermane Dragon.
@_er(r"^you may cast (creature|instant|sorcery|enchantment|artifact) "
     r"spells with "
     r"(?:mana value (?:\d+|x|one|two|three|four|five|six|seven|eight|"
     r"nine|ten) or (?:less|greater|more|fewer)|"
     r"power (?:\d+|x|one|two|three|four|five) or "
     r"(?:less|greater|more|fewer)) "
     r"(?:by paying [^,]+?|from the top of your library)"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_cast_typed_spells_alt(m):
    return UnknownEffect(
        raw_text=f"you may cast {m.group(1)} spells with X by alt")


# --- "each player may draw up to N cards" -------------------------------
# Cluster (2). Truce / Temporary Truce shared draw.
@_er(r"^each player may draw up to "
     r"(one|two|three|four|five|six|seven|\d+) cards?(?:\.|$)")
def _each_player_may_draw_up_to(m):
    return UnknownEffect(
        raw_text=f"each player may draw up to {m.group(1)} cards")


# --- "each player may play an additional land on each of their turns" --
# Cluster (2). Rites of Flourishing / Ghirapur Orrery shared extra land.
@_er(r"^each player may play "
     r"(?:an additional land|up to (?:one|two|\d+) additional lands?) "
     r"on each of their turns(?:\.|$)")
def _each_player_extra_land(m):
    return UnknownEffect(
        raw_text="each player may play additional land each turn")


# --- "each player discards their hand, then <verb>" ---------------------
# Cluster (2). Juggle the Performance / Ill-Gotten Gains wheel-tail.
@_er(r"^each player discards their hand,?\s+then "
     r"(?:conjures?|returns?|draws?|reveals?|puts?|exiles?|searches?) "
     r"[^.]+?(?:\.|$)")
def _each_player_discards_then(m):
    return UnknownEffect(raw_text="each player discards their hand then X")


# --- "you may play lands from among <X>" --------------------------------
# Cluster (2). Cosima // Omenkeel / Hedonist's Trove.
@_er(r"^you may play lands from "
     r"(?:among (?:those cards|the exiled cards|them|cards exiled with [^,.]+?)|"
     r"your graveyard)"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_play_lands_from_among(m):
    return UnknownEffect(raw_text="you may play lands from among X")


# --- "you may put a card that <pred> from <X> into <Y>" ----------------
# Cluster (2). Memory Theft / Tajuru Paragon variant.
@_er(r"^you may put a card that [^,]+? from "
     r"(?:exile|among them|that player'?s? hand|your graveyard|your hand) "
     r"(?:into (?:that player'?s? graveyard|your hand|your graveyard|"
     r"the battlefield)|onto the battlefield)"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_put_card_that_from_to(m):
    return UnknownEffect(raw_text="you may put a card that <pred> from X to Y")


# --- "put the rest of the revealed cards <destination>" -----------------
# Cluster (2). Thought Dissector / Sibylline Soothsayer.
@_er(r"^put the rest of the revealed cards "
     r"(?:into (?:that player'?s? graveyard|your graveyard|your hand)|"
     r"on the bottom of (?:that player'?s?|your) library "
     r"in (?:any|a random) order)(?:\.|$)")
def _put_rest_revealed(m):
    return UnknownEffect(raw_text="put the rest of the revealed cards")


# --- "starting with the next opponent in turn order, ..." ---------------
# Cluster (2). Sadistic Shell Game / Manifold Insights round-the-table.
@_er(r"^starting with the next opponent in turn order,?\s+"
     r"(?:each player|each opponent) [^.]+?(?:\.|$)")
def _starting_next_opp_turn_order(m):
    return UnknownEffect(
        raw_text="starting with the next opponent in turn order, X")


# --- "add x mana of any one color, where x is ..." ----------------------
# Cluster (2). Empowered Autogenerator / Metamorphosis.
@_er(r"^add x mana of any one color,?\s+where x is [^.]+?(?:\.|$)")
def _add_x_mana_any_one_color_where(m):
    return UnknownEffect(raw_text="add x mana of any one color where x is")


# --- "copy target instant or sorcery spell <pred>" ----------------------
# Cluster (2). Increasing Vengeance / Expansion // Explosion (qualifier
# beyond the base copy rule).
@_er(r"^copy target (?:instant|sorcery|instant or sorcery) spell "
     r"(?:you control|with mana value [^,.]+?|with [^.]+?)"
     r"(?:[,.][^.]+?)?(?:\.|$)")
def _copy_target_is_qualified(m):
    return UnknownEffect(raw_text="copy target instant or sorcery spell qual")


# --- "shuffle your library, then reveal the top card" -------------------
# Cluster (2). Generic shuffle-then-reveal.
@_er(r"^shuffle your library,?\s+then reveal the top "
     r"(?:card|(?:one|two|three|four|five|six|seven|\d+) cards?) "
     r"(?:of (?:your library|it))?(?:\.|$)")
def _shuffle_then_reveal_top(m):
    return UnknownEffect(raw_text="shuffle your library, then reveal top")


# --- "you draw cards equal to that creature's power" --------------------
# Cluster (2). Twisted Justice / Vanish into Memory.
@_er(r"^you draw cards equal to "
     r"(?:that creature'?s? (?:power|toughness|mana value)|"
     r"its (?:power|toughness)|the number of [^.]+?)(?:\.|$)")
def _you_draw_cards_eq(m):
    return UnknownEffect(raw_text="you draw cards equal to X")


# --- "until your next end step, each player may ..." --------------------
# Cluster (2). Delayed-duration multi-player rider.
@_er(r"^until your next end step,?\s+"
     r"(?:each player|each opponent|that player|target player) "
     r"may [^.]+?(?:\.|$)")
def _until_next_end_step_each_may(m):
    return UnknownEffect(
        raw_text="until your next end step, each may X")


# --- "return that card to the battlefield <tail>" -----------------------
# Cluster (2). Thunderkin Awakener / Shepherd of the Clouds.
@_er(r"^return that card to the battlefield "
     r"(?:tapped(?:\s+and attacking)?|instead(?:\s+if [^.]+?)?|"
     r"under (?:its owner'?s?|your) control)"
     r"(?:[^.]+?)?(?:\.|$)")
def _return_that_card_to_bf_tail(m):
    return UnknownEffect(raw_text="return that card to the battlefield <tail>")


# --- "each of those creatures doesn't untap during ..." -----------------
# Cluster (2). Juvenile Mist Dragon / Dread Wight.
@_er(r"^each of those creatures doesn'?t untap during "
     r"(?:its controller'?s?(?:\s+next)?|the next) untap step"
     r"(?:\s+for as long as [^.]+?)?(?:\.|$)")
def _each_those_doesnt_untap(m):
    return UnknownEffect(raw_text="each of those creatures doesn't untap")


# --- "you may activate each of those abilities only once each turn" -----
# Cluster (2). Mairsil / Enigma Jewel.
@_er(r"^you may activate each of those abilities only once each turn(?:\.|$)")
def _may_activate_each_only_once(m):
    return UnknownEffect(
        raw_text="you may activate each of those abilities only once each turn")


# --- "you may look at and play that card <duration>" --------------------
# Cluster (2). Thought-String Analyst / Headliner Scarlett.
@_er(r"^you may look at and play that card "
     r"(?:this turn|for as long as it remains exiled[^.]*?)"
     r"(?:[,.][^.]+?)?(?:\.|$)")
def _may_look_at_and_play_that_card(m):
    return UnknownEffect(raw_text="you may look at and play that card")


# --- "you gain control of that creature [if|until ...]" -----------------
# Cluster (2). Debt of Loyalty / Nipton Lottery.
@_er(r"^you gain control of that creature"
     r"(?:\s+(?:if|until|for as long as) [^.]+?)?(?:\.|$)")
def _you_gain_control_of_that_creature(m):
    return UnknownEffect(raw_text="you gain control of that creature")


# --- "each player may attack only the nearest opponent ..." -------------
# Cluster (2). Mystic Barrier / Pramikon, Sky Rampart.
@_er(r"^each player may attack only the nearest opponent "
     r"(?:in [^.]+?)?(?:\.|$)")
def _each_player_attack_nearest(m):
    return UnknownEffect(raw_text="each player may attack only the nearest")


# --- "the player to your right chooses a color, ..." --------------------
# Cluster (2). Paliano, the High City / Regicide three-color-pick.
@_er(r"^the player to your "
     r"(right|left) chooses a color,?\s+you choose another color,?\s+"
     r"then the player to your "
     r"(left|right) chooses a third color(?:\.|$)")
def _three_color_pick_around(m):
    return UnknownEffect(raw_text="player to your right/left chooses color")


# --- "the player discards that card" ------------------------------------
# Cluster (2). Doomsday Specter / Leshrac's Sigil terse "the player".
@_er(r"^the player discards that card(?:\.|$)")
def _the_player_discards_that(m):
    return UnknownEffect(raw_text="the player discards that card")


# --- "you may put that card onto the battlefield. then ..." -------------
# Cluster (2). Hei Bai / Genesis Storm — chained tail.
@_er(r"^you may put that card onto the battlefield\.\s+then "
     r"(?:shuffle|put [^.]+?|each player [^.]+?|exile [^.]+?)"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_put_that_card_then(m):
    return UnknownEffect(raw_text="you may put that card to bf, then X")


# --- "you may reveal that card" -----------------------------------------
# Cluster (2). Runo Stromkirk / Delver of Secrets terse flip-trigger tail.
@_er(r"^you may reveal that card(?:\.|$)")
def _may_reveal_that_card(m):
    return UnknownEffect(raw_text="you may reveal that card")


# --- "you may reveal a creature card from among them and put it ..." ----
# Cluster (2). Memorial to Unity / Radagast variant of "may reveal".
@_er(r"^you may reveal a "
     r"(creature|land|artifact|enchantment|planeswalker|instant|sorcery|"
     r"nonland|noncreature)"
     r"(?:\s+card that [^,]+?)?\s+card "
     r"from "
     r"(?:among (?:them|those cards|the [^.]+?))"
     r"(?:\s+and put it into your hand)?"
     r"(?:[,.][^.]+?)?(?:\.|$)")
def _may_reveal_typed_card_from_among(m):
    return UnknownEffect(
        raw_text=f"you may reveal a {m.group(1)} card from among them")


# --- "reveal the top x plus one cards of your library [...]" -----------
# Cluster (2). Green Sun's Twilight / Epiphany at the Drownyard.
@_er(r"^reveal the top x plus one cards of your library"
     r"(?:[^.]+?)?(?:\.|$)")
def _reveal_top_x_plus_one(m):
    return UnknownEffect(raw_text="reveal the top x plus one cards")


# --- "target opponent reveals each nonland card in their hand ..." ------
# Cluster (2). Phantasmal Extraction / Thought Rattle.
@_er(r"^target opponent reveals each nonland card in their hand"
     r"(?:\s+with mana value [^.]+?)?(?:\.|$)")
def _target_opp_reveals_each_nonland(m):
    return UnknownEffect(
        raw_text="target opponent reveals each nonland card in hand")


# --- "search its controller's graveyard, hand, and library for ..." ----
# Cluster (2). The End / Test of Talents 3-zone tutor-and-exile.
@_er(r"^search its controller'?s? graveyard,\s+hand,\s+and library "
     r"for [^.]+?(?:\.|$)")
def _search_controller_three_zones(m):
    return UnknownEffect(
        raw_text="search controller's graveyard, hand, and library")


# --- "draw two cards. then you may discard <a card|two cards|a nonland
#      card>" -----------------------------------------------------------
# Cluster (2). Ill-Timed Explosion / Hypothesizzle compound.
@_er(r"^draw (one|two|three|four|five|x|\d+) cards\.\s+"
     r"then you may discard "
     r"(?:a card|(?:one|two|three|four|x|\d+) cards|"
     r"a (?:nonland|noncreature|nonbasic) card)"
     r"(?:[^.]+?)?(?:\.|$)")
def _draw_n_then_may_discard(m):
    return UnknownEffect(
        raw_text=f"draw {m.group(1)} cards then discard X")


# --- "you may put any number of <type> cards <with N or less> from
#      among them onto the battlefield. then ..." ----------------------
# Cluster (2). Saheeli's Directive / Knickknack Ouphe — adds the
# "then put all cards revealed this way ..." chain that scrubber #9 cuts
# off.
@_er(r"^you may put any number of "
     r"(?:[a-z]+(?:\s+and/or\s+[a-z]+)?\s+cards|them)"
     r"(?:\s+with mana value [^,]+?)?"
     r"(?:\s+from among them)?"
     r"\s+onto the battlefield\.\s+then "
     r"put all cards revealed this way [^.]+?(?:\.|$)")
def _may_put_any_number_then(m):
    return UnknownEffect(
        raw_text="you may put any number of X to bf then put rest")


# --- "until end of turn, it <verb>" -------------------------------------
# Cluster (3). Generic "until eot, it <gets|has|gains|can't>".
# Distinct from scrubber #9's "until eot, that creature".
@_er(r"^until end of turn,?\s+it "
     r"(?:gets|has|gains|can'?t|deals?|becomes|is) [^.]+?(?:\.|$)")
def _until_eot_it_does(m):
    return UnknownEffect(raw_text="until end of turn, it does X")


# --- "that player puts the rest [on the bottom of their library ...]" ---
# Cluster (2). Generic post-look table-pile.
@_er(r"^that player puts the rest "
     r"(?:on the bottom of (?:their|that player'?s?) library "
     r"in (?:any|a random) order|"
     r"into (?:their|that player'?s?) graveyard|"
     r"into (?:their|that player'?s?) hand)(?:\.|$)")
def _that_player_puts_rest(m):
    return UnknownEffect(raw_text="that player puts the rest")


# --- "that player reveals cards from <X> until ..." ---------------------
# Cluster (2). Sideboard / library reveal-until.
@_er(r"^that player reveals cards from "
     r"(?:their sideboard|the top of their library|their hand) "
     r"(?:at random )?until [^.]+?(?:\.|$)")
def _that_player_reveals_until(m):
    return UnknownEffect(raw_text="that player reveals cards until X")


# --- "you may reveal a card <pred> and put it into your hand" -----------
# Cluster (3). Generic reveal-and-pick.
@_er(r"^you may reveal a card "
     r"(?:that [^,]+?|named [^,]+?|with [^,]+?) "
     r"and put it into your hand(?:[^.]+?)?(?:\.|$)")
def _may_reveal_a_card_and_put(m):
    return UnknownEffect(
        raw_text="you may reveal a card <pred> and put into hand")


# --- "as you draft a creature card, you may reveal it, ..." ------------
# Cluster (2). Paliano Vanguard / Noble Banneret. Draft conspiracy
# variant of "as you draft a card".
@_er(r"^as you draft a creature card,?\s+you may reveal it,\s+"
     r"note its [^,]+?,\s+then turn this card face down(?:\.|$)")
def _as_you_draft_creature_card(m):
    return UnknownEffect(
        raw_text="as you draft a creature card, you may reveal it, note its X")


# --- "shuffle your library, then reveal the top card" — also covered by
#      _shuffle_then_reveal_top above.

# --- "you may put a card that has an adventure ..." — covered above
# --- "each player may draw up to two cards" — covered above

# --- "creatures you control also get +1/+0 ..." — handled in static.

# --- "that player may cast that card without paying its mana cost. then
#      ..." -----------------------------------------------------------
# Cluster (2). Spellshift / Possibility Storm tail with chained "then".
@_er(r"^that player may cast that card without paying its mana cost\."
     r"\s+then (?:they put|the player|each player|"
     r"that player) [^.]+?(?:\.|$)")
def _that_player_may_cast_then(m):
    return UnknownEffect(
        raw_text="that player may cast wpmc, then chain")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [

    # --- "when ~ leaves the battlefield" -----------------------------------
    # Cluster (2). Stangg / Mysterio orphan LTB header. The body is
    # usually on the next line / sentence.
    (re.compile(
        r"^when ~ leaves the battlefield\s*$",
        re.I),
     "self_ltb", "self"),

    # --- "when you attack with three or more creatures" --------------------
    # Cluster (2). Ruby Collector / Adanto, the First Fort.
    (re.compile(
        r"^when you attack with "
        r"(one|two|three|four|five|six|seven|\d+) or more creatures",
        re.I),
     "you_attack_with_n_or_more", "self"),

    # --- "when you draw your third card in a turn" -------------------------
    # Cluster (2). Emerald Collector / Sneaky Snacker.
    (re.compile(
        r"^when you draw your "
        r"(?:second|third|fourth|fifth|\d+(?:nd|rd|th)) card "
        r"(?:in a turn|each turn|this turn)",
        re.I),
     "you_draw_nth_card", "self"),

    # --- "whenever you conjure one or more (other) cards" -------------------
    # Cluster (2). Thayan Evokers / Third Little Pig.
    (re.compile(
        r"^whenever you conjure one or more "
        r"(?:other\s+)?(?:cards?|[a-z]+\s+cards?)",
        re.I),
     "you_conjure_one_or_more", "self"),

    # --- "at the beginning of this turn" -----------------------------------
    # Cluster (2). Mindstorm Crown / Power Surge orphan trigger header.
    # The body is the rest of the rules text (continuation).
    (re.compile(
        r"^at the beginning of this turn\s*$",
        re.I),
     "beginning_of_this_turn", "self"),

    # --- "whenever one or more other <type|kind> ..." ----------------------
    # Cluster (4). Generic "one or more other" tribe trigger. Distinct
    # from prior "whenever one or more <type>" without "other".
    (re.compile(
        r"^whenever one or more other "
        r"(?:nontoken\s+)?(?:[a-z, ]+?(?:permanents?|creatures?|"
        r"artifacts?|enchantments?|lands?|planeswalkers?|cards?))"
        r"(?:\s+(?:you control|with [^,]+?|named [^,]+?))?\s+"
        r"(?:enters?|dies?|leaves?|attacks?|are placed|become|"
        r"is dealt|deals?)",
        re.I),
     "one_or_more_other_event", "self"),

    # --- "whenever an aura you control becomes attached to ~" — handled
    #      by scrubber #8.
]
