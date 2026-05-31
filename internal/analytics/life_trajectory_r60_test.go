package analytics

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// life_trajectory_r60_test.go — pins the PlayerAnalysis.LifeByTurn
// snapshot + the per-game life-trajectory report section. Both feed a
// dashboard line-chart surface (raw slice = y-axis series, index =
// x-axis turn).

// mkLifeEvent constructs a canonical life_change event. Amount is the
// delta (positive = gain, negative = loss).
func mkLifeEvent(seat, delta int) gameengine.Event {
	return gameengine.Event{
		Kind:   "life_change",
		Seat:   seat,
		Amount: delta,
	}
}

// mkTurnStart builds a turn_start event whose Details carry the turn
// number. The analyzer reads `Details["turn"]` to advance currentTurn.
func mkTurnStart(turn int) gameengine.Event {
	return gameengine.Event{
		Kind:    "turn_start",
		Details: map[string]interface{}{"turn": turn},
	}
}

// TestLifeTrajectory_StartingFortyAndDecline pins the simplest happy
// path: seat starts at 40, takes 5 damage on turn 1, 10 on turn 2, 3
// on turn 3 — trajectory reads [40, 35, 25, 22] (index 0 = start,
// index N = end of turn N).
func TestLifeTrajectory_StartingFortyAndDecline(t *testing.T) {
	events := []gameengine.Event{
		mkTurnStart(1),
		mkLifeEvent(0, -5), // 40 → 35 on turn 1
		mkTurnStart(2),
		mkLifeEvent(0, -10), // 35 → 25 on turn 2
		mkTurnStart(3),
		mkLifeEvent(0, -3), // 25 → 22 on turn 3
	}
	ga := AnalyzeGame(events, 1, []string{"Cmdr"}, -1, 3, []int{0}, []int{22})
	pa := &ga.Players[0]
	got := pa.LifeByTurn
	want := []int{40, 35, 25, 22}
	if len(got) != len(want) {
		t.Fatalf("LifeByTurn len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("LifeByTurn[%d] = %d, want %d (full series %v)", i, got[i], v, got)
		}
	}
}

// TestLifeTrajectory_FlatTurnsBackfilled pins the gap-filling
// behavior: a seat that takes no damage on turn 2 still has a turn-2
// sample equal to their turn-1 life. Dashboards rendering the series
// as a line chart need this so a flat span renders as a horizontal
// segment, not a missing-data discontinuity.
func TestLifeTrajectory_FlatTurnsBackfilled(t *testing.T) {
	events := []gameengine.Event{
		mkTurnStart(1),
		mkLifeEvent(0, -10), // 40 → 30 on turn 1
		mkTurnStart(2),
		// No life change this turn.
		mkTurnStart(3),
		mkLifeEvent(0, -5), // 30 → 25 on turn 3
	}
	ga := AnalyzeGame(events, 1, []string{"Cmdr"}, -1, 3, []int{0}, []int{25})
	pa := &ga.Players[0]
	want := []int{40, 30, 30, 25}
	if len(pa.LifeByTurn) != len(want) {
		t.Fatalf("LifeByTurn len = %d, want %d (got %v)", len(pa.LifeByTurn), len(want), pa.LifeByTurn)
	}
	for i, v := range want {
		if pa.LifeByTurn[i] != v {
			t.Errorf("LifeByTurn[%d] = %d, want %d (full series %v)", i, pa.LifeByTurn[i], v, pa.LifeByTurn)
		}
	}
}

// TestLifeTrajectory_FinalSnapshotAfterLastTurnStart pins that the
// trailing turn is sampled by the post-loop final-snap. Without this
// the last turn's life would be missing — only turn_start of the NEXT
// turn writes the prior turn's sample, and the next turn never starts
// when the game ends.
func TestLifeTrajectory_FinalSnapshotAfterLastTurnStart(t *testing.T) {
	events := []gameengine.Event{
		mkTurnStart(1),
		mkLifeEvent(0, -20), // 40 → 20 on turn 1
		mkTurnStart(2),
		mkLifeEvent(0, -20), // 20 → 0 on turn 2 (game ends)
	}
	ga := AnalyzeGame(events, 1, []string{"Cmdr"}, -1, 2, []int{0}, []int{0})
	pa := &ga.Players[0]
	want := []int{40, 20, 0}
	if len(pa.LifeByTurn) != len(want) {
		t.Fatalf("LifeByTurn len = %d, want %d (got %v)", len(pa.LifeByTurn), len(want), pa.LifeByTurn)
	}
	for i, v := range want {
		if pa.LifeByTurn[i] != v {
			t.Errorf("LifeByTurn[%d] = %d, want %d", i, pa.LifeByTurn[i], v)
		}
	}
}

// TestLifeTrajectory_LifeGains pins that positive life_change deltas
// also drive trajectory updates (Soul Warden, Aetherflux Reservoir,
// gain-life triggers). Pre-fix the analyzer only tracked PeakLife on
// gain; trajectory needs both directions.
func TestLifeTrajectory_LifeGains(t *testing.T) {
	events := []gameengine.Event{
		mkTurnStart(1),
		mkLifeEvent(0, +10), // 40 → 50 on turn 1
		mkTurnStart(2),
		mkLifeEvent(0, -8), // 50 → 42 on turn 2
	}
	ga := AnalyzeGame(events, 1, []string{"Cmdr"}, -1, 2, []int{0}, []int{42})
	pa := &ga.Players[0]
	want := []int{40, 50, 42}
	for i, v := range want {
		if i >= len(pa.LifeByTurn) || pa.LifeByTurn[i] != v {
			t.Errorf("LifeByTurn[%d] = %v, want %d (full %v)",
				i, pa.LifeByTurn[i:i+1], v, pa.LifeByTurn)
		}
	}
	if pa.PeakLife < 50 {
		t.Errorf("PeakLife = %d, want >= 50 (gain tracked)", pa.PeakLife)
	}
}

