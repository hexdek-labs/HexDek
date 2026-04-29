#!/usr/bin/env python3
"""PARTIAL-bucket scrubber (seventh pass).

Family: PARTIAL -> GREEN promotions. Companion to ``partial_scrubber.py``
through ``partial_scrubber_6.py``; targets single-ability clusters still
unhandled after six prior passes. Patterns were picked by re-bucketing
PARTIAL parse_errors after scrubber #6 shipped (2,940 PARTIAL cards,
9.29% PARTIAL, 28,329 GREEN) and keeping the densest remaining clusters
that map cleanly onto static regex and aren't already subsumed by any
prior scrubber.

Highlights of the remaining long tail (top clusters, 2-6 hits each):

Keyword-shape clusters the base ``KEYWORD_RE`` never listed:
- ``mutate {cost}`` (~40 cards across W/U/B/R/G color variants) — Ikoria
  mutate keyword. The base regex has no ``mutate`` entry, so every
  single-cost mutate card was leaking.
- ``awaken N-{cost}`` (~14 cards) — BFZ awaken keyword. Normalize
  collapses em-dash.
- ``firebending N`` / ``firebending x, where x is ...`` (~10) — Avatar
  TLA set keyword, bare with integer or X-plus-rider.
- ``offering`` with type prefix (``rat offering``, ``goblin offering``,
  ``artifact offering``, ``snake offering``, ``moonfolk offering``,
  ``fox offering`` — 6 Patron-of-* cards).
- ``vanishing`` bare (2) — Time Spiral keyword that arrives arg-less
  on some cards (KEYWORD_RE lists ``vanishing \\d+`` but not bare).
- ``echo-discard a card`` (2) — Old echo variant with normalized
  em-dash.
- ``artifact landcycling {cost}`` (1) — tribal-cycling missing from
  prefix list.
- ``solved - <rider>`` (~4) — Case-file solved ability-word rider,
  prior scrubbers missed.
- ``transform ~`` / ``transform this artifact`` (~3).

Static / rule clusters:
- ``spells you control can't be countered`` (2) — bare uncounterable
  static (scrubber #5 required ``[type] spells``).
- ``you may play lands and cast spells from <zone>`` (2) — Sen
  Triplets / Brilliant Ultimatum control-opp-play rider.
- ``each opponent's maximum hand size is reduced by N`` (1) — Jin
  static, parallel to scrubber #5's YOU variant.
- ``~ gets +X/+0, where x is <description>`` (2) — Carrion Grub /
  Glint Raker variable self-buff.
- ``you take the initiative`` bare (2).
- ``players can't draw cards`` / ``players can't draw cards or gain
  life`` (3) — Maralen / Omen Machine static.
- ``enchanted permanent can't attack or block, and its ...`` (2) —
  Paralyze-family multi-clause pacifism.
- ``that permanent doesn't untap during its controller's untap step``
  (2) — Frost Titan cousin.
- ``that permanent loses indestructible until end of turn`` (2) —
  Mortal Wound tail.
- ``creatures it was blocking that had become blocked ...`` (2) —
  Maze of Ith tail.
- ``each creature you control can't be blocked by ...`` (2) — Filtered
  evasion anthem.
- ``creatures you control can attack as though they ...`` (2) — Maze
  Abomination / Mobilization tail.
- ``this creature gains landwalk of the chosen type`` (2) — Boreal
  Centaur-style.
- ``creature cards in your hand perpetually get +1/+1`` (2) — Alchemy
  perpetual rider.

Effect-rule clusters:
- ``return up to N target creatures to their owner's hand`` (3) —
  Multi-bounce Rhystic Deluge cousin.
- ``return up to N target creature cards with <mv-filter> from your
  graveyard to the battlefield`` (~5) — Proclamation of Rebirth /
  Scout for Survivors multi-reanimate.
- ``return x target cards from your graveyard to your hand`` (2) —
  Wildest Dreams / Nostalgic Dreams X-recur.
- ``choose any number of target creatures`` (2) — Solidarity of
  Heroes bare chooser.
- ``choose a creature card in your hand`` (3) — Sap Vitality.
- ``those creatures fight each other`` (2) — Arena / Magus of the
  Arena.
- ``sacrifice a land`` / ``sacrifice a creature`` bare (3+3) — orphan
  edict lines.
- ``each player may discard their hand and draw N cards`` (2) —
  Raphael's Technique / Wheel cousin.
- ``exile cards from the top of your library equal to ...`` (3) —
  Archaic's Agony family.
- ``you may cast that card without paying its mana cost. then ...``
  (6) — Chaos Wand / Dazzling Sphinx tails.
- ``you may cast it without paying its mana cost ...`` (2) — Galvanoth
  impulse recast tail.
- ``you may cast this creature from your graveyard [by paying X ...]``
  (2) — Scourge of Nel Toth / Risen Executioner alt-cost.
- ``target player reveals their hand and discards all ...`` (3) —
  Mind Extraction / Cabal Therapy tail.
- ``target player reveals their hand, then you choose ...`` (2) —
  Vendilion Clique / Chamber Sentry tail.
- ``when you discard a nonland card this way, <effect>`` (3) — cycling
  trigger rider (Glacial Dragonhunt / Fiery Encore).
- ``when you cast it`` bare trigger (3) — Tzaangor Shaman foretell
  cousin.
- ``up to N target creatures you control each <verb> ...`` (1) —
  Allies at Last crowd-effect.

Trigger-pattern clusters:
- ``whenever one or more artifact and/or creature cards ...`` (2) —
  Teshar / Lurrus zone event.

Keyword-ish ``~walk`` (2) — Desert Nomads / Mountain Yeti: normalize
tilded ``desertwalk`` / ``mountainwalk`` because the card's own name
starts with ``Desert``/``Mountain`` and we replace it with ``~``.

Ordering: specific-first within each table; lists are spliced into
the base parser's pattern lists in ``parser.load_extensions``.
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
    Bounce, Choice, Discard, Draw, Filter, Keyword, Modification,
    Recurse, Sacrifice, Static, UnknownEffect,
    TARGET_CREATURE, TARGET_PLAYER, TARGET_OPPONENT,
)


# ---------------------------------------------------------------------------
# STATIC_PATTERNS
# ---------------------------------------------------------------------------

STATIC_PATTERNS: list[tuple[re.Pattern, callable]] = []


def _sp(pattern: str):
    def deco(fn):
        STATIC_PATTERNS.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- Mutate {cost} ---------------------------------------------------------
# Cluster (~40). Ikoria mutate is missing from KEYWORD_RE entirely.
@_sp(r"^mutate (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _mutate(m, raw):
    return Keyword(name="mutate", args=(m.group(1),), raw=raw)


# --- Awaken N-{cost} -------------------------------------------------------
# Cluster (~14). BFZ awaken, em-dash normalized to ASCII dash.
@_sp(r"^awaken (\d+)[-–—](\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _awaken_cost(m, raw):
    return Keyword(name="awaken", args=(m.group(1), m.group(2)), raw=raw)


# --- Firebending N / Firebending X, where X is ... -------------------------
# Cluster (~10). Avatar TLA set keyword. Integer variant + X-with-rider.
@_sp(r"^firebending (\d+)\s*$")
def _firebending_n(m, raw):
    return Keyword(name="firebending", args=(int(m.group(1)),), raw=raw)


@_sp(r"^firebending x,?\s+where x is [^.]+?\s*$")
def _firebending_x(m, raw):
    return Keyword(name="firebending", args=("x",), raw=raw)


# --- Offering (tribal / type prefix) ---------------------------------------
# Cluster (6). Patron-of-* cards: "rat offering", "goblin offering", etc.
# Also "artifact offering" (Blast-Furnace Hellkite).
@_sp(r"^([a-z]+) offering\s*$")
def _typed_offering(m, raw):
    return Keyword(name="offering", args=(m.group(1),), raw=raw)


# --- Vanishing (bare, no counter count) ------------------------------------
# Cluster (2). Tidewalker / Out of Time — the counter-count form is in
# KEYWORD_RE but bare "vanishing" (as a type-line-only keyword ability)
# wasn't listed.
@_sp(r"^vanishing\s*$")
def _vanishing_bare(m, raw):
    return Keyword(name="vanishing", raw=raw)


# --- Echo-discard a card ---------------------------------------------------
# Cluster (2). Old echo variant arrives with em-dash normalized.
@_sp(r"^echo[-–—]discard a card\s*$")
def _echo_discard(m, raw):
    return Keyword(name="echo", args=("discard_a_card",), raw=raw)


# --- Artifact landcycling {cost} -------------------------------------------
# Cluster (1). Sojourner's Companion-style artifact-typed cycling.
@_sp(r"^artifact landcycling (\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _artifact_landcycling(m, raw):
    return Keyword(name="artifact_landcycling", args=(m.group(1),), raw=raw)


# --- ~walk (self-typed landwalk after normalize) ---------------------------
# Cluster (2). Desert Nomads / Mountain Yeti: "Desertwalk" / "Mountainwalk"
# where the first-word replacement in normalize() tilded the card name.
@_sp(r"^~walk\s*$")
def _tilde_walk(m, raw):
    return Keyword(name="landwalk", args=("self_typed",), raw=raw)


# --- "solved - <rider>" ----------------------------------------------------
# Cluster (~4). Case-file ability-word rider; scrubber #5's ability-word
# list didn't include "solved".
@_sp(r"^solved\s*[-–—]\s*(.+?)\s*$")
def _solved_rider(m, raw):
    return Static(modification=Modification(
        kind="ability_word_rider",
        args=("solved", m.group(1).strip())), raw=raw)


# --- "transform ~" / "transform this artifact" -----------------------------
# Cluster (~3). Bare transform effect (arrives as a separate ability line
# on certain dual-face cards after normalize).
@_sp(r"^transform (?:~|this (?:creature|artifact|enchantment|permanent|land))\s*$")
def _transform_self(m, raw):
    return Static(modification=Modification(kind="transform_self"), raw=raw)


# --- "spells you control can't be countered" (bare) ------------------------
# Cluster (2). Scrubber #5 required a leading [type], this is the bare
# form (Chimil, the Inner Sun / Hexing Squelcher).
@_sp(r"^spells you control can'?t be countered(?: this turn)?\s*$")
def _all_spells_uncounterable(m, raw):
    return Static(modification=Modification(
        kind="your_spells_uncounterable_all"), raw=raw)


# --- "you take the initiative" (bare) --------------------------------------
# Cluster (2). "Take the initiative" as a stand-alone static/effect line.
@_sp(r"^you take the initiative\s*$")
def _take_initiative(m, raw):
    return Static(modification=Modification(kind="take_initiative"), raw=raw)


# --- "players can't draw cards [or gain life]" -----------------------------
# Cluster (3). Maralen / Omen Machine / Mornsong Aria.
@_sp(r"^players can'?t draw cards(?: or gain life)?\s*$")
def _players_cant_draw(m, raw):
    return Static(modification=Modification(
        kind="players_cant_draw_cards"), raw=raw)


# --- "each opponent's maximum hand size is reduced by N" -------------------
# Cluster (1+). Jin-Gitaxias cousin (opp-side of scrubber #5's you-form).
@_sp(r"^each opponent'?s? maximum hand size is reduced by "
     r"(one|two|three|four|five|six|seven|eight|nine|ten|\d+)\s*$")
def _opp_max_hand_reduced(m, raw):
    return Static(modification=Modification(
        kind="opp_max_hand_reduced",
        args=(m.group(1),)), raw=raw)


# --- "~ gets +X/+0, where x is <description>" ------------------------------
# Cluster (2). Carrion Grub / Glint Raker variable self-buff.
@_sp(r"^(?:this creature|~) gets \+x/\+(\d+|x),?\s+where x is [^.]+?\s*$")
def _self_plus_x_var(m, raw):
    return Static(modification=Modification(
        kind="self_plus_x_var",
        args=(m.group(1),)), raw=raw)


# --- "enchanted permanent can't attack or block, and its ..." --------------
# Cluster (2). Paralyze family multi-clause pacifism aura.
@_sp(r"^enchanted (?:creature|permanent) can'?t attack or block,?"
     r"(?: and [^.]+)?\s*$")
def _enchanted_pacifism_plus(m, raw):
    return Static(modification=Modification(
        kind="enchanted_pacifism_extended"), raw=raw)


# --- "that permanent doesn't untap during its controller's untap step" -----
# Cluster (2). Target-stun variant; parser has "that creature doesn't untap"
# but not "that permanent".
@_sp(r"^that permanent doesn'?t untap during "
     r"(?:its controller'?s?|your)[^.]*?untap step\s*$")
def _that_perm_stun(m, raw):
    return Static(modification=Modification(kind="stun_target_perm"), raw=raw)


# --- "that permanent loses indestructible until end of turn" --------------
# Cluster (2). Mortal Wound tail / Disenchant rider.
@_sp(r"^that permanent loses "
     r"(indestructible|flying|vigilance|hexproof|shroud|deathtouch|[a-z ]+?) "
     r"until end of turn\s*$")
def _that_perm_loses_kw(m, raw):
    return Static(modification=Modification(
        kind="that_perm_loses_kw",
        args=(m.group(1).strip(),)), raw=raw)


# --- "creatures it was blocking that had become blocked by it this turn" ---
# Cluster (2). Maze of Ith / Guardian Beast tail fragment.
@_sp(r"^creatures it was blocking(?: that had become blocked by it)?"
     r"[^.]*?\s*$")
def _creatures_it_was_blocking(m, raw):
    return Static(modification=Modification(
        kind="creatures_it_was_blocking_tail"), raw=raw)


# --- "each creature you control can't be blocked by ..." -------------------
# Cluster (2). Filtered evasion anthem.
@_sp(r"^each creature you control can'?t be blocked by "
     r"(?:creatures? with [^.]+|[a-z ]+?)\s*$")
def _each_ally_cant_be_blocked_by(m, raw):
    return Static(modification=Modification(
        kind="each_ally_cant_be_blocked_by"), raw=raw)


# --- "creatures you control can attack as though they ..." -----------------
# Cluster (2). Maze Abomination / Mobilization tail; allows vigilance-like
# or summoning-sick bypass.
@_sp(r"^creatures you control can attack as though they "
     r"(?:didn'?t have summoning sickness|had haste|were untapped)"
     r"[^.]*?\s*$")
def _ally_can_attack_as_though(m, raw):
    return Static(modification=Modification(
        kind="ally_can_attack_as_though"), raw=raw)


# --- "this creature gains landwalk of the chosen type" --------------------
# Cluster (2). Boreal Centaur / chameleon-landwalk static.
@_sp(r"^(?:this creature|~) gains landwalk of the chosen type\s*$")
def _gains_chosen_landwalk(m, raw):
    return Static(modification=Modification(
        kind="gains_chosen_landwalk"), raw=raw)


# --- "creature cards in your hand perpetually get +N/+N" -------------------
# Cluster (2). Alchemy perpetual rider.
@_sp(r"^([a-z ]+? cards?) in your (hand|graveyard|library) "
     r"perpetually get \+(\d+)/\+(\d+)\s*$")
def _perpetual_anthem(m, raw):
    return Static(modification=Modification(
        kind="perpetual_anthem",
        args=(m.group(1).strip(), m.group(2),
              int(m.group(3)), int(m.group(4)))), raw=raw)


# --- "this creature can block creatures with shadow as though ..." ---------
# Cluster (2). Aetherflame Wall / Wall of Diffusion shadow-block rider.
@_sp(r"^(?:this creature|~) can block creatures with shadow as "
     r"though [^.]+?\s*$")
def _block_shadow_as_though(m, raw):
    return Static(modification=Modification(
        kind="block_shadow_as_though"), raw=raw)


# --- "the next creature spell you cast this turn ..." ---------------------
# Cluster (2). Commander legends-style cost-rider / buff-rider static.
@_sp(r"^the next creature spell you cast this turn "
     r"[^.]+?\s*$")
def _next_creature_spell_rider(m, raw):
    return Static(modification=Modification(
        kind="next_creature_spell_rider"), raw=raw)


# --- "other ~s you control get +N/+N" --------------------------------------
# Cluster (2). Tribal anthem where the tilde refers to self-name (parser
# didn't catch the self-named tribal form).
@_sp(r"^other ~s you control get \+(\d+)/\+(\d+)\s*$")
def _other_selfname_anthem(m, raw):
    return Static(modification=Modification(
        kind="self_named_tribal_anthem",
        args=(int(m.group(1)), int(m.group(2)))), raw=raw)


# --- "you may cast that card without paying its mana cost. then <tail>" ---
# Cluster (6, largest). Chaos Wand / Dazzling Sphinx / Maelstrom Wanderer
# compound-sentence "may cast that card without paying... then <tail>".
# Routed through STATIC_PATTERNS (parse_static runs before parse_effect) to
# dodge an earlier ``A. then B.`` EFFECT rule (compound_seq.py
# ``_then_chain``) that greedily matches and returns None on "may"-bearing
# bodies, which parse_effect does not treat as a failure-continue.
@_sp(r"^you may cast that card without paying its mana cost\.?\s+"
     r"then [^.]+?\s*$")
def _cast_that_without_paying_then(m, raw):
    return Static(modification=Modification(
        kind="may_cast_that_without_paying_then"), raw=raw)


# --- "you may play lands and cast spells from <zone>" ----------------------
# Cluster (2). Sen Triplets / Brilliant Ultimatum cast-from-alt-zone rider.
@_sp(r"^you may play lands and cast (?:[a-z, ]+? )?spells from "
     r"(?:that player'?s? hand this turn|one of those piles|the top [^.]+)"
     r"\s*$")
def _may_play_cast_from_zone(m, raw):
    return Static(modification=Modification(
        kind="may_play_cast_from_zone"), raw=raw)


# ---------------------------------------------------------------------------
# EFFECT_RULES
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "return up to N target creatures to their owner's hand" ---------------
# Cluster (3). Multi-bounce (Rhythmic Water Vortex / Jace's Ruse).
@_er(r"^return up to (one|two|three|four|five|\d+) target creatures? "
     r"to (?:their|its) owner'?s? hand(?:\.|$)")
def _return_up_to_n_creatures_hand(m):
    return Bounce(target=Filter(base="creature", quantifier="up_to_n",
                                targeted=True))


# --- "return up to N target creature cards with <mv>/total mana value ...
# from your graveyard to the battlefield" ----------------------------------
# Cluster (~5). Proclamation of Rebirth / Scout for Survivors /
# Call of the Death-Dweller / Dewdrop Cure.
@_er(r"^return up to (one|two|three|\d+) target creature cards? "
     r"(?:with [^,.]+|each with [^,.]+)? ?"
     r"from (?:your|a|any) graveyard to the battlefield(?:\.|$)")
def _return_up_to_n_creature_cards_mv_reanimate(m):
    return Recurse(query=Filter(base="creature_card", quantifier="up_to_n"),
                   destination="battlefield")


# --- "return x target cards from your graveyard to your hand" --------------
# Cluster (2). Wildest Dreams / Nostalgic Dreams.
@_er(r"^return x target cards? "
     r"(?:with [^,.]+ )?"
     r"from (?:your|a|any) graveyard to your hand(?:\.|$)")
def _return_x_cards_recurse(m):
    return Recurse(query=Filter(base="card", quantifier="up_to_n", count="x"),
                   destination="hand")


# --- "choose any number of target creatures" -------------------------------
# Cluster (2). Solidarity of Heroes / Nature's Panoply orphan header.
@_er(r"^choose any number of target "
     r"(creatures?|creature and/or planeswalker cards? in graveyards?|"
     r"creatures?,? planeswalkers?,? and/or players?)(?:\.|$)")
def _choose_any_number_targets(m):
    return UnknownEffect(raw_text=f"choose any number of target {m.group(1)}")


# --- "choose a creature card in your hand" ---------------------------------
# Cluster (3). Sap Vitality / Pass the Torch orphan line.
@_er(r"^choose a ([a-z]+) card in your hand(?:\.|$)")
def _choose_card_in_hand(m):
    return UnknownEffect(raw_text=f"choose a {m.group(1)} card in hand")


# --- "those creatures fight each other" ------------------------------------
# Cluster (2). Arena / Magus of the Arena.
@_er(r"^those creatures fight each other(?:\.|$)")
def _those_fight(m):
    return UnknownEffect(raw_text="those creatures fight each other")


# --- Bare "sacrifice a creature|land|artifact|enchantment|permanent" -------
# Cluster (3+3). Rupture / Bound // Determined / Roiling Regrowth /
# Entish Restoration. Prior scrubber #6 caught this with a multi-noun
# list but some bare forms still surface; this is a simpler strict-bare
# variant that also handles the fenced "or <type>" shape.
@_er(r"^sacrifice (?:a|an|one|two|three|\d+|\w+) "
     r"(creature|artifact|enchantment|land|permanent|token|planeswalker)"
     r"(?:\.|$)")
def _sac_bare_v7(m):
    return Sacrifice(query=Filter(base=m.group(1), targeted=False))


# --- "each player may discard their hand and draw N cards" -----------------
# Cluster (2). Raphael's Technique / Wheel-cousin (also Snort multi-line).
@_er(r"^each player may discard their hand and draw "
     r"(one|two|three|four|five|six|seven|\d+) cards?"
     r"(?:\.|$)")
def _each_player_may_wheel(m):
    return UnknownEffect(
        raw_text=f"each player may discard hand, draw {m.group(1)}")


# --- "exile cards from the top of your library equal to ..." ---------------
# Cluster (3). Archaic's Agony / Bone Mask / Snow-Covered Library.
@_er(r"^exile cards from the top of your library equal to [^.]+?(?:\.|$)")
def _exile_top_equal_to(m):
    return UnknownEffect(raw_text="exile cards from top of library equal to X")


# Note: "you may cast that card without paying... then <tail>" was moved to
# STATIC_PATTERNS above, because an earlier EFFECT rule ("A. then B.") in
# compound_seq.py greedily matches compound-sentence shapes, returns None
# when it sees "may", and parse_effect doesn't `continue` past a None
# return — meaning any EFFECT rule for this shape is unreachable.


# --- "you may cast it without paying its mana cost [tail]" -----------------
# Cluster (2). Galvanoth / Planeswalker's Mischief impulse recast.
@_er(r"^you may cast it without paying its mana cost"
     r"(?:\s+(?:if|for as long as|as long as)\s+[^.]+)?(?:\.|$)")
def _cast_it_without_paying(m):
    return UnknownEffect(raw_text="you may cast it without paying mana cost")


# --- "you may cast this creature from your graveyard [by paying ... | if ...]"
# Cluster (2). Scourge of Nel Toth / Risen Executioner alt-cost from-gy.
@_er(r"^you may cast (?:this creature|~) from your graveyard"
     r"(?:\s+(?:by paying|if you pay|for|as long as|if it'?s)\s+[^.]+)?"
     r"(?:\.|$)")
def _cast_self_from_gy(m):
    return UnknownEffect(raw_text="you may cast self from graveyard alt-cost")


# --- "target player reveals their hand and discards all <filter>" ----------
# Cluster (3). Mind Extraction / Cabal Therapy / Mesmeric Fiend-cousin.
@_er(r"^target player reveals their hand and discards all "
     r"(?:cards of [^.]+|cards with [^.]+|[a-z ]+ cards?)(?:\.|$)")
def _reveal_and_discard_all(m):
    return UnknownEffect(
        raw_text="target player reveals hand and discards all <filter>")


# --- "target player reveals their hand, then you choose ..." ---------------
# Cluster (2). Vendilion Clique-cousin umbrella.
@_er(r"^target player reveals their hand,\s+then you choose "
     r"[^.]+?(?:\.|$)")
def _reveal_then_you_choose(m):
    return UnknownEffect(
        raw_text="target player reveals hand, then you choose")


# --- "each player reveals their hand and discards ..." ---------------------
# Cluster (1+). Symmetric Blackmail.
@_er(r"^each player reveals their hand(?:\s+and (?:discards?|puts?) [^.]+)?"
     r"(?:\.|$)")
def _each_reveals_hand(m):
    return UnknownEffect(raw_text="each player reveals their hand")


# --- "when you discard a nonland card this way, <effect>" -----------------
# Cluster (3). Glacial Dragonhunt / Fiery Encore — cycling-style chained
# trigger that survives as an orphan line after split.
@_er(r"^when you discard a (?:nonland|land)? ?cards?(?: this way)?,\s+"
     r"[^.]+?(?:\.|$)")
def _when_discard_this_way(m):
    return UnknownEffect(
        raw_text="when you discard a nonland card this way, <effect>")


# --- "when you cast it, ..." / bare "when you cast it" --------------------
# Cluster (3). Tzaangor Shaman / Sea Gate Stormcaller / Dragon's Rage
# Channeler-cousin cast-it trigger orphan.
@_er(r"^when you cast it(?:,\s+[^.]+)?(?:\.|$)")
def _when_you_cast_it(m):
    return UnknownEffect(raw_text="when you cast it trigger")


# --- "up to N target creatures you control each <verb> ..." ---------------
# Cluster (~1-2). Allies at Last / Overrun cousin.
@_er(r"^up to (one|two|three|\d+) target creatures you control each "
     r"(deal|deals|gain|gains|get|gets) [^.]+?(?:\.|$)")
def _up_to_n_ally_each(m):
    return UnknownEffect(
        raw_text=f"up to {m.group(1)} target ally creatures each {m.group(2)}")


# --- "round up each time" / "rounded up each time" -------------------------
# Cluster (3). Peer into the Abyss / Fraying Omnipotence rounding rider.
@_er(r"^round(?:ed)? up each time(?:\.|$)")
def _round_up_each_time(m):
    return UnknownEffect(raw_text="round up each time")


# --- "destroy the rest" / "exile the rest" --------------------------------
# Cluster (2+2). Pre-sorted pile tail.
@_er(r"^(destroy|exile) the rest(?:\.|$)")
def _destroy_exile_rest(m):
    return UnknownEffect(raw_text=f"{m.group(1)} the rest")


# --- "untap up to three lands" ---------------------------------------------
# Cluster (2). Stone-Seeder Hierophant / Natural Spring-cousin.
@_er(r"^untap up to (one|two|three|\d+) (?:target )?lands?(?:\.|$)")
def _untap_up_to_n_lands(m):
    return UnknownEffect(raw_text=f"untap up to {m.group(1)} lands")


# --- "untap each creature you control with a +1/+1 counter on it" ---------
# Cluster (2). Nine-Lives / Soul of New Phyrexia-cousin.
@_er(r"^untap each creature you control with a "
     r"(\+1/\+1|\-1/\-1|[a-z]+) counter on it(?:\.|$)")
def _untap_each_ally_with_counter(m):
    return UnknownEffect(
        raw_text=f"untap each ally with {m.group(1)} counter")


# --- "you create a 2/2 black zombie creature token with decayed" -----------
# Cluster (3). No Way Out / Revenge of the Drowned / Chill to the Bone.
# Token body with a riding keyword that the base creation rule's regex
# doesn't anchor.
@_er(r"^(?:you )?creates? (a|an|one|two|three|\d+) "
     r"(\d+)/(\d+) ([a-z ]+?) creature tokens? "
     r"with ([a-z, ]+?)(?:\.|$)")
def _token_with_keyword(m):
    return UnknownEffect(
        raw_text=f"create {m.group(2)}/{m.group(3)} {m.group(4).strip()} "
                 f"token with {m.group(5).strip()}")


# --- "that player discards that card" (as standalone) ---------------------
# Cluster (1-2). Follow-up line from cabal-therapy-cousins.
@_er(r"^that player discards that card(?:\.|$)")
def _that_player_discards_that_card(m):
    return Discard(count=1, target=Filter(base="that_player", targeted=False),
                   chosen_by="revealer")


# --- "choose two target creatures" (bare, for fight-pair / similar) --------
# Cluster (2). Orphan chooser line without follow-up.
@_er(r"^choose (two|three|\d+) target (creatures?|players?|permanents?)"
     r"(?:\.|$)")
def _choose_n_targets(m):
    return UnknownEffect(
        raw_text=f"choose {m.group(1)} target {m.group(2)}")


# ---------------------------------------------------------------------------
# TRIGGER_PATTERNS
# ---------------------------------------------------------------------------

TRIGGER_PATTERNS: list[tuple[re.Pattern, str, str]] = [
    # --- "whenever one or more artifact and/or creature cards leave ... " ---
    # Cluster (2). Teshar / Lurrus-cousin zone event.
    (re.compile(
        r"^whenever one or more "
        r"(?:artifact|creature|enchantment|planeswalker|instant|sorcery|"
        r"noncreature|nonland)"
        r"(?:\s+and/or\s+"
        r"(?:artifact|creature|enchantment|planeswalker|instant|sorcery|"
        r"noncreature|nonland))*"
        r"\s+cards?\s+"
        r"(?:are put into|leave|enter|are exiled from|are placed into)",
        re.I),
     "compound_card_zone_event", "self"),

    # --- "when you cast a creature spell, sacrifice this creature" ---------
    # Cluster (1). Skittering Skirge self-sac-on-cast (cumulative-upkeep-
    # cousin). Base has "when you cast this spell" but not the
    # "cast a creature spell, sacrifice self" shape.
    (re.compile(
        r"^when(?:ever)? you cast an? "
        r"(creature|instant|sorcery|artifact|enchantment|planeswalker|\w+)"
        r" spell,? sacrifice (?:this creature|~|this permanent)",
        re.I),
     "self_sac_on_cast", "self"),

    # --- "when a player doesn't pay this enchantment's cumulative upkeep" --
    # Cluster (2). Mystical Tutor-cousin cumulative-upkeep default trigger.
    (re.compile(
        r"^when (?:a player|you) doesn'?t pay "
        r"(?:this enchantment'?s?|this creature'?s?|~'?s?|its) "
        r"cumulative upkeep",
        re.I),
     "cumulative_upkeep_unpaid", "self"),

    # --- "at the beginning of this turn" ----------------------------------
    # Cluster (2). Narset / Mystic Confluence at-start-of-this-turn.
    (re.compile(
        r"^at the beginning of this turn'?s? (?:[a-z]+ ?){1,3}",
        re.I),
     "this_turn_phase", "self"),

    # --- "until the beginning of your next upkeep, you ..." ---------------
    # Cluster (2). Temporary-effect prefix trigger line.
    (re.compile(
        r"^until the beginning of your next (upkeep|end step|turn)",
        re.I),
     "until_next_phase", "self"),
]
