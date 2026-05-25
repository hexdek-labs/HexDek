package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedTime returns a stable timestamp for deterministic test entries.
// Using UTC explicitly because resultsToHistoryEntry normalizes there
// — comparisons against the input must do the same.
func fixedTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse fixed time: %v", err)
	}
	return tt.UTC()
}

func TestResultsToHistoryEntry_CountsByClass(t *testing.T) {
	rs := fixture() // 12 MISSING, 10 EMPTY_AST, 3 PARTIAL, 4 OK, 1 OK_VANILLA
	now := fixedTime(t, "2026-05-24T12:00:00Z")
	e := resultsToHistoryEntry(rs, 30, 26, 7, "r60", now)

	if e.Label != "r60" {
		t.Errorf("Label: got %q", e.Label)
	}
	if !e.Timestamp.Equal(now) {
		t.Errorf("Timestamp: got %v, want %v", e.Timestamp, now)
	}
	if e.OracleTotal != 30 || e.CorpusTotal != 26 || e.ParseWarnings != 7 {
		t.Errorf("totals: got oracle=%d corpus=%d warnings=%d", e.OracleTotal, e.CorpusTotal, e.ParseWarnings)
	}
	if e.Missing != 12 || e.EmptyAST != 10 || e.Partial != 3 || e.OK != 4 || e.OKVanilla != 1 {
		t.Errorf("class counts: got missing=%d empty=%d partial=%d ok=%d vanilla=%d",
			e.Missing, e.EmptyAST, e.Partial, e.OK, e.OKVanilla)
	}
}

func TestResultsToHistoryEntry_NormalizesToUTC(t *testing.T) {
	// Pacific time input must be normalized to UTC so JSON-serialized
	// timestamps from different developer machines compare directly.
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip("time zone DB unavailable")
	}
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, loc)
	e := resultsToHistoryEntry(nil, 0, 0, 0, "", now)
	if e.Timestamp.Location() != time.UTC {
		t.Errorf("Timestamp must be in UTC, got %s", e.Timestamp.Location())
	}
}

func TestAppendHistory_CreatesFileWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	e := HistoryEntry{Label: "first", Timestamp: fixedTime(t, "2026-05-24T12:00:00Z"), OK: 100}
	if err := appendHistory(path, e); err != nil {
		t.Fatalf("appendHistory: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"label":"first"`) {
		t.Errorf("file content missing label: %s", got)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Errorf("file must end with newline for JSONL append safety, got %q", got)
	}
}

func TestAppendHistory_AppendsToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	a := HistoryEntry{Label: "a", Timestamp: fixedTime(t, "2026-05-22T00:00:00Z"), OK: 1}
	b := HistoryEntry{Label: "b", Timestamp: fixedTime(t, "2026-05-24T00:00:00Z"), OK: 2}
	if err := appendHistory(path, a); err != nil {
		t.Fatal(err)
	}
	if err := appendHistory(path, b); err != nil {
		t.Fatal(err)
	}
	entries, err := readHistory(path)
	if err != nil {
		t.Fatalf("readHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Label != "a" || entries[1].Label != "b" {
		t.Errorf("append order broken: %+v", entries)
	}
}

func TestReadHistory_MissingFileReturnsEmpty(t *testing.T) {
	entries, err := readHistory(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if entries != nil {
		t.Errorf("want nil slice for missing file, got %v", entries)
	}
}

func TestReadHistory_SkipsBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	e1, _ := json.Marshal(HistoryEntry{Label: "a"})
	e2, _ := json.Marshal(HistoryEntry{Label: "b"})
	// Interleave blank lines, leading/trailing whitespace, multiple
	// consecutive blanks. readHistory must shrug them all off.
	content := string(e1) + "\n\n   \n" + string(e2) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := readHistory(path)
	if err != nil {
		t.Fatalf("readHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 non-blank entries, got %d", len(entries))
	}
}

func TestReadHistory_BadJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	good, _ := json.Marshal(HistoryEntry{Label: "ok"})
	// Good entry first, then a malformed one — error message should
	// pinpoint the bad line.
	content := string(good) + "\n{not valid json}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readHistory(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error should mention line 2 (the bad one), got %q", err.Error())
	}
}

