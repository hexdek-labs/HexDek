package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type ELORecord struct {
	DeckKey   string
	Commander string
	Owner     string
	Rating    float64
	HexRating float64
	Games     int
	Wins      int
	Losses    int
	Delta     float64
	HexDelta  float64
	Bracket   int
	// TSMu / TSSigma are the raw TrueSkill skill estimate and uncertainty
	// (r62, report 09 C-2). Before these columns existed only the
	// collapsed conservative rating (mu-3*sigma) was persisted, so every
	// server restart reconstructed sigma at its sigma_0 default (400) and
	// the ladder was permanently stuck re-converging. TSSigma == 0 means
	// "row predates the migration" — loaders fall back to the legacy
	// reconstruction for those rows.
	TSMu    float64
	TSSigma float64
}

type GameRecord struct {
	GameID     int64
	StartedAt  int64
	FinishedAt int64
	Turns      int
	Winner     int
	WinnerName string
	EndReason  string
	// WinReason captures HOW the winner won (heimdall.ClassifyKill:
	// "win_combat", "win_commander", "win_poison", "win_mill",
	// "win_combo"). Orthogonal to EndReason (the game-end TYPE). Empty
	// for draws / unclassified games.
	WinReason string
	// Seed is the engine RNG seed for the game. Captured for replay /
	// anti-cheat (Phase 1: storage only, no verification yet). 0 means
	// the seed wasn't surfaced by the runner — treat as "unknown".
	Seed int64
}

type GameSeatRecord struct {
	GameID           int64
	Seat             int
	Commander        string
	DeckKey          string
	Life             int
	HandSize         int
	LibrarySize      int
	GYSize           int
	BFSize           int
	Lost             bool
	BattlefieldCards string // JSON array of card names on battlefield at game end
}

type CardWinStat struct {
	CardName      string
	Commander     string
	Games         int
	Wins          int
	OnBoardAtWin  int
	WinRate       float64
	BoardPresence float64
}

func LoadAllELO(ctx context.Context, db *sql.DB) ([]ELORecord, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, ts_mu, ts_sigma FROM showmatch_elo ORDER BY rating DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ELORecord
	for rows.Next() {
		var r ELORecord
		if err := rows.Scan(&r.DeckKey, &r.Commander, &r.Owner, &r.Rating, &r.HexRating, &r.Games, &r.Wins, &r.Losses, &r.Delta, &r.HexDelta, &r.Bracket, &r.TSMu, &r.TSSigma); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func UpsertELO(ctx context.Context, db *sql.DB, r ELORecord) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, ts_mu, ts_sigma, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		 ON CONFLICT(deck_key) DO UPDATE SET
		   commander=excluded.commander, owner=excluded.owner,
		   rating=excluded.rating, hex_rating=excluded.hex_rating,
		   games=excluded.games, wins=excluded.wins,
		   losses=excluded.losses, delta=excluded.delta, hex_delta=excluded.hex_delta,
		   bracket=excluded.bracket, ts_mu=excluded.ts_mu, ts_sigma=excluded.ts_sigma,
		   updated_at=excluded.updated_at`,
		r.DeckKey, r.Commander, r.Owner, r.Rating, r.HexRating, r.Games, r.Wins, r.Losses, r.Delta, r.HexDelta, r.Bracket, r.TSMu, r.TSSigma)
	return err
}

func BatchUpsertELO(ctx context.Context, sqlDB *sql.DB, records []ELORecord) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, ts_mu, ts_sigma, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		 ON CONFLICT(deck_key) DO UPDATE SET
		   commander=excluded.commander, owner=excluded.owner,
		   rating=excluded.rating, hex_rating=excluded.hex_rating,
		   games=excluded.games, wins=excluded.wins,
		   losses=excluded.losses, delta=excluded.delta, hex_delta=excluded.hex_delta,
		   bracket=excluded.bracket, ts_mu=excluded.ts_mu, ts_sigma=excluded.ts_sigma,
		   updated_at=excluded.updated_at`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, r.DeckKey, r.Commander, r.Owner, r.Rating, r.HexRating, r.Games, r.Wins, r.Losses, r.Delta, r.HexDelta, r.Bracket, r.TSMu, r.TSSigma); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func InsertGame(ctx context.Context, db *sql.DB, g GameRecord) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason, win_reason, rng_seed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		g.StartedAt, g.FinishedAt, g.Turns, g.Winner, g.WinnerName, g.EndReason, g.WinReason, g.Seed)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func PersistGameTx(ctx context.Context, sqlDB *sql.DB, g GameRecord, seats []GameSeatRecord) (int64, error) {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason, win_reason, rng_seed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		g.StartedAt, g.FinishedAt, g.Turns, g.Winner, g.WinnerName, g.EndReason, g.WinReason, g.Seed)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	gameID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("LastInsertId for showmatch_game: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO showmatch_game_seat (game_id, seat, commander, deck_key, life, hand_size, library_size, gy_size, bf_size, lost, battlefield_cards)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	defer stmt.Close()
	for _, s := range seats {
		lost := 0
		if s.Lost {
			lost = 1
		}
		bfCards := s.BattlefieldCards
		if bfCards == "" {
			bfCards = "[]"
		}
		if _, err := stmt.ExecContext(ctx, gameID, s.Seat, s.Commander, s.DeckKey, s.Life, s.HandSize, s.LibrarySize, s.GYSize, s.BFSize, lost, bfCards); err != nil {
			tx.Rollback()
			return 0, err
		}
	}
	return gameID, tx.Commit()
}

