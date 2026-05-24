package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fixture builds a deterministic results slice spanning every class so
// the sampler / filter / report tests can pin behavior without needing
// the full oracle corpus.
func fixture() []result {
	// 30 results: 12 MISSING, 10 EMPTY_AST, 3 PARTIAL, 4 OK, 1 OK_VANILLA.
	rs := make([]result, 0, 30)
	for i := 0; i < 12; i++ {
		rs = append(rs, result{
			Name:       "Missing Card " + string(rune('A'+i)),
			Class:      classMissing,
			OracleText: "",
		})
	}
	for i := 0; i < 10; i++ {
		rs = append(rs, result{
			Name:       "EmptyAST Card " + string(rune('A'+i)),
			Class:      classEmptyAST,
			OracleText: "Some oracle text that the parser produced zero abilities from.",
		})
	}
	for i := 0; i < 3; i++ {
		rs = append(rs, result{
			Name:        "Partial Card " + string(rune('A'+i)),
			Class:       classPartial,
			OracleText:  "Complicated text",
			ParseErrors: []string{"unknown_clause"},
		})
	}
	for i := 0; i < 4; i++ {
		rs = append(rs, result{Name: "OK Card " + string(rune('A'+i)), Class: classOK, OracleText: "Flying."})
	}
	rs = append(rs, result{Name: "Forest", Class: classOKVanilla})
	return rs
}

func TestParseSampleClasses_DefaultIncludesAllUncoveredClasses(t *testing.T) {
	set, err := parseSampleClasses("missing,empty_ast,partial")
	if err != nil {
		t.Fatalf("parseSampleClasses default: %v", err)
	}
	for _, c := range []classification{classMissing, classEmptyAST, classPartial} {
		if !set[c] {
			t.Errorf("default set should include %s", c)
		}
	}
	if set[classOK] || set[classOKVanilla] {
		t.Error("default set must not include OK/OK_VANILLA")
	}
}

func TestParseSampleClasses_AcceptsCaseAndWhitespace(t *testing.T) {
	set, err := parseSampleClasses(" Missing , EMPTY_AST ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set) != 2 || !set[classMissing] || !set[classEmptyAST] {
		t.Fatalf("case/whitespace handling: got %v", set)
	}
}

func TestParseSampleClasses_RejectsUnknownName(t *testing.T) {
	if _, err := parseSampleClasses("missing,bogus"); err == nil {
		t.Fatal("expected error for unknown class name")
	}
}

func TestParseSampleClasses_RejectsOKAsSampleClass(t *testing.T) {
	// "ok" is a real class name but not sample-eligible — the sample is
	// for SCAFFOLD TARGETS, and OK cards are already covered.
	if _, err := parseSampleClasses("ok"); err == nil {
		t.Fatal("expected error when asking to sample OK")
	}
}

func TestParseSampleClasses_EmptyDisables(t *testing.T) {
	for _, s := range []string{"", "  ", "none", "NONE"} {
		set, err := parseSampleClasses(s)
		if err != nil {
			t.Errorf("parseSampleClasses(%q): %v", s, err)
			continue
		}
		if len(set) != 0 {
			t.Errorf("parseSampleClasses(%q) expected empty set, got %v", s, set)
		}
	}
}

func TestSampleUncovered_RespectsClassFilter(t *testing.T) {
	rs := fixture()
	// MISSING only: there are 12 in the fixture; ask for 20, get all 12.
	got := sampleUncovered(rs, map[classification]bool{classMissing: true}, 20, 42)
	if len(got) != 12 {
		t.Fatalf("MISSING-only with n>pool size: want all 12, got %d", len(got))
	}
	for _, r := range got {
		if r.Class != classMissing {
			t.Errorf("sample contained non-MISSING entry: %s (%s)", r.Name, r.Class)
		}
	}
}

func TestSampleUncovered_OKClassNeverSampled(t *testing.T) {
	rs := fixture()
	// Sample 20 from the union of all uncovered classes. The fixture has
	// 12+10+3 = 25 uncovered cards, so all 20 must come from that pool.
	got := sampleUncovered(rs, map[classification]bool{
		classMissing:  true,
		classEmptyAST: true,
		classPartial:  true,
	}, 20, 42)
	if len(got) != 20 {
		t.Fatalf("want 20 samples, got %d", len(got))
	}
	for _, r := range got {
		if r.Class == classOK || r.Class == classOKVanilla {
			t.Errorf("sample leaked OK card: %s (%s)", r.Name, r.Class)
		}
	}
}

