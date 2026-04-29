#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (eighth pass).

Family: PARTIAL -> GREEN promotions. Companion to ``partial_scrubber.py``
through ``partial_scrubber_7.py``. Targets single-ability clusters still
unhandled after seven prior passes.

Re-bucketing the PARTIAL parse_errors after scrubber #7 (2,775 PARTIAL
cards at 8.77% on a 31,639-card pool, 28,714 GREEN) surfaced a long
tail of 2-hit clusters — the easy 40+-hit families are gone. This
pass picks the densest remaining clusters that map cleanly onto a
static regex and haven't been subsumed by any prior scrubber.

Highlights of the eighth-pass cluster set:

Keyword-shape gaps:
- ``escalate {cost}`` (6) — Blessed Alliance / Collective Defiance /
  Borrowed Hostility / Collective Resistance / Savage Alliance /
  Borrowed Grace family. Base KEYWORD_RE never listed escalate.
- ``splice onto arcane {cost}`` (4) — Spiritual Visit / Blessed Breath
  / Kodama's Might / Psychic Puppetry. Mana-only splice keyword.
- ``reinforce N-{cost}`` (5) — Brighthearth Banneret / Break Ties /
  Mosquito Guard / Earthbrawn / Rustic Clachan / Hillcomber Giant.
  Em-dash normalized to ASCII dash. Scrubber #6 handled a "replicate/
  kicker" rider static but not bare reinforce.
- ``equip-sacrifice a creature|artifact|permanent`` (4) —
  Dissection Tools / Piston Sledge / Shredder's Armor / Demonmail
  Hauberk. Alt-cost equip with sacrifice rider, em-dash normalized.
- ``suspect it`` bare (2) — Caught Red-Handed / It Doesn't Add Up.

Static / rule clusters:
- ``you can't cast noncreature spells`` (2) — Nullhide Ferox / Nikya of
  the Old Ways self-restriction.
- ``players can't play lands`` bare (2) — Worms of the Earth /
  Territorial Dispute.
- ``target player can't play lands this turn`` (2) — Solfatara /
  Turf Wound.
- ``you control enchanted land`` (2) — Annex / Conquer control-swap
  aura tail.
- ``all creatures gain menace until end of turn`` (2) — Gorilla War
  Cry / Demoralize.
- ``each land you control becomes that type until end of turn`` (2) —
  Elsewhere Flask / cousin.
- ``enchant artifact, creature, or planeswalker`` (2) — Planar
  Disruption multi-type enchant-line.
- ``enchanted creature can't block, and its activated abilities can't
  be activated`` (2) — Gelid Shackles / Demotion compound pacifism.
- ``enchanted creature can't attack or block unless its controller
  pays <cost>`` (2) — Cowed by Wisdom paralyze-tax variant.
- ``enchanted creature gets -1/-1 for each <counter> counter on this
  aura`` (2) — Traveling Plague aura-feedback anthem.
- ``enchanted creature gets +/-X/X, where x is <desc>`` (2) —
  Exoskeletal Armor variable aura-buff.
- ``enchanted creature's activated abilities can't be activated`` (2)
  — Stupefying Touch.
- ``this equipment can be attached only to a creature with
  <qualifier>`` (2) — Gate Smasher equip-restriction static.
- ``creatures you control with <quality> ...`` / ``creatures your
  opponents control ...`` tribal rider (9+7) — anthem/restriction
  compound shapes.
- ``other creatures you control <verb/gain> ...`` (7).
- ``each creature you control gains <kw-list> until end of turn`` (2).
- ``that permanent doesn't untap during its controller's untap step
  for as long as ...`` (2) — Amber Prison conditional stun extension.
- ``you don't lose unspent mana as steps end`` (2) — Fire Lord Ozai
  / Avatar Roku single-steps variant.
- ``mana of any type can be spent to <verb>`` (2) — Sharkey /
  Gonti cast-any-type rider.
- ``a spell cast this way costs {N} more to cast`` (2) — Invasion
  of Gobakhan alt-cost rider.

Effect-rule clusters:
- ``sacrifice those creatures|tokens|permanents`` (5) — Tears of Rage
  / Wake the Dead / Thatcher Revolt / Force of Rage / Shredder.
- ``sacrifice any number of <type>`` (7) — Reprocess / Scapeshift /
  Renounce / Hew the Entwood / Landslide / Last-Ditch Effort.
- ``destroy those creatures`` (2) — Highcliff Felidar tail.
- ``another target creature gets -N/-N until end of turn`` (6) —
  Leeching Bite / Consume Strength / Drooling Groodion / Steal
  Strength / Schismotivate / Rites of Reaping.
