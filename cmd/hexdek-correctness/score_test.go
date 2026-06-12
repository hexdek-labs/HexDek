package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
)

func TestPct(t *testing.T) {
	if got := pct(0, 0); got != 0 {
		t.Errorf("pct(0,0) = %v, want 0", got)
	}
	if got := pct(3, 4); got != 75 {
		t.Errorf("pct(3,4) = %v, want 75", got)
	}
	if got := pct(4, 4); got != 100 {
		t.Errorf("pct(4,4) = %v, want 100", got)
	}
}

func TestTopline_MeanOverDimensionsWithData(t *testing.T) {
	dims := []DimensionScore{
		{Dimension: judge.DimensionLegality, Checked: 100, Passed: 90, Pct: 90},
		{Dimension: judge.DimensionConservation, Checked: 10, Passed: 5, Pct: 50},
		{Dimension: judge.DimensionOutcome, Checked: 0, Passed: 0, Pct: 0}, // no data — excluded
	}
	if got := topline(dims); got != 70 {
		t.Errorf("topline = %v, want 70 (mean of 90 and 50, zero-data dimension excluded)", got)
	}
	if got := topline(nil); got != 0 {
		t.Errorf("topline(nil) = %v, want 0", got)
	}
}

func TestAssembleScore_PinsDimensionOrder(t *testing.T) {
	dims := []DimensionScore{
		{Dimension: judge.DimensionOutcome, Checked: 1, Passed: 1, Pct: 100},
		{Dimension: judge.DimensionLegality, Checked: 1, Passed: 1, Pct: 100},
		{Dimension: judge.DimensionProgression, Checked: 1, Passed: 1, Pct: 100},
		{Dimension: judge.DimensionStateIntegrity, Checked: 1, Passed: 1, Pct: 100},
		{Dimension: judge.DimensionConservation, Checked: 1, Passed: 1, Pct: 100},
	}
	s := assembleScore("2026-06-12T00:00:00Z", map[string]string{"games": "1"}, dims, nil, nil)
	want := []string{
		judge.DimensionLegality,
		judge.DimensionConservation,
		judge.DimensionStateIntegrity,
		judge.DimensionProgression,
		judge.DimensionOutcome,
	}
	if len(s.Dimensions) != len(want) {
		t.Fatalf("got %d dimensions, want %d", len(s.Dimensions), len(want))
	}
	for i, w := range want {
		if s.Dimensions[i].Dimension != w {
			t.Errorf("dimension[%d] = %s, want %s", i, s.Dimensions[i].Dimension, w)
		}
	}
	if s.ToplinePct != 100 {
		t.Errorf("topline = %v, want 100", s.ToplinePct)
	}
	if s.SchemaVersion != scoreSchemaVersion {
		t.Errorf("schema version = %d, want %d", s.SchemaVersion, scoreSchemaVersion)
	}
}

func TestRenderJSON_RoundTrips(t *testing.T) {
	s := assembleScore("2026-06-12T00:00:00Z", map[string]string{"games": "2"},
		[]DimensionScore{{Dimension: judge.DimensionLegality, Unit: "actions", Checked: 4, Passed: 3, Pct: 75}},
		&GamePassStats{Games: 2, Seats: 4, ViolationsByName: map[string]int{"invariants/ZoneConservation": 7}},
		[]string{"note"})
	b, err := renderJSON(s)
	if err != nil {
		t.Fatalf("renderJSON: %v", err)
	}
	var back Score
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.ToplinePct != 75 || len(back.Dimensions) != 1 || back.Dimensions[0].Checked != 4 {
		t.Errorf("round-trip mismatch: %+v", back)
	}
	if back.GamePass == nil || back.GamePass.ViolationsByName["invariants/ZoneConservation"] != 7 {
		t.Errorf("game pass round-trip mismatch: %+v", back.GamePass)
	}
}

func TestRenderMarkdown_ContainsHeadlineAndRows(t *testing.T) {
	s := assembleScore("2026-06-12T00:00:00Z", nil,
		[]DimensionScore{
			{Dimension: judge.DimensionLegality, Unit: "actions", Checked: 200, Passed: 150, Pct: 75},
			{Dimension: judge.DimensionOutcome, Unit: "effects", Checked: 0, Passed: 0, Pct: 0},
		},
		&GamePassStats{Games: 3, Seats: 4, MaxTurns: 60, CrashedGames: 1, Crashes: 2,
			GamesAffected: map[string]int{judge.DimensionLegality: 2}},
		[]string{"a note"})
	md := renderMarkdown(s)
	for _, want := range []string{
		"Top-line: 75.00% correct",
		"| legality | actions | 200 | 150 | 75.00% |",
		"| outcome | effects | 0 | 0 | n/a |", // zero-data renders n/a, never 0%
		"Crashes: 2 (in 1 games)",
		"- a note",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}
}

