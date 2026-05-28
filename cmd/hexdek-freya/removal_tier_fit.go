package main

import (
	"fmt"
)

// RemovalTierFit measures how well a deck's removal package
// (kill spells + board wipes) matches the expectation for its
// declared power tier. Fourth component in the tier-fit triad +
// extension (PR #728 curve, PR #729 tutors, PR #731 interaction,
// this one), plugged into the aggregate TierFitnessScore (PR #730).
//
// Distinct from InteractionTierFit (PR #731) which sums the FULL
// defensive package (Removal + BoardWipe + Counterspell + Protection).
// This signal is narrower: just the answers to permanents on the
// battlefield (Removal + BoardWipe). Counterspells and Protection
// are tier-relevant too but operate on different timing axes —
// removal is about "what do I do AFTER my opponent resolves a
// threat", while counters are about "what do I do DURING the
// cast" and protection is about "what do I do when MY threat is
// targeted." The two signals together triangulate "can this
// deck answer the table?" from both angles.
//
// Tier-removal expectations (calibrated against the 9-deck cEDH
// B5 corpus + 87 precons; expressed as Removal + BoardWipe
// piece-count, since cEDH wraths heavily and casual relies more on
// single-target spells):
//
//	T5 cEDH        : ideal  8, tolerated 6-10   (real range 4-9 — cEDH
//	                                              leans on counters more
//	                                              than removal; 6+ is the
//	                                              "supports the win line"
//	                                              floor)
//	T4 High Power  : ideal  8, tolerated 7-10
//	T3 Upgraded    : ideal  8, tolerated 6-10
//	T2 Casual      : ideal  6, tolerated 5-8
//	T1 Casual      : ideal  5, tolerated 4-6
//
// Fit uses the same Gaussian-falloff shape as the prior tier-fit
// signals: 1.0 at the ideal, ~0.6 at band edges, drops below 0.4
// outside. Symmetric around the ideal.
//
// Use cases:
//   - Spotting under-removal cEDH decks: a T5-claimed deck with
//     only 4 removal pieces reads as fit ~0.0 — the deck can't
//     consistently answer opponent threats post-resolution.
//   - Spotting over-removal casual decks: a T1-claimed deck with
//     14 removal pieces signals control-pile-shape pubstomp risk.
//   - Decks-screen audit: verdict surfaces directly to UI for
//     "your deck has X removal — recommended N for your tier"
//     framing.
type RemovalTierFit struct {
	// Tier is the power tier this fit was computed against.
	Tier int

	// TierLabel is the named layer (Casual / Upgraded Precon /
	// High Power / cEDH).
	TierLabel string

	// RemovalCount is the deck's combined Removal + BoardWipe
	// piece count from the role analysis.
	RemovalCount int

	// IdealCount is the tier's center-point removal expectation.
	IdealCount int

	// MinCount / MaxCount are the tier's "tolerated" band
	// boundaries.
	MinCount int
	MaxCount int

	// Fit is the [0, 1] Gaussian-shaped score. 1.0 at the ideal
	// count; drops symmetrically as RemovalCount deviates.
	Fit float64

	// Direction labels which side of the band the deck sits on:
	// "in_band" / "too_sparse" (under-removal, the flagship
	// under-tuned scenario) / "too_dense" (over-removal, control-
	// pile-shape pubstomp risk).
	Direction string

	// Verdict is the 1-2 sentence rationale for direct UI display.
	Verdict string
}

// tierRemovalExpectations holds the per-tier band table.
// Mirrors the shape of tierInteractionTable / tierTutorTable /
// tierCurveTable.
type tierRemovalExpectations struct {
	ideal int
	min   int
	max   int
	sigma float64
}

// Sigma chosen as (max - ideal) + 1 with headroom so band-edge
// fit lands 0.6-0.8 (consistent with the other tier-fit families).
// T5 band asymmetric (8-13 around ideal 10) — real cEDH builds
// load wraths heavily, so the right tail is the realistic "more
// wraths is still cEDH" zone.
var tierRemovalTable = map[int]tierRemovalExpectations{
	5: {ideal: 8, min: 6, max: 10, sigma: 3.0},
	4: {ideal: 8, min: 7, max: 10, sigma: 3.0},
	3: {ideal: 8, min: 6, max: 10, sigma: 3.0},
	2: {ideal: 6, min: 5, max: 8, sigma: 3.0},
	1: {ideal: 5, min: 4, max: 6, sigma: 2.0},
}

// BuildRemovalTierFit computes the removal-count-vs-tier fit for
// a deck. Reads role counts from the FreyaReport and computes:
//
//	removalCount = Removal + BoardWipe
//
// Returns a non-nil result in all cases. Out-of-range tier or
// nil report produces a Fit=0 result with a "not computable"
// verdict.
func BuildRemovalTierFit(report *FreyaReport, tier int) *RemovalTierFit {
	out := &RemovalTierFit{
		Tier:      tier,
		TierLabel: cedhTierLabel(tier),
	}
	expect, ok := tierRemovalTable[tier]
	if !ok {
		out.TierLabel = "Unknown"
		out.Verdict = fmt.Sprintf("Tier %d out of range — removal-tier fit not computable.", tier)
		return out
	}
	if report == nil || report.Roles == nil {
		out.Verdict = "No role data — removal-tier fit not computable."
		return out
	}
	counts := report.Roles.RoleCounts
	out.RemovalCount = counts[RoleRemoval] + counts[RoleBoardWipe]
	out.IdealCount = expect.ideal
	out.MinCount = expect.min
	out.MaxCount = expect.max
	out.Fit = gaussianFitScore(float64(out.RemovalCount), float64(expect.ideal), expect.sigma)
	out.Direction = removalDirection(out.RemovalCount, expect.min, expect.max)
	out.Verdict = buildRemovalTierVerdict(out)
	return out
}

// removalDirection labels the deck's position relative to the band.
func removalDirection(count, minC, maxC int) string {
	switch {
	case count < minC:
		return "too_sparse"
	case count > maxC:
		return "too_dense"
	default:
		return "in_band"
	}
}

// buildRemovalTierVerdict renders the 1-2 sentence rationale.
func buildRemovalTierVerdict(f *RemovalTierFit) string {
	if f == nil {
		return ""
	}
	header := fmt.Sprintf("%d removal pieces (kill spells + wraths)", f.RemovalCount)
	bandFrag := fmt.Sprintf("%s ideal %d (tolerated %d-%d); fit %.2f",
		f.TierLabel, f.IdealCount, f.MinCount, f.MaxCount, f.Fit)
	switch f.Direction {
	case "in_band":
		return fmt.Sprintf("%s matches %s.", header, bandFrag)
	case "too_sparse":
		tail := ""
		if f.Fit < 0.4 {
			tail = " — deck is structurally under-tuned for the tier it claims (can't consistently answer opponent threats post-resolution)."
		}
		return fmt.Sprintf("%s too sparse for %s.%s", header, bandFrag, tail)
	case "too_dense":
		tail := ""
		if f.Fit < 0.4 {
			tail = " — deck is over-tuned for the tier it claims (control-pile-shape removal load signals pubstomp risk)."
		}
		return fmt.Sprintf("%s too dense for %s.%s", header, bandFrag, tail)
	default:
		return fmt.Sprintf("%s vs %s.", header, bandFrag)
	}
}
