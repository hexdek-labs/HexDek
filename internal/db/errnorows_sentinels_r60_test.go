package db

import (
	"context"
	"testing"
)

// -----------------------------------------------------------------------------
// errors.Is(sql.ErrNoRows) migration — sentinel coverage for the 4 db-package
// call sites migrated from `err == sql.ErrNoRows` to errors.Is. The four
// functions each translate sql.ErrNoRows into a "no row" sentinel (nil, "",
// zeros) — these tests pin that behavior against the running sqlite
// schema so any regression in the migration surfaces immediately.
// Companion: internal/lint/errors_is_sql_errnorows_test.go enforces no
// new occurrences of the raw `==` form land in the tree.
// -----------------------------------------------------------------------------

// ClaimNextVerification returns (nil, nil) when the queue is empty.
// anticheat.go:91 was the first migrated site.
func TestClaimNextVerification_EmptyQueueReturnsNil(t *testing.T) {
	d := newAnticheatTestDB(t)
	ctx := context.Background()

	row, err := ClaimNextVerification(ctx, d)
	if err != nil {
		t.Fatalf("unexpected error on empty queue: %v", err)
	}
	if row != nil {
		t.Errorf("expected nil row on empty queue, got %+v", row)
	}
}

// ActiveBan returns (nil, nil) when no sanction exists for the deck key.
// anticheat.go:361 was the second migrated site.
func TestActiveBan_NoSanctionReturnsNil(t *testing.T) {
	d := newAnticheatTestDB(t)
	ctx := context.Background()

	row, err := ActiveBan(ctx, d, "ghost/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error for missing sanction: %v", err)
	}
	if row != nil {
		t.Errorf("expected nil sanction row, got %+v", row)
	}
}

// GetCommanderStats returns (0, 0, nil) for an unknown commander.
// showmatch.go:259 was the third migrated site.
//
// Implementation detail: the SQL uses COALESCE(SUM(...), 0), so an
// unknown commander actually returns 0,0,nil WITHOUT triggering
// sql.ErrNoRows — but the guard is defense-in-depth (e.g. if a future
// schema migration removes the COALESCE, the sentinel branch fires).
// Both pre- and post-migration code returns the same sentinel here;
// the test just confirms the contract.
func TestGetCommanderStats_UnknownCommanderReturnsZero(t *testing.T) {
	d := newAnticheatTestDB(t)
	ctx := context.Background()

	games, wins, err := GetCommanderStats(ctx, d, "Nonexistent, the Unsummoned")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if games != 0 || wins != 0 {
		t.Errorf("expected (0, 0) for unknown commander, got (%d, %d)", games, wins)
	}
}

// KVGet returns ("", nil) for a missing key.
// showmatch.go:298 was the fourth migrated site.
func TestKVGet_MissingKeyReturnsEmpty(t *testing.T) {
	d := newAnticheatTestDB(t)
	ctx := context.Background()

	val, err := KVGet(ctx, d, "no_such_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for missing key, got %q", val)
	}
}

// KVGet still returns the stored value when present — defends against
// the migration accidentally swallowing the "happy path."
func TestKVGet_ExistingKeyReturnsValue(t *testing.T) {
	d := newAnticheatTestDB(t)
	ctx := context.Background()

	if err := KVSet(ctx, d, "greeting", "hello"); err != nil {
		t.Fatalf("KVSet: %v", err)
	}
	val, err := KVGet(ctx, d, "greeting")
	if err != nil {
		t.Fatalf("KVGet: %v", err)
	}
	if val != "hello" {
		t.Errorf("expected %q, got %q", "hello", val)
	}
}
