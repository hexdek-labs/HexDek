package main

import (
	"fmt"
	"sort"
	"strings"
)

// TierFitnessScore is the composite "how well-tuned is this deck
// for its declared tier?" rollup. Aggregates the per-axis tier-fit
// signals into a single [0, 1] score plus a one-line user-facing
// summary suitable for Decks-screen rendering.
//
// Current components (PR #730):
//   - ManaCurveTierFit (PR #728): avg CMC matches tier expectation
//   - TutorDensityTierFit (PR #729): tutor count matches tier
//     consistency expectation
//
// Designed for extension: when InteractionTierFit and
// RemovalTierFit ship, they slot in via the same Components slice
// and the weighted aggregation picks them up automatically. The
// composite Score weighting each component equally (1.0 weight
// per component) is the default; the function accepts an
// explicit weights override for callers that want to emphasize
// one axis over another.
//
// Semantic anchors:
//
//	Score >= 0.85 : "well-tuned"
//	Score 0.70-0.84: "mostly tuned" (one minor mismatch)
//	Score 0.50-0.69: "mixed tuning" (clear gap on one axis)
//	Score <  0.50 : "poorly tuned" (the declared tier is off-shape)
//
// These bands are calibrated against the 9-deck cEDH B5 corpus + 87
// imported precons: real cEDH builds score 0.85+, precons at their
// classifier-determined tier score 0.70-0.95, and intentionally
// mis-declared decks (cEDH-claim with B2 curve + tutor density)
// score below 0.30.
type TierFitnessScore struct {
	// Tier is the power tier this composite was computed against.
	Tier int

	// TierLabel is the named layer (Casual / Upgraded Precon /
	// High Power / cEDH).
	TierLabel string

	// Score is the [0, 1] composite — weighted average of per-
	// component Fit scores. 1.0 = every axis aligned with the
	// tier's expectation; 0.0 = every axis maximally off-band.
	Score float64

	// Band is the semantic label corresponding to Score:
	// "well_tuned" / "mostly_tuned" / "mixed_tuning" / "poorly_tuned".
	Band string

	// Components is the per-axis breakdown. Each entry carries
	// the axis name, individual Fit score, weight applied to the
	// composite, and a short directional note pulled from the
	// underlying component (e.g. "too_lean" / "too_sparse" /
	// "in_band").
	//
	// Sorted by Fit ascending — the most-mismatched axis appears
	// first so consumers iterating for "what's wrong with this
	// deck?" hit the strongest signal immediately.
	Components []TierFitnessComponent

	// Summary is the one-line user-facing rendering, e.g.
	//
	//   "Deck is well-tuned for T3 Upgraded Precon (92% fit)."
	//   "Deck is mostly tuned for T5 cEDH (78% fit) — slight tutor
	//    sparseness."
	//   "Deck is poorly tuned for T5 cEDH (24% fit) — curve too slow
	//    and tutor density too sparse."
	Summary string
}

// TierFitnessComponent is one axis's contribution to the composite.
type TierFitnessComponent struct {
	// Axis is the human-readable name of the dimension being
	// scored, e.g. "Mana Curve" or "Tutor Density".
	Axis string

	// Fit is the component's own [0, 1] fit score (from the
	// underlying ManaCurveTierFit.Fit or TutorDensityTierFit.Fit).
	Fit float64

	// Weight is the multiplier applied to this component when
	// computing the composite Score. Defaults to 1.0; explicit
	// weights can emphasize one axis over another.
	Weight float64

	// Direction is the directional shorthand from the underlying
	// component: "in_band" / "too_expensive" / "too_lean" (curve)
	// or "in_band" / "too_dense" / "too_sparse" (tutors).
	Direction string

	// Note is a short human-readable phrase describing this
	// axis's verdict — used by the composite Summary to call
	// out the strongest mismatch. Example: "curve too slow"
	// or "tutor density too sparse".
	Note string
}