func LoadRecentGames(ctx context.Context, db *sql.DB, limit int) ([]GameRecord, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT game_id, started_at, finished_at, turns, winner, winner_name, end_reason, win_reason, rng_seed
		 FROM showmatch_game ORDER BY finished_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameRecord
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.GameID, &g.StartedAt, &g.FinishedAt, &g.Turns, &g.Winner, &g.WinnerName, &g.EndReason, &g.WinReason, &g.Seed); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// LoadGameByID fetches a single GameRecord by its primary key. Returns
// sql.ErrNoRows verbatim when the id is unknown so callers can map it
// to a 404. battlefield_cards and per-seat detail live on
// LoadGameSeats; this is the lightweight metadata-only fetch used by
// hexapi's /api/games/{id}/summary endpoint.
func LoadGameByID(ctx context.Context, db *sql.DB, gameID int64) (GameRecord, error) {
	var g GameRecord
	err := db.QueryRowContext(ctx,
		`SELECT game_id, started_at, finished_at, turns, winner, winner_name, end_reason, win_reason, rng_seed
		 FROM showmatch_game WHERE game_id = ?`, gameID).Scan(
		&g.GameID, &g.StartedAt, &g.FinishedAt, &g.Turns, &g.Winner, &g.WinnerName, &g.EndReason, &g.WinReason, &g.Seed)
	return g, err
}

func LoadGameSeats(ctx context.Context, db *sql.DB, gameID int64) ([]GameSeatRecord, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT game_id, seat, commander, life, hand_size, library_size, gy_size, bf_size, lost
		 FROM showmatch_game_seat WHERE game_id = ? ORDER BY seat`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameSeatRecord
	for rows.Next() {
		var s GameSeatRecord
		var lost int
		if err := rows.Scan(&s.GameID, &s.Seat, &s.Commander, &s.Life, &s.HandSize, &s.LibrarySize, &s.GYSize, &s.BFSize, &lost); err != nil {
			return nil, err
		}
		s.Lost = lost == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

func CountGames(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM showmatch_game`).Scan(&count)
	return count, err
}

func GetTotalTurns(ctx context.Context, db *sql.DB) (int, error) {
	var total sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT SUM(turns) FROM showmatch_game`).Scan(&total)
	if err != nil {
		return 0, err
	}
	return int(total.Int64), nil
}

func GetCommanderStats(ctx context.Context, db *sql.DB, commander string) (games, wins int, err error) {
	err = db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(games),0), COALESCE(SUM(wins),0) FROM showmatch_elo WHERE commander = ?`, commander).Scan(&games, &wins)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	return
}

