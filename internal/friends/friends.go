// Package friends is HexDek's owner-keyed friend list.
//
// The existing `friend` table in db/schema.sql is keyed by device_id and
// belongs to the WebSocket device-pairing layer. This package introduces
// owner_friend, keyed by the URL-facing `owner` slug (e.g. "josh",
// "meglin") that the deck archive already uses.
//
// Friendship is bidirectional. AddFriend(a, b) inserts two rows — (a, b)
// and (b, a) — so ListFriends(a) shows b without a self-join. This is
// "mutual add for now"; a request/accept gate is a future iteration.
//
// All operations are idempotent: AddFriend can be called twice with no
// duplicate-row error, RemoveFriend on a non-friendship is a no-op.
package friends

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"encoding/json"
)

// Tracker owns the SQLite-backed friend list.
type Tracker struct {
	db *sql.DB
}

// New ensures the schema is in place and returns a ready-to-use Tracker.
func New(db *sql.DB) (*Tracker, error) {
	if db == nil {
		return nil, errors.New("friends: nil db")
	}
	t := &Tracker{db: db}
	if err := t.EnsureSchema(context.Background()); err != nil {
		return nil, err
	}
	return t, nil
}

const schemaDDL = `
CREATE TABLE IF NOT EXISTS owner_friend (
    owner_a    TEXT NOT NULL,
    owner_b    TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (owner_a, owner_b)
);
CREATE INDEX IF NOT EXISTS idx_owner_friend_a ON owner_friend(owner_a);
CREATE INDEX IF NOT EXISTS idx_owner_friend_b ON owner_friend(owner_b);
`

func (t *Tracker) EnsureSchema(ctx context.Context) error {
	_, err := t.db.ExecContext(ctx, schemaDDL)
	return err
}

// ─────────────────────────── CRUD ─────────────────────────────────────

// AddFriend records a bidirectional friendship between ownerA and ownerB.
// Idempotent. Returns an error if either slug is empty or a self-add is
// attempted.
func (t *Tracker) AddFriend(ctx context.Context, ownerA, ownerB string) error {
	a, b, err := normalizePair(ownerA, ownerB)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, pair := range [2][2]string{{a, b}, {b, a}} {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO owner_friend (owner_a, owner_b, created_at) VALUES (?, ?, ?)`,
			pair[0], pair[1], now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RemoveFriend tears down both directions of a friendship. Idempotent.
func (t *Tracker) RemoveFriend(ctx context.Context, ownerA, ownerB string) error {
	a, b, err := normalizePair(ownerA, ownerB)
	if err != nil {
		return err
	}
	_, err = t.db.ExecContext(ctx,
		`DELETE FROM owner_friend WHERE (owner_a = ? AND owner_b = ?) OR (owner_a = ? AND owner_b = ?)`,
		a, b, b, a)
	return err
}

// ListFriends returns the slugs of every owner whom `owner` is friends
// with, alphabetically sorted.
func (t *Tracker) ListFriends(ctx context.Context, owner string) ([]string, error) {
	owner = strings.ToLower(strings.TrimSpace(owner))
	if owner == "" {
		return nil, errors.New("friends: empty owner")
	}
	rows, err := t.db.QueryContext(ctx,
		`SELECT owner_b FROM owner_friend WHERE owner_a = ? ORDER BY owner_b ASC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// AreFriends reports whether ownerA and ownerB are mutually friends.
// Checks one row only — AddFriend's bidirectional insert means a single
// hit is enough.
func (t *Tracker) AreFriends(ctx context.Context, ownerA, ownerB string) (bool, error) {
	a, b, err := normalizePair(ownerA, ownerB)
	if err != nil {
		return false, err
	}
	var x int
	err = t.db.QueryRowContext(ctx,
		`SELECT 1 FROM owner_friend WHERE owner_a = ? AND owner_b = ?`, a, b).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func normalizePair(a, b string) (string, string, error) {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return "", "", errors.New("friends: empty owner slug")
	}
	if a == b {
		return "", "", errors.New("friends: cannot friend yourself")
	}
	return a, b, nil
}

// ─────────────────────────── HTTP API ─────────────────────────────────

// Register wires the friend endpoints onto mux.
//
//   POST   /api/friends/{owner}?as=<me>   add `owner` to my friend list
//   DELETE /api/friends/{owner}?as=<me>   drop `owner` from my friend list
//   GET    /api/friends?as=<me>           list my friends
//
// `as` carries the requestor's owner slug. Auth-gating this against the
// authenticated identity is a follow-up — the codebase's existing endpoints
// trust the client similarly while the auth model is still in flux.
func (t *Tracker) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/friends/{owner}", t.handleAdd)
	mux.HandleFunc("DELETE /api/friends/{owner}", t.handleRemove)
	mux.HandleFunc("GET /api/friends", t.handleList)
}

func (t *Tracker) handleAdd(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("owner")
	requestor := r.URL.Query().Get("as")
	if requestor == "" {
		http.Error(w, "missing ?as=<your owner slug>", http.StatusBadRequest)
		return
	}
	if err := t.AddFriend(r.Context(), requestor, target); err != nil {
		http.Error(w, "add: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"added": true, "owner": strings.ToLower(target), "as": strings.ToLower(requestor)})
}

func (t *Tracker) handleRemove(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("owner")
	requestor := r.URL.Query().Get("as")
	if requestor == "" {
		http.Error(w, "missing ?as=<your owner slug>", http.StatusBadRequest)
		return
	}
	if err := t.RemoveFriend(r.Context(), requestor, target); err != nil {
		http.Error(w, "remove: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"removed": true, "owner": strings.ToLower(target), "as": strings.ToLower(requestor)})
}

func (t *Tracker) handleList(w http.ResponseWriter, r *http.Request) {
	requestor := r.URL.Query().Get("as")
	if requestor == "" {
		http.Error(w, "missing ?as=<your owner slug>", http.StatusBadRequest)
		return
	}
	list, err := t.ListFriends(r.Context(), requestor)
	if err != nil {
		http.Error(w, "list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"owner": strings.ToLower(requestor), "friends": list})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
