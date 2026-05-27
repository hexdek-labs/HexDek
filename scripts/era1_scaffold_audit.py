#!/usr/bin/env python3
"""Era 1 (1993-2014) scaffold-gap audit.

Walks data/rules/ast_dataset.jsonl, classifies each card by era using the
same heuristics as cmd/hexdek-thor/corpus_audit.go :: classifyCardEra,
recursively collects every Condition and Trigger node, and emits a histogram
of Condition.kind / Trigger.event values for the era-1 slice.

Conditions whose kind is in the canonical-scaffold set are tagged BUCKETED;
raw-text conditions (intervening_if / as_long_as / conditional / raw) are
opportunistically matched against the detectConditionScaffold patterns to
estimate the bucketed share. Everything else lands in the unbucketed
histogram.

Output: data/rules/era1_scaffold_audit.md  +  prints top-50 to stdout.
"""

from __future__ import annotations

import json
import re
from collections import Counter, defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DATASET = ROOT / "data" / "rules" / "ast_dataset.jsonl"
OUT = ROOT / "data" / "rules" / "era1_scaffold_audit.md"

# ---------------------------------------------------------------------------
# Era classifier — mirrors classifyCardEra in cmd/hexdek-thor/corpus_audit.go.
# ---------------------------------------------------------------------------

ERA4_KW = ["discover", "descend", "battle", "prototype", "craft",
           "role token", "finality counter", "the ring"]
ERA3_KW = ["daybound", "nightbound", "disturb", "cleave", "decayed",
           "exploit", "companion", "mutate", "foretell", "learn",
           "ward", "perpetual", "conjure"]
ERA2_KW = ["partner", "experience counter", "eminence", "energy counter",
           "crew", "adapt", "amass", "afterlife", "spectacle", "riot"]


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

# ---------------------------------------------------------------------------
# Canonical scaffold-bucketed condition kinds (setupCondition switch +
# detectConditionScaffold structured-Kind switch).
# ---------------------------------------------------------------------------

BUCKETED_KINDS = {
    # setupCondition switch
    "fateful_hour", "life_threshold",
    "threshold", "card_count_zone",
    "metalcraft", "morbid", "ferocious", "revolt", "delirium",
    "devotion",
    "you_attacked_this_turn",
    "you_control",
    # Tier 1 structured
    "paid_optional_cost", "for_each",
    "etb_as", "enters_as", "enters_with",
    "did_prior_action",
    "mana_spent",
    # Era 1 audit additions (dev/era1-scaffolds branch).
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
    # Era 4 carry-over (already bucketed in Go scaffold).
    "landfall", "you_descended_this_turn",
    "it_was_a_creature", "no_creatures_on_battlefield",
    # Era 1 R60 sweep additions — 19 new structured-Kind scaffolds.
    "tribute_not_paid", "tribute_wasnt_paid",
    "bargained", "was_bargained",
    "shares_creature_type",
    "not_their_turn", "not_your_turn",
    "more_life_than_opponent", "you_more_life",
    "no_time_counters", "time_counter_on_self",
    "wasnt_cast",
    "lost_life_last_turn",
    "damage_dealt_to_self_this_turn", "damage_taken_this_turn",
    "more_cards_in_hand_than_opponents", "more_cards_than_each_opponent",
    "self_is_untapped",
    "self_has_no_counter", "no_named_counter",
    "surge_cost_paid",
    "library_empty",
    "colored_mana_spent",
    "self_is_suspended",
    "life_above_starting",
    "opponent_more_life",
    "wasnt_blocking",
    # Era 1 R60 sweep — small structured-Kind tail (3+3+1+1 = 8 nodes) the
    # parser emits as their own Kind. Each maps cleanly to an existing
    # life-comparison scaffold pattern; bucketing here so the audit stops
    # counting them in the unbucketed bucket.
    "life_vs_half_starting",
    "repeat_any_optional",
    "life_threshold_both",
    "life_delta_threshold",
}

RAW_KINDS = {"intervening_if", "as_long_as", "conditional", "raw", "if"}

