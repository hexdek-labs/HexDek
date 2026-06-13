package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Wheel of Torture: each opponent's upkeep, deal (3 - their hand size) damage.
// Previously inert (trigger_tail_fragment).

func TestWheelOfTorture_DamagesEmptyHandOpponent(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Wheel of Torture", "artifact")
	gs.Seats[1].Hand = nil // 0 cards → 3 damage
	life := gs.Seats[1].Life

	fireUpkeep(gs, 1)
	if gs.Seats[1].Life != life-3 {
		t.Fatalf("expected 3 damage to empty-handed opponent, life %d→%d", life, gs.Seats[1].Life)
	}
}

func TestWheelOfTorture_NoDamageWhenHandFull(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Wheel of Torture", "artifact")
	// Opponent holds exactly 3 cards → 3-3 = 0 damage.
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "a", Owner: 1}, {Name: "b", Owner: 1}, {Name: "c", Owner: 1},
	}
	life := gs.Seats[1].Life
	fireUpkeep(gs, 1)
	if gs.Seats[1].Life != life {
		t.Fatalf("expected no damage when hand has 3 cards (3-3=0), life changed to %d", gs.Seats[1].Life)
	}
}

func TestWheelOfTorture_DoesNotHitController(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Wheel of Torture", "artifact")
	gs.Seats[0].Hand = nil
	life := gs.Seats[0].Life
	fireUpkeep(gs, 0) // controller's own upkeep
	if gs.Seats[0].Life != life {
		t.Fatalf("Wheel of Torture must not damage its controller, life changed to %d", gs.Seats[0].Life)
	}
}
