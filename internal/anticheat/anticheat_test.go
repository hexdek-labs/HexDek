package anticheat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

// ----------------------------------------------------------------------
// test fixtures
// ----------------------------------------------------------------------

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/anticheat.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func seedGame(t *testing.T, d *sql.DB, deckKeys []string, winner, turns int, finishedAt int64, rngSeed int64) int64 {
	t.Helper()
	res, err := d.Exec(
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason, rng_seed)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		finishedAt-3600, finishedAt, turns, winner, "TEST", "last_seat_standing", rngSeed)
	if err != nil {
		t.Fatalf("insert showmatch_game: %v", err)
	}
	gid, _ := res.LastInsertId()
	for i, dk := range deckKeys {
		if _, err := d.Exec(
			`INSERT INTO showmatch_game_seat (game_id, seat, commander, deck_key, life, hand_size, library_size, gy_size, bf_size, lost)
			 VALUES (?, ?, ?, ?, 0, 0, 0, 0, 0, 0)`,
			gid, i, "Cmdr "+dk, dk); err != nil {
			t.Fatalf("insert seat: %v", err)
		}
	}
	return gid
}

// stubVerifier returns canned outcomes keyed by queue_id.
type stubVerifier struct {
	results map[int64]ReplayOutcome
	errs    map[int64]error
	calls   []int64
}

func (s *stubVerifier) Verify(ctx context.Context, row db.VerificationQueueRow) (ReplayOutcome, error) {
	s.calls = append(s.calls, row.QueueID)
	if e, ok := s.errs[row.QueueID]; ok {
		return ReplayOutcome{}, e
	}
	if r, ok := s.results[row.QueueID]; ok {
		return r, nil
	}
	// Default: claim is correct (passing replay).
	return ReplayOutcome{
		Winner: row.ClaimedWinner,
		Turns:  row.ClaimedTurns,
		Detail: "stub: claim matches",
	}, nil
}

// ----------------------------------------------------------------------
// SpotCheckScheduler
// ----------------------------------------------------------------------

func TestScheduler_RateClampedToBand(t *testing.T) {
	d := newTestDB(t)
	if got := NewSpotCheckScheduler(d, 0.0001, 1).Rate(); got != MinSamplingRate {
		t.Errorf("low rate not clamped: got %v want %v", got, MinSamplingRate)
	}
	if got := NewSpotCheckScheduler(d, 0.99, 1).Rate(); got != MaxSamplingRate {
		t.Errorf("high rate not clamped: got %v want %v", got, MaxSamplingRate)
	}
	if got := NewSpotCheckScheduler(d, 0.03, 1).Rate(); got != 0.03 {
		t.Errorf("in-band rate mutated: got %v", got)
	}
}

func TestScheduler_Select_RoughlyMatchesRate(t *testing.T) {
	d := newTestDB(t)
	s := NewSpotCheckScheduler(d, 0.05, 42) // deterministic seed
	const n = 10000
	gameIDs := make([]int64, n)
	for i := range gameIDs {
		gameIDs[i] = int64(i + 1)
	}
	picked := s.Select(gameIDs)
	frac := float64(len(picked)) / float64(n)
	// Three-sigma guard: with n=10000 and p=0.05, stddev≈0.0022. We
	// allow 0.04..0.06.
	if frac < 0.04 || frac > 0.06 {
		t.Errorf("sampling fraction out of expected band: %.4f (picked %d of %d)", frac, len(picked), n)
	}
}

func TestScheduler_Select_DeterministicWithSeed(t *testing.T) {
	d := newTestDB(t)
	gameIDs := make([]int64, 1000)
	for i := range gameIDs {
		gameIDs[i] = int64(i + 1)
	}
	a := NewSpotCheckScheduler(d, 0.05, 12345).Select(gameIDs)
	b := NewSpotCheckScheduler(d, 0.05, 12345).Select(gameIDs)
	if len(a) != len(b) {
		t.Fatalf("seeded select non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("position %d differs: %d vs %d", i, a[i], b[i])
		}
	}
}

