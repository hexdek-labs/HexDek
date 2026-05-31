package deckparser

import (
	"bytes"
	"strings"
	"testing"
)

// parse_report_r60_test.go — structured per-line parse coverage report.
//
// Verifies the LineStatus / ParseReport plumbing introduced for the
// hexdek-judge `--report-parse` flag. Pre-fix the parser silently
// dropped failed lines into td.Unresolved with no source-line context
// and no path-of-resolution detail — UIs surfacing "X cards missing"
// couldn't tell the user WHICH line failed or WHY. New ParseReport
// captures: per-line LineStatus (Resolved direct meta hit / Fallback
// face-match-or-DFC-canonicalize / Unresolved), roll-up counts
// (TotalLines / ResolvedLines / FallbackResolved / UnresolvedLines /
// DroppedLines), and per-failure UnresolvedDetails with source line
// number + raw text + reason.

func reportMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice", "Sol Ring", "Lightning Bolt",
		"Forest", "Mountain", "Counterspell",
		// DFC entry — only the canonical ` // ` form is in meta. Front-face
		// lookups must take the buildCard face-match fallback.
		"Aang, Swift Savior // Aang and La, Ocean's Fury",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, Types: []string{"generic"}, CMC: 1,
		}
	}
	return meta
}

// TestParseReport_CleanDeckAllResolved — a deck where every card hits
// meta directly: report shows full coverage, zero unresolved, every
// CardLine stamped LineStatusResolved.
func TestParseReport_CleanDeckAllResolved(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Lightning Bolt
1 Counterspell
2 Forest
2 Mountain
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, reportMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := td.ParseReport
	if r.UnresolvedLines != 0 {
		t.Errorf("UnresolvedLines = %d, want 0", r.UnresolvedLines)
	}
	if r.FallbackResolved != 0 {
		t.Errorf("FallbackResolved = %d, want 0 (every card has direct meta hit)", r.FallbackResolved)
	}
	if r.ResolvedLines == 0 {
		t.Errorf("ResolvedLines = 0, want >= 1")
	}
	if cov := r.CoveragePercent(); cov != 100.0 {
		t.Errorf("CoveragePercent = %v, want 100.0", cov)
	}
	for i, cl := range td.CardLines {
		if cl.Status != LineStatusResolved {
			t.Errorf("CardLines[%d] (%s) status = %v, want LineStatusResolved", i, cl.Name, cl.Status)
		}
		if cl.LineNumber == 0 {
			t.Errorf("CardLines[%d] (%s) LineNumber = 0; want 1-based source line", i, cl.Name)
		}
	}
}

// TestParseReport_UnresolvedLinesPopulated — a deck mixing valid cards
// with typos / unknown names. Report.UnresolvedDetails should list
// each broken line with its source line number, raw text, name, and
// a non-empty reason. The hexdek-judge --report-parse flag prints
// these to point the user at the broken line.
func TestParseReport_UnresolvedLinesPopulated(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Definitely Not A Real Card
1 Lightning Bolt
1 Another Made-Up Spell
2 Forest
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, reportMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := td.ParseReport
	if r.UnresolvedLines != 2 {
		t.Errorf("UnresolvedLines = %d, want 2 (1 typo each at lines 3 and 5)", r.UnresolvedLines)
	}
	if len(r.UnresolvedDetails) != 2 {
		t.Fatalf("UnresolvedDetails: want 2, got %d (%+v)", len(r.UnresolvedDetails), r.UnresolvedDetails)
	}
	// Each unresolved entry must have line number, name, and a non-empty reason.
	for _, u := range r.UnresolvedDetails {
		if u.LineNumber == 0 {
			t.Errorf("UnresolvedDetails entry %+v: LineNumber = 0; want 1-based source line", u)
		}
		if u.Name == "" {
			t.Errorf("UnresolvedDetails entry %+v: Name empty", u)
		}
		if u.Reason == "" {
			t.Errorf("UnresolvedDetails entry %+v: Reason empty", u)
		}
		if u.Section != "main" {
			t.Errorf("UnresolvedDetails entry %+v: Section = %q, want \"main\"", u, u.Section)
		}
	}
	// The two unresolved details should reference lines 3 and 5
	// (1-based, counting the COMMANDER directive at line 1).
	wantLines := map[int]bool{3: true, 5: true}
	for _, u := range r.UnresolvedDetails {
		if !wantLines[u.LineNumber] {
			t.Errorf("UnresolvedDetails: unexpected LineNumber %d (want 3 or 5); entry=%+v", u.LineNumber, u)
		}
		delete(wantLines, u.LineNumber)
	}
	for ln := range wantLines {
		t.Errorf("expected unresolved at line %d but not found in %+v", ln, r.UnresolvedDetails)
	}
}

// TestParseReport_FallbackResolvedDetected — a DFC front-face lookup
// (`1 Aang, Swift Savior`) misses meta directly but resolves via
// buildCard's face-match fallback. Report should classify this as
// LineStatusFallbackResolved, not LineStatusResolved.
func TestParseReport_FallbackResolvedDetected(t *testing.T) {
	// dfcMeta stores the canonical ` // ` form; front-face lookups
	// take the face-match path inside buildCard.
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Aang, Swift Savior
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, reportMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if td.ParseReport.FallbackResolved == 0 {
		t.Errorf("FallbackResolved = 0, want >= 1 (Aang front-face takes face-match fallback)")
	}
	// CardLines should reflect: Sol Ring = Resolved, Aang = Fallback.
	var aangStatus, solRingStatus LineStatus
	for _, cl := range td.CardLines {
		switch cl.Name {
		case "Aang, Swift Savior":
			aangStatus = cl.Status
		case "Sol Ring":
			solRingStatus = cl.Status
		}
	}
	if aangStatus != LineStatusFallbackResolved {
		t.Errorf("Aang Status = %v, want LineStatusFallbackResolved", aangStatus)
	}
	if solRingStatus != LineStatusResolved {
		t.Errorf("Sol Ring Status = %v, want LineStatusResolved", solRingStatus)
	}
}

