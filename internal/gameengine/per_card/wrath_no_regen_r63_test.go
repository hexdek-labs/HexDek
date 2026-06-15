package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// CR §701.15g — Wrath of God ("Destroy all creatures. They can't be
// regenerated.") must not be stopped by a regeneration shield. Regression
// for the r63 regenerate probe: board wipes routed through DestroyPermanent
// (which honors regen), so a creature with a regen shield survived Wrath —
// the "can't be regenerated" clause was a documented no-op.
func TestWrathOfGod_IgnoresRegenerationShield(t *testing.T) {
	gs := newGame(t, 2)
	shielded := addPerm(gs, 0, "Drudge Skeletons", "creature")
	gameengine.GrantRegenerationShield(gs, shielded)
	// A second creature with no shield as a sanity control.
	plain := addPerm(gs, 1, "Grizzly Bears", "creature")

	wrathOfGodResolve(gs, &gameengine.StackItem{Controller: 0,
		Card: &gameengine.Card{Name: "Wrath of God", Owner: 0, Types: []string{"sorcery"}}})

	for _, p := range gs.Seats[0].Battlefield {
		if p == shielded {
			t.Error("Wrath of God must destroy a regen-shielded creature (can't be regenerated)")
		}
	}
	for _, p := range gs.Seats[1].Battlefield {
		if p == plain {
			t.Error("Wrath of God must destroy the unshielded creature too")
		}
	}
}