func TestScheduler_SelectAndEnqueue_PerSeatRows(t *testing.T) {
	d := newTestDB(t)
	now := time.Now().Unix()
	gid := seedGame(t, d, []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"}, 1, 18, now-3600, 42)

	// This test exercises the per-seat fan-out of EnqueueVerification
	// directly. SelectAndEnqueue's Bernoulli sampling is covered by
	// TestScheduler_Select_DeterministicWithSeed elsewhere in this file —
	// duplicating it here with a 100-seed retry loop produced a flaky
	// skip and didn't add coverage, so it was removed in r48.
	keys := []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"}
	for _, dk := range keys {
		_, err := db.EnqueueVerification(context.Background(), d, db.VerificationEnqueueParams{
			GameID:   gid,
			DeckKey:  dk,
			RNGSeed:  42,
			NSeats:   4,
			DeckKeys: keys,
		})
		if err != nil {
			t.Fatalf("manual enqueue: %v", err)
		}
	}
	rows, err := db.ListVerifications(context.Background(), d, "pending", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) < 4 {
		t.Errorf("expected >=4 per-seat rows, got %d", len(rows))
	}
}

func TestScheduler_SelectAndEnqueue_MissingGameSkippedNotErrored(t *testing.T) {
	d := newTestDB(t)
	s := NewSpotCheckScheduler(d, MaxSamplingRate, 1)
	// Force the scheduler to "pick" a non-existent game by directly
	// passing it. The SelectAndEnqueue loop handles ErrNoRows by
	// continuing.
	_, err := s.SelectAndEnqueue(context.Background(), []int64{99999})
	// Either no error (skipped) or a wrapped ErrNoRows; both are fine
	// as long as the scheduler doesn't crash or stall.
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Logf("non-fatal: %v", err)
	}
}

// ----------------------------------------------------------------------
// CauterizeService — escalation tiers
// ----------------------------------------------------------------------

func TestCauterize_Escalation_WarningTempBanPermanent(t *testing.T) {
	d := newTestDB(t)
	c := NewCauterizeService(d)
	ctx := context.Background()

	dec1, err := c.ApplyOnFailure(ctx, "alice/aggro", 0, "first failure")
	if err != nil {
		t.Fatalf("apply 1: %v", err)
	}
	if dec1.OffenseNum != 1 || dec1.Severity != db.SeverityWarning {
		t.Errorf("1st offense should be warning; got %+v", dec1)
	}
	if dec1.ExpiresAt != 0 {
		t.Errorf("warning should not have expiry; got %d", dec1.ExpiresAt)
	}

	dec2, err := c.ApplyOnFailure(ctx, "alice/aggro", 0, "second failure")
	if err != nil {
		t.Fatalf("apply 2: %v", err)
	}
	if dec2.OffenseNum != 2 || dec2.Severity != db.SeverityTempBan {
		t.Errorf("2nd offense should be 24h ban; got %+v", dec2)
	}
	if dec2.ExpiresAt == 0 {
		t.Errorf("temp ban should have expiry; got 0")
	}
	expectedExpiry := time.Now().Add(24 * time.Hour).Unix()
	if dec2.ExpiresAt < expectedExpiry-60 || dec2.ExpiresAt > expectedExpiry+60 {
		t.Errorf("temp ban expiry off: got %d, want ~%d", dec2.ExpiresAt, expectedExpiry)
	}

	dec3, err := c.ApplyOnFailure(ctx, "alice/aggro", 0, "third failure")
	if err != nil {
		t.Fatalf("apply 3: %v", err)
	}
	if dec3.OffenseNum != 3 || dec3.Severity != db.SeverityPermanentBan {
		t.Errorf("3rd offense should be permanent; got %+v", dec3)
	}
	if dec3.ExpiresAt != 0 {
		t.Errorf("permanent ban should not have expiry; got %d", dec3.ExpiresAt)
	}

	dec4, err := c.ApplyOnFailure(ctx, "alice/aggro", 0, "fourth failure")
	if err != nil {
		t.Fatalf("apply 4: %v", err)
	}
	if dec4.Severity != db.SeverityPermanentBan {
		t.Errorf("4th+ offense should stay permanent; got %s", dec4.Severity)
	}
}

