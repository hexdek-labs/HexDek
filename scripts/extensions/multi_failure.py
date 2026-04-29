#!/usr/bin/env python3
"""Multi-failure pair scrubber.

Family: PARTIAL -> GREEN promotions for cards whose oracle text leaves
TWO OR MORE sister abilities unparsed at once. Single-failure patterns
are well covered by ``partial_scrubber*.py`` and the residual-sweep
extensions; this module targets cards where the *combined* signature is
the bottleneck — fixing one ability still leaves the card PARTIAL.

Diagnostic on the post-scrubber-9 baseline (29,652 GREEN / 1,986
PARTIAL) found only ~147 cards with 2+ parse_errors. Pairs are heavily
long-tail, but a handful of recurring families surfaced:

- ``each friend / each foe`` symmetric Pir's Whim shape
  (Pir's Whim, Virtus's Maneuver, Zndrsplt's Judgment, Regna's Sanction)
- ``{P} - <effect>`` Saviors-of-Kamigawa sweep tier list shape
  (Season of Loss / Gathering / Weaving / Burrow / Bold)
- ``the <kw> cost is equal to its mana cost`` paired with
  ``<noun> cards in <zone> have <kw>`` alt-cost grant shape
  (Herigast/Wire Surgeons/Norman Osborn/Ninja Teen/Sliver Weftwinder/
  Fblthp)
- ``firebending|waterbending|earthbending|airbending N`` Avatar set
  bender keyword family (Avatar Aang / Ran and Shaw / Hama)
- ``<bender>-<cost>`` cast-from-elsewhere alt-cost paired with grant
  (Hama, the Bloodbender)
- ``<named-ability> - equip {N}`` named-equip cost paired with the
  long quoted equipment grant (Pact Weapon / Blue Mage's Cane /
  Ninja's Blades / Summoner's Grimoire — Final Fantasy / Avatar
  named-equip family)
- ``creatures that share a color/type with <ref> get +1/+1`` paired
  bidirectional anthem (Konda's Banner / Lifecraft Engine / Jetmir
  threshold-anthem)
- ``each opponent who <verb> ... can't <verb>`` opposed-conditional
  punisher (Ward of Bones / Angelic Arbiter / Captain Eberhart)
- ``you may cast the exiled card[s] without paying ...`` post-reveal
  cast-without-paying tail paired with the reveal/exile setup
  (Talent of the Telepath / Allure of the Unknown / Hama / Bamboozle /
  Break Expectations)
- ``put the [revealed|chosen] cards into your hand[, then] shuffle``
  reveal-pick-shuffle tail (Knight-Errant of Eos / Sibylline Soothsayer /
  Turtles Forever / Stargaze / Guided Passage)
- ``each player who <verb> ... reveals the top card ...`` cascading
  punisher/reward (Vaevictis Asmadi / Master of the Wild Hunt)
- ``support x``, ``negamorph {cost}``, ``escalate-discard a card``,
  ``desertwalk``, ``planeswalkerwalk`` orphan keyword bodies that
  lack the canonical "Reminder text" templating
- ``you may planeswalk`` paired with ``jump-~`` Doctor Who keyword pair
- conditional-ETB life/token symmetric pair
  (Beza, the Bounding Spring)

Only ~30-50 of the 147 multi-failure cards fit recurring shapes; the
rest are true snowflakes (Worldpurge, Order of Succession, Phyrexian
Portal, etc.) better handled by ``per_card.py`` if at all.

Ordering: each pattern is anchored to the whole sentence (``^...$``)
and lifted into the parser's STATIC / TRIGGER / EFFECT pattern lists by
``parser.load_extensions``. Patterns are kept narrow enough to avoid
touching GREEN cards.
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
    Modification, Static, UnknownEffect,
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


# --- bender keyword bodies: "firebending N", "waterbending N", etc. -------
# Pair signature: paired with another keyword on the same line — Avatar
# Aang shows "flying, firebending 2" then a separate ETB sentence. The
# base keyword splitter doesn't recognise the bender suite, so the comma
# leaves a residual fragment.
@_sp(r"^(?:flying,\s+)?(fire|water|earth|air)bending "
     r"(one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s*$")
def _bending_keyword(m, raw):
    return Static(modification=Modification(
        kind="keyword_bending",
        args=(m.group(1), m.group(2))), raw=raw)


# --- bender alt-cost: "<bender>-{cost}" / "<bender> {cost}" --------------
# Pair signature: appears beside a "for as long as you control ~, you may
# cast the exiled card during your turn by <bender>-{X}" grant.
@_sp(r"^(fire|water|earth|air)bending[\s\-–—]*"
     r"(\{[^}]+\}(?:\{[^}]+\})*|x)\s*$")
def _bending_alt_cost(m, raw):
    return Static(modification=Modification(
        kind="bending_alt_cost",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "<named-ability> - equip {N}" — named-equip cost on equipment with
#     a long quoted grant (Pact Weapon: equip-discard a card). The named
#     prefix replaces the "equip" reminder, leaving the cost orphan.
@_sp(r"^[a-z][a-z'’ \-]+?\s*[-–—]\s*equip\s+"
     r"(\{[^}]+\}(?:\{[^}]+\})*|"
     r"discard a card|"
     r"sacrifice a [a-z]+|"
     r"remove a [a-z+/0-9-]+ counter from this equipment|"
     r"pay \d+ life|"
     r"\d+)\s*$",)
def _named_equip_cost(m, raw):
    return Static(modification=Modification(
        kind="named_equip_cost",
        args=(m.group(1),)), raw=raw)


# --- "the <alt-keyword> cost is equal to its mana cost" -------------------
# Extends partial_scrubber_9 (blitz/disturb/spectacle/prowl/surge/escape)
# with the alt-cost names that appear paired with a "cards have <kw>"
# grant on the *same card*: emerge, encore, mayhem, sneak, warp, plot,
# evoke, suspend, cleave, conjure-equivalents.
@_sp(r"^the (emerge|encore|mayhem|sneak|warp|plot|evoke|suspend|cleave|"
     r"madness|miracle|jump-start|flashback|aftermath|embalm|eternalize|"
     r"adapt|kicker|adventure|foretell|mutate|impulse) cost is equal to "
     r"[^.]+?\s*$")
def _named_alt_cost_eq_mana(m, raw):
    return Static(modification=Modification(
        kind="alt_cost_is_equal_to",
        args=(m.group(1),)), raw=raw)


# --- "<noun> cards in <zone> have <alt-kw> [{cost}]" ---------------------
# Pair signature: the alt-cost grant body that appears with the
# matching "the <kw> cost is equal to its mana cost" tail.
@_sp(r"^(?:each\s+)?(?:[a-z]+(?:,\s*[a-z]+)*\s+|nonland\s+|noncreature\s+)?"
     r"cards? (?:in|on top of) (?:your|each player'?s?) "
     r"(graveyard|hand|library|exile)\s+(?:has|have)\s+"
     r"(emerge|encore|mayhem|sneak|warp|plot|evoke|suspend|cleave|madness|"
     r"miracle|jump-start|flashback|aftermath|embalm|eternalize|adapt|"
     r"kicker|adventure|foretell|mutate|impulse)"
     r"(?:\s+\{[^}]+\}(?:\{[^}]+\})*)?\s*$")
def _zone_cards_have_kw(m, raw):
    return Static(modification=Modification(
        kind="zone_cards_have_alt_kw",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "the top card of your library has <alt-kw>" --------------------------
# Pair signature: Fblthp / top-of-library granters — paired with
# "the <kw> cost is equal to its mana cost" + "you may <verb> <noun>
# from the top of your library".
@_sp(r"^the top card of your library has\s+"
     r"(plot|adventure|disturb|adapt|foretell|cleave|miracle|"
     r"madness|mutate|impulse|escape|forecast)\s*$")
def _top_card_has_kw(m, raw):
    return Static(modification=Modification(
        kind="top_card_has_alt_kw",
        args=(m.group(1),)), raw=raw)


# --- "creatures that share a <color|type> with <ref> get +N/+N" ----------
# Pair signature: bidirectional anthem (Konda's Banner). Two near-identical
# sentences differing only by color/type axis.
@_sp(r"^creatures that share a (color|creature type|type|name) with\s+"
     r"(?:equipped creature|enchanted creature|attached creature|"
     r"the chosen creature|~)\s+gets?\s+"
     r"([+-]\d+|[+-]x)/([+-]\d+|[+-]x)\s*$")
def _share_axis_anthem(m, raw):
    return Static(modification=Modification(
        kind="share_axis_anthem",
        args=(m.group(1), m.group(2), m.group(3))), raw=raw)


# --- "creatures you control also get +N/+0 and have <kw> as long as you
#      control <N> or more creatures" -------------------------------------
# Pair signature: Jetmir threshold-anthem doubled (six/nine creatures).
@_sp(r"^creatures you control also get\s+"
     r"([+-]\d+)/([+-]\d+)\s+and have\s+[a-z, ]+?\s+as long as you "
     r"control\s+"
     r"(?:one|two|three|four|five|six|seven|eight|nine|ten|\d+)"
     r"\s+or more (?:creatures|[a-z]+)\s*$")
def _jetmir_threshold_anthem(m, raw):
    return Static(modification=Modification(
        kind="threshold_double_anthem",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "each opponent who <verb> ... can't <verb> ..." opposed conditional --
# Pair signature: Ward of Bones / Angelic Arbiter / Captain Eberhart —
# always two near-mirrored "each opponent who <X> ... can't <Y>"
# sentences side-by-side.
@_sp(r"^each opponent who\s+"
     r"(?:controls?|cast|attacked|attacks?|drew|discarded|sacrificed|"
     r"investigated|gained|lost|paid|tapped|untapped|played|revealed|"
     r"chose|doesn'?t|didn'?t)\s+[^.]+?\s+(?:can'?t|loses|gains|may)\s+"
     r"[^.]+?\s*$")
def _each_opponent_who_punisher(m, raw):
    return Static(modification=Modification(
        kind="each_opponent_who_punisher"), raw=raw)


# --- "spells cast from among cards <subj> drew this turn cost {1} <more|
#      less> to cast" -----------------------------------------------------
# Pair signature: Captain Eberhart symmetric cost-rider.
@_sp(r"^spells cast from among cards\s+"
     r"(?:you|your opponents|each opponent|each player|target opponent)\s+"
     r"drew this turn cost \{[^}]+\}(?:\{[^}]+\})*\s+"
     r"(?:more|less|fewer|greater)\s+to cast\s*$")
def _spells_from_drawn_cost_mod(m, raw):
    return Static(modification=Modification(
        kind="spells_from_drawn_cost_mod"), raw=raw)


# --- "all permanents other than this creature that weren't chosen this
#      way phase out" — Disciple of Caelus Nin ---------------------------
@_sp(r"^all permanents other than this (?:creature|permanent)\s+"
     r"that weren'?t chosen this way phase out\s*$")
def _all_perms_other_phase_out(m, raw):
    return Static(modification=Modification(
        kind="all_perms_other_phase_out"), raw=raw)


# --- "permanents can't phase in" — Disciple of Caelus Nin tail -----------
@_sp(r"^permanents can'?t phase in\s*$")
def _perms_cant_phase_in(m, raw):
    return Static(modification=Modification(
        kind="perms_cant_phase_in"), raw=raw)


# --- "creatures enchanted player controls lose all abilities and have
#      base power and toughness 1/1" — Overwhelming Splendor -----------
@_sp(r"^creatures enchanted (?:player|opponent) controls\s+"
     r"lose all abilities(?:\s+and have base power and toughness\s+"
     r"\d+/\d+)?\s*$")
def _enchanted_player_creatures_lose(m, raw):
    return Static(modification=Modification(
        kind="enchanted_player_creatures_lose_all"), raw=raw)


# --- "enchanted player can't activate abilities that aren't mana
#      abilities or loyalty abilities" — Overwhelming Splendor tail ----
@_sp(r"^enchanted (?:player|opponent) can'?t activate abilities\s+"
     r"that aren'?t (?:mana abilities|loyalty abilities)"
     r"(?:\s+or (?:mana abilities|loyalty abilities))?\s*$")
def _enchanted_player_cant_activate(m, raw):
    return Static(modification=Modification(
        kind="enchanted_player_cant_activate"), raw=raw)


# --- "support x" orphan keyword body --------------------------------------
@_sp(r"^support\s+(x|one|two|three|four|five|\d+)\s*$")
def _support_keyword(m, raw):
    return Static(modification=Modification(
        kind="keyword_support",
        args=(m.group(1),)), raw=raw)


# --- "desertwalk" / "planeswalkerwalk" / "~swalk" orphan landwalk -------
@_sp(r"^(?:desert|planeswalker|forest|island|mountain|plains|swamp|gate|"
     r"cave|sphere|legendary|snow|nonbasic|basic|~s?)walk\s*$")
def _orphan_landwalk(m, raw):
    return Static(modification=Modification(kind="keyword_landwalk"), raw=raw)


# --- "you may planeswalk" / "you may plane(s)walk to <X>" ---------------
@_sp(r"^you may planes?walk(?:\s+to [^.]+?)?\s*$")
def _you_may_planeswalk(m, raw):
    return Static(modification=Modification(kind="you_may_planeswalk"), raw=raw)


# --- "jump-~" — Doctor Who Start the TARDIS planeswalk pair ------------
@_sp(r"^jump[\-–—]~\s*$")
def _jump_self(m, raw):
    return Static(modification=Modification(kind="jump_self"), raw=raw)


# --- "escalate-discard a card" / "escalate-pay N life" / "escalate-
#      sacrifice a creature" — non-mana escalate cost forms -----------
@_sp(r"^escalate[\-–—]"
     r"(?:discard a card|pay (?:\d+|x) life|sacrifice an? [a-z]+|"
     r"remove an? [a-z+/0-9-]+ counter from [a-z ]+?|"
     r"\{[^}]+\}(?:\{[^}]+\})*)\s*$")
def _escalate_nonmana(m, raw):
    return Static(modification=Modification(
        kind="keyword_escalate_nonmana"), raw=raw)


# --- "negamorph {cost}" / "spirit of the X - equip {N}" - orphan named
#     forms used by sandbox/playtest cards (Flavor Disaster) -----------
@_sp(r"^(?:negamorph|necromorph|metamorph)\s+"
     r"\{[^}]+\}(?:\{[^}]+\})*\s*$")
def _orphan_morph_named(m, raw):
    return Static(modification=Modification(kind="keyword_morph_named"), raw=raw)


# --- "{P}/{P}{P}/{P}{P}{P} - <effect>" Saviors-of-Kamigawa sweep tier --
# Pair signature: Season of Loss / Gathering / Weaving / Burrow / Bold —
# every Season card has 3 "{P}^N - body" tier sentences. We treat each
# tier line as a static rider so the card's three orphan tiers all parse.
@_sp(r"^\{p\}(?:\{p\}){0,4}\s*[-–—]\s*[^.]+?\s*$")
def _season_proliferate_tier(m, raw):
    return Static(modification=Modification(
        kind="season_proliferate_tier"), raw=raw)


# --- "<keyword|attribute> → <value>" Vuzzle Spaceship / Catch of the Day
#     "tier list" rider — newer Un-style cards write the body as
#     "<feature> → <body>" with a unicode arrow. -----------------------
@_sp(r"^[a-z][a-z' \[\]\d]+?\s+(?:→|->|=>)\s+[^.]+?\s*$")
def _arrow_tier_rider(m, raw):
    return Static(modification=Modification(
        kind="arrow_tier_rider"), raw=raw)


# --- "<N> point each → <kw>" / "<N> points → <kw>" Built Bear point-buy -
@_sp(r"^(?:one|two|three|four|five|\d+)\s+points?\s+(?:each\s+)?"
     r"(?:→|->|=>)\s+[^.]+?\s*$")
def _point_buy_tier_rider(m, raw):
    return Static(modification=Modification(
        kind="point_buy_tier"), raw=raw)


# --- "vehicle creatures you control are the chosen creature type ..." ---
# Lifecraft Engine.
@_sp(r"^vehicle creatures you control are the chosen creature type\s+"
     r"in addition to their other types\s*$")
def _vehicle_chosen_type(m, raw):
    return Static(modification=Modification(
        kind="vehicle_chosen_type"), raw=raw)


# --- "each creature you control of the chosen type other than this
#     vehicle gets +N/+N" — Lifecraft Engine pair tail ----------------
@_sp(r"^each creature you control of the chosen type\s+"
     r"other than this (?:vehicle|creature|artifact|permanent)\s+"
     r"gets\s+([+-]\d+)/([+-]\d+)\s*$")
def _each_chosen_other_anthem(m, raw):
    return Static(modification=Modification(
        kind="each_chosen_other_anthem",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "each creature with mana value of the chosen quality has <kw>" -----
# Ashling's Prerogative.
@_sp(r"^each creature with(?:out)? mana value of the chosen quality\s+"
     r"(?:has|enters tapped|gets [+-]\d+/[+-]\d+)\s*[^.]*?\s*$")
def _each_creature_chosen_quality(m, raw):
    return Static(modification=Modification(
        kind="each_creature_chosen_quality"), raw=raw)


# --- "<color> creatures can't <verb> [creatures] you control [unless ...]"
# Heat Wave / Elephant Grass — color-pair restriction.
@_sp(r"^(?:non)?(?:white|blue|black|red|green)\s+creatures\s+"
     r"can'?t (?:attack|block)(?:\s+(?:creatures\s+)?you control)?"
     r"(?:\s+unless their controller pays [^.]+?)?\s*$")
def _color_creatures_cant_act(m, raw):
    return Static(modification=Modification(
        kind="color_creatures_cant_act"), raw=raw)


# --- "you may spend white mana as though it were mana of any color" /
#     "you may spend other mana only as though it were colorless mana" -
# Celestial Dawn pair tail; Mycosynth Lattice cousin.
@_sp(r"^you may spend\s+(?:any|white|blue|black|red|green|colorless|other)"
     r"\s+mana(?:\s+only)? as though it were mana of\s+"
     r"(?:any color|colorless|the chosen color)\s*$")
def _spend_mana_as_though(m, raw):
    return Static(modification=Modification(
        kind="spend_mana_as_though"), raw=raw)


# --- "all cards that aren't on the battlefield, spells, and permanents
#      are colorless" — Mycosynth Lattice ----------------------------
@_sp(r"^all cards that aren'?t on the battlefield,\s+spells,\s+"
     r"and permanents are colorless\s*$")
def _all_cards_colorless(m, raw):
    return Static(modification=Modification(
        kind="all_cards_colorless"), raw=raw)


# --- "lands you control are <basic>" / "nonland permanents you control
#      are <color>" — Celestial Dawn pair --------------------------
@_sp(r"^(?:lands? you control are\s+(plains|island|swamp|mountain|"
     r"forest|gates?)|"
     r"nonland permanents you control are\s+"
     r"(white|blue|black|red|green|colorless))\s*$")
def _celestial_dawn_pair(m, raw):
    return Static(modification=Modification(
        kind="celestial_dawn_pair"), raw=raw)


# --- "this creature must be blocked by exactly one creature if able" ---
# Nacatl War-Pride.
@_sp(r"^this creature must be blocked by exactly\s+"
     r"(one|two|three|four|five|\d+)\s+creature\s+if able\s*$")
def _must_be_blocked_by_exactly(m, raw):
    return Static(modification=Modification(
        kind="must_be_blocked_by_exactly",
        args=(m.group(1),)), raw=raw)


# --- "this creature can only attack alone" / "this creature assigns no
#      combat damage this combat" — Master of Cruelties pair --------
@_sp(r"^this creature (?:can only attack alone|"
     r"assigns no combat damage(?:\s+this combat)?)\s*$")
def _master_cruelties_pair(m, raw):
    return Static(modification=Modification(
        kind="master_cruelties_pair"), raw=raw)


# --- "you (each )?put the (revealed|chosen) cards into your hand[,
#      then|and] shuffle [the rest into your library]" --------------
# Reveal-pick-shuffle tail (Knight-Errant of Eos / Sibylline / Turtles
# Forever / Stargaze / Guided Passage). Single-direction pair partner
# of base "reveal the top N cards" rules.
@_sp(r"^(?:you may\s+)?put the (?:revealed|chosen|exiled)\s+cards?\s+"
     r"into your hand(?:,?\s+(?:and|then) shuffle"
     r"(?:\s+the rest into your library)?)?\s*$")
def _put_revealed_cards_to_hand(m, raw):
    return Static(modification=Modification(
        kind="put_revealed_cards_to_hand"), raw=raw)


# --- "put the rest of the revealed cards on the bottom of your library
#      [in a random order]" — Sibylline Soothsayer / Tibalt's Trickery -
@_sp(r"^put the rest of the revealed cards on the bottom of your library"
     r"(?:\s+in a random order)?\s*$")
def _put_rest_revealed_bottom(m, raw):
    return Static(modification=Modification(
        kind="put_rest_revealed_bottom"), raw=raw)


# --- "your life total becomes x" / "you can't gain life for the rest
#      of the game" — Welcome the Darkness pair --------------------
@_sp(r"^your life total becomes\s+(x|one|two|three|four|five|six|"
     r"seven|eight|nine|ten|\d+)\s*$")
def _life_total_becomes(m, raw):
    return Static(modification=Modification(
        kind="life_total_becomes",
        args=(m.group(1),)), raw=raw)


@_sp(r"^you can'?t gain life for the rest of the game\s*$")
def _cant_gain_life_rest_game(m, raw):
    return Static(modification=Modification(
        kind="cant_gain_life_rest_game"), raw=raw)


# --- "you can't get poison counters" / "creatures you control can't
#      have <ctr> counters put on them" — Melira pair --------------
@_sp(r"^you can'?t get (poison|rad|experience|energy|ticket)\s+counters\s*$")
def _you_cant_get_counters(m, raw):
    return Static(modification=Modification(
        kind="you_cant_get_named_counters",
        args=(m.group(1),)), raw=raw)


@_sp(r"^creatures you control can'?t have\s+"
     r"([+-]?\d+/[+-]?\d+|[a-z+/0-9\-]+)\s+counters put on them\s*$")
def _creatures_you_control_cant_have_counters(m, raw):
    return Static(modification=Modification(
        kind="creatures_cant_have_counters",
        args=(m.group(1),)), raw=raw)


# --- "<X> can't be chosen" / "the name <X> can't be chosen" — Glimpse,
#     the Unthinkable pair ------------------------------------------
@_sp(r"^(?:the name\s+)?(?:~|[A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)"
     r"\s+can'?t be chosen\s*$")
def _name_cant_be_chosen(m, raw):
    return Static(modification=Modification(
        kind="name_cant_be_chosen"), raw=raw)


# --- "spells you cast cost {WUBRG} less to cast" — Avatar Aang ----------
@_sp(r"^spells you cast cost\s+"
     r"(?:\{[wubrgcxs]\}){2,6}\s+less to cast\s*$")
def _spells_cost_wubrg_less(m, raw):
    return Static(modification=Modification(
        kind="spells_cost_wubrg_less"), raw=raw)


# --- "<juggernauts|tribe> you control can't be blocked by walls" -------
# Graaz, Unstoppable Juggernaut.
@_sp(r"^[a-z]+s? you control can'?t be blocked by walls\s*$")
def _tribe_cant_be_blocked_by_walls(m, raw):
    return Static(modification=Modification(
        kind="tribe_cant_be_blocked_by_walls"), raw=raw)


# --- "<tribe> you control attack each combat if able" -------------------
@_sp(r"^[a-z]+s? you control attack each combat if able\s*$")
def _tribe_attack_each_combat(m, raw):
    return Static(modification=Modification(
        kind="tribe_attack_each_combat"), raw=raw)


# --- "this artifact has all activated abilities of all lands on the
#      battlefield" — Manascape Refractor ---------------------------
@_sp(r"^this (?:artifact|creature|permanent) has all activated abilities "
     r"of all (?:lands|creatures|artifacts|enchantments)\s+"
     r"on the battlefield\s*$")
def _has_all_activated_abilities_of(m, raw):
    return Static(modification=Modification(
        kind="has_all_activated_abilities_of"), raw=raw)


# --- "<self> has all activated abilities of all cards in exile with
#      <noun> counters on them" — Rex, Cyber-Hound ------------------
@_sp(r"^[a-z~]+ has all activated abilities of all cards in exile\s+"
     r"with\s+[a-z+/0-9\-]+\s+counters on them\s*$")
def _has_abilities_exiled_with_ctrs(m, raw):
    return Static(modification=Modification(
        kind="has_abilities_exiled_with_counters"), raw=raw)


# --- "creatures the active player controls attack this turn if able" --
# Siren's Call.
@_sp(r"^creatures the active player controls attack this turn if able\s*$")
def _active_player_attack_if_able(m, raw):
    return Static(modification=Modification(
        kind="active_player_attack_if_able"), raw=raw)


# --- "creatures target opponent controls attack this turn if able" ----
# Imaginary Threats.
@_sp(r"^creatures target (?:opponent|player) controls attack this turn\s+"
     r"if able\s*$")
def _target_opp_creatures_attack(m, raw):
    return Static(modification=Modification(
        kind="target_opp_creatures_attack"), raw=raw)


# --- "during that player's next untap step, creatures they control
#      don't untap" — Imaginary Threats tail ----------------------
@_sp(r"^during that player'?s next untap step,?\s+"
     r"creatures they control don'?t untap\s*$")
def _next_untap_step_dont_untap(m, raw):
    return Static(modification=Modification(
        kind="next_untap_step_dont_untap"), raw=raw)


# --- "you can't attack that player this turn" / "you can't sacrifice
#      those creatures this turn" — Call for Aid post-effect tail ---
@_sp(r"^you can'?t (?:attack that (?:player|planeswalker)|"
     r"sacrifice (?:those creatures|that creature|those permanents))\s+"
     r"this turn\s*$")
def _you_cant_act_post_effect(m, raw):
    return Static(modification=Modification(
        kind="you_cant_act_post_effect"), raw=raw)


# --- "<target creature> blocks each attacking creature this turn if able" -
# Blaze of Glory tail.
@_sp(r"^it blocks each attacking creature this turn if able\s*$")
def _blocks_each_attacking_if_able(m, raw):
    return Static(modification=Modification(
        kind="blocks_each_attacking_if_able"), raw=raw)


# --- "target creature defending player controls can block any number
#      of creatures this turn" — Blaze of Glory ---------------------
@_sp(r"^target creature defending player controls can block any number\s+"
     r"of creatures this turn\s*$")
def _target_def_block_any_number(m, raw):
    return Static(modification=Modification(
        kind="target_def_block_any_number"), raw=raw)


# --- "other creatures that player controls can't block this turn" -----
# Mark for Death tail.
@_sp(r"^other creatures that player controls can'?t block this turn\s*$")
def _other_creatures_cant_block(m, raw):
    return Static(modification=Modification(
        kind="other_creatures_cant_block"), raw=raw)


# --- "target creature an opponent controls blocks this turn if able" --
@_sp(r"^target creature an opponent controls blocks this turn if able\s*$")
def _target_opp_creature_blocks(m, raw):
    return Static(modification=Modification(
        kind="target_opp_creature_blocks"), raw=raw)


# --- "until end of turn, creatures other than ~ and the chosen creature
#      get -2/-2" — Zenos yae Galvus mass debuff -----------------
@_sp(r"^until end of turn, creatures other than\s+"
     r"~?\s*(?:and the chosen creature)?\s+gets?\s+"
     r"([+-]\d+)/([+-]\d+)\s*$")
def _eot_creatures_other_anthem(m, raw):
    return Static(modification=Modification(
        kind="eot_creatures_other_anthem",
        args=(m.group(1), m.group(2))), raw=raw)


# --- "for as long as those cards remain exiled, you may look at them,
#      you may cast permanent spells from among them, and you may spend
#      mana as though it were mana of any color to cast those spells" --
# Arvinox, the Mind Flail. Long disjunctive grant.
@_sp(r"^for as long as those cards remain exiled,?\s+"
     r"you may look at them,?\s+you may cast [^.]+? from among them"
     r"(?:,?\s+and you may spend mana as though it were mana of any color"
     r"\s+to cast those spells)?\s*$")
def _arvinox_grant(m, raw):
    return Static(modification=Modification(
        kind="for_as_long_as_exiled_grant"), raw=raw)


# --- "you may play lands from among cards exiled with this enchantment"
# Hedonist's Trove.
@_sp(r"^you may play lands from among cards exiled with this\s+"
     r"(?:enchantment|artifact|creature|permanent)\s*$")
def _play_lands_exiled_with(m, raw):
    return Static(modification=Modification(
        kind="play_lands_exiled_with"), raw=raw)


# --- "you can't cast more than one spell this way each turn" ----------
# Hedonist's Trove tail / similar.
@_sp(r"^you can'?t cast more than (?:one|two|three|\d+) spells? this way\s+"
     r"each turn\s*$")
def _cant_cast_more_than_n_each_turn(m, raw):
    return Static(modification=Modification(
        kind="cant_cast_more_than_n_each_turn"), raw=raw)


# --- "vehicles you control are the chosen creature type" / Lifecraft
#     Engine — already above; here adding "creatures other than this
#     vehicle" companion (handled above).


# ---------------------------------------------------------------------------
# EFFECT_RULES — sub-sentence resolutions for paired tails
# ---------------------------------------------------------------------------

EFFECT_RULES: list[tuple[re.Pattern, callable]] = []


def _er(pattern: str):
    def deco(fn):
        EFFECT_RULES.append((re.compile(pattern, re.I | re.S), fn))
        return fn
    return deco


# --- "each friend <verb> ..." / "each foe <verb> ..." Pir's Whim shape --
# Pair signature: every Pir's-Whim/Virtus/Regna/Zndrsplt card has BOTH
# friend AND foe sentence — both must promote.
@_er(r"^each friend\s+[^.]+?(?:\.|$)")
def _each_friend(m):
    return UnknownEffect(raw_text="each friend <action>")


@_er(r"^each foe\s+[^.]+?(?:\.|$)")
def _each_foe(m):
    return UnknownEffect(raw_text="each foe <action>")


# --- "you may accept any one offer" — Deal Broker tail ----------------
@_er(r"^you may accept any (?:one|two|three|\d+) offers?(?:\.|$)")
def _accept_any_offer(m):
    return UnknownEffect(raw_text="you may accept any N offers")


# --- "each other player may offer you one card in their card pool ..."
@_er(r"^each other player may offer you (?:one|two|three|\d+) cards?\s+"
     r"in their card pool[^.]*?(?:\.|$)")
def _each_other_offer(m):
    return UnknownEffect(raw_text="each other player may offer cards")


# --- "immediately after the draft, you may <verb> ..." -----------------
@_er(r"^immediately after the draft,?\s+you may [^.]+?(?:\.|$)")
def _immediately_after_draft(m):
    return UnknownEffect(raw_text="immediately after the draft, you may X")


# --- "you may cast the exiled card[s] without paying [its|their] mana
#     cost[s]" — paired post-reveal cast-without-paying tail. ---------
@_er(r"^(?:that opponent|target opponent|that player|you|they)\s+"
     r"may cast (?:the|that) exiled cards?\s+"
     r"without paying (?:its|their) mana costs?(?:\.|$)")
def _may_cast_exiled_no_pay(m):
    return UnknownEffect(raw_text="may cast exiled card(s) without paying")


# --- "you may cast an instant or sorcery spell from among them without
#     paying its mana cost" — Talent of the Telepath ----------------
@_er(r"^you may cast an?\s+[a-z ]+?spell from among them\s+"
     r"without paying its mana cost(?:\.\s+then[^.]+?)?(?:\.|$)")
def _cast_spell_among_them_no_pay(m):
    return UnknownEffect(raw_text="cast spell from pile without paying")


# --- "copy those cards" / "copy that spell" / "copy the exiled card" --
@_er(r"^copy (?:those cards|that spell|the exiled card|the copy|"
     r"that card)(?:\.|$)")
def _copy_those_cards(m):
    return UnknownEffect(raw_text="copy those/that")


# --- "cast the copies if able without paying their mana costs" --------
@_er(r"^cast the copies?\s+if able\s+"
     r"without paying their mana costs?(?:\.|$)")
def _cast_copies_if_able(m):
    return UnknownEffect(raw_text="cast the copies if able without paying")


# --- "leave the chosen cards in your graveyard and put the rest into
#     your hand" — Deliver Unto Evil ----------------------------------
@_er(r"^leave the chosen cards in your graveyard\s+"
     r"and put the rest into your hand(?:\.|$)")
def _leave_chosen_put_rest_hand(m):
    return UnknownEffect(raw_text="leave chosen, put rest into hand")


# --- "you choose <N> of those cards and put them into <zone>" --------
@_er(r"^you choose (?:one|two|three|four|five|\d+) of those cards\s+"
     r"and put them into [^.]+?(?:\.|$)")
def _you_choose_n_put_them(m):
    return UnknownEffect(raw_text="you choose N of those cards, put into zone")


# --- "you choose a card from among those cards" — Break Expectations -
@_er(r"^you choose an?\s+(?:card|creature|land|nonland|noncreature|"
     r"instant|sorcery|enchantment|artifact|planeswalker)\s+card?\s+"
     r"from among (?:those cards|them)(?:\.|$)")
def _you_choose_typed_among(m):
    return UnknownEffect(raw_text="you choose a card from among those")


# --- "exile any number of them in a face-down pile and the rest in a
#     face-up pile" — The Celestial Toymaker / Phyrexian Portal ------
@_er(r"^exile any number of them in an?\s+face-(?:up|down)\s+pile\s+"
     r"and the rest in an?\s+face-(?:up|down)\s+pile(?:\.|$)")
def _exile_pile_split(m):
    return UnknownEffect(raw_text="exile in two face-up/down piles")


# --- "defending player chooses one of those piles" / "exile one of
#     those piles" / "search the other pile for a card ..." --------
@_er(r"^(?:defending player|target opponent|target player|you)\s+"
     r"chooses one of those piles(?:\.|$)")
def _player_chooses_pile(m):
    return UnknownEffect(raw_text="player chooses one of those piles")


@_er(r"^exile one of those piles(?:\.|$)")
def _exile_one_pile(m):
    return UnknownEffect(raw_text="exile one of those piles")


@_er(r"^search the other pile for an?\s+[a-z]+,?\s+put it into your hand,"
     r"\s+then shuffle the rest of that pile into your library(?:\.|$)")
def _search_other_pile(m):
    return UnknownEffect(raw_text="search the other pile, put into hand, shuffle")


# --- "draw a card[s] for each <noun> [...]" left orphan in compounds ---
@_er(r"^draw a card for each [a-z ]+?(?:that died|that you control|"
     r"in your graveyard|on the battlefield)?(?:\.|$)")
def _draw_card_for_each_noun(m):
    return UnknownEffect(raw_text="draw a card for each <noun>")


# --- "each opponent loses x life, where x is the number of <noun>
#     cards in your graveyard" --------------------------------------
@_er(r"^each opponent loses x life,?\s+where x is the number of\s+"
     r"[a-z ]+?\s+(?:cards in your graveyard|creatures? you control|"
     r"permanents you control)(?:\.|$)")
def _each_opp_loses_x_life_count(m):
    return UnknownEffect(raw_text="each opp loses X life from count")


# --- "they may cast that card without paying its mana cost. then they
#     put the exiled cards on the bottom of their library in a random
#     order" — Tibalt's Trickery ---------------------------------
@_er(r"^they may cast that card without paying its mana cost\.?\s*"
     r"then they put the exiled cards on the bottom of their library\s+"
     r"in a random order(?:\.|$)")
def _tibalts_trickery_tail(m):
    return UnknownEffect(raw_text="they may cast that card, bottom in random")


# --- "starting with you and proceeding in the chosen direction, each
#     player <verb>" — Order of Succession turn-order shape -------
@_er(r"^starting with you and proceeding in (?:the chosen|a chosen|"
     r"clockwise|counter-?clockwise)\s+(?:direction|order),?\s+"
     r"each player [^.]+?(?:\.|$)")
def _starting_with_you_proceeding(m):
    return UnknownEffect(raw_text="starting with you, each player ...")


# --- "each player gains control of the creature they chose" -----------
@_er(r"^each player gains control of the (?:creature|permanent|card)\s+"
     r"they chose(?:\.|$)")
def _each_player_gains_chosen(m):
    return UnknownEffect(raw_text="each player gains control of chosen")


# --- "each player chooses up to <N> cards in their hand, then shuffles
#     the rest into their library" — Worldpurge -------------------
@_er(r"^each player chooses up to (?:one|two|three|four|five|six|seven|\d+)"
     r"\s+cards in their hand,?\s+then shuffles the rest into their\s+"
     r"library(?:\.|$)")
def _each_player_chooses_shuffles_rest(m):
    return UnknownEffect(raw_text="each player keeps N, shuffles rest")


# --- "each player loses all unspent mana" -----------------------------
@_er(r"^each player loses all unspent mana(?:\.|$)")
def _each_player_loses_unspent(m):
    return UnknownEffect(raw_text="each player loses all unspent mana")


# --- "while an opponent is searching their library, they exile each
#     card they find" / "you control your opponents while they're
#     searching their libraries" — Opposition Agent pair ----------
@_er(r"^while an opponent is searching their library,\s+"
     r"they exile each card they find(?:\.|$)")
def _opp_searching_exile_finds(m):
    return UnknownEffect(raw_text="opp searching library exiles finds")


@_er(r"^you control your opponents while they'?re searching their\s+"
     r"libraries(?:\.|$)")
def _you_control_opponents_searching(m):
    return UnknownEffect(raw_text="you control opponents while searching")


# --- "cards exiled this way that don't have suspend gain suspend" ---
# The Wedding of River Song.
@_er(r"^cards exiled this way that don'?t have (?:suspend|flashback|plot|"
     r"foretell|adapt|escape)\s+gain (?:suspend|flashback|plot|foretell|"
     r"adapt|escape)(?:\.|$)")
def _exiled_no_kw_gain_kw(m):
    return UnknownEffect(raw_text="exiled cards without K gain K")


# --- "you investigate x times, where x is one plus the number of
#     opponents who investigated this way" -----------------------
@_er(r"^you investigate x times,?\s+where x is\s+"
     r"(?:one|two|three|\d+)\s+plus the number of opponents who\s+"
     r"investigated this way(?:\.|$)")
def _investigate_x_times(m):
    return UnknownEffect(raw_text="investigate X times based on opp count")


# --- "each opponent who doesn't loses N life" ----------------------
@_er(r"^each opponent who doesn'?t loses (?:one|two|three|\d+) life(?:\.|$)")
def _each_opp_who_doesnt_loses(m):
    return UnknownEffect(raw_text="each opp who doesn't loses N life")


# --- "they put one on the bottom of your library. then you put one
#     into your hand. then they put one on the bottom of your
#     library" — Memories Returning trade ----------------------
@_er(r"^they put one on the bottom of your library\.?\s+"
     r"then you put one into your hand\.?\s+"
     r"then they put one on the bottom of your library(?:\.|$)")
def _memories_returning_trade(m):
    return UnknownEffect(raw_text="they/you/they bottom-and-hand trade")


@_er(r"^put the other into your hand(?:\.|$)")
def _put_the_other_into_hand(m):
    return UnknownEffect(raw_text="put the other into your hand")


# --- "you each lose life equal to the mana value of the card revealed
#     by the other player" — Keen Duelist ---------------------
@_er(r"^you each lose life equal to the mana value of the card revealed\s+"
     r"by the other player(?:\.|$)")
def _keen_duelist_pair_loss(m):
    return UnknownEffect(raw_text="you each lose life from other's reveal")


@_er(r"^you each put the card you revealed into your hand(?:\.|$)")
def _keen_duelist_pair_hand(m):
    return UnknownEffect(raw_text="you each put revealed into hand")


# --- "you and target spell's controller bid life" / "the high bidder
#     loses life equal to the high bid" — Mages' Contest --------
@_er(r"^you and target [a-z'’ ]+? controller bid life(?:\.|$)")
def _mages_contest_bid(m):
    return UnknownEffect(raw_text="you and X bid life")


@_er(r"^the high bidder loses life equal to the high bid(?:\.|$)")
def _high_bidder_loses(m):
    return UnknownEffect(raw_text="high bidder loses life equal to bid")


# --- "an opponent guesses whether the top card of your library is the
#     chosen kind" — Gollum, Scheming Guide -------------------
@_er(r"^an opponent guesses whether the top card of your library\s+"
     r"is the chosen kind(?:\.|$)")
def _opp_guesses_top(m):
    return UnknownEffect(raw_text="opponent guesses top card kind")


# --- "reveal that card" -----------------------------------------
@_er(r"^reveal that card(?:\.|$)")
def _reveal_that_card(m):
    return UnknownEffect(raw_text="reveal that card")


# --- "secretly choose <X> or <Y>" / "seek <N> cards of the chosen
#     kind" — Cunning Azurescale tail ----------------------
@_er(r"^secretly choose [a-z]+(?:\s+or\s+[a-z]+)+(?:\.|$)")
def _secretly_choose(m):
    return UnknownEffect(raw_text="secretly choose X or Y")


@_er(r"^seek (?:a|an|one|two|three|four|five|\d+)\s+(?:cards?|land card|"
     r"creature card|nonland card)\s+of the chosen kind(?:\.|$)")
def _seek_chosen_kind(m):
    return UnknownEffect(raw_text="seek N cards of the chosen kind")


# --- "remove all attacking creatures from combat and untap them" ---
@_er(r"^remove all attacking creatures from combat\s+"
     r"and untap them(?:\.|$)")
def _remove_attackers_untap(m):
    return UnknownEffect(raw_text="remove attackers from combat and untap")


# --- "they can't attack you or planeswalkers you control that combat" -
@_er(r"^they can'?t attack you or planeswalkers you control\s+"
     r"that combat(?:\.|$)")
def _cant_attack_you_pw_combat(m):
    return UnknownEffect(raw_text="they can't attack you/PW that combat")


# --- "each of those creatures attacks that combat if able" ---------
@_er(r"^each of those creatures attacks that combat if able(?:\.|$)")
def _each_those_attacks_combat(m):
    return UnknownEffect(raw_text="each of those creatures attacks combat")


# --- "remove target creature defending player controls from combat" /
#     "you may have it block an attacking creature of your choice" -
@_er(r"^remove target creature defending player controls from combat"
     r"(?:\s+and untap it)?(?:\.|$)")
def _remove_def_creature_combat(m):
    return UnknownEffect(raw_text="remove def creature from combat")


@_er(r"^you may have it block an attacking creature of your choice"
     r"(?:\.|$)")
def _it_may_block_chosen(m):
    return UnknownEffect(raw_text="you may have it block chosen attacker")


# --- "create a token that's a copy of <X>, except <Y>" left orphan ---
@_er(r"^create a token that'?s a copy of\s+~"
     r"(?:,?\s+except [^.]+?)?(?:\.|$)")
def _create_copy_self(m):
    return UnknownEffect(raw_text="create token copy of self")


# --- "untap those creatures and they gain haste until end of turn" ---
@_er(r"^untap those creatures and they gain haste until end of turn"
     r"(?:\.|$)")
def _untap_those_haste(m):
    return UnknownEffect(raw_text="untap those creatures, gain haste EOT")


# --- "each opponent chooses fame or fortune" — Seize the Spotlight --
@_er(r"^each opponent chooses fame or fortune(?:\.|$)")
def _each_opp_chooses_fame_fortune(m):
    return UnknownEffect(raw_text="each opp chooses fame or fortune")


# --- "the chosen creature gains <kw> until end of turn" ---------------
@_er(r"^the chosen creature gains\s+[a-z, ]+?\s+until end of turn(?:\.|$)")
def _chosen_creature_gains_kw_eot(m):
    return UnknownEffect(raw_text="chosen creature gains kw EOT")


# --- "an opponent of your choice gains control of it" ----------------
@_er(r"^an opponent of your choice gains control of it(?:\.|$)")
def _opp_choice_gains_control(m):
    return UnknownEffect(raw_text="opp of your choice gains control of it")


# --- "an opponent exiles a nonland card from among them, then you put
#     the rest into your hand" — Allure of the Unknown ---------
@_er(r"^an opponent exiles an?\s+(?:nonland|land|creature|noncreature|"
     r"instant|sorcery|enchantment|artifact|planeswalker)\s+card\s+"
     r"from among them,?\s+then you put the rest into your hand(?:\.|$)")
def _opp_exiles_typed_rest_hand(m):
    return UnknownEffect(raw_text="opp exiles typed, rest into hand")


# --- "create a tapped 1/1 white spirit creature token with flying" /
#     "you [and its controller each] draw a card [and lose 2 life]" /
#     "multicleave {N}" — Cleaver Blow tier ----------------------
@_er(r"^create a (?:tapped\s+)?\d+/\d+\s+(?:white|blue|black|red|green|"
     r"colorless)\s+[a-z ]+?\s+creature token(?:\s+with [a-z, ]+?)?(?:\.|$)")
def _create_typed_token(m):
    return UnknownEffect(raw_text="create typed token")


@_er(r"^multicleave\s+\{[^}]+\}(?:\{[^}]+\})*(?:\.|$)")
def _multicleave_keyword(m):
    return UnknownEffect(raw_text="multicleave {cost}")


# --- "you [and its controller each] draw a card [and lose N life]" ---
@_er(r"^you (?:and its controller each\s+)?draw a card"
     r"(?:\s+and lose (?:one|two|three|\d+) life)?(?:\.|$)")
def _you_and_controller_draw(m):
    return UnknownEffect(raw_text="you (and ctrl) draw, optionally lose life")


# --- "draw two cards, then you may exile a nonland card from your
#     hand with a number of time counters on it equal to its mana
#     value" — The Wedding of River Song ----------------------
@_er(r"^draw (?:a|one|two|three|four|five|\d+) cards?,?\s+"
     r"then you may exile an?\s+(?:nonland|land|creature)?\s*card\s+"
     r"from your hand with a number of time counters on it\s+"
     r"equal to its mana value(?:\.\s+then [^.]+?)?(?:\.|$)")
def _draw_then_exile_time_counters(m):
    return UnknownEffect(raw_text="draw, then exile to time counters")


# --- "destroy it [and this creature] at end of combat" — Goblin Sappers
@_er(r"^destroy it(?:\s+and this creature)?\s+at end of combat(?:\.|$)")
def _destroy_it_at_eoc(m):
    return UnknownEffect(raw_text="destroy it (and this) at end of combat")


# --- "you may turn a creature you control face up" — Hustle // Bustle -
@_er(r"^you may turn a creature you control face up(?:\.|$)")
def _may_turn_face_up(m):
    return UnknownEffect(raw_text="you may turn a creature face up")


# --- "target creature attacks or blocks this turn if able" ----------
@_er(r"^target creature attacks or blocks this turn if able(?:\.|$)")
def _target_creature_atk_or_blk(m):
    return UnknownEffect(raw_text="target creature attacks or blocks if able")


# --- "their controller chooses and sacrifices one of them" / "return
#     the other to its owner's hand" — Barrin's Spite -----------
@_er(r"^their controller chooses and sacrifices one of them(?:\.|$)")
def _ctrl_chooses_sacs_one(m):
    return UnknownEffect(raw_text="their controller chooses, sacrifices one")


@_er(r"^return the other to its owner'?s hand(?:\.|$)")
def _return_the_other_owner(m):
    return UnknownEffect(raw_text="return the other to owner's hand")


# --- "exile the tokens" — Nacatl War-Pride tail --------------------
@_er(r"^exile the tokens?(?:\.|$)")
def _exile_the_tokens(m):
    return UnknownEffect(raw_text="exile the tokens")


# --- "rex has all activated abilities of all cards in exile with brain
#     counters on them" — covered by static. Pair tail: "exile it
#     with a brain counter on it" --------------------------------
@_er(r"^exile it with an?\s+[a-z+/0-9\-]+\s+counter on it(?:\.|$)")
def _exile_it_with_counter(m):
    return UnknownEffect(raw_text="exile it with a <noun> counter")


# --- "darigaaz: ~ and that dragon card each perpetually get +1/+1" ---
@_er(r"^~ and that\s+[a-z]+\s+card each perpetually get\s+"
     r"([+-]\d+)/([+-]\d+)(?:\.|$)")
def _self_and_typed_perp_pump(m):
    return UnknownEffect(raw_text="~ and typed card each perpetually +N/+N")


# --- "sacrifice ~ when that token leaves the battlefield" — Stangg --
@_er(r"^sacrifice ~ when that token leaves the battlefield(?:\.|$)")
def _sac_self_when_token_ltb(m):
    return UnknownEffect(raw_text="sacrifice ~ when that token leaves")


# --- "you gain N life if an opponent <verb>" / "create N <token> if an
#     opponent <verb>" — Beza, the Bounding Spring conditional ETB --
@_er(r"^you gain (?:one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)"
     r"\s+life if an opponent\s+[^.]+?(?:\.|$)")
def _gain_life_if_opp(m):
    return UnknownEffect(raw_text="you gain N life if an opponent <cond>")


@_er(r"^create (?:one|two|three|four|five|x|\d+)\s+"
     r"\d+/\d+\s+[a-z ]+?\s+creature tokens?\s+if an opponent\s+"
     r"[^.]+?(?:\.|$)")
def _create_tokens_if_opp(m):
    return UnknownEffect(raw_text="create tokens if an opponent <cond>")


# --- "lands you control enter tapped this turn" — Nahiri's Lithoforming
@_er(r"^lands you control enter tapped this turn(?:\.|$)")
def _lands_enter_tapped_this_turn(m):
    return UnknownEffect(raw_text="lands you control enter tapped this turn")


@_er(r"^you may play (?:x|one|two|three|\d+) additional lands? this turn"
     r"(?:\.|$)")
def _play_additional_lands_this_turn(m):
    return UnknownEffect(raw_text="you may play X additional lands this turn")


@_er(r"^sacrifice (?:x|one|two|three|\d+) lands?(?:\.|$)")
def _sacrifice_n_lands(m):
    return UnknownEffect(raw_text="sacrifice N lands")


# --- "that planeswalker enters with an additional loyalty counter on
#     it" — Dark Intimations companion ---------------------------
@_er(r"^that planeswalker enters with an additional\s+"
     r"(?:loyalty|[+-]?\d+/[+-]?\d+|[a-z+/0-9\-]+)\s+counter on it(?:\.|$)")
def _that_pw_enters_extra_counter(m):
    return UnknownEffect(raw_text="that PW enters with extra counter")


# --- "you return a creature or planeswalker card from your graveyard
#     to your hand, then draw a card" — Dark Intimations -----------
@_er(r"^you return an?\s+[a-z, ]+?\s+card from your graveyard\s+"
     r"to your hand(?:,?\s+then draw a card)?(?:\.|$)")
def _you_return_typed_then_draw(m):
    return UnknownEffect(raw_text="return typed card from gy to hand, then draw")


# --- "that creature enters with an additional +1/+1 counter on it" /
#     "that spell can't be countered" — Savage Summoning pair ----
@_er(r"^that creature enters with an additional\s+"
     r"([+-]?\d+/[+-]?\d+|[a-z+/0-9\-]+)\s+counter on it(?:\.|$)")
def _that_creature_extra_counter(m):
    return UnknownEffect(raw_text="that creature enters with extra counter")


@_er(r"^that spell can'?t be countered(?:\.|$)")
def _that_spell_cant_be_countered(m):
    return UnknownEffect(raw_text="that spell can't be countered")


# --- "ignore this effect for each creature the player didn't control
#     continuously since the beginning of the turn" — Siren's Call -
@_er(r"^ignore this effect for each creature the player didn'?t\s+"
     r"control continuously since the beginning of the turn(?:\.|$)")
def _ignore_effect_per_recent(m):
    return UnknownEffect(raw_text="ignore effect per non-tenured creature")


# --- "creature cards in your graveyard have <kw> {cost}" Ninja Teen ---
@_sp(r"^creature cards in your graveyard have\s+"
     r"(sneak|encore|escape|flashback|jump-start|disturb|unearth|delve|"
     r"emerge|adapt|adventure|cleave|mayhem|miracle|mutate|warp|plot)"
     r"(?:\s+\{[^}]+\}(?:\{[^}]+\})*)?\s*$")
def _gy_creatures_have_alt_kw(m, raw):
    return Static(modification=Modification(
        kind="gy_creatures_have_alt_kw",
        args=(m.group(1),)), raw=raw)


# --- "you may cast creature spells from your graveyard using their
#     <kw> abilities" — Ninja Teen tail -----------------------
@_er(r"^you may cast (?:creature|land|noncreature|instant or sorcery|"
     r"nonland|enchantment|artifact|planeswalker)\s+spells\s+"
     r"from your graveyard using their\s+[a-z\-]+\s+abilities(?:\.|$)")
def _cast_typed_from_gy_using_kw(m):
    return UnknownEffect(raw_text="cast typed spells from gy using <kw>")


# --- "i" / "am" / "talking! - whenever ~ deals combat damage to a
#     player ..." — The Eleventh Doctor "I AM TALKING!" ability --
@_sp(r"^(?:i|am)\s*$")
def _eleventh_doctor_word_orphan(m, raw):
    return Static(modification=Modification(
        kind="orphan_keyword_word"), raw=raw)


@_sp(r"^talking!\s*[-–—]\s*whenever ~ deals combat damage to a player[^.]*?\s*$")
def _eleventh_doctor_talking(m, raw):
    return Static(modification=Modification(
        kind="eleventh_doctor_talking"), raw=raw)


# --- "as ~ enters, discard any number of creature cards" / "it enters
#     with a flying counter on it if a card discarded this way has
#     flying" — Indominus Rex, Alpha ETB-conditional ----------
@_sp(r"^as ~\s*[a-z]*?\s*enters,?\s+discard any number of\s+"
     r"(?:creature|land|nonland|instant|sorcery|enchantment|artifact|"
     r"planeswalker)\s+cards?\s*$")
def _as_self_enters_discard_typed(m, raw):
    return Static(modification=Modification(
        kind="as_self_enters_discard_typed"), raw=raw)


@_sp(r"^it enters with an?\s+[a-z\-+]+\s+counter on it if a card\s+"
     r"discarded this way has\s+[a-z\-]+\s*$")
def _it_enters_extra_counter_if_discarded_kw(m, raw):
    return Static(modification=Modification(
        kind="extra_counter_if_discarded_has_kw"), raw=raw)


# --- "elemental permanent spells you cast from your hand gain evoke
#     {N} as you cast them" — Ashling, the Limitless ----------
@_sp(r"^[a-z]+\s+(?:permanent|creature|noncreature|nonland)\s+spells\s+"
     r"you cast(?:\s+from your hand)?\s+gain\s+"
     r"(evoke|prowl|surge|emerge|cleave|warp|adapt|kicker|mutate|jump-start)"
     r"\s+\{[^}]+\}(?:\{[^}]+\})*\s+as you cast them\s*$")
def _typed_spells_gain_alt_cost_kw(m, raw):
    return Static(modification=Modification(
        kind="typed_spells_gain_alt_cost_kw",
        args=(m.group(1),)), raw=raw)


# --- "the token gains haste until end of turn" — generic tail -------
@_er(r"^the token gains haste until end of turn(?:\.|$)")
def _token_gains_haste_eot(m):
    return UnknownEffect(raw_text="the token gains haste EOT")


# --- "spells your opponents cast with the chosen name cost {2} more
#     to cast" / "activated abilities of sources with the chosen
#     name cost {2} more to activate unless they're mana abilities" -
@_sp(r"^spells your opponents cast with the chosen name cost\s+"
     r"\{[^}]+\}(?:\{[^}]+\})*\s+more to cast\s*$")
def _opp_spells_chosen_name_cost(m, raw):
    return Static(modification=Modification(
        kind="opp_spells_chosen_name_cost"), raw=raw)


@_sp(r"^activated abilities of sources with the chosen name cost\s+"
     r"\{[^}]+\}(?:\{[^}]+\})*\s+more to activate\s+"
     r"unless they'?re mana abilities\s*$")
def _activated_chosen_name_cost(m, raw):
    return Static(modification=Modification(
        kind="activated_chosen_name_cost"), raw=raw)


# --- "this aura enters with three ore counters on it" / "when the
#     last ore counter is removed from this aura, destroy enchanted
#     land and this aura deals 2 damage to that land's controller" -
@_sp(r"^this (?:aura|equipment|artifact|enchantment)\s+enters with\s+"
     r"(one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+"
     r"[a-z+/0-9\-]+\s+counters on it\s*$")
def _this_aura_enters_with_counters(m, raw):
    return Static(modification=Modification(
        kind="this_aura_enters_with_counters",
        args=(m.group(1),)), raw=raw)


# --- "exile that card with three time counters on it" — Sibylline ---
@_er(r"^exile that card with\s+"
     r"(?:one|two|three|four|five|six|seven|eight|nine|ten|x|\d+)\s+"
     r"[a-z+/0-9\-]+\s+counters on it(?:\.|$)")
def _exile_that_card_with_counters(m):
    return UnknownEffect(raw_text="exile that card with N <kw> counters")


# --- "an opponent chooses from among them a creature card, a land card,
#     and a noncreature, nonland card" — Guided Passage -----------
@_er(r"^an opponent chooses from among them\s+"
     r"an?\s+[a-z]+\s+card,?\s+an?\s+[a-z]+\s+card,?\s+"
     r"and an?\s+[a-z, ]+?\s+card(?:\.|$)")
def _opp_chooses_three_typed(m):
    return UnknownEffect(raw_text="opp chooses three typed cards from pile")


# --- "reveal the cards in your library" — Guided Passage tail ----
@_er(r"^reveal the cards in your library(?:\.|$)")
def _reveal_cards_in_library(m):
    return UnknownEffect(raw_text="reveal the cards in your library")
