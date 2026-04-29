#!/usr/bin/env python3
"""Damage prevention, redirection, reflection, and protective effects.

Family: DAMAGE PREVENTION / REDIRECTION / REFLECTION / HALVING / COMBAT-ONLY
PROTECTION / SOURCE-FILTERED PREVENTION / DAMAGE-TO-NONLIFE / BANDING, and
related protective phrasings beyond the bare "prevent all damage" rule
already in ``parser.py``.

Exported tables (shape matches the parser's merge points):

- ``EFFECT_RULES`` — ``(compiled_regex, builder_fn)`` entries appended to
  ``parser.EFFECT_RULES``. Fire when the sentence IS the effect (spell
  ability body, activated effect body, triggered effect body). Covers:

    * "prevent the next N damage that would be dealt to <filter> this turn"
      (Samite Healer, Healing Salve, Sacred Boon, bulleted-mode variants)
    * "the next time a source of your choice would deal damage to <filter>
      this turn, prevent that damage." (Deflecting Palm, Cho-Arrim Alchemist,
      CoP activations, Story Circle)
    * "the next time a [color] source of your choice would deal damage to
      you this turn, prevent that damage." (CoP cycle)
    * "the next time a source of your choice would deal damage to <target>
      this turn, that source deals that damage to you/~ instead." (Jade
      Monolith, Reflect Damage redirection)
    * "prevent half that damage, rounded up|down" (Gisela fork, Dark Sphere)
    * "prevent all combat damage that would be dealt by <filter> this turn"
      (Boros Fury-Shield, Guard Dogs)
    * "prevent all damage that <creature> would deal this turn" (Angelic
      Blessing-adjacent attacker-pacify instants)
    * "prevent all [non]combat damage that would be dealt to <filter>"
      (Sandskin, CoP-like selective hosers)
    * "prevent all damage that a source of your choice would deal this turn"
      (Guardian Angel-style)
    * "prevent all damage that [color] sources would deal this turn" (Brave
      the Elements-adjacent, Circle of Protection spell forms)

- ``STATIC_PATTERNS`` — ``(compiled_regex, builder_fn)`` entries consulted
  by ``parser.parse_static`` before the generic catch-all. Covers:

    * Pariah-class full-redirect: "All damage that would be dealt to you is
      dealt to <filter> instead." (Pariah, Pariah's Shield, Palisade Giant
      — with "and other permanents you control" extension.)
    * Mirror-Strike-class reflection: "All combat damage that would be dealt
      to you this turn by <filter> is dealt to its controller instead."
    * Gisela-class halving: "If a source would deal damage to <filter>,
      prevent half that damage, rounded up|down."
    * Gisela-class doubling (dual to halving, emits a Replacement): "If a
      source would deal damage to <filter>, that source deals double that
      damage to <filter> instead."
    * Worship-class life floor variant: "damage doesn't cause <who> to lose
      life" (Archon of Coronation monarch clause).
    * Indestructible-via-toughness: "damage doesn't reduce <subject>'s
      toughness".
    * "Damage can't be prevented." (Leyline of Punishment, Flame-Blessed
      Bolt rider.) Emitted as a Replacement of the prevent event with a
      no-op.
    * "Combat damage can't be prevented" and "damage that would be dealt by
      <filter> can't be prevented" — scoped variants.
    * Banding reminder-text hoser: bare "Banding" is already a keyword, but
      the full reminder sentence "Any creatures with banding, and up to one
      without, can attack in a band…" was slipping through when Scryfall
      preserved it inline. We swallow it as a static note.
    * "Target creature gains banding until end of turn" (Helm of Chatzuk
      body) — as an effect, not a static, but carried here for family.

- ``TRIGGER_PATTERNS`` — ``(compiled_regex, event_name, scope)`` entries
  merged into ``parser._TRIGGER_PATTERNS``. Covers damage-prevention-event
  triggers:

    * "When damage is prevented this way, ~ deals that much damage to …"
      (Deflecting Palm, Eight-and-a-Half-Tails tail)
    * "Whenever damage that would be dealt to you is prevented, …"
    * "Whenever damage from a [color] source is prevented this way this
      turn, …" (Samite Ministration)

Ordering: most specific first. The parser's first-match-wins loop relies on
the specific-before-general order so that (e.g.) the color-source CoP rule
fires before the generic "source of your choice" rule.

`~` is already normalised in oracle text by ``parser.normalize()`` before
any of these regexes see the string.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

# Allow this file to live in scripts/extensions/ and still import the AST
# nodes from scripts/mtg_ast.py.
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Damage, Filter, Modification, Prevent, Replacement, Static, UnknownEffect,
)


# ---------------------------------------------------------------------------
# Shared helpers (kept local — replacements.py has its own copy, we don't
# cross-import to avoid load-order coupling).
# ---------------------------------------------------------------------------

_NUM_WORDS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
    "half": "half",
}

_COLOR_WORD = r"(?:white|blue|black|red|green|colorless|mono[a-z]*|multicolored)"
_COLOR_GROUP = (
    r"(?:(?:a |an )?" + _COLOR_WORD + r"(?:\s+(?:and/or|or|and)\s+" +
    _COLOR_WORD + r")*)"
)


def _num(tok: str):
    tok = tok.strip().lower()
    if tok.isdigit():
        return int(tok)
    return _NUM_WORDS.get(tok, tok)


def _parse_filter_safe(text: str) -> Filter:
    """Import parser.parse_filter lazily; fall back to a bare-word Filter."""
    try:
        from parser import parse_filter  # type: ignore
        f = parse_filter(text)
        if f is not None:
            return f
    except Exception:
        pass
    head = (text.strip().split() or ["thing"])[0]
    return Filter(base=head, targeted=False)


def _mk_static(repl: Replacement, raw: str, tag: Optional[str] = None) -> Static:
    args = (repl, raw) if tag is None else (repl, raw, tag)
    return Static(
        modification=Modification(kind="replacement_static", args=args),
        raw=raw,
    )


# ===========================================================================
# EFFECT_RULES — sentence-body prevent/redirect/reflect effects
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _effect_rule(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Prevent next N damage to <filter>, optionally bulleted, optional
#     "this turn" tail, optional "by a source of your choice" qualifier.
#     Catches modal-bulleted "• Prevent the next 2 damage that would be
#     dealt to any target this turn" (Healing Salve mode).
@_effect_rule(
    r"^(?:•\s*)?prevent the next (\d+|x) damage that (?:a source of your choice )?"
    r"would be dealt (?:this turn )?to ([^.]+?)(?: this turn)?(?:\.|$)"
)
def _prevent_next_n_to(m):
    amt = _num(m.group(1))
    tgt = _parse_filter_safe(m.group(2))
    return Prevent(amount=amt, damage_filter=tgt, duration="until_end_of_turn")


# --- Prevent next N damage "to any number of targets" (Divine Deflection,
#     some splice/convoke variants that spread damage).
@_effect_rule(
    r"^prevent the next (\d+|x) damage that would be dealt this turn to any number of targets(?:,\s*divided as you choose)?(?:\.|$)"
)
def _prevent_next_n_divided(m):
    amt = _num(m.group(1))
    return Prevent(amount=amt, damage_filter=Filter(base="any_targets", targeted=True),
                   duration="until_end_of_turn")


# --- CoP activation (color-filtered, to you): "the next time a [color] source
#     of your choice would deal damage to you this turn, prevent that damage."
@_effect_rule(
    r"^the next time (?:a|an) (" + _COLOR_WORD + r") source of your choice would deal damage to you this turn, prevent that damage(?:\.|$)"
)
def _cop_color(m):
    color = m.group(1).lower()
    tgt = Filter(base="you", targeted=False)
    repl = Replacement(
        trigger_event=f"would_deal_damage_by_{color}_source",
        replacement=Prevent(amount="all", damage_filter=tgt),
    )
    # As an effect body we return the Replacement — downstream engines can
    # schedule the one-shot prevention delta.
    return repl


# --- Story-Circle-style "of the chosen color" variant (color chosen at ETB).
@_effect_rule(
    r"^the next time a source of your choice of the chosen color would deal damage to you this turn, prevent that damage(?:\.|$)"
)
def _story_circle(m):
    tgt = Filter(base="you", targeted=False)
    return Replacement(
        trigger_event="would_deal_damage_by_chosen_color",
        replacement=Prevent(amount="all", damage_filter=tgt),
    )


# --- Deflecting-Palm-class: "the next time a source of your choice would
#     deal damage to <tgt> this turn, prevent that damage."
@_effect_rule(
    r"^the next time a source of your choice would deal damage to ([^.]+?) this turn, prevent that damage(?:\.|$)"
)
def _choose_source_prevent(m):
    tgt = _parse_filter_safe(m.group(1))
    return Replacement(
        trigger_event="chosen_source_would_deal_damage",
        replacement=Prevent(amount="all", damage_filter=tgt),
    )


# --- Jade-Monolith / Reflect-Damage class: redirect chosen source's next
#     damage to caster/self. Replacement whose replacement is a Damage node
#     rerouted at the same amount ("var").
@_effect_rule(
    r"^the next time a source of your choice would deal damage (?:to [^.]+? )?this turn, "
    r"(?:that source deals that damage to ([^.]+?) instead|that damage is dealt to ([^.]+?) instead)(?:\.|$)"
)
def _choose_source_redirect(m):
    dst_text = m.group(1) or m.group(2) or "you"
    dst = _parse_filter_safe(dst_text)
    return Replacement(
        trigger_event="chosen_source_would_deal_damage",
        replacement=Damage(amount="var", target=dst),
    )


# --- Generic "the next time <X> would deal damage this turn, prevent that
#     damage." (target creature / that creature variants.)
@_effect_rule(
    r"^the next time ([^.]+?) would deal damage (?:to [^.]+? )?this turn, prevent that damage(?:\.|$)"
)
def _next_time_x_prevent(m):
    src = _parse_filter_safe(m.group(1))
    return Replacement(
        trigger_event="source_would_deal_damage",
        replacement=Prevent(amount="all", damage_filter=src),
    )


# --- Dark-Sphere-class one-shot halving: "prevent half that damage, rounded
#     up|down." Appears as a rider clause after a "source would deal damage"
#     conditional. We capture as a Prevent with amount="half" + rounding tag
#     in damage_filter's base (lightweight, stays in AST).
@_effect_rule(
    r"^prevent half that damage, rounded (up|down)(?:\.|$)"
)
def _prevent_half(m):
    rounding = m.group(1).lower()
    return Prevent(
        amount="half",
        damage_filter=Filter(base=f"half_{rounding}", targeted=False),
        duration="one_shot",
    )


# --- "prevent all combat damage that would be dealt by <filter> this turn"
#     (Boros Fury-Shield, Guard Dogs, Moment's Peace-ish scoped fogs). The
#     bare "prevent all combat damage … this turn" is already covered by the
#     parser's own rule.
@_effect_rule(
    r"^prevent all (?:combat )?damage that would be dealt by ([^.]+?) this turn(?:\.|$)"
)
def _prevent_all_by(m):
    src = _parse_filter_safe(m.group(1))
    return Prevent(amount="all", damage_filter=src, duration="until_end_of_turn")


# --- "prevent all damage <filter> would deal this turn" (Ensnare-adjacent).
@_effect_rule(
    r"^prevent all (?:combat )?damage (?:that )?([^.]+?) would deal(?: to [^.]+?)? this turn(?:\.|$)"
)
def _prevent_all_x_would_deal(m):
    src = _parse_filter_safe(m.group(1))
    return Prevent(amount="all", damage_filter=src, duration="until_end_of_turn")


# --- "prevent all [non]combat damage that would be dealt to <filter>" — the
#     top-level prevent rule in parser.py requires the sentence end with
#     "that would be dealt…" but some variants have a target tail.
@_effect_rule(
    r"^prevent all (?:(?:non)?combat )?damage that would be dealt to ([^.]+?)(?: this turn)?(?:\.|$)"
)
def _prevent_all_to(m):
    tgt = _parse_filter_safe(m.group(1))
    return Prevent(amount="all", damage_filter=tgt, duration="until_end_of_turn")


# --- "prevent all damage that a source of your choice would deal this turn"
#     (Guardian Angel, Samite Ministration tail).
@_effect_rule(
    r"^prevent all damage that a source of your choice would deal (?:to [^.]+? )?this turn(?:\.|$)"
)
def _prevent_all_chosen_source(m):
    return Replacement(
        trigger_event="chosen_source_any_damage",
        replacement=Prevent(amount="all", damage_filter=Filter(base="any", targeted=False)),
    )


# --- Color-source global prevent: "prevent all damage that [color]
#     (and/or [color]) sources would deal this turn" (CoP-as-spell, Brave
#     the Elements cousins).
@_effect_rule(
    r"^prevent all damage that " + _COLOR_GROUP + r" sources would deal(?: to [^.]+?)? this turn(?:\.|$)"
)
def _prevent_all_color_sources(m):
    return Prevent(
        amount="all",
        damage_filter=Filter(base="color_source", targeted=False),
        duration="until_end_of_turn",
    )


# --- Target-creature scoped fog: "prevent all combat damage target creature
#     would deal this turn" (Guard Dogs exact phrasing).
@_effect_rule(
    r"^prevent all (?:combat )?damage target (attacking|blocking|attacking or blocking|creature) [^.]*would deal this turn(?:\.|$)"
)
def _prevent_target_creature_would_deal(m):
    return Prevent(
        amount="all",
        damage_filter=Filter(base="target_creature", targeted=True),
        duration="until_end_of_turn",
    )


# --- "Target creature gains banding until end of turn." (Helm of Chatzuk
#     activated body). Technically an ability grant; we route through the
#     UnknownEffect so downstream still records the grant.
@_effect_rule(
    r"^target creature gains banding until end of turn(?:\.|$)"
)
def _grant_banding(m):
    return UnknownEffect(raw_text="grant banding target creature EOT")


# ===========================================================================
# STATIC_PATTERNS — always-on protective statics
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _static_pattern(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Pariah-class redirect: "All damage that would be dealt to you (and
#     other permanents you control) is dealt to <target> instead."
@_static_pattern(
    r"^all damage that would be dealt to you(?: and(?: other)? permanents you control)? is dealt to ([^.]+?) instead$"
)
def _pariah(m, raw):
    dst = _parse_filter_safe(m.group(1))
    repl = Replacement(
        trigger_event="damage_to_you_redirect",
        replacement=Damage(amount="var", target=dst),
    )
    return _mk_static(repl, raw, "pariah_redirect")


# --- Mirror-Strike-class reflection: "All combat damage that would be dealt
#     to you this turn by <source> is dealt to its controller instead."
@_static_pattern(
    r"^all (?:combat )?damage that would be dealt(?: to you)?(?: this turn)? by ([^.]+?) is dealt to (?:its controller|that source's controller|[^.]+?) instead$"
)
def _mirror_strike(m, raw):
    src = _parse_filter_safe(m.group(1))
    repl = Replacement(
        trigger_event="damage_by_source_reflect",
        replacement=Damage(amount="var", target=Filter(base="source_controller", targeted=False)),
    )
    # Keep src inside args for downstream query
    return Static(
        modification=Modification(kind="replacement_static",
                                  args=(repl, raw, "reflect", src)),
        raw=raw,
    )


# --- Gisela-class halving prevent: "If a source would deal damage to
#     <filter>, prevent half that damage, rounded up|down."
@_static_pattern(
    r"^if a source would deal damage to ([^,.]+?), prevent half that damage, rounded (up|down)$"
)
def _halving_static(m, raw):
    tgt = _parse_filter_safe(m.group(1))
    rounding = m.group(2).lower()
    repl = Replacement(
        trigger_event="would_deal_damage_to_filter",
        replacement=Prevent(
            amount="half",
            damage_filter=Filter(base=f"half_{rounding}", targeted=False),
        ),
    )
    return Static(
        modification=Modification(kind="replacement_static",
                                  args=(repl, raw, "halving", tgt)),
        raw=raw,
    )


# --- Gisela-class doubling (dual of halving — often on the same card):
#     "If a source would deal damage to <filter>, that source deals double
#     that damage to <same filter> instead."
@_static_pattern(
    r"^if a source would deal damage to ([^,.]+?), that source deals double that damage to ([^.]+?) instead$"
)
def _doubling_static(m, raw):
    tgt = _parse_filter_safe(m.group(1))
    repl = Replacement(
        trigger_event="would_deal_damage_to_filter",
        replacement=Damage(amount="double", target=tgt),
    )
    return Static(
        modification=Modification(kind="replacement_static",
                                  args=(repl, raw, "doubling", tgt)),
        raw=raw,
    )


# --- Worship-class life floor (also in replacements.py, but we include
#     the "damage doesn't cause <who> to lose life" phrasing which is a
#     different grammar production — Archon of Coronation's monarch clause).
@_static_pattern(
    r"^(?:as long as [^,]+?, )?damage doesn'?t cause (you|its controller|that player|[^.]+?) to lose life$"
)
def _no_life_loss(m, raw):
    who = m.group(1).strip()
    repl = Replacement(
        trigger_event="damage_would_cause_life_loss",
        replacement=UnknownEffect(raw_text=f"damage doesn't reduce life: {who}"),
    )
    return _mk_static(repl, raw, "no_life_loss")


# --- Indestructible-via-toughness replacement (damage doesn't reduce
#     toughness — old Legend-rule-adjacent cards like Worship's close
#     cousins and Ali from Cairo family).
@_static_pattern(
    r"^damage (?:dealt to (?:~|this creature|this permanent) )?doesn'?t reduce (?:~'s|this creature's|its) toughness$"
)
def _no_toughness_reduction(m, raw):
    repl = Replacement(
        trigger_event="damage_would_reduce_toughness",
        replacement=UnknownEffect(raw_text="damage doesn't reduce toughness"),
    )
    return _mk_static(repl, raw, "no_toughness_reduction")


# --- "Damage can't be prevented." (Leyline of Punishment class — anti-
#     prevention global static.) Scope variants:
#       * bare "damage can't be prevented"
#       * "combat damage can't be prevented"
#       * "damage that would be dealt by <filter> can't be prevented"
#       * "<filter> damage can't be prevented this turn"
@_static_pattern(
    r"^(?:combat )?damage can'?t be prevented(?: this turn)?$"
)
def _damage_cant_be_prevented(m, raw):
    repl = Replacement(
        trigger_event="damage_prevention",
        replacement=UnknownEffect(raw_text="suppress prevention"),
    )
    return _mk_static(repl, raw, "no_prevent")


@_static_pattern(
    r"^damage that would be dealt by ([^.]+?) can'?t be prevented$"
)
def _damage_by_x_cant_be_prevented(m, raw):
    src = _parse_filter_safe(m.group(1))
    repl = Replacement(
        trigger_event="damage_prevention_by_source",
        replacement=UnknownEffect(raw_text="suppress prevention"),
    )
    return Static(
        modification=Modification(kind="replacement_static",
                                  args=(repl, raw, "no_prevent_by", src)),
        raw=raw,
    )


@_static_pattern(
    r"^combat damage that would be dealt by ([^.]+?) can'?t be prevented$"
)
def _combat_damage_by_x_cant_be_prevented(m, raw):
    src = _parse_filter_safe(m.group(1))
    repl = Replacement(
        trigger_event="combat_damage_prevention_by_source",
        replacement=UnknownEffect(raw_text="suppress prevention combat"),
    )
    return Static(
        modification=Modification(kind="replacement_static",
                                  args=(repl, raw, "no_prevent_combat_by", src)),
        raw=raw,
    )


# --- Combat-only self-protection: "Prevent all combat damage that would be
#     dealt to and dealt by ~." Pattern appears bare on pacifism-adjacent
#     auras (Sandskin family) — shows up in parser errors as an unparsed
#     static when not terminated by "this turn".
@_static_pattern(
    r"^prevent all combat damage that would be dealt to and dealt by (?:~|this creature|enchanted creature|equipped creature)$"
)
def _combat_only_self(m, raw):
    repl = Replacement(
        trigger_event="combat_damage_self",
        replacement=Prevent(amount="all", damage_filter=Filter(base="self", targeted=False)),
    )
    return _mk_static(repl, raw, "pacify_self")


# --- Banding reminder text swallower. Bare `Banding` is handled as a
#     keyword. When Scryfall leaves the reminder attached as its own
#     paragraph (rare, but it happens on preconstructed reprints and some
#     debug dumps), we keep the parser green by emitting a static note.
@_static_pattern(
    r"^(?:\()?any creatures with banding[^.]*can attack in a band[^$]*$"
)
def _banding_reminder(m, raw):
    return Static(
        modification=Modification(kind="reminder_text", args=("banding",)),
        raw=raw,
    )


# --- "This creature has banding as long as you control a Plains." —
#     conditional banding grant (Benalish Infantry etc.). Emit as a
#     self-keyword with a condition tag; downstream builders can special-
#     case the conditional.
@_static_pattern(
    r"^(?:~|this creature) has banding as long as you control (?:a|an|the) ([a-z]+)$"
)
def _conditional_banding(m, raw):
    land = m.group(1).lower()
    return Static(
        modification=Modification(kind="conditional_keyword",
                                  args=("banding", f"control_{land}")),
        raw=raw,
    )


# ===========================================================================
# TRIGGER_PATTERNS — damage-prevention-event triggers
# ===========================================================================
# Parser's three-tuple shape: (compiled_regex, event_name, scope).

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # "Whenever damage from a [color] source is prevented this way this turn,
    #  …" (Samite Ministration). MUST come before the bare prevention
    # trigger.
    (re.compile(
        r"^whenever damage from (?:a|an) " + _COLOR_WORD +
        r"(?: or " + _COLOR_WORD + r")? source is prevented this way(?: this turn)?", re.I),
     "colored_damage_prevented", "self"),

    # "Whenever damage that would be dealt to you is prevented, …"
    (re.compile(
        r"^whenever damage that would be dealt to (?:you|~|this creature|[^,]+?) is prevented",
        re.I),
     "damage_to_x_prevented", "self"),

    # "When damage is prevented this way, <effect>." — the tail payoff on
    # Deflecting Palm / Divine Deflection / Blessed Wind family.
    (re.compile(r"^when damage is prevented this way", re.I),
     "damage_prevented_this_way", "self"),

    # "For each 1 damage prevented this way, …" — scaling payoff rider.
    (re.compile(r"^for each 1 damage prevented this way", re.I),
     "per_damage_prevented", "self"),
]


__all__ = ["EFFECT_RULES", "STATIC_PATTERNS", "TRIGGER_PATTERNS"]


# ---------------------------------------------------------------------------
# Self-check — run `python3 scripts/extensions/damage_prevention.py` to
# verify every regex compiles and at least covers its sample phrasings.
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    effect_samples = [
        "prevent the next 3 damage that would be dealt to any target this turn",
        "prevent the next 1 damage that would be dealt to any target this turn",
        "prevent the next x damage that would be dealt to target creature this turn",
        "• prevent the next 2 damage that would be dealt to any target this turn",
        "prevent the next 4 damage that would be dealt this turn to any number of targets",
        "the next time a red source of your choice would deal damage to you this turn, prevent that damage",
        "the next time a source of your choice of the chosen color would deal damage to you this turn, prevent that damage",
        "the next time a source of your choice would deal damage to target creature this turn, that source deals that damage to you instead",
        "the next time a source of your choice would deal damage to you this turn, prevent that damage",
        "the next time target creature would deal damage this turn, prevent that damage",
        "prevent half that damage, rounded up",
        "prevent half that damage, rounded down",
        "prevent all combat damage that would be dealt by target attacking or blocking creature this turn",
        "prevent all noncombat damage that would be dealt to you and creatures you control",
        "prevent all damage that a source of your choice would deal this turn",
        "prevent all damage that black and/or red sources would deal this turn",
        "prevent all combat damage target creature would deal this turn",
        "target creature gains banding until end of turn",
    ]
    static_samples = [
        "all damage that would be dealt to you is dealt to enchanted creature instead",
        "all damage that would be dealt to you and other permanents you control is dealt to this creature instead",
        "all combat damage that would be dealt to you this turn by target unblocked creature is dealt to its controller instead",
        "if a source would deal damage to you or a permanent you control, prevent half that damage, rounded up",
        "if a source would deal damage to an opponent or a permanent an opponent controls, that source deals double that damage to that player or permanent instead",
        "as long as you're the monarch, damage doesn't cause you to lose life",
        "damage doesn't cause that player to lose life",
        "damage dealt to ~ doesn't reduce its toughness",
        "damage can't be prevented",
        "combat damage can't be prevented",
        "damage that would be dealt by this creature can't be prevented",
        "combat damage that would be dealt by creatures you control can't be prevented",
        "prevent all combat damage that would be dealt to and dealt by ~",
        "any creatures with banding, and up to one without, can attack in a band",
        "~ has banding as long as you control a plains",
    ]
    trigger_samples = [
        "whenever damage from a black or red source is prevented this way this turn, you gain that much life",
        "whenever damage that would be dealt to you is prevented, put that many +1/+1 counters on that creature",
        "when damage is prevented this way, ~ deals that much damage to that source's controller",
        "for each 1 damage prevented this way, put a +1/+1 counter on that creature",
    ]

    def _norm(s):
        return s.strip().rstrip(".").lower()

    def _check_effect(s):
        for pat, _ in EFFECT_RULES:
            if pat.match(_norm(s)):
                return True
        return False

    def _check_static(s):
        for pat, _ in STATIC_PATTERNS:
            if pat.match(_norm(s)):
                return True
        return False

    def _check_trigger(s):
        for pat, _, _ in TRIGGER_PATTERNS:
            if pat.match(_norm(s)):
                return True
        return False

    he = sum(1 for s in effect_samples if _check_effect(s))
    hs = sum(1 for s in static_samples if _check_static(s))
    ht = sum(1 for s in trigger_samples if _check_trigger(s))
    print(f"damage_prevention.py: {len(EFFECT_RULES)} effects, "
          f"{len(STATIC_PATTERNS)} statics, "
          f"{len(TRIGGER_PATTERNS)} triggers")
    print(f"  effect  sample hits: {he}/{len(effect_samples)}")
    print(f"  static  sample hits: {hs}/{len(static_samples)}")
    print(f"  trigger sample hits: {ht}/{len(trigger_samples)}")

    # Show which samples MISSED so debugging is easy.
    for s in effect_samples:
        if not _check_effect(s):
            print(f"  MISS effect : {s}")
    for s in static_samples:
        if not _check_static(s):
            print(f"  MISS static : {s}")
    for s in trigger_samples:
        if not _check_trigger(s):
            print(f"  MISS trigger: {s}")
