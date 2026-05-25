package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedHistoryEntry builds a small HistoryEntry with sensible defaults
// — local helper to keep the test fixtures short.
func he(label, date string, ok, missing, empty, partial, total int) HistoryEntry {
	t, _ := time.Parse("2006-01-02", date)
	return HistoryEntry{
		Timestamp:   t.UTC(),
		Label:       label,
		OracleTotal: total,
		OK:          ok,
		OKVanilla:   0,
		Missing:     missing,
		EmptyAST:    empty,
		Partial:     partial,
	}
}

func TestComputeTrend_CardsGainedMath(t *testing.T) {
	from := he("r58", "2026-05-01", 27000, 3700, 3500, 1000, 35200)
	to := he("r60", "2026-05-24", 31000, 3700, 12, 1, 34713)
	tr := computeTrend(from, to)
	if tr.CardsGained != 4000 {
		t.Errorf("CardsGained: want 4000 (31000-27000), got %d", tr.CardsGained)
	}
}

func TestComputeTrend_AllDeltaSigns(t *testing.T) {
	// Build a case where each class moves in a distinct direction so
	// the sign handling is verified end-to-end.
	from := he("a", "2026-05-01", 100, 50, 30, 20, 200)
	to := he("b", "2026-05-24", 150, 40, 5, 25, 220)
	tr := computeTrend(from, to)
	if tr.MissingDelta != -10 {
		t.Errorf("MissingDelta: want -10, got %d", tr.MissingDelta)
	}
	if tr.EmptyASTDelta != -25 {
		t.Errorf("EmptyASTDelta: want -25, got %d", tr.EmptyASTDelta)
	}
	if tr.PartialDelta != 5 {
		t.Errorf("PartialDelta: want +5, got %d", tr.PartialDelta)
	}
	if tr.UncoveredDelta != -30 {
		// Missing+Empty+Partial: 100→70 (-30)
		t.Errorf("UncoveredDelta: want -30, got %d", tr.UncoveredDelta)
	}
	if tr.CardsGained != 50 {
		t.Errorf("CardsGained: want 50, got %d", tr.CardsGained)
	}
}

func TestComputeTrend_CoveragePPDelta(t *testing.T) {
	// from 50/100 = 50%; to 75/100 = 75% → +25.0 pp
	from := he("", "2026-05-01", 50, 25, 15, 10, 100)
	to := he("", "2026-05-24", 75, 10, 10, 5, 100)
	tr := computeTrend(from, to)
	if math.Abs(tr.CoveragePPDelta-25.0) > 0.001 {
		t.Errorf("CoveragePPDelta: want 25.0, got %.4f", tr.CoveragePPDelta)
	}
}

func TestComputeTrend_Span(t *testing.T) {
	from := he("", "2026-05-01", 0, 0, 0, 0, 0)
	to := he("", "2026-05-24", 0, 0, 0, 0, 0)
	tr := computeTrend(from, to)
	if tr.Span != 23*24*time.Hour {
		t.Errorf("Span: want 23 days, got %v", tr.Span)
	}
}

func TestComputeTrend_ZeroOracleTotalNoCrash(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("zero OracleTotal panicked: %v", r)
		}
	}()
	from := HistoryEntry{}
	to := HistoryEntry{}
	tr := computeTrend(from, to)
	if tr.CoveragePPDelta != 0 {
		t.Errorf("zero/zero: want 0 pp delta, got %.4f", tr.CoveragePPDelta)
	}
}

func TestSelectBaseline_DefaultIsOldest(t *testing.T) {
	entries := []HistoryEntry{
		he("first", "2026-04-01", 0, 0, 0, 0, 0),
		he("second", "2026-05-01", 0, 0, 0, 0, 0),
		he("third", "2026-05-24", 0, 0, 0, 0, 0),
	}
	for _, sel := range []string{"", "oldest"} {
		got, err := selectBaseline(entries, sel)
		if err != nil {
			t.Fatalf("selector %q: %v", sel, err)
		}
		if got.Label != "first" {
			t.Errorf("selector %q: want 'first', got %q", sel, got.Label)
		}
	}
}

func TestSelectBaseline_Previous(t *testing.T) {
	entries := []HistoryEntry{
		he("a", "2026-04-01", 0, 0, 0, 0, 0),
		he("b", "2026-05-01", 0, 0, 0, 0, 0),
		he("c", "2026-05-24", 0, 0, 0, 0, 0),
	}
	got, err := selectBaseline(entries, "previous")
	if err != nil {
		t.Fatalf("previous: %v", err)
	}
	if got.Label != "b" {
		t.Errorf("previous: want 'b' (second-to-last), got %q", got.Label)
	}
}

