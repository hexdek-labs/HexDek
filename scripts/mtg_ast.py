#!/usr/bin/env python3
"""Typed AST for MTG card oracle text.

Per comp rules §113, every ability is one of:
- Static (always-on rules text, e.g. "Flying", "Creatures you control have +1/+1")
- Activated (cost: effect, e.g. "{T}: Add {C}", "{2}{B}, Sacrifice X: Draw a card")
- Triggered (When/Whenever/At trigger, effect)
- Keyword (named shorthand, e.g. Cycling, Flashback — expand to one of the above)

The parser's job is to consume a card's oracle text and emit a list of these
typed abilities. The cluster signature of a card becomes the structural
fingerprint of its AST — two cards cluster iff their ASTs are equivalent up
to renaming of unconstrained parameters.

Coverage metric: a card is GREEN iff every word of its oracle text is consumed
by the parser (no UNPARSED tail). Goal: 100%, where every miss is a specific
unhandled grammar production we can implement.

This module is *just the schema*. The parser lives in `scripts/parser.py`.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional, Union


# ============================================================================
# Mana costs
# ============================================================================

@dataclass(frozen=True)
class ManaSymbol:
    """A single mana symbol from a cost. {2}, {U}, {U/B}, {2/W}, {X}, {S} (snow)."""
    raw: str  # canonical "{U}" form
    generic: int = 0           # {2}
    color: tuple[str, ...] = ()  # ("W",) or ("U","B") for hybrid; empty for generic/X/snow
    is_x: bool = False
    is_phyrexian: bool = False
    is_snow: bool = False


@dataclass(frozen=True)
class ManaCost:
    """Sequence of mana symbols."""
    symbols: tuple[ManaSymbol, ...]

    @property
    def cmc(self) -> int:
        return sum(s.generic + (1 if s.color else 0) for s in self.symbols)


# ============================================================================
# Targets / filters
# ============================================================================

@dataclass(frozen=True)
class Filter:
    """A filter expression like 'target creature you control', 'each opponent',
    'a basic Plains card', 'two target nonland permanents'.

    Stored structurally so two filters with the same shape compare equal.
    """
    base: str                        # "creature" / "land" / "permanent" / "spell" / "player" / "opponent" / etc.
    quantifier: str = "one"          # "one" / "n" / "all" / "each" / "any" / "up_to_n"
    count: Union[int, str, None] = None  # 1, 2, "x", None
    targeted: bool = True            # "target X" vs "a/an X" (untargeted)
    you_control: bool = False
    opponent_controls: bool = False
    nontoken: bool = False
    creature_types: tuple[str, ...] = ()  # ("Goblin", "Human")
    color_filter: tuple[str, ...] = ()    # ("W", "U")
    color_exclude: tuple[str, ...] = ()   # for "nonblack creature"
    mana_value_op: Optional[str] = None   # "<=", ">=", "==", None
    mana_value: Optional[int] = None
    extra: tuple[str, ...] = ()           # other adjectives we don't normalize: ("attacking", "tapped", ...)


# A few built-in shorthands for common filters
TARGET_ANY = Filter(base="any_target")
TARGET_CREATURE = Filter(base="creature")
TARGET_PLAYER = Filter(base="player")
TARGET_OPPONENT = Filter(base="opponent")
EACH_OPPONENT = Filter(base="opponent", quantifier="each", targeted=False)
EACH_PLAYER = Filter(base="player", quantifier="each", targeted=False)
SELF = Filter(base="self", targeted=False)


# ============================================================================
# Triggers
# ============================================================================

@dataclass(frozen=True)
class Trigger:
    """The event side of a triggered ability."""
    event: str  # "etb" / "die" / "ltb" / "attack" / "block" / "deal_combat_damage" /
                # "cast" / "phase" / "untap" / "transform" / "discover" / etc.
    actor: Optional[Filter] = None    # what triggers it (e.g. "a creature you control")
    target_filter: Optional[Filter] = None  # for damage/cast triggers, what was targeted/cast
    phase: Optional[str] = None       # for phase triggers: "upkeep" / "end_step" / "combat_start"
    controller: Optional[str] = None  # "you" / "each" / "active_player"
    condition: Optional["Condition"] = None  # intervening "if" clause


# ============================================================================
# Costs (for activated abilities and additional costs)
# ============================================================================

@dataclass(frozen=True)
class Cost:
    """A composite cost. Each field is None or set."""
    mana: Optional[ManaCost] = None
    tap: bool = False
    untap: bool = False                   # {Q}
    sacrifice: Optional[Filter] = None    # "Sacrifice a creature"
    discard: Optional[int] = None         # "Discard N cards"
    pay_life: Optional[int] = None
    exile_self: bool = False
    return_self_to_hand: bool = False
    remove_counters: Optional[tuple[int, str]] = None  # (count, kind)
    extra: tuple[str, ...] = ()           # uncategorized cost text


# ============================================================================
# Conditions (intervening / static)
# ============================================================================

@dataclass(frozen=True)
class Condition:
    """A boolean condition like 'if you control a Plains', 'as long as you have
    20 or more life', 'if it had no -1/-1 counters on it'."""
    kind: str  # "you_control" / "life_threshold" / "card_count_zone" / "tribal" / etc.
    args: tuple = ()


