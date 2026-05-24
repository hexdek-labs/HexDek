package trueskill

import (
	"math"
	"testing"
)

// playFFA runs `count` games over the four named players with a fixed
// finishing order each game. Used to build synthetic histories with
// known characteristics (always-wins, always-loses, etc.).
func playFFA(ts *TrueSkillRatings, names []string, ranks []int, count int) {
	for i := 0; i < count; i++ {
		ts.Update(names, ranks)
	}
}

// TestUpdate_RecordsHistory pins the structural contract: every Update
// must append one RatingDelta per participant, with correctly populated
// before/after fields and the rank carried through. This is the
// load-bearing prerequisite for any drift detection.
func TestUpdate_RecordsHistory(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	beforeA := ts.Ratings["a"]

	ts.Update([]string{"a", "b", "c", "d"}, []int{0, 1, 2, 3})

	if len(ts.History["a"]) != 1 {
		t.Fatalf("History[a] length = %d, want 1", len(ts.History["a"]))
	}
	d := ts.History["a"][0]
	if d.Game != 1 {
		t.Errorf("Game = %d, want 1", d.Game)
	}
	if d.Rank != 0 {
		t.Errorf("Rank = %d, want 0", d.Rank)
	}
	if d.MuBefore != beforeA.Mu {
		t.Errorf("MuBefore = %f, want %f", d.MuBefore, beforeA.Mu)
	}
	if d.MuAfter != ts.Ratings["a"].Mu {
		t.Errorf("MuAfter = %f, want %f", d.MuAfter, ts.Ratings["a"].Mu)
	}
	if d.SigmaBefore != beforeA.Sigma {
		t.Errorf("SigmaBefore = %f, want %f", d.SigmaBefore, beforeA.Sigma)
	}
	if d.DeltaMu() <= 0 {
		t.Errorf("DeltaMu for the winner should be positive: %f", d.DeltaMu())
	}
}

// TestRatingDelta_SigmaNormalizedShift_ZeroSigma confirms the guard:
// if SigmaBefore is somehow zero (post-clamp, post-serialization round-
// trip), the shift returns 0 rather than dividing by zero.
func TestRatingDelta_SigmaNormalizedShift_ZeroSigma(t *testing.T) {
	d := RatingDelta{MuBefore: 25, MuAfter: 30, SigmaBefore: 0}
	if got := d.SigmaNormalizedShift(); got != 0 {
		t.Errorf("zero-σ shift should be 0, got %f", got)
	}
}

// TestDetectDrift_StableSkill_NoFlags is the false-positive guard.
// Under a deterministic same-ordering-every-game regime, after the
// rating system has settled, mean σ-shift should drop well below the
// default 0.5 threshold. If this ever flags, we'd see false positives
// in real tournaments.
func TestDetectDrift_StableSkill_NoFlags(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	// 100 games to let σ settle, then 20 more to populate the window.
	playFFA(ts, []string{"a", "b", "c", "d"}, []int{0, 1, 2, 3}, 120)

	flags := ts.DetectDrift(DefaultDriftWindow, DefaultDriftThreshold)
	for _, f := range flags {
		t.Errorf("unexpected drift flag on stable-skill regime: %+v", f)
	}
}

