package main

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// setFixture builds a multi-set result slice with known per-set counts.
// Set A: 4 uncovered (2 missing, 1 empty, 1 partial), 6 covered (5 OK + 1 OKV) — 10 total
// Set B: 3 uncovered (3 empty), 7 covered (7 OK) — 10 total
// Set C: 1 uncovered (1 missing), 0 covered — 1 total
// "(unknown)": 1 uncovered (1 empty), 0 covered — 1 total (no set name)
func setFixture() []result {
	var rs []result
	// Set A
	rs = append(rs,
		result{Name: "A1", Class: classMissing, SetName: "Alpha"},
		result{Name: "A2", Class: classMissing, SetName: "Alpha"},
		result{Name: "A3", Class: classEmptyAST, SetName: "Alpha"},
		result{Name: "A4", Class: classPartial, SetName: "Alpha"},
		result{Name: "A5", Class: classOK, SetName: "Alpha"},
		result{Name: "A6", Class: classOK, SetName: "Alpha"},
		result{Name: "A7", Class: classOK, SetName: "Alpha"},
		result{Name: "A8", Class: classOK, SetName: "Alpha"},
		result{Name: "A9", Class: classOK, SetName: "Alpha"},
		result{Name: "A10", Class: classOKVanilla, SetName: "Alpha"},
	)
	// Set B
	for i := 1; i <= 3; i++ {
		rs = append(rs, result{Name: "B-empty-" + string(rune('A'+i)), Class: classEmptyAST, SetName: "Beta"})
	}
	for i := 1; i <= 7; i++ {
		rs = append(rs, result{Name: "B-ok-" + string(rune('A'+i)), Class: classOK, SetName: "Beta"})
	}
	// Set C
	rs = append(rs, result{Name: "C1", Class: classMissing, SetName: "Charlie"})
	// No set name (legacy / fixture-built)
	rs = append(rs, result{Name: "lost", Class: classEmptyAST, SetName: ""})
	return rs
}

func TestGroupBySet_AggregatesEachClassCorrectly(t *testing.T) {
	groups := groupBySet(setFixture())

	m := map[string]SetGroup{}
	for _, g := range groups {
		m[g.SetName] = g
	}

	a := m["Alpha"]
	if a.Total != 10 || a.Uncovered != 4 || a.Missing != 2 || a.EmptyAST != 1 || a.Partial != 1 || a.OK != 5 || a.OKVanilla != 1 {
		t.Errorf("Alpha aggregate: got %+v", a)
	}

	b := m["Beta"]
	if b.Total != 10 || b.Uncovered != 3 || b.EmptyAST != 3 || b.OK != 7 {
		t.Errorf("Beta aggregate: got %+v", b)
	}

	c := m["Charlie"]
	if c.Total != 1 || c.Uncovered != 1 || c.Missing != 1 {
		t.Errorf("Charlie aggregate: got %+v", c)
	}

	unk, ok := m["(unknown)"]
	if !ok {
		t.Fatalf("missing-set entry should bucket as '(unknown)', got groups: %+v", groups)
	}
	if unk.Total != 1 || unk.Uncovered != 1 || unk.EmptyAST != 1 {
		t.Errorf("(unknown) aggregate: got %+v", unk)
	}
}

func TestGroupBySet_SortsByUncoveredDescending(t *testing.T) {
	groups := groupBySet(setFixture())
	uncoveredOrder := make([]int, len(groups))
	for i, g := range groups {
		uncoveredOrder[i] = g.Uncovered
	}
	if !sort.IsSorted(sort.Reverse(sort.IntSlice(uncoveredOrder))) {
		t.Errorf("groups must be sorted by Uncovered descending, got %v", uncoveredOrder)
	}
	// Alpha (4) must come before Beta (3) must come before Charlie/(unknown) (1).
	if groups[0].SetName != "Alpha" {
		t.Errorf("Alpha (4 uncovered) should be first, got %q", groups[0].SetName)
	}
	if groups[1].SetName != "Beta" {
		t.Errorf("Beta (3 uncovered) should be second, got %q", groups[1].SetName)
	}
}

func TestGroupBySet_TiebreaksByNameAscending(t *testing.T) {
	// Charlie and (unknown) both have Uncovered=1. The tiebreak is
	// SetName ascending — '(' (0x28) sorts before 'C' (0x43), so
	// "(unknown)" must come first.
	groups := groupBySet(setFixture())
	var tied []string
	for _, g := range groups {
		if g.Uncovered == 1 {
			tied = append(tied, g.SetName)
		}
	}
	if len(tied) != 2 {
		t.Fatalf("expected 2 sets tied at uncovered=1, got %v", tied)
	}
	if tied[0] != "(unknown)" || tied[1] != "Charlie" {
		t.Errorf("tiebreak should be alphabetic asc: want [(unknown), Charlie], got %v", tied)
	}
}

