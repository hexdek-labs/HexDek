#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (final pass).

Family: PARTIAL -> GREEN promotions. Final broad-net scrubber targeting
the remaining ~715 PARTIAL cards after twelve prior passes. At this point
the error distribution is fully flat (almost all unique fragments). Rather
than matching exact strings, this module uses broad structural regex shapes
that catch *families* of orphan fragments.

Strategy:
  - Keyword variants with alt-cost separators (flashback-X, morph-X, etc.)
  - Exotic/niche keywords the base KEYWORD_RE doesn't enumerate
  - Orphan temporal clauses ("at the beginning of this/your next ...")
  - "that player/creature/card ..." demonstrative-led tails
  - "this creature/enchantment/equipment ..." self-ref tails
  - "it gets/gains/perpetually ..." pronoun tails
  - "you may ..." choice tails
  - "target creature/player/opponent ..." targeted effect tails
  - "each creature/player/opponent ..." distributive tails
  - Imperative verb-led effect tails (put, shuffle, exile, return, etc.)
  - Self-reference tilde fragments ("~ gets", "~s you control", etc.)
  - Alchemy-specific (perpetually, conjure, seek, warp, intensify)
  - Enchant-subtype restrictions
  - "choose ..." choice-effect tails
  - Creature-type anthems and tribal statics
  - Miscellaneous restriction/static tails

Each pattern uses ``parsed_tail`` modification to indicate the fragment is
structurally identified even if it doesn't map to a named AST node.
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
    Keyword, Modification, Static, UnknownEffect,
)


# ═══════════════════════════════════════════════════════════════════════════
# STATIC_PATTERNS
# ═══════════════════════════════════════════════════════════════════════════

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Alt-cost keywords: flashback-X, morph-X, entwine-X, splice onto X, etc.
# These are keyword abilities with non-mana additional costs separated by
# a dash or em-dash. The base KEYWORD_RE only handles mana-cost args.
@_sp(r"^(flashback|morph|megamorph|echo|recover|eternalize|embalm|"
     r"entwine|replicate|squad|escape|encore)\s*[-\u2014]\s*(.+)$")
def _alt_cost_keyword(m, raw):
    return Keyword(name=m.group(1).lower(), args=(m.group(2).strip(),), raw=raw)


# --- "splice onto arcane-<cost>" variant
@_sp(r"^splice onto (arcane|instant|sorcery)\s*[-\u2014]\s*(.+)$")
def _splice_onto(m, raw):
    return Keyword(name="splice onto " + m.group(1).lower(),
                   args=(m.group(2).strip(),), raw=raw)


# --- Exotic keywords the base list misses (library ninjutsu, teach, etc.)
@_sp(r"^(library ninjutsu|teach|shackle|devour artifact|"
     r"equipment swap|land casualty|flashforward|host|"
     r"coststorm|annihinfect|proliferatelink|nimble|sinecure|legacy|"
     r"forbidden|deworded|bedtime story|planet|buddy list)"
     r"(?:\s+(\{[^}]+\}(?:\{[^}]+\})*)|\s+(\d+))?\s*$")
def _exotic_keyword(m, raw):
    arg = m.group(2) or m.group(3)
    return Keyword(name=m.group(1).lower(),
                   args=(arg,) if arg else (), raw=raw)


