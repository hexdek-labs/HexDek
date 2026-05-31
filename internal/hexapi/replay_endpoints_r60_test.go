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

// replay_endpoints_r60_test.go — covers the three new structured
// replay endpoints. Reuses the seedSummaryGame fixture but builds
// its own snapshot with a richer payload (regret cards, mulligan
// stats, zone visits) so the forensics endpoint has real signals to
// surface.

// seedReplayEndpointsFixture plants a game + a snapshot exercising
// every analysis array the forensics endpoint surfaces. Turns 1, 3,
// 5, 7, 9 are populated so per-turn projection tests have boundaries
// to assert against.
func seedReplayEndpointsFixture(t *testing.T, h *Handler) int64 {
	t.Helper()
	gid := seedSummaryGame(t, h, 1, 9, "combat")
	snap := heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{
			RNGSeed: 4242,
			Winner:  1,
			Turns:   9,
		},
		CardFirstPlayed: map[string]int{
			"Sol Ring":                1,
			"Sylvan Library":          3,
			"Atraxa, Praetors' Voice": 5,
			"Cyclonic Rift":           7,
		},
		MVPCards: []heimdall.MVPCard{
			{Seat: 0, CardName: "Sol Ring", CMC: 1, TurnPlayed: 1, Score: 6, IsCommander: false},
			{Seat: 1, CardName: "Atraxa, Praetors' Voice", CMC: 4, TurnPlayed: 5, Score: 9, IsCommander: true},
			{Seat: 1, CardName: "Cyclonic Rift", CMC: 7, TurnPlayed: 7, Score: 11, IsCommander: false},
		},
		RegretCards: []heimdall.RegretCard{
			{Seat: 0, CardName: "Counterspell", Reason: "held a counter through a game-ending threat"},
		},
		MulliganStats: []heimdall.MulliganStat{
			{Seat: 0, MulligansTaken: 1, OpeningHandSize: 6, Lands: 3, ActionCards: 3, Keepable: true},
			{Seat: 1, MulligansTaken: 0, OpeningHandSize: 7, Lands: 3, ActionCards: 4, Keepable: true},
		},
		CommanderZoneVisits: []heimdall.CommanderZoneVisit{
			{Seat: 1, CommanderName: "Atraxa, Praetors' Voice", CastCount: 1, InZoneAtEnd: false, Visits: 1},
		},
		CommanderNamesBySeat: [][]string{nil, {"Atraxa, Praetors' Voice"}, nil, nil},
		FinalTurn:            9,
	}
	payload, err := heimdall.MarshalSnapshot(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	return gid
}

// --- GET /api/replay/{game_id} (full) -----------------------------------

func TestReplayFull_ReturnsAllEventsInSingleJSONDocument(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayEndpointsFixture(t, h)

	req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type=%q, want JSON", ct)
	}
	var resp FullReplayResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	if resp.GameID != gid {
		t.Errorf("game_id=%d, want %d", resp.GameID, gid)
	}
	if resp.Winner != 1 {
		t.Errorf("winner=%d, want 1", resp.Winner)
	}
	if resp.TotalTurns != 9 {
		t.Errorf("total_turns=%d, want 9", resp.TotalTurns)
	}
	if len(resp.Events) == 0 {
		t.Fatal("events should be populated (snapshot has 4 first-plays + 3 mvps + 1 game_end)")
	}
	// Last event must be game_end.
	if last := resp.Events[len(resp.Events)-1]; last.Kind != "game_end" {
		t.Errorf("last event kind=%q, want game_end", last.Kind)
	}
}

func TestReplayFull_400OnInvalidGameID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)
	for _, badID := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest("GET", "/api/replay/"+badID, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("game_id=%q: status=%d, want 400", badID, rec.Code)
		}
		assertUnifiedEnvelope(t, rec, "bad_request")
	}
}

func TestReplayFull_404OnUnknownGameID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)
	req := httptest.NewRequest("GET", "/api/replay/99999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}
	assertUnifiedEnvelope(t, rec, "not_found")
}

