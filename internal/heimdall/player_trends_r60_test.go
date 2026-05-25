package heimdall

import (
	"testing"
)

// player_trends_r60_test.go — pins ComputePlayerTrends as a pure
// function over a slice of PlayerGameInput, independent of any DB.

// -----------------------------------------------------------------------------
// Empty / degenerate input.
// -----------------------------------------------------------------------------

func TestComputePlayerTrends_EmptyGames(t *testing.T) {
	got := ComputePlayerTrends("alice", 30, nil, nil)
	if got.PlayerID != "alice" {
		t.Errorf("PlayerID = %q, want alice", got.PlayerID)
	}
	if got.WindowSize != 30 {
		t.Errorf("WindowSize = %d, want 30", got.WindowSize)
	}
	if got.GamesPlayed != 0 {
		t.Errorf("GamesPlayed = %d, want 0", got.GamesPlayed)
	}
	if got.WinRate != 0 {
		t.Errorf("WinRate = %f, want 0", got.WinRate)
	}
	if got.Archetypes == nil {
		t.Errorf("Archetypes should be non-nil even for empty input (frontend renders as [])")
	}
}

// -----------------------------------------------------------------------------
// Winrate.
// -----------------------------------------------------------------------------

func TestComputePlayerTrends_WinrateCounts(t *testing.T) {
	games := []PlayerGameInput{
		{Won: true, DeckKey: "alice/voltron"},
		{Won: true, DeckKey: "alice/voltron"},
		{Won: false, DeckKey: "alice/voltron"},
		{Won: false, DeckKey: "alice/voltron"},
		{Draw: true, DeckKey: "alice/voltron"},
	}
	got := ComputePlayerTrends("alice", 30, games, nil)
	if got.Wins != 2 || got.Losses != 2 || got.Draws != 1 {
		t.Errorf("Wins/Losses/Draws = %d/%d/%d, want 2/2/1", got.Wins, got.Losses, got.Draws)
	}
	if got.GamesPlayed != 5 {
		t.Errorf("GamesPlayed = %d, want 5", got.GamesPlayed)
	}
	wantWinRate := 2.0 / 5.0
	if abs(got.WinRate-wantWinRate) > 1e-9 {
		t.Errorf("WinRate = %f, want %f", got.WinRate, wantWinRate)
	}
}

// -----------------------------------------------------------------------------
// Archetype distribution.
// -----------------------------------------------------------------------------

func TestComputePlayerTrends_ArchetypeBreakdownAndWinRates(t *testing.T) {
	lookup := map[string]string{
		"alice/voltron":  "voltron",
		"alice/combo":    "combo",
		"alice/midrange": "midrange",
	}
	games := []PlayerGameInput{
		{Won: true, DeckKey: "alice/voltron"},
		{Won: true, DeckKey: "alice/voltron"},
		{Won: false, DeckKey: "alice/voltron"},
		{Won: true, DeckKey: "alice/combo"},
		{Won: false, DeckKey: "alice/combo"},
		{Won: false, DeckKey: "alice/midrange"},
	}
	got := ComputePlayerTrends("alice", 30, games, func(dk string) string {
		return lookup[dk]
	})
	if len(got.Archetypes) != 3 {
		t.Fatalf("expected 3 archetypes, got %d (%+v)", len(got.Archetypes), got.Archetypes)
	}
	// Sorted by games desc: voltron(3), combo(2), midrange(1).
	if got.Archetypes[0].Archetype != "voltron" || got.Archetypes[0].Games != 3 || got.Archetypes[0].Wins != 2 {
		t.Errorf("first slot = %+v, want voltron 3/2", got.Archetypes[0])
	}
	if got.Archetypes[1].Archetype != "combo" || got.Archetypes[1].Games != 2 || got.Archetypes[1].Wins != 1 {
		t.Errorf("second slot = %+v, want combo 2/1", got.Archetypes[1])
	}
	if got.Archetypes[2].Archetype != "midrange" || got.Archetypes[2].Games != 1 || got.Archetypes[2].Wins != 0 {
		t.Errorf("third slot = %+v, want midrange 1/0", got.Archetypes[2])
	}
	if abs(got.Archetypes[0].WinRate-2.0/3.0) > 1e-9 {
		t.Errorf("voltron WinRate = %f, want 2/3", got.Archetypes[0].WinRate)
	}
}

func TestComputePlayerTrends_MissingArchetypeBucketedAsUnknown(t *testing.T) {
	// Three decks, but the lookup only knows two.
	lookup := map[string]string{
		"alice/voltron": "voltron",
	}
	games := []PlayerGameInput{
		{Won: true, DeckKey: "alice/voltron"},
		{Won: false, DeckKey: "alice/freshly-imported"}, // no strategy.json yet
		{Won: false, DeckKey: ""},                       // missing deck mapping
	}
	got := ComputePlayerTrends("alice", 30, games, func(dk string) string {
		return lookup[dk]
	})
	gotUnknown := 0
	for _, a := range got.Archetypes {
		if a.Archetype == "unknown" {
			gotUnknown = a.Games
		}
	}
	if gotUnknown != 2 {
		t.Errorf("expected 2 games in 'unknown' bucket, got %d (%+v)", gotUnknown, got.Archetypes)
	}
}