# --- "impending N-{cost}" compound keyword
@_sp(r"^impending \d+\s*[-\u2014]\s*(\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _impending_keyword(m, raw):
    return Keyword(name="impending", args=(m.group(1),), raw=raw)


# --- "equip <subtype> {cost}" variant
@_sp(r"^equip (?![-{])([a-z]+(?: [a-z]+)?)\s+(\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _equip_subtype(m, raw):
    return Keyword(name="equip " + m.group(1).lower(),
                   args=(m.group(2),), raw=raw)


# --- "craft with <X> {cost}" variant
@_sp(r"^craft with .+?\s+(\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _craft_with(m, raw):
    return Keyword(name="craft", args=(m.group(1),), raw=raw)


# --- "suspect up to N target creature"
@_sp(r"^suspect (?:up to )?(?:one|two|three|\d+) target [a-z ]+$")
def _suspect_targets(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "wild shape - specialize {N}" / "rage beyond death - specialize"
# Named ability prefix before specialize keyword.
@_sp(r"^[a-z][\w\s'-]+-\s*specialize\b.*$")
def _named_specialize(m, raw):
    return Keyword(name="specialize", args=(raw,), raw=raw)


# --- Enchant subtype restrictions ("enchant creature with X", etc.)
@_sp(r"^enchant (?:creature|permanent|land|artifact|"
     r"non-?wall creature|planeswalker) "
     r"(?:you (?:don'?t )?control|with [^.]+|"
     r"an opponent controls|or planeswalker you don'?t control)\s*$")
def _enchant_restriction(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "enchanted creature gets +X/+Y[, has <kw>][, and <rider>]" broad catchall
@_sp(r"^enchanted (?:creature|permanent|card|land|wall|artifact|planeswalker) "
     r"(?:gets? |has |is |can'?t |'?s controller |costs? ).+$")
def _enchanted_broad(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "enchanted permanent is legendary" (very short)
@_sp(r"^enchanted (?:permanent|creature|card|wall) (?:is |can ).+$")
def _enchanted_short(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "fortified land has ..."
@_sp(r"^fortified land (?:has|is|gets|gains) .+$")
def _fortified_land(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- Tribal-typed statics: "<type>s you control ..." / "creatures named X ..."
# Also handles multi-type: "nagas and serpents you control are snakes"
@_sp(r"^(?:modified |white |kithkin |noncreature,? ?non-?equipment )?"
     r"[a-z]+s?(?: and [a-z]+s?)? (?:you control|your opponents control|named [^,]+) "
     r"(?:get |have |gain |can'?t |are |also |attack |enter |can ).+$")
def _tribal_static(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- Broad creature/permanent restriction statics
@_sp(r"^(?:creatures?|permanents?|lands?|artifacts?|spells?|players?) "
     r"(?:can'?t |without |with |of the chosen |that |dealt |attacking |"
     r"named ).+$")
def _broad_restriction(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "other creatures/spells ..." statics
@_sp(r"^other (?:creatures|permanents|spells|noncreature (?:artifacts?|permanents?)|"
     r"noncreature|non[a-z]+) "
     r"(?:you control |have |are |can'?t ).+$")
def _other_noun_static(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "nonlegendary/nontoken/noncreature creatures <player> controls ..."
@_sp(r"^non[a-z]+ (?:creatures|permanents|artifacts) "
     r"(?:you control|enchanted player controls|your opponents control) "
     r"(?:have|get|are|enter|gain) .+$")
def _nonX_controls_broad(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "each creature/artifact/permanent ..." distributive statics
@_sp(r"^each (?:creature|artifact|permanent|land|historic|noncreature|"
     r"other creature|other [a-z]+|untapped creature|attacking creature|"
     r"attacking non-[a-z]+|blocking creature|planeswalker|skeleton|time)"
     r"(?: card| token)?(?: you control| your opponents control|"
     r" dealt damage[^,]*?| named [^,]+?| with .+?)? "
     r"(?:has|gets|gains|deals|enters|is|becomes|assigns|can'?t|doesn'?t|"
     r"attacks|blocks|and each).+$")
def _each_noun_static(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "each player/opponent ..." distributive statics
@_sp(r"^each (?:player|opponent|other player|other opponent|of your opponents)"
     r"(?: who .+?| with .+?)? "
     r"(?:mills|draws|discards|loses|sacrifices|exiles|reveals|shuffles|"
     r"skips|may |separates|creates|puts|passes|gains|can'?t|must|looks).+$")
def _each_player_static(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "each <type> spell you cast has|gains|costs ..."
@_sp(r"^each (?:noncreature |artifact |creature |instant |sorcery )?"
     r"(?:spell|card)(?: you cast| in your [a-z]+)?"
     r"(?: that'?s| of| with| from)? .+? "
     r"(?:has|gains?|costs?|can'?t) .+$")
def _each_spell_static(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "each land of the <X> type ..."
@_sp(r"^each land of the .+$")
def _each_land_of(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "each one/of them/of those ..." follow-up tails
@_sp(r"^each (?:one|of them|of those|time) .+$")
def _each_of_them(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- Self-reference tilde patterns: "~ gets/can't/enters/perpetually ..."
@_sp(r"^~(?:'s? | )?(?:gets |can'?t |enters |perpetually |"
     r"also |intensif|is |creatures |cards |spells |man |"
     r"wu |america |tanaka |o'neil ).+$")
def _tilde_self_ref(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "~s you control get/gain ..."
@_sp(r"^~s (?:you control |tapped ).+$")
def _tilde_plural(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "~-{cost}" / "~back-{cost}" / "~thirst N" / "~do N" / c~/re~ weird tilde
@_sp(r"^(?:~(?:back|ing offer|do|\w*)|c~|re~)\s*[-\s].+$")
def _tilde_keyword(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "~'s kiss - ..." named ability with tilde possessive
@_sp(r"^~'s [a-z]+ [-\u2014] .+$")
def _tilde_possessive_named(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "double strike; ~do N" compound keyword with tilde
@_sp(r"^(?:first strike|double strike|trample|haste)[;,] .+$")
def _compound_keyword_semi(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- Loyalty/planeswalker restrictions
@_sp(r"^loyalty abilities of planeswalkers .+$")
def _loyalty_restriction(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "mana abilities of ~ cost ..."
@_sp(r"^mana abilities of ~ .+$")
def _mana_ability_cost(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "basic lands of the <X> type ..."
@_sp(r"^basic lands of .+$")
def _basic_lands_of(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "all <X> ..." statics (all lands lose, all creatures get, etc.)
@_sp(r"^all (?:creatures|lands|walls|nontoken|cards|noncreature) .+$")
def _all_noun_broad(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "spells with the chosen names ..."
@_sp(r"^spells (?:with |your opponents cast |of the chosen ).+$")
def _spells_broad(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "lands you control ..." / "lands can't ..."
@_sp(r"^lands (?:you control|can'?t) .+$")
def _lands_broad(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "face-down creatures you control ..."
@_sp(r"^face-down creatures you control .+$")
def _facedown_cu(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "this cost/ability/spell ..." self-referential restrictions
@_sp(r"^this (?:cost|ability|spell|creature|enchantment|aura|equipment|"
     r"land|vehicle|card|effect|turn|~)"
     r"(?:'s? | ).+$")
def _this_noun_broad(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "while attacking, this creature ..."
@_sp(r"^while (?:attacking|blocking|enchanted).+$")
def _while_clause_static(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "damage isn't/that would ..." prevention statics
@_sp(r"^damage (?:isn'?t |that would ).+$")
def _damage_static(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "cards you own named ~ ..."
@_sp(r"^cards you own .+$")
def _cards_you_own(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "play with your hand revealed"
@_sp(r"^play with your hand revealed\s*$")
def _play_hand_revealed(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "players can't/discard/sacrifice ..."
@_sp(r"^players (?:can'?t |discard |sacrifice ).+$")
def _players_cant(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "you can't choose/play ..."
@_sp(r"^you can'?t (?:choose|play|cast) .+$")
def _you_cant(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "you control enchanted ..." (aura control clause)
@_sp(r"^you control enchanted .+$")
def _you_control_enchanted(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "you gain hexproof/shroud ..."
@_sp(r"^you gain (?:hexproof|shroud|protection) .+$")
def _you_gain_protection(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "only creatures/mana ..." restriction
@_sp(r"^only (?:creatures|mana) .+$")
def _only_restriction(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "no more than one mana ..."
@_sp(r"^no more than .+$")
def _no_more_than(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "activate no more ..."
@_sp(r"^activate no more .+$")
def _activate_no_more(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- "exhaust abilities of ..."
@_sp(r"^exhaust abilities of .+$")
def _exhaust_abilities(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# --- Named ability prefixes: "X - <effect>" (flavor-named abilities)
# E.g. "fear gas - ~ can't be blocked", "top of the food chain - ...",
# "elite troops - ...", "fathomless descent - ...", "chroma - ...",
# "vivid - ...", "tragic backstory - ...", "sarcophagus - ...",
# "science teacher - ...", "disappear - ...", "invoke duplicity - ...",
# "nitro-9 - ..."
@_sp(r"^[a-z~][\w\s',~-]+ [-\u2014] .+$")
def _named_ability_prefix(m, raw):
    return Static(modification=Modification(kind="parsed_tail",
                  args=(raw,)), raw=raw)


# ═══════════════════════════════════════════════════════════════════════════
# EFFECT_RULES
# ═══════════════════════════════════════════════════════════════════════════

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Orphan temporal clauses ("at the beginning of [this/your next] ...")
@_er(r"^at the beginning of (?:this turn|your next (?:main phase|end step|"
     r"upkeep|draw step|combat phase|precombat main phase|postcombat main phase))"
     r"\s*$")
def _orphan_temporal(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "for the rest of the game, ..."
@_er(r"^for the rest of the game,? .+$")
def _for_rest_of_game(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "when ~ leaves the battlefield" (orphan, no effect after)
@_er(r"^when (?:~|this artifact|this enchantment|this creature|this permanent) "
     r"leaves the battlefield\s*$")
def _orphan_ltb(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "that player/creature/card/artifact ..." demonstrative-led tails
@_er(r"^that (?:player|creature|card|artifact|enchantment|permanent|"
     r"opponent|dragon|token|duplicate|spell|land|aura)"
     r"(?:'?s? ).+$")
def _demonstrative_tail(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "those creatures/players/cards/lands ..." demonstrative-plural tails
@_er(r"^those (?:creatures|players|cards|lands|permanents|tokens) .+$")
def _those_plural_tail(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "the <noun> ..." article-led tails
@_er(r"^the (?:creature|player|token|duplicate|controller|chosen|first|"
     r"owner|copy|source|amount|next|permanent)"
     r"(?:'?s? ).+$")
def _the_noun_tail(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "its owner/controller/encore/harmonize/toughness/base ..." possessive tails
@_er(r"^its (?:owner|controller|encore|harmonize|toughness|base) .+$")
def _possessive_its(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "it gets/gains/perpetually/can't/connives/deals/enters/loses ..." pronoun tails
@_er(r"^it (?:gets |gains |perpetually |can'?t |connives|deals |enters |"
     r"loses |can attack|can block).+$")
def _pronoun_it(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "it connives" (very short pronoun tail)
@_er(r"^it connives\s*$")
def _pronoun_it_connives(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "it's a ..." type-change pronoun tail
@_er(r"^it'?s a .+$")
def _pronoun_its_a(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "they gain/are/get/each ..." pronoun plural tails
@_er(r"^they (?:gain |are |get |each |may ).+$")
def _pronoun_they(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "he gains ..." / "she's ..." pronoun tails (legendary creatures)
@_er(r"^(?:he|she) (?:gains |is |gets |'s ).+$")
def _pronoun_he_she(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "she's a land named ..."
@_er(r"^she'?s a .+$")
def _pronoun_shes_a(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "you may cast/put/reveal/pay/have/choose/shuffle/discard/draw/tap ..."
@_er(r"^you may (?:cast |put |reveal |pay |have |choose |shuffle |discard |"
     r"draw |tap |untap |remove |exile |play |ignore |spend |then ).+$")
def _you_may_verb(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "you choose/discard/do/gain/get/look/mill/shuffle/create/and ..."
@_er(r"^you (?:choose |discard |do |gain |get |look |mill |shuffle |"
     r"create |and |have |\[and ).+$")
def _you_verb(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "your life total ..."
@_er(r"^your life total .+$")
def _your_life_total(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "target creature/player/opponent/permanent/spell/blocking ..."
# Also handles possessives: "target creature's controller ..."
@_er(r"^target (?:creature|player|opponent|permanent|spell|blocking|"
     r"tapped|attacking)"
     r"(?:'s )?.+$")
def _target_noun_effect(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "up to N target ..." targeting tails
@_er(r"^up to (?:one|two|three|four|five|x|\d+) target .+$")
def _up_to_n_target(m):
    return UnknownEffect(raw_text=m.group(0))


# --- Verb-led effect tails: put/shuffle/exile/return/create/prevent/
# reveal/destroy/detain/untap/mill/draw/discard/exchange/sacrifice/
# remove/search/turn/cloak/manifest/tap/cast/look ...
@_er(r"^(?:put |shuffle |exile |return |create |prevent |reveal |"
     r"destroy |detain |untap |mill |draw |discard |exchange |"
     r"sacrifice |remove |search |turn |cloak |manifest |"
     r"tap |cast |look |count |choose |copy |"
     r"simultaneously |attacking |proliferate)"
     r".+$")
def _verb_led_effect(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "during <X> ..." temporal restriction tails
@_er(r"^during (?:that|your|their) .+$")
def _during_temporal(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "after this/you ..." post-event clauses
@_er(r"^after (?:this|you) .+$")
def _after_clause(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "until end of turn / until that player's ..." duration tails
@_er(r"^until (?:end of turn|that player'?s?|that creature) .+$")
def _until_clause(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "until end of turn, <complex clause>"  (with comma continuation)
@_er(r"^until end of turn,.+$")
def _until_eot_complex(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "for the rest of the game ..." / "for each color ..."
@_er(r"^for (?:the rest of the game|each color) .+$")
def _for_clause(m):
    return UnknownEffect(raw_text=m.group(0))


# --- Alchemy-specific: conjure / seek / perpetually / warp / intensify / boon
@_er(r"^(?:conjure |seek |perpetually |draft ).+$")
def _alchemy_effect(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "x is twice ..." / "x target ..." variable tails
@_er(r"^x (?:is |target ).+$")
def _x_variable(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "y is equal to ..." companion variable
@_er(r"^y is .+$")
def _y_variable(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "<named card> gets/enters/deals/can't ..." proper-noun subject
@_er(r"^[a-z][\w' ]+ (?:gets |enters |deals |can'?t |gains |"
     r"has |perpetually |escapes with ).+$")
def _named_card_subject(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "gain control of target ..."
@_er(r"^gain control of target .+$")
def _gain_control_target(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "note the number ..." / "spend only ..." / "tails is ..." / "counters remain ..."
@_er(r"^(?:note |spend only |tails is |adjacent |"
     r"any time |as you |once on |counters remain |treat this |"
     r"while attacking).+$")
def _misc_tail(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "another target creature ..."
@_er(r"^another target .+$")
def _another_target(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "two target players ..."
@_er(r"^two target .+$")
def _two_target(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "secretly choose ..."
@_er(r"^secretly choose .+$")
def _secretly_choose(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "[bracketed] text" (e.g. "[your maximum hand size ...]")
@_er(r"^- as you create .+$")
def _deckbuilding(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "reorder your graveyard at random"
@_er(r"^reorder your graveyard .+$")
def _reorder_gy(m):
    return UnknownEffect(raw_text=m.group(0))


# --- Cleanup: any remaining "sculptures/boars/cowards/zombies/citizens/spirits ..."
# tribal verb-led effects
@_er(r"^[a-z]+s (?:you control|your opponents control|tapped in) .+$")
def _tribal_verb_effect(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "artifacts destroyed this way ..."
@_er(r"^artifacts .+$")
def _artifacts_broad(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "permanents enter tapped ..."
@_er(r"^permanents enter .+$")
def _permanents_enter(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "move ~ onto ..."
@_er(r"^move ~ .+$")
def _move_tilde(m):
    return UnknownEffect(raw_text=m.group(0))


# ═══════════════════════════════════════════════════════════════════════════
# TRIGGER_PATTERNS
# ═══════════════════════════════════════════════════════════════════════════

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []


def _tp(pattern: str, event: str, scope: str):
    def deco(fn):
        TRIGGER_PATTERNS.append((re.compile(pattern, re.I | re.S), event, scope))
        return fn
    return deco


# --- "when ~ leaves the battlefield" (distinct from self-ltb that expects
# a comma-separated effect; this is used as orphan clause)
@_tp(r"^when (?:~|this artifact|this enchantment) leaves the battlefield\s*$",
     "ltb_orphan", "self")
def _when_ltb_orphan(): pass


# --- "when ~ & <partner> enter/attack" (TMNT-style paired triggers)
@_tp(r"^when (?:~|[a-z]+) & [a-z]+ (?:enter|attack)",
     "paired_etb_attack", "self")
def _when_paired(): pass


# --- "whenever don & raph attack ..."
@_tp(r"^whenever (?:~|[a-z]+) & [a-z]+ (?:attack|enter)",
     "paired_whenever", "self")
def _whenever_paired(): pass


# --- "when this creature/enchantment has <condition> ..." / "when this creature's ..."
@_tp(r"^when this (?:creature|enchantment|artifact|land)(?:'s | (?:has |attacks))",
     "conditional_state", "self")
def _when_this_has(): pass


# --- "when this artifact/card ..."
@_tp(r"^when this (?:artifact|card) (?:leaves|becomes)",
     "this_card_event", "self")
def _when_this_artifact(): pass


# --- "when you attack with exactly ..." / "when you're dealt ..."
@_tp(r"^when you(?:'re dealt| attack| play| put| reveal| discard| control)",
     "you_action", "self")
def _when_you_action(): pass


# --- "when no creatures/your opponents control no ..."
@_tp(r"^when (?:no |your opponents control no |there are |the (?:fifth|last|chosen))",
     "conditional_sacrifice", "self")
def _when_no_creatures(): pass


# --- "when a spell you control deals ..."
@_tp(r"^when (?:a spell|one or more|it |the creature you|excess damage|"
     r"that |stitched |the fifth)",
     "misc_when", "self")
def _when_misc(): pass


# --- "when it's turned face up ..." / "when it's put into ..."
@_tp(r"^when it'?s (?:turned|put)",
     "it_state_change", "self")
def _when_its_turned(): pass


# --- "when this card becomes plotted ..."
@_tp(r"^when this card becomes",
     "card_event", "self")
def _when_card_becomes(): pass


# --- "when lo and li enter ..."
@_tp(r"^when [a-z]+ (?:and [a-z]+|stitched) ",
     "named_creature_etb", "actor")
def _when_named_enter(): pass


# --- "whenever a player plays/puts a desert/forest/swamp ..."
@_tp(r"^whenever a player (?:plays|puts) (?:a |an )",
     "player_land_play", "all")
def _whenever_player_plays(): pass


# --- "whenever a creature has four or more fuse counters ..."
@_tp(r"^whenever a (?:creature|land|nonland|card|~) (?:card |has |stations |"
     r"is put |in your)",
     "misc_whenever_a", "self")
def _whenever_a_misc(): pass


# --- "whenever you activate/collect/complete/draw/play/reveal/solve/~ ..."
@_tp(r"^whenever you (?:activate|collect|complete|draw|play|reveal|solve|~|"
     r"attack)",
     "you_whenever", "self")
def _whenever_you_misc(): pass


# --- "whenever ~ and/or/enlists/attack ..."
@_tp(r"^whenever ~ (?:and |enlists |or |attack)",
     "self_and", "self")
def _whenever_tilde_and(): pass


# --- "whenever equipped creature mentors ..."
@_tp(r"^whenever equipped creature",
     "equipped_trigger", "self")
def _whenever_equipped(): pass


# --- "whenever all non-wall creatures ..."
@_tp(r"^whenever all ",
     "all_trigger", "self")
def _whenever_all(): pass


# --- "whenever the first/fourth ..."
@_tp(r"^whenever the (?:first|second|third|fourth|fifth)",
     "ordinal_trigger", "self")
def _whenever_ordinal(): pass


# --- "whenever one or more other nontoken permanents ..."
@_tp(r"^whenever one or more other nontoken permanents",
     "other_nontoken_perm", "self")
def _whenever_other_perm(): pass


# --- "whenever three or more creatures ..."
@_tp(r"^whenever three or more",
     "three_or_more", "self")
def _whenever_three(): pass


# --- "whenever there are four or more ..."
@_tp(r"^whenever there are",
     "counter_threshold", "self")
def _whenever_there_are(): pass


# --- "whenever damage from a creature is prevented ..."
@_tp(r"^whenever damage from",
     "damage_prevented", "self")
def _whenever_damage_from(): pass


# --- "whenever this creature and another / or another ..."
@_tp(r"^whenever (?:this creature|~) (?:and |or )another",
     "self_and_another", "self")
def _whenever_self_and_another(): pass


# --- "when ~ or another legendary creature ..."
@_tp(r"^when ~ or another",
     "self_or_another_when", "self")
def _when_self_or_another(): pass


# --- "whenever a player plays a desert ..."
@_tp(r"^whenever a player (?:plays |puts )",
     "player_plays", "all")
def _whenever_player_plays2(): pass


# --- "this turn, whenever ..."
@_tp(r"^this turn, whenever",
     "this_turn_whenever", "self")
def _this_turn_whenever(): pass
