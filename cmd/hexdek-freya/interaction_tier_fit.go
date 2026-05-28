package main

import (
	"fmt"
)

// InteractionTierFit measures how well a deck's interaction
// package (counterspells + removal + board wipes + protection)
// matches the expectation for its declared power tier. Third
// component in the curve / tutors / interaction tier-fit triad
// (PRs #728 / #729 / this one), plugged into the aggregate
// TierFitnessScore (PR #730).
//
// Tier-interaction expectations (calibrated against the 9-deck
// cEDH B5 corpus + the 87 imported precons; expressed as raw
// piece-count, not density, because the "you need N interaction
// pieces to navigate a 4-player table" framing is absolute, not
// proportional):
//
//	T5 cEDH        : ideal 17, tolerated 15-22  (real range 15-23)
//	T4 High Power  : ideal 13, tolerated 11-15
//	T3 Upgraded    : ideal 11, tolerated 10-12
//	T2 Casual      : ideal  8, tolerated 7-10
//	T1 Casual      : ideal  6, tolerated 5-8
//
// Interaction = Removal + BoardWipe + Counterspell + Protection.
// Mirrors the classifyContext.interactionCount accumulator with
// Protection added — Protection (Heroic Intervention, Veil of
// Summer, Lightning Greaves, Mother of Runes, etc.) IS
// interaction in the sense the user's task framing means: the
// deck's "stop opponents / defend my plan" suite.
//
// Fit uses the same Gaussian-falloff shape as ManaCurveTierFit:
// 1.0 at the ideal, ~0.6 at the band edges, drops below 0.4 well
// outside the band ("clear drift"). Symmetric around the ideal.
//
// Use cases:
//   - Spotting under-interacted cEDH decks: a T5-claimed deck
//     with only 6 interaction pieces (T1-casual level) reads as
//     fit ~0.0 — the deck can't survive turn-3 races.
//   - Spotting over-interacted casual decks: a T1-claimed deck
//     with 18 interaction pieces is racing harder than it
//     claims, signaling pubstomp risk.
//   - Decks-screen audit: the verdict string surfaces directly
//     to UI for "your deck has X interaction — recommended N
//     for your tier" framing.
type InteractionTierFit struct {
	// Tier is the power tier this fit was computed against.
	Tier int

	// TierLabel is the named layer (Casual / Upgraded Precon /
	// High Power / cEDH).
	TierLabel string

	// InteractionCount is the deck's total interaction-piece
	// count — sum of Removal + BoardWipe + Counterspell +
	// Protection role counts.
	InteractionCount int

	// IdealCount is the tier's center-point interaction
	// expectation.
	IdealCount int

	// MinCount / MaxCount are the tier's "tolerated" band
	// boundaries.
	MinCount int
	MaxCount int

	// Fit is the [0, 1] Gaussian-shaped score. 1.0 at the ideal
	// count; drops symmetrically as InteractionCount deviates.
	// ~0.6 at the band edges, below 0.4 outside.
	Fit float64

	// Direction labels which side of the band the deck sits on:
	// "in_band" / "too_sparse" (under-interacted, the flagship
	// under-tuned scenario) / "too_dense" (over-interacted,
	// pubstomp risk).
	Direction string

	// Verdict is the 1-2 sentence rationale for direct UI display.
	Verdict string
}

// tierInteractionExpectations holds the per-tier band table.
// Mirrors the shape of tierTutorTable / tierCurveTable for
// consistency across the three tier-fit families.
type tierInteractionExpectations struct {
	ideal int
	min   int
	max   int
	sigma float64 // Gaussian width over piece-count units
}

// Sigma values chosen so band-edge fit lands in 0.55-0.65, matching
// the calibration of ManaCurveTierFit / TutorDensityTierFit.
// Sigma chosen as (max - ideal) + 1 (with a 0.5+ headroom so
// band-edge fit lands 0.6-0.8). The T5 band is intentionally
// asymmetric (15-22 around ideal 17) because real cEDH builds
// stack interaction heavily — the right tail is the realistic
// "more interaction is still cEDH" zone. Sigma absorbs the
// larger right deviation.
var tierInteractionTable = map[int]tierInteractionExpectations{
	5: {ideal: 17, min: 15, max: 22, sigma: 6.0},
	4: {ideal: 13, min: 11, max: 15, sigma: 3.0},
	3: {ideal: 11, min: 10, max: 12, sigma: 2.0},
	2: {ideal: 8, min: 7, max: 10, sigma: 3.0},
	1: {ideal: 6, min: 5, max: 8, sigma: 3.0},
}