func TestComputePlayerTrends_NilLookupRoutesEverythingToUnknown(t *testing.T) {
	games := []PlayerGameInput{
		{Won: true, DeckKey: "alice/voltron"},
		{Won: false, DeckKey: "alice/combo"},
	}
	got := ComputePlayerTrends("alice", 30, games, nil)
	if len(got.Archetypes) != 1 {
		t.Fatalf("expected 1 archetype (unknown), got %d", len(got.Archetypes))
	}
	if got.Archetypes[0].Archetype != "unknown" || got.Archetypes[0].Games != 2 {
		t.Errorf("expected unknown/2 games, got %+v", got.Archetypes[0])
	}
}

// -----------------------------------------------------------------------------
// Opponent diversity.
// -----------------------------------------------------------------------------

func TestComputePlayerTrends_OpponentDiversity(t *testing.T) {
	games := []PlayerGameInput{
		{Won: true, OpponentCmdrs: []string{"Atraxa", "Edgar Markov", "Krenko"}},
		{Won: false, OpponentCmdrs: []string{"Atraxa", "Yuriko", "Krenko"}},
		{Won: false, OpponentCmdrs: []string{"Atraxa", "Korvold", "Tergrid"}},
	}
	got := ComputePlayerTrends("alice", 30, games, nil)
	if got.OpponentDiversity.TotalEncounters != 9 {
		t.Errorf("TotalEncounters = %d, want 9", got.OpponentDiversity.TotalEncounters)
	}
	// Unique: Atraxa, Edgar, Krenko, Yuriko, Korvold, Tergrid = 6.
	if got.OpponentDiversity.UniqueCommanders != 6 {
		t.Errorf("UniqueCommanders = %d, want 6", got.OpponentDiversity.UniqueCommanders)
	}
	wantRatio := 6.0 / 9.0
	if abs(got.OpponentDiversity.DiversityRatio-wantRatio) > 1e-9 {
		t.Errorf("DiversityRatio = %f, want %f", got.OpponentDiversity.DiversityRatio, wantRatio)
	}
	// Repeats: Atraxa (3), Krenko (2). Sorted by encounters desc.
	repeats := got.OpponentDiversity.TopRepeatOpponents
	if len(repeats) != 2 {
		t.Fatalf("expected 2 repeat opponents, got %d (%+v)", len(repeats), repeats)
	}
	if repeats[0].Commander != "Atraxa" || repeats[0].Encounters != 3 {
		t.Errorf("first repeat = %+v, want Atraxa/3", repeats[0])
	}
	if repeats[1].Commander != "Krenko" || repeats[1].Encounters != 2 {
		t.Errorf("second repeat = %+v, want Krenko/2", repeats[1])
	}
}

func TestComputePlayerTrends_TopRepeatCappedAtFive(t *testing.T) {
	// Seven commanders that each appear in 2+ games — the response
	// should cap at 5.
	g := func(opps ...string) PlayerGameInput {
		return PlayerGameInput{Won: false, OpponentCmdrs: opps}
	}
	games := []PlayerGameInput{
		g("A", "B", "C"),
		g("A", "B", "C"),
		g("D", "E", "F"),
		g("D", "E", "F"),
		g("G", "G", "G"), // odd but legal: same opp at multiple seats? we tolerate; counts as 3 encounters
	}
	got := ComputePlayerTrends("alice", 30, games, nil)
	if len(got.OpponentDiversity.TopRepeatOpponents) != 5 {
		t.Errorf("TopRepeatOpponents should be capped at 5, got %d", len(got.OpponentDiversity.TopRepeatOpponents))
	}
}

func TestComputePlayerTrends_EmptyOpponentNameSkipped(t *testing.T) {
	games := []PlayerGameInput{
		{Won: true, OpponentCmdrs: []string{"Atraxa", "", "Krenko"}},
	}
	got := ComputePlayerTrends("alice", 30, games, nil)
	if got.OpponentDiversity.TotalEncounters != 2 {
		t.Errorf("TotalEncounters = %d, want 2 (empty name skipped)", got.OpponentDiversity.TotalEncounters)
	}
}

// -----------------------------------------------------------------------------
// Window vs games-played asymmetry.
// -----------------------------------------------------------------------------

func TestComputePlayerTrends_GamesPlayedReflectsActualInput(t *testing.T) {
	// Caller requested window=30 but only 12 games on record.
	games := make([]PlayerGameInput, 12)
	for i := range games {
		games[i] = PlayerGameInput{Won: i%2 == 0}
	}
	got := ComputePlayerTrends("alice", 30, games, nil)
	if got.WindowSize != 30 {
		t.Errorf("WindowSize should echo request, got %d", got.WindowSize)
	}
	if got.GamesPlayed != 12 {
		t.Errorf("GamesPlayed should reflect actual input, got %d", got.GamesPlayed)
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
