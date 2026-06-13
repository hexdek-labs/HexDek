package per_card

// cross_seat_reanimate_dead_owner_r63_test.go — CONSERVATION residual
// class (r63), §800.4a sibling sweep of the Gisa fix (commit 67810ac9).
//
// A seat that has left the game keeps its zone slices POPULATED for
// forensics — only the card InstanceIDs are ceased (see
// gisa_dead_owner_r63_test.go and HandleSeatElimination's Phase 4
// cease loops). Any handler that scans EVERY player's graveyard and
// LIFTS a card onto the battlefield must therefore skip LeftGame seats,
// or it resurrects a §800.4a-ceased pointer — a present-but-ceased
// fabrication that the strict census flags every tick from that turn on.
//
// The Gisa commit audited the 96 creature_dies handlers; these are the
// ACTIVATED / triggered cross-seat reanimators that scan "a graveyard"
// (any player's) on a non-death surface. Pre-fix each test FAILS.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// deadOwnerGraveyardCreature eliminates seat `dead`, leaving one ceased
// creature card in its (forensically-retained) graveyard. Returns the
// card so the caller can assert it never reaches a living battlefield.
func deadOwnerGraveyardCreature(t *testing.T, gs *gameengine.GameState, dead int, name string, power int) *gameengine.Card {
	t.Helper()
	c := &gameengine.Card{
		Name: name, Owner: dead,
		Types:     []string{"creature"},
		BasePower: power, BaseToughness: power,
	}
	gameengine.MintOGInstanceID(gs, c)
	gs.Seats[dead].Graveyard = append(gs.Seats[dead].Graveyard, c)
	gameengine.HandleSeatElimination(gs, dead)
	if _, ceased := gs.CeasedInstanceIDs[c.InstanceID]; !ceased {
		t.Fatalf("fixture: elimination must cease the dead owner's graveyard card %q", name)
	}
	return c
}

func TestChainer_DeadOwnerGraveyard_NotReanimated(t *testing.T) {
	Reset()
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Life = 20

	chainerCard := &gameengine.Card{
		Name: "Chainer, Dementia Master", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	chainer := &gameengine.Permanent{
		Card: chainerCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, chainer)

	// Only candidate creature sits in the DEAD seat 1's graveyard.
	victim := deadOwnerGraveyardCreature(t, gs, 1, "Phyrexian Negator", 5)

	chainerReanimate(gs, chainer, 0, map[string]interface{}{})

	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == victim {
			t.Fatal("Chainer reanimated a §800.4a-ceased card from a dead seat's graveyard — present-but-ceased fabrication")
		}
	}
	if err, ok := gameengine.ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must stay clean after Chainer skips the dead seat: ok=%v err=%v", ok, err)
	}
}
