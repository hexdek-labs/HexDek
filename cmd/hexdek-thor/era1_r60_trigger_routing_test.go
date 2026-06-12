package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestClassifyTrigger_Era1R60SweepRouting pins the Era 1 sweep additions:
// 4 routes to brand-new scaffolds (becomes_untapped, becomes_monstrous,
// tapped_for_mana, you_roll_dice) plus ~70 routes to existing scaffolds.
// Identical pattern to PR #447 (Era 2) and PR #451 (Era 3).
func TestClassifyTrigger_Era1R60SweepRouting(t *testing.T) {
	cases := map[string]string{
		// Four new scaffolds.
		"becomes_untapped":     "becomes_untapped",
		"becomes_monstrous":    "becomes_monstrous",
		"tapped_for_mana":      "tapped_for_mana",
		"tap_for_mana":         "tapped_for_mana",
		"land_tapped_for_mana": "tapped_for_mana",
		"you_roll_dice":        "you_roll_dice",

		// First-pass long tail.
		"block_or_becomes_blocked": "attacks",
		"block_creature":           "attacks",
		"becomes_blocked_by":       "attacks",
		"self_blocks":              "attacks",
		"tap_opp_creature":         "attacks",
		"self_deals_damage_player": "combat_damage",
		"you_dealt_damage":         "combat_damage",
		"per_damage_prevented":     "combat_damage",
		"opp_dealt_damage":         "combat_damage",
		"creature_etb_any":         "creature_etb",
		"land_etb_any":             "creature_etb",
		"artifact_etb":             "creature_etb",
		"any_typed_etb":            "creature_etb",
		"one_or_more_lands":        "creature_etb",
		"another_typed_etb":        "tribe_you_control_etb",
		"ally_typed_etb":           "ally_etb",
		"legend_ally_event":        "ally_etb",
		"you_scry":                 "draw_card",
		"you_surveil":              "draw_card",
		"to_gy_from_anywhere":      "creature_dies",
		"creature_cards_leave_gy":  "creature_dies",
		"any_player_loses_game":    "creature_dies",
		"any_cycle":                "discard",
		"you_put_counters_on":      "counters_put_on_self",
		"you_proliferate":          "counters_put_on_self",
		"any_player_sacs":          "sacrifice",
		"opp_activate":             "opp_creature_event",
		"opp_searches_library":     "opp_creature_event",
		"any_player_tap_land":      "upkeep",
		"self_phase_inout":         "upkeep",
		"until_eot_trigger":        "upkeep",
		"you_misc_event":           "when_you_do",
		"become_monarch":           "when_you_do",
		"you_expend_n":             "when_you_do",
		"self_ability_activated":   "when_you_do",

		// Second-pass phase-style + long tail.
		"end_step":                    "end_step",
		"upkeep":                      "upkeep",
		"untap_step":                  "untap_step",
		"each_upkeep":                 "upkeep",
		"cumulative_upkeep_unpaid":    "upkeep",
		"cumulative_upkeep_paid":      "upkeep",
		"until_next_phase":            "upkeep",
		"saga_final_chapter":          "upkeep",
		"ordinal_trigger":             "upkeep",
		"upkeep_life_leader":          "upkeep",
		"first_main":                  "upkeep",
		"becomes_renowned":            "becomes_monstrous",
		"evolve_event":                "counters_put_on_self",
		"counter_put_on_self":         "counters_put_on_self",
		"counters_removed_from_self":  "counters_put_on_self",
		"counters_put_on_actor_any":   "counters_put_on_self",
		"remove_last_counter":         "counters_put_on_self",
		"counter_threshold":           "counters_put_on_self",
		"proliferate":                 "counters_put_on_self",
		"flip":                        "player_wins_coin_flip",
		"aura_attached_event":         "creature_etb",
		"forest_etb":                  "creature_etb",
		"creature_etb":                "creature_etb",
		"power_threshold_etb":         "creature_etb",
		"compound_opponents_event":    "opp_creature_event",
		"opp_landfall":                "opp_creature_event",
		"opp_shuffle":                 "opp_creature_event",
		"compound_tribe_die_or_leave": "creature_dies",
		"self_card_zone_to_zone":      "creature_dies",
		"lose_control":                "creature_dies",
		"is_sac_or_destroyed":         "creature_dies",
		"permanent_returned":          "creature_dies",
		"self_enter_or_die":           "creature_dies",
		"exiled":                      "creature_dies",
		"activation_non_mana":         "when_you_do",
		"this_card_event":             "when_you_do",
		"chosen_color_mana_added":     "when_you_do",
		"tempting_offer":              "when_you_do",
		"vote":                        "when_you_do",
		"self_squad_action":           "when_you_do",
		"one_or_more_other_creatures": "etb_or_another",
		"one_or_more_ally_creatures":  "ally_etb",
		"paired_whenever":             "self_and",
		"opponents_dealt_combat_dmg":  "combat_damage",
		"becomes_saddled_first":       "self_saddles_mount",
		"you_sac_one_or_more":         "sacrifice",
		"you_control_7_thrulls":       "tribe_you_control_etb",
	}

	for ev, wantSlug := range cases {
		ev, wantSlug := ev, wantSlug
		t.Run(ev, func(t *testing.T) {
			tr := &gameast.Trigger{Event: ev}
			got := classifyTrigger(tr)
			if got != wantSlug {
				t.Errorf("classifyTrigger(%q): want %q, got %q", ev, wantSlug, got)
			}
			if _, ok := triggerConditionActions[wantSlug]; !ok {
				t.Errorf("triggerConditionActions[%q] missing — scaffold gap re-opened", wantSlug)
			}
		})
	}
}

// TestTriggerConditionActions_Era1R60Apply verifies each of the 4 new
// Era 1 scaffolds runs without panic and produces the expected state /
// event signature.
func TestTriggerConditionActions_Era1R60Apply(t *testing.T) {
	cases := []struct {
		slug      string
		wantEvent string
	}{
		{"becomes_untapped", "becomes_untapped"},
		{"becomes_monstrous", "becomes_monstrous"},
		{"tapped_for_mana", "tapped_for_mana"},
		{"you_roll_dice", "die_rolled"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.slug, func(t *testing.T) {
			gs := newTestGameState(2)
			gs.EventPolicy = gameengine.EventLogFull
			src := &gameengine.Permanent{
				Controller: 0,
				Flags:      map[string]int{},
			}
			triggerConditionActions[tc.slug].apply(gs, src)
			found := false
			for _, e := range gs.EventLog {
				if e.Kind == tc.wantEvent {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: event %q not logged", tc.slug, tc.wantEvent)
			}
		})
	}
	// becomes_monstrous + tapped_for_mana also stamp specific flags.
	t.Run("becomes_monstrous_stamps_flag", func(t *testing.T) {
		gs := newTestGameState(2)
		gs.EventPolicy = gameengine.EventLogFull
		src := &gameengine.Permanent{Controller: 0, Flags: map[string]int{}}
		triggerConditionActions["becomes_monstrous"].apply(gs, src)
		if src.Flags["monstrous"] != 1 {
			t.Errorf("monstrous flag not stamped: %v", src.Flags)
		}
	})
	t.Run("tapped_for_mana_taps_source", func(t *testing.T) {
		gs := newTestGameState(2)
		gs.EventPolicy = gameengine.EventLogFull
		src := &gameengine.Permanent{Controller: 0, Flags: map[string]int{}}
		triggerConditionActions["tapped_for_mana"].apply(gs, src)
		if !src.Tapped {
			t.Error("tapped_for_mana did not tap source")
		}
	})
}
