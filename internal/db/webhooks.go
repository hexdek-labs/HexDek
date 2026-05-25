package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// webhooks — owner-registered HTTP callbacks fired on engine events
// (currently just `game.end`). Each row carries its own HMAC-SHA256
// secret used to sign outbound POST bodies, plus health counters so
// the dispatcher can auto-disable persistently failing endpoints.
//
// Lifecycle:
//   - InsertWebhook on register; secret is plaintext at rest (single-
//     tenant engine; if multi-tenant ever lands, encrypt at column).
//   - ListWebhooksByOwner powers the per-owner UI listing.
//   - ListActiveWebhooksForEvent is what the dispatcher walks at
//     fire time — indexed on (event_type, active).
//   - DeleteWebhook is owner-gated at the caller level.
//   - UpdateWebhookHealth records delivery outcomes (status code or
//     -1 for transport error); the dispatcher auto-clears Active=0
//     once Failures crosses a threshold.

const webhooksSchema = `
CREATE TABLE IF NOT EXISTS webhooks (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    owner         TEXT    NOT NULL,
    url           TEXT    NOT NULL,
    event_type    TEXT    NOT NULL,
    secret        TEXT    NOT NULL,
    created_at    INTEGER NOT NULL,
    last_fired_at INTEGER NOT NULL DEFAULT 0,
    last_status   INTEGER NOT NULL DEFAULT 0,
    failures      INTEGER NOT NULL DEFAULT 0,
    active        INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS webhooks_owner_idx ON webhooks(owner);
CREATE INDEX IF NOT EXISTS webhooks_event_active_idx ON webhooks(event_type, active);
`

// Webhook is the row + helper view used by the hexapi handlers and
// the WebhookDispatcher. Secret is intentionally a struct field, not
// JSON-tagged for export — the HTTP layer hides it from list /
// fetch responses (returned once at register time).
type Webhook struct {
	ID          int64
	Owner       string
	URL         string
	EventType   string
	Secret      string
	CreatedAt   time.Time
	LastFiredAt time.Time
	LastStatus  int
	Failures    int
	Active      bool
}

// EnsureWebhooksSchema creates the webhooks table idempotently.
// Call once at startup before any webhook handler is wired in.
func EnsureWebhooksSchema(ctx context.Context, sqlDB *sql.DB) error {
	if sqlDB == nil {
		return errors.New("db: nil database for webhooks schema")
	}
	_, err := sqlDB.ExecContext(ctx, webhooksSchema)
	return err
}

