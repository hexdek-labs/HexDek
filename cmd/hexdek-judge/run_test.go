package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// run_test.go — Judge gate plumbing pins (r63). The end-to-end gate is
// exercised by CI itself (.github/workflows/judge.yml); these pin the
// baseline round-trip + the CI sample filters.

func TestJudgeBaseline_RoundTripAndShape(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(p, []byte(`{
  "comment": "test",
  "fingerprints": {"game|conservation|invariants|ZoneConservation": 3}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var bl judgeBaseline
	if err := json.Unmarshal(data, &bl); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if bl.Fingerprints["game|conservation|invariants|ZoneConservation"] != 3 {
		t.Fatalf("round-trip lost the fingerprint: %+v", bl)
	}
}

func TestSplitJoinLines(t *testing.T) {
	in := "a\nb\n\nc"
	lines := splitLines(in)
	if len(lines) != 4 || lines[3] != "c" {
		t.Fatalf("splitLines(%q) = %q", in, lines)
	}
	if joinLines([]string{"x", "y"}) != "x\ny\n" {
		t.Fatalf("joinLines wrong: %q", joinLines([]string{"x", "y"}))
	}
}

// TestCommittedBaselineParses guards the in-repo baseline file against
// hand-edit syntax errors — CI reads it on every PR.
func TestCommittedBaselineParses(t *testing.T) {
	data, err := os.ReadFile("../../data/judge/judge-baseline.json")
	if err != nil {
		t.Skipf("baseline not present: %v", err)
	}
	var bl judgeBaseline
	if err := json.Unmarshal(data, &bl); err != nil {
		t.Fatalf("committed baseline does not parse: %v", err)
	}
	if bl.Fingerprints == nil {
		t.Fatalf("committed baseline has no fingerprints map")
	}
}
