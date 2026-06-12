package db

import (
	"context"
	"testing"
)

// elo_tsmusigma_r62_test.go — pins the r62 ts_mu/ts_sigma persistence
// (review r62, report 09 C2). Before these columns, showmatch_elo stored
// only the conservative rating (μ−3σ); every server restart reconstructed
// σ=σ₀(400) for all decks (the PR #996 band-aid), erasing the ladder's
// learned certainty on every deploy/crash.

func TestUpsertELO_RoundTripsTSMuSigma(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()

	in := ELORecord{
		DeckKey: "owner/deck", Commander: "Cmdr", Owner: "owner",
		Rating: 1640, HexRating: 1500, Games: 500, Wins: 130,
		Losses: 370, Delta: 4.2, HexDelta: 1.1, Bracket: 3,
		TSMu: 2000, TSSigma: 120,
	}
	if err := UpsertELO(ctx, d, in); err != nil {
		t.Fatalf("UpsertELO: %v", err)
	}

	recs, err := LoadAllELO(ctx, d)
	if err != nil {
		t.Fatalf("LoadAllELO: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0].TSMu != 2000 || recs[0].TSSigma != 120 {
		t.Errorf("TS state lost in round trip: got μ=%v σ=%v, want μ=2000 σ=120",
			recs[0].TSMu, recs[0].TSSigma)
	}

	// Upsert path (conflict on deck_key) must also carry the TS columns.
	in.TSMu, in.TSSigma = 2050, 110
	if err := UpsertELO(ctx, d, in); err != nil {
		t.Fatalf("UpsertELO update: %v", err)
	}
	recs, err = LoadAllELO(ctx, d)
	if err != nil {
		t.Fatalf("LoadAllELO after update: %v", err)
	}
	if recs[0].TSMu != 2050 || recs[0].TSSigma != 110 {
		t.Errorf("TS state lost in upsert-update: got μ=%v σ=%v, want μ=2050 σ=110",
			recs[0].TSMu, recs[0].TSSigma)
	}
}

func TestBatchUpsertELO_RoundTripsTSMuSigma(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()

	in := []ELORecord{
		{DeckKey: "a/one", Rating: 1500, Bracket: 2, TSMu: 2700, TSSigma: 400},
		{DeckKey: "b/two", Rating: 1800, Bracket: 4, TSMu: 2100, TSSigma: 95},
	}
	if err := BatchUpsertELO(ctx, d, in); err != nil {
		t.Fatalf("BatchUpsertELO: %v", err)
	}
	recs, err := LoadAllELO(ctx, d)
	if err != nil {
		t.Fatalf("LoadAllELO: %v", err)
	}
	got := map[string][2]float64{}
	for _, r := range recs {
		got[r.DeckKey] = [2]float64{r.TSMu, r.TSSigma}
	}
	if got["a/one"] != [2]float64{2700, 400} {
		t.Errorf("a/one TS state: got %v, want [2700 400]", got["a/one"])
	}
	if got["b/two"] != [2]float64{2100, 95} {
		t.Errorf("b/two TS state: got %v, want [2100 95]", got["b/two"])
	}
}

// TestApplyMigrations_AddsTSColumnsToLegacyTable simulates a database
// created before the r62 columns: drop them, re-run the migration pass,
// and verify they come back with the 0.0 legacy-sentinel default.
func TestApplyMigrations_AddsTSColumnsToLegacyTable(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()

	for _, col := range []string{"ts_mu", "ts_sigma"} {
		if _, err := d.Exec("ALTER TABLE showmatch_elo DROP COLUMN " + col); err != nil {
			t.Fatalf("drop %s (legacy-table setup): %v", col, err)
		}
	}
	// Legacy row written while the columns are absent.
	if _, err := d.Exec(`INSERT INTO showmatch_elo (deck_key, rating, updated_at) VALUES ('legacy/deck', 1500, 0)`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	if err := applyMigrations(d); err != nil {
		t.Fatalf("applyMigrations on legacy table: %v", err)
	}
	for _, col := range []string{"ts_mu", "ts_sigma"} {
		ok, err := columnExists(d, "showmatch_elo", col)
		if err != nil {
			t.Fatalf("columnExists(%s): %v", col, err)
		}
		if !ok {
			t.Errorf("migration did not add showmatch_elo.%s", col)
		}
	}

	recs, err := LoadAllELO(ctx, d)
	if err != nil {
		t.Fatalf("LoadAllELO post-migration: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0].TSMu != 0 || recs[0].TSSigma != 0 {
		t.Errorf("legacy row must read 0/0 (the reconstruct-on-load sentinel): got μ=%v σ=%v",
			recs[0].TSMu, recs[0].TSSigma)
	}
}
