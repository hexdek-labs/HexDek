#!/usr/bin/env python3
"""Scaling-amount effect rules — `where X is ...` / `equal to ...` parses.

Many MTG effects read an integer that depends on board state at resolution
time. Historically the parser caught the effect VERB (damage / lose_life /
draw / look_at / mill) but the trailing scaling clause ("where X is your
devotion to black") fell off into ``parsed_effect_residual`` or an
``UnknownEffect`` wrapping the whole sentence. The resolver then read the
amount as either static (usually 1) or treated the ability as a no-op.

This module registers EFFECT_RULES that produce typed nodes with
``amount=ScalingAmount(...)`` / ``count=ScalingAmount(...)``. The resolver
in ``playloop.resolve_amount`` dispatches on ``ScalingAmount.kind`` to
evaluate the expression against the current game state.

Named ``a_scaling_amounts.py`` so it loads BEFORE ``partial_final.py`` and
``color_devotion.py`` (alphabetical), giving its precise shapes priority
over the catch-alls that previously matched these clauses as opaque
``UnknownEffect`` blobs.

Patterns implemented:
  1. "each opponent loses X life, where X is your devotion to <color>"
     (Gray Merchant of Asphodel)
  2. "you gain X life, where X is your devotion to <color>"
     (Heliod's Pilgrim-class; also Karametra's Favor)
  3. "you lose X life, where X is your devotion to <color>"
     (symmetric)
  4. "~ deals X damage to <target>, where X is your devotion to <color>"
     (Fanatic of Mogis, Wingcrafter Phoenix variants)
  5. "~ deals damage to <target> equal to your devotion to <color>"
     (existing color_devotion pattern — now emits typed Damage)
  6. "each opponent loses N life equal to your devotion to <color>"
     (alternative phrasing)
  7. "look at the top X cards of your library, where X is your devotion to
     <color>" (Thassa's Oracle)
  8. "target player/you/each player mills X cards, where X is your
     devotion to <color>"
  9. "target player draws X cards, where X is your devotion to <color>"
     (Disciple of Deceit variants)
 10. "add X mana of <color>, where X is your devotion to <color>"
     / "add an amount of mana of that color equal to your devotion to
     that color" (Nykthos — emits AddMana with scaling pool)
 11. "creatures you control get +X/+X until end of turn, where X is your
     devotion to <color>" (Nylea pump — emits Buff with scaling P/T)
 12. "equal to the number of <creature-type|permanent|card> you control"
     (Baleful Eidolon-class, Rhonas-class)
 13. "equal to the number of cards in your <zone>"
     (Laboratory Maniac variants, Thought Collapse)

Patterns INTENTIONALLY deferred to a later wave (documented here so future
passes can pick them up without re-discovering):
  - "for each <something>" pump/damage (e.g. "deals damage to target
    creature equal to the number of creatures you control" IS covered;
    "target creature gets +1/+1 for each artifact you control" is not —
    it parses to a Buff with static P/T today).
  - Compound devotion ("equal to your devotion to black AND white" —
    Pharika's arena phrasing is rare enough to skip this wave).
  - Mana value filter predicates ("destroy each creature with mana value
    3 or less" — the Destroy node drops the predicate today; the filter
    is already produced with ``mana_value_op=None``. Fixing this would
    touch ``parse_filter`` in parser.py, which is out of scope for this
    extension file.)
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
    AddMana, Buff, Damage, Draw, Filter, GainLife, LookAt, LoseLife,
    ManaSymbol, Mill, Scry, ScalingAmount, Sequence, Tutor,
    TARGET_ANY, TARGET_CREATURE, TARGET_PLAYER, TARGET_OPPONENT,
    EACH_OPPONENT, EACH_PLAYER, SELF,
)

# Import parse_filter lazily so we don't circularly import parser.py at module
# load time (parser.py imports extensions via load_extensions()).
def _parse_filter(s):
    import parser as _p
    return _p.parse_filter(s)


EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


_COLOR_WORD = {"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G"}
_COLOR = r"(?:white|blue|black|red|green)"


# ============================================================================
# Devotion-scaled effects: "..., where X is your devotion to <color>"
# ============================================================================
#
# Grammar: <verb phrase with X> ", where x is your devotion to <color>"
# We match the whole sentence (including the `where` rider) so the engine
# carries the scaling amount rather than dropping it.

# "each opponent loses x life, where x is your devotion to <color>"
@_eff(
    rf"^each opponent loses x life,\s*where x is your devotion "
    rf"to (?P<c>{_COLOR})\.?$"
)
def _each_opp_lose_devotion(m):
    return LoseLife(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=EACH_OPPONENT,
    )


# "each player loses x life, where x is your devotion to <color>"
@_eff(
    rf"^each player loses x life,\s*where x is your devotion "
    rf"to (?P<c>{_COLOR})\.?$"
)
def _each_player_lose_devotion(m):
    return LoseLife(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=EACH_PLAYER,
    )


# "target player loses x life, where x is your devotion to <color>"
@_eff(
    rf"^target (?:opponent|player) loses x life,\s*where x is your devotion "
    rf"to (?P<c>{_COLOR})\.?$"
)
def _target_player_lose_devotion(m):
    # Use TARGET_OPPONENT when the phrasing says "opponent"; both resolve to
    # a single player slot at runtime.
    tgt = TARGET_OPPONENT if "opponent" in m.string.lower() else TARGET_PLAYER
    return LoseLife(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=tgt,
    )


# "you gain x life, where x is your devotion to <color>"
@_eff(
    rf"^you gain x life,\s*where x is your devotion to (?P<c>{_COLOR})\.?$"
)
def _you_gain_devotion(m):
    return GainLife(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=SELF,
    )


# "you lose x life, where x is your devotion to <color>"
@_eff(
    rf"^you lose x life,\s*where x is your devotion to (?P<c>{_COLOR})\.?$"
)
def _you_lose_devotion(m):
    return LoseLife(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=SELF,
    )


# "you draw x cards, where x is your devotion to <color>"
@_eff(
    rf"^you draw x cards,\s*where x is your devotion to (?P<c>{_COLOR})\.?$"
)
def _you_draw_devotion(m):
    return Draw(
        count=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=SELF,
    )


# "target player draws x cards, where x is your devotion to <color>"
@_eff(
    rf"^target (?:opponent|player) draws x cards,\s*where x is your devotion "
    rf"to (?P<c>{_COLOR})\.?$"
)
def _target_player_draw_devotion(m):
    tgt = TARGET_OPPONENT if "opponent" in m.string.lower() else TARGET_PLAYER
    return Draw(
        count=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=tgt,
    )


# "you mill x cards, where x is your devotion to <color>"
@_eff(
    rf"^you mill x cards,\s*where x is your devotion to (?P<c>{_COLOR})\.?$"
)
def _you_mill_devotion(m):
    return Mill(
        count=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=SELF,
    )


# "target player mills x cards, where x is your devotion to <color>"
@_eff(
    rf"^target (?:opponent|player) mills x cards,\s*where x is your devotion "
    rf"to (?P<c>{_COLOR})\.?$"
)
def _target_player_mill_devotion(m):
    tgt = TARGET_OPPONENT if "opponent" in m.string.lower() else TARGET_PLAYER
    return Mill(
        count=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=tgt,
    )


# "look at the top x cards of your library, where x is your devotion to <color>"
# (Thassa's Oracle ETB)
@_eff(
    rf"^look at the top x cards of your library,\s*where x is your devotion "
    rf"to (?P<c>{_COLOR})\.?$"
)
def _look_top_devotion(m):
    return LookAt(
        target=SELF,
        zone="library_top_n",
        count=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
    )


# "scry x, where x is your devotion to <color>"  (rare; Theros block)
@_eff(
    rf"^scry x,\s*where x is your devotion to (?P<c>{_COLOR})\.?$"
)
def _scry_devotion(m):
    return Scry(
        count=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
    )


# "~ deals x damage to <target>, where x is your devotion to <color>"
# (Fanatic of Mogis-class — the pronoun-resolution pass rewrites "it deals"
# to "this creature deals" BEFORE we see the text, but some raw inputs leave
# "it" intact, so both prefixes are accepted here.)
@_eff(
    rf"^(?:~|this creature|it) deals x damage to (?P<who>[^,]+?),\s*where x is your "
    rf"devotion to (?P<c>{_COLOR})\.?$"
)
def _self_damage_x_devotion(m):
    tgt = _parse_filter(m.group("who")) or TARGET_ANY
    return Damage(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=tgt,
    )


# ============================================================================
# Devotion-scaled effects: "...equal to your devotion to <color>"
# ============================================================================
#
# Some cards spell the scaling out without the "where X is" rider.

# "~ deals damage to <target> equal to your devotion to <color>"
# (Fanatic of Mogis / Master of Waves; "it" handles the undropped-pronoun
# form seen when the ETB trigger's "When this creature enters" prefix is
# stripped and the rest begins with a back-reference.)
@_eff(
    rf"^(?:~|this creature|it) deals damage to (?P<who>[^.]+?) equal to "
    rf"your devotion to (?P<c>{_COLOR})\.?$"
)
def _self_damage_eq_devotion(m):
    tgt = _parse_filter(m.group("who")) or TARGET_ANY
    return Damage(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=tgt,
    )


# "you gain life equal to your devotion to <color>"
@_eff(
    rf"^you gain life equal to your devotion to (?P<c>{_COLOR})\.?$"
)
def _you_gain_eq_devotion(m):
    return GainLife(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=SELF,
    )


# "target player loses life equal to your devotion to <color>"
@_eff(
    rf"^target (?:opponent|player) loses life equal to your devotion "
    rf"to (?P<c>{_COLOR})\.?$"
)
def _target_player_lose_eq_devotion(m):
    tgt = TARGET_OPPONENT if "opponent" in m.string.lower() else TARGET_PLAYER
    return LoseLife(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=tgt,
    )


# "each opponent loses life equal to your devotion to <color>"
@_eff(
    rf"^each opponent loses life equal to your devotion to (?P<c>{_COLOR})\.?$"
)
def _each_opp_lose_eq_devotion(m):
    return LoseLife(
        amount=ScalingAmount(kind="devotion", args=(_COLOR_WORD[m.group("c").lower()],)),
        target=EACH_OPPONENT,
    )


# ============================================================================
# Devotion-scaled Nykthos-style mana production
# ============================================================================
#
# Nykthos's "add an amount of mana of that color equal to your devotion to
# that color" is already caught by color_devotion.py as an UnknownEffect tag
# (``add_mana_eq_devotion_that_color``). We shadow that with a typed AddMana
# whose pool is empty but whose ``any_color_count`` is a ScalingAmount of
# kind ``devotion_chosen`` — the resolver knows to read ``chosen_color``
# from the activated-ability context and produce that many mana.

@_eff(
    r"^add an amount of mana of that color equal to your devotion "
    r"to that color\.?$"
)
def _nykthos_add(m):
    return AddMana(
        pool=(),
        # Encode the dynamic count as a ScalingAmount via a wrapper tuple.
        # AddMana's ``any_color_count`` is declared ``int`` but Python dataclass
        # type hints are not enforced; downstream readers already accept any
        # integer-like value. The resolver dispatches on ``isinstance(value,
        # ScalingAmount)`` to evaluate.
        any_color_count=ScalingAmount(kind="devotion_chosen_color"),
    )


# ============================================================================
# Devotion-scaled team pump
# ============================================================================

# "creatures you control get +x/+x until end of turn, where x is your
#  devotion to <color>" (Nylea, God of the Hunt)
@_eff(
    rf"^creatures you control get \+x/\+x until end of turn,\s*where x is "
    rf"your devotion to (?P<c>{_COLOR})\.?$"
)
def _team_pump_devotion(m):
    # Buff's power/toughness are int; we encode the scaling via a Modification
    # by wrapping the int slot in a ScalingAmount where runtime-duck-typing
    # accepts it. To keep Buff's structural signature intact we set power and
    # toughness to 0 and tag the scaling via the (ab-used) extra channel.
    # Engines that want the true value should call resolve_amount on
    # ScalingAmount(kind='devotion', args=(col,)).
    col = _COLOR_WORD[m.group("c").lower()]
    # Using Python's dataclass laxity to carry a ScalingAmount in the int
    # fields. The engine's Buff handler reads buff.power / buff.toughness via
    # resolve_amount so ScalingAmount Just Works; pure-int consumers (tests,
    # signature computation) coerce to "unknown int" (0 today, equivalent to
    # the pre-change static-1 treatment). The extra slot is a tuple so it
    # accepts our hint.
    scaling = ScalingAmount(kind="devotion", args=(col,))
    return Buff(
        power=scaling,
        toughness=scaling,
        target=Filter(base="creature", quantifier="all", you_control=True),
        duration="until_end_of_turn",
    )


# ============================================================================
# Counting permanents / cards in zones
# ============================================================================
#
# Many cards read an amount equal to a count of a predicate over a zone.
# We expose a generic ``count_filter_you_control`` / ``cards_in_zone``
# ScalingAmount so the resolver can walk the filter and count matches.

# "deals damage to <target> equal to the number of <noun> you control"
@_eff(
    r"^(?:~|this creature) deals damage to (?P<who>[^.]+?) equal to the number "
    r"of (?P<noun>[^.]+?) you control\.?$"
)
def _damage_eq_count_you_control(m):
    who = _parse_filter(m.group("who")) or TARGET_ANY
    noun_filter = _parse_filter(m.group("noun") + " you control") or \
                  Filter(base=m.group("noun").strip().rstrip("s"), you_control=True)
    return Damage(
        amount=ScalingAmount(kind="count_filter", args=(noun_filter,)),
        target=who,
    )


# "you gain life equal to the number of <noun> you control"
@_eff(
    r"^you gain life equal to the number of (?P<noun>[^.]+?) you control\.?$"
)
def _gain_life_eq_count_you_control(m):
    noun_filter = _parse_filter(m.group("noun") + " you control") or \
                  Filter(base=m.group("noun").strip().rstrip("s"), you_control=True)
    return GainLife(
        amount=ScalingAmount(kind="count_filter", args=(noun_filter,)),
        target=SELF,
    )


# "each opponent loses life equal to the number of <noun> you control"
@_eff(
    r"^each opponent loses life equal to the number of (?P<noun>[^.]+?) "
    r"you control\.?$"
)
def _each_opp_lose_eq_count_you_control(m):
    noun_filter = _parse_filter(m.group("noun") + " you control") or \
                  Filter(base=m.group("noun").strip().rstrip("s"), you_control=True)
    return LoseLife(
        amount=ScalingAmount(kind="count_filter", args=(noun_filter,)),
        target=EACH_OPPONENT,
    )


# "target player loses life equal to the number of <noun> you control"
@_eff(
    r"^target (?:opponent|player) loses life equal to the number of "
    r"(?P<noun>[^.]+?) you control\.?$"
)
def _target_player_lose_eq_count_you_control(m):
    tgt = TARGET_OPPONENT if "opponent" in m.string.lower() else TARGET_PLAYER
    noun_filter = _parse_filter(m.group("noun") + " you control") or \
                  Filter(base=m.group("noun").strip().rstrip("s"), you_control=True)
    return LoseLife(
        amount=ScalingAmount(kind="count_filter", args=(noun_filter,)),
        target=tgt,
    )


# "draw cards equal to the number of <noun> you control"
@_eff(
    r"^draw cards equal to the number of (?P<noun>[^.]+?) you control\.?$"
)
def _draw_eq_count_you_control(m):
    noun_filter = _parse_filter(m.group("noun") + " you control") or \
                  Filter(base=m.group("noun").strip().rstrip("s"), you_control=True)
    return Draw(
        count=ScalingAmount(kind="count_filter", args=(noun_filter,)),
        target=SELF,
    )


# ============================================================================
# Counting cards in a zone
# ============================================================================

# "deals damage to <target> equal to the number of cards in your graveyard"
@_eff(
    r"^(?:~|this creature) deals damage to (?P<who>[^.]+?) equal to the number "
    r"of cards in your graveyard\.?$"
)
def _damage_eq_cards_gy(m):
    who = _parse_filter(m.group("who")) or TARGET_ANY
    return Damage(
        amount=ScalingAmount(kind="cards_in_zone", args=("graveyard", "you")),
        target=who,
    )


# "deals damage to <target> equal to the number of cards in <whose> hand"
@_eff(
    r"^(?:~|this creature) deals damage to (?P<who>[^.]+?) equal to the number "
    r"of cards in (?P<whose>your|target player'?s?|each opponent'?s?|that player'?s?) "
    r"hand\.?$"
)
def _damage_eq_cards_hand(m):
    who = _parse_filter(m.group("who")) or TARGET_ANY
    whose_raw = m.group("whose").lower().rstrip("'s")
    whose_norm = {
        "your": "you",
        "target player": "target",
        "target player's": "target",
        "each opponent": "each_opp",
        "each opponent's": "each_opp",
        "that player": "target",
        "that player's": "target",
    }.get(whose_raw.rstrip("s"), "you")
    return Damage(
        amount=ScalingAmount(kind="cards_in_zone", args=("hand", whose_norm)),
        target=who,
    )


# "each opponent loses life equal to the number of cards in your graveyard"
@_eff(
    r"^each opponent loses life equal to the number of cards in your graveyard\.?$"
)
def _each_opp_lose_eq_cards_gy(m):
    return LoseLife(
        amount=ScalingAmount(kind="cards_in_zone", args=("graveyard", "you")),
        target=EACH_OPPONENT,
    )


# "you gain life equal to the number of cards in your graveyard"
@_eff(
    r"^you gain life equal to the number of cards in your graveyard\.?$"
)
def _you_gain_eq_cards_gy(m):
    return GainLife(
        amount=ScalingAmount(kind="cards_in_zone", args=("graveyard", "you")),
        target=SELF,
    )


__all__ = ["EFFECT_RULES"]
