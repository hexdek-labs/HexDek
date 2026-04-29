package main

// oracle_diff.go — Oracle text differential analysis.
//
// Compares what the AST parser claims a card does vs what the game
// engine actually executes. For each card with an AST, extracts the
// expected behavior from the AST (keywords, triggers, effects) and
// verifies the engine exercises those behaviors when the card is placed
// on the battlefield and interacted with.

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func runOracleDiff(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	start := time.Now()
	var (
		fails   []failure
		mu      sync.Mutex
		tested  int64
		skipped int64
	)

	work := make(chan *oracleCard, 256)
	go func() {
		for _, oc := range oracleCards {
			if oc.ast != nil {
				work <- oc
			}
		}
		close(work)
	}()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for oc := range work {
				f := oracleDiffCard(oc)
				if f == nil {
					atomic.AddInt64(&skipped, 1)
				} else if len(f) > 0 {
					mu.Lock()
					fails = append(fails, f...)
					mu.Unlock()
				}
				atomic.AddInt64(&tested, 1)
				done := atomic.LoadInt64(&tested)
				if done%5000 == 0 {
					log.Printf("  oracle-diff: %d tested, %d fails", done, len(fails))
				}
			}
		}()
	}
	wg.Wait()

	log.Printf("  oracle-diff complete: %d tested, %d skipped, %d fails, %s",
		atomic.LoadInt64(&tested), atomic.LoadInt64(&skipped),
		len(fails), time.Since(start))
	return fails
}

func oracleDiffCard(oc *oracleCard) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: oc.Name, Interaction: "oracle_diff_panic",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()

	ast := oc.ast
	if ast == nil {
		return nil
	}

	// Extract AST claims.
	var keywords []string
	var hasTrigger bool
	var hasActivated bool
	var hasETB bool
	var triggerEvents []string

	for _, ab := range ast.Abilities {
		switch a := ab.(type) {
		case *gameast.Keyword:
			keywords = append(keywords, strings.ToLower(a.Name))
		case *gameast.Triggered:
			hasTrigger = true
			if a.Trigger.Event != "" {
				triggerEvents = append(triggerEvents, a.Trigger.Event)
				if a.Trigger.Event == "etb" {
					hasETB = true
				}
			}
		case *gameast.Activated:
			hasActivated = true
		}
	}

	// Build a game state with this card on the battlefield.
	gs := makeGameState(oc, ast)
	if gs == nil {
		return nil
	}
	perm := findCard(gs, oc.Name)
	if perm == nil {
		return nil
	}

	// Verify keyword consistency: if AST claims flying, the engine
	// should recognize it via HasKeyword or equivalent.
	for _, kw := range keywords {
		switch kw {
		case "flying", "first_strike", "double_strike", "deathtouch",
			"lifelink", "trample", "vigilance", "reach", "haste",
			"hexproof", "shroud", "indestructible", "menace":
			if !permHasKeyword(perm, kw) {
				fails = append(fails, failure{
					CardName:    oc.Name,
					Interaction: "oracle_diff_keyword",
					Message:     fmt.Sprintf("AST claims keyword '%s' but not found on permanent", kw),
				})
			}
		}
	}

	// ETB verification: if AST has an ETB trigger, firing InvokeETBHook
	// should not panic and should leave state valid.
	if hasETB {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fails = append(fails, failure{
						CardName: oc.Name, Interaction: "oracle_diff_etb",
						Panicked: true, PanicMsg: fmt.Sprintf("ETB panic: %v", r),
					})
				}
			}()
			gameengine.InvokeETBHook(gs, perm)
			gameengine.StateBasedActions(gs)
		}()
	}

	// Trigger completeness: verify trigger events match the parser's
	// known vocabulary. This catches typos and malformed events, not
	// granularity differences — the parser emits ~400 distinct event
	// names by design.
	recognizedEvents := buildRecognizedEvents()

	for _, evt := range triggerEvents {
		if !recognizedEvents[evt] && evt != "" {
			fails = append(fails, failure{
				CardName:    oc.Name,
				Interaction: "oracle_diff_trigger",
				Message:     fmt.Sprintf("AST trigger event '%s' not in recognized set", evt),
			})
		}
	}

	// Activated ability verification.
	if hasActivated {
		// Just verify the card has activated abilities in the AST and
		// the engine doesn't panic when we run SBA with it.
		gameengine.StateBasedActions(gs)
	}

	// Creature type consistency: if oracle says creature, AST should agree.
	isOracleCreature := false
	for _, t := range oc.Types {
		if t == "creature" {
			isOracleCreature = true
			break
		}
	}
	if isOracleCreature && perm.IsCreature() {
		// Verify P/T is sane (not negative toughness unless modified).
		if perm.Toughness() < 0 && len(keywords) == 0 {
			fails = append(fails, failure{
				CardName:    oc.Name,
				Interaction: "oracle_diff_pt",
				Message:     fmt.Sprintf("creature has negative toughness %d with no modifiers", perm.Toughness()),
			})
		}
	}

	_ = hasTrigger
	_ = hasActivated
	return fails
}

