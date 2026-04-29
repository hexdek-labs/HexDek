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
