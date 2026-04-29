#!/usr/bin/env python3
"""UNPARSED-bucket residual patterns — second pass.

Family: UNPARSED → GREEN/PARTIAL promotions.

After ``unparsed_residual.py`` cleared 462 cards from the residual UNPARSED
bucket, ~1,063 cards remain where ZERO abilities parse. This second-pass
scrubber targets the next set of phrasing clusters that survived. We
intentionally avoid every shape already handled in the first scrubber.

Coverage targets (top clusters we found in the residual):

  * ``search your library for up to N <filter> [, reveal them], put them
    into your hand, then shuffle``                                  (~12)
  * ``whenever a player taps a <land/island/swamp> for mana, ...``   (~8)
  * ``whenever another <type> you control [with <stat>] enters, ...``(~14)
  * ``return target creature card from your graveyard to the battlefield
    with <suffix>``                                                  (~10)
  * ``up to N target creatures (each get .../can't.../gain ...)``    (~10)
  * ``until end of turn, whenever ...`` — temporary trigger riders   (~5)
  * ``target creature an opponent controls <effect>``                (~6)
  * ``whenever a player cycles a card, ...``                         (~3)
  * ``target creature and all other creatures with the same name as
    that creature get +N/+N``                                        (~3)
  * ``whenever an enchantment/aura/artifact you control is put into a
    graveyard from the battlefield, ...``                            (~10)
  * ``whenever a spell or ability causes <player> to <verb>, ...``   (~3)
  * ``return up to N target <filter> cards from your graveyard to your
    hand``                                                           (~6)
  * ``choose two target creatures controlled by [same/different]``   (~3)
  * ``one or two target creatures (each) (gain/get) ...``            (~3)
  * ``each land is a swamp/island/forest in addition to ...``        (~3)
  * ``no more than N creatures can attack/block ...``                (~4)
  * ``creatures you control get +N/+N during your turn``             (~3)
  * ``creatures without <kw> can't attack``                          (~2)
  * ``each player's life total becomes <X>``                         (~2)
  * ``each creature deals damage to itself equal to its <stat>``     (~2)
  * ``each player loses half their life ... [discards/sacrifices]``  (~2)
  * ``switch target creature's power and toughness until end of turn``(~2)
  * ``each creature assigns combat damage equal to its <stat>``      (~2)
  * ``creatures entering don't cause abilities to trigger``          (~2)
  * ``nonbasic lands are mountains`` (Blood Moon family)             (~2)
  * ``mill three cards, then return ...``                            (~3)
  * ``shuffle any number of target <filter> cards from your gy``     (~2)
  * ``sacrifice any number of lands. <effect>``                      (~2)
  * ``starting with you, each player <verb>``                        (~3)
  * ``exchange control of two target <filter>``                      (~3)
  * ``target player draws X cards and loses X life, where X is ...`` (~3)
  * ``each player discards their hand, then draws ...``              (~2)
  * ``each player discards all the cards in their hand, then ...``   (~2)
  * ``each opponent's maximum hand size is reduced by N``            (~2)
  * ``creatures of <color>/this creature get -1/-1``                 (~2)
  * ``conjure a duplicate of <X>``                                   (~2)
  * ``put target creature card from <gy> onto the battlefield ...``  (~2)
  * ``each player sacrifices a <thing> of their choice``             (~3)
  * ``whenever you roll one or more dice, ...``                      (~2)
  * ``while an opponent is choosing targets ..., that player must
    choose at least one Flagbearer ...``                             (~2)
  * ``return all artifact and enchantment cards from your gy to ...``(~2)

Three export tables are merged by ``parser.load_extensions``:

    STATIC_PATTERNS  — ability-level static / anthem shapes
    EFFECT_RULES     — body-level effect productions (parse_effect)
    TRIGGER_PATTERNS — (re, event, scope) tuples for parse_triggered

Ordering is specific-first within each list. Like its sibling, this file
prefers ``UnknownEffect`` / generic ``Static(Modification(kind=...))`` for
shapes the AST doesn't model precisely so the ability gets recorded
(promoting UNPARSED → at least PARTIAL) without inventing semantics.
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
    Bounce, Buff, Damage, Destroy, Discard, Draw, Exile,
    Filter, GainControl, GrantAbility, Keyword, LoseLife, Mill, Modification,
    Recurse, Sacrifice, Sequence, Static, UnknownEffect,
    TARGET_ANY, TARGET_CREATURE, TARGET_OPPONENT, TARGET_PLAYER,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}

_BASIC = r"(?:swamps?|islands?|mountains?|forests?|plains|lands?)"


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
# Combat-quota and table-staple statics.
# ---------------------------------------------------------------------------

# "no more than one creature can attack each combat" / "no more than two
# creatures can block each combat" / "no more than N creatures can attack
# you each combat" — Silent Arbiter, Crawlspace, Caverns of Despair.
@_sp(r"^no more than (one|two|three|\d+) creatures? can "
     r"(attack|block)(?: you)?(?: each combat)?\s*$")
def _quota_attack_block(m, raw):
    return Static(modification=Modification(kind="combat_quota_cap",
                                            args=(m.group(1).lower(),
                                                  m.group(2).lower())),
                  raw=raw)


# "creatures without <kw> can't attack" / "creatures with <kw> can't block"
# Moat, Magus of the Moat, Dungeon of the Mad Mage tier.
@_sp(r"^creatures (with|without) ([a-z][a-z ]+?) can'?t "
     r"(attack|block)(?: this turn)?\s*$")
def _filter_kw_cant(m, raw):
    return Static(modification=Modification(kind="filter_kw_cant",
                                            args=(m.group(1).lower(),
                                                  m.group(2).strip(),
                                                  m.group(3).lower())),
                  raw=raw)


# "creatures entering don't cause abilities to trigger" — Torpor Orb,
# Tocatli Honor Guard, Hushbringer-class. Includes "creatures entering the
# battlefield don't cause abilities to trigger" old templating.
@_sp(r"^creatures entering(?: the battlefield)? don'?t cause abilities "
     r"to trigger\s*$")
def _torpor(m, raw):
    return Static(modification=Modification(kind="suppress_etb_triggers"),
                  raw=raw)


# "each creature assigns combat damage equal to its toughness rather than
# its power" — Doran, Assault Formation-style.
@_sp(r"^each creature assigns combat damage equal to its (toughness|"
     r"mana value|power) rather than its (power|toughness|mana value)\s*$")
def _doran(m, raw):
    return Static(modification=Modification(kind="assign_dmg_by_stat",
                                            args=(m.group(1).lower(),)),
                  raw=raw)


# "each opponent's maximum hand size is reduced by N" — Locust Miser,
# Black Vise relatives.
@_sp(r"^each opponents? maximum hand size is reduced by "
     r"(one|two|three|four|\d+)\s*$")
def _hand_size_minus(m, raw):
    return Static(modification=Modification(kind="opp_max_hand_minus",
                                            args=(_num(m.group(1)),)),
                  raw=raw)


# "nonbasic lands are mountains" / "nonbasic lands are swamps" —
# Blood Moon, Magus of the Moon, Contamination, Cursed Land tier.
@_sp(rf"^nonbasic lands are ({_BASIC})\s*$")
def _nonbasic_are(m, raw):
    return Static(modification=Modification(kind="nonbasic_become",
                                            args=(m.group(1).rstrip("s"),)),
                  raw=raw)


# "each land is a swamp in addition to its other land types" —
# Urborg / Blanket of Night family.
@_sp(r"^each land is an? (swamp|island|mountain|forest|plains) in "
     r"addition to its other land types\s*$")
def _each_land_is(m, raw):
    return Static(modification=Modification(kind="each_land_is_type",
                                            args=(m.group(1).lower(),)),
                  raw=raw)


# "during your turn, creatures you control get +N/+N (and have <kw>)" —
# Street Riot, Vibrating Sphere line A.
@_sp(r"^during your turn, creatures? you control get \+(\d+)/\+(\d+)"
     r"(?: and have ([^.]+?))?\s*$")
def _your_turn_anthem(m, raw):
    return Static(modification=Modification(kind="your_turn_anthem",
                                            args=(int(m.group(1)),
                                                  int(m.group(2)),
                                                  (m.group(3) or "").strip())),
                  raw=raw)


# "during turns other than yours, creatures you control get -N/-N" —
# Vibrating Sphere line B.
@_sp(r"^during turns other than yours, creatures? you control get "
     r"([+-]\d+)/([+-]\d+)\s*$")
def _opp_turn_anthem(m, raw):
    return Static(modification=Modification(kind="opp_turn_anthem",
                                            args=(int(m.group(1)),
                                                  int(m.group(2)))),
                  raw=raw)


# "creatures your opponents control get -N/-N (until end of turn)?" —
# Turn the Tide, Severed Legion-class anti-anthem.
@_sp(r"^creatures your opponents control get ([+-]\d+)/([+-]\d+)"
     r"(?: until end of turn)?\s*$")
def _enemy_anthem(m, raw):
    return Static(modification=Modification(kind="enemy_anthem",
                                            args=(int(m.group(1)),
                                                  int(m.group(2)))),
                  raw=raw)


# "while an opponent is choosing targets ..., that player must choose at
# least one Flagbearer ..." — Standard Bearer / Coalition Honor Guard.
@_sp(r"^while an opponent is choosing targets [^.]+\s*$")
def _flagbearer(m, raw):
    return Static(modification=Modification(kind="flagbearer_redirect"),
                  raw=raw)


# "spend only mana produced by <source> to cast this spell" — Myr Superion,
# Imperiosaur, Tide Skimmer-class.
@_sp(r"^spend only mana produced by ([^.]+?) to cast (?:this spell|~)\s*$")
def _spend_only_mana(m, raw):
    return Static(modification=Modification(kind="cast_mana_restriction",
                                            args=(m.group(1).strip(),)),
                  raw=raw)


# "the first spell you cast from <zone> each turn has <kw>" —
# Twelfth Doctor / Wild-Magic Sorcerer-style.
@_sp(r"^the first spell you cast from ([^.]+?) each turn has ([^.]+?)\s*$")
def _first_spell_has(m, raw):
    return Static(modification=Modification(kind="first_spell_grant",
                                            args=(m.group(1).strip(),
                                                  m.group(2).strip())),
                  raw=raw)


# "this creature gets -N/-N for each <X>" — Mogg Squad, Grim Strider line.
@_sp(r"^(?:this creature|~) gets ([+-]\d+)/([+-]\d+) for each ([^.]+?)\s*$")
def _self_pt_per(m, raw):
    return Static(modification=Modification(kind="self_pt_per_thing",
                                            args=(int(m.group(1)),
                                                  int(m.group(2)),
                                                  m.group(3).strip())),
                  raw=raw)


# "<color> creatures get -N/-N" — bare global color anti-anthem (Drown in
# Sorrow / Death Cloud-style residue). Distinct from the first scrubber's
# "<color> creatures you control" anthem (this one is unscoped).
@_sp(r"^(?:other )?(white|blue|black|red|green) creatures? get "
     r"([+-]\d+)/([+-]\d+)\s*$")
def _color_global_pt(m, raw):
    return Static(modification=Modification(kind="color_global_pt",
                                            args=(m.group(1).lower(),
                                                  int(m.group(2)),
                                                  int(m.group(3)))),
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
# Tutor — search-then-reveal-then-hand. The first scrubber covered "search
# your library for up to N <filter>, put them into your hand, then shuffle"
# but NOT the more common shape that includes the "reveal them" step.
# ---------------------------------------------------------------------------

# "search your library for up to N <filter>, reveal them, put them into
# your hand, then shuffle" — Ignite the Beacon, Plea for Guidance, etc.
@_er(r"^search your library for up to (one|two|three|four|five|\d+) "
     r"([^,.]+?),? reveal (?:them|it),? put (?:them|it) into your hand,? "
     r"then shuffle(?:\.|$)")
def _search_reveal_to_hand(m):
    return UnknownEffect(raw_text=f"search lib reveal->hand up to {m.group(1)} "
                                  f"{m.group(2).strip()}")


# "search your library for any number of <filter>, reveal them, put them
# into your hand, then shuffle" — Living Death-class wide tutors.
@_er(r"^search your library for any number of ([^,.]+?),?"
     r"(?: reveal (?:them|it),?)? put (?:them|it) into your hand,? "
     r"then shuffle(?:\.|$)")
def _search_any_to_hand(m):
    return UnknownEffect(raw_text=f"search lib any to hand "
                                  f"{m.group(1).strip()}")


# "search your library and graveyard for <X>, reveal it, put it into your
# hand, then shuffle" — Cabal Ritual / Diabolic Revelation residue.
@_er(r"^search your library and graveyard for ([^,.]+?),?"
     r"(?: reveal (?:them|it),?)?(?: put (?:them|it) into your hand,?)?"
     r"(?: then shuffle)?(?:\.|$)")
def _search_lib_gy(m):
    return UnknownEffect(raw_text=f"search lib+gy {m.group(1).strip()}")


# ---------------------------------------------------------------------------
# Reanimation — "return target creature card from your graveyard to the
# battlefield with <suffix>" / "...to your hand with <suffix>".
# ---------------------------------------------------------------------------

# "return target creature card from your graveyard to the battlefield
# with <stuff>" — Evil Reawakened, Unbreakable Bond, etc.
@_er(r"^return target ([^.]+?) card from your graveyard to the "
     r"battlefield(?: with [^.]+)?(?:\.|$)")
def _reanimate_with(m):
    return Recurse(target=Filter(base=m.group(1).strip(), targeted=True),
                   to="battlefield")


# "return up to N target <filter> cards from your graveyard to your hand,
# then <effect>" — Cathartic Operation, Pull Through the Weft-style.
@_er(r"^return up to (one|two|three|four|five|\d+) target ([^.]+?) cards? "
     r"from your graveyard to your hand(?:,? then [^.]+)?(?:\.|$)")
def _recurse_up_to_n(m):
    return Recurse(target=Filter(base=m.group(2).strip(), targeted=True,
                                 extra=("up_to_" + str(_num(m.group(1))),)),
                   to="hand")


# "return two target creature cards from your graveyard to your hand"
@_er(r"^return (two|three|four|\d+) target ([^.]+?) cards? from your "
     r"graveyard to your hand(?:\.|$)")
def _recurse_n_to_hand(m):
    return Recurse(target=Filter(base=m.group(2).strip(), targeted=True,
                                 extra=(m.group(1),)),
                   to="hand")


# "return target permanent card from your graveyard to your hand" —
# bare singular variant (the first scrubber covers "to owner's hand"
# bounce; this is graveyard recursion).
@_er(r"^return target ([^.]+?) card from your graveyard to your hand"
     r"(?:\.|$)")
def _recurse_single(m):
    return Recurse(target=Filter(base=m.group(1).strip(), targeted=True),
                   to="hand")


# "return all artifact and enchantment cards from your graveyard to the
# battlefield" — Open the Vaults / Replenish-class mass reanimation.
@_er(r"^return all ([^.]+?) cards? from your graveyard to (the battlefield|"
     r"your hand)(?:\.|$)")
def _recurse_all(m):
    to = "battlefield" if "battlefield" in m.group(2) else "hand"
    return Recurse(target=Filter(base=m.group(1).strip(), targeted=False),
                   to=to)


# ---------------------------------------------------------------------------
# Multi-target body effects.
# ---------------------------------------------------------------------------

# "up to three target creatures can't block this turn" / "up to two target
# creatures can't be blocked this turn" — Unearthly Blizzard, Ghostform.
@_er(r"^up to (one|two|three|four|\d+) target creatures? can'?t "
     r"(block|be blocked|attack)(?: this turn)?(?:\.|$)")
def _utn_target_cant(m):
    return UnknownEffect(raw_text=f"up to {m.group(1)} target creatures "
                                  f"can't {m.group(2)}")


# "up to N target creatures each get +X/+X until end of turn[, where x is
# ...]" — Allied Assault, Sweeping Battle line.
@_er(r"^up to (one|two|three|four|\d+) target creatures? each get "
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x) until end of turn"
     r"(?:,? where x is [^.]+)?(?:\.|$)")
def _utn_target_pump(m):
    return Buff(power=m.group(2), toughness=m.group(3),
                target=Filter(base="creature", targeted=True,
                              extra=(f"up_to_{_num(m.group(1))}",)))


# "up to N target creatures gain <kw> until end of turn" — buff/grant on a
# multi-target spell.
@_er(r"^up to (one|two|three|four|\d+) target creatures? gain ([^.]+?) "
     r"until end of turn(?:\.|$)")
def _utn_target_grant(m):
    return GrantAbility(ability_name=m.group(2).strip(),
                        target=Filter(base="creature", targeted=True,
                                      extra=(f"up_to_{_num(m.group(1))}",)))


# "one or two target creatures (each)? gain <kw> (until end of turn)?" —
# Hearts on Fire, Wind Sail.
@_er(r"^one or two target creatures?(?: each)? gain ([^.]+?)"
     r"(?: until end of turn)?(?:\.|$)")
def _12_target_grant(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="creature", targeted=True,
                                      extra=("one_or_two",)))


# "one or two target creatures each get +N/+N until end of turn" —
# Hearts on Fire-style pump.
@_er(r"^one or two target creatures? each get ([+-]\d+)/([+-]\d+) "
     r"until end of turn(?:\.|$)")
def _12_target_pump(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", targeted=True,
                              extra=("one_or_two",)))


# "choose two target creatures controlled by (different|the same) player"
# — Run Away Together, Barrin's Spite (header line only; subsequent
# sentence carries the real effect and parses on its own).
@_er(r"^choose two target creatures controlled by (different players|the "
     r"same player)(?:\.|$)")
def _choose_two_targets(m):
    return UnknownEffect(raw_text=f"choose 2 targets {m.group(1)}")


# "exchange control of two target permanents (that share a card type)?" /
# "exchange control of two target creatures" — Switcheroo, Shifting
# Loyalties.
@_er(r"^exchange control of two target ([^.]+?)(?:\.|$)")
def _exchange_two(m):
    return UnknownEffect(raw_text=f"exchange control 2 {m.group(1).strip()}")


# "exchange control of target artifact you control and target artifact an
# opponent controls" — Steal Artifact-style asymmetric swap.
@_er(r"^exchange control of target ([^.]+?) and target ([^.]+?)(?:\.|$)")
def _exchange_pair(m):
    return UnknownEffect(raw_text=f"exchange control {m.group(1).strip()} "
                                  f"and {m.group(2).strip()}")


# ---------------------------------------------------------------------------
# Body effects targeting opposing creatures.
# ---------------------------------------------------------------------------

# "target creature an opponent controls (perpetually )?gets +/-N/+/-N" —
# Davriel's Withering, Alchemy perpetual nudges.
@_er(r"^target creature an opponent controls (?:perpetually )?gets "
     r"([+-]\d+)/([+-]\d+)(?: until end of turn)?(?:\.|$)")
def _tc_opp_pump(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", targeted=True,
                              extra=("opponent_controls",)))


# "target creature an opponent controls deals damage equal to its power
# to another target creature that player controls" — Mutiny / Twisted
# Justice. We model as Damage source=target.
@_er(r"^target creature an opponent controls deals damage equal to its "
     r"(power|toughness) to another target [^.]+(?:\.|$)")
def _tc_opp_self_dmg(m):
    return UnknownEffect(raw_text=f"opp creature deals damage = {m.group(1)} "
                                  "to another opp creature")


# "target creature gets -N/-N until end of turn" — straight debuff body.
@_er(r"^target creature gets -(\d+)/-(\d+) until end of turn(?:\.|$)")
def _tc_debuff(m):
    return Buff(power=-int(m.group(1)), toughness=-int(m.group(2)),
                target=TARGET_CREATURE)


# "target creature gets +N/-N until end of turn" / "+N/+N" — mixed-sign
# Wildsize/Blackmail-style.
@_er(r"^target creature gets ([+-]\d+)/([+-]\d+) until end of turn(?:\.|$)")
def _tc_mixed(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=TARGET_CREATURE)


# "target creature you control gets +X/+X (and gains <kws>) until end of
# turn" — Tyvar's Stand, Frantic Confrontation. Combines Buff + Grant.
@_er(r"^target creature you control gets ([+-]\d+|[+-]x)/([+-]\d+|[+-]x) "
     r"(?:and gains ([^.]+?) )?until end of turn(?:\.|$)")
def _tc_yours_pump_grant(m):
    pump = Buff(power=m.group(1), toughness=m.group(2),
                target=Filter(base="creature", targeted=True,
                              extra=("you_control",)))
    if m.group(3):
        return Sequence(items=(pump, GrantAbility(
            ability_name=m.group(3).strip(),
            target=Filter(base="creature", targeted=True,
                          extra=("you_control",)))))
    return pump


# ---------------------------------------------------------------------------
# Player-targeted compound effects we still miss.
# ---------------------------------------------------------------------------

# "target player draws X cards and loses X life, where X is ..." —
# Monumental Corruption, Eldritch Pact.
@_er(r"^target player draws (x|\d+) cards? and loses (x|\d+) life,? "
     r"where x is [^.]+(?:\.|$)")
def _tp_draw_lose_x(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1)), target=TARGET_PLAYER),
        LoseLife(amount=_num(m.group(2)), target=TARGET_PLAYER),
    ))


# "target player draws cards equal to <X>" — variable draw.
@_er(r"^target player draws cards equal to [^.]+(?:\.|$)")
def _tp_draw_equal(m):
    return Draw(count="var", target=TARGET_PLAYER)


# "target player draws two cards, then discards (a|N) cards?" — Whirlwind
# Technique, Wistful Thinking.
@_er(r"^target player draws (a|one|two|three|four|five|x|\d+) cards?, "
     r"then discards (a|one|two|three|four|five|x|\d+) cards?(?:\.|$)")
def _tp_draw_then_discard(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1)), target=TARGET_PLAYER),
        Discard(count=_num(m.group(2)), target=TARGET_PLAYER,
                chosen_by="discarder"),
    ))


# "target player discards X cards at random (unless ...)?" — Hymn to Tourach,
# Skullscorch.
@_er(r"^target player discards (a|one|two|three|four|five|x|\d+) cards? "
     r"at random(?: unless [^.]+)?(?:\.|$)")
def _tp_discard_random(m):
    return Discard(count=_num(m.group(1)), target=TARGET_PLAYER,
                   chosen_by="random")


# "any number of target players each <verb> ..." — Cabal Conditioning,
# Turtle Tracks (multi-player edict).
@_er(r"^any number of target players each [^.]+(?:\.|$)")
def _any_players_each(m):
    return UnknownEffect(raw_text="any number of target players each ...")


# "each player sacrifices a <thing> of their choice (for each ...)?" —
# Tremble, Thoughts of Ruin, Lethal Vapors-class group sacrifice.
@_er(r"^each player sacrifices an? ([^.]+?) of their choice"
     r"(?: for each [^.]+)?(?:\.|$)")
def _each_player_sac(m):
    return Sacrifice(what=Filter(base=m.group(1).strip(), targeted=False),
                     who="each_player")


# "target player sacrifices a creature of their choice (and loses N life)?"
# — Geth's Verdict, Devour Flesh, Cruel Edict.
@_er(r"^target player sacrifices an? ([^.]+?) of their choice"
     r"(?:,? (?:then|and)? ?(?:loses|gains) [^.]+)?(?:\.|$)")
def _tp_sac_choice(m):
    return Sacrifice(what=Filter(base=m.group(1).strip(), targeted=False),
                     who="target_player")


# "target player sacrifices an attacking creature" / "target player
# sacrifices a creature with <X>" — Diabolic Edict cousins.
@_er(r"^target player sacrifices an? ([^.]+?)(?:\.|$)")
def _tp_sac_filtered(m):
    return Sacrifice(what=Filter(base=m.group(1).strip(), targeted=False),
                     who="target_player")


# "each player discards their hand, then draws <something>" —
# Wheel of Fortune, Windfall, Echo of Eons.
@_er(r"^each player discards their hand,? then draws ([^.]+?)(?:\.|$)")
def _wheel(m):
    return Sequence(items=(
        Discard(count="hand", target=Filter(base="player", targeted=False),
                chosen_by="self"),
        Draw(count="var", target=Filter(base="player", targeted=False)),
    ))


# "each player discards all the cards in their hand, then <effect>" —
# Awaken the Erstwhile, Dark Deal-style.
@_er(r"^each player discards all the cards in their hand,? then [^.]+"
     r"(?:\.|$)")
def _each_discard_all(m):
    return Discard(count="hand", target=Filter(base="player", targeted=False),
                   chosen_by="self")


# "each player loses half their life, then discards/sacrifices ..." —
# Fraying Omnipotence / Pox Plague.
@_er(r"^each player loses half their life,? then [^.]+(?:\.|$)")
def _half_life_then(m):
    return LoseLife(amount="half", target=Filter(base="player", targeted=False))


# "each player's life total becomes <X>" — Repay in Kind, Biorhythm,
# Tree of Redemption-style equalizer.
@_er(r"^each players?'? life total becomes [^.]+(?:\.|$)")
def _life_total_becomes(m):
    return UnknownEffect(raw_text="life total becomes X")


# "each player may search their library for <X>" — New Frontiers,
# Weird Harvest, group hug tutors.
@_er(r"^each player may search their library for [^.]+(?:\.|$)")
def _each_may_search(m):
    return UnknownEffect(raw_text="group-hug search lib")


# "starting with you, each player <verb> ..." — Eureka, Plague of Vermin,
# Joyous Respite. Repeat-process effects.
@_er(r"^starting with you,? each player [^.]+(?:\.|$)")
def _starting_with_you(m):
    return UnknownEffect(raw_text="starting with you, each player ...")


# ---------------------------------------------------------------------------
# Misc remaining shapes.
# ---------------------------------------------------------------------------

# "target creature and all other creatures with the same name as that
# creature get +/-N/+/-N until end of turn" — Bile Blight, Echoing Courage.
@_er(r"^target creature and all other creatures with the same name as "
     r"that creature get ([+-]\d+)/([+-]\d+)(?: until end of turn)?"
     r"(?:\.|$)")
def _name_pump(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="creature", targeted=True,
                              extra=("same_name_as_target",)))


# "switch target creature's power and toughness until end of turn" —
# Transmutation, About Face.
@_er(r"^switch target creatures? power and toughness(?: until end of turn)?"
     r"(?:\.|$)")
def _switch_pt(m):
    return UnknownEffect(raw_text="switch P/T target creature")


# "shuffle any number of target <filter> cards from your graveyard into
# your library" — Renewing Touch, Piper's Melody.
@_er(r"^shuffle any number of target ([^.]+?) cards? from your graveyard "
     r"into your library(?:\.|$)")
def _shuffle_gy_back(m):
    return UnknownEffect(raw_text=f"shuffle gy->lib {m.group(1).strip()}")


# "sacrifice any number of lands(. <effect>)?" — Mana Seism, Hew the
# Entwood. The follow-up sentence is parsed separately by split_abilities.
@_er(r"^sacrifice any number of lands(?:\.|$)")
def _sac_any_lands(m):
    return Sacrifice(what=Filter(base="land", targeted=False,
                                 extra=("any_number",)),
                     who="self")


# "mill three cards, then return a creature card from your graveyard to
# your hand" — Grapple with the Past, Corpse Churn-style.
@_er(r"^mill (a|one|two|three|four|five|x|\d+) cards?, then "
     r"(?:you may )?return [^.]+(?:\.|$)")
def _mill_then_return(m):
    return Sequence(items=(
        Mill(count=_num(m.group(1))),
        UnknownEffect(raw_text="return creature/land from gy to hand"),
    ))


# "mill three cards, then <effect>" — generic mill-then chain.
@_er(r"^mill (a|one|two|three|four|five|x|\d+) cards?, then [^.]+"
     r"(?:\.|$)")
def _mill_then_anything(m):
    return Mill(count=_num(m.group(1)))


# "tap up to N target lands" / "tap up to N target <filter>" — Early
# Frost, Stasis Field-style.
@_er(r"^tap up to (one|two|three|four|five|\d+) target ([^.]+?)(?:\.|$)")
def _tap_up_to(m):
    return UnknownEffect(raw_text=f"tap up to {m.group(1)} {m.group(2).strip()}")


# "put target creature card from <gy> onto the battlefield (tapped)?
# under your control" — Reanimate-class, Nurgle's Conscription, Ashen
# Powder, Whisper of the Muse-residue.
@_er(r"^put target creature card from ([^.]+?) onto the battlefield"
     r"(?: tapped)?(?: under your control)?(?:,? then [^.]+)?(?:\.|$)")
def _put_creature_card(m):
    return Recurse(target=Filter(base="creature", targeted=True,
                                 extra=("from_" + m.group(1).strip(),)),
                   to="battlefield")


# "conjure a duplicate of <X>" — Alchemy "conjure" mechanic.
@_er(r"^conjure a duplicate of [^.]+(?:\.|$)")
def _conjure_dupe(m):
    return UnknownEffect(raw_text="conjure duplicate")


# "target creature fights another target creature" — Clash of Titans,
# Blood Feud, Savage Stomp residue.
@_er(r"^target creature fights another target creature(?:\.|$)")
def _two_target_fight(m):
    return UnknownEffect(raw_text="target creature fights another target")


# "all creatures get -X/-X until end of turn[, where x is ...]" —
# Cloudkill, Deluge of Doom — variable sweep that base parser misses
# because of the bare "x" literal in the slash.
@_er(r"^all creatures get -x/-x until end of turn"
     r"(?:,? where x is [^.]+)?(?:\.|$)")
def _all_creatures_xx(m):
    return Buff(power="-x", toughness="-x",
                target=Filter(base="creature", targeted=False, extra=("all",)))


# "attacking creatures get +N/+0 and gain <kw> until end of turn" — Rally
# the Forces, Stampede style combat-state pump+grant.
@_er(r"^attacking creatures get \+(\d+)/\+(\d+) and gain ([^.]+?) until "
     r"end of turn(?:\.|$)")
def _attacking_pump_grant(m):
    pump = Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=Filter(base="attacking_creatures", targeted=False))
    return Sequence(items=(pump, GrantAbility(
        ability_name=m.group(3).strip(),
        target=Filter(base="attacking_creatures", targeted=False))))


# "until end of turn, target ..." — duration-first targeted effect that
# the base parser doesn't find because the duration is at the front.
@_er(r"^until end of turn,? target creature loses all abilities and "
     r"becomes [^.]+(?:\.|$)")
def _eot_loses_becomes(m):
    return UnknownEffect(raw_text="target creature loses abilities + becomes")


@_er(r"^until end of turn,? target ([^.]+?) gets ([+-]\d+)/([+-]\d+)"
     r"(?: and gains [^.]+)?(?:\.|$)")
def _eot_target_pump(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=Filter(base=m.group(1).strip(), targeted=True))


# "until end of turn, each creature ..." — Damping Field-style global rider.
@_er(r"^until end of turn,? each creature [^.]+(?:\.|$)")
def _eot_each_creature(m):
    return UnknownEffect(raw_text="UEOT each creature ...")


# "until end of turn, if <cond>, ..." — Blood of the Martyr, Pale Moon
# duration+conditional rider.
@_er(r"^until end of turn,? if [^.]+(?:\.|$)")
def _eot_if(m):
    return UnknownEffect(raw_text="UEOT conditional rider")


# "you may mill three cards. then ..." — Body Snatcher / Tymaret-style
# optional mill prefix (the "then" clause is its own sentence and the
# splitter handles it; here we just need the optional prefix recognized).
@_er(r"^you may mill (a|one|two|three|four|five|\d+) cards?(?:\.|$)")
def _may_mill(m):
    return Mill(count=_num(m.group(1)))


# "you draw two cards, lose N life" — Plumb the Forbidden / Painful
# Truths shapes that escaped the first scrubber's "lose ... life" form.
@_er(r"^you draw (a|one|two|three|four|five|x|\d+) cards?, lose (\d+|x) "
     r"life(?:\.|$)")
def _you_draw_lose(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1))),
        LoseLife(amount=_num(m.group(2))),
    ))


# "you draw three cards and you lose N life" — variant phrasing.
@_er(r"^you draw (a|one|two|three|four|five|x|\d+) cards? and you lose "
     r"(\d+|x) life(?:\.|$)")
def _you_draw_and_you_lose(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1))),
        LoseLife(amount=_num(m.group(2))),
    ))


# "draw four cards, then discard three cards" — bare draw/discard
# (no "you" prefix; rummage-style).
@_er(r"^draw (a|one|two|three|four|five|x|\d+) cards?, then discard "
     r"(a|one|two|three|four|five|x|\d+) cards?(?:\.|$)")
def _draw_then_discard(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1))),
        Discard(count=_num(m.group(2)), chosen_by="self"),
    ))


# "draw three cards, then put a card from your hand on top of your library"
# — bounce-back card-selection bodies (Brainstorm-cousin).
@_er(r"^draw (a|one|two|three|four|five|x|\d+) cards?, then put [^.]+ "
     r"on top of your library(?:\.|$)")
def _draw_then_top(m):
    return Draw(count=_num(m.group(1)))


# ===========================================================================
# TRIGGER_PATTERNS — (regex, event, scope) tuples
# ===========================================================================
# parse_triggered tries these BEFORE the core list. The matched prefix is
# stripped and the remainder fed through parse_effect (falls back to
# UnknownEffect if no rule matches), so we just need to recognize the
# trigger header to promote UNPARSED → PARTIAL.
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # "whenever a player taps a <land/island/swamp> for mana" — Manabarbs,
    # Scald, Bubbling Muck, Storm Cauldron-class mana-tap watchers.
    (re.compile(r"^whenever a player taps an? ([^,.]+?) for mana", re.I),
     "any_tap_for_mana", "all"),

    # "whenever another <type> you control [with <stat>] enters" —
    # Garruk's Packleader, Paleoloth, Wayward Servant, Binding Mummy.
    # Broader than the core "another creature you control enters" because
    # it also covers tribal restrictions (Zombie/Beast/Ally/Aura) AND
    # power riders.
    (re.compile(r"^whenever another ([^,.]+?) you control(?: with [^,.]+?)? "
                r"enters", re.I),
     "another_typed_etb", "actor"),

    # "whenever a <type> you control enters" — sister to the above
    # without "another" (Aether Charge, Woodland Liege).
    (re.compile(r"^whenever a ([^,.]+?) you control enters", re.I),
     "ally_typed_etb", "actor"),

    # "whenever a beast enters, ..." — bare typed-trigger (no
    # control restriction).
    (re.compile(r"^whenever an? ([a-z]+) enters,", re.I),
     "any_typed_etb", "actor"),

    # "whenever this <perm> or another <X> you control enters" —
    # Oath of the Ancient Wood, Veil of Assimilation.
    (re.compile(r"^whenever this (?:enchantment|artifact|creature|"
                r"permanent|land) or another [^,.]+? "
                r"(?:enters|is put into a graveyard from the battlefield)",
                re.I),
     "self_or_typed_event", "self"),

    # "whenever a player cycles a card" — Astral Slide, Lightning Rift,
    # Warped Researcher tier.
    (re.compile(r"^whenever a player cycles a card", re.I),
     "any_cycle", "all"),

    # "whenever an enchantment/aura/artifact you control is put into a
    # graveyard from the battlefield" — Wicked Visitor, Ashiok's Reaper,
    # Slagstone Refinery.
    (re.compile(r"^whenever an? ([^,.]+?) you control is put into a "
                r"graveyard from the battlefield", re.I),
     "ally_typed_to_gy", "actor"),

    # "whenever an enchantment/aura/artifact you control enters" —
    # Tanglespan Lookout, Light-Paws, etc.
    (re.compile(r"^whenever an ([^,.]+?) you control enters", re.I),
     "ally_typed_etb_a", "actor"),

    # "whenever a spell or ability causes a player to <verb>" —
    # Psychogenic Probe, Widespread Panic, Trauma family.
    (re.compile(r"^whenever a spell or ability causes (?:a player|its "
                r"controller|that player) to ([^,.]+)", re.I),
     "spell_or_ability_causes", "all"),

    # "whenever an opponent shuffles their library" — Cosi's Trickster,
    # Psychic Surgery.
    (re.compile(r"^whenever an opponent shuffles their library", re.I),
     "opp_shuffle", "actor"),

    # "whenever an opponent activates an ability of <X>" — Harsh Mentor,
    # Runic Armasaur, Manabarbs companion line.
    (re.compile(r"^whenever an opponent activates an ability(?: of [^,.]+)?",
                re.I),
     "opp_activate", "actor"),

    # "whenever an opponent plays a land" — Dirtcowl Wurm, Burgeoning,
    # Oracle of Mul Daya-cousin.
    (re.compile(r"^whenever an opponent plays a land", re.I),
     "opp_landfall", "actor"),

    # "whenever an opponent is dealt <N or more>? damage (by ...)?" —
    # Pain Magnification, Wildfire Elemental.
    (re.compile(r"^whenever an opponent is dealt [^,.]+? damage", re.I),
     "opp_dealt_damage", "actor"),

    # "whenever a player sacrifices a permanent/creature" — Mayhem
    # Devil, Mortician Beetle.
    (re.compile(r"^whenever a player sacrifices an? [^,.]+", re.I),
     "any_sacrifice", "all"),

    # "whenever a nontoken creature an opponent controls dies/enters" —
    # Fire Nation Sentinels, Theoretical Duplication.
    (re.compile(r"^whenever a nontoken creature an opponent controls "
                r"(dies|enters[^,.]*)", re.I),
     "enemy_creature_event", "actor"),

    # "whenever this creature and at least <N> other <X> attack" —
    # Haazda Marshal, Paired Tactician (formation triggers).
    (re.compile(r"^whenever (?:this creature|~) and at least "
                r"(?:one|two|three|\d+) other [^,.]+ attacks?", re.I),
     "formation_attack", "self"),

    # "whenever you roll one or more dice" — Brazen Dwarf, Feywild
    # Trickster. Distinct from base "whenever you roll a die".
    (re.compile(r"^whenever you roll one or more dice", re.I),
     "roll_dice", "self"),

    # "whenever you activate an ability of <X>" — Training Grounds-cousin
    # ability-watcher.
    (re.compile(r"^whenever you activate an ability(?: of [^,.]+)?", re.I),
     "self_activate", "self"),

    # "when this creature is put into your graveyard from the battlefield"
    # — Brood of Cockroaches, Zodiac Dragon (older "is put" templating).
    (re.compile(r"^when (?:this creature|~) is put into your graveyard "
                r"from the battlefield", re.I),
     "self_to_gy", "self"),

    # "until end of turn, whenever <subj> <verb>" — Bonus Round, Bubbling
    # Muck. Temporary trigger riders. We strip the duration prefix and
    # leave the rest to either a follow-up trigger pattern or
    # UnknownEffect.
    (re.compile(r"^until end of turn,? whenever [^,.]+", re.I),
     "until_eot_trigger", "all"),

    # "when you cast a creature spell" — Goblin Dark-Dwellers / Adventurer
    # cousin. Base parser has "whenever you cast", we add the singular
    # "when ... a creature spell" form.
    (re.compile(r"^when you cast (?:a|an) ([^,.]+?) spell", re.I),
     "cast_filtered_when", "self"),

    # "target player draws X cards. <effect>" — When the rest of the
    # spell is a single Draw clause that's actually a body effect, but
    # the splitter sometimes leaves the trigger header on its own line.
    # (Already covered by core; included here only for completeness.)
]


__all__ = ["EFFECT_RULES", "STATIC_PATTERNS", "TRIGGER_PATTERNS"]


# ---------------------------------------------------------------------------
# Smoke test
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    print(f"STATIC_PATTERNS:  {len(STATIC_PATTERNS)}")
    print(f"EFFECT_RULES:     {len(EFFECT_RULES)}")
    print(f"TRIGGER_PATTERNS: {len(TRIGGER_PATTERNS)}")