func BatchUpsertCardWinStats(ctx context.Context, sqlDB *sql.DB, stats []CardWinStat) error {
	if len(stats) == 0 {
		return nil
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO card_win_stats (card_name, commander, games, wins, on_board_at_win, updated_at)
		 VALUES (?, ?, 1, ?, ?, unixepoch())
		 ON CONFLICT(card_name, commander) DO UPDATE SET
		   games = games + 1,
		   wins = wins + excluded.wins,
		   on_board_at_win = on_board_at_win + excluded.on_board_at_win,
		   updated_at = excluded.updated_at`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, s := range stats {
		if _, err := stmt.ExecContext(ctx, s.CardName, s.Commander, s.Wins, s.OnBoardAtWin); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func KVGet(ctx context.Context, db *sql.DB, key string) (string, error) {
	var val string
	err := db.QueryRowContext(ctx, `SELECT value FROM kv_store WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func KVSet(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO kv_store (key, value, updated_at) VALUES (?, ?, unixepoch())
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value)
	return err
}

func LoadCardWinStats(ctx context.Context, db *sql.DB, commander string, limit int) ([]CardWinStat, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT card_name, commander, games, wins, on_board_at_win
		 FROM card_win_stats
		 WHERE commander = ? AND games >= 10
		 ORDER BY CAST(wins AS REAL) / CAST(games AS REAL) DESC
		 LIMIT ?`, commander, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CardWinStat
	for rows.Next() {
		var s CardWinStat
		if err := rows.Scan(&s.CardName, &s.Commander, &s.Games, &s.Wins, &s.OnBoardAtWin); err != nil {
			return nil, err
		}
		if s.Games > 0 {
			s.WinRate = float64(s.Wins) / float64(s.Games)
			s.BoardPresence = float64(s.OnBoardAtWin) / float64(s.Wins)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type OwnerStats struct {
	Owner   string  `json:"owner"`
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	Losses  int     `json:"losses"`
	WinRate float64 `json:"win_rate"`
}

func LoadOwnerStats(ctx context.Context, sqlDB *sql.DB, owner string) (OwnerStats, error) {
	var s OwnerStats
	s.Owner = owner
	err := sqlDB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(games),0), COALESCE(SUM(wins),0), COALESCE(SUM(losses),0)
		 FROM showmatch_elo WHERE owner = ?`, owner).Scan(&s.Games, &s.Wins, &s.Losses)
	if err != nil {
		return s, err
	}
	if s.Games > 0 {
		s.WinRate = float64(s.Wins) / float64(s.Games) * 100
	}
	return s, nil
}

type OwnerGameRow struct {
	GameID      int64    `json:"game_id"`
	FinishedAt  int64    `json:"finished_at"`
	Turns       int      `json:"turns"`
	Winner      int      `json:"winner"`
	WinnerName  string   `json:"winner_name"`
	EndReason   string   `json:"end_reason"`
	MySeat      int      `json:"my_seat"`
	MyCommander string   `json:"my_commander"`
	MyDeckKey   string   `json:"my_deck_key,omitempty"`
	Opponents   []string `json:"opponents"`
}