# ============================================================================
# Effects (recursive AST)
# ============================================================================

# Effect is the recursive part. We use a tagged-union pattern: each Effect is
# a subclass of EffectNode with `kind` discriminating the node type.

@dataclass(frozen=True)
class EffectNode:
    """Base class. Subclasses set `kind` and add typed fields."""
    kind: str = field(init=False, default="")


@dataclass(frozen=True)
class Sequence(EffectNode):
    """Comma- or period-joined effects executed in order."""
    items: tuple["Effect", ...] = ()
    kind: str = field(init=False, default="sequence")


@dataclass(frozen=True)
class Choice(EffectNode):
    """Player chooses one (or N) of these. Models 'choose one — A; B; C'."""
    options: tuple["Effect", ...] = ()
    pick: Union[int, str] = 1
    or_more: bool = False
    kind: str = field(init=False, default="choice")


@dataclass(frozen=True)
class Optional_(EffectNode):
    """'You may [effect].' — optional effect with no opportunity cost."""
    body: "Effect" = None
    kind: str = field(init=False, default="optional")


@dataclass(frozen=True)
class Conditional(EffectNode):
    """'If [condition], [body]; otherwise [else_body].'"""
    condition: Condition = None
    body: "Effect" = None
    else_body: Optional["Effect"] = None
    kind: str = field(init=False, default="conditional")


# Concrete leaf effects — one per comp-rules-defined effect type.
# Each leaf carries the structural parameters that distinguish it from
# functionally different cards.

@dataclass(frozen=True)
class Damage(EffectNode):
    amount: Union[int, str]              # 3, "x", "var"
    target: Filter = TARGET_ANY
    divided: bool = False
    kind: str = field(init=False, default="damage")


@dataclass(frozen=True)
class Draw(EffectNode):
    count: Union[int, str] = 1
    target: Filter = SELF                 # SELF or a Filter for "target player draws"
    kind: str = field(init=False, default="draw")


@dataclass(frozen=True)
class Discard(EffectNode):
    count: Union[int, str] = 1
    target: Filter = SELF                 # SELF or "target player discards"
    chosen_by: str = "controller"         # "controller" / "discarder" / "random"
    kind: str = field(init=False, default="discard")


@dataclass(frozen=True)
class Mill(EffectNode):
    count: Union[int, str] = 1
    target: Filter = SELF
    kind: str = field(init=False, default="mill")


@dataclass(frozen=True)
class Scry(EffectNode):
    count: Union[int, str] = 1
    kind: str = field(init=False, default="scry")


@dataclass(frozen=True)
class Surveil(EffectNode):
    count: Union[int, str] = 1
    kind: str = field(init=False, default="surveil")


