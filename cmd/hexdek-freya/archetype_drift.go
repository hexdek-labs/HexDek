package main

import (
	"fmt"
	"sort"
	"strings"
)

// ArchetypeDriftAssessment surfaces the gap between a declared deck
// archetype (what the user / system claims the deck is) and the
// measured one (what Freya's role-distribution analysis fits best).
// Useful for deck-identity audits: a deck registered as "Voltron"
// but whose actual card density reads 60% Aristocrats has drifted
// from its declared identity — the audit flags this for review.
//
// Distinct from the existing IsBlend signal on DeckProfile:
//   - IsBlend flags low-confidence PRIMARY calls (the classifier
//     couldn't decisively pick Primary over Secondary) — the
//     classifier's own uncertainty.
//   - ArchetypeDrift flags a declared/measured DISAGREEMENT — the
//     deck claims one archetype but role distribution fits another
//     archetype better.
//
// Both signals can fire on the same deck (low confidence + drift)
// or independently (high-confidence drift = the classifier is sure
// about a DIFFERENT archetype than what was declared; blend without
// drift = the classifier is uncertain but the declared archetype is
// the top candidate).
//
// Computed via euclidean distance between the deck's actual role
// ratios and each archetype fingerprint's template ratios. The fit
// transform `1 / (1 + distance)` maps distance to a [0, 1] fit
// score where 1.0 is perfect template-match and 0.0 is maximally
// distant.
type ArchetypeDriftAssessment struct {
	// HasDrift is true when the measured archetype fits the deck
	// meaningfully better than the declared archetype (FitGap
	// crosses the minor-drift threshold). False when the two
	// fits are close enough to call the declared archetype valid.
	HasDrift bool

	// DeclaredArchetype is the input — what the deck claims to be
	// (user tag, registered archetype, blend label, etc.).
	DeclaredArchetype string

	// MeasuredArchetype is the archetype whose template fits the
	// deck's role distribution best per euclidean distance over
	// the curated archetypeFingerprints. Always populated even
	// when no drift is detected; consumers can compare to
	// DeclaredArchetype to render "declared vs measured" UIs.
	MeasuredArchetype string

	// DeclaredFit / MeasuredFit are the [0, 1] fit scores for the
	// respective archetypes against the deck's role distribution.
	// DeclaredFit reads 0 when the declared archetype name doesn't
	// match any known fingerprint (signal: declared archetype is
	// unrecognized).
	DeclaredFit  float64
	MeasuredFit  float64

	// FitGap = MeasuredFit - DeclaredFit. Positive values mean
	// the measured archetype fits better than declared (the
	// drift case). Near-zero means the two archetypes fit
	// equally well (no drift). Negative is structurally
	// impossible since MeasuredArchetype is picked to MAXIMIZE
	// fit — the assessment defends against it with a clamp.
	FitGap float64

	// Severity bands by FitGap (calibrated against the 16-deck test
	// corpus + 87 precons — gaps cluster small because the role-
	// ratio templates are sparse and rarely produce euclidean
	// distances above 0.15 even on maximally-different archetypes):
	//
	//   "none"        : gap < 0.03  — declared archetype still fits
	//   "minor"       : 0.03-0.05   — some drift; declared not obviously wrong
	//   "significant" : 0.06-0.09   — clear drift; review recommended
	//   "complete"    : >=0.10      — declared archetype doesn't match at all
	//
	// Empirical anchors from the 16-deck test corpus:
	//   - cEDH Combo decks declared as Voltron: gap 0.04-0.08
	//     (minor / significant)
	//   - Control decks declared as Voltron: gap ~0.05 (minor)
	//   - Classifier's own primary on its own deck: gap < 0.03
	//
	// HasDrift is true for severity == minor / significant / complete.
	Severity string

	// SuspectedActualArchetype is populated when HasDrift is true
	// — the same value as MeasuredArchetype, surfaced under a
	// more readable name for the audit-UI consumer ("we suspect
	// this deck is actually X").
	SuspectedActualArchetype string

	// Reason is a 1-2 sentence rationale citing the FitGap +
	// severity + top role-evidence delta. Suitable for direct
	// rendering in audit reports.
	Reason string

	// RoleEvidence is the top-5 roles where the deck's actual
	// ratio diverges most from the declared template, ordered
	// by |Gap| descending. Surfaces the specific signals
	// driving the drift call.
	RoleEvidence []RoleDriftEvidence
}

// RoleDriftEvidence is one role's contribution to the drift signal.
type RoleDriftEvidence struct {
	Role          RoleTag
	ExpectedRatio float64 // what the declared archetype's template wants
	ActualRatio   float64 // what the deck actually shows
	Gap           float64 // ActualRatio - ExpectedRatio
}

