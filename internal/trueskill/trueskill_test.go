package trueskill

import (
	"math"
	"testing"
)

func TestDefaultRating(t *testing.T) {
	r := DefaultRating()
	if r.Mu != 25.0 {
		t.Errorf("mu = %f, want 25.0", r.Mu)
	}
	if math.Abs(r.Sigma-25.0/3.0) > 1e-9 {
		t.Errorf("sigma = %f, want %f", r.Sigma, 25.0/3.0)
	}
}

func TestConservative(t *testing.T) {
	r := Rating{Mu: 25.0, Sigma: 8.333}
	c := r.Conservative()
	if math.Abs(c-0.001) > 0.01 {
		t.Errorf("conservative = %f, want ~0", c)
	}
}

func TestUpdate2Player_WinnerGains(t *testing.T) {
	cfg := DefaultConfig()
	w := DefaultRating()
	l := DefaultRating()

	wNew, lNew := Update2Player(cfg, w, l)

	if wNew.Mu <= w.Mu {
		t.Errorf("winner mu should increase: %f -> %f", w.Mu, wNew.Mu)
	}
	if lNew.Mu >= l.Mu {
		t.Errorf("loser mu should decrease: %f -> %f", l.Mu, lNew.Mu)
	}
	if wNew.Sigma >= w.Sigma {
		t.Errorf("winner sigma should decrease: %f -> %f", w.Sigma, wNew.Sigma)
	}
	if lNew.Sigma >= l.Sigma {
		t.Errorf("loser sigma should decrease: %f -> %f", l.Sigma, lNew.Sigma)
	}
}

func TestUpdate2Player_Symmetry(t *testing.T) {
	cfg := DefaultConfig()
	a := DefaultRating()
	b := DefaultRating()

	aNew, bNew := Update2Player(cfg, a, b)

	muGain := aNew.Mu - a.Mu
	muLoss := b.Mu - bNew.Mu
	if math.Abs(muGain-muLoss) > 1e-6 {
		t.Errorf("equal-rated update should be symmetric: gain=%f loss=%f", muGain, muLoss)
	}
}

func TestUpdate2Player_StrongerWinnerGainsLess(t *testing.T) {
	cfg := DefaultConfig()
	strong := Rating{Mu: 35.0, Sigma: 5.0}
	weak := Rating{Mu: 15.0, Sigma: 5.0}

	sNew, _ := Update2Player(cfg, strong, weak)
	strongGain := sNew.Mu - strong.Mu

	equal := Rating{Mu: 25.0, Sigma: 5.0}
	eNew, _ := Update2Player(cfg, equal, equal)
	equalGain := eNew.Mu - equal.Mu

	if strongGain >= equalGain {
		t.Errorf("strong beating weak should gain less (%f) than equal beating equal (%f)",
			strongGain, equalGain)
	}
}

func TestUpdateMultiplayer_4Player(t *testing.T) {
	cfg := DefaultConfig()
	ratings := []Rating{
		DefaultRating(),
		DefaultRating(),
		DefaultRating(),
		DefaultRating(),
	}
	ranks := []int{0, 1, 2, 3} // player 0 won, player 3 last

	updated := UpdateMultiplayer(cfg, ratings, ranks)

	if updated[0].Mu <= ratings[0].Mu {
		t.Errorf("1st place mu should increase: %f -> %f", ratings[0].Mu, updated[0].Mu)
	}
	if updated[3].Mu >= ratings[3].Mu {
		t.Errorf("4th place mu should decrease: %f -> %f", ratings[3].Mu, updated[3].Mu)
	}
	if updated[0].Mu <= updated[1].Mu {
		t.Errorf("1st place should rate higher than 2nd: %f vs %f", updated[0].Mu, updated[1].Mu)
	}
	if updated[2].Mu <= updated[3].Mu {
		t.Errorf("3rd place should rate higher than 4th: %f vs %f", updated[2].Mu, updated[3].Mu)
	}
}

func TestUpdateMultiplayer_RanksOutOfOrder(t *testing.T) {
	cfg := DefaultConfig()
	ratings := []Rating{
		DefaultRating(),
		DefaultRating(),
		DefaultRating(),
		DefaultRating(),
	}
	// Player 2 won, player 0 last
	ranks := []int{3, 1, 0, 2}

	updated := UpdateMultiplayer(cfg, ratings, ranks)

	if updated[2].Mu <= updated[0].Mu {
		t.Errorf("winner (idx 2) should rate higher than last (idx 0): %f vs %f",
			updated[2].Mu, updated[0].Mu)
	}
}