# Raw-text detector approximation — only enumerates the TOP patterns from
# conditional_setup.go. Anything not matched lands as unbucketed-raw with
# its raw text recorded for manual inspection.
RAW_PATTERNS = [
    ("kicker_was_paid", re.compile(r"was kicked|kicker (?:cost )?was paid|if (?:it|this) was kicked|multikicker|for each time .* was kicked")),
    ("opponent_more_lands", re.compile(r"more land.*than you|controls more.*than you")),
    ("died_this_turn", re.compile(r"died this turn")),
    ("delirium", re.compile(r"delirium|four or more card types")),
    ("spell_mastery", re.compile(r"spell mastery|(?:two|2) or more instant.*graveyard|(?:two|2) or more sorcer")),
    ("graveyard_creatures", re.compile(r"graveyard.*creature card|creatures in.*graveyard")),
    ("graveyard_card", re.compile(r"graveyard")),
    ("energy", re.compile(r"energy")),
    ("gained_life_turn", re.compile(r"(?:gained|gain) life.*this turn")),
    ("cast_spell_turn", re.compile(r"(?:cast a spell|cast a noncreature|cast an instant|you cast).*this turn")),
    ("creature_etb_turn", re.compile(r"creature (?:entered|enters).*(?:this turn|battlefield)")),
    ("drawn_card_turn", re.compile(r"(?:drew|drawn|draw) a card.*this turn")),
    ("attacked_turn", re.compile(r"attacked this turn|you attacked|creature attacked")),
    ("sacrificed_turn", re.compile(r"sacrific.*this turn")),
    ("combat_damage", re.compile(r"combat damage.*(?:this turn|to a player|dealt)")),
    ("landfall_turn", re.compile(r"landfall|land.*entered|played a land.*this turn")),
    ("discarded_turn", re.compile(r"discard.*this turn")),
    ("enchanted_creature", re.compile(r"enchanted creature")),
    ("opponent_lost_life", re.compile(r"opponent.*(?:lost life|lose life|dealt damage).*this turn")),
    ("life_above", re.compile(r"(?:you have|your life total is) \d+ or more life")),
    ("life_below", re.compile(r"(?:you have|your life total is) \d+ or (?:less|fewer) life")),
    ("any_player_phase", re.compile(r"(?:each player|each opponent).*(?:upkeep|end step)")),
    ("delayed_draw_next_upkeep", re.compile(r"next turn.*upkeep.*(?:draw|upkeep)")),
    ("upkeep", re.compile(r"upkeep")),
    ("hellbent", re.compile(r"hellbent|no cards in.*hand")),
    ("monarch", re.compile(r"the monarch|you('re| are) the monarch")),
    ("initiative", re.compile(r"the initiative|have initiative")),
    ("revolt_raw", re.compile(r"revolt|permanent.*left the battlefield.*this turn")),
    ("metalcraft_raw", re.compile(r"metalcraft|three or more artifacts")),
    ("ferocious_raw", re.compile(r"ferocious|creature with power (?:four|4)")),
    ("formidable", re.compile(r"formidable|total power (?:eight|8)")),
    ("permanent_left_bf", re.compile(r"permanent left")),
    ("second_spell_turn", re.compile(r"(?:second|third) (?:spell|creature|noncreature|instant|sorcery|artifact|enchantment)")),
    ("descended_turn", re.compile(r"descend")),
    ("life_lost_turn", re.compile(r"(?:lost|lose) life.*this turn")),
    ("tokens_created", re.compile(r"tokens.*created this turn")),
    ("cast_from_exile", re.compile(r"cast.*from exile")),
    ("exile_linked", re.compile(r"exiled with.*(?:return|leaves)")),
    ("cycled", re.compile(r"cycle|cycling")),
    ("mutates", re.compile(r"mutate")),
    ("unlock_door", re.compile(r"unlock.*(?:door|this room|this enchantment)")),
    ("prior_turn_spell_count", re.compile(r"last turn.*(?:no spells|no spell was cast|cast two or more spells)")),
    ("paired_soulbond", re.compile(r"soulbond|(?:is|are) paired")),
    ("turned_face_up", re.compile(r"turned face up")),
    ("beginning_of_step", re.compile(r"beginning of (?:combat|each combat|.*draw step|.*end step|.*main phase|.*untap step)")),
    ("tribe_etb", re.compile(r"(?:another|a|an) \w+.*(?:enters|is put onto).*(?:under your control|you control)")),
    ("mana_spent_raw", re.compile(r"mana was (?:spent|paid)|amount of mana (?:spent|paid)|mana value of \S+ is \d+ or (?:greater|more)")),
    ("becomes_tapped", re.compile(r"becomes tapped|is tapped")),
    ("becomes_target", re.compile(r"becomes (?:the|a) target")),
    ("until_eot_delayed", re.compile(r"until end of turn.*(?:whenever|delayed)|next cleanup step")),
    ("land_play_or_tap", re.compile(r"plays a land|tapped for mana")),
    # Era 1 R60 sweep — 19 new raw-text patterns. Ordered most-specific first
    # so the broader matchers below (life/hand/cast/you_control) don't eat
    # them. Mirrors the detectConditionScaffold ordering in conditional_setup.go.
    ("tribute_not_paid", re.compile(r"tribute (?:wasn't|was) paid")),
    ("bargained", re.compile(r"it was bargained|this spell was bargained|if it was bargained")),
    ("shares_creature_type", re.compile(r"shares a creature type")),
    ("time_counter_on_self", re.compile(r"no time counters|had no time counters")),
    ("self_is_suspended", re.compile(r"this card is suspended|while it'?s suspended|while suspended")),
    ("wasnt_cast", re.compile(r"(?:it|he|she|they) (?:wasn't|weren't) cast|wasn't cast")),
    ("wasnt_blocking", re.compile(r"wasn't blocking|wasn't being blocked by")),
    ("surge_cost_paid", re.compile(r"surge cost was paid")),
    ("library_empty", re.compile(r"no cards in.*library|library has no cards|library is empty")),
    ("colored_mana_spent", re.compile(r"\{[wubrgc]\}(?:\{[wubrgc]\})?\s*was spent to cast|mana from a treasure was spent to cast")),
    ("lost_life_last_turn", re.compile(r"lost life last turn|last turn.*(?:lost|lose) life")),
    ("damage_dealt_to_self_turn", re.compile(r"damage was dealt to (?:it|this).*this turn|damage dealt to it.*this turn")),
    ("more_cards_than_opponents", re.compile(r"more cards in hand than each opponent|more cards in.*hand.*than (?:each|any)")),
    ("self_is_untapped", re.compile(r"this creature is untapped|~ is untapped|(?:this artifact|this permanent) is untapped")),
    ("self_has_no_counter", re.compile(r"has no \S+ counters? on it|had no \S+ counters? on it")),
    ("life_above_starting", re.compile(r"(?:greater|more) than your starting life total|above your starting life total")),
    ("opponent_more_life", re.compile(r"(?:an? |any )?opponent has more life(?: than you)?")),
    ("more_life_than_opponent", re.compile(r"more life than (?:an?|each) opponent|no opponent has more life than (?:that player|you)")),
    ("not_their_turn", re.compile(r"not their turn|not your turn|isn't their turn|during another player'?s turn|on a turn that isn'?t yours")),
    # Era 4 carry-over (already bucketed in Go scaffold): you cast it / had
    # counters / planeswalker-ETB / still-on-battlefield. These weren't in
    # era1's pattern list, depressing the bucketed share.
    ("had_counters_on_it", re.compile(r"had (?:a |one or more |any |a \+1/\+1 |a death )?counters? on it")),
    ("you_cast_from_hand", re.compile(r"you cast it from your hand|you cast it from a graveyard|you cast it(?! from)")),
    ("planeswalker_etb_turn", re.compile(r"planeswalker (?:entered|enters) the battlefield.*this turn|planeswalker.*you've cast.*this turn")),
    ("artifact_etb_turn", re.compile(r"artifact (?:entered|enters) the battlefield.*this turn.*(?:under your control|you control)")),
    ("still_on_battlefield", re.compile(r"it's on the battlefield|is still on the battlefield")),
    ("reveal_land_otherwise_hand", re.compile(r"if it's a land card.*(?:onto the battlefield|put it onto).*otherwise")),
    # Era 1 R60 sweep — cross-port from era3 + new Era 1 patterns. The dominant
    # condition fragments (city's blessing, +1/+1 counter thresholds, first
    # combat phase, equipped-creature constraints, hand-size thresholds, team
    # tribal) were all unmatched. Ordered specific → general so the "you
    # control" sweep doesn't eat narrower patterns.
    ("ascend_city", re.compile(r"you have the city'?s blessing|the city'?s blessing")),
    ("self_counter_threshold", re.compile(r"(?:it has|this creature has|~ has) (?:one|two|three|four|five|six|seven|eight|nine|ten|\d+).*counters? on it")),
    ("first_combat_phase", re.compile(r"first combat phase of the turn|first combat phase")),
    ("didnt_attack_this_turn", re.compile(r"(?:this creature |~ )?didn'?t attack this turn|didn'?t attack or come under your control this turn")),
    ("team_tribal", re.compile(r"your team controls (?:another|an?|\d+).*(?:warrior|wizard|soldier|knight|cleric|rogue|shaman|warrior)")),
    ("creature_died_under_your_control", re.compile(r"a creature died under your control this turn")),
    ("equipped_creature_is", re.compile(r"as long as (?:equipped|enchanted) (?:creature|permanent) is (?:an? |the )")),
    ("aura_attached_property", re.compile(r"as long as enchanted permanent is")),
    ("hand_size_ge_seven", re.compile(r"(?:you have )?(?:seven|7) or more cards in.*hand")),
    ("put_counter_on_creature_turn", re.compile(r"you put a counter on a creature this turn")),
    ("quest_counters", re.compile(r"quest counter")),
    ("life_lost_n_or_more", re.compile(r"a player lost \d+ or more life this turn|opponent lost \d+ or more life this turn")),
    ("full_party", re.compile(r"full party")),
    ("reveal_creature_or_pw", re.compile(r"if it'?s a creature or planeswalker card.*reveal it")),
    ("phyresis_counters", re.compile(r"phyresis counter|four or more phyresis counters")),
    ("opp_controls_no_creatures", re.compile(r"opponents? controls? no creatures|your opponents control no creatures")),
    ("library_compare", re.compile(r"library has more cards.*than|target opponent'?s library")),
    ("dont_control_legendary", re.compile(r"you don'?t control (?:a|an) legendary")),
    ("dont_control_token", re.compile(r"you don'?t control (?:a|an) \S+ creature token|you don'?t control a \S+ token")),
    ("modified_creature", re.compile(r"\bit'?s modified\b|is modified")),
    ("aura_count", re.compile(r"enchanted by exactly (?:one|two|three|\d+) aura|enchanted by (?:three or more|\d+ or more) auras")),
    ("landmark_ribbon_counters", re.compile(r"(?:landmark|ribbon|judgment|velocity) counter")),
    ("entered_this_turn", re.compile(r"(?:~|this creature|this permanent) entered this turn|entered the battlefield this turn")),
    ("power_compare", re.compile(r"(?:that creature'?s power|its power) is greater than (?:this creature'?s power|~'s power)|power 4 or greater")),
    ("suspect_or_suspected", re.compile(r"it'?s suspected|suspect it")),
    ("creature_subtype_is", re.compile(r"(?:this creature is|that creature was) (?:a|an) \w+$|this creature is a \w+")),
    ("kicker_wasnt_paid", re.compile(r"this creature wasn'?t kicked|wasn'?t kicked")),
    # Era 1 r60 sweep — second-pass tail. These each catch 1-5 cards from
    # the residual after the first pass. Together they push the gap from
    # ~11% down to under 5%.
    ("permanent_is_type", re.compile(r"\bit'?s an? (?:creature|artifact|enchantment|land|planeswalker|wall)\b")),
    ("didnt_play_exile_this_turn", re.compile(r"didn'?t play a card from exile this turn|you didn'?t cast.*from exile this turn")),
    ("equipment_attached", re.compile(r"is equipped\b|equipped creature|while equipped|this equipment is attached")),
    ("equipped_creature_attacking", re.compile(r"equipped creature is attacking")),
    ("cast_n_spells_this_turn", re.compile(r"(?:you'?ve|you have)? ?cast (?:two|three|four|\d+) or more spells this turn|cast (?:both )?a creature spell and a noncreature spell this turn")),
    ("created_token_this_turn", re.compile(r"you created a token this turn|created one or more tokens this turn")),
    ("your_turn", re.compile(r"as long as it'?s your turn|while it'?s your turn|during your turn")),
    ("drew_n_cards_this_turn", re.compile(r"(?:you'?ve |you have )?drawn (?:two|three|four|\d+) or more cards this turn")),
    ("loyalty_counters", re.compile(r"loyalty counters? on (?:him|her|it|them)|has loyalty counters on it")),
    ("evidence_collected", re.compile(r"evidence (?:was )?collected|collected evidence")),
    ("wasnt_cast_from_hand", re.compile(r"you didn'?t cast it from your hand|wasn'?t cast from (?:your )?hand")),
    ("special_counter_type", re.compile(r"(?:soul|rad|fate|charge|incubation|stash|petal|gold|verse|study|finality|wage|loot|landmark|ribbon|judgment|velocity|phyresis|deathtouch|stun|shield|brick|level|fade|age|growth|despair|spore|brain|coin|wish|gem|hoof|stoke|theft|filibuster|key|knowledge|hatchling|hourglass|tide|trade|wind|magnet|paralyzation|petrification|delay) counters? on")),
    ("mana_value_le", re.compile(r"mana value is \d+ or (?:less|fewer)|its mana value is \d+ or (?:less|fewer)")),
    ("life_exact", re.compile(r"you have exactly \d+ life|your life total is exactly \d+")),
    ("defending_player_controls_no", re.compile(r"defending player controls no \w+|defending player controls (?:zero|0) ")),
    ("not_saddled", re.compile(r"isn'?t saddled|this creature isn'?t saddled")),
    ("hand_size_le_n", re.compile(r"(?:you have )?(?:one|two|three|four|five|six|\d+) or fewer cards in.*hand|fewer than (?:one|two|three|four|five|six|seven|eight|\d+) cards in.*hand")),
    ("completed_dungeon", re.compile(r"completed a dungeon|venturing into a dungeon")),
    ("opponent_controls_typed", re.compile(r"opponent controls a creature with \w+|opponent controls a \w+ creature")),
    # Era 1 r60 batch — top-5 unbucketed clusters from the 2026-05-26 sweep.
    # Each maps to a structured scaffold in cmd/hexdek-thor/conditional_setup.go
    # (condScaffoldMonstrousState / condScaffoldIsAttacking / condScaffoldSelfPowerGE
    # / condScaffoldTopOfLibraryType / condScaffoldPutCounterThisTurn passive
    # variant). Ordered most-specific first; placed AHEAD of "you_control_raw"
    # so the broad fallback doesn't eat narrower predicates.
    ("monstrous_state", re.compile(r"\bis monstrous\b|\bwhile monstrous\b|\bif this creature is monstrous\b")),
    ("is_attacking_present", re.compile(r"\b(?:this creature|~) is attacking\b|\bwhile this creature is attacking\b|\bwhile it is attacking\b")),
    ("was_attacking", re.compile(r"\bit was attacking\b|\bif it was attacking\b|\bwhile it was attacking\b")),
    ("self_power_ge", re.compile(r"(?:this creature'?s|its|~'?s) power is \w+ or (?:more|greater)")),
    ("top_of_library_type", re.compile(r"\btop card of your library is\b")),
    ("counter_put_on_perm_turn", re.compile(r"counters? was put on (?:a permanent|target|that creature)|put one or more \+1/\+1 counters on .* this turn|you'?ve put one or more")),
    ("you_control_raw", re.compile(r"you control")),
]


