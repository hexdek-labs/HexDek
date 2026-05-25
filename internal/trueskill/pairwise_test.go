package trueskill

import (
	"math"
	"testing"
)

// --- WinProbability ------------------------------------------------------

func TestWinProbability_EqualRatingsIsFiftyPercent(t *testing.T) {
	cfg := DefaultConfig()
	r := Rating{Mu: 25, Sigma: 3}
	p := WinProbability(cfg, r, r)
	if math.Abs(p-0.5) > 1e-9 {
		t.Errorf("equal ratings WinProbability: got %.6f, want 0.5", p)
	}
}

func TestWinProbability_SymmetrySumsToOne(t *testing.T) {
	cfg := DefaultConfig()
	a := Rating{Mu: 30, Sigma: 4}
	b := Rating{Mu: 22, Sigma: 5}
	p := WinProbability(cfg, a, b) + WinProbability(cfg, b, a)
	if math.Abs(p-1.0) > 1e-9 {
		t.Errorf("WinProb(A,B) + WinProb(B,A) = %.6f, want 1.0", p)
	}
}

func TestWinProbability_HigherMuFavored(t *testing.T) {
	cfg := DefaultConfig()
	favored := Rating{Mu: 35, Sigma: 3}
	underdog := Rating{Mu: 20, Sigma: 3}
	p := WinProbability(cfg, favored, underdog)
	if p <= 0.5 {
		t.Errorf("higher-μ favored: WinProb = %.4f, want > 0.5", p)
	}
	if p >= 1.0 {
		t.Errorf("WinProb should be < 1.0 even for a heavy favorite, got %.4f", p)
	}
}

func TestWinProbability_HigherSigmaMovesTowardFifty(t *testing.T) {
	// Same μ gap but higher σ in both players → less certain who wins,
	// so the favored player's win probability is CLOSER to 0.5.
	cfg := DefaultConfig()
	confidentA := Rating{Mu: 30, Sigma: 1}
	confidentB := Rating{Mu: 25, Sigma: 1}
	uncertainA := Rating{Mu: 30, Sigma: 6}
	uncertainB := Rating{Mu: 25, Sigma: 6}

	pConfident := WinProbability(cfg, confidentA, confidentB)
	pUncertain := WinProbability(cfg, uncertainA, uncertainB)

	if !(pUncertain < pConfident && pUncertain > 0.5) {
		t.Errorf("uncertainty should pull win-prob toward 0.5: confident=%.4f uncertain=%.4f",
			pConfident, pUncertain)
	}
}

// --- OutcomeProbabilities -----------------------------------------------

func TestOutcomeProbabilities_SumToOne(t *testing.T) {
	cfg := DefaultConfig()
	a := Rating{Mu: 30, Sigma: 4}
	b := Rating{Mu: 22, Sigma: 5}
	pA, pD, pB := OutcomeProbabilities(cfg, a, b)
	sum := pA + pD + pB
	if math.Abs(sum-1.0) > 1e-9 {
		t.Errorf("W/D/L sum: got %.6f, want 1.0 (pA=%.4f pD=%.4f pB=%.4f)",
			sum, pA, pD, pB)
	}
	if pA < 0 || pD < 0 || pB < 0 {
		t.Errorf("probabilities must be non-negative: %.4f / %.4f / %.4f", pA, pD, pB)
	}
}

func TestOutcomeProbabilities_ZeroDrawProbCollapsesToBinary(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DrawProbability = 0
	a := Rating{Mu: 30, Sigma: 3}
	b := Rating{Mu: 25, Sigma: 3}
	pA, pD, pB := OutcomeProbabilities(cfg, a, b)
	// drawMargin uses the Abramowitz-Stegun rational approximation to
	// the inverse normal CDF, which yields ε on the order of 1e-5 (not
	// exactly 0) at drawProb=0. The match-up math is then off by a few
	// parts in 10⁴, well below any matchmaking-visible threshold. Use
	// a 1e-3 tolerance to acknowledge the approximation while still
	// catching any real regression in the W/D/L collapse.
	const tol = 1e-3
	if math.Abs(pD) > tol {
		t.Errorf("zero draw prob: pDraw = %.6f, want ~0 (tol %.0e)", pD, tol)
	}
	wp := WinProbability(cfg, a, b)
	if math.Abs(pA-wp) > tol {
		t.Errorf("zero draw: pAWin (%.6f) should be ≈ WinProbability (%.6f)", pA, wp)
	}
	if math.Abs(pA+pB-1.0) > tol {
		t.Errorf("zero draw: pA + pB = %.6f, want ≈ 1.0", pA+pB)
	}
}

