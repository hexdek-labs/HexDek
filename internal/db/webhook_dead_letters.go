package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// webhook_dead_letters — terminal record for webhook deliveries that
// exhausted every retry attempt. The companion to webhooks (#303): a
// retry-loop in the dispatcher tries `MaxAttempts` times with
// exponential backoff; if all of them fail (transport error or
// retriable HTTP status), the dispatcher inserts a row here so the
// payload + last error are forensically available for operators.
//
// Why a separate table:
//   - The live webhooks row only tracks rolling-failure counters +
//     last_status, which is enough to auto-disable but not enough to
//     reconstruct what was lost. Dead-letter rows keep the full
//     payload so an operator can replay or audit.
//   - 4xx (client) failures land here too — those aren't retried,
//     they're terminal on the first attempt. Same forensic value.
//
// The table is owned by the dispatcher path; the HTTP layer reads it
// via GET /api/webhooks/{id}/dead-letters.

const webhookDeadLettersSchema = `
CREATE TABLE IF NOT EXISTS webhook_dead_letters (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    webhook_id      INTEGER NOT NULL,
    event_type      TEXT NOT NULL,
    payload         BLOB NOT NULL,
    attempts        INTEGER NOT NULL,
    last_status     INTEGER NOT NULL,
    last_error      TEXT NOT NULL DEFAULT '',
    first_attempted INTEGER NOT NULL,
    last_attempted  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS wdl_webhook_idx ON webhook_dead_letters(webhook_id, id DESC);
`

// WebhookDeadLetter is the in-memory view of one row.
type WebhookDeadLetter struct {
	ID             int64
	WebhookID      int64
	EventType      string
	Payload        []byte
	Attempts       int
	LastStatus     int
	LastError      string
	FirstAttempted time.Time
	LastAttempted  time.Time
}

// EnsureWebhookDeadLettersSchema creates the table idempotently.
// Call at startup before any dispatcher writes.
func EnsureWebhookDeadLettersSchema(ctx context.Context, sqlDB *sql.DB) error {
	if sqlDB == nil {
		return errors.New("db: nil database for webhook_dead_letters schema")
	}
	_, err := sqlDB.ExecContext(ctx, webhookDeadLettersSchema)
	return err
}

// InsertWebhookDeadLetter persists one terminal-failure record. The
// dispatcher calls this after exhausting retries OR on a non-
// retriable status. Returns the assigned id.
func InsertWebhookDeadLetter(ctx context.Context, sqlDB *sql.DB, d WebhookDeadLetter) (int64, error) {
	if sqlDB == nil {
		return 0, errors.New("db: nil database")
	}
	if d.WebhookID <= 0 || d.EventType == "" {
		return 0, errors.New("db: webhook_id + event_type required")
	}
	first := d.FirstAttempted.Unix()
	last := d.LastAttempted.Unix()
	if first <= 0 {
		first = time.Now().Unix()
	}
	if last <= 0 {
		last = first
	}
	res, err := sqlDB.ExecContext(ctx,
		`INSERT INTO webhook_dead_letters
		 (webhook_id, event_type, payload, attempts, last_status, last_error,
		  first_attempted, last_attempted)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.WebhookID, d.EventType, d.Payload, d.Attempts, d.LastStatus, d.LastError,
		first, last)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListWebhookDeadLetters returns dead-letter rows for a single
// webhook, newest first, capped at limit. Limit ≤ 0 defaults to 50.
func ListWebhookDeadLetters(ctx context.Context, sqlDB *sql.DB, webhookID int64, limit int) ([]WebhookDeadLetter, error) {
	if sqlDB == nil {
		return nil, errors.New("db: nil database")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT id, webhook_id, event_type, payload, attempts, last_status, last_error,
		        first_attempted, last_attempted
		 FROM webhook_dead_letters
		 WHERE webhook_id = ?
		 ORDER BY id DESC
		 LIMIT ?`, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookDeadLetter
	for rows.Next() {
		var d WebhookDeadLetter
		var first, last int64
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload,
			&d.Attempts, &d.LastStatus, &d.LastError, &first, &last); err != nil {
			return nil, err
		}
		d.FirstAttempted = time.Unix(first, 0)
		d.LastAttempted = time.Unix(last, 0)
		out = append(out, d)
	}
	return out, rows.Err()
}

// CountWebhookDeadLetters returns the dead-letter row count for one
// webhook. Used by tests + observability.
func CountWebhookDeadLetters(ctx context.Context, sqlDB *sql.DB, webhookID int64) (int, error) {
	if sqlDB == nil {
		return 0, errors.New("db: nil database")
	}
	var n int
	err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM webhook_dead_letters WHERE webhook_id = ?`, webhookID).Scan(&n)
	return n, err
}
