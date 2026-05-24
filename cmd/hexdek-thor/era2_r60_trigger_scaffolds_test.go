package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestClassifyTrigger_Era2R60 pins the 19 new exact-match trigger event
// slugs added in the R60 era 2 sweep. Each event name corresponds to a
// real Era 2 (2015-2019) trigger pattern surfaced by
// scripts/era2_scaffold_audit.py.
func TestClassifyTrigger_Era2R60(t *testing.T) {
	cases := map[string]string{
		"when_you_do":              "when_you_do",
		"counters_put_on_self":     "counters_put_on_self",
		"tribe_you_control_etb":    "tribe_you_control_etb",
		"self_crews_vehicle":       "self_crews_vehicle",
		"you_whenever":             "you_whenever",
		"ally_etb":                 "ally_etb",
		"ally_typed_etb_a":         "ally_typed_etb_a",
		"ally_subtype_deal_damage": "ally_subtype_deal_damage",
		"self_and":                 "self_and",
		"etb_or_another":           "etb_or_another",
		"opp_creature_event":       "opp_creature_event",
		"self_saddles_mount":       "self_saddles_mount",
		"becomes_tapped":           "becomes_tapped_trigger",
		"you_get_energy":           "you_get_energy",
		"player_wins_coin_flip":    "player_wins_coin_flip",
		"you_action":               "you_action",
		"becomes_crewed":           "becomes_crewed",
		"becomes_crewed_first":     "becomes_crewed_first",
		"opp_draw_card":            "opp_draw_card",
	}

	for ev, wantSlug := range cases {
		t.Run(ev, func(t *testing.T) {
			tr := &gameast.Trigger{Event: ev}
			got := classifyTrigger(tr)
			if got != wantSlug {
				t.Errorf("classifyTrigger(%q): want %q, got %q", ev, wantSlug, got)
			}
			if _, ok := triggerConditionActions[wantSlug]; !ok {
				t.Errorf("triggerConditionActions[%q] missing", wantSlug)
			}
		})
	}
}

// TestTriggerConditionActions_Era2R60Apply verifies each new action runs
// without panicking and mutates state in a recognizable direction.

func TestTriggerConditionActions_Era2R60Apply_WhenYouDo(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	triggerConditionActions["when_you_do"].apply(gs, nil)
	if gs.Seats[0].Flags["reflexive_action_done"] != 1 {
		t.Errorf("reflexive_action_done not stamped")
	}
	if !hasEvent(gs, "when_you_do") {
		t.Errorf("when_you_do event not logged")
	}
}

func TestTriggerConditionActions_Era2R60Apply_CountersPutOnSelf(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Walking Ballista", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	triggerConditionActions["counters_put_on_self"].apply(gs, src)
	if src.Counters["+1/+1"] != 1 {
		t.Errorf("+1/+1 counter not placed, got %d", src.Counters["+1/+1"])
	}
	if !hasEvent(gs, "counter_placed") {
		t.Errorf("counter_placed event not logged")
	}
}

func TestTriggerConditionActions_Era2R60Apply_TribeYouControlETB(t *testing.T) {
	gs := newTestGameState(2)
	triggerConditionActions["tribe_you_control_etb"].apply(gs, nil)
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, ty := range p.Card.Types {
			if ty == "wizard" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no wizard creature placed on seat 0")
	}
}

func TestTriggerConditionActions_Era2R60Apply_SelfCrewsVehicle(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Smuggler's Copter", Owner: 0, Types: []string{"artifact", "vehicle"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	triggerConditionActions["self_crews_vehicle"].apply(gs, src)
	if src.Flags["crewed_this_turn"] != 1 {
		t.Errorf("crewed_this_turn flag not set")
	}
	if !hasEvent(gs, "vehicle_crewed") {
		t.Errorf("vehicle_crewed event not logged")
	}
}

func TestTriggerConditionActions_Era2R60Apply_SelfSaddlesMount(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Bristly Bill, Spine Sower", Owner: 0, Types: []string{"creature", "mount"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	triggerConditionActions["self_saddles_mount"].apply(gs, src)
	if src.Flags["saddled_this_turn"] != 1 {
		t.Errorf("saddled_this_turn flag not set")
	}
	if len(src.SaddlersThisTurn) == 0 {
		t.Errorf("SaddlersThisTurn empty after saddle prime")
	}
}

func TestTriggerConditionActions_Era2R60Apply_YouGetEnergy(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	triggerConditionActions["you_get_energy"].apply(gs, nil)
	if gs.Seats[0].Flags["energy_counters"] < 3 {
		t.Errorf("energy_counters want >=3, got %d", gs.Seats[0].Flags["energy_counters"])
	}
	if !hasEvent(gs, "energy_gained") {
		t.Errorf("energy_gained event not logged")
	}
}

