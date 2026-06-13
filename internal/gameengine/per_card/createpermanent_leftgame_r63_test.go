package per_card

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// createpermanent_leftgame_r63_test.go — seed-42 game 283 (the
// correctness score's CONSERVATION offender): Athreos, Shroud-Veiled
// returned an ELIMINATED player's coin-countered Gurmag Angler from the
// dead seat's graveyard onto a live battlefield via createPermanent —
// the per_card direct-append mover that bypasses MoveCard's #1041
// §800.4a guard (that fix's documented residual gap). The census then
// flagged the ceased InstanceID on the battlefield every tick.
//
// Pins: (1) the createPermanent chokepoint refuses dead-owner cards —
// covering all ~196 per_card callers; (2) the Athreos selector skips
// dead owners; (3) a living owner's card still works (no over-fire).

func mintedCreatureInGraveyard(t *testing.T, gs *gameengine.GameState, owner int, name string) *gameengine.Card {
	t.Helper()
	c := &gameengine.Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     5,
		BaseToughness: 5,
	}
	gameengine.MintOGInstanceID(gs, c)
	if c.InstanceID == "" {
		t.Fatalf("mint failed")
	}
	gs.Seats[owner].Graveyard = append(gs.Seats[owner].Graveyard, c)
	return c
}

func TestCreatePermanent_RefusesDeadOwnersCard(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(283)), nil)
	dead := mintedCreatureInGraveyard(t, gs, 1, "Gurmag Angler")

	// Eliminate seat 1 through the real §800.4a pipeline (ceases the
	// ID, keeps the graveyard pointer).
	gs.Seats[1].Lost = true
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"
	gameengine.HandleSeatElimination(gs, 1)

	// A live seat's handler tries to materialize the dead card — the
	// game-283 Athreos shape, via the shared chokepoint.
	if got := createPermanent(gs, 2, dead, false); got != nil {
		t.Fatalf("createPermanent materialized a LeftGame owner's card on seat 2")
	}
	for i, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == dead {
				t.Fatalf("dead player's card on seat %d battlefield", i)
			}
		}
	}
	// Census must stay clean (the ceased ID must not be present).
	for _, v := range gameengine.RunAllInvariants(gs) {
		if v.Name == "ZoneConservation" {
			t.Fatalf("ZoneConservation after refused createPermanent: %s", v.Message)
		}
	}
}

// TestCreatePermanent_RefusesDeadSeat0OwnersCard pins the r63 seed-1337
// game 2293 hole: the chokepoint keyed on `Owner > 0`, so a card owned
// by SEAT 0 (Owner==0, Go's int zero value) bypassed the §800.4a guard
// entirely. When seat 0 was the eliminated player, The Cruelty of Gix
// chapter III reanimated its ceased Flayer of Loyalties (owner 0) onto a
// live battlefield — present-but-ceased fabrication, flagged every tick
// from turn 69 on. Same scenario as game 283 but owner==0.
func TestCreatePermanent_RefusesDeadSeat0OwnersCard(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(2293)), nil)
	dead := mintedCreatureInGraveyard(t, gs, 0, "Flayer of Loyalties")

	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "21+ commander damage (CR 704.6c)"
	gameengine.HandleSeatElimination(gs, 0)

	if got := createPermanent(gs, 1, dead, false); got != nil {
		t.Fatalf("createPermanent materialized SEAT-0 leaver's card on seat 1 (the Owner>0 hole)")
	}
	for i, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == dead {
				t.Fatalf("dead seat-0 player's card on seat %d battlefield", i)
			}
		}
	}
	for _, v := range gameengine.RunAllInvariants(gs) {
		if v.Name == "ZoneConservation" {
			t.Fatalf("ZoneConservation after refused seat-0 createPermanent: %s", v.Message)
		}
	}
}

// TestCrueltyOfGixCh3_SkipsDeadSeat0Graveyard is the headline pin: the
// game-2293 root path. The Cruelty of Gix chapter III selector walked
// every seat's graveyard with no LeftGame check and reanimated the
// highest-CMC creature; with eliminated seat 0's Flayer of Loyalties
// (a 10/10) the highest in the game, it pulled the ceased card onto the
// controller's battlefield. The selector must skip departed graveyards.
func TestCrueltyOfGixCh3_SkipsDeadSeat0Graveyard(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(2293)), nil)

	// Eliminated seat 0's graveyard holds the biggest creature.
	flayer := mintedCreatureInGraveyard(t, gs, 0, "Flayer of Loyalties")
	flayer.Types = []string{"creature", "cmc:7"}
	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "21+ commander damage (CR 704.6c)"
	gameengine.HandleSeatElimination(gs, 0)

	// The Cruelty of Gix on seat 1.
	gixCard := &gameengine.Card{Name: "The Cruelty of Gix", Owner: 1,
		Types: []string{"enchantment", "saga"}}
	gameengine.MintOGInstanceID(gs, gixCard)
	gix := &gameengine.Permanent{Card: gixCard, Controller: 1, Owner: 1,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, gix)

	sagaTheCrueltyOfGix(gs, gix, 3)

	for i, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == flayer {
				t.Fatalf("Cruelty of Gix reanimated eliminated seat 0's Flayer onto seat %d", i)
			}
		}
	}
	for _, v := range gameengine.RunAllInvariants(gs) {
		if v.Name == "ZoneConservation" {
			t.Fatalf("ZoneConservation after Cruelty ch3 skip: %s", v.Message)
		}
	}
}

func TestCreatePermanent_LivingOwnerStillWorks(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(284)), nil)
	alive := mintedCreatureInGraveyard(t, gs, 1, "Gravecrawler")

	got := createPermanent(gs, 2, alive, false)
	if got == nil {
		t.Fatalf("guard over-fired: living owner's card refused")
	}
	if got.Controller != 2 {
		t.Fatalf("controller = %d, want 2", got.Controller)
	}
	for _, v := range gameengine.RunAllInvariants(gs) {
		if v.Name == "ZoneConservation" {
			t.Fatalf("ZoneConservation on legitimate steal: %s", v.Message)
		}
	}
}

func TestAthreosShroudDies_SkipsDeadOwner(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(285)), nil)

	// Athreos on seat 2.
	athCard := &gameengine.Card{Name: "Athreos, Shroud-Veiled", Owner: 2,
		Types: []string{"legendary", "enchantment", "creature"}}
	gameengine.MintOGInstanceID(gs, athCard)
	athreos := &gameengine.Permanent{Card: athCard, Controller: 2, Owner: 2,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[2].Battlefield = append(gs.Seats[2].Battlefield, athreos)

	// The coin-countered creature, dead-owned: in seat 1's graveyard,
	// seat 1 eliminated.
	dying := mintedCreatureInGraveyard(t, gs, 1, "Gurmag Angler")
	dyingPerm := &gameengine.Permanent{Card: dying, Controller: 1, Owner: 1,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{"coin": 1}, Flags: map[string]int{}}
	gs.Seats[1].Lost = true
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"
	gameengine.HandleSeatElimination(gs, 1)

	athreosShroudDies(gs, athreos, map[string]interface{}{
		"perm": dyingPerm,
		"card": dying,
	})

	for _, p := range gs.Seats[2].Battlefield {
		if p != nil && p.Card == dying {
			t.Fatalf("Athreos returned a dead player's creature")
		}
	}
	for _, v := range gameengine.RunAllInvariants(gs) {
		if v.Name == "ZoneConservation" {
			t.Fatalf("ZoneConservation after Athreos skip: %s", v.Message)
		}
	}
}
