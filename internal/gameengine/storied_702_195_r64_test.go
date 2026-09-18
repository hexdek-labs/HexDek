package gameengine

import "testing"

// TestStoried_702_195 pins the new Storied keyword (CR §702.195): three or
// more permanents that are artifacts, Sagas, and/or legendary grant the
// "enduring story" designation, which is permanent for the rest of the game.
// Built on the Ascend machinery with a filtered count of 3.
func TestStoried_702_195(t *testing.T) {
	gs := newCombatGame(t)

	// 1 qualifier (artifact) + a plain creature that does NOT qualify.
	addBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	addBattlefield(gs, 0, "Plain Bear", 2, 2, "creature")
	CheckStoried(gs, 0)
	if HasEnduringStory(gs, 0) {
		t.Fatal("1 qualifying permanent must not grant enduring story")
	}

	// Add a Saga and a legendary → 3 qualifiers (artifact + Saga + legendary).
	addBattlefield(gs, 0, "A Saga", 0, 0, "enchantment", "saga")
	addBattlefield(gs, 0, "Legend", 3, 3, "legendary", "creature")
	CheckStoried(gs, 0)
	if !HasEnduringStory(gs, 0) {
		t.Fatal("3 artifact/Saga/legendary permanents must grant enduring story")
	}

	// Permanent designation: persists even if all permanents leave (§702.195a
	// "for the rest of the game").
	gs.Seats[0].Battlefield = nil
	CheckStoried(gs, 0)
	if !HasEnduringStory(gs, 0) {
		t.Error("enduring story must persist for the rest of the game")
	}
}
