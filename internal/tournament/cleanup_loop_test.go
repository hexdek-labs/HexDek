package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// ---------------------------------------------------------------------------
// P1 #7: Cleanup Step Looping (CR §514.3a) Tests
// ---------------------------------------------------------------------------

func TestCleanupLoop_BasicTurnCompletes(t *testing.T) {
	// Verify a normal turn still completes without issues.
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Active = 0
	gs.Turn = 2

	// Give seat 0 some cards so it has something to do.
	for i := 0; i < 5; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
	}
	for i := 0; i < 5; i++ {
		gs.Seats[1].Library = append(gs.Seats[1].Library,
			&gameengine.Card{Name: "Island", Types: []string{"land"}, Owner: 1})
	}
	for i := 0; i < 3; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
	}

	// Take a turn — should not panic or infinite loop.
	TakeTurn(gs)

	// Verify the turn completed.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "turn_start" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected turn_start event")
	}
}

func TestCleanupLoop_SBADuringCleanupTriggersLoop(t *testing.T) {
	// This test verifies the loop happens when SBAs fire during cleanup.
	// Setup: a creature at exactly 0 toughness enters cleanup, SBA kills
	// it, which should trigger the cleanup loop (§514.3a).
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Active = 0
	gs.Turn = 2

	// Give both seats libraries so draw step works.
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
		gs.Seats[1].Library = append(gs.Seats[1].Library,
			&gameengine.Card{Name: "Island", Types: []string{"land"}, Owner: 1})
	}

	// Put a creature with a until-EOT buff that will expire during cleanup,
	// leaving it at 0 toughness.
	creature := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          "Fragile Thing",
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 0, // 0 base toughness
		},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Modifications: []gameengine.Modification{
			{Power: 0, Toughness: 1, Duration: "until_end_of_turn"},
		},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)

	TakeTurn(gs)

	// The creature should have been destroyed by SBAs during cleanup.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Fragile Thing" {
			t.Fatal("Fragile Thing should have been destroyed during cleanup")
		}
	}
}
