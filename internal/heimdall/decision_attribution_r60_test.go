package heimdall

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// decision_attribution_r60_test.go — R60 audit. Heimdall layers
// decision-class attribution on top of the per-event HatDecision rows
// already extracted from gs.EventLog. Each hat decision is classified
// into a strategic DecisionClass, paired with the game outcome,
// aggregated across a gauntlet of games, and emitted as a per-
// (archetype × decision-class) misweighting table that dev-3's
// recommender / feedback loop consumes.

func TestClassifyDecision_TableMapping(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		details map[string]interface{}
		want    DecisionClass
	}{
		{"mulligan", "mulligan", nil, DCMulliganKeep},
		{"cast_default", "cast", nil, DCCastVsHold},
		{"cast_with_unrelated_alt", "cast",
			map[string]interface{}{"alternative": "cast_smaller"}, DCCastVsHold},
		{"cast_with_hold_interaction", "cast",
			map[string]interface{}{"alternative": "hold_interaction"}, DCManaVsInteraction},
		{"attack_target_default", "attack_target", nil, DCTargetChoice},
		{"attack_target_chose_pass", "attack_target",
			map[string]interface{}{"chose": "pass"}, DCAttackVsHold},
		{"attack_target_chose_no_attack", "attack_target",
			map[string]interface{}{"chose": "no_attack"}, DCAttackVsHold},
		{"block", "block", nil, DCBlockChoice},
		{"response_counter", "response_counter", nil, DCResponseCounter},
		{"mode", "mode", nil, DCModeChoice},
		{"unknown_kind", "weird_new_kind", nil, DCUnclassified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := HatDecision{Kind: tc.kind, Details: tc.details}
			if got := ClassifyDecision(d); got != tc.want {
				t.Errorf("ClassifyDecision(kind=%q details=%v) = %q, want %q",
					tc.kind, tc.details, got, tc.want)
			}
		})
	}
}

func TestAttributeDecisions_PerSeatWonFieldAgainstWinner(t *testing.T) {
	gs := &gameengine.GameState{Turn: 3, EventLog: []gameengine.Event{
		{Kind: "hat_decision_cast", Seat: 0, Amount: 3,
			Details: map[string]interface{}{"score_delta": 0.42, "archetype": "midrange", "turn": 3}},
		{Kind: "hat_decision_attack_target", Seat: 1, Amount: 3,
			Details: map[string]interface{}{"score_delta": 0.10, "archetype": "aggro", "turn": 3}},
		{Kind: "hat_decision_block", Seat: 2, Amount: 3,
			Details: map[string]interface{}{"score_delta": 0.05, "archetype": "control", "turn": 3}},
	}}
	attr := AttributeDecisions(gs, "game-1", 1, []string{"midrange", "aggro", "control", "combo"})
	if attr == nil {
		t.Fatal("AttributeDecisions returned nil for non-nil gs")
	}
	if attr.GameID != "game-1" {
		t.Errorf("GameID = %q, want game-1", attr.GameID)
	}
	if attr.WinnerSeat != 1 {
		t.Errorf("WinnerSeat = %d, want 1", attr.WinnerSeat)
	}
	if len(attr.Decisions) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(attr.Decisions))
	}
	if attr.Decisions[0].Won {
		t.Errorf("seat-0 decision: Won should be false (winner is seat 1)")
	}
	if !attr.Decisions[1].Won {
		t.Errorf("seat-1 decision: Won should be true (seat 1 won)")
	}
	if attr.Decisions[2].Won {
		t.Errorf("seat-2 decision: Won should be false")
	}
	if attr.Decisions[0].ScoreDelta != 0.42 {
		t.Errorf("score_delta on first row = %v, want 0.42", attr.Decisions[0].ScoreDelta)
	}
	if attr.Decisions[0].Class != DCCastVsHold {
		t.Errorf("first row class = %q, want %q", attr.Decisions[0].Class, DCCastVsHold)
	}
}

func TestAttributeDecisions_NilGameState(t *testing.T) {
	if got := AttributeDecisions(nil, "x", -1, nil); got != nil {
		t.Errorf("nil gs must yield nil; got %v", got)
	}
}

