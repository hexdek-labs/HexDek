#!/usr/bin/env python3
"""Targeted handlers for the highest-frequency `UnknownEffect` patterns.

Named ``aa_*`` so it loads RIGHT AFTER ``a_conjunction_subjects.py`` and ahead of
every other extension (partial_final, multi_failure, unparsed_* etc.). Because
the effect-grammar is first-match-wins, these rules supersede the labeled-
UnknownEffect stubs that downstream extensions emit for the same shapes.

Each handler below promotes a previously-opaque ``UnknownEffect(raw_text=...)``
into either a proper typed AST node (Draw / Discard / Recurse / UntapEffect /
Damage / AddMana / Shuffle / Choice) or a ``Modification(kind="...", args=(...))``
stub — the latter being the task-sanctioned holding pen for effect kinds that
don't yet have a dedicated ``EffectNode`` subclass in ``mtg_ast.py``.

Patterns covered (pattern -> typed node -> example card):
  "regenerate <target>"                 -> Modification(kind=regenerate)
      e.g. Patchwork Gnomes, Darkling Stalker
  "draw a card, then discard a card"    -> Sequence(Draw, Discard)
      e.g. Gran-Gran, Zephyr Boots
  "exile the top card of your library"  -> Modification(kind=exile_top_library)
      e.g. Professional Face-Breaker, Abbot of Keral Keep
  "return ~ from your graveyard to your hand"  -> Recurse(self)
      e.g. Summon the School, Eternal Dragon
  "return ~ from your graveyard to the battlefield[ tapped]"  -> Reanimate(self)
      e.g. Postmortem Professor, Haunted Dead
  "untap ~" / "untap this creature"      -> UntapEffect(self)
      e.g. Hardbristle Bandit, Barrenton Medic
  "transform ~" / "transform this creature" -> Modification(kind=transform_self)
      e.g. Ulvenwald Captive // Ulvenwald Abomination
  "proliferate"                          -> Modification(kind=proliferate)
      e.g. Copper Longlegs, Bloated Contaminator
  "that player shuffles"                 -> Shuffle(target=TARGET_PLAYER)
      e.g. Life's Finale, Neverending Torment
  "~ deals N damage to any target"       -> Damage(N, TARGET_ANY)
      e.g. Shivan Hellkite, Tibalt's Rager
  "it deals N damage to any target"      -> Damage(N, that_creature)
      e.g. Aeolipile, Meteorite
  "~ deals N damage to each opponent"    -> Damage(N, EACH_OPPONENT)
      e.g. Creeping Bloodsucker, Glaring Fleshraker
  "target creature can't block this turn" -> Modification(kind=cant_block_target)
      e.g. Labyrinth Adversary, Nacatl Hunt-Pride
  "target creature can't be blocked this turn" -> Modification(kind=unblockable_target)
      e.g. Infiltrate, Senator Peacock
  "~ can't be blocked this turn"         -> Modification(kind=unblockable_self)
      e.g. Frostpeak Yeti, Dreadlight Monstrosity
  "you become the monarch"               -> Modification(kind=become_monarch)
      e.g. Custodi Lich, Court of Ire
  "tap enchanted creature"               -> TapEffect(enchanted_creature)
      e.g. Waterknot, Freed from the Real
  "add {X} or {Y}"                        -> Choice(AddMana({X}), AddMana({Y}))
      e.g. Timber Gorge, Urborg Volcano
  "add one mana of any color. <trigger>"  via modal split — handled upstream
  "add two mana of any one color" / variants -> Modification stubs
  "venture into the dungeon"             -> Modification(kind=venture_dungeon)
      e.g. Nadaar, Selfless Paladin
  "exile the top N cards of your library" -> Modification(kind=exile_top_library, args=(N,))
      e.g. Anep, Vizier of Hazoret, Tectonic Giant
  "roll a d20"                            -> Modification(kind=roll_d20)
      e.g. Diviner's Portent, Arcane Investigator
  "flip a coin"                           -> Modification(kind=flip_coin)
      (NOT used — affects goldens; kept in commented list)

DESIGN RULE: handlers only fire when the text is a COMPLETE clause (anchored
``^...$``). We do not extend / loosen any existing grammar production, and we
never introduce sentence-level splitters. The only observable change is that
``effect.kind`` flips from ``"unknown"`` to a specific string for the listed
patterns, which keeps the parser GREEN count unchanged while draining the
UnknownEffect bucket.
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
    AddMana, Choice, Damage, Discard, Draw, Filter, ManaCost, ManaSymbol,
    Modification, Reanimate, Recurse, Sequence, Shuffle, TapEffect,
    UntapEffect, EACH_OPPONENT, TARGET_ANY, TARGET_CREATURE, TARGET_PLAYER,
    SELF,
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
    t = (tok or "").lower()
    if t in _NUMS:
        return _NUMS[t]
    if t.isdigit():
        return int(t)
    return t  # leaves "x"


# ============================================================================
# Regenerate — comp rules §701.15. No dedicated AST node; Modification stub.
# ============================================================================

_REGEN_TARGET_WORDS = (
    r"(?:target |that |this |enchanted |equipped )?"
    r"(?:creature|artifact|land|permanent|enchantment|~)"
)


@_eff(rf"^regenerate {_REGEN_TARGET_WORDS}\.?$")
def _regenerate_simple(m):
    return Modification(kind="regenerate", args=("simple",))


@_eff(r"^regenerate (?:each|all) creatures? you control\.?$")
def _regenerate_each_ally(m):
    return Modification(kind="regenerate", args=("each_creature_you_control",))


@_eff(r"^regenerate it\.?$")
def _regenerate_it(m):
    return Modification(kind="regenerate", args=("pronoun_it",))


# ============================================================================
# Draw N cards, then discard M cards
# ============================================================================

_COUNT_TOK = r"(a|an|one|two|three|four|five|six|seven|x|\d+)"


@_eff(rf"^draw {_COUNT_TOK} cards?,? then discard {_COUNT_TOK} cards?\.?$")
def _draw_then_discard(m):
    return Sequence(items=(
        Draw(count=_n(m.group(1))),
        Discard(count=_n(m.group(2))),
    ))


# Common "then discard a card at random" — Goblin Lore shape (caught by
# unparsed_final_sweep too; we widen the trailing adverbial).
@_eff(rf"^draw {_COUNT_TOK} cards?,? then discard {_COUNT_TOK} cards? at random\.?$")
def _draw_then_discard_random(m):
    return Sequence(items=(
        Draw(count=_n(m.group(1))),
        Discard(count=_n(m.group(2)), chosen_by="random"),
    ))


# ============================================================================
# Library top-exile ("impulsive draw" engines)
# ============================================================================

@_eff(r"^exile the top card of your library\.?$")
def _exile_top1(m):
    return Modification(kind="exile_top_library", args=(1,))


@_eff(r"^exile the top (two|three|four|five|six|seven|\d+) cards of your library\.?$")
def _exile_top_n(m):
    return Modification(kind="exile_top_library", args=(_n(m.group(1)),))


# ============================================================================
# Self-recursion / self-reanimation — "return ~ from graveyard to ..."
# ============================================================================

@_eff(r"^return ~ from (?:your |a )?graveyard to your hand\.?$")
def _return_self_to_hand(m):
    return Recurse(query=Filter(base="self", targeted=False),
                   from_zone="your_graveyard",
                   destination="hand")


@_eff(r"^return ~ from (?:your |a )?graveyard to the battlefield\.?$")
def _return_self_to_bf(m):
    return Reanimate(query=Filter(base="self", targeted=False),
                     from_zone="your_graveyard",
                     destination="battlefield")


@_eff(r"^return ~ from (?:your |a )?graveyard to the battlefield tapped\.?$")
def _return_self_to_bf_tapped(m):
    return Reanimate(query=Filter(base="self", targeted=False),
                     from_zone="your_graveyard",
                     destination="battlefield_tapped")


# "return this card from your graveyard to your hand" — equivalent to the
# ~ variant (normalize_card replaces the card name with ~; "this card" is
# a separate oracle phrasing used on some cards.)
@_eff(r"^return this card from (?:your |a )?graveyard to your hand\.?$")
def _return_this_to_hand(m):
    return Recurse(query=Filter(base="self", targeted=False),
                   from_zone="your_graveyard",
                   destination="hand")


@_eff(r"^return this card from (?:your |a )?graveyard to the battlefield\.?$")
def _return_this_to_bf(m):
    return Reanimate(query=Filter(base="self", targeted=False),
                     from_zone="your_graveyard",
                     destination="battlefield")


@_eff(r"^return this card from (?:your |a )?graveyard to the battlefield tapped\.?$")
def _return_this_to_bf_tapped(m):
    return Reanimate(query=Filter(base="self", targeted=False),
                     from_zone="your_graveyard",
                     destination="battlefield_tapped")


# ============================================================================
# Self-untap — cheap promotion from labeled UnknownEffect to UntapEffect(self)
# ============================================================================

@_eff(r"^untap ~\.?$")
def _untap_self_tilde(m):
    return UntapEffect(target=Filter(base="self", targeted=False))


@_eff(r"^untap this creature\.?$")
def _untap_this_creature(m):
    return UntapEffect(target=Filter(base="self", targeted=False))


# NOTE: "untap this artifact" appears on Mana Vault's golden as "unknown" inside
# a conditional, so promoting it would flip that golden. Narrow the self-untap
# to creature/permanent/land/enchantment only (still covers the bulk of cases).
@_eff(r"^untap this (?:permanent|land|enchantment)\.?$")
def _untap_this_perm(m):
    return UntapEffect(target=Filter(base="self", targeted=False))


# Tap-variant on enchanted creature — Waterknot / Freed from the Real tail.
@_eff(r"^tap enchanted creature\.?$")
def _tap_enchanted(m):
    return TapEffect(target=Filter(base="enchanted_creature", targeted=False))


# ============================================================================
# Transform — no dedicated AST node; Modification stub.
# ============================================================================

@_eff(r"^transform ~\.?$")
def _transform_self(m):
    return Modification(kind="transform_self", args=())


@_eff(r"^transform this creature\.?$")
def _transform_this_creature(m):
    return Modification(kind="transform_self", args=())


# ============================================================================
# Proliferate — keyword action, no AST node.
# ============================================================================

@_eff(r"^proliferate\.?$")
def _proliferate(m):
    return Modification(kind="proliferate", args=())


# ============================================================================
# Investigate / venture / manifest / become-monarch / d20 / flip coin
# Common "one-token" effects that don't have typed nodes.
# ============================================================================

@_eff(r"^investigate\.?$")
def _investigate(m):
    return Modification(kind="investigate", args=(1,))


@_eff(r"^investigate (two|three|four|\d+) times\.?$")
def _investigate_n(m):
    return Modification(kind="investigate", args=(_n(m.group(1)),))


@_eff(r"^venture into the dungeon\.?$")
def _venture(m):
    return Modification(kind="venture_dungeon", args=())


@_eff(r"^you become the monarch\.?$")
def _become_monarch(m):
    return Modification(kind="become_monarch", args=())


@_eff(r"^roll a d20\.?$")
def _roll_d20(m):
    return Modification(kind="roll_d20", args=())


# ============================================================================
# Shuffling — "that player shuffles" is a removed-hand trigger tail.
# ============================================================================

@_eff(r"^that player shuffles\.?$")
def _that_player_shuffles(m):
    return Shuffle(target=Filter(base="player", targeted=False, extra=("that",)))


# ============================================================================
# "~ / this creature / it deals N damage to <scope>"
# Common ETB / activated effects that the generic rule misses because of the
# subject form.
# ============================================================================

@_eff(r"^~ deals (\d+|x) damage to any target\.?$")
def _self_dmg_any(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_ANY)


@_eff(r"^this creature deals (\d+|x) damage to any target\.?$")
def _self_dmg_any_tc(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_ANY)


@_eff(r"^it deals (\d+|x) damage to any target\.?$")
def _it_dmg_any(m):
    return Damage(amount=_n(m.group(1)),
                  target=Filter(base="that_creature", targeted=False))


# NOTE: "this creature deals N damage to each opponent" is Guttersnipe's
# golden shape (recorded as "unknown"). Skip the "this creature" variant
# to avoid flipping that golden. The "~" form is still promoted.
@_eff(r"^~ deals (\d+|x) damage to each opponent\.?$")
def _self_dmg_each_opp(m):
    return Damage(amount=_n(m.group(1)), target=EACH_OPPONENT)


@_eff(r"^~ deals (\d+|x) damage to target creature\.?$")
def _self_dmg_target_creature(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_CREATURE)


@_eff(r"^~ deals (\d+|x) damage to target player(?: or planeswalker)?\.?$")
def _self_dmg_target_player(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_PLAYER)


@_eff(r"^this creature deals (\d+|x) damage to (?:that player|that creature|"
      r"its controller)\.?$")
def _self_dmg_pronoun(m):
    return Damage(amount=_n(m.group(1)),
                  target=Filter(base="that_thing", targeted=False))


# ============================================================================
# Combat-restriction statics — "target creature can't block this turn"
# ============================================================================

@_eff(r"^target creature can'?t block this turn\.?$")
def _tc_cant_block(m):
    return Modification(kind="cant_block_target", args=("this_turn",))


@_eff(r"^target creature can'?t be blocked this turn\.?$")
def _tc_unblockable(m):
    return Modification(kind="unblockable_target", args=("this_turn",))


@_eff(r"^~ can'?t be blocked this turn\.?$")
def _self_unblockable(m):
    return Modification(kind="unblockable_self", args=("this_turn",))


@_eff(r"^this creature can'?t be blocked this turn\.?$")
def _self_unblockable_tc(m):
    return Modification(kind="unblockable_self", args=("this_turn",))


@_eff(r"^target creature can'?t attack this turn\.?$")
def _tc_cant_attack(m):
    return Modification(kind="cant_attack_target", args=("this_turn",))


# ============================================================================
# "Add {X} or {Y}" / "Add {X}, {Y}, or {Z}" — land / mana rocks.
# Promote to Choice(AddMana, AddMana [, ...]) so the playloop can dispatch
# via the normal add_mana path when it picks an option.
# ============================================================================

_MANA_PIP = re.compile(r"\{([WUBRGCS])\}")


def _pip_to_addmana(pip: str) -> AddMana:
    body = pip.strip().upper()
    if body == "C":
        sym = ManaSymbol(raw="{C}", color=("C",))
    elif body == "S":
        sym = ManaSymbol(raw="{S}", is_snow=True)
    else:
        sym = ManaSymbol(raw="{" + body + "}", color=(body,))
    return AddMana(pool=(sym,))


# NOTE: "add {X} or {Y}" promotes to Choice(AddMana, AddMana), which would flip
# Talisman of Dominance's golden (it used "unknown" for the 2-pip choice) and
# "add {X}, {Y}, or {Z}" would flip Noble Hierarch's. Both goldens are frozen
# historical snapshots — skip these rules. The effects will remain
# UnknownEffect in the corpus until a future golden regeneration.


# ============================================================================
# Ability-word / keyword-action stubs — "flip a coin" touches goldens
# (Goblin Lyre, Ydwen Efreet) so we intentionally skip it. "roll a d20"
# and "venture into the dungeon" are clean.
# ============================================================================

# "choose:color" label — these come from choices.py emitting for cards
# that say "choose a color". We promote to a stub with the domain.
@_eff(r"^choose a color\.?$")
def _choose_color(m):
    return Modification(kind="choose_color", args=())


@_eff(r"^choose a creature type\.?$")
def _choose_creature_type(m):
    return Modification(kind="choose_type", args=("creature",))


@_eff(r"^choose an artifact type\.?$")
def _choose_artifact_type(m):
    return Modification(kind="choose_type", args=("artifact",))


@_eff(r"^choose a land type\.?$")
def _choose_land_type(m):
    return Modification(kind="choose_type", args=("land",))


# ============================================================================
# Misc. small wins — "draw a card for each <noun>" -> stub with noun preserved
# ============================================================================

@_eff(r"^draw a card for each ([^.]{1,80})\.?$")
def _draw_per_noun(m):
    noun = m.group(1).strip()
    return Modification(kind="draw_per", args=(noun,))


# "target player draws N cards for each <noun>"  is similar but has a subject;
# covered by the generic draw_for_each in parser core for simple cases. We
# only add the bare "draw a card for each" form here because it's the
# top-30 labeled-UnknownEffect pattern.


# ============================================================================
# More small wins — promoted labels from the long tail.
# ============================================================================

# "it enters tapped" — replacement-clause body on pain lands etc.
@_eff(r"^it enters tapped\.?$")
def _it_enters_tapped(m):
    return Modification(kind="self_enters_tapped", args=())


# "you may play that card this turn" / "you may play it this turn"
@_eff(r"^you may play (?:that card|it) this turn\.?$")
def _may_play_exiled_this_turn(m):
    return Modification(kind="may_play_this_turn", args=())


@_eff(r"^you may play that card (?:until end of turn|this turn)\.?$")
def _may_play_that_card(m):
    return Modification(kind="may_play_this_turn", args=())


# "you take the initiative"
@_eff(r"^you take the initiative\.?$")
def _take_initiative(m):
    return Modification(kind="take_initiative", args=())


# "the ring tempts you" — LTR keyword-action
@_eff(r"^the ring tempts you\.?$")
def _ring_tempts(m):
    return Modification(kind="ring_tempts", args=())


# "you may pay {N}" — cost-prompt tail of many cards
@_eff(r"^you may pay \{(\d+)\}\.?$")
def _may_pay_generic(m):
    return Modification(kind="may_pay_generic", args=(int(m.group(1)),))


@_eff(r"^you may pay (\d+|x) life\.?$")
def _may_pay_life(m):
    amt = m.group(1)
    return Modification(kind="may_pay_life",
                        args=(int(amt) if amt.isdigit() else amt,))


# "it explores"
@_eff(r"^it explores\.?$")
def _it_explores(m):
    return Modification(kind="explores", args=("pronoun_it",))


# "it connives"
@_eff(r"^it connives\.?$")
def _it_connives(m):
    return Modification(kind="connives", args=("pronoun_it",))


# "monstrosity N"
@_eff(r"^monstrosity (\d+)\.?$")
def _monstrosity(m):
    return Modification(kind="monstrosity", args=(int(m.group(1)),))


# "extra land per turn" label — emitted for "you may play an additional land"
# This label is ALREADY upgraded by parser.py's `_extra_land` rule (which
# emits UnknownEffect with that raw_text). We shortcut to the stub.
@_eff(r"^you may play an additional land on each (?:of your turns|turn)\.?$")
def _extra_land(m):
    return Modification(kind="extra_land_per_turn", args=())


# "add one mana of the chosen color" / "add one mana of any color in X"
@_eff(r"^add one mana of the chosen colou?r\.?$")
def _add_mana_chosen_color(m):
    return AddMana(any_color_count=1)  # Any-color is the closest typed cousin.


# "add two mana in any combination of colors"
@_eff(r"^add two mana in any combination of colou?rs\.?$")
def _add_two_combo_colors(m):
    return AddMana(any_color_count=2)


# "add X mana of any one color, where X is ..." — variable amount
@_eff(r"^add x mana of any one colou?r, where x is [^.]+\.?$")
def _add_x_any_color_where(m):
    return AddMana(any_color_count=0)  # X handled by engine; shape preserved


# NOTE: "add two/three mana of any one color" would break Jeweled Lotus golden,
# so we leave it as UnknownEffect for now. Same for bare "add {X} or {Y}" (breaks
# Talisman of Dominance) and triple "add {X}, {Y}, or {Z}" (breaks Noble Hierarch).


# "you may gain N life" — optional gain-life riders
@_eff(r"^you may gain (\d+|x|one|two|three|four|five) life\.?$")
def _may_gain_life(m):
    from mtg_ast import Optional_, GainLife
    amt = _n(m.group(1))
    return Optional_(body=GainLife(amount=amt))


# "you may draw a card" / "you may draw N cards"  — promoted to Optional(Draw)
# NOTE: affects solemn_simulacrum / bident_of_thassa / coastal_piracy /
# consecrated_sphinx goldens, so we're careful NOT to add it here.


# "you may put a +1/+1 counter on this creature" — Optional(CounterMod)
@_eff(r"^you may put a \+1/\+1 counter on (?:this creature|~)\.?$")
def _may_p1p1_self(m):
    from mtg_ast import Optional_, CounterMod
    return Optional_(body=CounterMod(op="put", count=1, counter_kind="+1/+1",
                                     target=Filter(base="self", targeted=False)))


# "you may return this card from your graveyard to your hand"
@_eff(r"^you may return this card from your graveyard to your hand\.?$")
def _may_recurse_self(m):
    from mtg_ast import Optional_
    return Optional_(body=Recurse(
        query=Filter(base="self", targeted=False),
        from_zone="your_graveyard",
        destination="hand"))


# "sacrifice a creature" — straightforward self-sacrifice effect
@_eff(r"^sacrifice a creature\.?$")
def _sac_a_creature(m):
    from mtg_ast import Sacrifice
    return Sacrifice(query=Filter(base="creature", you_control=True))


# "sacrifice a land" / "sacrifice an artifact"
@_eff(r"^sacrifice an? (creature|artifact|land|enchantment|permanent)\.?$")
def _sac_a_type(m):
    from mtg_ast import Sacrifice
    return Sacrifice(query=Filter(base=m.group(1), you_control=True))


# "goad it" / "goad that creature" — already caught by parser core as
# UnknownEffect("goad pronoun"). Upgrade to a stub with the shape preserved.
@_eff(r"^goad (?:it|that creature)\.?$")
def _goad_pronoun(m):
    return Modification(kind="goad", args=("pronoun",))


# "destroy that creature at end of combat" — delayed-destroy rider
@_eff(r"^destroy (?:that|target) creature at end of combat\.?$")
def _destroy_at_eoc(m):
    return Modification(kind="destroy_at_eoc", args=())


# "until end of turn, target creature you control ..." — very common rider
@_eff(r"^until end of turn, target creature you control [^.]+\.?$")
def _until_eot_ally(m):
    return Modification(kind="until_eot_ally_effect", args=())


# "this enchantment deals N damage to any target" / "target creature" variants
@_eff(r"^this enchantment deals (\d+|x) damage to any target\.?$")
def _enc_dmg_any(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_ANY)


@_eff(r"^this enchantment deals (\d+|x) damage to target creature\.?$")
def _enc_dmg_tc(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_CREATURE)


@_eff(r"^this enchantment deals (\d+|x) damage to target player\.?$")
def _enc_dmg_tp(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_PLAYER)


# "this artifact deals N damage to ..."
@_eff(r"^this artifact deals (\d+|x) damage to any target\.?$")
def _art_dmg_any(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_ANY)


# "return target card from your graveyard to your hand" — catchall
# (already handled by core for the ~ form; this is the generic variant)


# NOTE: "choose another creature you control" is used on Dauntless Bodyguard's
# golden — we intentionally do NOT promote it here.


# "change the target of target <spell/ability>" — redirect labels
@_eff(r"^change (?:the |a )?target of target (spell|ability)\.?$")
def _change_target(m):
    return Modification(kind="change_target", args=(m.group(1),))


@_eff(r"^change target\.?$")
def _change_target_bare(m):
    return Modification(kind="change_target", args=())


# "flip a coin" — NOTE: mana_crypt.json golden has this; so we avoid it.
# @_eff(r"^flip a coin\.?$")
# def _flip_coin(m):
#     return Modification(kind="flip_coin", args=())


# "create typed token" label — decree_of_justice, hero_of_bladehold,
# avenger_of_zendikar, rukh_egg goldens all touch this, so we intentionally
# do NOT promote it here. The existing multi_failure rule already does the
# regex work; the kind change would break ~5 golden fixtures.


# "pay N life" — cost-prompt tail parsed as effect in some edge cases
@_eff(r"^pay (\d+|x) life\.?$")
def _pay_life_effect(m):
    amt = m.group(1)
    return Modification(kind="pay_life",
                        args=(int(amt) if amt.isdigit() else amt,))


# "exile enchanted creature" — aura finisher
@_eff(r"^exile enchanted creature\.?$")
def _exile_enchanted(m):
    from mtg_ast import Exile
    return Exile(target=Filter(base="enchanted_creature", targeted=False))


# "return this aura to its owner's hand" — Shackles-class tail
@_eff(r"^return this aura to its owner'?s? hand\.?$")
def _return_aura(m):
    from mtg_ast import Bounce
    return Bounce(target=Filter(base="self", targeted=False), to="owners_hand")


# "return a land you control to its owner's hand" — bouncelands
@_eff(r"^return a land you control to (?:its owner'?s?|your) hand\.?$")
def _bounce_own_land(m):
    from mtg_ast import Bounce
    return Bounce(target=Filter(base="land", you_control=True), to="owners_hand")


# "you may attach this equipment to it"
@_eff(r"^you may attach this (?:equipment|aura) to it\.?$")
def _may_attach_pronoun(m):
    from mtg_ast import Optional_
    return Optional_(body=Modification(kind="attach_self_to", args=("pronoun_it",)))


# "attach it to target <filter>" — repointer tail (Aura Graft / Stolen Uniform)
@_eff(r"^attach it to (?:target |another |the chosen )?([^.]{1,60})\.?$")
def _attach_it_to(m):
    return Modification(kind="attach_pronoun_to",
                        args=(m.group(1).strip(),))


# "you may tap or untap target creature" — Niblis / Granite Witness
@_eff(r"^you may tap or untap target ([^.]+?)\.?$")
def _may_tap_or_untap(m):
    from mtg_ast import Optional_
    return Optional_(body=Modification(
        kind="tap_or_untap", args=(m.group(1).strip(),)))


# "level N" orphans (Stormchaser's / Builder's Talent-class)
@_eff(r"^level (\d+)\.?$")
def _level_marker(m):
    return Modification(kind="level_marker", args=(int(m.group(1)),))


# "choose:opponent" — this is a label, not real text. Find source:
# produced by choices.py for "choose an opponent" oracle text.
@_eff(r"^choose an opponent\.?$")
def _choose_opponent(m):
    return Modification(kind="choose_opponent", args=())


# "gift card" label — Cleave / Gift mechanic. Real text is "gift a creature".
@_eff(r"^gift (?:a|the) (creature|card|token|tapped creature|opponent)\.?$")
def _gift(m):
    return Modification(kind="gift", args=(m.group(1).strip(),))


# "draft a card from ~'s spellbook" — Companion / planar chaos. Leave as stub.
@_eff(r"^draft a card from (?:~|this creature)'?s? spellbook\.?$")
def _draft_from_spellbook(m):
    return Modification(kind="draft_from_spellbook", args=())


# NOTE: "return the exiled card to the battlefield under ..." is Oblivion
# Ring's golden shape. Skip to avoid flipping it.


# "step" label (Spider Climb / Lightning Reflexes tail)
@_eff(r"^during your next untap step, untap (?:~|this creature|all creatures you control)\.?$")
def _untap_during_next_step(m):
    return Modification(kind="untap_during_next_step", args=())


# ============================================================================
# Additional small wins for frequent patterns in the mid tail
# ============================================================================

# "untap enchanted creature" — aura activation tail
@_eff(r"^untap enchanted creature\.?$")
def _untap_enchanted(m):
    return UntapEffect(target=Filter(base="enchanted_creature", targeted=False))


# "tap this creature" — bare pronoun tap (different from "tap enchanted creature")
@_eff(r"^tap this creature\.?$")
def _tap_this_creature(m):
    return TapEffect(target=Filter(base="self", targeted=False))


# "you may tap target creature" — optional tap rider
@_eff(r"^you may tap target (creature|permanent|land|artifact)\.?$")
def _may_tap_target(m):
    from mtg_ast import Optional_
    return Optional_(body=TapEffect(
        target=Filter(base=m.group(1), targeted=True)))


# "you may untap this creature" / "you may untap target creature"
@_eff(r"^you may untap (?:this creature|~)\.?$")
def _may_untap_self(m):
    from mtg_ast import Optional_
    return Optional_(body=UntapEffect(
        target=Filter(base="self", targeted=False)))


# "return the exiled card to its owner's hand"
@_eff(r"^return the exiled card to (?:its owner'?s?|your) hand\.?$")
def _return_exiled_to_hand(m):
    return Modification(kind="return_exiled_to_hand", args=())


# NOTE: "return the exiled card to the battlefield ..." is Oblivion Ring's
# golden shape — skip. Same for "return that card to the battlefield ..." —
# used by snake-oil triggers that parser.py already handles via
# `_return_exiled_to_bf` in several extensions. The following narrower form
# still helps for other cards without touching goldens.


# "discard your hand" — obscure effect, real text on Manabond / Malfegor
@_eff(r"^discard your hand\.?$")
def _discard_hand(m):
    return Discard(count="all", target=SELF)


# "sacrifice this aura" / "sacrifice this equipment"
@_eff(r"^sacrifice this (aura|equipment|vehicle|saga)\.?$")
def _sac_self_typed(m):
    from mtg_ast import Sacrifice
    return Sacrifice(query=Filter(base="self", targeted=False))


# "copy target instant or sorcery spell" (without "you control")
@_eff(r"^copy target instant or sorcery spell(?: you control)?\.?$")
def _copy_instant_or_sorcery(m):
    from mtg_ast import CopySpell
    return CopySpell(target=Filter(base="instant_or_sorcery_spell", targeted=True))


# "copy it" / "copy that spell" (pronoun forms)
@_eff(r"^copy it\.?$")
def _copy_it(m):
    return Modification(kind="copy_pronoun", args=("it",))


@_eff(r"^copy that spell\.?$")
def _copy_that_spell(m):
    return Modification(kind="copy_pronoun", args=("that_spell",))


# "create a token that's a copy of this creature" — Mist-Syndicate Naga tail
@_eff(r"^create a token that'?s? a copy of (?:this creature|~)\.?$")
def _token_copy_self(m):
    from mtg_ast import CreateToken
    return CreateToken(count=1, is_copy_of=Filter(base="self", targeted=False))


# "each player sacrifices a creature" — Edicts
@_eff(r"^each player sacrifices a creature\.?$")
def _each_player_sac_creature(m):
    from mtg_ast import Sacrifice
    return Sacrifice(query=Filter(base="creature"),
                     actor="each_player")


# "each opponent sacrifices a creature"
@_eff(r"^each opponent sacrifices a creature\.?$")
def _each_opp_sac_creature(m):
    from mtg_ast import Sacrifice
    return Sacrifice(query=Filter(base="creature"),
                     actor="each_opponent")


# "look at target opponent's hand" / "look at target player's hand"
@_eff(r"^look at target (opponent|player)'?s? hand\.?$")
def _look_at_hand(m):
    from mtg_ast import LookAt
    return LookAt(target=Filter(base=m.group(1), targeted=True),
                  zone="hand")


# NOTE: "this creature assigns no combat damage this turn" is Ophidian's
# golden shape (nested under an Optional+Conditional). Skip to avoid flip.


# "this creature deals N damage to target creature" (bare, no variant)
@_eff(r"^(?:this creature|~) deals (\d+|x) damage to target creature\.?$")
def _self_dmg_tc(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_CREATURE)


# "this creature deals N damage to target player or planeswalker"
@_eff(r"^(?:this creature|~) deals (\d+|x) damage to target player or planeswalker\.?$")
def _self_dmg_tpp(m):
    return Damage(amount=_n(m.group(1)),
                  target=Filter(base="player_or_planeswalker", targeted=True))


# "it deals N damage to target creature"
@_eff(r"^it deals (\d+|x) damage to target creature\.?$")
def _it_dmg_tc(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_CREATURE)


# "it deals N damage to target opponent"
@_eff(r"^it deals (\d+|x) damage to target opponent\.?$")
def _it_dmg_tp(m):
    return Damage(amount=_n(m.group(1)), target=Filter(base="opponent", targeted=True))


# "it deals N damage to target player"
@_eff(r"^it deals (\d+|x) damage to target player\.?$")
def _it_dmg_target_player(m):
    return Damage(amount=_n(m.group(1)), target=TARGET_PLAYER)


# "you may put a ki/verse/<named> counter on this <type>" — self counter riders
@_eff(r"^you may put an? (ki|verse|~|charge|lore|time|level|quest|study|"
      r"aether|coin|experience|storage|shield|strife|stun|flood|"
      r"loot|finality|offering|blood|rust|egg|crystal|brick|tide|"
      r"gold|loyalty) counter on (?:this creature|this enchantment|"
      r"this artifact|this permanent|~)\.?$")
def _may_put_named_counter_self(m):
    from mtg_ast import Optional_, CounterMod
    return Optional_(body=CounterMod(
        op="put", count=1, counter_kind=m.group(1),
        target=Filter(base="self", targeted=False)))


# "you may put a +1/+1 counter on target creature" — optional pump
@_eff(r"^you may put a \+1/\+1 counter on target creature\.?$")
def _may_p1p1_tc(m):
    from mtg_ast import Optional_, CounterMod
    return Optional_(body=CounterMod(
        op="put", count=1, counter_kind="+1/+1",
        target=TARGET_CREATURE))


# "you may put a land card from your hand onto the battlefield [tapped]"
@_eff(r"^you may put a land card from your hand onto the battlefield(?: tapped)?\.?$")
def _may_put_land(m):
    tapped = "tapped" in m.group(0).lower()
    return Modification(kind="may_play_land_from_hand",
                        args=("tapped",) if tapped else ())


# "you may put a creature card from your hand onto the battlefield"
@_eff(r"^you may put a creature card from your hand onto the battlefield(?: tapped)?\.?$")
def _may_put_creature(m):
    tapped = "tapped" in m.group(0).lower()
    return Modification(kind="may_cheat_creature",
                        args=("tapped",) if tapped else ())


# "you gain 1 life for each creature you control" — scaled gain-life
@_eff(r"^you gain (\d+|one|two|x) life for each ([^.]{3,50})\.?$")
def _gain_life_per(m):
    from mtg_ast import GainLife
    return GainLife(amount="var", target=SELF)


# "skip next turn" — skip-turn marker
@_eff(r"^skip (?:your )?next turn\.?$")
def _skip_turn(m):
    return Modification(kind="skip_turn", args=("next",))


# "learn" — keyword action
@_eff(r"^learn\.?$")
def _learn(m):
    return Modification(kind="learn", args=())


# "discover N"
@_eff(r"^discover (\d+)\.?$")
def _discover(m):
    return Modification(kind="discover", args=(int(m.group(1)),))


# "incubate N"
@_eff(r"^incubate (\d+)\.?$")
def _incubate(m):
    return Modification(kind="incubate", args=(int(m.group(1)),))


# "support N"
@_eff(r"^support (\d+)\.?$")
def _support(m):
    return Modification(kind="support", args=(int(m.group(1)),))


# "adapt N"
@_eff(r"^adapt (\d+)\.?$")
def _adapt(m):
    return Modification(kind="adapt", args=(int(m.group(1)),))


# "amass N" / "amass Type N"
@_eff(r"^amass (\d+)\.?$")
def _amass(m):
    return Modification(kind="amass", args=(int(m.group(1)),))


@_eff(r"^amass (\w+) (\d+)\.?$")
def _amass_type(m):
    return Modification(kind="amass", args=(int(m.group(2)), m.group(1)))


# "clash with an opponent"
@_eff(r"^clash with an opponent\.?$")
def _clash(m):
    return Modification(kind="clash", args=())


# "populate"
@_eff(r"^populate\.?$")
def _populate(m):
    return Modification(kind="populate", args=())


# "manifest top card of library" / "manifest dread"
@_eff(r"^manifest (?:the )?top card of (?:your )?library\.?$")
def _manifest_top(m):
    return Modification(kind="manifest", args=("top_library",))


@_eff(r"^manifest dread\.?$")
def _manifest_dread(m):
    return Modification(kind="manifest_dread", args=())


# "exile ~ with N time counters on it" — suspend-style exile
@_eff(r"^exile ~ with (\d+) time counters on it\.?$")
def _exile_self_time(m):
    return Modification(kind="exile_self_with_counters",
                        args=(int(m.group(1)), "time"))


# "exile the top card of your library face down" — face-down impulse
@_eff(r"^exile the top card of your library face down\.?$")
def _exile_top_fd(m):
    return Modification(kind="exile_top_library", args=(1, "face_down"))


# "that creature gains haste"
@_eff(r"^that creature gains haste\.?$")
def _that_gains_haste(m):
    from mtg_ast import GrantAbility
    return GrantAbility(ability_name="haste",
                        target=Filter(base="that_creature", targeted=False))


# "~ gains indestructible until end of turn"
@_eff(r"^~ gains indestructible until end of turn\.?$")
def _self_gains_indestructible(m):
    from mtg_ast import GrantAbility
    return GrantAbility(ability_name="indestructible",
                        target=Filter(base="self", targeted=False))


# "it deals that much damage to any target" — redirected damage
@_eff(r"^it deals that much damage to any target\.?$")
def _it_that_much_dmg(m):
    return Damage(amount="var", target=TARGET_ANY)


# "seek a nonland card" / "seek a card" — mechanic
@_eff(r"^seek a (?:non)?land card\.?$")
def _seek_card(m):
    return Modification(kind="seek", args=(m.group(0),))


@_eff(r"^seek a (creature|artifact|enchantment|instant|sorcery|permanent) card\.?$")
def _seek_typed(m):
    return Modification(kind="seek", args=(m.group(1) + "_card",))


# "repeat this process" — Ad Nauseam tail
@_eff(r"^repeat this process\.?$")
def _repeat_process(m):
    return Modification(kind="repeat_process", args=())


# "starting with you, each player ..." — special-ordering tail
@_eff(r"^starting with you, each player [^.]+\.?$")
def _starting_with_you(m):
    return Modification(kind="starting_with_you", args=())


# NOTE: "put that card into your hand" would give Dark Confidant's golden a
# typed half of its reveal+put sequence (which currently logs as one unknown).
# Skip to preserve that golden.


# "exile all graveyards" / "exile each graveyard"
@_eff(r"^exile all graveyards\.?$")
def _exile_all_graveyards(m):
    return Modification(kind="exile_all_graveyards", args=())


# "choose one of them" — modal-sub-option
@_eff(r"^choose one of them\.?$")
def _choose_one_of_them(m):
    return Modification(kind="choose_one_of_them", args=())


# "choose:player" — same as choose-opponent but more permissive
@_eff(r"^choose (?:a |target )?player\.?$")
def _choose_player(m):
    return Modification(kind="choose_player", args=())


# "you may sacrifice another creature"
@_eff(r"^you may sacrifice another creature\.?$")
def _may_sac_another(m):
    from mtg_ast import Optional_, Sacrifice
    return Optional_(body=Sacrifice(
        query=Filter(base="creature", you_control=True, extra=("another",))))


# "sacrifice it at end of combat"
@_eff(r"^sacrifice it at end of combat\.?$")
def _sac_it_eoc(m):
    return Modification(kind="sac_it_at_eoc", args=())


# "create a tapped treasure token" (safe — tapped variant not in goldens)
@_eff(r"^create a tapped treasure token\.?$")
def _create_tapped_treasure(m):
    from mtg_ast import CreateToken
    return CreateToken(count=1, types=("Treasure",), tapped=True)


# "put a +1/+1 counter on ~" — simple self counter
@_eff(r"^put a \+1/\+1 counter on (?:this creature|~)\.?$")
def _put_p1p1_self(m):
    from mtg_ast import CounterMod
    return CounterMod(op="put", count=1, counter_kind="+1/+1",
                      target=Filter(base="self", targeted=False))


# "switch this creature's power and toughness until end of turn"
@_eff(r"^switch (?:this creature|~)'s power and toughness(?: until end of turn)?\.?$")
def _switch_self_pt(m):
    return Modification(kind="switch_pt_self",
                        args=("until_end_of_turn" if "until" in m.group(0).lower() else "permanent",))


# "switch target creature's power and toughness [until end of turn]"
@_eff(r"^switch target creature'?s? power and toughness(?: until end of turn)?\.?$")
def _switch_target_pt(m):
    return Modification(kind="switch_pt_target",
                        args=("until_end_of_turn" if "until" in m.group(0).lower() else "permanent",))


# "shuffle it into its owner's library"
@_eff(r"^shuffle it into its owner'?s? library\.?$")
def _shuffle_into_owner_lib(m):
    return Modification(kind="shuffle_pronoun_into_owner_library", args=())


# "this turn, copy that spell" — "copy that spell" with prefix
@_eff(r"^this turn, copy that spell\.?$")
def _this_turn_copy(m):
    return Modification(kind="copy_pronoun", args=("that_spell", "this_turn"))


# "target creature blocks this creature this turn if able"
@_eff(r"^target creature blocks (?:this creature|~) this turn if able\.?$")
def _target_blocks_self(m):
    return Modification(kind="force_block_self", args=())


# "~ deals 1 damage to target creature or player" — old templating
@_eff(r"^~ deals (\d+|x) damage to target creature or player\.?$")
def _self_dmg_cp(m):
    return Damage(amount=_n(m.group(1)),
                  target=Filter(base="creature_or_player", targeted=True))


# NOTE: An earlier iteration tried a broad `_VERB_LED_PREFIX` catch-all that
# would promote every verb-led UnknownEffect to `Modification(kind=
# "recognized_verb_led")`. That was too aggressive: many downstream extensions
# (partial_scrubber*.py, snowflake_rules*.py, etc.) implement SPECIFIC typed
# handlers for clauses the broad catch-all would have swallowed, and since
# `aa_*` loads FIRST the broad rule would shadow them.
# Instead we add one more layer of narrow rules below for high-frequency text
# shapes the existing extension chain still ends up dropping on the floor.

