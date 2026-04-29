package db

import (
	"context"
	"database/sql"
)

type ELORecord struct {
	DeckKey   string
	Commander string
	Owner     string
	Rating    float64
	Games     int
	Wins      int
	Losses    int
	Delta     float64
}

type GameRecord struct {
	GameID     int64
	StartedAt  int64
	FinishedAt int64
	Turns      int
	Winner     int
	WinnerName string
	EndReason  string
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
		`SELECT deck_key, commander, owner, rating, games, wins, losses, delta FROM showmatch_elo ORDER BY rating DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ELORecord
	for rows.Next() {
		var r ELORecord
		if err := rows.Scan(&r.DeckKey, &r.Commander, &r.Owner, &r.Rating, &r.Games, &r.Wins, &r.Losses, &r.Delta); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func UpsertELO(ctx context.Context, db *sql.DB, r ELORecord) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO showmatch_elo (deck_key, commander, owner, rating, games, wins, losses, delta, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		 ON CONFLICT(deck_key) DO UPDATE SET
		   commander=excluded.commander, owner=excluded.owner,
		   rating=excluded.rating, games=excluded.games, wins=excluded.wins,
		   losses=excluded.losses, delta=excluded.delta, updated_at=excluded.updated_at`,
		r.DeckKey, r.Commander, r.Owner, r.Rating, r.Games, r.Wins, r.Losses, r.Delta)
	return err
}

func BatchUpsertELO(ctx context.Context, sqlDB *sql.DB, records []ELORecord) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO showmatch_elo (deck_key, commander, owner, rating, games, wins, losses, delta, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		 ON CONFLICT(deck_key) DO UPDATE SET
		   commander=excluded.commander, owner=excluded.owner,
		   rating=excluded.rating, games=excluded.games, wins=excluded.wins,
		   losses=excluded.losses, delta=excluded.delta, updated_at=excluded.updated_at`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, r.DeckKey, r.Commander, r.Owner, r.Rating, r.Games, r.Wins, r.Losses, r.Delta); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func InsertGame(ctx context.Context, db *sql.DB, g GameRecord) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		g.StartedAt, g.FinishedAt, g.Turns, g.Winner, g.WinnerName, g.EndReason)
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
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		g.StartedAt, g.FinishedAt, g.Turns, g.Winner, g.WinnerName, g.EndReason)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	gameID, _ := res.LastInsertId()
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
		`SELECT game_id, started_at, finished_at, turns, winner, winner_name, end_reason
		 FROM showmatch_game ORDER BY finished_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameRecord
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.GameID, &g.StartedAt, &g.FinishedAt, &g.Turns, &g.Winner, &g.WinnerName, &g.EndReason); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
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