@dataclass(frozen=True)
class CounterSpell(EffectNode):
    target: Filter = Filter(base="spell")
    unless: Optional[Cost] = None         # "unless its controller pays X"
    kind: str = field(init=False, default="counter_spell")


@dataclass(frozen=True)
class Destroy(EffectNode):
    target: Filter = TARGET_CREATURE
    kind: str = field(init=False, default="destroy")


@dataclass(frozen=True)
class Exile(EffectNode):
    target: Filter = TARGET_CREATURE
    until: Optional[str] = None            # "next_end_step" / "leaves_battlefield" / None
    face_down: bool = False
    kind: str = field(init=False, default="exile")


@dataclass(frozen=True)
class Bounce(EffectNode):
    target: Filter = TARGET_CREATURE
    to: str = "owners_hand"                # "owners_hand" / "top_of_library" / "bottom_of_library"
    kind: str = field(init=False, default="bounce")


@dataclass(frozen=True)
class Tutor(EffectNode):
    query: Filter                          # what to search for
    destination: str = "hand"              # "hand" / "battlefield" / "battlefield_tapped" / "graveyard" / "top_of_library"
    count: Union[int, str] = 1
    optional: bool = False                 # "you may search" vs unconditional
    shuffle_after: bool = True
    reveal: bool = False
    rest: Optional[str] = None             # what happens to non-chosen cards: "bottom" / "graveyard" / "exile"
    kind: str = field(init=False, default="tutor")


@dataclass(frozen=True)
class Reanimate(EffectNode):
    """Move a card from a graveyard to the battlefield."""
    query: Filter = Filter(base="creature_card")
    from_zone: str = "any_graveyard"       # "your_graveyard" / "any_graveyard"
    destination: str = "battlefield"       # "battlefield" / "battlefield_tapped"
    controller: str = "you"                # "you" / "owner"
    with_modifications: tuple[str, ...] = ()  # "haste" / "p1p1_counter" / "tapped"
    kind: str = field(init=False, default="reanimate")


@dataclass(frozen=True)
class Recurse(EffectNode):
    """Move a card from a graveyard to a hand."""
    query: Filter = Filter(base="card")
    from_zone: str = "your_graveyard"
    destination: str = "hand"
    kind: str = field(init=False, default="recurse")


@dataclass(frozen=True)
class GainLife(EffectNode):
    amount: Union[int, str] = 1
    target: Filter = SELF                  # SELF or "target player gains"
    kind: str = field(init=False, default="gain_life")


@dataclass(frozen=True)
class LoseLife(EffectNode):
    amount: Union[int, str] = 1
    target: Filter = SELF
    kind: str = field(init=False, default="lose_life")


@dataclass(frozen=True)
class SetLife(EffectNode):
    amount: Union[int, str]
    target: Filter = SELF
    kind: str = field(init=False, default="set_life")


@dataclass(frozen=True)
class Sacrifice(EffectNode):
    """Effect-driven sacrifice (not a cost-side sacrifice)."""
    query: Filter
    actor: str = "controller"              # "controller" / "target_player" / "each_opponent"
    kind: str = field(init=False, default="sacrifice")


@dataclass(frozen=True)
class CreateToken(EffectNode):
    count: Union[int, str] = 1
    pt: Optional[tuple[int, int]] = None   # (2, 2) for 2/2
    types: tuple[str, ...] = ()            # ("Soldier", "Creature")
    color: tuple[str, ...] = ()
    keywords: tuple[str, ...] = ()         # ("flying",)
    is_copy_of: Optional[Filter] = None    # "create a token that's a copy of"
    legendary: bool = False
    tapped: bool = False
    kind: str = field(init=False, default="create_token")


@dataclass(frozen=True)
class CounterMod(EffectNode):
    """Put or remove counters."""
    op: str = "put"                        # "put" / "remove" / "double" / "move"
    count: Union[int, str] = 1
    counter_kind: str = "+1/+1"
    target: Filter = TARGET_CREATURE
    kind: str = field(init=False, default="counter_mod")


