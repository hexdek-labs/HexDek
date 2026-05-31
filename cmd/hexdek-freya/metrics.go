package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// metrics — Freya output-quality measurement surface.
//
// Two things show up here:
//
//   - FreyaMetrics: three "% of X meets quality bar" scores computed from
//     a single FreyaReport. Surfaced via `--mode metrics`.
//   - ConsistencyResult: the output of running Freya twice on the same
//     deck and diffing the JSON. Any drift is a non-determinism quality
//     smell (map iteration leaking through to a slice, time.Now() in a
//     report field, etc.) — the codebase aims for bit-stable per-deck
//     output and the probe is the regression bar.
//
// The metrics are intentionally simple ratios — they're a smoke test, not
// a calibrated scorecard. The point is to surface regressions (we dropped
// from 78% combo-summary coverage to 41% last week, what broke?) rather
// than to render an absolute quality grade.

// ConfidenceThreshold is the score floor for "high-confidence"
// classifications. 0.8 is set above the BlendConfidenceCeiling (0.7) so a
// blend-eligible archetype call doesn't automatically count as
// high-confidence; the bar is "the classifier is sure enough that no
// human-in-the-loop review would change the call."
const ConfidenceThreshold = 0.8

// FreyaMetrics is the per-deck quality scorecard. JSON-encodable so
// downstream tooling (CI dashboards, regression bisectors) can ingest it
// without parsing the freeform text report.
type FreyaMetrics struct {
	DeckName   string `json:"deck_name"`
	DeckPath   string `json:"deck_path,omitempty"`
	TotalCards int    `json:"total_cards"`

	// Confidence coverage: of the items in the report that carry a
	// Confidence score, what fraction meet the high-confidence bar?
	// Currently this is WinLines and the Archetype primary call.
	ConfidenceItemCount int     `json:"confidence_item_count"`
	ConfidenceHighCount int     `json:"confidence_high_count"`
	ConfidenceHighPct   float64 `json:"confidence_high_pct"`

	// Tag coverage: of the per-card role assignments, what fraction got
	// at least one explicit non-Land role tag? An untagged card is
	// either a basic land (uninteresting) or an unclassified body
	// (Freya gap — we want this metric to surface those).
	CardTagCount             int     `json:"card_tag_count"`
	CardWithExplicitTagCount int     `json:"card_with_explicit_tag_count"`
	CardWithExplicitTagPct   float64 `json:"card_with_explicit_tag_pct"`

	// Combo summary coverage: of the combos detected across all five
	// buckets, what fraction carry a non-empty human-readable
	// Description? An empty Description means the detector matched the
	// shape but the curated/heuristic summary path didn't populate.
	ComboCount            int     `json:"combo_count"`
	ComboWithSummaryCount int     `json:"combo_with_summary_count"`
	ComboWithSummaryPct   float64 `json:"combo_with_summary_pct"`

	// Consistency is populated when ComputeMetrics is paired with a
	// double-run consistency probe (see RunConsistencyProbe). Nil when
	// the caller skipped the probe.
	Consistency *ConsistencyResult `json:"consistency,omitempty"`
}

// ConsistencyResult is the output of running Freya twice on the same
// deck and diffing the JSON-encoded reports.
type ConsistencyResult struct {
	// JSONByteEqual is true when the two runs produce byte-identical
	// JSON. This is the strongest determinism signal — anything weaker
	// (line counts equal, fields equal in any order) lets map-iteration
	// leak through.
	JSONByteEqual bool `json:"json_byte_equal"`

	Run1Bytes int `json:"run1_bytes"`
	Run2Bytes int `json:"run2_bytes"`

	// FirstDiffLine is the 1-indexed line number where the JSON outputs
	// first diverge. Zero when JSONByteEqual is true. Useful for
	// pointing the next investigator at the field that drifted.
	FirstDiffLine int `json:"first_diff_line,omitempty"`

	// FirstDiffRun1 / FirstDiffRun2 are the actual diverging lines
	// (truncated to 200 chars) — enough context to recognize the field
	// without dumping the whole JSON blob.
	FirstDiffRun1 string `json:"first_diff_run1,omitempty"`
	FirstDiffRun2 string `json:"first_diff_run2,omitempty"`
}