// BuildInteractionTierFit computes the interaction-count-vs-tier
// fit for a deck. Reads the role counts from the FreyaReport and
// computes:
//
//	interactionCount = Removal + BoardWipe + Counterspell + Protection
//
// Returns a non-nil result in all cases. Out-of-range tier or
// nil report produces a Fit=0 result with a "not computable"
// verdict.
func BuildInteractionTierFit(report *FreyaReport, tier int) *InteractionTierFit {
	out := &InteractionTierFit{
		Tier:      tier,
		TierLabel: cedhTierLabel(tier),
	}
	expect, ok := tierInteractionTable[tier]
	if !ok {
		out.TierLabel = "Unknown"
		out.Verdict = fmt.Sprintf("Tier %d out of range — interaction-tier fit not computable.", tier)
		return out
	}
	if report == nil || report.Roles == nil {
		out.Verdict = "No role data — interaction-tier fit not computable."
		return out
	}
	counts := report.Roles.RoleCounts
	out.InteractionCount = counts[RoleRemoval] + counts[RoleBoardWipe] +
		counts[RoleCounterspell] + counts[RoleProtection]
	out.IdealCount = expect.ideal
	out.MinCount = expect.min
	out.MaxCount = expect.max
	out.Fit = gaussianFitScore(float64(out.InteractionCount), float64(expect.ideal), expect.sigma)
	out.Direction = interactionDirection(out.InteractionCount, expect.min, expect.max)
	out.Verdict = buildInteractionTierVerdict(out)
	return out
}

// interactionDirection labels the deck's position relative to the
// tier band. "in_band" between min and max inclusive; "too_sparse"
// below min (under-interacted for the claimed tier — the flagship
// under-tuned scenario); "too_dense" above max (over-interacted,
// pubstomp risk).
func interactionDirection(count, minC, maxC int) string {
	switch {
	case count < minC:
		return "too_sparse"
	case count > maxC:
		return "too_dense"
	default:
		return "in_band"
	}
}

// buildInteractionTierVerdict renders the 1-2 sentence rationale.
// Format mirrors ManaCurveTierFit / TutorDensityTierFit:
//
//	"11 interaction pieces matches T3 Upgraded Precon ideal 11
//	 (tolerated 10-12); fit 1.00."
//	"6 interaction pieces too sparse for T5 cEDH ideal 17
//	 (tolerated 15-22); fit 0.00 — deck is structurally under-
//	 tuned for the tier it claims (can't survive a 4-player race)."
func buildInteractionTierVerdict(f *InteractionTierFit) string {
	if f == nil {
		return ""
	}
	header := fmt.Sprintf("%d interaction pieces", f.InteractionCount)
	bandFrag := fmt.Sprintf("%s ideal %d (tolerated %d-%d); fit %.2f",
		f.TierLabel, f.IdealCount, f.MinCount, f.MaxCount, f.Fit)
	switch f.Direction {
	case "in_band":
		return fmt.Sprintf("%s matches %s.", header, bandFrag)
	case "too_sparse":
		tail := ""
		if f.Fit < 0.4 {
			tail = " — deck is structurally under-tuned for the tier it claims (can't survive a 4-player race)."
		}
		return fmt.Sprintf("%s too sparse for %s.%s", header, bandFrag, tail)
	case "too_dense":
		tail := ""
		if f.Fit < 0.4 {
			tail = " — deck is over-tuned for the tier it claims (heavy interaction without the matching power level signals pubstomp risk)."
		}
		return fmt.Sprintf("%s too dense for %s.%s", header, bandFrag, tail)
	default:
		return fmt.Sprintf("%s vs %s.", header, bandFrag)
	}
}