@dataclass(frozen=True)
class Buff(EffectNode):
    """Temporary or permanent stat modification (+N/+N until EOT, anthems)."""
    power: int = 0
    toughness: int = 0
    target: Filter = TARGET_CREATURE
    duration: str = "until_end_of_turn"    # "until_end_of_turn" / "permanent" / "until_your_next_turn"
    kind: str = field(init=False, default="buff")


@dataclass(frozen=True)
class GrantAbility(EffectNode):
    """Grant a static or activated ability to a target until duration."""
    ability_name: str                      # "flying" / "haste" / "indestructible" / etc.
    target: Filter = TARGET_CREATURE
    duration: str = "until_end_of_turn"
    kind: str = field(init=False, default="grant_ability")


@dataclass(frozen=True)
class TapEffect(EffectNode):
    target: Filter = TARGET_CREATURE
    kind: str = field(init=False, default="tap")


@dataclass(frozen=True)
class UntapEffect(EffectNode):
    target: Filter = TARGET_CREATURE
    kind: str = field(init=False, default="untap")


@dataclass(frozen=True)
class AddMana(EffectNode):
    """Mana-ability output: 'Add {U}', 'Add one mana of any color', 'Add {C}{C}'."""
    pool: tuple[ManaSymbol, ...] = ()
    any_color_count: int = 0               # for "add N mana of any color"
    kind: str = field(init=False, default="add_mana")


@dataclass(frozen=True)
class GainControl(EffectNode):
    target: Filter = TARGET_CREATURE
    duration: str = "permanent"
    kind: str = field(init=False, default="gain_control")


@dataclass(frozen=True)
class CopySpell(EffectNode):
    target: Filter = Filter(base="instant_or_sorcery_spell")
    may_choose_new_targets: bool = True
    kind: str = field(init=False, default="copy_spell")


@dataclass(frozen=True)
class CopyPermanent(EffectNode):
    target: Filter = Filter(base="creature")
    as_token: bool = True
    kind: str = field(init=False, default="copy_permanent")


@dataclass(frozen=True)
class Fight(EffectNode):
    a: Filter = TARGET_CREATURE
    b: Filter = TARGET_CREATURE
    kind: str = field(init=False, default="fight")


@dataclass(frozen=True)
class Reveal(EffectNode):
    """Reveal cards from a zone."""
    source: str = "your_hand"              # "your_hand" / "top_of_library" / "graveyard" / "exile"
    count: Union[int, str] = 1
    actor: str = "controller"              # who reveals
    kind: str = field(init=False, default="reveal")


@dataclass(frozen=True)
class LookAt(EffectNode):
    """Look at a hidden zone."""
    target: Filter = TARGET_PLAYER
    zone: str = "hand"                     # "hand" / "library_top_n"
    count: Union[int, str] = 1
    kind: str = field(init=False, default="look_at")


@dataclass(frozen=True)
class Shuffle(EffectNode):
    target: Filter = SELF                  # whose library
    kind: str = field(init=False, default="shuffle")


@dataclass(frozen=True)
class ExtraTurn(EffectNode):
    after_this: bool = True
    target: Filter = SELF
    kind: str = field(init=False, default="extra_turn")


@dataclass(frozen=True)
class ExtraCombat(EffectNode):
    after_this: bool = True
    kind: str = field(init=False, default="extra_combat")


@dataclass(frozen=True)
class WinGame(EffectNode):
    target: Filter = SELF
    kind: str = field(init=False, default="win_game")


@dataclass(frozen=True)
class LoseGame(EffectNode):
    target: Filter = SELF
    kind: str = field(init=False, default="lose_game")


@dataclass(frozen=True)
class Replacement(EffectNode):
    """Replacement effect: 'If [event] would happen, [replacement] instead.'"""
    trigger_event: str
    replacement: "Effect"
    kind: str = field(init=False, default="replacement")


@dataclass(frozen=True)
class Prevent(EffectNode):
    amount: Union[int, str] = "all"
    damage_filter: Optional[Filter] = None
    duration: str = "until_end_of_turn"
    kind: str = field(init=False, default="prevent")


