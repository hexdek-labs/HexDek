package trueskill

import (
	"math"
	"testing"
)

// r60 TrueSkill init-tuning regressions. These pin the specific new
// values + behaviors introduced by the r60 retune of DefaultFFAConfig:
//
//   τ = σ/200 (was σ/100) — static-deck self-play, less drift than
//                            human-skill action games
//   dP = 0.005 (was 0.02) — observed 4-player Commander draw rate
//
// Documented in docs/trueskill-tuning-r60.md.

// TestFFATau_ExactValue pins the exact tau value (σ/200) so re-tunes
// require explicit re-evaluation rather than silent drift.
func TestFFATau_ExactValue(t *testing.T) {
	ffa := DefaultFFAConfig()
	want := defaultSigma / 200.0
	if math.Abs(ffa.Tau-want) > 1e-12 {
		t.Errorf("FFA Tau = %f, want σ/200 = %f", ffa.Tau, want)
	}
	// Sanity: numerical value rounds to ~0.0417.
	if ffa.Tau < 0.040 || ffa.Tau > 0.045 {
		t.Errorf("FFA Tau looks off the expected band [0.040, 0.045]: %f", ffa.Tau)
	}
}

// TestFFADrawProbability_ExactValue pins the exact draw probability
// (0.005) and confirms it's strictly between 0 and the 1v1 baseline.
func TestFFADrawProbability_ExactValue(t *testing.T) {
	ffa := DefaultFFAConfig()
	if math.Abs(ffa.DrawProbability-0.005) > 1e-12 {
		t.Errorf("FFA DrawProbability = %f, want 0.005", ffa.DrawProbability)
	}
	base := DefaultConfig()
	if !(ffa.DrawProbability > 0 && ffa.DrawProbability < base.DrawProbability) {
		t.Errorf("FFA DrawProbability should be (0, %f); got %f", base.DrawProbability, ffa.DrawProbability)
	}
}

// TestFFA1v1_BetaWidensTauShrinks_DrawShrinks documents the full
// retune signature in one place: relative to the 1v1 reference
// baseline, FFA β is wider, τ is narrower, dP is narrower. If any
// future change accidentally flips one of these relationships (e.g.
// pulls dP back to 0.02 for a calibration test), this test surfaces
// it as a single explicit assertion.
func TestFFA1v1_BetaWidensTauShrinks_DrawShrinks(t *testing.T) {
	base := DefaultConfig()
	ffa := DefaultFFAConfig()

	if !(ffa.Beta > base.Beta) {
		t.Errorf("β must widen 1v1→FFA: base=%f ffa=%f", base.Beta, ffa.Beta)
	}
	if !(ffa.Tau < base.Tau) {
		t.Errorf("τ must shrink 1v1→FFA: base=%f ffa=%f", base.Tau, ffa.Tau)
	}
	if !(ffa.DrawProbability < base.DrawProbability) {
		t.Errorf("dP must shrink 1v1→FFA: base=%f ffa=%f", base.DrawProbability, ffa.DrawProbability)
	}
}

// TestFFATau_ConvergesFasterThanReferenceSlowDrift confirms the
// operational consequence of the tau retune: across 500 self-play
// games on a static deck-strength scenario, the FFA preset
// converges to a tighter σ than a hypothetical "FFA-with-1v1-tau"
// preset. Pins that the τ reduction does what we built it for —
// faster convergence in the static-deck context — without depending
// on the β contribution (we hold β constant via DeepCopy of the FFA
// config).
func TestFFATau_ConvergesFasterThanReferenceSlowDrift(t *testing.T) {
	// Strong > weak1 > weak2 > weak3 every game.
	rankSequence := []int{0, 1, 2, 3}
	names := []string{"strong", "weak1", "weak2", "weak3"}

	runTo := func(cfg Config, games int) float64 {
		ts := NewTrueSkillRatingsWithConfig(names, cfg)
		for i := 0; i < games; i++ {
			ts.Update(names, rankSequence)
		}
		return ts.Ratings["strong"].Sigma
	}

	// FFA preset (β widened + τ shrunk + dP shrunk)
	ffa := DefaultFFAConfig()
	sigmaFFA := runTo(ffa, 500)

	// Reference: same β + dP as FFA but τ at the slow-drift 1v1 value.
	slow := ffa
	slow.Tau = DefaultConfig().Tau
	sigmaSlow := runTo(slow, 500)

	if sigmaFFA >= sigmaSlow {
		t.Errorf("FFA τ (σ/200) should converge to tighter σ than reference 1v1 τ (σ/100) on the same β/dP: ffa=%f slow=%f",
			sigmaFFA, sigmaSlow)
	}
}

// TestFFADrawProbability_ShrinksUpdatesNearEvenMatchups confirms the
// operational consequence of the drawP retune. The TrueSkill epsilon
// (drawMargin(p, β)) is the half-width of the "draw band" in
// standardized-skill space. A LARGER drawProbability means a wider
// band — a win that lands across the band is more surprising, so it
// produces a larger update. A SMALLER drawProbability tightens the
// band — wins are less surprising and updates are more conservative.
//
// For HexDek's near-zero-draw 4-player Commander context, this
// conservatism is appropriate: most wins are not flukes-that-could-
// have-been-draws, so each one should move the rating less. The
// rating becomes more about cumulative performance than swinging on
// a single dramatic upset.
//
// Pin the direction: with the new FFA dP (0.005) the same crisp-win
// scenario produces a STRICTLY SMALLER winner-mu gain than with the
// 1v1 dP (0.02).
func TestFFADrawProbability_ShrinksUpdatesNearEvenMatchups(t *testing.T) {
	runner := func(dP float64) float64 {
		cfg := DefaultFFAConfig()
		cfg.DrawProbability = dP
		w, l := DefaultRating(), DefaultRating()
		wNew, _ := Update2Player(cfg, w, l)
		return wNew.Mu - w.Mu
	}

	gainSmall := runner(0.005) // r60 FFA
	gainLarge := runner(0.02)  // 1v1 baseline

	if gainSmall >= gainLarge {
		t.Errorf("smaller drawP should shrink the update size (tighter draw band = less surprising win): small=%f large=%f",
			gainSmall, gainLarge)
	}
	// Sanity: gap is non-trivial (> 0.005 mu units). Catches a
	// regression where someone accidentally aligns dP back to 1v1.
	if gainLarge-gainSmall < 0.005 {
		t.Errorf("update-size gap looks suspiciously thin: small=%f large=%f (delta %f)",
			gainSmall, gainLarge, gainLarge-gainSmall)
	}
}
