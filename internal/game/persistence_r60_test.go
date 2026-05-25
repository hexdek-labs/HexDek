package game

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// persistence_r60_test.go — wires the game-table persistence flow
// end-to-end and verifies the R60 seat-bias columns (winner_seat,
// turns, end_reason) populate correctly. Pre-R60 internal/game/ had
// zero tests; the seat-bias measurement (PR #258) discovered the
// games table was empty in production but couldn't tell if that was
// because no one had played or because the persistence chain was
// broken. These tests pin the broken-vs-empty distinction.

// freshDB opens a fresh SQLite DB at a temp path, runs migrations,
// and returns it for the test to use. Required because Open() runs
// applyMigrations which adds the R60 columns.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// seedPartyAndDevices creates the minimal db scaffolding a game needs:
// devices, decks, and a party with each device joined. Returns the
// party ID + device IDs (in seat order) + deck IDs.
func seedPartyAndDevices(t *testing.T, database *sql.DB, nSeats int) (string, []string, []string) {
	t.Helper()
	ctx := context.Background()

	deviceIDs := make([]string, nSeats)
	deckIDs := make([]string, nSeats)
	for i := 0; i < nSeats; i++ {
		dev, err := db.CreateDevice(ctx, database, "seat"+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("create device %d: %v", i, err)
		}
		deviceIDs[i] = dev.ID
		deck := &db.Deck{
			OwnerDeviceID: dev.ID,
			Name:          "TestDeck-" + string(rune('A'+i)),
			Format:        "commander",
			RawJSON:       `{"mainboard":[],"commander":"Test Commander"}`,
		}
		if err := db.CreateDeck(ctx, database, deck); err != nil {
			t.Fatalf("create deck %d: %v", i, err)
		}
		deckIDs[i] = deck.ID
	}

	party, err := db.CreateParty(ctx, database, deviceIDs[0], nSeats)
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	// CreateParty implicitly joins the host (seat 0) — only join the
	// remaining devices to avoid the "already in party" constraint.
	for i := 1; i < nSeats; i++ {
		if _, err := db.JoinParty(ctx, database, party.ID, deviceIDs[i], deckIDs[i], false); err != nil {
			t.Fatalf("join party seat %d: %v", i, err)
		}
	}
	return party.ID, deviceIDs, deckIDs
}

// -----------------------------------------------------------------------------
// CreateGame + GetGame round-trip
// -----------------------------------------------------------------------------

func TestCreateGame_PersistsBasicFields(t *testing.T) {
	database := freshDB(t)
	ctx := context.Background()
	partyID, _, _ := seedPartyAndDevices(t, database, 4)

	g, err := CreateGame(ctx, database, partyID, "seed-hash-abc")
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	if g.ID == "" {
		t.Fatal("CreateGame returned empty ID")
	}
	if g.PartyID != partyID {
		t.Errorf("PartyID = %q, want %q", g.PartyID, partyID)
	}
	if g.StartedAt == 0 {
		t.Error("StartedAt not set")
	}
	if g.FinishedAt != 0 {
		t.Error("FinishedAt should be 0 on freshly created game")
	}
	if g.WinnerSeat != nil {
		t.Errorf("WinnerSeat should be nil on fresh game, got %v", *g.WinnerSeat)
	}
}

