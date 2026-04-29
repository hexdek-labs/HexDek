#!/usr/bin/env python3
"""Replacement-effect extensions (comp rules §614).

A replacement effect watches for an event and SUBSTITUTES a different event.
Structural shape:

    "If [event] would happen, [different event] instead."

All of these produce a ``Replacement`` AST node whose ``trigger_event`` string
identifies the event being replaced (so downstream engine code can hook the
right event bus) and whose ``replacement`` field carries the substituted
``Effect`` (a typed leaf where possible, else an ``UnknownEffect``).

Two tables are exported so the parser can merge them the same way it merges
``equipment_aura`` / ``ability_words``:

- ``EFFECT_RULES``: ``(compiled_regex, builder)`` — appended to
  ``parser.EFFECT_RULES``. These match ability BODIES (the effect part of a
  triggered/activated/spell ability). Most replacement spells (Prevent-damage
  instants, redirect effects, one-shot "the next time …" substitutions) live
  here.

- ``STATIC_PATTERNS``: ``(compiled_regex, builder)`` — consulted by
  ``parser.parse_static`` before the ``conditional_static`` catch-all. These
  are the always-on replacement abilities: "If [X] would die, exile it
  instead.", "Damage that would be dealt to ~ is dealt to [target] instead.",
  static ETB-with-counters, static "as ~ enters, choose a color", type-add
  riders ("is [type] in addition to its other types"), and hosers that
  replace the way an event resolves (rather than triggering on it).

Coverage targets (from parser_coverage.md, Apr 2026 snapshot):

    ~250+ cards with "this creature enters with N [counter] on it"
    ~80  cards with "as [permanent] enters, choose a [thing]"
    ~40  cards with "prevent the next N damage …"
    ~30  cards with "if [X] would die, exile it instead" gain-quotes
    ~20  cards with "rather than pay this spell's mana cost"
    ~20  cards with "all damage that would be dealt to you is dealt to …"
    ~15  cards with "is [type] in addition to its other types"

Each STATIC_PATTERNS builder returns a ``Static`` whose ``modification.kind``
is ``'replacement_static'`` (per the Modification docstring in mtg_ast) and
whose ``modification.args`` is a tuple ``(Replacement(...), raw_text)`` —
keeping the full Replacement AST inside the Static is what lets downstream
signature/clustering code reason about the replacement structure without
re-parsing.

The parser does not import this file yet. To wire it in without modifying
``parser.py``, callers (tests, clustering pipelines) can do:

    import parser, extensions.replacements as rep
    parser.EFFECT_RULES.extend(rep.EFFECT_RULES)
    rep.install_static_hook(parser)   # monkey-patches parse_static to
                                       # consult STATIC_PATTERNS first
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

# Allow `import mtg_ast` from scripts/ when this module is loaded directly.
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    CounterMod, Damage, Draw, Effect, Exile, Filter, GainLife, LoseLife,
    Modification, Prevent, Replacement, Static, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
    "half": "half",
}

_COLOR_WORD = r"(?:white|blue|black|red|green|colorless|mono[a-z]*|multicolored)"


def _num(tok: str):
    tok = tok.strip().lower()
    if tok.isdigit():
        return int(tok)
    return _NUM_WORDS.get(tok, tok)


def _mk_static(repl: Replacement, raw: str) -> Static:
    """Wrap a Replacement node in a Static carrying kind='replacement_static'."""
    return Static(
        modification=Modification(kind="replacement_static", args=(repl, raw)),
        raw=raw,
    )


def _parse_filter_safe(text: str) -> Filter:
    """Import parser.parse_filter lazily (avoids a circular at module-load)."""
    try:
        from parser import parse_filter  # type: ignore
        f = parse_filter(text)
        if f is not None:
            return f
    except Exception:
        pass
    return Filter(base=(text.strip().split() or ["thing"])[0], targeted=False)


# ===========================================================================
# EFFECT_RULES — one-shot replacement spells / ability bodies
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _effect_rule(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Prevent next-N damage (Healing Salve, Shieldmate's Blessing, Sacred Boon)
@_effect_rule(r"^prevent the next (\d+|x) damage that (?:a source of your choice )?would be dealt to ([^.]+?)(?:\.|$)")
def _prevent_next_n(m):
    amt = _num(m.group(1))
    tgt = _parse_filter_safe(m.group(2))
    return Prevent(amount=amt, damage_filter=tgt, duration="until_end_of_turn")


# --- "The next N damage that would be dealt to X this turn is prevented"
@_effect_rule(r"^the next (\d+|x) damage that (?:a source of your choice )?would (?:be )?deal(?:t)? to ([^.]+?) (?:this turn |)(?:is prevented|is dealt to [^.]+)(?:[.]|$)")
def _next_n_redirect_or_prevent(m):
    amt = _num(m.group(1))
    tgt = _parse_filter_safe(m.group(2))
    return Prevent(amount=amt, damage_filter=tgt, duration="until_end_of_turn")


# --- Prevent all damage/combat damage a source would deal this turn
@_effect_rule(r"^prevent all (?:combat )?damage (?:that would be dealt )?(?:to|by) ([^.]+?) (?:this turn|this combat)(?:\.|$)")
def _prevent_all_to_target(m):
    tgt = _parse_filter_safe(m.group(1))
    return Prevent(amount="all", damage_filter=tgt, duration="until_end_of_turn")


# --- "damage that [source] would deal this turn is dealt to [target] instead"
@_effect_rule(r"^(?:all )?(?:combat )?damage that would be dealt (?:this turn )?(?:to |by )?([^.]+?) is dealt to ([^.]+?) instead(?:\.|$)")
def _redirect_damage_one_shot(m):
    src = _parse_filter_safe(m.group(1))
    dst = _parse_filter_safe(m.group(2))
    return Replacement(
        trigger_event="deal_damage",
        replacement=Damage(amount="var", target=dst),
    )


# --- "Instead, <effect>" — used as a one-shot replacement rider on spells
#     (Winds of Qal Sisma ferocious branch, Stunning Reversal, etc.)
@_effect_rule(r"^instead (?:prevent|draw|exile|gain|lose|return|sacrifice|create|destroy|counter) ([^.]+?)(?:\.|$)")
def _instead_rider(m):
    return Replacement(
        trigger_event="spell_alt_mode",
        replacement=UnknownEffect(raw_text=m.group(0)),
    )


# --- "If a source would deal damage to you/a permanent you control this turn,
#      prevent N of that damage." (Wall of Reverence-style riders)
@_effect_rule(r"^if (?:a|an) (?:" + _COLOR_WORD + r" )?source would deal damage to ([^,]+?), prevent (\d+|x|that|all) (?:of that )?damage(?:\.|$)")
def _prevent_colored_source_damage(m):
    tgt = _parse_filter_safe(m.group(1))
    amt_raw = m.group(2).lower()
    amt = _num(amt_raw) if amt_raw not in {"that", "all"} else "all"
    return Replacement(
        trigger_event="deal_damage_to_you",
        replacement=Prevent(amount=amt, damage_filter=tgt),
    )


# --- "If you would draw a card, instead [effect]" / "draw N cards instead"
@_effect_rule(r"^if you would draw (?:a card|one or more cards), (?:instead )?([^.]+?)(?: instead)?(?:\.|$)")
def _if_would_draw(m):
    body = m.group(1).strip()
    # Best-effort: recognize "draw two cards" as the substitute
    dm = re.match(r"draw (\d+|two|three|x|one) cards?", body)
    if dm:
        repl = Draw(count=_num(dm.group(1)))
    else:
        repl = UnknownEffect(raw_text=body)
    return Replacement(trigger_event="draw_card", replacement=repl)


# --- "If you would gain life, [you gain that much life plus N / instead …]"
@_effect_rule(r"^if you would gain life, (?:you gain )?(?:that much life plus (\d+)|twice that much life|[^.]+?) instead(?:\.|$)")
def _if_would_gain_life(m):
    plus = m.group(1)
    repl = (GainLife(amount=f"var+{plus}") if plus
            else UnknownEffect(raw_text=m.group(0)))
    return Replacement(trigger_event="gain_life", replacement=repl)


# --- "If an opponent would gain life, that player loses that much life instead"
@_effect_rule(r"^if (?:an opponent|a player|target player) would gain life, (?:that player|they) loses? that much life instead(?:\.|$)")
def _opponent_gain_life_swap(m):
    return Replacement(
        trigger_event="opponent_gain_life",
        replacement=LoseLife(amount="var"),
    )


# --- "If [filter] would die, exile it instead" (as a one-shot body, e.g. a
#     granted ability's quoted body that we end up trying to parse flat)
@_effect_rule(r"^if (?:this (?:creature|permanent|land|artifact|enchantment)|~|it) would die, exile it instead(?:\.|$)")
def _would_die_exile_self(m):
    return Replacement(
        trigger_event="would_die",
        replacement=Exile(target=Filter(base="self", targeted=False)),
    )


@_effect_rule(r"^if (?:this (?:creature|permanent|land|artifact|enchantment)|~|it) would (?:leave the battlefield|be put into a graveyard)(?: from (?:anywhere|the battlefield))?, exile it instead(?: of putting it (?:anywhere|into a graveyard))?(?:\.|$)")
def _would_leave_exile_self(m):
    return Replacement(
        trigger_event="would_leave_battlefield",
        replacement=Exile(target=Filter(base="self", targeted=False)),
    )


# --- "If a [filter] would be put into a graveyard from anywhere, exile it instead"
@_effect_rule(r"^if (?:a|an|each) ([^,.]+?) would (?:die|be put into (?:a|its owner'?s|any) graveyard)(?: from anywhere)?, exile (?:it|them|that card) instead(?:\.|$)")
def _would_die_filter_exile(m):
    target_filter = _parse_filter_safe(m.group(1))
    return Replacement(
        trigger_event="would_die",
        replacement=Exile(target=target_filter),
    )


# --- Alternative-cost replacement: "You may pay [cost] rather than pay this
#     spell's mana cost." (Bringer cycle, Force of Will, Snapback, Gush)
@_effect_rule(r"^(?:once each turn, )?you may (?:pay ([^.]+?)|sacrifice [^.]+?|return [^.]+?|exile [^.]+?|discard [^.]+?) rather than pay (?:this spell'?s mana cost|the mana cost for [^.]+?|~'s mana cost)(?:\.|$)")
def _alt_cost(m):
    return Replacement(
        trigger_event="pay_mana_cost",
        replacement=UnknownEffect(raw_text=m.group(0)),
    )


# --- "You may cast [this spell] without paying its mana cost" — also a
#     replacement of the cost step (Force of Negation-style)
@_effect_rule(r"^you may cast (?:~|this spell|that card|it) without paying its mana cost(?:\.|$)")
def _cast_without_paying(m):
    return Replacement(
        trigger_event="pay_mana_cost",
        replacement=UnknownEffect(raw_text=m.group(0)),
    )


# ===========================================================================
# STATIC_PATTERNS — always-on replacement abilities
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _static_pattern(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Self-ETB with N <kind> counters -------------------------------------
#     Covers "this creature enters with two +1/+1 counters on it",
#     "~ enters with four +1/+1 counters on it",
#     "this artifact enters with three charge counters on it",
#     "this creature enters with an oil counter on it",
#     "this creature enters with a -1/-1 counter on it",
#     and the "on it if [cond] / for each [thing] / equal to [expr]" tails
#     (which we absorb as an extra arg so a single pattern handles the family).
@_static_pattern(
    r"^(?:~|this (?:creature|permanent|land|artifact|enchantment|planeswalker|token))"
    r" enters with (a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+|a number of|half that many)"
    r" ([+\-]\d+/[+\-]\d+|[a-z][a-z\- ]*?) counters? on (?:it|them)"
    r"(?: (?:for each [^.]+|equal to [^.]+|if [^.]+|rounded [^.]+))?\.?$"
)
def _etb_counters(m, raw):
    count_tok = m.group(1).lower()
    if count_tok in {"a number of", "half that many"}:
        count = "var"
    else:
        count = _num(count_tok)
    kind = m.group(2).strip().lower()
    repl = Replacement(
        trigger_event="self_etb",
        replacement=CounterMod(
            op="put",
            count=count,
            counter_kind=kind,
            target=Filter(base="self", targeted=False),
        ),
    )
    return _mk_static(repl, raw)


# --- "As [permanent] enters, choose a [thing]" ---------------------------
#     (Metallic Mimic, Icon of Ancestry, Unclaimed Territory, Utopia Sprawl,
#     Coalition Construct, Throne of Eldraine, True-Name Nemesis, Nyxathid.)
@_static_pattern(
    r"^as (?:~|this (?:creature|permanent|land|artifact|enchantment|aura|token)|it) enters,"
    r" (?:you (?:may )?)?choose (?:a|an|one|two) ([^.]+?)\.?$"
)
def _as_enters_choose(m, raw):
    what = m.group(1).strip()
    repl = Replacement(
        trigger_event="self_etb",
        replacement=UnknownEffect(raw_text=f"choose {what}"),
    )
    return _mk_static(repl, raw)


# --- Shock-land pay-life-or-tapped: "As this land enters, you may pay 2
#     life. If you don't, it enters tapped." -------------------------------
@_static_pattern(
    r"^as (?:~|this land|this aura) enters, you may pay (\d+) life\.(?: if you don'?t, (?:it|this land) enters tapped\.?)?$"
)
def _shockland(m, raw):
    life = int(m.group(1))
    repl = Replacement(
        trigger_event="self_etb",
        replacement=UnknownEffect(raw_text=f"may pay {life} life else enters tapped"),
    )
    return _mk_static(repl, raw)


# --- Damage-redirect aura/static: "All damage that would be dealt to [X]
#     is dealt to [Y] instead." (Pariah, Pariah's Shield, Empyrial Archangel,
#     Treacherous Link, Sivvi's Valor limited.) ------------------------------
@_static_pattern(
    r"^all (?:combat )?damage that would be dealt(?: this turn)? to ([^.]+?) is dealt to ([^.]+?) instead\.?$"
)
def _redirect_static(m, raw):
    src = _parse_filter_safe(m.group(1))
    dst = _parse_filter_safe(m.group(2))
    repl = Replacement(
        trigger_event="deal_damage_to_permanent",
        replacement=Damage(amount="var", target=dst),
    )
    return _mk_static(repl, raw)


# --- "Damage that would reduce your life total to less than 1 reduces it
#     to 1 instead." (Ali from Cairo, Angel's Grace, Worship-ish.) ---------
@_static_pattern(
    r"^(?:until end of turn, )?damage that would reduce your life total to less than (\d+) reduces it to \1 instead\.?$"
)
def _life_floor(m, raw):
    floor = int(m.group(1))
    repl = Replacement(
        trigger_event="life_would_go_below",
        replacement=UnknownEffect(raw_text=f"floor life at {floor}"),
    )
    return _mk_static(repl, raw)


# --- Type-add rider: "[thing] is [type] in addition to its/their other
#     types." (Arcane Adaptation, Metallic Mimic rider, Leyline of the
#     Guildpact, Phyrexian Scriptures, Portal to Phyrexia, Abuelo's
#     Awakening, Ensoul Artifact continuous "has base P/T"). --------------
@_static_pattern(
    r"^(?:~|this creature|this permanent|that (?:creature|land|permanent)|enchanted (?:creature|artifact|permanent|land)|creatures you control|lands you control|it'?s?|it) (?:is|are|a|an) (?:(?:a|an|every) )?([^.]+?) in addition to (?:its|their) other (?:types|colors)\.?$"
)
def _type_add(m, raw):
    type_text = m.group(1).strip()
    repl = Replacement(
        trigger_event="type_query",
        replacement=UnknownEffect(raw_text=f"add type: {type_text}"),
    )
    return Static(
        modification=Modification(kind="replacement_static",
                                  args=(repl, raw, "type_add")),
        raw=raw,
    )


# --- "Cards in [zone] count as [type] in addition to their other types."
#     (Dralnu-style, Haunted Crossroads cycles, rare graveyard matters.) ---
@_static_pattern(
    r"^cards? in (?:all )?(?:graveyards?|exile|your hand|your library) (?:count as|are) ([^.]+?) in addition to (?:its|their) other types\.?$"
)
def _cards_count_as(m, raw):
    type_text = m.group(1).strip()
    repl = Replacement(
        trigger_event="type_query_zone",
        replacement=UnknownEffect(raw_text=f"zone type: {type_text}"),
    )
    return Static(
        modification=Modification(kind="replacement_static",
                                  args=(repl, raw, "zone_type_add")),
        raw=raw,
    )


# --- Static "if [filter] would die, exile it instead" on a permanent ------
#     (Leyline of the Void-shaped hosers, Grafdigger's Cage's cousins.) ----
@_static_pattern(
    r"^if (?:a|an|each) ([^,.]+?) would (?:die|be put into (?:a|its owner'?s|any) graveyard)(?: from anywhere)?, exile (?:it|them|that card) instead\.?$"
)
def _static_would_die_exile(m, raw):
    who = _parse_filter_safe(m.group(1))
    repl = Replacement(
        trigger_event="would_die",
        replacement=Exile(target=who),
    )
    return _mk_static(repl, raw)


# --- Self-static "if ~ would die / leave / be put into a graveyard, exile
#     it instead" when it appears as its own sentence (Mistveil Plains, Dark
#     Depths-adjacent, some legendary creatures). -------------------------
@_static_pattern(
    r"^if (?:~|this (?:creature|permanent|land|artifact|enchantment)) would"
    r" (?:die|leave the battlefield|be put into (?:a|its owner'?s) graveyard(?: from anywhere)?),"
    r" (?:exile it|(?:its owner |)shuffles? it into (?:their|its owner'?s) library|put it on top of its owner'?s library)"
    r" instead(?: of putting it (?:anywhere|into a graveyard))?\.?$"
)
def _self_static_would_die(m, raw):
    repl = Replacement(
        trigger_event="self_would_die",
        replacement=Exile(target=Filter(base="self", targeted=False)),
    )
    return _mk_static(repl, raw)


# --- Hoser-style search replacement (Aven Mindcensor, Opposition Agent,
#     Stranglehold): "If [player(s)] would search a library, they search
#     the top N cards of that library instead." / "If an opponent would
#     search a library, ~ searches that library instead." ----------------
@_static_pattern(
    r"^if (?:an opponent|a player|your opponents|target opponent|any player) would search a library,"
    r" (?:they|that player|~ searches that library|you search that library)[^.]+?instead\.?$"
)
def _search_replacement(m, raw):
    repl = Replacement(
        trigger_event="search_library",
        replacement=UnknownEffect(raw_text=raw),
    )
    return _mk_static(repl, raw)


# --- "Players can't search libraries" / "Players can't draw more than one
#     card each turn" — prohibition replacements (Stranglehold, Arcane
#     Lab-rule hosers). NOTE: these are technically continuous effects that
#     REPLACE drawing/searching with nothing. ------------------------------
@_static_pattern(
    r"^(?:your opponents|opponents|players|each player) can'?t"
    r" (?:search libraries|draw more than (?:one|\d+) cards? each turn|gain life|win the game|lose the game)(?:\.|$)"
)
def _prohibition_replacement(m, raw):
    repl = Replacement(
        trigger_event="player_action_prohibited",
        replacement=UnknownEffect(raw_text=raw),
    )
    return _mk_static(repl, raw)


# --- Sphere-of-Resistance cost-up: "Spells cost {N} more to cast." This is
#     a replacement of the cost-assignment step. --------------------------
@_static_pattern(
    r"^(?:spells|creature spells|noncreature spells|[^.]+? spells)(?: your opponents cast)? cost \{(\d+)\} more to cast\.?$"
)
def _cost_up(m, raw):
    n = int(m.group(1))
    repl = Replacement(
        trigger_event="determine_cost",
        replacement=UnknownEffect(raw_text=f"add generic {n} to cost"),
    )
    return _mk_static(repl, raw)


# --- Copy-entry replacement: "You may have ~ enter as a copy of any
#     creature on the battlefield." (Clone, Spark Double, Pirated Copy.) --
@_static_pattern(
    r"^you may have (?:~|this creature) enter as a copy of (?:any|a) ([^.]+?)\.?$"
)
def _enter_as_copy(m, raw):
    what = m.group(1).strip()
    repl = Replacement(
        trigger_event="self_etb",
        replacement=UnknownEffect(raw_text=f"enter as copy of {what}"),
    )
    return _mk_static(repl, raw)


# ---------------------------------------------------------------------------
# Integration helper
# ---------------------------------------------------------------------------

def install_static_hook(parser_module) -> None:
    """Monkey-patch parser.parse_static so it consults STATIC_PATTERNS
    BEFORE its ``conditional_static`` catch-all. Safe to call more than once;
    re-installation is idempotent.

    Usage (from a test or clustering driver):

        import parser
        import extensions.replacements as rep
        parser.EFFECT_RULES.extend(rep.EFFECT_RULES)
        rep.install_static_hook(parser)

    We do NOT modify parser.py itself per the module-owner contract.
    """
    if getattr(parser_module, "_replacements_hook_installed", False):
        return
    original_parse_static = parser_module.parse_static

    def patched_parse_static(text):
        s = text.strip().rstrip(".").lower()
        for pat, builder in STATIC_PATTERNS:
            m = pat.match(s)
            if not m:
                continue
            try:
                out = builder(m, text)
            except Exception:
                continue
            if out is not None:
                return out
        return original_parse_static(text)

    parser_module.parse_static = patched_parse_static
    parser_module._replacements_hook_installed = True


# ---------------------------------------------------------------------------
# Self-check
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    # Compile smoke-test — ensures every regex is valid.
    samples_effect = [
        "prevent the next 3 damage that would be dealt to any target",
        "prevent the next 1 damage that would be dealt to any target",
        "the next 2 damage that a source of your choice would deal to any target this turn is prevented",
        "if a source would deal damage to you, prevent 2 of that damage",
        "if you would draw a card, draw two cards instead",
        "if you would gain life, you gain that much life plus 1 instead",
        "if this creature would die, exile it instead",
        "if a creature would die, exile it instead",
        "you may pay {w}{u}{b}{r}{g} rather than pay this spell's mana cost",
        "you may cast ~ without paying its mana cost",
    ]
    samples_static = [
        "this creature enters with two +1/+1 counters on it",
        "~ enters with x +1/+1 counters on it",
        "this creature enters with a -1/-1 counter on it",
        "this artifact enters with three charge counters on it",
        "this creature enters with an oil counter on it",
        "as this creature enters, choose a creature type",
        "as ~ enters, choose a color",
        "as this land enters, you may pay 2 life. if you don't, it enters tapped",
        "all damage that would be dealt to you is dealt to this creature instead",
        "damage that would reduce your life total to less than 1 reduces it to 1 instead",
        "creatures you control are the chosen type in addition to their other types",
        "it's a phyrexian in addition to its other types",
        "if a creature would die, exile it instead",
        "if this creature would leave the battlefield, exile it instead",
        "spells your opponents cast cost {1} more to cast",
        "you may have ~ enter as a copy of any creature on the battlefield",
    ]
    hit_e = sum(1 for s in samples_effect
                for pat, _ in EFFECT_RULES if pat.match(s))
    hit_s = sum(1 for s in samples_static
                for pat, _ in STATIC_PATTERNS if pat.match(s))
    print(f"replacements.py: {len(EFFECT_RULES)} effect rules, "
          f"{len(STATIC_PATTERNS)} static patterns")
    print(f"  effect  sample hits: {hit_e}/{len(samples_effect)}")
    print(f"  static  sample hits: {hit_s}/{len(samples_static)}")
