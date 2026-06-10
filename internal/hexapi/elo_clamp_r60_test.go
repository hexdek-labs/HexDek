package hexapi

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/trueskill"
)

// elo_clamp_r60_test.go — pins the ELO corruption band-aid (r60): clampedTS
// must bound μ/σ and the conservative rating into the sane band for runaway,
// NaN, and degenerate-σ inputs, while leaving sane ratings essentially intact.

func TestClampedTS_BoundsRunawayNaNAndDegenerate(t *testing.T) {
	cases := []struct {
		name string
		in   trueskill.Rating
	}{
		{"runaway-negative", trueskill.Rating{Mu: -4.83e8, Sigma: 1.6e8}}, // the observed corruption
		{"runaway-positive", trueskill.Rating{Mu: 9.2e8, Sigma: 1e8}},
		{"nan", trueskill.Rating{Mu: math.NaN(), Sigma: math.NaN()}},
		{"sigma-zero", trueskill.Rating{Mu: 1800, Sigma: 0}}, // post-reload degenerate
		{"sane-b3", trueskill.Rating{Mu: 3000, Sigma: 400}},
	}
	for _, tc := range cases {
		mu, sigma, rating := clampedTS(tc.in)
		if math.IsNaN(mu) || math.IsNaN(sigma) || math.IsNaN(rating) {
			t.Errorf("%s: produced NaN (mu=%v sigma=%v rating=%v)", tc.name, mu, sigma, rating)
		}
		if mu < tsMuFloor || mu > tsMuCeil {
			t.Errorf("%s: mu %v outside [%v,%v]", tc.name, mu, tsMuFloor, tsMuCeil)
		}
		if sigma < tsSigmaFloor || sigma > tsDefaultSigma {
			t.Errorf("%s: sigma %v outside [%v,%v]", tc.name, sigma, tsSigmaFloor, tsDefaultSigma)
		}
		if rating < eloRatingMin || rating > eloRatingMax {
			t.Errorf("%s: rating %v outside [%v,%v]", tc.name, rating, eloRatingMin, eloRatingMax)
		}
	}
}

func TestClampedTS_PreservesSaneRating(t *testing.T) {
	// A healthy B3-ish rating (μ=3000, σ=400 → conservative 1800) should pass
	// through unchanged — the clamp must not move in-band values.
	mu, sigma, rating := clampedTS(trueskill.Rating{Mu: 3000, Sigma: 400})
	if mu != 3000 || sigma != 400 {
		t.Errorf("sane μ/σ moved: mu=%v sigma=%v, want 3000/400", mu, sigma)
	}
	if rating != 1800 {
		t.Errorf("conservative rating = %v, want 1800", rating)
	}
}
