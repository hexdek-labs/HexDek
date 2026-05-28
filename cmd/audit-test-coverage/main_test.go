package main

import (
	"math"
	"strings"
	"testing"
)

// sampleProfile constructs a minimal but realistic coverprofile body
// for the analyze tests. Two packages:
//
//	internal/gameengine/   — 2 files, one well-covered, one never-tested
//	cmd/hexdek-server/    — 1 file, partially covered
//
// File ratios chosen so the assertions in TestAnalyze_PercentMath hit
// exact expected values (no float drift).
const sampleProfile = `mode: set
github.com/hexdek/hexdek/internal/gameengine/zones.go:10.1,12.1 4 1
github.com/hexdek/hexdek/internal/gameengine/zones.go:15.1,17.1 6 1
github.com/hexdek/hexdek/internal/gameengine/zones.go:20.1,22.1 5 0
github.com/hexdek/hexdek/internal/gameengine/sba.go:30.1,32.1 8 0
github.com/hexdek/hexdek/internal/gameengine/sba.go:35.1,37.1 12 0
github.com/hexdek/hexdek/cmd/hexdek-server/main.go:1.1,5.1 10 1
github.com/hexdek/hexdek/cmd/hexdek-server/main.go:6.1,8.1 10 0
`

// TestAnalyze_PercentMath pins exact coverage math for the fixture
// above. zones.go: 10 of 15 covered = 66.7%; sba.go: 0 of 20 covered =
// 0%; main.go: 10 of 20 covered = 50%. Package gameengine: 10/35 =
// 28.57%; package hexdek-server: 10/20 = 50%. Overall: 20/55 = 36.4%.
func TestAnalyze_PercentMath(t *testing.T) {
	r, err := analyze(sampleProfile, "github.com/hexdek/hexdek", 50.0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(r.Files) != 3 {
		t.Fatalf("want 3 files, got %d", len(r.Files))
	}

	fileByPath := map[string]FileCoverage{}
	for _, f := range r.Files {
		fileByPath[f.Path] = f
	}

	zones := fileByPath["internal/gameengine/zones.go"]
	if zones.Statements != 15 || zones.Covered != 10 {
		t.Errorf("zones.go: stmts=%d cov=%d, want 15/10", zones.Statements, zones.Covered)
	}
	if math.Abs(zones.Percent-66.6667) > 0.01 {
		t.Errorf("zones.go percent = %.4f, want 66.67", zones.Percent)
	}

	sba := fileByPath["internal/gameengine/sba.go"]
	if sba.Statements != 20 || sba.Covered != 0 {
		t.Errorf("sba.go: stmts=%d cov=%d, want 20/0", sba.Statements, sba.Covered)
	}
	if sba.Percent != 0 {
		t.Errorf("sba.go percent = %.4f, want 0", sba.Percent)
	}

	main := fileByPath["cmd/hexdek-server/main.go"]
	if main.Percent != 50.0 {
		t.Errorf("main.go percent = %.4f, want 50.0", main.Percent)
	}

	// Package-level aggregation.
	pkgByPath := map[string]PackageCoverage{}
	for _, p := range r.Packages {
		pkgByPath[p.Path] = p
	}
	ge := pkgByPath["internal/gameengine"]
	if math.Abs(ge.Percent-28.5714) > 0.01 {
		t.Errorf("gameengine pkg percent = %.4f, want 28.57", ge.Percent)
	}
	if ge.Statements != 35 || ge.Covered != 10 {
		t.Errorf("gameengine pkg totals: stmts=%d cov=%d, want 35/10", ge.Statements, ge.Covered)
	}

	// Overall.
	if math.Abs(r.OverallPercent-36.3636) > 0.01 {
		t.Errorf("overall percent = %.4f, want 36.36", r.OverallPercent)
	}
}

// TestAnalyze_UndertestedThreshold verifies the threshold gate: at 50%
// both gameengine (28.57%) and main.go are eligible, but main.go is at
// EXACTLY 50% — the gate is strict `< threshold`, so main.go's package
// (hexdek-server, 50%) must NOT be flagged. gameengine must be.
func TestAnalyze_UndertestedThreshold(t *testing.T) {
	r, err := analyze(sampleProfile, "github.com/hexdek/hexdek", 50.0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(r.UndertestedPackages) != 1 {
		t.Fatalf("want 1 undertested pkg, got %d: %+v", len(r.UndertestedPackages), r.UndertestedPackages)
	}
	if r.UndertestedPackages[0].Path != "internal/gameengine" {
		t.Errorf("wrong undertested pkg: %q", r.UndertestedPackages[0].Path)
	}

	// At threshold=60, hexdek-server (50%) should now be flagged too.
	r2, _ := analyze(sampleProfile, "github.com/hexdek/hexdek", 60.0)
	if len(r2.UndertestedPackages) != 2 {
		t.Errorf("at threshold 60, want 2 undertested pkgs, got %d", len(r2.UndertestedPackages))
	}
}

// TestAnalyze_UndertestedSortedAscending pins the ordering — the
// most-needy (lowest-coverage) package should render first.
func TestAnalyze_UndertestedSortedAscending(t *testing.T) {
	r, _ := analyze(sampleProfile, "github.com/hexdek/hexdek", 100.0) // include all
	for i := 1; i < len(r.UndertestedPackages); i++ {
		if r.UndertestedPackages[i].Percent < r.UndertestedPackages[i-1].Percent {
			t.Errorf("undertested not ascending at idx %d: %+v", i, r.UndertestedPackages)
		}
	}
}

// TestAnalyze_NeverTestedFlagsZeroCoverage verifies sba.go (covered=0,
// statements>0) lands in NeverTestedFiles, while zones.go (partial)
// and main.go (50%) do NOT.
func TestAnalyze_NeverTestedFlagsZeroCoverage(t *testing.T) {
	r, _ := analyze(sampleProfile, "github.com/hexdek/hexdek", 50.0)
	if len(r.NeverTestedFiles) != 1 {
		t.Fatalf("want 1 never-tested file, got %d: %+v", len(r.NeverTestedFiles), r.NeverTestedFiles)
	}
	if r.NeverTestedFiles[0].Path != "internal/gameengine/sba.go" {
		t.Errorf("wrong never-tested file: %q", r.NeverTestedFiles[0].Path)
	}
}

// TestAnalyze_ModulePrefixStripped verifies the module-prefix trim so
// display paths are relative and stable.
func TestAnalyze_ModulePrefixStripped(t *testing.T) {
	r, _ := analyze(sampleProfile, "github.com/hexdek/hexdek", 50.0)
	for _, f := range r.Files {
		if strings.HasPrefix(f.Path, "github.com/") {
			t.Errorf("module prefix not stripped: %q", f.Path)
		}
	}
}

// TestAnalyze_EmptyProfileErrors verifies the error path: an empty or
// header-only profile produces an error rather than a misleading
// "100% coverage" report.
func TestAnalyze_EmptyProfileErrors(t *testing.T) {
	if _, err := analyze("mode: set\n", "github.com/hexdek/hexdek", 50.0); err == nil {
		t.Errorf("header-only profile should error, got nil")
	}
}

// TestAnalyze_MalformedLineSkipped verifies that malformed lines don't
// poison the analysis — they're skipped silently. Defends against a
// profile with stray output (e.g. a build warning prepended).
func TestAnalyze_MalformedLineSkipped(t *testing.T) {
	body := `mode: set
this is not a coverage line
github.com/hexdek/hexdek/internal/x/y.go:1.1,2.1 5 1
`
	r, err := analyze(body, "github.com/hexdek/hexdek", 50.0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(r.Files) != 1 || r.Files[0].Path != "internal/x/y.go" {
		t.Errorf("malformed-line tolerance broke: %+v", r.Files)
	}
}

// TestRecommend_PerCardEngine pins the per-card engine recommendation
// path. These get a tailored message because the per_card dir has
// the densest test-pairing convention in the codebase.
func TestRecommend_PerCardEngine(t *testing.T) {
	f := FileCoverage{Path: "internal/gameengine/per_card/abdel_adrian.go", Percent: 0}
	got := recommend(f)
	if !strings.Contains(got, "per-card handler") {
		t.Errorf("per_card recommendation missing handler hint, got: %q", got)
	}
	if !strings.Contains(got, "never tested") {
		t.Errorf("0%% file should get 'never tested' band, got: %q", got)
	}
}

// TestRecommend_EngineVsHexapiVsHat verifies the architecture buckets
// route to their respective tailored hints.
func TestRecommend_EngineVsHexapiVsHat(t *testing.T) {
	cases := []struct {
		path, wantSubstr string
	}{
		{"internal/gameengine/sba.go", "core engine path"},
		{"internal/hexapi/server.go", "HTTP/WS handler"},
		{"internal/hat/decision.go", "AI decision logic"},
		{"internal/analytics/heimdall.go", "Heimdall analytics"},
		{"internal/tournament/runner.go", "tournament runner"},
		{"internal/gameast/parser.go", "AST parser"},
		{"cmd/hexdek-server/main.go", "CLI entry point"},
		{"internal/util/foo.go", "internal utility"},
	}
	for _, tc := range cases {
		f := FileCoverage{Path: tc.path, Percent: 10}
		got := recommend(f)
		if !strings.Contains(got, tc.wantSubstr) {
			t.Errorf("recommend(%q) = %q, want substring %q", tc.path, got, tc.wantSubstr)
		}
	}
}

// TestRecommend_CoverageBands verifies the coverage-tier wording shifts
// across 0% / <25% / <50%.
func TestRecommend_CoverageBands(t *testing.T) {
	cases := []struct {
		percent  float64
		wantBand string
	}{
		{0, "never tested"},
		{15, "minimal coverage"},
		{40, "below threshold"},
	}
	for _, tc := range cases {
		f := FileCoverage{Path: "internal/foo/bar.go", Percent: tc.percent}
		got := recommend(f)
		if !strings.Contains(got, tc.wantBand) {
			t.Errorf("at %.0f%%, recommend missing %q, got: %q", tc.percent, tc.wantBand, got)
		}
	}
}

// TestRenderMarkdown_SectionsPresent verifies the report has the four
// expected sections — Overall, Undertested, Never-Tested, Per-File.
func TestRenderMarkdown_SectionsPresent(t *testing.T) {
	r, _ := analyze(sampleProfile, "github.com/hexdek/hexdek", 50.0)
	md := renderMarkdown(r, 50.0)
	for _, want := range []string{
		"# Test Coverage Audit",
		"## Overall",
		"## Undertested Packages",
		"## Never-Tested Files",
		"## Per-File Recommendations",
		"internal/gameengine/sba.go", // should appear in never-tested table
	} {
		if !strings.Contains(md, want) {
			t.Errorf("rendered markdown missing %q", want)
		}
	}
}

// TestRenderMarkdown_AllPassNoFindings verifies the "everything's
// fine" branch — when every package meets the threshold, the section
// renders an explicit "None" line rather than an empty table.
func TestRenderMarkdown_AllPassNoFindings(t *testing.T) {
	body := `mode: set
github.com/hexdek/hexdek/internal/x/y.go:1.1,2.1 10 1
`
	r, err := analyze(body, "github.com/hexdek/hexdek", 50.0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	md := renderMarkdown(r, 50.0)
	if !strings.Contains(md, "every package meets the threshold") {
		t.Errorf("all-pass branch missing 'every package meets the threshold' line")
	}
	if !strings.Contains(md, "every file with statements has at least one block executed") {
		t.Errorf("all-pass branch missing the never-tested 'None' line")
	}
}
