package tournament

import (
	"testing"
)

// R60 sigma-weighted DE pairing regressions. Pre-fix
// pairDoubleElimRound used (losses asc, pods asc, shuffle) with no
// TrueSkill σ input — every round threw away the per-deck
// uncertainty signal that ts.Ratings carried. Now σ is the third
// sort key within a losses+pods tier so high-σ decks pod up
// together (max info gain per match) and low-σ decks pod up
// together (don't disturb settled posteriors).

// allLosses constructs a state slice of length n where every deck
// has zero losses + zero pods (round-1-like baseline).
func allLosses(n int) []deckState {
	out := make([]deckState, n)
	return out
}

// TestPairDoubleElim_RoundOneIsSigmaInsensitive pins that when
// every deck shares the default σ (round 1, no games played yet),
// sub-epsilon sigma differences fall through to the shuffle
// tiebreak — preserving the existing first-round behavior.
func TestPairDoubleElim_RoundOneIsSigmaInsensitive(t *testing.T) {
	state := allLosses(8)
	eligible := []int{0, 1, 2, 3, 4, 5, 6, 7}
	// All decks at default sigma 25/3 ≈ 8.333.
	sigmas := []float64{8.333, 8.333, 8.333, 8.333, 8.333, 8.333, 8.333, 8.333}

	pods := pairDoubleElimRound(eligible, state, sigmas, 4, 42)
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	// Both pods together cover all 8 decks.
	seen := make(map[int]bool)
	for _, pod := range pods {
		for _, d := range pod {
			seen[d] = true
		}
	}
	if len(seen) != 8 {
		t.Errorf("expected all 8 decks covered across pods; saw %d", len(seen))
	}
}

// TestPairDoubleElim_GroupsBySigmaWithinTier is the headline test:
// within a single losses-tier (all decks tied on losses + pods),
// high-σ decks must pod up together and low-σ decks must pod up
// together. Pre-fix the shuffle would interleave them.
func TestPairDoubleElim_GroupsBySigmaWithinTier(t *testing.T) {
	state := allLosses(8)
	eligible := []int{0, 1, 2, 3, 4, 5, 6, 7}
	// 4 high-σ decks (0..3) and 4 low-σ decks (4..7). σ gap is
	// well above sigmaPairingEpsilon = 0.05.
	sigmas := []float64{
		7.0, 7.0, 7.0, 7.0, // high-σ cluster
		2.0, 2.0, 2.0, 2.0, // low-σ cluster
	}
	pods := pairDoubleElimRound(eligible, state, sigmas, 4, 42)
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}

	// One pod must be all high-σ (decks 0-3), the other all low-σ
	// (decks 4-7). The pod ORDER doesn't matter (pod[0] could be
	// either cluster depending on σ desc ordering), but each pod
	// must be cluster-pure.
	classify := func(pod []int) (high, low int) {
		for _, d := range pod {
			if sigmas[d] >= 5.0 {
				high++
			} else {
				low++
			}
		}
		return
	}
	for i, pod := range pods {
		h, l := classify(pod)
		if h != 0 && l != 0 {
			t.Errorf("pod %d is mixed (h=%d l=%d); want σ-cluster-pure: %v", i, h, l, pod)
		}
	}
}

// TestPairDoubleElim_LossesTrumpSigma pins that the σ layer is
// SUBORDINATE to losses — a high-σ deck in the losers tier (1 loss)
// must still pair with other losers-tier decks, NOT the high-σ
// winners-tier decks.
func TestPairDoubleElim_LossesTrumpSigma(t *testing.T) {
	state := []deckState{
		{losses: 0}, {losses: 0}, {losses: 0}, {losses: 0}, // winners
		{losses: 1}, {losses: 1}, {losses: 1}, {losses: 1}, // losers
	}
	eligible := []int{0, 1, 2, 3, 4, 5, 6, 7}
	// Crucially, the σ profile is INVERTED from the losses tier:
	// winners (low σ — well-known strong) have small σ, losers
	// (high σ — uncertain) have large σ. If σ trumped losses, deck
	// 4 (high σ) would pod up with deck 5 (high σ) — but that's
	// what we WANT here anyway. The test specifically checks the
	// winners tier doesn't mix with losers tier despite σ overlap.
	sigmas := []float64{
		3.0, 3.0, 3.0, 3.0, // winners tier σ
		8.0, 8.0, 8.0, 8.0, // losers tier σ
	}
	pods := pairDoubleElimRound(eligible, state, sigmas, 4, 42)
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	// First pod (winners tier) must be all 0-losses.
	for _, d := range pods[0] {
		if state[d].losses != 0 {
			t.Errorf("first pod should be 0-losses; deck %d has %d losses", d, state[d].losses)
		}
	}
	// Second pod (losers tier) must be all 1-losses.
	for _, d := range pods[1] {
		if state[d].losses != 1 {
			t.Errorf("second pod should be 1-losses; deck %d has %d losses", d, state[d].losses)
		}
	}
}

