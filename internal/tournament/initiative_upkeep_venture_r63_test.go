package tournament

// r63 — CR §722: the initiative-holder ventures into the Undercity at the
// beginning of THEIR upkeep. This recurring upkeep venture was unwired (only
// the on-take venture in TakeInitiative fired). Driving a full turn for the
// initiative-holder must advance the dungeon by exactly one room.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

func TestInitiative_UpkeepVenture_DriverWired(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(11)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
		s.Hat = &hat.GreedyHat{}
		// A few library cards so untap/draw/etc. don't underflow.
		for i := 0; i < 12; i++ {
			s.Library = append(s.Library, &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"land", "forest"}})
		}
	}
	gs.Turn = 3
	gs.Active = 0

	// Seat 0 holds the initiative AT THE START of its turn (already on the
	// dungeon from a prior take). Clear the on-take venture's room so we
	// measure only the upkeep venture.
	gs.Seats[0].Flags = map[string]int{"has_initiative": 1}
	gs.Flags = map[string]int{"initiative_holder": 0}
	roomBefore := gs.Seats[0].Flags["dungeon_room"]

	takeTurnImpl(gs, nil)

	if got := gs.Seats[0].Flags["dungeon_room"]; got != roomBefore+1 {
		t.Fatalf("initiative-holder should venture once at its upkeep: dungeon_room %d → %d (want +1)", roomBefore, got)
	}
}