# Catch-all for "we know it's an effect but we don't have a structured node yet"
@dataclass(frozen=True)
class UnknownEffect(EffectNode):
    raw_text: str
    kind: str = field(init=False, default="unknown")


# ============================================================================
# Face-down family (CR §708, §702.37 Morph, §701.40 Manifest, §701.58 Cloak,
# §702.168 Disguise, §701.62 Manifest Dread)
# ============================================================================
#
# Every member of the face-down family shares the same "2/2 colorless nameless
# no-text" characteristic override (CR §708.2a) implemented as a §613.2b
# layer-1b copy effect. What differs is ONLY:
#   - casting cost / special action to get onto the battlefield face-down
#   - optional ward {N} on the face-down permanent (Disguise, Cloak, Cybermen)
#   - optional +1/+1 counter on turn-face-up (Megamorph)
#   - look-and-mill-other procedure (Manifest Dread)
#
# Mechanic taxonomy (names match Keyword.name for Morph/Megamorph/Disguise,
# and appear as CreateToken.subtype marker for Manifest/Manifest-Dread/Cloak
# since those are keyword ACTIONS not keyword abilities).

# Canonical 2/2 colorless nameless characteristics per CR §708.2a. Referenced
# by the §613 layer 1b face_down_copy_effect resolver to override ALL other
# characteristics (except counter-driven P/T modifications — see §613.4,
# layer 7d sublayer runs AFTER layer 1, so a face-down 2/2 + a +1/+1 counter
# is a 3/3).
#
# Frozen + fields-only so the dataclass is hashable and can be embedded in
# Modification.args tuples without breaking signature() equality checks.
@dataclass(frozen=True)
class FaceDownCharacteristics:
    """The canonical face-down copy-effect override per CR §708.2a.

    The face-down characteristic-defining effect replaces a permanent's
    copiable values at layer 1b (CR §613.2b) with this fixed set, unless
    the enabling ability/rule explicitly overrides one of the fields
    (Disguise, Cloak, and Cybermen override ``ward`` with ward {2};
    no current-printed ability changes the 2/2 or colorless aspects).

    Layer 7d counter-based P/T adjustments still apply after the layer-1b
    override — this is why Megamorph's +1/+1 counter bumps a face-down
    2/2 to 3/3 when turned face-up via the megamorph cost (CR §702.37b).
    """
    power: int = 2
    toughness: int = 2
    # CR §708.2a: "no text, no name, no subtypes, and no mana cost". Type
    # line is bare "Creature" — no supertype, no subtype. Colorless per
    # §708.2a (no colors because no mana cost to derive them from).
    name: str = ""
    type_line: str = "Creature"
    types: tuple[str, ...] = ("creature",)
    supertypes: tuple[str, ...] = ()
    subtypes: tuple[str, ...] = ()
    colors: tuple[str, ...] = ()
    mana_cost_str: str = ""
    # Abilities the face-down characteristics grant. Default is none; Disguise
    # / Cloak / Cybermen override to ``("ward_2",)``.
    abilities: tuple[str, ...] = ()


# Canonical singleton for the default vanilla face-down characteristics
# (Morph, Manifest, Manifest-Dread, plain "turn face down"). Shared so every
# face_down_copy_effect node references the same hashable instance.
FACE_DOWN_VANILLA = FaceDownCharacteristics()

# Face-down variant that grants ward {2} (Disguise CR §702.168a, Cloak CR
# §701.58a, and the Doctor Who Cybermen keyword). A SINGLE abilities tuple
# entry "ward_2" labels the ward for the engine's ward-trigger wiring.
FACE_DOWN_WARD_2 = FaceDownCharacteristics(abilities=("ward_2",))


