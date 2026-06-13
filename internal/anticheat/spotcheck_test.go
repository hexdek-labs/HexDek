package anticheat

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/heimdall"

	_ "modernc.org/sqlite"
)

// newSpotCheckTestDB opens a file-backed SQLite with the schema
// pieces this orchestrator needs (showmatch_game + verification_queue
// + contributor_sanctions). Mirrors the production DSN so locking
// behaviour matches.
func newSpotCheckTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "anticheat.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// Minimal subset of schema.sql we need. Kept here rather than
	// driving the embedded schema.sql to keep the test isolated from
	// unrelated table churn.
	stmts := []string{
		`CREATE TABLE showmatch_game (
		    game_id      INTEGER PRIMARY KEY AUTOINCREMENT,
		    started_at   INTEGER NOT NULL,
		    finished_at  INTEGER NOT NULL,
		    turns        INTEGER NOT NULL,
		    winner       INTEGER NOT NULL DEFAULT -1,
		    winner_name  TEXT NOT NULL DEFAULT 'DRAW',
		    end_reason   TEXT NOT NULL DEFAULT 'unknown',
		    rng_seed     INTEGER NOT NULL DEFAULT 0,
		    verified     INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE showmatch_game_seat (
		    game_id      INTEGER NOT NULL,
		    seat         INTEGER NOT NULL,
		    commander    TEXT NOT NULL,
		    deck_key     TEXT NOT NULL DEFAULT '',
		    life         INTEGER NOT NULL DEFAULT 0,
		    hand_size    INTEGER NOT NULL DEFAULT 0,
		    library_size INTEGER NOT NULL DEFAULT 0,
		    gy_size      INTEGER NOT NULL DEFAULT 0,
		    bf_size      INTEGER NOT NULL DEFAULT 0,
		    lost         INTEGER NOT NULL DEFAULT 0,
		    battlefield_cards TEXT NOT NULL DEFAULT '[]',
		    PRIMARY KEY (game_id, seat)
		)`,
		`CREATE TABLE verification_queue (
		    queue_id         INTEGER PRIMARY KEY AUTOINCREMENT,
		    game_id          INTEGER NOT NULL,
		    deck_key         TEXT NOT NULL,
		    enqueued_at      INTEGER NOT NULL,
		    status           TEXT NOT NULL DEFAULT 'pending',
		    started_at       INTEGER,
		    finished_at      INTEGER,
		    detail           TEXT NOT NULL DEFAULT '',
		    rng_seed         INTEGER NOT NULL DEFAULT 0,
		    n_seats          INTEGER NOT NULL DEFAULT 0,
		    deck_keys_json   TEXT NOT NULL DEFAULT '[]',
		    claimed_winner   INTEGER NOT NULL DEFAULT -1,
		    claimed_turns    INTEGER NOT NULL DEFAULT 0,
		    replayed_winner  INTEGER NOT NULL DEFAULT -1,
		    replayed_turns   INTEGER NOT NULL DEFAULT 0
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return db
}

// insertFakeGame inserts a finished showmatch_game row plus per-seat
// rows so the scheduler scan + SelectAndEnqueue have something to
// chew on. Returns the new game_id.
func insertFakeGame(t *testing.T, db *sql.DB, finishedAt int64, deckKeys []string) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, end_reason, rng_seed)
		 VALUES (?, ?, ?, ?, 'last_seat_standing', ?)`,
		finishedAt-300, finishedAt, 12, 0, finishedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	gid, _ := res.LastInsertId()
	for i, dk := range deckKeys {
		if _, err := db.Exec(
			`INSERT INTO showmatch_game_seat (game_id, seat, commander, deck_key, life)
			 VALUES (?, ?, ?, ?, ?)`,
			gid, i, "Cmdr "+dk, dk, 40,
		); err != nil {
			t.Fatal(err)
		}
	}
	return gid
}

