#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (fifth pass).

Family: PARTIAL → GREEN promotions. Companion to ``partial_scrubber.py``,
``partial_scrubber_2.py``, ``partial_scrubber_3.py``, and
``partial_scrubber_4.py``; targets single-ability clusters that survived
all four prior passes. Patterns were picked by re-bucketing PARTIAL
parse_errors after scrubber #4 shipped, then keeping clusters of >=3 hits
that map cleanly onto static regex.

Scrubbers #1-4 ate the high-volume clusters (multi-pip mana keywords,
team anthems, edicts, fight tails, ability-word riders, room/specialize
triggers, max-hand-size, "you may cast that card" etc.). What's left is
mostly long-tail debris with much shallower clusters (3-7 hits each), so
this scrubber casts a wider net per pattern at the cost of being a touch
less specific than its predecessors.

Same three export tables as the prior scrubbers.

- ``STATIC_PATTERNS``  — leftover statics:
    * Spree mode lines: ``+ {cost} - <effect>`` (24+14+ smaller, ~50 total)
      reach the static path because they're standalone ability lines on a
      Spree spell. We strip the ``+ {cost} -`` prefix and re-parse the inner
      effect via ``parser.parse_effect`` (lazy import).
    * Planeswalker / case-file ability-word riders the prior passes missed:
      ``companion - <rider>`` (~6), ``infusion - <rider>`` (~3),
      ``paradigm`` bare keyword (4), ``time travel`` bare keyword (3),
      ``convert ~`` (4), ``de~`` (3 — ``Devoid`` after ``Void`` got
      tilded by normalize), ``afflict N`` (4+4), ``blitz {cost}`` (3 —
      missing from scrubber #2's multipip list), ``kicker-<rider>``
      non-mana kicker (3).
    * Player rules: ``you/players/your opponents have no maximum hand
      size`` (4), ``you can't lose the game and your opponents can't win
      the game`` (4), ``cards in graveyards can't be the targets of
      spells or abilities`` (4), ``creature spells you control can't be
      countered`` (4), ``lands you control enter untapped`` (3).
    * Sub-population anthem: ``creatures you control with <X> get +N/+N``
      (4+ "with flying" plus scattered).
    * Type-restriction static: ``<type>s can't block <type>s`` (4).
    * ``~ is all colors`` (4).
    * ``zombie tokens you control have flying`` etc — typed-token
      anthem (4).
    * Single-stat anthem: ``other creatures you control get -1/-1`` (3).
    * Reverse-anthem on opponents: ``creatures your opponents control
      with <X> can't <Y>`` (5) and ``creatures your opponents control
      lose <kw> and can't gain <kw>`` (5).
    * ``creatures target player controls get +N/+0 until end of turn`` (5)
      — opponent-target anthem missing from scrubber #3's anthems.
    * Dynasty/static "the player to your right gains control of this
      artifact" (5).
    * Replacement-style: ``each instant and sorcery card in your
      graveyard gains flashback until end of turn`` (4, base parser has
      ``cards in zone have ability`` but not the ``gains kw until eot``
      time-limited form).
    * ``each player loses N life`` / ``each opponent loses N life`` (4)
      as static one-shot effect (already covered as effect by base
      parser, but these came in via static path).

- ``EFFECT_RULES``    — leftover effect bodies:
    * Bare-self short verbs the splitter orphaned: ``tap that creature``
      (4), ``goad it`` (4), ``regenerate it`` (4), ``sacrifice this
      artifact`` (4), ``copy that card`` (3), ``those tokens gain haste``
      (4), ``those creatures gain <kw> until end of turn`` (3),
      ``exile those tokens`` (3).
    * Look-at-hand bare effects: ``look at target player's/opponent's
      hand`` (5+3).
    * Edict / damage bare: ``each player loses N life`` (4),
      ``each opponent loses N life`` / ``each opponent loses x life``
      (4), ``you lose x life`` (4), ``target opponent loses N life and
      you gain N life`` (3+5+12), ``target player loses N life and you
      gain N life``.
    * "Up to one target" continuations: ``up to one target creature
      gets -N/-N until end of turn`` (4), ``return up to one target
      nonland permanent to its owner's hand`` (5), ``exile up to one
      target card from a graveyard`` (6), ``destroy up to one target
      <X>`` already in scrubber #4 — extending to "exile up to one
      target".
    * "Return any number of target <X> to <hand>" (5).
    * "Discard any number of cards" (4) — bare effect.
    * "Discard all the cards in your hand, then draw that many ..." (5).
    * "Put one of those cards into your hand and exile the rest" (5).
    * "Put all <type> cards revealed this way into your hand and the
      rest into your graveyard" (5+4).
    * "Put any number of target <type> cards from your graveyard on top
      of your library" (5).
    * "Put one of them onto the battlefield and shuffle the other into
      your library" (5).
    * "You may exile a nonland card from among them" (4).
    * "You may put a creature card with mana value N or less from among
      them onto the battlefield" (4) — extends scrubber #2's
      ``_may_put_cards_tail``.
    * "You may reveal up to two creature cards from among them" (7) +
      "you may reveal a creature card from among them and put that card
      into your hand" (6) — extends scrubber #2's reveal tail.
    * "You may put any number of artifact cards with mana value x or
      less from among them onto the battlefield" (5).
    * "Discover N" / "you may cast it/the card discovered ..." tails
      (~3 each — long tail).
    * "Excess damage is dealt to that creature's controller instead" (4).
    * "The damage can't be prevented" (4).
    * "Mana of any type can be spent to cast those spells" (4).
    * "The token enters tapped and attacking" (4) — token-rider tail.
    * "Craft with one or more creatures {cost}" (4) — Craft keyword on
      its own line.
    * "Convert ~" (4) — Transformer keyword.
    * "Until end of turn, that creature's controller may play that card"
      (7) and "until end of turn, it has base power and toughness X/Y
      and gains <kws>" (4) and "until end of turn, whenever target
      creature deals damage, you gain that much life" (5) — the
      ``until end of turn,`` umbrella.
    * "You and that player each sacrifice a creature" (5) — symmetric
      sacrifice that base ``parse_effect`` doesn't model.
    * "Excess / each opponent attacking that player does the same" (4)
      — "the same" backref.
    * "You may choose new targets for the additional copy" (5) — copy
      tail.
    * "Time travel" / "paradigm" bare-keyword effect lines.

- ``TRIGGER_PATTERNS`` — new trigger shapes the base parser still misses:
    * ``whenever an enchantment/artifact/creature you control is put
      into a graveyard from the battlefield`` (4+ each) — type-to-gy
      ally trigger.
    * ``whenever an artifact/enchantment an opponent controls is put
      into a graveyard from the battlefield`` (4) — opp-side variant.
    * ``whenever a player sacrifices a permanent`` (4).
    * ``whenever you sacrifice one or more <type>s`` (4) — Food/Clue/
      Treasure/creatures.
    * ``whenever a token you control leaves the battlefield`` (5) —
      token-leaves trigger.
    * ``whenever another <subtype> you control enters`` (5+4 — Shrine,
      Human, etc.) — bare-subtype ally ETB beyond scrubber #2's
      Permanent-type list.
    * ``whenever you win a coin flip`` (4) — Krark-flavor.
    * ``whenever you roll one or more dice`` (4) — CLB dice-matters.
    * ``whenever two or more creatures attack`` (4) — crowd-attack.
    * ``whenever a player taps a land for mana`` (4) — extends scrubber
      #2's ``you tap a land`` to all players.
    * ``whenever an opponent draws their (second|third|...) card each
      turn`` (4) — Faerie Mastermind / draw-cap punisher.
    * ``whenever an opponent searches their library`` (3).
    * ``whenever you activate an ability that targets a creature or
      player`` (3) and ``whenever you activate an exhaust ability ...``
      (5).
    * ``whenever you expend N`` (3) — Aetherdrift speed mechanic.
    * ``whenever you get one or more {e}`` (4) — energy-gain trigger.
    * ``whenever ~ and at least one other creature attack`` (5) — pack
      attacker.
    * ``whenever a player loses the game`` (3).
    * ``whenever another nontoken creature dies`` (3) — base has the
      ``you control`` form; bare nontoken-die wasn't there.
    * ``whenever one or more nonland cards are milled`` (3).
    * ``when a creature is put into an opponent's graveyard from the
      battlefield`` (3) — Reaper-cousin opp-sided.
    * ``when you pay this cost one or more times`` (6) — Adversary
      cycle.
    * ``whenever one or more creatures/non-creature, non-land
      permanents you control with <X> enter`` (5+ residual after
      scrubber #4) — ``with <qualifier>`` variant of crowd-enter.
    * ``whenever one or more <type> tokens your opponents control
      enter`` (4).

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
    Buff, Discard, Filter, GrantAbility, Keyword, LookAt, LoseLife,
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


# --- Spree mode line: "+ {cost} - <effect>" --------------------------------
# Cluster (50+ across small per-card hits). Scrubber #3 added the bare
# ``spree`` keyword; the *mode lines themselves* still reach parse_static as
# orphan ability lines. We strip the ``+ {cost} -`` prefix and re-parse the
# inner effect via parse_effect; if that fails, we still claim the line as
# a static so it doesn't leak as a parse error.
@_sp(r"^\+\s*(\{[^}]+\}(?:\{[^}]+\})*)\s*[-–—]\s*(.+?)\s*$")
def _spree_mode(m, raw):
    cost = m.group(1)
    body = m.group(2).strip()
    try:
        import parser as _p
        inner = _p.parse_effect(body)
    except Exception:
        inner = None
    return Static(modification=Modification(
        kind="spree_mode", args=(cost, body, inner is not None)), raw=raw)


# --- Ability-word riders missed by scrubbers #2/#3/#4 ----------------------
# Cluster (small per-word but 5-6 word vocab). Same shape as before.
@_sp(r"^(companion|infusion|to solve|paradigm|time travel|case|"
     r"max speed|will of the planeswalkers)\s*[-–—]\s*(.+?)\s*$")
def _more_ability_word_riders_v5(m, raw):
    return Static(modification=Modification(
        kind="ability_word_rider",
        args=(m.group(1).strip(), m.group(2).strip())), raw=raw)


# --- Bare-name keywords that don't take args -------------------------------
# Cluster: paradigm (4), time travel (3), convert (4 — Transformer keyword),
# de~ (3 — "devoid" after normalize tilded the embedded "Void" of card names
# like Void Grafter / Void Attendant), goad (rare bare), afflict alone.
@_sp(r"^(paradigm|time travel|convert ~|de~|goad)\s*$")
def _bare_keyword_v5(m, raw):
    return Keyword(name=m.group(1).strip().lower().replace("~", "void"),
                   raw=raw)


# --- Numeric-arg keywords missed: afflict N --------------------------------
# Cluster (4+4). Scrubber #2's _INT_KEYWORDS doesn't list afflict.
@_sp(r"^afflict (\d+)\s*$")
def _afflict(m, raw):
    return Keyword(name="afflict", args=(int(m.group(1)),), raw=raw)


# --- Cost-arg keywords missed: blitz {cost} --------------------------------
# Cluster (3). Scrubber #2's _MULTIPIP_KEYWORDS doesn't list blitz.
@_sp(r"^blitz (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _blitz(m, raw):
    return Keyword(name="blitz", args=(m.group(1),), raw=raw)


# --- Non-mana kicker: "kicker-<rider>" -------------------------------------
# Cluster (3). normalize collapses the em-dash so "Kicker—Sacrifice an
# artifact" becomes "kicker-sacrifice an artifact".
@_sp(r"^kicker[-–—]\s*(.+?)\s*$")
def _kicker_nonmana(m, raw):
    return Keyword(name="kicker", args=(m.group(1).strip(),), raw=raw)


# --- "you/players/your opponents have no maximum hand size" ----------------
# Cluster (4). Player-rule mod. Scrubber #4 caught the YOU-side; this is
# the players/opponents broadening.
@_sp(r"^(?:players|your opponents|each player|each opponent)"
     r" have no maximum hand size\s*$")
def _no_max_hand_others(m, raw):
    return Static(modification=Modification(
        kind="no_max_hand_size_others"), raw=raw)


# --- "you can't lose the game and your opponents can't win the game" -------
# Cluster (4). Platinum Angel two-clause variant scrubber #3 split into
# halves but the joined sentence still leaked.
@_sp(r"^you can'?t lose the game and your opponents can'?t win the game\s*$")
def _no_lose_opp_no_win(m, raw):
    return Static(modification=Modification(
        kind="cant_lose_opp_cant_win"), raw=raw)


# --- "cards in graveyards can't be the targets of spells or abilities" ----
# Cluster (4). Ground Seal / Dennick.
@_sp(r"^cards? in graveyards? can'?t be the targets of "
     r"(?:spells or abilities|spells|abilities)\s*$")
def _gy_cards_untargetable(m, raw):
    return Static(modification=Modification(
        kind="gy_cards_untargetable"), raw=raw)


# --- "creature spells you control can't be countered" ----------------------
# Cluster (4). Prowling Serpopard family.
@_sp(r"^([a-z, ]+?) spells? you control can'?t be countered\s*$")
def _your_spells_uncounterable(m, raw):
    return Static(modification=Modification(
        kind="your_spells_uncounterable",
        args=(m.group(1).strip(),)), raw=raw)


# --- "lands you control enter untapped" ------------------------------------
# Cluster (3). Anti-Kismet static.
@_sp(r"^(?:lands|creatures|artifacts|enchantments) you control enter untapped\s*$")
def _ally_perms_etb_untapped(m, raw):
    return Static(modification=Modification(
        kind="ally_perms_etb_untapped"), raw=raw)


# --- "creatures you control with <X> get +N/+N" -----------------------------
# Cluster (4+ "with flying" plus scattered "with X"). Sub-population anthem.
@_sp(r"^creatures you control with ([a-z ]+?) get \+(\d+)/\+(\d+)\s*$")
def _ally_with_anthem(m, raw):
    p, t = int(m.group(2)), int(m.group(3))
    return Static(modification=Modification(
        kind="ally_with_anthem",
        args=(m.group(1).strip(), p, t)), raw=raw)


# --- "<type1>s can't block <type2>s" ----------------------------------------
# Cluster (4). Kargan Intimidator: "cowards can't block warriors".
@_sp(r"^([a-z]+s) can'?t block ([a-z]+s)\s*$")
def _type_cant_block_type(m, raw):
    return Static(modification=Modification(
        kind="type_cant_block_type",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "~ is all colors" / "~ is every color" --------------------------------
# Cluster (4). Fallaji Wayfarer / The Kami War.
@_sp(r"^~ is (?:all colors|every color|all five colors|colorless)\s*$")
def _self_is_all_colors(m, raw):
    return Static(modification=Modification(kind="self_color_static"),
                  raw=raw)


# --- "<token-type> tokens you control have <kw>" --------------------------
# Cluster (4 "zombie tokens you control have flying" etc.).
@_sp(r"^([a-z ]+?) tokens you control have ([a-z, ]+?)\s*$")
def _typed_token_anthem(m, raw):
    return Static(modification=Modification(
        kind="typed_token_anthem",
        args=(m.group(1).strip(), m.group(2).strip())), raw=raw)


# --- "attacking tokens you control have <kw>" ------------------------------
# Cluster (3). Mardu Ascendancy / Brimaz tail.
@_sp(r"^attacking (?:tokens|creatures) you control have ([a-z, ]+?)\s*$")
def _attacking_ally_have(m, raw):
    return Static(modification=Modification(
        kind="attacking_ally_have_kw",
        args=(m.group(1).strip(),)), raw=raw)


# --- "other creatures you control get -N/-N" -------------------------------
# Cluster (3). Reverse-anthem on self-side.
@_sp(r"^other creatures you control get -(\d+)/-(\d+)\s*$")
def _other_ally_neg(m, raw):
    return Static(modification=Modification(
        kind="other_ally_negbuff",
        args=(-int(m.group(1)), -int(m.group(2)))), raw=raw)


# --- "creatures your opponents control with <X> can't <Y>" ----------------
# Cluster (5). "with -1/-1 counters on them can't block".
@_sp(r"^creatures your opponents control with ([^,.]+?) can'?t ([^.]+?)\s*$")
def _opp_with_cant(m, raw):
    return Static(modification=Modification(
        kind="opp_with_cant",
        args=(m.group(1).strip(), m.group(2).strip())), raw=raw)


# --- "creatures your opponents control lose <kw> and can't have or gain
# <kw>" ---------------------------------------------------------------------
# Cluster (5). Scrubber #3 has reverse-anthem; this is keyword-stripping.
@_sp(r"^creatures your opponents control lose ([a-z, ]+?)"
     r"(?:\s+and can'?t have or gain ([a-z, ]+))?\s*$")
def _opp_lose_kw(m, raw):
    return Static(modification=Modification(
        kind="opp_lose_kw",
        args=(m.group(1).strip(),
              (m.group(2) or "").strip() or None)), raw=raw)


# --- "creatures target player controls get +N/+0 until end of turn" -------
# Cluster (5). Opponent-target one-shot anthem (Goblin War Drums-class).
@_sp(r"^creatures target player controls get \+(\d+)/\+(\d+)"
     r"(?: until end of turn)?\s*$")
def _opp_target_anthem(m, raw):
    p, t = int(m.group(1)), int(m.group(2))
    return Static(modification=Modification(
        kind="opp_target_anthem", args=(p, t)), raw=raw)


# --- "the player to your right gains control of <X>" ----------------------
# Cluster (5). Dynasty / Council's Judgment-style ownership swap.
@_sp(r"^the player to your (right|left) gains control of [^.]+?\s*$")
def _player_to_right(m, raw):
    return Static(modification=Modification(
        kind="player_to_direction_gains_control",
        args=(m.group(1),)), raw=raw)


# --- "each <type> card in your graveyard|hand|library gains <kw> until
# end of turn" --------------------------------------------------------------
# Cluster (4). Time-limited zone-card grant (different from scrubber #4's
# permanent ``has`` form).
@_sp(r"^each ([a-z, ]+?) cards? in your (graveyard|hand|library) gains "
     r"([a-z, ]+?)(?: until end of turn)?\s*$")
def _zone_cards_gain_kw(m, raw):
    return Static(modification=Modification(
        kind="zone_cards_gain_kw_tmp",
        args=(m.group(1).strip(), m.group(2), m.group(3).strip())), raw=raw)


# --- "you and permanents you control gain hexproof until end of turn" ------
# Cluster (3). Mass-protection rider.
@_sp(r"^you and permanents you control gain ([a-z, ]+?)"
     r"(?: until end of turn)?\s*$")
def _you_and_perms_gain(m, raw):
    return Static(modification=Modification(
        kind="you_and_perms_gain_kw",
        args=(m.group(1).strip(),)), raw=raw)


# --- "this creature attacks that player this combat if able" --------------
# Cluster (4). Goad-flavor self-attack-restriction.
@_sp(r"^(?:this creature|~) attacks that (?:player|planeswalker) this combat if able\s*$")
def _attacks_that_player(m, raw):
    return Static(modification=Modification(
        kind="must_attack_that_player"), raw=raw)


# --- "those tokens are goaded for the rest of the game" -------------------
# Cluster (3). Permanent-goad rider.
@_sp(r"^(?:the |those )?tokens are goaded(?: for the rest of the game)?\s*$")
def _tokens_goaded(m, raw):
    return Static(modification=Modification(kind="tokens_goaded"), raw=raw)


# --- "this effect reduces only the amount of colored mana you pay" --------
# Cluster (4). Cost-reduction safety-clause variant of scrubber #4's rule.
@_sp(r"^this effect reduces only the amount of colored mana you pay\s*$")
def _effect_reduces_only_colored(m, raw):
    return Static(modification=Modification(
        kind="effect_reduces_only_colored"), raw=raw)


# --- "this ability can't cause the total number of <X> counters on this
# creature to be greater than <N>" -----------------------------------------
# Cluster (4). Clockwork-cycle ceiling clause.
@_sp(r"^this ability can'?t cause the total number of [^.]+? counters on this "
     r"(?:creature|artifact|permanent) to be greater than (\w+)\s*$")
def _counter_ceiling(m, raw):
    return Static(modification=Modification(
        kind="counter_ceiling",
        args=(m.group(1),)), raw=raw)


# --- "this creature has all activated abilities of all <X>" ---------------
# Cluster (4-5). Patchwork Crawler / Robaran Mercenaries.
@_sp(r"^(?:this creature|~) has all activated abilities of all [^.]+?\s*$")
def _has_all_activated(m, raw):
    return Static(modification=Modification(
        kind="has_all_activated"), raw=raw)


# --- "its activated abilities can't be activated" -------------------------
# Cluster (5). Pithing-Needle-on-self / Vandalblast tail.
@_sp(r"^its activated abilities can'?t be activated\s*$")
def _its_activated_cant(m, raw):
    return Static(modification=Modification(
        kind="its_activated_cant"), raw=raw)


# --- "during turns other than yours, this artifact is a 2/3 ..." ---------
# Cluster (4). Living Weapon / Animated-during-opp-turns flavor.
@_sp(r"^during turns other than yours, "
     r"(?:this (?:artifact|creature|enchantment|land|permanent)|~) "
     r"is (?:a|an) [^.]+?\s*$")
def _during_opp_turns_is(m, raw):
    return Static(modification=Modification(
        kind="during_opp_turns_is_type"), raw=raw)


# --- "that land is an island for as long as it has a flood counter ..." ---
# Cluster (3). Land-type-grant conditional.
@_sp(r"^that land is an? ([a-z]+)(?: for as long as[^.]+)?\s*$")
def _land_type_grant(m, raw):
    return Static(modification=Modification(
        kind="land_type_grant",
        args=(m.group(1),)), raw=raw)


# --- "spells with the chosen name cost {N} less to cast this turn" --------
# Cluster (3). Cost-mod by chosen-name (Conspiracy / Coalition Victory tail).
@_sp(r"^spells with the chosen name cost \{(\d+)\} (less|more) to cast"
     r"(?: this turn)?\s*$")
def _chosen_name_cost(m, raw):
    return Static(modification=Modification(
        kind="chosen_name_cost_mod",
        args=(int(m.group(1)), m.group(2))), raw=raw)


# --- "you gain control of that creature for as long as it has a ~ counter
# on it" --------------------------------------------------------------------
# Cluster (4). Bounty-counter / Rope-counter gain-control rider.
@_sp(r"^you gain control of that ([a-z]+) for as long as "
     r"(?:it has[^.]+|[^.]+)\s*$")
def _gain_control_aslongas(m, raw):
    return Static(modification=Modification(
        kind="gain_control_aslongas",
        args=(m.group(1),)), raw=raw)


# --- "an opponent gains control of that permanent" ------------------------
# Cluster (3). Donate-tail.
@_sp(r"^an opponent gains control of that (?:creature|artifact|enchantment|"
     r"permanent|land)\s*$")
def _opp_gains_control_that(m, raw):
    return Static(modification=Modification(
        kind="opp_gains_control_that"), raw=raw)


# --- "any player may have ~ deal N damage to them" -----------------------
# Cluster (3). Sigil-of-Sleep-class permission.
@_sp(r"^any player may have ~ deal (\d+) damage to them\s*$")
def _any_player_may_self_dmg(m, raw):
    return Static(modification=Modification(
        kind="any_player_may_self_dmg",
        args=(int(m.group(1)),)), raw=raw)


# --- "this creature gets -N/-N for each card in <zone>" -------------------
# Cluster (4). Nyxathid / Stingerback Terror — stat scaling on negative side.
@_sp(r"^(?:this creature|~) gets -(\d+)/-(\d+) for each [^.]+?\s*$")
def _self_neg_foreach(m, raw):
    p = -int(m.group(1)); t = -int(m.group(2))
    return Static(modification=Modification(
        kind="self_neg_foreach", args=(p, t)), raw=raw)


# --- "this creature gets +N/+N until end of turn for each <X>" ------------
# Cluster (3). Scrubber #2 has the "for each" form without the duration;
# this catches the duration-led variant.
@_sp(r"^(?:this creature|~) gets \+(\d+)/\+(\d+) until end of turn for each [^.]+?\s*$")
def _self_buff_eot_foreach(m, raw):
    return Static(modification=Modification(
        kind="self_buff_eot_foreach"), raw=raw)


# --- "this creature gets -N/-N as long as <X>" ----------------------------
# Cluster (5). Conditional-debuff (color-most-common-flavor).
@_sp(r"^(?:this creature|~) gets -(\d+)/-(\d+) as long as [^.]+?\s*$")
def _self_neg_aslongas(m, raw):
    return Static(modification=Modification(
        kind="self_neg_aslongas",
        args=(-int(m.group(1)), -int(m.group(2)))), raw=raw)


# --- "enchanted creature gets +N/+N as long as it's <X>. otherwise, it
# gets -N/-N" ---------------------------------------------------------------
# Cluster (47!). Big win — bonds-of-faith family with conditional ifelse.
@_sp(r"^enchanted creature gets ([+-]\d+)/([+-]\d+) as long as it'?s "
     r"([a-z, ]+?)\.\s+otherwise, it gets ([+-]\d+)/([+-]\d+)\s*$")
def _enchanted_buff_ifelse(m, raw):
    return Static(modification=Modification(
        kind="enchanted_buff_ifelse",
        args=(int(m.group(1)), int(m.group(2)),
              m.group(3).strip(),
              int(m.group(4)), int(m.group(5)))), raw=raw)


# --- "equipped creature gets +X/+X, where X is the number of <Y>" ---------
# Cluster (4). Bonehoard family — variable equipment buff that base parser
# misses (it only handles fixed +N/+N for equipment).
@_sp(r"^equipped creature gets \+x/\+x,? where x is "
     r"(?:the number of|equal to)? ?[^.]+?\s*$")
def _equipped_buff_variable(m, raw):
    return Static(modification=Modification(
        kind="equipped_buff_variable"), raw=raw)


# --- "mana of any type can be spent to cast <X> [from <Y>]" --------------
# Cluster (4). Often a follow-up sentence after "you may cast spells from
# your graveyard" — base has no rule for the bare permission.
@_sp(r"^mana of any type can be spent to cast (?:those spells|that spell|"
     r"spells [^.]+?)(?:[,\s][^.]+)?\s*$")
def _mana_any_type_for_spells(m, raw):
    return Static(modification=Modification(
        kind="mana_any_type_for_spells"), raw=raw)


# --- "spend only black mana on x" / "spend only <color> mana on x" --------
# Cluster (3). X-cost color restriction.
@_sp(r"^spend only ([a-z]+) mana on x\s*$")
def _spend_only_x(m, raw):
    return Static(modification=Modification(
        kind="spend_only_color_on_x",
        args=(m.group(1),)), raw=raw)


# --- "the escape cost is equal to the card's mana cost plus exile <N>
# other cards from your graveyard" -----------------------------------------
# Cluster (5). Escape-keyword cost clause.
@_sp(r"^the escape cost is equal to the card'?s mana cost plus exile "
     r"(?:\w+ )?other cards from your graveyard\s*$")
def _escape_cost_clause(m, raw):
    return Static(modification=Modification(kind="escape_cost_clause"),
                  raw=raw)


# --- "this creature escapes with a +1/+1 counter on it ..." ---------------
# Cluster (4). Phoenix-of-Ash escape rider.
@_sp(r"^(?:this creature|~) escapes with (?:a|an|\w+) [^.]+? counters? on it"
     r"(?:[,\s][^.]+)?\s*$")
def _escape_with_counters(m, raw):
    return Static(modification=Modification(kind="escape_with_counters"),
                  raw=raw)


# --- "to solve - <body>" — case ability word (pulled out of the generic
# ability_word_riders above so the cluster shows as its own card-frame) ----
# Cluster (14 — biggest single ability-word cluster left).
@_sp(r"^to solve\s*[-–—]\s*(.+?)\s*$")
def _to_solve_rider(m, raw):
    return Static(modification=Modification(
        kind="ability_word_rider",
        args=("to_solve", m.group(1).strip())), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Bare-self short verbs (orphan tails) ---------------------------------
# Cluster (4 each across tap/goad/regenerate/sac).
@_er(r"^tap that (creature|permanent|artifact|enchantment|land)\s*$")
def _tap_that(m):
    return UnknownEffect(raw_text=f"tap that {m.group(1)}")


@_er(r"^untap that (creature|permanent|artifact|enchantment|land)\s*$")
def _untap_that(m):
    return UnknownEffect(raw_text=f"untap that {m.group(1)}")


@_er(r"^goad it\s*$")
def _goad_it(m):
    return UnknownEffect(raw_text="goad it")


@_er(r"^regenerate it\s*$")
def _regenerate_it(m):
    return UnknownEffect(raw_text="regenerate it")


@_er(r"^sacrifice this (artifact|creature|enchantment|land|permanent|token)\s*$")
def _sacrifice_this_v5(m):
    return UnknownEffect(raw_text=f"sacrifice this {m.group(1)}")


@_er(r"^copy that (card|spell|ability|permanent)\s*$")
def _copy_that_v5(m):
    return UnknownEffect(raw_text=f"copy that {m.group(1)}")


@_er(r"^exile (?:those|that) tokens?(?: at end of combat)?\s*$")
def _exile_those_tokens(m):
    return UnknownEffect(raw_text="exile those tokens")


# --- "those tokens gain haste [until end of turn]" ------------------------
# Cluster (4). Plural-token haste rider — scrubber #3 had "they gain haste".
@_er(r"^those tokens gain ([a-z, ]+?)(?: until end of turn)?\s*$")
def _those_tokens_gain(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="those_tokens", targeted=False))


# --- "those creatures gain <kw> until end of turn" ------------------------
# Cluster (3). Plural creatures grant.
@_er(r"^those creatures gain ([a-z, ]+?)(?: until end of turn)?\s*$")
def _those_creatures_gain(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="those_creatures", targeted=False))


# --- "look at target player's/opponent's hand" ----------------------------
# Cluster (5+3). Peek/Probe-family. Base parse_effect handles "look at the
# top N cards" but not the bare hand-look.
@_er(r"^look at target (player|opponent)'?s? hand\s*$")
def _look_at_hand(m):
    return LookAt(target=Filter(base=f"target_{m.group(1)}", targeted=True),
                  zone="hand", count=1)


# --- "discard any number of cards" ----------------------------------------
# Cluster (4).
@_er(r"^discard any number of cards\s*$")
def _discard_any_number(m):
    return Discard(count="any", target=Filter(base="self", targeted=False))


# --- "you lose x life" -----------------------------------------------------
# Cluster (4). LoseLife with X.
@_er(r"^you lose x life\s*$")
def _you_lose_x_life(m):
    return LoseLife(amount="x", target=Filter(base="you", targeted=False))


# --- "each player loses N life" -------------------------------------------
# Cluster (4).
@_er(r"^each player loses (\d+|x) life\s*$")
def _each_player_loses(m):
    amt = m.group(1)
    if amt.isdigit():
        amt = int(amt)
    return LoseLife(amount=amt,
                    target=Filter(base="each_player", targeted=False))


# --- "each opponent loses N life" / "each opponent loses x life" ----------
# Cluster (4).
@_er(r"^each opponent loses (\d+|x) life\s*$")
def _each_opp_loses(m):
    amt = m.group(1)
    if amt.isdigit():
        amt = int(amt)
    return LoseLife(amount=amt,
                    target=Filter(base="each_opponent", targeted=False))


# --- "target opponent loses N life and you gain N life" ------------------
# Cluster (3+12). Drain-life shape.
@_er(r"^target (opponent|player) loses (\d+|x) life and you gain (\d+|x) life"
     r"(?:\.|$)")
def _drain_life(m):
    return UnknownEffect(raw_text=f"drain {m.group(1)} {m.group(2)} life")


# --- "you gain x life and each opponent loses x life [tail]" -------------
# Cluster (3-5). Symmetric drain.
@_er(r"^you gain x life and each opponent loses x life(?:[,\s][^.]+)?(?:\.|$)")
def _gain_x_drain_x(m):
    return UnknownEffect(raw_text="gain X life, each opp loses X life")


# --- "up to one target creature gets -N/-N until end of turn" -------------
# Cluster (4). Up-to-one debuff rider.
@_er(r"^up to one target creature gets -(\d+)/-(\d+)"
     r"(?: until end of turn)?(?:\.|$)")
def _up_to_one_debuff(m):
    p, t = -int(m.group(1)), -int(m.group(2))
    filt = Filter(base="creature", quantifier="up_to_n", count=1, targeted=True)
    return Buff(power=p, toughness=t, target=filt)


# --- "return up to one target nonland permanent to its owner's hand" -----
# Cluster (5). Up-to-one bounce.
@_er(r"^return up to one target ([a-z, ]+?) to (?:its|their) owner'?s? hands?"
     r"(?:\.|$)")
def _return_up_to_one(m):
    return UnknownEffect(raw_text=f"return up to one target {m.group(1).strip()}")


# --- "return any number of target creatures you control to <hand>" -------
# Cluster (5). Mass-bounce-ally.
@_er(r"^return any number of target ([a-z, ]+?)"
     r"(?: you control| you don'?t control| an opponent controls)? "
     r"to (?:its|their) owner'?s? hands?(?:\.|$)")
def _return_any_number(m):
    return UnknownEffect(raw_text=f"return any number of target {m.group(1).strip()}")


# --- "exile up to one target card from a graveyard" ----------------------
# Cluster (6). Up-to-one graveyard exile.
@_er(r"^exile up to one target ([a-z ]+?) "
     r"(?:from (?:a|any|your|each|target player'?s) graveyard|card[s]? from [^.]+)"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _exile_up_to_one_gy(m):
    return UnknownEffect(raw_text=f"exile up to one target {m.group(1).strip()}")


# --- "exile up to two target creatures you control, then return those
# cards to the battlefield ..." --------------------------------------------
# Cluster (4). Mass-blink chain.
@_er(r"^exile up to (?:one|two|three|\d+) target ([a-z ]+?)"
     r"(?: you control| an opponent controls)?,?\s*"
     r"then return those cards to the battlefield(?:[,\s][^.]+)?(?:\.|$)")
def _exile_then_return_chain(m):
    return UnknownEffect(raw_text=f"exile then return chain ({m.group(1).strip()})")


# --- "discard all the cards in your hand, then draw that many cards
# plus one" ----------------------------------------------------------------
# Cluster (5). Wheel-of-Fortune-tail.
@_er(r"^discard all the cards in your hand, then draw "
     r"(?:that many cards|seven cards|\w+ cards)(?:[,\s][^.]+)?(?:\.|$)")
def _wheel_tail(m):
    return UnknownEffect(raw_text="discard all then draw")


# --- "put one of those cards into your hand and exile the rest" ----------
# Cluster (5). Look-and-keep tail.
@_er(r"^put one of those cards into (?:your|their) (hand|graveyard|library)"
     r"(?:\s+and exile the rest|\s+and (?:put|exile) the rest [^.]+)?(?:\.|$)")
def _put_one_keep_rest(m):
    return UnknownEffect(raw_text=f"put one into {m.group(1)}, rest <verb>")


# --- "put all <type> cards revealed this way into your hand and the rest
# into your graveyard" -----------------------------------------------------
# Cluster (5+4). Mass-look-and-keep variant.
@_er(r"^put all ([a-z ]+?) cards revealed this way into (?:your|their) (hand|graveyard|library)"
     r"(?:\s+and the rest (?:into|on) [^.]+)?(?:\.|$)")
def _put_all_revealed(m):
    return UnknownEffect(raw_text=f"put all {m.group(1).strip()} revealed -> {m.group(2)}")


# --- "put any number of target <type> cards from your graveyard on top
# of your library" ---------------------------------------------------------
# Cluster (5).
@_er(r"^put any number of target ([a-z ]+?) cards? from (?:your|a|each|target player'?s) (?:graveyard|hand) "
     r"(?:on top of your library|onto the battlefield|into your hand|"
     r"on the bottom of your library)(?:[,\s][^.]+)?(?:\.|$)")
def _put_any_number_target_cards(m):
    return UnknownEffect(raw_text=f"put any number target {m.group(1).strip()} cards")


# --- "put one of them onto the battlefield and shuffle the other into
# your library" ------------------------------------------------------------
# Cluster (5).
@_er(r"^put one of them (?:onto the battlefield|into your hand|into your graveyard)"
     r"(?:\s+and shuffle the (?:other|rest) into (?:your|their) library)?"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _put_one_of_them_v5(m):
    return UnknownEffect(raw_text="put one of them <zone>, shuffle other")


# --- "you may exile a nonland card from among them" ----------------------
# Cluster (4). Discover/Cascade-tail variant.
@_er(r"^you may exile (?:a|an|up to (?:one|two)) ([a-z ]+?) cards? from "
     r"(?:among them|the cards milled this way|exile)(?:[,\s][^.]+)?(?:\.|$)")
def _may_exile_from_among(m):
    return UnknownEffect(raw_text=f"may exile {m.group(1).strip()} card from among")


# --- "you may put a creature card with mana value N or less from among
# them onto the battlefield" -----------------------------------------------
# Cluster (4). Extends scrubber #2's _may_put_cards_tail to include the
# "with mana value N or less" qualifier explicitly so we don't depend on
# the broader regex's optional groups.
@_er(r"^you may put (?:a|an|up to (?:one|two)|any number of) ([a-z ]+?) cards? "
     r"with mana value (?:\d+|x) or less from "
     r"(?:among them|a graveyard|the cards milled this way|your hand|exile)"
     r"(?:\s+(?:onto the battlefield(?:\s+tapped(?:\s+and\s+[^.]+)?)?|"
     r"into your hand))?"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _may_put_with_mv(m):
    return UnknownEffect(raw_text=f"may put {m.group(1).strip()} with MV<=N")


# --- "you may reveal up to two creature cards from among them ..." -------
# Cluster (7). Extends scrubber #2's _may_reveal_tail.
@_er(r"^you may reveal (?:up to (?:one|two|three|\w+))? ?(?:a |an )?"
     r"([a-z ]+?) cards? from among them"
     r"(?:[,\s][^.]+|\s+and (?:put|exile) [^.]+)?(?:\.|$)")
def _may_reveal_among_them(m):
    return UnknownEffect(raw_text=f"may reveal {m.group(1).strip()} cards from among")


# --- "you may put any number of artifact cards with mana value x or less
# from among them onto the battlefield" -----------------------------------
# Cluster (5). Reveal-and-mass-put.
@_er(r"^you may put any number of ([a-z ]+?) cards "
     r"(?:with mana value [^,]+ )?from among them onto the battlefield"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _may_put_any_num_among(m):
    return UnknownEffect(raw_text=f"may put any number {m.group(1).strip()} cards")


# --- "you may put that card onto the battlefield. then shuffle" ----------
# Cluster (5). Tutor-tail with "then shuffle".
@_er(r"^you may put that card onto the battlefield"
     r"(?:\s+(?:tapped|under (?:your|their) control))?\.?\s*"
     r"then shuffle\s*$")
def _may_put_then_shuffle(m):
    return UnknownEffect(raw_text="may put that card BF, then shuffle")


# --- "the damage can't be prevented" --------------------------------------
# Cluster (4). Scrubber #1 had "this turn"; this is the bare form.
@_er(r"^(?:the |this )?damage can'?t be prevented(?: this turn)?\s*$")
def _damage_no_prevent_v5(m):
    return UnknownEffect(raw_text="damage can't be prevented")


# --- "excess damage is dealt to that creature's controller instead" ------
# Cluster (4). Trample-replacement on damage rider.
@_er(r"^excess damage is dealt to that creature'?s? controller instead"
     r"(?:\.|$)")
def _excess_dmg_to_controller(m):
    return UnknownEffect(raw_text="excess damage to controller")


# --- "mana of any type can be spent to cast that spell" ------------------
# Cluster (4 — same as the static, but reaches via effect path on some
# cards because the splitter classifies it differently).
@_er(r"^mana of any type can be spent to cast "
     r"(?:that spell|those spells|spells [^.]+?)(?:\.|$)")
def _mana_any_type_to_cast(m):
    return UnknownEffect(raw_text="mana of any type to cast")


# --- "the token enters tapped and attacking" ------------------------------
# Cluster (4). Token rider on creation.
@_er(r"^the tokens? enters? tapped"
     r"(?: and attacking)?(?:[,\s][^.]+)?(?:\.|$)")
def _token_enters_tapped_attacking(m):
    return UnknownEffect(raw_text="token enters tapped/attacking")


# --- "craft with one or more <X> {cost}" ----------------------------------
# Cluster (4). Craft is a transform-keyword (LCI). Bare-line shape.
@_er(r"^craft with (?:one or more |any |[^,]+? )?[a-z ]+? "
     r"\{[^}]+\}(?:\{[^}]+\})*\s*$")
def _craft_keyword(m):
    return UnknownEffect(raw_text="craft keyword")


# --- "until end of turn, that creature's controller may play that card and
# they may spend mana as though it were mana of any color" ----------------
# Cluster (7). Long umbrella the splitter hands as a bare effect line.
@_er(r"^until end of turn, that creature'?s controller may play that card"
     r"(?:[,\s][^.]+|\s+and they may spend mana[^.]+)?(?:\.|$)")
def _eot_controller_may_play(m):
    return UnknownEffect(raw_text="until eot, that creature's controller may play")


# --- "until end of turn, it has base power and toughness X/Y and gains
# <kws>" -------------------------------------------------------------------
# Cluster (4). Becomes-a-creature transform rider.
@_er(r"^until end of turn, it has base power and toughness "
     r"(?:\d+|x)/(?:\d+|x)(?:\s+and (?:gains?|has) [a-z, ]+)?(?:\.|$)")
def _eot_base_pt(m):
    return UnknownEffect(raw_text="until eot, base P/T and gains kws")


# --- "until end of turn, whenever target creature deals damage, you gain
# that much life" ----------------------------------------------------------
# Cluster (5). Lifelink-rider tail.
@_er(r"^until end of turn, whenever target creature deals damage, "
     r"you gain that much life(?:\.|$)")
def _eot_target_lifelink(m):
    return UnknownEffect(raw_text="until eot, lifelink-rider on target")


# --- "you and that player each sacrifice a creature" ---------------------
# Cluster (5). Symmetric edict.
@_er(r"^you and that player each sacrifice (?:a|an) "
     r"(creature|artifact|enchantment|land|permanent)\s*$")
def _symmetric_sac(m):
    return UnknownEffect(raw_text=f"you and that player each sac {m.group(1)}")


# --- "each opponent attacking that player does the same" -----------------
# Cluster (4). Curse-of-Disturbance-tail symmetric replay.
@_er(r"^each opponent attacking that player does the same(?:\.|$)")
def _each_opp_does_same(m):
    return UnknownEffect(raw_text="each opp attacking that player does the same")


# --- "you may choose new targets for the additional copy" ----------------
# Cluster (5). Twincast-tail.
@_er(r"^you may choose new targets for "
     r"(?:the additional copy|each copy|that copy|those copies)\s*$")
def _new_targets_for_copy(m):
    return UnknownEffect(raw_text="may choose new targets for copy")


# --- "you may cast it/this card for as long as it remains exiled" --------
# Cluster (7). Suspend-resolve / Adventure-tail permission.
@_er(r"^you may cast (?:it|this card|that card|those cards|the exiled cards?)"
     r" for as long as (?:it remains|they remain) exiled(?:[,\s][^.]+)?(?:\.|$)")
def _may_cast_while_exiled(m):
    return UnknownEffect(raw_text="may cast while exiled")


# --- "you may cast that card without paying its mana cost. then that
# player puts the exiled cards that weren't cast this way on the bottom
# of their library" -------------------------------------------------------
# Cluster (6). Long Chaos-Wand-tail.
@_er(r"^you may cast that card without paying its mana cost\.?\s*"
     r"then that player puts the exiled cards [^.]+(?:\.|$)")
def _chaos_wand_long_tail(m):
    return UnknownEffect(raw_text="may cast WPM, then put exiled to bottom")


# --- "this creature gets +1/+1 until end of turn for each creature
# tapped this way" --------------------------------------------------------
# Cluster (3). Already partially covered above as static; this catches the
# effect-path version.
@_er(r"^(?:this creature|~) gets \+(\d+)/\+(\d+) until end of turn "
     r"for each [^.]+?(?:\.|$)")
def _self_buff_eot_foreach_eff(m):
    p, t = int(m.group(1)), int(m.group(2))
    return Buff(power=p, toughness=t,
                target=Filter(base="self", targeted=False))


# --- "you choose an artifact or creature card from it" -------------------
# Cluster (5+4). Scrubber #1 has the bare "you choose a card from it"; the
# multi-type variant slips past.
@_er(r"^you choose an? ([a-z, ]+? (?:or|and) [a-z, ]+?) cards? from it"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _you_choose_multitype(m):
    return UnknownEffect(raw_text=f"you choose {m.group(1).strip()} card from it")


# --- "you may put a card from your hand on the bottom of your library
# in a random order" ------------------------------------------------------
# Cluster (3). Wheel-of-fate / random-order tail.
@_er(r"^put the rest (?:of the cards )?on the bottom (?:of (?:your|their) library)?"
     r"(?: in a random order)?(?:\.|$)")
def _put_rest_bottom(m):
    return UnknownEffect(raw_text="put the rest on the bottom (random)")


# --- "regenerate target <X>" / "regenerate <X>" --------------------------
# Cluster: scrubber-cheap. Base parser doesn't have a regenerate effect.
@_er(r"^regenerate (?:target |that |this )?(creature|artifact|land|permanent|"
     r"enchantment|~)(?:\.|$)")
def _regenerate_target(m):
    return UnknownEffect(raw_text=f"regenerate {m.group(1)}")


# --- "this creature has all activated abilities of all <X>" --------------
# Cluster (4). Patchwork Crawler — also as effect.
@_er(r"^(?:this creature|~) has all activated abilities of all [^.]+?"
     r"(?:\.|$)")
def _has_all_activated_eff(m):
    return UnknownEffect(raw_text="has all activated abilities")


# --- Player-rules effect form: "players have no maximum hand size" -------
@_er(r"^(?:players|your opponents|each player|each opponent) have no "
     r"maximum hand size(?:\.|$)")
def _no_max_hand_others_eff(m):
    return UnknownEffect(raw_text="players have no maximum hand size")


# --- "the player puts that card onto the battlefield, then shuffles ..." -
# Cluster (4). Tutor-resolution-tail leaking as a separate ability.
@_er(r"^(?:the |that )?player puts that card onto the battlefield"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _the_player_puts_bf(m):
    return UnknownEffect(raw_text="the player puts that card BF, then shuffles")


# --- "you may play one of those cards this turn" -------------------------
# Cluster (3). Discover/Cascade-tail.
@_er(r"^you may play one of those cards"
     r"(?:[,\s][^.]+|\s+(?:this turn|until [^.]+))?(?:\.|$)")
def _may_play_one_of_those(m):
    return UnknownEffect(raw_text="may play one of those cards")


# --- "you may cast it this turn, and mana of any type can be spent ..." --
# Cluster (4).
@_er(r"^you may cast it this turn(?:[,\s][^.]+)?(?:\.|$)")
def _may_cast_it_this_turn(m):
    return UnknownEffect(raw_text="may cast it this turn")


# --- "you may cast spells from among those cards for as long as they
# remain exiled, and mana of any type can be spent to cast those spells" --
# Cluster (5). Etali-tail / Aminatou's Augury-tail.
@_er(r"^you may cast spells from "
     r"(?:among those cards|those exiled cards|exile)"
     r" for as long as they remain exiled(?:[,\s][^.]+)?(?:\.|$)")
def _may_cast_from_among_while_exiled(m):
    return UnknownEffect(raw_text="may cast from among, while exiled")


# --- "for as long as that creature has a bounty counter on it, ..." ------
# Cluster (3). Bounty-rider attached-ability tail.
@_er(r"^for as long as that (?:creature|card|permanent) has [^,.]+ counter on it,"
     r" [^.]+?(?:\.|$)")
def _while_counter_on_it(m):
    return UnknownEffect(raw_text="while X-counter on it, ...")


# --- "search your library for up to X basic land cards, where X is ..." --
# Cluster (5). Variable-tutor.
@_er(r"^search your library for up to x ([a-z ]+? cards?)"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _search_up_to_x(m):
    return UnknownEffect(raw_text=f"search lib for up to X {m.group(1).strip()}")


# --- "discover N" ---------------------------------------------------------
# Cluster (~3 bare forms). LCI mechanic.
@_er(r"^discover (\d+|x)\s*$")
def _discover_n(m):
    return UnknownEffect(raw_text=f"discover {m.group(1)}")


# --- "exile this saga, then return it to the battlefield ..." ------------
# Cluster (5). MOM saga-flip rider.
@_er(r"^exile this (?:saga|enchantment|creature),?\s*"
     r"then return it to the battlefield(?:[,\s][^.]+)?(?:\.|$)")
def _exile_then_return_self(m):
    return UnknownEffect(raw_text="exile this, return it to BF")


# --- "an opponent separates those cards into two piles" ------------------
# Cluster (5). Fact or Fiction-tail (parser handles the FoF setup but the
# trailing "an opponent separates ..." sentence is its own ability line).
@_er(r"^an opponent separates those cards into two piles\s*$")
def _opp_separates_piles(m):
    return UnknownEffect(raw_text="opp separates piles")


# --- "reveal the top five cards of your library and separate them into
# two piles" ---------------------------------------------------------------
# Cluster (3). FoF setup-line.
@_er(r"^reveal the top (?:\d+|x|\w+) cards of your library"
     r"(?:[,\s][^.]+|\s+and separate them into two piles)?(?:\.|$)")
def _reveal_top_n_separate(m):
    return UnknownEffect(raw_text="reveal top N + separate")


# --- "put that pile into your hand and the other into your graveyard" ----
# Cluster (7). FoF resolution.
@_er(r"^put that pile into (?:your|their) (hand|graveyard|library)"
     r"(?:\s+and the other into (?:your|their) (?:hand|graveyard|library))?"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _put_that_pile(m):
    return UnknownEffect(raw_text=f"put that pile -> {m.group(1)}")


# --- "you draw x cards and you lose x life" ------------------------------
# Cluster (6). Skeletal Scrying / Necropotence-tail.
@_er(r"^you draw x cards and you lose x life(?:[,\s][^.]+)?(?:\.|$)")
def _draw_x_lose_x(m):
    return UnknownEffect(raw_text="draw X, lose X life")


# --- "choose two target creatures controlled by the same opponent" -------
# Cluster (6). Specialized-target Edict / Trial of Agony.
@_er(r"^choose (?:two|three|\w+) target creatures controlled by "
     r"(?:the same opponent|different players|the same player)\s*$")
def _choose_targets_constraint(m):
    return UnknownEffect(raw_text="choose targets controlled by ...")


# --- "choose target creature you control and target creature you don't
# control" ----------------------------------------------------------------
# Cluster (5). Bite-spell pair-target setup.
@_er(r"^choose target creature you control and target creature "
     r"(?:you don'?t control|an opponent controls)(?:\.|$)")
def _choose_pair_targets(m):
    return UnknownEffect(raw_text="choose pair: ally + opp creature")


# --- "the tokens are goaded for the rest of the game" --------------------
# Cluster (3). Goad-rider on token creation.
@_er(r"^(?:the |those )?tokens are goaded(?: for the rest of the game)?"
     r"(?:\.|$)")
def _tokens_goaded_eff(m):
    return UnknownEffect(raw_text="tokens are goaded")


# --- "convert ~" / "time travel" / "paradigm" — bare keyword as effect --
# These reach the effect path on some Transformer/CLB cards.
@_er(r"^(convert ~|time travel|paradigm)\s*$")
def _bare_keyword_as_effect(m):
    return UnknownEffect(raw_text=f"keyword: {m.group(1)}")


# --- "creatures target player controls get +N/+N until end of turn" ------
# Cluster (5). Goblin War Drums-class.
@_er(r"^creatures target player controls get \+(\d+)/\+(\d+)"
     r"(?: until end of turn)?(?:\.|$)")
def _opp_target_anthem_eff(m):
    p, t = int(m.group(1)), int(m.group(2))
    return Buff(power=p, toughness=t,
                target=Filter(base="creatures_target_player_controls",
                              targeted=True))


# --- "each creature you control with a +1/+1 counter on it deals damage
# equal to its power to that creature" ------------------------------------
# Cluster (4). Witnessed-fight rider.
@_er(r"^each creature you control with [^,]+ counter on it deals damage "
     r"equal to its power to (?:that creature|target creature|each [^.]+)"
     r"(?:\.|$)")
def _ally_with_counter_deals(m):
    return UnknownEffect(raw_text="each ally with counter deals damage")


# --- "each of those creatures deals damage equal to its power to <X>" ----
# Cluster (4). Mass-fight tail.
@_er(r"^each of those creatures deals damage equal to its power to [^.]+?"
     r"(?:\.|$)")
def _each_those_deals(m):
    return UnknownEffect(raw_text="each of those creatures deals dmg=power")


# --- "draw a card for each <X> in it" ------------------------------------
# Cluster (5). Mountains-and-red-cards tail.
@_er(r"^you draw a card for each [^.]+? in it(?:\.|$)")
def _draw_for_each_in_it(m):
    return UnknownEffect(raw_text="draw a card for each X in it")


# --- "x is the number of cards in an opponent's hand" --------------------
# Cluster (5+ spillover). Bare definition-of-X clause.
@_er(r"^x is the (?:number|amount|mana value) of [^.]+?(?:\.|$)")
def _x_definition(m):
    return UnknownEffect(raw_text="x is the number/amount of ...")


# --- "you may play them until the end of your next turn" -----------------
# Cluster (3). Cascade-tail extended-window.
@_er(r"^you may play them until the end of your next turn(?:[,\s][^.]+)?(?:\.|$)")
def _play_them_until_next(m):
    return UnknownEffect(raw_text="may play them until next end")


# --- "exile one of them face down and put the rest on the bottom of your
# library in a random order" ----------------------------------------------
# Cluster (3). Goblin Recruiter-tail / Treasure Hunt-tail.
@_er(r"^exile one of them face down(?:[,\s][^.]+)?(?:\.|$)")
def _exile_one_facedown(m):
    return UnknownEffect(raw_text="exile one face down")


# --- "manifest the top card of your library and attach this enchantment
# to it" -----------------------------------------------------------------
# Cluster (3). Manifest-and-attach.
@_er(r"^manifest the top card of your library"
     r"(?:[,\s][^.]+|\s+and attach [^.]+)?(?:\.|$)")
def _manifest_and_attach(m):
    return UnknownEffect(raw_text="manifest top + attach")


# --- "exchange control of two target creatures controlled by different
# players" ----------------------------------------------------------------
# Cluster (3). Donate-symmetric.
@_er(r"^exchange control of (?:two|\w+) target [a-z ]+? controlled by [^.]+?"
     r"(?:\.|$)")
def _exchange_control(m):
    return UnknownEffect(raw_text="exchange control of targets")


# --- "target opponent gains control of this creature" --------------------
# Cluster (3). Donate-self.
@_er(r"^(?:target opponent|an opponent) gains control of this "
     r"(?:creature|artifact|enchantment|permanent|land)(?:\.|$)")
def _opp_gains_control_self(m):
    return UnknownEffect(raw_text="opp gains control of this perm")


# --- "those creatures gain flying until end of turn" -- handled above ---
# (Subsumed by _those_creatures_gain.)


# --- "until end of turn, target creature gains <kw> and <rider>" ---------
# Cluster (~3 residual after scrubber #1's covers).
@_er(r"^until end of turn, target creature gains ([a-z, ]+?)"
     r"(?: and [^.]+)?(?:\.|$)")
def _eot_target_gains(m):
    return GrantAbility(ability_name=m.group(1).strip(), target=TARGET_CREATURE)


# --- "+ {cost} - <effect>" — Spree mode line as effect path ------------
# Same as the static handler but spliced into EFFECT_RULES so the splitter's
# alternate categorisation also gets a hit.
@_er(r"^\+\s*\{[^}]+\}(?:\{[^}]+\})*\s*[-–—]\s*(.+?)(?:\.|$)")
def _spree_mode_eff(m):
    body = m.group(1).strip()
    try:
        import parser as _p
        inner = _p.parse_effect(body)
        if inner is not None:
            return inner
    except Exception:
        pass
    return UnknownEffect(raw_text=f"spree-mode: {body}")


# --- "+ {N} -" loyalty-style headers without a body (orphan). -----------
# Cluster (small). Spree mode where the body landed on the next line.
@_er(r"^[+-]\s*\{[^}]+\}(?:\{[^}]+\})*\s*[-–—]?\s*$")
def _spree_header_orphan(m):
    return UnknownEffect(raw_text="spree-mode header (orphan)")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # --- Type-to-graveyard ally trigger (type qualifier) ----------------
    # Cluster (4 enchantment + smaller artifact/creature). Scrubber #2 had
    # ``whenever ~ is put into a graveyard from anywhere`` but not the
    # ``whenever an enchantment you control is put into a graveyard from
    # the battlefield`` form.
    (re.compile(r"^whenever an? (?:artifact|enchantment|creature|land|"
                r"planeswalker|permanent) you control is put into a "
                r"graveyard from the battlefield", re.I),
     "ally_type_to_gy_from_bf", "self"),

    # --- Type-to-graveyard opp-side ------------------------------------
    # Cluster (4 artifact). Kibo / Pain Distributor.
    (re.compile(r"^whenever an? (?:artifact|enchantment|creature|land|"
                r"planeswalker|permanent) an opponent controls is put into "
                r"a graveyard from the battlefield", re.I),
     "opp_type_to_gy_from_bf", "opp"),

    # --- "whenever a player sacrifices a permanent" --------------------
    # Cluster (4). Symmetric sac trigger.
    (re.compile(r"^whenever a player sacrifices a (?:permanent|creature|"
                r"artifact|enchantment|land|token)", re.I),
     "any_player_sacs", "self"),

    # --- "whenever you sacrifice one or more <type>s" ------------------
    # Cluster (4). Food/Clue/Treasure/creatures sacrifice trigger.
    (re.compile(r"^whenever you sacrifice one or more "
                r"(?:foods|clues|treasures|creatures|artifacts|enchantments|"
                r"permanents|tokens|lands)", re.I),
     "you_sac_one_or_more", "self"),

    # --- "whenever a token you control leaves the battlefield" ---------
    # Cluster (5). Token-die-or-flicker trigger.
    (re.compile(r"^whenever a token you control "
                r"(?:leaves the battlefield|dies|enters)", re.I),
     "token_event", "self"),

    # --- "whenever another <subtype> you control enters" ---------------
    # Cluster (5 shrine + 4 human + scattered ally subtypes). Scrubber
    # #2 covered Permanent-types; this is creature-subtype.
    (re.compile(r"^whenever another (?:shrine|human|elf|goblin|zombie|"
                r"vampire|merfolk|wizard|warrior|knight|soldier|cleric|"
                r"dragon|angel|demon|dwarf|spirit|sliver|cat|bird|beast|"
                r"sphinx|hydra|saproling|treefolk|insect|fish|eldrazi|"
                r"phyrexian|rogue|pirate|samurai|ninja|monk|druid|kithkin|"
                r"shaman|berserker|kor|kavu|advisor|noble|assassin|scout|"
                r"horror|construct|golem|illusion|elemental) you control "
                r"enters", re.I),
     "another_subtype_enters", "self"),

    # --- "whenever you win a coin flip" ---------------------------------
    # Cluster (4). Krark-flavor trigger.
    (re.compile(r"^whenever you (?:win|lose) a (?:coin )?flip", re.I),
     "coin_flip_result", "self"),

    # --- "whenever you roll one or more dice" --------------------------
    # Cluster (4). CLB dice-matters trigger.
    (re.compile(r"^whenever you roll (?:one or more dice|a die|two or more dice|"
                r"a \d+|a die for [^,]+)", re.I),
     "you_roll_dice", "self"),

    # --- "whenever two or more creatures attack" -----------------------
    # Cluster (4). Crowd-attack trigger.
    (re.compile(r"^whenever (?:two or more creatures|three or more creatures|"
                r"\w+ or more creatures) attack", re.I),
     "n_or_more_attack", "self"),

    # --- "whenever a player taps a land for mana" ----------------------
    # Cluster (4). Scrubber #2 has ``you tap a land`` for self only;
    # broaden to any player.
    (re.compile(r"^whenever a player taps (?:a|an) [a-z ]*?land for mana",
                re.I), "any_player_tap_land", "self"),

    # --- "whenever an opponent draws their (second|third|...) card each
    # turn" --------------------------------------------------------------
    # Cluster (4). Faerie Mastermind-style draw-cap punisher.
    (re.compile(r"^whenever an opponent draws their (?:second|third|fourth|"
                r"fifth|first additional|next) card each turn", re.I),
     "opp_draws_ordinal", "opp"),

    # --- "whenever an opponent searches their library" -----------------
    # Cluster (3). Trinisphere/Aven Mindcensor-cousin trigger.
    (re.compile(r"^whenever an opponent searches their library", re.I),
     "opp_searches_library", "opp"),

    # --- "whenever you activate an exhaust ability that isn't a mana
    # ability" ----------------------------------------------------------
    # Cluster (5). New Foundations-era exhaust mechanic.
    (re.compile(r"^whenever you activate an exhaust ability"
                r"(?: that isn'?t a mana ability)?", re.I),
     "you_activate_exhaust", "self"),

    # --- "whenever you activate an ability that targets a creature or
    # player" -----------------------------------------------------------
    # Cluster (3). Riku-of-Two-Reflections-cousin / Strionic Resonator-trig.
    (re.compile(r"^whenever you activate an ability that targets "
                r"(?:a creature or player|a creature|a player|target [^,]+)",
                re.I), "you_activate_targeted_ability", "self"),

    # --- "whenever you expend N" ---------------------------------------
    # Cluster (3). Aetherdrift expend mechanic.
    (re.compile(r"^whenever you expend (?:\d+|x)", re.I),
     "you_expend_n", "self"),

    # --- "whenever you get one or more {e}" ----------------------------
    # Cluster (4). Energy-gain trigger.
    (re.compile(r"^whenever you get one or more \{e\}", re.I),
     "you_get_energy", "self"),

    # --- "whenever ~ and at least one other creature attack" -----------
    # Cluster (5). Pack-attacker / Anointer-Priest-tail trigger.
    (re.compile(r"^whenever ~ and at least one other "
                r"(?:creature|[a-z]+) attacks?", re.I),
     "self_and_others_attack", "self"),

    # --- "whenever a player loses the game" ----------------------------
    # Cluster (3). Game-end trigger.
    (re.compile(r"^whenever a player loses the game", re.I),
     "any_player_loses_game", "self"),

    # --- "whenever another nontoken creature dies" ---------------------
    # Cluster (3). Bare nontoken-die (no "you control" qualifier).
    (re.compile(r"^whenever another nontoken creature dies", re.I),
     "another_nontoken_creature_dies_any", "self"),

    # --- "whenever one or more nonland cards are milled" ---------------
    # Cluster (3). Mill-matters trigger.
    (re.compile(r"^whenever one or more (?:nonland |land |creature |"
                r"instant |sorcery |artifact )?cards? (?:are|is) milled",
                re.I), "one_or_more_milled", "self"),

    # --- "when a creature is put into an opponent's graveyard from the
    # battlefield" ------------------------------------------------------
    # Cluster (3). Reaper-cousin opp-sided creature-die trigger.
    (re.compile(r"^when (?:a|an) (?:creature|artifact|enchantment|permanent)"
                r" is put into an opponent'?s graveyard from the battlefield",
                re.I), "opp_creature_to_gy", "opp"),

    # --- "when you pay this cost one or more times" --------------------
    # Cluster (6). Adversary cycle (Primal Adversary etc.).
    (re.compile(r"^when you pay this cost one or more times", re.I),
     "pay_cost_multiple", "self"),

    # --- "whenever one or more creatures/permanents you control with
    # <X> enter" --------------------------------------------------------
    # Cluster (5+ residual). Crowd-enter with qualifier.
    (re.compile(r"^whenever one or more (?:other )?creatures? you control "
                r"with [^,.]+? enter", re.I),
     "one_or_more_ally_with_X_enter", "self"),

    # --- "whenever one or more <type> tokens your opponents control
    # enter" ------------------------------------------------------------
    # Cluster (4). Token-flicker symmetric trigger.
    (re.compile(r"^whenever one or more "
                r"(?:[a-z]+ )?tokens your opponents control "
                r"(?:enter|leave|die)", re.I),
     "opp_tokens_event", "opp"),

    # --- "whenever one or more artifacts/enchantments you control enter"
    # Cluster (5). Bare-type ally-enter (no "another").
    (re.compile(r"^whenever one or more (artifacts?|enchantments?|"
                r"creatures?|permanents?|lands?) you control enter", re.I),
     "one_or_more_type_ally_enter", "self"),

    # --- "whenever a face-down creature you control enters" ------------
    # Cluster (5). Manifest/Morph trigger.
    (re.compile(r"^whenever a face-down creature you control enters", re.I),
     "facedown_ally_enters", "self"),

    # --- "whenever you exert a creature" -------------------------------
    # Cluster (4). Amonkhet exert mechanic.
    (re.compile(r"^whenever you exert a creature", re.I),
     "you_exert_creature", "self"),

    # --- "whenever an enchantment you control becomes the target" ------
    # Cluster (~3). Aura-targeted trigger.
    (re.compile(r"^whenever an? (?:aura|enchantment|artifact) you control "
                r"becomes the target", re.I),
     "ally_perm_targeted", "self"),

    # --- "whenever one or more nonland cards leave your graveyard" -----
    # Cluster (~3). Reverse-mill trigger.
    (re.compile(r"^whenever one or more (?:nonland |creature |land |"
                r"instant |sorcery |artifact )?cards? leaves? "
                r"(?:your|a) graveyard", re.I),
     "cards_leave_gy", "self"),

    # --- "whenever you play a card with two or more card types" --------
    # Cluster (3). Multi-type-matters.
    (re.compile(r"^whenever you (?:play|cast) a (?:card|spell) with "
                r"(?:two or more|three or more|\w+ or more) (?:card )?types",
                re.I), "you_play_multi_type", "self"),

    # --- "whenever enchanted player casts a spell other than the first"
    # Cluster (3). Curse-of-Exhaustion-tail trigger.
    (re.compile(r"^whenever enchanted player casts a spell"
                r"(?: other than the first[^,]*)?", re.I),
     "enchanted_player_casts", "self"),

    # --- "when this creature regenerates this way" ---------------------
    # Cluster (3). Old-bordered regenerate-rider trigger.
    (re.compile(r"^when (?:this creature|~|it) regenerates(?: this way)?",
                re.I), "self_regenerates", "self"),
]
