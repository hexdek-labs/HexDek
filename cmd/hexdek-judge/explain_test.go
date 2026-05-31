package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// explain_test.go — pins the --explain helper across the
// well-defined sections of the report.
//
// Coverage:
//   1. Known rule with codebase checks + historical fix renders all
//      four sections (CR text / codebase checks / related rules /
//      resolved-issue history) with content
//   2. Known rule with NO historical fix renders the "no historical
//      fixes" placeholder rather than dropping the section
//   3. Known rule with NO codebase probes renders the "pure invariant
//      coverage" placeholder
//   4. Known rule with NO related rules renders the empty placeholder
//   5. §-prefix is stripped — `--explain §704.5f` resolves the same
//      as `--explain 704.5f`
//   6. Unknown rule returns known=false, prints "No index entry for"
//      message, doesn't panic
//   7. Empty rule arg returns an error
//   8. Output-path variant writes to the file rather than stdout
//   9. Every probe in probeRules has a probeFileHints entry (sync
//      invariant pin)
//  10. The wrapWords helper correctly splits long summary lines

// captureExplain runs runExplain to a temp file and returns the
// rendered content + the known flag.
func captureExplain(t *testing.T, rule string) (string, bool) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "out.txt")
	known, err := runExplain(rule, tmp)
	if err != nil {
		t.Fatalf("runExplain(%q): %v", rule, err)
	}
	raw, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), known
}

// ---------------------------------------------------------------------------
// Known rule with codebase + history — every section populated
// ---------------------------------------------------------------------------

