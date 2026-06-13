package gameengine

// Forensic-stamp regression for the fishtank-radar r63 CardIdentity
// basic-land self-duplication cluster (Plains game 40 / Swamp game 291:
// one OG basic land double-listed in a single seat's battlefield). The
// producer could not be pinned without a real-pool repro, so checkCardIdentity
// now appends [prov=… same_card=… ts=…/…] to the violation so the NEXT
// grinder run localizes the producing effect:
//   - same_card=true  → one *Card wrapped by two permanents (unguarded
//                       direct-append of the original card — the deduced shape).
//   - same_card=false → two *Card structs sharing an InstanceID (mint dup).
//   - the higher ts is the duplicate added second → event-log anchor.

import (
	"math/rand"
	"strings"
	"testing"
)

func TestCardIdentity_DupForensics_SameCardOnBattlefield(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)

	// A single OG basic-land *Card wrapped by TWO permanents on seat 0's
	// battlefield — the exact shape of the live land-self-dup cluster.
	land := &Card{Name: "Plains", Types: []string{"land"}, Owner: 0}
	MintOGInstanceID(gs, land)
	if land.InstanceID == "" {
		t.Fatal("setup: MintOGInstanceID did not stamp an InstanceID")
	}

	gs.Seats[0].Battlefield = []*Permanent{
		{Card: land, Controller: 0, Timestamp: 5},
		{Card: land, Controller: 0, Timestamp: 9}, // the duplicate, added later
	}

	err := checkCardIdentity(gs)
	if err == nil {
		t.Fatal("checkCardIdentity should fire on a land double-listed in one battlefield")
	}
	msg := err.Error()
	for _, want := range []string{
		"same_card=true", // one *Card, two permanents
		"prov=OG",        // original deck card, not a re-minted token
		"ts=5/9",         // later timestamp = the second-added duplicate
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("CardIdentity forensics missing %q; got: %s", want, msg)
		}
	}
}

func TestCardIdentity_DupForensics_DistinctCardsSameID(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(2)), nil)

	// Two DISTINCT *Card structs sharing an InstanceID (mint-duplication
	// shape) — same_card must read false.
	a := &Card{Name: "Swamp", Types: []string{"land"}, Owner: 0, InstanceID: "h0OGVC000068"}
	b := &Card{Name: "Swamp", Types: []string{"land"}, Owner: 0, InstanceID: "h0OGVC000068"}
	gs.Seats[0].Battlefield = []*Permanent{
		{Card: a, Controller: 0, Timestamp: 3},
		{Card: b, Controller: 0, Timestamp: 7},
	}

	err := checkCardIdentity(gs)
	if err == nil {
		t.Fatal("checkCardIdentity should fire on two cards sharing an InstanceID")
	}
	if msg := err.Error(); !strings.Contains(msg, "same_card=false") {
		t.Fatalf("expected same_card=false for distinct *Card structs; got: %s", msg)
	}
}
