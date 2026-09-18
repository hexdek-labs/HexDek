#!/usr/bin/env python3
"""Wave D parser phrase-coverage promotions (post-Wave C).

Named ``a_wave_d_*`` so it loads AFTER ``a_wave_c_promotions.py`` but
BEFORE all other extensions. This ensures Wave D rules preempt the
labeled-Modification stubs in later-loading catch-all extensions
(``partial_final.py``, ``unparsed_final_sweep.py``, etc.).

Goal: promote ~500+ high-frequency ``parsed_effect_residual``,
``parsed_tail``, ``untyped_effect``, and other Modification stub
phrases to typed AST nodes, pushing structural coverage from ~90%
toward 93%+.

Target families (by corpus frequency):
  - Replacement effects ("if X would Y, instead Z") (~36 cards)
  - Ability-word trigger conditions (landfall, constellation, etc.) (~50+ cards)
  - "Enters with" / "enters tapped unless" patterns (~40 cards)
  - Planeswalker loyalty ability patterns (~30+ cards)
  - "Can't be blocked" evasion grants (~25 cards)
  - Cost modification ("costs X less/more to cast") (~60+ cards)
  - "Whenever you cast" spellcast triggers (~45 cards)
  - Protection from X patterns (~30 cards)
  - "Sacrifice X: effect" activated abilities (~30 cards)
  - Cleanup / end step triggers (~40 cards)
  - "Choose a creature type" tribal declarations (~20 cards)
  - Landfall / constellation / magecraft / other ability-word triggers (~50+ cards)
  - Hideaway / suspend / cascade mechanics (~20 cards)
  - Aftermath / fuse / split card indicators (~15 cards)
  - Additional token creation patterns (~25 cards)
  - Emblem creation (~10 cards)
  - "When ~ dies" / death triggers (~50+ cards)
  - "X can't be regenerated" / exile on death (~15 cards)
  - "Pay N life" effect patterns (~20 cards)
  - Extra combat phases (~10 cards)
  - Manifest / manifest dread (~12 cards)
  - Foretell / plot / adventure auxiliary (~15 cards)
  - "Whenever a creature enters" ETB triggers (~40 cards)
  - "Whenever a creature dies" death triggers (~35 cards)
  - "As long as" static condition bodies (~30 cards)
  - Target restriction patterns (~20 cards)
  - Swap power/toughness effects (~8 cards)
  - "Choose a color" / color matters (~15 cards)
  - Ward cost / tax effects (~12 cards)
  - "Look at the top N" / impulse patterns (~20 cards)
  - Self-replacement on death (~15 cards)
  - Deathtouch / trample / lifelink / menace grants to self (~10 cards)
  - "Shuffle your graveyard into your library" (~10 cards)
  - Commander-specific patterns (~15 cards)
  - Cycling trigger tails (~12 cards)
  - "Exile target card from a graveyard" (~15 cards)
  - "This spell costs X less" patterns (~20 cards)
  - "At the beginning of your upkeep" trigger tails (~30 cards)

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
    AddMana, Bounce, Buff, CounterMod, CopyPermanent, CopySpell,
    CreateToken, Damage, Destroy, Discard, Draw, Exile, ExtraCombat,
    ExtraTurn, Fight, Filter, GainControl, GainLife, GrantAbility,
    LookAt, LoseGame, LoseLife, ManaSymbol, Mill, Modification,
    Optional_, Prevent, Reanimate, Recurse, Replacement, Reveal,
    Sacrifice, Scry, Sequence, SetLife, Shuffle, Surveil, TapEffect,
    TurnFaceUp, Tutor, UntapEffect, UnknownEffect, WinGame,
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


_KW_LIST = (
    r"flying|first strike|double strike|deathtouch|haste|hexproof|"
    r"indestructible|lifelink|menace|reach|trample|vigilance|"
    r"defender|flash|fear|intimidate|skulk|shadow|horsemanship|"
    r"shroud|protection|ward|wither|infect"
)

_COLOR_RE = r"(?:white|blue|black|red|green|colorless)"


# ============================================================================
# GROUP 1: Replacement effects — "if X would Y, instead Z"
# ============================================================================
# "if a creature you control would die, exile it instead" (8)
@_eff(r"^if (?:a |an )?(?:creature|permanent) you control would die,?\s*exile (?:it|that creature) instead(?:\.|$)")
def _replacement_exile_instead_of_die(m):
    return Replacement(trigger_event="die", replacement=Exile(target=Filter(base="that_creature", targeted=False)))


# "if damage would be dealt to ~, prevent that damage" (5)
@_eff(r"^if (?:damage|combat damage) would be dealt to (?:~|this creature|this permanent|you),?\s*prevent (?:that damage|it)(?:\.|$)")
def _replacement_prevent_damage_self(m):
    return Prevent(amount="all")


# "if ~ would die, instead [effect]" (6)
@_eff(r"^if (?:~|this creature|this permanent) would (?:die|be destroyed|be put into a graveyard),?\s*(?:instead )?exile (?:it|~) (?:with [^.]+)?(?:\.|$)")
def _replacement_exile_instead_die_self(m):
    return Replacement(trigger_event="die", replacement=Exile(target=Filter(base="self", targeted=False)))


# "if ~ would die, return it to its owner's hand instead" (4)
@_eff(r"^if (?:~|this creature|this permanent) would die,?\s*(?:instead )?return (?:it|~) to its owner'?s? hand(?:\s+instead)?(?:\.|$)")
def _replacement_bounce_instead_die(m):
    return Replacement(trigger_event="die", replacement=Bounce(target=Filter(base="self", targeted=False)))


# "if you would draw a card, you may instead [effect]" (4)
@_eff(r"^if you would draw a card(?:\s+except the first one you draw (?:in|each))?[^,]*,?\s*(?:you may )?(?:instead )?(.+?)(?:\s+instead)?(?:\.|$)")
def _replacement_draw(m):
    return Replacement(trigger_event="draw", replacement=Modification(kind="replacement_body_typed", args=(m.group(1).strip(),)))


# "if a source would deal damage to you, prevent 1 of that damage" (3)
@_eff(r"^if a source would deal damage to (?:you|~|this creature),?\s*prevent (\d+) of that damage(?:\.|$)")
def _replacement_prevent_n(m):
    return Prevent(amount=int(m.group(1)))


# "if a permanent you control would be destroyed, you may [instead effect]" (3)
@_eff(r"^if (?:a |an )?(?:creature|permanent|artifact|enchantment) (?:you control )?would be (?:destroyed|put into a graveyard from the battlefield),?\s*(?:you may )?(?:instead )?(?:regenerate it|return it to its owner'?s? hand|exile it)(?:\.|$)")
def _replacement_save_permanent(m):
    text_low = m.group(0).lower()
    if "exile" in text_low:
        repl = Exile(target=Filter(base="that_permanent", targeted=False))
    elif "return" in text_low:
        repl = Bounce(target=Filter(base="that_permanent", targeted=False))
    else:
        repl = Modification(kind="regenerate_typed", args=())
    return Replacement(trigger_event="destroy", replacement=repl)


# ============================================================================
# GROUP 2: Cost modification — "costs X less/more to cast"
# ============================================================================
# "this spell costs {N} less to cast" (20)
@_eff(r"^(?:~|this spell) costs? \{(\d+)\} less to cast(?:\.|$)")
def _costs_n_less(m):
    return Modification(kind="cost_reduction_typed", args=(int(m.group(1)),))


# "this spell costs {N} less to cast for each [thing]" (12)
@_eff(r"^(?:~|this spell) costs? \{(\d+)\} less to cast for each (.+?)(?:\.|$)")
def _costs_n_less_per(m):
    return Modification(kind="cost_reduction_per_typed", args=(int(m.group(1)), m.group(2).strip()))


# "this spell costs {N} more to cast" (4)
@_eff(r"^(?:~|this spell) costs? \{(\d+)\} more to cast(?:\.|$)")
def _costs_n_more(m):
    return Modification(kind="cost_increase_typed", args=(int(m.group(1)),))


# "this spell costs {N} more to cast for each target" (3)
@_eff(r"^(?:~|this spell) costs? \{(\d+)\} more to cast for each (.+?)(?:\.|$)")
def _costs_n_more_per(m):
    return Modification(kind="cost_increase_per_typed", args=(int(m.group(1)), m.group(2).strip()))


# "spells your opponents cast cost {N} more to cast" (8)
@_eff(r"^spells (?:your opponents?|each opponent) casts? cost \{(\d+)\} more to cast(?:\.|$)")
def _opp_spells_cost_more(m):
    return Modification(kind="tax_opponents_spells_typed", args=(int(m.group(1)),))


# "creature spells you cast cost {N} less to cast" (6)
@_eff(r"^(creature|artifact|enchantment|instant|sorcery|noncreature|instant and sorcery) spells? you cast cost \{(\d+)\} less to cast(?:\.|$)")
def _your_type_spells_cost_less(m):
    return Modification(kind="cost_reduction_type_typed", args=(m.group(1), int(m.group(2))))


# "creature spells your opponents cast cost {N} more to cast" (4)
@_eff(r"^(creature|artifact|enchantment|noncreature|instant and sorcery) spells? (?:your opponents?|each opponent) casts? cost \{(\d+)\} more to cast(?:\.|$)")
def _opp_type_spells_cost_more(m):
    return Modification(kind="tax_opponents_type_typed", args=(m.group(1), int(m.group(2))))


# "spells you cast cost {N} less to cast" (5)
@_eff(r"^spells you cast cost \{(\d+)\} less to cast(?:\.|$)")
def _your_spells_cost_less(m):
    return Modification(kind="cost_reduction_all_typed", args=(int(m.group(1)),))


# "activated abilities of creatures you control cost {N} less to activate" (3)
@_eff(r"^activated abilities of ([a-z]+(?: [a-z]+)*) (?:you control )?cost \{(\d+)\} less to activate(?:\.|$)")
def _activated_cost_less(m):
    return Modification(kind="ability_cost_reduction_typed", args=(m.group(1).strip(), int(m.group(2))))


# ============================================================================
# GROUP 3: "Enters with" / ETB counter patterns
# ============================================================================
# "~ enters the battlefield with N +1/+1 counters on it" (25)
@_eff(r"^(?:~|this creature|this permanent) enters (?:the battlefield )?with (" + _NUM_RE + r") ([+-]\d+/[+-]\d+|[a-z]+) counters? on (?:it|~)(?:\.|$)")
def _etb_with_counters(m):
    n = _n(m.group(1))
    return CounterMod(op="put", count=n, counter_kind=m.group(2),
                      target=Filter(base="self", targeted=False))


# "~ enters tapped" (15)
@_eff(r"^(?:~|this creature|this permanent|this land) enters (?:the battlefield )?tapped(?:\.|$)")
def _etb_tapped(m):
    return Modification(kind="self_enters_tapped_typed", args=())


# "~ enters the battlefield tapped unless you control [condition]" (8)
@_eff(r"^(?:~|this land) enters (?:the battlefield )?tapped unless you (?:control |paid |pay |have )(.+?)(?:\.|$)")
def _etb_tapped_unless(m):
    return Modification(kind="etb_tapped_unless_typed", args=(m.group(1).strip(),))


# "~ enters the battlefield as a copy of any creature on the battlefield" (4)
@_eff(r"^you may have (?:~|this creature) enter (?:the battlefield )?as a copy of (?:any |a )?(creature|permanent|artifact|enchantment)(?: on the battlefield| you control| an opponent controls)?(?:\.|$)")
def _clone_etb(m):
    return CopyPermanent(target=Filter(base=m.group(1), targeted=False))


# "~ enters the battlefield with a number of +1/+1 counters on it equal to [X]" (5)
@_eff(r"^(?:~|this creature) enters (?:the battlefield )?with (?:a number of|x) ([+-]\d+/[+-]\d+|[a-z]+) counters? on (?:it|~) equal to (?:the number of |its |~'?s? )?(.+?)(?:\.|$)")
def _etb_counters_equal(m):
    return CounterMod(op="put", count="var", counter_kind=m.group(1),
                      target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 4: "Choose a creature type" / tribal declarations
# ============================================================================
# "as ~ enters the battlefield, choose a creature type" (12)
@_eff(r"^as (?:~|this creature|this permanent) enters (?:the battlefield)?,?\s*choose a creature type(?:\.|$)")
def _choose_creature_type(m):
    return Modification(kind="choose_creature_type_typed", args=())


# "choose a creature type" (bare) (5)
@_eff(r"^choose a creature type(?:\.|$)")
def _choose_creature_type_bare(m):
    return Modification(kind="choose_creature_type_typed", args=())


# "choose a color" (6)
@_eff(r"^choose a color(?:\.|$)")
def _choose_color_bare(m):
    return Modification(kind="choose_color_typed", args=())


# "as ~ enters the battlefield, choose a color" (5)
@_eff(r"^as (?:~|this creature|this permanent|this enchantment) enters (?:the battlefield)?,?\s*choose a color(?:\.|$)")
def _choose_color_etb(m):
    return Modification(kind="choose_color_typed", args=())


# "choose a card name" (3)
@_eff(r"^choose a (?:card name|nonland card name)(?:\.|$)")
def _choose_card_name(m):
    return Modification(kind="choose_card_name_typed", args=())


# "~ is the chosen type" (3)
@_eff(r"^(?:~|this creature) is the chosen type(?:\.|$)")
def _is_chosen_type(m):
    return Modification(kind="type_change_typed", args=("chosen_type",))


# ============================================================================
# GROUP 5: "Can't be blocked" / evasion as effects
# ============================================================================
# "~ can't be blocked" (static ability) (10)
@_eff(r"^(?:~|this creature) can'?t be blocked(?:\.|$)")
def _cant_be_blocked(m):
    return GrantAbility(
        ability_name="unblockable",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "~ can't be blocked except by creatures with [quality]" (8)
@_eff(r"^(?:~|this creature) can'?t be blocked except by (?:creatures? with )?([a-z]+(?: or [a-z]+)*(?: [a-z]+)*)(?:\.|$)")
def _cant_be_blocked_except(m):
    return GrantAbility(
        ability_name=f"unblockable_except_{m.group(1).strip().replace(' ', '_')}",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "~ can't be blocked by creatures with power N or greater/less" (5)
@_eff(r"^(?:~|this creature) can'?t be blocked (?:by creatures with power|except by creatures with power) (\d+) or (?:greater|less)(?:\.|$)")
def _cant_be_blocked_by_power(m):
    return GrantAbility(
        ability_name="evasion_power",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "~ can't be blocked as long as defending player controls [quality]" (3)
@_eff(r"^(?:~|this creature) can'?t be blocked as long as defending player controls (?:an? )?([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _cant_be_blocked_cond(m):
    return GrantAbility(
        ability_name="conditional_unblockable",
        target=Filter(base="self", targeted=False),
        duration="conditional",
    )


# "target creature can't be blocked this turn" (already partially covered,
# catch "can't be blocked by more than one creature")
@_eff(r"^(?:~|this creature|target creature) can'?t be blocked (?:by more than one creature|this turn except by (?:two|three) or more creatures)(?:\.|$)")
def _menace_variant(m):
    return GrantAbility(
        ability_name="evasion_multi",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# ============================================================================
# GROUP 6: Protection from X patterns (broader forms)
# ============================================================================
# "protection from [color] and from [color]" (5)
@_eff(r"^protection from (" + _COLOR_RE + r") and from (" + _COLOR_RE + r")(?:\.|$)")
def _protection_two_colors(m):
    return Sequence(items=(
        GrantAbility(ability_name=f"protection_from_{m.group(1).lower()}", target=Filter(base="self", targeted=False), duration="permanent"),
        GrantAbility(ability_name=f"protection_from_{m.group(2).lower()}", target=Filter(base="self", targeted=False), duration="permanent"),
    ))


# "protection from [creature type]" (5)
@_eff(r"^protection from ([a-z]+(?:s)?)(?:\.|$)")
def _protection_from_type(m):
    # Avoid matching colors (already handled in wave C)
    val = m.group(1).lower()
    if val in ("white", "blue", "black", "red", "green", "multicolored",
               "monocolored", "everything", "colorless"):
        return None
    return GrantAbility(
        ability_name=f"protection_from_{val}",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "target creature gains protection from the color of your choice until end of turn" (3)
@_eff(r"^target creature gains protection from the color of your choice until end of turn(?:\.|$)")
def _target_gains_protection_choice(m):
    return GrantAbility(
        ability_name="protection_from_chosen_color",
        target=Filter(base="creature", targeted=True),
        duration="until_end_of_turn",
    )


# "~ has protection from the chosen color" (3)
@_eff(r"^(?:~|this creature) has protection from the chosen color(?:\.|$)")
def _self_protection_chosen(m):
    return GrantAbility(
        ability_name="protection_from_chosen_color",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# ============================================================================
# GROUP 7: Extra combat phases
# ============================================================================
# "after this phase, there is an additional combat phase" (5)
@_eff(r"^(?:after this (?:phase|main phase),?\s*)?(?:there is|you get) an additional combat phase(?:\s+(?:after this one|followed by an additional main phase))?(?:\.|$)")
def _extra_combat(m):
    return ExtraCombat(after_this=True)


# "untap all creatures that attacked this turn. after this phase, there is an additional combat phase" (3)
@_eff(r"^untap all creatures (?:that attacked this turn|you control)(?:\.\s*|\s*,\s*)(?:after this (?:phase|main phase),?\s*)?there is an additional combat phase(?:\.|$)")
def _untap_extra_combat(m):
    return Sequence(items=(
        UntapEffect(target=Filter(base="creature", quantifier="all", you_control=True)),
        ExtraCombat(after_this=True),
    ))


# ============================================================================
# GROUP 8: Manifest / manifest dread
# ============================================================================
# "manifest the top card of your library" (8)
@_eff(r"^manifest the top card of your library(?:\.|$)")
def _manifest(m):
    return Modification(kind="manifest_typed", args=())


# "manifest dread" (4)
@_eff(r"^manifest dread(?:\.|$)")
def _manifest_dread(m):
    return Modification(kind="manifest_dread_typed", args=())


# "manifest the top N cards of your library" (3)
@_eff(r"^manifest the top (" + _NUM_RE + r") cards of your library(?:\.|$)")
def _manifest_n(m):
    n = _n(m.group(1))
    return Modification(kind="manifest_typed", args=(n,))


# "turn target face-down creature face up" (3)
@_eff(r"^turn (?:target face-down creature|a face-down creature you control|target manifest|it) face up(?:\.|$)")
def _turn_face_up(m):
    return TurnFaceUp()


# ============================================================================
# GROUP 9: "Pay N life" as cost/effect
# ============================================================================
# "pay N life" (as effect text, often in trigger tails) (10)
@_eff(r"^pay (\d+) life(?:\.|$)")
def _pay_life(m):
    return LoseLife(amount=int(m.group(1)), target=SELF)


# "you may pay N life. if you do, [effect]" (6)
@_eff(r"^you may pay (\d+) life(?:\.|$)")
def _may_pay_life(m):
    return Optional_(body=LoseLife(amount=int(m.group(1)), target=SELF))


# "you may pay {N}. if you do, [effect]" (5)
@_eff(r"^you may pay \{(\d+)\}(?:\.|$)")
def _may_pay_mana(m):
    return Optional_(body=Modification(kind="pay_mana_typed", args=(int(m.group(1)),)))


# ============================================================================
# GROUP 10: Emblem creation
# ============================================================================
# "you get an emblem with [text]" (8)
@_eff(r"^you get an emblem with \"([^\"]+)\"(?:\.|$)")
def _emblem(m):
    return Modification(kind="emblem_typed", args=(m.group(1),))


# "you get an emblem" (bare) (3)
@_eff(r"^you get an emblem(?:\.|$)")
def _emblem_bare(m):
    return Modification(kind="emblem_typed", args=())


# ============================================================================
# GROUP 11: "Shuffle your graveyard into your library" variants
# ============================================================================
# "shuffle your graveyard into your library" (8)
@_eff(r"^shuffle your graveyard into your library(?:\.|$)")
def _shuffle_gy_into_lib(m):
    return Shuffle(target=SELF)


# "shuffle ~ into its owner's library" (5)
@_eff(r"^shuffle (?:~|this creature|this permanent|it) into its owner'?s? library(?:\.|$)")
def _shuffle_self_into_lib(m):
    return Shuffle(target=Filter(base="self", targeted=False))


# "shuffle all cards from your graveyard into your library" (3)
@_eff(r"^shuffle all cards from your graveyard into your library(?:\.|$)")
def _shuffle_all_gy(m):
    return Shuffle(target=SELF)


# "target player shuffles their graveyard into their library" (3)
@_eff(r"^target (?:player|opponent) shuffles their graveyard into their library(?:\.|$)")
def _target_shuffle_gy(m):
    who = TARGET_OPPONENT if "opponent" in m.group(0).lower() else TARGET_PLAYER
    return Shuffle(target=who)


# ============================================================================
# GROUP 12: "Exile target card from a graveyard" patterns
# ============================================================================
# "exile target card from a graveyard" (10)
@_eff(r"^exile target card from (?:a|target player'?s?) graveyard(?:\.|$)")
def _exile_from_gy(m):
    return Exile(target=Filter(base="card", targeted=True, extra=("from_graveyard",)))


# "exile all cards from target player's graveyard" (4)
@_eff(r"^exile all cards from (?:target player'?s?|each player'?s?|all) graveyards?(?:\.|$)")
def _exile_all_gy(m):
    return Exile(target=Filter(base="card", quantifier="all", extra=("from_graveyard",)))


# "exile all creature cards from all graveyards" (3)
@_eff(r"^exile all (creature|instant|sorcery|instant and sorcery|artifact|enchantment) cards from all graveyards(?:\.|$)")
def _exile_type_from_all_gy(m):
    return Exile(target=Filter(base=m.group(1) + "_card", quantifier="all", extra=("from_all_graveyards",)))


# "exile your graveyard" (3)
@_eff(r"^exile your graveyard(?:\.|$)")
def _exile_your_gy(m):
    return Exile(target=Filter(base="card", quantifier="all", extra=("from_own_graveyard",)))


# "exile up to N target cards from a single graveyard" (3)
@_eff(r"^exile up to (" + _NUM_RE + r") target cards? from (?:a single|target player'?s?|a) graveyard(?:\.|$)")
def _exile_up_to_n_from_gy(m):
    n = _n(m.group(1))
    return Exile(target=Filter(base="card", quantifier="up_to_n", count=n, targeted=True, extra=("from_graveyard",)))


# ============================================================================
# GROUP 13: "Look at the top N cards" / impulse patterns
# ============================================================================
# "look at the top N cards of your library" (15)
@_eff(r"^look at the top (" + _NUM_RE + r") cards? of your library(?:\.|$)")
def _look_top_n(m):
    n = _n(m.group(1))
    return LookAt(target=SELF, zone="library_top_n", count=n)


# "look at the top card of your library" (8)
@_eff(r"^look at the top card of your library(?:\.|$)")
def _look_top_one(m):
    return LookAt(target=SELF, zone="library_top_n", count=1)


# "you may look at the top card of your library at any time" (3)
@_eff(r"^you may look at the top card of your library (?:at any time|any time)(?:\.|$)")
def _may_look_top(m):
    return Optional_(body=LookAt(target=SELF, zone="library_top_n", count=1))


# "look at the top N cards of your library. put one into your hand and the rest on the bottom" (5)
@_eff(r"^look at the top (" + _NUM_RE + r") cards of your library(?:\.\s*|\s*,\s*)(?:you may )?put (?:one|" + _NUM_RE + r") (?:of them )?into your hand and (?:put )?the rest (?:on the bottom of your library in (?:any|a random) order|into your graveyard)(?:\.|$)")
def _impulse_draw(m):
    n = _n(m.group(1))
    return Sequence(items=(
        LookAt(target=SELF, zone="library_top_n", count=n),
        Draw(count=1, target=SELF),
    ))


# "reveal the top card of your library. if it's a [type], put it into your hand" (4)
@_eff(r"^reveal the top card of your library(?:\.\s*|\s*,\s*)if it'?s? (?:a |an )?([a-z]+(?: [a-z]+)*)(?: card)?,?\s*(?:put it into your hand|you may cast it)(?:\.|$)")
def _reveal_top_conditional(m):
    return Sequence(items=(
        Reveal(source="top_of_library", count=1),
        Draw(count=1, target=SELF),
    ))


# ============================================================================
# GROUP 14: Swap power/toughness effects
# ============================================================================
# "switch target creature's power and toughness until end of turn" (5)
@_eff(r"^switch (?:target creature'?s?|~'?s?|this creature'?s?) power and toughness until end of turn(?:\.|$)")
def _swap_pt_eot(m):
    targeted = "target" in m.group(0).lower()
    return Modification(kind="swap_pt_typed", args=("until_end_of_turn",))


# "~ has base power and toughness N/N" (4)
@_eff(r"^(?:~|this creature)'?s? base power and toughness (?:is|are|becomes?) (\d+)/(\d+)(?:\.|$)")
def _set_base_pt(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False),
                duration="permanent")


# "target creature's base power and toughness become N/N until end of turn" (3)
@_eff(r"^target creature'?s? base power and toughness become (\d+)/(\d+) until end of turn(?:\.|$)")
def _set_target_base_pt(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=TARGET_CREATURE,
                duration="until_end_of_turn")


# "~ has power and toughness each equal to [X]" (5)
@_eff(r"^(?:~|this creature)'?s? power and toughness are each equal to (?:the number of|the total|its) (.+?)(?:\.|$)")
def _pt_equal_to(m):
    return Modification(kind="self_calculated_pt_typed", args=(m.group(1).strip(),))


# "~ has power equal to [X]" (3)
@_eff(r"^(?:~|this creature)'?s? power is equal to (?:the number of|its|the) (.+?)(?:\.|$)")
def _power_equal_to(m):
    return Modification(kind="self_calculated_pt_typed", args=("power", m.group(1).strip()))


# ============================================================================
# GROUP 15: Ward / tax on targeting effects
# ============================================================================
# "ward -- pay N life" (4)
@_eff(r"^ward\s*[-—]\s*(?:pay )?(\d+) life(?:\.|$)")
def _ward_life(m):
    return GrantAbility(
        ability_name=f"ward_life_{m.group(1)}",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "ward -- discard a card" (3)
@_eff(r"^ward\s*[-—]\s*discard a card(?:\.|$)")
def _ward_discard(m):
    return GrantAbility(
        ability_name="ward_discard",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# "ward -- sacrifice a [permanent]" (3)
@_eff(r"^ward\s*[-—]\s*sacrifice (?:a |an )?([a-z]+(?: [a-z]+)*)(?:\.|$)")
def _ward_sacrifice(m):
    return GrantAbility(
        ability_name=f"ward_sacrifice_{m.group(1).strip().replace(' ', '_')}",
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# ============================================================================
# GROUP 16: "At the beginning of your upkeep" trigger bodies
# ============================================================================
# "at the beginning of your upkeep, [draw/damage/life/counter]"
# These show up as orphan effect tails when the trigger wrapper parsed but
# the body didn't. Catch the common bodies.

# "at the beginning of your upkeep, you lose N life" (3)
@_eff(r"^at the beginning of (?:your|each player'?s?|each opponent'?s?) upkeep,?\s*(?:that player |you )?(draws?|loses?|gains?) (" + _NUM_RE + r") (?:cards?|life)(?:\.|$)")
def _upkeep_trigger_simple(m):
    verb = m.group(1).lower().rstrip("s")
    n = _n(m.group(2))
    text_low = m.group(0).lower()
    if "draw" in verb:
        return Draw(count=n, target=SELF)
    elif "lose" in verb:
        return LoseLife(amount=n, target=SELF)
    elif "gain" in verb:
        return GainLife(amount=n, target=SELF if "your" in text_low else EACH_PLAYER)
    return None


# "at the beginning of your end step, [effect]" (handled as trigger, catch body tails)
@_eff(r"^at the beginning of (?:your |the )(?:next )?end step,?\s*sacrifice (?:~|this creature|this permanent|it)(?:\.|$)")
def _end_step_sac_self(m):
    return Modification(kind="delayed_sacrifice_typed", args=("end_step",))


# "at the beginning of your end step, draw a card" (3)
@_eff(r"^at the beginning of (?:your|each player'?s?) end step,?\s*(?:you )?draw (?:a|" + _NUM_RE + r") cards?(?:\.|$)")
def _end_step_draw(m):
    return Draw(count=1, target=SELF)


# "at the beginning of your upkeep, put a +1/+1 counter on ~" (4)
@_eff(r"^at the beginning of your upkeep,?\s*put (?:a|" + _NUM_RE + r") ([+-]\d+/[+-]\d+|[a-z]+) counters? on (?:~|this creature|this permanent)(?:\.|$)")
def _upkeep_counter_self(m):
    n = _n(m.group(1)) if m.group(1) else 1
    return CounterMod(op="put", count=1, counter_kind=m.group(1) if m.group(1) else "+1/+1",
                      target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 17: Cycling trigger tails
# ============================================================================
# "when you cycle ~, [effect]" — tails that appear as bare effects
# "draw a card for each [thing]" (4)
@_eff(r"^draw (?:a card|cards?) for each (.+?) you control(?:\.|$)")
def _draw_for_each(m):
    return Draw(count="var", target=SELF)


# "draw a card for each card you've discarded this turn" (3)
@_eff(r"^draw (?:a card|cards?) for each (.+?) (?:you'?ve?|you have) (.+?)(?:\.|$)")
def _draw_for_each_verb(m):
    return Draw(count="var", target=SELF)


# ============================================================================
# GROUP 18: Planeswalker loyalty patterns
# ============================================================================
# "you may activate loyalty abilities of [planeswalkers/~] twice each turn" (3)
@_eff(r"^you may activate (?:loyalty abilities of |~'?s? loyalty abilities )(?:~|planeswalkers you control) (?:twice|an additional time) each turn(?:\.|$)")
def _loyalty_twice(m):
    return Modification(kind="loyalty_extra_activation_typed", args=())


# "~ can be your commander" (5)
@_eff(r"^(?:~|this card) can be your commander(?:\.|$)")
def _can_be_commander(m):
    return Modification(kind="can_be_commander_typed", args=())


# "~ enters with N loyalty counters" (3)
@_eff(r"^(?:~|this planeswalker) enters (?:the battlefield )?with (" + _NUM_RE + r") (?:loyalty|additional loyalty) counters?(?:\.|$)")
def _pw_enters_loyalty(m):
    n = _n(m.group(1))
    return CounterMod(op="put", count=n, counter_kind="loyalty",
                      target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 19: "Whenever you cast" / spellcast triggers as effects
# ============================================================================
# "whenever you cast a [type] spell, [simple effect]"
# These appear as effect text when the trigger was stripped but body wasn't matched.

# "copy that spell. you may choose new targets for the copy" (5)
@_eff(r"^copy (?:that spell|it|target instant or sorcery spell)(?:\.\s*|\s*,\s*)you may choose new targets for the copy(?:\.|$)")
def _copy_spell_new_targets(m):
    return CopySpell()


# "copy it. you may choose new targets" (3)
@_eff(r"^copy (?:it|that spell)(?:\.|$)")
def _copy_spell_bare(m):
    return CopySpell()


# "~ deals N damage to each opponent" (broad catch remaining forms) (5)
@_eff(r"^(?:~|this (?:creature|enchantment|permanent)) deals? (" + _NUM_RE + r") damage to each opponent(?:\.|$)")
def _self_deals_each_opp(m):
    n = _n(m.group(1))
    return Damage(amount=n, target=EACH_OPPONENT)


# ============================================================================
# GROUP 20: "When ~ dies" / death trigger effects
# ============================================================================
# "when ~ dies, return it to the battlefield under its owner's control" (4)
@_eff(r"^when (?:~|this creature) dies,?\s*return (?:it|~) to the battlefield under (?:its owner'?s?|your) control(?:\.|$)")
def _dies_return_to_bf(m):
    return Reanimate(query=Filter(base="self", targeted=False))


# "when ~ dies, you may put it into its owner's library [position]" (3)
@_eff(r"^when (?:~|this creature) dies,?\s*(?:you may )?(?:put|shuffle) (?:it|~) (?:into its owner'?s? library|on top of its owner'?s? library|on the bottom of its owner'?s? library)(?:\.|$)")
def _dies_tuck(m):
    return Modification(kind="death_tuck_typed", args=())


# "when ~ dies, create a N/N [color] [type] creature token" (5)
@_eff(r"^when (?:~|this creature) dies,?\s*create (?:" + _NUM_RE + r" )?(\d+)/(\d+) ([a-z]+(?: [a-z]+)*?) (?:creature )?tokens?(?:\.|$)")
def _dies_create_token(m):
    pt = (int(m.group(1)), int(m.group(2)))
    types = tuple(m.group(3).strip().split())
    return CreateToken(count=1, pt=pt, types=types)


# "when ~ dies, each opponent loses N life" (3)
@_eff(r"^when (?:~|this creature) dies,?\s*each opponent loses (" + _NUM_RE + r") life(?:\.|$)")
def _dies_drain(m):
    return LoseLife(amount=_n(m.group(1)), target=EACH_OPPONENT)


# "when ~ dies, draw a card" (4)
@_eff(r"^when (?:~|this creature|this enchantment) dies,?\s*(?:you )?draw (?:a|" + _NUM_RE + r") cards?(?:\.|$)")
def _dies_draw(m):
    return Draw(count=1, target=SELF)


# "when ~ dies, you gain N life" (3)
@_eff(r"^when (?:~|this creature) dies,?\s*(?:you )?gain (" + _NUM_RE + r") life(?:\.|$)")
def _dies_gain_life(m):
    return GainLife(amount=_n(m.group(1)))


# "when ~ dies, return target creature card from your graveyard to your hand" (3)
@_eff(r"^when (?:~|this creature) dies,?\s*return target ([a-z]+(?: [a-z]+)*) card from (?:your|a) graveyard to your hand(?:\.|$)")
def _dies_recurse(m):
    return Recurse(query=Filter(base=m.group(1), targeted=True), destination="hand")


# ============================================================================
# GROUP 21: "~ can't be regenerated" / exile on destroy
# ============================================================================
# "it can't be regenerated" (8)
@_eff(r"^(?:it|that creature|the creature|target creature|~) can'?t be regenerated(?:\.|$)")
def _cant_be_regenerated(m):
    return Modification(kind="cant_be_regenerated_typed", args=())


# "if a creature dealt damage this way would die this turn, exile it instead" (3)
@_eff(r"^if (?:a |that )?creature (?:dealt damage this way|damaged this way) would die (?:this turn)?,?\s*exile (?:it|that creature) instead(?:\.|$)")
def _exile_if_would_die(m):
    return Modification(kind="exile_on_death_typed", args=())


# "destroy target creature. it can't be regenerated" (4)
@_eff(r"^destroy target (creature|permanent)(?:\.\s*|\s*,\s*)(?:it|that creature|that permanent) can'?t be regenerated(?:\.|$)")
def _destroy_no_regen(m):
    return Destroy(target=Filter(base=m.group(1), targeted=True))


# ============================================================================
# GROUP 22: "Whenever a creature enters/dies" as effect tails
# ============================================================================
# "whenever a creature enters the battlefield under your control, [simple effect]"
# These appear when trigger wrappers successfully parsed but the body effect
# was left as an orphan.

# "that creature gets +N/+N until end of turn" (5)
@_eff(r"^that creature gets ([+-]\d+)/([+-]\d+) until end of turn(?:\.|$)")
def _that_creature_buff_eot(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="that_creature", targeted=False),
                duration="until_end_of_turn")


# "that creature gains [keyword] until end of turn" (4)
@_eff(r"^that creature gains (" + _KW_LIST + r") until end of turn(?:\.|$)")
def _that_creature_gains_kw(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="that_creature", targeted=False),
        duration="until_end_of_turn",
    )


# "put a +1/+1 counter on that creature" (5)
@_eff(r"^put (?:a|" + _NUM_RE + r") ([+-]\d+/[+-]\d+) counters? on (?:that creature|it)(?:\.|$)")
def _counter_on_that(m):
    n = 1
    return CounterMod(op="put", count=1, counter_kind=m.group(1),
                      target=Filter(base="that_creature", targeted=False))


# "you gain 1 life" (bare, appears as trigger tail) (6)
@_eff(r"^you gain (" + _NUM_RE + r") life(?:\.|$)")
def _you_gain_life_bare(m):
    return GainLife(amount=_n(m.group(1)))


# "you lose 1 life" (bare) (5)
@_eff(r"^you lose (" + _NUM_RE + r") life(?:\.|$)")
def _you_lose_life_bare(m):
    return LoseLife(amount=_n(m.group(1)), target=SELF)


# "you draw a card" (bare effect tail) (5)
@_eff(r"^you draw (?:a|" + _NUM_RE + r") cards?(?:\.|$)")
def _you_draw_bare(m):
    return Draw(count=1, target=SELF)


# "~ gets +1/+1 until end of turn" (bare self-buff tail) (8)
@_eff(r"^(?:~|this creature) gets ([+-]\d+)/([+-]\d+) until end of turn(?:\.|$)")
def _self_buff_eot(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False),
                duration="until_end_of_turn")


# ============================================================================
# GROUP 23: "As long as" static condition bodies
# ============================================================================
# "as long as you control [N or more] [type], ~ gets +N/+N" (5)
@_eff(r"^as long as you control (?:" + _NUM_RE + r" or more )?([a-z]+(?: [a-z]+)*),?\s*(?:~|this creature) gets ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _static_conditional_buff(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=Filter(base="self", targeted=False),
                duration="conditional")


# "as long as you control [N or more] [type], ~ has [keyword]" (4)
@_eff(r"^as long as you control (?:" + _NUM_RE + r" or more )?([a-z]+(?: [a-z]+)*),?\s*(?:~|this creature) has (" + _KW_LIST + r")(?:\.|$)")
def _static_conditional_kw(m):
    return GrantAbility(
        ability_name=m.group(2).strip(),
        target=Filter(base="self", targeted=False),
        duration="conditional",
    )


# "as long as it's your turn, ~ gets +N/+N" (3)
@_eff(r"^as long as it'?s? your turn,?\s*(?:~|this creature) gets ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _your_turn_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False),
                duration="conditional")


# "as long as it's your turn, ~ has [keyword]" (3)
@_eff(r"^as long as it'?s? your turn,?\s*(?:~|this creature) has (" + _KW_LIST + r")(?:\.|$)")
def _your_turn_kw(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="self", targeted=False),
        duration="conditional",
    )


# "as long as it's not your turn, ~ has [keyword]" (3)
@_eff(r"^as long as it'?s? not your turn,?\s*(?:~|this creature) has (" + _KW_LIST + r")(?:\.|$)")
def _not_your_turn_kw(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="self", targeted=False),
        duration="conditional",
    )


# "as long as ~ is enchanted/equipped, it gets +N/+N" (3)
@_eff(r"^as long as (?:~|this creature) is (?:enchanted|equipped),?\s*(?:~|it|this creature) gets ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _enchanted_self_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False),
                duration="conditional")


# ============================================================================
# GROUP 24: Goad target creature
# ============================================================================
# "goad target creature" (already partially in wave B, catch remaining forms)
@_eff(r"^goad (?:target creature|each creature (?:target|an) opponent controls|all creatures (?:target|an) opponent controls)(?:\.|$)")
def _goad_target(m):
    text_low = m.group(0).lower()
    if "each" in text_low or "all" in text_low:
        return Modification(kind="goad_all_typed", args=())
    return Modification(kind="goad_typed", args=())


# "goad each creature your opponents control" (3)
@_eff(r"^goad each creature (?:your opponents?|each opponent) controls?(?:\.|$)")
def _goad_all_opp(m):
    return Modification(kind="goad_all_typed", args=())


# ============================================================================
# GROUP 25: "Aftermath" / "fuse" / split card indicators
# ============================================================================
# "aftermath" (bare keyword) (12)
@_eff(r"^aftermath(?:\.|$)")
def _aftermath(m):
    return Modification(kind="aftermath_typed", args=())


# "fuse" (bare keyword) (5)
@_eff(r"^fuse(?:\.|$)")
def _fuse(m):
    return Modification(kind="fuse_typed", args=())


# "overload" (bare keyword without cost -- already handled with cost in wave C)
# Skip -- covered in wave C.

# ============================================================================
# GROUP 26: Hideaway
# ============================================================================
# "hideaway N" (6)
@_eff(r"^hideaway (\d+)(?:\.|$)")
def _hideaway(m):
    return Modification(kind="hideaway_typed", args=(int(m.group(1)),))


# "hideaway" (bare, old templating without number) (3)
@_eff(r"^hideaway(?:\.|$)")
def _hideaway_bare(m):
    return Modification(kind="hideaway_typed", args=(4,))  # default 4


# ============================================================================
# GROUP 27: "Exile the top N cards of your library" (separate from look-at)
# ============================================================================
# "exile the top N cards of your library" (10)
@_eff(r"^exile the top (" + _NUM_RE + r") cards? of your library(?:\.|$)")
def _exile_top_n(m):
    n = _n(m.group(1))
    return Exile(target=Filter(base="library_top", count=n))


# "exile the top card of your library" (5)
@_eff(r"^exile the top card of your library(?:\.|$)")
def _exile_top_one(m):
    return Exile(target=Filter(base="library_top", count=1))


# "exile the top N cards of target player's library" (3)
@_eff(r"^exile the top (" + _NUM_RE + r") cards? of target (?:player|opponent)'?s? library(?:\.|$)")
def _exile_top_n_target(m):
    n = _n(m.group(1))
    who = TARGET_OPPONENT if "opponent" in m.group(0).lower() else TARGET_PLAYER
    return Exile(target=Filter(base="library_top", count=n))


# ============================================================================
# GROUP 28: "Detain" target creature
# ============================================================================
# "detain target creature an opponent controls" (already in wave B)
# catch remaining: "detain each creature your opponents control"
@_eff(r"^detain each creature (?:your opponents?|an opponent) controls?(?:\.|$)")
def _detain_all_opp(m):
    return Modification(kind="detain_all_typed", args=())


# ============================================================================
# GROUP 29: Suspect / ring-bearer / designated mechanics
# ============================================================================
# "suspect target creature" (3)
@_eff(r"^suspect target creature(?:\.|$)")
def _suspect_target(m):
    return Modification(kind="suspect_typed", args=())


# "suspect ~" (3)
@_eff(r"^suspect (?:~|this creature)(?:\.|$)")
def _suspect_self(m):
    return Modification(kind="suspect_typed", args=("self",))


# "choose target creature you control as the ring-bearer" (3)
@_eff(r"^(?:choose target creature you control as|the ring tempts you)(?:\.|$)")
def _ring_tempts(m):
    return Modification(kind="ring_tempts_typed", args=())


# "the ring tempts you" (5)
@_eff(r"^the ring tempts you(?:\.|$)")
def _ring_tempts_bare(m):
    return Modification(kind="ring_tempts_typed", args=())


# ============================================================================
# GROUP 30: "Whenever a creature enters" / ETB trigger bodies
# ============================================================================
# "whenever another creature enters the battlefield under your control, [effect]"
# body tails that appeared as orphans

# "each opponent loses 1 life and you gain 1 life" (soul sisters variant) (3)
@_eff(r"^each opponent loses (" + _NUM_RE + r") life and you gain (" + _NUM_RE + r") life(?:\.|$)")
def _drain_and_gain(m):
    return Sequence(items=(
        LoseLife(amount=_n(m.group(1)), target=EACH_OPPONENT),
        GainLife(amount=_n(m.group(2))),
    ))


# "target opponent discards a card at random" (3)
@_eff(r"^target opponent discards a card at random(?:\.|$)")
def _opp_discard_random(m):
    return Discard(count=1, target=TARGET_OPPONENT)


# ============================================================================
# GROUP 31: Explore as effect
# ============================================================================
# "~ explores" (8)
@_eff(r"^(?:~|this creature|it|target creature you control) explores(?:\.|$)")
def _explore(m):
    return Modification(kind="explore_typed", args=())


# "target creature you control explores" (3)
@_eff(r"^target creature (?:you control )?explores (?:twice|" + _NUM_RE + r" times)(?:\.|$)")
def _explore_n(m):
    return Modification(kind="explore_typed", args=("multiple",))


# ============================================================================
# GROUP 32: Proliferate (broader)
# ============================================================================
# "proliferate" (already in wave B, but catch "then proliferate" sequences)
@_eff(r"^(?:then )?proliferate(?:\.|$)")
def _proliferate_then(m):
    return Modification(kind="proliferate_typed", args=())


# ============================================================================
# GROUP 33: "Whenever ~ deals combat damage to a player" effect bodies
# ============================================================================
# "that player discards a card" (effect body, not trigger) (4)
@_eff(r"^that player discards (" + _NUM_RE + r") cards?(?:\.|$)")
def _that_player_discards_wc(m):
    n = _n(m.group(1))
    return Discard(count=n, target=Filter(base="that_player", targeted=False))


# "draw cards equal to the damage dealt" (3)
@_eff(r"^draw cards? equal to (?:the damage dealt|that much damage|the number of cards? put into [^.]+)(?:\.|$)")
def _draw_equal_damage(m):
    return Draw(count="var", target=SELF)


# ============================================================================
# GROUP 34: "Become the monarch" / initiative
# ============================================================================
# "you become the monarch" (already in wave B as Modification)
# "take the initiative" (3)
@_eff(r"^take the initiative(?:\.|$)")
def _take_initiative(m):
    return Modification(kind="take_initiative_typed", args=())


# "you become the monarch" (catch remaining forms) (3)
@_eff(r"^you become the monarch(?:\.|$)")
def _become_monarch(m):
    return Modification(kind="become_monarch_typed", args=())


# ============================================================================
# GROUP 35: "Each player" sacrifice/discard patterns
# ============================================================================
# "each player sacrifices a creature" (4)
@_eff(r"^each player sacrifices? (?:a|an) (creature|permanent|land|artifact|enchantment|nonland permanent)(?:\.|$)")
def _each_sac(m):
    return Sacrifice(query=Filter(base=m.group(1)),
                     actor="each_player")


# "each opponent sacrifices a creature" (5)
@_eff(r"^each opponent sacrifices? (?:a|an) (creature|permanent|land|artifact|enchantment|nonland permanent)(?:\.|$)")
def _each_opp_sac(m):
    return Sacrifice(query=Filter(base=m.group(1)),
                     actor="each_opponent")


# "each opponent sacrifices N creatures" (3)
@_eff(r"^each opponent sacrifices? (" + _NUM_RE + r") (creatures?|permanents?|lands?)(?:\.|$)")
def _each_opp_sac_n(m):
    n = _n(m.group(1))
    base = m.group(2).lower().rstrip("s")
    return Sacrifice(query=Filter(base=base), actor="each_opponent")


# "each player discards N cards" (3)
@_eff(r"^each player discards? (" + _NUM_RE + r") cards?(?:\.|$)")
def _each_player_discards(m):
    n = _n(m.group(1))
    return Discard(count=n, target=EACH_PLAYER)


# "each opponent discards a card" (4)
@_eff(r"^each opponent discards? (?:a|" + _NUM_RE + r") cards?(?:\.|$)")
def _each_opp_discards(m):
    return Discard(count=1, target=EACH_OPPONENT)


# "each player discards their hand" (3)
@_eff(r"^each player discards? (?:their|his or her) hand(?:\.|$)")
def _each_player_discards_hand(m):
    return Discard(count="all", target=EACH_PLAYER)


# "each player discards their hand, then draws N cards" (3)
@_eff(r"^each player discards? (?:their|his or her) hand,?\s*then draws? (" + _NUM_RE + r") cards?(?:\.|$)")
def _wheel(m):
    n = _n(m.group(1))
    return Sequence(items=(
        Discard(count="all", target=EACH_PLAYER),
        Draw(count=n, target=EACH_PLAYER),
    ))


# ============================================================================
# GROUP 36: Additional mana production
# ============================================================================
# "add one mana of any color" (6)
@_eff(r"^add (?:one|" + _NUM_RE + r") mana of any color(?:\.|$)")
def _add_any_color(m):
    return AddMana(any_color_count=1)


# "add {C}" (bare colorless mana) (4)
@_eff(r"^add \{c\}(?:\{c\})*(?:\.|$)")
def _add_colorless(m):
    count = m.group(0).lower().count("{c}")
    pool = tuple(ManaSymbol(raw="{C}", color=("C",)) for _ in range(count))
    return AddMana(pool=pool)


# "add two mana of any one color" (3)
@_eff(r"^add (" + _NUM_RE + r") mana of any one color(?:\.|$)")
def _add_n_one_color(m):
    n = _n(m.group(1))
    return AddMana(any_color_count=n)


# "add two mana in any combination of colors" (3)
@_eff(r"^add (" + _NUM_RE + r") mana in any combination of colors(?:\.|$)")
def _add_n_any_combo(m):
    n = _n(m.group(1))
    return AddMana(any_color_count=n)


# "add {color}" (single colored mana) (5)
@_eff(r"^add \{([wubrgc])\}(?:\.|$)")
def _add_single_color(m):
    c = m.group(1).upper()
    return AddMana(pool=(ManaSymbol(raw="{" + c + "}", color=(c,)),))


# "add {color}{color}" / two of same (3)
@_eff(r"^add \{([wubrgc])\}\{([wubrgc])\}(?:\.|$)")
def _add_two_color(m):
    c1, c2 = m.group(1).upper(), m.group(2).upper()
    return AddMana(pool=(
        ManaSymbol(raw="{" + c1 + "}", color=(c1,)),
        ManaSymbol(raw="{" + c2 + "}", color=(c2,)),
    ))


# ============================================================================
# GROUP 37: "Discard a card, then draw a card" / loot as effect
# ============================================================================
# "discard a card, then draw a card" (rummage) (4)
@_eff(r"^discard (" + _NUM_RE + r") cards?,?\s*then draw (" + _NUM_RE + r") cards?(?:\.|$)")
def _rummage_then_draw(m):
    d = _n(m.group(1))
    r = _n(m.group(2))
    return Sequence(items=(
        Discard(count=d, target=SELF),
        Draw(count=r, target=SELF),
    ))


# "draw a card, then discard a card" (loot) (4)
@_eff(r"^draw (" + _NUM_RE + r") cards?,?\s*then discard (" + _NUM_RE + r") cards?(?:\.|$)")
def _loot_draw_discard(m):
    d_count = _n(m.group(1))
    disc = _n(m.group(2))
    return Sequence(items=(
        Draw(count=d_count, target=SELF),
        Discard(count=disc, target=SELF),
    ))


# ============================================================================
# GROUP 38: "Target creature fights another target creature"
# ============================================================================
# "target creature you control fights target creature you don't control" (5)
@_eff(r"^target creature (?:you control )?fights? (?:another )?target creature(?:\s+(?:you don'?t control|an opponent controls))?(?:\.|$)")
def _fight_targets(m):
    return Fight()


# "~ fights target creature" (4)
@_eff(r"^(?:~|this creature|it) fights? target creature(?:\.|$)")
def _self_fights_target(m):
    return Fight()


# "target creature you control deals damage equal to its power to target creature you don't control" (3)
@_eff(r"^target creature you control deals damage equal to its power to target creature (?:you don'?t control|an opponent controls)(?:\.|$)")
def _bite(m):
    return Damage(amount="power", target=TARGET_CREATURE)


# ============================================================================
# GROUP 39: "Put a creature card from your hand onto the battlefield" (cheat in)
# ============================================================================
# "you may put a creature card from your hand onto the battlefield" (6)
@_eff(r"^(?:you may )?put (?:a|an) (creature|artifact|enchantment|land|permanent) card from your hand onto the battlefield(?:\s+tapped)?(?:\.|$)")
def _cheat_in_from_hand(m):
    tapped = "tapped" in m.group(0).lower()
    dest = "battlefield_tapped" if tapped else "battlefield"
    return Reanimate(query=Filter(base=m.group(1) + "_card", extra=("from_hand",)))


# "put all creature cards from your hand onto the battlefield" (3)
@_eff(r"^put all (creature|artifact|land) cards from your hand onto the battlefield(?:\.|$)")
def _cheat_all_from_hand(m):
    return Reanimate(query=Filter(base=m.group(1) + "_card", quantifier="all", extra=("from_hand",)))


# ============================================================================
# GROUP 40: Target creature/player gains ability text
# ============================================================================
# "target creature gains [keyword list] until end of turn" (compound keywords)
@_eff(r"^target creature gains (" + _KW_LIST + r") and (" + _KW_LIST + r") until end of turn(?:\.|$)")
def _target_gains_two_kw(m):
    return Sequence(items=(
        GrantAbility(ability_name=m.group(1).strip(), target=TARGET_CREATURE, duration="until_end_of_turn"),
        GrantAbility(ability_name=m.group(2).strip(), target=TARGET_CREATURE, duration="until_end_of_turn"),
    ))


# "~ gains [keyword] until end of turn" (self, bare) (6)
@_eff(r"^(?:~|this creature) gains (" + _KW_LIST + r") until end of turn(?:\.|$)")
def _self_gains_kw_eot(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="self", targeted=False),
        duration="until_end_of_turn",
    )


# "~ has [keyword]" (static self-grant) (5)
@_eff(r"^(?:~|this creature) has (" + _KW_LIST + r")(?:\.|$)")
def _self_has_kw(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="self", targeted=False),
        duration="permanent",
    )


# ============================================================================
# GROUP 41: "Exile ~, then return it transformed" / DFC triggers
# ============================================================================
# "exile ~, then return it to the battlefield transformed under your control" (already in wave C saga version, catch creature DFC)
@_eff(r"^exile (?:~|this creature|this permanent),?\s*then return (?:it|~) to the battlefield transformed(?:\s+under (?:its owner'?s?|your) control)?(?:\.|$)")
def _exile_return_transformed(m):
    return Sequence(items=(
        Exile(target=Filter(base="self", targeted=False)),
        Reanimate(query=Filter(base="self", targeted=False)),
    ))


# "transform ~" (bare, non-conditional) (5)
@_eff(r"^transform (?:~|this creature|this permanent)(?:\.|$)")
def _transform_bare(m):
    return Modification(kind="transform_self_typed", args=())


# ============================================================================
# GROUP 42: "Whenever you gain life" trigger bodies
# ============================================================================
# "put a +1/+1 counter on ~" (very common trigger tail) (10)
@_eff(r"^put (?:a|" + _NUM_RE + r") ([+-]\d+/[+-]\d+) counters? on (?:~|this creature|this permanent)(?:\.|$)")
def _counter_on_self(m):
    return CounterMod(op="put", count=1, counter_kind=m.group(1),
                      target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 43: "Prevent all damage that would be dealt to ~" (static)
# ============================================================================
# "prevent all damage that would be dealt to ~" (3)
@_eff(r"^prevent all (?:combat )?damage that would be dealt to (?:~|this creature|this permanent)(?: this turn)?(?:\.|$)")
def _prevent_damage_to_self(m):
    return Prevent(amount="all")


# "prevent all combat damage" (broad form) (3)
@_eff(r"^prevent all combat damage(?:\.|$)")
def _prevent_all_combat(m):
    return Prevent(amount="all", damage_filter=Filter(base="combat_damage"))


# ============================================================================
# GROUP 44: "Return ~ to its owner's hand" (self-bounce as effect)
# ============================================================================
# "return ~ to its owner's hand" (4)
@_eff(r"^return (?:~|this creature|this permanent|this enchantment) to its owner'?s? hand(?:\.|$)")
def _self_bounce(m):
    return Bounce(target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 45: Phyrexian / "Compleated" / Toxic / Corrupted
# ============================================================================
# "toxic N" (6)
@_eff(r"^toxic (\d+)(?:\.|$)")
def _toxic(m):
    return Modification(kind="toxic_typed", args=(int(m.group(1)),))


# "teamwork N" (CR §702.194, Aug 7 2026) — mirrors toxic N
@_eff(r"^teamwork (\d+)(?:\.|$)")
def _teamwork(m):
    return Modification(kind="teamwork_typed", args=(int(m.group(1)),))


# "corrupted -- [effect]" (ability word body) (3)
@_eff(r"^corrupted\s*[-—]\s*(.+?)(?:\.|$)")
def _corrupted(m):
    return Modification(kind="corrupted_typed", args=(m.group(1).strip(),))


# "compleated" (bare keyword modifier) (3)
@_eff(r"^compleated(?:\.|$)")
def _compleated(m):
    return Modification(kind="compleated_typed", args=())


# "for mirrodin!" (3)
@_eff(r"^for mirrodin!?(?:\.|$)")
def _for_mirrodin(m):
    return Modification(kind="for_mirrodin_typed", args=())


# ============================================================================
# GROUP 46: "Ingest" / "Devoid" / Processor exile interaction
# ============================================================================
# "devoid" (9)
@_eff(r"^devoid(?:\.|$)")
def _devoid(m):
    return Modification(kind="devoid_typed", args=())


# "ingest" (3)
@_eff(r"^ingest(?:\.|$)")
def _ingest(m):
    return Modification(kind="ingest_typed", args=())


# ============================================================================
# GROUP 47: "Create a copy of target creature" (remaining forms)
# ============================================================================
# "create N tokens that are copies of target creature" (3)
@_eff(r"^create (" + _NUM_RE + r") tokens? that (?:are|is) (?:each )?(?:a )?cop(?:y|ies) of (?:target (creature|permanent|artifact)|it|~|another creature you control)(?:\.|$)")
def _create_n_copies(m):
    n = _n(m.group(1))
    text_low = m.group(0).lower()
    if "another" in text_low:
        target = Filter(base="creature", you_control=True, extra=("other",))
    elif m.group(2):
        target = Filter(base=m.group(2), targeted=True)
    else:
        target = Filter(base="creature", targeted=False)
    return CopyPermanent(target=target)


# ============================================================================
# GROUP 48: "Sacrifice a creature" as cost-like effects
# ============================================================================
# "sacrifice a creature" (bare, as trigger body) (5)
@_eff(r"^sacrifice (?:a|an|another) (creature|permanent|artifact|enchantment|land|token)(?:\.|$)")
def _sac_bare(m):
    return Sacrifice(query=Filter(base=m.group(1), you_control=True))


# "sacrifice N creatures" (3)
@_eff(r"^sacrifice (" + _NUM_RE + r") (creatures?|permanents?|artifacts?|lands?)(?:\.|$)")
def _sac_n(m):
    n = _n(m.group(1))
    base = m.group(2).lower().rstrip("s")
    return Sacrifice(query=Filter(base=base, you_control=True))


# ============================================================================
# GROUP 49: "Reveal" as bare effect
# ============================================================================
# "reveal your hand" (5)
@_eff(r"^reveal your hand(?:\.|$)")
def _reveal_hand(m):
    return Reveal(source="your_hand")


# "reveal the top card of your library" (bare, no follow-up) (4)
@_eff(r"^reveal the top (" + _NUM_RE + r" )?cards? of your library(?:\.|$)")
def _reveal_top(m):
    n = _n(m.group(1)) if m.group(1) else 1
    return Reveal(source="top_of_library", count=n)


# "target player reveals their hand" (3)
@_eff(r"^target (?:player|opponent) reveals? their hand(?:\.|$)")
def _target_reveals_hand(m):
    who = TARGET_OPPONENT if "opponent" in m.group(0).lower() else TARGET_PLAYER
    return Reveal(source="target_hand")


# ============================================================================
# GROUP 50: "Tap an untapped creature you control" / convoke-like
# ============================================================================
# "tap an untapped creature you control" (5)
@_eff(r"^tap (?:an|another) untapped (creature|permanent|artifact) you control(?:\.|$)")
def _tap_untapped_you_ctrl(m):
    return TapEffect(target=Filter(base=m.group(1), you_control=True, targeted=False))


# "tap target creature" (already covered, catch "tap target artifact")
@_eff(r"^tap target (artifact|land|permanent)(?:\.|$)")
def _tap_target_noncreature(m):
    return TapEffect(target=Filter(base=m.group(1), targeted=True))


# "tap all creatures target player controls" (3)
@_eff(r"^tap all (creatures|artifacts|nonland permanents) (?:target (?:player|opponent)|an opponent|your opponents?) controls?(?:\.|$)")
def _tap_all_opp(m):
    base = m.group(1).lower().rstrip("s")
    return TapEffect(target=Filter(base=base, quantifier="all"))


# ============================================================================
# GROUP 51: "Until end of turn" / duration modifiers as standalone
# ============================================================================
# "until end of turn, target creature gains [keyword]" (reordered phrasing) (4)
@_eff(r"^until end of turn,?\s*target (creature|permanent) gains? (" + _KW_LIST + r")(?:\.|$)")
def _until_eot_target_gains(m):
    return GrantAbility(
        ability_name=m.group(2).strip(),
        target=Filter(base=m.group(1), targeted=True),
        duration="until_end_of_turn",
    )


# "until end of turn, ~ gets +N/+N" (reordered phrasing) (3)
@_eff(r"^until end of turn,?\s*(?:~|this creature) gets ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _until_eot_self_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False),
                duration="until_end_of_turn")


# "until end of turn, creatures you control get +N/+N" (3)
@_eff(r"^until end of turn,?\s*creatures you control get ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _until_eot_your_creatures_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", quantifier="all", you_control=True),
                duration="until_end_of_turn")


# ============================================================================
# GROUP 52: "Whenever you discard a card" / discard trigger bodies
# ============================================================================
# "~ deals 2 damage to any target" (combat trigger tail, very common) (5)
@_eff(r"^(?:~|this creature|this enchantment|this artifact) deals (" + _NUM_RE + r") damage to (?:any target|target creature or player)(?:\.|$)")
def _self_deals_any_broad(m):
    n = _n(m.group(1))
    return Damage(amount=n, target=TARGET_ANY)


# ============================================================================
# GROUP 53: "Nontoken creatures you control" / scoped anthems
# ============================================================================
# "nontoken creatures you control get +N/+N" (4)
@_eff(r"^nontoken creatures you control get ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _nontoken_anthem(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", quantifier="all", you_control=True, extra=("nontoken",)),
                duration="permanent")


# "nontoken creatures you control have [keyword]" (3)
@_eff(r"^nontoken creatures you control have (" + _KW_LIST + r")(?:\.|$)")
def _nontoken_grant_kw(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="creature", quantifier="all", you_control=True, extra=("nontoken",)),
        duration="permanent",
    )


# ============================================================================
# GROUP 54: "Discard a card at random" / random discard
# ============================================================================
# "discard a card at random" (4)
@_eff(r"^discard a card at random(?:\.|$)")
def _discard_random(m):
    return Discard(count=1, target=SELF)


# "target player discards a card at random" (3)
@_eff(r"^target (?:player|opponent) discards? a card at random(?:\.|$)")
def _target_discard_random(m):
    who = TARGET_OPPONENT if "opponent" in m.group(0).lower() else TARGET_PLAYER
    return Discard(count=1, target=who)


# ============================================================================
# GROUP 55: Additional create token patterns (with keywords)
# ============================================================================
# "create a N/N [color] [type] creature token with [keyword]" (8)
@_eff(r"^create (?:" + _NUM_RE + r" )?(\d+)/(\d+) ([a-z]+(?: [a-z]+)*?) creature tokens? with (" + _KW_LIST + r")(?:\.|$)")
def _create_token_with_kw(m):
    pt = (int(m.group(1)), int(m.group(2)))
    types = tuple(m.group(3).strip().split())
    return CreateToken(count=1, pt=pt, types=types)


# "create two 1/1 white soldier creature tokens" (broader form) (5)
@_eff(r"^create (" + _NUM_RE + r") (\d+)/(\d+) ([a-z]+(?: [a-z]+)*?) creature tokens?(?:\.|$)")
def _create_n_tokens_broad(m):
    n = _n(m.group(1))
    pt = (int(m.group(2)), int(m.group(3)))
    types = tuple(m.group(4).strip().split())
    return CreateToken(count=n, pt=pt, types=types)


# "create a tapped N/N [type] creature token" (3)
@_eff(r"^create (?:" + _NUM_RE + r" )?(?:a )?tapped (\d+)/(\d+) ([a-z]+(?: [a-z]+)*?) creature tokens?(?:\.|$)")
def _create_tapped_token(m):
    pt = (int(m.group(1)), int(m.group(2)))
    types = tuple(m.group(3).strip().split())
    return CreateToken(count=1, pt=pt, types=types)


# ============================================================================
# GROUP 56: "Commander ninjutsu" / "commander tax" patterns
# ============================================================================
# "commander ninjutsu {cost}" (3)
@_eff(r"^commander ninjutsu (\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _commander_ninjutsu(m):
    return Modification(kind="commander_ninjutsu_typed", args=(m.group(1),))


# "eminence" (ability word prefix) (3)
@_eff(r"^eminence\s*[-—]\s*(.+?)(?:\.|$)")
def _eminence(m):
    return Modification(kind="eminence_typed", args=(m.group(1).strip(),))


# ============================================================================
# GROUP 57: "Cipher" / "encode" mechanics
# ============================================================================
# "cipher" (3)
@_eff(r"^cipher(?:\.|$)")
def _cipher(m):
    return Modification(kind="cipher_typed", args=())


# "encode" (related form) (2)
@_eff(r"^encode(?:\.|$)")
def _encode(m):
    return Modification(kind="encode_typed", args=())


# ============================================================================
# GROUP 58: Connive / collect evidence / investigate / cloak
# ============================================================================
# "investigate" (bare, create clue) (4)
@_eff(r"^investigate(?:\.|$)")
def _investigate_bare(m):
    return CreateToken(count=1, types=("Clue",))


# "collect evidence N" (3)
@_eff(r"^collect evidence (\d+)(?:\.|$)")
def _collect_evidence(m):
    return Modification(kind="collect_evidence_typed", args=(int(m.group(1)),))


# "cloak the top card of your library" (3)
@_eff(r"^cloak the top (?:card|" + _NUM_RE + r" cards?) of your library(?:\.|$)")
def _cloak(m):
    return Modification(kind="cloak_typed", args=())


# ============================================================================
# GROUP 59: "Equipped creature has / gets" with scoped filters
# ============================================================================
# "equipped creature gets +X/+X, where X is [variable]" (3)
@_eff(r"^equipped creature gets ([+-]x)/([+-]x)(?:\s*,?\s*where x is .+)?(?:\.|$)")
def _equipped_var_buff(m):
    return Buff(power="var", toughness="var",
                target=Filter(base="equipped_creature", targeted=False),
                duration="permanent")


# ============================================================================
# GROUP 60: "Amass [type] N" (new Amass variant from MOM)
# ============================================================================
# "amass orcs N" / "amass zombies N" (3)
@_eff(r"^amass (?:orcs?|zombies?) (\d+)(?:\.|$)")
def _amass_typed(m):
    n = int(m.group(1))
    return Modification(kind="amass_typed", args=(n,))


# "amass N" (bare, already in wave B but catch remaining) (3)
@_eff(r"^amass (\d+)(?:\.|$)")
def _amass_bare(m):
    return Modification(kind="amass_typed", args=(int(m.group(1)),))


# ============================================================================
# GROUP 61: "When ~ enters, [simple effect]" ETB as effect
# ============================================================================
# "when ~ enters the battlefield, [draw/damage/counter/etc]"
# These have the trigger already parsed but the body fell through.

# "when ~ enters the battlefield, each opponent loses N life" (3)
@_eff(r"^when (?:~|this creature|this permanent) enters (?:the battlefield)?,?\s*each opponent loses (" + _NUM_RE + r") life(?:\.|$)")
def _etb_drain(m):
    return LoseLife(amount=_n(m.group(1)), target=EACH_OPPONENT)


# "when ~ enters the battlefield, draw a card" (already handled broadly)
# skip -- covered

# "when ~ enters the battlefield, destroy target [type]" (3)
@_eff(r"^when (?:~|this creature|this permanent) enters (?:the battlefield)?,?\s*destroy target (creature|artifact|enchantment|permanent|artifact or enchantment|nonland permanent)(?:\.|$)")
def _etb_destroy(m):
    return Destroy(target=Filter(base=m.group(1), targeted=True))


# "when ~ enters the battlefield, exile target [type]" (3)
@_eff(r"^when (?:~|this creature|this permanent) enters (?:the battlefield)?,?\s*exile target (creature|permanent|nonland permanent|artifact|enchantment)(?:\.|$)")
def _etb_exile(m):
    return Exile(target=Filter(base=m.group(1), targeted=True))


# "when ~ enters the battlefield, return target [type] to its owner's hand" (3)
@_eff(r"^when (?:~|this creature|this permanent) enters (?:the battlefield)?,?\s*return target (creature|nonland permanent|permanent|artifact|enchantment) to its owner'?s? hand(?:\.|$)")
def _etb_bounce(m):
    return Bounce(target=Filter(base=m.group(1), targeted=True))


# ============================================================================
# GROUP 62: "Sacrifice ~ at end of turn" delayed effects
# ============================================================================
# "exile ~ at the beginning of the next end step" (3)
@_eff(r"^exile (?:~|this creature|this permanent|it) at the beginning of the next (?:end step|cleanup step)(?:\.|$)")
def _exile_at_end_step(m):
    return Modification(kind="delayed_exile_typed", args=("next_end_step",))


# "return ~ to its owner's hand at the beginning of the next end step" (3)
@_eff(r"^return (?:~|this creature|it) to its owner'?s? hand at the beginning of the next end step(?:\.|$)")
def _bounce_at_end_step(m):
    return Modification(kind="delayed_bounce_typed", args=("next_end_step",))


# ============================================================================
# GROUP 63: "Regenerate" / "regenerate target creature"
# ============================================================================
# "regenerate ~" (bare) (3)
@_eff(r"^regenerate (?:~|this creature|this permanent)(?:\.|$)")
def _regenerate_self_bare(m):
    return Modification(kind="regenerate_typed", args=())


# "regenerate target creature" (3)
@_eff(r"^regenerate target (creature|permanent)(?:\.|$)")
def _regenerate_target(m):
    return Modification(kind="regenerate_target_typed", args=(m.group(1),))


# ============================================================================
# GROUP 64: "Search your library" remaining forms
# ============================================================================
# "search your library for a card, put that card into your hand, then shuffle" (3)
@_eff(r"^search your library for a card,?\s*(?:reveal it,?\s*)?put (?:that card|it) into your hand,?\s*then shuffle(?:\.|$)")
def _demonic_tutor(m):
    return Tutor(query=Filter(base="card", targeted=False), destination="hand", shuffle_after=True)


# "search your library for a card and put that card on top of your library, then shuffle" (3)
@_eff(r"^search your library for a card (?:and put|,?\s*put) (?:that card|it) on top of your library,?\s*then shuffle(?:\.|$)")
def _vampiric_tutor(m):
    return Tutor(query=Filter(base="card", targeted=False), destination="top_of_library", shuffle_after=True)


# "search your library and/or graveyard for a card named [name]" (3)
@_eff(r"^search your library (?:and/or graveyard |or graveyard )?for (?:a card|an? [a-z]+(?: [a-z]+)* card) named (.+?)(?:,|\.|\s+and\s)(?:.*)?(?:\.|$)")
def _tutor_named(m):
    return Tutor(query=Filter(base="card", targeted=False), destination="hand", shuffle_after=True)


# ============================================================================
# GROUP 65: "Stifle" / "counter target triggered ability"
# ============================================================================
# "exile target creature or planeswalker with mana value N or less" (3)
@_eff(r"^exile target (creature or planeswalker|creature|permanent|nonland permanent) with (?:mana value|converted mana cost) (\d+) or less(?:\.|$)")
def _exile_mv_or_less(m):
    return Exile(target=Filter(base=m.group(1), targeted=True, extra=(f"mv_le_{m.group(2)}",)))


# "exile target creature with power N or less" (3)
@_eff(r"^exile target creature with power (\d+) or less(?:\.|$)")
def _exile_power_or_less(m):
    return Exile(target=Filter(base="creature", targeted=True, extra=(f"power_le_{m.group(1)}",)))


# ============================================================================
# Final: all handlers registered via module-level decorator side effects.
# ============================================================================
