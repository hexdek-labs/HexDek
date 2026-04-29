"""Card-type changes and animation effects.

Family: CARD-TYPE CHANGES / ANIMATIONS.

Covers every shape in which a permanent temporarily or permanently changes
its card types, subtypes, colors, P/T, or gains abilities via an animation
effect. This is the "becomes / is also / is no longer" grammar surface.

Coverage targets (Apr 2026 parser_coverage snapshot):

  ~400 fragments  "... becomes [a|an] ... [creature|artifact|...] ..."
     ~ 8 fragments "... is also a <type>"
     ~ 3 fragments "... is no longer <type|counter>"
     ~ 6 fragments "... is every creature type"
     ~30 fragments "it's still a land" post-animation tail
     ~60 fragments "in addition to its other types/colors" type-add tails

Shapes (precedence: most specific first):

EFFECT_RULES (ability-body patterns, wired before old_templating catch-all):

  - "<subject> becomes a <P/T>? <colors>? <types> [creature|artifact|...]
     [with <keywords>] [in addition to ...]? [until end of turn|permanently]?"
       → UnknownEffect(raw_text="animate:...") carrying a structured key.
     Covers manlands ({t}: target land becomes a 3/3), vehicle activations
     ("this vehicle becomes an artifact creature"), enchantment-creatures
     ("if this permanent is an enchantment, it becomes ..."), aura-type
     transforms ("enchanted creature becomes a 0/0 artifact creature"),
     planeswalker animations ("+2: ~ becomes a 5/5 soldier creature
     that's still a planeswalker"), and pronoun-led "it becomes a ...".

  - "<subject> becomes a copy of <filter> [except ...] [until end of turn]"
       → UnknownEffect(raw_text="becomes_copy:...").
     Covers Cytoshape, Mirrorweave, Copy Enchantment, Oko PW copy, Sakashima,
     and "becomes a copy of a creature card exiled with ~".

  - "<subject> becomes [a|an] <basic-land-type>"
       → UnknownEffect(raw_text="become_basic:TYPE")
     Covers Spreading Seas, Sea's Claim, Phantasmal Terrain.

  - Tail: "it's still a land" / "that's still a <type>"
     → Handled as a static on its own line AND as an optional tail consumed
     inside the animate regex.

  - Tail: "in addition to its other types/colors"
     → Type-add rider attached to the animation; separately matched as
     Static when it stands alone ("it becomes a cat in addition to its
     other types").

STATIC_PATTERNS (parse_static, pre-catchall):

  - "it's still a <type>"                      → type_add_still
  - "that's still a <type>"                    → type_add_still
  - "<subject> is also a <type-list>"          → type_add
  - "<subject> is a <type> in addition to ..." → type_add
  - "<subject> is every creature type"         → changeling_static
  - "<subject> is no longer <qualifier>"       → type_remove
  - "it can't be a creature"                   → anti_animate
  - "~ is [no longer] a <type>"                → type_remove / type_add
  - Reminder-style "it's still a land and has its other abilities"
     → type_add_still_full

No TRIGGER_PATTERNS exported here — animation triggers ("whenever this
vehicle becomes crewed", "whenever ~ attacks") already live in
vehicles_mounts.py and combat_triggers.py; this extension only recognises
the EFFECTS those triggers produce.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Optional

# Allow the extension to import mtg_ast from scripts/.
_HERE = Path(__file__).resolve().parent
if str(_HERE.parent) not in sys.path:
    sys.path.insert(0, str(_HERE.parent))

from mtg_ast import (  # noqa: E402
    Modification, Static, UnknownEffect,
)


# ===========================================================================
# Shared fragments
# ===========================================================================

# Subjects that can be animated. Broad — we don't gate on exact noun since
# downstream just needs a structured key.
_SUBJECT = (
    r"(?:"
    r"~|"
    r"it|that|this|they|"
    r"(?:this|that|target|each|every|another|up to (?:one|two|three) target|"
    r"all|every other|each other|a random) "
    r"(?:non[a-z]+ |noncreature |nontoken |nonland |nonbasic |creature |"
    r"artifact |enchantment |land |permanent |vehicle |token |snow |"
    r"legendary |basic |shapeshifter |spell |instant or sorcery )?"
    r"(?:creature|permanent|artifact|enchantment|land|vehicle|token|"
    r"planeswalker|card|spell)(?: (?:you|an opponent|a player) controls?)?"
    r"(?: or (?:creature|land|vehicle|artifact|enchantment|permanent))?|"
    r"enchanted (?:creature|permanent|artifact|land|player)|"
    r"equipped (?:creature|permanent)|"
    r"this (?:vehicle|mount|creature|artifact|enchantment|land|permanent|spacecraft|spell|card)|"
    r"that (?:creature|permanent|artifact|enchantment|land|card|spell)|"
    r"oko|[a-z]+,?\s+the\s+[a-z ]+"  # loose named-PW escape hatch
    r")"
)

# Optional power/toughness: "3/3 ", "x/x ", "*/* "
_PT = r"(?:(?:\d+|x|\*)/(?:\d+|x|\*)\s+)?"

# Optional colors: "blue ", "white and black ", "all colors ", "colorless "
_COLORS = (
    r"(?:(?:white|blue|black|red|green|colorless)"
    r"(?:(?: and | or )(?:white|blue|black|red|green|colorless))*\s+|"
    r"all colors\s+|mono(?:colored)?\s+|"
    r"(?:[a-z]+-colored)\s+)?"
)

# Duration tails we accept after the animation body.
_DURATION = (
    r"(?:\s+until end of turn|"
    r"\s+until your next turn|"
    r"\s+permanently|"
    r"\s+until your next upkeep|"
    r"\s+for (?:as long as|the rest of the game)[^.]*|"
    r"\s+this turn)?"
)

# Type-add rider after the animation ("in addition to its other types/colors")
_ADDITION = r"(?:\s+in addition to (?:its|their) other (?:types|colors|types and colors|colors and types))?"

# "it's still a land" tail ON the same sentence.
_STILL = r"(?:\.\s+(?:it'?s|they'?re) still (?:a|an) [a-z ]+)?"


# ===========================================================================
# EFFECT_RULES (ability-body matchers)
#
# parser.parse_effect picks the first EFFECT_RULES entry that consumes the
# WHOLE text. So each regex here anchors with ^ and ends with a flexible tail.
# Extension rules are appended AFTER built-ins and BEFORE late catch-alls,
# which is fine for our shapes — built-in parser has no "becomes a creature"
# rule (only old_templating has a narrow one), so ours wins for the general
# case.
# ===========================================================================

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---------------------------------------------------------------------------
# "<subject> becomes a copy of <X> [except ...] [until ...]"
# Placed BEFORE the generic animate rule because "copy" is a strict subtype
# of the becomes-a family.
# ---------------------------------------------------------------------------
@_er(
    r"^(?:until end of turn,?\s+)?"
    r"(.+?) becomes? (?:a |an )?copy of ([^.]+?)"
    r"(?:,?\s+except [^.]+?)?"
    r"(?:\s+until end of turn|\s+until your next turn|\s+permanently|"
    r"\s+for the rest of the game[^.]*)?\.?$"
)
def _becomes_copy(m):
    subj = m.group(1).strip()
    target = m.group(2).strip()
    return UnknownEffect(
        raw_text=f"becomes_copy:subj={subj};target={target}"
    )


# ---------------------------------------------------------------------------
# "<subject> becomes a/an <basic-land-type>" (Spreading Seas / Sea's Claim)
# Single-word land-type transform. Keep ABOVE the generic animate because
# the animate rule requires a card-type noun ("creature"/"artifact"/...).
# ---------------------------------------------------------------------------
_BASIC_LAND = r"(?:plains|island|swamp|mountain|forest|wastes)"

@_er(
    r"^(?:until end of turn,?\s+)?"
    r"(.+?) becomes? (?:a |an )?"
    rf"({_BASIC_LAND})"
    rf"{_ADDITION}"
    rf"{_DURATION}\.?$"
)
def _becomes_basic_land(m):
    subj = m.group(1).strip()
    land_type = m.group(2).lower()
    return UnknownEffect(raw_text=f"become_basic:subj={subj};type={land_type}")


# ---------------------------------------------------------------------------
# Core "becomes a creature/artifact/..." animation rule.
#
# Structure:
#   [until end of turn,]? <subj> [loses <kw> and ]? becomes
#   [a|an] <P/T>? <colors>? <subtypes/types>
#   [creature|artifact|enchantment|land|planeswalker|...] card-type
#   [with <keywords>]? [named <name>]?
#   [in addition to its other types/colors]?
#   [until end of turn|permanently|...]?
#   [. it's still a land]?
#
# Intentionally greedy on the descriptor; we capture whatever lies between
# "becomes a" and the duration / addition / still-tail, and store it as a
# raw descriptor in the UnknownEffect key so downstream engines can further
# decompose if needed.
# ---------------------------------------------------------------------------
_CARDTYPES = (
    r"(?:creature|artifact|enchantment|land|planeswalker|vehicle|spacecraft|"
    r"artifact creature|enchantment creature|artifact land|aura|"
    r"artifact enchantment|legendary creature|creature enchantment)"
)

@_er(
    r"^(?:until end of turn,?\s+|until your next turn,?\s+)?"
    r"(.+?)"                                    # subject (+ optional "loses X and")
    r" (?:perpetually )?becomes? (?:a |an )?"
    rf"({_PT})"                                 # optional P/T
    r"([^.]*?)\s+"                              # descriptor (colors + subtypes)
    rf"({_CARDTYPES})"
    r"(?:\s+named [^.,]+?)?"
    r"(?:\s+with [^.]*?)?"                      # keyword list or quoted text
    rf"{_ADDITION}"
    rf"{_DURATION}"
    r"(?:,?\s+where x is [^.]+?)?"
    r"(?:\.\s+(?:it'?s|they'?re) still (?:a|an) [a-z ]+)?"
    r"\.?$"
)
def _becomes_creature_etc(m):
    subj = m.group(1).strip()
    pt = (m.group(2) or "").strip()
    descr = (m.group(3) or "").strip()
    ctype = m.group(4).strip().lower()
    return UnknownEffect(
        raw_text=f"animate:subj={subj};pt={pt};descr={descr};type={ctype}"
    )


# ---------------------------------------------------------------------------
# Short pronoun-led "it becomes a <type>" (the descriptor is just a subtype,
# no card-type word). Covers "it becomes a rogue in addition to its other
# types", "it becomes a cat", "that creature becomes a coward".
# ---------------------------------------------------------------------------
@_er(
    r"^(?:until end of turn,?\s+)?"
    r"(it|that (?:creature|permanent|card)|this (?:creature|permanent|card)|"
    r"target (?:creature|permanent|token|land)(?: (?:you|an opponent) controls?)?) "
    r"becomes? (?:a |an )?"
    r"([a-z][a-z ,'-]+?)"
    rf"{_ADDITION}"
    rf"{_DURATION}\.?$"
)
def _becomes_subtype(m):
    subj = m.group(1).strip()
    subtype = m.group(2).strip()
    # Don't swallow things that look like a full card-type line — the main
    # rule should have caught those; if it didn't, still capture.
    return UnknownEffect(
        raw_text=f"type_change:subj={subj};to={subtype}"
    )


# ---------------------------------------------------------------------------
# "<subject> becomes all colors" / "becomes a random color"
# ---------------------------------------------------------------------------
@_er(
    r"^(?:until end of turn,?\s+)?"
    r"(.+?) becomes? (all colors|a random color|the chosen color|"
    r"the color of [^.]+?)"
    rf"{_DURATION}\.?$"
)
def _becomes_color(m):
    subj = m.group(1).strip()
    color = m.group(2).strip()
    return UnknownEffect(raw_text=f"color_change:subj={subj};to={color}")


# ===========================================================================
# STATIC_PATTERNS (parse_static matchers, pre-catchall)
#
# Used when the ability is a standalone static/rider line the main parser
# would otherwise treat as a generic Modification. Order: most-specific
# phrases first (so "is every creature type" beats "is <subtype>").
# ===========================================================================

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# ---- Changeling-style "is every creature type" ---------------------------
@_sp(r"^(?:~|this (?:creature|permanent|card)|it|equipped creature|"
     r"enchanted creature|each (?:nonland )?creature[^.]*) is every creature type\s*$")
def _is_every_creature_type(m, raw):
    return Static(
        modification=Modification(kind="changeling_static", args=()),
        raw=raw,
    )


# ---- "it's still a land" / "that's still a <type>" -----------------------
@_sp(r"^(?:it'?s|that'?s|they'?re) still (?:a|an)\s+([a-z ]+?)\s*$")
def _still_a(m, raw):
    return Static(
        modification=Modification(kind="type_add_still",
                                  args=(m.group(1).strip(),)),
        raw=raw,
    )


# ---- "it's still a land and has its other abilities" --------------------
@_sp(r"^(?:it'?s|that'?s) still (?:a|an) [a-z ]+ and has its other abilities\s*$")
def _still_a_full(m, raw):
    return Static(
        modification=Modification(kind="type_add_still_full", args=()),
        raw=raw,
    )


# ---- "<subject> is no longer <qualifier>" --------------------------------
# Covers: "that creature is no longer goaded", "target snow land is no
# longer snow", "this creature is no longer suspected".
@_sp(r"^(?:~|it|this (?:creature|permanent|land|card)|"
     r"that (?:creature|permanent|land|card)|"
     r"target (?:snow )?(?:creature|permanent|land|card)[^.]*|"
     r"each (?:creature|permanent)[^.]*) is no longer ([a-z][a-z ]*?)\s*$")
def _is_no_longer(m, raw):
    return Static(
        modification=Modification(kind="type_remove",
                                  args=(m.group(1).strip(),)),
        raw=raw,
    )


# ---- "<subject> is also a <type-list>" -----------------------------------
# Covers "~ is also a cleric, rogue, warrior, and wizard", "~ is also a
# rebel with ...", "each +1/+1 counter on a creature you control is also a
# food token".
@_sp(r"^(.+?) is also (?:a |an )?([a-z][a-z ,'-]+?)(?: with [^.]+)?\s*$")
def _is_also_a(m, raw):
    subj = m.group(1).strip()
    type_list = m.group(2).strip()
    # Guard against over-match (e.g. "angelo is also a roommate" — harmless
    # but we only want MTG-ish subjects). Skip if subject looks like prose.
    if len(subj.split()) > 8:
        return None
    return Static(
        modification=Modification(kind="type_add",
                                  args=(subj, type_list)),
        raw=raw,
    )


# ---- "<subject> is <type> in addition to its other types" ----------------
@_sp(r"^(.+?) is (?:a |an )?([a-z][a-z ,'-]+?) in addition to (?:its|their) other (?:types|colors|types and colors)\s*$")
def _is_type_in_addition(m, raw):
    subj = m.group(1).strip()
    type_ = m.group(2).strip()
    return Static(
        modification=Modification(kind="type_add",
                                  args=(subj, type_)),
        raw=raw,
    )


# ---- "it gains <keyword>" as a post-animation tail -----------------------
# Often appears as its own sentence after a manland animation: "then it
# gains haste until end of turn." The base parser already has a pronoun
# keyword-grant rule ("it gains haste") — keep a targeted variant for
# "then it gains X until end of turn" so it's classified consistently.
@_sp(r"^then it gains ([a-z, ]+?)(?: until end of turn)?\s*$")
def _then_it_gains(m, raw):
    return Static(
        modification=Modification(kind="post_animate_grant",
                                  args=(m.group(1).strip(),)),
        raw=raw,
    )


# ---- "it can't be a creature" (anti-animate; inverse of Living Lands) ----
@_sp(r"^(?:~|it|this (?:land|permanent|creature)) can'?t be (?:a |an )?(creature|artifact|enchantment|land|planeswalker)\s*$")
def _cant_be_type(m, raw):
    return Static(
        modification=Modification(kind="anti_animate",
                                  args=(m.group(1).strip(),)),
        raw=raw,
    )


# ---- "~ is no longer a land" / "~ is no longer snow" (already covered
# above by _is_no_longer, but some phrasings omit "a/an" before the type). -


# ===========================================================================
# No TRIGGER_PATTERNS — becomes-crewed / becomes-saddled triggers belong to
# vehicles_mounts.py; we only handle the effects here.
# ===========================================================================

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = []


__all__ = ["EFFECT_RULES", "STATIC_PATTERNS", "TRIGGER_PATTERNS"]


# ===========================================================================
# Self-check — run: python3 scripts/extensions/type_changes.py
# ===========================================================================

if __name__ == "__main__":
    effect_samples = [
        # Manlands
        "target land you control becomes a 3/3 elemental creature with haste until end of turn",
        "this land becomes a 1/1 ~ artifact creature with flying until end of turn",
        "until end of turn, this land becomes a 7/7 blue giant creature with ward {3}",
        "this land becomes a 3/3 creature with vigilance and all creature types",
        # Vehicle activations
        "this vehicle becomes an artifact creature until end of turn",
        "this artifact becomes a 4/4 artifact creature until end of turn",
        # Enchantment-creatures
        "it becomes a 3/3 monk creature with haste in addition to its other types until end of turn",
        "this enchantment becomes a 3/3 monk creature with haste in addition to its other types until end of turn",
        # Pronoun-led animation
        "it becomes a treefolk creature with haste and base power and toughness equal to this creature's power",
        "it becomes a 0/0 bird creature with flying in addition to its other types",
        # Becomes a copy
        "target creature becomes a copy of that creature until end of turn",
        "each other creature becomes a copy of target nonlegendary creature until end of turn",
        "this creature becomes a copy of that card, except it has this ability",
        # Becomes a basic land
        "target land becomes an island until end of turn",
        "target land becomes a forest",
        # Subtype-only animation
        "target creature becomes a coward until end of turn",
        "it becomes a cat in addition to its other types",
        "it becomes a rogue in addition to its other types",
        # All colors / random color
        "target creature you control becomes all colors until end of turn",
        "~ becomes a random color permanently",
        # Planeswalker animation
        "until end of turn, ~ becomes a 5/5 white soldier creature that's still a planeswalker",
    ]

    static_samples = [
        "it's still a land",
        "that's still a planeswalker",
        "it's still a land and has its other abilities",
        "that creature is no longer goaded",
        "target snow land is no longer snow",
        "this creature is no longer suspected",
        "~ is also a cleric, rogue, warrior, and wizard",
        "each +1/+1 counter on a creature you control is also a food token",
        "that creature is an artifact in addition to its other types",
        "it is a cat in addition to its other types",
        "it can't be a creature",
        "~ is every creature type",
        "equipped creature is every creature type",
        "then it gains haste until end of turn",
    ]

    def _try(pats, s):
        for pat, _ in pats:
            if pat.match(s.rstrip(".")):
                return True
        return False

    unmatched = []
    for s in effect_samples:
        if not _try(EFFECT_RULES, s):
            unmatched.append(("EFFECT", s))
    for s in static_samples:
        if not _try(STATIC_PATTERNS, s):
            unmatched.append(("STATIC", s))

    total = len(effect_samples) + len(static_samples)
    ok = total - len(unmatched)
    print(f"type_changes.py: {len(EFFECT_RULES)} effects, "
          f"{len(STATIC_PATTERNS)} statics")
    print(f"  sample coverage: {ok}/{total}")
    if unmatched:
        print("\n  UNMATCHED:")
        for kind, s in unmatched:
            print(f"    [{kind}] {s}")
        raise SystemExit(1)
