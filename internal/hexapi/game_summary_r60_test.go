package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

func newSummaryTestHandler(t *testing.T) (*Handler, http.Handler) {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/summary.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	h := &Handler{}
	h.SetDB(d)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux
}

func seedSummaryGame(t *testing.T, d *Handler, winner, turns int, endReason string) int64 {
	t.Helper()
	ctx := context.Background()
	gid, err := db.PersistGameTx(ctx, d.db, db.GameRecord{
		StartedAt:  100,
		FinishedAt: 200,
		Turns:      turns,
		Winner:     winner,
		WinnerName: "Krenko, Mob Boss",
		EndReason:  endReason,
		Seed:       4242,
	}, []db.GameSeatRecord{
		{Seat: 0, Commander: "Krenko, Mob Boss", DeckKey: "alice/krenko"},
		{Seat: 1, Commander: "Atraxa, Praetors' Voice", DeckKey: "bob/atraxa"},
	})
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	return gid
}

func TestHandleGameSummary_ReturnsDBOnlySummaryForPersistedGame(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 17, "combat")

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
	if s.GameID != intToStr(gid) {
		t.Errorf("GameID: want %s, got %s", intToStr(gid), s.GameID)
	}
	if s.Winner != 0 || s.Turns != 17 || s.EndReason != "combat" {
		t.Errorf("metadata mismatch: %+v", s)
	}
	if s.WinnerName != "Krenko, Mob Boss" {
		t.Errorf("WinnerName: want Krenko..., got %q", s.WinnerName)
	}
	if s.DataSource != "db_only" {
		t.Errorf("DataSource: want db_only, got %q", s.DataSource)
	}
	// Empty signal arrays should be non-nil so frontend code can iterate.
	if s.CommanderZoneVisits == nil || s.RegretCards == nil || s.MVPCards == nil {
		t.Errorf("R60 signal arrays should be non-nil empty slices; got %+v", s)
	}
	if len(s.TurningPoints) != 1 || s.TurningPoints[0].Kind != "game_end" {
		t.Errorf("expected single game_end turning point; got %+v", s.TurningPoints)
	}
	if s.TurningPoints[0].Turn != 17 || s.TurningPoints[0].Seat != 0 {
		t.Errorf("game_end turning point should match game metadata; got %+v", s.TurningPoints[0])
	}
}

func TestHandleGameSummary_404OnUnknownID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)

	req := httptest.NewRequest("GET", "/api/games/999999/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleGameSummary_400OnInvalidID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)

	for _, bad := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest("GET", "/api/games/"+bad+"/summary", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("id=%q: want 400, got %d", bad, rec.Code)
		}
	}
}

func TestHandleGameSummary_503WhenDBMissing(t *testing.T) {
	h := &Handler{} // no DB attached
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/games/1/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleGameSummary_DrawGameSurfacesMinusOneWinner(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, -1, 80, "timeout")

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
	if s.Winner != -1 || s.EndReason != "timeout" {
		t.Errorf("draw metadata: %+v", s)
	}
	if len(s.TurningPoints) != 1 ||
		s.TurningPoints[0].Seat != -1 {
		t.Errorf("draw turning point should carry seat=-1; got %+v", s.TurningPoints)
	}
}

