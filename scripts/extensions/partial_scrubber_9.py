#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (ninth pass).

Family: PARTIAL -> GREEN promotions. Companion to ``partial_scrubber.py``
through ``partial_scrubber_8.py``. Targets remaining single-ability
clusters after eight prior passes.

Re-bucketing the PARTIAL parse_errors after scrubber #8 (2,607 PARTIAL
cards at 8.24% on a 31,639-card pool, 29,029 GREEN, 2,932 distinct
parse_error fragments) surfaced a long tail of 2-4 hit clusters. The
40+-hit and even 6+-hit clusters are gone — this pass picks the
densest remaining clusters that map cleanly onto a static regex
without overlapping any earlier scrubber.

Highlights of the ninth-pass cluster set:

Effect-rule clusters:
- ``put one of those cards into <hand|gy|library> ...`` (4) — Telling
  Time / Maestros Charm / Cruel Fate distributive-recur tail.
- ``you may choose new targets for it|target spell`` (4) — Aethersnatch
  / Wild Ricochet / Commandeer redirect tail.
- ``you may reveal up to two <type> cards from among them ...`` (4) —
  Knight-Errant of Eos / Zimone's Experiment / Uncovered Clues
  reveal-and-pick mill-pile tail.
- ``sweep - return any number of <basic> you control ...`` (4) — Sink
  into Takenuma / Charge Across the Araba / Barrel Down Sokenzan
  Saviors-of-Kamigawa sweep keyword body.
- ``you may put a permanent card from among ... into your hand`` (3) —
  Cache Grab / Selvala's Stampede / Wasteful Harvest.
- ``put a land card from among them ...`` (3) — Inscribed Tablet /
  Cavalier of Thorns / Patient Naturalist mill-pick land tail.
- ``you draw a card for each <noun> [put into ... this way]`` (3) —
  Coerced Confession / Struggle for Project Purity / Eumidian
  Wastewaker self-rewarding draw rider.
- ``each player who discarded a card [this way] <verb>`` (3) — Scythe
  Specter / Borderland Explorer / Mog Moogle Warrior post-discard
  punisher.
- ``that player or that planeswalker's controller <discards|may>`` (3)
  — Rakdos's Return / Blightning / Flaming Gambit player-or-pw
  redirect tail.
- ``up to one other target creature gets/blocks/...`` (3) — Rally
  Maneuver / Monstrous Step / Mabel's Mettle.
- ``you may exile a nonland card from it [until ...]`` (3) — Invasion
  of Gobakhan / Elite Spellbinder / Deep-Cavern Bat peek-and-exile
  tail.
- ``you may put a creature card from it onto the battlefield ...``
  (3) — Zara Renegade Recruiter / Anzrag's Rampage / Treacherous
  Urge.
- ``create a token that's a copy of <X>`` (3) — Benthic Anomaly /
  Here Comes a New Hero! / Who's That Praetor? token-copy tail.
- ``you may put any number of <type> cards ... onto the battlefield``
  (4) — Saheeli's Directive / Knickknack Ouphe / Blex etc. mass-
  enter tail.
- ``shuffle the chosen cards into your library and ...`` (2) —
  Ecological Appreciation / Threats Undetected.
- ``target player reveals their hand and ...`` (2) — Psychotic
  Episode / Addle.
- ``you choose a card revealed this way`` (2) — Psychotic Episode /
  Mind Spike post-reveal pick.
- ``each player searches their library for ... basic land card[s] ...``
  (2) — Collective Voyage / Field of Ruin.
- ``put up to two creature cards ... from among them onto the
  battlefield`` (2) — Collected Company / Michelangelo's Technique.
- ``put up to two land cards ... from among them onto the battlefield
  tapped`` (2) — Expand the Sphere / Cartographer's Survey.
- ``creatures of the chosen type have <kw>`` (2) — Steely Resolve /
  Cover of Darkness chosen-type kw-grant.
- ``until end of turn, that creature gets/gains ...`` (2) — Aurelia
  Exemplar / Malakir Rebirth post-effect rider.
- ``it deals N damage instead if <cond>`` (2) — Invasive Maneuvers /
  Lithomantic Barrage conditional-damage replacement.
- ``you may cast an instant or sorcery spell ... without paying ...``
  (2) — Muse Vortex / Talent of the Telepath top-of-pile cast tail.
- ``it can't be blocked by creatures of/with ...`` (2) — Skrelv /
  Cheeky House-Mouse evasion-grant tail.
- ``you may shuffle up to four target cards from your graveyard into
  your library`` (2) — Cathartic Parting / Devious Cover-Up.
- ``each player may put a land card from their hand onto the
  battlefield ...`` (2) — Kynaios and Tiro / Anax-Cymede land-share
  rider.
- ``return that creature to its owner's hand at end of combat`` (2) —
  Scrappy Bruiser / Arthur Marigold Knight blink-tail.
