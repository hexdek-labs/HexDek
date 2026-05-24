package auth

import (
	"context"
	"database/sql"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// r60 — pin the session-lifecycle additions: RevokeAllForDevice
// (multi-tab logout primitive) and ExpireSessions (periodic reaper
// for past-expiry rows). Both are pure additions; existing callers of
// RevokeSession / ValidateSession / IssueSession are unaffected.

// openTestDB returns an in-memory SQLite with the full hexdek schema
// applied, and registers a test cleanup that closes it.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// seedDevice inserts a row in the device table so session inserts
// satisfy the device_id foreign key. Returns the device id.
func seedDevice(t *testing.T, d *sql.DB, id, displayName string) string {
	t.Helper()
	now := db.Now()
	if _, err := d.ExecContext(context.Background(),
		`INSERT INTO device (id, display_name, created_at, last_seen_at) VALUES (?, ?, ?, ?)`,
		id, displayName, now, now); err != nil {
		t.Fatalf("seed device %q: %v", id, err)
	}
	return id
}

// countSessions counts rows in the session table for assertions.
// optional deviceID filter; pass "" for all.
func countSessions(t *testing.T, d *sql.DB, deviceID string) int {
	t.Helper()
	var n int
	var err error
	if deviceID == "" {
		err = d.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM session`).Scan(&n)
	} else {
		err = d.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM session WHERE device_id = ?`, deviceID).Scan(&n)
	}
	if err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return n
}

// TestRevokeAllForDevice covers the "log out everywhere on this
// device" primitive: a single call must remove every session row for
// the target device, leave other devices untouched, and report the
// number removed for the audit trail. The empty-deviceID guard is
// exercised separately so a typo in caller code can't accidentally
// wipe the whole session table.
func TestRevokeAllForDevice(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()

	alice := seedDevice(t, d, "device_alice", "Alice's iPhone")
	bob := seedDevice(t, d, "device_bob", "Bob's laptop")

	// Issue three sessions for alice (matches the multi-tab scenario:
	// each tab triggers a fresh /login -> session row) and one for bob
	// (control — must not be touched by alice's logout-everywhere).
	for i := 0; i < 3; i++ {
		if _, err := IssueSession(ctx, d, alice, 0); err != nil {
			t.Fatalf("issue alice #%d: %v", i, err)
		}
	}
	if _, err := IssueSession(ctx, d, bob, 0); err != nil {
		t.Fatalf("issue bob: %v", err)
	}

	if got := countSessions(t, d, alice); got != 3 {
		t.Fatalf("baseline alice sessions: want 3, got %d", got)
	}
	if got := countSessions(t, d, bob); got != 1 {
		t.Fatalf("baseline bob sessions: want 1, got %d", got)
	}

	n, err := RevokeAllForDevice(ctx, d, alice)
	if err != nil {
		t.Fatalf("RevokeAllForDevice: %v", err)
	}
	if n != 3 {
		t.Errorf("revoked count: want 3, got %d", n)
	}
	if got := countSessions(t, d, alice); got != 0 {
		t.Errorf("alice sessions after revoke: want 0, got %d", got)
	}
	if got := countSessions(t, d, bob); got != 1 {
		t.Errorf("bob sessions after alice revoke: want 1, got %d", got)
	}
}

// TestRevokeAllForDevice_EmptyDeviceRejected pins the safety guard
// that prevents a typo / nil-string bug from blowing away every
// session in the table.
func TestRevokeAllForDevice_EmptyDeviceRejected(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	alice := seedDevice(t, d, "device_alice", "Alice")
	if _, err := IssueSession(ctx, d, alice, 0); err != nil {
		t.Fatalf("issue: %v", err)
	}

	n, err := RevokeAllForDevice(ctx, d, "")
	if err == nil {
		t.Fatal("RevokeAllForDevice(\"\") must return an error so a typo can't mass-revoke")
	}
	if n != 0 {
		t.Errorf("revoked count: want 0, got %d", n)
	}
	if got := countSessions(t, d, ""); got != 1 {
		t.Errorf("session table should be untouched, got %d rows", got)
	}
}

// TestRevokeAllForDevice_NoSessions returns 0 cleanly when the
// device has no active sessions (idempotent — calling it twice in a
// row, or against a device that's already logged out, must not
// error). This makes it safe to wire into a "logout" handler that
// fires on every page unload without caring whether sessions
// actually existed.
func TestRevokeAllForDevice_NoSessions(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	alice := seedDevice(t, d, "device_alice", "Alice")

	n, err := RevokeAllForDevice(ctx, d, alice)
	if err != nil {
		t.Fatalf("RevokeAllForDevice on empty session set: %v", err)
	}
	if n != 0 {
		t.Errorf("revoked count: want 0, got %d", n)
	}
}

