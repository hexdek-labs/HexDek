package trueskill

import (
	"math"
	"testing"
)

// convergence_r60_test.go — sigma asymptotic-convergence audit.
//
// Existing tests verify σ shrinks after 1 game (TestRatingChangesAfterUpdate),
// hits < 0.6·σ₀ after 200 games (TestFFAConvergence_4PlayerDeterministicSkill),
// and stays above τ/2 after 1000 games (TestTauPreservesInformationOverTime).
//
// What none of them pinned: the ASYMPTOTE shape. The TrueSkill literature
// predicts σ converges to a τ-driven equilibrium where per-update inflation
// (√(σ² + τ²)) exactly offsets the truncated-Gaussian shrinkage. Probing the
// trajectory shows σ reaches a minimum around game 500 (~0.9 for the
// FFA preset's τ=σ/200) then *slightly inflates* back up by game 2000 as
// the τ floor takes over. Without that mechanism σ would decay
// monotonically toward the 1e-6 safety clamp — a deck rating with σ ≈ 0
// is a confident lie if the underlying skill has drifted at all.
//
// These tests pin the equilibrium and prove τ is the load-bearing
// mechanism via the τ=0 counterfactual.

// TestSigmaPlateausAfterConvergence — σ between game 500 and game 2000
// must stay within a tight band. The τ-driven equilibrium prevents
// continued decay; if a future change to τ or the update math broke
// the inflation step, this would fire (σ would keep shrinking).
func TestSigmaPlateausAfterConvergence(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	for i := 0; i < 500; i++ {
		ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})
	}
	sigmaAt500 := map[string]float64{
		"a": ts.Ratings["a"].Sigma, "b": ts.Ratings["b"].Sigma,
		"c": ts.Ratings["c"].Sigma, "d": ts.Ratings["d"].Sigma,
	}
	for i := 0; i < 1500; i++ {
		ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})
	}
	for name, prior := range sigmaAt500 {
		now := ts.Ratings[name].Sigma
		// Allow ±20% drift around the game-500 value. Empirically the σ
		// plateau oscillates ~5-10% (probe shows a moves 1.27 → 1.32,
		// b moves 0.90 → 1.07). 20% gives headroom for FP variation
		// without admitting a true "continued decay" regression.
		ratio := now / prior
		if ratio < 0.8 || ratio > 1.25 {
			t.Errorf("%s σ did not plateau: game-500 σ=%f, game-2000 σ=%f (ratio %f outside [0.8, 1.25])",
				name, prior, now, ratio)
		}
	}
}

// TestTauZeroCounterfactual_SigmaMonotonicallyDecays — τ=0 disables the
// per-update inflation step. Without it σ should decay monotonically
// past the τ-driven equilibrium toward the 1e-6 safety clamp. Pins
// that τ is the load-bearing mechanism: if a future refactor accidentally
// zeroed τ, the plateau test above would fire, but the FAILURE MODE
// being prevented is the one this test demonstrates.
func TestTauZeroCounterfactual_SigmaMonotonicallyDecays(t *testing.T) {
	cfg := DefaultFFAConfig()
	cfg.Tau = 0
	ts := NewTrueSkillRatingsWithConfig([]string{"a", "b", "c", "d"}, cfg)

	var sigma200, sigma1000, sigma2000 float64
	for game := 1; game <= 2000; game++ {
		ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})
		switch game {
		case 200:
			sigma200 = ts.Ratings["a"].Sigma
		case 1000:
			sigma1000 = ts.Ratings["a"].Sigma
		case 2000:
			sigma2000 = ts.Ratings["a"].Sigma
		}
	}
	if !(sigma1000 < sigma200) {
		t.Errorf("with τ=0, σ should monotonically decay: σ@200=%f, σ@1000=%f (should be strictly less)",
			sigma200, sigma1000)
	}
	if !(sigma2000 < sigma1000) {
		t.Errorf("with τ=0, σ should keep decaying: σ@1000=%f, σ@2000=%f (should be strictly less)",
			sigma1000, sigma2000)
	}
	// At τ=σ/200 the equilibrium σ is ~1.0+; at τ=0 it should have
	// already decayed past that by game 2000.
	if sigma2000 >= 1.0 {
		t.Errorf("τ=0 σ@2000 should be < 1.0 (proving the FFA preset's τ floor is what holds σ above 1): got %f",
			sigma2000)
	}
}

