package trueskill

import (
	"math"
	"testing"
)

func TestRatingInterval_BasicMath(t *testing.T) {
	r := Rating{Mu: 25.0, Sigma: 5.0}
	lo, hi := r.Interval(1.0)
	if math.Abs(lo-20.0) > 1e-9 || math.Abs(hi-30.0) > 1e-9 {
		t.Errorf("Interval(1): got [%.4f, %.4f], want [20, 30]", lo, hi)
	}
	lo2, hi2 := r.Interval(2.0)
	if math.Abs(lo2-15.0) > 1e-9 || math.Abs(hi2-35.0) > 1e-9 {
		t.Errorf("Interval(2): got [%.4f, %.4f], want [15, 35]", lo2, hi2)
	}
}

func TestRatingInterval_ZeroKDegenerate(t *testing.T) {
	r := Rating{Mu: 25.0, Sigma: 8.333}
	lo, hi := r.Interval(0)
	if lo != hi || lo != r.Mu {
		t.Errorf("Interval(0) should collapse to μ: got [%.4f, %.4f], want [25, 25]", lo, hi)
	}
}

func TestRatingInterval_MatchesConservativeAtK3(t *testing.T) {
	r := Rating{Mu: 25.0, Sigma: 8.333}
	lo, _ := r.Interval(3.0)
	if math.Abs(lo-r.Conservative()) > 1e-9 {
		t.Errorf("Interval(3) lo (%.4f) should equal Conservative() (%.4f)", lo, r.Conservative())
	}
}

func TestSkillInterval95_MatchesBookValue(t *testing.T) {
	r := Rating{Mu: 25.0, Sigma: 1.0}
	lo, hi := r.SkillInterval95()
	// μ ± 1.96σ → [23.04, 26.96]
	if math.Abs(lo-23.04) > 1e-9 || math.Abs(hi-26.96) > 1e-9 {
		t.Errorf("SkillInterval95 on σ=1: got [%.4f, %.4f], want [23.04, 26.96]", lo, hi)
	}
}

// --- RankInterval --------------------------------------------------------

// TestRankInterval_FullyConvergedLeaderboardIsFixed — when σ is tiny
// across the table (every player has lots of games), every player's
// k=1.96 CI is non-overlapping with their neighbors, so Best=Center=Worst.
func TestRankInterval_FullyConvergedLeaderboardIsFixed(t *testing.T) {
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["alpha"] = Rating{Mu: 35, Sigma: 0.5}
	ts.Ratings["bravo"] = Rating{Mu: 30, Sigma: 0.5}
	ts.Ratings["charlie"] = Rating{Mu: 25, Sigma: 0.5}
	ts.Ratings["delta"] = Rating{Mu: 20, Sigma: 0.5}

	ri := ts.RankInterval("bravo", 1.96)
	// bravo's [29.02, 30.98] doesn't overlap with alpha's [34.02, 35.98]
	// or charlie's [24.02, 25.98] → rank is locked at 2.
	if ri.Center != 2 || ri.Best != 2 || ri.Worst != 2 {
		t.Errorf("converged bravo: got Center=%d Best=%d Worst=%d, want 2/2/2",
			ri.Center, ri.Best, ri.Worst)
	}
	if ri.PlusMinus() != 0 {
		t.Errorf("PlusMinus on fully-converged rank: got %d, want 0", ri.PlusMinus())
	}
}

// TestRankInterval_DefaultPriorIsWideOpen — at default rating
// (μ=25, σ≈8.33) every player's k=1.96 CI overlaps every other player's
// CI, so Best=1 and Worst=N for everyone.
func TestRankInterval_DefaultPriorIsWideOpen(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d", "e"})
	for name := range ts.Ratings {
		ri := ts.RankInterval(name, 1.96)
		if ri.Best != 1 {
			t.Errorf("default-prior %s: Best=%d, want 1 (all CIs overlap fully)", name, ri.Best)
		}
		if ri.Worst != 5 {
			t.Errorf("default-prior %s: Worst=%d, want 5 (all CIs overlap fully)", name, ri.Worst)
		}
	}
}

