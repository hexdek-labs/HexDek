package db

import (
	"context"
	"database/sql"
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
}

type GameRecord struct {
	GameID     int64
	StartedAt  int64
	FinishedAt int64
	Turns      int
	Winner     int
	WinnerName string
	EndReason  string
	// Seed is the engine RNG seed for the game. Captured for replay /
	// anti-cheat (Phase 1: storage only, no verification yet). 0 means
	// the seed wasn't surfaced by the runner — treat as "unknown".
	Seed int64
}

type GameSeatRecord struct {
	GameID          int64
	Seat            int
	Commander       string
	DeckKey         string
	Life            int
	HandSize        int
	LibrarySize     int
	GYSize          int
	BFSize          int
	Lost            bool
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
		`SELECT deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket FROM showmatch_elo ORDER BY rating DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ELORecord
	for rows.Next() {
		var r ELORecord
		if err := rows.Scan(&r.DeckKey, &r.Commander, &r.Owner, &r.Rating, &r.HexRating, &r.Games, &r.Wins, &r.Losses, &r.Delta, &r.HexDelta, &r.Bracket); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func UpsertELO(ctx context.Context, db *sql.DB, r ELORecord) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		 ON CONFLICT(deck_key) DO UPDATE SET
		   commander=excluded.commander, owner=excluded.owner,
		   rating=excluded.rating, hex_rating=excluded.hex_rating,
		   games=excluded.games, wins=excluded.wins,
		   losses=excluded.losses, delta=excluded.delta, hex_delta=excluded.hex_delta,
		   bracket=excluded.bracket, updated_at=excluded.updated_at`,
		r.DeckKey, r.Commander, r.Owner, r.Rating, r.HexRating, r.Games, r.Wins, r.Losses, r.Delta, r.HexDelta, r.Bracket)
	return err
}

func BatchUpsertELO(ctx context.Context, sqlDB *sql.DB, records []ELORecord) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		 ON CONFLICT(deck_key) DO UPDATE SET
		   commander=excluded.commander, owner=excluded.owner,
		   rating=excluded.rating, hex_rating=excluded.hex_rating,
		   games=excluded.games, wins=excluded.wins,
		   losses=excluded.losses, delta=excluded.delta, hex_delta=excluded.hex_delta,
		   bracket=excluded.bracket, updated_at=excluded.updated_at`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, r.DeckKey, r.Commander, r.Owner, r.Rating, r.HexRating, r.Games, r.Wins, r.Losses, r.Delta, r.HexDelta, r.Bracket); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func InsertGame(ctx context.Context, db *sql.DB, g GameRecord) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason, rng_seed)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.StartedAt, g.FinishedAt, g.Turns, g.Winner, g.WinnerName, g.EndReason, g.Seed)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func InsertGameSeat(ctx context.Context, db *sql.DB, s GameSeatRecord) error {
	lost := 0
	if s.Lost {
		lost = 1
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO showmatch_game_seat (game_id, seat, commander, life, hand_size, library_size, gy_size, bf_size, lost)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.GameID, s.Seat, s.Commander, s.Life, s.HandSize, s.LibrarySize, s.GYSize, s.BFSize, lost)
	return err
}

func PersistGameTx(ctx context.Context, sqlDB *sql.DB, g GameRecord, seats []GameSeatRecord) (int64, error) {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason, rng_seed)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.StartedAt, g.FinishedAt, g.Turns, g.Winner, g.WinnerName, g.EndReason, g.Seed)
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
		`SELECT game_id, started_at, finished_at, turns, winner, winner_name, end_reason, rng_seed
		 FROM showmatch_game ORDER BY finished_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameRecord
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.GameID, &g.StartedAt, &g.FinishedAt, &g.Turns, &g.Winner, &g.WinnerName, &g.EndReason, &g.Seed); err != nil {
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
		`SELECT game_id, started_at, finished_at, turns, winner, winner_name, end_reason, rng_seed
		 FROM showmatch_game WHERE game_id = ?`, gameID).Scan(
		&g.GameID, &g.StartedAt, &g.FinishedAt, &g.Turns, &g.Winner, &g.WinnerName, &g.EndReason, &g.Seed)
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
	if err == sql.ErrNoRows {
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
	if err == sql.ErrNoRows {
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
