package main

import (
	"fmt"
)

// TutorDensityTierFit measures how well a deck's nonland tutor
// density matches the consistency expectation for its declared
// power tier. Companion to ManaCurveTierFit (PR #725) — together
// they triangulate "under-tuned for tier" vs "over-tuned for tier"
// at the two strongest curve+consistency axes.
//
// Tutor-density expectations (calibrated against the 9-deck cEDH
// B5 corpus + the 87 imported precons; expressed as fraction of
// the deck's total card count, not nonlands-only, so the numbers
// compare directly to what report.NonLandTutorCount / TotalCards
// produces):
//
//	T5 cEDH        : ideal 12%, tolerated 10-18%  (real range 9-18%)
//	T4 High Power  : ideal  7%, tolerated 5-9%
//	T3 Upgraded    : ideal  4%, tolerated 3-6%
//	T2 Casual      : ideal  2%, tolerated 1-3%
//	T1 Casual      : ideal  1%, tolerated 0-2%
//
// Fit uses the same Gaussian-falloff shape as ManaCurveTierFit:
// 1.0 at the ideal, ~0.6 at the band edges, drops below 0.4 well
// outside the band ("clear drift"). Symmetric around the ideal.
//
// Use cases:
//   - Spotting under-tuned cEDH decks: a T5-claimed deck with 4%
//     tutors (1-2 tutors total) reads as fit ~0.05 — the deck
//     can't reliably assemble its win line under pressure.
//   - Spotting over-tuned casual decks: a T1-claimed deck with
//     15% tutors is racing harder than it claims, signaling
//     pubstomp risk.
//   - Decks-screen audit: the verdict string surfaces directly
//     to UI for "your deck is at X% tutors — recommended N% for
//     your tier" framing.
type TutorDensityTierFit struct {
	// Tier is the power tier this fit was computed against.
	// Echoed from input for consumer rendering.
	Tier int

	// TierLabel is the named layer (Casual / Upgraded Precon /
	// High Power / cEDH).
	TierLabel string

	// TutorCount is the deck's nonland tutor count (from
	// report.NonLandTutorCount).
	TutorCount int

	// TotalCards is the deck's total card count (denominator
	// for the density ratio).
	TotalCards int

	// ActualDensity is the deck's NonLandTutorCount / TotalCards
	// ratio in [0, 1]. Reads as a fraction (0.12 = 12%).
	ActualDensity float64

	// IdealDensity is the tier's center-point density expectation.
	IdealDensity float64

	// MinDensity / MaxDensity are the tier's "tolerated" band
	// boundaries (also in [0, 1]).
	MinDensity float64
	MaxDensity float64

	// Fit is the [0, 1] Gaussian-shaped score. 1.0 at the
	// ideal density; drops symmetrically as ActualDensity
	// deviates. ~0.6 at the band edges, below 0.4 outside.
	Fit float64

	// Direction labels which side of the band the deck sits on
	// when out of band: "in_band" / "too_dense" (above MaxDensity,
	// over-tuned for the claimed tier) / "too_sparse" (below
	// MinDensity, under-tuned for the claimed tier).
	Direction string

	// Verdict is the 1-2 sentence rationale for direct UI display.
	Verdict string
}

// tierTutorExpectations: per-tier curve target. Stored as a struct
// matching the ManaCurveTierFit table shape for consistency.
type tierTutorExpectations struct {
	ideal float64 // fraction [0, 1]
	min   float64
	max   float64
	sigma float64
}

// Tier-density bands calibrated against real corpus data:
//   - cEDH B5 decks routinely run 9-18 tutors out of 99 cards
//     (9-18%); the 9% lower edge (kinnan / good_luck builds)
//     is real cEDH that runs leaner on consistency
//   - The 87-precon corpus shows 0-3 tutors typical (0-3%) at
//     T1-T2 and 3-5 at T3 (upgraded precons with a Demonic Tutor
//     or two)
//
// Sigma values chosen so band-edge fit is ~0.6 (consistent with
// ManaCurveTierFit's calibration).
var tierTutorTable = map[int]tierTutorExpectations{
	5: {ideal: 0.12, min: 0.10, max: 0.18, sigma: 0.065},
	4: {ideal: 0.07, min: 0.05, max: 0.09, sigma: 0.025},
	3: {ideal: 0.04, min: 0.03, max: 0.06, sigma: 0.02},
	2: {ideal: 0.02, min: 0.01, max: 0.03, sigma: 0.015},
	1: {ideal: 0.01, min: 0.00, max: 0.02, sigma: 0.02},
}