// InsertWebhook persists a fresh registration. Owner, URL, event
// type, and secret are mandatory; the caller is responsible for
// generating a sufficiently strong secret (the hexapi layer uses
// 32 random bytes hex-encoded). Returns the assigned row id.
func InsertWebhook(ctx context.Context, sqlDB *sql.DB, owner, url, eventType, secret string) (int64, error) {
	if sqlDB == nil {
		return 0, errors.New("db: nil database")
	}
	if owner == "" || url == "" || eventType == "" || secret == "" {
		return 0, errors.New("db: owner, url, event_type, secret all required")
	}
	res, err := sqlDB.ExecContext(ctx,
		`INSERT INTO webhooks (owner, url, event_type, secret, created_at, active)
		 VALUES (?, ?, ?, ?, ?, 1)`,
		owner, url, eventType, secret, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetWebhook returns one row by id. Returns sql.ErrNoRows when
// nothing matches. Used by the DELETE handler to confirm ownership
// before deletion.
func GetWebhook(ctx context.Context, sqlDB *sql.DB, id int64) (Webhook, error) {
	if sqlDB == nil {
		return Webhook{}, errors.New("db: nil database")
	}
	var w Webhook
	var createdAt, lastFiredAt int64
	var active int
	err := sqlDB.QueryRowContext(ctx,
		`SELECT id, owner, url, event_type, secret, created_at,
		        last_fired_at, last_status, failures, active
		 FROM webhooks WHERE id = ?`, id).Scan(
		&w.ID, &w.Owner, &w.URL, &w.EventType, &w.Secret, &createdAt,
		&lastFiredAt, &w.LastStatus, &w.Failures, &active)
	if err != nil {
		return Webhook{}, err
	}
	w.CreatedAt = time.Unix(createdAt, 0)
	if lastFiredAt > 0 {
		w.LastFiredAt = time.Unix(lastFiredAt, 0)
	}
	w.Active = active != 0
	return w, nil
}

// ListWebhooksByOwner returns every webhook (active or not) owned by
// the caller, newest first. Powers GET /api/webhooks.
func ListWebhooksByOwner(ctx context.Context, sqlDB *sql.DB, owner string) ([]Webhook, error) {
	if sqlDB == nil {
		return nil, errors.New("db: nil database")
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT id, owner, url, event_type, secret, created_at,
		        last_fired_at, last_status, failures, active
		 FROM webhooks WHERE owner = ? ORDER BY id DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWebhooks(rows)
}

// ListActiveWebhooksForEvent returns the active subscriptions for an
// event type, in stable id order. The dispatcher's fan-out walks
// this list — index `webhooks_event_active_idx` keeps the scan
// O(matches).
func ListActiveWebhooksForEvent(ctx context.Context, sqlDB *sql.DB, eventType string) ([]Webhook, error) {
	if sqlDB == nil {
		return nil, errors.New("db: nil database")
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT id, owner, url, event_type, secret, created_at,
		        last_fired_at, last_status, failures, active
		 FROM webhooks WHERE event_type = ? AND active = 1 ORDER BY id`, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWebhooks(rows)
}

func scanWebhooks(rows *sql.Rows) ([]Webhook, error) {
	var out []Webhook
	for rows.Next() {
		var w Webhook
		var createdAt, lastFiredAt int64
		var active int
		if err := rows.Scan(&w.ID, &w.Owner, &w.URL, &w.EventType, &w.Secret,
			&createdAt, &lastFiredAt, &w.LastStatus, &w.Failures, &active); err != nil {
			return nil, err
		}
		w.CreatedAt = time.Unix(createdAt, 0)
		if lastFiredAt > 0 {
			w.LastFiredAt = time.Unix(lastFiredAt, 0)
		}
		w.Active = active != 0
		out = append(out, w)
	}
	return out, rows.Err()
}

// DeleteWebhookByOwner removes one webhook iff (id, owner) match.
// Returning rowsAffected lets the HTTP handler distinguish a real
// 404 (no row at all) from an ownership mismatch (row exists but
// not yours). Both cases get the same caller-facing 404 to avoid
// leaking the existence of someone else's webhook.
func DeleteWebhookByOwner(ctx context.Context, sqlDB *sql.DB, id int64, owner string) (int64, error) {
	if sqlDB == nil {
		return 0, errors.New("db: nil database")
	}
	res, err := sqlDB.ExecContext(ctx,
		`DELETE FROM webhooks WHERE id = ? AND owner = ?`, id, owner)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateWebhookHealth records the outcome of one delivery attempt.
// status is the HTTP status code on success, -1 for transport error.
// failures auto-bumps on 5xx + transport errors and auto-resets on
// 2xx — callers don't have to compute that themselves.
//
// When failuresThreshold > 0 and the rolling Failures count crosses
// it, Active flips to 0 in the same statement, halting further
// fan-outs without a follow-up round trip.
func UpdateWebhookHealth(ctx context.Context, sqlDB *sql.DB, id int64, status int, failuresThreshold int) error {
	if sqlDB == nil {
		return errors.New("db: nil database")
	}
	// Compute Failures delta inline so the increment/reset is atomic
	// with the status write. SQLite supports CASE in UPDATE since
	// forever.
	_, err := sqlDB.ExecContext(ctx, `
		UPDATE webhooks
		SET last_fired_at = ?,
		    last_status   = ?,
		    failures = CASE
		        WHEN ? BETWEEN 200 AND 299 THEN 0
		        ELSE failures + 1
		    END,
		    active = CASE
		        WHEN ? > 0 AND
		             CASE
		                 WHEN ? BETWEEN 200 AND 299 THEN 0
		                 ELSE failures + 1
		             END >= ?
		        THEN 0
		        ELSE active
		    END
		WHERE id = ?`,
		time.Now().Unix(),
		status,
		status,
		failuresThreshold,
		status,
		failuresThreshold,
		id,
	)
	return err
}

// SetWebhookActive flips the active flag explicitly. Used by tests
// + administrative scripts; the dispatcher reaches this state via
// UpdateWebhookHealth's auto-disable path.
func SetWebhookActive(ctx context.Context, sqlDB *sql.DB, id int64, active bool) error {
	if sqlDB == nil {
		return errors.New("db: nil database")
	}
	v := 0
	if active {
		v = 1
	}
	_, err := sqlDB.ExecContext(ctx, `UPDATE webhooks SET active = ? WHERE id = ?`, v, id)
	return err
}