// TestSchedulerTickEnqueuesCandidates drives the scheduler tick
// directly (without the goroutine) and asserts that an unsampled
// finished game becomes a verification_queue row.
//
// SampleRate is forced to MaxSamplingRate to avoid the test
// flake where the Bernoulli roll skips the only candidate.
func TestSchedulerTickEnqueuesCandidates(t *testing.T) {
	db := newSpotCheckTestDB(t)
	now := time.Now().Unix()
	// Insert enough games that the per-game Bernoulli at MaxSamplingRate
	// (10%) reliably fires on at least one. P(zero picks | n=60, p=0.1)
	// ≈ 0.18%; with seed=1 the deterministic stream picks several.
	for i := 0; i < 60; i++ {
		insertFakeGame(t, db, now-int64(i), []string{"alice/k", "bob/k", "carol/k", "dave/k"})
	}

	// 16-byte minimum key.
	cfg := Config{
		SampleRate:        MaxSamplingRate,
		ContractKey:       []byte("0123456789abcdef"),
		LookbackWindow:    time.Hour,
		SchedulerInterval: time.Hour,        // we drive ticks manually
		VerifierPollEvery: time.Hour,        // worker won't fire
		RngSeed:           1,                 // deterministic
	}
	// We don't need a real ReplayContext for the scheduler tick test.
	// NewService rejects nil rc, so build a stub one. The verifier
	// won't be invoked because we never start the service goroutines.
	rc := &heimdall.ReplayContext{}
	svc, err := NewService(db, rc, cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	svc.runSchedulerTick(context.Background())

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM verification_queue WHERE status='pending'`).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	// One game × 4 deck keys = up to 4 queue rows (scheduler enqueues
	// per seat). Bernoulli'd at MaxSamplingRate with seed=1, all 4
	// candidates are picked (the rate is 10% but the test uses a
	// single game so the per-game roll either fires once or not at
	// all; with deterministic seed=1 it fires).
	if n == 0 {
		t.Fatalf("expected at least 1 enqueued row, got 0")
	}
}

// TestSchedulerTickSkipsAlreadyEnqueued asserts the second tick
// doesn't double-enqueue games already in verification_queue.
func TestSchedulerTickSkipsAlreadyEnqueued(t *testing.T) {
	db := newSpotCheckTestDB(t)
	now := time.Now().Unix()
	for i := 0; i < 60; i++ {
		insertFakeGame(t, db, now-int64(i), []string{"alice/k", "bob/k", "carol/k", "dave/k"})
	}

	cfg := Config{
		SampleRate:        MaxSamplingRate,
		ContractKey:       []byte("0123456789abcdef"),
		LookbackWindow:    time.Hour,
		SchedulerInterval: time.Hour,
		VerifierPollEvery: time.Hour,
		RngSeed:           1,
	}
	svc, err := NewService(db, &heimdall.ReplayContext{}, cfg)
	if err != nil {
		t.Fatal(err)
	}

	svc.runSchedulerTick(context.Background())
	var first int
	db.QueryRow(`SELECT COUNT(*) FROM verification_queue`).Scan(&first)

	svc.runSchedulerTick(context.Background())
	var second int
	db.QueryRow(`SELECT COUNT(*) FROM verification_queue`).Scan(&second)

	if second != first {
		t.Errorf("second tick added rows: first=%d second=%d", first, second)
	}
}

// TestSchedulerTickIgnoresVerifiedGames asserts that already-verified
// rows (verified=1) are not re-enqueued.
func TestSchedulerTickIgnoresVerifiedGames(t *testing.T) {
	db := newSpotCheckTestDB(t)
	now := time.Now().Unix()
	gid := insertFakeGame(t, db, now, []string{"alice/k", "bob/k"})
	if _, err := db.Exec(`UPDATE showmatch_game SET verified=1 WHERE game_id=?`, gid); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		SampleRate:        MaxSamplingRate,
		ContractKey:       []byte("0123456789abcdef"),
		LookbackWindow:    time.Hour,
		SchedulerInterval: time.Hour,
		VerifierPollEvery: time.Hour,
	}
	svc, err := NewService(db, &heimdall.ReplayContext{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc.runSchedulerTick(context.Background())

	var n int
	db.QueryRow(`SELECT COUNT(*) FROM verification_queue`).Scan(&n)
	if n != 0 {
		t.Errorf("verified game should be skipped, got %d rows", n)
	}
}

// TestNewServiceRejectsBadConfig pins the constructor's input
// validation: nil DB, nil rc, short key.
func TestNewServiceRejectsBadConfig(t *testing.T) {
	db := newSpotCheckTestDB(t)
	rc := &heimdall.ReplayContext{}
	good := Config{ContractKey: []byte("0123456789abcdef")}

	if _, err := NewService(nil, rc, good); err == nil {
		t.Error("expected error for nil DB")
	}
	if _, err := NewService(db, nil, good); err == nil {
		t.Error("expected error for nil rc")
	}
	short := Config{ContractKey: []byte("short")}
	if _, err := NewService(db, rc, short); err == nil {
		t.Error("expected error for <16 byte key")
	}
}

// TestNewServiceForwardsMaxTurns pins the r63 anticheat residual C-H2 #1
// wiring: a configured MaxTurns lands on rc.MaxTurns (so the verifier
// caps replays identically to the live games), while the zero value
// leaves rc.MaxTurns untouched (the replay default).
func TestNewServiceForwardsMaxTurns(t *testing.T) {
	db := newSpotCheckTestDB(t)
	base := Config{ContractKey: []byte("0123456789abcdef")}

	rc := &heimdall.ReplayContext{}
	cfg := base
	cfg.MaxTurns = 33
	if _, err := NewService(db, rc, cfg); err != nil {
		t.Fatal(err)
	}
	if rc.MaxTurns != 33 {
		t.Fatalf("cfg.MaxTurns not forwarded onto rc: got %d want 33", rc.MaxTurns)
	}

	// Zero MaxTurns must not clobber a cap a caller set on rc directly.
	rc2 := &heimdall.ReplayContext{MaxTurns: 50}
	if _, err := NewService(db, rc2, base); err != nil {
		t.Fatal(err)
	}
	if rc2.MaxTurns != 50 {
		t.Fatalf("zero cfg.MaxTurns clobbered a caller-set rc.MaxTurns: got %d want 50", rc2.MaxTurns)
	}
}

// TestServiceStartStopIdempotent confirms double-Start is a no-op
// and Stop before Start doesn't panic.
func TestServiceStartStopIdempotent(t *testing.T) {
	db := newSpotCheckTestDB(t)
	cfg := Config{
		ContractKey:       []byte("0123456789abcdef"),
		SchedulerInterval: 100 * time.Millisecond,
		VerifierPollEvery: 100 * time.Millisecond,
		LookbackWindow:    time.Hour,
	}
	svc, err := NewService(db, &heimdall.ReplayContext{}, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Stop before Start — no-op.
	svc.Stop()

	ctx := context.Background()
	svc.Start(ctx)
	svc.Start(ctx) // second call should be no-op

	// Let the goroutines breathe at least one tick so any panic
	// path has a chance to surface.
	time.Sleep(150 * time.Millisecond)
	svc.Stop()
}
