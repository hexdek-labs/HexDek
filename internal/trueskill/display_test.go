package trueskill

import (
	"strings"
	"testing"
)

// --- ConfidenceLabel ----------------------------------------------------

func TestConfidenceLabel_AllBuckets(t *testing.T) {
	cases := []struct {
		sigma float64
		want  string
	}{
		{0.5, "high"},
		{2.0, "high"},          // boundary inclusive
		{2.0001, "medium"},     // just over → medium
		{3.5, "medium"},
		{4.0, "medium"},        // boundary inclusive
		{5.0, "low"},
		{6.0, "low"},           // boundary inclusive
		{6.0001, "very low"},
		{8.333, "very low"},    // default prior
	}
	for _, c := range cases {
		got := (Rating{Sigma: c.sigma}).ConfidenceLabel()
		if got != c.want {
			t.Errorf("σ=%.4f: ConfidenceLabel = %q, want %q", c.sigma, got, c.want)
		}
	}
}

func TestConfidenceLabel_DefaultRatingIsVeryLow(t *testing.T) {
	if got := DefaultRating().ConfidenceLabel(); got != "very low" {
		t.Errorf("default rating confidence: got %q, want %q (σ ≈ 8.33 is past every threshold)",
			got, "very low")
	}
}

// --- DisplayRating ------------------------------------------------------

func TestDisplayRating_DefaultRatingIs1000(t *testing.T) {
	// μ=25 σ=8.33 → conservative ≈ 0 → display ≈ 1000.
	got := DefaultRating().DisplayRating()
	if got < 999 || got > 1001 {
		t.Errorf("default rating display: got %d, want ≈ 1000 (rounding only)", got)
	}
}

func TestDisplayRating_StrongConvergedPlayer(t *testing.T) {
	// μ=35 σ=2 → conservative = 29 → 29*40 + 1000 = 2160.
	r := Rating{Mu: 35, Sigma: 2}
	got := r.DisplayRating()
	if got != 2160 {
		t.Errorf("strong converged player: got %d, want 2160", got)
	}
}

func TestDisplayRating_WeakConvergedPlayer(t *testing.T) {
	// μ=15 σ=2 → conservative = 9 → 9*40 + 1000 = 1360.
	r := Rating{Mu: 15, Sigma: 2}
	got := r.DisplayRating()
	if got != 1360 {
		t.Errorf("weak converged player: got %d, want 1360", got)
	}
}

func TestDisplayRating_BelowBaseClampsNaturally(t *testing.T) {
	// μ=5 σ=2 → conservative = -1 → -1*40 + 1000 = 960. Can go below
	// the 1000 offset for very-weak converged players; that's
	// intentional, the display has no floor.
	r := Rating{Mu: 5, Sigma: 2}
	got := r.DisplayRating()
	if got != 960 {
		t.Errorf("below-base converged: got %d, want 960", got)
	}
}

// --- DisplayUncertainty -------------------------------------------------

func TestDisplayUncertainty_ScalesSigma(t *testing.T) {
	cases := []struct {
		sigma float64
		want  int
	}{
		{0.5, 20},
		{1.0, 40},
		{2.0, 80},
		{2.5, 100},
		{8.333, 333}, // default prior
	}
	for _, c := range cases {
		got := (Rating{Sigma: c.sigma}).DisplayUncertainty()
		if got != c.want {
			t.Errorf("σ=%.4f: DisplayUncertainty = %d, want %d", c.sigma, got, c.want)
		}
	}
}

// --- PlayerProfile ------------------------------------------------------