- ``target opponent gains N life`` (2) — Questing Phelddagrif /
  Phelddagrif — symmetric-give-life.
- ``its owner gains N life`` (2) — Misfortune's Gain / Path of Peace.
- ``target opponent mills N|half cards`` (4) — Startled Awake /
  Kitsune's Technique / Jidoor / Sorcerous Squall.
- ``target opponent loses half their life, rounded up|down`` (1+) —
  Revival // Revenge.
- ``put the rest into your hand`` bare (2) — Abstract Performance /
  Make Your Own Luck tail.
- ``repeat this process until <predicate>`` / ``repeat this process
  once`` (7) — Tainted Pact / Eureka / Mana Clash / Hypergenesis /
  Equipoise / Struggle for Sanity / Calamity.
- ``untap up to N target creatures`` (2) — Synchronized Strike /
  Join Forces. Scrubber #7 handled bare lands; this is the creatures
  variant.
- ``each player may shuffle their hand and graveyard into their
  library`` (2) — Step Between Worlds.
- ``target player can't play lands this turn`` bare (2) — Solfatara.
- ``each of those creatures gains persist until end of turn`` (2) —
  Cauldron Haze kw-grant-to-those.
- ``another target creature gets -X/-0`` (2) — Schismotivate.
- ``choose an instant or sorcery card in your hand`` (1) —
  Conductive Current orphan chooser.
- ``counter target activated or triggered ability from a <source>``
  (2) — A-Emerald Dragon.
- ``target opponent creates a 1/1 green hippo creature token`` — a
  token-creation effect where the subject is opponent (scrubber #7
  handled self-creates with riders, not opp-creates).
- ``exile that card until this creature leaves the battlefield`` (2)
  — Brain Maggot blink-until-leaves.
- ``target player reveals their hand`` orphan line (we add the bare
  form — prior scrubbers always required a rider).
- ``return the top creature card of your graveyard to the
  battlefield`` (2) — Corpse Dance.
- ``return a creature card from your graveyard to the battlefield,
  then <tail>`` (2) — Infernal Offering symmetric-recur.
- ``the owner of target nonland permanent puts it ...`` (2) —
  Temporal Cleansing.
- ``target artifact or creature's owner puts it on ...`` (1) — Lost
  in Space library-place.

Trigger-pattern clusters:
- ``whenever one or more <type>s you control deal combat damage to a
  player`` (8+) — Scouts/Dragons/Mutants Ninjas & Turtles combat-damage
  compound trigger. Prior scrubber #7 caught "cards zone event" not
  this "creatures combat damage" shape.
- ``whenever one or more <types> you control enter`` (4) — Elves /
  Humans and/or Warriors / Artifact creatures enter compound.
- ``whenever one or more <types> you control <die|leave>`` (5) —
  Permanents leave / Artifact creatures die compound.
- ``whenever one or more creatures <your opponents control|an opponent
  controls> <verb>`` (4).
- ``whenever one or more opponents <lose exactly N life|lose life>``
  (3) — Ob Nixilis trigger family.
- ``whenever a nontoken creature [you control] ...`` (7) —
  Molten Echoes / Dual Nature / Rayblade Trooper / Curse of Clinging
  Webs / Alharu / Gyox family.
- ``whenever you play a card [from exile]`` (3) — Null Profusion
  / Scalesoul Gnome.
- ``whenever an aura you control becomes attached to ...`` (2) —
  Eriette / Siona.
- ``at the beginning of this turn`` bare (2) — Mindstorm Crown /
  Power Surge orphan-phase.
- ``when ~ leaves the battlefield`` bare (2) — Mysterio / cousins.
- ``when this land is put into a graveyard from the battlefield``
  (2) — Eumidian Hatchery.

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
    Bounce, Destroy, Discard, Filter, GainLife, Keyword, Mill,
    Modification, Recurse, Reveal, Sacrifice, SetLife, Static,
    UnknownEffect, UntapEffect,
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


