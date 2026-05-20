package deckid

import "testing"

func TestCardDelta_DistinctCards(t *testing.T) {
	parent := []string{"1x ancestral recall", "1x black lotus", "1x mox jet"}
	child := []string{"1x ancestral recall", "1x black lotus", "1x mox sapphire"}
	if got := CardDelta(parent, child); got != 2 {
		t.Errorf("CardDelta with one swap: got %d, want 2 (1 added + 1 removed)", got)
	}
}

func TestCardDelta_DuplicateBasics(t *testing.T) {
	// 30 Plains vs 35 Plains — the historical r46 implementation collapsed
	// duplicates into a set and returned 0. The fix must count multiplicity.
	parent := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		parent = append(parent, "1x plains")
	}
	child := make([]string, 0, 35)
	for i := 0; i < 35; i++ {
		child = append(child, "1x plains")
	}
	if got := CardDelta(parent, child); got != 5 {
		t.Errorf("CardDelta 30-Plains vs 35-Plains: got %d, want 5", got)
	}
}

func TestCardDelta_Identical(t *testing.T) {
	list := []string{"1x sol ring", "1x sol ring", "1x command tower"}
	if got := CardDelta(list, list); got != 0 {
		t.Errorf("CardDelta identical lists: got %d, want 0", got)
	}
}

func TestCardDelta_EmptyBothSides(t *testing.T) {
	if got := CardDelta(nil, nil); got != 0 {
		t.Errorf("CardDelta nil/nil: got %d, want 0", got)
	}
}

func TestCardDelta_AsymmetricBasics(t *testing.T) {
	// Parent has 5 Forest 3 Mountain; child has 3 Forest 5 Mountain.
	// Symmetric swap of 2 basics in each direction → delta = 4.
	parent := []string{"1x forest", "1x forest", "1x forest", "1x forest", "1x forest",
		"1x mountain", "1x mountain", "1x mountain"}
	child := []string{"1x forest", "1x forest", "1x forest",
		"1x mountain", "1x mountain", "1x mountain", "1x mountain", "1x mountain"}
	if got := CardDelta(parent, child); got != 4 {
		t.Errorf("CardDelta basic-swap: got %d, want 4 (2 forest removed + 2 mountain added)", got)
	}
}