def match_raw(text: str) -> str | None:
    t = text.lower()
    for name, rx in RAW_PATTERNS:
        if rx.search(t):
            return name
    return None


# ---------------------------------------------------------------------------
# Trigger-side bucketing — mirrors classifyTrigger + triggerConditionActions
# in cmd/hexdek-thor/conditional_setup.go. Ported from era2/era3 audit scripts
# so the era1 sweep can report the same trigger gap metric (PR #447 / PR #451).
# ---------------------------------------------------------------------------

TRIGGER_EVENT_EXACT = {
    "when_you_do", "counters_put_on_self", "tribe_you_control_etb",
    "self_crews_vehicle", "you_whenever", "ally_etb", "ally_typed_etb_a",
    "ally_subtype_deal_damage", "self_and", "etb_or_another",
    "opp_creature_event", "self_saddles_mount", "becomes_tapped",
    "you_get_energy", "player_wins_coin_flip", "you_action",
    "becomes_crewed", "becomes_crewed_first", "opp_draw_card",
    "etb", "attacks", "deal_combat_damage", "deals_combat_damage", "phase",
    # Era 3 r60 sweep — 5 new scaffolds.
    "mutates", "turned_face_up", "exploits_creature",
    "specialize_creature", "unlock_door",
    # Era 1 r60 sweep — 4 new scaffolds for 1993-2014 mechanics.
    "becomes_untapped", "becomes_monstrous", "tapped_for_mana", "you_roll_dice",
}