// TestRankInterval_PartialOverlapShape — middle player whose CI overlaps
// neighbors on both sides but NOT the extremes. "rank 3 ± 1" shape:
// Center=3, Best=2, Worst=4.
func TestRankInterval_PartialOverlapShape(t *testing.T) {
	ts := NewTrueSkillRatings(nil)
	// 5 players with descending μ; player "mid" has σ=2 so its
	// [μ−3.92, μ+3.92] overlaps immediate neighbors but not the extremes.
	ts.Ratings["top"] = Rating{Mu: 40, Sigma: 0.5}    // CI [39.02, 40.98]
	ts.Ratings["high"] = Rating{Mu: 32, Sigma: 0.5}   // CI [31.02, 32.98]
	ts.Ratings["mid"] = Rating{Mu: 30, Sigma: 2.0}    // CI [26.08, 33.92]
	ts.Ratings["low"] = Rating{Mu: 28, Sigma: 0.5}    // CI [27.02, 28.98]
	ts.Ratings["bot"] = Rating{Mu: 20, Sigma: 0.5}    // CI [19.02, 20.98]

	ri := ts.RankInterval("mid", 1.96)
	// Center: order by Conservative (μ-3σ):
	//   top  40 - 1.5 = 38.5
	//   high 32 - 1.5 = 30.5
	//   low  28 - 1.5 = 26.5
	//   mid  30 - 6   = 24
	//   bot  20 - 1.5 = 18.5
	// Center for "mid" = 4 (top, high, low above).
	if ri.Center != 4 {
		t.Errorf("mid Center: got %d, want 4 (Conservative ordering)", ri.Center)
	}
	// Best: count of "definitely above" = players whose lo > mid's hi (33.92).
	//   top.lo=39.02 > 33.92 ✓
	//   high.lo=31.02 < 33.92 ✗ (overlaps)
	//   low.lo=27.02 < 33.92 ✗
	//   bot.lo=19.02 < 33.92 ✗
	// definitelyAbove = 1, Best = 2.
	if ri.Best != 2 {
		t.Errorf("mid Best: got %d, want 2 (only top is unambiguously above)", ri.Best)
	}
	// Worst: count of "definitely below" = players whose hi < mid's lo (26.08).
	//   top.hi=40.98 ✗
	//   high.hi=32.98 ✗
	//   low.hi=28.98 ✗ (overlaps low's hi > mid's lo)
	//   bot.hi=20.98 < 26.08 ✓
	// definitelyBelow = 1, Worst = 5 - 1 = 4.
	if ri.Worst != 4 {
		t.Errorf("mid Worst: got %d, want 4 (only bot is unambiguously below)", ri.Worst)
	}
	if ri.PlusMinus() != 2 {
		t.Errorf("mid PlusMinus: got %d, want 2 (Center=4, Best=2 → ±2)", ri.PlusMinus())
	}
}

func TestRankInterval_UnknownPlayerReturnsZero(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"alpha"})
	ri := ts.RankInterval("missing", 1.96)
	if (ri != RankInterval{}) {
		t.Errorf("unknown player should return zero RankInterval, got %+v", ri)
	}
}

func TestLeaderboardWithIntervals_OrderingAndCoverage(t *testing.T) {
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["alpha"] = Rating{Mu: 35, Sigma: 1.0}
	ts.Ratings["bravo"] = Rating{Mu: 30, Sigma: 1.0}
	ts.Ratings["charlie"] = Rating{Mu: 25, Sigma: 1.0}

	board := ts.LeaderboardWithIntervals(1.96)
	if len(board) != 3 {
		t.Fatalf("LeaderboardWithIntervals returned %d entries, want 3", len(board))
	}
	// Must be ordered by Center ascending (rank 1, 2, 3).
	for i, ri := range board {
		if ri.Center != i+1 {
			t.Errorf("entry %d Center: got %d, want %d", i, ri.Center, i+1)
		}
	}
	if board[0].Player != "alpha" || board[1].Player != "bravo" || board[2].Player != "charlie" {
		t.Errorf("leaderboard order: got %s/%s/%s, want alpha/bravo/charlie",
			board[0].Player, board[1].Player, board[2].Player)
	}
}

// TestRankInterval_AsymmetricPlusMinus — when Best=6 and Worst=10 with
// Center=7, PlusMinus should report 3 (the WIDER side), not 1.
// Surfaces with one summary number must not understate uncertainty.
func TestRankInterval_AsymmetricPlusMinusReportsWiderSide(t *testing.T) {
	ri := RankInterval{Center: 7, Best: 6, Worst: 10}
	if ri.PlusMinus() != 3 {
		t.Errorf("asymmetric PlusMinus: got %d, want 3 (Worst-Center=3 > Center-Best=1)",
			ri.PlusMinus())
	}
}
