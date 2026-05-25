package heimdall

import (
	"testing"
)

// deck_version_trends_r60_test.go — pins ComputeDeckVersionTrends
// as a pure bucketing function over (lineage, games).

// -----------------------------------------------------------------------------
// Empty / degenerate.
// -----------------------------------------------------------------------------

func TestComputeDeckVersionTrends_EmptyEverything(t *testing.T) {
	got := ComputeDeckVersionTrends("alice", "iroh", nil, nil)
	if got.Owner != "alice" || got.DeckID != "iroh" {
		t.Errorf("owner/deck = %q/%q, want alice/iroh", got.Owner, got.DeckID)
	}
	if len(got.Versions) != 0 {
		t.Errorf("Versions should be empty, got %d", len(got.Versions))
	}
	if got.UntrackedGames != 0 {
		t.Errorf("UntrackedGames = %d, want 0", got.UntrackedGames)
	}
}

func TestComputeDeckVersionTrends_LineageWithoutGames(t *testing.T) {
	// Three versions but no games played yet — every row should
	// appear with Games=0 so the frontend can render "published but
	// unplayed" instead of dropping the row.
	lineage := []DeckVersionEntry{
		{Hash: "h1", Version: 1, CardCount: 100, CreatedAt: 1000},
		{Hash: "h2", Version: 2, CardCount: 100, CardDelta: 4, CreatedAt: 2000},
		{Hash: "h3", Version: 3, CardCount: 100, CardDelta: 2, CreatedAt: 3000},
	}
	got := ComputeDeckVersionTrends("alice", "iroh", lineage, nil)
	if len(got.Versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(got.Versions))
	}
	for _, v := range got.Versions {
		if v.Games != 0 || v.Wins != 0 || v.Losses != 0 || v.Draws != 0 {
			t.Errorf("version %d should have zero counts, got %+v", v.Version, v)
		}
	}
}

// -----------------------------------------------------------------------------
// Bucketing — games land in the version that was HEAD at finished_at.
// -----------------------------------------------------------------------------

func TestComputeDeckVersionTrends_GamesBucketByVersionEra(t *testing.T) {
	lineage := []DeckVersionEntry{
		{Hash: "v1", Version: 1, CreatedAt: 1000},
		{Hash: "v2", Version: 2, CardDelta: 5, CreatedAt: 2000},
		{Hash: "v3", Version: 3, CardDelta: 3, CreatedAt: 3000},
	}
	// v1 era (FinishedAt in [1000, 2000)): 1W 1L
	// v2 era ([2000, 3000)): 2W 1L 1D
	// v3 era (≥ 3000): 0W 3L
	games := []DeckVersionOutcome{
		{FinishedAt: 1100, Won: true},
		{FinishedAt: 1500, Won: false},
		{FinishedAt: 2100, Won: true},
		{FinishedAt: 2200, Won: false},
		{FinishedAt: 2300, Won: true},
		{FinishedAt: 2999, Draw: true},
		{FinishedAt: 3001, Won: false},
		{FinishedAt: 3200, Won: false},
		{FinishedAt: 3500, Won: false},
	}
	got := ComputeDeckVersionTrends("alice", "iroh", lineage, games)
	if len(got.Versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(got.Versions))
	}
	v1 := got.Versions[0]
	if v1.Games != 2 || v1.Wins != 1 || v1.Losses != 1 {
		t.Errorf("v1 = %+v, want games=2 wins=1 losses=1", v1)
	}
	if abs(v1.WinRate-0.5) > 1e-9 {
		t.Errorf("v1 WinRate = %f, want 0.5", v1.WinRate)
	}
	v2 := got.Versions[1]
	if v2.Games != 4 || v2.Wins != 2 || v2.Losses != 1 || v2.Draws != 1 {
		t.Errorf("v2 = %+v, want games=4 wins=2 losses=1 draws=1", v2)
	}
	v3 := got.Versions[2]
	if v3.Games != 3 || v3.Wins != 0 || v3.Losses != 3 {
		t.Errorf("v3 = %+v, want games=3 wins=0 losses=3", v3)
	}
}

