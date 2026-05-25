package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

// openMemDB returns a fresh in-memory SQLite with webhooks schema
// applied. Each test gets its own DB so concurrent runs don't share
// state.
func openMemDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := EnsureWebhooksSchema(context.Background(), d); err != nil {
		t.Fatalf("EnsureWebhooksSchema: %v", err)
	}
	return d
}

func TestEnsureWebhooksSchema_NilDB(t *testing.T) {
	if err := EnsureWebhooksSchema(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestEnsureWebhooksSchema_Idempotent(t *testing.T) {
	d := openMemDB(t)
	// Second invocation must not error.
	if err := EnsureWebhooksSchema(context.Background(), d); err != nil {
		t.Fatalf("re-apply schema: %v", err)
	}
}

func TestInsertAndGetWebhook(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()

	id, err := InsertWebhook(ctx, d, "alice", "https://example.com/hook", "game.end", "secret-abc")
	if err != nil {
		t.Fatalf("InsertWebhook: %v", err)
	}
	if id <= 0 {
		t.Fatalf("InsertWebhook: want id > 0, got %d", id)
	}
	got, err := GetWebhook(ctx, d, id)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if got.Owner != "alice" || got.URL != "https://example.com/hook" ||
		got.EventType != "game.end" || got.Secret != "secret-abc" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if !got.Active {
		t.Errorf("new webhook should be active")
	}
	if got.Failures != 0 || got.LastStatus != 0 {
		t.Errorf("new webhook should have zero failures/last_status: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("created_at should be set")
	}
}

func TestInsertWebhook_RejectsEmptyFields(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()
	cases := []struct {
		name                              string
		owner, url, eventType, secret string
	}{
		{"empty owner", "", "https://e", "game.end", "s"},
		{"empty url", "alice", "", "game.end", "s"},
		{"empty event", "alice", "https://e", "", "s"},
		{"empty secret", "alice", "https://e", "game.end", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if _, err := InsertWebhook(ctx, d, tc.owner, tc.url, tc.eventType, tc.secret); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestGetWebhook_NotFound(t *testing.T) {
	d := openMemDB(t)
	_, err := GetWebhook(context.Background(), d, 9999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}

func TestListWebhooksByOwner(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()
	// Seed three rows across two owners.
	seedWebhook(t, ctx, d, "alice", "https://a/1", "game.end", "s1")
	seedWebhook(t, ctx, d, "bob", "https://b/1", "game.end", "s2")
	seedWebhook(t, ctx, d, "alice", "https://a/2", "game.end", "s3")

	rows, err := ListWebhooksByOwner(ctx, d, "alice")
	if err != nil {
		t.Fatalf("ListWebhooksByOwner: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows for alice, got %d", len(rows))
	}
	// Newest first.
	if rows[0].URL != "https://a/2" {
		t.Errorf("want newest first; got %s", rows[0].URL)
	}

	// Bob's rows shouldn't leak.
	for _, r := range rows {
		if r.Owner != "alice" {
			t.Errorf("got non-alice row: %+v", r)
		}
	}
}

func TestListActiveWebhooksForEvent(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()
	id1 := seedWebhook(t, ctx, d, "alice", "https://a/1", "game.end", "s1")
	seedWebhook(t, ctx, d, "bob", "https://b/1", "other.event", "s2")
	id3 := seedWebhook(t, ctx, d, "carol", "https://c/1", "game.end", "s3")

	// Disable id1 so the active filter must skip it.
	if err := SetWebhookActive(ctx, d, id1, false); err != nil {
		t.Fatalf("SetWebhookActive: %v", err)
	}

	rows, err := ListActiveWebhooksForEvent(ctx, d, "game.end")
	if err != nil {
		t.Fatalf("ListActiveWebhooksForEvent: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 active row for game.end, got %d (%+v)", len(rows), rows)
	}
	if rows[0].ID != id3 {
		t.Errorf("want id3=%d, got %d", id3, rows[0].ID)
	}
}

func TestDeleteWebhookByOwner(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()
	id := seedWebhook(t, ctx, d, "alice", "https://a/1", "game.end", "s")

	// Wrong owner — no delete.
	n, err := DeleteWebhookByOwner(ctx, d, id, "bob")
	if err != nil {
		t.Fatalf("DeleteWebhookByOwner (wrong owner): %v", err)
	}
	if n != 0 {
		t.Errorf("wrong-owner delete affected %d rows, want 0", n)
	}
	// Row still present.
	if _, err := GetWebhook(ctx, d, id); err != nil {
		t.Errorf("row should still exist: %v", err)
	}

	// Correct owner — delete.
	n, err = DeleteWebhookByOwner(ctx, d, id, "alice")
	if err != nil {
		t.Fatalf("DeleteWebhookByOwner: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 affected, got %d", n)
	}

	// Now missing.
	if _, err := GetWebhook(ctx, d, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("want ErrNoRows post-delete, got %v", err)
	}
}

func TestUpdateWebhookHealth_2xxResetsFailures(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()
	id := seedWebhook(t, ctx, d, "alice", "https://a", "game.end", "s")

	// Bump twice.
	mustErr(t, UpdateWebhookHealth(ctx, d, id, 500, 5))
	mustErr(t, UpdateWebhookHealth(ctx, d, id, -1, 5))
	got, _ := GetWebhook(ctx, d, id)
	if got.Failures != 2 {
		t.Fatalf("failures: want 2, got %d", got.Failures)
	}

	// Successful response resets failures and updates last_status.
	mustErr(t, UpdateWebhookHealth(ctx, d, id, 200, 5))
	got, _ = GetWebhook(ctx, d, id)
	if got.Failures != 0 {
		t.Errorf("failures after 200: want 0, got %d", got.Failures)
	}
	if got.LastStatus != 200 {
		t.Errorf("last_status: want 200, got %d", got.LastStatus)
	}
	if !got.Active {
		t.Errorf("webhook should still be active")
	}
}

func TestUpdateWebhookHealth_5xxAutoDisablesAtThreshold(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()
	id := seedWebhook(t, ctx, d, "alice", "https://a", "game.end", "s")

	threshold := 3
	// 1st + 2nd failures: still active.
	mustErr(t, UpdateWebhookHealth(ctx, d, id, 500, threshold))
	mustErr(t, UpdateWebhookHealth(ctx, d, id, 502, threshold))
	got, _ := GetWebhook(ctx, d, id)
	if !got.Active {
		t.Errorf("after 2 failures should still be active (threshold=%d)", threshold)
	}
	if got.Failures != 2 {
		t.Errorf("failures: want 2, got %d", got.Failures)
	}

	// 3rd failure crosses threshold → auto-disable in the same SQL.
	mustErr(t, UpdateWebhookHealth(ctx, d, id, 503, threshold))
	got, _ = GetWebhook(ctx, d, id)
	if got.Active {
		t.Errorf("after %d failures should be inactive", threshold)
	}
	if got.Failures != 3 {
		t.Errorf("failures: want 3, got %d", got.Failures)
	}
	if got.LastStatus != 503 {
		t.Errorf("last_status: want 503, got %d", got.LastStatus)
	}
}

func TestUpdateWebhookHealth_TransportErrorCountsAsFailure(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()
	id := seedWebhook(t, ctx, d, "alice", "https://a", "game.end", "s")

	// -1 == transport error per dispatcher contract.
	mustErr(t, UpdateWebhookHealth(ctx, d, id, -1, 5))
	got, _ := GetWebhook(ctx, d, id)
	if got.Failures != 1 {
		t.Errorf("failures: want 1, got %d", got.Failures)
	}
	if got.LastStatus != -1 {
		t.Errorf("last_status: want -1, got %d", got.LastStatus)
	}
}

func TestUpdateWebhookHealth_ZeroThresholdDoesNotAutoDisable(t *testing.T) {
	d := openMemDB(t)
	ctx := context.Background()
	id := seedWebhook(t, ctx, d, "alice", "https://a", "game.end", "s")

	// threshold=0 disables the auto-disable behaviour.
	for i := 0; i < 10; i++ {
		mustErr(t, UpdateWebhookHealth(ctx, d, id, 500, 0))
	}
	got, _ := GetWebhook(ctx, d, id)
	if !got.Active {
		t.Errorf("threshold=0 should leave webhook active regardless of failures (failures=%d)", got.Failures)
	}
	if got.Failures != 10 {
		t.Errorf("failures: want 10, got %d", got.Failures)
	}
}

// seedWebhook wraps InsertWebhook so test bodies stay one-liner per
// seed row. Fatals on error; returns the assigned id.
func seedWebhook(t *testing.T, ctx context.Context, d *sql.DB,
	owner, url, eventType, secret string) int64 {
	t.Helper()
	id, err := InsertWebhook(ctx, d, owner, url, eventType, secret)
	if err != nil {
		t.Fatalf("InsertWebhook(%s): %v", url, err)
	}
	return id
}

// mustErr fatals when err is non-nil. Used for setup statements
// whose only failure mode is a fundamentally broken DB.
func mustErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("update: %v", err)
	}
}
