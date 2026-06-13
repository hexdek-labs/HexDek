package gameengine

import "testing"

// sba_704_5f_toughness_r63_test.go — investigation of the live-fishtank
// §704.5f finding (Silverwing Squadron / Termagant Swarm at toughness 0 on
// the battlefield). Distinguishes a REAL SBA miss from a checker-timing FP:
// if StateBasedActions reliably removes a 0-toughness creature, the SBA
// path is sound and the live finding is a post-game-snapshot timing
// artifact.

// base 0/0 creature (Termagant Swarm shape with its Ravenous counters
// removed) → must die to §704.5f when SBAs run.
func TestSBA704_5f_RemovesBaseZeroToughness(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Termagant Swarm", 0, 0, "creature")
	if gs.ToughnessOf(p) != 0 {
		t.Fatalf("fixture: expected toughness 0, got %d", gs.ToughnessOf(p))
	}
	StateBasedActions(gs)
	for _, q := range gs.Seats[0].Battlefield {
		if q == p {
			t.Fatalf("§704.5f did NOT remove the 0-toughness creature — REAL SBA BUG")
		}
	}
}

// A 0/0 creature kept alive by +1/+1 counters, then the counters are
// removed → 0 toughness → must die on the next SBA pass. Models Termagant
// Swarm losing its Ravenous counters (proliferate-removal / -1/-1
// annihilation / Hex Parasite).
func TestSBA704_5f_RemovesAfterCounterLoss(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Termagant Swarm", 0, 0, "creature")
	p.Counters["+1/+1"] = 3
	gs.InvalidateCharacteristicsCache()
	if gs.ToughnessOf(p) != 3 {
		t.Fatalf("fixture: expected toughness 3 with counters, got %d", gs.ToughnessOf(p))
	}
	StateBasedActions(gs) // alive
	if !onBattlefield(gs, p) {
		t.Fatal("3-toughness creature wrongly removed")
	}
	// Counters wear off / removed.
	delete(p.Counters, "+1/+1")
	gs.InvalidateCharacteristicsCache()
	if gs.ToughnessOf(p) != 0 {
		t.Fatalf("expected toughness 0 after counter removal, got %d", gs.ToughnessOf(p))
	}
	StateBasedActions(gs)
	if onBattlefield(gs, p) {
		t.Fatalf("§704.5f did NOT remove the 0-toughness creature after counter loss — REAL SBA BUG")
	}
}

// THE FALSE-POSITIVE MECHANISM. A printed creature whose creature type is
// stripped by a continuous (layer-4) effect — Song of the Dryads turning
// it into a Forest land, Imprisoned in the Moon, etc. — is NO LONGER a
// creature, so §704.5f does not apply even though it now reads toughness
// 0 (a land has no P/T). sba704_5f uses the layer-aware gs.IsCreatureOf
// predicate and CORRECTLY leaves it on the battlefield. This pins the
// engine's correct behavior; the post-game feynman checker (which used
// base printed-type p.IsCreature) was the one that diverged and flagged
// the surviving non-creature. See the matching hat-package regression.
func TestSBA704_5f_SkipsTypeStrippedNonCreature(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Silverwing Squadron", 0, 0, "creature")
	// Layer-4 type-replace: becomes a (Forest) land, losing creature-ness.
	ts := gs.NextTimestamp()
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerType, Timestamp: ts,
		SourcePerm: p, SourceCardName: "Song of the Dryads",
		ControllerSeat: p.Controller,
		HandlerID:      "test_type_strip",
		ApplyFn: func(_ *GameState, q *Permanent, chars *Characteristics) {
			if q == p {
				chars.Types = []string{"land"}
			}
		},
	})
	gs.InvalidateCharacteristicsCache()

	if !p.IsCreature() {
		t.Fatal("base printed type should still be creature (p.IsCreature)")
	}
	if gs.IsCreatureOf(p) {
		t.Fatal("layer-aware type should be non-creature (gs.IsCreatureOf)")
	}
	if got := gs.ToughnessOf(p); got != 0 {
		t.Fatalf("expected toughness 0 as a land, got %d", got)
	}
	StateBasedActions(gs)
	if !onBattlefield(gs, p) {
		t.Fatalf("§704.5f WRONGLY removed a non-creature land at toughness 0 — SBA over-applied")
	}
}