func TestAttributeDecisions_NoHatEvents(t *testing.T) {
	gs := &gameengine.GameState{Turn: 1, EventLog: []gameengine.Event{
		{Kind: "cast", Source: "Lightning Bolt"},
	}}
	got := AttributeDecisions(gs, "g", 0, []string{"a", "b", "c", "d"})
	if got == nil {
		t.Fatal("AttributeDecisions returned nil for gs with no hat events; want empty slice")
	}
	if got.Decisions == nil {
		t.Error("Decisions should be empty slice (not nil)")
	}
	if len(got.Decisions) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(got.Decisions))
	}
}

func TestAttributeDecisions_ScoreDeltaIntCoercion(t *testing.T) {
	// In-process tests can stamp Details with int values; coerceFloat
	// must accept them as well as float64 (JSON wire format).
	gs := &gameengine.GameState{Turn: 1, EventLog: []gameengine.Event{
		{Kind: "hat_decision_cast", Seat: 0, Amount: 1,
			Details: map[string]interface{}{"score_delta": int(2), "turn": 1}},
	}}
	attr := AttributeDecisions(gs, "g", 0, nil)
	if attr.Decisions[0].ScoreDelta != 2.0 {
		t.Errorf("int score_delta should coerce to 2.0; got %v", attr.Decisions[0].ScoreDelta)
	}
}

func TestBuildAttributionTable_CountsWinRateAndSort(t *testing.T) {
	games := []HeimdallDecisionAttribution{
		{
			GameID:     "g1",
			WinnerSeat: 0,
			Decisions: []AttributedDecision{
				{HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold, Won: true, ScoreDelta: 0.5},
				{HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold, Won: false, ScoreDelta: 0.3},
				{HatDecision: HatDecision{Archetype: "stax"}, Class: DCBlockChoice, Won: true, ScoreDelta: 0.0},
			},
		},
		{
			GameID:     "g2",
			WinnerSeat: 1,
			Decisions: []AttributedDecision{
				{HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold, Won: false, ScoreDelta: 0.1},
				{HatDecision: HatDecision{Archetype: "aggro"}, Class: DCAttackVsHold, Won: true, ScoreDelta: 0.2},
			},
		},
		{
			GameID:     "g3",
			WinnerSeat: 0,
			Decisions: []AttributedDecision{
				{HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold, Won: true, ScoreDelta: 0.4},
			},
		},
	}
	table := BuildAttributionTable(games, nil)
	if table.GamesProcessed != 3 {
		t.Errorf("GamesProcessed = %d, want 3", table.GamesProcessed)
	}
	if table.TotalDecisions != 6 {
		t.Errorf("TotalDecisions = %d, want 6", table.TotalDecisions)
	}
	if len(table.Metrics) != 3 {
		t.Fatalf("expected 3 cells (combo/cast, stax/block, aggro/attack), got %d", len(table.Metrics))
	}
	// Sort order: Archetype asc, Class asc.
	wantArchOrder := []string{"aggro", "combo", "stax"}
	for i, want := range wantArchOrder {
		if table.Metrics[i].Archetype != want {
			t.Errorf("Metrics[%d].Archetype = %q, want %q", i, table.Metrics[i].Archetype, want)
		}
	}
	// Combo+cast_vs_hold across all 3 games: g1 has 2 (won+lost),
	// g2 has 1 (lost), g3 has 1 (won) → 4 total, 2 wins.
	for _, m := range table.Metrics {
		if m.Archetype == "combo" && m.Class == DCCastVsHold {
			if m.DecisionCount != 4 {
				t.Errorf("combo/cast DecisionCount = %d, want 4", m.DecisionCount)
			}
			if m.WonDecisionCount != 2 {
				t.Errorf("combo/cast WonDecisionCount = %d, want 2", m.WonDecisionCount)
			}
			wantWin := 0.5
			if attrAbsFloat(m.WinRate-wantWin) > 1e-9 {
				t.Errorf("combo/cast WinRate = %v, want %v", m.WinRate, wantWin)
			}
			wantAvgDelta := (0.5 + 0.3 + 0.1 + 0.4) / 4.0
			if attrAbsFloat(m.AvgScoreDelta-wantAvgDelta) > 1e-9 {
				t.Errorf("combo/cast AvgScoreDelta = %v, want %v", m.AvgScoreDelta, wantAvgDelta)
			}
		}
	}
}

