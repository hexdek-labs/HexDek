package tournament

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	// Blank import installs the per-card hooks (Tymna's handlers).
	_ "github.com/hexdek/hexdek/internal/gameengine/per_card"
	"github.com/hexdek/hexdek/internal/hat"
)

// TestTymnaPostcombatDrawFiresInTurnLoop pins the postcombat-main-phase
// trigger surface end to end: the turn driver fires
// "postcombat_main_controller" at the main-2 boundary (turn.go MAIN
// PHASE 2 section) and Tymna the Weaver's per_card handler — which is
// fully implemented gameplay, not a stub — pays X life and draws X
// cards for the X opponents dealt combat damage this turn.
//
// Provenance: the 2026-05-30 engine-event-registry audit (CLAUDE.md
// Issue Log) claimed this event had no fire surface at all and that
// Tymna silently never triggers. The emission in the tournament turn
// driver in fact predates that audit (the audit only inspected
// gameengine.FirePhaseTriggers), but nothing pinned the end-to-end
// path, so the claim was undecidable from tests. Surfaced by the
// 2026-06-12 hexdek-judge CR-compliance sweep; this test is the
// adjudicator and the regression guard.
func TestTymnaPostcombatDrawFiresInTurnLoop(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := gameengine.NewGameState(2, rng, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Turn = 3
	gs.Active = 0

	for i := 0; i < 20; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
			Name: "Filler", Owner: 0, Types: []string{"creature"}, CMC: 1,
		})
		gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{
			Name: "Filler", Owner: 1, Types: []string{"creature"}, CMC: 1,
		})
	}

	tymna := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Tymna the Weaver", Owner: 0,
			Types:     []string{"legendary", "creature"},
			BasePower: 2, BaseToughness: 2, CMC: 3,
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{},
		// Pretend seat 1 was dealt combat damage this turn (the
		// damage-observer half stamps this from combat_damage_player;
		// here we pin the postcombat half in isolation).
		Flags: map[string]int{"tymna_hit_1": gs.Turn},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tymna)

	TakeTurn(gs)

	var drew bool
	var details map[string]interface{}
	for _, ev := range gs.EventLog {
		if ev.Kind != "per_card_handler" || ev.Details == nil {
			continue
		}
		if slug, _ := ev.Details["slug"].(string); slug == "tymna_the_weaver_postcombat_draw" {
			drew = true
			details = ev.Details
		}
	}
	if !drew {
		t.Fatal("Tymna postcombat draw never fired during TakeTurn — " +
			"postcombat_main_controller emission missing or per_card dispatch broken " +
			"(CLAUDE.md Issue Log 2026-05-30 engine-event-registry row)")
	}
	if paid, _ := details["paid"].(bool); !paid {
		t.Fatalf("Tymna trigger fired but declined payment: %v", details)
	}
	if x, _ := details["x"].(int); x != 1 {
		t.Errorf("Tymna X = %v, want 1 (one opponent hit this turn)", details["x"])
	}
}