func TestPlayerProfile_FormatMatchesSpec(t *testing.T) {
	// Hit the exact format from the task spec: "Rank: 1247 ± 23 (high
	// confidence)". Construct a rating that yields display rating 1247
	// and uncertainty 23:
	//   conservative = (1247 − 1000) / 40 = 6.175
	//   σ = 23 / 40 = 0.575
	//   μ = conservative + 3σ = 6.175 + 1.725 = 7.9
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["player"] = Rating{Mu: 7.9, Sigma: 0.575}
	ts.Games["player"] = 42

	p := ts.PlayerProfile("player")
	if p.Rating != 1247 {
		t.Errorf("Rating: got %d, want 1247", p.Rating)
	}
	if p.Uncertainty != 23 {
		t.Errorf("Uncertainty: got %d, want 23", p.Uncertainty)
	}
	if p.Confidence != "high" {
		t.Errorf("Confidence: got %q, want %q", p.Confidence, "high")
	}
	if p.Games != 42 {
		t.Errorf("Games: got %d, want 42", p.Games)
	}
	want := "Rank: 1247 ± 23 (high confidence)"
	if p.Display != want {
		t.Errorf("Display:\n  got:  %q\n  want: %q", p.Display, want)
	}
}

func TestPlayerProfile_PopulatesRawMuSigma(t *testing.T) {
	// Raw TrueSkill numbers must round-trip exactly so callers needing
	// non-display math don't need a separate lookup.
	ts := NewTrueSkillRatings(nil)
	ts.Ratings["raw"] = Rating{Mu: 28.7, Sigma: 3.14}
	p := ts.PlayerProfile("raw")
	if p.Mu != 28.7 {
		t.Errorf("raw Mu: got %f, want 28.7", p.Mu)
	}
	if p.Sigma != 3.14 {
		t.Errorf("raw Sigma: got %f, want 3.14", p.Sigma)
	}
}

func TestPlayerProfile_DefaultPriorPlayer(t *testing.T) {
	// A brand-new player using the default prior should surface as
	// ~1000 rating, large uncertainty, "very low" confidence — the
	// "we don't know you yet" UI state.
	ts := NewTrueSkillRatings([]string{"newcomer"})
	p := ts.PlayerProfile("newcomer")
	if p.Rating < 990 || p.Rating > 1010 {
		t.Errorf("default prior Rating: got %d, want ≈ 1000", p.Rating)
	}
	if p.Confidence != "very low" {
		t.Errorf("default prior Confidence: got %q, want %q", p.Confidence, "very low")
	}
	if !strings.Contains(p.Display, "very low confidence") {
		t.Errorf("Display string should label very-low confidence: got %q", p.Display)
	}
	if !strings.HasPrefix(p.Display, "Rank: ") {
		t.Errorf("Display should start with 'Rank: ': got %q", p.Display)
	}
}

func TestPlayerProfile_ConfidenceLabelsInDisplayString(t *testing.T) {
	// All four confidence labels must surface unchanged in the Display
	// string. Build one rating per bucket and verify the label appears.
	ts := NewTrueSkillRatings(nil)
	cases := []struct {
		name  string
		sigma float64
		label string
	}{
		{"sharp", 1.0, "high"},
		{"steady", 3.0, "medium"},
		{"new", 5.0, "low"},
		{"fresh", 7.5, "very low"},
	}
	for _, c := range cases {
		ts.Ratings[c.name] = Rating{Mu: 25, Sigma: c.sigma}
		p := ts.PlayerProfile(c.name)
		if p.Confidence != c.label {
			t.Errorf("%s: Confidence=%q, want %q", c.name, p.Confidence, c.label)
		}
		wantFragment := "(" + c.label + " confidence)"
		if !strings.Contains(p.Display, wantFragment) {
			t.Errorf("%s: Display missing %q: %q", c.name, wantFragment, p.Display)
		}
	}
}

func TestPlayerProfile_UnknownPlayerReturnsZero(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"known"})
	p := ts.PlayerProfile("missing")
	if (p != PlayerProfile{}) {
		t.Errorf("unknown player: got %+v, want zero PlayerProfile", p)
	}
}

func TestPlayerProfile_GamesCountFlowsThrough(t *testing.T) {
	ts := NewTrueSkillRatings([]string{"counted"})
	ts.Games["counted"] = 137
	p := ts.PlayerProfile("counted")
	if p.Games != 137 {
		t.Errorf("Games: got %d, want 137", p.Games)
	}
}