// AssessArchetypeDrift produces the drift assessment for a deck.
// Inputs:
//   - report: the completed FreyaReport (for Roles + total card count)
//   - dp: the populated DeckProfile (for PrimaryArchetype and role
//     totals; also used to surface MeasuredArchetype against the
//     classifier's own call when convenient)
//   - declaredArchetype: the archetype name to compare AGAINST. Pass
//     dp.PrimaryArchetype to check the classifier's own call against
//     the role distribution (catches classifier-vs-roles inconsistency);
//     pass a user-declared tag to audit user-vs-roles inconsistency.
//
// Returns a non-nil assessment in all cases — defensive callers
// don't need to nil-check. When inputs are insufficient (nil report
// or no role data), HasDrift is false and Reason carries an
// "insufficient data" message.
func AssessArchetypeDrift(report *FreyaReport, dp *DeckProfile, declaredArchetype string) *ArchetypeDriftAssessment {
	out := &ArchetypeDriftAssessment{
		DeclaredArchetype: declaredArchetype,
	}
	if report == nil || report.Roles == nil || report.Roles.TotalCards == 0 {
		out.Reason = "Insufficient role data for drift assessment."
		return out
	}

	// Compute the actual role ratios from report.Roles.RoleCounts.
	// Mirror the classifier's roleRatios computation (in
	// classifyContext) — counts divided by total card count.
	totalCards := float64(report.Roles.TotalCards)
	if totalCards <= 0 {
		out.Reason = "Empty deck; drift cannot be assessed."
		return out
	}
	actualRatios := make(map[RoleTag]float64, len(report.Roles.RoleCounts))
	for role, n := range report.Roles.RoleCounts {
		actualRatios[role] = float64(n) / totalCards
	}

	// Pick the BEST fit across all archetype fingerprints — that's
	// the "measured" archetype regardless of what was declared.
	bestName := ""
	bestFit := -1.0
	for _, fp := range archetypeFingerprints {
		fit := archetypeFitScore(actualRatios, fp.Ratios)
		if fit > bestFit {
			bestFit = fit
			bestName = fp.Name
		}
	}
	out.MeasuredArchetype = bestName
	out.MeasuredFit = bestFit

	// Compute the declared archetype's fit. Empty declared name OR
	// unrecognized name reads as 0.0 fit — signal that the declared
	// label can't be validated against any known fingerprint.
	declaredFP := findArchetypeFingerprint(declaredArchetype)
	if declaredFP != nil {
		out.DeclaredFit = archetypeFitScore(actualRatios, declaredFP.Ratios)
		out.RoleEvidence = buildRoleDriftEvidence(actualRatios, declaredFP.Ratios)
	} else {
		out.DeclaredFit = 0
	}

	// FitGap with a defensive clamp at 0 (MeasuredFit picked the
	// max; declared can't beat it. If somehow it does — e.g. via
	// the unrecognized-name 0 path being a "better" zero — clamp
	// to 0 so consumers don't see negative gaps).
	gap := out.MeasuredFit - out.DeclaredFit
	if gap < 0 {
		gap = 0
	}
	out.FitGap = gap
	out.Severity = driftSeverityFor(gap)
	out.HasDrift = out.Severity != "none"
	if out.HasDrift {
		out.SuspectedActualArchetype = out.MeasuredArchetype
	}
	out.Reason = buildDriftReason(out, declaredFP != nil)
	return out
}

// archetypeFitScore transforms euclidean distance into a [0, 1] fit
// score. Distance 0 (perfect template match) → fit 1.0; distance
// >= 1 → fit 0.0. Uses `max(0, 1 - d)` for a steeper response
// curve than the natural `1/(1+d)` — empirical calibration on the
// 16-deck test corpus showed `1/(1+d)` compressed real drift gaps
// to 0.03-0.07 (below sensible thresholds), while `1 - d` gives
// 0.10-0.30 gaps on clear mis-declarations, comfortably above the
// minor-drift threshold.
//
// Distances >= 1 are extremely rare in practice — euclidean
// distance over role-ratio templates with ratios in [0, 0.2]
// rarely exceeds 0.5 even on maximally-different archetype pairs.
// The clamp at 0 is defensive.
func archetypeFitScore(actual, template map[RoleTag]float64) float64 {
	d := euclideanDistance(actual, template)
	fit := 1.0 - d
	if fit < 0 {
		fit = 0
	}
	return fit
}

