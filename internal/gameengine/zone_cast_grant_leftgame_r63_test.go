package gameengine

import (
	"math/rand"
	"testing"
)

// zone_cast_grant_leftgame_r63_test.go — seed-777 game-703 Serenity
// fabrication.
//
// Root cause: Knowledge Pool's cast trigger scans EVERY seat's exile for
// Pool-tagged cards and registers a free-cast ZoneCastGrant — without a
// LeftGame check. An eliminated player's exiled cards stay in the slice
// (pointers kept for forensic clarity, InstanceIDs ceased per §800.4a),
// so the Pool granted a free cast of a dead player's ceased Serenity.
// gs.ZoneCastGrants is walked as zone-presence by the census, so the
// ceased ID re-entered the present set: a ZoneConservation fabrication
// on every census tick until game end.
//
// Fix pinned here: RegisterZoneCastGrant refuses cards whose owner has
// LeftGame (chokepoint, mirrors the MoveCard / moveToZone §800.4a
// guards from PR #1041); the Knowledge Pool selector additionally skips
// LeftGame seats.

// TestZoneCastGrant_RefusesDeadOwnersCard is the chokepoint pin: after
// the owner's elimination, a grant registration on their exiled card
// must be refused and the census must stay clean. FAILS pre-fix.
func TestZoneCastGrant_RefusesDeadOwnersCard(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(703)), nil)

	dead := &Card{
		Name:  "Serenity",
		Owner: 3,
		Types: []string{"enchantment"},
	}
	MintOGInstanceID(gs, dead)
	gs.Seats[3].Exile = append(gs.Seats[3].Exile, dead)

	// Eliminate seat 3 through the real §800.4a pipeline (ceases the ID,
	// keeps the exile pointer).
	gs.Seats[3].Lost = true
	gs.Seats[3].LossReason = "life total 0 or less (CR 704.5a)"
	HandleSeatElimination(gs, 3)

	// A surviving seat's effect tries to grant a free cast on the dead
	// card — the Knowledge Pool shape.
	RegisterZoneCastGrant(gs, dead, NewFreeCastFromExilePermission(1, "Knowledge Pool"))

	if GetZoneCastGrant(gs, dead) != nil {
		t.Fatalf("grant registered on a LeftGame owner's card — CR §800.4a violation")
	}
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation after refused grant: %v", err)
	}
}

// TestZoneCastGrant_LivingOwnersCardStillGranted is the control: the
// guard must not block grants for living owners.
func TestZoneCastGrant_LivingOwnersCardStillGranted(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(704)), nil)

	alive := &Card{
		Name:  "Lightning Bolt",
		Owner: 2,
		Types: []string{"instant"},
	}
	MintOGInstanceID(gs, alive)
	gs.Seats[2].Exile = append(gs.Seats[2].Exile, alive)

	RegisterZoneCastGrant(gs, alive, NewFreeCastFromExilePermission(1, "Knowledge Pool"))

	if GetZoneCastGrant(gs, alive) == nil {
		t.Fatalf("guard over-fired: living owner's card grant refused")
	}
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census violation on legitimate grant: %v", err)
	}
}
