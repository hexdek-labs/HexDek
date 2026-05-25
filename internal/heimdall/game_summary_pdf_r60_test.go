package heimdall

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func sampleRichSummary() GameSummary {
	return GameSummary{
		GameID:     "4242",
		Winner:     1,
		WinnerName: "Atraxa",
		Turns:      9,
		EndReason:  "combat",
		CommanderZoneVisits: []CommanderZoneVisit{
			{Seat: 1, CommanderName: "Atraxa, Praetors' Voice", CastCount: 2, InZoneAtEnd: true, Visits: 3},
			{Seat: 0, CommanderName: "Krenko, Mob Boss", CastCount: 0, InZoneAtEnd: true, Visits: 1},
		},
		RegretCards: []RegretCard{
			{Seat: 2, CardName: "Plague Wind", CMC: 9, Reason: "stranded_in_hand"},
		},
		MVPCards: []MVPCard{
			{Seat: 1, CardName: "Atraxa, Praetors' Voice", CMC: 4, Score: 8, Counters: 4, IsCommander: true, TurnPlayed: 5},
		},
		TurningPoints: []TurningPoint{
			{Turn: 5, Kind: "commander_first_cast", Seat: 1, Description: `seat 1 cast "Atraxa, Praetors' Voice" on turn 5`},
			{Turn: 9, Kind: "game_end", Seat: 1, Description: "seat 1 won on turn 9"},
		},
		DataSource: "rich",
	}
}

// PDF starts with the canonical %PDF-1.4 header and ends with
// %%EOF. The xref table is present.
func TestRenderGameSummaryPDF_StructurallyValid(t *testing.T) {
	out := RenderGameSummaryPDF(sampleRichSummary())
	if !bytes.HasPrefix(out, []byte("%PDF-1.4")) {
		t.Errorf("output should start with %%PDF-1.4 header; first 16 bytes: %q", out[:16])
	}
	if !bytes.Contains(out, []byte("%%EOF")) {
		t.Errorf("output should contain %%EOF trailer marker")
	}
	if !bytes.Contains(out, []byte("xref")) {
		t.Errorf("output should contain xref table")
	}
	if !bytes.Contains(out, []byte("trailer")) {
		t.Errorf("output should contain trailer dictionary")
	}
	if !bytes.Contains(out, []byte("/Type /Catalog")) || !bytes.Contains(out, []byte("/Type /Pages")) {
		t.Errorf("output should declare Catalog + Pages objects")
	}
}

// All four R60 signal sections are present in the rendered text.
func TestRenderGameSummaryPDF_ContainsAllSignalSections(t *testing.T) {
	out := string(RenderGameSummaryPDF(sampleRichSummary()))
	wantSections := []string{
		"GAME SUMMARY",
		"GAME #4242",
		"COMMANDER ZONE VISITS",
		"REGRET CARDS",
		"MVP CARDS",
		"TURNING POINTS",
	}
	for _, s := range wantSections {
		if !strings.Contains(out, s) {
			t.Errorf("missing section %q in PDF output", s)
		}
	}
}

// Per-row data lands in the page content stream.
func TestRenderGameSummaryPDF_ContainsRowData(t *testing.T) {
	out := string(RenderGameSummaryPDF(sampleRichSummary()))
	wantStrings := []string{
		"Atraxa",
		"Krenko",
		"Plague Wind",
		"stranded_in_hand",
		"CMDR",
		"COMMANDER FIRST CAST",
		"GAME END",
	}
	for _, s := range wantStrings {
		if !strings.Contains(out, s) {
			t.Errorf("missing row content %q in PDF output", s)
		}
	}
}

// Empty signal arrays render an explanatory "no data" line for
// each section instead of dropping the section entirely.
func TestRenderGameSummaryPDF_EmptySignalsRenderEmptyStateLines(t *testing.T) {
	out := string(RenderGameSummaryPDF(GameSummary{
		GameID:     "1",
		Winner:     0,
		Turns:      1,
		EndReason:  "timeout",
		DataSource: "db_only",
	}))
	// Parens in source strings get escaped as \( \) inside the
	// content stream — match the substring inside the parens
	// rather than the literal form.
	wantEmpty := []string{
		"no commander zone activity recorded",
		"no stranded-in-hand cards",
		"no standout permanents at game end",
	}
	for _, s := range wantEmpty {
		if !strings.Contains(out, s) {
			t.Errorf("missing empty-state line %q", s)
		}
	}
}