func TestHistoryEntry_JSONRoundTrip(t *testing.T) {
	// Field tags + JSON encoding must preserve all stats. Regression
	// guard against accidentally renaming a tag in the struct.
	want := HistoryEntry{
		Timestamp:     fixedTime(t, "2026-05-24T12:00:00Z"),
		Label:         "r60",
		OracleTotal:   35708,
		CorpusTotal:   31963,
		ParseWarnings: 104,
		OK:            28000,
		OKVanilla:     5,
		Missing:       1200,
		EmptyAST:      2200,
		Partial:       300,
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got HistoryEntry
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
	// Snake_case keys (matches CSV / jq workflows).
	for _, key := range []string{"timestamp", "label", "oracle_total", "corpus_total", "ok", "ok_vanilla", "missing", "empty_ast", "partial"} {
		if !strings.Contains(string(b), `"`+key+`":`) {
			t.Errorf("expected JSON key %q in output: %s", key, b)
		}
	}
}

func TestHistoryEntry_OmitsLabelWhenEmpty(t *testing.T) {
	// `omitempty` on Label keeps unlabeled baseline rows tidy.
	b, _ := json.Marshal(HistoryEntry{OK: 1})
	if strings.Contains(string(b), `"label"`) {
		t.Errorf("empty label should be omitted, got %s", b)
	}
}

func TestComputeDelta_DiffsAllFields(t *testing.T) {
	prev := HistoryEntry{
		Label: "r59", Timestamp: fixedTime(t, "2026-05-22T00:00:00Z"),
		OracleTotal: 100, CorpusTotal: 90, ParseWarnings: 5,
		OK: 70, OKVanilla: 5, Missing: 10, EmptyAST: 12, Partial: 3,
	}
	cur := HistoryEntry{
		Label: "r60", Timestamp: fixedTime(t, "2026-05-24T00:00:00Z"),
		OracleTotal: 105, CorpusTotal: 92, ParseWarnings: 4,
		OK: 75, OKVanilla: 5, Missing: 8, EmptyAST: 14, Partial: 3,
	}
	d := computeDelta(prev, cur)
	if d.OracleTotal != 5 {
		t.Errorf("OracleTotal delta: want 5, got %d", d.OracleTotal)
	}
	if d.CorpusTotal != 2 {
		t.Errorf("CorpusTotal delta: want 2, got %d", d.CorpusTotal)
	}
	if d.ParseWarnings != -1 {
		t.Errorf("ParseWarnings delta: want -1, got %d", d.ParseWarnings)
	}
	if d.OK != 5 {
		t.Errorf("OK delta: want 5, got %d", d.OK)
	}
	if d.OKVanilla != 0 {
		t.Errorf("OKVanilla delta: want 0, got %d", d.OKVanilla)
	}
	if d.Missing != -2 {
		t.Errorf("Missing delta: want -2, got %d", d.Missing)
	}
	if d.EmptyAST != 2 {
		t.Errorf("EmptyAST delta: want 2, got %d", d.EmptyAST)
	}
	if d.Partial != 0 {
		t.Errorf("Partial delta: want 0, got %d", d.Partial)
	}
	if d.Prev.Label != "r59" || d.Cur.Label != "r60" {
		t.Errorf("delta should retain prev/cur for rendering, got %+v", d)
	}
}

func TestSigned_FormatsZeroAndNonzero(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{5, "+5"},
		{-3, "-3"},
		{1000, "+1000"},
	}
	for _, c := range cases {
		if got := signed(c.in); got != c.want {
			t.Errorf("signed(%d): want %q, got %q", c.in, c.want, got)
		}
	}
}

func TestRenderHistoryDelta_IncludesAllClassesWithSignedFormat(t *testing.T) {
	prev := HistoryEntry{
		Label: "r59", Timestamp: fixedTime(t, "2026-05-22T18:00:00Z"),
		OK: 28341, OKVanilla: 5, Missing: 1265, EmptyAST: 2200, Partial: 293,
	}
	cur := HistoryEntry{
		Label: "r60", Timestamp: fixedTime(t, "2026-05-24T12:00:00Z"),
		OK: 28353, OKVanilla: 5, Missing: 1263, EmptyAST: 2190, Partial: 293,
	}
	out := renderHistoryDelta(computeDelta(prev, cur))
	for _, want := range []string{
		"delta vs previous (r59 @ 2026-05-22T18:00:00Z)",
		"OK         +12",
		"OK_VANILLA 0",
		"MISSING    -2",
		"EMPTY_AST  -10",
		"PARTIAL    0",
		"(28341 → 28353)",
		"(2200 → 2190)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q\n---\n%s---", want, out)
		}
	}
}

func TestRenderHistoryDelta_UnlabeledPrevShowsPlaceholder(t *testing.T) {
	prev := HistoryEntry{Timestamp: fixedTime(t, "2026-05-22T00:00:00Z")}
	cur := HistoryEntry{Label: "r60", Timestamp: fixedTime(t, "2026-05-24T00:00:00Z")}
	out := renderHistoryDelta(computeDelta(prev, cur))
	if !strings.Contains(out, "(unlabeled) @") {
		t.Errorf("expected '(unlabeled)' placeholder for empty prev label, got:\n%s", out)
	}
}

func TestAppendHistory_RoundTripPreservesData(t *testing.T) {
	// End-to-end: append → read → confirm exact equality. This is the
	// invariant the rest of the tool depends on (delta computation
	// against the last entry).
	path := filepath.Join(t.TempDir(), "history.jsonl")
	want := HistoryEntry{
		Timestamp:   fixedTime(t, "2026-05-24T12:00:00Z"),
		Label:       "r60-test",
		OracleTotal: 100, CorpusTotal: 90, ParseWarnings: 3,
		OK: 70, OKVanilla: 5, Missing: 10, EmptyAST: 12, Partial: 3,
	}
	if err := appendHistory(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}