func LoadOwnerGames(ctx context.Context, sqlDB *sql.DB, owner string, limit int) ([]OwnerGameRow, error) {
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT g.game_id, g.finished_at, g.turns, g.winner, g.winner_name, g.end_reason, me.seat, me.commander, me.deck_key
		 FROM showmatch_game g
		 JOIN showmatch_game_seat me ON me.game_id = g.game_id
		 JOIN showmatch_elo e ON e.deck_key = me.deck_key AND e.owner = ?
		 ORDER BY g.finished_at DESC LIMIT ?`, owner, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var games []OwnerGameRow
	for rows.Next() {
		var g OwnerGameRow
		if err := rows.Scan(&g.GameID, &g.FinishedAt, &g.Turns, &g.Winner, &g.WinnerName, &g.EndReason, &g.MySeat, &g.MyCommander, &g.MyDeckKey); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range games {
		oppRows, qerr := sqlDB.QueryContext(ctx,
			`SELECT commander FROM showmatch_game_seat WHERE game_id = ? AND seat != ? ORDER BY seat`,
			games[i].GameID, games[i].MySeat)
		if qerr != nil {
			return nil, fmt.Errorf("load opponents for game %d: %w", games[i].GameID, qerr)
		}
		for oppRows.Next() {
			var c string
			if serr := oppRows.Scan(&c); serr != nil {
				oppRows.Close()
				return nil, fmt.Errorf("scan opponent for game %d: %w", games[i].GameID, serr)
			}
			games[i].Opponents = append(games[i].Opponents, c)
		}
		if rerr := oppRows.Err(); rerr != nil {
			oppRows.Close()
			return nil, fmt.Errorf("opponents rows.Err for game %d: %w", games[i].GameID, rerr)
		}
		oppRows.Close()
	}
	return games, nil
}

// DeckGameOutcome is one row of a deck's game history reduced to the
// signals heimdall.ComputeDeckVersionTrends needs: when the game
// finished and whether this deck won, lost, or drew it. Used by the
// /api/decks/{owner}/{id}/version-trends endpoint to bucket games
// into version eras without re-loading the full GameRecord per row.
type DeckGameOutcome struct {
	FinishedAt int64 `json:"finished_at"`
	Won        bool  `json:"won"`
	Draw       bool  `json:"draw"`
}

// LoadDeckGameOutcomes returns one DeckGameOutcome per game in which
// deckKey participated, ordered by finished_at ascending so the
// version-trend walker can advance through versions in lockstep with
// games.
//
// Won = (winner == me.seat); Draw = (winner < 0). The remaining case
// is a loss, computed by subtraction at aggregation time. No paging:
// the caller is expected to operate on the full per-deck history.
func LoadDeckGameOutcomes(ctx context.Context, db *sql.DB, deckKey string) ([]DeckGameOutcome, error) {
	if deckKey == "" {
		return []DeckGameOutcome{}, nil
	}
	rows, err := db.QueryContext(ctx,
		`SELECT g.finished_at, g.winner, me.seat
		 FROM showmatch_game_seat me
		 JOIN showmatch_game g ON g.game_id = me.game_id
		 WHERE me.deck_key = ?
		 ORDER BY g.finished_at ASC`, deckKey)
	if err != nil {
		return nil, fmt.Errorf("deck outcomes query: %w", err)
	}
	defer rows.Close()
	var out []DeckGameOutcome
	for rows.Next() {
		var fin int64
		var winner, seat int
		if err := rows.Scan(&fin, &winner, &seat); err != nil {
			return nil, err
		}
		out = append(out, DeckGameOutcome{
			FinishedAt: fin,
			Won:        winner == seat,
			Draw:       winner < 0,
		})
	}
	return out, rows.Err()
}

// DeckGameHistoryRow is one game-row for a single deck_key, joined
// with the game metadata (turns, winner, end_reason) and the seat the
// deck played. Richer than DeckGameOutcome (which is purpose-built for
// the version-trend bucketer and only needs finished_at + won + draw);
// surfaced by /api/decks/{owner}/{id}/history so the frontend can plot
// per-game performance over time.
type DeckGameHistoryRow struct {
	GameID     int64  `json:"game_id"`
	FinishedAt int64  `json:"finished_at"`
	Turns      int    `json:"turns"`
	Seat       int    `json:"seat"`
	Won        bool   `json:"won"`
	Draw       bool   `json:"draw"`
	EndReason  string `json:"end_reason,omitempty"`
}

// LoadDeckGameHistory returns up to limit games (newest first) where
// deckKey participated. limit==0 returns nothing — the caller is
// responsible for picking a sane cap since this query grows linearly
// with deck game-count and the endpoint surface walks every row at
// aggregation time. Empty deckKey returns an empty slice without
// hitting the DB.
func LoadDeckGameHistory(ctx context.Context, sqlDB *sql.DB, deckKey string, limit int) ([]DeckGameHistoryRow, error) {
	if deckKey == "" || limit <= 0 {
		return []DeckGameHistoryRow{}, nil
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT g.game_id, g.finished_at, g.turns, g.winner, g.end_reason, me.seat
		 FROM showmatch_game_seat me
		 JOIN showmatch_game g ON g.game_id = me.game_id
		 WHERE me.deck_key = ?
		 ORDER BY g.finished_at DESC, g.game_id DESC
		 LIMIT ?`, deckKey, limit)
	if err != nil {
		return nil, fmt.Errorf("deck game history query: %w", err)
	}
	defer rows.Close()
	out := []DeckGameHistoryRow{}
	for rows.Next() {
		var (
			gid, fin            int64
			turns, winner, seat int
			endReason           string
		)
		if err := rows.Scan(&gid, &fin, &turns, &winner, &endReason, &seat); err != nil {
			return nil, err
		}
		out = append(out, DeckGameHistoryRow{
			GameID:     gid,
			FinishedAt: fin,
			Turns:      turns,
			Seat:       seat,
			Won:        winner == seat,
			Draw:       winner < 0,
			EndReason:  endReason,
		})
	}
	return out, rows.Err()
}

