// Package db provides the SQLite-backed persistence layer for mtgsquad.
//
// The schema is split into two tiers: persistent identity (devices, decks,
// friends) and ephemeral game state (parties, games, cards, turn state).
// Persistent data survives server restarts; ephemeral data is wiped on
// each cold start.
package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Open opens (or creates) a SQLite database at path and ensures the schema
// is applied. Pass ":memory:" for an ephemeral in-memory database.
func Open(path string) (*sql.DB, error) {
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-8000)&_pragma=temp_store(MEMORY)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	db.SetMaxOpenConns(4)
	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return db, nil
}

// applyMigrations runs idempotent ALTER TABLE statements. SQLite has no
// "ADD COLUMN IF NOT EXISTS"; we check pragma table_info instead.
func applyMigrations(db *sql.DB) error {
	type colAdd struct {
		table, column, ddl string
	}
	migrations := []colAdd{
		{"game_player", "lands_played_turn", "INTEGER NOT NULL DEFAULT 0"},
		{"game_card", "tapped_for_mana_this_turn", "INTEGER NOT NULL DEFAULT 0"},
		{"showmatch_game_seat", "battlefield_cards", "TEXT NOT NULL DEFAULT '[]'"},
		{"showmatch_game_seat", "deck_key", "TEXT NOT NULL DEFAULT ''"},
	}
	// Migrate showmatch_elo from commander-keyed to deck_key-keyed.
	hasDeckKey, _ := columnExists(db, "showmatch_elo", "deck_key")
	if !hasDeckKey {
		db.Exec("DROP TABLE IF EXISTS showmatch_elo")
		db.Exec(`CREATE TABLE IF NOT EXISTS showmatch_elo (
			deck_key     TEXT PRIMARY KEY,
			commander    TEXT NOT NULL DEFAULT '',
			owner        TEXT NOT NULL DEFAULT '',
			rating       REAL NOT NULL DEFAULT 1500.0,
			games        INTEGER NOT NULL DEFAULT 0,
			wins         INTEGER NOT NULL DEFAULT 0,
			losses       INTEGER NOT NULL DEFAULT 0,
			delta        REAL NOT NULL DEFAULT 0.0,
			updated_at   INTEGER NOT NULL DEFAULT 0
		)`)
	}

	for _, m := range migrations {
		exists, err := columnExists(db, m.table, m.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.table, m.column, m.ddl))
		if err != nil {
			return fmt.Errorf("add %s.%s: %w", m.table, m.column, err)
		}
	}
	return nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// NewID returns a hex-encoded random ID suitable for primary keys.
// Length is in characters, must be even.
func NewID(length int) string {
	if length%2 != 0 {
		length++
	}
	b := make([]byte, length/2)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is fatal; the process can't continue safely
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// NewPartyCode returns a short, human-friendly join code for parties.
// Format: 6 uppercase alphanumeric characters, ambiguity-stripped (no
// 0/O/I/1 to reduce typo confusion).
func NewPartyCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const length = 6
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	out := make([]byte, length)
	for i, v := range b {
		out[i] = charset[int(v)%len(charset)]
	}
	return string(out)
}

// Now returns the current unix epoch in seconds.
func Now() int64 {
	return time.Now().Unix()
}