func TestReplayFull_404WhenSnapshotMissing(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	// Seed a game WITHOUT a snapshot — must 404 with "no observation
	// snapshot for game" via the unified envelope.
	gid := seedSummaryGame(t, h, 0, 5, "concede")

	req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "snapshot") {
		t.Errorf("404 body should mention snapshot, got %q", rec.Body.String())
	}
}

func TestReplayFull_503WhenDBMissing(t *testing.T) {
	h := &Handler{} // no db
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/replay/1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status=%d, want 503", rec.Code)
	}
	assertUnifiedEnvelope(t, rec, "service_unavailable")
}

// --- GET /api/replay/{game_id}/turn/{n} (turn projection) ---------------

func TestReplayTurn_FiltersEventsThroughGivenTurn(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayEndpointsFixture(t, h)

	req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid)+"/turn/5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp TurnStateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Turn != 5 {
		t.Errorf("turn=%d, want 5", resp.Turn)
	}
	if resp.TotalTurns != 9 {
		t.Errorf("total_turns=%d, want 9", resp.TotalTurns)
	}
	// Every event must have Turn ≤ 5; game_end must NOT appear
	// (snapshot ended at turn 9, but from turn 5's perspective the
	// game wasn't over).
	for _, ev := range resp.EventsThroughN {
		if ev.Turn > 5 {
			t.Errorf("event Turn=%d > 5: %+v", ev.Turn, ev)
		}
		if ev.Kind == "game_end" {
			t.Errorf("game_end leaked into pre-end turn projection: %+v", ev)
		}
	}
	// Sol Ring (turn 1), Sylvan Library (turn 3), Atraxa (turn 5)
	// should be present; Cyclonic Rift (turn 7) should NOT.
	gotNames := map[string]bool{}
	for _, ev := range resp.EventsThroughN {
		if ev.CardName != "" {
			gotNames[ev.CardName] = true
		}
	}
	if !gotNames["Sol Ring"] || !gotNames["Sylvan Library"] || !gotNames["Atraxa, Praetors' Voice"] {
		t.Errorf("missing expected turn≤5 names: got %v", gotNames)
	}
	if gotNames["Cyclonic Rift"] {
		t.Errorf("Cyclonic Rift (turn 7) leaked into turn=5 projection")
	}
	// MVPs: Sol Ring and Atraxa emerged by turn 5; Cyclonic Rift did NOT.
	if len(resp.MVPsEmerged) != 2 {
		t.Errorf("mvps_emerged count=%d, want 2 (Sol Ring + Atraxa); got %+v", len(resp.MVPsEmerged), resp.MVPsEmerged)
	}
}

func TestReplayTurn_IncludesGameEndWhenAskingForFinalTurn(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayEndpointsFixture(t, h)

	req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid)+"/turn/9", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp TurnStateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sawEnd := false
	for _, ev := range resp.EventsThroughN {
		if ev.Kind == "game_end" {
			sawEnd = true
		}
	}
	if !sawEnd {
		t.Error("game_end should appear when turn == total_turns")
	}
}

func TestReplayTurn_400OnInvalidTurn(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayEndpointsFixture(t, h)
	for _, badTurn := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid)+"/turn/"+badTurn, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("turn=%q: status=%d, want 400 (body=%s)", badTurn, rec.Code, rec.Body.String())
		}
		// The "turn" error must come back specifically (not "game id")
		// — proves the turn-validation gate runs AFTER snapshot load
		// rather than collapsing into the id gate.
		if !strings.Contains(rec.Body.String(), "turn") {
			t.Errorf("turn=%q: body must mention turn, got %q", badTurn, rec.Body.String())
		}
	}
}

func TestReplayTurn_TurnBeyondGameLengthReturnsAllEvents(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayEndpointsFixture(t, h)

	// Asking for turn=999 when game ended at 9 should return every
	// event including game_end (no over-filter, no error).
	req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid)+"/turn/999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp TurnStateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sawEnd := false
	for _, ev := range resp.EventsThroughN {
		if ev.Kind == "game_end" {
			sawEnd = true
		}
	}
	if !sawEnd {
		t.Error("turn > total_turns should still include game_end")
	}
}

