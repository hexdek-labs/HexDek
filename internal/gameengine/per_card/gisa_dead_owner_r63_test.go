package per_card

// gisa_dead_owner_r63_test.go — CONSERVATION residual class (r63),
// seed-7 game 1546: Malcator's Watcher died in combat, the same death
// eliminated its owner (CheckEnd runs inside the trigger batch), and
// Gisa's "exile it instead" trigger then lifted the §800.4a-ceased card
// from the dead seat's graveyard into HER controller's exile — a
// present-but-ceased fabrication from that turn on. The handler must
// refuse cards whose owner has left the game (the object no longer
// exists), and the upkeep reanimation must skip dead-owner pile entries.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func gisaDeadOwnerFixture(t *testing.T) (*gameengine.GameState, *gameengine.Permanent, *gameengine.Card) {
	t.Helper()
	Reset()
	gs := gameengine.NewGameState(2, nil, nil)

	gisaCard := &gameengine.Card{
		Name: "Gisa, Glorious Resurrector", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	gisa := &gameengine.Permanent{
		Card: gisaCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, gisa)

	victim := &gameengine.Card{
		Name: "Malcator's Watcher", Owner: 1,
		Types: []string{"creature"},
	}
	gameengine.MintOGInstanceID(gs, victim)
	return gs, gisa, victim
}

func TestGisaResurrector_DeadOwner_DoesNotStealCeasedCard(t *testing.T) {
	gs, gisa, victim := gisaDeadOwnerFixture(t)

	// The death routed the card to its owner's graveyard, then the same
	// death eliminated the owner (§800.4a ceased the ID; the pointer
	// stays in the dead graveyard for forensics).
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, victim)
	gameengine.HandleSeatElimination(gs, 1)
	if _, ceased := gs.CeasedInstanceIDs[victim.InstanceID]; !ceased {
		t.Fatal("fixture: elimination must cease the dead owner's graveyard card")
	}

	gisaResurrectorDies(gs, gisa, map[string]interface{}{
		"card":            victim,
		"controller_seat": 1,
	})

	for _, c := range gs.Seats[0].Exile {
		if c == victim {
			t.Fatal("Gisa stole a §800.4a-ceased card into a living seat's exile — present-but-ceased fabrication")
		}
	}
	if err, ok := gameengine.ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must stay clean: ok=%v err=%v", ok, err)
	}
}

func TestGisaResurrector_Upkeep_SkipsDeadOwnerPileEntries(t *testing.T) {
	gs, gisa, victim := gisaDeadOwnerFixture(t)

	// Captured while the owner was alive...
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, victim)
	gisaResurrectorDies(gs, gisa, map[string]interface{}{
		"card":            victim,
		"controller_seat": 1,
	})
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c == victim {
			found = true
		}
	}
	if !found {
		t.Fatal("fixture: live-owner capture should land in Gisa's exile")
	}

	// ...then the owner is eliminated before Gisa's upkeep. The Phase E
	// sweep ceases + purges the zone copy; the side pile still holds the
	// raw pointer.
	gameengine.HandleSeatElimination(gs, 1)

	gisaResurrectorUpkeep(gs, gisa, map[string]interface{}{"active_seat": 0})

	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == victim {
			t.Fatal("Gisa reanimated a §800.4a-ceased card from her side pile")
		}
	}
	if err, ok := gameengine.ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must stay clean: ok=%v err=%v", ok, err)
	}
}
