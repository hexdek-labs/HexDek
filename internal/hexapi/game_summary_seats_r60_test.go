package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// Pins the R60 addition of GameSummary.Seats — the per-seat end-of-game
// scoreboard loaded from showmatch_game_seat. Available on both rich
// and db_only paths; omitted (empty) when no seat rows are persisted.

// seedSummaryGameWithScoreboard mirrors seedSummaryGame but stamps
// life / hand / library / graveyard / battlefield sizes + the lost flag
// so the seat-loader has a non-trivial state to project.
func seedSummaryGameWithScoreboard(t *testing.T, h *Handler, winner, turns int, endReason string) int64 {
	t.Helper()
	ctx := context.Background()
	gid, err := db.PersistGameTx(ctx, h.db, db.GameRecord{
		StartedAt:  100,
		FinishedAt: 200,
		Turns:      turns,
		Winner:     winner,
		WinnerName: "Krenko, Mob Boss",
		EndReason:  endReason,
		Seed:       4242,
	}, []db.GameSeatRecord{
		{Seat: 0, Commander: "Krenko, Mob Boss", DeckKey: "alice/krenko",
			Life: 27, HandSize: 4, LibrarySize: 62, GYSize: 5, BFSize: 8, Lost: false},
		{Seat: 1, Commander: "Atraxa, Praetors' Voice", DeckKey: "bob/atraxa",
			Life: 0, HandSize: 6, LibrarySize: 70, GYSize: 11, BFSize: 4, Lost: true},
	})
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	return gid
}

// TestHandleGameSummary_DBOnlyPathIncludesSeatScoreboard: the db_only
// path stamps Seats from showmatch_game_seat onto the response.
func TestHandleGameSummary_DBOnlyPathIncludesSeatScoreboard(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGameWithScoreboard(t, h, 0, 17, "combat")

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var s heimdall.GameSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.DataSource != "db_only" {
		t.Fatalf("expected db_only path, got %q", s.DataSource)
	}
	if len(s.Seats) != 2 {
		t.Fatalf("want 2 seats, got %d (%+v)", len(s.Seats), s.Seats)
	}
	// Sort is ORDER BY seat in LoadGameSeats, so index == seat number.
	want0 := heimdall.SeatFinalState{
		Seat: 0, Commander: "Krenko, Mob Boss",
		Life: 27, HandSize: 4, LibrarySize: 62, GraveyardSize: 5, BattlefieldSize: 8, Lost: false,
	}
	want1 := heimdall.SeatFinalState{
		Seat: 1, Commander: "Atraxa, Praetors' Voice",
		Life: 0, HandSize: 6, LibrarySize: 70, GraveyardSize: 11, BattlefieldSize: 4, Lost: true,
	}
	if s.Seats[0] != want0 {
		t.Errorf("seat 0: got %+v, want %+v", s.Seats[0], want0)
	}
	if s.Seats[1] != want1 {
		t.Errorf("seat 1: got %+v, want %+v", s.Seats[1], want1)
	}
}

// TestHandleGameSummary_RichPathAlsoIncludesSeatScoreboard: when a
// snapshot is present, the rich path still loads + stamps the seat
// scoreboard from DB. Snapshot doesn't carry per-seat life totals, so
// DB is the only source — and the contract should be the same shape.
func TestHandleGameSummary_RichPathAlsoIncludesSeatScoreboard(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGameWithScoreboard(t, h, 1, 12, "combat")

	snap := heimdall.GameObservationSnapshot{
		Seed:                 heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 12},
		CommanderNamesBySeat: [][]string{nil, {"Atraxa, Praetors' Voice"}, nil, nil},
		FinalTurn:            12,
	}
	payload, err := heimdall.MarshalSnapshot(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var s heimdall.GameSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.DataSource != "rich" {
		t.Fatalf("expected rich path, got %q", s.DataSource)
	}
	if len(s.Seats) != 2 {
		t.Fatalf("rich path should still surface seats: got %d (%+v)", len(s.Seats), s.Seats)
	}
	if s.Seats[1].Lost != true || s.Seats[1].Life != 0 {
		t.Errorf("rich path seat 1: want Lost=true Life=0, got %+v", s.Seats[1])
	}
}

// TestHandleGameSummary_SeatsOmittedWhenNoSeatRows: a game with no
// persisted seat rows (very old games or partial-write scenarios)
// should produce a response with the Seats field absent — `omitempty`
// drops the empty slice from the JSON entirely so clients can rely on
// `seats` being present-iff-populated.
func TestHandleGameSummary_SeatsOmittedWhenNoSeatRows(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	ctx := context.Background()
	gid, err := db.PersistGameTx(ctx, h.db, db.GameRecord{
		StartedAt:  100,
		FinishedAt: 200,
		Turns:      5,
		Winner:     0,
		WinnerName: "Krenko, Mob Boss",
		EndReason:  "combat",
		Seed:       9999,
	}, nil) // <-- no seat rows
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, `"seats"`) {
		t.Errorf("seats field should be omitted when no seat rows; body=%s", body)
	}
	var s heimdall.GameSummary
	if err := json.Unmarshal([]byte(body), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.Seats) != 0 {
		t.Errorf("Seats should be empty when no rows; got %+v", s.Seats)
	}
}

// TestHandleGameSummary_LosingSeatLifeZeroFlagged: defensive pin — a
// seat with life=0 and lost=true should round-trip without the JSON
// coalescing either to a default/zero placeholder.
func TestHandleGameSummary_LosingSeatLifeZeroFlagged(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGameWithScoreboard(t, h, 0, 17, "combat")

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	// JSON should explicitly include lost: true for seat 1 — guards
	// against omitempty being applied to the boolean field by mistake.
	// writeJSON uses indented output, so the colon has a trailing space.
	if !strings.Contains(body, `"lost": true`) {
		t.Errorf("losing-seat lost flag missing from JSON: body=%s", body)
	}
	_ = gid
}
