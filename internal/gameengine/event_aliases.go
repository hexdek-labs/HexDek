package gameengine

import "strings"

// NormalizeEvent maps a parser-emitted trigger event name to the canonical
// engine event name(s) that should match it. Returns nil if the event is
// already canonical or has no known mapping (caller should fall through to
// the original event name).
//
// Some parser events are "compound" — they should match on MORE THAN ONE
// engine event. For example, "enter_or_attack" fires on either ETB or attack.
// These return multiple canonical names.
func NormalizeEvent(event string) []string {
	event = strings.ToLower(strings.TrimSpace(event))
	if aliases, ok := eventAliasTable[event]; ok {
		return aliases
	}
	return nil
}

// NormalizeEventSingle returns the single canonical event (or the original
// if no alias exists). For compound events, returns the first canonical form.
func NormalizeEventSingle(event string) string {
	event = strings.ToLower(strings.TrimSpace(event))
	if aliases, ok := eventAliasTable[event]; ok && len(aliases) > 0 {
		return aliases[0]
	}
	return event
}

// EventMatchesAny returns true if the trigger event (after normalization)
// matches any of the target events.
func EventMatchesAny(triggerEvent string, targets []string) bool {
	triggerEvent = strings.ToLower(strings.TrimSpace(triggerEvent))

	// Direct match first.
	for _, t := range targets {
		if triggerEvent == t {
			return true
		}
	}

	// Normalized match.
	if aliases := NormalizeEvent(triggerEvent); aliases != nil {
		for _, alias := range aliases {
			for _, t := range targets {
				if alias == t {
					return true
				}
			}
		}
	}

	return false
}

// EventEquals returns true if the trigger event (after normalization)
// matches the target event exactly.
func EventEquals(triggerEvent, target string) bool {
	triggerEvent = strings.ToLower(strings.TrimSpace(triggerEvent))
	target = strings.ToLower(strings.TrimSpace(target))

	if triggerEvent == target {
		return true
	}

	if aliases := NormalizeEvent(triggerEvent); aliases != nil {
		for _, a := range aliases {
			if a == target {
				return true
			}
		}
	}

	return false
}

