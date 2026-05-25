package hexapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// setupPDFBench mirrors setupBenchHandler from
// game_summary_bench_r60_test.go and seeds the same realistic-
// shaped snapshot so the /summary, /replay, and /summary.pdf
// benches are directly comparable.
func setupPDFBench(b *testing.B) (*Handler, http.Handler, int64) {
	b.Helper()
	d, err := db.Open(b.TempDir() + "/pdfbench.db")
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

// End-to-end /summary.pdf bench — what a user pays per PDF
// download. Same cache as /summary (#202) and /replay (#212);
// the bench captures the steady-state hit path since the cache
// gets primed by the first iteration.
func BenchmarkHandleGameSummaryPDF(b *testing.B) {
	_, mux, gid := setupPDFBench(b)
	url := "/api/games/" + intToStr(gid) + "/summary.pdf"
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

// Stage: RenderGameSummaryPDF — pure in-memory byte-building, so
// we can see how much of the PDF response is the renderer vs
// everything else (load + summary-build + response writes).
// Larger than RenderJSON because PDFs include layout, font
// declarations, xref tables, etc.
func BenchmarkRenderGameSummaryPDF(b *testing.B) {
	snap := newBenchSnapshot()
	summary := heimdall.BuildGameSummaryFromSnapshot(snap, "combat")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = heimdall.RenderGameSummaryPDF(summary)
	}
}
