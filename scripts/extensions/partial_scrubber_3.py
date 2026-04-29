#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (third pass).

Family: PARTIAL → GREEN promotions. Companion to ``partial_scrubber.py`` and
``partial_scrubber_2.py``; targets single-ability clusters that survived
both prior passes. Patterns were picked by re-bucketing PARTIAL
parse_errors by first-3/5/8 words after scrubbers #1 and #2 shipped and
keeping clusters of ≥5 hits that map cleanly onto static regex.

Same three export tables as the prior scrubbers.

- ``STATIC_PATTERNS``   — static/anthem shapes the base ``parse_static``
  missed: ``creatures you control gain <kw>``, ``creatures your opponents
  control get -N/-N``, ``creature tokens you control have <kw>``,
  ``activated abilities of <X> cost {N} less/more to activate``,
  ``~ must be blocked if able``, ``this creature can block any number``,
  ``all creatures able to block X do so``, ``this ability costs ...``,
  evergreen-keyword bare forms (``afterlife N``, ``warp {cost}``),
  ability-word riders (``adamant - ...``), and a handful of Platinum
  Angel-class "you don't lose the game" sentences.

- ``EFFECT_RULES``     — body-level shapes the splitter hands us as
  standalone effect lines. Biggest clusters: ``target creature you control
  gets +N/+N [and gains KW] until end of turn`` (28), ``its controller
  creates a <token>`` (19), ``sacrifice/exile that token [at EOC]`` (19),
  ``each opponent sacrifices a creature of their choice`` (11),
  ``return up to two target creatures to their owner's hand`` (8),
  ``up to two target creatures each get +N/+N`` (8), ``roll a d20`` (10),
  ``target opponent sacrifices...`` (7), ``you skip your next turn`` (7),
  ``return those cards to the battlefield`` (7), ``fight tail``,
  ``choose one of them``, ``put one [pile] into your hand and the other
  into your graveyard``, plus bare P/T tails (``4/4``, ``6/6``) the
  splitter leaves dangling from token-creation sentences.

- ``TRIGGER_PATTERNS`` — new trigger shapes: ``when you unlock this door``
  (20), ``when this creature specializes`` (27), ``when this card
  specializes from your graveyard/any zone`` (10), ``whenever this
  creature mutates`` (19), ``whenever equipped creature dies`` (17),
  ``whenever you commit a crime`` (16), ``when this creature blocks`` (6).

Ordering: specific-first. Cluster counts come from the dry-run analysis
embedded in scrubber #1's docstring (same methodology).
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
    Buff, Fight, Filter, GrantAbility, Keyword,
    Modification, Sequence, Static, UnknownEffect,
    TARGET_CREATURE,
)


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — static/anthem shapes missed by parse_static
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "creatures you control gain <kw> [until end of turn]" -----------------
# Cluster (21). Base parse_static has "creatures you control get +N/+N|have
# <kw>" but no "gain" variant, which is the phrasing used by a lot of
# instants (Rally-the-Peasants-style), continuation sentences, and the
# "gain that ability" forwarder.
@_sp(r"^creatures you control gain ([^.]+?)(?: until end of turn)?\s*$")
def _ally_creatures_gain(m, raw):
    return Static(modification=Modification(
        kind="team_grant_tmp", args=(m.group(1).strip(),)), raw=raw)


# --- "creature tokens you control (get +N/+N | have <kw>) [and ...]" --------
# Cluster (15). Token-anthem static flavor from Anointed Procession-class or
# tribal-token commanders. Base parse_static only matches all-creatures
# anthem without the "tokens" qualifier.
@_sp(r"^creature tokens you control "
     r"(?:get \+(\d+)/\+(\d+)(?: and have ([^.]+?))?|have ([^.]+?))"
     r"\s*$")
