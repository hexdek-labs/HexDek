package trueskill

import (
	"math"
	"sort"
)

// WinProbabilityCI returns a credible interval on the head-to-head win
// probability of A over B at k standard deviations of skill posterior.
//
// The headline number WinProbability(cfg, a, b) is the EXPECTED win prob
// after marginalizing over both players' skill posteriors — it absorbs
// σ² into c. The CI exposed here is the spread of that estimate: if A's
// true skill is at the high end of its posterior and B's at the low
// end, the win prob is much higher than the point estimate suggests,
// and vice versa. Useful for "favored player but high uncertainty"
// matchmaking UIs that want to say "65% favored, but the system is
// only 95% sure it's in [54%, 75%]".
//
// Math (analytical propagation):
//
//	diff = μ_A − μ_B
//	σ_diff = √(σ_A² + σ_B²)
//	diff_lo = diff − k·σ_diff
//	diff_hi = diff + k·σ_diff
//	perfNoise = √(2·β²)
//	lo = Φ(diff_lo / perfNoise)
//	hi = Φ(diff_hi / perfNoise)
//	point = WinProbability(cfg, a, b)   // unchanged from existing API
//
// Note that lo/hi are the win prob CONDITIONAL on the skill draw at the
// CI endpoints; the divisor is √(2β²), not c, because the σ² uncertainty
// has already been used to widen `diff`. The `point` value uses c (the
// expected win prob over the full posterior), so it can deviate from
// the midpoint of [lo, hi] when σ is large — this is correct behavior,
// not a bug. The point estimate is risk-neutral; the CI endpoints are
// the realized win probs at specific skill draws.
//
// Common k values: 1.0 → 68% CI, 1.96 → 95% CI, 2.0 → 95.4%, 3.0 → 99.7%.
// k ≤ 0 collapses the CI to the point estimate.
func WinProbabilityCI(cfg Config, a, b Rating, k float64) (point, lo, hi float64) {
	point = WinProbability(cfg, a, b)
	if k <= 0 {
		return point, point, point
	}
	diff := a.Mu - b.Mu
	sigmaDiff := math.Sqrt(a.Sigma*a.Sigma + b.Sigma*b.Sigma)
	perfNoise := math.Sqrt(2 * cfg.Beta * cfg.Beta)
	if perfNoise <= 0 {
		return point, point, point
	}
	diffLo := diff - k*sigmaDiff
	diffHi := diff + k*sigmaDiff
	lo = normCDF(diffLo / perfNoise)
	hi = normCDF(diffHi / perfNoise)
	return point, lo, hi
}

// WinProbability95CI returns the 95% credible interval (k=1.96). Sugar
// over WinProbabilityCI for the most-common display case.
func WinProbability95CI(cfg Config, a, b Rating) (point, lo, hi float64) {
	return WinProbabilityCI(cfg, a, b, 1.96)
}

// WinProbabilityCI is the tracker-aware version: looks up both players
// by name and returns the k-σ credible interval on the win prob of
// nameA over nameB. Returns (0.5, 0.5, 0.5) if either name is missing
// — neutral collapse matching tracker WinProbability's "neutral
// default" behavior so matchmaking doesn't bias against missing data.
func (ts *TrueSkillRatings) WinProbabilityCI(nameA, nameB string, k float64) (point, lo, hi float64) {
	ra, okA := ts.Ratings[nameA]
	rb, okB := ts.Ratings[nameB]
	if !okA || !okB {
		return 0.5, 0.5, 0.5
	}
	return WinProbabilityCI(ts.cfg, ra, rb, k)
}

// MatchCandidateCI augments MatchCandidate with a CI on WinProbability.
// Lo/Hi flank Point (which equals MatchCandidate.WinProbability) at the
// requested k-σ skill posterior bounds.
type MatchCandidateCI struct {
	MatchCandidate
	WinProbabilityLo float64 `json:"win_probability_lo"`
	WinProbabilityHi float64 `json:"win_probability_hi"`
	CIWidth          float64 `json:"ci_width"` // Hi − Lo; convenience for sorting/filtering
}

// BestMatchesWithCI returns the top-n opponents for the named player,
// each with a k-σ CI on the win-probability prediction. Sorted by
// MatchQuality descending (same as BestMatches), tie-broken by name.
// Pass n ≤ 0 for all opponents; pass k = 1.96 for the canonical 95% CI.
//
// Use case: a profile page wants to show "your next match" with both
// the favored side AND the uncertainty in that prediction. If
// CIWidth is large (the system isn't sure), the UI can render a
// "preview" badge or queue more games to converge the rating.
func (ts *TrueSkillRatings) BestMatchesWithCI(name string, n int, k float64) []MatchCandidateCI {
	self, ok := ts.Ratings[name]
	if !ok {
		return nil
	}
	out := make([]MatchCandidateCI, 0, len(ts.Ratings)-1)
	for other, r := range ts.Ratings {
		if other == name {
			continue
		}
		point, lo, hi := WinProbabilityCI(ts.cfg, self, r, k)
		out = append(out, MatchCandidateCI{
			MatchCandidate: MatchCandidate{
				Opponent:       other,
				Quality:        MatchQuality(ts.cfg, self, r),
				WinProbability: point,
			},
			WinProbabilityLo: lo,
			WinProbabilityHi: hi,
			CIWidth:          hi - lo,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Quality != out[j].Quality {
			return out[i].Quality > out[j].Quality
		}
		return out[i].Opponent < out[j].Opponent
	})
	if n > 0 && n < len(out) {
		out = out[:n]
	}
	return out
}