// TestPairDoubleElim_SubEpsilonSigmaDeferToShuffle pins that
// near-identical σ values (within sigmaPairingEpsilon) fall
// through to the shuffle layer — preventing degenerate fixed-order
// pairing when ratings happen to be very close.
func TestPairDoubleElim_SubEpsilonSigmaDeferToShuffle(t *testing.T) {
	state := allLosses(4)
	eligible := []int{0, 1, 2, 3}
	// All within sigmaPairingEpsilon of each other.
	sigmas := []float64{8.30, 8.31, 8.32, 8.33}

	// Two different seeds should pick different orderings if the
	// shuffle tiebreak is in play. If σ dominates we'd get the same
	// order both times (deck 3, 2, 1, 0 — σ desc).
	podsA := pairDoubleElimRound(eligible, state, sigmas, 4, 1)
	podsB := pairDoubleElimRound(eligible, state, sigmas, 4, 999)
	if len(podsA) != 1 || len(podsB) != 1 {
		t.Fatalf("expected 1 pod per call; got A=%d B=%d", len(podsA), len(podsB))
	}
	// At least one position must differ between the two seeds.
	diff := false
	for i := 0; i < 4; i++ {
		if podsA[0][i] != podsB[0][i] {
			diff = true
			break
		}
	}
	if !diff {
		t.Errorf("sub-epsilon σ should defer to shuffle (per-seed variance); both seeds produced identical pod: %v", podsA[0])
	}
}

// TestPairDoubleElim_HighSigmaFirst pins that within a tier, the
// HIGHER-σ deck sorts EARLIER — so the highest-uncertainty decks
// land in the first pod. The principle is "extract info where it's
// scarcest first."
func TestPairDoubleElim_HighSigmaFirst(t *testing.T) {
	state := allLosses(8)
	eligible := []int{0, 1, 2, 3, 4, 5, 6, 7}
	// Strictly monotone σ; first pod should contain the four
	// highest-σ decks (4, 5, 6, 7).
	sigmas := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0}

	pods := pairDoubleElimRound(eligible, state, sigmas, 4, 42)
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	// First pod should be the high-σ four.
	highSet := map[int]bool{4: true, 5: true, 6: true, 7: true}
	for _, d := range pods[0] {
		if !highSet[d] {
			t.Errorf("first pod should be the high-σ cluster {4,5,6,7}; got deck %d (σ=%.1f). Full first pod: %v",
				d, sigmas[d], pods[0])
		}
	}
}

// TestPairDoubleElim_NilSigmasSafeDefault pins that an empty σ
// slice (defensive — should not happen via RunDoubleElimination,
// but unit-callable) doesn't crash and falls through to shuffle.
func TestPairDoubleElim_NilSigmasSafeDefault(t *testing.T) {
	state := allLosses(4)
	eligible := []int{0, 1, 2, 3}
	pods := pairDoubleElimRound(eligible, state, nil, 4, 42)
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod (nil sigmas should not crash), got %d", len(pods))
	}
}

// TestRunDoubleElimination_SigmaSnapshotPasses confirms the
// end-to-end wiring: a real DE tournament completes without crash
// and the ts snapshot is non-trivial (σ diverges from the default).
func TestRunDoubleElimination_SigmaSnapshotPasses(t *testing.T) {
	decks := loadDecksOrSkip(t, 4)
	r, err := RunDoubleElimination(DoubleEliminationConfig{
		Decks:         decks,
		NSeats:        4,
		GamesPerPod:   1,
		MaxRounds:     4,
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
	})
	if err != nil {
		t.Fatalf("RunDoubleElimination: %v", err)
	}
	if len(r.TrueSkill) == 0 {
		t.Fatal("TrueSkill snapshot empty")
	}
	// At least one deck's σ must have moved from default 8.333.
	anyMoved := false
	for _, e := range r.TrueSkill {
		if e.Sigma < 8.0 || e.Sigma > 8.6 {
			anyMoved = true
			break
		}
	}
	if !anyMoved {
		t.Errorf("expected at least one deck's σ to diverge from default ~8.33; got %+v", r.TrueSkill)
	}
}
