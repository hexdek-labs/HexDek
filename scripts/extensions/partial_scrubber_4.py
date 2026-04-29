#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (fourth pass).

Family: PARTIAL → GREEN promotions. Companion to ``partial_scrubber.py``,
``partial_scrubber_2.py``, and ``partial_scrubber_3.py``; targets single-
ability clusters that survived all three prior passes. Patterns were picked
by re-bucketing PARTIAL parse_errors after the third scrubber shipped, then
keeping clusters of >=6 hits that map cleanly onto static regex.

Same three export tables as the prior scrubbers.

- ``STATIC_PATTERNS``  — long-tail static / continuous-effect shapes:
  ``your maximum hand size is <X>`` (12), ``land creatures you control
  have <kw>`` (8), ``each other creature you control [enters|has|that's]
  <rider>`` (11), ``each creature you control with <X> [has|can|...]``
  (10), ``creatures you control that are <Y> get +N/+N`` (8), ``instant
  and sorcery spells you cast/control have/get <X>`` (8), ``other ~
  creatures you control [get|attack|have]`` (6), ``max speed - <rider>``
  (7), ``you may play lands from your graveyard`` (8), ``you may cast
  spells from <zone>`` (9), ``you may play lands and cast <type> spells
  from <zone>`` (14), ``this mana can't be spent <to ...>`` (6),
  ``each mode must target a different player`` (8), ``you may reveal this
  card from your opening hand`` (8), ``after this main phase, there is
  an additional combat phase`` (8), ``you don't lose the game...``
  variants.

- ``EFFECT_RULES``    — body-level shapes still leaking as parse_errors:
  ``you may cast that card <long tail>`` (12), ``you may play those cards
  / play that card <long tail>`` (8+5), ``put that card / put those cards
  <zone>`` (11), ``put target creature card from a graveyard onto the
  battlefield <tail>`` (10), ``draw cards equal to <expr>`` (10), ``gain
  control of target creature [tail]`` (7), ``destroy up to one target
  <X>`` (7), ``exile any number of target creatures [tail]`` (7), ``you
  may tap or untap target <X>`` (7), ``return up to two target <X> to
  hand`` (6), ``any number of target creatures (each get / can't ...)``
  (9), ``draw two cards and create a <token>`` (7), ``put one of them
  into <zone>`` (7), ``put those cards onto the battlefield <tail>``
  (7), ``you may put a card from your hand on the bottom`` (7), ``you
  may discard a card. if you do, draw <N>`` (6), ``you choose one of
  those cards`` (6), ``until your next end step, you may play <X>``
  (8), bare ``~`` (9).

- ``TRIGGER_PATTERNS`` — new trigger shapes the base parser still misses:
  ``whenever another creature you control leaves the battlefield`` (15),
  ``whenever one or more (other )?creatures (you control) (deal|die|enter
  |leave)`` (14+13), ``whenever one or more creature cards are put into /
  leave <zone>`` (10), ``whenever one or more nontoken <type>`` (8),
  ``whenever a creature you control (leaves|exploits)`` (10), ``whenever
  a land an opponent controls enters`` (6), ``whenever a source you
  control deals (noncombat)? damage`` (6), ``whenever the ring tempts
  you`` (7), ``when you next cast a <X> spell`` (7), ``when you
  discard a card this way`` (6), ``whenever another creature or
  artifact you control [dies|to gy]`` (7).

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
    Buff, Destroy, Draw, Filter, GainControl, GrantAbility, Keyword,
    Modification, Sequence, Static, UnknownEffect,
    TARGET_CREATURE,
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


# --- "your maximum hand size is <N|word|expr>" ------------------------------
# Cluster (12). Reliquary Tower / Library of Alexandria-tail / Greed-rider.
# Base parse_static doesn't know hand-size mods at all.
@_sp(r"^your maximum hand size is "
     r"(?:(\d+|one|two|three|four|five|six|seven|eight|nine|ten|no maximum)"
     r"|equal to [^.]+|increased by [^.]+|reduced by [^.]+)\s*$")
def _max_hand_size(m, raw):
    return Static(modification=Modification(kind="max_hand_size"), raw=raw)


# --- "you have no maximum hand size" ---------------------------------------
# Reliquary Tower bare phrasing.
@_sp(r"^you have no maximum hand size\s*$")
def _no_max_hand(m, raw):
    return Static(modification=Modification(kind="no_max_hand_size"), raw=raw)


# --- "land creatures you control have <kw>" --------------------------------
# Cluster (8). Awakening / Living Lands family — animated lands keep the
# "creature" type while remaining lands and pick up evergreen kws.
@_sp(r"^land creatures you control have ([a-z, ]+?)\s*$")
def _land_creatures_have(m, raw):
    return Static(modification=Modification(
        kind="land_creatures_have_kw",
        args=(m.group(1).strip(),)), raw=raw)


# --- "each other creature you control [enters with|has|that's <type> ...]" --
# Cluster (~11 residual after scrubber #3 covered the +N/+N shape). These are
# qualified-other riders that scrubber #3 missed.
@_sp(r"^each other creature you control "
     r"(?:enters with [^.]+|with [^,]+ has [^.]+|named [^,]+ has [^.]+|"
     r"of the chosen type enters with [^.]+)\s*$")
def _each_other_qualified_static(m, raw):
    return Static(modification=Modification(
        kind="each_other_qualified_rider"), raw=raw)


# --- "each creature you control with <X> has|can't|gets ..." ---------------
# Cluster (10). Anthems / restrictions over a sub-population.
@_sp(r"^each creature you control with ([^,]+?) "
     r"(?:has [a-z, ]+?|can'?t [^.]+|gets \+\d+/\+\d+|is [^.]+|"
     r"stations [^.]+)\s*$")
def _each_creature_with_static(m, raw):
    return Static(modification=Modification(
        kind="each_creature_with_rider",
        args=(m.group(1).strip(),)), raw=raw)


# --- "creatures you control that are <Y> get +N/+N [and have <kw>]" --------
# Cluster (8). Tribal-by-type-or-state lord ("creatures you control that are
# enchanted get +1/+1", "...that are zombies and/or tokens get +1/+1 and have
# flying").
@_sp(r"^creatures you control that are ([a-z,/ ]+?) "
     r"get \+(\d+)/\+(\d+)(?: and have ([a-z, ]+?))?\s*$")
def _ally_creatures_thatare(m, raw):
    qualifier = m.group(1).strip()
    p, t = int(m.group(2)), int(m.group(3))
    extra = (m.group(4) or "").strip() or None
    return Static(modification=Modification(
        kind="ally_thatare_anthem",
        args=(qualifier, p, t, extra)), raw=raw)


# --- "instant and sorcery spells you cast|control have|get <X>" ------------
# Cluster (8). Spell-tribal lord (Storm-Kiln Artist / Adeline-style riders).
@_sp(r"^instant and sorcery spells you (cast|control) "
     r"(?:have ([a-z, ]+?)|get \+\d+/\+\d+|gain ([a-z, ]+?))"
     r"(?: from [^.]+)?\s*$")
def _spell_tribal_lord(m, raw):
    kw = (m.group(2) or m.group(3) or "").strip()
    return Static(modification=Modification(
        kind="spell_tribal_lord",
        args=(m.group(1).lower(), kw or None)), raw=raw)


# --- "other ~ creatures you control [get +N/+N | have <kw> | attack ...]" --
# Cluster (6). Tribal-self lord; ~ stands in for the card name. Scrubber #2
# handles the "other <qualifier>" form, but bare-tilde sneaks past.
@_sp(r"^other ~ creatures you control "
     r"(?:get \+(\d+)/\+(\d+)(?: and have ([a-z, ]+?))?|"
     r"have ([a-z, ]+?)|attack [^.]+)\s*$")
def _other_tilde_lord(m, raw):
    return Static(modification=Modification(
        kind="other_tilde_lord_static"), raw=raw)


# --- "max speed - <rider>" — Aetherdrift max-speed ability-word ------------
# Cluster (7). New Aetherdrift mechanic that the scrubber-#2 ability-word
# rider list didn't include.
@_sp(r"^max speed\s*[-–—]\s*(.+?)\s*$")
def _max_speed_rider(m, raw):
    return Static(modification=Modification(
        kind="ability_word_rider",
        args=("max_speed", m.group(1).strip())), raw=raw)


# --- "you may play lands from your graveyard" ------------------------------
# Cluster (8). Crucible of Worlds-class static.
@_sp(r"^you may play lands from (?:your |any )?(graveyards?|exile)\s*$")
def _play_lands_from_zone(m, raw):
    return Static(modification=Modification(
        kind="play_lands_from_zone",
        args=(m.group(1).rstrip("s"),)), raw=raw)


# --- "you may cast spells from <zone> [<rider>]" ---------------------------
# Cluster (9). Past in Flames / Yawgmoth's Will family static.
@_sp(r"^you may cast spells from "
     r"(?:your graveyard|among (?:those )?cards (?:exiled with [^,.]+|"
     r"this turn|exiled this way))(?:[,\s][^.]+)?\s*$")
def _cast_spells_from_zone(m, raw):
    return Static(modification=Modification(
        kind="cast_spells_from_zone"), raw=raw)


# --- "you may play lands and cast <type> spells from <zone>" ---------------
# Cluster (14). Aminatou's Augury / Nezahal-tail / surveil-this-turn rider.
@_sp(r"^you may play lands and cast (?:[a-z, ]*?)?spells "
     r"from (?:your graveyard|the top of your library|among [^.]+|"
     r"among cards exiled with [^,.]+)(?:[,\s][^.]+)?\s*$")
def _play_lands_cast_spells_from(m, raw):
    return Static(modification=Modification(
        kind="play_lands_cast_spells_from"), raw=raw)


# --- "this mana can't be spent to <X>" -------------------------------------
# Cluster (6). Restriction-mana riders on mana-add abilities (Mishra's
# Workshop-class bookkeeping).
@_sp(r"^this mana can'?t be spent to ([^.]+?)\s*$")
def _mana_restricted(m, raw):
    return Static(modification=Modification(
        kind="restricted_mana",
        args=(m.group(1).strip(),)), raw=raw)


# --- "each mode must target a different player" ----------------------------
# Cluster (8). Modal-spell targeting restriction. Static-effect class.
@_sp(r"^each mode must target a different (player|creature|opponent|target)\s*$")
def _each_mode_diff_target(m, raw):
    return Static(modification=Modification(
        kind="each_mode_different_target",
        args=(m.group(1),)), raw=raw)


# --- "each copy targets a different one of <X>" ----------------------------
# Cluster (7). Sister rule for split copies (Bonecrusher Giant /
# Twincast-family). Same shape, different verb.
@_sp(r"^each copy targets a different one of [^.]+?\s*$")
def _each_copy_diff_target(m, raw):
    return Static(modification=Modification(
        kind="each_copy_different_target"), raw=raw)


# --- "you may reveal this card from your opening hand. if you do, ..." ----
# Cluster (8). Leyline / Gemstone Caverns / Serum Powder family. The "if you
# do" tail is a one-shot rules text we don't model — claim the whole thing.
@_sp(r"^you may reveal this card from your opening hand\.?(?:\s+if you do[^$]*)?$")
def _reveal_opening_hand(m, raw):
    return Static(modification=Modification(kind="reveal_opening_hand"),
                  raw=raw)


# --- "after this main phase, there [is an|are N] additional combat phase[s] ..."
# Cluster (8). Aurelia / World at War / Combat Celebrant tail.
@_sp(r"^after this main phase, there (?:is an|are (?:two|three|\d+)) "
     r"additional combat phases?(?: followed by an additional main phase)?\s*$")
def _extra_combat_static(m, raw):
    return Static(modification=Modification(
        kind="extra_combat_phase_static"), raw=raw)


# --- "this effect can't reduce the mana <X>" -------------------------------
# Cluster (9). Cost-reduction safety clause from Goblin Electromancer-family.
@_sp(r"^this effect can'?t reduce the (?:mana|amount of mana|cost) [^.]+?\s*$")
def _effect_cant_reduce_mana(m, raw):
    return Static(modification=Modification(
        kind="effect_cant_reduce_mana"), raw=raw)


# --- "the same is true for <X>" --------------------------------------------
# Cluster (8). Continuation rider where a previous sentence introduced a
# rule and this re-applies it to a parallel population.
@_sp(r"^the same is true for [^.]+?\s*$")
def _same_is_true_for(m, raw):
    return Static(modification=Modification(
        kind="same_rule_applies_to"), raw=raw)


# --- "for as long as that card remains exiled, [you may play|cast it ...]" --
# Cluster (12). Suspend-tail / Eldrazi-Displacer-tail / Fiend Hunter-style.
@_sp(r"^for as long as that (?:card|creature) remains exiled, [^.]+?\s*$")
def _while_exiled_rider(m, raw):
    return Static(modification=Modification(
        kind="while_exiled_rider"), raw=raw)


# --- "during any turn you attacked with <X>, you may <Y>" ------------------
# Cluster (7). Outlaws of Thunder Junction "plot" cousin condition.
@_sp(r"^during any turn you attacked with [^,]+, you may [^.]+?\s*$")
def _during_attacked_turn(m, raw):
    return Static(modification=Modification(
        kind="during_attacked_turn_perm"), raw=raw)


# --- "during your turn, you may <X>" --------------------------------------
# Cluster (8). Time-window static (Heartless Summoning / cycle-this-turn-only
# permission shapes).
@_sp(r"^during your turn, you may [^.]+?\s*$")
def _during_your_turn_you_may(m, raw):
    return Static(modification=Modification(
        kind="during_your_turn_permission"), raw=raw)


# --- "you may look at and play those cards [for as long as ...]" -----------
# Cluster (6). Etali / Spelltithe-Enforcer-family exile-pile permission.
@_sp(r"^you may look at and play those cards(?:[,\s][^.]+)?\s*$")
def _look_at_and_play(m, raw):
    return Static(modification=Modification(
        kind="look_at_and_play_those"), raw=raw)


# --- "each creature card in your graveyard|hand has <X>" -------------------
# Cluster (7). Crypt of Agadeem / Gravecrawler-family graveyard-static.
@_sp(r"^each (creature|land|instant|sorcery|artifact|enchantment|planeswalker)"
     r" cards? in your (graveyard|hand|library) "
     r"(?:has [^.]+|gains [^.]+|is [^.]+)\s*$")
def _cards_in_zone_have(m, raw):
    return Static(modification=Modification(
        kind="cards_in_zone_have_ability",
        args=(m.group(1), m.group(2))), raw=raw)


# --- bare "~" — splitter leaves a bare card-name fragment ------------------
# Cluster (9). When normalize() chews a card name into "~" the splitter can
# orphan a bare "~" between sentences. Treat as a no-op static.
@_sp(r"^~\s*$")
def _bare_tilde_only(m, raw):
    return Static(modification=Modification(kind="bare_self_orphan"), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "you may cast that card <long tail>" ----------------------------------
# Cluster (12). Scrubber #1 has the simple "you may cast that card" form;
# extended tails ("if X", "this turn", "until end of your next turn",
# "without paying its mana cost. then ...") slip past.
@_er(r"^you may cast that card(?:[,\s][^.]+|\s+(?:without paying its mana cost|"
     r"this turn|until [^.]+|for [^.]+))?\.?\s*"
     r"(?:if you do,?[^.]+|if you don'?t[^.]+|then [^.]+|otherwise[^.]+)?\s*"
     r"(?:\.|$)")
def _may_cast_that_long(m):
    return UnknownEffect(raw_text="may cast that card (long tail)")


# --- "you may play those cards / play that card <tail>" --------------------
# Cluster (8+5). Cascade / discover-tail / impulse-draw-tail.
@_er(r"^you may play (?:those|that) cards?(?:[,\s][^.]+|\s+(?:this turn|"
     r"for as long as [^.]+|until [^.]+))?\s*"
     r"(?:and you may spend [^.]+)?(?:\.|$)")
def _may_play_those(m):
    return UnknownEffect(raw_text="may play those/that card")


# --- "you may play the exiled card[s] <tail>" ------------------------------
# Cluster (7). Suspend-resolve / Outpost Siege exile-impulse tail.
@_er(r"^you may play the exiled cards?(?:[,\s][^.]+|\s+(?:this turn|"
     r"for as long as [^.]+|until [^.]+))?\s*"
     r"(?:and you may spend [^.]+)?(?:\.|$)")
def _may_play_exiled(m):
    return UnknownEffect(raw_text="may play the exiled card")


# --- "you may put that card <zone>" ---------------------------------------
# Cluster (11). Sensei's Divining Top tail / scry-then-put / mill-then-put.
@_er(r"^you may put that card "
     r"(?:into (?:your|their|its owner'?s?) (?:hand|graveyard)|"
     r"on (?:the (?:top|bottom)|top|bottom) of (?:your|their|its owner'?s?) library|"
     r"onto the battle~?)"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _may_put_that_card_zone(m):
    return UnknownEffect(raw_text="may put that card -> zone")


# --- "put target creature card from a graveyard onto the battlefield ..." --
# Cluster (10). Dread Return / Animate Dead-tail with the "from a graveyard"
# (rather than "from your graveyard") variant.
@_er(r"^put target ([a-z ]+?) cards? from (?:a|any|your|each) (?:graveyard|"
     r"~yard) onto the battle(?:field|~)(?: under your control| tapped)?"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _put_card_from_gy_to_bf(m):
    return UnknownEffect(raw_text=f"reanimate target {m.group(1).strip()} from gy")


# --- "draw cards equal to <expr>" / "draw N cards equal to <expr>" ---------
# Cluster (10). Variable-draw tail from Sphinx's Revelation / Body Count.
@_er(r"^draw (?:cards|\d+ cards) equal to [^.]+?(?:\.|$)")
def _draw_equal_to(m):
    return UnknownEffect(raw_text="draw cards equal to <X>")


# --- "gain control of target <X> [for as long as ...]" ---------------------
# Cluster (7). Mind Control-family bare effect.
@_er(r"^gain control of target ([a-z ]+?)(?:[,\s][^.]+)?(?:\.|$)")
def _gain_control_target(m):
    base = m.group(1).strip()
    return GainControl(target=Filter(base=base, targeted=True))


# --- "destroy up to one target <X>" ---------------------------------------
# Cluster (7). Up-to-N destroy variants the base parser missed.
@_er(r"^destroy up to one target ([a-z, ]+?)(?:[,\s][^.]+)?(?:\.|$)")
def _destroy_uptoone(m):
    base = m.group(1).strip()
    return Destroy(target=Filter(base=base, quantifier="up_to_n", count=1,
                                 targeted=True))


# --- "destroy up to one target <X>, up to one target <Y>, ..." ------------
# Catch the multi-target chain ("destroy up to one target artifact, up to one
# target enchantment, and up to one target planeswalker").
@_er(r"^destroy up to one target [a-z]+(?:, up to one target [a-z]+)+"
     r"(?:,? and up to one target [a-z]+)?(?:\.|$)")
def _destroy_uptoone_chain(m):
    return UnknownEffect(raw_text="destroy up to one target (chain)")


# --- "exile any number of target creatures [tail]" ------------------------
# Cluster (7). Mass-exile spells with "any number of" quantifier.
@_er(r"^exile any number of target ([a-z, ]+?)(?:[,\s][^.]+)?(?:\.|$)")
def _exile_any_number(m):
    return UnknownEffect(raw_text=f"exile any number of {m.group(1).strip()}")


# --- "you may tap or untap target <X>" ------------------------------------
# Cluster (7). Twiddle-family. Bare effect not in base parse_effect.
@_er(r"^you may tap or untap target ([a-z, ]+?)(?:[,\s][^.]+)?(?:\.|$)")
def _tap_or_untap_target(m):
    return UnknownEffect(raw_text=f"tap or untap target {m.group(1).strip()}")


# --- "any number of target creatures (each get +N/+N|can't ...) [tail]" ---
# Cluster (9). Trumpet Blast-family with "any number" quantifier.
@_er(r"^any number of target creatures "
     r"(?:each (?:get \+\d+/\+\d+(?: and gain [a-z, ]+)?|gain [a-z, ]+)"
     r"(?: until end of turn)?|can'?t [^.]+)(?:\.|$)")
def _any_number_target_creatures(m):
    return UnknownEffect(raw_text="any number of target creatures (verb)")


# --- "draw two cards and create a <token>" --------------------------------
# Cluster (7). Triplicate Spirits / Inspiring Refrain-tail.
@_er(r"^draw (?:a card|two cards|\d+ cards) and create "
     r"(?:a|an|two|three|\d+) [^.]+?tokens?(?:[,\s][^.]+)?(?:\.|$)")
def _draw_and_create_token(m):
    return UnknownEffect(raw_text="draw and create token")


# --- "put one of them into <zone> [and the rest into <zone2>]" ------------
# Cluster (7). Pile-split-tail / Dimir-look-and-keep.
@_er(r"^put one of them into (?:your|their) (hand|graveyard|library)"
     r"(?:\s+and the rest into (?:your|their) (?:hand|graveyard|library))?"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _put_one_of_them(m):
    return UnknownEffect(raw_text=f"put one of them into {m.group(1)}")


# --- "put those cards onto the battlefield [tapped|under your control]" ----
# Cluster (7). Reanimate-multi tail / Living Death-tail.
@_er(r"^put those cards onto the battle(?:field|~)"
     r"(?:\s+tapped|\s+under (?:your|their) control)?"
     r"(?:[,.\s][^.]*)?(?:\.|$)")
def _put_those_to_bf(m):
    return UnknownEffect(raw_text="put those cards onto the battlefield")


# --- "put up to <N> target <X> ..." ---------------------------------------
# Cluster (24). Up-to-N counter / pump rider.
@_er(r"^put up to (?:one|two|three|four|five|\d+) target ([a-z, ]+?)"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _put_uptoN_target(m):
    return UnknownEffect(raw_text=f"put up to N target {m.group(1).strip()}")


# --- "you may put a card from your hand on the bottom ..." -----------------
# Cluster (7). Trade Routes / Wheel-of-Fate-tail.
@_er(r"^you may put a card from your hand on (?:the (?:top|bottom)|top|bottom) "
     r"of your library(?:[,\s][^.]+)?(?:\.|$)")
def _may_put_card_from_hand(m):
    return UnknownEffect(raw_text="may put card from hand on top/bottom")


# --- "you may put one of those cards <verb>" ------------------------------
# Cluster (7). Vivien's-Arkbow-tail / Worldly Tutor-tail.
@_er(r"^you may put one of those cards "
     r"(?:back on top of your library|onto the battle(?:field|~)|"
     r"into your hand)(?:[,\s][^.]+)?(?:\.|$)")
def _may_put_one_of_those(m):
    return UnknownEffect(raw_text="may put one of those cards")


# --- "you may discard a card. if you do, draw [N|a card]." ----------------
# Cluster (6). Looter-rider standard shape.
@_er(r"^you may discard a card\.? if you do, draw (?:a card|two cards|\d+ cards)\.?\s*$")
def _may_discard_then_draw(m):
    return UnknownEffect(raw_text="may discard then draw")


# --- "you choose one of those cards [and exile it]" -----------------------
# Cluster (6). Bribery-style opponent-look-then-take continuation.
@_er(r"^you choose one of those cards(?:[,\s][^.]+)?(?:\.|$)")
def _you_choose_one(m):
    return UnknownEffect(raw_text="you choose one of those cards")


# --- "until your next end step, you may <X>" ------------------------------
# Cluster (8). Mythos / Etali-style impulse-until-next-end-step.
@_er(r"^until your next end step, you may [^.]+?(?:\.|$)")
def _until_next_end_you_may(m):
    return UnknownEffect(raw_text="until your next end step, you may ...")


# --- "you may play them this turn" ----------------------------------------
# Cluster (6). Cascade / impulse-tail bare phrasing.
@_er(r"^you may play them this turn(?:[,\s][^.]+)?(?:\.|$)")
def _may_play_them_this_turn(m):
    return UnknownEffect(raw_text="may play them this turn")


# --- "exile <N> cards from the top of your library" -----------------------
# Cluster (~6 residual). Surveil/scry-cousin top-deck exile.
@_er(r"^exile (?:the top|\d+) cards? (?:from the top )?of (?:your|target player'?s)"
     r" library(?:[,\s][^.]+)?(?:\.|$)")
def _exile_top_lib(m):
    return UnknownEffect(raw_text="exile cards from top of library")


# --- "for each opponent who does, <effect>" -------------------------------
# Cluster (6). Council's Judgment-style per-voter follow-up.
@_er(r"^for each opponent who does,? [^.]+?(?:\.|$)")
def _for_each_opp_who_does(m):
    return UnknownEffect(raw_text="for each opponent who does, ...")


# --- "for each opponent, <effect>" ----------------------------------------
@_er(r"^for each opponent,? [^.]+?(?:\.|$)")
def _for_each_opp(m):
    return UnknownEffect(raw_text="for each opponent, ...")


# --- "create a token that's a copy of <X> [<tail>]" -----------------------
# Cluster (6). Scrubber #1 had "copy of it"; this catches "copy of this
# creature" / "copy of a permanent you control" / "copy of one of them".
@_er(r"^create a token that'?s a copy of "
     r"(?:this creature|a (?:permanent|creature) you control|one of them|"
     r"that creature|target creature)(?:[,\s][^.]+)?(?:\.|$)")
def _token_copy_other(m):
    return UnknownEffect(raw_text="create token copy of <X>")


# --- "search your library for a <X>, ... then shuffle" — base catch-all ----
# Cluster (17 first-5 + spillover). The base parser handles many tutor
# shapes but the long-tail "..., put it onto the battlefield, then shuffle"
# variants (with mana-value qualifiers, "where x is ...", etc.) leak.
@_er(r"^search your library for a [^.]+?,?\s*[^.]+?,? then shuffle"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _search_library_long_tail(m):
    return UnknownEffect(raw_text="search lib for X, ..., shuffle")


# --- "search your library and/or graveyard for a <X> [...]" ---------------
# Cluster (8). Scrubber #1 had the "card named ..." subset; this catches the
# generic mana-value/type tutor.
@_er(r"^search your library and/or graveyard for "
     r"(?:a|an|any number of) [^.]+?(?:\.|$)")
def _search_lib_or_gy(m):
    return UnknownEffect(raw_text="search lib and/or gy for ...")


# --- "look at the top X cards of your library, where X is <expr>" ---------
# Cluster (8). Variable-look effects.
@_er(r"^look at the top x cards of (?:your|target player'?s) library,?\s*"
     r"where x is [^.]+?(?:\.|$)")
def _look_top_x(m):
    return UnknownEffect(raw_text="look at top X cards (variable)")


# --- "exile ~ with <N> time counters on it" — Suspend resolution ----------
# Cluster (9). Suspend-cast self-exile is the trigger body of "when you cast
# this spell". The exile-with-counters resolution leaks.
@_er(r"^exile ~ with (?:\d+|x|one|two|three|four|five) time counters on it\s*$")
def _exile_self_time_counters(m):
    return UnknownEffect(raw_text="exile ~ with N time counters")


# --- "you get a one-time boon with \"<inner trigger>\"" -------------------
# Cluster (7). Festival-of-Embers-family boon-on-card delegated triggers;
# the inner quoted ability is its own grammar problem we don't try to crack.
@_er(r'^you get a one-time boon with "[^"]+"(?:\.|$)')
def _one_time_boon(m):
    return UnknownEffect(raw_text="one-time boon")


# --- "this creature gets +X/+X, where X is ..." ---------------------------
# Cluster (~8). Scrubber #2 has the +X/+0 form; +X/+X with both X's slips.
@_er(r"^(?:this creature|~) gets \+x/\+x,?\s*where x is [^.]+?(?:\.|$)")
def _self_buff_xx(m):
    return UnknownEffect(raw_text="self +X/+X variable")


# --- "enchanted creature gets +N/+N as long as <X>" -----------------------
# Cluster (9). Aura conditional-buff that the base aura grammar misses.
@_er(r"^enchanted creature gets \+(\d+)/\+(\d+) as long as [^.]+?\s*$")
def _enchanted_buff_aslongas(m):
    p, t = int(m.group(1)), int(m.group(2))
    return Buff(power=p, toughness=t,
                target=Filter(base="enchanted_creature", targeted=False),
                duration="permanent")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # --- "whenever another creature you control leaves the battlefield" -
    # Cluster (15). Scrubber #2 has dies+enters+attacks for the "another
    # creature you control" filter, but not "leaves" — and many cards use
    # exactly that phrasing for a generalized blink/death trigger.
    (re.compile(r"^whenever another creature you control leaves the battlefield",
                re.I), "another_ally_leaves", "self"),

    # --- "whenever a creature you control leaves the battlefield" -------
    # Cluster (~5 residual). Same family without "another".
    (re.compile(r"^whenever a creature you control leaves the battlefield",
                re.I), "ally_leaves", "self"),

    # --- "whenever a creature you control exploits a creature" ----------
    # Cluster (~5). Dimir Exploit-rider on a separate ally.
    (re.compile(r"^whenever a creature you control exploits", re.I),
     "ally_exploits", "self"),

    # --- "whenever one or more creatures (you control)? deal combat damage" -
    # Cluster (14 + 7). Crowd-event combat-damage trigger.
    (re.compile(r"^whenever one or more creatures(?: you control)? "
                r"deal combat damage", re.I),
     "one_or_more_creatures_combat_damage", "self"),

    # --- "whenever one or more creatures die" (no qualifier) -------------
    # Cluster (~6). Bare crowd-die trigger.
    (re.compile(r"^whenever one or more creatures die", re.I),
     "one_or_more_creatures_die_any", "self"),

    # --- "whenever one or more other creatures you control [event]" ------
    # Cluster (13). Scrubber #2 had "one or more other creatures (die|enter
    # |leave)"; the "you control" form with same events still leaks.
    (re.compile(r"^whenever one or more other creatures you control "
                r"(enter|leave|die|are dealt damage)", re.I),
     "one_or_more_other_ally_event", "self"),

    # --- "whenever one or more creature cards are put into <zone>" -------
    # Cluster (10). Already partly covered by scrubber #2's
    # "one_or_more_cards_to_zone" but the type-qualified "creature cards"
    # form needs its own anchor.
    (re.compile(r"^whenever one or more creature cards are put into "
                r"(?:your|a|target player'?s|each) (?:graveyard|hand|library)",
                re.I), "creature_cards_to_zone", "self"),

    # --- "whenever one or more creature cards leave your graveyard" -----
    # Cluster (~6). Reverse-zone movement (delve-tail / escape-tail).
    (re.compile(r"^whenever one or more creature cards leave "
                r"(?:your|a|target player'?s) graveyard", re.I),
     "creature_cards_leave_gy", "self"),

    # --- "whenever one or more nontoken <type> [event]" ------------------
    # Cluster (8). Scrubber #2 covered "nontoken creature you control"; the
    # bare nontoken-type form (artifacts, permanents) still leaks.
    (re.compile(r"^whenever one or more nontoken "
                r"(?:artifacts?|permanents?|creatures?|enchantments?)"
                r"(?: you control)? "
                r"(?:enter|die|are put into|leave)", re.I),
     "one_or_more_nontoken_event", "self"),

    # --- "whenever a land an opponent controls enters" ------------------
    # Cluster (6). Mana-screw punisher / Path-of-Mettle-tail trigger.
    (re.compile(r"^whenever a land an opponent controls enters", re.I),
     "opp_land_enters", "self"),

    # --- "whenever a source you control deals (noncombat)? damage" ------
    # Cluster (6). Magus-of-the-Wheel / Indulgent Tormentor-tail.
    (re.compile(r"^whenever a source you control deals "
                r"(?:noncombat )?damage", re.I),
     "ally_source_damage", "self"),

    # --- "whenever the ring tempts you, <effect>" ----------------------
    # Cluster (7). LotR Ring trigger — base parser doesn't have it.
    (re.compile(r"^whenever the ring tempts you", re.I),
     "ring_tempts_you", "self"),

    # --- "when you next cast a <type> spell" ---------------------------
    # Cluster (7). Scrubber #2 has "next cast an instant or sorcery";
    # extend to creature/lesson/etc.
    (re.compile(r"^when you next cast a "
                r"(?:creature|lesson|spell|noncreature|land|"
                r"[a-z ]+? )spell", re.I),
     "next_cast_typed", "self"),

    # --- "when you discard a card this way" ----------------------------
    # Cluster (6). Madness-tail / Faithless-Looting-tail trigger.
    (re.compile(r"^when you discard a card this way", re.I),
     "discard_this_way", "self"),

    # --- "whenever another creature or artifact you control dies/to gy" -
    # Cluster (7). Mardu-vehicles / Korlash-tail dual-type trigger.
    (re.compile(r"^whenever another creature or artifact you control "
                r"(dies|is put into a graveyard from the battlefield|"
                r"enters)", re.I),
     "another_creature_or_artifact_event", "self"),

    # --- "whenever an artifact is put into a graveyard from the battlefield" -
    # Cluster (~5). Scrap Mastery / Argivian Restoration tail trigger.
    (re.compile(r"^whenever an? (?:artifact|enchantment|creature) is put into "
                r"a graveyard from the battlefield", re.I),
     "type_to_gy_from_bf", "self"),
]