// BuildTutorDensityTierFit computes the tutor-density-vs-tier fit
// for a deck. Inputs:
//   - tutorCount: report.NonLandTutorCount (only NONLAND tutors —
//     land tutors are ramp, not consistency engines)
//   - totalCards: report.TotalCards (denominator for the density
//     ratio)
//   - tier: 1-5 from ClassifyCEDHPowerTier
//
// Returns a non-nil result in all cases. Out-of-range tier or
// zero TotalCards produces a Fit=0 result with a "not computable"
// verdict — defensive callers don't need to nil-check.
func BuildTutorDensityTierFit(tutorCount, totalCards, tier int) *TutorDensityTierFit {
	out := &TutorDensityTierFit{
		Tier:       tier,
		TierLabel:  cedhTierLabel(tier),
		TutorCount: tutorCount,
		TotalCards: totalCards,
	}
	expect, ok := tierTutorTable[tier]
	if !ok {
		out.TierLabel = "Unknown"
		out.Verdict = fmt.Sprintf("Tier %d out of range — tutor-density fit not computable.", tier)
		return out
	}
	if totalCards <= 0 {
		out.Verdict = "No card-count data — tutor-density fit not computable."
		return out
	}
	out.ActualDensity = float64(tutorCount) / float64(totalCards)
	out.IdealDensity = expect.ideal
	out.MinDensity = expect.min
	out.MaxDensity = expect.max
	out.Fit = gaussianFitScore(out.ActualDensity, expect.ideal, expect.sigma)
	out.Direction = tutorDirection(out.ActualDensity, expect.min, expect.max)
	out.Verdict = buildTutorTierVerdict(out)
	return out
}

// tutorDirection labels the deck's position relative to the tier band.
// "in_band" when ActualDensity is between min and max inclusive;
// "too_dense" above max (over-tuned for the claimed tier);
// "too_sparse" below min (under-tuned for the claimed tier — the
// flagship spec scenario).
func tutorDirection(density, minD, maxD float64) string {
	switch {
	case density < minD:
		return "too_sparse"
	case density > maxD:
		return "too_dense"
	default:
		return "in_band"
	}
}

// buildTutorTierVerdict renders the 1-2 sentence rationale. Format:
//
//	"4 tutors / 99 cards (4.0%) matches T3 Upgraded Precon ideal
//	 4.0% (tolerated 3.0-6.0%); fit 1.00."
//	"2 tutors / 99 cards (2.0%) too sparse for T5 cEDH ideal 12.0%
//	 (tolerated 10.0-18.0%); fit 0.05 — deck is structurally
//	 under-tuned for the tier it claims (consistency too low to
//	 reliably assemble win line)."
func buildTutorTierVerdict(f *TutorDensityTierFit) string {
	if f == nil {
		return ""
	}
	header := fmt.Sprintf("%d tutors / %d cards (%.1f%%)",
		f.TutorCount, f.TotalCards, f.ActualDensity*100)
	bandFrag := fmt.Sprintf("%s ideal %.1f%% (tolerated %.1f-%.1f%%); fit %.2f",
		f.TierLabel, f.IdealDensity*100, f.MinDensity*100, f.MaxDensity*100, f.Fit)
	switch f.Direction {
	case "in_band":
		return fmt.Sprintf("%s matches %s.", header, bandFrag)
	case "too_sparse":
		tail := ""
		if f.Fit < 0.4 {
			tail = " — deck is structurally under-tuned for the tier it claims (consistency too low to reliably assemble win line)."
		}
		return fmt.Sprintf("%s too sparse for %s.%s", header, bandFrag, tail)
	case "too_dense":
		tail := ""
		if f.Fit < 0.4 {
			tail = " — deck is over-tuned for the tier it claims (cEDH-shape consistency without the matching power level signals pubstomp risk)."
		}
		return fmt.Sprintf("%s too dense for %s.%s", header, bandFrag, tail)
	default:
		return fmt.Sprintf("%s vs %s.", header, bandFrag)
	}
}
