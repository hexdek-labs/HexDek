#!/usr/bin/env python3
"""Semantic clustering of MTG cards by canonical effect AST.

The premise (per Josh + 7174n1c): the engine's surface area isn't the count of
unique oracle-text phrasings, it's the count of unique *game-state mutations*.
Basic landcycling and "search your library for a basic land card" are the same
mutation in two surface forms — they share one handler.

Pipeline:
  oracle_text → keyword expansion → pattern matching → canonical effect set
  cards with identical effect sets share a handler.

Output:
  data/rules/semantic_clusters.md — the work queue. Top-N clusters by card
  count are the highest-ROI handlers to build first.

Usage: python3 scripts/semantic_clusters.py
"""

from __future__ import annotations

import json
import re
from collections import defaultdict
from pathlib import Path

# ============================================================================
# Paths
# ============================================================================

ROOT = Path(__file__).resolve().parents[1]
ORACLE_DUMP = ROOT / "data" / "rules" / "oracle-cards.json"
REPORT = ROOT / "data" / "rules" / "semantic_clusters.md"


# ============================================================================
# Card-pool filter
# ============================================================================
# Skip non-game card types (tokens are output, not input; planes/schemes are
# Vanguard/Planechase metaformats; bios are collector inserts; silver-bordered
# Unfinity cards are joke mechanics not worth modeling).

SKIP_TYPES = {"token", "scheme", "plane", "phenomenon", "vanguard",
              "conspiracy", "dungeon"}
SKIP_SET_TYPES = {"memorabilia", "token", "minigame", "funny"}


def is_real_card(card: dict) -> bool:
    types = (card.get("type_line") or "").lower()
    if any(t in types for t in SKIP_TYPES):
        return False
    if card.get("set_type") in SKIP_SET_TYPES:
        return False
    if card.get("border_color") == "silver":
        return False
    if (card.get("name") or "").endswith(" Bio"):
        return False
    return True


# ============================================================================
# Keyword expansion: shorthand keywords → canonical activated/triggered text
# ============================================================================
# Many keywords are shorthand for a specific activated or triggered ability.
# Expanding them before clustering lets the clusterer see the underlying
# effect, so e.g. "Cycling {2}" clusters with "{2}, Discard this card: Draw."

KEYWORD_EXPANSIONS = [
    # ---- Cycling family — activated discard for draw / typed tutor ----
    (r"\bcycling \{([^}]+)\}",        r"{\1}, discard ~: draw a card."),
    (r"\bplainscycling \{([^}]+)\}",  r"{\1}, discard ~: search your library for a plains card, put it into your hand, then shuffle."),
    (r"\bislandcycling \{([^}]+)\}",  r"{\1}, discard ~: search your library for an island card, put it into your hand, then shuffle."),
    (r"\bswampcycling \{([^}]+)\}",   r"{\1}, discard ~: search your library for a swamp card, put it into your hand, then shuffle."),
    (r"\bmountaincycling \{([^}]+)\}", r"{\1}, discard ~: search your library for a mountain card, put it into your hand, then shuffle."),
    (r"\bforestcycling \{([^}]+)\}",  r"{\1}, discard ~: search your library for a forest card, put it into your hand, then shuffle."),
    (r"\bbasic landcycling \{([^}]+)\}", r"{\1}, discard ~: search your library for a basic land card, put it into your hand, then shuffle."),
    (r"\blandcycling \{([^}]+)\}",    r"{\1}, discard ~: search your library for a land card, put it into your hand, then shuffle."),

    # ---- Alt-cast costs ----
    (r"\bflashback \{([^}]+)\}",      r"you may cast ~ from your graveyard for {\1}. exile ~ on resolution."),
    (r"\bbuyback \{([^}]+)\}",        r"as an additional cost to cast ~ you may pay {\1}. if you do, return ~ to hand instead of graveyard."),
    (r"\bmadness \{([^}]+)\}",        r"if ~ is discarded, you may cast it from exile for {\1}."),
    (r"\bkicker \{([^}]+)\}",         r"as an additional cost to cast ~, you may pay {\1}."),
    (r"\bmultikicker \{([^}]+)\}",    r"as an additional cost to cast ~, you may pay {\1} any number of times."),
    (r"\bbestow \{([^}]+)\}",         r"you may cast ~ for {\1} as an aura."),
    (r"\bsuspend (\d+) \{([^}]+)\}",  r"exile ~ from hand with \1 time counters, paying {\2}."),

    # ---- Graveyard recursion / token creation from grave ----
    (r"\bdredge (\d+)",               r"if you would draw a card, you may instead mill \1 cards and return ~ from graveyard to hand."),
    (r"\bembalm \{([^}]+)\}",         r"{\1}, exile ~ from graveyard: create a token that's a copy of ~."),
    (r"\beternalize \{([^}]+)\}",     r"{\1}, exile ~ from graveyard: create a 4/4 token that's a copy of ~."),
    (r"\bunearth \{([^}]+)\}",        r"{\1}: return ~ from graveyard to battlefield with haste. exile it at the next end step."),
    (r"\bencore \{([^}]+)\}",         r"{\1}, exile ~ from graveyard: create token copies that attack each opponent."),
    (r"\bscavenge \{([^}]+)\}",       r"{\1}, exile ~ from graveyard: put +1/+1 counters on target creature."),

    # ---- Death / undying triggers ----
    (r"\bpersist",                    r"when ~ dies, if it had no -1/-1 counters on it, return it to the battlefield with a -1/-1 counter."),
    (r"\bundying",                    r"when ~ dies, if it had no +1/+1 counters on it, return it to the battlefield with a +1/+1 counter."),

    # ---- Counter/pump triggers ----
    (r"\boutlast \{([^}]+)\}",        r"{\1}, {t}: put a +1/+1 counter on ~. activate only as a sorcery."),
    (r"\breinforce (\d+) \{([^}]+)\}", r"{\2}, discard ~: put \1 +1/+1 counters on target creature."),
    (r"\badapt (\d+)",                r"if ~ has no +1/+1 counters, put \1 +1/+1 counters on it."),
    (r"\bmonstrosity (\d+)",          r"if ~ is not monstrous, put \1 +1/+1 counters on it and it becomes monstrous."),
    (r"\bdevour (\d+)",               r"as ~ enters, may sacrifice creatures. ~ enters with \1 +1/+1 counters per creature sacrificed."),

    # ---- Equip / vehicle ----
    (r"\bequip \{([^}]+)\}",          r"{\1}: attach ~ to target creature you control."),
    (r"\bcrew (\d+)",                 r"tap any number of other creatures with total power \1 or more: ~ becomes an artifact creature."),

    # ---- Cost-reduction keywords ----
    (r"\bconvoke",                    r"convoke (creatures you control may tap to pay generic mana)."),
    (r"\bimprovise",                  r"improvise (artifacts you control may tap to pay generic mana)."),
]