// GameSeatGrouped is one game's worth of seat outcomes, used by the
// archetype-vs-archetype matrix builder. Within a single game we need
// every (this-seat, that-seat) pair to count toward the matrix, so
// the natural shape is "all seats grouped by game" — flat per-seat
// rows would require the caller to re-group.
type GameSeatGrouped struct {
	GameID     int64
	FinishedAt int64
	Seats      []GameSeatGroupedSeat
}

// GameSeatGroupedSeat is one seat row within a GameSeatGrouped.
type GameSeatGroupedSeat struct {
	DeckKey string
	Won     bool
}

// LoadGameSeatGroups returns games (with their seat lists) whose
// finished_at ≥ sinceUnix, sorted newest-first. limit caps the
// number of returned games; 0 means no cap. Only seats with a
// non-empty deck_key are included — the matrix can't bucket a seat
// whose archetype can't be resolved.
func LoadGameSeatGroups(ctx context.Context, sqlDB *sql.DB, sinceUnix int64, limit int) ([]GameSeatGrouped, error) {
	q := `SELECT g.game_id, g.finished_at, s.deck_key, (CASE WHEN g.winner = s.seat THEN 1 ELSE 0 END) AS won
	      FROM showmatch_game g
	      JOIN showmatch_game_seat s ON s.game_id = g.game_id
	      WHERE g.finished_at >= ? AND s.deck_key != ''
	      ORDER BY g.finished_at DESC, g.game_id DESC, s.seat ASC`
	args := []any{sinceUnix}
	rows, err := sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("game seat groups query: %w", err)
	}
	defer rows.Close()

	out := []GameSeatGrouped{}
	var cur *GameSeatGrouped
	for rows.Next() {
		var gid, fin int64
		var deckKey string
		var won int
		if err := rows.Scan(&gid, &fin, &deckKey, &won); err != nil {
			return nil, err
		}
		if cur == nil || cur.GameID != gid {
			if limit > 0 && len(out) >= limit {
				break
			}
			out = append(out, GameSeatGrouped{GameID: gid, FinishedAt: fin})
			cur = &out[len(out)-1]
		}
		cur.Seats = append(cur.Seats, GameSeatGroupedSeat{
			DeckKey: deckKey,
			Won:     won == 1,
		})
	}
	return out, rows.Err()
}

// HeadToHeadGame is one game where both ownerA and ownerB had seats.
// Same shape regardless of which owner won (the caller compares
// Winner to SeatA / SeatB to decide). Used by /api/players/compare.
type HeadToHeadGame struct {
	GameID     int64
	FinishedAt int64
	Winner     int // -1 = draw
	SeatA      int
	SeatB      int
}

