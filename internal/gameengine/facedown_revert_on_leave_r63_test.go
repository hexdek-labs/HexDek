package gameengine

import "testing"

// r63 — face-down REVERT-ON-LEAVE (CR §707.2): when a face-down permanent
// in the real-card overlay model (disguise, cyber / external turn-down)
// leaves the battlefield, the overlay ceases and the TRUE card enters the
// destination zone FACE UP. The real card is the permanent of record, so
// it is the card that moves — there is no synthetic token to delete.
//
// The clear happens at the canonical battlefield-exit chokepoint
// (FireZoneChange → moveToZone), which all of destroy / exile / bounce
// route the non-token card through.

// cyberFaceDown turns seat 0's freshly-added creature face down via the
// external (Cyber Conversion / Ixidron) entry point and returns the perm +
// its real card.
func cyberFaceDown(t *testing.T, gs *GameState) (*Permanent, *Card) {
	t.Helper()
	p := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	p.Card.Owner = 0
	if !TurnPermanentFaceDown(gs, p, "cyber") {
		t.Fatal("TurnPermanentFaceDown returned false")
	}
	return p, p.Card
}

func TestFaceDownRevert_Cyber_OnDeath(t *testing.T) {
	gs := newFixtureGame(t)
	p, real := cyberFaceDown(t, gs)
	if !DestroyPermanent(gs, p, nil) {
		t.Fatal("DestroyPermanent returned false")
	}
	gy := gs.Seats[0].Graveyard
	if len(gy) != 1 || gy[0] != real {
		t.Fatalf("real card should be in graveyard, got %d cards", len(gy))
	}
	if real.FaceDown {
		t.Error("card must enter graveyard FACE UP (CR §707.2)")
	}
}

func TestFaceDownRevert_Cyber_OnExile(t *testing.T) {
	gs := newFixtureGame(t)
	p, real := cyberFaceDown(t, gs)
	if !ExilePermanent(gs, p, nil) {
		t.Fatal("ExilePermanent returned false")
	}
	if real.FaceDown {
		t.Error("card must enter exile FACE UP")
	}
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c == real {
			found = true
		}
	}
	if !found {
		t.Error("real card should be in exile")
	}
}

func TestFaceDownRevert_Cyber_OnBounce(t *testing.T) {
	gs := newFixtureGame(t)
	p, real := cyberFaceDown(t, gs)
	if !BouncePermanent(gs, p, nil, "hand") {
		t.Fatal("BouncePermanent returned false")
	}
	if real.FaceDown {
		t.Error("card must return to hand FACE UP")
	}
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c == real {
			found = true
		}
	}
	if !found {
		t.Error("real card should be in hand")
	}
}

// Disguise is already the real-card model — confirm it reverts on death.
func TestFaceDownRevert_Disguise_OnDeath(t *testing.T) {
	gs := newDisguiseGame(t)
	gs.Seats[0].ManaPool = DisguiseFaceDownCost
	card := disguiseHandCard(gs, 0, "Hooded Hydra", 4)
	perm, err := CastDisguiseFaceDown(gs, 0, card)
	if err != nil {
		t.Fatalf("CastDisguiseFaceDown: %v", err)
	}
	if !card.FaceDown {
		t.Fatal("precondition: disguise creature should be face down")
	}
	if !DestroyPermanent(gs, perm, nil) {
		t.Fatal("DestroyPermanent returned false")
	}
	if card.FaceDown {
		t.Error("disguise card must enter graveyard FACE UP")
	}
	gy := gs.Seats[0].Graveyard
	if len(gy) != 1 || gy[0] != card {
		t.Fatalf("real disguise card should be in graveyard, got %d cards", len(gy))
	}
}
