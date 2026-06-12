package gameengine

// r63 owner-immutability regressions (owner design from 7174n1c).
// CR §108.3: ownership is write-once; only control changes; return to
// owner is ONE shared operation (control_revert.go) used by until-EOT
// expiry and §800.4a controller-leaves-game.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func ownerHasViolation(gs *GameState, substr string) bool {
	for _, v := range RunAllInvariants(gs) {
		if v.Name == "OwnerImmutability" {
			return true
		}
		_ = substr
	}
	return false
}

// The invariant trips when any code path mutates Card.Owner post-mint.
func TestOwnerImmutability_TripsOnCardOwnerMutation(t *testing.T) {
	gs := newFixtureGame(t)
	c := &Card{Name: "Honest Bear", Owner: 0, Types: []string{"creature"}}
	MintOGInstanceID(gs, c)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)

	if ownerHasViolation(gs, "") {
		t.Fatal("clean state must not flag")
	}
	c.Owner = 1 // the write-once violation
	if !ownerHasViolation(gs, "") {
		t.Fatal("Card.Owner mutation after mint must trip OwnerImmutability")
	}
}

// The invariant trips on Permanent.Owner diverging from Card.Owner —
// the PR-#1047 theft-handler corruption shape.
func TestOwnerImmutability_TripsOnPermOwnerDivergence(t *testing.T) {
	gs := newFixtureGame(t)
	c := &Card{Name: "Brute Suit", Owner: 0, Types: []string{"artifact"}}
	MintOGInstanceID(gs, c)
	p := &Permanent{
		Card: c, Controller: 1, Owner: 1, // corrupt: perm claims thief
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, p)

	if !ownerHasViolation(gs, "") {
		t.Fatal("Permanent.Owner diverging from Card.Owner must trip OwnerImmutability")
	}
	p.Owner = 0 // aligned again
	if ownerHasViolation(gs, "") {
		t.Fatal("aligned ownership must not flag")
	}
}

// Until-end-of-turn steal reverts at cleanup through the shared op.
func TestTempControlSteal_RevertsAtCleanup(t *testing.T) {
	gs := newFixtureGame(t)
	victim := addBattlefield(gs, 1, "Borrowed Bear", 2, 2, "creature")
	victim.Owner = 1
	victim.Card.Owner = 1
	thiefSrc := addBattlefield(gs, 0, "Act Source", 1, 1, "creature")

	ResolveEffect(gs, thiefSrc, &gameast.GainControl{
		Target:   gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true},
		Duration: "until_end_of_turn",
	})

	if victim.Controller != 0 {
		t.Fatalf("steal should move control to seat 0, got %d", victim.Controller)
	}
	if len(gs.TempControlGrants) != 1 {
		t.Fatalf("until-EOT steal must register a TempControlGrant, got %d", len(gs.TempControlGrants))
	}

	ScanExpiredDurations(gs, "ending", "cleanup")

	if victim.Controller != 1 {
		t.Fatalf("cleanup must revert control to the owner: got controller %d", victim.Controller)
	}
	found := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == victim {
			found = true
		}
	}
	if !found {
		t.Fatal("reverted permanent must sit on the owner's battlefield slice")
	}
	if len(gs.TempControlGrants) != 0 {
		t.Fatal("grants must be consumed at cleanup")
	}
}

// Permanent steal (no duration) does NOT register a grant and survives
// cleanup under the thief's control.
func TestPermanentSteal_NoRevert(t *testing.T) {
	gs := newFixtureGame(t)
	victim := addBattlefield(gs, 1, "Taken Bear", 2, 2, "creature")
	victim.Owner = 1
	victim.Card.Owner = 1
	src := addBattlefield(gs, 0, "Control Magic Source", 1, 1, "creature")

	ResolveEffect(gs, src, &gameast.GainControl{
		Target:   gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true},
		Duration: "permanent",
	})
	ScanExpiredDurations(gs, "ending", "cleanup")

	if victim.Controller != 0 {
		t.Fatal("permanent steal must survive cleanup")
	}
}

// §800.4a: when the thief is eliminated mid-steal, the SAME shared op
// reverts the permanent to its owner's battlefield (supersedes the
// #1046 exile-always MVP).
func TestStolenPermanent_RevertsOnControllerElimination(t *testing.T) {
	gs := newFixtureGame(t)
	victim := addBattlefield(gs, 1, "Borrowed Bear", 2, 2, "creature")
	victim.Owner = 1
	victim.Card.Owner = 1
	MintOGInstanceID(gs, victim.Card)
	src := addBattlefield(gs, 0, "Act Source", 1, 1, "creature")
	src.Owner = 0

	ResolveEffect(gs, src, &gameast.GainControl{
		Target:   gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true},
		Duration: "until_end_of_turn",
	})
	if victim.Controller != 0 {
		t.Fatal("setup: steal failed")
	}

	HandleSeatElimination(gs, 0) // thief leaves mid-steal

	if victim.Controller != 1 {
		t.Fatalf("§800.4a must revert control to the owner, got controller %d", victim.Controller)
	}
	onOwnerBF := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == victim {
			onOwnerBF = true
		}
	}
	if !onOwnerBF {
		t.Fatal("reverted permanent must be on the owner's battlefield")
	}
	if _, ceased := gs.CeasedInstanceIDs[victim.Card.InstanceID]; ceased {
		t.Fatal("other-owned card must never cease on the thief's elimination")
	}
}
