package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func seatLandCount(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsLand() {
			n++
		}
	}
	return n
}

func permOnSeatBF(gs *gameengine.GameState, seat int, p *gameengine.Permanent) bool {
	for _, q := range gs.Seats[seat].Battlefield {
		if q == p {
			return true
		}
	}
	return false
}

func TestManaVortex_EachUpkeepSacrificesALand_PrefersBasic(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Command Tower", "land")
	addPerm(gs, 0, "Forest", "land") // basic — should be the one sacrificed
	vortex := addPerm(gs, 0, "Mana Vortex", "enchantment")

	manaVortexUpkeep(gs, vortex, map[string]interface{}{"active_seat": 0})

	if got := seatLandCount(gs, 0); got != 1 {
		t.Fatalf("seat 0 should have 1 land left after sacrifice, got %d", got)
	}
	// The basic Forest should be gone; the Command Tower should remain.
	survivedNonbasic := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsLand() && p.Card.Name == "Command Tower" {
			survivedNonbasic = true
		}
		if p != nil && p.IsLand() && p.Card.Name == "Forest" {
			t.Error("basic Forest should have been preferentially sacrificed")
		}
	}
	if !survivedNonbasic {
		t.Error("nonbasic Command Tower should have survived")
	}
}

func TestManaVortex_SelfSacrificesWhenNoLandsRemain(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Wastes", "land") // the only land anywhere
	vortex := addPerm(gs, 0, "Mana Vortex", "enchantment")

	manaVortexUpkeep(gs, vortex, map[string]interface{}{"active_seat": 0})

	if seatLandCount(gs, 0) != 0 {
		t.Fatalf("the only land should have been sacrificed")
	}
	if permOnSeatBF(gs, 0, vortex) {
		t.Error("Mana Vortex must sacrifice itself once no lands remain on the battlefield")
	}
}

func TestManaVortex_OpponentUpkeepSacrificesOpponentLand(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mana Vortex", "enchantment")
	addPerm(gs, 0, "Island", "land")
	addPerm(gs, 1, "Mountain", "land")
	vortex := gs.Seats[0].Battlefield[0]

	// Opponent's (seat 1) upkeep: seat 1 sacrifices their own land.
	manaVortexUpkeep(gs, vortex, map[string]interface{}{"active_seat": 1})

	if seatLandCount(gs, 1) != 0 {
		t.Errorf("seat 1 should have sacrificed their land on their own upkeep")
	}
	if seatLandCount(gs, 0) != 1 {
		t.Errorf("seat 0's land must be untouched on seat 1's upkeep")
	}
}
