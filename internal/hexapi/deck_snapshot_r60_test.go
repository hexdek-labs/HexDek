package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// Pins GET /api/decks/{owner}/{id}/snapshot — the tournament-ready
// summary endpoint: deck list + commander + bracket + primary archetype
// + recorded W/L stats in a single round-trip.

// newSnapshotEnv stands up a Handler with a temp DecksDir + in-memory
// SQLite, registers routes, and returns the handler + mux + decksDir.
func newSnapshotEnv(t *testing.T) (*Handler, http.Handler, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	decksDir := t.TempDir()
	h := &Handler{
		DecksDir:  decksDir,
		Showmatch: &Showmatch{sqlDB: d},
	}
	h.SetDB(d)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux, decksDir
}

// writeDeckFixture writes a deck list + (optional) Freya strategy.json
// to the temp decksDir so the snapshot handler has something to read.
func writeDeckFixture(t *testing.T, decksDir, owner, id, commander string, cards string, strategy map[string]any) {
	t.Helper()
	dir := filepath.Join(decksDir, owner)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "COMMANDER: " + commander + "\n" + cards
	if err := os.WriteFile(filepath.Join(dir, id+".txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if strategy != nil {
		fdir := filepath.Join(dir, "freya")
		if err := os.MkdirAll(fdir, 0o755); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(strategy)
		if err := os.WriteFile(filepath.Join(fdir, id+".strategy.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func getSnapshot(t *testing.T, mux http.Handler, owner, id string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/decks/"+owner+"/"+id+"/snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return rec, nil
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode snapshot: %v (body=%s)", err, rec.Body.String())
	}
	return rec, body
}

// Happy path: bracketed filename + Freya strategy.json + ELO row all
// present. Snapshot surfaces every field.
func TestHandleDeckSnapshot_HappyPath(t *testing.T) {
	h, mux, decksDir := newSnapshotEnv(t)
	writeDeckFixture(t, decksDir, "alice", "krenko_b3_rg",
		"Krenko, Mob Boss",
		"1 Sol Ring\n20 Mountain\n",
		map[string]any{"primary_archetype": "Aggro / Tokens", "bracket": 3})

	// Seed an ELO row so the record block has live data.
	if err := db.UpsertELO(context.Background(), h.Showmatch.sqlDB, db.ELORecord{
		DeckKey: "alice/krenko_b3_rg", Commander: "Krenko, Mob Boss", Owner: "alice",
		Rating: 1530.5, Games: 24, Wins: 13, Losses: 11, Bracket: 3,
	}); err != nil {
		t.Fatalf("UpsertELO: %v", err)
	}

	rec, body := getSnapshot(t, mux, "alice", "krenko_b3_rg")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	if got := body["owner"]; got != "alice" {
		t.Errorf("owner: got %v", got)
	}
	if got := body["id"]; got != "krenko_b3_rg" {
		t.Errorf("id: got %v", got)
	}
	if got := body["deck_key"]; got != "alice/krenko_b3_rg" {
		t.Errorf("deck_key: got %v", got)
	}
	if got, _ := body["bracket"].(float64); got != 3 {
		t.Errorf("bracket: want 3, got %v", body["bracket"])
	}
	if got := body["primary_archetype"]; got != "Aggro / Tokens" {
		t.Errorf("primary_archetype: got %v", got)
	}
	// parseDeckFilename returns "?" for color; it doesn't extract color
	// from the filename suffix. The snapshot endpoint surfaces whatever
	// resolveDeckMetadata returns; pin that contract here.
	if got := body["color"]; got != "?" {
		t.Errorf("color: want ? (parser doesn't extract from filename), got %v", got)
	}
	// parseDeckList appends the COMMANDER: line as a 1x card entry if
	// not already present, so the deck (COMMANDER + 1 Sol Ring + 20
	// Mountain) yields 3 entries and 22 total copies.
	if got, _ := body["card_count"].(float64); got != 22 {
		t.Errorf("card_count: want 22 (1 + 20 + 1 commander), got %v", body["card_count"])
	}
	cards, _ := body["cards"].([]any)
	if len(cards) != 3 {
		t.Errorf("cards: want 3 entries (Sol Ring + Mountain + commander), got %d", len(cards))
	}

	rec2, _ := body["record"].(map[string]any)
	if rec2 == nil {
		t.Fatalf("record missing: body=%v", body)
	}
	if g, _ := rec2["games"].(float64); g != 24 {
		t.Errorf("record.games: want 24, got %v", rec2["games"])
	}
	if w, _ := rec2["wins"].(float64); w != 13 {
		t.Errorf("record.wins: want 13, got %v", rec2["wins"])
	}
	if l, _ := rec2["losses"].(float64); l != 11 {
		t.Errorf("record.losses: want 11, got %v", rec2["losses"])
	}
	wr, _ := rec2["win_rate"].(float64)
	if wr < 0.541 || wr > 0.542 {
		t.Errorf("record.win_rate: want ~0.5417 (ratio not percent), got %v", rec2["win_rate"])
	}
	if r, _ := rec2["rating"].(float64); r != 1530.5 {
		t.Errorf("record.rating: want 1530.5, got %v", rec2["rating"])
	}
}

// No ELO row → record block is present with zero games and the
// rating / win_rate fields are omitted via omitempty.
func TestHandleDeckSnapshot_NoELORowZeroRecord(t *testing.T) {
	_, mux, decksDir := newSnapshotEnv(t)
	writeDeckFixture(t, decksDir, "bob", "atraxa_b4_wubg",
		"Atraxa, Praetors' Voice",
		"1 Sol Ring\n",
		map[string]any{"primary_archetype": "Superfriends", "bracket": 4})

	rec, body := getSnapshot(t, mux, "bob", "atraxa_b4_wubg")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	rec2, _ := body["record"].(map[string]any)
	if rec2 == nil {
		t.Fatalf("record missing")
	}
	if g, _ := rec2["games"].(float64); g != 0 {
		t.Errorf("games: want 0, got %v", rec2["games"])
	}
	if _, ok := rec2["win_rate"]; ok {
		t.Errorf("win_rate should be omitted when games=0; got %v", rec2["win_rate"])
	}
	if _, ok := rec2["rating"]; ok {
		t.Errorf("rating should be omitted when no ELO row; got %v", rec2["rating"])
	}
}

// Filename has no _bN_ marker AND no strategy.json → bracket=0
// (unresolvable). primary_archetype is "" but cards/commander still
// surface from the filesystem.
func TestHandleDeckSnapshot_UnresolvableBracket(t *testing.T) {
	_, mux, decksDir := newSnapshotEnv(t)
	writeDeckFixture(t, decksDir, "carol", "casual-pile",
		"Sliver Overlord",
		"1 Sol Ring\n",
		nil)

	rec, body := getSnapshot(t, mux, "carol", "casual-pile")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	if b, _ := body["bracket"].(float64); b != 0 {
		t.Errorf("bracket: want 0 (unresolvable), got %v", body["bracket"])
	}
	if a := body["primary_archetype"]; a != "" {
		t.Errorf("primary_archetype: want empty, got %v", a)
	}
}

// 404 when the deck file doesn't exist.
func TestHandleDeckSnapshot_404OnMissingDeck(t *testing.T) {
	_, mux, _ := newSnapshotEnv(t)

	req := httptest.NewRequest("GET", "/api/decks/nobody/no-such-deck/snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// 400 when the path component is unsafe (e.g. contains a char outside
// validatePathComponent's allowlist). The router matches the route, the
// handler rejects with 400. Uses "@" which is URL-legal but not in the
// [a-zA-Z0-9_.-] allowlist that validatePathComponent enforces.
func TestHandleDeckSnapshot_400OnInvalidPathChar(t *testing.T) {
	_, mux, _ := newSnapshotEnv(t)

	req := httptest.NewRequest("GET", "/api/decks/alice@bad/krenko/snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// Record block surfaces even when h.Showmatch is nil — empty zero
// values, no panic. Guards the share-card path where the deck import
// pipeline doesn't always wire Showmatch.
func TestHandleDeckSnapshot_RecordSafeWhenShowmatchNil(t *testing.T) {
	decksDir := t.TempDir()
	h := &Handler{DecksDir: decksDir} // no Showmatch
	mux := http.NewServeMux()
	h.Register(mux)

	writeDeckFixture(t, decksDir, "dave", "muldrotha_b4_bug",
		"Muldrotha, the Gravetide",
		"1 Sol Ring\n",
		nil)

	rec, body := getSnapshot(t, mux, "dave", "muldrotha_b4_bug")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	rec2, _ := body["record"].(map[string]any)
	if rec2 == nil {
		t.Fatalf("record missing")
	}
	if g, _ := rec2["games"].(float64); g != 0 {
		t.Errorf("games: want 0 with nil Showmatch, got %v", rec2["games"])
	}
}
