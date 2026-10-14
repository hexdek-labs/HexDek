package per_card

// r102 — CR §302.6 driven through two REAL cards, one per arm of the
// class: a permanent steal that prints no haste, and a mass steal that
// prints haste. See internal/gameengine/control_change_302_6_r102.go.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r102Settled: a creature that has been under this seat's control for
// turns. A freshly-played creature is already summoning-sick, which is
// what made the defect invisible.
func r102Settled(gs *gameengine.GameState, seat int, name string) *gameengine.Permanent {
	p := r87Body(addPerm(gs, seat, name, "creature"), 3, 3)
	p.SummoningSick = false
	p.Tapped = false
	return p
}

// Roil Elemental: "Landfall — Whenever a land enters under your control,
// gain control of target creature for as long as you control Roil
// Elemental." No haste anywhere on the card.
func TestRoilElemental_StolenCreatureIsSummoningSick_r102(t *testing.T) {
	gs := newGame(t, 2)
	roil := r87Body(addPerm(gs, 0, "Roil Elemental", "creature"), 2, 2)
	victim := r102Settled(gs, 1, "Settled Opponent Creature")

	if victim.SummoningSick {
		t.Fatal("precondition: the victim must start NOT summoning-sick")
	}

	land := addPerm(gs, 0, "Evolving Wilds", "land")
	roilElementalLandfall(gs, roil, map[string]interface{}{"perm": land})

	if victim.Controller != 0 {
		t.Fatalf("precondition: Roil Elemental should steal the creature, controller=%d", victim.Controller)
	}
	if !victim.SummoningSick {
		t.Error("CR 302.6: Roil Elemental prints no haste — the stolen creature has not " +
			"been under seat 0's control since seat 0's most recent turn began")
	}
	if gs.HasKeywordOf(victim, "haste") {
		t.Error("fixture guard: nothing should have granted haste here")
	}
}

// Insurrection: "Untap all creatures and gain control of them until end
// of turn. They gain haste until end of turn." The §302.6 stamp must NOT
// reach this card — the reparent happens first precisely so the printed
// haste clear lands after it.
func TestInsurrection_StolenCreaturesStayHasty_r102(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	victim := r102Settled(gs, 1, "Conscripted Beast")
	victim.Tapped = true

	insurrectionResolve(gs, &gameengine.StackItem{
		Card:       &gameengine.Card{Name: "Insurrection"},
		Controller: 0,
	})

	if victim.Controller != 0 {
		t.Fatalf("precondition: Insurrection should take the creature, controller=%d", victim.Controller)
	}
	if victim.Tapped {
		t.Error("Insurrection untaps the creatures it takes")
	}
	if victim.SummoningSick {
		t.Error("Insurrection prints haste: the §302.6 stamp must be cleared AFTER the reparent, not before")
	}
	if !gs.HasKeywordOf(victim, "haste") {
		t.Error("Insurrection grants haste until end of turn")
	}

	// And the §514.2 revert hands it back summoning-sick — the owner's
	// control was interrupted, so it was not continuous.
	gameengine.FireCleanupStepExpiries(gs)
	if victim.Controller != 1 {
		t.Fatalf("precondition: control must revert to the owner at cleanup, got %d", victim.Controller)
	}
	if !victim.SummoningSick {
		t.Error("CR 302.6: coming home is a control change too — the owner did not have it continuously")
	}
}
