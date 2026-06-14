package gameengine

// CR §712.4 / §711.4: a transforming double-faced card has only its
// FRONT-face characteristics in every zone other than the battlefield.
// A back-face-up permanent that leaves the battlefield (dies, is exiled,
// bounced, milled) must revert to its front face in the destination zone.
//
// Regression for the r63 DFC mechanic audit: before the fix, a flipped
// permanent kept its back-face Name + AST in the graveyard, breaking
// graveyard-recursion name matching, recast-from-graveyard, the
// CardIdentity invariant, and MV-in-graveyard reads.

import (
	"math/rand"
	"testing"
)

func TestDFC_RevertsToFrontFace_OnDeath(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(0)), nil)
	p := syntheticEtali(gs, 0)

	// Flip to the back face on the battlefield.
	if !TransformPermanent(gs, p, "activated") {
		t.Fatalf("transform returned false")
	}
	if !p.Transformed || p.Card.Name != "Etali, Primal Sickness" {
		t.Fatalf("precondition: not on back face (name=%q transformed=%v)",
			p.Card.Name, p.Transformed)
	}
	deadCard := p.Card

	if !DestroyPermanent(gs, p, nil) {
		t.Fatalf("DestroyPermanent returned false")
	}

	// The card must now be in seat 0's graveyard wearing its FRONT face.
	gy := gs.Seats[0].Graveyard
	if len(gy) != 1 || gy[0] != deadCard {
		t.Fatalf("expected dead card in graveyard, got %d cards", len(gy))
	}
	if deadCard.Name != "Etali, Primal Conqueror" {
		t.Fatalf("graveyard card Name = %q, want front-face name "+
			"(CR §712.4)", deadCard.Name)
	}
	if deadCard.AST != p.FrontFaceAST {
		t.Fatalf("graveyard card AST not reverted to front face")
	}
	if p.Transformed {
		t.Fatalf("permanent still marked Transformed after leaving play")
	}
}

func TestDFC_RevertsToFrontFace_OnExile(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(0)), nil)
	p := syntheticEtali(gs, 0)
	if !TransformPermanent(gs, p, "activated") {
		t.Fatalf("transform returned false")
	}
	exiledCard := p.Card

	if !ExilePermanent(gs, p, nil) {
		t.Fatalf("ExilePermanent returned false")
	}

	ex := gs.Seats[0].Exile
	if len(ex) != 1 || ex[0] != exiledCard {
		t.Fatalf("expected exiled card in exile, got %d cards", len(ex))
	}
	if exiledCard.Name != "Etali, Primal Conqueror" {
		t.Fatalf("exiled card Name = %q, want front-face name", exiledCard.Name)
	}
	if exiledCard.AST != p.FrontFaceAST {
		t.Fatalf("exiled card AST not reverted to front face")
	}
}

// A permanent that was never transformed (front face up) is untouched by
// the revert path — guards against the helper clobbering a normal death.
func TestDFC_FrontFaceDeath_Unaffected(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(0)), nil)
	p := syntheticEtali(gs, 0)
	deadCard := p.Card

	if !DestroyPermanent(gs, p, nil) {
		t.Fatalf("DestroyPermanent returned false")
	}
	if deadCard.Name != "Etali, Primal Conqueror" {
		t.Fatalf("front-face death Name = %q, want unchanged", deadCard.Name)
	}
	if p.Transformed {
		t.Fatalf("Transformed flipped true on a front-face death")
	}
}
