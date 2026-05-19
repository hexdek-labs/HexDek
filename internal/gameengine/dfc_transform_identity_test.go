package gameengine

// Identity tests for CR §712 transform: the same Permanent pointer must
// remain on the battlefield across a face swap. Regression guard against
// any future path that "transforms" by appending a new pointer instead
// of mutating in place.
//
// Modeled after Etali, Primal Conqueror // Etali, Primal Sickness — the
// shape that motivated the audit — but the invariant applies to every
// DFC.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// syntheticEtali builds a DFC permanent shaped like Etali: front face is
// a legendary creature, back face carries trample. Parser-free.
func syntheticEtali(gs *GameState, seatIdx int) *Permanent {
	front := &gameast.CardAST{
		Name:        "Etali, Primal Conqueror",
		FullyParsed: true,
	}
	back := &gameast.CardAST{
		Name: "Etali, Primal Sickness",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "trample", Raw: "Trample"},
		},
		FullyParsed: true,
	}
	card := &Card{
		AST:           front,
		Name:          "Etali, Primal Conqueror",
		Owner:         seatIdx,
		BasePower:     6,
		BaseToughness: 6,
		Types:         []string{"creature", "legendary"},
		CMC:           7,
		TypeLine:      "Legendary Creature — Dinosaur",
	}
	perm := &Permanent{
		Card:       card,
		Controller: seatIdx,
		Owner:      seatIdx,
	}
	perm.Timestamp = gs.NextTimestamp()
	InitDFCFaces(perm, front, back,
		"Etali, Primal Conqueror", "Etali, Primal Sickness")
	gs.Seats[seatIdx].Battlefield =
		append(gs.Seats[seatIdx].Battlefield, perm)
	return perm
}

func TestTransform_PreservesBattlefieldIdentity_Etali(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(0)), nil)
	p := syntheticEtali(gs, 0)
	cardPtrBefore := p.Card
	bfBefore := len(gs.Seats[0].Battlefield)

	if ok := TransformPermanent(gs, p, "etali_activated_transform"); !ok {
		t.Fatalf("TransformPermanent returned false for DFC permanent")
	}

	if got := len(gs.Seats[0].Battlefield); got != bfBefore {
		t.Fatalf("battlefield size changed across transform: %d -> %d",
			bfBefore, got)
	}

	count := 0
	for _, q := range gs.Seats[0].Battlefield {
		if q == p {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("permanent pointer present %d times on battlefield "+
			"after transform; want exactly 1", count)
	}

	if p.Card != cardPtrBefore {
		t.Fatalf("p.Card pointer replaced across transform; want reuse")
	}
	if !p.Transformed {
		t.Fatalf("p.Transformed not toggled")
	}
	if p.Card.Name != "Etali, Primal Sickness" {
		t.Fatalf("Card.Name = %q, want back-face name", p.Card.Name)
	}
	if p.Card.AST != p.BackFaceAST {
		t.Fatalf("Card.AST not swapped to BackFaceAST")
	}
}

// Round-trip: transform back to front must also reuse the same
// permanent and card pointers.
func TestTransform_RoundTripPreservesIdentity(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(0)), nil)
	p := syntheticEtali(gs, 0)
	cardPtrBefore := p.Card

	if !TransformPermanent(gs, p, "to_back") {
		t.Fatalf("first transform returned false")
	}
	if !TransformPermanent(gs, p, "to_front") {
		t.Fatalf("second transform returned false")
	}

	if got := len(gs.Seats[0].Battlefield); got != 1 {
		t.Fatalf("battlefield size after round-trip: %d, want 1", got)
	}
	if gs.Seats[0].Battlefield[0] != p {
		t.Fatalf("battlefield[0] pointer differs from original permanent")
	}
	if p.Card != cardPtrBefore {
		t.Fatalf("p.Card pointer replaced across round-trip")
	}
	if p.Transformed {
		t.Fatalf("p.Transformed should be false after round-trip")
	}
	if p.Card.Name != "Etali, Primal Conqueror" {
		t.Fatalf("Card.Name = %q, want front-face name", p.Card.Name)
	}
	if p.Card.AST != p.FrontFaceAST {
		t.Fatalf("Card.AST not restored to FrontFaceAST")
	}
}

// Counters, attachments, and the permanent's place in the battlefield
// slice must survive a transform (§712.3).
func TestTransform_PreservesCountersAndSlicePosition(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(0)), nil)
	// Sandwich the DFC between two unrelated permanents so any
	// accidental "remove + re-append" would scramble the order.
	other1 := &Permanent{
		Card:       &Card{Name: "Forest", Types: []string{"land"}},
		Controller: 0, Owner: 0,
	}
	other1.Timestamp = gs.NextTimestamp()
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, other1)

	p := syntheticEtali(gs, 0)
	p.Counters = map[string]int{"+1/+1": 2}

	other2 := &Permanent{
		Card:       &Card{Name: "Mountain", Types: []string{"land"}},
		Controller: 0, Owner: 0,
	}
	other2.Timestamp = gs.NextTimestamp()
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, other2)

	if !TransformPermanent(gs, p, "etali_activated_transform") {
		t.Fatalf("TransformPermanent returned false")
	}

	bf := gs.Seats[0].Battlefield
	if len(bf) != 3 {
		t.Fatalf("battlefield length = %d, want 3", len(bf))
	}
	if bf[0] != other1 || bf[1] != p || bf[2] != other2 {
		t.Fatalf("battlefield ordering scrambled by transform: %v", bf)
	}
	if p.Counters["+1/+1"] != 2 {
		t.Fatalf("counters lost across transform: %v", p.Counters)
	}
}