def _token_anthem(m, raw):
    if m.group(1):
        p, t = int(m.group(1)), int(m.group(2))
        extra = m.group(3)
        return Static(modification=Modification(
            kind="token_anthem",
            args=(p, t, (extra or "").strip() or None)), raw=raw)
    return Static(modification=Modification(
        kind="token_keyword_grant", args=(m.group(4).strip(),)), raw=raw)


# --- "creatures your opponents control get -N/-N [until end of turn]" ------
# Cluster (14). Reverse-anthem (Curse of the Pierced Heart rider /
# Breathkeeper Seraph / Vraska's Contempt follow-on).
@_sp(r"^creatures your opponents control get -(\d+)/-(\d+)(?: until end of turn)?\s*$")
def _opp_reverse_anthem(m, raw):
    return Static(modification=Modification(
        kind="opp_reverse_anthem",
        args=(-int(m.group(1)), -int(m.group(2)))), raw=raw)


# --- "creatures your opponents control enter tapped" -----------------------
# Cluster (3). Thalia's Lancers-tail / Kismet-tail static.
@_sp(r"^creatures your opponents control enter tapped\s*$")
def _opp_etb_tapped(m, raw):
    return Static(modification=Modification(kind="opp_creatures_etb_tapped"),
                  raw=raw)


# --- "creatures your opponents control have base toughness N" --------------
# Cluster (2) but cheap catch. Humility-Elspeth Conquers Death rider.
@_sp(r"^creatures your opponents control have base (power|toughness) (\d+)\s*$")
def _opp_base_stat(m, raw):
    return Static(modification=Modification(
        kind="opp_base_stat",
        args=(m.group(1), int(m.group(2)))), raw=raw)


# --- "activated abilities of <X> cost {N} (less|more) to activate" ---------
# Cluster (10). Training Grounds-style cost mod for a qualifier other than
# "creatures you control" (which the base parser's cost_reduction does
# handle). We don't unpack the filter — it's a static-effect raw stash.
@_sp(r"^activated abilities of ([^.]+?) cost \{([^}]+)\} (less|more) to activate"
     r"(?: if [^.]+)?\s*$")
def _activated_cost_mod(m, raw):
    return Static(modification=Modification(
        kind="activated_cost_mod",
        args=(m.group(1).strip(), m.group(2), m.group(3))), raw=raw)


# --- "activated abilities of <X> can't be activated [unless...]" -----------
# Cluster (5). Linvala / Pithing Needle-class lockdown.
@_sp(r"^activated abilities of ([^.]+?) can'?t be activated(?:\s+unless[^.]+)?\s*$")
def _activated_cant(m, raw):
    return Static(modification=Modification(
        kind="activated_cant",
        args=(m.group(1).strip(),)), raw=raw)


# --- "this ability costs {N|X} (less|more) to activate [if/for each ...]" --
# Cluster (16). Sting the activation of the card's OWN ability — shows up
# as a stand-alone ability line ("this ability costs {2} less to activate
# if..."). Not to be confused with the cost-reduction for spells the base
# parser handles.
@_sp(r"^this ability costs \{([^}]+)\} (less|more) to activate"
     r"(?:[,\s][^.]+)?\s*$")