func TestOutcomeProbabilities_HigherDrawProbInflatesDraws(t *testing.T) {
	a := Rating{Mu: 25, Sigma: 3}
	b := Rating{Mu: 25, Sigma: 3}
	low := DefaultConfig()
	low.DrawProbability = 0.01
	high := DefaultConfig()
	high.DrawProbability = 0.30

	_, pDLow, _ := OutcomeProbabilities(low, a, b)
	_, pDHigh, _ := OutcomeProbabilities(high, a, b)

	if !(pDHigh > pDLow) {
		t.Errorf("higher DrawProbability should inflate pDraw: low=%.4f high=%.4f",
			pDLow, pDHigh)
	}
}

// --- MatchQuality --------------------------------------------------------

func TestMatchQuality_EqualRatingsLowSigmaNearOne(t *testing.T) {
	cfg := DefaultConfig() // β ≈ σ/2 ≈ 4.17
	r := Rating{Mu: 25, Sigma: 0.5} // tight prior
	q := MatchQuality(cfg, r, r)
	if q < 0.95 {
		t.Errorf("equal-rating low-σ match should be near 1.0: got %.4f", q)
	}
}

func TestMatchQuality_Symmetric(t *testing.T) {
	cfg := DefaultConfig()
	a := Rating{Mu: 30, Sigma: 4}
	b := Rating{Mu: 22, Sigma: 5}
	qa := MatchQuality(cfg, a, b)
	qb := MatchQuality(cfg, b, a)
	if math.Abs(qa-qb) > 1e-12 {
		t.Errorf("MatchQuality not symmetric: %.6f vs %.6f", qa, qb)
	}
}

func TestMatchQuality_DropsWithMuGap(t *testing.T) {
	cfg := DefaultConfig()
	close := Rating{Mu: 25.5, Sigma: 2}
	far := Rating{Mu: 45.0, Sigma: 2}
	mid := Rating{Mu: 25.0, Sigma: 2}

	qClose := MatchQuality(cfg, mid, close)
	qFar := MatchQuality(cfg, mid, far)
	if !(qClose > qFar) {
		t.Errorf("closer μ should produce higher quality: close=%.4f far=%.4f",
			qClose, qFar)
	}
}

func TestMatchQuality_DropsWithUncertainty(t *testing.T) {
	cfg := DefaultConfig()
	// Same μ — only σ differs.
	tight := Rating{Mu: 25, Sigma: 0.5}
	wide := Rating{Mu: 25, Sigma: 8}

	qTight := MatchQuality(cfg, tight, tight)
	qWide := MatchQuality(cfg, wide, wide)
	if !(qTight > qWide) {
		t.Errorf("uncertain matchup should have LOWER quality: tight=%.4f wide=%.4f",
			qTight, qWide)
	}
}

func TestMatchQuality_BlowoutNearZero(t *testing.T) {
	cfg := DefaultConfig()
	expert := Rating{Mu: 50, Sigma: 1}
	novice := Rating{Mu: 5, Sigma: 1}
	q := MatchQuality(cfg, expert, novice)
	if q > 0.01 {
		t.Errorf("expert vs novice MatchQuality: %.6f, want near 0", q)
	}
}

// --- Tracker convenience methods ----------------------------------------

func TestTracker_WinProbabilityNamed(t *testing.T) {
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["alpha"] = Rating{Mu: 35, Sigma: 2}
	ts.Ratings["bravo"] = Rating{Mu: 25, Sigma: 2}

	pAlpha := ts.WinProbability("alpha", "bravo")
	pBravo := ts.WinProbability("bravo", "alpha")
	if !(pAlpha > 0.5) {
		t.Errorf("alpha (μ=35) vs bravo (μ=25): pAlpha = %.4f, want > 0.5", pAlpha)
	}
	if math.Abs(pAlpha+pBravo-1.0) > 1e-9 {
		t.Errorf("named symmetry: pAlpha + pBravo = %.6f, want 1.0", pAlpha+pBravo)
	}
}

func TestTracker_WinProbabilityUnknownReturnsHalf(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"alpha"})
	if p := ts.WinProbability("alpha", "ghost"); p != 0.5 {
		t.Errorf("unknown opponent: got %.4f, want 0.5 (neutral default)", p)
	}
	if p := ts.WinProbability("ghost", "alpha"); p != 0.5 {
		t.Errorf("unknown query player: got %.4f, want 0.5", p)
	}
}

