package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Seizan, Perverter of Truth: each player's upkeep, that player loses 2 life
// and draws two. Previously inert (trigger_fragment_upkeep).

func fireUpkeep(gs *gameengine.GameState, activeSeat int) {
	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{
		"active_seat": activeSeat,
	})
}

func TestSeizan_DrainsAndDrawsActivePlayer(t *testing.T) {
	gs := newGame(t, 2)
	s := addPerm(gs, 0, "Seizan, Perverter of Truth", "creature", "legendary")
	s.Card.BasePower = 8
	s.Card.BaseToughness = 8 // nonzero so SBA does not kill it when its upkeep trigger resolves
	addLibrary(gs, 0, "A", "B", "C")
	addLibrary(gs, 1, "X", "Y", "Z")

	life0 := gs.Seats[0].Life
	hand0 := len(gs.Seats[0].Hand)

	// Controller's own upkeep: controller drains 2 and draws 2.
	fireUpkeep(gs, 0)
	if gs.Seats[0].Life != life0-2 {
		t.Fatalf("expected controller to lose 2 life, got %d (was %d)", gs.Seats[0].Life, life0)
	}
	if len(gs.Seats[0].Hand) != hand0+2 {
		t.Fatalf("expected controller to draw 2, hand delta=%d", len(gs.Seats[0].Hand)-hand0)
	}
}

func TestSeizan_SymmetricHitsOpponentUpkeep(t *testing.T) {
	gs := newGame(t, 2)
	s := addPerm(gs, 0, "Seizan, Perverter of Truth", "creature", "legendary")
	s.Card.BasePower = 8
	s.Card.BaseToughness = 8
	addLibrary(gs, 1, "X", "Y", "Z")

	life1 := gs.Seats[1].Life
	hand1 := len(gs.Seats[1].Hand)

	// Opponent's upkeep: the OPPONENT drains 2 and draws 2 (symmetric).
	fireUpkeep(gs, 1)
	if gs.Seats[1].Life != life1-2 {
		t.Fatalf("expected opponent to lose 2 life on their upkeep, got %d", gs.Seats[1].Life)
	}
	if len(gs.Seats[1].Hand) != hand1+2 {
		t.Fatalf("expected opponent to draw 2 on their upkeep, hand delta=%d", len(gs.Seats[1].Hand)-hand1)
	}
}
