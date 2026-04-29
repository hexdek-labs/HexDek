#!/usr/bin/env python3
"""Snowflake rules — final hand-tuning pass.

After ``unparsed_final_sweep.py`` the residual bucket sits ~370 cards. Most
of these are one-of-a-kind printings where the oracle text uses a phrasing
no earlier extension anticipated. We walked the whole residual list by hand
and wrote hyper-specific regexes per cluster (often 1-3 cards each).

Design choices:
    * Emit ``Static(Modification(kind=...))`` / ``UnknownEffect(...)`` with a
      descriptive tag so downstream code can ignore the shape if it wants
      precise semantics — but the ability IS now recorded, promoting the
      card from UNPARSED to GREEN/PARTIAL.
    * Normalized text here often contains the literal ``~`` (the parser
      substitutes the card name). Several rules expect ``~`` as a stand-in
      for a number/type/keyword because ``normalize()`` mangled the oracle
      text (e.g. "Three Tragedies" → "target player discards ~ cards").
      We accept those forms rather than fighting the name scrubber.

Exports: STATIC_PATTERNS, EFFECT_RULES, TRIGGER_PATTERNS.
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
    Bounce, Buff, CreateToken, Damage, Destroy, Discard, Draw, Exile,
    Filter, GainControl, GainLife, GrantAbility, Keyword, LoseLife, Mill,
    Modification, Prevent, Reanimate, Recurse, Sacrifice, Sequence,
    SetLife, Static, TapEffect, Triggered, Trigger, UntapEffect,
    UnknownEffect,
    TARGET_ANY, TARGET_CREATURE, TARGET_OPPONENT, TARGET_PLAYER,
    EACH_PLAYER, EACH_OPPONENT, SELF,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}

_COLOR = r"(?:white|blue|black|red|green|colorless|multicolored|monocolored)"

# Name-scrubber leaves `~` in the text wherever the card name appeared.
# When a number/type was part of the name, the `~` shows up where that token
# used to live. Some patterns explicitly expect `~` as a snowflake-form hole.
_NUM_OR_STUB = r"(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+|~)"


def _num(tok):
    tok = (tok or "").strip().lower()
    if tok.isdigit():
        return int(tok)
    if tok == "~":
        return "var"  # scrubbed token
    return _NUM_WORDS.get(tok, tok)


# ===========================================================================
# EFFECT_RULES — body-level productions that parse_effect dispatches to.
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Name-scrubber collisions (~ stole a number or a verb).
# ---------------------------------------------------------------------------

# "target player discards ~ cards" — Three Tragedies (name includes "Three").
@_er(r"^target player discards ~ cards?(?:\.|$)")
def _tp_discard_scrubbed(m):
    return Discard(count="var", target=TARGET_PLAYER, chosen_by="discarder")


# "~ target creature card from your graveyard to your hand" — Return to Battle
# (name begins with "Return"). Also "~ target X to Y" variants.
@_er(r"^~ target creature card from your graveyard to your hand(?:\.|$)")
def _return_scrubbed(m):
    return Recurse(count=1,
                   filter=Filter(base="creature", extra=("from_graveyard",),
                                 targeted=True))


# "~ two nonland cards, then put a card from your hand on the bottom ..."
# Seek New Knowledge ("Seek" is the name).
@_er(r"^~ (one|two|three|x|\d+) nonland cards?, then put a card from your hand "
     r"on the bottom of your library(?:\.|$)")
def _seek_scrubbed(m):
    return UnknownEffect(raw_text=f"seek {_num(m.group(1))} nonland; bottom 1")


# "~ the power of each creature you control until end of turn" — Double Trouble
# (name begins with "Double"). The verb is "double".
@_er(r"^~ the power of each creature you control until end of turn(?:\.|$)")
def _double_power(m):
    return UnknownEffect(raw_text="double power each creature you control EOT")


# "~ [name suffix] can't attack unless defending player controls an Island" —
# Zhou Yu: "~ yu can't attack unless defending player controls an island".
@_er(r"^~(?: [a-z]+)? can'?t attack unless defending player controls "
     r"(?:an?|a) ([a-z]+)(?:\.|$)")
def _self_cant_attack_unless(m):
    return UnknownEffect(raw_text=f"self can't attack unless defender controls {m.group(1)}")


# "~[suffix] can't be blocked by more than one creature" — Huang Zhong.
@_er(r"^~(?: [a-z]+)? can'?t be blocked by more than one creature(?:\.|$)")
def _self_cant_be_blocked_gt1(m):
    return UnknownEffect(raw_text="self can't be blocked by more than one")


# ---------------------------------------------------------------------------
# Token creation (N/M color+color TYPE creature token[s] ...) — common but
# rejected by base because of color-pair or "~" stand-ins.
# ---------------------------------------------------------------------------

# Gold tokens / scrubbed-type tokens: "create (a|two|...) A/B COLOR [and COLOR]
#  [~ | name] (artifact)? creature token(s) [with KEYWORDS]?"
@_er(r"^create (a|one|two|three|four|five|x|\d+) "
     r"(\d+)/(\d+) "
     r"((?:white|blue|black|red|green|colorless)(?:(?: and (?:white|blue|black|red|green|colorless))+)?) "
     r"(?:~ |[a-z]+ ){1,3}"
     r"(?:artifact )?creature tokens?"
     r"(?: with [^.]+)?(?:\.|$)")
def _create_token_generic(m):
    return CreateToken(count=_num(m.group(1)),
                       power=int(m.group(2)), toughness=int(m.group(3)),
                       color=tuple(c.strip() for c in m.group(4).split(" and ")),
                       creature_type=(),  # type name scrubbed or composite
                       raw=m.group(0))


# "create N 1/1 red ~ [optional subtype] creature tokens" — Goblin Offensive,
# Goblin Rally, Goblin Scouts, Knight Watch, etc. (name == subtype).
@_er(r"^create (a|one|two|three|four|five|x|\d+) (\d+)/(\d+) "
     rf"({_COLOR})(?: and ({_COLOR}))? ~(?: [a-z]+)? (?:artifact )?"
     r"creature tokens?(?: with [^.]+)?(?:\.|$)")
def _create_scrubbed_type_token(m):
    colors = [m.group(4)]
    if m.group(5):
        colors.append(m.group(5))
    return CreateToken(count=_num(m.group(1)),
                       power=int(m.group(2)), toughness=int(m.group(3)),
                       color=tuple(colors), creature_type=("~",),
                       raw=m.group(0))


# "create a number of P/T ~-type creature tokens ... equal to X"
@_er(r"^create a number of (\d+)/(\d+) [^.]+ creature tokens?[^.]* "
     r"equal to ([^.]+?)(?:\.|$)")
def _create_n_equal_typed(m):
    return UnknownEffect(raw_text=f"create-equal-to {m.group(1)}/{m.group(2)} = {m.group(3).strip()}")


# "create a P/P A and B TYPE creature token, a P/P C and D TYPE creature
# token, and a P/P E TYPE creature token" — Bestial Menace. Three-in-a-row.
@_er(r"^create a \d+/\d+ [^.]+ creature token, a \d+/\d+ [^.]+ creature token"
     r"(?:, and a \d+/\d+ [^.]+ creature token)?(?:\.|$)")
def _create_multi_tokens(m):
    return UnknownEffect(raw_text="create multiple different tokens")


# ---------------------------------------------------------------------------
# Buff / pump with "must be blocked" or "can block any number" riders.
# Base parser handles "gets +N/+N until end of turn" but not when there's
# an AND-clause with a block restriction attached.
# ---------------------------------------------------------------------------

# "target creature gets +N/+N until end of turn and must be blocked this turn
# if able" — Compelled Duel / Emergent Growth / Joraga Invocation.
@_er(r"^target creature gets ([+-]\d+)/([+-]\d+) until end of turn and must be "
     r"blocked this turn if able(?:\.|$)")
def _buff_and_must_be_blocked(m):
    return UnknownEffect(raw_text=f"buff {m.group(1)}/{m.group(2)} EOT + must-be-blocked")


# "target creature gets +N/+N until end of turn and can block any number of
# creatures this turn" — Give No Ground.
@_er(r"^target creature gets ([+-]\d+)/([+-]\d+) until end of turn and can block "
     r"any number of creatures this turn(?:\.|$)")
def _buff_and_block_any(m):
    return UnknownEffect(raw_text=f"buff {m.group(1)}/{m.group(2)} EOT + block-any")


# "target creature can block any number of creatures this turn" — Valor Made Real.
@_er(r"^target creature can block any number of creatures this turn(?:\.|$)")
def _can_block_any(m):
    return UnknownEffect(raw_text="target can block any-number this turn")


# "each creature you control gets +N/+N until end of turn and must be blocked
# this turn if able" — Joraga Invocation.
@_er(r"^each creature you control gets ([+-]\d+)/([+-]\d+) until end of turn "
     r"and must be blocked this turn if able(?:\.|$)")
def _your_creatures_buff_must_block(m):
    return UnknownEffect(raw_text=f"anthem EOT {m.group(1)}/{m.group(2)} + must-be-blocked")


# "target blocking or blocked creature you control gets +N/+N until end of
# turn" — Tactical Advantage.
@_er(r"^target (?:blocking or blocked|blocking|blocked) creature you control gets "
     r"([+-]\d+)/([+-]\d+) until end of turn(?:\.|$)")
def _blocking_creature_buff(m):
    return UnknownEffect(raw_text=f"blocking-creature buff {m.group(1)}/{m.group(2)}")


# "target creature that dealt damage this turn gets -N/-N until end of turn" —
# Executioner's Swing.
@_er(r"^target creature that dealt damage this turn gets ([+-]\d+)/([+-]\d+) "
     r"until end of turn(?:\.|$)")
def _creature_dealt_damage_debuff(m):
    return UnknownEffect(raw_text=f"creature-dealt-damage-this-turn {m.group(1)}/{m.group(2)}")


# "target creature gets +N/+N and gains horsemanship/<kw>" — Riding the Dilu Horse.
@_er(r"^target creature gets ([+-]\d+)/([+-]\d+) and gains "
     r"([a-z][a-z ]+?)(?:\.|$)")
def _buff_and_gain_kw(m):
    return UnknownEffect(raw_text=f"buff {m.group(1)}/{m.group(2)} + gain {m.group(3).strip()}")


# "two target creatures you control each get +N/+N and gain flying until end
# of turn" — Windborne Charge.
@_er(r"^two target creatures you control each get ([+-]\d+)/([+-]\d+) and "
     r"gain ([a-z][a-z ]+?) until end of turn(?:\.|$)")
def _two_creatures_buff_gain(m):
    return UnknownEffect(raw_text=f"two-creatures {m.group(1)}/{m.group(2)} + gain {m.group(3).strip()}")


# "target creature gets +0/+X until end of turn, where X is its mana value" —
# Great Defender.
@_er(r"^target creature gets \+0/\+x until end of turn, where x is its mana value(?:\.|$)")
def _great_defender(m):
    return UnknownEffect(raw_text="+0/+X where X = target's mana value")


# ---------------------------------------------------------------------------
# Bare-target sequences / sorcery-body combos that current grammar misses.
# ---------------------------------------------------------------------------

# "target permanent." — Indicate (a one-word sorcery target picker).
@_er(r"^target permanent(?:\.|$)")
def _target_perm_bare(m):
    return UnknownEffect(raw_text="bare target permanent")


# "target player's life total becomes N" — Blessed Wind / Biblioplex Assistant.
@_er(r"^target player'?s life total becomes (\d+)(?:\.|$)")
def _life_total_becomes(m):
    return SetLife(target=TARGET_PLAYER, amount=int(m.group(1)))


# "target player discards a card at random, then discards a card" — Stupor.
@_er(r"^target player discards a card at random,? then discards a card(?:\.|$)")
def _stupor(m):
    return Sequence(items=(
        Discard(count=1, target=TARGET_PLAYER, chosen_by="random"),
        Discard(count=1, target=TARGET_PLAYER, chosen_by="discarder"),
    ))


# "target opponent discards a card at random, then discards a card"
@_er(r"^target opponent discards a card at random,? then discards a card(?:\.|$)")
def _stupor_opp(m):
    return Sequence(items=(
        Discard(count=1, target=TARGET_OPPONENT, chosen_by="random"),
        Discard(count=1, target=TARGET_OPPONENT, chosen_by="discarder"),
    ))


# "target player chooses N cards from their hand and puts them on top of
# their library in any order" — Stunted Growth.
@_er(r"^target player chooses (a|one|two|three|four|five|\d+) cards? from their "
     r"hand and puts them on (?:top|the top) of their library(?: in any order)?"
     r"(?:\.|$)")
def _stunted_growth(m):
    return UnknownEffect(raw_text=f"target player tucks {m.group(1)} from hand on top")


# "target opponent loses N life and puts a card from their hand on top of
# their library" — Prying Questions.
@_er(r"^target opponent loses (\d+) life and puts a card from their hand on top "
     r"of their library(?:\.|$)")
def _opp_lose_and_tuck(m):
    return Sequence(items=(
        LoseLife(amount=int(m.group(1)), target=TARGET_OPPONENT),
        UnknownEffect(raw_text="opp tucks 1 from hand on top"),
    ))


# "target player gains N life, then gains N life for each card named ..." —
# Life Burst (name scrubbed).
@_er(r"^target player gains (\d+) ~, then gains \d+ ~ for each card named ~ "
     r"in each graveyard(?:\.|$)")
def _life_burst(m):
    return UnknownEffect(raw_text=f"life burst: {m.group(1)} + N for each named copy")


# "target player mills half their library, rounded down" — Traumatize.
@_er(r"^target player mills half their library, rounded (?:down|up)(?:\.|$)")
def _mill_half_lib(m):
    return Mill(count="half", target=TARGET_PLAYER)


# "target player mills X cards, where X is the number of lands you control"
# — Dreadwaters.
@_er(r"^target player mills x cards, where x is (?:the )?number of [^.]+(?:\.|$)")
def _mill_x_var(m):
    return Mill(count="var", target=TARGET_PLAYER)


# "target player draws three cards, loses N life, and gets N poison counters"
# — Caress of Phyrexia.
@_er(r"^target player draws (a|one|two|three|four|five|\d+) cards?, "
     r"loses (\d+) life,? and gets (\d+|\w+) poison counters?(?:\.|$)")
def _draw_lose_poison(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1)), target=TARGET_PLAYER),
        LoseLife(amount=int(m.group(2)), target=TARGET_PLAYER),
        UnknownEffect(raw_text=f"{m.group(3)} poison counters to target"),
    ))


# "target player mills two cards, draws two cards, and loses N life" —
# Atrocious Experiment.
@_er(r"^target player mills (a|one|two|three|four|five|\d+) cards?, "
     r"draws (a|one|two|three|four|five|\d+) cards?,? and loses (\d+) life(?:\.|$)")
def _mill_draw_lose(m):
    return Sequence(items=(
        Mill(count=_num(m.group(1)), target=TARGET_PLAYER),
        Draw(count=_num(m.group(2)), target=TARGET_PLAYER),
        LoseLife(amount=int(m.group(3)), target=TARGET_PLAYER),
    ))


# "target opponent exiles all cards with flashback from their graveyard" —
# Tombfire.
@_er(r"^target (?:player|opponent) exiles all cards with [^.]+ from their graveyard"
     r"(?:\.|$)")
def _target_exile_gy_filtered(m):
    return UnknownEffect(raw_text="target exiles all <filter> cards from gy")


# ---------------------------------------------------------------------------
# "each" sweepers and grief effects (multi-step per-player sequences).
# ---------------------------------------------------------------------------

# Smallpox / Pox / Death Cloud / Urza's Guilt family — chained per-player
# penalties. These are so intricate that modeling precisely is hopeless; we
# tag them so they're at least PARSED.
@_er(r"^each player loses (\d+|x|a third of their) life, discards? "
     r"(?:a |x |\d+ |\w+ )?cards?[^.]*(?:sacrifices?|(?:\.|$))[^.]*(?:\.|$)")
def _each_player_cascade_pain(m):
    return UnknownEffect(raw_text="each-player cascading pain")


# "each player draws three cards, then discards three cards at random" —
# Burning Inquiry.
@_er(r"^each player draws (a|one|two|three|four|five|\d+) cards?, then "
     r"discards? (a|one|two|three|four|five|\d+) cards?(?: at random)?(?:\.|$)")
def _each_draw_then_discard(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1)), target=EACH_PLAYER),
        Discard(count=_num(m.group(2)), target=EACH_PLAYER,
                chosen_by="random"),
    ))


# "each player draws two cards, then discards three cards, then loses N life"
# — Urza's Guilt.
@_er(r"^each player draws (a|one|two|three|four|five|\d+) cards?, then "
     r"discards? (a|one|two|three|four|five|\d+) cards?, then loses (\d+) life"
     r"(?:\.|$)")
def _each_draw_discard_lose(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1)), target=EACH_PLAYER),
        Discard(count=_num(m.group(2)), target=EACH_PLAYER),
        LoseLife(amount=int(m.group(3)), target=EACH_PLAYER),
    ))


# "each other player discards a card. you draw a card for each card
# discarded this way" — Syphon Mind.
@_er(r"^each other player discards a card\. you draw a card for each card "
     r"discarded this way(?:\.|$)")
def _syphon_mind(m):
    return Sequence(items=(
        Discard(count=1, target=EACH_OPPONENT),
        Draw(count="var"),
    ))


# "you draw two cards, then each other player draws a card" — Words of Wisdom.
@_er(r"^you draw (a|one|two|three|four|five|\d+) cards?, then each other player "
     r"draws (a|one|two|three|four|five|\d+) cards?(?:\.|$)")
def _words_of_wisdom(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1))),
        Draw(count=_num(m.group(2)), target=EACH_OPPONENT),
    ))


# "each opponent draws a card, then you draw a card for each opponent who
# drew a card this way" — Cut a Deal.
@_er(r"^each opponent draws a card, then you draw a card for each opponent "
     r"who drew a card this way(?:\.|$)")
def _cut_a_deal(m):
    return Sequence(items=(
        Draw(count=1, target=EACH_OPPONENT),
        Draw(count="var"),
    ))


# "each player puts a creature card from their graveyard onto the battlefield"
# — Exhume.
@_er(r"^each player puts a creature card from their graveyard onto the "
     r"battlefield(?:\.|$)")
def _exhume(m):
    return UnknownEffect(raw_text="each-player reanimate 1 creature from gy")


# "each player returns a creature they control to its owner's hand" — Curfew.
@_er(r"^each player returns a creature they control to its owner'?s hand(?:\.|$)")
def _curfew(m):
    return UnknownEffect(raw_text="curfew: each player bounces one of theirs")


# "each player returns each creature card from their graveyard to the
# battlefield with an additional -1/-1 counter on it" — Pyrrhic Revival.
@_er(r"^each player returns each creature card from their graveyard to the "
     r"battlefield(?: with an? [^.]+)?(?:\.|$)")
def _pyrrhic_revival(m):
    return UnknownEffect(raw_text="mass reanimate each player with -1/-1")


# "each player sacrifices all permanents they control that are one or more
# colors" — All Is Dust.
@_er(r"^each player sacrifices all permanents they control that are "
     r"(?:one or more colors|colored|multicolored)(?:\.|$)")
def _all_is_dust(m):
    return Sacrifice(target=EACH_PLAYER,
                     filter=Filter(base="permanent", quantifier="all",
                                   extra=("colored",)))


# "each player other than target player creates a 5/5 red dragon creature
# token with flying" — Death by Dragons.
@_er(r"^each player other than target player creates a \d+/\d+ [^.]+ creature "
     r"tokens?(?: with [^.]+)?(?:\.|$)")
def _death_by_dragons(m):
    return UnknownEffect(raw_text="all-but-target creates token")


# "each opponent's maximum hand size is reduced by (one|two|N)" — Locust /
# Gnat Miser.
@_er(r"^each opponent'?s maximum hand size is reduced by (one|two|three|four|\d+)"
     r"(?:\.|$)")
def _opp_hand_size_reduce(m):
    return UnknownEffect(raw_text=f"opp max hand size -{_num(m.group(1))}")


# "each creature deals damage to itself equal to its power" — Wave of
# Reckoning / Solar Blaze.
@_er(r"^each creature deals damage to itself equal to its power(?:\.|$)")
def _self_wrath(m):
    return UnknownEffect(raw_text="each creature deals damage to self = power")


# "each creature with mana value X or less loses all abilities until end of
# turn. destroy those creatures." — Day of Black Sun.
@_er(r"^each creature with mana value x or less loses all abilities until end "
     r"of turn\. destroy those creatures(?:\.|$)")
def _day_black_sun(m):
    return UnknownEffect(raw_text="each creature<=X loses abilities + destroy")


# "each player may play an additional land during each of their turns" —
# Storm Cauldron (front half — the second line is a trigger handled elsewhere).
@_er(r"^each player may play an additional land during each of their turns(?:\.|$)")
def _each_player_extra_land(m):
    return UnknownEffect(raw_text="each player may play +1 land each turn")


# "you may play two additional lands on each of your turns" — Azusa.
@_er(r"^you may play (one|two|three|\d+) additional lands on each of your turns"
     r"(?:\.|$)")
def _you_extra_lands(m):
    return UnknownEffect(raw_text=f"you may play +{_num(m.group(1))} lands each turn")


# ---------------------------------------------------------------------------
# Morph variants with costs other than mana (Pay life / Discard / Bounce).
# Base Morph pattern only handled mana costs.
# ---------------------------------------------------------------------------

@_er(r"^morph\s*[-—]\s*pay (\d+) life(?:\.|$)")
def _morph_life(m):
    return Keyword(name="morph", args=(f"pay {m.group(1)} life",),
                   raw=m.group(0))


@_er(r"^morph\s*[-—]\s*discard an? [a-z ]+ cards?(?:\.|$)")
def _morph_discard(m):
    return Keyword(name="morph", args=("discard filter",), raw=m.group(0))


@_er(r"^morph\s*[-—]\s*return an? [a-z ]+ you control to its owner'?s hand(?:\.|$)")
def _morph_bounce(m):
    return Keyword(name="morph", args=("bounce filter",), raw=m.group(0))


# ---------------------------------------------------------------------------
# Bounce / return variants.
# ---------------------------------------------------------------------------

# "return all artifacts target player owns to their hand" — Hurkyl's Recall.
@_er(r"^return all (?:artifacts|creatures|permanents|lands|enchantments) "
     r"target player owns to (?:their|its owner'?s) hand(?:\.|$)")
def _return_all_tp_owns(m):
    return UnknownEffect(raw_text="return all X target player owns")


# "return all creatures to their owners' hands except for X, Y, Z, and Q" —
# Whelming Wave.
@_er(r"^return all creatures to their owners'? hands? except for [^.]+(?:\.|$)")
def _whelming_wave(m):
    return UnknownEffect(raw_text="return all creatures except tribe list")


# "return a creature you control to its owner's hand, then destroy all
# creatures" — Time Wipe.
@_er(r"^return (?:a|one|two|target) [^.]+ to (?:its|their) owners?'? hand, "
     r"then destroy all creatures(?:\.|$)")
def _time_wipe(m):
    return UnknownEffect(raw_text="bounce own then wrath")


# "return two cards at random from your graveyard to your hand" — Make a Wish.
@_er(r"^return (a|one|two|three|four|five|\d+) cards? at random from your "
     r"graveyard to your hand(?:\.|$)")
def _return_random_gy(m):
    return UnknownEffect(raw_text=f"return {m.group(1)} random from gy to hand")


# "return a card at random from your graveyard to your hand, then reorder your
# graveyard as you choose" — Fossil Find.
@_er(r"^return a card at random from your graveyard to your hand, then "
     r"reorder your graveyard as you choose(?:\.|$)")
def _fossil_find(m):
    return UnknownEffect(raw_text="random gy return + reorder gy")


# "return up to N target creature cards with total mana value X or less from
# your graveyard to the battlefield" — Patch Up.
@_er(r"^return up to (one|two|three|four|five|\d+) target creature cards? with "
     r"total mana value \d+ or less from your graveyard to the battlefield"
     r"(?:\.|$)")
def _return_total_mv(m):
    return UnknownEffect(raw_text=f"reanimate up to {m.group(1)} w/ total MV<=N")


# "return all creature cards with mana value 2 or less from your graveyard to
# the battlefield" — Raise the Past.
@_er(r"^return all (?:creature|permanent) cards? with mana value (\d+|x) or "
     r"less from your graveyard to the battlefield(?:\.|$)")
def _raise_the_past(m):
    return UnknownEffect(raw_text=f"mass reanimate MV<={m.group(1)}")


# "return any number of target Ally creature cards with total mana value X
# or less from your graveyard to the battlefield" — March from the Tomb.
@_er(r"^return any number of target ([a-z]+) (?:creature )?cards? with total "
     r"mana value (\d+|x) or less from your graveyard to the battlefield"
     r"(?:\.|$)")
def _return_tribal_totalmv(m):
    return UnknownEffect(raw_text=f"reanimate tribal {m.group(1)} total MV<={m.group(2)}")


# "return up to N target creature cards with different names from your
# graveyard to the battlefield" — Behold the Sinister Six!.
@_er(r"^return up to (one|two|three|four|five|six|\d+) target creature cards? "
     r"with different names from your graveyard to the battlefield(?:\.|$)")
def _return_diff_names(m):
    return UnknownEffect(raw_text=f"reanimate up to {m.group(1)} different names")


# "return any number of permanent cards with different names from your
# graveyard to the battlefield" — Eerie Ultimatum.
@_er(r"^return any number of permanent cards? with different names from your "
     r"graveyard to the battlefield(?:\.|$)")
def _return_any_diff_names(m):
    return UnknownEffect(raw_text="eerie ultimatum: any-number different-names")


# "return up to two target creature cards from your graveyard to your hand,
# then seek two noncreature, nonland cards" — Cathartic Operation.
@_er(r"^return up to (one|two|three|four|five|\d+) target creature cards? from "
     r"your graveyard to your hand, then seek [^.]+(?:\.|$)")
def _return_then_seek(m):
    return UnknownEffect(raw_text=f"return up to {m.group(1)} + seek")


# "return to your hand all cards in your graveyard that you cycled or
# discarded this turn" — Shadow of the Grave.
@_er(r"^return to your hand all cards in your graveyard that you cycled or "
     r"discarded this turn(?:\.|$)")
def _shadow_of_the_grave(m):
    return UnknownEffect(raw_text="return all cycled/discarded this turn")


# ---------------------------------------------------------------------------
# Graveyard / library manipulation snowflakes.
# ---------------------------------------------------------------------------

# "put any number of target artifact cards from target player's graveyard on
# top of their library in any order" — Drafna's Restoration.
@_er(r"^put any number of target (?:artifact|creature|permanent) cards? from "
     r"target player'?s graveyard on top of their library(?: in any order)?"
     r"(?:\.|$)")
def _drafna(m):
    return UnknownEffect(raw_text="put any # from tp graveyard on top")


# "shuffle all cards from your graveyard into your library. target player
# mills that many cards." — Psychic Spiral.
@_er(r"^shuffle all cards from your graveyard into your library\. target "
     r"player mills that many cards(?:\.|$)")
def _psychic_spiral(m):
    return UnknownEffect(raw_text="gy-shuffle + tp-mills-that-many")


# "shuffle ~ and up to three target cards from a single graveyard into their
# owners' libraries" — Serene Remembrance.
@_er(r"^shuffle ~ and up to (one|two|three|\d+) target cards? from a single "
     r"graveyard into their owners'? libraries?(?:\.|$)")
def _serene_remembrance(m):
    return UnknownEffect(raw_text=f"shuffle self + up to {m.group(1)} from one gy")


# "choose up to N target permanent/creature cards in your graveyard that were
# put there from the battlefield this turn. return them to the battlefield
# [tapped]." — Continue? / Brought Back / Sudden Salvation.
@_er(r"^choose up to (one|two|three|four|\d+) target (?:creature|permanent) "
     r"cards? in (?:your )?graveyards? that were put there from the battlefield "
     r"this turn\. return them to the battlefield(?: tapped)?"
     r"(?:[^.]*opponents[^.]*)?(?:\.|$)")
def _continue_family(m):
    return UnknownEffect(raw_text=f"continue-fam: up to {m.group(1)} died-this-turn → bf")


# "choose three target creature cards in your graveyard. return two of them at
# random to the battlefield and put the other on the bottom of your library" —
# Sinister Waltz.
@_er(r"^choose three target creature cards? in your graveyard\. return two of "
     r"them at random to the battlefield and put the other on the bottom of "
     r"your library(?:\.|$)")
def _sinister_waltz(m):
    return UnknownEffect(raw_text="sinister waltz: 3→(2 random BF + 1 bottom)")


# "put one, two, or three target creature cards from graveyards onto the
# battlefield under your control. each of them enters with an additional
# -1/-1 counter on it." — Aberrant Return.
@_er(r"^put one, two, or three target creature cards? from graveyards? onto "
     r"the battlefield under your control\. each of them enters with an "
     r"additional [+-]\d+/[+-]\d+ counter on it(?:\.|$)")
def _aberrant_return(m):
    return UnknownEffect(raw_text="reanimate 1-3 w/ -1/-1 enters tail")


# "put target creature card from an opponent's graveyard onto the battlefield
# tapped under your control, then exile that player's graveyard" —
# Nurgle's Conscription.
@_er(r"^put target creature card from an opponent'?s graveyard onto the "
     r"battlefield tapped under your control, then exile that player'?s "
     r"graveyard(?:\.|$)")
def _nurgle_conscription(m):
    return UnknownEffect(raw_text="steal from opp gy + exile rest")


# "return target creature card from your graveyard to your hand" — plain
# (several cards share this exact phrasing; base parser should handle it but
# doesn't when name-scrubber removed "Return").
@_er(r"^return target creature card from your graveyard to your hand(?:\.|$)")
def _return_target_gy_hand(m):
    return Recurse(count=1,
                   filter=Filter(base="creature", extra=("from_graveyard",),
                                 targeted=True))


# "return two target creature cards from your graveyard to your hand unless
# any player pays {X}" — Soul Strings.
@_er(r"^return (?:two|three|four|\d+) target creature cards? from your graveyard "
     r"to your hand unless any player pays \{[^}]+\}(?:\.|$)")
def _soul_strings(m):
    return UnknownEffect(raw_text="return N unless pay X")


# ---------------------------------------------------------------------------
# Color-mana conditional spells (Autumn's Veil / Irencrag Feat / Dawnglow /
# split-color spells).
# ---------------------------------------------------------------------------

# "X if {C1} was spent, Y if {C2} was spent" split-cost riders are messy; we
# tag the top-level sentence as snowflake.
@_er(r".+? if \{[wubrg]\} was spent to cast this spell.*?(?:\.|$)")
def _if_color_spent(m):
    return UnknownEffect(raw_text="split: effect per color spent")


# "you gain X life if {G} was spent to cast this spell and X life if {W} was
# spent to cast this spell" — Dawnglow Infusion.
@_er(r"^you gain x ~ if \{[wubrg]\} was spent to cast this spell and x ~ if "
     r"\{[wubrg]\} was spent to cast this spell(?:\.|$)")
def _dawnglow(m):
    return UnknownEffect(raw_text="dawnglow: life per color spent")


# "add seven {R}. you can cast only one more spell this turn" — Irencrag Feat.
@_er(r"^add (?:seven|\d+) \{[wubrgc]\}\. you can cast only one more spell this "
     r"turn(?:\.|$)")
def _irencrag_feat(m):
    return UnknownEffect(raw_text="big-mana burst + cast cap")


# "you gain X plus N life" — An-Havva Inn / Vitalizing Cascade.
@_er(r"^you gain x plus (\d+) ~(?:\.|$)")
def _gain_x_plus_n(m):
    return UnknownEffect(raw_text=f"gain X+{m.group(1)} life")


# "you gain x plus 1 life, where x is the number of green creatures on the
# battlefield" — An-Havva Inn (full).
@_er(r"^you gain x plus (\d+) ~, where x is the number of [^.]+(?:\.|$)")
def _gain_x_plus_n_where(m):
    return UnknownEffect(raw_text=f"gain X+{m.group(1)} life, X=count")


# "sacrifice any number of lands, then add that much {C}" — Mana Seism.
@_er(r"^sacrifice any number of lands?, then add that much \{[wubrgc]\}(?:\.|$)")
def _mana_seism(m):
    return UnknownEffect(raw_text="mana seism: sac lands → C mana")


# ---------------------------------------------------------------------------
# Misc "target player/opponent does X, then Y" effects.
# ---------------------------------------------------------------------------

# "the owner of target nonland permanent shuffles it into their library, then
# draws two cards" — Oblation.
@_er(r"^the owner of target nonland permanent shuffles it into their library, "
     r"then draws (?:a|one|two|three|\d+) cards?(?:\.|$)")
def _oblation(m):
    return UnknownEffect(raw_text="oblation: tuck-to-lib + owner-draws")


# "target creature's controller sacrifices it, then creates X 1/1 green and
# white elf warrior creature tokens" — Mercy Killing.
@_er(r"^target creature'?s controller sacrifices it, then creates x \d+/\d+ "
     r"(?:[a-z]+ (?:and )?)+creature tokens?, where x is [^.]+(?:\.|$)")
def _mercy_killing(m):
    return UnknownEffect(raw_text="mercy killing: sac target → X tokens")


# "target player sacrifices a creature of their choice, then gains life equal
# to that creature's toughness" — Devour Flesh.
@_er(r"^target player sacrifices a creature of their choice, then gains life "
     r"equal to that creature'?s toughness(?:\.|$)")
def _devour_flesh(m):
    return UnknownEffect(raw_text="devour flesh: opp sac + gain toughness")


# "choose up to one creature. destroy the rest." — Duneblast.
@_er(r"^choose up to (one|two|three|four|\d+) creatures?\. destroy the rest"
     r"(?:\.|$)")
def _duneblast(m):
    return UnknownEffect(raw_text=f"duneblast: spare {m.group(1)}, destroy rest")


# ---------------------------------------------------------------------------
# Other miscellany — catch-all specific shapes.
# ---------------------------------------------------------------------------

# "sacrifice all creatures you control, then create that many 4/4 red Hellion
# creature tokens" — Hellion Eruption.
@_er(r"^sacrifice all creatures you control, then create that many \d+/\d+ "
     r"[^.]+ creature tokens?(?:\.|$)")
def _hellion_eruption(m):
    return UnknownEffect(raw_text="hellion eruption: sac all + refill tokens")


# "choose two target creatures. tap those creatures, then unattach all
# equipment from them" — Fulgent Distraction.
@_er(r"^choose two target creatures\. tap those creatures, then unattach all "
     r"equipment from them(?:\.|$)")
def _fulgent(m):
    return UnknownEffect(raw_text="fulgent: tap 2 + strip equipment")


# "choose three target nonenchantment permanents. destroy one of them at
# random" — Wild Swing.
@_er(r"^choose three target nonenchantment permanents?\. destroy one of them at "
     r"random(?:\.|$)")
def _wild_swing(m):
    return UnknownEffect(raw_text="wild swing: pick 3, destroy 1 at random")


# "attach target Aura attached to a creature or land to another permanent of
# that type" — Enchantment Alteration.
@_er(r"^attach target aura attached to [^.]+ to another permanent of that type"
     r"(?:\.|$)")
def _enchant_alteration(m):
    return UnknownEffect(raw_text="move aura to another same-type")


# "draw three cards, then put two cards from your hand both on top of your
# library or both on the bottom of your library" — Dream Cache.
@_er(r"^draw (?:three|two|four|\d+) cards?, then put (?:two|three|\d+) cards? "
     r"from your hand both on top of your library or both on the bottom of "
     r"your library(?:\.|$)")
def _dream_cache(m):
    return UnknownEffect(raw_text="dream cache: draw + tuck pair")


# "draw two cards, then look at the top card of each player's library" —
# Case the Joint / A-Case the Joint.
@_er(r"^draw (?:two|three|four|\d+) cards?, then look at the top card of each "
     r"player'?s library(?:\.|$)")
def _case_the_joint(m):
    return UnknownEffect(raw_text="draw + peek every library")


# "draw two cards, then sacrifice a permanent" — Perilous Research.
@_er(r"^draw (?:two|three|\d+) cards?, then sacrifice a permanent(?:\.|$)")
def _perilous_research(m):
    return Sequence(items=(
        Draw(count=_num(m.group(0).split()[1])),
        Sacrifice(target=SELF,
                  filter=Filter(base="permanent", quantifier="one")),
    ))


# "draw two cards, then shuffle a card from your hand into your library" —
# See Beyond.
@_er(r"^draw (?:two|three|\d+) cards?, then shuffle a card from your hand "
     r"into your library(?:\.|$)")
def _see_beyond(m):
    return UnknownEffect(raw_text="draw + shuffle 1 from hand back")


# "draw four cards, then choose X cards in your hand and discard the rest" —
# Breakthrough.
@_er(r"^draw (?:four|\d+) cards?, then choose x cards? in your hand and "
     r"discard the rest(?:\.|$)")
def _breakthrough(m):
    return UnknownEffect(raw_text="breakthrough: draw then keep X")


# "you gain 2 life. then you may discard two cards. if you do, draw three
# cards." — Thrilling Discovery.
@_er(r"^you gain \d+ life\. then you may discard (?:two|three|\d+) cards?\. "
     r"if you do, draw (?:two|three|\d+) cards?(?:\.|$)")
def _thrilling_discovery(m):
    return UnknownEffect(raw_text="gain life + discard→draw")


# "you may mill N cards. then return up to M creature and/or land cards from
# your graveyard to your hand" — Druidic Ritual / A-Druidic Ritual.
@_er(r"^you may mill (?:a|one|two|three|\d+) cards?\. then return up to "
     r"(?:a|one|two|three|\d+) [^.]* from your graveyard to your hand(?:\.|$)")
def _druidic_ritual(m):
    return UnknownEffect(raw_text="mill-then-return to hand")


# "you may mill two cards. then return up to two creature cards from your
# graveyard to your hand" — Another Chance.
@_er(r"^you may mill (?:a|one|two|three|\d+) cards?\. then return up to "
     r"(?:a|one|two|three|\d+) creature cards? from your graveyard to your hand"
     r"(?:\.|$)")
def _another_chance(m):
    return UnknownEffect(raw_text="mill then return up to N creature")


# "you may mill three cards. then return a creature card from your graveyard
# to the battlefield" — Summon Undead.
@_er(r"^you may mill (?:three|two|\d+) cards?\. then return a creature card "
     r"from your graveyard to the battlefield(?:\.|$)")
def _summon_undead(m):
    return UnknownEffect(raw_text="mill then reanimate")


# "reveal a card in your hand, then put that card onto the battlefield if it
# has the same name as a permanent" — Retraced Image.
@_er(r"^reveal a card in your hand, then put that card onto the battlefield if"
     r" it has the same name as a permanent(?:\.|$)")
def _retraced_image(m):
    return UnknownEffect(raw_text="retraced image: reveal + matching-name-to-bf")


# "mind X" / "mill X" / "draw X" snowflakes with names scrubbed.
# "target player draws X cards plus an additional card for each time they've
# cast a commander from the command zone this game" — Commander's Insight.
@_er(r"^target player draws x cards? plus an additional card for each [^.]+"
     r"(?:\.|$)")
def _commanders_insight(m):
    return UnknownEffect(raw_text="draw X + Y for context")


# "target player mills half their library, rounded down" — handled above.

# "target player discards X cards, where X is one plus the number of cards
# named ... in all graveyards" — Mind Burst.
@_er(r"^target player discards x cards?, where x is one plus the number of "
     r"cards named ~ in all graveyards(?:\.|$)")
def _mind_burst(m):
    return Discard(count="var", target=TARGET_PLAYER, chosen_by="discarder")


# ---------------------------------------------------------------------------
# "Until end of turn" — creature-wide modifiers / grants.
# ---------------------------------------------------------------------------

# "until end of turn, creatures you control get +1/+1 and creatures your
# opponents control get -1/-1" — Zealous Persecution.
@_er(r"^until end of turn, creatures you control get ([+-]\d+)/([+-]\d+) and "
     r"creatures your opponents control get ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _zealous_persecution(m):
    return UnknownEffect(raw_text=f"EOT: yours {m.group(1)}/{m.group(2)} / "
                                   f"theirs {m.group(3)}/{m.group(4)}")


# "until end of turn, all creatures become black and all lands become swamps"
# — Nightcreep.
@_er(r"^until end of turn, all creatures become ([a-z]+) and all lands become "
     r"([a-z]+)(?:s)?(?:\.|$)")
def _nightcreep(m):
    return UnknownEffect(raw_text=f"EOT: creatures={m.group(1)}, lands={m.group(2)}")


# "until end of turn, target creature loses all abilities and becomes a blue
# Frog with base power and toughness 1/1" — Turn to Frog.
@_er(r"^until end of turn, target creature loses all abilities and becomes "
     r"a(?:n)? ([a-z ]+?) with base power and toughness (\d+)/(\d+)(?:\.|$)")
def _turn_to_frog(m):
    return UnknownEffect(raw_text=f"EOT: becomes {m.group(1)} {m.group(2)}/{m.group(3)}")


# "until end of turn, any number of target creatures you control each get
# +N/+M and gain 'When this creature dies, draw a card.'" — Rabid Attack.
@_er(r"^until end of turn, any number of target creatures you control each "
     r"get ([+-]\d+)/([+-]\d+) and gain \"[^\"]+\"(?:\.|$)")
def _rabid_attack(m):
    return UnknownEffect(raw_text=f"EOT any# yours {m.group(1)}/{m.group(2)} + grant")


# "until end of turn, up to two target creatures you control each gain
# 'Whenever this creature deals combat damage to a player, draw a card.'" —
# Warriors' Lesson.
@_er(r"^until end of turn, up to (one|two|three|\d+) target creatures you "
     r"control each gain \"[^\"]+\"(?:\.|$)")
def _warriors_lesson(m):
    return UnknownEffect(raw_text=f"EOT up to {m.group(1)} grant quoted kw")


# "until end of turn, permanents your opponents control gain '...'" —
# Hellish Rebuke.
@_er(r"^until end of turn, permanents your opponents control gain \"[^\"]+\""
     r"(?:\.|$)")
def _hellish_rebuke(m):
    return UnknownEffect(raw_text="EOT: opp perms gain quoted tail")


# "until end of turn, creatures you control gain menace and '...'" —
# Predators' Hour.
@_er(r"^until end of turn, creatures you control gain ([a-z]+) and \"[^\"]+\""
     r"(?:\.|$)")
def _predators_hour(m):
    return UnknownEffect(raw_text=f"EOT: your creatures gain {m.group(1)} + quoted")


# ---------------------------------------------------------------------------
# Target-cant-attack-or-block / off-balance style denials.
# ---------------------------------------------------------------------------

# "target creature can't attack or block this turn" — Off Balance.
@_er(r"^target creature can'?t attack or block this turn(?:\.|$)")
def _off_balance(m):
    return UnknownEffect(raw_text="target can't attack or block")


# "nonartifact creatures can't block this turn" — Ruthless Invasion body.
@_er(r"^nonartifact creatures can'?t block this turn(?:\.|$)")
def _nonartifact_cant_block(m):
    return UnknownEffect(raw_text="nonartifact creatures can't block this turn")


# "target opponent can't cast instant or sorcery spells during that player's
# next turn" — Sphinx's Decree (and "each opponent" variant).
@_er(r"^(?:target opponent|each opponent) can'?t cast instant or sorcery spells"
     r" during (?:that|their) player'?s next turn(?:\.|$)")
def _sphinx_decree(m):
    return UnknownEffect(raw_text="opp can't cast I/S next turn")


# "target player can't play lands this turn if {R} was spent to cast this
# spell and can't cast creature spells this turn if {W} was spent ..." —
# Moonhold (handled by generic "if color was spent" rule above, but keep
# specific fallback).
@_er(r"^target player can'?t play lands this turn if \{[wubrg]\} was spent to "
     r"cast this spell and can'?t cast creature spells this turn if \{[wubrg]\} "
     r"was spent to cast this spell(?:\.|$)")
def _moonhold(m):
    return UnknownEffect(raw_text="moonhold: R→no-lands / W→no-creatures")


# ---------------------------------------------------------------------------
# Combat-math quirks.
# ---------------------------------------------------------------------------

# "target creature an opponent controls deals damage equal to its power to
# each other creature that player controls, then each of those creatures
# deals damage equal to its power to that creature" — Alpha Brawl.
@_er(r"^target creature an opponent controls deals damage equal to its power "
     r"to each other creature that player controls, then each of those "
     r"creatures deals damage equal to its power to that creature(?:\.|$)")
def _alpha_brawl(m):
    return UnknownEffect(raw_text="alpha brawl: 1 vs all, then all vs 1")


# "two target creatures your team controls each deal damage equal to their
# power to target creature" — Combo Attack.
@_er(r"^two target creatures your (?:team controls|control) each deal damage "
     r"equal to their power to target creature(?:\.|$)")
def _combo_attack(m):
    return UnknownEffect(raw_text="two target fight target")


# "up to two target creatures you control each deal damage equal to their
# power to another target creature" — Band Together.
@_er(r"^up to (?:two|three|\d+) target creatures you control each deal damage "
     r"equal to their power to another target creature(?:\.|$)")
def _band_together(m):
    return UnknownEffect(raw_text="up-to-2 gang up on target")


# "up to two target creatures you control each get +1/+0 until end of turn.
# they each deal damage equal to their power to another target creature,
# planeswalker, or battle" — Tandem Takedown.
@_er(r"^up to (?:two|three|\d+) target creatures you control each get "
     r"([+-]\d+)/([+-]\d+) until end of turn\. they each deal damage equal to "
     r"their power to another target [^.]+(?:\.|$)")
def _tandem_takedown(m):
    return UnknownEffect(raw_text=f"up-to-2 buff + gang up")


# "target creature you control fights target creature the opponent to your
# left controls. then that player may copy this spell ..." — Barroom Brawl.
@_er(r"^target creature you control fights target creature the opponent to "
     r"your left controls\. then that player may copy this spell(?: and [^.]+)?"
     r"(?:\.|$)")
def _barroom_brawl(m):
    return UnknownEffect(raw_text="fight opp-left + pass-copy")


# "target blocking creature fights another target blocking creature" —
# Dissension in the Ranks.
@_er(r"^target blocking creature fights another target blocking creature(?:\.|$)")
def _dissension_ranks(m):
    return UnknownEffect(raw_text="blocking fights blocking")


# "target creature you control with a +1/+1 counter on it fights target
# creature an opponent controls" — Mutant's Prey.
@_er(r"^target creature you control with a [+-]\d+/[+-]\d+ counter on it "
     r"fights target creature an opponent controls(?:\.|$)")
def _mutants_prey(m):
    return UnknownEffect(raw_text="own-with-counter fights opp")


# "target creature you control deals X damage to any other target and X damage
# to itself, where X is its power" — Self-Destruct.
@_er(r"^target creature you control deals x damage to any other target and "
     r"x damage to itself, where x is its power(?:\.|$)")
def _self_destruct(m):
    return UnknownEffect(raw_text="self-destruct: own deals P to target + self")


# "target creature you control and up to one other target legendary creature
# you control each deal damage equal to their power to target creature you
# don't control" — Friendly Rivalry.
@_er(r"^target creature you control and up to (?:one|two|\d+) other target "
     r"(?:legendary |)creature(?:s)? you control each deal damage equal to "
     r"(?:its|their) power to target creature you don'?t control(?:\.|$)")
def _friendly_rivalry(m):
    return UnknownEffect(raw_text="own + legendary gang up")


# "any number of target enchanted creatures you control and up to one other
# target creature you control each deal damage equal to their power to
# target creature you don't control" — Graceful Takedown.
@_er(r"^any number of target (?:enchanted )?creatures? you control and up to "
     r"(?:one|two|\d+) other target creatures? you control each deal damage "
     r"equal to their power to target creature you don'?t control(?:\.|$)")
def _graceful_takedown(m):
    return UnknownEffect(raw_text="anynum gang up")


# ---------------------------------------------------------------------------
# Misc / hard-to-cluster oddballs.
# ---------------------------------------------------------------------------

# "unless target player pays {N}, that player loses N life and you gain N life"
# — Rhystic Syphon.
@_er(r"^unless target player pays \{[^}]+\}, that player loses (\d+) life and"
     r" you gain (\d+) life(?:\.|$)")
def _rhystic_syphon(m):
    return Sequence(items=(
        LoseLife(amount=int(m.group(1)), target=TARGET_PLAYER),
        GainLife(amount=int(m.group(2))),
    ))


# "target creature gains shroud until end of turn and can't be blocked this
# turn" — Veil of Secrecy front half (before splice text).
@_er(r"^target creature gains shroud until end of turn and can'?t be blocked "
     r"this turn(?:\.|$)")
def _veil_secrecy(m):
    return UnknownEffect(raw_text="target shroud + unblockable EOT")


# "destroy target noncreature permanent. then that permanent's controller may
# copy this spell and may choose a new target for that copy" — Chain of Acid /
# Chain of Vapor family.
@_er(r"^destroy target [a-z ]+ permanent\. then that permanent'?s controller "
     r"may copy this spell and may choose a new target for that copy(?:\.|$)")
def _chain_of_acid(m):
    return UnknownEffect(raw_text="chain: destroy + pass-copy")


# "each player shuffles the cards from their hand into their library, then
# draws that many cards" — Winds of Change.
@_er(r"^each player shuffles the cards from their hand into their library,? "
     r"then draws that many cards(?:\.|$)")
def _winds_of_change(m):
    return UnknownEffect(raw_text="hand shuffle + redraw")


# "each player may reveal any number of creature cards from their hand. then
# each player creates a 2/2 green Bear creature token for each card they
# revealed this way" — Kamahl's Summons.
@_er(r"^each player may reveal any number of creature cards? from their hand"
     r"\. then each player creates (?:a )?(\d+)/(\d+) [^.]+ creature tokens? "
     r"for each card they revealed this way(?:\.|$)")
def _kamahl_summons(m):
    return UnknownEffect(raw_text=f"kamahl: reveal→{m.group(1)}/{m.group(2)} token each")


# "each player may put an artifact, creature, enchantment, or land card from
# their hand onto the battlefield" — Show and Tell.
@_er(r"^each player may put an artifact, creature, enchantment, or land card "
     r"from their hand onto the battlefield(?:\.|$)")
def _show_and_tell(m):
    return UnknownEffect(raw_text="each-player put perm from hand bf")


# "each player chooses from the lands they control a land of each basic land
# type, then sacrifices the rest" — Global Ruin.
@_er(r"^each player chooses from the lands they control a land of each basic "
     r"land type, then sacrifices the rest(?:\.|$)")
def _global_ruin(m):
    return UnknownEffect(raw_text="each-player keep 5 basics, sac rest")


# "each player chooses from among the permanents they control an artifact, a
# creature, an enchantment, and a land, then sacrifices the rest" — Cataclysm.
@_er(r"^each player chooses from among the permanents they control an artifact,"
     r" a creature, an enchantment,? and a land, then sacrifices the rest"
     r"(?:\.|$)")
def _cataclysm(m):
    return UnknownEffect(raw_text="cataclysm: keep 1 each, sac rest")


# "each player shuffles all permanents they own into their library, then
# reveals that many cards ..." — Warp World (heavy; catch-all).
@_er(r"^each player shuffles all permanents they own into their library,? then"
     r" reveals that many cards[^.]+(?:\.|$)")
def _warp_world(m):
    return UnknownEffect(raw_text="warp world variant")


# "each player reveals the top five cards of their library, puts all land
# cards revealed this way onto the battlefield tapped, and exiles the rest"
# — Clear the Land.
@_er(r"^each player reveals the top (?:\w+|\d+) cards? of their library, puts"
     r" all land cards revealed this way onto the battlefield tapped, and "
     r"exiles the rest(?:\.|$)")
def _clear_the_land(m):
    return UnknownEffect(raw_text="clear the land: reveal top N, lands bf, rest exile")


# "each player may search their library for up to X basic land cards and put
# them onto the battlefield tapped..." — New Frontiers.
@_er(r"^each player may search their library for up to x basic land cards? "
     r"and put them onto the battlefield tapped[^.]*(?:\.|$)")
def _new_frontiers(m):
    return UnknownEffect(raw_text="each-player search up to X basic lands")


# "each player sacrifices all artifacts, enchantments, and nonbasic lands ..."
# — Wave of Vitriol family.
@_er(r"^each player sacrifices all artifacts, enchantments,? and nonbasic "
     r"lands they control[^.]*(?:\.|$)")
def _wave_vitriol(m):
    return UnknownEffect(raw_text="wave of vitriol variant")


# "each player who controls six or more lands chooses five lands ..." —
# Natural Balance.
@_er(r"^each player who controls (?:six|\w+) or more lands chooses (?:five|\w+)"
     r" lands they control and sacrifices the rest[^.]*(?:\.|$)")
def _natural_balance(m):
    return UnknownEffect(raw_text="natural balance: cap lands at 5")


# "each attacking creature gets +1/+0 until end of turn for each nonbasic land
# defending player controls" — Mercadia's Downfall.
@_er(r"^each attacking creature gets ([+-]\d+)/([+-]\d+) until end of turn for "
     r"each [^.]+(?:\.|$)")
def _mercadias_downfall(m):
    return UnknownEffect(raw_text=f"each attacker {m.group(1)}/{m.group(2)} × each X")


# "two target players exchange life totals. you create an X/X colorless Horror
# artifact creature token, where X is the difference ..." — Profane Transfusion.
@_er(r"^two target players exchange life totals\. you create an? x/x [^.]+"
     r" token, where x is the difference[^.]+(?:\.|$)")
def _profane_transfusion(m):
    return UnknownEffect(raw_text="profane transfusion")


# "copy target creature spell you control, except it isn't legendary if the
# spell is legendary" — Double Major.
@_er(r"^copy target (?:creature|instant|sorcery|spell) spell you control, "
     r"except it isn'?t legendary if the spell is legendary(?:\.|$)")
def _double_major(m):
    return UnknownEffect(raw_text="copy spell except legendary strip")


# "return target permanent to its owner's hand if that permanent shares a
# color with the most common color ..." — Barrin's Unmaking.
@_er(r"^return target permanent to its owner'?s hand if that permanent shares "
     r"a color with the most common color[^.]+(?:\.|$)")
def _barrins_unmaking(m):
    return UnknownEffect(raw_text="bounce if shares color with mode-color")


# "choose any number of target creatures. each of those creatures gains
# persist until end of turn" — Cauldron Haze.
@_er(r"^choose any number of target creatures\. each of those creatures "
     r"gains ([a-z]+) until end of turn(?:\.|$)")
def _cauldron_haze(m):
    return UnknownEffect(raw_text=f"any# creatures gain {m.group(1)} EOT")


# "put a random creature card with mana value X or less from target
# opponent's library onto the battlefield under your control" — Better Offer.
@_er(r"^put a random creature card with mana value x or less from target "
     r"opponent'?s library onto the battlefield under your control[^.]*(?:\.|$)")
def _better_offer(m):
    return UnknownEffect(raw_text="better offer: steal random from opp lib")


# "put target creature card from your graveyard onto the battlefield" — base
# parser has this but not when name was scrubbed.
# (Already handled as _return_target_gy_hand above for 'hand'; battlefield)

# "put a +1/+1 counter and a lifelink counter on target creature" —
# Unexpected Fangs.
@_er(r"^put a ([+-]\d+/[+-]\d+|\w+) counter and an? ([a-z]+) counter on target "
     r"creature(?:\.|$)")
def _two_counter_kinds(m):
    return UnknownEffect(raw_text=f"put {m.group(1)} + {m.group(2)} counter")


# "look at the top ten cards of your library, exile up to two creature cards
# from among them, then shuffle. target opponent may choose ..." —
# Dubious Challenge.
@_er(r"^look at the top (?:ten|\d+) cards? of your library, exile up to "
     r"(?:two|three|\d+) creature cards? from among them, then shuffle\."
     r" target opponent may choose [^.]+(?:\.|$)")
def _dubious_challenge(m):
    return UnknownEffect(raw_text="dubious challenge: opp picks 1")


# "choose two target players. each of them searches their library for a card,
# then shuffles and puts that card on top" — Scheming Symmetry.
@_er(r"^choose two target players\. each of them searches their library for "
     r"a card, then shuffles and puts that card on top(?:\.|$)")
def _scheming_symmetry(m):
    return UnknownEffect(raw_text="scheming symmetry")


# "exile three random cards from your library face down and look at them.
# for as long as they remain exiled, you may play one of those cards" —
# Tezzeret's Reckoning.
@_er(r"^exile (?:three|\d+) random cards? from your library face down and "
     r"look at them\. for as long as they remain exiled, you may play one of "
     r"those cards(?:\.|$)")
def _tezzerets_reckoning(m):
    return UnknownEffect(raw_text="exile 3 random + play one")


# "target opponent draws two cards, then you draw up to four cards" —
# Trade Secrets front half.
@_er(r"^target opponent draws (?:two|three|\d+) cards?, then you draw up to "
     r"(?:four|three|\d+) cards?[^.]*(?:\.|$)")
def _trade_secrets(m):
    return UnknownEffect(raw_text="opp draws then you draw more")


# "the bidding ends if the high bid stands..." family — Illicit Auction /
# Pain's Reward: multi-paragraph bid mechanics; just tag.
@_er(r"^each player may bid life(?: for control of target creature)?\."
     r" you start the bidding[^.]+\.[^.]+\.(?:[^.]+\.)*")
def _bidding_mechanic(m):
    return UnknownEffect(raw_text="bidding mechanic")


# "search your library for an instant card with mana value 3, reveal it, and
# put it into your hand. then repeat this process for instant cards with
# mana values 2 and 1..." — Firemind's Foresight.
@_er(r"^search your library for an instant card with mana value \d+, reveal "
     r"it,? and put it into your hand\.[^.]+repeat[^.]+(?:\.|$)")
def _firemind_foresight(m):
    return UnknownEffect(raw_text="firemind's foresight chain tutor")


# ---------------------------------------------------------------------------
# "Cast this spell only..." - timing gates.
# ---------------------------------------------------------------------------

@_er(r"^this turn and next turn, creatures can'?t attack, and players and "
     r"permanents can'?t be the targets of spells or activated abilities(?:\.|$)")
def _peace_talks(m):
    return UnknownEffect(raw_text="peace talks: 2-turn no-attack/no-target")


# ---------------------------------------------------------------------------
# Remorseless Punishment / villainous choice cascades.
# ---------------------------------------------------------------------------

@_er(r"^target opponent loses (\d+) life unless that player discards "
     r"(?:two|three|\d+) cards? or sacrifices a creature or planeswalker of "
     r"their choice\. repeat this process once(?:\.|$)")
def _remorseless_punishment(m):
    return UnknownEffect(raw_text="remorseless punishment")


# ---------------------------------------------------------------------------
# "Create X 1/1 red Goblin creature tokens" — name-scrubbed simple token
# spells where base patterns failed due to "~" substitution.
# ---------------------------------------------------------------------------

# "create X 1/1 red ~ creature tokens" — Goblin Offensive / Goblin Rally /
# Goblin Scouts / Knight Watch / Servo Exhibition / Inkling Summoning /
# Spirit Summoning / Elemental Summoning / Pest Summoning / Goblin Wizardry.
@_er(r"^create (a|one|two|three|four|five|x|\d+) "
     r"(\d+)/(\d+) "
     r"((?:~|[a-z]+)(?:(?: and (?:~|[a-z]+))+)?) "
     r"(?:~ )?(?:[a-z]+ )*"
     r"(?:artifact )?creature tokens?"
     r"(?: with [^.]+)?(?:\.|$)")
def _create_token_catchall(m):
    # Broad catch-all: just record the token creation. Downstream consumers
    # can re-parse the raw text if they need the exact subtype/keywords.
    return CreateToken(count=_num(m.group(1)),
                       power=int(m.group(2)), toughness=int(m.group(3)),
                       color=(m.group(4),),
                       creature_type=(),
                       raw=m.group(0))


# "create two 1/1 blue Fish creature tokens with 'This token can't be
# blocked.' Then for each kind of counter among creatures you control, put a
# counter of that kind on either of those tokens" — Exotic Pets.
@_er(r"^create (?:two|three|\d+) \d+/\d+ [^.]+ creature tokens? with \"[^\"]+\""
     r"\. then for each kind of counter among [^.]+(?:\.|$)")
def _exotic_pets(m):
    return UnknownEffect(raw_text="exotic pets: tokens + sprinkle counters")


# ---------------------------------------------------------------------------
# Very short action-only sentences (one-word fragments after name scrub).
# ---------------------------------------------------------------------------

# "~walk" — Mountain Goat ("mountainwalk" became "~walk" because "Mountain" is
# part of the card name). Single-token landwalk keyword.
@_er(r"^~walk(?:\.|$)")
def _scrubbed_landwalk(m):
    return Keyword(name="landwalk", args=("~-scrubbed",), raw="~walk")


# "~" alone (extremely rare edge case).
@_er(r"^~$")
def _just_tilde(m):
    return UnknownEffect(raw_text="scrubbed-only text")


# ===========================================================================
# STATIC_PATTERNS — ability-level statics (used by parse_static).
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Anthems / tribal statics we missed.
# ---------------------------------------------------------------------------

# "nontoken creatures you control get +N/+N and have <kw>" — Always Watching.
@_sp(r"^nontoken creatures you control get ([+-]\d+)/([+-]\d+) and have "
     r"([a-z]+)\s*$")
def _nontoken_yours_anthem(m, raw):
    return Static(modification=Modification(
        kind="nontoken_yours_anthem",
        args=(int(m.group(1)), int(m.group(2)), m.group(3))),
        raw=raw)


# "<TYPE> creatures you control get +N/+N and gain <kw>" — Vampiric Fury.
@_sp(r"^([a-z]+) creatures you control get ([+-]\d+)/([+-]\d+) and gain "
     r"([a-z ]+?) until end of turn\s*$")
def _tribe_anthem_eot_gain(m, raw):
    return Static(modification=Modification(
        kind="tribe_anthem_eot_gain",
        args=(m.group(1), int(m.group(2)), int(m.group(3)), m.group(4).strip())),
        raw=raw)


# "<TYPE> creatures get +N/+N and have <kw>" — Goblin King-style.
@_sp(r"^([a-z]+) creatures get ([+-]\d+)/([+-]\d+) and have ([a-z ]+?)\s*$")
def _tribe_anthem_have(m, raw):
    return Static(modification=Modification(
        kind="tribe_anthem_have",
        args=(m.group(1), int(m.group(2)), int(m.group(3)), m.group(4).strip())),
        raw=raw)


# "other <TYPE> get +N/+N and have <kw>" — Lord of Atlantis / Goblin King
# (after name scrub "other ~s" / "other merfolk").
@_sp(r"^other (?:~|~s|[a-z]+)(?: creatures)? get ([+-]\d+)/([+-]\d+) "
     r"and have ([a-z ]+?)\s*$")
def _other_tribe_anthem(m, raw):
    return Static(modification=Modification(
        kind="other_tribe_anthem",
        args=(int(m.group(1)), int(m.group(2)), m.group(3).strip())),
        raw=raw)


# "other <TYPE> (creatures )?you control get +N/+N" — Squirrel Sovereign,
# Merfolk Mistbinder.
@_sp(r"^other (?:~|~s|[a-z]+)(?: creatures)? you control get ([+-]\d+)/"
     r"([+-]\d+)\s*$")
def _other_yours_anthem(m, raw):
    return Static(modification=Modification(
        kind="other_yours_anthem",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "other creatures get -N/-N" — Kaervek, the Spiteful.
@_sp(r"^other creatures get ([+-]\d+)/([+-]\d+)\s*$")
def _other_creatures_global(m, raw):
    return Static(modification=Modification(
        kind="other_creatures_global_pt",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "<TYPE> creatures get +N/+0" — Anaba Spirit Crafter.
@_sp(r"^([a-z]+) creatures get ([+-]\d+)/([+-]\d+)\s*$")
def _tribe_global_pt(m, raw):
    return Static(modification=Modification(
        kind="tribe_global_pt",
        args=(m.group(1), int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "<TYPE> creatures you control get +N/+N" — Lord of the Unreal /
# Lord of Atlantis variants.
@_sp(r"^([a-z]+) creatures you control get ([+-]\d+)/([+-]\d+) and have "
     r"([a-z ]+?)\s*$")
def _tribe_yours_anthem_have(m, raw):
    return Static(modification=Modification(
        kind="tribe_yours_anthem_have",
        args=(m.group(1), int(m.group(2)), int(m.group(3)), m.group(4).strip())),
        raw=raw)


# "commander creatures you control get +N/+N and have <kw>" —
# Bastion Protector.
@_sp(r"^commander creatures you (?:control|own) get ([+-]\d+)/([+-]\d+) and "
     r"have ([a-z ]+?)\s*$")
def _commander_anthem(m, raw):
    return Static(modification=Modification(
        kind="commander_anthem",
        args=(int(m.group(1)), int(m.group(2)), m.group(3).strip())),
        raw=raw)


# "<tribe1> and <tribe2> you control get +N/+N" — Valley Questcaller second
# line.
@_sp(r"^other ([a-z]+), ([a-z]+),? and ([a-z]+) (?:creatures )?you control "
     r"get ([+-]\d+)/([+-]\d+)\s*$")
def _tri_tribe_anthem(m, raw):
    return Static(modification=Modification(
        kind="tri_tribe_anthem",
        args=(m.group(1), m.group(2), m.group(3),
              int(m.group(4)), int(m.group(5)))),
        raw=raw)


# "creatures you control get +N/-N" — Flowstone Surge (shadow/anti-anthem).
@_sp(r"^creatures you control get ([+-]\d+)/([+-]\d+)\s*$")
def _your_creatures_anthem_bare(m, raw):
    return Static(modification=Modification(
        kind="your_creatures_anthem_bare",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "non-<type> creatures get -N/-N until end of turn" — Eyeblight Massacre
# (name-scrub variant).
@_sp(r"^non-?([a-z]+) creatures? get ([+-]\d+)/([+-]\d+)"
     r"(?: until end of turn)?\s*$")
def _non_type_global_pt(m, raw):
    return Static(modification=Modification(
        kind="non_type_global_pt",
        args=(m.group(1), int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "each non-<type> creature gets -X/-X until end of turn, where X is ..." —
# Olivia's Wrath.
@_sp(r"^each non-?([a-z]+) creature gets -x/-x until end of turn, where x is"
     r" [^.]+\s*$")
def _each_non_type_xx(m, raw):
    return Static(modification=Modification(
        kind="each_non_type_xx",
        args=(m.group(1),)), raw=raw)


# "all creatures get -1/-1 until end of turn for each swamp you control" —
# Mutilate.
@_sp(r"^all creatures get ([+-]\d+)/([+-]\d+) until end of turn for each "
     r"[a-z]+ you control\s*$")
def _mutilate_like(m, raw):
    return Static(modification=Modification(
        kind="mutilate_like",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "creatures your opponents control have base power and toughness N/M until
# end of turn" — Flatline.
@_sp(r"^creatures your opponents control have base power and toughness "
     r"(\d+)/(\d+)(?: until end of turn)?\s*$")
def _opp_creatures_base_pt(m, raw):
    return Static(modification=Modification(
        kind="opp_creatures_base_pt",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "creatures your opponents control get -N/-N until end of turn" —
# Turn the Tide (scrubbed: "until end of ~").
@_sp(r"^creatures your opponents control get ([+-]\d+)/([+-]\d+)"
     r"(?: until end of (?:turn|~))?\s*$")
def _opp_creatures_pt(m, raw):
    return Static(modification=Modification(
        kind="opp_creatures_pt",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# ---------------------------------------------------------------------------
# "All <type> are P/P creatures that are still lands" — Kormus Bell family.
# ---------------------------------------------------------------------------

@_sp(r"^all (?:lands|forests|swamps|mountains|islands|plains) are (\d+)/(\d+) "
     r"(?:[a-z]+ )?creatures that are still lands\s*$")
def _lands_are_creatures(m, raw):
    return Static(modification=Modification(
        kind="lands_are_creatures",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "each land gets +N/+N as long as it's a creature" — Earth Surge.
@_sp(r"^each land gets ([+-]\d+)/([+-]\d+) as long as it'?s a creature\s*$")
def _land_pump_if_creature(m, raw):
    return Static(modification=Modification(
        kind="land_pump_if_creature",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "all <TYPE> creatures get +N/+N for each other <TYPE> on the battlefield" —
# Sliver Legion (post name-scrub).
@_sp(r"^all ~ creatures get ([+-]\d+)/([+-]\d+) for each other ~ on the "
     r"battlefield\s*$")
def _sliver_legion(m, raw):
    return Static(modification=Modification(
        kind="sliver_legion_scaling",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# ---------------------------------------------------------------------------
# Spell-cost / activation statics.
# ---------------------------------------------------------------------------

# "creature spells you cast cost {N} more to cast" — Geist-Fueled Scarecrow.
@_sp(r"^creature spells you cast cost \{(\d+)\} more to cast\s*$")
def _creature_spells_cost_more(m, raw):
    return Static(modification=Modification(
        kind="creature_spells_cost_more",
        args=(int(m.group(1)),)),
        raw=raw)


# "noncreature spells with mana value N or greater can't be cast" — Gaddock.
@_sp(r"^noncreature spells with mana value (\d+) or greater can'?t be cast\s*$")
def _gaddock_noncreature(m, raw):
    return Static(modification=Modification(
        kind="nc_spells_mv_cant_cast",
        args=(int(m.group(1)),)),
        raw=raw)


# "noncreature spells with {X} in their mana costs can't be cast" — Gaddock.
@_sp(r"^noncreature spells with \{x\} in their mana costs can'?t be cast\s*$")
def _gaddock_x(m, raw):
    return Static(modification=Modification(
        kind="nc_x_spells_cant_cast"),
        raw=raw)


# "activated abilities of nontoken <type> cost an additional 'sacrifice a
# land' to activate" — Brutal Suppression.
@_sp(r"^activated abilities of nontoken ([a-z]+) cost an additional \"[^\"]+\""
     r" to activate\s*$")
def _activated_extra_cost(m, raw):
    return Static(modification=Modification(
        kind="activated_extra_cost",
        args=(m.group(1),)),
        raw=raw)


# "each player who has cast a nonartifact spell this turn can't cast
# additional nonartifact spells" — Ethersworn Canonist.
@_sp(r"^each player who has cast a ([a-z]+) spell this turn can'?t cast "
     r"additional \1 spells\s*$")
def _canonist(m, raw):
    return Static(modification=Modification(
        kind="one_per_turn_filter",
        args=(m.group(1),)),
        raw=raw)


# "colorless spells you cast from your hand with mana value N or greater have
# 'cascade, cascade.'" — Zhulodok.
@_sp(r"^colorless spells you cast from your hand with mana value (\d+) or "
     r"greater have \"cascade, cascade\.?\"\s*$")
def _zhulodok(m, raw):
    return Static(modification=Modification(
        kind="double_cascade_grant",
        args=(int(m.group(1)),)),
        raw=raw)


# ---------------------------------------------------------------------------
# "You may / players may / any player may" statics.
# ---------------------------------------------------------------------------

# "you may look at the top card of your library and at face-down creatures
# you don't control any time" — Lens of Clarity.
@_sp(r"^you may look at (?:the top card of your library and at )?"
     r"face-down creatures you don'?t control any time\s*$")
def _lens_clarity(m, raw):
    return Static(modification=Modification(
        kind="peek_fd_and_topdeck"),
        raw=raw)


# "you may activate equip abilities any time you could cast an instant" —
# Leonin Shikari.
@_sp(r"^you may activate equip abilities any time you could cast an instant"
     r"\s*$")
def _shikari(m, raw):
    return Static(modification=Modification(
        kind="equip_at_instant_speed"),
        raw=raw)


# "you may cast aura spells with enchant creature as though they had flash"
# — Rootwater Shaman.
@_sp(r"^you may cast aura spells with enchant creature as though they had "
     r"flash\s*$")
def _aura_flash(m, raw):
    return Static(modification=Modification(
        kind="aura_flash"),
        raw=raw)


# "you may spend white mana as though it were red mana" — Sunglasses of Urza.
@_sp(r"^you may spend ([a-z]+) mana as though it were ([a-z]+) mana\s*$")
def _sub_mana(m, raw):
    return Static(modification=Modification(
        kind="sub_mana",
        args=(m.group(1), m.group(2))),
        raw=raw)


# "any player may cast creature spells with mana value 3 or less without
# paying their mana costs and as though they had flash" — Aluren.
@_sp(r"^any player may cast creature spells with mana value (\d+) or less "
     r"without paying their mana costs and as though they had flash\s*$")
def _aluren(m, raw):
    return Static(modification=Modification(
        kind="aluren",
        args=(int(m.group(1)),)),
        raw=raw)


# "rather than pay the mana cost for a spell, its controller may discard a
# card that shares a color with that spell" — Dream Halls.
@_sp(r"^rather than pay the mana cost for a spell, its controller may discard "
     r"a card that shares a color with that spell\s*$")
def _dream_halls(m, raw):
    return Static(modification=Modification(
        kind="dream_halls"),
        raw=raw)


# "during each of your turns, you may play a land and cast a permanent spell
# of each permanent type from your graveyard" — Muldrotha.
@_sp(r"^during each of your turns, you may play a land and cast a permanent "
     r"spell of each permanent type from your graveyard\s*$")
def _muldrotha(m, raw):
    return Static(modification=Modification(
        kind="muldrotha"),
        raw=raw)


# "while voting, you get an additional vote" — Brago's Representative /
# Ballot Broker.
@_sp(r"^while voting, you (?:get an additional vote|may vote an additional "
     r"time)\s*$")
def _voting_extra(m, raw):
    return Static(modification=Modification(
        kind="vote_extra"),
        raw=raw)


# "wall creatures can attack as though they didn't have defender" —
# Rolling Stones.
@_sp(r"^wall creatures can attack as though they didn'?t have defender\s*$")
def _rolling_stones(m, raw):
    return Static(modification=Modification(
        kind="walls_can_attack"),
        raw=raw)


# "buyback costs cost {N} less" — Memory Crystal.
@_sp(r"^buyback costs cost \{(\d+)\} less\s*$")
def _memory_crystal(m, raw):
    return Static(modification=Modification(
        kind="buyback_less",
        args=(int(m.group(1)),)),
        raw=raw)


# "the 'legend rule' doesn't apply" — Mirror Gallery.
@_sp(r"^the \"legend rule\" doesn'?t apply\s*$")
def _mirror_gallery(m, raw):
    return Static(modification=Modification(
        kind="no_legend_rule"),
        raw=raw)


# "all creatures with flying able to block this creature do so" — Talruum
# Piper (scrubbed: this creature → ~).
@_sp(r"^all creatures with flying able to block (?:this creature|~) do so\s*$")
def _talruum_piper(m, raw):
    return Static(modification=Modification(
        kind="forced_block_flyers"),
        raw=raw)


# "creatures with landwalk abilities can be blocked as though they didn't
# have those abilities" — Staff of the Ages.
@_sp(r"^creatures with landwalk abilities can be blocked as though they "
     r"didn'?t have those abilities\s*$")
def _staff_of_ages(m, raw):
    return Static(modification=Modification(
        kind="strip_landwalk_for_blocking"),
        raw=raw)


# "creatures you control can't be blocked by more than one creature" —
# Familiar Ground-style (generic and with ~ scrub).
@_sp(r"^each creature you control can'?t be blocked by more than one creature"
     r"\s*$")
def _familiar_ground(m, raw):
    return Static(modification=Modification(
        kind="yours_cap_blockers_1"),
        raw=raw)


# "this creature can block creatures with shadow as though it had shadow" —
# Heartwood Dryad.
@_sp(r"^(?:this creature|~) can block creatures with ([a-z]+) as though it "
     r"had \1\s*$")
def _block_as_though(m, raw):
    return Static(modification=Modification(
        kind="block_as_though",
        args=(m.group(1),)),
        raw=raw)


# "this creature can't be destroyed by lethal damage unless lethal damage
# dealt by a single source is marked on it" — Ogre Enforcer.
@_sp(r"^(?:this creature|~) can'?t be destroyed by lethal damage unless lethal "
     r"damage dealt by a single source is marked on it\s*$")
def _ogre_enforcer(m, raw):
    return Static(modification=Modification(
        kind="ogre_enforcer_no_aggregate_lethal"),
        raw=raw)


# "lands you control and land cards in your library are basic" —
# Rootpath Purifier.
@_sp(r"^lands you control and land cards in your library are basic\s*$")
def _rootpath(m, raw):
    return Static(modification=Modification(
        kind="your_lands_basic"),
        raw=raw)


# "basic lands each player controls have shroud as long as ..." —
# Sheltering Prayers.
@_sp(r"^basic lands each player controls have shroud as long as that player "
     r"controls [^.]+lands\s*$")
def _sheltering(m, raw):
    return Static(modification=Modification(
        kind="shroud_basics_while_few_lands"),
        raw=raw)


# "creatures you control are slivers in addition to their other creature
# types" — Hivestone.
@_sp(r"^creatures you control are [a-z]+ in addition to their other creature "
     r"types\s*$")
def _hivestone(m, raw):
    return Static(modification=Modification(
        kind="type_add_yours"),
        raw=raw)


# "all lands are 2/2 creatures that are still lands" — Nature's Revolt /
# Living Plane.
@_sp(r"^all lands are (\d+)/(\d+) creatures that are still lands\s*$")
def _lands_are_creatures_generic(m, raw):
    return Static(modification=Modification(
        kind="all_lands_creatures",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "all forests are 1/1 creatures that are still lands" — Living Lands.
@_sp(r"^all ([a-z]+s) are (\d+)/(\d+) (?:[a-z]+ )*creatures that are still "
     r"lands\s*$")
def _typed_lands_creatures(m, raw):
    return Static(modification=Modification(
        kind="typed_lands_creatures",
        args=(m.group(1), int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "each noncreature artifact is an artifact creature with power and toughness
# each equal to its mana value" — March of the Machines.
@_sp(r"^each noncreature artifact is an artifact creature with power and "
     r"toughness each equal to its mana value\s*$")
def _march_machines(m, raw):
    return Static(modification=Modification(
        kind="march_of_machines"),
        raw=raw)


# "each other non-aura enchantment is a creature ... and has base power and
# base toughness each equal to its mana value" — Opalescence.
@_sp(r"^each other non-aura enchantment is a creature in addition to its other"
     r" types and has base power and base toughness each equal to its mana "
     r"value\s*$")
def _opalescence(m, raw):
    return Static(modification=Modification(
        kind="opalescence"),
        raw=raw)


# "each noncreature, non-equipment artifact is an equipment with equip {X}
# and 'Equipped creature gets +X/+0,' where X is that artifact's mana value"
# — Bludgeon Brawl.
@_sp(r"^each noncreature, non-equipment artifact is an equipment with equip "
     r"\{x\} and \"[^\"]+\", where x is that artifact'?s mana value\s*$")
def _bludgeon_brawl(m, raw):
    return Static(modification=Modification(
        kind="bludgeon_brawl"),
        raw=raw)


# "each creature with the greatest mana value has protection from each color"
# — Favor of the Mighty.
@_sp(r"^each creature with the greatest mana value has protection from each "
     r"color\s*$")
def _favor_mighty(m, raw):
    return Static(modification=Modification(
        kind="greatest_mv_protection_all"),
        raw=raw)


# "nonartifact creatures get +N/+N as long as they all share a color" —
# Common Cause.
@_sp(r"^nonartifact creatures get ([+-]\d+)/([+-]\d+) as long as they all "
     r"share a color\s*$")
def _common_cause(m, raw):
    return Static(modification=Modification(
        kind="conditional_anthem_all_same_color",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "this creature gets +X/+0, where X is the greatest mana value among other
# artifacts you control" — Emissary Escort.
@_sp(r"^(?:this creature|~) gets \+x/\+0, where x is the greatest mana value "
     r"among (?:other )?[^.]+\s*$")
def _greatest_mv_self_pump(m, raw):
    return Static(modification=Modification(
        kind="greatest_mv_self_pump"),
        raw=raw)


# "this creature gets -X/-X, where X is your life total" — Death's Shadow.
@_sp(r"^(?:this creature|~) gets -x/-x, where x is your life total\s*$")
def _deaths_shadow(m, raw):
    return Static(modification=Modification(
        kind="deaths_shadow"),
        raw=raw)


# "this creature crews vehicles using its toughness rather than its power" —
# Giant Ox.
@_sp(r"^(?:this creature|~) crews vehicles using its toughness rather than "
     r"its power\s*$")
def _giant_ox(m, raw):
    return Static(modification=Modification(
        kind="crew_with_toughness"),
        raw=raw)


# "except for creatures named X and artifact creatures, creatures you control
# can't attack" — Akron Legionnaire.
@_sp(r"^except for creatures named ~ and artifact creatures, creatures you "
     r"control can'?t attack\s*$")
def _akron_legionnaire(m, raw):
    return Static(modification=Modification(
        kind="akron_legionnaire"),
        raw=raw)


# "each enchantment deals N damage to its controller, then each <type>
# attached to a creature deals N damage to the creature it's attached to" —
# Aura Barbs.
@_sp(r"^each enchantment deals (\d+) damage to its controller, then each ~ "
     r"attached to a creature deals (\d+) damage to the creature it'?s "
     r"attached to\s*$")
def _aura_barbs(m, raw):
    return Static(modification=Modification(
        kind="aura_barbs",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "untap this creature during each other player's untap step" — Thousand
# Moons Infantry.
@_sp(r"^untap (?:this creature|~) during each other player'?s untap step\s*$")
def _untap_each_opp_step(m, raw):
    return Static(modification=Modification(
        kind="untap_each_opp_step"),
        raw=raw)


# "untap each creature you control with a +1/+1 counter on it during each
# other player's untap step" — Ivorytusk Fortress.
@_sp(r"^untap each creature you control with a [+-]\d+/[+-]\d+ counter on it "
     r"during each other player'?s untap step\s*$")
def _untap_your_pumped(m, raw):
    return Static(modification=Modification(
        kind="untap_your_pumped_each_opp_step"),
        raw=raw)


# "this creature gets +N/+0 and has menace as long as you sacrificed a
# permanent this turn" — Goblin Blast-Runner.
@_sp(r"^(?:this creature|~) gets ([+-]\d+)/([+-]\d+) and has ([a-z ]+?) as "
     r"long as you sacrificed a permanent this turn\s*$")
def _goblin_blast_runner(m, raw):
    return Static(modification=Modification(
        kind="self_pump_with_kw_if_sacced",
        args=(int(m.group(1)), int(m.group(2)), m.group(3).strip())),
        raw=raw)


# "you win the game if you control a land of each basic land type and a
# creature of each color" — Coalition Victory.
@_sp(r"^you win the game if you control [^.]+\s*$")
def _coalition_victory(m, raw):
    return Static(modification=Modification(
        kind="win_on_condition"),
        raw=raw)


# "this spell costs 3 life more to cast for each target" — Phyrexian Purge.
@_sp(r"^this spell costs (\d+) life more to cast for each target\s*$")
def _phy_purge_cost(m, raw):
    return Static(modification=Modification(
        kind="self_life_cost_per_target",
        args=(int(m.group(1)),)),
        raw=raw)


# "spells you control can't be countered by blue or black spells this turn,
# and creatures you control can't be the targets of blue or black spells
# this turn" — Autumn's Veil.
@_sp(r"^spells you control can'?t be countered by [a-z ]+ spells this turn,"
     r" and creatures you control can'?t be the targets of [a-z ]+ spells "
     r"this turn\s*$")
def _autumns_veil(m, raw):
    return Static(modification=Modification(
        kind="autumns_veil"),
        raw=raw)


# "creatures you control enter as a copy of this creature" —
# Essence of the Wild.
@_sp(r"^creatures you control enter as a copy of (?:this creature|~)\s*$")
def _essence_wild(m, raw):
    return Static(modification=Modification(
        kind="etb_as_copy_of_self"),
        raw=raw)


# "all creatures with flying get -N/-N and lose flying until end of turn" —
# Wind Shear.
@_sp(r"^attacking creatures with flying get ([+-]\d+)/([+-]\d+) and lose "
     r"flying until end of turn\s*$")
def _wind_shear(m, raw):
    return Static(modification=Modification(
        kind="attacking_flyers_debuff_strip",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "you choose which creatures attack this turn" — Master Warcraft halves.
@_sp(r"^you choose which creatures attack this turn\s*$")
def _master_warcraft_attack(m, raw):
    return Static(modification=Modification(
        kind="replace_attack_chooser"),
        raw=raw)


@_sp(r"^you choose which creatures block this turn(?: and how those creatures "
     r"block)?\s*$")
def _master_warcraft_block(m, raw):
    return Static(modification=Modification(
        kind="replace_block_chooser"),
        raw=raw)


# "cast this spell only before attackers are declared" — Master Warcraft gate.
@_sp(r"^cast (?:this spell|~) only before attackers are declared\s*$")
def _cast_before_attackers(m, raw):
    return Static(modification=Modification(
        kind="cast_timing_before_attackers"),
        raw=raw)


# "you may have creatures you control assign their combat damage this turn
# as though they weren't blocked" — Predatory Focus.
@_sp(r"^you may have creatures you control assign their combat damage this "
     r"turn as though they weren'?t blocked\s*$")
def _predatory_focus(m, raw):
    return Static(modification=Modification(
        kind="trample_grant_mass_eot"),
        raw=raw)


# "rather than the attacking player, you assign the combat damage of each
# creature attacking you..." — Defensive Formation.
@_sp(r"^rather than the attacking player, you assign the combat damage of "
     r"each creature attacking you[^.]+\s*$")
def _defensive_formation(m, raw):
    return Static(modification=Modification(
        kind="defensive_formation"),
        raw=raw)


# "you may assign this creature's combat damage divided as you choose among
# defending player and/or any number of creatures they control" — Butcher Orgg.
@_sp(r"^you may assign (?:this creature'?s|~'?s) combat damage divided as you "
     r"choose[^.]+\s*$")
def _butcher_orgg(m, raw):
    return Static(modification=Modification(
        kind="division_damage_divided"),
        raw=raw)


# "you can't untap more than one land during your untap step" — Mungha Wurm.
@_sp(r"^you can'?t untap more than one land during your untap step\s*$")
def _mungha_wurm(m, raw):
    return Static(modification=Modification(
        kind="cap_own_untap"),
        raw=raw)


# "each creature spell you cast with mana value N or greater has blitz" —
# Henzie "Toolbox" Torre.
@_sp(r"^each creature spell you cast with mana value (\d+) or greater has "
     r"([a-z]+)\s*\.?(?:\s+the \2 cost is equal to its mana cost\.?)?\s*$")
def _henzie(m, raw):
    return Static(modification=Modification(
        kind="grant_mechanic_mv_threshold",
        args=(int(m.group(1)), m.group(2))),
        raw=raw)


# "other players can't play lands or cast spells from their graveyards this
# turn. you may play lands and cast spells from other players' graveyards
# this turn as though those cards were in your graveyard" — Shaman's Trance.
@_sp(r"^other players can'?t play lands or cast spells from their graveyards "
     r"this turn\. you may play lands and cast spells from other players'? "
     r"graveyards this turn as though [^.]+\s*$")
def _shamans_trance(m, raw):
    return Static(modification=Modification(
        kind="shamans_trance"),
        raw=raw)


# "creatures with landwalk ..." — already handled.

# "room abilities of dungeons you own trigger an additional time" —
# Hama Pashar.
@_sp(r"^room abilities of dungeons you own trigger an additional time\s*$")
def _hama_pashar(m, raw):
    return Static(modification=Modification(
        kind="dungeon_rooms_double"),
        raw=raw)


# "merfolk and druid cards in your graveyard have retrace" —
# Deeproot Historian.
@_sp(r"^[a-z]+ and [a-z]+ cards in your graveyard have retrace\s*$")
def _deeproot_historian(m, raw):
    return Static(modification=Modification(
        kind="gy_retrace_grant"),
        raw=raw)


# "other creatures have base power and toughness 2/2 and are bears in
# addition to their other types" — Kudo, King Among Bears.
@_sp(r"^other creatures have base power and toughness (\d+)/(\d+) and are "
     r"[a-z]+ in addition to their other types\s*$")
def _kudo(m, raw):
    return Static(modification=Modification(
        kind="set_others_pt_and_type",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "vernal equinox - any player may cast creature and enchantment spells as
# though they had flash" — Vernal Equinox.
@_sp(r"^any player may cast creature and enchantment spells as though they "
     r"had flash\s*$")
def _vernal_equinox(m, raw):
    return Static(modification=Modification(
        kind="mass_flash_creature_enchant"),
        raw=raw)


# "black and/or red permanents and spells are colorless sources of damage" —
# Ghostly Flame.
@_sp(r"^black and/or red permanents and spells are colorless sources of "
     r"damage\s*$")
def _ghostly_flame(m, raw):
    return Static(modification=Modification(
        kind="ghostly_flame"),
        raw=raw)


# "<color> <type> tokens you control have <kw>" — various tail-grants (rare).

# "spells and abilities your opponents control can't cause you to sacrifice
# permanents" — Tajuru Preserver.
@_sp(r"^spells and abilities your opponents control can'?t cause you to "
     r"sacrifice permanents\s*$")
def _tajuru_preserver(m, raw):
    return Static(modification=Modification(
        kind="no_forced_sac"),
        raw=raw)


# "this creature's power is equal to the number of swamps you control" —
# Sima Yi name-scrub.
@_sp(r"^(?:this creature|~)(?:'?s)? power is equal to the number of [a-z]+ "
     r"you control\s*$")
def _sima_yi(m, raw):
    return Static(modification=Modification(
        kind="self_power_count_swamps"),
        raw=raw)


# "this creature enters with your choice of two different counters on it
# from among <list>" — Grimdancer.
@_sp(r"^(?:this creature|~) enters with your choice of (?:two|three|\d+) "
     r"different counters on it from among [^.]+\s*$")
def _grimdancer(m, raw):
    return Static(modification=Modification(
        kind="etb_counter_choice"),
        raw=raw)


# "whenever a player taps a land for mana, that player adds one mana of any
# type that land produced" — Mana Flare / Mana Web (name-scrub: "~").
# Also "whenever a forest is tapped for mana, its controller adds an
# additional {G}" — Vernal Bloom.
@_sp(r"^whenever a (?:land|forest|swamp|mountain|island|plains) is tapped "
     r"for (?:mana|~), (?:its controller|that player) adds an? additional "
     r"\{[wubrgc]\}\s*$")
def _vernal_bloom(m, raw):
    return Static(modification=Modification(
        kind="tap_land_extra_mana"),
        raw=raw)


# ---------------------------------------------------------------------------
# Enchant <non-creature> / aura anomalies.
# ---------------------------------------------------------------------------

# "en~ creature" / "en~ed creature gets -13/-0" — Chant of the Skifsang
# (card name "Chant" makes "enchant" become "en~"). Accept either line as a
# static.
@_sp(r"^en~\s*(?:creature)?\s*$")
def _scrubbed_enchant(m, raw):
    return Keyword(name="enchant", args=("~-scrubbed",), raw=raw)


@_sp(r"^en~ed creature gets ([+-]\d+)/([+-]\d+)\s*$")
def _scrubbed_enchant_pt(m, raw):
    return Static(modification=Modification(
        kind="enchanted_creature_pt",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "enchanted creature gets -13/-0" bare — covered by base parser; add
# fallback for scrub.
@_sp(r"^enchanted creature gets ([+-]\d+)/([+-]\d+)\s*$")
def _enchanted_pt_bare(m, raw):
    return Static(modification=Modification(
        kind="enchanted_creature_pt",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# ===========================================================================
# TRIGGER_PATTERNS — new trigger shapes.
# ===========================================================================

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []


# "whenever one or more artifact and/or creature cards leave your graveyard
# during your turn" — Thran Vigil.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever one or more [^,]+ leave your graveyard during your "
               r"turn", re.I),
    "leave_gy_during_turn", "self"))


# "whenever another goblin you control is put into a graveyard from the
# battlefield" — Boggart Shenanigans / name-scrub variants.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever another [a-z]+ you control is put into a graveyard "
               r"from the battlefield", re.I),
    "tribal_to_gy_from_bf", "self"))


# "whenever another creature you control or a land you control is put into a
# graveyard from the battlefield" — Long Feng.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever another creature you control or a land you control "
               r"is put into a graveyard from the battlefield", re.I),
    "creature_or_land_to_gy", "self"))


# "whenever this creature dies or another artifact you control is put into a
# graveyard from the battlefield" — Scrap Trawler.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever (?:this creature|~) dies or another [^,]+ is put "
               r"into a graveyard from the battlefield", re.I),
    "self_die_or_ally_gy", "self"))


# "whenever another artifact is put into a graveyard from the battlefield" —
# Magnetic Mine.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever another (?:artifact|enchantment|creature|"
               r"permanent|land) is put into a graveyard from the battlefield",
               re.I),
    "any_type_to_gy_from_bf", "self"))


# "whenever another card is put into a graveyard from anywhere" —
# Planar Void.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever another card is put into a graveyard from anywhere",
               re.I),
    "any_card_to_gy_anywhere", "self"))


# "whenever you control a creature with toughness N or less, sacrifice this
# creature" — Endangered Armodon (technically a static-trigger combo, but
# treat as state-based trigger).
TRIGGER_PATTERNS.append((
    re.compile(r"^when you control a creature with toughness \d+ or less",
               re.I),
    "state_tough_leq", "self"))


# "whenever tui and la become tapped" / "whenever ~ become tapped" — Tui and
# La, Moon and Ocean.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever (?:~|[a-z ]+? and [a-z]+) becomes? tapped", re.I),
    "self_becomes_tapped", "self"))
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever (?:~|[a-z ]+? and [a-z]+) becomes? untapped", re.I),
    "self_becomes_untapped", "self"))


# "whenever ~ attack or block" — Bebop & Rocksteady (paired-name scrub).
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever ~ attacks? or blocks?", re.I),
    "attack_or_block", "self"))


# "whenever ~ enter or attack" — Aang and Katara.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever ~ enters? or attacks?", re.I),
    "etb_or_attack", "self"))


# "whenever another ~ enters" — Kavu Monarch / tribal self-scrubbed.
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever another ~ enters", re.I),
    "another_self_tribe_etb", "self"))


# "whenever you cast a creature spell, sacrifice this creature" —
# Skittering Horror / Skittering Monstrosity.
TRIGGER_PATTERNS.append((
    re.compile(r"^when you cast a creature spell", re.I),
    "on_cast_creature", "self"))


# "whenever a spell you control causes you to gain card advantage" —
# Toofer, Keeper of the Full Grip (Un-set-style).
TRIGGER_PATTERNS.append((
    re.compile(r"^whenever a spell you control causes you to gain card "
               r"advantage", re.I),
    "on_card_advantage", "self"))


# "whenever another kavu enters" / scrubbed "~ creatures have trample" —
# tribal-self statics handled above; trigger variant here covers the 2nd line.


__all__ = ["EFFECT_RULES", "STATIC_PATTERNS", "TRIGGER_PATTERNS"]


# ---------------------------------------------------------------------------
# Smoke test
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    print(f"STATIC_PATTERNS: {len(STATIC_PATTERNS)}")
    print(f"EFFECT_RULES:    {len(EFFECT_RULES)}")
    print(f"TRIGGER_PATTERNS:{len(TRIGGER_PATTERNS)}")
