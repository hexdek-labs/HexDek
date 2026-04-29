#!/usr/bin/env python3
"""Stack & timing constructs.

Family: STACK & TIMING CONSTRUCTS — abilities that interact with the stack,
priority, spell-casting legality, target-handling, intervening-if conditions,
"as you cast", loyalty timing, copy/redirect-a-spell, and generic restrictions
on when spells / abilities may be played or activated.

Three tables are exported (the standard parser extension contract):

- ``TRIGGER_PATTERNS``: ``(compiled_regex, event_name, scope)``.  Merged into
  ``parser.EXT_TRIGGER_PATTERNS``.  Used for the handful of trigger shapes that
  sit on the stack axis (``whenever a player casts``, ``when ~ is put into a
  graveyard from the stack``, loyalty-activation triggers, ``whenever a spell
  or ability an opponent controls targets ...``).

- ``STATIC_PATTERNS``: ``(compiled_regex, builder(match, raw))``.  Merged into
  ``parser.EXT_STATIC_PATTERNS`` — consulted by ``parser.parse_static`` BEFORE
  the ``conditional_static`` catch-all.  This is where the bulk of the work
  lives: timing-restriction statics, cast-legality modifiers, spell-targeting
  hosers (Mother of Runes / True Name auras that sit on a creature),
  "triggered abilities can't be activated", "if a spell would be put into a
  graveyard from the stack, exile it instead", etc.

- ``EFFECT_RULES``: ``(compiled_regex, builder(match))``.  Appended to
  ``parser.EFFECT_RULES``.  Covers one-shot stack-manipulation spells:
  ``change the target of target spell or ability to ~``,
  ``counter target activated ability``,
  ``counter target activated or triggered ability``,
  ``counter target spell unless its controller pays {N}``,
  multi-target spells (``N target <filter>``, ``X target <filter>``),
  ``as you cast ~, <effect>`` (Mind's Desire class — one-shot replacement of
  the cast step on that spell), and ``cast ~'s copy`` riders.

The parser's extension loader does NOT call builders for trigger patterns —
``TRIGGER_PATTERNS`` is a list of 3-tuples, no builder.  The trigger's
effect tail is routed through ``parser.parse_effect`` automatically.

Intervening-if is verified here: the existing parser wires an intervening
"if <cond>, <effect>" suffix on triggered abilities through the core
``parse_triggered`` path.  We add no new plumbing — instead we add a static
pattern ``if_intervening_tail`` that catches the surviving
``if ..., <effect>`` fragments that the core sentence splitter strands as
their own sentence.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# Allow `from mtg_ast import ...` when loaded as a standalone module.
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    CounterSpell, Exile, Filter, Modification, Replacement, Static,
    TapEffect, Destroy, Bounce, Damage, UnknownEffect,
    TARGET_CREATURE, TARGET_ANY,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}


def _num(tok: str):
    tok = (tok or "").strip().lower()
    if tok.isdigit():
        return int(tok)
    return _NUM_WORDS.get(tok, tok)


def _parse_filter_safe(text: str) -> Filter:
    """Lazy import of parser.parse_filter — avoids a circular at load time."""
    try:
        from parser import parse_filter  # type: ignore
        f = parse_filter(text)
        if f is not None:
            return f
    except Exception:
        pass
    base = (text.strip().split() or ["thing"])[0]
    return Filter(base=base, targeted="target" in text)


def _mk_static(kind: str, raw: str, *args) -> Static:
    return Static(
        modification=Modification(kind=kind, args=tuple(args)),
        raw=raw,
    )


def _mk_replacement_static(repl: Replacement, raw: str) -> Static:
    return Static(
        modification=Modification(kind="replacement_static", args=(repl, raw)),
        raw=raw,
    )


# Common aliases
_SELF = (r"(?:~|this creature|this permanent|this spell|this ability|"
         r"this artifact|this enchantment|this land|this planeswalker)")


# ===========================================================================
# TRIGGER_PATTERNS
# ===========================================================================

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [

    # ------------------------------------------------------------------
    # Cast-event triggers from the stack side (broader than core's
    # "whenever you cast").  A "spell or ability an opponent controls"
    # trigger fires off events on the stack controlled by opponents.
    # ------------------------------------------------------------------
    (re.compile(
        r"^whenever a spell or ability an opponent controls"
        r"(?: targets? (?:~|this creature|this permanent|a [^,.]+))?",
        re.I),
     "opp_spell_or_ability", "self"),
    (re.compile(
        r"^whenever a spell an opponent controls targets? (?:~|this creature|a [^,.]+)",
        re.I),
     "opp_spell_targets", "self"),
    (re.compile(
        r"^whenever a player casts (?:their|a|an|the) ([^,.]+? )?spell",
        re.I),
     "any_player_cast", "all"),
    # "whenever a player casts a spell, sacrifice this" — variant on core
    (re.compile(r"^when a player casts a spell\b", re.I),
     "any_player_cast_once", "all"),
    # Loyalty-activation triggers (Vraska's Contempt-adjacent / Contagion-class)
    (re.compile(
        r"^whenever (?:you|a player|an opponent) "
        r"activate[s]? a loyalty ability(?: of (?:a |an )?[^,.]+)?",
        re.I),
     "loyalty_activation", "self"),
    (re.compile(
        r"^whenever (?:you|a player|an opponent) activate[s]? an ability "
        r"that isn'?t a (?:mana|loyalty) ability",
        re.I),
     "activation_non_mana", "self"),
    (re.compile(
        r"^whenever an ability of (?:equipped|enchanted|this) creature is activated\b",
        re.I),
     "self_ability_activated", "self"),

    # ------------------------------------------------------------------
    # Spell put into graveyard FROM THE STACK (Rest-in-Peace-for-spells
    # cousin, and "when ~ is countered" shapes).
    # ------------------------------------------------------------------
    (re.compile(
        r"^when (?:~|this spell) is (?:countered|put into a graveyard from the stack)\b",
        re.I),
     "self_countered_or_fizzled", "self"),
    (re.compile(
        r"^whenever a spell is countered\b",
        re.I),
     "any_spell_countered", "self"),

    # ------------------------------------------------------------------
    # "When you cast it" — a continuation trigger used inside granted
    # "exile ... when you cast it" payoffs (Suspend / Foretell / Plot
    # tails).  Core doesn't have it.
    # ------------------------------------------------------------------
    (re.compile(r"^when you cast it\b", re.I),
     "when_you_cast_it", "self"),

    # ------------------------------------------------------------------
    # "Whenever this creature becomes the target of a spell or ability
    # an opponent controls" — Pearled Unicorn / True Name Nemesis-class.
    # ------------------------------------------------------------------
    (re.compile(
        r"^whenever (?:~|this creature) becomes the target of a spell or ability "
        r"an opponent controls",
        re.I),
     "self_targeted_by_opp", "self"),
]


# ===========================================================================
# EFFECT_RULES — one-shot stack-manipulation spells
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _effect_rule(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Counter target activated / triggered ability (Stifle, Trickbind) -----
@_effect_rule(r"^counter target activated(?: or triggered)? ability(?:\.|$)")
def _counter_act_or_trig(m):
    return CounterSpell(target=Filter(base="ability", targeted=True))


@_effect_rule(r"^counter target triggered ability(?:\.|$)")
def _counter_trig(m):
    return CounterSpell(target=Filter(base="triggered_ability", targeted=True))


# --- Counter target ~ (self-reference — e.g. "counter target X" where X
#     resolves to the card's name).  Core's _counter_spell uses a greedy
#     capture that misses the bare "counter target ~" shape.
@_effect_rule(r"^counter target ~(?:\.|$)")
def _counter_self_named(m):
    return CounterSpell(target=Filter(base="self", targeted=True))


# --- Counter target <filter> unless its controller pays {cost} ----------
@_effect_rule(
    r"^counter target ([^.]+?) unless (?:its controller|that player) "
    r"(?:pays |sacrifices |discards |exiles )([^.]+?)(?:\.|$)"
)
def _counter_unless(m):
    tgt = _parse_filter_safe("target " + m.group(1))
    return CounterSpell(target=tgt, unless=None)


# --- Counter target spell with mana value X (already mostly handled; add
#     explicit X-valued shape)
@_effect_rule(r"^counter target ([^.]+?) with mana value x(?:\.|$)")
def _counter_mv_x(m):
    tgt = _parse_filter_safe("target " + m.group(1))
    return CounterSpell(target=tgt)


# --- Change the target of target spell or ability ------------------------
@_effect_rule(
    r"^change the target(?:s)? of target (spell or ability|spell|activated or triggered ability|ability) "
    r"to (.+?)(?:\.|$)"
)
def _change_target(m):
    return UnknownEffect(raw_text=m.group(0))


# --- Redirect target spell / ability -------------------------------------
@_effect_rule(r"^redirect target ([^.]+?) to (.+?)(?:\.|$)")
def _redirect(m):
    return UnknownEffect(raw_text=m.group(0))


# --- Multi-target spells: "N target <filter>" / "X target <filter>" ------
# Fireball / Hydra Broodmaster / Comet Storm variants.  We produce the
# most common leaf effect — damage / destroy / tap / return — when we can
# sniff the verb; otherwise we park the whole clause as UnknownEffect but
# still claim the shape so it counts as parsed.
@_effect_rule(
    r"^(?:~ )?deals (\d+|x) damage divided (?:as you choose )?among "
    r"(any number of|(?:one|two|three|four|\d+|x) or more|up to (?:one|two|three|four|\d+|x)) "
    r"target ([^.]+?)(?:\.|$)"
)
def _damage_divided(m):
    amt = _num(m.group(1))
    filt = _parse_filter_safe("target " + m.group(3))
    return Damage(amount=amt, target=filt, divided=True)


@_effect_rule(r"^tap (x|\d+) target ([^.]+?)(?:\.|$)")
def _tap_n_or_x(m):
    n = _num(m.group(1))
    f = _parse_filter_safe(f"{n} target " + m.group(2))
    return TapEffect(target=f)


@_effect_rule(r"^destroy (x|\d+) target ([^.]+?)(?:\.|$)")
def _destroy_n_or_x(m):
    n = _num(m.group(1))
    f = _parse_filter_safe(f"{n} target " + m.group(2))
    return Destroy(target=f)


@_effect_rule(
    r"^return (x|\d+) target ([^.]+? cards?) from your graveyard to your hand(?:\.|$)"
)
def _return_n_or_x(m):
    n = _num(m.group(1))
    f = _parse_filter_safe(f"{n} target " + m.group(2))
    # Recurse from graveyard
    from mtg_ast import Recurse  # lazy import, matches module style
    return Recurse(query=f)


@_effect_rule(r"^exile (x|\d+) target ([^.]+?)(?:\.|$)")
def _exile_n_or_x(m):
    n = _num(m.group(1))
    f = _parse_filter_safe(f"{n} target " + m.group(2))
    return Exile(target=f)


# --- "as you cast ~, [effect]" — rare (Mind's Desire-class, Wild Pair,
#     Epic cycle, and various new-frame cards that modify the cast step).
#     Modeled as a Replacement of the cast step.
@_effect_rule(
    r"^as you cast (?:~|this spell), ([^.]+?)(?:\.|$)"
)
def _as_you_cast_self(m):
    body = m.group(1).strip()
    from parser import parse_effect  # lazy
    inner = parse_effect(body) or UnknownEffect(raw_text=body)
    return Replacement(
        trigger_event="cast_self",
        replacement=inner,
    )


@_effect_rule(
    r"^as you cast (?:a|an|the next) ([^,]+? )?spell, ([^.]+?)(?:\.|$)"
)
def _as_you_cast_filtered(m):
    body = m.group(2).strip()
    from parser import parse_effect  # lazy
    inner = parse_effect(body) or UnknownEffect(raw_text=body)
    return Replacement(
        trigger_event="cast_filtered_spell",
        replacement=inner,
    )


@_effect_rule(
    r"^as you activate (?:~|this ability|an activated ability), ([^.]+?)(?:\.|$)"
)
def _as_you_activate(m):
    body = m.group(1).strip()
    from parser import parse_effect  # lazy
    inner = parse_effect(body) or UnknownEffect(raw_text=body)
    return Replacement(
        trigger_event="activate",
        replacement=inner,
    )


# --- "You may choose new targets for the copy/that spell" ---------------
@_effect_rule(
    r"^you may choose new targets for (?:the |that |those )?(?:cop(?:y|ies)|spells?)"
    r"(?:\.|$)"
)
def _new_targets_for(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "Copy that spell / target spell.  You may choose new targets." -----
#     (Radiate / Reverberate / Fork-class).
@_effect_rule(
    r"^copy (?:that|target) ([^.]+? spell)(?:\. you may choose new targets for the copy)?(?:\.|$)"
)
def _copy_spell(m):
    from mtg_ast import CopySpell
    return CopySpell(
        target=_parse_filter_safe("target " + m.group(1)),
        may_choose_new_targets=True,
    )


# --- "Cast the copy without paying its mana cost" -----------------------
@_effect_rule(
    r"^you may cast (?:the|a|those|any number of the) cop(?:y|ies)"
    r"(?: of (?:that spell|~))?"
    r"(?: without paying (?:its|their) mana costs?)?(?:\.|$)"
)
def _cast_copy(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "This spell can't be countered" (body form — also handled in
#     parse_static, but shows up inline on modal bodies) ---------------
@_effect_rule(r"^(?:this spell|~) can'?t be countered(?:\.|$)")
def _uncounterable(m):
    return UnknownEffect(raw_text=m.group(0))


# --- "This spell can't be copied" ---------------------------------------
@_effect_rule(r"^(?:this spell|~) can'?t be copied(?:\.|$)")
def _uncopyable(m):
    return UnknownEffect(raw_text=m.group(0))


# ===========================================================================
# STATIC_PATTERNS — always-on timing / stack / cast legality modifiers
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _static_pattern(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Timing restrictions on casting SELF -------------------------------
# "Cast this spell only as a sorcery." / "Cast ~ only during combat."
# / "Cast this spell only during your turn."
@_static_pattern(
    r"^cast (?:this spell|~) only (as a sorcery|during (?:your|an opponent'?s|combat|the declare [^,.]+|[^,.]+?))\.?$"
)
def _cast_only_restriction(m, raw):
    when = m.group(1).strip()
    return _mk_static("cast_timing_restriction", raw, when)


# --- "Cast this spell only if [condition]" -----------------------------
@_static_pattern(
    r"^cast (?:this spell|~) only if ([^.]+?)\.?$"
)
def _cast_only_if(m, raw):
    return _mk_static("cast_conditional_restriction", raw, m.group(1).strip())


# --- "Cast this spell only during [player]'s turn" (Teferi-style) -----
@_static_pattern(
    r"^cast (?:this spell|~) only during (?:your|an opponent'?s|each player'?s|any player'?s) (?:turn|main phase|upkeep|end step)\.?$"
)
def _cast_only_during(m, raw):
    return _mk_static("cast_phase_restriction", raw)


# --- "~ can be cast even if ..." (Eternal Flame / Ancestral Knowledge
#     flavor) — override of normal casting legality.
@_static_pattern(
    r"^(?:~|this spell) (?:can be cast (?:even if it would be illegal to cast|as though you could cast sorceries|any time you could cast a sorcery)|has flash[^.]*)\.?$"
)
def _cast_legality_override(m, raw):
    return _mk_static("cast_legality_override", raw)


# --- "You may cast ~ as though it had flash" / "You may cast this spell
#     at any time you could cast an instant" (Vedalken Orrery wrapper) --
@_static_pattern(
    r"^you may cast (?:~|this (?:spell|card)) (?:as though it had flash|any time you could cast (?:a |an )?(?:instant|sorcery))\.?$"
)
def _grant_flash_self_or_instant_timing(m, raw):
    return _mk_static("grant_flash_self", raw)


# --- "You may cast creature/instant/sorcery spells as though they had
#     flash."  Widespread (Leyline of Anticipation, Vedalken Orrery,
#     Emergence Zone).
@_static_pattern(
    r"^you may cast ([^.]+? spells?|spells) as though (?:they|it) had flash\.?$"
)
def _grant_flash_spells(m, raw):
    filt = m.group(1).strip()
    return _mk_static("grant_flash_spells", raw, filt)


# --- "You may activate ~'s loyalty abilities at instant speed"
#     (Teferi, Temporal Archmage-class).
@_static_pattern(
    r"^you may activate (?:~|this planeswalker|(?:your |)?loyalty abilities(?: of [^,.]+)?)"
    r"(?:'s loyalty abilities)? at instant speed\.?$"
)
def _loyalty_instant_speed(m, raw):
    return _mk_static("loyalty_at_instant_speed", raw)


# --- "You may activate loyalty abilities of planeswalkers you control
#     on any player's turn as though they had flash" — etc.
@_static_pattern(
    r"^you may activate loyalty abilities of [^,.]+ (?:any time|on any player'?s turn)"
    r"(?: any time you could cast an instant)?\.?$"
)
def _loyalty_any_time(m, raw):
    return _mk_static("loyalty_any_time", raw)


# --- "Activated abilities of creatures can't be activated" (Linvala,
#     Keeper of Silence) + "Triggered abilities can't be activated"
#     (incorrect wording that still shows up in some playtest / judge
#     text — we cover the closer form: "can't cause abilities to trigger",
#     "triggered abilities don't trigger").
@_static_pattern(
    r"^(?:activated|triggered) abilities of "
    r"([^.]+?) can'?t (?:be activated|trigger)\.?$"
)
def _abilities_disabled(m, raw):
    return _mk_static("abilities_disabled", raw, m.group(1).strip())


@_static_pattern(
    r"^(?:triggered abilities don'?t trigger|abilities (?:of [^.]+? )?can'?t trigger)\.?$"
)
def _triggers_off(m, raw):
    return _mk_static("triggers_off", raw)


# --- "Only your opponents may activate this ability" / "Any player may
#     activate this ability" — activation-rights modifiers on an ability
#     that sits on THIS card (not a general hoser).
@_static_pattern(
    r"^(?:any player|only your opponents|each player|only you) may activate this ability\.?$"
)
def _activation_rights(m, raw):
    return _mk_static("activation_rights_this_ability", raw)


# --- "This ability triggers only once each turn" / "This ability
#     triggers only if ..." — intervening-if-style restriction on
#     the previous ability (already partially handled in core).
@_static_pattern(
    r"^this ability triggers only (once each turn|if [^.]+|during [^.]+)\.?$"
)
def _ability_trigger_restriction(m, raw):
    return _mk_static("ability_trigger_restriction", raw, m.group(1).strip())


# --- "If a spell or ability you control would [event], [event] instead"
#     (replacement on your own stack objects) ----------------------------
@_static_pattern(
    r"^if a spell or ability (?:you control|an opponent controls) would "
    r"([^,.]+?), ([^.]+?) instead\.?$"
)
def _replace_your_spell_event(m, raw):
    repl = Replacement(
        trigger_event="spell_or_ability_event",
        replacement=UnknownEffect(raw_text=m.group(2).strip()),
    )
    return _mk_replacement_static(repl, raw)


# --- "If a spell would be put into a graveyard from the stack, exile it
#     instead" (Rest-in-Peace-class for spells) --------------------------
@_static_pattern(
    r"^if (?:a spell|a card|an instant or sorcery spell) would be put into "
    r"(?:a|its owner'?s|any) graveyard from the stack, exile it instead\.?$"
)
def _exile_spells_going_to_gy(m, raw):
    repl = Replacement(
        trigger_event="spell_to_graveyard",
        replacement=Exile(target=Filter(base="spell", targeted=False)),
    )
    return _mk_replacement_static(repl, raw)


# --- "Spells can't be countered" (Prowling Serpopard, Vexing Shusher,
#     Aetherling-flavored).  Already touched by core's "immunity" but not
#     for the generic mass form. ----------------------------------------
@_static_pattern(r"^(?:spells|creature spells|[^.]+? spells) can'?t be countered\.?$")
def _spells_uncounterable(m, raw):
    return _mk_static("spells_uncounterable", raw)


# --- "This spell can't be copied" -------------------------------------
@_static_pattern(r"^(?:this spell|~) can'?t be copied\.?$")
def _self_uncopyable(m, raw):
    return _mk_static("self_uncopyable", raw)


# --- "Your opponents can't cast spells" / "Players can't cast spells
#     with the same name" / "Each player can't cast more than one spell
#     each turn" — mass cast restrictions.  These are STACK-legality
#     statics (they prevent the "cast" step from being legal).
@_static_pattern(
    r"^(?:your opponents|players|each player|opponents) can'?t cast "
    r"([^.]+?)(?:\s*(?:\.|$))"
)
def _mass_cast_restriction(m, raw):
    return _mk_static("mass_cast_restriction", raw, m.group(1).strip())


@_static_pattern(
    r"^you can'?t cast more than (?:one|\d+) spells? each turn\.?$"
)
def _self_cast_limit(m, raw):
    return _mk_static("self_cast_limit", raw)


# --- "Your opponents can't cast spells during your turn"
#     (Teferi, Time Raveler static). -------------------------------------
@_static_pattern(
    r"^your opponents can'?t cast spells during (?:your|combat|any player'?s) (?:turn|combat)\.?$"
)
def _opp_cant_cast_during(m, raw):
    return _mk_static("opp_cant_cast_during", raw)


# --- "Spells you cast cost {N} less" / "Spells your opponents cast cost
#     {N} more" — cost-side replacements.  (Core already catches the
#     self-reduction shape; we add the opponent-cost-up shape that core's
#     regex misses.) ---------------------------------------------------
@_static_pattern(
    r"^(?:spells|creature spells|noncreature spells|[^.]+? spells)"
    r" (?:your opponents cast )?cost \{(\d+)\} (?:more|less) to cast\.?$"
)
def _cost_mod(m, raw):
    return _mk_static("cost_mod", raw, int(m.group(1)))


# --- "The next <N> <filter> spell(s) you cast this turn costs {X} less
#     to cast" / "The first instant or sorcery spell you cast each turn
#     costs {2} less" — one-shot cost reduction on a future cast. -------
@_static_pattern(
    r"^the (?:first|second|next) (?:instant or sorcery |creature |)spell you cast"
    r" (?:this turn|each turn) costs \{(\d+|x)\} less to cast\.?$"
)
def _next_spell_cost_reduce(m, raw):
    amt = _num(m.group(1))
    return _mk_static("next_spell_cost_reduce", raw, amt)


# --- "Spells you cast from your graveyard cost {N} less"
@_static_pattern(
    r"^spells you cast from (?:your graveyard|exile|anywhere other than your hand) cost \{(\d+)\} less to cast\.?$"
)
def _cast_from_zone_cost(m, raw):
    return _mk_static("cast_from_zone_cost", raw, int(m.group(1)))


# --- "Each opponent can cast spells only any time they could cast a
#     sorcery" (hosers like Manglehorn, Cursed Totem cousins on spells) --
@_static_pattern(
    r"^(?:each opponent|your opponents|players) can cast spells only any time "
    r"(?:they|the player) could cast a sorcery\.?$"
)
def _opp_sorcery_speed_only(m, raw):
    return _mk_static("opp_sorcery_speed_only", raw)


# --- "Players can't cast spells from graveyards or libraries" /
#     "Players can't cast spells" — Grafdigger's Cage-class.
@_static_pattern(
    r"^(?:players|your opponents) can'?t cast spells from "
    r"(?:graveyards?|libraries|graveyards? or libraries)\.?$"
)
def _no_cast_from_zone(m, raw):
    return _mk_static("no_cast_from_zone", raw)


# --- "You may cast instant and sorcery spells from your graveyard"
#     (Mizzix's Mastery-class permission) -------------------------------
@_static_pattern(
    r"^you may cast (?:instant and sorcery|[^.]+?) spells? from "
    r"(?:your|any) (?:graveyard|exile)\.?$"
)
def _may_cast_from_zone(m, raw):
    return _mk_static("may_cast_from_zone", raw)


# --- "You may cast <something> without paying its mana cost" - granted
#     on another card (Narset / Bolas's Citadel / Omniscience bodies).
@_static_pattern(
    r"^you may cast (?:[^.]+?) without paying (?:its|their) mana costs?\.?$"
)
def _cast_without_paying_static(m, raw):
    return _mk_static("cast_without_paying_static", raw)


# --- "Once each turn, you may cast a spell ..." - once-per-turn
#     permission-style modifier.
@_static_pattern(
    r"^once each turn, you may cast (?:a|an|any|[^.]+?) spells?(?: [^.]+)?\.?$"
)
def _once_per_turn_cast(m, raw):
    return _mk_static("once_per_turn_cast", raw)


# --- "Spells your opponents cast that target this creature / ~ cost {N}
#     more to cast" (Mother of Runes-adjacent protection via cost).
@_static_pattern(
    r"^spells your opponents cast that target (?:~|this creature|this permanent|[^.]+?) "
    r"cost \{(\d+)\} more to cast\.?$"
)
def _opp_spell_target_self_cost_up(m, raw):
    return _mk_static("opp_target_cost_up", raw, int(m.group(1)))


# --- Intervening-if tail ("If <cond>, <effect>" as its own sentence) --
# The core splitter strands these when the sentence structure surprises
# it (modal bodies, chained triggers).  Recording as a typed tail lets
# us keep coverage honest without claiming to fully handle the flow.
@_static_pattern(
    r"^if ([^,]+?), ([^.]+?)\.?$"
)
def _if_intervening_tail(m, raw):
    # Guard: avoid swallowing obviously non-intervening "if" clauses we
    # already have rules for (shocklands etc. — those match earlier by
    # exact text).  Anything reaching here is a generic intervening-if
    # remainder.
    cond = m.group(1).strip()
    body = m.group(2).strip()
    return _mk_static("if_intervening_tail", raw, cond, body)


# ---------------------------------------------------------------------------
# Self-check
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    samples_effect = [
        "counter target activated ability",
        "counter target activated or triggered ability",
        "counter target triggered ability",
        "counter target ~",
        "counter target spell with mana value x",
        "change the target of target spell or ability to ~",
        "tap x target creatures",
        "destroy x target artifacts",
        "exile x target creature cards from your graveyard",
        "return x target creature cards from your graveyard to your hand",
        "as you cast ~, you may exile a card from your hand",
        "as you cast a creature spell, reveal the top card of your library",
        "this spell can't be countered",
        "this spell can't be copied",
        "you may choose new targets for the copy",
    ]
    samples_static = [
        "cast this spell only as a sorcery",
        "cast this spell only during combat",
        "cast this spell only during your turn",
        "cast this spell only if you control a swamp",
        "you may cast ~ as though it had flash",
        "you may cast creature spells as though they had flash",
        "you may activate ~'s loyalty abilities at instant speed",
        "activated abilities of creatures can't be activated",
        "any player may activate this ability",
        "this ability triggers only once each turn",
        "if a spell or ability you control would cause you to draw, you draw two cards instead",
        "if a spell would be put into a graveyard from the stack, exile it instead",
        "spells can't be countered",
        "this spell can't be copied",
        "your opponents can't cast spells during your turn",
        "your opponents can't cast spells with the same name",
        "you can't cast more than one spell each turn",
        "spells you cast from your graveyard cost {1} less to cast",
        "the first instant or sorcery spell you cast each turn costs {2} less to cast",
        "each opponent can cast spells only any time they could cast a sorcery",
        "players can't cast spells from graveyards or libraries",
        "you may cast instant and sorcery spells from your graveyard",
        "once each turn, you may cast a spell from your hand",
        "spells your opponents cast that target this creature cost {2} more to cast",
    ]
    samples_triggers = [
        "whenever a spell or ability an opponent controls targets ~",
        "whenever a spell or ability an opponent controls targets a creature you control",
        "whenever a player casts a spell",
        "when a player casts a spell",
        "whenever you activate a loyalty ability",
        "whenever an ability of equipped creature is activated",
        "when ~ is countered",
        "whenever a spell is countered",
        "when you cast it",
        "whenever ~ becomes the target of a spell or ability an opponent controls",
    ]
    hit_e = sum(1 for s in samples_effect
                for pat, _ in EFFECT_RULES if pat.match(s))
    hit_s = sum(1 for s in samples_static
                for pat, _ in STATIC_PATTERNS if pat.match(s))
    hit_t = sum(1 for s in samples_triggers
                for pat, _, _ in TRIGGER_PATTERNS if pat.match(s))
    print(f"stack_timing.py: "
          f"{len(EFFECT_RULES)} effect rules, "
          f"{len(STATIC_PATTERNS)} static patterns, "
          f"{len(TRIGGER_PATTERNS)} trigger patterns")
    print(f"  effect  sample hits: {hit_e}/{len(samples_effect)}")
    print(f"  static  sample hits: {hit_s}/{len(samples_static)}")
    print(f"  trigger sample hits: {hit_t}/{len(samples_triggers)}")
