package main

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// typeFixture mirrors setFixture's shape but keyed on TypeLine so
// the groupByType bucketing is exercised across the actual primary
// types we care about. Counts per type:
//
//	Creature: 10 total (4 uncovered: 2 missing, 1 empty, 1 partial; 6 covered)
//	Instant:  6 total  (2 uncovered: 2 empty; 4 OK)
//	Sorcery:  4 total  (1 uncovered: 1 missing; 3 OK)
//	Land:     2 total  (0 uncovered; 2 OK_VANILLA)
//	(none):   1 total  (1 empty; empty type line)
func typeFixture() []result {
	var rs []result
	// Creatures
	rs = append(rs,
		result{Name: "C1", Class: classMissing, TypeLine: "Creature — Elf"},
		result{Name: "C2", Class: classMissing, TypeLine: "Legendary Creature — Wizard"},
		result{Name: "C3", Class: classEmptyAST, TypeLine: "Creature — Goblin"},
		result{Name: "C4", Class: classPartial, TypeLine: "Creature — Dragon"},
	)
	for i := 5; i <= 10; i++ {
		rs = append(rs, result{Name: "CR-OK-" + string(rune('A'+i)), Class: classOK, TypeLine: "Creature"})
	}
	// Instants
	rs = append(rs,
		result{Name: "I1", Class: classEmptyAST, TypeLine: "Instant"},
		result{Name: "I2", Class: classEmptyAST, TypeLine: "Instant"},
	)
	for i := 1; i <= 4; i++ {
		rs = append(rs, result{Name: "I-OK-" + string(rune('A'+i)), Class: classOK, TypeLine: "Instant"})
	}
	// Sorceries
	rs = append(rs, result{Name: "S1", Class: classMissing, TypeLine: "Sorcery"})
	for i := 1; i <= 3; i++ {
		rs = append(rs, result{Name: "S-OK-" + string(rune('A'+i)), Class: classOK, TypeLine: "Sorcery"})
	}
	// Lands (basic + non-basic, both vanilla / OK)
	rs = append(rs,
		result{Name: "L1", Class: classOKVanilla, TypeLine: "Basic Land — Forest"},
		result{Name: "L2", Class: classOKVanilla, TypeLine: "Basic Land — Mountain"},
	)
	// Empty type line — should bucket as (none)
	rs = append(rs, result{Name: "X1", Class: classEmptyAST, TypeLine: ""})
	return rs
}

func TestGroupByType_BucketsByPrimaryType(t *testing.T) {
	groups := groupByType(typeFixture())
	m := map[string]TypeGroup{}
	for _, g := range groups {
		m[g.TypeName] = g
	}

	c := m["Creature"]
	if c.Total != 10 || c.Uncovered != 4 || c.Missing != 2 || c.EmptyAST != 1 || c.Partial != 1 || c.OK != 6 {
		t.Errorf("Creature aggregate: got %+v", c)
	}

	i := m["Instant"]
	if i.Total != 6 || i.Uncovered != 2 || i.EmptyAST != 2 || i.OK != 4 {
		t.Errorf("Instant aggregate: got %+v", i)
	}

	s := m["Sorcery"]
	if s.Total != 4 || s.Uncovered != 1 || s.Missing != 1 || s.OK != 3 {
		t.Errorf("Sorcery aggregate: got %+v", s)
	}

	l := m["Land"]
	if l.Total != 2 || l.Uncovered != 0 || l.OKVanilla != 2 {
		t.Errorf("Land aggregate: got %+v", l)
	}

	none := m["(none)"]
	if none.Total != 1 || none.Uncovered != 1 || none.EmptyAST != 1 {
		t.Errorf("(none) bucket should capture empty-type-line cards, got %+v", none)
	}
}

func TestGroupByType_StripsLegendarySupertype(t *testing.T) {
	// "Legendary Creature — X" should still bucket as Creature, not
	// Legendary. Defends against accidentally dropping the supertype
	// rule from primaryType.
	rs := []result{
		{Name: "L1", Class: classMissing, TypeLine: "Legendary Creature — Wizard"},
	}
	groups := groupByType(rs)
	if len(groups) != 1 || groups[0].TypeName != "Creature" {
		t.Errorf("Legendary should be stripped; got %+v", groups)
	}
}

func TestGroupByType_ArtifactCreatureBucketsAsArtifact(t *testing.T) {
	// Hangarback-Walker shape: documented behavior is bucket-by-first.
	// Test pins this so a future change to primaryType doesn't
	// silently re-route the bucketing without updating the docs.
	rs := []result{
		{Name: "Hangarback", Class: classMissing, TypeLine: "Artifact Creature — Construct"},
	}
	groups := groupByType(rs)
	if len(groups) != 1 || groups[0].TypeName != "Artifact" {
		t.Errorf("Artifact Creature should bucket as Artifact (first non-supertype), got %+v", groups)
	}
}

