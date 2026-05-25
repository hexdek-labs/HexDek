package hexapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// seedReplayTest seeds a game + a snapshot with a couple of plays
// and an MVP so the replay handler has real events to stream.
func seedReplayTest(t *testing.T, h *Handler) int64 {
	t.Helper()
	gid := seedSummaryGame(t, h, 1, 9, "combat")
	snap := heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{
			RNGSeed: 4242,
			Winner:  1,
			Turns:   9,
		},
		CardFirstPlayed: map[string]int{
			"Sol Ring":                 1,
			"Counterspell":             3,
			"Atraxa, Praetors' Voice": 5,
		},
		MVPCards: []heimdall.MVPCard{
			{Seat: 1, CardName: "Atraxa, Praetors' Voice", CMC: 4, TurnPlayed: 5, Score: 8, IsCommander: true},
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

// decodeNDJSON parses an NDJSON response body into a slice of
// ReplayEvent. Each non-empty line must be a valid JSON event.
func decodeNDJSON(t *testing.T, body string) []heimdall.ReplayEvent {
	t.Helper()
	var out []heimdall.ReplayEvent
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev heimdall.ReplayEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("decode line %q: %v", line, err)
		}
		out = append(out, ev)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner: %v", err)
	}
	return out
}

func TestHandleGameReplay_StreamsNDJSONInChronologicalOrder(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayTest(t, h)

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("content-type: want application/x-ndjson, got %q", ct)
	}

	events := decodeNDJSON(t, rec.Body.String())
	// Expected: turn 1 Sol Ring cast, turn 3 Counterspell cast,
	// turn 5 Atraxa cast, turn 5 Atraxa MVP, turn 9 game_end.
	if len(events) != 5 {
		t.Fatalf("want 5 events, got %d (%+v)", len(events), events)
	}
	wantTurns := []int{1, 3, 5, 5, 9}
	wantKinds := []string{"card_first_played", "card_first_played", "card_first_played", "mvp_emerged", "game_end"}
	for i, e := range events {
		if e.Turn != wantTurns[i] || e.Kind != wantKinds[i] {
			t.Errorf("position %d: want (turn=%d kind=%s), got %+v",
				i, wantTurns[i], wantKinds[i], e)
		}
	}
	// Atraxa is the commander; both its play and MVP events should
	// attribute to seat 1.
	for _, e := range events {
		if e.CardName == "Atraxa, Praetors' Voice" && e.Seat != 1 {
			t.Errorf("Atraxa event should attribute to seat 1; got %+v", e)
		}
	}
	// game_end carries the winner seat.
	last := events[len(events)-1]
	if last.Seat != 1 || !strings.Contains(last.Description, "won on turn 9") {
		t.Errorf("game_end: %+v", last)
	}
}

// Streaming: NDJSON output flushes after each event when the
// ResponseWriter implements http.Flusher. httptest.ResponseRecorder
// does NOT implement Flusher, but we can verify the handler doesn't
// crash and that every event is in the body even when the recorder
// can't flush. Streaming-Flusher behavior is tested via a fake
// ResponseWriter wrapper below.
func TestHandleGameReplay_NoFlusherStillEmitsAllEvents(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayTest(t, h)

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder() // does NOT implement http.Flusher
	mux.ServeHTTP(rec, req)

	events := decodeNDJSON(t, rec.Body.String())
	if len(events) != 5 {
		t.Errorf("want 5 events even without Flusher, got %d", len(events))
	}
}

// flushingRecorder wraps httptest.ResponseRecorder + counts how
// many times Flush() was called. Used to assert the handler
// actively flushes after each event.
type flushingRecorder struct {
	*httptest.ResponseRecorder
	flushes int
}

func (f *flushingRecorder) Flush() { f.flushes++ }

func TestHandleGameReplay_FlushesAfterEveryEvent(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayTest(t, h)

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := &flushingRecorder{ResponseRecorder: httptest.NewRecorder()}
	mux.ServeHTTP(rec, req)

	events := decodeNDJSON(t, rec.Body.String())
	if len(events) != 5 {
		t.Fatalf("want 5 events, got %d", len(events))
	}
	if rec.flushes != 5 {
		t.Errorf("Flush should be called once per event (5); got %d", rec.flushes)
	}
}

// X-Accel-Buffering: no — so Caddy/nginx pass chunked writes
// through immediately rather than buffering the whole response.
func TestHandleGameReplay_DisablesProxyBuffering(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedReplayTest(t, h)

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if v := rec.Header().Get("X-Accel-Buffering"); v != "no" {
		t.Errorf("X-Accel-Buffering: want %q, got %q", "no", v)
	}
}

func TestHandleGameReplay_404OnUnknownID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)

	req := httptest.NewRequest("GET", "/api/games/999999/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rec.Code)
	}
}

func TestHandleGameReplay_404WhenSnapshotMissing(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	// Intentionally do NOT insert a snapshot.

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404 (no snapshot), got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "snapshot") {
		t.Errorf("404 body should mention snapshot; got %s", rec.Body.String())
	}
}

func TestHandleGameReplay_400OnInvalidID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)
	for _, bad := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest("GET", "/api/games/"+bad+"/replay", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("id=%q: want 400, got %d", bad, rec.Code)
		}
	}
}

func TestHandleGameReplay_503WhenDBMissing(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/games/1/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d", rec.Code)
	}
}

func TestHandleGameReplay_500OnMalformedSnapshot(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	if err := db.InsertGameObservation(context.Background(), h.db, gid, "{{ not json"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500 on malformed snapshot, got %d", rec.Code)
	}
}

// Snapshot with no events (no CardFirstPlayed, no MVPs) still
// streams the single game_end event — useful contract for clients
// that always expect at least one line.
func TestHandleGameReplay_EmptySnapshotEmitsOnlyGameEnd(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 1, "timeout")

	snap := heimdall.GameObservationSnapshot{
		Seed:      heimdall.GameSeed{RNGSeed: 4242, Winner: 0, Turns: 1},
		FinalTurn: 1,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	events := decodeNDJSON(t, rec.Body.String())
	if len(events) != 1 || events[0].Kind != "game_end" {
		t.Errorf("empty snapshot should still stream one game_end; got %+v", events)
	}
}
