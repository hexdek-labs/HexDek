package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestSaturationFloor_NoHighSignalFindings is the canonical regression
// net for the Phase 1D residue trajectory.
//
// As of fix-5 (this PR), every unused_switch_case_literals finding in
// `internal/gameengine` is classified as either:
//   - card-name (JSON-driven dispatch, expected FP)
//   - ast-enum (parser-emitted enum, expected FP)
//   - high-signal-documented (per-arm exception with PR reference)
//
// **Zero findings should fall through to the bare "high-signal" bucket.**
// If this test starts failing, a new dead branch (or a new false-
// positive class) has appeared in the engine — either delete the
// unreachable case OR investigate and add it to the appropriate
// classifier rule (`classifyTag` / `documentedHighSignalArms`).
//
// The test runs the real AnalyzePackageWithScope against
// `internal/gameengine` to keep the regression honest. Walk up from
// this file's directory to find the module root (the same logic
// astload's corpusPath uses), then resolve the engine path. Skipped
// if the engine source can't be located (fresh checkout edge case).
func TestSaturationFloor_NoHighSignalFindings(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	var engineDir string
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "internal", "gameengine")
		// Try this candidate by attempting an analyze; if the path
		// doesn't exist AnalyzePackage returns an error.
		if _, err := AnalyzePackage(candidate); err == nil {
			engineDir = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if engineDir == "" {
		t.Skip("internal/gameengine source not found within 6 parents; can't run saturation integration test")
	}

	// Reset dir for refScan to module root.
	moduleRoot := filepath.Dir(filepath.Dir(engineDir))
	res, err := AnalyzePackageWithScope(engineDir, []string{
		filepath.Join(moduleRoot, "internal"),
		filepath.Join(moduleRoot, "cmd"),
	})
	if err != nil {
		t.Fatalf("AnalyzePackageWithScope: %v", err)
	}

	var (
		nCardName   int
		nASTEnum    int
		nDocumented int
		highSignal  []CaseLiteral
	)
	for _, c := range res.UnusedSwitchCases {
		switch classifyCase(c) {
		case "card-name":
			nCardName++
		case "ast-enum":
			nASTEnum++
		case "high-signal-documented":
			nDocumented++
		default:
			highSignal = append(highSignal, c)
		}
	}

	t.Logf("saturation: card-name=%d ast-enum=%d documented=%d high-signal=%d (total=%d)",
		nCardName, nASTEnum, nDocumented, len(highSignal), len(res.UnusedSwitchCases))

	if len(highSignal) > 0 {
		t.Errorf("expected 0 high-signal findings (saturation floor); got %d", len(highSignal))
		for _, c := range highSignal {
			t.Errorf("  unclassified: %q at %s:%d (tag=%q)", c.Value, c.File, c.Line, c.SwitchTag)
		}
	}
}