func TestExplain_KnownRuleWithFullCoverage(t *testing.T) {
	// Seed CLAUDE.md so the report has historical-fix content.
	doc := `### Resolved

| Date | Source | Issue | Resolution |
|---|---|---|---|
| 2026-05-24 | Loki seed-2025 game-3180 | **Charix toughness=-7 SBA gap** — survives at CR §704.5f. | Closed via SBA scaffold fix. |
`
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	os.WriteFile(path, []byte(doc), 0644)
	t.Setenv("CLAUDEMD_PATH", path)

	out, known := captureExplain(t, "704.5f")
	if !known {
		t.Fatalf("expected known=true for §704.5f")
	}
	for _, want := range []string{
		"CR §704.5f",
		"State-Based Actions",
		"CODEBASE CHECKS",
		"Probes that check this rule:",
		"sba_probe",
		"Engine invariants that cite this rule:",
		"RELATED RULES",
		"RESOLVED-ISSUE HISTORY",
		"1 historical fix(es) in CLAUDE.md cite this rule",
		"2026-05-24",
		"Loki seed-2025",
		"Charix toughness=-7 SBA gap",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain output missing %q in:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Known rule with NO historical fix — section header still present
// with the "no fixes cite this rule" placeholder
// ---------------------------------------------------------------------------

func TestExplain_KnownRule_NoHistoricalFixes_PlaceholderRenders(t *testing.T) {
	// CLAUDE.md with no citations.
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	os.WriteFile(path, []byte("# empty\n"), 0644)
	t.Setenv("CLAUDEMD_PATH", path)

	out, known := captureExplain(t, "704.5g")
	if !known {
		t.Fatalf("expected known=true for §704.5g")
	}
	if !strings.Contains(out, "No historical fixes in CLAUDE.md cite this rule") {
		t.Errorf("expected 'No historical fixes' placeholder; got:\n%s", out)
	}
	// All four section headers must still be present.
	for _, want := range []string{
		"CODEBASE CHECKS",
		"RELATED RULES",
		"RESOLVED-ISSUE HISTORY",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing section header %q in:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Rule with no codebase probes — "pure invariant coverage" placeholder
// ---------------------------------------------------------------------------

func TestExplain_RuleWithNoProbes_PlaceholderRenders(t *testing.T) {
	// §119 is cited via LifeConsistency invariant but no probe directly
	// implements it — so CheckedBy should be empty and the placeholder
	// must render. (The §-prefix path also covered here.)
	t.Setenv("CLAUDEMD_PATH", "/dev/null")
	out, known := captureExplain(t, "§119")
	if !known {
		t.Fatalf("expected §119 to be in the index (via LifeConsistency invariant)")
	}
	// §119's RelatedInvariants should be non-empty (LifeConsistency).
	if !strings.Contains(out, "LifeConsistency") {
		t.Errorf("expected LifeConsistency in invariants section; got:\n%s", out)
	}
	// CheckedBy IS empty for §119 — should produce the placeholder.
	if !strings.Contains(out, "(none — rule cited via engine invariant only, not by a judge probe)") {
		t.Errorf("expected '(none — rule cited via engine invariant only)' placeholder; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Rule with no related-rules cross-references — placeholder
// ---------------------------------------------------------------------------

func TestExplain_RuleWithNoRelatedRules_PlaceholderRenders(t *testing.T) {
	t.Setenv("CLAUDEMD_PATH", "/dev/null")
	// §202.2 is in the probe-only set and has no relatedRules entries.
	out, _ := captureExplain(t, "202.2")
	if !strings.Contains(out, "no cross-references in the relatedRules table") {
		t.Errorf("expected related-rules placeholder for §202.2; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// §-prefix folding — both forms resolve the same entry
// ---------------------------------------------------------------------------

func TestExplain_SectionPrefixStripped(t *testing.T) {
	t.Setenv("CLAUDEMD_PATH", "/dev/null")
	withPrefix, k1 := captureExplain(t, "§704.5a")
	withoutPrefix, k2 := captureExplain(t, "704.5a")
	if !k1 || !k2 {
		t.Fatalf("both forms must resolve §704.5a")
	}
	if withPrefix != withoutPrefix {
		t.Errorf("§-prefix vs bare form produced different outputs; diff:\n--- with §:\n%s\n--- without §:\n%s",
			withPrefix, withoutPrefix)
	}
}

// ---------------------------------------------------------------------------
// Unknown rule — known=false, descriptive message, no panic
// ---------------------------------------------------------------------------

func TestExplain_UnknownRule_NotKnown_AndGracefulMessage(t *testing.T) {
	t.Setenv("CLAUDEMD_PATH", "/dev/null")
	out, known := captureExplain(t, "999.99z")
	if known {
		t.Fatalf("expected known=false for §999.99z (not in index)")
	}
	if !strings.Contains(out, "No index entry for §999.99z") {
		t.Errorf("expected 'No index entry for §999.99z' message; got:\n%s", out)
	}
	if !strings.Contains(out, "--citation-index") {
		t.Errorf("expected suggestion to run --citation-index; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Empty rule arg — error
// ---------------------------------------------------------------------------

func TestExplain_EmptyRule_ReturnsError(t *testing.T) {
	t.Setenv("CLAUDEMD_PATH", "/dev/null")
	_, err := runExplain("", "")
	if err == nil {
		t.Fatalf("expected error for empty rule arg")
	}
	if !strings.Contains(err.Error(), "--explain requires") {
		t.Errorf("error message should mention the missing rule arg; got: %v", err)
	}
	// Whitespace-only also rejected.
	_, err = runExplain("   ", "")
	if err == nil {
		t.Errorf("expected error for whitespace-only rule arg")
	}
}

// ---------------------------------------------------------------------------
// --check-out writes to file rather than stdout
// ---------------------------------------------------------------------------

func TestExplain_CheckOutWritesToFile(t *testing.T) {
	t.Setenv("CLAUDEMD_PATH", "/dev/null")
	out := filepath.Join(t.TempDir(), "report.txt")
	known, err := runExplain("704.5a", out)
	if err != nil {
		t.Fatalf("runExplain: %v", err)
	}
	if !known {
		t.Fatalf("expected known=true")
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read out file: %v", err)
	}
	if !strings.Contains(string(body), "CR §704.5a") {
		t.Errorf("out file missing report content; got:\n%s", body)
	}
}

// ---------------------------------------------------------------------------
// Sync invariant: every probe in probeRules has a probeFileHints entry
// (so the report's "(file)" hint never goes missing for a real probe).
// ---------------------------------------------------------------------------

func TestExplain_ProbeFileHints_CoversAllProbes(t *testing.T) {
	for probe := range probeRules {
		if _, ok := probeFileHints[probe]; !ok {
			t.Errorf("probe %q has probeRules entry but no probeFileHints — report will show no file hint",
				probe)
		}
	}
}

// ---------------------------------------------------------------------------
// wrapWords — word-boundary splitting for long summary lines
// ---------------------------------------------------------------------------

func TestExplain_WrapWords(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  []string
	}{
		{"", 10, nil},
		{"short", 10, []string{"short"}},
		{"one two three four five", 10, []string{"one two", "three four", "five"}},
		{strings.Repeat("a", 20), 10, []string{strings.Repeat("a", 20)}}, // one word longer than width stays alone
	}
	for _, tc := range cases {
		got := wrapWords(tc.in, tc.width)
		if len(got) != len(tc.want) {
			t.Errorf("wrapWords(%q, %d) returned %d lines, want %d (lines=%v)",
				tc.in, tc.width, len(got), len(tc.want), got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("wrapWords(%q, %d) line %d = %q, want %q",
					tc.in, tc.width, i, got[i], tc.want[i])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Output stability — rendering the same rule twice produces identical
// bytes (no Date.Now / Math.Random / map-iteration drift).
// ---------------------------------------------------------------------------

func TestExplain_OutputIsStableAcrossRuns(t *testing.T) {
	t.Setenv("CLAUDEMD_PATH", "/dev/null")
	out1, _ := captureExplain(t, "704.5a")
	for i := 0; i < 5; i++ {
		out2, _ := captureExplain(t, "704.5a")
		if out1 != out2 {
			t.Fatalf("render is not deterministic across calls; iter %d differs", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Renderer directly — confirm the bar / section headers / order
// ---------------------------------------------------------------------------

func TestExplain_RendererSectionOrder(t *testing.T) {
	// Synthetic entry with content in every section.
	e := &CitationIndexEntry{
		Rule:              "704.5a",
		Description:       "Life ≤ 0 loss check",
		SectionTitle:      "State-Based Actions",
		CheckedBy:         []string{"sba_probe"},
		RelatedInvariants: []string{"LifeConsistency"},
		RelatedRules:      []string{"104.2", "119"},
		HistoricalFixes: []ClaudemdFix{
			{Date: "2026-05-30", Source: "dev/foo", IssueSummary: "fixture fix one"},
			{Date: "2026-05-29", Source: "dev/bar", IssueSummary: "fixture fix two"},
		},
	}
	var buf bytes.Buffer
	renderExplainReport(&buf, e)
	out := buf.String()
	// Section order must be CR text → CODEBASE → RELATED → RESOLVED.
	wantOrder := []string{
		"CR §704.5a",
		"CODEBASE CHECKS",
		"RELATED RULES",
		"RESOLVED-ISSUE HISTORY",
	}
	prev := -1
	for _, want := range wantOrder {
		idx := strings.Index(out, want)
		if idx < 0 {
			t.Errorf("missing section %q in output", want)
			continue
		}
		if idx <= prev {
			t.Errorf("section %q (at %d) out of order vs previous (at %d)", want, idx, prev)
		}
		prev = idx
	}
	// Both fixes render (no 5-cap like the interactive surface — this
	// is the full report).
	for _, want := range []string{"fixture fix one", "fixture fix two"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing fixture summary %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Exit-status convention — known rule → known=true, unknown → known=false.
// Sanity that the contract is reasonable.
// ---------------------------------------------------------------------------

func TestExplain_ExitStatusContract(t *testing.T) {
	t.Setenv("CLAUDEMD_PATH", "/dev/null")
	cases := []struct {
		rule  string
		known bool
	}{
		{"704.5a", true},
		{"903.4", true},
		{"§202.2", true},
		{"999.99z", false},
		{"not-a-rule", false},
	}
	for _, tc := range cases {
		_, known := captureExplain(t, tc.rule)
		if known != tc.known {
			t.Errorf("runExplain(%q): known = %v, want %v", tc.rule, known, tc.known)
		}
	}
	// Diagnostic — print all known rules so a future test author can
	// see what's actually in the index.
	idx := BuildCitationIndex()
	fmt.Fprintf(os.Stderr, "[diagnostic] %d rules in index at test time\n", idx.Counts.TotalRules)
}
