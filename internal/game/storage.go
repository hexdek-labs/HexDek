package game

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hexdek/hexdek/internal/db"
)

// ---------- Game ----------

// CreateGame inserts a new game record tied to a party.
func CreateGame(ctx context.Context, database *sql.DB, partyID, shuffleSeedHash string) (*Game, error) {
	g := &Game{
		ID:        db.NewID(32),
		PartyID:   partyID,
		StartedAt: db.Now(),
	}
	_, err := database.ExecContext(ctx,
		`INSERT INTO game (id, party_id, started_at, shuffle_seed_hash) VALUES (?, ?, ?, ?)`,
		g.ID, g.PartyID, g.StartedAt, shuffleSeedHash)
	if err != nil {
		return nil, fmt.Errorf("insert game: %w", err)
	}
	return g, nil
}

// FinishGame marks a game finished and records the winner.
func FinishGame(ctx context.Context, database *sql.DB, gameID string, winnerDeviceID string) error {
	_, err := database.ExecContext(ctx,
		`UPDATE game SET finished_at = ?, winner_device_id = ? WHERE id = ?`,
		db.Now(), winnerDeviceID, gameID)
	return err
}

// GetGame fetches a game by ID.
func GetGame(ctx context.Context, database *sql.DB, gameID string) (*Game, error) {
	g := &Game{}
	var winner sql.NullString
	var finishedAt sql.NullInt64
	err := database.QueryRowContext(ctx,
		`SELECT id, party_id, started_at, finished_at, winner_device_id FROM game WHERE id = ?`, gameID,
	).Scan(&g.ID, &g.PartyID, &g.StartedAt, &finishedAt, &winner)
	if err != nil {
		return nil, fmt.Errorf("get game: %w", err)
	}
	if finishedAt.Valid {
		g.FinishedAt = finishedAt.Int64
	}
	if winner.Valid {
		g.Winner = winner.String
	}
	return g, nil
}

// ---------- GamePlayer ----------

func CreateGamePlayer(ctx context.Context, database *sql.DB, p *Player) error {
	_, err := database.ExecContext(ctx,
		`INSERT INTO game_player (game_id, seat_position, device_id, deck_id, life)
		 VALUES (?, ?, ?, ?, ?)`,
		p.GameID, p.SeatPosition, p.DeviceID, p.DeckID, p.Life)
	return err
}

func GetGamePlayer(ctx context.Context, database *sql.DB, gameID string, seat int) (*Player, error) {
	p := &Player{}
	err := database.QueryRowContext(ctx,
		`SELECT game_id, seat_position, device_id, deck_id, life, poison_counters,
		        mana_pool_w, mana_pool_u, mana_pool_b, mana_pool_r, mana_pool_g, mana_pool_c,
		        lands_played_turn
		 FROM game_player WHERE game_id = ? AND seat_position = ?`, gameID, seat,
	).Scan(&p.GameID, &p.SeatPosition, &p.DeviceID, &p.DeckID, &p.Life, &p.PoisonCounters,
		&p.ManaPoolW, &p.ManaPoolU, &p.ManaPoolB, &p.ManaPoolR, &p.ManaPoolG, &p.ManaPoolC,
		&p.LandsPlayedTurn)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func ListGamePlayers(ctx context.Context, database *sql.DB, gameID string) ([]*Player, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT game_id, seat_position, device_id, deck_id, life, poison_counters,
		        mana_pool_w, mana_pool_u, mana_pool_b, mana_pool_r, mana_pool_g, mana_pool_c,
		        lands_played_turn
		 FROM game_player WHERE game_id = ? ORDER BY seat_position`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Player
	for rows.Next() {
		p := &Player{}
		if err := rows.Scan(&p.GameID, &p.SeatPosition, &p.DeviceID, &p.DeckID, &p.Life, &p.PoisonCounters,
			&p.ManaPoolW, &p.ManaPoolU, &p.ManaPoolB, &p.ManaPoolR, &p.ManaPoolG, &p.ManaPoolC,
			&p.LandsPlayedTurn); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func UpdateGamePlayer(ctx context.Context, database *sql.DB, p *Player) error {
	_, err := database.ExecContext(ctx,
		`UPDATE game_player SET life = ?, poison_counters = ?,
		   mana_pool_w = ?, mana_pool_u = ?, mana_pool_b = ?,
		   mana_pool_r = ?, mana_pool_g = ?, mana_pool_c = ?,
		   lands_played_turn = ?
		 WHERE game_id = ? AND seat_position = ?`,
		p.Life, p.PoisonCounters,
		p.ManaPoolW, p.ManaPoolU, p.ManaPoolB,
		p.ManaPoolR, p.ManaPoolG, p.ManaPoolC,
		p.LandsPlayedTurn,
		p.GameID, p.SeatPosition)
	return err
}

// ---------- GameCard ----------

func CreateGameCard(ctx context.Context, database *sql.DB, c *Card) error {
	_, err := database.ExecContext(ctx,
		`INSERT INTO game_card (game_id, instance_id, card_name, card_data, owner_seat, zone, zone_position, tapped, revealed_to)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.GameID, c.InstanceID, c.Name, marshalCardData(c), c.OwnerSeat, string(c.Zone), c.ZonePosition, boolToInt(c.Tapped), c.RevealedTo)
	return err
}

