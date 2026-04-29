#!/usr/bin/env python3
"""Life-change trigger & payoff extensions.

Family: LIFE-CHANGE TRIGGERS AND PAYOFFS — abilities that fire from gaining
or losing life, replacement effects that substitute the life event, and
static abilities that scale by life total or life-delta.

Exported tables (shape matches the parser's merge points):

- ``TRIGGER_PATTERNS`` — ``(compiled_regex, event_name, scope)`` entries
  merged into ``parser._TRIGGER_PATTERNS``. Covers:
    * "whenever you gain life, …"
    * "whenever you gain life from a [filter] source, …"
    * "whenever you gain N or more life, …"
    * "whenever an opponent loses life, …"
    * "whenever you lose life, …"
    * "whenever a player loses life, …"
    * "whenever an opponent gains life, …"
    * "whenever a player gains life, …"
    * "whenever ~ deals damage to a player, you gain that much life"
      (lifelink-equivalent rider — handled as a specific trigger shape so
      the parser recognises the combined trigger+effect).

- ``STATIC_PATTERNS`` — ``(compiled_regex, builder_fn)`` entries consulted
  by ``parser.parse_static`` before the generic fallthrough. Covers:
    * "as long as you have N or more life, [body]"
    * "as long as your life total is [op] [N/starting], [body]"
    * "as long as an opponent has N or more life, …"
    * "as long as an opponent's life total is …, …"
    * "if you would gain life, you gain that much life plus N instead"
    * "if you would gain life, you gain twice that much life instead"
    * "if you would gain life, [substitution] instead"
    * "if a player / an opponent would gain life, …"
    * "players can't gain life" / "your opponents can't gain life"

- ``EFFECT_RULES`` — ``(compiled_regex, builder_fn)`` entries appended to
  ``parser.EFFECT_RULES``. Covers body-level life phrases that are common
  payoffs of a life-change trigger:
    * "you gain life equal to …"
    * "you lose life equal to …"
    * "target opponent loses that much life"
    * "each opponent loses N life"
    * "its controller gains life equal to its power/toughness"
    * "your life total becomes that number"

Ordering: most specific regex first, most general last. The parser's
first-match-wins loop relies on this ordering so that "whenever you gain
life from a land source" fires before plain "whenever you gain life".

`~` is already normalised in oracle text. Self-aliases are kept narrow
(only the canonical "this creature" / "~") because life triggers are
whole-text-anchored and don't need Equipment/Aura subjects.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# Allow this file to live in scripts/extensions/ and still import the AST
# nodes from scripts/mtg_ast.py.
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

from mtg_ast import (  # noqa: E402
    Condition, Damage, Discard, Draw, Effect, Filter, GainLife, LoseLife,
    Modification, Replacement, Static, UnknownEffect,
    EACH_OPPONENT, EACH_PLAYER, SELF, TARGET_OPPONENT, TARGET_PLAYER,
)


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------
# Parser's three-tuple shape: (compiled_regex, event_name, scope).
# Scope is advisory metadata — the parser only consumes the regex match +
# event_name. We use "self" when the capturing actor is fixed (you / ~)
# and "actor" when group(1) names who triggered it.

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # ----- "whenever you gain life from a [filter] source, …" -----
    # Must come BEFORE the bare "whenever you gain life".
    (re.compile(r"^whenever you gain life from (?:a|an|the) [^,.]+source", re.I),
     "gain_life_from_source", "self"),

    # ----- "whenever you gain N or more life, …" (threshold gain) -----
    (re.compile(r"^whenever you gain (\d+) or more life", re.I),
     "gain_n_or_more_life", "self"),

    # ----- "whenever an opponent who controls [X] gains life …" -----
    # Highly specific — Ashnod / "who controls an artifact named ~" shape.
    (re.compile(r"^whenever an opponent who controls [^,.]+ gains life", re.I),
     "filtered_opponent_gains_life", "self"),

    # ----- "whenever an opponent gains life, …" -----
    (re.compile(r"^whenever (?:an|each) opponent gains life", re.I),
     "opponent_gains_life", "self"),

    # ----- "whenever a player gains life, …" -----
    (re.compile(r"^whenever (?:a|another) player gains life", re.I),
     "player_gains_life", "all"),

    # ----- "whenever you gain life, …" (bare) -----
    (re.compile(r"^whenever you gain life", re.I),
     "gain_life", "self"),

    # ----- "whenever an opponent loses life, …" -----
    (re.compile(r"^whenever (?:an|each) opponent loses life", re.I),
     "opponent_loses_life", "self"),

    # ----- "whenever a player loses life, …" -----
    (re.compile(r"^whenever (?:a|another) player loses life", re.I),
     "player_loses_life", "all"),

    # ----- "whenever you lose life, …" -----
    (re.compile(r"^whenever you lose life", re.I),
     "you_lose_life", "self"),

    # ----- "whenever ~ / this creature deals damage to a player,
    # you gain that much life" — lifelink-style rider without the keyword.
    # Captured as a compound trigger so the effect tail ("you gain that
    # much life") is left for parse_effect to consume.
    (re.compile(
        r"^whenever (?:~|this creature) deals damage to (?:a player|an opponent)",
        re.I),
     "self_deals_damage_player", "self"),

    # ----- "at the beginning of your end step, if you have N or more life, …"
    # The parser already matches "at the beginning of your end step" — this
    # entry specifically binds the intervening-if life-total shape so the
    # effect tail is parsed as the body. It's positioned before the generic
    # phase trigger in the parser's list (extensions win first).
    (re.compile(
        r"^at the beginning of your end step, if you have (\d+) or more life",
        re.I),
     "end_step_life_threshold", "self"),

    # ----- "at the beginning of your end step, if you gained N or more
    # life this turn, …" — life-delta-per-turn payoff.
    (re.compile(
        r"^at the beginning of (?:your|each) end step, if you gained (\d+) or more life this turn",
        re.I),
     "end_step_gained_n_life_this_turn", "self"),

    # ----- "at the beginning of your upkeep, if a player has more life
    # than each other player, …" — high-life upkeep trigger.
    (re.compile(
        r"^at the beginning of your upkeep, if a player has more life than each other player",
        re.I),
     "upkeep_life_leader", "self"),
]


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# ---------------------------------------------------------------------------
# Parser's shape: (compiled_regex, builder(match, raw_text) -> Static | None)
# Patterns are matched against the LOWERCASED cleaned text; builders receive
# both the match and the original-case raw string for round-tripping.

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "as long as you have N or more life, [body]" --------------------------
@_sp(r"^as long as you have (\d+) or more life,\s*(?P<body>.+)$")
def _long_you_have_n_life(m, raw):
    n = int(m.group(1))
    return Static(
        condition=Condition(kind="life_threshold",
                            args=("you", ">=", n)),
        modification=Modification(kind="life_gated_static",
                                  args=(m.group("body").strip(),)),
        raw=raw,
    )


# --- "as long as you have at least N more life than your starting life
#      total, [body]" — life-delta static (Ajani, Mentor of Heroes-style).
@_sp(
    r"^as long as you have at least (\d+) (?:more )?life more than "
    r"your starting life total,\s*(?P<body>.+)$"
)
def _long_life_delta(m, raw):
    n = int(m.group(1))
    return Static(
        condition=Condition(kind="life_delta_threshold",
                            args=("you", ">=", n)),
        modification=Modification(kind="life_gated_static",
                                  args=(m.group("body").strip(),)),
        raw=raw,
    )


# --- "as long as your life total is greater than your starting life total,
#      [body]" — ahead-of-starting qualitative static.
@_sp(
    r"^as long as your life total is (greater than|less than|equal to) "
    r"your starting life total,\s*(?P<body>.+)$"
)
def _long_life_vs_start(m, raw):
    op_map = {"greater than": ">", "less than": "<", "equal to": "=="}
    return Static(
        condition=Condition(kind="life_vs_starting",
                            args=("you", op_map[m.group(1)])),
        modification=Modification(kind="life_gated_static",
                                  args=(m.group("body").strip(),)),
        raw=raw,
    )


# --- "as long as your life total is less than or equal to half your starting
#      life total, [body]" — perilous-play static.
@_sp(
    r"^as long as your life total is (less than or equal to|greater than or equal to) "
    r"half your starting life total,\s*(?P<body>.+)$"
)
def _long_life_half_start(m, raw):
    op = "<=" if m.group(1).startswith("less") else ">="
    return Static(
        condition=Condition(kind="life_vs_half_starting",
                            args=("you", op)),
        modification=Modification(kind="life_gated_static",
                                  args=(m.group("body").strip(),)),
        raw=raw,
    )


# --- "as long as an opponent has N or more life, [body]" -------------------
@_sp(r"^as long as an opponent has (\d+) or more life,\s*(?P<body>.+)$")
def _long_opp_has_n_life(m, raw):
    n = int(m.group(1))
    return Static(
        condition=Condition(kind="life_threshold",
                            args=("opponent", ">=", n)),
        modification=Modification(kind="life_gated_static",
                                  args=(m.group("body").strip(),)),
        raw=raw,
    )


# --- "as long as an opponent's life total is less than half their starting
#      life total, [body]" — close-the-game static.
@_sp(
    r"^as long as an opponent'?s life total is "
    r"(less than or equal to|less than|greater than) "
    r"half their starting life total,\s*(?P<body>.+)$"
)
def _long_opp_life_half_start(m, raw):
    op_map = {"less than or equal to": "<=",
              "less than": "<",
              "greater than": ">"}
    return Static(
        condition=Condition(kind="life_vs_half_starting",
                            args=("opponent", op_map[m.group(1)])),
        modification=Modification(kind="life_gated_static",
                                  args=(m.group("body").strip(),)),
        raw=raw,
    )


# --- "as long as you have N or more life AND an opponent has M or less life,
#      [body]" — two-sided gate.
@_sp(
    r"^as long as you have (\d+) or more life and an opponent has "
    r"(\d+) or less life,\s*(?P<body>.+)$"
)
def _long_two_sided(m, raw):
    return Static(
        condition=Condition(
            kind="life_threshold_both",
            args=(("you", ">=", int(m.group(1))),
                  ("opponent", "<=", int(m.group(2)))),
        ),
        modification=Modification(kind="life_gated_static",
                                  args=(m.group("body").strip(),)),
        raw=raw,
    )


# --- Replacement effects: "if you would gain life, …" ----------------------

# "if you would gain life, you gain that much life plus N instead"
@_sp(
    r"^if you would gain life, you gain that much life plus (\d+) instead\s*$"
)
def _repl_gain_plus_n(m, raw):
    n = int(m.group(1))
    return Static(
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event="gain_life",
                replacement=GainLife(amount=f"x+{n}"),
            ), raw),
        ),
        raw=raw,
    )


# "if you would gain life, you gain twice/thrice that much life instead"
@_sp(
    r"^if you would gain life, you gain (twice|thrice|three times|four times) "
    r"that much life instead\s*$"
)
def _repl_gain_multiplier(m, raw):
    factor_map = {"twice": 2, "thrice": 3, "three times": 3, "four times": 4}
    f = factor_map[m.group(1)]
    return Static(
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event="gain_life",
                replacement=GainLife(amount=f"x*{f}"),
            ), raw),
        ),
        raw=raw,
    )


# "if you would gain life while you have N or less life, you gain twice
# that much life instead" — Ajani's Pridemate-style conditional multiplier.
@_sp(
    r"^if you would gain life while you have (\d+) or less life, "
    r"you gain (twice|thrice) that much life instead\s*$"
)
def _repl_gain_conditional_multiplier(m, raw):
    factor_map = {"twice": 2, "thrice": 3}
    return Static(
        condition=Condition(
            kind="life_threshold",
            args=("you", "<=", int(m.group(1))),
        ),
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event="gain_life",
                replacement=GainLife(amount=f"x*{factor_map[m.group(2)]}"),
            ), raw),
        ),
        raw=raw,
    )


# "if you would gain life, draw that many cards instead"
@_sp(r"^if you would gain life, draw that many cards instead\s*$")
def _repl_gain_to_draw(m, raw):
    return Static(
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event="gain_life",
                replacement=Draw(count="x", target=SELF),
            ), raw),
        ),
        raw=raw,
    )


# "if an opponent would gain life, that player loses that much life instead"
@_sp(
    r"^if an opponent would gain life, that player loses that much life instead\s*$"
)
def _repl_opp_gain_flipped(m, raw):
    return Static(
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event="opponent_gains_life",
                replacement=LoseLife(amount="x", target=TARGET_OPPONENT),
            ), raw),
        ),
        raw=raw,
    )


# "if a player would gain life, that player gains no life instead"
@_sp(
    r"^if (?:a|an) (?:player|opponent) would gain life, "
    r"(?:that player|they) gains? no life instead\s*$"
)
def _repl_gain_nullified(m, raw):
    return Static(
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event="any_gain_life",
                replacement=UnknownEffect(raw_text="no life gained"),
            ), raw),
        ),
        raw=raw,
    )


# "if you would lose life, [substitution] instead" — generic catch.
# Accepts either order: "..., X instead" or "..., instead X".
@_sp(
    r"^if you would lose life,\s*"
    r"(?:instead\s+(?P<body1>[^.]+?)|(?P<body2>[^.]+?)\s+instead)\s*$"
)
def _repl_you_lose(m, raw):
    body = (m.group("body1") or m.group("body2") or "").strip()
    return Static(
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event="you_lose_life",
                replacement=UnknownEffect(raw_text=body),
            ), raw),
        ),
        raw=raw,
    )


# "if an opponent would lose life during your turn, they lose twice that
# much life instead" — aggro doubler.
@_sp(
    r"^if an opponent would lose life(?: during your turn)?, "
    r"(?:they|that player) loses? (twice|thrice) that much life instead\s*$"
)
def _repl_opp_lose_doubler(m, raw):
    factor_map = {"twice": 2, "thrice": 3}
    return Static(
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event="opponent_loses_life",
                replacement=LoseLife(
                    amount=f"x*{factor_map[m.group(1)]}",
                    target=TARGET_OPPONENT,
                ),
            ), raw),
        ),
        raw=raw,
    )


# "players can't gain life" / "your opponents can't gain life"
@_sp(r"^(?P<who>players|your opponents|opponents) can'?t gain life\s*$")
def _cant_gain_life(m, raw):
    who = m.group("who").lower()
    scope = "all" if who == "players" else "opponents"
    return Static(
        modification=Modification(
            kind="replacement_static",
            args=(Replacement(
                trigger_event=f"{scope}_gain_life",
                replacement=UnknownEffect(raw_text="no life gained"),
            ), raw),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------
# Body-of-effect patterns — used inside triggered/activated abilities.
# Parser's shape: (compiled_regex, builder(match) -> Effect).

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# "you gain life equal to …" — very common payoff. We keep the "equal to"
# tail opaque (amount="var") but record the source string for clustering.
@_er(r"^you gain life equal to ([^.]+?)(?:\.|$)")
def _you_gain_life_equal_to(m):
    return GainLife(amount="var", target=SELF)


# "target player gains life equal to …"
@_er(r"^target player gains life equal to ([^.]+?)(?:\.|$)")
def _target_player_gains_equal(m):
    return GainLife(amount="var", target=TARGET_PLAYER)


# "its controller gains life equal to its power/toughness/mana value"
@_er(r"^its controller gains life equal to ([^.]+?)(?:\.|$)")
def _its_controller_gains(m):
    return GainLife(amount="var", target=TARGET_PLAYER)


# "you lose life equal to …"
@_er(r"^you lose life equal to ([^.]+?)(?:\.|$)")
def _you_lose_life_equal_to(m):
    return LoseLife(amount="var", target=SELF)


# "target opponent loses that much life" — common life-drain tail.
@_er(r"^target opponent loses that much life(?:\.|$)")
def _opp_loses_that_much(m):
    return LoseLife(amount="var", target=TARGET_OPPONENT)


# "each opponent loses that much life"
@_er(r"^each opponent loses that much life(?:\.|$)")
def _each_opp_loses_that_much(m):
    return LoseLife(amount="var", target=EACH_OPPONENT)


# "that player loses life equal to …"
@_er(r"^that player loses life equal to ([^.]+?)(?:\.|$)")
def _that_player_loses_equal(m):
    return LoseLife(amount="var", target=TARGET_PLAYER)


# "you gain that much life" — bare "that much" tail (used after a set-up
# clause earlier in the same ability, e.g. lifelink-style rider).
@_er(r"^you gain that much life(?:\.|$)")
def _you_gain_that_much(m):
    return GainLife(amount="var", target=SELF)


# "each opponent loses N life"
@_er(r"^each opponent loses (\d+) life(?:\.|$)")
def _each_opp_loses_n(m):
    return LoseLife(amount=int(m.group(1)), target=EACH_OPPONENT)


# "your life total becomes that number" / "your life total becomes N"
@_er(r"^your life total becomes that number(?:\.|$)")
def _life_total_becomes_that(m):
    return UnknownEffect(raw_text="set_life_var")


@_er(r"^your life total becomes (\d+)(?:\.|$)")
def _life_total_becomes_n(m):
    return UnknownEffect(raw_text=f"set_life_{m.group(1)}")


# "discard that many cards" — pairs with "whenever you lose life" triggers.
@_er(r"^discard that many cards(?:\.|$)")
def _discard_that_many(m):
    return Discard(count="var", target=SELF)


# "draw that many cards" — pairs with gain-life triggers.
@_er(r"^draw that many cards(?:\.|$)")
def _draw_that_many(m):
    return Draw(count="var", target=SELF)


# "put that many +1/+1 counters on …" — gain-life payoff (Ajani's Pridemate
# scaled variant). We return an UnknownEffect since the counter extension
# may already cover this; leaving the tail typed so downstream can split.
@_er(r"^put that many \+1/\+1 counters on ([^.]+?)(?:\.|$)")
def _put_that_many_counters(m):
    return UnknownEffect(raw_text=f"counters_var on {m.group(1).strip()}")


__all__ = ["TRIGGER_PATTERNS", "STATIC_PATTERNS", "EFFECT_RULES"]


# ---------------------------------------------------------------------------
# Smoke test: run directly to verify every pattern compiles and matches its
# own exemplar phrase. Mirrors the pattern used by combat_triggers.py.
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    trigger_samples = [
        "whenever you gain life, put a +1/+1 counter on this creature.",
        "whenever you gain life from a land source, draw a card.",
        "whenever you gain 3 or more life, create a 1/1 spirit.",
        "whenever an opponent loses life, you gain that much life.",
        "whenever you lose life, discard that many cards.",
        "whenever a player loses life, scry 1.",
        "whenever an opponent gains life, you may pay {r}.",
        "whenever an opponent who controls an artifact named ~ gains life, draw a card.",
        "whenever ~ deals damage to a player, you gain that much life.",
        "at the beginning of your end step, if you have 40 or more life, you win the game.",
        "at the beginning of your end step, if you gained 3 or more life this turn, create a 3/3.",
        "at the beginning of your upkeep, if a player has more life than each other player, draw a card.",
    ]
    static_samples = [
        "as long as you have 30 or more life, this creature gets +5/+5 and has flying.",
        "as long as you have at least 7 life more than your starting life total, creatures you control have flying.",
        "as long as your life total is greater than your starting life total, ~ gets +1/+1 and has vigilance.",
        "as long as your life total is less than or equal to half your starting life total, ~ has indestructible.",
        "as long as an opponent has 10 or more life, ~ has menace.",
        "as long as an opponent's life total is less than half their starting life total, ~ has indestructible.",
        "as long as you have 30 or more life and an opponent has 10 or less life, this creature gets +5/+5.",
        "if you would gain life, you gain that much life plus 1 instead.",
        "if you would gain life, you gain twice that much life instead.",
        "if you would gain life while you have 5 or less life, you gain twice that much life instead.",
        "if you would gain life, draw that many cards instead.",
        "if an opponent would gain life, that player loses that much life instead.",
        "if a player would gain life, that player gains no life instead.",
        "if you would lose life, instead put one of your shields into your hand.",
        "if an opponent would lose life during your turn, they lose twice that much life instead.",
        "players can't gain life.",
        "your opponents can't gain life.",
    ]
    effect_samples = [
        "you gain life equal to its power.",
        "you gain life equal to the damage dealt this way.",
        "target player gains life equal to its toughness.",
        "its controller gains life equal to its power.",
        "you lose life equal to its mana value.",
        "target opponent loses that much life.",
        "each opponent loses that much life.",
        "that player loses life equal to the number of cards.",
        "you gain that much life.",
        "each opponent loses 1 life.",
        "your life total becomes that number.",
        "your life total becomes 20.",
        "discard that many cards.",
        "draw that many cards.",
        "put that many +1/+1 counters on this creature.",
    ]

    def _check(samples, rules, label):
        unmatched = []
        for s in samples:
            low = s.lower().rstrip(".")
            hit = None
            for entry in rules:
                if len(entry) == 3:
                    pat, ev, _scope = entry
                    label_val = ev
                else:
                    pat, builder = entry
                    label_val = getattr(builder, "__name__", "?")
                if pat.match(low):
                    hit = label_val
                    break
            if hit is None:
                unmatched.append(s)
            else:
                print(f"OK  [{label:<8s} {hit:<32s}]  {s}")
        return unmatched

    bad = []
    bad += _check(trigger_samples, TRIGGER_PATTERNS, "trigger")
    bad += _check(static_samples, STATIC_PATTERNS, "static")
    bad += _check(effect_samples, EFFECT_RULES, "effect")
    if bad:
        print("\nUNMATCHED:")
        for s in bad:
            print(" ", s)
        raise SystemExit(1)
    total = len(trigger_samples) + len(static_samples) + len(effect_samples)
    print(f"\nAll {total} samples matched.")
