package analytics

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// curve_realization_r60_test.go — pins the per-seat per-turn cast-CMC
// tracking surfaced as part of the Heimdall analytics enhancement
// (PR opened from `dev/heimdall-analytics-enhance-r60`).
//
// CurveRealization data flow:
//   1. Engine emits `cast` events with Amount = mana cost paid.
//   2. analyzer.go tracks each cast on PlayerAnalysis.CastsByTurn[turn]
//      and accumulates TotalSpellCMC.
//   3. finalizeCurveRealization computes AvgCMCCast = TotalSpellCMC /
//      total spell count when AnalyzeGame returns.
//   4. report.go renders a per-commander row aggregating across all
//      games the deck played.

// mkCastEvent is a compact helper for building the canonical cast
// event shape the analyzer expects.
func mkCastEvent(seat int, cardName string, cmc int) gameengine.Event {
	return gameengine.Event{
		Kind:   "cast",
		Seat:   seat,
		Source: cardName,
		Amount: cmc,
		Details: map[string]interface{}{
			"rule": "601.2",
		},
	}
}

// TestCurveRealization_PerTurnBuckets pins the basic shape: 4 casts
// on turns 2, 3, 3, 5 land in the right buckets, with AvgCMCCast
// reflecting the total.
func TestCurveRealization_PerTurnBuckets(t *testing.T) {
	events := []gameengine.Event{
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 1}},
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 2}},
		mkCastEvent(0, "Sol Ring", 1),
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 3}},
		mkCastEvent(0, "Cultivate", 3),
		mkCastEvent(0, "Beast Within", 3),
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 5}},
		mkCastEvent(0, "Wrath of God", 4),
	}
	ga := AnalyzeGame(events, 1, []string{"Cmdr A"}, -1, 5, []int{7}, []int{40})
	pa := &ga.Players[0]

	// CastsByTurn[2] = [1], [3] = [3,3], [5] = [4].
	if got := len(pa.CastsByTurn); got < 6 {
		t.Fatalf("CastsByTurn len = %d, want >= 6 (indexed by turn)", got)
	}
	if got := pa.CastsByTurn[2]; len(got) != 1 || got[0] != 1 {
		t.Errorf("turn 2 = %v, want [1]", got)
	}
	if got := pa.CastsByTurn[3]; len(got) != 2 || got[0] != 3 || got[1] != 3 {
		t.Errorf("turn 3 = %v, want [3 3]", got)
	}
	if got := pa.CastsByTurn[5]; len(got) != 1 || got[0] != 4 {
		t.Errorf("turn 5 = %v, want [4]", got)
	}
	if pa.TotalSpellCMC != 11 {
		t.Errorf("TotalSpellCMC = %d, want 11", pa.TotalSpellCMC)
	}
	wantAvg := 11.0 / 4.0
	if pa.AvgCMCCast < wantAvg-1e-9 || pa.AvgCMCCast > wantAvg+1e-9 {
		t.Errorf("AvgCMCCast = %f, want %f", pa.AvgCMCCast, wantAvg)
	}
}

// TestCurveRealization_ZeroCostCastsIgnored guards against zero-cost
// casts (cost == 0 spells like Crashing Footfalls cascaded for free)
// inflating the spell count denominator. The engine emits cast with
// Amount == 0 in those paths, and the analyzer intentionally skips
// them — counting them would push AvgCMCCast down without reflecting
// real mana spent.
func TestCurveRealization_ZeroCostCastsIgnored(t *testing.T) {
	events := []gameengine.Event{
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 1}},
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 2}},
		mkCastEvent(0, "Real Cast", 2),
		mkCastEvent(0, "Free Cascade", 0),
	}
	ga := AnalyzeGame(events, 1, []string{"Cmdr"}, -1, 2, []int{7}, []int{40})
	pa := &ga.Players[0]
	if pa.TotalSpellCMC != 2 {
		t.Errorf("TotalSpellCMC = %d, want 2 (zero-cost cast skipped)", pa.TotalSpellCMC)
	}
	if got := pa.CastsByTurn[2]; len(got) != 1 || got[0] != 2 {
		t.Errorf("turn 2 = %v, want [2] (zero-cost cast skipped)", got)
	}
	if pa.AvgCMCCast != 2.0 {
		t.Errorf("AvgCMCCast = %f, want 2.0 (denominator includes only paid casts)", pa.AvgCMCCast)
	}
}