# Turn-face-up is a SPECIAL ACTION (CR §116 / §702.37e), not a spell/ability
# — it doesn't go on the stack. We model it as an Effect node so per-card
# handlers and policies can schedule / reason about it uniformly, but the
# engine's dispatcher must NEVER push it onto game.stack.
@dataclass(frozen=True)
class TurnFaceUp(EffectNode):
    """Turn a face-down permanent face up (CR §702.37e / §702.168d / §708.7).

    This is a **special action** (CR §116.2g) — it doesn't use the stack.
    The controller pays ``cost`` (the permanent's morph / megamorph /
    disguise cost), shows the face, and the face-down characteristic
    override ends. The resulting face-up permanent retains its prior
    damage / counters / attachments (CR §708.8).

    Fields:
      target           — the face-down Permanent (or a structural marker
                         like SELF for parser-time emission).
      cost             — Cost node (the flip cost). May be None for
                         "turn face up for free" / Morph-granted-via-copy
                         cases where the copy lacks a morph cost (CR
                         §702.37e — in which case the permanent CAN'T be
                         turned face up this way).
      megamorph        — True iff the flip is a megamorph cost payment
                         (CR §702.37b adds a +1/+1 counter on turn up).
      triggers_ward    — False for normal flips; set True if some future
                         ability wraps flipping in a targeted effect.
    """
    target: Filter = field(default_factory=lambda: Filter(base="self", targeted=False))
    cost: Optional[Cost] = None
    megamorph: bool = False
    triggers_ward: bool = False
    kind: str = field(init=False, default="turn_face_up")


# ============================================================================
# Scaling amounts (dynamic integer expressions)
# ============================================================================
#
# Many effects read an integer that depends on board state at resolution time:
# "equal to your devotion to black" (Gray Merchant), "equal to the number of
# creatures you control" (Overrun), "equal to X, where X is the number of
# cards in your graveyard" (Gurmag Angler's cost reduction). Prior to this
# node, the parser stuffed such scaling text into the leaf effect's raw_text
# via an UnknownEffect wrapper; the resolver then treated the amount as static
# (usually 1) which silently produced wrong gameplay.
#
# ``ScalingAmount`` is a first-class amount expression. It can appear anywhere
# a static int would (``amount``, ``count``, ``mana_value`` filter values).
# The resolver calls ``resolve_amount`` (in playloop.py) to evaluate it
# against the current ``Game`` / ``source_seat``.
#
# Two hard-rule constraints informed the shape:
#   1. We DO NOT mutate existing AST node field types. ``amount: Union[int, str]``
#      already accepts any Python value at runtime (dataclass type hints are
#      not enforced), so an engine that receives a ``ScalingAmount`` here
#      simply needs a new branch in its amount-reader.
#   2. The field is frozen and hashable (consumers put AST nodes in sets for
#      signature comparison). We use ``tuple`` for args.

@dataclass(frozen=True)
class ScalingAmount:
    """A dynamic integer expression evaluated against game state at resolution.

    ``kind`` discriminates the scaling variant; ``args`` carries kind-specific
    parameters. Canonical kinds (resolver must handle each):

      ``devotion``          args=(color_symbol,)  — e.g. ("B",) for devotion to black
                                                    args=("B","W") for devotion to black+white
      ``creatures_you_control`` args=(filter,)    — count of permanents matching filter
      ``permanents_you_control`` args=(filter,)
      ``cards_in_zone``     args=(zone, whose)    — zone ∈ {graveyard, hand, library, exile};
                                                    whose ∈ {you, target, each_opp, all_opp, each_player}
      ``life_lost_this_way`` args=()              — scoped to the resolving spell/ability
      ``life_gained_this_turn`` args=(whose,)
      ``counters_on_self``  args=(counter_kind,)  — e.g. "+1/+1", "charge"
      ``x``                 args=()               — the X from a cost-with-X (resolved from
                                                    the spell's on-stack x_value)
      ``literal``           args=(int,)           — escape hatch for patterns that parse
                                                    to a ScalingAmount for shape-consistency
                                                    but are actually static
      ``raw``               args=(text,)          — unknown scaling; resolver should log
                                                    an UnknownEffect-equivalent warning.
    """
    kind: str
    args: tuple = ()

    def __repr__(self) -> str:  # prettier debug output
        if not self.args:
            return f"ScalingAmount({self.kind})"
        return f"ScalingAmount({self.kind}:{','.join(repr(a) for a in self.args)})"