// TestLifeTrajectory_MultiSeatIndependent pins that each seat tracks
// its own series independently. Three seats taking different damage
// patterns should produce three distinct LifeByTurn slices, not a
// shared backing array or cross-talk.
func TestLifeTrajectory_MultiSeatIndependent(t *testing.T) {
	events := []gameengine.Event{
		mkTurnStart(1),
		mkLifeEvent(0, -5),
		mkLifeEvent(1, -10),
		mkLifeEvent(2, -15),
		mkTurnStart(2),
		mkLifeEvent(0, -1),
		mkLifeEvent(2, -3),
	}
	ga := AnalyzeGame(events, 3, []string{"A", "B", "C"}, -1, 2, []int{0, 0, 0}, []int{34, 30, 22})
	want := [][]int{
		{40, 35, 34},
		{40, 30, 30},
		{40, 25, 22},
	}
	for s := 0; s < 3; s++ {
		got := ga.Players[s].LifeByTurn
		if len(got) != len(want[s]) {
			t.Errorf("seat %d len = %d, want %d (got %v)", s, len(got), len(want[s]), got)
			continue
		}
		for i, v := range want[s] {
			if got[i] != v {
				t.Errorf("seat %d LifeByTurn[%d] = %d, want %d", s, i, got[i], v)
			}
		}
	}
}

// TestWriteLifeTrajectories_RenderShape pins the markdown table
// surface: per-game heading, T0..TN header, per-seat rows including
// (W) tag for the winner.
func TestWriteLifeTrajectories_RenderShape(t *testing.T) {
	events := []gameengine.Event{
		mkTurnStart(1),
		mkLifeEvent(0, -5),
		mkLifeEvent(1, -10),
		mkTurnStart(2),
		mkLifeEvent(1, -30), // seat 1 to 0; would lose
	}
	ga := AnalyzeGame(events, 2, []string{"Winner Cmdr", "Loser Cmdr"}, 0, 2, []int{0, 0}, []int{35, 0})
	r := &AnalyticsReport{
		Analyses:       []*GameAnalysis{ga},
		CommanderNames: []string{"Winner Cmdr", "Loser Cmdr"},
		TotalGames:     1,
	}
	var b strings.Builder
	r.writeLifeTrajectories(&b)
	out := b.String()

	if !strings.Contains(out, "## Life Trajectory") {
		t.Errorf("missing section header; got:\n%s", out)
	}
	if !strings.Contains(out, "### Game 1") {
		t.Errorf("missing per-game header; got:\n%s", out)
	}
	for _, col := range []string{"| T0 |", "| T1 |", "| T2 |"} {
		if !strings.Contains(out, col) {
			t.Errorf("missing column %q in:\n%s", col, out)
		}
	}
	if !strings.Contains(out, "Winner Cmdr (W)") {
		t.Errorf("winner should be flagged (W); got:\n%s", out)
	}
	if !strings.Contains(out, "Loser Cmdr |") {
		t.Errorf("loser row missing; got:\n%s", out)
	}
	// Starting life cell.
	if !strings.Contains(out, " 40 |") {
		t.Errorf("missing T0=40 starting-life cell; got:\n%s", out)
	}
}

// TestWriteLifeTrajectories_CapsAtFiveGames pins the markdown
// render-cap: with 7 analyzed games only the first 5 are tabled, and
// a footnote reports +2 more games available via the structured
// slice.
func TestWriteLifeTrajectories_CapsAtFiveGames(t *testing.T) {
	mk := func() *GameAnalysis {
		return AnalyzeGame(
			[]gameengine.Event{mkTurnStart(1), mkLifeEvent(0, -5)},
			1, []string{"Cmdr"}, -1, 1, []int{0}, []int{35})
	}
	r := &AnalyticsReport{
		Analyses:       []*GameAnalysis{mk(), mk(), mk(), mk(), mk(), mk(), mk()},
		CommanderNames: []string{"Cmdr"},
		TotalGames:     7,
	}
	var b strings.Builder
	r.writeLifeTrajectories(&b)
	out := b.String()

	if strings.Contains(out, "### Game 6") {
		t.Errorf("Game 6 should not render (above cap); got:\n%s", out)
	}
	if !strings.Contains(out, "### Game 5") {
		t.Errorf("Game 5 should render (at cap); got:\n%s", out)
	}
	if !strings.Contains(out, "+2 more games") {
		t.Errorf("missing tail footnote; got:\n%s", out)
	}
}

// TestWriteLifeTrajectories_EmptyAnalyses pins the no-data guard.
func TestWriteLifeTrajectories_EmptyAnalyses(t *testing.T) {
	var b strings.Builder
	(&AnalyticsReport{}).writeLifeTrajectories(&b)
	out := b.String()
	if !strings.Contains(out, "## Life Trajectory") {
		t.Errorf("missing header even when empty; got:\n%s", out)
	}
	if !strings.Contains(out, "No game data") {
		t.Errorf("expected empty-state message; got:\n%s", out)
	}
}