def expand_keywords(text: str) -> str:
    for pat, repl in KEYWORD_EXPANSIONS:
        text = re.sub(pat, repl, text, flags=re.I)
    return text


# ============================================================================
# Keyword-only detection
# ============================================================================
# A card whose entire normalized text is keyword listings (Flying, Trample,
# Cycling {2}, etc.) clusters as KEYWORDS_ONLY — a single canonical handler
# covers vanilla creatures and pure-keyword cards.

_KEYWORDS = {
    # Evergreen
    "flying", "trample", "haste", "vigilance", "deathtouch", "lifelink",
    "first strike", "double strike", "reach", "hexproof", "indestructible",
    "menace", "defender", "flash", "shroud", "ward",
    # Common deciduous + landwalk variants
    "infect", "wither", "skulk", "intimidate", "fear", "shadow", "horsemanship",
    "prowess", "scry", "landwalk", "swampwalk", "islandwalk", "forestwalk",
    "mountainwalk", "plainswalk", "legendary landwalk", "nonbasic landwalk",
    # Action keywords
    "regenerate", "fight", "exile", "sacrifice", "discard", "tap", "untap",
    "attach", "destroy", "counter", "investigate", "explore", "venture",
    "amass", "learn", "surveil", "proliferate",
    # Counter keywords
    "modular", "graft", "fading", "vanishing", "training",
    # Set-specific keywords often appearing alone
    "changeling", "delve", "devoid", "evolve", "exalted", "exploit", "extort",
    "outlast", "phasing", "populate", "provoke", "rampage", "rebound",
    "renown", "scavenge", "soulbond", "soulshift", "sunburst", "support",
    "tribute", "unearth", "undying", "persist", "haunt", "morbid",
    "delirium", "metalcraft", "spell mastery", "ferocious", "formidable",
    "hellbent", "battalion", "constellation", "magecraft", "coven",
    "ashiok",  # placeholder for misc set keywords
    # Newer set keywords
    "decayed", "blitz", "casualty", "channel", "compleated", "encore",
    "exhaust", "celebration", "domain", "raid", "revolt", "boast",
    "discover", "corrupted", "plot", "saddle", "station", "tiered",
    "expend", "valiant", "harmonize", "renew", "freerunning", "impending",
    "warp", "max speed", "gift", "mutate", "doctor's companion",
    "the ring tempts you", "ring-bearer", "for mirrodin", "more than meets the eye",
    "living metal", "umbra armor", "totem armor",
    # Ability words (act like keywords once their trigger text is split off)
    "heroic", "survival", "inspired", "eerie", "alliance", "addendum",
    "lieutenant", "parley", "radiance", "tempting offer", "undergrowth",
    "will of the council", "fateful hour", "kinship", "join forces",
    "tap chord", "paradox", "disappear", "lightning breath", "celestial breath",
    "fire breath", "icebreath", "dragon breath", "venom", "wail",
    "psionic blast", "start your engines", "draft this card face up",
}

