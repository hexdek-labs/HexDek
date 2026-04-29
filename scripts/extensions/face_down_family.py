"""Face-down creature mechanic family.

Covers the seven face-down mechanics defined by the Comprehensive Rules:

    mechanic         CR section        ships                notes
    Morph            §702.37           Onslaught 2002       pay {3} face-down; flip for morph cost
    Megamorph        §702.37b          Dragons of Tarkir    flip gains +1/+1 counter
    Manifest         §701.40           Fate Reforged        top-of-library face-down; flip for mana cost if creature
    Manifest Dread   §701.62           Murders at Karlov    look 2, manifest 1, other to graveyard
    Cloak            §701.58           Murders at Karlov    manifest-like; face-down has ward {2}
    Disguise         §702.168          Thunder Junction     pay {3} face-down; face-down has ward {2}
    Cybermen         (custom keyword)  Doctor Who           face-down w/ ward {2}, custom flip procedure

All seven share the characteristic-override semantics defined in CR §708.2a:
a face-down permanent is a 2/2 colorless creature with no name, no subtypes,
no text, and no mana cost. Disguise / Cloak / Cybermen additionally grant the
face-down form ward {2}.

Implementation strategy
-----------------------
1. Face-down characteristic override is a §613.2b layer-1b copy effect
   modelled as ``Modification(kind="face_down_copy_effect", layer="1")``.
   The engine's ``get_effective_characteristics`` walks layer 1 first; if
   ``perm.face_down`` is True the face-down characteristics replace all
   other copiable values (CR §708.2a / §613.2b).

2. Morph / Megamorph / Disguise are KEYWORD ABILITIES. Their flip cost is
   parsed into ``CardAST.morph_cost`` (or ``disguise_cost``) so the engine
   can route turn-face-up (a special action, CR §116.2g / §702.37e) without
   walking every ability. The ``Keyword`` node still appears in
   ``CardAST.abilities`` for backwards-compatibility with existing
   consumers.

3. Manifest / Manifest Dread / Cloak are KEYWORD ACTIONS. They appear inside
   effect text ("manifest the top card of your library") and produce
   face-down permanents. We emit ``Modification(kind="manifest_action",
   args=(count, source_zone))`` or ``Modification(kind="cloak_action",
   args=(count,))`` so per-card-runtime can map the action to the engine's
   ``manifest(game, seat, n)`` / ``cloak(game, seat, n)`` helpers.

4. Cybermen is a custom Doctor Who keyword that most printings render as
   oracle text reading "{cost}: Turn this creature face up. Cybermen." Its
   CR posture is "custom set-specific keyword" — treated as a morph-variant
   with ward {2} while face down.

Why a dedicated extension
-------------------------
The base parser's ``KEYWORD_RE`` already recognises ``morph {cost}`` and
``disguise {cost}`` as Keyword nodes. But three things it does NOT do:

  - Extract the flip cost into a typed ``ManaCost`` on ``CardAST.morph_cost``.
    Without that, the engine can't price the turn-face-up special action.
  - Emit a Static with ``face_down_copy_effect`` so layer-1b resolution
    knows which cards CAN be face-down.
  - Typify the ``manifest`` / ``cloak`` effect actions as first-class
    effect nodes so ``playloop.manifest()`` can be invoked from a
    generic dispatcher rather than per-card handlers.

Parser-output contract
----------------------
For each recognised face-down mechanic, we produce ONE or more of:

  1. A Keyword ability (morph / megamorph / disguise).
  2. A Static ability with
     ``Modification(kind="face_down_copy_effect", layer="1",
                    args=(variant,))``
     where ``variant ∈ {"vanilla", "ward_2"}``. The variant is
     "ward_2" iff the card has Disguise / Cloak / Cybermen. The engine's
     §613 layer-1b apply_fn reads this modification to pick the right
     FaceDownCharacteristics instance.
  3. Extracted mana costs on ``CardAST.morph_cost`` /
     ``CardAST.disguise_cost`` — NOT on the Keyword node itself (which
     carries the raw ``args=(cost_str,)`` string).
  4. For manifest/cloak/manifest-dread effect actions: an effect-rule
     emitter that produces a typed ``Modification(kind="manifest_action",
     ...)`` Static wrapper when the action appears as a spell's effect,
     or a bare ``Modification`` embedded in a Triggered's effect when it
     appears inside a trigger body.

Coverage strategy
-----------------
The base KEYWORD_RE already matches ``morph {cost}`` / ``disguise {cost}``
as Keyword nodes, so the existing cards parse GREEN without this
extension. This extension is PURE UPGRADE: we don't lose coverage, but we
REPLACE the opaque Keyword nodes with a richer (Keyword + Static +
morph_cost) triple so the runtime layer has the data it needs without
re-parsing the ability text.

We achieve this by post-processing in ``parse_card`` rather than by adding
to the grammar, because the keyword is already recognised. Specifically:
extension modules can't hook ``parse_card`` directly, so we implement
PER_CARD_HANDLERS for the handful of face-down cards we want to upgrade
— but that's N^2 in the card count. Better: we implement the upgrade as
an EFFECT_RULES / STATIC_PATTERNS entry that fires BEFORE the base
KEYWORD_RE, producing a *richer* node.

For manifest / cloak / manifest-dread effect actions we add EFFECT_RULES
that emit typed nodes.

CR citations summary
--------------------
  §116.2g       — turning a face-down permanent face up is a special action
  §613.2b       — face-down characteristics applied in layer 1b
  §613.4        — layer 7 (P/T) runs AFTER layer 1, so counters on a
                  face-down creature still matter (2/2 + 1 +1/+1 = 3/3)
  §701.40       — Manifest keyword action
  §701.40a      — face-down card is 2/2 with no text/name/subtypes/mana cost
  §701.40c/d    — if manifested card had morph/disguise, EITHER flip
                  procedure may be used
  §701.58       — Cloak keyword action
  §701.58a      — cloaked permanent is 2/2 with ward {2}, no name/subtypes/
                  mana cost; ward attaches to the face-down characteristics
  §701.62       — Manifest Dread
  §701.62a      — "Look at top 2, manifest 1, put others in graveyard"
  §702.37       — Morph
  §702.37a      — Morph [cost] means "You may cast this card as a 2/2 face-
                  down creature with no text, no name, no subtypes, and no
                  mana cost by paying {3} rather than paying its mana cost"
  §702.37b      — Megamorph: turn-face-up adds a +1/+1 counter
  §702.37c      — casting face-down procedure
  §702.37e      — turn-face-up special action: pay the morph cost, then
                  turn face up, face-down effect ends
  §702.168      — Disguise
  §702.168a     — Disguise [cost] is like Morph but face-down has ward {2}
  §702.168d     — turn-face-up procedure for disguise
  §708.2a       — default face-down characteristics (2/2, no text/name/
                  subtypes/mana cost, colorless per 708.2a + absence of
                  mana cost)
  §708.3        — face-down ETB doesn't fire face-up ETB triggers
  §708.4        — face-down spells on stack carry the face-down
                  characteristics
  §708.7/§708.8 — turning a face-down permanent face up: effects already
                  applied still apply, but face-up ETB triggers do NOT
                  fire
  §708.10       — face-down copy: still face-down; face-up reveals copied
                  values

Out of scope (NOT implemented here, flagged for future work)
------------------------------------------------------------
  - Morph characteristic-defining abilities (CDA) that alter the face-down
    characteristics (§604.3, rare; current printings: none alter the 2/2).
  - Face-up-via-foreign-effect paths (e.g. a card targeted by
    "turn target face-down creature face up" without paying the morph
    cost — CR §702.37e says such an effect DOES turn it face up but does
    NOT trigger megamorph counter placement, since megamorph only fires
    "if its megamorph cost was paid").
  - Exile-face-down zone handling (CR §406.3a — face-down in exile has
    no characteristics). Handled at the exile helper in playloop.
  - Face-down merged permanents (§729.2e/f). We flag but don't implement.
  - Cybermen's variable/X flip cost on some Doctor Who printings.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

# Allow the extension to import mtg_ast whether loaded via importlib.util
# (by parser.load_extensions) or by direct import path.
_HERE = Path(__file__).resolve().parent
if str(_HERE.parent) not in sys.path:
    sys.path.insert(0, str(_HERE.parent))

from mtg_ast import (  # noqa: E402
    Keyword, Static, Modification, Choice, Sequence,
    Mill, LookAt, UnknownEffect,
    Filter, ManaCost, ManaSymbol,
    SELF,
)


# ---------------------------------------------------------------------------
# Mana cost parsing (local helper — mirrors parser.parse_mana_cost so the
# extension doesn't need to import parser.py at module import time, which
# would create a circular dependency since parser.py imports extensions).
# ---------------------------------------------------------------------------
_MANA_TOKEN = re.compile(r"\{([^}]+)\}")


def _parse_mana_cost(s: str) -> Optional[ManaCost]:
    """Parse {X}{Y}{Z} sequences into a ManaCost. Returns None on empty."""
    syms: list[ManaSymbol] = []
    for m in _MANA_TOKEN.finditer(s):
        raw = "{" + m.group(1) + "}"
        body = m.group(1).strip().upper()
        if body in {"T", "Q", "E"}:
            continue
        sym = ManaSymbol(raw=raw)
        if body.isdigit():
            sym = ManaSymbol(raw=raw, generic=int(body))
        elif body == "X":
            sym = ManaSymbol(raw=raw, is_x=True)
        elif body == "S":
            sym = ManaSymbol(raw=raw, is_snow=True)
        elif "/" in body:
            parts = body.split("/")
            colors = tuple(p for p in parts if p in {"W", "U", "B", "R", "G", "C"})
            generic = next((int(p) for p in parts if p.isdigit()), 0)
            phyrexian = "P" in parts
            sym = ManaSymbol(
                raw=raw, generic=generic, color=colors,
                is_phyrexian=phyrexian,
            )
        elif body in {"W", "U", "B", "R", "G", "C"}:
            sym = ManaSymbol(raw=raw, color=(body,))
        syms.append(sym)
    return ManaCost(symbols=tuple(syms)) if syms else None


# ---------------------------------------------------------------------------
# STATIC_PATTERNS — parsed as Static ability bodies. Order matters: the most
# specific shape wins, so megamorph must beat morph, manifest-dread must
# beat manifest, cloak-N must beat bare cloak.
#
# Each builder returns a LIST of ability nodes (Keyword, Static, ...). The
# parser harness accepts either a single Ability or a list; see
# parse_keyword_list handling. If a builder returns an inconvenient shape
# we emit a tuple of abilities wrapped in a Keyword placeholder whose
# signature still clusters with the canonical face-down family.
#
# Reality check: parser.parse_static returns Optional[Static | Keyword], so
# returning a list is NOT directly supported at this call site. Instead, we
# produce the richest single node (a Keyword) and rely on the base KEYWORD_RE
# + our post-processing hook below to attach the face_down_copy_effect
# Static via PER_CARD_HANDLERS for cards that need the richer triple.
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# Morph {cost} — CR §702.37. Base KEYWORD_RE already catches this, but when
# the ability is shipped WITHOUT reminder text (rare — some promos strip
# it), parse_static is the final arbiter. We return a Keyword node with
# the parsed mana cost in args so downstream consumers (ast_dataset export
# + Go loader) can read it uniformly.
@_sp(r"^morph\s+(\{[^}]+(?:\}\{[^}]+)*\})\s*$")
def _sp_morph(m, raw):
    mc = _parse_mana_cost(m.group(1))
    return Keyword(name="morph",
                   args=(m.group(1), mc) if mc else (m.group(1),),
                   raw=raw)


# Megamorph {cost} — CR §702.37b. "A megamorph cost is a morph cost" per
# §702.37b, so the engine MUST NOT treat megamorph and morph as distinct
# special actions; the difference is the +1/+1 counter placement on flip.
@_sp(r"^megamorph\s+(\{[^}]+(?:\}\{[^}]+)*\})\s*$")
def _sp_megamorph(m, raw):
    mc = _parse_mana_cost(m.group(1))
    return Keyword(name="megamorph",
                   args=(m.group(1), mc) if mc else (m.group(1),),
                   raw=raw)


# Disguise {cost} — CR §702.168.
@_sp(r"^disguise\s+(\{[^}]+(?:\}\{[^}]+)*\})\s*$")
def _sp_disguise(m, raw):
    mc = _parse_mana_cost(m.group(1))
    return Keyword(name="disguise",
                   args=(m.group(1), mc) if mc else (m.group(1),),
                   raw=raw)


# Cybermen — Doctor Who universes-beyond keyword. Custom set-specific
# variant of morph with ward {2}. Oracle wording varies by printing, but
# the canonical form is "Cybermen {cost}".
@_sp(r"^cybermen\s+(\{[^}]+(?:\}\{[^}]+)*\})\s*$")
def _sp_cybermen(m, raw):
    mc = _parse_mana_cost(m.group(1))
    return Keyword(name="cybermen",
                   args=(m.group(1), mc) if mc else (m.group(1),),
                   raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES — for manifest / manifest-dread / cloak keyword actions
# appearing inside triggered / activated / spell effect text.
#
# Shape per entry: (compiled regex, builder). Appended to the base parser's
# EFFECT_RULES list at load_extensions() time. Match must consume (almost)
# the whole input text — parser.parse_effect requires m.end() >= len(text)-2.
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# -- Manifest / Manifest Dread / Cloak effect actions --
#
# IMPORTANT: legacy extension `aa_unknown_hunt.py` already emits
#   Modification(kind="manifest", args=("top_library",))
#   Modification(kind="manifest_dread", args=())
# for the narrow shapes "manifest (the) top card of (your) library" and
# "manifest dread". We MUST preserve those exact kinds and args so downstream
# consumers (per_card_runtime, Go loader) that pattern-match on them keep
# working.
#
# Our new rules below add:
#   - broader shape coverage ("manifest the top N cards", "cloak" action)
#   - multi-card manifest counts in args
#   - "cloak_action" kind (new — no prior handler)
#   - distinct variant for Manifest Dread that keeps backward-compatible kind
# Since EFFECT_RULES is a list and the first matching rule wins, we append
# AFTER aa_unknown_hunt (alphabetical load order: "aa_..." loads before
# "face_down_family"), so the legacy rules win for their narrow shapes.
# Our rules catch the patterns the legacy rules miss.
#
# CR §701.40a: to manifest a card, turn it face down and put it onto the
# battlefield as a 2/2 face-down creature. The card comes from the TOP of
# the manifesting player's LIBRARY (§701.40a). Multiple-manifest texts
# ("manifest the top N cards") are rare but printed; §701.40e says "cards
# are manifested one at a time" in that case.


@_er(r"^manifest the top (\d+|two|three|x) cards? of your library(?:\s*$|\.)")
def _fx_manifest_top_n(m):
    """Plural-N manifest. Args encode (count, source_zone) so downstream
    code can distinguish "manifest 2" from the legacy single-card kind.
    Uses a distinct kind="manifest_n" to keep legacy "manifest" clean."""
    word = m.group(1).lower()
    n: int | str
    if word.isdigit():
        n = int(word)
    elif word == "two":
        n = 2
    elif word == "three":
        n = 3
    else:
        n = "x"
    return Modification(
        kind="manifest_n",
        args=(n, "top_of_library"),
    )


# -- Cloak: CR §701.58a. "Cloak the top card of your library" or "cloak N".
# Cloaked permanent is a 2/2 face-down creature with ward {2}.


@_er(r"^cloak the top card of your library(?:\s*$|\.)")
def _fx_cloak_top(m):
    return Modification(
        kind="cloak",
        args=(1, "top_of_library"),
    )


@_er(r"^cloak the top (\d+|two|three|x) cards? of your library(?:\s*$|\.)")
def _fx_cloak_top_n(m):
    word = m.group(1).lower()
    n: int | str
    if word.isdigit():
        n = int(word)
    elif word == "two":
        n = 2
    elif word == "three":
        n = 3
    else:
        n = "x"
    return Modification(
        kind="cloak",
        args=(n, "top_of_library"),
    )


# -- Bare keyword action forms that appear as spell-effect sentences --


@_er(r"^manifest (?:a|one) card(?:\s*$|\.)")
def _fx_manifest_a(m):
    return Modification(
        kind="manifest",
        args=("top_library",),
    )


@_er(r"^cloak (\d+)(?:\s*$|\.)")
def _fx_cloak_n(m):
    return Modification(
        kind="cloak",
        args=(int(m.group(1)), "top_of_library"),
    )


# ---------------------------------------------------------------------------
# PER_CARD_HANDLERS — post-processing hook to enrich face-down-capable cards.
#
# We DON'T individually enumerate face-down cards (would be ~100 entries).
# Instead we expose a function the parser can call to upgrade any CardAST
# after the grammar run. parser.parse_card applies PER_CARD_HANDLERS by
# exact name match BEFORE the grammar runs, so this path is unsuitable.
#
# The clean approach: hook the POST-parse enrichment into
# ``scripts/extensions/per_card_runtime.py`` (the runtime registry) — but
# that would couple parse-time and run-time concerns.
#
# Chosen approach: `enrich_face_down_ast(ast, oracle_text)` is exposed for
# callers that want the richer triple. It's defensive-additive — returns
# a new CardAST with morph_cost / disguise_cost / manifest_token / has_morph
# fields set based on the parsed keywords. The parser's main loop imports
# this function at extension-load time and calls it on every CardAST before
# returning.
# ---------------------------------------------------------------------------

# Shared dataclasses from mtg_ast — imported lazily below to avoid cycles
# during test collection ordering.
from mtg_ast import CardAST  # noqa: E402


def _extract_first_mana_cost(kw: Keyword) -> Optional[ManaCost]:
    """Pull the ManaCost out of a Keyword's args if present.

    Our extension's STATIC_PATTERNS for Morph/Megamorph/Disguise/Cybermen
    tuck the parsed ManaCost into args[1] alongside the raw cost string.
    The base KEYWORD_RE path only captures args=(cost_string,), so we
    also accept a raw string and re-parse it.
    """
    for a in kw.args:
        if isinstance(a, ManaCost):
            return a
        if isinstance(a, str) and a.startswith("{"):
            mc = _parse_mana_cost(a)
            if mc is not None:
                return mc
    return None


def _ast_contains_action(ast: CardAST, kinds: tuple[str, ...]) -> bool:
    """Return True iff some ability in ``ast`` carries a Modification with
    one of the given kinds (directly or inside Static.modification /
    Triggered.effect / Activated.effect). Keeps recursion shallow —
    face-down actions appear at top-level effect positions.

    Must descend through ``Modification(kind='spell_effect', args=(inner,))``
    wrappers, where the legacy parser puts the real effect node inside
    ``args[0]``. A shallow ``isinstance(Modification)`` check would early-
    return on the outer wrapper, missing the inner node.
    """
    def _walk(node) -> bool:
        if node is None:
            return False
        if isinstance(node, Modification):
            if node.kind in kinds:
                return True
            # Don't early-return — descend into args for wrapped nodes like
            # Modification(kind="spell_effect", args=(Modification(kind="cloak"),)).
            # Fall through to the generic args-walker below.
        # Unpack common wrappers
        for attr in ("modification", "effect", "body"):
            v = getattr(node, attr, None)
            if v is not None and _walk(v):
                return True
        items = getattr(node, "items", None)
        if items:
            for it in items:
                if _walk(it):
                    return True
        options = getattr(node, "options", None)
        if options:
            for it in options:
                if _walk(it):
                    return True
        # Modification.args sometimes wrap nested effect nodes (spell_effect
        # wrapper pattern). Descend into those too.
        args = getattr(node, "args", None)
        if args:
            for it in args:
                if hasattr(it, "kind"):
                    if _walk(it):
                        return True
        return False
    for ab in ast.abilities:
        if _walk(ab):
            return True
    return False


def enrich_face_down_ast(ast: CardAST) -> CardAST:
    """Post-process a parsed CardAST to surface face-down metadata.

    This runs AFTER grammar parsing, via a hook wired from parser.parse_card.
    Idempotent and additive — if no face-down mechanic is present, returns
    ``ast`` unchanged. Otherwise returns a new frozen CardAST with:

      - ``morph_cost`` set from any Keyword(name="morph" or "megamorph")
      - ``disguise_cost`` set from any Keyword(name="disguise" or "cybermen")
      - ``has_morph`` / ``has_megamorph`` / ``has_disguise`` flags set
      - ``manifest_token`` True iff a manifest/cloak/manifest_dread action
        is anywhere in the ability tree
      - An additional Static ability appended that carries the §613 layer-1b
        ``face_down_copy_effect`` modification, so the engine's §613
        resolver sees it via the abilities list.

    CR cite: §702.37a / §702.168a define morph/disguise as STATIC abilities
    that function in any zone, so the added Static is CR-accurate.
    """
    morph_cost: Optional[ManaCost] = None
    disguise_cost: Optional[ManaCost] = None
    has_morph = False
    has_megamorph = False
    has_disguise = False
    has_cybermen = False

    has_manifest_keyword = False
    for ab in ast.abilities:
        if not isinstance(ab, Keyword):
            continue
        n = (ab.name or "").lower()
        if n == "morph":
            has_morph = True
            if morph_cost is None:
                morph_cost = _extract_first_mana_cost(ab)
        elif n == "megamorph":
            has_megamorph = True
            has_morph = True  # megamorph IS morph per §702.37b
            if morph_cost is None:
                morph_cost = _extract_first_mana_cost(ab)
        elif n == "disguise":
            has_disguise = True
            if disguise_cost is None:
                disguise_cost = _extract_first_mana_cost(ab)
        elif n == "cybermen":
            has_cybermen = True
            # Cybermen uses disguise_cost slot — same runtime treatment
            # (ward {2} face-down, special-action flip).
            if disguise_cost is None:
                disguise_cost = _extract_first_mana_cost(ab)
        elif n in ("manifest", "manifest dread", "manifest_dread",
                   "cloak"):
            # These aren't keyword ABILITIES (they're keyword ACTIONS per
            # CR §701.40 / §701.58 / §701.62), but the base KEYWORD_RE path
            # sometimes captures them when they're the whole ability line
            # (as in Soul Summons: "Manifest the top card of your library").
            # Flag the card as producing face-down permanents.
            has_manifest_keyword = True

    # manifest_token — True if the card has a manifest/cloak/manifest-dread
    # keyword action anywhere in its ability tree. We match ALL known kinds
    # (legacy "manifest"/"manifest_dread" from aa_unknown_hunt.py AND the
    # new "manifest_n" / "cloak" kinds from this extension).
    has_manifest_action = _ast_contains_action(
        ast,
        (
            "manifest", "manifest_n", "manifest_dread",
            "cloak", "cloak_action",
            # Accept "manifest_action" too as a forward-compat alias in case
            # some future extension emits that name.
            "manifest_action",
        ),
    ) or has_manifest_keyword

    if not (has_morph or has_megamorph or has_disguise or has_cybermen
            or has_manifest_action):
        # Nothing to do — plain card.
        return ast

    # Attach a §613.2b layer-1b Static modification that declares THIS
    # card CAN exist as a face-down permanent. The variant discriminator
    # tells the engine which FaceDownCharacteristics to apply:
    #   - "vanilla": Morph/Megamorph/Manifest/Manifest-Dread (no ward)
    #   - "ward_2":  Disguise/Cloak/Cybermen (face-down has ward {2})
    added: list = list(ast.abilities)
    if has_disguise or has_cybermen:
        variant = "ward_2"
    elif has_morph or has_megamorph:
        variant = "vanilla"
    else:
        # Only a manifest/cloak action — the SOURCE of the action, not the
        # CARD being manifested. Don't add a face_down_copy_effect Static
        # to the source; the created face-down permanent gets its override
        # from the manifest/cloak keyword-action helpers in playloop.
        variant = None

    if variant is not None:
        added.append(Static(
            modification=Modification(
                kind="face_down_copy_effect",
                args=(variant,),
                layer="1",
            ),
            raw=f"(face-down characteristic override — variant={variant})",
        ))

    return CardAST(
        name=ast.name,
        abilities=tuple(added),
        parse_errors=ast.parse_errors,
        fully_parsed=ast.fully_parsed,
        morph_cost=morph_cost,
        disguise_cost=disguise_cost,
        manifest_token=has_manifest_action,
        has_morph=has_morph,
        has_megamorph=has_megamorph,
        has_disguise=(has_disguise or has_cybermen),
    )


# Expose the enrichment hook to parser.py. parser.load_extensions() walks
# extension modules for these attribute names. We add ``POST_PARSE_HOOKS``
# as a new extension contract — a list of (CardAST -> CardAST) functions
# applied after grammar parsing. If parser.py doesn't know about this yet,
# a one-line change there wires it in.
POST_PARSE_HOOKS = [enrich_face_down_ast]


__all__ = [
    "STATIC_PATTERNS", "EFFECT_RULES", "POST_PARSE_HOOKS",
    "enrich_face_down_ast",
]

# TRIGGER_PATTERNS intentionally empty — the base parser already handles
# "when this creature is turned face up" via its EVENT_VERBS entry
# "is turned face up" → "turned_face_up". Verified via:
#     python3 scripts/parser.py --card "Ainok Survivalist"
# (produces Triggered(trigger=Trigger(event='turned_face_up', ...))).
TRIGGER_PATTERNS: list = []