func TestConvergence_80PercentWinner(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"strong", "weak1", "weak2", "weak3"})

	for i := 0; i < 200; i++ {
		if i%5 == 4 {
			// 20% of the time, weak1 wins (upset)
			ts.Update([]string{"strong", "weak1", "weak2", "weak3"}, []int{1, 0, 2, 3})
		} else {
			ts.Update([]string{"strong", "weak1", "weak2", "weak3"}, []int{0, 1, 2, 3})
		}
	}

	if ts.Ratings["strong"].Mu <= ts.Ratings["weak1"].Mu {
		t.Errorf("strong should rate higher than weak1 after 200 games: %f vs %f",
			ts.Ratings["strong"].Mu, ts.Ratings["weak1"].Mu)
	}
	if ts.Ratings["strong"].Sigma >= DefaultRating().Sigma {
		t.Errorf("sigma should decrease with games: %f vs %f",
			ts.Ratings["strong"].Sigma, DefaultRating().Sigma)
	}
}

func TestSigmaNeverZero(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})

	for i := 0; i < 1000; i++ {
		ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})
	}

	for name, r := range ts.Ratings {
		if r.Sigma <= 0 {
			t.Errorf("%s sigma should be positive: %f", name, r.Sigma)
		}
		if r.Sigma < 0.01 {
			t.Errorf("%s sigma suspiciously small: %f", name, r.Sigma)
		}
	}
}

func TestInheritRating(t *testing.T) {
	parent := Rating{Mu: 30.0, Sigma: 3.0}

	child := InheritRating(parent, 5)
	if child.Mu != parent.Mu {
		t.Errorf("child mu should equal parent: %f vs %f", child.Mu, parent.Mu)
	}
	if child.Sigma <= parent.Sigma {
		t.Errorf("child sigma should be inflated: %f vs %f", child.Sigma, parent.Sigma)
	}

	zeroChange := InheritRating(parent, 0)
	if zeroChange.Sigma != parent.Sigma {
		t.Errorf("0 card delta should not inflate sigma: %f vs %f", zeroChange.Sigma, parent.Sigma)
	}

	bigChange := InheritRating(parent, 100)
	if bigChange.Sigma > defaultSigma {
		t.Errorf("sigma should cap at default: %f vs %f", bigChange.Sigma, defaultSigma)
	}
}

func TestTrueSkillRatings_Snapshot(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c"})
	ts.Update([]string{"a", "b", "c"}, []int{0, 1, 2})
	ts.Update([]string{"a", "b", "c"}, []int{0, 1, 2})

	snap := ts.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("snapshot length = %d, want 3", len(snap))
	}
	if snap[0].Commander != "a" {
		t.Errorf("top rated should be 'a', got %q", snap[0].Commander)
	}
	if snap[0].Games != 2 {
		t.Errorf("games should be 2, got %d", snap[0].Games)
	}
	for i := 0; i < len(snap)-1; i++ {
		if snap[i].Conservative < snap[i+1].Conservative {
			t.Errorf("snapshot not sorted: %f < %f", snap[i].Conservative, snap[i+1].Conservative)
		}
	}
}

func TestNormPDF(t *testing.T) {
	if math.Abs(normPDF(0)-0.3989422804) > 1e-6 {
		t.Errorf("normPDF(0) = %f", normPDF(0))
	}
}

func TestNormCDF(t *testing.T) {
	if math.Abs(normCDF(0)-0.5) > 1e-9 {
		t.Errorf("normCDF(0) = %f", normCDF(0))
	}
	if normCDF(-5) > 1e-5 {
		t.Errorf("normCDF(-5) should be near 0: %f", normCDF(-5))
	}
	if normCDF(5) < 1-1e-5 {
		t.Errorf("normCDF(5) should be near 1: %f", normCDF(5))
	}
}

// TestDefaultConfig_LiteratureDefaults pins the Microsoft 1v1 reference
// values for DefaultConfig: β = σ/2, τ = σ/100, drawP = 2%. If anyone
// later "tunes" DefaultConfig (which serves head-to-head consumers), this
// test catches it and forces them to read the FFA-vs-1v1 commentary.
func TestDefaultConfig_LiteratureDefaults(t *testing.T) {
	cfg := DefaultConfig()
	wantBeta := defaultSigma / 2.0
	wantTau := defaultSigma / 100.0
	if math.Abs(cfg.Beta-wantBeta) > 1e-9 {
		t.Errorf("DefaultConfig.Beta = %f, want σ/2 = %f", cfg.Beta, wantBeta)
	}
	if math.Abs(cfg.Tau-wantTau) > 1e-9 {
		t.Errorf("DefaultConfig.Tau = %f, want σ/100 = %f", cfg.Tau, wantTau)
	}
	if math.Abs(cfg.DrawProbability-0.02) > 1e-9 {
		t.Errorf("DefaultConfig.DrawProbability = %f, want 0.02", cfg.DrawProbability)
	}
}

