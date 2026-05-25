package trueskill

import (
	"math"
	"testing"
)

func TestDecay_WithinGracePeriodNoChange(t *testing.T) {
	cfg := DefaultDecayConfig()
	r := Rating{Mu: 35.0, Sigma: 3.0}
	for _, days := range []int{0, 1, 7, 13, 14} {
		out := r.Decayed(days, cfg)
		if out != r {
			t.Errorf("decay within grace (%d days): got %+v, want unchanged %+v", days, out, r)
		}
	}
}

func TestDecay_AtOneHalfLifeMuMovesHalfwayToBase(t *testing.T) {
	cfg := DefaultDecayConfig() // grace=14, halflife=90
	r := Rating{Mu: 35.0, Sigma: 3.0}
	// 14 + 90 = 104 days inactive → exactly one half-life past grace.
	out := r.Decayed(104, cfg)

	// μ' = 25 + (35 − 25) * 0.5 = 30.
	if math.Abs(out.Mu-30.0) > 1e-9 {
		t.Errorf("one half-life past grace: μ=%.6f, want 30.0", out.Mu)
	}
}

func TestDecay_AtTwoHalfLivesMuMovesThreeQuartersToBase(t *testing.T) {
	cfg := DefaultDecayConfig()
	r := Rating{Mu: 35.0, Sigma: 3.0}
	// 14 + 180 = 194 days → two half-lives past grace.
	out := r.Decayed(194, cfg)

	// μ' = 25 + 10 * 0.25 = 27.5.
	if math.Abs(out.Mu-27.5) > 1e-9 {
		t.Errorf("two half-lives past grace: μ=%.6f, want 27.5", out.Mu)
	}
}

func TestDecay_BelowBaseDriftsUpToBase(t *testing.T) {
	// Symmetric case: a player at μ=15 (10 below base) should drift UP
	// toward 25 with the same half-life.
	cfg := DefaultDecayConfig()
	r := Rating{Mu: 15.0, Sigma: 3.0}
	out := r.Decayed(104, cfg) // one half-life

	// μ' = 25 + (15 − 25) * 0.5 = 20.
	if math.Abs(out.Mu-20.0) > 1e-9 {
		t.Errorf("below-base one half-life: μ=%.6f, want 20.0 (drift UP toward base)", out.Mu)
	}
}

func TestDecay_AtBaseStaysAtBase(t *testing.T) {
	cfg := DefaultDecayConfig()
	r := Rating{Mu: 25.0, Sigma: 3.0}
	out := r.Decayed(365, cfg) // a year of inactivity
	if math.Abs(out.Mu-25.0) > 1e-9 {
		t.Errorf("at-base rating after 365 days: μ=%.6f, want 25.0 (no drift when already at base)", out.Mu)
	}
}

func TestDecay_SigmaInflates(t *testing.T) {
	cfg := DefaultDecayConfig() // sigma rate = 0.05/day
	r := Rating{Mu: 30.0, Sigma: 3.0}
	out := r.Decayed(64, cfg) // 50 days past grace
	// σ' = 3.0 + 50 * 0.05 = 5.5
	if math.Abs(out.Sigma-5.5) > 1e-9 {
		t.Errorf("σ after 50 days past grace: got %.6f, want 5.5", out.Sigma)
	}
}

func TestDecay_SigmaCappedAtMaxSigma(t *testing.T) {
	cfg := DefaultDecayConfig()
	r := Rating{Mu: 30.0, Sigma: 6.0}
	// 14 + 365 = 379 days past grace at 0.05/day = +18.25 raw → would
	// exceed MaxSigma (≈8.33). Verify capping.
	out := r.Decayed(379, cfg)
	if out.Sigma > cfg.MaxSigma+1e-9 {
		t.Errorf("σ should be capped at MaxSigma (%.4f), got %.4f", cfg.MaxSigma, out.Sigma)
	}
	if math.Abs(out.Sigma-cfg.MaxSigma) > 1e-9 {
		t.Errorf("σ at capping limit: got %.6f, want %.6f", out.Sigma, cfg.MaxSigma)
	}
}