// --- GET /api/replay/{game_id}/forensics --------------------------------

func TestReplayForensics_BundlesEveryAnalysisSignal(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayEndpointsFixture(t, h)

	req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid)+"/forensics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp ForensicsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	if resp.GameID != gid {
		t.Errorf("game_id=%d, want %d", resp.GameID, gid)
	}
	if resp.Winner != 1 {
		t.Errorf("winner=%d, want 1", resp.Winner)
	}
	if resp.TotalTurns != 9 {
		t.Errorf("total_turns=%d, want 9", resp.TotalTurns)
	}
	if len(resp.TurningPoints) == 0 {
		t.Error("turning_points empty — should at least have commander_first_cast + game_end")
	}
	if len(resp.MVPCards) != 3 {
		t.Errorf("mvp_cards count=%d, want 3", len(resp.MVPCards))
	}
	if len(resp.RegretCards) != 1 {
		t.Errorf("regret_cards count=%d, want 1", len(resp.RegretCards))
	}
	if len(resp.MulliganStats) != 2 {
		t.Errorf("mulligan_stats count=%d, want 2", len(resp.MulliganStats))
	}
	if len(resp.CommanderZoneVisits) != 1 {
		t.Errorf("commander_zone_visits count=%d, want 1", len(resp.CommanderZoneVisits))
	}
}

// Pin that nil-slice fields render as `[]` not `null` — JS array
// consumers shouldn't have to nil-check before iterating. Seed a
// snapshot with EVERY analysis array nil/missing and verify the
// JSON body still parses arrays cleanly.
func TestReplayForensics_NilSlicesRenderAsEmptyArrays(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	// Minimal snapshot — every analysis array nil.
	snap := heimdall.GameObservationSnapshot{
		Seed:      heimdall.GameSeed{RNGSeed: 1, Winner: 0, Turns: 5},
		FinalTurn: 5,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid)+"/forensics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	// writeJSON pretty-prints with indents, so normalize whitespace
	// between key and value before substring-matching.
	body := strings.Join(strings.Fields(rec.Body.String()), " ")
	for _, field := range []string{
		`"mvp_cards": []`,
		`"regret_cards": []`,
		`"mulligan_stats": []`,
		`"commander_zone_visits": []`,
	} {
		if !strings.Contains(body, field) {
			t.Errorf("body missing %q (must render nil slices as empty arrays, not null); body=%s", field, rec.Body.String())
		}
	}
	// Defensive: no `null` for these array fields anywhere.
	for _, field := range []string{
		`"mvp_cards": null`,
		`"regret_cards": null`,
		`"mulligan_stats": null`,
		`"commander_zone_visits": null`,
	} {
		if strings.Contains(body, field) {
			t.Errorf("body contains %q — must render nil as [] not null", field)
		}
	}
}

func TestReplayForensics_404OnMissingSnapshot(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "concede")
	req := httptest.NewRequest("GET", "/api/replay/"+intToStr(gid)+"/forensics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rec.Code)
	}
}

// --- Shared envelope assertion ------------------------------------------

// assertUnifiedEnvelope checks that an error response decodes as the
// unified ErrorResponse envelope (PR #778) with the expected code
// slug, the right Content-Type, and the nosniff guard. Used across
// every 400/404/503 case in this file so a future tweak to writeError
// can't accidentally regress one endpoint.
func assertUnifiedEnvelope(t *testing.T, rec *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type=%q, want application/json", ct)
	}
	if nosniff := rec.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("nosniff header missing")
	}
	var body ErrorResponse
	if err := json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&body); err != nil {
		t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, rec.Body.String())
	}
	if body.Error.Code != wantCode {
		t.Errorf("error.code=%q, want %q", body.Error.Code, wantCode)
	}
	if body.Status == 0 {
		t.Error("body.status missing")
	}
}