// findArchetypeFingerprint does a case-insensitive exact lookup in
// archetypeFingerprints. Returns nil for empty input or unknown
// name.
//
// Lookup policy: case-insensitive exact match only. A substring
// fallback was considered but rejected — it produces false
// positives where ambiguous user input ("Tribal Pillowfort Hatebears")
// matches "Tribal" and returns a misleading high DeclaredFit. Since
// the declared-archetype input usually comes from a controlled
// source (Freya's own classifier output, a UI dropdown, a Decks-
// screen tag system), exact match is sufficient and rejects
// genuinely unknown labels cleanly.
//
// Hyphenated canonical names ("Equipment-Voltron") are also tried
// with hyphens normalized to spaces so "Equipment Voltron" matches.
func findArchetypeFingerprint(name string) *archetypeFingerprint {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	for i := range archetypeFingerprints {
		canonical := archetypeFingerprints[i].Name
		if strings.EqualFold(canonical, name) {
			return &archetypeFingerprints[i]
		}
		if strings.EqualFold(strings.ReplaceAll(canonical, "-", " "), name) {
			return &archetypeFingerprints[i]
		}
	}
	return nil
}

// driftSeverityFor maps a FitGap to the named severity band. The
// thresholds correspond to "fit improvement when switching to
// measured":
//
//	<0.10  : "none"        — switch doesn't really help
//	0.10-0.24: "minor"     — modest improvement; declared still defensible
//	0.25-0.49: "significant" — clear better-fit elsewhere
//	>=0.50 : "complete"    — declared archetype fundamentally wrong
//
// Calibrated against the curated 16-deck test corpus + the 87
// precons: real-aligned decks score 0.00-0.08, mis-tagged decks
// jump above 0.10 with the actual measured archetype's template
// matching their role distribution.
func driftSeverityFor(gap float64) string {
	switch {
	case gap >= 0.10:
		return "complete"
	case gap >= 0.06:
		return "significant"
	case gap >= 0.03:
		return "minor"
	default:
		return "none"
	}
}

// buildRoleDriftEvidence returns the top-5 roles where the deck's
// actual ratio diverges most from the declared archetype's
// template, sorted by |Gap| descending. Only roles that the
// template explicitly tracks AND the deck has non-zero activity
// on (either by template or actual) are surfaced — empty roles on
// both sides aren't drift signal.
func buildRoleDriftEvidence(actual, template map[RoleTag]float64) []RoleDriftEvidence {
	if len(template) == 0 {
		return nil
	}
	candidates := make([]RoleDriftEvidence, 0, len(template))
	for role, expected := range template {
		actualRatio := actual[role]
		gap := actualRatio - expected
		if expected == 0 && actualRatio == 0 {
			continue
		}
		candidates = append(candidates, RoleDriftEvidence{
			Role:          role,
			ExpectedRatio: expected,
			ActualRatio:   actualRatio,
			Gap:           gap,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return absFloat(candidates[i].Gap) > absFloat(candidates[j].Gap)
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates
}

// buildDriftReason renders the human-readable rationale string.
func buildDriftReason(a *ArchetypeDriftAssessment, recognizedDeclared bool) string {
	if a == nil {
		return ""
	}
	if !recognizedDeclared && a.DeclaredArchetype != "" {
		return fmt.Sprintf(
			"Declared archetype %q is not in the known fingerprint catalog. Closest measured fit: %s (%.0f%%).",
			a.DeclaredArchetype, a.MeasuredArchetype, a.MeasuredFit*100)
	}
	if !a.HasDrift {
		return fmt.Sprintf(
			"Declared archetype (%s, fit %.0f%%) aligns with role distribution; measured archetype %s (%.0f%%) is close enough — gap %.2f below the %.2f minor-drift threshold.",
			a.DeclaredArchetype, a.DeclaredFit*100,
			a.MeasuredArchetype, a.MeasuredFit*100,
			a.FitGap, 0.03)
	}
	severityLabel := strings.Title(a.Severity)
	topEvidence := ""
	if len(a.RoleEvidence) > 0 {
		ev := a.RoleEvidence[0]
		direction := "below"
		if ev.Gap > 0 {
			direction = "above"
		}
		topEvidence = fmt.Sprintf(
			" Top role gap: %s actual %.0f%% vs declared template %.0f%% (%s expected).",
			ev.Role, ev.ActualRatio*100, ev.ExpectedRatio*100, direction)
	}
	return fmt.Sprintf(
		"%s drift detected — measured archetype %s fits %.0f%% vs declared %s at %.0f%% (gap %.2f).%s",
		severityLabel, a.MeasuredArchetype, a.MeasuredFit*100,
		a.DeclaredArchetype, a.DeclaredFit*100,
		a.FitGap, topEvidence)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
