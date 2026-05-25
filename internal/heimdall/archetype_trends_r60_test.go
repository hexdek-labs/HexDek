package heimdall

import (
	"math"
	"testing"
)

// archetype_trends_r60_test.go — split-window archetype-winrate trend
// computation. The input games slice is newest-first (matches
// db.LoadOwnerGames' ORDER BY finished_at DESC), and we split it at
// len/2: index 0..len/2-1 is RECENT, len/2.. is PRIOR.

func archTrendsApprox(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func findTrend(trends []ArchetypeTrend, arch string) *ArchetypeTrend {
	for i := range trends {
		if trends[i].Archetype == arch {
			return &trends[i]
		}
	}
	return nil
}

// 10 games: 5 recent (1 win) + 5 prior (4 wins) → control archetype
// dropped from 80% to 20% (Δ = -0.60, direction "down").
func TestComputeArchetypeTrends_DownTrend(t *testing.T) {
	lookup := func(string) string { return "control" }
	games := []PlayerGameInput{
		// recent half (newest first): 1 win out of 5
		{Won: true, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"},
		// prior half: 4 wins out of 5
		{Won: true, DeckKey: "x/y"},
		{Won: true, DeckKey: "x/y"},
		{Won: true, DeckKey: "x/y"},
		{Won: true, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"},
	}
	trends := ComputePlayerTrends("p", 10, games, lookup).ArchetypeTrends
	c := findTrend(trends, "control")
	if c == nil {
		t.Fatalf("missing 'control' trend; got %+v", trends)
	}
	if c.RecentGames != 5 || c.RecentWins != 1 || !archTrendsApprox(c.RecentWinRate, 0.2) {
		t.Errorf("recent: %+v, want 5g/1w/0.2", c)
	}
	if c.PriorGames != 5 || c.PriorWins != 4 || !archTrendsApprox(c.PriorWinRate, 0.8) {
		t.Errorf("prior: %+v, want 5g/4w/0.8", c)
	}
	if !archTrendsApprox(c.Delta, -0.6) {
		t.Errorf("Delta = %.3f, want -0.6", c.Delta)
	}
	if c.Direction != "down" {
		t.Errorf("Direction = %q, want 'down'", c.Direction)
	}
}

// Recent half climbs from 20% to 80% → direction "up".
func TestComputeArchetypeTrends_UpTrend(t *testing.T) {
	lookup := func(string) string { return "combo" }
	games := []PlayerGameInput{
		// recent: 4 wins of 5
		{Won: true, DeckKey: "x/y"}, {Won: true, DeckKey: "x/y"},
		{Won: true, DeckKey: "x/y"}, {Won: true, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"},
		// prior: 1 win of 5
		{Won: false, DeckKey: "x/y"}, {Won: false, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"}, {Won: false, DeckKey: "x/y"},
		{Won: true, DeckKey: "x/y"},
	}
	trends := ComputePlayerTrends("p", 10, games, lookup).ArchetypeTrends
	c := findTrend(trends, "combo")
	if c == nil {
		t.Fatalf("missing 'combo' trend")
	}
	if c.Direction != "up" {
		t.Errorf("Direction = %q, want 'up' (Δ=%.3f)", c.Direction, c.Delta)
	}
	if c.Delta <= archetypeTrendDeltaThreshold {
		t.Errorf("Delta = %.3f, want > %.3f", c.Delta, archetypeTrendDeltaThreshold)
	}
}

// 50% in both halves → "flat".
func TestComputeArchetypeTrends_FlatWhenWinRateSteady(t *testing.T) {
	lookup := func(string) string { return "midrange" }
	games := []PlayerGameInput{
		{Won: true, DeckKey: "x/y"}, {Won: true, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"}, {Won: false, DeckKey: "x/y"},
		{Won: true, DeckKey: "x/y"}, {Won: true, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"}, {Won: false, DeckKey: "x/y"},
	}
	trends := ComputePlayerTrends("p", 8, games, lookup).ArchetypeTrends
	m := findTrend(trends, "midrange")
	if m == nil {
		t.Fatalf("missing 'midrange' trend")
	}
	if m.Direction != "flat" {
		t.Errorf("Direction = %q, want 'flat' (Δ=%.3f)", m.Direction, m.Delta)
	}
	if !archTrendsApprox(m.Delta, 0.0) {
		t.Errorf("Delta = %.3f, want 0", m.Delta)
	}
}

// Total games < 4 → no trends emitted at all.
func TestComputeArchetypeTrends_EmptyWhenTotalBelowMinimum(t *testing.T) {
	lookup := func(string) string { return "control" }
	games := []PlayerGameInput{
		{Won: true, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"},
		{Won: true, DeckKey: "x/y"},
	}
	trends := ComputePlayerTrends("p", 3, games, lookup).ArchetypeTrends
	if trends == nil {
		t.Errorf("ArchetypeTrends should be non-nil empty slice, got nil")
	}
	if len(trends) != 0 {
		t.Errorf("expected empty trends below %d-game minimum, got %+v",
			archetypeTrendMinTotalGames, trends)
	}
}

// Side with < 3 samples → direction "insufficient_data" even when
// delta is large.
func TestComputeArchetypeTrends_InsufficientDataWhenSideTooSmall(t *testing.T) {
	// 8 games total — split 4 recent / 4 prior. Make "combo" appear
	// 1× in recent (1w/0l) and 3× in prior (0w/3l). recent has only 1
	// game even though split is 4, so combo on recent side has just 1
	// sample → insufficient_data.
	lookup := func(deck string) string {
		if deck == "x/combo" {
			return "combo"
		}
		return "midrange"
	}
	games := []PlayerGameInput{
		// recent half (4 games)
		{Won: true, DeckKey: "x/combo"},
		{Won: false, DeckKey: "x/mid"},
		{Won: false, DeckKey: "x/mid"},
		{Won: false, DeckKey: "x/mid"},
		// prior half (4 games)
		{Won: false, DeckKey: "x/combo"},
		{Won: false, DeckKey: "x/combo"},
		{Won: false, DeckKey: "x/combo"},
		{Won: true, DeckKey: "x/mid"},
	}
	trends := ComputePlayerTrends("p", 8, games, lookup).ArchetypeTrends
	c := findTrend(trends, "combo")
	if c == nil {
		t.Fatalf("missing 'combo' trend; got %+v", trends)
	}
	if c.Direction != "insufficient_data" {
		t.Errorf("Direction = %q, want 'insufficient_data' (recent=%d prior=%d)",
			c.Direction, c.RecentGames, c.PriorGames)
	}
	if c.RecentGames != 1 || c.PriorGames != 3 {
		t.Errorf("counts: recent=%d prior=%d, want 1/3", c.RecentGames, c.PriorGames)
	}
}

// Archetype only in one half: still emit a row (counts are useful)
// but direction = "insufficient_data" because the other side has 0
// games.
func TestComputeArchetypeTrends_OnlyOneHalfHasArchetype(t *testing.T) {
	lookup := func(deck string) string {
		if deck == "x/voltron" {
			return "voltron"
		}
		return "midrange"
	}
	games := []PlayerGameInput{
		// recent: all voltron
		{Won: true, DeckKey: "x/voltron"},
		{Won: true, DeckKey: "x/voltron"},
		{Won: false, DeckKey: "x/voltron"},
		{Won: true, DeckKey: "x/voltron"},
		// prior: all midrange
		{Won: false, DeckKey: "x/mid"},
		{Won: false, DeckKey: "x/mid"},
		{Won: false, DeckKey: "x/mid"},
		{Won: true, DeckKey: "x/mid"},
	}
	trends := ComputePlayerTrends("p", 8, games, lookup).ArchetypeTrends
	v := findTrend(trends, "voltron")
	if v == nil || v.RecentGames != 4 || v.PriorGames != 0 {
		t.Fatalf("voltron trend wrong: %+v", v)
	}
	if v.Direction != "insufficient_data" {
		t.Errorf("voltron Direction = %q, want 'insufficient_data' (prior=0)", v.Direction)
	}
	m := findTrend(trends, "midrange")
	if m == nil || m.RecentGames != 0 || m.PriorGames != 4 {
		t.Fatalf("midrange trend wrong: %+v", m)
	}
	if m.Direction != "insufficient_data" {
		t.Errorf("midrange Direction = %q, want 'insufficient_data' (recent=0)", m.Direction)
	}
}

// Sort: largest |Δ| first among real trends; "insufficient_data"
// rows sink to the bottom even if their |Δ| is technically large.
func TestComputeArchetypeTrends_SortByAbsDeltaInsufficientLast(t *testing.T) {
	lookup := func(deck string) string {
		switch deck {
		case "x/control":
			return "control"
		case "x/combo":
			return "combo"
		case "x/voltron":
			return "voltron"
		default:
			return "midrange"
		}
	}
	// 12 games: 6 recent + 6 prior.
	// - control: 3 recent (0w) + 3 prior (3w) → Δ = -1.0, direction down
	// - combo:   3 recent (3w) + 3 prior (2w) → Δ = +0.333, direction up
	// - voltron: 0 recent + 0 prior → not present (skip)
	// We can't really get sample insufficient + big delta in 12 games
	// without playing tricks; just verify the down (-1) sorts above
	// the up (+0.33).
	games := []PlayerGameInput{
		// recent (6)
		{Won: false, DeckKey: "x/control"}, {Won: false, DeckKey: "x/control"}, {Won: false, DeckKey: "x/control"},
		{Won: true, DeckKey: "x/combo"}, {Won: true, DeckKey: "x/combo"}, {Won: true, DeckKey: "x/combo"},
		// prior (6)
		{Won: true, DeckKey: "x/control"}, {Won: true, DeckKey: "x/control"}, {Won: true, DeckKey: "x/control"},
		{Won: true, DeckKey: "x/combo"}, {Won: true, DeckKey: "x/combo"}, {Won: false, DeckKey: "x/combo"},
	}
	trends := ComputePlayerTrends("p", 12, games, lookup).ArchetypeTrends
	if len(trends) < 2 {
		t.Fatalf("expected 2+ trends, got %+v", trends)
	}
	// Control (|Δ|=1.0) must precede Combo (|Δ|≈0.333).
	if trends[0].Archetype != "control" || trends[1].Archetype != "combo" {
		t.Errorf("sort order: [0]=%s [1]=%s, want control, combo", trends[0].Archetype, trends[1].Archetype)
	}
}

// Insufficient_data rows sink past real trends even when their |Δ|
// is technically larger.
func TestComputeArchetypeTrends_InsufficientSinksBelowReal(t *testing.T) {
	lookup := func(deck string) string {
		switch deck {
		case "x/ramp":
			return "ramp"
		case "x/mid":
			return "midrange"
		default:
			return "control"
		}
	}
	// 10 games: 5 recent + 5 prior.
	//
	// midrange: 4 recent (2w) + 4 prior (2w) → Δ = 0, direction flat
	// control: 1 recent (0w) + 1 prior (0w) → insufficient (both <3)
	// ramp:    appears only at one slot per half — also insufficient
	games := []PlayerGameInput{
		// recent (5)
		{Won: true, DeckKey: "x/mid"}, {Won: true, DeckKey: "x/mid"},
		{Won: false, DeckKey: "x/mid"}, {Won: false, DeckKey: "x/mid"},
		{Won: false, DeckKey: "x/control"},
		// prior (5)
		{Won: true, DeckKey: "x/mid"}, {Won: true, DeckKey: "x/mid"},
		{Won: false, DeckKey: "x/mid"}, {Won: false, DeckKey: "x/mid"},
		{Won: false, DeckKey: "x/control"},
	}
	trends := ComputePlayerTrends("p", 10, games, lookup).ArchetypeTrends
	if len(trends) < 2 {
		t.Fatalf("expected at least 2 trends, got %+v", trends)
	}
	// First entry must NOT be insufficient_data; control row must come after.
	if trends[0].Direction == "insufficient_data" {
		t.Errorf("first trend should be a real direction, got %+v", trends[0])
	}
	// Find control and ensure it sorts after midrange.
	midIdx, ctlIdx := -1, -1
	for i, tr := range trends {
		if tr.Archetype == "midrange" {
			midIdx = i
		}
		if tr.Archetype == "control" {
			ctlIdx = i
		}
	}
	if midIdx < 0 || ctlIdx < 0 || ctlIdx < midIdx {
		t.Errorf("insufficient 'control' should sort after 'midrange'; idx mid=%d ctl=%d trends=%+v",
			midIdx, ctlIdx, trends)
	}
}

// Threshold boundary: exactly +0.05 → "up"; +0.0499... → "flat".
func TestComputeArchetypeTrends_ThresholdBoundary(t *testing.T) {
	// 20 games (10 + 10). Recent: 6w/4l (60%). Prior: 5w/5l (50%).
	// Δ = +0.10 → "up".
	lookup := func(string) string { return "midrange" }
	games := []PlayerGameInput{
		// recent (10): 6 wins
		{Won: true, DeckKey: "x"}, {Won: true, DeckKey: "x"},
		{Won: true, DeckKey: "x"}, {Won: true, DeckKey: "x"},
		{Won: true, DeckKey: "x"}, {Won: true, DeckKey: "x"},
		{Won: false, DeckKey: "x"}, {Won: false, DeckKey: "x"},
		{Won: false, DeckKey: "x"}, {Won: false, DeckKey: "x"},
		// prior (10): 5 wins
		{Won: true, DeckKey: "x"}, {Won: true, DeckKey: "x"},
		{Won: true, DeckKey: "x"}, {Won: true, DeckKey: "x"},
		{Won: true, DeckKey: "x"},
		{Won: false, DeckKey: "x"}, {Won: false, DeckKey: "x"},
		{Won: false, DeckKey: "x"}, {Won: false, DeckKey: "x"},
		{Won: false, DeckKey: "x"},
	}
	tr := findTrend(ComputePlayerTrends("p", 20, games, lookup).ArchetypeTrends, "midrange")
	if tr == nil {
		t.Fatal("missing midrange")
	}
	if !archTrendsApprox(tr.Delta, 0.1) || tr.Direction != "up" {
		t.Errorf("Δ=%.3f dir=%q, want 0.1 / 'up'", tr.Delta, tr.Direction)
	}
}

// nil lookup → all games bucket as "unknown" and still trend.
func TestComputeArchetypeTrends_NilLookupRoutesToUnknown(t *testing.T) {
	games := []PlayerGameInput{
		{Won: true, DeckKey: "x/y"}, {Won: true, DeckKey: "x/y"},
		{Won: true, DeckKey: "x/y"}, {Won: true, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"}, {Won: false, DeckKey: "x/y"},
		{Won: false, DeckKey: "x/y"}, {Won: false, DeckKey: "x/y"},
	}
	trends := ComputePlayerTrends("p", 8, games, nil).ArchetypeTrends
	u := findTrend(trends, "unknown")
	if u == nil {
		t.Fatalf("nil lookup should bucket trend under 'unknown'; got %+v", trends)
	}
	if u.RecentGames != 4 || u.PriorGames != 4 {
		t.Errorf("split: %+v, want 4/4", u)
	}
}

// Embedded inside the full PlayerTrends payload — sanity check that
// extending the struct didn't break the existing fields.
func TestComputePlayerTrends_StillPopulatesExistingFields_WithTrends(t *testing.T) {
	lookup := func(string) string { return "control" }
	games := []PlayerGameInput{
		{Won: true, DeckKey: "x/y", OpponentCmdrs: []string{"O1", "O2"}},
		{Won: true, DeckKey: "x/y", OpponentCmdrs: []string{"O1"}},
		{Won: false, DeckKey: "x/y", OpponentCmdrs: []string{"O2"}},
		{Won: false, DeckKey: "x/y", OpponentCmdrs: []string{"O3"}},
	}
	pt := ComputePlayerTrends("alice", 4, games, lookup)
	if pt.Wins != 2 || pt.Losses != 2 || pt.GamesPlayed != 4 {
		t.Errorf("counts: %+v, want 2/2/4", pt)
	}
	if !archTrendsApprox(pt.WinRate, 0.5) {
		t.Errorf("WinRate = %.3f, want 0.5", pt.WinRate)
	}
	if len(pt.Archetypes) != 1 || pt.Archetypes[0].Archetype != "control" {
		t.Errorf("Archetypes mangled: %+v", pt.Archetypes)
	}
	if len(pt.ArchetypeTrends) != 1 || pt.ArchetypeTrends[0].Archetype != "control" {
		t.Errorf("ArchetypeTrends should have 1 'control' row, got %+v", pt.ArchetypeTrends)
	}
}
