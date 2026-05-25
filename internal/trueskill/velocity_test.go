package trueskill

import (
	"math"
	"testing"
)

// makeHistory builds a synthetic per-player history with given μAfter
// values. SigmaBefore is 5.0 throughout (unused by velocity but populated
// so the entries are realistic).
func makeHistory(name string, muAfters ...float64) (*TrueSkillRatings, []RatingDelta) {
	ts := &TrueSkillRatings{
		Ratings: map[string]Rating{name: {Mu: muAfters[len(muAfters)-1], Sigma: 5.0}},
		Games:   map[string]int{name: len(muAfters)},
		History: map[string][]RatingDelta{},
	}
	deltas := make([]RatingDelta, len(muAfters))
	prev := 25.0
	for i, m := range muAfters {
		deltas[i] = RatingDelta{
			Game:        i + 1,
			MuBefore:    prev,
			MuAfter:     m,
			SigmaBefore: 5.0,
			SigmaAfter:  5.0,
			Rank:        0,
		}
		prev = m
	}
	ts.History[name] = deltas
	return ts, deltas
}

func TestRatingVelocity_PureLinearAscent(t *testing.T) {
	// μ climbs by exactly 1.0 per game: slope = 1.0, R² = 1.0.
	ts, _ := makeHistory("rising", 26, 27, 28, 29, 30, 31, 32, 33, 34, 35)
	v := ts.RatingVelocity("rising", 10)
	if math.Abs(v.MuPerGame-1.0) > 1e-9 {
		t.Errorf("pure +1/game ascent: MuPerGame=%.6f, want 1.0", v.MuPerGame)
	}
	if math.Abs(v.R2-1.0) > 1e-9 {
		t.Errorf("pure linear: R²=%.6f, want 1.0", v.R2)
	}
	if math.Abs(v.NetMuChange-9.0) > 1e-9 {
		t.Errorf("net Δμ over 10 games of +1: got %.4f, want 9.0", v.NetMuChange)
	}
	if v.FirstMu != 26 || v.LastMu != 35 {
		t.Errorf("First/Last: got %.2f/%.2f, want 26/35", v.FirstMu, v.LastMu)
	}
	if v.Window != 10 {
		t.Errorf("Window: got %d, want 10", v.Window)
	}
}

func TestRatingVelocity_PureLinearDescent(t *testing.T) {
	// μ falls by 2.0 per game (sandbagger / decline / smurf-exposed
	// trajectory): slope = -2.0, R² = 1.0.
	ts, _ := makeHistory("falling", 40, 38, 36, 34, 32, 30)
	v := ts.RatingVelocity("falling", 6)
	if math.Abs(v.MuPerGame-(-2.0)) > 1e-9 {
		t.Errorf("pure -2/game descent: MuPerGame=%.6f, want -2.0", v.MuPerGame)
	}
	if math.Abs(v.R2-1.0) > 1e-9 {
		t.Errorf("pure linear: R²=%.6f, want 1.0", v.R2)
	}
}

func TestRatingVelocity_FlatTrajectoryR2Is1(t *testing.T) {
	// μ identical for every game: slope = 0, R² = 1 (perfect fit by
	// convention — the constant function fits exactly).
	ts, _ := makeHistory("stable", 25, 25, 25, 25, 25)
	v := ts.RatingVelocity("stable", 5)
	if v.MuPerGame != 0 {
		t.Errorf("flat: MuPerGame=%f, want 0", v.MuPerGame)
	}
	if math.Abs(v.R2-1.0) > 1e-9 {
		t.Errorf("flat trajectory R² convention: got %.6f, want 1.0", v.R2)
	}
}

func TestRatingVelocity_NoisyTrajectoryLowR2(t *testing.T) {
	// μ oscillates wildly with no trend: slope ≈ 0, R² near 0.
	ts, _ := makeHistory("noisy", 25, 30, 22, 28, 24, 27, 23, 26, 25, 26)
	v := ts.RatingVelocity("noisy", 10)
	// Slope is small (no real trend).
	if math.Abs(v.MuPerGame) > 0.5 {
		t.Errorf("noisy trajectory should have near-zero slope: got %.4f", v.MuPerGame)
	}
	// R² should be well below "sustained trend" territory.
	if v.R2 > 0.3 {
		t.Errorf("noisy trajectory R²: got %.4f, want < 0.3 (no trend)", v.R2)
	}
}

func TestRatingVelocity_SmurfTrajectoryHighVelocityHighR2(t *testing.T) {
	// A smurf starts at default μ=25 and climbs sharply toward true
	// skill μ≈45 over their first 10 games. Velocity should be strongly
	// positive (climbing) with high R² (sustained, not random).
	ts, _ := makeHistory("smurf", 27, 30, 33, 36, 39, 41, 43, 44, 45, 45)
	v := ts.RatingVelocity("smurf", 10)
	if v.MuPerGame < 1.5 {
		t.Errorf("smurf trajectory: MuPerGame=%.4f, want > 1.5 (sustained climb)",
			v.MuPerGame)
	}
	if v.R2 < 0.9 {
		t.Errorf("smurf trajectory R²: got %.4f, want > 0.9 (sustained, not noise)",
			v.R2)
	}
}