func TestDecay_NeverDecreasesSigma(t *testing.T) {
	// Defensive: a malformed config (e.g. negative inflation) must not
	// shrink σ — decay represents EROSION of information, never gain.
	cfg := DefaultDecayConfig()
	cfg.SigmaInflationPerDay = -1.0 // pathological
	r := Rating{Mu: 30.0, Sigma: 4.0}
	out := r.Decayed(100, cfg)
	if out.Sigma < r.Sigma {
		t.Errorf("decay must never decrease σ: input %.4f, output %.4f", r.Sigma, out.Sigma)
	}
}

func TestDecay_ZeroHalfLifeSkipsMuDrift(t *testing.T) {
	// Caller wants σ-only decay (a common preference): set HalfLifeDays=0
	// and μ stays put while σ still inflates.
	cfg := DefaultDecayConfig()
	cfg.HalfLifeDays = 0
	r := Rating{Mu: 40.0, Sigma: 3.0}
	out := r.Decayed(100, cfg)
	if out.Mu != 40.0 {
		t.Errorf("HalfLifeDays=0 should freeze μ: got %.4f, want 40.0", out.Mu)
	}
	if out.Sigma <= r.Sigma {
		t.Errorf("σ should still inflate even with HalfLifeDays=0: input %.4f, output %.4f",
			r.Sigma, out.Sigma)
	}
}

func TestDecay_PureFunctionDoesNotMutateInput(t *testing.T) {
	cfg := DefaultDecayConfig()
	r := Rating{Mu: 35.0, Sigma: 3.0}
	original := r
	_ = r.Decayed(200, cfg)
	if r != original {
		t.Errorf("Decayed mutated receiver: before %+v, after %+v", original, r)
	}
}

// --- ApplyDecay (bulk) ---------------------------------------------------

func TestApplyDecay_OnlyAffectsListedPlayers(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"alice", "bob", "carol"})
	ts.Ratings["alice"] = Rating{Mu: 35.0, Sigma: 3.0}
	ts.Ratings["bob"] = Rating{Mu: 40.0, Sigma: 3.0}
	ts.Ratings["carol"] = Rating{Mu: 20.0, Sigma: 3.0}

	bobBefore := ts.Ratings["bob"]
	carolBefore := ts.Ratings["carol"]

	// Decay only alice; bob and carol untouched.
	ts.ApplyDecay(map[string]int{"alice": 104}, DefaultDecayConfig())

	aliceAfter := ts.Ratings["alice"]
	if math.Abs(aliceAfter.Mu-30.0) > 1e-9 {
		t.Errorf("alice μ after one-halflife decay: got %.4f, want 30.0", aliceAfter.Mu)
	}
	if ts.Ratings["bob"] != bobBefore {
		t.Errorf("bob unchanged invariant: before %+v, after %+v", bobBefore, ts.Ratings["bob"])
	}
	if ts.Ratings["carol"] != carolBefore {
		t.Errorf("carol unchanged invariant: before %+v, after %+v", carolBefore, ts.Ratings["carol"])
	}
}

func TestApplyDecay_UnknownPlayerIsNoop(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"alice"})
	ts.Ratings["alice"] = Rating{Mu: 35.0, Sigma: 3.0}
	before := ts.Ratings["alice"]

	// Map references a player not in the tracker — should be a no-op,
	// no panic, no spurious entry.
	ts.ApplyDecay(map[string]int{"ghost": 365}, DefaultDecayConfig())

	if ts.Ratings["alice"] != before {
		t.Errorf("alice changed by ghost-only decay: before %+v, after %+v",
			before, ts.Ratings["alice"])
	}
	if _, ok := ts.Ratings["ghost"]; ok {
		t.Errorf("ApplyDecay must not create entries for unknown players")
	}
}

func TestApplyDecay_MultiPlayerHonorsPerPlayerDays(t *testing.T) {
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["fresh"] = Rating{Mu: 35.0, Sigma: 3.0}  // 5 days off
	ts.Ratings["lapsed"] = Rating{Mu: 35.0, Sigma: 3.0} // 1 year off
	cfg := DefaultDecayConfig()
	ts.ApplyDecay(map[string]int{"fresh": 5, "lapsed": 365}, cfg)

	if ts.Ratings["fresh"].Mu != 35.0 {
		t.Errorf("fresh (within grace) should not decay: got μ=%.4f, want 35.0",
			ts.Ratings["fresh"].Mu)
	}
	if ts.Ratings["lapsed"].Mu >= 35.0 {
		t.Errorf("lapsed (year off) must drift toward base: got μ=%.4f, want < 35.0",
			ts.Ratings["lapsed"].Mu)
	}
}