func TestBuildAttributionTable_AggregationMath(t *testing.T) {
	// Focused, explicit math test for aggregation.
	games := []HeimdallDecisionAttribution{
		{
			GameID: "g", WinnerSeat: 0,
			Decisions: []AttributedDecision{
				{HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold, Won: true, ScoreDelta: 0.5},
				{HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold, Won: false, ScoreDelta: 0.3},
				{HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold, Won: true, ScoreDelta: 0.1},
				{HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold, Won: false, ScoreDelta: 0.7},
			},
		},
	}
	table := BuildAttributionTable(games, nil)
	if len(table.Metrics) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(table.Metrics))
	}
	m := table.Metrics[0]
	if m.DecisionCount != 4 {
		t.Errorf("DecisionCount = %d, want 4", m.DecisionCount)
	}
	if m.WonDecisionCount != 2 {
		t.Errorf("WonDecisionCount = %d, want 2", m.WonDecisionCount)
	}
	if attrAbsFloat(m.WinRate-0.5) > 1e-9 {
		t.Errorf("WinRate = %v, want 0.5", m.WinRate)
	}
	wantAvg := (0.5 + 0.3 + 0.1 + 0.7) / 4.0
	if attrAbsFloat(m.AvgScoreDelta-wantAvg) > 1e-9 {
		t.Errorf("AvgScoreDelta = %v, want %v", m.AvgScoreDelta, wantAvg)
	}
	// DefaultMisweightEstimator: won=0, lost=abs(delta). Lost: 0.3, 0.7.
	wantMis := (0 + 0.3 + 0 + 0.7) / 4.0
	if attrAbsFloat(m.AvgMisweight-wantMis) > 1e-9 {
		t.Errorf("AvgMisweight = %v, want %v", m.AvgMisweight, wantMis)
	}
}

func TestBuildAttributionTable_UnknownArchetypeBucket(t *testing.T) {
	// Decisions whose archetype is empty AND whose seat has no
	// per-seat archetype fall under "unknown".
	games := []HeimdallDecisionAttribution{{
		GameID: "g", WinnerSeat: -1, SeatArchetypes: nil,
		Decisions: []AttributedDecision{
			{HatDecision: HatDecision{Seat: 0, Archetype: ""}, Class: DCCastVsHold},
			{HatDecision: HatDecision{Seat: 1, Archetype: ""}, Class: DCCastVsHold},
		},
	}}
	table := BuildAttributionTable(games, nil)
	if len(table.Metrics) != 1 {
		t.Fatalf("expected 1 cell (unknown/cast_vs_hold), got %d", len(table.Metrics))
	}
	if table.Metrics[0].Archetype != "unknown" {
		t.Errorf("Archetype = %q, want unknown", table.Metrics[0].Archetype)
	}
}

