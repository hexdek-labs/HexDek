package gameengine

import (
	"math/rand"
	"testing"
)

// cardidentity_alias_mismatch_r63_test.go — r63 CardIdentity cross-seat
// aliasing class (loki seed 99 game 222: "The Scarab God appears in both
// seat 0 battlefield and seat 1 battlefield", same_card=true).
//
// Root cause: a handler flipped Permanent.Controller without relocating the
// *Permanent to the new controller's battlefield slice (The Reaper King's
// reanimate-and-steal drops the card on its OWNER's slice, then sets
// Controller=thief). gs.removePermanent only scanned the controller's slice,
// so on the next control op it no-op'd and the caller appended the same
// *Permanent to a second slice — the object was then on two battlefields at
// once. Fix: removePermanent removes the permanent from wherever it
// physically sits, regardless of a transient slice/Controller mismatch.

// A permanent sitting in seat 0's slice but carrying Controller=1 must still
// be removable; removePermanent must not no-op on the mismatch.
func TestRemovePermanent_FindsAcrossSlicesOnControllerMismatch(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	card := &Card{Name: "The Scarab God", Owner: 0, Types: []string{"legendary", "creature"}}
	MintOGInstanceID(gs, card)
	p := &Permanent{Card: card, Controller: 1, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	// Physically in seat 0's slice, but Controller says seat 1 (the mismatch).
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	if !gs.removePermanent(p) {
		t.Fatal("removePermanent must find the permanent in seat 0's slice despite Controller=1")
	}
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Errorf("permanent not removed from seat 0's slice: %d left", len(gs.Seats[0].Battlefield))
	}
}

// End-to-end: a slice/Controller mismatch followed by a control change that
// routes through removePermanent must NOT leave the *Permanent on two
// battlefields. This is the exact CardIdentity dup shape.
func TestControlChange_AfterMismatch_NoCardIdentityDup(t *testing.T) {
	gs := NewGameState(3, rand.New(rand.NewSource(2)), nil)
	card := &Card{Name: "The Scarab God", Owner: 0, Types: []string{"legendary", "creature"}}
	MintOGInstanceID(gs, card)
	ts := gs.NextTimestamp()
	// The Reaper-King shape: card physically on owner seat 0's slice, but
	// controlled by seat 2 (the thief).
	p := &Permanent{Card: card, Controller: 2, Owner: 0, Timestamp: ts, Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	// A new control op (seat 1 steals it): the canonical remove-then-append.
	if !gs.removePermanent(p) {
		t.Fatal("removePermanent must locate the mismatched permanent")
	}
	p.Controller = 1
	p.Timestamp = gs.NextTimestamp()
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, p)

	for _, v := range RunAllInvariants(gs) {
		if v.Name == "CardIdentity" {
			t.Fatalf("CardIdentity dup after control change: %s", v.Message)
		}
	}
	// Sanity: exactly one battlefield holds it.
	count := 0
	for _, s := range gs.Seats {
		for _, q := range s.Battlefield {
			if q == p {
				count++
			}
		}
	}
	if count != 1 {
		t.Errorf("permanent appears on %d battlefields, want 1", count)
	}
}
