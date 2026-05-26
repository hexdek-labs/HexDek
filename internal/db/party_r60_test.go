package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	sqlitedrv "modernc.org/sqlite"
	sqliteerr "modernc.org/sqlite/lib"
)

// R60 — closes half-finished-features-r48 #9: the M1 (constraint-vs-
// transport errno distinction) and M3 (RowsAffected swallow) gaps in
// internal/db/party.go.

// -----------------------------------------------------------------------
// M1 — IsSQLitePrimaryKeyConflict introspects driver errno
// -----------------------------------------------------------------------

func TestIsSQLitePrimaryKeyConflict_DetectsPKCollision(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()

	seedDeviceForPartyTest(t, ctx, d, "dev-pk-test")
	// Insert one party explicitly.
	if _, err := d.ExecContext(ctx,
		`INSERT INTO party (id, host_device_id, state, max_players, created_at) VALUES (?, ?, 'lobby', 4, 0)`,
		"FIXEDID", "dev-pk-test"); err != nil {
		t.Fatalf("seed first party: %v", err)
	}
	// Try to insert another row with the same ID — expect a PK
	// constraint violation that IsSQLitePrimaryKeyConflict detects.
	_, err := d.ExecContext(ctx,
		`INSERT INTO party (id, host_device_id, state, max_players, created_at) VALUES (?, ?, 'lobby', 4, 0)`,
		"FIXEDID", "dev-pk-test")
	if err == nil {
		t.Fatal("expected PK collision error, got nil")
	}
	if !IsSQLitePrimaryKeyConflict(err) {
		t.Errorf("IsSQLitePrimaryKeyConflict should return true for PK collision; err=%v", err)
	}
	// Sanity check the broader predicate too.
	if !IsSQLiteConstraint(err) {
		t.Errorf("IsSQLiteConstraint should also return true; err=%v", err)
	}
}

func TestIsSQLitePrimaryKeyConflict_RejectsForeignKeyViolation(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()

	// FK violation — host_device_id="never-existed" doesn't reference
	// a real device row. This is exactly the failure mode the M1 fix
	// distinguishes from a code collision: should NOT be retried.
	_, err := d.ExecContext(ctx,
		`INSERT INTO party (id, host_device_id, state, max_players, created_at) VALUES (?, ?, 'lobby', 4, 0)`,
		"ANOTHER", "never-existed")
	if err == nil {
		t.Fatal("expected FK violation, got nil")
	}
	if IsSQLitePrimaryKeyConflict(err) {
		t.Errorf("IsSQLitePrimaryKeyConflict should return false for FK violation; err=%v", err)
	}
	// IsSQLiteConstraint is true (FK is still a constraint), but the
	// retry-loop only treats PK conflicts as retryable.
	if !IsSQLiteConstraint(err) {
		t.Errorf("IsSQLiteConstraint should be true for FK violation; err=%v", err)
	}
	// Confirm the actual code is FOREIGNKEY = 787 so the test fails
	// loudly if the driver renames the constant.
	var sqliteErr *sqlitedrv.Error
	if !errors.As(err, &sqliteErr) {
		t.Fatalf("expected *sqlite.Error; got %T", err)
	}
	if sqliteErr.Code() != sqliteerr.SQLITE_CONSTRAINT_FOREIGNKEY {
		t.Errorf("expected SQLITE_CONSTRAINT_FOREIGNKEY=%d; got %d",
			sqliteerr.SQLITE_CONSTRAINT_FOREIGNKEY, sqliteErr.Code())
	}
}

func TestIsSQLitePrimaryKeyConflict_NilAndNonSQLite(t *testing.T) {
	if IsSQLitePrimaryKeyConflict(nil) {
		t.Error("nil should return false")
	}
	if IsSQLitePrimaryKeyConflict(errors.New("plain error")) {
		t.Error("non-sqlite error should return false")
	}
	// Wrapped non-sqlite error.
	wrapped := errors.Join(errors.New("outer"), errors.New("inner"))
	if IsSQLitePrimaryKeyConflict(wrapped) {
		t.Error("wrapped non-sqlite error should return false")
	}
}

// -----------------------------------------------------------------------
// M1 — CreateParty bails immediately on non-PK errors
// -----------------------------------------------------------------------

