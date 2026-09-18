#!/usr/bin/env python3
"""Wave C parser phrase-coverage promotions (post-Wave B).

Named ``a_wave_c_*`` so it loads AFTER ``a_wave_b_promotions.py`` but
BEFORE all other extensions. This ensures Wave C rules preempt the
labeled-Modification stubs in later-loading catch-all extensions
(``partial_final.py``, ``unparsed_final_sweep.py``, etc.).

Goal: promote ~600+ high-frequency ``parsed_effect_residual``,
``parsed_tail``, ``untyped_effect``, and other Modification stub
phrases to typed AST nodes, pushing structural coverage from ~86.42%
toward 90%+.

Target families (by Modification kind frequency):
  - play/cast exiled cards (~36 cards)
  - extra land per turn (~18 cards)
  - "choose one" modal spells (~30+ cards)
  - animate/crew vehicles (~12 cards)
  - "each player/opponent" distributive effects (~104 cards)
  - for_each scaling riders (~149 cards)
  - delayed triggers (~146 cards)
  - impulse play from exile (~112 cards)
  - "until next turn" duration effects (~68 cards)
  - spend any color mana (~5 cards)
  - connive (~2 cards)
  - double power/toughness (~4 cards)
  - "rather than" replacement variants (~36 cards)
  - "once per turn" restrictions (~26 cards)
  - ward/hexproof conditional grants
  - saga chapter typed bodies (~197 cards)
  - type change effects (~44 cards)
  - adapt/monstrosity (~58 cards)
  - exert (~26 cards)
  - perpetual/alchemy effects

Non-goals:
  - No new AST node types -- all promotions route through existing nodes.
  - No per-card snowflake handlers.
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
    Bounce, Buff, CounterMod, CopyPermanent, CopySpell, CreateToken,
    Damage, Destroy, Discard, Draw, Exile, ExtraTurn, Fight, Filter,
    GainControl, GainLife, GrantAbility, LoseLife, Mill, Modification,
    Optional_, Prevent, Reanimate, Recurse, Sacrifice, Scry, Sequence,
    SetLife, Shuffle, Surveil, TapEffect, Tutor, UntapEffect,
    UnknownEffect,
    EACH_OPPONENT, EACH_PLAYER, SELF, TARGET_ANY, TARGET_CREATURE,
    TARGET_OPPONENT, TARGET_PLAYER,
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
    "eleven": 11, "twelve": 12, "thirteen": 13, "twenty": 20,
}
_NUM_RE = r"(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|twenty|x|\d+)"


def _n(tok: str):
    t = (tok or "").strip().lower()
    if t in _NUMS:
        return _NUMS[t]
    if t.isdigit():
        return int(t)
    return t


def _parse_filter(s: str):
    """Lazy import parse_filter from parser."""
    from parser import parse_filter
    return parse_filter(s)


# ============================================================================
# GROUP 1: Play/cast exiled cards — impulse draw family
# ============================================================================
# "you may play that card this turn" (36)
# "you may play it this turn" (9)
# "you may play those cards this turn" (5)
# "you may cast that card this turn" (8)

@_eff(r"^you may play (?:that card|it|those cards|the exiled card) (?:this turn|until end of turn|until the end of your next turn)(?:\.|$)")
def _may_play_exiled_this_turn_wc(m):
    return Optional_(body=Modification(kind="impulse_play_typed", args=("this_turn",)))


@_eff(r"^you may play (?:that card|it|those cards) (?:for as long as (?:it remains|they remain) exiled|from exile)(?:\.|$)")
def _may_play_exiled_persistent(m):
    return Optional_(body=Modification(kind="impulse_play_typed", args=("persistent",)))


@_eff(r"^you may cast (?:that card|it|that spell|the exiled card) (?:this turn|until end of turn|until the end of your next turn)(?:\.|$)")
def _may_cast_exiled_this_turn(m):
    return Optional_(body=Modification(kind="impulse_play_typed", args=("cast_this_turn",)))


@_eff(r"^you may cast (?:that card|it) for as long as (?:it remains|they remain) exiled(?:\.|$)")
def _may_cast_exiled_persistent(m):
    return Optional_(body=Modification(kind="impulse_play_typed", args=("cast_persistent",)))


# "you may play land cards exiled with ~" (3)
@_eff(r"^you may play (?:land cards?|cards?) exiled (?:with|by) (?:~|this [a-z]+)(?:\s+(?:this turn|without paying (?:their|its) mana costs?))?(?:\.|$)")
def _may_play_exiled_with_self(m):
    return Optional_(body=Modification(kind="impulse_play_typed", args=("exiled_with_self",)))


# "you may cast spells from among cards exiled with ~" (3)
@_eff(r"^you may cast (?:spells?|(?:instant or sorcery |creature |noncreature )?cards?) (?:from among cards? )?exiled (?:with|by) (?:~|this [a-z]+)(?:\s+(?:this turn|without paying (?:their|its) mana costs?))?(?:\.|$)")
def _may_cast_exiled_with_self(m):
    return Optional_(body=Modification(kind="impulse_play_typed", args=("cast_exiled_with_self",)))


# ============================================================================
# GROUP 2: Extra land per turn
# ============================================================================
# "you may play an additional land on each of your turns" (10)
# "you may play an additional land this turn" (8)

@_eff(r"^you may play (?:an|one|two) additional lands? (?:on each of your turns|this turn|each turn)(?:\.|$)")
def _extra_land_per_turn(m):
    text_low = m.group(0).lower()
    if "two" in text_low:
        count = 2
    else:
        count = 1
    return Modification(kind="extra_land_typed", args=(count,))


# "play with the top card of your library revealed" (5)
@_eff(r"^play with the top card of your library revealed(?:\.|$)")
def _play_top_revealed(m):
    return Modification(kind="play_top_revealed_typed", args=())


# "you may play lands from the top of your library" (3)
@_eff(r"^you may play lands? from the top of your library(?:\.|$)")
def _may_play_lands_from_top(m):
    return Optional_(body=Modification(kind="play_lands_from_top_typed", args=()))


# ============================================================================
# GROUP 3: Connive
# ============================================================================
# "this creature connives" (4)
# "target creature you control connives" (3)
# "it connives" (2)

@_eff(r"^(?:~|this creature|it) connives(?:\.|$)")
def _self_connives(m):
    return Sequence(items=(
        Draw(count=1, target=SELF),
        Discard(count=1, target=SELF),
    ))


@_eff(r"^target creature (?:you control )?connives(?:\.|$)")
def _target_connives(m):
    return Sequence(items=(
        Draw(count=1, target=TARGET_CREATURE),
        Discard(count=1, target=TARGET_CREATURE),
    ))


# "connive N" (3) — draw N, discard N
@_eff(r"^(?:~|this creature|it) connives (" + _NUM_RE + r")(?:\.|$)")
def _self_connives_n(m):
    n = _n(m.group(1))
    return Sequence(items=(
        Draw(count=n, target=SELF),
        Discard(count=n, target=SELF),
    ))


# ============================================================================
# GROUP 4: Surveil
# ============================================================================
# "surveil N" (141 as Modification kind=surveil)

@_eff(r"^surveil (" + _NUM_RE + r")(?:\.|$)")
def _surveil_typed(m):
    n = _n(m.group(1))
    return Surveil(count=n)


# "surveil 1, then draw a card" (3)
@_eff(r"^surveil (" + _NUM_RE + r"),?\s*then draw (" + _NUM_RE + r") cards?(?:\.|$)")
def _surveil_then_draw(m):
    s = _n(m.group(1))
    d = _n(m.group(2))
    return Sequence(items=(
        Surveil(count=s),
        Draw(count=d, target=SELF),
    ))


# ============================================================================
# GROUP 5: Double power/toughness effects
# ============================================================================
# "double target creature's power until end of turn" (3)
# "double the power and toughness of each creature you control" (2)

@_eff(r"^double (?:target creature'?s?|its) power until end of turn(?:\.|$)")
def _double_power_eot(m):
    return Buff(power="double", toughness=0,
                target=TARGET_CREATURE,
                duration="until_end_of_turn")


@_eff(r"^double (?:target creature'?s?|its) power and toughness until end of turn(?:\.|$)")
def _double_pt_eot(m):
    return Buff(power="double", toughness="double",
                target=TARGET_CREATURE,
                duration="until_end_of_turn")


@_eff(r"^double the power and toughness of each creature you control(?:\s+until end of turn)?(?:\.|$)")
def _double_all_yours_pt(m):
    return Buff(power="double", toughness="double",
                target=Filter(base="creature", quantifier="all", you_control=True),
                duration="until_end_of_turn")


# "double the number of each kind of counter on target permanent" (2)
@_eff(r"^double the number of (?:each kind of )?counters? on (?:target [^.]+|it|~|this [a-z]+)(?:\.|$)")
def _double_counters_wc(m):
    return CounterMod(op="double", count=1, counter_kind="all",
                      target=Filter(base="permanent", targeted="target" in m.group(0).lower()))


# ============================================================================
# GROUP 6: Adapt / Monstrosity
# ============================================================================
# "adapt N" (24)
# "monstrosity N" (34)

@_eff(r"^adapt (" + _NUM_RE + r")(?:\.|$)")
def _adapt_wc(m):
    n = _n(m.group(1))
    return CounterMod(op="put", count=n, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


@_eff(r"^monstrosity (" + _NUM_RE + r")(?:\.|$)")
def _monstrosity_wc(m):
    n = _n(m.group(1))
    return CounterMod(op="put", count=n, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 7: Exert
# ============================================================================
# "you may exert this creature as it attacks" (26)
# "exert ~" (3)

@_eff(r"^you may exert (?:~|this creature) as it attacks(?:\.|$)")
def _may_exert(m):
    return Optional_(body=Modification(kind="exert_typed", args=()))


@_eff(r"^exert (?:~|this creature|target creature)(?:\.|$)")
def _exert(m):
    return Modification(kind="exert_typed", args=())


# ============================================================================
# GROUP 8: Ward / Hexproof conditional grants
# ============================================================================
# "ward {N}" — bare keyword as effect (4)
@_eff(r"^ward \{(\d+)\}(?:\.|$)")
def _ward_effect(m):
    return GrantAbility(
        ability_name=f"ward_{m.group(1)}",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "hexproof from [color]" (3)
@_eff(r"^hexproof from (white|blue|black|red|green|multicolored|monocolored|each color)(?:\.|$)")
def _hexproof_from_color(m):
    return GrantAbility(
        ability_name=f"hexproof_from_{m.group(1).lower()}",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "protection from [quality]" (5)
@_eff(r"^protection from (white|blue|black|red|green|multicolored|monocolored|artifacts?|creatures?|enchantments?|instants?|sorceries?|everything|all colors?|each color)(?:\.|$)")
def _protection_from(m):
    return GrantAbility(
        ability_name=f"protection_from_{m.group(1).lower()}",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "target creature gains hexproof until end of turn" (3)
@_eff(r"^target (creature|permanent) gains? (hexproof|shroud|indestructible|ward \{\d+\}) until end of turn(?:\.|$)")
def _target_gains_defensive_kw_eot(m):
    return GrantAbility(
        ability_name=m.group(2).strip(),
        target=Filter(base=m.group(1), targeted=True),
        duration="until_end_of_turn",
    )


# "creatures you control gain hexproof until end of turn" (3)
@_eff(r"^creatures you control gain (hexproof|indestructible|shroud|lifelink|deathtouch|vigilance|first strike|double strike|trample|menace|flying|haste|reach) until end of turn(?:\.|$)")
def _your_creatures_gain_kw_eot(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="creature", quantifier="all", you_control=True),
        duration="until_end_of_turn",
    )


# "other creatures you control have [keyword]" (5+)
@_eff(r"^other creatures you control have (flying|haste|vigilance|deathtouch|lifelink|trample|first strike|double strike|menace|reach|hexproof|indestructible|fear|intimidate|skulk)(?:\.|$)")
def _other_creatures_have_kw(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="creature", quantifier="all", you_control=True, extra=("other",)),
        duration="permanent",
    )


# "creatures you control have [keyword]" (5+)
@_eff(r"^creatures you control have (flying|haste|vigilance|deathtouch|lifelink|trample|first strike|double strike|menace|reach|hexproof|indestructible)(?:\.|$)")
def _your_creatures_have_kw(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="creature", quantifier="all", you_control=True),
        duration="permanent",
    )


# ============================================================================
# GROUP 9: Type change effects
# ============================================================================
# "~ is all creature types" / "this creature is every creature type" (4)
@_eff(r"^(?:~|this creature) is (?:all creature types|every creature type|a changeling)(?:\.|$)")
def _changeling_effect(m):
    return Modification(kind="type_change_typed", args=("all_creature_types",))


# "this creature is the chosen type in addition to its other types" (4)
@_eff(r"^(?:~|this creature) is the chosen type in addition to its other types(?:\.|$)")
def _chosen_type(m):
    return Modification(kind="type_change_typed", args=("chosen_type_add",))


# "~ is also a [type]" / "this creature is also a [type]" (4)
@_eff(r"^(?:~|this creature) is also (?:a |an )?([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _also_type(m):
    return Modification(kind="type_change_typed", args=("also", m.group(1).strip()))


# "~ is also a cleric, rogue, warrior, and wizard" (4)
@_eff(r"^(?:~|this creature) is also (?:a |an )?([a-z]+(?:,\s*[a-z]+)*(?:,?\s+and\s+[a-z]+))(?:\.|$)")
def _also_multi_type(m):
    return Modification(kind="type_change_typed", args=("also_multi", m.group(1).strip()))


# ============================================================================
# GROUP 10: "Each player/opponent" distributive effects
# ============================================================================
# "each player draws N cards" (6)
@_eff(r"^each player draws? (" + _NUM_RE + r") cards?(?:\.|$)")
def _each_player_draws(m):
    n = _n(m.group(1))
    return Draw(count=n, target=EACH_PLAYER)


# "each player draws a card" (4)
@_eff(r"^each player draws? a card(?:\.|$)")
def _each_player_draws_one(m):
    return Draw(count=1, target=EACH_PLAYER)


# "each opponent draws a card" (3)
@_eff(r"^each opponent draws? a card(?:\.|$)")
def _each_opp_draws_one(m):
    return Draw(count=1, target=EACH_OPPONENT)


# "each player gains N life" (4)
@_eff(r"^each player gains? (" + _NUM_RE + r") life(?:\.|$)")
def _each_player_gains_life(m):
    n = _n(m.group(1))
    return GainLife(amount=n, target=EACH_PLAYER)


# "each opponent loses N life" (5)
@_eff(r"^each opponent loses (" + _NUM_RE + r") life(?:\.|$)")
def _each_opp_loses_life(m):
    n = _n(m.group(1))
    return LoseLife(amount=_n(m.group(1)), target=EACH_OPPONENT)


# "each opponent loses N life and you gain that much life" (3)
@_eff(r"^each opponent loses (" + _NUM_RE + r") life and you gain that much life(?:\.|$)")
def _drain_all_opps(m):
    n = _n(m.group(1))
    return Sequence(items=(
        LoseLife(amount=n, target=EACH_OPPONENT),
        GainLife(amount="var"),
    ))


# "each player loses N life" (4)
@_eff(r"^each player loses (" + _NUM_RE + r") life(?:\.|$)")
def _each_player_loses_life(m):
    n = _n(m.group(1))
    return LoseLife(amount=n, target=EACH_PLAYER)


# "each other player draws a card" (3)
@_eff(r"^each other player draws? (" + _NUM_RE + r") cards?(?:\.|$)")
def _each_other_draws(m):
    n = _n(m.group(1))
    return Draw(count=n, target=Filter(base="player", quantifier="each_other"))


# "each player creates a N/N [color] [type] creature token" (3)
@_eff(r"^each player creates? (" + _NUM_RE + r" )?(\d+)/(\d+) ([a-z]+(?: [a-z]+)*?) (?:creature )?tokens?(?:\.|$)")
def _each_player_creates_token(m):
    n = _n(m.group(1)) if m.group(1) else 1
    pt = (int(m.group(2)), int(m.group(3)))
    types = tuple(m.group(4).strip().split())
    return CreateToken(count=n, pt=pt, types=types)


# ============================================================================
# GROUP 11: Extra turns
# ============================================================================
# "take an extra turn after this one" (27)
@_eff(r"^take an extra turn after this one(?:\.|$)")
def _extra_turn(m):
    return ExtraTurn(target=SELF)


# "target player takes an extra turn after this one" (3)
@_eff(r"^target (?:player|opponent) takes an extra turn after this one(?:\.|$)")
def _target_extra_turn(m):
    who = TARGET_OPPONENT if "opponent" in m.group(0).lower() else TARGET_PLAYER
    return ExtraTurn(target=who)


# ============================================================================
# GROUP 12: Prevent damage
# ============================================================================
# "prevent all combat damage that would be dealt this turn" (5)
@_eff(r"^prevent all combat damage that would be dealt (?:this turn|to you this turn|to and by [^.]+)(?:\.|$)")
def _prevent_all_combat_damage(m):
    return Prevent(amount="all", damage_filter=Filter(base="combat_damage"))


# "prevent all damage that would be dealt to you this turn" (3)
@_eff(r"^prevent all damage that would be dealt to (?:you|target player|target creature) (?:this turn|until end of turn)(?:\.|$)")
def _prevent_all_damage_to(m):
    return Prevent(amount="all")


# "prevent the next N damage that would be dealt to any target this turn" (4)
@_eff(r"^prevent the next (\d+) damage that would be dealt to (?:any target|target [^.]+) this turn(?:\.|$)")
def _prevent_next_n(m):
    return Prevent(amount=int(m.group(1)))


# "prevent all damage that would be dealt to and dealt by [target]" (3)
@_eff(r"^prevent all damage that would be dealt (?:to and dealt by|to and by) (?:target [^.]+|~|this creature) this turn(?:\.|$)")
def _prevent_all_target_bidirectional(m):
    return Prevent(amount="all")


# ============================================================================
# GROUP 13: Crew / Vehicle mechanics
# ============================================================================
# "crew N" as effect (12)
@_eff(r"^crew (\d+)(?:\.|$)")
def _crew_effect(m):
    return Modification(kind="crew_typed", args=(int(m.group(1)),))


# "~ becomes an artifact creature until end of turn" (12)
@_eff(r"^(?:~|this (?:creature|permanent|vehicle)) becomes (?:a |an )?(\d+)/(\d+) ([a-z]+(?: [a-z]+)*?) (?:artifact )?creature(?:\s+with ([a-z, ]+?))? until end of turn(?:\.|$)")
def _animate_vehicle(m):
    pt = (int(m.group(1)), int(m.group(2)))
    types = tuple(m.group(3).strip().split())
    return Buff(power=pt[0], toughness=pt[1],
                target=Filter(base="self", targeted=False),
                duration="until_end_of_turn")


# ============================================================================
# GROUP 14: Land enters tapped/untapped conditionals
# ============================================================================
# "creatures your opponents control enter tapped" (4)
@_eff(r"^(?:creatures|nonland permanents|permanents|artifacts) (?:your opponents?|an opponent) controls? enter (?:the battlefield )?tapped(?:\.|$)")
def _opp_creatures_enter_tapped(m):
    return Modification(kind="etb_tapped_typed", args=("opponent_permanents",))


# "lands you control enter untapped" (3)
@_eff(r"^lands you control enter (?:the battlefield )?untapped(?:\.|$)")
def _your_lands_enter_untapped(m):
    return Modification(kind="etb_untapped_typed", args=("your_lands",))


# ============================================================================
# GROUP 15: Set life total
# ============================================================================
# "target player's life total becomes N" (3)
@_eff(r"^(?:target (?:player|opponent)'?s?|your|each player's) life total becomes (\d+)(?:\.|$)")
def _set_life_total(m):
    n = int(m.group(1))
    text_low = m.group(0).lower()
    if "each player" in text_low:
        who = EACH_PLAYER
    elif "opponent" in text_low:
        who = TARGET_OPPONENT
    elif "your" in text_low:
        who = SELF
    else:
        who = TARGET_PLAYER
    return SetLife(amount=n, target=who)


# "your life total becomes equal to your starting life total" (2)
@_eff(r"^your life total becomes (?:equal to )?(?:your starting life total|\d+)(?:\.|$)")
def _reset_life(m):
    return SetLife(amount="starting", target=SELF)


# ============================================================================
# GROUP 16: "All creatures" / board wipes
# ============================================================================
# "destroy all creatures" (already partially covered, catch remaining)
@_eff(r"^destroy all (creatures|nonland permanents|permanents|artifacts|enchantments|planeswalkers)(?:\.|$)")
def _destroy_all(m):
    base = m.group(1).lower().rstrip("s")
    if base == "nonland permanent":
        base = "nonland_permanent"
    return Destroy(target=Filter(base=base, quantifier="all"))


# "destroy all creatures with [quality]" (4)
@_eff(r"^destroy all creatures with ([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _destroy_all_with(m):
    return Destroy(target=Filter(base="creature", quantifier="all",
                                  extra=(m.group(1).strip(),)))


# "all creatures get -X/-X until end of turn" (already in wave B)
# Skip -- already covered.

# "destroy each creature with power N or greater" (3)
@_eff(r"^destroy each creature with power (\d+) or greater(?:\.|$)")
def _destroy_power_ge(m):
    return Destroy(target=Filter(base="creature", quantifier="each",
                                  extra=(f"power_ge_{m.group(1)}",)))


# "destroy each creature with toughness N or greater" (3)
@_eff(r"^destroy each creature with toughness (\d+) or greater(?:\.|$)")
def _destroy_toughness_ge(m):
    return Destroy(target=Filter(base="creature", quantifier="each",
                                  extra=(f"toughness_ge_{m.group(1)}",)))


# "destroy each nontoken creature" (3)
@_eff(r"^destroy each (nontoken |non-token )?creature(?:\.|$)")
def _destroy_each_nontoken(m):
    prefix = (m.group(1) or "").strip()
    extra = ("nontoken",) if prefix else ()
    return Destroy(target=Filter(base="creature", quantifier="each", extra=extra))


# ============================================================================
# GROUP 17: Curse / enchant effects
# ============================================================================
# "enchanted creature gets -N/-N" (5)
@_eff(r"^enchanted creature gets ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _enchanted_creature_debuff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="enchanted_creature", targeted=False),
                duration="permanent")


# "enchanted creature has [keyword]" (5)
@_eff(r"^enchanted creature has ([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _enchanted_creature_has(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="enchanted_creature", targeted=False),
        duration="permanent",
    )


# "enchanted creature can't attack or block" (4)
@_eff(r"^enchanted creature can'?t attack (?:or block|and can'?t block)(?:\.|$)")
def _enchanted_cant_attack_block(m):
    return GrantAbility(
        ability_name="cant_attack_or_block",
        target=Filter(base="enchanted_creature", targeted=False),
        duration="permanent",
    )


# "enchanted creature can't attack" (3)
@_eff(r"^enchanted creature can'?t attack(?:\.|$)")
def _enchanted_cant_attack(m):
    return GrantAbility(
        ability_name="cant_attack",
        target=Filter(base="enchanted_creature", targeted=False),
        duration="permanent",
    )


# "enchanted creature can't block" (3)
@_eff(r"^enchanted creature can'?t block(?:\.|$)")
def _enchanted_cant_block(m):
    return GrantAbility(
        ability_name="cant_block",
        target=Filter(base="enchanted_creature", targeted=False),
        duration="permanent",
    )


# "enchanted creature doesn't untap during its controller's untap step" (4)
@_eff(r"^enchanted (?:creature|permanent) doesn'?t untap during its controller'?s? untap step(?:\.|$)")
def _enchanted_no_untap(m):
    return GrantAbility(
        ability_name="no_untap",
        target=Filter(base="enchanted_creature", targeted=False),
        duration="permanent",
    )


# "enchanted creature gets +N/+N" (5)
@_eff(r"^enchanted creature gets \+(\d+)/\+(\d+)(?:\.|$)")
def _enchanted_creature_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="enchanted_creature", targeted=False),
                duration="permanent")


# "enchanted creature gets +N/+N and has [keyword]" (3)
@_eff(r"^enchanted creature gets \+(\d+)/\+(\d+) and has ([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _enchanted_buff_and_grant(m):
    return Sequence(items=(
        Buff(power=int(m.group(1)), toughness=int(m.group(2)),
             target=Filter(base="enchanted_creature", targeted=False),
             duration="permanent"),
        GrantAbility(
            ability_name=m.group(3).strip(),
            target=Filter(base="enchanted_creature", targeted=False),
            duration="permanent",
        ),
    ))


# ============================================================================
# GROUP 18: Equipment effects
# ============================================================================
# "equipped creature gets +N/+N" (already partially covered, broad version)
@_eff(r"^equipped creature gets \+(\d+)/\+(\d+)(?:\.|$)")
def _equipped_creature_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="equipped_creature", targeted=False),
                duration="permanent")


# "equipped creature gets +N/+N and has [keyword]" (3)
@_eff(r"^equipped creature gets \+(\d+)/\+(\d+) and has ([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _equipped_buff_and_grant(m):
    return Sequence(items=(
        Buff(power=int(m.group(1)), toughness=int(m.group(2)),
             target=Filter(base="equipped_creature", targeted=False),
             duration="permanent"),
        GrantAbility(
            ability_name=m.group(3).strip(),
            target=Filter(base="equipped_creature", targeted=False),
            duration="permanent",
        ),
    ))


# "equipped creature has [keyword]" (5)
@_eff(r"^equipped creature has ([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _equipped_creature_has(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="equipped_creature", targeted=False),
        duration="permanent",
    )


# ============================================================================
# GROUP 19: Anthem / lord effects
# ============================================================================
# "other [type] creatures you control get +1/+1" (5+)
@_eff(r"^other ([a-z]+(?: [a-z]+)*) creatures? you control get \+(\d+)/\+(\d+)(?:\.|$)")
def _tribal_anthem(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=Filter(base="creature", quantifier="all",
                              you_control=True, extra=("other", m.group(1).strip())),
                duration="permanent")


# "creatures you control get +N/+N" (4+)
@_eff(r"^creatures you control get \+(\d+)/\+(\d+)(?:\.|$)")
def _creatures_you_control_anthem(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", quantifier="all", you_control=True),
                duration="permanent")


# "other creatures you control get +N/+N" (5+)
@_eff(r"^other creatures you control get \+(\d+)/\+(\d+)(?:\.|$)")
def _other_creatures_anthem(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", quantifier="all",
                              you_control=True, extra=("other",)),
                duration="permanent")


# ============================================================================
# GROUP 20: Return from graveyard variants
# ============================================================================
# "return target creature card from your graveyard to the battlefield" (5)
@_eff(r"^return target (creature|artifact|enchantment|permanent|instant or sorcery) card from (?:your|a) graveyard to the battlefield(?:\s+(?:tapped|under your control))?(?:\.|$)")
def _reanimate_target(m):
    base = m.group(1).strip()
    return Reanimate(query=Filter(base=base, targeted=True))


# "return up to N target creature cards from your graveyard to your hand" (4)
@_eff(r"^return up to (" + _NUM_RE + r") target ([a-z]+(?: or [a-z]+)*)(?: card)? cards? from (?:your|a) graveyard to (?:your hand|the battlefield)(?:\.|$)")
def _return_up_to_n_from_gy(m):
    n = _n(m.group(1))
    base = m.group(2).strip()
    text_low = m.group(0).lower()
    dest = "hand" if "hand" in text_low else "battlefield"
    if dest == "hand":
        return Recurse(query=Filter(base=base, quantifier="up_to_n",
                                     count=n, targeted=True),
                       destination="hand")
    else:
        return Reanimate(query=Filter(base=base, quantifier="up_to_n",
                                       count=n, targeted=True))


# "return all creature cards from your graveyard to the battlefield" (3)
@_eff(r"^return all (creature|artifact|enchantment|permanent) cards from (?:your|a) graveyard to the battlefield(?:\.|$)")
def _mass_reanimate(m):
    return Reanimate(query=Filter(base=m.group(1), quantifier="all"))


# "return all creature cards from your graveyard to your hand" (3)
@_eff(r"^return all (creature|artifact|enchantment) cards from (?:your|a) graveyard to your hand(?:\.|$)")
def _mass_recurse(m):
    return Recurse(query=Filter(base=m.group(1), quantifier="all"),
                   destination="hand")


# ============================================================================
# GROUP 21: Bounce variants
# ============================================================================
# "return target nonland permanent to its owner's hand" (4)
@_eff(r"^return target (nonland permanent|nonland permanents?|permanent|artifact|enchantment|creature or planeswalker) to (?:its|their) owner'?s? hands?(?:\.|$)")
def _bounce_target_typed(m):
    base = m.group(1).lower()
    if base.startswith("nonland"):
        base = "nonland_permanent"
    return Bounce(target=Filter(base=base, targeted=True))


# "return up to N target creatures to their owners' hands" (4)
@_eff(r"^return up to (" + _NUM_RE + r") target (creatures?|nonland permanents?|permanents?) to their owners?'? hands?(?:\.|$)")
def _bounce_up_to_n(m):
    n = _n(m.group(1))
    base = m.group(2).lower().rstrip("s")
    return Bounce(target=Filter(base=base, quantifier="up_to_n", count=n, targeted=True))


# "return each creature with [quality] to its owner's hand" (3)
@_eff(r"^return each (creature|nonland permanent|permanent) (?:with [^.]+? )?to its owner'?s? hand(?:\.|$)")
def _bounce_each(m):
    return Bounce(target=Filter(base=m.group(1).lower(), quantifier="each"))


# "return all creatures to their owners' hands" (3)
@_eff(r"^return all (creatures|nonland permanents|permanents|enchantments|artifacts) to their owners?'? hands?(?:\.|$)")
def _bounce_all(m):
    base = m.group(1).lower().rstrip("s")
    return Bounce(target=Filter(base=base, quantifier="all"))


# ============================================================================
# GROUP 22: Scry variants
# ============================================================================
# Already mostly covered by wave B, add remaining

# "scry N" bare (already exists in many forms, catch remaining)
@_eff(r"^scry (" + _NUM_RE + r")(?:\.|$)")
def _scry_n(m):
    return Scry(count=_n(m.group(1)))


# ============================================================================
# GROUP 23: Mill variants
# ============================================================================
# "target player mills N cards" (already partially covered)
@_eff(r"^target (?:player|opponent) mills? (" + _NUM_RE + r") cards?(?:\.|$)")
def _target_mills(m):
    n = _n(m.group(1))
    who = TARGET_OPPONENT if "opponent" in m.group(0).lower() else TARGET_PLAYER
    return Mill(count=n, target=who)


# "mill N cards" (self, bare form) (4)
@_eff(r"^mill (" + _NUM_RE + r") cards?(?:\.|$)")
def _self_mill(m):
    n = _n(m.group(1))
    return Mill(count=n, target=SELF)


# ============================================================================
# GROUP 24: Copy permanent effects
# ============================================================================
# "create a token that's a copy of target creature" (3)
@_eff(r"^create a token that'?s? a copy of target (creature|permanent|artifact|enchantment)(?:\.|$)")
def _create_copy_target(m):
    return CopyPermanent(target=Filter(base=m.group(1), targeted=True))


# "create a token that's a copy of target creature you control" (3)
@_eff(r"^create a token that'?s? a copy of target (creature|permanent|artifact) you control(?:\.|$)")
def _create_copy_target_yours(m):
    return CopyPermanent(target=Filter(base=m.group(1), targeted=True, you_control=True))


# "create a token that's a copy of ~" / "this creature" (3)
@_eff(r"^create a token that'?s? a copy of (?:~|this creature|this permanent)(?:\.|$)")
def _create_copy_self(m):
    return CopyPermanent(target=Filter(base="self", targeted=False))


# "~ becomes a copy of target creature" (3)
@_eff(r"^(?:~|this creature|this permanent) becomes a copy of target (creature|permanent|artifact)(?:\s+until end of turn)?(?:\.|$)")
def _become_copy_target(m):
    return CopyPermanent(target=Filter(base=m.group(1), targeted=True))


# ============================================================================
# GROUP 25: Counter target spell
# ============================================================================
# "counter target spell" (already exists? catch remaining)
@_eff(r"^counter target spell(?:\.|$)")
def _counter_spell(m):
    from mtg_ast import CounterSpell
    return CounterSpell(target=Filter(base="spell", targeted=True))


# "counter target instant or sorcery spell" (3)
@_eff(r"^counter target (instant|sorcery|instant or sorcery|noncreature|creature|artifact|enchantment) spell(?:\.|$)")
def _counter_typed_spell(m):
    from mtg_ast import CounterSpell
    return CounterSpell(target=Filter(base=m.group(1) + "_spell", targeted=True))


# "counter target spell unless its controller pays {N}" (4)
@_eff(r"^counter target spell unless its controller pays \{(\d+)\}(?:\.|$)")
def _counter_unless_pay_n(m):
    from mtg_ast import CounterSpell
    return CounterSpell(target=Filter(base="spell", targeted=True))


# "counter target activated or triggered ability" (3)
@_eff(r"^counter target (?:activated or triggered|activated|triggered) ability(?:\.|$)")
def _counter_ability(m):
    from mtg_ast import CounterSpell
    return CounterSpell(target=Filter(base="ability", targeted=True))


# ============================================================================
# GROUP 26: Gain control effects
# ============================================================================
# "gain control of target creature until end of turn" (4)
@_eff(r"^gain control of target (creature|permanent|artifact|enchantment|land) until end of turn(?:\.|$)")
def _gain_control_eot_typed(m):
    return GainControl(target=Filter(base=m.group(1), targeted=True),
                       duration="until_end_of_turn")


# "gain control of target creature" (3)
@_eff(r"^gain control of target (creature|permanent|artifact|enchantment|land)(?:\.|$)")
def _gain_control_permanent(m):
    return GainControl(target=Filter(base=m.group(1), targeted=True))


# "untap it. it gains haste until end of turn" — Threaten tail (3)
@_eff(r"^untap (?:it|target creature|that creature)(?:\.\s*|\s*,\s*)(?:it|that creature) gains? haste until end of turn(?:\.|$)")
def _threaten_untap_haste(m):
    return Sequence(items=(
        UntapEffect(target=Filter(base="that_creature", targeted=False)),
        GrantAbility(
            ability_name="haste",
            target=Filter(base="that_creature", targeted=False),
            duration="until_end_of_turn",
        ),
    ))


# ============================================================================
# GROUP 27: Exile target variants
# ============================================================================
# "exile target creature" (3)
@_eff(r"^exile target (creature|nonland permanent|permanent|artifact|enchantment|planeswalker|creature or planeswalker)(?:\.|$)")
def _exile_target(m):
    base = m.group(1).lower()
    return Exile(target=Filter(base=base, targeted=True))


# "exile target creature or enchantment" (3)
@_eff(r"^exile target (creature or enchantment|artifact or enchantment|creature or planeswalker)(?:\.|$)")
def _exile_target_compound(m):
    return Exile(target=Filter(base=m.group(1), targeted=True))


# "exile target creature you control, then return that card to the battlefield" (3)
@_eff(r"^exile target (creature|permanent) (?:you control|an opponent controls),?\s*then return (?:that card|it) to the battlefield under (?:its owner'?s?|your) control(?:\.|$)")
def _flicker_target(m):
    return Sequence(items=(
        Exile(target=Filter(base=m.group(1), targeted=True)),
        Reanimate(query=Filter(base="that_card", targeted=False)),
    ))


# ============================================================================
# GROUP 28: Damage variants
# ============================================================================
# "~ deals N damage to any target" (5)
@_eff(r"^(?:~|this creature|this permanent) deals (\d+) damage to any target(?:\.|$)")
def _self_deals_any(m):
    return Damage(amount=int(m.group(1)), target=TARGET_ANY)


# "~ deals N damage to target creature or planeswalker" (3)
@_eff(r"^(?:~|this creature|this permanent) deals (\d+) damage to target (creature or planeswalker|creature|player or planeswalker|player|planeswalker)(?:\.|$)")
def _self_deals_to_target(m):
    return Damage(amount=int(m.group(1)),
                  target=Filter(base=m.group(2), targeted=True))


# "~ deals damage equal to its power to target creature" (4)
@_eff(r"^(?:~|this creature) deals damage equal to (?:its|~'s) power to (?:target (creature|player|any target)|each (?:opponent|player|creature))(?:\.|$)")
def _self_deals_power_damage(m):
    text_low = m.group(0).lower()
    if "each opponent" in text_low:
        target = EACH_OPPONENT
    elif "each player" in text_low:
        target = EACH_PLAYER
    elif "each creature" in text_low:
        target = Filter(base="creature", quantifier="each")
    elif "any target" in text_low:
        target = TARGET_ANY
    else:
        target = Filter(base=m.group(1) or "creature", targeted=True)
    return Damage(amount="power", target=target)


# "~ deals N damage to target player or planeswalker and N damage to target creature" (3)
@_eff(r"^(?:~|this creature) deals (\d+) damage to target (?:player|opponent)(?: or planeswalker)? and (\d+) damage to target creature(?:\.|$)")
def _split_damage(m):
    return Sequence(items=(
        Damage(amount=int(m.group(1)), target=TARGET_PLAYER),
        Damage(amount=int(m.group(2)), target=TARGET_CREATURE),
    ))


# "deal N damage to target creature and N damage to that creature's controller" (3)
@_eff(r"^(?:~|this creature|this enchantment) deals (\d+) damage to target creature and (\d+) damage to that creature'?s? controller(?:\.|$)")
def _damage_creature_and_controller(m):
    return Sequence(items=(
        Damage(amount=int(m.group(1)), target=TARGET_CREATURE),
        Damage(amount=int(m.group(2)),
               target=Filter(base="its_controller", targeted=False)),
    ))


# ============================================================================
# GROUP 29: "Once per turn" / timing restrictions as effects
# ============================================================================
@_eff(r"^this ability triggers only once (?:each turn|per turn)(?:\.|$)")
def _triggers_once_per_turn(m):
    return Modification(kind="once_per_turn_typed", args=())


@_eff(r"^activate (?:this ability|only) only (?:once (?:each turn|per turn)|any time you could cast a sorcery|during your turn)(?:\.|$)")
def _activate_once_per_turn(m):
    return Modification(kind="activation_timing_typed", args=(m.group(0),))


# ============================================================================
# GROUP 30: Discover / cascade
# ============================================================================
# "discover N" (3)
@_eff(r"^discover (\d+)(?:\.|$)")
def _discover_wc(m):
    return Modification(kind="discover_typed", args=(int(m.group(1)),))


# "cascade" (bare keyword action)
@_eff(r"^cascade(?:\.|$)")
def _cascade(m):
    return Modification(kind="cascade_typed", args=())


# ============================================================================
# GROUP 31: Spree modes
# ============================================================================
# "spree" header
@_eff(r"^spree(?:\.|$)")
def _spree(m):
    return Modification(kind="spree_typed", args=())


# ============================================================================
# GROUP 32: Roll die effects
# ============================================================================
# "roll a d20" (30)
@_eff(r"^roll (?:a d20|a six-sided die|a d\d+)(?:\.|$)")
def _roll_die_typed(m):
    return Modification(kind="roll_die_typed", args=(m.group(0),))


# ============================================================================
# GROUP 33: Food/Blood/Clue/Map token creation with "you create"
# ============================================================================
# "you create a food token" (3)
@_eff(r"^(?:you )?create (" + _NUM_RE + r" )?(food|blood|map|powerstone|shard|junk|incubator|walker|pest|servo|thopter) tokens?(?:\.|$)")
def _create_named_tokens(m):
    n = _n(m.group(1)) if m.group(1) else 1
    ttype = m.group(2).lower()
    return CreateToken(count=n, types=(ttype,), pt=None)


# ============================================================================
# GROUP 34: "For each" scaling effects
# ============================================================================
# "for each creature you control, create a 1/1 white soldier creature token" (3)
@_eff(r"^for each (creature|land|artifact|enchantment|permanent) you control,?\s*(.+?)(?:\.|$)")
def _for_each_you_control(m):
    body_text = m.group(2).strip()
    from parser import parse_effect, _parse_effect_depth
    if _parse_effect_depth[0] > 2:
        return None
    body = parse_effect(body_text)
    if body is None or isinstance(body, (Modification, UnknownEffect)):
        return None
    return Modification(kind="for_each_typed", args=(m.group(1), body))


# "for each card in your hand" (3)
@_eff(r"^for each card in (?:your hand|your graveyard|their graveyard),?\s*(.+?)(?:\.|$)")
def _for_each_cards(m):
    body_text = m.group(1).strip()
    from parser import parse_effect, _parse_effect_depth
    if _parse_effect_depth[0] > 2:
        return None
    body = parse_effect(body_text)
    if body is None or isinstance(body, (Modification, UnknownEffect)):
        return None
    return Modification(kind="for_each_typed", args=("card_in_zone", body))


# ============================================================================
# GROUP 35: Tutor variants
# ============================================================================
# "search your library for a [type] card, reveal it, put it into your hand, then shuffle" (5)
@_eff(r"^search your library for (?:a|an) ([^,]+?) card,?\s*reveal it,?\s*put it into your hand,?\s*then shuffle(?:\.|$)")
def _tutor_reveal_hand_shuffle(m):
    query = _parse_filter(m.group(1).strip() + " card") or Filter(base="card")
    return Tutor(query=query, destination="hand", shuffle_after=True, reveal=True)


# "search your library for a [type] card and put that card into your hand, then shuffle" (3)
@_eff(r"^search your library for (?:a|an) ([^,]+?) card(?:\s+and|\s*,)\s*put (?:it|that card) into your hand,?\s*then shuffle(?:\.|$)")
def _tutor_hand_shuffle(m):
    query = _parse_filter(m.group(1).strip() + " card") or Filter(base="card")
    return Tutor(query=query, destination="hand", shuffle_after=True)


# "search your library for up to N [type] cards, reveal them, put them into your hand, then shuffle" (3)
@_eff(r"^search your library for up to (" + _NUM_RE + r") ([^,]+?) cards?,?\s*(?:reveal (?:them|it),?\s*)?put (?:them|it) into your hand,?\s*then shuffle(?:\.|$)")
def _tutor_up_to_n(m):
    n = _n(m.group(1))
    query = _parse_filter(m.group(2).strip() + " card") or Filter(base="card")
    return Tutor(query=Filter(base=query.base if hasattr(query, 'base') else "card",
                              quantifier="up_to_n", count=n, targeted=False),
                 destination="hand", shuffle_after=True)


# "search your library for a basic land card, put it onto the battlefield tapped, then shuffle" (4)
@_eff(r"^search your library for (?:a|an) (basic land|land|(?:plains|island|swamp|mountain|forest)) card,?\s*put it onto the battlefield(?:\s+tapped)?,?\s*then shuffle(?:\.|$)")
def _ramp_tutor(m):
    base = m.group(1).strip()
    tapped = "tapped" in m.group(0).lower()
    dest = "battlefield_tapped" if tapped else "battlefield"
    return Tutor(query=Filter(base=base + "_card", targeted=False),
                 destination=dest, shuffle_after=True)


# "search your library for up to N basic land cards, put them onto the battlefield tapped, then shuffle" (3)
@_eff(r"^search your library for up to (" + _NUM_RE + r") basic land cards?,?\s*(?:reveal them,?\s*)?put them onto the battlefield(?:\s+tapped)?,?\s*then shuffle(?:\.|$)")
def _ramp_tutor_n(m):
    n = _n(m.group(1))
    tapped = "tapped" in m.group(0).lower()
    dest = "battlefield_tapped" if tapped else "battlefield"
    return Tutor(query=Filter(base="basic_land_card", quantifier="up_to_n",
                              count=n, targeted=False),
                 destination=dest, shuffle_after=True)


# ============================================================================
# GROUP 36: "This turn" / timing effects
# ============================================================================
# "you can't lose the game this turn" (3)
@_eff(r"^you can'?t lose the game this turn(?:\.|$)")
def _cant_lose_this_turn(m):
    return Modification(kind="cant_lose_typed", args=("this_turn",))


# "your opponents can't gain life this turn" (3)
@_eff(r"^your opponents? can'?t gain life this turn(?:\.|$)")
def _opps_cant_gain_life(m):
    return Modification(kind="cant_gain_life_typed", args=("opponents", "this_turn"))


# "your opponents can't cast spells this turn" (3)
@_eff(r"^your opponents? can'?t cast (?:spells?|noncreature spells?|instant or sorcery spells?) this turn(?:\.|$)")
def _opps_cant_cast(m):
    return Modification(kind="cant_cast_typed", args=("opponents", "this_turn"))


# "damage can't be prevented this turn" (3)
@_eff(r"^damage can'?t be prevented this turn(?:\.|$)")
def _damage_cant_be_prevented(m):
    return Modification(kind="damage_cant_be_prevented_typed", args=())


# "players can't gain life this turn" (3)
@_eff(r"^players? can'?t gain life (?:this turn|as long as [^.]+)(?:\.|$)")
def _players_cant_gain_life(m):
    return Modification(kind="cant_gain_life_typed", args=("all",))


# ============================================================================
# GROUP 37: Sacrifice-unless-pay patterns
# ============================================================================
# "sacrifice ~ unless you pay {N}" (12)
@_eff(r"^sacrifice (?:~|this creature|this permanent|this enchantment) unless you pay (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _sac_unless_pay(m):
    return Modification(kind="sacrifice_unless_pay_typed", args=(m.group(1),))


# "sacrifice ~ at the beginning of the next end step" (4)
@_eff(r"^sacrifice (?:~|this creature|this permanent|it) at the beginning of the next (?:end step|cleanup step)(?:\.|$)")
def _sac_at_end_step(m):
    return Modification(kind="delayed_sacrifice_typed", args=("next_end_step",))


# ============================================================================
# GROUP 38: Phasing effects
# ============================================================================
# "target creature phases out" (3)
@_eff(r"^target (creature|permanent|nonland permanent) phases? out(?:\.|$)")
def _target_phases_out(m):
    return Modification(kind="phase_out_typed", args=(m.group(1),))


# "~ phases out" / "this creature phases out" (already in wave1a)
# Skip -- covered.

# "each creature your opponents control phases out" (2)
@_eff(r"^each (creature|nonland permanent) (?:your opponents?|an opponent) controls? phases? out(?:\.|$)")
def _opp_phase_out(m):
    return Modification(kind="phase_out_typed", args=("each_opponent_" + m.group(1),))


# ============================================================================
# GROUP 39: "That player" / "this player" effect variants
# ============================================================================
# "that player draws a card" (3)
@_eff(r"^that player draws? (" + _NUM_RE + r") cards?(?:\.|$)")
def _that_player_draws(m):
    n = _n(m.group(1))
    return Draw(count=n, target=Filter(base="that_player", targeted=False))


# "that player draws a card"
@_eff(r"^that player draws? a card(?:\.|$)")
def _that_player_draws_one(m):
    return Draw(count=1, target=Filter(base="that_player", targeted=False))


# "that player gains N life" (3)
@_eff(r"^that player gains? (" + _NUM_RE + r") life(?:\.|$)")
def _that_player_gains_life(m):
    n = _n(m.group(1))
    return GainLife(amount=n, target=Filter(base="that_player", targeted=False))


# ============================================================================
# GROUP 40: Destroy target variants
# ============================================================================
# "destroy target artifact or enchantment" (5)
@_eff(r"^destroy target (artifact or enchantment|artifact|enchantment|creature|planeswalker|land|nonland permanent|permanent|creature or planeswalker)(?:\.|$)")
def _destroy_target(m):
    return Destroy(target=Filter(base=m.group(1), targeted=True))


# "destroy target creature with flying" (3)
@_eff(r"^destroy target creature with ([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _destroy_target_with(m):
    return Destroy(target=Filter(base="creature", targeted=True,
                                  extra=(m.group(1).strip(),)))


# "destroy target tapped creature" (3)
@_eff(r"^destroy target (tapped|attacking|blocking|nonblack|nonwhite|nonblue|nonred|nongreen|non-[a-z]+) creature(?:\.|$)")
def _destroy_target_qualified(m):
    return Destroy(target=Filter(base="creature", targeted=True,
                                  extra=(m.group(1).strip(),)))


# "destroy up to N target creatures" (3)
@_eff(r"^destroy up to (" + _NUM_RE + r") target (creatures?|permanents?|artifacts?|enchantments?)(?:\.|$)")
def _destroy_up_to_n(m):
    n = _n(m.group(1))
    base = m.group(2).lower().rstrip("s")
    return Destroy(target=Filter(base=base, quantifier="up_to_n",
                                  count=n, targeted=True))


# ============================================================================
# GROUP 41: "Put N +1/+1 counters on each creature you control" (3)
# ============================================================================
@_eff(r"^put (" + _NUM_RE + r") ([+-]\d+/[+-]\d+) counters? on each (creature|permanent|artifact) you control(?:\.|$)")
def _counter_each_yours(m):
    n = _n(m.group(1))
    kind = m.group(2)
    return CounterMod(op="put", count=n, counter_kind=kind,
                      target=Filter(base=m.group(3), quantifier="each",
                                    you_control=True))


# "put a +1/+1 counter on each creature you control" (4)
@_eff(r"^put a ([+-]\d+/[+-]\d+) counter on each (creature|permanent|artifact) you control(?:\.|$)")
def _counter_one_each_yours(m):
    return CounterMod(op="put", count=1, counter_kind=m.group(1),
                      target=Filter(base=m.group(2), quantifier="each",
                                    you_control=True))


# "put N +1/+1 counters on target creature" (already partially covered)
@_eff(r"^put (" + _NUM_RE + r") ([+-]\d+/[+-]\d+) counters? on target (creature|permanent|artifact)(?:\.|$)")
def _counter_target(m):
    n = _n(m.group(1))
    kind = m.group(2)
    return CounterMod(op="put", count=n, counter_kind=kind,
                      target=Filter(base=m.group(3), targeted=True))


# "remove a +1/+1 counter from ~" (3)
@_eff(r"^remove (" + _NUM_RE + r") ([+-]\d+/[+-]\d+|[a-z]+) counters? from (?:~|this creature|this permanent|it)(?:\.|$)")
def _remove_counter_self(m):
    n = _n(m.group(1))
    return CounterMod(op="remove", count=n, counter_kind=m.group(2),
                      target=Filter(base="self", targeted=False))


# "remove all +1/+1 counters from ~" (2)
@_eff(r"^remove all ([+-]\d+/[+-]\d+|[a-z]+) counters from (?:~|this creature|this permanent|it)(?:\.|$)")
def _remove_all_counters_self(m):
    return CounterMod(op="remove", count="all", counter_kind=m.group(1),
                      target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 42: Discard variants
# ============================================================================
# "target player discards N cards" (3)
@_eff(r"^target (?:player|opponent) discards? (" + _NUM_RE + r") cards?(?:\.|$)")
def _target_discards(m):
    n = _n(m.group(1))
    who = TARGET_OPPONENT if "opponent" in m.group(0).lower() else TARGET_PLAYER
    return Discard(count=n, target=who)


# "discard your hand" (3)
@_eff(r"^discard your hand(?:\.|$)")
def _discard_hand_wc(m):
    return Discard(count="all", target=SELF)


# "target opponent discards a card" (3)
@_eff(r"^target opponent discards? a card(?:\.|$)")
def _target_opp_discards_one(m):
    return Discard(count=1, target=TARGET_OPPONENT)


# ============================================================================
# GROUP 43: "All creatures/permanents" global restriction effects
# ============================================================================
# "all creatures have [keyword]" (4)
@_eff(r"^all creatures have (haste|flying|vigilance|deathtouch|lifelink|trample|first strike|fear|intimidate|menace|reach|shroud|hexproof)(?:\.|$)")
def _all_creatures_have(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="creature", quantifier="all"),
        duration="permanent",
    )


# "all creatures attack each combat if able" (3)
@_eff(r"^all creatures attack each combat if able(?:\.|$)")
def _all_must_attack(m):
    return GrantAbility(
        ability_name="must_attack",
        target=Filter(base="creature", quantifier="all"),
        duration="permanent",
    )


# "creatures you control have [keyword]" (already covered above)
# Skip duplicate

# "each player can't cast more than one spell each turn" (5)
@_eff(r"^each player can'?t cast more than (" + _NUM_RE + r") spells? each turn(?:\.|$)")
def _rule_of_law(m):
    n = _n(m.group(1))
    return Modification(kind="cast_limit_typed", args=(n, "each_player"))


# "nonland permanents your opponents control enter tapped" (4)
@_eff(r"^nonland permanents? (?:your opponents?|an opponent) controls? enter (?:the battlefield )?tapped(?:\.|$)")
def _opp_nonland_etb_tapped(m):
    return Modification(kind="etb_tapped_typed", args=("opponent_nonland",))


# ============================================================================
# GROUP 44: "Whenever" trigger tails as effects
# ============================================================================
# "you and permanents you control gain hexproof until end of turn" (3)
@_eff(r"^you and permanents you control gain (hexproof|indestructible) until end of turn(?:\.|$)")
def _you_and_permanents_gain(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="player_and_permanents", you_control=True),
        duration="until_end_of_turn",
    )


# "~ has indestructible as long as it has a [type] counter on it" (4)
@_eff(r"^(?:~|this creature|this permanent) has (indestructible|hexproof|flying|vigilance) as long as (?:it has|there (?:is|are)) (?:a |an |one or more )?([a-z]+(?:/[a-z]+)?) counters? on (?:it|~)(?:\.|$)")
def _conditional_keyword_counter(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="self", targeted=False),
        duration="conditional",
    )


# ============================================================================
# GROUP 45: Flashback / escape / foretell / disturb cost actions
# ============================================================================
# "flashback {cost}" — as effect (these show up as verb phrases sometimes)
@_eff(r"^flashback (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _flashback_cost(m):
    return Modification(kind="flashback_typed", args=(m.group(1),))


# "escape -- {cost}, exile N other cards from your graveyard" (3)
@_eff(r"^escape\s*[-—]\s*(\{[^}]+\}(?:\{[^}]+\})*),?\s*exile (" + _NUM_RE + r") other cards from your graveyard(?:\.|$)")
def _escape_cost(m):
    return Modification(kind="escape_typed", args=(m.group(1), _n(m.group(2))))


# "foretell {cost}" (3)
@_eff(r"^foretell (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _foretell_cost(m):
    return Modification(kind="foretell_typed", args=(m.group(1),))


# "disturb {cost}" (3)
@_eff(r"^disturb (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _disturb_cost(m):
    return Modification(kind="disturb_typed", args=(m.group(1),))


# "unearth {cost}" (3)
@_eff(r"^unearth (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _unearth_cost(m):
    return Modification(kind="unearth_typed", args=(m.group(1),))


# "encore {cost}" (2)
@_eff(r"^encore (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _encore_cost(m):
    return Modification(kind="encore_typed", args=(m.group(1),))


# "embalm {cost}" (3)
@_eff(r"^embalm (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _embalm_cost(m):
    return Modification(kind="embalm_typed", args=(m.group(1),))


# "eternalize {cost}" (3)
@_eff(r"^eternalize (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _eternalize_cost(m):
    return Modification(kind="eternalize_typed", args=(m.group(1),))


# "overload {cost}" (3)
@_eff(r"^overload (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _overload_cost(m):
    return Modification(kind="overload_typed", args=(m.group(1),))


# "kicker {cost}" (3)
@_eff(r"^kicker (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _kicker_cost(m):
    return Modification(kind="kicker_typed", args=(m.group(1),))


# "multikicker {cost}" (2)
@_eff(r"^multikicker (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _multikicker_cost(m):
    return Modification(kind="multikicker_typed", args=(m.group(1),))


# "buyback {cost}" (3)
@_eff(r"^buyback (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _buyback_cost(m):
    return Modification(kind="buyback_typed", args=(m.group(1),))


# "madness {cost}" (3)
@_eff(r"^madness (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _madness_cost(m):
    return Modification(kind="madness_typed", args=(m.group(1),))


# "suspend N -- {cost}" (3)
@_eff(r"^suspend (" + _NUM_RE + r")\s*[-—]\s*(\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _suspend_cost(m):
    return Modification(kind="suspend_typed", args=(_n(m.group(1)), m.group(2)))


# "retrace" (bare keyword)
@_eff(r"^retrace(?:\.|$)")
def _retrace(m):
    return Modification(kind="retrace_typed", args=())


# "cycling {cost}" (3)
@_eff(r"^cycling (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _cycling_cost(m):
    return Modification(kind="cycling_typed", args=(m.group(1),))


# "basic landcycling {cost}" (3)
@_eff(r"^(?:basic land|swamp|island|mountain|forest|plains)cycling (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _landcycling_cost(m):
    return Modification(kind="landcycling_typed", args=(m.group(1),))


# ============================================================================
# GROUP 46: Choose one / modal effects
# ============================================================================
# "choose one --" header (9+)
@_eff(r"^choose one(?:\s*[-—])?(?:\.|$)")
def _choose_one_header(m):
    return Modification(kind="modal_header_typed", args=(1,))


# "choose two --" (3)
@_eff(r"^choose two(?:\s*[-—])?(?:\.|$)")
def _choose_two_header(m):
    return Modification(kind="modal_header_typed", args=(2,))


# "choose up to N --" (3)
@_eff(r"^choose up to (" + _NUM_RE + r")(?:\s*[-—])?(?:\.|$)")
def _choose_up_to_header(m):
    return Modification(kind="modal_header_typed", args=("up_to", _n(m.group(1))))


# ============================================================================
# GROUP 47: Win the game
# ============================================================================
# "you win the game" (3)
@_eff(r"^you win the game(?:\.|$)")
def _you_win(m):
    from mtg_ast import WinGame
    return WinGame(target=SELF)


# "target player loses the game" (2)
@_eff(r"^target (?:player|opponent) loses the game(?:\.|$)")
def _target_loses_game(m):
    from mtg_ast import LoseGame
    who = TARGET_OPPONENT if "opponent" in m.group(0).lower() else TARGET_PLAYER
    return LoseGame(target=who)


# ============================================================================
# GROUP 48: Noncombat damage redirection / dealing
# ============================================================================
# "deal N damage to target creature or player" (common effect phrasing)
@_eff(r"^deal (\d+) damage to (target [^.]+|any target|each (?:opponent|player|creature))(?:\.|$)")
def _deal_n_damage(m):
    n = int(m.group(1))
    text_low = m.group(2).lower()
    if "each opponent" in text_low:
        target = EACH_OPPONENT
    elif "each player" in text_low:
        target = EACH_PLAYER
    elif "each creature" in text_low:
        target = Filter(base="creature", quantifier="each")
    elif "any target" in text_low:
        target = TARGET_ANY
    else:
        target = Filter(base=text_low.replace("target ", ""), targeted=True)
    return Damage(amount=n, target=target)


# ============================================================================
# GROUP 49: "Exile ~ with N time counters on it" (suspend from play)
# ============================================================================
@_eff(r"^exile (?:~|this creature|this permanent) with (" + _NUM_RE + r") time counters? on (?:it|~)(?:\.|$)")
def _exile_self_time_counters(m):
    n = _n(m.group(1))
    return Sequence(items=(
        Exile(target=Filter(base="self", targeted=False)),
        CounterMod(op="put", count=n, counter_kind="time",
                   target=Filter(base="self", targeted=False)),
    ))


# ============================================================================
# GROUP 50: Placeholder / catch-all typed stubs for remaining keyword actions
# ============================================================================

# "decayed" (3) -- token modifier keyword
@_eff(r"^decayed(?:\.|$)")
def _decayed(m):
    return Modification(kind="decayed_typed", args=())


# "blitz {cost}" (3)
@_eff(r"^blitz (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _blitz_cost(m):
    return Modification(kind="blitz_typed", args=(m.group(1),))


# "dash {cost}" (3)
@_eff(r"^dash (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _dash_cost(m):
    return Modification(kind="dash_typed", args=(m.group(1),))


# "evoke {cost}" (3)
@_eff(r"^evoke (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _evoke_cost(m):
    return Modification(kind="evoke_typed", args=(m.group(1),))


# "mutate {cost}" (3)
@_eff(r"^mutate (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _mutate_cost(m):
    return Modification(kind="mutate_typed", args=(m.group(1),))


# "ninjutsu {cost}" (3)
@_eff(r"^ninjutsu (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _ninjutsu_cost(m):
    return Modification(kind="ninjutsu_typed", args=(m.group(1),))


# "prowl {cost}" (2)
@_eff(r"^prowl (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _prowl_cost(m):
    return Modification(kind="prowl_typed", args=(m.group(1),))


# "spectacle {cost}" (3)
@_eff(r"^spectacle (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _spectacle_cost(m):
    return Modification(kind="spectacle_typed", args=(m.group(1),))


# "surge {cost}" (2)
@_eff(r"^surge (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _surge_cost(m):
    return Modification(kind="surge_typed", args=(m.group(1),))


# "bestow {cost}" (3)
@_eff(r"^bestow (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _bestow_cost(m):
    return Modification(kind="bestow_typed", args=(m.group(1),))


# "prowess" (bare keyword action) (3)
@_eff(r"^prowess(?:\.|$)")
def _prowess(m):
    return Modification(kind="prowess_typed", args=())


# "exploit" (bare keyword action)
@_eff(r"^exploit(?:\.|$)")
def _exploit(m):
    return Optional_(body=Sacrifice(
        query=Filter(base="creature", you_control=True, targeted=False),
    ))


# "fabricate N" (3)
@_eff(r"^fabricate (\d+)(?:\.|$)")
def _fabricate(m):
    n = int(m.group(1))
    return Modification(kind="fabricate_typed", args=(n,))


# "afterlife N" (3)
@_eff(r"^afterlife (\d+)(?:\.|$)")
def _afterlife(m):
    n = int(m.group(1))
    return CreateToken(count=n, types=("spirit",), pt=(1, 1))


# "riot" (3)
@_eff(r"^riot(?:\.|$)")
def _riot(m):
    return Modification(kind="riot_typed", args=())


# "mentor" (3)
@_eff(r"^mentor(?:\.|$)")
def _mentor(m):
    return Modification(kind="mentor_typed", args=())


# "ascend" (3)
@_eff(r"^ascend(?:\.|$)")
def _ascend(m):
    return Modification(kind="ascend_typed", args=())


# "storied" (3) — CR §702.195 (Aug 7 2026 edition); mirrors ascend_typed
@_eff(r"^storied(?:\.|$)")
def _storied(m):
    return Modification(kind="storied_typed", args=())


# "convoke" (3)
@_eff(r"^convoke(?:\.|$)")
def _convoke(m):
    return Modification(kind="convoke_typed", args=())


# "delve" (3)
@_eff(r"^delve(?:\.|$)")
def _delve(m):
    return Modification(kind="delve_typed", args=())


# "dredge N" (3)
@_eff(r"^dredge (\d+)(?:\.|$)")
def _dredge(m):
    return Modification(kind="dredge_typed", args=(int(m.group(1)),))


# "undying" (3)
@_eff(r"^undying(?:\.|$)")
def _undying(m):
    return Modification(kind="undying_typed", args=())


# "persist" (3)
@_eff(r"^persist(?:\.|$)")
def _persist(m):
    return Modification(kind="persist_typed", args=())


# "modular N" (3)
@_eff(r"^modular (\d+)(?:\.|$)")
def _modular(m):
    return CounterMod(op="put", count=int(m.group(1)), counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# "annihilator N" (3)
@_eff(r"^annihilator (\d+)(?:\.|$)")
def _annihilator(m):
    n = int(m.group(1))
    return Sacrifice(query=Filter(base="permanent", you_control=True),
                     actor="each_opponent")


# "dethrone" (3)
@_eff(r"^dethrone(?:\.|$)")
def _dethrone(m):
    return Modification(kind="dethrone_typed", args=())


# "extort" (3)
@_eff(r"^extort(?:\.|$)")
def _extort(m):
    return Modification(kind="extort_typed", args=())


# "devour N" (3)
@_eff(r"^devour (\d+)(?:\.|$)")
def _devour(m):
    return Modification(kind="devour_typed", args=(int(m.group(1)),))


# "evolve" (3)
@_eff(r"^evolve(?:\.|$)")
def _evolve(m):
    return Modification(kind="evolve_typed", args=())


# "unleash" (3)
@_eff(r"^unleash(?:\.|$)")
def _unleash(m):
    return Modification(kind="unleash_typed", args=())


# "totem armor" (3)
@_eff(r"^totem armor(?:\.|$)")
def _totem_armor(m):
    return Modification(kind="totem_armor_typed", args=())


# "soulbond" (3)
@_eff(r"^soulbond(?:\.|$)")
def _soulbond(m):
    return Modification(kind="soulbond_typed", args=())


# "myriad" (3)
@_eff(r"^myriad(?:\.|$)")
def _myriad(m):
    return Modification(kind="myriad_typed", args=())


# "melee" (3)
@_eff(r"^melee(?:\.|$)")
def _melee(m):
    return Modification(kind="melee_typed", args=())


# "partner" / "partner with [name]" (3)
@_eff(r"^partner(?:\s+with .+)?(?:\.|$)")
def _partner(m):
    return Modification(kind="partner_typed", args=(m.group(0),))


# "companion" (2)
@_eff(r"^companion(?:\.|$)")
def _companion(m):
    return Modification(kind="companion_typed", args=())


# ============================================================================
# GROUP 51: Saga chapter typed body promotions
# ============================================================================
# "i - [effect]" / "ii - [effect]" etc. that contain a parseable body
# These are already handled by sagas_adventures.py STATIC_PATTERNS, but
# some appear as bare effect text fragments. Route them through parse_effect
# for typed promotion.

# "exile this saga, then return it to the battlefield transformed" (5)
@_eff(r"^exile this saga,?\s*then return (?:it|this saga) to the battlefield (?:transformed )?under (?:your|its owner'?s?) control(?:\.|$)")
def _saga_exile_transform(m):
    return Sequence(items=(
        Exile(target=Filter(base="self", targeted=False)),
        Reanimate(query=Filter(base="self", targeted=False)),
    ))


# ============================================================================
# GROUP 52: "Spend mana as though" effects
# ============================================================================
# "you may spend mana as though it were mana of any color" (5)
@_eff(r"^you may spend mana as though it were mana of any (?:color|type)(?:\s+to cast [^.]+)?(?:\.|$)")
def _spend_any_color(m):
    return Modification(kind="spend_any_color_typed", args=())


# ============================================================================
# GROUP 53: "Draw cards equal to" / variable draw
# ============================================================================
# "draw cards equal to the number of creatures you control" (3)
@_eff(r"^draw cards equal to (?:the number of |its |~'?s? )(.+?)(?:\.|$)")
def _draw_equal_to(m):
    return Draw(count="var", target=SELF)


# "draw cards equal to its power" (3)
@_eff(r"^draw cards equal to (?:its|~'?s?|target creature'?s?) power(?:\.|$)")
def _draw_equal_power(m):
    return Draw(count="var", target=SELF)


# ============================================================================
# GROUP 54: Exile/return flicker combos for other creatures
# ============================================================================
# "exile another target creature you control, then return that card" (3)
@_eff(r"^exile another target (?:creature|permanent)(?: you control)?,?\s*then return (?:that card|it) to the battlefield under (?:its owner'?s?|your) control(?:\.|$)")
def _flicker_another(m):
    return Sequence(items=(
        Exile(target=Filter(base="creature", targeted=True, extra=("other",))),
        Reanimate(query=Filter(base="that_card", targeted=False)),
    ))


# ============================================================================
# GROUP 55: Create X/X token where X = variable
# ============================================================================
# "create an X/X green ooze creature token" (3)
@_eff(r"^create (" + _NUM_RE + r" )?(?:an? )?x/x ([a-z]+(?: [a-z]+)*?) creature tokens?(?:\.|$)")
def _create_xx_token(m):
    n = _n(m.group(1)) if m.group(1) else 1
    types = tuple(m.group(2).strip().split())
    return CreateToken(count=n, pt=("x", "x"), types=types)


# ============================================================================
# GROUP 56: "Whenever ~ attacks" trigger-as-effect tails
# ============================================================================
# "~ attacks each combat if able" (5)
@_eff(r"^(?:~|this creature) attacks each combat if able(?:\.|$)")
def _must_attack_each_combat(m):
    return GrantAbility(
        ability_name="must_attack",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "~ blocks each combat if able" (3)
@_eff(r"^(?:~|this creature) blocks each combat if able(?:\.|$)")
def _must_block_each_combat(m):
    return GrantAbility(
        ability_name="must_block",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "~ can't be sacrificed" (2)
@_eff(r"^(?:~|this creature|this permanent) can'?t be sacrificed(?:\.|$)")
def _cant_be_sacrificed(m):
    return Modification(kind="cant_be_sacrificed_typed", args=())


# "~ can't be countered" (3)
@_eff(r"^(?:~|this spell) can'?t be countered(?:\.|$)")
def _cant_be_countered(m):
    return Modification(kind="cant_be_countered_typed", args=())


# "~ is all colors" (4)
@_eff(r"^(?:~|this creature|this permanent) is all colors(?:\.|$)")
def _is_all_colors(m):
    return Modification(kind="type_change_typed", args=("all_colors",))


# ============================================================================
# Final: all handlers registered via module-level decorator side effects.
# ============================================================================
