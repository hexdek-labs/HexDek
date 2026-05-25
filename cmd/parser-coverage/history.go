package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"
)

// HistoryEntry is one parser-coverage run's stats, written as a single
// JSONL line. The file format is append-only — runs accumulate and a
// later analysis can replay the file to chart progress across
// releases.
//
// JSON tags use snake_case to match the rest of the tool's emitted
// data (see CSV columns) and to play nicely with jq one-liners.
//
// New fields can be added safely: missing fields decode to their zero
// value, so older entries continue to parse even after the schema
// grows. Old entries written with fewer fields will simply read with
// the new fields zero-valued in deltas.
type HistoryEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	Label         string    `json:"label,omitempty"`
	OracleTotal   int       `json:"oracle_total"`
	CorpusTotal   int       `json:"corpus_total"`
	ParseWarnings int       `json:"parse_warnings"`
	OK            int       `json:"ok"`
	OKVanilla     int       `json:"ok_vanilla"`
	Missing       int       `json:"missing"`
	EmptyAST      int       `json:"empty_ast"`
	Partial       int       `json:"partial"`
}

// resultsToHistoryEntry rolls up a classified result set into a single
// HistoryEntry stamped with the given timestamp + label. now is
// injectable so tests can pin a stable timestamp.
func resultsToHistoryEntry(results []result, oracleTotal, corpusTotal, parseWarnings int, label string, now time.Time) HistoryEntry {
	var counts [5]int
	for _, r := range results {
		counts[r.Class]++
	}
	return HistoryEntry{
		Timestamp:     now.UTC(),
		Label:         label,
		OracleTotal:   oracleTotal,
		CorpusTotal:   corpusTotal,
		ParseWarnings: parseWarnings,
		OK:            counts[classOK],
		OKVanilla:     counts[classOKVanilla],
		Missing:       counts[classMissing],
		EmptyAST:      counts[classEmptyAST],
		Partial:       counts[classPartial],
	}
}

// appendHistory writes entry as a single JSONL line at the end of path,
// creating the file if it doesn't exist. Append-only — no rewriting,
// no deduplication. Successive runs simply accumulate; a `jq -s` over
// the file replays the full progress trace.
func appendHistory(path string, entry HistoryEntry) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := f.Write(b); err != nil {
		return err
	}
	return nil
}

// readHistory returns every entry in the JSONL file in order. A
// missing file is treated as an empty history (nil, nil) — the
// first-ever --history run shouldn't fail just because the file
// doesn't exist yet. Blank lines are skipped silently. A malformed
// JSON line returns an error with the offending content so the user
// can diagnose corruption without re-running the audit.
func readHistory(path string) ([]HistoryEntry, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []HistoryEntry
	s := bufio.NewScanner(f)
	// Some long lines (e.g., a label with a verbose release note) can
	// exceed the default 64 KiB scanner buffer; raise the ceiling.
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineno := 0
	for s.Scan() {
		lineno++
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("history line %d: %w (%q)", lineno, err, line)
		}
		out = append(out, e)
	}
	return out, s.Err()
}

// HistoryDelta is the signed difference between two HistoryEntries
// (typically the most recent and the just-computed one). Field
// semantics: positive = more cards in that bucket vs. previous run;
// negative = fewer. For OK this means "more covered" (good); for
// Missing / EmptyAST / Partial this means "more uncovered" (bad).
type HistoryDelta struct {
	Prev          HistoryEntry
	Cur           HistoryEntry
	OracleTotal   int
	CorpusTotal   int
	ParseWarnings int
	OK            int
	OKVanilla     int
	Missing       int
	EmptyAST      int
	Partial       int
}

func computeDelta(prev, cur HistoryEntry) HistoryDelta {
	return HistoryDelta{
		Prev:          prev,
		Cur:           cur,
		OracleTotal:   cur.OracleTotal - prev.OracleTotal,
		CorpusTotal:   cur.CorpusTotal - prev.CorpusTotal,
		ParseWarnings: cur.ParseWarnings - prev.ParseWarnings,
		OK:            cur.OK - prev.OK,
		OKVanilla:     cur.OKVanilla - prev.OKVanilla,
		Missing:       cur.Missing - prev.Missing,
		EmptyAST:      cur.EmptyAST - prev.EmptyAST,
		Partial:       cur.Partial - prev.Partial,
	}
}

// signed renders an integer delta with an explicit + / - sign so the
// log line reads naturally ("OK +12 / Missing -5"). Zero renders as
// "0" to keep the visual width tight on the no-change case.
func signed(n int) string {
	if n > 0 {
		return fmt.Sprintf("+%d", n)
	}
	return fmt.Sprintf("%d", n)
}

// renderHistoryDelta builds a short multi-line summary suitable for
// stderr / log output. Format is stable so a CI step can grep on it.
//
// Sample output:
//
//	delta vs previous (r59 @ 2026-05-22T18:00:00Z)
//	  OK         +12   (28341 → 28353)
//	  OK_VANILLA +0    (5      → 5)
//	  MISSING    -2    (1265   → 1263)
//	  EMPTY_AST  -10   (2200   → 2190)
//	  PARTIAL    +0    (293    → 293)
func renderHistoryDelta(d HistoryDelta) string {
	var b strings.Builder
	label := d.Prev.Label
	if label == "" {
		label = "(unlabeled)"
	}
	fmt.Fprintf(&b, "delta vs previous (%s @ %s)\n",
		label, d.Prev.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(&b, "  OK         %-5s (%d → %d)\n", signed(d.OK), d.Prev.OK, d.Cur.OK)
	fmt.Fprintf(&b, "  OK_VANILLA %-5s (%d → %d)\n", signed(d.OKVanilla), d.Prev.OKVanilla, d.Cur.OKVanilla)
	fmt.Fprintf(&b, "  MISSING    %-5s (%d → %d)\n", signed(d.Missing), d.Prev.Missing, d.Cur.Missing)
	fmt.Fprintf(&b, "  EMPTY_AST  %-5s (%d → %d)\n", signed(d.EmptyAST), d.Prev.EmptyAST, d.Cur.EmptyAST)
	fmt.Fprintf(&b, "  PARTIAL    %-5s (%d → %d)\n", signed(d.Partial), d.Prev.Partial, d.Cur.Partial)
	return b.String()
}
