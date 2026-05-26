package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makePkg builds a temporary Go-source directory from a name→content
// map. Each file is written verbatim; callers control the test/non-
// test suffix via the filename.
func makePkg(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAnalyze_ExportedTestOnly_Flagged(t *testing.T) {
	dir := makePkg(t, map[string]string{
		"helper.go": `package x

// Exposed exists only for tests — should be flagged.
func Exposed() int { return 1 }

// Live is called from prod.go in the same package.
func Live() int { return 2 }
`,
		"prod.go": `package x

func consumer() int { return Live() }
`,
		"helper_test.go": `package x

func TestExposed() { _ = Exposed() }
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	names := declNames(res.ExportedTestOnly)
	if !containsString(names, "Exposed") {
		t.Errorf("Exposed should be flagged as test-only, got %v", names)
	}
	if containsString(names, "Live") {
		t.Errorf("Live has a non-test caller; should NOT be flagged, got %v", names)
	}
}

func TestAnalyze_NotFlaggedIfUnusedEverywhere(t *testing.T) {
	// A helper with ZERO references (neither prod nor tests) is NOT
	// flagged by the exported_but_test_only category — that's a
	// different finding shape (truly unused export) outside this
	// audit's scope. Pin the behavior so future tweaks don't widen
	// it accidentally.
	dir := makePkg(t, map[string]string{
		"lonely.go": `package x

func Orphan() int { return 7 }
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if containsString(declNames(res.ExportedTestOnly), "Orphan") {
		t.Errorf("Orphan has 0 refs anywhere; must not be flagged in test-only category")
	}
}

func TestAnalyze_UnexportedNotFlagged(t *testing.T) {
	// Lowercase helpers are out of scope: dead unexported helpers are
	// a different cleanup pass.
	dir := makePkg(t, map[string]string{
		"helper.go": `package x

func internalOnly() int { return 1 }
`,
		"helper_test.go": `package x

func TestX() { _ = internalOnly() }
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if containsString(declNames(res.ExportedTestOnly), "internalOnly") {
		t.Errorf("unexported helper should not be in exported_but_test_only")
	}
}

func TestAnalyze_MethodFlagged(t *testing.T) {
	// Receiver methods on exported types follow the same rule as free
	// functions.
	dir := makePkg(t, map[string]string{
		"gs.go": `package x

type GS struct{}

// TestHook only called from tests.
func (g *GS) TestHook() int { return 1 }
`,
		"prod.go": `package x

func use() *GS { return &GS{} }
`,
		"gs_test.go": `package x

func TestGS_TestHook() {
    gs := &GS{}
    _ = gs.TestHook()
}
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !containsString(declNames(res.ExportedTestOnly), "TestHook") {
		t.Errorf("(*GS).TestHook should be flagged, got %v", declNames(res.ExportedTestOnly))
	}
}

func TestAnalyze_UnusedSwitchCaseFlagged(t *testing.T) {
	dir := makePkg(t, map[string]string{
		"router.go": `package x

func route(kind string) string {
	switch kind {
	case "alpha":
		return "a"
	case "beta_dead":
		return "b"
	case "gamma":
		return "c"
	}
	return ""
}

func emit() []string {
	return []string{"alpha", "gamma"}
}
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !containsCaseVal(res.UnusedSwitchCases, "beta_dead") {
		t.Errorf("beta_dead has no other emitter; should be flagged, got %v",
			caseVals(res.UnusedSwitchCases))
	}
	for _, c := range res.UnusedSwitchCases {
		if c.Value == "alpha" || c.Value == "gamma" {
			t.Errorf("%q is emitted by emit(); must not be flagged", c.Value)
		}
	}
}

func TestAnalyze_UnusedSwitchCase_RawStringsCount(t *testing.T) {
	// Backtick raw strings should be treated as regular string
	// literals — emitting a value via raw-string still counts.
	dir := makePkg(t, map[string]string{
		"router.go": "package x\n\n" +
			"func route(k string) bool {\n" +
			"\tswitch k {\n" +
			"\tcase \"go\":\n" +
			"\t\treturn true\n" +
			"\t}\n" +
			"\treturn false\n" +
			"}\n\n" +
			"func emit() string { return `go` }\n",
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	for _, c := range res.UnusedSwitchCases {
		if c.Value == "go" {
			t.Errorf("raw-string emitter should disqualify case as dead")
		}
	}
}

func TestAnalyze_UnusedSwitchCase_SwitchTagRecorded(t *testing.T) {
	dir := makePkg(t, map[string]string{
		"router.go": `package x

type Event struct{ Kind string }

func route(e Event) bool {
	switch e.Kind {
	case "nope_dead":
		return false
	}
	return true
}
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var found bool
	for _, c := range res.UnusedSwitchCases {
		if c.Value == "nope_dead" {
			found = true
			if c.SwitchTag != "e.Kind" {
				t.Errorf("switch tag: want %q got %q", "e.Kind", c.SwitchTag)
			}
		}
	}
	if !found {
		t.Errorf("dead case not flagged, got %+v", res.UnusedSwitchCases)
	}
}

func TestAnalyze_SelfReferenceIgnored(t *testing.T) {
	// A helper that references itself (recursion) in its declaring
	// file shouldn't count that as a "non-test reference" preventing
	// the test-only flag. Defends the "skip self-file refs" filter.
	dir := makePkg(t, map[string]string{
		"recur.go": `package x

func Recur(n int) int {
	if n <= 0 {
		return 0
	}
	return Recur(n - 1) + 1
}
`,
		"recur_test.go": `package x

func TestRecur() { _ = Recur(5) }
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !containsString(declNames(res.ExportedTestOnly), "Recur") {
		t.Errorf("Recur with only test+self refs should be flagged, got %v",
			declNames(res.ExportedTestOnly))
	}
}

func TestAnalyze_TotalCounts(t *testing.T) {
	dir := makePkg(t, map[string]string{
		"x.go": `package x
func A() int { return 1 }
func B() int { return A() + A() }
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if res.TotalDecls != 2 {
		t.Errorf("TotalDecls: want 2, got %d", res.TotalDecls)
	}
	if res.TotalRefs < 2 {
		t.Errorf("TotalRefs: want ≥2 (two A() calls in B), got %d", res.TotalRefs)
	}
}

func TestAnalyze_SkipsParseErrors(t *testing.T) {
	// A syntactically broken file should be silently skipped without
	// failing the whole audit.
	dir := makePkg(t, map[string]string{
		"good.go": `package x
func Good() int { return 1 }
`,
		"bad.go": `package x
func {{{ broken
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if res.TotalDecls < 1 {
		t.Errorf("Good() should still be decl'd despite bad.go: got %d decls", res.TotalDecls)
	}
}

func TestWriteReport_ShapeAndCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	r := &AnalyzeResult{
		TotalDecls: 5,
		TotalRefs:  10,
		ExportedTestOnly: []Declaration{
			{Name: "Helper", File: "foo.go", Line: 10, IsExported: true},
		},
		UnusedSwitchCases: []CaseLiteral{
			{Value: "dead_kind", File: "router.go", Line: 22, SwitchTag: "e.Kind"},
		},
	}
	if err := writeReport(path, "internal/gameengine", r, 50); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	for _, want := range []string{
		"# Audit: Engine Dead Branches (R60 Phase 1D)",
		"Declarations scanned | 5",
		"Identifier references counted | 10",
		"`exported_but_test_only` findings | 1",
		"`unused_switch_case_literals` findings | 1",
		"`Helper`",
		"`dead_kind`",
		"`e.Kind`",
		"Methodology",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n---\n%s", want, out)
		}
	}
}

func TestWriteReport_TruncatesToTopN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	r := &AnalyzeResult{}
	for i := 0; i < 100; i++ {
		r.ExportedTestOnly = append(r.ExportedTestOnly, Declaration{
			Name: "Helper", File: "f.go", Line: i, IsExported: true,
		})
	}
	if err := writeReport(path, "x", r, 10); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "90 more") {
		t.Errorf("expected '90 more' truncation marker")
	}
	if !strings.Contains(string(got), "exported_but_test_only` findings | 100") {
		t.Errorf("count must reflect ALL findings, not just shown")
	}
}

func TestUnquote_RegularAndRaw(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{`"hello"`, "hello", false},
		{"`raw value`", "raw value", false},
		{`"escaped \"quote\""`, `escaped "quote"`, false},
		{`"with\ttab"`, "with\ttab", false},
		{`"x"`, "x", false},
		{`""`, "", false},
		{`x`, "", true},
		{`"unterminated`, "", true},
	}
	for _, c := range cases {
		got, err := unquote(c.in)
		if (err != nil) != c.err {
			t.Errorf("unquote(%q): err=%v want err=%v", c.in, err, c.err)
			continue
		}
		if !c.err && got != c.want {
			t.Errorf("unquote(%q): got %q want %q", c.in, got, c.want)
		}
	}
}

func TestExprToString_CommonShapes(t *testing.T) {
	// Indirectly exercise via the switch-tag extraction in
	// TestAnalyze_UnusedSwitchCase_SwitchTagRecorded. This explicit
	// test pins the dot-selector composition; if exprToString later
	// loses recursion, this catches it directly.
	dir := makePkg(t, map[string]string{
		"x.go": `package x

type Foo struct{ Bar struct{ Baz string } }

func route(f Foo) bool {
	switch f.Bar.Baz {
	case "dead":
		return false
	}
	return true
}
`,
	})
	res, err := AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	var found bool
	for _, c := range res.UnusedSwitchCases {
		if c.Value == "dead" {
			found = true
			if c.SwitchTag != "f.Bar.Baz" {
				t.Errorf("tag for nested selector: want f.Bar.Baz got %q", c.SwitchTag)
			}
		}
	}
	if !found {
		t.Fatal("nested-selector dead case not flagged")
	}
}

// helpers

func declNames(ds []Declaration) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Name)
	}
	return out
}

func caseVals(cs []CaseLiteral) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Value)
	}
	return out
}

func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func containsCaseVal(cs []CaseLiteral, v string) bool {
	for _, c := range cs {
		if c.Value == v {
			return true
		}
	}
	return false
}
