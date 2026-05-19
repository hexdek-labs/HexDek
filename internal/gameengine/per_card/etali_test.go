package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestEtali_ETB_FreeCastsExiledNonlands verifies the each-player exile
// chain plus free-cast onto Etali's controller for permanent nonlands.
func TestEtali_ETB_FreeCastsExiledNonlands(t *testing.T) {
	gs := newGame(t, 4)

	// Each opponent: top-of-library is two lands then a creature.
	for seat := 1; seat <= 3; seat++ {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library,
			&gameengine.Card{Name: "Mountain", Owner: seat, Types: []string{"land"}},
			&gameengine.Card{Name: "Forest", Owner: seat, Types: []string{"land"}},
			&gameengine.Card{Name: "TestCreature", Owner: seat, Types: []string{"creature"}},
		)
	}
	// Etali's own seat: top is a sorcery (instant/sorcery free-cast is
	// flagged as partial — exercises the partial-event path).
	gs.Seats[0].Library = append(gs.Seats[0].Library,
		&gameengine.Card{Name: "TestBolt", Owner: 0, Types: []string{"sorcery"}},
	)

	etali := addPerm(gs, 0, "Etali, Primal Conqueror", "creature", "legendary")
	gameengine.InvokeETBHook(gs, etali)

	// Three opponent permanent nonlands should now be on seat 0's
	// battlefield (in addition to Etali itself).
	freeCasts := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p == etali {
			continue
		}
		if p.Card != nil && p.Card.Name == "TestCreature" {
			freeCasts++
		}
	}
	if freeCasts != 3 {
		t.Errorf("expected 3 free-cast TestCreature permanents on Etali's battlefield, got %d", freeCasts)
	}

	// Lands should have been moved to exile, not battlefield.
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler breadcrumb")
	}
	// Sorcery should have triggered the partial event.
	if hasEvent(gs, "per_card_partial") < 1 {
		t.Errorf("expected per_card_partial for instant/sorcery free-cast")
	}
}

// TestEtali_ETB_DeadOpponentsSkipped ensures dead seats don't have
// their library touched.
func TestEtali_ETB_DeadOpponentsSkipped(t *testing.T) {
	gs := newGame(t, 4)
	gs.Seats[2].Lost = true
	for seat := 1; seat <= 3; seat++ {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library,
			&gameengine.Card{Name: "Filler", Owner: seat, Types: []string{"creature"}},
		)
	}
	libBefore2 := len(gs.Seats[2].Library)

	etali := addPerm(gs, 0, "Etali, Primal Conqueror", "creature", "legendary")
	gameengine.InvokeETBHook(gs, etali)

	if len(gs.Seats[2].Library) != libBefore2 {
		t.Errorf("expected dead seat 2 library untouched, before=%d after=%d", libBefore2, len(gs.Seats[2].Library))
	}
}

// TestEtali_ETB_NoDuplicatePermanent locks the r42b fix: each free-cast
// exiled card produces exactly one Permanent (on Etali's controller's
// side), not two. Previously a redundant MoveCard(ownerSeat, ...)
// alongside enterBattlefieldWithETB(controller, ...) wrapped the card
// in a Permanent twice — once on the owner's battlefield and once on
// the controller's — leaving the same *Card pointer in two zones and
// tripping the CardIdentity invariant.
func TestEtali_ETB_NoDuplicatePermanent(t *testing.T) {
	gs := newGame(t, 4)

	// Seat 1 library: a single creature on top.
	creatureCard := &gameengine.Card{
		Name:  "Free-Cast Creature",
		Owner: 1,
		Types: []string{"creature"},
	}
	gs.Seats[1].Library = append(gs.Seats[1].Library, creatureCard)
	// Other seats: empty libraries so only seat 1 contributes.
	etali := addPerm(gs, 0, "Etali, Primal Conqueror", "creature", "legendary")
	gameengine.InvokeETBHook(gs, etali)

	// The exiled creature must appear in exactly one zone — Etali's
	// controller (seat 0) battlefield — never in two zones at once.
	occurrences := 0
	for seatIdx, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == creatureCard {
				occurrences++
				if seatIdx != 0 {
					t.Errorf("free-cast Permanent landed on seat %d battlefield, want seat 0 (Etali's controller)", seatIdx)
				}
			}
		}
	}
	if occurrences != 1 {
		t.Errorf("free-cast card appears in %d Permanents across all battlefields, want 1", occurrences)
	}
}
