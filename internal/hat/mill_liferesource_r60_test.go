package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mill_liferesource_r60_test.go — follow-up to PR #194's gauntlet
// finding. The cross-cutting late-game LifeResource bump from #191
// (`w.LifeResource *= 1.0 + lateFactor*0.15`) pushed Mill's effective
// late-game weight above midrange, contradicting Mill's gameplan
// reality: the deck WANTS to trade life for mill progress (take damage
// to keep mana up for Maddening Cacophony / Bruvac doubler triggers).
// Dropping Mill's baseline LifeResource to 0.3 lets the global bump
// land it at 0.345 late-game — below midrange's late-game 0.575 —
// restoring the "life is cheaper for Mill" property.

func TestMillWeights_LifeResourceBelowMidrange(t *testing.T) {
	mill := DefaultWeightsForArchetype(ArchetypeMill)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if mill.LifeResource >= mid.LifeResource {
		t.Errorf("Mill LifeResource (%.2f) must be < midrange (%.2f) — Mill trades life for mill progress",
			mill.LifeResource, mid.LifeResource)
	}
}

func TestMillWeights_LifeResourceMatchesGauntletTuning(t *testing.T) {
	// Hard-pin the chosen value (0.3) so regressions are obvious. The
	// number is load-bearing: the late-game multiplier (×1.15 at
	// lateFactor=1.0) lands it at 0.345 — comfortably under midrange's
	// 0.575 even after the global bump.
	mill := DefaultWeightsForArchetype(ArchetypeMill)
	if mill.LifeResource != 0.3 {
		t.Errorf("Mill LifeResource pinned at 0.3 by R60 follow-up; got %.2f", mill.LifeResource)
	}
}

// TestMillWeights_LateGameStillBelowMidrange verifies the actual
// late-game effective weight (post-rescaleWeights bump) keeps Mill
// below midrange. This is the property the gauntlet measured.
func TestMillWeights_LateGameEffectiveBelowMidrange(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	for i := 1; i < 4; i++ {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	gs.Turn = 25 // lateFactor ≈ 1.0 at this turn

	millEv := NewEvaluator(&StrategyProfile{Archetype: ArchetypeMill})
	wMill := millEv.rescaleWeights(gs, 0)

	midEv := NewEvaluator(&StrategyProfile{Archetype: ArchetypeMidrange})
	wMid := midEv.rescaleWeights(gs, 0)

	if wMill.LifeResource >= wMid.LifeResource {
		t.Errorf("late-game effective LifeResource: Mill (%.3f) must be < midrange (%.3f) after global late-bump",
			wMill.LifeResource, wMid.LifeResource)
	}
}

// TestMillWeights_StackInteractionUnchangedByLifeBump verifies the
// LifeResource drop didn't inadvertently disturb the rest of Mill's
// R60 round-1 tuning. The signature dimensions (CardAdvantage / Stack-
// Interaction / DrainEngine / ToolboxBreadth / ComboProximity) must
// all still clear their gauntlet thresholds.
func TestMillWeights_OtherDimensionsHoldR1Tuning(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMill)
	cases := []struct {
		field   string
		got     float64
		want    float64
	}{
		{"CardAdvantage", w.CardAdvantage, 1.2},     // R1 floor
		{"StackInteraction", w.StackInteraction, 1.0}, // R1 floor
		{"DrainEngine", w.DrainEngine, 0.7},          // R1 floor
		{"ToolboxBreadth", w.ToolboxBreadth, 0.6},    // R1 floor
		{"ComboProximity", w.ComboProximity, 1.0},    // existing floor
	}
	for _, c := range cases {
		if c.got < c.want {
			t.Errorf("Mill %s = %.2f, must remain ≥ %.2f (R60 round 1 contract)",
				c.field, c.got, c.want)
		}
	}
}

// Sanity: Storm and Aristocrats — the other "life is tradeable" decks
// — should ALSO have LifeResource below midrange. Storm already does
// (baseline 0.2); Aristocrats does not (baseline 0.5, unchanged). The
// test pins Storm's existing low value to flag any future regression
// in the same shape, and intentionally allows Aristocrats > Mill since
// Aristocrats also runs Blood Artist / Disciple of the Vault lifegain.
func TestStormWeights_LifeResourceStaysLowForGauntletShape(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeStorm)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.LifeResource >= mid.LifeResource {
		t.Errorf("Storm LifeResource (%.2f) must remain < midrange (%.2f) for same gauntlet-shape reason as Mill",
			w.LifeResource, mid.LifeResource)
	}
}

// Helper for the engine state — re-declared here to avoid coupling
// across test files. Matches the pattern used by evaluator_test.go.
var _ = gameengine.NewGameState // keep import lint-quiet
