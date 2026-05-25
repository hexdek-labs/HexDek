package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// readCSV loads a written export back into [][]string so tests can
// assert on rows + cells without re-implementing CSV parsing.
func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	return rows
}

func TestCSVHeader_OrderAndColumns(t *testing.T) {
	got := csvHeader()
	want := []string{
		"name", "class", "type_line", "oracle_len",
		"parse_error_count", "parse_errors", "pattern", "oracle_text",
	}
	if len(got) != len(want) {
		t.Fatalf("header length: want %d, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("header[%d]: want %q, got %q", i, want[i], got[i])
		}
	}
}

func TestResultToCSVRecord_FieldsMatchHeader(t *testing.T) {
	r := result{
		Name:        "Mulldrifter",
		Class:       classEmptyAST,
		OracleText:  "Flying\nWhen Mulldrifter enters the battlefield, draw two cards.",
		TypeLine:    "Creature — Elemental",
		ParseErrors: nil,
	}
	rec := resultToCSVRecord(r)
	if len(rec) != len(csvHeader()) {
		t.Fatalf("record length must match header: want %d, got %d", len(csvHeader()), len(rec))
	}
	if rec[0] != "Mulldrifter" {
		t.Errorf("name col: got %q", rec[0])
	}
	if rec[1] != "EMPTY_AST" {
		t.Errorf("class col: got %q", rec[1])
	}
	if rec[2] != "Creature — Elemental" {
		t.Errorf("type_line col: got %q", rec[2])
	}
	if got, _ := strconv.Atoi(rec[3]); got != len(r.OracleText) {
		t.Errorf("oracle_len col: want %d, got %s", len(r.OracleText), rec[3])
	}
	if rec[4] != "0" {
		t.Errorf("parse_error_count col with no errors: got %q", rec[4])
	}
	if rec[5] != "" {
		t.Errorf("parse_errors col with no errors should be empty: got %q", rec[5])
	}
	// Oracle text must be newline-collapsed.
	if strings.ContainsAny(rec[7], "\r\n") {
		t.Errorf("oracle_text col must not contain newlines: %q", rec[7])
	}
}

func TestResultToCSVRecord_JoinsParseErrorsWithSemicolon(t *testing.T) {
	r := result{
		Name:        "Wacky",
		Class:       classPartial,
		ParseErrors: []string{"unknown_clause:choose_x", "missing_keyword:cascade"},
	}
	rec := resultToCSVRecord(r)
	if rec[4] != "2" {
		t.Errorf("parse_error_count: want 2, got %q", rec[4])
	}
	if rec[5] != "unknown_clause:choose_x; missing_keyword:cascade" {
		t.Errorf("parse_errors join: got %q", rec[5])
	}
}

func TestCsvOracleText_CollapsesAllWhitespace(t *testing.T) {
	in := "Line one\nLine two\r\nLine three\twith\ttabs"
	got := csvOracleText(in)
	if strings.ContainsAny(got, "\n\r\t") {
		t.Errorf("expected no newlines/tabs after collapse, got %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Errorf("expected runs of spaces collapsed, got %q", got)
	}
	if got != "Line one Line two Line three with tabs" {
		t.Errorf("collapse: got %q", got)
	}
}

func TestCsvOracleText_EmptyInputReturnsEmpty(t *testing.T) {
	if got := csvOracleText(""); got != "" {
		t.Errorf("empty input must return empty string, got %q", got)
	}
}

func TestCsvOracleText_TrimsLeadingTrailing(t *testing.T) {
	if got := csvOracleText("\n  hello  \n"); got != "hello" {
		t.Errorf("trim: got %q", got)
	}
}

func TestWriteCSV_EmitsHeaderAndUncoveredRowsOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	rs := fixture() // 12 MISSING, 10 EMPTY_AST, 3 PARTIAL, 4 OK, 1 OK_VANILLA → 25 uncovered

	if err := writeCSV(path, rs, false); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	rows := readCSV(t, path)
	if len(rows) < 1 {
		t.Fatal("expected at least the header row")
	}
	// Header
	if rows[0][0] != "name" {
		t.Errorf("first row should be header, got %v", rows[0])
	}
	// Row count: 25 uncovered + 1 header = 26
	if len(rows) != 26 {
		t.Fatalf("want 26 rows (1 header + 25 uncovered), got %d", len(rows))
	}
	// No OK rows.
	for _, row := range rows[1:] {
		switch row[1] {
		case "OK", "OK_VANILLA":
			t.Errorf("OK row leaked into uncovered-only export: %v", row)
		}
	}
}