func TestComputeDeckVersionTrends_GameAtExactBoundaryRoutesToNewVersion(t *testing.T) {
	// finished_at exactly equal to a version's CreatedAt should route
	// to that version (the upgrade applied at second t — games at t
	// played the new version).
	lineage := []DeckVersionEntry{
		{Hash: "v1", Version: 1, CreatedAt: 1000},
		{Hash: "v2", Version: 2, CreatedAt: 2000},
	}
	games := []DeckVersionOutcome{
		{FinishedAt: 2000, Won: true},
	}
	got := ComputeDeckVersionTrends("alice", "iroh", lineage, games)
	if got.Versions[0].Games != 0 {
		t.Errorf("v1 should have 0 games at boundary, got %d", got.Versions[0].Games)
	}
	if got.Versions[1].Games != 1 || got.Versions[1].Wins != 1 {
		t.Errorf("v2 should have 1 win at boundary, got %+v", got.Versions[1])
	}
}

func TestComputeDeckVersionTrends_GamesBeforeAnyVersionAreUntracked(t *testing.T) {
	lineage := []DeckVersionEntry{
		{Hash: "v1", Version: 1, CreatedAt: 5000},
	}
	games := []DeckVersionOutcome{
		{FinishedAt: 100, Won: true},   // way before
		{FinishedAt: 4999, Won: false}, // just before
		{FinishedAt: 5500, Won: true},  // in v1 era
	}
	got := ComputeDeckVersionTrends("alice", "iroh", lineage, games)
	if got.UntrackedGames != 2 {
		t.Errorf("UntrackedGames = %d, want 2", got.UntrackedGames)
	}
	if got.Versions[0].Games != 1 || got.Versions[0].Wins != 1 {
		t.Errorf("v1 = %+v, want games=1 wins=1", got.Versions[0])
	}
}

// -----------------------------------------------------------------------------
// Sort / output stability.
// -----------------------------------------------------------------------------

func TestComputeDeckVersionTrends_OutputSortedByVersionAsc(t *testing.T) {
	// Lineage given OUT of order — output must still be Version asc.
	lineage := []DeckVersionEntry{
		{Hash: "v3", Version: 3, CreatedAt: 3000},
		{Hash: "v1", Version: 1, CreatedAt: 1000},
		{Hash: "v2", Version: 2, CreatedAt: 2000},
	}
	got := ComputeDeckVersionTrends("alice", "iroh", lineage, nil)
	for i, v := range got.Versions {
		want := i + 1
		if v.Version != want {
			t.Errorf("Versions[%d].Version = %d, want %d", i, v.Version, want)
		}
	}
}

func TestComputeDeckVersionTrends_GameOrderIndependent(t *testing.T) {
	lineage := []DeckVersionEntry{
		{Hash: "v1", Version: 1, CreatedAt: 1000},
		{Hash: "v2", Version: 2, CreatedAt: 2000},
	}
	gamesForward := []DeckVersionOutcome{
		{FinishedAt: 1500, Won: true},
		{FinishedAt: 2500, Won: false},
	}
	gamesReverse := []DeckVersionOutcome{
		{FinishedAt: 2500, Won: false},
		{FinishedAt: 1500, Won: true},
	}
	a := ComputeDeckVersionTrends("alice", "iroh", lineage, gamesForward)
	b := ComputeDeckVersionTrends("alice", "iroh", lineage, gamesReverse)
	if a.Versions[0].Wins != b.Versions[0].Wins || a.Versions[1].Losses != b.Versions[1].Losses {
		t.Errorf("bucketing should be order-independent: %+v vs %+v", a, b)
	}
}

// -----------------------------------------------------------------------------
// CardDelta carries through to the response (frontend uses it for the
// "v2 = 5 cards swapped from v1" hover label).
// -----------------------------------------------------------------------------

func TestComputeDeckVersionTrends_CardDeltaPreserved(t *testing.T) {
	lineage := []DeckVersionEntry{
		{Hash: "v1", Version: 1, CreatedAt: 1000},
		{Hash: "v2", Version: 2, CardDelta: 7, CreatedAt: 2000},
	}
	got := ComputeDeckVersionTrends("alice", "iroh", lineage, nil)
	if got.Versions[1].CardDelta != 7 {
		t.Errorf("v2 CardDelta = %d, want 7", got.Versions[1].CardDelta)
	}
}