func TestCreateParty_HappyPath(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()
	seedDeviceForPartyTest(t, ctx, d, "host-happy")

	p, err := CreateParty(ctx, d, "host-happy", 4)
	if err != nil {
		t.Fatalf("CreateParty: %v", err)
	}
	if len(p.ID) != 6 {
		t.Errorf("expected 6-char code, got %q (len=%d)", p.ID, len(p.ID))
	}
	if p.HostDeviceID != "host-happy" || p.MaxPlayers != 4 || p.State != "lobby" {
		t.Errorf("unexpected fields: %+v", p)
	}
}

func TestCreateParty_FKViolationBailsImmediately(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()

	// host_device_id="never-existed" → FK violation → should NOT retry,
	// should return the wrapped insert error immediately.
	_, err := CreateParty(ctx, d, "never-existed", 4)
	if err == nil {
		t.Fatal("expected FK error, got nil")
	}
	// Pre-fix behavior: retry-loop burned through 5 attempts then
	// returned "party code generation failed after 5 attempts." The
	// fix surfaces the actual FK error wrapped as "insert party row".
	msg := err.Error()
	if strings.Contains(msg, "code generation failed after") {
		t.Errorf("FK error should not have been retried as if it were a code collision; got %q", msg)
	}
	if !strings.Contains(msg, "insert party row") {
		t.Errorf("error should be wrapped with insert party row context; got %q", msg)
	}
}

func TestCreateParty_MaxPlayersOutOfRange(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()
	seedDeviceForPartyTest(t, ctx, d, "host-range")

	if _, err := CreateParty(ctx, d, "host-range", 1); err == nil {
		t.Error("expected error for maxPlayers=1")
	}
	if _, err := CreateParty(ctx, d, "host-range", 5); err == nil {
		t.Error("expected error for maxPlayers=5")
	}
}

// -----------------------------------------------------------------------
// M3 — SetMemberDeck handles RowsAffected error
// -----------------------------------------------------------------------

func TestSetMemberDeck_HappyPath(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()
	seedDeviceForPartyTest(t, ctx, d, "host-deck")
	seedDeckForPartyTest(t, ctx, d, "deck-target", "host-deck")

	p, err := CreateParty(ctx, d, "host-deck", 4)
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	// host is auto-joined as seat 0 by CreateParty.

	if err := SetMemberDeck(ctx, d, p.ID, "host-deck", "deck-target"); err != nil {
		t.Fatalf("SetMemberDeck: %v", err)
	}

	// Verify the row was updated.
	members, err := ListPartyMembers(ctx, d, p.ID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if !members[0].DeckID.Valid || members[0].DeckID.String != "deck-target" {
		t.Errorf("DeckID not updated; got %+v", members[0].DeckID)
	}
}

func TestSetMemberDeck_NoRowMatched(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()
	ctx := context.Background()
	seedDeviceForPartyTest(t, ctx, d, "host-nomatch")

	// No party + no member exist; UPDATE matches zero rows.
	err := SetMemberDeck(ctx, d, "BOGUSID", "never-joined", "any-deck")
	if err == nil {
		t.Fatal("expected error for missing party_member, got nil")
	}
	if !strings.Contains(err.Error(), "no party_member matched") {
		t.Errorf("expected 'no party_member matched' in error; got %q", err.Error())
	}
}

// -----------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	return d
}

func seedDeviceForPartyTest(t *testing.T, ctx context.Context, d *sql.DB, deviceID string) {
	t.Helper()
	if _, err := d.ExecContext(ctx,
		`INSERT INTO device (id, display_name, created_at, last_seen_at) VALUES (?, ?, 0, 0)`,
		deviceID, deviceID); err != nil {
		t.Fatalf("seed device: %v", err)
	}
}

func seedDeckForPartyTest(t *testing.T, ctx context.Context, d *sql.DB, deckID, ownerDeviceID string) {
	t.Helper()
	if _, err := d.ExecContext(ctx,
		`INSERT INTO deck (id, owner_device_id, name, commander_name, format, imported_at, raw_json)
		 VALUES (?, ?, 'r60 party test deck', 'Test Commander', 'commander', 0, '{}')`,
		deckID, ownerDeviceID); err != nil {
		t.Fatalf("seed deck: %v", err)
	}
}
