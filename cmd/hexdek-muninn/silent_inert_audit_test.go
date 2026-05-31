package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/muninn"
)

// silent_inert_audit_test.go — pins the Genesis Chamber detection
// pattern. The 22,662-hit / multi-game / zero-effect signature
// that PR #815 surfaced manually must classify as `critical` under
// the default thresholds. Below the watch threshold, entries fall
// into `trivial` and are excluded from the per-tier text output.

// TestClassifySeverity_TierBoundaries pins the threshold semantics:
// at-the-threshold counts land in the HIGHER tier (>= comparison),
// just-below counts land in the next tier down. This matters because
// the dispatch's stated tier semantics (critical >= 1000) need to
// match exactly — a 1000-hit handler IS critical, not moderate.
func TestClassifySeverity_TierBoundaries(t *testing.T) {
	const (
		critical = 1000
		moderate = 100
		watch    = 10
	)
	cases := []struct {
		hitCount int
		want     SilentInertSeverity
		label    string
	}{
		{22662, SeverityCritical, "Genesis Chamber observed scope value (PR #815)"},
		{1000, SeverityCritical, "exactly at critical threshold"},
		{999, SeverityModerate, "one below critical"},
		{100, SeverityModerate, "exactly at moderate threshold"},
		{99, SeverityWatch, "one below moderate"},
		{10, SeverityWatch, "exactly at watch threshold"},
		{9, SeverityTrivial, "one below watch"},
		{0, SeverityTrivial, "zero hits"},
	}
	for _, c := range cases {
		got := classifySeverity(c.hitCount, critical, moderate, watch)
		if got != c.want {
			t.Errorf("classifySeverity(%d) = %q, want %q  (%s)", c.hitCount, got, c.want, c.label)
		}
	}
}

// TestBuildCandidates_GenesisChamberScenario reconstructs the
// PR #815 scenario: Genesis Chamber accumulating 22,662 hits across
// many games while a hand-curated set of low-noise entries (a stray
// 5-hit trigger from a single test) coexists. Asserts the candidate
// list is sorted with the critical hit at the top, per-game-average
// calculated correctly, and the trivial entry is still in the
// candidate slice (excluded from text output only, not the data).
func TestBuildCandidates_GenesisChamberScenario(t *testing.T) {
	triggers := []muninn.DeadTrigger{
		{
			TriggerName: "triggered_ability",
			CardName:    "Genesis Chamber",
			Count:       22662,
			GamesSeen:   142,
			LastSeen:    "2026-05-08T12:00:00Z",
		},
		{
			TriggerName: "triggered_ability",
			CardName:    "Anafenza, Kin-Tree Spirit",
			Count:       450,
			GamesSeen:   38,
			LastSeen:    "2026-05-08T12:00:00Z",
		},
		{
			TriggerName: "triggered_ability",
			CardName:    "Rivaz of the Claw",
			Count:       45,
			GamesSeen:   12,
			LastSeen:    "2026-05-08T12:00:00Z",
		},
		{
			TriggerName: "triggered_ability",
			CardName:    "Stray Test Card",
			Count:       5,
			GamesSeen:   1,
			LastSeen:    "2026-05-08T12:00:00Z",
		},
	}
	opts := SilentInertAuditOpts{
		CriticalThreshold: 1000,
		ModerateThreshold: 100,
		WatchThreshold:    10,
	}
	got := buildCandidates(triggers, opts)
	if len(got) != 4 {
		t.Fatalf("buildCandidates returned %d entries, want 4", len(got))
	}
	// First entry MUST be Genesis Chamber (critical, highest count).
	if got[0].CardName != "Genesis Chamber" {
		t.Errorf("got[0] = %s, want Genesis Chamber (critical tier should sort first)", got[0].CardName)
	}
	if got[0].Severity != SeverityCritical {
		t.Errorf("Genesis Chamber severity = %q, want critical", got[0].Severity)
	}
	// Per-game-average should be hit_count / games_seen.
	wantAvg := 22662.0 / 142.0
	if got[0].PerGameAvg < wantAvg-0.01 || got[0].PerGameAvg > wantAvg+0.01 {
		t.Errorf("Genesis Chamber PerGameAvg = %.2f, want %.2f", got[0].PerGameAvg, wantAvg)
	}
	// Anafenza: moderate (450 in [100, 1000)).
	if got[1].CardName != "Anafenza, Kin-Tree Spirit" || got[1].Severity != SeverityModerate {
		t.Errorf("got[1] = (%s, %q), want (Anafenza, Kin-Tree Spirit, moderate)", got[1].CardName, got[1].Severity)
	}
	// Rivaz: watch (45 in [10, 100)).
	if got[2].CardName != "Rivaz of the Claw" || got[2].Severity != SeverityWatch {
		t.Errorf("got[2] = (%s, %q), want (Rivaz of the Claw, watch)", got[2].CardName, got[2].Severity)
	}
	// Stray: trivial (5 < 10).
	if got[3].CardName != "Stray Test Card" || got[3].Severity != SeverityTrivial {
		t.Errorf("got[3] = (%s, %q), want (Stray Test Card, trivial)", got[3].CardName, got[3].Severity)
	}
}

