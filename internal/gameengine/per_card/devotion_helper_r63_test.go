package per_card

import (
	"testing"
)

// TestCountDevotion_ReadsRealManaCost pins the fix to the per_card devotion
// helper (countDevotion / countPipsOnCard). It previously read ONLY "pip:U"
// Type markers — present on hand-built fixtures but never on corpus cards —
// so every real-game consumer routed through it (Thassa's Oracle, Xenagos,
// Namor) silently counted 0 devotion. Now it reads the printed
// Card.ManaCostString via the canonical pip parser.
func TestCountDevotion_ReadsRealManaCost(t *testing.T) {
	gs := newGame(t, 2)

	// Thassa {3}{U}{U}: 2 blue pips. Plus a {1}{U}: 1 blue pip.
	p1 := addPerm(gs, 0, "Thassa, God of the Sea", "creature")
	p1.Card.ManaCostString = "{3}{U}{U}"
	p2 := addPerm(gs, 0, "Cheap Blue", "creature")
	p2.Card.ManaCostString = "{1}{U}"

	if got := countDevotion(gs.Seats[0], "U"); got != 3 {
		t.Fatalf("countDevotion must read real ManaCostString pips; want 3, got %d (pip:-only regression?)", got)
	}
}

// TestCountDevotion_HybridTwoColorReal verifies the helper's two-color path
// (used by Xenagos' R+G defensive gate) reads real costs and counts hybrid
// pips toward both colors.
func TestCountDevotion_HybridTwoColorReal(t *testing.T) {
	gs := newGame(t, 2)

	// {R}{G} → 1 red + 1 green = 2 toward "red and green".
	p1 := addPerm(gs, 0, "Gruul Beast", "creature")
	p1.Card.ManaCostString = "{R}{G}"
	// {R/G} hybrid → counts toward both; summed across R and G = 2.
	p2 := addPerm(gs, 0, "Hybrid Brute", "creature")
	p2.Card.ManaCostString = "{R/G}"

	got := countDevotion(gs.Seats[0], "R", "G")
	if got != 4 {
		t.Fatalf("two-color devotion over real mana costs; want 4 (R+G: 1+1 + hybrid 1+1), got %d", got)
	}
}