// LoadPlayerHeadToHead returns up to limit games (newest first) in
// which BOTH ownerA and ownerB had a seat. The query joins
// showmatch_game_seat to itself via game_id and filters each side
// against showmatch_elo to get the owner — same join pattern as
// LoadOwnerGames so the existing idx_showmatch_elo_owner index is
// hit on both legs.
//
// Dedupe by game_id (one row per unique game), keeping the first
// (sa.seat, sb.seat) pair observed — the caller doesn't need every
// (A-deck × B-deck) cartesian row when both players brought multiple
// decks to the same game. limit==0 means no cap.
func LoadPlayerHeadToHead(ctx context.Context, sqlDB *sql.DB, ownerA, ownerB string, limit int) ([]HeadToHeadGame, error) {
	if ownerA == "" || ownerB == "" || ownerA == ownerB {
		return []HeadToHeadGame{}, nil
	}
	q := `SELECT g.game_id, g.finished_at, g.winner, sa.seat, sb.seat
	      FROM showmatch_game_seat sa
	      JOIN showmatch_game_seat sb ON sb.game_id = sa.game_id AND sb.seat != sa.seat
	      JOIN showmatch_game g ON g.game_id = sa.game_id
	      JOIN showmatch_elo ea ON ea.deck_key = sa.deck_key AND ea.owner = ?
	      JOIN showmatch_elo eb ON eb.deck_key = sb.deck_key AND eb.owner = ?
	      ORDER BY g.finished_at DESC`
	args := []any{ownerA, ownerB}
	if limit > 0 {
		// Pull limit*4 worth of raw rows; dedupe will collapse them.
		// 4 covers the worst case where each player has 4 decks in
		// a multi-seat Commander game (highly unlikely but bounded).
		q += " LIMIT ?"
		args = append(args, limit*4)
	}
	rows, err := sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("head-to-head query: %w", err)
	}
	defer rows.Close()
	seen := make(map[int64]bool)
	var out []HeadToHeadGame
	for rows.Next() {
		var g HeadToHeadGame
		if err := rows.Scan(&g.GameID, &g.FinishedAt, &g.Winner, &g.SeatA, &g.SeatB); err != nil {
			return nil, err
		}
		if seen[g.GameID] {
			continue
		}
		seen[g.GameID] = true
		out = append(out, g)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

// MetaSeatOutcome is one (game, seat) row reduced to the fields the
// meta-trends endpoint needs: when the game finished, which deck the
// seat piloted, and whether that seat won. The caller resolves
// DeckKey → archetype via the freya strategy-file lookup.
type MetaSeatOutcome struct {
	FinishedAt int64
	DeckKey    string
	Won        bool
}

// LoadMetaSeatOutcomes returns every (game, seat) row whose game
// finished at or after sinceUnix, with deck_key set. Rows are
// returned newest-first so the caller can stop early once it crosses
// the requested window without buffering the full result. The
// deck_key=” filter drops seats from games that predate the
// deck_key backfill (BackfillDeckKeys); without a deck_key we can't
// resolve the seat's archetype, so the row contributes nothing.
func LoadMetaSeatOutcomes(ctx context.Context, sqlDB *sql.DB, sinceUnix int64) ([]MetaSeatOutcome, error) {
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT g.finished_at, s.deck_key, (CASE WHEN g.winner = s.seat THEN 1 ELSE 0 END) AS won
		 FROM showmatch_game_seat s
		 JOIN showmatch_game g ON g.game_id = s.game_id
		 WHERE g.finished_at >= ? AND s.deck_key != ''
		 ORDER BY g.finished_at DESC`, sinceUnix)
	if err != nil {
		return nil, fmt.Errorf("meta seat outcomes query: %w", err)
	}
	defer rows.Close()
	var out []MetaSeatOutcome
	for rows.Next() {
		var o MetaSeatOutcome
		var won int
		if err := rows.Scan(&o.FinishedAt, &o.DeckKey, &won); err != nil {
			return nil, err
		}
		o.Won = won == 1
		out = append(out, o)
	}
	return out, rows.Err()
}

func BackfillDeckKeys(ctx context.Context, sqlDB *sql.DB) (int64, error) {
	res, err := sqlDB.ExecContext(ctx,
		`UPDATE showmatch_game_seat SET deck_key = (
			SELECT e.deck_key FROM showmatch_elo e
			WHERE e.commander = showmatch_game_seat.commander AND e.owner != '' AND e.deck_key != ''
			LIMIT 1
		) WHERE deck_key = '' AND commander != ''`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