_KEYWORD_LINE = re.compile(r"^([a-z][a-z' \-]*?)(?=\s*(?:[,;.]|\{|\d|—|-|$))")


def is_keyword_only(text: str) -> bool:
    """True if the text consists entirely of recognized keywords with optional
    arguments (mana costs, numbers, "from X" clauses, dash-prefixed labels)."""
    s = text.strip().rstrip(".").lower()
    if not s:
        return True
    while s:
        m = _KEYWORD_LINE.match(s)
        if not m:
            return False
        head = m.group(1).strip()
        if head not in _KEYWORDS:
            # Try "ward N" / "ward {X}" / "protection from X" etc.
            base = head.split()[0]
            if base not in _KEYWORDS:
                return False
        # Eat the matched head + any optional argument (cost/number/from-clause)
        rest = s[m.end():]
        rest = re.sub(r"^\s*(?:\{[^}]+\}|\d+|from \w+(?:\s+(?:and|or)\s+\w+)*|[—-]\s*\w[\w \-]*)?",
                      "", rest)
        s = rest.lstrip(" ,;.")
    return True


# ============================================================================
# Effect patterns
# ============================================================================
# Each pattern: (regex, canonical_verb, modifier_template).
# Patterns are evaluated specific→general; first match wins per canonical verb,
# so "deals 3 damage to any target" claims `damage` and prevents the broader
# `damage_g` from also firing on the same card.
#
# `${N}` in modifier values pulls regex group N at match time.