func TestTracker_MatchQualityUnknownReturnsZero(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"alpha"})
	// Unknown opponent — quality should be 0 so matchmaking
	// heuristics don't prefer ghost candidates.
	if q := ts.MatchQuality("alpha", "ghost"); q != 0 {
		t.Errorf("unknown opponent MatchQuality: got %.4f, want 0", q)
	}
}

// --- BestMatches --------------------------------------------------------

func TestBestMatches_SortsByQualityDescending(t *testing.T) {
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["me"] = Rating{Mu: 25, Sigma: 2}
	ts.Ratings["even"] = Rating{Mu: 25, Sigma: 2}     // best quality
	ts.Ratings["slight"] = Rating{Mu: 28, Sigma: 2}   // medium
	ts.Ratings["blowout"] = Rating{Mu: 45, Sigma: 2}  // worst

	best := ts.BestMatches("me", 0) // all
	if len(best) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(best))
	}
	if best[0].Opponent != "even" {
		t.Errorf("rank 1: got %s, want even (equal μ)", best[0].Opponent)
	}
	if best[1].Opponent != "slight" {
		t.Errorf("rank 2: got %s, want slight", best[1].Opponent)
	}
	if best[2].Opponent != "blowout" {
		t.Errorf("rank 3: got %s, want blowout", best[2].Opponent)
	}
	if !(best[0].Quality > best[1].Quality && best[1].Quality > best[2].Quality) {
		t.Errorf("qualities should be strictly descending: %.4f / %.4f / %.4f",
			best[0].Quality, best[1].Quality, best[2].Quality)
	}
}

func TestBestMatches_WinProbabilityIsFromQueryPerspective(t *testing.T) {
	// A 35-μ "me" against three opponents must report DIFFERENT win
	// probabilities (favored against weak, even against equal,
	// underdog against strong).
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["me"] = Rating{Mu: 35, Sigma: 2}
	ts.Ratings["weak"] = Rating{Mu: 20, Sigma: 2}
	ts.Ratings["equal"] = Rating{Mu: 35, Sigma: 2}
	ts.Ratings["strong"] = Rating{Mu: 50, Sigma: 2}

	best := ts.BestMatches("me", 0)
	winByName := make(map[string]float64, len(best))
	for _, c := range best {
		winByName[c.Opponent] = c.WinProbability
	}
	if !(winByName["weak"] > 0.5) {
		t.Errorf("vs weak: WinProb = %.4f, want > 0.5", winByName["weak"])
	}
	if math.Abs(winByName["equal"]-0.5) > 1e-9 {
		t.Errorf("vs equal: WinProb = %.6f, want 0.5", winByName["equal"])
	}
	if !(winByName["strong"] < 0.5) {
		t.Errorf("vs strong: WinProb = %.4f, want < 0.5", winByName["strong"])
	}
}

func TestBestMatches_RespectsNCap(t *testing.T) {
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["me"] = Rating{Mu: 25, Sigma: 2}
	for i := 0; i < 10; i++ {
		ts.Ratings[string(rune('a'+i))] = Rating{Mu: float64(20 + i), Sigma: 2}
	}
	best := ts.BestMatches("me", 3)
	if len(best) != 3 {
		t.Errorf("n=3 cap: got %d, want 3", len(best))
	}
}

func TestBestMatches_ExcludesSelfAndUnknown(t *testing.T) {
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["me"] = Rating{Mu: 25, Sigma: 2}
	ts.Ratings["other"] = Rating{Mu: 30, Sigma: 2}

	best := ts.BestMatches("me", 0)
	if len(best) != 1 {
		t.Fatalf("got %d candidates, want 1 (must exclude self)", len(best))
	}
	if best[0].Opponent != "other" {
		t.Errorf("got %s, want other", best[0].Opponent)
	}

	if got := ts.BestMatches("ghost", 0); got != nil {
		t.Errorf("BestMatches for unknown player: got %v, want nil", got)
	}
}

func TestBestMatches_TieBreaksByName(t *testing.T) {
	// Two opponents with identical ratings against the query must sort
	// by name so output is stable across runs.
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["me"] = Rating{Mu: 25, Sigma: 2}
	ts.Ratings["zeta"] = Rating{Mu: 25, Sigma: 2}
	ts.Ratings["alpha"] = Rating{Mu: 25, Sigma: 2}

	for trial := 0; trial < 5; trial++ {
		best := ts.BestMatches("me", 0)
		if best[0].Opponent != "alpha" || best[1].Opponent != "zeta" {
			t.Errorf("trial %d: tie-break should be alphabetical, got %s / %s",
				trial, best[0].Opponent, best[1].Opponent)
		}
	}
}
