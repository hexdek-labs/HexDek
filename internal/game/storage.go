package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

// FinishGame marks a game finished and records the winner. A blank
// winnerDeviceID records a draw (CR §104.4 — e.g. simultaneous §704.5b
// empty-library losses); persisted as NULL to satisfy the
// winner_device_id → device(id) FK constraint.
func FinishGame(ctx context.Context, database *sql.DB, gameID string, winnerDeviceID string) error {
	var winner any
	if winnerDeviceID == "" {
		winner = nil
	} else {
		winner = winnerDeviceID
	}
	_, err := database.ExecContext(ctx,
		`UPDATE game SET finished_at = ?, winner_device_id = ? WHERE id = ?`,
		db.Now(), winner, gameID)
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
	var attemptedEmptyDraw, winEffectTriggered int
	err := database.QueryRowContext(ctx,
		`SELECT game_id, seat_position, device_id, deck_id, life, poison_counters,
		        mana_pool_w, mana_pool_u, mana_pool_b, mana_pool_r, mana_pool_g, mana_pool_c,
		        lands_played_turn, attempted_empty_draw, win_effect_triggered
		 FROM game_player WHERE game_id = ? AND seat_position = ?`, gameID, seat,
	).Scan(&p.GameID, &p.SeatPosition, &p.DeviceID, &p.DeckID, &p.Life, &p.PoisonCounters,
		&p.ManaPoolW, &p.ManaPoolU, &p.ManaPoolB, &p.ManaPoolR, &p.ManaPoolG, &p.ManaPoolC,
		&p.LandsPlayedTurn, &attemptedEmptyDraw, &winEffectTriggered)
	if err != nil {
		return nil, err
	}
	p.AttemptedEmptyDraw = attemptedEmptyDraw != 0
	p.WinEffectTriggered = winEffectTriggered != 0
	return p, nil
}

func ListGamePlayers(ctx context.Context, database *sql.DB, gameID string) ([]*Player, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT game_id, seat_position, device_id, deck_id, life, poison_counters,
		        mana_pool_w, mana_pool_u, mana_pool_b, mana_pool_r, mana_pool_g, mana_pool_c,
		        lands_played_turn, attempted_empty_draw, win_effect_triggered
		 FROM game_player WHERE game_id = ? ORDER BY seat_position`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Player
	for rows.Next() {
		p := &Player{}
		var attemptedEmptyDraw, winEffectTriggered int
		if err := rows.Scan(&p.GameID, &p.SeatPosition, &p.DeviceID, &p.DeckID, &p.Life, &p.PoisonCounters,
			&p.ManaPoolW, &p.ManaPoolU, &p.ManaPoolB, &p.ManaPoolR, &p.ManaPoolG, &p.ManaPoolC,
			&p.LandsPlayedTurn, &attemptedEmptyDraw, &winEffectTriggered); err != nil {
			return nil, err
		}
		p.AttemptedEmptyDraw = attemptedEmptyDraw != 0
		p.WinEffectTriggered = winEffectTriggered != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

func UpdateGamePlayer(ctx context.Context, database *sql.DB, p *Player) error {
	_, err := database.ExecContext(ctx,
		`UPDATE game_player SET life = ?, poison_counters = ?,
		   mana_pool_w = ?, mana_pool_u = ?, mana_pool_b = ?,
		   mana_pool_r = ?, mana_pool_g = ?, mana_pool_c = ?,
		   lands_played_turn = ?, attempted_empty_draw = ?, win_effect_triggered = ?
		 WHERE game_id = ? AND seat_position = ?`,
		p.Life, p.PoisonCounters,
		p.ManaPoolW, p.ManaPoolU, p.ManaPoolB,
		p.ManaPoolR, p.ManaPoolG, p.ManaPoolC,
		p.LandsPlayedTurn, boolToInt(p.AttemptedEmptyDraw), boolToInt(p.WinEffectTriggered),
		p.GameID, p.SeatPosition)
	return err
}

// ---------- GameCard ----------

// CreateGameCard inserts a card row. When the caller hasn't supplied
// Power/Toughness (both zero, the "no gameplay-data provided" signal),
// the canonical CR-compliant base stats are hydrated from the cached
// oracle row — this is the "Moxfield is printing-data, oracle is
// gameplay-data" split per 7174n1c's r60 architecture call. Front-face
// selection picks CardFaces[0] for DFC/MDFC (Delver of Secrets // Insectile
// Aberration enters as 1/1, not 3/2). Cache miss leaves the card at 0/0
// and the combat fallback (1/1) applies.
//
// Hydration is cache-only (no Scryfall round-trip) so test setups that
// don't seed card_oracle aren't slowed down by network requests; explicit
// P/T on `c` always wins (token creation, mid-game stat overrides, test
// fixtures with hand-picked values).
func CreateGameCard(ctx context.Context, database *sql.DB, c *Card) error {
	if c.Power == 0 && c.Toughness == 0 {
		hydratePTFromOracle(ctx, database, c)
	}
	_, err := database.ExecContext(ctx,
		`INSERT INTO game_card (game_id, instance_id, card_name, card_data, owner_seat, zone, zone_position, tapped, revealed_to)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.GameID, c.InstanceID, c.Name, marshalCardData(c), c.OwnerSeat, string(c.Zone), c.ZonePosition, boolToInt(c.Tapped), c.RevealedTo)
	return err
}