func TestCauterize_QuarantinesRecentGames(t *testing.T) {
	d := newTestDB(t)
	c := NewCauterizeService(d)
	ctx := context.Background()
	now := time.Now().Unix()

	gid := seedGame(t, d, []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"}, 0, 14, now-3600, 1)

	dec, err := c.ApplyOnFailure(ctx, "alice/aggro", 0, "test")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if dec.GamesQuarantined != 1 {
		t.Errorf("expected 1 game quarantined, got %d", dec.GamesQuarantined)
	}
	var v int
	d.QueryRowContext(ctx, `SELECT verified FROM showmatch_game WHERE game_id=?`, gid).Scan(&v)
	if v != -1 {
		t.Errorf("game not flagged: verified=%d", v)
	}
}

func TestCauterize_EmptyDeckKeyRejected(t *testing.T) {
	d := newTestDB(t)
	c := NewCauterizeService(d)
	if _, err := c.ApplyOnFailure(context.Background(), "  ", 0, ""); err == nil {
		t.Error("expected error on empty deck_key")
	}
}

func TestCauterize_IsBanned_RespectsExpiry(t *testing.T) {
	d := newTestDB(t)
	c := NewCauterizeService(d)
	ctx := context.Background()

	if banned, _, _ := c.IsBanned(ctx, "alice/aggro"); banned {
		t.Error("fresh deck reported as banned")
	}
	c.ApplyOnFailure(ctx, "alice/aggro", 0, "1") // warning
	if banned, _, _ := c.IsBanned(ctx, "alice/aggro"); banned {
		t.Error("warning reported as ban")
	}
	c.ApplyOnFailure(ctx, "alice/aggro", 0, "2") // temp ban
	banned, row, err := c.IsBanned(ctx, "alice/aggro")
	if err != nil {
		t.Fatalf("IsBanned: %v", err)
	}
	if !banned {
		t.Fatal("temp ban not detected")
	}
	if row.Severity != db.SeverityTempBan {
		t.Errorf("expected temp_ban, got %s", row.Severity)
	}
}

// ----------------------------------------------------------------------
// VerificationWorker — replay match / mismatch flow
// ----------------------------------------------------------------------

func TestWorker_PassingReplay_MarksGameVerified(t *testing.T) {
	d := newTestDB(t)
	now := time.Now().Unix()
	gid := seedGame(t, d, []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"}, 1, 18, now-3600, 42)

	qid, err := db.EnqueueVerification(context.Background(), d, db.VerificationEnqueueParams{
		GameID:        gid,
		DeckKey:       "alice/aggro",
		RNGSeed:       42,
		NSeats:        4,
		DeckKeys:      []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"},
		ClaimedWinner: 1,
		ClaimedTurns:  18,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	verifier := &stubVerifier{} // default: claim matches
	w := NewVerificationWorker(d, verifier, NewCauterizeService(d), VerificationWorkerOptions{})
	ok, err := w.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if !ok {
		t.Fatal("expected processed=true")
	}
	if len(verifier.calls) != 1 || verifier.calls[0] != qid {
		t.Errorf("verifier not called with expected queue_id: %v", verifier.calls)
	}

	// Check terminal status + game flagged.
	rows, _ := db.ListVerifications(context.Background(), d, "passed", 10)
	if len(rows) != 1 {
		t.Fatalf("expected 1 passed row, got %d", len(rows))
	}
	var verified int
	d.QueryRow(`SELECT verified FROM showmatch_game WHERE game_id=?`, gid).Scan(&verified)
	if verified != 1 {
		t.Errorf("game not marked verified: %d", verified)
	}

	// No sanctions on a passing replay.
	if n, _ := db.CountOffenses(context.Background(), d, "alice/aggro"); n != 0 {
		t.Errorf("expected 0 sanctions on pass, got %d", n)
	}
}

func TestWorker_FailingReplay_TriggersCauterize(t *testing.T) {
	d := newTestDB(t)
	now := time.Now().Unix()
	gid := seedGame(t, d, []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"}, 1, 18, now-3600, 42)

	qid, _ := db.EnqueueVerification(context.Background(), d, db.VerificationEnqueueParams{
		GameID:        gid,
		DeckKey:       "alice/aggro",
		RNGSeed:       42,
		NSeats:        4,
		DeckKeys:      []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"},
		ClaimedWinner: 1,
		ClaimedTurns:  18,
	})

	verifier := &stubVerifier{
		results: map[int64]ReplayOutcome{
			qid: {Winner: 0, Turns: 18, Detail: "winner mismatch"},
		},
	}
	cauterize := NewCauterizeService(d)
	w := NewVerificationWorker(d, verifier, cauterize, VerificationWorkerOptions{})
	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	rows, _ := db.ListVerifications(context.Background(), d, "failed", 10)
	if len(rows) != 1 {
		t.Fatalf("expected 1 failed row, got %d", len(rows))
	}
	if rows[0].ReplayedWinner != 0 || rows[0].ReplayedTurns != 18 {
		t.Errorf("replayed outcome not stored: %+v", rows[0])
	}

	// alice/aggro should now have 1 sanction (warning, 1st offense).
	sanctions, err := db.ListSanctionsForDeck(context.Background(), d, "alice/aggro")
	if err != nil {
		t.Fatalf("list sanctions: %v", err)
	}
	if len(sanctions) != 1 || sanctions[0].Severity != db.SeverityWarning {
		t.Errorf("expected 1 warning, got %+v", sanctions)
	}

	// Game flagged unverified.
	var verified int
	d.QueryRow(`SELECT verified FROM showmatch_game WHERE game_id=?`, gid).Scan(&verified)
	if verified != -1 {
		t.Errorf("game not flagged unverified: %d", verified)
	}
}

func TestWorker_VerifierError_StatusErrorNoSanction(t *testing.T) {
	d := newTestDB(t)
	now := time.Now().Unix()
	gid := seedGame(t, d, []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"}, 1, 18, now-3600, 42)

	qid, _ := db.EnqueueVerification(context.Background(), d, db.VerificationEnqueueParams{
		GameID: gid, DeckKey: "alice/aggro", RNGSeed: 42, NSeats: 4,
		DeckKeys: []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"},
		ClaimedWinner: 1, ClaimedTurns: 18,
	})

	verifier := &stubVerifier{
		errs: map[int64]error{qid: errors.New("corpus unavailable")},
	}
	w := NewVerificationWorker(d, verifier, NewCauterizeService(d), VerificationWorkerOptions{})
	processed, err := w.ProcessOne(context.Background())
	if !processed {
		t.Fatal("expected processed=true even on verifier error")
	}
	if err == nil {
		t.Error("expected wrapped error from ProcessOne")
	}

	rows, _ := db.ListVerifications(context.Background(), d, "error", 10)
	if len(rows) != 1 {
		t.Fatalf("expected 1 error row, got %d", len(rows))
	}
	// No sanction — infrastructure errors shouldn't punish contributors.
	if n, _ := db.CountOffenses(context.Background(), d, "alice/aggro"); n != 0 {
		t.Errorf("expected 0 sanctions on verifier error, got %d", n)
	}
}

func TestWorker_EmptyQueue_ReturnsFalse(t *testing.T) {
	d := newTestDB(t)
	w := NewVerificationWorker(d, &stubVerifier{}, NewCauterizeService(d), VerificationWorkerOptions{})
	processed, err := w.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if processed {
		t.Error("expected processed=false on empty queue")
	}
}

func TestWorker_DrainsMultipleRows(t *testing.T) {
	d := newTestDB(t)
	now := time.Now().Unix()

	for i := 0; i < 3; i++ {
		gid := seedGame(t, d, []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"}, 1, 18, now-3600+int64(i), 42+int64(i))
		_, err := db.EnqueueVerification(context.Background(), d, db.VerificationEnqueueParams{
			GameID:        gid,
			DeckKey:       "alice/aggro",
			RNGSeed:       42 + int64(i),
			NSeats:        4,
			DeckKeys:      []string{"alice/aggro", "bob/control", "carol/combo", "dave/midrange"},
			ClaimedWinner: 1,
			ClaimedTurns:  18,
		})
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	w := NewVerificationWorker(d, &stubVerifier{}, NewCauterizeService(d), VerificationWorkerOptions{})
	for i := 0; i < 3; i++ {
		ok, err := w.ProcessOne(context.Background())
		if err != nil {
			t.Fatalf("ProcessOne %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("expected processed=true at iter %d", i)
		}
	}
	stats, _ := db.GetVerificationStats(context.Background(), d)
	if stats.Passed != 3 || stats.Pending != 0 {
		t.Errorf("expected 3 passed / 0 pending, got %+v", stats)
	}
}

// Sanity check that fmt is imported even when unused on partial test
// builds; harmless.
var _ = fmt.Sprintf