EFFECT_PATTERNS: list[tuple[str, str, dict]] = [
    # ---- DAMAGE ----
    (r"deals (\d+) damage to any target",                   "damage",       {"target": "any",       "n": "${1}"}),
    (r"deals (\d+) damage to target creature",              "damage",       {"target": "creature",  "n": "${1}"}),
    (r"deals (\d+) damage to target player",                "damage",       {"target": "player",    "n": "${1}"}),
    (r"deals (\d+) damage to target opponent",              "damage",       {"target": "opponent",  "n": "${1}"}),
    (r"deals (\d+) damage to each opponent",                "damage",       {"target": "each_opp",  "n": "${1}"}),
    (r"deals (\d+) damage to each creature",                "damage",       {"target": "each_creature", "n": "${1}"}),
    (r"deals damage equal to [^.]+ to any target",          "damage",       {"target": "any",       "n": "var"}),
    (r"deals \d+ damage divided as you choose among",       "damage",       {"target": "split"}),

    # ---- DRAW / DISCARD / MILL ----
    (r"draw (\d+) cards?",                                  "draw",         {"n": "${1}"}),
    (r"\bdraw a card\b",                                    "draw",         {"n": "1"}),
    (r"target player draws (\d+) cards?",                   "force_draw",   {"n": "${1}"}),
    (r"each (?:opponent|player) draws (\d+) cards?",        "draw_each",    {"n": "${1}"}),
    (r"target player discards (\d+) cards?",                "force_discard",{"n": "${1}"}),
    (r"target player discards a card",                      "force_discard",{"n": "1"}),
    (r"discard (\d+) cards?",                               "self_discard", {"n": "${1}"}),
    (r"\bdiscard a card\b",                                 "self_discard", {"n": "1"}),
    (r"discard your hand",                                  "discard_hand", {}),
    (r"target player mills (\d+) cards?",                   "mill_target",  {"n": "${1}"}),
    (r"\bmill (\d+) cards?",                                "mill_self",    {"n": "${1}"}),
    (r"\bscry (\d+)",                                       "scry",         {"n": "${1}"}),
    (r"\bsurveil (\d+)",                                    "surveil",      {"n": "${1}"}),

    # ---- COUNTER (the spell) ----
    (r"counter target spell unless its controller pays",    "counter_spell", {"target": "any", "alt": "pay"}),
    (r"counter target creature spell",                      "counter_spell", {"target": "creature"}),
    (r"counter target noncreature spell",                   "counter_spell", {"target": "noncreature"}),
    (r"counter target instant or sorcery spell",            "counter_spell", {"target": "instant_sorcery"}),
    (r"counter target spell\b",                             "counter_spell", {"target": "any"}),

    # ---- REMOVAL ----
    (r"destroy target creature\b",                          "destroy",      {"target": "creature"}),
    (r"destroy target artifact\b",                          "destroy",      {"target": "artifact"}),
    (r"destroy target enchantment\b",                       "destroy",      {"target": "enchantment"}),
    (r"destroy target nonblack creature",                   "destroy",      {"target": "creature", "color_excl": "B"}),
    (r"destroy target permanent\b",                         "destroy",      {"target": "permanent"}),
    (r"destroy target land\b",                              "destroy",      {"target": "land"}),
    (r"destroy all creatures",                              "destroy_all",  {"target": "creatures"}),
    (r"destroy all (?:nonland )?permanents",                "destroy_all",  {"target": "permanents"}),
    (r"destroy each creature you don'?t control",           "destroy_others", {}),
    (r"destroy all lands",                                  "destroy_all",  {"target": "lands"}),
    (r"exile target creature\b",                            "exile",        {"target": "creature"}),
    (r"exile target nonland permanent",                     "exile",        {"target": "nonland_perm"}),
    (r"exile target permanent",                             "exile",        {"target": "permanent"}),
    (r"exile all creatures",                                "exile_all",    {"target": "creatures"}),
    (r"return target creature to its owner'?s? hand",       "bounce",       {"target": "creature"}),
    (r"return target nonland permanent to its owner'?s? hand", "bounce",    {"target": "nonland_perm"}),
    (r"return target permanent to its owner'?s? hand",      "bounce",       {"target": "permanent"}),

    # ---- TUTOR (library search) ----
    (r"search your library for a basic land card",          "tutor_land",   {"to": "hand"}),
    (r"search your library for an? (?:land|basic land) card", "tutor_land", {"to": "hand"}),
    (r"search your library for a creature card",            "tutor_creature", {"to": "hand"}),
    (r"search your library for a (?:n? )?(?:artifact|enchantment|instant|sorcery|planeswalker) card", "tutor_typed", {}),
    (r"search your library for a card",                     "tutor_any",    {"to": "hand"}),
    (r"search target opponent'?s? library for [^.]+",       "force_tutor",  {}),

    # ---- RECURSION (graveyard → other zone) ----
    (r"return target creature card from (?:your |a |any )?graveyard to your hand", "recursion", {"filter": "creature", "to": "hand"}),
    (r"return target creature card from (?:your |a |any )?graveyard to the battlefield", "reanimate", {"filter": "creature"}),
    (r"return target [^.]+ card from (?:your |a |any )?graveyard to (?:your hand|the battlefield)", "recursion", {}),
    (r"put target creature card from (?:your |a |any )?graveyard onto the battlefield", "reanimate", {"filter": "creature"}),
    (r"return ~ from (?:your |a |any )?graveyard to (?:your hand|the battlefield)", "self_recursion", {}),

    # ---- LIFE ----
    (r"you gain (\d+) life",                                "lifegain",     {"n": "${1}"}),
    (r"you gain life equal to [^.]+",                       "lifegain",     {"n": "var"}),
    (r"target player loses (\d+) life",                     "lifeloss_target", {"n": "${1}"}),
    (r"each opponent loses (\d+) life",                     "lifeloss_each",{"n": "${1}"}),
    (r"you lose (\d+) life",                                "lifeloss_self",{"n": "${1}"}),
    (r"you lose life equal to [^.]+",                       "lifeloss_self",{"n": "var"}),
    (r"your life total becomes [^.]+",                      "life_set",     {}),
    (r"if a (?:[^.]+) source would deal damage to you, prevent", "filtered_dmg_prevent", {}),
    (r"prevent (?:all|the next \d+|that) damage",           "prevent_damage", {}),

    # ---- MANA ----
    (r"\{t\}: add \{c\}\{c\}",                              "mana_ability", {"out": "2_C"}),
    (r"\{t\}: add \{[wubrgc]\}\{[wubrgc]\}",                "mana_ability", {"out": "2_color"}),
    (r"\{t\}: add \{[wubrgc]\}",                            "mana_ability", {"out": "1_color"}),
    (r"\{t\}: add one mana of any color",                   "mana_ability", {"out": "1_any"}),
    (r"\{t\}: add (?:two|three) mana",                      "mana_ability", {"out": "n_color"}),
    (r"add \{[^}]+\} for each [^.]+",                       "mana_for_each", {}),
    (r"add (?:that much|x|x mana) [^.]*",                   "mana_var",     {}),

    # ---- COUNTERS (the +1/+1 kind) ----
    (r"put a \+1/\+1 counter on",                           "counter_p1p1", {"n": "1"}),
    (r"put (\d+) \+1/\+1 counters? on",                     "counter_p1p1", {"n": "${1}"}),
    (r"put a -1/-1 counter on",                             "counter_m1m1", {"n": "1"}),
    (r"put (\d+) -1/-1 counters? on",                       "counter_m1m1", {"n": "${1}"}),
    (r"put a (\w+) counter on",                             "counter_typed",{"kind": "${1}"}),
    (r"put (\d+) [^.]* counters? on",                       "counter_other",{"n": "${1}"}),
    (r"distribute (?:a number of |x )?\+1/\+1 counters",    "distribute_counters", {}),

    # ---- BUFFS ----
    (r"target creature gets \+(\d+)/\+(\d+) until end of turn", "buff_target", {"p": "${1}", "t": "${2}"}),
    (r"~ gets \+(\d+)/\+(\d+) until end of turn",           "buff_self",    {"p": "${1}", "t": "${2}"}),
    (r"creatures you control get \+(\d+)/\+(\d+)",          "anthem",       {"p": "${1}", "t": "${2}"}),
    (r"target creature gets \+x/\+x",                       "buff_target_x", {}),
    (r"~ gets \+x/\+x",                                     "buff_self_x",  {}),
    (r"~ gets \+\d+/\+\d+ as long as",                      "conditional_buff_self", {}),
    (r"~'s? power (?:and toughness )?(?:is|are)? ?equal to", "calc_pt",     {}),

    # ---- SACRIFICE ----
    (r"sacrifice (?:a |an |another )?creature",             "sac",          {"type": "creature"}),
    (r"sacrifice (?:a |an )?artifact",                      "sac",          {"type": "artifact"}),
    (r"sacrifice (?:a |an )?(?:permanent|land|enchantment)", "sac",         {"type": "other"}),
    (r"target player sacrifices a creature",                "force_sac",    {"target": "creature"}),

    # ---- TOKEN CREATION ----
    (r"create (?:a|\d+) treasure tokens?",                  "treasure",     {}),
    (r"create (?:a|\d+) (?:food|clue|map|blood|powerstone|gold) tokens?", "utility_token", {}),
    (r"create (\d+) (\d+/\d+)[^.]*creature tokens?",        "token",        {"n": "${1}", "stats": "${2}"}),
    (r"create a token that's a copy of",                    "token_copy",   {}),
    (r"create (?:a|an) [^.]*creature token",                "token",        {}),
    (r"create (?:a|an|\d+) [^.]*token",                     "token_other",  {}),

    # ---- TRIGGERED ABILITIES (categorical — the *trigger* type, not the effect) ----
    (r"^when[^\.]+enters the battlefield",                  "trig_etb",     {}),
    (r"^when[^\.]+enters\b(?!\s+the battlefield)",          "trig_etb",     {}),
    (r"^when[^\.]+dies",                                    "trig_death",   {}),
    (r"^when[^\.]+leaves the battlefield",                  "trig_ltb",     {}),
    (r"^whenever[^\.]+attacks alone",                       "trig_attack_alone", {}),
    (r"^whenever[^\.]+attacks",                             "trig_attack",  {}),
    (r"^whenever[^\.]+blocks or becomes blocked",           "trig_block_either", {}),
    (r"^whenever[^\.]+blocks",                              "trig_block",   {}),
    (r"^whenever[^\.]+deals combat damage to a player",     "trig_combat_dmg_player", {}),
    (r"^whenever[^\.]+deals combat damage",                 "trig_combat_dmg", {}),
    (r"^whenever[^\.]+deals damage",                        "trig_damage",  {}),
    (r"^when you cast (?:this spell|~)",                    "trig_self_cast", {}),
    (r"^whenever you cast (?:a|an|the) [^.]+ spell",        "trig_cast_filtered", {}),
    (r"^whenever you cast",                                 "trig_cast_any", {}),
    (r"^whenever a player casts",                           "trig_player_cast", {}),
    (r"^at the beginning of your upkeep",                   "trig_upkeep",  {}),
    (r"^at the beginning of your end step",                 "trig_endstep", {}),
    (r"^at the beginning of (?:each |the next )?combat on your turn", "trig_combat_start", {}),
    (r"^at the beginning of (?:the |your )?(?:next |first )?(?:upkeep|end step|combat|draw step|untap step|main phase)", "trig_phase", {}),
    (r"^at the beginning of the end step of enchanted creature", "trig_aura_endstep", {}),
    (r"^whenever a (?:creature|permanent) you control [^.]+enters", "trig_ally_etb", {}),
    (r"^whenever a creature [^.]* dies",                    "trig_creature_dies", {}),
    (r"^whenever a [^.]+ is put into [^.]+ graveyard",      "trig_to_graveyard", {}),
    (r"^whenever a player plays a land",                    "trig_landfall_any", {}),
    (r"^whenever you play a land",                          "trig_landfall_you", {}),
    (r"^whenever (?:this )?vehicle attacks",                "trig_vehicle_attack", {}),
    (r"^when you cycle",                                    "trig_cycle",   {}),

    # ---- STATIC ABILITIES ----
    (r"~ has flying|~ has trample|~ has haste|~ has lifelink|~ has hexproof|~ has indestructible|~ has menace|~ has reach|~ has vigilance|~ has flash", "static_keyword_self", {}),
    (r"creatures you control have [^.]+",                   "static_creatures", {}),
    (r"other [^.]+ you control (?:have|get) [^.]+",         "tribal_anthem", {}),
    (r"as long as you control [^.]+",                       "as_long_as",   {}),
    (r"as long as [^.]+",                                   "conditional_static", {}),
    (r"~ enters tapped",                                    "etb_tapped",   {}),
    (r"this creature enters with [^.]+ counter",            "etb_with_counters", {}),
    (r"(?:~|this creature) can'?t be (?:countered|blocked|targeted)", "restriction", {}),
    (r"(?:~|this creature) can'?t (?:attack|block)",        "combat_restriction", {}),
    (r"(?:~|this creature) attacks each (?:turn|combat) (?:if able)?", "must_attack", {}),
    (r"(?:~|this creature) can block only [^.]+",           "block_only_filter", {}),

    # ---- COST MODIFIERS ----
    (r"costs? \{(\d+)\} less to cast",                      "cost_reduce",  {"n": "${1}"}),
    (r"costs? \{(\d+)\} more to cast",                      "cost_increase",{"n": "${1}"}),

    # ---- ALT COSTS ----
    (r"as an additional cost to cast (?:this spell|~), sacrifice", "addl_cost_sac", {}),
    (r"as an additional cost to cast (?:this spell|~), pay [^.]+",  "addl_cost_pay", {}),
    (r"as an additional cost to cast (?:this spell|~), discard [^.]+", "addl_cost_discard", {}),
    (r"as an additional cost to cast (?:this spell|~), reveal [^.]+",  "addl_cost_reveal", {}),
    (r"you may cast [^.]+ without paying its mana cost",    "free_cast",    {}),

    # ---- ACTIVATED ABILITIES (categorical — needs recursive AST in the engine but cluster as one for now) ----
    (r"\{t\}(?:, [^:]+)?: ",                                "act_tap",      {}),
    (r"\{[^}]+\}(?:, \{t\})?(?:, [^:]+)?: ",                "act_mana",     {}),

    # ---- LIBRARY MANIPULATION ----
    (r"reveal the top card of your library",                "reveal_top",   {}),
    (r"look at the top (\d+) cards? of your library",       "peek_top",     {"n": "${1}"}),
    (r"look at the top x cards? of your library",           "peek_top",     {"n": "x"}),
    (r"look at target player'?s? hand",                     "peek_hand",    {}),
    (r"target opponent reveals their hand",                 "reveal_opp_hand", {}),
    (r"shuffle your library",                               "shuffle",      {}),
    (r"the rest on the bottom of your library",             "library_bottom", {}),
    (r"you may look at the top card of your library",       "may_peek_top", {}),
    (r"you may play (?:lands|cards|spells) from the top of your library", "play_from_top", {}),

    # ---- COMBAT EFFECTS ----
    (r"target creature [^.]*fights? target creature",       "fight",        {}),
    (r"~ fights target creature",                           "fight_self",   {}),
    (r"deals combat damage as though it weren'?t blocked",  "damage_unblocked", {}),
    (r"assigns combat damage [^.]+ rather than [^.]+",      "damage_assign_other", {}),

    # ---- COPY / CONTROL ----
    (r"copy target (?:instant|sorcery|spell)",              "copy_spell",   {}),
    (r"create a token that'?s? a copy of",                  "token_copy",   {}),
    (r"gain control of target creature",                    "control_creature", {}),
    (r"gain control of target permanent",                   "control_permanent", {}),

    # ---- AURA / EQUIPMENT static ----
    (r"enchanted creature gets \+(\d+)/\+(\d+)",            "aura_buff",    {"p": "${1}", "t": "${2}"}),
    (r"enchanted creature has",                             "aura_keyword", {}),
    (r"equipped creature gets \+(\d+)/\+(\d+)",             "equip_buff",   {"p": "${1}", "t": "${2}"}),
    (r"equipped creature has",                              "equip_keyword", {}),
    (r"becomes attached to",                                "becomes_attached", {}),

    # ---- MODAL / CHOICE ----
    (r"choose one (?:[—-])",                                "modal_one",    {}),
    (r"choose two (?:[—-])",                                "modal_two",    {}),
    (r"choose (?:one or both|two or more)",                 "modal_more",   {}),
    (r"choose odd or even",                                 "choose_parity", {}),

    # ---- RANDOMIZATION ----
    (r"flip (?:a coin|coins)",                              "coin_flip",    {}),
    (r"roll (?:a |two |\d+ )?(?:six-sided )?(?:dice|die|d\d+)", "dice_roll", {}),
    (r"choose [^.]+ at random",                             "random_choice", {}),

    # ---- EXTRA TURNS / COMBAT ----
    (r"take an? extra turn",                                "extra_turn",   {}),
    (r"there'?s? an additional combat (?:phase|step)",      "extra_combat", {}),

    # ---- PHASE/STEP MANIPULATION ----
    (r"skip your (?:next |first |draw |untap |combat |end |main )?(?:step|phase|turn)", "skip_step", {}),
    (r"(?:[^.]+ )?don'?t untap during (?:its controller'?s?|your) [^.]+", "no_untap", {}),

    # ---- AURA TRIGGERED ----
    (r"^at the beginning of the end step of enchanted creature'?s? controller", "aura_eot_trigger", {}),

    # ---- ENERGY ----
    (r"you get \{e\}\{e\}",                                 "get_energy",   {}),
    (r"you get (\d+) \{e\}",                                "get_energy",   {"n": "${1}"}),
    (r"pay \{e\}",                                          "spend_energy", {}),

    # ---- CONJURE / ARENA-SPECIFIC ----
    (r"conjure (?:a |\d+ )?cards? named",                   "conjure",      {}),
    (r"perpetually [^.]+",                                  "perpetually",  {}),
    (r"seek a [^.]+",                                       "seek",         {}),

    # ---- RECONFIGURE / EXPLORE / OTHER 2020+ ----
    (r"reconfigure \{[^}]+\}",                              "reconfigure",  {}),
    (r"\bexplores?\b",                                      "explore",      {}),
    (r"\binvestigate\b",                                    "investigate",  {}),

    # ---- WIN/LOSE ----
    (r"you win the game",                                   "win_game",     {}),
    (r"target player loses the game",                       "make_lose",    {}),
    (r"you lose the game",                                  "self_lose",    {}),

    # ---- FLICKER / EXILE-RETURN ----
    (r"exile target [^.]+, then return (?:that card|it) to the battlefield", "flicker_target", {}),
    (r"exile [^.]+ then return (?:those cards|it) to the battlefield", "flicker", {}),
    (r"exile (?:~|this creature)[^.]*at the beginning of (?:the )?next end step", "exile_until_eot", {}),
]