func TestSelectBaseline_PreviousRequiresTwoEntries(t *testing.T) {
	entries := []HistoryEntry{he("only", "2026-05-24", 0, 0, 0, 0, 0)}
	if _, err := selectBaseline(entries, "previous"); err == nil {
		t.Fatal("previous on a 1-entry history should error")
	}
}

func TestSelectBaseline_LabelMatch(t *testing.T) {
	entries := []HistoryEntry{
		he("r58", "2026-05-01", 0, 0, 0, 0, 0),
		he("r59", "2026-05-15", 0, 0, 0, 0, 0),
		he("r60", "2026-05-24", 0, 0, 0, 0, 0),
	}
	got, err := selectBaseline(entries, "r59")
	if err != nil {
		t.Fatalf("label r59: %v", err)
	}
	if got.Label != "r59" {
		t.Errorf("got %q", got.Label)
	}
}

func TestSelectBaseline_DateMatchesFirstOnOrAfter(t *testing.T) {
	entries := []HistoryEntry{
		he("a", "2026-04-01", 0, 0, 0, 0, 0),
		he("b", "2026-05-01", 0, 0, 0, 0, 0),
		he("c", "2026-05-24", 0, 0, 0, 0, 0),
	}
	got, err := selectBaseline(entries, "2026-04-15")
	if err != nil {
		t.Fatalf("date selector: %v", err)
	}
	// 2026-04-15 falls between a (2026-04-01) and b (2026-05-01);
	// "on or after" should pick b.
	if got.Label != "b" {
		t.Errorf("want b (first on or after 2026-04-15), got %q", got.Label)
	}
}

func TestSelectBaseline_DateBeforeAllEntriesPicksOldest(t *testing.T) {
	entries := []HistoryEntry{
		he("a", "2026-05-01", 0, 0, 0, 0, 0),
	}
	got, err := selectBaseline(entries, "2026-01-01")
	if err != nil {
		t.Fatalf("date before all: %v", err)
	}
	if got.Label != "a" {
		t.Errorf("date before all should still pick first entry, got %q", got.Label)
	}
}

func TestSelectBaseline_DateAfterAllEntriesErrors(t *testing.T) {
	entries := []HistoryEntry{
		he("a", "2026-04-01", 0, 0, 0, 0, 0),
	}
	if _, err := selectBaseline(entries, "2026-12-31"); err == nil {
		t.Fatal("date after all entries should error")
	}
}

func TestSelectBaseline_UnknownSelectorErrors(t *testing.T) {
	entries := []HistoryEntry{he("a", "2026-05-01", 0, 0, 0, 0, 0)}
	if _, err := selectBaseline(entries, "bogus-xyz"); err == nil {
		t.Fatal("nonexistent label should error")
	}
}

func TestSelectBaseline_EmptyHistoryErrors(t *testing.T) {
	if _, err := selectBaseline(nil, "oldest"); err == nil {
		t.Fatal("empty history should error")
	}
}

func TestRenderTrendMarkdown_LeadsWithCardsGained(t *testing.T) {
	from := he("r58", "2026-05-01", 27000, 3700, 3500, 1050, 35250)
	to := he("r60", "2026-05-24", 27142, 3700, 3400, 1008, 35250)
	md := string(renderTrendMarkdown(computeTrend(from, to)))
	if !strings.Contains(md, "**+142 cards** newly covered since `r58`") {
		t.Errorf("headline should match brief's 'we gained 142 cards' framing\n%s", md)
	}
	if !strings.Contains(md, "23 days ago") {
		t.Errorf("span should be in headline parenthetical\n%s", md)
	}
}

func TestRenderTrendMarkdown_IncludesAllClassRows(t *testing.T) {
	from := he("a", "2026-05-01", 100, 50, 30, 20, 200)
	to := he("b", "2026-05-24", 150, 40, 5, 25, 220)
	md := string(renderTrendMarkdown(computeTrend(from, to)))
	for _, want := range []string{
		"# Parser Coverage Trend",
		"Coverage:",
		"| Metric | a | b | Δ |",
		"| Covered (OK + OK_VANILLA) | 100 | 150 | +50 |",
		"| Uncovered total |",
		"| MISSING | 50 | 40 | -10 |",
		"| EMPTY_AST | 30 | 5 | -25 |",
		"| PARTIAL | 20 | 25 | +5 |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("trend report missing %q\n---\n%s", want, md)
		}
	}
}

func TestRenderTrendMarkdown_NegativeGainShownWithMinus(t *testing.T) {
	// Regression in the other direction — losing coverage.
	from := he("a", "2026-05-01", 200, 0, 0, 0, 200)
	to := he("b", "2026-05-24", 150, 50, 0, 0, 200)
	md := string(renderTrendMarkdown(computeTrend(from, to)))
	if !strings.Contains(md, "**-50 cards** newly covered") {
		t.Errorf("negative gain should render with explicit '-' sign\n%s", md)
	}
}

