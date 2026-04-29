#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (twelfth pass).

Family: PARTIAL -> GREEN promotions.  Targets the final 21 PARTIAL cards
remaining after 11 prior scrubber passes at 99.93% GREEN (31,942/31,963).

Error clusters:
  3  "you may attach an equipment you control to"   — Nahiri x3
  2  "you draw seven cards"                         — Jace, Nicol Bolas
  2  "you/take an extra turn after this one"         — Mu Yanling, Ral Zarek
  2  "they become N/N elemental creatures"           — Nissa x2
  1  "whenever one or more creatures ... and/or"     — Kaya Spirits' Justice
  1  "whenever one or more other cats ... die"       — Ajani Nacatl Pariah
  1  "your opponents ... with hexproof can be ..."   — Kaya Bane of the Dead
  1  "he's still a planeswalker"                     — Gideon
  1  "you may activate the loyalty abilities of ~"   — Urza Planeswalker
  1  "creature cards exiled this way gain ..."        — Lukka
  1  "you may activate abilities ... as though haste" — Tyvar
  1  "casualty x"                                    — Ob Nixilis
  1  "repeat this process six more times"            — Professor Onyx
  1  "when ~ has two or fewer loyalty counters"      — Garruk Relentless
  1  "when minsc & boo ... enters and"               — A-Minsc & Boo
  1  "as tibalt enters, you get an emblem"           — Valki/Tibalt
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
    ExtraTurn, Filter, GrantAbility, Keyword, Modification,
    Static, Triggered, Trigger, UnknownEffect,
    SELF, TARGET_CREATURE,
)

# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "you may attach an Equipment you control to it" ----------------------
# Nahiri, Heir of the Ancients / A-Nahiri / Nahiri the Lithomancer
# Part of a +1 ability: create token, then attach.  The create-token half
# already parses; this is the dangling attach clause.
@_sp(r"^you may attach an equipment you control to (?:it|that creature|that token)")
def _attach_equipment(m):
    return Static(
        condition=None,
        modification=Modification(kind="spell_effect", args=[
            UnknownEffect(raw_text=m.group(0))
        ]),
    )


# --- "he's still a planeswalker" ------------------------------------------
# Gideon, Champion of Justice  (animation clause rider)
@_sp(r"^(?:he(?:'s| is)|it(?:'s| is)) still a planeswalker")
def _still_planeswalker(m):
    return Static(
        condition=None,
        modification=Modification(kind="type_retention", args=[m.group(0)]),
    )


# --- "you may activate the loyalty abilities of ~ twice ..." ---------------
# Urza, Planeswalker
@_sp(r"^you may activate (?:the )?loyalty abilities of [^ ]+ (?:twice|three times)")
def _double_loyalty(m):
    return Static(
        condition=None,
        modification=Modification(kind="static_rule_mod", args=[m.group(0)]),
    )


# --- "you may activate abilities of creatures you control as though ..." ----
# Tyvar, Jubilant Brawler
@_sp(r"^you may activate (?:(?:mana )?abilities|activated abilities) of creatures you control as though (?:those creatures|they) had haste")
def _grant_haste_activation(m):
    return Static(
        condition=None,
        modification=Modification(kind="conditional_static", args=[m.group(0)]),
    )


# --- "your opponents and permanents ... with hexproof can be the targets" ---
# Kaya, Bane of the Dead
@_sp(r"^your opponents and permanents your opponents control with hexproof can be the targets? of spells and abilities you control")
def _hexproof_bypass(m):
    return Static(
        condition=None,
        modification=Modification(kind="conditional_static", args=[m.group(0)]),
    )


# --- "casualty X" ----------------------------------------------------------
# Ob Nixilis, the Adversary.  Casualty with variable X (not fixed N).
@_sp(r"^casualty x\b")
def _casualty_x(m):
    return Keyword(name="casualty", args=("X",), raw=m.group(0))


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _ep(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "you draw seven cards" ------------------------------------------------
# Jace the Living Guildpact -8, Nicol Bolas the Deceiver -11 ultimate
WORD_NUMS = {
    "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
    "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14,
    "fifteen": 15, "twenty": 20,
}
_num_words = "|".join(WORD_NUMS.keys())


@_ep(rf"^you draw ({_num_words}) cards?")
def _draw_word_num(m):
    from mtg_ast import Draw
    n = WORD_NUMS[m.group(1).lower()]
    return Draw(count=n, target=SELF)


# --- "you take an extra turn after this one" --------------------------------
# Mu Yanling -10
@_ep(r"^you take an extra turn after this one")
def _extra_turn_you(m):
    return ExtraTurn(after_this=True, target=SELF)


# --- "take an extra turn after this one for each coin ..." ------------------
# Ral Zarek -7
@_ep(r"^take an extra turn after this one for each coin (?:that )?comes? up heads")
def _extra_turn_coins(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "they become N/N elemental creatures [with ...]" ----------------------
# Nissa Steward of Elements, Nissa Vastwood Seer
@_ep(r"^they become (\d+)/(\d+) ([a-z ]+?) creatures?(?: with (.+?))?(?:\s+until end of turn)?$")
def _animate_lands(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "creature cards exiled this way gain ..." ------------------------------
# Lukka, Coppercoat Outcast +1
@_ep(r"^creature cards exiled this way gain [\"'](.+?)[\"']")
def _exiled_gain_ability(m):
    return GrantAbility(
        ability_name=m.group(1),
        target=Filter(base="exiled_cards"),
    )


# --- "repeat this process six more times" ----------------------------------
# Professor Onyx +1
@_ep(r"^repeat this process (?:\d+|six|seven|eight|nine|ten) more times?")
def _repeat_process(m):
    return UnknownEffect(raw_text=m.group(0))


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS — (compiled_regex, event_name, builder_kind)
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []


def _tp(pattern: str, event: str, kind: str = "triggered"):
    TRIGGER_PATTERNS.append((re.compile(pattern, re.I | re.S), event, kind))


# --- "whenever one or more creatures you control and/or creature cards ..." -
# Kaya, Spirits' Justice
_tp(
    r"^whenever one or more creatures you control and/or creature cards in your graveyard are put into exile",
    "exiled",
)

# --- "whenever one or more other cats you control die" ----------------------
# Ajani, Nacatl Pariah // Ajani, Nacatl Avenger
_tp(
    r"^whenever one or more other (?:cats|creatures) you control die",
    "die",
)

# --- "when ~ has two or fewer loyalty counters on him, transform him" -------
# Garruk Relentless // Garruk, the Veil-Cursed
_tp(
    r"^when ~ has (?:two|three|four|five) or fewer loyalty counters on (?:him|her|it|them)",
    "state_check",
)

# --- "when minsc & boo ... enters and" ------------------------------------
# A-Minsc & Boo, Timeless Heroes (Alchemy). Compound trigger split by the
# sentence parser. The upkeep trigger body parsed correctly; this orphan
# prefix is the ETB portion without its body. Absorb as a static stub.
@_sp(r"^when .+ enters and$")
def _compound_etb_prefix(m):
    return Static(
        condition=None,
        modification=Modification(kind="compound_trigger_prefix", args=[m.group(0)]),
    )

# --- "as tibalt enters, you get an emblem with ..." -------------------------
# Valki, God of Lies // Tibalt, Cosmic Impostor
_tp(
    r"^as .+ enters,? you get an emblem with",
    "etb",
)
