#!/usr/bin/env python3
"""Era 4 (2023-2026) scaffold-gap audit.

Mirrors scripts/era1_scaffold_audit.py but filters for cards classified as
era 4 by classifyCardEra (discover, descend, battles, prototype, craft,
role token, finality counter, the ring).

Output: data/rules/era4_scaffold_audit.md  +  prints top-50 to stdout.
"""

from __future__ import annotations

import json
import re
from collections import Counter, defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DATASET = ROOT / "data" / "rules" / "ast_dataset.jsonl"
OUT = ROOT / "data" / "rules" / "era4_scaffold_audit.md"

ERA4_KW = ["discover", "descend", "battle", "prototype", "craft",
           "role token", "finality counter", "the ring"]
ERA3_KW = ["daybound", "nightbound", "disturb", "cleave", "decayed",
           "exploit", "companion", "mutate", "foretell", "learn",
           "ward", "perpetual", "conjure"]
ERA2_KW = ["partner", "experience counter", "eminence", "energy counter",
           "crew", "adapt", "amass", "afterlife", "spectacle", "riot"]

TARGET_ERA = 4


def classify_era(oracle_text: str, type_line: str) -> int:
    text = (oracle_text or "").lower()
    types = (type_line or "").lower()
    for kw in ERA4_KW:
        if kw in text or kw in types:
            return 4
    for kw in ERA3_KW:
        if kw in text:
            return 3
    for kw in ERA2_KW:
        if kw in text:
            return 2
    return 1


BUCKETED_KINDS = {
    "fateful_hour", "life_threshold",
    "threshold", "card_count_zone",
    "metalcraft", "morbid", "ferocious", "revolt", "delirium",
    "devotion",
    "you_attacked_this_turn",
    "you_control",
    "paid_optional_cost", "for_each",
    "etb_as", "enters_as", "enters_with",
    "did_prior_action",
    "mana_spent",
    "was_kicked",
    "hellbent", "raid", "attacked_this_turn",
    "spell_mastery", "gained_life_this_turn", "creature_died_this_turn",
    "no_spells_cast_last_turn", "two_plus_spells_cast_last_turn",
    "you_control_creature_power_ge",
    "etb_tapped_unless", "domain", "etb_if", "repeat_n",
    "lieutenant", "ki_counters_ge_2", "self_is_tapped",
    "attacked_or_blocked_this_combat", "coven", "self_has_counter",
    "didnt_attack_this_turn", "dealt_damage_to_opponent_this_turn",
    "no_mana_spent_to_cast",
    # Era 4 audit additions (dev/era2-era4-scaffolds branch).
    "landfall", "you_descended_this_turn",
    "it_was_a_creature", "no_creatures_on_battlefield",
    # Era 1 R60 sweep — also valid as bucketed Kinds when emitted directly.
    "tribute_not_paid", "tribute_wasnt_paid", "bargained", "was_bargained",
    "shares_creature_type", "not_their_turn", "not_your_turn",
    "more_life_than_opponent", "you_more_life", "opponent_more_life",
    "no_time_counters", "time_counter_on_self", "wasnt_cast",
    "lost_life_last_turn", "damage_dealt_to_self_this_turn",
    "damage_taken_this_turn",
    "more_cards_in_hand_than_opponents", "more_cards_than_each_opponent",
    "self_is_untapped", "self_has_no_counter", "no_named_counter",
    "surge_cost_paid", "library_empty", "colored_mana_spent",
    "self_is_suspended", "life_above_starting", "wasnt_blocking",
    # Era 4 R60 sweep — 19 new structured Kinds + synonym variants.
    "opponent_more_creatures", "opp_more_creatures",
    "wasnt_creature_type", "wasnt_subtype",
    "all_lands_type", "all_lands_subtype",
    "was_attacking",
    "permanent_mv_le", "permanent_mana_value_le",
    "was_cast", "it_was_cast",
    "didnt_die",
    "self_didnt_etb_this_turn", "didnt_enter_this_turn",
    "no_named_tokens", "wasnt_sacrificed",
    "counters_on_self_ge", "self_counter_threshold",
    "no_damage_since_last_turn", "first_time_resolved_this_turn",
    "player_no_creatures", "another_typed_etb_this_turn",
    "creature_etb_last_turn", "exiled_with_count_ge",
    "exiled_this_turn", "damaged_creature_died",
    # Era 4 r60 long-tail close-out (this branch): two structured Kinds that
    # the original sweep left unbucketed because they're 1-of cards each
    # (Anya's life_delta_threshold, Bhaal/Myrkul/Bane's life_vs_half_starting).
    # Wired in conditional_setup.go as aliases to LifeAboveStarting (delta)
    # and a new LifeBelowHalfStarting scaffold (Bhaal/Myrkul/Bane gate).
    "life_delta_threshold", "life_vs_half_starting", "life_below_half_starting",
}

