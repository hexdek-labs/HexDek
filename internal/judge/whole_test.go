package judge_test

// whole_test.go — the Hex Judge completeness pin (phase 3 final): every
// dimension in the registry reports through the ONE LogViolation seam.
// External test package so it can exercise gameengine + the judge
// sub-packages together without import cycles.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/judge/outcome"
	"github.com/hexdek/hexdek/internal/judge/progression"
)

// TestJudgeWhole_AllFiveDimensionsReport produces one violation per
// dimension through that dimension's real origin path and asserts a
// single registered sink observes all five, each tagged with its
// dimension. Detection-correctness of each check is pinned by its home
// package's own tests; THIS pin is the routing completeness claim the
// registry makes.
func TestJudgeWhole_AllFiveDimensionsReport(t *testing.T) {
	seen := map[string]int{}
	done := judge.RegisterSink(func(v judge.ValidationViolation) {
		if v.Dimension != "" {
			seen[v.Dimension]++
		}
	})
	defer done()

	// CONSERVATION — fabricated InstanceID trips the strict census via
	// the invariant table.
	gs := gameengine.NewGameState(2, nil, nil)
	gs.MintedInstanceIDs = map[string]struct{}{"h0OG-real": {}}
	gs.CeasedInstanceIDs = map[string]struct{}{}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		&gameengine.Card{Name: "Forged", Owner: 0, InstanceID: "h0OG-forged"})
	gameengine.RunAllInvariants(gs)

	// STATE-INTEGRITY — unmarked poison loss through the end-of-game
	// snapshot checks.
	judge.CheckStateIntegrity(judge.GameSnapshot{
		TotalSeats: 2,
		Seats:      []judge.SeatSnapshot{{Seat: 0, Life: 40, PoisonCounters: 12}},
	})

	// LEGALITY — the action validator's canonical record shape (the
	// record-time tap itself is pinned by gameengine's router tests).
	judge.LogViolation(gameengine.LegalityViolation{
		Turn: 3, Seat: 1, Action: "cast:Test", Rule: "601.2f", Detail: "underpaid",
	}.Canonical())

	// OUTCOME — an expected-vs-actual divergence finding through the
	// origin emitter RunEffect uses.
	outcome.EmitFinding(&outcome.Finding{
		CardName: "Test Bolt", Kind: "damage",
		Expected: "life[1]-3", Actual: "no change", Raw: "deal 3 damage",
	})

	// PROGRESSION — a missed-trigger finding through the origin emitter
	// the Check* scenario functions use.
	progression.EmitFinding(&progression.Finding{
		CardName: "Test Herald", Event: "etb", Check: "fire",
		Expected: "draw 1", Actual: "no change", Raw: "when this enters, draw a card",
	})

	for _, reg := range judge.Dimensions() {
		if seen[reg.Dimension] == 0 {
			t.Errorf("dimension %q is registered but produced no violation through LogViolation", reg.Dimension)
		}
	}
	if len(judge.Dimensions()) != 5 {
		t.Fatalf("the Judge registry must hold exactly 5 dimensions, got %d", len(judge.Dimensions()))
	}
}

// TestJudgeRegistry_DimensionsDistinctAndTagged pins registry hygiene.
func TestJudgeRegistry_DimensionsDistinctAndTagged(t *testing.T) {
	seenDim := map[string]bool{}
	for _, reg := range judge.Dimensions() {
		if seenDim[reg.Dimension] {
			t.Errorf("duplicate dimension %q", reg.Dimension)
		}
		seenDim[reg.Dimension] = true
		if len(reg.Surfaces) == 0 || reg.Home == "" || reg.Hook == "" || reg.Mode == "" {
			t.Errorf("dimension %q has an incomplete registration: %+v", reg.Dimension, reg)
		}
	}
	for _, want := range []string{
		judge.DimensionLegality, judge.DimensionConservation,
		judge.DimensionStateIntegrity, judge.DimensionProgression,
		judge.DimensionOutcome,
	} {
		if !seenDim[want] {
			t.Errorf("dimension %q missing from the registry", want)
		}
	}
}