func TestRatingVelocity_LastGameSpikeDoesNotDominateSlope(t *testing.T) {
	// A flat trajectory with a single last-game spike. Naive end-to-end
	// (NetMuChange / window) would report a large velocity; OLS slope
	// should be moderate because most of the data is flat.
	ts, _ := makeHistory("spike", 25, 25, 25, 25, 25, 25, 25, 25, 25, 40)
	v := ts.RatingVelocity("spike", 10)
	// Naive end-to-end would suggest ~1.5/game; OLS should be smaller.
	// Verify the SLOPE is meaningfully lower than NetMuChange/(window-1)
	// — that's the whole point of using regression over end-to-end.
	naive := v.NetMuChange / float64(v.Window-1)
	if v.MuPerGame >= naive {
		t.Errorf("OLS slope (%.4f) should be smaller than naive end-to-end (%.4f) for late spike",
			v.MuPerGame, naive)
	}
	// R² should be modest — one spike out of 10 doesn't make a trend.
	if v.R2 > 0.6 {
		t.Errorf("last-game spike R²: got %.4f, want < 0.6 (one outlier ≠ trend)", v.R2)
	}
}

func TestRatingVelocity_UnknownPlayerReturnsZero(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"alpha"})
	v := ts.RatingVelocity("missing", 10)
	if (v != RatingVelocity{}) {
		t.Errorf("unknown player: got %+v, want zero value", v)
	}
}

func TestRatingVelocity_InsufficientHistoryReturnsZero(t *testing.T) {
	ts, _ := makeHistory("newbie", 26)
	v := ts.RatingVelocity("newbie", 10)
	if (v != RatingVelocity{}) {
		t.Errorf("single-game history: got %+v, want zero (slope undefined for 1 point)", v)
	}
}

func TestRatingVelocity_WindowLargerThanHistoryUsesAll(t *testing.T) {
	ts, _ := makeHistory("short", 26, 27, 28)
	v := ts.RatingVelocity("short", 100)
	if v.Window != 3 {
		t.Errorf("Window when requested > history: got %d, want 3 (actual count used)", v.Window)
	}
	if math.Abs(v.MuPerGame-1.0) > 1e-9 {
		t.Errorf("MuPerGame on short history: got %.4f, want 1.0", v.MuPerGame)
	}
}

func TestRatingVelocity_NonPositiveWindowUsesFullHistory(t *testing.T) {
	ts, _ := makeHistory("full", 26, 27, 28, 29, 30)
	v := ts.RatingVelocity("full", 0)
	if v.Window != 5 {
		t.Errorf("Window=0 should use full history: got %d, want 5", v.Window)
	}
	v2 := ts.RatingVelocity("full", -1)
	if v2.Window != 5 {
		t.Errorf("Window=-1 should use full history: got %d, want 5", v2.Window)
	}
}

func TestTopVelocities_SortedByAbsoluteVelocity(t *testing.T) {
	// Three players with very different trajectory magnitudes.
	ts := &TrueSkillRatings{
		Ratings: map[string]Rating{},
		Games:   map[string]int{},
		History: map[string][]RatingDelta{},
	}
	addHistory := func(name string, mus ...float64) {
		prev := 25.0
		for i, m := range mus {
			ts.History[name] = append(ts.History[name], RatingDelta{
				Game: i + 1, MuBefore: prev, MuAfter: m,
				SigmaBefore: 5.0, SigmaAfter: 5.0,
			})
			prev = m
		}
	}
	addHistory("fastClimb", 30, 33, 36, 39, 42)  // slope ≈ +3
	addHistory("steadyDrop", 24, 23, 22, 21, 20) // slope ≈ -1
	addHistory("stable", 25, 25, 25, 25, 25)     // slope = 0

	top := ts.TopVelocities(5, 0)
	if len(top) != 3 {
		t.Fatalf("got %d entries, want 3", len(top))
	}
	if top[0].Name != "fastClimb" {
		t.Errorf("ranked first: got %s, want fastClimb (|slope| ≈ 3)", top[0].Name)
	}
	if top[1].Name != "steadyDrop" {
		t.Errorf("ranked second: got %s, want steadyDrop (|slope| ≈ 1)", top[1].Name)
	}
	if top[2].Name != "stable" {
		t.Errorf("ranked third: got %s, want stable (slope = 0)", top[2].Name)
	}

	// n-cap respected.
	topTwo := ts.TopVelocities(5, 2)
	if len(topTwo) != 2 {
		t.Errorf("TopVelocities cap: got %d, want 2", len(topTwo))
	}
}

func TestTopVelocities_SkipsPlayersWithSinglePoint(t *testing.T) {
	ts := &TrueSkillRatings{
		Ratings: map[string]Rating{},
		Games:   map[string]int{},
		History: map[string][]RatingDelta{
			"newbie": {{Game: 1, MuBefore: 25, MuAfter: 26, SigmaBefore: 5}},
			"vet": {
				{Game: 1, MuBefore: 25, MuAfter: 26, SigmaBefore: 5},
				{Game: 2, MuBefore: 26, MuAfter: 28, SigmaBefore: 5},
			},
		},
	}
	top := ts.TopVelocities(10, 0)
	if len(top) != 1 {
		t.Fatalf("got %d entries, want 1 (newbie should be skipped)", len(top))
	}
	if top[0].Name != "vet" {
		t.Errorf("got %s, want vet", top[0].Name)
	}
}