RAW_KINDS = {"intervening_if", "as_long_as", "conditional", "raw", "if"}

# Era 4-flavored raw patterns: discover, descend, battle defeated, prototype,
# craft, role tokens, the ring, finality counters, plus the carry-over
# evergreens.
RAW_PATTERNS = [
    ("discover_n", re.compile(r"discover \d")),
    ("descended_turn", re.compile(r"descend(?:ed)? this turn|fully descended|you have descend")),
    ("battle_defeated", re.compile(r"defeated (?:this turn|a battle)|battle.*defeated|defeat (?:a )?battle")),
    ("battle_attack", re.compile(r"attacks? (?:a battle|battle)")),
    ("prototype_cast", re.compile(r"prototype|cast .* for its prototype")),
    ("craft_with", re.compile(r"craft with|crafted")),
    ("role_token", re.compile(r"role token|monster role|wicked role|young hero|virtuous role|sorcerer role|cursed role|royal role")),
    ("the_ring_tempts", re.compile(r"the ring tempts you|ring-bearer|your ring-bearer")),
    ("finality_counter", re.compile(r"finality counter")),
    ("bargain_perm", re.compile(r"bargain|you may sacrifice an artifact, enchantment, or token")),
    ("celebration_perm_etb", re.compile(r"celebration|two or more nonland permanents")),
    ("commit_a_crime", re.compile(r"commit(?:s|ted)? a crime|committed a crime this turn")),
    ("collect_evidence", re.compile(r"collect evidence \d|collected evidence")),
    ("plot_exile", re.compile(r"plot|cast it without paying its mana cost.*plot")),
    ("disguise_facedown", re.compile(r"disguise|face-down|turn.*face up.*for its disguise")),
    ("cloak_facedown", re.compile(r"cloak|cloaked|the top card.*face down")),
    ("offspring_token", re.compile(r"offspring|1/1 copy")),
    ("freerunning", re.compile(r"freerunning|its freerunning cost")),
    ("expend_n", re.compile(r"expend \d")),
    ("eerie_room", re.compile(r"eerie|whenever an enchantment.*enters")),
    ("survival_tapped", re.compile(r"survival|untapped creature you control")),
    ("flurry_second_spell", re.compile(r"flurry|second spell each turn")),
    ("manifest_dread", re.compile(r"manifest dread")),
    ("warp_cast", re.compile(r"warp|its warp cost")),
    # Era 1 R60 sweep additions — bucket the raw fragments emitted by the
    # Era 1 scaffold matchers so the unbucketed list reflects post-R60 reality.
    ("tribute_not_paid", re.compile(r"tribute (?:wasn't|was) paid")),
    ("bargained_raw", re.compile(r"(?:it was|this spell was) bargained|if it was bargained")),
    ("shares_creature_type", re.compile(r"shares a creature type")),
    ("not_their_turn", re.compile(r"(?:not their turn|not your turn|isn't their turn|during another player's turn|on a turn that isn't yours)")),
    ("opponent_more_life", re.compile(r"opponent has more life than you|any opponent has more life|an opponent has more life|opponent.*more life than you")),
    ("more_life_than_opponent", re.compile(r"more life than (?:an|each) opponent|no opponent has more life than (?:that player|you)")),
    ("time_counter_self", re.compile(r"(?:had )?no time counters|no time counters on it")),
    ("self_is_suspended", re.compile(r"this card is suspended|while (?:it's )?suspended")),
    ("wasnt_cast", re.compile(r"(?:it|he|she|they) (?:wasn't|weren't) cast|wasn't cast")),
    ("wasnt_blocking", re.compile(r"(?:it )?wasn't blocking|wasn't being blocked by")),
    ("surge_cost_paid", re.compile(r"surge cost was paid")),
    ("library_empty", re.compile(r"no cards in.*library|library has no cards|library is empty")),
    ("colored_mana_spent", re.compile(r"(?:\{[wubrgc]\}.*was spent to cast|spent to cast it.*\{[wubrgc]\}|mana from a treasure was spent)")),
    ("lost_life_last_turn", re.compile(r"(?:lost|lose) life last turn|last turn.*(?:lost|lose) life")),
    ("damage_dealt_to_self_turn", re.compile(r"damage (?:was )?dealt to (?:it|this).*this turn")),
    ("more_cards_than_opponents", re.compile(r"more cards in.*hand.*than (?:each|any) opponent")),
    ("self_is_untapped", re.compile(r"(?:this creature|this artifact|this permanent|~) is untapped")),
    ("self_has_no_counter", re.compile(r"(?:has|had) no [a-z\+\-/0-9]+ counters? on it")),
    ("life_above_starting", re.compile(r"(?:greater|more|above) than your starting life total")),
    # Era 4 R60 sweep raw additions — bucket the 19 new patterns.
    ("opponent_more_creatures", re.compile(r"more creatures than you|controls more creatures.*than you")),
    ("opponent_more_lands", re.compile(r"more lands than you|controls more lands.*than you|opponent controls more lands")),
    ("wasnt_creature_type", re.compile(r"(?:it|he|she) wasn't (?:a|an) (?!creature\b)[a-z]+")),
    ("all_lands_type", re.compile(r"all lands on the battlefield are |every land on the battlefield is a |each land on the battlefield is a ")),
    ("was_attacking", re.compile(r"(?:it|~) was attacking|while it was attacking")),
    ("permanent_mv_le", re.compile(r"permanent card (?:with|whose) mana value.*(?:or less|or fewer)")),
    ("was_cast_positive", re.compile(r"(?:it|he|she|they) (?:was|were) cast(?!\sfrom)")),
    ("didnt_die", re.compile(r"(?:it )?didn't die")),
    ("self_didnt_etb_turn", re.compile(r"didn't enter the battlefield this turn|didn't enter this turn|hasn't entered the battlefield this turn|wasn't put onto the battlefield this turn")),
    ("no_named_tokens", re.compile(r"(?:there are no|if no) [a-z]+ tokens on the battlefield")),
    ("wasnt_sacrificed", re.compile(r"(?:it )?wasn't sacrificed")),
    ("counters_on_self_ge", re.compile(r"(?:it|this creature|this permanent|~) has [a-z0-9]+ or more [a-z\+\-/0-9]+ counters? on it|has (?:three|three or more|two or more|four or more) [a-z\+\-/0-9]+ counters|there are (?:two|three|four|five|six|seven|eight|nine|ten|\d+) or more [a-z\+\-/0-9]+ counters on (?:it|this)")),
    ("it_was_a_creature_raw", re.compile(r"^it was a creature\b|, it was a creature\b|if it was a creature\b")),
    ("still_on_battlefield", re.compile(r"(?:it's|is still|this (?:creature|enchantment|permanent|artifact) is) on the battlefield")),
    ("no_damage_since_last_turn", re.compile(r"haven't been dealt.*damage.*since (?:your|the) last turn")),
    ("first_time_resolved_turn", re.compile(r"first time this ability has resolved")),
    ("player_no_creatures", re.compile(r"(?:a|any) player controls no creatures|controls no creatures")),
    ("another_typed_etb_turn", re.compile(r"another [a-z]+ (?:entered|enters) the battlefield.*(?:under your control|you control).*this turn")),
    ("creature_etb_last_turn", re.compile(r"creature (?:enter|entered).*(?:under your control|you control).*last turn")),
    ("exiled_with_count_ge", re.compile(r"cards (?:have been )?exiled with (?:this|~)|cards exiled with this")),
    ("exiled_this_turn", re.compile(r"cards? (?:were|was) put into exile this turn|(?:were|was) exiled this turn|one or more cards.*exile.*this turn")),
    ("damaged_creature_died", re.compile(r"creature dealt damage by (?:this creature|~).*this turn.*(?:died|would die)")),
    # Era 4 scaffold additions (dev/era2-era4-scaffolds branch).
    ("planeswalker_etb_turn", re.compile(r"planeswalker (?:entered|enters) the battlefield.*this turn|planeswalker.*you've cast.*this turn")),
    ("artifact_etb_turn", re.compile(r"artifact (?:entered|enters) the battlefield(?:.*this turn.*(?:under your control|you control)|.*(?:under your control|you control).*this turn)")),
    ("reveal_land_otherwise_hand", re.compile(r"if it's a land card.*(?:onto the battlefield|put it onto).*otherwise")),
    ("had_counters_on_it", re.compile(r"had (?:a |one or more |any |a \+1/\+1 |a death )?counters? on it")),
    ("you_cast_from_hand", re.compile(r"you cast it from your hand|you cast it from a graveyard|you cast it(?! from)")),
    ("hellbent_no_hand", re.compile(r"hellbent|no cards in.*hand")),
    ("monarch", re.compile(r"the monarch|you('re| are) the monarch")),
    ("initiative", re.compile(r"the initiative|have initiative")),
    ("revolt_perm_left", re.compile(r"revolt|permanent.*left the battlefield.*this turn")),
    ("delirium_types", re.compile(r"delirium|four or more card types")),
    ("metalcraft_three_art", re.compile(r"metalcraft|three or more artifacts")),
    ("ferocious_power4", re.compile(r"ferocious|creature with power (?:four|4)")),
    ("formidable", re.compile(r"formidable|total power (?:eight|8)")),
    ("kicker_paid", re.compile(r"kicked|kicker (?:cost )?was paid|multikicker")),
    ("gained_life_turn", re.compile(r"(?:gained|gain) life.*this turn")),
    ("life_lost_turn", re.compile(r"(?:lost|lose) life.*this turn")),
    ("opponent_lost_life", re.compile(r"opponent.*(?:lost life|lose life|dealt damage).*this turn")),
    ("life_above", re.compile(r"(?:you have|your life total is) \d+ or more life")),
    ("life_below", re.compile(r"(?:you have|your life total is) \d+ or (?:less|fewer) life")),
    ("cast_spell_turn", re.compile(r"(?:cast a spell|cast a noncreature|cast an instant|you cast).*this turn")),
    ("second_spell_turn", re.compile(r"(?:second|third) (?:spell|creature|noncreature|instant|sorcery|artifact|enchantment)")),
    ("creature_etb_turn", re.compile(r"creature (?:entered|enters).*(?:this turn|battlefield)")),
    ("drawn_card_turn", re.compile(r"(?:drew|drawn|draw) a card.*this turn")),
    ("attacked_turn", re.compile(r"attacked this turn|you attacked|creature attacked")),
    ("sacrificed_turn", re.compile(r"sacrific.*this turn")),
    ("combat_damage", re.compile(r"combat damage.*(?:this turn|to a player|dealt)")),
    ("landfall_turn", re.compile(r"landfall|land.*entered|played a land.*this turn")),
    ("discarded_turn", re.compile(r"discard.*this turn")),
    ("enchanted_creature", re.compile(r"enchanted creature")),
    ("equipped_creature", re.compile(r"equipped creature")),
    ("any_player_phase", re.compile(r"(?:each player|each opponent).*(?:upkeep|end step)")),
    ("upkeep", re.compile(r"upkeep")),
    ("died_this_turn", re.compile(r"died this turn")),
    ("graveyard_creatures", re.compile(r"graveyard.*creature card|creatures in.*graveyard")),
    ("graveyard_card", re.compile(r"graveyard")),
    ("mana_spent_raw", re.compile(r"mana was (?:spent|paid)|amount of mana (?:spent|paid)|mana value of \S+ is \d+ or (?:greater|more)")),
    ("permanent_left_bf", re.compile(r"permanent left")),
    ("becomes_tapped", re.compile(r"becomes tapped|is tapped")),
    ("becomes_target", re.compile(r"becomes (?:the|a) target")),
    ("tokens_created", re.compile(r"tokens.*created this turn")),
    # Parser-coverage R60 batch — named-counter threshold cluster.
    # Mirror of the era1 RAW_PATTERNS addition; same engine coverage
    # (condScaffoldCountersOnSelfGE at conditional_setup.go:2573).
    ("named_counter_threshold_on_perm",
     re.compile(r"(?:\d+|one|two|three|four|five|six|seven|eight|nine|ten|thirteen|twenty) or more \w+ counters? on (?:it|this artifact|this enchantment|this creature|this permanent|them|that)|there are (?:\d+|one|two|three|four|five|six|seven|eight|nine|ten) or more \w+ counters? on (?:it|this)")),
    # Era 4 r60 condition-longtail — mirror the detectConditionScaffold
    # batch in cmd/hexdek-thor/conditional_setup.go. Ordered most-specific
    # first; placed AHEAD of you_control_raw so the broad fallback doesn't
    # eat narrower predicates.
    ("battlefield_count_typed", re.compile(r"there are (?:\d+|two|three|four|five|six|seven|eight|nine|ten) or more (?:other )?(?:creatures|lands|artifacts|enchantments|planeswalkers|permanents|zombies) (?:on|still on) the battlefield|there are no (?:zombies|creatures|lands|artifacts|enchantments|planeswalkers) on the battlefield|a creature named [\w\-' ]+ is on the battlefield")),
    ("chosen_permanent_still_on_bf", re.compile(r"one or more of the chosen permanents are still on the battlefield")),
    ("that_card_on_battlefield", re.compile(r"^that card is on the battlefield|, that card is on the battlefield")),
    ("card_exiled_with_counter", re.compile(r"this card is exiled with (?:a|an) \w+ counter on it")),
    ("enchanted_target_is_type", re.compile(r"enchanted permanent is a (?:creature|artifact|enchantment|land|planeswalker)|enchanted land is a (?:basic )?(?:mountain|island|forest|swamp|plains)|enchanted permanent is a creature with the greatest power")),
    ("enchantment_remains_on_bf", re.compile(r"this enchantment remains on the battlefield")),
    ("card_type_reveal_route_broad", re.compile(r"(?:if it's a|if it's an|if that card is a|if that card is an|if a card with the chosen name was|if you reveal a card named) [\w\-]+(?: card)?.*(?:put it onto the battlefield|put that card)|if you searched for a creature card that doesn't")),
    ("mana_value_vs_state", re.compile(r"mana value is less than or equal to the number of|if it has mana value \w+ or (?:less|fewer)|its mana value is \w+ or (?:less|fewer)")),
    ("cast_n_instant_or_sorcery_turn", re.compile(r"(?:you've|you have) cast (?:\d+|two|three|four|five) or more (?:instant (?:and|or) sorcery|instant or sorcery) spells this turn")),
    ("another_creature_etb_turn", re.compile(r"had another creature enter the battlefield (?:under your control )?this turn|(?:\d+|two|three|four|five) or more creatures (?:entered|enter) the battlefield (?:under your control )?this turn")),
    ("permanent_to_hand_turn", re.compile(r"a permanent was put into your hand from the battlefield this turn")),
    ("creature_attacked_with_friends", re.compile(r"at least (?:\d+|one|two|three|four|five) other creatures? attacked")),
    ("self_entered_this_turn", re.compile(r"^~ entered this turn|, ~ entered this turn")),
    ("self_not_on_battlefield", re.compile(r"~ isn'?t on the battlefield|this card isn'?t on the battlefield")),
    ("self_hasnt_dealt_damage", re.compile(r"hasn'?t dealt damage yet")),
    ("self_paired_with", re.compile(r"~ is paired with|is paired with another creature")),
    ("self_top_of_library_state", re.compile(r"this card is the top card of your library")),
    ("self_card_exiled_bare", re.compile(r"^this card is exiled$|^this card is exiled[,.]|, this card is exiled[,.]")),
    ("self_power_ge_possessive", re.compile(r"as long as ~'s power is \w+ or (?:greater|more)")),
    ("self_is_attacking_era4", re.compile(r"as long as ~ is attacking")),
    ("counters_exactly_n_on_self", re.compile(r"exactly (?:one|two|three|four|five|\d+) \w+ counters? on (?:this enchantment|this artifact|this creature|this permanent|it)")),
    ("first_end_step_turn", re.compile(r"first end step of the turn|first end step")),
    ("not_main_phase", re.compile(r"isn'?t your main phase|not your main phase")),
    ("opening_hand_not_starting", re.compile(r"in your opening hand and you'?re not the starting player")),
    ("is_historic", re.compile(r"\bit was historic\b|\bis historic\b")),
    ("a_creature_died_under_control", re.compile(r"a creature died under your control this turn")),
    ("you_control_raw", re.compile(r"you control")),
]


