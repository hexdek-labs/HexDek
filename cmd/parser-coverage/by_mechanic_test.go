package main

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mechanicFixture builds a small synthetic dataset that touches the
// curated mechanic matchers. Counts per mechanic:
//
//	adventure: 3 total (2 missing, 1 OK)
//	flashback: 2 total (1 empty, 1 OK)
//	saga:      2 total (1 partial, 1 OK)
//	cycling:   1 total (1 OK)
//	prototype: 1 total (1 empty)
//
// The fixture intentionally overlaps a card across two mechanics
// (Saga + Cycling) to defend the documented "single card counts
// toward multiple mechanics" behavior.
func mechanicFixture() []result {
	return []result{
		// Adventures
		{Name: "Brazen Borrower", Class: classMissing, OracleText: "Adventure // ...", TypeLine: "Creature — Faerie // Adventure"},
		{Name: "Bonecrusher Giant", Class: classOK, OracleText: "Adventure", TypeLine: "Creature — Giant // Adventure"},
		{Name: "Edgewall Innkeeper", Class: classMissing, OracleText: "Adventure trigger", TypeLine: "Creature — Human // Adventure"},
		// Flashbacks
		{Name: "Snapcaster Mage", Class: classOK, OracleText: "Flashback {U}", TypeLine: "Creature — Human Wizard"},
		{Name: "Past in Flames", Class: classEmptyAST, OracleText: "Flashback {R}", TypeLine: "Sorcery"},
		// Sagas (one also has cycling)
		{Name: "Elspeth Conquers Death", Class: classPartial, OracleText: "Enchantment", TypeLine: "Enchantment — Saga"},
		{Name: "Old Saga With Cycling", Class: classOK, OracleText: "Cycling {2}", TypeLine: "Enchantment — Saga"},
		// Prototype
		{Name: "Phyrexian Fleshgorger", Class: classEmptyAST, OracleText: "Prototype {1}{B}", TypeLine: "Artifact Creature"},
		// Pure non-mechanic card to ensure denominator hygiene
		{Name: "Lightning Bolt", Class: classOK, OracleText: "Deal 3 damage to any target.", TypeLine: "Instant"},
	}
}

func TestGroupByMechanic_BucketsByOracleAndType(t *testing.T) {
	groups := groupByMechanic(mechanicFixture())
	m := map[string]MechanicCoverage{}
	for _, g := range groups {
		m[g.Name] = g
	}

	if a := m["adventure"]; a.Total != 3 || a.Missing != 2 || a.OK != 1 || a.Uncovered != 2 {
		t.Errorf("adventure aggregate: got %+v", a)
	}
	if f := m["flashback"]; f.Total != 2 || f.EmptyAST != 1 || f.OK != 1 || f.Uncovered != 1 {
		t.Errorf("flashback aggregate: got %+v", f)
	}
	if s := m["saga"]; s.Total != 2 || s.Partial != 1 || s.OK != 1 || s.Uncovered != 1 {
		t.Errorf("saga aggregate: got %+v", s)
	}
	if c := m["cycling"]; c.Total != 1 || c.OK != 1 || c.Uncovered != 0 {
		t.Errorf("cycling aggregate: got %+v", c)
	}
	if p := m["prototype"]; p.Total != 1 || p.EmptyAST != 1 || p.Uncovered != 1 {
		t.Errorf("prototype aggregate: got %+v", p)
	}
}

func TestGroupByMechanic_OverlappingMechanicsBothCount(t *testing.T) {
	// The "Old Saga With Cycling" card should appear in BOTH the saga
	// and cycling buckets — single card, two mechanic-handler
	// responsibilities.
	groups := groupByMechanic([]result{
		{Name: "MultiMech", Class: classOK, OracleText: "Cycling {2}", TypeLine: "Enchantment — Saga"},
	})
	m := map[string]MechanicCoverage{}
	for _, g := range groups {
		m[g.Name] = g
	}
	if m["saga"].Total != 1 || m["cycling"].Total != 1 {
		t.Errorf("overlapping mechanics should both count: saga.Total=%d cycling.Total=%d",
			m["saga"].Total, m["cycling"].Total)
	}
}

func TestGroupByMechanic_DropsEmptyMechanics(t *testing.T) {
	// A dataset that doesn't touch most matchers should produce a
	// short groups slice — no zero-total rows.
	rs := []result{
		{Name: "X", Class: classOK, OracleText: "Cycling {2}", TypeLine: "Instant"},
	}
	groups := groupByMechanic(rs)
	for _, g := range groups {
		if g.Total == 0 {
			t.Errorf("group with Total=0 leaked into output: %+v", g)
		}
	}
	// Cycling should be present; everything else absent (or zero-total).
	found := false
	for _, g := range groups {
		if g.Name == "cycling" {
			found = true
		}
	}
	if !found {
		t.Error("cycling group should be present for a cycling-bearing card")
	}
}

func TestGroupByMechanic_SortsByUncoveredDesc(t *testing.T) {
	groups := groupByMechanic(mechanicFixture())
	uncovered := make([]int, len(groups))
	for i, g := range groups {
		uncovered[i] = g.Uncovered
	}
	if !sort.IsSorted(sort.Reverse(sort.IntSlice(uncovered))) {
		t.Errorf("groups must be sorted by Uncovered desc, got %v", uncovered)
	}
	// adventure (2 uncovered) should be #1.
	if groups[0].Name != "adventure" {
		t.Errorf("rank 1: want adventure, got %q", groups[0].Name)
	}
}

