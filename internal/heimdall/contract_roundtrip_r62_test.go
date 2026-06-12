package heimdall

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/seedcontract"
)

// r62 (review 08, C-H2) — the honest-game round-trip the anticheat
// verifier never had: seal a real game's outcome, verify it by replay,
// expect VALID; forge the outcome, expect INVALID.
//
// Needs the AST corpus (data/rules/ast_dataset.jsonl, gitignored) and
// the calibration deck files, so it SKIPS in environments without them
// (CI). Run locally with the corpus in place — it is the end-to-end
// proof that an honest seal→verify round-trip now passes.
//
// Determinism note: replayForOutcome stamps gs.Seed from the contract,
// so the default hat's noise RNG reseeds deterministically per seat
// (r62 #1020), and the calibration land decks contain none of the
// per_card global-rand cards (review 08, C-L1) — both replays here are
// pure functions of the contract inputs.

func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// internal/heimdall → repo root is two up.
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}

func newRoundtripContext(t *testing.T) *ReplayContext {
	t.Helper()
	root := repoRootForTest(t)
	astPath := filepath.Join(root, "data", "rules", "ast_dataset.jsonl")
	deckDir := filepath.Join(root, "data", "decks")
	if _, err := os.Stat(astPath); err != nil {
		t.Skipf("AST corpus not available (%v) — round-trip needs data/rules/ast_dataset.jsonl", err)
	}
	for _, k := range []string{"calibration/99lands_isamaru", "calibration/99lands_golos"} {
		p := filepath.Join(deckDir, filepath.FromSlash(k)+".txt")
		if _, err := os.Stat(p); err != nil {
			t.Skipf("calibration deck %s not available (%v)", k, err)
		}
	}
	rc, err := NewReplayContext(astPath, "", deckDir)
	if err != nil {
		t.Skipf("replay context unavailable: %v", err)
	}
	rc.EngineVersion = "roundtrip-test"
	return rc
}

func roundtripInputs() seedcontract.Inputs {
	return seedcontract.Inputs{
		RNGSeed:       424242,
		DeckKeys:      [4]string{"calibration/99lands_isamaru", "calibration/99lands_golos"},
		NSeats:        2,
		EngineVersion: "roundtrip-test",
		SealedAtUnix:  1_750_000_000, // fixed: digests must never depend on wall clock
	}
}

func TestVerifyReplay_HonestGameRoundTripsValid(t *testing.T) {
	if testing.Short() {
		t.Skip("full game replay — skipped in -short")
	}
	rc := newRoundtripContext(t)
	key := seedcontract.DeriveContractKey([]byte("rt-master"), "tournament:roundtrip")

	// Seal side: an honest game. The live runner now builds its sealed
	// outcome with the exact shared bookkeeping replayForOutcome uses
	// (tournament.ElimTracker / AdjudicateGameEnd / FinalLifeFromState),
	// so producing the honest outcome through replayForOutcome is
	// faithful to a live seal — and additionally proves the verify
	// path's replay is self-deterministic (two runs, same digest).
	contract := seedcontract.New(roundtripInputs())
	honest, err := replayForOutcome(rc, contract)
	if err != nil {
		t.Fatalf("honest game replay failed: %v", err)
	}
	contract.Seal(honest)
	contract.Sign(key)

	res, err := VerifyReplay(rc, contract, key)
	if err != nil {
		t.Fatalf("VerifyReplay error: %v", err)
	}
	if !res.OK {
		t.Fatalf("HONEST game failed verification: %s\n  sealed:   %+v\n  replayed: %+v",
			res.Detail, contract.Outcome, res.ReplayedOutcome)
	}

	// The game must have produced a real elimination — the exact field
	// the pre-r62 replay hardcoded to -1. A 2-seat decided game has
	// loser slot 0, winner slot 1.
	elim := contract.Outcome.EliminationOrder
	if elim[0] < 0 && elim[1] < 0 {
		t.Fatalf("round-trip game produced no elimination slots (%v) — test no longer exercises C-H2", elim)
	}
}

func TestVerifyReplay_ForgedOutcomeFails(t *testing.T) {
	if testing.Short() {
		t.Skip("full game replay — skipped in -short")
	}
	rc := newRoundtripContext(t)
	key := seedcontract.DeriveContractKey([]byte("rt-master"), "tournament:roundtrip")

	contract := seedcontract.New(roundtripInputs())
	honest, err := replayForOutcome(rc, contract)
	if err != nil {
		t.Fatalf("honest game replay failed: %v", err)
	}

	// Forgery with key access: flip the winner and re-seal + re-sign so
	// the contract is internally self-consistent (CheckIntegrity
	// passes). Only the replay can catch this — and must.
	forged := honest
	forged.Winner = 1 - honest.Winner
	contract.Seal(forged)
	contract.Sign(key)

	res, err := VerifyReplay(rc, contract, key)
	if err != nil {
		t.Fatalf("VerifyReplay error: %v", err)
	}
	if res.OK {
		t.Fatalf("FORGED winner passed verification (claimed %d, real %d)", forged.Winner, honest.Winner)
	}

	// Forgery without key access: mutate a stored field post-sign.
	// CheckIntegrity must catch it before any replay runs.
	contract2 := seedcontract.New(roundtripInputs())
	contract2.Seal(honest)
	contract2.Sign(key)
	contract2.Outcome.FinalLife[0] += 5
	res2, err := VerifyReplay(rc, contract2, key)
	if err != nil {
		t.Fatalf("VerifyReplay error: %v", err)
	}
	if res2.OK {
		t.Fatal("post-sign field mutation passed verification")
	}
}

func TestVerifyReplay_EngineVersionGate(t *testing.T) {
	// Corpus-free: the version gate fires before replay, so a bare
	// ReplayContext suffices.
	rc := &ReplayContext{EngineVersion: "build-B"}
	key := seedcontract.DeriveContractKey([]byte("k"), "tournament:vg")
	in := roundtripInputs()
	in.EngineVersion = "build-A"
	c := seedcontract.New(in)
	c.Seal(seedcontract.Outcome{Winner: 0, Turns: 10, EndReason: "last_seat_standing"})
	c.Sign(key)

	res, err := VerifyReplay(rc, c, key)
	if err != nil {
		t.Fatalf("VerifyReplay error: %v", err)
	}
	if res.OK {
		t.Fatal("engine-version mismatch passed verification")
	}
	if res.Detail == "" || res.ReplayedOutcome.Turns != 0 {
		t.Fatalf("version gate should fail fast without replaying; detail=%q replayed=%+v", res.Detail, res.ReplayedOutcome)
	}
}
