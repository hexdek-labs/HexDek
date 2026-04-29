#!/usr/bin/env python3
"""PARTIAL-bucket scrubber.

Family: PARTIAL → GREEN promotions. This module targets abilities that the
base parser currently spits out as `parse_errors` fragments on otherwise-
parsed cards. The patterns here were picked by bucketing PARTIAL-card
fragments by first-N words and taking the biggest clusters that map cleanly
onto a single-line grammar production.

Three export tables are merged by `parser.load_extensions`:

- ``STATIC_PATTERNS`` — ``(compiled_regex, builder)``; consulted by
  ``parse_static``. Builders return a ``Static`` / ``Keyword`` / ``None``.
- ``EFFECT_RULES``   — ``(compiled_regex, builder)``; appended to
  ``EFFECT_RULES``. Builders return an ``Effect`` node (commonly ``Buff``
  combined via ``GrantAbility`` or an ``UnknownEffect`` placeholder whose
  ``raw_text`` preserves the shape).
- ``TRIGGER_PATTERNS`` — none here; the clusters we picked are either body
  effects or standalone static/keyword phrasings.

Ordering: specific-first. Each cluster ships a short comment naming the
cards that motivated it, so when something regresses the culprit is easy
to bisect.
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
    Buff, Damage, Filter, GrantAbility, Keyword, LoseLife, Modification,
    Sequence, Static, UnknownEffect,
    TARGET_CREATURE,
)


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — ability-level static / keyword shapes
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Kicker ({cost}) / Kicker ({a} and/or {b}) -----------------------------
# Cluster (121): Archangel of Wrath "kicker {b} and/or {r}", Oran-Rief Recluse
# "kicker {2}{g}", etc. KEYWORD_RE only recognized single-cost kicker.
@_sp(r"^kicker (\{[^}]+\}(?:\{[^}]+\})*(?:\s+and/or\s+\{[^}]+\}(?:\{[^}]+\})*)?)\s*$")
def _kicker(m, raw):
    return Keyword(name="kicker", args=(m.group(1),), raw=raw)


# --- Ninjutsu ({cost}) ------------------------------------------------------
# Cluster (39). KEYWORD_RE didn't list ninjutsu.
@_sp(r"^ninjutsu (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _ninjutsu(m, raw):
    return Keyword(name="ninjutsu", args=(m.group(1),), raw=raw)


# --- Entwine ({cost}) -------------------------------------------------------
@_sp(r"^entwine (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _entwine(m, raw):
    return Keyword(name="entwine", args=(m.group(1),), raw=raw)


# --- Transmute ({cost}) -----------------------------------------------------
@_sp(r"^transmute (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _transmute(m, raw):
    return Keyword(name="transmute", args=(m.group(1),), raw=raw)


# --- Offspring ({cost}) — MH3 keyword --------------------------------------
@_sp(r"^offspring (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _offspring(m, raw):
    return Keyword(name="offspring", args=(m.group(1),), raw=raw)


# --- Firebending N ----------------------------------------------------------
@_sp(r"^firebending (\d+)\s*$")
def _firebending(m, raw):
    return Keyword(name="firebending", args=(int(m.group(1)),), raw=raw)


# --- Modular N --------------------------------------------------------------
@_sp(r"^modular (\d+)\s*$")
def _modular(m, raw):
    return Keyword(name="modular", args=(int(m.group(1)),), raw=raw)


# --- Mobilize N / Mobilize X, where X is ... -------------------------------
@_sp(r"^mobilize (\d+|x(?:,.*)?)\s*$")
def _mobilize(m, raw):
    return Keyword(name="mobilize", args=(m.group(1),), raw=raw)


# --- Bare-keyword one-liners (ability-word / evergreen shapes) -------------
# Triggers a lot of low-volume misses: training, riot, learn, dethrone,
# ingest, increment, living metal. The parser's KEYWORD_RE listed most of
# these — but they live under *explicit* patterns (e.g. dredge \d+), so
# the bare word form fell through. These are promoted to a Keyword node.
_BARE_KEYWORDS = {
    "training", "dethrone", "riot", "learn", "ingest", "increment",
    "living metal", "melee", "myriad", "enlist", "mentor", "exalted",
    "battle cry", "assist", "hideaway",  # latter rarely bare but harmless
    "double agenda", "council's dilemma", "squad",
    "job select", "reconfigure", "backup",
    "for mirrodin!", "living weapon",
    "suspend", "bloodthirst", "evolve",
    "prowl", "persist", "undying",
    "boast", "disturb", "foretell", "plot",
    "cycling", "flashback", "madness",
    "conspire", "retrace", "cipher",
    "the ring tempts you", "start your engines!",
}


@_sp(r"^([a-z][a-z'!\- ]+?)\s*$")
def _bare_keyword(m, raw):
    word = m.group(1).strip()
    if word in _BARE_KEYWORDS:
        return Keyword(name=word, raw=raw)
    return None


# --- Custom ward: "ward-discard a card", "ward-pay N life", etc. -----------
# Cluster (47). Scryfall renders non-mana ward as "ward—<cost>"; our normalize
# collapses the dash to a hyphen.
@_sp(r"^ward-(.+?)\s*$")
def _ward_custom(m, raw):
    return Keyword(name="ward", args=(m.group(1).strip(),), raw=raw)


# --- "~ enters the battlefield tapped" --------------------------------------
# Cluster (12). Older oracle phrasing the base parser's etb_tapped rule missed
# (it expects "enters tapped", not "enters the battlefield tapped").
@_sp(r"^~ enters the battlefield tapped\s*$")
def _self_etb_tapped_full(m, raw):
    return Static(modification=Modification(kind="etb_tapped"), raw=raw)


# --- "this land doesn't untap during your (next) untap step [if depl]" ------
# Cluster (15). Depletion lands / Frost Titan flavor.
@_sp(r"^this land doesn'?t untap during your (?:next )?untap step"
     r"(?: if it has a depletion counter on it)?\s*$")
def _land_no_untap(m, raw):
    return Static(modification=Modification(kind="no_untap"), raw=raw)


# --- "this artifact|land deals N damage to you" — painland tail ------------
# Cluster (10). Already handled for the word "land" in the base parser; we
# broaden to "artifact" (Sol-Ring-ish pain flavor on a few cards).
@_sp(r"^this (?:artifact|land) deals \d+ damage to you\s*$")
def _pain_tail(m, raw):
    return Static(modification=Modification(kind="painland_tail"), raw=raw)


# --- "you have hexproof" / "you have hexproof and indestructible" -----------
# Cluster (9). Leyline of Sanctity, Spirit of the Hearth, Witchbane Orb.
@_sp(r"^you have (hexproof(?: and indestructible)?|shroud|protection from [^.]+)\s*$")
def _you_have_kw(m, raw):
    return Static(modification=Modification(kind="player_keyword_grant",
                                            args=(m.group(1),)), raw=raw)


# --- "this creature must be blocked if able" --------------------------------
# Cluster (8). Provoke-ish / Lure-ish static flavor.
@_sp(r"^this creature must be blocked if able\s*$")
def _must_be_blocked(m, raw):
    return Static(modification=Modification(kind="must_be_blocked"), raw=raw)


# --- "they're still lands" — Awakening / Sylvan Awakening tail -------------
@_sp(r"^they'?re still lands\s*$")
def _still_lands(m, raw):
    return Static(modification=Modification(kind="still_lands_tail"), raw=raw)


# --- "damage can't be prevented this turn" ---------------------------------
@_sp(r"^damage can'?t be prevented this turn\s*$")
def _damage_no_prevent(m, raw):
    return Static(modification=Modification(kind="damage_no_prevent"), raw=raw)


# --- "skip your draw step" --------------------------------------------------
@_sp(r"^skip your draw step\s*$")
def _skip_draw(m, raw):
    return Static(modification=Modification(kind="skip_phase", args=("draw",)),
                  raw=raw)


# --- "you choose a (non-land) card from it" — Vendilion-tail ---------------
# Base parser has "^you choose a [^.]+ card from it" but requires the word
# "card" — oracle text sometimes phrases without it in older printings.
@_sp(r"^you choose (?:a|an) (?:nonland |noncreature |artifact |instant |sorcery )?card from it\s*$")
def _opp_choice_card(m, raw):
    return Static(modification=Modification(kind="opp_choice_card_pick"), raw=raw)


# --- "its controller loses N life" / "its controller gains N life" ---------
# These are rare static tails (Backlash-tail flavor), but they show up as
# continuation sentences the splitter didn't glue.
@_sp(r"^its controller (loses|gains) (\d+) life\s*$")
def _its_controller_life(m, raw):
    verb, n = m.group(1), int(m.group(2))
    return Static(modification=Modification(
        kind="its_controller_life",
        args=(verb, n)), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES — body-level effect shapes (used by parse_effect + spell_effect)
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "target creature gets +N/+N and gains <kws> until end of turn" --------
# Cluster (50). Pump-plus-grant instants (Giant Growth variants).
@_er(r"^target creature gets ([+-]\d+)/([+-]\d+) and gains ([a-z, ]+?) until end of turn(?:\.|$)")
def _target_buff_and_gain(m):
    p = int(m.group(1))
    t = int(m.group(2))
    kws = m.group(3).strip()
    # Combine the buff and the grant into a Sequence so both pieces survive.
    return Sequence(items=(
        Buff(power=p, toughness=t, target=TARGET_CREATURE),
        GrantAbility(ability_name=kws, target=TARGET_CREATURE),
    ))


# --- "target creature gains <kws> until end of turn" ------------------------
# Cluster (35). Bare-grant instants (Flight, Teferi's Curse-style).
@_er(r"^target creature gains ([a-z, ]+?) until end of turn(?:\.|$)")
def _target_grant(m):
    return GrantAbility(ability_name=m.group(1).strip(), target=TARGET_CREATURE)


# --- "that creature gains haste" / "that creature gains <kws>" -------------
# Cluster (11). Reanimator-rider / haste-rider continuations.
@_er(r"^that creature gains ([a-z, ]+?)(?: until end of turn)?(?:\.|$)")
def _that_creature_gains(m):
    return GrantAbility(ability_name=m.group(1).strip(),
                        target=Filter(base="that_creature", targeted=False))


# --- "target creature gains haste" (bare, no duration) ----------------------
@_er(r"^target creature gains haste(?:\.|$)")
def _tgt_haste(m):
    return GrantAbility(ability_name="haste", target=TARGET_CREATURE)


# --- "attach it to target creature (you control)" ---------------------------
# Cluster (12). Reconfigure/Equipment triggers that continue into an attach.
@_er(r"^attach it to target creature(?: you control)?(?:\.|$)")
def _attach_it(m):
    return UnknownEffect(raw_text="attach it to target creature")


# --- "flip a coin." / "flip a coin and call it" -----------------------------
# Cluster (12). Coin-flip cards with trailing effect on another sentence.
@_er(r"^flip a coin(?:\.|$)")
def _flip_coin(m):
    return UnknownEffect(raw_text="flip a coin")


# --- "you may play an additional land this turn" ----------------------------
# Cluster (8). Base rule exists for "on each of your turns"; this variant
# (one-shot, for the current turn, used on spells like Exploration-one-off)
# wasn't covered.
@_er(r"^you may play an additional land this turn(?:\.|$)")
def _extra_land_this_turn(m):
    return UnknownEffect(raw_text="extra land this turn")


# --- "you may cast that card (without paying its mana cost|this turn|for ...)"
# Cluster (25). Chaos Wand / cascade-style continuations.
@_er(r"^you may cast that card(?: without paying its mana cost| this turn| for as long[^.]*)?"
     r"(?:\.|$)")
def _may_cast_that(m):
    return UnknownEffect(raw_text="may cast that card")


# --- "you may search your library and/or graveyard for a card named ..." ----
# Cluster (10). Liliana's-Scorn-family tutor for a specific named card.
@_er(r"^you may search your library and/or graveyard for a card named [^,.]+,"
     r"[^.]+(?:\.|$)")
def _search_lib_and_gy(m):
    return UnknownEffect(raw_text="tutor named card from lib and/or gy")


# --- "target player sacrifices a creature (or planeswalker) of their choice"
# Cluster (7-9). Edict effects with the explicit "of their choice" tail.
@_er(r"^target player sacrifices a creature(?: or planeswalker)? of their choice"
     r"(?:\.|$)")
def _edict_of_their_choice(m):
    return UnknownEffect(raw_text="edict of their choice")


# --- "that creature deals damage equal to its power to <target>" -----------
# Cluster (9). Backlash / Hunter's Bow style reverse-fight.
@_er(r"^that creature deals damage equal to its power to ([^.]+?)(?:\.|$)")
def _that_creature_dmg_power(m):
    return UnknownEffect(raw_text=f"that_creature dmg=power to {m.group(1).strip()}")


# --- "when you control no <basic>, sacrifice this creature" -----------------
# Cluster (16). Island/Swamp/etc. commitment ("when you control no islands,
# sacrifice this creature"). These read as triggered statics — the parser
# treated the whole sentence as an effect rather than a conditional trigger.
# We register it as an EFFECT so the spell_effect wrapping catches it.
@_er(r"^when you control no (islands?|swamps?|mountains?|forests?|plains?|"
     r"[a-z ]+s),? sacrifice this creature(?:\.|$)")
def _commitment_sac(m):
    return UnknownEffect(raw_text=f"commitment-sac if no {m.group(1)}")


# --- Modal option bodies that the splitter hands us with a leading bullet --
# Cluster (~70 combined across the "• destroy target …", "• draw a card",
# "• create a treasure token", "• put a +1/+1 counter on this creature"
# shapes). The splitter pass keeps the bullet when the modal-consolidation
# step didn't swallow the header; a leading "• " prefix then blocks every
# effect rule. We re-run the body without the bullet.
@_er(r"^•\s*(.+?)(?:\.|$)")
def _modal_bullet_passthrough(m):
    # Defer to the main parser's recursive effect matcher via module import.
    # The rule returns UnknownEffect as a safety net; when parse_effect
    # succeeds on the stripped body that result is used instead (see the
    # post-processing hook below via lazy import).
    body = m.group(1).strip()
    try:  # lazy import to avoid circular at module-load time
        import parser as _p
        inner = _p.parse_effect(body)
        if inner is not None:
            return inner
    except Exception:
        pass
    return UnknownEffect(raw_text=f"modal-option: {body}")


# --- "create a token that's a copy of it[, except ...]" --------------------
# Cluster (10). Parallel Lives-style copy continuation where "it" is the
# just-created token.
@_er(r"^create a token that'?s a copy of it(?:, except [^.]+)?(?:\.|$)")
def _token_copy_it(m):
    return UnknownEffect(raw_text="create token copy of it")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — none for this module (see docstring).
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []
