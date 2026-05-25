package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Trend is the diff between two HistoryEntries — typically a "now"
// snapshot vs a baseline from a chosen point in the past. Delta
// fields are From → To (positive = increased; the sign meaning
// flips by field, e.g. +OK is good, +Missing is bad).
type Trend struct {
	From            HistoryEntry
	To              HistoryEntry
	Span            time.Duration
	CardsGained     int     // (To.OK + To.OKVanilla) - (From.OK + From.OKVanilla)
	UncoveredDelta  int     // (To.Missing+EmptyAST+Partial) - (From same)
	MissingDelta    int
	EmptyASTDelta   int
	PartialDelta    int
	CoveragePPDelta float64 // percentage points
}

// computeTrend builds the Trend between two history points. Order
// matters: from is the baseline, to is the current snapshot.
// Both-zero OracleTotal yields a 0.0 CoveragePPDelta rather than NaN.
func computeTrend(from, to HistoryEntry) Trend {
	fromCov := from.OK + from.OKVanilla
	toCov := to.OK + to.OKVanilla
	fromUnc := from.Missing + from.EmptyAST + from.Partial
	toUnc := to.Missing + to.EmptyAST + to.Partial
	return Trend{
		From:            from,
		To:              to,
		Span:            to.Timestamp.Sub(from.Timestamp),
		CardsGained:     toCov - fromCov,
		UncoveredDelta:  toUnc - fromUnc,
		MissingDelta:    to.Missing - from.Missing,
		EmptyASTDelta:   to.EmptyAST - from.EmptyAST,
		PartialDelta:    to.Partial - from.Partial,
		CoveragePPDelta: entryCoveragePct(to) - entryCoveragePct(from),
	}
}

// entryCoveragePct returns (OK + OK_VANILLA) / OracleTotal * 100,
// or 0 for the zero-total degenerate case. Local helper kept out of
// progress.go because it operates on HistoryEntry rather than
// ProgressPoint — they're structurally close but distinct types.
func entryCoveragePct(e HistoryEntry) float64 {
	if e.OracleTotal == 0 {
		return 0
	}
	return 100.0 * float64(e.OK+e.OKVanilla) / float64(e.OracleTotal)
}

// selectBaseline picks the baseline HistoryEntry for the trend
// comparison. Selector grammar (in priority order):
//
//	""            → "oldest"
//	"oldest"      → entries[0]
//	"previous"    → entries[len-2] (errors if only one entry)
//	"<label>"     → first entry whose Label matches exactly
//	"YYYY-MM-DD"  → first entry with Timestamp on or after that date
//
// Returns a typed error if no match — caller turns it into a user-
// friendly log line.
func selectBaseline(entries []HistoryEntry, selector string) (HistoryEntry, error) {
	if len(entries) == 0 {
		return HistoryEntry{}, errors.New("history is empty — record at least one entry with --history before computing a trend")
	}
	sel := strings.TrimSpace(selector)
	switch sel {
	case "", "oldest":
		return entries[0], nil
	case "previous":
		if len(entries) < 2 {
			return HistoryEntry{}, errors.New("--trend-baseline=previous requires at least 2 history entries")
		}
		return entries[len(entries)-2], nil
	}
	// Exact label match.
	for _, e := range entries {
		if e.Label == sel {
			return e, nil
		}
	}
	// Date selector (YYYY-MM-DD) — find earliest entry on or after.
	if cutoff, err := time.Parse("2006-01-02", sel); err == nil {
		for _, e := range entries {
			if !e.Timestamp.Before(cutoff) {
				return e, nil
			}
		}
		return HistoryEntry{}, fmt.Errorf("no history entry on or after %s", sel)
	}
	return HistoryEntry{}, fmt.Errorf("baseline %q matches no label and is not 'oldest', 'previous', or a YYYY-MM-DD date", sel)
}

// labelOrDate renders an entry's identity for human reading: the
// label if present, otherwise the YYYY-MM-DD timestamp.
func labelOrDate(e HistoryEntry) string {
	if e.Label != "" {
		return e.Label
	}
	return e.Timestamp.Format("2006-01-02")
}

// renderTrendMarkdown produces the release-note-friendly summary.
// Leads with "+N cards" because that's the framing the brief asked
// for — "we gained 142 cards" — followed by the coverage% move and
// a per-class delta table.
func renderTrendMarkdown(tr Trend) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# Parser Coverage Trend\n\n")

	fromName := labelOrDate(tr.From)
	toName := labelOrDate(tr.To)

	gainStr := signed(tr.CardsGained)
	fmt.Fprintf(&b, "**%s cards** newly covered since `%s`", gainStr, fromName)
	if tr.Span > 0 {
		fmt.Fprintf(&b, " (%s ago)", humanDuration(tr.Span))
	}
	b.WriteString(".\n\n")

	fromPct := entryCoveragePct(tr.From)
	toPct := entryCoveragePct(tr.To)
	fmt.Fprintf(&b, "Coverage: **%.2f%%** → **%.2f%%** (%+.2f pp)\n\n", fromPct, toPct, tr.CoveragePPDelta)

	fromCov := tr.From.OK + tr.From.OKVanilla
	toCov := tr.To.OK + tr.To.OKVanilla
	fromUnc := tr.From.Missing + tr.From.EmptyAST + tr.From.Partial
	toUnc := tr.To.Missing + tr.To.EmptyAST + tr.To.Partial

	fmt.Fprintf(&b, "| Metric | %s | %s | Δ |\n", fromName, toName)
	fmt.Fprintf(&b, "|---|---:|---:|---:|\n")
	fmt.Fprintf(&b, "| Covered (OK + OK_VANILLA) | %d | %d | %s |\n", fromCov, toCov, signed(toCov-fromCov))
	fmt.Fprintf(&b, "| Uncovered total | %d | %d | %s |\n", fromUnc, toUnc, signed(tr.UncoveredDelta))
	fmt.Fprintf(&b, "| MISSING | %d | %d | %s |\n", tr.From.Missing, tr.To.Missing, signed(tr.MissingDelta))
	fmt.Fprintf(&b, "| EMPTY_AST | %d | %d | %s |\n", tr.From.EmptyAST, tr.To.EmptyAST, signed(tr.EmptyASTDelta))
	fmt.Fprintf(&b, "| PARTIAL | %d | %d | %s |\n", tr.From.Partial, tr.To.Partial, signed(tr.PartialDelta))
	b.WriteString("\n")

	fmt.Fprintf(&b, "Baseline: `%s` (%s)  \n", fromName, tr.From.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(&b, "Current: `%s` (%s)\n", toName, tr.To.Timestamp.Format(time.RFC3339))

	return []byte(b.String())
}

// writeTrendReport reads the history JSONL, selects the baseline,
// computes the trend against the most-recent entry, and writes the
// markdown summary at path. An empty path writes to stdout — useful
// for `parser-coverage --trend - --trend-baseline r58 > release-notes.md`
// shell-pipe workflows.
func writeTrendReport(path, historyPath, baselineSelector string) error {
	entries, err := readHistory(historyPath)
	if err != nil {
		return fmt.Errorf("read history: %w", err)
	}
	if len(entries) == 0 {
		return errors.New("history is empty — record at least one entry with --history before computing a trend")
	}
	baseline, err := selectBaseline(entries, baselineSelector)
	if err != nil {
		return err
	}
	tr := computeTrend(baseline, entries[len(entries)-1])
	body := renderTrendMarkdown(tr)
	if path == "" || path == "-" {
		_, err := os.Stdout.Write(body)
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