def match_raw(text: str) -> str | None:
    t = text.lower()
    for name, rx in RAW_PATTERNS:
        if rx.search(t):
            return name
    return None


# Parser-coverage R60 batch — Trigger-side bucketing for Era 4.
# Era 4 historically only counted total trigger nodes and never split
# bucketed/unbucketed (Era 1/2/3 do). Adding parity so the cross-era
# gap metric reflects the engine's classifyTrigger coverage of the
# 2023+ slug surface (specialize_from_zone, ring_tempts_you, the
# *_to_gy_from_bf family, etc.). Sets mirror the era1 audit's
# TRIGGER_EVENT_EXACT / TRIGGER_EXTRA_EXACT / TRIGGER_SUBSTRING_CATCHES.
TRIGGER_EVENT_EXACT = {
    "when_you_do", "counters_put_on_self", "tribe_you_control_etb",
    "self_crews_vehicle", "you_whenever", "ally_etb", "ally_typed_etb_a",
    "ally_subtype_deal_damage", "self_and", "etb_or_another",
    "opp_creature_event", "self_saddles_mount", "becomes_tapped",
    "you_get_energy", "player_wins_coin_flip", "you_action",
    "becomes_crewed", "becomes_crewed_first", "opp_draw_card",
    "etb", "attacks", "deal_combat_damage", "deals_combat_damage", "phase",
    "mutates", "turned_face_up", "exploits_creature",
    "specialize_creature", "unlock_door",
    "becomes_untapped", "becomes_monstrous", "tapped_for_mana", "you_roll_dice",
}