func TestSetGroup_CoveragePct(t *testing.T) {
	g := SetGroup{Total: 10, OK: 5, OKVanilla: 1}
	if math.Abs(g.CoveragePct()-60.0) > 0.0001 {
		t.Errorf("coverage 6/10: want 60.0, got %.4f", g.CoveragePct())
	}
	g = SetGroup{Total: 4, OK: 4}
	if g.CoveragePct() != 100 {
		t.Errorf("coverage 4/4: want 100, got %.4f", g.CoveragePct())
	}
	g = SetGroup{Total: 10}
	if g.CoveragePct() != 0 {
		t.Errorf("coverage 0/10: want 0, got %.4f", g.CoveragePct())
	}
}

func TestSetGroup_CoveragePct_ZeroTotalNoDivisionPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CoveragePct must not panic on Total=0, got %v", r)
		}
	}()
	g := SetGroup{Total: 0}
	if g.CoveragePct() != 0 {
		t.Errorf("Total=0 should yield 0%%, got %.4f", g.CoveragePct())
	}
}

func TestGroupBySet_EmptyInput(t *testing.T) {
	groups := groupBySet(nil)
	if len(groups) != 0 {
		t.Errorf("empty input should yield 0 groups, got %d", len(groups))
	}
}

func TestWriteBySetReport_EmitsAllSetsByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "by-set.md")
	groups := groupBySet(setFixture())
	if err := writeBySetReport(path, groups, 0); err != nil {
		t.Fatalf("writeBySetReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	for _, want := range []string{
		"# Parser Coverage by Set",
		"| Rank | Set | Total | Uncovered | Coverage | MISSING | EMPTY_AST | PARTIAL |",
		"Alpha",
		"Beta",
		"Charlie",
		"(unknown)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestWriteBySetReport_TopNLimitsRowsButKeepsHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "by-set.md")
	groups := groupBySet(setFixture())
	if err := writeBySetReport(path, groups, 2); err != nil {
		t.Fatalf("writeBySetReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	if !strings.Contains(out, "Alpha") {
		t.Error("top-2 should include Alpha (highest uncovered)")
	}
	if !strings.Contains(out, "Beta") {
		t.Error("top-2 should include Beta (second-highest)")
	}
	if strings.Contains(out, "| Charlie ") || strings.Contains(out, "| (unknown) ") {
		t.Error("top-2 must NOT include tail sets Charlie or (unknown)")
	}
	if !strings.Contains(out, "Sets shown (top-N)") {
		t.Error("summary should call out top-N truncation when active")
	}
}

func TestWriteBySetReport_CoveragePctRenderedToOneDecimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "by-set.md")
	// Alpha: 6/10 covered = 60.0%
	groups := groupBySet(setFixture())
	if err := writeBySetReport(path, groups, 0); err != nil {
		t.Fatalf("writeBySetReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "60.0%") {
		t.Errorf("expected '60.0%%' in Alpha row, got:\n%s", got)
	}
}

func TestWriteBySetReport_EmptyGroupsStillEmitsHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "by-set.md")
	if err := writeBySetReport(path, nil, 0); err != nil {
		t.Fatalf("writeBySetReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	if !strings.Contains(out, "# Parser Coverage by Set") {
		t.Error("empty groups should still emit the header")
	}
	if !strings.Contains(out, "| Rank |") {
		t.Error("empty groups should still emit table header for consistent shape")
	}
}

func TestWriteBySetReport_BadPathReturnsError(t *testing.T) {
	err := writeBySetReport("/this/path/definitely/does/not/exist/by-set.md", nil, 0)
	if err == nil {
		t.Fatal("expected error for unwritable path, got nil")
	}
}

func TestFormatBySetSummary_TopNFormatting(t *testing.T) {
	groups := groupBySet(setFixture())
	got := formatBySetSummary(groups, 2)
	if !strings.Contains(got, "Alpha=4") {
		t.Errorf("summary should report Alpha=4, got %q", got)
	}
	if !strings.Contains(got, "Beta=3") {
		t.Errorf("summary should report Beta=3, got %q", got)
	}
	if strings.Contains(got, "Charlie") {
		t.Errorf("n=2 should not include Charlie, got %q", got)
	}
}

func TestFormatBySetSummary_HandlesEmpty(t *testing.T) {
	if got := formatBySetSummary(nil, 5); !strings.Contains(got, "no uncovered") {
		t.Errorf("empty groups should say 'no uncovered cards', got %q", got)
	}
}

func TestFormatBySetSummary_NCapsAtLength(t *testing.T) {
	// Asking for more than len(groups) must clamp, not panic.
	groups := groupBySet(setFixture())
	got := formatBySetSummary(groups, 999)
	if !strings.Contains(got, "(unknown)") {
		t.Errorf("n>len should include all groups, got %q", got)
	}
}