# ============================================================================
# Normalization
# ============================================================================

def normalize_text(card: dict) -> str:
    """Pull oracle text (concatenating dual-faced card_faces if needed),
    lowercase, strip reminder text, normalize dashes/whitespace, and replace
    the card's name (and each face name) with `~`."""
    text = card.get("oracle_text") or ""
    if not text and card.get("card_faces"):
        text = "\n".join(f.get("oracle_text") or "" for f in card["card_faces"])
    text = text.lower()
    name = (card.get("name") or "").lower()
    if name:
        text = text.replace(name, "~")
        for half in name.split(" // "):
            text = text.replace(half, "~")
        first = name.split(",")[0].split(" ")[0]
        if first and len(first) > 3:
            text = text.replace(first, "~")
    text = re.sub(r"\([^)]*\)", "", text)            # strip reminder text
    text = re.sub(r"[\u2013\u2014]", "-", text)      # normalize en/em dashes
    text = expand_keywords(text)                     # shorthand → canonical
    text = re.sub(r"\s+", " ", text).strip()
    return text


# ============================================================================
# Effect extraction
# ============================================================================

def extract_effects(card: dict) -> tuple:
    """Return a frozen, hashable signature: a sorted tuple of (verb, modifiers)
    pairs. Two cards with identical signatures share a handler.

    Special signatures:
      (("KEYWORDS_ONLY", ()),) — vanilla / pure-keyword cards
      (("UNPARSED", ()),)      — no patterns matched (custom-handler queue)
    """
    text = normalize_text(card)
    if not text:
        return (("KEYWORDS_ONLY", ()),)
    if is_keyword_only(text):
        return (("KEYWORDS_ONLY", ()),)

    # First-match-wins per canonical verb: collect verb matches in order, dedupe.
    seen_verbs = {}
    for pat, verb, mod_template in EFFECT_PATTERNS:
        if verb in seen_verbs:
            continue
        m = re.search(pat, text, re.I)
        if not m:
            continue
        mods = {}
        for k, v in mod_template.items():
            if isinstance(v, str) and v.startswith("${") and v.endswith("}"):
                idx = int(v[2:-1])
                try:
                    mods[k] = m.group(idx)
                except IndexError:
                    mods[k] = v
            else:
                mods[k] = v
        seen_verbs[verb] = tuple(sorted(mods.items()))

    if not seen_verbs:
        return (("UNPARSED", ()),)
    return tuple(sorted(seen_verbs.items()))


