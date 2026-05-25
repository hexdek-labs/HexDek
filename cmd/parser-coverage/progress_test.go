package main

import (
	"encoding/xml"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ts builds a UTC timestamp from "YYYY-MM-DD". Local helper since the
// existing fixedTime() in history_test.go takes RFC3339 and we want
// shorter dates here.
func ts(t *testing.T, ymd string) time.Time {
	t.Helper()
	tt, err := time.Parse("2006-01-02", ymd)
	if err != nil {
		t.Fatalf("ts(%q): %v", ymd, err)
	}
	return tt.UTC()
}

func TestEntryToProgress_ComputesSuccessPct(t *testing.T) {
	e := HistoryEntry{
		Timestamp:   ts(t, "2026-05-24"),
		Label:       "r60",
		OracleTotal: 100,
		OK:          70,
		OKVanilla:   5,
		Missing:     10,
		EmptyAST:    12,
		Partial:     3,
	}
	p := entryToProgress(e)
	if p.SuccessPct != 75.0 {
		t.Errorf("SuccessPct: want 75.0, got %.4f", p.SuccessPct)
	}
	if p.OK != 75 {
		t.Errorf("OK should be OK+OK_VANILLA: want 75, got %d", p.OK)
	}
	if p.Uncovered != 25 {
		t.Errorf("Uncovered should be Missing+EmptyAST+Partial: want 25, got %d", p.Uncovered)
	}
	if p.Label != "r60" {
		t.Errorf("Label passthrough: got %q", p.Label)
	}
}

func TestEntryToProgress_ZeroTotalDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("entryToProgress panicked on zero total: %v", r)
		}
	}()
	p := entryToProgress(HistoryEntry{OracleTotal: 0})
	if p.SuccessPct != 0 {
		t.Errorf("zero total should yield 0%%, got %.4f", p.SuccessPct)
	}
}

func TestSparkline_EmptyReturnsEmpty(t *testing.T) {
	if got := sparkline(nil); got != "" {
		t.Errorf("nil: got %q", got)
	}
	if got := sparkline([]float64{}); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

func TestSparkline_FlatLineUsesMiddleBlock(t *testing.T) {
	got := sparkline([]float64{50, 50, 50})
	gotRunes := []rune(got)
	if len(gotRunes) != 3 {
		t.Fatalf("flat 3-point line should be 3 runes, got %q", got)
	}
	palette := []rune(sparkBraille)
	mid := palette[len(palette)/2]
	for _, r := range gotRunes {
		if r != mid {
			t.Errorf("flat line should render as middle block %q; got rune %q in %q", string(mid), string(r), got)
		}
	}
}

func TestSparkline_MapsExtremesToEndsOfPalette(t *testing.T) {
	// Min value should pick the lowest block; max value the highest.
	got := []rune(sparkline([]float64{0, 100}))
	palette := []rune(sparkBraille)
	if got[0] != palette[0] {
		t.Errorf("min should map to first palette rune (%q), got %q", string(palette[0]), string(got[0]))
	}
	if got[1] != palette[len(palette)-1] {
		t.Errorf("max should map to last palette rune (%q), got %q", string(palette[len(palette)-1]), string(got[1]))
	}
}

func TestSparkline_MonotonicSeriesRendersMonotonically(t *testing.T) {
	got := []rune(sparkline([]float64{10, 30, 50, 70, 90}))
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Errorf("ascending series should produce non-decreasing runes; got %q", string(got))
		}
	}
}

func TestSummarizeProgress_DeltaAndSpan(t *testing.T) {
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-01"), SuccessPct: 65, Label: "a"},
		{Timestamp: ts(t, "2026-05-15"), SuccessPct: 72, Label: "b"},
		{Timestamp: ts(t, "2026-05-24"), SuccessPct: 78, Label: "c"},
	}
	s := summarizeProgress(points)
	if math.Abs(s.DeltaPP-13.0) > 0.0001 {
		t.Errorf("DeltaPP: want 13.0, got %.4f", s.DeltaPP)
	}
	if s.Points != 3 {
		t.Errorf("Points: want 3, got %d", s.Points)
	}
	if s.Span != 23*24*time.Hour {
		t.Errorf("Span: want 23d, got %v", s.Span)
	}
	if s.First.Label != "a" || s.Last.Label != "c" {
		t.Errorf("First/Last: %+v", s)
	}
}

func TestSummarizeProgress_Empty(t *testing.T) {
	s := summarizeProgress(nil)
	if s.Points != 0 {
		t.Errorf("empty: got %d points", s.Points)
	}
}

