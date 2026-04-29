package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Device is a persistent identity for a player or AI agent. Devices own
// decks, join parties, and accumulate game history.
type Device struct {
	ID          string
	DisplayName string
	CreatedAt   int64
	LastSeenAt  int64
}

// CreateDevice inserts a new device with a freshly-generated ID.
func CreateDevice(ctx context.Context, db *sql.DB, displayName string) (*Device, error) {
	d := &Device{
		ID:          NewID(32),
		DisplayName: displayName,
		CreatedAt:   Now(),
		LastSeenAt:  Now(),
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO device (id, display_name, created_at, last_seen_at) VALUES (?, ?, ?, ?)`,
		d.ID, d.DisplayName, d.CreatedAt, d.LastSeenAt)
	if err != nil {
		return nil, fmt.Errorf("insert device: %w", err)
	}
	return d, nil
}

// GetDevice fetches a device by ID. Returns sql.ErrNoRows if not found.
func GetDevice(ctx context.Context, db *sql.DB, id string) (*Device, error) {
	d := &Device{}
	err := db.QueryRowContext(ctx,
		`SELECT id, display_name, created_at, last_seen_at FROM device WHERE id = ?`, id,
	).Scan(&d.ID, &d.DisplayName, &d.CreatedAt, &d.LastSeenAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("get device %q: %w", id, err)
	}
	return d, nil
}

// TouchDevice updates the last_seen_at timestamp on an existing device.
func TouchDevice(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE device SET last_seen_at = ? WHERE id = ?`, Now(), id)
	return err
}
