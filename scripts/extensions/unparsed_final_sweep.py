#!/usr/bin/env python3
"""UNPARSED-bucket final sweep — third pass.

Family: UNPARSED -> GREEN/PARTIAL promotions.

After ``unparsed_residual.py`` and ``unparsed_residual_2.py`` the UNPARSED
bucket sits near ~791 cards (2.50%). Most remaining cards are singletons
or 2-3 card clusters of phrasings no prior pass anticipated. This file is
a broad catch-all net: specific enough per-pattern that we don't risk
false positives on already-GREEN cards (extensions run first, but we still
emit Static(Modification(kind=...)) with descriptive kinds so downstream
consumers can ignore them when they want precise semantics).

Standard exports: STATIC_PATTERNS, EFFECT_RULES, TRIGGER_PATTERNS.
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
    Filter, GainControl, GrantAbility, Keyword, LoseLife, Mill,
    Modification, Prevent, Recurse, Sacrifice, Sequence, Static, TapEffect,
    UntapEffect, UnknownEffect,
    TARGET_ANY, TARGET_CREATURE, TARGET_OPPONENT, TARGET_PLAYER,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}

_COLOR = r"(?:white|blue|black|red|green|colorless|multicolored|monocolored)"


def _num(tok):
    tok = (tok or "").strip().lower()
    if tok.isdigit():
        return int(tok)
    return _NUM_WORDS.get(tok, tok)


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
# Color/type anti-anthems (more shapes)
# ---------------------------------------------------------------------------

# "non<color> creatures get -N/-N (until end of turn)?" (e.g. Festergloom)
# / "non<type> creatures get -N/-N" (e.g. Eyeblight Massacre, Stench of Decay)
@_sp(r"^non([a-z][a-z\- ]+?) creatures? get ([+-]\d+)/([+-]\d+)"
     r"(?: until end of turn)?\s*$")
def _non_typed_pt(m, raw):
    return Static(modification=Modification(
        kind="non_typed_global_pt",
        args=(m.group(1).strip(), int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "<color> creatures get +N/+N (until end of turn)?" unscoped (e.g. Valorous
# Charge: "white creatures get +2/+0", Nocturnal Raid, Holy Light)
@_sp(rf"^({_COLOR}) creatures? get ([+-]\d+)/([+-]\d+)"
     r"(?: until end of turn)?\s*$")
def _color_unscoped_pt(m, raw):
    return Static(modification=Modification(
        kind="color_global_pt",
        args=(m.group(1).lower(), int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "creatures target player controls get +N/+N (until end of turn)?"
# (Arms of Hadar: "creatures target player controls get -2/-2 until end of turn")
@_sp(r"^creatures target player controls get ([+-]\d+)/([+-]\d+)"
     r"(?: until end of turn)?\s*$")
def _target_player_creatures_pt(m, raw):
    return Static(modification=Modification(
        kind="target_player_creatures_pt",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "creature tokens get -N/-N" / "creature tokens get +N/+N"
# Virulent Plague, Illness in the Ranks, etc.
@_sp(r"^creature tokens get ([+-]\d+)/([+-]\d+)\s*$")
def _token_anthem(m, raw):
    return Static(modification=Modification(
        kind="token_anthem",
        args=(int(m.group(1)), int(m.group(2)))),
        raw=raw)


# "attacking creatures you control get +N/+N (and have <kw>)?"
# Gruul War Chant, Dire Fleet Neckbreaker, Ferocity of the Wilds
@_sp(r"^attacking (?:([a-z\- ]+?) )?(?:creatures? you control|"
     r"creatures?) get \+(\d+)/\+(\d+)"
     r"(?: and have ([^.]+?))?\s*$")
def _attacking_anthem(m, raw):
    return Static(modification=Modification(
        kind="attacking_anthem",
        args=(((m.group(1) or "").strip() or None),
              int(m.group(2)), int(m.group(3)),
              ((m.group(4) or "").strip() or None))),
        raw=raw)


# "creatures with no abilities get +N/+N" / "creatures with no counters on
# them get -N/-N until end of turn" — Muraganda Petroglyphs / Hazardous Conditions
@_sp(r"^creatures with no (abilities|counters(?: on them)?) get "
     r"([+-]\d+)/([+-]\d+)(?: until end of turn)?\s*$")
def _no_abilities_anthem(m, raw):
    return Static(modification=Modification(
        kind="no_quality_anthem",
        args=(m.group(1).strip(), int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "other <type>s you control get +N/+N" (Merfolk Mistbinder: "other ~ you
# control get +1/+1"; Squirrel Sovereign "other ~s you control get +1/+1")
# base parser's "other ... you control" only matches single-word types.
@_sp(r"^other ([a-z][a-z\- ~]+?) you control get \+(\d+)/\+(\d+)\s*$")
def _other_you_control_get(m, raw):
    return Static(modification=Modification(
        kind="other_you_control_get",
        args=(m.group(1).strip(), int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "other <type>s you control get +N/+N and have <kw>" — compound anthem
@_sp(r"^other ([a-z][a-z\- ~]+?) you control get \+(\d+)/\+(\d+) "
     r"and have ([^.]+?)\s*$")
def _other_get_and_have(m, raw):
    return Static(modification=Modification(
        kind="other_get_and_have",
        args=(m.group(1).strip(), int(m.group(2)), int(m.group(3)),
              m.group(4).strip())),
        raw=raw)


# "<type> creatures get +N/+N and have <kw>" — global typed compound anthem
# (Field Marshal: "other soldier creatures get +1/+1 and have first strike")
@_sp(r"^(?:other )?([a-z][a-z\- ]+?) creatures? get \+(\d+)/\+(\d+) "
     r"and have ([^.]+?)\s*$")
def _typed_get_and_have(m, raw):
    return Static(modification=Modification(
        kind="typed_get_and_have",
        args=(m.group(1).strip(), int(m.group(2)), int(m.group(3)),
              m.group(4).strip())),
        raw=raw)


# "all <type>s get +N/+N" / "all goblins get +1/+1" — Dralnu's Crusade
@_sp(r"^all ([a-z][a-z\- ]+?) get \+(\d+)/\+(\d+)\s*$")
def _all_type_get(m, raw):
    return Static(modification=Modification(
        kind="global_type_get",
        args=(m.group(1).strip(), int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "<type>s you control get +N/+N" — bare tribal anthem missed by base parser
# (Pride of the Perfect: "elves you control get +2/+0", Anaba Spirit Crafter)
@_sp(r"^([a-z][a-z\- ]+?)s? you control get \+(\d+)/\+(\d+)\s*$")
def _typed_you_control_get(m, raw):
    tribe = m.group(1).strip()
    # Avoid swallowing "creatures you control" (handled by core) and single
    # words that are actually colors.
    if tribe in ("creature", "creatures", "white", "blue", "black",
                 "red", "green"):
        return None
    return Static(modification=Modification(
        kind="tribal_anthem_get",
        args=(tribe, int(m.group(2)), int(m.group(3)))),
        raw=raw)


# "<type> creatures have <kw>" (unscoped) — Kavu Monarch, Dense Canopy etc.
# Distinct from the first scrubber's "<type> creatures you control have".
@_sp(r"^([a-z][a-z\- ]+?) creatures? have ([a-z][a-z\- ]+?)\s*$")
def _typed_have_global(m, raw):
    typ = m.group(1).strip()
    kws = m.group(2).strip()
    # Skip colors (handled elsewhere) and "all" (which has its own rule).
    if typ in ("all",):
        return None
    return Static(modification=Modification(
        kind="global_typed_have",
        args=(typ, kws)),
        raw=raw)


# "all <type> have <kw>" — "all slivers have shroud" / "all artifacts have shroud"
@_sp(r"^all ([a-z][a-z\- ]+?) have ([^.]+?)\s*$")
def _all_type_have(m, raw):
    return Static(modification=Modification(
        kind="global_type_have",
        args=(m.group(1).strip(), m.group(2).strip())),
        raw=raw)


# "all <type> are <color>" — "all slivers are colorless" / "all creatures are black"
@_sp(r"^all ([a-z][a-z\- ]+?) are ([a-z][a-z ]+?)\s*$")
def _all_type_are(m, raw):
    return Static(modification=Modification(
        kind="global_type_are",
        args=(m.group(1).strip(), m.group(2).strip())),
        raw=raw)


# "all lands have <kw>" — Terra Eternal ("all lands have indestructible")
@_sp(r"^all lands have ([^.]+?)\s*$")
def _all_lands_have(m, raw):
    return Static(modification=Modification(
        kind="all_lands_have",
        args=(m.group(1).strip(),)),
        raw=raw)


# "<types> you control have <kw>" — Leonin Abunas ("artifacts you control
# have hexproof"), Darksteel Forge ("artifacts you control have
# indestructible"), Hanna's Custody ("all artifacts have shroud" — already
# handled above), Fountain Watch, Spiritual Asylum, Magnetic Flux
# ("artifact creatures you control gain flying until end of turn"), Umbra
# Mystic.
@_sp(r"^([a-z][a-z,\- ]*?) you control (?:have|gain) ([^.]+?)"
     r"(?: until end of turn)?\s*$")
def _typed_you_control_have(m, raw):
    typ = m.group(1).strip()
    if typ in ("creatures", "creature"):
        return None  # core handles this
    return Static(modification=Modification(
        kind="typed_you_control_have",
        args=(typ, m.group(2).strip())),
        raw=raw)


# "<type>s your team control(s)? have <kw>" — Rushblade Commander, Warriors'
# team shape
@_sp(r"^([a-z][a-z\- ]+?) your team controls? have ([^.]+?)\s*$")
def _team_control_have(m, raw):
    return Static(modification=Modification(
        kind="team_control_have",
        args=(m.group(1).strip(), m.group(2).strip())),
        raw=raw)


# "attacking <type> you control have <kw>" — Elderfang Venom ("attacking
# elves you control have deathtouch"), Dire Fleet Neckbreaker variant,
# Throatseeker ("unblocked attacking ninjas you control have lifelink")
@_sp(r"^(?:unblocked )?attacking ([a-z][a-z\- ]+?) you control have "
     r"([^.]+?)\s*$")
def _attacking_typed_have(m, raw):
    return Static(modification=Modification(
        kind="attacking_typed_have",
        args=(m.group(1).strip(), m.group(2).strip())),
        raw=raw)


# "creatures that are <cond> can't attack or block" — Song of Serenity
# ("creatures that are enchanted can't attack or block")
@_sp(r"^creatures that are ([a-z][a-z\- ]+?) can'?t (attack|block|"
     r"attack or block)\s*$")
def _cond_creatures_cant(m, raw):
    return Static(modification=Modification(
        kind="cond_creatures_cant",
        args=(m.group(1).strip(), m.group(2).lower())),
        raw=raw)


# "creatures with power <op> <N> can't attack you" — Reverence
# ("creatures with power 2 or less can't attack you")
@_sp(r"^creatures with (?:power|toughness) (?:\d+ or (?:less|greater)) "
     r"can'?t attack you\s*$")
def _power_threshold_cant_attack(m, raw):
    return Static(modification=Modification(
        kind="power_threshold_cant_attack"),
        raw=raw)


# "cards in graveyards lose all abilities" — Yixlid Jailer
@_sp(r"^cards in graveyards? (?:lose all abilities|have [^.]+?)\s*$")
def _gy_cards_modified(m, raw):
    return Static(modification=Modification(kind="graveyard_cards_mod"),
                  raw=raw)


# "players don't lose unspent mana as steps and phases end" — Upwelling
@_sp(r"^players don'?t lose unspent mana as steps and phases end\s*$")
def _mana_persists(m, raw):
    return Static(modification=Modification(kind="mana_persists"), raw=raw)


# "players skip their <phase> steps" — Eon Hub ("players skip their upkeep
# steps")
@_sp(r"^players skip their ([a-z]+) steps?\s*$")
def _players_skip_phase(m, raw):
    return Static(modification=Modification(
        kind="players_skip_phase",
        args=(m.group(1).strip(),)),
        raw=raw)


# "players can cast spells only during their own turns" — Dosan the Falling
# Leaf, City of Solitude
@_sp(r"^players can cast (?:spells )?(?:and activate abilities )?only "
     r"during [^.]+\s*$")
def _timing_restriction_global(m, raw):
    return Static(modification=Modification(
        kind="global_timing_restriction"),
        raw=raw)


# "players can't cycle cards" — Stabilizer
@_sp(r"^players can'?t cycle cards\s*$")
def _no_cycle(m, raw):
    return Static(modification=Modification(kind="no_cycle"), raw=raw)


# "all creatures are tokens" — Intangible Vibes
@_sp(r"^all creatures are tokens\s*$")
def _all_creatures_are_tokens(m, raw):
    return Static(modification=Modification(kind="all_creatures_are_tokens"),
                  raw=raw)


# "equipped creature can't attack, block, or transform, and its activated
# abilities can't be activated" — Avacyn's Collar
@_sp(r"^equipped creature can'?t [^.]+\s*$")
def _equipped_cant(m, raw):
    return Static(modification=Modification(kind="equipped_cant"), raw=raw)


# "auras attached to permanents you control have <~armor>" — Umbra Mystic
@_sp(r"^auras? attached to (?:permanents|creatures) (?:you control )?"
     r"have ([^.]+?)\s*$")
def _auras_attached_have(m, raw):
    return Static(modification=Modification(
        kind="auras_attached_have",
        args=(m.group(1).strip(),)),
        raw=raw)


# "creatures you control can't be the targets of <filter> spells or
# abilities from <filter> sources" — Spellbane Centaur
@_sp(r"^creatures you control can'?t be the targets of [^.]+\s*$")
def _allied_untargetable(m, raw):
    return Static(modification=Modification(kind="allied_untargetable"),
                  raw=raw)


# "<color> creatures you control can't be blocked" — Deepchannel Mentor,
# Dread Charge. (already mostly handled by core, but edge cases remain)
@_sp(rf"^({_COLOR}) creatures you control can'?t be blocked(?: this turn)?"
     r"(?: except by [^.]+)?\s*$")
def _color_allies_unblockable(m, raw):
    return Static(modification=Modification(
        kind="color_allies_unblockable",
        args=(m.group(1).lower(),)),
        raw=raw)


# "<color> creatures can block only <color> creatures" variants already
# handled; add "creatures can't attack a player unless ..." — Arboria
@_sp(r"^creatures can'?t attack a player unless [^.]+\s*$")
def _arboria(m, raw):
    return Static(modification=Modification(kind="arboria_restriction"),
                  raw=raw)


# "<color> creatures can't attack|block" — Light of Day
# ("black creatures can't attack or block"), Razorjaw Oni
@_sp(rf"^({_COLOR}) creatures? can'?t (attack|block|attack or block)\s*$")
def _color_cant(m, raw):
    return Static(modification=Modification(
        kind="color_cant",
        args=(m.group(1).lower(), m.group(2).lower())),
        raw=raw)


# "<color> creatures can't attack unless their controller sacrifices ..." —
# Flooded Woodlands, Reclamation
@_sp(rf"^({_COLOR}) creatures? can'?t attack unless [^.]+\s*$")
def _color_cant_attack_unless(m, raw):
    return Static(modification=Modification(
        kind="color_cant_attack_unless",
        args=(m.group(1).lower(),)),
        raw=raw)


# "creatures enter the battlefield with an additional -1/-1 counter on it"
# — Pyrrhic Revival tail, Grumgully (each creature enters with an
# additional +1/+1 counter on it)
@_sp(r"^each (?:other )?([a-z\-~ ]+?) you control enters with an additional "
     r"[+-]\d+/[+-]\d+ counter on it\s*$")
def _enters_additional_counter(m, raw):
    return Static(modification=Modification(
        kind="enters_additional_counter",
        args=(m.group(1).strip(),)),
        raw=raw)


# "your life total can't change" — already in file; add "life total becomes X"
# static (Blessed Wind: "target player's life total becomes 20" is effect).
# "each player's life total becomes <X>" — Repay in Kind, Biorhythm,
# Sway of the Stars tail, etc.
@_sp(r"^each player'?s life total becomes [^.]+\s*$")
def _life_total_becomes_each(m, raw):
    return Static(modification=Modification(kind="life_total_becomes_global"),
                  raw=raw)


# "all creatures get -X/-0, where X is ..." — Meishin, the Mind Cage,
# Valorous Charge-variant. Already partially handled but the tail "where X
# is ..." makes the core pattern fail.
@_sp(r"^all creatures get ([+-][\dxX])/([+-][\dxX])(?: until end of turn)?"
     r",? where [^.]+\s*$")
def _all_creatures_get_where(m, raw):
    return Static(modification=Modification(
        kind="all_creatures_get_where",
        args=(m.group(1), m.group(2))),
        raw=raw)


# "all non<type> creatures get -X/-X until end of turn, where X is ..." —
# Dead of Winter, Eyeblight Massacre-variant, Olivia's Wrath
@_sp(r"^all non([a-z\- ]+?) creatures get ([+-][\dxX])/([+-][\dxX])"
     r"(?: until end of turn)?,? where [^.]+\s*$")
def _all_non_type_get_where(m, raw):
    return Static(modification=Modification(
        kind="all_non_type_get_where",
        args=(m.group(1).strip(), m.group(2), m.group(3))),
        raw=raw)


# "each <filter> creature you control gets +1/+1 for each of its colors/
# types/etc." — Knight of New Alara, Diligent Zookeeper
@_sp(r"^each (?:other )?([a-z\-~ ]+?) creature you control gets "
     r"\+(\d+)/\+(\d+) for each of its ([a-z ]+?)(?:,? to a maximum [^.]+)?"
     r"\s*$")
def _typed_plus_per_self_attr(m, raw):
    return Static(modification=Modification(
        kind="typed_plus_per_self_attr",
        args=(m.group(1).strip(), int(m.group(2)), int(m.group(3)),
              m.group(4).strip())),
        raw=raw)


# "each creature gets +1/+1 for each other creature on the battlefield
# that shares ..." — Coat of Arms
@_sp(r"^each creature gets [+-]\d+/[+-]\d+ for each [^.]+\s*$")
def _each_creature_per_thing(m, raw):
    return Static(modification=Modification(kind="each_creature_per_thing"),
                  raw=raw)


# "each spell costs {N} more to cast except during its controller's turn"
# — Defense Grid
@_sp(r"^each spell costs \{(\d+)\} more to cast except during [^.]+\s*$")
def _spell_surtax_except(m, raw):
    return Static(modification=Modification(
        kind="spell_surtax_except",
        args=(int(m.group(1)),)),
        raw=raw)


# "each <color|type> spell costs {X} more/less to cast (for each <thing>)?" —
# Hum of the Radix ("each artifact spell costs {1} more to cast for each
# artifact its controller controls")
@_sp(r"^each ([a-z\- ]+?) spell costs? \{(\d+)\} (more|less) to cast"
     r"(?: for each [^.]+)?\s*$")
def _spell_surtax_typed(m, raw):
    return Static(modification=Modification(
        kind="spell_surtax_typed",
        args=(m.group(1).strip(), int(m.group(2)), m.group(3))),
        raw=raw)


# "activated abilities (cost {N} more to activate|can't be activated)
# unless they're mana abilities" — Suppression Field
@_sp(r"^activated abilities (?:cost \{(\d+)\} more to activate|can'?t be "
     r"activated) unless [^.]+\s*$")
def _activated_surtax(m, raw):
    return Static(modification=Modification(kind="activated_abilities_surtax"),
                  raw=raw)


# "<self> can't be blocked by <filter>" — Kraken of the Straits
@_sp(r"^creatures with power [^.]+? can'?t block (?:~|this creature)\s*$")
def _cant_block_self_threshold(m, raw):
    return Static(modification=Modification(kind="self_blocker_power_cap"),
                  raw=raw)


# "<type>, <type>, ..., and ~s you control can't be blocked except by
# <same list>" — Serpent of Yawning Depths
@_sp(r"^([a-z,\-~ ]+?) (?:you control )?can'?t be blocked except by "
     r"[^.]+\s*$")
def _typed_unblockable_except(m, raw):
    return Static(modification=Modification(
        kind="typed_unblockable_except",
        args=(m.group(1).strip(),)),
        raw=raw)


# "<x>s can't be blocked except by <y>s" (Slivers can't be blocked except by
# Slivers) — Shifting Sliver variant
@_sp(r"^([a-z\-~ ]+?)s? can'?t be blocked except by ([a-z\-~ ]+?)s?\s*$")
def _tribe_unblockable_except_tribe(m, raw):
    return Static(modification=Modification(
        kind="tribe_unblockable_except_tribe",
        args=(m.group(1).strip(), m.group(2).strip())),
        raw=raw)


# "<tribe> you control and <tribe> you control get +N/+N (and have <kw>)?"
# — Caterwauling Boggart "goblins you control and elementals you control have
# menace"; Death Baron "skeletons you control and other zombies you control get
# +1/+1 and have ~touch"
@_sp(r"^([a-z][a-z\- ~]+?) you control and (?:other )?([a-z][a-z\- ~]+?) you "
     r"control (?:get \+(\d+)/\+(\d+)(?: and have ([^.]+?))?|have ([^.]+?))\s*$")
def _dual_tribal_anthem(m, raw):
    return Static(modification=Modification(
        kind="dual_tribal_anthem",
        args=(m.group(1).strip(), m.group(2).strip(),
              raw)),
        raw=raw)


# ---------------------------------------------------------------------------
# Tap-state / ETB-state statics
# ---------------------------------------------------------------------------

# "artifacts, creatures, and lands your opponents control enter tapped"
# Loxodon Gatekeeper, Kismet, Frozen Aether
@_sp(r"^(?:artifacts?|creatures?|lands?|enchantments?|"
     r"[a-z,\- ]+?) (?:your )?(?:opponents? control|played by your opponents) "
     r"enter tapped\s*$")
def _opp_enter_tapped(m, raw):
    return Static(modification=Modification(kind="opp_enter_tapped"), raw=raw)


# "artifacts and lands enter tapped" / "permanents enter tapped" (global)
# Root Maze, Orb of Dreams
@_sp(r"^(?:artifacts?|creatures?|lands?|enchantments?|permanents?|"
     r"[a-z, ]+?) enter tapped\s*$")
def _global_enter_tapped(m, raw):
    return Static(modification=Modification(kind="global_enter_tapped"), raw=raw)


# ---------------------------------------------------------------------------
# Self-static P/T / abilities
# ---------------------------------------------------------------------------

# "~ can't attack unless defending player controls a(n) <filter>" —
# Zhou Yu, Chief Commander-style
@_sp(r"^(?:~|this creature) can'?t attack unless (?:defending player controls|"
     r"[^.]+)\s*$")
def _self_cant_attack_unless(m, raw):
    return Static(modification=Modification(kind="self_cant_attack_unless"),
                  raw=raw)


# "~ can't be blocked except by <filter>" (Nick Valentine: "except by artifact
# creatures"; Huang Zhong: "by more than one creature")
@_sp(r"^(?:~|this creature) can'?t be blocked (?:except by|by more than) "
     r"[^.]+\s*$")
def _self_unblockable_except(m, raw):
    return Static(modification=Modification(kind="self_unblockable_except"),
                  raw=raw)


# "you can't cast creature spells." — Grid Monitor, Steel Golem
@_sp(r"^you can'?t cast creature spells\s*$")
def _no_creature_spells(m, raw):
    return Static(modification=Modification(kind="cant_cast_creatures"), raw=raw)


# "your life total can't change" — Platinum Emperion
@_sp(r"^your life total can'?t change\s*$")
def _life_cant_change(m, raw):
    return Static(modification=Modification(kind="life_cant_change"), raw=raw)


# "players can't get counters" — Solemnity
@_sp(r"^players can'?t get counters\s*$")
def _no_counters(m, raw):
    return Static(modification=Modification(kind="no_counters"), raw=raw)


# "this creature can't have counters put on it" — Melira's Keepers
@_sp(r"^(?:~|this creature) can'?t have counters put on it\s*$")
def _self_no_counters(m, raw):
    return Static(modification=Modification(kind="self_no_counters"), raw=raw)


# ---------------------------------------------------------------------------
# Combat restrictions / shapes
# ---------------------------------------------------------------------------

# "creatures can't block." — Bedlam / "beasts can't block." Frenetic Raptor
@_sp(r"^([a-z\- ]*?) ?can'?t (attack|block)\.?\s*$")
def _creatures_cant(m, raw):
    filt = (m.group(1) or "creatures").strip() or "creatures"
    return Static(modification=Modification(
        kind="filter_cant_combat",
        args=(filt, m.group(2).lower())),
        raw=raw)


# "<filter> creatures can't attack or block" — Light of Day style
@_sp(r"^([a-z][a-z\- ]+?) creatures? can'?t (attack|block|attack or block)\s*$")
def _filter_creatures_cant(m, raw):
    return Static(modification=Modification(
        kind="filter_creatures_cant",
        args=(m.group(1).strip(), m.group(2).lower())),
        raw=raw)


# "<color> creatures and <color> creatures can't block" — Flash of Defiance /
# Magistrate's Veto
@_sp(rf"^({_COLOR}) creatures? and ({_COLOR}) creatures? can'?t "
     r"(attack|block)(?: this turn)?\s*$")
def _two_color_cant(m, raw):
    return Static(modification=Modification(
        kind="two_color_cant",
        args=(m.group(1).lower(), m.group(2).lower(), m.group(3).lower())),
        raw=raw)


# "creatures can't attack you or planeswalkers you control unless <cond>" —
# Sphere of Safety, Propaganda, Ghostly Prison, Norn's Annex
@_sp(r"^creatures can'?t attack you(?: or planeswalkers you control)? "
     r"unless [^.]+\s*$")
def _propaganda(m, raw):
    return Static(modification=Modification(kind="propaganda_tax"), raw=raw)


# "creatures with flying can block only creatures with flying" — Dense Canopy,
# Chaosphere; "creatures with flying can't block creatures you control"  —
# Bower Passage
@_sp(r"^creatures with ([a-z]+?) can block only creatures with [a-z]+\s*$")
def _flying_blocks_flying(m, raw):
    return Static(modification=Modification(
        kind="only_kw_blocks_kw",
        args=(m.group(1).strip(),)),
        raw=raw)


@_sp(r"^creatures with ([a-z]+?) can'?t block creatures you control\s*$")
def _kw_cant_block_you(m, raw):
    return Static(modification=Modification(
        kind="kw_cant_block_you",
        args=(m.group(1).strip(),)),
        raw=raw)


# "all creatures attack each combat if able" / "all creatures block each combat
# if able" — Grand Melee, Invasion Plans
@_sp(r"^all creatures (attack|block) each combat if able\s*$")
def _all_must_combat(m, raw):
    return Static(modification=Modification(
        kind="all_must_combat",
        args=(m.group(1).lower(),)),
        raw=raw)


# "creatures can't be the targets of spells." — Dense Foliage
@_sp(r"^creatures can'?t be the targets of spells(?: or abilities)?\s*$")
def _creatures_untargetable(m, raw):
    return Static(modification=Modification(kind="creatures_untargetable"),
                  raw=raw)


# ---------------------------------------------------------------------------
# Spell-cost / grants
# ---------------------------------------------------------------------------

# "<color> spells you cast cost {X} more to cast" — Jade Leech / Derelor /
# Alabaster Leech
@_sp(rf"^({_COLOR}) spells you cast cost \{{(\d+|[wubrg])\}} more to cast\s*$")
def _color_spell_surtax(m, raw):
    return Static(modification=Modification(
        kind="color_spell_surtax",
        args=(m.group(1).lower(), m.group(2).lower())),
        raw=raw)


# "<color> spells you cast cost {X} less to cast" / "spells you cast that are
# <color> or <color> cost {1} less" — Goblin Anarchomancer
@_sp(r"^each spell you cast that'?s ([a-z ]+?) cost[s]? \{(\d+)\} less "
     r"to cast\s*$")
def _mc_spell_discount(m, raw):
    return Static(modification=Modification(
        kind="multicolor_spell_discount",
        args=(m.group(1).strip(), int(m.group(2)))),
        raw=raw)


# "<kw> costs you pay cost {N} less" — Catalyst Stone / Memory Crystal
@_sp(r"^([a-z]+?) costs? you pay cost \{(\d+)\} less\s*$")
def _kw_cost_less(m, raw):
    return Static(modification=Modification(
        kind="kw_cost_less",
        args=(m.group(1).strip(), int(m.group(2)))),
        raw=raw)


# "all <kw> costs cost {N} more" — Exiled Doomsayer
@_sp(r"^all ([a-z]+?) costs? cost \{(\d+)\} more\s*$")
def _all_kw_cost_more(m, raw):
    return Static(modification=Modification(
        kind="all_kw_cost_more",
        args=(m.group(1).strip(), int(m.group(2)))),
        raw=raw)


# "spells you cast have <kw>" / "<spell type> spells you cast have <kw>" —
# Raiding Schemes, Thrumming Stone, Creature spells you cast have demonstrate
@_sp(r"^(?:each )?([a-z\- ]*?)?\s*spells? you cast have ([^.]+?)\s*$")
def _spells_you_cast_have(m, raw):
    kind = (m.group(1) or "").strip() or "any"
    return Static(modification=Modification(
        kind="spells_you_cast_have",
        args=(kind, m.group(2).strip())),
        raw=raw)


# ---------------------------------------------------------------------------
# Attach / bare keywords / misc
# ---------------------------------------------------------------------------

# "mountainwalk" / "forestwalk" / "plainswalk" / "islandwalk" / "swampwalk"
# also: "nonbasic landwalk", "legendary landwalk"
@_sp(r"^(mountain|forest|plain|island|swamp|nonbasic land|legendary land|"
     r"snow-covered mountain|snow-covered island|snow-covered forest|"
     r"snow-covered plains|snow-covered swamp)walk\s*$")
def _landwalk_bare(m, raw):
    return Keyword(name=f"{m.group(1).lower()}walk", raw=raw)


# "suspend N-<cost>" — bare header (Living End, Resurgent Belief, Durkwood Baloth)
# Durkwood Baloth already works via partial_scrubber but some printings lose the
# body text making it truly bare.
@_sp(r"^suspend (\d+|x)[\-\s—]+([^.]+?)\s*$")
def _suspend_bare(m, raw):
    return Keyword(name="suspend", args=(m.group(1).lower(), m.group(2).strip()),
                   raw=raw)


# Bare "bolster N" / "bolster N, then ..." — Cached Defenses, Dromoka's Gift
@_sp(r"^bolster (\d+|x)\s*$")
def _bolster_bare(m, raw):
    return Keyword(name="bolster", args=(_num(m.group(1)),), raw=raw)


# Bare "populate." — Wake the Reflections
@_sp(r"^populate\s*$")
def _populate_bare(m, raw):
    return Keyword(name="populate", raw=raw)


# Bare ability-word activation like "earthbend N" — Earthbending Lesson
@_sp(r"^(earthbend|waterbend|airbend|firebend|amass|incubate|manifest dread|"
     r"surveil|connive|investigate|fateseal|cloak|seek|detain|goad|conjure|"
     r"proliferate|descend) (\d+|x)\s*$")
def _ability_word_bare(m, raw):
    return Keyword(name=m.group(1).lower(), args=(m.group(2).lower(),), raw=raw)


# Bare "mayhem {cost}" / "web-slinging {cost}" / "sneak {cost}" /
# "spaceship N" / other bare-keyword-with-cost patterns
@_sp(r"^(mayhem|web-slinging|sneak|spaceship|positioning|soulshift)"
     r"\s*(?:\{[^}]+\}|\d+|[^\s].*?)?\s*(?:,.*)?\s*$")
def _misc_kw_with_cost(m, raw):
    return Keyword(name=m.group(1).lower(), raw=raw)


# "double b̶r̶e̶a̶k̶e̶r̶ strike" — weird strike-through printing
@_sp(r"^double [^\s]+ strike\s*$")
def _double_breaker_strike(m, raw):
    return Keyword(name="double strike", raw=raw)


# ---------------------------------------------------------------------------
# Grant / self-modification shapes
# ---------------------------------------------------------------------------

# "your commander costs {N} less to cast for each time ..." — Myth Unbound
@_sp(r"^your commander costs \{(\d+)\} less to cast for each time [^.]+\s*$")
def _commander_cost_less(m, raw):
    return Static(modification=Modification(
        kind="commander_cost_less",
        args=(int(m.group(1)),)),
        raw=raw)


# "creatures you control with <property> can't be blocked" — Herald of Secret
# Streams / Tetsuko Umezawa; generic unblockable rider.
@_sp(r"^creatures you control with ([^.]+?) can'?t be blocked\s*$")
def _cond_unblockable(m, raw):
    return Static(modification=Modification(
        kind="conditional_unblockable",
        args=(m.group(1).strip(),)),
        raw=raw)


# "each creature you control can block an additional creature each combat"
# (High Ground) / "each creature you control can't be blocked by more than one
# creature" (Familiar Ground)
@_sp(r"^each creature you control can (block an additional creature each combat|"
     r"'?t be blocked by more than one creature)\s*$")
def _each_you_control_blocker_mod(m, raw):
    return Static(modification=Modification(
        kind="blocker_count_mod",
        args=(m.group(1).strip(),)),
        raw=raw)


# "permanents you control gain hexproof and indestructible until end of turn" —
# Heroic Intervention-style (body effect of a sorcery)
@_sp(r"^permanents you control gain ([^.]+?)(?: until end of turn)?\s*$")
def _permanents_gain(m, raw):
    return Static(modification=Modification(
        kind="permanents_gain_kw",
        args=(m.group(1).strip(),)),
        raw=raw)


# "during <phase>, <anthem>" — generic conditional anthem
# Oak Street Innkeeper: "during turns other than yours, tapped creatures you
# control have hexproof"
@_sp(r"^during (?:your turn|turns other than yours|each player's turn|"
     r"combat|each (?:other )?player's upkeep), [^.]+\s*$")
def _during_phase_static(m, raw):
    return Static(modification=Modification(
        kind="phase_scoped_static",
        args=(raw,)),
        raw=raw)


# "<self>'s power is equal to ..." — Sima Yi-style (base parser has a similar
# rule but misses some variants)
@_sp(r"^~'s? (?:power|toughness|power and toughness)[^.]*equal to [^.]+\s*$")
def _self_calc_pt(m, raw):
    return Static(modification=Modification(kind="self_calculated_pt",
                                            args=(raw,)),
                  raw=raw)


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
# Bounce extensions
# ---------------------------------------------------------------------------

# "return all <filter> you control to their/its owner's hand" — Retract,
# Part the Veil, "return all creatures you control to their owners' hands"
@_er(r"^return all ([^.]+?) to (?:their|its|your|the) owners?'?s? hands?(?:\.|$)")
def _return_all_to_owners_hand(m):
    return Bounce(target=Filter(base=m.group(1).strip(), targeted=False))


# "return <N> target <filter> to their owners' hands" / "return <N> target
# nonland permanents to their owners' hands" — Distorting Wake, River's Rebuke,
# Aether Gale
@_er(r"^return (x|one|two|three|four|five|six|seven|\d+) target "
     r"([^.]+?) to (?:their|its) owners?'? hands?(?:\.|$)")
def _return_n_target(m):
    return Bounce(target=Filter(base=m.group(2).strip(), targeted=True,
                                count=_num(m.group(1))))


# "return up to N target <filter> to their/their owners' hands (, where X ...)?"
@_er(r"^return up to (x|one|two|three|four|five|\d+) target ([^.]+?) to "
     r"(?:their|its) owners?'? hands?(?:,? where [^.]+)?(?:\.|$)")
def _return_up_to_n_target(m):
    return Bounce(target=Filter(base=m.group(2).strip(), targeted=True,
                                count=_num(m.group(1)),
                                quantifier="up_to_n"))


# "return to their owners' hands all <filter>" (Engulf the Shore-class inversion)
@_er(r"^return to their owners'? hands all ([^.]+?)(?:\.|$)")
def _return_to_hands_all(m):
    return Bounce(target=Filter(base=m.group(1).strip(), targeted=False))


# "return each <filter> to its owner's hand" / "return each creature that dealt
# damage this turn to its owner's hand" — Restore the Peace, Curfew ("each
# player returns a creature...")
@_er(r"^return each ([^.]+?) to its owner'?s hand(?:\.|$)")
def _return_each_to_hand(m):
    return Bounce(target=Filter(base=m.group(1).strip(), targeted=False))


# "return all attacking creatures to their owner's hand" — Aetherize
@_er(r"^return all ([^.]+?) to (?:their|its) owner'?s hands?(?:\.|$)")
def _return_all_to_owner(m):
    return Bounce(target=Filter(base=m.group(1).strip(), targeted=False))


# ---------------------------------------------------------------------------
# Graveyard recursion
# ---------------------------------------------------------------------------

# "return N target creature cards from your graveyard to your hand" —
# Death's Duet, Cathartic Operation opening clause, Grim Captain's Call
@_er(r"^return (up to )?(one|two|three|four|five|six|\d+|x) target "
     r"([^.]+?) cards? from your graveyard to your hand(?:\.|$)")
def _return_from_gy(m):
    return Recurse(query=Filter(base=m.group(3).strip(), targeted=True,
                                count=_num(m.group(2))),
                   destination="hand")


# "return up to N target <filter> cards from your graveyard to the battlefield"
# — Back in Town, Raise the Past, Patch Up (with total mana value)
@_er(r"^return (up to )?(one|two|three|four|five|six|\d+|x) target "
     r"([^.]+?) cards? from your graveyard to the battlefield"
     r"(?: tapped)?(?:\.|$)")
def _return_from_gy_to_bf(m):
    return Recurse(query=Filter(base=m.group(3).strip(), targeted=True,
                                count=_num(m.group(2))),
                   destination="battlefield")


# "return all <filter> cards from your graveyard to the battlefield (tapped)?"
# — Replenish, Primevals' Glorious Rebirth, Triumphant Reckoning, Splendid
# Reclamation, Brilliant Restoration, Open the Vaults
@_er(r"^return all ([^.]+?) cards? from (?:your|all) graveyards? "
     r"to the battlefield(?: tapped)?(?: under (?:their|your) "
     r"owners?'? control)?(?:\.|$)")
def _return_all_from_gy(m):
    return Recurse(query=Filter(base=m.group(1).strip(), targeted=False),
                   destination="battlefield")


# "return all <filter> cards from your graveyard to your hand" / "return
# all <filter> from your graveyard to your hand" — Aphetto Dredging-class
@_er(r"^return all ([^.]+?) (?:cards? )?from your graveyard to your hand(?:\.|$)")
def _return_all_from_gy_hand(m):
    return Recurse(query=Filter(base=m.group(1).strip(), targeted=False),
                   destination="hand")


# "return up to N target <filter> cards of the creature type of your choice
# from your graveyard to your hand" — Aphetto Dredging, Sacred Excavation
@_er(r"^return up to (one|two|three|four|five|\d+|x) target ([^.]+?) "
     r"from your graveyard to your hand(?:\.|$)")
def _return_up_to_n_gy_hand(m):
    return Recurse(query=Filter(base=m.group(2).strip(), targeted=True,
                                count=_num(m.group(1))),
                   destination="hand")


# "return each <filter> card from your graveyard to the battlefield" —
# Retether
@_er(r"^return each ([^.]+?) from your graveyard to the battlefield"
     r"(?:\.|$)")
def _return_each_gy_bf(m):
    return Recurse(query=Filter(base=m.group(1).strip(), targeted=False),
                   destination="battlefield")


# "put all creature cards from all graveyards onto the battlefield under your
# control" — Rise of the Dark Realms
@_er(r"^put all ([^.]+?) cards? from (?:all )?graveyards? onto the "
     r"battlefield(?:,? under [^.]+)?(?:\.|$)")
def _put_all_gy_to_bf(m):
    return Recurse(query=Filter(base=m.group(1).strip(), targeted=False),
                   destination="battlefield")


# "put target creature card from an opponent's graveyard onto the battlefield
# under your control" — Nurgle's Conscription, Ashen Powder
@_er(r"^put target ([^.]+?) card from (?:an )?opponent'?s graveyard onto the "
     r"battlefield(?: tapped)?(?: under your control)?(?:\.|$)")
def _steal_from_gy(m):
    return Recurse(query=Filter(base=m.group(1).strip() + "_opp_gy",
                                 targeted=True),
                   destination="battlefield")


# ---------------------------------------------------------------------------
# Search-library extensions
# ---------------------------------------------------------------------------

# "search your library for up to N <filter> cards, put them into your
# graveyard, then shuffle" — Buried Alive
@_er(r"^search your library for up to (one|two|three|four|five|\d+) "
     r"([^,.]+?),? put (?:them|it) into your graveyard,? then shuffle(?:\.|$)")
def _search_to_gy(m):
    return UnknownEffect(raw_text=f"search lib to gy {m.group(1)} "
                                  f"{m.group(2).strip()}")


# "search your library for any number of <filter>, exile them, then shuffle" —
# Selective Memory, Mana Severance
@_er(r"^search your library for any number of ([^,.]+?),? exile "
     r"(?:them|it),? then shuffle(?:\.|$)")
def _search_exile(m):
    return UnknownEffect(raw_text=f"search lib exile any {m.group(1).strip()}")


# "search your library for up to N <filter> with different names, reveal them,
# put them into your hand, then shuffle" — Three Dreams, Congregation at Dawn,
# Firemind's Foresight (partial)
@_er(r"^search your library for up to (one|two|three|four|five|\d+) "
     r"([^,.]+?)(?: with different names)?,? reveal (?:them|it),? "
     r"(?:put (?:them|it) into your hand|then shuffle[^.]*),? "
     r"then shuffle(?:[^.]*)?(?:\.|$)")
def _search_reveal_diff_names(m):
    return UnknownEffect(raw_text=f"search reveal hand {m.group(1)} "
                                  f"{m.group(2).strip()}")


# "search your library for any number of basic land cards, reveal those cards,
# then shuffle and put them on top" — Scouting Trek
@_er(r"^search your library for any number of ([^,.]+?),? reveal "
     r"(?:those cards|them|it),? then shuffle and put (?:them|it|those cards) "
     r"on top(?:\.|$)")
def _search_reveal_top(m):
    return UnknownEffect(raw_text=f"search reveal top {m.group(1).strip()}")


# "search target opponent's library for ..." —
@_er(r"^search target opponent'?s library [^.]+(?:\.|$)")
def _search_opp_lib(m):
    return UnknownEffect(raw_text="search opp lib")


# ---------------------------------------------------------------------------
# Destroy / exile shapes
# ---------------------------------------------------------------------------

# "destroy N target <filter>" / "exile N target <filter>" — Hex, Into the Core,
# Dust to Dust, Exotic Pets, Exile two target artifacts
@_er(r"^(destroy|exile) (two|three|four|five|six|seven|\d+|x) target "
     r"([^.]+?)(?:\.|$)")
def _destroy_exile_n(m):
    verb = m.group(1).lower()
    n = _num(m.group(2))
    f = Filter(base=m.group(3).strip(), targeted=True, count=n)
    if verb == "destroy":
        return Destroy(target=f)
    return Exile(target=f)


# "exile up to N target <filter>" — Decompose
@_er(r"^exile up to (one|two|three|four|five|\d+|x) target ([^.]+?)(?:\.|$)")
def _exile_up_to_n(m):
    return Exile(target=Filter(base=m.group(2).strip(), targeted=True,
                               count=_num(m.group(1)),
                               quantifier="up_to_n"))


# "for any number of opponents, destroy target nonland permanent that player
# controls" — Windgrace's Judgment
@_er(r"^for any number of opponents, destroy target ([^.]+?) that player "
     r"controls(?:\.|$)")
def _windgrace(m):
    return Destroy(target=Filter(base=m.group(1).strip() + "_per_opp",
                                 targeted=True))


# ---------------------------------------------------------------------------
# Player damage / discard / life shapes
# ---------------------------------------------------------------------------

# "target opponent discards a card at random" — Stupor, Mind Knives
@_er(r"^target opponent discards a card at random(?:\.|$)")
def _tplayer_discard_random(m):
    return Discard(count=1, target=TARGET_OPPONENT, chosen_by="random")


# "target player discards their hand (unless ...)" — Wit's End, Tyrannize
@_er(r"^target player discards their hand(?: unless [^.]+)?(?:\.|$)")
def _discard_hand(m):
    return Discard(count="all", target=TARGET_PLAYER)


# "target player reveals their hand and discards all <filter> cards" —
# Amnesia, Trapfinder's Trick
@_er(r"^target player reveals their hand and discards all ([^.]+?) cards?"
     r"(?:\.|$)")
def _reveal_hand_discard(m):
    return Discard(count="all_filter", target=TARGET_PLAYER)


# "target player discards N cards (and loses N life)" / "target player
# discards N cards, then draws N" — Davriel's Shadowfugue, Mental Agony,
# Three Tragedies
@_er(r"^target player discards (a|one|two|three|four|five|\d+|x) cards? "
     r"and loses (\d+|x) life(?:\.|$)")
def _discard_and_life_loss(m):
    return Sequence(items=(
        Discard(count=_num(m.group(1)), target=TARGET_PLAYER),
        LoseLife(amount=_num(m.group(2)), target=TARGET_PLAYER),
    ))


# "each opponent discards <N> cards" — Unnerve
@_er(r"^each opponent discards (a|one|two|three|four|\d+) cards?(?:\.|$)")
def _each_opp_discard(m):
    return Discard(count=_num(m.group(1)),
                   target=Filter(base="each_opponent", targeted=False))


# "target opponent mills N cards" / "target player mills N cards" —
# Dreadwaters, Space-Time Anomaly, Grasping Tentacles, Mind Sculpt
@_er(r"^target (?:opponent|player) mills (\d+|x|seven|eight|half[^.]*) "
     r"cards?(?:\.|$)")
def _tplayer_mill(m):
    return Mill(count=_num(m.group(1)) if m.group(1).isdigit() else m.group(1),
                target=TARGET_PLAYER)


# "target player draws N cards" (with optional tail) — Overflowing Insight,
# Stream of Life, Heroes' Reunion
@_er(r"^target player (?:draws (x|a|one|two|three|four|five|six|seven|\d+) "
     r"cards?|gains (\d+|x) life)(?:\.|$)")
def _tplayer_draw_or_gain(m):
    if m.group(1):
        return Draw(count=_num(m.group(1)), target=TARGET_PLAYER)
    from mtg_ast import GainLife
    return GainLife(amount=_num(m.group(2)), target=TARGET_PLAYER)


# "target player loses N life" / "target opponent loses N life (for each X)?"
@_er(r"^target (?:opponent|player) loses (\d+|x) life"
     r"(?: for each [^.]+)?(?:\.|$)")
def _tplayer_loses(m):
    return LoseLife(amount=_num(m.group(1)), target=TARGET_PLAYER)


# "target player gains N life" — Heroes' Reunion, Stream of Life, Natural Spring
@_er(r"^target player gains (\d+|x) life(?:[^.]+)?(?:\.|$)")
def _tplayer_gain(m):
    from mtg_ast import GainLife
    return GainLife(amount=_num(m.group(1)), target=TARGET_PLAYER)


# "you gain N life" with trailing "for each X" / Congregate style
@_er(r"^you gain (\d+|x) life(?: plus [^.]+)?(?: for each [^.]+)?(?:\.|$)")
def _you_gain_life(m):
    from mtg_ast import GainLife
    return GainLife(amount=_num(m.group(1)))


# "you draw N cards, lose N life, then <tail>" — Funeral Rites, Pointed
# Discussion, Atrocious Experiment
@_er(r"^you draw (a|one|two|three|four|five|\d+) cards?,? lose (\d+|x) life,"
     r"? (?:then )?([^.]+?)(?:\.|$)")
def _draw_lose_then(m):
    tail = m.group(3).strip()
    seq = [Draw(count=_num(m.group(1))),
           LoseLife(amount=_num(m.group(2)))]
    # We can't parse the tail cleanly here so record it as UnknownEffect.
    seq.append(UnknownEffect(raw_text=tail))
    return Sequence(items=tuple(seq))


# "draw N cards, then discard N cards (at random)?" — Goblin Lore,
# Control of the Court, Breakthrough, Burning Inquiry (prefix "you"),
# Amass the Components
@_er(r"^draw (a|one|two|three|four|five|\d+|x) cards?,? then (?:discard|"
     r"put) (a|one|two|three|four|five|\d+|x) (?:cards?|card from your hand)"
     r"(?: at random| on the bottom of your library| from your hand)?(?:\.|$)")
def _draw_then_discard(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1))),
        Discard(count=_num(m.group(2))),
    ))


# "target player gains N life and draws N cards" — Kiss of the Amesha
@_er(r"^target player gains (\d+|x) life and draws (a|one|two|three|four|"
     r"five|\d+) cards?(?:\.|$)")
def _tplayer_gain_and_draw(m):
    from mtg_ast import GainLife
    return Sequence(items=(
        GainLife(amount=_num(m.group(1)), target=TARGET_PLAYER),
        Draw(count=_num(m.group(2)), target=TARGET_PLAYER),
    ))


# "you and target opponent each draw N cards" — Secret Rendezvous
@_er(r"^you and target opponent each draw (a|one|two|three|four|\d+) "
     r"cards?(?:\.|$)")
def _both_draw(m):
    return Sequence(items=(
        Draw(count=_num(m.group(1))),
        Draw(count=_num(m.group(1)), target=TARGET_OPPONENT),
    ))


# "each player discards <N> cards" — Delirium Skeins
@_er(r"^each player discards (a|one|two|three|four|\d+) cards?(?:\.|$)")
def _each_player_discard(m):
    return Discard(count=_num(m.group(1)),
                   target=Filter(base="each_player", targeted=False))


# "each player draws N cards" / "each player draws a card for each ..."
@_er(r"^each (?:other )?player draws (a|one|two|three|four|\d+) cards?"
     r"(?: for each [^.]+)?(?:\.|$)")
def _each_player_draw(m):
    return Draw(count=_num(m.group(1)),
                target=Filter(base="each_player", targeted=False))


# ---------------------------------------------------------------------------
# Damage / destroy body effects
# ---------------------------------------------------------------------------

# "target creature gets +X/+Y and gains <kw> until end of turn" — Run Amok,
# Double Cleave ("target creature gains double strike until end of turn")
@_er(r"^target creature gains ([^.]+?) until end of turn(?:\.|$)")
def _target_gains_kw(m):
    return GrantAbility(target=TARGET_CREATURE,
                        ability_name=m.group(1).strip(),
                        duration="until_end_of_turn")


# "target creature gets +X/+Y (and ...)? until end of turn" — many
# (Aliban's Tower, Compelled Duel, Emergent Growth, Righteousness etc.)
@_er(r"^target creature gets \+(\d+|x)/\+(\d+|x)"
     r"(?:\s+(?:and [^.]+))?\s*until end of turn(?:\.|$)")
def _target_creature_pump(m):
    return Buff(target=TARGET_CREATURE,
                power=_num(m.group(1)), toughness=_num(m.group(2)),
                duration="until_end_of_turn")


# "target creature gets -X/-X until end of turn" — Stench of Decay residuals
@_er(r"^target creature gets -(\d+|x)/-(\d+|x) until end of turn(?:\.|$)")
def _target_creature_debuff(m):
    return Buff(target=TARGET_CREATURE,
                power=-_num(m.group(1)) if m.group(1).isdigit() else -1,
                toughness=-_num(m.group(2)) if m.group(2).isdigit() else -1,
                duration="until_end_of_turn")


# "target blocking creature gets +N/+N until end of turn" —
# Aliban's Tower, Furious Resistance, Yare, Righteousness
@_er(r"^target (blocking|blocked|attacking|attacking or blocking) creature "
     r"(?:you control )?gets \+(\d+)/\+(\d+)(?: and gains ([^.]+?))?"
     r" until end of turn(?:\.|$)")
def _combat_creature_pump(m):
    grant = m.group(4)
    buff = Buff(target=TARGET_CREATURE, power=int(m.group(2)),
                toughness=int(m.group(3)), duration="until_end_of_turn")
    if not grant:
        return buff
    return Sequence(items=(
        buff,
        GrantAbility(target=TARGET_CREATURE, ability_name=grant.strip(),
                     duration="until_end_of_turn"),
    ))


# "target player takes (one|two|\d+) extra turns? after this one" —
# Time Stretch, Time Warp
@_er(r"^target player takes (an|one|two|three|\d+) extra turns? "
     r"after this one(?:\.|$)")
def _extra_turns(m):
    from mtg_ast import ExtraTurn
    return ExtraTurn(target=TARGET_PLAYER)


# "target player skips their/all ... phase(s) / step(s)" — Moment of Silence,
# False Peace, Empty City Ruse
@_er(r"^target (?:player|opponent) skips (?:their next|all) "
     r"(?:combat phase|combat phases of their next turn|next draw step|"
     r"combat step|upkeep)[^.]*(?:\.|$)")
def _target_skips_phase(m):
    return UnknownEffect(raw_text="target skips phase")


# ---------------------------------------------------------------------------
# Counter-spell variants
# ---------------------------------------------------------------------------

# "counter up to N target spells (and/or abilities)?" —
# Double Negative, Katara's Reversal
@_er(r"^counter up to (one|two|three|four|\d+) target spells?"
     r"(?: and/or abilities)?(?:\.|$)")
def _counter_up_to_n(m):
    from mtg_ast import CounterSpell
    return CounterSpell(target=Filter(base="spell", targeted=True,
                                      count=_num(m.group(1)),
                                      quantifier="up_to_n"))


# ---------------------------------------------------------------------------
# Token creation shapes
# ---------------------------------------------------------------------------

# "create N 1/1 <color> <type> creature tokens (with <kw>)?" —
# Knight Watch, Carrion Call, Elven Ambush, Goblin Rally, Servo Exhibition
@_er(r"^create (a|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+) "
     r"(\d+)/(\d+) ([a-z\- ]+?) (?:creature )?tokens?"
     r"(?: with [^.]+)?(?:\.|$)")
def _create_basic_token(m):
    return CreateToken(
        count=_num(m.group(1)),
        pt=(int(m.group(2)), int(m.group(3))),
        types=(m.group(4).strip(),),
    )


# "create a <p>/<t> <color> <type> creature token" (no count)
@_er(r"^create an? (\d+)/(\d+) ([a-z\- ]+?) (?:creature )?token"
     r"(?: with [^.]+)?(?:\.|$)")
def _create_a_token(m):
    return CreateToken(count=1,
                       pt=(int(m.group(1)), int(m.group(2))),
                       types=(m.group(3).strip(),))


# "create a Food token" / "create a Treasure token" / "create a Blood token" —
# noncreature tokens (Pointed Discussion tail etc.)
@_er(r"^create an? (food|treasure|blood|clue|gold|map|powerstone|shard) "
     r"token(?:\.|$)")
def _create_named_noncreature_token(m):
    return CreateToken(count=1, types=(m.group(1).lower() + "_token",))


# ---------------------------------------------------------------------------
# Simple one-off shapes
# ---------------------------------------------------------------------------

# "switch target creature's power and toughness until end of turn" —
# Transmutation, About Face, plain switch
@_er(r"^switch target creature'?s power and toughness(?: until end of turn)?"
     r"(?:\.|$)")
def _switch_pt(m):
    return UnknownEffect(raw_text="switch power toughness")


# "tap all <filter>" — Blinding Light, Riptide, Metal Fatigue, Deluge
@_er(r"^tap all ([^.]+?)(?:\.|$)")
def _tap_all(m):
    return TapEffect(target=Filter(base=m.group(1).strip(), targeted=False))


# "untap all <filter>" — Dramatic Reversal
@_er(r"^untap all ([^.]+?)(?:\.|$)")
def _untap_all(m):
    return UntapEffect(target=Filter(base=m.group(1).strip(), targeted=False))


# "regenerate each creature you control" — Wrap in Vigor; "regenerate target
# creature" bare — Death Ward-class
@_er(r"^regenerate (target creature|each creature you control|[^.]+?)"
     r"(?:\.|$)")
def _regenerate_effect(m):
    return UnknownEffect(raw_text=f"regenerate {m.group(1).strip()}")


# "each player sacrifices a <filter> of their choice (for each <thing>)?" —
# Innocent Blood, Tectonic Break, Simplify, Thoughts of Ruin
@_er(r"^each player sacrifices (?:an? |x |a number of )?([^.]+?) of their "
     r"choice(?: for each [^.]+)?(?:\.|$)")
def _each_player_sacrifices(m):
    return Sacrifice(query=Filter(base=m.group(1).strip(), targeted=False),
                     actor="each_player")


# "target player sacrifices a <filter> of their choice (and loses N life)?" —
# Geth's Verdict, Devour Flesh, Misguided Rage, Celestial Flare, Abzan Advantage
@_er(r"^target player sacrifices (?:an? )?([^.]+?) of their choice"
     r"(?: and loses (\d+) life)?(?:\.|$)")
def _target_player_sacrifices(m):
    sac = Sacrifice(query=Filter(base=m.group(1).strip(), targeted=False),
                    actor="target_player")
    if m.group(2):
        return Sequence(items=(sac, LoseLife(amount=int(m.group(2)),
                                             target=TARGET_PLAYER)))
    return sac


# "sacrifice any number of lands(/artifacts/etc.). <tail>" — Scapeshift,
# Mana Seism, Lich-Knights' Conquest
@_er(r"^sacrifice any number of ([^.]+?)\.\s+(.*)(?:\.|$)", )
def _sac_any_then(m):
    sac = Sacrifice(query=Filter(base=m.group(1).strip(), targeted=False,
                                 quantifier="any"))
    return Sequence(items=(sac, UnknownEffect(raw_text=m.group(2).strip())))


# "sacrifice N lands/artifacts/etc. <tail>?" — Planar Engineering opener
@_er(r"^sacrifice (one|two|three|four|\d+) ([^.]+?)(?:\.|$)")
def _sac_n(m):
    return Sacrifice(query=Filter(base=m.group(2).strip(), targeted=False,
                                  count=_num(m.group(1))))


# "put all <filter> on top of their owners' libraries" — Harmonic Convergence,
# Plow Under, Hallowed Burial, Rebuking Ceremony
@_er(r"^put (all|two|\d+) ([^.]+?)(?: cards?)? on (?:top of|the bottom of) "
     r"(?:their|its) owners?'? (?:libraries|library)(?:\.|$)")
def _put_on_lib(m):
    return UnknownEffect(raw_text=f"put {m.group(1)} {m.group(2).strip()} on lib")


# "put target <filter> on (top of|the bottom of) <owner> library" — Spin into
# Myth, Unexpectedly Absent, Lost to Legend
@_er(r"^put target ([^.]+?) (?:on (?:top of|the bottom of)|into) "
     r"its owner'?s library[^.]*(?:\.|$)")
def _put_target_lib(m):
    return UnknownEffect(raw_text=f"put target {m.group(1).strip()} on lib")


# "put target face-up exiled card into its owner's graveyard" — Pull from
# Eternity
@_er(r"^put target (?:face-up |face-down )?exiled card into its owner'?s "
     r"(?:graveyard|hand)(?:\.|$)")
def _put_exiled_to_owner_zone(m):
    return UnknownEffect(raw_text="pull from exile")


# ---------------------------------------------------------------------------
# Name-specific quirks that show up repeatedly as unparsed tails
# ---------------------------------------------------------------------------

# "choose (up to)? N target <filter>. <tail>" — Duneblast, Pest Infestation,
# Wild Swing, Continue?
@_er(r"^choose (?:up to )?(one|two|three|four|\d+) target ([^.]+?)\.\s+"
     r"([^.]+?)(?:\.|$)")
def _choose_n_target_then(m):
    return UnknownEffect(raw_text=f"choose {m.group(1)} target "
                                  f"{m.group(2).strip()} then "
                                  f"{m.group(3).strip()}")


# "you may mill N cards. then <tail>" — Druidic Ritual, Another Chance,
# Summon Undead, A-Druidic Ritual
@_er(r"^you may mill (a|one|two|three|four|\d+) cards?\.\s+"
     r"then (.*)(?:\.|$)")
def _may_mill_then(m):
    mill = Mill(count=_num(m.group(1)))
    return Sequence(items=(mill, UnknownEffect(raw_text=m.group(2).strip())))


# "you may sacrifice a <filter>. if you do, <tail>" — Giant Opportunity,
# Insatiable Appetite, Pippin's Bravery
@_er(r"^you may sacrifice an? ([^.]+?)\. if you do, (.*)(?:\.|$)")
def _may_sac_then(m):
    return UnknownEffect(raw_text=f"may sac {m.group(1).strip()} then "
                                  f"{m.group(2).strip()}")


# "exchange control of target artifact or creature and another target
# permanent that shares one of those types with it" — Legerdemain
@_er(r"^exchange control of target ([^.]+?) and another target ([^.]+?)"
     r"(?:\.|$)")
def _exchange_control(m):
    return GainControl(target=TARGET_CREATURE)


# "gain control of target creature" (one-off, most variations already handled)
# "target opponent gains control of target permanent you control" —
# Harmless Offering, Wrong Turn
@_er(r"^target opponent gains control of target ([^.]+?)(?:\.|$)")
def _opp_gains_control(m):
    return GainControl(target=TARGET_OPPONENT)


# "gain control of X target creatures and/or planeswalkers" — Mass Manipulation
@_er(r"^gain control of (x|one|two|three|\d+) target ([^.]+?)(?:\.|$)")
def _gain_control_n(m):
    return GainControl(target=Filter(base=m.group(2).strip(), targeted=True,
                                     count=_num(m.group(1))))


# "gain control of each <filter>" — Subjugate the Hobbits
@_er(r"^gain control of each ([^.]+?)(?:\.|$)")
def _gain_control_each(m):
    return GainControl(target=Filter(base=m.group(1).strip(), targeted=False,
                                     quantifier="each"))


# "target creature fights another/target creature" — Pit Fight, Mutant's Prey
@_er(r"^target creature (?:you control )?fights (another|target) target? "
     r"creature(?: you don'?t control| an opponent controls)?(?:\.|$)")
def _creature_fights(m):
    from mtg_ast import Fight
    return Fight(a=TARGET_CREATURE, b=TARGET_CREATURE)


# "target creature blocks target creature this turn if able" / "target
# creature blocks this turn if able" — Hunt Down, Culling Mark
@_er(r"^target creature blocks(?: target creature)? this turn if able(?:\.|$)")
def _must_block(m):
    return UnknownEffect(raw_text="must block this turn")


# ---------------------------------------------------------------------------
# Misc
# ---------------------------------------------------------------------------

# "you may change any targets of target <spell type> spell" — Sideswipe,
# Redirect
@_er(r"^you may change (?:any targets of target|new targets for target) "
     r"[^.]+ spell\s*(?:\.|$)")
def _change_targets(m):
    return UnknownEffect(raw_text="change spell targets")


# "target permanent." bare — Indicate
@_er(r"^target permanent\.\s*$")
def _target_permanent_bare(m):
    return UnknownEffect(raw_text="target permanent")


# "look at target player's hand and choose N cards from it. put them ..." —
# Painful Memories, Agonizing Memories (first sentence only parses cleanly)
@_er(r"^look at target (?:player'?s|opponent'?s) hand(?: and choose [^.]+)?"
     r"(?:\.|$)")
def _look_at_hand(m):
    from mtg_ast import LookAt
    return LookAt(target=TARGET_PLAYER)


# "choose a <quality>" bare (already in scrubber_1 for some — this adds
# "choose target <thing>" bare that still slip through)
@_er(r"^choose target ([^.]+?)(?:\.|$)")
def _choose_target_bare(m):
    return UnknownEffect(raw_text=f"choose target {m.group(1).strip()}")


# "return all <list of types> cards from (your|all|its) graveyard(s)? to the
# battlefield (tapped)? (under <owner> control)?" — Replenish, Splendid
# Reclamation, Triumphant Reckoning, Brilliant Restoration, Second Sunrise,
# Open the Vaults, Primevals' Glorious Rebirth, Planar Birth, Faith's Reward.
# The earlier "return all <filter> cards from your graveyard" rule uses
# `[^.]+?` which can fail on multi-type lists joined with commas/"and".
# This re-tries with a more permissive body shape.
@_er(r"^return all ([a-z,\- ]+?) cards? from (?:your|all|their) graveyards? "
     r"to the battlefield(?: tapped)?(?: under (?:their|your|its|the) "
     r"owners?'? control)?(?:\.|$)")
def _return_all_gy_to_bf_v2(m):
    return Recurse(query=Filter(base=m.group(1).strip(), targeted=False),
                   destination="battlefield")


# "each player returns all <filter> cards from their graveyard to (their
# hand|the battlefield)" — Empty the Catacombs, Roar of Reclamation
@_er(r"^each player returns? all ([^.]+?) (?:cards? )?from their graveyards? "
     r"to (?:their hand|the battlefield)(?:\.|$)")
def _each_player_returns_all_gy(m):
    return UnknownEffect(raw_text=f"each player returns all "
                                  f"{m.group(1).strip()} from gy")


# "each player returns all <filter> cards from their graveyard to their hand"
# — variant already captured above; also add "each player returns to the
# battlefield all <filter>" — Second Sunrise body shape.
@_er(r"^each player returns? to the battlefield all [^.]+(?:\.|$)")
def _each_player_returns_bf_all(m):
    return UnknownEffect(raw_text="each player returns to bf all")


# "return to the battlefield all <filter> cards in your graveyard that were
# put there from the battlefield this turn" — Faith's Reward
@_er(r"^return to the battlefield all ([^.]+?) cards? in your graveyards? "
     r"that were put there from the battlefield this turn(?:\.|$)")
def _return_bf_all_put_there(m):
    return Recurse(query=Filter(base=m.group(1).strip(), targeted=False),
                   destination="battlefield")


# "put onto the battlefield under your control all <filter> cards in
# <zone>" — Thrilling Encore
@_er(r"^put onto the battlefield under your control all ([^.]+?) cards? "
     r"in (?:all )?graveyards?(?: that were put there from the battlefield "
     r"this turn)?(?:\.|$)")
def _put_bf_all_gy(m):
    return Recurse(query=Filter(base=m.group(1).strip(), targeted=False),
                   destination="battlefield")


# "each player returns all creatures they own into their library" or
# similar "shuffle all creatures" — Collision of Realms header (partial)
@_er(r"^each player shuffles? all ([^.]+?) (?:they own )?into their "
     r"library(?:\.|$)")
def _each_player_shuffles_all(m):
    return UnknownEffect(raw_text=f"each player shuffles {m.group(1).strip()}")


# "each player exiles all <filter> cards from their graveyard" — Mudhole,
# Tombfire tail, Living Death opener
@_er(r"^(?:target player|each player) exiles all ([^.]+?) cards? from "
     r"(?:their|target player'?s) graveyards?(?:\.|$)")
def _each_player_exiles_all_gy(m):
    return UnknownEffect(raw_text=f"exile all {m.group(1).strip()} from gy")


# "each opponent loses N life for each <thing>" — Gruesome Fate
@_er(r"^each opponent loses (\d+|x) life for each [^.]+(?:\.|$)")
def _each_opp_loses_per(m):
    return LoseLife(amount=_num(m.group(1)),
                    target=Filter(base="each_opponent", targeted=False))


# "each player loses N life for each <thing>" — Stronghold Discipline
@_er(r"^each player loses (\d+|x) life for each [^.]+(?:\.|$)")
def _each_player_loses_per(m):
    return LoseLife(amount=_num(m.group(1)),
                    target=Filter(base="each_player", targeted=False))


# "target player creates X <token> tokens" / "create X <token> tokens" —
# Dark Salvation (for target player)
@_er(r"^target player creates (x|\d+) [^.]+ creature tokens?"
     r"(?:,? then [^.]+)?(?:\.|$)")
def _target_player_creates(m):
    return CreateToken(count=_num(m.group(1)))


# "create X 1/1 <color> <type> creature tokens" with count variable —
# Goblin Offensive, Dark Salvation opener, Knight Watch variant
@_er(r"^create (x|a number of|twice x|\d+) (\d+)/(\d+) ([a-z\- ]+?) "
     r"(?:creature )?tokens?(?: with [^.]+)?"
     r"(?: equal to [^.]+)?(?:\.|$)")
def _create_var_token(m):
    c = m.group(1).lower()
    count = _num(c) if c.isdigit() else c
    return CreateToken(
        count=count,
        pt=(int(m.group(2)), int(m.group(3))),
        types=(m.group(4).strip(),),
    )


# "create X <N/M> <color> <type> tokens with <kw>" — fallback generic
# capture (Song of Totentanz, Elemental Summoning, Fractal Anomaly opener)
@_er(r"^create (a|an|\d+|x|two|three|four|five|six|seven|eight|nine|ten) "
     r"(\d+|x)/(\d+|x) ([a-z\-, ]+?) creature tokens?"
     r"(?: with [^.]*)?(?:, where [^.]+)?(?:\.|$)")
def _create_generic_token(m):
    p_v = _num(m.group(2)) if m.group(2).isdigit() else 0
    t_v = _num(m.group(3)) if m.group(3).isdigit() else 0
    return CreateToken(
        count=_num(m.group(1)),
        pt=(p_v, t_v),
        types=(m.group(4).strip(),),
    )


# "create a tapped <N/M> <color> <type> token for each <thing>" — Crash the
# Party, Rise from the Tides
@_er(r"^create an? (?:tapped )?(\d+)/(\d+) ([a-z\- ]+?) creature token "
     r"for each [^.]+(?:\.|$)")
def _create_token_per(m):
    return CreateToken(count="per",
                       pt=(int(m.group(1)), int(m.group(2))),
                       types=(m.group(3).strip(),))


# "each player creates a <N/M> <color> <type> creature token for each <thing>"
# — Waiting in the Weeds, Death by Dragons
@_er(r"^each (?:player|other player|opponent) creates? an? (\d+)/(\d+) "
     r"([a-z\- ]+?) creature token(?: with [^.]+)?(?: for each [^.]+)?(?:\.|$)")
def _each_player_creates_token(m):
    return CreateToken(count=1, pt=(int(m.group(1)), int(m.group(2))),
                       types=(m.group(3).strip(),))


# "create a number of <tokens> equal to <X> with <kw>" — Tyranid Invasion
@_er(r"^create a number of (\d+)/(\d+) ([a-z\- ]+?) (?:creature )?tokens?"
     r"(?: with [^.]+?)? equal to (?:the number of )?[^.]+(?:\.|$)")
def _create_token_equal_to(m):
    return CreateToken(count="equal_to",
                       pt=(int(m.group(1)), int(m.group(2))),
                       types=(m.group(3).strip(),))


# "destroy up to <N> target <filter>" — Pest Infestation
@_er(r"^destroy up to (x|one|two|three|four|five|\d+) target ([^.]+?)"
     r"(?:\.|$)")
def _destroy_up_to(m):
    return Destroy(target=Filter(base=m.group(2).strip(), targeted=True,
                                 count=_num(m.group(1)),
                                 quantifier="up_to_n"))


# "exile up to <N> target <filter> from <zone>" — Decompose (graveyard)
@_er(r"^exile up to (one|two|three|four|five|\d+|x) target ([^.]+?) from "
     r"(?:a single graveyard|your graveyard|any graveyard)(?:\.|$)")
def _exile_up_to_from_gy(m):
    return Exile(target=Filter(base=m.group(2).strip(), targeted=True,
                               count=_num(m.group(1)),
                               quantifier="up_to_n"))


# "untap <N> target creatures. each of them gets +1/+1 until end of turn" —
# Hope and Glory, Part Water — first sentence only.
@_er(r"^untap (two|three|\d+|x) target ([^.]+?)(?:\.|$)")
def _untap_n_target(m):
    return UntapEffect(target=Filter(base=m.group(2).strip(), targeted=True,
                                     count=_num(m.group(1))))


# "target player untaps all <filter>" — Early Harvest
@_er(r"^target player untaps all ([^.]+?)(?: they control)?(?:\.|$)")
def _tplayer_untaps_all(m):
    return UntapEffect(target=Filter(base=m.group(1).strip(), targeted=False,
                                     quantifier="all"))


# "search target opponent's graveyard, hand, and library for ..." — Necromentia
@_er(r"^(?:choose a card name [^.]+\.\s+)?search target opponent'?s "
     r"graveyard(?:,? hand,? (?:and|or) library)? for [^.]+(?:\.|$)")
def _necromentia(m):
    return UnknownEffect(raw_text="necromentia-search")


# "target opponent reveals cards from the top of their library until ..." —
# Telemin Performance, Mind Funeral
@_er(r"^target opponent reveals cards from the top of their library "
     r"until [^.]+(?:\.|$)")
def _reveal_until(m):
    return UnknownEffect(raw_text="reveal until condition")


# "reveal cards from the top of your library until ..." — Fathom Trawl
@_er(r"^reveal cards from the top of your library until [^.]+(?:\.|$)")
def _you_reveal_until(m):
    return UnknownEffect(raw_text="you reveal until condition")


# "unless any player pays {X}, <effect>" — Rhystic Tutor opener
@_er(r"^unless any player pays \{[^}]+\}, (.+?)(?:\.|$)")
def _unless_any_pays(m):
    return UnknownEffect(raw_text=f"unless-any-pays {m.group(1).strip()}")


# "reveal the first card you draw each turn" — Rowen, Primitive Etchings
# opener, reveal ability. (static-ish but we treat as effect for spell-card
# attachment too)
@_er(r"^reveal the first card you draw each turn(?:\.|$)")
def _reveal_first_card(m):
    return UnknownEffect(raw_text="reveal first draw each turn")


# "double the power of <filter> until end of turn" — Double Trouble, Unleash
# Fury
@_er(r"^(?:~ )?(?:double|triple) the power of (?:each creature you control|"
     r"target creature)(?: until end of turn)?(?:\.|$)")
def _double_power(m):
    return UnknownEffect(raw_text="double power")


# "target creature's base power perpetually becomes 0" — Baffling Defenses
@_er(r"^target creature'?s base (?:power|toughness|power and toughness) "
     r"perpetually becomes [^.]+(?:\.|$)")
def _perpetual_base_pt(m):
    return UnknownEffect(raw_text="perpetual base pt")


# "target creature perpetually loses all abilities, then ..." — Patriar's
# Humiliation
@_er(r"^target creature perpetually loses all abilities(?:[,.] [^.]+)?"
     r"(?:\.|$)")
def _perpetual_lose_abilities(m):
    return UnknownEffect(raw_text="perpetual lose abilities")


# "prevent all damage that this creature would deal to <filter>" — Goblin
# Furrier, Indentured Oaf
@_er(r"^prevent all damage that (?:~|this creature) would deal to "
     r"([^.]+?)(?:\.|$)")
def _prevent_self_damage_to(m):
    return Prevent(damage_filter=Filter(base=m.group(1).strip(), targeted=False))


# "during your turn, prevent all damage that would be dealt to you" —
# Personal Sanctuary (already covered by existing static?). Add explicit.
@_er(r"^prevent all damage that would be dealt to you(?:\.|$)")
def _prevent_all_to_you(m):
    return Prevent()


# "amass <type> N (, then <tail>)?" — Amass Zombies 2 then ..., Assault on
# Osgiliath, Widespread Brutality, Invade the City, Surrounded by Orcs
@_er(r"^amass ([a-z]+) (\d+|x)(?:,? then [^.]+)?(?:\.|$)")
def _amass(m):
    from mtg_ast import Keyword
    return UnknownEffect(raw_text=f"amass {m.group(1)} {m.group(2)}")


# "incubate X twice, where X is ..." — Glistening Dawn
@_er(r"^incubate (\d+|x)(?: twice)?(?:,? where [^.]+)?(?:\.|$)")
def _incubate(m):
    return UnknownEffect(raw_text=f"incubate {m.group(1)}")


# "seek <N> <filter> cards, then <tail>" — Seek New Knowledge, Tasteful
# Offering partial
@_er(r"^seek (a|two|three|four|\d+|x) ([^.,]+?) cards?"
     r"(?:,? then [^.]+)?(?:\.|$)")
def _seek_n(m):
    return UnknownEffect(raw_text=f"seek {m.group(1)} {m.group(2).strip()}")


# "exile target creature. if you don't control a <filter>, <tail>" —
# Dire Tactics
@_er(r"^exile target creature\. if you don'?t control an? [^.]+,? "
     r"(?:you lose|that player) [^.]+(?:\.|$)")
def _exile_target_if_no(m):
    return Exile(target=TARGET_CREATURE)


# "destroy target noncreature permanent. then <tail>" — Chain of Acid
@_er(r"^destroy target ([^.]+?)\.\s+then [^.]+(?:\.|$)")
def _destroy_target_then(m):
    return Destroy(target=Filter(base=m.group(1).strip(), targeted=True))


# "turn target face-down creature an opponent controls face up" — Break Open
@_er(r"^turn target face-down creature (?:an opponent controls|you control) "
     r"face up(?:\.|$)")
def _turn_face_up(m):
    return UnknownEffect(raw_text="turn face up")


# "turn target creature face down. it's a 2/2 ..." — Cyber Conversion
@_er(r"^turn target creature face down(?:\.|$).*$")
def _turn_face_down(m):
    return UnknownEffect(raw_text="turn face down")


# "bolster N, then <tail>" — Scale Blessing, Abzan Advantage, etc.
@_er(r"^bolster (\d+|x)(?:,? then [^.]+)?(?:\.|$)")
def _bolster_effect(m):
    return UnknownEffect(raw_text=f"bolster {m.group(1)}")


# "detain up to N target creatures your opponents control" — Lyev Decree
@_er(r"^detain up to (one|two|three|four|\d+) target ([^.]+?)(?:\.|$)")
def _detain_n(m):
    return UnknownEffect(raw_text=f"detain {m.group(1)} {m.group(2).strip()}")


# "move any number of +1/+1 counters from <source> onto <target>" — Bioshift,
# Fate Transfer
@_er(r"^move (?:any number of|all) \+?\d+/\+?\d+ counters from target creature"
     r" onto (?:another target|target) creature(?: with the same controller)?"
     r"(?:\.|$)")
def _move_counters(m):
    return UnknownEffect(raw_text="move counters")


# "move all counters from target creature onto another target creature" —
# Fate Transfer
@_er(r"^move all counters from target creature onto another target "
     r"creature(?:\.|$)")
def _move_all_counters(m):
    return UnknownEffect(raw_text="move all counters")


# "remove all counters from all permanents (and ...)?" — Aether Snap
@_er(r"^remove all counters from all permanents(?:[^.]*)?(?:\.|$)")
def _remove_all_counters(m):
    return UnknownEffect(raw_text="remove all counters")


# "remove up to N counters from <target>" — Price of Betrayal
@_er(r"^remove up to (one|two|three|four|five|\d+) counters from target "
     r"([^.]+?)(?:\.|$)")
def _remove_up_to_counters(m):
    return UnknownEffect(raw_text=f"remove up to {m.group(1)} counters")


# "each player exiles all <filter> cards from their graveyard, then ..." —
# Scrap Mastery
@_er(r"^each player exiles all ([^.]+?) cards? from their graveyards?"
     r"(?:,? [^.]+)?(?:\.|$)")
def _each_player_exiles_all(m):
    return UnknownEffect(raw_text=f"each player exiles {m.group(1).strip()}")


# "tap one or two target creatures without <kw>" — Broken Dam
@_er(r"^tap (one or two|up to (?:one|two|three|\d+)|two|three|\d+) target "
     r"([^.]+?)(?:\.|$)")
def _tap_n_target(m):
    c = 2 if m.group(1).startswith("one or") else _num(m.group(1).split()[-1])
    return TapEffect(target=Filter(base=m.group(2).strip(), targeted=True,
                                   count=c))


# "regenerate each creature you control" / "regenerate target creature"
# already covered, add bare "~ target creature" (Regenerate card has its name
# replaced with ~: "~ target creature")
@_er(r"^~ target creature(?:\.|$)")
def _regenerate_self_replace(m):
    return UnknownEffect(raw_text="regenerate target creature")


# "unattach all equipment from target creature" — Disarm
@_er(r"^unattach all equipment from target creature(?:\.|$)")
def _unattach_all(m):
    return UnknownEffect(raw_text="unattach all")


# "attach target equipment to target creature" — Magnetic Theft
@_er(r"^attach target ([^.]+?) to target ([^.]+?)(?:\.|$)")
def _attach_target(m):
    return UnknownEffect(raw_text=f"attach {m.group(1).strip()} to "
                                  f"{m.group(2).strip()}")


# "you may cast a card you own from outside the game this turn" — Wish
@_er(r"^you may (?:play|cast) a card you own from outside the game "
     r"this turn(?:\.|$)")
def _wish(m):
    return UnknownEffect(raw_text="wish")


# "you may play up to <N> additional lands this turn" — Summer Bloom
@_er(r"^you may play up to (one|two|three|four|five|\d+) additional lands? "
     r"this turn(?:\.|$)")
def _extra_lands_this_turn(m):
    return UnknownEffect(raw_text=f"extra lands {m.group(1)}")


# "target player discards a card for each <X>" — Mind Sludge
@_er(r"^target player discards a card for each [^.]+(?:\.|$)")
def _tplayer_discard_per(m):
    return Discard(count="per", target=TARGET_PLAYER)


# "target player mills cards equal to <X>" — Space-Time Anomaly
@_er(r"^target player mills cards equal to [^.]+(?:\.|$)")
def _tplayer_mill_equal(m):
    return Mill(count="equal_to", target=TARGET_PLAYER)


# "put target face-up exiled card into its owner's graveyard/hand" — already
# covered; add "put all <filter> on top of their owners' libraries" —
# Hallowed Burial, Plow Under
# (generalized version of earlier put-on-lib pattern)
@_er(r"^put all ([^.]+?) on the bottom of their owners?'? libraries?"
     r"(?:\.|$)")
def _put_all_bottom_lib(m):
    return UnknownEffect(raw_text=f"put all {m.group(1).strip()} bottom")


# "put any number of target <filter> cards from target player's graveyard
# on top of their library in any order" — Drafna's Restoration
@_er(r"^put any number of target ([^.]+?) cards? from target player'?s "
     r"graveyards? on top of their libraries?[^.]*(?:\.|$)")
def _drafna_restore(m):
    return UnknownEffect(raw_text=f"put any {m.group(1).strip()} on top lib")


# "counter target activated ability from an artifact source" — Rust
@_er(r"^counter target activated ability(?: from [^.]+)?(?:\.|$)")
def _counter_activated(m):
    from mtg_ast import CounterSpell
    return CounterSpell(target=Filter(base="activated_ability", targeted=True))


# "counter target <filter> with mana value <op> <N>" — Spell Snare
@_er(r"^counter target [^.]+? with mana value [^.]+(?:\.|$)")
def _counter_with_mv(m):
    from mtg_ast import CounterSpell
    return CounterSpell(target=Filter(base="spell", targeted=True))


# "add <N> {mana}. you can cast only one more spell this turn" — Irencrag Feat
@_er(r"^add (\w+) \{[^}]+\}\. you can cast only one more spell this turn"
     r"(?:\.|$)")
def _irencrag(m):
    return UnknownEffect(raw_text="add mana rider")


# "add <N> {mana} <tail>?" — bare ritual
@_er(r"^add (?:\w+ )?\{[^}]+\}(?:\.|$)")
def _add_mana_bare(m):
    from mtg_ast import AddMana
    return AddMana(amount=1)


# "you may copy this spell ..." / "when you cast this spell, copy it ..."
# These are tricky but we add a generic "copy target spell" wrapper.
@_er(r"^copy target spell(?:\.|$)")
def _copy_target_spell(m):
    from mtg_ast import CopySpell
    return CopySpell(target=Filter(base="spell", targeted=True))


# "you make all choices for <filter> you control" — Spuzzem Strategist
@_er(r"^you make all choices for [^.]+ you control(?:\.|$)")
def _make_choices(m):
    return UnknownEffect(raw_text="make choices for")


# "shuffle <N> target cards from a single graveyard into their owner's
# library" — Serene Remembrance
@_er(r"^shuffle (?:up to )?(\d+|x|any number of) target ([^.]+?) cards? "
     r"from [^.]+ graveyards? into (?:their|its) owners?'? libraries?"
     r"(?:\.|$)")
def _shuffle_n_cards(m):
    return UnknownEffect(raw_text=f"shuffle {m.group(1)} cards into lib")


# "shuffle ~ and <N> target cards from a single graveyard into their owners'
# libraries" — Serene Remembrance
@_er(r"^shuffle ~ and up to (\d+|x) target cards? from [^.]+(?:\.|$)")
def _shuffle_self_and_cards(m):
    return UnknownEffect(raw_text="shuffle self and cards")


# "shuffle all cards from your graveyard into your library. target player
# mills that many cards" — Psychic Spiral
@_er(r"^shuffle all cards from your graveyard into your library\.[^.]+"
     r"(?:\.|$)")
def _shuffle_all_then(m):
    return UnknownEffect(raw_text="shuffle all then")


# "exile all <filter>" — generic compact exile
@_er(r"^exile all ([^.]+?)(?:\.|$)")
def _exile_all(m):
    return Exile(target=Filter(base=m.group(1).strip(), targeted=False))


# "goad all creatures you don't control" — Disrupt Decorum
@_er(r"^goad all ([^.]+?)(?:\.|$)")
def _goad_all(m):
    return UnknownEffect(raw_text=f"goad {m.group(1).strip()}")


# "redistribute any number of players' life totals" — Reverse the Sands
@_er(r"^redistribute any number of players'? life totals(?:\.|$)")
def _redistribute_life(m):
    return UnknownEffect(raw_text="redistribute life")


# "search target opponent's library for a card with that name" — Necromentia
# partial / other search-opp-lib shapes
@_er(r"^search (?:target opponent'?s|your|their) library for [^.]+"
     r"(?:\.|$)")
def _search_generic(m):
    return UnknownEffect(raw_text="search library generic")


# "separate <filter> into two piles. exile the pile of <X>'s choice and
# return the other" — Death or Glory
@_er(r"^separate all [^.]+ into two piles[^.]*(?:\.|$)")
def _split_piles(m):
    return UnknownEffect(raw_text="split piles")


# "take an extra turn after this one" (target player variants)
@_er(r"^take an extra turn after this one(?:\.|$)")
def _take_extra_turn(m):
    from mtg_ast import ExtraTurn
    return ExtraTurn(count=1)


# "draw X cards, where X is ..." — Lucid Dreams-style
@_er(r"^draw (x|\d+) cards?,? where [^.]+(?:\.|$)")
def _draw_x_where(m):
    return Draw(count=_num(m.group(1)))


# "you draw <N> cards. then you may ..." — Mind into Matter opener
@_er(r"^draw (\w+) cards?\.[\s]+then (.+?)(?:\.|$)")
def _draw_then_may(m):
    return Sequence(items=(Draw(count=_num(m.group(1))),
                           UnknownEffect(raw_text=m.group(2).strip())))


# "scry X, where X is <N>, then draw <N> cards" — Ugin's Insight
@_er(r"^scry (x|\d+)(?:,? where [^.]+)?,? then draw (\d+|x|three) cards?"
     r"(?:\.|$)")
def _scry_x_then_draw(m):
    from mtg_ast import Scry
    return Sequence(items=(Scry(amount=_num(m.group(1))),
                           Draw(count=_num(m.group(2)))))


# "pore over the pages" shape: "draw three cards, untap up to two lands,
# then discard a card"
@_er(r"^draw (\d+|one|two|three|four) cards?, untap up to [^.]+,? "
     r"then discard [^.]+(?:\.|$)")
def _pore_shape(m):
    return Sequence(items=(Draw(count=_num(m.group(1))),
                           UnknownEffect(raw_text="untap"),
                           Discard(count=1)))


# "search your library for a card, put that card into your hand, then
# shuffle" — bare tutor (base parser handles some, not all variants)
@_er(r"^search your library for a card, put (?:that card|it) into your "
     r"hand,? then shuffle(?:\.|$)")
def _tutor_any_to_hand(m):
    from mtg_ast import Tutor
    return Tutor(search_filter=Filter(base="any_card", targeted=False),
                 to="hand")


# "~ deals X damage to any target" / "target creature gets +3/+0 until end
# of turn" — Pedal to the Metal-style
# (covered separately; add pump with single-stat +X/+0)
@_er(r"^target creature gets \+(x|\d+)/\+(\d+) and gains ([^.]+?) until "
     r"end of turn(?:\.|$)")
def _pump_and_gain(m):
    buff = Buff(target=TARGET_CREATURE, power=_num(m.group(1)),
                toughness=int(m.group(2)), duration="until_end_of_turn")
    grant = GrantAbility(target=TARGET_CREATURE,
                         ability_name=m.group(3).strip(),
                         duration="until_end_of_turn")
    return Sequence(items=(buff, grant))


# "X target creatures gain/get +/-N/+/-N until end of turn" / "X target
# blocked creatures assign ..." — Outmaneuver, Part Water
@_er(r"^x target ([^.]+?) (?:can'?t [^.]+|assign [^.]+|gain [^.]+|get "
     r"[+-]\d+/[+-]\d+[^.]*)(?:\.|$)")
def _x_target_stuff(m):
    return UnknownEffect(raw_text=f"x target {m.group(1).strip()}")


# "choose up to one creature. destroy the rest." — Duneblast (bare variant)
@_er(r"^choose (?:up to )?one creature\.\s+destroy the rest(?:\.|$)")
def _duneblast(m):
    return Destroy(target=Filter(base="all_creatures_except_one", targeted=False))


# "choose a creature at random, then destroy the rest" — Last One Standing
@_er(r"^choose a creature at random,? then destroy the rest(?:\.|$)")
def _last_one_standing(m):
    return Destroy(target=Filter(base="all_creatures_except_one_random",
                                 targeted=False))


# "the owner of target <filter> shuffles/puts/sends it to <zone>" —
# Cathartic Parting, Endless Detour, Run Out of Town, Unlucky Drop,
# Sudden Setback
@_er(r"^the owner of target [^.]+? (?:shuffles? it into their library|"
     r"puts it on their choice of the top or bottom of their library)"
     r"(?:\.|$)")
def _owner_puts(m):
    return UnknownEffect(raw_text="owner puts on lib")


# "put <N> target lands on top of their owners' libraries" — Plow Under
@_er(r"^put (two|three|\d+) target lands? on top of their owners?'? "
     r"libraries?(?:\.|$)")
def _put_lands_top(m):
    return UnknownEffect(raw_text=f"put {m.group(1)} lands top")


# "put two target artifacts on top of their owners' libraries" — Rebuking
# Ceremony
@_er(r"^put (two|three|\d+) target artifacts? on top of their owners?'? "
     r"libraries?(?:\.|$)")
def _put_artifacts_top(m):
    return UnknownEffect(raw_text="put artifacts top")


# "put all enchantments on top of their owners' libraries" — Harmonic
# Convergence
@_er(r"^put all ([^.]+?) on top of their owners?'? libraries?(?:\.|$)")
def _put_all_top_lib(m):
    return UnknownEffect(raw_text=f"put all {m.group(1).strip()} top lib")


# "put all creatures on the bottom of their owners' libraries" — Hallowed
# Burial
@_er(r"^put all ([^.]+?) on the bottom of their owners?'? libraries?(?:\.|$)")
def _put_all_bottom_lib_v2(m):
    return UnknownEffect(raw_text=f"put all {m.group(1).strip()} bottom lib")


# ===========================================================================
# TRIGGER_PATTERNS
# ===========================================================================

# (re_pattern, event_name, scope)
TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []


def _tp(pattern: str, event: str, scope: str = "any"):
    TRIGGER_PATTERNS.append((re.compile(pattern, re.I | re.S), event, scope))


# NOTE: patterns below intentionally do NOT capture the effect body —
# the core parser uses `m.end()` to find where the trigger clause ends and
# hands the rest to `parse_effect` / `UnknownEffect`. Capturing the body
# would make `m.end()` span the entire line and leave no rest to parse.

# "when you cast a <filter> spell, <effect>" — Jackalope Herd / Skittering
# Horror ("when you cast a creature spell, sacrifice this creature")
_tp(r"^when you cast (?:an? |a creature )?(?:[^,]+?)(?: spell)?",
    event="cast_spell", scope="you")


# "whenever another creature enters, you (may )?gain 1 life" — Soul Warden,
# Essence Warden, Soul's Attendant
_tp(r"^whenever another creature enters(?: the battlefield)?",
    event="another_creature_enters", scope="any")


# "whenever another <type> enters" — Elvish Vanguard, Wirewood Hivemaster,
# Decorated Champion. Also handles "whenever another creature you control
# enters" (core had a pattern for this but it only matched when followed by
# an effect parse_effect could handle; this one promotes UNPARSED to partial
# regardless).
_tp(r"^whenever another ([a-z][a-z\- ]+?) (?:you control |your team control )?"
    r"enters(?: the battlefield)?",
    event="another_typed_enters", scope="you")


# "whenever another <type> dies" — Village Cannibals, Skemfar Avenger
_tp(r"^whenever another ([a-z][a-z\- ]+?) (?:you control )?dies",
    event="another_typed_dies", scope="you")


# "whenever another nontoken <type> is put into your graveyard from the
# battlefield" — Prowess of the Fair, Boggart Shenanigans
_tp(r"^whenever another (?:nontoken )?([a-z][a-z\- ]+?) (?:you control )?"
    r"is put into your graveyard from the battlefield",
    event="nontoken_type_to_gy", scope="you")


# "whenever a <filter> is put into a graveyard from the battlefield"
# — Viridian Revel, Scrapheap, Magnetic Mine, Planar Void
_tp(r"^whenever (?:an? |another )?([a-z][a-z\- ]+?) is put into "
    r"(?:an opponent'?s |your )?graveyard from (?:the battlefield|anywhere)",
    event="permanent_to_gy", scope="any")


# "whenever a player taps a <land type> for mana" — Vernal Bloom,
# Sanctimony, Mana Flare, Keeper of Progenitus
_tp(r"^whenever (?:a player|an opponent) taps an? ([a-z][a-z\-, ]+?) for mana",
    event="land_tapped_for_mana", scope="any")


# "whenever you cycle/discard a card"
_tp(r"^whenever you (?:cycle|discard) a card",
    event="you_cycle_or_discard", scope="you")


# "whenever you manifest dread / roll dice / scry / surveil / connive / explore"
_tp(r"^whenever you (?:manifest dread|roll (?:one or more )?dice|scry|surveil|"
    r"connive|explore|seek (?:one or more )?cards?)",
    event="you_mechanic", scope="you")


# "whenever one or more tokens you control enter" — Woodland Champion
_tp(r"^whenever one or more tokens you control enter(?: the battlefield)?",
    event="tokens_you_enter", scope="you")


# "whenever a player kicks a spell" — Saproling Infestation
_tp(r"^whenever a player kicks a spell",
    event="spell_kicked", scope="any")


# "whenever another creature or planeswalker you control dies" — Rising
# Populace
_tp(r"^whenever another creature or planeswalker you control dies",
    event="creature_or_pw_dies", scope="you")


# "whenever you activate a <filter> ability" — Frenzied Raider
_tp(r"^whenever you activate an? ([a-z][a-z\- ]+?) ability",
    event="activate_typed_ability", scope="you")


# "whenever you give a gift / finish voting / clash" — Jolly Gerbils /
# Grudge Keeper
_tp(r"^whenever (?:players? )?(?:you )?(?:give a gift|finish voting|"
    r"clash[^,]*)",
    event="you_misc_event", scope="you")


# "whenever a creature you control mutates" — Essence Symbiote
_tp(r"^whenever (?:a|another) ([a-z][a-z\- ]+?) (?:you control )?"
    r"(?:mutates|perpetually [^,]+|is dealt (?:\d+ or more )?(?:combat )?damage)",
    event="creature_modified_event", scope="you")


# "during each player's upkeep, that player <verb>"
_tp(r"^during each player'?s upkeep",
    event="each_upkeep", scope="any")


# "whenever this creature is dealt N or more damage" — Innocent Bystander
_tp(r"^whenever (?:~|this creature) is dealt (?:\d+|x) or more damage",
    event="self_dealt_damage", scope="self")


# "whenever a player draws their <Nth> card each turn" — Ian Malcolm
_tp(r"^whenever a player draws their \w+ card each turn",
    event="nth_card_drawn", scope="any")


# "whenever a nontoken creature enters, if <cond>" — Genesis Chamber
_tp(r"^whenever a (?:nontoken )?creature enters, if [^,]+",
    event="nontoken_creature_enters_cond", scope="any")


# "whenever <list of types> you control deals combat damage to a player" —
# Spawning Kraken
_tp(r"^whenever an? (?:nontoken |attacking )?([a-z][a-z\-, ]+?) "
    r"(?:you control )?deals combat damage to a player",
    event="typed_combat_dmg", scope="you")


# "whenever a spell you've cast is countered" — Multani's Presence
_tp(r"^whenever a spell you'?ve cast is countered",
    event="your_spell_countered", scope="you")


# "whenever your opponents are dealt combat damage[, if <cond>]" —
# Mindblade Render
_tp(r"^whenever your opponents are dealt combat damage(?:, if [^,]+)?",
    event="opponents_dealt_combat_dmg", scope="opponents")


# "whenever an opponent pays a tax from [...]" — Tax Taker
_tp(r"^whenever an opponent pays a tax from [^,]+",
    event="opponent_pays_tax", scope="you")


# "whenever a spell you control causes you to <verb>" — Toofer
_tp(r"^whenever a spell you control causes you to (?:gain card advantage|"
    r"draw|discard|sacrifice) [^,]*",
    event="your_spell_causes_action", scope="you")


# "whenever a(n) <type> enters under your control" — Slimefoot (Swamp or
# Forest entering under your control)
_tp(r"^whenever an? ([a-z][a-z\- ]+?) enters (?:the battlefield )?"
    r"under your control",
    event="typed_enters_your_control", scope="you")


# "whenever this creature or another <filter> leaves/enters/dies/attacks" —
# compound self-or-ally. Three Tree Scribe etc.
_tp(r"^whenever (?:~|this creature) or another ([a-z][a-z\- ]+?) "
    r"(?:you control )?(?:leaves the battlefield(?: without dying)?|enters|"
    r"dies|attacks)",
    event="self_or_ally_event", scope="self")


# "whenever a player returns a permanent to a player's hand, ..." — Warped
# Devotion (permanent returned -> discard)
_tp(r"^whenever a permanent is returned to a player'?s hand",
    event="permanent_returned", scope="any")


# "whenever an artifact or enchantment is put into your graveyard ..." —
# Scrapheap
_tp(r"^whenever an? (?:artifact|enchantment|creature|planeswalker|land)s? "
    r"(?:and/or (?:artifact|enchantment|creature|planeswalker|land)s? )?"
    r"(?:is|are) put into your graveyard from the battlefield",
    event="typed_to_your_gy", scope="you")


# "whenever an artifact is put into an opponent's graveyard" — Viridian Revel
_tp(r"^whenever an? ([a-z][a-z\- ]+?) is put into an opponent'?s graveyard "
    r"from the battlefield",
    event="typed_to_opp_gy", scope="opponent")


# "whenever an instant or sorcery spell is cast during your turn" —
# Sentinel Tower
_tp(r"^whenever an? (?:instant or sorcery|instant|sorcery|noncreature|"
    r"artifact|creature) spell is cast(?: during your turn)?",
    event="spell_cast_typed", scope="any")


# "whenever you're dealt damage" — Darien, King of Kjeldor
_tp(r"^whenever you'?re dealt (?:combat )?damage",
    event="you_dealt_damage", scope="you")


# "whenever ~ enters or attacks" — Aang and Katara-style compound trigger
_tp(r"^whenever (?:~|this creature) enters or attacks",
    event="self_etb_or_attack", scope="self")


# "whenever ~ (becomes tapped|becomes untapped|is tapped)" — Tui and La
_tp(r"^whenever (?:~|this creature) becomes? (?:tapped|untapped)",
    event="self_tap_state_change", scope="self")


# "whenever a spell or ability is put onto the stack, if ..." — Grip of Chaos
_tp(r"^whenever a spell or ability is put onto the stack(?:, if [^,]+)?",
    event="spell_or_ability_on_stack", scope="any")


# "whenever a swamp or forest enters under your control" — Slimefoot; the
# typed_enters_your_control pattern above already handles this shape but we
# add this compound form explicitly because "swamp or forest" doesn't match
# a single [a-z][a-z\- ]+? run after comma.
_tp(r"^whenever an? [a-z]+ or [a-z]+ enters under your control",
    event="compound_typed_enters", scope="you")


# "whenever this creature is dealt 3 or more damage, investigate" —
# reuse the broader "self_dealt_damage" bucket. No new pattern needed.


# "whenever a nontoken modified creature you control dies" —
# Akki Ember-Keeper
_tp(r"^whenever a nontoken modified creature you control dies",
    event="nontoken_modified_creature_dies", scope="you")


# "whenever an attacking <type> or <type> is put into your graveyard ..."
# — Kithkin Mourncaller
_tp(r"^whenever an attacking [a-z~\- ]+? is put into your graveyard from "
    r"the battlefield",
    event="attacking_type_to_gy", scope="you")


# "whenever a creature with power N or greater enters" — Kavu Lair
_tp(r"^whenever a creature with power \d+ or (?:greater|less) enters",
    event="power_threshold_etb", scope="any")


# "whenever another <type> your team controls enters" — Decorated Champion
# (handled above via "another typed enters" with the new optional
# "your team control" scope).


# "whenever ~ or another <type> enters under your control" — Fludge-style
# compound ETB with filter list
_tp(r"^whenever (?:~|this creature) or another [^,]+ enters(?: the "
    r"battlefield)?(?: under your control)?",
    event="self_or_typed_etb", scope="self")


# "whenever another <type> is put into a graveyard from anywhere" —
# Planar Void / Bereavement
_tp(r"^whenever an? ([a-z][a-z\- ]+?) (?:card )?is put into (?:an?|any) "
    r"graveyard from anywhere",
    event="typed_to_any_gy_anywhere", scope="any")


# "whenever a green creature dies, its controller discards a card" —
# Bereavement
_tp(r"^whenever a (?:nontoken )?([a-z]+) creature dies",
    event="color_creature_dies", scope="any")


# "whenever this creature is dealt damage by a creature" — niche (matches
# several singletons)
_tp(r"^whenever (?:~|this creature) is dealt damage by [^,]+",
    event="self_dealt_damage_by", scope="self")


# ---------------------------------------------------------------------------
# Exports
# ---------------------------------------------------------------------------

__all__ = ["EFFECT_RULES", "STATIC_PATTERNS", "TRIGGER_PATTERNS"]


if __name__ == "__main__":
    print(f"STATIC_PATTERNS: {len(STATIC_PATTERNS)}")
    print(f"EFFECT_RULES:    {len(EFFECT_RULES)}")
    print(f"TRIGGER_PATTERNS:{len(TRIGGER_PATTERNS)}")