// TestDefaultFFAConfig_DivergenceFromBaseline pins all three of the
// FFA preset's intentional divergences from the Microsoft 1v1 literature
// defaults. Documented in `docs/trueskill-tuning-r60.md`.
//
//	β  widens  σ/2   → σ·0.6  — Commander pod noise (politics, mana, position)
//	τ  shrinks σ/100 → σ/200  — static-deck self-play needs less drift than action games
//	dP shrinks 0.02  → 0.005  — observed 4p Commander draw rate is ~0.01%
//
// If you find yourself re-tuning any of these, update this test, the
// matching ffa*Scale / ffaDrawProbability constant comment, AND the
// rationale doc.
func TestDefaultFFAConfig_DivergenceFromBaseline(t *testing.T) {
	base := DefaultConfig()
	ffa := DefaultFFAConfig()

	wantBeta := defaultSigma * ffaBetaScale
	if math.Abs(ffa.Beta-wantBeta) > 1e-9 {
		t.Errorf("DefaultFFAConfig.Beta = %f, want σ·%.2f = %f", ffa.Beta, ffaBetaScale, wantBeta)
	}
	if ffa.Beta <= base.Beta {
		t.Errorf("FFA Beta should exceed 1v1 Beta: ffa=%f base=%f", ffa.Beta, base.Beta)
	}

	// r60 retune: τ = σ/200. Must be STRICTLY less than the 1v1 baseline.
	wantTau := defaultSigma * ffaTauScale
	if math.Abs(ffa.Tau-wantTau) > 1e-9 {
		t.Errorf("DefaultFFAConfig.Tau = %f, want σ·%.4f = %f", ffa.Tau, ffaTauScale, wantTau)
	}
	if ffa.Tau >= base.Tau {
		t.Errorf("FFA Tau should be less than 1v1 Tau (static-deck context): ffa=%f base=%f", ffa.Tau, base.Tau)
	}

	// r60 retune: DrawProbability = 0.005. Must be STRICTLY less than
	// the 1v1 baseline (0.02 action-game hedge).
	if math.Abs(ffa.DrawProbability-ffaDrawProbability) > 1e-9 {
		t.Errorf("DefaultFFAConfig.DrawProbability = %f, want %f", ffa.DrawProbability, ffaDrawProbability)
	}
	if ffa.DrawProbability >= base.DrawProbability {
		t.Errorf("FFA DrawProbability should be less than 1v1: ffa=%f base=%f",
			ffa.DrawProbability, base.DrawProbability)
	}
}

// TestNewTrueSkillRatings_UsesFFAPreset confirms the no-arg
// NewTrueSkillRatings constructor wires up the FFA config — tournament
// code paths must not silently pick up the 1v1 baseline.
func TestNewTrueSkillRatings_UsesFFAPreset(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	want := DefaultFFAConfig()
	if math.Abs(ts.cfg.Beta-want.Beta) > 1e-9 {
		t.Errorf("NewTrueSkillRatings Beta = %f, want FFA preset %f", ts.cfg.Beta, want.Beta)
	}
}

// TestHigherBeta_ProducesHumblerUpdates is the load-bearing convergence
// behavior pin: a higher β (more performance noise) must produce
// SMALLER per-game μ shifts than a lower β when the same upset occurs.
// This is the mathematical reason we bump β for FFA — pod outcomes are
// noisier per unit skill, so each game should move the rating less.
// If a future refactor accidentally swaps the role of Beta in v/w, this
// test catches it.
func TestHigherBeta_ProducesHumblerUpdates(t *testing.T) {
	lowBeta := Config{Beta: defaultSigma / 2.0, Tau: defaultSigma / 100.0, DrawProbability: 0.02}
	highBeta := Config{Beta: defaultSigma * 0.8, Tau: defaultSigma / 100.0, DrawProbability: 0.02}

	w := DefaultRating()
	l := DefaultRating()

	wLow, _ := Update2Player(lowBeta, w, l)
	wHigh, _ := Update2Player(highBeta, w, l)

	gainLow := wLow.Mu - w.Mu
	gainHigh := wHigh.Mu - w.Mu

	if gainHigh >= gainLow {
		t.Errorf("higher β should produce smaller μ shift: lowβ gain=%f highβ gain=%f",
			gainLow, gainHigh)
	}
	if gainHigh <= 0 {
		t.Errorf("higher β should still produce a positive update for the winner: %f", gainHigh)
	}
}

