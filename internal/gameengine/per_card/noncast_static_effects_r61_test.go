package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// noncast_static_effects_r61_test.go — repro for the 2026-06-10 "clauses
// getting missed" cluster in a Coram (graveyard/reanimator) deck.
//
// Root: RegisterContinuousEffectsForPermanent (which wires named layer
// handlers like Doran/Blood Moon/Humility AND generic AST anthem/keyword
// statics via registerASTStaticEffects) was called ONLY from the cast path
// (stack.go:resolvePermanentSpellETB). Every NON-cast entry —
// reanimate (resolve.go:resolveReanimate → FirePermanentETBTriggers),
// token-mint, blink — went through FirePermanentETBTriggers, which never
// registered continuous effects. So a reanimated/tokened/blinked permanent
// silently lost all its static abilities.

func TestNonCastEntry_RegistersStaticAbilities_Doran(t *testing.T) {
	gs := newGame(t, 2)
	// Simulate a non-cast entry exactly as resolveReanimate / token-mint /
	// blink do: place the permanent, then fire the shared ETB dispatcher.
	doran := addPerm(gs, 0, "Doran, the Siege Tower", "creature", "legendary")
	gameengine.FirePermanentETBTriggers(gs, doran)

	found := false
	for _, ce := range gs.ContinuousEffects {
		if ce != nil && ce.SourcePerm == doran {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Doran entered via the non-cast (reanimate/token/blink) path " +
			"registered NO continuous effect — static abilities silently dropped")
	}
}