func TestTriggerConditionActions_Era2R60Apply_PlayerWinsCoinFlip(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	triggerConditionActions["player_wins_coin_flip"].apply(gs, nil)
	if !hasEvent(gs, "won_coin_flip") {
		t.Errorf("won_coin_flip event not logged")
	}
}

func TestTriggerConditionActions_Era2R60Apply_BecomesCrewed(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Heart of Kiran", Owner: 0, Types: []string{"artifact", "vehicle"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	triggerConditionActions["becomes_crewed"].apply(gs, src)
	if src.Flags["becomes_crewed"] != 1 {
		t.Errorf("becomes_crewed flag not set")
	}
}

func TestTriggerConditionActions_Era2R60Apply_BecomesCrewedFirst(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Cultivator of Blades", Owner: 0, Types: []string{"artifact", "vehicle"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	triggerConditionActions["becomes_crewed_first"].apply(gs, src)
	if src.Flags["becomes_crewed_first"] != 1 {
		t.Errorf("becomes_crewed_first flag not set")
	}
}

func TestTriggerConditionActions_Era2R60Apply_OppDrawCard(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	triggerConditionActions["opp_draw_card"].apply(gs, nil)
	if len(gs.Seats[1].Library) < 5 {
		t.Errorf("seat 1 library want >=5, got %d", len(gs.Seats[1].Library))
	}
	if gs.Seats[1].Flags["drew_card_this_turn"] != 1 {
		t.Errorf("opp drew_card_this_turn flag not set")
	}
}

func TestTriggerConditionActions_Era2R60Apply_AllyETBFamily(t *testing.T) {
	for _, slug := range []string{"ally_etb", "ally_typed_etb_a", "etb_or_another", "self_and"} {
		t.Run(slug, func(t *testing.T) {
			gs := newTestGameState(2)
			triggerConditionActions[slug].apply(gs, nil)
			creatures := 0
			for _, p := range gs.Seats[0].Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				for _, ty := range p.Card.Types {
					if ty == "creature" {
						creatures++
						break
					}
				}
			}
			if creatures < 1 {
				t.Errorf("slug %q: expected >=1 creature on seat 0, got %d", slug, creatures)
			}
		})
	}
}

func TestTriggerConditionActions_Era2R60Apply_AllySubtypeDealDamage(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	triggerConditionActions["ally_subtype_deal_damage"].apply(gs, nil)
	if !hasEvent(gs, "damage") {
		t.Errorf("damage event not logged")
	}
	hasWarrior := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, ty := range p.Card.Types {
			if ty == "warrior" {
				hasWarrior = true
			}
		}
	}
	if !hasWarrior {
		t.Errorf("no warrior placed on seat 0")
	}
}

func TestTriggerConditionActions_Era2R60Apply_OppCreatureEvent(t *testing.T) {
	gs := newTestGameState(2)
	triggerConditionActions["opp_creature_event"].apply(gs, nil)
	creatures := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, ty := range p.Card.Types {
			if ty == "creature" {
				creatures++
			}
		}
	}
	if creatures < 1 {
		t.Errorf("expected opponent (seat 1) to have a creature, got %d", creatures)
	}
}

func TestTriggerConditionActions_Era2R60Apply_BecomesTapped(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Insolence", Owner: 0, Types: []string{"enchantment"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	triggerConditionActions["becomes_tapped_trigger"].apply(gs, src)
	if !src.Tapped {
		t.Errorf("srcPerm should be tapped after becomes_tapped prime")
	}
	if !hasEvent(gs, "becomes_tapped") {
		t.Errorf("becomes_tapped event not logged")
	}
}

func TestTriggerConditionActions_Era2R60Apply_YouAction(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	triggerConditionActions["you_action"].apply(gs, nil)
	if gs.Seats[0].Flags["you_action"] != 1 {
		t.Errorf("you_action flag not set")
	}
	if !hasEvent(gs, "you_action") {
		t.Errorf("you_action event not logged")
	}
}

func TestTriggerConditionActions_Era2R60Apply_YouWhenever(t *testing.T) {
	gs := newTestGameState(2)
	gs.RetainEvents = true
	triggerConditionActions["you_whenever"].apply(gs, nil)
	if gs.Seats[0].Flags["you_whenever_active"] != 1 {
		t.Errorf("you_whenever_active flag not set")
	}
}

// hasEvent reports whether gs.EventLog contains an event with the given Kind.
func hasEvent(gs *gameengine.GameState, kind string) bool {
	for _, e := range gs.EventLog {
		if e.Kind == kind {
			return true
		}
	}
	return false
}