func TestGroupByType_SortsByUncoveredDescending(t *testing.T) {
	groups := groupByType(typeFixture())
	uncovered := make([]int, len(groups))
	for i, g := range groups {
		uncovered[i] = g.Uncovered
	}
	if !sort.IsSorted(sort.Reverse(sort.IntSlice(uncovered))) {
		t.Errorf("groups must be sorted by Uncovered desc, got %v", uncovered)
	}
	// Creature (4 uncovered) > Instant (2) > Sorcery (1) ≈ (none) (1)
	// > Land (0). Tied at 1: (none) before Sorcery alphabetically.
	if groups[0].TypeName != "Creature" {
		t.Errorf("rank 1: want Creature, got %q", groups[0].TypeName)
	}
}

func TestGroupByType_TiebreaksByName(t *testing.T) {
	// Two types with identical uncovered counts → ascending name.
	rs := []result{
		{Name: "x", Class: classMissing, TypeLine: "Sorcery"},
		{Name: "y", Class: classMissing, TypeLine: "Instant"},
	}
	groups := groupByType(rs)
	if groups[0].TypeName != "Instant" || groups[1].TypeName != "Sorcery" {
		t.Errorf("tied uncovered counts should sort by name asc; got %q / %q",
			groups[0].TypeName, groups[1].TypeName)
	}
}

func TestTypeGroup_CoveragePct(t *testing.T) {
	g := TypeGroup{Total: 10, OK: 7, OKVanilla: 1}
	if math.Abs(g.CoveragePct()-80.0) > 0.0001 {
		t.Errorf("coverage 8/10: want 80, got %.4f", g.CoveragePct())
	}
	if (TypeGroup{Total: 0}).CoveragePct() != 0 {
		t.Errorf("zero total should yield 0%%")
	}
	if (TypeGroup{Total: 5, OK: 5}).CoveragePct() != 100 {
		t.Errorf("5/5 should yield 100%%")
	}
}

func TestTypeGroup_CoveragePct_ZeroTotalNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CoveragePct panicked on Total=0: %v", r)
		}
	}()
	_ = TypeGroup{Total: 0}.CoveragePct()
}

func TestGroupByType_EmptyInput(t *testing.T) {
	if got := groupByType(nil); len(got) != 0 {
		t.Errorf("nil input should yield 0 groups, got %d", len(got))
	}
}

func TestWriteByTypeReport_EmitsExpectedColumnsAndRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	groups := groupByType(typeFixture())
	if err := writeByTypeReport(path, groups); err != nil {
		t.Fatalf("writeByTypeReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	for _, want := range []string{
		"# Parser Coverage by Card Type",
		"| Rank | Type | Total | Uncovered | Coverage | MISSING | EMPTY_AST | PARTIAL |",
		"| 1 | Creature |",
		"Instant",
		"Sorcery",
		"Land",
		"(none)",
		"Multi-type cards",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n---\n%s", want, out)
		}
	}
}

func TestWriteByTypeReport_CoverageRenderedToOneDecimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	// Force a specific Coverage% to verify formatting precision.
	groups := []TypeGroup{
		{TypeName: "Creature", Total: 10, OK: 7, OKVanilla: 1, Uncovered: 2, Missing: 1, EmptyAST: 1},
	}
	if err := writeByTypeReport(path, groups); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "80.0%") {
		t.Errorf("Creature 8/10 should render as '80.0%%', got:\n%s", got)
	}
}

func TestWriteByTypeReport_EmptyGroupsStillEmitsHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if err := writeByTypeReport(path, nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	if !strings.Contains(out, "# Parser Coverage by Card Type") {
		t.Error("empty groups should still emit title")
	}
	if !strings.Contains(out, "| Rank | Type |") {
		t.Error("empty groups should still emit table header")
	}
}

func TestWriteByTypeReport_BadPathReturnsError(t *testing.T) {
	err := writeByTypeReport("/this/path/does/not/exist/out.md", nil)
	if err == nil {
		t.Fatal("expected error for unwritable path")
	}
}

func TestFormatByTypeSummary_TopNFormatting(t *testing.T) {
	groups := groupByType(typeFixture())
	got := formatByTypeSummary(groups, 2)
	if !strings.Contains(got, "Creature=4") {
		t.Errorf("summary should report Creature=4, got %q", got)
	}
	if !strings.Contains(got, "Instant=2") {
		t.Errorf("summary should report Instant=2, got %q", got)
	}
	// 3rd-place type must NOT appear when n=2.
	if strings.Contains(got, "Sorcery") || strings.Contains(got, "Land") {
		t.Errorf("n=2 should not include 3rd+ types, got %q", got)
	}
}

func TestFormatByTypeSummary_EmptyGroups(t *testing.T) {
	if got := formatByTypeSummary(nil, 5); !strings.Contains(got, "no uncovered") {
		t.Errorf("empty groups should say 'no uncovered cards', got %q", got)
	}
}

func TestFormatByTypeSummary_NClampedToLength(t *testing.T) {
	groups := groupByType(typeFixture())
	got := formatByTypeSummary(groups, 99)
	if !strings.Contains(got, "Creature") {
		t.Errorf("n > len(groups) should include all groups; got %q", got)
	}
}
