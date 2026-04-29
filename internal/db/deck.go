package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Deck is a stored deck owned by a device. raw_json holds the full Moxfield
// deck export for re-shuffling when a game starts.
type Deck struct {
	ID            string
	OwnerDeviceID string
	Name          string
	CommanderName string
	Format        string
	MoxfieldURL   string
	ImportedAt    int64
	RawJSON       string
}

// CreateDeck inserts a deck record and returns it with its assigned ID.
func CreateDeck(ctx context.Context, db *sql.DB, d *Deck) error {
	d.ID = NewID(32)
	d.ImportedAt = Now()
	if d.Format == "" {
		d.Format = "commander"
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO deck (id, owner_device_id, name, commander_name, format, moxfield_url, imported_at, raw_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.OwnerDeviceID, d.Name, d.CommanderName, d.Format, d.MoxfieldURL, d.ImportedAt, d.RawJSON)
	if err != nil {
		return fmt.Errorf("insert deck: %w", err)
	}
	return nil
}

// GetDeck fetches a deck by ID.
func GetDeck(ctx context.Context, db *sql.DB, id string) (*Deck, error) {
	d := &Deck{}
	err := db.QueryRowContext(ctx,
		`SELECT id, owner_device_id, name, COALESCE(commander_name,''), format, COALESCE(moxfield_url,''), imported_at, raw_json
		 FROM deck WHERE id = ?`, id,
	).Scan(&d.ID, &d.OwnerDeviceID, &d.Name, &d.CommanderName, &d.Format, &d.MoxfieldURL, &d.ImportedAt, &d.RawJSON)
	if err != nil {
		return nil, fmt.Errorf("get deck %q: %w", id, err)
	}
	return d, nil
}

// ListDecksByDevice returns all decks owned by a device, newest first.
func ListDecksByDevice(ctx context.Context, db *sql.DB, deviceID string) ([]*Deck, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, owner_device_id, name, COALESCE(commander_name,''), format, COALESCE(moxfield_url,''), imported_at, raw_json
		 FROM deck WHERE owner_device_id = ? ORDER BY imported_at DESC`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("list decks: %w", err)
	}
	defer rows.Close()

	var out []*Deck
	for rows.Next() {
		d := &Deck{}
		if err := rows.Scan(&d.ID, &d.OwnerDeviceID, &d.Name, &d.CommanderName, &d.Format, &d.MoxfieldURL, &d.ImportedAt, &d.RawJSON); err != nil {
			return nil, fmt.Errorf("scan deck row: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
