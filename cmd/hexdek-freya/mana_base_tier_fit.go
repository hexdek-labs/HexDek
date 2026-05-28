package main

import (
	"fmt"
)

// ManaBaseTierFit measures how well a deck's mana-base grade
// matches the expectation for its declared power tier. Fifth
// component in the tier-fit family (curve PR #728, tutors PR
// #729, interaction PR #731, removal PR #732, this one).
// Plugged into the aggregate TierFitnessScore (PR #730).
//
// Tier-grade expectations (calibrated against the 9-deck cEDH B5
// corpus + 87 imported precons):
//
//	T5 cEDH        : expect A      (fetches, original duals,
//	                                fast lands, untapped sources)
//	T4 High Power  : expect B-A    (shocks + 1-2 fetches, mostly
//	                                untapped)
//	T3 Upgraded    : expect B-C    (some shocks, few taplands)
//	T2 Casual      : expect C-D    (mostly basics, several
//	                                taplands)
//	T1 Casual      : expect D-F    (precon-level: basics + tap
//	                                duals only)
//
// Grade-to-score mapping: A=5, B=4, C=3, D=2, F=1. Empty grade
// (no data) skips the fit computation and returns 0.
//
// Fit uses the same Gaussian-falloff shape as the prior tier-fit
// signals: 1.0 at the ideal grade score; ~0.6 at one grade off;
// drops below 0.4 at two grades off. Symmetric around the ideal.
//
// Use cases:
//   - Spotting under-tuned cEDH decks: a T5-claimed deck with a
//     D-grade mana base (taplands + basics, no fetches) reads as
//     fit ~0.0 — the deck can't execute on a turn-3 race.
//   - Spotting over-built casual decks: a T1-claimed deck with
//     an A-grade mana base (full fetch package) is signaling
//     pubstomp shape — the player invested heavily in
//     consistency at a power level that doesn't need it.
//   - Decks-screen audit: surfaces directly to UI for "your deck
//     has X-grade mana, recommended Y-grade for your tier"
//     framing.
type ManaBaseTierFit struct {
	// Tier is the power tier this fit was computed against.
	Tier int

	// TierLabel is the named layer (Casual / Upgraded Precon /
	// High Power / cEDH).
	TierLabel string

	// ActualGrade is the deck's mana-base grade (A/B/C/D/F),
	// echoed from DeckProfile.ManaBaseGrade.
	ActualGrade string

	// ActualGradeScore is the numeric mapping of ActualGrade
	// (A=5, B=4, C=3, D=2, F=1, ""=0).
	ActualGradeScore int

	// IdealGrade is the tier's center-point grade expectation
	// (single letter).
	IdealGrade string

	// IdealGradeScore is the numeric mapping of IdealGrade.
	IdealGradeScore int

	// MinGrade / MaxGrade are the tier's "tolerated" band
	// boundaries — Grade strings.
	MinGrade string
	MaxGrade string

	// Fit is the [0, 1] Gaussian-shaped score. 1.0 at the ideal
	// grade score; drops symmetrically as ActualGradeScore
	// deviates.
	Fit float64

	// Direction labels which side of the band the deck sits on:
	// "in_band" / "too_weak" (mana base under-built for the tier
	// — the flagship under-tuned scenario) / "too_strong" (mana
	// base over-built for the tier, pubstomp signal).
	Direction string

	// Verdict is the 1-2 sentence rationale for direct UI display.
	Verdict string
}

// tierManaBaseExpectations holds the per-tier band table.
type tierManaBaseExpectations struct {
	idealScore int // A=5, B=4, C=3, D=2, F=1
	minScore   int
	maxScore   int
	sigma      float64
}

// Tier bands centered on the canonical "you need this kind of
// mana base to support this kind of deck" intuition. Sigma at
// 1.5 gives band-edge fit ~0.8 at 1 grade off (consistent with
// the other tier-fit families' band-edge calibration).
var tierManaBaseTable = map[int]tierManaBaseExpectations{
	5: {idealScore: 5, minScore: 4, maxScore: 5, sigma: 2.0}, // A required, B tolerated
	4: {idealScore: 4, minScore: 3, maxScore: 5, sigma: 2.0}, // B ideal, C-A tolerated
	3: {idealScore: 3, minScore: 2, maxScore: 4, sigma: 2.0}, // C ideal, D-B tolerated
	2: {idealScore: 2, minScore: 1, maxScore: 3, sigma: 2.0}, // D ideal, F-C tolerated
	1: {idealScore: 2, minScore: 1, maxScore: 3, sigma: 2.0}, // D ideal, F-C tolerated
}