// TestParseReport_DroppedLinesCounted — sideboard + signature spells +
// maybeboard lines are intentionally dropped; the report rolls them up
// into DroppedLines (NOT UnresolvedLines). Distinct categories so a
// 0% unresolved + 15-card sideboard is correctly classified as a
// healthy deck, not a broken one.
func TestParseReport_DroppedLinesCounted(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Lightning Bolt

Sideboard (3)
1 Counterspell
1 Forest
1 Mountain

Maybeboard (2)
1 Sol Ring
1 Lightning Bolt
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, reportMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := td.ParseReport
	if r.DroppedLines != 5 {
		t.Errorf("DroppedLines = %d, want 5 (3 sideboard + 2 maybeboard)", r.DroppedLines)
	}
	if td.SideboardCount != 3 {
		t.Errorf("SideboardCount = %d, want 3", td.SideboardCount)
	}
	if r.UnresolvedLines != 0 {
		t.Errorf("UnresolvedLines = %d, want 0 (dropped lines aren't unresolved)", r.UnresolvedLines)
	}
}

// TestParseReport_CoveragePercentRounding — coverage % math sanity
// check. 8 resolved + 2 unresolved (out of 10 resolvable lines) = 80%.
// Dropped lines don't enter the coverage denominator.
func TestParseReport_CoveragePercentRounding(t *testing.T) {
	r := ParseReport{ResolvedLines: 7, FallbackResolved: 1, UnresolvedLines: 2}
	got := r.CoveragePercent()
	want := 80.0 // (7+1) / (7+1+2) * 100
	if got != want {
		t.Errorf("CoveragePercent = %v, want %v", got, want)
	}
}

// TestParseReport_CoveragePercentEmptyDeck — divide-by-zero guard.
// Empty report returns 0%, not NaN/+Inf.
func TestParseReport_CoveragePercentEmptyDeck(t *testing.T) {
	r := ParseReport{}
	if got := r.CoveragePercent(); got != 0 {
		t.Errorf("CoveragePercent on empty report = %v, want 0", got)
	}
}

// TestPrintReport_OutputContainsKeySignals — PrintReport emits a
// human-readable summary. Verify the output contains the format,
// line counts, coverage %, and (when present) each unresolved
// detail's source line number + name.
func TestPrintReport_OutputContainsKeySignals(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Definitely Not Real
2 Forest
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, reportMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := td.PrintReport(&buf); err != nil {
		t.Fatalf("PrintReport: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Parse coverage report",
		"Format:",
		"Total lines:",
		"Resolved (clean):",
		"Resolved (fallback):",
		"Unresolved:",
		"Coverage:",
		"Unresolved details",
		"Definitely Not Real",
		"line 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintReport output missing %q; full output:\n%s", want, out)
		}
	}
}

// TestPrintReport_NilDeckHandled — defensive PrintReport on a nil
// receiver writes the header and returns cleanly. Drives the CLI
// path where ParseDeckFile may have returned an error and the caller
// still wants to print something.
func TestPrintReport_NilDeckHandled(t *testing.T) {
	var td *TournamentDeck
	var buf bytes.Buffer
	if err := td.PrintReport(&buf); err != nil {
		t.Fatalf("PrintReport on nil: %v", err)
	}
	if !strings.Contains(buf.String(), "no deck") {
		t.Errorf("nil-deck PrintReport missing diagnostic; output: %q", buf.String())
	}
}

// TestParseReport_LineStatusStringStable — the LineStatus.String()
// method is the column label used by the report formatter. Pin the
// stable strings so renderers don't regress.
func TestParseReport_LineStatusStringStable(t *testing.T) {
	cases := []struct {
		s    LineStatus
		want string
	}{
		{LineStatusResolved, "resolved"},
		{LineStatusFallbackResolved, "fallback"},
		{LineStatusUnresolved, "unresolved"},
		{LineStatusUnknown, "unknown"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("LineStatus(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

// TestParseReport_TotalLinesIncludesDropped — TotalLines is the sum
// of every card-shaped source line, including the dropped ones.
// Defends the invariant Total = Resolved + Fallback + Unresolved + Dropped.
func TestParseReport_TotalLinesIncludesDropped(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Lightning Bolt
1 Definitely Not Real
2 Forest

Sideboard (3)
1 Counterspell
1 Forest
1 Mountain
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, reportMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := td.ParseReport
	sum := r.ResolvedLines + r.FallbackResolved + r.UnresolvedLines + r.DroppedLines
	if r.TotalLines != sum {
		t.Errorf("TotalLines = %d, want sum-of-parts = %d (R=%d F=%d U=%d D=%d)",
			r.TotalLines, sum, r.ResolvedLines, r.FallbackResolved, r.UnresolvedLines, r.DroppedLines)
	}
}
