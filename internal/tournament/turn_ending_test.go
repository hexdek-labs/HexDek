package tournament

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	// Blank import to install per-card hooks (Sundial, etc.)
	_ "github.com/hexdek/hexdek/internal/gameengine/per_card"
	"github.com/hexdek/hexdek/internal/hat"
)

// =============================================================================
// Turn-ending fast-forward tests (Sundial of the Infinite, CR §712.5).
// =============================================================================

func TestTurnEnding_FlagConsumedByTurnLoop(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := gameengine.NewGameState(2, rng, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Turn = 2
	gs.Active = 0

	// Give each seat some library cards.
	for i := 0; i < 20; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
			Name: "Card", Owner: 0, Types: []string{"creature"}, CMC: 1,
		})
		gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{
			Name: "Card", Owner: 1, Types: []string{"creature"}, CMC: 1,
		})
	}

	// Set the turn_ending_now flag mid-turn (simulating Sundial activation).
	gs.Flags["turn_ending_now"] = 1

	TakeTurn(gs)

	// The flag should have been consumed.
	if gs.Flags["turn_ending_now"] != 0 {
		t.Fatal("turn_ending_now flag should be consumed after fast-forward")
	}

	// The turn should have ended at cleanup (phase = "ending", step = "cleanup").
	if gs.Phase != "ending" || gs.Step != "cleanup" {
		t.Fatalf("expected phase=ending step=cleanup, got phase=%s step=%s",
			gs.Phase, gs.Step)
	}

	// Should see the fast-forward event.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "turn_ending_fast_forward" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing turn_ending_fast_forward event")
	}
}

func TestTurnEnding_SkipsEndStepTriggers(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := gameengine.NewGameState(2, rng, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Turn = 2
	gs.Active = 0

	for i := 0; i < 20; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
			Name: "Card", Owner: 0, Types: []string{"creature"}, CMC: 1,
		})
		gs.Seats[1].Library = append(gs.Seats[1].Library, &gameengine.Card{
			Name: "Card", Owner: 1, Types: []string{"creature"}, CMC: 1,
		})
	}

	// Register a delayed trigger that would fire at end_of_turn.
	triggerFired := false
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: 0,
		SourceCardName: "Final Fortune",
		EffectFn: func(gs *gameengine.GameState) {
			triggerFired = true
		},
	})

	// Set the flag before upkeep so it fires early.
	gs.Flags["turn_ending_now"] = 1

	TakeTurn(gs)

	// The end-of-turn trigger should NOT have fired because the turn
	// was fast-forwarded past the end step.
	if triggerFired {
		t.Fatal("end_of_turn delayed trigger should not fire when turn is ended early")
	}
}

// =============================================================================
// Sundial "your turn only" restriction test.
// =============================================================================

func TestSundial_NotYourTurn_Blocked(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := gameengine.NewGameState(2, rng, nil)
	gs.Active = 1 // It's seat 1's turn.

	// Create a Sundial permanent controlled by seat 0.
	perm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Sundial of the Infinite", Owner: 0},
		Controller: 0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	// Try to activate Sundial via the per-card hook.
	gameengine.InvokeActivatedHook(gs, perm, 0, nil)

	// The activation should have been blocked — turn_ending_now should NOT be set.
	if gs.Flags != nil && gs.Flags["turn_ending_now"] > 0 {
		t.Fatal("Sundial should not activate on opponent's turn")
	}

	// Should see a per_card_failed event.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_failed" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing per_card_failed event for Sundial not-your-turn")
	}
}
