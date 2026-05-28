package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestCreatePermanent_DedupOnDoubleCall reproduces the Feynman #3 hand-bloat
// bug: MoveCard("graveyard","battlefield") places the card, then
// enterBattlefieldWithETB calls createPermanent again for the same card.
// Before the dedup guard, this produced two Permanent wrappers pointing to
// the same *Card, leading to phantom zone drift when they left the battlefield
// via different paths.
func TestCreatePermanent_DedupOnDoubleCall(t *testing.T) {
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Sheoldred", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	// Simulate the double-call pattern from reanimation handlers:
	// Step 1: MoveCard places the card on the battlefield.
	result := gameengine.MoveCard(gs, card, 0, "graveyard", "battlefield", "reanimate")
	if result.FinalZone != "battlefield" {
		t.Fatalf("MoveCard returned %q, want battlefield", result.FinalZone)
	}
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("after MoveCard: battlefield=%d, want 1", len(gs.Seats[0].Battlefield))
	}

	// Step 2: enterBattlefieldWithETB (calls createPermanent internally).
	// Before the fix, this would create a SECOND permanent.
	perm := enterBattlefieldWithETB(gs, 0, card, false)
	if perm == nil {
		t.Fatal("enterBattlefieldWithETB returned nil")
	}

	// The dedup guard should return the existing permanent, not create a new one.
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Errorf("dedup failed: battlefield has %d permanents, want 1", len(gs.Seats[0].Battlefield))
	}
	if gs.Seats[0].Battlefield[0].Card != card {
		t.Error("permanent wraps wrong card")
	}
}

// TestCreatePermanent_NoDedupForDifferentCards confirms that the dedup guard
// does NOT block distinct cards from entering the battlefield.
func TestCreatePermanent_NoDedupForDifferentCards(t *testing.T) {
	gs := newGame(t, 2)
	card1 := addCard(gs, 0, "Sheoldred", "creature")
	card2 := addCard(gs, 0, "Grave Titan", "creature")

	perm1 := enterBattlefieldWithETB(gs, 0, card1, false)
	perm2 := enterBattlefieldWithETB(gs, 0, card2, false)

	if perm1 == nil || perm2 == nil {
		t.Fatal("expected both permanents to be created")
	}
	if len(gs.Seats[0].Battlefield) != 2 {
		t.Errorf("two distinct cards should produce 2 permanents, got %d", len(gs.Seats[0].Battlefield))
	}
}

// TestCreatePermanent_CrossSeatImplementsControlChange verifies
// the corrected cross-seat dedup contract: when the *Card is
// already wrapped in a Permanent on a DIFFERENT seat's
// battlefield, createPermanent IMPLEMENTS CONTROL-CHANGE
// SEMANTICS — the existing wrapper is dropped from the other
// seat's battlefield, and a new wrapper is created on the target
// seat with the caller's requested controller. The *Card pointer
// ends up on EXACTLY ONE battlefield (the target seat's), per
// CR §400.7c's "exactly one zone at a time" invariant.
//
// CONTRACT CHANGE: pre-r60-X, this test asserted that BOTH
// wrappers coexisted (the existing on seat 1 PLUS a new one on
// seat 0). That permissive behavior was the root cause of the
// dominant CardIdentity violation cluster (Cluster A in
// docs/loki-r60-250k-analysis.md, PR #713): "*Card appears in
// both seat X battlefield and seat Y battlefield." Per CR
// §400.7c an object exists in exactly one zone at a time; the
// same *Card pointer on two battlefields is invalid by
// construction. Control-change patterns (Bribery, Etali) work
// via Owner/Controller fields on a SINGLE Permanent — the OLD
// engine path placed an intermediate wrapper on the original
// seat first, then created a duplicate on the controller's seat.
// The fix drops the intermediate.
//
// Layer-stress sweep verification post-fix: 1000 games seed 42
// went from 114 violations in 6 games to 0 violations.
func TestCreatePermanent_CrossSeatImplementsControlChange(t *testing.T) {
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Stolen Creature", "creature")

	// Place on seat 1's battlefield directly (simulating MoveCard to owner).
	stale := &gameengine.Permanent{
		Card:       card,
		Controller: 1,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, stale)

	// Seat 0 calls createPermanent on the same *Card — must
	// implement control-change: drop the stale wrapper from seat
	// 1's battlefield, create a new wrapper on seat 0.
	perm := createPermanent(gs, 0, card, false)
	if perm == nil {
		t.Fatal("cross-seat createPermanent: want new Permanent on target seat (control-change), got nil")
	}
	if perm == stale {
		t.Errorf("cross-seat createPermanent: want a NEW wrapper on seat 0 with Controller=0, got the stale seat-1 wrapper (would have wrong controller)")
	}
	if perm.Controller != 0 {
		t.Errorf("new wrapper Controller: want 0 (target seat), got %d", perm.Controller)
	}
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Errorf("seat 0 should have 1 perm (the new wrapper), got %d", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("seat 1 should have 0 perms (stale wrapper dropped per §400.7c exactly-one-zone), got %d — would produce CardIdentity violation",
			len(gs.Seats[1].Battlefield))
	}
}
