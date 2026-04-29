#!/usr/bin/env python3
"""UNPARSED-bucket residual patterns.

Family: UNPARSED → GREEN/PARTIAL promotions. The base parser plus the 24
existing extensions still leave ~1,693 cards (5.35%) where ZERO abilities
parse. These cards typically have a single strange phrasing the existing
grammar productions don't touch.

This file targets the biggest first-five-word clusters in that residual
bucket. We grouped 1,693 cards by their leading phrase and picked clusters
of ≥3 cards (and a few high-impact singletons). Coverage targets:

  * "choose one or both / one or more" modal headers without bullets that
    survive consolidation                              (~50 cards)
  * "commander creatures you own have ..." / "X creatures you control
    have ..." / "X creatures you control get ..."     (~80 cards)
  * Tribal/typed anthems: "all <type> creatures get/have ..."         (~25)
  * "all creatures lose <kw>" / "all permanents are <color>"           (~15)
  * "until end of turn, creatures you control get/gain ..."             (~25)
  * "<color> creatures you control get/have ..."                        (~15)
  * "regenerate target creature" (bare) — Death Ward / Mending Touch     (~5)
  * "target spell or permanent becomes <color>" — laces                  (~5)
  * "this creature enters with your choice of <counter> or <counter>" (~6)
  * Body effects:
      - "return all <filter> to their owners' hands" (Inundate-style)   (~12)
      - "return one or two target creatures to ..." / "return up to N
         target creatures to ..."                                       (~15)
      - "destroy two target <type>" / "destroy N target ..."            (~10)
      - "exile all <filter> permanents/creatures/cards"                  (~10)
      - "target player draws N cards (and loses N life)"                 (~10)
      - "target opponent exiles a creature they control"                 (~5)
      - "target player gains control of target permanent you control"   (~3)
      - "target player skips their next draw step"                       (~3)
      - "discard <filter> card: this creature gets +N/+N ..." activated (~5)
      - "<self> can't <action> as long as <cond>" / "<self> can't
        <action> if/unless ..."                                          (~8)

Three export tables are merged by ``parser.load_extensions``:

    STATIC_PATTERNS  — ability-level static / anthem shapes
    EFFECT_RULES     — body-level effect productions (parse_effect)
    TRIGGER_PATTERNS — empty here (handled by niche_triggers.py et al.)

Ordering is specific-first within each list. We chose UnknownEffect /
generic Static(Modification(kind=...)) for shapes the AST doesn't model
precisely so the ability gets recorded (promotes UNPARSED → at least
PARTIAL) without inventing semantics that downstream rule extraction
wouldn't trust.
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
    Filter, GainControl, GrantAbility, Keyword, LoseLife, Modification,
    Sacrifice, Sequence, Static, UnknownEffect,
    TARGET_ANY, TARGET_CREATURE, TARGET_OPPONENT, TARGET_PLAYER,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_SELF = (
    r"(?:~|this creature|this permanent|this land|this artifact|"
    r"this enchantment|this card|this vehicle)"
)

_COLOR = (
    r"(?:white|blue|black|red|green|colorless|multicolored|monocolored)"
)

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}


def _num(tok: str):
    tok = (tok or "").strip().lower()
    if tok.isdigit():
        return int(tok)
    return _NUM_WORDS.get(tok, tok)


# ===========================================================================
# STATIC_PATTERNS — ability-level static / anthem shapes
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Modal header orphans that survived consolidation. The splitter sometimes
# leaves the bare header on a line by itself when bullets used `;` instead of
# newlines. We turn those into a `modal_header_orphan` Static so the card
# at least has SOMETHING parsed (promoting UNPARSED → PARTIAL). The bullets
# become separate abilities that the base parser handles normally.
# ---------------------------------------------------------------------------

@_sp(r"^choose (one or both|one or more|two or more|any number)\s*[—-]?\s*$")
def _choose_orphan(m, raw):
    return Static(modification=Modification(kind="modal_header_orphan",
                                            args=(m.group(1).lower(),)),
                  raw=raw)


# "choose one or both — • <bullet1> ; • <bullet2>" — inline modal that
# survived consolidation (bullets joined with `;` instead of newlines).
# Core has an equivalent pattern for plain "choose one|two|three" but it
# doesn't cover the "or both" / "or more" headers.
@_sp(r"^choose (?:one or both|one or more|two or more|any number) [—-] [•·]")
def _inline_modal_or_both(m, raw):
    return Static(modification=Modification(kind="inline_modal_with_bullets",
                                            args=(raw,)),
                  raw=raw)


# "Choose exactly two creatures you control" — Spry and Mighty-class
@_sp(r"^choose exactly (one|two|three|four|\d+) ([^.]+?)\s*$")
def _choose_exactly(m, raw):
    return Static(modification=Modification(kind="choose_exactly",
                                            args=(m.group(1), m.group(2).strip())),
                  raw=raw)


# "For each player, choose friend or foe" — Pir's Whim / Council's Dilemma
@_sp(r"^for each player, choose ([^.]+?)\s*$")
def _per_player_choice(m, raw):
    return Static(modification=Modification(kind="per_player_choice",
                                            args=(m.group(1).strip(),)),
                  raw=raw)


# "Choose odd or even" — Extinction Event-style
@_sp(r"^choose (odd or even|a number|a creature type|a card type|a color)\s*$")
def _choose_quality(m, raw):
    return Static(modification=Modification(kind="choose_quality",
                                            args=(m.group(1).lower(),)),
                  raw=raw)


# ---------------------------------------------------------------------------
# Tribal / typed anthems and overrides. Core has "creatures you control" and
# "other <type> you control"; these add the missing shapes.
# ---------------------------------------------------------------------------

# "<type> creatures you control have <kw>" — Slivers, Allies, etc.
# Examples: "sliver creatures you control have flying",
#           "rats you control have deathtouch"
@_sp(r"^([a-z][a-z\- ]+?) creatures? you control have ([^.]+?)\s*$")
def _typed_have(m, raw):
    typ = m.group(1).strip()
    kws = m.group(2).strip()
    return Static(modification=Modification(kind="typed_anthem_have",
                                            args=(typ, kws)),
                  raw=raw)


# "~ creatures you control have <kw>" / "~s you control have <kw>" — when
# the card's NAME is the tribe (Sliver Hivelord, Goblin Chieftain in some
# printings, etc.). The base parser's `~` substitution leaves a literal
# tilde where the type would be.
@_sp(r"^~s? you control have ([^.]+?)\s*$")
def _self_named_tribe_have(m, raw):
    return Static(modification=Modification(kind="self_named_tribe_have",
                                            args=(m.group(1).strip(),)),
                  raw=raw)


@_sp(r"^~s? creatures? you control have ([^.]+?)\s*$")
def _self_named_creature_have(m, raw):
    return Static(modification=Modification(kind="self_named_tribe_have",
                                            args=(m.group(1).strip(),)),
                  raw=raw)


@_sp(r"^~s? creatures? you control get \+(\d+)/\+(\d+)\s*$")
def _self_named_creature_get(m, raw):
    return Static(modification=Modification(kind="self_named_tribe_get",
                                            args=(int(m.group(1)),
                                                  int(m.group(2)))),
                  raw=raw)


# "<type> creatures you control get +N/+N (until end of turn)?"
@_sp(r"^([a-z][a-z\- ]+?) creatures? you control get \+(\d+)/\+(\d+)"
     r"(?: until end of turn)?\s*$")
def _typed_get(m, raw):
    typ = m.group(1).strip()
    return Static(modification=Modification(kind="typed_anthem_get",
                                            args=(typ, int(m.group(2)), int(m.group(3)))),
                  raw=raw)


# "<color> creatures you control get +N/+N (until end of turn)?"
# (Color words trip the previous pattern via the broader "[a-z]+" group, but
# we still want them stamped with a clean kind so downstream code can tell
# tribal-vs-color anthems apart.)
@_sp(rf"^({_COLOR}|non{_COLOR}) creatures? you control get \+(\d+)/\+(\d+)"
     r"(?: until end of turn)?\s*$")
def _color_anthem_get(m, raw):
    return Static(modification=Modification(kind="color_anthem_get",
                                            args=(m.group(1), int(m.group(2)),
                                                  int(m.group(3)))),
                  raw=raw)


# "<color> creatures you control have <kw>"
@_sp(rf"^({_COLOR}|non{_COLOR}) creatures? you control have ([^.]+?)\s*$")
def _color_anthem_have(m, raw):
    return Static(modification=Modification(kind="color_anthem_have",
                                            args=(m.group(1), m.group(2).strip())),
                  raw=raw)


# "all <type> creatures get +N/+N" / "all <type> creatures have <kw>" / bare
# "all <type> creatures have <kw>" — global tribal anthem (Muscle Sliver,
# Winged Sliver-class lords printed before "you control" templating).
@_sp(r"^all ([a-z][a-z\- ]+?) creatures? get \+(\d+)/\+(\d+)\s*$")
def _all_typed_get(m, raw):
    return Static(modification=Modification(kind="global_typed_anthem_get",
                                            args=(m.group(1).strip(),
                                                  int(m.group(2)), int(m.group(3)))),
                  raw=raw)


@_sp(r"^all ([a-z][a-z\- ]+?) creatures? have ([^.]+?)\s*$")
def _all_typed_have(m, raw):
    return Static(modification=Modification(kind="global_typed_anthem_have",
                                            args=(m.group(1).strip(),
                                                  m.group(2).strip())),
                  raw=raw)


# "all creatures lose <kw>" / "all creatures lose <kw> and <kw>"
# Mystic Decree, Gravity Sphere
@_sp(r"^all creatures lose ([^.]+?)\s*$")
def _all_creatures_lose(m, raw):
    return Static(modification=Modification(kind="global_lose_kw",
                                            args=(m.group(1).strip(),)),
                  raw=raw)


# "all creatures get +N/+N" / "all creatures get -N/-N (until end of turn)?"
@_sp(r"^all creatures get ([+-]\d+|[+-]x)/([+-]\d+|[+-]x)"
     r"(?: until end of turn)?\s*$")
def _all_creatures_get(m, raw):
    return Static(modification=Modification(kind="global_creatures_get",
                                            args=(m.group(1), m.group(2))),
                  raw=raw)


# "all creatures have <kw>"
@_sp(r"^all creatures have ([^.]+?)\s*$")
def _all_creatures_have(m, raw):
    return Static(modification=Modification(kind="global_creatures_have",
                                            args=(m.group(1).strip(),)),
                  raw=raw)


# "all permanents are <color>" / "all permanents are colorless" — Thran Lens
@_sp(r"^all permanents are ([a-z][a-z ]+?)\s*$")
def _all_permanents_are(m, raw):
    return Static(modification=Modification(kind="all_permanents_color",
                                            args=(m.group(1).strip(),)),
                  raw=raw)


# "commander creatures you own have <X>" / "commander creatures you own have
# base power and toughness N/N and ..." — Background-style enchantments.
@_sp(r"^commander creatures you own have ([^.]+?)\s*$")
def _commander_anthem(m, raw):
    return Static(modification=Modification(kind="commander_anthem",
                                            args=(m.group(1).strip(),)),
                  raw=raw)


# ---------------------------------------------------------------------------
# Self-static shapes the base parser doesn't catch.
# ---------------------------------------------------------------------------

# "this creature enters with your choice of a <counter> counter or a
# <counter> counter on it" / "...your choice of two different counters on it
# from among <list>" — Flycatcher Giraffid, Ferocious Tigorilla, Grimdancer.
@_sp(r"^(?:this creature|~) enters with your choice of [^.]+ on it\s*$")
def _enters_choice_counter(m, raw):
    return Static(modification=Modification(kind="enters_with_counter_choice"),
                  raw=raw)


# "you can't cast ~ if you've played a land this turn" / "you can't cast ~
# unless you've ..." — Rock Jockey-style cast restrictions.
@_sp(r"^you can'?t cast ~ (?:if|unless) [^.]+\s*$")
def _cant_cast_if(m, raw):
    return Static(modification=Modification(kind="cast_self_restriction"),
                  raw=raw)


# "you can't play lands if this creature was cast this turn" — companion to
# Rock Jockey's second line.
@_sp(r"^you can'?t play lands? (?:if|unless) [^.]+\s*$")
def _cant_play_lands_if(m, raw):
    return Static(modification=Modification(kind="land_play_restriction"),
                  raw=raw)


# "each creature you control can't be blocked as long as <cond>" —
# Tanglewalker-style conditional unblockable.
@_sp(r"^each creature you control can'?t be blocked as long as [^.]+\s*$")
def _ally_unblockable_cond(m, raw):
    return Static(modification=Modification(kind="ally_unblockable_cond"),
                  raw=raw)


# Bare keyword one-liners the partial_scrubber dictionary missed. Older
# Mirrodin-block keywords / new mechanics that arrive as a single token.
_BARE_NICHE_KEYWORDS = {
    "demonstrate", "rebound", "modular-sunburst", "grazing type",
    "undaunted", "sunburst", "fateful hour", "kinship",
}


@_sp(r"^([a-z][a-z' \-!]+?)\s*$")
def _bare_niche_keyword(m, raw):
    word = m.group(1).strip().lower()
    if word in _BARE_NICHE_KEYWORDS:
        return Keyword(name=word, raw=raw)
    return None


# "instant and sorcery spells you control have <kw>" — Cast Through Time
@_sp(r"^instant and sorcery spells you control have ([^.]+?)\s*$")
def _instant_sorcery_have(m, raw):
    return Static(modification=Modification(kind="spell_type_anthem",
                                            args=("instant_or_sorcery",
                                                  m.group(1).strip())),
                  raw=raw)


# "creatures with <kw> get +N/+N" / "creatures with <kw> get -N/-N" /
# "creatures without <kw> get -N/-N" — Gravitational Shift-class.
@_sp(r"^creatures (with|without) ([a-z]+) get ([+-]\d+)/([+-]\d+)\s*$")
def _filter_kw_anthem(m, raw):
    sense = m.group(1).lower()
    kw = m.group(2).strip()
    return Static(modification=Modification(kind="filter_kw_anthem",
                                            args=(sense, kw,
                                                  int(m.group(3)),
                                                  int(m.group(4)))),
                  raw=raw)


# ===========================================================================
# EFFECT_RULES — body effect productions
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Bounce — broader templating than the base parser handles.
# ---------------------------------------------------------------------------

# "return all <filter> to their owners' hands" — Inundate, Aether Gale-class.
@_er(r"^return all ([^.]+?) to (?:their|its) owners?'? hands?(?:\.|$)")
def _return_all_to_hand(m):
    return Bounce(target=Filter(base=m.group(1).strip(), targeted=False),
                  to="owners_hand")


# "return all <filter> on the battlefield and all <filter> in graveyards to
# their owners' hands" — Soulquake-class compound bounce.
@_er(r"^return all ([^.]+?) on the battlefield and all ([^.]+?) in graveyards "
     r"to (?:their|its) owners?'? hands?(?:\.|$)")
def _return_all_compound(m):
    return Bounce(target=Filter(base=m.group(1).strip() + "+gy:" + m.group(2).strip(),
                                targeted=False),
                  to="owners_hand")


# "return one or two target creatures to their owners' hands" /
# "return up to two target <filter> to ..." / "return up to N target ..."
@_er(r"^return (one or two|up to (?:one|two|three|four|five|\d+)|two) target "
     r"([^.]+?) to (?:their|its) owners?'? hands?(?:\.|$)")
def _return_n_target_to_hand(m):
    qty = m.group(1).strip()
    return Bounce(target=Filter(base=m.group(2).strip(), targeted=True,
                                extra=(qty,)),
                  to="owners_hand")


# "return target permanent to its owner's hand" / "return target X to its
# owner's hand" — bare singular form (the base parser handles "target
# creature" specifically; broaden to any noun phrase).
@_er(r"^return target ([^.]+?) to (?:its|their) owners?'? hands?(?:\.|$)")
def _return_single_to_hand(m):
    return Bounce(target=Filter(base=m.group(1).strip(), targeted=True),
                  to="owners_hand")


# "return each creature without a +1/+1 counter on it to its owner's hand"
# Wave Goodbye-class filtered bounce.
@_er(r"^return each ([^.]+?) to (?:its|their) owners?'? hands?(?:\.|$)")
def _return_each_to_hand(m):
    return Bounce(target=Filter(base=m.group(1).strip(), targeted=False),
                  to="owners_hand")


# ---------------------------------------------------------------------------
# Destroy/Exile — quantified targeting that the base parser misses.
# ---------------------------------------------------------------------------

# "destroy two target <filter>" / "destroy three target <filter>"
@_er(r"^destroy (two|three|four|\d+) target ([^.]+?)(?:\.|$)")
def _destroy_n_target(m):
    return Destroy(target=Filter(base=m.group(2).strip(), targeted=True,
                                 extra=(m.group(1).strip(),)))


# "destroy each <filter>" — sweeper variant. Base parser has "destroy all";
# we add the rarer "destroy each" phrasing.
@_er(r"^destroy each ([^.]+?)(?:\.|$)")
def _destroy_each(m):
    return Destroy(target=Filter(base=m.group(1).strip(), targeted=False,
                                 extra=("each",)))


# "exile all <filter>" — Ravnica at War, Identity Crisis-class.
@_er(r"^exile all ([^.]+?)(?:\.|$)")
def _exile_all(m):
    return Exile(target=Filter(base=m.group(1).strip(), targeted=False))


# "exile all cards from target player's hand and graveyard" — Identity Crisis.
@_er(r"^exile all cards from target player'?s? ([^.]+?)(?:\.|$)")
def _exile_all_zone(m):
    return Exile(target=Filter(base="cards_from_" + m.group(1).strip(),
                               targeted=True))


# "exile each creature with mana value <X>" — Extinction Event tail.
@_er(r"^exile each ([^.]+?)(?:\.|$)")
def _exile_each(m):
    return Exile(target=Filter(base=m.group(1).strip(), targeted=False,
                               extra=("each",)))


# ---------------------------------------------------------------------------
# Direct-targeted player effects.
# ---------------------------------------------------------------------------

# "target player draws X cards" / "target player draws N cards" / "target
# player draws N cards and loses N life" — Monumental Corruption, etc.
@_er(r"^target player draws (a|one|two|three|four|five|x|\d+) cards?"
     r"(?: and loses (\d+|x) life)?(?:\.|$)")
def _tplayer_draws_optionally_loses(m):
    n = _num(m.group(1))
    if m.group(2) is None:
        return Draw(count=n, target=TARGET_PLAYER)
    life = _num(m.group(2))
    return Sequence(items=(
        Draw(count=n, target=TARGET_PLAYER),
        LoseLife(amount=life, target=TARGET_PLAYER),
    ))


# "target opponent discards N cards" / "target opponent discards N cards,
# mills N cards, and loses N life" — Demogorgon's Clutches-class.
@_er(r"^target opponent discards (a|one|two|three|four|five|x|\d+) cards?"
     r"(?:,? mills (a|one|two|three|four|five|x|\d+) cards?)?"
     r"(?:,? and loses (\d+|x) life)?(?:\.|$)")
def _topp_discards(m):
    parts = [Discard(count=_num(m.group(1)), target=TARGET_OPPONENT,
                     chosen_by="discarder")]
    if m.group(2):
        # Mill is on the opponent — we don't import Mill, model as UnknownEffect.
        parts.append(UnknownEffect(raw_text=f"target opponent mills {m.group(2)}"))
    if m.group(3):
        parts.append(LoseLife(amount=_num(m.group(3)), target=TARGET_OPPONENT))
    if len(parts) == 1:
        return parts[0]
    return Sequence(items=tuple(parts))


# "target player skips their next draw step" / "target player skips their
# next <phase>" — Fatigue / Time Walk-edict.
@_er(r"^target player skips their next ([a-z ]+? step)(?:\.|$)")
def _tplayer_skip_phase(m):
    return UnknownEffect(raw_text="skip " + m.group(1).strip())


# "target player gains control of target permanent you control" / "target
# player gains control of target <filter>" — Donate.
@_er(r"^target player gains control of target ([^.]+?)(?:\.|$)")
def _donate(m):
    return GainControl(target=Filter(base=m.group(1).strip(), targeted=True))


# "target opponent exiles a creature they control" / "target opponent
# exiles a creature they control and their graveyard" — Strategic Betrayal.
@_er(r"^target opponent exiles ([^.]+?)(?:\.|$)")
def _topp_exiles(m):
    return UnknownEffect(raw_text="opponent self-exile: " + m.group(1).strip())


# "target player shuffles up to N target cards from their graveyard into
# their library" — Dwell on the Past-class.
@_er(r"^target player shuffles up to (one|two|three|four|five|\d+) target "
     r"cards? from (?:their|his or her) graveyard into (?:their|his or her) "
     r"library(?:\.|$)")
def _tplayer_shuffle_gy(m):
    return UnknownEffect(raw_text="player-shuffle gy back: " + m.group(1))


# "put up to N target cards from an opponent's graveyard on top of their
# library in any order" — Misinformation-class.
@_er(r"^put up to (one|two|three|four|five|\d+) target cards? from "
     r"(?:an opponent'?s?|target player'?s?) graveyard on top of "
     r"(?:their|his or her) library(?: in any order)?(?:\.|$)")
def _put_gy_on_top(m):
    return UnknownEffect(raw_text="put gy cards on top: " + m.group(1))


# ---------------------------------------------------------------------------
# Modal-with-bullets that survived consolidation. The splitter sometimes
# leaves "choose one or both —" on its own line (because bullets used `;`),
# so we promote bare bullet lines on Choice cards to standalone effects.
# ---------------------------------------------------------------------------

# "regenerate target creature" — bare Death Ward / Mending Touch.
@_er(r"^regenerate target creature(?:\.|$)")
def _regen_target(m):
    return UnknownEffect(raw_text="regenerate target creature")


# "regenerate target <filter>" — broader regenerate bodies.
@_er(r"^regenerate target ([^.]+?)(?:\.|$)")
def _regen_target_filtered(m):
    return UnknownEffect(raw_text="regenerate " + m.group(1).strip())


# ---------------------------------------------------------------------------
# Color-changing / type-changing one-liners.
# ---------------------------------------------------------------------------

# "target spell or permanent becomes <color>" — Chaoslace, Purelace,
# Moonlace, Thoughtlace, Deathlace, Circle of Affliction.
@_er(rf"^target (?:spell or permanent|permanent|spell) becomes ({_COLOR})"
     r"(?:\.|$)")
def _lace_color(m):
    return UnknownEffect(raw_text="lace " + m.group(1).lower())


# "target creature becomes a <type>" / "target creature becomes a <type>
# until end of turn" — bare type-bestow.
@_er(r"^target creature becomes (?:a|an) ([^.]+?)(?: until end of turn)?(?:\.|$)")
def _target_becomes_type(m):
    return UnknownEffect(raw_text="becomes " + m.group(1).strip())


# ---------------------------------------------------------------------------
# Power/toughness override and pumps the base parser doesn't catch.
# ---------------------------------------------------------------------------

# "target creature has base power and toughness N/N until end of turn" —
# Square Up, Burst of Strength.
@_er(r"^target creature has base power and toughness (\d+)/(\d+)"
     r"(?: until end of turn)?(?:\.|$)")
def _target_base_pt(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=TARGET_CREATURE)


# "target creature gets +X/+0 until end of turn, where x is ..." /
# "target creature gets +x/+x until end of turn" — variable pumps.
@_er(r"^target creature gets ([+-]x)/([+-]\d+|[+-]x) until end of turn"
     r"(?:,? where x is [^.]+)?(?:\.|$)")
def _target_var_pump(m):
    return Buff(power=m.group(1), toughness=m.group(2), target=TARGET_CREATURE)


# "target creature you control gets +N/+N until end of turn" — controller-
# scoped pump (base parser handles bare "target creature" only).
@_er(r"^target creature you control gets ([+-]\d+)/([+-]\d+)"
     r"(?: until end of turn)?(?:\.|$)")
def _target_ally_pump(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", targeted=True,
                              extra=("you_control",)))


# "two target creatures each get -N/-N until end of turn" — split-pump
# templating.
@_er(r"^two target creatures each get ([+-]\d+)/([+-]\d+) until end of turn"
     r"(?:\.|$)")
def _two_target_pump(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", targeted=True,
                              extra=("two",)))


# ---------------------------------------------------------------------------
# "blocking creatures gain <kw> until end of turn" /
# "attacking creatures gain <kw> until end of turn" — combat-state grant.
# ---------------------------------------------------------------------------

@_er(r"^(blocking|attacking) creatures gain ([^.]+?) until end of turn(?:\.|$)")
def _combat_state_grant(m):
    return GrantAbility(ability_name=m.group(2).strip(),
                        target=Filter(base=m.group(1).lower() + "_creatures",
                                      targeted=False))


# ---------------------------------------------------------------------------
# "creatures you control get +N/+N until end of turn" / "...gain <kw>
# until end of turn" — Overrun-class one-shot anthems on instants/sorceries.
# Distinct from the static anthem parsed by parse_static.
# ---------------------------------------------------------------------------

@_er(r"^creatures you control get \+(\d+)/\+(\d+) until end of turn(?:\.|$)")
def _your_creatures_pump(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", targeted=False,
                              extra=("you_control",)))


@_er(r"^creatures you control gain ([^.]+?) until end of turn(?:\.|$)")
def _your_creatures_grant(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="creature", targeted=False,
                                      extra=("you_control",)))


# "until end of turn, creatures you control get +N/+N (and gain <kws>)" —
# Volatile Claws / Titanic Ultimatum / Overrun-style with the duration first.
@_er(r"^until end of turn, creatures you control get \+(\d+)/\+(\d+)"
     r"(?: and gain ([^.]+?))?(?:\.|$)")
def _eot_creatures_pump(m):
    pump = Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", targeted=False,
                              extra=("you_control",)))
    if m.group(3):
        return Sequence(items=(pump, GrantAbility(
            ability_name=m.group(3).strip(),
            target=Filter(base="creature", targeted=False,
                          extra=("you_control",)))))
    return pump


# "until end of turn, creatures you control gain <kw>" / "...gain <X>" —
# Showstopper-class duration-first grant.
@_er(r"^until end of turn, creatures you control gain ([^.]+?)(?:\.|$)")
def _eot_creatures_grant(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="creature", targeted=False,
                                      extra=("you_control",)))


# ---------------------------------------------------------------------------
# Compound spell bodies "you draw N cards, lose N life, and get {e}{e}" —
# Live Fast / Strange Insight-style.
# ---------------------------------------------------------------------------

@_er(r"^you draw (a|one|two|three|four|five|x|\d+) cards?,?"
     r"(?: lose (\d+|x) life)?(?:,? and get \{[^}]+\}(?:\{[^}]+\})*)?\s*"
     r"(?:\.|$)")
def _live_fast(m):
    n = _num(m.group(1))
    parts = [Draw(count=n)]
    if m.group(2):
        parts.append(LoseLife(amount=_num(m.group(2))))
    if len(parts) == 1:
        return parts[0]
    return Sequence(items=tuple(parts))


# ---------------------------------------------------------------------------
# Token creation with computed count.
# ---------------------------------------------------------------------------

# "create a number of <type> tokens equal to ..." — Goblin Gathering-class.
@_er(r"^create a number of ([^.]+?) creature tokens? equal to ([^.]+?)(?:\.|$)")
def _create_n_equal(m):
    # CreateToken has no "count equals X" structural slot; use UnknownEffect
    # with the raw shape so the ability is recorded but downstream code
    # doesn't mistake a variable count for a literal one.
    return UnknownEffect(raw_text=f"create N {m.group(1).strip()} tokens equal to {m.group(2).strip()}")


# ---------------------------------------------------------------------------
# Additional residuals discovered after first measurement pass.
# ---------------------------------------------------------------------------

# "search your library for up to N <filter>, put them onto the battlefield
# (tapped)?, then shuffle" — Map the Frontier / Explore the Underdark / Cultivate
# (the base parser handles "search ... for a", but not "up to two").
@_er(r"^search your library for up to (one|two|three|four|five|\d+) "
     r"([^,.]+?), put (?:them|it) onto the battlefield(?: tapped)?,? "
     r"then shuffle(?:\.|$)")
def _search_up_to_n(m):
    return UnknownEffect(raw_text=f"search lib up to {m.group(1)} {m.group(2).strip()}")


# "search your library for up to N <filter> and put them into your hand,
# then shuffle"
@_er(r"^search your library for up to (one|two|three|four|five|\d+) "
     r"([^,.]+?),? (?:and put|put) (?:them|it) into your hand,? "
     r"then shuffle(?:\.|$)")
def _search_up_to_n_hand(m):
    return UnknownEffect(raw_text=f"search lib to hand up to {m.group(1)} {m.group(2).strip()}")


# "one or more target creatures become <color> until end of turn" —
# Touch of Darkness / Dwarven Song / Whim of Volrath.
@_er(rf"^one or more target creatures become ({_COLOR})"
     r"(?: until end of turn)?(?:\.|$)")
def _multi_become_color(m):
    return UnknownEffect(raw_text=f"multi-target become {m.group(1).lower()}")


# "creatures of the creature type of your choice get +/-N/+/-N until end
# of turn" — Tribal Unity / Witch's Vengeance.
@_er(r"^creatures of the creature type of your choice get "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x) until end of turn(?:\.|$)")
def _chosen_tribe_pump(m):
    return UnknownEffect(raw_text=f"chosen-tribe pump {m.group(1)}/{m.group(2)}")


# "creatures without flying can't block this turn" — sweeper-rider /
# Magmatic Chasm. Body effect on a sorcery.
@_er(r"^creatures without ([a-z ]+?) can'?t block this turn(?:\.|$)")
def _without_kw_cant_block(m):
    return UnknownEffect(raw_text=f"creatures without {m.group(1).strip()} can't block")


# "target creature deals damage to itself equal to its power" — Justice
# Strike / Repentance / Vengeance.
@_er(r"^target creature deals damage to itself equal to its power(?:\.|$)")
def _self_damage_equal_power(m):
    return UnknownEffect(raw_text="creature deals damage to itself = power")


# "discard your hand(,? then draw N cards)?" — Dangerous Wager / Change of
# Fortune.
@_er(r"^discard your hand(?:,? then draw (a|one|two|three|four|five|\d+) cards?"
     r"(?: for each [^.]+)?)?(?:\.|$)")
def _discard_then_draw(m):
    parts = [Discard(count="all", target=Filter(base="self", targeted=False))]
    if m.group(1):
        parts.append(Draw(count=_num(m.group(1))))
    if len(parts) == 1:
        return parts[0]
    return Sequence(items=tuple(parts))


# "you draw two cards and lose N life" — Decode Transmissions / Bad Deal /
# Live Fast (alternate form).
@_er(r"^you draw (a|one|two|three|four|five|x|\d+) cards? and "
     r"(?:lose|gain) (\d+|x) life(?:\.|$)")
def _draw_and_life(m):
    n = _num(m.group(1))
    life = _num(m.group(2))
    return Sequence(items=(
        Draw(count=n),
        LoseLife(amount=life),
    ))


# "you may have this creature assign its combat damage as though it
# weren't blocked" — Pride of Lions / Deathcoil Wurm-class trample-like.
@_er(r"^you may have (?:this creature|~) assign its combat damage as "
     r"though it weren'?t blocked(?:\.|$)")
def _assign_unblocked(m):
    return UnknownEffect(raw_text="may assign combat damage as though unblocked")


# "target player discards N cards unless ..." — Wrench Mind-class.
@_er(r"^target player discards (a|one|two|three|four|five|\d+) cards? unless "
     r"[^.]+(?:\.|$)")
def _tplayer_discard_unless(m):
    n = _num(m.group(1))
    return Discard(count=n, target=TARGET_PLAYER, chosen_by="discarder")


# "target player discards N cards, then draws as many cards as they
# discarded this way" — Forget-class.
@_er(r"^target player discards (a|one|two|three|four|five|\d+) cards?, "
     r"then draws as many [^.]+(?:\.|$)")
def _tplayer_discard_then_draw(m):
    n = _num(m.group(1))
    return Sequence(items=(
        Discard(count=n, target=TARGET_PLAYER, chosen_by="discarder"),
        Draw(count="var", target=TARGET_PLAYER),
    ))


# "support N" — bare keyword (Tarkir's "support N" mechanic). Lead by
# Example / Nissa's Judgment.
@_sp(r"^support (\d+)\s*$")
def _support_n(m, raw):
    return Keyword(name="support", args=(int(m.group(1)),), raw=raw)


# "descend N - <body>" — MKM "descend" ability word; the rest of the chunk
# is a static or triggered effect, but at minimum we record the keyword
# header so UNPARSED → at least PARTIAL.
@_sp(r"^descend (\d+)\s*[-—]\s*[^.]+\s*$")
def _descend_n(m, raw):
    return Keyword(name="descend", args=(int(m.group(1)),), raw=raw)


# "celebration - <body>" — DSK ability word.
@_sp(r"^celebration\s*[-—]\s*[^.]+\s*$")
def _celebration(m, raw):
    return Keyword(name="celebration", raw=raw)


# "void - <body>" — DSK ability word ("void" condition).
@_sp(r"^void\s*[-—]\s*[^.]+\s*$")
def _void_aw(m, raw):
    return Keyword(name="void", raw=raw)


# "players can't untap more than one X during their untap steps" — Imi Statue
# / Damping Field / Stasis-class.
@_sp(r"^players can'?t untap more than (one|two|\d+) ([^.]+?) during their "
     r"untap steps?\s*$")
def _no_multi_untap(m, raw):
    return Static(modification=Modification(kind="cap_untap_count",
                                            args=(m.group(1),
                                                  m.group(2).strip())),
                  raw=raw)


__all__ = ["EFFECT_RULES", "STATIC_PATTERNS", "TRIGGER_PATTERNS"]


# TRIGGER_PATTERNS not used here — niche_triggers.py / counter_triggers.py
# already cover the residual trigger shapes.
TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []


# ---------------------------------------------------------------------------
# Smoke test
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    # Minimal compile-check; matching is exercised by the parser at load.
    print(f"STATIC_PATTERNS: {len(STATIC_PATTERNS)}")
    print(f"EFFECT_RULES:    {len(EFFECT_RULES)}")
    print(f"TRIGGER_PATTERNS:{len(TRIGGER_PATTERNS)}")
