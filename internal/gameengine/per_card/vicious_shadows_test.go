package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Vicious Shadows: creature dies → deal damage = dead creature's controller's
// hand size to a target opponent. Previously inert (parsed_effect_residual).

func fireCreatureDies(gs *gameengine.GameState, controllerSeat int) {
	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"controller_seat": controllerSeat,
	})
}

func TestViciousShadows_DealsDamageByHandSize(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Vicious Shadows", "enchantment")
	// Controller (seat 0) holds 4 cards; their creature dies.
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "a", Owner: 0}, {Name: "b", Owner: 0}, {Name: "c", Owner: 0}, {Name: "d", Owner: 0},
	}
	life1 := gs.Seats[1].Life

	fireCreatureDies(gs, 0)
	if gs.Seats[1].Life != life1-4 {
		t.Fatalf("expected 4 damage (controller's hand) to opponent, life %d→%d", life1, gs.Seats[1].Life)
	}
}

func TestViciousShadows_TargetsLowestLifeOpponent(t *testing.T) {
	gs := newGame(t, 3)
	addPerm(gs, 0, "Vicious Shadows", "enchantment")
	gs.Seats[2].Hand = []*gameengine.Card{{Name: "x", Owner: 2}, {Name: "y", Owner: 2}}
	gs.Seats[1].Life = 30
	gs.Seats[2].Life = 5 // opponent 2 is lowest-life among 0's opponents... but seat 2's own creature dies

	// Seat 2's creature dies → damage = seat 2's hand (2). Target = lowest-life
	// opponent of the Vicious Shadows controller (seat 0): seats 1 (30) and 2 (5) → seat 2.
	l2 := gs.Seats[2].Life
	fireCreatureDies(gs, 2)
	if gs.Seats[2].Life != l2-2 {
		t.Fatalf("expected lowest-life opponent (seat 2) to take 2 damage, life %d→%d", l2, gs.Seats[2].Life)
	}
	if gs.Seats[1].Life != 30 {
		t.Fatalf("higher-life opponent should be untouched, got %d", gs.Seats[1].Life)
	}
}

func TestViciousShadows_NoDamageWhenEmptyHand(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Vicious Shadows", "enchantment")
	gs.Seats[0].Hand = nil
	life1 := gs.Seats[1].Life
	fireCreatureDies(gs, 0)
	if gs.Seats[1].Life != life1 {
		t.Fatalf("empty hand → 0 damage, but life changed to %d", gs.Seats[1].Life)
	}
}
