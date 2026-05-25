package trueskill

import (
	"math"
	"sort"
)

// WinProbability returns the probability that player A beats player B
// in a hypothetical decisive (no-draw) head-to-head match. Accounts for
// the uncertainty in BOTH players' ratings — two evenly-matched players
// at high σ still get ≈0.5 but with the formula correctly absorbing
// their joint uncertainty.
//
// Formula (TrueSkill paper, decisive case):
//
//	c = √(2β² + σ_A² + σ_B²)
//	P(A > B) = Φ((μ_A − μ_B) / c)
//
// Properties:
//   - Symmetry: WinProbability(cfg, a, b) + WinProbability(cfg, b, a) = 1.
//   - Equal μ → 0.5 regardless of σ.
//   - μ_A ≫ μ_B → 1 (with rate moderated by σ).
//   - Increasing either σ pulls the result toward 0.5 (the system is
//     less sure who's better).
//
// For a full W/D/L breakdown (using the config's draw probability),
// use [OutcomeProbabilities].
func WinProbability(cfg Config, a, b Rating) float64 {
	c := math.Sqrt(2*cfg.Beta*cfg.Beta + a.Sigma*a.Sigma + b.Sigma*b.Sigma)
	if c <= 0 {
		return 0.5
	}
	return normCDF((a.Mu - b.Mu) / c)
}

// OutcomeProbabilities returns the (P(A wins), P(draw), P(B wins))
// distribution for a head-to-head match between A and B, accounting for
// the config's DrawProbability. The three values sum to 1.
//
// Formula:
//
//	c = √(2β² + σ_A² + σ_B²)
//	ε = drawMargin(DrawProbability, β)
//	P(A wins) = Φ((μ_A − μ_B − ε) / c)
//	P(B wins) = Φ((μ_B − μ_A − ε) / c)
//	P(draw)   = 1 − P(A wins) − P(B wins)
//
// When DrawProbability is 0, ε is 0 and P(draw) collapses to 0, giving
// the same answer as [WinProbability] split between A and B.
func OutcomeProbabilities(cfg Config, a, b Rating) (pAWin, pDraw, pBWin float64) {
	c := math.Sqrt(2*cfg.Beta*cfg.Beta + a.Sigma*a.Sigma + b.Sigma*b.Sigma)
	if c <= 0 {
		return 0.5, 0, 0.5
	}
	epsilon := drawMargin(cfg.DrawProbability, cfg.Beta)
	pAWin = normCDF((a.Mu - b.Mu - epsilon) / c)
	pBWin = normCDF((b.Mu - a.Mu - epsilon) / c)
	pDraw = 1 - pAWin - pBWin
	if pDraw < 0 {
		pDraw = 0
	}
	return pAWin, pDraw, pBWin
}

// MatchQuality returns a 0..1 score for how well A and B are matched —
// the standard TrueSkill match-quality measure. 1.0 means a maximally
// even match (equal μ, low σ); 0 means certain blowout.
//
// Formula:
//
//	c² = 2β² + σ_A² + σ_B²
//	q  = √(2β² / c²) · exp(−(μ_A − μ_B)² / (2c²))
//
// Use for matchmaking: pair players to maximize q across the pool.
// Properties:
//   - Symmetric: MatchQuality(cfg, a, b) = MatchQuality(cfg, b, a).
//   - Equal μ + low σ → near 1.
//   - Wide μ gap → near 0.
//   - Same μ but high σ → reduced quality (uncertain matchups are
//     lower quality even if the means align).
func MatchQuality(cfg Config, a, b Rating) float64 {
	c2 := 2*cfg.Beta*cfg.Beta + a.Sigma*a.Sigma + b.Sigma*b.Sigma
	if c2 <= 0 {
		return 0
	}
	prefactor := math.Sqrt(2 * cfg.Beta * cfg.Beta / c2)
	muDiff := a.Mu - b.Mu
	return prefactor * math.Exp(-muDiff*muDiff/(2*c2))
}

// WinProbability returns P(named A beats named B) under the tracker's
// config. Returns 0.5 if either name is unknown — neutral default that
// won't bias matchmaking heuristics against missing data.
func (ts *TrueSkillRatings) WinProbability(nameA, nameB string) float64 {
	ra, okA := ts.Ratings[nameA]
	rb, okB := ts.Ratings[nameB]
	if !okA || !okB {
		return 0.5
	}
	return WinProbability(ts.cfg, ra, rb)
}

// MatchQuality returns the 0..1 quality score for a hypothetical
// head-to-head pairing of the two named players under the tracker's
// config. Returns 0 if either name is unknown (no-data pairings should
// not be preferred over real candidates).
func (ts *TrueSkillRatings) MatchQuality(nameA, nameB string) float64 {
	ra, okA := ts.Ratings[nameA]
	rb, okB := ts.Ratings[nameB]
	if !okA || !okB {
		return 0
	}
	return MatchQuality(ts.cfg, ra, rb)
}

// MatchCandidate is one row of BestMatches output.
type MatchCandidate struct {
	Opponent       string  `json:"opponent"`
	Quality        float64 `json:"quality"`
	WinProbability float64 `json:"win_probability"` // for the QUERY player against this opponent
}

// BestMatches returns the top-n opponents for the named player, sorted
// by MatchQuality descending. WinProbability is filled in for each
// candidate so callers can render a paired "even match, you're 53%
// favored" UI without re-querying. Pass n ≤ 0 to return every tracked
// opponent. Returns nil if the player isn't tracked.
//
// Tie-breaking by name keeps results stable across calls with the same
// inputs — useful when two prospective opponents have identical
// quality (e.g. two fresh-prior players paired against a third fresh-
// prior player).
func (ts *TrueSkillRatings) BestMatches(name string, n int) []MatchCandidate {
	self, ok := ts.Ratings[name]
	if !ok {
		return nil
	}
	out := make([]MatchCandidate, 0, len(ts.Ratings)-1)
	for other, r := range ts.Ratings {
		if other == name {
			continue
		}
		out = append(out, MatchCandidate{
			Opponent:       other,
			Quality:        MatchQuality(ts.cfg, self, r),
			WinProbability: WinProbability(ts.cfg, self, r),
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
