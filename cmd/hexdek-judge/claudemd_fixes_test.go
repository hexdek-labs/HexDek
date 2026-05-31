package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claudemd_fixes_test.go — pins the CLAUDE.md Resolved-table parser
// + the bidirectional merge into the citation index.
//
// Coverage:
//   1. parseClaudemdResolvedFixesFromReader on a synthetic fixture
//      (deterministic, doesn't depend on the real CLAUDE.md churn)
//   2. The §-prefix variants (CR §X / §X / multiple per row)
//   3. The §-prefix false-positive filter (rule numbers must be ≥ 3
//      digits — doc-section refs like §3.1 don't match)
//   4. Dedup within a row (same rule cited twice → one fix entry)
//   5. Multiple rows for the same rule → multiple fix entries
//   6. MergeClaudemdFixesIntoIndex populates HistoricalFixes AND
//      surfaces the unmapped slugs gap
//   7. Section-header sentinel — fixes section ends at the next ###
//   8. Real CLAUDE.md smoke (sanity that the production path works
//      against the production doc) — gated on the file existing so
//      out-of-repo builds still pass
//   9. Date-desc sort within a rule
//  10. Interactive `index <rule>` surfaces the historical fixes block
//      when present
//  11. The bold-sentence summary extraction
//  12. The "fixes block omitted when empty" branch

