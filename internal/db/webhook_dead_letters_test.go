package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Local helper to spin up an in-memory DB with BOTH webhook-related
// schemas applied. Companion to openMemDB in webhooks_test.go — kept
// separate so we don't have to touch the sibling file's signature.
func openDeadLetterDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	if err := EnsureWebhooksSchema(ctx, d); err != nil {
		t.Fatalf("EnsureWebhooksSchema: %v", err)
	}
	if err := EnsureWebhookDeadLettersSchema(ctx, d); err != nil {
		t.Fatalf("EnsureWebhookDeadLettersSchema: %v", err)
	}
	return d
}

func TestEnsureWebhookDeadLettersSchema_NilDB(t *testing.T) {
	if err := EnsureWebhookDeadLettersSchema(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestEnsureWebhookDeadLettersSchema_Idempotent(t *testing.T) {
	d := openDeadLetterDB(t)
	if err := EnsureWebhookDeadLettersSchema(context.Background(), d); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
}

func TestInsertWebhookDeadLetter_RoundTrip(t *testing.T) {
	d := openDeadLetterDB(t)
	ctx := context.Background()
	first := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	last := time.Date(2026, 5, 24, 12, 0, 30, 0, time.UTC)
	id, err := InsertWebhookDeadLetter(ctx, d, WebhookDeadLetter{
		WebhookID:      42,
		EventType:      "game.end",
		Payload:        []byte(`{"foo":"bar"}`),
		Attempts:       5,
		LastStatus:     503,
		LastError:      "transport: connection refused",
		FirstAttempted: first,
		LastAttempted:  last,
	})
	if err != nil {
		t.Fatalf("InsertWebhookDeadLetter: %v", err)
	}
	if id <= 0 {
		t.Errorf("id: want > 0, got %d", id)
	}

	rows, err := ListWebhookDeadLetters(ctx, d, 42, 10)
	if err != nil {
		t.Fatalf("ListWebhookDeadLetters: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	got := rows[0]
	if got.WebhookID != 42 || got.EventType != "game.end" {
		t.Errorf("row mismatch: %+v", got)
	}
	if string(got.Payload) != `{"foo":"bar"}` {
		t.Errorf("payload: want %q, got %q", `{"foo":"bar"}`, string(got.Payload))
	}
	if got.Attempts != 5 || got.LastStatus != 503 {
		t.Errorf("attempts/status: %+v", got)
	}
	if !got.FirstAttempted.Equal(first) || !got.LastAttempted.Equal(last) {
		t.Errorf("timestamps: first=%v last=%v", got.FirstAttempted, got.LastAttempted)
	}
}

func TestInsertWebhookDeadLetter_RejectsEmptyFields(t *testing.T) {
	d := openDeadLetterDB(t)
	ctx := context.Background()
	cases := []struct {
		name string
		row  WebhookDeadLetter
	}{
		{"zero webhook_id", WebhookDeadLetter{EventType: "game.end"}},
		{"empty event_type", WebhookDeadLetter{WebhookID: 1}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if _, err := InsertWebhookDeadLetter(ctx, d, tc.row); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestListWebhookDeadLetters_OrderAndIsolation(t *testing.T) {
	d := openDeadLetterDB(t)
	ctx := context.Background()
	// Seed three rows for webhook 1 + one for webhook 2.
	for i := 0; i < 3; i++ {
		_, err := InsertWebhookDeadLetter(ctx, d, WebhookDeadLetter{
			WebhookID: 1, EventType: "game.end", Payload: []byte("{}"),
			Attempts: i + 1, LastStatus: 500,
		})
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	if _, err := InsertWebhookDeadLetter(ctx, d, WebhookDeadLetter{
		WebhookID: 2, EventType: "game.end", Payload: []byte("{}"),
		Attempts: 1, LastStatus: 500,
	}); err != nil {
		t.Fatalf("seed for webhook 2: %v", err)
	}

	rows, err := ListWebhookDeadLetters(ctx, d, 1, 0) // default limit
	if err != nil {
		t.Fatalf("ListWebhookDeadLetters: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows for webhook 1, got %d", len(rows))
	}
	// Newest first → ids should descend.
	for i := 0; i < len(rows)-1; i++ {
		if rows[i].ID <= rows[i+1].ID {
			t.Errorf("rows not in DESC id order: rows[%d].ID=%d rows[%d].ID=%d",
				i, rows[i].ID, i+1, rows[i+1].ID)
		}
	}
	for _, r := range rows {
		if r.WebhookID != 1 {
			t.Errorf("webhook 2 row leaked into webhook 1 query: %+v", r)
		}
	}
}

func TestListWebhookDeadLetters_LimitTruncates(t *testing.T) {
	d := openDeadLetterDB(t)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_, _ = InsertWebhookDeadLetter(ctx, d, WebhookDeadLetter{
			WebhookID: 1, EventType: "game.end", Payload: []byte("{}"),
			Attempts: 1, LastStatus: 500,
		})
	}
	rows, err := ListWebhookDeadLetters(ctx, d, 1, 3)
	if err != nil {
		t.Fatalf("ListWebhookDeadLetters: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("limit not honored: want 3, got %d", len(rows))
	}
}

func TestCountWebhookDeadLetters(t *testing.T) {
	d := openDeadLetterDB(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_, _ = InsertWebhookDeadLetter(ctx, d, WebhookDeadLetter{
			WebhookID: 1, EventType: "game.end", Payload: []byte("{}"),
			Attempts: 1, LastStatus: 500,
		})
	}
	n, err := CountWebhookDeadLetters(ctx, d, 1)
	if err != nil {
		t.Fatalf("CountWebhookDeadLetters: %v", err)
	}
	if n != 4 {
		t.Errorf("count: want 4, got %d", n)
	}
	// Other webhook id has 0 rows.
	n, _ = CountWebhookDeadLetters(ctx, d, 999)
	if n != 0 {
		t.Errorf("unrelated webhook: want 0, got %d", n)
	}
}