// TestDetectDrift_SmurfTrajectory is the primary positive case: a
// player who consistently wins despite the system rating them at parity
// with their pod will exhibit a high σ-shift average across the window.
// Scenario: 30 games of "victims dominate smurf" calibrate the pod's
// ratings (smurf low, victims high), then exactly 10 reversal wins fill
// the detection window. Empirically (see drift probe in PR notes) this
// produces avg σ-shift ≈ 0.76 — well above DefaultDriftThreshold.
//
// Note on streak length: a longer streak (>15 games) decays the avg
// below threshold as the system catches up; a shorter streak (<window)
// doesn't fill the window. The cal=30/streak=10 scenario sits in the
// sweet spot where the surprise is still fresh across all 10 deltas.
func TestDetectDrift_SmurfTrajectory(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"smurf", "v1", "v2", "v3"})
	playFFA(ts, []string{"smurf", "v1", "v2", "v3"}, []int{3, 0, 1, 2}, 30)
	playFFA(ts, []string{"smurf", "v1", "v2", "v3"}, []int{0, 1, 2, 3}, 10)

	flags := ts.DetectDrift(DefaultDriftWindow, DefaultDriftThreshold)

	found := false
	for _, f := range flags {
		if f.Name == "smurf" {
			found = true
			if f.AvgSigmaPerGame < DefaultDriftThreshold {
				t.Errorf("smurf σ-shift below threshold: %f", f.AvgSigmaPerGame)
			}
			if f.Direction != "rising" {
				t.Errorf("smurf direction should be 'rising', got %q (net Δμ=%f)",
					f.Direction, f.NetMuChange)
			}
			if f.NetMuChange <= 0 {
				t.Errorf("smurf net μ change should be positive: %f", f.NetMuChange)
			}
			if len(f.Recent) != DefaultDriftWindow {
				t.Errorf("Recent length = %d, want %d", len(f.Recent), DefaultDriftWindow)
			}
		}
	}
	if !found {
		t.Errorf("expected smurf to be flagged; got flags=%+v", flags)
	}
}

// TestDetectDrift_SandbaggerTrajectory mirrors the smurf case in
// reverse: a player established at high skill suddenly starts throwing.
// The system is also repeatedly surprised, but in the negative
// direction.
func TestDetectDrift_SandbaggerTrajectory(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"thrower", "v1", "v2", "v3"})
	// Establish thrower as the strongest with 30 wins, then 10 last-
	// place finishes fill the window with consistent reversals.
	// Same cal/streak shape as the smurf scenario (mirrored direction).
	playFFA(ts, []string{"thrower", "v1", "v2", "v3"}, []int{0, 1, 2, 3}, 30)
	playFFA(ts, []string{"thrower", "v1", "v2", "v3"}, []int{3, 0, 1, 2}, 10)

	flags := ts.DetectDrift(DefaultDriftWindow, DefaultDriftThreshold)

	found := false
	for _, f := range flags {
		if f.Name == "thrower" {
			found = true
			if f.AvgSigmaPerGame < DefaultDriftThreshold {
				t.Errorf("thrower σ-shift below threshold: %f", f.AvgSigmaPerGame)
			}
			if f.Direction != "falling" {
				t.Errorf("thrower direction should be 'falling', got %q (net Δμ=%f)",
					f.Direction, f.NetMuChange)
			}
			if f.NetMuChange >= 0 {
				t.Errorf("thrower net μ change should be negative: %f", f.NetMuChange)
			}
		}
	}
	if !found {
		t.Errorf("expected thrower to be flagged; got flags=%+v", flags)
	}
}

// TestDetectDrift_OneGameSpike_NotFlagged confirms a single-game
// outlier doesn't trigger a flag when averaged over the window. This is
// what protects fair players who happened to have one upset.
func TestDetectDrift_OneGameSpike_NotFlagged(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	// 100 stable games — a always wins.
	playFFA(ts, []string{"a", "b", "c", "d"}, []int{0, 1, 2, 3}, 100)
	// One single upset: a comes dead last.
	playFFA(ts, []string{"a", "b", "c", "d"}, []int{3, 0, 1, 2}, 1)
	// 9 more stable games to fill the window with mostly-noise.
	playFFA(ts, []string{"a", "b", "c", "d"}, []int{0, 1, 2, 3}, 9)

	flags := ts.DetectDrift(DefaultDriftWindow, DefaultDriftThreshold)
	for _, f := range flags {
		if f.Name == "a" {
			t.Errorf("a should NOT be flagged for one-game spike in a 10-window: %+v", f)
		}
	}
}

