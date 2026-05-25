package trueskill

import (
	"fmt"
	"math"
)

// Display scaling constants. TrueSkill operates in a [0, ~50] μ range
// with σ₀ ≈ 8.33; the ELO-style "1000-2200" range most ladder UIs show
// is more legible to players. The default scale maps:
//   - μ=25, σ=8.33 (fresh player) → DisplayRating ≈ 1000 (conservative ≈ 0)
//   - μ=25, σ=2.0  (converged median) → DisplayRating ≈ 1760
//   - μ=35, σ=2.0  (strong, converged) → DisplayRating ≈ 2160
//
// Callers wanting raw TrueSkill numbers can read Mu/Sigma directly off
// the Rating struct; this package handles the display-layer affordances.
const (
	DisplayScale  = 40.0
	DisplayOffset = 1000.0
)

// Confidence label thresholds (in TrueSkill σ units, not display units).
// Tuned against the SigmaNormalizedShift baselines documented on
// RatingDelta: a player with σ ≤ 2 has converged enough that one upset
// won't materially shift them; σ in (2, 4] is the established-but-still-
// moving regime; σ in (4, 6] is early-career; σ > 6 is still essentially
// at the prior.
const (
	HighConfidenceSigma   = 2.0
	MediumConfidenceSigma = 4.0
	LowConfidenceSigma    = 6.0
)

// ConfidenceLabel categorizes σ into a UI-friendly label. The four
// buckets ("high", "medium", "low", "very low") let UIs render
// confidence as a chip / color without re-deriving the math.
func (r Rating) ConfidenceLabel() string {
	switch {
	case r.Sigma <= HighConfidenceSigma:
		return "high"
	case r.Sigma <= MediumConfidenceSigma:
		return "medium"
	case r.Sigma <= LowConfidenceSigma:
		return "low"
	default:
		return "very low"
	}
}

// DisplayRating returns the conservative rating (μ − 3σ) scaled to the
// ELO-friendly display range, rounded to the nearest integer. Conservative
// is used (not raw μ) because UI surfaces should show what the system is
// confident the player is AT LEAST — surfacing μ alone would over-promise
// for high-σ players. Symmetric with the rest of the Conservative-based
// ranking system (Snapshot, RankInterval).
func (r Rating) DisplayRating() int {
	return int(math.Round(r.Conservative()*DisplayScale + DisplayOffset))
}

// DisplayUncertainty returns σ scaled to the display range, rounded to
// the nearest integer. This is the "± N" half-width in the canonical
// "Rank: X ± N" display. Tied to the same DisplayScale so units match
// between the rating value and its uncertainty.
func (r Rating) DisplayUncertainty() int {
	return int(math.Round(r.Sigma * DisplayScale))
}

// PlayerProfile is the UI-ready bundle for a single player's rating.
// All fields are display-formatted: Rating and Uncertainty are
// pre-scaled integers in ELO-style units, Confidence is one of the
// four labels, and Display is a pre-formatted string ready to drop
// into a profile card.
type PlayerProfile struct {
	Name        string `json:"name"`
	Rating      int    `json:"rating"`      // scaled conservative rating, ELO-style
	Uncertainty int    `json:"uncertainty"` // scaled σ in same units as Rating
	Confidence  string `json:"confidence"`  // "high" / "medium" / "low" / "very low"
	Games       int    `json:"games"`
	Mu          float64 `json:"mu"`     // raw TrueSkill values for callers who need them
	Sigma       float64 `json:"sigma"`
	Display     string `json:"display"` // "Rank: 1247 ± 23 (high confidence)"
}

// PlayerProfile assembles a UI-display bundle for the named player.
// Returns the zero PlayerProfile (with Name="") if the player isn't
// tracked.
//
// Display string format: "Rank: <rating> ± <uncertainty> (<confidence>
// confidence)" — drop directly into a profile card or leaderboard
// hover. For unconventional formatting (e.g. localized strings),
// access the structured fields and format yourself.
func (ts *TrueSkillRatings) PlayerProfile(name string) PlayerProfile {
	r, ok := ts.Ratings[name]
	if !ok {
		return PlayerProfile{}
	}
	rating := r.DisplayRating()
	uncertainty := r.DisplayUncertainty()
	confidence := r.ConfidenceLabel()
	return PlayerProfile{
		Name:        name,
		Rating:      rating,
		Uncertainty: uncertainty,
		Confidence:  confidence,
		Games:       ts.Games[name],
		Mu:          r.Mu,
		Sigma:       r.Sigma,
		Display:     fmt.Sprintf("Rank: %d ± %d (%s confidence)", rating, uncertainty, confidence),
	}
}
