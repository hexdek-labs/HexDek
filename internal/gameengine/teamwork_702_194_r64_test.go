package gameengine

import "testing"

// TestTapCreaturesForTotalPower_702_194 pins the shared tap-for-total-power
// primitive behind Teamwork (and crew/saddle): a valid set meeting the power
// requirement is tapped; an insufficient or invalid set taps nothing.
func TestTapCreaturesForTotalPower_702_194(t *testing.T) {
	gs := newCombatGame(t)
	a := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	b := addBattlefield(gs, 0, "Ogre", 3, 3, "creature")

	// Total power 5 >= need 4 → taps both, returns true.
	if !TapCreaturesForTotalPower(gs, 0, 4, []*Permanent{a, b}) {
		t.Fatal("power 5 should satisfy teamwork 4")
	}
	if !a.Tapped || !b.Tapped {
		t.Error("both chosen creatures should be tapped")
	}

	// Insufficient: a single power-2 creature cannot pay need 4, taps nothing.
	c := addBattlefield(gs, 0, "Small", 2, 2, "creature")
	if TapCreaturesForTotalPower(gs, 0, 4, []*Permanent{c}) {
		t.Error("power 2 must not satisfy teamwork 4")
	}
	if c.Tapped {
		t.Error("a failed payment must not tap anything")
	}

	// Already-tapped creature is invalid input.
	d := addBattlefield(gs, 0, "Tapped", 5, 5, "creature")
	d.Tapped = true
	if TapCreaturesForTotalPower(gs, 0, 4, []*Permanent{d}) {
		t.Error("an already-tapped creature cannot pay a tap cost")
	}
}
