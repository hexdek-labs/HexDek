package auth

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

// session_gc_r60_test.go — pins the long-missing periodic-reaper wiring.
// ExpireSessions and its tests have lived in this package since session
// lifecycle work landed, but no production caller scheduled it; expired
// rows accumulated forever. RunSessionGC is the scheduler the docstring
// always promised.

// insertExpiredSession plants a session row with an already-past
// expires_at so a reap pass should delete it on next call.
func insertExpiredSession(t *testing.T, d *sql.DB, deviceID, token string) {
	t.Helper()
	now := db.Now()
	if _, err := d.ExecContext(context.Background(),
		`INSERT INTO session (token, device_id, created_at, expires_at, last_used_at)
		 VALUES (?, ?, ?, ?, ?)`,
		token, deviceID, now-7200, now-3600, now-3600); err != nil {
		t.Fatalf("seed expired session %q: %v", token, err)
	}
}

// TestRunSessionGC_InitialPassRunsSynchronously is the load-bearing
// guarantee: when callers invoke RunSessionGC before binding the http
// listener, expired rows are cleared BEFORE the function returns so
// no incoming auth request can race against the reaper. (The
// docstring on RunSessionGC commits to this — if we ever switch to a
// fully-async first pass, this test surfaces the change.)
func TestRunSessionGC_InitialPassRunsSynchronously(t *testing.T) {
	d := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	alice := seedDevice(t, d, "device_alice", "Alice")
	insertExpiredSession(t, d, alice, "tok_expired_1")
	insertExpiredSession(t, d, alice, "tok_expired_2")
	if got := countSessions(t, d, alice); got != 2 {
		t.Fatalf("baseline: want 2, got %d", got)
	}

	// 1-hour interval so the goroutine tick can't fire during the test —
	// the deletes we observe MUST come from the synchronous first pass.
	RunSessionGC(ctx, d, time.Hour)

	if got := countSessions(t, d, alice); got != 0 {
		t.Errorf("after RunSessionGC: want 0 sessions, got %d — initial pass should have reaped synchronously", got)
	}
}

// TestRunSessionGC_TickerReapsRowsThatExpireAfterStartup is the
// scheduled-reaper invariant: rows that EXPIRE WHILE the server is
// running (the normal lifecycle) must be reaped on the next tick,
// not just at boot.
func TestRunSessionGC_TickerReapsRowsThatExpireAfterStartup(t *testing.T) {
	d := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	alice := seedDevice(t, d, "device_alice", "Alice")

	// Start the GC with a very short interval. No rows exist yet, so
	// the initial pass is a no-op.
	RunSessionGC(ctx, d, 20*time.Millisecond)

	// Insert an expired row AFTER the GC is running.
	insertExpiredSession(t, d, alice, "tok_post_startup")
	if got := countSessions(t, d, alice); got != 1 {
		t.Fatalf("post-insert baseline: want 1, got %d", got)
	}

	// Wait long enough for the ticker to fire at least twice. Bounded
	// wait via polling so the test doesn't flake on slow CI.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if countSessions(t, d, alice) == 0 {
			return // success
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Errorf("ticker did not reap post-startup expired row within 2s — RunSessionGC goroutine appears stuck")
}

// TestRunSessionGC_RespectsContextCancellation pins the shutdown
// contract: once ctx is cancelled, the GC goroutine MUST stop firing
// against the database. We verify by cancelling, then planting a new
// expired row, then proving it survives well past the would-be-fire
// window.
func TestRunSessionGC_RespectsContextCancellation(t *testing.T) {
	d := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	alice := seedDevice(t, d, "device_alice", "Alice")

	RunSessionGC(ctx, d, 15*time.Millisecond)

	// Let the goroutine run a few ticks to prove it's alive.
	time.Sleep(50 * time.Millisecond)

	// Cancel and wait briefly so any in-flight tick can complete.
	cancel()
	time.Sleep(30 * time.Millisecond)

	// Plant an expired row AFTER cancel. If the goroutine respected
	// ctx, this row stays put.
	insertExpiredSession(t, d, alice, "tok_after_cancel")
	// Wait 5x the original interval — ample time for a ghost tick if
	// the goroutine leaked.
	time.Sleep(80 * time.Millisecond)
	if got := countSessions(t, d, alice); got != 1 {
		t.Errorf("after ctx cancel + post-cancel insert: want 1 session (cancelled goroutine should not reap), got %d", got)
	}
}

// TestRunSessionGC_LeavesNonExpiredRowsAlone is the integration
// safety net: the scheduler must NEVER delete unexpired rows. Pairs
// with the (already-passing) ExpireSessions unit tests but exercises
// the scheduled invocation path.
func TestRunSessionGC_LeavesNonExpiredRowsAlone(t *testing.T) {
	d := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	alice := seedDevice(t, d, "device_alice", "Alice")

	// Mix of rows: one expired, one future-expiring, one never-expires.
	insertExpiredSession(t, d, alice, "tok_dead")
	live, err := IssueSession(context.Background(), d, alice, 3600) // 1h
	if err != nil {
		t.Fatalf("issue live: %v", err)
	}
	immortal, err := IssueSession(context.Background(), d, alice, 0) // never
	if err != nil {
		t.Fatalf("issue immortal: %v", err)
	}

	RunSessionGC(ctx, d, time.Hour) // initial pass only

	// Survivors must still authenticate.
	if _, err := ValidateSession(context.Background(), d, live.Token); err != nil {
		t.Errorf("live session broken by GC: %v", err)
	}
	if _, err := ValidateSession(context.Background(), d, immortal.Token); err != nil {
		t.Errorf("immortal session broken by GC: %v", err)
	}
	// Dead row must be gone.
	if _, err := ValidateSession(context.Background(), d, "tok_dead"); err != ErrInvalidToken {
		t.Errorf("dead token after GC: want ErrInvalidToken, got %v", err)
	}
}

// TestRunSessionGC_DefaultIntervalWhenZeroOrNegative pins the
// safety-default that the API documents — passing 0 or a negative
// interval falls back to 1h rather than spinning a hot CPU loop.
// We can't observe the actual ticker rate without an injected clock,
// but we CAN observe that the goroutine doesn't immediately exit
// (it would race past the synchronous initial pass and then no rows
// would ever be reaped after that). Verify by inserting an expired
// row AFTER startup and confirming it's NOT reaped within 1s — i.e.
// the interval is much larger than the test window.
func TestRunSessionGC_DefaultIntervalWhenZeroOrNegative(t *testing.T) {
	d := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	alice := seedDevice(t, d, "device_alice", "Alice")

	RunSessionGC(ctx, d, 0) // initial pass runs; ticker should be 1h default
	insertExpiredSession(t, d, alice, "tok_post_start")
	// Wait 200ms. With the default 1h interval, the ticker doesn't
	// fire, so the row stays. (If the bad-input guard had been left
	// out, interval=0 would either crash time.NewTicker or spin
	// infinitely fast, deleting the row instantly.)
	time.Sleep(200 * time.Millisecond)
	if got := countSessions(t, d, alice); got != 1 {
		t.Errorf("0-interval input should have fallen back to 1h default; got %d sessions (want 1)", got)
	}
}
