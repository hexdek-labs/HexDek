#!/usr/bin/env python3
"""r140 regression — a timing restriction is bound to the ability it limits.

Inline Scryfall-shaped fixtures, no corpus. Exits non-zero on failure.

What this pins, and why each case is here:
  * A trailing "this ability triggers only once each turn" must land on the
    Triggered ability in its own paragraph, and the detached Static must be
    gone. That is the whole point of the change (Kambal's shape).
  * The same for "Activate only as a sorcery." on an Activated ability, where
    the restriction is normalised to the existing "sorcery" vocabulary.
  * A continuation sentence between the ability and its restriction must not
    block the bind -- 160 of the 831 live nodes have exactly that shape.
  * The restriction must NOT cross a paragraph boundary. This is the case the
    binding could silently get wrong, and the one that would be invisible in
    an aggregate count.
  * An unbindable restriction must be LEFT ALONE, not attached to something
    that is not the ability it limits.

Run: python3 -I scripts/test_timing_restriction_binding_r140.py
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from mtg_ast import Activated, Static, Triggered  # noqa: E402
import parser as P  # noqa: E402
from parser import parse_card  # noqa: E402

# The extensions are what production runs with (export_ast_dataset.py calls
# this). Without them the core's own `trigger_restriction` fallback at
# parser.py:2510 shadows the promoter's `timing_restriction`, and the test
# would be exercising a code path the real corpus never takes: the live
# dataset holds 120 `timing_restriction` once-per-turn nodes and zero
# `trigger_restriction` ones.
P.load_extensions()

FAILURES = []


def check(label, cond, detail=""):
    if cond:
        print(f"  PASS  {label}")
    else:
        print(f"  FAIL  {label}  {detail}")
        FAILURES.append(label)


def card(name, text, type_line="Legendary Creature - Human"):
    return {"name": name, "oracle_text": text, "type_line": type_line,
            "mana_cost": "{2}{W}", "layout": "normal"}


def kinds(ast):
    return [type(a).__name__ for a in ast.abilities]


def detached(ast):
    return [a for a in ast.abilities
            if isinstance(a, Static)
            and getattr(a.modification, "kind", None) == "timing_restriction"]


# ---------------------------------------------------------------------------
print("1. triggered ability + 'triggers only once each turn' (the Kambal shape)")
ast = parse_card(card(
    "R140 Trigger Limiter",
    "Whenever this creature attacks, create a Treasure token. "
    "This ability triggers only once each turn."))
trig = [a for a in ast.abilities if isinstance(a, Triggered)]
check("a Triggered ability survives", len(trig) == 1, kinds(ast))
check("the restriction is bound to it",
      bool(trig) and trig[0].timing_restriction == "once_per_turn",
      trig[0].timing_restriction if trig else "no Triggered")
check("the detached Static is gone", not detached(ast), kinds(ast))

# ---------------------------------------------------------------------------
print("2. activated ability + 'Activate only as a sorcery.'")
ast = parse_card(card(
    "R140 Sorcery Timing",
    "{2}{W}: Draw a card. Activate only as a sorcery."))
act = [a for a in ast.abilities if isinstance(a, Activated)]
check("an Activated ability survives", len(act) == 1, kinds(ast))
check("restriction normalised to 'sorcery'",
      bool(act) and act[0].timing_restriction == "sorcery",
      act[0].timing_restriction if act else "no Activated")
check("the detached Static is gone", not detached(ast), kinds(ast))

# ---------------------------------------------------------------------------
print("3. a continuation sentence between ability and restriction")
ast = parse_card(card(
    "R140 Intervening Sentence",
    "{T}: Target land becomes a 3/3 Elemental creature with haste until end "
    "of turn. It's still a land. Activate only as a sorcery."))
act = [a for a in ast.abilities if isinstance(a, Activated)]
check("the restriction reaches past the continuation",
      bool(act) and act[0].timing_restriction == "sorcery",
      kinds(ast))
check("the detached Static is gone", not detached(ast), kinds(ast))

# ---------------------------------------------------------------------------
print("4. the restriction must NOT cross a paragraph boundary")
ast = parse_card(card(
    "R140 Two Paragraphs",
    "{T}: Add {G}.\n{2}{W}: Draw a card. Activate only as a sorcery."))
act = [a for a in ast.abilities if isinstance(a, Activated)]
restricted = [a for a in act if a.timing_restriction]
check("exactly one ability is restricted", len(restricted) == 1,
      [(a.raw[:34], a.timing_restriction) for a in act])
check("it is the one in the restriction's own paragraph",
      len(restricted) == 1 and "draw a card" in restricted[0].raw.lower(),
      restricted[0].raw[:60] if restricted else "none")

# ---------------------------------------------------------------------------
print("5. an unbindable restriction is left alone, not guessed at")
ast = parse_card(card(
    "R140 No Owner",
    "Flying\nActivate only as a sorcery."))
check("no Activated/Triggered was invented",
      not [a for a in ast.abilities if isinstance(a, (Activated, Triggered))],
      kinds(ast))
check("the Static is still there rather than silently dropped",
      len(detached(ast)) == 1, kinds(ast))

# ---------------------------------------------------------------------------
print("6. an ability with no restriction keeps a null field")
ast = parse_card(card(
    "R140 Unrestricted",
    "Whenever this creature attacks, draw a card."))
trig = [a for a in ast.abilities if isinstance(a, Triggered)]
check("timing_restriction stays None",
      bool(trig) and trig[0].timing_restriction is None,
      trig[0].timing_restriction if trig else "no Triggered")

# ---------------------------------------------------------------------------
print("7. the paragraph guard: an orphan restriction must not reach backwards")
# This is the case the group check exists for, and the only one that
# discriminates it -- in case 4 the nearest preceding ability happens to be
# the right one anyway, so that test passes with the guard disabled.
# Here the restriction's own paragraph parses to a Static with no ability
# node, exactly like the 4 live corpus cards (G'raha Tia, Leather Armor,
# Curator of Sun's Creation, The Sixth Doctor) whose governing ability does
# not parse. Without the guard the restriction walks back into the previous
# paragraph and makes a MANA ABILITY activatable only once -- a silent,
# card-breaking misbind that no aggregate count would show.
ast = parse_card(card(
    "R140 Orphan Restriction",
    "{T}: Add {G}.\nEquipped creature gets +1/+1. Activate only once.",
    type_line="Artifact - Equipment"))
act = [a for a in ast.abilities if isinstance(a, Activated)]
check("the mana ability in the previous paragraph is untouched",
      len(act) == 1 and act[0].timing_restriction is None,
      [(a.raw[:30], a.timing_restriction) for a in act])
check("the orphan restriction is kept, not bound to the wrong ability",
      len(detached(ast)) == 1, kinds(ast))

# ---------------------------------------------------------------------------
print("8. one concept, one token -- and once-per-GAME is not once-per-turn")
# "Activate only once each turn" (Activated, 102 live nodes) and "this ability
# triggers only once each turn" (Triggered, 115) are the same cap under two
# spellings; the node type already says which ability it limits, so they must
# reach the engine as ONE token. "Activate only once" has no "each turn" --
# that is once per GAME and collapsing it would silently loosen 8 cards.
ast = parse_card(card(
    "R140 Activate Once Each Turn",
    "{T}: Draw a card. Activate only once each turn."))
act = [a for a in ast.abilities if isinstance(a, Activated)]
check("'activate only once each turn' canonicalises to once_per_turn",
      bool(act) and act[0].timing_restriction == "once_per_turn",
      act[0].timing_restriction if act else kinds(ast))

ast = parse_card(card(
    "R140 Activate Once Per Game",
    "{T}: Draw a card. Activate only once."))
act = [a for a in ast.abilities if isinstance(a, Activated)]
check("'activate only once' is NOT collapsed into once_per_turn",
      bool(act) and act[0].timing_restriction == "activate only once",
      act[0].timing_restriction if act else kinds(ast))

print()
if FAILURES:
    print(f"FAILED ({len(FAILURES)}): {FAILURES}")
    sys.exit(1)
print("all r140 binding checks passed")
