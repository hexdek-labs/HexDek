#!/usr/bin/env python3
"""Utility-resource mechanic extensions.

This module owns the oracle-text grammar productions for the game's
"utility resource" family: predefined artifact/creature tokens (Treasure,
Food, Clue, Map, Blood, Powerstone, Gold, Lander, Junk), the {E} (Energy)
counter, and the verbs that produce or consume them (Investigate, Connive,
Manifest / Manifest Dread, Plot, Conjure, Discover, Amass, Proliferate,
Explore, Seek, Gift).

Design:

* ``EFFECT_RULES`` is a list of ``(compiled_regex, builder_fn)`` entries in
  the same shape as ``parser.EFFECT_RULES``. Builders return an ``Effect``
  AST node. Where the grammar defines a first-class AST (``CreateToken``),
  we emit that node with the token's flavor carried in ``types``. Where the
  AST has no matching node (Investigate, Connive, {E} production, Plot,
  etc.), we emit ``UnknownEffect`` whose ``raw_text`` is a *canonical* tag
  (``"investigate"``, ``"energy:+2"``, ``"plot"``, …) so downstream code
  can pattern-match on a stable string rather than free-form oracle prose.

* ``STATIC_PATTERNS`` mirrors ``equipment_aura.py`` — builders take ``(m,
  raw)`` and return a ``Static`` / ``Keyword`` / ``None`` to extend
  ``parse_static``. Used here for "spend only mana produced by treasures",
  "activated abilities of foods you control cost {1} less", and similar
  token-flavor static modifiers.

* ``apply_extensions(parser_module)`` patches a live ``parser`` module in
  place — appends ``EFFECT_RULES`` and wraps ``parse_static`` to try our
  patterns before its built-in fallthrough. Mirror of the contract in
  ``equipment_aura.py``; callable from a test harness or opt-in runner.

Coverage target: every ``parser_coverage.md`` fragment that mentions one of
our resource verbs or tokens.
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
    CreateToken, Filter, Keyword, Modification, Sequence, Static, UnknownEffect,
    SELF,
)


# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

_NUM = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "x": "x",
}

# Flavors that enter as predefined artifact/creature tokens. Each maps to a
# canonical (types, color) tuple used when emitting CreateToken.
# (Energy is NOT here — it's a counter, not a token.)
_TOKEN_FLAVORS = {
    "treasure":   ("Treasure",   ()),
    "food":       ("Food",       ()),
    "clue":       ("Clue",       ()),
    "map":        ("Map",        ()),
    "blood":      ("Blood",      ()),
    "powerstone": ("Powerstone", ()),
    "gold":       ("Gold",       ()),
    "lander":     ("Lander",     ()),
    "junk":       ("Junk",       ()),
    "shard":      ("Shard",      ()),  # Fomori shard / Role — rare but valid
    "incubator":  ("Incubator",  ()),
}

_FLAVOR_RE = r"(?:" + "|".join(_TOKEN_FLAVORS.keys()) + r")"


def _parse_count(token: str):
    """'three' -> 3, '5' -> 5, 'x' -> 'x', 'a'/'an' -> 1."""
    token = token.strip().lower()
    if token in _NUM:
        val = _NUM[token]
    elif token.isdigit():
        val = int(token)
    else:
        val = token  # e.g. "that many"
    return val


def _build_token(count, flavor: str, tapped: bool = False) -> CreateToken:
    """Construct a CreateToken for a utility-resource flavor."""
    types_map, colors = _TOKEN_FLAVORS[flavor]
    return CreateToken(
        count=count,
        pt=None,
        types=(types_map,),
        color=colors,
        tapped=tapped,
    )


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Token creation: "create [N] [tapped] <flavor> tokens" -----------------
# Covers: "create a treasure token", "create two food tokens",
# "create a tapped powerstone token", "create three treasure tokens",
# "create X blood tokens", "create four treasure tokens".
@_er(
    rf"^create (?P<n>a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)"
    rf"(?: (?P<tapped>tapped))? (?P<flavor>{_FLAVOR_RE}) tokens?(?:\.|$)"
)
def _create_flavor_token(m):
    count = _parse_count(m.group("n"))
    tapped = bool(m.group("tapped"))
    return _build_token(count, m.group("flavor").lower(), tapped=tapped)


# --- "create a number of <flavor> tokens equal to …" -----------------------
@_er(
    rf"^create a number of (?P<flavor>{_FLAVOR_RE}) tokens equal to [^.]+?(?:\.|$)"
)
def _create_flavor_equal_to(m):
    return _build_token("var", m.group("flavor").lower())


# --- "you create a[n] <flavor> token" (trigger body rewording) -------------
@_er(
    rf"^you create (?P<n>a|an|one|two|three|four|five|x|\d+) "
    rf"(?P<flavor>{_FLAVOR_RE}) tokens?(?:\.|$)"
)
def _you_create(m):
    return _build_token(_parse_count(m.group("n")), m.group("flavor").lower())


# --- "its controller creates [N] <flavor> tokens" --------------------------
@_er(
    rf"^its controller creates (?P<n>a|an|one|two|three|four|five|x|\d+) "
    rf"(?P<flavor>{_FLAVOR_RE}) tokens?(?:\.|$)"
)
def _controller_creates(m):
    tok = _build_token(_parse_count(m.group("n")), m.group("flavor").lower())
    # Annotate the controller semantics on the raw node via a Sequence wrapper.
    return Sequence(items=(tok,))


# --- "create a <flavor> token for each …" ----------------------------------
@_er(
    rf"^(?:you create )?a (?P<flavor>{_FLAVOR_RE}) token for each [^.]+?(?:\.|$)"
)
def _create_per_each(m):
    return _build_token("var", m.group("flavor").lower())


# --- "create a tapped <flavor> token for each …" ---------------------------
@_er(
    rf"^(?:you create )?a tapped (?P<flavor>{_FLAVOR_RE}) token for each [^.]+?(?:\.|$)"
)
def _create_tapped_per_each(m):
    return _build_token("var", m.group("flavor").lower(), tapped=True)


# --- "for each <X>, create a <flavor> token" (Brass's Bounty et al.) -------
@_er(
    rf"^for each [^,]+?, create (?P<n>a|an|one|\d+) (?P<flavor>{_FLAVOR_RE}) "
    rf"tokens?(?:\.|$)"
)
def _per_each_prefix(m):
    return _build_token("var", m.group("flavor").lower())


# ---------------------------------------------------------------------------
# Energy counters ({E})
# ---------------------------------------------------------------------------
# Energy is a player-level counter, not a token. The AST has no EnergyEffect
# type yet, so we emit UnknownEffect with canonical tags:
#   "energy:+N"     — gained N energy
#   "energy:+var"   — gained X / "an amount of"
#   "energy:-N"     — paid N energy  (rarely appears as bare effect; usually a cost)

@_er(r"^you get (?P<syms>(?:\{e\}\s*)+)(?:\.|$)")
def _energy_get(m):
    n = len(re.findall(r"\{e\}", m.group("syms"), re.I))
    return UnknownEffect(raw_text=f"energy:+{n}")


@_er(r"^you get x \{e\}(?:\.|$)")
def _energy_get_x(m):
    return UnknownEffect(raw_text="energy:+x")


@_er(r"^you get an amount of \{e\} equal to [^.]+?(?:\.|$)")
def _energy_get_equal(m):
    return UnknownEffect(raw_text="energy:+var")


# "you get {E}{E}, then you may pay …" — compound; the second clause is left
# to the main parser's comma-sequence splitter. This rule just anchors the
# first clause so the sequence parser has something to resume from.
@_er(r"^you get (?P<syms>(?:\{e\}\s*)+), then [^.]+?(?:\.|$)")
def _energy_then(m):
    n = len(re.findall(r"\{e\}", m.group("syms"), re.I))
    return UnknownEffect(raw_text=f"energy:+{n}")


# ---------------------------------------------------------------------------
# Keyword-verbs: Investigate / Connive / Manifest / Plot / Discover /
#                Amass / Proliferate / Explore / Seek / Conjure
# ---------------------------------------------------------------------------
# These verbs carry their own comprehensive-rules definition. We keep them as
# UnknownEffect with a canonical tag so the AST stays typed-ish without
# forcing a new dataclass per mechanic.

@_er(r"^investigate(?: (?P<n>\d+))?(?:\.|$)")
def _investigate(m):
    n = m.group("n") or "1"
    return UnknownEffect(raw_text=f"investigate:{n}")


@_er(r"^(?:its controller )?investigates(?:\.|$)")
def _investigates_verb(m):
    return UnknownEffect(raw_text="investigate:1")


@_er(r"^connive(?: (?P<n>\d+))?(?:\.|$)")
def _connive(m):
    n = m.group("n") or "1"
    return UnknownEffect(raw_text=f"connive:{n}")


@_er(r"^target creature connives(?:\.|$)")
def _target_connives(m):
    return UnknownEffect(raw_text="connive:target:1")


# Manifest — place the top card of library face-down on the battlefield.
@_er(r"^manifest the top card of your library(?:\.|$)")
def _manifest_top(m):
    return UnknownEffect(raw_text="manifest:1")


@_er(r"^manifest dread(?:\.|$)")
def _manifest_dread(m):
    return UnknownEffect(raw_text="manifest_dread:1")


@_er(r"^(?:its controller )?manifests dread(?:\.|$)")
def _controller_manifests_dread(m):
    return UnknownEffect(raw_text="manifest_dread:1")


# Plot — exile from hand with a plot counter; cast later at sorcery speed.
# Seen both as a bare keyword verb ("plot ~") and as a mana-cost form
# ("plot {2}{R}") that lives alongside the cast cost on the card.
@_er(r"^plot(?: (?:~|this (?:card|spell)))?(?:\.|$)")
def _plot_verb(m):
    return UnknownEffect(raw_text="plot")


@_er(r"^plot (?P<cost>\{[^}]+\}(?:\{[^}]+\})*)(?:\.|$)")
def _plot_cost(m):
    return UnknownEffect(raw_text=f"plot:cost={m.group('cost')}")


@_er(r"^you may pay (?P<cost>\{[^}]+\}(?:\{[^}]+\})*) to plot (?:~|this card)(?:\.|$)")
def _may_pay_to_plot(m):
    return UnknownEffect(raw_text=f"plot:may_pay={m.group('cost')}")


# Discover N — cascade-like reveal.
@_er(r"^discover (?P<n>\d+)(?:\.|$)")
def _discover(m):
    return UnknownEffect(raw_text=f"discover:{m.group('n')}")


@_er(r"^discover x, where x is [^.]+?(?:\.|$)")
def _discover_x(m):
    return UnknownEffect(raw_text="discover:x")


@_er(r"^whenever you discover, discover again[^.]*?(?:\.|$)")
def _rediscover(m):
    return UnknownEffect(raw_text="discover:again")


# Amass [type] N — put N counters on an army (or create a 0/0 army first).
@_er(r"^amass (?P<what>[a-z]+) (?P<n>\d+)(?:\.|$)")
def _amass_named(m):
    return UnknownEffect(raw_text=f"amass:{m.group('what').lower()}:{m.group('n')}")


@_er(r"^amass (?P<n>\d+)(?:\.|$)")
def _amass_num(m):
    return UnknownEffect(raw_text=f"amass:army:{m.group('n')}")


@_er(r"^amass an army(?:\.|$)")
def _amass_army(m):
    return UnknownEffect(raw_text="amass:army:1")


# Proliferate — add a counter of each kind already present on any number of
# permanents / players.
@_er(r"^proliferate(?:\.|$)")
def _proliferate(m):
    return UnknownEffect(raw_text="proliferate")


# Explore — "~ explores" / "target creature explores".
@_er(r"^(?:~|this creature) explores(?:\.|$)")
def _self_explores(m):
    return UnknownEffect(raw_text="explore:self")


@_er(r"^target creature explores(?:\.|$)")
def _target_explores(m):
    return UnknownEffect(raw_text="explore:target")


# Seek — "seek a <filter> card" (library search without shuffling).
@_er(r"^seek (?P<what>a|an|\d+) (?P<body>[^.]+?card[^.]*?)(?:\.|$)")
def _seek(m):
    return UnknownEffect(raw_text=f"seek:{m.group('body').strip()}")


# Conjure — create a non-physical (digital) card with a specified name.
@_er(r"^conjure a card named (?P<card>[^.]+?) into your hand(?:\.|$)")
def _conjure_named(m):
    return UnknownEffect(raw_text=f"conjure:named:{m.group('card').strip()}")


@_er(r"^conjure (?P<n>a|an|\d+) cards? named (?P<card>[^.]+?) into [^.]+?(?:\.|$)")
def _conjure_n_named(m):
    n = _parse_count(m.group("n"))
    return UnknownEffect(
        raw_text=f"conjure:n={n}:named:{m.group('card').strip()}"
    )


@_er(r"^conjure a duplicate of (?:the top card of [^.]+?|target [^.]+?|~)(?:\.|$)")
def _conjure_duplicate(m):
    return UnknownEffect(raw_text="conjure:duplicate")


# Gift — "gift a <flavor>" rider; the body of the bonus lives elsewhere, we
# just capture that the gift-clause is recognized.
@_er(rf"^gift (?P<n>a|an) (?P<flavor>{_FLAVOR_RE})(?:\.|$)")
def _gift(m):
    return UnknownEffect(raw_text=f"gift:{m.group('flavor').lower()}")


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# Card-wide statics that touch utility-resource flavor.
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Bare keyword-verb one-liners -----------------------------------------
# parser.parse_ability only promotes a parse_effect result into a Static
# when it's NOT an UnknownEffect. Our resource verbs (investigate,
# proliferate, manifest dread, plot, …) emit UnknownEffect because the AST
# has no dedicated node — so a standalone verb line slips through to None.
# STATIC_PATTERNS runs BEFORE the spell-effect fallback (via the patched
# parse_static), so a Keyword return here catches bare verbs as abilities.

@_sp(r"^investigate(?:\s+(?P<n>\d+))?\s*$")
def _bare_investigate(m, raw):
    return Keyword(name="investigate",
                   args=((m.group("n") or "1"),), raw=raw)


@_sp(r"^proliferate\s*$")
def _bare_proliferate(m, raw):
    return Keyword(name="proliferate", raw=raw)


@_sp(r"^manifest dread\s*$")
def _bare_manifest_dread(m, raw):
    return Keyword(name="manifest dread", raw=raw)


@_sp(r"^manifest the top card of your library\s*$")
def _bare_manifest_top(m, raw):
    return Keyword(name="manifest", raw=raw)


# Plot as a standalone keyword on a card face (e.g. "Plot {2}{R}" printed
# under the mana cost). When it appears in parsed oracle text it's a bare
# keyword ability, so emit a Keyword node carrying the cost.
@_sp(r"^plot\s*(?P<cost>\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _bare_plot_cost(m, raw):
    return Keyword(name="plot", args=(m.group("cost"),), raw=raw)


@_sp(r"^plot\s*$")
def _bare_plot(m, raw):
    return Keyword(name="plot", raw=raw)


@_sp(r"^discover\s+(?P<n>\d+)\s*$")
def _bare_discover(m, raw):
    return Keyword(name="discover", args=(m.group("n"),), raw=raw)


@_sp(r"^amass\s+(?P<what>[a-z]+)\s+(?P<n>\d+)\s*$")
def _bare_amass_named(m, raw):
    return Keyword(name="amass", args=(m.group("what"), m.group("n")), raw=raw)


@_sp(r"^amass\s+(?P<n>\d+)\s*$")
def _bare_amass(m, raw):
    return Keyword(name="amass", args=("army", m.group("n")), raw=raw)


@_sp(r"^amass an army\s*$")
def _bare_amass_army(m, raw):
    return Keyword(name="amass", args=("army", "1"), raw=raw)


# "You get {E}{E}." as a standalone spell-ability line (common on Kaladesh
# block cards). Emit a Keyword to survive parser.parse_ability's
# "no UnknownEffect in the spell-effect slot" filter.
@_sp(r"^you get (?P<syms>(?:\{e\}\s*)+)\s*$")
def _bare_energy_get(m, raw):
    n = len(re.findall(r"\{e\}", m.group("syms"), re.I))
    return Keyword(name="energy_get", args=(n,), raw=raw)


@_sp(r"^you get x \{e\}\s*$")
def _bare_energy_get_x(m, raw):
    return Keyword(name="energy_get", args=("x",), raw=raw)


# Connive as a bare line ("~ connives." / "target creature connives.")
@_sp(r"^(?:~|this creature) connives\s*$")
def _bare_self_connives(m, raw):
    return Keyword(name="connive", args=("self",), raw=raw)


@_sp(r"^target creature connives\s*$")
def _bare_target_connives(m, raw):
    return Keyword(name="connive", args=("target",), raw=raw)


@_sp(r"^connive(?:\s+(?P<n>\d+))?\s*$")
def _bare_connive(m, raw):
    return Keyword(name="connive", args=((m.group("n") or "1"),), raw=raw)


# "spend only mana produced by treasures to cast this spell"
@_sp(rf"^spend (?:only )?mana produced by (?P<flavor>{_FLAVOR_RE})s "
     r"to cast (?:this spell|~)\s*$")
def _spend_only_flavor(m, raw):
    return Static(
        modification=Modification(
            kind="flavor_mana_restriction",
            args=(m.group("flavor").lower(),),
        ),
        raw=raw,
    )


# "activated abilities of foods you control cost {N} less to activate"
@_sp(rf"^activated abilities of (?P<flavor>{_FLAVOR_RE})s you control "
     r"cost \{(?P<n>\d+)\} less(?: to activate)?\s*$")
def _flavor_ability_cost_reduce(m, raw):
    return Static(
        modification=Modification(
            kind="flavor_activation_discount",
            args=(m.group("flavor").lower(), int(m.group("n"))),
        ),
        raw=raw,
    )


# "each creature you control that's a food or …" — type-add static rider.
@_sp(rf"^each creature you control that'?s a (?P<flavor>{_FLAVOR_RE})"
     r"(?: or [a-z ]+)?\s*[^.]*$")
def _creature_is_flavor(m, raw):
    return Static(
        modification=Modification(
            kind="flavor_tribal_rider",
            args=(m.group("flavor").lower(), raw),
        ),
        raw=raw,
    )


# "when there are five or more plot counters on …" — plot-threshold static.
@_sp(r"^when there are (?P<n>[a-z]+|\d+) or more plot counters on [^.]+\s*$")
def _plot_counter_threshold(m, raw):
    n = _parse_count(m.group("n"))
    return Static(
        modification=Modification(
            kind="plot_counter_threshold",
            args=(n,),
        ),
        raw=raw,
    )


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def apply_extensions(parser_module) -> None:
    """Patch a ``parser`` module in place.

    Appends our ``EFFECT_RULES`` and wraps ``parse_static`` so our
    ``STATIC_PATTERNS`` fire before the built-in fallthrough. Mirrors the
    ``equipment_aura.apply_extensions`` contract so a runner can call both.
    """
    parser_module.EFFECT_RULES.extend(EFFECT_RULES)

    original_parse_static = parser_module.parse_static

    def patched_parse_static(text: str):
        cleaned = text.strip().rstrip(".")
        low = cleaned.lower()
        for pat, builder in STATIC_PATTERNS:
            m = pat.match(low)
            if not m:
                continue
            try:
                result = builder(m, text)
            except Exception:
                continue
            if result is not None:
                return result
        return original_parse_static(text)

    parser_module.parse_static = patched_parse_static


__all__ = [
    "EFFECT_RULES",
    "STATIC_PATTERNS",
    "apply_extensions",
]