func TestWriteCSV_IncludeOKAddsCoveredRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	rs := fixture()

	if err := writeCSV(path, rs, true); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	rows := readCSV(t, path)
	// 30 results + 1 header = 31
	if len(rows) != 31 {
		t.Fatalf("want 31 rows with includeOK, got %d", len(rows))
	}
	okCount := 0
	for _, row := range rows[1:] {
		if row[1] == "OK" || row[1] == "OK_VANILLA" {
			okCount++
		}
	}
	if okCount != 5 {
		t.Errorf("want 5 OK/OK_VANILLA rows (4+1 from fixture), got %d", okCount)
	}
}

func TestWriteCSV_EmptyResultsStillEmitsHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	if err := writeCSV(path, nil, false); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	rows := readCSV(t, path)
	if len(rows) != 1 {
		t.Fatalf("empty results should still yield exactly the header row, got %d", len(rows))
	}
	if rows[0][0] != "name" {
		t.Errorf("header missing, got %v", rows[0])
	}
}

func TestWriteCSV_QuotesEmbeddedCommasAndQuotes(t *testing.T) {
	// encoding/csv handles this — we just verify round-trip fidelity
	// so a future refactor that hand-rolls CSV output doesn't silently
	// regress.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	rs := []result{
		{
			Name:       `Strange "Quoted, Name" Card`,
			Class:      classEmptyAST,
			OracleText: `Says "Boo", then says "Bah".`,
			TypeLine:   "Artifact",
		},
	}
	if err := writeCSV(path, rs, false); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	rows := readCSV(t, path)
	if len(rows) != 2 {
		t.Fatalf("want 1 header + 1 row, got %d", len(rows))
	}
	if rows[1][0] != `Strange "Quoted, Name" Card` {
		t.Errorf("name round-trip: got %q", rows[1][0])
	}
	if rows[1][7] != `Says "Boo", then says "Bah".` {
		t.Errorf("oracle_text round-trip: got %q", rows[1][7])
	}
}

func TestWriteCSV_NewlinesInOracleAreCollapsedNotEmitted(t *testing.T) {
	// encoding/csv WILL faithfully encode embedded newlines inside
	// quoted fields if we let them through. The CSV column is more
	// useful as a single-line value (spreadsheet rendering, regex
	// filters); writeCSV must collapse them before handoff.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	rs := []result{
		{
			Name:       "Two Line Card",
			Class:      classEmptyAST,
			OracleText: "First line\nSecond line",
			TypeLine:   "Creature",
		},
	}
	if err := writeCSV(path, rs, false); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Two newlines total in a well-formed 1-data-row CSV: end of header
	// + end of data row. Any third newline means an embedded one
	// survived collapsing.
	if n := strings.Count(string(raw), "\n"); n != 2 {
		t.Errorf("expected exactly 2 newlines in file (header + row terminators), got %d:\n%s", n, raw)
	}
}

func TestWriteCSV_PatternColumnPopulated(t *testing.T) {
	// classifyPattern is the heuristic bucket — verify the CSV captures
	// SOMETHING in that column for an obviously matching card. We don't
	// pin the exact bucket name (that's classifyPattern's contract, not
	// CSV's) — only that the column is populated and non-default for
	// long text.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	rs := []result{
		{
			Name:       "Long Card",
			Class:      classEmptyAST,
			OracleText: strings.Repeat("Lorem ipsum dolor sit amet. ", 15), // >200 chars
			TypeLine:   "Sorcery",
		},
	}
	if err := writeCSV(path, rs, false); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	rows := readCSV(t, path)
	if len(rows) != 2 {
		t.Fatalf("want 1 header + 1 row, got %d", len(rows))
	}
	if rows[1][6] == "" {
		t.Error("pattern column should be populated for non-OK rows")
	}
}

func TestWriteCSV_OracleLenMatchesOriginalNotCollapsed(t *testing.T) {
	// oracle_len reports the original oracle text length (newlines
	// included). The collapsed/displayed text in the oracle_text
	// column is a rendering convenience; the length column should
	// reflect the underlying corpus value so spreadsheet filters
	// like "oracle_len > 200" map to the original Scryfall text.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	orig := "Line A\nLine B\nLine C"
	rs := []result{{Name: "X", Class: classEmptyAST, OracleText: orig, TypeLine: "Creature"}}
	if err := writeCSV(path, rs, false); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	rows := readCSV(t, path)
	if rows[1][3] != strconv.Itoa(len(orig)) {
		t.Errorf("oracle_len col: want %d (original length), got %s", len(orig), rows[1][3])
	}
}

func TestWriteCSV_BadPathReturnsError(t *testing.T) {
	// Writing into a non-existent directory must error; main.go
	// log.Fatal's on this, so the function contract is just to
	// propagate the underlying os.Create failure rather than panic.
	err := writeCSV("/this/path/definitely/does/not/exist/out.csv", fixture(), false)
	if err == nil {
		t.Fatal("expected error for unwritable path, got nil")
	}
}