// TestSigmaEquilibriumScalesWithTau — higher τ produces a higher
// equilibrium σ. Two configs run 2000 games each; the one with 4× τ
// must plateau at a meaningfully higher σ. Pins that τ is the
// PARAMETER that controls equilibrium height, not just a safety floor.
func TestSigmaEquilibriumScalesWithTau(t *testing.T) {
	run := func(tau float64) float64 {
		cfg := DefaultFFAConfig()
		cfg.Tau = tau
		ts := NewTrueSkillRatingsWithConfig([]string{"a", "b", "c", "d"}, cfg)
		for i := 0; i < 2000; i++ {
			ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})
		}
		// Average the bottom two ranks (they converge tightest under
		// deterministic ordering) for a stable equilibrium readout.
		return (ts.Ratings["b"].Sigma + ts.Ratings["c"].Sigma) / 2
	}
	sigmaLowTau := run(defaultSigma / 200.0) // FFA default
	sigmaHighTau := run(defaultSigma / 50.0) // 4× higher τ
	if sigmaHighTau <= sigmaLowTau {
		t.Errorf("higher τ should yield higher equilibrium σ: σ(τ=σ/200)=%f, σ(τ=σ/50)=%f",
			sigmaLowTau, sigmaHighTau)
	}
	// The relationship should be roughly proportional. With τ scaling
	// 4× we'd expect equilibrium σ to grow noticeably (not strict 4×
	// because of the √(σ²+τ²) interaction with the shrinkage term).
	// Require at least 40% growth.
	if sigmaHighTau < sigmaLowTau*1.4 {
		t.Errorf("4× τ should yield ≥1.4× equilibrium σ: low=%f high=%f ratio=%f",
			sigmaLowTau, sigmaHighTau, sigmaHighTau/sigmaLowTau)
	}
}

// TestSigmaNeverGrowsAboveDefaultDuringConvergence — σ should never
// transiently inflate above σ₀ during normal updates. (InheritRating's
// inflation is a separate, intentional path; this checks the Update
// hot loop.) If an update accidentally added τ² instead of √(σ²+τ²)
// somewhere, σ could runaway-grow.
func TestSigmaNeverGrowsAboveDefaultDuringConvergence(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	maxSeen := 0.0
	for i := 0; i < 2000; i++ {
		ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})
		for _, r := range ts.Ratings {
			if r.Sigma > maxSeen {
				maxSeen = r.Sigma
			}
		}
	}
	if maxSeen > defaultSigma {
		t.Errorf("σ exceeded σ₀ during update loop: max=%f σ₀=%f — runaway inflation",
			maxSeen, defaultSigma)
	}
}

// TestSigmaShrinkageRateIsMonotonic_EarlyGames — across the first 50
// games (where τ inflation is negligible vs the steep early shrinkage),
// σ should monotonically decrease at every game tick. A single
// non-monotonic step early on would signal a sign error in the update.
func TestSigmaShrinkageRateIsMonotonic_EarlyGames(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	prev := math.Inf(1)
	for game := 1; game <= 50; game++ {
		ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})
		// Use the middle-rank player whose updates are cleanest
		// (extreme ranks see asymmetric pairwise dynamics).
		now := ts.Ratings["b"].Sigma
		if now >= prev {
			t.Errorf("σ non-monotonic in early-convergence regime at game %d: prev=%f now=%f",
				game, prev, now)
		}
		prev = now
	}
}
