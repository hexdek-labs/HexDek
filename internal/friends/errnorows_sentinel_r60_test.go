package friends

import (
	"context"
	"testing"
)

// errors.Is(sql.ErrNoRows) migration — sentinel coverage for
// Tracker.AreFriends. friends.go:133 was the migrated site. Companion
// CI guard: internal/lint/errors_is_sql_errnorows_test.go.

func TestAreFriends_UnrelatedPairReturnsFalse(t *testing.T) {
	tr := newTestTracker(t)
	ctx := context.Background()

	ok, err := tr.AreFriends(ctx, "alice_unrelated", "bob_unrelated")
	if err != nil {
		t.Fatalf("unexpected error for unrelated pair: %v", err)
	}
	if ok {
		t.Errorf("expected AreFriends=false for an unrelated pair, got true")
	}
}

// Mixed-case + whitespace inputs still resolve through the
// normalizePair lowercase/trim path. The sentinel branch must fire
// only for the canonical normalized form not being in the table —
// guard against the migration accidentally short-circuiting on raw
// input.
func TestAreFriends_NormalizationStillBypassed(t *testing.T) {
	tr := newTestTracker(t)
	ctx := context.Background()

	if err := tr.AddFriend(ctx, "alice", "bob"); err != nil {
		t.Fatalf("AddFriend: %v", err)
	}
	// Mixed-case lookup should hit the normalized row.
	ok, err := tr.AreFriends(ctx, "  ALICE ", "Bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Errorf("expected mixed-case lookup to find normalized friendship")
	}
}