// TestDetectDrift_InsufficientHistory_Skipped pins the early-life
// behavior: players with fewer than `window` games are silently
// excluded — there's not enough data to distinguish anomaly from
// initial-rating noise.
func TestDetectDrift_InsufficientHistory_Skipped(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"newcomer", "v1", "v2", "v3"})
	playFFA(ts, []string{"newcomer", "v1", "v2", "v3"}, []int{0, 1, 2, 3}, 3)

	flags := ts.DetectDrift(DefaultDriftWindow, DefaultDriftThreshold)
	for _, f := range flags {
		if f.Name == "newcomer" {
			t.Errorf("newcomer with 3 games should not be flagged: %+v", f)
		}
	}
}

// TestDetectDrift_WindowExactlyEqualsHistory confirms the boundary
// condition: if a player has EXACTLY `window` games, the detector
// includes them (>= window, not strict >). Uses direct history
// injection to isolate the boundary check from any TrueSkill update-
// math interaction — 10 synthetic deltas at σ-norm = 0.6 each.
func TestDetectDrift_WindowExactlyEqualsHistory(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"target"})
	for i := 0; i < 10; i++ {
		ts.History["target"] = append(ts.History["target"], RatingDelta{
			Game:        i + 1,
			MuBefore:    25.0,
			MuAfter:     27.4, // Δμ = 2.4, σ_before = 4.0 → σ-norm = 0.6
			SigmaBefore: 4.0,
			SigmaAfter:  3.95,
			Rank:        0,
		})
	}

	flags := ts.DetectDrift(10, DefaultDriftThreshold)
	if len(flags) != 1 {
		t.Fatalf("target with exactly window games should flag: got %d", len(flags))
	}
	if flags[0].Window != 10 {
		t.Errorf("Window = %d, want 10", flags[0].Window)
	}
	// Add an 11th delta and confirm the window slides forward (only the
	// LAST 10 are considered, not all 11).
	ts.History["target"] = append(ts.History["target"], RatingDelta{
		Game:        11,
		MuBefore:    27.4,
		MuAfter:     27.4, // Δμ = 0 → σ-norm = 0, drags average down
		SigmaBefore: 3.95,
		SigmaAfter:  3.9,
		Rank:        1,
	})
	flags = ts.DetectDrift(10, DefaultDriftThreshold)
	if len(flags) != 1 {
		t.Fatalf("target should still flag after 11 games (9×0.6 + 1×0 = avg 0.54): got %d",
			len(flags))
	}
}

// TestDetectDrift_ZeroParams_UsesDefaults confirms that calling with
// window=0 / threshold=0 uses the package-level defaults — convenient
// for "just give me the default report" callers.
func TestDetectDrift_ZeroParams_UsesDefaults(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	playFFA(ts, []string{"a", "b", "c", "d"}, []int{0, 1, 2, 3}, DefaultDriftWindow+2)

	defaultFlags := ts.DetectDrift(DefaultDriftWindow, DefaultDriftThreshold)
	zeroFlags := ts.DetectDrift(0, 0)
	if len(defaultFlags) != len(zeroFlags) {
		t.Errorf("zero-arg should equal default-arg: default=%d zero=%d",
			len(defaultFlags), len(zeroFlags))
	}
}

// TestDetectDrift_SortedByMagnitude confirms the output ordering: the
// worst offender (highest AvgSigmaPerGame) appears first. Tournament
// dashboards rely on this for triage.
func TestDetectDrift_SortedByMagnitude(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"smurf1", "smurf2", "v1", "v2"})
	// smurf1: 30 games losing, then 12 winning (worse anomaly)
	playFFA(ts, []string{"smurf1", "smurf2", "v1", "v2"}, []int{2, 3, 0, 1}, 30)
	playFFA(ts, []string{"smurf1", "smurf2", "v1", "v2"}, []int{0, 3, 1, 2}, 12)
	// During the second phase smurf2 also won some, but less consistently.

	flags := ts.DetectDrift(DefaultDriftWindow, DefaultDriftThreshold)
	for i := 1; i < len(flags); i++ {
		if flags[i-1].AvgSigmaPerGame < flags[i].AvgSigmaPerGame {
			t.Errorf("flags not sorted by AvgSigmaPerGame desc: %v", flags)
		}
	}
}