// fixturePath writes the supplied content to a temp file and returns
// the path. Used by tests that need to drive parseClaudemdResolvedFixes
// (the path-taking entry point).
func fixturePath(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "CLAUDE.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Parser shape
// ---------------------------------------------------------------------------

func TestParseClaudemdResolvedFixes_BasicShape(t *testing.T) {
	doc := `# Some Project

### Open
| Date | Source | Issue | Sev | Notes |
|------|--------|-------|-----|-------|
| 2026-05-30 | open-thing | open-row | medium | n/a |

### Resolved

| Date | Source | Issue | Resolution |
|------|--------|-------|------------|
| 2026-05-28 | ` + "`dev/foo-r60`" + ` | **Issue A** — touches CR §704.5f mainly. | Closed via §704.5f scaffold fix. |
| 2026-05-29 | ` + "`dev/bar-r60`" + ` | **Issue B** — separate concern under §903.4. | Closed. |
`
	path := fixturePath(t, doc)
	fixes, err := parseClaudemdResolvedFixes(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(fixes["704.5f"]) != 1 {
		t.Fatalf("expected 1 fix for §704.5f, got %d: %+v", len(fixes["704.5f"]), fixes["704.5f"])
	}
	if len(fixes["903.4"]) != 1 {
		t.Fatalf("expected 1 fix for §903.4, got %d: %+v", len(fixes["903.4"]), fixes["903.4"])
	}
	a := fixes["704.5f"][0]
	if a.Date != "2026-05-28" {
		t.Errorf("Date = %q, want 2026-05-28", a.Date)
	}
	if !strings.Contains(a.Source, "dev/foo-r60") {
		t.Errorf("Source = %q, want containing dev/foo-r60", a.Source)
	}
	if a.IssueSummary != "Issue A" {
		t.Errorf("IssueSummary = %q, want Issue A", a.IssueSummary)
	}
	// Open-section rows must NOT be picked up.
	if len(fixes["open"]) != 0 {
		t.Errorf("Open-table rows leaked into fix map: %+v", fixes)
	}
}

// ---------------------------------------------------------------------------
// § variant coverage — CR § prefix, bare §, multiple per row, dedup
// ---------------------------------------------------------------------------

func TestParseClaudemdResolvedFixes_VariantsAndDedup(t *testing.T) {
	doc := `### Resolved

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-30 | x | **multi-cite** touching CR §704.5a and §704.5b and §704.5a again. | Closed via CR §704.5a fix. |
`
	path := fixturePath(t, doc)
	fixes, _ := parseClaudemdResolvedFixes(path)
	// §704.5a appears 3 times in the cells but should dedup to 1 fix.
	if len(fixes["704.5a"]) != 1 {
		t.Errorf("expected dedup to 1 fix for §704.5a, got %d", len(fixes["704.5a"]))
	}
	if len(fixes["704.5b"]) != 1 {
		t.Errorf("expected 1 fix for §704.5b, got %d", len(fixes["704.5b"]))
	}
}

// ---------------------------------------------------------------------------
// §-prefix false-positive filter — doc-section refs (e.g. "§3.1
// audit") must not match.
// ---------------------------------------------------------------------------

func TestParseClaudemdResolvedFixes_FiltersShortNumbers(t *testing.T) {
	doc := `### Resolved

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-30 | y | **doc-section ref** under §3.1 audit, not a CR section. | Closed. |
| 2026-05-30 | z | **real CR ref** under §704.5a actual rule. | Closed. |
`
	path := fixturePath(t, doc)
	fixes, _ := parseClaudemdResolvedFixes(path)
	if _, has := fixes["3.1"]; has {
		t.Errorf("§3.1 doc-section ref leaked into fixes (only ≥ 100 should match): %+v", fixes)
	}
	if _, has := fixes["3"]; has {
		t.Errorf("§3 doc-section ref leaked into fixes: %+v", fixes)
	}
	if len(fixes["704.5a"]) != 1 {
		t.Errorf("expected real CR §704.5a to still match; got %d fixes", len(fixes["704.5a"]))
	}
}

// ---------------------------------------------------------------------------
// Multiple rows for same rule → multiple fix entries (sorted date desc)
// ---------------------------------------------------------------------------

func TestParseClaudemdResolvedFixes_MultipleFixesSortedDateDesc(t *testing.T) {
	doc := `### Resolved

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-20 | older | **Older fix** touches §704.5a. | Closed. |
| 2026-05-30 | newer | **Newer fix** touches §704.5a. | Closed. |
| 2026-05-25 | middle | **Middle fix** touches §704.5a. | Closed. |
`
	path := fixturePath(t, doc)
	fixes, _ := parseClaudemdResolvedFixes(path)
	got := fixes["704.5a"]
	if len(got) != 3 {
		t.Fatalf("expected 3 fixes for §704.5a, got %d", len(got))
	}
	wantOrder := []string{"2026-05-30", "2026-05-25", "2026-05-20"}
	for i, want := range wantOrder {
		if got[i].Date != want {
			t.Errorf("fixes[%d].Date = %q, want %q (date-desc order)", i, got[i].Date, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Section sentinel — next ### terminates parsing
// ---------------------------------------------------------------------------

func TestParseClaudemdResolvedFixes_TerminatesAtNextSection(t *testing.T) {
	doc := `### Resolved

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-30 | a | **In Resolved** touches §704.5a. | Closed. |

### Subsequent Section

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-30 | b | **In Other section** touches §704.5b — must NOT be picked up. | Closed. |
`
	path := fixturePath(t, doc)
	fixes, _ := parseClaudemdResolvedFixes(path)
	if len(fixes["704.5a"]) != 1 {
		t.Errorf("expected 1 fix for §704.5a from Resolved section; got %d", len(fixes["704.5a"]))
	}
	if _, has := fixes["704.5b"]; has {
		t.Errorf("post-Resolved-section row leaked into fixes: %+v", fixes)
	}
}

// ---------------------------------------------------------------------------
// MergeClaudemdFixesIntoIndex — populates HistoricalFixes AND surfaces
// the unmapped slugs gap
// ---------------------------------------------------------------------------

func TestMergeClaudemdFixesIntoIndex_PopulatesAndSurfacesGaps(t *testing.T) {
	idx := &CitationIndex{
		Entries: map[string]*CitationIndexEntry{
			"704.5a": {Rule: "704.5a", Description: "Life ≤ 0 loss check"},
		},
	}
	fixes := map[string][]ClaudemdFix{
		"704.5a": {{Date: "2026-05-28", Source: "dev/foo", IssueSummary: "Atemsis Platinum Angel cancel"}},
		"727":    {{Date: "2026-05-30", Source: "dev/bar", IssueSummary: "No-op detector edge case"}},
	}
	unmapped := MergeClaudemdFixesIntoIndex(idx, fixes)
	if len(idx.Entries["704.5a"].HistoricalFixes) != 1 {
		t.Errorf("expected 1 fix attached to §704.5a, got %d",
			len(idx.Entries["704.5a"].HistoricalFixes))
	}
	if len(unmapped) != 1 || unmapped[0] != "727" {
		t.Errorf("unmapped slugs = %v, want [727] (rule not in index → gap-surfacing)", unmapped)
	}
}

// ---------------------------------------------------------------------------
// Build-time integration — BuildCitationIndex populates HistoricalFixes
// against a synthetic CLAUDEMD_PATH and updates the counts.
// ---------------------------------------------------------------------------

func TestBuildCitationIndex_AttachesHistoricalFixesAndUpdatesCounts(t *testing.T) {
	doc := `### Resolved

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-28 | ` + "`dev/sba-fix`" + ` | **SBA toughness=-7** — Charix survives, mismatches CR §704.5f. | Closed via §704.5f scaffold fix. |
| 2026-05-29 | ` + "`dev/cmdr-fix`" + ` | **Color identity off** — CR §903.4 violation hit production. | Closed. |
`
	path := fixturePath(t, doc)
	t.Setenv("CLAUDEMD_PATH", path)
	idx := BuildCitationIndex()
	if idx.Counts.TotalFixes < 2 {
		t.Errorf("Counts.TotalFixes = %d, want ≥ 2", idx.Counts.TotalFixes)
	}
	if idx.Counts.RulesWithFixes < 2 {
		t.Errorf("Counts.RulesWithFixes = %d, want ≥ 2", idx.Counts.RulesWithFixes)
	}
	for _, rule := range []string{"704.5f", "903.4"} {
		e := idx.LookupByRule(rule)
		if e == nil {
			t.Fatalf("missing index entry for §%s", rule)
		}
		if len(e.HistoricalFixes) < 1 {
			t.Errorf("§%s missing HistoricalFixes after build", rule)
		}
	}
}

// ---------------------------------------------------------------------------
// Real CLAUDE.md smoke — sanity that the production path works against
// the actual doc. Gated on the file existing so out-of-repo builds pass.
// ---------------------------------------------------------------------------

func TestParseClaudemdResolvedFixes_RealCLAUDEmdSmoke(t *testing.T) {
	if _, err := os.Stat("../../CLAUDE.md"); err != nil {
		t.Skip("CLAUDE.md not at expected path — skipping production smoke")
	}
	fixes, err := parseClaudemdResolvedFixes("../../CLAUDE.md")
	if err != nil {
		t.Fatalf("parse real CLAUDE.md: %v", err)
	}
	if len(fixes) < 5 {
		t.Errorf("expected at least 5 rules cited in real CLAUDE.md Resolved table, got %d",
			len(fixes))
	}
	// §704.5f is cited in the Charix toughness=-7 row — load-bearing anchor.
	if len(fixes["704.5f"]) < 1 {
		t.Errorf("real CLAUDE.md should have ≥ 1 fix for §704.5f (Charix toughness=-7); got %d",
			len(fixes["704.5f"]))
	}
}

// ---------------------------------------------------------------------------
// Issue summary extraction — bold sentence prefix
// ---------------------------------------------------------------------------

func TestExtractIssueSummary_BoldPrefix(t *testing.T) {
	cases := []struct {
		issue, want string
	}{
		{"**Short bold** then prose", "Short bold"},
		{"No bold here — just prose", "No bold here"},
		{strings.Repeat("a", 150), strings.Repeat("a", 120) + "…"},
	}
	for _, tc := range cases {
		got := extractIssueSummary(tc.issue)
		if got != tc.want {
			t.Errorf("extractIssueSummary(%.40s…) = %q, want %q",
				tc.issue, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Interactive — `index <rule>` surfaces the historical fixes block
// ---------------------------------------------------------------------------

func TestInteractive_IndexLookup_ShowsHistoricalFixes(t *testing.T) {
	doc := `### Resolved

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-28 | ` + "`dev/sba-fix`" + ` | **SBA toughness=-7** — Charix survives at CR §704.5f. | Closed. |
`
	path := fixturePath(t, doc)
	t.Setenv("CLAUDEMD_PATH", path)
	out := runScript(t, emptyCtx(), "index 704.5f\n")
	for _, want := range []string{
		"historical fixes (1):",
		"2026-05-28",
		"dev/sba-fix",
		"SBA toughness=-7",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("index lookup missing %q in:\n%s", want, out)
		}
	}
}

func TestInteractive_IndexLookup_OmitsFixesBlockWhenEmpty(t *testing.T) {
	// CLAUDEMD_PATH set to a doc that doesn't mention §704.5g.
	doc := `### Resolved

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-28 | x | **Touches §704.5f only**. | Closed. |
`
	path := fixturePath(t, doc)
	t.Setenv("CLAUDEMD_PATH", path)
	out := runScript(t, emptyCtx(), "index 704.5g\n")
	if strings.Contains(out, "historical fixes") {
		t.Errorf("§704.5g entry should NOT print 'historical fixes' when none present; got:\n%s",
			out)
	}
	// But the rest of the entry must still render.
	if !strings.Contains(out, "§704.5g") {
		t.Errorf("missing baseline §704.5g render: %s", out)
	}
}

// ---------------------------------------------------------------------------
// Date sentinel — non-date first cell rejects the row
// ---------------------------------------------------------------------------

func TestParseClaudemdRow_RejectsNonDateRows(t *testing.T) {
	cases := []string{
		"| Date | Source | Issue | Resolution |", // header row
		"|------|--------|-------|------------|", // separator
		"| not-a-date | x | y | z |",
	}
	for _, c := range cases {
		if r := parseClaudemdRow(c); r != nil {
			t.Errorf("parseClaudemdRow(%q) = %+v, want nil", c, r)
		}
	}
}
