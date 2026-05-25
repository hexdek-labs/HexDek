package trueskill

import "math"

// WinrateInterval is a confidence-bounded prediction from the
// composition prior. Point matches what ExpectedWinrate would return;
// Low / High are the lower / upper 95% Wilson score bounds. Samples
// is the effective sample count the bounds are derived from. Source
// indicates which tier of the prior's fallback chain produced the
// interval — useful for tuning workflows that want to filter out
// uniform-fallback predictions.
//
// At the uniform/cold-start fallback the interval widens to [0, 1]
// (or close to it) so callers can detect "the prior has nothing to
// say" without checking Samples or Source.
type WinrateInterval struct {
	Point   float64 `json:"point"`
	Low     float64 `json:"low"`
	High    float64 `json:"high"`
	Samples int     `json:"samples"`
	Source  string  `json:"source"`
}

// WinrateInterval source labels — exposed as constants so callers can
// switch on them without string typos.
const (
	WinrateSourcePairwise          = "pairwise"
	WinrateSourceArchetypeBaseline = "archetype_baseline"
	WinrateSourceUniform           = "uniform"
)

// wilsonZ95 is the z-score for a 95% two-sided confidence interval
// under the normal distribution. Used by the Wilson score interval.
const wilsonZ95 = 1.959963984540054

// ExpectedWinrateInterval returns the prior's expected winrate
// together with a 95% Wilson confidence interval and metadata about
// how it was derived. Mirrors ExpectedWinrate's tiered fallback:
//
//  1. Pairwise: aggregate (wins, games) across all opponents in
//     the pod (skipping self-mirrors). Apply Wilson once to the
//     pooled binomial.
//  2. Archetype baseline: use the archetype's total (wins, games).
//  3. Uniform: Point = 1/podSize, Low = 0, High = 1, Samples = 0,
//     Source = "uniform".
//
// Pooling pairwise opponents is a simplification — it treats
// opponents as exchangeable, which is correct in expectation but
// slightly understates the interval width when opponent-specific
// variances differ. Acceptable for the prior's monitoring use case;
// stricter callers can fall back to per-opponent intervals via
// the (still-supported) ExpectedWinrate + Confidence API.
//
// Nil-safe: returns a uniform interval for any nil receiver or
// empty archetype.
func (cp *CompositionPrior) ExpectedWinrateInterval(deckArchetype string, pod []string) WinrateInterval {
	if cp == nil || deckArchetype == "" {
		return uniformInterval(cp.podSizeOrDefault())
	}

	// Tier 1: pool pairwise (wins, games) across opponents.
	pairWins := 0
	pairGames := 0
	for _, opp := range pod {
		if opp == "" || opp == deckArchetype {
			continue
		}
		key := archetypePair{a: deckArchetype, b: opp}
		pairGames += cp.matchupGames[key]
		pairWins += cp.matchupWins[key]
	}
	if pairGames > 0 {
		low, high := wilsonScoreInterval(pairWins, pairGames, wilsonZ95)
		return WinrateInterval{
			Point:   float64(pairWins) / float64(pairGames),
			Low:     low,
			High:    high,
			Samples: pairGames,
			Source:  WinrateSourcePairwise,
		}
	}

	// Tier 2: archetype baseline.
	if g := cp.archGames[deckArchetype]; g > 0 {
		w := cp.archWins[deckArchetype]
		low, high := wilsonScoreInterval(w, g, wilsonZ95)
		return WinrateInterval{
			Point:   float64(w) / float64(g),
			Low:     low,
			High:    high,
			Samples: g,
			Source:  WinrateSourceArchetypeBaseline,
		}
	}

	// Tier 3: cold start.
	return uniformInterval(cp.podSizeOrDefault())
}

// wilsonScoreInterval returns the (low, high) bounds of the Wilson
// score interval at the given z-score for k successes in n trials.
// More robust than the normal approximation for small n or extreme
// p̂ — degrades gracefully toward [0, 1] as n → 0 instead of producing
// undefined or negative bounds.
//
// Formula (Wilson 1927):
//
//	center = (p̂ + z²/(2n)) / (1 + z²/n)
//	margin = (z·√(p̂(1-p̂)/n + z²/(4n²))) / (1 + z²/n)
//	low    = max(0, center - margin)
//	high   = min(1, center + margin)
//
// For n == 0, returns (0, 1) — the trivially-wide interval that
// signals "no information."
func wilsonScoreInterval(k, n int, z float64) (low, high float64) {
	if n <= 0 {
		return 0, 1
	}
	nf := float64(n)
	pHat := float64(k) / nf
	z2 := z * z
	denom := 1 + z2/nf
	center := (pHat + z2/(2*nf)) / denom
	margin := z * math.Sqrt(pHat*(1-pHat)/nf+z2/(4*nf*nf)) / denom
	low = center - margin
	high = center + margin
	if low < 0 {
		low = 0
	}
	if high > 1 {
		high = 1
	}
	return
}

// uniformInterval is the cold-start fallback: the point estimate is
// uniform 1/podSize and the bounds span [0, 1] to express maximal
// uncertainty.
func uniformInterval(podSize int) WinrateInterval {
	if podSize < 2 {
		podSize = 4
	}
	return WinrateInterval{
		Point:   1.0 / float64(podSize),
		Low:     0,
		High:    1,
		Samples: 0,
		Source:  WinrateSourceUniform,
	}
}
