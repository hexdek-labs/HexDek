package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestClassifyTrigger_Era2R60SweepRouting pins the long-tail event slugs
// surfaced by the r60 era 2 sweep. Each slug routes to an EXISTING
// scaffold rather than coining a new one; this test asserts both the
// slug routing AND that the destination scaffold remains registered, so
// a future refactor that deletes a scaffold can't silently re-open the
// gap.
func TestClassifyTrigger_Era2R60SweepRouting(t *testing.T) {
	cases := map[string]string{
		// First-tier — direct event-slug → scaffold mappings.
		"die":              "creature_dies",
		"to_graveyard":     "creature_dies",
		"etb_as":           "creature_etb",
		"cycle":            "discard",
		"block":            "attacks",
		"coin_flip_result": "player_wins_coin_flip",
		"lose_game":        "sacrifice",

		// Underscore-form combat damage family — the load-bearing fix.
		// Pre-sweep the prose "combat damage" substring missed these
		// because the parser emits underscores. These 5 slugs cover
		// 32/59 = 54% of the original Era 2 trigger gap.
		"combat_damage_player":        "combat_damage",
		"combat_damage_player_or_pw":  "combat_damage",
		"group_combat_damage_player":  "combat_damage",
		"combat_damage_opponent":      "combat_damage",
		"self_combat_damage":          "combat_damage",

		// Second-tier long tail — each routes via the rationale block in
		// classifyTrigger's switch.
		"beginning_of_ordinal_step": "upkeep",
		"token_event":               "creature_etb",
		"nontoken_ally_event":       "ally_etb",
		"nontoken_creature_event":   "creature_etb",
		"compound_opp_tribe_event":  "opp_creature_event",
		"one_or_more_typed_event":   "tribe_you_control_etb",
		"ally_explore":              "ally_etb",
		"self_and_another":          "self_and",
		"conditional_state":         "when_you_do",
		"misc_when":                 "when_you_do",
		"spend_this_mana":           "you_get_energy",
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
