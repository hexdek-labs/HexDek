package db

import (
	"context"
	"path/filepath"
	"testing"
)

// trueskill_persist_r62_test.go — report 09 C-2.
//
// showmatch_elo persisted only the collapsed conservative rating
// (mu - 3*sigma); the raw TrueSkill mu/sigma were dropped, so every server
// restart reconstructed every deck at sigma_0=400 and the ladder was
// permanently stuck re-converging. These tests pin the new ts_mu /
// ts_sigma columns through the idempotent migration, single upsert, batch
// upsert, and load.

func TestTrueSkillMuSigma_RoundTrip(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "ts.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	want := ELORecord{
		DeckKey: "deck-a", Commander: "Cmdr A", Owner: "tester",
		Rating: 1730.5, HexRating: 1200, Games: 412, Wins: 130,
		Losses: 282, Delta: 4.2, HexDelta: -1.1, Bracket: 3,
		TSMu: 1990.25, TSSigma: 61.75,
	}
	if err := UpsertELO(ctx, d, want); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := LoadAllELO(ctx, d)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 record, got %d", len(got))
	}
	if got[0].TSMu != want.TSMu || got[0].TSSigma != want.TSSigma {
		t.Fatalf("mu/sigma did not round-trip: got (%v, %v) want (%v, %v)",
			got[0].TSMu, got[0].TSSigma, want.TSMu, want.TSSigma)
	}

	// Upsert path (ON CONFLICT) must update the pair too.
	want.TSMu, want.TSSigma = 2001.0, 58.5
	if err := UpsertELO(ctx, d, want); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err = LoadAllELO(ctx, d)
	if err != nil {
		t.Fatalf("re-load: %v", err)
	}
	if len(got) != 1 || got[0].TSMu != 2001.0 || got[0].TSSigma != 58.5 {
		t.Fatalf("conflict-update did not write mu/sigma: %+v", got)
	}
}

func TestTrueSkillMuSigma_BatchUpsertRoundTrip(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "ts-batch.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	records := []ELORecord{
		{DeckKey: "deck-a", Rating: 1500, TSMu: 1800, TSSigma: 100},
		{DeckKey: "deck-b", Rating: 1400, TSMu: 1650, TSSigma: 350},
	}
	if err := BatchUpsertELO(ctx, d, records); err != nil {
		t.Fatalf("batch upsert: %v", err)
	}
	got, err := LoadAllELO(ctx, d)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	byKey := map[string]ELORecord{}
	for _, r := range got {
		byKey[r.DeckKey] = r
	}
	for _, want := range records {
		g, ok := byKey[want.DeckKey]
		if !ok {
			t.Fatalf("record %s missing after batch upsert", want.DeckKey)
		}
		if g.TSMu != want.TSMu || g.TSSigma != want.TSSigma {
			t.Fatalf("%s: mu/sigma got (%v, %v) want (%v, %v)",
				want.DeckKey, g.TSMu, g.TSSigma, want.TSMu, want.TSSigma)
		}
	}
}

// TestTrueSkillMuSigma_MigrationIdempotent reopens the same database so
// applyMigrations runs a second time over a schema that already carries
// the columns, and confirms persisted values survive the reopen — the
// restart scenario the fix exists for.
func TestTrueSkillMuSigma_MigrationIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ts-migrate.db")
	ctx := context.Background()

	d, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := UpsertELO(ctx, d, ELORecord{DeckKey: "deck-a", Rating: 1600, TSMu: 1875, TSSigma: 92}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	d2, err := Open(path) // second migration pass must be a no-op
	if err != nil {
		t.Fatalf("reopen (migration idempotency): %v", err)
	}
	defer d2.Close()
	got, err := LoadAllELO(ctx, d2)
	if err != nil {
		t.Fatalf("load after reopen: %v", err)
	}
	if len(got) != 1 || got[0].TSMu != 1875 || got[0].TSSigma != 92 {
		t.Fatalf("mu/sigma lost across restart: %+v", got)
	}
}
