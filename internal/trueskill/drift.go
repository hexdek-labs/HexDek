package trueskill

import (
	"math"
	"sort"
)

// RatingDelta is one row of a player's update history: the μ and σ
// before and after one TrueSkillRatings.Update call they participated
// in, plus the rank they finished at. The series is append-only and
// indexed in chronological order.
type RatingDelta struct {
	// Game is the participant's 1-indexed game number — incremented
	// for each Update call this player appears in. Distinct from
	// any global game counter; lets a drift detector compare players
	// who joined late to the population.
	Game        int     `json:"game"`
	MuBefore    float64 `json:"mu_before"`
	MuAfter     float64 `json:"mu_after"`
	SigmaBefore float64 `json:"sigma_before"`
	SigmaAfter  float64 `json:"sigma_after"`
	Rank        int     `json:"rank"`
}

// DeltaMu returns the signed μ shift this update produced.
func (d RatingDelta) DeltaMu() float64 {
	return d.MuAfter - d.MuBefore
}

// SigmaNormalizedShift returns |Δμ| / σ_before — the σ-normalized
// magnitude of one update. This is the load-bearing drift metric:
// scale-invariant (so an early-career and a late-career update compare
// fairly), bounded in practice by the TrueSkill update math.
//
// Baseline reference under the FFA preset (β = σ·0.6, drawP = 2%):
//
//	Fresh equal-skill player (σ ≈ σ₀ = 8.33): ~0.35 per game.
//	Established equal-skill player (σ ≈ 3.0): ~0.23 per game.
//	Established mismatched (σ ≈ 3.0, upset): can spike to ~0.7 once.
//
// Persistent values above ~0.5 over a 10-game window indicate the
// system is being repeatedly surprised — either the player's true
// skill genuinely shifted (legitimate improvement) or the rating was
// miscalibrated to begin with (smurf account, deck-swap, throwing).
// Direction disambiguates the two interpretations.
func (d RatingDelta) SigmaNormalizedShift() float64 {
	if d.SigmaBefore <= 0 {
		return 0
	}
	return math.Abs(d.DeltaMu()) / d.SigmaBefore
}

// DriftFlag is one flagged player. AvgSigmaPerGame is the mean
// SigmaNormalizedShift over the last Window games (those games are
// echoed in Recent for inspection). Direction summarizes the signed
// trend: "rising" if ≥80% of the deltas in the window are positive,
// "falling" if ≥80% are negative, otherwise "mixed".
type DriftFlag struct {
	Name            string        `json:"name"`
	Window          int           `json:"window"`
	AvgSigmaPerGame float64       `json:"avg_sigma_per_game"`
	NetMuChange     float64       `json:"net_mu_change"`
	Direction       string        `json:"direction"`
	Recent          []RatingDelta `json:"recent"`
}

// DefaultDriftThreshold is the recommended σ/game cutoff for the FFA
// preset. Calibrated against the SigmaNormalizedShift baselines in
// the RatingDelta doc: 0.5 sits above the established-player and
// fresh-player floors but well below the 0.7+ smurf-trajectory region.
// Callers tuning for sensitivity vs noise can override per-call.
const DefaultDriftThreshold = 0.5

// DefaultDriftWindow is the recommended look-back. 10 games is short
// enough to catch a fresh smurf within a couple sessions yet long
// enough that one upset doesn't trigger a flag on its own.
const DefaultDriftWindow = 10

// DetectDrift scans the per-player history and returns one DriftFlag
// for each name whose mean σ-normalized shift over its most recent
// `window` games exceeds `threshold`. Players with fewer than `window`
// games are skipped — there isn't enough signal yet to distinguish
// drift from initial-rating noise.
//
// Returned flags are sorted by AvgSigmaPerGame descending (worst
// offender first). Pass DefaultDriftWindow / DefaultDriftThreshold
// for the FFA-tuned defaults; pass smaller window/larger threshold
// for noisier populations (live play, frequent deck-swaps); pass
// larger window/smaller threshold for higher-confidence triage of
// established players.
func (ts *TrueSkillRatings) DetectDrift(window int, threshold float64) []DriftFlag {
	if window <= 0 {
		window = DefaultDriftWindow
	}
	if threshold <= 0 {
		threshold = DefaultDriftThreshold
	}

	flags := make([]DriftFlag, 0)
	for name, history := range ts.History {
		if len(history) < window {
			continue
		}
		recent := history[len(history)-window:]

		sum := 0.0
		net := 0.0
		pos := 0
		neg := 0
		for _, d := range recent {
			sum += d.SigmaNormalizedShift()
			delta := d.DeltaMu()
			net += delta
			switch {
			case delta > 0:
				pos++
			case delta < 0:
				neg++
			}
		}
		avg := sum / float64(window)
		if avg < threshold {
			continue
		}

		direction := "mixed"
		switch {
		case float64(pos)/float64(window) >= 0.8:
			direction = "rising"
		case float64(neg)/float64(window) >= 0.8:
			direction = "falling"
		}

		recentCopy := make([]RatingDelta, len(recent))
		copy(recentCopy, recent)
		flags = append(flags, DriftFlag{
			Name:            name,
			Window:          window,
			AvgSigmaPerGame: avg,
			NetMuChange:     net,
			Direction:       direction,
			Recent:          recentCopy,
		})
	}

	sort.SliceStable(flags, func(i, j int) bool {
		return flags[i].AvgSigmaPerGame > flags[j].AvgSigmaPerGame
	})
	return flags
}