# Type alias for any effect node
Effect = EffectNode


# ============================================================================
# Modifications (for static abilities)
# ============================================================================

@dataclass(frozen=True)
class Modification:
    """A static-ability body. Anthems, replacement effects, restrictions, type-adds.

    `layer` tags the §613 layer this modification operates in, so the engine
    can resolve continuous effects in the correct order without re-deriving:
      '1'  = copy effects
      '2'  = control-changing
      '3'  = text-changing
      '4'  = type/subtype/supertype changes
      '5'  = color-changing
      '6'  = ability add/remove
      '7a' = characteristic-defining P/T
      '7b' = P/T set ("becomes N/N")
      '7c' = P/T modify (anthems +N/+N)
      '7d' = counters (+1/+1, -1/-1)
      '7e' = P/T switching
      None = not a layered effect (activated costs, triggers, spell effects, timing restrictions).
    """
    kind: str
    args: tuple = ()
    layer: Optional[str] = None


# ============================================================================
# Abilities (top-level types per comp rules §113)
# ============================================================================

@dataclass(frozen=True)
class Static:
    condition: Optional[Condition] = None
    modification: Optional[Modification] = None
    raw: str = ""

    @property
    def kind(self) -> str:
        return "static"


@dataclass(frozen=True)
class Activated:
    cost: Cost
    effect: Effect
    timing_restriction: Optional[str] = None  # "sorcery" / "once_per_turn" / etc.
    raw: str = ""

    @property
    def kind(self) -> str:
        return "activated"


@dataclass(frozen=True)
class Triggered:
    trigger: Trigger
    effect: Effect
    intervening_if: Optional[Condition] = None
    raw: str = ""

    @property
    def kind(self) -> str:
        return "triggered"


@dataclass(frozen=True)
class Keyword:
    name: str
    args: tuple = ()
    raw: str = ""

    @property
    def kind(self) -> str:
        return "keyword"


Ability = Union[Static, Activated, Triggered, Keyword]


# ============================================================================
# Card AST (top level)
# ============================================================================

@dataclass(frozen=True)
class CardAST:
    name: str
    abilities: tuple[Ability, ...] = ()
    parse_errors: tuple[str, ...] = ()    # unconsumed token ranges, with offsets
    fully_parsed: bool = True             # True iff parse_errors is empty
    # ------------------------------------------------------------------
    # Face-down family fields (CR §702.37 Morph, §702.168 Disguise,
    # §701.40 Manifest, §701.58 Cloak). These are ADDITIVE — default None/
    # False so every existing consumer (signature, asdict, Go loader) sees
    # the same schema unless a card actually has a morph/disguise cost.
    # ------------------------------------------------------------------
    # Morph / megamorph / disguise flip cost (the cost to turn a face-down
    # permanent face up — CR §702.37a, §702.37e, §702.168a, §702.168d).
    # This is stored SEPARATELY from the mana cost ManaCost attribute on
    # Activated abilities because the flip is a SPECIAL ACTION, not an
    # activated ability, and must not be processed by the activated-ability
    # dispatcher. ``None`` for cards with no flip cost (Manifest / Cloak /
    # Manifest Dread — these cards are turned face up by paying the
    # original mana cost iff the card is a creature card; CR §701.40a).
    morph_cost: Optional[ManaCost] = None
    # Disguise cost — same shape as morph_cost but semantically distinct
    # (the flipped permanent has ward {2} while face down, and the morph
    # procedure's wording differs). A card MAY have both (hypothetical
    # future design space) but today's printed cards have exactly one.
    disguise_cost: Optional[ManaCost] = None
    # True iff this card has an ability that CREATES face-down permanents
    # (i.e. has "manifest" / "manifest dread" / "cloak" in its effect
    # text). Used by the Go loader to fast-filter cards that produce
    # face-down tokens without having to walk the AST.
    manifest_token: bool = False
    # True iff the card itself has a morph/megamorph ability (so it CAN
    # be cast face-down via the §702.37c special-action pathway). Shallow
    # predicate: the engine additionally scans ``abilities`` for a
    # Keyword(name="morph" / "megamorph" / "disguise").
    has_morph: bool = False
    has_megamorph: bool = False
    has_disguise: bool = False