// TestDetectDrift_ThresholdControlsSensitivity confirms threshold
// tuning works as documented. A very low threshold (0.05) should
// flag nearly everyone after a few games (since the σ-shift baseline
// is ~0.25-0.35); a very high threshold (5.0) should flag no one
// even in a smurf scenario.
func TestDetectDrift_ThresholdControlsSensitivity(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"a", "b", "c", "d"})
	playFFA(ts, []string{"a", "b", "c", "d"}, []int{0, 1, 2, 3}, 15)

	low := ts.DetectDrift(DefaultDriftWindow, 0.05)
	if len(low) == 0 {
		t.Errorf("threshold=0.05 should flag the strong-position players")
	}
	high := ts.DetectDrift(DefaultDriftWindow, 5.0)
	if len(high) != 0 {
		t.Errorf("threshold=5.0 should flag nobody (no rating moves >5σ/game): %v", high)
	}
}

// TestDetectDrift_AvgComputationCorrectness pins the arithmetic. Use a
// known sequence of deltas, inject them directly into History, and
// verify AvgSigmaPerGame equals the manually-computed mean. Bypasses
// the TrueSkill update math so any future Refactor of Update can't
// silently invalidate the drift metric.
func TestDetectDrift_AvgComputationCorrectness(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"target"})
	// Inject a hand-built history: 10 games with σ=4.0 and Δμ
	// alternating ±2.0. Each |Δμ|/σ = 0.5 → average exactly 0.5.
	for i := 0; i < 10; i++ {
		delta := 2.0
		if i%2 == 1 {
			delta = -2.0
		}
		ts.History["target"] = append(ts.History["target"], RatingDelta{
			Game:        i + 1,
			MuBefore:    25.0,
			MuAfter:     25.0 + delta,
			SigmaBefore: 4.0,
			SigmaAfter:  4.0,
			Rank:        0,
		})
	}

	flags := ts.DetectDrift(10, 0.4) // threshold below 0.5 so target flags
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}
	f := flags[0]
	if math.Abs(f.AvgSigmaPerGame-0.5) > 1e-9 {
		t.Errorf("AvgSigmaPerGame = %f, want exactly 0.5", f.AvgSigmaPerGame)
	}
	if f.Direction != "mixed" {
		t.Errorf("alternating ± should classify as 'mixed', got %q", f.Direction)
	}
	if math.Abs(f.NetMuChange) > 1e-9 {
		t.Errorf("NetMuChange should be ~0 for alternating signs: %f", f.NetMuChange)
	}
}

// TestDetectDrift_DirectionThresholds pins the 80%-positive / 80%-
// negative rule for direction classification. 7-of-10 positive → mixed;
// 8-of-10 positive → rising. Synthetic injection again to bypass
// floating-point.
func TestDetectDrift_DirectionThresholds(t *testing.T) {
	for _, c := range []struct {
		name      string
		posCount  int
		wantLabel string
	}{
		{"7-pos-3-neg", 7, "mixed"},
		{"8-pos-2-neg", 8, "rising"},
		{"2-pos-8-neg", 2, "falling"},
		{"5-pos-5-neg", 5, "mixed"},
		{"10-pos-0-neg", 10, "rising"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ts := NewTrueSkillRatings([]string{"x"})
			for i := 0; i < 10; i++ {
				delta := -3.0
				if i < c.posCount {
					delta = 3.0
				}
				ts.History["x"] = append(ts.History["x"], RatingDelta{
					Game:        i + 1,
					MuBefore:    25.0,
					MuAfter:     25.0 + delta,
					SigmaBefore: 4.0,
					SigmaAfter:  4.0,
					Rank:        0,
				})
			}
			flags := ts.DetectDrift(10, 0.5)
			if len(flags) != 1 {
				t.Fatalf("expected 1 flag, got %d", len(flags))
			}
			if flags[0].Direction != c.wantLabel {
				t.Errorf("posCount=%d direction=%q, want %q",
					c.posCount, flags[0].Direction, c.wantLabel)
			}
		})
	}
}