func MoveCard(ctx context.Context, database *sql.DB, gameID, instanceID string, newZone Zone, newPos int) error {
	// Moving zones clears tap state and the per-turn mana flag — a permanent
	// that bounces back to hand and replays starts fresh.
	_, err := database.ExecContext(ctx,
		`UPDATE game_card SET zone = ?, zone_position = ?, tapped = 0, tapped_for_mana_this_turn = 0 WHERE game_id = ? AND instance_id = ?`,
		string(newZone), newPos, gameID, instanceID)
	return err
}

func SetCardTapped(ctx context.Context, database *sql.DB, gameID, instanceID string, tapped bool) error {
	_, err := database.ExecContext(ctx,
		`UPDATE game_card SET tapped = ? WHERE game_id = ? AND instance_id = ?`,
		boolToInt(tapped), gameID, instanceID)
	return err
}

// SetTappedForManaThisTurn marks (or clears) the per-card flag used to prevent
// the tap → untap → retap free-mana exploit. The flag is also cleared in bulk
// at each player's untap step.
func SetTappedForManaThisTurn(ctx context.Context, database *sql.DB, gameID, instanceID string, used bool) error {
	_, err := database.ExecContext(ctx,
		`UPDATE game_card SET tapped_for_mana_this_turn = ? WHERE game_id = ? AND instance_id = ?`,
		boolToInt(used), gameID, instanceID)
	return err
}

// ClearTappedForManaForSeat clears the per-turn mana-tap flag for every card
// owned by the given seat. Called from the untap step.
func ClearTappedForManaForSeat(ctx context.Context, database *sql.DB, gameID string, seat int) error {
	_, err := database.ExecContext(ctx,
		`UPDATE game_card SET tapped_for_mana_this_turn = 0 WHERE game_id = ? AND owner_seat = ?`,
		gameID, seat)
	return err
}

func GetGameCard(ctx context.Context, database *sql.DB, gameID, instanceID string) (*Card, error) {
	c := &Card{}
	var tappedInt, tappedManaInt int
	var cardData string
	err := database.QueryRowContext(ctx,
		`SELECT game_id, instance_id, card_name, card_data, owner_seat, zone, zone_position, tapped, tapped_for_mana_this_turn, revealed_to
		 FROM game_card WHERE game_id = ? AND instance_id = ?`, gameID, instanceID,
	).Scan(&c.GameID, &c.InstanceID, &c.Name, &cardData, &c.OwnerSeat, (*string)(&c.Zone), &c.ZonePosition, &tappedInt, &tappedManaInt, &c.RevealedTo)
	if err != nil {
		return nil, err
	}
	c.Tapped = tappedInt != 0
	c.TappedForManaThisTurn = tappedManaInt != 0
	hydrateCardData(c, cardData)
	return c, nil
}

