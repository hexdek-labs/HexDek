#!/usr/bin/env python3
"""MVP self-play loop for mtgsquad.

Goal: prove the parser's typed AST can drive a working game engine end-to-end,
using only structurally-parsed cards. No combat (priority over the beat), no
colored mana, no real stack — just dispatch Effect nodes through a handler
table, step through turns, and log what happens.

Architecture:
    Oracle JSON --parser.parse_card--> CardAST --playloop--> log

Run:
    python3 scripts/playloop.py                  # 100 games, default decks
    python3 scripts/playloop.py --games 5 --verbose

The report lands at data/rules/playloop_report.md.
"""

from __future__ import annotations

import argparse
import json
import random
import re
import sys
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Optional, Union

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import parser as mtg_parser  # parser.py in same dir
from mtg_ast import (
    Activated, AddMana, Buff, CardAST, CounterMod, CreateToken, CounterSpell,
    Damage, Destroy, Discard, Draw, Effect, Exile, Filter, GainLife, Keyword,
    LoseLife, Modification, Mill, Reanimate, Recurse, Reveal, Scry,
    ScalingAmount, Sequence,
    Choice, Optional_, Conditional, Static, Tutor, Triggered, UnknownEffect,
    FaceDownCharacteristics, FACE_DOWN_VANILLA, FACE_DOWN_WARD_2,
    TurnFaceUp, ManaCost, ManaSymbol,
)

# Runtime dispatch for per-card snowflake handlers. Parse-time handlers in
# extensions/per_card.py emit Modification(kind="custom", args=(slug, ...))
# markers; this module turns those slugs into actual game-state mutations.
# The import is defensive — if the extensions package isn't available the
# engine falls back to the legacy "silently skip" behavior.
try:
    from extensions.per_card_runtime import (
        dispatch_custom as _dispatch_per_card,
        NAME_TO_ETB_SLUG as _NAME_TO_ETB_SLUG,
        NAME_TO_LTB_SLUG as _NAME_TO_LTB_SLUG,
        NAME_TO_SPELL_SLUGS as _NAME_TO_SPELL_SLUGS,
        NAME_TO_ACTIVATED_SLUG as _NAME_TO_ACTIVATED_SLUG,
    )
except Exception:  # pragma: no cover — keeps the engine importable standalone
    _dispatch_per_card = None
    _NAME_TO_ETB_SLUG = {}
    _NAME_TO_LTB_SLUG = {}
    _NAME_TO_SPELL_SLUGS = {}
    _NAME_TO_ACTIVATED_SLUG = {}

# Pluggable AI decision policy. All decision sites in this module route
# through `seat.policy.method(...)` — the engine never inspects the
# concrete hat type. Default is `GreedyHat` (heuristic baseline
# identical to the legacy inline logic). Alternatives like `PokerHat`
# (HOLD/CALL/RAISE adaptive) plug in via a single-line reassignment:
#     seat.policy = PokerHat()
# See scripts/extensions/policies/README.md for the full interface.
try:
    from extensions.policies import GreedyHat as _DefaultPolicy  # noqa: E402
except Exception:  # pragma: no cover — keep engine importable standalone
    _DefaultPolicy = None

ROOT = HERE.parent
ORACLE_DUMP = ROOT / "data" / "rules" / "oracle-cards.json"
REPORT = ROOT / "data" / "rules" / "playloop_report.md"

MAX_TURNS = 40  # safety cutoff (2-player)
# Multiplayer Commander games run materially longer. CR §903 games with
# 3-4 seats regularly hit 20+ turns per seat before a last-seat-standing
# outcome. Bumping the multiplayer cap keeps the gauntlet from cutting
# games short by the turn-cap heuristic. 2-player path keeps MAX_TURNS.
MAX_TURNS_MULTIPLAYER = 100  # safety cutoff (N-seat, N>=3)
STARTING_LIFE = 20
STARTING_HAND = 7
DECK_SIZE = 60


# ============================================================================
# Game state
# ============================================================================

@dataclass
class Permanent:
    """An object on the battlefield. Lands count. Tokens count.

    State-based-action support (CR 704.5):
      - counters: dict of counter-kind → count. Used for loyalty (704.5i),
        +1/+1 vs -1/-1 annihilation (704.5q), lore counters on Sagas (704.5v),
        defense counters on Battles (704.5s), and generic "can't have more
        than N" rules (704.5r).
      - attached_to: the object/player this permanent is attached to, for
        auras (704.5n), equipment/fortifications (704.5p), battles/creatures
        (704.5q), and tokens (704.5q tail). None = not attached.
      - timestamp: monotonically-increasing creation-order stamp used by the
        Legend Rule (704.5j — choose which one to keep) and the World Rule
        (704.5k — oldest world permanent dies first).
    """
    card: "CardEntry"
    controller: int
    tapped: bool = False
    summoning_sick: bool = True
    damage_marked: int = 0
    buffs_pt: tuple[int, int] = (0, 0)  # +P/+T until end of turn
    granted: list[str] = field(default_factory=list)
    counters: dict = field(default_factory=dict)  # counter_kind -> int
    attached_to: Optional["Permanent"] = None
    timestamp: int = 0
    # CR §506.3 — a permanent that enters the battlefield "attacking" IS
    # an attacking creature (506.2 counts it as attacking), but it
    # wasn't declared as an attacker, so "whenever ~ attacks" triggers
    # (603.3 + 508.2 — check for declared attackers) DON'T fire. Hero
    # of Bladehold's Soldier tokens are the canonical example: they
    # enter tapped and attacking; the Hero itself fires its own
    # attacks-trigger exactly once (for the Hero's declared attack);
    # the tokens do NOT fire any attacks-triggers they might have.
    attacking: bool = False
    # Entered as a declared attacker? Used by the attacks-trigger firing
    # path to distinguish "declared" from "came in attacking via effect".
    declared_attacker_this_combat: bool = False
    # CR 108.3 — a card's owner is the player who started the game with
    # it in their deck (or the player who brought it into the game).
    # Ownership does NOT change when control changes (e.g. Gilded Drake).
    # `owner` is a seat index that's ONLY meaningful when it differs
    # from `controller`; None means "owner == controller" (the common
    # case — a permanent on the battlefield of its original player).
    # Used by §903.9 commander zone-change replacement (the OWNER's
    # commander return, even when opp currently controls the card).
    owner: Optional[int] = None
    # ------------------------------------------------------------------
    # Face-down state (CR §702.37 Morph, §701.40 Manifest, §701.58 Cloak,
    # §702.168 Disguise, §701.62 Manifest Dread).
    # ------------------------------------------------------------------
    # CR §708: a face-down permanent is a 2/2 nameless colorless creature
    # with no text, no subtypes, no mana cost. The face_down flag is
    # consulted by get_effective_characteristics() at §613 layer 1b to
    # apply the characteristic-override. Layer 7d counter modifications
    # STILL apply over the override (a face-down 2/2 with a +1/+1
    # counter is 3/3 — see CR §613.4 layer order).
    face_down: bool = False
    # CR §708.5 allows the controller to look at a face-down permanent
    # they control at any time. We record the "true" card here so
    # turn_face_up() can restore the original characteristics and the
    # controller-side look helper can peek.
    # None when ``face_down=False`` (normal face-up permanent); also None
    # for a vanilla 2/2 face-down token created without an underlying
    # card (rare — manifest creates a token only if the top of library
    # is NOT a creature card... but per §701.40a the manifested CARD
    # goes to the battlefield regardless, so this should usually be set).
    original_card: Optional["CardEntry"] = None
    # The face-down family variant (which FaceDownCharacteristics apply).
    # "vanilla" = Morph/Megamorph/Manifest/Manifest-Dread (CR §708.2a
    # default 2/2 no text / no name / no abilities)
    # "ward_2"  = Disguise/Cloak/Cybermen (§702.168a / §701.58a — same
    # 2/2 nameless shell + ward {2}).
    # Unused when ``face_down=False``; "vanilla" is the safe default.
    face_down_variant: str = "vanilla"
    # The flip cost to pay to turn face up (the permanent's morph /
    # megamorph / disguise cost captured at cast/manifest time). None
    # for manifested cards — those are turned face up for their
    # original MANA COST if the card is a creature card (CR §701.40a).
    turn_face_up_cost: Optional["ManaCost"] = None
    # True iff the flip cost is a megamorph cost (CR §702.37b adds a
    # +1/+1 counter when paid as the flip cost). This is a PROPERTY of
    # the flip cost, not of the card — a copy effect that makes a
    # megamorph card into a non-megamorph card's face-up form still
    # uses the megamorph cost for flipping, which still places the
    # counter. We track it on the permanent so turn_face_up() can read
    # it without re-walking abilities.
    flip_cost_is_megamorph: bool = False

    # ------------------------------------------------------------------
    # DFC / transform state (CR §712 double-faced cards).
    # ------------------------------------------------------------------
    # `transformed` is False while the permanent's FRONT face is active
    # (the default at ETB — DFCs enter with their front face up, CR
    # §712.2), True once Transform has flipped it to the back face.
    # Every transform event toggles this flag.
    transformed: bool = False
    # Cached AST for the inactive face. At ETB we freeze the CardAST of
    # the CURRENTLY-INACTIVE face here so Transform can swap it in
    # without re-parsing oracle text. For single-faced cards this is
    # None and transform() is a no-op.
    inactive_face_ast: Optional["CardAST"] = None
    # Cached display name for the inactive face. Used by `display_name`
    # and by event logs so the `transform` event cleanly reports the
    # before/after.
    inactive_face_name: Optional[str] = None
    # Cached P/T for the inactive face (used by power/toughness accessors
    # to honor the currently-active face's printed P/T). None for the
    # back-face entries of non-creature DFCs (e.g. Tergrid's Lantern
    # back face is an artifact with no P/T).
    inactive_face_power: Optional[int] = None
    inactive_face_toughness: Optional[int] = None
    inactive_face_type_line: Optional[str] = None
    inactive_face_mana_cost: Optional[str] = None
    inactive_face_cmc: Optional[int] = None
    inactive_face_colors: tuple = ()

    @property
    def owner_seat(self) -> int:
        """Resolved owner seat: override if set, else controller (CR 108.3)."""
        return self.owner if self.owner is not None else self.controller

    @property
    def power(self) -> int:
        base = self.card.power or 0
        bonus = self.counters.get("+1/+1", 0) - self.counters.get("-1/-1", 0)
        return base + self.buffs_pt[0] + bonus

    @property
    def toughness(self) -> int:
        base = self.card.toughness or 0
        bonus = self.counters.get("+1/+1", 0) - self.counters.get("-1/-1", 0)
        return base + self.buffs_pt[1] + bonus

    @property
    def is_creature(self) -> bool:
        return "creature" in self.card.type_line.lower()

    @property
    def is_land(self) -> bool:
        return "land" in self.card.type_line.lower()

    @property
    def is_artifact(self) -> bool:
        """CR §301 — artifact supertype. Matters for the mana-source
        iteration in fill_mana_pool / _available_mana: artifacts with
        {T}-add-mana abilities (Sol Ring, Moxes, Signets, Talismans,
        Mana Crypt, Grim Monolith, Treasure tokens) need their own
        branch because they are neither lands nor creatures. Note: an
        artifact creature routes through the CREATURE branch (tap for
        mana if it has the ability, respects summoning sickness);
        artifact lands route through the LAND branch. This property
        returns True ONLY for pure artifacts to avoid double-counting."""
        tl = self.card.type_line.lower()
        return ("artifact" in tl and
                "creature" not in tl and
                "land" not in tl)

    @property
    def is_token(self) -> bool:
        # Tokens are flagged via the "Token" prefix the create_token handler
        # puts on their type_line, plus ad-hoc test-harness tokens that start
        # with "Token " in the card name. CR 704.5d uses "token" as the key
        # distinguishing feature here.
        tl = self.card.type_line.lower()
        return tl.startswith("token ") or " token" in tl.split("—")[0]

    @property
    def is_planeswalker(self) -> bool:
        return "planeswalker" in self.card.type_line.lower()

    @property
    def is_legendary(self) -> bool:
        return "legendary" in self.card.type_line.lower()

    @property
    def is_world(self) -> bool:
        return "world" in self.card.type_line.lower().split("—")[0]

    @property
    def is_aura(self) -> bool:
        tl = self.card.type_line.lower()
        return "enchantment" in tl and "aura" in tl

    @property
    def is_equipment(self) -> bool:
        tl = self.card.type_line.lower()
        return "artifact" in tl and "equipment" in tl

    @property
    def is_fortification(self) -> bool:
        tl = self.card.type_line.lower()
        return "artifact" in tl and "fortification" in tl

    @property
    def is_saga(self) -> bool:
        tl = self.card.type_line.lower()
        return "enchantment" in tl and "saga" in tl

    @property
    def is_battle(self) -> bool:
        return "battle" in self.card.type_line.lower()

    @property
    def is_role(self) -> bool:
        tl = self.card.type_line.lower()
        return "enchantment" in tl and "role" in tl


@dataclass
class CardEntry:
    """A deck card — static data derived from oracle + parser."""
    name: str
    mana_cost: str
    cmc: int
    type_line: str
    oracle_text: str
    power: Optional[int]
    toughness: Optional[int]
    ast: CardAST
    colors: tuple[str, ...] = ()  # ("R",), ("W","U"), or () for colorless
    # Starting loyalty for planeswalkers (from Scryfall 'loyalty' field). Used
    # by CR 704.5i SBA to initialize a Permanent's loyalty counter on ETB.
    starting_loyalty: Optional[int] = None
    # Starting defense counters for battles (from Scryfall 'defense' field).
    # Used by CR 704.5s SBA and for battle ETB initialization.
    starting_defense: Optional[int] = None

    # CR §712 DFC / transform support. For double-faced cards
    # (layout=transform / modal_dfc), we preload the BACK-FACE AST + P/T
    # + type line at load time so transform() can swap without re-parsing.
    # Both `layout` and back_face_* are None on single-faced cards.
    # Populated by load_card_by_name when the scryfall entry carries
    # `card_faces`.
    layout: str = ""               # "normal" / "transform" / "modal_dfc" / "meld" / ...
    is_dfc: bool = False            # convenience predicate
    # Back-face characteristics (all None for non-DFC cards).
    back_face_name: Optional[str] = None
    back_face_ast: Optional[CardAST] = None
    back_face_mana_cost: Optional[str] = None
    back_face_cmc: Optional[int] = None
    back_face_type_line: Optional[str] = None
    back_face_oracle_text: Optional[str] = None
    back_face_power: Optional[int] = None
    back_face_toughness: Optional[int] = None
    back_face_colors: tuple = ()
    # Front-face characteristics mirror the top-level fields on a DFC —
    # we ALSO capture them separately so transform() can swap back on
    # night→day without re-reading the scryfall entry. On a non-DFC this
    # is None; on a DFC front_face_* equals top-level fields.
    front_face_name: Optional[str] = None
    front_face_ast: Optional[CardAST] = None
    front_face_mana_cost: Optional[str] = None
    front_face_cmc: Optional[int] = None
    front_face_type_line: Optional[str] = None
    front_face_oracle_text: Optional[str] = None
    front_face_power: Optional[int] = None
    front_face_toughness: Optional[int] = None
    front_face_colors: tuple = ()


@dataclass
class Deck:
    """A deck plus optional §903 Commander metadata.

    Fields:
      cards           — the 99-card (Commander) or N-card list that
                        becomes the seat's library. The commander card
                        itself should NOT be included here; it goes to
                        the command zone at game setup per §903.6.
      commander_name  — single-commander card name (§903.3). None for
                        non-Commander games.
      commander_names — list of card names when the deck uses partner,
                        partner-with, friends-forever, or a Background
                        (§903.3a / §702.124). Architected for partner
                        support; single-commander implementation treats
                        this as derived from commander_name when unset.
                        Partner/companion is OUT OF SCOPE TODO — the
                        gauntlet's 4 decks are all single-commander.

    In non-Commander flows, callers pass raw list[CardEntry] to
    play_game as before; Deck is only used for Commander setup.
    """
    cards: list  # list[CardEntry]
    commander_name: Optional[str] = None
    commander_names: Optional[list] = None  # list[str] for partner; optional


# ---------------------------------------------------------------------------
# Typed mana pool (CR §106 — Mana).
#
# A player's mana pool tracks mana by COLOR (W/U/B/R/G), plus colorless C
# mana (§106.1b — distinct from generic; Sol Ring, Wastes, Eldrazi Temple
# produce {C}), plus an "any"-color bucket for sources that let the player
# choose at spend time (Moxes, Lotus Petal, Treasures, Chromatic Lantern).
# The restricted-mana list models §106.4a restriction riders — Food Chain's
# "spend only to cast creature spells", Cabal Coffers' "spend only on
# noncreature abilities", Powerstone's "spend only on non-creature
# activated abilities".
#
# Phase-end drain (§106.4) empties the pool at every phase/step boundary
# unless a continuous effect like Upwelling (all colors retained) or
# Omnath, Locus of Mana (green retained) exempts it. See
# drain_pool()/_pool_exempt_colors() below.
# ---------------------------------------------------------------------------

@dataclass
class RestrictedMana:
    """A unit of mana in the pool carrying a spend-time restriction.
    CR §106.4a — "Some effects add mana that has restrictions on how it
    can be spent." Examples: Food Chain's creature-only mana, Cabal
    Coffers' noncreature-activations-only, Powerstone's {C} noncreature
    activations only, Crypt Ghast's etc.
    """
    amount: int
    color: Optional[str]  # "W"/"U"/"B"/"R"/"G"/"C" or None for any-color
    restriction: str  # e.g. "creature_spell_only", "noncreature_or_artifact_activation"
    source_name: str = ""  # attribution for event logging


@dataclass
class ManaPool:
    """Five-color + colorless + any + restricted mana pool (CR §106).

    Buckets:
      W/U/B/R/G — colored pools (§106.1a)
      C          — colorless pool from {C} sources (§106.1b)
      any        — mana-of-any-color (Mox, Lotus Petal, Treasure, Chromatic
                   Lantern, Commander's Sphere). Player picks which color
                   at SPEND time, so we defer resolution until _pay_cost.
      restricted — list[RestrictedMana]. Each unit carries a tag like
                   'creature_spell_only' that limits what it can pay for.

    Why a dedicated class and not a dict[str,int]: the restricted-mana
    list can't live in a flat dict, and pay-time choice logic for "any"
    is clearer with named buckets.
    """
    W: int = 0
    U: int = 0
    B: int = 0
    R: int = 0
    G: int = 0
    C: int = 0
    any: int = 0
    restricted: list = field(default_factory=list)

    def total(self) -> int:
        """Total untyped pool size (sum of every bucket incl. restricted).
        This is what the legacy `seat.mana_pool: int` API returns."""
        t = self.W + self.U + self.B + self.R + self.G + self.C + self.any
        for r in self.restricted:
            t += r.amount
        return t

    def clear(self) -> None:
        """Empty every bucket. Used by drain_pool for unexempted colors
        and by test resets."""
        self.W = self.U = self.B = self.R = self.G = self.C = self.any = 0
        self.restricted = []

    def clear_except(self, exempt_colors: set) -> None:
        """Empty every bucket whose color is NOT in `exempt_colors`.
        exempt_colors is a set of single-character color codes
        ("W"/"U"/"B"/"R"/"G"/"C"); "any" exempts all (Upwelling). When
        the empty set is passed, behaves like clear()."""
        if "any" in exempt_colors:
            return  # Upwelling: nothing empties.
        if "W" not in exempt_colors: self.W = 0
        if "U" not in exempt_colors: self.U = 0
        if "B" not in exempt_colors: self.B = 0
        if "R" not in exempt_colors: self.R = 0
        if "G" not in exempt_colors: self.G = 0
        if "C" not in exempt_colors: self.C = 0
        # Only clear "any" bucket if no colors at all are exempt.
        if not exempt_colors:
            self.any = 0
            self.restricted = []
            return
        # With partial exemption, "any" and restricted drain by default —
        # a "mana of any color" token isn't tagged to a specific color,
        # so it doesn't match a color-specific exemption. Exception:
        # Upwelling-style "all colors" exemption handled above.
        self.any = 0
        self.restricted = []

    def add(self, color: str, amount: int = 1) -> None:
        """Add `amount` mana of `color` to the appropriate bucket.
        color: "W"/"U"/"B"/"R"/"G"/"C"/"any"."""
        if amount <= 0:
            return
        if color == "W": self.W += amount
        elif color == "U": self.U += amount
        elif color == "B": self.B += amount
        elif color == "R": self.R += amount
        elif color == "G": self.G += amount
        elif color == "C": self.C += amount
        elif color in ("any", "*"): self.any += amount
        else:
            # Unknown color code — fall back to `any` for safety rather
            # than silently dropping the mana.
            self.any += amount

    def add_restricted(self, amount: int, color: Optional[str],
                       restriction: str, source_name: str = "") -> None:
        """Add restricted mana (Food Chain, Powerstone, etc.)."""
        if amount <= 0:
            return
        self.restricted.append(RestrictedMana(
            amount=amount, color=color,
            restriction=restriction, source_name=source_name))

    def can_pay_generic(self, amount: int,
                        spell_type: str = "generic") -> bool:
        """Can we pay `amount` generic mana? Generic accepts any bucket
        (respecting restrictions against this spell_type)."""
        if amount <= 0:
            return True
        avail = self.W + self.U + self.B + self.R + self.G + self.C + self.any
        for r in self.restricted:
            if _restriction_allows(r.restriction, spell_type, colorless=False):
                avail += r.amount
        return avail >= amount

    def can_pay_colored(self, color: str, amount: int,
                        spell_type: str = "generic") -> bool:
        """Can we pay `amount` pips of a specific color?"""
        if amount <= 0:
            return True
        avail = getattr(self, color, 0) + self.any
        for r in self.restricted:
            if r.color is None or r.color == color:
                if _restriction_allows(r.restriction, spell_type, colorless=False):
                    avail += r.amount
        return avail >= amount


def _restriction_allows(restriction: str, spell_type: str,
                        colorless: bool) -> bool:
    """Check whether mana with a given restriction may pay for a spell
    of the given type. spell_type ∈ {"creature","noncreature","instant",
    "sorcery","activated","generic"}. Unknown restriction defaults to
    allowing (conservative)."""
    r = (restriction or "").lower()
    if not r:
        return True
    if r == "creature_spell_only":
        return spell_type == "creature"
    if r in ("noncreature_or_artifact_activation",
             "non_creature_activation_only",
             "noncreature_activation_only"):
        return spell_type in ("noncreature", "activated", "instant", "sorcery")
    if r == "artifact_only":
        return spell_type == "artifact" or spell_type == "activated"
    if r == "instant_or_sorcery_only":
        return spell_type in ("instant", "sorcery")
    return True


@dataclass
class Seat:
    idx: int
    life: int = STARTING_LIFE
    library: list[CardEntry] = field(default_factory=list)
    hand: list[CardEntry] = field(default_factory=list)
    battlefield: list[Permanent] = field(default_factory=list)
    graveyard: list[CardEntry] = field(default_factory=list)
    exile: list[CardEntry] = field(default_factory=list)
    # Typed five-color pool (CR §106). The legacy `seat.mana_pool: int`
    # field is exposed as a BACKWARDS-COMPATIBLE PROPERTY below that
    # reads total() and writes `any` bucket — see @mana_pool.setter. New
    # code should prefer calling into `mana` directly for color control.
    mana: ManaPool = field(default_factory=ManaPool)
    lands_played_this_turn: int = 0
    lost: bool = False
    loss_reason: str = ""
    # CR 704.5b flag: set True by draw_cards when a player tries to draw from
    # an empty library. Consumed and cleared by state_based_actions. Each
    # attempt surviving into the next SBA check is a game loss.
    attempted_draw_from_empty_library: bool = False
    # CR 704.5c: poison counters (player). Not used in the 4-deck tournament
    # (no infect cards) but tracked for judge-grade correctness.
    poison_counters: int = 0
    # ------------------------------------------------------------------
    # §903 Commander-variant state. All fields no-op when this seat has
    # no commander (commander_names == []) — non-Commander games ignore
    # them entirely. Cite points:
    #   §903.4 — starting_life defaults to 40 in Commander; STARTING_LIFE
    #            (20) elsewhere. Driven by Game.commander_format flag.
    #   §903.6 — command_zone holds cards designated as commanders before
    #            they've been cast (and after returns per §903.9a/b).
    #   §903.8 — commander_tax[commander_name] is the count of previous
    #            casts from the command zone. Base cost + 2 × this count.
    #   §903.9 — commander_names is the set of card names the owner has
    #            designated as their commander(s). Owner-keyed, not
    #            controller-keyed: a Gilded Drake-stolen commander still
    #            returns to its original owner's command zone on would-
    #            change-zone replacement.
    #   §903.10a — commander_damage is a NESTED dict keyed by
    #              (dealer_seat → commander_name → int). 21 from one
    #              (dealer, name) bucket → loss. Partner pairs track
    #              damage INDEPENDENTLY — 15 Kraum + 10 Tymna = survival.
    # ------------------------------------------------------------------
    command_zone: list[CardEntry] = field(default_factory=list)
    # Names of the cards the OWNER of this seat designated as their
    # commander(s). Empty in non-Commander games. Length 1 for single
    # commander, length 2 for partner pairs (Kraum+Tymna, Ardenn+Rograkh,
    # Kinnan+Thrasios, Friends Forever, Doctor+Companion, Choose-a-
    # Background).
    commander_names: list[str] = field(default_factory=list)
    # Per-commander-name cast tax counter (§903.8). Only incremented by
    # successful casts from the command zone. Partner pairs keep TWO
    # independent entries so casting Kraum three times doesn't tax Tymna.
    commander_tax: dict = field(default_factory=dict)
    # Combat damage dealt to THIS seat, keyed (dealer_seat → commander_name
    # → int). 21 from ONE (dealer, name) bucket → loss per §704.6c SBA.
    # Partner-aware: Kraum and Tymna damage sit in separate sub-dict
    # entries even though they share the same dealer_seat.
    commander_damage: dict = field(default_factory=dict)
    # Per-seat starting life (§903.4 = 40 in Commander; 20 elsewhere).
    # Set at init; `life` is seeded from this when a Commander game is
    # bootstrapped via setup_commander_game().
    starting_life: int = STARTING_LIFE
    # §800.4 — set True after the seat's leave-the-game cleanup has run
    # (battlefield → graveyard, stack items exiled). Idempotency guard
    # so `check_end` can call the cleanup once even if multiple SBA
    # passes see `lost=True`.
    _left_game: bool = False
    # ------------------------------------------------------------------
    # Cast-count bookkeeping (CR §700.4, §702.40 Storm, and cast-trigger
    # observers like Storm-Kiln Artist, Young Pyromancer, Birgi, Monastery
    # Mentor, Niv-Mizzet Parun, Runaway Steam-Kin).
    #
    # `spells_cast_this_turn` — count of spells THIS SEAT has cast during
    #   the current turn. Incremented by cast_spell / cast_commander_from_
    #   command_zone after a cast actually lands on the stack (cost paid,
    #   card removed from origin zone). Storm copies do NOT increment this
    #   (they aren't cast — §706.10). Reset to 0 at this seat's turn_start.
    # `spells_cast_last_turn` — snapshot of the previous turn's final
    #   `spells_cast_this_turn`. Some combo pieces read this ("whenever a
    #   creature enters, if you cast a spell last turn…" — Raging River's
    #   friends). Snapshot is taken when a seat's NEW turn begins, before
    #   zeroing the current-turn counter.
    # ------------------------------------------------------------------
    spells_cast_this_turn: int = 0
    spells_cast_last_turn: int = 0
    # ------------------------------------------------------------------
    # Pluggable AI decision policy. The engine calls
    # `seat.policy.method(...)` at every decision site — the concrete
    # class (GreedyHat / PokerHat / LLMHat / HumanUIHat /
    # ...) is owned by the hat author, not the engine. Swapping is a
    # one-line change: `seats[0].policy = PokerHat()`.
    # Defaults to GreedyHat (heuristic baseline) so existing call
    # sites keep working without explicit attachment. Excluded from
    # repr/compare since hats are opaque.
    # ------------------------------------------------------------------
    policy: Any = field(
        default=None, repr=False, compare=False,
    )

    def __post_init__(self) -> None:
        if self.policy is None and _DefaultPolicy is not None:
            self.policy = _DefaultPolicy()

    # ------------------------------------------------------------------
    # Legacy-compat mana_pool shim (CR §106).
    #
    # Historical call sites treat `seat.mana_pool: int` as a single
    # untyped counter: `s.mana_pool += 1`, `s.mana_pool = 0`,
    # `if s.mana_pool >= amount:`, etc. We keep that API working by
    # exposing a property that reads/writes the TYPED pool:
    #
    #   - Reads → total() across all buckets.
    #   - Writes: setting `seat.mana_pool = 0` clears the pool entirely.
    #     Setting `seat.mana_pool = N` (N > 0) REPLACES the pool with N
    #     mana of `any` color (legacy "generic" interpretation).
    #   - Augmented writes (`seat.mana_pool += 3` ==
    #     `seat.mana_pool = seat.mana_pool + 3`) therefore become
    #     "set pool to (total_before + 3) any-color mana". When the
    #     caller was adding mana that's correct. When the caller was
    #     subtracting (`-= N`), we use _debit_any() on the typed pool to
    #     spend the smallest-restriction unit first — see setter below.
    #
    # New code should prefer `seat.mana` directly, or the helpers
    # `_pay_cost_from_pool` / `add_mana_to_pool`.
    # ------------------------------------------------------------------
    @property
    def mana_pool(self) -> int:
        return self.mana.total()

    @mana_pool.setter
    def mana_pool(self, value: int) -> None:
        current = self.mana.total()
        if value <= 0:
            self.mana.clear()
            return
        if value == current:
            return
        if value > current:
            # Additive path (legacy "+= N"). Credit the delta as any-color.
            self.mana.add("any", value - current)
            return
        # Subtractive path (legacy "-= N"). Debit `delta` from the pool,
        # preferring unrestricted colorless / any first, then colors,
        # then restricted (callers should migrate to pay_cost_from_pool
        # for colored spending — this is a conservative fallback).
        delta = current - value
        self._debit_untyped(delta)

    def _debit_untyped(self, amount: int) -> None:
        """Remove `amount` mana from the pool in a restriction-neutral
        order. Used by the legacy mana_pool setter when the caller does
        `seat.mana_pool -= N`. Prefers spending `any`/`C` first so that
        colored mana survives longer — real cost resolution should go
        through pay_cost_from_pool instead."""
        order = ("any", "C", "W", "U", "B", "R", "G")
        for bucket in order:
            if amount <= 0:
                return
            have = getattr(self.mana, bucket)
            if have <= 0:
                continue
            spend = min(have, amount)
            setattr(self.mana, bucket, have - spend)
            amount -= spend
        # Finally drain restricted buckets (FIFO) if still needed.
        while amount > 0 and self.mana.restricted:
            r = self.mana.restricted[0]
            spend = min(r.amount, amount)
            r.amount -= spend
            amount -= spend
            if r.amount <= 0:
                self.mana.restricted.pop(0)


@dataclass
class StackItem:
    """A spell (or eventually ability) on the stack, awaiting resolution."""
    card: "CardEntry"
    controller: int                 # seat that cast it
    is_permanent_spell: bool        # resolves to battlefield vs graveyard
    effects: list                   # list of Effect nodes to resolve on hit
    countered: bool = False         # set True by a resolving counterspell
    # CR §706.10 — a copy of a spell is NOT a real card. It ceases to
    # exist on resolution rather than being put in its owner's graveyard
    # (which would violate zone conservation because the copy isn't in
    # any starting deck). Storm copies / Twinflame copies / Dualcaster
    # Mage copies all set this True via _apply_storm_copies etc.
    is_copy: bool = False


@dataclass
class Game:
    seats: list[Seat]
    active: int = 0
    turn: int = 1
    # NOTE: `phase` is the legacy flat string ("untap"/"upkeep"/"main1"/
    # "combat"/"main2"/"end"/"beginning"). Kept as a writable attribute so
    # older callers that do `game.phase = "x"` still work. For judge-grade
    # coordinates, new code should read `phase_kind` and `step_kind`
    # (see set_phase_step() below, which updates all three in lockstep).
    # Per CR §500–§514: Magic divides a turn into 5 phases (beginning,
    # precombat main, combat, postcombat main, ending). Beginning has
    # 3 steps (untap/upkeep/draw), combat has 5 (begin/declare atk/
    # declare blk/combat damage/end of combat), ending has 2 (end / cleanup).
    phase: str = "beginning"
    phase_kind: str = "beginning"   # §500.1: one of the 5 canonical phases
    step_kind: str = ""             # sub-step name or "" for main phases
    # `priority_round` bumps each time priority is passed back to the
    # active player with stack unchanged; used in Lasagna event coords.
    priority_round: int = 0
    # Opaque game_id for the Lasagna event coordinate schema. Callers may
    # set this to any stable identifier (tournament_id + matchup + game#).
    game_id: Optional[str] = None
    log: list[str] = field(default_factory=list)
    unknown_nodes: Counter = field(default_factory=Counter)
    events: list[dict] = field(default_factory=list)
    verbose: bool = False
    ended: bool = False
    winner: Optional[int] = None
    end_reason: str = ""
    stack: list["StackItem"] = field(default_factory=list)
    # Per-game seeded RNG. All effect-time randomization (tutor shuffles,
    # on-demand library shuffles, etc.) must go through this instance rather
    # than the global `random` module to preserve reproducibility under --seed.
    rng: random.Random = field(default_factory=random.Random)
    # Set transiently while a counter_spell effect is resolving so the
    # resolver knows which stack item it's pointed at.
    _pending_counter_target: Optional["StackItem"] = None
    # Monotonic stamp issued to every Permanent that enters the battlefield.
    # Used by CR 704.5j (Legend Rule — owner keeps the lowest-timestamp copy
    # by default) and CR 704.5k (World Rule — oldest world permanent dies).
    _timestamp_counter: int = 0
    # §614 replacement-effect registry. Each ReplacementEffect instance
    # watches a specific event_type (e.g. "would_draw", "would_die",
    # "would_enter_battlefield") and may rewrite the event when it fires.
    # Populated by register_replacements_for_permanent() at ETB and cleared
    # by unregister_replacements_for_permanent() at LTB. See §614 framework
    # block below for the full contract.
    replacement_effects: list["ReplacementEffect"] = field(default_factory=list)
    # Per-event recursion depth guard for fire_event, so a self-referential
    # replacement chain can't loop forever (CR 614.5 says each replacement
    # applies once per event, which we enforce via per-event applied-set,
    # but the depth cap is a belt-and-suspenders safety net).
    _fire_event_depth: int = 0
    # §903 Commander-variant flag. When True, state_based_actions applies
    # §704.6c (21 commander damage → loss) and §704.6d (commander in
    # graveyard/exile → may return to command zone). cast_spell and its
    # cost helpers consult this to enforce §903.8 commander tax and to
    # allow casting from the command zone. Non-Commander games leave
    # this False and pay zero runtime cost.
    commander_format: bool = False
    # Commander damage checkpoint: next event seq we haven't yet
    # attributed to seats[t].commander_damage. Incremented by the
    # §704.6c SBA helper after each scan so we don't double-count.
    _commander_damage_next_seq: int = 0
    # §613 continuous-effect registry. Any static/triggered/activated effect
    # that modifies a permanent's characteristics (P/T, type, color,
    # abilities, …) registers a ContinuousEffect here. Queried at
    # characteristic-resolution time via get_effective_characteristics.
    # Unregistered at LTB alongside replacement_effects.
    continuous_effects: list["ContinuousEffect"] = field(default_factory=list)
    # §603.7 delayed trigger queue. Entries added as spells/abilities
    # resolve (e.g. Sneak Attack's "sacrifice at end of turn"); drained at
    # phase/step boundaries by _fire_delayed_triggers().
    delayed_triggers: list["DelayedTrigger"] = field(default_factory=list)
    # Cache for get_effective_characteristics() — invalidated any time a
    # continuous effect is added/removed or a permanent changes state.
    # Key: id(perm); Value: dict of resolved characteristics.
    _char_cache: dict = field(default_factory=dict)
    # Combat-phase counter (0 = no combat has happened this turn, 1 = first
    # combat phase in progress/done, 2 = extra combat, …). Reset to 0 at
    # the start of each turn (untap). CR §506.1 + Aggravated Assault.
    combat_phase_number: int = 0
    # Pending extra combat phases queued by effects like Seize the Day /
    # Aggravated Assault / Relentless Assault. Consumed by take_turn().
    pending_extra_combats: int = 0
    # Per-turn bookkeeping for "until end of turn" duration tracking —
    # transient list of Modifier IDs scheduled to expire at cleanup.
    # See scan_expired_durations() / §514.2.
    _active_modifiers: list["Modifier"] = field(default_factory=list)
    # Global cast-count counter (CR §700.4 + §702.40 Storm). This counts
    # EVERY spell cast this turn, regardless of controller. It is what
    # Storm reads at cast time to determine how many copies to make
    # ((game.spells_cast_this_turn - 1) copies — the storm spell itself
    # is the "current" count, copies are of all prior casts this turn).
    # Resets at turn_start. Storm copies do NOT increment (§706.10 — a
    # copy isn't cast). Mana abilities and activated/triggered abilities
    # don't increment either — only spells going on the stack via
    # cast_spell / cast_commander_from_command_zone.
    spells_cast_this_turn: int = 0

    # CR §726 Day / Night designation. Begins as "neither" per §726.2
    # and transitions on specific boundaries:
    #   - If game state is "neither" and a permanent with daybound or
    #     nightbound enters the battlefield (or was already present at
    #     first-relevant moment), game becomes "day" (§726.2).
    #   - If game state is "day" at the start of the turn AND the
    #     PREVIOUS turn's active player cast no spells during their
    #     own turn, game becomes "night" (§726.3a).
    #   - If game state is "night" at the start of the turn AND the
    #     PREVIOUS turn's active player cast two or more spells during
    #     their own turn, game becomes "day" (§726.3a).
    # Valid values: "neither", "day", "night".
    day_night: str = "neither"
    # Snapshot of spells cast by the active player during the turn that
    # is about to END — used by the §726.3a day↔night transition check
    # which runs at the START of the next turn (i.e. compares against
    # "last turn"). Captured at turn-end before the active player
    # rotates; consumed at next turn's untap-before-untap hook.
    spells_cast_by_active_last_turn: int = 0

    def next_timestamp(self) -> int:
        self._timestamp_counter += 1
        return self._timestamp_counter

    def emit(self, msg: str) -> None:
        self.log.append(f"T{self.turn} P{self.active} [{self.phase}] {msg}")
        if self.verbose:
            print(self.log[-1])

    # --- JSON event stream (additive; no effect unless --json-log is used) ---
    def ev(self, type_: str, **fields) -> None:
        """Append a structured event. seq/turn/phase/seat filled automatically.

        Lasagna event schema (additive — backwards-compatible):

        WHERE coordinates (auto-filled):
            seq            — monotonically-increasing per-game event index.
            game_id        — opaque game identifier (or None).
            turn           — 1-indexed turn number.
            phase          — legacy flat phase string (for back-compat).
            phase_kind     — one of {beginning, precombat_main, combat,
                             postcombat_main, ending} (CR §500.1).
            step_kind      — sub-step string (untap, upkeep, draw,
                             begin_of_combat, declare_attackers,
                             declare_blockers, combat_damage,
                             first_strike_damage, end_of_combat, end,
                             cleanup) or "" for main phases.
            priority_round — bumps when priority cycles back to active.
            seat           — active player seat index.

        WHAT/WHO/WHEN (caller-supplied via **fields); judge-grade callers
        pass any of:
            source_card, source_permanent_id, controller,
            timestamp      — CR §613.7a timestamp, not just seq.
            layer          — §613 layer for P/T/type/color/ability events.
            target         — {object_id, kind, name, controller, pre_state,
                              post_state, active_modifications}.
            depends_on     — §613.8 dependency tracking (may be None).

        Pre-existing callers pass whatever kwargs they want; this function
        just auto-fills the WHERE coordinates.
        """
        evt = {
            "seq": len(self.events),
            "game_id": self.game_id,
            "turn": self.turn,
            "phase": self.phase,
            "phase_kind": self.phase_kind,
            "step_kind": self.step_kind,
            "priority_round": self.priority_round,
            "seat": self.active,
            "type": type_,
        }
        evt.update(fields)
        self.events.append(evt)
        # Pluggable-policy observation hook. Every seat's policy is
        # notified of every event so policies can update internal state
        # (mode transitions, memory, learning signals, etc.). The engine
        # itself is ignorant of what a policy does with the event — it
        # just fires the hook. Failures are swallowed so a buggy policy
        # never takes down the engine.
        for _pol_seat in self.seats:
            pol = getattr(_pol_seat, "policy", None)
            if pol is None:
                continue
            try:
                pol.observe_event(self, _pol_seat, evt)
            except Exception:
                pass

    def snapshot(self) -> None:
        """Append a full-state snapshot event so the viewer can re-sync.

        N-seat aware: emits `seats` as a list of per-seat dicts, plus the
        legacy `seat_0` / `seat_1` keys when exactly 2 seats exist so
        existing viewer/auditor consumers don't break."""
        def seat_state(s: Seat) -> dict:
            return {
                "idx": s.idx,
                "life": s.life,
                "hand": len(s.hand),
                "library": len(s.library),
                "graveyard": [c.name for c in s.graveyard],
                "battlefield": [
                    {"name": p.card.name, "tapped": p.tapped,
                     "summoning_sick": p.summoning_sick,
                     "power": p.power, "toughness": p.toughness,
                     "damage": p.damage_marked}
                    for p in s.battlefield
                ],
                "mana_pool": s.mana_pool,
                "lost": s.lost,
            }
        payload = {"seats": [seat_state(s) for s in self.seats]}
        if len(self.seats) == 2:
            # Back-compat: old viewers read seat_0/seat_1 directly.
            payload["seat_0"] = seat_state(self.seats[0])
            payload["seat_1"] = seat_state(self.seats[1])
        self.ev("state", **payload)

    def opp(self, seat_idx: int) -> Seat:
        """Primary opponent.

        2-player: the other seat (back-compat).
        N-player: the first *living* non-source seat in APNAP order from
        `seat_idx`. If no living opponents remain, falls back to the
        source seat itself so callers that blindly read `.life` don't
        crash — the game should already be ended in that case.

        Callers that need ALL opponents (threat iteration, "each
        opponent" effects, N-way priority) should use ``opponents()``
        or ``living_opponents()``, not ``opp()``.
        """
        if len(self.seats) == 2:
            return self.seats[1 - seat_idx]
        # N-player: cycle from seat_idx + 1 in APNAP order, prefer
        # living opponents. Fall back to any non-source seat if all
        # non-source seats are already dead.
        n = len(self.seats)
        for k in range(1, n):
            cand = self.seats[(seat_idx + k) % n]
            if cand.idx == seat_idx:
                continue
            if not cand.lost:
                return cand
        # All opponents eliminated — return the next non-source seat
        # as a degenerate fallback.
        for k in range(1, n):
            cand = self.seats[(seat_idx + k) % n]
            if cand.idx != seat_idx:
                return cand
        return self.seats[seat_idx]

    def opponents(self, seat_idx: int) -> list[Seat]:
        """All non-source seats (living + dead). APNAP order from seat_idx."""
        n = len(self.seats)
        return [self.seats[(seat_idx + k) % n] for k in range(1, n)
                if (seat_idx + k) % n != seat_idx]

    def living_opponents(self, seat_idx: int) -> list[Seat]:
        """All living non-source seats in APNAP order from seat_idx."""
        return [s for s in self.opponents(seat_idx) if not s.lost]

    def living_seats(self) -> list[Seat]:
        """All seats (including active) that haven't been eliminated."""
        return [s for s in self.seats if not s.lost]

    def apnap_order(self, from_seat: Optional[int] = None) -> list[Seat]:
        """Return seats in APNAP order (active player, then non-active in
        turn order). If ``from_seat`` is given, it's used as the anchor;
        otherwise ``self.active`` anchors the sequence. Dead seats are
        INCLUDED — callers filter if they care about responsiveness."""
        anchor = self.active if from_seat is None else from_seat
        n = len(self.seats)
        return [self.seats[(anchor + k) % n] for k in range(n)]

    # --- Phase/step coordination (CR §500–§514) --------------------------
    def set_phase_step(self, phase_kind: str, step_kind: str = "",
                       *, legacy_phase: Optional[str] = None) -> None:
        """Update (phase_kind, step_kind, legacy phase) atomically and fire
        the boundary-transition hooks:
          1. scan_expired_durations() — §514.2 and all other duration kinds
          2. _fire_delayed_triggers() — §603.7

        ``legacy_phase`` overrides the auto-derived flat-phase string so
        existing event-stream consumers (auditor, tests) keep seeing their
        expected labels (untap / upkeep / draw / main1 / combat / main2 /
        end). If omitted, a sensible default is picked.
        """
        # Compute backwards-compatible flat phase string.
        if legacy_phase is None:
            if phase_kind == "beginning":
                legacy_phase = step_kind or "beginning"
            elif phase_kind == "precombat_main":
                legacy_phase = "main1"
            elif phase_kind == "combat":
                legacy_phase = "combat"
            elif phase_kind == "postcombat_main":
                legacy_phase = "main2"
            elif phase_kind == "ending":
                legacy_phase = step_kind or "end"
            else:
                legacy_phase = phase_kind
        prev_phase_kind = self.phase_kind
        prev_step_kind = self.step_kind
        self.phase_kind = phase_kind
        self.step_kind = step_kind
        self.phase = legacy_phase
        self.ev("phase_step_change",
                from_phase_kind=prev_phase_kind,
                from_step_kind=prev_step_kind,
                to_phase_kind=phase_kind,
                to_step_kind=step_kind,
                rule="500.1")
        # §106.4 — at the end of every phase and step, each player's
        # mana pool empties. Exemptions (Upwelling, Omnath) are honored
        # per seat via _pool_exempt_colors(). We drain BEFORE running
        # scan_expired_durations so that if Upwelling itself leaves the
        # battlefield this very boundary, the next boundary is the one
        # that drains — but if we're here and Upwelling was on the
        # battlefield during the phase, its static still applies.
        drain_all_pools(self, prev_phase_kind, prev_step_kind)
        # Duration-tracking sweep at each boundary. §514.2 for cleanup,
        # §514.2 + §500.2 for other phase boundaries. Delayed triggers
        # fire next — CR 603.7 fires them when the trigger_at boundary
        # (e.g. "at the beginning of the next end step") arrives.
        scan_expired_durations(self, phase_kind, step_kind)
        _fire_delayed_triggers(self, phase_kind, step_kind)

    def invalidate_characteristics_cache(self) -> None:
        """Mark the §613 characteristics cache stale. Called automatically
        by register/unregister_continuous_effect() and by any code that
        adds/removes counters or changes timestamps mid-SBA."""
        self._char_cache.clear()

    def check_end(self) -> None:
        """§104.2a — last seat standing wins. §800.4 — when a player
        leaves the game, all objects they control leave the game.
        Cleanup is idempotent and safe to call on every SBA pass.

        Works for N seats: ≥2 living seats = game continues, ≤1 living
        = ended. For 0 living seats (simultaneous elimination), the
        winner is None and the game is a draw."""
        # Run §800.4 leave-the-game cleanup for any seat whose `lost`
        # flag flipped since the last pass. Idempotent via the
        # `_left_game` guard.
        for s in self.seats:
            if s.lost and not getattr(s, "_left_game", False):
                _handle_seat_elimination(self, s)
        alive = [s for s in self.seats if not s.lost]
        if len(alive) <= 1:
            self.ended = True
            if alive:
                self.winner = alive[0].idx
                self.end_reason = (
                    f"seat {self.winner} is the last one standing")
            else:
                self.winner = None
                self.end_reason = (
                    f"all {len(self.seats)} seats dead simultaneously (draw)")


# ============================================================================
# §800.4 — Leave-the-game procedure (multiplayer)
# ============================================================================
#
# CR 800.4a — "When a player leaves the game, all objects (see rule 109)
#             owned by that player leave the game and any effects which
#             give that player control of any objects or players end.
#             Then, if that player controlled any objects on the stack not
#             represented by cards, those objects cease to exist. Then, if
#             there are any objects still controlled by that player, those
#             objects are exiled."
# CR 800.4e — "If a player leaves the game during their turn, that turn
#             continues to its completion without an active player."
#             (We model this as the turn-rotation code skipping eliminated
#             seats; an active seat that loses mid-turn does NOT abort the
#             remainder of their turn in the strictest sense, but for
#             simulation sanity we let the current take_turn() unwind via
#             check_end short-circuits.)

def _handle_seat_elimination(game: "Game", seat: "Seat") -> None:
    """Apply §800.4a cleanup when ``seat`` leaves the game.

    Idempotent via ``seat._left_game``. Called from ``check_end`` whenever
    a seat's ``lost`` flag is observed True for the first time.

    Actions:
      1. Remove every permanent the seat CONTROLS from the battlefield.
         Per CR 800.4a these leave the game entirely (OWNED-by-seat
         objects — and they're controlled by seat, so they're owned by
         the eliminated player in our MVP). Unregister §614 / §613
         effect registrations so dangling hooks don't fire.
         Tokens simply cease to exist (CR 704.5d handles this via the
         SBA sweep; we drop them here too for belt-and-suspenders).
      2. Remove every stack item this seat CONTROLS. Per §800.4a these
         cease to exist (for abilities) or are exiled (for spells).
         We drop them from the stack — any downstream counter/response
         logic treats them as gone.
      3. Clear §614 replacement effects whose source permanent was just
         removed (step 1 already does this per-permanent).
      4. Clear §613 continuous effects sourced from this seat.
      5. Emit `seat_eliminated` event with the loss_reason + rule cite.

    Note on OWNERSHIP vs CONTROL (CR 108.3): a Gilded-Drake-style control
    swap would technically require the seat's OWNED objects to leave
    regardless of current controller. MVP simplification: we clean up
    by CONTROL (which is usually the same thing in practice), plus we
    ALSO walk every battlefield for permanents whose `owner_seat`
    equals the leaving player. That captures the stolen-Drake case.
    """
    if getattr(seat, "_left_game", False):
        return
    seat._left_game = True  # type: ignore[attr-defined]

    # --- Step 1: remove controlled permanents ----------------------------
    # We walk EVERY seat's battlefield because the leaving player might
    # still OWN cards that an opponent currently controls (§108.3). Both
    # cases: (a) seat controls it, (b) another seat controls it but seat
    # owns it.
    removed_count = 0
    for other in game.seats:
        for p in list(other.battlefield):
            if p.controller == seat.idx or p.owner_seat == seat.idx:
                unregister_replacements_for_permanent(game, p)
                try:
                    other.battlefield.remove(p)
                except ValueError:
                    continue
                removed_count += 1

    # --- Step 2: purge stack items controlled by the leaving seat --------
    if game.stack:
        before = len(game.stack)
        game.stack = [item for item in game.stack
                      if item.controller != seat.idx]
        purged = before - len(game.stack)
        if purged:
            game.ev("stack_purged_on_leave", seat=seat.idx,
                    purged=purged, rule="800.4a")

    # --- Step 3: drop §613 continuous effects this seat sourced ---------
    if game.continuous_effects:
        before = len(game.continuous_effects)
        game.continuous_effects = [
            ce for ce in game.continuous_effects
            if getattr(ce, "controller_seat", None) != seat.idx
        ]
        if len(game.continuous_effects) != before:
            game.invalidate_characteristics_cache()

    # --- Step 4: drop replacement effects this seat sourced -------------
    if game.replacement_effects:
        game.replacement_effects = [
            re for re in game.replacement_effects
            if getattr(re, "controller_seat", None) != seat.idx
        ]

    # --- Step 5: drop delayed triggers owned by the leaving seat -------
    if game.delayed_triggers:
        game.delayed_triggers = [
            dt for dt in game.delayed_triggers
            if getattr(dt, "controller_seat", seat.idx) != seat.idx
        ]

    # --- Emit the observation event ------------------------------------
    game.ev("seat_eliminated", seat=seat.idx,
            reason=getattr(seat, "loss_reason", ""),
            permanents_removed=removed_count,
            rule="800.4a")
    game.emit(f"seat {seat.idx} leaves the game "
              f"({getattr(seat, 'loss_reason', '?')}); "
              f"{removed_count} permanents removed (§800.4a)")


# ============================================================================
# §614 Replacement-effect framework
# ============================================================================
#
# Comp rules citations (MagicCompRules-20260227.txt):
#   614.1       Replacement effects watch for an event and substitute a
#               modified event ("instead").
#   614.1a-e    Categories: "instead", "skip", "enters with ...", "as ...
#               enters", "as ... is turned face up".
#   614.5       A replacement effect doesn't invoke itself repeatedly; it
#               gets only one opportunity to affect an event or any
#               modified events that may replace that event.
#   614.6       The modified event may in turn trigger abilities.
#   614.7       If the event never happens, the replacement does nothing.
#   614.8       Regeneration is a destruction-replacement effect.
#   614.9       Redirection effects.
#   614.10      "Skip" effects.
#   614.11      Replacing card draws.
#   614.12      Modifying how a permanent enters the battlefield.
#   614.15      Self-replacement effects (applied before other replacements).
#   614.16      Replacements on "if an effect would create tokens/counters".
#   614.17      "Can't" effects.
#   616.1       When multiple replacements apply, the affected object's
#               controller (or the affected player) chooses one to apply,
#               with sub-steps (self-replacement first, then control-of-ETB,
#               then copy-as-ETB, then back-face-up, then any-remaining).
#   616.1f      Once the chosen effect has been applied, repeat until no
#               more applicable replacements remain.
#   101.4       APNAP order for simultaneous choices.
#
# Architecture overview
# ---------------------
#
#   ReplacementEffect  — a single registered replacement. Fields:
#       event_type    : string matching Event.type (e.g. "would_draw").
#       source_perm   : the Permanent generating this effect (used for
#                       §101.4 APNAP ordering via source.controller, and
#                       for "this permanent" self-referential checks).
#       source_card_name : for diagnostics + per_card dispatch.
#       applies       : callable (game, event) -> bool. Should return True
#                       iff this replacement applies to the given event.
#       apply_fn      : callable (game, event) -> None. Mutates the event
#                       in place (setting event.cancelled = True to fully
#                       replace, or rewriting event.kwargs to modify).
#       category      : one of {"self_replacement", "control_etb",
#                       "copy_etb", "back_face_up", "other"} for §616.1
#                       sub-step ordering. "other" is the common case.
#       timestamp     : §613.7 timestamp for tie-breaking.
#
#   Event             — a mutable dict-wrapper carrying the pending mutation.
#                       Fields include:
#                         type       : string (e.g. "would_draw")
#                         player     : int | None — affected player's seat.
#                         target     : Permanent | None — affected object.
#                         cancelled  : bool — if True, the original mutation
#                                      is skipped (e.g. "skip your draw").
#                         <kwargs>   : event-specific fields (amount, count,
#                                      destination_zone, etc.)
#                       The fire_event() driver returns the final (possibly
#                       modified) event. The caller then executes the
#                       residual mutation against game state.
#
#   fire_event(game, event) -> Event:
#       1. Save a snapshot of this event's already-applied replacements set
#          (§614.5 — each replacement applies once per event-lineage).
#       2. Loop: collect all replacements that apply to `event` and haven't
#          already applied.
#          - If none → break.
#          - Else §616.1: pick the first §616.1 sub-category with any
#            matching replacement. Within that sub-category, use APNAP to
#            select the chooser. For now we pick deterministically by
#            (category rank, source timestamp, source seat) so tests are
#            reproducible. This is a principled APNAP-tiebreak, not a
#            full interactive ordering yet.
#          - Mark chosen as applied; call apply_fn.
#          - If event.cancelled → return immediately.
#       3. Return the (possibly modified) event.
#
# Hard invariants
# ---------------
# - A replacement NEVER mutates game state directly. It mutates the Event.
#   The caller of fire_event is responsible for carrying out the residual
#   mutation (or not, if cancelled).
# - register_replacements_for_permanent() is idempotent w.r.t. duplicate
#   registrations (we key by (source_perm, event_type, handler_id)).
# - unregister is called from both _destroy_perm and any other LTB path;
#   any LTB that doesn't route through _destroy_perm must also call
#   unregister_replacements_for_permanent.
# ============================================================================


@dataclass
class Event:
    """A pending mutation that might be replaced by §614 effects.

    Fields used by the framework:
      type        — string identifier (e.g. "would_draw", "would_die").
      player      — seat index of the affected player, or None.
      target      — Permanent being affected, or None.
      source      — Permanent that generated the event (attacker, ETBing
                    creature, spell resolving, etc.), or None.
      cancelled   — set to True by a replacement that wants to fully
                    replace the event (e.g. "instead of drawing, scry 2").
      applied_ids — set of handler_ids that have already applied to this
                    event lineage (CR 614.5 — each applies at most once).

    Extra kwargs carry event-specific data (amount, count, etc.) and are
    accessed via .kwargs dict for uniformity.
    """
    type: str
    player: Optional[int] = None
    target: Optional["Permanent"] = None
    source: Optional["Permanent"] = None
    cancelled: bool = False
    kwargs: dict = field(default_factory=dict)
    applied_ids: set = field(default_factory=set)

    def get(self, key, default=None):
        return self.kwargs.get(key, default)

    def set(self, key, value):
        self.kwargs[key] = value


@dataclass
class ReplacementEffect:
    """A single registered §614 replacement.

    - event_type      : which Event.type this watches.
    - handler_id      : unique string ("{card_name}:{slug}") for §614.5
                        "applied once per event" tracking.
    - source_perm     : the permanent generating this effect (may be None
                        for effects coming from a player or from the rules
                        themselves).
    - source_card_name: for diagnostics.
    - controller_seat : the seat that controls this replacement (used for
                        §101.4 APNAP ordering in §616.1).
    - timestamp       : §613.7 timestamp for tie-breaking.
    - category        : §616.1 sub-category for ordering. One of:
                        'self_replacement', 'control_etb', 'copy_etb',
                        'back_face_up', 'other'.
    - applies         : (game, event) -> bool. Return True to apply.
    - apply_fn        : (game, event) -> None. Mutate the event.
    """
    event_type: str
    handler_id: str
    source_perm: Optional["Permanent"]
    source_card_name: str
    controller_seat: Optional[int]
    timestamp: int
    category: str
    applies: callable
    apply_fn: callable


_CATEGORY_RANK = {
    "self_replacement": 0,
    "control_etb": 1,
    "copy_etb": 2,
    "back_face_up": 3,
    "other": 4,
}


def fire_event(game: "Game", event: "Event") -> "Event":
    """Run `event` through the §614 replacement-effect chain.

    Returns the same event instance (possibly with mutated kwargs, possibly
    with cancelled=True). The caller inspects the returned event and
    performs the residual mutation on the engine state.

    Implements §614.5 (each replacement applies at most once per event
    lineage), §614.6 (modified event replaces original), §616.1 (ordering
    by sub-category + APNAP/timestamp tiebreak), §616.1f (repeat until no
    applicable replacements remain).
    """
    game._fire_event_depth += 1
    # Safety net — CR 614.5 should prevent infinite re-entry, but a buggy
    # handler that re-fires could still loop. 64 is comfortably above any
    # realistic replacement chain depth (Doubling Season + Hardened Scales
    # + Anointed Procession etc. would be depth ~4).
    if game._fire_event_depth > 64:
        game.ev("replacement_depth_cap",
                event_type=event.type, rule="614.5")
        game._fire_event_depth -= 1
        return event

    try:
        max_iter = 32
        iters = 0
        while iters < max_iter:
            iters += 1
            if event.cancelled:
                return event
            applicable = []
            for re_ in game.replacement_effects:
                if re_.event_type != event.type:
                    continue
                if re_.handler_id in event.applied_ids:
                    continue  # 614.5
                try:
                    if not re_.applies(game, event):
                        continue
                except Exception as exc:
                    game.ev("replacement_applies_crashed",
                            handler_id=re_.handler_id,
                            exception=f"{type(exc).__name__}: {exc}")
                    continue
                applicable.append(re_)
            if not applicable:
                return event

            # §616.1 sub-category selection: find the lowest-rank
            # sub-category that has any applicable replacement, and limit
            # candidates to that sub-category.
            applicable.sort(key=lambda r: (
                _CATEGORY_RANK.get(r.category, 4),
                r.timestamp,
                r.controller_seat if r.controller_seat is not None else -1,
            ))
            first_cat_rank = _CATEGORY_RANK.get(applicable[0].category, 4)
            same_cat = [r for r in applicable
                        if _CATEGORY_RANK.get(r.category, 4) == first_cat_rank]

            # §616.1: the affected player (or affected object's controller)
            # chooses among same-category replacements. §101.4 APNAP order
            # for simultaneous choices.
            chooser = _choose_replacement_chooser(game, event)
            chosen = _pick_replacement(same_cat, chooser, game)

            # §614.5: mark as applied BEFORE firing, so if apply_fn recurses
            # into fire_event, the recursion won't re-select this handler.
            event.applied_ids.add(chosen.handler_id)

            game.ev("replacement_applied",
                    event_type=event.type,
                    handler_id=chosen.handler_id,
                    source_card=chosen.source_card_name,
                    controller_seat=chosen.controller_seat,
                    rule="614",
                    chooser_seat=chooser)
            try:
                chosen.apply_fn(game, event)
            except Exception as exc:
                game.ev("replacement_apply_crashed",
                        handler_id=chosen.handler_id,
                        exception=f"{type(exc).__name__}: {exc}")

        if iters >= max_iter:
            game.ev("replacement_iter_cap",
                    event_type=event.type, iterations=iters,
                    rule="616.1f_cap")
        return event
    finally:
        game._fire_event_depth -= 1


def _choose_replacement_chooser(game: "Game",
                                event: "Event") -> Optional[int]:
    """§616.1 + §101.4: the affected player chooses. If the affected thing
    is an object, its controller chooses. Returns a seat index or None.
    """
    if event.player is not None:
        return event.player
    if event.target is not None and hasattr(event.target, "controller"):
        return event.target.controller
    # No explicit affected entity — fall back to active player (APNAP).
    return game.active


def _pick_replacement(candidates: list["ReplacementEffect"],
                      chooser_seat: Optional[int],
                      game: Optional["Game"] = None) -> "ReplacementEffect":
    """Choose one replacement from a same-sub-category list. §616.1e lets
    any applicable be chosen.

    The chooser's policy drives the order via
    ``policy.order_replacements``; we pick the first one the policy
    returns. Legacy fallback (and :class:`GreedyHat` default):
    self-controlled first, then oldest-timestamp ascending — matches
    the behavior prior to the policy layer.
    """
    if game is not None and chooser_seat is not None and 0 <= chooser_seat < len(game.seats):
        seat = game.seats[chooser_seat]
        if getattr(seat, "policy", None) is not None:
            try:
                ordered = seat.policy.order_replacements(
                    game, seat, list(candidates),
                )
                if ordered:
                    return ordered[0]
            except Exception:
                pass
    if chooser_seat is not None:
        own = [r for r in candidates if r.controller_seat == chooser_seat]
        if own:
            candidates = own
    return min(candidates, key=lambda r: (r.timestamp, r.handler_id))


def register_replacement_effect(game: "Game",
                                effect: "ReplacementEffect") -> None:
    """Add a replacement effect to the registry. Idempotent — if an effect
    with the same handler_id is already registered, this is a no-op."""
    for existing in game.replacement_effects:
        if existing.handler_id == effect.handler_id:
            return
    game.replacement_effects.append(effect)


def unregister_replacements_for_permanent(game: "Game",
                                          perm: "Permanent") -> int:
    """Drop every replacement whose source_perm is this permanent.
    Called from _destroy_perm and any other LTB path. Returns the count
    of removed handlers."""
    before = len(game.replacement_effects)
    game.replacement_effects = [
        r for r in game.replacement_effects if r.source_perm is not perm
    ]
    removed = before - len(game.replacement_effects)
    if removed:
        game.ev("replacement_unregistered",
                card=perm.card.name if perm.card else "?",
                count=removed, rule="614")
    return removed


def register_replacements_for_permanent(game: "Game",
                                        perm: "Permanent") -> int:
    """Consult _REPLACEMENT_REGISTRY_BY_NAME for per-card replacement
    bindings and register each one. Also walks the permanent's AST for
    Static(replacement_static) and Static(replacement) markers emitted by
    extensions/replacements.py to register generic ETB/would-die handlers.

    Returns the number of replacement effects registered.
    """
    registered = 0
    card_name = perm.card.name
    # 1. Hardcoded per-card replacements (Rest in Peace, Anafenza, Doubling
    # Season, etc.). Keyed by card name because the AST for most of these
    # either doesn't produce a clean Replacement node yet, or produces one
    # whose args don't capture the full semantics.
    factory_list = _REPLACEMENT_REGISTRY_BY_NAME.get(card_name, [])
    for factory in factory_list:
        for effect in factory(game, perm):
            register_replacement_effect(game, effect)
            registered += 1
    if registered:
        game.ev("replacement_registered",
                card=card_name, count=registered, rule="614")
    return registered


# ---------------------------------------------------------------------------
# Canonical replacement handlers — one function per (card, event-type).
# Each factory returns a list of ReplacementEffect instances to register.
# ---------------------------------------------------------------------------


def _next_handler_timestamp(game: "Game") -> int:
    # Piggyback on the timestamp counter so replacement ordering uses
    # the same monotonic source as 613.7 / 704.5j.
    return game.next_timestamp()


def _make_labman_win(game: "Game", perm: "Permanent"):
    """Laboratory Maniac / Jace, Wielder of Mysteries — 'If you would draw
    a card while your library has no cards in it, you win the game
    instead.'"""
    ts = _next_handler_timestamp(game)
    hid = f"{perm.card.name}:labman_win:{ts}"

    def applies(game, event):
        if event.type != "would_draw":
            return False
        if event.player != perm.controller:
            return False
        # Replacement fires only if the library is empty AT fire time.
        return len(game.seats[event.player].library) == 0

    def apply_fn(game, event):
        event.cancelled = True
        event.set("replaced_by", "labman_win")
        winner = perm.controller
        game.ev("per_card_win", slug="labman_win",
                card=perm.card.name, winner_seat=winner,
                reason="laboratory_maniac_framework")
        for other in game.seats:
            if other.idx != winner:
                other.lost = True
                other.loss_reason = "Laboratory Maniac opponent win"
        game.seats[winner].lost = False
        game.ended = True
        game.winner = winner
        game.end_reason = "Laboratory Maniac §614 replacement win"

    return [ReplacementEffect(
        event_type="would_draw",
        handler_id=hid,
        source_perm=perm,
        source_card_name=perm.card.name,
        controller_seat=perm.controller,
        timestamp=ts,
        category="other",
        applies=applies,
        apply_fn=apply_fn,
    )]


def _make_alhammarret_draw_doubler(game: "Game", perm: "Permanent"):
    """Alhammarret's Archive — 'If you would draw a card, draw two cards
    instead.' + 'If you would gain life, you gain twice that much life
    instead.' Two separate replacements, both registered.
    """
    ts_draw = _next_handler_timestamp(game)
    ts_life = _next_handler_timestamp(game)

    def draw_applies(game, event):
        if event.type != "would_draw":
            return False
        return event.player == perm.controller

    def draw_apply(game, event):
        # Replace "draw 1 card" with "draw 2 cards". Note that CR 614.5
        # prevents this from firing on the doubled event.
        event.set("count", event.get("count", 1) * 2)
        event.set("doubled_by", perm.card.name)

    def life_applies(game, event):
        if event.type != "would_gain_life":
            return False
        return event.player == perm.controller

    def life_apply(game, event):
        event.set("amount", event.get("amount", 0) * 2)
        event.set("doubled_by", perm.card.name)

    return [
        ReplacementEffect(
            event_type="would_draw",
            handler_id=f"{perm.card.name}:draw_double:{ts_draw}",
            source_perm=perm, source_card_name=perm.card.name,
            controller_seat=perm.controller, timestamp=ts_draw,
            category="other", applies=draw_applies, apply_fn=draw_apply,
        ),
        ReplacementEffect(
            event_type="would_gain_life",
            handler_id=f"{perm.card.name}:life_double:{ts_life}",
            source_perm=perm, source_card_name=perm.card.name,
            controller_seat=perm.controller, timestamp=ts_life,
            category="other", applies=life_applies, apply_fn=life_apply,
        ),
    ]


def _make_rhox_life_doubler(game: "Game", perm: "Permanent"):
    """Boon Reflection / Rhox Faithmender — 'If you would gain life,
    you gain twice that much life instead.'"""
    ts = _next_handler_timestamp(game)

    def applies(game, event):
        if event.type != "would_gain_life":
            return False
        return event.player == perm.controller

    def apply_fn(game, event):
        event.set("amount", event.get("amount", 0) * 2)
        event.set("doubled_by", perm.card.name)

    return [ReplacementEffect(
        event_type="would_gain_life",
        handler_id=f"{perm.card.name}:life_double:{ts}",
        source_perm=perm, source_card_name=perm.card.name,
        controller_seat=perm.controller, timestamp=ts,
        category="other", applies=applies, apply_fn=apply_fn,
    )]


def _make_rest_in_peace(game: "Game", perm: "Permanent"):
    """Rest in Peace — 'If a card or token would be put into a graveyard
    from anywhere, exile it instead.' Replaces the zone-destination on
    any would-be-put-into-graveyard event.
    """
    ts = _next_handler_timestamp(game)

    def applies(game, event):
        if event.type != "would_be_put_into_graveyard":
            return False
        # This replacement applies to BOTH players' cards (unlike
        # Leyline of the Void below which is opponents-only).
        return True

    def apply_fn(game, event):
        event.set("destination_zone", "exile")
        event.set("redirected_by", perm.card.name)

    return [ReplacementEffect(
        event_type="would_be_put_into_graveyard",
        handler_id=f"{perm.card.name}:gy_to_exile:{ts}",
        source_perm=perm, source_card_name=perm.card.name,
        controller_seat=perm.controller, timestamp=ts,
        category="other", applies=applies, apply_fn=apply_fn,
    )]


def _make_leyline_of_the_void(game: "Game", perm: "Permanent"):
    """Leyline of the Void — 'If a card that isn't a token would be put
    into an opponent's graveyard from anywhere, exile it instead.'"""
    ts = _next_handler_timestamp(game)

    def applies(game, event):
        if event.type != "would_be_put_into_graveyard":
            return False
        target_seat = event.get("owner_seat")
        if target_seat is None:
            return False
        # Opponents only.
        if target_seat == perm.controller:
            return False
        if event.get("is_token", False):
            return False
        return True

    def apply_fn(game, event):
        event.set("destination_zone", "exile")
        event.set("redirected_by", perm.card.name)

    return [ReplacementEffect(
        event_type="would_be_put_into_graveyard",
        handler_id=f"{perm.card.name}:opp_gy_to_exile:{ts}",
        source_perm=perm, source_card_name=perm.card.name,
        controller_seat=perm.controller, timestamp=ts,
        category="other", applies=applies, apply_fn=apply_fn,
    )]


def _make_anafenza(game: "Game", perm: "Permanent"):
    """Anafenza, the Foremost — 'If a creature an opponent controls would
    die, exile it instead.' A would-die replacement, NOT a would-be-put-
    into-graveyard replacement (though the practical effect overlaps).
    """
    ts = _next_handler_timestamp(game)

    def applies(game, event):
        if event.type != "would_die":
            return False
        victim = event.target
        if victim is None or not hasattr(victim, "controller"):
            return False
        if victim.controller == perm.controller:
            return False
        return getattr(victim, "is_creature", False)

    def apply_fn(game, event):
        event.set("destination_zone", "exile")
        event.set("exiled_by", perm.card.name)

    return [ReplacementEffect(
        event_type="would_die",
        handler_id=f"{perm.card.name}:opp_die_exile:{ts}",
        source_perm=perm, source_card_name=perm.card.name,
        controller_seat=perm.controller, timestamp=ts,
        category="other", applies=applies, apply_fn=apply_fn,
    )]


def _make_doubling_season(game: "Game", perm: "Permanent"):
    """Doubling Season — 'If an effect would create one or more tokens
    under your control, it creates twice that many of those tokens
    instead. If an effect would put one or more counters on a permanent
    you control, it puts twice that many of those counters on that
    permanent instead.'
    """
    ts_tok = _next_handler_timestamp(game)
    ts_ctr = _next_handler_timestamp(game)

    def token_applies(game, event):
        if event.type != "would_create_token":
            return False
        return event.get("controller_seat") == perm.controller

    def token_apply(game, event):
        event.set("count", event.get("count", 1) * 2)
        event.set("doubled_by", perm.card.name)

    def counter_applies(game, event):
        if event.type != "would_put_counter":
            return False
        tgt = event.target
        if tgt is None or not hasattr(tgt, "controller"):
            return False
        return tgt.controller == perm.controller

    def counter_apply(game, event):
        event.set("count", event.get("count", 1) * 2)
        event.set("doubled_by", perm.card.name)

    return [
        ReplacementEffect(
            event_type="would_create_token",
            handler_id=f"{perm.card.name}:token_double:{ts_tok}",
            source_perm=perm, source_card_name=perm.card.name,
            controller_seat=perm.controller, timestamp=ts_tok,
            category="other", applies=token_applies, apply_fn=token_apply,
        ),
        ReplacementEffect(
            event_type="would_put_counter",
            handler_id=f"{perm.card.name}:counter_double:{ts_ctr}",
            source_perm=perm, source_card_name=perm.card.name,
            controller_seat=perm.controller, timestamp=ts_ctr,
            category="other", applies=counter_applies, apply_fn=counter_apply,
        ),
    ]


def _make_hardened_scales(game: "Game", perm: "Permanent"):
    """Hardened Scales — 'If one or more +1/+1 counters would be put on
    a creature you control, that many plus one +1/+1 counters are put
    on it instead.'

    §614.6 interaction with Doubling Season is the canonical APNAP test:
      - Correct order: Hardened Scales FIRST (1 -> 2), then Doubling
        Season (2 -> 4). The controller chooses the order, so a sensible
        controller picks this sequence for more counters.
      - Wrong order: Doubling Season first (1 -> 2), then Hardened
        Scales (2 -> 3).
      Our _pick_replacement sorts by timestamp; a Hardened Scales that
      enters before Doubling Season is applied first. Tests seed both
      and verify the 4 outcome.
    """
    ts = _next_handler_timestamp(game)

    def applies(game, event):
        if event.type != "would_put_counter":
            return False
        if event.get("counter_kind") != "+1/+1":
            return False
        tgt = event.target
        if tgt is None or not hasattr(tgt, "controller"):
            return False
        if tgt.controller != perm.controller:
            return False
        return getattr(tgt, "is_creature", False)

    def apply_fn(game, event):
        event.set("count", event.get("count", 1) + 1)
        event.set("hardened_by", perm.card.name)

    return [ReplacementEffect(
        event_type="would_put_counter",
        handler_id=f"{perm.card.name}:plus_one:{ts}",
        source_perm=perm, source_card_name=perm.card.name,
        controller_seat=perm.controller, timestamp=ts,
        category="other", applies=applies, apply_fn=apply_fn,
    )]


def _make_panharmonicon(game: "Game", perm: "Permanent"):
    """Panharmonicon — 'If an artifact or creature entering the
    battlefield causes a triggered ability of a permanent you control to
    trigger, that ability triggers an additional time.'

    This isn't strictly §614 (it's §603 — triggered-ability modification),
    but it lives in the same bucket functionally and most engines
    implement it via the replacement-effect pathway. We register it as a
    'would_etb_trigger' replacement that increments a `copies` count on
    the ETB-trigger event.
    """
    ts = _next_handler_timestamp(game)

    def applies(game, event):
        if event.type != "would_etb_trigger":
            return False
        if event.get("controller_seat") != perm.controller:
            return False
        etbing = event.get("entering_type_line", "").lower()
        return "artifact" in etbing or "creature" in etbing

    def apply_fn(game, event):
        event.set("copies", event.get("copies", 1) + 1)
        event.set("doubled_by", perm.card.name)

    return [ReplacementEffect(
        event_type="would_etb_trigger",
        handler_id=f"{perm.card.name}:etb_double:{ts}",
        source_perm=perm, source_card_name=perm.card.name,
        controller_seat=perm.controller, timestamp=ts,
        category="other", applies=applies, apply_fn=apply_fn,
    )]


def _make_platinum_angel(game: "Game", perm: "Permanent"):
    """Platinum Angel — 'You can't lose the game and your opponents
    can't win the game.' Implemented as two replacements: would_lose
    and would_win (opponent-scope). Both cancel the event.
    """
    ts_lose = _next_handler_timestamp(game)
    ts_win = _next_handler_timestamp(game)

    def lose_applies(game, event):
        if event.type != "would_lose_game":
            return False
        return event.player == perm.controller

    def lose_apply(game, event):
        event.cancelled = True
        event.set("cancelled_by", perm.card.name)

    def win_applies(game, event):
        if event.type != "would_win_game":
            return False
        return event.player != perm.controller

    def win_apply(game, event):
        event.cancelled = True
        event.set("cancelled_by", perm.card.name)

    return [
        ReplacementEffect(
            event_type="would_lose_game",
            handler_id=f"{perm.card.name}:no_lose:{ts_lose}",
            source_perm=perm, source_card_name=perm.card.name,
            controller_seat=perm.controller, timestamp=ts_lose,
            category="other", applies=lose_applies, apply_fn=lose_apply,
        ),
        ReplacementEffect(
            event_type="would_win_game",
            handler_id=f"{perm.card.name}:no_opp_win:{ts_win}",
            source_perm=perm, source_card_name=perm.card.name,
            controller_seat=perm.controller, timestamp=ts_win,
            category="other", applies=win_applies, apply_fn=win_apply,
        ),
    ]


# ============================================================================
# §613 Continuous-effect + duration + §603.7 delayed-trigger framework
# ============================================================================
#
# Comp rules citations (MagicCompRules-20260227.txt):
#   613.1     Layer system — characteristics are resolved by starting with
#             printed values and applying continuous effects in layer order:
#             1 (copy) → 2 (control) → 3 (text) → 4 (type) → 5 (color) →
#             6 (ability add/remove) → 7a (CDA P/T) → 7b (set P/T) →
#             7c (modify P/T / counters) → 7d (switch P/T).
#             Our engine splits "counters" into a 7d sub-bucket and "switch"
#             into 7e to match the parser/layer_harness tagging (widely-
#             taught public Wizards cascade).
#   613.7     Timestamp assignment (7a static = object ts; 7b resolution =
#             creation time; zone-change = new ts).
#   613.7m    APNAP for simultaneous timestamps.
#   613.8     Dependency overrides timestamp within a layer.
#   514.2     During cleanup, damage wears off AND all "until end of turn"
#             / "this turn" effects end.
#   603.7     Delayed triggered abilities — created during resolution or
#             replacement, fire once at the next trigger event.
#   723.1     "End the turn" (Sundial of the Infinite, Time Stop) —
#             expedited resolution that skips to cleanup, ending "until
#             end of turn" effects like a normal cleanup would, but
#             suppresses end-step triggers. The rule is §723, not §713.
#   122.1d    Stun counters — replacement: "if would untap, remove a
#             stun counter instead".
# ============================================================================


# Duration kinds (authoritative enum). Names chosen to match the text that
# would appear on a card (or a reasonable canonicalization). Callers set
# Modifier.duration to one of these; scan_expired_durations() honors them.
DURATION_END_OF_TURN = "end_of_turn"
DURATION_UNTIL_YOUR_NEXT_TURN = "until_your_next_turn"
DURATION_UNTIL_END_OF_YOUR_NEXT_TURN = "until_end_of_your_next_turn"
DURATION_UNTIL_NEXT_END_STEP = "until_next_end_step"
DURATION_UNTIL_YOUR_NEXT_END_STEP = "until_your_next_end_step"
DURATION_UNTIL_NEXT_UPKEEP = "until_next_upkeep"
DURATION_UNTIL_SOURCE_LEAVES = "until_source_leaves"
DURATION_UNTIL_CONDITION_CHANGES = "until_condition_changes"
DURATION_PERMANENT = "permanent"

DURATION_KINDS = (
    DURATION_END_OF_TURN,
    DURATION_UNTIL_YOUR_NEXT_TURN,
    DURATION_UNTIL_END_OF_YOUR_NEXT_TURN,
    DURATION_UNTIL_NEXT_END_STEP,
    DURATION_UNTIL_YOUR_NEXT_END_STEP,
    DURATION_UNTIL_NEXT_UPKEEP,
    DURATION_UNTIL_SOURCE_LEAVES,
    DURATION_UNTIL_CONDITION_CHANGES,
    DURATION_PERMANENT,
)


@dataclass
class Modifier:
    """A single stateful modification with a duration.

    Used for "until end of turn" buffs/grants, fight-style temporary
    P/T mods, and anywhere callers previously mutated ``buffs_pt`` or
    ``granted`` directly. Registered on a Permanent via
    ``register_modifier()``; expires at the appropriate phase boundary.

    Fields:
      target        — Permanent the modifier applies to.
      duration      — one of DURATION_KINDS.
      kind          — free-form label ("buff_pt", "grant_kw", "tap",
                      "fight_buff", etc).
      data          — kind-specific payload dict.
      created_turn  — turn number when the modifier was registered.
      created_seat  — seat of the player who controlled the creating
                      spell/ability ("your" in durations is relative).
      source_perm   — the Permanent that generated this modifier, if any
                      (for DURATION_UNTIL_SOURCE_LEAVES).
      condition_fn  — optional callable(game) -> bool for
                      DURATION_UNTIL_CONDITION_CHANGES.
      timestamp     — §613.7 timestamp for tie-breaking inside a layer.
      expired       — set True by scan_expired_durations once cleared.
    """
    target: "Permanent"
    duration: str
    kind: str
    data: dict = field(default_factory=dict)
    created_turn: int = 0
    created_seat: int = 0
    source_perm: Optional["Permanent"] = None
    condition_fn: Optional[callable] = None
    timestamp: int = 0
    expired: bool = False


@dataclass
class ContinuousEffect:
    """A single registered §613 continuous effect.

    Fields:
      layer            — '1', '2', '3', '4', '5', '6', '7a', '7b', '7c',
                         '7d', '7e'. See §613.1/§613.4.
      timestamp        — §613.7 for tie-breaking.
      source_perm      — the Permanent generating this effect (or None).
      source_card_name
      controller_seat
      predicate        — callable(game, perm) -> bool.
      apply_fn         — callable(game, perm, chars) -> None. Mutates
                         the ``chars`` dict in place.
      duration         — DURATION_* constant. Defaults to "permanent"
                         for static abilities.
      handler_id       — stable string for idempotency / dependency
                         lookup.
      depends_on       — tuple of handler_ids this effect declares
                         dependency on (§613.8). Rarely used.
    """
    layer: str
    timestamp: int
    source_perm: Optional["Permanent"]
    source_card_name: str
    controller_seat: Optional[int]
    predicate: callable
    apply_fn: callable
    duration: str = DURATION_PERMANENT
    handler_id: str = ""
    depends_on: tuple = ()


@dataclass
class DelayedTrigger:
    """A single §603.7 delayed triggered ability.

    Fields:
      trigger_at       — one of: 'end_of_turn', 'end_of_combat',
                         'your_next_upkeep', 'your_next_end_step',
                         'next_end_step', 'next_upkeep',
                         'your_next_turn'.
      condition        — optional callable(game) -> bool gating the fire.
      effect_fn        — callable(game) -> None.
      controller_seat  — §603.7d.
      source_card_name
      source_permanent — the Permanent that created this (for tracing).
      source_timestamp — §613.7.
      created_turn     — turn number of registration.
      consumed         — set True by _fire_delayed_triggers when fired.
    """
    trigger_at: str
    effect_fn: callable
    controller_seat: int = 0
    source_card_name: str = ""
    source_permanent: Optional["Permanent"] = None
    source_timestamp: int = 0
    created_turn: int = 0
    condition: Optional[callable] = None
    consumed: bool = False


# ----------------------------------------------------------------------------
# Registration / lifecycle helpers
# ----------------------------------------------------------------------------


def register_modifier(game: "Game", mod: "Modifier") -> "Modifier":
    """Add a Modifier to the game's active list and apply its immediate
    side-effect (e.g. stamp buffs_pt on the target)."""
    if mod.timestamp == 0:
        mod.timestamp = game.next_timestamp()
    if mod.created_turn == 0:
        mod.created_turn = game.turn
    game._active_modifiers.append(mod)
    if mod.kind == "buff_pt" and mod.target is not None:
        p = mod.target
        dp = int(mod.data.get("power", 0))
        dt = int(mod.data.get("toughness", 0))
        p.buffs_pt = (p.buffs_pt[0] + dp, p.buffs_pt[1] + dt)
    elif mod.kind == "grant_kw" and mod.target is not None:
        kw_name = mod.data.get("keyword", "")
        if kw_name and kw_name not in mod.target.granted:
            mod.target.granted.append(kw_name)
    game.invalidate_characteristics_cache()
    game.ev("modifier_registered", kind=mod.kind,
            duration=mod.duration, timestamp=mod.timestamp,
            target=getattr(getattr(mod.target, "card", None),
                           "name", "?"),
            rule="613.7b")
    return mod


def _revert_modifier(game: "Game", mod: "Modifier") -> None:
    """Reverse a modifier's immediate side-effect."""
    if mod.expired:
        return
    p = mod.target
    if p is None:
        mod.expired = True
        return
    if mod.kind == "buff_pt":
        dp = int(mod.data.get("power", 0))
        dt = int(mod.data.get("toughness", 0))
        p.buffs_pt = (p.buffs_pt[0] - dp, p.buffs_pt[1] - dt)
    elif mod.kind == "grant_kw":
        kw_name = mod.data.get("keyword", "")
        if kw_name in p.granted:
            p.granted.remove(kw_name)
    mod.expired = True
    game.invalidate_characteristics_cache()


def _duration_expires_now(mod: "Modifier", phase_kind: str,
                          step_kind: str, game: "Game") -> bool:
    """Return True iff this modifier's duration elapses at the
    (phase_kind, step_kind) boundary we just entered.

    CR §514.2: "all damage marked on permanents is removed and all
    'until end of turn' and 'this turn' effects end". That's the
    canonical cleanup boundary.

    Sundial of the Infinite's end-the-turn path (§723.1) routes into
    cleanup step; DURATION_END_OF_TURN expires there regardless of
    which codepath got us there.
    """
    d = mod.duration
    if d == DURATION_PERMANENT:
        return False
    if step_kind == "cleanup":
        if d == DURATION_END_OF_TURN:
            return True
        if d == DURATION_UNTIL_END_OF_YOUR_NEXT_TURN:
            return (game.active == mod.created_seat and
                    game.turn > mod.created_turn)
    if step_kind == "end":
        if d == DURATION_UNTIL_NEXT_END_STEP:
            return True
        if d == DURATION_UNTIL_YOUR_NEXT_END_STEP:
            return (game.active == mod.created_seat and
                    game.turn > mod.created_turn)
    if step_kind == "upkeep":
        if d == DURATION_UNTIL_NEXT_UPKEEP:
            return True
    if step_kind == "untap":
        if d == DURATION_UNTIL_YOUR_NEXT_TURN:
            return (game.active == mod.created_seat and
                    game.turn > mod.created_turn)
    if d == DURATION_UNTIL_SOURCE_LEAVES:
        src = mod.source_perm
        if src is None:
            return True
        owner = game.seats[src.controller]
        return src not in owner.battlefield
    if d == DURATION_UNTIL_CONDITION_CHANGES:
        if mod.condition_fn is None:
            return False
        try:
            return not mod.condition_fn(game)
        except Exception:
            return True
    return False


def _pool_exempt_colors(game: "Game", seat: "Seat") -> set:
    """CR §106.4 / §613 layer 6. Return the set of color codes whose
    mana DOES NOT empty from `seat`'s pool at phase/step boundaries.
    Values: subset of {"W","U","B","R","G","C"}, or {"any"} for
    "all colors retained" (Upwelling).

    Implementation scans every permanent on every seat's battlefield
    for a named continuous effect that grants pool-retention — this is
    conservative and matches CR §613.1f (static abilities generate a
    continuous effect while their source is on the battlefield). Two
    recognized sources today:

      - Upwelling — "Mana in each player's mana pool doesn't empty as
        steps and phases end." All colors, ALL seats. Returns {"any"}.
      - Omnath, Locus of Mana — "Green mana doesn't empty from your
        mana pool as steps and phases end." Only green, only Omnath's
        controller.

    Additional cards (Lotus Cobra / Chromatic Orrery / Power Surge
    style items that also retain mana) can be added here without
    schema changes — the set semantics compose cleanly.
    """
    exempt: set = set()
    for s in game.seats:
        for perm in s.battlefield:
            name = perm.card.name
            if name == "Upwelling":
                # Universal exemption for every seat.
                return {"any"}
            if name == "Omnath, Locus of Mana":
                # Only Omnath's CONTROLLER retains green.
                if perm.controller == seat.idx:
                    exempt.add("G")
    return exempt


def drain_all_pools(game: "Game", prev_phase_kind: str,
                    prev_step_kind: str) -> None:
    """CR §106.4 — "When a phase or step ends, each player's mana pool
    empties." Called from set_phase_step on every transition.

    Honors per-seat exemption sets from _pool_exempt_colors. Emits one
    `pool_drain` event per seat whose pool actually changed."""
    for seat in game.seats:
        before = seat.mana.total()
        if before == 0:
            continue
        exempt = _pool_exempt_colors(game, seat)
        if "any" in exempt:
            # Full exemption (Upwelling). Skip drain entirely.
            continue
        seat.mana.clear_except(exempt)
        after = seat.mana.total()
        if after != before:
            game.ev("pool_drain", seat=seat.idx,
                    amount=(before - after),
                    remaining=after,
                    exempt_colors=sorted(exempt),
                    from_phase=prev_phase_kind,
                    from_step=prev_step_kind,
                    rule="106.4")


def scan_expired_durations(game: "Game", phase_kind: str,
                           step_kind: str) -> int:
    """Walk every active Modifier + ContinuousEffect and expire those
    whose duration elapses NOW. CR §514.2 is the canonical cleanup
    boundary; non-cleanup boundaries handle the other duration kinds.

    Also clears buffs_pt on every creature at cleanup (belt-and-
    suspenders) and resets damage_marked (§514.2).
    """
    expired_count = 0
    remaining: list[Modifier] = []
    for mod in game._active_modifiers:
        if mod.expired:
            continue
        if _duration_expires_now(mod, phase_kind, step_kind, game):
            _revert_modifier(game, mod)
            expired_count += 1
            game.ev("modifier_expired", kind=mod.kind,
                    duration=mod.duration,
                    target=getattr(getattr(mod.target, "card", None),
                                   "name", "?"),
                    rule="514.2" if step_kind == "cleanup" else "613")
        else:
            remaining.append(mod)
    game._active_modifiers = remaining
    ce_kept: list[ContinuousEffect] = []
    for ce in game.continuous_effects:
        if ce.duration == DURATION_PERMANENT:
            ce_kept.append(ce)
            continue
        pseudo = Modifier(
            target=ce.source_perm,
            duration=ce.duration,
            kind="continuous_effect",
            created_turn=0,
            created_seat=ce.controller_seat or 0,
            source_perm=ce.source_perm,
        )
        if _duration_expires_now(pseudo, phase_kind, step_kind, game):
            expired_count += 1
            game.ev("continuous_effect_expired",
                    layer=ce.layer, duration=ce.duration,
                    source_card=ce.source_card_name,
                    rule="613")
        else:
            ce_kept.append(ce)
    if len(ce_kept) != len(game.continuous_effects):
        game.continuous_effects = ce_kept
        game.invalidate_characteristics_cache()
    if step_kind == "cleanup":
        for seat in game.seats:
            for p in seat.battlefield:
                p.buffs_pt = (0, 0)
                if p.damage_marked:
                    game.ev("damage_wears_off",
                            card=p.card.name,
                            amount=p.damage_marked,
                            seat=p.controller, rule="514.2")
                    p.damage_marked = 0
        game.invalidate_characteristics_cache()
    return expired_count


def _fire_delayed_triggers(game: "Game", phase_kind: str,
                           step_kind: str) -> int:
    """Walk game.delayed_triggers and fire any whose trigger_at matches
    the current (phase_kind, step_kind) boundary. Fires in timestamp
    order (§603.7)."""
    if not game.delayed_triggers:
        return 0
    fired_count = 0
    to_fire: list[DelayedTrigger] = []
    for dt in game.delayed_triggers:
        if dt.consumed:
            continue
        trig = dt.trigger_at
        matches = False
        if trig == "end_of_turn":
            matches = (step_kind == "end")
        elif trig == "next_end_step":
            matches = (step_kind == "end")
        elif trig == "your_next_end_step":
            matches = (step_kind == "end" and
                       game.active == dt.controller_seat and
                       game.turn > dt.created_turn)
        elif trig == "next_upkeep":
            matches = (step_kind == "upkeep" and
                       (game.turn > dt.created_turn or
                        game.active != dt.controller_seat))
        elif trig == "your_next_upkeep":
            matches = (step_kind == "upkeep" and
                       game.active == dt.controller_seat and
                       game.turn > dt.created_turn)
        elif trig == "end_of_combat":
            matches = (phase_kind == "combat" and
                       step_kind == "end_of_combat")
        elif trig == "your_next_turn":
            matches = (step_kind == "untap" and
                       game.active == dt.controller_seat and
                       game.turn > dt.created_turn)
        if matches:
            if dt.condition is not None:
                try:
                    if not dt.condition(game):
                        continue
                except Exception:
                    continue
            to_fire.append(dt)
    to_fire.sort(key=lambda d: d.source_timestamp)
    for dt in to_fire:
        dt.consumed = True
        game.ev("delayed_trigger_fires",
                trigger_at=dt.trigger_at,
                source_card=dt.source_card_name,
                controller_seat=dt.controller_seat,
                rule="603.7")
        try:
            dt.effect_fn(game)
        except Exception as exc:
            game.ev("delayed_trigger_crashed",
                    source_card=dt.source_card_name,
                    exception=f"{type(exc).__name__}: {exc}")
        fired_count += 1
    game.delayed_triggers = [d for d in game.delayed_triggers
                             if not d.consumed]
    return fired_count


def register_delayed_trigger(game: "Game",
                             dt: "DelayedTrigger") -> "DelayedTrigger":
    """Register a delayed triggered ability."""
    if dt.source_timestamp == 0:
        dt.source_timestamp = game.next_timestamp()
    if dt.created_turn == 0:
        dt.created_turn = game.turn
    game.delayed_triggers.append(dt)
    game.ev("delayed_trigger_registered",
            trigger_at=dt.trigger_at,
            source_card=dt.source_card_name,
            controller_seat=dt.controller_seat,
            rule="603.7")
    return dt


# ----------------------------------------------------------------------------
# §613 Continuous-effect registration + layered characteristics query
# ----------------------------------------------------------------------------


def register_continuous_effect(game: "Game",
                               ce: "ContinuousEffect") -> "ContinuousEffect":
    """Add a ContinuousEffect to the §613 registry. Idempotent on
    handler_id if set."""
    if ce.timestamp == 0:
        ce.timestamp = game.next_timestamp()
    if ce.handler_id:
        for existing in game.continuous_effects:
            if existing.handler_id == ce.handler_id:
                return existing
    game.continuous_effects.append(ce)
    game.invalidate_characteristics_cache()
    return ce


def unregister_continuous_effects_for_permanent(game: "Game",
                                                perm: "Permanent") -> int:
    """Drop every continuous effect whose source_perm is ``perm``."""
    before = len(game.continuous_effects)
    game.continuous_effects = [ce for ce in game.continuous_effects
                               if ce.source_perm is not perm]
    removed = before - len(game.continuous_effects)
    if removed:
        game.invalidate_characteristics_cache()
    return removed


# Layer application order (§613.1 + §613.4).
_LAYER_ORDER = ("1", "2", "3", "4", "5", "6", "7a", "7b", "7c", "7d", "7e")


def _baseline_characteristics(perm: "Permanent") -> dict:
    """Start-of-layer-1 characteristics drawn from the printed card
    (CR 613.1)."""
    card = perm.card
    tl = (card.type_line or "").lower()
    parts = tl.split("—")
    head = parts[0].strip()
    tail = parts[1].strip() if len(parts) > 1 else ""
    head_tokens = [t.strip() for t in head.split() if t.strip()]
    supertype_vocab = {"legendary", "basic", "snow", "tribal", "world",
                       "ongoing", "host"}
    core_types = {"creature", "artifact", "enchantment", "instant",
                  "sorcery", "land", "planeswalker", "battle", "tribal"}
    supertypes = [t for t in head_tokens if t in supertype_vocab]
    types = [t for t in head_tokens if t in core_types]
    subtypes = [t.strip().lower() for t in tail.split() if t.strip()]
    abilities: list[str] = []
    for ab in card.ast.abilities:
        if isinstance(ab, Keyword) and ab.name:
            abilities.append(ab.name)
    for g in perm.granted:
        if g not in abilities:
            abilities.append(g)
    return {
        "name": card.name,
        "type_line": card.type_line,
        "types": list(types),
        "supertypes": list(supertypes),
        "subtypes": list(subtypes),
        "colors": list(card.colors or ()),
        "power": card.power,
        "toughness": card.toughness,
        "abilities": abilities,
        "controller": perm.controller,
    }


def get_effective_characteristics(game: "Game",
                                  perm: "Permanent") -> dict:
    """§613 layer-resolution. Compute the current effective
    characteristics of ``perm`` by applying continuous effects in layer
    order (1→7e). Cached via ``game._char_cache``.

    CR 613.1: start with printed values.
    CR 613.7: within-layer timestamp order.
    CR 613.3: within layers 2-6, CDA first.
    """
    cache_key = id(perm)
    cached = game._char_cache.get(cache_key)
    if cached is not None:
        return cached
    chars = _baseline_characteristics(perm)
    for layer in _LAYER_ORDER:
        candidates = [ce for ce in game.continuous_effects
                      if ce.layer == layer]
        candidates.sort(key=lambda c: c.timestamp)
        for ce in candidates:
            try:
                if not ce.predicate(game, perm):
                    continue
            except Exception:
                continue
            try:
                ce.apply_fn(game, perm, chars)
            except Exception as exc:
                game.ev("continuous_effect_crashed",
                        source_card=ce.source_card_name,
                        layer=layer,
                        exception=f"{type(exc).__name__}: {exc}")
    # Merge counter-based P/T + buffs_pt AFTER layered effects
    # (CR 613.4c: 7c counter after 7b set).
    if chars.get("power") is not None:
        bonus = (perm.counters.get("+1/+1", 0)
                 - perm.counters.get("-1/-1", 0))
        chars["power"] = (chars["power"] or 0) + bonus + perm.buffs_pt[0]
    if chars.get("toughness") is not None:
        bonus = (perm.counters.get("+1/+1", 0)
                 - perm.counters.get("-1/-1", 0))
        chars["toughness"] = (chars["toughness"] or 0) + bonus + perm.buffs_pt[1]
    game._char_cache[cache_key] = chars
    return chars


def layer_system_smoke_test(game: "Game") -> dict:
    """Diagnostic: walk every on-battlefield permanent, query
    ``get_effective_characteristics``, return a summary."""
    out = {}
    for seat in game.seats:
        for perm in seat.battlefield:
            c = get_effective_characteristics(game, perm)
            out[perm.card.name] = {
                "power": c.get("power"),
                "toughness": c.get("toughness"),
                "types": list(c.get("types", [])),
                "subtypes": list(c.get("subtypes", [])),
                "abilities": list(c.get("abilities", [])),
            }
    return out


# ----------------------------------------------------------------------------
# Centralized "would untap" routing — required for §614 stun-counter
# integration. Every untap path (untap step, Seize the Day, activated-
# ability effect, etc.) must go through this function so the stun-
# counter replacement can intercept.
# CR 122.1d: "If a permanent with a stun counter on it would become
# untapped, instead remove a stun counter from it."
# ----------------------------------------------------------------------------


def attempt_untap(game: "Game", perm: "Permanent",
                  reason: str = "") -> bool:
    """Try to untap a permanent, routing through §614."""
    if not perm.tapped:
        return False
    event = Event(
        type="would_untap",
        player=perm.controller,
        target=perm,
        kwargs={"reason": reason},
    )
    fire_event(game, event)
    if event.cancelled:
        return False
    if event.get("replaced_by") == "stun_counter":
        return False
    perm.tapped = False
    game.ev("untap_done", card=perm.card.name, seat=perm.controller,
            reason=reason, rule="500.2")
    return True


def _make_stun_counter_replacement(game: "Game", perm: "Permanent"):
    """Per-permanent §614 replacement for CR 122.1d."""
    ts = game.next_timestamp()
    hid = f"stun_counter:{id(perm)}:{ts}"

    def applies(game, event):
        if event.type != "would_untap":
            return False
        if event.target is not perm:
            return False
        return perm.counters.get("stun", 0) > 0

    def apply_fn(game, event):
        remaining = max(0, perm.counters.get("stun", 0) - 1)
        if remaining == 0:
            perm.counters.pop("stun", None)
        else:
            perm.counters["stun"] = remaining
        event.set("replaced_by", "stun_counter")
        game.ev("stun_counter_consumed",
                card=perm.card.name,
                seat=perm.controller,
                remaining=remaining,
                rule="122.1d")

    return ReplacementEffect(
        event_type="would_untap",
        handler_id=hid,
        source_perm=perm,
        source_card_name=f"stun_counter:{perm.card.name}",
        controller_seat=perm.controller,
        timestamp=ts,
        category="other",
        applies=applies,
        apply_fn=apply_fn,
    )


def ensure_stun_replacement(game: "Game", perm: "Permanent") -> None:
    """Idempotently register a stun-counter untap replacement for
    ``perm``. Call this whenever you add a stun counter to a permanent."""
    if perm.counters.get("stun", 0) <= 0:
        return
    tag = f"stun_counter:{id(perm)}:"
    for re_ in game.replacement_effects:
        if re_.handler_id.startswith(tag):
            return
    register_replacement_effect(
        game, _make_stun_counter_replacement(game, perm))


def queue_extra_combat(game: "Game", count: int = 1) -> None:
    """Schedule ``count`` additional combat phases after the current one.
    Used by Seize the Day / Aggravated Assault / Relentless Assault.
    CR §500.5 — effects that add combat phases insert them after the
    current main phase.

    Note: for an authentic Aggravated Assault experience the caller
    would also need to untap all creatures you control between the
    two combat phases (Assault's additional text). Seize the Day does
    that too. For now, this helper just adds the extra combat_phase
    slot; callers that want to untap should call untap on each
    creature via attempt_untap() themselves."""
    game.pending_extra_combats += count
    game.ev("extra_combat_queued", count=count,
            pending=game.pending_extra_combats, rule="500.5")


def add_stun_counter(game: "Game", perm: "Permanent", count: int = 1) -> None:
    """Add ``count`` stun counters to ``perm`` and ensure the §614
    replacement is registered. CR 122.1d.

    Convenience wrapper used by per_card handlers and the test harness."""
    if count <= 0:
        return
    perm.counters["stun"] = perm.counters.get("stun", 0) + count
    ensure_stun_replacement(game, perm)
    game.ev("stun_counter_added", card=perm.card.name,
            count=count, total=perm.counters["stun"],
            seat=perm.controller, rule="122.1d")


def create_token_tapped_and_attacking(game: "Game", controller_seat: int,
                                      *, types: str,
                                      pt: tuple[int, int],
                                      count: int = 1,
                                      keywords: tuple = ()) -> list:
    """Create N tokens "tapped and attacking" (Hero of Bladehold,
    Etali, etc.). CR §506.3 — the tokens ARE attacking creatures but
    were never declared, so their own 'attacks' triggers don't fire.
    `attacking=True, declared_attacker_this_combat=False`.

    Does NOT route through §614 would_create_token — callers wanting
    Doubling Season interaction should emit would_create_token first
    and then call this with the doubled count.

    Returns the list of created Permanent instances."""
    token_card = CardEntry(
        name=f"{types} Token ({pt[0]}/{pt[1]})",
        mana_cost="", cmc=0,
        type_line=f"Token Creature — {types}",
        oracle_text="",
        power=pt[0], toughness=pt[1],
        ast=CardAST(name=f"{types} token",
                    abilities=tuple(Keyword(name=kw) for kw in keywords),
                    parse_errors=(), fully_parsed=True),
    )
    created = []
    for _ in range(count):
        perm = Permanent(card=token_card, controller=controller_seat,
                         tapped=True,
                         summoning_sick=False,  # tokens in combat can attack
                         attacking=True,
                         declared_attacker_this_combat=False)
        _etb_initialize(game, perm)
        game.seats[controller_seat].battlefield.append(perm)
        created.append(perm)
        game.ev("token_created_attacking",
                card=token_card.name,
                controller_seat=controller_seat,
                pt=pt,
                rule="506.3")
    return created


# Name -> list of ReplacementEffect factory functions.
# Factories take (game, perm) and return list[ReplacementEffect].
# Keyed by exact card name (case-sensitive as Scryfall dumps them).
_REPLACEMENT_REGISTRY_BY_NAME: dict = {
    "Laboratory Maniac": [_make_labman_win],
    "Jace, Wielder of Mysteries": [_make_labman_win],
    "Alhammarret's Archive": [_make_alhammarret_draw_doubler],
    "Boon Reflection": [_make_rhox_life_doubler],
    "Rhox Faithmender": [_make_rhox_life_doubler],
    "Rest in Peace": [_make_rest_in_peace],
    "Leyline of the Void": [_make_leyline_of_the_void],
    "Anafenza, the Foremost": [_make_anafenza],
    "Doubling Season": [_make_doubling_season],
    "Hardened Scales": [_make_hardened_scales],
    "Panharmonicon": [_make_panharmonicon],
    "Platinum Angel": [_make_platinum_angel],
}


# ============================================================================
# §903 Commander-variant infrastructure
# ============================================================================
#
# Scope of this block:
#   - Game setup for Commander (§903.6 command zone, §903.4 40 life)
#   - §903.9b replacement (commander would be put into hand or library →
#     owner may put it into the command zone instead). Registered as a
#     §614 ReplacementEffect for each commander at game start.
#   - §903.9a + §704.6d SBA (commander in graveyard or exile → owner may
#     put it into the command zone). Runs in state_based_actions.
#   - §903.8 commander tax + cast-from-command-zone path, integrated
#     with cast_spell and cost helpers.
#   - §903.10a + §704.6c commander-damage tracking + 21-damage-loss SBA,
#     derived from the existing damage event stream so no combat code
#     gets modified (the peer agent owns combat).
#
# Citations below refer to `data/rules/MagicCompRules-20260227.txt`.
# Exact section numbers verified against that file.
#
# Partner / partner-with / companion / meld / merged-permanent edge
# cases are ARCHITECTED FOR (commander_names is a list, commander_tax
# and commander_damage are keyed by name) but NOT IMPLEMENTED. The 4-
# deck gauntlet uses single-commander decks only; partner/companion
# remain as OUT-OF-SCOPE TODOs.
# ============================================================================


def _greedy_allow_return_to_command_zone(game: "Game", event: "Event",
                                         from_zone_hint: str) -> bool:
    """Owner-choice policy for §903.9 returns. The comp rules say "its
    owner MAY put it into the command zone" — it's optional each time.
    Policy (greedy): always divert. Keeps the commander accessible,
    matches what nearly every competent Commander pilot does. A future
    smart policy could refuse when graveyard positioning matters (e.g.
    Meren of Clan Nel Toth owner wants the commander in the graveyard
    for reanimation value) — flagged as a TODO, not implemented.
    """
    # Placeholder signature is future-proof for a policy overhaul.
    return True


def _make_commander_zone_change(game: "Game",
                                owner_seat: int,
                                commander_name: str) -> "ReplacementEffect":
    """Register a §903.9b replacement: commander would be put into its
    owner's HAND or LIBRARY from anywhere → owner may put it into the
    command zone instead.

    NOT registered for graveyard or exile destinations — those are
    handled by the §903.9a / §704.6d state-based action (see
    _sba_704_6d below). The comp rules split these two paths:
      §903.9a — graveyard/exile: STATE-BASED ACTION, owner chooses.
      §903.9b — hand/library:     REPLACEMENT EFFECT, owner chooses.
                                  Explicit exception to §614.5 (the
                                  effect may apply more than once to
                                  the same event). We don't need the
                                  applied-once-override for single-
                                  commander: the event just fires.

    Per §903.9b "if a commander WOULD be put into its owner's hand or
    library from anywhere, its owner may put it into the command zone
    instead." The key word is OWNER — a Gilded-Drake-stolen commander
    still uses the original owner's replacement when it would change
    zones. Our event kwargs carry owner_seat so this works correctly.
    """
    ts = _next_handler_timestamp(game)
    hid = f"commander_903_9b:{owner_seat}:{commander_name}:{ts}"

    def applies(game, event):
        if event.type != "would_change_zone":
            return False
        # Only hand / library destinations go through the §614
        # replacement path; graveyard/exile go through the §704.6d SBA.
        dest = event.get("to_zone")
        if dest not in ("hand", "library"):
            return False
        # Owner-keyed (not controller-keyed): the replacement belongs
        # to the original owner, per §903.9b.
        owner = event.get("owner_seat")
        if owner != owner_seat:
            return False
        name = event.get("card_name")
        if name != commander_name:
            return False
        return _greedy_allow_return_to_command_zone(
            game, event, from_zone_hint=event.get("from_zone", ""))

    def apply_fn(game, event):
        event.set("to_zone", "command_zone")
        event.set("redirected_by", "CR 903.9b")
        game.ev("commander_zone_return",
                card=commander_name, owner_seat=owner_seat,
                from_zone=event.get("from_zone"),
                origin_destination=event.get("origin_to_zone",
                                             event.get("to_zone")),
                rule="903.9b")

    return ReplacementEffect(
        event_type="would_change_zone",
        handler_id=hid,
        source_perm=None,  # a rule, not a permanent on the battlefield
        source_card_name=commander_name,
        controller_seat=owner_seat,
        timestamp=ts,
        category="other",
        applies=applies,
        apply_fn=apply_fn,
    )


def fire_zone_change(game: "Game",
                     card: "CardEntry",
                     owner_seat: int,
                     from_zone: str,
                     to_zone: str,
                     source: Optional["Permanent"] = None) -> "Event":
    """Route a zone change through the §614 replacement chain.

    This is the single entry point for "this card would change zones"
    events. It only fires §903.9b-style replacements for hand/library
    destinations in Commander games; graveyard and exile destinations
    are handled by the §704.6d state-based action AFTER the zone
    change has taken place, so we don't fire for those destinations
    (§903.9a is explicitly an SBA, not a replacement — see rules
    file §704.6d and §903.9a).

    battlefield → battlefield transitions (e.g. Gilded Drake ownership
    shuffle, flicker's exile-and-return counts as two separate zone
    changes, not a battlefield→battlefield change) are not routed
    through here; callers should only invoke fire_zone_change for real
    cross-zone transitions.

    Returns the (possibly modified) Event. Callers inspect
    event.get("to_zone") to decide the residual destination.
    """
    event = Event(
        type="would_change_zone",
        player=owner_seat,
        source=source,
        kwargs={
            "card_name": card.name,
            "owner_seat": owner_seat,
            "from_zone": from_zone,
            "to_zone": to_zone,
            "origin_to_zone": to_zone,  # captured for diagnostics
        },
    )
    fire_event(game, event)
    return event


# ============================================================================
# DFC (double-faced card) commander name resolution (CR §712)
# ============================================================================
#
# The deck file may declare a DFC commander by its FRONT-FACE name
# ("Ral, Monsoon Mage") while the oracle card entry carries the full
# double-slash name ("Ral, Monsoon Mage // Ral, Leyline Prodigy"). These
# helpers normalize that mismatch so §903 setup, the cast-from-command-
# zone path, and the §903.9b replacement all see the same key.
#
# Without this, Ral / Ulrich / Tergrid / Ashling DFC decks silently can
# never cast their commander — the commander is never put in the command
# zone, commander_names is seeded with a name that never matches anything
# on the stack, and setup_commander_game's `c.name == name` test fails.


def _dfc_front_face_name(card_name: str) -> str:
    """Return the front-face half of a DFC oracle name, or the name as-is
    for non-DFC cards. ``"Ral, Monsoon Mage // Ral, Leyline Prodigy"`` →
    ``"Ral, Monsoon Mage"``. Also handles the single-slash variant some
    decklists use (``"Ashling, Rekindled / Ashling, Rimebound"``).

    CR §712.1 — each face of a double-faced card has its own name; the
    "card's name" on a DFC oracle entry is conventionally the front-face
    name. This helper canonicalizes whichever separator style the input
    uses so name matching is transparent.
    """
    if not card_name:
        return card_name
    if " // " in card_name:
        return card_name.split(" // ", 1)[0].strip()
    if " / " in card_name:
        pre, post = card_name.split(" / ", 1)
        if pre.strip() and post.strip():
            return pre.strip()
    return card_name


def _dfc_card_matches_name(card: "CardEntry", declared_name: str) -> bool:
    """True iff ``card`` is the oracle entry for the commander named
    ``declared_name``. Matches exact name, DFC front-face equality, and
    DFC back-face equality. Case-insensitive. Handles both ``//``
    (canonical) and ``/`` (legacy) separators.
    """
    if card is None:
        return False
    cn = card.name or ""
    dn = (declared_name or "").strip()
    if not cn or not dn:
        return False
    if cn.lower() == dn.lower():
        return True
    faces: list[str] = []
    if " // " in cn:
        faces = [f.strip() for f in cn.split(" // ")]
    elif " / " in cn:
        faces = [f.strip() for f in cn.split(" / ")]
    else:
        return False
    return any(f.lower() == dn.lower() for f in faces)


def _resolve_commander_card(deck: "Deck", seat: "Seat",
                            declared_name: str,
                            strip_from_library: bool = False
                            ) -> Optional["CardEntry"]:
    """Find the CardEntry that represents the commander whose name the
    deck file declared as ``declared_name``. Searches in order:

      1. deck.commander_cards (preferred — gauntlet-produced decks
         already include the full oracle entry here).
      2. seat.command_zone (idempotent re-setup path).
      3. seat.library (fallback — if the decklist included the commander
         line as a main-list entry and it survived into the library).

    Returns the CardEntry on match, None on miss. Matching is DFC-aware:
    ``"Ral, Monsoon Mage"`` matches the oracle entry
    ``"Ral, Monsoon Mage // Ral, Leyline Prodigy"``.

    CR §712 / §903.3 — a commander designation is an attribute of the
    card, not the face, so name matching must fold DFC halves together.
    """
    for c in getattr(deck, "commander_cards", []) or []:
        if _dfc_card_matches_name(c, declared_name):
            return c
    for c in seat.command_zone:
        if _dfc_card_matches_name(c, declared_name):
            return c
    for c in list(seat.library):
        if _dfc_card_matches_name(c, declared_name):
            if strip_from_library:
                seat.library.remove(c)
            return c
    return None


def setup_commander_game(game: "Game",
                         *decks: "Deck") -> None:
    """Perform §903.6 + §903.7 setup for a Commander game.

    N-seat aware. Accepts positional ``Deck`` args (one per seat). Back-
    compat: ``setup_commander_game(game, deck_a, deck_b)`` still works
    as before for 2-player. For N>=3 pass ``setup_commander_game(game,
    deck_0, deck_1, deck_2, deck_3)`` and so on. The number of decks
    must match ``len(game.seats)`` — this is a programmer-error assert.

    §903.6 — "At the start of the game, each player puts their
             commander from their deck face up into the command zone.
             Then each player shuffles the remaining cards of their
             deck so that the cards are in a random order."
    §903.7 — "Once the starting player has been determined, each
             player sets their life total to 40 and draws a hand of
             seven cards."

    The caller constructs game with seat.library already built from
    the 99-card remainder (commander NOT in the library). This helper:
      1. Sets seat.starting_life = 40 and seat.life = 40 (§903.4 /
         §903.7).
      2. Populates seat.commander_names from deck.commander_name /
         deck.commander_names.
      3. Puts the commander CardEntry into seat.command_zone.
      4. Registers the §903.9b zone-change replacement per commander,
         keyed to the OWNER (not controller).
      5. Flips game.commander_format = True so SBAs and cast helpers
         activate Commander-specific rules.

    IMPORTANT: build seat.library with commander already REMOVED.
    setup_commander_game doesn't touch the library; putting the
    commander into the zone is done via deck.commander_name matching
    the CardEntry already held by the Deck object.
    """
    if len(decks) != len(game.seats):
        raise ValueError(
            f"setup_commander_game: got {len(decks)} decks for "
            f"{len(game.seats)} seats (mismatch — should be one per seat)")
    game.commander_format = True
    for seat, deck in zip(game.seats, decks):
        # §903.4 + §903.7: starting life total.
        seat.starting_life = 40
        seat.life = 40
        # Resolve commander names. Single-commander path.
        names = []
        if deck.commander_names:
            names = list(deck.commander_names)
        elif deck.commander_name:
            names = [deck.commander_name]

        # DFC commander name resolution (CR §712). The deck file may name
        # a DFC commander by its FRONT-FACE name ("Ral, Monsoon Mage",
        # "Ulrich of the Krallenhorde", "Tergrid, God of Fright") while
        # the oracle card entry carries the full double-slash name
        # ("Ral, Monsoon Mage // Ral, Leyline Prodigy"). Canonicalize each
        # declared name to the oracle card's full name so seat.command_zone
        # lookups, §903.9b replacement keys, _is_commander_card, and the
        # cast-from-command-zone path all compare apples-to-apples. Without
        # this fix, DFC-commander decks silently can never cast their
        # commander.
        canonicalized: list[str] = []
        for name in names:
            resolved = _resolve_commander_card(deck, seat, name)
            canonicalized.append(resolved.name if resolved is not None
                                 else name)
        names = canonicalized
        seat.commander_names = names
        # Initialise per-name tax counters so cast_spell sees stable keys.
        # commander_damage is nested (dealer_seat → name → int); it stays
        # empty-by-default and gets populated lazily by the damage
        # accumulator.
        for name in names:
            seat.commander_tax.setdefault(name, 0)
        # Put each commander card into the command zone. We accept a
        # `commander_cards` attribute if the Deck-builder provided
        # CardEntry objects; otherwise the caller is expected to have
        # pre-populated seat.command_zone. Post-canonicalization above,
        # `name` is the oracle card's full (double-slash) name so the
        # equality comparison hits cleanly for DFC commanders too.
        for name in names:
            if any(c.name == name for c in seat.command_zone):
                continue
            card_entry = _resolve_commander_card(deck, seat, name,
                                                 strip_from_library=True)
            if card_entry is not None:
                seat.command_zone.append(card_entry)
        # Register §903.9b hand/library redirect for every commander.
        for name in seat.commander_names:
            eff = _make_commander_zone_change(game, seat.idx, name)
            register_replacement_effect(game, eff)
    game.ev("commander_setup",
            seats=[{"seat": s.idx,
                    "commander_names": list(s.commander_names),
                    "starting_life": s.starting_life,
                    "command_zone_size": len(s.command_zone)}
                   for s in game.seats],
            rule="903.6+903.7")


def _is_commander_card(game: "Game", card: "CardEntry",
                       seat: "Seat") -> bool:
    """Is `card` a commander belonging to `seat` (by name)?"""
    return game.commander_format and card.name in seat.commander_names


# ---------------------------------------------------------------------------
# Partner-commander legality — CR §702.124 / §903.3c
# ---------------------------------------------------------------------------

def _card_has_partner_keyword(card: "CardEntry") -> bool:
    """True iff the card has the bare Partner keyword in its oracle text.

    Keyword detection is case-insensitive + fuzzy on reminder text. We
    check two surfaces:

      1. Oracle-text membership: cards print "Partner" as a standalone
         keyword followed by parenthesised reminder ("(You can have two
         commanders ...)"). We match a line that starts with "Partner"
         and ISN'T "Partner with X" (the latter is specific-partner; see
         partner_with_name()).
      2. AST keyword nodes: parser.py emits partner as a Keyword AST
         node; we scan CardAST.abilities for any Keyword whose raw text
         is exactly "partner" or starts with "partner (".

    The AST check is authoritative; the text fallback catches cards
    whose AST didn't parse (rare; parser_coverage.md has the list).
    """
    if card is None:
        return False
    # AST-level check.
    ast = getattr(card, "ast", None)
    if ast is not None:
        for ab in getattr(ast, "abilities", ()) or ():
            # Keyword nodes carry a .name attr + optional raw.
            name = getattr(ab, "name", "") or ""
            raw = getattr(ab, "raw", "") or ""
            rlo = raw.strip().lower()
            if rlo == "partner":
                return True
            if rlo.startswith("partner (") or rlo.startswith("partner\n"):
                return True
            if name.lower() == "partner" and not rlo.startswith("partner with"):
                return True
    # Oracle-text fallback for cards whose AST missed the keyword line.
    text = (getattr(card, "oracle_text", "") or "").lower()
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.startswith("partner (") or stripped == "partner":
            return True
    return False


def _partner_with_name(card: "CardEntry") -> Optional[str]:
    """Return the specific-partner target name for "Partner with X",
    or None if the card doesn't carry that keyword. CR §702.124g."""
    if card is None:
        return None
    # AST-level.
    ast = getattr(card, "ast", None)
    if ast is not None:
        for ab in getattr(ast, "abilities", ()) or ():
            raw = (getattr(ab, "raw", "") or "").strip()
            rlo = raw.lower()
            if rlo.startswith("partner with "):
                name = raw[len("partner with "):].strip()
                # Strip reminder text tail "Partner with X (When this ...)"
                for sep in (" (", ". ", "\n"):
                    idx = name.find(sep)
                    if idx >= 0:
                        name = name[:idx].strip()
                return name.rstrip(".,;:")
    # Oracle-text fallback.
    text = getattr(card, "oracle_text", "") or ""
    for line in text.splitlines():
        stripped = line.strip()
        low = stripped.lower()
        if low.startswith("partner with "):
            name = stripped[len("partner with "):].strip()
            for sep in (" (", ". "):
                idx = name.find(sep)
                if idx >= 0:
                    name = name[:idx].strip()
            return name.rstrip(".,;:")
    return None


def _card_keyword_present(card: "CardEntry", keyword: str) -> bool:
    """True iff the card's oracle text or AST carries the named bare
    keyword. Lowercase-insensitive. Used for Friends Forever / Choose
    a Background / Doctor's Companion."""
    if card is None:
        return False
    kwl = keyword.strip().lower()
    ast = getattr(card, "ast", None)
    if ast is not None:
        for ab in getattr(ast, "abilities", ()) or ():
            raw = (getattr(ab, "raw", "") or "").strip().lower()
            name = (getattr(ab, "name", "") or "").strip().lower()
            if raw == kwl or name == kwl:
                return True
            if raw.startswith(kwl + " ") or raw.startswith(kwl + "("):
                return True
    text = (getattr(card, "oracle_text", "") or "").lower()
    for line in text.splitlines():
        stripped = line.strip()
        if stripped == kwl or stripped.startswith(kwl + " (") or stripped.startswith(kwl + "("):
            return True
    return False


def _card_has_subtype(card: "CardEntry", subtype: str) -> bool:
    """True iff the card's type_line contains the given subtype,
    case-insensitive. Used for Background + Doctor membership."""
    if card is None:
        return False
    t = (getattr(card, "type_line", "") or "").lower()
    return subtype.lower() in t


def validate_partner_pair(
        cards: list["CardEntry"]) -> tuple[bool, str]:
    """Check CR §702.124 / §903.3c partner legality for a commander list.

    Returns ``(legal, reason)``. reason is empty when legal.

    Valid configurations:
      1. Both cards have bare Partner keyword (§702.124a).
      2. "Partner with X" + named X on each side (§702.124g).
      3. Both Friends Forever (§702.124f).
      4. Choose-a-Background commander + Background-typed card (§702.124h).
      5. Doctor + Doctor's Companion (§702.124i).

    Single commander → trivially legal. Empty / 3+ cards → illegal.
    """
    if not cards:
        return False, "no_commander"
    if len(cards) == 1:
        return True, ""
    if len(cards) > 2:
        return False, "too_many_commanders"
    a, b = cards[0], cards[1]
    if a is None or b is None:
        return False, "nil_commander"

    a_partner = _card_has_partner_keyword(a)
    b_partner = _card_has_partner_keyword(b)
    # Case 1: bare Partner on both.
    if a_partner and b_partner:
        return True, ""
    # Case 2: Partner with X.
    a_pw = _partner_with_name(a)
    b_pw = _partner_with_name(b)
    if a_pw and a_pw.lower() == b.name.lower() and \
            (b_pw is None or b_pw.lower() == a.name.lower()):
        return True, ""
    if b_pw and b_pw.lower() == a.name.lower() and \
            (a_pw is None or a_pw.lower() == b.name.lower()):
        return True, ""
    # Case 3: Friends Forever.
    if _card_keyword_present(a, "friends forever") and \
            _card_keyword_present(b, "friends forever"):
        return True, ""
    # Case 4: Choose-a-Background + Background.
    if (_card_keyword_present(a, "choose a background") and
            _card_has_subtype(b, "background")):
        return True, ""
    if (_card_keyword_present(b, "choose a background") and
            _card_has_subtype(a, "background")):
        return True, ""
    # Case 5: Doctor + Doctor's Companion.
    if (_card_has_subtype(a, "doctor") and
            _card_keyword_present(b, "doctor's companion")):
        return True, ""
    if (_card_has_subtype(b, "doctor") and
            _card_keyword_present(a, "doctor's companion")):
        return True, ""
    return False, "invalid_partner_pair"


def commander_cast_cost(seat: "Seat", commander_name: str,
                        base_cmc: int) -> int:
    """§903.8 — "A commander cast from the command zone costs an
    additional {2} for each previous time the player casting it has
    cast it from the command zone that game."
    """
    tax = seat.commander_tax.get(commander_name, 0)
    return base_cmc + 2 * tax


def can_cast_commander_from_command_zone(game: "Game",
                                         seat: "Seat",
                                         card: "CardEntry") -> bool:
    """True iff `seat` can legally cast `card` from the command zone
    this moment: card is in seat.command_zone, card name is one of
    seat.commander_names, and seat has the mana to pay base_cmc +
    2 × current_tax. Ignores color identity for now (MVP).
    """
    if not game.commander_format:
        return False
    if card not in seat.command_zone:
        return False
    if card.name not in seat.commander_names:
        return False
    total_cost = commander_cast_cost(seat, card.name, card.cmc)
    return _available_mana(seat) >= total_cost


def cast_commander_from_command_zone(game: "Game",
                                     card: "CardEntry") -> None:
    """Cast a commander from the command zone, paying §903.8 tax.

    Mirrors cast_spell's contract but:
      1. Pulls the card from seat.command_zone (not hand).
      2. Pays base cost + 2 × current tax via _pay_generic_cost.
      3. Increments seat.commander_tax[commander_name] AFTER the
         payment succeeds (§903.8 semantics — "each previous time"
         means the Nth cast pays 2(N-1), so we increment after
         payment, before resolution).
      4. Pushes onto the stack as an ordinary permanent spell.
      5. On resolution, the commander enters the battlefield in the
         normal way — the zone exit from command_zone has already
         happened when we popped the card, so no §614 zone-change
         event fires for the cast itself. (§903.9b only governs
         hand/library returns, not casts.)

    The caller (policy layer) is responsible for deciding WHEN to
    call this — typically the AI would check on the main phase.
    """
    s = game.seats[game.active]
    if card not in s.command_zone:
        return
    name = card.name
    total_cost = commander_cast_cost(s, name, card.cmc)
    # §903.8 — additional cost, so pay the full total as generic mana.
    paid = _pay_generic_cost(game, s, total_cost, reason="commander_cast",
                             card_name=name)
    if not paid:
        return
    # Remove from command zone. CR 601.2a — cast begins by moving the
    # card from its zone to the stack.
    s.command_zone.remove(card)
    # Increment tax AFTER a successful payment + zone exit. Next cast
    # of the same commander from the command zone will pay base + 2 ×
    # new tax per §903.8.
    s.commander_tax[name] = s.commander_tax.get(name, 0) + 1
    game.emit(f"cast commander {name} from command zone "
              f"(tax so far → {s.commander_tax[name]})")
    game.ev("commander_cast_from_command_zone",
            card=name, seat=s.idx,
            base_cost=card.cmc,
            total_paid=total_cost,
            tax_after=s.commander_tax[name],
            rule="903.8")
    # CR §700.4 / §702.40 cast-count bookkeeping — commander casts from
    # the command zone are still casts per §601 (§903.8 only modifies
    # the cost; the cast itself goes through the normal sequence), so
    # they increment the storm counter and trigger cast-observer effects
    # just like any other cast.
    game.spells_cast_this_turn += 1
    s.spells_cast_this_turn += 1
    # Push onto stack as a permanent spell. The existing stack +
    # resolution path handles ETB / triggers / priority — we route
    # through the same _push_stack_item that cast_spell uses.
    item = _push_stack_item(game, card, game.active)
    # Commanders with storm don't exist in print, but fire the hook for
    # completeness + symmetry with cast_spell.
    if _has_storm_keyword(card):
        _apply_storm_copies(game, item, game.active)
    _fire_cast_trigger_observers(game, card, game.active, from_copy=False)
    _priority_round(game)
    while game.stack and not game.ended:
        _resolve_stack_top(game)
        state_based_actions(game)
    state_based_actions(game)
    game.snapshot()


def _sba_704_6d(game: "Game") -> bool:
    """§704.6d — "In a Commander game, if a commander is in a graveyard
    or in exile and that object was put into that zone since the last
    time state-based actions were checked, its owner may put it into
    the command zone."

    Also §903.9a — same rule phrased from §903's side.

    Owner-keyed: we scan every seat's graveyard + exile for cards
    whose name is in that seat's commander_names. Policy greedy-allows
    the return, so every matching card is lifted out and appended to
    the command zone.

    NOT a replacement — the card has already arrived in the zone. We
    emit a `sba_704_6d` event so the auditor can trace the return.

    Returns True iff anything moved.
    """
    if not game.commander_format:
        return False
    changed = False
    for s in game.seats:
        if not s.commander_names:
            continue
        for zone_name in ("graveyard", "exile"):
            zone = getattr(s, zone_name)
            kept = []
            for c in zone:
                if c.name in s.commander_names and \
                        _greedy_allow_return_to_command_zone(
                            game, None, from_zone_hint=zone_name):
                    s.command_zone.append(c)
                    game.ev("sba_704_6d", seat=s.idx,
                            card=c.name,
                            from_zone=zone_name,
                            rule="704.6d+903.9a")
                    changed = True
                else:
                    kept.append(c)
            if len(kept) != len(zone):
                setattr(s, zone_name, kept)
    return changed


def _sba_704_6c(game: "Game") -> bool:
    """§704.6c — "In a Commander game, a player who's been dealt 21 or
    more combat damage by the same commander over the course of the
    game loses the game."

    Implementation: we mine the existing `damage` event stream
    (emitted by _apply_damage_to_player) for target_kind="player"
    events whose source_card name matches ANY seat's
    commander_names. We attribute that damage to
    seats[target_seat].commander_damage[dealer_seat][source_card_name].

    Checkpoint: _commander_damage_next_seq tracks the next event
    index we haven't scanned yet, so each event is attributed exactly
    once even though the SBA loop runs many times per game.

    Partner-aware: commander_damage is a nested dict keyed by
    (dealer_seat, commander_name). Kraum and Tymna (same dealer seat)
    live in separate sub-dict entries, so 15 Kraum + 10 Tymna = no
    loss (§704.6c "the same commander"). 21 from just Kraum alone = loss.

    Only COMBAT damage counts per §903.10a ("dealt 21 or more
    combat damage"). We filter by the `reason` / phase. Our combat
    resolver emits the damage event during phase == "combat" and
    does NOT annotate reason; non-combat damage (Lightning Bolt,
    ETB pings) comes from resolve_effect and is emitted during main
    phases. We use the event's `phase` field as a proxy for combat
    vs non-combat. This is a best-effort classifier; a future combat
    resolver upgrade should annotate combat damage events with
    reason="combat" for a crisper predicate.

    Returns True iff a seat crossed the 21-threshold this pass.
    """
    if not game.commander_format:
        return False
    commander_names_by_owner: dict = {}
    for s in game.seats:
        for nm in s.commander_names:
            commander_names_by_owner[nm] = s.idx
    if not commander_names_by_owner:
        return False
    changed_loss = False
    start = game._commander_damage_next_seq
    for i in range(start, len(game.events)):
        evt = game.events[i]
        if evt.get("type") != "damage":
            continue
        if evt.get("target_kind") != "player":
            continue
        source_card = evt.get("source_card")
        if source_card not in commander_names_by_owner:
            continue
        # §903.10a is explicit: COMBAT damage only. Use the recorded
        # phase as a heuristic. Events fired by _apply_damage_to_player
        # during a "combat" phase are by definition combat damage.
        phase = evt.get("phase", "")
        if phase != "combat":
            continue
        target_seat = evt.get("target_seat")
        if target_seat is None or target_seat >= len(game.seats):
            continue
        seat_obj = game.seats[target_seat]
        amount = int(evt.get("amount", 0))
        if amount <= 0:
            continue
        dealer_seat = commander_names_by_owner[source_card]
        # commander_damage is nested dict[int, dict[str, int]] — the
        # dealer_seat layer lets partner pairs keep Kraum and Tymna
        # separate even though both share seat 0 as their dealer.
        bucket = seat_obj.commander_damage.setdefault(dealer_seat, {})
        bucket[source_card] = bucket.get(source_card, 0) + amount
        game.ev("commander_damage_accum",
                commander=source_card,
                owner_seat=dealer_seat,
                target_seat=target_seat,
                amount=amount,
                total=bucket[source_card],
                rule="903.10a")
    game._commander_damage_next_seq = len(game.events)
    # Now check the 21-threshold. Each (dealer_seat, commander_name)
    # bucket is checked separately; 15 Kraum + 10 Tymna must NOT
    # cumulatively trip the SBA.
    for s in game.seats:
        if s.lost:
            continue
        found_loss = False
        for dealer, by_name in s.commander_damage.items():
            for nm, dmg in by_name.items():
                if dmg >= 21:
                    s.lost = True
                    s.loss_reason = (f"21+ commander damage from {nm} "
                                     f"(CR 704.6c)")
                    game.ev("sba_704_6c", seat=s.idx,
                            commander=nm, dealer_seat=dealer,
                            total_damage=dmg,
                            rule="704.6c+903.10a")
                    changed_loss = True
                    found_loss = True
                    break
            if found_loss:
                break
    return changed_loss


# End §903 infrastructure block.


# ============================================================================
# Deck building — pick only structural cards we can actually execute
# ============================================================================

# Target card list — real Magic cards whose AST we've confirmed the simulator
# can execute. We pull by name from the oracle dump, parse through the real
# parser, and build a deck from whatever hits.
BURN_DECK_LIST = [
    ("Lightning Bolt", 4),
    ("Shock", 4),
    ("Lava Spike", 4),
    ("Chain Lightning", 4),
    ("Rift Bolt", 2),      # suspend — may parse as keyword; we skip if unplayable
    ("Searing Blaze", 2),
    ("Lightning Strike", 4),
    ("Grizzly Bears", 2),   # vanilla beater
    ("Goblin Guide", 2),    # haste keyword (we honor summoning_sick=False for haste)
    ("Mountain", 24),
]

CONTROL_DECK_LIST = [
    # Card draw + library manipulation
    ("Opt", 3),
    ("Ponder", 2),
    ("Preordain", 2),
    ("Divination", 2),
    # Removal (need enough to survive aggro)
    ("Doom Blade", 4),
    # Counter suite — now that the engine has a real stack + priority,
    # these actually fire.
    ("Counterspell", 4),
    ("Negate", 3),
    ("Mana Leak", 3),
    ("Cancel", 2),
    ("Dissolve", 2),
    # Finishers
    ("Grizzly Bears", 3),
    # Mana
    ("Dark Ritual", 2),
    ("Swamp", 13),
    ("Island", 15),
]

# Creature-heavy aggro (goblin-flavored). Majority-creatures, varied keywords
# so the combat logic exercises flying/haste/first strike/double strike/trample.
CREATURES_DECK_LIST = [
    ("Goblin Guide", 4),        # 2/2 haste
    ("Jackal Pup", 4),          # 2/1
    ("Goblin Cohort", 4),       # 2/2 for 1
    ("Raging Goblin", 4),       # 1/1 haste
    ("Goblin Raider", 4),       # 2/2 for 2
    ("Mogg Flunkies", 4),       # 3/3 for 2
    ("Goblin Piker", 2),        # 2/1 for 2
    ("Viashino Pyromancer", 2), # 2/1 for 2 (ETB dmg if parsed; safe anyway)
    ("Boros Swiftblade", 2),    # 1/2 double strike
    ("Fencing Ace", 2),         # 1/1 double strike
    ("Keldon Raider", 2),       # 4/3 for 4
    ("Hell's Thunder", 2),      # 4/4 flying haste
    ("Grizzly Bears", 2),       # filler 2/2
    ("Mountain", 22),
]

# Mono-green ramp. Total: 20+4+4+4+4+4+4+3+4+2+1+2+4 = 60.
RAMP_DECK_LIST = [
    ("Forest", 20),
    ("Cultivate", 4),
    ("Kodama's Reach", 4),
    ("Rampant Growth", 4),
    ("Farseek", 4),
    ("Sakura-Tribe Elder", 4),
    ("Wood Elves", 4),
    ("Solemn Simulacrum", 3),
    ("Llanowar Elves", 4),
    # Payoffs
    ("Craterhoof Behemoth", 2),
    ("Avenger of Zendikar", 1),
    ("Primeval Titan", 2),
    ("Elvish Mystic", 4),
]


def _parse_pt(s: Optional[str]) -> Optional[int]:
    if not s:
        return None
    s = str(s).replace("*", "0").strip()
    try:
        return int(s)
    except ValueError:
        return None


def load_card_by_name(cards_by_name: dict, name: str) -> Optional[CardEntry]:
    c = cards_by_name.get(name.lower())
    if not c:
        # DFC fallback: lookup might be by front-face name only. Scan all
        # entries whose oracle name contains " // " and check each face.
        low = name.lower()
        for cn, cdict in cards_by_name.items():
            full = cdict.get("name", "")
            if " // " in full:
                faces_lc = [f.strip().lower() for f in full.split(" // ")]
                if low in faces_lc:
                    c = cdict
                    break
        if not c:
            return None
    ast = mtg_parser.parse_card(c)
    colors_raw = c.get("colors") or ()
    # CR §712 DFC detection + face preload. Scryfall "layout" field
    # identifies transform ("transform"), modal DFC ("modal_dfc"), meld
    # ("meld"), reversible ("reversible_card"). For each of these we
    # preload both faces so transform() can swap instantly.
    layout = (c.get("layout") or "normal").lower()
    is_dfc = layout in ("transform", "modal_dfc", "meld",
                        "reversible_card")
    front_face_name = front_face_ast = None
    front_face_mana_cost = front_face_cmc = None
    front_face_type_line = front_face_oracle_text = None
    front_face_power = front_face_toughness = None
    front_face_colors: tuple = ()
    back_face_name = back_face_ast = None
    back_face_mana_cost = back_face_cmc = None
    back_face_type_line = back_face_oracle_text = None
    back_face_power = back_face_toughness = None
    back_face_colors: tuple = ()
    faces = c.get("card_faces") or []
    if is_dfc and len(faces) >= 2:
        ff, bf = faces[0], faces[1]
        # Front face
        front_face_name = ff.get("name") or ""
        front_face_mana_cost = ff.get("mana_cost") or ""
        front_face_cmc = _parse_pt(ff.get("cmc")) if ff.get("cmc") is not None else None
        front_face_type_line = ff.get("type_line") or ""
        front_face_oracle_text = ff.get("oracle_text") or ""
        front_face_power = _parse_pt(ff.get("power"))
        front_face_toughness = _parse_pt(ff.get("toughness"))
        front_face_colors = tuple(ff.get("colors") or ())
        # Parse the FRONT face oracle text on its own so daybound /
        # nightbound and other face-specific keywords are emitted.
        try:
            front_face_ast = mtg_parser.parse_card({
                "name": front_face_name,
                "oracle_text": front_face_oracle_text,
                "type_line": front_face_type_line,
                "mana_cost": front_face_mana_cost,
            })
        except Exception:
            front_face_ast = None
        # Back face
        back_face_name = bf.get("name") or ""
        back_face_mana_cost = bf.get("mana_cost") or ""
        back_face_cmc = _parse_pt(bf.get("cmc")) if bf.get("cmc") is not None else None
        back_face_type_line = bf.get("type_line") or ""
        back_face_oracle_text = bf.get("oracle_text") or ""
        back_face_power = _parse_pt(bf.get("power"))
        back_face_toughness = _parse_pt(bf.get("toughness"))
        back_face_colors = tuple(bf.get("colors") or ())
        try:
            back_face_ast = mtg_parser.parse_card({
                "name": back_face_name,
                "oracle_text": back_face_oracle_text,
                "type_line": back_face_type_line,
                "mana_cost": back_face_mana_cost,
            })
        except Exception:
            back_face_ast = None
        # For DFCs we prefer the FRONT face's AST + P/T at top level
        # because the card enters battlefield front-face-up (§712.2).
        if front_face_ast is not None:
            ast = front_face_ast
    return CardEntry(
        name=c["name"],
        mana_cost=c.get("mana_cost") or (front_face_mana_cost or ""),
        cmc=int(c.get("cmc") or (front_face_cmc or 0) or 0),
        type_line=c.get("type_line") or (front_face_type_line or ""),
        oracle_text=c.get("oracle_text") or (front_face_oracle_text or ""),
        power=_parse_pt(c.get("power")) if c.get("power") is not None else front_face_power,
        toughness=_parse_pt(c.get("toughness")) if c.get("toughness") is not None else front_face_toughness,
        ast=ast,
        colors=tuple(colors_raw) or front_face_colors,
        # Scryfall 'loyalty' for planeswalkers (e.g. "5", "3"); None otherwise.
        # Used by CR 704.5i SBA + planeswalker ETB init.
        starting_loyalty=_parse_pt(c.get("loyalty")),
        # Scryfall 'defense' for battles. Same idea.
        starting_defense=_parse_pt(c.get("defense")),
        layout=layout,
        is_dfc=is_dfc,
        back_face_name=back_face_name,
        back_face_ast=back_face_ast,
        back_face_mana_cost=back_face_mana_cost,
        back_face_cmc=back_face_cmc,
        back_face_type_line=back_face_type_line,
        back_face_oracle_text=back_face_oracle_text,
        back_face_power=back_face_power,
        back_face_toughness=back_face_toughness,
        back_face_colors=back_face_colors,
        front_face_name=front_face_name,
        front_face_ast=front_face_ast,
        front_face_mana_cost=front_face_mana_cost,
        front_face_cmc=front_face_cmc,
        front_face_type_line=front_face_type_line,
        front_face_oracle_text=front_face_oracle_text,
        front_face_power=front_face_power,
        front_face_toughness=front_face_toughness,
        front_face_colors=front_face_colors,
    )


def build_deck(deck_list, cards_by_name, rng: random.Random) -> list[CardEntry]:
    entries: list[CardEntry] = []
    for name, count in deck_list:
        ce = load_card_by_name(cards_by_name, name)
        if ce is None:
            continue
        for _ in range(count):
            entries.append(ce)
    # Pad up to DECK_SIZE with basic lands matching the deck's dominant color.
    if len(entries) < DECK_SIZE:
        fill_land = "Mountain"
        # dumb heuristic: whichever basic land is already in the deck most
        from collections import Counter as C
        basic_count = C()
        for e in entries:
            if e.type_line.lower().startswith("basic land"):
                basic_count[e.name] += 1
        if basic_count:
            fill_land = basic_count.most_common(1)[0][0]
        land_ce = load_card_by_name(cards_by_name, fill_land)
        while len(entries) < DECK_SIZE and land_ce is not None:
            entries.append(land_ce)
    elif len(entries) > DECK_SIZE:
        entries = entries[:DECK_SIZE]
    rng.shuffle(entries)
    return entries


# ============================================================================
# Effect resolution
# ============================================================================

def unwrap_spell_effect(ab) -> Optional[Effect]:
    """Many sorcery/instant cards parse as Static(Modification(kind='spell_effect', args=(Effect,))).
    Extract the inner effect node. Returns None if not a spell_effect or wrapped Effect."""
    if isinstance(ab, Static) and ab.modification and ab.modification.kind == "spell_effect":
        if ab.modification.args:
            e = ab.modification.args[0]
            if hasattr(e, "kind"):
                return e
    return None


# Some common cards get a "parsed_tail" Modification that keeps raw text.
# We extract simple damage patterns as a fallback.
_DAMAGE_TAIL_RE = re.compile(r"~ deals (\d+) damage to (any target|target player|target creature|target opponent)")


def synthesize_effect_from_parsed_tail(raw_text: str) -> Optional[Effect]:
    m = _DAMAGE_TAIL_RE.search(raw_text)
    if m:
        amount = int(m.group(1))
        tgt_txt = m.group(2)
        if "creature" in tgt_txt:
            tgt = Filter(base="creature", targeted=True)
        elif "player" in tgt_txt or "opponent" in tgt_txt:
            tgt = Filter(base="player", targeted=True)
        else:
            tgt = Filter(base="any_target", targeted=True)
        return Damage(amount=amount, target=tgt)
    return None


# ---- Ramp fallback -----------------------------------------------------------
# Match common "search your library for ... land ... battlefield" tails that
# either (a) sit in CardAST.parse_errors because the grammar skipped them
# (Rampant Growth, Farseek), or (b) arrive as UnknownEffect inside a
# Triggered/Activated body (Wood Elves, Solemn Simulacrum). We synthesize a
# Tutor node so the existing `tutor` resolver executes the ramp.
_RAMP_LAND_RE = re.compile(
    r"search your library for "
    r"(?:a |an |up to (?:one|two|three|\d+) )?"
    r"(basic land|land|forest|plains|island|swamp|mountain|"
    r"plains,\s*island,\s*swamp,?\s*or\s*mountain|"
    r"[a-z, ]+?)"
    r"\s*cards?,"
    r"[^.]*?"
    r"onto the battlefield"
    r"(?:\s+tapped)?",
    re.IGNORECASE,
)


def synthesize_tutor_from_raw(raw_text: str) -> Optional[Effect]:
    """Build a Tutor from a raw 'search your library for ...' sentence.

    Returns None if the text doesn't match a ramp pattern.
    """
    if not raw_text:
        return None
    text = raw_text.lower().strip()
    m = _RAMP_LAND_RE.search(text)
    if not m:
        return None
    land_phrase = m.group(1).strip()
    if "basic land" in land_phrase:
        base = "basic_land_card"
    elif land_phrase in ("forest", "plains", "island", "swamp", "mountain"):
        base = "basic_land_card"
    elif ", " in land_phrase or " or " in land_phrase:
        # "plains, island, swamp, or mountain" — treat as basic for MVP.
        base = "basic_land_card"
    else:
        base = "land_card"
    destination = "battlefield_tapped" if " tapped" in text else "battlefield"
    return Tutor(
        query=Filter(base=base),
        destination=destination,
        count=1,
        shuffle_after=True,
    )


def collect_spell_effects(ast: CardAST) -> list[Effect]:
    """For sorceries/instants, gather the effects to resolve when the spell resolves."""
    effs: list[Effect] = []
    for ab in ast.abilities:
        e = unwrap_spell_effect(ab)
        if e is not None:
            # If the parser wrapped an UnknownEffect in spell_effect, try synth.
            if getattr(e, "kind", None) == "unknown":
                synth = synthesize_tutor_from_raw(getattr(e, "raw_text", ""))
                if synth is not None:
                    effs.append(synth)
                    continue
            effs.append(e)
            continue
        if isinstance(ab, Static) and ab.modification and ab.modification.kind == "parsed_tail":
            raw = ab.modification.args[0] if ab.modification.args else ""
            synth = synthesize_effect_from_parsed_tail(raw)
            if synth:
                effs.append(synth)
                continue
            ramp = synthesize_tutor_from_raw(raw)
            if ramp:
                effs.append(ramp)
    # Fallback: scan parse_errors for ramp sentences the grammar dropped
    # entirely (Rampant Growth, Farseek).
    for raw in ast.parse_errors:
        synth = synthesize_tutor_from_raw(raw)
        if synth is not None:
            effs.append(synth)
    return effs


# ---------------------------------------------------------------------------
# Per-card (snowflake) runtime dispatch
# ---------------------------------------------------------------------------
#
# per_card.py emits three distinct shapes we have to recognize at runtime:
#
#   (a) Static with Modification(kind="custom", args=(slug, ...))
#       Spells like Doomsday use this to mark "when this resolves, run this
#       named custom effect". We walk the AST for these on spell resolution.
#
#   (b) Triggered(trigger=Trigger(event=<event>),
#                 effect=UnknownEffect(raw_text="<per-card:slug>"))
#       The _custom_triggered() builder wraps the slug inside an
#       UnknownEffect. We extract the slug via regex when a trigger fires
#       at the matching lifecycle event (ETB, LTB, upkeep, ...).
#
#   (c) Activated(... effect=UnknownEffect(raw_text="<per-card:slug>"))
#       Currently surfaced through ``resolve_effect``'s unknown branch —
#       we add a peek-at-raw-text dispatch there.
#
# Every slug lookup goes through ``_dispatch_per_card`` (imported at the
# top of this module). If the slug isn't registered, the call returns
# False and the caller emits a ``per_card_unhandled`` breadcrumb so we
# can catalogue the gap.

_PER_CARD_SLUG_RE = re.compile(r"<per-card:([^>]+)>")


def _extract_custom_slugs(ab) -> list[tuple[str, tuple]]:
    """Return the (slug, args) pairs hanging off this ability, if any.

    Handles both Static(Modification(kind='custom', args=(slug, ...)))
    and per_card's triggered/activated shape where the slug is embedded
    in ``UnknownEffect.raw_text`` as ``<per-card:slug>``.
    """
    out: list[tuple[str, tuple]] = []
    if isinstance(ab, Static) and ab.modification and ab.modification.kind == "custom":
        args = ab.modification.args or ()
        if args:
            out.append((str(args[0]), tuple(args[1:])))
        return out
    # Triggered / Activated with UnknownEffect body
    eff = getattr(ab, "effect", None)
    if eff is not None and getattr(eff, "kind", None) == "unknown":
        m = _PER_CARD_SLUG_RE.search(getattr(eff, "raw_text", "") or "")
        if m:
            out.append((m.group(1), ()))
    # Also inspect raw ability text as a last-ditch fallback
    raw = getattr(ab, "raw", "") or ""
    if raw:
        m = _PER_CARD_SLUG_RE.search(raw)
        if m:
            slug = m.group(1)
            if not any(existing[0] == slug for existing in out):
                out.append((slug, ()))
    return out


def apply_per_card_spell_effects(game: "Game", source_seat: int,
                                 card: "CardEntry") -> int:
    """Fire every custom-static handler registered on a resolving *spell*
    (sorcery/instant). Returns the count of handlers that fired.

    For permanent spells we call ``apply_per_card_etb`` instead, after
    the permanent is on the battlefield.
    """
    if _dispatch_per_card is None:
        return 0
    fired = 0
    seen: set[str] = set()
    # 1. Walk all Static(custom) abilities on the AST.
    for ab in card.ast.abilities:
        if isinstance(ab, Triggered):
            # Skip triggered abilities — those fire from their dedicated
            # lifecycle hook (ETB / LTB / upkeep / end_step), not at
            # spell-resolution time.
            continue
        for slug, args in _extract_custom_slugs(ab):
            if slug in seen:
                continue
            seen.add(slug)
            if _dispatch_per_card(game, source_seat, card, slug,
                                  {"spell_resolution": True, "args": args}):
                fired += 1
            else:
                game.ev("per_card_unhandled",
                        slug=slug, card=card.name, site="spell")
    # 2. Name-based second-chance for cards whose per_card.py handler
    #    doesn't emit the slugs we expect (or the card isn't in per_card).
    name_slugs = _NAME_TO_SPELL_SLUGS.get(card.name, [])
    for slug in name_slugs:
        if slug in seen:
            continue
        seen.add(slug)
        if _dispatch_per_card(game, source_seat, card, slug,
                              {"spell_resolution": True,
                               "name_based": True}):
            fired += 1
    return fired


def apply_per_card_etb(game: "Game", perm: "Permanent") -> int:
    """Fire every per-card ETB-triggered handler for a permanent that just
    entered the battlefield. Called from _resolve_stack_top.
    """
    if _dispatch_per_card is None:
        return 0
    card = perm.card
    seat_idx = perm.controller
    fired = 0
    seen: set[str] = set()
    # 1. Triggered(etb) abilities with per-card slug
    for ab in card.ast.abilities:
        if isinstance(ab, Triggered) and ab.trigger.event == "etb":
            for slug, args in _extract_custom_slugs(ab):
                if slug in seen:
                    continue
                seen.add(slug)
                if _dispatch_per_card(game, seat_idx, card, slug,
                                      {"permanent": perm, "args": args}):
                    fired += 1
                else:
                    game.ev("per_card_unhandled",
                            slug=slug, card=card.name, site="etb")
        # Static(custom) abilities that represent on-ETB set-up effects —
        # for per_card cards the Static-custom stub also serves as
        # "apply this when you enter the battlefield" (e.g. Painter's
        # Servant color wash, Humility strip abilities).
        if isinstance(ab, Static) and ab.modification and ab.modification.kind == "custom":
            for slug, args in _extract_custom_slugs(ab):
                if slug in seen:
                    continue
                seen.add(slug)
                if _dispatch_per_card(game, seat_idx, card, slug,
                                      {"permanent": perm, "args": args}):
                    fired += 1
    # 2. Name-based fallback for ETB triggers (e.g. Thassa's Oracle
    #    isn't in per_card.py — it parses with an if_intervening_tail
    #    that the grammar can't consume; we wire the win check by name).
    etb_slug = _NAME_TO_ETB_SLUG.get(card.name)
    if etb_slug and etb_slug not in seen:
        seen.add(etb_slug)
        if _dispatch_per_card(game, seat_idx, card, etb_slug,
                              {"permanent": perm, "name_based": True}):
            fired += 1
    return fired


def apply_per_card_ltb(game: "Game", perm: "Permanent") -> int:
    """Fire every per-card LTB-triggered handler for a permanent leaving
    the battlefield. Called from zone-change codepaths.
    """
    if _dispatch_per_card is None:
        return 0
    card = perm.card
    seat_idx = perm.controller
    fired = 0
    seen: set[str] = set()
    for ab in card.ast.abilities:
        if isinstance(ab, Triggered) and ab.trigger.event == "ltb":
            for slug, args in _extract_custom_slugs(ab):
                if slug in seen:
                    continue
                seen.add(slug)
                if _dispatch_per_card(game, seat_idx, card, slug,
                                      {"permanent": perm, "args": args}):
                    fired += 1
    ltb_slug = _NAME_TO_LTB_SLUG.get(card.name)
    if ltb_slug and ltb_slug not in seen:
        seen.add(ltb_slug)
        if _dispatch_per_card(game, seat_idx, card, ltb_slug,
                              {"permanent": perm, "name_based": True}):
            fired += 1
    return fired


def collect_etb_effects(ast: CardAST) -> list[Effect]:
    """Triggered abilities on ETB."""
    effs: list[Effect] = []
    for ab in ast.abilities:
        if isinstance(ab, Triggered) and ab.trigger.event == "etb":
            if ab.effect is None:
                continue
            # Try ramp synthesis if the body is Unknown (Wood Elves, Solemn
            # Simulacrum). Tutor nodes generated here flow through the normal
            # `tutor` resolver.
            if getattr(ab.effect, "kind", None) == "unknown":
                synth = synthesize_tutor_from_raw(getattr(ab.effect, "raw_text", ""))
                if synth is not None:
                    effs.append(synth)
                    continue
            effs.append(ab.effect)
    return effs


def _apply_self_etb_synthesized(game, perm) -> None:
    """Fallback ETB synthesis hook. Currently delegates to the unknown/tutor
    path inside collect_etb_effects, so this is a no-op stub. Defined here to
    keep cast_spell importable while another agent finishes its implementation
    for self-referential ETB triggers (Craterhoof-class anthems)."""
    return


def collect_upkeep_effects(ast: CardAST) -> list[Effect]:
    effs: list[Effect] = []
    for ab in ast.abilities:
        if isinstance(ab, Triggered) and ab.trigger.phase == "upkeep":
            if ab.effect is not None:
                effs.append(ab.effect)
    return effs


def has_keyword(ast: CardAST, name: str) -> bool:
    return any(isinstance(ab, Keyword) and ab.name == name for ab in ast.abilities)


def get_tap_mana_ability(ast: CardAST) -> Optional[AddMana]:
    """Find a {T}: Add mana activated ability, if any."""
    for ab in ast.abilities:
        if isinstance(ab, Activated) and ab.cost.tap and ab.cost.mana is None:
            if isinstance(ab.effect, AddMana):
                return ab.effect
    return None


def _opponent_land_colors(game: "Game", seat_idx: int) -> set:
    """Union of colors produced by opponents' lands.

    Used by Fellwar Stone and similar "color a land an opponent controls
    could produce" effects. CR §106.1b — mana colors are W, U, B, R, G, C.

    Per 7174n1c ruling 2026-04-16: runtime check, fresh at each tap.
    Iterates every opposing seat's battlefield, reads each land's
    basic_land_mana (for basics) and AST tap-mana ability (for
    nonbasics). Returns the set of distinct colors producible.

    Edge case (noted, not handled): opponents can't strategically tap
    their own lands before Fellwar fires to deny colors — that's
    strategic counterplay and outside the rules-engine scope.
    """
    colors: set = set()
    for other_seat in game.seats:
        if other_seat.idx == seat_idx or other_seat.lost:
            continue
        for perm in other_seat.battlefield:
            if not perm.is_land:
                continue
            bl = basic_land_mana(perm.card.type_line)
            if bl:
                for sym in bl.pool:
                    if sym.color:
                        colors.update(sym.color)
                if bl.any_color_count > 0:
                    colors.update({"W", "U", "B", "R", "G"})
                continue
            tm = get_tap_mana_ability(perm.card.ast)
            if tm:
                for sym in tm.pool:
                    if sym.color:
                        colors.update(sym.color)
                if tm.any_color_count > 0:
                    colors.update({"W", "U", "B", "R", "G"})
    return colors


def basic_land_mana(type_line: str) -> Optional[AddMana]:
    """Basic lands have implicit mana abilities not printed in oracle text.
    We synthesize one pip of the land's associated color.

    CR §305.6 — "If an object has the land subtype Plains, it has '{T}:
    Add {W}' …" and likewise for Island/{U}, Swamp/{B}, Mountain/{R},
    Forest/{G}, Wastes/{C}."""
    t = type_line.lower()
    if "basic" not in t or "land" not in t:
        return None
    # Map subtype → color. Dual-typed basics (Snow-Covered Plains etc.)
    # still have the basic land subtype; we read the first land subtype
    # we find.
    color = None
    if "plains" in t: color = "W"
    elif "island" in t: color = "U"
    elif "swamp" in t: color = "B"
    elif "mountain" in t: color = "R"
    elif "forest" in t: color = "G"
    elif "wastes" in t: color = "C"
    if color is None:
        # Unknown basic — fall back to any-color so the game still plays.
        return AddMana(pool=(), any_color_count=1)
    # Synthesize a single-pip AddMana of the right color. We use the raw
    # ManaSymbol constructor since we know the shape.
    sym = ManaSymbol(raw="{%s}" % color, generic=0, color=(color,))
    return AddMana(pool=(sym,), any_color_count=0)


def _add_mana_colors(ae: AddMana) -> list[str]:
    """Flatten an AddMana effect into a list of single-color codes.
    Each entry is 'W'/'U'/'B'/'R'/'G'/'C'/'any'. Length = total pips
    the ability produces. Used by add_mana_from_ability to route each
    pip into the typed pool.

    Hybrid symbols (e.g. {U/B}) are reported as 'any' for MVP — picking
    which side to generate is a player choice; a later pass may model
    this properly (coloring would depend on §117.6 for auto-resolution
    when the opt is forced)."""
    colors: list[str] = []
    for sym in ae.pool:
        if sym.is_phyrexian:
            # Phyrexian pay-2-life alternative — the "{P/color}" symbol
            # produces the color side when tapped. We report the color.
            if sym.color:
                colors.append(sym.color[0])
            else:
                colors.append("any")
            continue
        if sym.color:
            if len(sym.color) == 1:
                colors.append(sym.color[0])
            else:
                # Hybrid — treat as any-color for now.
                colors.append("any")
        elif sym.raw == "{C}":
            colors.append("C")
        elif sym.generic > 0:
            # Generic pool entries ({2}) produce N any-color mana.
            for _ in range(sym.generic):
                colors.append("any")
        # X/S: skip for now. Snow treated as any.
        elif sym.is_snow:
            colors.append("any")
    any_n = 0
    if isinstance(ae.any_color_count, int):
        any_n = ae.any_color_count
    for _ in range(max(0, any_n)):
        colors.append("any")
    return colors


def add_mana_from_ability(game: "Game", seat: "Seat", ae: AddMana,
                          source_name: str = "", source_kind: str = "effect") -> int:
    """Route the pips produced by an AddMana into the seat's typed pool.
    Returns the number of pips added."""
    pips = _add_mana_colors(ae)
    for c in pips:
        seat.mana.add(c, 1)
    if pips:
        game.ev("add_mana", seat=seat.idx, amount=len(pips),
                source=source_kind, source_card=source_name,
                colors="".join(pips),
                pool_after=seat.mana.total())
    return len(pips)


# --- target selection ---

def threat_score(game: Game, source_seat: int, target_seat: Seat) -> float:
    """Heuristic threat score the ``source_seat`` uses to pick which
    opponent to target. Higher = more threatening = pick them first.

    Factors:
      - Sum of power of creatures on target's battlefield.
      - Commander damage this source has already dealt to target
        (chunk toward the 21 threshold — finishing a commit is strong).
      - Low-life bonus (≤10 life → killable, finish them).
      - Combo-piece / high-CMC permanent count (proxy for scary boards).

    Works for any N >=2. Already-eliminated opponents return -inf so
    they're never picked.
    """
    if target_seat.lost:
        return float("-inf")
    score = 0.0
    # Board power contribution.
    score += sum(max(0, p.power) for p in target_seat.battlefield
                 if p.is_creature)
    # Commander damage leverage. Count damage that the SOURCE seat's
    # commander has already dealt to this target. commander_damage is
    # nested (dealer_seat → name → int); we only credit damage that WE
    # (source_seat) have dealt, because §704.6c is per-commander and
    # partner pairs share a dealer seat.
    src = game.seats[source_seat]
    by_dealer = target_seat.commander_damage.get(source_seat, {})
    for nm in src.commander_names:
        score += 2 * by_dealer.get(nm, 0)
    # Low-life finish bonus.
    if target_seat.life <= 10:
        score += 5
    if target_seat.life <= 5:
        score += 5
    # Combo-piece / high-CMC proxy: permanents with CMC >= 5 suggest
    # scary late-game engines.
    score += sum(1 for p in target_seat.battlefield
                 if (p.card.cmc or 0) >= 5)
    return score


def _pick_opponent_by_threat(game: Game, source_seat: int) -> Optional[Seat]:
    """Return the highest-threat living opponent for ``source_seat``,
    breaking ties by next-in-turn-order (left neighbor). Returns None
    if no living opponents exist."""
    living = [s for s in game.seats if not s.lost and s.idx != source_seat]
    if not living:
        return None
    n = len(game.seats)

    def _key(seat_obj: Seat) -> tuple:
        # Distance-in-turn-order from source (1 = immediate left neighbor).
        dist = (seat_obj.idx - source_seat) % n
        return (-threat_score(game, source_seat, seat_obj), dist)

    living.sort(key=_key)
    return living[0]


def pick_target(game: Game, source_seat: int, flt: Filter) -> tuple[str, object]:
    """Return (kind, thing) where kind in {'player','permanent','none'}.

    This is now a thin dispatcher: the source seat's policy does the
    actual picking (via ``choose_target``). The :class:`GreedyHat`
    default preserves the original behavior — threat-ranked opponent
    for player/opponent/any_target, lowest-toughness creature across
    all opponents for creature, etc. Alternate hats (Poker, LLM)
    plug in at the same call site with zero engine changes.
    """
    seat = game.seats[source_seat]
    # Engine-supplied legal-target short-list (for policies that want to
    # pick from a concrete list rather than re-derive from the filter).
    # Computed lazily by base kind to keep the happy path cheap.
    legal: list = []
    base = flt.base
    if base in ("player", "opponent", "any_target"):
        legal = [s for s in game.seats
                 if not s.lost and s.idx != source_seat]
    elif base == "self":
        legal = [seat]
    elif base == "creature":
        opps = [s for s in game.seats
                if not s.lost and s.idx != source_seat]
        pool: list[Permanent] = []
        for opp in opps:
            pool.extend(p for p in opp.battlefield if p.is_creature)
        if flt.targeted:
            pool = [p for p in pool if not is_hexproof(p)]
        legal = pool
    if seat.policy is not None:
        out = seat.policy.choose_target(game, seat, flt, legal)
        # Back-compat: policies may return either the legacy
        # (kind, target) tuple or a bare target (which we wrap here).
        if isinstance(out, tuple) and len(out) == 2 and isinstance(out[0], str):
            return out
        if out is None:
            return "none", None
        if isinstance(out, Seat):
            return "player", out
        if isinstance(out, Permanent):
            return "permanent", out
        return "none", None
    # No policy attached — engine has no default strategy. Return a
    # safe no-target.
    return "none", None


# --- dynamic amount evaluator ---
#
# MTG effects that read an integer sometimes carry a typed ``ScalingAmount``
# placeholder (see ``mtg_ast.ScalingAmount``) instead of a bare int.
# Everywhere the resolver reads ``effect.amount`` or ``effect.count`` it now
# routes through ``resolve_amount`` so scaling expressions are evaluated
# against the current game state. Static ints fall through unchanged.
#
# Coverage:
#   - devotion (per color & compound multi-color)
#   - devotion_chosen_color (Nykthos — requires chosen_color context)
#   - count_filter (permanents-you-control matching a Filter)
#   - cards_in_zone ((zone, whose))
#   - literal (escape hatch)
#   - x (reads from a transient game.x_value slot set by the caster)
#   - raw (unknown scaling, falls back to ``default`` and logs a warning)

_DEVOTION_PIP_RE = re.compile(r"\{([^}]+)\}")


def _count_devotion(seat: Seat, colors: tuple) -> int:
    """Count mana symbols of any of ``colors`` in mana costs of permanents the
    seat controls. Rule 700.5: hybrid pips count for EITHER color; phyrexian
    {W/P} counts as {W}. A hybrid pip {R/G} counts once toward devotion to red
    AND once toward devotion to green (if we're counting either) — but if the
    caller asks for devotion to red only, a single {R/G} pip still counts
    once (not twice).
    """
    wanted = set(c.upper() for c in colors)
    total = 0
    for perm in seat.battlefield:
        cost = perm.card.mana_cost or ""
        if not cost:
            continue
        for m in _DEVOTION_PIP_RE.finditer(cost):
            body = m.group(1).strip().upper()
            # Phyrexian: {W/P} → split on /, drop "P"
            parts = [p for p in body.split("/") if p and p != "P"]
            # Numeric only (e.g. "{2}") doesn't contribute.
            if all(p.isdigit() for p in parts):
                continue
            # Any part matches any wanted color → contributes 1.
            if any(p in wanted for p in parts):
                total += 1
    return total


def _permanents_matching(game: Game, source_seat: int, flt: Filter) -> list:
    """Return Permanent objects matching `flt` given the source controller.

    Supports the filter shapes commonly emitted by ``_parse_filter``:
      - ``you_control`` / ``opponent_controls`` scope
      - ``base`` noun (creature, artifact, enchantment, land, permanent, ...)
      - ``creature_types`` subtype filter
      - ``quantifier`` is ignored (counting is size of the list)
    """
    base = (flt.base or "permanent").lower()
    # Determine which seats' battlefields to scan.
    if flt.you_control:
        seats = [game.seats[source_seat]]
    elif flt.opponent_controls:
        seats = [game.opp(source_seat)]
    else:
        seats = list(game.seats)
    out: list = []
    for s in seats:
        for perm in s.battlefield:
            tl = perm.card.type_line.lower()
            # Base type match
            if base == "creature" and "creature" not in tl:
                continue
            if base == "artifact" and "artifact" not in tl:
                continue
            if base == "enchantment" and "enchantment" not in tl:
                continue
            if base == "land" and "land" not in tl:
                continue
            if base == "planeswalker" and "planeswalker" not in tl:
                continue
            # Subtype filter
            if flt.creature_types:
                if not all(ct.lower() in tl for ct in flt.creature_types):
                    continue
            out.append(perm)
    return out


def _count_cards_in_zone(game: Game, source_seat: int,
                         zone: str, whose: str) -> int:
    """Count cards in a given zone, per ``whose`` scope."""
    zone = (zone or "graveyard").lower()
    whose = (whose or "you").lower()
    if whose == "you":
        seats = [game.seats[source_seat]]
    elif whose == "target":
        # MVP has no real target-player tracking for counting expressions.
        # Default to opponent (the usual offensive use of "target player").
        seats = [game.opp(source_seat)]
    elif whose == "each_opp":
        seats = [s for s in game.seats if s.idx != source_seat]
    elif whose == "each_player":
        seats = list(game.seats)
    else:
        seats = [game.seats[source_seat]]
    total = 0
    for s in seats:
        if zone == "graveyard":
            total += len(s.graveyard)
        elif zone == "hand":
            total += len(s.hand)
        elif zone == "library":
            total += len(s.library)
        elif zone == "exile":
            total += len(s.exile)
    return total


def resolve_amount(game: Game, source_seat: int, amount_spec,
                   *, default: int = 1) -> int:
    """Evaluate a possibly-dynamic amount against the current game state.

    Accepts:
      * ``int``             → returned as-is
      * ``str`` "x" / "var" → returns ``default`` (0 for "x" with no context,
                              1 otherwise; pre-existing convention preserved)
      * ``ScalingAmount``   → dispatched on ``kind`` (devotion, count_filter,
                              cards_in_zone, devotion_chosen_color, literal, x,
                              life_lost_this_way, raw)
      * ``None``            → returns ``default``

    Unknown ``ScalingAmount.kind`` values log one ``scaling_unknown`` event
    and return ``default`` so the game keeps progressing. The event is
    picked up by ``rule_auditor`` so we can see where the engine has a gap.
    """
    if amount_spec is None:
        return default
    if isinstance(amount_spec, bool):
        # Guard: bools subclass int in Python; treat True/False as literals
        # (normally this path is unreachable, but the guard avoids surprises).
        return int(amount_spec)
    if isinstance(amount_spec, int):
        return amount_spec
    if isinstance(amount_spec, str):
        tok = amount_spec.strip().lower()
        if tok == "":
            return default
        if tok.isdigit():
            return int(tok)
        # "x" / "var" — no stack x_value in MVP yet; degrade to default.
        # (When a future pass wires spell-level X costs through the stack,
        # this branch can read ``game.current_x_value`` or similar.)
        return default
    if isinstance(amount_spec, ScalingAmount):
        kind = amount_spec.kind
        args = amount_spec.args or ()
        if kind == "literal":
            return int(args[0]) if args else default
        if kind == "devotion":
            seat = game.seats[source_seat]
            return _count_devotion(seat, args)
        if kind == "devotion_chosen_color":
            # Needs the spell/ability to have resolved a "chosen color" earlier.
            # The engine doesn't track chosen colors in MVP; pick the seat's
            # dominant color from mana-cost pips and devotion to that.
            seat = game.seats[source_seat]
            color_counts = {}
            for perm in seat.battlefield:
                for m in _DEVOTION_PIP_RE.finditer(perm.card.mana_cost or ""):
                    body = m.group(1).strip().upper()
                    parts = [p for p in body.split("/") if p and p != "P"]
                    for p in parts:
                        if p in ("W", "U", "B", "R", "G"):
                            color_counts[p] = color_counts.get(p, 0) + 1
            if not color_counts:
                return 0
            chosen = max(color_counts.items(), key=lambda kv: kv[1])[0]
            game.ev("chosen_color_heuristic", seat=source_seat, chosen=chosen,
                    devotion=color_counts[chosen])
            return color_counts[chosen]
        if kind == "count_filter":
            flt = args[0] if args else None
            if flt is None:
                return default
            return len(_permanents_matching(game, source_seat, flt))
        if kind == "cards_in_zone":
            zone = args[0] if len(args) >= 1 else "graveyard"
            whose = args[1] if len(args) >= 2 else "you"
            return _count_cards_in_zone(game, source_seat, zone, whose)
        if kind == "x":
            # Reserved for spell-level X; falls back to default until we
            # thread x_value through the stack.
            return default
        if kind == "life_lost_this_way":
            # Scoped to the resolving spell/ability — filled in by the
            # caller via a transient game attribute.
            return int(getattr(game, "_life_lost_this_way", default) or default)
        # Unknown kind — keep gameplay moving but flag.
        game.ev("scaling_unknown", kind=kind, args=repr(args), seat=source_seat)
        game.unknown_nodes[f"ScalingAmount:{kind}"] += 1
        return default
    # Fallback — unknown type, preserve legacy default.
    return default


# --- conditional evaluator ---
#
# The AST uses Condition(kind, args) to model intervening-if / if-otherwise /
# for-each-X / repeat-N / payoff-if-you-did style gating on Conditional effect
# nodes. Historically the resolver unconditionally fired the body, which was a
# latent correctness bug — every Conditional across the 31k-card corpus burned
# regardless of state. `evaluate_condition` walks the Condition schema and
# returns a tri-valued verdict:
#   * True        -> condition met, fire body
#   * False       -> condition not met, fire else_body (or skip)
#   * ("repeat", n) -> loop body n times (repeat_n / repeat_any_optional /
#                      for_each)
#
# Unknown kinds fall through to True + `conditional_unknown` event so the
# auditor can discover gaps without regressing existing behavior.


def _count_creatures(seat: Seat) -> int:
    return sum(1 for p in seat.battlefield if p.is_creature)


def _count_artifacts(seat: Seat) -> int:
    return sum(1 for p in seat.battlefield
               if "artifact" in p.card.type_line.lower())


def _count_lands(seat: Seat) -> int:
    return sum(1 for p in seat.battlefield if p.is_land)


def _count_gy_card_types(seat: Seat) -> int:
    """Number of distinct card types represented in the graveyard.
    Relevant for delirium (4+ types)."""
    type_words = ("artifact", "creature", "enchantment", "instant",
                  "land", "planeswalker", "sorcery", "tribal", "battle")
    seen = set()
    for c in seat.graveyard:
        tl = c.type_line.lower()
        for w in type_words:
            if w in tl:
                seen.add(w)
    return len(seen)


def _count_gy_instant_sorcery(seat: Seat) -> int:
    return sum(1 for c in seat.graveyard
               if "instant" in c.type_line.lower()
               or "sorcery" in c.type_line.lower())


def _count_distinct_creature_powers(seat: Seat) -> int:
    return len({p.power for p in seat.battlefield if p.is_creature})


def _total_creature_power(seat: Seat) -> int:
    return sum(p.power for p in seat.battlefield if p.is_creature)


def _max_creature_power(seat: Seat) -> int:
    powers = [p.power for p in seat.battlefield if p.is_creature]
    return max(powers) if powers else 0


def _basic_land_types(seat: Seat) -> int:
    """Count distinct basic land types you control (domain)."""
    basics = ("plains", "island", "swamp", "mountain", "forest")
    seen = set()
    for p in seat.battlefield:
        tl = p.card.type_line.lower()
        for b in basics:
            if b in tl:
                seen.add(b)
    return len(seen)


# Regex atlas for the free-form "if" / "intervening_if" / "etb_if" / "then_if"
# condition bodies. Parser stores raw sub-text in args[0]; we parse a small,
# well-tested set of shapes and fall through to "unknown" for the rest.
_COND_TEXT_RULES: list[tuple[re.Pattern, str]] = [
    # life thresholds
    (re.compile(r"\byou have (\d+) or less life\b", re.I), "you_life_leq"),
    (re.compile(r"\byou have (\d+) or more life\b", re.I), "you_life_geq"),
    (re.compile(r"\ban opponent has (\d+) or less life\b", re.I), "opp_life_leq"),
    (re.compile(r"\ban opponent has (\d+) or more life\b", re.I), "opp_life_geq"),
    (re.compile(r"\ba player has (\d+) or less life\b", re.I), "any_life_leq"),
    (re.compile(r"\ba player has 0 or less life\b", re.I), "any_dead"),
    # hand size
    (re.compile(r"\byou have no cards in (?:your )?hand\b", re.I), "hand_empty"),
    (re.compile(r"\byou have (\d+) or more cards in (?:your )?hand\b", re.I),
     "hand_geq"),
    (re.compile(r"\byou have (\d+) or fewer cards in (?:your )?hand\b", re.I),
     "hand_leq"),
    # library
    (re.compile(r"\byour library (?:has|contains) no cards\b", re.I), "lib_empty"),
    (re.compile(r"\byour library is empty\b", re.I), "lib_empty"),
    # graveyard
    (re.compile(r"\bseven or more cards in your graveyard\b", re.I), "gy_geq_7"),
    (re.compile(r"(?:there are )?(\d+) or more cards in your graveyard", re.I),
     "gy_geq_n"),
    # control counts (creatures/artifacts/lands)
    (re.compile(r"\byou control (?:a|an|one or more) creatures?\b", re.I),
     "ctrl_creature_geq_1"),
    (re.compile(
        r"\byou control (?:three|3) or more creatures with different powers\b",
        re.I), "coven"),
    (re.compile(r"\byou control (\d+) or more creatures?\b", re.I),
     "ctrl_creatures_geq"),
    (re.compile(r"\byou control (?:three|3) or more artifacts?\b", re.I),
     "metalcraft"),
    (re.compile(r"\byou control (\d+) or more artifacts?\b", re.I),
     "ctrl_artifacts_geq"),
    (re.compile(r"\byou control a creature with power (\d+) or (?:greater|more)\b",
                re.I), "ctrl_creature_power_geq"),
    (re.compile(
        r"\bcreatures you control have total power (\d+) or (?:greater|more)\b",
        re.I), "total_creature_power_geq"),
    # opponent controls
    (re.compile(r"\ban opponent controls (\d+) or more creatures?\b", re.I),
     "opp_creatures_geq"),
    (re.compile(r"\ban opponent controls (?:a|an) ([a-z]+)\b", re.I),
     "opp_controls_a"),
    (re.compile(r"\ban opponent controls four or more creatures\b", re.I),
     "opp_creatures_geq_4_word"),
    # would-clauses — these are replacement-effect style, not resolve-time
    # checks, so we treat them as unknown (default True preserves prior
    # behavior for the few Conditionals that model replacements this way).
    (re.compile(r"\bwould be put into a graveyard\b", re.I), "would_clause"),
    (re.compile(r"\bwould draw a card\b", re.I), "would_clause"),
]

# Spelled-out number mapping for "four or more" phrasings.
_NUM_WORDS = {
    "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
    "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}


def _eval_text_condition(game: Game, source_seat: int, text: str) -> Optional[bool]:
    """Evaluate free-form condition text for `if` / `intervening_if` / `etb_if`
    / `then_if` kinds. Returns True/False if a rule matched, else None for
    'unknown' (caller will default to True and emit conditional_unknown)."""
    if not text:
        return None
    lowered = text.strip().lower()
    # Normalize common spelled-out numbers ("four or more" -> "4 or more").
    for word, num in _NUM_WORDS.items():
        lowered = re.sub(rf"\b{word}\b(?=\s+or\s+(?:more|less|greater|fewer))",
                         str(num), lowered)
    seat = game.seats[source_seat]
    opp = game.opp(source_seat)
    for pat, tag in _COND_TEXT_RULES:
        m = pat.search(lowered)
        if not m:
            continue
        if tag == "you_life_leq":
            return seat.life <= int(m.group(1))
        if tag == "you_life_geq":
            return seat.life >= int(m.group(1))
        if tag == "opp_life_leq":
            return opp.life <= int(m.group(1))
        if tag == "opp_life_geq":
            return opp.life >= int(m.group(1))
        if tag == "any_life_leq":
            n = int(m.group(1))
            return any(s.life <= n for s in game.seats)
        if tag == "any_dead":
            return any(s.life <= 0 for s in game.seats)
        if tag == "hand_empty":
            return len(seat.hand) == 0
        if tag == "hand_geq":
            return len(seat.hand) >= int(m.group(1))
        if tag == "hand_leq":
            return len(seat.hand) <= int(m.group(1))
        if tag == "lib_empty":
            return len(seat.library) == 0
        if tag == "gy_geq_7":
            return len(seat.graveyard) >= 7
        if tag == "gy_geq_n":
            return len(seat.graveyard) >= int(m.group(1))
        if tag == "ctrl_creature_geq_1":
            return _count_creatures(seat) >= 1
        if tag == "coven":
            return (_count_creatures(seat) >= 3
                    and _count_distinct_creature_powers(seat) >= 3)
        if tag == "ctrl_creatures_geq":
            return _count_creatures(seat) >= int(m.group(1))
        if tag == "metalcraft":
            return _count_artifacts(seat) >= 3
        if tag == "ctrl_artifacts_geq":
            return _count_artifacts(seat) >= int(m.group(1))
        if tag == "ctrl_creature_power_geq":
            return _max_creature_power(seat) >= int(m.group(1))
        if tag == "total_creature_power_geq":
            return _total_creature_power(seat) >= int(m.group(1))
        if tag == "opp_creatures_geq":
            return _count_creatures(opp) >= int(m.group(1))
        if tag == "opp_creatures_geq_4_word":
            return _count_creatures(opp) >= 4
        if tag == "opp_controls_a":
            word = m.group(1)
            return any(word in p.card.type_line.lower() for p in opp.battlefield)
        if tag == "would_clause":
            # These are replacement-shaped conditions stamped into Conditional
            # wrappers; without a dedicated would-event hook we can't decide
            # them structurally. Return None so caller defaults to True.
            return None
    return None


def evaluate_condition(game: Game, source_seat: int,
                       condition) -> Union[bool, tuple]:
    """Evaluate a Condition AST node against current game state.

    Returns True/False for boolean gates. Returns a `("repeat", n)` tuple
    for loop-kind conditions (`repeat_n`, `repeat_any_optional`, `for_each`)
    so the caller can fire the body n times. Unknown kinds return True (caller
    records a `conditional_unknown` event).
    """
    if condition is None:
        return True
    kind = getattr(condition, "kind", None)
    args = getattr(condition, "args", ()) or ()
    seat = game.seats[source_seat]
    opp = game.opp(source_seat)

    # --- Greedy "did we do the thing?" gates ---------------------------------
    # Prior behavior was always-true; our greedy-AI policy takes optional
    # costs/options whenever offered, so True matches the simulator's actual
    # game flow. Flagged here so a future policy engine can flip them.
    if kind in ("paid_optional_cost", "did_prior_action", "may_choose",
                "if_you_did"):
        return True
    if kind == "if_you_didnt":
        return False

    # --- Loop / set-size gates ---------------------------------------------
    if kind == "repeat_n":
        n = args[0] if args else 1
        if isinstance(n, int):
            return ("repeat", max(0, n))
        # "x" or other symbolic — default to 1 iteration
        return ("repeat", 1)
    if kind == "repeat_any_optional":
        # Greedy: fire once (cheap and deterministic; full policy needs a
        # planner to decide the optimal iteration count).
        return ("repeat", 1)
    if kind == "for_each":
        # We don't yet parse the `set_clause` into a countable predicate.
        # Fire once to preserve prior behavior; future work: count
        # permanents/cards matching the filter.
        return ("repeat", 1)

    # --- Life thresholds (numeric args already normalized by parser) --------
    if kind == "life_threshold":
        # args = (who, op, n)
        who, op, n = args
        target = seat if who == "you" else opp
        v = target.life
        return _cmp(v, op, n)
    if kind == "life_delta_threshold":
        who, op, n = args
        target = seat if who == "you" else opp
        return _cmp(target.life - STARTING_LIFE, op, n)
    if kind == "life_vs_starting":
        who, op = args
        target = seat if who == "you" else opp
        return _cmp(target.life, op, STARTING_LIFE)
    if kind == "life_vs_half_starting":
        who, op = args
        target = seat if who == "you" else opp
        return _cmp(target.life, op, STARTING_LIFE // 2)
    if kind == "life_threshold_both":
        # args = ((who1, op1, n1), (who2, op2, n2))
        for who, op, n in args:
            target = seat if who == "you" else opp
            if not _cmp(target.life, op, n):
                return False
        return True

    # --- Ability-word conditions (from ability_words.py) --------------------
    if kind == "threshold":
        return len(seat.graveyard) >= 7
    if kind == "delirium":
        return _count_gy_card_types(seat) >= 4
    if kind == "spell_mastery":
        return _count_gy_instant_sorcery(seat) >= 2
    if kind == "ferocious":
        return _max_creature_power(seat) >= 4
    if kind == "formidable":
        return _total_creature_power(seat) >= 8
    if kind == "fateful_hour":
        return seat.life <= 5
    if kind == "hellbent":
        return len(seat.hand) == 0
    if kind == "metalcraft":
        return _count_artifacts(seat) >= 3
    if kind == "coven":
        return (_count_creatures(seat) >= 3
                and _count_distinct_creature_powers(seat) >= 3)
    if kind == "domain":
        return _basic_land_types(seat) >= 1  # "for each" — at least one
    if kind in ("lieutenant", "morbid", "battalion", "raid", "revolt",
                "landfall"):
        # These need turn-history or combat state we don't track yet.
        # Return None-equivalent via unknown path.
        return None  # type: ignore[return-value]

    # --- Free-form "if"-family bodies: args[0] is raw cond text -------------
    if kind in ("if", "intervening_if", "etb_if", "then_if", "conditional",
                "as_long_as"):
        text = args[0] if args and isinstance(args[0], str) else ""
        verdict = _eval_text_condition(game, source_seat, text)
        if verdict is None:
            return None  # type: ignore[return-value]
        return verdict

    # --- Unknown: let caller record + default to True ----------------------
    return None  # type: ignore[return-value]


def _cmp(v: int, op: str, n: int) -> bool:
    if op == ">=":
        return v >= n
    if op == "<=":
        return v <= n
    if op == ">":
        return v > n
    if op == "<":
        return v < n
    if op in ("==", "="):
        return v == n
    if op == "!=":
        return v != n
    return True


# --- resolver ---

def resolve_effect(game: Game, source_seat: int, effect: Effect,
                   depth: int = 0,
                   source_colors_hint: tuple[str, ...] = ()) -> None:
    """Dispatch an Effect node. Drops the event into the log.

    source_colors_hint: colors of the spell/ability source, used for
    protection checks when the source isn't a Permanent on the battlefield
    (e.g. instants/sorceries dealing damage).
    """
    if effect is None:
        return
    if depth > 20:
        game.emit("  (effect recursion cap)")
        return

    k = getattr(effect, "kind", None)

    # Per-card snowflake dispatch. Before falling through the normal
    # effect-kind table, check whether this is an UnknownEffect whose
    # raw_text carries a `<per-card:slug>` marker (emitted by the
    # triggered/activated-per-card builders in extensions/per_card.py).
    # If so, route to the runtime handler registry and return.
    if k == "unknown" and _dispatch_per_card is not None:
        raw = getattr(effect, "raw_text", "") or ""
        m = _PER_CARD_SLUG_RE.search(raw)
        if m:
            slug = m.group(1)
            if _dispatch_per_card(game, source_seat, None, slug,
                                  {"inline": True}):
                return
            # Unregistered slug — log a breadcrumb and fall through so
            # the legacy "unknown" branch still counts it for coverage.
            game.ev("per_card_unhandled", slug=slug, site="resolve_effect")

    if k == "sequence":
        for item in effect.items:
            resolve_effect(game, source_seat, item, depth + 1,
                           source_colors_hint=source_colors_hint)
            state_based_actions(game)
            if game.ended:
                return
        return

    if k == "choice":
        # Modal effect — the controller picks one (or more, for "choose
        # two" modal spells). Policy drives the decision; legacy default
        # (GreedyHat) returns [0] to preserve the original "pick
        # first option" behavior.
        if effect.options:
            seat_obj = game.seats[source_seat]
            idxs: list[int] = [0]
            if seat_obj.policy is not None:
                try:
                    idxs = seat_obj.policy.choose_mode(
                        game, seat_obj, effect, list(effect.options),
                    ) or [0]
                except Exception:
                    idxs = [0]
            # Clamp + dedupe — a policy can't pick out-of-range indices.
            seen: set = set()
            for i in idxs:
                if 0 <= i < len(effect.options) and i not in seen:
                    seen.add(i)
                    resolve_effect(game, source_seat, effect.options[i],
                                   depth + 1,
                                   source_colors_hint=source_colors_hint)
                    if game.ended:
                        return
        return

    if k == "optional":
        resolve_effect(game, source_seat, effect.body, depth + 1,
                       source_colors_hint=source_colors_hint)
        return

    if k == "conditional":
        cond = getattr(effect, "condition", None)
        cond_kind = getattr(cond, "kind", None) if cond is not None else None
        verdict = evaluate_condition(game, source_seat, cond) if cond is not None else True
        else_body = getattr(effect, "else_body", None)

        # Loop verdict: ("repeat", n) -> fire body n times. No else branch.
        if isinstance(verdict, tuple) and verdict and verdict[0] == "repeat":
            n = verdict[1]
            game.ev("conditional_evaluated",
                    condition_kind=cond_kind,
                    result="repeat",
                    iterations=n,
                    branch="body")
            if effect.body is not None:
                for _ in range(n):
                    resolve_effect(game, source_seat, effect.body, depth + 1,
                                   source_colors_hint=source_colors_hint)
                    if game.ended:
                        return
            return

        # Unknown condition: default to body (preserves prior behavior) and
        # flag for future parser work.
        if verdict is None:
            game.ev("conditional_unknown",
                    condition_kind=cond_kind,
                    condition_args=repr(getattr(cond, "args", ())))
            if effect.body is not None:
                resolve_effect(game, source_seat, effect.body, depth + 1,
                               source_colors_hint=source_colors_hint)
            return

        # Boolean verdict.
        if verdict:
            game.ev("conditional_evaluated",
                    condition_kind=cond_kind,
                    result=True,
                    branch="body")
            if effect.body is not None:
                resolve_effect(game, source_seat, effect.body, depth + 1,
                               source_colors_hint=source_colors_hint)
        else:
            game.ev("conditional_evaluated",
                    condition_kind=cond_kind,
                    result=False,
                    branch="else" if else_body is not None else "skip")
            if else_body is not None:
                resolve_effect(game, source_seat, else_body, depth + 1,
                               source_colors_hint=source_colors_hint)
        return

    if k == "damage":
        amount = resolve_amount(game, source_seat, effect.amount, default=1)
        # "X damage to each opponent" (Blasphemous Act has target=creature
        # but Pyrohemia, Earthquake, Rolling Earthquake, etc. distribute.
        # When quantifier=each and base=opponent/player, fan out.)
        is_each_opp = (
            effect.target.base in ("opponent", "each_opp")
            and getattr(effect.target, "quantifier", None) in ("each", "all")
        )
        is_each_player = getattr(effect.target, "quantifier", None) == "each_player"
        if is_each_opp or is_each_player:
            if is_each_player:
                targets_players = [s for s in game.seats if not s.lost]
            else:
                targets_players = [s for s in game.seats
                                   if not s.lost and s.idx != source_seat]
            for tgt in targets_players:
                before = tgt.life
                tgt.life -= amount
                game.emit(
                    f"  deal {amount} to seat {tgt.idx} "
                    f"(life → {tgt.life})")
                game.ev("damage", amount=amount,
                        target_kind="player", target_seat=tgt.idx)
                game.ev("life_change", seat=tgt.idx,
                        **{"from": before, "to": tgt.life})
                if tgt.life <= 0:
                    tgt.lost = True
                    tgt.loss_reason = "life total reached 0"
                    game.check_end()
                    if game.ended:
                        return
            return
        kind, tgt = pick_target(game, source_seat, effect.target)
        if kind == "player":
            before = tgt.life
            tgt.life -= amount
            game.emit(f"  deal {amount} to seat {tgt.idx} (life → {tgt.life})")
            game.ev("damage", amount=amount, target_kind="player", target_seat=tgt.idx)
            game.ev("life_change", seat=tgt.idx,
                    **{"from": before, "to": tgt.life})
            if tgt.life <= 0:
                tgt.lost = True
                tgt.loss_reason = "life total reached 0"
                game.check_end()
        elif kind == "permanent":
            # Protection: if the target has protection from the spell's color,
            # damage is prevented (0 dealt).
            prot = protection_colors(tgt)
            src_colors = set(source_colors_hint)
            if prot and ("*" in prot or (prot & src_colors)):
                game.emit(f"  damage {amount} to {tgt.card.name} "
                          f"(protection prevents)")
                game.ev("damage_prevented", amount=amount, target_kind="permanent",
                        target_card=tgt.card.name, target_seat=tgt.controller,
                        reason="protection")
            else:
                tgt.damage_marked += amount
                game.emit(f"  deal {amount} to {tgt.card.name} ({tgt.damage_marked}/{tgt.toughness})")
                game.ev("damage", amount=amount, target_kind="permanent",
                        target_card=tgt.card.name, target_seat=tgt.controller)
        else:
            game.emit(f"  damage {amount} — no legal target")
        return

    if k == "draw":
        count = resolve_amount(game, source_seat, effect.count, default=1)
        quant = getattr(effect.target, "quantifier", None)
        # "each player draws" (Howling Mine, Prosperity, Wheel of
        # Fortune templates); "each opponent draws" (rare, but exists).
        if quant == "each_player":
            targets = [s for s in game.seats if not s.lost]
        elif (effect.target.base in ("opponent", "each_opp")
              and quant in ("each", "all")):
            targets = [s for s in game.seats
                       if not s.lost and s.idx != source_seat]
        elif effect.target.base == "self":
            targets = [game.seats[source_seat]]
        elif effect.target.base in ("player", "opponent"):
            # Single target. "target player" → we prefer ourselves
            # (offensive engine / card-draw value). "target opponent" →
            # pick highest-threat opp via pick_target.
            if effect.target.base == "player":
                targets = [game.seats[source_seat]]
            else:
                picked = _pick_opponent_by_threat(game, source_seat)
                targets = [picked] if picked is not None else [
                    game.seats[source_seat]]
        else:
            targets = [game.seats[source_seat]]
        for target_seat in targets:
            draw_cards(game, target_seat, count)
            game.emit(
                f"  seat {target_seat.idx} draws {count} "
                f"(hand={len(target_seat.hand)})")
            game.ev("draw", seat=target_seat.idx,
                    count=count, hand_size=len(target_seat.hand))
            # Draw-trigger observers fire for effect-driven draws (NOT
            # a "first draw in draw step" so Bowmasters/Notion Thief
            # apply normally).
            _fire_draw_trigger_observers(game, target_seat.idx, count=count)
            if game.ended:
                return
        return

    if k == "discard":
        count = resolve_amount(game, source_seat, effect.count, default=1)
        # "each opponent discards" fan-out (Mind Twist / Hymn to Tourach
        # are single-target; Mindslicer / Wrench Mind / Sadistic Sacrament
        # aren't "each opponent" per se, but Cabal Interrogator / Dark Deal
        # etc. are — when we see quantifier each + base opponent, fan out).
        is_each_opp = (
            effect.target.base in ("opponent", "each_opp")
            and getattr(effect.target, "quantifier", None) in ("each", "all")
        )
        if effect.target.base == "self":
            targets = [game.seats[source_seat]]
        elif is_each_opp:
            targets = [s for s in game.seats
                       if not s.lost and s.idx != source_seat]
        else:
            picked = _pick_opponent_by_threat(game, source_seat)
            targets = [picked] if picked is not None else [
                game.seats[source_seat]]
        for target in targets:
            for _ in range(count):
                if target.hand:
                    gone = target.hand.pop(0)
                    target.graveyard.append(gone)
                    game.emit(f"  seat {target.idx} discards {gone.name}")
                    game.ev("discard", seat=target.idx, card=gone.name)
        return

    if k == "mill":
        count = resolve_amount(game, source_seat, effect.count, default=1)
        is_each_opp = (
            effect.target.base in ("opponent", "each_opp")
            and getattr(effect.target, "quantifier", None) in ("each", "all")
        )
        if effect.target.base == "self":
            targets = [game.seats[source_seat]]
        elif is_each_opp:
            targets = [s for s in game.seats
                       if not s.lost and s.idx != source_seat]
        else:
            picked = _pick_opponent_by_threat(game, source_seat)
            targets = [picked] if picked is not None else [
                game.seats[source_seat]]
        for target in targets:
            milled = []
            for _ in range(count):
                if target.library:
                    gone = target.library.pop(0)
                    target.graveyard.append(gone)
                    milled.append(gone.name)
            game.emit(f"  seat {target.idx} mills {count}")
            game.ev("mill", seat=target.idx,
                    count=len(milled), cards=milled)
            if not target.library:
                target.lost = True
                target.loss_reason = "decked out"
                game.check_end()
                if game.ended:
                    return
        return

    if k == "destroy":
        kind, tgt = pick_target(game, source_seat, effect.target)
        if kind == "permanent":
            # Indestructible: destroy effects don't destroy it (Rule 702.12b).
            if is_indestructible(tgt):
                game.emit(f"  destroy {tgt.card.name} — indestructible, no effect")
                game.ev("destroy_prevented", card=tgt.card.name,
                        seat=tgt.controller, reason="indestructible")
            else:
                do_permanent_die(game, tgt, reason="destroy_effect")
        elif effect.target.quantifier == "all":
            # Wrath-type — indestructible creatures survive, everyone else
            # dies. Route each through do_permanent_die so §614
            # replacements (Rest in Peace, Anafenza) apply.
            for s in game.seats:
                destroyed = 0
                survived_indestructible = 0
                for p in list(s.battlefield):
                    if not p.is_creature:
                        continue
                    if is_indestructible(p):
                        survived_indestructible += 1
                        continue
                    do_permanent_die(game, p, reason="wrath")
                    destroyed += 1
                if destroyed or survived_indestructible:
                    game.emit(f"  wrath → seat {s.idx} loses {destroyed} creatures"
                              + (f" ({survived_indestructible} indestructible survived)"
                                 if survived_indestructible else ""))
                    game.ev("wrath", seat=s.idx, destroyed=destroyed,
                            indestructible_survived=survived_indestructible)
        else:
            game.emit(f"  destroy — no legal target")
        return

    if k == "exile":
        kind, tgt = pick_target(game, source_seat, effect.target)
        if kind == "permanent":
            # §108.3: exiled cards go to their OWNER's exile zone.
            # §903.9a (§704.6d SBA) will return a commander to the
            # command zone next SBA pass — NOT a §614 replacement.
            owner_idx = tgt.owner_seat
            controller_seat = game.seats[tgt.controller]
            unregister_replacements_for_permanent(game, tgt)
            controller_seat.battlefield.remove(tgt)
            game.seats[owner_idx].exile.append(tgt.card)
            game.emit(f"  exile {tgt.card.name}")
        else:
            game.emit(f"  exile — no legal target")
        return

    if k == "bounce":
        kind, tgt = pick_target(game, source_seat, effect.target)
        if kind == "permanent":
            # Bounce sends to OWNER's hand (§108.3). For commanders in
            # a Commander game, §903.9b lets the owner redirect to
            # command zone instead — fire_zone_change routes through
            # the §614 replacement chain.
            owner_idx = tgt.owner_seat
            controller_seat = game.seats[tgt.controller]
            unregister_replacements_for_permanent(game, tgt)
            controller_seat.battlefield.remove(tgt)
            dest = "hand"
            if game.commander_format:
                zc_event = fire_zone_change(
                    game, tgt.card, owner_idx,
                    from_zone="battlefield", to_zone="hand")
                dest = zc_event.get("to_zone", "hand")
            if dest == "command_zone":
                game.seats[owner_idx].command_zone.append(tgt.card)
                game.emit(f"  bounce {tgt.card.name} → command zone "
                          f"(CR 903.9b)")
            else:
                game.seats[owner_idx].hand.append(tgt.card)
                game.emit(f"  bounce {tgt.card.name}")
        return

    if k == "gain_life":
        amount = resolve_amount(game, source_seat, effect.amount, default=1)
        if effect.target.base == "self":
            target = game.seats[source_seat]
        else:
            target = game.seats[source_seat]
        # §614: route through the would_gain_life replacement chain.
        # Alhammarret's Archive, Boon Reflection, Rhox Faithmender etc.
        # may double `amount`. A future "opponent_gain_life → lose_life"
        # replacement (Sulfuric Vortex, Tainted Remedy, Erebos, God of
        # the Dead) would fire from here as well.
        event = Event(type="would_gain_life", player=target.idx,
                      kwargs={"amount": amount})
        fire_event(game, event)
        if event.cancelled:
            game.emit(f"  seat {target.idx} life gain cancelled "
                      f"({event.get('cancelled_by','?')})")
            return
        amount = event.get("amount", amount)
        before = target.life
        target.life += amount
        game.emit(f"  seat {target.idx} gains {amount} life (→{target.life})")
        game.ev("life_change", seat=target.idx,
                **{"from": before, "to": target.life})
        return

    if k == "lose_life":
        amount = resolve_amount(game, source_seat, effect.amount, default=1)
        # "each opponent loses life" (Gray Merchant of Asphodel, Exsanguinate,
        # Debt to the Deathless, etc.) — quantifier "each" with
        # base="opponent" fan-outs to every living opponent. Without the
        # fan-out the life-drain only clips one seat and Gary/Exsang
        # undertune catastrophically in 4-player.
        is_each_opp = (
            effect.target.base == "opponent"
            and getattr(effect.target, "quantifier", None) in ("each", "all")
        )
        # ALSO accept "each other player" — another common templating.
        is_each_other = getattr(effect.target, "quantifier", None) == "each_other"
        if is_each_opp or is_each_other:
            targets = [s for s in game.seats
                       if not s.lost and s.idx != source_seat]
        else:
            # target filter base: self / opponent / player
            if effect.target.base == "self":
                targets = [game.seats[source_seat]]
            elif effect.target.base == "opponent":
                picked = _pick_opponent_by_threat(game, source_seat)
                targets = [picked] if picked is not None else [
                    game.seats[source_seat]]
            elif effect.target.base == "player" and getattr(effect.target, "targeted", False):
                # "target player loses N life" (Tendrils of Agony body,
                # Debt to the Deathless, Exquisite Blood secondary clauses)
                # — targeted life-loss is an attack spell by default. Aim
                # at the highest-threat opponent rather than draining
                # ourselves (which was the old fallback and made storm
                # copies of Tendrils do literally zero damage).
                picked = _pick_opponent_by_threat(game, source_seat)
                targets = [picked] if picked is not None else [
                    game.seats[source_seat]]
            else:
                targets = [game.seats[source_seat]]
        for target in targets:
            before = target.life
            target.life -= amount
            game.emit(
                f"  seat {target.idx} loses {amount} life "
                f"(→{target.life})")
            game.ev("life_change", seat=target.idx,
                    **{"from": before, "to": target.life})
            if target.life <= 0:
                target.lost = True
                target.loss_reason = "life total reached 0"
                game.check_end()
                if game.ended:
                    return
        return

    if k == "buff":
        kind, tgt = pick_target(game, source_seat, effect.target)
        # Buff power/toughness may carry a ScalingAmount (Nylea team pump
        # "where X is your devotion to green"). resolve_amount handles ints
        # and static "x" strings transparently.
        p_val = resolve_amount(game, source_seat, effect.power, default=0)
        t_val = resolve_amount(game, source_seat, effect.toughness, default=0)
        if kind == "permanent":
            tgt.buffs_pt = (tgt.buffs_pt[0] + p_val,
                            tgt.buffs_pt[1] + t_val)
            game.emit(f"  {tgt.card.name} gets +{p_val}/+{t_val}")
        elif effect.target.quantifier in ("all", "each") and \
                effect.target.you_control:
            # Team pump — apply to every creature the controller has.
            owner = game.seats[source_seat]
            touched = 0
            for perm in owner.battlefield:
                if perm.is_creature:
                    perm.buffs_pt = (perm.buffs_pt[0] + p_val,
                                     perm.buffs_pt[1] + t_val)
                    touched += 1
            if touched:
                game.emit(f"  team pump +{p_val}/+{t_val} → {touched} creatures")
        return

    if k == "create_token":
        count = resolve_amount(game, source_seat, effect.count, default=1)
        pt = effect.pt or (1, 1)
        types = " ".join(effect.types) or "Token"
        token_card = CardEntry(
            name=f"{types} Token ({pt[0]}/{pt[1]})",
            mana_cost="", cmc=0,
            type_line=f"Token Creature — {types}",
            oracle_text="",
            power=pt[0], toughness=pt[1],
            ast=CardAST(name=f"{types} token", abilities=(), parse_errors=(), fully_parsed=True),
        )
        controller = source_seat
        # §614: route through would_create_token so Doubling Season /
        # Anointed Procession / Parallel Lives can double `count`.
        event = Event(
            type="would_create_token",
            player=controller,
            kwargs={"count": count, "controller_seat": controller,
                    "token_name": token_card.name,
                    "types": types, "pt": pt},
        )
        fire_event(game, event)
        if event.cancelled:
            game.emit(f"  create_token cancelled "
                      f"({event.get('cancelled_by','?')})")
            return
        count = event.get("count", count)
        for _ in range(count):
            perm = Permanent(card=token_card, controller=controller,
                             summoning_sick=True)
            _etb_initialize(game, perm)
            game.seats[controller].battlefield.append(perm)
        game.emit(f"  create {count} {types} {pt[0]}/{pt[1]}")
        return

    if k == "add_mana":
        # Route colored pips into the typed ManaPool. Handles rituals
        # (Dark Ritual → {B}{B}{B}, Seething Song → {R}{R}{R}{R}{R}),
        # mana dorks resolving AddMana effects via activation, and
        # scaling amounts (Nykthos-style, any_color_count may be a
        # ScalingAmount).
        seat = game.seats[source_seat]
        # Resolve any_color_count which may be a ScalingAmount.
        any_count = resolve_amount(game, source_seat, effect.any_color_count,
                                   default=0)
        if any_count < 0:
            any_count = 0
        pip_colors: list[str] = []
        for sym in effect.pool:
            if sym.color and len(sym.color) == 1:
                pip_colors.append(sym.color[0])
            elif sym.raw == "{C}":
                pip_colors.append("C")
            elif sym.generic > 0:
                for _ in range(sym.generic):
                    pip_colors.append("any")
            else:
                pip_colors.append("any")
        for _ in range(any_count):
            pip_colors.append("any")
        for c in pip_colors:
            seat.mana.add(c, 1)
        pips = len(pip_colors)
        game.emit(f"  +{pips} mana (pool={seat.mana.total()})")
        game.ev("add_mana", seat=source_seat, amount=pips, source="effect",
                colors="".join(pip_colors),
                pool_after=seat.mana.total())
        return

    if k == "tutor":
        seat = game.seats[source_seat]
        lib = seat.library
        needle = (effect.query.base or "").lower()
        # Normalize a few aliases the parser uses interchangeably.
        if needle in ("basic_land", "basic"):
            needle = "basic_land_card"
        elif needle == "land":
            needle = "land_card"
        elif needle == "creature":
            needle = "creature_card"
        matcher = lambda ce: _tutor_matches(ce, needle)
        count = resolve_amount(game, source_seat, effect.count, default=1)
        if count <= 0:
            count = 1
        dest = effect.destination or "hand"
        is_land_query = needle in ("basic_land_card", "land_card") or "land" in needle
        # Per MTG rule: every printed "search for land, put onto the battlefield"
        # puts the land tapped. The parser's Cultivate/Kodama rule emits
        # destination='battlefield' (sans _tapped), so normalize that here.
        if dest == "battlefield" and is_land_query:
            dest = "battlefield_tapped"
        found = 0
        for _ in range(count):
            chosen = next((c for c in lib if matcher(c)), None)
            if chosen is None:
                break
            lib.remove(chosen)
            found += 1
            if dest in ("battlefield", "battlefield_tapped"):
                tapped = (dest == "battlefield_tapped")
                # Lands aren't summoning-sick (rule 302.1 applies only to creatures).
                is_land = "land" in chosen.type_line.lower()
                perm = Permanent(card=chosen, controller=source_seat,
                                 tapped=tapped,
                                 summoning_sick=(not is_land))
                _etb_initialize(game, perm)
                seat.battlefield.append(perm)
                game.ev("enter_battlefield", card=chosen.name, seat=source_seat,
                        summoning_sick=(not is_land))
            elif dest == "graveyard":
                seat.graveyard.append(chosen)
            elif dest == "top_of_library":
                lib.insert(0, chosen)
            else:  # "hand" (default) or anything else
                seat.hand.append(chosen)
            game.emit(f"  tutor → {chosen.name} to {dest}")
            game.ev("tutor", card=chosen.name, seat=source_seat, to=dest,
                    query=needle)
        if found == 0:
            game.emit(f"  tutor — no match for {needle}")
        if effect.shuffle_after:
            game.rng.shuffle(lib)
        return

    if k == "reanimate":
        gy = game.seats[source_seat].graveyard
        chosen = next((c for c in gy if "creature" in c.type_line.lower()), None)
        if chosen is not None:
            gy.remove(chosen)
            perm = Permanent(card=chosen, controller=source_seat,
                             summoning_sick=True)
            _etb_initialize(game, perm)
            game.seats[source_seat].battlefield.append(perm)
            game.emit(f"  reanimate {chosen.name}")
        return

    if k == "recurse":
        # Move a card from graveyard to hand or library_top. Recurse differs
        # from Reanimate in destination (hand vs battlefield) and typically
        # in query breadth (any card vs creature). Academy Ruins uses
        # destination="library_top" for the put-on-top variant.
        from_zone = getattr(effect, "from_zone", "your_graveyard")
        destination = getattr(effect, "destination", "hand")
        query = getattr(effect, "query", None)
        # Filter → string needle extraction (same pattern as tutor handler).
        # Previously passed Filter object to _tutor_matches which expected string → AttributeError.
        if query is not None and hasattr(query, "base"):
            needle = (query.base or "").lower()
            if needle in ("basic_land", "basic"):
                needle = "basic_land_card"
            elif needle == "land":
                needle = "land_card"
            elif needle == "creature":
                needle = "creature_card"
        else:
            needle = None
        gy_seats = [game.seats[source_seat]] if from_zone == "your_graveyard" else game.seats
        chosen = None
        chosen_owner = None
        for s in gy_seats:
            for c in s.graveyard:
                if needle is None or _tutor_matches(c, needle):
                    chosen = c
                    chosen_owner = s
                    break
            if chosen is not None:
                break
        if chosen is not None:
            chosen_owner.graveyard.remove(chosen)
            if destination == "library_top":
                game.seats[source_seat].library.insert(0, chosen)
                game.emit(f"  recurse {chosen.name} → library top")
                game.ev("recurse", seat=source_seat, card=chosen.name,
                        destination="library_top")
            elif destination == "battlefield":
                perm = Permanent(card=chosen, controller=source_seat,
                                 summoning_sick=True)
                _etb_initialize(game, perm)
                game.seats[source_seat].battlefield.append(perm)
                game.emit(f"  recurse {chosen.name} → battlefield")
                game.ev("recurse", seat=source_seat, card=chosen.name,
                        destination="battlefield")
            else:
                # default: hand
                game.seats[source_seat].hand.append(chosen)
                game.emit(f"  recurse {chosen.name} → hand")
                game.ev("recurse", seat=source_seat, card=chosen.name,
                        destination="hand")
        return

    if k == "scry":
        count = resolve_amount(game, source_seat, effect.count, default=1)
        # Dumb: look at top, bottom if it's a land and we have plenty, else keep
        s = game.seats[source_seat]
        for _ in range(count):
            if not s.library:
                break
            # Keep on top (no-op). Could do smarter sorting later.
        game.emit(f"  scry {count} (kept)")
        game.ev("scry", seat=source_seat, count=count)
        return

    if k == "surveil":
        count = resolve_amount(game, source_seat, effect.count, default=1)
        s = game.seats[source_seat]
        game.emit(f"  surveil {count} (no-op)")
        game.ev("surveil", seat=source_seat, count=count)
        return

    if k == "reveal":
        game.emit(f"  reveal (no-op)")
        return

    if k == "look_at":
        count = resolve_amount(game, source_seat, effect.count, default=1)
        # Log the evaluated count so downstream combo logic (Thassa's Oracle
        # §614 replacement-effect gap) can reason about it. The actual
        # "look" is a no-op in MVP — the §614 handler that turns this into
        # a win on empty-library lives in the replacement-effect wave.
        s = game.seats[source_seat]
        lib_size = len(s.library)
        game.emit(f"  look at top {count} (library size {lib_size})")
        game.ev("look_at", seat=source_seat, count=count,
                library_size=lib_size, zone=effect.zone)
        # Wave-2 hook: Thassa's Oracle win check. If the LookAt count is >=
        # the library size, the companion ``if X is greater than or equal
        # to the number of cards in your library, you win the game`` rider
        # (currently parsed as if_intervening_tail Modification) fires. The
        # proper fix lives in the §614 replacement-effect pass; this inline
        # check is gated on an emit-only log so we don't silently mutate
        # game outcomes without the replacement handler wired up.
        if count >= lib_size and lib_size >= 0:
            game.ev("thoracle_win_threshold_met", seat=source_seat,
                    count=count, library_size=lib_size)
        return

    if k == "shuffle":
        game.rng.shuffle(game.seats[source_seat].library)
        game.emit(f"  shuffle")
        return

    if k == "counter_spell":
        tgt = game._pending_counter_target
        if tgt is None:
            game.emit(f"  counter — no pending target")
            return
        if tgt.countered:
            game.emit(f"  counter — target already countered")
            return
        tgt.countered = True
        game.emit(f"  counter {tgt.card.name} (seat {tgt.controller})")
        game.ev("stack_counter", target_card=tgt.card.name,
                target_seat=tgt.controller)
        return

    if k == "tap":
        kind, tgt = pick_target(game, source_seat, effect.target)
        if kind == "permanent":
            tgt.tapped = True
            game.emit(f"  tap {tgt.card.name}")
        return

    if k == "untap":
        kind, tgt = pick_target(game, source_seat, effect.target)
        if kind == "permanent":
            tgt.tapped = False
            game.emit(f"  untap {tgt.card.name}")
        return

    if k == "gain_control":
        return  # stubbed

    if k == "counter_mod":
        kind, tgt = pick_target(game, source_seat, effect.target)
        count = resolve_amount(game, source_seat, effect.count, default=1)
        if kind == "permanent" and effect.counter_kind == "+1/+1":
            # §614: route through would_put_counter chain so Doubling
            # Season / Hardened Scales can modify the count. APNAP
            # ordering (§614.6) is handled by timestamp inside
            # _pick_replacement (Hardened Scales with an older
            # timestamp applies first, giving 1 → 2 → 4 with Doubling
            # Season vs the wrong 1 → 2 → 3 order).
            do_put_counter(game, tgt, "+1/+1", count)
        return

    if k == "prevent":
        game.emit(f"  prevent (no-op in MVP)")
        return

    if k == "extra_turn":
        # Rare; no-op for MVP to avoid infinite loops
        game.emit(f"  extra_turn (ignored)")
        return

    if k == "win_game":
        # N-seat: "you win the game" makes all OTHER living seats lose
        # (CR 104.2a: if a player wins, the game ends; in multiplayer
        # the winning player's opponents all lose simultaneously).
        game.seats[source_seat].lost = False
        for other in game.seats:
            if other.idx != source_seat and not other.lost:
                other.lost = True
                other.loss_reason = f"seat {source_seat} wins the game"
        game.check_end()
        return

    if k == "lose_game":
        game.seats[source_seat].lost = True
        game.seats[source_seat].loss_reason = "lose_game effect"
        game.check_end()
        return

    # Unknown / structural but unhandled
    if k == "unknown":
        # Last-chance ramp synthesis: some trigger bodies (Primeval Titan,
        # etc.) may flow into the resolver directly as UnknownEffect.
        raw = getattr(effect, "raw_text", "") or ""
        synth = synthesize_tutor_from_raw(raw)
        if synth is not None:
            resolve_effect(game, source_seat, synth, depth + 1)
            return
        game.unknown_nodes["UnknownEffect"] += 1
        game.emit(f"  (UnknownEffect: {raw[:60]!r})")
        return

    # Caller reached an uncovered kind — log it.
    game.unknown_nodes[str(k)] += 1
    game.emit(f"  (unhandled effect kind: {k})")


def _tutor_matches(card: CardEntry, needle: str) -> bool:
    n = needle.lower().strip()
    t = card.type_line.lower()
    if n in ("basic_land_card", "basic land card", "basic_land", "basic"):
        return "basic land" in t
    if n in ("land_card", "land card", "land"):
        return "land" in t
    if n in ("creature_card", "creature card", "creature"):
        return "creature" in t
    if n in ("artifact_card", "artifact card", "artifact"):
        return "artifact" in t
    if n == "card":
        return True
    # Basic-land type names ("forest", "plains", ...) — match a basic of that type.
    if n in ("forest", "plains", "island", "swamp", "mountain"):
        return ("basic land" in t) and (n in t)
    return n in t


# ============================================================================
# Engine primitives
# ============================================================================

def draw_cards(game: Game, seat: Seat, n: int) -> None:
    """Move N cards from top of library to hand, routing each draw
    through the §614 would_draw replacement chain.

    Model: CR 614.11a — "If an effect replaces a draw within a sequence
    of card draws, all actions required by the replacement are completed,
    if possible, before resuming the sequence." Each card in the
    requested N is its own would_draw event. The event carries a
    `count=1` that replacements (Alhammarret's Archive) may increase.
    Cancellation (Laboratory Maniac) consumes the draw.

    CR 614.5: a replacement applies once per event, so a doubler won't
    re-fire on the doubled cards.

    CR 704.5b: an attempt to draw from an empty library sets
    `attempted_draw_from_empty_library`. This happens AFTER the
    replacement chain has fired (so Lab Maniac can replace with a win
    before the empty-library loss takes effect).
    """
    for _ in range(n):
        event = Event(
            type="would_draw",
            player=seat.idx,
            kwargs={"count": 1, "library_size": len(seat.library)},
        )
        fire_event(game, event)
        if game.ended:
            return
        if event.cancelled:
            # A replacement (e.g. Lab Maniac win) fully replaced the draw.
            continue
        count = event.get("count", 1)
        # Each of the `count` cards is drawn as a result of this single
        # replacement-chain fire. Per CR 614.5 we do NOT re-fire the
        # chain for these — they're part of the modified event's
        # completion.
        for _j in range(count):
            if not seat.library:
                seat.attempted_draw_from_empty_library = True
                seat.lost = True
                seat.loss_reason = "drew from empty library (CR 704.5b)"
                game.ev("sba_704_5b_trigger", seat=seat.idx,
                        rule="704.5b", reason="draw_from_empty_library")
                game.check_end()
                return
            seat.hand.append(seat.library.pop(0))


# ----------------------------------------------------------------------------
# Face-down family — CR §702.37 (Morph), §702.37b (Megamorph), §702.168
# (Disguise), §701.40 (Manifest), §701.58 (Cloak), §701.62 (Manifest Dread),
# §708 (Face-Down Spells and Permanents).
#
# The helpers below provide:
#   face_down_characteristics(perm) -> dict
#       The §708.2a characteristic-override dict used by §613 layer-1b.
#   cast_face_down(game, seat, card, flip_cost, variant='vanilla')
#       Put a card from ``seat.hand`` onto the battlefield face down
#       via the Morph / Disguise special-cast path (CR §702.37c /
#       §702.168a). Does NOT use the stack in the current MVP engine
#       (which skips spell casting for most paths); the engine-level
#       integration is scoped to "the permanent enters face down".
#   manifest(game, seat, count=1) -> list[Permanent]
#       Top N cards of ``seat.library`` go onto the battlefield face
#       down (CR §701.40a). Returns the created Permanents.
#   cloak(game, seat, count=1) -> list[Permanent]
#       Same as manifest but the face-down form has ward {2}
#       (CR §701.58a).
#   manifest_dread(game, seat)
#       Look at top 2, manifest 1, put the other in graveyard
#       (CR §701.62a). The "which to manifest" decision routes
#       through seat.policy if available.
#   turn_face_up(game, perm, paying=True)
#       SPECIAL ACTION per CR §116.2g / §702.37e / §702.168d / §708.7.
#       Pays the flip cost iff ``paying=True``, clears the face-down
#       state, restores copiable values, and places a +1/+1 counter
#       for megamorph (CR §702.37b).
# ----------------------------------------------------------------------------


def face_down_characteristics(perm: Permanent) -> dict:
    """Return the §613 layer-1b characteristic override for a face-down
    permanent. Callers that want the layer-agnostic "what is this thing"
    view should use ``get_effective_characteristics`` instead — that path
    already consults this helper via the face-down continuous effect.

    CR §708.2a: default is 2/2 colorless nameless creature with no text,
    no subtypes, no mana cost. Disguise / Cloak / Cybermen override the
    abilities list with ("ward_2",) per §702.168a / §701.58a.

    Returns the plain dict the layer system expects (keys: power,
    toughness, name, type_line, types, supertypes, subtypes, colors,
    abilities). We intentionally do NOT set "controller" here — that
    stays whatever the permanent already has (turning a permanent face
    down doesn't change its controller; §708 is silent on that and
    empirical play doesn't alter control).
    """
    if perm.face_down_variant == "ward_2":
        shell = FACE_DOWN_WARD_2
    else:
        shell = FACE_DOWN_VANILLA
    return {
        "name": shell.name,  # empty string = nameless (§708.2a)
        "type_line": shell.type_line,
        "types": list(shell.types),
        "supertypes": list(shell.supertypes),
        "subtypes": list(shell.subtypes),
        "colors": list(shell.colors),
        "power": shell.power,
        "toughness": shell.toughness,
        "abilities": list(shell.abilities),
    }


def _face_down_card_shell(original: Optional[CardEntry]) -> CardEntry:
    """Build the CardEntry representation the engine attaches to a
    face-down Permanent. Keeps ``ast`` empty so normal ETB/trigger walks
    don't fire face-up abilities (CR §708.3 / §708.4 — only face-down
    characteristics are visible).

    The Permanent retains ``original_card`` separately so
    turn_face_up() can restore the true card's state.
    """
    name = "Face-Down Creature"
    if original is not None:
        # Never expose the name (§708.2a "no name"), but track internally
        # via original_card. name stays empty so characteristic-driven
        # filters ("target creature named X") correctly miss the FD perm.
        pass
    return CardEntry(
        name=name,
        mana_cost="",
        cmc=0,
        type_line="Creature",
        oracle_text="",
        power=2,
        toughness=2,
        ast=CardAST(name=name, abilities=(), parse_errors=(),
                    fully_parsed=True),
        colors=(),
    )


def _face_down_copy_effect_ce(game: Game, perm: Permanent) -> ContinuousEffect:
    """Build the §613.2b layer-1b ContinuousEffect that overrides a
    face-down permanent's copiable values with FaceDownCharacteristics.

    This is the canonical §613 implementation of CR §708.2a: "face-down
    permanents have no characteristics other than those listed by the
    ability or rules that allowed the permanent to be face down".

    The apply_fn REPLACES the entire ``chars`` dict fields — it doesn't
    merge, because a face-down permanent's copiable values ARE the
    face-down characteristics (§613.2c). Layer 7d counter modifications
    still apply AFTER this (§613.1g), so counters bump P/T as expected
    (2/2 + a +1/+1 counter = 3/3 face-down, which is how megamorph's
    counter flips the face-up reveal to 3/3 too).
    """
    ts = game.next_timestamp()
    handler_id = f"face_down_copy_effect:{id(perm)}"

    def predicate(game: Game, p: Permanent) -> bool:
        # Applies iff this IS the face-down permanent. Other perms see
        # nothing.
        return p is perm and p.face_down

    def apply_fn(game: Game, p: Permanent, chars: dict) -> None:
        shell = (FACE_DOWN_WARD_2 if p.face_down_variant == "ward_2"
                 else FACE_DOWN_VANILLA)
        # CR §708.2a — replace everything. Layer 7b/7c/7d effects then
        # apply on top (buffs, counters, set-P/T), which is correct:
        # §613.4 runs layer 7 sublayers AFTER layer 1.
        chars["name"] = shell.name
        chars["type_line"] = shell.type_line
        chars["types"] = list(shell.types)
        chars["supertypes"] = list(shell.supertypes)
        chars["subtypes"] = list(shell.subtypes)
        chars["colors"] = list(shell.colors)
        chars["power"] = shell.power
        chars["toughness"] = shell.toughness
        chars["abilities"] = list(shell.abilities)

    return ContinuousEffect(
        layer="1",               # §613.2b — face-down is layer 1b
        timestamp=ts,
        source_perm=perm,
        source_card_name=perm.card.name,
        controller_seat=perm.controller,
        predicate=predicate,
        apply_fn=apply_fn,
        duration=DURATION_PERMANENT,   # lasts until turned face up
        handler_id=handler_id,
    )


def cast_face_down(game: Game, seat_idx: int, card: CardEntry,
                   flip_cost: Optional[ManaCost] = None,
                   variant: str = "vanilla",
                   is_megamorph: bool = False) -> Permanent:
    """Put ``card`` from the seat's hand onto the battlefield face down.

    CR §702.37c: to cast a card using its morph ability, turn it face
    down and announce that you're using a morph ability. It becomes a
    2/2 face-down creature card with no text, no name, no subtypes,
    and no mana cost. Put it onto the stack, and pay {3} rather than
    its mana cost. When the spell resolves, it enters the battlefield
    with the same (face-down) characteristics.

    In the MVP engine we short-circuit stack routing and place directly
    on the battlefield face-down. The {3} alternative cost is NOT paid
    here (the calling code is expected to have already debited mana);
    callers that want judge-grade accuracy should pay {3} before
    calling.

    ``variant`` is "vanilla" for morph/megamorph, "ward_2" for
    disguise/cloak/cybermen. ``flip_cost`` is the cost to pay later in
    ``turn_face_up()`` — for morph/disguise this is the morph/disguise
    cost from the original face-up ability; for manifest it's the
    original mana cost iff the card is a creature (CR §701.40a).
    """
    # Guarded: callers pass a card that should be in hand. Tolerate
    # the rare test-harness case where a card isn't strictly in hand
    # (unit tests construct CardEntry outside the deck flow).
    seat = game.seats[seat_idx]
    if card in seat.hand:
        seat.hand.remove(card)
    perm = Permanent(
        card=_face_down_card_shell(card),
        controller=seat_idx,
        tapped=False,
        summoning_sick=True,
        face_down=True,
        original_card=card,
        face_down_variant=variant,
        turn_face_up_cost=flip_cost,
        flip_cost_is_megamorph=is_megamorph,
    )
    _etb_initialize(game, perm)
    # Register the §613.2b layer-1b face-down continuous effect BEFORE
    # appending to battlefield so any ETB trigger walker that queries
    # get_effective_characteristics() sees the correct (face-down)
    # characteristics.
    register_continuous_effect(game, _face_down_copy_effect_ce(game, perm))
    seat.battlefield.append(perm)
    game.ev("cast_face_down",
            seat=seat_idx,
            original_card=card.name,
            variant=variant,
            is_megamorph=is_megamorph,
            rule="702.37c" if variant == "vanilla" else "702.168a")
    return perm


def manifest(game: Game, seat_idx: int,
             count: int = 1) -> list[Permanent]:
    """Move the top ``count`` cards of ``seat.library`` onto the
    battlefield face down as 2/2 face-down creatures (CR §701.40a).

    Per §701.40e, multiple manifests from a single library happen one
    at a time. We emit a separate event per card for auditability.

    If the top card is a creature card, the resulting face-down
    permanent CAN be turned face up later for its ORIGINAL MANA COST
    (CR §701.40a / §708.7 — the "turn face up" procedure comes from
    the manifest rule itself). We record the mana cost on
    ``turn_face_up_cost`` for creatures and leave it None for
    noncreatures (noncreatures can't be turned face up by manifest).

    Returns the list of created face-down Permanents.
    """
    seat = game.seats[seat_idx]
    created: list[Permanent] = []
    for _ in range(count):
        if not seat.library:
            game.ev("manifest_failed_empty_library",
                    seat=seat_idx, rule="701.40a")
            break
        card = seat.library.pop(0)
        # CR §701.40a — creature cards can be flipped for their mana
        # cost. Non-creatures stay face-down forever unless an outside
        # effect flips them.
        type_line = (card.type_line or "").lower()
        is_creature_card = "creature" in type_line
        flip_cost = None
        if is_creature_card:
            flip_cost = _parse_mana_cost_str(card.mana_cost)
        perm = Permanent(
            card=_face_down_card_shell(card),
            controller=seat_idx,
            tapped=False,
            summoning_sick=True,
            face_down=True,
            original_card=card,
            face_down_variant="vanilla",
            turn_face_up_cost=flip_cost,
            flip_cost_is_megamorph=False,
        )
        _etb_initialize(game, perm)
        register_continuous_effect(game,
                                   _face_down_copy_effect_ce(game, perm))
        seat.battlefield.append(perm)
        created.append(perm)
        game.ev("manifest_done",
                seat=seat_idx,
                original_name=card.name,
                is_creature=is_creature_card,
                rule="701.40a")
    return created


def cloak(game: Game, seat_idx: int,
          count: int = 1) -> list[Permanent]:
    """Cloak the top ``count`` cards of ``seat.library`` — manifest-like
    but the face-down form has ward {2} (CR §701.58a).

    Returns the list of created face-down Permanents.
    """
    seat = game.seats[seat_idx]
    created: list[Permanent] = []
    for _ in range(count):
        if not seat.library:
            game.ev("cloak_failed_empty_library",
                    seat=seat_idx, rule="701.58a")
            break
        card = seat.library.pop(0)
        type_line = (card.type_line or "").lower()
        is_creature_card = "creature" in type_line
        flip_cost = None
        if is_creature_card:
            flip_cost = _parse_mana_cost_str(card.mana_cost)
        perm = Permanent(
            card=_face_down_card_shell(card),
            controller=seat_idx,
            tapped=False,
            summoning_sick=True,
            face_down=True,
            original_card=card,
            face_down_variant="ward_2",
            turn_face_up_cost=flip_cost,
            flip_cost_is_megamorph=False,
        )
        _etb_initialize(game, perm)
        register_continuous_effect(game,
                                   _face_down_copy_effect_ce(game, perm))
        seat.battlefield.append(perm)
        created.append(perm)
        game.ev("cloak_done",
                seat=seat_idx,
                original_name=card.name,
                is_creature=is_creature_card,
                rule="701.58a")
    return created


def manifest_dread(game: Game, seat_idx: int) -> Optional[Permanent]:
    """CR §701.62a: look at top 2 cards, manifest 1, put the other in
    graveyard.

    The choice of WHICH card to manifest routes through the seat's
    policy (``seat.policy.choose_manifest_dread(game, cards)``). The
    default greedy policy picks the first creature card if present
    (since only creatures can be turned face up later), else the
    first card.

    Returns the manifested Permanent (or None if the library has
    fewer than 2 cards — in which case we manifest what's available
    per §701.62a's "look at" language).
    """
    seat = game.seats[seat_idx]
    # Peek up to 2 cards
    n_peek = min(2, len(seat.library))
    if n_peek == 0:
        game.ev("manifest_dread_failed_empty_library",
                seat=seat_idx, rule="701.62a")
        return None
    peek = seat.library[:n_peek]
    # Pick which to manifest
    policy = getattr(seat, "policy", None)
    chosen_idx = None
    if policy is not None and hasattr(policy, "choose_manifest_dread"):
        try:
            chosen_idx = policy.choose_manifest_dread(game, peek)
        except Exception:
            chosen_idx = None
    if chosen_idx is None or not (0 <= chosen_idx < n_peek):
        # Heuristic: prefer a creature card (can flip) over a non-creature.
        for i, c in enumerate(peek):
            if "creature" in (c.type_line or "").lower():
                chosen_idx = i
                break
        if chosen_idx is None:
            chosen_idx = 0
    # Pull the chosen card out of library, manifest it.
    chosen_card = seat.library.pop(chosen_idx)
    # Build the face-down perm — effectively the core of manifest() but
    # bypassing the "pop from top" because we've already chosen which.
    type_line = (chosen_card.type_line or "").lower()
    is_creature_card = "creature" in type_line
    flip_cost = None
    if is_creature_card:
        flip_cost = _parse_mana_cost_str(chosen_card.mana_cost)
    perm = Permanent(
        card=_face_down_card_shell(chosen_card),
        controller=seat_idx,
        tapped=False,
        summoning_sick=True,
        face_down=True,
        original_card=chosen_card,
        face_down_variant="vanilla",
        turn_face_up_cost=flip_cost,
        flip_cost_is_megamorph=False,
    )
    _etb_initialize(game, perm)
    register_continuous_effect(game,
                               _face_down_copy_effect_ce(game, perm))
    seat.battlefield.append(perm)
    game.ev("manifest_dread_manifest",
            seat=seat_idx,
            original_name=chosen_card.name,
            is_creature=is_creature_card,
            rule="701.62a")
    # Mill the rest (CR §701.62a — "put the cards you looked at that
    # were not manifested this way into your graveyard").
    # Indices of the un-chosen peek relative to ORIGINAL order. After we
    # popped ``chosen_idx``, library's [0:n_peek-1] slice contains the
    # rest in their original order.
    rest_count = n_peek - 1
    for _ in range(rest_count):
        if not seat.library:
            break
        discarded = seat.library.pop(0)
        seat.graveyard.append(discarded)
        game.ev("manifest_dread_mill",
                seat=seat_idx,
                card=discarded.name,
                rule="701.62a")
    return perm


def _parse_mana_cost_str(mana_cost_str: str) -> Optional[ManaCost]:
    """Parse a raw mana-cost string like "{1}{G}{G}" into a ManaCost.
    Returns None if the string is empty or unparseable. Mirrors
    parser.parse_mana_cost for use inside playloop.py.
    """
    if not mana_cost_str:
        return None
    import re as _re
    syms: list = []
    for m in _re.finditer(r"\{([^}]+)\}", mana_cost_str):
        raw = "{" + m.group(1) + "}"
        body = m.group(1).strip().upper()
        if body in {"T", "Q", "E"}:
            continue
        from mtg_ast import ManaSymbol as _MS
        sym = _MS(raw=raw)
        if body.isdigit():
            sym = _MS(raw=raw, generic=int(body))
        elif body == "X":
            sym = _MS(raw=raw, is_x=True)
        elif body == "S":
            sym = _MS(raw=raw, is_snow=True)
        elif "/" in body:
            parts = body.split("/")
            colors = tuple(p for p in parts
                           if p in {"W", "U", "B", "R", "G", "C"})
            generic = next((int(p) for p in parts if p.isdigit()), 0)
            phyrexian = "P" in parts
            sym = _MS(raw=raw, generic=generic, color=colors,
                      is_phyrexian=phyrexian)
        elif body in {"W", "U", "B", "R", "G", "C"}:
            sym = _MS(raw=raw, color=(body,))
        syms.append(sym)
    return ManaCost(symbols=tuple(syms)) if syms else None


def turn_face_up(game: Game, perm: Permanent,
                 paying: bool = True) -> bool:
    """Turn ``perm`` face up. SPECIAL ACTION (CR §116.2g / §702.37e /
    §702.168d / §708.7) — doesn't use the stack.

    Returns True iff the flip succeeded. Fails if:
      - The permanent isn't face down (CR §708.2b: can't turn a face-
        down face down; symmetric read — can't re-flip a face-up).
      - ``paying=True`` and the permanent has no flip cost (CR §702.37e:
        "If the permanent wouldn't have a morph cost if it were face
        up, it can't be turned face up this way"). For manifested
        non-creatures we correctly leave turn_face_up_cost=None.

    On success:
      - Removes the §613.2b face-down continuous effect.
      - Clears ``perm.face_down`` and restores the original card entry.
      - Places a +1/+1 counter for megamorph (CR §702.37b).
      - Updates the timestamp (CR §613.7f: "A permanent receives a new
        timestamp each time it turns face up or face down").
      - Fires a ``turned_face_up`` event so triggered abilities like
        "when this creature is turned face up, ..." (Ainok Survivalist
        et al.) can fire via the trigger dispatcher.
      - Does NOT fire ETB triggers (CR §708.8 — the permanent has
        already entered the battlefield).
    """
    if not perm.face_down:
        game.ev("turn_face_up_failed_already_face_up",
                card=getattr(perm.original_card, "name",
                             perm.card.name),
                rule="708.2b")
        return False
    if paying and perm.turn_face_up_cost is None:
        # No printed flip cost — CR §702.37e says can't flip this way.
        # Non-creatures that were manifested fall in this bucket.
        game.ev("turn_face_up_failed_no_flip_cost",
                seat=perm.controller,
                rule="702.37e")
        return False

    # Pay the flip cost. MVP engine uses generic mana_pool only, so we
    # just debit the CMC. Judge-grade mana handling is out of scope.
    if paying and perm.turn_face_up_cost is not None:
        cost_cmc = perm.turn_face_up_cost.cmc
        seat = game.seats[perm.controller]
        if seat.mana_pool < cost_cmc:
            # Not enough mana — the flip fails. No state change.
            game.ev("turn_face_up_failed_insufficient_mana",
                    seat=perm.controller,
                    needed=cost_cmc,
                    available=seat.mana_pool,
                    rule="702.37e")
            return False
        seat.mana_pool -= cost_cmc

    original_name = (perm.original_card.name if perm.original_card
                     else perm.card.name)

    # Remove the face-down continuous effect (§708.8 — face-down
    # effect ends when the permanent is turned face up).
    removed = 0
    kept: list[ContinuousEffect] = []
    for ce in game.continuous_effects:
        if (ce.source_perm is perm
                and ce.handler_id.startswith("face_down_copy_effect:")):
            removed += 1
            continue
        kept.append(ce)
    if removed:
        game.continuous_effects = kept
        game.invalidate_characteristics_cache()

    # Restore the original card's copiable values by swapping the
    # shell card with the original. Counters / attachments / damage
    # marked all persist (CR §708.8: "Any effects that have been
    # applied to the face-down permanent still apply").
    was_megamorph = perm.flip_cost_is_megamorph
    if perm.original_card is not None:
        perm.card = perm.original_card
    perm.face_down = False
    perm.original_card = None
    perm.face_down_variant = "vanilla"
    perm.turn_face_up_cost = None
    perm.flip_cost_is_megamorph = False

    # Refresh timestamp (CR §613.7f).
    perm.timestamp = game.next_timestamp()

    # Megamorph places a +1/+1 counter (CR §702.37b).
    if was_megamorph:
        perm.counters["+1/+1"] = perm.counters.get("+1/+1", 0) + 1
        game.ev("megamorph_counter_placed",
                card=original_name,
                seat=perm.controller,
                rule="702.37b")

    # Register face-up replacements (the true card may have regen /
    # win-the-game replacements that were dormant while face down).
    register_replacements_for_permanent(game, perm)

    # Emit the turned_face_up event so "when this creature is turned
    # face up, ..." triggers fire. The trigger dispatcher hooks this
    # event name via the existing parse_trigger EVENT_VERBS entry
    # "is turned face up" → "turned_face_up".
    try:
        fire_event(game, Event(
            type="turned_face_up",
            player=perm.controller,
            target=perm,
            kwargs={"was_megamorph": was_megamorph},
        ))
    except Exception:
        # fire_event should never raise, but defend against it.
        pass

    # Dispatch any ``turned_face_up`` Triggered abilities on the card
    # itself. The base AST walks Triggered abilities at ETB/dies/etc.
    # but the turned_face_up path is currently not wired; iterate the
    # card's abilities manually so Ainok Survivalist et al. fire.
    for ab in perm.card.ast.abilities:
        if isinstance(ab, Triggered) and ab.trigger.event == "turned_face_up":
            # Queue the effect via the standard effect resolver. We use
            # fire_event's default queue path if present; otherwise emit
            # a log line and rely on the resolver to consume it.
            game.ev("turned_face_up_trigger_fires",
                    card=original_name,
                    seat=perm.controller,
                    rule="603.2")
            # Runtime effect resolution — dispatch through the existing
            # effect resolver if available.
            try:
                resolver = globals().get("_resolve_effect")
                if resolver is not None:
                    resolver(game, perm.controller, ab.effect, perm)
            except Exception as exc:
                game.ev("turned_face_up_resolver_error",
                        card=original_name,
                        exception=f"{type(exc).__name__}: {exc}")

    game.ev("turned_face_up",
            card=original_name,
            seat=perm.controller,
            megamorph=was_megamorph,
            rule="702.37e" if paying else "708.7")
    return True


def _etb_initialize(game: Game, perm: Permanent) -> None:
    """Set timestamp + initial counters when a Permanent enters the battlefield.

    This is the single entry point for "new permanent on battlefield" state.
    Call it right after appending a Permanent to a Seat.battlefield list.

    CR 613.7 / 603.2: every object on the battlefield has a timestamp. We
    assign a monotonic stamp so Legend Rule (CR 704.5j) and World Rule
    (CR 704.5k) can pick a survivor deterministically.

    CR 306.5b: a planeswalker enters with a number of loyalty counters equal
    to the number in the lower-right corner (the starting loyalty).

    CR 310.3: a battle enters with the number of defense counters specified.
    """
    if perm.timestamp == 0:
        perm.timestamp = game.next_timestamp()
    # Initialize loyalty counters for planeswalkers (CR 306.5b).
    if perm.is_planeswalker and "loyalty" not in perm.counters:
        n = perm.card.starting_loyalty
        if n is None:
            # Fallback: derive from CMC when Scryfall data was missing.
            n = max(1, perm.card.cmc or 3)
        perm.counters["loyalty"] = n
    # Initialize defense counters for battles (CR 310.3).
    if perm.is_battle and "defense" not in perm.counters:
        n = perm.card.starting_defense
        if n is None:
            n = max(1, perm.card.cmc or 1)
        perm.counters["defense"] = n
    # §614 replacement-effect registration. Any card with a hardcoded
    # handler list (_REPLACEMENT_REGISTRY_BY_NAME) has its replacements
    # activated here. De-duped inside register_replacement_effect.
    register_replacements_for_permanent(game, perm)
    # CR §726.2 — first time a daybound/nightbound permanent enters the
    # battlefield, the game becomes day. _maybe_become_day short-circuits
    # when the state isn't "neither" (idempotent on repeated ETBs).
    card = perm.card
    if card and getattr(card, "is_dfc", False):
        # Fast path: only worth scanning if this card itself is a DFC.
        # Non-DFC cards can't carry daybound/nightbound.
        _maybe_become_day(game, reason="daybound_etb")


# ---------------------------------------------------------------------------
# State-based actions (CR 704.5)
# ---------------------------------------------------------------------------
# Each SBA helper returns True when it mutated state. `state_based_actions`
# runs the full suite, then repeats while any helper reported a change
# (CR 704.3: all applicable SBAs are performed simultaneously as a single
# event, and the check repeats until none apply). Every mutation emits a
# `sba_704_5X` event citing the exact CR sub-section so downstream tooling
# (rule auditor, replay viewer) can trace SBA firings.
#
# Per-check helpers are intentionally granular so a future judge-mode can
# turn individual rules off for debugging.

def do_permanent_die(game: Game, perm: Permanent, reason: str) -> None:
    """Route a "this permanent would die" event through §614.

    A creature-death event may be replaced by:
      - Anafenza, the Foremost (exile opponent creature instead of die)
      - Rest in Peace / Leyline of the Void (exile instead of graveyard)
      - Regeneration (not yet implemented; CR 614.8)

    If the event is cancelled (e.g. a card we haven't implemented that
    fully prevents death), the permanent stays on the battlefield.

    If the destination_zone was redirected to "exile", the permanent
    goes to exile instead of the graveyard. Otherwise it goes to the
    owner's graveyard as usual.

    CR 108.3 — a card's OWNER is the player who started the game with
    it in their deck. When a permanent that changed controllers (e.g.
    Gilded Drake swap) leaves the battlefield, it goes to its owner's
    zone, not the current controller's. `perm.owner_seat` resolves the
    override if set; otherwise falls back to controller.
    """
    # Controller removes the card from their battlefield; card goes
    # to OWNER's graveyard / exile per §108.3.
    controller_seat = game.seats[perm.controller]
    owner_idx = perm.owner_seat
    owner = game.seats[owner_idx]
    if perm not in controller_seat.battlefield:
        return  # already gone; idempotent safety net
    # Fire would_die first. A handler may redirect to exile, or cancel.
    die_event = Event(
        type="would_die",
        player=perm.controller,
        target=perm,
        kwargs={"reason": reason,
                "destination_zone": "graveyard",
                "is_token": perm.is_token,
                "owner_seat": owner_idx},
    )
    fire_event(game, die_event)
    if die_event.cancelled:
        game.ev("die_replaced", card=perm.card.name,
                seat=perm.controller, cancelled_by=die_event.get("cancelled_by"),
                rule="614")
        return
    # If no redirect happened, ALSO fire would_be_put_into_graveyard
    # (covers Rest in Peace / Leyline of the Void — these don't watch
    # "would_die", they watch the graveyard-zone-entry event).
    destination = die_event.get("destination_zone", "graveyard")
    if destination == "graveyard" and not perm.is_token:
        gy_event = Event(
            type="would_be_put_into_graveyard",
            player=perm.controller,
            target=perm,
            kwargs={"from_zone": "battlefield",
                    "owner_seat": owner_idx,
                    "is_token": perm.is_token,
                    "destination_zone": "graveyard"},
        )
        fire_event(game, gy_event)
        if gy_event.cancelled:
            game.ev("gy_replaced", card=perm.card.name,
                    seat=perm.controller, rule="614")
            return
        destination = gy_event.get("destination_zone", "graveyard")
    # Execute the residual zone change.
    unregister_replacements_for_permanent(game, perm)
    # §613: any continuous effects sourced by this permanent stop applying.
    unregister_continuous_effects_for_permanent(game, perm)
    # Modifiers targeting this perm with DURATION_UNTIL_SOURCE_LEAVES
    # (Mark of Asylum etc.) expire; they're swept automatically on the
    # next scan_expired_durations() pass because _duration_expires_now
    # keys on source_perm.
    controller_seat.battlefield.remove(perm)
    # CR §708.4 / §708.11: a face-down permanent leaving the battlefield
    # is revealed (the card goes to its destination zone face-up and
    # resumes its normal copiable values). We put the ORIGINAL card, not
    # the _face_down_card_shell, into graveyard/exile.
    card_to_place = perm.original_card if perm.face_down else perm.card
    reveal_name = card_to_place.name if card_to_place else perm.card.name
    if perm.is_token:
        # Tokens cease to exist — SBA 704.5d will sweep them from any
        # zone they land in.
        pass
    elif destination == "exile":
        owner.exile.append(card_to_place)
        game.ev("gy_redirected_to_exile", card=reveal_name,
                seat=perm.controller, owner_seat=owner_idx,
                reason=reason, rule="614",
                was_face_down=perm.face_down)
    else:
        owner.graveyard.append(card_to_place)
    game.emit(f"  {reveal_name} dies → {destination} ({reason})")
    game.ev("destroy", card=reveal_name, seat=perm.controller,
            owner_seat=owner_idx,
            reason=reason, destination=destination,
            was_face_down=perm.face_down)
    # Clear attach references.
    for s in game.seats:
        for other in s.battlefield:
            if other.attached_to is perm:
                other.attached_to = None
    # Per-card LTB hooks.
    try:
        apply_per_card_ltb(game, perm)
    except Exception as exc:
        game.ev("per_card_ltb_crashed",
                card=perm.card.name,
                exception=f"{type(exc).__name__}: {exc}")


def do_put_counter(game: Game, target, counter_kind: str,
                   count: int) -> int:
    """Route a "put N counters on target" through §614 would_put_counter.

    Returns the actual number of counters placed (may be 0 if cancelled,
    or > N if doubled). Directly mutates target.counters or, for +1/+1,
    target.buffs_pt for back-compat with the legacy P/T math.
    """
    if target is None or count <= 0:
        return 0
    event = Event(
        type="would_put_counter",
        target=target,
        player=getattr(target, "controller", None),
        kwargs={"count": count, "counter_kind": counter_kind},
    )
    fire_event(game, event)
    if event.cancelled:
        game.ev("counter_cancelled", count=count,
                counter_kind=counter_kind,
                target_card=getattr(getattr(target, "card", None),
                                    "name", "?"),
                rule="614")
        return 0
    final = max(0, event.get("count", count))
    if final == 0:
        return 0
    if counter_kind == "+1/+1":
        # Historically we represented +1/+1 counter anthems via buffs_pt;
        # keep that for tokens / non-creature targets, but also populate
        # counters so SBA 704.5q (±1/±1 annihilation) and Hardened
        # Scales-like effects see it.
        target.counters["+1/+1"] = target.counters.get("+1/+1", 0) + final
    else:
        target.counters[counter_kind] = target.counters.get(counter_kind, 0) + final
    name = getattr(getattr(target, "card", None), "name", "?")
    game.emit(f"  {final}x {counter_kind} counter on {name}")
    game.ev("counter_placed", counter_kind=counter_kind,
            count=final, original_count=count,
            target_card=name, rule="614")
    return final


def _destroy_perm(game: Game, perm: Permanent, reason: str,
                  rule: str) -> None:
    """Move `perm` from its controller's battlefield to its owner's graveyard
    and emit a sba_* event. Idempotent if the perm isn't on the battlefield.

    §614 note: this is called by SBA 704.5f/g/i/t/v for dying permanents,
    and by legend/world rule SBAs. For creature-death paths we route
    through `do_permanent_die` so Anafenza/Rest in Peace/etc. can
    intercept. For non-creature "destroy" SBAs (battle defense 0, saga
    final chapter → sacrifice, legend rule, world rule) we follow the
    original direct-removal path because §614 regen/exile replacements
    don't apply to these.
    """
    seat = game.seats[perm.controller]
    if perm not in seat.battlefield:
        return  # idempotent
    # Emit the sba_* event FIRST so the auditor always sees the intent,
    # even when a replacement later redirects to exile.
    game.ev("sba_" + rule.replace(".", "_"),
            card=perm.card.name, seat=perm.controller,
            rule=rule, reason=reason)
    # Route creature-death SBAs (704.5f toughness≤0, 704.5g lethal
    # damage, 704.5h deathtouch) through the §614 chain.
    if perm.is_creature and rule in ("704.5f", "704.5g", "704.5h"):
        do_permanent_die(game, perm, reason=reason)
        return
    # Non-creature / non-death direct removal (legend rule, world rule,
    # battle SBAs, planeswalker loyalty 0): mirror the original
    # semantics without routing through would_die, since regen-class
    # replacements don't apply to these.
    seat.battlefield.remove(perm)
    if not perm.is_token:
        # Rest in Peace / Leyline of the Void STILL apply here
        # (graveyard-zone-entry) for non-creature-death SBAs like
        # planeswalker loyalty 0 or saga final chapter.
        gy_event = Event(
            type="would_be_put_into_graveyard",
            player=perm.controller,
            target=perm,
            kwargs={"from_zone": "battlefield",
                    "owner_seat": perm.controller,
                    "is_token": False,
                    "destination_zone": "graveyard"},
        )
        fire_event(game, gy_event)
        if gy_event.cancelled:
            # Extremely rare path — some hypothetical replacement
            # cancels the zone change entirely. Put it back.
            seat.battlefield.append(perm)
            return
        destination = gy_event.get("destination_zone", "graveyard")
        # CR §708.4 — face-down permanent leaving the battlefield is
        # revealed and resumes face-up characteristics. Place the ORIGINAL
        # card, not the face-down shell, into the destination zone.
        card_to_place = (perm.original_card
                         if perm.face_down and perm.original_card is not None
                         else perm.card)
        if destination == "exile":
            seat.exile.append(card_to_place)
            game.ev("gy_redirected_to_exile", card=card_to_place.name,
                    seat=perm.controller, reason=reason, rule="614",
                    was_face_down=perm.face_down)
        else:
            seat.graveyard.append(card_to_place)
    # Clear attach references to this perm so 704.5n/704.5p don't mis-fire.
    for s in game.seats:
        for other in s.battlefield:
            if other.attached_to is perm:
                other.attached_to = None
    # §614: a permanent leaving the battlefield unregisters any
    # replacement effects it was generating.
    unregister_replacements_for_permanent(game, perm)
    # §613: any continuous effects sourced by this permanent stop.
    unregister_continuous_effects_for_permanent(game, perm)
    # Per-card LTB hooks (Worldgorger Dragon return-from-exile, Animate
    # Dead sacrifice-the-bound-creature, etc.). Fires AFTER the
    # permanent has been moved to the graveyard so downstream handlers
    # see the current zone layout.
    try:
        apply_per_card_ltb(game, perm)
    except Exception as exc:
        game.ev("per_card_ltb_crashed",
                card=perm.card.name,
                exception=f"{type(exc).__name__}: {exc}")


def _sba_704_5a(game: Game) -> bool:
    """CR 704.5a — If a player has 0 or less life, that player loses the game.

    Note: several non-SBA callsites (damage resolver, lose_life) eagerly set
    `seat.lost = True` on life<=0 and call `check_end` immediately. That's
    fine for game-end correctness, but skips the SBA-event audit trail. We
    paper over the gap by emitting the formal 704.5a event once per seat
    when we first observe a lost-to-life state.
    """
    changed = False
    for s in game.seats:
        if s.life <= 0 and not getattr(s, "_sba_704_5a_emitted", False):
            # Mark emission so we don't spam the event stream each pass.
            s._sba_704_5a_emitted = True  # type: ignore[attr-defined]
            game.ev("sba_704_5a", seat=s.idx, rule="704.5a",
                    reason="life_total_zero_or_less", life=s.life,
                    already_lost=s.lost)
            if not s.lost:
                s.lost = True
                s.loss_reason = "life total 0 or less (CR 704.5a)"
                changed = True
    return changed


def _sba_704_5b(game: Game) -> bool:
    """CR 704.5b — If a player attempted to draw from an empty library since
    the last SBA check, that player loses."""
    changed = False
    for s in game.seats:
        if not s.attempted_draw_from_empty_library:
            continue
        # Emit the formal SBA observation event even when `draw_cards` already
        # flagged `lost=True` — downstream auditors want a 704.5b trace.
        game.ev("sba_704_5b", seat=s.idx, rule="704.5b",
                reason="draw_from_empty_library",
                already_lost=s.lost)
        if not s.lost:
            s.lost = True
            s.loss_reason = "drew from empty library (CR 704.5b)"
        s.attempted_draw_from_empty_library = False
        changed = True
    return changed


def _sba_704_5c(game: Game) -> bool:
    """CR 704.5c — If a player has ten or more poison counters, that player
    loses. (No infect cards in the 4-deck tournament, but the check is here
    for correctness.)"""
    changed = False
    for s in game.seats:
        if s.lost:
            continue
        if s.poison_counters >= 10:
            s.lost = True
            s.loss_reason = "ten or more poison counters (CR 704.5c)"
            game.ev("sba_704_5c", seat=s.idx, rule="704.5c",
                    reason="poison_counters", poison=s.poison_counters)
            changed = True
    return changed


def _sba_704_5d(game: Game) -> bool:
    """CR 704.5d — A token in a zone other than the battlefield ceases to
    exist.

    In this engine tokens on the battlefield are Permanent instances with a
    "Token" type-line prefix; tokens never enter hand/graveyard/exile — the
    zone-transition sites drop them directly instead of adding a CardEntry
    to the new zone. This SBA is implemented as a clean-up sweep: any token
    CardEntry we find in a non-battlefield zone is removed.
    """
    changed = False
    for s in game.seats:
        for zone_name in ("hand", "graveyard", "exile", "library"):
            zone = getattr(s, zone_name)
            kept = []
            removed = 0
            for c in zone:
                tl = (c.type_line or "").lower()
                is_token_card = tl.startswith("token ") or (
                    "token" in tl.split("—")[0] and "—" in tl)
                if is_token_card:
                    removed += 1
                    changed = True
                else:
                    kept.append(c)
            if removed:
                setattr(s, zone_name, kept)
                game.ev("sba_704_5d", seat=s.idx, rule="704.5d",
                        reason="token_in_nonbattlefield_zone",
                        zone=zone_name, count=removed)
    return changed


def _sba_704_5e(game: Game) -> bool:
    """CR 704.5e — A copy of a spell in any zone other than the stack ceases
    to exist. A copy of a card in any zone other than stack/battlefield
    ceases to exist.

    This engine does not yet create spell-copies via Fork/Twinflame/etc.
    (no resolver sets `is_copy`). Placeholder for when copy-tracking lands.
    """
    # TODO: implement when the stack starts tracking spell copies. See
    # interaction_harness_infinites for combos (Doubling Season ult drawing)
    # that would eventually create spell/card copies.
    return False


def _sba_704_5f(game: Game) -> bool:
    """CR 704.5f — If a creature has toughness 0 or less, it's put into its
    owner's graveyard. Regeneration can't replace this event."""
    changed = False
    for s in game.seats:
        for p in list(s.battlefield):
            if p.is_creature and p.toughness <= 0:
                _destroy_perm(game, p, reason="toughness_zero_or_less",
                              rule="704.5f")
                changed = True
    return changed


def _sba_704_5g(game: Game) -> bool:
    """CR 704.5g — A creature with toughness > 0, damage marked ≥ toughness
    has been dealt lethal damage and is destroyed. Regeneration CAN replace.
    Indestructible creatures are not destroyed (per rule 702.12b)."""
    changed = False
    for s in game.seats:
        for p in list(s.battlefield):
            if not p.is_creature:
                continue
            t = p.toughness
            if t > 0 and p.damage_marked >= t:
                if is_indestructible(p):
                    continue  # indestructible: lethal damage does nothing
                _destroy_perm(game, p, reason="lethal_damage",
                              rule="704.5g")
                changed = True
    return changed


def _sba_704_5h(game: Game) -> bool:
    """CR 704.5h — A creature dealt damage by a deathtouch source since the
    last SBA check is destroyed. Engine doesn't track deathtouch-damage
    flags per permanent; lethal-damage handling in 704.5g is a strict
    superset for toughness-based kills, so this SBA only fires when
    deathtouch damage < toughness. Stubbed with TODO for full fidelity.
    """
    # TODO: add `deathtouch_damaged: bool` to Permanent, set in damage
    # resolver when source has deathtouch. Current engine's 704.5g covers
    # ≥toughness case which subsumes deathtouch-lethal in most scenarios.
    return False


def _sba_704_5i(game: Game) -> bool:
    """CR 704.5i — A planeswalker with 0 loyalty is put into its owner's
    graveyard.

    Lazy-init safety net: harness-placed planeswalkers may not have gone
    through `_etb_initialize`, so they lack a "loyalty" key. We treat a
    missing key as "hasn't had loyalty assigned yet" and initialize
    instead of instant-killing (which would break the Jace-Wielder combo
    harness that pre-places Jace and then activates -8 directly without
    paying loyalty costs).
    """
    changed = False
    for s in game.seats:
        for p in list(s.battlefield):
            if not p.is_planeswalker:
                continue
            if "loyalty" not in p.counters:
                # First time we've seen this planeswalker via SBA; assign
                # starting loyalty per CR 306.5b and skip the death check
                # for this pass.
                n = p.card.starting_loyalty
                if n is None:
                    n = max(1, p.card.cmc or 3)
                p.counters["loyalty"] = n
                continue
            if p.counters["loyalty"] <= 0:
                _destroy_perm(game, p, reason="zero_loyalty",
                              rule="704.5i")
                changed = True
    return changed


def _sba_704_5j(game: Game) -> bool:
    """CR 704.5j — Legend Rule. If two or more legendary permanents with the
    same name are controlled by the same player, that player chooses one to
    keep; the rest are put into their owners' graveyards.

    Engine policy: the keeper is the one with the LOWEST timestamp
    (first-cast stays). This is a deterministic tie-break; a real judge
    asks the player, but it's always a legal choice.
    """
    changed = False
    for s in game.seats:
        # Group controller's legendaries by name.
        groups: dict[str, list[Permanent]] = {}
        for p in s.battlefield:
            if p.is_legendary:
                groups.setdefault(p.card.name, []).append(p)
        for name, perms in groups.items():
            if len(perms) < 2:
                continue
            # Keep the earliest timestamp; destroy the rest.
            perms.sort(key=lambda q: q.timestamp)
            keeper = perms[0]
            for p in perms[1:]:
                _destroy_perm(game, p, reason="legend_rule",
                              rule="704.5j")
                changed = True
            game.ev("sba_704_5j_keep", seat=s.idx, rule="704.5j",
                    card=name, keeper_timestamp=keeper.timestamp,
                    destroyed_count=len(perms) - 1)
    return changed


def _sba_704_5k(game: Game) -> bool:
    """CR 704.5k — World Rule. If two or more permanents have the world
    supertype, all except the one with the shortest time as 'world' are put
    into their owners' graveyards. Ties: all are destroyed.

    Engine policy: since every world permanent gained the supertype when it
    ETB'd (we don't yet track gain-type effects), "shortest time as world"
    equals "most recent ETB" = highest timestamp. The youngest survives,
    older worlds die.
    """
    changed = False
    worlds: list[Permanent] = []
    for s in game.seats:
        for p in s.battlefield:
            if p.is_world:
                worlds.append(p)
    if len(worlds) < 2:
        return False
    worlds.sort(key=lambda q: q.timestamp)
    # Ties on shortest-time (highest timestamp): all die.
    ts = [w.timestamp for w in worlds]
    newest = max(ts)
    newest_count = ts.count(newest)
    if newest_count > 1:
        # Ambiguous newest — all worlds die.
        for p in worlds:
            _destroy_perm(game, p, reason="world_rule_tie",
                          rule="704.5k")
            changed = True
    else:
        # Keep newest, kill the rest.
        for p in worlds:
            if p.timestamp != newest:
                _destroy_perm(game, p, reason="world_rule",
                              rule="704.5k")
                changed = True
    return changed


def _aura_attach_is_legal(perm: Permanent) -> bool:
    """Minimal legality check for auras. Current engine doesn't track enchant-
    type restrictions (e.g. enchant creature vs enchant artifact); we accept
    any attach that points at an object still on the battlefield. Flag as
    illegal when attached_to is None or to a permanent not on its
    controller's battlefield."""
    if perm.attached_to is None:
        return False
    for s in (None,):  # dummy so the scope reads right
        pass
    # The target must be on some seat's battlefield.
    return True  # conservative: only detect "not attached" as illegal


def _sba_704_5m(game: Game) -> bool:
    """CR 704.5m — An Aura attached to an illegal object/player, or not
    attached at all, is put into its owner's graveyard.

    Engine scope: we treat an Aura as illegally-attached only if its
    `attached_to` is None AND the Aura is on the battlefield. This catches
    the common case (Worldgorger / Humility interactions that exile or
    morph the attached creature). A full enchant-type legality check
    requires parsing Enchant rulings per-card.
    """
    changed = False
    for s in game.seats:
        for p in list(s.battlefield):
            if p.is_aura and p.attached_to is None:
                # Only fire if we've started tracking attachment at all. If
                # the engine never assigned attached_to, we'd wipe every
                # aura the moment it ETBs — not what we want. Gate on a
                # sentinel: we only treat None as illegal when the perm has
                # a flag indicating an attach WAS expected. For now this
                # is a conservative no-op: stubbed until attach hooks are
                # wired into the resolver by the sibling agent.
                # TODO: when resolve_aura_target() is added, flip this on.
                continue
    return changed


def _sba_704_5n(game: Game) -> bool:
    """CR 704.5n — An Equipment or Fortification attached to an illegal
    permanent or to a player becomes unattached (stays on the battlefield).
    """
    changed = False
    for s in game.seats:
        for p in s.battlefield:
            if not (p.is_equipment or p.is_fortification):
                continue
            if p.attached_to is None:
                continue
            tgt = p.attached_to
            owner_bf = game.seats[tgt.controller].battlefield
            if tgt not in owner_bf:
                # Attached target no longer on battlefield: unattach.
                p.attached_to = None
                game.ev("sba_704_5n", seat=p.controller, rule="704.5n",
                        card=p.card.name,
                        reason="equipment_target_gone")
                changed = True
                continue
            # Equipment must attach to a creature (CR 301.5); fortifications
            # to a land (CR 301.5a). Flag the illegal attach.
            if p.is_equipment and not tgt.is_creature:
                p.attached_to = None
                game.ev("sba_704_5n", seat=p.controller, rule="704.5n",
                        card=p.card.name,
                        reason="equipment_non_creature")
                changed = True
            elif p.is_fortification and not tgt.is_land:
                p.attached_to = None
                game.ev("sba_704_5n", seat=p.controller, rule="704.5n",
                        card=p.card.name,
                        reason="fortification_non_land")
                changed = True
    return changed


def _sba_704_5p(game: Game) -> bool:
    """CR 704.5p — If a battle or creature is attached to an object or
    player, it becomes unattached. Similarly, any nonbattle/noncreature
    permanent that isn't an Aura/Equipment/Fortification that's attached
    becomes unattached.
    """
    changed = False
    for s in game.seats:
        for p in s.battlefield:
            if p.attached_to is None:
                continue
            if p.is_aura or p.is_equipment or p.is_fortification:
                continue
            # Creature, battle, or random permanent that shouldn't be
            # attached: detach.
            p.attached_to = None
            game.ev("sba_704_5p", seat=p.controller, rule="704.5p",
                    card=p.card.name,
                    reason="non_attachable_was_attached")
            changed = True
    return changed


def _sba_704_5q(game: Game) -> bool:
    """CR 704.5q — If a permanent has both +1/+1 and -1/-1 counters on it,
    N of each are removed (N = min of the two)."""
    changed = False
    for s in game.seats:
        for p in s.battlefield:
            plus = p.counters.get("+1/+1", 0)
            minus = p.counters.get("-1/-1", 0)
            if plus > 0 and minus > 0:
                n = min(plus, minus)
                p.counters["+1/+1"] = plus - n
                p.counters["-1/-1"] = minus - n
                if p.counters["+1/+1"] == 0:
                    del p.counters["+1/+1"]
                if p.counters["-1/-1"] == 0:
                    del p.counters["-1/-1"]
                game.ev("sba_704_5q", seat=p.controller, rule="704.5q",
                        card=p.card.name,
                        removed_plus=n, removed_minus=n)
                changed = True
    return changed


def _sba_704_5r(game: Game) -> bool:
    """CR 704.5r — If a permanent has an ability saying it can't have more
    than N counters of a kind and it has more than N, remove down to N.

    We don't yet parse 'can't have more than' clauses; this is a placeholder
    that will fire when the parser surfaces those abilities (e.g. Solemnity
    says permanents can't have counters at all — that's a separate rule).
    """
    # TODO: wire up parser output for "can't have more than N" clauses.
    # Solemnity is a layer-6 replacement effect, not SBA-driven, but
    # restrictive-counter abilities exist (e.g. Vorel of the Hull Clade
    # Vampire-hate cards).
    return False


def _sba_704_5s(game: Game) -> bool:
    """CR 704.5s (reworded older rule text; maps to CR 704.5v in the
    2026-02-27 file) — A battle with 0 defense counters that isn't the
    source of an ability that has triggered but not yet left the stack is
    put into its owner's graveyard.

    Note: the 2026-02-27 comprehensive rules file renumbered battle SBAs;
    this helper implements the battle-defense-zero check. See _sba_704_5v
    below for the exact 2026-02-27 sub-letter and for the protector
    cleanup that's co-located in the same area of the CR.
    """
    # Forwarded below; keep this as a no-op anchor so the numbering stays
    # explicit in the dispatch table.
    return False


def _sba_704_5t(game: Game) -> bool:
    """CR 704.5s (2026-02-27 numbering) — If a Saga has lore counters ≥ its
    final chapter number and isn't the source of a chapter ability that has
    triggered but not yet left the stack, the controller sacrifices it.

    The engine's Saga support is minimal: we count 'chapter N' markers in
    the oracle text (roman numerals I/II/III) to infer the final chapter
    number, and compare against `counters.get("lore", 0)`. Stack-ability
    gate skipped (no saga triggers flow through the stack yet).
    """
    changed = False
    for s in game.seats:
        for p in list(s.battlefield):
            if not p.is_saga:
                continue
            lore = p.counters.get("lore", 0)
            final = _saga_final_chapter(p.card.oracle_text)
            if final is not None and lore >= final:
                # "Sacrifice" for a Saga — CR 704.5s explicitly says
                # sacrifice, not destroy. Goes to owner's graveyard.
                seat = game.seats[p.controller]
                if p in seat.battlefield:
                    seat.battlefield.remove(p)
                seat.graveyard.append(p.card)
                game.ev("sba_704_5s_saga", card=p.card.name,
                        seat=p.controller, rule="704.5s",
                        reason="saga_final_chapter",
                        lore=lore, final_chapter=final)
                changed = True
    return changed


def _sba_704_5u(game: Game) -> bool:
    """CR 704.5t (2026-02-27 numbering) — Dungeon completion: when the
    venture marker is on the bottommost room, remove the dungeon from the
    game.

    Dungeons are command-zone objects and NOT part of the 4-deck
    tournament. Stubbed with a clear TODO per the spec.
    """
    # TODO: implement when Adventures in the Forgotten Realms / command-
    # zone dungeon cards are added. Dungeons aren't in the tournament
    # decklists, so no test coverage today.
    return False


def _sba_704_5v(game: Game) -> bool:
    """CR 704.5v (2026-02-27 numbering) — If a battle has defense 0 and it
    isn't the source of an ability that has triggered but not yet left the
    stack, it's put into its owner's graveyard.
    """
    changed = False
    for s in game.seats:
        for p in list(s.battlefield):
            if p.is_battle and p.counters.get("defense", 0) <= 0:
                _destroy_perm(game, p, reason="battle_defense_zero",
                              rule="704.5v")
                changed = True
    return changed


def _sba_704_5w(game: Game) -> bool:
    """CR 704.5w (2026-02-27) — A battle with no protector chooses one or,
    if none can be chosen, is put into its owner's graveyard.

    Engine scope: protectors aren't modeled. Stubbed.
    """
    # TODO: model battle protectors. No-op until battles are fully wired.
    return False


def _sba_704_5x(game: Game) -> bool:
    """CR 704.5x (2026-02-27) — Siege-controller-is-protector cleanup.

    No-op until the engine understands Siege subtype + protector state.
    """
    return False


def _sba_704_5y(game: Game) -> bool:
    """CR 704.5y (2026-02-27) — If a permanent has more than one Role with
    the same controller attached, all but the most recent timestamp are put
    into their owners' graveyards.
    """
    changed = False
    # Collect permanents with Roles attached, grouped by (holder, controller).
    for s in game.seats:
        for holder in s.battlefield:
            # Find all roles attached to `holder`, grouped by controller.
            roles_by_ctrl: dict[int, list[Permanent]] = {}
            for s2 in game.seats:
                for r in s2.battlefield:
                    if r.is_role and r.attached_to is holder:
                        roles_by_ctrl.setdefault(r.controller, []).append(r)
            for ctrl, roles in roles_by_ctrl.items():
                if len(roles) < 2:
                    continue
                roles.sort(key=lambda q: q.timestamp)
                # Keep newest; kill the rest.
                for r in roles[:-1]:
                    _destroy_perm(game, r, reason="role_uniqueness",
                                  rule="704.5y")
                    changed = True
    return changed


def _sba_704_5z(game: Game) -> bool:
    """CR 704.5z (2026-02-27) — Start your engines: if a player controls a
    permanent with that ability and their speed is 0, their speed becomes 1.

    Engine doesn't model speed. Stubbed.
    """
    return False


_ROMAN_RE = re.compile(r"\b(I{1,3}V?|IV|V|VI{0,3}|VII|VIII|IX|X)(?:\s*[—\-,])",
                        re.IGNORECASE)


def _saga_final_chapter(text: str) -> Optional[int]:
    """Parse 'I, II, III —' style chapter markers from a Saga's oracle text
    and return the highest chapter number. Returns None if no chapter
    markers are present."""
    if not text:
        return None
    mapping = {"I": 1, "II": 2, "III": 3, "IV": 4, "V": 5,
               "VI": 6, "VII": 7, "VIII": 8, "IX": 9, "X": 10}
    best = 0
    # Lines like "I, II —" list multiple chapters sharing text.
    for m in re.finditer(r"(?m)^\s*(I{1,3}|IV|V|VI{0,3}|VII|VIII|IX|X)"
                          r"(?:\s*,\s*(I{1,3}|IV|V|VI{0,3}|VII|VIII|IX|X))*"
                          r"\s*[—\-–]",
                          text):
        for grp in m.groups():
            if grp:
                n = mapping.get(grp.upper())
                if n is not None and n > best:
                    best = n
    return best if best > 0 else None


def state_based_actions(game: Game) -> None:
    """Run all CR 704.5 state-based actions in a single simultaneous pass,
    then repeat until nothing changes (CR 704.3).

    Dispatch order follows the 2026-02-27 comp-rules file sub-letters
    (`data/rules/MagicCompRules-20260227.txt` §704.5). Order within a pass
    doesn't affect outcome — per CR 704.3 all applicable SBAs happen
    simultaneously; we call helpers sequentially and coalesce their
    observed changes. The outer while-loop re-runs the entire suite until
    a pass finds nothing to do.
    """
    # Safety cap so a malformed card can't spin forever.
    max_passes = 40
    passes = 0
    any_change_total = False
    while passes < max_passes:
        passes += 1
        changed = False
        # Player-loss SBAs first (though CR 704.5 order is irrelevant, we
        # check them first so a dead player's perms don't waste work).
        changed |= _sba_704_5a(game)  # life ≤ 0
        changed |= _sba_704_5b(game)  # drew from empty library
        changed |= _sba_704_5c(game)  # ten poison counters

        # Zone-existence SBAs.
        changed |= _sba_704_5d(game)  # token outside battlefield

        # CR 704.5e stubbed (spell/card copies outside legal zone).
        _sba_704_5e(game)

        # Creature/planeswalker/battle death SBAs.
        changed |= _sba_704_5f(game)  # toughness ≤ 0
        changed |= _sba_704_5g(game)  # lethal damage
        _sba_704_5h(game)              # deathtouch damage (stub)
        changed |= _sba_704_5i(game)  # planeswalker loyalty 0

        # Duplicate-rule SBAs.
        changed |= _sba_704_5j(game)  # legend rule
        changed |= _sba_704_5k(game)  # world rule

        # Attachment SBAs.
        changed |= _sba_704_5m(game)  # aura attach (gated stub)
        changed |= _sba_704_5n(game)  # equipment / fortification
        changed |= _sba_704_5p(game)  # creature/battle attached

        # Counter SBAs.
        changed |= _sba_704_5q(game)  # +1/+1 vs -1/-1 annihilation
        _sba_704_5r(game)              # can't-have-more-than (stub)

        # Saga / Dungeon / Battle / Role SBAs.
        _sba_704_5s(game)              # anchor (battles handled by 704.5v)
        changed |= _sba_704_5t(game)  # saga final chapter → sacrifice
        _sba_704_5u(game)              # dungeon (stub; not in tournament)
        changed |= _sba_704_5v(game)  # battle defense 0
        _sba_704_5w(game)              # battle no protector (stub)
        _sba_704_5x(game)              # siege protector (stub)
        changed |= _sba_704_5y(game)  # role uniqueness
        _sba_704_5z(game)              # speed initialization (stub)

        # §704.6 — variant-specific SBAs. Commander clauses are gated
        # on game.commander_format; non-Commander games short-circuit
        # at the top of each helper, so the cost is one branch.
        #   §704.6c — 21+ combat damage from same commander → loss.
        #   §704.6d — commander in graveyard/exile → may return to
        #             command zone (owner choice; policy = greedy).
        changed |= _sba_704_6c(game)
        changed |= _sba_704_6d(game)

        if not changed:
            break
        any_change_total = True

    if passes >= max_passes:
        game.ev("sba_cap_hit", rule="704.3",
                reason="exceeded_max_passes", passes=passes)

    if any_change_total:
        game.ev("sba_cycle_complete", passes=passes)
    game.check_end()


# ============================================================================
# Paradox detection
# ============================================================================
# Used by the paradox harness (interaction_harness_paradoxes.py). Looks at the
# battlefield and returns a list of diagnostic strings describing any known
# unstable configurations §613 / replacement-effect loops. Empty list means
# "no paradox detected". This is purely diagnostic — the harness drives the
# actual resolution; this helper is here so other agents can probe state.
#
# Known paradox shapes:
#   - Humility + Opalescence  ("humility_opalescence"):
#       both enchantments on battlefield → layer-6 vs layer-7b ordering is
#       timestamp-dependent; a stable state exists but is not unique.
#   - Worldgorger Dragon + Animate Dead / Dance of the Dead / Necromancy:
#       ETB exile-all + LTB-on-aura-loss → classic Dragon loop.
#   - Blood Moon + Dryad Arbor (or any Forest-Creature land):
#       type-replacement erases Forest, layer-4 order interacts w/ land-type
#       vs creature-type.
#   - Painter's Servant + Grindstone:
#       color-wash makes every card share a color → Grindstone repeats to
#       library empty.

_PARADOX_SHAPES = {
    "humility_opalescence": {"Humility", "Opalescence"},
    "worldgorger_animate": {"Worldgorger Dragon", "Animate Dead"},
    "worldgorger_dance":   {"Worldgorger Dragon", "Dance of the Dead"},
    "worldgorger_necro":   {"Worldgorger Dragon", "Necromancy"},
    "painter_grindstone":  {"Painter's Servant", "Grindstone"},
}


def detect_paradox(game: Game) -> list[str]:
    """Return a list of paradox-shape tags that are present on the battlefield
    right now (across both seats). Empty list = no paradox detected."""
    names = set()
    for s in game.seats:
        for p in s.battlefield:
            names.add(p.card.name)
    hits: list[str] = []
    for tag, required in _PARADOX_SHAPES.items():
        if required.issubset(names):
            hits.append(tag)
    # Blood-Moon-class: Blood Moon OR Magus of the Moon alongside any Forest-
    # creature land. Dryad Arbor is the canonical example; Treetop Village etc.
    # are manland-animations and don't count as land-creatures at rest.
    moon_on = any(n in names for n in ("Blood Moon", "Magus of the Moon"))
    if moon_on:
        for s in game.seats:
            for p in s.battlefield:
                tl = p.card.type_line.lower()
                if "land" in tl and "creature" in tl:
                    hits.append("blood_moon_land_creature")
                    break
            else:
                continue
            break
    return hits


# ============================================================================
# Day / Night + DFC transform (CR §726, §712, §702.144, §702.145)
# ============================================================================
#
# Scope of this block:
#   - game.day_night ∈ {"neither", "day", "night"} per CR §726.
#   - has_daybound_or_nightbound_permanent() — §726.2 trigger.
#   - evaluate_day_night_at_turn_start() — §726.3a day↔night transition.
#   - transform_permanent() — CR §712 face swap for DFCs.
#   - apply_daybound_nightbound_transforms() — §702.144/145 auto-flip
#     every daybound/nightbound creature whenever day/night changes.
#
# Reference citations (data/rules/MagicCompRules-20260227.txt):
#
#   §712.1   A double-faced card has two faces, a front and a back; only
#            one face is up at a time. The other face's characteristics
#            are not taken into account.
#   §712.2   A double-faced card enters the battlefield with its front
#            face up by default.
#   §712.3   Transform swaps which face is up. The permanent keeps its
#            counters, attachments, etc. — only its characteristics
#            change to those of the other face.
#   §712.8   A permanent gets a new timestamp when it transforms.
#   §726.1   Some effects instruct the game to "become day" or "become
#            night." The game tracks which of day, night, or neither
#            the game currently is.
#   §726.2   The game begins as "neither day nor night." A game becomes
#            day the first time that a permanent with daybound or
#            nightbound enters the battlefield under any player's
#            control.
#   §726.3a  day + prev-turn active cast 0 spells → night;
#            night + prev-turn active cast 2+ spells → day.
#   §702.144 Daybound — while night, daybound creatures you control
#            transform.
#   §702.145 Nightbound — while day, nightbound creatures you control
#            transform.


def has_daybound_or_nightbound_permanent(game: "Game") -> bool:
    """True iff any permanent on any battlefield has daybound or
    nightbound (on either face). §726.2 trigger."""
    for s in game.seats:
        for p in s.battlefield:
            card = p.card
            if not card:
                continue
            ast = card.ast
            if ast:
                for a in getattr(ast, "abilities", ()) or ():
                    if isinstance(a, Keyword) and a.name in ("daybound",
                                                             "nightbound"):
                        return True
            for face_ast in (card.front_face_ast, card.back_face_ast):
                if face_ast is None:
                    continue
                for a in getattr(face_ast, "abilities", ()) or ():
                    if isinstance(a, Keyword) and a.name in ("daybound",
                                                             "nightbound"):
                        return True
    return False


def _permanent_has_keyword(perm: "Permanent", keyword: str) -> bool:
    """True iff the permanent's CURRENTLY-ACTIVE face has the named
    keyword."""
    if perm is None or perm.card is None or perm.card.ast is None:
        return False
    for a in getattr(perm.card.ast, "abilities", ()) or ():
        if isinstance(a, Keyword) and a.name == keyword:
            return True
    return False


def transform_permanent(game: "Game", perm: "Permanent",
                        reason: str = "effect") -> bool:
    """CR §712 — swap the permanent's active face. Returns True on a
    successful transform, False if the permanent can't transform.

    On transform:
      - perm.transformed toggles.
      - perm.card is replaced with a new CardEntry whose top-level
        characteristics reflect the other face (AST, P/T, type line,
        colors, oracle text, mana cost). The ORIGINAL CardEntry remains
        unmolested — copies elsewhere (graveyard, library, command
        zone) keep the front-face-up representation.
      - perm.timestamp is refreshed (§712.8).
      - Characteristics cache is invalidated (§613 re-tagging).
      - A `transform` event is emitted.
    """
    if perm is None or perm.card is None:
        return False
    card = perm.card
    if not getattr(card, "is_dfc", False):
        game.ev("transform_noop", card=card.name, reason="not_dfc",
                rule="712.1")
        return False
    front_active = not perm.transformed
    target_front = not front_active
    if target_front:
        new_ast = card.front_face_ast or card.ast
        new_name = card.front_face_name or card.name
        new_mana_cost = card.front_face_mana_cost or card.mana_cost
        new_cmc = card.front_face_cmc if card.front_face_cmc is not None else card.cmc
        new_type_line = card.front_face_type_line or card.type_line
        new_oracle = card.front_face_oracle_text or card.oracle_text
        new_power = card.front_face_power if card.front_face_power is not None else card.power
        new_toughness = card.front_face_toughness if card.front_face_toughness is not None else card.toughness
        new_colors = card.front_face_colors or card.colors
    else:
        new_ast = card.back_face_ast or card.ast
        new_name = card.back_face_name or card.name
        new_mana_cost = card.back_face_mana_cost or card.mana_cost
        new_cmc = card.back_face_cmc if card.back_face_cmc is not None else card.cmc
        new_type_line = card.back_face_type_line or card.type_line
        new_oracle = card.back_face_oracle_text or card.oracle_text
        new_power = card.back_face_power if card.back_face_power is not None else card.power
        new_toughness = card.back_face_toughness if card.back_face_toughness is not None else card.toughness
        new_colors = card.back_face_colors or card.colors
    from_name = card.name
    import dataclasses as _dc
    swapped = _dc.replace(
        card,
        name=new_name,
        mana_cost=new_mana_cost or "",
        cmc=int(new_cmc or 0),
        type_line=new_type_line or "",
        oracle_text=new_oracle or "",
        power=new_power,
        toughness=new_toughness,
        ast=new_ast,
        colors=new_colors,
    )
    perm.card = swapped
    perm.transformed = not perm.transformed
    perm.timestamp = game.next_timestamp()
    game.invalidate_characteristics_cache()
    game.ev("transform",
            card=from_name,
            to_face=new_name,
            controller_seat=perm.controller,
            now_transformed=perm.transformed,
            reason=reason,
            rule="712.3")
    return True


def _set_day_night(game: "Game", new_state: str, *,
                   reason: str, rule: str) -> None:
    """Transition the game's day/night state and fire the §702.144/145
    auto-transform sweep. Emits `day_night_change`.
    """
    if new_state == game.day_night:
        return
    prev = game.day_night
    game.day_night = new_state
    game.ev("day_night_change",
            from_state=prev,
            to_state=new_state,
            reason=reason,
            rule=rule)
    apply_daybound_nightbound_transforms(game)


def _maybe_become_day(game: "Game", reason: str = "daybound_etb") -> None:
    """§726.2 — if the game is currently 'neither' and there exists a
    daybound/nightbound permanent on any battlefield, game becomes day.
    Idempotent."""
    if game.day_night != "neither":
        return
    if not has_daybound_or_nightbound_permanent(game):
        return
    _set_day_night(game, "day", reason=reason, rule="726.2")


def apply_daybound_nightbound_transforms(game: "Game") -> None:
    """Walk every permanent. Transform each daybound/nightbound
    permanent whose ACTIVE-face keyword says it should flip given the
    current day/night state.

    §702.144: while night, daybound active face → transform to back.
    §702.145: while day, nightbound active face → transform to front.
    """
    state = game.day_night
    if state not in ("day", "night"):
        return
    candidates = []
    for s in game.seats:
        for p in list(s.battlefield):
            candidates.append(p)
    for p in candidates:
        if p.card is None or not getattr(p.card, "is_dfc", False):
            continue
        has_daybound = _permanent_has_keyword(p, "daybound")
        has_nightbound = _permanent_has_keyword(p, "nightbound")
        if state == "day" and has_nightbound:
            transform_permanent(game, p, reason="state_became_day")
        elif state == "night" and has_daybound:
            transform_permanent(game, p, reason="state_became_night")


def evaluate_day_night_at_turn_start(game: "Game") -> None:
    """§726.3a — at each turn start (BEFORE untap), check the day↔night
    transition based on the PREVIOUS turn's active-player spell count
    (game.spells_cast_by_active_last_turn)."""
    last_casts = game.spells_cast_by_active_last_turn
    if game.day_night == "day" and last_casts == 0:
        _set_day_night(game, "night",
                       reason="prev_active_cast_zero",
                       rule="726.3a")
    elif game.day_night == "night" and last_casts >= 2:
        _set_day_night(game, "day",
                       reason="prev_active_cast_two_plus",
                       rule="726.3a")


# ============================================================================
# Turn structure
# ============================================================================

def untap_step(game: Game) -> None:
    """CR §502. Untap step — active player untaps each of their permanents.
    Routes every untap attempt through attempt_untap() so §614 replacements
    like stun counters (§122.1d) can intercept.

    Reset per-turn bookkeeping (mana pool, lands played, combat phase
    counter). "Until end of turn" buffs/damage are NOT cleared here —
    they clear at the previous turn's cleanup (§514.2). We previously
    did it here as a workaround; scan_expired_durations() in
    set_phase_step() now handles this correctly at cleanup time.
    """
    s = game.seats[game.active]
    game.set_phase_step("beginning", "untap")
    # CR §502.1: active player's phasing-in phenomena, then untap.
    # We skip phasing (not implemented). Untap everything the active
    # player controls — but route through attempt_untap so stun counters
    # and other "would_untap" §614 replacements fire.
    for p in s.battlefield:
        p.summoning_sick = False
        if p.tapped:
            attempt_untap(game, p, reason="untap_step")
    if s.mana_pool:
        game.ev("pool_drain", seat=s.idx, amount=s.mana_pool,
                reason="untap_step")
    s.mana_pool = 0
    s.lands_played_this_turn = 0
    # Reset combat-phase counter for the new turn.
    game.combat_phase_number = 0
    game.pending_extra_combats = 0
    # CR §700.4 / §702.40 — reset per-turn cast counters. The GLOBAL
    # counter (game.spells_cast_this_turn) resets every untap because
    # Storm reads "spells cast this turn" across all seats and a new
    # "turn" begins at every untap step.
    #
    # The active seat's per-seat counter is snapshotted into
    # spells_cast_last_turn and then zeroed. Non-active seats keep their
    # counters intact — their cast-count observability window ends at
    # THEIR untap step, not this one (a counterspell cast during the
    # active player's turn still "counts" for the defender until they
    # take their next untap).
    game.spells_cast_this_turn = 0
    s.spells_cast_last_turn = s.spells_cast_this_turn
    s.spells_cast_this_turn = 0
    game.emit("untap")


def upkeep_step(game: Game) -> None:
    """CR §503. Upkeep step."""
    game.set_phase_step("beginning", "upkeep")
    for seat in game.seats:
        for p in list(seat.battlefield):
            for eff in collect_upkeep_effects(p.card.ast):
                resolve_effect(game, p.controller, eff)
                if game.ended: return


def draw_step(game: Game) -> None:
    """CR §504. Draw step."""
    game.set_phase_step("beginning", "draw")
    # Skip draw on turn 1 for active player? Classic rule. MVP: always draw.
    draw_cards(game, game.seats[game.active], 1)
    game.emit(f"draw → hand={len(game.seats[game.active].hand)}")
    # CR §504 "draw step" — the drawn card IS the "first draw in each of
    # their draw steps" that Orcish Bowmasters / Notion Thief skip. Flag
    # it so _fire_draw_trigger_observers knows to suppress those.
    game._suppress_first_draw_trigger = game.active
    game.ev("draw", seat=game.active, count=1, hand_size=len(game.seats[game.active].hand))
    _fire_draw_trigger_observers(game, game.active, count=1)


def fill_mana_pool(game: Game, reserve: int = 0) -> None:
    """Tap untapped mana sources to fill the pool. If `reserve` > 0,
    leave that many pips worth of lands untapped so the player can hold
    them for an instant-speed response on the opponent's next turn
    (counterspell).

    CR §106 — mana. Now source-aware across three families:
      - lands (basic subtype → color; activated → ability.pool colors)
      - mana dorks (non-summoning-sick creatures with {T}: add mana)
      - artifacts (Sol Ring, Signets, Moxes, Mana Crypt, Treasures, …)
    Each tapped source routes its pips into the TYPED ManaPool via
    add_mana_from_ability / _try_tap_artifact_for_mana, so color
    information is preserved at pool-level.
    """
    s = game.seats[game.active]
    # Gather candidate sources and their pip counts for reserve accounting.
    sources: list[tuple[Permanent, int, str]] = []
    for p in s.battlefield:
        if p.tapped:
            continue
        if p.is_land:
            bl = basic_land_mana(p.card.type_line)
            if bl:
                pips = bl.any_color_count + len(bl.pool)
                sources.append((p, pips, "land"))
                continue
            tm = get_tap_mana_ability(p.card.ast)
            if tm:
                pips = tm.any_color_count + len(tm.pool)
                sources.append((p, pips, "land_activated"))
        elif p.is_creature and not p.summoning_sick:
            tm = get_tap_mana_ability(p.card.ast)
            if tm:
                pips = tm.any_color_count + len(tm.pool)
                sources.append((p, pips, "mana_dork"))
        elif p.is_artifact:
            # Skip artifacts with destructive costs (LED-style) in
            # auto-tap. Those should require explicit policy activation.
            if _artifact_has_destructive_cost(p):
                continue
            # Artifacts don't pre-check their pip count here — defer
            # to _try_tap_artifact_for_mana, which knows about token
            # synthetics + sacrifice costs. Assume 1 pip for reserve
            # accounting (a conservative overestimate that still allows
            # reserve to hold off on artifact mana).
            sources.append((p, 1, "artifact"))
    total_avail = sum(pips for _, pips, _ in sources)
    to_tap_budget = max(0, total_avail - reserve)
    tapped_so_far = 0
    for p, pips, src in sources:
        if tapped_so_far + pips > to_tap_budget:
            continue
        if src == "land":
            bl = basic_land_mana(p.card.type_line)
            p.tapped = True
            add_mana_from_ability(game, s, bl,
                                   source_name=p.card.name,
                                   source_kind="land")
        elif src == "land_activated":
            tm = get_tap_mana_ability(p.card.ast)
            p.tapped = True
            add_mana_from_ability(game, s, tm,
                                   source_name=p.card.name,
                                   source_kind="land_activated")
        elif src == "mana_dork":
            tm = get_tap_mana_ability(p.card.ast)
            p.tapped = True
            add_mana_from_ability(game, s, tm,
                                   source_name=p.card.name,
                                   source_kind="mana_dork")
        elif src == "artifact":
            if not _try_tap_artifact_for_mana(game, s, p):
                # Couldn't derive mana from this artifact — skip it.
                continue
        tapped_so_far += pips


def can_play_land(game: Game) -> Optional[CardEntry]:
    s = game.seats[game.active]
    if s.lands_played_this_turn >= 1:
        return None
    for c in s.hand:
        if "land" in c.type_line.lower():
            return c
    return None


def play_land(game: Game, card: CardEntry) -> None:
    s = game.seats[game.active]
    s.hand.remove(card)
    perm = Permanent(card=card, controller=game.active,
                     tapped=False, summoning_sick=False)
    _etb_initialize(game, perm)
    s.battlefield.append(perm)
    s.lands_played_this_turn += 1
    game.emit(f"play land {card.name}")
    game.ev("play_land", card=card.name, seat=game.active)


def can_cast(game: Game, card: CardEntry) -> bool:
    s = game.seats[game.active]
    if "land" in card.type_line.lower():
        return False
    # If card is creature and it's main phase we can cast
    if card.cmc > s.mana_pool:
        return False
    return True


# ---------------------------------------------------------------------------
# Stack / priority helpers
# ---------------------------------------------------------------------------


def _card_has_counterspell(card: CardEntry) -> bool:
    """Does this card contain a CounterSpell effect in its AST?"""
    for eff in collect_spell_effects(card.ast):
        if getattr(eff, "kind", None) == "counter_spell":
            return True
    return False


def _is_instant(card: CardEntry) -> bool:
    return "instant" in card.type_line.lower()


def _get_parsed_mana_cost(card: "CardEntry") -> "Optional[ManaCost]":
    """Return a parsed ManaCost for the card. CardAST doesn't carry a
    ManaCost directly (the parser keeps it on CardEntry.mana_cost as a
    raw string), so we parse it on demand. Results are cached on the
    card via a side-attribute so repeated casts don't re-parse."""
    cached = getattr(card, "_parsed_mana_cost", "MISSING")
    if cached != "MISSING":
        return cached
    raw = getattr(card, "mana_cost", "") or ""
    parsed = None
    if raw:
        try:
            parsed = mtg_parser.parse_mana_cost(raw)
        except Exception:
            parsed = None
    try:
        object.__setattr__(card, "_parsed_mana_cost", parsed)
    except Exception:
        pass
    return parsed


def _classify_spell_type(card: "CardEntry") -> str:
    """Return the restriction-aware spell-type tag used by
    _restriction_allows. Precedence:
      creature > instant > sorcery > artifact > noncreature > generic.
    (A creature-artifact-spell is classified as CREATURE so Food Chain
    mana can pay it; Powerstone mana can't.)"""
    tl = (card.type_line or "").lower()
    if "creature" in tl:
        return "creature"
    if "instant" in tl:
        return "instant"
    if "sorcery" in tl:
        return "sorcery"
    if "artifact" in tl:
        return "artifact"
    if "land" in tl:
        return "land"
    return "noncreature"


def pay_mana_cost(game: "Game", seat: "Seat", mana_cost: "Optional[ManaCost]",
                  spell_type: str = "generic",
                  reason: str = "cast",
                  card_name: str = "") -> bool:
    """Pay a FULL colored mana cost from seat's typed pool. Attempts to
    tap mana sources as needed. Returns True on success (state mutated)
    or False if unpayable (state untouched).

    Algorithm:
      1. Break mana_cost into colored requirements + generic requirement.
      2. Check if we can pay from pool alone (with top-up from sources);
         if not, return False.
      3. Fill pool from typed sources (lands, dorks, artifacts).
      4. Debit colored costs from matching color bucket (falling back to
         `any`, then to compatible restricted mana).
      5. Debit generic cost from any remaining (any-first to preserve
         colors, respecting spend-restrictions).
      6. Emit pay_mana event tagged with bucket breakdown.
    """
    if mana_cost is None:
        return True
    colored_needed: dict[str, int] = {"W": 0, "U": 0, "B": 0, "R": 0, "G": 0, "C": 0}
    generic_needed = 0
    for sym in mana_cost.symbols:
        if sym.is_x:
            continue  # X is resolved separately; caller pays the chosen value
        if sym.generic > 0:
            generic_needed += sym.generic
            continue
        if sym.raw == "{C}":
            colored_needed["C"] += 1
            continue
        if sym.color:
            if len(sym.color) == 1:
                colored_needed[sym.color[0]] += 1
            else:
                # Hybrid — add to generic (the player picks a side, and
                # any-color mana can pay either).
                generic_needed += 1
    # Ensure pool + producible sources is enough.
    if not _ensure_pool_can_pay(game, seat, colored_needed, generic_needed,
                                spell_type=spell_type):
        return False
    paid_breakdown: dict[str, int] = {}
    # Colored pips first.
    for color, amt in colored_needed.items():
        for _ in range(amt):
            bucket = _debit_colored_pip(seat, color, spell_type)
            if bucket is None:
                return False  # Shouldn't happen after _ensure check.
            paid_breakdown[bucket] = paid_breakdown.get(bucket, 0) + 1
    # Generic pips.
    for _ in range(generic_needed):
        bucket = _debit_generic_pip(seat, spell_type)
        if bucket is None:
            return False
        paid_breakdown[bucket] = paid_breakdown.get(bucket, 0) + 1
    total_paid = sum(paid_breakdown.values())
    game.ev("pay_mana", seat=seat.idx, amount=total_paid,
            reason=reason, card=card_name,
            breakdown=paid_breakdown,
            pool_after=seat.mana.total())
    return True


def _debit_colored_pip(seat: "Seat", color: str, spell_type: str) -> Optional[str]:
    """Debit ONE pip of `color` from the seat's typed pool. Prefers the
    matching color bucket, falls back to `any`, then to compatible
    restricted mana. Returns the bucket name spent, or None if we
    couldn't find a source."""
    if color in ("W", "U", "B", "R", "G", "C"):
        have = getattr(seat.mana, color)
        if have > 0:
            setattr(seat.mana, color, have - 1)
            return color
    if seat.mana.any > 0:
        seat.mana.any -= 1
        return "any"
    # Try restricted buckets. For colored pips, the restricted mana must
    # match the color (or be any-color) AND the restriction must permit
    # the spell type.
    for r in seat.mana.restricted:
        if r.amount <= 0:
            continue
        if r.color is not None and r.color != color:
            continue
        if not _restriction_allows(r.restriction, spell_type, colorless=(color == "C")):
            continue
        r.amount -= 1
        if r.amount == 0:
            seat.mana.restricted.remove(r)
        return "restricted:%s" % r.restriction
    # {C} has a special rule: ONLY colorless or any may pay it (CR §107.4c
    # — colored mana can't pay a {C} cost). If we've fallen through above,
    # we've already exhausted C, any, and restricted-C/any.
    return None


def _debit_generic_pip(seat: "Seat", spell_type: str) -> Optional[str]:
    """Debit ONE pip of generic mana. Any color works. Prefers `any`
    first to preserve colored buckets for colored costs on subsequent
    spells. Falls back to restricted mana that permits the spell type."""
    order = ("any", "C", "W", "U", "B", "R", "G")
    for bucket in order:
        have = getattr(seat.mana, bucket)
        if have > 0:
            setattr(seat.mana, bucket, have - 1)
            return bucket
    for r in seat.mana.restricted:
        if r.amount <= 0:
            continue
        if not _restriction_allows(r.restriction, spell_type, colorless=False):
            continue
        r.amount -= 1
        if r.amount == 0:
            seat.mana.restricted.remove(r)
        return "restricted:%s" % r.restriction
    return None


def _ensure_pool_can_pay(game: "Game", seat: "Seat",
                          colored_needed: dict, generic_needed: int,
                          spell_type: str) -> bool:
    """Ensure seat's typed pool has enough buckets to cover the cost,
    tapping untapped mana sources if necessary.

    Simplified payability heuristic:
      - For each colored requirement: need color_bucket + any + compatible
        restricted ≥ amount. If not, try tapping sources that produce that
        color. If still not met → False.
      - For generic: total pool after colored reservation must cover it.
      - Tapping is greedy: we iterate sources, tap any that contribute
        colors we're short on, until we either meet all requirements or
        run out of sources.

    Mutates state (taps sources, adds mana) even on False return — but
    only if we actually couldn't pay; the pool state after a False is
    recoverable because nothing has been DEBITED yet (only tapped and
    filled), so the seat just has more mana than it started with. The
    caller can always continue the turn.
    """
    # First pass: tap all untapped sources the seat would need. For
    # simplicity we tap aggressively — any unused mana will drain at
    # phase end anyway. This avoids the combinatorial "which color
    # source should I tap?" problem by just producing everything we can.
    #
    # A later optimization pass can add minimally-sufficient tapping;
    # the current behavior matches legacy fill_mana_pool + _pay_generic
    # which also effectively taps everything available on demand.
    #
    # We still short-circuit if the pool already has enough — no point
    # tapping when we're full.
    total_needed = sum(colored_needed.values()) + generic_needed
    if seat.mana.total() < total_needed:
        # Tap every untapped source. Colors will sort out at debit time.
        _tap_all_available_sources(game, seat)
    # Now verify color-by-color.
    # Track "any" reserve — `any` can sub for any color but each unit
    # spent on color X can't also pay color Y. So we tally in order,
    # consuming any/restricted-compatible as we go.
    pool_copy = _snapshot_pool(seat.mana)
    for color, amt in colored_needed.items():
        for _ in range(amt):
            if pool_copy.get(color, 0) > 0:
                pool_copy[color] -= 1
            elif pool_copy.get("any", 0) > 0:
                pool_copy["any"] -= 1
            else:
                # Try restricted.
                spent = False
                for r in pool_copy.get("restricted", []):
                    if r["amount"] <= 0:
                        continue
                    if r["color"] not in (None, color):
                        continue
                    if not _restriction_allows(r["restriction"], spell_type, colorless=(color == "C")):
                        continue
                    r["amount"] -= 1
                    spent = True
                    break
                if not spent:
                    return False
    # Remaining pool must cover generic.
    remaining = (pool_copy["W"] + pool_copy["U"] + pool_copy["B"]
                 + pool_copy["R"] + pool_copy["G"] + pool_copy["C"]
                 + pool_copy["any"])
    for r in pool_copy["restricted"]:
        if r["amount"] <= 0:
            continue
        if _restriction_allows(r["restriction"], spell_type, colorless=False):
            remaining += r["amount"]
    return remaining >= generic_needed


def _snapshot_pool(pool: ManaPool) -> dict:
    """Deep-copy the pool into a plain-dict form for dry-run
    _ensure_pool_can_pay checks (so we don't accidentally mutate the
    live pool while probing payability)."""
    return {
        "W": pool.W, "U": pool.U, "B": pool.B,
        "R": pool.R, "G": pool.G, "C": pool.C,
        "any": pool.any,
        "restricted": [{"amount": r.amount, "color": r.color,
                         "restriction": r.restriction} for r in pool.restricted],
    }


def _tap_all_available_sources(game: "Game", seat: "Seat") -> None:
    """Tap every untapped land/dork/artifact mana source and route its
    output into the typed pool. Used by _ensure_pool_can_pay when the
    pool is short. Idempotent for already-tapped permanents."""
    for p in seat.battlefield:
        if p.tapped:
            continue
        if p.is_land:
            bl = basic_land_mana(p.card.type_line)
            if bl:
                p.tapped = True
                add_mana_from_ability(game, seat, bl,
                                       source_name=p.card.name,
                                       source_kind="land")
                continue
            tm = get_tap_mana_ability(p.card.ast)
            if tm:
                p.tapped = True
                add_mana_from_ability(game, seat, tm,
                                       source_name=p.card.name,
                                       source_kind="land_activated")
        elif p.is_creature and not p.summoning_sick:
            tm = get_tap_mana_ability(p.card.ast)
            if tm:
                p.tapped = True
                add_mana_from_ability(game, seat, tm,
                                       source_name=p.card.name,
                                       source_kind="mana_dork")
        elif p.is_artifact:
            if _artifact_has_destructive_cost(p):
                continue
            _try_tap_artifact_for_mana(game, seat, p)


def _artifact_has_destructive_cost(p: "Permanent") -> bool:
    """True if tapping this artifact for mana has side effects harmful
    to the controller (Lion's Eye Diamond's discard-your-hand cost,
    prospective Mindslaver-class artifacts, etc.). Auto-pay paths
    skip these; explicit policy activation is required to tap them."""
    return p.card.name == "Lion's Eye Diamond"


def _try_tap_artifact_for_mana(game: "Game", seat: "Seat",
                               p: "Permanent") -> bool:
    """Tap an artifact for mana if it has a suitable activated ability.

    Handles three sub-cases:
      (a) Plain {T}: Add … — Sol Ring, Mana Vault, Grim Monolith,
          Basalt Monolith, Thran Dynamo, Fellwar Stone, Signets,
          Talismans, Arcane Signet, Mox*, Chromatic Lantern.
      (b) {T}, sacrifice: Add … — Lotus Petal, Treasure tokens.
      (c) {T}, discard-hand, sacrifice: Add … — Lion's Eye Diamond
          (discard-hand is NOT modeled here; the basic {T}, sac branch
          approximates via hardcoded LED logic).

    We additionally recognize TOKEN artifacts by name (Treasure, Gold,
    Powerstone — their AST may be empty) and synthesize mana from them
    according to the standard token rules.
    """
    if p.tapped:
        return False
    name = p.card.name
    tl = (p.card.type_line or "").lower()
    # Token artifacts: recognize by name (their AST is empty).
    if "treasure" in tl or name == "Treasure Token":
        # {T}, sacrifice: add one mana of any color.
        p.tapped = True
        seat.mana.add("any", 1)
        _sacrifice_permanent(game, p, reason="treasure_tap")
        game.ev("add_mana", seat=seat.idx, amount=1,
                source="treasure_token", source_card=name,
                colors="any", pool_after=seat.mana.total())
        return True
    if "powerstone" in tl or name == "Powerstone Token":
        # {T}: add {C}, spend only on noncreature costs.
        p.tapped = True
        seat.mana.add_restricted(1, "C",
                                  "noncreature_or_artifact_activation",
                                  source_name=name)
        game.ev("add_mana", seat=seat.idx, amount=1,
                source="powerstone_token", source_card=name,
                colors="C_restricted", pool_after=seat.mana.total())
        return True
    if name == "Gold Token":
        # Sacrifice (no tap): add one mana of any color. Since this
        # isn't a tap ability we still model it via sac on demand.
        seat.mana.add("any", 1)
        _sacrifice_permanent(game, p, reason="gold_sac")
        game.ev("add_mana", seat=seat.idx, amount=1,
                source="gold_token", source_card=name,
                colors="any", pool_after=seat.mana.total())
        return True
    if name == "Meteorite Token" or "meteorite" in tl:
        # {T}: Add one mana of any color. Permanent rock — no sacrifice.
        # ETB 2 damage handled at creation time via _create_meteorite_token.
        p.tapped = True
        seat.mana.add("any", 1)
        game.ev("add_mana", seat=seat.idx, amount=1,
                source="meteorite_token", source_card=name,
                colors="any", pool_after=seat.mana.total())
        return True
    # Fellwar Stone — runtime check of opponents' lands' color output.
    # CR §ruling: Fellwar reads "a color a land an opponent controls could
    # produce." Computed fresh at each tap. Per 7174n1c 2026-04-16:
    # runtime check with pilot forcing the color. Edge case noted but
    # deferred: seat Y won't sandbag their own lands to deny colors to
    # seat X — strategic counterplay, not rules-mandated.
    if name == "Fellwar Stone":
        available = _opponent_land_colors(game, seat.idx)
        if not available:
            # No opponent lands producing any color → no mana.
            # (Degenerate case; Fellwar just doesn't tap.)
            return False
        # Pilot-forced color choice: take first color of the sorted set
        # for determinism. Future: delegate to hat.ChooseColor(available).
        chosen = sorted(available)[0]
        p.tapped = True
        seat.mana.add(chosen, 1)
        game.ev("add_mana", seat=seat.idx, amount=1,
                source="fellwar_runtime", source_card=name,
                colors=chosen,
                available_opponent_colors=sorted(available),
                pool_after=seat.mana.total())
        return True
    # Real artifacts: walk the AST for a tap-add-mana activated ability.
    tm = get_tap_mana_ability(p.card.ast)
    if tm:
        p.tapped = True
        add_mana_from_ability(game, seat, tm,
                               source_name=name,
                               source_kind="artifact")
        # Mana Vault: "doesn't untap during your untap step unless you
        # pay {4}" — we approximate by tagging a flag on the permanent;
        # untap handling can consult it. For MVP we just leave it
        # tapped (a future extension can model the pay-{4} option).
        if name in ("Mana Vault", "Grim Monolith", "Basalt Monolith"):
            if not hasattr(p, "_mana_vault_stuck"):
                setattr(p, "_mana_vault_stuck", True)
        return True
    # Sacrifice-as-cost mana (Lotus Petal, Jeweled Lotus). Recognized
    # by exact name + oracle-text sniff.
    if name == "Lotus Petal":
        p.tapped = True
        seat.mana.add("any", 1)
        _sacrifice_permanent(game, p, reason="lotus_petal_tap")
        game.ev("add_mana", seat=seat.idx, amount=1,
                source="lotus_petal", source_card=name,
                colors="any", pool_after=seat.mana.total())
        return True
    if name == "Jeweled Lotus":
        # {T}, sacrifice Jeweled Lotus: add three mana of any one color.
        # Spend only to cast your commander. We model the spend
        # restriction as "commander_only"; the cast path can honor it
        # when it's implemented. MVP: treat as any.
        p.tapped = True
        seat.mana.add("any", 3)
        _sacrifice_permanent(game, p, reason="jeweled_lotus_tap")
        game.ev("add_mana", seat=seat.idx, amount=3,
                source="jeweled_lotus", source_card=name,
                colors="any*3", pool_after=seat.mana.total())
        return True
    if name == "Lion's Eye Diamond":
        # {T}, Discard your hand, Sacrifice Lion's Eye Diamond: Add
        # three mana of one color.
        # CRITICAL: LED can only be activated at instant speed with an
        # empty hand afterwards — typical use is "in response to a
        # tutor resolving" to add mana while discarding the card you'd
        # otherwise keep. For this pass we fire the mana ONLY if the
        # seat has a hand to discard; we discard the hand as cost.
        p.tapped = True
        # Discard the hand as a cost (moves cards to graveyard).
        while seat.hand:
            card = seat.hand.pop(0)
            seat.graveyard.append(card)
            game.ev("discard", seat=seat.idx, card=card.name,
                    reason="lions_eye_diamond_cost")
        seat.mana.add("any", 3)
        _sacrifice_permanent(game, p, reason="lions_eye_diamond_tap")
        game.ev("add_mana", seat=seat.idx, amount=3,
                source="lions_eye_diamond", source_card=name,
                colors="any*3", pool_after=seat.mana.total())
        return True
    return False


def _sacrifice_permanent(game: "Game", p: "Permanent",
                         reason: str = "sacrifice") -> None:
    """Move a permanent from the battlefield to its owner's graveyard.
    For tokens, they cease to exist (§704.5d) — we remove without
    adding to graveyard. Emits a `sacrifice` event.

    This is a minimal helper for the mana-tap flow; a fuller
    sacrifice-as-cost handler (§701.16) would also fire §603.7 die
    triggers, §614 LTB replacements, etc. For now we call
    unregister_replacements_for_permanent if available so continuous
    effects from this permanent stop applying."""
    seat = game.seats[p.controller]
    if p in seat.battlefield:
        seat.battlefield.remove(p)
    try:
        unregister_replacements_for_permanent(game, p)
    except Exception:
        pass
    try:
        unregister_continuous_effects_for_permanent(game, p)
    except Exception:
        pass
    is_token = False
    try:
        is_token = p.is_token
    except Exception:
        pass
    if not is_token:
        owner_idx = getattr(p, "owner", None)
        if owner_idx is None:
            owner_idx = p.controller
        owner = game.seats[owner_idx]
        owner.graveyard.append(p.card)
    game.ev("sacrifice", seat=p.controller, card=p.card.name,
            reason=reason)


def _available_mana(seat: Seat) -> int:
    """Pool + mana we could produce by tapping untapped mana sources.

    Now includes artifact mana (Sol Ring, Signets, Moxes, Mana Crypt,
    Treasures, etc.) — the previous two-branch (land / creature) logic
    silently ignored every artifact mana source, which is why every
    fast-mana combo deck was underperforming.
    """
    pool = seat.mana.total()
    potential = 0
    for p in seat.battlefield:
        if p.tapped:
            continue
        if p.is_land:
            bl = basic_land_mana(p.card.type_line)
            if bl:
                potential += bl.any_color_count + len(bl.pool)
                continue
            tm = get_tap_mana_ability(p.card.ast)
            if tm:
                potential += tm.any_color_count + len(tm.pool)
        elif p.is_creature and not p.summoning_sick:
            tm = get_tap_mana_ability(p.card.ast)
            if tm:
                potential += tm.any_color_count + len(tm.pool)
        elif p.is_artifact:
            potential += _artifact_mana_potential(p)
    return pool + potential


def _artifact_mana_potential(p: "Permanent") -> int:
    """Expected pip count if we were to tap this artifact. Named tokens
    and well-known fast-mana cards have explicit pip counts; generic
    artifacts derive from their AST's tap-add-mana ability."""
    name = p.card.name
    tl = (p.card.type_line or "").lower()
    if "treasure" in tl or name == "Treasure Token":
        return 1
    if "powerstone" in tl or name == "Powerstone Token":
        return 1
    if name == "Gold Token":
        return 1
    if name == "Sol Ring":
        return 2
    if name in ("Mana Vault", "Grim Monolith"):
        return 3
    if name == "Mana Crypt":
        return 2
    if name == "Basalt Monolith":
        return 3
    if name == "Thran Dynamo":
        return 3
    if name == "Lotus Petal":
        return 1
    if name == "Jeweled Lotus":
        return 3
    if name == "Lion's Eye Diamond":
        return 3
    tm = get_tap_mana_ability(p.card.ast)
    if tm:
        return tm.any_color_count + len(tm.pool)
    return 0


def _pay_generic_cost(game: Game, seat: Seat, amount: int,
                      reason: str, card_name: str,
                      spell_type: str = "generic") -> bool:
    """Drain pool first, then tap lands/dorks/artifacts for the
    remainder. Returns True on success (mutates state), False on failure
    (no-op).

    Now artifact-source-aware and typed-pool-aware. Accepts an optional
    spell_type for §106.4a restriction compatibility (Food Chain mana
    can't pay Dark Ritual; Powerstone mana can't pay a creature spell).
    """
    if amount <= 0:
        return True
    # Fast path: pool already has enough under restriction rules.
    if seat.mana.can_pay_generic(amount, spell_type=spell_type):
        return _debit_generic_from_pool(game, seat, amount, reason,
                                        card_name, spell_type)
    # Slow path: tap sources until we can pay. Include artifacts.
    shortfall_needed = amount - seat.mana.total()
    if shortfall_needed < 0:
        shortfall_needed = 0
    to_tap: list[tuple[Permanent, int, str]] = []
    for p in seat.battlefield:
        if shortfall_needed <= 0:
            break
        if p.tapped:
            continue
        pips = 0
        src = ""
        if p.is_land:
            bl = basic_land_mana(p.card.type_line)
            if bl:
                pips = bl.any_color_count + len(bl.pool)
                src = "land"
            else:
                tm = get_tap_mana_ability(p.card.ast)
                if tm:
                    pips = tm.any_color_count + len(tm.pool)
                    src = "land_activated"
        elif p.is_creature and not p.summoning_sick:
            tm = get_tap_mana_ability(p.card.ast)
            if tm:
                pips = tm.any_color_count + len(tm.pool)
                src = "mana_dork"
        elif p.is_artifact:
            # Skip artifacts with destructive costs (e.g. Lion's Eye
            # Diamond's discard-your-hand) in the auto-pay path. These
            # should only be activated by explicit policy decisions,
            # not blind pay-for-cost logic — discarding the defender's
            # hand to pay for a counterspell would silently destroy
            # the very card being cast.
            if _artifact_has_destructive_cost(p):
                pips = 0
            else:
                pips = _artifact_mana_potential(p)
                src = "artifact"
        if pips > 0:
            to_tap.append((p, pips, src))
            shortfall_needed -= pips
    if shortfall_needed > 0:
        return False
    # Fill the pool by tapping the selected sources.
    for p, pips, src in to_tap:
        if src == "land":
            bl = basic_land_mana(p.card.type_line)
            p.tapped = True
            add_mana_from_ability(game, seat, bl,
                                   source_name=p.card.name,
                                   source_kind="land")
        elif src == "land_activated":
            tm = get_tap_mana_ability(p.card.ast)
            p.tapped = True
            add_mana_from_ability(game, seat, tm,
                                   source_name=p.card.name,
                                   source_kind="land_activated")
        elif src == "mana_dork":
            tm = get_tap_mana_ability(p.card.ast)
            p.tapped = True
            add_mana_from_ability(game, seat, tm,
                                   source_name=p.card.name,
                                   source_kind="mana_dork")
        elif src == "artifact":
            _try_tap_artifact_for_mana(game, seat, p)
    # Verify and debit.
    if not seat.mana.can_pay_generic(amount, spell_type=spell_type):
        return False
    return _debit_generic_from_pool(game, seat, amount, reason,
                                    card_name, spell_type)


def _debit_generic_from_pool(game: Game, seat: Seat, amount: int,
                              reason: str, card_name: str,
                              spell_type: str) -> bool:
    """Helper: debit `amount` generic pips from the typed pool, respecting
    restrictions. Emits a pay_mana event with per-bucket breakdown."""
    pool_before = seat.mana.total()
    breakdown: dict[str, int] = {}
    for _ in range(amount):
        bucket = _debit_generic_pip(seat, spell_type)
        if bucket is None:
            return False
        breakdown[bucket] = breakdown.get(bucket, 0) + 1
    game.ev("pay_mana", seat=seat.idx, amount=amount,
            reason=reason, card=card_name,
            breakdown=breakdown,
            pool_before=pool_before, pool_after=seat.mana.total())
    return True


def _stack_item_threat_score(item: "StackItem") -> int:
    """Threat heuristic: counter anything with score >= 3."""
    card = item.card
    score = 0
    if card.cmc >= 4:
        score += 3
    elif card.cmc >= 2:
        score += 1
    for eff in item.effects:
        k = getattr(eff, "kind", None)
        if k == "damage":
            score += 3
        elif k in ("destroy", "exile", "bounce"):
            score += 3
        elif k == "draw":
            score += 2
        elif k == "tutor":
            score += 4
        elif k == "counter_spell":
            score += 0
        elif k == "win_game":
            score += 10
    if "creature" in card.type_line.lower():
        pt = (card.power or 0) + (card.toughness or 0)
        if pt >= 6:
            score += 3
        elif pt >= 4:
            score += 2
        elif pt >= 2:
            score += 1
    return score


def _find_counter_in_hand(game: Game, seat: Seat) -> Optional[CardEntry]:
    """Pick the cheapest instant counterspell in hand we can afford."""
    candidates: list[CardEntry] = []
    for c in seat.hand:
        if not _is_instant(c):
            continue
        if not _card_has_counterspell(c):
            continue
        if _available_mana(seat) < c.cmc:
            continue
        candidates.append(c)
    if not candidates:
        return None
    candidates.sort(key=lambda c: (c.cmc, c.name))
    return candidates[0]


def _split_second_active(game: Game) -> bool:
    """CR 702.61a — Split second is a static ability that functions only
    while the spell with split second is on the stack. As long as that
    spell is on the stack, "players can't cast other spells or activate
    abilities that aren't mana abilities." We walk the entire stack
    (not just the top) because the split-second spell might have other
    items stacked on top of it (e.g. its own triggered abilities, or
    a mana ability's resolution), and it still forbids responses.

    We detect split second by the Keyword node on the AST, matching any
    keyword name whose normalized form contains the token "split"
    (canonical parser name is 'split_second').
    """
    for item in game.stack:
        if item.countered:
            continue
        for ab in item.card.ast.abilities:
            if isinstance(ab, Keyword):
                name = (ab.name or "").lower()
                if "split" in name:
                    return True
    return False


def _opp_restricts_defender_to_sorcery_speed(game: Game,
                                             defender_seat: int) -> bool:
    """Detect a Teferi, Time Raveler-style static restriction that forbids
    the defender from casting instants (or any spell at non-sorcery speed).

    CR 601.3a — "If an effect prohibits a player from casting a spell with
    certain qualities, that player [can't cast it]." CR 307.1 defines
    sorcery timing: a sorcery may only be cast during its controller's
    main phase, with an empty stack. We're being asked to pick a response
    to a spell on the stack — by definition the stack is non-empty, so
    any spell the defender casts here is cast at instant speed. If a
    static ability restricts the defender to sorcery speed, the defender
    cannot legally respond.

    Scans every opponent's battlefield for a Static ability whose
    Modification.kind is an "opp_sorcery_speed_only" variant (the parser
    tags Teferi's text this way via extensions/stack_timing.py). The
    static is only relevant when controlled by a player ≠ defender_seat
    (a player isn't restricted by their own Teferi).
    """
    for seat in game.seats:
        if seat.idx == defender_seat:
            continue
        for perm in seat.battlefield:
            for ab in perm.card.ast.abilities:
                if not isinstance(ab, Static):
                    continue
                mod = ab.modification
                if mod is None:
                    continue
                if mod.kind in ("opp_sorcery_speed_only",
                                "cast_timing_opp_sorcery",
                                "opp_only_sorcery_speed"):
                    return True
                # Fall back to raw-text match for parser variants.
                raw = (ab.raw or "").lower()
                if ("each opponent can cast spells only any time they "
                        "could cast a sorcery") in raw:
                    return True
    return False


def _get_response(game: Game, defender_seat: int,
                  top: "StackItem") -> Optional[CardEntry]:
    """Defender-policy hook. Returns a card to cast in response, or None.

    The legality guards (CR 702.61a split-second, Teferi-style
    sorcery-speed restrictions, self-counter-own-spell) are enforced by
    the engine irrespective of policy — the policy only decides whether
    to burn a counter / response card given a legal opportunity.
    Policies may OVERRIDE the engine's filters by returning a card
    anyway; the engine still requires the resulting cast to be
    mana-affordable, which ``_priority_round`` checks post-policy.
    """
    seat = game.seats[defender_seat]
    if seat.policy is None:
        return None
    return seat.policy.respond_to_stack_item(game, seat, top)


# ============================================================================
# Loop classification — CR 731 ("Taking Shortcuts") + CR 104.3a / CR 104.4b
# ============================================================================
#
# When the engine (or a harness) detects a repeating game state, the comp
# rules prescribe an outcome based on whether the loop is *optional* (some
# participating action involves a player choice) or *mandatory* (every
# action is forced — only triggered/replacement effects, no activated
# abilities or casting decisions):
#
#   CR 731.1b  "Occasionally the game gets into a state in which a set of
#              actions could be repeated indefinitely (thus creating a
#              'loop'). In that case, the shortcut rules can be used to
#              determine how many times those actions are repeated without
#              having to actually perform them, and how the loop is broken."
#   CR 731.4   "If a loop contains only mandatory actions, the game is a
#              draw. (See rules 104.4b and 104.4f.)"
#   CR 104.4b  "If a game ... somehow enters a 'loop' of mandatory actions,
#              repeating a sequence of events with no way to stop, the game
#              is a draw. Loops that contain an optional action don't
#              result in a draw."
#   CR 104.3a  "A player can concede the game at any time. [...] If a player
#              can't perform a required game action they lose the game."
#              — relevant when a one-sided mandatory loop runs the
#              controller out of a required resource.
#
# The user-prompt used "716" as shorthand for the loop rule; the actual
# section in the 2026-02-27 comprehensive rules is 731 ("Taking Shortcuts").
# Rule 716 is "Class Cards" and has nothing to do with loops. We keep
# the helper name the call-sites use but the citations refer to 731.

def classify_loop_716(game: Game, repeating_state: dict) -> dict:
    """Classify a detected repeating game state per CR 731 + CR 104.3a.

    `repeating_state` is a free-form dict describing the loop:
      - `optional_actions` : list of actions that involve a player choice
                             (activated abilities, cast decisions, mode
                             choices, optional triggered abilities). Even
                             one optional action makes the whole loop
                             optional per CR 731.6 / 104.4b.
      - `mandatory_actions`: list of actions that are forced (mandatory
                             triggered abilities, replacement effects).
      - `both_sided`       : True if participation inextricably requires
                             actions from both players (e.g. Sanguine
                             Bond + Exquisite Blood: each cascade trigger
                             alternates between the two players).
      - `controller_seat`  : integer seat of the loop's controller, for
                             one-sided mandatory loops.
      - `iteration_count`  : caller's announced iteration budget (CR
                             731.2) for optional loops.

    Returns a dict with:
      - `kind`    : "optional" | "mandatory_two_sided" | "mandatory_one_sided"
      - `outcome` : "pass_with_controller_choice" | "draw" | "controller_loses"
      - `citation`: the CR text justifying the outcome
      - `iteration_count` (optional loops only)
    """
    optional = list(repeating_state.get("optional_actions", []))
    mandatory = list(repeating_state.get("mandatory_actions", []))
    both_sided = bool(repeating_state.get("both_sided", False))
    iteration_count = repeating_state.get("iteration_count")

    if optional:
        # CR 104.4b second sentence: "Loops that contain an optional action
        # don't result in a draw." CR 731.2 shortcut: the controller
        # announces a finite number of iterations.
        return {
            "kind": "optional",
            "outcome": "pass_with_controller_choice",
            "iteration_count": iteration_count,
            "citation": (
                "CR 104.4b: 'Loops that contain an optional action don't "
                "result in a draw.' CR 731.2: controller announces a finite "
                "iteration count via the shortcut system."
            ),
        }

    # No optional actions — every step is forced.
    if mandatory and both_sided:
        # CR 731.4 / CR 104.4b: mandatory-only loop → draw.
        return {
            "kind": "mandatory_two_sided",
            "outcome": "draw",
            "citation": (
                "CR 731.4: 'If a loop contains only mandatory actions, the "
                "game is a draw.' CR 104.4b: the game is a draw when the "
                "mandatory loop spans both players with no way to stop."
            ),
        }

    if mandatory and not both_sided:
        # One-sided mandatory loop: the controller cannot progress, so
        # they lose under CR 104.3a (unable to perform a required action).
        # CR 731.4 still applies to the loop itself, but with no other
        # player involved the game-end-by-draw clause doesn't kick in —
        # the controller simply runs out of a required resource (typical
        # Worldgorger Dragon + Animate Dead outcome when the controller
        # has no way to break the loop with a stack response).
        return {
            "kind": "mandatory_one_sided",
            "outcome": "controller_loses",
            "citation": (
                "CR 104.3a: a player unable to perform a required action "
                "loses the game. For a one-sided mandatory loop with no "
                "opposing participation and no break, the controller "
                "cannot progress and therefore loses."
            ),
        }

    # Degenerate case: no actions detected at all. Treat as optional pass.
    return {
        "kind": "optional",
        "outcome": "pass_with_controller_choice",
        "iteration_count": iteration_count,
        "citation": "CR 731.2 shortcut fallback — no mandatory actions detected.",
    }


# ============================================================================
# Cast-count + Storm infrastructure (CR §702.40, §700.4)
# ============================================================================
#
# Every spell cast increments:
#   - game.spells_cast_this_turn   (global, used by Storm)
#   - seat.spells_cast_this_turn   (per-seat, used by Storm-Kiln Artist,
#                                    Young Pyromancer, Birgi, Monastery
#                                    Mentor, Niv-Mizzet, Runaway Steam-Kin)
#
# CR §706.10: a copy of a spell is NOT cast. Storm copies, Twinflame copies,
# Dualcaster Mage copies — none of them increment the cast counters, trigger
# cast-based observers, or themselves trigger Storm.
#
# CR §702.40a: "Storm is a triggered ability. 'Storm' means 'When you cast
# this spell, copy it for each other spell cast before it this turn. You may
# choose new targets for the copies.'" — The "before it" wording is why we
# copy (spells_cast_this_turn - 1) times: the storm spell itself has ALREADY
# been counted by the time the trigger resolves, so subtract 1 to get the
# count of spells cast before it.
# ----------------------------------------------------------------------------


# Cards whose cast-trigger observer behavior we implement inline as a bridge
# until a per-card handler agent wires them up through the normal
# per_card_runtime registry. Keyed by card name; the body of the function
# runs AFTER the controller has paid costs and the card is on the stack
# (but before priority opens), which is functionally equivalent to the
# trigger going onto the stack above the cast and resolving before the
# original spell (the "trigger above spell" ordering since triggers go on
# top). For the small number of observers we care about right now (mana-
# gen, token-create, +1/+1-counter-accumulate) the ordering difference is
# not gameplay-visible.
#
# IMPORTANT: these handlers read the CAST spell (the spell being cast now),
# not the stack top's card. The per-card handler agent is expected to port
# these into the extensions/per_card*.py registry and convert them to
# proper Triggered nodes emitted by the parser. Until then, this is our
# bridge so storm decks can be benchmarked.

def _fire_cast_trigger_observers(game: "Game", cast_card: "CardEntry",
                                  controller: int,
                                  from_copy: bool = False) -> None:
    """Fire 'whenever you cast a spell' / 'whenever a player casts a spell'
    style triggers for every permanent on the battlefield.

    Only runs for real casts (CR §601) — copies don't trigger these per
    §706.10, so callers pass from_copy=True to no-op. Likewise this is not
    called for mana abilities or triggered/activated abilities (those don't
    go through cast_spell).
    """
    if from_copy:
        return
    # Aggregate salient tags about the spell being cast.
    type_line = (cast_card.type_line or "").lower()
    is_instant = "instant" in type_line
    is_sorcery = "sorcery" in type_line
    is_creature = "creature" in type_line
    is_noncreature = not is_creature
    colors = tuple(cast_card.colors or ())
    is_red = "R" in colors
    is_instant_or_sorcery = is_instant or is_sorcery

    # Walk every battlefield permanent (all seats — cast triggers that key on
    # "whenever YOU cast" filter on controller, "whenever a player casts"
    # fires for everyone).
    for seat in game.seats:
        for perm in list(seat.battlefield):
            name = perm.card.name
            # --------------------------------------------------------------
            # Storm-Kiln Artist (cEDH storm staple)
            # "Whenever you cast an instant or sorcery spell, create a
            #  Treasure token."
            # --------------------------------------------------------------
            if name == "Storm-Kiln Artist":
                if perm.controller == controller and is_instant_or_sorcery:
                    _create_treasure_token(game, perm.controller)
                    game.ev("cast_trigger_observer",
                            source=name,
                            seat=perm.controller,
                            cast=cast_card.name,
                            effect="treasure_token")
            # --------------------------------------------------------------
            # Young Pyromancer
            # "Whenever you cast an instant or sorcery spell, create a 1/1
            #  red Elemental creature token."
            # --------------------------------------------------------------
            elif name == "Young Pyromancer":
                if perm.controller == controller and is_instant_or_sorcery:
                    _create_simple_creature_token(
                        game, perm.controller, "Elemental Token",
                        power=1, toughness=1, colors=("R",))
                    game.ev("cast_trigger_observer",
                            source=name,
                            seat=perm.controller,
                            cast=cast_card.name,
                            effect="elemental_token")
            # --------------------------------------------------------------
            # Third Path Iconoclast
            # "Whenever you cast a noncreature spell, create a 1/1 colorless
            #  Soldier artifact creature token."
            # --------------------------------------------------------------
            elif name == "Third Path Iconoclast":
                if perm.controller == controller and is_noncreature:
                    _create_simple_creature_token(
                        game, perm.controller, "Soldier Artifact Token",
                        power=1, toughness=1, colors=())
                    game.ev("cast_trigger_observer",
                            source=name,
                            seat=perm.controller,
                            cast=cast_card.name,
                            effect="soldier_token")
            # --------------------------------------------------------------
            # Monastery Mentor
            # "Prowess. Whenever you cast a noncreature spell, create a 1/1
            #  white Human Monk creature token with prowess."
            # --------------------------------------------------------------
            elif name == "Monastery Mentor":
                if perm.controller == controller and is_noncreature:
                    _create_simple_creature_token(
                        game, perm.controller, "Monk Token",
                        power=1, toughness=1, colors=("W",))
                    game.ev("cast_trigger_observer",
                            source=name,
                            seat=perm.controller,
                            cast=cast_card.name,
                            effect="monk_token")
            # --------------------------------------------------------------
            # Runaway Steam-Kin
            # "Whenever you cast a red spell, if Runaway Steam-Kin has fewer
            #  than three +1/+1 counters on it, put a +1/+1 counter on it.
            #  Remove three +1/+1 counters from Runaway Steam-Kin: Add {R}{R}
            #  {R}."
            # --------------------------------------------------------------
            elif name == "Runaway Steam-Kin":
                if perm.controller == controller and is_red:
                    current = perm.counters.get("+1/+1", 0) if hasattr(perm, "counters") else 0
                    if current < 3:
                        do_put_counter(game, perm, "+1/+1", 1)
                        game.ev("cast_trigger_observer",
                                source=name,
                                seat=perm.controller,
                                cast=cast_card.name,
                                effect="plus_one_counter")
            # --------------------------------------------------------------
            # Birgi, God of Storytelling // Harnfel, Horn of Bounty (front
            # face)
            # "Whenever you cast a spell, add {R}."
            # --------------------------------------------------------------
            elif name == "Birgi, God of Storytelling":
                if perm.controller == controller:
                    game.seats[perm.controller].mana_pool += 1
                    game.ev("cast_trigger_observer",
                            source=name,
                            seat=perm.controller,
                            cast=cast_card.name,
                            effect="add_mana_R")
            # --------------------------------------------------------------
            # Niv-Mizzet, Parun
            # "Whenever you cast an instant or sorcery spell, draw a card.
            #  Whenever you draw a card, Niv-Mizzet, Parun deals 1 damage
            #  to any target."
            # (The draw-trigger side is left to the existing draw-trigger
            #  infrastructure; we only wire the cast-trigger draw here.)
            # --------------------------------------------------------------
            elif name == "Niv-Mizzet, Parun":
                if perm.controller == controller and is_instant_or_sorcery:
                    draw_cards(game, game.seats[perm.controller], 1)
                    game.ev("draw",
                            seat=perm.controller, count=1,
                            hand_size=len(game.seats[perm.controller].hand),
                            source=name,
                            reason="niv_mizzet_parun")
                    game.ev("cast_trigger_observer",
                            source=name,
                            seat=perm.controller,
                            cast=cast_card.name,
                            effect="draw_card")
                    _fire_draw_trigger_observers(game,
                                                 perm.controller, count=1)
            # --------------------------------------------------------------
            # Rhystic Study (Wave 1b — the "bless you" check)
            # "Whenever an opponent casts a spell, that player may pay {1}.
            #  If that player doesn't, you draw a card."
            #
            # Policy for "unless they pay {1}" (mirrors Go): greedy pay
            # when affordable — opponent spends 1 generic mana from their
            # pool, no draw. Otherwise controller draws a card. Future Hat
            # integration can override.
            # --------------------------------------------------------------
            elif name == "Rhystic Study":
                if perm.controller != controller:
                    opp = game.seats[controller]
                    if opp.mana.total() >= 1:
                        opp.mana_pool = opp.mana.total() - 1
                        game.ev("pay_mana",
                                seat=controller, amount=1,
                                reason="rhystic",
                                source=name,
                                card_name=cast_card.name)
                        game.ev("cast_trigger_observer",
                                source=name,
                                seat=perm.controller,
                                cast=cast_card.name,
                                effect="rhystic_tax_paid",
                                caster_seat=controller)
                    else:
                        draw_cards(game, game.seats[perm.controller], 1)
                        game.ev("draw",
                                seat=perm.controller, count=1,
                                hand_size=len(game.seats[perm.controller].hand),
                                source=name,
                                reason="rhystic_study")
                        game.ev("cast_trigger_observer",
                                source=name,
                                seat=perm.controller,
                                cast=cast_card.name,
                                effect="rhystic_draw",
                                caster_seat=controller)
                        _fire_draw_trigger_observers(game,
                                                     perm.controller,
                                                     count=1)
            # --------------------------------------------------------------
            # Mystic Remora (Wave 1b)
            # "Cumulative upkeep {1}. Whenever an opponent casts a
            #  noncreature spell, that player may pay {4}. If the player
            #  doesn't, you draw a card."
            #
            # The cumulative upkeep side is handled elsewhere; this wires
            # the cast-trigger "draw unless {4}" half.
            # --------------------------------------------------------------
            elif name == "Mystic Remora":
                if perm.controller != controller and is_noncreature:
                    opp = game.seats[controller]
                    if opp.mana.total() >= 4:
                        opp.mana_pool = opp.mana.total() - 4
                        game.ev("pay_mana",
                                seat=controller, amount=4,
                                reason="remora",
                                source=name,
                                card_name=cast_card.name)
                        game.ev("cast_trigger_observer",
                                source=name,
                                seat=perm.controller,
                                cast=cast_card.name,
                                effect="remora_tax_paid",
                                caster_seat=controller)
                    else:
                        draw_cards(game, game.seats[perm.controller], 1)
                        game.ev("draw",
                                seat=perm.controller, count=1,
                                hand_size=len(game.seats[perm.controller].hand),
                                source=name,
                                reason="mystic_remora")
                        game.ev("cast_trigger_observer",
                                source=name,
                                seat=perm.controller,
                                cast=cast_card.name,
                                effect="remora_draw",
                                caster_seat=controller)
                        _fire_draw_trigger_observers(game,
                                                     perm.controller,
                                                     count=1)
            # --------------------------------------------------------------
            # Esper Sentinel (Wave 1b)
            # "Whenever an opponent casts their first noncreature spell
            #  each turn, unless that player pays {X}, where X is the
            #  number of creatures you control, you draw a card."
            #
            # "First noncreature spell each turn" is tracked per opponent
            # via seat.first_noncreature_cast_triggered[turn] — a flag
            # that flips True on first hit each opponent has this turn.
            # --------------------------------------------------------------
            elif name == "Esper Sentinel":
                if perm.controller != controller and is_noncreature:
                    opp = game.seats[controller]
                    # Per-turn per-opponent first-noncreature tracker.
                    tracker = getattr(opp, "_esper_sentinel_fired_turn", -1)
                    if tracker != game.turn:
                        # First noncreature spell this opponent casts this
                        # turn. X = number of creatures controller has.
                        x_cost = sum(
                            1 for p in game.seats[perm.controller].battlefield
                            if p.is_creature
                        )
                        opp._esper_sentinel_fired_turn = game.turn
                        if opp.mana.total() >= x_cost and x_cost > 0:
                            opp.mana_pool = opp.mana.total() - x_cost
                            game.ev("pay_mana",
                                    seat=controller, amount=x_cost,
                                    reason="sentinel",
                                    source=name,
                                    card_name=cast_card.name)
                            game.ev("cast_trigger_observer",
                                    source=name,
                                    seat=perm.controller,
                                    cast=cast_card.name,
                                    effect="sentinel_tax_paid",
                                    caster_seat=controller, x=x_cost)
                        else:
                            draw_cards(game,
                                       game.seats[perm.controller], 1)
                            game.ev("draw",
                                    seat=perm.controller, count=1,
                                    hand_size=len(
                                        game.seats[perm.controller].hand),
                                    source=name,
                                    reason="esper_sentinel")
                            game.ev("cast_trigger_observer",
                                    source=name,
                                    seat=perm.controller,
                                    cast=cast_card.name,
                                    effect="sentinel_draw",
                                    caster_seat=controller, x=x_cost)
                            _fire_draw_trigger_observers(game,
                                                         perm.controller,
                                                         count=1)


def _fire_draw_trigger_observers(game: "Game", drawer_seat: int,
                                  count: int = 1,
                                  from_replacement: bool = False) -> None:
    """Fire 'whenever a player draws a card' style reactive triggers.

    Walks every battlefield permanent looking for cards that care about
    an opponent (or any player) drawing a card. Covers:

      - Smothering Tithe: "Whenever an opponent draws a card, that
        player may pay {2}. If the player doesn't, you create a Treasure
        token."
      - Orcish Bowmasters: "When Orcish Bowmasters enters the battle-
        field and whenever an opponent draws a card except the first
        one they draw in each of their draw steps, create a 1/1 black
        Zombie Archer token with 'This creature gets +1/+1 as long as
        its controller has more cards in hand than each opponent.' Then
        Orcish Bowmasters and each other Army you control deal 1 damage
        to any target."
      - Notion Thief: "If an opponent would draw a card except the first
        one they draw in each of their draw steps, instead that player
        skips that draw and you draw a card." — this is a REPLACEMENT,
        not a trigger. We honor it as a trigger-shaped observer for the
        analyze harness's visibility but defer actual replacement
        semantics. Guarded by from_replacement=False to avoid double-
        firing if the replacement path wires it up later.

    Args:
        game:             the game instance
        drawer_seat:      seat index of the player drawing
        count:            number of cards drawn in this single draw event
                          (we fire the trigger `count` times for card-by-
                          card handlers like Bowmasters — CR §614.6
                          "applied once per draw").
        from_replacement: when True, suppress triggers that are logically
                          handled at the replacement layer (e.g. Notion
                          Thief's "instead that player skips that draw").

    The main turn draw (CR §504.a) is the "first draw in each of their
    draw steps" — Bowmasters + Notion Thief DON'T fire on that draw.
    Callers that are emitting the turn-draw pass is_turn_draw via the
    draw event itself; we read it back from context in callers.
    """
    # Map seat to its "has taken turn draw this step" flag. Bowmasters
    # + Notion Thief's "except the first one in each of their draw
    # steps" text. The caller tags the turn draw via
    # game._suppress_first_draw_trigger = drawer_seat for the single
    # upkeep/draw-step fire; we consume + clear that marker here.
    skip_turn_draw_observers = (
        getattr(game, "_suppress_first_draw_trigger", None) == drawer_seat
    )
    if skip_turn_draw_observers:
        # Consume the marker so only the actual turn-draw gets the pass.
        game._suppress_first_draw_trigger = None

    for _ in range(max(1, count)):
        for seat in game.seats:
            for perm in list(seat.battlefield):
                name = perm.card.name
                # ----------------------------------------------------
                # Smothering Tithe
                # ----------------------------------------------------
                if name == "Smothering Tithe":
                    if perm.controller == drawer_seat:
                        continue  # opponent-only
                    opp = game.seats[drawer_seat]
                    if opp.mana.total() >= 2:
                        opp.mana_pool = opp.mana.total() - 2
                        game.ev("pay_mana",
                                seat=drawer_seat, amount=2,
                                reason="tithe",
                                source=name)
                        game.ev("draw_trigger_observer",
                                source=name,
                                seat=perm.controller,
                                drawer_seat=drawer_seat,
                                effect="tithe_tax_paid")
                    else:
                        _create_treasure_token(game, perm.controller)
                        game.ev("draw_trigger_observer",
                                source=name,
                                seat=perm.controller,
                                drawer_seat=drawer_seat,
                                effect="tithe_treasure")
                # ----------------------------------------------------
                # Orcish Bowmasters — skip turn-draw-step first draw.
                # ----------------------------------------------------
                elif name == "Orcish Bowmasters":
                    if perm.controller == drawer_seat:
                        continue  # opponent-only
                    if skip_turn_draw_observers:
                        continue  # first draw of drawer's draw step
                    # Make a 1/1 Zombie Archer token + deal 1 damage to any
                    # target. Target picked via the standard opponent
                    # threat heuristic.
                    _create_simple_creature_token(
                        game, perm.controller, "Zombie Archer Token",
                        power=1, toughness=1, colors=("B",))
                    # Damage 1 to an opponent (or the drawer).
                    tgt = game.seats[drawer_seat]
                    tgt.life -= 1
                    game.ev("damage", amount=1,
                            target_kind="player",
                            target_seat=drawer_seat,
                            source_card=name)
                    game.ev("life_change", seat=drawer_seat,
                            amount=-1, new_life=tgt.life,
                            source_card=name)
                    game.ev("draw_trigger_observer",
                            source=name,
                            seat=perm.controller,
                            drawer_seat=drawer_seat,
                            effect="bowmasters_ping")


def _create_treasure_token(game: "Game", seat_idx: int) -> None:
    """Create a Treasure artifact token under `seat_idx`'s control.

    MVP: a Treasure token grants {T}, sacrifice: add one mana of any color.
    We model the mana-ability as +1 to the mana pool on demand via the
    normal tap-mana scanner — for cast-count infrastructure purposes the
    presence of the Treasure is what matters. The existing fill_mana_pool
    path will pick it up once it recognizes Treasure's tap ability.

    For now we just append a Permanent to the battlefield with minimal
    metadata. A fuller Treasure model is a separate task.
    """
    # Build a lightweight CardEntry for the token. CardAST is intentionally
    # empty — the Permanent is just a marker for cast-count observers that
    # count Treasures (Hullbreaker Horror, Smothering Tithe payoff, etc.).
    token_card = CardEntry(
        name="Treasure Token",
        mana_cost="",
        cmc=0,
        type_line="Token Artifact — Treasure",
        oracle_text="{T}, Sacrifice this token: Add one mana of any color.",
        power=None,
        toughness=None,
        ast=CardAST(name="Treasure Token", abilities=(),
                    parse_errors=(), fully_parsed=True),
        colors=(),
    )
    perm = Permanent(card=token_card, controller=seat_idx,
                     tapped=False, summoning_sick=False)
    game.seats[seat_idx].battlefield.append(perm)
    game.ev("create_token", seat=seat_idx, token="Treasure Token")


def _create_meteorite_token(game: "Game", seat_idx: int,
                             etb_damage_target=None) -> None:
    """Create a Meteorite artifact token under `seat_idx`'s control (OTJ).

    Unlike Treasure (tap-sac-one-shot), Meteorite is a PERMANENT mana rock
    that stays on the battlefield:
      - ETB: deals 2 damage to any target (pilot-chosen)
      - {T}: Add one mana of any color (pilot chooses at spend time via
        the typed mana pool's 'any' bucket — the tap handler routes into
        _try_tap_artifact_for_mana's name-based dispatch)

    Generated by OTJ cards like Prosperous Bandit, Outlaws' Nest, etc.

    Args:
        etb_damage_target: optional pre-chosen target for the ETB damage.
            If None, a default opponent is picked via the standard
            threat-based selector. Pilot/Hat ChooseTarget gets first say.
    """
    token_card = CardEntry(
        name="Meteorite Token",
        mana_cost="",
        cmc=0,
        type_line="Token Artifact",
        oracle_text=(
            "When Meteorite enters the battlefield, it deals 2 damage to "
            "any target. {T}: Add one mana of any color."
        ),
        power=None,
        toughness=None,
        ast=CardAST(name="Meteorite Token", abilities=(),
                    parse_errors=(), fully_parsed=True),
        colors=(),
    )
    perm = Permanent(card=token_card, controller=seat_idx,
                     tapped=False, summoning_sick=False)
    game.seats[seat_idx].battlefield.append(perm)
    game.ev("create_token", seat=seat_idx, token="Meteorite Token")

    # ETB: 2 damage to any target. For MVP we pick an opponent via the
    # standard threat-based selector (same as Lightning Bolt would).
    if etb_damage_target is None:
        target_seat = _pick_opponent_by_threat(game, seat_idx)
    else:
        target_seat = etb_damage_target
    if target_seat is not None:
        _apply_damage_to_player(
            game,
            perm,  # source permanent (the Meteorite)
            2,
            target_seat.idx if hasattr(target_seat, "idx") else target_seat,
        )


def _create_simple_creature_token(game: "Game", seat_idx: int, name: str,
                                   power: int, toughness: int,
                                   colors: tuple = ()) -> None:
    """Create a vanilla creature token. Used by Young Pyromancer,
    Third Path Iconoclast, Monastery Mentor cast-trigger handlers."""
    type_line = "Token Creature"
    token_card = CardEntry(
        name=name,
        mana_cost="",
        cmc=0,
        type_line=type_line,
        oracle_text="",
        power=power,
        toughness=toughness,
        ast=CardAST(name=name, abilities=(), parse_errors=(),
                    fully_parsed=True),
        colors=colors,
    )
    # Summoning sick per §302.1 — haste not granted by default.
    perm = Permanent(card=token_card, controller=seat_idx,
                     tapped=False, summoning_sick=True)
    game.seats[seat_idx].battlefield.append(perm)
    game.ev("create_token", seat=seat_idx, token=name,
            power=power, toughness=toughness)


# Cards whose oracle text contains the Storm keyword (CR §702.40). We
# maintain an explicit lookup because the parser's Keyword node may not
# be emitted uniformly across all storm cards (some have a post-storm
# reminder-text paragraph the parser skips, some fold the storm word into
# a flavor clause). Every card in this set gets (spells_cast_this_turn - 1)
# copies added to the stack per §702.40a.
_STORM_CARDS = frozenset({
    "Grapeshot",
    "Tendrils of Agony",
    "Brain Freeze",
    "Mind's Desire",
    "Haze of Rage",
    "Inner Fire",
    "Flusterstorm",
    "Wing Shards",
    "Volcanic Awakening",
    "Empty the Warrens",
    "Maelstrom Nexus",  # cascade-as-storm variant
})


def _has_storm_keyword(card: "CardEntry") -> bool:
    """True iff the card carries Storm (CR §702.40).

    Prefers the explicit name set above (robust against parser drift), falls
    back to walking the AST for a Keyword(name='storm') node.
    """
    if card.name in _STORM_CARDS:
        return True
    for ab in card.ast.abilities:
        if isinstance(ab, Keyword) and ab.name.lower() == "storm":
            return True
    return False


def _apply_storm_copies(game: "Game", original: "StackItem",
                        controller: int) -> int:
    """Put (spells_cast_this_turn - 1) copies of `original` onto the stack.

    CR §702.40a: "Storm is a triggered ability. 'Storm' means 'When you cast
    this spell, copy it for each other spell cast before it this turn. You
    may choose new targets for the copies.'"

    Copies are NOT cast (§706.10 — a copy is created directly on the stack).
    They do not trigger Storm again, don't trigger cast-count observers,
    don't pay costs, and go to graveyard on resolution (or simply cease to
    exist, depending on rules; we route them to graveyard to keep the
    zone-conservation invariants satisfied — the copy's card_obj is a fresh
    in-memory CardEntry).

    Returns the number of copies made (useful for tests + logging).
    """
    if game.spells_cast_this_turn <= 1:
        # "spells_cast_this_turn - 1" would be 0 or negative; the storm
        # spell itself was just counted. No copies needed.
        return 0
    copies_to_make = game.spells_cast_this_turn - 1
    game.ev("storm_trigger",
            source=original.card.name,
            seat=controller,
            copies=copies_to_make,
            spells_cast_this_turn=game.spells_cast_this_turn,
            rule="702.40a")
    # Push copies on top of the original. LIFO resolution means copies
    # resolve first, then the original — CR §608.2 preserves normal stack
    # ordering, and storm copies go ON TOP of the spell that triggered
    # them per §405.2 (triggered abilities go on the stack above the spell
    # whose casting triggered them).
    for i in range(copies_to_make):
        # Fresh CardEntry so graveyard + resolution don't alias the same
        # object. Effects are a FRESH copy too: collect_spell_effects
        # builds new Effect instances from the frozen AST.
        copy_card = CardEntry(
            name=f"{original.card.name} (storm copy {i + 1})",
            mana_cost=original.card.mana_cost,
            cmc=0,  # copies don't cost anything to exist
            type_line=original.card.type_line,
            oracle_text=original.card.oracle_text,
            power=original.card.power,
            toughness=original.card.toughness,
            ast=original.card.ast,
            colors=original.card.colors,
        )
        effects = list(collect_spell_effects(original.card.ast))
        copy_item = StackItem(
            card=copy_card,
            controller=controller,
            is_permanent_spell=original.is_permanent_spell,
            effects=effects,
            is_copy=True,   # CR §706.10 — copy ceases to exist on resolution
        )
        # Mark the copy so downstream code paths (e.g. a future per-card
        # handler that cares "is this a copy?") can tell. The StackItem
        # dataclass doesn't have a native field for this; we stamp the name
        # prefix ("(storm copy N)") as the marker. That's enough to detect
        # a copy without touching the StackItem schema.
        game.stack.append(copy_item)
        game.ev("stack_push_storm_copy",
                card=copy_card.name,
                seat=controller,
                stack_size=len(game.stack))
    return copies_to_make


def _push_stack_item(game: Game, card: CardEntry,
                     controller: int) -> "StackItem":
    """Create and push a StackItem. Caller is responsible for paying costs."""
    is_permanent_spell = any(t in card.type_line.lower()
                             for t in ("creature", "artifact", "enchantment",
                                       "planeswalker"))
    effects = [] if is_permanent_spell else list(collect_spell_effects(card.ast))
    item = StackItem(card=card, controller=controller,
                     is_permanent_spell=is_permanent_spell,
                     effects=effects)
    game.stack.append(item)
    game.ev("stack_push", card=card.name, seat=controller,
            stack_size=len(game.stack), permanent=is_permanent_spell)
    game.emit(f"  push {card.name} onto stack (size {len(game.stack)})")
    return item


def _resolve_stack_top(game: Game) -> None:
    """Pop top of stack. Fizzle if countered, else apply effects or ETB."""
    if not game.stack:
        return
    item = game.stack.pop()
    game.ev("stack_resolve", card=item.card.name, seat=item.controller,
            countered=item.countered, stack_size=len(game.stack))

    if item.countered:
        game.seats[item.controller].graveyard.append(item.card)
        game.emit(f"  resolve: {item.card.name} was countered -> graveyard")
        game.ev("resolve", card=item.card.name, to="graveyard",
                seat=item.controller, countered=True)
        return

    if item.is_permanent_spell:
        sick = not has_keyword(item.card.ast, "haste")
        perm = Permanent(card=item.card, controller=item.controller,
                         tapped=False, summoning_sick=sick)
        # CR §706.10a — a copy of a permanent spell becomes a TOKEN when
        # it resolves. For the storm scope there are no permanent-typed
        # storm spells, but this keeps the is_copy invariant clean for
        # future copy-spell sources (Twinflame, Flameshadow Conjuring).
        if getattr(item, "is_copy", False):
            perm.card = item.card  # already a fresh CardEntry
            game.ev("token_from_copy", card=item.card.name,
                    seat=item.controller, rule="706.10a")
        _etb_initialize(game, perm)
        game.seats[item.controller].battlefield.append(perm)
        game.ev("enter_battlefield", card=item.card.name,
                seat=item.controller, summoning_sick=sick)
        # Per-card snowflake ETB handlers. Runs BEFORE the generic ETB
        # loop so that cards like Thassa's Oracle (which wins at ETB) or
        # Painter's Servant (which needs to stamp its chosen color
        # before any downstream effect reads it) apply their effect
        # before any other ETB resolution.
        apply_per_card_etb(game, perm)
        if game.ended:
            return
        for eff in collect_etb_effects(item.card.ast):
            resolve_effect(game, item.controller, eff,
                           source_colors_hint=item.card.colors)
            if game.ended:
                return
        _apply_self_etb_synthesized(game, perm)
        if game.ended:
            return
    else:
        for eff in item.effects:
            if getattr(eff, "kind", None) == "counter_spell":
                # MVP: counter whatever opponent spell sits directly below us.
                victim: Optional[StackItem] = None
                for s in reversed(game.stack):
                    if s.controller != item.controller and not s.countered:
                        victim = s
                        break
                game._pending_counter_target = victim
                resolve_effect(game, item.controller, eff,
                               source_colors_hint=item.card.colors)
                game._pending_counter_target = None
            else:
                resolve_effect(game, item.controller, eff,
                               source_colors_hint=item.card.colors)
            if game.ended:
                return
        # Per-card snowflake spell-resolution handlers. The ``effects``
        # list above is empty for cards whose oracle text parses only
        # into Static(custom) stubs (Doomsday, Demonic Consultation,
        # Tainted Pact). Firing these here is the missing bridge.
        apply_per_card_spell_effects(game, item.controller, item.card)
        if game.ended:
            return
        # CR §706.10 — a copy of a spell ceases to exist on resolution
        # rather than going to graveyard. Routing storm / twinflame /
        # dualcaster copies to graveyard would violate zone conservation
        # (copies aren't in any starting deck).
        if getattr(item, "is_copy", False):
            game.ev("resolve", card=item.card.name, to="ceases_to_exist",
                    seat=item.controller, rule="706.10")
        else:
            game.seats[item.controller].graveyard.append(item.card)
            game.ev("resolve", card=item.card.name, to="graveyard",
                    seat=item.controller)


def _priority_round(game: Game) -> None:
    """Post-cast instant window. CR §117 priority: after each spell
    hits the stack, priority passes in APNAP order (active player first,
    then each non-active seat in turn order).

    MVP policy: the caster always passes; every OPPONENT gets a chance
    to respond with a counterspell independently (no collusion).
    Opponents are polled in APNAP order from the current active seat.
    If any opponent responds, a new top-of-stack appears and we loop
    again — new stack item means a fresh priority round.

    N-seat aware: for 2 seats this collapses to the legacy behavior
    (single non-caster response window). For 3+ seats, every living
    opponent of the top's controller can fire a counter.
    """
    depth = 0
    # Depth cap prevents pathological counter-war loops. A well-behaved
    # AI will naturally resolve quickly; the cap catches bugs.
    while depth < 16 and game.stack and not game.ended:
        top = game.stack[-1]
        responded = False
        # APNAP order, anchored on the current active player.
        for seat_obj in game.apnap_order():
            if seat_obj.lost:
                continue
            if seat_obj.idx == top.controller:
                # Caster passes priority in the MVP.
                continue
            defender_seat = seat_obj.idx
            response_card = _get_response(game, defender_seat, top)
            if response_card is None:
                game.ev("priority_pass", seat=defender_seat)
                continue
            if not _pay_generic_cost(game, seat_obj, response_card.cmc,
                                     reason="cast",
                                     card_name=response_card.name):
                continue
            seat_obj.hand.remove(response_card)
            game.emit(
                f"seat {defender_seat} responds with {response_card.name} "
                f"-> counter {top.card.name}")
            game.ev("cast", card=response_card.name,
                    cmc=response_card.cmc, seat=defender_seat,
                    in_response_to=top.card.name)
            # CR §700.4 / §702.40 — response casts (counterspells) are
            # still casts per §601 and increment the per-turn cast
            # counter + fire reactive observers. Without this, Rhystic
            # Study / Mystic Remora / Esper Sentinel never see
            # counterspells and miss the "bless you" trigger — the
            # exact Wave 1b gap we're closing.
            game.spells_cast_this_turn += 1
            seat_obj.spells_cast_this_turn += 1
            _fire_cast_trigger_observers(game, response_card,
                                          defender_seat, from_copy=False)
            _push_stack_item(game, response_card, defender_seat)
            responded = True
            # A new stack item exists. Break out of the APNAP loop and
            # start a fresh priority round — the new top object may
            # itself be counterable by any opponent.
            break
        if not responded:
            # Everyone passed. Exit — resolution happens in cast_spell.
            break
        depth += 1


def cast_spell(game: Game, card: CardEntry) -> None:
    """Cast a spell: pay cost, push onto stack, open instant window for
    opponent response, then resolve top-down until stack is empty.

    Typed-pool-aware: if the card's AST carries a full ManaCost we
    route through pay_mana_cost which honors colored pips and spend
    restrictions. Otherwise we fall back to _pay_generic_cost on the
    CMC so legacy cards without parsed costs still cast."""
    s = game.seats[game.active]
    s.hand.remove(card)
    spell_type = _classify_spell_type(card)
    parsed_cost = _get_parsed_mana_cost(card)
    paid = False
    if parsed_cost is not None and parsed_cost.symbols:
        paid = pay_mana_cost(game, s, parsed_cost,
                             spell_type=spell_type,
                             reason="cast", card_name=card.name)
    if not paid:
        # Fallback: pay generic from pool via the untyped path.
        paid = _pay_generic_cost(game, s, card.cmc,
                                  reason="cast", card_name=card.name,
                                  spell_type=spell_type)
    if not paid:
        # Truly unable to pay — push card back to hand and abort cast.
        s.hand.append(card)
        game.ev("cast_failed", card=card.name, seat=game.active,
                reason="unpayable")
        return
    game.emit(f"cast {card.name} (cmc {card.cmc})")
    game.ev("cast", card=card.name, cmc=card.cmc, seat=game.active)

    # CR §700.4 / §702.40 cast-count bookkeeping. Increment BEFORE pushing
    # the spell onto the stack + storm copies, so the storm calculation
    # sees the storm spell itself already counted. Per §702.40a "copy it
    # for each OTHER spell cast before it this turn" — with the spell
    # counted, copies = count - 1.
    game.spells_cast_this_turn += 1
    s.spells_cast_this_turn += 1

    item = _push_stack_item(game, card, game.active)

    # CR §702.40 — fire storm trigger (copies are pushed on top; they will
    # resolve before the original because the stack is LIFO). _push_stack_
    # item has already emitted the cast event, so copies are logged
    # separately via stack_push_storm_copy.
    if _has_storm_keyword(card):
        _apply_storm_copies(game, item, game.active)

    # Fire cast-trigger observer permanents AFTER storm copies are queued.
    # (Placement relative to storm is a judgment call: strictly by the
    # rules, both storm and observer triggers go on the stack above the
    # spell that caused them, in APNAP order chosen by each controller.
    # For the observers we're modeling here — all of which just generate
    # a token / mana / counter — the ordering has no gameplay impact.)
    _fire_cast_trigger_observers(game, card, game.active, from_copy=False)

    _priority_round(game)
    while game.stack and not game.ended:
        _resolve_stack_top(game)
        state_based_actions(game)

    state_based_actions(game)
    game.snapshot()


def _enumerate_legal_activations(game: "Game", seat: "Seat") -> list:
    """Walk the seat's battlefield and return legal activated abilities.

    Returns a list of (permanent, ability_index) tuples. Filters out:
      - Mana abilities (tap-for-mana already fires via fill_mana_pool)
      - Loyalty abilities (planeswalkers — once-per-turn, need separate
        tracking we haven't built yet; deferred)
      - Already-activated once-per-turn abilities this turn
      - Abilities whose cost can't be paid right now

    Used by main_phase after the cast loop to surface activated-ability
    decision points to the Hat policy. Per 7174n1c 2026-04-16: wired so
    OctoHat's "activate everything" design principle actually fires.

    Hat sees a curated list; engine enforces legality.
    """
    options: list = []
    for perm in list(seat.battlefield):
        if perm.tapped and _ability_requires_untapped(perm):
            continue
        ast = perm.card.ast
        if ast is None or not ast.abilities:
            continue
        for idx, abil in enumerate(ast.abilities):
            # Only activated abilities (not keywords, triggered, static)
            if getattr(abil, "__class__", None) is None:
                continue
            if type(abil).__name__ != "Activated":
                continue
            # Skip mana abilities (already handled by fill_mana_pool).
            eff = getattr(abil, "effect", None)
            if eff is not None and type(eff).__name__ == "AddMana":
                continue
            # Respect tap-cost requirement (can't activate tapped creature
            # with a tap-cost ability)
            cost = getattr(abil, "cost", None)
            if cost is not None and getattr(cost, "tap", False) and perm.tapped:
                continue
            # Check summoning sickness for creature activations (except
            # non-tap-cost abilities which are legal regardless)
            if (perm.is_creature and perm.summoning_sick
                    and cost is not None and getattr(cost, "tap", False)):
                continue
            # Mana cost affordability check (approximate — just verify
            # total available mana covers the printed CMC of the cost)
            if cost is not None:
                mana_cost_obj = getattr(cost, "mana", None)
                if mana_cost_obj is not None:
                    symbols = getattr(mana_cost_obj, "symbols", ())
                    approx_cmc = sum(1 + getattr(sym, "generic", 0)
                                     for sym in symbols)
                    if approx_cmc > seat.mana.total():
                        continue
            options.append((perm, idx))
    return options


def _ability_requires_untapped(perm: "Permanent") -> bool:
    """Conservative check: does this permanent have at least one ability
    whose cost requires tapping? If so, skip re-surfacing when tapped."""
    ast = perm.card.ast
    if ast is None or not ast.abilities:
        return False
    for abil in ast.abilities:
        if type(abil).__name__ != "Activated":
            continue
        cost = getattr(abil, "cost", None)
        if cost is not None and getattr(cost, "tap", False):
            return True
    return False


def _resolve_activated_ability(game: "Game", seat: "Seat",
                                perm: "Permanent", abil_idx: int) -> None:
    """Pay the activation cost and resolve the effect for a chosen
    activated ability. Uses existing resolve_effect infrastructure for
    the effect body.

    Conservative MVP: pays mana cost + applies tap cost, then routes the
    effect through _resolve_spell_effect-equivalent logic. Does NOT yet
    handle sacrifice-as-activation-cost, pay-life-as-cost, etc. — those
    fall through as unpaid and the activation is skipped.
    """
    ast = perm.card.ast
    if ast is None or abil_idx >= len(ast.abilities):
        return
    abil = ast.abilities[abil_idx]
    if type(abil).__name__ != "Activated":
        return
    cost = getattr(abil, "cost", None)
    # Pay tap cost
    if cost is not None and getattr(cost, "tap", False):
        perm.tapped = True
    # Pay mana cost
    if cost is not None:
        mana_cost_obj = getattr(cost, "mana", None)
        if mana_cost_obj is not None and getattr(mana_cost_obj, "symbols", ()):
            paid = pay_mana_cost(game, seat, mana_cost_obj,
                                 spell_type="activated_ability",
                                 reason="activate",
                                 card_name=perm.card.name)
            if not paid:
                # Couldn't pay — untap + bail (refund tap cost).
                if cost is not None and getattr(cost, "tap", False):
                    perm.tapped = False
                game.ev("activation_failed", card=perm.card.name,
                        seat=seat.idx, reason="unpayable_mana")
                return
    game.ev("activate", card=perm.card.name, seat=seat.idx,
            ability_idx=abil_idx)
    # Push the effect onto the stack for resolution via the normal path.
    # For MVP we resolve inline — activated abilities that aren't mana
    # abilities DO use the stack (CR §602) but the current resolver
    # doesn't have an "activated ability" stack item type, so we apply
    # the effect directly.
    eff = getattr(abil, "effect", None)
    if eff is not None:
        # resolve_effect(game, source_seat: int, effect, depth, source_colors)
        try:
            resolve_effect(game, seat.idx, eff)
        except Exception as exc:
            # Log + skip — activation resolution failure shouldn't crash
            # the game. Parser-side gaps on the effect body surface here.
            game.ev("activation_resolve_error",
                    card=perm.card.name, seat=seat.idx,
                    error=type(exc).__name__, message=str(exc)[:120])


def _counterspell_reserve(seat: Seat) -> int:
    """If this seat is holding a counterspell, return the cost of the
    cheapest one so main_phase can leave that many lands untapped to hold
    priority on the opponent's next turn. Returns 0 if no counter in hand."""
    cheapest = None
    for c in seat.hand:
        if not _is_instant(c):
            continue
        if not _card_has_counterspell(c):
            continue
        if cheapest is None or c.cmc < cheapest:
            cheapest = c.cmc
    return cheapest or 0


def main_phase(game: Game, label: str) -> None:
    """CR §505. Main phase — precombat (label='main1') or postcombat
    (label='main2'). The step_kind is "" per §500.2 (main phases have
    no steps).

    All choices here delegate to ``seat.policy`` — the land to play,
    which affordable spell from hand to cast next, and whether to prefer
    a commander cast over a hand spell. The engine supplies the list of
    LEGAL options; the policy returns its pick.
    """
    phase_kind = "precombat_main" if label == "main1" else "postcombat_main"
    game.set_phase_step(phase_kind, "", legacy_phase=label)
    if game.ended: return
    s = game.seats[game.active]
    # Land drop — policy picks which land (if any) from hand this turn.
    if s.lands_played_this_turn < 1:
        lands_in_hand = [c for c in s.hand if "land" in c.type_line.lower()]
        if lands_in_hand and s.policy is not None:
            land_choice = s.policy.choose_land_to_play(game, s, lands_in_hand)
            if land_choice is not None and land_choice in s.hand:
                play_land(game, land_choice)
    # Fill mana pool — but if we're holding a counterspell, keep enough
    # lands untapped to fire it on the opponent's next turn (priority hold).
    reserve = _counterspell_reserve(s)
    fill_mana_pool(game, reserve=reserve)
    casts_this_phase = 0
    while casts_this_phase < 20:
        # §903.8 — if we can afford a commander from the command zone
        # and nothing comparable is in hand, cast the commander. The
        # commander-preference tiebreak stays in the engine for now
        # (it's a §903 correctness concern, not a strategic preference);
        # the policy still controls which HAND card wins if one is
        # competitive.
        cmd_card = _pick_castable_commander(game, s)
        # Build the engine's LEGAL-to-cast list; the policy filters
        # further by its own preferences (e.g. Poker-HOLD holds big
        # spells back).
        castable = [c for c in s.hand if can_cast(game, c)]
        chosen = None
        if s.policy is not None and castable:
            chosen = s.policy.choose_cast_from_hand(game, s, castable)
        if cmd_card is not None and (
                chosen is None or
                commander_cast_cost(s, cmd_card.name, cmd_card.cmc)
                >= chosen.cmc):
            cast_commander_from_command_zone(game, cmd_card)
            casts_this_phase += 1
            if game.ended:
                return
            continue
        if chosen is None:
            break
        # Policy may have returned a card that slipped past our filter
        # (e.g. a counterspell it wants to maindeck) — the engine
        # respects the policy's choice as long as it's legal.
        if chosen not in s.hand or not can_cast(game, chosen):
            break
        cast_spell(game, chosen)
        casts_this_phase += 1
        if game.ended:
            return
        # Refill mana pool in case we tapped mana creatures that just entered
        # (handles Dark Ritual style pool additions — AddMana already bumps pool)

    # Activation loop — surface legal activated abilities to the hat so
    # policies like OctoHat can actually fire them. Wired 2026-04-16 per
    # 7174n1c's finding that choose_activation was dead code.
    # Cap at 50 activations to prevent infinite loops (flicker/untap combos).
    activations_this_phase = 0
    while activations_this_phase < 50 and s.policy is not None:
        if game.ended:
            return
        activatable = _enumerate_legal_activations(game, s)
        if not activatable:
            break
        choice = s.policy.choose_activation(game, s, activatable)
        if choice is None:
            break
        try:
            perm, abil_idx = choice
        except (TypeError, ValueError):
            break
        if perm not in s.battlefield:
            break
        _resolve_activated_ability(game, s, perm, abil_idx)
        activations_this_phase += 1
        if game.ended:
            return
    # End of main — unspent generic mana empties in MVP.


def _pick_castable_commander(game: "Game",
                             seat: "Seat") -> Optional["CardEntry"]:
    """Return the cheapest commander in this seat's command zone the
    seat can afford to cast right now, or None. §903.8 tax applied."""
    if not game.commander_format:
        return None
    if not seat.command_zone:
        return None
    best = None
    best_cost = None
    for c in seat.command_zone:
        if c.name not in seat.commander_names:
            continue
        total = commander_cast_cost(seat, c.name, c.cmc)
        if _available_mana(seat) < total:
            continue
        if best is None or total < best_cost:
            best = c
            best_cost = total
    return best


# ============================================================================
# Combat phase
# ============================================================================


def kw(p: Permanent, name: str) -> bool:
    """Keyword check on a permanent (direct or granted)."""
    if has_keyword(p.card.ast, name):
        return True
    return name in p.granted


# ----------------------------------------------------------------------------
# Protection / indestructible / hexproof helpers
# ----------------------------------------------------------------------------

_COLOR_WORDS = {
    "white": "W", "blue": "U", "black": "B", "red": "R", "green": "G",
}


def protection_colors(p: Permanent) -> set[str]:
    """Return the set of color letters this creature has protection from.
    Parses the raw text of any Keyword(name='protection', raw='protection from X and from Y').
    Returns a set like {'R', 'B'}. Empty set if no protection.
    'protection from everything' → returns the sentinel {'*'} (we block all colored sources)."""
    out: set[str] = set()
    for ab in p.card.ast.abilities:
        if isinstance(ab, Keyword) and ab.name == "protection":
            raw = (ab.raw or "").lower()
            if "from everything" in raw:
                out.add("*")
                continue
            for word, letter in _COLOR_WORDS.items():
                if word in raw:
                    out.add(letter)
    return out


def source_colors(source: Permanent) -> set[str]:
    """Colors of a permanent's card — used to check protection / damage sources."""
    return set(source.card.colors or ())


def blocked_by_protection(attacker: Permanent, blocker: Permanent) -> bool:
    """True iff the attacker has protection from a color/type of the blocker.
    MVP: only color-based protection is checked."""
    prot = protection_colors(attacker)
    if not prot:
        return False
    if "*" in prot:
        # Protection from everything — any creature can't block it.
        return True
    return bool(prot & source_colors(blocker))


def damage_shields_protection(defender: Permanent, source: Permanent) -> bool:
    """True iff protection on the defender blocks damage from this source."""
    prot = protection_colors(defender)
    if not prot:
        return False
    if "*" in prot:
        return True
    return bool(prot & source_colors(source))


def is_hexproof(p: Permanent) -> bool:
    return kw(p, "hexproof")


def is_indestructible(p: Permanent) -> bool:
    return kw(p, "indestructible")


def can_attack(p: Permanent) -> bool:
    if not p.is_creature:
        return False
    if p.tapped:
        return False
    if p.summoning_sick and not kw(p, "haste"):
        return False
    if kw(p, "defender"):
        return False
    if p.power <= 0:
        return False
    return True


def can_block(blocker: Permanent, attacker: Permanent) -> bool:
    if not blocker.is_creature:
        return False
    if blocker.tapped:
        return False
    if kw(attacker, "flying"):
        if not (kw(blocker, "flying") or kw(blocker, "reach")):
            return False
    # Protection: attacker has protection from a color/type of the blocker.
    # Rule 702.16b: attackers with protection can't be blocked by matching creatures.
    if blocked_by_protection(attacker, blocker):
        return False
    # Symmetric: if the blocker has protection from the attacker, the attacker
    # simply does nothing when it hits that blocker — but legality-wise 702.16
    # only forbids blocking in the attacker-protection direction. Leave blocker
    # protection to the damage step.
    return True


def _eff_toughness_remaining(p: Permanent) -> int:
    return p.toughness - p.damage_marked


def _lethal_amount(attacker: Permanent, blocker: Permanent) -> int:
    if kw(attacker, "deathtouch"):
        return 1
    return max(1, _eff_toughness_remaining(blocker))


def declare_attackers(game: Game) -> list[Permanent]:
    s = game.seats[game.active]
    # Start each combat phase by clearing the "declared this combat"
    # flag on every creature. This is critical for multi-combat-phase
    # support (Aggravated Assault) — a creature that attacked in combat
    # phase 1, got untapped between phases, and attacks again in combat
    # phase 2 should fire its attacks trigger twice.
    for p in s.battlefield:
        p.declared_attacker_this_combat = False
        # NOTE: we do NOT clear `p.attacking` here. CR §506.4 says an
        # "attacking creature" remains so until end_of_combat (or until
        # removed from combat). Tokens created "tapped and attacking"
        # (Hero of Bladehold) carry attacking=True into this function
        # and must be recognized as attacking creatures without firing
        # their own "attacks" triggers.
    # Policy hook: the engine enumerates legal attackers, the policy
    # picks which to declare. This is the "hat" 7174n1c referred to —
    # Greedy / Poker / LLM / Human all satisfy the same interface and
    # the engine never inspects which is attached.
    legal = [p for p in list(s.battlefield) if can_attack(p)]
    chosen_by_policy = s.policy.declare_attackers(game, s, legal) \
        if s.policy is not None else legal
    chosen_set = set(id(p) for p in chosen_by_policy)
    attackers: list[Permanent] = []
    for p in list(s.battlefield):
        if can_attack(p) and id(p) in chosen_set:
            attackers.append(p)
            p.declared_attacker_this_combat = True
            p.attacking = True
            if not kw(p, "vigilance"):
                p.tapped = True
    if attackers:
        names = ", ".join(a.card.name for a in attackers)
        game.emit(f"  attack with: {names}")
        game.ev("attackers", attackers=[a.card.name for a in attackers])

    # Include any creatures that entered the battlefield "attacking"
    # (via effects like Hero of Bladehold's tokens). They count as
    # attacking creatures per §506.3, but their own "attacks" triggers
    # DON'T fire because they weren't declared. See CR 506.3/508.2.
    for p in s.battlefield:
        if p.attacking and not p.declared_attacker_this_combat:
            attackers.append(p)
            game.ev("entered_attacking", card=p.card.name,
                    rule="506.3")

    # Fire "whenever ~ attacks" triggers (Rule 603.2). Resolved in APNAP order,
    # but MVP simplifies: fire each attacker's own trigger first, then global
    # "whenever a creature you control attacks" triggers from other permanents.
    # Only DECLARED attackers fire their own triggers (506.3 note).
    declared_only = [a for a in attackers
                     if a.declared_attacker_this_combat]
    _fire_attack_triggers(game, declared_only)
    return attackers


def _iter_attack_triggers(ast: CardAST) -> list:
    """Pull every Triggered ability whose trigger.event is 'attack'."""
    out = []
    for ab in ast.abilities:
        if isinstance(ab, Triggered) and ab.trigger.event == "attack":
            out.append(ab)
    return out


def _fire_attack_triggers(game: Game, attackers: list[Permanent]) -> None:
    """Attack triggers fire once per attack declaration."""
    if not attackers:
        return
    controller_perms = list(game.seats[game.active].battlefield)

    # 1) Each attacker's own "attacks" trigger.
    for atk in attackers:
        for ab in _iter_attack_triggers(atk.card.ast):
            game.emit(f"  trigger: {atk.card.name} attacks")
            game.ev("trigger_fires", source_card=atk.card.name,
                    source_seat=atk.controller, event="attack")
            _resolve_attack_trigger_effect(game, atk, ab)
            if game.ended:
                return

    # 2) Global "whenever a creature you control attacks" triggers from OTHER
    #    permanents the active player controls. Fire once per attacker.
    for perm in controller_perms:
        if perm in attackers:
            continue
        for ab in _iter_attack_triggers(perm.card.ast):
            raw = (ab.raw or "").lower()
            if ("a creature you control attacks" not in raw and
                    "another creature attacks" not in raw):
                continue
            for atk in attackers:
                game.ev("trigger_fires", source_card=perm.card.name,
                        source_seat=perm.controller, event="attack_ally",
                        trigger_by_card=atk.card.name)
                _resolve_attack_trigger_effect(game, perm, ab)
                if game.ended:
                    return


def _resolve_attack_trigger_effect(game: Game, source: Permanent, trig) -> None:
    """Resolve the effect of an attack trigger. If the effect is UnknownEffect,
    try to synthesize common patterns from the raw text."""
    eff = trig.effect
    if eff is None:
        return
    if getattr(eff, "kind", None) == "unknown":
        synth = _synthesize_combat_effect(getattr(eff, "raw_text", "") or "", source)
        if synth is not None:
            for sub in synth:
                _apply_synth_effect(game, source, sub)
                if game.ended:
                    return
            return
        game.unknown_nodes["attack_trigger_unknown"] += 1
        return
    # Structured effect — route through the normal resolver.
    resolve_effect(game, source.controller, eff,
                   source_colors_hint=source.card.colors)


# ----------------------------------------------------------------------------
# Synthesized payloads for UnknownEffect raw_text (combat/ETB triggers).
# ----------------------------------------------------------------------------

_SYNTH_RE_BUFF_GAIN = re.compile(
    r"creatures you control gain ([a-z ,]+?) and get \+(\d+|x)/\+(\d+|x)"
)
_SYNTH_RE_BUFF_GET = re.compile(
    r"creatures you control get \+(\d+|x)/\+(\d+|x)"
    r"(?: and gain ([a-z ,]+?))?(?:\s+until end of turn|\.|,|$)"
)
_SYNTH_RE_SELF_BUFF = re.compile(r"^it gets \+(\d+)/\+(\d+)")
_SYNTH_RE_PING_PLAYER = re.compile(
    r"(?:this creature|it) deals (\d+) damage to "
    r"(?:the player or planeswalker it's attacking|that player|any target|"
    r"target player|target opponent|each opponent|target player or planeswalker)"
)


def _parse_keywords_blob(blob: str) -> tuple[str, ...]:
    """Extract keyword names from 'first strike and trample' or 'flying, lifelink'."""
    if not blob:
        return ()
    b = blob.lower().replace(" and ", ",")
    parts = [x.strip().strip(".") for x in b.split(",") if x.strip()]
    canonical = set()
    for tok in parts:
        for known in ("flying", "first strike", "double strike", "trample",
                      "deathtouch", "lifelink", "vigilance", "haste", "reach",
                      "menace", "defender", "indestructible", "hexproof"):
            if tok == known or tok.startswith(known):
                canonical.add(known)
                break
    return tuple(sorted(canonical))


def _synthesize_combat_effect(raw: str, source: Permanent):
    """Pattern-match common raw_text payloads. Returns list of synth tuples or None."""
    if not raw:
        return None
    r = raw.lower()

    # Craterhoof/gainfirst: "creatures you control gain K and get +X/+X"
    m = _SYNTH_RE_BUFF_GAIN.search(r)
    if m:
        kw_blob, p_raw, t_raw = m.group(1), m.group(2), m.group(3)
        power = -1 if p_raw == "x" else int(p_raw)
        toughness = -1 if t_raw == "x" else int(t_raw)
        return [("buff_your_creatures", power, toughness,
                 _parse_keywords_blob(kw_blob))]

    # "creatures you control get +N/+N [and gain K]"
    m = _SYNTH_RE_BUFF_GET.search(r)
    if m:
        p_raw, t_raw = m.group(1), m.group(2)
        kw_blob = m.group(3) or ""
        power = -1 if p_raw == "x" else int(p_raw)
        toughness = -1 if t_raw == "x" else int(t_raw)
        return [("buff_your_creatures", power, toughness,
                 _parse_keywords_blob(kw_blob))]

    # Hellrider: "deals N damage to the player ... attacking"
    m = _SYNTH_RE_PING_PLAYER.search(r)
    if m:
        return [("damage_defending_player", int(m.group(1)))]

    # Goblin Rabblemaster: "it gets +N/+M until end of turn"
    m = _SYNTH_RE_SELF_BUFF.search(r)
    if m:
        return [("buff_self", int(m.group(1)), int(m.group(2)))]

    return None


def _apply_synth_effect(game: Game, source: Permanent, payload: tuple) -> None:
    """Apply one synthesized payload."""
    tag = payload[0]
    s = game.seats[source.controller]
    if tag == "buff_your_creatures":
        _, power, toughness, grants = payload
        # Sentinel "-1" = X = number of creatures you control
        if power == -1 or toughness == -1:
            n = sum(1 for p in s.battlefield if p.is_creature)
            if power == -1:
                power = n
            if toughness == -1:
                toughness = n
        bumped = 0
        for p in s.battlefield:
            if not p.is_creature:
                continue
            p.buffs_pt = (p.buffs_pt[0] + power, p.buffs_pt[1] + toughness)
            for g in grants:
                if g not in p.granted:
                    p.granted.append(g)
            bumped += 1
        if bumped:
            game.emit(f"  → +{power}/+{toughness} to {bumped} creatures"
                      + (f" + grant [{','.join(grants)}]" if grants else ""))
            game.ev("buff_team", seat=s.idx, count=bumped,
                    power=power, toughness=toughness, grants=list(grants))
        return
    if tag == "damage_defending_player":
        _, amount = payload
        defender = game.opp(source.controller)
        before = defender.life
        defender.life -= amount
        game.emit(f"  → {source.card.name} pings seat {defender.idx} for {amount}")
        # Emit damage + life_change WITHOUT a `reason` so the auditor can pair
        # them (it skips life_change events that carry a reason).
        game.ev("damage", amount=amount, target_kind="player",
                target_seat=defender.idx,
                source_card=source.card.name, source_seat=source.controller)
        game.ev("life_change", seat=defender.idx,
                **{"from": before, "to": defender.life})
        if defender.life <= 0:
            defender.lost = True
            defender.loss_reason = "attack trigger damage"
            game.check_end()
        return
    if tag == "buff_self":
        _, power, toughness = payload
        source.buffs_pt = (source.buffs_pt[0] + power,
                           source.buffs_pt[1] + toughness)
        game.emit(f"  → {source.card.name} gets +{power}/+{toughness}")
        return


def _apply_self_etb_synthesized(game: Game, perm: Permanent) -> None:
    """For Craterhoof-style ETBs whose effect is UnknownEffect, synthesize
    from raw_text. Called from cast_spell after the normal ETB triggers."""
    for ab in perm.card.ast.abilities:
        if not isinstance(ab, Triggered):
            continue
        if ab.trigger.event != "etb":
            continue
        eff = ab.effect
        if eff is None or getattr(eff, "kind", None) != "unknown":
            continue
        synth = _synthesize_combat_effect(getattr(eff, "raw_text", "") or "", perm)
        if not synth:
            continue
        game.ev("trigger_fires", source_card=perm.card.name,
                source_seat=perm.controller, event="etb_synth")
        for sub in synth:
            _apply_synth_effect(game, perm, sub)
            if game.ended:
                return


def declare_blockers(game: Game, attackers: list[Permanent]) -> dict:
    """Assign blockers. Returns {id(attacker): [blocker, ...]}.

    Policy-driven: the engine identifies the defending seat (next
    opponent in APNAP via ``game.opp``) and calls that seat's policy
    ``declare_blockers`` method. The default :class:`GreedyHat`
    implements the original deadliest-first / chump-if-lethal heuristic;
    alternative hats may block differently (e.g. :class:`PokerHat`
    in RAISE mode doesn't block at all unless lethal is on the table).

    The engine guarantees the returned dict's keys are ``id(attacker)``
    for each attacker — policies that omit keys get them defaulted to an
    empty list here.
    """
    defender_seat = game.opp(game.active)
    assignment: dict
    if defender_seat.policy is not None:
        assignment = defender_seat.policy.declare_blockers(
            game, defender_seat, attackers,
        )
        # Normalize: make sure every attacker has an entry.
        for atk in attackers:
            assignment.setdefault(id(atk), [])
    else:
        assignment = {id(a): [] for a in attackers}

    for atk in attackers:
        bs = assignment[id(atk)]
        if bs:
            game.emit(f"  block {atk.card.name} with "
                      + ", ".join(b.card.name for b in bs))
        else:
            game.emit(f"  {atk.card.name} unblocked")
    game.ev("blockers", pairs=[
        {"attacker": a.card.name,
         "blockers": [b.card.name for b in assignment[id(a)]]}
        for a in attackers
    ])
    return assignment


def _apply_damage_to_player(game: Game, source: Permanent, amount: int,
                             target_seat: Seat) -> None:
    before = target_seat.life
    target_seat.life -= amount
    game.emit(f"    {source.card.name} deals {amount} to seat {target_seat.idx} "
              f"(life → {target_seat.life})")
    game.ev("damage", amount=amount, target_kind="player",
            target_seat=target_seat.idx, source_card=source.card.name,
            source_seat=source.controller)
    game.ev("life_change", seat=target_seat.idx,
            **{"from": before, "to": target_seat.life})
    if kw(source, "lifelink"):
        controller = game.seats[source.controller]
        ll_before = controller.life
        controller.life += amount
        game.emit(f"    lifelink → seat {controller.idx} gains {amount} "
                  f"(life → {controller.life})")
        game.ev("life_change", seat=controller.idx,
                **{"from": ll_before, "to": controller.life},
                reason="lifelink")
    # CR §510.2: "whenever ~ deals combat damage to a player" triggers
    # fire PER DAMAGE INSTANCE. Because _deal_combat_damage_step iterates
    # per attacker / per blocker / per first-strike-vs-normal step, and
    # because double-strikers execute both first-strike and normal damage
    # steps, the same creature's trigger fires ONCE per damage event —
    # so double-strike = 2 triggers per swing. Perfect per-instance.
    _fire_combat_damage_triggers(game, source, amount,
                                 target_kind="player",
                                 target_seat=target_seat.idx,
                                 target_perm=None)
    if target_seat.life <= 0:
        target_seat.lost = True
        target_seat.loss_reason = "combat damage"
        game.check_end()


def _apply_damage_to_creature(game: Game, source: Permanent, amount: int,
                               target: Permanent) -> None:
    # Rule 702.16e: a permanent with protection can't be dealt damage by a
    # source of the stated color. Damage is prevented (0 is dealt).
    if damage_shields_protection(target, source):
        game.emit(f"    {source.card.name} → {target.card.name} "
                  f"(protection prevents {amount} damage)")
        game.ev("damage_prevented", source_card=source.card.name,
                source_seat=source.controller,
                target_card=target.card.name, target_seat=target.controller,
                amount=amount, reason="protection")
        return
    target.damage_marked += amount
    # Deathtouch (Rule 702.2b): any nonzero damage is lethal. But indestructible
    # creatures can't be destroyed by lethal damage — still mark damage, SBAs
    # will decline to destroy.
    if kw(source, "deathtouch") and amount > 0:
        target.damage_marked = max(target.damage_marked, target.toughness)
    game.emit(f"    {source.card.name} deals {amount} to {target.card.name} "
              f"({target.damage_marked}/{target.toughness})")
    game.ev("damage", amount=amount, target_kind="permanent",
            target_card=target.card.name, target_seat=target.controller,
            source_card=source.card.name, source_seat=source.controller)
    if kw(source, "lifelink"):
        controller = game.seats[source.controller]
        ll_before = controller.life
        controller.life += amount
        game.emit(f"    lifelink → seat {controller.idx} gains {amount} "
                  f"(life → {controller.life})")
        game.ev("life_change", seat=controller.idx,
                **{"from": ll_before, "to": controller.life},
                reason="lifelink")
    # CR §510.2 / §603.3: "whenever ~ deals combat damage to a creature"
    # triggers fire per damage instance.
    _fire_combat_damage_triggers(game, source, amount,
                                 target_kind="creature",
                                 target_seat=target.controller,
                                 target_perm=target)


def _iter_combat_damage_triggers(ast: "CardAST") -> list:
    """Yield triggered abilities whose trigger event is
    "deal_combat_damage" (CR §603.3a + §510.2 — "whenever ~ deals
    combat damage" templating)."""
    out = []
    for ab in ast.abilities:
        if isinstance(ab, Triggered):
            ev = ab.trigger.event or ""
            if ev in ("deal_combat_damage", "deals_combat_damage",
                      "deal_damage", "deals_damage"):
                out.append(ab)
    return out


def _fire_combat_damage_triggers(game: Game, source: Permanent,
                                  amount: int, target_kind: str,
                                  target_seat: int,
                                  target_perm: Optional[Permanent]) -> None:
    """Fire "whenever ~ deals combat damage" triggers PER DAMAGE INSTANCE
    (CR §510.2). Called from _apply_damage_to_{player,creature} after the
    damage has been applied. Each invocation = one instance.

    Double-strikers naturally fire twice because _deal_combat_damage_step
    is called twice (first-strike step + normal step), so the same
    creature's trigger fires once per step. Lifelink/deathtouch per-
    instance is also correct because they run inline per damage event.
    """
    if amount <= 0:
        return
    for ab in _iter_combat_damage_triggers(source.card.ast):
        game.ev("trigger_fires",
                source_card=source.card.name,
                source_seat=source.controller,
                event="deals_combat_damage",
                amount=amount,
                target_kind=target_kind,
                target_seat=target_seat,
                target_card=getattr(getattr(target_perm, "card", None),
                                    "name", None),
                rule="510.2")
        # Try to resolve the trigger body. Effects like "create a
        # Treasure" / "draw a card" flow through resolve_effect.
        if ab.effect is None:
            continue
        try:
            if getattr(ab.effect, "kind", None) == "unknown":
                synth = _synthesize_combat_effect(
                    getattr(ab.effect, "raw_text", "") or "", source)
                if synth is not None:
                    for sub in synth:
                        _apply_synth_effect(game, source, sub)
                        if game.ended:
                            return
                    continue
            resolve_effect(game, source.controller, ab.effect,
                           source_colors_hint=source.card.colors)
        except Exception as exc:
            game.ev("combat_damage_trigger_crashed",
                    source_card=source.card.name,
                    exception=f"{type(exc).__name__}: {exc}")


def _deals_in_step(p: Permanent, first_strike_step: bool) -> bool:
    fs = kw(p, "first strike")
    ds = kw(p, "double strike")
    if first_strike_step:
        return fs or ds
    return (not fs) or ds


def _alive(game: Game, p: Permanent) -> bool:
    return p in game.seats[p.controller].battlefield


def _deal_combat_damage_step(game: Game, attackers: list[Permanent],
                              assignment: dict,
                              first_strike: bool) -> None:
    defender_seat = game.opp(game.active)

    # ATTACKER → blockers (or defender)
    for atk in attackers:
        if not _alive(game, atk):
            continue
        if not _deals_in_step(atk, first_strike):
            continue
        amt = atk.power
        if amt <= 0:
            continue
        declared_blockers = assignment.get(id(atk), [])
        live_blockers = [b for b in declared_blockers if _alive(game, b)]

        if not declared_blockers:
            _apply_damage_to_player(game, atk, amt, defender_seat)
            if game.ended:
                return
            continue
        if not live_blockers:
            if kw(atk, "trample"):
                _apply_damage_to_player(game, atk, amt, defender_seat)
                if game.ended:
                    return
            continue

        remaining = amt
        for b in live_blockers:
            if remaining <= 0:
                break
            need = _lethal_amount(atk, b)
            give = min(remaining, need)
            _apply_damage_to_creature(game, atk, give, b)
            remaining -= give
        if remaining > 0 and kw(atk, "trample"):
            _apply_damage_to_player(game, atk, remaining, defender_seat)
            if game.ended:
                return

    # BLOCKERS → attackers
    for atk in attackers:
        blockers = assignment.get(id(atk), [])
        for b in blockers:
            if not _alive(game, b) or not _alive(game, atk):
                continue
            if not _deals_in_step(b, first_strike):
                continue
            amt = b.power
            if amt <= 0:
                continue
            _apply_damage_to_creature(game, b, amt, atk)


def combat_phase(game: Game) -> None:
    """CR §506. Combat phase — 5 steps: begin_of_combat, declare_attackers,
    declare_blockers, combat_damage (possibly twice for first-strike),
    end_of_combat.

    This function handles ONE combat phase. Multiple combat phases in a
    single turn (Aggravated Assault, Seize the Day) are handled by
    take_turn()'s loop via `game.pending_extra_combats`.
    """
    game.combat_phase_number += 1
    # Beginning of combat step (§507).
    game.set_phase_step("combat", "begin_of_combat")

    # Declare attackers step (§508). "Whenever ~ attacks" triggers fire
    # inside declare_attackers()/_fire_attack_triggers().
    game.set_phase_step("combat", "declare_attackers")
    attackers = declare_attackers(game)
    if not attackers:
        # Skip directly to end_of_combat per §506.1.
        game.set_phase_step("combat", "end_of_combat")
        return

    # Declare blockers step (§509).
    game.set_phase_step("combat", "declare_blockers")
    assignment = declare_blockers(game, attackers)

    has_fs = any(
        kw(a, "first strike") or kw(a, "double strike") for a in attackers
    ) or any(
        kw(b, "first strike") or kw(b, "double strike")
        for bs in assignment.values() for b in bs
    )

    # Combat damage steps (§510). If any creature has first or double
    # strike there are two damage steps.
    if has_fs:
        game.set_phase_step("combat", "first_strike_damage")
        _deal_combat_damage_step(game, attackers, assignment,
                                  first_strike=True)
        state_based_actions(game)
        if game.ended:
            return

    game.set_phase_step("combat", "combat_damage")
    _deal_combat_damage_step(game, attackers, assignment,
                              first_strike=False)
    state_based_actions(game)
    if game.ended:
        return

    # End of combat step (§511). "At end of combat" delayed triggers fire
    # here via set_phase_step's _fire_delayed_triggers().
    game.set_phase_step("combat", "end_of_combat")
    # CR §506.4: attacking/blocking status ends when creatures are
    # "removed from combat" at end_of_combat. Clear per-combat flags
    # so the next combat phase (or next turn) starts clean.
    for seat in game.seats:
        for p in seat.battlefield:
            p.attacking = False
            p.declared_attacker_this_combat = False


def end_step(game: Game) -> None:
    """CR §513. End step. Delayed 'at the beginning of the next end step'
    triggers fire via set_phase_step's _fire_delayed_triggers()."""
    game.set_phase_step("ending", "end")
    # Clear mana pools (§500.4)
    for seat in game.seats:
        if seat.mana_pool:
            game.ev("pool_drain", seat=seat.idx, amount=seat.mana_pool,
                    reason="end_step")
        seat.mana_pool = 0


def cleanup_step(game: Game) -> None:
    """CR §514. Cleanup step. Discard to hand size, wear off 'until end
    of turn' effects, remove all damage. scan_expired_durations() handles
    the EOT-effect and damage-wear-off via set_phase_step()."""
    game.set_phase_step("ending", "cleanup")
    # CR §514.1: discard down to maximum hand size (normally 7). Policy
    # decides WHICH cards to pitch. The engine enforces the count and
    # the §514.1 timing; picking the cards is a decision site.
    active = game.seats[game.active]
    if len(active.hand) > 7:
        overflow = len(active.hand) - 7
        if active.policy is not None:
            to_discard = active.policy.choose_discard(
                game, active, list(active.hand), overflow,
            )
        else:
            # Fallback (no policy): highest-CMC first.
            to_discard = sorted(
                active.hand, key=lambda c: -c.cmc,
            )[:overflow]
        for c in to_discard:
            if c in active.hand:
                active.hand.remove(c)
                active.graveyard.append(c)
                game.ev("discard", seat=active.idx, card=c.name,
                        reason="cleanup_hand_size", rule="514.1")
    # §514.2 (damage wear-off and EOT expirations) already handled by
    # scan_expired_durations() via set_phase_step("ending", "cleanup").
    # §514.3 priority — no triggers in our current engine require the
    # active player to receive priority during cleanup, so we're done.


def end_the_turn(game: Game, source_card_name: str = "Sundial") -> None:
    """CR §723.1. "End the turn" — the Sundial of the Infinite / Time
    Stop path. Skip straight to the cleanup step, suppressing any
    remaining end-step triggers from the current turn. "Until end of
    turn" effects still wear off (§514.2) — that's the canonical use.

    Per 7174n1c: damage wears off at ANY turn-end path, and EOT effects
    also expire here exactly like normal cleanup. End-of-turn triggers
    that were ALREADY on the stack are removed (§723.1b). We don't
    currently stack end-step triggers (end_step fires them inline), so
    we simply skip directly to cleanup.
    """
    game.ev("end_the_turn", source=source_card_name, rule="723.1")
    # Skip any remaining phases/steps — jump straight to cleanup.
    game.set_phase_step("ending", "cleanup")
    # Don't run end_step() — the whole point of "end the turn" is to
    # suppress end-step triggers (§723.1b removes triggered abilities
    # that would have fired later this turn).


def take_turn(game: Game) -> None:
    """CR §500.1. Turn structure: beginning (untap, upkeep, draw) →
    precombat main → combat → postcombat main → ending (end, cleanup).

    Multiple combat phases (Aggravated Assault / Seize the Day) are
    implemented via game.pending_extra_combats: resolving those effects
    bumps the counter; this function loops combat+main2 until the
    counter is exhausted. CR §500.5 allows any number of additional
    phases or steps added by effects.
    """
    if game.ended:
        return
    game.ev("turn_start")
    # §726.3a — day/night transition at turn start, BEFORE untap.
    # game.spells_cast_by_active_last_turn was snapshotted by the
    # previous cleanup_step before control rotated. This evaluates
    # whether the state flips day↔night.
    evaluate_day_night_at_turn_start(game)
    untap_step(game)
    upkeep_step(game)
    if game.ended: return
    draw_step(game)
    if game.ended: return
    main_phase(game, "main1")
    if game.ended: return
    combat_phase(game)
    if game.ended: return
    # Additional combat phases (Aggravated Assault etc.). Each adds a
    # full combat_phase without an intermediate main phase (§500.5 —
    # an effect that adds combat phases inserts them after the current
    # main phase is over).
    while game.pending_extra_combats > 0:
        game.pending_extra_combats -= 1
        combat_phase(game)
        if game.ended:
            return
    main_phase(game, "main2")
    if game.ended: return
    end_step(game)
    if game.ended: return
    cleanup_step(game)
    game.snapshot()


def swap_active(game: Game) -> None:
    """Advance to the next seat's turn (CR §500.7 — turn rotation).

    N-seat aware. Cycles through seats in order, skipping eliminated
    ones (they no longer take turns per §800.4). Bumps ``game.turn``
    when the active pointer wraps back to the lowest-index LIVING
    seat (approximating "round number" rather than strict seat-0 wraps
    — meaningful once seat 0 has been eliminated).

    Back-compat for 2-player: if no seats are eliminated, ``game.turn``
    still bumps when active returns to 0, matching the legacy behavior.
    """
    n = len(game.seats)
    if n <= 0:
        return
    start_idx = game.active
    # Identify the lowest-index living seat; turn counter bumps when
    # control returns to it.
    living = [s for s in game.seats if not s.lost]
    if not living:
        # Degenerate: all seats dead — leave game state alone; the
        # caller's loop guards on game.ended.
        return
    round_anchor = living[0].idx
    # Cycle forward, skipping dead seats. Worst case N steps (matters
    # if every other seat was just eliminated simultaneously).
    for step in range(1, n + 1):
        nxt = (start_idx + step) % n
        if not game.seats[nxt].lost:
            game.active = nxt
            break
    else:
        # No living seat found other than current (also degenerate).
        return
    if game.active == round_anchor:
        game.turn += 1


# ============================================================================
# Full game loop
# ============================================================================

def play_game(*args,
              rng: random.Random = None,
              verbose: bool = False,
              commander_format: bool = False,
              decks: list = None,
              n_seats: Optional[int] = None) -> Game:
    """Play an N-seat game (2, 3, 4, or more).

    Two calling conventions are supported:

      Legacy 2-player:
          play_game(deck_a, deck_b, rng, verbose=False,
                    commander_format=False)
          deck_a / deck_b are list[CardEntry] (non-Commander) or Deck
          (Commander). The rng is positional arg #3.

      N-seat:
          play_game(decks=[d0, d1, d2, d3], rng=rng,
                    commander_format=True)
          `decks` is a list of decks (list[CardEntry] for non-Commander
          or Deck for Commander). ``n_seats`` defaults to ``len(decks)``.

    Non-Commander mode:
      decks are list[CardEntry]. 20 life, 7-card opening.
    Commander mode (commander_format=True):
      decks are Deck(cards=[...], commander_name="..." [, commander_
      names=[...]]). §903.4 40 life, §903.6 commanders placed into
      command zone, §903.7 7-card opening, §903.8/.9/.10 active.

    The N-seat path rotates turns via ``swap_active`` and the multi-
    player turn cap ``MAX_TURNS_MULTIPLAYER``. Last seat standing wins
    (§104.2a). Seat elimination triggers §800.4a cleanup via
    ``check_end`` → ``_handle_seat_elimination``.
    """
    # --- Normalize the calling convention ------------------------------
    # Legacy positional: (deck_a, deck_b, rng, ...)
    if decks is None:
        if len(args) >= 2:
            # 2-player legacy path — first two positionals are decks.
            decks = [args[0], args[1]]
            if len(args) >= 3 and rng is None:
                rng = args[2]
        else:
            raise ValueError(
                "play_game requires either legacy (deck_a, deck_b, rng) "
                "positionals or keyword-arg `decks=[...]`.")
    elif args:
        # Mixed — rng may have been positional.
        if len(args) >= 1 and rng is None:
            rng = args[0]
    if rng is None:
        rng = random.Random()
    if n_seats is None:
        n_seats = len(decks)
    if n_seats != len(decks):
        raise ValueError(
            f"play_game: n_seats={n_seats} but got {len(decks)} decks")
    if n_seats < 2:
        raise ValueError(f"play_game: need >=2 seats, got {n_seats}")

    # --- Build seats ---------------------------------------------------
    if commander_format:
        # Deck objects carry the 99-card remainder and commander name.
        seat_libs = [list(d.cards) for d in decks]
        seats = [Seat(idx=i, library=seat_libs[i])
                 for i in range(n_seats)]
        game = Game(seats=seats, verbose=verbose, rng=rng)
        # Shuffle libraries BEFORE commander setup so any commander
        # still lurking in the library can be pulled deterministically.
        for s in seats:
            rng.shuffle(s.library)
        setup_commander_game(game, *decks)
        for s in seats:
            draw_cards_no_lose(s, STARTING_HAND)
        game.active = rng.randint(0, n_seats - 1)
        game.emit(
            f"commander game start — {n_seats} seats, "
            f"seat {game.active} on the play (CR 903.4+903.7)")
        game.ev("game_start", on_the_play=game.active,
                n_seats=n_seats, commander_format=True)
        game.snapshot()
    else:
        seats = [Seat(idx=i, library=list(decks[i]))
                 for i in range(n_seats)]
        for s in seats:
            rng.shuffle(s.library)
            draw_cards_no_lose(s, STARTING_HAND)
        game = Game(seats=seats, verbose=verbose, rng=rng)
        game.active = rng.randint(0, n_seats - 1)
        game.emit(
            f"game start — {n_seats} seats, "
            f"seat {game.active} on the play")
        game.ev("game_start", on_the_play=game.active, n_seats=n_seats)
        game.snapshot()

    # Choose the appropriate turn cap. 2-player preserves legacy
    # MAX_TURNS; 3+ seats use the longer multiplayer cap.
    turn_cap = MAX_TURNS if n_seats == 2 else MAX_TURNS_MULTIPLAYER

    while not game.ended and game.turn <= turn_cap:
        take_turn(game)
        if game.ended:
            break
        # CR §726.3a — snapshot the ending seat's per-turn spell cast
        # count BEFORE rotating the active pointer. The next turn's
        # evaluate_day_night_at_turn_start() reads this to decide
        # day↔night flips.
        ending_seat = game.seats[game.active]
        game.spells_cast_by_active_last_turn = ending_seat.spells_cast_this_turn
        swap_active(game)

    if not game.ended:
        # Turn cap → ranking tiebreaker. Living seats ranked by life;
        # highest life wins. Ties = no winner (draw).
        living = [s for s in game.seats if not s.lost]
        if not living:
            game.winner = None
            game.end_reason = f"turn cap ({turn_cap}); no living seats"
        else:
            living.sort(key=lambda s: s.life, reverse=True)
            top_life = living[0].life
            leaders = [s for s in living if s.life == top_life]
            if len(leaders) == 1:
                game.winner = leaders[0].idx
                others = [str(s.life) for s in living
                          if s.idx != leaders[0].idx]
                game.end_reason = (
                    f"turn cap ({turn_cap}); seat {game.winner} ahead "
                    f"on life {top_life} vs [{', '.join(others)}]")
            else:
                # Tie among leaders — no clean winner. Keep legacy
                # 2-player wording when applicable.
                game.winner = None
                tied = ",".join(str(s.idx) for s in leaders)
                game.end_reason = (
                    f"turn cap ({turn_cap}); tied on life at "
                    f"{top_life} (seats {tied})")
        game.ended = True

    for s in game.seats:
        if s.mana_pool > 0:
            game.ev("pool_drain", seat=s.idx, amount=s.mana_pool,
                    reason="game_over")
            s.mana_pool = 0
    game.emit(
        f"game over: winner={game.winner}, "
        f"reason={game.end_reason}, turn={game.turn}")
    game.snapshot()
    game.ev("game_over", winner=game.winner,
            reason=game.end_reason, turn=game.turn,
            n_seats=n_seats)
    return game


def draw_cards_no_lose(seat: Seat, n: int) -> None:
    """Opening-hand draw: don't trigger deck-out flag."""
    for _ in range(n):
        if seat.library:
            seat.hand.append(seat.library.pop(0))


# ============================================================================
# Main
# ============================================================================

@dataclass
class MatchupResult:
    name: str
    deck_a_name: str
    deck_b_name: str
    deck_a: list
    deck_b: list
    wins: Counter
    turns: list
    end_reasons: Counter
    unknown_nodes: Counter
    games: int


def _run_matchup(name: str, deck_a_name: str, deck_a: list,
                 deck_b_name: str, deck_b: list,
                 games: int, seed: int, verbose: bool,
                 sample_log_lines: list, first_game_log: bool,
                 capture_events: list | None = None,
                 audit_sink=None) -> MatchupResult:
    wins: Counter = Counter()
    turns: list[int] = []
    end_reasons: Counter = Counter()
    unknown_nodes: Counter = Counter()

    print(f"\n--- Matchup: {name} ({deck_a_name} vs {deck_b_name}) ---")
    for g in range(games):
        game_rng = random.Random(seed + g * 1000 + 1)
        game = play_game(deck_a, deck_b, game_rng, verbose=False)
        if first_game_log and g == 0:
            sample_log_lines.append(f"\n===== {name} — GAME 0 =====\n")
            sample_log_lines.extend(game.log)
        if capture_events is not None and g == 0:
            capture_events.extend(game.events)
        if audit_sink is not None:
            # Stream every event of every game to the audit writer, tagged.
            audit_sink(name, g, game.events)
        wins[game.winner] += 1
        turns.append(game.turn)
        end_reasons[game.end_reason[:60]] += 1
        for k, v in game.unknown_nodes.items():
            unknown_nodes[k] += v

    avg = sum(turns) / len(turns) if turns else 0
    med = sorted(turns)[len(turns) // 2] if turns else 0
    print(f"  {deck_a_name:>12} wins: {wins[0]:>3} ({100*wins[0]/games:.0f}%)  |  "
          f"{deck_b_name:>12} wins: {wins[1]:>3} ({100*wins[1]/games:.0f}%)  |  "
          f"draws: {wins[None]:>3}")
    print(f"  avg {avg:.2f} / med {med} / min {min(turns)} / max {max(turns)} turns")

    return MatchupResult(
        name=name, deck_a_name=deck_a_name, deck_b_name=deck_b_name,
        deck_a=deck_a, deck_b=deck_b,
        wins=wins, turns=turns, end_reasons=end_reasons,
        unknown_nodes=unknown_nodes, games=games,
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--games", type=int, default=100)
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--verbose", action="store_true",
                    help="print every event for each game")
    ap.add_argument("--print-log", type=int, default=1,
                    help="print full turn log for first N games per matchup")
    ap.add_argument("--json-log", type=str, default=None,
                    help="write JSONL event log of first matchup's first game to PATH (for replay viewer)")
    ap.add_argument("--audit", action="store_true",
                    help="write every game's event log to data/rules/audit_events.jsonl and run rule auditor")
    args = ap.parse_args()

    mtg_parser.load_extensions()
    cards = json.loads(ORACLE_DUMP.read_text())
    cards_by_name = {c["name"].lower(): c for c in cards if mtg_parser.is_real_card(c)}

    # Build all 4 decks
    deck_rng = random.Random(args.seed)
    burn = build_deck(BURN_DECK_LIST, cards_by_name, deck_rng)
    control = build_deck(CONTROL_DECK_LIST, cards_by_name, deck_rng)
    creatures = build_deck(CREATURES_DECK_LIST, cards_by_name, deck_rng)
    ramp = build_deck(RAMP_DECK_LIST, cards_by_name, deck_rng)

    if not burn or not control or not creatures or not ramp:
        print("error: could not build decks (missing cards in oracle dump)",
              file=sys.stderr)
        sys.exit(1)

    print(f"deck Burn:      {len(burn)} cards — unique: {len({c.name for c in burn})}")
    print(f"deck Control:   {len(control)} cards — unique: {len({c.name for c in control})}")
    print(f"deck Creatures: {len(creatures)} cards — unique: {len({c.name for c in creatures})}")
    print(f"deck Ramp:      {len(ramp)} cards — unique: {len({c.name for c in ramp})}")

    sample_log_path = REPORT.parent / "playloop_sample_log.txt"
    sample_log_lines: list[str] = []

    # Audit event sink: optionally stream every event of every game to disk.
    audit_events_path = REPORT.parent / "audit_events.jsonl"
    audit_fh = None
    audit_total_events = 0
    audit_sink = None
    if args.audit:
        audit_events_path.parent.mkdir(parents=True, exist_ok=True)
        audit_fh = audit_events_path.open("w")

        def _sink(matchup: str, game_idx: int, events: list) -> None:
            nonlocal audit_total_events
            for evt in events:
                rec = dict(evt)
                rec["_matchup"] = matchup
                rec["_game"] = game_idx
                audit_fh.write(json.dumps(rec) + "\n")
                audit_total_events += 1
        audit_sink = _sink

    # 4-deck round-robin → 6 matchups.
    # Decks & their display keys. Order fixed for matrix layout.
    DECKS = [
        ("Burn", burn),
        ("Control", control),
        ("Creatures", creatures),
        ("Ramp", ramp),
    ]

    results: list[MatchupResult] = []
    captured_events: list | None = [] if args.json_log else None
    # The first matchup still gets its first game captured for --json-log parity.
    first_matchup_flag = True
    mu_idx = 0
    for i in range(len(DECKS)):
        for j in range(i + 1, len(DECKS)):
            name_a, deck_a = DECKS[i]
            name_b, deck_b = DECKS[j]
            label = f"{name_a} vs {name_b}"
            seed_offset = args.seed + (mu_idx * 100000)
            results.append(_run_matchup(
                label, name_a, deck_a, name_b, deck_b,
                args.games, seed_offset, args.verbose,
                sample_log_lines, first_game_log=(args.print_log > 0),
                capture_events=(captured_events if first_matchup_flag else None),
                audit_sink=audit_sink,
            ))
            first_matchup_flag = False
            mu_idx += 1

    sample_log_path.write_text("\n".join(sample_log_lines))

    if args.json_log and captured_events is not None:
        jp = Path(args.json_log)
        jp.parent.mkdir(parents=True, exist_ok=True)
        with jp.open("w") as f:
            for evt in captured_events:
                f.write(json.dumps(evt) + "\n")
        print(f"  → wrote {len(captured_events)} events to {jp}")

    if audit_fh is not None:
        audit_fh.close()
        print(f"  → audit stream: {audit_total_events} events → {audit_events_path}")

    # Aggregate text summary
    print("\n" + "═" * 70)
    print(f"  Round-robin tournament — {args.games} games × {len(results)} matchups")
    print("═" * 70)
    for r in results:
        g = r.games
        avg = sum(r.turns) / len(r.turns) if r.turns else 0
        med = sorted(r.turns)[len(r.turns) // 2] if r.turns else 0
        print(f"\n  {r.name}")
        print(f"    {r.deck_a_name} wins:   {r.wins[0]:>3}  ({100*r.wins[0]/g:.1f}%)")
        print(f"    {r.deck_b_name} wins:   {r.wins[1]:>3}  ({100*r.wins[1]/g:.1f}%)")
        print(f"    Draws:                  {r.wins[None]:>3}  ({100*r.wins[None]/g:.1f}%)")
        print(f"    Avg turns: {avg:.2f}  Median: {med}  Min/Max: {min(r.turns)}/{max(r.turns)}")

    # Tournament matrix & report.
    matrix = _build_tournament_matrix(DECKS, results)
    _print_tournament_matrix(matrix, DECKS)
    write_tournament_report(args, results, DECKS, matrix)
    # Keep legacy report too (harmless).
    write_report(args, results, sample_log_path)

    # Run auditor if requested.
    if args.audit:
        try:
            import rule_auditor
            viol_path = REPORT.parent / "audit_violations.jsonl"
            summary_path = REPORT.parent / "audit_summary.md"
            counts = rule_auditor.audit_jsonl(
                audit_events_path, viol_path, summary_path)
            total = sum(counts.values())
            print("\n" + "═" * 70)
            print(f"  Rule auditor — {total} violations across {audit_total_events} events")
            print("═" * 70)
            for rule, n in counts.most_common():
                print(f"    {rule:>28}: {n}")
            print(f"  → {viol_path}")
            print(f"  → {summary_path}")
        except Exception as e:
            print(f"  ! auditor failed: {e!r}", file=sys.stderr)


def _build_tournament_matrix(decks, results) -> dict:
    """Return a dict keyed by (row_deck, col_deck) → row win-rate vs col."""
    names = [n for n, _ in decks]
    mat: dict = {(a, b): None for a in names for b in names}
    for r in results:
        g = r.games
        # r.deck_a_name vs r.deck_b_name. Row=a perspective, Col=a perspective.
        if g > 0:
            a_rate = r.wins[0] / g
            b_rate = r.wins[1] / g
        else:
            a_rate = b_rate = 0.0
        mat[(r.deck_a_name, r.deck_b_name)] = a_rate
        mat[(r.deck_b_name, r.deck_a_name)] = b_rate
    return mat


def _print_tournament_matrix(mat: dict, decks) -> None:
    names = [n for n, _ in decks]
    print("\n" + "═" * 70)
    print("  Tournament win-rate matrix (row-deck wins vs column-deck)")
    print("═" * 70)
    header = " " * 14 + "".join(f"{n[:8]:>8}" for n in names)
    print(header)
    for row in names:
        line = f"{row[:12]:>12}  "
        for col in names:
            if row == col:
                line += f"{'  -':>8}"
            else:
                v = mat.get((row, col))
                line += f"{(v*100):>7.0f}%" if v is not None else f"{'n/a':>8}"
        print(line)


def write_tournament_report(args, results, decks, matrix) -> None:
    out = REPORT.parent / "tournament_report.md"
    md = []
    md.append("# 4-Deck Round-Robin Tournament Report\n")
    md.append(
        f"_{args.games} games per matchup × {len(results)} matchups "
        f"= {args.games * len(results)} total games. "
        f"seed={args.seed}, max-turns={MAX_TURNS}._\n"
    )

    # Matrix
    md.append("## Win-rate matrix\n")
    md.append("Row-deck win-rate vs column-deck.\n")
    names = [n for n, _ in decks]
    header = "| | " + " | ".join(names) + " |"
    sep = "|---" * (len(names) + 1) + "|"
    md.append(header)
    md.append(sep)
    for row in names:
        cells = [f"**{row}**"]
        for col in names:
            if row == col:
                cells.append("—")
            else:
                v = matrix.get((row, col))
                cells.append(f"{v*100:.0f}%" if v is not None else "n/a")
        md.append("| " + " | ".join(cells) + " |")
    md.append("")

    # Per-matchup stats
    md.append("## Matchup details\n")
    md.append("| Matchup | Deck A wins | Deck B wins | Draws | Avg turns | Median |")
    md.append("|---|---:|---:|---:|---:|---:|")
    for r in results:
        g = r.games
        avg = sum(r.turns) / len(r.turns) if r.turns else 0
        med = sorted(r.turns)[len(r.turns) // 2] if r.turns else 0
        md.append(
            f"| **{r.name}** | "
            f"{r.wins[0]} ({100*r.wins[0]/g:.0f}%) | "
            f"{r.wins[1]} ({100*r.wins[1]/g:.0f}%) | "
            f"{r.wins[None]} ({100*r.wins[None]/g:.0f}%) | "
            f"{avg:.2f} | {med} |"
        )
    md.append("")

    # End reasons
    md.append("## End-reason breakdown\n")
    for r in results:
        md.append(f"### {r.name}\n")
        md.append("| Count | Reason |")
        md.append("|---:|---|")
        for reason, count in r.end_reasons.most_common():
            md.append(f"| {count} | {reason} |")
        md.append("")

    # Deck compositions
    md.append("## Deck compositions\n")
    seen = set()
    for name, deck in decks:
        if name in seen:
            continue
        seen.add(name)
        unique = sorted({c.name for c in deck})
        md.append(f"**{name}** — {len(deck)} cards, {len(unique)} unique:\n")
        md.append("\n".join(f"- {n}" for n in unique))
        md.append("")

    out.write_text("\n".join(md))
    print(f"  → {out}")


def _turn_histogram(turns: list[int]) -> str:
    """Return a small ascii histogram of turn counts."""
    if not turns:
        return "_(no games)_"
    buckets: dict[int, int] = {}
    for t in turns:
        # group into buckets of size 2 starting at 2
        lo = ((t - 1) // 2) * 2 + 1   # 1-2 → 1, 3-4 → 3, etc.
        buckets[lo] = buckets.get(lo, 0) + 1
    max_count = max(buckets.values())
    lines = ["| Turns | Games | |", "|---:|---:|:---|"]
    for lo in sorted(buckets):
        n = buckets[lo]
        bar = "█" * max(1, int(20 * n / max_count))
        lines.append(f"| {lo}-{lo+1} | {n} | {bar} |")
    return "\n".join(lines)


def write_report(args, results, sample_log_path):
    md = []
    md.append("# Playloop Report — 3 Matchups with Combat\n")
    md.append(
        f"_Self-play simulation, **{args.games} games per matchup**, "
        f"seed={args.seed}, max-turns={MAX_TURNS}._\n"
    )
    md.append("## What this is\n")
    md.append(
        "A Python-only, structural-AST self-play loop. Decks are drafted from the "
        "oracle dump, the parser emits typed AST, and the playloop dispatches on "
        "Effect node kinds. The turn structure is: untap → upkeep (triggers) → "
        "draw → main1 → **combat** → main2 → end.\n\n"
        "**Combat** (new): declare-attackers → declare-blockers → first-strike "
        "damage → regular damage, with state-based actions between damage steps. "
        "Heuristic policies: attack-with-everything, block smallest-survivor / "
        "chump-only-if-lethal. Menace requires 2 blockers; trample spillovers "
        "to player; lifelink heals controller; deathtouch marks lethal.\n"
    )

    md.append("## Matchup Results\n")
    md.append("| Matchup | Deck A | Wins A | Deck B | Wins B | Draws | Avg turns |")
    md.append("|---|---|---:|---|---:|---:|---:|")
    for r in results:
        g = r.games
        avg = sum(r.turns) / len(r.turns) if r.turns else 0
        md.append(
            f"| **{r.name}** | {r.deck_a_name} | "
            f"{r.wins[0]} ({100*r.wins[0]/g:.0f}%) | "
            f"{r.deck_b_name} | "
            f"{r.wins[1]} ({100*r.wins[1]/g:.0f}%) | "
            f"{r.wins[None]} ({100*r.wins[None]/g:.0f}%) | "
            f"{avg:.2f} |"
        )
    md.append("")

    # Per-matchup detail
    for r in results:
        md.append(f"### {r.name}\n")
        avg = sum(r.turns) / len(r.turns) if r.turns else 0
        med = sorted(r.turns)[len(r.turns) // 2] if r.turns else 0
        md.append(f"- **{r.deck_a_name}** wins: **{r.wins[0]}** "
                  f"({100*r.wins[0]/r.games:.1f}%)\n"
                  f"- **{r.deck_b_name}** wins: **{r.wins[1]}** "
                  f"({100*r.wins[1]/r.games:.1f}%)\n"
                  f"- Draws: {r.wins[None]} "
                  f"({100*r.wins[None]/r.games:.1f}%)\n"
                  f"- Turn stats — avg **{avg:.2f}**, median {med}, "
                  f"min/max {min(r.turns)}/{max(r.turns)}\n")
        md.append("\n**Turn distribution:**\n")
        md.append(_turn_histogram(r.turns))
        md.append("")
        md.append("**End reasons:**\n")
        md.append("| Count | Reason |")
        md.append("|---:|---|")
        for reason, count in r.end_reasons.most_common():
            md.append(f"| {count} | {reason} |")
        md.append("")

    # Deck compositions
    md.append("## Deck compositions\n")
    seen = set()
    for r in results:
        for name, deck in [(r.deck_a_name, r.deck_a), (r.deck_b_name, r.deck_b)]:
            if name in seen:
                continue
            seen.add(name)
            unique = sorted({c.name for c in deck})
            md.append(f"**{name}** — {len(deck)} cards, "
                      f"{len(unique)} unique:\n")
            md.append("\n".join(f"- {n}" for n in unique))
            md.append("")

    # Combine unknowns across matchups
    combined_unknowns: Counter = Counter()
    for r in results:
        combined_unknowns.update(r.unknown_nodes)
    md.append("## Unknown / unhandled AST nodes (combined)\n")
    if combined_unknowns:
        md.append("| Count | Node kind |")
        md.append("|---:|---|")
        for node, count in combined_unknowns.most_common():
            md.append(f"| {count} | `{node}` |")
    else:
        md.append("_None._")
    md.append("")

    md.append("## Combat keywords supported\n")
    md.append(
        "- **flying** — only blockable by flying/reach\n"
        "- **reach** — can block flyers\n"
        "- **first strike** — deals damage in the first-strike step\n"
        "- **double strike** — deals damage in BOTH steps\n"
        "- **trample** — excess damage after lethal-assigned goes to player\n"
        "- **deathtouch** — any damage marks lethal on the blocker\n"
        "- **lifelink** — damage dealt heals controller\n"
        "- **vigilance** — attacker does not tap\n"
        "- **menace** — must be blocked by 2+ creatures\n"
        "- **defender** — can block but never attacks\n"
        "- **haste** — ignores summoning sickness (pre-existing)\n"
    )

    md.append("## What works (beyond combat)\n")
    md.append(
        "- Full turn structure: untap → upkeep → draw → main1 → combat → main2 → end.\n"
        "- State-based actions between damage steps (first strike removes dead "
        "blockers before regular damage).\n"
        "- Effects executing: Damage, Draw, Discard, Destroy, Buff, CreateToken, "
        "AddMana, GainLife, LoseLife, CounterMod, Tap/Untap, Sequence/Choice/"
        "Optional/Conditional control flow, ETB triggers, upkeep triggers, "
        "Tutor (basic-land), Reanimate.\n"
    )

    md.append("## What's stubbed\n")
    md.append(
        "- **Stack**: counterspells resolve as no-ops; spells resolve at cast time.\n"
        "- **Colored mana**: single generic pool; CMC-only cost check.\n"
        "- **Combat tricks**: no instants during combat (no stack in MVP).\n"
        "- **Planeswalkers**: no attacking planeswalkers; attacks always target player.\n"
        "- **Protection / indestructible / shroud / hexproof**: not implemented.\n"
        "- **Damage-prevention / replacement effects**: not implemented.\n"
        "- **Scry / Surveil / LookAt / Reveal**: no-ops (library not reordered).\n"
        "- **Choice**: always picks option 1.\n"
        "- **Conditional**: always executes body.\n"
        "- **ExtraTurn**: ignored.\n"
        "- **GainControl**: no-op.\n"
    )

    md.append("## Files\n")
    md.append(f"- Simulator: `scripts/playloop.py`\n"
              f"- Sample game logs (first game of each matchup): "
              f"`{sample_log_path.relative_to(ROOT)}`\n"
              f"- This report: `{REPORT.relative_to(ROOT)}`\n")

    md.append("## Next steps\n")
    md.append(
        "1. Port combat resolver to Go so `internal/game/` can share it.\n"
        "2. Colored-mana constraints (ManaSymbol.color on pool entries).\n"
        "3. Instants during opponent's turn (requires a stack).\n"
        "4. Planeswalker attackers / redirected damage.\n"
        "5. MCTS / heuristic policy for Choice + Conditional branches.\n"
    )

    REPORT.write_text("\n".join(md))
    print(f"\n  → {REPORT}")
    print(f"  → {sample_log_path}")


# ============================================================================
# Test-harness helpers (additive — used by interaction_harness_*.py)
# ============================================================================

def setup_board_state(
    *,
    seat_0_bf=None,
    seat_0_hand=None,
    seat_0_lib=None,
    seat_0_mana=0,
    seat_0_life=STARTING_LIFE,
    seat_1_bf=None,
    seat_1_hand=None,
    seat_1_lib=None,
    seat_1_mana=0,
    seat_1_life=STARTING_LIFE,
    active=0,
    verbose=False,
) -> Game:
    """Construct a Game with explicit zone contents.

    Each `*_bf` / `*_hand` / `*_lib` argument is a list of CardEntry. Seats
    start at STARTING_LIFE unless overridden. Mana pools are preloaded
    (pretend-tap shortcut). Does NOT run turn structure — the caller drives
    the game via run_scripted_sequence or by invoking take_turn() manually.
    """
    def _seat(idx, bf, hand, lib, mana, life):
        s = Seat(
            idx=idx,
            life=life,
            library=list(lib or []),
            hand=list(hand or []),
            battlefield=[],
            graveyard=[],
            exile=[],
            mana_pool=mana,
            lands_played_this_turn=0,
        )
        for c in (bf or []):
            is_land = "land" in c.type_line.lower()
            s.battlefield.append(Permanent(
                card=c, controller=idx,
                tapped=False,
                summoning_sick=(not is_land),
            ))
        return s

    seats = [
        _seat(0, seat_0_bf, seat_0_hand, seat_0_lib, seat_0_mana, seat_0_life),
        _seat(1, seat_1_bf, seat_1_hand, seat_1_lib, seat_1_mana, seat_1_life),
    ]
    game = Game(seats=seats, verbose=verbose)
    # Now that the Game exists, stamp each pre-placed permanent with a
    # timestamp and initialize planeswalker loyalty / battle defense
    # counters (CR 306.5b, 310.3). See _etb_initialize() for details.
    for s in game.seats:
        for perm in s.battlefield:
            _etb_initialize(game, perm)
    game.active = active
    game.turn = 1
    # Test-harness default: start in precombat main so the caller can
    # immediately start casting. Use set_phase_step so phase_kind/
    # step_kind stay consistent with `phase`.
    game.set_phase_step("precombat_main", "", legacy_phase="main1")
    game.ev("game_start", on_the_play=active)
    # Fire per-card ETB handlers for permanents placed in the initial
    # board state. This mirrors cast_spell → _resolve_stack_top's
    # invocation of apply_per_card_etb so that test harnesses that seed
    # cards like Painter's Servant directly onto the battlefield still
    # get the ETB-time slug dispatch (chosen-color selection, etc.).
    for s in game.seats:
        for perm in list(s.battlefield):
            apply_per_card_etb(game, perm)
    game.snapshot()
    return game


def _find_in_hand(seat: Seat, name: str) -> Optional[CardEntry]:
    for c in seat.hand:
        if c.name == name:
            return c
    # case-insensitive fallback
    low = name.lower()
    for c in seat.hand:
        if c.name.lower() == low:
            return c
    return None


def run_scripted_sequence(game: Game, steps: list, *,
                          max_steps: int = 200) -> Game:
    """Force a seat to play specific cards in order, bypassing the greedy AI.

    Each step is a tuple:
        ("cast", "<Card Name>")                    -> cast by name from the
                                                      active player's hand
        ("cast", "<Card Name>", {"name": "..."})   -> cast with chosen name
                                                      param (Demonic Consult)
        ("activate", "<Card Name>", "<ability>")   -> activate an ability on a
                                                      permanent (planeswalker,
                                                      tap-mill, etc.)
        ("pass",)                                   -> pass priority (advance
                                                      to next action)
        ("end_turn",)                               -> end current turn, swap
                                                      active, enter new turn's
                                                      main1 (runs untap/upkeep/
                                                      draw)
        ("upkeep",)                                 -> explicit upkeep step
        ("draw",  N)                                -> force the active player
                                                      to draw N cards
        ("check_end",)                              -> run state-based actions
                                                      and check win/loss

    Unknown effects that have no engine handler still run (as no-ops in the
    resolver), so this function does NOT assert correctness — the caller
    inspects game.winner / game.ended / game.log afterward.
    """
    if max_steps <= 0:
        max_steps = 200
    steps_run = 0
    for step in steps:
        steps_run += 1
        if steps_run > max_steps:
            game.emit(f"[harness] max_steps ({max_steps}) exceeded")
            break
        if game.ended:
            break
        op = step[0]
        if op == "cast":
            name = step[1]
            params = step[2] if len(step) > 2 else {}
            seat = game.seats[game.active]
            card = _find_in_hand(seat, name)
            if card is None:
                game.emit(f"[harness] cast {name!r} — not in hand")
                continue
            # Ensure enough mana is available; preload pool if short
            if seat.mana_pool < card.cmc:
                seat.mana_pool = max(seat.mana_pool, card.cmc)
                game.ev("harness_mana_preload", seat=seat.idx,
                        amount=card.cmc, card=card.name)
            if params:
                game.ev("harness_cast_param", card=card.name, params=params)
            cast_spell(game, card)
        elif op == "activate":
            card_name = step[1]
            ability_label = step[2] if len(step) > 2 else ""
            seat = game.seats[game.active]
            perm = next((p for p in seat.battlefield
                         if p.card.name == card_name), None)
            if perm is None:
                game.emit(f"[harness] activate {card_name!r} — not on battlefield")
                continue
            # Find a matching activated ability
            target_ab = None
            for ab in perm.card.ast.abilities:
                if isinstance(ab, Activated):
                    # Match by loyalty-sign ('+1', '−8') or by raw substring
                    extras = ab.cost.extra or ()
                    if ability_label and ability_label in extras:
                        target_ab = ab
                        break
                    if ability_label and ability_label in (ab.raw or ""):
                        target_ab = ab
                        break
                    if not ability_label:
                        target_ab = ab
                        break
            if target_ab is None:
                game.emit(f"[harness] activate {card_name!r} "
                          f"{ability_label!r} — no matching ability")
                game.ev("harness_activate_missing", card=card_name,
                        label=ability_label)
                continue
            # Pay mana cost if any, tap if tap-cost present
            cost = target_ab.cost
            cmc = cost.mana.cmc if cost.mana is not None else 0
            if cost.tap and perm.tapped:
                game.emit(f"[harness] activate {card_name!r} — already tapped")
                continue
            if cmc > 0:
                if not _pay_generic_cost(game, seat, cmc,
                                         reason="activate",
                                         card_name=card_name):
                    game.emit(f"[harness] activate {card_name!r} — "
                              f"not enough mana ({seat.mana_pool}/{cmc})")
                    continue
            if cost.tap:
                perm.tapped = True
            game.ev("harness_activate", card=card_name, label=ability_label)
            game.emit(f"[harness] activate {card_name} "
                      f"({ability_label or 'only ability'})")
            # Per-card name-based activation dispatch. A handful of
            # snowflake activated abilities (Grindstone's mill loop,
            # Mindslaver's turn control) have a custom runtime keyed by
            # card name rather than by AST slug — the parser can't
            # emit a clean Static(custom) for an Activated body. We
            # dispatch first, and fall through to resolve_effect only
            # if no custom handler fired.
            custom_activation_slug = _NAME_TO_ACTIVATED_SLUG.get(card_name)
            custom_handled = False
            if custom_activation_slug and _dispatch_per_card is not None:
                custom_handled = _dispatch_per_card(
                    game, seat.idx, perm.card, custom_activation_slug,
                    {"permanent": perm, "activated": True})
            if not custom_handled:
                # Resolve the effect directly (simple no-stack activation).
                resolve_effect(game, seat.idx, target_ab.effect,
                               source_colors_hint=perm.card.colors)
            state_based_actions(game)
        elif op == "pass":
            game.ev("harness_pass")
        elif op == "end_turn":
            # Full turn-end: end step + cleanup step (§513 + §514).
            # cleanup_step fires scan_expired_durations → clears EOT
            # effects and damage. Then swap and start the next turn.
            end_step(game)
            if not game.ended:
                cleanup_step(game)
            swap_active(game)
            if game.ended:
                break
            untap_step(game)
            upkeep_step(game)
            if game.ended:
                break
            draw_step(game)
            state_based_actions(game)
        elif op == "end_the_turn":
            # Sundial of the Infinite / Time Stop path — §723.1.
            # Skip straight to cleanup, expire EOT effects, suppress
            # end-step triggers. Handy for testing the §723 semantics.
            end_the_turn(game, source_card_name=step[1] if len(step) > 1
                         else "test_harness_sundial")
        elif op == "upkeep":
            upkeep_step(game)
            state_based_actions(game)
        elif op == "draw":
            count = step[1] if len(step) > 1 else 1
            draw_cards(game, game.seats[game.active], count)
            state_based_actions(game)
        elif op == "check_end":
            state_based_actions(game)
            game.check_end()
        else:
            game.emit(f"[harness] unknown step op: {op!r}")
    # Final SBA sweep
    state_based_actions(game)
    game.check_end()
    return game


if __name__ == "__main__":
    main()