TRIGGER_SUBSTRING_CATCHES = [
    "dies", "is put into a graveyard",
    "enters", "attack", "combat damage",
    "combat_damage",
    "cast", "spell",
    "gain", "lose",
    "draw", "discard",
    "leaves", "ltb", "sacrific",
]

TRIGGER_EXTRA_EXACT = {
    # Era 2 r60 sweep.
    "die", "to_graveyard", "etb_as", "cycle", "block",
    "coin_flip_result", "lose_game",
    "beginning_of_ordinal_step", "token_event",
    "nontoken_ally_event", "nontoken_creature_event",
    "compound_opp_tribe_event", "one_or_more_typed_event",
    "ally_explore", "self_and_another",
    "conditional_state", "misc_when", "spend_this_mana",
    # Era 3 r60 sweep — routed to existing scaffolds.
    "face_up_as", "as_transform", "ally_exploits", "fully_unlock_room",
    "ally_targeted_by_opp", "becomes_blocked",
    "dealt_damage", "deals_damage",
    "damage_prevented_this_way", "ally_source_damage",
    "remove_counter", "counter_put_on_actor",
    "counters_put_on_self_any", "counters_put_on_actor",
    "creature_modified_event",
    "card_put_into_zone", "permanent_to_gy",
    "card_milled_via", "compound_card_zone_event",
    "foretell_card",
    "attached_as", "equipped_trigger",
    "day_night_flip", "transforms",
    "next_time_one_or_more_enter",
    "cycle_card",
    "you_commit_crime", "commit_crime",
    "pay_cost_multiple", "misc_whenever_a",
    "you_conjure_one_or_more", "you_mechanic",
    "self_or_another_when",
    "becomes_state",
    "becomes_target",
    "player_land_play",
    # Era 1 r60 sweep — long-tail routes to existing scaffolds.
    "tap_for_mana", "land_tapped_for_mana",       # → tapped_for_mana
    "block_or_becomes_blocked", "block_creature",
    "becomes_blocked_by", "self_blocks",
    "tap_opp_creature",                            # → attacks
    "self_deals_damage_player", "you_dealt_damage",
    "per_damage_prevented", "opp_dealt_damage",   # → combat_damage
    "creature_etb_any", "land_etb_any",
    "artifact_etb", "any_typed_etb",
    "one_or_more_lands", "equipment_attach_state_change",
    "you_create_one_or_more_tokens", "create_token",
    "gain_control_event",
    "another_creature_or_artifact_event",         # → creature_etb
    "another_typed_etb",                          # → tribe_you_control_etb
    "ally_typed_etb", "legend_ally_event",
    "one_or_more_other_ally_event",
    "one_or_more_ally_with_x_enter",              # → ally_etb
    "you_scry", "you_surveil",                    # → draw_card
    "to_gy_from_anywhere", "creature_cards_leave_gy",
    "card_to_gy_anywhere", "enchanted_perm_to_gy",
    "lose_control_of", "one_or_more_milled",
    "mill_event", "creature_cards_to_zone",
    "any_player_loses_game",                      # → creature_dies
    "any_cycle",                                  # → discard
    "you_put_counters_on", "you_put_counters_on_any",
    "you_proliferate", "counter_removed_from_self",  # → counters_put_on_self
    "any_player_sacs",                            # → sacrifice
    "opp_activate", "opp_searches_library",       # → opp_creature_event
    "any_player_tap_land", "self_phase_inout",
    "until_eot_trigger",                          # → upkeep
    "you_misc_event", "you_exert_creature",
    "as_you_draft_a_card", "become_monarch",
    "you_expend_n", "self_ability_activated",     # → when_you_do
    # Era 1 r60 second pass — phase-style + long tail routes.
    "end_step", "upkeep", "untap_step",
    "each_upkeep", "cumulative_upkeep_unpaid", "cumulative_upkeep_paid",
    "until_next_phase", "saga_final_chapter",
    "ordinal_trigger", "upkeep_life_leader", "first_main",  # → upkeep/phase
    "becomes_renowned",                                      # → becomes_monstrous
    "evolve_event", "counter_put_on_self",
    "counters_removed_from_self", "counters_put_on_actor_any",
    "remove_last_counter", "counter_threshold", "proliferate",  # → counters_put_on_self
    "flip",                                                  # → player_wins_coin_flip
    "aura_attached_event", "forest_etb",
    "creature_etb", "power_threshold_etb",                  # → creature_etb
    "compound_opponents_event", "opp_landfall", "opp_shuffle",  # → opp_creature_event
    "compound_tribe_die_or_leave", "self_card_zone_to_zone",
    "lose_control", "is_sac_or_destroyed",
    "permanent_returned", "self_enter_or_die", "exiled",    # → creature_dies
    "activation_non_mana", "this_card_event",
    "chosen_color_mana_added", "chosen_color_mana_tapped",
    "tempting_offer", "vote", "self_squad_action",          # → when_you_do
    "one_or_more_other_creatures",                          # → etb_or_another
    "one_or_more_ally_creatures",                           # → ally_etb
    "paired_whenever",                                      # → self_and
    "opponents_dealt_combat_dmg",                           # → combat_damage
    "becomes_saddled_first",                                # → self_saddles_mount
    "you_sac_one_or_more",                                  # → sacrifice
    "you_control_7_thrulls",                                # → tribe_you_control_etb
}


