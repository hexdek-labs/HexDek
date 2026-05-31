package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loki_replay_test.go — pins the judge ↔ Loki replay integration.
// Tests cover:
//
//   - Replay JSON loads and produces one ViolationAnalysis per entry
//   - Each known invariant maps to its CR citation list
//   - The embedded SBASnapshot runs through the §704 probe and the
//     findings show up as SBAFindings
//   - Unmapped invariant names land in UnmappedInvariants without
//     erroring (forward-compat for new engine invariants)
//   - The replay-without-snapshot path (Loki today) still produces a
//     CR-citation explanation even though SBAFindings is empty
//   - JSON output schema is stable across runs
//
// The fixtures here are hand-authored — Loki doesn't yet emit this
// format, but the shape is defined as the forward contract.

// writeReplayFixture marshals a LokiReplay to a temp file and returns
// the path.
func writeReplayFixture(t *testing.T, r LokiReplay) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "replay.json")
	body, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runReplay(t *testing.T, r LokiReplay) *ReplayAnalysis {
	t.Helper()
	path := writeReplayFixture(t, r)
	rep, err := runReplayAnalysis(path, "")
	if err != nil {
		t.Fatalf("runReplayAnalysis: %v", err)
	}
	return rep
}

// ---------------------------------------------------------------------------
// LifeConsistency — with embedded snapshot whose seat is at life=0.
// SBA probe must fire §704.5a; CR citations must include both §704.5a
// and §119.
// ---------------------------------------------------------------------------

func TestReplay_LifeConsistency_WithSnapshot_FiresSBAAndCRMap(t *testing.T) {
	replay := LokiReplay{
		Meta: LokiReplayMeta{Source: "hand-rolled-fixture", Seed: 42},
		Violations: []LokiReplayViolation{
			{
				GameIdx:       137,
				GameSeed:      42,
				Turn:          12,
				InvariantName: "LifeConsistency",
				Message:       "seat 0 is alive but life=0",
				Commanders:    []string{"Edgar Markov", "Atraxa, Praetors' Voice"},
				SBASnapshot: &SBASnapshot{
					Format: "commander",
					Turn:   12,
					Seats:  []SBASeat{{Idx: 0, Life: 0}, {Idx: 1, Life: 30}},
				},
			},
		},
	}
	rep := runReplay(t, replay)
	if rep.ViolationCount != 1 {
		t.Errorf("ViolationCount = %d, want 1", rep.ViolationCount)
	}
	if rep.WithSnapshot != 1 {
		t.Errorf("WithSnapshot = %d, want 1", rep.WithSnapshot)
	}
	a := rep.Analyses[0]
	// CR citation map: LifeConsistency → 704.5a + 119.
	if len(a.CRCitations) < 2 {
		t.Fatalf("CRCitations = %d, want ≥2 (704.5a + 119): %+v", len(a.CRCitations), a.CRCitations)
	}
	want := map[string]bool{"704.5a": false, "119": false}
	for _, c := range a.CRCitations {
		if _, ok := want[c.Rule]; ok {
			want[c.Rule] = true
		}
	}
	for rule, found := range want {
		if !found {
			t.Errorf("missing expected CR citation %q in %+v", rule, a.CRCitations)
		}
	}
	// SBA probe ran on the embedded snapshot and found 704.5a.
	foundSBA := false
	for _, f := range a.SBAFindings {
		if f.Rule == "704.5a" {
			foundSBA = true
		}
	}
	if !foundSBA {
		t.Errorf("SBAFindings missing 704.5a from embedded snapshot: %+v", a.SBAFindings)
	}
	// Summary string mentions both the CR citation AND the SBA count.
	if !strings.Contains(a.Summary, "704.5a") {
		t.Errorf("Summary doesn't mention 704.5a: %q", a.Summary)
	}
	if !strings.Contains(a.Summary, "SBA condition") {
		t.Errorf("Summary doesn't mention SBA condition count: %q", a.Summary)
	}
}

// ---------------------------------------------------------------------------
// ZoneConservation — multi-citation invariant. CR map should include
// §400.6, §109.3, and the §704.5d/e ephemeral-cleanup paths.
// ---------------------------------------------------------------------------

func TestReplay_ZoneConservation_CRMapIncludesAllCitations(t *testing.T) {
	replay := LokiReplay{
		Violations: []LokiReplayViolation{
			{
				GameIdx:       420,
				InvariantName: "ZoneConservation",
				Message:       "extra real cards appeared",
				Turn:          47,
			},
		},
	}
	rep := runReplay(t, replay)
	a := rep.Analyses[0]
	wantRules := []string{"400.6", "109.3", "704.5d", "704.5e"}
	got := map[string]bool{}
	for _, c := range a.CRCitations {
		got[c.Rule] = true
	}
	for _, w := range wantRules {
		if !got[w] {
			t.Errorf("missing expected CR citation §%s for ZoneConservation; got: %+v",
				w, a.CRCitations)
		}
	}
}

