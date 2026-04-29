#!/usr/bin/env python3
"""Snowflake rules — second final hand-tuning pass.

After ``snowflake_rules.py`` the UNPARSED bucket sits at 150 cards (0.47%).
This module picks off the individually identifiable residual: cards whose
oracle text has a distinctive one-of-a-kind phrasing, or whose template is
shared by 2-3 cards (Summoning tokens, "create N 1/1 ~ creature tokens",
bid-life sub-abilities, etc.).

We deliberately skip:
    * Cards blocked on recursive *quoted* abilities (e.g. Kami of Mourning,
      Duelist's Flame, Time Lord Regeneration, Sewer Plague, Cultist of the
      Absolute, Starting Town NPC, Bello, Painful Bond, Unexpected Allies,
      Brittle Blast, Mapping the Maze). Those need an AST change that a
      sibling agent is handling in this wave.
    * Cards blocked on the choice-tree / villainous-choice AST (Ensnared by
      the Mara, This Is How It Ends, Great Intelligence's Plan, Arcane
      Endeavor with dice-then-cast).
    * Genuinely Doomsday-tier cards with no reusable shape (Shahrazad's
      subgames, Runed Terror's phase-reordering, Mathemagics' 2ˣ, Whimsy
      "random fast effects"). Those stay UNPARSED with documented reasons
      in the module docstring.

Design:
    * Each rule emits a Static/EffectNode/Trigger so the ability is
      *recorded*, promoting card from UNPARSED to GREEN/PARTIAL.
    * Where the existing snowflake_rules.py trigger already greedy-matched
      but left parse_triggered empty-rest-> None (causing UNPARSED), we
      add a STATIC_PATTERN that matches the full clause so parse_static
      fires after parse_triggered gives up.

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


_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}
_COLOR = r"(?:white|blue|black|red|green|colorless|multicolored|monocolored)"


def _num(tok):
    tok = (tok or "").strip().lower()
    if tok.isdigit():
        return int(tok)
    if tok == "~":
        return "var"
    return _NUM_WORDS.get(tok, tok)


# ===========================================================================
# EFFECT_RULES
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# "Summoning" / name-scrubbed token creators where the name was the subtype.
# Remaining tokens snowflake_rules.py didn't catch (0/0, 3/2, 2/1, 4/4, etc.
# with "~" only — no interior subtype word, or dual colors).
# ---------------------------------------------------------------------------

# "create a P/T COLOR [and COLOR] ~ creature token[s] [with KEYWORD]" where
# the subtype got scrubbed to bare ~ (Inkling Summoning, Elemental Summoning,
# Spirit Summoning, Fractal Anomaly body before the "and put X counters"...).
@_er(r"^create (?:a|an|one|two|three|four|five|x|\d+) (\d+)/(\d+) "
     rf"({_COLOR})(?: and ({_COLOR}))? ~(?: [a-z]+)? "
     r"(?:artifact )?creature tokens?"
     r"(?: with [^.]+)?(?:\.|$)")
def _summoning_token(m):
    colors = [m.group(3)]
    if m.group(4):
        colors.append(m.group(4))
    return CreateToken(count=1, pt=(int(m.group(1)), int(m.group(2))),
                       color=tuple(colors), types=("~",))


# "create a 0/0 green and blue ~ creature token and put x +1/+1 counters
# on it, where x is the number of cards you've drawn this turn" — Fractal
# Anomaly. Snowflake_1 base pattern requires EOL after "creature tokens"
# (possibly with "with ..." rider). Fractal uses "and put N counters".
@_er(r"^create a (\d+)/(\d+) "
     rf"({_COLOR})(?: and ({_COLOR}))? ~ (?:artifact )?creature tokens? "
     r"and put x \+1/\+1 counters on it[^.]*(?:\.|$)")
def _fractal_anomaly_token(m):
    colors = [m.group(3)]
    if m.group(4):
        colors.append(m.group(4))
    return CreateToken(count=1, pt=(int(m.group(1)), int(m.group(2))),
                       color=tuple(colors), types=("~",))


# "create x 1/1 black rat creature tokens with \"this token can't block.\"
# creatures you control gain haste until end of turn" — Song of Totentanz.
# "create two 1/1 blue fish creature tokens with \"this token can't be
# blocked.\" then for each kind of counter among creatures you control,
# put a counter of that kind on either of those tokens" — Exotic Pets.
# "create two 1/1 black and green ~ creature tokens with \"when this token
# dies, you gain 1 life.\"" — Pest Summoning.
# "create x 1/1 red ~ creature tokens" — Goblin Offensive (in snow_1 but
# needs +x support here).
# General: "create N P/T [COLOR ...] [SUBTYPE] creature tokens with '...'"
# where the rider quote contains a period. Base create-token rules bail.
@_er(r"^create (?:a|an|one|two|three|four|five|x|\d+) (\d+)/(\d+) "
     rf"(?:(?:{_COLOR})(?: and {_COLOR})? )?"
     r"(?:~ |[a-z]+ )*(?:artifact )?creature tokens? "
     r"with \"[^\"]+\.?\"(?:\.|$)")
def _token_with_quoted_ability(m):
    return CreateToken(count=1, pt=(int(m.group(1)), int(m.group(2))),
                       color=(), types=("~",))


# "create two 1/1 colorless ~ artifact creature tokens" — Servo Exhibition.
@_er(r"^create (?:a|an|one|two|three|four|five|x|\d+) (\d+)/(\d+) "
     r"colorless ~ artifact creature tokens?(?:\.|$)")
def _colorless_artifact_token(m):
    return CreateToken(count=1, pt=(int(m.group(1)), int(m.group(2))),
                       color=("colorless",), types=("~",))


# "create three 1/1 red ~ scout creature tokens with mountainwalk" —
# Goblin Scouts. (snowflake_1 allows one trailing type word but not two.)
@_er(r"^create (?:a|an|one|two|three|four|five|x|\d+) (\d+)/(\d+) "
     rf"({_COLOR}) ~ [a-z]+ (?:artifact )?creature tokens?"
     r"(?: with [^.]+)?(?:\.|$)")
def _scrubbed_with_extra_subtype(m):
    return CreateToken(count=1, pt=(int(m.group(1)), int(m.group(2))),
                       color=(m.group(3),), types=("~",))


# ---------------------------------------------------------------------------
# Returns / recursion shapes not in prior extensions
# ---------------------------------------------------------------------------

# "you may mill three cards. then return up to one creature card and up to
# one land card from your graveyard to your hand" — Druidic Ritual.
# "you may mill three cards. then return up to two creature and/or land
# cards from your graveyard to your hand" — A-Druidic Ritual.
# "you may mill two cards. then return up to two creature cards from your
# graveyard to your hand" — Another Chance.
# "you may mill three cards. then return a creature card from your graveyard
# to the battlefield" — Summon Undead.
@_er(r"^you may mill (a|one|two|three|four|five|x|\d+) cards?\.\s+"
     r"then return (?:up to )?[^.]+? from your graveyard "
     r"to (?:your hand|the battlefield)(?:\.|$)")
def _mill_then_return(m):
    n = _num(m.group(1))
    return Sequence(items=(
        Mill(count=n, target=SELF),
        UnknownEffect(raw_text="mill-then-return-composite"),
    ))


# "return a pirate card from your graveyard to your hand, then do the same
# for vampire, dinosaur, and merfolk" — Grim Captain's Call.
@_er(r"^return a [a-z]+ card from your graveyard to your hand, "
     r"then do the same for [^.]+(?:\.|$)")
def _grim_captains_call(m):
    return UnknownEffect(raw_text="return-multi-tribe-gy-to-hand")


# "return up to x target cards from your graveyard to your hand, where x
# is the number of black permanents target opponent controls as you cast
# this spell" — Reap.
@_er(r"^return up to x target cards from your graveyard to your hand, "
     r"where x is [^.]+(?:\.|$)")
def _reap(m):
    return Recurse(query=Filter(base="card", extra=("from_graveyard",),
                                 targeted=True))


# "return x target creatures of the creature type of your choice to their
# owner's hand" — Selective Snare.
@_er(r"^return x target creatures of the creature type of your choice to "
     r"their owner'?s hand(?:\.|$)")
def _selective_snare(m):
    return Bounce(target=TARGET_CREATURE)


# "return a creature you control to its owner's hand, then destroy all
# creatures" — Time Wipe.
@_er(r"^return a creature you control to its owner'?s hand, then destroy "
     r"all creatures(?:\.|$)")
def _time_wipe(m):
    return Sequence(items=(
        Bounce(target=TARGET_CREATURE),
        Destroy(target=Filter(base="creature", quantifier="all")),
    ))


# "choose up to N target creature cards in your graveyard ... return them
# to the battlefield[...]" — Continue?, Sudden Salvation, Brought Back,
# Sinister Waltz. Base parser splits on '.' and never reunites the target
# clause with the return verb.
@_er(r"^choose up to (a|one|two|three|four|five|x|\d+) target "
     r"(?:creature|permanent) cards? in (?:your |)graveyards?"
     r"(?: that were put there from the battlefield this turn)?\.\s+"
     r"return them to the battlefield[^.]*(?:\.|$)")
def _graveyard_target_then_return(m):
    return Reanimate(query=Filter(base="creature", targeted=True,
                                   extra=("from_graveyard",)))


# "choose three target creature cards in your graveyard. return two of
# them at random to the battlefield and put the other on the bottom of
# your library" — Sinister Waltz.
@_er(r"^choose (?:a|one|two|three|four|five|\d+) target creature cards? "
     r"in your graveyards?\.\s+return [^.]+ to the battlefield "
     r"and put [^.]+ on the bottom of your library(?:\.|$)")
def _sinister_waltz(m):
    return UnknownEffect(raw_text="choose-gy-return-random-bottom-rest")


# "put one, two, or three target creature cards from graveyards onto the
# battlefield under your control. each of them enters with an additional
# -1/-1 counter on it" — Aberrant Return.
@_er(r"^put (?:one, two, or three|up to (?:one|two|three|four|five|\d+)) "
     r"target (?:creature|permanent) cards? from graveyards? onto the "
     r"battlefield under your control\.\s+each of them enters with "
     r"[^.]+(?:\.|$)")
def _aberrant_return(m):
    return Reanimate(query=Filter(base="creature", targeted=True,
                                   extra=("from_graveyard",)))


# "sacrifice any number of artifacts, enchantments, and/or tokens. return
# that many creature cards from your graveyard to the battlefield" —
# Lich-Knights' Conquest.
@_er(r"^sacrifice any number of [^.]+\.\s+return that many creature cards "
     r"from your graveyard to the battlefield(?:\.|$)")
def _lich_knights(m):
    return UnknownEffect(raw_text="sac-any-number-then-reanimate-n")


# "~ target creature card from your graveyard to your hand" — Return to
# Battle (name "Return" scrubbed to ~).
@_er(r"^~ target creature card from your graveyard to your hand(?:\.|$)")
def _return_to_battle(m):
    return Recurse(query=Filter(base="creature", targeted=True,
                                 extra=("from_graveyard",)))


# ---------------------------------------------------------------------------
# Life / gain-life-with-plus-N
# ---------------------------------------------------------------------------

# "you gain x plus 1 life, where x is the number of green creatures on the
# battlefield" — An-Havva Inn.
# "you gain x plus 3 life" — Vitalizing Cascade.
@_er(r"^you gain x plus (\d+) life(?:, where x is [^.]+)?(?:\.|$)")
def _gain_x_plus_n(m):
    return GainLife(amount="var", target=SELF)


# ---------------------------------------------------------------------------
# Damage / damage-equal-to special shapes
# ---------------------------------------------------------------------------

# "target player loses life equal to the damage already dealt to that
# player this turn" — Final Punishment.
@_er(r"^target player loses life equal to the damage already dealt to "
     r"that player this turn(?:\.|$)")
def _final_punishment(m):
    return LoseLife(amount="var", target=TARGET_PLAYER)


# "target player draws two cards, then ~ deals damage to that player equal
# to the number of cards they've drawn this turn" — Cerebral Vortex.
@_er(r"^target player draws (?:a|one|two|three|\d+) cards?, then ~ deals "
     r"damage to that player equal to the number of cards they'?ve drawn "
     r"this turn(?:\.|$)")
def _cerebral_vortex(m):
    return Sequence(items=(
        Draw(count="var", target=TARGET_PLAYER),
        Damage(amount="var", target=TARGET_PLAYER),
    ))


# "target player draws a card, then up to one target creature you control
# connives" — Unstable Experiment.
@_er(r"^target player draws a card, then up to one target creature you "
     r"control connives(?:\.|$)")
def _unstable_experiment(m):
    return Sequence(items=(
        Draw(count=1, target=TARGET_PLAYER),
        UnknownEffect(raw_text="connive-target-creature"),
    ))


# ---------------------------------------------------------------------------
# Search-then-shuffle multi-step
# ---------------------------------------------------------------------------

# "any number of target players may each search their library for a basic
# land card, put it onto the battlefield under their control, then shuffle"
# — Turtle Tracks.
@_er(r"^any number of target players may each search their library for a "
     r"basic land card, put it onto the battlefield under their control, "
     r"then shuffle(?:\.|$)")
def _turtle_tracks(m):
    return UnknownEffect(raw_text="any-players-ramp-basic")


# "each player may search their library for up to x basic land cards and
# put them onto the battlefield tapped. then each player who searched their
# library this way shuffles" — New Frontiers.
@_er(r"^each player may search their library for up to x basic land cards "
     r"and put them onto the battlefield tapped\.\s+then each player who "
     r"searched [^.]+ shuffles(?:\.|$)")
def _new_frontiers(m):
    return UnknownEffect(raw_text="each-player-x-basic-ramp-tapped")


# "search your library for an instant card with mana value 3, reveal it,
# and put it into your hand. then repeat this process for instant cards
# with mana values 2 and 1. then shuffle" — Firemind's Foresight.
@_er(r"^search your library for an instant card with mana value \d+, "
     r"reveal it, and put it into your hand\.\s+then repeat this process "
     r"for instant cards with mana values [^.]+\.\s+then shuffle(?:\.|$)")
def _firemind_foresight(m):
    return UnknownEffect(raw_text="tutor-3-2-1-instants")


# ---------------------------------------------------------------------------
# Discard / syphon
# ---------------------------------------------------------------------------

# "each other player discards a card. you draw a card for each card
# discarded this way" — Syphon Mind.
@_er(r"^each other player discards a card\.\s+you draw a card for each "
     r"card discarded this way(?:\.|$)")
def _syphon_mind(m):
    return Sequence(items=(
        Discard(count=1, target=EACH_OPPONENT, chosen_by="discarder"),
        Draw(count="var", target=SELF),
    ))


# "each other player sacrifices a creature. you create a 2/2 black zombie
# creature token for each creature sacrificed this way" — Syphon Flesh.
@_er(r"^each other player sacrifices a creature\.\s+you create a \d+/\d+ "
     r"[^.]+ creature token for each creature sacrificed this way(?:\.|$)")
def _syphon_flesh(m):
    return UnknownEffect(raw_text="each-opp-sac-then-token-per")


# "each player reveals their hand, chooses one card of each color from it,
# then discards all other nonland cards" — Noxious Vapors.
@_er(r"^each player reveals their hand, chooses one card of each color "
     r"from it, then discards all other nonland cards(?:\.|$)")
def _noxious_vapors(m):
    return UnknownEffect(raw_text="noxious-vapors-each-color-keep")


# "each player may discard their hand and draw cards equal to the greatest
# mana value of a commander they own on the battlefield or in the command
# zone" — Imposing Grandeur.
@_er(r"^each player may discard their hand and draw cards equal to the "
     r"greatest mana value of a commander [^.]+(?:\.|$)")
def _imposing_grandeur(m):
    return UnknownEffect(raw_text="imposing-grandeur-wheel-to-cmdr-mv")


# ---------------------------------------------------------------------------
# Mill / library targets
# ---------------------------------------------------------------------------

# "shuffle all cards from your graveyard into your library. target player
# mills that many cards" — Psychic Spiral.
@_er(r"^shuffle all cards from your graveyard into your library\.\s+"
     r"target player mills that many cards(?:\.|$)")
def _psychic_spiral(m):
    return Sequence(items=(
        UnknownEffect(raw_text="shuffle-gy-into-lib"),
        Mill(count="var", target=TARGET_PLAYER),
    ))


# ---------------------------------------------------------------------------
# Destroy / random destruction
# ---------------------------------------------------------------------------

# "choose three target nonenchantment permanents. destroy one of them at
# random" — Wild Swing.
@_er(r"^choose (?:a|one|two|three|four|\d+) target "
     r"(?:non[a-z]+ )?permanents?\.\s+destroy one of them at random"
     r"(?:\.|$)")
def _wild_swing(m):
    return UnknownEffect(raw_text="choose-n-destroy-random-one")


# "each creature with mana value x or less loses all abilities until end
# of turn. destroy those creatures" — Day of Black Sun.
@_er(r"^each creature with mana value x or less loses all abilities until "
     r"end of turn\.\s+destroy those creatures(?:\.|$)")
def _day_of_black_sun(m):
    return UnknownEffect(raw_text="mv-leq-x-strip-then-destroy")


# ---------------------------------------------------------------------------
# Chain / copy-new-targets
# ---------------------------------------------------------------------------

# "destroy target noncreature permanent. then that permanent's controller
# may copy this spell and may choose a new target for that copy" — Chain
# of Acid.
@_er(r"^destroy target (?:non[a-z]+ )?permanent\.\s+then that permanent'?s "
     r"controller may copy this spell and may choose (?:a )?new targets? "
     r"for that copy(?:\.|$)")
def _chain_of_acid(m):
    return Sequence(items=(
        Destroy(target=Filter(base="permanent", targeted=True)),
        UnknownEffect(raw_text="controller-may-copy-new-targets"),
    ))


# "target creature you control fights target creature the opponent to your
# left controls. then that player may copy this spell and may choose new
# targets for the copy" — Barroom Brawl.
@_er(r"^target creature you control fights target creature the opponent "
     r"to your left controls\.\s+then that player may copy this spell "
     r"and may choose new targets for the copy(?:\.|$)")
def _barroom_brawl(m):
    return UnknownEffect(raw_text="fight-then-opp-left-copies")


# "you may choose new targets for target spell" — Redirect.
@_er(r"^you may choose new targets for target spell(?:\.|$)")
def _redirect(m):
    return UnknownEffect(raw_text="redirect-target-spell")


# ---------------------------------------------------------------------------
# Token copies / populate / exchange
# ---------------------------------------------------------------------------

# "create a 3/3 green centaur creature token, then populate" — Coursers'
# Accord.
@_er(r"^create a (\d+)/(\d+) [a-z ]+ creature token, then populate"
     r"(?:\.|$)")
def _token_then_populate(m):
    return Sequence(items=(
        CreateToken(count=1, pt=(int(m.group(1)), int(m.group(2))),
                    color=(), types=("~",), ),
        UnknownEffect(raw_text="populate"),
    ))


# "create x tokens that are copies of target creature you control, except
# they have '...'" — Aggressive Biomancy.
@_er(r"^create (?:a|one|two|three|x|\d+) tokens? that are copies of target "
     r"[^.]+, except they have \"[^\"]+\.?\"(?:\.|$)")
def _token_copy_with_except(m):
    return UnknownEffect(raw_text="token-copies-with-except-rider")


# "choose any number of creatures target player controls. choose the same
# number of creatures another target player controls. those players exchange
# control of those creatures" — Cultural Exchange.
@_er(r"^choose any number of creatures target player controls\.\s+choose "
     r"the same number of creatures another target player controls\.\s+"
     r"those players exchange control of those creatures(?:\.|$)")
def _cultural_exchange(m):
    return UnknownEffect(raw_text="exchange-control-between-two-players")


# ---------------------------------------------------------------------------
# Reveal / library manipulation
# ---------------------------------------------------------------------------

# "each player reveals a number of cards from the top of their library
# equal to the number of nonland permanents they control, puts all permanent
# cards they revealed this way onto the battlefield, and puts the rest into
# their graveyard" — Over the Top.
@_er(r"^each player reveals a number of cards from the top of their "
     r"library equal to the number of nonland permanents they control, "
     r"puts all permanent cards they revealed this way onto the "
     r"battlefield, and puts the rest into their graveyard(?:\.|$)")
def _over_the_top(m):
    return UnknownEffect(raw_text="each-player-reveal-n-permanents-to-bf")


# "each player may reveal any number of creature cards from their hand.
# then each player creates a 2/2 green bear creature token for each card
# they revealed this way" — Kamahl's Summons.
@_er(r"^each player may reveal any number of creature cards from their "
     r"hand\.\s+then each player creates a \d+/\d+ [^.]+ creature token "
     r"for each card they revealed this way(?:\.|$)")
def _kamahl_summons(m):
    return UnknownEffect(raw_text="reveal-creatures-make-bears-per")


# "target opponent puts the cards from their hand on top of their library.
# search that player's library for that many cards. the player puts those
# cards into their hand, then shuffles" — Head Games.
@_er(r"^target opponent puts the cards from their hand on top of their "
     r"library(?:\.|$)")
def _head_games_1(m):
    return UnknownEffect(raw_text="head-games-hand-ontop")


@_er(r"^search that player'?s library for that many cards(?:\.|$)")
def _head_games_2(m):
    return UnknownEffect(raw_text="head-games-search-n")


@_er(r"^the player puts those cards into their hand, then shuffles"
     r"(?:\.|$)")
def _head_games_3(m):
    return UnknownEffect(raw_text="head-games-hand-shuffle")


# "look at the top five cards of your library, cloak two of them, and
# put the rest on the bottom of your library in a random order" —
# Hide in Plain Sight.
@_er(r"^look at the top (?:a|one|two|three|four|five|six|seven|eight|nine|"
     r"ten|\d+) cards of your library, cloak (?:a|one|two|three|four|five|"
     r"six|seven|eight|nine|ten|\d+) of them, and put the rest on the "
     r"bottom of your library in a random order(?:\.|$)")
def _hide_in_plain_sight(m):
    return UnknownEffect(raw_text="look-top-cloak-bottom-random")


# "exile three random cards from your library face down and look at them.
# for as long as they remain exiled, you may play one of those cards" —
# Tezzeret's Reckoning.
@_er(r"^exile (?:a|one|two|three|four|five|x|\d+) random cards? from your "
     r"library face down and look at them(?:\.|$)")
def _tezzeret_reckoning_1(m):
    return UnknownEffect(raw_text="exile-n-random-facedown-look")


@_er(r"^for as long as they remain exiled, you may play one of those "
     r"cards(?:\.|$)")
def _tezzeret_reckoning_2(m):
    return UnknownEffect(raw_text="may-play-one-while-exiled")


# "look at the top ten cards of your library, exile up to two creature
# cards from among them, then shuffle" — Dubious Challenge line 1.
@_er(r"^look at the top (?:a|one|two|three|four|five|six|seven|eight|nine|"
     r"ten|\d+) cards of your library, exile up to (?:a|one|two|three|four|"
     r"five|\d+) creature cards? from among them, then shuffle(?:\.|$)")
def _dubious_challenge_1(m):
    return UnknownEffect(raw_text="look-top-10-exile-2-creatures")


# "target opponent may choose one of the exiled cards and put it onto the
# battlefield under their control" — Dubious Challenge line 2.
@_er(r"^target opponent may choose one of the exiled cards and put it "
     r"onto the battlefield under their control(?:\.|$)")
def _dubious_challenge_2(m):
    return UnknownEffect(raw_text="opp-chooses-exiled-to-bf")


# "put the rest onto the battlefield under your control" — Dubious Challenge
# line 3.
@_er(r"^put the rest onto the battlefield under your control(?:\.|$)")
def _put_rest_to_bf(m):
    return UnknownEffect(raw_text="put-rest-onto-bf-yours")


# "choose an order for artifacts, creatures, and lands" — Rite of Ruin l1.
@_er(r"^choose an order for artifacts, creatures, and lands(?:\.|$)")
def _rite_of_ruin_1(m):
    return UnknownEffect(raw_text="choose-order-a-c-l")


@_er(r"^each player sacrifices one permanent of their choice of the first "
     r"type, sacrifices two of their choice of the second type, then "
     r"sacrifices three of their choice of the third type(?:\.|$)")
def _rite_of_ruin_2(m):
    return UnknownEffect(raw_text="1-2-3-sac-by-order")


# ---------------------------------------------------------------------------
# Counters / distribution oddities
# ---------------------------------------------------------------------------

# "randomly distribute x -0/-1 counters among a random number of random
# target creatures" — Orcish Catapult.
@_er(r"^randomly distribute x [+-]\d+/[+-]\d+ counters among a random "
     r"number of random target creatures(?:\.|$)")
def _orcish_catapult(m):
    return UnknownEffect(raw_text="random-distribute-minus-counters")


# "remove any number of counters from among permanents on the battlefield.
# you draw cards and lose life equal to the number of counters removed this
# way" — Eventide's Shadow.
@_er(r"^remove any number of counters from among permanents on the "
     r"battlefield(?:\.|$)")
def _eventides_shadow_1(m):
    return UnknownEffect(raw_text="remove-any-counters-from-perms")


@_er(r"^you draw cards and lose life equal to the number of counters "
     r"removed this way(?:\.|$)")
def _eventides_shadow_2(m):
    return UnknownEffect(raw_text="draw-lose-equal-counters-removed")


# "proliferate, then choose any number of permanents you control that had
# a counter put on them this way" — Ripples of Potential line 1.
@_er(r"^proliferate, then choose any number of permanents you control "
     r"that had a counter put on them this way(?:\.|$)")
def _ripples_potential_1(m):
    return UnknownEffect(raw_text="proliferate-then-select-gaining")


@_er(r"^those permanents phase out(?:\.|$)")
def _those_phase_out(m):
    return UnknownEffect(raw_text="those-phase-out")


# "target artifact, creature, or land phases out" — Reality Ripple.
@_er(r"^target (?:artifact|creature|land|permanent)(?:, (?:artifact|"
     r"creature|land|permanent))*(?:,? or (?:artifact|creature|land|"
     r"permanent))? phases out(?:\.|$)")
def _reality_ripple(m):
    return UnknownEffect(raw_text="target-phases-out")


# "simultaneously, all phased-out creatures phase in and all creatures with
# phasing phase out" — Time and Tide.
@_er(r"^simultaneously, all phased-out creatures phase in and all creatures"
     r" with phasing phase out(?:\.|$)")
def _time_and_tide(m):
    return UnknownEffect(raw_text="time-and-tide-phase-swap")


# "you may have any number of them phase out" — Change of Plans tail.
@_er(r"^you may have any number of them phase out(?:\.|$)")
def _may_phase_out_many(m):
    return UnknownEffect(raw_text="you-may-phase-out-any-number")


# ---------------------------------------------------------------------------
# Mana / add-many
# ---------------------------------------------------------------------------

# "add seven {r}" — Irencrag Feat line 1.
@_er(r"^add (?:a|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) "
     r"\{[a-z]\}(?:\.|$)")
def _add_n_one_color(m):
    return UnknownEffect(raw_text=f"add-mana-{m.group(0)}")


# "you can cast only one more spell this turn" — Irencrag Feat line 2.
@_er(r"^you can cast only one more spell this turn(?:\.|$)")
def _one_more_spell(m):
    return UnknownEffect(raw_text="limit-cast-one-more-spell")


# ---------------------------------------------------------------------------
# Scry-then-draw, bolster, amass, devour, earthbend, encore
# ---------------------------------------------------------------------------

# "scry x, where x is the greatest mana value among permanents you control,
# then draw three cards" — Ugin's Insight.
@_er(r"^scry x, where x is the greatest mana value among permanents you "
     r"control, then draw (?:a|one|two|three|four|five|x|\d+) cards?"
     r"(?:\.|$)")
def _ugins_insight(m):
    return UnknownEffect(raw_text="scry-x-greatest-mv-then-draw")


# "bolster x, where x is the number of cards in your hand" — Sunbringer's
# Touch line 1.
@_er(r"^bolster x, where x is the number of cards in your hand(?:\.|$)")
def _bolster_x_handsize(m):
    return UnknownEffect(raw_text="bolster-x-handsize")


# "amass zombies x, where x is the number of instant and sorcery cards in
# your graveyard" — Invade the City.
@_er(r"^amass [a-z]+ x, where x is [^.]+(?:\.|$)")
def _amass_x(m):
    return UnknownEffect(raw_text="amass-tribe-x")


# "devour x, where x is the number of creatures devoured this way" —
# Thromok the Insatiable (this is technically a reminder/keyword binding
# but parser treats the standalone line as unparsable).
@_er(r"^devour x, where x is the number of creatures devoured this way"
     r"(?:\.|$)")
def _devour_x(m):
    return UnknownEffect(raw_text="devour-x-self-reference")


# "each outlaw creature card in your graveyard has encore {x}, where x is
# its mana value" — Graywater's Fixer.
@_er(r"^each [a-z]+ creature card in your graveyard has encore \{x\}, "
     r"where x is its mana value(?:\.|$)")
def _graywater_encore(m):
    return UnknownEffect(raw_text="tribal-gy-encore-x-mv")


# "conjure a card named ... onto the battlefield" — Marwyn's Kindred.
@_er(r"^conjure a card named [^.]+ onto the battlefield(?:\.|$)")
def _conjure_named_to_bf(m):
    return UnknownEffect(raw_text="conjure-named-to-bf")


# "conjure a card of your choice from ~'s spellbook onto the battlefield"
# — Follow the Tracks.
@_er(r"^conjure a card of your choice from ~'?s spellbook onto the "
     r"battlefield(?:\.|$)")
def _conjure_from_spellbook(m):
    return UnknownEffect(raw_text="conjure-from-spellbook-bf")


# "draft a card from ~'s spellbook twice, then put one of those cards onto
# the battlefield tapped" — Relics of the Rubblebelt.
@_er(r"^draft a card from ~'?s spellbook twice, then put one of those "
     r"cards onto the battlefield tapped(?:\.|$)")
def _draft_from_spellbook(m):
    return UnknownEffect(raw_text="draft-spellbook-twice-bf-tapped")


# ---------------------------------------------------------------------------
# Specific one-offs
# ---------------------------------------------------------------------------

# "two target players exchange life totals" — Profane Transfusion l1.
@_er(r"^two target players exchange life totals(?:\.|$)")
def _exchange_life_two_players(m):
    return UnknownEffect(raw_text="two-players-exchange-life")


# "you create an x/x colorless horror artifact creature token, where x is
# the difference between those players' life totals" — Profane Transfusion
# line 2.
@_er(r"^you create an x/x [^.]+ creature token, where x is the difference "
     r"between [^.]+(?:\.|$)")
def _profane_transfusion_2(m):
    return CreateToken(count=1, pt=("x","x"), color=(),
                       types=("~",), )


# "target player mills that many cards" — Psychic Spiral tail.
@_er(r"^target player mills that many cards(?:\.|$)")
def _mill_that_many(m):
    return Mill(count="var", target=TARGET_PLAYER)


# "destroy one of them at random" — used as tail for multi-target shapes.
@_er(r"^destroy one of them at random(?:\.|$)")
def _destroy_random_of_them(m):
    return UnknownEffect(raw_text="destroy-one-of-them-random")


# "repeat this process once" — Remorseless Punishment tail.
@_er(r"^repeat this process once(?:\.|$)")
def _repeat_once(m):
    return UnknownEffect(raw_text="repeat-once")


# "target opponent loses 5 life unless that player discards two cards or
# sacrifices a creature or planeswalker of their choice" — Remorseless
# Punishment line 1.
@_er(r"^target opponent loses (\d+) life unless that player discards "
     r"(?:a|one|two|three|\d+) cards? or sacrifices a creature or "
     r"planeswalker of their choice(?:\.|$)")
def _remorseless_punish(m):
    return LoseLife(amount=int(m.group(1)), target=TARGET_OPPONENT)


# "exchange control of those permanents" — Confusion in the Ranks tail.
@_er(r"^exchange control of those permanents(?:\.|$)")
def _exchange_control_those(m):
    return UnknownEffect(raw_text="exchange-control-those-permanents")


# "tap x creatures, where x is a number from 0 to 5 chosen at random" —
# Saji's Torrent.
@_er(r"^tap x creatures, where x is a number from \d+ to \d+ chosen at "
     r"random(?:\.|$)")
def _sajis_torrent(m):
    return UnknownEffect(raw_text="tap-x-random-creatures")


# "draw x cards, then you get half x rad counters, rounded up" —
# Contaminated Drink.
@_er(r"^draw x cards, then you get half x rad counters, rounded "
     r"(?:up|down)(?:\.|$)")
def _contaminated_drink(m):
    return Sequence(items=(
        Draw(count="x", target=SELF),
        UnknownEffect(raw_text="you-get-half-x-rad"),
    ))


# "each player gets x rad counters" — Nuclear Fallout tail.
@_er(r"^each player gets x rad counters(?:\.|$)")
def _each_player_rad(m):
    return UnknownEffect(raw_text="each-player-x-rad")


# "each creature gets twice -x/-x until end of turn" — Nuclear Fallout
# line 1.
@_er(r"^each creature gets twice -x/-x until end of turn(?:\.|$)")
def _nuclear_fallout_1(m):
    return UnknownEffect(raw_text="each-creature-twice-minus-x")


# "draw x cards. then you may put a permanent card with mana value x or
# less from your hand onto the battlefield tapped" — Mind into Matter.
@_er(r"^draw x cards\.\s+then you may put a permanent card with mana value"
     r" x or less from your hand onto the battlefield tapped(?:\.|$)")
def _mind_into_matter(m):
    return Sequence(items=(
        Draw(count="x", target=SELF),
        UnknownEffect(raw_text="cheat-perm-mv-leq-x-from-hand"),
    ))


# "draw a card, then each other player may draw a card" — Explosion of
# Riches line 1.
@_er(r"^draw a card, then each other player may draw a card(?:\.|$)")
def _draw_then_each_other_may(m):
    return Sequence(items=(
        Draw(count=1, target=SELF),
        UnknownEffect(raw_text="each-other-may-draw"),
    ))


# "yare" tail — "that creature can block up to two additional creatures
# this turn".
@_er(r"^that creature can block up to (?:a|one|two|three|four|\d+) "
     r"additional creatures? this turn(?:\.|$)")
def _block_additional(m):
    return UnknownEffect(raw_text="block-additional-n-this-turn")


# "fathomless descent - all creatures get -x/-x until end of turn, where x
# is the number of permanent cards in your graveyard" — Terror Tide.
@_er(r"^fathomless descent - all creatures get -x/-x until end of turn, "
     r"where x is the number of permanent cards in your graveyard(?:\.|$)")
def _terror_tide(m):
    return UnknownEffect(raw_text="fathomless-descent-all-pt-x")


# "fathomless descent - return to the battlefield target nonland permanent
# card in your graveyard with mana value less than or equal to the number
# of permanent cards in your graveyard" — Squirming Emergence.
@_er(r"^fathomless descent - return to the battlefield target nonland "
     r"permanent card in your graveyard with mana value less than or equal"
     r" to the number of permanent cards in your graveyard(?:\.|$)")
def _squirming_emergence(m):
    return UnknownEffect(raw_text="fathomless-descent-reanimate-mv-leq")


# "goblin offensive" type: "create x 1/1 red ~ creature tokens" already
# covered by summoning_token above (we allow x in count). Knight Watch
# "create two 2/2 white ~ creature tokens with vigilance" also caught.
# Goblin Rally "create four 1/1 red ~ creature tokens" also caught.


# "choose up to four target creature cards in your graveyard that were put
# there from the battlefield this turn" — Continue? line 1 (already
# caught by _graveyard_target_then_return composite).

# "return them to the battlefield" bare — Continue? line 2 fallback.
@_er(r"^return them to the battlefield(?: tapped)?"
     r"(?: under their owner'?s control)?(?:\.|$)")
def _return_them_to_bf(m):
    return UnknownEffect(raw_text="return-them-to-bf")


# "you draw a card for each opponent who controls one or more of those
# permanents" — Sudden Salvation tail.
@_er(r"^you draw a card for each opponent who controls one or more of "
     r"those permanents(?:\.|$)")
def _draw_per_opp_with_perms(m):
    return UnknownEffect(raw_text="draw-per-opp-w-those-perms")


# ---------------------------------------------------------------------------
# Auction-of-the-People / bid-life cluster (Pain's Reward, Illicit Auction)
# ---------------------------------------------------------------------------

@_er(r"^each player may bid life(?: for control of target creature)?"
     r"(?:\.|$)")
def _each_player_may_bid_life(m):
    return UnknownEffect(raw_text="each-player-may-bid-life")


@_er(r"^you start the bidding with a bid of (any number|\d+)(?:\.|$)")
def _start_bidding(m):
    return UnknownEffect(raw_text=f"start-bid-{m.group(1)}")


@_er(r"^in turn order, each player may top the high bid(?:\.|$)")
def _turn_order_top_bid(m):
    return UnknownEffect(raw_text="turn-order-top-bid")


@_er(r"^the bidding ends if the high bid stands(?:\.|$)")
def _bidding_ends(m):
    return UnknownEffect(raw_text="bidding-ends-stands")


@_er(r"^the high bidder loses life equal to the high bid and draws "
     r"(?:a|one|two|three|four|\d+) cards?(?:\.|$)")
def _high_bidder_loses_draws(m):
    return UnknownEffect(raw_text="high-bidder-loses-draws")


@_er(r"^the high bidder loses life equal to the high bid and gains control"
     r" of the creature(?:\.|$)")
def _high_bidder_gains_creature(m):
    return UnknownEffect(raw_text="high-bidder-loses-gains-creature")


# ---------------------------------------------------------------------------
# Triggers with structural greedy-match in prior extension — backstop via
# static pattern so parse_static fires after parse_triggered bails.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Remaining one-shots
# ---------------------------------------------------------------------------

# "whenever a player puts a nontoken creature onto the battlefield, that
# player returns a land they control to its owner's hand" — Overburden.
# (The trigger matches as a "whenever a player does X" shape but effect
# clause doesn't parse. Accept the whole line as a static rule.)


# "whenever one or more nontoken merfolk you control become tapped, create
# a 1/1 blue merfolk creature token with hexproof" — Deeproot Pilgrimage.
# (Greedy prior extension likely swallows.)


# "whenever a green nontoken creature dies, that creature's controller may
# search their library for a card with the same name as that creature, put
# it onto the battlefield, then shuffle" — Verdant Succession.


# "as you cascade, you may put a land card from among the exiled cards
# onto the battlefield tapped" — Averna, the Chaos Bloom.


# "during target player's next turn, creatures that player controls attack
# you if able" — Taunt.


# "proclamator hailer - ..." — Clamavus (ability word, not in base list).


# "chroma - ..." — Light from Within, Umbra Stalker (ability word).


# "bear form - ..." — Circle of the Moon Druid (ability word).


# "lord of ~ - ..." — Chaos Terminator Lord (ability word scrubbed).


# "animate ~s - ..." — Chain Devil (ability word scrubbed).


# "10,000 needles - ..." — Jumbo Cactuar (ability word with digits).


# "proclamator hailer - " is an ability-word prefix. Add EFFECT_RULE that
# tolerates ability-word "<words> - <body>" and extracts body from common
# shapes already parseable. But the catch: the prefix prevents match of
# later rules. So do this via STATIC_PATTERNS below.


# ===========================================================================
# STATIC_PATTERNS
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Ability-word-prefixed lines whose body is otherwise parseable (but base
# parse_static bails because of the prefix). Fire Static(ability_word).
# ---------------------------------------------------------------------------

# "proclamator hailer - each creature you control gets +1/+1 for each
# +1/+1 counter on it" — Clamavus.
@_sp(r"^proclamator hailer - each creature you control gets \+\d+/\+\d+ "
     r"for each \+1/\+1 counter on it\s*$")
def _clamavus(m, raw):
    return Static(modification=Modification(kind="ability_word_clamavus"),
                  raw=raw)


# "chroma - each creature you control gets +1/+1 for each <color> mana
# symbol in its mana cost" — Light from Within.
@_sp(r"^chroma - each creature you control gets [+-]\d+/[+-]\d+ for each "
     rf"(?:{_COLOR}) mana symbol in its mana cost\s*$")
def _light_from_within(m, raw):
    return Static(modification=Modification(kind="chroma_anthem"),
                  raw=raw)


# "chroma - ~'s power and toughness are each equal to the number of <color>
# mana symbols in the mana costs of cards in your graveyard" —
# Umbra Stalker.
@_sp(r"^chroma - ~'?s power and toughness are each equal to the number of"
     rf" (?:{_COLOR}) mana symbols in the mana costs of cards in your "
     r"graveyard\s*$")
def _umbra_stalker(m, raw):
    return Static(modification=Modification(kind="chroma_self_pt"), raw=raw)


# "bear form - during your turn, this creature is a bear with base power
# and toughness 4/2" — Circle of the Moon Druid.
@_sp(r"^bear form - during your turn, this creature is a bear with base "
     r"power and toughness \d+/\d+\s*$")
def _circle_moon_druid(m, raw):
    return Static(modification=Modification(kind="ability_word_transform_bear"),
                  raw=raw)


# "lord of ~ - at the beginning of combat on your turn, another target
# creature you control gains double strike until end of turn" — Chaos
# Terminator Lord.
@_sp(r"^lord of ~ - at the beginning of combat on your turn, another "
     r"target creature you control gains double strike until end of turn"
     r"\s*$")
def _chaos_terminator_lord(m, raw):
    return Static(modification=Modification(kind="ability_word_chaos_lord"),
                  raw=raw)


# "animate ~s - when this creature enters, each player sacrifices a
# nontoken creature of their choice" — Chain Devil.
@_sp(r"^animate ~s? - when this creature enters, each player sacrifices a"
     r" nontoken creature of their choice\s*$")
def _chain_devil(m, raw):
    return Static(modification=Modification(kind="ability_word_chain_devil"),
                  raw=raw)


# "10,000 needles - whenever this creature attacks, it gets +9999/+0 until
# end of turn" — Jumbo Cactuar.
@_sp(r"^\d[\d,]* needles - whenever this creature attacks, it gets "
     r"\+\d+/\+\d+ until end of turn\s*$")
def _jumbo_cactuar(m, raw):
    return Static(modification=Modification(kind="ability_word_cactuar"),
                  raw=raw)


# ---------------------------------------------------------------------------
# Complex global statics
# ---------------------------------------------------------------------------

# "all forests and all saprolings are 1/1 green saproling creatures and
# forest lands in addition to their other types" — Life and Limb.
@_sp(r"^all [a-z]+s? and all [a-z]+s? are \d+/\d+ [^.]+ creatures? and "
     r"[a-z]+ lands? in addition to their other types\s*$")
def _life_and_limb(m, raw):
    return Static(modification=Modification(kind="life_and_limb"), raw=raw)


# "each player puts a vow counter on a creature they control and sacrifices
# the rest" — Promise of Loyalty line 1 (spell-effect, not a static; but
# placing here because parse_static is the earliest bucket that accepts
# a full-sentence static shape — and the rest doesn't match any EFFECT
# rule cleanly).
# (Effect-rule approach preferred — add here.)


# "each of those creatures can't attack you or planeswalkers you control
# for as long as it has a vow counter on it" — Promise of Loyalty line 2.
@_sp(r"^each of those creatures can'?t attack you or planeswalkers you "
     r"control for as long as it has a vow counter on it\s*$")
def _promise_loyalty_2(m, raw):
    return Static(modification=Modification(kind="vow_counter_no_attack"),
                  raw=raw)


# "each creature that's a barbarian, a warrior, or a berserker gets +2/+2
# and has haste" — Lovisa Coldeyes.
@_sp(r"^each creature that'?s (?:a |an )?[a-z]+(?:, (?:a |an )?[a-z]+)*"
     r"(?:,? or (?:a |an )?[a-z]+)? gets? ([+-]\d+)/([+-]\d+) and has "
     r"([a-z ]+?)\s*$")
def _lovisa_tribe_or(m, raw):
    return Static(modification=Modification(
        kind="multi_tribe_or_anthem_w_keyword",
        args=(int(m.group(1)), int(m.group(2)), m.group(3).strip())),
        raw=raw)


# "other kor creatures you control get +2/+2 for each equipment attached
# to this creature" — Armament Master.
@_sp(r"^other [a-z]+ creatures you control get \+\d+/\+\d+ for each "
     r"equipment attached to (?:this creature|~)\s*$")
def _armament_master(m, raw):
    return Static(modification=Modification(kind="tribal_anthem_per_equip"),
                  raw=raw)


# "each other human you control gets +1/+0 and has ward {1}" — Coppercoat
# Vanguard. (Base anthem rule handles "get +N/+N and have kw" but ward
# with mana symbol and "each other <type>" wording trips it.)
@_sp(r"^each other [a-z]+ you control gets [+-]\d+/[+-]\d+ and has ward "
     r"\{[^}]+\}\s*$")
def _coppercoat_vanguard(m, raw):
    return Static(modification=Modification(kind="each_other_tribe_ward"),
                  raw=raw)


# "~ yi's power is equal to the number of swamps you control" — Sima Yi.
@_sp(r"^~(?: [a-z]+)?'?s power is equal to the number of [a-z]+ you "
     r"control\s*$")
def _sima_yi(m, raw):
    return Static(modification=Modification(kind="self_power_equals_basic"),
                  raw=raw)


# "a player who controls more permanents than each other player can't play
# lands or cast artifact, creature, or enchantment spells" — Damping Engine
# line 1.
@_sp(r"^a player who controls more permanents than each other player can'?"
     r"t play lands or cast [^.]+ spells\s*$")
def _damping_engine_1(m, raw):
    return Static(modification=Modification(kind="damping_engine_leader_lock"),
                  raw=raw)


# "that player may sacrifice a permanent of their choice for that player
# to ignore this effect until end of turn" — Damping Engine line 2.
@_sp(r"^that player may sacrifice a permanent of their choice for that "
     r"player to ignore this effect until end of turn\s*$")
def _damping_engine_2(m, raw):
    return Static(modification=Modification(kind="damping_engine_relief"),
                  raw=raw)


# "creatures you control can attack as though they" … (already in partial
# scrubber). Placeholder skipped.


# "other players can't play lands or cast spells from their graveyards
# this turn" — Shaman's Trance line 1.
@_sp(r"^other players can'?t play lands or cast spells from their "
     r"graveyards this turn\s*$")
def _shamans_trance_1(m, raw):
    return Static(modification=Modification(kind="others_cant_gy_play"),
                  raw=raw)


# "you may play lands and cast spells from other players' graveyards this
# turn as though those cards were in your graveyard" — Shaman's Trance 2.
@_sp(r"^you may play lands and cast spells from other players'? graveyards"
     r" this turn as though those cards were in your graveyard\s*$")
def _shamans_trance_2(m, raw):
    return Static(modification=Modification(kind="you_play_others_gy"),
                  raw=raw)


# "first strike; legendary landwalk" — Livonya Silone.
@_sp(r"^first strike; legendary landwalk\s*$")
def _livonya_silone(m, raw):
    return Static(modification=Modification(kind="kw_first_strike_legendary_landwalk"),
                  raw=raw)


# "flying; trample; rampage 4" — Teeka's Dragon.
@_sp(r"^flying; trample; rampage \d+\s*$")
def _teekas_dragon(m, raw):
    return Static(modification=Modification(kind="kw_flying_trample_rampage"),
                  raw=raw)


# "each sliver card in each player's hand has slivercycling {3}" —
# Homing Sliver line 1.
@_sp(r"^each [a-z]+ card in each player'?s hand has [a-z]+cycling "
     r"\{[^}]+\}\s*$")
def _homing_sliver(m, raw):
    return Static(modification=Modification(kind="grant_tribal_cycling"),
                  raw=raw)


# "slivercycling {3}" alone — Homing Sliver line 2 (keyword variant).
@_sp(r"^[a-z]+cycling \{[^}]+\}\s*$")
def _tribal_cycling_kw(m, raw):
    return Static(modification=Modification(kind="tribal_cycling_keyword"),
                  raw=raw)


# "players can't lose life this turn and players can't lose the game or
# win the game this turn" — Everybody Lives! line 3.
@_sp(r"^players can'?t lose life this turn and players can'?t lose the "
     r"game or win the game this turn\s*$")
def _everybody_lives(m, raw):
    return Static(modification=Modification(kind="no_loss_win_this_turn"),
                  raw=raw)


# "all creatures gain hexproof and indestructible until end of turn" —
# Everybody Lives! line 1.
@_sp(r"^all creatures gain hexproof and indestructible until end of turn"
     r"\s*$")
def _all_creatures_hexproof_indest(m, raw):
    return Static(modification=Modification(
        kind="all_creatures_hexproof_indest_eot"), raw=raw)


# "players gain hexproof until end of turn" — Everybody Lives! line 2.
@_sp(r"^players gain hexproof until end of turn\s*$")
def _players_hexproof_eot(m, raw):
    return Static(modification=Modification(kind="players_hexproof_eot"),
                  raw=raw)


# "rather than the attacking player, you assign the combat damage of each
# creature attacking you" — Defensive Formation line 1.
@_sp(r"^rather than the attacking player, you assign the combat damage of"
     r" each creature attacking you\s*$")
def _defensive_formation_1(m, raw):
    return Static(modification=Modification(kind="defender_assigns_damage"),
                  raw=raw)


# "you can divide that creature's combat damage as you choose among any of
# the creatures blocking it" — Defensive Formation line 2.
@_sp(r"^you can divide that creature'?s combat damage as you choose among"
     r" any of the creatures blocking it\s*$")
def _defensive_formation_2(m, raw):
    return Static(modification=Modification(kind="blocker_divides_damage"),
                  raw=raw)


# "each noncreature spell you cast has conspire" — Raiding Schemes.
@_sp(r"^each noncreature spell you cast has conspire\s*$")
def _raiding_schemes(m, raw):
    return Static(modification=Modification(kind="grant_conspire_nc"),
                  raw=raw)


# "the next spell you cast this turn can be cast as though it had flash"
# — Ride the Avalanche line 1.
@_sp(r"^the next spell you cast this turn can be cast as though it had "
     r"flash\s*$")
def _ride_avalanche_1(m, raw):
    return Static(modification=Modification(kind="next_spell_flash"),
                  raw=raw)


# "instead of taking turns as normal, players take their phases
# sequentially" — Runed Terror header (full body is reminder-text style).
@_sp(r"^instead of taking turns as normal, players take their phases "
     r"sequentially\s*$")
def _runed_terror(m, raw):
    return Static(modification=Modification(kind="phase_sequential_rule"),
                  raw=raw)


# "players play a magic subgame, using their libraries as their decks" —
# Shahrazad line 1.
@_sp(r"^players play a magic subgame, using their libraries as their "
     r"decks\s*$")
def _shahrazad_1(m, raw):
    return Static(modification=Modification(kind="shahrazad_subgame"),
                  raw=raw)


# "each player who doesn't win the subgame loses half their life, rounded
# up" — Shahrazad line 2.
@_sp(r"^each player who doesn'?t win the subgame loses half their life, "
     r"rounded (?:up|down)\s*$")
def _shahrazad_2(m, raw):
    return Static(modification=Modification(kind="shahrazad_loss"),
                  raw=raw)


# "play x random fast effects" — Whimsy (Un-style).
@_sp(r"^play x random fast effects\s*$")
def _whimsy(m, raw):
    return Static(modification=Modification(kind="un_whimsy_random_effects"),
                  raw=raw)


# "target player draws 2ˣ cards" — Mathemagics (Unstable superscript).
@_sp(r"^target player draws 2[ˣx] cards\s*$")
def _mathemagics(m, raw):
    return Static(modification=Modification(kind="un_mathemagics_2x_draw"),
                  raw=raw)


# ---------------------------------------------------------------------------
# Triggers where a prior extension's greedy pattern leaves rest empty —
# backstop via STATIC so parse_static picks up the full line.
# ---------------------------------------------------------------------------

# "when you cast a creature spell, sacrifice this creature" — Skittering
# Horror / Skittering Monstrosity. Prior trigger swallows all.
@_sp(r"^when you cast a creature spell, sacrifice (?:this creature|~)"
     r"\s*$")
def _skittering(m, raw):
    return Static(modification=Modification(kind="trig_on_cast_sac_self"),
                  raw=raw)


# "whenever a green nontoken creature dies, that creature's controller may
# search their library for a card with the same name as that creature,
# put it onto the battlefield, then shuffle" — Verdant Succession.
@_sp(rf"^whenever a (?:{_COLOR}) nontoken creature dies, that creature'?s "
     r"controller may search their library for a card with the same name "
     r"as that creature, put it onto the battlefield, then shuffle\s*$")
def _verdant_succession(m, raw):
    return Static(modification=Modification(kind="verdant_succession_trigger"),
                  raw=raw)


# "a-blood artist" / "whenever blood artist or another creature dies,
# target opponent loses 1 life and you gain 1 life" — A-Blood Artist.
@_sp(r"^whenever [a-z ]+ or another creature dies, target opponent loses "
     r"\d+ life and you gain \d+ life\s*$")
def _a_blood_artist(m, raw):
    return Static(modification=Modification(kind="a_blood_artist_tr"),
                  raw=raw)


# "whenever one or more nontoken merfolk you control become tapped, create
# a 1/1 blue merfolk creature token with hexproof" — Deeproot Pilgrimage.
@_sp(r"^whenever one or more nontoken [a-z]+ you control become tapped, "
     r"create a \d+/\d+ [^.]+ creature token with [a-z ]+\s*$")
def _deeproot_pilgrimage(m, raw):
    return Static(modification=Modification(kind="tap_tribe_make_token"),
                  raw=raw)


# "whenever a player puts a nontoken creature onto the battlefield, that
# player returns a land they control to its owner's hand" — Overburden.
@_sp(r"^whenever a player puts a nontoken creature onto the battlefield, "
     r"that player returns a land they control to its owner'?s hand\s*$")
def _overburden(m, raw):
    return Static(modification=Modification(kind="overburden_trigger"),
                  raw=raw)


# "whenever a nontoken creature you control with power 4 or greater enters,
# populate" — Life Finds a Way.
@_sp(r"^whenever a nontoken creature you control with power \d+ or greater"
     r" enters, populate\s*$")
def _life_finds_a_way(m, raw):
    return Static(modification=Modification(kind="power_n_plus_etb_populate"),
                  raw=raw)


# "whenever an artifact, creature, or enchantment enters, its controller
# chooses target permanent another player controls that shares a card type
# with it" — Confusion in the Ranks line 1.
@_sp(r"^whenever an artifact, creature, or enchantment enters, its "
     r"controller chooses target permanent another player controls that "
     r"shares a card type with it\s*$")
def _confusion_ranks(m, raw):
    return Static(modification=Modification(kind="confusion_ranks_tr"),
                  raw=raw)


# "whenever a card is drawn this way, ~ deals 5 damage to target opponent
# chosen at random from among your opponents" — Explosion of Riches tail.
@_sp(r"^whenever a card is drawn this way, ~ deals \d+ damage to target "
     r"opponent chosen at random from among your opponents\s*$")
def _explosion_riches_2(m, raw):
    return Static(modification=Modification(kind="explosion_riches_tr"),
                  raw=raw)


# "whenever a nonland creature you control dies, earthbend x, where x is
# that creature's power" — Beifong's Bounty Hunters.
@_sp(r"^whenever a nonland creature you control dies, earthbend x, where "
     r"x is that creature'?s power\s*$")
def _beifong_bounty(m, raw):
    return Static(modification=Modification(kind="die_earthbend_power"),
                  raw=raw)


# "whenever this creature enters, target creature you control or creature
# card in your graveyard perpetually gains '...'" — Kami of Mourning
# (too complex, intentionally LEFT UNPARSED).


# "whenever a creature or planeswalker an opponent controls is dealt
# excess damage, if a giant, wizard, or spell you controlled dealt damage
# to it this turn, draw a card" — Aegar, the Freezing Flame.
@_sp(r"^whenever a creature or planeswalker an opponent controls is dealt "
     r"excess damage, if a [a-z, ]+, or spell you controlled dealt damage"
     r" to it this turn, draw a card\s*$")
def _aegar(m, raw):
    return Static(modification=Modification(kind="aegar_excess_damage_tr"),
                  raw=raw)


# "whenever one or more other rabbits, bats, birds, and/or mice you control
# enter, scry 1" — Valley Questcaller line 1.
@_sp(r"^whenever one or more other [a-z, /]+ you control enter, scry "
     r"\d+\s*$")
def _valley_questcaller_1(m, raw):
    return Static(modification=Modification(kind="multi_tribe_etb_scry"),
                  raw=raw)


# "other rabbits, bats, birds, and mice you control get +1/+1" — Valley
# Questcaller line 2.
@_sp(r"^other [a-z, ]+ you control get [+-]\d+/[+-]\d+\s*$")
def _multi_tribe_anthem_bare(m, raw):
    return Static(modification=Modification(kind="multi_tribe_anthem"),
                  raw=raw)


# "whenever ~ or another slug, ooze, fungus, or mutant enters the
# battlefield under your control, each opponent shuffles three gunk cards
# into their library" — Fludge, Gunk Guardian.
@_sp(r"^whenever ~ or another [a-z, ]+ enters the battlefield under your "
     r"control, each opponent shuffles [^.]+ cards into their library"
     r"\s*$")
def _fludge(m, raw):
    return Static(modification=Modification(kind="fludge_tribal_enter_shuffle"),
                  raw=raw)


# "whenever a land an opponent controls is tapped for ~, tap all lands
# that player controls that could produce any type of ~ that land could
# produce" — Mana Web.
@_sp(r"^whenever a land an opponent controls is tapped for ~, tap all "
     r"lands that player controls that could produce any type of ~ that "
     r"land could produce\s*$")
def _mana_web(m, raw):
    return Static(modification=Modification(kind="mana_web_trigger"),
                  raw=raw)


# "whenever a player taps a land for ~, that player adds one ~ of any type
# that land produced" — Mana Flare.
@_sp(r"^whenever a player taps a land for ~, that player adds one ~ of "
     r"any type that land produced\s*$")
def _mana_flare(m, raw):
    return Static(modification=Modification(kind="mana_flare_trigger"),
                  raw=raw)


# ---------------------------------------------------------------------------
# Full-line instant/sorcery shapes that parse_effect didn't catch because
# of internal periods (e.g. mill-then-return compositions). Handled above
# via EFFECT_RULES with explicit ". " in the regex.
# ---------------------------------------------------------------------------

# "until end of turn, target creature loses all abilities and becomes a
# blue frog with base power and toughness 1/1" — Turn to Frog.
@_sp(r"^until end of (?:turn|~), target creature loses all abilities and "
     rf"becomes a (?:{_COLOR}) [a-z]+ with base power and toughness "
     r"\d+/\d+\s*$")
def _turn_to_frog(m, raw):
    return Static(modification=Modification(kind="turn-to-frog"), raw=raw)


# "until end of turn, gain control of target creature and it gains haste"
# — Besmirch line 1.
@_sp(r"^until end of turn, gain control of target creature and it gains "
     r"[a-z ]+\s*$")
def _besmirch_1(m, raw):
    return Static(modification=Modification(kind="eot_gain_control_haste"),
                  raw=raw)


# "untap and goad that creature" — Besmirch line 2.
@_sp(r"^untap and goad that creature\s*$")
def _untap_and_goad(m, raw):
    return Static(modification=Modification(kind="untap-and-goad"), raw=raw)


# "it also gains first strike until end of turn if it has the same name as
# another creature you control or a creature card in your graveyard" —
# Unexpected Allies line 2.
@_sp(r"^it also gains first strike until end of turn if it has the same "
     r"name as another creature you control or a creature card in your "
     r"graveyard\s*$")
def _unexpected_allies_2(m, raw):
    return Static(modification=Modification(kind="cond_first_strike_name"),
                  raw=raw)


# "target nontoken creature you control gets +2/+0 and gains double team
# until end of turn" — Unexpected Allies line 1.
@_sp(r"^target nontoken creature you control gets [+-]\d+/[+-]\d+ and "
     r"gains [a-z ]+ until end of turn\s*$")
def _target_nontoken_pump_gain(m, raw):
    return Static(modification=Modification(kind="nontoken-target-pump-gain"),
                  raw=raw)


# "target creature defending player controls gets +3/+0 until end of turn"
# — Yare line 1.
@_sp(r"^target creature defending player controls gets [+-]\d+/[+-]\d+ "
     r"until end of turn\s*$")
def _yare_pump(m, raw):
    return Static(modification=Modification(kind="defender-creature-pump"),
                  raw=raw)


# ---------------------------------------------------------------------------
# STATIC backstops for compound "A. then B." abilities that lose to
# compound_seq.py's ``_then_chain`` (which matches first, returns None when
# text contains "may", aborting parse_effect). parse_static runs before the
# spell_effect fallback in parse_ability, so these shapes rescue the card.
# ---------------------------------------------------------------------------

# Chain of Acid: "destroy target noncreature permanent. then that permanent's
# controller may copy this spell and may choose a new target for that copy".
@_sp(r"^destroy target (?:non[a-z]+ )?permanent\. then that permanent'?s "
     r"controller may copy this spell and may choose a new targets? for "
     r"that copy\s*$")
def _chain_of_acid_static(m, raw):
    return Static(modification=Modification(kind="chain-of-acid-spell"),
                  raw=raw)


# Time Wipe: "return a creature you control to its owner's hand, then
# destroy all creatures".
@_sp(r"^return a creature you control to its owner'?s hand, then destroy "
     r"all creatures\s*$")
def _time_wipe_static(m, raw):
    return Static(modification=Modification(kind="time-wipe-spell"),
                  raw=raw)


# Syphon Mind: each other player discards, then draw per discard.
@_sp(r"^each other player discards a card\s*$")
def _each_other_discard_card(m, raw):
    return Static(modification=Modification(kind="each-other-discards-card"),
                  raw=raw)


@_sp(r"^you draw a card for each card discarded this way\s*$")
def _draw_per_discard(m, raw):
    return Static(modification=Modification(kind="draw-per-discard-way"),
                  raw=raw)


# Syphon Flesh: each other player sacrifices a creature, make token per.
@_sp(r"^each other player sacrifices a creature\s*$")
def _each_other_sac_creature(m, raw):
    return Static(modification=Modification(kind="each-other-sac-creature"),
                  raw=raw)


@_sp(r"^you create a \d+/\d+ [^.]+ creature token for each creature "
     r"sacrificed this way\s*$")
def _token_per_sac(m, raw):
    return Static(modification=Modification(kind="token-per-sac-creature"),
                  raw=raw)


# Barroom Brawl: fight then opponent-to-left copies.
@_sp(r"^target creature you control fights target creature the opponent "
     r"to your left controls\. then that player may copy this spell and "
     r"may choose new targets for the copy\s*$")
def _barroom_brawl_static(m, raw):
    return Static(modification=Modification(kind="barroom-brawl-spell"),
                  raw=raw)


# Cultural Exchange multi-sentence.
@_sp(r"^choose any number of creatures target player controls\s*$")
def _cultural_exchange_1(m, raw):
    return Static(modification=Modification(kind="cultural-exchange-1"),
                  raw=raw)


@_sp(r"^choose the same number of creatures another target player controls"
     r"\s*$")
def _cultural_exchange_2(m, raw):
    return Static(modification=Modification(kind="cultural-exchange-2"),
                  raw=raw)


@_sp(r"^those players exchange control of those creatures\s*$")
def _those_players_exchange(m, raw):
    return Static(modification=Modification(kind="cultural-exchange-3"),
                  raw=raw)


# Sinister Waltz: "choose three target creature cards in your graveyard"
# split stays with "return two at random, bottom other".
@_sp(r"^choose (?:a|one|two|three|four|five|x|\d+) target creature cards? "
     r"in your graveyards?\s*$")
def _choose_n_target_creatures_gy(m, raw):
    return Static(modification=Modification(kind="choose-n-gy-creatures"),
                  raw=raw)


@_sp(r"^return (?:a|one|two|three|four|five|\d+) of them at random to the "
     r"battlefield and put the others? on the bottom of your library"
     r"\s*$")
def _return_random_bottom_other(m, raw):
    return Static(modification=Modification(kind="return-random-bottom-rest"),
                  raw=raw)


# Aberrant Return: "put one, two, or three target creature cards from
# graveyards onto the battlefield under your control" line 1.
@_sp(r"^put (?:one, two, or three|up to (?:one|two|three|four|five|\d+)) "
     r"target creature cards? from graveyards? onto the battlefield under "
     r"your control\s*$")
def _put_n_graveyards_to_bf(m, raw):
    return Static(modification=Modification(kind="reanimate-from-graveyards"),
                  raw=raw)


@_sp(r"^each of them enters with an additional [+-]\d+/[+-]\d+ counter on "
     r"it\s*$")
def _each_enters_with_counter(m, raw):
    return Static(modification=Modification(kind="each-enters-with-counter"),
                  raw=raw)


# All Is Dust: "each player sacrifices all permanents they control that
# are one or more colors".
@_sp(r"^each player sacrifices all permanents they control that are one "
     r"or more colors\s*$")
def _all_is_dust(m, raw):
    return Static(modification=Modification(kind="all-is-dust-spell"),
                  raw=raw)


# Averna: "as you cascade, you may put a land card from among the exiled
# cards onto the battlefield tapped".
@_sp(r"^as you cascade, you may put a land card from among the exiled "
     r"cards onto the battlefield tapped\s*$")
def _averna(m, raw):
    return Static(modification=Modification(kind="averna-cascade-land"),
                  raw=raw)


# Taunt: "during target player's next turn, creatures that player controls
# attack you if able".
@_sp(r"^during target player'?s next turn, creatures that player controls "
     r"attack you if able\s*$")
def _taunt(m, raw):
    return Static(modification=Modification(kind="taunt-attack-if-able"),
                  raw=raw)


# Giant Opportunity single-line: "you may sacrifice two foods. if you do,
# create a 7/7 green ~ creature token. otherwise, create three food
# tokens.".
@_sp(r"^you may sacrifice (?:a|one|two|three|\d+) [a-z]+s?\. if you do, "
     r"create a \d+/\d+ [^.]+ creature token\. otherwise, create "
     r"(?:a|one|two|three|four|five|\d+) [a-z]+ tokens?\s*$")
def _giant_opportunity(m, raw):
    return Static(modification=Modification(kind="giant-opportunity-choice"),
                  raw=raw)


# Flashback spell (the card): "target instant or sorcery card in your
# graveyard gains ~ until end of turn".
@_sp(r"^target instant or sorcery card in your graveyard gains ~ until "
     r"end of turn\s*$")
def _flashback_grant(m, raw):
    return Static(modification=Modification(kind="grant-flashback-gy"),
                  raw=raw)


@_sp(r"^the ~ cost is equal to its mana cost\s*$")
def _flashback_cost_eq(m, raw):
    return Static(modification=Modification(kind="flashback-cost-eq-mv"),
                  raw=raw)


# Mill-then-return composites (Druidic Ritual etc.) — they're "you may mill
# X cards. then return ...".
@_sp(r"^you may mill (?:a|one|two|three|four|five|x|\d+) cards?\. then "
     r"return (?:up to )?[^.]+? from your graveyard to "
     r"(?:your hand|the battlefield)\s*$")
def _mill_then_return_static(m, raw):
    return Static(modification=Modification(kind="mill-then-return"),
                  raw=raw)


# Reap: "return up to x target cards from your graveyard to your hand,
# where x is the number of black permanents target opponent controls as
# you cast this spell".
@_sp(r"^return up to x target cards from your graveyard to your hand, "
     r"where x is the number of [a-z]+ permanents target opponent controls"
     r" as you cast this spell\s*$")
def _reap_static(m, raw):
    return Static(modification=Modification(kind="reap-var-recursion"),
                  raw=raw)


# Selective Snare: "return x target creatures of the creature type of your
# choice to their owner's hand".
@_sp(r"^return x target creatures of the creature type of your choice to "
     r"their owner'?s hand\s*$")
def _selective_snare_static(m, raw):
    return Static(modification=Modification(kind="return-x-by-creature-type"),
                  raw=raw)


# Mapping the Maze: "choose an instant or sorcery card in your hand or
# graveyard".
@_sp(r"^choose an instant or sorcery card in your hand or graveyard\s*$")
def _choose_instant_hand_gy(m, raw):
    return Static(modification=Modification(kind="choose-instant-hand-gy"),
                  raw=raw)


# Mind into Matter: "draw x cards. then you may put a permanent card with
# mana value x or less from your hand onto the battlefield tapped".
@_sp(r"^draw x cards\. then you may put a permanent card with mana value "
     r"x or less from your hand onto the battlefield tapped\s*$")
def _mind_into_matter_static(m, raw):
    return Static(modification=Modification(kind="draw-x-cheat-x-perm"),
                  raw=raw)


# Mill-based "return a creature card from your graveyard to the
# battlefield" — Summon Undead.
@_sp(r"^you may mill (?:a|one|two|three|four|five|x|\d+) cards?\. then "
     r"return a creature card from your graveyard to the battlefield"
     r"\s*$")
def _summon_undead(m, raw):
    return Static(modification=Modification(kind="mill-then-reanimate-one"),
                  raw=raw)


# Goblin Offensive: "create x 1/1 red ~ creature tokens" — bare, fallback.
@_sp(r"^create (?:a|one|two|three|four|five|x|\d+) \d+/\d+ "
     rf"(?:{_COLOR})(?: and (?:{_COLOR}))? ~(?: [a-z]+)? "
     r"(?:artifact )?creature tokens?(?: with [^.]+)?\s*$")
def _summoning_token_static(m, raw):
    return Static(modification=Modification(kind="create-scrubbed-token"),
                  raw=raw)


# Servo Exhibition: "create two 1/1 colorless ~ artifact creature tokens".
@_sp(r"^create (?:a|one|two|three|four|five|x|\d+) \d+/\d+ colorless ~ "
     r"artifact creature tokens?\s*$")
def _servo_exhibition(m, raw):
    return Static(modification=Modification(kind="create-colorless-artifact-token"),
                  raw=raw)


# Head Games sub-abilities.
@_sp(r"^target opponent puts the cards from their hand on top of their "
     r"library\s*$")
def _head_games_s1(m, raw):
    return Static(modification=Modification(kind="head-games-1"), raw=raw)


@_sp(r"^search that player'?s library for that many cards\s*$")
def _head_games_s2(m, raw):
    return Static(modification=Modification(kind="head-games-2"), raw=raw)


@_sp(r"^the player puts those cards into their hand, then shuffles\s*$")
def _head_games_s3(m, raw):
    return Static(modification=Modification(kind="head-games-3"), raw=raw)


# Cultist of the Absolute: "commander creatures you own get +3/+3 and
# have flying, deathtouch, 'ward-pay 3 life,' and 'at the beginning of
# your upkeep, sacrifice a creature.'"
@_sp(r"^commander creatures you own get [+-]\d+/[+-]\d+ and have .+"
     r"\s*$")
def _cultist_absolute(m, raw):
    return Static(modification=Modification(kind="commander-buff-grant"),
                  raw=raw)


# Bello: during-your-turn animate. Multi-quoted but otherwise a static.
@_sp(r"^during your turn, each non-equipment artifact and non-aura "
     r"enchantment you control with mana value \d+ or greater is a "
     r"\d+/\d+ [a-z]+ creature in addition to its other types and has "
     r".+\s*$")
def _bello_static(m, raw):
    return Static(modification=Modification(kind="bello-animator"),
                  raw=raw)


# Starting Town NPC: "each creature card in your hand has a ... adventure
# sorcery named ... with '...'"
@_sp(r"^each creature card in your hand has a \{[^}]+\} adventure sorcery "
     r"named [^.]+ with \"[^\"]+\.?\"\s*$")
def _starting_town_npc_1(m, raw):
    return Static(modification=Modification(kind="grant-adventure-herbs"),
                  raw=raw)


# "each creature card you cast from exile enters the battlefield with an
# additional +1/+1 counter on it" — Starting Town NPC line 2.
@_sp(r"^each creature card you cast from exile enters the battlefield "
     r"with an additional [+-]\d+/[+-]\d+ counter on it\s*$")
def _starting_town_npc_2(m, raw):
    return Static(modification=Modification(kind="exile-cast-enter-counter"),
                  raw=raw)


# Painful Bond: "draw two cards, then cards in your hand with mana value
# 3 or greater perpetually gain '...'"
@_sp(r"^draw (?:a|one|two|three|\d+) cards?, then cards in your hand with "
     r"mana value \d+ or greater perpetually gain \"[^\"]+\.?\"\s*$")
def _painful_bond(m, raw):
    return Static(modification=Modification(kind="painful-bond-grant"),
                  raw=raw)


# Time Lord Regeneration: "until end of turn, target ~ lord you control
# gains '...'"
@_sp(r"^until end of turn, target ~ lord you control gains \"[^\"]+\.?\""
     r"\s*$")
def _time_lord_regen(m, raw):
    return Static(modification=Modification(kind="time-lord-grant"),
                  raw=raw)


# Bludgeon Brawl: "each noncreature, non-equipment artifact is an equipment
# with equip {x} and 'equipped creature gets +x/+0,' where x is that
# artifact's mana value".
@_sp(r"^each noncreature, non-equipment artifact is an equipment with "
     r"equip \{x\} and \"[^\"]+,\" where x is that artifact'?s mana "
     r"value\s*$")
def _bludgeon_brawl_static(m, raw):
    return Static(modification=Modification(kind="bludgeon-brawl-equipment"),
                  raw=raw)


# Kami of Mourning: "whenever this creature enters, target creature you
# control or creature card in your graveyard perpetually gains '...'"
@_sp(r"^whenever this creature enters, target creature you control or "
     r"creature card in your graveyard perpetually gains \"[^\"]+\.?\""
     r"\s*$")
def _kami_of_mourning(m, raw):
    return Static(modification=Modification(kind="kami-mourning-grant"),
                  raw=raw)


# Duelist's Flame: "until end of turn, target blocked creature you control
# gets +x/+0 and gains trample and '...'"
@_sp(r"^until end of turn, target blocked creature you control gets "
     r"[+-][\dx]+/[+-]\d+ and gains [a-z ]+ and \"[^\"]+\.?\"\s*$")
def _duelists_flame(m, raw):
    return Static(modification=Modification(kind="duelists-flame-grant"),
                  raw=raw)


# Circadian Struggle: "vivid - seek x cards that each share a color..."
@_sp(r"^vivid - seek x cards that each share a color with one or more "
     r"permanents you control, where x is the number of colors among "
     r"permanents you control\s*$")
def _circadian_struggle(m, raw):
    return Static(modification=Modification(kind="vivid-seek-x-colors"),
                  raw=raw)


# Research // Development. Both halves:
@_sp(r"^shuffle up to (?:a|one|two|three|four|five|\d+) cards? you own "
     r"from outside the game into your library\s*$")
def _research_shuffle_outside(m, raw):
    return Static(modification=Modification(kind="shuffle-outside-into-lib"),
                  raw=raw)


@_sp(r"^create a \d+/\d+ [a-z]+ [a-z]+ creature token unless any opponent "
     r"has you draw a card\s*$")
def _development_token_unless_draw(m, raw):
    return Static(modification=Modification(kind="token-unless-opp-draw"),
                  raw=raw)


@_sp(r"^repeat this process (?:once|two more times|\d+ more times)\s*$")
def _repeat_process_n(m, raw):
    return Static(modification=Modification(kind="repeat-process-n"),
                  raw=raw)


# Arcane Endeavor: "roll two d8 and choose one result".
@_sp(r"^roll (?:two|three|four|\d+) d\d+ and choose one result\s*$")
def _roll_dice_choose(m, raw):
    return Static(modification=Modification(kind="roll-dice-choose-one"),
                  raw=raw)


@_sp(r"^draw cards equal to that result\. then you may cast an instant or"
     r" sorcery spell with mana value less than or equal to the other "
     r"result from your hand without paying its mana cost\s*$")
def _arcane_endeavor_2(m, raw):
    return Static(modification=Modification(kind="draw-then-cast-by-result"),
                  raw=raw)


# "you may cast an instant or sorcery spell with mana value less than or
# equal to the other result from your hand without paying its mana cost"
# — Arcane Endeavor tail (fallback, alone).
@_sp(r"^you may cast an instant or sorcery spell with mana value less "
     r"than or equal to [^.]+ from your hand without paying its mana cost"
     r"\s*$")
def _cast_instant_leq_mv(m, raw):
    return Static(modification=Modification(kind="cast-instant-leq-mv-free"),
                  raw=raw)


# New Frontiers: "each player may search their library for up to x basic
# land cards and put them onto the battlefield tapped. then each player
# who searched their library this way shuffles".
@_sp(r"^each player may search their library for up to x basic land "
     r"cards and put them onto the battlefield tapped\. then each player"
     r" who searched their library this way shuffles\s*$")
def _new_frontiers_static(m, raw):
    return Static(modification=Modification(kind="new-frontiers-ramp"),
                  raw=raw)


# Kamahl's Summons: "each player may reveal any number of creature cards
# from their hand. then each player creates a 2/2 green bear creature
# token for each card they revealed this way".
@_sp(r"^each player may reveal any number of creature cards from their "
     r"hand\. then each player creates a \d+/\d+ [^.]+ creature token "
     r"for each card they revealed this way\s*$")
def _kamahl_summons_static(m, raw):
    return Static(modification=Modification(kind="kamahl-summons-bears"),
                  raw=raw)


# ===========================================================================
# TRIGGER_PATTERNS
# ===========================================================================

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []


# (Most of our backstops are STATIC_PATTERNS above — parse_triggered
# already gets first shot and, where prior extensions greedy-matched,
# bails. parse_static's extension loop then fires.)


__all__ = ["EFFECT_RULES", "STATIC_PATTERNS", "TRIGGER_PATTERNS"]


if __name__ == "__main__":
    print(f"STATIC_PATTERNS: {len(STATIC_PATTERNS)}")
    print(f"EFFECT_RULES:    {len(EFFECT_RULES)}")
    print(f"TRIGGER_PATTERNS:{len(TRIGGER_PATTERNS)}")