func TestGroupByMechanic_TiebreaksByName(t *testing.T) {
	// Two mechanics with identical uncovered counts → ascending name.
	rs := []result{
		{Name: "X", Class: classEmptyAST, OracleText: "Flashback {U}", TypeLine: "Sorcery"},
		{Name: "Y", Class: classEmptyAST, OracleText: "Mutate {1}", TypeLine: "Creature"},
	}
	groups := groupByMechanic(rs)
	// Filter to just the tied pair for clarity.
	var fbIdx, muIdx = -1, -1
	for i, g := range groups {
		if g.Name == "flashback" {
			fbIdx = i
		}
		if g.Name == "mutate" {
			muIdx = i
		}
	}
	if fbIdx < 0 || muIdx < 0 {
		t.Fatalf("expected both flashback and mutate in groups, got %+v", groups)
	}
	if fbIdx > muIdx {
		t.Errorf("tied uncovered counts should sort by name asc; flashback at %d, mutate at %d",
			fbIdx, muIdx)
	}
}

func TestMechanicCoverage_CoveragePct(t *testing.T) {
	g := MechanicCoverage{Total: 10, OK: 7, OKVanilla: 1}
	if math.Abs(g.CoveragePct()-80.0) > 0.0001 {
		t.Errorf("8/10: want 80, got %.4f", g.CoveragePct())
	}
	if (MechanicCoverage{Total: 0}).CoveragePct() != 0 {
		t.Errorf("zero total should yield 0%%")
	}
	if (MechanicCoverage{Total: 5, OK: 5}).CoveragePct() != 100 {
		t.Errorf("5/5 should yield 100%%")
	}
}

func TestMechanicCoverage_CoveragePct_ZeroTotalNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CoveragePct panicked on Total=0: %v", r)
		}
	}()
	_ = MechanicCoverage{Total: 0}.CoveragePct()
}

func TestGroupByMechanic_EmptyInput(t *testing.T) {
	if got := groupByMechanic(nil); len(got) != 0 {
		t.Errorf("nil input should yield 0 groups, got %d", len(got))
	}
}

func TestWriteByMechanicReport_EmitsExpectedColumnsAndRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	groups := groupByMechanic(mechanicFixture())
	if err := writeByMechanicReport(path, groups); err != nil {
		t.Fatalf("writeByMechanicReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	for _, want := range []string{
		"# Parser Coverage by Mechanic",
		"| Rank | Mechanic | Total | Uncovered | Coverage | MISSING | EMPTY_AST | PARTIAL |",
		"`adventure`",
		"`flashback`",
		"`saga`",
		"`prototype`",
		"`cycling`",
		"Curated matcher list",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n---\n%s", want, out)
		}
	}
}

func TestWriteByMechanicReport_CoverageRenderedToOneDecimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	groups := []MechanicCoverage{
		{Name: "adventure", Total: 10, OK: 7, OKVanilla: 1, Uncovered: 2, Missing: 1, EmptyAST: 1},
	}
	if err := writeByMechanicReport(path, groups); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "80.0%") {
		t.Errorf("8/10 should render as '80.0%%', got:\n%s", got)
	}
}

func TestWriteByMechanicReport_EmptyGroupsStillEmitsHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if err := writeByMechanicReport(path, nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	if !strings.Contains(out, "# Parser Coverage by Mechanic") {
		t.Error("empty groups should still emit title")
	}
	if !strings.Contains(out, "| Rank | Mechanic |") {
		t.Error("empty groups should still emit table header")
	}
}

func TestWriteByMechanicReport_BadPathReturnsError(t *testing.T) {
	err := writeByMechanicReport("/this/path/does/not/exist/out.md", nil)
	if err == nil {
		t.Fatal("expected error for unwritable path")
	}
}

func TestFormatByMechanicSummary_TopNFormatting(t *testing.T) {
	groups := groupByMechanic(mechanicFixture())
	got := formatByMechanicSummary(groups, 2)
	if !strings.Contains(got, "adventure=2") {
		t.Errorf("summary should report adventure=2, got %q", got)
	}
	// 3rd-place mechanic must NOT appear when n=2.
	parts := strings.Count(got, "=")
	if parts != 2 {
		t.Errorf("n=2 should produce exactly 2 entries, got %d (%q)", parts, got)
	}
}

func TestFormatByMechanicSummary_EmptyGroups(t *testing.T) {
	if got := formatByMechanicSummary(nil, 5); !strings.Contains(got, "no mechanic-bearing") {
		t.Errorf("empty groups should report no mechanic-bearing cards, got %q", got)
	}
}

func TestFormatByMechanicSummary_NClampedToLength(t *testing.T) {
	groups := groupByMechanic(mechanicFixture())
	got := formatByMechanicSummary(groups, 99)
	if !strings.Contains(got, "adventure") {
		t.Errorf("n > len(groups) should include all groups; got %q", got)
	}
}

func TestMechanicMatchers_AreCaseInsensitive(t *testing.T) {
	// Flashback matcher should hit regardless of case in oracle text.
	rs := []result{
		{Name: "Lower", Class: classOK, OracleText: "flashback {U}"},
		{Name: "Upper", Class: classOK, OracleText: "FLASHBACK {U}"},
		{Name: "Mixed", Class: classOK, OracleText: "Flashback {U}"},
	}
	groups := groupByMechanic(rs)
	m := map[string]MechanicCoverage{}
	for _, g := range groups {
		m[g.Name] = g
	}
	if m["flashback"].Total != 3 {
		t.Errorf("case-insensitive flashback should match all 3 spellings, got Total=%d",
			m["flashback"].Total)
	}
}