func TestRenderTrendMarkdown_UnlabeledEntriesUseDate(t *testing.T) {
	from := he("", "2026-05-01", 10, 5, 5, 0, 20)
	to := he("", "2026-05-24", 15, 0, 5, 0, 20)
	md := string(renderTrendMarkdown(computeTrend(from, to)))
	if !strings.Contains(md, "since `2026-05-01`") {
		t.Errorf("unlabeled from should fall back to date\n%s", md)
	}
	if !strings.Contains(md, "| Metric | 2026-05-01 | 2026-05-24 | Δ |") {
		t.Errorf("table header should use dates when labels are absent\n%s", md)
	}
}

func TestRenderTrendMarkdown_ZeroSpanOmitsAgoSuffix(t *testing.T) {
	// from and to share a timestamp (e.g., recorded twice in the same
	// run for debugging). Span == 0 should not produce the "N ago"
	// parenthetical because "0 minutes ago" is silly.
	now, _ := time.Parse("2006-01-02", "2026-05-24")
	from := HistoryEntry{Label: "a", Timestamp: now.UTC(), OracleTotal: 100, OK: 50}
	to := HistoryEntry{Label: "b", Timestamp: now.UTC(), OracleTotal: 100, OK: 75}
	md := string(renderTrendMarkdown(computeTrend(from, to)))
	if strings.Contains(md, " ago)") {
		t.Errorf("zero-span trend should not say '... ago'\n%s", md)
	}
	if !strings.Contains(md, "newly covered since `a`.") {
		t.Errorf("zero-span headline still names the baseline\n%s", md)
	}
}

func TestWriteTrendReport_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history.jsonl")
	for _, e := range []HistoryEntry{
		he("r58", "2026-05-01", 27000, 3700, 3500, 1050, 35250),
		he("r59", "2026-05-15", 29500, 3700, 1100, 1050, 35350),
		he("r60", "2026-05-24", 31601, 3745, 12, 1, 35708),
	} {
		b, _ := json.Marshal(e)
		f, _ := os.OpenFile(histPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		f.Write(b)
		f.Write([]byte("\n"))
		f.Close()
	}
	out := filepath.Join(dir, "trend.md")
	if err := writeTrendReport(out, histPath, "r58"); err != nil {
		t.Fatalf("writeTrendReport: %v", err)
	}
	body, _ := os.ReadFile(out)
	md := string(body)
	if !strings.Contains(md, "newly covered since `r58`") {
		t.Errorf("end-to-end report should reference baseline label\n%s", md)
	}
	if !strings.Contains(md, "| r58 | r60 |") {
		t.Errorf("table header should pair baseline and current labels\n%s", md)
	}
}

func TestWriteTrendReport_EmptyHistoryErrors(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history.jsonl")
	os.WriteFile(histPath, []byte(""), 0o644)
	err := writeTrendReport(filepath.Join(dir, "out.md"), histPath, "oldest")
	if err == nil {
		t.Fatal("empty history should error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty history, got: %v", err)
	}
}

func TestWriteTrendReport_UnknownBaselineErrors(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history.jsonl")
	e, _ := json.Marshal(he("r60", "2026-05-24", 100, 0, 0, 0, 100))
	os.WriteFile(histPath, append(e, '\n'), 0o644)
	err := writeTrendReport(filepath.Join(dir, "out.md"), histPath, "nonexistent")
	if err == nil {
		t.Fatal("unknown baseline should error")
	}
}

func TestWriteTrendReport_StdoutWhenDashOrEmpty(t *testing.T) {
	// Spot-check that path="-" and path="" both route to stdout
	// without writing a file. We don't capture stdout; we just verify
	// no error and no on-disk artifact.
	dir := t.TempDir()
	histPath := filepath.Join(dir, "h.jsonl")
	a, _ := json.Marshal(he("a", "2026-05-01", 50, 50, 0, 0, 100))
	b, _ := json.Marshal(he("b", "2026-05-24", 75, 25, 0, 0, 100))
	os.WriteFile(histPath, append(append(a, '\n'), append(b, '\n')...), 0o644)

	// Redirect stdout to a temp file so the test isn't noisy.
	origStdout := os.Stdout
	stdoutFile, _ := os.CreateTemp(dir, "stdout-*")
	os.Stdout = stdoutFile
	defer func() {
		os.Stdout = origStdout
		stdoutFile.Close()
	}()

	for _, p := range []string{"", "-"} {
		if err := writeTrendReport(p, histPath, "oldest"); err != nil {
			t.Errorf("writeTrendReport(%q): %v", p, err)
		}
	}
	stdoutFile.Seek(0, 0)
	body, _ := os.ReadFile(stdoutFile.Name())
	if !strings.Contains(string(body), "# Parser Coverage Trend") {
		t.Errorf("stdout path didn't receive the report")
	}
}