# ============================================================================
# AST equivalence helpers
# ============================================================================

def signature(ast: CardAST) -> tuple:
    """A hashable structural fingerprint of the AST. Two cards with identical
    signatures share an effect handler.

    Strategy: walk the abilities in canonical order, emit a tuple of
    (kind, sub-fingerprint) for each. Sub-fingerprints elide variable params
    (specific creature names, mana costs above a threshold) but preserve
    structural type information."""
    parts = []
    for ab in sorted(ast.abilities, key=lambda a: a.kind):
        parts.append(_ability_sig(ab))
    return tuple(parts)


def _ability_sig(ab: Ability) -> tuple:
    if isinstance(ab, Keyword):
        return ("kw", ab.name)
    if isinstance(ab, Static):
        return ("static",
                ab.modification.kind if ab.modification else "?",
                ab.condition.kind if ab.condition else "_")
    if isinstance(ab, Activated):
        return ("act",
                _cost_sig(ab.cost),
                _effect_sig(ab.effect))
    if isinstance(ab, Triggered):
        return ("trig",
                ab.trigger.event,
                _effect_sig(ab.effect))
    return ("?", repr(ab))


def _cost_sig(c: Cost) -> tuple:
    flags = []
    if c.tap: flags.append("T")
    if c.untap: flags.append("Q")
    if c.sacrifice: flags.append(f"sac:{c.sacrifice.base}")
    if c.discard: flags.append(f"discard:{c.discard}")
    if c.pay_life: flags.append(f"life:{c.pay_life}")
    if c.exile_self: flags.append("exile_self")
    if c.mana: flags.append(f"mana:{c.mana.cmc}")
    return tuple(flags) or ("free",)


def _effect_sig(e: Effect) -> tuple:
    if e is None:
        return ("none",)
    if isinstance(e, Sequence):
        return ("seq",) + tuple(_effect_sig(i) for i in e.items)
    if isinstance(e, Choice):
        return ("choice", e.pick) + tuple(_effect_sig(o) for o in e.options)
    if isinstance(e, Optional_):
        return ("opt", _effect_sig(e.body))
    if isinstance(e, Conditional):
        return ("if", e.condition.kind if e.condition else "?", _effect_sig(e.body))
    # Leaf effects: kind + a small set of distinguishing params
    extras = ()
    if hasattr(e, "count"):
        extras = (e.count,)
    elif hasattr(e, "amount"):
        extras = (e.amount,)
    elif hasattr(e, "pt"):
        extras = (e.pt,)
    return (e.kind,) + extras


__all__ = [
    "ManaSymbol", "ManaCost", "Filter", "Trigger", "Cost", "Condition",
    "Sequence", "Choice", "Optional_", "Conditional",
    "Damage", "Draw", "Discard", "Mill", "Scry", "Surveil", "CounterSpell",
    "Destroy", "Exile", "Bounce", "Tutor", "Reanimate", "Recurse",
    "GainLife", "LoseLife", "SetLife", "Sacrifice", "CreateToken",
    "CounterMod", "Buff", "GrantAbility", "TapEffect", "UntapEffect",
    "AddMana", "GainControl", "CopySpell", "CopyPermanent", "Fight",
    "Reveal", "LookAt", "Shuffle", "ExtraTurn", "ExtraCombat",
    "WinGame", "LoseGame", "Replacement", "Prevent", "UnknownEffect",
    "ScalingAmount",
    "FaceDownCharacteristics", "FACE_DOWN_VANILLA", "FACE_DOWN_WARD_2",
    "TurnFaceUp",
    "Modification", "Static", "Activated", "Triggered", "Keyword",
    "CardAST", "signature",
    "TARGET_ANY", "TARGET_CREATURE", "TARGET_PLAYER", "TARGET_OPPONENT",
    "EACH_OPPONENT", "EACH_PLAYER", "SELF",
]