// Rich path: when a snapshot is persisted for the game, the handler
// returns the rich GameSummary (data_source = "rich") with all
// three R60 signal arrays populated and turning points derived from
// CardFirstPlayed.
func TestHandleGameSummary_RichPathWhenSnapshotPersisted(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 12, "combat")

	snap := heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{
			RNGSeed: 4242,
			Winner:  1,
			Turns:   12,
		},
		CommanderZoneVisits: []heimdall.CommanderZoneVisit{
			{Seat: 1, CommanderName: "Atraxa, Praetors' Voice", CastCount: 1, InZoneAtEnd: true, Visits: 2},
		},
		RegretCards: []heimdall.RegretCard{
			{Seat: 0, CardName: "Plague Wind", CMC: 9, Reason: "stranded_in_hand"},
		},
		MVPCards: []heimdall.MVPCard{
			{Seat: 1, CardName: "Atraxa, Praetors' Voice", CMC: 4, Score: 8, IsCommander: true},
		},
		CardFirstPlayed:      map[string]int{"Atraxa, Praetors' Voice": 5},
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
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var s heimdall.GameSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.DataSource != "rich" {
		t.Errorf("DataSource: want rich, got %q", s.DataSource)
	}
	if len(s.CommanderZoneVisits) != 1 || len(s.RegretCards) != 1 || len(s.MVPCards) != 1 {
		t.Errorf("rich signals not surfaced: %+v", s)
	}
	if s.RegretCards[0].CardName != "Plague Wind" {
		t.Errorf("regret card mismatch: %+v", s.RegretCards[0])
	}
	if s.MVPCards[0].CardName != "Atraxa, Praetors' Voice" || !s.MVPCards[0].IsCommander {
		t.Errorf("MVP card mismatch: %+v", s.MVPCards[0])
	}
	// 2 turning points: commander_first_cast on turn 5, game_end on turn 12.
	if len(s.TurningPoints) != 2 {
		t.Fatalf("want 2 turning points, got %d", len(s.TurningPoints))
	}
	if s.TurningPoints[0].Kind != "commander_first_cast" || s.TurningPoints[0].Turn != 5 {
		t.Errorf("position 0: %+v", s.TurningPoints[0])
	}
	if s.TurningPoints[1].Kind != "game_end" || s.TurningPoints[1].Turn != 12 {
		t.Errorf("position 1: %+v", s.TurningPoints[1])
	}
	// GameRecord metadata (WinnerName, end_reason) wins over snapshot
	// values since the GameRecord is the authoritative end-state.
	if s.WinnerName != "Krenko, Mob Boss" || s.EndReason != "combat" {
		t.Errorf("GameRecord metadata overlay failed: %+v", s)
	}
}

// Malformed snapshot in DB: handler logs and falls back to db_only
// rather than 500-ing.
func TestHandleGameSummary_MalformedSnapshotFallsBackToDBOnly(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 7, "commander")

	if err := db.InsertGameObservation(context.Background(), h.db, gid, "{{ not json"); err != nil {
		t.Fatalf("insert malformed snapshot: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200 (graceful fallback), got %d (%s)", rec.Code, rec.Body.String())
	}
	var s heimdall.GameSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.DataSource != "db_only" {
		t.Errorf("DataSource: want db_only fallback on malformed snapshot, got %q", s.DataSource)
	}
}

// Rich-path winner conflict: snapshot says winner=2, GameRecord says
// winner=1 — GameRecord wins.
func TestHandleGameSummary_GameRecordWinsOverSnapshotForOutcome(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 14, "combat")

	snap := heimdall.GameObservationSnapshot{
		Seed:                 heimdall.GameSeed{RNGSeed: 4242, Winner: 2, Turns: 10},
		CommanderNamesBySeat: [][]string{nil, {"Atraxa"}, nil, nil},
		FinalTurn:            10,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var s heimdall.GameSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Winner != 1 {
		t.Errorf("GameRecord winner should override snapshot; got winner=%d", s.Winner)
	}
	if s.Turns != 14 {
		t.Errorf("GameRecord turns (14) should win over snapshot turns (10); got %d", s.Turns)
	}
}

// buildDBOnlyGameSummary is exercised through the handler above; this
// unit-test pin lets it fail fast if the helper drifts.
func TestBuildDBOnlyGameSummary_FieldsAndNonNilSlices(t *testing.T) {
	g := db.GameRecord{
		GameID:     42,
		Turns:      11,
		Winner:     2,
		WinnerName: "Edgar Markov",
		EndReason:  "combat",
	}
	s := buildDBOnlyGameSummary(g)
	if s.GameID != "42" || s.Winner != 2 || s.Turns != 11 || s.EndReason != "combat" {
		t.Errorf("metadata mismatch: %+v", s)
	}
	if s.DataSource != "db_only" {
		t.Errorf("DataSource: want db_only, got %q", s.DataSource)
	}
	if s.CommanderZoneVisits == nil || s.RegretCards == nil || s.MVPCards == nil {
		t.Errorf("signal arrays should be initialized to empty (non-nil) slices")
	}
}

func intToStr(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var out []byte
	for n > 0 {
		out = append([]byte{digits[n%10]}, out...)
		n /= 10
	}
	if negative {
		out = append([]byte{'-'}, out...)
	}
	return string(out)
}