def _this_ability_cost_mod(m, raw):
    return Static(modification=Modification(
        kind="this_ability_cost_mod",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "this ability also triggers if ~ is in [your graveyard|...]" ----------
# Cluster (2). Replicating Ring / Extra-trigger-zone rider (niche but cheap).
@_sp(r"^this ability also triggers if (?:~|this [a-z]+) is in (?:your |the )?([a-z ]+)\s*$")
def _ability_also_triggers(m, raw):
    return Static(modification=Modification(
        kind="ability_also_triggers_in",
        args=(m.group(1).strip(),)), raw=raw)


# --- "all creatures able to block <X> do so" --------------------------------
# Cluster (12). Lure / Provoke rider. Must-block-all static.
@_sp(r"^all creatures able to block "
     r"(?:this creature|enchanted creature|equipped creature|~|target creature this turn)"
     r" do so\s*$")
def _all_must_block(m, raw):
    return Static(modification=Modification(kind="all_must_block"), raw=raw)


# --- "~ must be blocked (by two or more creatures)? if able" ---------------
# Cluster (6). Provoke-ish lure-self. Base parser's "must_be_blocked"
# handles "this creature must be blocked if able" but not the ~-rooted form.
@_sp(r"^~ must be blocked(?: by two or more creatures)?(?:,? and ~[^.]+)? if able\s*$")
def _tilde_must_be_blocked(m, raw):
    return Static(modification=Modification(kind="must_be_blocked_self"),
                  raw=raw)


# --- "this creature can block any number of creatures" ---------------------
# Cluster (5). Two-Headed Giant / Wall of Blossoms flavor static.
@_sp(r"^this creature can block any number of creatures\s*$")
def _can_block_any(m, raw):
    return Static(modification=Modification(kind="can_block_any_number"),
                  raw=raw)


# --- "you don't lose the game for having 0 or less life" -------------------
# Cluster (5). Platinum Angel / Phyrexian Unlife family.
@_sp(r"^you don'?t lose the game(?: for having 0 or less life)?\s*$")
def _no_lose_zero(m, raw):
    return Static(modification=Modification(kind="no_lose_zero_life"),
                  raw=raw)


# --- "your opponents can't win the game" -----------------------------------
# Cluster (cheap). Platinum Angel B-side.
@_sp(r"^your opponents can'?t win the game\s*$")
def _opp_cant_win(m, raw):
    return Static(modification=Modification(kind="opponents_cant_win"),
                  raw=raw)


# --- "each creature you control assigns combat damage equal to its toughness ..."
# Cluster (4). Doran / Assault Formation-family static.
@_sp(r"^each creature you control assigns combat damage equal to its toughness"
     r" rather than its power\s*$")
def _doran_static(m, raw):
    return Static(modification=Modification(kind="damage_uses_toughness"),
                  raw=raw)


# --- "during your turn, <rider>" -------------------------------------------
# Cluster (~8-10 leftover). Various "during your turn, X" statics. Very
# broad, so we only claim shapes with the pump/keyword rider — effect
# rules handle "during your turn, you may ...".
@_sp(r"^during your turn, (?:equipped creature|this creature|~|creature tokens you control)"
     r" (?:gets \+\d+/[+\-]\d+|has [a-z, ]+?|gains [a-z, ]+?|get \+\d+/[+\-]\d+|have [a-z, ]+?)"
     r"(?:[,\s]+(?:and [^.]+|has [^.]+))?\s*$")
def _during_your_turn_rider(m, raw):
    return Static(modification=Modification(kind="during_your_turn_rider"),
                  raw=raw)


# --- "spells you cast <rider> cost {N} less to cast" -----------------------
# Cluster (~7 residual). Base parser has a cost_reduction rule but it misses
# shapes with a long qualifier ("from your graveyard", "this way", "but
# don't own", etc.).
@_sp(r"^spells you cast (?:from [^,.]+|this way|but don'?t own|that target [^.]+|with [^.]+)"
     r" cost \{(\d+)\} (less|more) to cast\s*$")
def _spells_you_cast_qualified(m, raw):
    return Static(modification=Modification(
        kind="qualified_spell_cost_mod",
        args=(int(m.group(1)), m.group(2))), raw=raw)


# --- "your opponents can't cast <X>" ---------------------------------------
# Cluster (13). Silence / Grand Abolisher-family lockdown. Base has no such
# rule for "your opponents can't cast ...".
@_sp(r"^your opponents can'?t cast ([^.]+?)(?:\.|$)")
def _opp_cant_cast(m, raw):
    return Static(modification=Modification(
        kind="opp_cant_cast", args=(m.group(1).strip(),)), raw=raw)


# --- "your opponents can't play land cards from graveyards" ----------------
@_sp(r"^your opponents can'?t play land cards from (?:graveyards?|exile)\s*$")
def _opp_cant_play_land(m, raw):
    return Static(modification=Modification(kind="opp_cant_play_land_from_zone"),
                  raw=raw)


# --- Adamant / Fateful hour etc. ability-word riders (scrubber #2 added
# several, but ADAMANT was missed) ------------------------------------------
@_sp(r"^(adamant|secret council|kinship|join forces|tempting offer|strive|"
     r"heroic|grandeur|radiance|imprint)\s*[-–—]\s*(.+?)\s*$")
def _more_ability_word_riders(m, raw):
    return Static(modification=Modification(
        kind="ability_word_rider",
        args=(m.group(1).strip(), m.group(2).strip())), raw=raw)


# --- Afterlife N (bare keyword — base regex only has cost-form keywords) ---
# Cluster (8). Afterlife is a *numeric* keyword the base KEYWORD_RE doesn't
# list at all.
@_sp(r"^afterlife (\d+)\s*$")
def _afterlife(m, raw):
    return Keyword(name="afterlife", args=(int(m.group(1)),), raw=raw)


# --- Warp {cost} (MH3 / Foundations-ish mana keyword) ----------------------
# Cluster (32). Big win: base KEYWORD_RE doesn't know about warp.
@_sp(r"^warp (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _warp(m, raw):
    return Keyword(name="warp", args=(m.group(1),), raw=raw)


# --- Saddle N (Outlaws of Thunder Junction) --------------------------------
@_sp(r"^saddle (\d+)\s*$")
def _saddle(m, raw):
    return Keyword(name="saddle", args=(int(m.group(1)),), raw=raw)


# --- Freerunning {cost} (Thunder Junction) ---------------------------------
@_sp(r"^freerunning (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _freerunning(m, raw):
    return Keyword(name="freerunning", args=(m.group(1),), raw=raw)


# --- Spree (Thunder Junction modal cost keyword, bare-name form) -----------
@_sp(r"^spree\s*$")
def _spree(m, raw):
    return Keyword(name="spree", raw=raw)


# --- Crime / Bargain bare-word ability-words --------------------------------
@_sp(r"^(bargain|crime)\s*$")
def _bare_ability_word(m, raw):
    return Keyword(name=m.group(1).lower(), raw=raw)


# --- Intensify N (Alchemy/digital) -----------------------------------------
@_sp(r"^(?:all )?(\w+) cards you own intensify by (\d+)\s*$")
def _intensify(m, raw):
    return Static(modification=Modification(
        kind="intensify",
        args=(m.group(1).lower(), int(m.group(2)))), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES — body-level effect shapes
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "target creature you control gets +N/+N [and gains KW] until EOT" -----
# Cluster (28). Scrubber #1 handled "target creature gets ..." but not the
# "you control" specifier.
@_er(r"^target creature you control gets ([+-]\d+)/([+-]\d+)"
     r"(?: and gains ([a-z, ]+?))?"
     r" until end of turn(?:\.|$)")
def _ally_tgt_buff(m):
    p, t = int(m.group(1)), int(m.group(2))
    filt = Filter(base="creature", you_control=True, targeted=True)
    buff = Buff(power=p, toughness=t, target=filt)
    if m.group(3):
        return Sequence(items=(buff,
                               GrantAbility(ability_name=m.group(3).strip(),
                                            target=filt)))
    return buff


# --- "target creature you control gains <kws> until end of turn" -----------
# Cluster (11). Scrubber #1 has the non-you-control form.
@_er(r"^target creature you control gains ([a-z, ]+?) until end of turn"
     r"(?:\.|$)")
def _ally_tgt_grant(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="creature", you_control=True,
                                      targeted=True))


# --- "target creature you control gets +N/+N" (no duration) ----------------
@_er(r"^target creature you control gets ([+-]\d+)/([+-]\d+)\s*$")
def _ally_tgt_buff_nodur(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", you_control=True, targeted=True))


# --- "up to two target creatures each get +N/+N [and gain KW] until EOT" ---
# Cluster (8). Pair of targets that each receive the same buff.
@_er(r"^up to two target creatures each get ([+-]\d+)/([+-]\d+)"
     r"(?: and gain ([a-z, ]+?))?"
     r"(?: until end of turn)?"
     r"(?: and can'?t be blocked this turn)?"
     r"(?:\.|$)")
def _up_to_two_buff(m):
    p, t = int(m.group(1)), int(m.group(2))
    filt = Filter(base="creature", quantifier="up_to_n", count=2, targeted=True)
    buff = Buff(power=p, toughness=t, target=filt)
    if m.group(3):
        return Sequence(items=(buff,
                               GrantAbility(ability_name=m.group(3).strip(),
                                            target=filt)))
    return buff


# --- "up to two target creatures can't block this turn" --------------------
@_er(r"^up to two target creatures can'?t block this turn(?:\.|$)")
def _up_to_two_cant_block(m):
    return UnknownEffect(raw_text="up to two target creatures can't block")


# --- "return up to two target creatures to their owner's hand(s)" ----------
# Cluster (8). Aether Spellbomb / Titan's Revenge tail.
@_er(r"^return up to two target creatures(?: your opponents control| you control)?"
     r" to their owners?'? hands?(?:\.|$)")
def _return_up_to_two_bounce(m):
    return UnknownEffect(raw_text="return up to two target creatures to hand")


# --- "return those cards to the battlefield [under their owner's control]" -
# Cluster (7). Chainer's Edict-tail / Scapegoat-tail reanimator continuation.
@_er(r"^return those cards to the battlefield(?: under their owner'?s? control)?"
     r"(?:\.|$)")
def _return_those_bf(m):
    return UnknownEffect(raw_text="return those cards to the battlefield")


# --- "target creature you control fights <X>" -----------------------------
# Cluster (9). Prey Upon-family bite spells with "you control"+tail.
@_er(r"^target creature you control fights "
     r"(?:that creature|target creature(?: you don'?t control| an opponent controls)?|"
     r"up to one target creature(?: you don'?t control| an opponent controls)?)"
     r"(?:\.|$)")
def _fight_tail(m):
    return Fight(a=Filter(base="creature", you_control=True, targeted=True),
                 b=Filter(base="creature", opponent_controls=True, targeted=True))


# --- "each opponent sacrifices a creature (or planeswalker)? of their choice [tail]"
# Cluster (11). Classic edict hitting all opponents.
@_er(r"^each opponent sacrifices (?:a|an) (creature(?: or planeswalker)?|artifact|enchantment|land|nonland permanent|permanent)"
     r"(?: of their choice)?(?:[,\s][^.]+)?(?:\.|$)")
def _each_opp_sac(m):
    return UnknownEffect(raw_text=f"each opponent sacs {m.group(1)}")


# --- "target opponent sacrifices a creature (or planeswalker)? ..." --------
# Cluster (7). Diabolic Edict-family.
@_er(r"^target opponent sacrifices (?:a|an) (creature(?: or planeswalker)?|artifact|enchantment|land|permanent)"
     r"(?: of their choice)?(?:[,\s][^.]+)?(?:\.|$)")
def _tgt_opp_sac(m):
    return UnknownEffect(raw_text=f"target opponent sacs {m.group(1)}")


# --- "its controller creates a <token>" ------------------------------------
# Cluster (19). Backside of "when ~ dies" / triggers whose effect lands on
# the creature's controller (typically the opponent).
@_er(r"^its controller creates (?:a|an|\d+) [^.]+? (?:creature )?tokens?"
     r"(?:\s+with [^.]+)?(?:\.|$)")
def _its_controller_creates(m):
    return UnknownEffect(raw_text="its controller creates token")


# --- "sacrifice/exile that token [at end of combat]" -----------------------
# Cluster (19). Trailing rider on temp-token spells (Master of Cruelties
# copy / Feldon of the Third Path).
@_er(r"^(sacrifice|exile) that token(?: at end of combat)?\s*\"?\s*$")
def _token_cleanup(m):
    return UnknownEffect(raw_text=f"{m.group(1).lower()} that token")


# --- "that token gains haste [until end of turn] [and attacks this combat if able]"
# Cluster (10). Haste-rider on token creation.
@_er(r"^that token gains haste(?: until end of turn)?"
     r"(?: and attacks(?: this combat)? if able)?(?:\.|$)")
def _that_token_haste(m):
    return GrantAbility(ability_name="haste",
                        target=Filter(base="that_token", targeted=False))


# --- "they gain haste [until end of turn] [and attacks this combat if able]"
# Cluster (6). Plural-token haste rider.
@_er(r"^they gain haste(?: until end of turn)?"
     r"(?: and attack(?: this combat)? if able)?(?:\.|$)")
def _they_gain_haste(m):
    return GrantAbility(ability_name="haste",
                        target=Filter(base="they", targeted=False))


# --- "you skip your next turn / untap step / draw step" --------------------
# Cluster (7). Time Stop / Timetwister-family drawback rider.
@_er(r"^you skip your next (turn|untap step|draw step|combat phase)(?:\.|$)")
def _skip_next(m):
    return UnknownEffect(raw_text=f"skip next {m.group(1)}")


# --- "roll a d20 [and add ...]" --------------------------------------------
# Cluster (10). CLB d20 flavor.
@_er(r"^roll a d20(?: and add [^.]+)?(?:\.|$)")
def _roll_d20(m):
    return UnknownEffect(raw_text="roll a d20")


# --- "you draw a card and (gain N life | lose N life)" ---------------------
# Cluster (6). Common paired effect on cyclers and edict tails.
@_er(r"^you draw a card and (?:you )?(gain|lose) (\d+) life\s*$")
def _draw_and_life(m):
    return UnknownEffect(raw_text=f"draw and {m.group(1)} {m.group(2)} life")


# --- "put one [pile] into your hand and the other into your graveyard [; then shuffle]"
# Cluster (11). Fact-or-Fiction / Browbeat-family pile tails.
@_er(r"^put one(?: pile)? into (?:your|their) (hand|graveyard) and the other"
     r" into (?:your|their) (hand|graveyard)(?:\.\s*then shuffle)?(?:\.|$)")
def _pile_split(m):
    return UnknownEffect(raw_text=f"pile split {m.group(1)}/{m.group(2)}")


# --- "choose one of them" --------------------------------------------------
# Cluster (10). Pile-chooser continuation.
@_er(r"^choose one of them(?:\.|$)")
def _choose_one_of_them(m):
    return UnknownEffect(raw_text="choose one of them")


# --- "at the beginning of your next upkeep" (bare tail) --------------------
# Cluster (5). Dangling phase phrase the splitter didn't glue to its body.
@_er(r"^at the beginning of your next upkeep\s*$")
def _next_upkeep(m):
    return UnknownEffect(raw_text="at the beginning of your next upkeep")


# --- "at end of combat, <verb> it" -----------------------------------------
# Cluster (~5). Sac-at-EOC continuation.
@_er(r"^at end of combat,? (sacrifice|exile|destroy|return) it(?: to its owner'?s hand)?(?:\.|$)")
def _eoc_it(m):
    return UnknownEffect(raw_text=f"eoc {m.group(1)} it")


# --- "sacrifice this creature" (bare, standalone ability line) ------------
# Cluster (2). Phage-family self-sac riders, and leftover from split cards.
@_er(r"^sacrifice this creature\s*$")
def _sac_this_creature(m):
    return UnknownEffect(raw_text="sacrifice this creature")


# --- Bare P/T ("4/4", "6/6", "x/x") — token-size tail ----------------------
# Cluster (~13 across 4/4 and 6/6, plus x/x residuals). The token-creation
# splitter occasionally leaves the P/T field as its own ability. Swallow it.
@_er(r"^(\d+|x)/(\d+|x)\s*$")
def _bare_pt(m):
    return UnknownEffect(raw_text=f"bare P/T {m.group(1)}/{m.group(2)}")


# --- "put that card into your hand" (scrubber #1 covered the standalone
# body; add the more-common "into hand" variants with minor riders) ---------
@_er(r"^put (?:that card|those cards) into (?:your|their|its owner'?s?) hands?"
     r"(?:[,\s][^.]+)?(?:\.|$)")
def _put_into_hand_variants(m):
    return UnknownEffect(raw_text="put card into hand")


# --- "each other creature you control that's <type> gets +N/+N" -----------
# Cluster (13). Tribal lord but with a "that's X" qualifier form.
@_er(r"^each other creature you control (?:that'?s [a-z,/ ]+?|with [^,]+?|of the chosen type)"
     r" gets \+(\d+)/\+(\d+)(?:\.|$)")
def _each_other_qualified_anthem(m):
    p, t = int(m.group(1)), int(m.group(2))
    filt = Filter(base="creature", you_control=True,
                  extra=("other", "qualified"), targeted=False)
    return Buff(power=p, toughness=t, target=filt, duration="permanent")


# --- "each other creature you control has <kw>" ----------------------------
# Same family but keyword-grant static.
@_er(r"^each other creature you control has ([a-z, ]+?)(?:\s+from [^.]+)?\s*$")
def _each_other_has(m):
    filt = Filter(base="creature", you_control=True,
                  extra=("other",), targeted=False)
    return GrantAbility(ability_name=m.group(1).strip(), target=filt,
                        duration="permanent")


# --- "enchanted/equipped creature gets +N/+N and <tail>" (residual shapes)
# Cluster (49 + 92 combined residuals). Scrubber #2 and base parser handle
# many enchanted-get shapes; these additional patterns cover the "... and
# is a <type>" / "... and can't <verb>" / "... and doesn't untap" tails.
@_er(r"^(enchanted|equipped) creature gets \+(\d+)/\+(\d+)"
     r"(?:, has [^,.]+?)*"
     r"(?:,? and (?:is a [^.]+|can'?t [^.]+|doesn'?t untap[^.]*|loses? [^.]+|"
     r"has [^.]+|is all creature types|is every creature type|is goaded))?"
     r"\s*$")
def _aura_eq_rider(m):
    who = m.group(1).lower() + "_creature"
    p, t = int(m.group(2)), int(m.group(3))
    return Buff(power=p, toughness=t,
                target=Filter(base=who, targeted=False),
                duration="permanent")


# --- "enchanted creature has <kw or quoted ability>" -----------------------
# Cluster (16). Scrubber catches the base grant form; quoted-ability grants
# fall through.
@_er(r"^enchanted creature has (?:\"[^\"]+\"|[a-z, ]+?)"
     r"(?:\s+and (?:\"[^\"]+\"|[a-z, ]+?))?\s*$")
def _enchanted_has_quoted(m):
    return GrantAbility(ability_name="quoted_ability_grant",
                        target=Filter(base="enchanted_creature",
                                      targeted=False),
                        duration="permanent")


# --- "for each creature <...>, <effect>" — for-each prelude -----------------
# Cluster (21+). Many cards have a standalone "for each creature you control,
# create a ..." sentence.
@_er(r"^for each (?:[a-z/ ]+? )?creature (?:you control|your opponents control|"
     r"card exiled this way|tapped this way|card milled this way|attacking[^,]*),?"
     r" [^.]+?(?:\.|$)")
def _for_each_creature_effect(m):
    return UnknownEffect(raw_text="for each creature <scope>, <effect>")


# --- "when you sacrifice this <type>, <effect>" (aura/enchantment riders) --
# Cluster (~10 — scrubber #2 had "when you sacrifice" implicitly but the
# aura-specific shape falls through. Claim it generically as a trigger by
# wrapping as an UnknownEffect at parse-time.
# (Note: this one goes under TRIGGER_PATTERNS below.)


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — new trigger shapes
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # --- "when you unlock this door" (Duskmourn rooms) ------------------
    # Cluster (20). Brand-new-mechanic trigger from DSK.
    (re.compile(r"^when you unlock this door", re.I),
     "unlock_door", "self"),

    # --- "when this creature specializes" (LCI class/specialize) --------
    # Cluster (27). Specialize trigger on the creature side.
    (re.compile(r"^when this creature specializes", re.I),
     "specialize_creature", "self"),

    # --- "when this card specializes from <zone>" -----------------------
    # Cluster (10). Specialize-from-graveyard / from-any-zone riders.
    (re.compile(r"^when this card specializes from (?:your graveyard|any zone|exile)",
                re.I),
     "specialize_from_zone", "self"),

    # --- "whenever this creature mutates" (IKO) -------------------------
    # Cluster (19). Mutate trigger — surprisingly absent from base.
    (re.compile(r"^whenever this creature mutates", re.I),
     "mutates", "self"),

    # --- "whenever equipped creature dies" ------------------------------
    # Cluster (17). Equipment-specific graveyard trigger. Base has
    # "whenever ~ dies" but "equipped creature" is a filter we didn't
    # teach the trigger matcher.
    (re.compile(r"^whenever equipped creature dies", re.I),
     "equipped_creature_dies", "self"),

    # --- "whenever enchanted creature dies" (parallel to above) ---------
    (re.compile(r"^whenever enchanted creature dies", re.I),
     "enchanted_creature_dies", "self"),

    # --- "whenever you commit a crime" (MKM) ----------------------------
    # Cluster (16). MKM crime trigger.
    (re.compile(r"^whenever you commit a crime", re.I),
     "commit_crime", "self"),

    # --- "when this creature blocks" ------------------------------------
    # Cluster (6). Wall-of-Shadows-era block trigger.
    (re.compile(r"^when (?:this creature|~) blocks", re.I),
     "self_blocks", "self"),

    # --- "when you sacrifice this (creature|aura|enchantment|artifact)" -
    # Cluster (~15). Enduring-cycle / aura-centered sacrifice triggers.
    (re.compile(r"^when you sacrifice (?:this|a|an) "
                r"(?:creature|aura|enchantment|artifact|permanent|token|clue|treasure|food)",
                re.I),
     "you_sacrifice_type", "self"),

    # --- "when you lose control of <X>" ---------------------------------
    # Cluster (8). Mind-Slaver-adjacent / enchantment-steal cleanup.
    (re.compile(r"^when you lose control of (?:this|~|[a-z ]+)", re.I),
     "lose_control_of", "self"),

    # --- "when this creature's power is N or greater" -------------------
    # Cluster (~5). "Tome of Legends" flavor sigil.
    (re.compile(r"^when (?:this creature|~)'?s power is \d+ or greater",
                re.I), "self_power_threshold", "self"),

    # --- "when this creature phases in / out" ---------------------------
    # Cluster (~4). Teferi-style phase triggers.
    (re.compile(r"^when(?:ever)? (?:this creature|~) phases (?:in|out)",
                re.I), "self_phase_inout", "self"),

    # --- "whenever this creature's cumulative upkeep is paid" -----------
    # Cluster (~3). Cumulative-upkeep pay trigger.
    (re.compile(r"^whenever (?:this creature|~)'?s cumulative upkeep is paid",
                re.I), "cumulative_upkeep_paid", "self"),

    # --- "whenever this creature enlists/trains/mentors a creature" -----
    # Cluster (~3 each). Scrubber-cheap addition.
    (re.compile(r"^whenever (?:this creature|~) (enlists|trains|mentors) "
                r"(?:a creature|another creature)", re.I),
     "self_squad_action", "self"),

    # --- "whenever this creature enters or dies" -----------------------
    # Cluster (~5). Combined-event trigger the base didn't handle.
    (re.compile(r"^whenever (?:this creature|~) enters or dies", re.I),
     "self_enter_or_die", "self"),
]
