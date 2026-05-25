package trueskill

import (
	"math"
	"testing"
)

// composition_update_test.go — synthetic 4-deck-pod regressions for
// the composition-prior-aware TrueSkill update. Validates that:
//   - With nil prior / zero weight / cold start, behavior matches
//     standard Update exactly (no silent regressions).
//   - With a populated prior, players in composition-favored seats
//     get LESS μ credit for winning and players in composition-
//     disfavored seats get MORE μ credit — the prior correctly
//     dampens the over-attribution.

// makeSeededPrior builds a prior with confidence > 0 for a given
// pod by simulating n games where the named winner wins all of them.
// Used to construct controlled "Mill always wins this pod" scenarios.
func makeSeededPrior(t *testing.T, n int, pod []string, winner string) *CompositionPrior {
	t.Helper()
	cp := NewCompositionPrior(len(pod))
	for i := 0; i < n; i++ {
		if err := cp.ObserveGame(pod, winner); err != nil {
			t.Fatalf("seed game %d: %v", i, err)
		}
	}
	return cp
}

// runTwoUpdates runs the standard Update on tsA and UpdateWithComposition
// on tsB with identical inputs, then returns each player's μ-after-update
// for comparison.
func runUpdates(participantNames, archetypes []string, ranks []int, prior *CompositionPrior, weight float64) (stdMu, priorMu []float64) {
	stdTS := NewTrueSkillRatings(participantNames)
	priorTS := NewTrueSkillRatings(participantNames)
	stdTS.Update(participantNames, ranks)
	priorTS.UpdateWithComposition(participantNames, ranks, archetypes,
		CompositionUpdateConfig{Prior: prior, Weight: weight, MuOffsetScale: 10})
	stdMu = make([]float64, len(participantNames))
	priorMu = make([]float64, len(participantNames))
	for i, name := range participantNames {
		stdMu[i] = stdTS.Ratings[name].Mu
		priorMu[i] = priorTS.Ratings[name].Mu
	}
	return
}

// -----------------------------------------------------------------------------
// Degenerate cases: prior-aware update reduces to standard Update
// -----------------------------------------------------------------------------