func TestGetGame_RoundTripsCreatedFields(t *testing.T) {
	database := freshDB(t)
	ctx := context.Background()
	partyID, _, _ := seedPartyAndDevices(t, database, 4)

	g, err := CreateGame(ctx, database, partyID, "seed-hash")
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	got, err := GetGame(ctx, database, g.ID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if got.ID != g.ID || got.PartyID != partyID {
		t.Errorf("round-trip mismatch: got %+v want partyID=%s", got, partyID)
	}
}

// -----------------------------------------------------------------------------
// FinishGame writes the new R60 columns
// -----------------------------------------------------------------------------

func TestFinishGame_PopulatesWinnerSeatTurnsAndEndReason(t *testing.T) {
	database := freshDB(t)
	ctx := context.Background()
	partyID, deviceIDs, _ := seedPartyAndDevices(t, database, 4)

	g, err := CreateGame(ctx, database, partyID, "seed-hash")
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	seat := 2
	if err := FinishGame(ctx, database, g.ID, deviceIDs[2], &seat, 37, "last_seat_standing"); err != nil {
		t.Fatalf("FinishGame: %v", err)
	}

	got, err := GetGame(ctx, database, g.ID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if got.FinishedAt == 0 {
		t.Error("FinishedAt not set after FinishGame")
	}
	if got.Winner != deviceIDs[2] {
		t.Errorf("Winner = %q, want %q", got.Winner, deviceIDs[2])
	}
	if got.WinnerSeat == nil || *got.WinnerSeat != 2 {
		t.Errorf("WinnerSeat = %v, want pointer to 2", got.WinnerSeat)
	}
	if got.Turns != 37 {
		t.Errorf("Turns = %d, want 37", got.Turns)
	}
	if got.EndReason != "last_seat_standing" {
		t.Errorf("EndReason = %q, want %q", got.EndReason, "last_seat_standing")
	}
}

func TestFinishGame_DrawHasNilSeatAndDrawReason(t *testing.T) {
	database := freshDB(t)
	ctx := context.Background()
	partyID, _, _ := seedPartyAndDevices(t, database, 4)

	g, err := CreateGame(ctx, database, partyID, "seed-hash")
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	if err := FinishGame(ctx, database, g.ID, "", nil, 80, "draw"); err != nil {
		t.Fatalf("FinishGame draw: %v", err)
	}

	got, err := GetGame(ctx, database, g.ID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if got.WinnerSeat != nil {
		t.Errorf("draw game should have nil WinnerSeat; got %v", *got.WinnerSeat)
	}
	if got.Winner != "" {
		t.Errorf("draw game should have empty Winner; got %q", got.Winner)
	}
	if got.EndReason != "draw" {
		t.Errorf("draw game EndReason = %q, want %q", got.EndReason, "draw")
	}
	if got.Turns != 80 {
		t.Errorf("Turns = %d, want 80", got.Turns)
	}
}

// -----------------------------------------------------------------------------
// WinsBySeat aggregation — the SQL the seat-bias pipeline runs
// -----------------------------------------------------------------------------

func TestWinsBySeat_EmptyDBReturnsNothing(t *testing.T) {
	database := freshDB(t)
	ctx := context.Background()
	rows, err := WinsBySeat(ctx, database)
	if err != nil {
		t.Fatalf("WinsBySeat: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("empty db should return 0 rows, got %d", len(rows))
	}
}

func TestWinsBySeat_AggregatesByWinnerSeat(t *testing.T) {
	database := freshDB(t)
	ctx := context.Background()
	partyID, deviceIDs, _ := seedPartyAndDevices(t, database, 4)

	// 5 games: winners at seats 0, 0, 2, 3, 3
	wins := []int{0, 0, 2, 3, 3}
	for _, seat := range wins {
		g, err := CreateGame(ctx, database, partyID, "h")
		if err != nil {
			t.Fatalf("CreateGame: %v", err)
		}
		s := seat
		if err := FinishGame(ctx, database, g.ID, deviceIDs[seat], &s, 30, "last_seat_standing"); err != nil {
			t.Fatalf("FinishGame: %v", err)
		}
	}

	rows, err := WinsBySeat(ctx, database)
	if err != nil {
		t.Fatalf("WinsBySeat: %v", err)
	}
	got := map[int]int{}
	for _, r := range rows {
		got[r.Seat] = r.Wins
	}
	want := map[int]int{0: 2, 2: 1, 3: 2}
	for seat, n := range want {
		if got[seat] != n {
			t.Errorf("seat %d wins = %d, want %d", seat, got[seat], n)
		}
	}
	// Seat 1 should NOT appear (no wins)
	if _, ok := got[1]; ok {
		t.Errorf("seat 1 should not appear in results (no wins), got %d", got[1])
	}
}

func TestWinsBySeat_ExcludesUnfinishedAndDrawGames(t *testing.T) {
	database := freshDB(t)
	ctx := context.Background()
	partyID, deviceIDs, _ := seedPartyAndDevices(t, database, 4)

	// Unfinished game — no FinishGame call.
	if _, err := CreateGame(ctx, database, partyID, "h-unfinished"); err != nil {
		t.Fatalf("CreateGame unfinished: %v", err)
	}
	// Draw game — FinishGame with nil seat.
	g2, _ := CreateGame(ctx, database, partyID, "h-draw")
	if err := FinishGame(ctx, database, g2.ID, "", nil, 80, "draw"); err != nil {
		t.Fatalf("FinishGame draw: %v", err)
	}
	// Real winner at seat 1.
	g3, _ := CreateGame(ctx, database, partyID, "h-win")
	s := 1
	if err := FinishGame(ctx, database, g3.ID, deviceIDs[1], &s, 25, "last_seat_standing"); err != nil {
		t.Fatalf("FinishGame win: %v", err)
	}

	rows, err := WinsBySeat(ctx, database)
	if err != nil {
		t.Fatalf("WinsBySeat: %v", err)
	}
	if len(rows) != 1 || rows[0].Seat != 1 || rows[0].Wins != 1 {
		t.Errorf("expected seats with wins to be [{1,1}], got %+v", rows)
	}
}
