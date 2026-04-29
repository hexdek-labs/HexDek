#!/usr/bin/env python3
"""Combat-math static/restriction/replacement family.

Family: COMBAT MATH EDGE CASES — the rules-text shapes that govern HOW a
creature participates in combat beyond bare attack/block/damage triggers.
Specifically:

  * Block restrictions (``~ can't be blocked except by N or more``,
    ``~ can't be blocked except by <filter>``, ``~ can't be blocked by
    more than one creature``, ``~ can block any number of creatures``,
    ``~ can block an additional ...``, ``~ can block only ...``)
  * Block compulsions (``~ must be blocked if able``, ``all creatures
    able to block ~ do [so]`` — Lure / Breaker of Armies family)
  * Attack restrictions / compulsions (``~ attacks each combat if able``,
    ``~ attacks and blocks each combat if able``, ``~ can't attack``,
    ``~ can't attack or block alone``)
  * Damage-assignment overrides (``~ assigns its combat damage as though
    it weren't blocked`` — trample's expanded form; ``~ assigns combat
    damage equal to its toughness rather than its power`` — Baldin /
    Ancient Lumberknot)
  * Damage-assignment replacement (``if ~ would deal combat damage, it
    deals double that damage instead`` — Furnace of Rath / Charging
    Tuskodon) and source-side doublers
  * Defensive trample grants (``~ has trample as long as ...``,
    ``~ has trample while blocking``)
  * Global attacker/blocker anthems (``attacking creatures get +N/+N``,
    ``blocking creatures get -N/-0``)
  * Defending-player restrictions (``defending player can't cast spells``)
  * Opponent attack-scaling triggers (``whenever two or more creatures
    your opponents control attack``)
  * "Block by N or more" scaling triggers (``if N or more creatures
    block ~, ...`` — Triumph-of-the-Hordes-scaling family)

Exported tables — shape matches parser merge points (see
``damage_prevention.py`` for the same pattern):

  - ``STATIC_PATTERNS = [(compiled_regex, builder_fn), ...]``
  - ``EFFECT_RULES    = [(compiled_regex, builder_fn), ...]``
  - ``TRIGGER_PATTERNS = [(compiled_regex, event_name, scope), ...]``

Builders receive ``(m, raw)`` for STATIC_PATTERNS (or just ``(m,)`` —
the parser tolerates both) and ``(m,)`` for EFFECT_RULES, consistent
with the convention already used in ``damage_prevention.py``.

Ordering rule: most specific first. The parser's first-match-wins loop
depends on this (e.g. ``can't be blocked except by two or more creatures``
must match before the generic ``can't be blocked except by <filter>``).

``~`` is already normalised in oracle text by ``parser.normalize()``
before any of these regexes see the string.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Buff, Damage, Filter, Modification, Replacement, Static, UnknownEffect,
    TARGET_CREATURE,
)


# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}

# Self-aliases (mirrors the convention in combat_triggers.py).
_SELF = (
    r"(?:~|this creature|this permanent|this vehicle|this artifact|"
    r"this enchantment|this card|equipped creature|enchanted creature)"
)


def _num(tok: str):
    tok = tok.strip().lower()
    if tok.isdigit():
        return int(tok)
    return _NUM_WORDS.get(tok, tok)


def _parse_filter_safe(text: str) -> Filter:
    """Lazy import of parser.parse_filter; fall back to a bare Filter."""
    try:
        from parser import parse_filter  # type: ignore
        f = parse_filter(text)
        if f is not None:
            return f
    except Exception:
        pass
    head = (text.strip().split() or ["thing"])[0]
    return Filter(base=head, targeted=False)


def _static(kind: str, args: tuple, raw: str) -> Static:
    return Static(modification=Modification(kind=kind, args=args), raw=raw)


# ===========================================================================
# STATIC_PATTERNS — always-on combat-math rules
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _static_pattern(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# Block restrictions
# ---------------------------------------------------------------------------

# "~ can't be blocked except by N or more creatures" (Yargle, Harald's
# Menace reminder-text body, Trygon-Predator-like phrasings). Must come
# before the generic "except by <filter>".
@_static_pattern(
    rf"^{_SELF} can'?t be blocked except by (\d+|two|three|four|five|six|seven|eight|nine|ten|x) or more creatures$"
)
def _cant_be_blocked_except_n_or_more(m, raw):
    n = _num(m.group(1))
    return _static("block_restriction_min_blockers", (n,), raw)


# "~ can't be blocked except by [filter]" (Storm Crow, walls-only, etc.)
@_static_pattern(
    rf"^{_SELF} can'?t be blocked except by ([^.]+)$"
)
def _cant_be_blocked_except_by(m, raw):
    flt = _parse_filter_safe(m.group(1))
    return _static("block_restriction_filter", (flt,), raw)


# "~ can't be blocked by more than one creature" (Menace-base-case,
# Foretold Soldier bottom line).
@_static_pattern(
    rf"^{_SELF} can'?t be blocked by more than one creature$"
)
def _cant_be_blocked_by_more_than_one(m, raw):
    return _static("block_restriction_max_one", (), raw)


# "~ can't be blocked by creatures with power N or less/greater" (Skulk,
# Intimidate-adjacent).
@_static_pattern(
    rf"^{_SELF} can'?t be blocked by creatures with power (\d+) (or (?:less|greater|more))$"
)
def _cant_be_blocked_by_power(m, raw):
    n = int(m.group(1))
    cmp_ = m.group(2).lower()
    return _static("block_restriction_power", (n, cmp_), raw)


# "~ can't be blocked and can't block" — combined isolation.
@_static_pattern(
    rf"^{_SELF} can'?t block and can'?t be blocked$"
)
def _cant_block_or_be_blocked(m, raw):
    return _static("combat_isolated", (), raw)


# ---------------------------------------------------------------------------
# Must-be-blocked / Lure effects
# ---------------------------------------------------------------------------

# "~ must be blocked if able" (Foretold Soldier, Riveteers Decoy).
@_static_pattern(
    rf"^{_SELF} must be blocked if able$"
)
def _must_be_blocked(m, raw):
    return _static("must_be_blocked", (), raw)


# "Each <filter> must be blocked if able" / "each creature you control
# must be blocked if able" (Sisters of Stone Death Avatar).
@_static_pattern(
    r"^each ([^.]+?) must be blocked if able$"
)
def _each_x_must_be_blocked(m, raw):
    flt = _parse_filter_safe(m.group(1))
    return _static("must_be_blocked_filter", (flt,), raw)


# "All creatures able to block ~ do [so]" (Lure, Breaker of Armies,
# Elvish Bard, Nessian Boar).
@_static_pattern(
    rf"^all creatures able to block {_SELF} do(?: so)?$"
)
def _lure(m, raw):
    return _static("lure_self", (), raw)


# Same but targeting an external named object (Lure on an enchanted
# creature resolves via normalize() to ~; this branch picks up the rare
# "all creatures able to block <filter>" variant).
@_static_pattern(
    r"^all creatures able to block ([^.]+?) do(?: so)?$"
)
def _lure_filter(m, raw):
    flt = _parse_filter_safe(m.group(1))
    return _static("lure_filter", (flt,), raw)


# ---------------------------------------------------------------------------
# Extra-blocks / flexible-blockers
# ---------------------------------------------------------------------------

# "~ can block any number of creatures" (Entangler's enchanted creature
# gets this — but after normalize it reads as ~).
@_static_pattern(
    rf"^{_SELF} can block any number of creatures$"
)
def _block_any_number(m, raw):
    return _static("block_any_number", (), raw)


# "~ can block an additional creature (each combat|this turn)"
@_static_pattern(
    rf"^{_SELF} can block an additional (?:(\d+|two|three|four) )?creatures?(?: each combat| this turn)?$"
)
def _block_additional(m, raw):
    n = _num(m.group(1)) if m.group(1) else 1
    return _static("block_additional", (n,), raw)


# "~ can block only creatures with [filter]" — already handled in core
# parser as a bare test, but we add a scoped variant that extracts the
# filter so downstream rules-engine queries work.
@_static_pattern(
    rf"^{_SELF} can block only creatures with ([^.]+)$"
)
def _block_only_with(m, raw):
    flt = _parse_filter_safe(m.group(1))
    return _static("block_only_with", (flt,), raw)


# ---------------------------------------------------------------------------
# Attack restrictions / compulsions
# ---------------------------------------------------------------------------

# "~ attacks each combat if able" — mirror of blocks_each_combat in
# combat_triggers.py. Core parser handles some of this but not the
# attack-and-blocks compound.
@_static_pattern(
    rf"^{_SELF} attacks each (?:combat|turn) if able$"
)
def _attacks_each_combat(m, raw):
    return _static("attacks_each_combat", (), raw)


# "~ attacks and blocks each combat if able" — compound (Ulamog's
# Reclaimer-adjacent, some must-attack-and-block creatures).
@_static_pattern(
    rf"^{_SELF} attacks and blocks each (?:combat|turn) if able$"
)
def _attacks_and_blocks_each(m, raw):
    return _static("attacks_and_blocks_each", (), raw)


# "~ attacks or blocks each combat if able" (Khârn the Betrayer rider).
@_static_pattern(
    rf"^{_SELF} attacks or blocks each (?:combat|turn) if able$"
)
def _attacks_or_blocks_each(m, raw):
    return _static("attacks_or_blocks_each", (), raw)


# "~ blocks each combat if able" (Razorgrass Screen).
@_static_pattern(
    rf"^{_SELF} blocks each (?:combat|turn) if able$"
)
def _blocks_each_combat(m, raw):
    return _static("blocks_each_combat", (), raw)


# ---------------------------------------------------------------------------
# Damage-assignment overrides
# ---------------------------------------------------------------------------

# Trample's expanded form: "~ assigns its combat damage as though it
# weren't blocked". Also: "enchanted creature's controller may have it
# assign its combat damage as though it weren't blocked" (Indomitable
# Might body — may-form, same replacement).
@_static_pattern(
    rf"^{_SELF} assigns its combat damage as though it weren'?t blocked$"
)
def _assigns_as_unblocked(m, raw):
    repl = Replacement(
        trigger_event="assign_combat_damage",
        replacement=UnknownEffect(raw_text="assign as unblocked"),
    )
    return _static("replacement_static", (repl, raw, "assign_as_unblocked"), raw)


@_static_pattern(
    r"^(?:~'s? controller|enchanted creature'?s? controller|its controller) may have it assign its combat damage as though it weren'?t blocked$"
)
def _controller_may_assign_as_unblocked(m, raw):
    repl = Replacement(
        trigger_event="assign_combat_damage",
        replacement=UnknownEffect(raw_text="may assign as unblocked"),
    )
    return _static("replacement_static", (repl, raw, "may_assign_as_unblocked"), raw)


# "during your turn, each creature assigns combat damage equal to its
# toughness rather than its power" — Baldin global.
@_static_pattern(
    r"^during your turn, each creature assigns combat damage equal to its toughness rather than its power$"
)
def _global_toughness_damage_yours(m, raw):
    repl = Replacement(
        trigger_event="assign_combat_damage",
        replacement=UnknownEffect(raw_text="use toughness as power, your turn only"),
    )
    return _static("replacement_static", (repl, raw, "toughness_damage_your_turn"), raw)


# "each creature you control with toughness greater than its power
# assigns combat damage equal to its toughness rather than its power"
# — Ancient Lumberknot.
@_static_pattern(
    r"^each creature you control with toughness greater than its power assigns combat damage equal to its toughness rather than its power$"
)
def _toughness_damage_scoped(m, raw):
    repl = Replacement(
        trigger_event="assign_combat_damage",
        replacement=UnknownEffect(raw_text="use toughness as power, scoped"),
    )
    return _static("replacement_static", (repl, raw, "toughness_damage_scoped"), raw)


# "~ assigns combat damage equal to its toughness rather than its power"
# — self form.
@_static_pattern(
    rf"^{_SELF} assigns combat damage equal to its toughness rather than its power$"
)
def _self_toughness_damage(m, raw):
    repl = Replacement(
        trigger_event="assign_combat_damage",
        replacement=UnknownEffect(raw_text="use toughness as power, self"),
    )
    return _static("replacement_static", (repl, raw, "toughness_damage_self"), raw)


# "~ deals combat damage equal to its toughness rather than its power"
# — alternate wording (Strange Inversion flavor, some modern printings).
@_static_pattern(
    rf"^{_SELF} deals combat damage equal to its toughness rather than its power$"
)
def _self_deals_toughness(m, raw):
    repl = Replacement(
        trigger_event="deal_combat_damage",
        replacement=UnknownEffect(raw_text="deal toughness instead of power"),
    )
    return _static("replacement_static", (repl, raw, "deal_toughness_as_power"), raw)


# ---------------------------------------------------------------------------
# Damage doublers (Furnace of Rath family)
# ---------------------------------------------------------------------------

# Global: "If a source would deal damage to a permanent or player, it
# deals double that damage to that permanent or player instead."
# (Furnace of Rath). damage_prevention.py has the halving dual; this
# captures the doubling dual in its Furnace-global form.
@_static_pattern(
    r"^if a source would deal damage to a permanent or player, it deals double that damage to that permanent or player instead$"
)
def _furnace_of_rath(m, raw):
    repl = Replacement(
        trigger_event="would_deal_damage_any",
        replacement=Damage(amount="double", target=Filter(base="same", targeted=False)),
    )
    return _static("replacement_static", (repl, raw, "global_damage_double"), raw)


# Self-doubler: "if ~ would deal (combat) damage to <filter>, it deals
# double that damage to <same> instead." (Charging Tuskodon combat-only,
# Gratuitous Violence variants).
@_static_pattern(
    rf"^if {_SELF} would deal (combat )?damage to ([^,.]+?), it deals double that damage to (?:that (?:player|permanent|creature)|[^.]+?) instead$"
)
def _self_damage_doubler(m, raw):
    combat_only = bool(m.group(1))
    tgt = _parse_filter_safe(m.group(2))
    evt = "would_deal_combat_damage" if combat_only else "would_deal_damage"
    repl = Replacement(
        trigger_event=evt,
        replacement=Damage(amount="double", target=tgt),
    )
    return _static(
        "replacement_static",
        (repl, raw, "self_damage_double", tgt, "combat" if combat_only else "any"),
        raw,
    )


# "If enchanted creature would deal combat damage to a permanent or
# player, it deals double that damage instead" (The Sound of Drums).
@_static_pattern(
    r"^if (?:enchanted|equipped) creature would deal (combat )?damage to a permanent or player, it deals double that damage instead$"
)
def _enchanted_damage_doubler(m, raw):
    combat_only = bool(m.group(1))
    evt = "would_deal_combat_damage" if combat_only else "would_deal_damage"
    repl = Replacement(
        trigger_event=evt,
        replacement=Damage(amount="double", target=Filter(base="same", targeted=False)),
    )
    return _static(
        "replacement_static",
        (repl, raw, "enchanted_damage_double", "combat" if combat_only else "any"),
        raw,
    )


# ---------------------------------------------------------------------------
# Defensive trample / conditional keyword grants
# ---------------------------------------------------------------------------

# "~ has trample as long as <condition>" / "while <condition>".
# Primordial Hydra, Grand Ball Guest, Earth Rumble Wrestlers.
@_static_pattern(
    rf"^{_SELF} has trample as long as ([^.]+)$"
)
def _conditional_trample(m, raw):
    cond = m.group(1).strip()
    return _static("conditional_keyword", ("trample", cond), raw)


# Defensive trample: "~ has trample while blocking [filter]" (rare, but
# some judge-gotcha cards have this — include for completeness).
@_static_pattern(
    rf"^{_SELF} has trample (?:while|when|if it'?s?) blocking(?: ([^.]+))?$"
)
def _trample_while_blocking(m, raw):
    scope = m.group(1).strip() if m.group(1) else "any"
    return _static("conditional_keyword", ("trample", f"blocking:{scope}"), raw)


# Generic "~ has <keyword> while blocking/attacking" — captures defensive
# first-strike, vigilance-while-attacking, etc. One rule covers the family.
@_static_pattern(
    rf"^{_SELF} has ([a-z ,]+?) (?:while|when|as long as it'?s?) (attacking|blocking|attacking or blocking)$"
)
def _conditional_kw_combat(m, raw):
    kws = m.group(1).strip()
    scope = m.group(2).strip()
    return _static("conditional_keyword", (kws, f"combat:{scope}"), raw)


# ---------------------------------------------------------------------------
# Global attacker/blocker anthems
# ---------------------------------------------------------------------------

# "Attacking creatures get +N/+N" (Orcish Oriflamme-style anthem on
# attackers). Signed ±N/±N so Weakstone (-1/-0) works too.
@_static_pattern(
    r"^attacking creatures(?: you control)? get ([+\-]\d+)/([+\-]\d+)$"
)
def _attackers_anthem(m, raw):
    p = int(m.group(1))
    t = int(m.group(2))
    return _static("anthem_attackers", (p, t), raw)


# "Attacking creatures get -N/-0" — non-you-control (Weakstone: applies
# to all attackers).
@_static_pattern(
    r"^attacking creatures get ([+\-]\d+)/([+\-]\d+)$"
)
def _attackers_anthem_global(m, raw):
    p = int(m.group(1))
    t = int(m.group(2))
    return _static("anthem_attackers_global", (p, t), raw)


# "Blocking creatures get +N/+N"
@_static_pattern(
    r"^blocking creatures(?: you control)? get ([+\-]\d+)/([+\-]\d+)$"
)
def _blockers_anthem(m, raw):
    p = int(m.group(1))
    t = int(m.group(2))
    return _static("anthem_blockers", (p, t), raw)


# "Attacking and blocking creatures get +N/+N" (rare — combined).
@_static_pattern(
    r"^attacking and blocking creatures(?: you control)? get ([+\-]\d+)/([+\-]\d+)$"
)
def _attackers_blockers_anthem(m, raw):
    p = int(m.group(1))
    t = int(m.group(2))
    return _static("anthem_attackers_blockers", (p, t), raw)


# ---------------------------------------------------------------------------
# Defending-player restrictions
# ---------------------------------------------------------------------------

# "As long as ~ is attacking, defending player can't cast spells"
# (Wardscale Dragon static form).
@_static_pattern(
    rf"^as long as {_SELF} is attacking, defending player can'?t ([^.]+)$"
)
def _defending_player_cant_static(m, raw):
    action = m.group(1).strip()
    return _static("defending_player_cant", (action,), raw)


# "Defending player can't cast spells" — bare static on rare cards
# (Wydwen-adjacent). We include this as a standalone static; combat-
# triggered "whenever ~ attacks, defending player can't cast spells"
# is handled as an effect rule below.
@_static_pattern(
    r"^defending player can'?t ([^.]+)$"
)
def _defending_cant_bare(m, raw):
    action = m.group(1).strip()
    return _static("defending_player_cant", (action,), raw)


# ---------------------------------------------------------------------------
# Blocker-count scaling (Triumph of the Hordes family)
# ---------------------------------------------------------------------------

# "If N or more creatures block ~, <effect>" — Triumph-scaling static.
# Emitted as an unknown effect since the effect clause varies; the
# downstream engine consumes the sentinel.
@_static_pattern(
    rf"^if (\d+|two|three|four|five) or more creatures block {_SELF}, (.+)$"
)
def _if_n_or_more_block(m, raw):
    n = _num(m.group(1))
    effect_tail = m.group(2).strip()
    return _static("if_n_or_more_block", (n, effect_tail), raw)


# ===========================================================================
# EFFECT_RULES — one-shot combat-math effects (sentence bodies of
# spell / activated / triggered abilities).
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _effect_rule(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# "Attacking creatures get +N/+N until end of turn" — common instant /
# activated-ability body (Thunderstaff activated, overrun-cycle lites).
@_effect_rule(
    r"^attacking creatures(?: you control)? get ([+\-]\d+)/([+\-]\d+) until end of turn$"
)
def _eff_attackers_buff(m):
    p = int(m.group(1))
    t = int(m.group(2))
    return Buff(
        power=p, toughness=t,
        target=Filter(base="attacking_creatures", targeted=False),
        duration="until_end_of_turn",
    )


# "Blocking creatures get ±N/±N until end of turn"
@_effect_rule(
    r"^blocking creatures(?: you control)? get ([+\-]\d+)/([+\-]\d+) until end of turn$"
)
def _eff_blockers_buff(m):
    p = int(m.group(1))
    t = int(m.group(2))
    return Buff(
        power=p, toughness=t,
        target=Filter(base="blocking_creatures", targeted=False),
        duration="until_end_of_turn",
    )


# "Defending player can't cast spells this turn." — body of an ability
# whose trigger fires on attack (Xantid Swarm-style).
@_effect_rule(
    r"^defending player can'?t ([^.]+?)(?: this turn)?$"
)
def _eff_defending_cant(m):
    action = m.group(1).strip()
    return UnknownEffect(raw_text=f"defending player can't {action}")


# "<actor> must block <target> this turn if able" — one-shot Lure
# (single-target form; rare, but appears on some cards).
@_effect_rule(
    r"^(.+?) must block ([^.]+?) (?:this turn )?if able(?: this turn)?$"
)
def _eff_must_block_target(m):
    return UnknownEffect(
        raw_text=f"must block: {m.group(1).strip()} -> {m.group(2).strip()}"
    )


# "Up to N target creatures must block ~ this turn if able" (Provoke-like
# effect body).
@_effect_rule(
    rf"^up to (\d+|one|two|three) target creatures? must block {_SELF}(?: this turn)? if able$"
)
def _eff_up_to_n_must_block(m):
    n = _num(m.group(1))
    return UnknownEffect(raw_text=f"up to {n} must block self")


# ===========================================================================
# TRIGGER_PATTERNS — combat-count aggregate triggers
# ===========================================================================

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # "Whenever N or more creatures your opponents control attack, …"
    # (Flummoxed Cyclops).
    (re.compile(
        r"^whenever (?:two|three|four|five|\d+) or more creatures your opponents control attack\b",
        re.I),
     "opp_multi_attack", "self"),

    # "Whenever one or more creatures block, …" (Tide of War).
    (re.compile(r"^whenever one or more creatures block\b", re.I),
     "any_block", "self"),

    # "Whenever N or more creatures block ~, …" — a triggered variant of
    # the static "if N or more creatures block ~" scaling.
    (re.compile(
        rf"^whenever (?:\d+|two|three|four|five) or more creatures block {_SELF}(?=[,.\s]|$)",
        re.I),
     "n_or_more_block_self", "self"),

    # "Whenever ~ is blocked by two or more creatures, …" — multi-blocker
    # aggregate.
    (re.compile(
        rf"^whenever {_SELF} (?:is|becomes) blocked by (?:two|three|four|five|\d+) or more creatures\b",
        re.I),
     "blocked_by_multi", "self"),
]


__all__ = ["STATIC_PATTERNS", "EFFECT_RULES", "TRIGGER_PATTERNS"]


# ---------------------------------------------------------------------------
# Self-check
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    static_samples = [
        "~ can't be blocked except by two or more creatures",
        "~ can't be blocked except by three or more creatures",
        "~ can't be blocked except by creatures with flying or reach",
        "~ can't be blocked except by walls",
        "~ can't be blocked by more than one creature",
        "~ can't be blocked by creatures with power 2 or less",
        "~ can't block and can't be blocked",
        "~ must be blocked if able",
        "each creature you control must be blocked if able",
        "all creatures able to block ~ do",
        "all creatures able to block ~ do so",
        "all creatures able to block enchanted creature do so",
        "~ can block any number of creatures",
        "~ can block an additional creature each combat",
        "~ can block an additional two creatures",
        "~ can block only creatures with flying",
        "~ attacks each combat if able",
        "~ attacks and blocks each combat if able",
        "~ attacks or blocks each combat if able",
        "~ blocks each combat if able",
        "~ assigns its combat damage as though it weren't blocked",
        "enchanted creature's controller may have it assign its combat damage as though it weren't blocked",
        "during your turn, each creature assigns combat damage equal to its toughness rather than its power",
        "each creature you control with toughness greater than its power assigns combat damage equal to its toughness rather than its power",
        "~ assigns combat damage equal to its toughness rather than its power",
        "~ deals combat damage equal to its toughness rather than its power",
        "if a source would deal damage to a permanent or player, it deals double that damage to that permanent or player instead",
        "if ~ would deal combat damage to a player, it deals double that damage to that player instead",
        "if ~ would deal damage to a creature, it deals double that damage to that creature instead",
        "if enchanted creature would deal combat damage to a permanent or player, it deals double that damage instead",
        "~ has trample as long as it has ten or more +1/+1 counters on it",
        "~ has trample while blocking",
        "~ has first strike while blocking",
        "~ has vigilance while attacking",
        "attacking creatures you control get +1/+0",
        "attacking creatures get -1/-0",
        "blocking creatures you control get +0/+2",
        "attacking and blocking creatures you control get +1/+1",
        "as long as ~ is attacking, defending player can't cast spells",
        "defending player can't cast spells",
        "if three or more creatures block ~, draw three cards",
    ]

    effect_samples = [
        "attacking creatures get +1/+0 until end of turn",
        "attacking creatures you control get +2/+2 until end of turn",
        "blocking creatures get +0/+2 until end of turn",
        "defending player can't cast spells this turn",
        "up to two target creatures must block ~ this turn if able",
    ]

    trigger_samples = [
        "whenever two or more creatures your opponents control attack, this creature can't block this combat",
        "whenever one or more creatures block, flip a coin",
        "whenever three or more creatures block ~, draw three cards",
        "whenever ~ becomes blocked by two or more creatures, it gets +2/+2 until end of turn",
    ]

    def _norm(s):
        return s.strip().rstrip(".").lower()

    def _check_static(s):
        for pat, _ in STATIC_PATTERNS:
            if pat.match(_norm(s)):
                return True
        return False

    def _check_effect(s):
        for pat, _ in EFFECT_RULES:
            if pat.match(_norm(s)):
                return True
        return False

    def _check_trigger(s):
        for pat, _, _ in TRIGGER_PATTERNS:
            if pat.match(_norm(s)):
                return True
        return False

    hs = sum(1 for s in static_samples if _check_static(s))
    he = sum(1 for s in effect_samples if _check_effect(s))
    ht = sum(1 for s in trigger_samples if _check_trigger(s))

    print(f"combat_math.py: {len(STATIC_PATTERNS)} statics, "
          f"{len(EFFECT_RULES)} effects, {len(TRIGGER_PATTERNS)} triggers")
    print(f"  static  sample hits: {hs}/{len(static_samples)}")
    print(f"  effect  sample hits: {he}/{len(effect_samples)}")
    print(f"  trigger sample hits: {ht}/{len(trigger_samples)}")

    for s in static_samples:
        if not _check_static(s):
            print(f"  MISS static : {s}")
    for s in effect_samples:
        if not _check_effect(s):
            print(f"  MISS effect : {s}")
    for s in trigger_samples:
        if not _check_trigger(s):
            print(f"  MISS trigger: {s}")
