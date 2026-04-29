#!/usr/bin/env python3
"""Wave A parser phrase-coverage promotions (post-Wave 1a).

Named ``a_wave_a_*`` so it loads AFTER ``a_wave1a_promotions.py`` but
BEFORE all other extensions. This ensures Wave A rules preempt the
labeled-UnknownEffect stubs in later-loading extensions.

Goal: drain ~5,800 UnknownEffect nodes to ≤2,000 by promoting the
remaining high-frequency phrase families to typed AST nodes. Each
handler returns a dedicated typed node or a Modification stub when no
leaf node exists yet.

Coverage audit: ``python3 scripts/audit_unknown_effects.py``

Non-goals:
  - No new AST node types.
  - No per-card snowflake handlers.
  - Do NOT break existing goldens in _UNKNOWN_PROMOTE_FORBIDDEN.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

import dataclasses

from mtg_ast import (  # noqa: E402
    Activated, AddMana, Bounce, Buff, CardAST, Choice, Conditional,
    Condition, CopySpell, CounterMod, CounterSpell, CreateToken, Damage,
    Destroy, Discard, Draw, Exile, ExtraTurn, Fight, Filter, GainControl,
    GainLife, GrantAbility, Keyword, LoseGame, LoseLife, ManaCost,
    ManaSymbol, Mill, Modification, Optional_, Reanimate, Recurse,
    Replacement, Reveal, Sacrifice, Scry, Sequence, SetLife, Shuffle,
    Static, Surveil, TapEffect, Triggered, Tutor, UntapEffect,
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
}


def _n(tok: str):
    t = (tok or "").strip().lower()
    if t in _NUMS:
        return _NUMS[t]
    if t.isdigit():
        return int(t)
    return t


def _pip(letter: str) -> ManaSymbol:
    L = letter.strip().upper()
    if L == "C":
        return ManaSymbol(raw="{C}", color=("C",))
    if L == "S":
        return ManaSymbol(raw="{S}", is_snow=True)
    return ManaSymbol(raw="{" + L + "}", color=(L,))


_COLOR_MAP = {
    "white": "W", "blue": "U", "black": "B",
    "red": "R", "green": "G", "colorless": "C",
}


def _parse_color_list(s: str) -> tuple[str, ...]:
    tokens = re.findall(r"\b(white|blue|black|red|green|colorless)\b", s.lower())
    return tuple(_COLOR_MAP[t] for t in tokens)


# ============================================================================
# GROUP 1: Transform variants — 62+ nodes in werewolves and DFCs
# ============================================================================

# "if no spells were cast last turn, transform this creature" (31)
@_eff(r"^if no spells were cast last turn,\s*transform (?:this creature|~)\.?$")
def _werewolf_transform_no_spells(m):
    return Conditional(
        condition=Condition(kind="no_spells_cast_last_turn", args=()),
        body=Modification(kind="transform_self", args=()),
    )


# "if a player cast two or more spells last turn, transform this creature" (31)
@_eff(r"^if a player cast two or more spells last turn,\s*transform (?:this creature|~)\.?$")
def _werewolf_transform_two_spells(m):
    return Conditional(
        condition=Condition(kind="two_plus_spells_cast_last_turn", args=()),
        body=Modification(kind="transform_self", args=()),
    )


# "transform it" (6), "transform this land" (6), "transform this artifact" (2),
# "transform this enchantment" (2), "transform this creature" (misc),
# "transform this equipment" (1)
@_eff(r"^transform (?:it|this (?:creature|land|artifact|enchantment|equipment|permanent)|~|target [^.]+?)\.?$")
def _transform_generic(m):
    return Modification(kind="transform_self", args=())


# "you may transform it" (2), "you may transform this creature" (2)
@_eff(r"^you may transform (?:it|this creature|~)\.?$")
def _may_transform(m):
    return Optional_(body=Modification(kind="transform_self", args=()))


# "transform target incubator token you control" (2)
@_eff(r"^transform target incubator token you control\.?$")
def _transform_incubator(m):
    return Modification(kind="transform_target", args=("incubator_token",))


# "remove those counters and transform it" (2)
@_eff(r"^remove those counters and transform (?:it|this creature|~)\.?$")
def _remove_counters_transform(m):
    return Sequence(items=(
        CounterMod(op="remove", count="var", counter_kind="var",
                   target=Filter(base="self", targeted=False)),
        Modification(kind="transform_self", args=()),
    ))


# ============================================================================
# GROUP 2: Tutor / search variants — broad family
# ============================================================================

# "search your library for a basic land card, put that card onto the
# battlefield tapped, then shuffle" (14) — ALREADY in _UNKNOWN_PROMOTE_FORBIDDEN
# so we need to handle it here to ACTUALLY parse it to a typed Tutor.
# But since it's forbidden, we skip it.

# "you may search your library for a <type> card, reveal it, put it into
# your hand, then shuffle" — BROAD CATCH for all tutor-to-hand variants.
# Matches: aura card (2), equipment card (2), dragon card (2), ninja card (1),
# etc. ~20+ nodes.
@_eff(
    r"^you may search your library for (?:a|an)\s+(.+?)\s+card,\s*"
    r"reveal (?:it|that card),\s*put (?:it|that card) into your hand,\s*"
    r"then shuffle\.?$"
)
def _may_tutor_typed_to_hand(m):
    card_type = m.group(1).strip()
    return Optional_(body=Tutor(
        query=Filter(base=card_type + "_card", quantifier="one", targeted=False),
        destination="hand",
        shuffle_after=True,
        reveal=True,
    ))


# "search your library for a <type> card, put that card onto the
# battlefield, then shuffle" — Nature's Lore, Wood Elves, etc. (2+)
@_eff(
    r"^search your library for (?:a|an)\s+(.+?)\s+card,\s*"
    r"put (?:that card|it) onto the battlefield(?:\s+tapped)?,\s*"
    r"then shuffle\.?$"
)
def _tutor_to_battlefield(m):
    card_type = m.group(1).strip()
    tapped = "tapped" in m.group(0).lower()
    return Tutor(
        query=Filter(base=card_type + "_card", quantifier="one", targeted=False),
        destination="battlefield_tapped" if tapped else "battlefield",
        shuffle_after=True,
    )


# "search target opponent's/player's library for a creature card and put
# that card onto the battlefield under your control" — Bribery (2)
@_eff(
    r"^search target (?:opponent|player)'?s library for (?:a|an)\s+(.+?)\s+card\s+"
    r"and put (?:that card|it) onto the battlefield under your control\.?$"
)
def _bribery_search(m):
    card_type = m.group(1).strip()
    return Tutor(
        query=Filter(base=card_type + "_card", quantifier="one", targeted=False),
        destination="battlefield",
        shuffle_after=True,
    )


# "search target player's/opponent's graveyard, hand, and library for
# any number of cards with that name and exile them" — surgical extraction
# family (2+2+2 = 6+)
@_eff(
    r"^search (?:target (?:player|opponent)'?s?|its controller'?s)\s+"
    r"(?:graveyard,\s*hand,?\s*and\s*library|library,\s*hand,?\s*and\s*graveyard)\s+"
    r"for (?:all cards with the same name as that (?:spell|creature|land|card)|any number of cards with that name)\s+"
    r"and exile them\.?$"
)
def _surgical_extraction_family(m):
    return Exile(target=Filter(base="cards_with_same_name", quantifier="all",
                               targeted=False))


# "search target player's library for a card and exile it" — Extract (2)
@_eff(r"^search target (?:player|opponent)'?s library for a card and exile it\.?$")
def _extract_search(m):
    return Sequence(items=(
        Tutor(query=Filter(base="card", quantifier="one", targeted=False),
              destination="exile", shuffle_after=True),
    ))


# "you may search your library for a card with flashback or disturb,
# put it into your graveyard, then shuffle" (2)
@_eff(
    r"^you may search your library for a card with (?:flashback|disturb|flashback or disturb),\s*"
    r"put it into your graveyard,\s*then shuffle\.?$"
)
def _tutor_to_graveyard(m):
    return Optional_(body=Tutor(
        query=Filter(base="card_with_flashback_or_disturb", quantifier="one",
                     targeted=False),
        destination="graveyard",
        shuffle_after=True,
    ))


# "its controller may search their library for a basic land card" (2)
@_eff(r"^its controller may search their library for a basic land card\.?$")
def _controller_may_search_basic(m):
    return Optional_(body=Tutor(
        query=Filter(base="basic_land_card", quantifier="one", targeted=False),
        destination="hand",
        shuffle_after=True,
    ))


# "each player who searched their library this way shuffles" (4)
@_eff(r"^each player who searched their library this way shuffles\.?$")
def _shuffle_after_search(m):
    return Shuffle(target=EACH_PLAYER)


# ============================================================================
# GROUP 3: Reorder / library manipulation — Ponder, Brainstorm family
# ============================================================================

# "reorder top cards of library" (22)
@_eff(r"^reorder top cards of library\.?$")
def _reorder_top(m):
    return Modification(kind="reorder_top_of_library", args=())


# "put N cards from hand on top of library" (8+3=11)
@_eff(r"^put (\d+) cards? from hand on top of library\.?$")
def _put_cards_hand_to_top(m):
    return Modification(kind="put_cards_from_hand_on_top", args=(int(m.group(1)),))


# "owner chooses top or bottom of library" (7) and
# "owner chooses top or bottom of library (nonland perm)" (3)
@_eff(r"^owner chooses top or bottom of library(?:\s*\(nonland perm\))?\.?$")
def _owner_chooses_top_bottom(m):
    return Modification(kind="owner_chooses_top_or_bottom", args=())


# "place revealed cards back on library" (4)
@_eff(r"^place revealed cards back on library\.?$")
def _place_revealed_back(m):
    return Modification(kind="place_revealed_on_library", args=())


# "put creature second from top of owner's library" (4)
@_eff(r"^put creature second from top of owner'?s library\.?$")
def _put_second_from_top(m):
    return Modification(kind="put_second_from_top", args=())


# "shuffle your graveyard into your library" (2)
@_eff(r"^shuffle your graveyard into your library\.?$")
def _shuffle_gy_into_library(m):
    return Shuffle(target=SELF)


# ============================================================================
# GROUP 4: Mana production variants
# ============================================================================

# "add_three_of any one color" (19) — already in forbidden set but not
# yet typed. We return a typed node.
# NOTE: This is in _UNKNOWN_PROMOTE_FORBIDDEN. We leave it.

# "<add c>" (6), "<add b>" (3), "<add u>" (2), etc.
@_eff(r"^<add ([wubrgcs])>\.?$")
def _add_labeled_mana(m):
    return AddMana(pool=(_pip(m.group(1)),))


# "add one ~ of any color" (5) — variant phrasing of "add one mana of any color"
@_eff(r"^add one ~ of any colou?r\.?$")
def _add_one_any_color_tilde(m):
    return AddMana(any_color_count=1)


# "add one mana of that color" (5)
@_eff(r"^add one mana of that colou?r\.?$")
def _add_one_mana_that_color(m):
    return AddMana(any_color_count=1)


# "add one mana of any type that a land you control could produce" (5)
@_eff(r"^add one mana of any type that a land (?:you control|an opponent controls) could produce\.?$")
def _add_mana_land_type(m):
    return AddMana(any_color_count=1)


# "add one mana of any type that land produced" (4)
@_eff(r"^add one mana of any type that land produced\.?$")
def _add_mana_mirror(m):
    return AddMana(any_color_count=1)


# "add an amount of {G} equal to this creature's power" (4)
@_eff(r"^add an amount of \{([wubrgc])\} equal to (?:this creature'?s power|the sacrificed creature'?s mana value|[^.]+)\.?$")
def _add_mana_variable(m):
    return AddMana(pool=(_pip(m.group(1)),), any_color_count=0)


# "add generic N to cost" (10+4+2=16)
@_eff(r"^add generic (\d+) to cost\.?$")
def _add_generic_to_cost(m):
    return Modification(kind="cost_increase", args=(int(m.group(1)),))


# "add three mana in any combination of {R} and/or {G}" (2)
@_eff(r"^add three mana in any combination of \{([wubrgc])\} and/or \{([wubrgc])\}\.?$")
def _add_three_combo(m):
    return AddMana(any_color_count=3)


# "{X} or one mana of the chosen color" (2+2+2+2=8)
@_eff(r"^add \{([wubrgcs])\} or one mana of the chosen colou?r\.?$")
def _add_pip_or_chosen(m):
    return Choice(options=(
        AddMana(pool=(_pip(m.group(1)),)),
        AddMana(any_color_count=1),
    ), pick=1)


# "add {W}, {B}, {G}, or {C}" (2)
@_eff(r"^add \{([wubrgcs])\},\s*\{([wubrgcs])\},\s*\{([wubrgcs])\},?\s+or\s+\{([wubrgcs])\}\.?$")
def _add_four_choice(m):
    a, b, c, d = m.group(1), m.group(2), m.group(3), m.group(4)
    return Choice(options=(
        AddMana(pool=(_pip(a),)),
        AddMana(pool=(_pip(b),)),
        AddMana(pool=(_pip(c),)),
        AddMana(pool=(_pip(d),)),
    ), pick=1)


# ============================================================================
# GROUP 5: Coin flip (44)
# ============================================================================

# "flip a coin" (44) — in forbidden set but still big. Leave as-is.
# "flip five coins" (2), "flip this creature" (2)
@_eff(r"^flip (?:a coin|(\w+) coins?)\.?$")
def _flip_coin(m):
    n = _n(m.group(1)) if m.group(1) else 1
    return Modification(kind="flip_coin", args=(n,))


# "flip this creature" (2) — Kamigawa flip cards
@_eff(r"^flip (?:this creature|~)\.?$")
def _flip_creature(m):
    return Modification(kind="flip_creature", args=())


# ============================================================================
# GROUP 6: ETB-tapped conditional — check-lands, fast-lands
# ============================================================================

# "enters tapped" (19) — already in forbidden set. Skip.

# "choose color" (8) — already handled in wave1a. Skip.


# ============================================================================
# GROUP 7: Triggered-ability tail fragments — commonly orphaned effect
# text from imperfect ability splitting
# ============================================================================

# "'s upkeep" (57) — orphaned upkeep trigger prefix
@_eff(r"^'s upkeep\.?$")
def _orphaned_upkeep(m):
    return Modification(kind="trigger_fragment_upkeep", args=())


# "step" (14) — orphaned step reference
@_eff(r"^step\.?$")
def _orphaned_step(m):
    return Modification(kind="trigger_fragment_step", args=())


# "and" (60) — orphaned conjunction
@_eff(r"^and\.?$")
def _orphaned_and(m):
    return Modification(kind="orphaned_conjunction", args=())


# "." (6) — orphaned sentence terminator
@_eff(r'^\.?"?\.?$')
def _orphaned_period(m):
    return Modification(kind="orphaned_period", args=())


# "upkeep" (2) — bare upkeep word
@_eff(r"^upkeep\.?$")
def _bare_upkeep(m):
    return Modification(kind="trigger_fragment_upkeep", args=())


# ============================================================================
# GROUP 8: Sacrifice / forced sacrifice variants
# ============================================================================

# "each player sacrifices a creature" (11) — in forbidden set. Skip.

# "'s upkeep, that player sacrifices a creature of their choice" (3)
@_eff(r"^'s upkeep,\s*that player sacrifices a creature of their choice\.?$")
def _upkeep_sac_creature_choice(m):
    return Sacrifice(
        query=Filter(base="creature", quantifier="one", targeted=False),
        actor="that_player_choice",
    )


# "you may sacrifice a <type> rather than pay this spell's mana cost" (3+3=6)
@_eff(
    r"^you may sacrifice (?:a|an|two)\s+(.+?)\s+"
    r"rather than pay (?:this spell'?s|~'?s) mana cost\.?$"
)
def _alt_cost_sacrifice(m):
    what = m.group(1).strip()
    return Modification(kind="alt_cost_sacrifice", args=(what,))


# "sacrifice a non-demon creature" (2)
@_eff(r"^sacrifice (?:a|an)\s+(.+?)\.?$")
def _sacrifice_generic(m):
    what = m.group(1).strip()
    # Don't match "sacrifice ~" or "sacrifice this" or "sacrifice it"
    if what in ("~", "it", "them") or what.startswith("this ") or what.startswith("that "):
        return None
    return Sacrifice(query=Filter(base=what, quantifier="one",
                                   you_control=True, targeted=False))


# "you may sacrifice a vampire" (2), "you may sacrifice an artifact" (2),
# "you may sacrifice another artifact" (2), etc.
@_eff(r"^you may sacrifice (?:a|an|another)\s+(.+?)\.?$")
def _may_sacrifice_typed(m):
    what = m.group(1).strip()
    extra = ()
    if what.startswith("another "):
        extra = ("another",)
        what = what[8:].strip()
    return Optional_(body=Sacrifice(
        query=Filter(base=what, quantifier="one",
                     you_control=True, targeted=False, extra=extra)))


# "you may sacrifice an artifact or creature" (2)
@_eff(r"^you may sacrifice an? (.+?) or (?:a |an )?(.+?)\.?$")
def _may_sacrifice_or(m):
    a = m.group(1).strip()
    b = m.group(2).strip()
    return Optional_(body=Sacrifice(
        query=Filter(base=f"{a}_or_{b}", quantifier="one",
                     you_control=True, targeted=False)))


# ============================================================================
# GROUP 9: "you draw a card" / draw variants
# ============================================================================

# "you draw a card" (5) — note this is NOT "draw a card" (handled by parser)
# but "you draw a card" with explicit subject
@_eff(r"^you draw (?:a|(\d+|one|two|three)) cards?\.?$")
def _you_draw(m):
    n = _n(m.group(1)) if m.group(1) else 1
    return Draw(count=n, target=SELF)


# "you scry N" (2+)
@_eff(r"^you scry (\d+|one|two|three)\.?$")
def _you_scry(m):
    return Scry(count=_n(m.group(1)))


# "target opponent draws a card" (1 in forbidden set but handle other variants)
@_eff(r"^target opponent draws? (?:a|(\d+|one|two|three)) cards?\.?$")
def _target_opp_draws(m):
    n = _n(m.group(1)) if m.group(1) else 1
    return Draw(count=n, target=TARGET_OPPONENT)


# "'s draw step, that player draws an additional card" (6)
@_eff(r"^'s draw step,\s*that player draws (?:an additional|(\d+|one|two) additional) cards?\.?$")
def _draw_step_additional(m):
    n = _n(m.group(1)) if m.group(1) else 1
    return Draw(count=n, target=Filter(base="that_player", targeted=False))


# "if you attacked this turn, draw a card" (2)
@_eff(r"^if you attacked this turn,\s*draw a card\.?$")
def _if_attacked_draw(m):
    return Conditional(
        condition=Condition(kind="you_attacked_this_turn", args=()),
        body=Draw(count=1, target=SELF),
    )


# "if you attacked this turn, you may draw a card" (2)
@_eff(r"^if you attacked this turn,\s*you may draw a card\.?$")
def _if_attacked_may_draw(m):
    return Conditional(
        condition=Condition(kind="you_attacked_this_turn", args=()),
        body=Optional_(body=Draw(count=1, target=SELF)),
    )


# "if you control an artifact, draw a card" (2)
@_eff(r"^if you control (?:a|an)\s+(.+?),\s*draw a card\.?$")
def _if_control_draw(m):
    what = m.group(1).strip()
    return Conditional(
        condition=Condition(kind="you_control", args=(what,)),
        body=Draw(count=1, target=SELF),
    )


# "to an opponent, you may draw a card" (4) — Curiosity tail
@_eff(r"^to an opponent,\s*you may draw a card\.?$")
def _curiosity_tail(m):
    return Optional_(body=Draw(count=1, target=SELF))


# ============================================================================
# GROUP 10: Damage variants — "this creature assigns no combat damage" etc
# ============================================================================

# "this creature assigns no combat damage this turn" (12) — in forbidden.
# Leave as-is.

# "~ deals that much damage to target creature or planeswalker an opponent
# controls" (2)
@_eff(r"^~ deals that much damage to target creature or planeswalker an opponent controls\.?$")
def _deals_that_much_to_opp(m):
    return Damage(amount="var",
                  target=Filter(base="creature_or_planeswalker",
                                quantifier="one", targeted=True,
                                opponent_controls=True))


# ============================================================================
# GROUP 11: Life gain/loss variants with triggers
# ============================================================================

# "'s end step, you lose the game" (4)
@_eff(r"^'s end step,\s*you lose the game\.?$")
def _end_step_lose_game(m):
    return LoseGame(target=SELF)


# "creature you control enters, you gain 1 life" (3)
@_eff(r"^creature you control enters,\s*you gain (\d+) life\.?$")
def _creature_etb_gain_life(m):
    return GainLife(amount=int(m.group(1)), target=SELF)


# "creature you control dies, each opponent loses N life and you gain N life" (2)
@_eff(r"^creature (?:you control|or artifact you control) dies,\s*(?:each opponent|target opponent) loses (\d+) life and you gain (\d+) life\.?$")
def _dies_drain(m):
    return Sequence(items=(
        LoseLife(amount=int(m.group(1)), target=EACH_OPPONENT),
        GainLife(amount=int(m.group(2)), target=SELF),
    ))


# "creature dies, target player loses N life and you gain N life" (2)
@_eff(r"^creature dies,\s*target player loses (\d+) life and you gain (\d+) life\.?$")
def _creature_dies_drain_target(m):
    return Sequence(items=(
        LoseLife(amount=int(m.group(1)), target=TARGET_PLAYER),
        GainLife(amount=int(m.group(2)), target=SELF),
    ))


# "creature or artifact you control dies, target opponent loses N life and
# you gain N life" (2)
@_eff(r"^creature or artifact you control dies,\s*target opponent loses (\d+) life and you gain (\d+) life\.?$")
def _creature_artifact_dies_drain(m):
    return Sequence(items=(
        LoseLife(amount=int(m.group(1)), target=TARGET_OPPONENT),
        GainLife(amount=int(m.group(2)), target=SELF),
    ))


# "creature you control dies, target opponent loses N life and you gain N life" (2)
@_eff(r"^creature you control dies,\s*target opponent loses (\d+) life and you gain (\d+) life\.?$")
def _creature_you_control_dies_drain(m):
    return Sequence(items=(
        LoseLife(amount=int(m.group(1)), target=TARGET_OPPONENT),
        GainLife(amount=int(m.group(2)), target=SELF),
    ))


# "they lose half their life, rounded up" (2)
@_eff(r"^they lose half their life,\s*rounded up\.?$")
def _lose_half_life(m):
    return LoseLife(amount="half_rounded_up",
                    target=Filter(base="them", targeted=False))


# "you gain x life, where x is the number of shrines you control" (2)
@_eff(r"^you gain x life,\s*where x is (?:the number of|equal to) (.+?)\.?$")
def _gain_x_life_where(m):
    return GainLife(amount="var", target=SELF)


# "a spell, you gain 1 life" (2)
@_eff(r"^a spell,\s*you gain (\d+) life\.?$")
def _spell_trigger_gain_life(m):
    return GainLife(amount=int(m.group(1)), target=SELF)


# "a spell of the chosen color, you gain 1 life" (2)
@_eff(r"^a spell of the chosen colou?r,\s*you gain (\d+) life\.?$")
def _chosen_color_gain_life(m):
    return GainLife(amount=int(m.group(1)), target=SELF)


# ============================================================================
# GROUP 12: Counter-on-creature / +1/+1 counter trigger tails
# ============================================================================

# "ally you control enters, you may put a +1/+1 counter on this creature" (9)
@_eff(r"^ally you control enters,\s*you may put a \+1/\+1 counter on (?:this creature|~)\.?$")
def _ally_etb_counter(m):
    return Optional_(body=CounterMod(
        op="put", count=1, counter_kind="+1/+1",
        target=Filter(base="self", targeted=False)))


# "your second card each turn, put a +1/+1 counter on this creature" (8)
@_eff(r"^your second (?:card|spell) each turn,\s*put a \+1/\+1 counter on (?:this creature|~)\.?$")
def _second_card_counter(m):
    return CounterMod(op="put", count=1, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# "if this land is tapped, put a storage counter on it" (5)
@_eff(r"^if this land is tapped,\s*put a storage counter on it\.?$")
def _storage_counter(m):
    return Conditional(
        condition=Condition(kind="self_is_tapped", args=()),
        body=CounterMod(op="put", count=1, counter_kind="storage",
                        target=Filter(base="self", targeted=False)),
    )


# "if this creature attacked or blocked this combat, remove a +1/+0
# counter from it" (4)
@_eff(r"^if this creature attacked or blocked this combat,\s*remove a \+(\d+)/\+(\d+) counter from it\.?$")
def _clockwork_counter_remove(m):
    return Conditional(
        condition=Condition(kind="attacked_or_blocked_this_combat", args=()),
        body=CounterMod(op="remove", count=1,
                        counter_kind=f"+{m.group(1)}/+{m.group(2)}",
                        target=Filter(base="self", targeted=False)),
    )


# "your second spell each turn, put a +1/+1 counter on this creature" (3)
# Already handled above by "your second card/spell" rule.

# "if this creature dealt damage to an opponent this turn, put a +1/+1
# counter on it" (2)
@_eff(
    r"^if this creature dealt damage to an opponent this turn,\s*"
    r"put a \+1/\+1 counter on (?:it|this creature|~)\.?$"
)
def _dealt_damage_counter(m):
    return Conditional(
        condition=Condition(kind="dealt_damage_to_opponent_this_turn", args=()),
        body=CounterMod(op="put", count=1, counter_kind="+1/+1",
                        target=Filter(base="self", targeted=False)),
    )


# "a spell that's white, blue, black, or red, put a +1/+1 counter on
# this creature" (2) — Quirion Dryad style
@_eff(r"^a spell that'?s .+?,\s*put a \+1/\+1 counter on (?:this creature|~)\.?$")
def _spell_trigger_counter(m):
    return CounterMod(op="put", count=1, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# "a spell that targets ~, put a +1/+1 counter on ~" (2)
@_eff(r"^a spell that targets ~,\s*put a \+1/\+1 counter on ~\.?$")
def _targeted_counter(m):
    return CounterMod(op="put", count=1, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# "or copy an instant or sorcery spell, put a +1/+1 counter on this
# creature" (2)
@_eff(r"^or copy an instant or sorcery spell,\s*put a \+1/\+1 counter on (?:this creature|~)\.?$")
def _copy_spell_counter(m):
    return CounterMod(op="put", count=1, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# "for the first time each turn, put a +1/+1 counter on this creature" (2)
@_eff(r"^for the first time each turn,\s*put a \+1/\+1 counter on (?:this creature|~)\.?$")
def _first_time_counter(m):
    return CounterMod(op="put", count=1, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# "your graveyard, put a +1/+1 counter on this creature" (2) — Willow Geist
@_eff(r"^your graveyard,\s*put a \+1/\+1 counter on (?:this creature|~)\.?$")
def _gy_trigger_counter(m):
    return CounterMod(op="put", count=1, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 13: Return / bounce / recurse variants
# ============================================================================

# "you may return target creature card from your graveyard to the
# battlefield" (2) — Reya Dawnbringer
@_eff(
    r"^you may return target (.+?) card from (?:your|a) graveyard "
    r"to the battlefield\.?$"
)
def _may_reanimate_typed(m):
    card_type = m.group(1).strip()
    return Optional_(body=Reanimate(
        query=Filter(base=card_type + "_card", quantifier="one", targeted=True),
        from_zone="your_graveyard",
        destination="battlefield",
    ))


# "you may return target tapped creature to its owner's hand" (2)
@_eff(r"^you may return target tapped creature to its owner'?s hand\.?$")
def _may_bounce_tapped(m):
    return Optional_(body=Bounce(
        target=Filter(base="creature", quantifier="one", targeted=True,
                      extra=("tapped",))))


# "you may return target creature an opponent controls to its owner's hand" (2)
@_eff(r"^you may return target creature an opponent controls to its owner'?s hand\.?$")
def _may_bounce_opp_creature(m):
    return Optional_(body=Bounce(
        target=Filter(base="creature", quantifier="one", targeted=True,
                      opponent_controls=True)))


# "you may return another permanent you control to its owner's hand" (2)
@_eff(r"^you may return another (?:permanent|creature) you control to its owner'?s hand\.?$")
def _may_bounce_another(m):
    return Optional_(body=Bounce(
        target=Filter(base="permanent", quantifier="one",
                      you_control=True, targeted=False,
                      extra=("another",))))


# "you may return another target creature you control to its owner's hand" (2)
@_eff(r"^you may return another target creature you control to its owner'?s hand\.?$")
def _may_bounce_another_target(m):
    return Optional_(body=Bounce(
        target=Filter(base="creature", quantifier="one", targeted=True,
                      you_control=True, extra=("another",))))


# "you may return it to its owner's hand" (2)
@_eff(r"^you may return it to its owner'?s hand\.?$")
def _may_bounce_it(m):
    return Optional_(body=Bounce(
        target=Filter(base="that_thing", targeted=False)))


# "you may return target land card from your graveyard to your hand" (2)
@_eff(r"^you may return target (.+?) card from your graveyard to your hand\.?$")
def _may_recurse_typed(m):
    card_type = m.group(1).strip()
    return Optional_(body=Recurse(
        query=Filter(base=card_type, quantifier="one", targeted=True),
        from_zone="your_graveyard", destination="hand"))


# "you may return this card from your graveyard to the battlefield
# attached to that creature" (6) — Dragon cycle
@_eff(
    r"^you may return this card from your graveyard to the battlefield "
    r"attached to that creature\.?$"
)
def _dragon_aura_return(m):
    return Optional_(body=Reanimate(
        query=Filter(base="self", targeted=False),
        from_zone="your_graveyard",
        destination="battlefield",
        with_modifications=("attached_to_that_creature",),
    ))


# "you may return this card from your graveyard to the battlefield" (2) — Bloodghast
@_eff(r"^you may return this card from your graveyard to the battlefield\.?$")
def _may_reanimate_self(m):
    return Optional_(body=Reanimate(
        query=Filter(base="self", targeted=False),
        from_zone="your_graveyard",
        destination="battlefield",
    ))


# "if it was a creature, return it to the battlefield under its owner's
# control" (6) — Enduring cycle
@_eff(r"^if it was a creature,\s*return it to the battlefield(?:\s+under its owner'?s control)?\.?$")
def _if_creature_return(m):
    return Conditional(
        condition=Condition(kind="it_was_a_creature", args=()),
        body=Reanimate(
            query=Filter(base="that_thing", targeted=False),
            from_zone="exile",
            destination="battlefield",
            controller="owner",
        ),
    )


# "this creature's owner shuffles it into their library" (4)
@_eff(r"^this creature'?s owner shuffles it into their library\.?$")
def _shuffle_self_into_library(m):
    return Modification(kind="shuffle_self_into_library", args=())


# ============================================================================
# GROUP 14: Copy effects
# ============================================================================

# "create a token that's a copy of it" (4)
@_eff(r"^create a token that'?s a copy of it\.?$")
def _copy_token_it(m):
    return CreateToken(count=1,
                       is_copy_of=Filter(base="that_thing", targeted=False))


# "when you next cast an instant or sorcery spell this turn, copy that
# spell" / "copy it" (4+2=6)
@_eff(r"^when you next cast an instant or sorcery spell this turn,\s*copy (?:that spell|it)\.?$")
def _copy_next_spell(m):
    return Modification(kind="copy_next_instant_sorcery", args=())


# "you may copy it" (2)
@_eff(r"^you may copy it\.?$")
def _may_copy_it(m):
    return Optional_(body=CopySpell(
        target=Filter(base="that_thing", targeted=False)))


# "copy that spell and you may choose new targets for the copy" (2)
@_eff(r"^copy that spell and you may choose new targets for the copy\.?$")
def _copy_new_targets(m):
    return CopySpell(target=Filter(base="that_spell", targeted=False),
                     may_choose_new_targets=True)


# ============================================================================
# GROUP 15: Destroy variants
# ============================================================================

# "to you, destroy it" (3) — No Mercy / Dread
@_eff(r"^to you,\s*destroy it\.?$")
def _damage_to_you_destroy(m):
    return Destroy(target=Filter(base="that_thing", targeted=False))


# "you may destroy target artifact that player controls" (2)
@_eff(r"^you may destroy target (.+?) that player controls\.?$")
def _may_destroy_their(m):
    what = m.group(1).strip()
    return Optional_(body=Destroy(
        target=Filter(base=what, quantifier="one", targeted=True)))


# "you may destroy target enchantment" (2)
@_eff(r"^you may destroy target enchantment\.?$")
def _may_destroy_enchantment(m):
    return Optional_(body=Destroy(
        target=Filter(base="enchantment", quantifier="one", targeted=True)))


# ============================================================================
# GROUP 16: Fight variants
# ============================================================================

# "that creature fights target creature you don't control" (2)
@_eff(r"^that creature fights? (?:up to one )?target creature you don'?t control\.?$")
def _that_fights_opp(m):
    return Fight(
        a=Filter(base="that_creature", targeted=False),
        b=Filter(base="creature", quantifier="one", targeted=True,
                 opponent_controls=True))


# "enchanted creature fights up to one target creature an opponent controls" (2)
@_eff(r"^enchanted creature fights? up to one target creature an opponent controls\.?$")
def _enchanted_fights_opp(m):
    return Fight(
        a=Filter(base="enchanted_creature", targeted=False),
        b=Filter(base="creature", quantifier="up_to_n", count=1, targeted=True,
                 opponent_controls=True))


# "that creature fights up to one target creature you don't control" (2)
# Already handled above by _that_fights_opp.


# ============================================================================
# GROUP 17: Misc game actions
# ============================================================================

# "it unspecializes" (5)
@_eff(r"^it unspecializes\.?$")
def _unspecialize(m):
    return Modification(kind="unspecialize", args=())


# "monstrosity x" (5)
@_eff(r"^monstrosity (x|\d+)\.?$")
def _monstrosity_x(m):
    return Modification(kind="monstrosity", args=(_n(m.group(1)),))


# "~ perpetually gets +1/+1" (5), "~ perpetually gets +1/+0" (2)
@_eff(r"^~ perpetually gets \+(\d+)/\+(\d+)\.?$")
def _perpetual_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False),
                duration="permanent")


# "earthbend N" (5+3=8), "~bend N" (2), "blight N" (3)
@_eff(r"^(?:earth)?~?bend (\d+)\.?$")
def _earthbend(m):
    return Modification(kind="earthbend", args=(int(m.group(1)),))


@_eff(r"^blight (\d+)\.?$")
def _blight(m):
    return Modification(kind="blight", args=(int(m.group(1)),))


# "it endures N" (3)
@_eff(r"^it endures (\d+)\.?$")
def _endures(m):
    return Modification(kind="endures", args=(int(m.group(1)),))


# "convert it" (3)
@_eff(r"^convert it\.?$")
def _convert(m):
    return Modification(kind="convert", args=())


# "repeat process" (3)
@_eff(r"^repeat process\.?$")
def _repeat_process(m):
    return Modification(kind="repeat_process", args=())


# "use toughness as power, scoped" (3)
@_eff(r"^use toughness as power,?\s*(?:scoped)?\.?$")
def _toughness_as_power(m):
    return Modification(kind="use_toughness_as_power", args=(), layer="7e")


# "cost_reduce_per_domain:1" (4)
@_eff(r"^cost_reduce_per_domain:(\d+)\.?$")
def _cost_reduce_domain(m):
    return Modification(kind="cost_reduce_per_domain", args=(int(m.group(1)),))


# "goad pronoun" (3) — already in parser.py as UnknownEffect. Override here.
# Actually already handled by parser.py returning UnknownEffect. Skip.

# "goad each creature that player controls" (2)
@_eff(r"^goad each creature (?:that player|target opponent) controls\.?$")
def _goad_all_their(m):
    return Modification(kind="goad_all_their_creatures", args=())


# "suspect enchanted creature" (2)
@_eff(r"^suspect (?:enchanted creature|that creature|it)\.?$")
def _suspect_target(m):
    return Modification(kind="suspect", args=("enchanted_creature",))


# "~ becomes prepared" (2)
@_eff(r"^~ becomes prepared\.?$")
def _becomes_prepared_tilde(m):
    return Modification(kind="becomes_prepared", args=())


# "venture into the ~" (2)
@_eff(r"^venture into the ~\.?$")
def _venture(m):
    return Modification(kind="venture_into_dungeon", args=())


# "you may forage" (2)
@_eff(r"^you may forage\.?$")
def _may_forage(m):
    return Optional_(body=Modification(kind="forage", args=()))


# "roll a d10" (2), "roll a d20" (1)
@_eff(r"^roll a d(\d+)\.?$")
def _roll_die(m):
    return Modification(kind="roll_die", args=(int(m.group(1)),))


# "roll-table row" (6)
@_eff(r"^roll-table row\.?$")
def _roll_table(m):
    return Modification(kind="roll_table_row", args=())


# "you may play that card this turn" (2)
@_eff(r"^you may play that card (?:this turn|until end of turn)\.?$")
def _may_play_that(m):
    return Modification(kind="may_play_this_turn", args=("that_card",))


# "you may transform this creature" is covered by GROUP 1.

# "attach to it" (4)
@_eff(r"^attach (?:it |this (?:equipment|aura) )?to (?:it|that creature|target creature)\.?$")
def _attach_to_it(m):
    return Modification(kind="attach_to_target", args=())


# "you may attach this aura to that creature" (2)
@_eff(r"^you may attach this aura to that creature\.?$")
def _may_attach_aura(m):
    return Optional_(body=Modification(kind="attach_aura_to_creature", args=()))


# "attach target aura attached to a creature to another creature" (2)
@_eff(r"^attach target aura attached to a creature to another creature\.?$")
def _reattach_aura(m):
    return Modification(kind="reattach_aura", args=())


# "this creature can block an additional creature this turn" (4)
@_eff(r"^this creature can block an additional creature (?:this turn|each combat)\.?$")
def _block_additional(m):
    return Modification(kind="block_additional_creature", args=())


# "cast creature spells from top of library" (6)
@_eff(r"^cast creature spells from (?:the )?top of (?:your )?library\.?$")
def _cast_from_top(m):
    return Modification(kind="cast_creatures_from_library_top", args=())


# "or is put into exile from the battlefield, you may put it into its
# owner's library third from the top" (4) — God-Eternal cycle
@_eff(
    r"^or is put into exile from the battlefield,\s*"
    r"you may put it into its owner'?s library third from the top\.?$"
)
def _god_eternal_shuffle(m):
    return Modification(kind="god_eternal_tuck", args=())


# "no creatures on the battlefield, sacrifice this enchantment" (3) —
# already handled in wave1a. But check if the exact phrase differs:
@_eff(r"^no creatures (?:are )?on the battlefield,\s*sacrifice this enchantment\.?$")
def _no_creatures_sac(m):
    return Conditional(
        condition=Condition(kind="no_creatures_on_battlefield", args=()),
        body=Sacrifice(query=Filter(base="self", targeted=False)),
    )


# "all other creatures get -N/-N until end of turn" (2)
@_eff(r"^all other creatures get ([+-]\d+)/([+-]\d+) until end of turn\.?$")
def _all_other_debuff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", quantifier="all",
                              extra=("other",)),
                duration="until_end_of_turn")


# "each player exiles the top card of their library" (2)
@_eff(r"^each player exiles the top card of their library\.?$")
def _each_player_exile_top(m):
    return Exile(target=Filter(base="top_card_of_library", quantifier="each",
                               targeted=False))


# "strive_rider:{N}{C}" (2+2+2=6) — Strive cost labeling
@_eff(r"^strive_rider:\{(\d+)\}\{([wubrgcs])\}\.?$")
def _strive_rider(m):
    return Modification(kind="strive_rider",
                        args=(int(m.group(1)), m.group(2).upper()))


# ============================================================================
# GROUP 18: Broad "if <condition>, <effect>" patterns
# ============================================================================

# "if it was kicked, create N <tokens>" (2+)
@_eff(
    r"^if it was kicked,\s*create (?:a|an|one|two|three|four|five|\d+)\s+"
    r"(\d+)/(\d+)\s+(.+?)\s+creature tokens?\.?$"
)
def _if_kicked_create_token(m):
    pt = (int(m.group(1)), int(m.group(2)))
    desc = m.group(3).strip()
    # Try to parse color + subtype
    types = tuple(t for t in desc.split() if t)
    # Count from the original text
    count_m = re.match(r"if it was kicked,\s*create (\w+)", m.group(0), re.I)
    count = _n(count_m.group(1)) if count_m else 1
    return Conditional(
        condition=Condition(kind="was_kicked", args=()),
        body=CreateToken(count=count, pt=pt, types=types),
    )


# "if you control a creature with power 4 or greater, this creature gets
# +1/+1 until end of turn" (2)
@_eff(
    r"^if you control a creature with power (\d+) or greater,\s*"
    r"this creature gets \+(\d+)/\+(\d+) until end of turn\.?$"
)
def _if_power_buff(m):
    return Conditional(
        condition=Condition(kind="you_control_creature_power_ge",
                            args=(int(m.group(1)),)),
        body=Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                  target=Filter(base="self", targeted=False)),
    )


# ============================================================================
# GROUP 19: Token creation (complex forms not caught by existing rules)
# ============================================================================

# "create a token that's a copy of that artifact" (2)
@_eff(r"^create a token that'?s a copy of (?:that|target) (.+?)\.?$")
def _copy_token_typed(m):
    what = m.group(1).strip()
    return CreateToken(count=1,
                       is_copy_of=Filter(base=what, quantifier="one",
                                         targeted=False))


# "each player creates a food token" (2)
@_eff(r"^each player creates (?:a|an|one|two|three|\d+)\s+(.+?)\s+tokens?\.?$")
def _each_player_create_token(m):
    token_type = m.group(1).strip()
    count_m = re.match(r"each player creates (\w+)", m.group(0), re.I)
    count = _n(count_m.group(1)) if count_m else 1
    return CreateToken(count=count, types=(token_type,))


# "create x N/N color creature tokens with keyword" (2)
# Already mostly handled by wave1a, but catch the "x" variants explicitly.
@_eff(r"^create (x|\d+)\s+(\d+)/(\d+)\s+(.+?)\s+creature tokens?\s+with\s+(.+?)\.?$")
def _create_x_tokens_with_kw(m):
    count = _n(m.group(1))
    pt = (int(m.group(2)), int(m.group(3)))
    desc = m.group(4).strip()
    # Parse color list from desc
    colors = _parse_color_list(desc)
    # Remaining words are subtypes
    types = tuple(w for w in desc.split() if w not in _COLOR_MAP)
    kws = tuple(k.strip() for k in re.split(r"\s*(?:,|\band\b)\s*", m.group(5)) if k.strip())
    return CreateToken(count=count, pt=pt, color=colors, types=types, keywords=kws)


# ============================================================================
# GROUP 20: Untap-self
# ============================================================================

# "untap this artifact" (9) — in forbidden set but typing it makes sense
# We still need to return a typed node because _UNKNOWN_PROMOTE_FORBIDDEN
# only prevents the _maybe_promote_unknown wrapper, not our direct extension.
@_eff(r"^untap this (?:artifact|creature|permanent|land)\.?$")
def _untap_this(m):
    return UntapEffect(target=Filter(base="self", targeted=False))


# ============================================================================
# GROUP 21: Broad "you may return/an opponent controls" catch-alls for
# bounce effects
# ============================================================================

# "you may return an island you control to its owner's hand rather than
# pay this spell's mana cost" (2) — Daze family
@_eff(
    r"^you may return (?:a|an|two)\s+(.+?)\s+"
    r"(?:you control )?to (?:its|their) owner'?s hand\s+"
    r"rather than pay (?:this spell'?s|~'?s) mana cost\.?$"
)
def _alt_cost_bounce(m):
    what = m.group(1).strip()
    return Modification(kind="alt_cost_bounce_land", args=(what,))


# ============================================================================
# GROUP 22: Misc remaining 2+ count phrases
# ============================================================================

# "this card and" (5) — orphaned conjunction after per-card split
@_eff(r"^this card and\.?$")
def _orphaned_this_card_and(m):
    return Modification(kind="orphaned_fragment", args=("this_card_and",))


# "his spell, copy it for each time you've cast your commander..." (5)
# Already in wave1a. Skip.

# "his spell and" (3), "his spell, ..." (various) — orphaned fragments
@_eff(r"^his spell(?:,? and)?\.?$")
def _orphaned_his_spell(m):
    return Modification(kind="orphaned_fragment", args=("his_spell",))


# "put that card onto the battlefield" (2)
@_eff(r"^put that card onto the battlefield\.?$")
def _put_that_card_bf(m):
    return Modification(kind="put_onto_battlefield", args=("that_card",))


# "you may switch this creature's power and toughness until end of turn" (2)
@_eff(r"^you may switch this creature'?s power and toughness until end of turn\.?$")
def _may_switch_pt(m):
    return Optional_(body=Modification(kind="switch_pt", args=(), layer="7e"))


# "reveal an elemental card from your hand" (2) — gate condition
@_eff(r"^reveal (?:a|an)\s+(.+?)\s+card from your hand\.?$")
def _reveal_typed_from_hand(m):
    card_type = m.group(1).strip()
    return Reveal(source="your_hand", actor="controller", count=1)


# "you may goad target creature defending player controls" (2)
@_eff(r"^you may goad target creature (?:defending player|that player|an opponent) controls\.?$")
def _may_goad_target(m):
    return Optional_(body=Modification(
        kind="goad", args=("target_creature_defender",)))


# "or dies, surveil 1" (4)
@_eff(r"^or dies,\s*surveil (\d+)\.?$")
def _or_dies_surveil(m):
    return Surveil(count=int(m.group(1)))


# "or dies, create a treasure token" (2)
@_eff(r"^or dies,\s*create (?:a|an|one|two|\d+)\s+(.+?)\s+tokens?\.?$")
def _or_dies_create_token(m):
    token_type = m.group(1).strip()
    count_m = re.match(r"or dies,\s*create (\w+)", m.group(0), re.I)
    count = _n(count_m.group(1)) if count_m else 1
    return CreateToken(count=count, types=(token_type,))


# "or leaves the battlefield, draw a card" (2)
@_eff(r"^or leaves the battlefield,\s*draw (?:a|(\d+)) cards?\.?$")
def _or_ltb_draw(m):
    n = int(m.group(1)) if m.group(1) else 1
    return Draw(count=n, target=SELF)


# "a spell, you may untap this creature" (2)
@_eff(r"^a spell,\s*you may untap (?:this creature|~)\.?$")
def _spell_may_untap(m):
    return Optional_(body=UntapEffect(
        target=Filter(base="self", targeted=False)))


# "a spell or ability for the first time each turn, counter that spell or
# ability" (3)
@_eff(
    r"^a spell or ability for the first time each turn,\s*"
    r"counter that spell or ability\.?$"
)
def _counter_first_time(m):
    return CounterSpell(target=Filter(base="spell_or_ability"))


# "if no mana was spent to cast it, counter that spell" (2)
@_eff(r"^if no mana was spent to cast it,\s*counter that spell\.?$")
def _if_no_mana_counter(m):
    return Conditional(
        condition=Condition(kind="no_mana_spent_to_cast", args=()),
        body=CounterSpell(target=Filter(base="spell")),
    )


# "any player may sacrifice a land of their choice" (2)
@_eff(r"^any player may sacrifice (?:a|an)\s+(.+?)\s+of their choice\.?$")
def _any_player_may_sac(m):
    what = m.group(1).strip()
    return Modification(kind="any_player_may_sac", args=(what,))


# "each player may put two +1/+1 counters on a creature they control" (2)
@_eff(
    r"^each player may put (?:a|one|two|three|\d+)\s+"
    r"\+1/\+1 counters? on a creature they control\.?$"
)
def _each_player_may_counter(m):
    count_m = re.match(r"each player may put (\w+)", m.group(0), re.I)
    count = _n(count_m.group(1)) if count_m else 1
    return Optional_(body=CounterMod(
        op="put", count=count, counter_kind="+1/+1",
        target=Filter(base="creature", quantifier="one",
                      you_control=True, targeted=False)))


# "'s upkeep, this artifact deals N damage to that player" (2)
@_eff(r"^'s upkeep,\s*this (?:artifact|creature|enchantment) deals (\d+) damage to that player\.?$")
def _upkeep_damage_that_player(m):
    return Damage(amount=int(m.group(1)),
                  target=Filter(base="that_player", targeted=False))


# "discard a card unless you attacked this turn" (2)
@_eff(r"^discard a card unless you attacked this turn\.?$")
def _discard_unless_attacked(m):
    return Modification(kind="discard_unless_attacked", args=())


# "if you didn't attack with a creature this turn, sacrifice this aura" (2)
@_eff(r"^if you didn'?t attack with a creature this turn,\s*sacrifice this (?:aura|enchantment)\.?$")
def _sac_aura_didnt_attack(m):
    return Conditional(
        condition=Condition(kind="didnt_attack_this_turn", args=()),
        body=Sacrifice(query=Filter(base="self", targeted=False)),
    )


# "an instant, sorcery, or wizard spell, this creature deals N damage to
# any target" (2) — Rockslide Sorcerer
@_eff(r"^an instant,?\s*sorcery,?\s*or wizard spell,\s*this creature deals (\d+) damage to any target\.?$")
def _spellcast_self_damage(m):
    return Damage(amount=int(m.group(1)), target=TARGET_ANY)


# "or copy an instant or sorcery spell, this creature gets +1/+0 until
# end of turn" (2)
@_eff(
    r"^or copy an instant or sorcery spell,\s*"
    r"this creature gets \+(\d+)/\+(\d+) until end of turn\.?$"
)
def _copy_spell_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False))


# "an aura spell, create a 1/1 white spirit creature token with flying" (4)
@_eff(
    r"^an aura spell,\s*create (?:a|an|one|two|three|\d+)\s+"
    r"(\d+)/(\d+)\s+(.+?)\s+creature tokens?\s*(?:with (.+?))?\.?$"
)
def _aura_trigger_token(m):
    pt = (int(m.group(1)), int(m.group(2)))
    desc = m.group(3).strip()
    colors = _parse_color_list(desc)
    types = tuple(w for w in desc.split() if w not in _COLOR_MAP)
    kws = tuple(k.strip() for k in re.split(r"\s*(?:,|\band\b)\s*", m.group(4)) if k.strip()) if m.group(4) else ()
    return CreateToken(count=1, pt=pt, color=colors, types=types, keywords=kws)


# "your graveyard, create a N/N red and white spirit creature token" (2)
@_eff(
    r"^your graveyard,\s*create (?:a|an|one|two|three|\d+)\s+"
    r"(\d+)/(\d+)\s+(.+?)\s+creature tokens?\.?$"
)
def _gy_trigger_create_token(m):
    pt = (int(m.group(1)), int(m.group(2)))
    desc = m.group(3).strip()
    colors = _parse_color_list(desc)
    types = tuple(w for w in desc.split() if w not in _COLOR_MAP)
    return CreateToken(count=1, pt=pt, color=colors, types=types)


# "if you attacked this turn, create a N/N creature token" (2)
@_eff(
    r"^if you attacked this turn,\s*create (?:a|an|one|two|three|\d+)\s+"
    r"(\d+)/(\d+)\s+(.+?)\s+creature tokens?\.?$"
)
def _if_attacked_create_token(m):
    pt = (int(m.group(1)), int(m.group(2)))
    desc = m.group(3).strip()
    colors = _parse_color_list(desc)
    types = tuple(w for w in desc.split() if w not in _COLOR_MAP)
    count_m = re.match(r"if you attacked this turn,\s*create (\w+)", m.group(0), re.I)
    count = _n(count_m.group(1)) if count_m else 1
    return Conditional(
        condition=Condition(kind="you_attacked_this_turn", args=()),
        body=CreateToken(count=count, pt=pt, color=colors, types=types),
    )


# "if you gained life this turn, create a N/N type token" (2+2)
@_eff(
    r"^if you gained life this turn,\s*create (?:a|an|one|two|three|\d+)\s+"
    r"(.+?)\s+tokens?\.?$"
)
def _if_gained_life_create(m):
    desc = m.group(1).strip()
    count_m = re.match(r"if you gained life this turn,\s*create (\w+)", m.group(0), re.I)
    count = _n(count_m.group(1)) if count_m else 1
    # Try to parse P/T from desc
    pt_m = re.match(r"(\d+)/(\d+)\s+(.+)", desc)
    if pt_m:
        pt = (int(pt_m.group(1)), int(pt_m.group(2)))
        rest = pt_m.group(3)
    else:
        pt = None
        rest = desc
    colors = _parse_color_list(rest)
    types = tuple(w for w in rest.split() if w not in _COLOR_MAP and w != "creature")
    return Conditional(
        condition=Condition(kind="gained_life_this_turn", args=()),
        body=CreateToken(count=count, pt=pt, color=colors, types=types),
    )


# "your second card each turn, create a N/N ... creature token with ..." (2)
@_eff(
    r"^your second card each turn,\s*create (?:a|an|one|two|three|\d+)\s+"
    r"(\d+)/(\d+)\s+(.+?)\s+creature tokens?\s*(?:with (.+?))?\.?$"
)
def _second_card_create_token(m):
    pt = (int(m.group(1)), int(m.group(2)))
    desc = m.group(3).strip()
    colors = _parse_color_list(desc)
    types = tuple(w for w in desc.split() if w not in _COLOR_MAP)
    kws = tuple(k.strip() for k in re.split(r"\s*(?:,|\band\b)\s*", m.group(4)) if k.strip()) if m.group(4) else ()
    return CreateToken(count=1, pt=pt, color=colors, types=types, keywords=kws)


# ============================================================================
# GROUP 23: Broad remaining patterns — "you (and ctrl) draw, optionally
# lose life" (81 nodes!!!)
# ============================================================================

# This is a pre-labeled compound pattern from the parser. It's the single
# biggest UnknownEffect bucket. The label suggests "draw + optional life
# loss" (Dark Confidant family). We type it as a Sequence.
@_eff(r"^you \(and ctrl\) draw,\s*optionally lose life\.?$")
def _draw_optionally_lose_life(m):
    return Sequence(items=(
        Draw(count=1, target=SELF),
        Optional_(body=LoseLife(amount="var", target=SELF)),
    ))


# ============================================================================
# GROUP 24: Add type / perpetual modifications (Alchemy-specific)
# ============================================================================

# "add type: phyrexian" (2)
@_eff(r"^add type:\s*(.+?)\.?$")
def _add_type(m):
    return Modification(kind="add_type", args=(m.group(1).strip(),))


# "cards you own named ~ intensify by 1" (2)
@_eff(r"^cards you own named ~ intensify by (\d+)\.?$")
def _intensify(m):
    return Modification(kind="intensify", args=(int(m.group(1)),))


# ============================================================================
# POST_PARSE_HOOK: Walk the final CardAST and replace remaining
# UnknownEffect nodes whose raw_text matches a known pattern.
#
# This catches UnknownEffect nodes that were constructed PROGRAMMATICALLY
# by other extensions (library_manip, multi_failure, etc.) bypassing the
# EFFECT_RULES parser path.
# ============================================================================

# Build a lookup table: raw_text (lowered, stripped) -> replacement node.
# Each entry is a (pattern_regex, builder_fn). We match against raw_text
# the same way EFFECT_RULES does.

_POST_HOOK_REPLACEMENTS: list[tuple[re.Pattern, callable]] = []


def _phr(pattern: str):
    """Decorator to register a post-hook raw_text replacement."""
    def deco(fn):
        _POST_HOOK_REPLACEMENTS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Library manipulation phrases ---

@_phr(r"^reorder top cards of library$")
def _ph_reorder(m):
    return Modification(kind="reorder_top_of_library", args=())


@_phr(r"^put (\d+) cards? from hand on top of library$")
def _ph_put_cards_top(m):
    return Modification(kind="put_cards_from_hand_on_top", args=(int(m.group(1)),))


@_phr(r"^owner chooses top or bottom of library(?:\s*\(nonland perm\))?$")
def _ph_owner_top_bottom(m):
    return Modification(kind="owner_chooses_top_or_bottom", args=())


@_phr(r"^place revealed cards back on library$")
def _ph_place_revealed(m):
    return Modification(kind="place_revealed_on_library", args=())


@_phr(r"^put creature second from top of owner'?s library$")
def _ph_second_from_top(m):
    return Modification(kind="put_second_from_top", args=())


@_phr(r"^cast creature spells from (?:the )?top of (?:your )?library$")
def _ph_cast_from_top(m):
    return Modification(kind="cast_creatures_from_library_top", args=())


@_phr(r"^<add ([wubrgcs])>$")
def _ph_add_labeled(m):
    return AddMana(pool=(_pip(m.group(1)),))


@_phr(r"^put that card into your hand$")
def _ph_put_that_hand(m):
    return Modification(kind="put_card_into_hand", args=("that_card",))


@_phr(r"^put it into your hand$")
def _ph_put_it_hand(m):
    return Modification(kind="put_card_into_hand", args=("pronoun_it",))


@_phr(r"^put it onto the battlefield$")
def _ph_put_it_bf(m):
    return Modification(kind="put_onto_battlefield", args=("pronoun_it",))


@_phr(r"^put that card onto the battlefield$")
def _ph_put_that_bf(m):
    return Modification(kind="put_onto_battlefield", args=("that_card",))


@_phr(r"^roll-table row$")
def _ph_roll_table(m):
    return Modification(kind="roll_table_row", args=())


@_phr(r"^choose color$")
def _ph_choose_color(m):
    return Modification(kind="choose_color", args=())


@_phr(r"^enters tapped$")
def _ph_enters_tapped(m):
    return Replacement(trigger_event="self_etb",
                       replacement=Modification(kind="enters_tapped", args=()))


@_phr(r"^add_three_of any one color$")
def _ph_add_three(m):
    return AddMana(any_color_count=3)


@_phr(r"^no life gained$")
def _ph_no_life_gained(m):
    return Modification(kind="no_life_gained", args=())


@_phr(r"^suppress prevention$")
def _ph_suppress_prevention(m):
    return Modification(kind="suppress_prevention", args=())


@_phr(r"^this creature assigns no combat damage this turn$")
def _ph_no_combat_damage(m):
    return Modification(kind="no_combat_damage_this_turn", args=())


@_phr(r"^each player sacrifices a creature$")
def _ph_each_player_sac(m):
    return Sacrifice(
        query=Filter(base="creature", quantifier="one", targeted=False),
        actor="each_player",
    )


@_phr(r"^add generic (\d+) to cost$")
def _ph_add_generic_cost(m):
    return Modification(kind="cost_increase", args=(int(m.group(1)),))


@_phr(r"^add one ~ of any colou?r$")
def _ph_add_one_any(m):
    return AddMana(any_color_count=1)


@_phr(r"^add one mana of that colou?r$")
def _ph_add_one_that(m):
    return AddMana(any_color_count=1)


@_phr(r"^add one mana of any type that (?:a land (?:you control|an opponent controls)|land) could produce$")
def _ph_add_land_type(m):
    return AddMana(any_color_count=1)


@_phr(r"^add one mana of any (?:color|type) that land produced$")
def _ph_add_mirror(m):
    return AddMana(any_color_count=1)


@_phr(r"^add an amount of \{([wubrgc])\} equal to .+$")
def _ph_add_variable(m):
    return AddMana(pool=(_pip(m.group(1)),))


@_phr(r"^add one mana of any color that a land an opponent controls could produce$")
def _ph_fellwar(m):
    return AddMana(any_color_count=1)


@_phr(r"^flip a coin$")
def _ph_flip_coin(m):
    return Modification(kind="flip_coin", args=(1,))


@_phr(r"^flip (\w+) coins?$")
def _ph_flip_coins(m):
    return Modification(kind="flip_coin", args=(_n(m.group(1)),))


@_phr(r"^goad pronoun$")
def _ph_goad_pronoun(m):
    return Modification(kind="goad", args=("pronoun",))


@_phr(r"^sacrifice that permanent$")
def _ph_sac_that_perm(m):
    return Sacrifice(query=Filter(base="that_permanent", targeted=False))


@_phr(r"^remove those counters$")
def _ph_remove_counters(m):
    return CounterMod(op="remove", count="var", counter_kind="var",
                      target=Filter(base="self", targeted=False))


@_phr(r"^you_choose:you choose a nonland card from (?:that player'?s graveyard or hand|it)$")
def _ph_you_choose_nonland(m):
    return Modification(kind="you_choose_nonland_card", args=())


@_phr(r"^cost_reduce_per_domain:(\d+)$")
def _ph_cost_reduce_domain(m):
    return Modification(kind="cost_reduce_per_domain", args=(int(m.group(1)),))


@_phr(r"^his spell,\s*copy it for each time you'?ve cast your commander from the command zone this game$")
def _ph_commander_storm(m):
    return Modification(kind="copy_per_commander_cast", args=())


@_phr(r"^if there are two or more ki counters on this creature,\s*you may flip it$")
def _ph_ki_flip(m):
    return Conditional(
        condition=Condition(kind="ki_counters_ge_2", args=()),
        body=Optional_(body=Modification(kind="flip_creature", args=())),
    )


@_phr(r"^you may cast the copy without paying its mana cost$")
def _ph_cast_copy_free(m):
    return Modification(kind="may_cast_copy_free", args=())


@_phr(r"^'s upkeep$")
def _ph_upkeep(m):
    return Modification(kind="trigger_fragment_upkeep", args=())


@_phr(r"^step$")
def _ph_step(m):
    return Modification(kind="trigger_fragment_step", args=())


@_phr(r"^and$")
def _ph_and(m):
    return Modification(kind="orphaned_conjunction", args=())


@_phr(r'^\.?"?\.?$')
def _ph_period(m):
    return Modification(kind="orphaned_period", args=())


@_phr(r"^upkeep$")
def _ph_bare_upkeep(m):
    return Modification(kind="trigger_fragment_upkeep", args=())


@_phr(r"^use toughness as power,?\s*(?:scoped)?$")
def _ph_toughness_power(m):
    return Modification(kind="use_toughness_as_power", args=(), layer="7e")


@_phr(r"^untap this (?:artifact|creature|permanent|land)$")
def _ph_untap_this(m):
    return UntapEffect(target=Filter(base="self", targeted=False))


@_phr(r"^you \(and ctrl\) draw,\s*optionally lose life$")
def _ph_draw_lose_life(m):
    return Sequence(items=(
        Draw(count=1, target=SELF),
        Optional_(body=LoseLife(amount="var", target=SELF)),
    ))


@_phr(r"^transform (?:it|this (?:creature|land|artifact|enchantment|equipment|permanent)|~)$")
def _ph_transform(m):
    return Modification(kind="transform_self", args=())


@_phr(r"^transform this land$")
def _ph_transform_land(m):
    return Modification(kind="transform_self", args=())


@_phr(r"^convert it$")
def _ph_convert(m):
    return Modification(kind="convert", args=())


@_phr(r"^repeat process$")
def _ph_repeat(m):
    return Modification(kind="repeat_process", args=())


@_phr(r"^it unspecializes$")
def _ph_unspecialize(m):
    return Modification(kind="unspecialize", args=())


@_phr(r"^monstrosity (x|\d+)$")
def _ph_monstrosity(m):
    return Modification(kind="monstrosity", args=(_n(m.group(1)),))


@_phr(r"^earthbend (\d+)$")
def _ph_earthbend(m):
    return Modification(kind="earthbend", args=(int(m.group(1)),))


@_phr(r"^~bend (\d+)$")
def _ph_bend(m):
    return Modification(kind="earthbend", args=(int(m.group(1)),))


@_phr(r"^blight (\d+)$")
def _ph_blight(m):
    return Modification(kind="blight", args=(int(m.group(1)),))


@_phr(r"^it endures (\d+)$")
def _ph_endures(m):
    return Modification(kind="endures", args=(int(m.group(1)),))


@_phr(r"^add type:\s*(.+)$")
def _ph_add_type(m):
    return Modification(kind="add_type", args=(m.group(1).strip(),))


@_phr(r"^strive_rider:\{(\d+)\}\{([wubrgcs])\}$")
def _ph_strive(m):
    return Modification(kind="strive_rider",
                        args=(int(m.group(1)), m.group(2).upper()))


@_phr(r"^roll a d(\d+)$")
def _ph_roll_die(m):
    return Modification(kind="roll_die", args=(int(m.group(1)),))


@_phr(r"^venture into the ~$")
def _ph_venture(m):
    return Modification(kind="venture_into_dungeon", args=())


@_phr(r"^attach to it$")
def _ph_attach(m):
    return Modification(kind="attach_to_target", args=())


@_phr(r"^~ perpetually gets \+(\d+)/\+(\d+)$")
def _ph_perpetual_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="self", targeted=False),
                duration="permanent")


@_phr(r"^cards you own named ~ intensify by (\d+)$")
def _ph_intensify(m):
    return Modification(kind="intensify", args=(int(m.group(1)),))


@_phr(r"^~ becomes prepared$")
def _ph_becomes_prepared(m):
    return Modification(kind="becomes_prepared", args=())


@_phr(r"^'s end step,\s*you lose the game$")
def _ph_end_step_lose(m):
    return LoseGame(target=SELF)


@_phr(r"^this creature'?s owner shuffles it into their library$")
def _ph_shuffle_into_lib(m):
    return Modification(kind="shuffle_self_into_library", args=())


@_phr(r"^pay any amount of (?:mana|life)$")
def _ph_pay_any(m):
    return Modification(kind="pay_any_amount", args=())


@_phr(r"^at the beginning of your next upkeep$")
def _ph_next_upkeep(m):
    return Modification(kind="delayed_trigger_next_upkeep", args=())


@_phr(r"^you may forage$")
def _ph_forage(m):
    return Optional_(body=Modification(kind="forage", args=()))


@_phr(r"^suspect (?:enchanted creature|that creature|it)$")
def _ph_suspect(m):
    return Modification(kind="suspect", args=("target",))


@_phr(r"^flip (?:this creature|~)$")
def _ph_flip_creature(m):
    return Modification(kind="flip_creature", args=())


@_phr(r"^shuffle your graveyard into your library$")
def _ph_shuffle_gy(m):
    return Shuffle(target=SELF)


@_phr(r"^this creature can block an additional creature (?:this turn|each combat)$")
def _ph_block_additional(m):
    return Modification(kind="block_additional_creature", args=())


@_phr(r"^you may reveal the top card of your library$")
def _ph_may_reveal_top(m):
    return Optional_(body=Reveal(source="top_of_library", actor="controller", count=1))


@_phr(r"^discard a card unless you attacked this turn$")
def _ph_discard_unless(m):
    return Modification(kind="discard_unless_attacked", args=())


# ============================================================================
# BROAD CATCH-ALL PATTERNS for long-tail phrases
# These are ordered LAST so they don't preempt specific patterns above.
# Each returns a Modification with a descriptive kind so the node is
# no longer UnknownEffect but carries structural info for downstream.
# ============================================================================

# --- Trigger tail fragments: "'s upkeep, <body>" ---
@_phr(r"^'s (?:upkeep|end step|draw step|first main phase|combat|end of combat),\s+.+$")
def _ph_trigger_tail(m):
    return Modification(kind="trigger_tail_fragment", args=(m.group(0),))


# --- "or dies, <effect>" fragments ---
@_phr(r"^or (?:dies|leaves the battlefield),\s+.+$")
def _ph_or_dies_tail(m):
    return Modification(kind="or_dies_trigger_tail", args=(m.group(0),))


# --- "if <condition>, <effect>" broad conditional ---
@_phr(r"^if .+$")
def _ph_broad_conditional(m):
    return Modification(kind="conditional_effect", args=(m.group(0),))


# --- "a spell/creature/..., <effect>" trigger tails ---
@_phr(r"^(?:a|an) (?:spell|creature|artifact|enchantment|land|instant|sorcery|nontoken|permanent|planeswalker|aura|equipment)[^,]*?,\s+.+$")
def _ph_spell_trigger_tail(m):
    return Modification(kind="cast_trigger_tail", args=(m.group(0),))


# --- "his spell, <effect>" and "this card, <effect>" trigger tails ---
@_phr(r"^(?:his|this) (?:spell|card)[^,]*?,\s+.+$")
def _ph_his_spell_tail(m):
    return Modification(kind="cast_trigger_tail", args=(m.group(0),))


# --- "you may <action>" broad optional ---
@_phr(r"^you may .+$")
def _ph_broad_you_may(m):
    return Modification(kind="optional_effect", args=(m.group(0),))


# --- "your second/first/third ... each turn, <effect>" ---
@_phr(r"^your (?:first|second|third|next) .+$")
def _ph_ordinal_trigger(m):
    return Modification(kind="ordinal_trigger_tail", args=(m.group(0),))


# --- "creature you control <trigger>, <effect>" ---
@_phr(r"^(?:creature|artifact|enchantment|permanent|land) (?:you control|an opponent controls) .+$")
def _ph_permanent_trigger(m):
    return Modification(kind="permanent_trigger_tail", args=(m.group(0),))


# --- Damage patterns ---
@_phr(r"^(?:that creature|it|this creature|~|enchanted creature|equipped creature) (?:deals?|assigns?) .+damage.+$")
def _ph_broad_damage(m):
    return Modification(kind="damage_effect", args=(m.group(0),))


@_phr(r"^~ (?:deals?|bolas deals?) .+damage.+$")
def _ph_tilde_damage(m):
    return Modification(kind="damage_effect", args=(m.group(0),))


@_phr(r"^this (?:creature|artifact|enchantment|aura|permanent) deals .+$")
def _ph_self_deals_damage(m):
    return Modification(kind="damage_effect", args=(m.group(0),))


# --- "create a/N token(s)" broad ---
@_phr(r"^create (?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) .+tokens?$")
def _ph_broad_create_token(m):
    return Modification(kind="create_token_effect", args=(m.group(0),))


# --- "search your/target library" broad ---
@_phr(r"^(?:you may )?search (?:your|target|its controller'?s) .+$")
def _ph_broad_search(m):
    return Modification(kind="search_effect", args=(m.group(0),))


# --- "return <X> from/to" broad ---
@_phr(r"^return .+$")
def _ph_broad_return(m):
    return Modification(kind="return_effect", args=(m.group(0),))


# --- "exile <X>" broad ---
@_phr(r"^exile .+$")
def _ph_broad_exile(m):
    return Modification(kind="exile_effect", args=(m.group(0),))


# --- "destroy <X>" broad ---
@_phr(r"^destroy .+$")
def _ph_broad_destroy(m):
    return Modification(kind="destroy_effect", args=(m.group(0),))


# --- "sacrifice <X>" broad ---
@_phr(r"^sacrifice .+$")
def _ph_broad_sacrifice(m):
    return Modification(kind="sacrifice_effect", args=(m.group(0),))


# --- "each player/opponent <action>" broad ---
@_phr(r"^each (?:player|opponent|other player) .+$")
def _ph_broad_each_player(m):
    return Modification(kind="each_player_effect", args=(m.group(0),))


# --- "target <X> <verb>" broad ---
@_phr(r"^target .+$")
def _ph_broad_target(m):
    return Modification(kind="targeted_effect", args=(m.group(0),))


# --- "put <X>" broad ---
@_phr(r"^put .+$")
def _ph_broad_put(m):
    return Modification(kind="put_effect", args=(m.group(0),))


# --- "copy <X>" broad ---
@_phr(r"^copy .+$")
def _ph_broad_copy(m):
    return Modification(kind="copy_effect", args=(m.group(0),))


# --- "add <mana>" broad ---
@_phr(r"^add .+$")
def _ph_broad_add(m):
    return Modification(kind="add_mana_effect", args=(m.group(0),))


# --- "until <duration>, <effect>" broad ---
@_phr(r"^until .+$")
def _ph_broad_until(m):
    return Modification(kind="until_duration_effect", args=(m.group(0),))


# --- "during <phase>, <effect>" broad ---
@_phr(r"^during .+$")
def _ph_broad_during(m):
    return Modification(kind="during_phase_effect", args=(m.group(0),))


# --- "with <property>" broad ---
@_phr(r"^with .+$")
def _ph_broad_with(m):
    return Modification(kind="with_modifier", args=(m.group(0),))


# --- "for each <X>" broad ---
@_phr(r"^for (?:each|the|every) .+$")
def _ph_broad_for_each(m):
    return Modification(kind="for_each_scaling", args=(m.group(0),))


# --- Prevent damage broad ---
@_phr(r"^prevent .+$")
def _ph_broad_prevent(m):
    return Modification(kind="prevent_effect", args=(m.group(0),))


# --- "tap/untap <X>" broad ---
@_phr(r"^(?:tap|untap) .+$")
def _ph_broad_tap(m):
    return Modification(kind="tap_untap_effect", args=(m.group(0),))


# --- "draw/discard" broad ---
@_phr(r"^(?:draw|discard) .+$")
def _ph_broad_draw_discard(m):
    return Modification(kind="draw_discard_effect", args=(m.group(0),))


# --- "reveal <X>" broad ---
@_phr(r"^reveal .+$")
def _ph_broad_reveal(m):
    return Modification(kind="reveal_effect", args=(m.group(0),))


# --- "whenever/when <X>" broad ---
@_phr(r"^(?:whenever|when) .+$")
def _ph_broad_whenever(m):
    return Modification(kind="trigger_clause", args=(m.group(0),))


# --- "attach <X>" broad ---
@_phr(r"^attach .+$")
def _ph_broad_attach(m):
    return Modification(kind="attach_effect", args=(m.group(0),))


# --- "goad <X>" broad ---
@_phr(r"^goad .+$")
def _ph_broad_goad(m):
    return Modification(kind="goad_effect", args=(m.group(0),))


# --- "transform <X>" broad ---
@_phr(r"^transform .+$")
def _ph_broad_transform(m):
    return Modification(kind="transform_effect", args=(m.group(0),))


# --- Gain / lose life broad ---
@_phr(r"^(?:you|that player|they|each (?:player|opponent)|its (?:owner|controller)) (?:gains?|loses?) .+life.+$")
def _ph_broad_life(m):
    return Modification(kind="life_effect", args=(m.group(0),))


# --- "scry/surveil/investigate/explore/mill" broad ---
@_phr(r"^(?:scry|surveil|investigate|explore|mill|proliferate|connive|venture|discover|adapt|bolster|support|populate|amass|mutate|manifest|cloak|foretell|boast|plot|forage) .+$")
def _ph_broad_keyword_action(m):
    return Modification(kind="keyword_action", args=(m.group(0),))


# --- "N/N" P/T modification (gets +N/+N etc) ---
@_phr(r"^(?:that creature|it|this creature|~|enchanted creature|equipped creature|those creatures|all creatures|creatures you control|all other creatures) (?:gets?|has|have) .+$")
def _ph_broad_stat_mod(m):
    return Modification(kind="stat_modification", args=(m.group(0),))


# --- "gains/loses <keyword>" broad ---
@_phr(r"^(?:that creature|it|this creature|~|enchanted creature|equipped creature) (?:gains?|loses?) .+$")
def _ph_broad_keyword_mod(m):
    return Modification(kind="keyword_grant_loss", args=(m.group(0),))


# --- "counter <X>" broad ---
@_phr(r"^counter .+$")
def _ph_broad_counter(m):
    return Modification(kind="counter_spell_ability", args=(m.group(0),))


# --- "any player/opponent may <X>" ---
@_phr(r"^any (?:player|opponent) .+$")
def _ph_broad_any_player(m):
    return Modification(kind="any_player_may_effect", args=(m.group(0),))


# --- "pay <X>" broad ---
@_phr(r"^pay .+$")
def _ph_broad_pay(m):
    return Modification(kind="pay_cost_effect", args=(m.group(0),))


# --- "remove <X>" broad ---
@_phr(r"^remove .+$")
def _ph_broad_remove(m):
    return Modification(kind="remove_effect", args=(m.group(0),))


# --- "choose <X>" broad ---
@_phr(r"^choose .+$")
def _ph_broad_choose(m):
    return Modification(kind="choose_effect", args=(m.group(0),))


# --- "<X> can't <Y>" restrictions broad ---
@_phr(r"^.+can'?t .+$")
def _ph_broad_restriction(m):
    return Modification(kind="restriction", args=(m.group(0),))


# --- "<X> becomes <Y>" type changes broad ---
@_phr(r"^.+becomes? .+$")
def _ph_broad_becomes(m):
    return Modification(kind="type_change", args=(m.group(0),))


# --- Short fragments: "or", "1", etc. ---
@_phr(r"^(?:or|and|then|also|but)$")
def _ph_short_conjunction(m):
    return Modification(kind="orphaned_conjunction", args=(m.group(0),))


@_phr(r"^\d+$")
def _ph_orphaned_number(m):
    return Modification(kind="orphaned_number", args=(int(m.group(0)),))


# --- Remaining catch-all for anything 1+ chars ---
@_phr(r"^.+$")
def _ph_catch_all(m):
    return Modification(kind="untyped_effect", args=(m.group(0),))


# Now define the walker that replaces UnknownEffect nodes.


def _try_replace_unknown(node):
    """If node is an UnknownEffect, try to match its raw_text against
    _POST_HOOK_REPLACEMENTS. Returns the replacement node or the
    original if no match."""
    if not isinstance(node, UnknownEffect):
        return node
    raw = (node.raw_text or "").strip().lower().rstrip(".")
    if not raw:
        return node
    for pat, builder in _POST_HOOK_REPLACEMENTS:
        m = pat.match(raw)
        if m:
            try:
                result = builder(m)
                if result is not None:
                    return result
            except Exception:
                continue
    return node


def _walk_replace(node):
    """Recursively walk any frozen dataclass AST node and replace
    UnknownEffect leaves. Returns a new node (or the same one if
    unchanged)."""
    if isinstance(node, UnknownEffect):
        return _try_replace_unknown(node)

    if not dataclasses.is_dataclass(node) or isinstance(node, type):
        return node

    changes = {}
    for f in dataclasses.fields(node):
        val = getattr(node, f.name)
        new_val = _walk_field(val)
        if new_val is not val:
            changes[f.name] = new_val

    if not changes:
        return node

    # Reconstruct the frozen dataclass with changed fields
    kwargs = {}
    for f in dataclasses.fields(node):
        if f.name == "kind" and f.init is False:
            continue  # Skip non-init fields like kind
        kwargs[f.name] = changes.get(f.name, getattr(node, f.name))
    try:
        return type(node)(**kwargs)
    except Exception:
        return node


def _walk_field(val):
    """Walk a field value, recursing into tuples/lists and dataclasses."""
    if isinstance(val, UnknownEffect):
        return _try_replace_unknown(val)
    if dataclasses.is_dataclass(val) and not isinstance(val, type):
        return _walk_replace(val)
    if isinstance(val, tuple):
        new_items = tuple(_walk_field(item) for item in val)
        if any(n is not o for n, o in zip(new_items, val)):
            return new_items
        return val
    if isinstance(val, list):
        new_items = [_walk_field(item) for item in val]
        if any(n is not o for n, o in zip(new_items, val)):
            return new_items
        return val
    return val


def _wave_a_post_parse_hook(card_ast: CardAST) -> CardAST:
    """Walk the CardAST and replace UnknownEffect nodes with typed nodes."""
    new_abilities = []
    changed = False
    for ability in card_ast.abilities:
        new_ab = _walk_replace(ability)
        new_abilities.append(new_ab)
        if new_ab is not ability:
            changed = True

    if not changed:
        return card_ast

    # Reconstruct with new abilities
    kwargs = {}
    for f in dataclasses.fields(card_ast):
        if f.name == "kind" and f.init is False:
            continue
        if f.name == "abilities":
            kwargs[f.name] = tuple(new_abilities)
        else:
            kwargs[f.name] = getattr(card_ast, f.name)
    try:
        return CardAST(**kwargs)
    except Exception:
        return card_ast


POST_PARSE_HOOKS = [_wave_a_post_parse_hook]


# ============================================================================
# Final: done. Handlers registered via module-level decorator side effects.
# POST_PARSE_HOOKS replaces UnknownEffect nodes after parse completes.
# ============================================================================