// TestCurveRealization_NoCastsLeavesZero pins the empty-input case:
// a player with no cast events has CastsByTurn empty (or len 0) and
// AvgCMCCast == 0 (not NaN).
func TestCurveRealization_NoCastsLeavesZero(t *testing.T) {
	events := []gameengine.Event{
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 1}},
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 2}},
		// No cast events.
	}
	ga := AnalyzeGame(events, 1, []string{"Cmdr"}, -1, 2, []int{7}, []int{40})
	pa := &ga.Players[0]
	if pa.TotalSpellCMC != 0 {
		t.Errorf("TotalSpellCMC = %d, want 0", pa.TotalSpellCMC)
	}
	if pa.AvgCMCCast != 0 {
		t.Errorf("AvgCMCCast = %f, want 0 (not NaN)", pa.AvgCMCCast)
	}
}

// TestWriteCurveRealization_RenderShape pins the markdown surface:
// header columns T1..T10 + T11+ + AvgAll + Casts; commander row
// includes the right averages with tail-bucket folding.
func TestWriteCurveRealization_RenderShape(t *testing.T) {
	// Build two games for "Cmdr A" — turn 2 always cast a CMC-2 spell,
	// turn 4 cast a CMC-4 spell. Turn 15 (tail) cast CMC-6.
	mk := func() *GameAnalysis {
		events := []gameengine.Event{
			{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 1}},
			{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 2}},
			mkCastEvent(0, "Two-Drop", 2),
			{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 4}},
			mkCastEvent(0, "Four-Drop", 4),
			{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 15}},
			mkCastEvent(0, "Big Threat", 6),
		}
		return AnalyzeGame(events, 1, []string{"Cmdr A"}, -1, 15, []int{0}, []int{40})
	}
	r := &AnalyticsReport{
		Analyses:       []*GameAnalysis{mk(), mk()},
		CommanderNames: []string{"Cmdr A"},
		TotalGames:     2,
	}
	var b strings.Builder
	r.writeCurveRealization(&b)
	out := b.String()

	if !strings.Contains(out, "## Curve Realization") {
		t.Errorf("missing section header; got:\n%s", out)
	}
	for _, want := range []string{
		"| Commander |", "T1 |", "T10 |", "T11+ |", "AvgAll |", "Casts |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing column %q in:\n%s", want, out)
		}
	}
	// Commander row present.
	if !strings.Contains(out, "| Cmdr A |") {
		t.Errorf("Cmdr A row missing; got:\n%s", out)
	}
	// T2=2.0 and T4=4.0 — single CMC value per turn so the avg is the
	// value itself.
	if !strings.Contains(out, " 2.0 |") {
		t.Errorf("expected T2=2.0; got:\n%s", out)
	}
	if !strings.Contains(out, " 4.0 |") {
		t.Errorf("expected T4=4.0; got:\n%s", out)
	}
	// T11+ bucket should reflect the turn-15 CMC-6 cast.
	if !strings.Contains(out, " 6.0 |") {
		t.Errorf("expected T11+=6.0 (tail bucket); got:\n%s", out)
	}
	// Across 2 games, 6 total casts (2+2 turn-2/turn-4/turn-15 each game).
	if !strings.Contains(out, " 6 |") {
		t.Errorf("expected 6 total casts; got:\n%s", out)
	}
}

// TestWriteCurveRealization_EmptyAnalyses pins the no-data guard.
func TestWriteCurveRealization_EmptyAnalyses(t *testing.T) {
	r := &AnalyticsReport{}
	var b strings.Builder
	r.writeCurveRealization(&b)
	out := b.String()
	if !strings.Contains(out, "## Curve Realization") {
		t.Errorf("missing header even when empty; got:\n%s", out)
	}
	if !strings.Contains(out, "No game data") && !strings.Contains(out, "No curve data") {
		t.Errorf("expected empty-state message; got:\n%s", out)
	}
}