// TestExpireSessions covers the periodic reaper: rows whose
// expires_at is in the past (and non-zero) must be deleted; rows
// with expires_at=0 (never expire) and rows whose expires_at is in
// the future stay. Returns the number reaped.
func TestExpireSessions(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	alice := seedDevice(t, d, "device_alice", "Alice")

	// Hand-build session rows with controlled expires_at so we don't
	// depend on time-mocking IssueSession.
	now := db.Now()
	rows := []struct {
		token     string
		expiresAt int64
		wantReap  bool
	}{
		{token: "tok_long_expired", expiresAt: now - 3600, wantReap: true},     // 1h ago
		{token: "tok_just_expired", expiresAt: now - 1, wantReap: true},        // 1s ago
		{token: "tok_at_now", expiresAt: now, wantReap: false},                  // boundary: not <
		{token: "tok_future", expiresAt: now + 3600, wantReap: false},          // 1h future
		{token: "tok_never_expires", expiresAt: 0, wantReap: false},            // convention: 0 = never
	}
	for _, r := range rows {
		if _, err := d.ExecContext(ctx,
			`INSERT INTO session (token, device_id, created_at, expires_at, last_used_at)
			 VALUES (?, ?, ?, ?, ?)`,
			r.token, alice, now-7200, r.expiresAt, now); err != nil {
			t.Fatalf("seed %q: %v", r.token, err)
		}
	}
	if got := countSessions(t, d, alice); got != len(rows) {
		t.Fatalf("baseline session count: want %d, got %d", len(rows), got)
	}

	n, err := ExpireSessions(ctx, d)
	if err != nil {
		t.Fatalf("ExpireSessions: %v", err)
	}

	wantReaped := int64(0)
	for _, r := range rows {
		if r.wantReap {
			wantReaped++
		}
	}
	if n != wantReaped {
		t.Errorf("reaped count: want %d, got %d", wantReaped, n)
	}

	// Verify only the should-stay rows remain.
	for _, r := range rows {
		var exists int
		if err := d.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM session WHERE token = ?`, r.token).Scan(&exists); err != nil {
			t.Fatalf("check %q: %v", r.token, err)
		}
		if r.wantReap && exists != 0 {
			t.Errorf("%q: should have been reaped, still present", r.token)
		}
		if !r.wantReap && exists != 1 {
			t.Errorf("%q: should have survived, missing", r.token)
		}
	}
}

// TestExpireSessions_EmptyTable returns 0 cleanly with no rows —
// safe to call from a periodic background goroutine that fires
// before any session has been issued.
func TestExpireSessions_EmptyTable(t *testing.T) {
	d := openTestDB(t)
	n, err := ExpireSessions(context.Background(), d)
	if err != nil {
		t.Fatalf("ExpireSessions on empty table: %v", err)
	}
	if n != 0 {
		t.Errorf("reaped count: want 0, got %d", n)
	}
}

// TestExpireSessions_OnlyExpiredRowsDeleted_ValidateSessionStillWorks
// is the cross-API integration check: after a reap pass, valid
// non-expired tokens must still authenticate via ValidateSession.
func TestExpireSessions_OnlyExpiredRowsDeleted_ValidateSessionStillWorks(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	alice := seedDevice(t, d, "device_alice", "Alice")

	// Issue one long-lived session (60s expiry).
	live, err := IssueSession(ctx, d, alice, 60)
	if err != nil {
		t.Fatalf("issue live: %v", err)
	}
	// Manually plant an already-expired row.
	now := db.Now()
	if _, err := d.ExecContext(ctx,
		`INSERT INTO session (token, device_id, created_at, expires_at, last_used_at)
		 VALUES (?, ?, ?, ?, ?)`,
		"tok_dead", alice, now-7200, now-3600, now-3600); err != nil {
		t.Fatalf("seed expired: %v", err)
	}

	n, err := ExpireSessions(ctx, d)
	if err != nil {
		t.Fatalf("ExpireSessions: %v", err)
	}
	if n != 1 {
		t.Errorf("reaped count: want 1, got %d", n)
	}

	// Live token must still validate.
	if _, err := ValidateSession(ctx, d, live.Token); err != nil {
		t.Errorf("live token broken by ExpireSessions: %v", err)
	}
	// Dead token must now be "invalid" (deleted), not "expired".
	if _, err := ValidateSession(ctx, d, "tok_dead"); err != ErrInvalidToken {
		t.Errorf("dead token after reap: want ErrInvalidToken, got %v", err)
	}
}
