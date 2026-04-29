#!/usr/bin/env python3
"""Phase 4 residual-triage extension.

Targets the top 6 clusters of ``parsed_effect_residual`` entries — effects
inside triggered abilities where ``parse_effect()`` returned None because
the subject phrase didn't match existing narrow rules.

Cluster coverage (estimated):
  self-source damage:    ~400  (this creature/enchantment/it deals N damage)
  gain control:          ~80   (gain control of target X until eot)
  put onto battlefield:  ~70   (put a creature card onto the battlefield)
  sacrifice unless:      ~50   (sacrifice this X unless you pay {N})
  self buff until eot:   ~70   (this creature gets +N/+N until eot)
  mill:                  ~25   (target player mills N)
  fight:                 ~40   (it/target creature fights)
  exile target/all:      ~90   (exile each/all X)
  return from graveyard: ~50   (return X from graveyard to battlefield/hand)
  each opponent effect:  ~40   (each opponent loses/discards/sacrifices)
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
    Bounce, Buff, CounterMod, Damage, Destroy, Discard, Draw,
    Exile, Fight, Filter, GainControl, GainLife, GrantAbility,
    LookAt, LoseLife, Mill, Modification, Reanimate, Recurse,
    Reveal, Sacrifice, Sequence, TapEffect, Tutor, UntapEffect,
    CreateToken, CopyPermanent,
    TARGET_ANY, TARGET_CREATURE, TARGET_PLAYER, SELF,
)
from parser import parse_filter, _NUM_WORDS  # noqa: E402

EFFECT_RULES: list = []


def _r(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


def _pf(s: str, default_base: str = "creature") -> Filter:
    f = parse_filter(s.strip())
    if f is None:
        return Filter(base=default_base, targeted="target" in s.lower())
    return f


def _num(s: str) -> int | str:
    if s is None:
        return 1
    n = _NUM_WORDS.get(s.lower(), s)
    if isinstance(n, str) and n.isdigit():
        return int(n)
    return n


# ---------------------------------------------------------------------------
# Self-source damage: "this creature/enchantment/permanent deals N damage to"
# ~400 residuals. Existing rules only match "~ deals N damage to".
# ---------------------------------------------------------------------------

_SELF_SUBJ = r"(?:this (?:creature|enchantment|permanent|artifact)|it|that creature|that permanent)"

@_r(rf"^{_SELF_SUBJ} deals? (\d+|x) damage to ([^.]+?)(?:\.|$)")
def _self_dmg_fixed(m):
    amt = _num(m.group(1))
    return Damage(amount=amt, target=_pf(m.group(2), "any"))

@_r(rf"^{_SELF_SUBJ} deals? damage equal to (?:its|their) power to ([^.]+?)(?:\.|$)")
def _self_dmg_power(m):
    return Damage(amount="var", target=_pf(m.group(1), "any"))

@_r(rf"^{_SELF_SUBJ} deals? damage equal to (?:the number of|its|their|the) [^.]+ to ([^.]+?)(?:\.|$)")
def _self_dmg_equal(m):
    return Damage(amount="var", target=_pf(m.group(1), "any"))

@_r(rf"^{_SELF_SUBJ} deals? (?:that much|x) damage to ([^.]+?)(?:\.|$)")
def _self_dmg_that_much(m):
    return Damage(amount="var", target=_pf(m.group(1), "any"))

# "it deals N damage to each opponent/creature" — compound target
@_r(rf"^{_SELF_SUBJ} deals? (\d+) damage to each ([^.]+?)(?:\.|$)")
def _self_dmg_each(m):
    n = int(m.group(1))
    return Damage(amount=n, target=_pf("each " + m.group(2), "any"))

# "this creature deals N damage to any target and N damage to you"
@_r(rf"^{_SELF_SUBJ} deals? (\d+) damage to ([^.]+?) and (\d+) damage to ([^.]+?)(?:\.|$)")
def _self_dmg_split(m):
    a = Damage(amount=int(m.group(1)), target=_pf(m.group(2), "any"))
    b = Damage(amount=int(m.group(3)), target=_pf(m.group(4), "any"))
    return Sequence(items=(a, b))


# ---------------------------------------------------------------------------
# Gain control: "gain control of target X [until end of turn]"
# ~80 residuals
# ---------------------------------------------------------------------------

@_r(r"^gain control of (target [^.]+?) until end of turn(?:\.|$)")
def _gain_control_eot(m):
    return GainControl(target=_pf(m.group(1)), duration="until_end_of_turn")

@_r(r"^gain control of (target [^.]+?)(?:\.|$)")
def _gain_control(m):
    return GainControl(target=_pf(m.group(1)))

@_r(r"^gain control of (it|that creature|that permanent) until end of turn(?:\.|$)")
def _gain_control_it_eot(m):
    return GainControl(target=TARGET_CREATURE, duration="until_end_of_turn")

@_r(r"^gain control of (it|that creature|that permanent)(?:\.|$)")
def _gain_control_it(m):
    return GainControl(target=TARGET_CREATURE)


# ---------------------------------------------------------------------------
# Put onto battlefield: "put a creature card ... onto the battlefield"
# ~70 residuals
# ---------------------------------------------------------------------------

@_r(r"^(?:you may )?put (a [^.]+? card) from (?:your|a|any|their|an opponent's) (?:hand|graveyard|library|the top of your library) onto the battlefield(?:\s+tapped)?(?:\s+under [^.]+)?(?:\.|$)")
def _put_onto_bf(m):
    return Reanimate(target=_pf(m.group(1), "card"))

@_r(r"^(?:you may )?put (target [^.]+? card) (?:from [^.]+ )?onto the battlefield(?:\s+tapped)?(?:\.|$)")
def _put_target_onto_bf(m):
    return Reanimate(target=_pf(m.group(1), "card"))

@_r(r"^(?:you may )?put (?:it|that card|that creature) onto the battlefield(?:\s+tapped)?(?:\s+under [^.]+)?(?:\.|$)")
def _put_it_onto_bf(m):
    return Reanimate(target=TARGET_CREATURE)


# ---------------------------------------------------------------------------
# Sacrifice unless: "sacrifice this X unless you pay {N}"
# ~50 residuals
# ---------------------------------------------------------------------------

@_r(r"^sacrifice (?:~|this creature|this enchantment|this artifact|this permanent|this land) unless you pay [^.]+(?:\.|$)")
def _sac_unless(m):
    return Sacrifice(query=SELF)


# ---------------------------------------------------------------------------
# Self buff until EOT: "this creature/it gets +N/+N until end of turn"
# ~70 residuals. Existing rules only match "~ gets +N/+N" (with ~)
# ---------------------------------------------------------------------------

@_r(rf"^{_SELF_SUBJ} gets? ([+-]\d+)/([+-]\d+) until end of turn(?:\.|$)")
def _self_buff_eot(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)), target=SELF)

# With trailing clause: "this creature gets +1/+0 until end of turn for each X"
@_r(rf"^{_SELF_SUBJ} gets? ([+-]\d+)/([+-]\d+) until end of turn(?:,| for| and)? [^.]+(?:\.|$)")
def _self_buff_eot_scaled(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)), target=SELF)

# Gain keyword: "it gains flying until end of turn"
@_r(rf"^{_SELF_SUBJ} gains? ([a-z ]+?) until end of turn(?:\.|$)")
def _self_grant_eot(m):
    return GrantAbility(ability_name=m.group(1).strip(), target=SELF)


# ---------------------------------------------------------------------------
# Mill: "target player/each opponent/you mill N"
# ~25 residuals. Existing rules only match "mill N" without subject.
# ---------------------------------------------------------------------------

@_r(r"^(target player|target opponent|each opponent|each player|that player|you) mills? (\d+|x|one|two|three|four|five|six|seven|eight|nine|ten) cards?(?:\.|$)")
def _mill_targeted(m):
    return Mill(count=_num(m.group(2)), target=_pf(m.group(1), "player"))


# ---------------------------------------------------------------------------
# Fight: "it/target creature fights target/another creature"
# ~40 residuals
# ---------------------------------------------------------------------------

@_r(rf"^{_SELF_SUBJ} fights? (target [^.]+?)(?:\.|$)")
def _fight_self(m):
    return Fight(a=SELF, b=_pf(m.group(1)))

@_r(r"^(target creature) fights? ((?:another )?target [^.]+?)(?:\.|$)")
def _fight_targets(m):
    return Fight(a=_pf(m.group(1)), b=_pf(m.group(2)))

@_r(rf"^{_SELF_SUBJ} fights? (?:up to (?:one|two|\d+) )?(?:other )?target [^.]+(?:\.|$)")
def _fight_self_multi(m):
    return Fight(a=SELF, b=TARGET_CREATURE)


# ---------------------------------------------------------------------------
# Exile: "exile each/all X" — broader patterns not covered by base rules
# ~90 residuals
# ---------------------------------------------------------------------------

@_r(r"^exile each ([^.]+?)(?:\.|$)")
def _exile_each(m):
    return Exile(target=_pf("each " + m.group(1)))

@_r(r"^exile (all [^.]+?)(?:\.|$)")
def _exile_all(m):
    return Exile(target=_pf(m.group(1)))

@_r(r"^exile (up to (?:one|two|three|x|\d+) target [^.]+?)(?:\.|$)")
def _exile_up_to(m):
    return Exile(target=_pf(m.group(1)))

@_r(r"^exile (any number of target [^.]+?)(?:\.|$)")
def _exile_any_number(m):
    return Exile(target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# Return from graveyard: broader patterns
# ~50 residuals
# ---------------------------------------------------------------------------

@_r(r"^return (a [^.]+? card) (?:at random )?from (?:your|a|any|their) graveyard to (?:your hand|the battlefield|its owner's hand)(?:\s+tapped)?(?:\.|$)")
def _return_gy_card(m):
    text = m.group(0).lower()
    if "battlefield" in text:
        return Reanimate(target=_pf(m.group(1), "card"))
    return Recurse(target=_pf(m.group(1), "card"))

@_r(r"^return (up to (?:one|two|three|x|\d+) target [^.]+? cards?) from (?:your|a|any) graveyard to (?:your hand|the battlefield)(?:\.|$)")
def _return_gy_multi(m):
    text = m.group(0).lower()
    if "battlefield" in text:
        return Reanimate(target=_pf(m.group(1), "card"))
    return Recurse(target=_pf(m.group(1), "card"))


# ---------------------------------------------------------------------------
# Each opponent/player effects
# ~40 residuals
# ---------------------------------------------------------------------------

@_r(r"^each opponent (?:loses?) (\d+|x|one|two|three|four|five) life(?:\.|$)")
def _each_opp_loses(m):
    return LoseLife(amount=_num(m.group(1)), target=_pf("each opponent", "player"))

@_r(r"^each player (?:loses?) (\d+|x|one|two|three|four|five) life(?:\.|$)")
def _each_player_loses(m):
    return LoseLife(amount=_num(m.group(1)), target=_pf("each player", "player"))

@_r(r"^each opponent discards? (\d+|a|one|two|three) cards?(?:\.|$)")
def _each_opp_discards(m):
    n = 1 if m.group(1) == "a" else _num(m.group(1))
    return Discard(count=n, target=_pf("each opponent", "player"))

@_r(r"^each player discards? (\d+|a|one|two|three) cards?(?:\.|$)")
def _each_player_discards(m):
    n = 1 if m.group(1) == "a" else _num(m.group(1))
    return Discard(count=n, target=_pf("each player", "player"))

@_r(r"^each opponent sacrifices? (?:a |an )?([^.]+?)(?:\.|$)")
def _each_opp_sacs(m):
    return Sacrifice(query=_pf(m.group(1)))

@_r(r"^each player sacrifices? (?:a |an )?([^.]+?)(?:\.|$)")
def _each_player_sacs(m):
    return Sacrifice(query=_pf(m.group(1)))

# Compound: "each opponent loses N life and you gain N life"
@_r(r"^each opponent loses? (\d+) life and you gain (\d+) life(?:\.|$)")
def _opp_lose_you_gain(m):
    a = LoseLife(amount=int(m.group(1)), target=_pf("each opponent", "player"))
    b = GainLife(amount=int(m.group(2)))
    return Sequence(items=(a, b))


# ---------------------------------------------------------------------------
# Target creature gets -N/-N until end of turn (negative buff / removal-like)
# Already exists for +N/+N but not -N/-N in base rules
# ---------------------------------------------------------------------------

@_r(r"^(target (?:creature|permanent)[^.]*?) gets? (-\d+)/(-\d+) until end of turn(?:\.|$)")
def _target_debuff(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=_pf(m.group(1)))

@_r(r"^(target creature an opponent controls[^.]*?) gets? (-\d+)/(-\d+) until end of turn(?:\.|$)")
def _target_opp_debuff(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)),
                target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# Untap/tap target broader patterns
# ---------------------------------------------------------------------------

@_r(r"^untap (each [^.]+?)(?:\.|$)")
def _untap_each(m):
    return UntapEffect(target=_pf(m.group(1)))

@_r(r"^untap (all [^.]+?)(?:\.|$)")
def _untap_all(m):
    return UntapEffect(target=_pf(m.group(1)))

@_r(r"^tap (each [^.]+?)(?:\.|$)")
def _tap_each(m):
    return TapEffect(target=_pf(m.group(1)))

@_r(r"^tap (all [^.]+?)(?:\.|$)")
def _tap_all(m):
    return TapEffect(target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# Destroy broader: "destroy that creature/permanent" (not "target")
# ---------------------------------------------------------------------------

@_r(r"^destroy (that (?:creature|permanent|artifact|enchantment|planeswalker)[^.]*)(?:\.|$)")
def _destroy_that(m):
    return Destroy(target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# "Until end of turn, target creature gets +N/+N and gains KEYWORD"
# ~50 residuals — needs Buff + GrantAbility as Sequence
# ---------------------------------------------------------------------------

@_r(r"^until end of turn, (target creature[^.]*?) gets? ([+-]\d+)/([+-]\d+) and gains? ([a-z, ]+?)(?:\.|$)")
def _eot_target_buff_grant(m):
    buff = Buff(power=int(m.group(2)), toughness=int(m.group(3)), target=_pf(m.group(1)))
    kws = [k.strip() for k in re.split(r",\s*(?:and\s+)?|\s+and\s+", m.group(4)) if k.strip()]
    if len(kws) == 1:
        return Sequence(items=(buff, GrantAbility(ability_name=kws[0], target=_pf(m.group(1)))))
    items = [buff] + [GrantAbility(ability_name=kw, target=_pf(m.group(1))) for kw in kws]
    return Sequence(items=tuple(items))

# "until end of turn, target creature gets +N/+N" (no keyword)
@_r(r"^until end of turn, (target creature[^.]*?) gets? ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _eot_target_buff_bare(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)), target=_pf(m.group(1)))

# "until end of turn, target creature gains KEYWORD"
@_r(r"^until end of turn, (target creature[^.]*?) gains? ([a-z, ]+?)(?:\.|$)")
def _eot_target_grant(m):
    kws = [k.strip() for k in re.split(r",\s*(?:and\s+)?|\s+and\s+", m.group(2)) if k.strip()]
    if len(kws) == 1:
        return GrantAbility(ability_name=kws[0], target=_pf(m.group(1)))
    return Sequence(items=tuple(GrantAbility(ability_name=kw, target=_pf(m.group(1))) for kw in kws))

# "until end of turn, creatures you control get +N/+N"
@_r(r"^until end of turn, creatures you control get ([+-]\d+)/([+-]\d+)(?:\.|$)")
def _eot_team_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=_pf("each creature you control"))

# "until end of turn, creatures you control get +N/+N and gain KEYWORD"
@_r(r"^until end of turn, creatures you control get ([+-]\d+)/([+-]\d+) and gain ([a-z, ]+?)(?:\.|$)")
def _eot_team_buff_grant(m):
    tgt = _pf("each creature you control")
    buff = Buff(power=int(m.group(1)), toughness=int(m.group(2)), target=tgt)
    kws = [k.strip() for k in re.split(r",\s*(?:and\s+)?|\s+and\s+", m.group(3)) if k.strip()]
    items = [buff] + [GrantAbility(ability_name=kw, target=tgt) for kw in kws]
    return Sequence(items=tuple(items))

# "until end of turn, creatures you control gain KEYWORD"
@_r(r"^until end of turn, creatures you control gain ([a-z, ]+?)(?:\.|$)")
def _eot_team_grant(m):
    tgt = _pf("each creature you control")
    kws = [k.strip() for k in re.split(r",\s*(?:and\s+)?|\s+and\s+", m.group(1)) if k.strip()]
    if len(kws) == 1:
        return GrantAbility(ability_name=kws[0], target=tgt)
    return Sequence(items=tuple(GrantAbility(ability_name=kw, target=tgt) for kw in kws))


# ---------------------------------------------------------------------------
# Search library — broader tutor patterns
# ~167 residuals
# ---------------------------------------------------------------------------

@_r(r"^search your library for (a [^,]+? card)[^.]*?(?:,\s*)?put (?:it|that card) onto the battlefield(?:\s+tapped)?(?:,\s*then shuffle)?[^.]*(?:\.|$)")
def _search_put_bf(m):
    return Tutor(target=_pf(m.group(1), "card"), destination="battlefield")

@_r(r"^search your library for (up to (?:one|two|three|x|\d+) [^,]+? cards?)[^.]*?(?:,\s*)?put (?:them|those cards?) onto the battlefield(?:\s+tapped)?(?:,\s*then shuffle)?[^.]*(?:\.|$)")
def _search_put_multi_bf(m):
    return Tutor(target=_pf(m.group(1), "card"), destination="battlefield")

@_r(r"^search your library (?:and/or graveyard |and graveyard )?for (a [^,]+? card)[^.]*?(?:,\s*)?put (?:it|that card) into your hand(?:,\s*then shuffle)?[^.]*(?:\.|$)")
def _search_to_hand(m):
    return Tutor(target=_pf(m.group(1), "card"), destination="hand")

@_r(r"^search your library for (up to (?:one|two|three|x|\d+) [^,]+? cards?)[^.]*?(?:,\s*)?(?:reveal (?:them|those cards?)(?:,\s*)?)?put (?:them|those cards?) into your hand(?:,\s*then shuffle)?[^.]*(?:\.|$)")
def _search_multi_hand(m):
    return Tutor(target=_pf(m.group(1), "card"), destination="hand")

@_r(r"^search your library for (any number of [^,]+? cards?)[^.]*?(?:,\s*)?put (?:them|those cards?) (?:onto the battlefield|into your hand)(?:\s+tapped)?(?:,\s*then shuffle)?[^.]*(?:\.|$)")
def _search_any_number(m):
    dest = "battlefield" if "battlefield" in m.group(0) else "hand"
    return Tutor(target=_pf(m.group(1), "card"), destination=dest)


# ---------------------------------------------------------------------------
# "you may play/cast that card [this turn / until end of turn]"
# ~90 residuals — these are impulse-draw effects
# ---------------------------------------------------------------------------

@_r(r"^you may (?:play|cast) (?:that card|it|those cards?|them|the exiled card)[^.]*(?:\.|$)")
def _may_play_impulse(m):
    return Modification(kind="impulse_play", args=(m.group(0).strip().rstrip("."),))

@_r(r"^you may (?:play|cast) (?:a |an |one )?(?:card|spell|instant|sorcery)[^.]*(?:\.|$)")
def _may_cast_generic(m):
    return Modification(kind="impulse_play", args=(m.group(0).strip().rstrip("."),))


# ---------------------------------------------------------------------------
# Equipped creature gets +N/+N [for each X]
# ~18 residuals
# ---------------------------------------------------------------------------

@_r(r"^equipped creature gets? ([+-]\d+)/([+-]\d+)(?:\s+for each [^.]+)?(?:\.|$)")
def _equipped_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=_pf("equipped creature"))

@_r(r"^enchanted creature gets? ([+-]\d+)/([+-]\d+)(?:\s+for each [^.]+)?(?:\.|$)")
def _enchanted_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)),
                target=_pf("enchanted creature"))

# Equipped/enchanted creature gains keyword
@_r(r"^(?:equipped|enchanted) creature (?:has|gains?) ([a-z, ]+?)(?:\.|$)")
def _equip_enchant_grant(m):
    kw = m.group(1).strip()
    target = _pf("equipped creature") if "equipped" in m.group(0).lower() else _pf("enchanted creature")
    return GrantAbility(ability_name=kw, target=target)


# ---------------------------------------------------------------------------
# "return that card to the battlefield" / "return it to the battlefield"
# ~14 residuals
# ---------------------------------------------------------------------------

@_r(r"^return (?:that card|it|that creature|that permanent) to the battlefield(?:\s+tapped)?(?:\s+under [^.]+)?(?:\.|$)")
def _return_to_bf(m):
    return Reanimate(target=TARGET_CREATURE)

@_r(r"^return (?:that card|it) to (?:your|its owner's) hand(?:\.|$)")
def _return_to_hand(m):
    return Bounce(target=TARGET_CREATURE)


# ---------------------------------------------------------------------------
# Goad: "goad target/each/all creature(s)"
# ~15 residuals
# ---------------------------------------------------------------------------

@_r(r"^goad (target [^.]+?)(?:\.|$)")
def _goad_target(m):
    return Modification(kind="goad", args=(_pf(m.group(1)),))

@_r(r"^goad (each [^.]+?)(?:\.|$)")
def _goad_each(m):
    return Modification(kind="goad", args=(_pf(m.group(1)),))

@_r(r"^goad (all [^.]+?)(?:\.|$)")
def _goad_all(m):
    return Modification(kind="goad", args=(_pf(m.group(1)),))


# ---------------------------------------------------------------------------
# "look at the top N cards of your library" — broader LookAt
# ~26 residuals
# ---------------------------------------------------------------------------

@_r(r"^look at the top (\d+|x|one|two|three|four|five|six|seven) cards? of (?:your|target player's) library[^.]*(?:\.|$)")
def _look_at_top(m):
    return LookAt(zone="library_top_n")

@_r(r"^reveal the top (\d+|x|one|two|three|four|five|six|seven) cards? of (?:your|target player's) library[^.]*(?:\.|$)")
def _reveal_top(m):
    return Reveal(source="top_of_library", count=_num(m.group(1)))


# ---------------------------------------------------------------------------
# "create a tapped powerstone/treasure/clue/food/blood token"
# ~23+ residuals
# ---------------------------------------------------------------------------

@_r(r"^create (?:a |an |(\d+|two|three|four|five) )?(tapped )?(?:(powerstone|treasure|clue|food|blood|map|gold|shard|incubator|junk) )tokens?(?:\.|$)")
def _create_named_token(m):
    n = _num(m.group(1)) if m.group(1) else 1
    tapped = bool(m.group(2))
    ttype = m.group(3)
    return CreateToken(count=n, types=(ttype,), tapped=tapped)


# Broad creature token: "create a/N P/T [color...] [type...] creature token [with ...]"
# Handles multi-word color+type, artifact creature, "with" keyword/ability clauses
_CREATE_BROAD = re.compile(
    r"^create\s+"
    r"(?:(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+)?"
    r"(?:tapped\s+)?(?:and\s+attacking\s+)?"
    r"(\d+)/(\d+)\s+"
    r"([\w ]+?)\s+"
    r"(?:artifact\s+)?creature\s+tokens?"
    r"(?:\s+with\s+[^.]+)?"
    r"(?:,\s+where\s+[^.]+)?"
    r"(?:\.|$)",
    re.I | re.S,
)

@_r(_CREATE_BROAD.pattern)
def _create_broad_token(m):
    p, t = int(m.group(1)), int(m.group(2))
    type_text = m.group(3).strip()
    colors = []
    types = []
    kws = ()
    for word in type_text.split():
        w = word.lower()
        if w in ("white", "blue", "black", "red", "green", "colorless", "multicolored"):
            colors.append({"white": "W", "blue": "U", "black": "B", "red": "R",
                          "green": "G"}.get(w, w))
        elif w not in ("and",):
            types.append(w)
    full = m.group(0).lower()
    wm = re.search(r"with\s+(flying|trample|haste|vigilance|deathtouch|lifelink|menace|first strike|defender|reach|hexproof|indestructible)", full)
    if wm:
        kws = (wm.group(1),)
    cnt_m = re.match(r"create\s+(?:a|an|one|(\d+|two|three|four|five|six|seven|eight|nine|ten|x))\s+", m.group(0), re.I)
    n = 1
    if cnt_m and cnt_m.group(1):
        n = _num(cnt_m.group(1))
    tapped = "tapped" in m.group(0).lower().split("create")[1].split(m.group(1))[0] if m.group(1) else False
    return CreateToken(count=n, pt=(p, t), types=tuple(types), color=tuple(colors),
                       keywords=kws, tapped=bool(tapped))


# "create a token that's a copy of TARGET" — broader than base rule
@_r(r"^create a token that's a copy of ([^.]+?)(?:\.|$)")
def _create_copy_token(m):
    return CopyPermanent(target=_pf(m.group(1)))

# "create X ... tokens, where X is ..." with variable count
@_r(r"^create x\s+(\d+)/(\d+)\s+([\w ]+?)\s+(?:artifact\s+)?creature\s+tokens?,\s+where\s+x\s+is\s+[^.]+(?:\.|$)")
def _create_x_tokens(m):
    p, t = int(m.group(1)), int(m.group(2))
    type_text = m.group(3).strip()
    types = [w for w in type_text.split() if w.lower() not in ("white", "blue", "black", "red", "green", "colorless", "and")]
    colors = [{"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G"}[w.lower()]
              for w in type_text.split() if w.lower() in ("white", "blue", "black", "red", "green")]
    return CreateToken(count="x", pt=(p, t), types=tuple(types), color=tuple(colors))


# "create a ROLE role token attached to ..."
@_r(r"^create (?:a |an )([\w ]+?) role token(?:\s+attached to [^.]+)?(?:\.|$)")
def _create_role_token(m):
    return CreateToken(count=1, types=(m.group(1).strip() + " role",))

# "create a lander/mutagen/~ token"
@_r(r"^create (?:a |an )(?:tapped )?([\w~][\w ]*?) tokens?(?:\.|$)")
def _create_misc_token(m):
    return CreateToken(count=1, types=(m.group(1).strip(),))


# ---------------------------------------------------------------------------
# Extra land per turn
# ~18 residuals
# ---------------------------------------------------------------------------

@_r(r"^you may play an? additional land (?:on each of your turns|this turn|each turn)(?:\.|$)")
def _extra_land(m):
    return Modification(kind="extra_land_drop", args=(1,))


# ---------------------------------------------------------------------------
# "exile the top N cards of your/target player's library"
# ~13 residuals
# ---------------------------------------------------------------------------

@_r(r"^exile the top (\d+|x|one|two|three|four|five|six|seven|ten) cards? of (?:your|target player's|each player's|each opponent's) library(?:\.|$)")
def _exile_top_library(m):
    return Exile(target=_pf("card"))


# ---------------------------------------------------------------------------
# Variable buff: "it/target creature gets +X/+X until eot, where X is ..."
# ~162 targeted + ~70 self = ~232 residuals
# ---------------------------------------------------------------------------

# Target creature gets +X/+X (variable) until end of turn
@_r(r"^(target creature[^.]*?) gets? \+x/\+([x0]) until end of turn(?:,\s+where [^.]+)?(?:\.|$)")
def _target_var_buff(m):
    return Buff(power=0, toughness=0, target=_pf(m.group(1)))

@_r(r"^(target creature[^.]*?) gets? \-x/\-x until end of turn(?:,\s+where [^.]+)?(?:\.|$)")
def _target_var_debuff(m):
    return Buff(power=0, toughness=0, target=_pf(m.group(1)))

# Self gets +X/+X (variable)
@_r(rf"^{_SELF_SUBJ} gets? \+x/\+([x0]) until end of turn(?:,\s+where [^.]+)?(?:\.|$)")
def _self_var_buff(m):
    return Buff(power=0, toughness=0, target=SELF)

# "it gets +N/+0 until end of turn for each X"
@_r(rf"^{_SELF_SUBJ} gets? ([+-]\d+)/([+-]\d+) until end of turn for each [^.]+(?:\.|$)")
def _self_scaled_buff(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)), target=SELF)

# "target creature gets +N/+N until end of turn for each X"
@_r(r"^(target creature[^.]*?) gets? ([+-]\d+)/([+-]\d+) until end of turn for each [^.]+(?:\.|$)")
def _target_scaled_buff(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)), target=_pf(m.group(1)))

# "up to N target creatures each get +N/+N until end of turn"
@_r(r"^(up to (?:one|two|three|x|\d+) target creatures?) (?:each )?gets? ([+-]\d+)/([+-]\d+) until end of turn(?:\.|$)")
def _multi_target_buff(m):
    return Buff(power=int(m.group(2)), toughness=int(m.group(3)), target=_pf(m.group(1)))


# ---------------------------------------------------------------------------
# "until end of turn, TARGET becomes a N/N TYPE creature"
# ~93 residuals (animation)
# ---------------------------------------------------------------------------

@_r(r"^until end of turn, (?:target |this )(?:land|artifact|permanent|noncreature artifact)[^.]*? becomes? (?:a |an )?(\d+)/(\d+) [^.]+(?:creature|land creature)[^.]*(?:\.|$)")
def _animate_to_creature(m):
    return Modification(kind="animate", args=(int(m.group(1)), int(m.group(2))))

@_r(r"^until end of turn, (?:this land|target land[^.]*?) becomes? a (\d+)/(\d+) [^.]+(?:\.|$)")
def _land_animation(m):
    return Modification(kind="animate", args=(int(m.group(1)), int(m.group(2))))


# ---------------------------------------------------------------------------
# "until end of turn, [it/this creature/target creature] has base power
#  and toughness N/N"
# ~20 residuals
# ---------------------------------------------------------------------------

@_r(r"^until end of turn, (?:target creature|this creature|it|~)[^.]*? has base power and toughness (\d+)/(\d+)(?:\.|$)")
def _set_base_pt(m):
    return Buff(power=int(m.group(1)), toughness=int(m.group(2)), target=SELF)


# ---------------------------------------------------------------------------
# "target creature/player loses N life"  (broader than base rule)
# ~45 residuals
# ---------------------------------------------------------------------------

@_r(r"^(target (?:creature|player|opponent)[^.]*?) loses? (\d+|x) life(?:\.|$)")
def _target_loses_life(m):
    return LoseLife(amount=_num(m.group(2)), target=_pf(m.group(1), "player"))


# ---------------------------------------------------------------------------
# "target creature/player gains N life"
# ---------------------------------------------------------------------------

@_r(r"^(target player[^.]*?) gains? (\d+|x) life(?:\.|$)")
def _target_gains_life(m):
    return GainLife(amount=_num(m.group(2)))


# ---------------------------------------------------------------------------
# "target creature can't block [this turn / until eot]"
# ~112 restriction residuals
# ---------------------------------------------------------------------------

@_r(r"^(target creature[^.]*?) can'?t (?:block|attack|be blocked|be regenerated)[^.]*(?:\.|$)")
def _target_restriction(m):
    return Modification(kind="restriction", args=(m.group(0).strip().rstrip("."),))


# ---------------------------------------------------------------------------
# "target creature's controller ... / target opponent ..."
# broader targeted effects
# ---------------------------------------------------------------------------

@_r(r"^(target opponent[^.]*?) (?:loses?|discards?|sacrifices?|mills?) [^.]+(?:\.|$)")
def _target_opp_effect(m):
    text = m.group(0).lower()
    if "loses" in text:
        n_m = re.search(r"loses?\s+(\d+)\s+life", text)
        if n_m:
            return LoseLife(amount=int(n_m.group(1)), target=_pf("target opponent", "player"))
    if "discards" in text:
        n_m = re.search(r"discards?\s+(\d+|a|an)\s+cards?", text)
        n = 1
        if n_m and n_m.group(1) not in ("a", "an"):
            n = int(n_m.group(1))
        return Discard(count=n, target=_pf("target opponent", "player"))
    if "sacrifices" in text:
        return Sacrifice(query=_pf("creature"))
    if "mills" in text:
        n_m = re.search(r"mills?\s+(\d+)", text)
        n = 1 if not n_m else int(n_m.group(1))
        return Mill(count=n, target=_pf("target opponent", "player"))
    return Modification(kind="targeted_effect", args=(m.group(0).strip().rstrip("."),))


# ---------------------------------------------------------------------------
# Round 3: broader patterns for the long tail
# ---------------------------------------------------------------------------

# "return it/that card to the battlefield [under your/its owner's control]"
@_r(r"^return (?:it|that card|that creature|the exiled card|this card|this creature) to the battlefield(?:\s+(?:tapped\s+)?under [^.]+)?(?:\s+tapped)?(?:\.|$)")
def _return_it_bf(m):
    return Reanimate(target=TARGET_CREATURE)

# "return it/this card from your graveyard to the battlefield"
@_r(r"^return (?:it|this card|this creature|that card) from (?:your|the|a) graveyard to the battlefield(?:\s+tapped)?(?:\.|$)")
def _return_self_from_gy(m):
    return Reanimate(target=SELF)

# "return this card from your graveyard to your hand"
@_r(r"^return (?:it|this card|that card) from (?:your|the) graveyard to (?:your|its owner's) hand(?:\.|$)")
def _return_from_gy_hand(m):
    return Recurse(target=SELF)

# "draw cards equal to the number of ..."
@_r(r"^draw cards? equal to (?:the number of|the|its|their|that) [^.]+(?:\.|$)")
def _draw_variable(m):
    return Draw(count="var")

# "discard your hand, then draw N cards" / "discard N, then draw N"
@_r(r"^discard (?:your hand|a card|two cards?|\d+ cards?), then draw (?:a card|(\d+|one|two|three|four|five|that many|as many) cards?)(?:\.|$)")
def _discard_then_draw(m):
    d_count = 1
    draw_n = _num(m.group(1)) if m.group(1) and m.group(1) not in ("that many", "as many") else "var"
    text = m.group(0).lower()
    if "your hand" in text:
        d_count = "var"
    elif "two" in text.split(",")[0]:
        d_count = 2
    elif re.match(r"discard (\d+)", text):
        d_count = int(re.match(r"discard (\d+)", text).group(1))
    return Sequence(items=(Discard(count=d_count), Draw(count=draw_n)))

# "draw a card, then [discard a card / you may discard]"
@_r(r"^draw (?:a card|(\d+|two|three) cards?), then (?:you may )?discard (?:a card|(\d+|one|two) cards?)(?:\.|$)")
def _draw_then_discard(m):
    draw_n = _num(m.group(1)) if m.group(1) else 1
    disc_n = _num(m.group(2)) if m.group(2) else 1
    return Sequence(items=(Draw(count=draw_n), Discard(count=disc_n)))

# "put that card onto/into the battlefield/your hand/top/bottom of library"
@_r(r"^put (?:that card|those cards?|it) (?:onto the battlefield|into your hand|on (?:top|the bottom) of (?:your|its owner's) library)(?:\s+tapped)?(?:\.|$)")
def _put_card_destination(m):
    text = m.group(0).lower()
    if "battlefield" in text:
        return Reanimate(target=TARGET_CREATURE)
    if "into your hand" in text:
        return Recurse(target=TARGET_CREATURE)
    return Modification(kind="library_manipulation", args=(text.strip().rstrip("."),))

# "you may put a creature/permanent/artifact card from your hand onto the battlefield"
@_r(r"^you may put (?:a |an )([\w ]+? card) from your hand onto the battlefield(?:\s+tapped)?(?:\.|$)")
def _put_from_hand(m):
    return Reanimate(target=_pf(m.group(1), "card"))

# "sacrifice it unless you [return/pay/...]"
@_r(r"^sacrifice (?:it|that creature|that permanent) unless [^.]+(?:\.|$)")
def _sac_it_unless(m):
    return Sacrifice(query=TARGET_CREATURE)

# "sacrifice this creature unless you [return/pay]"  (broader than "unless you pay")
@_r(r"^sacrifice (?:~|this creature|this enchantment|this artifact|this permanent) unless [^.]+(?:\.|$)")
def _sac_self_unless_broad(m):
    return Sacrifice(query=SELF)

# "draw a card if you [condition]"
@_r(r"^draw a card if [^.]+(?:\.|$)")
def _draw_conditional(m):
    return Draw(count=1)

# "put a [+1/+1 / -1/-1 / counter_type] counter on [target]"
@_r(r"^put (?:a |an |(\d+|one|two|three|four|five|x) )?(\+\d+/\+\d+|-\d+/-\d+|[\w]+) counters? on ([^.]+?)(?:\.|$)")
def _put_counter_broad(m):
    n = _num(m.group(1)) if m.group(1) else 1
    counter_kind = m.group(2)
    target = _pf(m.group(3))
    return CounterMod(op="put", count=n, counter_kind=counter_kind, target=target)

# "choose up to one/two target ..." — treated as targeted modal
@_r(r"^choose (?:up to )?(?:one|two|three) target [^.]+(?:\.|$)")
def _choose_target(m):
    return Modification(kind="choose_target", args=(m.group(0).strip().rstrip("."),))

# "you may have it/this creature deal damage equal to its power to ..."
@_r(r"^you may have (?:it|this creature|~) deal damage equal to (?:its|their) power to ([^.]+?)(?:\.|$)")
def _may_have_deal_power(m):
    return Damage(amount="var", target=_pf(m.group(1), "any"))
