#!/usr/bin/env python3
"""Wave 1a parser phrase-coverage promotions.

Named ``a_*`` so it loads BEFORE ``aa_unknown_hunt.py`` and every other
extension. Because the effect-grammar is first-match-wins at the extension
layer, these handlers preempt the labeled-UnknownEffect stubs emitted by
later-loading extensions (multi_failure, partial_final, etc.) for the
same shapes.

Goal: drain the ~7,000 ``UnknownEffect`` nodes in the AST dataset down
by promoting the top-frequency phrase patterns to structured typed AST
nodes. Each handler below matches a complete effect clause (anchored
``^...$``) and returns:

  - a dedicated typed node (``CreateToken``, ``Choice``, ``AddMana``,
    ``Optional_``, ``Bounce``, ``Destroy``, ``Recurse``, ``Reanimate``,
    ``Tutor``, ``Mill``, ``Reveal``, ``Sacrifice``, ``LoseGame``, ...)
    when the AST schema already has a matching leaf, OR
  - a ``Modification(kind="<label>", args=(...))`` stub when the shape
    doesn't yet have a dedicated leaf but is still structurally
    distinguishable from the surrounding UnknownEffect soup.

Coverage audit: ``python3 scripts/audit_unknown_effects.py`` writes
``data/rules/unknown_effect_audit.md`` with the pre-promotion frequency
table used to pick these shapes.

Non-goals:
  - No new AST node types — all promotions route through existing
    effect nodes or ``Modification`` stubs.
  - No changes to ``scripts/parser.py`` beyond removing obsolete entries
    from ``_UNKNOWN_PROMOTE_FORBIDDEN`` for phrases this file now types.
  - No per-card snowflake handlers — those belong in
    ``scripts/extensions/per_card.py`` (owned by a different agent).

Review constraint: every handler is accompanied by a docstring citing
the audit frequency and the specific example cards driving the shape.
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
    AddMana, Bounce, Choice, Conditional, Condition, CreateToken, CounterMod,
    Damage, Destroy, Discard, Draw, Exile, Fight, Filter, GainLife,
    GrantAbility, LoseGame, LoseLife, ManaCost, ManaSymbol, Mill,
    Modification, Optional_, Reanimate, Recurse, Reveal, Sacrifice, Scry,
    Sequence, Shuffle, TapEffect, Tutor, UntapEffect, UnknownEffect,
    EACH_OPPONENT, EACH_PLAYER, SELF, TARGET_ANY, TARGET_CREATURE,
    TARGET_OPPONENT, TARGET_PLAYER,
)


EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _eff(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


_NUMS = {
    "a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}


def _n(tok: str):
    t = (tok or "").lower()
    if t in _NUMS:
        return _NUMS[t]
    if t.isdigit():
        return int(t)
    return t


# Mana symbol helper (colored pip) → ManaSymbol.
def _pip(letter: str) -> ManaSymbol:
    L = letter.strip().upper()
    if L == "C":
        return ManaSymbol(raw="{C}", color=("C",))
    if L == "S":
        return ManaSymbol(raw="{S}", is_snow=True)
    return ManaSymbol(raw="{" + L + "}", color=(L,))


# ============================================================================
# GROUP B: Dual / triple mana choice — "add {X} or {Y}", "add {X}, {Y}, or {Z}"
# ============================================================================
# Audit frequency:
#   add {R} or {G} (41), add {B} or {R} (39), add {W} or {U} (36),
#   add {U} or {B} (36), add {G} or {W} (35), add {G} or {U} (34),
#   add {B} or {G} (33), add {W} or {B} (33), add {U} or {R} (31),
#   add {R} or {W} (30)  → ~348 nodes total across 5-color dual-land corpus.
#   add {U}, {B}, or {R} (7), add {R}, {G}, or {W} (5), add {G}, {W}, or {U} (5),
#   add {B}, {G}, or {U} (5), add {R}, {W}, or {B} (5), add {W}, {B}, or {G} (4),
#   add {B}, {R}, or {G} (4), add {U}, {R}, or {W} (4), add {W}, {U}, or {B} (3)
#                         → ~50 nodes across tri-land / shard-rock corpus.
# Example cards: Timber Gorge (uncommon), Talisman of Indulgence, Arcane
# Sanctum, Jund-identity rocks.
#
# Shape: ``{T}: Add {X} or {Y}.`` → Choice(pick=1, options=(AddMana(X),
# AddMana(Y))). The typed mana pool at runtime gets ONE pip of the
# chosen color; pilot picks at spend time.
@_eff(r"^add \{([wubrgcs])\}\s+or\s+\{([wubrgcs])\}\.?$")
def _add_two_choice(m):
    a, b = m.group(1), m.group(2)
    return Choice(options=(AddMana(pool=(_pip(a),)),
                            AddMana(pool=(_pip(b),))),
                   pick=1)


@_eff(r"^add \{([wubrgcs])\},\s*\{([wubrgcs])\},?\s+or\s+\{([wubrgcs])\}\.?$")
def _add_three_choice(m):
    a, b, c = m.group(1), m.group(2), m.group(3)
    return Choice(options=(AddMana(pool=(_pip(a),)),
                            AddMana(pool=(_pip(b),)),
                            AddMana(pool=(_pip(c),))),
                   pick=1)


# ``add an additional {G}`` — Leyline of Abundance, Badgermole Cub,
# Nissa, Who Shakes the World. The "additional" qualifier is a static-
# ability rider (not a plain activated), but the effect atom is still
# adding a colored pip. 3 nodes.
@_eff(r"^add an additional \{([wubrgcs])\}\.?$")
def _add_additional(m):
    return AddMana(pool=(_pip(m.group(1)),))


# ``add two mana of different colors`` — Interplanar Beacon, Component
# Pouch, Guild Globe, Firemind Vessel. 4 nodes. Typed-pool approximation:
# two any-color pips (pilot chooses DIFFERENT colors at spend time, same
# as any_color_count=2 pool semantics). The engine's typed pool tracks
# "different" as a soft constraint which is safe to elide at AST time.
@_eff(r"^add two mana of different colou?rs\.?$")
def _add_two_different(m):
    return AddMana(any_color_count=2)


# ============================================================================
# GROUP A: Token creation — "create a <tapped>? N/N color <subtype> creature
# token [with <keywords>]"
# ============================================================================
# Audit frequency: 311 nodes currently labeled "create typed token" by
# multi_failure.py's catch-all. Example cards: Stormchaser's Talent,
# Nimble Thopterist, Doomsday Confluence, Haunted Dead, Hero of Bladehold,
# Avenger of Zendikar, Rukh Egg.
#
# The built-in parser.py rule handles the simple "create a N/N color
# subtype creature token" shape. This rule handles the "tapped" prefix
# + "with keyword-list" suffix that otherwise falls through to the
# catch-all label stub. Returns a typed CreateToken with `tapped=True`,
# `keywords=(list,)`, pt, types, color.

_COLOR_WORD = r"(?:white|blue|black|red|green|colorless)"
_NUM_WORD = (r"(?:a|an|one|two|three|four|five|six|seven|eight|nine|ten|"
             r"x|\d+)")

# Mapping from color word → color short-code ("white" → "W").
_COLOR_MAP = {
    "white": "W", "blue": "U", "black": "B",
    "red": "R", "green": "G", "colorless": "C",
}


def _parse_color_list(s: str) -> tuple[str, ...]:
    """``white`` → ("W",); ``green and blue`` → ("G", "U"); ``white, green, blue`` → ("W","G","U")."""
    tokens = re.findall(r"\b(white|blue|black|red|green|colorless)\b", s.lower())
    return tuple(_COLOR_MAP[t] for t in tokens)


def _parse_keyword_list(s: str) -> tuple[str, ...]:
    """``flying`` → ("flying",); ``flying and haste`` → ("flying","haste"); ``flying, lifelink, and vigilance`` → 3-tuple."""
    # Split on commas or ``and``; filter empties.
    parts = re.split(r"\s*(?:,|and)\s*", s.strip().lower())
    return tuple(p.strip() for p in parts if p.strip())


# "create a tapped N/N color subtype creature token[ with keywords]"
# The built-in parser.py rule handles the bare form (no ``tapped``, no
# ``with <keywords>`` suffix) but captures color+subtype into a single
# ``types`` tuple, losing the color split. Our rule FIRES ONLY when the
# text has either the ``tapped`` prefix or a ``with <keywords>`` rider —
# i.e., where the built-in definitely misses — and emits a fully typed
# CreateToken with separate color / keywords / tapped fields. The bare
# form still routes through the built-in rule (unchanged) so no existing
# CreateToken signatures shift.
@_eff(
    r"^create\s+(a|an|one|two|three|four|five|\d+)\s+"
    r"(?:(tapped)\s+)?"
    r"(\d+)/(\d+)\s+"
    r"(" + _COLOR_WORD + r"(?:\s+(?:and\s+)?" + _COLOR_WORD + r")*)\s+"
    r"([a-z][a-z ]*?)\s+creature tokens?"
    r"(?:\s+with\s+([a-z ,]+?))?"
    r"\.?$"
)
def _create_typed_token(m):
    tapped = bool(m.group(2))
    kws_raw = m.group(7)
    # Skip bare form — the built-in parser.py rule already handles it.
    if not tapped and not kws_raw:
        return None
    count = _n(m.group(1))
    pt = (int(m.group(3)), int(m.group(4)))
    colors = _parse_color_list(m.group(5))
    subtypes_raw = m.group(6).strip()
    subtypes = tuple(t for t in subtypes_raw.split() if t)
    kws = _parse_keyword_list(kws_raw) if kws_raw else ()
    return CreateToken(
        count=count,
        pt=pt,
        color=colors,
        types=subtypes,
        keywords=kws,
        tapped=tapped,
    )


# ``create a token that's a copy of that creature`` — 3 nodes. Copy of
# the nearest creature antecedent.
@_eff(r"^create a token that'?s a copy of (that|this) creature\.?$")
def _create_copy_that_creature(m):
    return CreateToken(count=1,
                        is_copy_of=Filter(base="that_creature", targeted=False))


# ============================================================================
# GROUP C: Self-game-state — "you lose the game"
# ============================================================================
# Audit frequency: "you lose the game" (6). Example cards: Lich's Duel
# Mastery, Marina Vendrell's Grimoire, Lich, Archfiend of the Dross.
# Shape: straightforward LoseGame(target=SELF).
@_eff(r"^you lose the game\.?$")
def _you_lose_game(m):
    return LoseGame(target=SELF)


# ============================================================================
# GROUP D: each-player / each-opponent verbs
# ============================================================================
# "each player mills a card" (6), "each player mills three cards" (6) →
# Mill(count=n, target=EACH_PLAYER). 12 nodes.
@_eff(r"^each player mills\s+(a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+cards?\.?$")
def _each_player_mill(m):
    return Mill(count=_n(m.group(1)), target=EACH_PLAYER)


# "each opponent mills N cards" — variant.
@_eff(r"^each opponent mills\s+(a|an|one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+cards?\.?$")
def _each_opp_mill(m):
    return Mill(count=_n(m.group(1)), target=EACH_OPPONENT)


# "each player reveals the top card of their library" (9). Example cards:
# Game Preserve, Storm Fleet Negotiator, Dakra Mystic, Selvala's Enforcer.
@_eff(r"^each player reveals the top card of their library\.?$")
def _each_player_reveal_top(m):
    return Reveal(source="top_of_library", actor="each_player", count=1)


# "each opponent reveals the top card of their library"
@_eff(r"^each opponent reveals the top card of their library\.?$")
def _each_opp_reveal_top(m):
    return Reveal(source="top_of_library", actor="each_opponent", count=1)


# "each other player sacrifices a creature of their choice" (3). Example:
# Rampage of the Valkyries, Savra Queen of the Golgari, Grave Pact.
@_eff(r"^each other player sacrifices a creature of their choice\.?$")
def _each_other_sac_creature(m):
    return Sacrifice(query=Filter(base="creature", quantifier="one",
                                    targeted=False),
                      actor="each_other_player")


# "any opponent may sacrifice a creature of their choice" (3).
@_eff(r"^any opponent may sacrifice a creature of their choice\.?$")
def _any_opp_may_sac(m):
    return Modification(kind="any_opp_may_sac_creature", args=())


# ============================================================================
# GROUP E: Optional effects — "you may [typed effect]"
# ============================================================================
# These were previously left as UnknownEffect(raw_text="you may draw a card")
# etc. because promoting them affects several golden fixtures (Consecrated
# Sphinx, Reclamation Sage, Eternal Witness, Solemn Simulacrum, Gravedigger,
# etc.). Per Wave-1a spec the golden files are regeneratable; promoting these
# produces a typed ``Optional_(body=<typed>)`` that downstream engines can
# consume directly.

# "you may draw a card" (60). Covers Vedalken Heretic, Solemn Simulacrum,
# Gilt-Leaf Archdruid, Chambered Nautilus, Consecrated Sphinx's inner body.
@_eff(r"^you may draw a card\.?$")
def _may_draw_card(m):
    return Optional_(body=Draw(count=1, target=SELF))


# "you may draw two cards" (5). Consecrated Sphinx, Infiltration Lens,
# Bringer of the Blue Dawn, Drelnoch.
@_eff(r"^you may draw two cards\.?$")
def _may_draw_two(m):
    return Optional_(body=Draw(count=2, target=SELF))


# "you may destroy target artifact or enchantment" (4). Reclamation Sage,
# Foundation Breaker, Aura Shards, Conclave Naturalists.
@_eff(r"^you may destroy target artifact or enchantment\.?$")
def _may_destroy_art_enc(m):
    return Optional_(body=Destroy(
        target=Filter(base="artifact_or_enchantment", quantifier="one",
                       targeted=True)))


# "you may destroy target artifact" (3). Wild Celebrants, Manglehorn,
# Batterhorn.
@_eff(r"^you may destroy target artifact\.?$")
def _may_destroy_artifact(m):
    return Optional_(body=Destroy(
        target=Filter(base="artifact", quantifier="one", targeted=True)))


# "you may return target <type> card from your graveyard to your hand"
# (multiple variants, ~15 nodes combined). Examples: Gravedigger, Eternal
# Witness, Greenwarden of Murasa, Charnelhoard Wurm, Sanctum Gargoyle,
# Treasure Hunter.
@_eff(r"^you may return target creature card from your graveyard to your hand\.?$")
def _may_recurse_creature(m):
    return Optional_(body=Recurse(
        query=Filter(base="creature", quantifier="one", targeted=True),
        from_zone="your_graveyard", destination="hand"))


@_eff(r"^you may return target artifact card from your graveyard to your hand\.?$")
def _may_recurse_artifact(m):
    return Optional_(body=Recurse(
        query=Filter(base="artifact", quantifier="one", targeted=True),
        from_zone="your_graveyard", destination="hand"))


@_eff(r"^you may return target enchantment card from your graveyard to your hand\.?$")
def _may_recurse_enchantment(m):
    return Optional_(body=Recurse(
        query=Filter(base="enchantment", quantifier="one", targeted=True),
        from_zone="your_graveyard", destination="hand"))


@_eff(r"^you may return target card from your graveyard to your hand\.?$")
def _may_recurse_any_card(m):
    return Optional_(body=Recurse(
        query=Filter(base="card", quantifier="one", targeted=True),
        from_zone="your_graveyard", destination="hand"))


# Reanimate variants — "you may return target X card from your graveyard
# to the battlefield" (~5 nodes). Silent Sentinel, Archon of Falling Stars,
# Starfield of Nyx, Sharuum the Hegemon, Bringer of the White Dawn.
@_eff(r"^you may return target enchantment card from your graveyard to the battlefield\.?$")
def _may_reanimate_enc(m):
    return Optional_(body=Reanimate(
        query=Filter(base="enchantment_card", quantifier="one", targeted=True),
        from_zone="your_graveyard", destination="battlefield"))


@_eff(r"^you may return target artifact card from your graveyard to the battlefield\.?$")
def _may_reanimate_art(m):
    return Optional_(body=Reanimate(
        query=Filter(base="artifact_card", quantifier="one", targeted=True),
        from_zone="your_graveyard", destination="battlefield"))


# "you may return target creature to its owner's hand" (4).
@_eff(r"^you may return target creature to its owner'?s hand\.?$")
def _may_bounce_creature(m):
    return Optional_(body=Bounce(target=TARGET_CREATURE))


# "you may return this creature to its owner's hand" (5). Onna cycle.
@_eff(r"^you may return this creature to its owner'?s hand\.?$")
def _may_bounce_self(m):
    return Optional_(body=Bounce(target=Filter(base="self", targeted=False)))


# "you may return a land you control to its owner's hand" (3). Tazeem Raptor
# style.
@_eff(r"^you may return a land you control to its owner'?s hand\.?$")
def _may_bounce_your_land(m):
    return Optional_(body=Bounce(
        target=Filter(base="land", quantifier="one", you_control=True,
                       targeted=False)))


# "you may sacrifice a creature" (5).
@_eff(r"^you may sacrifice a creature\.?$")
def _may_sac_creature(m):
    return Optional_(body=Sacrifice(
        query=Filter(base="creature", you_control=True, targeted=False)))


# "you may sacrifice it" (3).
@_eff(r"^you may sacrifice it\.?$")
def _may_sac_pronoun(m):
    return Optional_(body=Sacrifice(
        query=Filter(base="that_thing", targeted=False)))


# "you may sacrifice another creature or an artifact" (3). Wyll cycle.
@_eff(r"^you may sacrifice another creature or an artifact\.?$")
def _may_sac_creature_or_artifact(m):
    return Optional_(body=Sacrifice(
        query=Filter(base="creature_or_artifact",
                      extra=("another",), you_control=True, targeted=False)))


# "you may create a 1/1 green elf warrior creature token" (3). Prowess of
# the Fair et al. Promoted via Optional_ wrapping a CreateToken.
@_eff(
    r"^you may create (a|an|one|two|three|\d+)\s+"
    r"(?:(tapped)\s+)?"
    r"(\d+)/(\d+)\s+"
    r"(" + _COLOR_WORD + r"(?:\s+(?:and\s+)?" + _COLOR_WORD + r")*)\s+"
    r"([a-z][a-z ]*?)\s+creature tokens?\.?$"
)
def _may_create_token(m):
    count = _n(m.group(1))
    tapped = bool(m.group(2))
    pt = (int(m.group(3)), int(m.group(4)))
    colors = _parse_color_list(m.group(5))
    subtypes = tuple(t for t in m.group(6).strip().split() if t)
    return Optional_(body=CreateToken(count=count, pt=pt, color=colors,
                                        types=subtypes, tapped=tapped))


# ============================================================================
# GROUP F: Tutors with optional "you may" — basic-land ramp / named-card tutor
# ============================================================================
# Audit frequency: 11 + 10 + 6 + 3 = 30 nodes across basic-land ramp variants.

# "you may search your library for a basic land card, put it onto the
# battlefield tapped, then shuffle" (10).
@_eff(r"^you may search your library for a basic land card,\s*put it onto the battlefield tapped,\s*then shuffle\.?$")
def _may_search_basic_tapped(m):
    return Optional_(body=Tutor(
        query=Filter(base="basic_land_card", quantifier="one", targeted=False),
        destination="battlefield_tapped",
        optional=False,  # already wrapped in Optional_
        shuffle_after=True,
    ))


# "you may search your library for a basic land card, put that card onto
# the battlefield tapped, then shuffle" (6).
@_eff(r"^you may search your library for a basic land card,\s*put that card onto the battlefield tapped,\s*then shuffle\.?$")
def _may_search_basic_tapped_v2(m):
    return Optional_(body=Tutor(
        query=Filter(base="basic_land_card", quantifier="one", targeted=False),
        destination="battlefield_tapped",
        optional=False,
        shuffle_after=True,
    ))


# "you may search your library for a basic land card, reveal it, put it
# into your hand, then shuffle" (11).
@_eff(r"^you may search your library for a basic land card,\s*reveal it,\s*put it into your hand,\s*then shuffle\.?$")
def _may_search_basic_reveal_hand(m):
    return Optional_(body=Tutor(
        query=Filter(base="basic_land_card", quantifier="one", targeted=False),
        destination="hand",
        shuffle_after=True,
        reveal=True,
    ))


# "you may search your library for a basic land card, reveal it, then
# shuffle and put that card on top" (7).
@_eff(r"^you may search your library for a basic land card,\s*reveal it,\s*then shuffle and put that card on top\.?$")
def _may_search_basic_top(m):
    return Optional_(body=Tutor(
        query=Filter(base="basic_land_card", quantifier="one", targeted=False),
        destination="top_of_library",
        shuffle_after=True,
        reveal=True,
    ))


# "you may search your library for a plains card, put it onto the
# battlefield tapped, then shuffle" (3). Kor Cartographer family.
@_eff(r"^you may search your library for an?\s+(plains|island|swamp|mountain|forest) card,\s*put it onto the battlefield tapped,\s*then shuffle\.?$")
def _may_search_basic_type(m):
    basic = m.group(1).lower()
    return Optional_(body=Tutor(
        query=Filter(base=basic, quantifier="one", targeted=False,
                      extra=("basic",)),
        destination="battlefield_tapped",
        shuffle_after=True,
    ))


# "you may search your library for a card named ~, reveal it, put it into
# your hand, then shuffle" (8). Screaming Seahawk, Growth-Chamber Guardian,
# Avarax, Tempest Hawk.
@_eff(r"^you may search your library for a card named ~,\s*reveal it,\s*put it into your hand,\s*then shuffle\.?$")
def _may_search_named_self(m):
    return Optional_(body=Tutor(
        query=Filter(base="card_named_self", quantifier="one", targeted=False),
        destination="hand",
        shuffle_after=True,
        reveal=True,
    ))


# "you may search your library for up to three cards named ~, reveal them,
# put them into your hand, then shuffle" (4). Squadron Hawk family.
@_eff(r"^you may search your library for up to\s+(one|two|three|four|five|\d+)\s+cards named ~,\s*reveal them,\s*put them into your hand,\s*then shuffle\.?$")
def _may_search_named_self_n(m):
    return Optional_(body=Tutor(
        query=Filter(base="card_named_self", quantifier="up_to_n",
                      count=_n(m.group(1)), targeted=False),
        destination="hand",
        shuffle_after=True,
        reveal=True,
    ))


# "you may search your library for any number of cards named ~, reveal them,
# put them into your hand, then shuffle" (3).
@_eff(r"^you may search your library for any number of cards named ~,\s*reveal them,\s*put them into your hand,\s*then shuffle\.?$")
def _may_search_named_self_any(m):
    return Optional_(body=Tutor(
        query=Filter(base="card_named_self", quantifier="any",
                      targeted=False),
        destination="hand",
        shuffle_after=True,
        reveal=True,
    ))


# "you may search your library for a ~ card, reveal it, then shuffle and
# put that card on top" (3). Harbinger cycle.
@_eff(r"^you may search your library for a ~ card,\s*reveal it,\s*then shuffle and put that card on top\.?$")
def _may_search_named_self_top(m):
    return Optional_(body=Tutor(
        query=Filter(base="card_named_self", quantifier="one", targeted=False),
        destination="top_of_library",
        shuffle_after=True,
        reveal=True,
    ))


# "you may search your library for a merfolk card, reveal it, then shuffle
# and put that card on top" (2). Merrow Harbinger, Forerunner.
@_eff(r"^you may search your library for an?\s+([a-z][a-z ]+?)\s+card,\s*reveal it,\s*then shuffle and put that card on top\.?$")
def _may_search_typed_card_top(m):
    query_word = m.group(1).strip()
    return Optional_(body=Tutor(
        query=Filter(base=query_word + "_card", quantifier="one",
                      targeted=False),
        destination="top_of_library",
        shuffle_after=True,
        reveal=True,
    ))


# ============================================================================
# GROUP G: Goad / Heist / Convert / Suspect — keyword actions
# ============================================================================

# "goad target creature that player controls" (4).
@_eff(r"^goad target creature that player controls\.?$")
def _goad_tc_their(m):
    return Modification(kind="goad", args=("target_creature_that_player",))


# "goad target creature an opponent controls" (3).
@_eff(r"^goad target creature an opponent controls\.?$")
def _goad_tc_opp(m):
    return Modification(kind="goad", args=("target_creature_opponent",))


# "heist target opponent's library" (3).
@_eff(r"^heist target opponent'?s library\.?$")
def _heist(m):
    return Modification(kind="heist", args=("target_opponent",))


# "suspect it" (4).
@_eff(r"^suspect it\.?$")
def _suspect_it(m):
    return Modification(kind="suspect", args=("pronoun",))


# "it becomes plotted" (4).
@_eff(r"^it becomes plotted\.?$")
def _it_plot(m):
    return Modification(kind="plot", args=("pronoun",))


# "investigate twice" (5), "investigate that many times" (3).
@_eff(r"^investigate twice\.?$")
def _investigate_twice(m):
    return Modification(kind="investigate", args=(2,))


@_eff(r"^investigate that many times\.?$")
def _investigate_var(m):
    return Modification(kind="investigate", args=("var",))


# "copy that ability" (5). Rings of Brighthearth, Battlemage's Bracers,
# Chandra's Regulator, Verrak.
@_eff(r"^copy that ability\.?$")
def _copy_ability(m):
    return Modification(kind="copy_ability", args=())


# "copy it for each time you've cast your commander from the command zone
# this game" (5). Commander-storm.
@_eff(r"^copy it for each time you'?ve cast your commander from the command zone this game\.?$")
def _copy_per_commander_cast(m):
    return Modification(kind="copy_per_commander_cast", args=())


# "those creatures fight each other" (3). Joust, Hog-Monkey Rampage,
# Longstalk Brawl.
@_eff(r"^those creatures fight each other\.?$")
def _fight_each_other(m):
    return Modification(kind="fight_each_other", args=())


# ============================================================================
# GROUP H: Self / "this creature" effects
# ============================================================================

# "this creature deals 2 damage to each opponent" (12). Embraal Gear-Smasher,
# Cabal Paladin, Fire Nation Archers, Teapot Slinger.
@_eff(r"^this creature deals\s+(\d+|x)\s+damage to each opponent\.?$")
def _self_damage_each_opp(m):
    return Damage(amount=_n(m.group(1)), target=EACH_OPPONENT)


# "this creature can attack this turn as though it didn't have defender"
# (11). Skyclave Squid, Krotiq Nestguard, Hightide Hermit, Steelclad Spirit.
@_eff(r"^this creature can attack this turn as though it didn'?t have defender\.?$")
def _attack_without_defender(m):
    return Modification(kind="attack_without_defender_eot", args=())


# "this creature phases out" (6). Blink Dog, Hostile Hostel, Mist Dragon,
# Rainbow Efreet.
@_eff(r"^this creature phases out\.?$")
def _phase_out_self(m):
    return Modification(kind="phase_out_self", args=())


# "this creature loses defender until end of turn" (5). Torpid Moloch,
# Tidewater Minion, Shoal Serpent, Grozoth.
@_eff(r"^this creature loses defender until end of turn\.?$")
def _lose_defender_eot(m):
    return Modification(kind="lose_ability_eot", args=("defender",))


# "this creature becomes prepared" (4). Emeritus cycle.
@_eff(r"^this creature becomes prepared\.?$")
def _become_prepared(m):
    return Modification(kind="becomes_prepared", args=())


# "it fights up to one target creature you don't control" (6). Wicked Wolf,
# Primal Might, Back for More.
@_eff(r"^it fights up to one target creature you don'?t control\.?$")
def _it_fights_up_to_one_opp(m):
    return Fight(
        a=Filter(base="that_creature", targeted=False),
        b=Filter(base="creature", quantifier="up_to_n", count=1,
                  opponent_controls=True, targeted=True),
    )


# "double target creature's power until end of turn" (3). Mr. Orfeo,
# Skullspore Nexus.
@_eff(r"^double target creature'?s power until end of turn\.?$")
def _double_target_power(m):
    return Modification(kind="double_power_eot", args=())


# ============================================================================
# GROUP I: Self-keyword grants (~, this creature) until end of turn
# ============================================================================
# "~ gains flying until end of turn" (5). Cromat, Zabaz, Phelddagrif.
_KEYWORDS_FOR_SELF = (
    r"flying|haste|vigilance|deathtouch|lifelink|trample|first strike|"
    r"double strike|menace|reach|hexproof|indestructible|protection|"
    r"infect|shroud|wither|flash"
)


@_eff(r"^~ gains (" + _KEYWORDS_FOR_SELF + r") until end of turn\.?$")
def _self_gain_keyword_eot(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="self", targeted=False),
        duration="until_end_of_turn",
    )


# "this creature gains X until end of turn" — variant phrasing.
@_eff(r"^this creature gains (" + _KEYWORDS_FOR_SELF + r") until end of turn\.?$")
def _this_creature_gain_kw_eot(m):
    return GrantAbility(
        ability_name=m.group(1).strip(),
        target=Filter(base="self", targeted=False),
        duration="until_end_of_turn",
    )


# ``~ gains protection from the color of your choice until end of turn`` (2).
@_eff(r"^~ gains protection from the colou?r of your choice until end of turn\.?$")
def _self_gain_protection_choice(m):
    return GrantAbility(
        ability_name="protection_from_chosen_color",
        target=Filter(base="self", targeted=False),
        duration="until_end_of_turn",
    )


# ============================================================================
# GROUP J: Stun / don't-untap riders
# ============================================================================

# "it doesn't untap during its controller's next untap step" (3). Stitcher's
# Graft, Lead Golem, Apes of Rath.
@_eff(r"^it doesn'?t untap during its controller'?s next untap step\.?$")
def _stun_it_next_untap(m):
    return Modification(kind="stun_target_next_untap",
                         args=("pronoun_it",))


# ============================================================================
# GROUP K: Conditional effects — "if <cond>, <effect>"
# ============================================================================

# "if you control a creature with power 4 or greater, draw a card" (5).
# Beastbond Outcaster, Garruk's Uprising, Master's Guidance, Hunter's
# Talent.
@_eff(r"^if you control a creature with power\s+(\d+)\s+or greater,\s*draw a card\.?$")
def _if_power_draw(m):
    n = int(m.group(1))
    return Conditional(
        condition=Condition(kind="you_control_creature_power_ge", args=(n,)),
        body=Draw(count=1, target=SELF),
    )


# "if a creature died this turn, put a +1/+1 counter on this creature" (3).
# Bulette, Cackling Prowler, Vashta Nerada.
@_eff(r"^if a creature died this turn,\s*put a \+1/\+1 counter on this creature\.?$")
def _if_creature_died_p1p1(m):
    return Conditional(
        condition=Condition(kind="creature_died_this_turn", args=()),
        body=CounterMod(op="put", count=1, counter_kind="+1/+1",
                         target=Filter(base="self", targeted=False)),
    )


# "if you attacked this turn, target opponent discards a card" (3).
@_eff(r"^if you attacked this turn,\s*target opponent discards a card\.?$")
def _if_attacked_discard(m):
    return Conditional(
        condition=Condition(kind="you_attacked_this_turn", args=()),
        body=Discard(count=1, target=TARGET_OPPONENT,
                      chosen_by="discarder"),
    )


# "if you attacked this turn, this creature deals 2 damage to any target"
# (3). Gorehorn Raider, Storm Fleet Pyromancer, Mardu Heart-Piercer.
@_eff(r"^if you attacked this turn,\s*this creature deals\s+(\d+|x)\s+damage to any target\.?$")
def _if_attacked_dmg_any(m):
    return Conditional(
        condition=Condition(kind="you_attacked_this_turn", args=()),
        body=Damage(amount=_n(m.group(1)), target=TARGET_ANY),
    )


# "if you descended this turn, put a +1/+1 counter on this creature" (3).
# Child of the Volcano, Deep Goblin Skulltaker, Stalactite Stalker.
@_eff(r"^if you descended this turn,\s*put a \+1/\+1 counter on this creature\.?$")
def _if_descended_p1p1(m):
    return Conditional(
        condition=Condition(kind="you_descended_this_turn", args=()),
        body=CounterMod(op="put", count=1, counter_kind="+1/+1",
                         target=Filter(base="self", targeted=False)),
    )


# "if no creatures are on the battlefield, sacrifice this enchantment" (3).
@_eff(r"^if no creatures are on the battlefield,\s*sacrifice this enchantment\.?$")
def _if_no_creatures_sac_self(m):
    return Conditional(
        condition=Condition(kind="no_creatures_on_battlefield", args=()),
        body=Sacrifice(query=Filter(base="self", targeted=False)),
    )


# ============================================================================
# GROUP L: Misc common phrases
# ============================================================================

# "no life gained" (16). Tibalt Rakish Instigator, Erebos God of the Dead,
# Leyline of Punishment. Quoted-ability payload of a prevention/replacement.
@_eff(r"^no life gained\.?$")
def _no_life_gained(m):
    return Modification(kind="no_life_gained", args=())


# "suppress prevention" (15). Leyline of Punishment etc. Payload label.
@_eff(r"^suppress prevention\.?$")
def _suppress_prev(m):
    return Modification(kind="suppress_prevention", args=())


# "put that card into your hand" (6). Discover the Impossible, Blurry
# Visionary, Matter Reshaper, Breaching Dragonstorm.
@_eff(r"^put that card into your hand\.?$")
def _put_that_card_hand(m):
    return Modification(kind="put_card_into_hand", args=("that_card",))


# "put it into your hand" (5). Songbirds' Blessing, Bre of Clan Stoutarm,
# Bortuk Bonerattle, Reason // Believe.
@_eff(r"^put it into your hand\.?$")
def _put_it_hand(m):
    return Modification(kind="put_card_into_hand", args=("pronoun_it",))


# "put it onto the battlefield" (3).
@_eff(r"^put it onto the battlefield\.?$")
def _put_it_battlefield(m):
    return Modification(kind="put_onto_battlefield", args=("pronoun_it",))


# "attach this aura to target creature" (3). Bound by Moonsilver,
# Stasis Cell, Detainment Spell.
@_eff(r"^attach this aura to target creature\.?$")
def _attach_aura_target_creature(m):
    return Modification(kind="attach_aura_target_creature", args=())


# "flip a coin until you lose a flip" (3). Zndrsplt, Okaun, Crazed Firecat.
# Distinct from bare "flip a coin" (snowflake per-card).
@_eff(r"^flip a coin until you lose a flip\.?$")
def _flip_until_lose(m):
    return Modification(kind="flip_coin_until_lose", args=())


# "you may flip a coin" (2).
@_eff(r"^you may flip a coin\.?$")
def _may_flip(m):
    return Modification(kind="may_flip_coin", args=())


# "pay any amount of life" (4). Minion of the Wastes, Nameless Race,
# Phyrexian Processor, Vizkopa Confessor. Variable-X cost.
@_eff(r"^pay any amount of life\.?$")
def _pay_any_life(m):
    return Modification(kind="pay_any_amount_life", args=())


# "they lose 2 life" (4). Liesa, Sheoldred, Mai. Usually the RHS of a
# split trigger "whenever ..., they lose 2 life".
@_eff(r"^they lose\s+(\d+|x|one|two|three|four|five|six|seven|eight|nine|ten)\s+life\.?$")
def _they_lose_life(m):
    return LoseLife(amount=_n(m.group(1)),
                     target=Filter(base="them", targeted=False))


# "you lose that much life" (3). Filthy Cur, Thrashing Mudspawn,
# Emberwilde Caliph.
@_eff(r"^you lose that much life\.?$")
def _you_lose_that_much_life(m):
    return LoseLife(amount="var", target=SELF)


# "return this card from your graveyard to the battlefield tapped and
# attacking" (3). Warcry Phoenix, Persistent Marshstalker, Interceptor.
@_eff(r"^return this card from your graveyard to the battlefield tapped and attacking\.?$")
def _return_tapped_attacking(m):
    return Reanimate(
        query=Filter(base="self", targeted=False),
        from_zone="your_graveyard",
        destination="battlefield",
        with_modifications=("tapped", "attacking"),
    )


# "you may play the exiled card without paying its mana cost" (5). Fight
# Rigging, Rabble Rousing, Collector's Cage, Cemetery Tampering.
@_eff(r"^you may play the exiled card without paying its mana cost\.?$")
def _may_play_exiled_free(m):
    return Modification(kind="may_play_exiled_free", args=())


# "you may cast the copy without paying its mana cost" (5). Panoptic
# Mirror, Reversal of Fortune, Spellweaver Helix, Spellbinder.
@_eff(r"^you may cast the copy without paying its mana cost\.?$")
def _may_cast_copy_free(m):
    return Modification(kind="may_cast_copy_free", args=())


# "defending player sacrifices a creature of their choice" (3). Nefarox,
# Gisa's Favorite Shovel, Thraximundar.
@_eff(r"^defending player sacrifices a creature of their choice\.?$")
def _def_player_sac_choice(m):
    return Sacrifice(
        query=Filter(base="creature", quantifier="one", targeted=False),
        actor="defending_player_choice",
    )


# "discard two cards unless you discard a <typed> card" (3 creature / 3
# artifact). Thirst for Identity / Thirst for Knowledge / Tezzeret etc.
@_eff(r"^discard two cards unless you discard an?\s+(creature|artifact|enchantment|land|planeswalker|nonland|nonartifact|nonbasic)\s+card\.?$")
def _discard_two_unless_typed(m):
    return Modification(kind="discard_two_unless_typed",
                         args=(m.group(1).lower(),))


# "a spell that targets this creature, put a +1/+1 counter on it" (4). This
# is a heroic-style ability tail where the trigger was eaten. Label
# preserves shape so downstream can recognize it.
@_eff(r"^a spell that targets this creature,\s*put a \+1/\+1 counter on it\.?$")
def _heroic_p1p1_it(m):
    return Modification(kind="heroic_rider_p1p1_pronoun", args=())


# "a spell that targets this creature, put a +1/+1 counter on this creature"
# (5). War-Wing Siren, Dawnbringer Charioteers.
@_eff(r"^a spell that targets this creature,\s*put a \+1/\+1 counter on this creature\.?$")
def _heroic_p1p1_self(m):
    return Modification(kind="heroic_rider_p1p1_self", args=())


# "a spell that targets this creature, put two +1/+1 counters on this
# creature" (3). Pheres-Band Thunderhoof, Setessan Oathsworn.
@_eff(r"^a spell that targets this creature,\s*put two \+1/\+1 counters on this creature\.?$")
def _heroic_two_p1p1_self(m):
    return Modification(kind="heroic_rider_two_p1p1_self", args=())


# "a spell that targets this creature, creatures you control get +1/+0
# until end of turn" (5). Hero of the Nyxborn cycle.
@_eff(r"^a spell that targets this creature,\s*creatures you control get \+(\d+)/\+(\d+) until end of turn\.?$")
def _heroic_army_buff(m):
    return Modification(kind="heroic_rider_anthem_eot",
                         args=(int(m.group(1)), int(m.group(2))))


# "return that card to the battlefield" (3). Nim Deathmantle, Bortuk
# Bonerattle, Angelic Renewal.
@_eff(r"^return that card to the battlefield\.?$")
def _return_that_card_bf(m):
    return Reanimate(
        query=Filter(base="that_card", targeted=False),
        from_zone="any_graveyard",
        destination="battlefield",
    )


# "return that card to the battlefield under its owner's control" (6). Also
# used by flicker cards.
@_eff(r"^return that card to the battlefield under its owner'?s control\.?$")
def _return_that_card_bf_owner(m):
    return Reanimate(
        query=Filter(base="that_card", targeted=False),
        from_zone="exile",
        destination="battlefield",
        controller="owner",
    )


# "return it to the battlefield under its owner's control" (3). Obzedat,
# Hikari, Long River Lurker.
@_eff(r"^return it to the battlefield under its owner'?s control\.?$")
def _return_it_bf_owner(m):
    return Reanimate(
        query=Filter(base="that_thing", targeted=False),
        from_zone="exile",
        destination="battlefield",
        controller="owner",
    )


# "return the exiled card to the battlefield under its owner's control"
# (13). Slithery Stalker, Journey to Nowhere, Faceless Butcher.
@_eff(r"^return the exiled card to the battlefield under its owner'?s control\.?$")
def _return_exiled_bf_owner(m):
    return Reanimate(
        query=Filter(base="exiled_card", targeted=False),
        from_zone="exile",
        destination="battlefield",
        controller="owner",
    )


# "reveal the top card of your library and put that card into your hand"
# (7). Dark Confidant, Dark Tutelage, Darkstar Augur, Sorin Grim Nemesis.
@_eff(r"^reveal the top card of your library and put that card into your hand\.?$")
def _reveal_top_put_hand(m):
    return Sequence(items=(
        Reveal(source="top_of_library", actor="controller", count=1),
        Modification(kind="put_card_into_hand", args=("revealed_card",)),
    ))


# "sacrifice that permanent" (3). Stitcher's Graft, Grafted Wargear,
# Grafted Exoskeleton.
@_eff(r"^sacrifice that permanent\.?$")
def _sac_that_permanent(m):
    return Sacrifice(query=Filter(base="that_permanent", targeted=False))


# "put one into your hand and the other into your graveyard" (3). Fork in
# the Road, Jarad's Orders, Final Parting.
@_eff(r"^put one into your hand and the other into your graveyard\.?$")
def _split_hand_gy(m):
    return Modification(kind="split_one_hand_one_gy", args=())


# "exile another target nonland permanent" (3). Argent Dais, Unyielding
# Gatekeeper, Oblivion Ring.
@_eff(r"^exile another target nonland permanent\.?$")
def _exile_another_nonland(m):
    return Exile(target=Filter(base="nonland_permanent", quantifier="one",
                                  targeted=True, extra=("another",)))


# "exile another target creature" (catch). Variant of above.
@_eff(r"^exile another target creature\.?$")
def _exile_another_creature(m):
    return Exile(target=Filter(base="creature", quantifier="one",
                                  targeted=True, extra=("another",)))


# "~ gains first strike until end of turn" / "flying" — already covered
# above. Skip duplicate.


# "that land becomes a 0/0 elemental creature with haste that's still a
# land" (3). Cyclone Sire, Wall of Resurgence, Noyan Dar.
@_eff(r"^that land becomes a\s+(\d+)/(\d+)\s+([a-z]+)\s+creature with haste that'?s still a land\.?$")
def _land_becomes_creature(m):
    return Modification(kind="land_becomes_creature_haste",
                         args=(int(m.group(1)), int(m.group(2)),
                               m.group(3).lower()))


# "you may look at the top card of your library any time" variant.
# Actually simpler: "choose color" (8) → choose-color modifier.
@_eff(r"^choose color\.?$")
def _choose_color_label(m):
    return Modification(kind="choose_color", args=())


# "it deals 3 damage to target player or planeswalker" (3). Keldon
# Champion, Emberwilde Augur, Vulshok Replica.
@_eff(r"^it deals\s+(\d+|x)\s+damage to target player or planeswalker\.?$")
def _it_damage_player_pw(m):
    return Damage(amount=_n(m.group(1)),
                   target=Filter(base="player_or_planeswalker", quantifier="one",
                                  targeted=True))


# "it deals 5 damage divided as you choose among any number of targets" —
# already in _UNKNOWN_PROMOTE_FORBIDDEN, skip.


# ============================================================================
# Rhystic Study / Mystic Remora family — "you may draw a card unless that
# player pays {N}" (2 nodes, but emotional-arc flagged in Wave 1a spec).
# Shape: Optional_(body=Modification(kind="draw_unless_pay_N", args=(N,)))
# — wraps the tax amount so the engine can prompt for the pay choice.
# ============================================================================

@_eff(r"^you may draw a card unless that player pays \{(\d+)\}\.?$")
def _may_draw_unless_pay(m):
    n = int(m.group(1))
    return Optional_(body=Modification(
        kind="draw_unless_pay",
        args=(n, "opponent"),
    ))


# ``counter that spell or ability unless its controller pays {N}`` (4
# nodes). Shimmering Glasskite, Jetting Glasskite, Glyph Keeper.
@_eff(r"^counter that spell or ability unless its controller pays \{(\d+)\}\.?$")
def _counter_unless_pay(m):
    return Modification(kind="counter_that_spell_unless_pay",
                         args=(int(m.group(1)),))


# ============================================================================
# Additional Wave 1a ~2-3 node phrases worth promoting for clarity
# ============================================================================

# ``create a 1/1 green elf warrior creature token`` bare form — not
# covered by the built-in bare-form rule because "warrior" is two
# subtype words. Example: Prowess of the Fair, Lys Alana Huntmaster,
# Nath of the Gilt-Leaf. This rule captures 2+-word subtypes explicitly.
# NOTE: the built-in core rule at parser.py line 670 uses ``[a-z ]+?``
# which DOES match multi-word subtypes like "elf warrior" — but feeds
# both color word and subtype into the types tuple. Our aim is the
# Optional-wrapped variant "you may create ..." so no duplicate here.


# "you may create a <tapped>? N/N color subtype creature token" already
# promoted via _may_create_token above.


# ============================================================================
# Final: done. Handlers registered via module-level decorator side effects.
# ============================================================================