func TestHumanDuration_Ranges(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "<1 minute"},
		{5 * time.Minute, "5 minutes"},
		{3 * time.Hour, "3 hours"},
		{2 * 24 * time.Hour, "2 days"},
		{45 * 24 * time.Hour, "45 days"},
		{90 * 24 * time.Hour, "3 months"},
	}
	for _, c := range cases {
		if got := humanDuration(c.d); got != c.want {
			t.Errorf("humanDuration(%v): want %q, got %q", c.d, c.want, got)
		}
	}
}

func TestRenderProgressMarkdown_HeadlineAndTable(t *testing.T) {
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-01"), Label: "r58", OracleTotal: 35000, OK: 22750, Uncovered: 12250, SuccessPct: 65.0},
		{Timestamp: ts(t, "2026-05-24"), Label: "r60", OracleTotal: 35708, OK: 27852, Uncovered: 7856, SuccessPct: 78.0},
	}
	md := string(renderProgressMarkdown(points, "Parser Coverage Progress"))
	for _, want := range []string{
		"# Parser Coverage Progress",
		"**65.00% → 78.00%**",
		"+13.00 pp",
		"23 days",
		"| # | When | Label | Oracle total | OK | Uncovered | Success% | Δ |",
		"| 1 | 2026-05-01 | r58 |",
		"| 2 | 2026-05-24 | r60 |",
		"+13.00 pp",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}
}

func TestRenderProgressMarkdown_EmptyHistory(t *testing.T) {
	md := string(renderProgressMarkdown(nil, "Parser Coverage Progress"))
	if !strings.Contains(md, "# Parser Coverage Progress") {
		t.Error("empty history should still emit title")
	}
	if !strings.Contains(md, "No history entries yet") {
		t.Error("empty history should explain how to start tracking")
	}
}

func TestRenderProgressMarkdown_LabelFallbackToDash(t *testing.T) {
	// Unlabeled entries (label == "") should render as "—" in the
	// table so the column doesn't go blank.
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-24"), Label: "", OracleTotal: 100, OK: 70, Uncovered: 30, SuccessPct: 70.0},
	}
	md := string(renderProgressMarkdown(points, "x"))
	if !strings.Contains(md, "| — |") {
		t.Errorf("empty label should render as '—' in the label column\n%s", md)
	}
}

func TestRenderProgressMarkdown_SinglePointSparklineDoesntCrash(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("single-point markdown panicked: %v", r)
		}
	}()
	points := []ProgressPoint{{Timestamp: ts(t, "2026-05-24"), SuccessPct: 70}}
	md := string(renderProgressMarkdown(points, "x"))
	if !strings.Contains(md, "70.00%") {
		t.Errorf("single-point summary should still print pct, got:\n%s", md)
	}
}

func TestRenderProgressSVG_WellFormedXML(t *testing.T) {
	// Defense against subtle escaping mistakes — verify the output
	// parses as XML (which is a superset of well-formed SVG).
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-01"), Label: "r58", SuccessPct: 65},
		{Timestamp: ts(t, "2026-05-24"), Label: "r60", SuccessPct: 78},
	}
	svg := renderProgressSVG(points, "Parser Coverage Progress")
	dec := xml.NewDecoder(strings.NewReader(string(svg)))
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("SVG is not well-formed XML: %v\n%s", err, svg)
		}
	}
}

func TestRenderProgressSVG_ContainsExpectedElements(t *testing.T) {
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-01"), Label: "r58", SuccessPct: 65},
		{Timestamp: ts(t, "2026-05-24"), Label: "r60", SuccessPct: 78},
	}
	svg := string(renderProgressSVG(points, "Parser Coverage Progress"))
	for _, want := range []string{
		`<svg `,
		`<polyline`,
		`<circle`,
		"Parser Coverage Progress",
		"65.00% → 78.00%",
		"+13.00 pp",
		"2026-05-01",
		"2026-05-24",
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG missing %q\n%s", want, svg)
		}
	}
}

func TestRenderProgressSVG_SinglePointCentersAndSkipsPolyline(t *testing.T) {
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-24"), Label: "r60", SuccessPct: 75},
	}
	svg := string(renderProgressSVG(points, "x"))
	if strings.Contains(svg, "<polyline") {
		t.Error("single-point series should NOT render a polyline (degenerate)")
	}
	if !strings.Contains(svg, "<circle") {
		t.Error("single-point series should still render the dot")
	}
}

func TestRenderProgressSVG_EmptyHistoryShowsMessage(t *testing.T) {
	svg := string(renderProgressSVG(nil, "x"))
	if !strings.Contains(svg, "no history entries") {
		t.Errorf("empty SVG should show placeholder text\n%s", svg)
	}
	if strings.Contains(svg, "<polyline") {
		t.Error("empty SVG should not contain a polyline")
	}
}

