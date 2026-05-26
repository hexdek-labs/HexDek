package userprofile

import (
	"context"
	"testing"
)

// errors.Is(sql.ErrNoRows) migration — sentinel coverage for
// GetCountry. userprofile.go:70 was the migrated site. Companion
// CI guard: internal/lint/errors_is_sql_errnorows_test.go.

func TestGetCountry_UnknownOwnerReturnsEmpty(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	code, err := GetCountry(ctx, db, "no_such_owner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Errorf("expected empty country for missing owner, got %q", code)
	}
}

// Happy-path companion: a known owner still returns the stored code
// post-migration. Defends against the migration accidentally
// short-circuiting the success path.
func TestGetCountry_KnownOwnerReturnsStoredCode(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := SetCountry(ctx, db, "claire", "FR"); err != nil {
		t.Fatalf("SetCountry: %v", err)
	}
	code, err := GetCountry(ctx, db, "claire")
	if err != nil {
		t.Fatalf("GetCountry: %v", err)
	}
	if code != "FR" {
		t.Errorf("expected FR, got %q", code)
	}
}

// Empty-owner short-circuit still bypasses the DB query (no row
// lookup, no errors.Is invocation). The migration did not change
// this early-return branch.
func TestGetCountry_EmptyOwnerReturnsEmpty(t *testing.T) {
	db := openTestDB(t)
	code, err := GetCountry(context.Background(), db, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Errorf("expected empty for empty owner, got %q", code)
	}
}