func TestSampleUncovered_DeterministicForFixedSeed(t *testing.T) {
	rs := fixture()
	filter := map[classification]bool{classMissing: true, classEmptyAST: true, classPartial: true}
	a := sampleUncovered(rs, filter, 10, 1234)
	b := sampleUncovered(rs, filter, 10, 1234)
	if len(a) != len(b) {
		t.Fatalf("len mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Errorf("sample[%d]: %q vs %q (same seed must yield same sample)", i, a[i].Name, b[i].Name)
		}
	}
}

func TestSampleUncovered_DifferentSeedsDiffer(t *testing.T) {
	rs := fixture()
	filter := map[classification]bool{classMissing: true, classEmptyAST: true, classPartial: true}
	a := sampleUncovered(rs, filter, 10, 1)
	b := sampleUncovered(rs, filter, 10, 999999)
	same := 0
	for i := range a {
		if a[i].Name == b[i].Name {
			same++
		}
	}
	// Some overlap is OK (both draw from same pool) but not all-identical.
	if same == len(a) {
		t.Fatal("two different seeds produced identical samples; reservoir RNG isn't being used")
	}
}

func TestSampleUncovered_ZeroDisabled(t *testing.T) {
	rs := fixture()
	got := sampleUncovered(rs, map[classification]bool{classMissing: true}, 0, 42)
	if got != nil {
		t.Fatalf("n=0 should disable sampling, got %d entries", len(got))
	}
}

func TestSampleUncovered_OutputSortedByName(t *testing.T) {
	rs := fixture()
	filter := map[classification]bool{classMissing: true, classEmptyAST: true, classPartial: true}
	got := sampleUncovered(rs, filter, 15, 7)
	names := make([]string, len(got))
	for i, r := range got {
		names[i] = r.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("sample output must be sorted by name for stable markdown rendering, got %v", names)
	}
}

func TestTruncateOracle_HandlesEmptyAndLong(t *testing.T) {
	if got := truncateOracle("", 120); !strings.Contains(got, "no oracle text") {
		t.Errorf("empty input: %q", got)
	}
	long := strings.Repeat("abc ", 100) // 400 chars
	got := truncateOracle(long, 120)
	if len(got) > 120+3 { // +3 for the ellipsis rune (3 bytes UTF-8)
		t.Errorf("truncate didn't cap length: %d", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated text should end with ellipsis, got %q", got[len(got)-6:])
	}
}

func TestTruncateOracle_CollapsesNewlines(t *testing.T) {
	in := "Line one\nLine two\r\nLine three"
	got := truncateOracle(in, 120)
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("newlines not collapsed: %q", got)
	}
}

func TestEscapeCell_HandlesPipesAndBackticks(t *testing.T) {
	got := escapeCell("Foo | Bar `baz`")
	if strings.Contains(got, "|") && !strings.Contains(got, `\|`) {
		t.Errorf("pipe not escaped: %q", got)
	}
	if strings.Contains(got, "`") {
		t.Errorf("backtick should be replaced: %q", got)
	}
}

func TestWriteReport_EmitsUncoveredSampleSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	rs := fixture()
	filter := map[classification]bool{classMissing: true, classEmptyAST: true, classPartial: true}
	sample := sampleUncovered(rs, filter, 5, 42)

	if err := writeReport(path, rs, len(rs), len(rs), 0, sample, 42); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	report := string(got)
	if !strings.Contains(report, "## Uncovered card sample (random 5, seed=42)") {
		t.Error("report missing 'Uncovered card sample' section header")
	}
	if !strings.Contains(report, "| # | Card | Class | Oracle text (truncated) |") {
		t.Error("report missing sample table header")
	}
	// Each sampled card name should appear in the section.
	for _, r := range sample {
		if !strings.Contains(report, escapeCell(r.Name)) {
			t.Errorf("sample card %q not found in report", r.Name)
		}
	}
}

func TestWriteReport_OmitsSampleSectionWhenSampleEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	rs := fixture()
	if err := writeReport(path, rs, len(rs), len(rs), 0, nil, 0); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "Uncovered card sample") {
		t.Error("empty sample should NOT emit the section header")
	}
}