// ---------------------------------------------------------------------------
// AttachmentConsistency — covers §704.5k / .5m / .5n + §303.4.
// ---------------------------------------------------------------------------

func TestReplay_AttachmentConsistency_FullCRSet(t *testing.T) {
	replay := LokiReplay{
		Violations: []LokiReplayViolation{
			{
				GameIdx:       12,
				InvariantName: "AttachmentConsistency",
				Message:       "Aura attached to creature no longer on battlefield",
			},
		},
	}
	rep := runReplay(t, replay)
	a := rep.Analyses[0]
	want := map[string]bool{
		"704.5k": false, "704.5m": false, "704.5n": false, "303.4": false,
	}
	for _, c := range a.CRCitations {
		if _, ok := want[c.Rule]; ok {
			want[c.Rule] = true
		}
	}
	for rule, found := range want {
		if !found {
			t.Errorf("AttachmentConsistency: missing §%s; got %+v", rule, a.CRCitations)
		}
	}
}

// ---------------------------------------------------------------------------
// Replay without snapshot — common Loki case today. CR explanation
// must still ship; SBAFindings should be empty.
// ---------------------------------------------------------------------------

func TestReplay_NoSnapshot_StillExplains(t *testing.T) {
	replay := LokiReplay{
		Violations: []LokiReplayViolation{
			{
				GameIdx:       1,
				InvariantName: "WinCondition",
				Message:       "game ended but no seat marked Won",
			},
		},
	}
	rep := runReplay(t, replay)
	a := rep.Analyses[0]
	if len(a.CRCitations) == 0 {
		t.Errorf("expected CR citations for WinCondition even without snapshot, got none")
	}
	if len(a.SBAFindings) != 0 {
		t.Errorf("SBAFindings should be empty without snapshot, got %+v", a.SBAFindings)
	}
	if rep.WithSnapshot != 0 {
		t.Errorf("WithSnapshot = %d, want 0", rep.WithSnapshot)
	}
	if a.Summary == "" {
		t.Errorf("Summary should be non-empty even without snapshot")
	}
}

// ---------------------------------------------------------------------------
// Unmapped invariant — forward-compat: when AllInvariants() adds a new
// entry that isn't in invariantCRCitations, the replay analyzer
// shouldn't error; it should record the name in UnmappedInvariants so
// the maintainer knows to update the map.
// ---------------------------------------------------------------------------

func TestReplay_UnmappedInvariant_RecordedWithoutError(t *testing.T) {
	replay := LokiReplay{
		Violations: []LokiReplayViolation{
			{
				GameIdx:       1,
				InvariantName: "NewBleedingEdgeInvariant",
				Message:       "something went wrong",
			},
		},
	}
	rep := runReplay(t, replay)
	if len(rep.UnmappedInvariants) != 1 || rep.UnmappedInvariants[0] != "NewBleedingEdgeInvariant" {
		t.Errorf("UnmappedInvariants = %v, want [NewBleedingEdgeInvariant]",
			rep.UnmappedInvariants)
	}
	if len(rep.Analyses[0].CRCitations) != 0 {
		t.Errorf("unmapped invariant should have 0 CR citations, got %+v",
			rep.Analyses[0].CRCitations)
	}
	// Summary should still ship a clear "no citation mapped" message.
	if !strings.Contains(rep.Analyses[0].Summary, "no CR citation mapped") {
		t.Errorf("Summary doesn't flag unmapped status: %q", rep.Analyses[0].Summary)
	}
}

// ---------------------------------------------------------------------------
// Multi-violation replay — every analysis present, InvariantsSeen tally
// correct, deterministic order matches input.
// ---------------------------------------------------------------------------

