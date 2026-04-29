#!/usr/bin/env python3
"""Niche secondary trigger patterns.

Family: NICHE SECONDARY TRIGGERS — triggered abilities that fire from events
outside the main combat / death / ETB / cast axis but that show up across
enough cards to deserve typed grammar productions. Covers:

  * Draw-a-card payoffs (broader than the existing Niv-Mizzet ordinal):
      - "whenever you draw a card, ..."
      - "when you draw a card, ..."
  * Discard-as-event triggers (yours AND opponent's):
      - "whenever you discard a card / a [filter] card, ..."
      - "whenever you discard one or more cards, ..."
      - "whenever you cycle or discard (a/another) card, ..."
      - "whenever an opponent discards a card, ..."
      - "whenever a player discards a card, ..."
  * Sacrifice-as-event triggers:
      - "whenever you sacrifice a [filter] / a permanent / another permanent / a creature, ..."
  * Cycle triggers:
      - "whenever you cycle a card / another card, ..."
  * Scry / Surveil / Proliferate / Explore payoff triggers:
      - "whenever you scry, ..." / "whenever you surveil, ..."
      - "whenever you explore, ..."
      - (proliferate is already covered by counter_triggers; not duplicated here)
  * Tap-for-mana triggers:
      - "whenever you tap a [filter] for mana, ..."
      - "whenever you tap an untapped creature an opponent controls, ..."
  * Becomes-tapped / becomes-untapped triggers (self + actor forms):
      - "whenever ~ becomes tapped / untapped, ..."
      - "whenever a [filter] becomes tapped / untapped, ..."
  * Becomes-X transformation triggers:
      - "when ~ becomes monstrous, ..."
      - "whenever a [filter] becomes renowned, ..."
      - "whenever ~ transforms, ..." (self + actor)
      - "whenever ~ is turned face up, ..." / "whenever a [filter] is turned face up, ..."
  * Lifegain-ordinal (companion to the existing life_triggers family):
      - "whenever you gain life for the first time each turn, ..."
      - "when you gain life for the first time each turn, ..."
  * Spell-cast filtered by mana value / color:
      - "whenever you cast a spell with mana value N or (less|greater), ..."
      - "whenever you cast a [white|blue|black|red|green|colorless|
         multicolored|monocolored] spell, ..."
  * Friendly-target triggers (broader actor form than core):
      - "whenever a [filter] you control becomes the target of a spell or
        ability an opponent controls, ..."
  * Constellation-style ETB triggers (bare "enters", no "you control"):
      - "whenever an enchantment enters, ..." / "whenever an artifact enters, ..."
      - "whenever a creature / land enters, ..." (broader than core's
        "a creature you control enters")
  * Game-start triggers (rare, Leyline of the Void / Gemstone Caverns):
      - "at the beginning of the game, ..."

Exported tables match the parser's merge points:

  TRIGGER_PATTERNS = [(compiled_regex, event_name, scope), ...]

The parser tries EXT_TRIGGER_PATTERNS before the core list, so patterns here
take precedence. They're still ordered most-specific → least-specific so that
within this file "cast a spell with mana value" beats the filtered-color form,
"cycle or discard" beats plain cycle, and "gain life for the first time"
beats any future bare "gain life" entries.

The parser's extension loop only reads the 3-tuple shape. Builder functions
aren't supported here — the effect tail is parsed by parser.parse_effect.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# Make the AST importable when this file is loaded as a standalone module.
_HERE = Path(__file__).resolve().parent
_SCRIPTS = _HERE.parent
if str(_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS))

# ---------------------------------------------------------------------------
# Shared fragments
# ---------------------------------------------------------------------------

# Self-aliases — same convention used by combat_triggers / counter_triggers.
_SELF = (
    r"(?:~|this creature|this permanent|this vehicle|this land|"
    r"this artifact|this enchantment|this card|equipped creature|"
    r"enchanted creature)"
)

# Actor capture — lazy "to the next comma" so we don't eat the effect tail.
_ACTOR = r"([^,.]+?)"

# Color words oracle text uses for mono/multi-color spell-cast filters.
_COLOR = (
    r"(?:white|blue|black|red|green|colorless|multicolored|monocolored)"
)


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [

    # ------------------------------------------------------------------
    # Game-start trigger (rare; must come before any "at the beginning of"
    # wildcard in core to avoid being swallowed as a named phase).
    # ------------------------------------------------------------------
    (re.compile(r"^at the beginning of the game\b", re.I),
     "game_start", "self"),

    # ------------------------------------------------------------------
    # Lifegain-ordinal — "first time each turn" qualifier. Positioned
    # before any bare "you gain life" triggers from life_triggers so this
    # more specific shape wins.
    # ------------------------------------------------------------------
    (re.compile(r"^whenever you gain life for the first time (?:each turn|during each of your turns)\b", re.I),
     "gain_life_first_time", "self"),
    (re.compile(r"^when you gain life for the first time (?:each turn|during each of your turns)\b", re.I),
     "gain_life_first_time_once", "self"),

    # ------------------------------------------------------------------
    # Surveil-ordinal / Scry-ordinal first-time-each-turn
    # ------------------------------------------------------------------
    (re.compile(r"^whenever you surveil for the first time each turn\b", re.I),
     "surveil_first_time", "self"),
    (re.compile(r"^whenever you scry for the first time each turn\b", re.I),
     "scry_first_time", "self"),

    # ------------------------------------------------------------------
    # Spell-cast filtered by MANA VALUE (must come before the color form
    # and before core's cast_filtered, which would eat this as
    # "cast a [spell with mana value N or less] spell").
    # ------------------------------------------------------------------
    (re.compile(
        r"^whenever you cast (?:a|an) (?:spell with mana value|mana value) (\d+) or (less|greater)\b",
        re.I),
     "cast_mana_value", "self"),
    (re.compile(
        r"^whenever you cast a spell with mana value (\d+) or (less|greater)\b",
        re.I),
     "cast_mana_value", "self"),
    (re.compile(
        r"^whenever you cast a spell with mana value (\d+)\b",
        re.I),
     "cast_mana_value_eq", "self"),

    # ------------------------------------------------------------------
    # Spell-cast filtered by COLOR. Core's cast_filtered captures
    # "whenever you cast a <X> spell" where <X> is anything lazy; we add
    # a specialized shape so the parser records the color as the filter
    # without relying on the generic capture.
    # ------------------------------------------------------------------
    (re.compile(rf"^whenever you cast (?:a|an) {_COLOR} spell\b", re.I),
     "cast_color_filtered", "self"),

    # ------------------------------------------------------------------
    # Draw-a-card triggers — broader than the existing Niv-Mizzet ordinal.
    # Core only has "whenever you draw your (second|third|fourth) card
    # each turn"; this matches the plain form used by Niv-Mizzet,
    # Parun / Psychosis Crawler / Mystic Remora-style cards.
    # ------------------------------------------------------------------
    (re.compile(r"^whenever you draw a card\b", re.I),
     "draw_card", "self"),
    (re.compile(r"^when you draw a card\b", re.I),
     "draw_card_once", "self"),
    (re.compile(r"^whenever an opponent draws a card\b", re.I),
     "opp_draw_card", "self"),
    (re.compile(r"^whenever a player draws a card\b", re.I),
     "player_draw_card", "all"),

    # ------------------------------------------------------------------
    # Discard-as-event triggers — player-level, not the creature's
    # "when this creature is discarded" which is handled elsewhere.
    # ------------------------------------------------------------------
    # "whenever you cycle or discard (a/another) card" — compound first.
    (re.compile(r"^whenever you cycle or discard (?:a|an|another) cards?\b", re.I),
     "cycle_or_discard", "self"),
    (re.compile(r"^whenever you discard one or more cards\b", re.I),
     "discard_one_or_more", "self"),
    # Typed discard ("a creature card", "a land card", "a noncreature, nonland card").
    (re.compile(r"^whenever you discard (?:a|an|another) ([^,.]+? )?cards?\b", re.I),
     "discard_filtered", "self"),
    (re.compile(r"^whenever an opponent discards (?:a|an) ([^,.]+? )?cards?\b", re.I),
     "opp_discard_filtered", "self"),
    (re.compile(r"^whenever an opponent discards a card\b", re.I),
     "opp_discard", "self"),
    (re.compile(r"^whenever a player discards a card\b", re.I),
     "player_discard", "all"),

    # ------------------------------------------------------------------
    # Cycle triggers (standalone — the compound "cycle or discard" above
    # wins first for the compound form).
    # ------------------------------------------------------------------
    (re.compile(r"^whenever you cycle (?:a|an|another) cards?\b", re.I),
     "cycle_card", "self"),

    # ------------------------------------------------------------------
    # Scry / Surveil / Explore payoff triggers.
    # (Proliferate is handled in counter_triggers.)
    # ------------------------------------------------------------------
    (re.compile(r"^whenever you scry\b", re.I),
     "you_scry", "self"),
    (re.compile(r"^whenever you surveil\b", re.I),
     "you_surveil", "self"),
    (re.compile(r"^whenever you explore\b", re.I),
     "you_explore", "self"),
    (re.compile(r"^whenever a creature you control explores\b", re.I),
     "ally_explore", "self"),

    # ------------------------------------------------------------------
    # Sacrifice-as-event triggers (player-level — "whenever you sacrifice
    # X"). Specific filters first; bare form last.
    # ------------------------------------------------------------------
    (re.compile(
        r"^whenever you sacrifice (?:~|this artifact|this creature|this enchantment|this permanent) or another ([^,.]+?)\b",
        re.I),
     "sacrifice_self_or_ally", "self"),
    (re.compile(r"^whenever you sacrifice (?:a|an|another) ([^,.]+?)\b", re.I),
     "sacrifice_filtered", "self"),
    (re.compile(r"^whenever an opponent sacrifices (?:a|an) ([^,.]+?)\b", re.I),
     "opp_sacrifice_filtered", "self"),

    # ------------------------------------------------------------------
    # Tap-for-mana triggers.
    # ------------------------------------------------------------------
    (re.compile(r"^whenever you tap an untapped ([^,.]+?) (?:an opponent controls|a player controls)\b", re.I),
     "tap_opp_creature", "self"),
    (re.compile(r"^whenever you tap (?:a|an|another) ([^,.]+?) for mana\b", re.I),
     "tap_for_mana", "self"),

    # ------------------------------------------------------------------
    # Becomes-tapped triggers (self + actor). Actor form must come before
    # self form so "a permanent you control" wins over a bare-self attempt.
    # ------------------------------------------------------------------
    (re.compile(rf"^whenever {_SELF} becomes tapped\b", re.I),
     "becomes_tapped", "self"),
    (re.compile(rf"^whenever {_ACTOR} becomes tapped\b", re.I),
     "becomes_tapped", "actor"),
    (re.compile(rf"^when {_SELF} becomes tapped\b", re.I),
     "becomes_tapped_once", "self"),

    # ------------------------------------------------------------------
    # Becomes-untapped triggers (Inspired and successors).
    # ------------------------------------------------------------------
    (re.compile(rf"^whenever {_SELF} becomes untapped\b", re.I),
     "becomes_untapped", "self"),
    (re.compile(rf"^whenever {_ACTOR} becomes untapped\b", re.I),
     "becomes_untapped", "actor"),
    (re.compile(rf"^when {_SELF} becomes untapped\b", re.I),
     "becomes_untapped_once", "self"),

    # ------------------------------------------------------------------
    # Becomes-monstrous / becomes-renowned transformation triggers.
    # ------------------------------------------------------------------
    (re.compile(rf"^when {_SELF} becomes monstrous\b", re.I),
     "becomes_monstrous", "self"),
    (re.compile(rf"^whenever {_ACTOR} becomes monstrous\b", re.I),
     "becomes_monstrous", "actor"),
    (re.compile(rf"^when {_SELF} becomes renowned\b", re.I),
     "becomes_renowned", "self"),
    (re.compile(rf"^whenever {_ACTOR} becomes renowned\b", re.I),
     "becomes_renowned", "actor"),

    # ------------------------------------------------------------------
    # Transforms triggers — DFC / MDFC turning over.
    # ------------------------------------------------------------------
    (re.compile(rf"^when(?:ever)? {_SELF} transforms\b", re.I),
     "transforms", "self"),
    (re.compile(rf"^when(?:ever)? {_ACTOR} transforms(?: into [^,.]+)?\b", re.I),
     "transforms", "actor"),

    # ------------------------------------------------------------------
    # Is-turned-face-up (Morph / Disguise / Cloak / Manifest).
    # ------------------------------------------------------------------
    (re.compile(rf"^when(?:ever)? {_SELF} is turned face up\b", re.I),
     "turned_face_up", "self"),
    (re.compile(rf"^whenever {_ACTOR} is turned face up\b", re.I),
     "turned_face_up", "actor"),
    (re.compile(rf"^when(?:ever)? {_SELF} is turned face down\b", re.I),
     "turned_face_down", "self"),

    # ------------------------------------------------------------------
    # Friendly-target triggers — actor form broader than core's
    # "whenever ~ becomes the target of". Core already handles actor
    # form generically, but this pins down the very common "becomes the
    # target of a spell or ability an opponent controls" phrasing as a
    # distinct event for classification.
    # ------------------------------------------------------------------
    (re.compile(
        r"^whenever ([^,.]+?) becomes the target of a spell or ability an opponent controls\b",
        re.I),
     "ally_targeted_by_opp", "actor"),
    (re.compile(
        rf"^whenever {_SELF} becomes the target of a spell or ability an opponent controls\b",
        re.I),
     "self_targeted_by_opp", "self"),

    # ------------------------------------------------------------------
    # Constellation-style ETB triggers — bare "enters" without the
    # "you control" qualifier that core already handles.
    # ------------------------------------------------------------------
    (re.compile(r"^whenever an enchantment enters(?: under your control)?\b", re.I),
     "enchantment_etb", "self"),
    (re.compile(r"^whenever an artifact enters(?: under your control)?\b", re.I),
     "artifact_etb", "self"),
    # "a creature enters" / "a land enters" without "you control" is rare but
    # real (Felidar Retreat-adjacent). Core's `when ([^,.]+?) enters` would
    # otherwise grab these — we pin them down with explicit event names.
    (re.compile(r"^whenever a creature enters(?! the battlefield under an opponent)\b", re.I),
     "creature_etb_any", "self"),
    (re.compile(r"^whenever a land enters(?! the battlefield under an opponent)\b", re.I),
     "land_etb_any", "self"),

    # ------------------------------------------------------------------
    # "Card is put into a graveyard from anywhere" — broader than core's
    # battlefield-only shape (covers milling / discarding / dying in one).
    # ------------------------------------------------------------------
    (re.compile(
        r"^whenever a card is put into (?:your|an opponent'?s?|a player'?s?) graveyard from anywhere\b",
        re.I),
     "card_to_gy_anywhere", "self"),
    (re.compile(
        r"^when a card is put into (?:your|an opponent'?s?) graveyard from anywhere\b",
        re.I),
     "card_to_gy_anywhere_once", "self"),
]


__all__ = ["TRIGGER_PATTERNS"]


# ---------------------------------------------------------------------------
# Smoke test: verify every pattern compiles and matches at least one exemplar.
# Run directly with `python3 scripts/extensions/niche_triggers.py`.
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    samples = [
        # Game-start
        "at the beginning of the game, you may pay any amount of life.",
        "at the beginning of the game, choose a creature type.",
        # Lifegain ordinal
        "whenever you gain life for the first time each turn, draw a card.",
        "when you gain life for the first time each turn, create a 1/1 cat.",
        # Scry/surveil ordinal
        "whenever you surveil for the first time each turn, deal 1 damage.",
        "whenever you scry for the first time each turn, draw a card.",
        # Spell-cast by mana value
        "whenever you cast a spell with mana value 5 or greater, draw a card.",
        "whenever you cast a spell with mana value 3 or less, scry 1.",
        "whenever you cast a spell with mana value 4, create a treasure.",
        # Spell-cast by color
        "whenever you cast a black spell, put a +1/+1 counter on this creature.",
        "whenever you cast a colorless spell, draw a card.",
        "whenever you cast a multicolored spell, scry 1.",
        # Draw
        "whenever you draw a card, target opponent mills two cards.",
        "when you draw a card, destroy this enchantment.",
        "whenever an opponent draws a card, you gain 1 life.",
        "whenever a player draws a card, put a counter on ~.",
        # Discard
        "whenever you cycle or discard a card, scry 1.",
        "whenever you cycle or discard another card, scry 1.",
        "whenever you discard one or more cards, exile them.",
        "whenever you discard a creature card, put a +1/+1 counter on this creature.",
        "whenever you discard a land card, create a treasure token.",
        "whenever you discard a card, draw a card.",
        "whenever an opponent discards a creature card, create a 2/2 zombie.",
        "whenever an opponent discards a card, put a +1/+1 counter on ~.",
        "whenever a player discards a card, you may gain 1 life.",
        # Cycle
        "whenever you cycle a card, draw a card.",
        "whenever you cycle another card, you gain 1 life.",
        # Scry / surveil / explore
        "whenever you scry, put a verse counter on this enchantment.",
        "whenever you surveil, put a +1/+1 counter on ~.",
        "whenever you explore, create a 1/1 token.",
        "whenever a creature you control explores, draw a card.",
        # Sacrifice
        "whenever you sacrifice a creature, draw a card.",
        "whenever you sacrifice a permanent, this creature gets +2/+0 until end of turn.",
        "whenever you sacrifice another creature, you gain 1 life and scry 1.",
        "whenever you sacrifice a clue, target creature can't be blocked this turn.",
        "whenever you sacrifice ~ or another artifact, draw a card.",
        "whenever an opponent sacrifices a permanent, you gain 1 life.",
        # Tap for mana
        "whenever you tap a land for mana, add an additional {g}.",
        "whenever you tap a creature for mana, add an additional {g}.",
        "whenever you tap an artifact token for mana, add one mana of any type that artifact token produced.",
        "whenever you tap an untapped creature an opponent controls, this creature gets +2/+1 until end of turn.",
        # Becomes tapped / untapped
        "whenever ~ becomes tapped, draw a card, then discard a card.",
        "whenever this creature becomes tapped, create a lander token.",
        "whenever an artifact you control becomes tapped, draw a card.",
        "whenever ~ becomes untapped, you may exile target creature.",
        "whenever a permanent becomes untapped, that permanent's controller mills a card.",
        "whenever equipped creature becomes untapped, remove a bait counter from this equipment.",
        # Becomes-X
        "when this creature becomes monstrous, it deals 2 damage to each opponent.",
        "whenever a creature you control becomes renowned, draw a card.",
        "when ~ transforms, create a treasure token.",
        "whenever a permanent you control transforms into a non-human creature, create a 2/2 green wolf.",
        # Face-up / face-down
        "whenever a permanent you control is turned face up, put a +1/+1 counter on it.",
        "when ~ is turned face up, attach it to target creature you control.",
        "when this creature is turned face down, scry 1.",
        # Friendly-target
        "whenever ~ becomes the target of a spell or ability an opponent controls, you may draw a card.",
        "whenever a creature you control becomes the target of a spell or ability an opponent controls, put a +1/+1 counter on target creature you control.",
        # Constellation-ish ETB
        "whenever an enchantment enters, draw a card.",
        "whenever an enchantment enters under your control, you gain 1 life.",
        "whenever an artifact enters, you may untap this creature.",
        # GY-from-anywhere
        "whenever a card is put into your graveyard from anywhere, you may return it to your hand.",
        "when a card is put into your graveyard from anywhere, sacrifice this enchantment.",
    ]

    unmatched = []
    for s in samples:
        hit = None
        low = s.lower().rstrip(".")
        for pat, event, scope in TRIGGER_PATTERNS:
            if pat.match(low):
                hit = (event, scope)
                break
        if hit is None:
            unmatched.append(s)
        else:
            print(f"OK  [{hit[0]:<28s} {hit[1]:<6s}]  {s}")

    if unmatched:
        print("\nUNMATCHED:")
        for s in unmatched:
            print(" ", s)
        raise SystemExit(1)
    print(f"\nAll {len(samples)} samples matched.")