TRIGGER_SUBSTRING_CATCHES = [
    "dies", "is put into a graveyard",
    "enters", "attack", "combat damage", "combat_damage",
    "cast", "spell", "gain", "lose",
    "draw", "discard", "leaves", "ltb", "sacrific",
]

TRIGGER_EXTRA_EXACT = {
    "die", "to_graveyard", "etb_as", "cycle", "block",
    "coin_flip_result", "lose_game",
    "beginning_of_ordinal_step", "token_event",
    "nontoken_ally_event", "nontoken_creature_event",
    "compound_opp_tribe_event", "one_or_more_typed_event",
    "ally_explore", "self_and_another",
    "conditional_state", "misc_when", "spend_this_mana",
    "face_up_as", "as_transform", "ally_exploits", "fully_unlock_room",
    "ally_targeted_by_opp", "becomes_blocked",
    "dealt_damage", "deals_damage",
    "damage_prevented_this_way", "ally_source_damage",
    "remove_counter", "counter_put_on_actor",
    "counters_put_on_self_any", "counters_put_on_actor",
    "creature_modified_event",
    "card_put_into_zone", "permanent_to_gy",
    "card_milled_via", "compound_card_zone_event",
    "foretell_card", "attached_as", "equipped_trigger",
    "day_night_flip", "transforms",
    "next_time_one_or_more_enter",
    "cycle_card",
    "you_commit_crime", "commit_crime",
    "pay_cost_multiple", "misc_whenever_a",
    "you_conjure_one_or_more", "you_mechanic",
    "self_or_another_when", "becomes_state",
    "becomes_target", "player_land_play",
    # Parser-coverage R60 batch — the *_to_gy_from_bf family and the
    # Era-4 long-tail slugs the new classifyTrigger cases route to
    # existing scaffolds. Same set as era1/2/3 audits.
    "self_put_into_graveyard_from_bf",
    "ally_type_to_gy_from_bf", "type_to_gy_from_bf",
    "to_gy_from_bf", "opp_type_to_gy_from_bf",
    "ally_typed_to_gy", "tribal_to_gy_from_bf",
    "nontoken_type_to_gy", "opp_creature_to_gy",
    "self_to_gy", "self_die_or_ally_gy",
    "specialize_from_zone",
    "ring_tempts_you", "train",
    "modified_creature_event",
    "face_down_creature_event",
    "compound_tribe_enter",
    "it_state_change",

    # Era 4 r60 trigger-gap closure — bring Era 4's bucketed-events set to
    # parity with Era 1's. The Go-side classifyTrigger already routes every
    # one of these slugs to an existing scaffold (see conditional_setup.go
    # lines 260-353); this is the audit-side mirror so the trigger-gap
    # metric stops reporting them as unbucketed. No engine change needed.
    "tap_for_mana", "land_tapped_for_mana",            # → tapped_for_mana
    "block_or_becomes_blocked", "block_creature",
    "becomes_blocked_by", "self_blocks",
    "tap_opp_creature",                                # → attacks
    "self_deals_damage_player", "you_dealt_damage",
    "per_damage_prevented", "opp_dealt_damage",        # → combat_damage
    "creature_etb_any", "land_etb_any",
    "artifact_etb", "any_typed_etb",
    "one_or_more_lands", "equipment_attach_state_change",
    "you_create_one_or_more_tokens", "create_token",
    "gain_control_event",
    "another_creature_or_artifact_event",              # → creature_etb
    "another_typed_etb",                               # → tribe_you_control_etb
    "ally_typed_etb", "legend_ally_event",
    "one_or_more_other_ally_event",
    "one_or_more_ally_with_x_enter",                   # → ally_etb
    "you_scry", "you_surveil",                         # → draw_card
    "to_gy_from_anywhere", "creature_cards_leave_gy",
    "card_to_gy_anywhere", "enchanted_perm_to_gy",
    "lose_control_of", "one_or_more_milled",
    "mill_event", "creature_cards_to_zone",
    "any_player_loses_game",                           # → creature_dies
    "any_cycle",                                       # → discard
    "you_put_counters_on", "you_put_counters_on_any",
    "you_proliferate", "counter_removed_from_self",    # → counters_put_on_self
    "any_player_sacs",                                 # → sacrifice
    "opp_activate", "opp_searches_library",            # → opp_creature_event
    "any_player_tap_land", "self_phase_inout",
    "until_eot_trigger",                               # → upkeep
    "you_misc_event", "you_exert_creature",
    "as_you_draft_a_card", "become_monarch",
    "you_expend_n", "self_ability_activated",          # → when_you_do
    # Phase-style + long tail routes.
    "end_step", "upkeep", "untap_step",
    "each_upkeep", "cumulative_upkeep_unpaid", "cumulative_upkeep_paid",
    "until_next_phase", "saga_final_chapter",
    "ordinal_trigger", "upkeep_life_leader", "first_main",
    "becomes_renowned",
    "evolve_event", "counter_put_on_self",
    "counters_removed_from_self", "counters_put_on_actor_any",
    "remove_last_counter", "counter_threshold", "proliferate",
    "flip",
    "aura_attached_event", "forest_etb",
    "creature_etb", "power_threshold_etb",
    "compound_opponents_event", "opp_landfall", "opp_shuffle",
    "compound_tribe_die_or_leave", "self_card_zone_to_zone",
    "lose_control", "is_sac_or_destroyed",
    "permanent_returned", "self_enter_or_die", "exiled",
    "activation_non_mana", "this_card_event",
    "chosen_color_mana_added", "chosen_color_mana_tapped",
    "tempting_offer", "vote", "self_squad_action",
    "one_or_more_other_creatures",
    "one_or_more_ally_creatures",
    "paired_whenever",
    "opponents_dealt_combat_dmg",
    "becomes_saddled_first",
    "you_sac_one_or_more",
    "you_control_7_thrulls",

    # Era 4 r60 final trigger-gap closure — 14 residual slugs from the
    # post-batch scan, each ×1 or ×2 in the corpus. Mirrors the Go
    # classifyTrigger routes in conditional_setup.go added in the same
    # commit. Drives Era 4 trigger gap 0.6% → 0.0%.
    "exiled_event", "any_type_to_gy_from_bf",
    "creature_or_land_to_gy", "compound_bounce_shuffle_event",  # → creature_dies
    "self_or_typed_event", "other_nontoken_perm",                # → self_and
    "put_onto_bf", "artifact_etb_yours", "merfolk_etb_any",      # → creature_etb
    "nonland_tapped_for_mana",                                    # → tapped_for_mana
    "becomes_untapped_once",                                      # → becomes_untapped
    "paid_cumulative_upkeep",                                     # → upkeep
    "discover",                                                   # → draw_card
    "targets_chosen",                                             # → when_you_do
}