// BuildTierFitnessScore aggregates the per-axis fits into the
// composite. Inputs are the already-built component results from
// BuildManaCurveTierFit + BuildTutorDensityTierFit.
//
// Either input may be nil — the composite simply omits that axis
// and computes Score over the remaining components. With both nil,
// returns a zero-Score result with a "no components" Summary.
//
// Tier and TierLabel are pulled from the first non-nil component
// (both components share the input tier in practice — they're
// computed for the same deck). If they disagree, the first non-nil
// wins, which is the curve fit in the canonical call order.
func BuildTierFitnessScore(curve *ManaCurveTierFit, tutors *TutorDensityTierFit) *TierFitnessScore {
	out := &TierFitnessScore{}
	if curve != nil {
		out.Tier = curve.Tier
		out.TierLabel = curve.TierLabel
		out.Components = append(out.Components, TierFitnessComponent{
			Axis:      "Mana Curve",
			Fit:       curve.Fit,
			Weight:    1.0,
			Direction: curve.Direction,
			Note:      curveComponentNote(curve.Direction),
		})
	}
	if tutors != nil {
		if out.Tier == 0 {
			out.Tier = tutors.Tier
			out.TierLabel = tutors.TierLabel
		}
		out.Components = append(out.Components, TierFitnessComponent{
			Axis:      "Tutor Density",
			Fit:       tutors.Fit,
			Weight:    1.0,
			Direction: tutors.Direction,
			Note:      tutorComponentNote(tutors.Direction),
		})
	}
	if len(out.Components) == 0 {
		out.Summary = "No tier-fit components supplied — fitness not computable."
		return out
	}

	weightedSum := 0.0
	totalWeight := 0.0
	for _, c := range out.Components {
		w := c.Weight
		if w <= 0 {
			w = 1.0
		}
		weightedSum += w * c.Fit
		totalWeight += w
	}
	if totalWeight > 0 {
		out.Score = weightedSum / totalWeight
	}
	out.Band = tierFitnessBandFor(out.Score)

	// Sort components by Fit ascending — strongest mismatch first
	// so the Summary's "weakest axis" callout pulls Components[0].
	sort.SliceStable(out.Components, func(i, j int) bool {
		return out.Components[i].Fit < out.Components[j].Fit
	})

	out.Summary = buildTierFitnessSummary(out)
	return out
}

// curveComponentNote translates the curve fit's Direction into a
// summary-ready phrase.
func curveComponentNote(direction string) string {
	switch direction {
	case "too_expensive":
		return "curve too slow"
	case "too_lean":
		return "curve too lean"
	case "in_band":
		return "curve aligned"
	default:
		return ""
	}
}

// tutorComponentNote translates the tutor fit's Direction into a
// summary-ready phrase.
func tutorComponentNote(direction string) string {
	switch direction {
	case "too_sparse":
		return "tutor density too sparse"
	case "too_dense":
		return "tutor density too dense"
	case "in_band":
		return "tutor density aligned"
	default:
		return ""
	}
}

// tierFitnessBandFor maps Score to the semantic band label.
func tierFitnessBandFor(score float64) string {
	switch {
	case score >= 0.85:
		return "well_tuned"
	case score >= 0.70:
		return "mostly_tuned"
	case score >= 0.50:
		return "mixed_tuning"
	default:
		return "poorly_tuned"
	}
}

// tierFitnessBandLabel is the user-facing display string for the
// band — sentence-leading title case.
func tierFitnessBandLabel(band string) string {
	switch band {
	case "well_tuned":
		return "well-tuned"
	case "mostly_tuned":
		return "mostly tuned"
	case "mixed_tuning":
		return "mixed tuning"
	case "poorly_tuned":
		return "poorly tuned"
	default:
		return band
	}
}

// buildTierFitnessSummary renders the one-line summary. Format:
//
//	Aligned:        "Deck is well-tuned for T3 Upgraded Precon (92% fit)."
//	Single mismatch: "Deck is mostly tuned for T5 cEDH (78% fit) — slight tutor density too sparse."
//	Multi mismatch: "Deck is poorly tuned for T5 cEDH (24% fit) — curve too slow and tutor density too sparse."
func buildTierFitnessSummary(s *TierFitnessScore) string {
	if s == nil || len(s.Components) == 0 {
		return s.Summary
	}
	bandLabel := tierFitnessBandLabel(s.Band)
	header := fmt.Sprintf("Deck is %s for T%d %s (%.0f%% fit)",
		bandLabel, s.Tier, s.TierLabel, s.Score*100)

	// Pull out-of-band components (sorted asc by Fit, so weakest first).
	var weakNotes []string
	for _, c := range s.Components {
		if c.Direction != "in_band" && c.Note != "" {
			weakNotes = append(weakNotes, c.Note)
		}
	}
	if len(weakNotes) == 0 {
		return header + "."
	}
	// Cap to two notes — the strongest mismatches are already
	// first via the Fit-asc sort; trimming to two keeps the
	// summary at one line.
	if len(weakNotes) > 2 {
		weakNotes = weakNotes[:2]
	}
	return header + " — " + strings.Join(weakNotes, " and ") + "."
}