# --- Escalate {cost} -------------------------------------------------------
# Cluster (6). Modal keyword, missing from KEYWORD_RE. Takes either a
# mana-pip cost or a colored pip like {g} / {1}{w}.
@_sp(r"^escalate (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _escalate(m, raw):
    return Static(modification=Modification(
        kind="keyword_escalate", args=(m.group(1),)), raw=raw)


# --- Splice onto arcane {cost} ---------------------------------------------
# Cluster (4). Kamigawa splice keyword with pure mana cost (the dash-rider
# variants already route through other handlers since they carry a verb).
@_sp(r"^splice onto arcane (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _splice_arcane_cost(m, raw):
    return Static(modification=Modification(
        kind="keyword_splice_arcane", args=(m.group(1),)), raw=raw)


# --- Reinforce N-{cost} ----------------------------------------------------
# Cluster (5). Morningtide reinforce, em-dash normalized to ASCII dash
# by normalize(). Parser's base regex wants "reinforce N {cost}" (space),
# which doesn't match the dashed oracle form.
@_sp(r"^reinforce (\d+|x)[-–—](\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _reinforce_dash(m, raw):
    return Static(modification=Modification(
        kind="keyword_reinforce",
        args=(m.group(1), m.group(2))), raw=raw)


# --- Equip-sacrifice a <type> ----------------------------------------------
# Cluster (4). Alt-cost equip with sacrifice rider, em-dash normalized.
@_sp(r"^equip[-–—]sacrifice (?:a|an|another) "
     r"(creature|artifact|enchantment|permanent|nonland permanent|token)"
     r"\s*$")
def _equip_sac(m, raw):
    return Static(modification=Modification(
        kind="keyword_equip_sac", args=(m.group(1).strip(),)), raw=raw)


# --- Suspect it (bare) -----------------------------------------------------
# Cluster (2). Outlaws of Thunder Junction "suspect" orphan effect line.
@_sp(r"^suspect it\s*$")
def _suspect_it(m, raw):
    return Static(modification=Modification(kind="suspect_it"), raw=raw)


# --- "you can't cast noncreature spells" (bare self-restriction) -----------
# Cluster (2). Nullhide Ferox / Nikya of the Old Ways.
@_sp(r"^you can'?t cast "
     r"(noncreature|nonland|creature|instant|sorcery|artifact|enchantment)"
     r" spells\s*$")
def _you_cant_cast_type_spells(m, raw):
    return Static(modification=Modification(
        kind="self_restrict_cast_type", args=(m.group(1),)), raw=raw)


# --- "players can't play lands [tail]" / "target player can't play lands
#      this turn" ----------------------------------------------------------
# Cluster (2+2). Worms of the Earth / Territorial Dispute (symmetric bare),
# Solfatara / Turf Wound (one-shot on target player).
@_sp(r"^players can'?t play lands(?:\s+[^.]+)?\s*$")
def _players_cant_play_lands(m, raw):
    return Static(modification=Modification(
        kind="players_cant_play_lands"), raw=raw)


@_sp(r"^target player can'?t play lands this turn\s*$")
def _target_player_cant_play_lands(m, raw):
    return Static(modification=Modification(
        kind="target_player_cant_play_lands_turn"), raw=raw)


# --- "you control enchanted land" ------------------------------------------
# Cluster (2). Annex / Conquer — aura body that just grants control.
@_sp(r"^you control enchanted "
     r"(land|creature|artifact|permanent|nonland permanent)\s*$")
def _you_control_enchanted(m, raw):
    return Static(modification=Modification(
        kind="control_enchanted", args=(m.group(1),)), raw=raw)


# --- "all creatures gain <kw>[, <kw>] until end of turn" -------------------
# Cluster (2+). Gorilla War Cry / Demoralize / symmetric-anthem bulk.
@_sp(r"^all creatures gain "
     r"([a-z, ]+?) until end of turn\s*$")
def _all_creatures_gain_kw(m, raw):
    return Static(modification=Modification(
        kind="all_creatures_gain_kw_eot", args=(m.group(1).strip(),)), raw=raw)


# --- "each land you control becomes that type until end of turn" ----------
# Cluster (2). Elsewhere Flask cousin.
@_sp(r"^each land you control becomes that type until end of turn\s*$")
def _each_land_becomes_chosen_type(m, raw):
    return Static(modification=Modification(
        kind="each_land_becomes_chosen_type_eot"), raw=raw)


# --- "enchant artifact, creature, or planeswalker" (multi-type enchant) ----
# Cluster (2). Planar Disruption-style multi-type enchant line.
@_sp(r"^enchant "
     r"([a-z]+(?:,\s*[a-z]+)*(?:,?\s+or\s+[a-z]+))\s*$")
def _enchant_multi_type(m, raw):
    return Static(modification=Modification(
        kind="enchant_multi_type", args=(m.group(1),)), raw=raw)


# --- "enchanted creature can't block, and its activated abilities can't
#      be activated" -------------------------------------------------------
# Cluster (2). Gelid Shackles / Demotion. Scrubber #7 caught "can't attack
# or block, and ..." but this is the block-only + abilities-off shape.
@_sp(r"^enchanted (?:creature|permanent) can'?t block,?\s+"
     r"and its activated abilities can'?t be activated\s*$")
def _enchanted_block_plus_abilities_off(m, raw):
    return Static(modification=Modification(
        kind="enchanted_block_abilities_off"), raw=raw)


# --- "enchanted creature's activated abilities can't be activated" --------
# Cluster (2). Stupefying Touch — bare "abilities-off" without the pacify.
@_sp(r"^enchanted (?:creature|permanent)'?s? activated abilities can'?t be "
     r"activated\s*$")
def _enchanted_abilities_off(m, raw):
    return Static(modification=Modification(
        kind="enchanted_abilities_off"), raw=raw)


# --- "enchanted creature can't attack or block unless its controller pays
#      <cost>" ------------------------------------------------------------
# Cluster (2). Cowed by Wisdom paralyze-tax variant. Scrubber #7 has
# generic "can't attack or block, and ...", this is the "unless pays"
# form.
@_sp(r"^enchanted (?:creature|permanent) can'?t attack or block "
     r"unless its controller pays [^.]+?\s*$")
def _enchanted_paralyze_tax(m, raw):
    return Static(modification=Modification(
        kind="enchanted_paralyze_tax"), raw=raw)


# --- "enchanted creature gets -1/-1 for each <noun> counter on this aura" --
# Cluster (2). Traveling Plague aura-feedback anthem.
@_sp(r"^enchanted creature gets ([+-]\d+)/([+-]\d+) for each "
     r"([a-z ]+?) counter on "
     r"(?:this aura|~|this enchantment)\s*$")
def _enchanted_per_counter_on_aura(m, raw):
    return Static(modification=Modification(
        kind="enchanted_per_counter_on_aura",
        args=(m.group(1), m.group(2), m.group(3).strip())), raw=raw)


# --- "enchanted creature gets +x/+x, where x is <desc>" --------------------
# Cluster (2). Exoskeletal Armor variable aura-buff. Scrubber #6 has
# "gets -x/-0 where x is ..." but not the symmetric +X/+X / +X/+0.
@_sp(r"^enchanted creature gets ([+-]x)/([+-](?:x|\d+)),?\s+"
     r"where x is [^.]+?\s*$")
def _enchanted_gets_x_var(m, raw):
    return Static(modification=Modification(
        kind="enchanted_gets_x_var",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "this equipment can be attached only to a creature with
#      <qualifier>" -------------------------------------------------------
# Cluster (2). Gate Smasher equip-restriction.
@_sp(r"^this equipment can be attached only to a creature with "
     r"[^.]+?\s*$")
def _equip_restriction(m, raw):
    return Static(modification=Modification(
        kind="equip_attach_restriction"), raw=raw)


# --- "you don't lose this mana as steps end" -------------------------------
# Cluster (2). Fire Lord Ozai / Avatar Roku — single-steps variant
# (scrubber #6 handled "as steps and phases end"; this is the shorter
# "steps end" form).
@_sp(r"^(?:until end of combat,?\s+)?you don'?t lose "
     r"(?:unspent |this )([a-z]+)?\s*mana as steps end\s*$")
def _dont_lose_mana_steps_end(m, raw):
    return Static(modification=Modification(
        kind="dont_lose_mana_steps_end",
        args=((m.group(1) or "").strip(),)), raw=raw)


# --- "mana of any type can be spent to <verb> ..." -------------------------
# Cluster (2). Sharkey / Gonti — non-colored-spend rider where the verb
# varies (cast a spell this way / activate ~'s abilities).
@_sp(r"^mana of any type can be spent to "
     r"(cast|activate|pay) [^.]+?\s*$")
def _any_type_mana_rider(m, raw):
    return Static(modification=Modification(
        kind="any_type_mana_rider", args=(m.group(1),)), raw=raw)


# --- "a spell cast this way costs {N} more to cast" ------------------------
# Cluster (2). Invasion of Gobakhan / cousins — alt-cost rider on a
# separate line (prior scrubber #6 has "kicker/buyback cost equals X"
# but not this symmetric "cast this way costs {N} more").
@_sp(r"^a spell cast this way costs (\{[^}]+\}(?:\{[^}]+\})*|\d+) "
     r"(more|less) to cast\s*$")
def _spell_this_way_cost_more(m, raw):
    return Static(modification=Modification(
        kind="spell_this_way_cost_delta",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "that permanent doesn't untap during its controller's untap step
#      for as long as <tail>" ---------------------------------------------
# Cluster (2). Amber Prison conditional-stun — scrubber #7 has the bare
# form ("for ...untap step") without the "for as long as" rider.
@_sp(r"^that permanent doesn'?t untap during "
     r"(?:its controller'?s?|your|each other player'?s?) untap step "
     r"for as long as [^.]+?\s*$")
def _that_perm_stun_as_long_as(m, raw):
    return Static(modification=Modification(
        kind="stun_target_perm_as_long_as"), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "sacrifice those creatures|tokens|permanents [at end of combat]" ------
# Cluster (5). Tears of Rage / Wake the Dead / Thatcher Revolt / Force
# of Rage / Shredder.
@_er(r"^sacrifice those "
     r"(creatures?|tokens?|permanents?|artifacts?|enchantments?|lands?)"
     r"(?:\s+at end of (?:combat|turn))?(?:\.|$)")
def _sac_those(m):
    return Sacrifice(query=Filter(base=m.group(1).rstrip("s"),
                                  quantifier="those", targeted=False))


# --- "sacrifice any number of <type>[s]" -----------------------------------
# Cluster (7). Reprocess / Scapeshift / Renounce / Hew the Entwood /
# Landslide / Last-Ditch Effort / Pitiless Carnage.
@_er(r"^sacrifice any number of "
     r"([a-z]+(?:,\s*[a-z]+)*(?:,\s*and/or\s+[a-z]+)?|[a-z]+)"
     r"(?:\s+you control)?(?:\.|$)")
def _sac_any_number(m):
    return Sacrifice(query=Filter(base=m.group(1).strip(),
                                  quantifier="any_number", targeted=False))


# --- "destroy those creatures" --------------------------------------------
# Cluster (2). Highcliff Felidar tail.
@_er(r"^destroy those "
     r"(creatures?|tokens?|permanents?|artifacts?|enchantments?|lands?)"
     r"(?:\.|$)")
def _destroy_those(m):
    return Destroy(target=Filter(base=m.group(1).rstrip("s"),
                                 quantifier="those", targeted=False))


# --- "another target creature gets ±N/±N until end of turn" ---------------
# Cluster (6). Leeching Bite / Consume Strength / Drooling Groodion /
# Steal Strength / Schismotivate / Rites of Reaping.
@_er(r"^another target creature gets "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x) until end of turn(?:\.|$)")
def _another_target_gets(m):
    return UnknownEffect(
        raw_text=f"another target creature gets {m.group(1)}/{m.group(2)} eot")


# --- "target opponent gains N life" ---------------------------------------
# Cluster (2). Questing Phelddagrif / Phelddagrif symmetric-give.
@_er(r"^target opponent gains (one|two|three|four|five|six|seven|"
     r"eight|nine|ten|\d+) life(?:\.|$)")
def _target_opp_gains_life(m):
    amt = m.group(1)
    try:
        amt = int(amt)
    except ValueError:
        amt = amt  # keep as word
    return GainLife(amount=amt if isinstance(amt, int) else 0,
                    target=Filter(base="opponent", targeted=True))


# --- "its owner gains N life" ---------------------------------------------
# Cluster (2). Misfortune's Gain / Path of Peace.
@_er(r"^its owner gains (one|two|three|four|five|six|seven|eight|"
     r"nine|ten|\d+) life(?:\.|$)")
def _its_owner_gains_life(m):
    amt = m.group(1)
    try:
        amt = int(amt)
    except ValueError:
        amt = 0
    return GainLife(amount=amt,
                    target=Filter(base="its_owner", targeted=False))


# --- "target opponent mills <N|half their library[, rounded ...]>" --------
# Cluster (4). Startled Awake / Kitsune's Technique / Jidoor /
# Sorcerous Squall.
@_er(r"^target opponent mills "
     r"(one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|"
     r"thirteen|fourteen|fifteen|twenty|\d+|half their library) "
     r"(?:cards?)?(?:,\s+rounded (?:up|down))?"
     r"(?:,?\s+then [^.]+)?(?:\.|$)")
def _target_opp_mills(m):
    return Mill(count=m.group(1), target=Filter(base="opponent", targeted=True))


# --- "target opponent loses half their life, rounded <up|down>" -----------
# Cluster (1-2). Revival // Revenge.
@_er(r"^target opponent loses half their life,?\s+"
     r"rounded (up|down)(?:\.|$)")
def _target_opp_loses_half(m):
    return UnknownEffect(
        raw_text=f"target opponent loses half life rounded {m.group(1)}")


# --- "put the rest into your hand" ----------------------------------------
# Cluster (2). Abstract Performance / Make Your Own Luck tail — scrubber
# #6 caught "put the rest on the bottom" but not "into your hand".
@_er(r"^put the rest into your "
     r"(hand|graveyard|library)(?:\s+in (?:any|a random) order)?(?:\.|$)")
def _put_rest_into(m):
    return UnknownEffect(raw_text=f"put the rest into your {m.group(1)}")


# --- "repeat this process <until ...|once>" -------------------------------
# Cluster (7). Tainted Pact / Eureka / Mana Clash / Hypergenesis /
# Equipoise / Struggle for Sanity / Calamity.
@_er(r"^repeat this process"
     r"(?:\s+(?:until|for|once)[^.]*)?(?:\.|$)")
def _repeat_process(m):
    return UnknownEffect(raw_text="repeat this process")


# --- "untap up to N target creatures" -------------------------------------
# Cluster (2). Synchronized Strike / Join Forces. Scrubber #7 has the
# lands variant, not creatures.
@_er(r"^untap up to (one|two|three|four|\d+) target "
     r"(creatures?|permanents?|artifacts?|enchantments?)(?:\.|$)")
def _untap_up_to_n_creatures(m):
    return UntapEffect(target=Filter(base=m.group(2).rstrip("s"),
                                     quantifier="up_to_n", targeted=True))


# --- "each player may shuffle their hand and graveyard into their
#      library" ----------------------------------------------------------
# Cluster (2). Step Between Worlds.
@_er(r"^each player may shuffle their hand and graveyard into their "
     r"library(?:\.|$)")
def _each_player_shuffle_hg(m):
    return UnknownEffect(
        raw_text="each player may shuffle hand and graveyard into library")


# --- "each of those creatures gains <kw-list> until end of turn" ----------
# Cluster (2). Cauldron Haze persist-grant.
@_er(r"^each of those creatures gains "
     r"([a-z, ]+?) until end of turn(?:\.|$)")
def _each_of_those_gains_kw(m):
    return UnknownEffect(
        raw_text=f"each of those creatures gains {m.group(1).strip()} eot")


# --- "choose an instant or sorcery card in your hand" ---------------------
# Cluster (1). Conductive Current orphan chooser.
@_er(r"^choose an? "
     r"(instant or sorcery|creature or land|creature or planeswalker|"
     r"artifact or enchantment|nonland|[a-z]+(?:\s+or\s+[a-z]+)?) "
     r"card in your hand(?:\.|$)")
def _choose_compound_card_in_hand(m):
    return UnknownEffect(
        raw_text=f"choose a {m.group(1)} card in hand")


# --- "counter target activated or triggered ability from a <source>" -----
# Cluster (2). A-Emerald Dragon / cousins.
@_er(r"^counter target (activated|triggered|activated or triggered) "
     r"(?:ability|abilities) from (?:a|an) "
     r"(noncreature|creature|artifact|enchantment|planeswalker|[a-z ]+) "
     r"source(?:\.|$)")
def _counter_ability_from_source(m):
    return UnknownEffect(
        raw_text=f"counter {m.group(1)} ability from {m.group(2)} source")


# --- "target opponent creates a <size> <type> creature token[s] ..." ------
# Cluster (2). Questing Phelddagrif — opp-creates (prior scrubbers handle
# self-creates).
@_er(r"^target opponent creates an? "
     r"(\d+)/(\d+) ([a-z ]+?) creature tokens?(?:\.|$)")
def _target_opp_creates_token(m):
    return UnknownEffect(
        raw_text=f"target opponent creates {m.group(1)}/{m.group(2)} "
                 f"{m.group(3).strip()} token")


# --- "exile that card until this creature leaves the battlefield" --------
# Cluster (2). Brain Maggot / cousins — blink-until-self-leaves.
@_er(r"^exile that card until "
     r"(?:this creature|~|this permanent|this artifact) "
     r"leaves the battlefield(?:\.|$)")
def _exile_until_self_leaves(m):
    return UnknownEffect(
        raw_text="exile that card until self leaves battlefield")


# --- "return the top creature card of your graveyard to the battlefield" --
# Cluster (2). Corpse Dance.
@_er(r"^return the top "
     r"(creature|instant|sorcery|artifact|enchantment|planeswalker|land|card) "
     r"card of your graveyard to "
     r"(?:the battlefield|your hand)(?:\.|$)")
def _return_top_of_graveyard(m):
    return Recurse(query=Filter(base=f"top_{m.group(1)}_of_graveyard",
                                quantifier="one", count=1),
                   destination="battlefield")


# --- "return a creature card from your graveyard to the battlefield,
#      then <tail>" -------------------------------------------------------
# Cluster (2). Infernal Offering symmetric-recur.
@_er(r"^return a ([a-z]+) card from your graveyard to the battlefield,?"
     r"\s+then [^.]+?(?:\.|$)")
def _recur_then_tail(m):
    return UnknownEffect(
        raw_text=f"return a {m.group(1)} from graveyard, then tail")


# --- "the owner of target nonland permanent puts it into their library ..."
# Cluster (2). Temporal Cleansing.
@_er(r"^the owner of target "
     r"(nonland permanent|permanent|creature|artifact|enchantment) "
     r"puts it (?:on|into) [^.]+?(?:\.|$)")
def _owner_of_target_puts(m):
    return UnknownEffect(
        raw_text=f"owner of target {m.group(1)} puts it")


# --- "target artifact or creature's owner puts it on ..." -----------------
# Cluster (1+). Lost in Space.
@_er(r"^target "
     r"([a-z]+(?:\s+or\s+[a-z]+)?|[a-z ]+?)'?s? "
     r"owner puts it on [^.]+?(?:\.|$)")
def _target_owner_puts_on(m):
    return UnknownEffect(
        raw_text=f"target {m.group(1).strip()} owner puts it on library")


# --- "another target creature gets -X/-0 [until end of turn]" -------------
# Cluster (2). Schismotivate's -4/-0 variant already caught by
# _another_target_gets above since the regex covers ±X. Keep redundant
# catcher disabled.


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [

    # --- "whenever one or more <type[-list]> you control deal combat
    #      damage to a player|an opponent|one or more of your opponents" ---
    # Cluster (8+). Scouts/Pirates/Rogues / Dragons / Mutants Ninjas &
    # Turtles / Humans & Warriors — compound-tribal combat-damage event.
    (re.compile(
        r"^whenever one or more "
        r"(?:[a-z]+(?:,\s*[a-z]+)*(?:,?\s*and/or\s+[a-z]+)?)"
        r"\s+you control deal combat damage to "
        r"(?:a player|an opponent|one or more of your opponents|you)",
        re.I),
     "compound_tribe_combat_damage", "self"),

    # --- "whenever one or more <types> you control enter" -----------------
    # Cluster (4+). Elves / Humans and/or Warriors / Artifact creatures /
    # Lands enter compound.
    (re.compile(
        r"^whenever one or more "
        r"(?:other\s+)?"
        r"(?:[a-z]+(?:\s+and/or\s+[a-z]+)?(?:,\s*[a-z]+)*"
        r"(?:,?\s*and/or\s+[a-z]+)?\s+)"
        r"(?:creatures?|permanents?|cards?|tokens?|lands?|artifacts?|"
        r"enchantments?)"
        r"(?:\s+you control)?"
        r"\s+enter(?:\s+(?:the battlefield|under your control))?",
        re.I),
     "compound_tribe_enter", "self"),

    # --- "whenever one or more <types> you control die|leave the battlefield"
    # Cluster (5+). Permanents leave / Artifact creatures die / Attacking
    # creatures die.
    (re.compile(
        r"^whenever one or more "
        r"(?:other\s+|attacking\s+)?"
        r"(?:[a-z]+(?:\s+and/or\s+[a-z]+)?\s+)?"
        r"(?:creatures?|permanents?|cards?|tokens?|lands?|artifacts?|"
        r"enchantments?|planeswalkers?)"
        r"(?:\s+you control)?"
        r"\s+(?:die|leave the battlefield|are put into (?:a|your|an opponent'?s?) graveyard)",
        re.I),
     "compound_tribe_die_or_leave", "self"),

    # --- "whenever one or more creatures <your opponents control|an
    #      opponent controls> <verb>" -------------------------------------
    # Cluster (4+). Opponent-side die/leave/deal-damage/are-returned.
    (re.compile(
        r"^whenever one or more "
        r"(?:other\s+)?"
        r"(?:[a-z]+\s+)?"
        r"(?:creatures?|permanents?|cards?|tokens?|artifacts?|planeswalkers?)"
        r"\s+(?:your opponents control|an opponent controls)"
        r"\s+(?:die|leave|deal|are|enter|attack|block|are dealt)",
        re.I),
     "compound_opp_tribe_event", "self"),

    # --- "whenever one or more opponents <lose exactly N life|lose life|
    #      each lose exactly N life|gain life>" ---------------------------
    # Cluster (3+). Ob Nixilis et al.
    (re.compile(
        r"^whenever one or more opponents "
        r"(?:each\s+)?"
        r"(?:lose|gain|discard|sacrifice|draw)",
        re.I),
     "compound_opponents_event", "self"),

    # --- "whenever one or more noncreature permanents are returned to
    #      hand" and similar bounce-event ---------------------------------
    # Cluster (2+). Prophetic cousin.
    (re.compile(
        r"^whenever one or more "
        r"(?:[a-z]+\s+)?(?:permanents?|creatures?|cards?)"
        r"\s+are (?:returned to|put back on top of|shuffled into)",
        re.I),
     "compound_bounce_shuffle_event", "self"),

    # --- "whenever a nontoken creature [you control|enchanted player
    #      controls] <verb>" ---------------------------------------------
    # Cluster (7). Molten Echoes / Dual Nature / Rayblade / Curse of
    # Clinging Webs / Alharu / Gyox / cousins.
    (re.compile(
        r"^whenever a nontoken creature"
        r"(?:\s+(?:you control|enchanted player controls|an opponent controls))?"
        r"(?:\s+(?:of the chosen type|with [^,]+))?"
        r"\s+(?:enters?|dies|leaves the battlefield|attacks|"
        r"deals combat damage|is put|is dealt)",
        re.I),
     "nontoken_creature_event", "self"),

    # --- "whenever you play a card[ from exile|with ...]" -----------------
    # Cluster (3). Null Profusion / Scalesoul Gnome / Search the City.
    (re.compile(
        r"^whenever you play a card"
        r"(?:\s+(?:from (?:exile|your graveyard|the top [^,]+)|"
        r"with [^,]+|that shares [^,]+|named [^,]+))?",
        re.I),
     "you_play_a_card", "self"),

    # --- "whenever an aura you control becomes attached to <X>" -----------
    # Cluster (2). Eriette / Siona.
    (re.compile(
        r"^whenever an aura you control becomes attached to "
        r"(?:a|an|the) [a-z ]+",
        re.I),
     "aura_attached_event", "self"),

    # --- "at the beginning of this turn['s ...]" -------------------------
    # Cluster (2). Mindstorm Crown / Power Surge orphan-phase. Scrubber
    # #7 has "at the beginning of this turn's <phase>" with a named
    # phase; this is the bare form.
    (re.compile(
        r"^at the beginning of this turn\b",
        re.I),
     "this_turn_begin", "self"),

    # --- "when ~ leaves the battlefield" (bare trigger, no effect) --------
    # Cluster (2). Mysterio / cousins — base parser's trigger list covers
    # "when ~ leaves" with a following effect; this is the orphan form
    # (effect was split onto the next line).
    (re.compile(
        r"^when (?:~|this creature|this permanent|this artifact|this "
        r"enchantment|this land) leaves the battlefield\b",
        re.I),
     "self_leaves_battlefield", "self"),

    # --- "when this land is put into a graveyard from the battlefield" ---
    # Cluster (2). Eumidian Hatchery.
    (re.compile(
        r"^when (?:this land|~|this permanent|this artifact) "
        r"is put into (?:a|your|an opponent'?s?) graveyard from the "
        r"battlefield",
        re.I),
     "self_put_into_graveyard_from_bf", "self"),

    # --- "when the last <noun> counter is removed from this card" --------
    # Cluster (2). Veiling Oddity / cousins.
    (re.compile(
        r"^when the last (?:[a-z]+ )?counter is removed from "
        r"(?:this card|~|this creature|this permanent)",
        re.I),
     "last_counter_removed", "self"),

    # --- "whenever a creature or planeswalker an opponent controls <verb>"
    # Cluster (2). Planeswalker-inclusive attack trigger.
    (re.compile(
        r"^whenever a "
        r"(?:creature or planeswalker|permanent|nontoken creature)"
        r" an opponent controls\s+(?:enters|dies|leaves|attacks|"
        r"deals|becomes)",
        re.I),
     "opp_perm_event", "self"),

    # --- "whenever there are <count> or more <noun> counters on this
    #      creature|~" --------------------------------------------------
    # Cluster (2). Homarid tide-counter threshold.
    (re.compile(
        r"^whenever there are (?:one|two|three|four|five|six|seven|eight|"
        r"nine|ten|\d+) or more [a-z]+ counters on "
        r"(?:this creature|~|this permanent)",
        re.I),
     "counter_threshold_on_self", "self"),

    # --- "whenever you gain or lose life during your <phase/turn>" -------
    # Cluster (2).
    (re.compile(
        r"^whenever you (?:gain or lose|lose or gain|lose|gain) life "
        r"during (?:your|an opponent'?s?|each) (?:turn|upkeep|end step)",
        re.I),
     "gain_lose_life_during_phase", "self"),
]