// TestFFAConvergence_4PlayerDeterministicSkill pins the FFA-preset
// convergence behavior: under a deterministic skill ordering (strong
// always 1st, weak always 4th, with one upset in 5), after 200 games
// the strong player's conservative skill (μ−3σ) must exceed every
// weaker player's by a clear margin. Locks in that the β bump does NOT
// break separation in the populated-tournament regime.
func TestFFAConvergence_4PlayerDeterministicSkill(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"strong", "mid", "weak", "weakest"})

	for i := 0; i < 200; i++ {
		ranks := []int{0, 1, 2, 3}
		// One upset every 5 games — mid leapfrogs strong, weakest stays last.
		if i%5 == 4 {
			ranks = []int{1, 0, 2, 3}
		}
		ts.Update([]string{"strong", "mid", "weak", "weakest"}, ranks)
	}

	strong := ts.Ratings["strong"]
	mid := ts.Ratings["mid"]
	weak := ts.Ratings["weak"]
	weakest := ts.Ratings["weakest"]

	if strong.Conservative() <= mid.Conservative() {
		t.Errorf("strong conservative should exceed mid: strong=%f mid=%f",
			strong.Conservative(), mid.Conservative())
	}
	if mid.Conservative() <= weak.Conservative() {
		t.Errorf("mid conservative should exceed weak: mid=%f weak=%f",
			mid.Conservative(), weak.Conservative())
	}
	if weak.Conservative() <= weakest.Conservative() {
		t.Errorf("weak conservative should exceed weakest: weak=%f weakest=%f",
			weak.Conservative(), weakest.Conservative())
	}
	// σ should shrink meaningfully — if it stays near σ₀ the β bump is
	// suppressing all information.
	if strong.Sigma >= defaultSigma*0.6 {
		t.Errorf("strong σ should shrink below 0.6·σ₀ after 200 games: σ=%f σ₀=%f",
			strong.Sigma, defaultSigma)
	}
}

// TestFFAvs1v1_ConvergesTighterUnderStaticDeckTau runs the same
// 200-game scenario under both presets and pins the joint signature of
// the r60 retune: σ converges TIGHTER under the FFA preset over a
// long-horizon static-deck self-play, because the τ reduction (σ/100 →
// σ/200) overpowers the β widening (σ/2 → σ·0.6) in σ-shrinkage rate.
//
// This is the *intended* property for HexDek: decks don't drift between
// games, so the rating system should converge faster to a tight σ
// rather than retaining session-noise headroom that a human-skill-drift
// scenario would need. The pre-r60 FFA preset retained MORE σ than 1v1
// (β-only divergence); the r60 retune flipped that for static-deck
// realism.
//
// Both presets still order strong > weak.
func TestFFAvs1v1_ConvergesTighterUnderStaticDeckTau(t *testing.T) {
	run := func(cfg Config) (strongMu, weakMu, strongSigma float64) {
		ts := NewTrueSkillRatingsWithConfig(
			[]string{"strong", "weak1", "weak2", "weak3"}, cfg)
		for i := 0; i < 200; i++ {
			ts.Update([]string{"strong", "weak1", "weak2", "weak3"}, []int{0, 1, 2, 3})
		}
		return ts.Ratings["strong"].Mu, ts.Ratings["weak1"].Mu, ts.Ratings["strong"].Sigma
	}

	strong1v1, weak1v1, sigma1v1 := run(DefaultConfig())
	strongFFA, weakFFA, sigmaFFA := run(DefaultFFAConfig())

	if strong1v1 <= weak1v1 {
		t.Errorf("1v1 preset must separate strong from weak: %f vs %f", strong1v1, weak1v1)
	}
	if strongFFA <= weakFFA {
		t.Errorf("FFA preset must separate strong from weak: %f vs %f", strongFFA, weakFFA)
	}
	if sigmaFFA >= sigma1v1 {
		t.Errorf("FFA σ should be LESS than 1v1 σ at convergence (r60: τ shrink overpowers β widen): "+
			"ffa=%f 1v1=%f", sigmaFFA, sigma1v1)
	}
}

// TestTauPreservesInformationOverTime confirms τ's role: σ inflates by
// √(σ² + τ²) at the start of each update, so after many games the
// floor σ stays just above τ rather than collapsing to 0. With τ =
// σ₀/100 ≈ 0.083, the floor is ~0.083 — well above the safety clamp at
// 1e-6 but well below σ₀.
func TestTauPreservesInformationOverTime(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})

	for i := 0; i < 1000; i++ {
		ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})
	}

	cfg := DefaultFFAConfig()
	for name, r := range ts.Ratings {
		if r.Sigma <= cfg.Tau*0.5 {
			t.Errorf("%s σ collapsed below τ/2 (τ-floor failure): σ=%f τ=%f",
				name, r.Sigma, cfg.Tau)
		}
		if r.Sigma > defaultSigma {
			t.Errorf("%s σ exceeds σ₀ after 1000 games: σ=%f σ₀=%f",
				name, r.Sigma, defaultSigma)
		}
	}
}
