#!/usr/bin/env python3
"""Specific effect rules for conjunction clause subjects.

Named with the ``a_`` prefix so it loads BEFORE ``partial_final.py`` (whose
`_target_noun_effect` / `_you_verb` / `_this_creature_*` catch-alls would
otherwise swallow these clauses as opaque UnknownEffect blobs). These rules
cover the left-hand / right-hand side shapes that survive the " and " trial-
split in ``parser._trial_and_split``:

- "target opponent loses N life"          (Vampire's Kiss, Sovereign's Bite)
- "defending player loses N life"         (Agate-Blade Assassin)
- "each opponent loses N life"            (existing _each_opp_lose covers; mirror here for safety)
- "target opponent/player gains N life"   (rare but observed)
- "this creature gets +X/+Y [until end of turn]"   (most +N/+N riders)
- "it gets +X/+Y [until end of turn]"
- "this creature gains KW [until end of turn]"     (gains trample, first strike, ...)
- "it gains KW [until end of turn]"
- "creatures you control gain KW [until end of turn]"
- "creatures you control get +X/+Y [until end of turn]"
- "target creature gets +X/+Y [until end of turn]"

All emit concrete typed AST nodes (Buff / GainLife / LoseLife / GrantAbility),
so the trial-split in parse_effect can see them as typed and commit the split
into a Sequence.
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
    AddMana, Buff, CreateToken, Effect, Filter, GainLife, GrantAbility,
    LoseLife, Sequence, Static, Modification, ManaSymbol, ManaCost,
    TARGET_CREATURE, TARGET_OPPONENT, TARGET_PLAYER, EACH_OPPONENT, SELF,
)


EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


_NUMS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}


def _n(tok: str):
    t = tok.lower()
    if t in _NUMS:
        return _NUMS[t]
    if t.isdigit():
        return int(t)
    return t  # leaves "x" as string


# -- Life loss with subjects the built-in rules don't cover --------------------

# "target opponent loses N life"
@_eff(r"^target opponent loses (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) life\.?$")
def _target_opp_lose(m):
    return LoseLife(amount=_n(m.group(1)), target=TARGET_OPPONENT)


# "defending player loses N life"
_DEFENDING_PLAYER = Filter(base="player", quantifier="one", targeted=False, extra=("defending",))


@_eff(r"^defending player loses (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) life\.?$")
def _def_player_lose(m):
    return LoseLife(amount=_n(m.group(1)), target=_DEFENDING_PLAYER)


# "that player loses N life"
_THAT_PLAYER = Filter(base="player", quantifier="one", targeted=False, extra=("that",))


@_eff(r"^that player loses (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) life\.?$")
def _that_player_lose(m):
    return LoseLife(amount=_n(m.group(1)), target=_THAT_PLAYER)


# "that player gains N life" (less common but shows up)
@_eff(r"^that player gains (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) life\.?$")
def _that_player_gain(m):
    return GainLife(amount=_n(m.group(1)), target=_THAT_PLAYER)


# -- +X/+Y buff with "this creature" / "it" subjects ---------------------------

_PT_RE = r"([+\-]\d+)/([+\-]\d+)"


def _parse_pt(power: str, toughness: str) -> tuple:
    def coerce(v):
        if v.startswith("+"):
            return int(v[1:])
        return int(v)
    return coerce(power), coerce(toughness)


# "this creature gets +X/+Y [until end of turn]"
@_eff(rf"^this creature gets {_PT_RE}(?: until end of turn)?\.?$")
def _this_creature_gets(m):
    p, t = _parse_pt(m.group(1), m.group(2))
    return Buff(power=p, toughness=t, target=Filter(base="self", targeted=False))


# "it gets +X/+Y [until end of turn]"  (pronoun-chained from prior sentence)
@_eff(rf"^it gets {_PT_RE}(?: until end of turn)?\.?$")
def _it_gets(m):
    p, t = _parse_pt(m.group(1), m.group(2))
    return Buff(power=p, toughness=t, target=Filter(base="pronoun_it", targeted=False))


# "target creature gets +X/+Y [until end of turn]"
@_eff(rf"^target creature gets {_PT_RE}(?: until end of turn)?\.?$")
def _target_creature_gets(m):
    p, t = _parse_pt(m.group(1), m.group(2))
    return Buff(power=p, toughness=t, target=TARGET_CREATURE)


# -- "<subject> gains KEYWORD [until end of turn]" -----------------------------
# Single-keyword grants from the subjects the catch-all would otherwise eat.

_GRANTABLE_KW = (
    r"flying|trample|haste|vigilance|deathtouch|lifelink|first strike|"
    r"double strike|reach|hexproof|indestructible|menace|defender|"
    r"flash|shroud|infect|wither|prowess|skulk|intimidate|fear|shadow|"
    r"horsemanship|swampwalk|islandwalk|forestwalk|mountainwalk|plainswalk|"
    r"landwalk|protection from \w+|ward \{[^}]+\}"
)


@_eff(rf"^this creature gains ({_GRANTABLE_KW})(?: until end of turn)?\.?$")
def _this_creature_gains(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="self", targeted=False),
    )


@_eff(rf"^it gains ({_GRANTABLE_KW})(?: until end of turn)?\.?$")
def _it_gains(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="pronoun_it", targeted=False),
    )


# "creatures you control get +X/+Y [until end of turn]"
_CREATURES_YOU_CONTROL = Filter(
    base="creature", quantifier="each", targeted=False, you_control=True,
)


@_eff(rf"^creatures? you control gets? {_PT_RE}(?: until end of turn)?\.?$")
def _creatures_you_control_get(m):
    p, t = _parse_pt(m.group(1), m.group(2))
    return Buff(power=p, toughness=t, target=_CREATURES_YOU_CONTROL)


# "creatures you control gain KW [until end of turn]"
@_eff(rf"^creatures? you control gains? ({_GRANTABLE_KW})(?: until end of turn)?\.?$")
def _creatures_you_control_gain(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=_CREATURES_YOU_CONTROL,
    )


# "another target creature you control gets +X/+Y" (Eel-Hounds style)
_ANOTHER_CYC = Filter(
    base="creature", quantifier="one", targeted=True, you_control=True,
    extra=("another",),
)


@_eff(rf"^another target creature you control gets {_PT_RE}(?: until end of turn)?\.?$")
def _another_target_creature_gets(m):
    p, t = _parse_pt(m.group(1), m.group(2))
    return Buff(power=p, toughness=t, target=_ANOTHER_CYC)


@_eff(rf"^another target creature you control gains ({_GRANTABLE_KW})(?: until end of turn)?\.?$")
def _another_target_creature_gains(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=_ANOTHER_CYC,
    )


# -- Artifact tokens (Treasure / Food / Blood / Clue / Map / Gold) -------------
# These aren't creature tokens so the built-in _token rule skips them. Add
# typed CreateToken nodes with `types=(kind, "token")` so dispatch can act.

_ARTIFACT_TOKEN_KINDS = (
    "treasure", "food", "blood", "clue", "map", "gold", "powerstone",
    "incubator", "junk", "walker",
)
_ARTIFACT_TOKEN_RE = "|".join(_ARTIFACT_TOKEN_KINDS)


@_eff(rf"^create (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) ({_ARTIFACT_TOKEN_RE}) tokens?\.?$")
def _create_artifact_token(m):
    n = _n(m.group(1))
    kind = m.group(2).strip().lower()
    return CreateToken(count=n, pt=None, types=(kind, "artifact", "token"))


# -- Energy ("get {E}{E}") -----------------------------------------------------
# Energy symbols aren't mana — they go into an energy pool. Represent as a
# labeled modification (we use Static+Modification via Sequence-compat, wrapped
# in the usual spell_effect path). The count is the number of {E} symbols.

@_eff(r"^(?:you )?get (\{e\}(?:\{e\})*)\.?$")
def _get_energy(m):
    count = m.group(1).lower().count("{e}")
    return Static(
        modification=Modification(
            kind="gain_energy",
            args=(count,),
        ),
    )


__all__ = ["EFFECT_RULES"]