// TestSummarize_TierCounts pins the per-tier rollup.
func TestSummarize_TierCounts(t *testing.T) {
	candidates := []SilentInertCandidate{
		{CardName: "A", Severity: SeverityCritical},
		{CardName: "B", Severity: SeverityCritical},
		{CardName: "C", Severity: SeverityModerate},
		{CardName: "D", Severity: SeverityWatch},
		{CardName: "E", Severity: SeverityWatch},
		{CardName: "F", Severity: SeverityWatch},
		{CardName: "G", Severity: SeverityTrivial},
	}
	r := summarize(candidates, SilentInertAuditOpts{
		CriticalThreshold: 1000,
		ModerateThreshold: 100,
		WatchThreshold:    10,
	})
	if r.TotalEntries != 7 {
		t.Errorf("TotalEntries = %d, want 7", r.TotalEntries)
	}
	if r.CriticalCount != 2 || r.ModerateCount != 1 || r.WatchCount != 3 || r.TrivialCount != 1 {
		t.Errorf("tier counts = (c=%d, m=%d, w=%d, t=%d), want (2, 1, 3, 1)",
			r.CriticalCount, r.ModerateCount, r.WatchCount, r.TrivialCount)
	}
	if r.Thresholds.Critical != 1000 || r.Thresholds.Moderate != 100 || r.Thresholds.Watch != 10 {
		t.Errorf("thresholds echoed wrong: got %+v", r.Thresholds)
	}
}

// TestRunSilentInertAudit_TextFormat smoke-tests the text writer by
// driving it via the public CLI entry point. Asserts the
// human-readable shape includes both the per-tier banner counts
// and the critical-tier candidate line.
func TestRunSilentInertAudit_TextFormat(t *testing.T) {
	dir := t.TempDir()
	if err := muninn.PersistDeadTriggersRaw(dir, []muninn.DeadTrigger{
		{TriggerName: "triggered_ability", CardName: "Genesis Chamber", Count: 22662, GamesSeen: 142, LastSeen: "2026-05-08T12:00:00Z"},
		{TriggerName: "triggered_ability", CardName: "Light Noise", Count: 5, GamesSeen: 1, LastSeen: "2026-05-08T12:00:00Z"},
	}); err != nil {
		t.Fatalf("persist seed fixture: %v", err)
	}
	var buf bytes.Buffer
	exitCode, err := runSilentInertAudit(&buf, SilentInertAuditOpts{
		Dir:               dir,
		Format:            "text",
		CriticalThreshold: 1000,
		ModerateThreshold: 100,
		WatchThreshold:    10,
	})
	if err != nil {
		t.Fatalf("runSilentInertAudit: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0 (FailOnCritical was false even though critical surfaced)", exitCode)
	}
	out := buf.String()
	if !strings.Contains(out, "CRITICAL: 1") {
		t.Errorf("text output missing 'CRITICAL: 1' banner; got:\n%s", out)
	}
	if !strings.Contains(out, "Genesis Chamber") {
		t.Errorf("text output missing 'Genesis Chamber' line; got:\n%s", out)
	}
	if strings.Contains(out, "Light Noise") {
		t.Errorf("text output should NOT print trivial-tier 'Light Noise' (5 < watch=10); got:\n%s", out)
	}
}