- ``return it to the battlefield tapped [under its owner's control]``
  (2) — Othelm / Nezahal post-flicker rider.
- ``return the exiled card to the battlefield under its owner's
  control`` (2) — Voyager Staff / Identity Thief flicker-resolve.
- ``draw a card if an opponent has <pred>`` (2) — Veil of Summer /
  Beza, the Bounding Spring conditional-draw tail.
- ``escape-{cost}, exile <N> other cards from your graveyard`` (2) —
  Woe Strider / Charred Graverobber alt-cost keyword (em-dash
  normalized).
- ``you may attach this equipment to it`` (2) — Cori-Steel Cutter
  free-equip rider.
- ``you may have that player shuffle`` (2) — Natural Selection /
  Portent post-look shuffle option.

Static / rule clusters:
- ``each other creature you control that's <type[s]> gets +N/+N
  [and has kw]`` (3) — Skeleton Crew / Kaheera / Hancock tribal
  conditional anthem.
- ``each creature you control with a <noun> counter on it has/can ...``
  (3) — Cathedral Acolyte / Cenn's Tactician / Sunbringer's Touch
  counter-conditional anthem.
- ``permanents you control with counters on them <gain|have> ...``
  (2) — Mutational Advantage / Innkeeper's Talent counter-conditional
  permanent-anthem.
- ``creatures you control can attack as though they didn't have
  defender`` (2) — Felothar / High Alert.
- ``creature cards you own that aren't on the battlefield have flash``
  (2) — Teferi Mage of Zhalfir / Druid of Argoth.
- ``instant and sorcery spells you cast [from hand] have/cost ...`` (2)
  — Quandrix the Proof / Vadrik Astral Archmage.
- ``spells you cast with mana value N or greater <pred>`` (2) —
  Thryx / Imoti.
- ``each legendary creature you control gets +1/+1 [for each ...]``
  (2) — Mirror Box / Heroes' Podium legendary-anthem.
- ``players can't pay life or sacrifice <type> to cast/activate ...``
  (2) — Angel of Jubilation / Yasharn.
- ``a deck can have up to <N> cards named ~`` (2) — Nazgûl / Seven
  Dwarves deck-construction rule.
- ``it can't be blocked by creatures of that color this turn`` /
  ``with power N or greater ...`` (2).

Trigger-pattern clusters:
- ``whenever you sacrifice one or more <noun>`` (3) — Blood Hypnotist
  / Forge Boss / cousins. Distinct from prior "sacrificed" patterns.
- ``when enchanted creature leaves the battlefield, ...`` (3) —
  Traveling Plague / Funeral March / Valor of the Worthy aura LTB.
- ``when that mana is spent to <verb>`` (3) — Primal Amulet /
  Path of Ancestry / Pyromancer's Goggles delayed-on-spend.
- ``whenever you create one or more (creature) tokens`` (2) — Staff
  of the Storyteller / Akim, Soaring Wind.
- ``whenever a modified creature you control <verb>`` (2) — Costume
  Closet / Guardian of the Forgotten.
- ``whenever you discard a noncreature, nonland card`` (2) — Surly
  Badgersaur / Bone Miser.
- ``whenever a face-down creature you control <dies|attacks|...>``
  (2) — Cryptic Pursuit / Yarus.
- ``whenever another permanent you control leaves the battlefield``
  (2) — Angelic Sleuth / Suki.
- ``whenever an <tribe> you control dies`` (2) — Elderfang Venom /
  Bishop of Wings / Rampage of the Valkyries angel/elf-die.
- ``whenever a player wins a coin flip`` (2) — Zndrsplt / Okaun.
- ``whenever ~ and/or one or more other <type> ... enter`` (2) —
  Anje Maid of Dishonor / Satoru, the Infiltrator self-or-tribe ETB.
- ``whenever combat damage is dealt to you [or a planeswalker ...]``
  (2) — Risona / Vengeful Pharaoh.
- ``whenever you reveal an instant or sorcery card this way`` (2) —
  God-Eternal Kefnet / Inquisitor Eisenhorn delayed-reveal trigger.
- ``whenever an aura you control becomes attached to <X>`` already in
  scrubber #8 — skipped.

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
    Filter, Modification, Recurse, Sacrifice, Static, UnknownEffect,
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


# --- "each other creature you control that's a|an <typelist> gets ±N/±N
#      [and has <kw>]" -----------------------------------------------------
# Cluster (3). Skeleton Crew (Pirate) / Kaheera (cat/elemental/...) /
# Hancock (zombie or mutant). Tribal conditional anthem. Prior
# scrubbers handle "each other <tribe> creature" but not "creature that's
# a <type>" qualifier shape.
@_sp(r"^each other creature you control that'?s "
     r"(?:a |an )?[a-z, ]+(?:\s+or\s+[a-z]+)? "
     r"gets ([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"(?:\s+and has [a-z, ]+?)?\s*$")
def _each_other_typed_anthem(m, raw):
    return Static(modification=Modification(
        kind="each_other_typed_creature_anthem",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "each creature you control with a <noun> counter on it
#      has|can|gains <kw|effect>" ------------------------------------------
# Cluster (3). Cathedral Acolyte / Cenn's Tactician / Sunbringer's Touch.
@_sp(r"^each creature you control with (?:a|an) "
     r"([a-z+/0-9-]+)\s+counter on it "
     r"(?:has|can|gains) [^.]+?\s*$")
def _each_creature_with_counter(m, raw):
    return Static(modification=Modification(
        kind="each_creature_with_counter_anthem",
        args=(m.group(1).strip(),)), raw=raw)


# --- "permanents you control with counters on them <have|gain> <kw>" ------
# Cluster (2). Mutational Advantage / Innkeeper's Talent.
@_sp(r"^permanents you control with counters on them "
     r"(?:have|gain) [^.]+?\s*$")
def _perms_with_counters_anthem(m, raw):
    return Static(modification=Modification(
        kind="perms_with_counters_anthem"), raw=raw)


# --- "creatures you control can attack as though they didn't have
#      defender" -----------------------------------------------------------
# Cluster (2). Felothar the Steadfast / High Alert.
@_sp(r"^creatures you control can attack as though they "
     r"didn'?t have defender\s*$")
def _creatures_attack_despite_defender(m, raw):
    return Static(modification=Modification(
        kind="creatures_attack_despite_defender"), raw=raw)


# --- "creature cards you own that aren't on the battlefield have flash" ---
# Cluster (2). Teferi, Mage of Zhalfir / Teferi, Druid of Argoth.
@_sp(r"^creature cards you own that aren'?t on the battlefield "
     r"have ([a-z]+)\s*$")
def _creature_cards_have_kw(m, raw):
    return Static(modification=Modification(
        kind="off_bf_creatures_have_kw",
        args=(m.group(1),)), raw=raw)


# --- "instant and sorcery spells you cast [from your hand] <pred>" --------
# Cluster (2). Quandrix the Proof / Vadrik Astral Archmage.
@_sp(r"^instant and sorcery spells you cast"
     r"(?:\s+from your hand)?"
     r"\s+(?:have|cost|can'?t be) [^.]+?\s*$")
def _is_spells_static(m, raw):
    return Static(modification=Modification(
        kind="is_spells_cost_or_have"), raw=raw)


# --- "spells you cast with mana value N or greater <pred>" ----------------
# Cluster (2). Thryx the Sudden Storm / Imoti, Celebrant of Bounty.
@_sp(r"^spells you cast with mana value (\d+|x|one|two|three|four|five|"
     r"six|seven|eight|nine|ten) or (greater|less|more|fewer) "
     r"[^.]+?\s*$")
def _spells_with_mv_static(m, raw):
    return Static(modification=Modification(
        kind="spells_with_mv_static",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "each legendary creature you control gets +1/+1 [for each ...]" ------
# Cluster (2). Mirror Box / Heroes' Podium.
@_sp(r"^each legendary creature you control gets "
     r"([+-]\d+)/([+-]\d+)(?:\s+for each [^.]+?)?\s*$")
def _legendary_anthem(m, raw):
    return Static(modification=Modification(
        kind="legendary_anthem",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "players can't pay life or sacrifice <type> to cast spells or
#      activate abilities" ------------------------------------------------
# Cluster (2). Angel of Jubilation / Yasharn, Implacable Earth.
@_sp(r"^players can'?t pay life or sacrifice "
     r"(creatures|nonland permanents|permanents|artifacts|enchantments|lands) "
     r"to cast spells or activate abilities\s*$")
def _players_cant_pay_or_sac(m, raw):
    return Static(modification=Modification(
        kind="players_cant_pay_or_sac",
        args=(m.group(1),)), raw=raw)


# --- "a deck can have up to <N|word> cards named ~" -----------------------
# Cluster (2). Nazgûl / Seven Dwarves — deck-construction static.
@_sp(r"^a deck can have up to "
     r"(one|two|three|four|five|six|seven|eight|nine|ten|\d+|~) "
     r"cards named ~\s*$")
def _deck_can_have_up_to(m, raw):
    return Static(modification=Modification(
        kind="deck_can_have_up_to",
        args=(m.group(1),)), raw=raw)


# --- "it can't be blocked by creatures of that color this turn" /
#      "...with power N or greater this turn" ----------------------------
# Cluster (2). Skrelv / Cheeky House-Mouse — orphan post-target evasion.
@_sp(r"^it can'?t be blocked by creatures "
     r"(?:of that color|of the chosen color|with power "
     r"(?:one|two|three|four|five|six|seven|eight|nine|ten|\d+) "
     r"or (?:greater|more|less|fewer)|with [^.]+?)\s+this turn\s*$")
def _it_cant_be_blocked_evasion(m, raw):
    return Static(modification=Modification(
        kind="it_cant_be_blocked_evasion_eot"), raw=raw)


# --- "creatures of the chosen type have <kw>" -----------------------------
# Cluster (2). Steely Resolve (shroud) / Cover of Darkness (fear).
@_sp(r"^creatures of the chosen "
     r"(type|color|name)\s+have ([a-z, ]+?)\s*$")
def _creatures_of_chosen_have_kw(m, raw):
    return Static(modification=Modification(
        kind="creatures_of_chosen_have_kw",
        args=(m.group(1), m.group(2).strip())), raw=raw)


# --- "escape-{cost}, exile <N> other cards from your graveyard" -----------
# Cluster (2). Woe Strider / Charred Graverobber. Em-dash normalized to
# ASCII dash by normalize(). Not in base KEYWORD_RE.
@_sp(r"^escape[-–—](\{[^}]+\}(?:\{[^}]+\})*),\s+"
     r"exile (one|two|three|four|five|six|seven|eight|nine|ten|\d+) "
     r"other cards? from your graveyard\s*$")
def _escape_keyword(m, raw):
    return Static(modification=Modification(
        kind="keyword_escape",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "as you draft a card, you may <verb>" --------------------------------
# Cluster (3). Animus of Predation / Smuggler Captain / Cogwork Grinder
# — Conspiracy draft-triggered ability shape.
@_sp(r"^as you draft a card,?\s+you may [^.]+?\s*$")
def _as_you_draft(m, raw):
    return Static(modification=Modification(kind="as_you_draft_may"), raw=raw)


# --- "this enchantment deals N damage to <X>" -----------------------------
# Cluster (3). Cindervines / Desolation / Path to the World Tree —
# enchantment as damage source orphan tail.
@_sp(r"^this enchantment deals (one|two|three|four|five|six|seven|eight|"
     r"nine|ten|x|\d+) damage to [^.]+?\s*$")
def _this_enchantment_deals(m, raw):
    return Static(modification=Modification(
        kind="this_enchantment_deals",
        args=(m.group(1),)), raw=raw)


# --- "this creature deals N damage to <that player|that creature|that
#      creature's controller|that planeswalker's controller>" -------------
# Cluster (3). Dwarven Sea Clan / Keeper of the Flame / Grotag Siege-Runner.
@_sp(r"^this creature deals (one|two|three|four|five|six|seven|eight|"
     r"nine|ten|x|\d+) damage to "
     r"(?:that player|that creature|that creature'?s? controller|"
     r"that planeswalker'?s? controller|that permanent'?s? controller)"
     r"(?:\s+at end of (?:combat|turn))?\s*$")
def _this_creature_deals_to_that(m, raw):
    return Static(modification=Modification(
        kind="this_creature_deals_to_that",
        args=(m.group(1),)), raw=raw)


# --- "this creature has protection from each <quality>" ------------------
# Cluster (2). Mirror Golem / Lavabrink Venturer dynamic-protection.
@_sp(r"^this creature has protection from each "
     r"(?:of the exiled card'?s? card types|"
     r"mana value of the chosen quality|"
     r"[a-z ]+?)\s*$")
def _this_creature_protection_each(m, raw):
    return Static(modification=Modification(
        kind="this_creature_protection_each"), raw=raw)


# --- "this creature can attack as though it didn't have defender ..." /
#      "as though it had haste unless ..." -------------------------------
# Cluster (2). Ogre Jailbreaker / Chaos Lord conditional-attack-restriction.
@_sp(r"^this creature can attack as though it "
     r"(?:didn'?t have defender|had haste)"
     r"(?:\s+(?:as long as|unless|if) [^.]+?)?\s*$")
def _this_creature_can_attack_as(m, raw):
    return Static(modification=Modification(
        kind="this_creature_can_attack_as_though"), raw=raw)


# --- "that land is an island in addition to its other types for as long
#      as it has a flood counter on it" ----------------------------------
# Cluster (2). Xolatoyac / Aquitect's Will.
@_sp(r"^that land is an? "
     r"(plains|island|swamp|mountain|forest|gate|cave|sphere)"
     r"(?:\s+in addition to its other types)?"
     r"(?:\s+for as long as [^.]+?)?\s*$")
def _that_land_is_basic(m, raw):
    return Static(modification=Modification(
        kind="that_land_is_basic_type",
        args=(m.group(1),)), raw=raw)


# --- "the blitz cost is equal to its mana cost" / cousin ------------------
# Cluster (2). Henzie "Toolbox" Torre / Riveteers Provocateur.
@_sp(r"^the (blitz|disturb|spectacle|prowl|surge|escape) cost is equal to "
     r"[^.]+?\s*$")
def _alt_cost_is_equal_to(m, raw):
    return Static(modification=Modification(
        kind="alt_cost_is_equal_to",
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


# --- "put one of those cards into your hand[ ...rest tail]" ---------------
# Cluster (4). Telling Time / Maestros Charm / Cruel Fate / Auntie's
# Sentence distributive recur tail.
@_er(r"^put one of those cards into "
     r"(?:your hand|that player'?s? graveyard|your graveyard)"
     r"(?:,?\s+(?:and|then) [^.]+?)?(?:\.|$)")
def _put_one_of_those(m):
    return UnknownEffect(raw_text="put one of those cards into hand/gy")


# --- "you may choose new targets for it|target ... spell" -----------------
# Cluster (4). Aethersnatch / Wild Ricochet / Commandeer redirect tail.
@_er(r"^you may choose new targets for "
     r"(?:it|that spell|the copy|target [^.]+?)(?:\.|$)")
def _new_targets_for(m):
    return UnknownEffect(raw_text="you may choose new targets for X")


# --- "you may reveal up to two <typelist> cards from among them ..." ------
# Cluster (4). Knight-Errant of Eos / Zimone's Experiment / Uncovered
# Clues reveal-pile tail.
@_er(r"^you may reveal up to (one|two|three|four|five|\d+) "
     r"([a-z]+(?:\s+and/or\s+[a-z]+)?(?:,\s*[a-z]+)*)\s+cards "
     r"(?:from among them|from among the [^.]+?)[^.]*?(?:\.|$)")
def _may_reveal_up_to_n(m):
    return UnknownEffect(
        raw_text=f"you may reveal up to {m.group(1)} {m.group(2).strip()} cards")


# --- "sweep - return any number of <basic> you control ..." ---------------
# Cluster (4). Sink into Takenuma / Charge Across the Araba / Barrel Down
# Sokenzan Saviors of Kamigawa keyword body. Em-dash normalized.
@_er(r"^sweep[-–—]\s*return any number of "
     r"(plains|islands|swamps|mountains|forests|lands) you control "
     r"to (?:their|its) owner'?s? hand(?:\.|$)")
def _sweep_keyword(m):
    return UnknownEffect(raw_text=f"sweep return {m.group(1)}")


# --- "you may put a permanent card from <X> [into your hand|onto bf]" -----
# Cluster (3). Cache Grab / Selvala's Stampede / Wasteful Harvest.
@_er(r"^you may put a permanent card from "
     r"(?:among (?:them|the cards [^,.]*)|your hand) "
     r"(?:into your hand|onto the battlefield)"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_put_permanent_card(m):
    return UnknownEffect(raw_text="you may put a permanent card from X")


# --- "put a land card from among them <into hand|onto bf|graveyard>" ------
# Cluster (3). Inscribed Tablet / Cavalier of Thorns / Patient Naturalist.
@_er(r"^put a land card from among "
     r"(?:them|the milled cards|those cards) "
     r"(?:into your hand|onto the battlefield|into your graveyard)"
     r"(?:[^.]+?)?(?:\.|$)")
def _put_land_card_from_among(m):
    return UnknownEffect(raw_text="put a land card from among them")


# --- "you draw a card for each <noun>" ------------------------------------
# Cluster (3). Coerced Confession / Struggle for Project Purity / Eumidian
# Wastewaker self-rewarding draw.
@_er(r"^you draw a card for each [^.]+?(?:\.|$)")
def _you_draw_for_each(m):
    return UnknownEffect(raw_text="you draw a card for each")


# --- "each player who discarded a card [this way] <verb>" -----------------
# Cluster (3). Scythe Specter / Borderland Explorer / Mog Moogle Warrior.
@_er(r"^each player who discarded a card"
     r"(?:\s+this way)?\s+(?:loses|may search|draws|gains|"
     r"sacrifices|discards|reveals|puts) [^.]+?(?:\.|$)")
def _each_player_who_discarded(m):
    return UnknownEffect(raw_text="each player who discarded a card")


# --- "that player or that planeswalker's controller <verb> ..." -----------
# Cluster (3). Rakdos's Return / Blightning / Flaming Gambit punisher
# redirect.
@_er(r"^that player or that planeswalker'?s? controller "
     r"(?:discards?|loses?|sacrifices?|may [^,]+?)"
     r"[^.]*?(?:\.|$)")
def _player_or_pw_controller(m):
    return UnknownEffect(raw_text="that player or that planeswalker controller")


# --- "up to one other target creature <gets|blocks|gains> ..." ------------
# Cluster (3). Rally Maneuver / Monstrous Step / Mabel's Mettle.
@_er(r"^up to one other target creature "
     r"(?:gets|blocks|gains|has|deals) [^.]+?(?:\.|$)")
def _up_to_one_other_target_creature(m):
    return UnknownEffect(
        raw_text="up to one other target creature does X")


# --- "you may exile a nonland card from it [until ...]" -------------------
# Cluster (3). Invasion of Gobakhan / Elite Spellbinder / Deep-Cavern Bat
# peek-and-exile tail.
@_er(r"^you may exile a nonland card from it"
     r"(?:\s+until [^.]+?)?(?:\.|$)")
def _may_exile_nonland_from_it(m):
    return UnknownEffect(raw_text="you may exile a nonland card from it")


# --- "you may put a creature card from it onto the battlefield ..." -------
# Cluster (3). Zara, Renegade Recruiter / Anzrag's Rampage / Treacherous
# Urge.
@_er(r"^you may put a creature card "
     r"(?:from it|exiled this way|from your graveyard) "
     r"onto the battlefield(?:[^.]+?)?(?:\.|$)")
def _may_put_creature_card_to_bf(m):
    return UnknownEffect(
        raw_text="you may put a creature card onto the battlefield")


# --- "create a token that's a copy of <X> ..." ----------------------------
# Cluster (3). Benthic Anomaly / Here Comes a New Hero! / Who's That
# Praetor? token-copy tail.
@_er(r"^create a token that'?s a copy of "
     r"(?:one of those creatures|the chosen card|up to one target [^,.]+?|"
     r"that creature|that permanent|target [^,.]+?)"
     r"(?:[,.][^.]+?)?(?:\.|$)")
def _create_token_copy_of(m):
    return UnknownEffect(raw_text="create a token that's a copy of X")


# --- "you may put any number of <typelist> cards ... onto the
#      battlefield" -----------------------------------------------------
# Cluster (4). Saheeli's Directive / Knickknack Ouphe / Blex / Search
# for Blex mass-enter tail.
@_er(r"^you may put any number of "
     r"(?:them|[a-z]+(?:\s+and/or\s+[a-z]+)?(?:\s+cards)?)"
     r"(?:\s+with [^.]+?)?"
     r"(?:\s+from among them)?"
     r"\s+(?:onto the battlefield|into your hand|into your graveyard)"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_put_any_number(m):
    return UnknownEffect(raw_text="you may put any number of X")


# --- "shuffle the chosen cards into your library and ..." -----------------
# Cluster (2). Ecological Appreciation / Threats Undetected.
@_er(r"^shuffle the chosen cards into your library and "
     r"put the rest (?:onto the battlefield|into your hand|into your "
     r"graveyard)(?:[^.]+?)?(?:\.|$)")
def _shuffle_chosen_into_lib(m):
    return UnknownEffect(raw_text="shuffle the chosen cards into your library")


# --- "target player reveals their hand and <tail>" ------------------------
# Cluster (2). Psychotic Episode / Addle.
@_er(r"^target player reveals their hand and "
     r"(?:the top card of their library|"
     r"you choose [^.]+?)(?:\.|$)")
def _target_player_reveals_and(m):
    return UnknownEffect(raw_text="target player reveals their hand and X")


# --- "you choose a card revealed this way" --------------------------------
# Cluster (2). Psychotic Episode / Mind Spike post-reveal pick.
@_er(r"^you choose a card revealed this way(?:\.|$)")
def _you_choose_card_revealed(m):
    return UnknownEffect(raw_text="you choose a card revealed this way")


# --- "each player searches their library for ..." -------------------------
# Cluster (2). Collective Voyage / Field of Ruin.
@_er(r"^each player searches their library for [^.]+?(?:\.|$)")
def _each_player_searches(m):
    return UnknownEffect(raw_text="each player searches their library")


# --- "put up to two creature cards <with ...> from among them onto the
#      battlefield" -----------------------------------------------------
# Cluster (2). Collected Company / Michelangelo's Technique.
@_er(r"^put up to (one|two|three|four|five|\d+) "
     r"([a-z]+(?:\s+and/or\s+[a-z]+)?)\s+cards "
     r"(?:with [^,]+? )?"
     r"from among them onto the battlefield"
     r"(?:[^.]+?)?(?:\.|$)")
def _put_up_to_n_typed_cards(m):
    return UnknownEffect(
        raw_text=f"put up to {m.group(1)} {m.group(2).strip()} cards onto bf")


# --- "creatures of the chosen type have <kw>" — already routed via
#      static; we DO NOT re-add as effect.

# --- "until end of turn, that creature gets/gains ..." --------------------
# Cluster (2). Aurelia, Exemplar of Justice / Malakir Rebirth post-effect
# rider on a separate sentence.
@_er(r"^until end of turn,?\s+that creature "
     r"(?:gets|gains|has|can'?t|gets and gains) [^.]+?(?:\.|$)")
def _until_eot_that_creature(m):
    return UnknownEffect(raw_text="until end of turn, that creature does X")


# --- "until end of turn, up to <N> target creature[s] gets/has ..." -------
# Cluster (2). Gold Rush / Phantasmal Form.
@_er(r"^until end of turn,?\s+up to (one|two|three|four|\d+) "
     r"target creatures? (?:gets|gains|has|each have|each get) [^.]+?(?:\.|$)")
def _until_eot_up_to_n(m):
    return UnknownEffect(raw_text="until end of turn, up to N target creatures")


# --- "it deals N damage instead if <cond>" -------------------------------
# Cluster (2). Invasive Maneuvers / Lithomantic Barrage conditional damage.
@_er(r"^it deals (one|two|three|four|five|six|seven|eight|nine|ten|\d+) "
     r"damage instead if [^.]+?(?:\.|$)")
def _it_deals_n_instead_if(m):
    return UnknownEffect(
        raw_text=f"it deals {m.group(1)} damage instead if cond")


# --- "you may cast an instant or sorcery spell ... without paying ..." ----
# Cluster (2). Muse Vortex / Talent of the Telepath top-of-pile cast.
@_er(r"^you may cast an instant or sorcery spell"
     r"(?:\s+with mana value [^,]+?)?"
     r"\s+from among them without paying its mana cost"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_cast_is_from_among(m):
    return UnknownEffect(
        raw_text="you may cast an instant or sorcery from among them wpmc")


# --- "you may cast a spell from among ..." -------------------------------
# Cluster (2). Portent of Calamity / Nathan Drake.
@_er(r"^you may cast a spell from among "
     r"(?:those cards|the exiled cards|them)"
     r"(?:\s+without paying its mana cost)?"
     r"(?:[^.]+?)?(?:\.|$)")
def _may_cast_spell_from_among(m):
    return UnknownEffect(raw_text="you may cast a spell from among X")


# --- "you may shuffle up to four target cards from your graveyard into
#      your library" ---------------------------------------------------
# Cluster (2). Cathartic Parting / Devious Cover-Up.
@_er(r"^you may shuffle up to (one|two|three|four|five|six|seven|"
     r"eight|nine|ten|\d+) target cards from your graveyard "
     r"into your library(?:\.|$)")
def _may_shuffle_up_to_n(m):
    return UnknownEffect(
        raw_text=f"you may shuffle up to {m.group(1)} cards into library")


# --- "each player may put a land card from their hand onto the
#      battlefield ..." ------------------------------------------------
# Cluster (2). Kynaios and Tiro / Anax-Cymede.
@_er(r"^each player may put a land card from their hand onto the "
     r"battlefield(?:[,.][^.]+?)?(?:\.|$)")
def _each_player_may_put_land(m):
    return UnknownEffect(raw_text="each player may put a land card from hand")


# --- "return that creature to its owner's hand at end of combat" ----------
# Cluster (2). Scrappy Bruiser / Arthur, Marigold Knight.
@_er(r"^return that creature to its owner'?s? hand "
     r"at end of (?:combat|turn)(?:\.|$)")
def _return_that_creature_eoc(m):
    return UnknownEffect(
        raw_text="return that creature to owner's hand at end of combat")


# --- "return it to the battlefield tapped [under its owner's control]" ----
# Cluster (2). Othelm / Nezahal — post-flicker resolve.
@_er(r"^return it to the battlefield tapped"
     r"(?:\s+under its owner'?s? control)?(?:\.|$)")
def _return_it_to_bf_tapped(m):
    return UnknownEffect(raw_text="return it to the battlefield tapped")


# --- "return the exiled card to the battlefield under its owner's
#      control" --------------------------------------------------------
# Cluster (2). Voyager Staff / Identity Thief.
@_er(r"^return the exiled card to the battlefield "
     r"(?:under its owner'?s? control|tapped|"
     r"under your control)(?:\.|$)")
def _return_exiled_to_bf(m):
    return UnknownEffect(raw_text="return the exiled card to the battlefield")


# --- "draw a card if an opponent has <pred>" ------------------------------
# Cluster (2). Veil of Summer / Beza, the Bounding Spring.
@_er(r"^draw a card if an opponent has [^.]+?(?:\.|$)")
def _draw_card_if_opp_has(m):
    return UnknownEffect(raw_text="draw a card if an opponent has X")


# --- "you may attach this equipment to it" --------------------------------
# Cluster (2). Cori-Steel Cutter / A-Cori-Steel Cutter free-equip.
@_er(r"^you may attach this equipment to it(?:\.|$)")
def _may_attach_equipment_to_it(m):
    return UnknownEffect(raw_text="you may attach this equipment to it")


# --- "you may have that player shuffle" ----------------------------------
# Cluster (2). Natural Selection / Portent post-look shuffle option.
@_er(r"^you may have that player shuffle(?:\.|$)")
def _may_have_player_shuffle(m):
    return UnknownEffect(raw_text="you may have that player shuffle")


# --- "exile the token at end of combat|turn" ------------------------------
# Cluster (2). Gyrus, Waker of Corpses / Flamerush Rider.
@_er(r"^exile the token at end of (combat|turn)(?:\.|$)")
def _exile_token_at_end_of(m):
    return UnknownEffect(
        raw_text=f"exile the token at end of {m.group(1)}")


# --- "the token enters with half that many <noun> counters on it,
#      rounded down|up" ------------------------------------------------
# Cluster (2). Ochre Jelly / A-Ochre Jelly.
@_er(r"^the token enters with half that many "
     r"([a-z+/0-9-]+)\s+counters on it,?\s+rounded (down|up)(?:\.|$)")
def _token_enters_half_counters(m):
    return UnknownEffect(
        raw_text=f"token enters with half that many {m.group(1)} counters")


# --- "this creature gains landwalk of the chosen type until end of turn"
# Cluster (2). Giant Slug / Illusionary Presence.
@_er(r"^this creature gains landwalk of the chosen type "
     r"until (?:end of turn|the end of that turn)(?:\.|$)")
def _gains_landwalk_chosen(m):
    return UnknownEffect(
        raw_text="this creature gains landwalk of chosen type eot")


# --- "you may put a land card milled this way into your hand|on top of
#      your library" --------------------------------------------------
# Cluster (2). Sparring Dummy / Glowspore Shaman.
@_er(r"^you may put a land card "
     r"(?:milled this way|from your graveyard) "
     r"(?:into your hand|on top of your library|on the bottom of your "
     r"library)(?:\.|$)")
def _may_put_land_milled(m):
    return UnknownEffect(raw_text="you may put a land card milled this way")


# --- "exile one of those cards and put the rest <tail>" -------------------
# Cluster (2). Sealed Fate / Florian, Voldaren Scion.
@_er(r"^exile one of those cards and put the rest "
     r"(?:back )?on (?:top of|the bottom of) "
     r"(?:that player'?s?|your) library "
     r"in (?:any|a random) order(?:\.|$)")
def _exile_one_put_rest(m):
    return UnknownEffect(
        raw_text="exile one of those cards and put the rest")


# --- "untap this artifact during each other player's untap step" ---------
# Cluster (2). Bender's Waterskin / Victory Chimes.
@_er(r"^untap this artifact during each other player'?s? "
     r"untap step(?:\.|$)")
def _untap_artifact_each_other_player(m):
    return UnknownEffect(
        raw_text="untap this artifact during each other player's untap step")


# --- "search your library and graveyard for ..." -------------------------
# Cluster (2). Nissa's Encouragement / Verdant Crescendo.
@_er(r"^search your library and graveyard for [^.]+?(?:\.|$)")
def _search_library_and_gy(m):
    return UnknownEffect(raw_text="search your library and graveyard for X")


# --- "you may exile one of those cards [...]" ----------------------------
# Cluster (2). Nahiri's Warcrafting / Psychic Surgery.
@_er(r"^you may exile one of those cards"
     r"(?:[,.][^.]+?)?(?:\.|$)")
def _may_exile_one_of_those(m):
    return UnknownEffect(raw_text="you may exile one of those cards")


# --- "you may look at it for as long as it remains exiled" / "those
#      cards" -----------------------------------------------------------
# Cluster (2+2). Gustha's Scepter / Rogue Class / Jester's Scepter /
# Three Wishes.
@_er(r"^you may look at "
     r"(?:it|those cards|the exiled cards) "
     r"for as long as (?:it|they) remains? exiled(?:\.|$)")
def _may_look_at_exiled(m):
    return UnknownEffect(raw_text="you may look at exiled cards")


# --- "any opponent may have you put that card into your <hand|gy>" --------
# Cluster (2). Distant Memories / Sin Prodder.
@_er(r"^any opponent may have you put that card into your "
     r"(hand|graveyard|library)(?:\.|$)")
def _any_opp_may_have_you_put(m):
    return UnknownEffect(
        raw_text=f"any opponent may have you put card into {m.group(1)}")


# --- "put all <X> cards revealed this way into your hand and the rest
#      ..." ------------------------------------------------------------
# Cluster (2+2). Goblin Ringleader / Kavu Howler / Alrund / Plargg Dean.
@_er(r"^put all (?:cards of the chosen type|~ cards|revealed cards "
     r"not cast this way|[a-z]+\s+(?:and|or)\s+[a-z]+\s+cards) "
     r"(?:revealed this way )?(?:into your hand|on the bottom of your "
     r"library)(?:[^.]+?)?(?:\.|$)")
def _put_all_revealed(m):
    return UnknownEffect(raw_text="put all cards revealed this way")


# --- "put the rest into your graveyard|on top of your library" ---------
# (already in scrubber #8 with hand variant — scrubber #8 covers
# hand|graveyard|library — skipped)


# --- "put up to two land cards from among them onto the battlefield
#      tapped" --------------------------------------------------------
# Cluster (2). Expand the Sphere / Cartographer's Survey. Specific case
# of "put up to N land cards" with tapped rider. Caught by the typed-cards
# rule above; this regex extends to handle "tapped" tail.
@_er(r"^put up to (one|two|three|four|\d+) land cards "
     r"from among them onto the battlefield tapped"
     r"(?:[^.]+?)?(?:\.|$)")
def _put_up_to_n_lands_tapped(m):
    return UnknownEffect(
        raw_text=f"put up to {m.group(1)} land cards from among them tapped")


# --- "put one of those cards into your hand and the rest into your
#      <gy|library>" -----------------------------------------------------
# Cluster overlap with "put one of those cards into" above; extra catcher
# specifically for the "rest" tail. Already covered by _put_one_of_those.

# --- "that creature can't be blocked this <turn|combat> [...]" -----------
# Cluster (2). Atomic Microsizer / Raging River.
@_er(r"^that creature can'?t be blocked this (turn|combat)"
     r"(?:[^.]+?)?(?:\.|$)")
def _that_creature_unblockable(m):
    return UnknownEffect(
        raw_text=f"that creature can't be blocked this {m.group(1)}")


# --- "this creature deals damage to that <X> equal to ..." --------------
# Cluster (2). Viashino Heretic / Hellhole Rats variable damage.
@_er(r"^this creature deals damage to that "
     r"(?:player|creature|artifact|permanent|planeswalker|"
     r"artifact'?s? controller)\s+equal to [^.]+?(?:\.|$)")
def _this_creature_damage_equal_to(m):
    return UnknownEffect(
        raw_text="this creature deals damage to that X equal to Y")


# --- "a creature dealt damage this way can't <block|be regenerated> ..." -
# Cluster (3). Ballista Watcher / Jaya Ballard / Incinerate.
@_er(r"^a creature dealt damage this way can'?t "
     r"(?:block this turn|be regenerated this turn|be blocked this turn|"
     r"have damage healed)(?:\.|$)")
def _creature_dealt_dmg_cant(m):
    return UnknownEffect(raw_text="a creature dealt damage this way can't X")


# --- "target creature you don't control gets -N/-0 ..." -----------------
# Cluster (2). Downsize / Chemister's Trick.
@_er(r"^target creature you don'?t control gets "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x) until end of turn"
     r"(?:[^.]+?)?(?:\.|$)")
def _target_creature_you_dont_control_gets(m):
    return UnknownEffect(
        raw_text=f"target creature you don't control gets {m.group(1)}/{m.group(2)}")


# --- "return up to x cards from <X> to <Y>, where x is ..." -------------
# Cluster (2). Bag of Devouring / Bound // Determined.
@_er(r"^return up to x cards from "
     r"(?:among cards exiled with this artifact|your graveyard) "
     r"to (?:their owners'? hands|your hand),?\s+"
     r"where x is [^.]+?(?:\.|$)")
def _return_up_to_x_cards(m):
    return UnknownEffect(raw_text="return up to x cards where x is Y")


# --- "until end of turn, that creature gains ..." — caught above

# --- "it deals damage equal to its power to target ..." ------------------
# Cluster (2). Archdruid's Charm / Brokers Charm modal-fight tail.
@_er(r"^it deals damage equal to its power to "
     r"target [^.]+?(?:\.|$)")
def _it_deals_damage_eq_power(m):
    return UnknownEffect(raw_text="it deals damage equal to its power to X")


# --- "it gets -5/-5 instead as long as <cond>" --------------------------
# Cluster (2). Precipitous Drop / A-Precipitous Drop.
@_er(r"^it gets ([+-]\d+)/([+-]\d+) instead as long as [^.]+?(?:\.|$)")
def _it_gets_n_n_instead_aslongas(m):
    return UnknownEffect(
        raw_text=f"it gets {m.group(1)}/{m.group(2)} instead aslongas X")


# --- "target opponent mills half their library, rounded up|down" --------
# Cluster (2). Kitsune's Technique / Jidoor — extends scrubber #8 which
# requires the trailing comma+rounded immediately after; this catches the
# bare orphan when split.
@_er(r"^target opponent mills half their library,?\s+"
     r"rounded (up|down)(?:\.|$)")
def _opp_mills_half_rounded(m):
    return UnknownEffect(
        raw_text=f"target opponent mills half library rounded {m.group(1)}")


# --- "you create a number of treasure tokens equal to <X>" --------------
# Cluster (2). Ancient Copper Dragon / Emissary Green.
@_er(r"^you create a number of "
     r"(treasure|food|clue|blood|gold|powerstone|map|incubator) tokens "
     r"equal to [^.]+?(?:\.|$)")
def _you_create_n_tokens_equal_to(m):
    return UnknownEffect(
        raw_text=f"you create a number of {m.group(1)} tokens equal to X")


# --- "each player loses life equal to <X>" -----------------------------
# Cluster (2). Goblin Game / Deadly Tempest.
@_er(r"^each player loses life equal to [^.]+?(?:\.|$)")
def _each_player_loses_life_eq(m):
    return UnknownEffect(raw_text="each player loses life equal to X")


# --- "surveil 3, then return a creature card from your graveyard to
#      the battlefield ..." -------------------------------------------
# Cluster (2). Charnel Serenade / Connive // Concoct.
@_er(r"^surveil (\d+|x),?\s+then return a "
     r"(creature|land|artifact|enchantment|planeswalker) card from your "
     r"graveyard to the battlefield"
     r"(?:[^.]+?)?(?:\.|$)")
def _surveil_then_recur(m):
    return UnknownEffect(
        raw_text=f"surveil {m.group(1)} then recur {m.group(2)}")


# --- "this creature and enchanted creature each get +X/+X ..." ----------
# Cluster (2). Eidolon of Countless Battles / Nighthowler.
@_er(r"^this creature and enchanted creature each get "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"(?:\s+for each [^.]+?)?(?:[^.]+?)?(?:\.|$)")
def _self_and_enchanted_each_get(m):
    return UnknownEffect(
        raw_text=f"self and enchanted each get {m.group(1)}/{m.group(2)}")


# --- "the duplicate perpetually gets +1/+0 and gains haste" -------------
# Cluster (2). Goblin Morale Sergeant / Viconia.
@_er(r"^the duplicate perpetually gets ([+-]\d+)/([+-]\d+) "
     r"and gains [^.]+?(?:\.|$)")
def _duplicate_perpetually_gets(m):
    return UnknownEffect(
        raw_text=f"duplicate perpetually gets {m.group(1)}/{m.group(2)}")


# --- "creatures attacking the last chosen player <verb>" ----------------
# Cluster (2). Triarch Stalker / Beckoning Will-o'-Wisp.
@_er(r"^creatures attacking the last chosen player "
     r"(?:have|gain|get) [^.]+?(?:\.|$)")
def _creatures_attacking_chosen(m):
    return UnknownEffect(raw_text="creatures attacking the last chosen player")


# --- "creatures controlled by players who chose <X> get ±N/±N" ----------
# Cluster (2). Archangel of Strife.
@_er(r"^creatures controlled by players who chose "
     r"(war|peace|[a-z]+) get ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _creatures_by_players_who_chose(m):
    return UnknownEffect(
        raw_text=f"creatures controlled by players who chose {m.group(1)}")


# --- "each of them is a 1/1 spirit ..." --------------------------------
# Cluster (2). Ghost Vacuum / Storm of Souls.
@_er(r"^each of them is an? (\d+)/(\d+) [a-z ]+?"
     r"(?:\s+with [a-z, ]+)?"
     r"\s+in addition to its other types(?:\.|$)")
def _each_of_them_is_n_n(m):
    return UnknownEffect(
        raw_text=f"each of them is a {m.group(1)}/{m.group(2)}")


# --- "you may put that card onto the battlefield ..." -------------------
# Cluster (2). Hei Bai / Genesis Storm.
@_er(r"^you may put that card onto the battlefield"
     r"(?:[,.][^.]+?)?(?:\.|$)")
def _may_put_that_card_to_bf(m):
    return UnknownEffect(raw_text="you may put that card onto the battlefield")


# --- "10-19 | each opponent sacrifices a permanent of their choice" ------
# Cluster (2). Earth-Cult Elemental / Myrkul's Edict roll-table row.
@_er(r"^\d+(?:[-–—]\d+)?\s*\|\s*[^.]+?(?:\.|$)")
def _roll_table_row(m):
    return UnknownEffect(raw_text="roll-table row")


# --- "you choose a card name other than a basic land card name" ---------
# Cluster (2). Desperate Research / Necromentia.
@_er(r"^choose a card name other than (?:a basic land card name|"
     r"[a-z ]+?)(?:\.|$)")
def _choose_card_name_other(m):
    return UnknownEffect(raw_text="choose a card name other than X")


# --- "put any number of creature and/or land cards from among them ..." -
# Cluster (2). Torsten / Rip, Spawn Hunter. Variant of any-number with
# "and/or" types and explicit destination "into your hand".
@_er(r"^put any number of "
     r"[a-z]+\s+and/or\s+[a-z]+\s+cards"
     r"(?:\s+with [^,]+?)?"
     r"\s+from among them into your hand"
     r"(?:[^.]+?)?(?:\.|$)")
def _put_any_number_andor_cards(m):
    return UnknownEffect(raw_text="put any number of A and/or B cards")


# --- "you may put an artifact, creature, [enchantment, land, planeswalker]
#      card from <X> into your hand" ---------------------------------
# Cluster (2). Dredger's Insight / Vessel of Nascency.
@_er(r"^you may put an? "
     r"[a-z]+(?:,\s*[a-z]+)*(?:,?\s+or\s+[a-z]+)?\s+card from "
     r"(?:among the milled cards|among them|your graveyard) "
     r"into your hand(?:\.|$)")
def _may_put_typelist_card(m):
    return UnknownEffect(raw_text="you may put a typelist card from X")


# --- "as many times as you choose, you may <verb>" ----------------------
# Cluster (2). Lim-Dûl's Vault / Dance with Calamity.
@_er(r"^as many times as you choose,?\s+you may [^.]+?(?:\.|$)")
def _as_many_times_as_you_choose(m):
    return UnknownEffect(raw_text="as many times as you choose, you may X")


# --- "put all ~ cards revealed this way into your hand and the rest ..." --
# Already covered by _put_all_revealed above.


# --- "you may reveal the first card you draw each turn as you draw it" ---
# Cluster (2). God-Eternal Kefnet / Inquisitor Eisenhorn.
@_er(r"^you may reveal the first card you draw each turn "
     r"as you draw it(?:\.|$)")
def _reveal_first_card_each_turn(m):
    return UnknownEffect(raw_text="you may reveal the first card each turn")


# --- "you may choose a nonland card from it[. tail]" --------------------
# Cluster (2). Cracked Skull / Binding Negotiation.
@_er(r"^you may choose a nonland card from it"
     r"(?:[,.][^.]+?)?(?:\.|$)")
def _may_choose_nonland_from_it(m):
    return UnknownEffect(raw_text="you may choose a nonland card from it")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [

    # --- "whenever you sacrifice one or more <noun>" ---------------------
    # Cluster (3). Blood Hypnotist / Forge Boss / A-Forge Boss.
    (re.compile(
        r"^whenever you sacrifice one or more "
        r"(?:other\s+)?[a-z ]+",
        re.I),
     "you_sacrifice_one_or_more", "self"),

    # --- "when enchanted creature leaves the battlefield" ----------------
    # Cluster (3). Traveling Plague / Funeral March / Valor of the Worthy.
    (re.compile(
        r"^when enchanted (?:creature|permanent|artifact|land) "
        r"leaves the battlefield",
        re.I),
     "enchanted_ltb", "self"),

    # --- "when that mana is spent to <verb>" -----------------------------
    # Cluster (3). Primal Amulet // Wellspring / Path of Ancestry /
    # Pyromancer's Goggles delayed-on-spend.
    (re.compile(
        r"^when that mana is spent to "
        r"(?:cast|activate|pay)",
        re.I),
     "that_mana_spent_to", "self"),

    # --- "whenever you create one or more (creature) tokens [for the
    #      first time each turn]" ---------------------------------------
    # Cluster (2). Staff of the Storyteller / Akim, the Soaring Wind.
    (re.compile(
        r"^whenever you create one or more "
        r"(?:[a-z]+\s+)?(?:tokens?|creature tokens?)"
        r"(?:\s+for the first time each turn)?",
        re.I),
     "you_create_one_or_more_tokens", "self"),

    # --- "whenever a modified creature you control <verb>" ---------------
    # Cluster (2). Costume Closet / Guardian of the Forgotten.
    (re.compile(
        r"^whenever a modified creature you control "
        r"(?:dies|leaves the battlefield|enters|attacks|"
        r"deals combat damage|is dealt)",
        re.I),
     "modified_creature_event", "self"),

    # --- "whenever you discard a noncreature, nonland card" --------------
    # Cluster (2). Surly Badgersaur / Bone Miser.
    (re.compile(
        r"^whenever you discard a "
        r"(?:noncreature(?:,\s*nonland)?|nonland(?:,\s*noncreature)?|"
        r"[a-z]+(?:,\s*[a-z]+)*)\s+card",
        re.I),
     "you_discard_typed_card", "self"),

    # --- "whenever a face-down creature you control <verb>" --------------
    # Cluster (2). Cryptic Pursuit / Yarus, Roar of the Old Gods.
    (re.compile(
        r"^whenever a face-down creature you control "
        r"(?:dies|attacks|enters|leaves the battlefield|is turned)",
        re.I),
     "face_down_creature_event", "self"),

    # --- "whenever another permanent you control leaves the battlefield" -
    # Cluster (2). Angelic Sleuth / Suki, Courageous Rescuer.
    (re.compile(
        r"^whenever another permanent you control "
        r"leaves the battlefield",
        re.I),
     "another_perm_ltb", "self"),

    # --- "whenever an <tribe> you control dies" --------------------------
    # Cluster (2). Elderfang Venom / Bishop of Wings / Rampage of the
    # Valkyries — angel/elf tribal LTB.
    (re.compile(
        r"^whenever an? [a-z]+ you control dies",
        re.I),
     "tribe_you_control_dies", "self"),

    # --- "whenever a player wins a coin flip" ----------------------------
    # Cluster (2). Zndrsplt / Okaun.
    (re.compile(
        r"^whenever a player wins a coin flip",
        re.I),
     "player_wins_coin_flip", "all"),

    # --- "whenever ~ and/or one or more other <type> ... <verb>" ---------
    # Cluster (2). Anje, Maid of Dishonor / Satoru, the Infiltrator.
    (re.compile(
        r"^whenever ~ and/or one or more "
        r"(?:other\s+)?[a-z ]+?(?:creatures?|permanents?)"
        r"(?:\s+you control)?"
        r"\s+(?:enter|die|leave|attack)",
        re.I),
     "self_and_or_others_event", "self"),

    # --- "whenever combat damage is dealt to you [or a planeswalker
    #      you control]" -----------------------------------------------
    # Cluster (2). Risona / Vengeful Pharaoh.
    (re.compile(
        r"^whenever combat damage is dealt to you"
        r"(?:\s+or a planeswalker you control)?",
        re.I),
     "combat_damage_to_you", "self"),

    # --- "whenever you reveal an instant or sorcery card this way" -------
    # Cluster (2). God-Eternal Kefnet / Inquisitor Eisenhorn.
    (re.compile(
        r"^whenever you reveal an? "
        r"(?:instant or sorcery|creature|artifact|enchantment|land|"
        r"planeswalker|nonland|noncreature)\s+card "
        r"(?:this way|from your library|from your hand)",
        re.I),
     "you_reveal_typed_card", "self"),

    # --- "whenever an angel|elf|<tribe> you control enters" --------------
    # Cluster general. Singular tribal-ETB shape that doesn't go through
    # the existing "creature you control enters" route.
    (re.compile(
        r"^whenever an? [a-z]+ you control enters",
        re.I),
     "tribe_you_control_etb", "self"),

    # --- "as you draft a card, you may <verb>" ---------------------------
    # Cluster (3). Animus of Predation / Smuggler Captain / Cogwork
    # Grinder. Conspiracy/draft trigger — register as a static was
    # done above; also register as trigger so the parser routes correctly
    # if it lands in trigger pass.
    (re.compile(
        r"^as you draft a card,?\s+you may",
        re.I),
     "as_you_draft_a_card", "self"),

    # --- "the next time one or more <X> enter this turn ..." -------------
    # Cluster (2). Mystic Reflection / Storyweave delayed replacement.
    (re.compile(
        r"^the next time one or more "
        r"(?:creatures or planeswalkers|[a-z ]+?)"
        r"\s+enter this turn",
        re.I),
     "next_time_one_or_more_enter", "self"),

    # --- "when you next cast a spell [this turn|with X|or activate ...]" -
    # Cluster (4). Codie / Magus Lucea Kane / Ride the Avalanche.
    (re.compile(
        r"^when you next cast a spell"
        r"(?:\s+(?:this turn|with [^,]+?|or activate [^,]+?))?",
        re.I),
     "when_you_next_cast_spell", "self"),
]