# ============================================================================
# Report
# ============================================================================

def render_signature(sig: tuple) -> str:
    if not sig:
        return "(empty)"
    parts = []
    for verb, mods in sig:
        if mods:
            mod_str = ",".join(f"{k}={v}" for k, v in mods)
            parts.append(f"`{verb}({mod_str})`")
        else:
            parts.append(f"`{verb}`")
    return " + ".join(parts)


def render_report(total: int, clusters: dict[tuple, list[str]]) -> str:
    sorted_clusters = sorted(clusters.items(), key=lambda x: -len(x[1]))
    n_clusters = len(clusters)
    keywords_only = len(clusters.get((("KEYWORDS_ONLY", ()),), []))
    unparsed = len(clusters.get((("UNPARSED", ()),), []))
    singletons = sum(1 for v in clusters.values() if len(v) == 1)
    singleton_cards = sum(len(v) for v in clusters.values() if len(v) == 1)

    cumulative_cards = []
    running = 0
    for _, cards in sorted_clusters:
        running += len(cards)
        cumulative_cards.append(running)

    lines = [
        "# Semantic Cluster Report",
        "",
        f"Pool: **{total:,} real cards** (filtered: tokens, schemes, planes, silver-bordered, bios).",
        "",
        "## Headline",
        "",
        f"- **Unique semantic clusters: {n_clusters:,}**",
        f"- KEYWORDS_ONLY (vanilla + pure keyword cards, single canonical handler): **{keywords_only:,}** ({pct(keywords_only, total)})",
        f"- UNPARSED (effects the analyzer can't bucket → custom-handler candidates): **{unparsed:,}** ({pct(unparsed, total)})",
        f"- Singletons (clusters of one — truly unique effect signatures): **{singletons:,}** clusters / {singleton_cards:,} cards ({pct(singleton_cards, total)})",
        "",
        "## Engine-build curve",
        "",
        "| Handlers built | Cards covered | % of pool |",
        "|---:|---:|---:|",
    ]
    for n in (10, 30, 50, 100, 200, 500, 1000, 2000):
        if n <= len(sorted_clusters):
            covered = cumulative_cards[n - 1]
            lines.append(f"| Top {n:,} | {covered:,} | {pct(covered, total)} |")

    lines += [
        "",
        "## Top 30 clusters (highest-ROI handlers to build first)",
        "",
        "| # | Cards | Effect signature | Sample cards |",
        "|---:|---:|---|---|",
    ]
    for i, (sig, cards) in enumerate(sorted_clusters[:30], start=1):
        sample = ", ".join(cards[:3])
        if len(cards) > 3:
            sample += f", … +{len(cards) - 3}"
        lines.append(f"| {i} | {len(cards):,} | {render_signature(sig)} | {sample} |")

    lines += [
        "",
        "## Cluster size distribution",
        "",
        "| Cluster size | Number of clusters | Cards covered |",
        "|---:|---:|---:|",
    ]
    bins = [(1, 1), (2, 5), (6, 20), (21, 100), (101, 500), (501, 10**9)]
    for lo, hi in bins:
        clust_in = sum(1 for v in clusters.values() if lo <= len(v) <= hi)
        cards_in = sum(len(v) for v in clusters.values() if lo <= len(v) <= hi)
        if clust_in == 0:
            continue
        label = f"{lo}" if lo == hi else f"{lo:,}-{hi:,}" if hi < 10**8 else f"{lo:,}+"
        lines.append(f"| {label} | {clust_in:,} | {cards_in:,} |")

    lines += [
        "",
        "## What this means",
        "",
        f"To get every printed card to GREEN, the engine needs **{n_clusters - 1:,} effect handlers** "
        "(minus the UNPARSED bucket, which still needs analyzer iteration before it's actionable).",
        "",
        f"But the distribution is heavy-tailed — the top 30 handlers cover **{pct(cumulative_cards[29], total)}** of the pool, "
        f"and the top 200 cover **{pct(cumulative_cards[min(199, len(sorted_clusters)-1)], total)}**.",
        "",
        f"The {singletons:,} singleton clusters are the real custom-handler queue. Most of those are jank that won't see play; "
        "the EDHRec play-rate weighting pass (next step, pending green light) will rank them by competitive relevance so we "
        "build handlers in deck-impact order rather than alphabetically.",
        "",
    ]
    return "\n".join(lines)