// TestRunSilentInertAudit_FailOnCritical pins the CI semantics:
// --audit-fail-on-critical returns exit code 1 when any critical
// candidate surfaces, exit code 0 when none.
func TestRunSilentInertAudit_FailOnCritical(t *testing.T) {
	dir := t.TempDir()
	if err := muninn.PersistDeadTriggersRaw(dir, []muninn.DeadTrigger{
		{TriggerName: "triggered_ability", CardName: "Genesis Chamber", Count: 22662, GamesSeen: 142, LastSeen: "2026-05-08T12:00:00Z"},
	}); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var buf bytes.Buffer
	exitCode, err := runSilentInertAudit(&buf, SilentInertAuditOpts{
		Dir:               dir,
		Format:            "text",
		CriticalThreshold: 1000,
		ModerateThreshold: 100,
		WatchThreshold:    10,
		FailOnCritical:    true,
	})
	if err != nil {
		t.Fatalf("audit error: %v", err)
	}
	if exitCode != 1 {
		t.Errorf("exitCode = %d, want 1 (critical surfaced + FailOnCritical=true)", exitCode)
	}

	// Same fixture, but threshold raised above the count → no critical → exit 0.
	exitCode, err = runSilentInertAudit(&bytes.Buffer{}, SilentInertAuditOpts{
		Dir:               dir,
		Format:            "text",
		CriticalThreshold: 50000, // above 22,662
		ModerateThreshold: 100,
		WatchThreshold:    10,
		FailOnCritical:    true,
	})
	if err != nil {
		t.Fatalf("audit error (raised threshold): %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d with raised threshold, want 0 (no critical surfaced)", exitCode)
	}
}

// TestRunSilentInertAudit_JSONFormat pins the JSON pipeline shape.
// Downstream consumers (CI dashboards, alerting) need stable field
// names. Asserts the wire format includes both the candidate list
// and the threshold echo.
func TestRunSilentInertAudit_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	if err := muninn.PersistDeadTriggersRaw(dir, []muninn.DeadTrigger{
		{TriggerName: "triggered_ability", CardName: "Genesis Chamber", Count: 22662, GamesSeen: 142, LastSeen: "2026-05-08T12:00:00Z"},
		{TriggerName: "triggered_ability", CardName: "Anafenza, Kin-Tree Spirit", Count: 450, GamesSeen: 38, LastSeen: "2026-05-08T12:00:00Z"},
	}); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var buf bytes.Buffer
	_, err := runSilentInertAudit(&buf, SilentInertAuditOpts{
		Dir:               dir,
		Format:            "json",
		CriticalThreshold: 1000,
		ModerateThreshold: 100,
		WatchThreshold:    10,
	})
	if err != nil {
		t.Fatalf("audit error: %v", err)
	}
	var got SilentInertAuditResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON decode failed (unstable shape): %v\nraw:\n%s", err, buf.String())
	}
	if got.TotalEntries != 2 {
		t.Errorf("TotalEntries = %d, want 2", got.TotalEntries)
	}
	if got.CriticalCount != 1 {
		t.Errorf("CriticalCount = %d, want 1", got.CriticalCount)
	}
	if got.ModerateCount != 1 {
		t.Errorf("ModerateCount = %d, want 1", got.ModerateCount)
	}
	if got.Thresholds.Critical != 1000 {
		t.Errorf("Thresholds.Critical = %d, want 1000", got.Thresholds.Critical)
	}
	if len(got.Candidates) != 2 || got.Candidates[0].CardName != "Genesis Chamber" {
		t.Errorf("Candidates[0] = %+v, want Genesis Chamber first", got.Candidates[0])
	}
}

// TestRunSilentInertAudit_TSVFormat pins the TSV shape for
// spreadsheet/grep pipelines. Asserts a header row + per-candidate
// rows.
func TestRunSilentInertAudit_TSVFormat(t *testing.T) {
	dir := t.TempDir()
	if err := muninn.PersistDeadTriggersRaw(dir, []muninn.DeadTrigger{
		{TriggerName: "triggered_ability", CardName: "Genesis Chamber", Count: 22662, GamesSeen: 142, LastSeen: "2026-05-08T12:00:00Z"},
	}); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var buf bytes.Buffer
	_, err := runSilentInertAudit(&buf, SilentInertAuditOpts{
		Dir:               dir,
		Format:            "tsv",
		CriticalThreshold: 1000,
		ModerateThreshold: 100,
		WatchThreshold:    10,
	})
	if err != nil {
		t.Fatalf("audit error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("TSV: got %d lines (header + 1 row expected), raw:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "severity\thit_count\t") {
		t.Errorf("TSV header malformed: %q", lines[0])
	}
	if !strings.Contains(lines[1], "critical\t22662\t142\t") {
		t.Errorf("TSV row missing critical / 22662 / 142 cells: %q", lines[1])
	}
}

// TestRunSilentInertAudit_UnknownFormatErrors pins the rejection
// path for unknown --audit-format values.
func TestRunSilentInertAudit_UnknownFormatErrors(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	_, err := runSilentInertAudit(&buf, SilentInertAuditOpts{
		Dir:    dir,
		Format: "yaml",
	})
	if err == nil {
		t.Fatal("expected error for unknown --audit-format, got nil")
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Errorf("error message should name the bad format; got: %v", err)
	}
}