// PDF metacharacters in card names (parens, backslashes) are
// escaped so the content stream stays parseable.
func TestRenderGameSummaryPDF_EscapesParensInStrings(t *testing.T) {
	s := GameSummary{
		GameID: "1",
		Winner: 0,
		Turns:  1,
		RegretCards: []RegretCard{
			{Seat: 0, CardName: "Card With (Parens) In Name", CMC: 3, Reason: "stranded_in_hand"},
		},
	}
	out := string(RenderGameSummaryPDF(s))
	// Escaped form lands in the content stream.
	if !strings.Contains(out, `\(Parens\)`) {
		t.Errorf("parens should be escaped as \\( \\); got output:\n%s",
			out)
	}
}

// Multi-page output: feed enough lines to overflow one page, then
// confirm Count > 1 in the Pages object.
func TestRenderGameSummaryPDF_PaginatesOnOverflow(t *testing.T) {
	s := GameSummary{GameID: "1", Winner: 0, Turns: 200, EndReason: "test"}
	// 120 regrets → far beyond pdfLinesPerPg.
	for i := 0; i < 120; i++ {
		s.RegretCards = append(s.RegretCards, RegretCard{
			Seat: 0, CardName: "Card", CMC: 3, Reason: "stranded_in_hand",
		})
	}
	out := string(RenderGameSummaryPDF(s))
	// /Count N — pull N out and ensure > 1.
	re := regexp.MustCompile(`/Count (\d+)`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatal("missing /Count in Pages object")
	}
	if m[1] == "1" {
		t.Errorf("Pages /Count should be > 1 for overflowing content; got %s", m[1])
	}
}

// Draw / no-winner summary renders without crashing and the
// WINNER row carries the DRAW label.
func TestRenderGameSummaryPDF_DrawGameRendersWithoutWinner(t *testing.T) {
	s := GameSummary{GameID: "1", Winner: -1, Turns: 80, EndReason: "timeout"}
	out := string(RenderGameSummaryPDF(s))
	if !strings.Contains(out, "DRAW") {
		t.Errorf("draw game should surface DRAW label in PDF")
	}
	if !strings.Contains(out, "TIMEOUT") {
		t.Errorf("end reason should appear uppercased: %q", out)
	}
}

// wrapLines splits long lines at word boundaries when possible.
func TestWrapLines_BreaksAtSpaceWithinCap(t *testing.T) {
	in := []string{"abcd efgh ijkl mnop qrst uvwx yz"}
	out := wrapLines(in, 15)
	if len(out) < 2 {
		t.Fatalf("want at least 2 wrapped lines; got %d (%+v)", len(out), out)
	}
	// First wrapped line shouldn't exceed 15 chars.
	if len(out[0]) > 15 {
		t.Errorf("wrapped first line should fit in 15 cols; got %d (%q)", len(out[0]), out[0])
	}
}

// Lines under the cap pass through untouched.
func TestWrapLines_ShortLinesPassThrough(t *testing.T) {
	in := []string{"short", "also short"}
	out := wrapLines(in, 50)
	if len(out) != len(in) {
		t.Errorf("short lines should pass through; got %d for %d input", len(out), len(in))
	}
}

// Output bytes parse cleanly when opened with a PDF reader (we
// can't run a reader in the test sandbox, but we can sanity-check
// that the xref offsets resolve to real `N 0 obj` headers).
func TestRenderGameSummaryPDF_XrefOffsetsPointToObjects(t *testing.T) {
	out := RenderGameSummaryPDF(sampleRichSummary())
	// Locate the xref block.
	xrefStart := bytes.Index(out, []byte("xref\n"))
	if xrefStart < 0 {
		t.Fatal("no xref section")
	}
	// Walk the xref entries; for each `NNNNNNNNNN 00000 n` row
	// (10-digit offset + " 00000 n "), confirm the offset
	// addresses a byte sequence starting with "<id> 0 obj".
	re := regexp.MustCompile(`(\d{10}) 00000 n `)
	matches := re.FindAllSubmatch(out[xrefStart:], -1)
	if len(matches) == 0 {
		t.Fatal("no xref n-records matched")
	}
	for i, m := range matches {
		offStr := string(m[1])
		off := 0
		for _, c := range offStr {
			off = off*10 + int(c-'0')
		}
		if off >= len(out) {
			t.Errorf("entry %d: offset %d beyond file length %d", i, off, len(out))
			continue
		}
		// Object id at this offset = i+1 (xref entry i corresponds
		// to object i+1; entry 0 is the free-list head).
		want := []byte{}
		if i+1 < 10 {
			want = []byte{byte('0' + i + 1), ' ', '0', ' ', 'o', 'b', 'j'}
		}
		if len(want) > 0 && !bytes.HasPrefix(out[off:], want) {
			t.Errorf("entry %d: offset %d does not lead to %q (got %q)",
				i, off, want, out[off:off+len(want)])
		}
	}
}