def classify_trigger_event(event: str, phase: str) -> bool:
    e = (event or "").lower().strip()
    p = (phase or "").lower().strip()
    if not e and not p:
        return False
    if e in TRIGGER_EVENT_EXACT or e in TRIGGER_EXTRA_EXACT:
        return True
    for kw in TRIGGER_SUBSTRING_CATCHES:
        if kw in e:
            if kw in ("gain", "lose") and "life" not in e:
                continue
            return True
    if p:
        return True
    return False


def walk(node, conds, trigs):
    if isinstance(node, dict):
        t = node.get("__ast_type__")
        if t == "Condition":
            conds.append(node)
        elif t == "Trigger":
            trigs.append(node)
        for v in node.values():
            walk(v, conds, trigs)
    elif isinstance(node, list):
        for x in node:
            walk(x, conds, trigs)


def main():
    cond_kinds = Counter()
    cond_kinds_bucketed = Counter()
    cond_kinds_unbucketed = Counter()
    trig_events = Counter()
    trig_events_bucketed = Counter()
    trig_events_unbucketed = Counter()
    raw_buckets = Counter()
    raw_unbucketed_text = Counter()
    raw_unbucketed_examples = defaultdict(list)

    total_cards = 0
    target_cards = 0
    era_counts = Counter()

    with DATASET.open("r", encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            row = json.loads(line)
            total_cards += 1
            era = classify_era(row.get("oracle_text", ""), row.get("type_line", ""))
            era_counts[era] += 1
            if era != TARGET_ERA:
                continue
            target_cards += 1
            ast = row.get("ast")
            if not ast:
                continue
            conds, trigs = [], []
            walk(ast, conds, trigs)
            name = row.get("name", "?")
            for c in conds:
                k = (c.get("kind") or "").lower()
                cond_kinds[k] += 1
                if k in BUCKETED_KINDS:
                    cond_kinds_bucketed[k] += 1
                elif k in RAW_KINDS:
                    args = c.get("args") or []
                    raw_txt = ""
                    if args and isinstance(args[0], str):
                        raw_txt = args[0]
                    bucket = match_raw(raw_txt) if raw_txt else None
                    if bucket:
                        raw_buckets[bucket] += 1
                        cond_kinds_bucketed[k] += 1
                    else:
                        cond_kinds_unbucketed[k] += 1
                        key = (raw_txt[:80] or "<empty>").lower()
                        raw_unbucketed_text[key] += 1
                        if len(raw_unbucketed_examples[key]) < 3:
                            raw_unbucketed_examples[key].append(name)
                else:
                    cond_kinds_unbucketed[k] += 1
            for tg in trigs:
                ev = (tg.get("event") or "").lower()
                ph = (tg.get("phase") or "").lower()
                trig_events[ev] += 1
                if classify_trigger_event(ev, ph):
                    trig_events_bucketed[ev] += 1
                else:
                    trig_events_unbucketed[ev] += 1

    total_conds = sum(cond_kinds.values())
    total_bucket = sum(cond_kinds_bucketed.values())
    total_unbucket = sum(cond_kinds_unbucketed.values())

    lines = []
    lines.append(f"# Era {TARGET_ERA} (2023-2026) Scaffold-Gap Audit\n")
    lines.append(f"- Total cards in dataset: **{total_cards}**\n")
    lines.append(f"- Era distribution: " +
                 ", ".join(f"era{e}={era_counts[e]}" for e in sorted(era_counts)) + "\n")
    lines.append(f"- Era {TARGET_ERA} cards: **{target_cards}**\n")
    lines.append(f"- Era {TARGET_ERA} Condition nodes: **{total_conds}** "
                 f"(bucketed {total_bucket}, unbucketed {total_unbucket}, "
                 f"{100.0*total_unbucket/max(1,total_conds):.1f}% gap)\n")
    total_trigs = sum(trig_events.values())
    total_trig_bucket = sum(trig_events_bucketed.values())
    total_trig_unbucket = sum(trig_events_unbucketed.values())
    lines.append(f"- Era {TARGET_ERA} Trigger nodes: **{total_trigs}** "
                 f"(bucketed {total_trig_bucket}, unbucketed {total_trig_unbucket}, "
                 f"{100.0*total_trig_unbucket/max(1,total_trigs):.1f}% gap)\n")

    lines.append("\n## Top unbucketed condition Kinds\n")
    for k, n in cond_kinds_unbucketed.most_common(60):
        lines.append(f"- `{k or '<empty>'}` × {n}")

    lines.append("\n## Top unbucketed raw-text fragments (kind in raw/intervening_if/as_long_as)\n")
    for txt, n in raw_unbucketed_text.most_common(60):
        ex = ", ".join(raw_unbucketed_examples[txt][:3])
        lines.append(f"- × {n}: `{txt}`  _(e.g. {ex})_")

    lines.append("\n## Bucketed condition Kinds (sanity)\n")
    for k, n in cond_kinds_bucketed.most_common(20):
        lines.append(f"- `{k}` × {n}")

    lines.append("\n## Top unbucketed trigger events\n")
    if not trig_events_unbucketed:
        lines.append("_(none — every Era 4 trigger event maps to a scaffold slug)_")
    for ev, n in trig_events_unbucketed.most_common(60):
        lines.append(f"- `{ev or '<empty>'}` × {n}")

    lines.append("\n## Top trigger events (bucketed + unbucketed)\n")
    for ev, n in trig_events.most_common(40):
        marker = "" if ev in trig_events_bucketed else " _(unbucketed)_"
        lines.append(f"- `{ev or '<empty>'}` × {n}{marker}")

    OUT.write_text("\n".join(lines))
    print("\n".join(lines))


if __name__ == "__main__":
    main()
