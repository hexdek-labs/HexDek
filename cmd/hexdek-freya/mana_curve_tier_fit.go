package main

import (
	"fmt"
	"math"
)

// ManaCurveTierFit is the rollup measuring how well a deck's average
// CMC matches the curve expectation for its power tier. Used to
// spot under-tuned decks (cEDH-tier decks with curves that are too
// expensive to actually race) and over-tuned decks (casual decks
// running cEDH-lean curves without the supporting interaction).
//
// Tier-CMC expectations (calibrated against the WotC bracket
// framework + cEDH community guidance):
//
//	T5 cEDH        : ideal 2.0, tolerated 2.0-2.5
//	T4 High Power  : ideal 2.5, tolerated 2.3-2.9
//	T3 Upgraded    : ideal 3.2, tolerated 3.0-3.5
//	T2 Casual      : ideal 3.6, tolerated 3.4-3.8
//	T1 Casual      : ideal 3.8, tolerated 3.5-4.2
//
// Fit is computed as a Gaussian centered on the ideal CMC, with
// the tier-specific tolerance defining the sigma. Fit=1.0 at the
// ideal; falls off as the deviation grows. The "tolerated" band
// edges sit ~1 sigma from the ideal so fit at the band edges is
// ~0.60-0.65 ("still in tolerance, but starting to drift"); fit
// drops below 0.4 well outside the band ("clear drift").
//
// Useful for the Decks-screen audit ("your B5-claimed deck has a
// 3.4 average CMC — that's a B3 curve") and for the hat's
// scaling-when-to-mulligan logic (low-fit decks need more
// aggressive openers).
type ManaCurveTierFit struct {
	// Tier is the power tier this fit was computed against — input
	// to BuildManaCurveTierFit, preserved for consumer rendering.
	Tier int

	// TierLabel is the named layer corresponding to Tier (Casual /
	// Upgraded Precon / High Power / cEDH).
	TierLabel string

	// AvgCMC is the deck's average converted mana cost across
	// nonland cards — read directly from the FreyaReport / DeckProfile.
	AvgCMC float64

	// IdealCMC is the tier's center-point CMC expectation.
	IdealCMC float64

	// MinCMC / MaxCMC are the tier's "tolerated" band boundaries.
	MinCMC float64
	MaxCMC float64

	// Fit is the [0, 1] Gaussian-shaped score. 1.0 at IdealCMC;
	// drops symmetrically as AvgCMC deviates. Fit ~0.85 at the
	// band edges; <0.6 well outside.
	Fit float64

	// Direction describes which side of the band the deck sits on
	// when out of band: "in_band" / "too_expensive" (above MaxCMC)
	// / "too_lean" (below MinCMC).
	Direction string

	// Verdict is a 1-2 sentence rationale rendered for direct UI
	// display. Format:
	//
	//   "Average CMC 3.45 matches T3 Upgraded Precon ideal 3.2
	//    (tolerated 3.0-3.5); fit 0.93."
	//   "Average CMC 3.40 too expensive for T5 cEDH ideal 2.0
	//    (tolerated 2.0-2.5); fit 0.27 — deck is structurally
	//    under-tuned for the tier it claims."
	Verdict string
}

// tierCurveExpectations holds the per-tier curve target table.
// Tier 1-5 map to T1 (Casual) through T5 (cEDH); tier 0 is treated
// as unknown and the function returns a zero-fit result.
type tierCurveExpectations struct {
	ideal float64
	min   float64
	max   float64
	// sigma controls the Gaussian falloff width — derived from the
	// (max-min)/2 half-width with a calibration multiplier. Stored
	// pre-computed for clarity.
	sigma float64
}

// Tier band calibration (PR #725) tuned against the 9-deck cEDH B5
// corpus + the 87 imported precons. T5 range widened to 1.7-2.5
// because real cEDH builds run 1.86-2.31 (kinnan turbo at 1.86,
// lumra at 2.31 — both legitimate B5 / T5 by the bracket
// classifier). Sigmas chosen so the Gaussian gives fit >=0.85 at
// the ideal and fit ~0.65 at the band edges — the boundary IS the
// "tolerated" zone, fit drops further into "drift" territory beyond.
var tierCurveTable = map[int]tierCurveExpectations{
	5: {ideal: 2.0, min: 1.7, max: 2.5, sigma: 0.55},
	4: {ideal: 2.5, min: 2.3, max: 2.9, sigma: 0.45},
	3: {ideal: 3.2, min: 3.0, max: 3.5, sigma: 0.45},
	2: {ideal: 3.6, min: 3.4, max: 3.8, sigma: 0.45},
	1: {ideal: 3.8, min: 3.5, max: 4.2, sigma: 0.55},
}

