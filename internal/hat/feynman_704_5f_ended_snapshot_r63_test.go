package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Regression for the recurring live-grinder finding (STATE-INTEGRITY,
// surface=feynman, rule 704.5f): a creature with toughness 0 flagged as
// "persisting on the battlefield" in the POST-GAME snapshot (e.g. Fertilid in
// the Gorma/Toph/Katara B-decks game).
//
// Root cause: the post-game Feynman oracle (hat.CheckGame → checkToughnessSBA)
// samples the ENDED game state. Once a game has ended (§104.2/§104.3a),
// gameengine.StateBasedActions returns early and runs no §704.5 checks
// (sba.go:43), so a creature whose toughness reached ≤0 in the same window the
// game resolved legitimately remains in that snapshot — the final SBA pass that
// would have binned it never runs. The in-game twin checkSBACompleteness
// already documents and applies this exact post-ended carve-out; the Feynman
// checker did NOT, so it flagged states the engine is correct to leave alone.
// The fix mirrors the in-game guard: skip the toughness SBA when ended==1.

// A genuine 0-toughness creature on a NON-Lost (winning) seat in the ENDED
// snapshot must NOT be flagged — the game is over and SBAs have ceased.
func TestFeynman_704_5f_EndedSnapshot_ZeroToughness_NotFlagged(t *testing.T) {
	gs := newFeynmanGame(t, 2)
	card := &gameengine.Card{
		Name:     "Fertilid",
		Owner:    0,
		Types:    []string{"creature"},
		TypeLine: "Creature — Elemental",
	}
	// Base 0/0, no counters left → layer toughness 0, still a creature.
	newTestPermanent(gs.Seats[0], card, 0, 0)
	gs.InvalidateCharacteristicsCache()

	// Game over: seat 0 won, seat 1 lost, ended flag set — exactly the
	// post-game snapshot the Feynman oracle runs on.
	gs.Seats[0].Won = true
	gs.Seats[1].Lost = true
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["ended"] = 1
	gs.Turn = 22

	// Precondition: the creature really is a layer-aware 0-toughness creature
	// (so absent the ended guard it WOULD be flagged — proving the guard, not
	// some other predicate, is what suppresses it).
	bf := gs.Seats[0].Battlefield
	if len(bf) != 1 || !gs.IsCreatureOf(bf[0]) || gs.ToughnessOf(bf[0]) != 0 {
		t.Fatalf("precondition: expected one layer-creature at toughness 0")
	}

	result := CheckGame(gs)
	for _, v := range result.Violations {
		if v.Name == "704.5f" {
			t.Errorf("ended-snapshot 0-toughness creature must NOT be flagged §704.5f (post-game SBAs ceased): %v", v)
		}
	}
}

// Guardrail: real-gap detection preserved. The SAME genuine 0-toughness
// creature, when the game has NOT ended (ended unset — e.g. a max-turns cap),
// MUST still be flagged. The fix narrows ONLY the post-ended snapshot, it does
// not blind the checker to an in-game §704.5f miss.
func TestFeynman_704_5f_NotEnded_ZeroToughness_StillFlagged(t *testing.T) {
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
	// ended NOT set — the game is still in progress for the oracle's purposes.

	result := CheckGame(gs)
	found := false
	for _, v := range result.Violations {
		if v.Name == "704.5f" {
			found = true
		}
	}
	if !found {
		t.Error("a genuine in-game creature at toughness 0 (ended unset) must still be flagged §704.5f")
	}
}
