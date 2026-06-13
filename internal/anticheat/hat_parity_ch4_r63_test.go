package anticheat

import (
	"context"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// r63 anticheat residual C-H4 — hat identity parity. The verifier must
// NEVER sanction a game whose live hat it could not reproduce: a replay
// with a mismatched hat diverges on (winner, turns) exactly like a
// forgery, so acting on it would false-positive an honest contributor.
// These pin the fail-closed interlock from BOTH sides — the sanction
// decision contrast and the HeimdallVerifier gate.

// TestWorker_HatParity_InconclusiveVsActionable is the headline
// regression: holding the replay DIVERGENCE constant (replayed winner 0
// vs claimed 1), the ONLY thing that decides "sanction" vs "do not
// sanction" is the hat-parity signal. Inconclusive ⇒ quarantined, no
// sanction, game left unverified-pending; conclusive ⇒ failed + cauterize.
func TestWorker_HatParity_InconclusiveVsActionable(t *testing.T) {
	d := newTestDB(t)
	now := time.Now().Unix()
	keys := []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"}

	// Two games with an identical divergent replay outcome (winner 0,
	// claimed 1) — one reported Inconclusive, one not.
	gidQ := seedGame(t, d, keys, 1, 18, now-3600, 42)        // quarantined (hat not reproduced)
	gidA := seedGame(t, d, []string{"erin/stax", "fred/ramp"}, 1, 18, now-3500, 99) // actionable

	qidQ, _ := db.EnqueueVerification(context.Background(), d, db.VerificationEnqueueParams{
		GameID: gidQ, DeckKey: "alice/aggro", RNGSeed: 42, NSeats: 4,
		DeckKeys: keys, ClaimedWinner: 1, ClaimedTurns: 18,
	})
	qidA, _ := db.EnqueueVerification(context.Background(), d, db.VerificationEnqueueParams{
		GameID: gidA, DeckKey: "erin/stax", RNGSeed: 99, NSeats: 2,
		DeckKeys: []string{"erin/stax", "fred/ramp"}, ClaimedWinner: 1, ClaimedTurns: 18,
	})

	verifier := &stubVerifier{results: map[int64]ReplayOutcome{
		// Same divergence; differ ONLY in the hat-parity signal.
		qidQ: {Winner: 0, Turns: 18, Detail: "hat not reproduced", Inconclusive: true},
		qidA: {Winner: 0, Turns: 18, Detail: "winner mismatch"},
	}}
	w := NewVerificationWorker(d, verifier, NewCauterizeService(d), VerificationWorkerOptions{})

	// Drain both rows.
	for i := 0; i < 2; i++ {
		if _, err := w.ProcessOne(context.Background()); err != nil {
			t.Fatalf("ProcessOne %d: %v", i, err)
		}
	}

	// --- quarantined row: inconclusive, no sanction, game untouched ---
	incRows, _ := db.ListVerifications(context.Background(), d, "inconclusive", 10)
	if len(incRows) != 1 || incRows[0].QueueID != qidQ {
		t.Fatalf("expected 1 inconclusive row for qidQ, got %+v", incRows)
	}
	var verifiedQ int
	d.QueryRow(`SELECT verified FROM showmatch_game WHERE game_id=?`, gidQ).Scan(&verifiedQ)
	if verifiedQ != 0 {
		t.Errorf("inconclusive game must stay unverified-pending (0), got %d", verifiedQ)
	}
	if n, _ := db.CountOffenses(context.Background(), d, "alice/aggro"); n != 0 {
		t.Errorf("inconclusive must NOT sanction; alice/aggro has %d offenses", n)
	}

	// --- actionable row: failed, sanctioned, game flagged unverified ---
	failRows, _ := db.ListVerifications(context.Background(), d, "failed", 10)
	if len(failRows) != 1 || failRows[0].QueueID != qidA {
		t.Fatalf("expected 1 failed row for qidA, got %+v", failRows)
	}
	var verifiedA int
	d.QueryRow(`SELECT verified FROM showmatch_game WHERE game_id=?`, gidA).Scan(&verifiedA)
	if verifiedA != -1 {
		t.Errorf("actionable failed game must be flagged unverified (-1), got %d", verifiedA)
	}
	if n, _ := db.CountOffenses(context.Background(), d, "erin/stax"); n != 1 {
		t.Errorf("conclusive mismatch must sanction; erin/stax has %d offenses (want 1)", n)
	}
}

// TestWorker_HatReproduced_HonestClaimPasses confirms the legitimate-pass
// path is intact: a verifier that reproduced the hat (Inconclusive=false)
// and replays the claimed outcome marks the game verified, no sanction.
func TestWorker_HatReproduced_HonestClaimPasses(t *testing.T) {
	d := newTestDB(t)
	now := time.Now().Unix()
	gid := seedGame(t, d, []string{"alice/aggro", "bob/control"}, 1, 12, now-3600, 7)
	qid, _ := db.EnqueueVerification(context.Background(), d, db.VerificationEnqueueParams{
		GameID: gid, DeckKey: "alice/aggro", RNGSeed: 7, NSeats: 2,
		DeckKeys: []string{"alice/aggro", "bob/control"}, ClaimedWinner: 1, ClaimedTurns: 12,
	})
	verifier := &stubVerifier{results: map[int64]ReplayOutcome{
		qid: {Winner: 1, Turns: 12, Detail: "ok (hat reproduced)"}, // Inconclusive false
	}}
	w := NewVerificationWorker(d, verifier, NewCauterizeService(d), VerificationWorkerOptions{})
	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	var verified int
	d.QueryRow(`SELECT verified FROM showmatch_game WHERE game_id=?`, gid).Scan(&verified)
	if verified != 1 {
		t.Errorf("honest reproduced game should be marked verified (1), got %d", verified)
	}
	if n, _ := db.CountOffenses(context.Background(), d, "alice/aggro"); n != 0 {
		t.Errorf("honest pass must not sanction, got %d offenses", n)
	}
}

// --- HeimdallVerifier gate (corpus-free: short-circuits before replay) ---

func heimdallRow() db.VerificationQueueRow {
	return db.VerificationQueueRow{
		QueueID: 1, GameID: 1, NSeats: 2, RNGSeed: 123,
		DeckKeys: []string{"x/y", "a/b"}, ClaimedWinner: 0, ClaimedTurns: 5,
	}
}

// TestHeimdallVerifier_NoHatFactory_Inconclusive: with rc.HatFactory
// unset, the verifier refuses to render a sanctionable verdict and never
// runs a (necessarily wrong-hat) replay — so it needs no corpus.
func TestHeimdallVerifier_NoHatFactory_Inconclusive(t *testing.T) {
	rc := &heimdall.ReplayContext{} // HatFactory nil
	v := NewHeimdallVerifier(rc, []byte("0123456789abcdef"), "")
	out, err := v.Verify(context.Background(), heimdallRow())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Inconclusive {
		t.Fatalf("expected Inconclusive=true when HatFactory is nil, got %+v", out)
	}
}

// TestHeimdallVerifier_HatFactorySet_GatePasses: with rc.HatFactory set,
// the verifier does NOT short-circuit to inconclusive — it proceeds to
// replay (which errors here only because no corpus is loaded). The point:
// the gate is conditioned strictly on HatFactory presence.
func TestHeimdallVerifier_HatFactorySet_GatePasses(t *testing.T) {
	rc := &heimdall.ReplayContext{
		HatFactory: func(int) gameengine.Hat { return &hat.GreedyHat{} },
	}
	v := NewHeimdallVerifier(rc, []byte("0123456789abcdef"), "")
	out, err := v.Verify(context.Background(), heimdallRow())
	// No corpus ⇒ deck resolution fails ⇒ error. The invariant under test
	// is that the hat-parity gate did NOT fire (Inconclusive stays false).
	if out.Inconclusive {
		t.Fatalf("gate must not fire when HatFactory is set; got Inconclusive=true (%s)", out.Detail)
	}
	if err == nil {
		t.Log("note: replay did not error (corpus present); gate correctly did not fire")
	}
}