func TestRenderProgressSVG_EscapesXMLInTitle(t *testing.T) {
	// A label containing '<' or '&' must not break XML well-formedness.
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-24"), Label: "weird <label> & co", SuccessPct: 75},
	}
	svg := string(renderProgressSVG(points, "title with <bracket> & amp"))
	if !strings.Contains(svg, "&amp;") {
		t.Errorf("expected & to be escaped in SVG output:\n%s", svg)
	}
	if !strings.Contains(svg, "&lt;bracket&gt;") {
		t.Errorf("expected < and > to be escaped:\n%s", svg)
	}
	// Round-trip parse to confirm well-formedness.
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		_, err := dec.Token()
		if err != nil && err.Error() != "EOF" {
			t.Fatalf("escaped SVG didn't parse as XML: %v", err)
		}
		if err != nil {
			break
		}
	}
}

func TestRenderProgressSVG_FlatLineDoesntDivideByZero(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("flat-line SVG panicked: %v", r)
		}
	}()
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-22"), SuccessPct: 75},
		{Timestamp: ts(t, "2026-05-23"), SuccessPct: 75},
		{Timestamp: ts(t, "2026-05-24"), SuccessPct: 75},
	}
	svg := string(renderProgressSVG(points, "flat"))
	if !strings.Contains(svg, "<polyline") {
		t.Error("flat-but-multi-point series should still emit a polyline")
	}
}

func TestWriteProgressChart_DetectsSVGExt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.svg")
	points := []ProgressPoint{
		{Timestamp: ts(t, "2026-05-24"), SuccessPct: 78},
	}
	if err := writeProgressChart(path, points, "x"); err != nil {
		t.Fatalf("writeProgressChart svg: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(got), "<?xml") && !strings.Contains(string(got), "<svg") {
		t.Errorf(".svg ext should write SVG content, got: %s", got[:min(80, len(got))])
	}
}

func TestWriteProgressChart_DetectsMarkdownExt(t *testing.T) {
	for _, ext := range []string{".md", ".markdown"} {
		dir := t.TempDir()
		path := filepath.Join(dir, "out"+ext)
		points := []ProgressPoint{
			{Timestamp: ts(t, "2026-05-24"), Label: "r60", SuccessPct: 78},
		}
		if err := writeProgressChart(path, points, "x"); err != nil {
			t.Fatalf("writeProgressChart %s: %v", ext, err)
		}
		got, _ := os.ReadFile(path)
		if !strings.HasPrefix(string(got), "# ") {
			t.Errorf("%s ext should write markdown (starts with '# '), got: %s", ext, got[:min(40, len(got))])
		}
	}
}

func TestWriteProgressChart_RejectsUnsupportedExt(t *testing.T) {
	for _, ext := range []string{".txt", ".json", ""} {
		err := writeProgressChart("/tmp/nope"+ext, nil, "x")
		if err == nil {
			t.Errorf("ext %q should error", ext)
		}
		if err != nil && !strings.Contains(err.Error(), "unsupported chart format") {
			t.Errorf("ext %q: error should explain supported formats, got %v", ext, err)
		}
	}
}

func TestWriteProgressChart_BadPathReturnsError(t *testing.T) {
	err := writeProgressChart("/this/path/definitely/does/not/exist/out.svg", nil, "x")
	if err == nil {
		t.Fatal("expected error for unwritable path")
	}
}

func TestComputeProgress_PreservesOrder(t *testing.T) {
	// History entries are append-only chronological — computeProgress
	// must not reorder. Tagging entries lets us verify positional
	// correspondence end-to-end.
	entries := []HistoryEntry{
		{Label: "first", Timestamp: ts(t, "2026-05-01"), OracleTotal: 100, OK: 60},
		{Label: "second", Timestamp: ts(t, "2026-05-15"), OracleTotal: 100, OK: 70},
		{Label: "third", Timestamp: ts(t, "2026-05-24"), OracleTotal: 100, OK: 78},
	}
	points := computeProgress(entries)
	if len(points) != 3 {
		t.Fatalf("len: want 3, got %d", len(points))
	}
	if points[0].Label != "first" || points[1].Label != "second" || points[2].Label != "third" {
		t.Errorf("order not preserved: %+v", points)
	}
	if points[2].SuccessPct != 78.0 {
		t.Errorf("third point SuccessPct: want 78, got %.4f", points[2].SuccessPct)
	}
}

// min: Go 1.21+ has this built in; declared locally as a tiny helper
// for the file-content snippet bound in TestWriteProgressChart_*.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