// eventAliasTable maps parser event names → engine canonical event name(s).
//
// Categories:
//   - Direct alias: parser spelling → single engine event
//   - Compound:     parser event → multiple engine events (fires on either)
//   - Observer:     parser observer event → engine event (with actor filter semantics)
var eventAliasTable = map[string][]string{
	// -----------------------------------------------------------------------
	// ETB aliases (self-trigger) — all normalize to "etb"
	// -----------------------------------------------------------------------
	"etb_as":             {"etb"}, // "enters as a copy of"
	"etb_tapped_event":   {"etb"}, // "enters the battlefield tapped"
	"put_onto_bf":        {"etb"}, // "is put onto the battlefield"
	"comes_into_play":    {"etb"}, // old wording

	// -----------------------------------------------------------------------
	// Attack aliases — normalize to "attack"
	// -----------------------------------------------------------------------
	"you_attack":         {"attack"}, // "whenever you attack"
	"attacks":            {"attack"}, // plural form
	"you_attack_with":    {"attack"}, // "whenever you attack with"
	"attack_while_saddled": {"attack"}, // attack + saddled condition

	// -----------------------------------------------------------------------
	// Dies / LTB aliases — normalize to zone-change canonical events
	// -----------------------------------------------------------------------
	// Observer dies (any creature / typed creature / tribe)
	"creature_dies":                    {"die"},
	"another_creature_dies":            {"die"},
	"another_creature_dies_any":        {"die"},
	"another_nontoken_creature_dies_any": {"die"},
	"tribe_you_control_dies":           {"die"},
	"another_typed_dies":               {"die"},

	// Self dies / LTB variants
	"self_put_into_graveyard_from_bf": {"die"},
	"self_leaves_battlefield":        {"ltb"},

	// Observer LTB
	"type_leaves_battlefield": {"ltb"},
	"another_ally_leaves":     {"ltb"},
	"enchanted_ltb":           {"ltb"},

	// Graveyard from anywhere
	"to_graveyard":          {"put_into_graveyard"},
	"to_gy_from_bf":         {"die"},    // battlefield→graveyard = dies
	"to_gy_from_anywhere":   {"put_into_graveyard"},
	"permanent_to_gy":       {"put_into_graveyard"},
	"type_to_gy_from_bf":    {"die"},
	"card_to_gy_anywhere":   {"put_into_graveyard"},
	"ally_type_to_gy_from_bf": {"die"},
	"ally_typed_to_gy":      {"put_into_graveyard"},
	"enchanted_perm_to_gy":  {"put_into_graveyard"},
	"opp_type_to_gy_from_bf": {"die"},
	"creature_cards_to_zone": {"put_into_graveyard"},

	// -----------------------------------------------------------------------
	// ETB observer — normalize to "etb" so fireObserverETBTriggers can match
	// -----------------------------------------------------------------------
	"another_typed_enters":   {"etb"},
	"tribe_you_control_etb":  {"etb"},
	"ally_etb":               {"etb"},
	"another_creature_enters": {"etb"},
	"another_typed_etb":      {"etb"},
	"ally_typed_etb":         {"etb"},
	"creature_etb_any":       {"etb"},
	"artifact_you_control_enters": {"etb"},
	"another_subtype_enters": {"etb"},
	"nontoken_ally_event":    {"etb"},
	"enchantment_you_control_enters": {"etb"},
	"artifact_etb":           {"etb"},
	"nontoken_creature_event": {"etb"},
	"land_etb_any":           {"etb"},
	"permanent_enters":       {"etb"},
	"typed_enters_your_control": {"etb"},
	"one_or_more_ally_with_X_enter": {"etb"},

	// -----------------------------------------------------------------------
	// Combat damage aliases
	// -----------------------------------------------------------------------
	"combat_damage_player":     {"deals_combat_damage"},
	"self_deals_damage_player": {"deals_combat_damage"},
	"group_combat_damage_player": {"deals_combat_damage"},
	"combat_damage":            {"deals_combat_damage"},
	"combat_damage_creature":   {"deals_combat_damage"},
	"combat_damage_player_or_pw": {"deals_combat_damage"},
	"combat_damage_opponent":   {"deals_combat_damage"},
	"combat_damage_player_or_battle": {"deals_combat_damage"},
	"combat_damage_to_player":  {"deals_combat_damage"},
	"self_combat_damage":       {"deals_combat_damage"},
	"compound_tribe_combat_damage": {"deals_combat_damage"},
	"one_or_more_creatures_combat_damage": {"deals_combat_damage"},

	// Engine-internal damage aliases (stack/combat tests use these forms)
	"deal_combat_damage": {"deals_combat_damage"},
	"deal_damage":        {"deals_damage"},

	// Non-combat damage
	"dealt_damage":    {"deals_damage"}, // "is dealt damage"

	// -----------------------------------------------------------------------
	// Cast aliases — normalize to "cast" for observer cast scanning
	// -----------------------------------------------------------------------
	"cast_filtered":      {"cast"},
	"cast_any":           {"cast"},
	"any_player_cast":    {"cast"},
	"cast_spell":         {"cast"},
	"cast_color_spell":   {"cast"},
	"opp_cast":           {"cast"},
	"opp_cast_spell":     {"cast"},
	"opp_cast_color_spell": {"cast"},
	"cast_mana_value":    {"cast"},
	"cast_color_filtered": {"cast"},
	"cast_x_spell":       {"cast"},
	"you_next_cast":      {"cast"},
	"any_cast":           {"cast"},
	"opp_spell_or_ability": {"cast"},
	"cast_self":          {"cast"},

	// -----------------------------------------------------------------------
	// Compound events — match on multiple canonical events
	// -----------------------------------------------------------------------
	"enter_or_attack":     {"etb", "attack"},
	"etb_or_attacks":      {"etb", "attack"},
	"etb_or_attack":       {"etb", "attack"},
	"self_and_another":    {"etb", "attack"}, // typically "when ~ enters or attacks"
	"self_and":            {"etb", "attack"},
	"paired_etb_attack":   {"etb", "attack"},
	"self_or_typed_event": {"etb", "attack"},
	"etb_or_ltb":          {"etb", "ltb"},
	"etb_or_die":          {"etb", "die"},
	"die_self_or_ally":    {"die"},

	// -----------------------------------------------------------------------
	// Block aliases — fireBlockTriggers dispatches "block" and "blocked"
	// -----------------------------------------------------------------------
	"self_blocks":              {"block"},
	"block_creature":           {"block"},
	"block_or_becomes_blocked": {"block", "blocked"},
	"attack_or_block":          {"attack", "block"},
	"attack_alone":             {"attack"},
	"attack_unblocked":         {"attack"},

	// -----------------------------------------------------------------------
	// Cycle aliases
	// -----------------------------------------------------------------------
	"cycle_event":       {"cycle"},
	"cycle_card":        {"cycle"},
	"any_cycle":         {"cycle"},
	"cycle_or_discard":  {"cycle", "discard"},

	// -----------------------------------------------------------------------
	// Life/damage event aliases
	// -----------------------------------------------------------------------
	"gain_life":            {"life_gained"},
	"gain_life_threshold":  {"life_gained"},
	"lose_life":            {"life_lost"},
	"lose_life_threshold":  {"life_lost"},
	"you_lose_life":        {"life_lost"},
	"opponent_loses_life":  {"life_lost"},
	"you_dealt_damage":     {"deals_damage"},
	"ally_source_damage":   {"deals_damage"},

	// -----------------------------------------------------------------------
	// Misc event aliases
	// -----------------------------------------------------------------------
	"becomes_tapped":  {"tap_event"},
	"tapped_for_mana": {"tap_for_mana"},
	"tap_for_mana":    {"tap_for_mana"},
	"land_tapped_for_mana": {"tap_for_mana"},
	"becomes_untapped": {"untap_event"},
	"turned_face_up":   {"face_up"},
	"transforms":       {"transform"},
	"day_night_flip":   {"transform"},
	"mutates":          {"mutate"},
	"exploits_creature": {"exploit"},
	"ally_exploits":    {"exploit"},
	"becomes_target":   {"targeted"},
	"becomes_target_whenever": {"targeted"},
	"ally_targeted_by_opp":    {"targeted"},
	"becomes_state":    {"state_change"},
	"becomes_monstrous": {"monstrous"},
	"becomes_blocked":  {"blocked"},
	"becomes_blocked_by": {"blocked"},
	"sacrifice_filtered":  {"sacrifice"},
	"sacrifices":          {"sacrifice"},
	"sacrifices_filtered": {"sacrifice"},
	"conditional_sacrifice": {"sacrifice"},
	"you_sacrifice_type": {"sacrifice"},
	"sacrifice_self_or_ally": {"sacrifice"},
	"opp_sacrifice_filtered": {"sacrifice"},
	"any_player_sacs":    {"sacrifice"},
	"draw_card":         {"draw"},
	"draw_event":        {"draw"},
	"ordinal_draw":      {"draw"},
	"opp_draw_card":     {"draw"},
	"opp_draws_ordinal": {"draw"},
	"discard_event":     {"discard"},
	"discard_filtered":  {"discard"},
	"discard_one_or_more": {"discard"},
	"opp_discard_filtered": {"discard"},
	"player_discard":    {"discard"},
	"opp_makes_you_discard_self": {"discard"},
	"discard_filter_cards": {"discard"},
	"counters_put_on_self": {"counter_placed"},
	"counters_put_on_actor": {"counter_placed"},
	"you_put_counters_on": {"counter_placed"},
	"you_put_counters_on_any": {"counter_placed"},
	"counter_removed_from_self": {"counter_removed"},
	"create_token":        {"token_created"},
	"create_or_sac_token": {"token_created", "sacrifice"},
	"token_event":         {"token_created"},
	"landfall":            {"land_played"},
	"any_landfall":        {"land_played"},
	"your_landfall":       {"land_played"},
	"one_or_more_lands":   {"land_played"},
	"player_land_play":    {"land_played"},
	"opp_land_enters":     {"land_played"},
	"you_scry":            {"scry"},
	"you_surveil":         {"surveil"},
	"ally_explore":        {"explore"},
	"you_proliferate":     {"proliferate"},
	"commit_crime":        {"crime"},
	"you_expend_n":        {"expend"},
	"ring_tempts_you":     {"ring_tempt"},
	"you_roll_dice":       {"roll_dice"},
	"coin_flip_result":    {"coin_flip"},
	"you_exert_creature":  {"exert"},
	"you_get_energy":      {"energy"},
	"equipped_trigger":    {"equip"},
	"equipment_attach_state_change": {"equip"},
	"self_crews_vehicle":  {"crew"},
	"class_becomes_level": {"class_level"},
	"specialize_creature": {"specialize"},
	"specialize_from_zone": {"specialize"},
	"unlock_door":         {"unlock_door"},
	"fully_unlock_room":   {"unlock_room"},
	"as_you_draft_a_card": {"draft"},
	"opp_creature_event":  {"etb"},
	"lose_control_of":     {"control_change"},
	"gain_control_event":  {"control_change"},
	"opp_searches_library": {"search_library"},
	"creature_cards_leave_gy": {"leave_gy"},
	"leave_gy":            {"leave_gy"},
	"leave_gy_single":     {"leave_gy"},
	"exiled_event":        {"exiled"},
	"that_creature_ltb_this_turn": {"ltb"},
	"self_phase_inout":    {"phase_inout"},
	"enchanted_player_attacked": {"attack"},
	"tap_opp_creature":    {"tap_event"},
	"opp_activate":        {"activate"},
	"pay_cost_multiple":   {"cost_paid"},
	"spend_this_mana":     {"mana_spent"},
	"end_step_gained_n_life_this_turn": {"end_step_condition"},
	"any_player_tap_land": {"tap_for_mana"},

	// -----------------------------------------------------------------------
	// Phase aliases (supplement triggerMatchesPhaseStep)
	// -----------------------------------------------------------------------
	"end_step":                   {"end_step"},
	"beginning_of_ordinal_step":  {"phase"},
	"upkeep":                     {"upkeep"},

	// -----------------------------------------------------------------------
	// Catch-all parser fallbacks
	// -----------------------------------------------------------------------
	"when_you_do":         {"cascading"},
	"you_whenever":        {"generic_you"},
	"you_action":          {"generic_you"},
	"you_misc_event":      {"generic_you"},
	"misc_when":           {"generic"},
	"misc_whenever_a":     {"generic"},
	"this_card_event":     {"generic_self"},
	"conditional_state":   {"state_check"},
	"until_eot_trigger":   {"eot_trigger"},
	"one_or_more_typed_event":    {"generic_typed"},
	"one_or_more_other_ally_event": {"generic_typed"},
	"another_creature_or_artifact_event": {"etb"},
	"legend_ally_event":   {"etb"},
	"group_attack":        {"attack"},
	"compound_opp_tribe_event": {"generic"},
	"compound_card_zone_event": {"generic"},
	"per_damage_prevented": {"damage_prevented"},
	"you_attack_player":   {"attack"},
	"attack_self_or_ally": {"attack"},
}
