package hexapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// setupReplayBench mirrors setupBenchHandler from
// game_summary_bench_r60_test.go but seeds the same realistic-
// shaped snapshot used there so the two benches are comparable.
func setupReplayBench(b *testing.B) (*Handler, http.Handler, int64) {
	b.Helper()
	d, err := db.Open(b.TempDir() + "/replaybench.db")
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	b.Cleanup(func() { d.Close() })
	h := &Handler{}
	h.SetDB(d)
	mux := http.NewServeMux()
	h.Register(mux)

	ctx := context.Background()
	gid, err := db.PersistGameTx(ctx, d, db.GameRecord{
		StartedAt: 100, FinishedAt: 200, Turns: 10, Winner: 1,
		WinnerName: "Atraxa", EndReason: "combat", Seed: 4242,
	}, []db.GameSeatRecord{
		{Seat: 0, Commander: "Commander A", DeckKey: "a/x"},
		{Seat: 1, Commander: "Commander B", DeckKey: "b/x"},
		{Seat: 2, Commander: "Commander C", DeckKey: "c/x"},
		{Seat: 3, Commander: "Commander D", DeckKey: "d/x"},
	})
	if err != nil {
		b.Fatalf("persist: %v", err)
	}
	payload, err := heimdall.MarshalSnapshot(newBenchSnapshot())
	if err != nil {
		b.Fatalf("marshal: %v", err)
	}
	if err := db.InsertGameObservation(ctx, d, gid, payload); err != nil {
		b.Fatalf("insert snapshot: %v", err)
	}
	return h, mux, gid
}

// End-to-end /replay bench — what the legality-auditor UI pays
// per polled refresh of a game.
func BenchmarkHandleGameReplay(b *testing.B) {
	_, mux, gid := setupReplayBench(b)
	url := "/api/games/" + intToStr(gid) + "/replay"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", url, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status: %d", rec.Code)
		}
	}
}

// Stage: BuildReplayLog — pure in-memory log derivation, so we
// can see how much of the response time is build-vs-IO.
func BenchmarkBuildReplayLog(b *testing.B) {
	snap := newBenchSnapshot()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = heimdall.BuildReplayLog(snap)
	}
}

// Isolated cache-hit bench — primes the cache once, then
// measures the steady-state loadCachedSnapshot cost so we can
// separate the win from the noise in the full-handler bench.
func BenchmarkLoadCachedSnapshot_Hit(b *testing.B) {
	h, _, gid := setupReplayBench(b)
	h.ensureSnapshotCache()
	ctx := context.Background()
	// Prime.
	if _, _, err := h.loadCachedSnapshot(ctx, gid); err != nil {
		b.Fatalf("prime: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := h.loadCachedSnapshot(ctx, gid); err != nil {
			b.Fatal(err)
		}
	}
}

// Isolated cache-miss bench — invalidates each call so every
// iteration pays the payload-fetch + unmarshal cost. Pairs with
// the hit bench so the speedup is computable.
func BenchmarkLoadCachedSnapshot_Miss(b *testing.B) {
	h, _, gid := setupReplayBench(b)
	h.ensureSnapshotCache()
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.snapshotCache.invalidate(gid)
		if _, _, err := h.loadCachedSnapshot(ctx, gid); err != nil {
			b.Fatal(err)
		}
	}
}
