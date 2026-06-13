package heimdall

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/seedcontract"
)

// r63 anticheat residuals C-H2 #1 (max-turns faithfulness) and #4
// (worker claim-mode digest is structurally meaningless). These pin the
// two honest-game-rejection gaps the parked plan left open.

// --- Residual #4: ReplayClaim gate behavior (corpus-free) -------------

// TestReplayClaim_NilArgs — defensive parity with VerifyReplay.
func TestReplayClaim_NilArgs(t *testing.T) {
	if _, err := ReplayClaim(nil, nil, nil); err == nil {
		t.Fatal("expected error for nil ReplayContext/contract")
	}
	rc := &ReplayContext{}
	if _, err := ReplayClaim(rc, nil, nil); err == nil {
		t.Fatal("expected error for nil contract")
	}
}

// TestReplayClaim_IntegrityGate_NotADigestMismatch — a tampered claim
// contract must surface as an INTEGRITY failure, never as the
// "outcome digest mismatch" detail the pre-r63 worker path leaked. The
// integrity gate fires before any replay, so this runs without a corpus.
func TestReplayClaim_IntegrityGate_NotADigestMismatch(t *testing.T) {
	rc := &ReplayContext{} // no corpus needed; gate fires pre-replay
	key := seedcontract.DeriveContractKey([]byte("master"), "claim:integrity")

	c := seedcontract.New(roundtripInputs())
	// The worker seals a PARTIAL "claimed" outcome — exactly the shape
	// whose digest can never equal a real replay's.
	c.Seal(seedcontract.Outcome{Winner: 0, Turns: 7, EndReason: "claimed"})
	c.Sign(key)
	c.Outcome.Winner = 1 // tamper after signing

	res, err := ReplayClaim(rc, c, key)
	if err != nil {
		t.Fatalf("ReplayClaim error: %v", err)
	}
	if res.OK {
		t.Fatal("tampered claim contract passed ReplayClaim")
	}
	if !strings.HasPrefix(res.Detail, "integrity") {
		t.Fatalf("expected an integrity detail, got %q", res.Detail)
	}
	if strings.Contains(res.Detail, "outcome digest mismatch") {
		t.Fatalf("claim path must never emit the meaningless digest-mismatch detail; got %q", res.Detail)
	}
}

// TestReplayClaim_EngineVersionGate — mirrors VerifyReplay's gate: a
// build mismatch fails fast without replaying, with its own detail.
func TestReplayClaim_EngineVersionGate(t *testing.T) {
	rc := &ReplayContext{EngineVersion: "build-B"}
	key := seedcontract.DeriveContractKey([]byte("k"), "claim:vg")
	in := roundtripInputs()
	in.EngineVersion = "build-A"
	c := seedcontract.New(in)
	c.Seal(seedcontract.Outcome{Winner: 0, Turns: 10, EndReason: "claimed"})
	c.Sign(key)

	res, err := ReplayClaim(rc, c, key)
	if err != nil {
		t.Fatalf("ReplayClaim error: %v", err)
	}
	if res.OK {
		t.Fatal("engine-version mismatch passed ReplayClaim")
	}
	if res.ReplayedOutcome.Turns != 0 {
		t.Fatalf("version gate should fail fast without replaying; replayed=%+v", res.ReplayedOutcome)
	}
	if !strings.Contains(res.Detail, "engine version mismatch") {
		t.Fatalf("expected engine-version detail, got %q", res.Detail)
	}
}

// --- Residuals #1 + #4: corpus-gated faithfulness round-trip ----------

// TestReplayClaim_HonestClaim_And_MaxTurnsFaithful exercises both
// residuals end-to-end on the land-only calibration decks (which run to
// the turn cap, so rc.MaxTurns directly controls the outcome):
//
//   #4 — ReplayClaim on an honest game reports an honest detail (no
//        "outcome digest mismatch") and returns the replayed
//        (winner, turns) the worker compares against the claim.
//   #1 — the SAME sealed game verified under a DIFFERENT cap diverges,
//        proving the replay cap is caller-controlled (rc.MaxTurns) and
//        an honest game only re-verifies under the cap it was sealed at.
//
// Skips without the AST corpus (data/rules/ast_dataset.jsonl) + the
// calibration decks, the same gate as the r62 round-trip tests.
func TestReplayClaim_HonestClaim_And_MaxTurnsFaithful(t *testing.T) {
	if testing.Short() {
		t.Skip("full game replay — skipped in -short")
	}
	rc := newRoundtripContext(t)
	key := seedcontract.DeriveContractKey([]byte("rt-master"), "tournament:roundtrip")

	// cap1 is low enough that the land-only game CANNOT end by then (a
	// 2-power commander deals at most 2*(cap1-1) damage — far under 21
	// commander / 40 life), so the loop is cut by the cap, not a natural
	// end. That's what makes the cap observable.
	const cap1 = 8
	const cap2 = 14

	rc.MaxTurns = cap1
	c := seedcontract.New(roundtripInputs())
	honest, err := replayForOutcome(rc, c)
	if err != nil {
		t.Fatalf("honest replay failed: %v", err)
	}
	if honest.Turns != cap1 {
		t.Fatalf("expected land-only game cut by cap %d, got %d turns (%s) — pick a lower cap1",
			cap1, honest.Turns, honest.EndReason)
	}
	c.Seal(honest)
	c.Sign(key)

	// Residual #4: honest claim → honest detail, no digest-mismatch leak.
	res, err := ReplayClaim(rc, c, key)
	if err != nil {
		t.Fatalf("ReplayClaim error: %v", err)
	}
	if !res.OK {
		t.Fatalf("honest claim failed ReplayClaim: %s", res.Detail)
	}
	if strings.Contains(res.Detail, "mismatch") {
		t.Fatalf("honest claim leaked a digest-mismatch detail: %q", res.Detail)
	}
	if res.ReplayedOutcome.Winner != honest.Winner || res.ReplayedOutcome.Turns != honest.Turns {
		t.Fatalf("claim replay diverged from seal: got (w=%d t=%d) want (w=%d t=%d)",
			res.ReplayedOutcome.Winner, res.ReplayedOutcome.Turns, honest.Winner, honest.Turns)
	}

	// Residual #1: re-verify under the matching cap → OK; under a
	// different cap → divergence. Pre-fix the replay was hardwired to 80
	// and an honest non-default-cap game could never re-verify.
	rc.MaxTurns = cap1
	if r, _ := VerifyReplay(rc, c, key); !r.OK {
		t.Fatalf("honest game failed VerifyReplay at its sealed cap %d: %s", cap1, r.Detail)
	}
	rc.MaxTurns = cap2
	if r, _ := VerifyReplay(rc, c, key); r.OK {
		t.Fatalf("game sealed at cap %d verified OK at cap %d — rc.MaxTurns not honored by the replay loop",
			cap1, cap2)
	}
}
