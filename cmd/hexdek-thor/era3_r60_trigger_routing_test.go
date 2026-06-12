package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestClassifyTrigger_Era3R60SweepRouting pins all 35 new Era 3 event slugs
// surfaced by scripts/era3_scaffold_audit.py: 5 routes to brand-new
// scaffolds (mutates / turned_face_up / exploits_creature /
// specialize_creature / unlock_door) plus 30 routes to existing scaffolds.
// Paired "scaffold still registered" assertion catches any future refactor
// that deletes a scaffold and silently re-opens the Era 3 gap.
func TestClassifyTrigger_Era3R60SweepRouting(t *testing.T) {
	cases := map[string]string{
		// Five new scaffolds for dominant Era 3 mechanics.
		"mutates":             "mutates",
		"turned_face_up":      "turned_face_up",
		"face_up_as":          "turned_face_up",
		"as_transform":        "turned_face_up",
		"exploits_creature":   "exploits_creature",
		"ally_exploits":       "exploits_creature",
		"specialize_creature": "specialize_creature",
		"unlock_door":         "unlock_door",
		"fully_unlock_room":   "unlock_door",

		// Long-tail routing — each to an existing scaffold.
		"becomes_target":               "attacks",
		"ally_targeted_by_opp":         "attacks",
		"becomes_blocked":              "attacks",
		"dealt_damage":                 "combat_damage",
		"deals_damage":                 "combat_damage",
		"damage_prevented_this_way":    "combat_damage",
		"ally_source_damage":           "combat_damage",
		"remove_counter":               "counters_put_on_self",
		"counter_put_on_actor":         "counters_put_on_self",
		"counters_put_on_self_any":     "counters_put_on_self",
		"counters_put_on_actor":        "counters_put_on_self",
		"creature_modified_event":      "counters_put_on_self",
		"card_put_into_zone":           "creature_dies",
		"permanent_to_gy":              "creature_dies",
		"card_milled_via":              "creature_dies",
		"compound_card_zone_event":     "creature_dies",
		"foretell_card":                "cast_spell",
		"attached_as":                  "creature_etb",
		"equipped_trigger":             "creature_etb",
		"day_night_flip":               "creature_etb",
		"transforms":                   "creature_etb",
		"next_time_one_or_more_enter":  "creature_etb",
		"cycle_card":                   "discard",
		"you_commit_crime":             "when_you_do",
		"commit_crime":                 "when_you_do",
		"pay_cost_multiple":            "when_you_do",
		"misc_whenever_a":              "when_you_do",
		"you_conjure_one_or_more":      "when_you_do",
		"you_mechanic":                 "when_you_do",
		"self_or_another_when":         "etb_or_another",
		"becomes_state":                "becomes_tapped_trigger",
		"player_land_play":             "upkeep",
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

// TestTriggerConditionActions_Era3R60Apply verifies each of the 5 new
// scaffolds runs without panic and mutates state in a recognisable
// direction (flag stamped + event logged).
func TestTriggerConditionActions_Era3R60Apply(t *testing.T) {
	cases := []struct {
		slug      string
		wantFlag  string // "" to skip flag check
		wantEvent string
	}{
		{"mutates", "mutated", "mutate"},
		{"turned_face_up", "turned_face_up", "turned_face_up"},
		{"exploits_creature", "", "exploits"},
		{"specialize_creature", "specialized", "specialize"},
		{"unlock_door", "room_unlocked", "unlock_door"},
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

			if tc.wantFlag != "" {
				if src.Flags[tc.wantFlag] != 1 {
					t.Errorf("%s: flag %q not stamped (flags=%v)",
						tc.slug, tc.wantFlag, src.Flags)
				}
			}
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
}