func ListCardsInZone(ctx context.Context, database *sql.DB, gameID string, ownerSeat int, zone Zone) ([]*Card, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT game_id, instance_id, card_name, card_data, owner_seat, zone, zone_position, tapped, tapped_for_mana_this_turn, revealed_to
		 FROM game_card WHERE game_id = ? AND owner_seat = ? AND zone = ? ORDER BY zone_position`,
		gameID, ownerSeat, string(zone))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Card{}
	for rows.Next() {
		c := &Card{}
		var tappedInt, tappedManaInt int
		var cardData string
		if err := rows.Scan(&c.GameID, &c.InstanceID, &c.Name, &cardData, &c.OwnerSeat, (*string)(&c.Zone), &c.ZonePosition, &tappedInt, &tappedManaInt, &c.RevealedTo); err != nil {
			return nil, err
		}
		c.Tapped = tappedInt != 0
		c.TappedForManaThisTurn = tappedManaInt != 0
		hydrateCardData(c, cardData)
		out = append(out, c)
	}
	return out, rows.Err()
}

func CountCardsInZone(ctx context.Context, database *sql.DB, gameID string, ownerSeat int, zone Zone) (int, error) {
	var n int
	err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM game_card WHERE game_id = ? AND owner_seat = ? AND zone = ?`,
		gameID, ownerSeat, string(zone)).Scan(&n)
	return n, err
}

// ---------- TurnState ----------

func CreateTurnState(ctx context.Context, database *sql.DB, t *TurnState) error {
	_, err := database.ExecContext(ctx,
		`INSERT INTO game_turn (game_id, active_seat, phase, priority_seat, turn_number)
		 VALUES (?, ?, ?, ?, ?)`,
		t.GameID, t.ActiveSeat, string(t.Phase), t.PrioritySeat, t.TurnNumber)
	return err
}

func GetTurnState(ctx context.Context, database *sql.DB, gameID string) (*TurnState, error) {
	t := &TurnState{}
	err := database.QueryRowContext(ctx,
		`SELECT game_id, active_seat, phase, priority_seat, turn_number
		 FROM game_turn WHERE game_id = ?`, gameID,
	).Scan(&t.GameID, &t.ActiveSeat, (*string)(&t.Phase), &t.PrioritySeat, &t.TurnNumber)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func UpdateTurnState(ctx context.Context, database *sql.DB, t *TurnState) error {
	_, err := database.ExecContext(ctx,
		`UPDATE game_turn SET active_seat = ?, phase = ?, priority_seat = ?, turn_number = ?
		 WHERE game_id = ?`,
		t.ActiveSeat, string(t.Phase), t.PrioritySeat, t.TurnNumber, t.GameID)
	return err
}

func AppendActionLog(ctx context.Context, database *sql.DB, gameID string, seat *int, actionType, payloadJSON string) error {
	_, err := database.ExecContext(ctx,
		`INSERT INTO action_log (game_id, seat_position, timestamp, action_type, payload)
		 VALUES (?, ?, ?, ?, ?)`,
		gameID, seat, db.Now(), actionType, payloadJSON)
	return err
}

// ---------- helpers ----------

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func marshalCardData(c *Card) string {
	type staticData struct {
		ManaCost string   `json:"mana_cost,omitempty"`
		CMC      int      `json:"cmc"`
		Types    []string `json:"types,omitempty"`
		Subtypes []string `json:"subtypes,omitempty"`
	}
	sd := staticData{
		ManaCost: c.ManaCost,
		CMC:      c.CMC,
		Types:    c.Types,
		Subtypes: c.Subtypes,
	}
	b, _ := jsonMarshal(sd)
	return string(b)
}

func hydrateCardData(c *Card, raw string) {
	type staticData struct {
		ManaCost string   `json:"mana_cost,omitempty"`
		CMC      int      `json:"cmc"`
		Types    []string `json:"types,omitempty"`
		Subtypes []string `json:"subtypes,omitempty"`
	}
	var sd staticData
	if err := jsonUnmarshal([]byte(raw), &sd); err != nil {
		return
	}
	c.ManaCost = sd.ManaCost
	c.CMC = sd.CMC
	c.Types = sd.Types
	c.Subtypes = sd.Subtypes
}