def pct(n: int, total: int) -> str:
    return f"{100 * n / total:.1f}%"


# ============================================================================
# Main
# ============================================================================

def main() -> None:
    cards = json.loads(ORACLE_DUMP.read_text())
    real = [c for c in cards if is_real_card(c)]

    clusters: dict[tuple, list[str]] = defaultdict(list)
    for c in real:
        clusters[extract_effects(c)].append(c["name"])

    report = render_report(len(real), clusters)
    REPORT.write_text(report)

    # Console summary
    sorted_clusters = sorted(clusters.items(), key=lambda x: -len(x[1]))
    print(f"\n{'═' * 60}")
    print(f"  Semantic clusters across {len(real):,} cards")
    print(f"{'═' * 60}")
    print(f"  unique clusters: {len(clusters):,}")
    print(f"  KEYWORDS_ONLY:   {len(clusters.get((('KEYWORDS_ONLY', ()),),[])):,}")
    print(f"  UNPARSED:        {len(clusters.get((('UNPARSED', ()),),[])):,}")
    print(f"  singletons:      {sum(1 for v in clusters.values() if len(v) == 1):,}")
    print(f"\n  top 10:")
    for sig, cards_list in sorted_clusters[:10]:
        print(f"    {len(cards_list):>5,}  {render_signature(sig)[:80]}")
    print(f"\n  build curve:")
    cum = 0
    cum_at = {}
    for i, (_, c) in enumerate(sorted_clusters, 1):
        cum += len(c)
        cum_at[i] = cum
    for n in (10, 30, 100, 500, 1000):
        if n in cum_at:
            print(f"    top {n:>4,} handlers → {cum_at[n]:>6,} cards ({pct(cum_at[n], len(real))})")
    print(f"\n  → {REPORT}")


if __name__ == "__main__":
    main()
