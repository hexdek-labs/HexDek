package heimdall

import (
	"math"
	"reflect"
	"testing"
)

func cmpApprox(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func findSharedArch(rows []SharedArchetype, arch string) *SharedArchetype {
	for i := range rows {
		if rows[i].Archetype == arch {
			return &rows[i]
		}
	}
	return nil
}

func findSharedOpp(rows []SharedOpponent, cmdr string) *SharedOpponent {
	for i := range rows {
		if rows[i].Commander == cmdr {
			return &rows[i]
		}
	}
	return nil
}

func mkTrends(player string, games, wins, losses int, archs []ArchetypeBreakdown, opps []RepeatOpponent) PlayerTrends {
	pt := PlayerTrends{
		PlayerID:    player,
		WindowSize:  games,
		GamesPlayed: games,
		Wins:        wins,
		Losses:      losses,
		Archetypes:  archs,
		OpponentDiversity: OpponentDiversity{
			TopRepeatOpponents: opps,
		},
	}
	if games > 0 {
		pt.WinRate = float64(wins) / float64(games)
	}
	return pt
}

// TestComparePlayers_WinRateAndGamesDelta pins the scalar diffs.
func TestComparePlayers_WinRateAndGamesDelta(t *testing.T) {
	a := mkTrends("alice", 100, 60, 40, nil, nil)
	b := mkTrends("bob", 80, 32, 48, nil, nil)
	out := ComparePlayers(PlayerComparisonInput{PlayerA: a, PlayerB: b})

	if !cmpApprox(out.WinRateDelta, 0.6-0.4) {
		t.Errorf("WinRateDelta = %.3f, want 0.2", out.WinRateDelta)
	}
	if out.GamesPlayedDelta != 20 {
		t.Errorf("GamesPlayedDelta = %d, want 20", out.GamesPlayedDelta)
	}
}

// TestComparePlayers_SharedArchetypes pins the intersection logic
// + per-side games/winrate + WinRateDelta.
func TestComparePlayers_SharedArchetypes(t *testing.T) {
	a := mkTrends("alice", 50, 25, 25, []ArchetypeBreakdown{
		{Archetype: "control", Games: 20, Wins: 10, WinRate: 0.5},
		{Archetype: "voltron", Games: 30, Wins: 15, WinRate: 0.5},
	}, nil)
	b := mkTrends("bob", 40, 28, 12, []ArchetypeBreakdown{
		{Archetype: "control", Games: 10, Wins: 8, WinRate: 0.8},
		{Archetype: "combo", Games: 30, Wins: 20, WinRate: 0.667},
	}, nil)
	out := ComparePlayers(PlayerComparisonInput{PlayerA: a, PlayerB: b})

	// Only "control" is shared.
	if len(out.SharedArchetypes) != 1 {
		t.Fatalf("expected 1 shared archetype, got %+v", out.SharedArchetypes)
	}
	sa := out.SharedArchetypes[0]
	if sa.Archetype != "control" || sa.PlayerAGames != 20 || sa.PlayerBGames != 10 {
		t.Errorf("control row: %+v", sa)
	}
	if !cmpApprox(sa.WinRateDelta, -0.3) {
		t.Errorf("WinRateDelta = %.3f, want -0.3 (A 0.5 - B 0.8)", sa.WinRateDelta)
	}
}

// TestComparePlayers_SharedArchetypesSortByCombinedGames pins the
// sort: most-played-overall archetypes first.
func TestComparePlayers_SharedArchetypesSortByCombinedGames(t *testing.T) {
	a := mkTrends("alice", 100, 50, 50, []ArchetypeBreakdown{
		{Archetype: "aaa", Games: 5},
		{Archetype: "bbb", Games: 40},
		{Archetype: "ccc", Games: 20},
	}, nil)
	b := mkTrends("bob", 100, 50, 50, []ArchetypeBreakdown{
		{Archetype: "aaa", Games: 5},
		{Archetype: "bbb", Games: 10},
		{Archetype: "ccc", Games: 35},
	}, nil)
	out := ComparePlayers(PlayerComparisonInput{PlayerA: a, PlayerB: b})

	// Combined: bbb=50, ccc=55, aaa=10 → order ccc, bbb, aaa
	wantOrder := []string{"ccc", "bbb", "aaa"}
	for i, want := range wantOrder {
		if out.SharedArchetypes[i].Archetype != want {
			t.Errorf("SharedArchetypes[%d] = %s, want %s", i, out.SharedArchetypes[i].Archetype, want)
		}
	}
}

// TestComparePlayers_SharedArchetypesEmptyWhenDisjoint pins the
// non-overlap case — empty (non-nil) slice.
func TestComparePlayers_SharedArchetypesEmptyWhenDisjoint(t *testing.T) {
	a := mkTrends("alice", 10, 5, 5, []ArchetypeBreakdown{{Archetype: "voltron", Games: 10}}, nil)
	b := mkTrends("bob", 10, 5, 5, []ArchetypeBreakdown{{Archetype: "combo", Games: 10}}, nil)
	out := ComparePlayers(PlayerComparisonInput{PlayerA: a, PlayerB: b})
	if out.SharedArchetypes == nil {
		t.Errorf("should be non-nil empty slice")
	}
	if len(out.SharedArchetypes) != 0 {
		t.Errorf("got %+v, want empty", out.SharedArchetypes)
	}
}

// TestComparePlayers_SharedOpponents pins the opponent-commander
// intersection + per-side encounter counts.
func TestComparePlayers_SharedOpponents(t *testing.T) {
	a := mkTrends("alice", 50, 25, 25, nil, []RepeatOpponent{
		{Commander: "Atraxa", Encounters: 5},
		{Commander: "Krenko", Encounters: 3},
	})
	b := mkTrends("bob", 50, 25, 25, nil, []RepeatOpponent{
		{Commander: "Atraxa", Encounters: 7},
		{Commander: "Edgar", Encounters: 4},
	})
	out := ComparePlayers(PlayerComparisonInput{PlayerA: a, PlayerB: b})

	if len(out.SharedOpponents) != 1 {
		t.Fatalf("expected 1 shared opponent, got %+v", out.SharedOpponents)
	}
	if out.SharedOpponents[0].Commander != "Atraxa" {
		t.Errorf("opp commander: %s, want Atraxa", out.SharedOpponents[0].Commander)
	}
	if out.SharedOpponents[0].PlayerAEncounters != 5 || out.SharedOpponents[0].PlayerBEncounters != 7 {
		t.Errorf("encounter counts: %+v", out.SharedOpponents[0])
	}
}

// TestComparePlayers_HeadToHeadAggregation pins the per-side win
// attribution from raw head-to-head game records.
func TestComparePlayers_HeadToHeadAggregation(t *testing.T) {
	games := []HeadToHeadGameRecord{
		{Winner: 0, SeatA: 0, SeatB: 1}, // A wins
		{Winner: 1, SeatA: 0, SeatB: 1}, // B wins
		{Winner: 2, SeatA: 0, SeatB: 1}, // neither
		{Winner: -1, SeatA: 0, SeatB: 1}, // draw
		{Winner: 0, SeatA: 0, SeatB: 1}, // A wins
	}
	out := ComparePlayers(PlayerComparisonInput{
		PlayerA: mkTrends("a", 0, 0, 0, nil, nil),
		PlayerB: mkTrends("b", 0, 0, 0, nil, nil),
		HeadToHead: games,
	})
	want := PlayerHeadToHead{GamesTogether: 5, PlayerAWins: 2, PlayerBWins: 1, Draws: 1}
	if !reflect.DeepEqual(out.HeadToHead, want) {
		t.Errorf("HeadToHead = %+v, want %+v", out.HeadToHead, want)
	}
}

// TestComparePlayers_EmbedsBothSummaries pins that the full
// PlayerTrends blobs are echoed through.
func TestComparePlayers_EmbedsBothSummaries(t *testing.T) {
	a := mkTrends("alice", 50, 25, 25, nil, nil)
	b := mkTrends("bob", 60, 30, 30, nil, nil)
	out := ComparePlayers(PlayerComparisonInput{PlayerA: a, PlayerB: b})
	if out.PlayerA.PlayerID != "alice" || out.PlayerB.PlayerID != "bob" {
		t.Errorf("ids: a=%s b=%s", out.PlayerA.PlayerID, out.PlayerB.PlayerID)
	}
	if out.PlayerA.GamesPlayed != 50 || out.PlayerB.GamesPlayed != 60 {
		t.Errorf("games: a=%d b=%d", out.PlayerA.GamesPlayed, out.PlayerB.GamesPlayed)
	}
}

// -----------------------------------------------------------------------------
// BuildRatingVelocityFromHalves tests
// -----------------------------------------------------------------------------

func TestBuildRatingVelocityFromHalves_Up(t *testing.T) {
	// Recent 10w/0l (100%) vs overall 50w/50l (50%) → Velocity +0.5 → up
	rv := BuildRatingVelocityFromHalves(10, 10, 100, 50)
	if rv.Direction != "up" {
		t.Errorf("Direction = %q, want 'up' (V=%.3f)", rv.Direction, rv.Velocity)
	}
	if !cmpApprox(rv.Velocity, 0.5) {
		t.Errorf("Velocity = %.3f, want 0.5", rv.Velocity)
	}
}

func TestBuildRatingVelocityFromHalves_Down(t *testing.T) {
	rv := BuildRatingVelocityFromHalves(10, 0, 100, 50)
	if rv.Direction != "down" {
		t.Errorf("Direction = %q, want 'down' (V=%.3f)", rv.Direction, rv.Velocity)
	}
}

func TestBuildRatingVelocityFromHalves_Flat(t *testing.T) {
	rv := BuildRatingVelocityFromHalves(10, 5, 100, 50)
	if rv.Direction != "flat" {
		t.Errorf("Direction = %q, want 'flat' (V=%.3f)", rv.Direction, rv.Velocity)
	}
}

func TestBuildRatingVelocityFromHalves_InsufficientWhenRecentTooSmall(t *testing.T) {
	// recent=4 (< min 5) — should be insufficient regardless of swing.
	rv := BuildRatingVelocityFromHalves(4, 4, 100, 50)
	if rv.Direction != "insufficient_data" {
		t.Errorf("Direction = %q, want 'insufficient_data'", rv.Direction)
	}
}

func TestBuildRatingVelocityFromHalves_InsufficientWhenOverallTooSmall(t *testing.T) {
	// overall=9 (< min 10).
	rv := BuildRatingVelocityFromHalves(5, 4, 9, 4)
	if rv.Direction != "insufficient_data" {
		t.Errorf("Direction = %q, want 'insufficient_data'", rv.Direction)
	}
}

func TestBuildRatingVelocityFromHalves_PopulatesRates(t *testing.T) {
	rv := BuildRatingVelocityFromHalves(10, 7, 100, 60)
	if !cmpApprox(rv.RecentWinRate, 0.7) {
		t.Errorf("RecentWinRate = %.3f, want 0.7", rv.RecentWinRate)
	}
	if !cmpApprox(rv.OverallWinRate, 0.6) {
		t.Errorf("OverallWinRate = %.3f, want 0.6", rv.OverallWinRate)
	}
}
