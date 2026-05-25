package hexapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// Benchmark stages of GET /api/games/{id}/summary so we can see
// which one dominates and whether the bottleneck moves after
// the fix. All benches use realistic-shaped snapshots: 12 MVPs
// (3 per seat × 4 seats), 8 regret cards, 4 commander zone
// visits, ~30 entries in CardFirstPlayed.

func newBenchSnapshot() heimdall.GameObservationSnapshot {
	visits := make([]heimdall.CommanderZoneVisit, 0, 4)
	for s := 0; s < 4; s++ {
		visits = append(visits, heimdall.CommanderZoneVisit{
			Seat: s, CommanderName: "Commander " + string(rune('A'+s)),
			CastCount: 1 + s, InZoneAtEnd: s%2 == 0, Visits: 2 + s,
		})
	}
	regrets := make([]heimdall.RegretCard, 0, 8)
	for i := 0; i < 8; i++ {
		regrets = append(regrets, heimdall.RegretCard{
			Seat: i % 4, CardName: "Stranded Card " + string(rune('A'+i)),
			CMC: 4 + (i % 6), Reason: "stranded_in_hand",
		})
	}
	mvps := make([]heimdall.MVPCard, 0, 12)
	for i := 0; i < 12; i++ {
		mvps = append(mvps, heimdall.MVPCard{
			Seat: i % 4, CardName: "MVP Card " + string(rune('A'+i)),
			CMC: 3 + (i % 5), TurnPlayed: 2 + i, Counters: i % 4,
			Score: 5 + i, IsCommander: i < 4,
		})
	}
	first := make(map[string]int, 30)
	for i := 0; i < 30; i++ {
		first["Card "+string(rune('A'+i%26))+string(rune('A'+i/26))] = 1 + i%10
	}
	cmdrs := make([][]string, 4)
	for s := 0; s < 4; s++ {
		cmdrs[s] = []string{"Commander " + string(rune('A'+s))}
	}
	return heimdall.GameObservationSnapshot{
		Seed:                 heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 10},
		CommanderZoneVisits:  visits,
		RegretCards:          regrets,
		MVPCards:             mvps,
		CardFirstPlayed:      first,
		CommanderNamesBySeat: cmdrs,
		FinalTurn:            10,
	}
}

func setupBenchHandler(b *testing.B) (*Handler, http.Handler, int64) {
	b.Helper()
	d, err := db.Open(b.TempDir() + "/bench.db")
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
		b.Fatalf("persist game: %v", err)
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

// End-to-end handler bench — what the user actually pays.
func BenchmarkHandleGameSummary_RichPath(b *testing.B) {
	_, mux, gid := setupBenchHandler(b)
	url := "/api/games/" + intToStr(gid) + "/summary"
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

// Stage 1: DB game-row fetch.
func BenchmarkLoadGameByID(b *testing.B) {
	h, _, gid := setupBenchHandler(b)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := db.LoadGameByID(ctx, h.db, gid); err != nil {
			b.Fatal(err)
		}
	}
}

// Stage 2: DB snapshot fetch (TEXT payload column).
func BenchmarkLoadGameObservation(b *testing.B) {
	h, _, gid := setupBenchHandler(b)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := db.LoadGameObservation(ctx, h.db, gid); err != nil {
			b.Fatal(err)
		}
	}
}

// Stage 3: JSON unmarshal of the snapshot payload.
func BenchmarkUnmarshalSnapshot(b *testing.B) {
	payload, err := heimdall.MarshalSnapshot(newBenchSnapshot())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := heimdall.UnmarshalSnapshot(payload); err != nil {
			b.Fatal(err)
		}
	}
}

// Stage 4: BuildGameSummaryFromSnapshot — pure in-memory shape.
func BenchmarkBuildGameSummaryFromSnapshot(b *testing.B) {
	snap := newBenchSnapshot()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = heimdall.BuildGameSummaryFromSnapshot(snap, "combat")
	}
}
