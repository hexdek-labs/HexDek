package hexapi

import (
	"context"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// elo_restart_tsmusigma_r62_test.go — pins that a server restart no longer
// erases the ladder's learned TrueSkill uncertainty (review r62, report 09
// C2). Pre-fix, ts_mu/ts_sigma were not persisted: loadPersistedState
// reconstructed EVERY deck with σ=σ₀(400), so each restart treated each
// deck as maximally uncertain and established ratings swung wildly until σ
// re-converged. The PR #996 clamp stays as the backstop for legacy and
// corrupted rows.

func TestFlushAndReload_PreservesTSMuSigmaAcrossRestart(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()

	// "First server life": an established deck with tight σ.
	sm1 := &Showmatch{
		sqlDB: d,
		elo: map[string]*eloState{
			"owner/established": {
				rating: 1700, hexRating: 1600, tsMu: 2060, tsSigma: 120,
				games: 800, wins: 230, delta: 2.5, hexDelta: 0.7,
				commander: "Cmdr", owner: "owner", bracket: 3,
			},
		},
	}
	sm1.flushELO()

	// "Restart": a fresh Showmatch reloads from the same database.
	sm2 := &Showmatch{sqlDB: d, elo: map[string]*eloState{}}
	sm2.loadPersistedState()

	e := sm2.elo["owner/established"]
	if e == nil {
		t.Fatal("established deck missing after reload")
	}
	if e.tsMu != 2060 || e.tsSigma != 120 {
		t.Errorf("restart erased TrueSkill state: got μ=%v σ=%v, want μ=2060 σ=120 (pre-r62 behavior was σ reset to %v)",
			e.tsMu, e.tsSigma, tsDefaultSigma)
	}
	if e.rating != 1700 || e.games != 800 || e.wins != 230 {
		t.Errorf("scalar state corrupted on reload: rating=%v games=%d wins=%d", e.rating, e.games, e.wins)
	}
}

// TestLoadPersistedState_LegacyRowFallsBackToBandAid pins the PR #996
// backstop: rows from before the ts_mu/ts_sigma columns existed read 0/0,
// which must trigger the band-aid reconstruction (σ=σ₀, μ=rating+3σ) —
// NOT be trusted as a literal σ=0, which makes every update degenerate.
func TestLoadPersistedState_LegacyRowFallsBackToBandAid(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	if err := db.UpsertELO(ctx, d, db.ELORecord{
		DeckKey: "owner/legacy", Rating: 1500, Games: 100, Bracket: 3,
		// TSMu / TSSigma left at 0/0 — the legacy sentinel.
	}); err != nil {
		t.Fatalf("UpsertELO: %v", err)
	}

	sm := &Showmatch{sqlDB: d, elo: map[string]*eloState{}}
	sm.loadPersistedState()

	e := sm.elo["owner/legacy"]
	if e == nil {
		t.Fatal("legacy deck missing after reload")
	}
	if e.tsSigma != tsDefaultSigma {
		t.Errorf("legacy row σ: got %v, want band-aid σ₀=%v", e.tsSigma, tsDefaultSigma)
	}
	wantMu := clampF(1500+3*tsDefaultSigma, tsMuFloor, tsMuCeil)
	if e.tsMu != wantMu {
		t.Errorf("legacy row μ: got %v, want reconstructed %v", e.tsMu, wantMu)
	}
}

// TestLoadPersistedState_CorruptTSStateFallsBackToBandAid pins the clamp
// backstop against out-of-band persisted values (the ±100M runaway class
// PR #996 was written for, now persisted instead of recomputed).
func TestLoadPersistedState_CorruptTSStateFallsBackToBandAid(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	if err := db.UpsertELO(ctx, d, db.ELORecord{
		DeckKey: "owner/corrupt", Rating: 1500, Bracket: 3,
		TSMu: 483_000_000, TSSigma: 5, // far out of the clampedTS band
	}); err != nil {
		t.Fatalf("UpsertELO: %v", err)
	}

	sm := &Showmatch{sqlDB: d, elo: map[string]*eloState{}}
	sm.loadPersistedState()

	e := sm.elo["owner/corrupt"]
	if e == nil {
		t.Fatal("corrupt deck missing after reload")
	}
	if e.tsSigma != tsDefaultSigma {
		t.Errorf("corrupt row σ: got %v, want band-aid σ₀=%v", e.tsSigma, tsDefaultSigma)
	}
	if e.tsMu < tsMuFloor || e.tsMu > tsMuCeil {
		t.Errorf("corrupt row μ not re-seeded into band: got %v", e.tsMu)
	}
}