func TestReplay_MultiViolation_TallyAndOrder(t *testing.T) {
	replay := LokiReplay{
		Meta: LokiReplayMeta{Source: "loki-fixture-multi", TotalGames: 1000},
		Violations: []LokiReplayViolation{
			{GameIdx: 1, InvariantName: "LifeConsistency", Message: "msg1"},
			{GameIdx: 2, InvariantName: "ZoneConservation", Message: "msg2"},
			{GameIdx: 3, InvariantName: "LifeConsistency", Message: "msg3"},
			{GameIdx: 4, InvariantName: "SBACompleteness", Message: "msg4"},
		},
	}
	rep := runReplay(t, replay)
	if rep.ViolationCount != 4 {
		t.Errorf("ViolationCount = %d, want 4", rep.ViolationCount)
	}
	if rep.InvariantsSeen["LifeConsistency"] != 2 {
		t.Errorf("LifeConsistency count = %d, want 2", rep.InvariantsSeen["LifeConsistency"])
	}
	if rep.InvariantsSeen["ZoneConservation"] != 1 {
		t.Errorf("ZoneConservation count = %d, want 1", rep.InvariantsSeen["ZoneConservation"])
	}
	// Input order preserved.
	for i, want := range []string{"LifeConsistency", "ZoneConservation", "LifeConsistency", "SBACompleteness"} {
		if rep.Analyses[i].InvariantName != want {
			t.Errorf("Analyses[%d].InvariantName = %q, want %q",
				i, rep.Analyses[i].InvariantName, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Embedded snapshot with multiple SBA conditions — all surface in
// SBAFindings; summary mentions the count.
// ---------------------------------------------------------------------------

func TestReplay_SnapshotWithMultipleSBAConditions(t *testing.T) {
	replay := LokiReplay{
		Violations: []LokiReplayViolation{
			{
				GameIdx:       1,
				InvariantName: "SBACompleteness",
				Message:       "multiple SBA conditions present",
				SBASnapshot: &SBASnapshot{
					Seats: []SBASeat{{
						Idx: 0, Life: 0, PoisonCounters: 12,
						Battlefield: []SBAPermanent{
							{Name: "Wisp", Types: []string{"creature"}, BaseToughness: 0},
							{Name: "Spent Jace", Types: []string{"planeswalker"}, Loyalty: 0},
						},
					}},
				},
			},
		},
	}
	rep := runReplay(t, replay)
	a := rep.Analyses[0]
	// Expect 4 SBA findings: 704.5a + 704.5c + 704.5f + 704.5h.
	if len(a.SBAFindings) < 4 {
		t.Errorf("SBAFindings = %d, want ≥4: %+v", len(a.SBAFindings), a.SBAFindings)
	}
	if !strings.Contains(a.Summary, "4 §704 SBA condition") &&
		!strings.Contains(a.Summary, "5 §704 SBA condition") {
		t.Errorf("Summary doesn't report ≥4 SBA conditions: %q", a.Summary)
	}
}

// ---------------------------------------------------------------------------
// Meta preservation — the replay's Meta field round-trips into the
// analysis output so downstream tooling can correlate.
// ---------------------------------------------------------------------------

func TestReplay_MetaRoundTripsIntoAnalysis(t *testing.T) {
	replay := LokiReplay{
		Meta: LokiReplayMeta{
			Source:     "hexdek-loki r60.8",
			CorpusSize: 31963,
			TotalGames: 5000,
			Seed:       42,
			RunAt:      "2026-05-30T19:00:00Z",
		},
		Violations: []LokiReplayViolation{
			{GameIdx: 1, InvariantName: "LifeConsistency", Message: "x"},
		},
	}
	rep := runReplay(t, replay)
	if rep.Meta.Source != "hexdek-loki r60.8" {
		t.Errorf("Meta.Source = %q, want hexdek-loki r60.8", rep.Meta.Source)
	}
	if rep.Meta.CorpusSize != 31963 {
		t.Errorf("Meta.CorpusSize = %d, want 31963", rep.Meta.CorpusSize)
	}
	if rep.Meta.Seed != 42 {
		t.Errorf("Meta.Seed = %d, want 42", rep.Meta.Seed)
	}
}

// ---------------------------------------------------------------------------
// JSON output schema lock
// ---------------------------------------------------------------------------

func TestReplay_OutputJSONShape(t *testing.T) {
	path := writeReplayFixture(t, LokiReplay{
		Meta: LokiReplayMeta{Source: "schema-test"},
		Violations: []LokiReplayViolation{
			{
				GameIdx:       1,
				InvariantName: "LifeConsistency",
				Message:       "seat 0 alive at life=0",
				SBASnapshot: &SBASnapshot{
					Seats: []SBASeat{{Idx: 0, Life: 0}},
				},
			},
		},
	})
	tmpOut := filepath.Join(filepath.Dir(path), "out.json")
	if _, err := runReplayAnalysis(path, tmpOut); err != nil {
		t.Fatalf("runReplayAnalysis: %v", err)
	}
	raw, err := os.ReadFile(tmpOut)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"rule"`, `"replay_path"`, `"meta"`, `"violation_count"`,
		`"with_snapshot"`, `"analyses"`, `"invariants_seen"`, `"valid"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing top-level key %s in:\n%s", key, raw)
		}
	}
	for _, key := range []string{
		`"game_idx"`, `"invariant_name"`, `"cr_citations"`, `"summary"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing per-analysis field %s in:\n%s", key, raw)
		}
	}
	// Round-trip.
	var rt ReplayAnalysis
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if rt.Rule != "CR §700 (judge ↔ Loki replay)" {
		t.Errorf("Rule = %q", rt.Rule)
	}
}