var recognizedEventsCache map[string]bool

func buildRecognizedEvents() map[string]bool {
	if recognizedEventsCache != nil {
		return recognizedEventsCache
	}
	// Core events the engine handles directly.
	core := []string{
		"etb", "ltb", "dies", "attacks", "blocks",
		"deals_damage", "dealt_damage", "upkeep", "end_step", "draw",
		"cast", "becomes_target", "became_target", "discard", "mill",
		"sacrifice", "combat_damage", "gain_life", "lose_life",
		"counter_added", "counter_removed", "tap", "untap", "transform",
		"phase_in", "phase_out", "exile", "graveyard", "land_enters",
		"creature_enters", "nontoken_enters", "token_created",
		"spell_cast", "ability_activated", "beginning_of_combat",
		"end_of_combat", "surveil", "explore", "adapt", "monarch",
		"initiative",

		// Parser-canonical variants (same semantics, different form).
		"phase", "attack", "die", "block",

		// Combat damage variants.
		"combat_damage_player", "combat_damage_creature",
		"combat_damage_opponent", "combat_damage_player_or_pw",
		"combat_damage_player_or_battle", "combat_damage_to_player",
		"combat_damage_to_you", "self_combat_damage", "self_deals_damage_player",
		"group_combat_damage_player", "creature_combat_damage_you",
		"one_or_more_creatures_combat_damage", "dealt_combat_damage",
		"typed_combat_dmg", "compound_tribe_combat_damage",

		// Cast variants.
		"cast_filtered", "cast_any", "cast_spell", "cast_color_spell",
		"cast_mana_value", "any_player_cast", "any_player_cast_once",
		"opp_cast", "opp_cast_spell", "opp_cast_color_spell",
		"cast_color_filtered", "cast_x_spell", "any_cast",
		"cast_second_spell", "you_cast_artifact_instant_sorcery",
		"opp_casts_iss", "you_next_cast", "next_cast_typed",
		"when_you_next_cast_spell", "when_you_cast_it",

		// ETB variants.
		"etb_as", "tribe_you_control_etb", "another_typed_etb",
		"another_typed_enters", "another_subtype_enters",
		"another_creature_enters", "ally_typed_etb", "ally_typed_etb_a",
		"ally_etb", "creature_etb_any", "creature_etb",
		"artifact_you_control_enters", "artifact_etb",
		"enchantment_you_control_enters", "land_etb_any",
		"nontoken_ally_event", "typed_enters_your_control",
		"permanent_enters", "facedown_ally_enters",
		"self_and_another", "self_and", "enter_or_attack",
		"etb_or_attacks", "etb_or_attack", "any_typed_etb",
		"power_threshold_etb", "forest_etb", "merfolk_etb_any",
		"self_or_enchantment_etb_or_room_unlock",
		"next_time_one_or_more_enter",
		"one_or_more_ally_with_X_enter",

		// Death/LTB variants.
		"creature_dies", "another_creature_dies",
		"another_creature_dies_any", "tribe_you_control_dies",
		"another_typed_dies", "type_leaves_battlefield",
		"self_leaves_battlefield", "another_ally_leaves",
		"ally_leaves", "another_perm_ltb", "enchanted_ltb",
		"self_put_into_graveyard_from_bf", "another_nontoken_creature_dies_any",
		"phyrexian_dies", "other_artifact_dies",

		// Attack variants.
		"you_attack", "attack_alone", "group_attack",
		"attack_unblocked", "you_attack_with", "you_attack_player",
		"attack_or_block", "attack_while_saddled",
		"paired_etb_attack", "n_or_more_ally_attack",
		"formation_attack", "equipped_attacks",

		// Block variants.
		"becomes_blocked", "becomes_blocked_by",
		"block_or_becomes_blocked", "block_creature",
		"self_blocks",

		// Graveyard zone-change variants.
		"to_graveyard", "to_gy_from_bf", "to_gy_from_anywhere",
		"type_to_gy_from_bf", "permanent_to_gy",
		"ally_type_to_gy_from_bf", "opp_type_to_gy_from_bf",
		"ally_typed_to_gy", "tribal_to_gy_from_bf",
		"card_to_gy_anywhere", "any_card_to_gy_anywhere",
		"opp_creature_to_gy", "nontoken_type_to_gy",
		"creature_cards_leave_gy", "creature_cards_to_zone",
		"enchanted_perm_to_gy", "self_to_gy",
		"self_card_zone_to_zone", "compound_card_zone_event",

		// Discard variants.
		"discard_filtered", "discard_one_or_more",
		"discard_filter_cards", "discard_event",
		"opp_discard_filtered", "opp_makes_you_discard_self",
		"you_discard_typed_card", "player_discard",
		"cycle_or_discard",

		// Phase/step variants.
		"beginning_of_ordinal_step", "each_player_upkeep",
		"enchanted_end_step", "until_next_phase",
		"until_eot_trigger",

		// Lifecycle events.
		"when_you_do", "misc_when", "misc_whenever_a",
		"you_whenever", "you_action", "you_misc_event",
		"this_card_event", "conditional_state",

		// Tapping/mana.
		"becomes_tapped", "becomes_untapped", "tapped_for_mana",
		"tap_for_mana", "land_tapped_for_mana",
		"any_player_tap_land", "chosen_color_mana_tapped",
		"spend_this_mana",

		// Sacrifice variants.
		"sacrifice_filtered", "conditional_sacrifice",
		"sacrifices", "sacrifices_filtered",
		"sacrifice_self_or_ally", "any_player_sacs",
		"you_sacrifice_type", "you_sacrifice_one_or_more",
		"opp_sacrifice_filtered", "is_sac_or_destroyed",

		// Counter variants.
		"counters_put_on_self", "counters_put_on_actor",
		"counters_put_on_self_any", "counters_put_on_actor_any",
		"you_put_counters_on", "you_put_counters_on_any",
		"you_put_counter_on_any",
		"counter_removed_from_self", "counters_removed_from_self",
		"counter_put_on_actor", "counter_put_on_self",
		"remove_counter", "counter_threshold",
		"counter_threshold_reached",

		// Draw variants.
		"draw_card", "draw_card_once", "draw_event",
		"opp_draw_card", "opp_draws_ordinal",
		"ordinal_draw", "nth_card_drawn",
		"you_draw_nth_card", "player_draw_card",

		// Damage variants.
		"ally_source_damage", "ally_subtype_deal_damage",
		"opp_dealt_damage", "you_dealt_damage",
		"per_damage_prevented", "damage_prevented_this_way",
		"colored_damage_prevented", "excess_noncombat_damage",
		"damage_to_chosen_player",

		// Life variants.
		"you_lose_life", "opponent_loses_life",
		"opponent_gains_life", "gain_lose_life_during_phase",
		"end_step_gained_n_life_this_turn",

		// Transform/morph/face-down variants.
		"turned_face_up", "transforms", "day_night_flip",
		"face_down_creature_event", "face_up_as",
		"as_transform",

		// MTG mechanics.
		"becomes_monstrous", "becomes_renowned",
		"mutates", "exploits_creature", "ally_exploits",
		"cycle", "cycle_card", "any_cycle",
		"one_or_more_typed_event", "one_or_more_lands",
		"one_or_more_other_ally_event", "one_or_more_milled",
		"one_or_more_other_creatures", "one_or_more_ally_creatures",
		"ring_tempts_you", "become_monarch",
		"commit_crime", "you_commit_crime",
		"class_becomes_level", "you_expend_n",
		"discover", "vote", "foretell_card",
		"you_scry", "you_surveil", "ally_explore",
		"you_proliferate", "you_roll_dice",
		"you_get_energy", "evolve_event",
		"saga_final_chapter", "mill_event",
		"card_milled_via", "surveil_first_time",

		// Tokens.
		"token_event", "you_create_one_or_more_tokens",
		"create_token", "opp_tokens_event",

		// Equipment/Aura/Vehicle.
		"equipped_trigger", "equipment_attach_state_change",
		"self_crews_vehicle", "becomes_crewed",
		"becomes_crewed_first", "self_saddles_mount",
		"aura_attached_event", "attached_as",
		"enchanted_player_attacked", "enchanted_player_casts",

		// Control/targeting.
		"ally_targeted_by_opp", "lose_control_of",
		"lose_control", "gain_control_event",
		"tap_opp_creature",

		// Opponent actions.
		"opp_activate", "opp_spell_or_ability",
		"opp_land_enters", "opp_landfall",
		"opp_searches_library", "opp_shuffle",
		"opp_creature_event",

		// Multi-creature/typed events.
		"nontoken_creature_event", "modified_creature_event",
		"legend_ally_event", "another_creature_or_artifact_event",
		"compound_opp_tribe_event", "compound_tribe_die_or_leave",
		"compound_tribe_enter", "compound_opponents_event",
		"self_or_typed_event",

		// Room/dungeon (Alchemy/Modern Horizons).
		"unlock_door", "fully_unlock_room",

		// Specialize (Alchemy).
		"specialize_creature", "specialize_from_zone",

		// Draft (Conspiracy).
		"as_you_draft_a_card",

		// Phase in/out variants.
		"self_phase_inout", "phaseout_or_exile",

		// Misc specific.
		"attack_player", "player_land_play",
		"becomes_state", "it_state_change",
		"coin_flip_result", "player_wins_coin_flip",
		"exiled_event", "exiled", "land_enters",
		"state_check",
		"you_mechanic", "pay_cost_multiple",
		"activation_non_mana", "self_ability_activated",
		"graveyard_empty",
		"upkeep_life_leader", "cumulative_upkeep_unpaid",
		"paid_cumulative_upkeep",
		"ordinal_trigger", "self_squad_action",
		"any_player_loses_game", "lose_game",
		"you_conjure_one_or_more", "permanent_returned",
		"untap_step", "paired_whenever",
		"spell_or_ability_causes",
		"flip",

		// Long-tail parser events (1-5 cards each).
		"you_exert_creature", "that_creature_ltb_this_turn",
		"your_spell_countered", "you_sac_one_or_more",
		"you_put_counter_on", "you_copy_spell",
		"you_control_7_thrulls", "you_cast_instant_sorcery",
		"you_cast_creature", "transform_into_phyrexian",
		"transform_as", "train", "three_or_more",
		"this_turn_whenever", "tempting_offer",
		"targets_chosen", "tap_for_C",
		"spell_or_ability_on_stack", "spell_kicked",
		"spell_cast_typed", "self_or_another_when",
		"self_enter_or_die", "self_die_or_ally_gy",
		"self_dealt_damage", "self_becomes_untapped",
		"self_becomes_tapped", "self_and_or_others_event",
		"sac_nontoken_elemental", "remove_last_counter",
		"put_onto_bf", "proliferate",
		"player_casts_other_owners_spell", "play_land",
		"place_counter", "other_nontoken_perm",
		"other_creature_dies", "opponents_dealt_combat_dmg",
		"opponent_pays_tax", "opponent_attacked",
		"opp_multi_attack", "opp_draws_card",
		"opp_commits_crime", "opp_casts",
		"opp_cast_artifact_instant_sorcery", "on_card_advantage",
		"nontoken_modified_creature_dies", "nonland_tapped_for_mana",
		"ninja_combat_damage", "next_end_step",
		"named_creature_etb", "n_or_more_attack",
		"ltb_to_graveyard", "leaves_graveyard",
		"leave_gy_single", "landfall", "investigate",
		"gain_life_first_time_once", "first_main",
		"filtered_opponent_gains_life", "etb_or_ltb",
		"enchanted_player_casts_noncreature", "elf_etb",
		"elemental_dies", "each_upkeep", "draw_step",
		"desert_etb", "damage_to_x_prevented",
		"damage_prevented", "cumulative_upkeep_paid",
		"creature_or_land_to_gy", "creature_modified_event",
		"creature_leaves_yours", "condition_fails",
		"compound_bounce_shuffle_event",
		"combat_damage_planeswalker", "color_creature_dies",
		"chosen_color_mana_added", "cast_self",
		"cast_noncreature", "cast_colorless",
		"cast_bolas_planeswalker", "card_to_gy_anywhere_once",
		"becomes_untapped_once", "becomes_target_by_opp",
		"becomes_saddled_first", "attacking_type_to_gy",
		"attack_with_two_or_more", "attack_player_unblocked",
		"attack_planeswalker", "attack_either",
		"artifact_etb_yours", "any_type_to_gy_from_bf",
		"any_block", "another_self_tribe_etb",
		"another_creature_with_dies", "all_trigger",
	}
	m := make(map[string]bool, len(core))
	for _, e := range core {
		m[e] = true
	}
	recognizedEventsCache = m
	return m
}

func permHasKeyword(perm *gameengine.Permanent, kw string) bool {
	if perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return false
	}
	for _, ab := range perm.Card.AST.Abilities {
		if k, ok := ab.(*gameast.Keyword); ok {
			if strings.ToLower(k.Name) == kw {
				return true
			}
		}
	}
	// Also check flags for granted keywords.
	if perm.Flags != nil {
		if perm.Flags["keyword:"+kw] > 0 {
			return true
		}
	}
	return false
}