// BuildManaCurveTierFit computes the curve-vs-tier fit for a deck
// at a given tier. Pass tier from ClassifyCEDHPowerTier's output
// (or any other tier source). Returns a non-nil result in all
// cases; out-of-range tier values produce a zero-fit result with
// TierLabel="Unknown".
//
// AvgCMC of 0 is treated as "no data" — fit returns 0 with a
// "no CMC data" verdict rather than producing a spurious fit
// reading.
func BuildManaCurveTierFit(avgCMC float64, tier int) *ManaCurveTierFit {
	out := &ManaCurveTierFit{
		Tier:      tier,
		TierLabel: cedhTierLabel(tier),
		AvgCMC:    avgCMC,
	}
	expect, ok := tierCurveTable[tier]
	if !ok {
		out.TierLabel = "Unknown"
		out.Verdict = fmt.Sprintf("Tier %d out of range — curve-vs-tier fit not computable.", tier)
		return out
	}
	out.IdealCMC = expect.ideal
	out.MinCMC = expect.min
	out.MaxCMC = expect.max
	if avgCMC <= 0 {
		out.Verdict = "No CMC data available — curve-vs-tier fit not computable."
		return out
	}
	out.Fit = gaussianFitScore(avgCMC, expect.ideal, expect.sigma)
	out.Direction = curveDirection(avgCMC, expect.min, expect.max)
	out.Verdict = buildCurveTierVerdict(out)
	return out
}

// gaussianFitScore returns exp(-(x - mu)^2 / (2 * sigma^2)) — the
// canonical Gaussian falloff centered on `mu` with width `sigma`.
// Clamped to [0, 1] defensively (gaussian output is always in that
// range mathematically; the clamp guards against numerical edge
// cases on extreme inputs).
func gaussianFitScore(x, mu, sigma float64) float64 {
	if sigma <= 0 {
		return 0
	}
	diff := x - mu
	score := math.Exp(-(diff * diff) / (2 * sigma * sigma))
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// curveDirection labels which side of the tier band the deck sits
// on. "in_band" when AvgCMC is between min and max inclusive;
// "too_expensive" above max; "too_lean" below min.
func curveDirection(avgCMC, minCMC, maxCMC float64) string {
	switch {
	case avgCMC < minCMC:
		return "too_lean"
	case avgCMC > maxCMC:
		return "too_expensive"
	default:
		return "in_band"
	}
}

// buildCurveTierVerdict renders the 1-2 sentence rationale.
func buildCurveTierVerdict(f *ManaCurveTierFit) string {
	if f == nil {
		return ""
	}
	header := fmt.Sprintf("Average CMC %.2f", f.AvgCMC)
	bandFrag := fmt.Sprintf("%s ideal %.1f (tolerated %.1f-%.1f); fit %.2f",
		f.TierLabel, f.IdealCMC, f.MinCMC, f.MaxCMC, f.Fit)
	switch f.Direction {
	case "in_band":
		return fmt.Sprintf("%s matches %s.", header, bandFrag)
	case "too_expensive":
		tail := ""
		if f.Fit < 0.6 {
			tail = " — deck is structurally under-tuned for the tier it claims (curve too slow)."
		}
		return fmt.Sprintf("%s too expensive for %s.%s", header, bandFrag, tail)
	case "too_lean":
		tail := ""
		if f.Fit < 0.6 {
			tail = " — deck is structurally over-tuned for the tier it claims (curve cEDH-shaped without the rest of the package)."
		}
		return fmt.Sprintf("%s too lean for %s.%s", header, bandFrag, tail)
	default:
		return fmt.Sprintf("%s vs %s.", header, bandFrag)
	}
}
