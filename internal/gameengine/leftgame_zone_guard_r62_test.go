package gameengine

import (
	"math/rand"
	"testing"
)

// leftgame_zone_guard_r62_test.go — Phase-H OGVC fabrication class
// (CLAUDE.md Open Issue, Silk / Opaline Bracers / Pathrazer of Ulamog).
//
// Root cause (seed 7777 game 2430, bisected r62): an eliminated player's
// zone slices deliberately keep their *Card pointers for forensic
// clarity, and HandleSeatElimination marks those cards' InstanceIDs
// ceased per CR §800.4a. A selector that walks gs.Seats[*].Graveyard
// WITHOUT a LeftGame check (per_card reanimateResolve) could then find
// one of those dead cards and MoveCard it onto a live battlefield —
// the census flags "present but not in (Minted - Ceased)" (fabrication)
// on every tick for the rest of the game.
//
// Fix pinned here: both zone-write chokepoints (MoveCard and
// gs.moveToZone) refuse to move a card whose owner has LeftGame.

// mkOwnedCard builds a creature card with a minted OG InstanceID owned
// by the given seat, parked in that seat's graveyard.
func mkOwnedGraveyardCard(gs *GameState, owner int, name string) *Card {
	c := &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     5,
		BaseToughness: 5,
	}
	MintOGInstanceID(gs, c)
	gs.Seats[owner].Graveyard = append(gs.Seats[owner].Graveyard, c)
	return c
}

// TestLeftGameGuard_MoveCardRefusesDeadOwnersCard is the chokepoint pin:
// after the owner's elimination, MoveCard must refuse to materialize the
// card in any zone, and the census must stay clean.
func TestLeftGameGuard_MoveCardRefusesDeadOwnersCard(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(7)), nil)
	dead := mkOwnedGraveyardCard(gs, 3, "Pathrazer of Ulamog")

	// Eliminate seat 3 through the real §800.4a pipeline (ceases the ID,
	// keeps the graveyard pointer).
	gs.Seats[3].Lost = true
	gs.Seats[3].LossReason = "life total 0 or less (CR 704.5a)"
	HandleSeatElimination(gs, 3)

	// A surviving seat's effect tries to reanimate the dead card —
	// exactly the per_card reanimateResolve shape.
	res := MoveCard(gs, dead, 3, "graveyard", "battlefield", "reanimate")
	if res.FinalZone != "" || res.Permanent != nil {
		t.Fatalf("MoveCard moved a LeftGame owner's card (final zone %q) — CR §800.4a violation", res.FinalZone)
	}

	// No live battlefield may hold it.
	for i, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == dead {
				t.Fatalf("dead player's card materialized on seat %d's battlefield", i)
			}
		}
	}

	// And the census must not see a fabrication.
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation after refused move: %v", err)
	}
}

// TestLeftGameGuard_MoveToZoneRefusesDeadOwnersCard pins the second
// chokepoint (direct gs.moveToZone callers bypass MoveCard).
func TestLeftGameGuard_MoveToZoneRefusesDeadOwnersCard(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(8)), nil)
	dead := mkOwnedGraveyardCard(gs, 2, "Silk, Web Weaver")

	gs.Seats[2].Lost = true
	gs.Seats[2].LossReason = "drew from empty library (CR 704.5b)"
	HandleSeatElimination(gs, 2)

	handBefore := len(gs.Seats[1].Hand)
	gs.moveToZone(1, dead, "hand")
	if len(gs.Seats[1].Hand) != handBefore {
		t.Fatalf("moveToZone placed a LeftGame owner's card into a live hand")
	}
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation after refused moveToZone: %v", err)
	}
}

// TestLeftGameGuard_Seat0OwnerRefused pins the r63 seed-1337 game 2293
// hole: the §800.4a chokepoint guards keyed on `Owner > 0`, silently
// exempting every card owned by SEAT 0. When seat 0 is the eliminated
// player, a reanimator (The Cruelty of Gix → Flayer of Loyalties) could
// pull its ceased graveyard cards onto a live battlefield — present-but-
// ceased fabrication. Both chokepoints must refuse seat-0-owned cards.
func TestLeftGameGuard_Seat0OwnerRefused(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(2293)), nil)
	dead := mkOwnedGraveyardCard(gs, 0, "Flayer of Loyalties")

	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "21+ commander damage (CR 704.6c)"
	HandleSeatElimination(gs, 0)

	// MoveCard chokepoint.
	res := MoveCard(gs, dead, 0, "graveyard", "battlefield", "reanimate")
	if res.FinalZone != "" || res.Permanent != nil {
		t.Fatalf("MoveCard materialized seat-0 leaver's card (final zone %q) — §800.4a seat-0 hole", res.FinalZone)
	}
	for i, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == dead {
				t.Fatalf("seat-0 leaver's card on seat %d battlefield via MoveCard", i)
			}
		}
	}

	// moveToZone chokepoint (direct callers bypass MoveCard).
	handBefore := len(gs.Seats[1].Hand)
	gs.moveToZone(1, dead, "hand")
	if len(gs.Seats[1].Hand) != handBefore {
		t.Fatalf("moveToZone placed seat-0 leaver's card into a live hand — §800.4a seat-0 hole")
	}

	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation after refused seat-0 moves: %v", err)
	}
}

// TestLeftGameGuard_LivingOwnersCardStillMoves is the control: the guard
// must not block normal zone movement for living owners.
func TestLeftGameGuard_LivingOwnersCardStillMoves(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(9)), nil)
	alive := mkOwnedGraveyardCard(gs, 2, "Gravecrawler")

	res := MoveCard(gs, alive, 2, "graveyard", "battlefield", "reanimate")
	if res.FinalZone != "battlefield" || res.Permanent == nil {
		t.Fatalf("guard over-fired: living owner's reanimation refused (final zone %q)", res.FinalZone)
	}
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation on legitimate reanimation: %v", err)
	}
}
