package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Regression for the live-fishtank §704.5f finding (r63): the post-game
// Feynman checker flagged "Silverwing Squadron" / "Termagant Swarm" as
// creatures sitting at toughness 0 on the battlefield, while the engine's
// state_integrity scored 100% on chaos/loki sweeps.
//
// Root cause: checkToughnessSBA used the BASE printed-type predicate
// (p.IsCreature) while the engine's sba704_5f uses the LAYER-AWARE
// predicate (gs.IsCreatureOf). When a continuous layer-4 effect strips
// the creature type — Song of the Dryads turning a creature into a Forest
// land, Imprisoned in the Moon, etc. — the permanent is correctly NO
// LONGER a creature, reads toughness 0 (a land has no P/T), and §704.5f
// does not apply. sba704_5f correctly leaves it on the battlefield; the
// post-game feynman check, reading the printed type, wrongly flagged it.
//
// The fix aligns the checker's predicate with the SBA's (gs.IsCreatureOf).
// A genuine layer-creature at toughness 0 is still flagged (IsCreatureOf
// true), so the fix narrows ONLY to the non-creature false positive.
func TestFeynman_704_5f_TypeStrippedNonCreature_NotFlagged(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	// Printed creature with base 0/0 — its runtime type is still creature
	// (p.IsCreature stays true), mirroring Termagant Swarm / a CDA */*
	// creature whose layer toughness lands on 0.
	card := &gameengine.Card{
		Name:     "Silverwing Squadron",
		Owner:    0,
		Types:    []string{"creature"},
		TypeLine: "Creature — Human Knight",
	}
	p := newTestPermanent(gs.Seats[0], card, 0, 0)

	// Layer-4 continuous effect strips creature-ness (becomes a land).
	ts := gs.NextTimestamp()
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer: gameengine.LayerType, Timestamp: ts,
		SourcePerm: p, SourceCardName: "Song of the Dryads",
		ControllerSeat: p.Controller,
		HandlerID:      "feynman_test_type_strip",
		ApplyFn: func(_ *gameengine.GameState, q *gameengine.Permanent, chars *gameengine.Characteristics) {
			if q == p {
				chars.Types = []string{"land"}
			}
		},
	})
	gs.InvalidateCharacteristicsCache()

	// Precondition checks: base type creature, layer type non-creature,
	// toughness 0 — exactly the live finding's shape.
	if !p.IsCreature() {
		t.Fatal("precondition: base printed type should still be creature")
	}
	if gs.IsCreatureOf(p) {
		t.Fatal("precondition: layer-aware type should be non-creature")
	}
	if got := gs.ToughnessOf(p); got != 0 {
		t.Fatalf("precondition: expected toughness 0, got %d", got)
	}

	gs.Seats[1].Lost = true
	gs.Turn = 12

	result := CheckGame(gs)
	for _, v := range result.Violations {
		if v.Name == "704.5f" {
			t.Errorf("type-stripped non-creature at toughness 0 must NOT be flagged §704.5f: %v", v)
		}
	}
}

// Guardrail: a GENUINE layer-creature at toughness 0 (no type strip) must
// STILL be flagged — the fix narrows only the non-creature FP, it does not
// blind the checker to a real SBA miss.
func TestFeynman_704_5f_GenuineCreatureZeroToughness_StillFlagged(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	card := &gameengine.Card{
		Name:     "Real Zero Creature",
		Owner:    0,
		Types:    []string{"creature"},
		TypeLine: "Creature — Horror",
	}
	newTestPermanent(gs.Seats[0], card, 0, 0)
	gs.InvalidateCharacteristicsCache()
	gs.Seats[1].Lost = true
	gs.Turn = 12

	result := CheckGame(gs)
	found := false
	for _, v := range result.Violations {
		if v.Name == "704.5f" {
			found = true
		}
	}
	if !found {
		t.Error("a genuine creature at toughness 0 must still be flagged §704.5f")
	}
}