// The game-sweep sink semantics: info-severity violations never fail a
// unit, untagged violations default to state_integrity.
func TestGameTallySemantics_ViaSink(t *testing.T) {
	tally := &gameTally{dims: map[string]int{}, names: map[string]int{}}
	unregister := judge.RegisterSink(func(v judge.ValidationViolation) {
		tally.names[v.Surface+"/"+v.Name]++
		if v.Severity == judge.SeverityInfo {
			return
		}
		dim := v.Dimension
		if dim == "" {
			dim = judge.DimensionStateIntegrity
		}
		tally.dims[dim]++
	})
	defer unregister()

	judge.LogViolation(judge.ValidationViolation{
		Surface: "invariants", Name: "ZoneConservation",
		Severity: judge.SeverityCritical, Dimension: judge.DimensionConservation,
	})
	judge.LogViolation(judge.ValidationViolation{
		Surface: "feynman", Name: "704.5a",
		Severity: judge.SeverityInfo, Dimension: judge.DimensionStateIntegrity,
	})
	judge.LogViolation(judge.ValidationViolation{
		Surface: "invariants", Name: "Untagged", Severity: judge.SeverityCritical,
	})

	if tally.dims[judge.DimensionConservation] != 1 {
		t.Errorf("conservation count = %d, want 1", tally.dims[judge.DimensionConservation])
	}
	// Info-severity excluded; untagged defaults to state_integrity.
	if tally.dims[judge.DimensionStateIntegrity] != 1 {
		t.Errorf("state_integrity count = %d, want 1 (info excluded, untagged included)", tally.dims[judge.DimensionStateIntegrity])
	}
	if tally.names["feynman/704.5a"] != 1 {
		t.Errorf("raw stream should still count info violations by name")
	}
}

// The legality census check must count every observation and flag
// exactly the observations whose checking grew the violation list.
func TestAttachLegalityCensus_CountsAndFlags(t *testing.T) {
	v := gameengine.NewLegalityValidator(1)
	// Replace the built-in checks with a deterministic pair: one that
	// fails casts only, then the census. Register order matters — the
	// census must run after the violating check, as it does in
	// production (NewLegalityValidator installs built-ins first).
	v.Checks = nil
	v.Register(gameengine.LegalityCheck{Name: "fail-casts", Fn: func(_ *gameengine.GameState, obs *gameengine.LegalityObservation) []gameengine.LegalityViolation {
		if obs.Kind == "cast" {
			return []gameengine.LegalityViolation{{Rule: "test", Action: "cast:test"}}
		}
		return nil
	}})
	tally := legalityTally{checkedByKind: map[string]int{}, illegalByKind: map[string]int{}}
	attachLegalityCensus(v, &tally)

	gs := gameengine.NewGameState(2, nil, nil)
	// Two casts (illegal under the synthetic check) + one activation.
	for i := 0; i < 2; i++ {
		obs := v.BeginCast(gs, 0, &gameengine.Card{Name: "T", Owner: 0})
		v.FinishCast(gs, obs, nil)
	}
	obs := v.BeginActivation(gs, 0, &gameengine.Permanent{Card: &gameengine.Card{Name: "P", Owner: 0}}, 0, nil)
	v.FinishActivation(gs, obs, nil)

	if tally.checkedByKind["cast"] != 2 || tally.checkedByKind["activate"] != 1 {
		t.Errorf("checked = %v, want cast:2 activate:1", tally.checkedByKind)
	}
	if tally.illegalByKind["cast"] != 2 {
		t.Errorf("illegal casts = %d, want 2", tally.illegalByKind["cast"])
	}
	if tally.illegalByKind["activate"] != 0 {
		t.Errorf("illegal activations = %d, want 0", tally.illegalByKind["activate"])
	}
}

func TestLegalityTallyTotals_SplitsPlayerActionsFromReplacements(t *testing.T) {
	lt := legalityTally{
		checkedByKind: map[string]int{"cast": 10, "attack": 5, "etb": 100, "zone_change": 200},
		illegalByKind: map[string]int{"cast": 2, "zone_change": 3},
	}
	actChecked, actIllegal := lt.totals(playerActionKinds, true)
	if actChecked != 15 || actIllegal != 2 {
		t.Errorf("player actions = %d/%d, want 15 checked / 2 illegal", actIllegal, actChecked)
	}
	repChecked, repIllegal := lt.totals(playerActionKinds, false)
	if repChecked != 300 || repIllegal != 3 {
		t.Errorf("replacements = %d/%d, want 300 checked / 3 illegal", repIllegal, repChecked)
	}
}