// ComputeMetrics walks a FreyaReport and produces the three quality
// scores. Always safe — handles nil sub-reports (Archetype, WinLines,
// Roles) by skipping their contribution rather than crashing.
func ComputeMetrics(report *FreyaReport) *FreyaMetrics {
	m := &FreyaMetrics{
		DeckName:   report.DeckName,
		DeckPath:   report.DeckPath,
		TotalCards: report.TotalCards,
	}

	// Confidence coverage: archetype primary + every WinLine.
	if report.Archetype != nil && report.Archetype.Primary != "" {
		m.ConfidenceItemCount++
		if report.Archetype.PrimaryConfidence >= ConfidenceThreshold {
			m.ConfidenceHighCount++
		}
	}
	if report.WinLines != nil {
		for i := range report.WinLines.WinLines {
			m.ConfidenceItemCount++
			if report.WinLines.WinLines[i].Confidence >= ConfidenceThreshold {
				m.ConfidenceHighCount++
			}
		}
	}
	if m.ConfidenceItemCount > 0 {
		m.ConfidenceHighPct = 100.0 *
			float64(m.ConfidenceHighCount) / float64(m.ConfidenceItemCount)
	}

	// Tag coverage: % of role-assigned cards carrying at least one
	// non-Land role. Lands are excluded from both numerator and
	// denominator — a basic land with only the Land tag is the expected
	// shape, not a Freya gap.
	if report.Roles != nil {
		for _, a := range report.Roles.Assignments {
			if isLandOnly(a.Roles) {
				continue
			}
			m.CardTagCount++
			if hasNonLandRole(a.Roles) {
				m.CardWithExplicitTagCount++
			}
		}
	}
	if m.CardTagCount > 0 {
		m.CardWithExplicitTagPct = 100.0 *
			float64(m.CardWithExplicitTagCount) / float64(m.CardTagCount)
	}

	// Combo summary coverage across all five buckets.
	comboBuckets := [][]ComboResult{
		report.TrueInfinites,
		report.Determined,
		report.Finishers,
		report.Synergies,
		report.GraveyardLoops,
	}
	for _, bucket := range comboBuckets {
		for _, c := range bucket {
			m.ComboCount++
			if hasHumanSummary(c) {
				m.ComboWithSummaryCount++
			}
		}
	}
	if m.ComboCount > 0 {
		m.ComboWithSummaryPct = 100.0 *
			float64(m.ComboWithSummaryCount) / float64(m.ComboCount)
	}

	return m
}

// isLandOnly returns true when the only role on a card is RoleLand. A
// card with no roles at all is NOT land-only — that's an unclassified
// non-land (Freya gap), which we want to surface via the tag-coverage
// metric.
func isLandOnly(roles []RoleTag) bool {
	if len(roles) == 0 {
		return false
	}
	for _, r := range roles {
		if r != RoleLand {
			return false
		}
	}
	return true
}

// hasNonLandRole returns true when at least one role on the card is not
// RoleLand. Empty role slices return false (= unclassified non-land).
func hasNonLandRole(roles []RoleTag) bool {
	for _, r := range roles {
		if r != RoleLand {
			return true
		}
	}
	return false
}

// hasHumanSummary returns true when a combo carries human-readable
// summary text. Either the Description field is non-empty OR the
// graph-walk Annotation populated a Summary. Empty descriptions mean
// the detector matched the shape but didn't enrich it with a curated
// or heuristic narrative — the metric's purpose is to surface those.
func hasHumanSummary(c ComboResult) bool {
	if strings.TrimSpace(c.Description) != "" {
		return true
	}
	if c.Annotation != nil && strings.TrimSpace(c.Annotation.Summary) != "" {
		return true
	}
	return false
}

// RunConsistencyProbe runs Freya twice on the same deck and diffs the
// JSON-encoded reports. Returns a ConsistencyResult describing whether
// the two runs were bit-identical and, if not, where they first
// diverged. The probe is intentionally synchronous and sequential — we
// want to test pure-function determinism, not concurrent-access
// stability.
func RunConsistencyProbe(deckPath string, oracle *oracleDB, mechDB *MechanicDB) (*ConsistencyResult, error) {
	report1, err := analyzeDeckFile(deckPath, oracle, mechDB)
	if err != nil {
		return nil, fmt.Errorf("probe run 1: %w", err)
	}
	report2, err := analyzeDeckFile(deckPath, oracle, mechDB)
	if err != nil {
		return nil, fmt.Errorf("probe run 2: %w", err)
	}

	buf1, err := marshalReportForDiff(report1)
	if err != nil {
		return nil, fmt.Errorf("marshal run 1: %w", err)
	}
	buf2, err := marshalReportForDiff(report2)
	if err != nil {
		return nil, fmt.Errorf("marshal run 2: %w", err)
	}

	res := &ConsistencyResult{
		Run1Bytes:     len(buf1),
		Run2Bytes:     len(buf2),
		JSONByteEqual: bytes.Equal(buf1, buf2),
	}
	if !res.JSONByteEqual {
		line, a, b := firstLineDiff(buf1, buf2)
		res.FirstDiffLine = line
		res.FirstDiffRun1 = truncateForDisplay(a, 200)
		res.FirstDiffRun2 = truncateForDisplay(b, 200)
	}
	return res, nil
}

