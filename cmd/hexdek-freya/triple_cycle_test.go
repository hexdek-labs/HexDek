package main

import "testing"

// TestCheckTripleCombo_AllInputOrderings constructs a 3-card cycle and
// verifies that checkTripleCombo detects it regardless of which permutation
// of the three cards is passed in. Pre-R59, only two of the six orderings
// were exercised, which left detection brittle to the order of (i,j,k) in
// the outer triple loop and to asymmetric Produces/Consumes population.
func TestCheckTripleCombo_AllInputOrderings(t *testing.T) {
	// A -> token; B (consumes token) -> mana; C (consumes mana) -> card,
	// and C produces something A consumes (let A also consume card draw).
	// Closes the cycle: A->B (token), B->C (mana), C->A (card).
	a := CardProfile{
		Name:              "TokenMaker",
		Produces:          []ResourceType{ResToken},
		Consumes:          []ResourceType{ResCard},
		MandatoryTriggers: true,
	}
	b := CardProfile{
		Name:              "TokenToMana",
		Produces:          []ResourceType{ResMana},
		Consumes:          []ResourceType{ResToken},
		MandatoryTriggers: true,
	}
	c := CardProfile{
		Name:              "ManaToCard",
		Produces:          []ResourceType{ResCard},
		Consumes:          []ResourceType{ResMana},
		MandatoryTriggers: true,
	}

	perms := [][3]CardProfile{
		{a, b, c},
		{a, c, b},
		{b, a, c},
		{b, c, a},
		{c, a, b},
		{c, b, a},
	}
	for i, p := range perms {
		combo := checkTripleCombo(p[0], p[1], p[2])
		if combo == nil {
			t.Errorf("permutation %d (%s,%s,%s): expected combo, got nil",
				i, p[0].Name, p[1].Name, p[2].Name)
			continue
		}
		if len(combo.Cards) != 3 {
			t.Errorf("permutation %d: expected 3 cards in combo, got %d", i, len(combo.Cards))
		}
	}
}

// TestCheckTripleCombo_NoCycleReturnsNil pins the negative case: three
// cards with non-overlapping resource flows should not be flagged.
func TestCheckTripleCombo_NoCycleReturnsNil(t *testing.T) {
	a := CardProfile{Name: "A", Produces: []ResourceType{ResMana}, Consumes: []ResourceType{ResLife}}
	b := CardProfile{Name: "B", Produces: []ResourceType{ResLife}, Consumes: []ResourceType{ResCounter}}
	c := CardProfile{Name: "C", Produces: []ResourceType{ResCounter}}
	if combo := checkTripleCombo(a, b, c); combo != nil {
		t.Errorf("expected nil (open chain, not cycle), got %+v", combo)
	}
}

// TestCheckTripleCombo_ReverseCycleDetected exercises the other directed
// cycle A->C->B->A explicitly: this is the second direction that the old
// 2-of-6 implementation handled, kept here so any future regression that
// only tests one direction is caught.
func TestCheckTripleCombo_ReverseCycleDetected(t *testing.T) {
	a := CardProfile{
		Name:              "A",
		Produces:          []ResourceType{ResToken},
		Consumes:          []ResourceType{ResMana},
		MandatoryTriggers: true,
	}
	b := CardProfile{
		Name:              "B",
		Produces:          []ResourceType{ResMana},
		Consumes:          []ResourceType{ResCard},
		MandatoryTriggers: true,
	}
	c := CardProfile{
		Name:              "C",
		Produces:          []ResourceType{ResCard},
		Consumes:          []ResourceType{ResToken},
		MandatoryTriggers: true,
	}
	// Cycle direction: A->C (token), C->B (card), B->A (mana).
	if combo := checkTripleCombo(a, b, c); combo == nil {
		t.Fatal("expected combo for reverse directed cycle, got nil")
	}
}