// gradeScoreMap maps the letter grade to its numeric value. A
// returns the highest (5); F the lowest (1). Empty / unknown
// returns 0 — caller treats as "no data".
var gradeScoreMap = map[string]int{
	"A": 5,
	"B": 4,
	"C": 3,
	"D": 2,
	"F": 1,
}

// scoreGradeMap is the inverse mapping. Used to render the ideal
// / min / max as letter grades in the verdict.
var scoreGradeMap = map[int]string{
	5: "A",
	4: "B",
	3: "C",
	2: "D",
	1: "F",
}

// BuildManaBaseTierFit computes the mana-base-grade-vs-tier fit.
// Inputs:
//   - grade: dp.ManaBaseGrade (A/B/C/D/F or empty)
//   - tier: 1-5 from ClassifyCEDHPowerTier
//
// Returns a non-nil result in all cases. Out-of-range tier or
// empty grade produces Fit=0 with a "not computable" verdict.
func BuildManaBaseTierFit(grade string, tier int) *ManaBaseTierFit {
	out := &ManaBaseTierFit{
		Tier:        tier,
		TierLabel:   cedhTierLabel(tier),
		ActualGrade: grade,
	}
	expect, ok := tierManaBaseTable[tier]
	if !ok {
		out.TierLabel = "Unknown"
		out.Verdict = fmt.Sprintf("Tier %d out of range — mana-base-tier fit not computable.", tier)
		return out
	}
	out.IdealGrade = scoreGradeMap[expect.idealScore]
	out.IdealGradeScore = expect.idealScore
	out.MinGrade = scoreGradeMap[expect.minScore]
	out.MaxGrade = scoreGradeMap[expect.maxScore]

	actualScore, knownGrade := gradeScoreMap[grade]
	if !knownGrade {
		out.Verdict = "No mana-base grade data — fit not computable."
		return out
	}
	out.ActualGradeScore = actualScore
	out.Fit = gaussianFitScore(float64(actualScore), float64(expect.idealScore), expect.sigma)
	out.Direction = manaBaseDirection(actualScore, expect.minScore, expect.maxScore)
	out.Verdict = buildManaBaseTierVerdict(out)
	return out
}

// manaBaseDirection labels the deck's grade-score relative to
// the tier band. "in_band" between min and max inclusive;
// "too_weak" below min (mana base under-built — the flagship
// scenario for under-tuned decks); "too_strong" above max
// (pubstomp shape).
func manaBaseDirection(score, minS, maxS int) string {
	switch {
	case score < minS:
		return "too_weak"
	case score > maxS:
		return "too_strong"
	default:
		return "in_band"
	}
}

// buildManaBaseTierVerdict renders the 1-2 sentence rationale.
func buildManaBaseTierVerdict(f *ManaBaseTierFit) string {
	if f == nil {
		return ""
	}
	header := fmt.Sprintf("Mana base grade %s", f.ActualGrade)
	bandFrag := fmt.Sprintf("%s ideal %s (tolerated %s-%s); fit %.2f",
		f.TierLabel, f.IdealGrade, f.MinGrade, f.MaxGrade, f.Fit)
	switch f.Direction {
	case "in_band":
		return fmt.Sprintf("%s matches %s.", header, bandFrag)
	case "too_weak":
		tail := ""
		if f.Fit < 0.4 {
			tail = " — deck is structurally under-tuned for the tier it claims (taplands and basics can't support a turn-3 race)."
		}
		return fmt.Sprintf("%s too weak for %s.%s", header, bandFrag, tail)
	case "too_strong":
		tail := ""
		if f.Fit < 0.4 {
			tail = " — deck is over-built for the tier it claims (fetch/dual investment signals pubstomp shape at a power level that doesn't need it)."
		}
		return fmt.Sprintf("%s too strong for %s.%s", header, bandFrag, tail)
	default:
		return fmt.Sprintf("%s vs %s.", header, bandFrag)
	}
}