func TestDefaultMisweightEstimator(t *testing.T) {
	cases := []struct {
		name string
		d    AttributedDecision
		want float64
	}{
		{"won_returns_zero", AttributedDecision{Won: true, ScoreDelta: 1.5}, 0.0},
		{"lost_returns_abs_delta_positive", AttributedDecision{Won: false, ScoreDelta: 0.5}, 0.5},
		{"lost_returns_abs_delta_negative", AttributedDecision{Won: false, ScoreDelta: -0.7}, 0.7},
		{"lost_zero_delta_returns_zero", AttributedDecision{Won: false, ScoreDelta: 0.0}, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DefaultMisweightEstimator(tc.d); attrAbsFloat(got-tc.want) > 1e-9 {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMinSamplesGating_SuggestionEmittedAboveThreshold(t *testing.T) {
	// Build a single cell with exactly MinSamplesForSuggestion-1
	// decisions and verify Suggestion is empty; bump to MinSamples
	// and verify it populates.
	makeGames := func(n int) []HeimdallDecisionAttribution {
		decs := make([]AttributedDecision, n)
		for i := range decs {
			decs[i] = AttributedDecision{
				HatDecision: HatDecision{Archetype: "combo"},
				Class:       DCCastVsHold,
				Won:         true,
				ScoreDelta:  0.0,
			}
		}
		return []HeimdallDecisionAttribution{{GameID: "g", WinnerSeat: 0, Decisions: decs}}
	}
	tBelow := BuildAttributionTable(makeGames(MinSamplesForSuggestion-1), nil)
	if tBelow.Metrics[0].Suggestion != "" {
		t.Errorf("below threshold: Suggestion should be empty; got %q", tBelow.Metrics[0].Suggestion)
	}
	tAt := BuildAttributionTable(makeGames(MinSamplesForSuggestion), nil)
	if tAt.Metrics[0].Suggestion == "" {
		t.Errorf("at threshold: Suggestion should be populated; got empty")
	}
}

func TestSuggestionTextBuckets(t *testing.T) {
	// Bucket 1: high misweight (>0.2) → "misweighted by +"
	gamesHigh := []HeimdallDecisionAttribution{{
		GameID: "g", WinnerSeat: -1,
		Decisions: makeDecisions(20, "combo", DCCastVsHold, false, 0.5),
	}}
	tHigh := BuildAttributionTable(gamesHigh, nil)
	if !strings.Contains(tHigh.Metrics[0].Suggestion, "misweighted by avg +") {
		t.Errorf("high-misweight suggestion should contain 'misweighted by avg +'; got %q", tHigh.Metrics[0].Suggestion)
	}

	// Bucket 2: healthy (misweight < 0.05 AND winrate >= 0.5) → "healthy win rate"
	gamesHealthy := []HeimdallDecisionAttribution{{
		GameID: "g", WinnerSeat: 0,
		Decisions: makeDecisions(20, "combo", DCCastVsHold, true, 0.0),
	}}
	tHealthy := BuildAttributionTable(gamesHealthy, nil)
	if !strings.Contains(tHealthy.Metrics[0].Suggestion, "healthy win rate") {
		t.Errorf("healthy suggestion should contain 'healthy win rate'; got %q", tHealthy.Metrics[0].Suggestion)
	}

	// Bucket 3: in-between → "minor weight drift"
	// 10 won (with 0 delta → 0 misweight) + 10 lost (with 0.1 delta → 0.1 misweight)
	// = avg misweight 0.05, but we want a case that's NOT < 0.05 AND NOT > 0.2.
	// 10 won + 10 lost @ delta 0.2 → avg misweight 0.1, winrate 0.5. Between buckets.
	midDecs := make([]AttributedDecision, 0, 20)
	for i := 0; i < 10; i++ {
		midDecs = append(midDecs, AttributedDecision{
			HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold,
			Won: true, ScoreDelta: 0.2,
		})
	}
	for i := 0; i < 10; i++ {
		midDecs = append(midDecs, AttributedDecision{
			HatDecision: HatDecision{Archetype: "combo"}, Class: DCCastVsHold,
			Won: false, ScoreDelta: 0.2,
		})
	}
	gamesMid := []HeimdallDecisionAttribution{{GameID: "g", WinnerSeat: -1, Decisions: midDecs}}
	tMid := BuildAttributionTable(gamesMid, nil)
	if !strings.Contains(tMid.Metrics[0].Suggestion, "minor weight drift") {
		t.Errorf("in-between suggestion should contain 'minor weight drift'; got %q", tMid.Metrics[0].Suggestion)
	}
}

func TestPluggableEstimator_RespectedInAvgMisweight(t *testing.T) {
	constEst := func(d AttributedDecision) float64 { return 0.5 }
	games := []HeimdallDecisionAttribution{{
		GameID: "g", WinnerSeat: 0,
		Decisions: makeDecisions(4, "combo", DCCastVsHold, true, 0.0),
	}}
	table := BuildAttributionTable(games, constEst)
	if attrAbsFloat(table.Metrics[0].AvgMisweight-0.5) > 1e-9 {
		t.Errorf("AvgMisweight = %v, want 0.5 (from constant estimator)", table.Metrics[0].AvgMisweight)
	}
}

func TestAttributionTablePersist_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := GauntletAttributionTable{
		GamesProcessed: 3,
		TotalDecisions: 12,
		GeneratedAt:    "2026-05-31T00:00:00Z",
		Metrics: []ArchetypeClassMetrics{
			{Archetype: "combo", Class: DCCastVsHold, DecisionCount: 8, WonDecisionCount: 5,
				WinRate: 0.625, AvgScoreDelta: 0.15, AvgMisweight: 0.05, Suggestion: ""},
			{Archetype: "stax", Class: DCBlockChoice, DecisionCount: 4, WonDecisionCount: 2,
				WinRate: 0.5, AvgScoreDelta: 0.1, AvgMisweight: 0.1, Suggestion: ""},
		},
	}
	if err := WriteGauntletAttributionTable(dir, want); err != nil {
		t.Fatalf("WriteGauntletAttributionTable: %v", err)
	}
	// Verify the file landed under data/heimdall/ convention (the path
	// helper uses dir/heimdall/attribution_table.json).
	expectedPath := filepath.Join(dir, "heimdall", "attribution_table.json")
	got, err := ReadGauntletAttributionTable(dir)
	if err != nil {
		t.Fatalf("ReadGauntletAttributionTable: %v", err)
	}
	if got.GamesProcessed != want.GamesProcessed {
		t.Errorf("GamesProcessed = %d, want %d", got.GamesProcessed, want.GamesProcessed)
	}
	if got.TotalDecisions != want.TotalDecisions {
		t.Errorf("TotalDecisions = %d, want %d", got.TotalDecisions, want.TotalDecisions)
	}
	if len(got.Metrics) != len(want.Metrics) {
		t.Fatalf("Metrics len = %d, want %d", len(got.Metrics), len(want.Metrics))
	}
	for i := range want.Metrics {
		if got.Metrics[i] != want.Metrics[i] {
			t.Errorf("Metrics[%d] = %+v, want %+v", i, got.Metrics[i], want.Metrics[i])
		}
	}
	// Missing-file tolerance.
	empty, err := ReadGauntletAttributionTable(t.TempDir())
	if err != nil {
		t.Errorf("missing file should be tolerated; got err %v", err)
	}
	if empty.GamesProcessed != 0 || len(empty.Metrics) != 0 {
		t.Errorf("missing file should yield zero-value table; got %+v", empty)
	}
	_ = expectedPath
}

// mockFeedbackConsumer is the minimal type implementing FeedbackInterface
// for the TestFeedbackInterface_Stub callability check.
type mockFeedbackConsumer struct {
	accepted []GauntletAttributionTable
	err      error
}

func (m *mockFeedbackConsumer) AcceptAttributionTable(t GauntletAttributionTable) error {
	m.accepted = append(m.accepted, t)
	return m.err
}

func TestFeedbackInterface_Stub(t *testing.T) {
	// Pin that the interface contract is callable and Heimdall doesn't
	// reach for the return value side-effect — the recommender layer
	// (dev-3) owns whatever it wants to do with the table.
	var sink FeedbackInterface = &mockFeedbackConsumer{}
	table := GauntletAttributionTable{GamesProcessed: 1}
	if err := sink.AcceptAttributionTable(table); err != nil {
		t.Errorf("AcceptAttributionTable: %v", err)
	}
	mfc := sink.(*mockFeedbackConsumer)
	if len(mfc.accepted) != 1 {
		t.Errorf("mock should have received 1 table; got %d", len(mfc.accepted))
	}
}

// ---------- helpers ----------

func attrAbsFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func makeDecisions(n int, arch string, class DecisionClass, won bool, delta float64) []AttributedDecision {
	out := make([]AttributedDecision, n)
	for i := range out {
		out[i] = AttributedDecision{
			HatDecision: HatDecision{Archetype: arch},
			Class:       class,
			Won:         won,
			ScoreDelta:  delta,
		}
	}
	return out
}