// hydratePTFromOracle looks up the card by name in card_oracle and fills
// in c.Power / c.Toughness from the cache. For DFC/MDFC/split cards the
// top-level power/toughness columns are empty and the data lives under
// card_faces[0] (Scryfall convention) — we parse the JSON blob and pick
// the front face.
//
// Best-effort: any error (missing oracle row, malformed JSON, non-integer
// "*"/"X" P/T) silently leaves c.Power / c.Toughness at zero so the
// combat fallback path stands.
func hydratePTFromOracle(ctx context.Context, database *sql.DB, c *Card) {
	if database == nil || c == nil || c.Name == "" {
		return
	}
	key := strings.ToLower(strings.TrimSpace(c.Name))
	if key == "" {
		return
	}
	var powerStr, toughnessStr, cardFacesJSON string
	err := database.QueryRowContext(ctx,
		`SELECT power, toughness, card_faces FROM card_oracle WHERE name = ?`, key,
	).Scan(&powerStr, &toughnessStr, &cardFacesJSON)
	if err != nil {
		return
	}
	if powerStr != "" || toughnessStr != "" {
		c.Power = parsePTString(powerStr)
		c.Toughness = parsePTString(toughnessStr)
		return
	}
	// DFC / MDFC / split — front face lives at card_faces[0].
	if cardFacesJSON == "" {
		return
	}
	var faces []struct {
		Power     string `json:"power"`
		Toughness string `json:"toughness"`
	}
	if jerr := json.Unmarshal([]byte(cardFacesJSON), &faces); jerr != nil {
		return
	}
	if len(faces) == 0 {
		return
	}
	c.Power = parsePTString(faces[0].Power)
	c.Toughness = parsePTString(faces[0].Toughness)
}

// parsePTString collapses Scryfall's string P/T into an int. Empty,
// non-integer ("*", "X", "1+*"), and negative values all return 0 — the
// MVP combat layer treats 0/0 as missing and applies the 1/1 fallback,
// matching the documented contract.
func parsePTString(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
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
		ManaCost  string   `json:"mana_cost,omitempty"`
		CMC       int      `json:"cmc"`
		Power     int      `json:"power,omitempty"`
		Toughness int      `json:"toughness,omitempty"`
		Types     []string `json:"types,omitempty"`
		Subtypes  []string `json:"subtypes,omitempty"`
	}
	sd := staticData{
		ManaCost:  c.ManaCost,
		CMC:       c.CMC,
		Power:     c.Power,
		Toughness: c.Toughness,
		Types:     c.Types,
		Subtypes:  c.Subtypes,
	}
	b, _ := jsonMarshal(sd)
	return string(b)
}

func hydrateCardData(c *Card, raw string) {
	type staticData struct {
		ManaCost  string   `json:"mana_cost,omitempty"`
		CMC       int      `json:"cmc"`
		Power     int      `json:"power,omitempty"`
		Toughness int      `json:"toughness,omitempty"`
		Types     []string `json:"types,omitempty"`
		Subtypes  []string `json:"subtypes,omitempty"`
	}
	var sd staticData
	if err := jsonUnmarshal([]byte(raw), &sd); err != nil {
		return
	}
	c.ManaCost = sd.ManaCost
	c.CMC = sd.CMC
	c.Power = sd.Power
	c.Toughness = sd.Toughness
	c.Types = sd.Types
	c.Subtypes = sd.Subtypes
}