// marshalReportForDiff produces a stable JSON encoding of a FreyaReport
// suitable for line-by-line comparison. SetIndent gives a per-field-per-
// line layout so the diff line number points at a concrete field; no
// extra normalization beyond what encoding/json already provides (map
// keys are alphabetized, slices preserve insertion order — slice-order
// drift IS the kind of non-determinism this probe is trying to catch).
func marshalReportForDiff(report *FreyaReport) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// firstLineDiff returns the 1-indexed line number where a and b first
// differ, plus the diverging line content from each. When one side is
// shorter and is a prefix of the other, returns the trailing line on
// the longer side. When both are equal, returns (0, "", "").
func firstLineDiff(a, b []byte) (int, string, string) {
	linesA := strings.Split(string(a), "\n")
	linesB := strings.Split(string(b), "\n")
	n := len(linesA)
	if len(linesB) < n {
		n = len(linesB)
	}
	for i := 0; i < n; i++ {
		if linesA[i] != linesB[i] {
			return i + 1, linesA[i], linesB[i]
		}
	}
	if len(linesA) != len(linesB) {
		if len(linesA) > n {
			return n + 1, linesA[n], ""
		}
		return n + 1, "", linesB[n]
	}
	return 0, "", ""
}

func truncateForDisplay(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// PrintMetricsReport writes the metrics scorecard in either text or JSON
// form. Text format renders as a five-section block (header, three
// percentages, consistency); JSON emits the FreyaMetrics struct directly.
func PrintMetricsReport(w io.Writer, m *FreyaMetrics, format string) {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(m)
		return
	}
	fmt.Fprintf(w, "\nFREYA -- Output Quality Metrics\n")
	fmt.Fprintf(w, "================================\n")
	if m.DeckPath != "" {
		fmt.Fprintf(w, "Deck: %s\n", m.DeckPath)
	} else {
		fmt.Fprintf(w, "Deck: %s\n", m.DeckName)
	}
	fmt.Fprintf(w, "Cards: %d\n\n", m.TotalCards)

	fmt.Fprintf(w, "Confidence coverage (threshold %.2f):\n", ConfidenceThreshold)
	fmt.Fprintf(w, "  %5.1f%%  (%d of %d items)\n\n",
		m.ConfidenceHighPct, m.ConfidenceHighCount, m.ConfidenceItemCount)

	fmt.Fprintf(w, "Cards with explicit non-land tag:\n")
	fmt.Fprintf(w, "  %5.1f%%  (%d of %d non-land cards)\n\n",
		m.CardWithExplicitTagPct, m.CardWithExplicitTagCount, m.CardTagCount)

	fmt.Fprintf(w, "Combos with human-readable summary:\n")
	fmt.Fprintf(w, "  %5.1f%%  (%d of %d combos)\n\n",
		m.ComboWithSummaryPct, m.ComboWithSummaryCount, m.ComboCount)

	if m.Consistency != nil {
		fmt.Fprintf(w, "Consistency probe (Freya run twice on same deck):\n")
		if m.Consistency.JSONByteEqual {
			fmt.Fprintf(w, "  [OK] bit-identical (%d bytes)\n", m.Consistency.Run1Bytes)
		} else {
			fmt.Fprintf(w, "  [DRIFT] first diff at line %d\n",
				m.Consistency.FirstDiffLine)
			fmt.Fprintf(w, "    run1: %s\n", m.Consistency.FirstDiffRun1)
			fmt.Fprintf(w, "    run2: %s\n", m.Consistency.FirstDiffRun2)
			fmt.Fprintf(w, "    (run1 %d bytes, run2 %d bytes)\n",
				m.Consistency.Run1Bytes, m.Consistency.Run2Bytes)
		}
	}
}