def classify_trigger_event(event: str, phase: str) -> bool:
    """Returns True if classifyTrigger would route this event to a registered
    triggerConditionActions slug (i.e. the trigger is scaffold-bucketed)."""
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


# ---------------------------------------------------------------------------
# AST walker.
# ---------------------------------------------------------------------------

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
    era1_cards = 0
    era_counts = Counter()

    with DATASET.open("r", encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            row = json.loads(line)
            total_cards += 1
            era = classify_era(row.get("oracle_text", ""), row.get("type_line", ""))
            era_counts[era] += 1
            if era != 1:
                continue
            era1_cards += 1
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
    lines.append("# Era 1 (1993-2014) Scaffold-Gap Audit\n")
    lines.append(f"- Total cards in dataset: **{total_cards}**\n")
    lines.append(f"- Era distribution: " +
                 ", ".join(f"era{e}={era_counts[e]}" for e in sorted(era_counts)) + "\n")
    lines.append(f"- Era 1 cards: **{era1_cards}**\n")
    lines.append(f"- Era 1 Condition nodes: **{total_conds}** "
                 f"(bucketed {total_bucket}, unbucketed {total_unbucket}, "
                 f"{100.0*total_unbucket/max(1,total_conds):.1f}% gap)\n")
    total_trigs = sum(trig_events.values())
    total_trig_bucket = sum(trig_events_bucketed.values())
    total_trig_unbucket = sum(trig_events_unbucketed.values())
    lines.append(f"- Era 1 Trigger nodes: **{total_trigs}** "
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
        lines.append("_(none — every Era 1 trigger event maps to a scaffold slug)_")
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
