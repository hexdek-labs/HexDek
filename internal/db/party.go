package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Party is a pre-game lobby that players join via a short code before
// starting a game.
type Party struct {
	ID            string
	HostDeviceID  string
	State         string // 'lobby' | 'playing' | 'finished'
	MaxPlayers    int
	CreatedAt     int64
}

// PartyMember represents a player or AI agent within a party.
type PartyMember struct {
	PartyID       string
	DeviceID      string
	DeckID        sql.NullString
	SeatPosition  int
	IsAI          bool
	JoinedAt      int64
}

// CreateParty creates a new party with the given host. Returns the party
// with a freshly-generated 6-char code as ID.
func CreateParty(ctx context.Context, db *sql.DB, hostDeviceID string, maxPlayers int) (*Party, error) {
	if maxPlayers < 2 || maxPlayers > 4 {
		return nil, fmt.Errorf("max_players must be 2-4, got %d", maxPlayers)
	}

	// Retry on code collision (very unlikely with 30^6 = 729M codes)
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		p := &Party{
			ID:           NewPartyCode(),
			HostDeviceID: hostDeviceID,
			State:        "lobby",
			MaxPlayers:   maxPlayers,
			CreatedAt:    Now(),
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO party (id, host_device_id, state, max_players, created_at) VALUES (?, ?, ?, ?, ?)`,
			p.ID, p.HostDeviceID, p.State, p.MaxPlayers, p.CreatedAt)
		if err == nil {
			// Auto-add host as first member at seat 0
			if _, err := db.ExecContext(ctx,
				`INSERT INTO party_member (party_id, device_id, seat_position, is_ai, joined_at)
				 VALUES (?, ?, 0, 0, ?)`, p.ID, hostDeviceID, p.CreatedAt); err != nil {
				return nil, fmt.Errorf("auto-join host: %w", err)
			}
			return p, nil
		}
		// On ID collision try a new code
	}
	return nil, fmt.Errorf("party code generation failed after %d attempts", maxAttempts)
}

// GetParty fetches a party by ID (case-insensitive on the code).
func GetParty(ctx context.Context, db *sql.DB, id string) (*Party, error) {
	p := &Party{}
	err := db.QueryRowContext(ctx,
		`SELECT id, host_device_id, state, max_players, created_at FROM party WHERE id = ?`, id,
	).Scan(&p.ID, &p.HostDeviceID, &p.State, &p.MaxPlayers, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("get party %q: %w", id, err)
	}
	return p, nil
}

// ListPartyMembers returns all members of a party in seat order.
func ListPartyMembers(ctx context.Context, db *sql.DB, partyID string) ([]*PartyMember, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT party_id, device_id, deck_id, seat_position, is_ai, joined_at
		 FROM party_member WHERE party_id = ? ORDER BY seat_position`, partyID)
	if err != nil {
		return nil, fmt.Errorf("list party members: %w", err)
	}
	defer rows.Close()

	var out []*PartyMember
	for rows.Next() {
		m := &PartyMember{}
		var isAIInt int
		if err := rows.Scan(&m.PartyID, &m.DeviceID, &m.DeckID, &m.SeatPosition, &isAIInt, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan party member: %w", err)
		}
		m.IsAI = isAIInt != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// JoinParty adds a device to a party and assigns it the next open seat.
// Returns ErrPartyFull if the party is already at max_players.
func JoinParty(ctx context.Context, db *sql.DB, partyID string, deviceID string, deckID string, isAI bool) (*PartyMember, error) {
	p, err := GetParty(ctx, db, partyID)
	if err != nil {
		return nil, err
	}
	if p.State != "lobby" {
		return nil, fmt.Errorf("party %q is not accepting joins (state: %s)", partyID, p.State)
	}

	members, err := ListPartyMembers(ctx, db, partyID)
	if err != nil {
		return nil, err
	}
	if len(members) >= p.MaxPlayers {
		return nil, ErrPartyFull
	}

	// Check duplicate device
	for _, m := range members {
		if m.DeviceID == deviceID {
			return nil, fmt.Errorf("device %q is already in party %q", deviceID, partyID)
		}
	}

	// Pick next available seat
	seatTaken := make(map[int]bool, len(members))
	for _, m := range members {
		seatTaken[m.SeatPosition] = true
	}
	seat := -1
	for i := 0; i < p.MaxPlayers; i++ {
		if !seatTaken[i] {
			seat = i
			break
		}
	}
	if seat < 0 {
		return nil, ErrPartyFull
	}

	m := &PartyMember{
		PartyID:      partyID,
		DeviceID:     deviceID,
		SeatPosition: seat,
		IsAI:         isAI,
		JoinedAt:     Now(),
	}
	if deckID != "" {
		m.DeckID = sql.NullString{String: deckID, Valid: true}
	}
	isAIInt := 0
	if isAI {
		isAIInt = 1
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO party_member (party_id, device_id, deck_id, seat_position, is_ai, joined_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.PartyID, m.DeviceID, m.DeckID, m.SeatPosition, isAIInt, m.JoinedAt)
	if err != nil {
		return nil, fmt.Errorf("insert party member: %w", err)
	}
	return m, nil
}

// GetActiveGameForParty returns the most recent unfinished game for a party,
// or sql.ErrNoRows if none exists.
func GetActiveGameForParty(ctx context.Context, db *sql.DB, partyID string) (string, error) {
	var gameID string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM game WHERE party_id = ? AND finished_at IS NULL ORDER BY started_at DESC LIMIT 1`,
		partyID).Scan(&gameID)
	return gameID, err
}

// SetMemberDeck updates the deck_id of an existing party member.
func SetMemberDeck(ctx context.Context, db *sql.DB, partyID, deviceID, deckID string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE party_member SET deck_id = ? WHERE party_id = ? AND device_id = ?`,
		deckID, partyID, deviceID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no party_member matched (party=%q, device=%q)", partyID, deviceID)
	}
	return nil
}

// ErrPartyFull indicates a join attempt against a party at max capacity.
var ErrPartyFull = errors.New("party is full")