func TestUpdateWithComposition_NilPriorMatchesStandard(t *testing.T) {
	names := []string{"P1", "P2", "P3", "P4"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	ranks := []int{0, 1, 2, 3}
	stdMu, priorMu := runUpdates(names, archs, ranks, nil, 0.5)
	for i := range names {
		if math.Abs(stdMu[i]-priorMu[i]) > 1e-9 {
			t.Errorf("%s: nil-prior update should match standard; std=%.4f prior=%.4f",
				names[i], stdMu[i], priorMu[i])
		}
	}
}

func TestUpdateWithComposition_ZeroWeightMatchesStandard(t *testing.T) {
	names := []string{"P1", "P2", "P3", "P4"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	ranks := []int{0, 1, 2, 3}
	// Build a HIGH-confidence prior to ensure the offsets WOULD be
	// nonzero — but weight=0 must zero them out.
	prior := makeSeededPrior(t, 500, archs, "Mill")
	stdMu, priorMu := runUpdates(names, archs, ranks, prior, 0.0)
	for i := range names {
		if math.Abs(stdMu[i]-priorMu[i]) > 1e-9 {
			t.Errorf("%s: weight=0 update should match standard; std=%.4f prior=%.4f",
				names[i], stdMu[i], priorMu[i])
		}
	}
}

func TestUpdateWithComposition_ColdStartPriorMatchesStandard(t *testing.T) {
	names := []string{"P1", "P2", "P3", "P4"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	ranks := []int{0, 1, 2, 3}
	prior := NewCompositionPrior(4) // empty → confidence=0 everywhere
	stdMu, priorMu := runUpdates(names, archs, ranks, prior, 0.5)
	for i := range names {
		if math.Abs(stdMu[i]-priorMu[i]) > 1e-9 {
			t.Errorf("%s: cold-start prior should match standard; std=%.4f prior=%.4f",
				names[i], stdMu[i], priorMu[i])
		}
	}
}

func TestUpdateWithComposition_ArchetypeLengthMismatchFallsBack(t *testing.T) {
	names := []string{"P1", "P2", "P3", "P4"}
	wrongArchs := []string{"Mill", "Voltron"} // length 2, want 4
	ranks := []int{0, 1, 2, 3}
	prior := makeSeededPrior(t, 100, []string{"Mill", "Voltron", "Aggro", "Combo"}, "Mill")

	priorTS := NewTrueSkillRatings(names)
	stdTS := NewTrueSkillRatings(names)
	priorTS.UpdateWithComposition(names, ranks, wrongArchs,
		CompositionUpdateConfig{Prior: prior, Weight: 0.5})
	stdTS.Update(names, ranks)

	for _, name := range names {
		if math.Abs(stdTS.Ratings[name].Mu-priorTS.Ratings[name].Mu) > 1e-9 {
			t.Errorf("%s: archetype length mismatch should fall back to standard Update", name)
		}
	}
}

// -----------------------------------------------------------------------------
// Active case: prior dampens over-attribution to composition-favored decks
// -----------------------------------------------------------------------------

func TestUpdateWithComposition_FavoredWinnerGainsLessThanStandard(t *testing.T) {
	// Scenario: in pod {Mill, Voltron, Aggro, Combo}, Mill historically
	// wins 90% of games — the prior knows Mill is favored. When Mill
	// wins again, the prior should credit LESS μ to Mill (it was
	// expected to win) compared to standard TrueSkill.
	names := []string{"P1", "P2", "P3", "P4"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	ranks := []int{0, 1, 2, 3} // Mill (P1) wins
	prior := makeSeededPrior(t, 500, archs, "Mill") // Mill wins all 500 prior games

	stdMu, priorMu := runUpdates(names, archs, ranks, prior, 0.5)

	// Mill (winner, favored) should gain LESS in the prior-aware update.
	if priorMu[0] >= stdMu[0] {
		t.Errorf("favored Mill winner should gain less μ with prior: std=%.4f prior=%.4f",
			stdMu[0], priorMu[0])
	}
	// Other players (losers in disfavored archetypes) should LOSE less.
	for i := 1; i < 4; i++ {
		stdDelta := stdMu[i] - defaultMu  // negative (they lost)
		priorDelta := priorMu[i] - defaultMu
		if priorDelta <= stdDelta {
			t.Errorf("%s (loser, disfavored): prior should preserve μ better; std-delta=%.4f prior-delta=%.4f",
				names[i], stdDelta, priorDelta)
		}
	}
}

func TestUpdateWithComposition_DisfavoredWinnerGainsMoreThanStandard(t *testing.T) {
	// Inverse scenario: Voltron is HEAVILY disfavored in this pod (Mill
	// has historically won everything). If Voltron wins anyway, the
	// prior should credit MORE μ to Voltron because that win went
	// against the composition expectation.
	names := []string{"P1", "P2", "P3", "P4"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	ranks := []int{1, 0, 2, 3} // Voltron (P2) wins, Mill (P1) second
	prior := makeSeededPrior(t, 500, archs, "Mill") // Mill historically dominant

	stdMu, priorMu := runUpdates(names, archs, ranks, prior, 0.5)

	// Voltron (winner, disfavored) should gain MORE in prior-aware.
	if priorMu[1] <= stdMu[1] {
		t.Errorf("disfavored Voltron winner should gain more μ with prior: std=%.4f prior=%.4f",
			stdMu[1], priorMu[1])
	}
	// Mill (loser, favored) should LOSE more — it under-performed.
	stdMillDelta := stdMu[0] - defaultMu
	priorMillDelta := priorMu[0] - defaultMu
	if priorMillDelta >= stdMillDelta {
		t.Errorf("favored Mill loser should lose more μ: std-delta=%.4f prior-delta=%.4f",
			stdMillDelta, priorMillDelta)
	}
}

// -----------------------------------------------------------------------------
// Bookkeeping invariants
// -----------------------------------------------------------------------------

func TestUpdateWithComposition_GamesAndHistoryUpdated(t *testing.T) {
	names := []string{"P1", "P2", "P3", "P4"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	ranks := []int{0, 1, 2, 3}
	prior := makeSeededPrior(t, 200, archs, "Mill")

	ts := NewTrueSkillRatings(names)
	ts.UpdateWithComposition(names, ranks, archs,
		CompositionUpdateConfig{Prior: prior, Weight: 0.5, MuOffsetScale: 10})

	for _, name := range names {
		if ts.Games[name] != 1 {
			t.Errorf("%s.Games = %d, want 1", name, ts.Games[name])
		}
		if len(ts.History[name]) != 1 {
			t.Errorf("%s.History len = %d, want 1", name, len(ts.History[name]))
		}
	}
}

func TestUpdateWithComposition_SigmaShrinksLikeStandard(t *testing.T) {
	// σ updates flow through unchanged by the offset (offset-invariant
	// part of the Gaussian update). With the same input, both standard
	// and prior-aware should shrink σ by similar amounts.
	names := []string{"P1", "P2", "P3", "P4"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	ranks := []int{0, 1, 2, 3}
	prior := makeSeededPrior(t, 200, archs, "Mill")

	stdTS := NewTrueSkillRatings(names)
	priorTS := NewTrueSkillRatings(names)
	stdTS.Update(names, ranks)
	priorTS.UpdateWithComposition(names, ranks, archs,
		CompositionUpdateConfig{Prior: prior, Weight: 0.5, MuOffsetScale: 10})

	for i, name := range names {
		stdSigma := stdTS.Ratings[name].Sigma
		priorSigma := priorTS.Ratings[name].Sigma
		// Allow small divergence because the rank-prediction in
		// shifted space slightly perturbs σ updates too. Should be
		// within ~5%.
		relDiff := math.Abs(stdSigma-priorSigma) / stdSigma
		if relDiff > 0.05 {
			t.Errorf("%s (seat %d): σ differs by %.1f%% (std=%.4f prior=%.4f)",
				name, i, relDiff*100, stdSigma, priorSigma)
		}
	}
}

// -----------------------------------------------------------------------------
// End-to-end: many games converge to skill-corrected ratings
// -----------------------------------------------------------------------------

func TestUpdateWithComposition_HighWeightFlattensFavoredArchetypeRating(t *testing.T) {
	// Stream 100 games where Mill always wins. With weight=1.0 the
	// prior should "explain" all of Mill's wins as composition-driven,
	// keeping Mill's μ close to its starting value. With weight=0
	// (standard TrueSkill) Mill's μ rises substantially.
	names := []string{"P1", "P2", "P3", "P4"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	ranks := []int{0, 1, 2, 3} // Mill always wins

	prior := makeSeededPrior(t, 500, archs, "Mill")

	stdTS := NewTrueSkillRatings(names)
	priorTS := NewTrueSkillRatings(names)
	for i := 0; i < 100; i++ {
		stdTS.Update(names, ranks)
		priorTS.UpdateWithComposition(names, ranks, archs,
			CompositionUpdateConfig{Prior: prior, Weight: 1.0, MuOffsetScale: 10})
	}

	stdMillMu := stdTS.Ratings["P1"].Mu
	priorMillMu := priorTS.Ratings["P1"].Mu

	// Standard TrueSkill should have Mill significantly above starting μ.
	if stdMillMu-defaultMu < 5 {
		t.Errorf("standard TrueSkill should give Mill >5 μ gain after 100 dominant games; got %.4f",
			stdMillMu-defaultMu)
	}
	// Prior-aware should keep Mill closer to start.
	if priorMillMu >= stdMillMu {
		t.Errorf("weight=1.0 prior should give LESS Mill μ than standard; std=%.4f prior=%.4f",
			stdMillMu, priorMillMu)
	}
}
