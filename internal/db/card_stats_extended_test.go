package db

import (
	"context"
	"testing"
)

func TestLoadCardStatsByCommander_ReturnsPerCommanderBreakdown(t *testing.T) {
	dbPath := t.TempDir() + "/extended_test.db"
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	// Seed card_win_stats with some data.
	stats := []CardWinStat{
		{CardName: "Sol Ring", Commander: "Korvold", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Korvold", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Korvold", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 0, OnBoardAtWin: 0},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 0, OnBoardAtWin: 0},
	}
	for _, s := range stats {
		if err := BatchUpsertCardWinStats(ctx, d, []CardWinStat{s}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	// Query per-commander breakdown for Sol Ring.
	results, err := LoadCardStatsByCommander(ctx, d, "Sol Ring", 10)
	if err != nil {
		t.Fatalf("LoadCardStatsByCommander: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 commanders, got %d", len(results))
	}

	// Should be sorted by games desc — Muldrotha has 4 games, Korvold has 3.
	if results[0].Commander != "Muldrotha" {
		t.Errorf("expected Muldrotha first (4 games), got %s", results[0].Commander)
	}
	if results[0].Games != 4 {
		t.Errorf("Muldrotha games: got %d, want 4", results[0].Games)
	}
	if results[0].Wins != 2 {
		t.Errorf("Muldrotha wins: got %d, want 2", results[0].Wins)
	}
	if results[1].Commander != "Korvold" {
		t.Errorf("expected Korvold second, got %s", results[1].Commander)
	}
	if results[1].Games != 3 {
		t.Errorf("Korvold games: got %d, want 3", results[1].Games)
	}
}

func TestLoadCardStatsByCommander_NoDataReturnsEmpty(t *testing.T) {
	dbPath := t.TempDir() + "/extended_test.db"
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	results, err := LoadCardStatsByCommander(ctx, d, "Nonexistent Card", 10)
	if err != nil {
		t.Fatalf("LoadCardStatsByCommander: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d entries", len(results))
	}
}

func TestLoadCardInclusionRate(t *testing.T) {
	dbPath := t.TempDir() + "/inclusion_test.db"
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	// Seed ELO data: 3 decks with games > 0.
	eloRecords := []ELORecord{
		{DeckKey: "josh/korvold", Commander: "Korvold", Owner: "josh", Rating: 1500, Games: 10, Wins: 5, Losses: 5},
		{DeckKey: "josh/muldrotha", Commander: "Muldrotha", Owner: "josh", Rating: 1450, Games: 8, Wins: 3, Losses: 5},
		{DeckKey: "josh/breya", Commander: "Breya", Owner: "josh", Rating: 1400, Games: 12, Wins: 6, Losses: 6},
	}
	for _, r := range eloRecords {
		if err := UpsertELO(ctx, d, r); err != nil {
			t.Fatalf("upsert elo: %v", err)
		}
	}

	// Seed card_win_stats: Sol Ring appears in Korvold and Muldrotha.
	winStats := []CardWinStat{
		{CardName: "Sol Ring", Commander: "Korvold", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 1, OnBoardAtWin: 1},
	}
	if err := BatchUpsertCardWinStats(ctx, d, winStats); err != nil {
		t.Fatalf("upsert card win stats: %v", err)
	}

	decksUsing, totalDecks, err := LoadCardInclusionRate(ctx, d, "Sol Ring")
	if err != nil {
		t.Fatalf("LoadCardInclusionRate: %v", err)
	}
	if decksUsing != 2 {
		t.Errorf("decks_using: got %d, want 2", decksUsing)
	}
	if totalDecks != 3 {
		t.Errorf("total_decks: got %d, want 3", totalDecks)
	}
}

func TestLoadCardBracketDistribution(t *testing.T) {
	dbPath := t.TempDir() + "/bracket_test.db"
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	// Seed ELO with bracket info.
	eloRecords := []ELORecord{
		{DeckKey: "josh/korvold", Commander: "Korvold", Owner: "josh", Rating: 1500, Games: 10, Wins: 5, Losses: 5, Bracket: 4},
		{DeckKey: "josh/muldrotha", Commander: "Muldrotha", Owner: "josh", Rating: 1450, Games: 8, Wins: 3, Losses: 5, Bracket: 3},
		{DeckKey: "josh/breya", Commander: "Breya", Owner: "josh", Rating: 1400, Games: 12, Wins: 6, Losses: 6, Bracket: 3},
	}
	for _, r := range eloRecords {
		if err := UpsertELO(ctx, d, r); err != nil {
			t.Fatalf("upsert elo: %v", err)
		}
	}

	// Sol Ring used by Korvold (bracket 4) and Muldrotha (bracket 3).
	winStats := []CardWinStat{
		{CardName: "Sol Ring", Commander: "Korvold", Wins: 1, OnBoardAtWin: 1},
		{CardName: "Sol Ring", Commander: "Muldrotha", Wins: 1, OnBoardAtWin: 1},
	}
	if err := BatchUpsertCardWinStats(ctx, d, winStats); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	dist, err := LoadCardBracketDistribution(ctx, d, "Sol Ring")
	if err != nil {
		t.Fatalf("LoadCardBracketDistribution: %v", err)
	}
	if len(dist) != 2 {
		t.Fatalf("expected 2 bracket entries, got %d", len(dist))
	}
	// Should be sorted by bracket: 3 first, then 4.
	if dist[0].Bracket != 3 || dist[0].Count != 1 {
		t.Errorf("bracket 3: got %+v", dist[0])
	}
	if dist[1].Bracket != 4 || dist[1].Count != 1 {
		t.Errorf("bracket 4: got %+v", dist[1])
	}
}
