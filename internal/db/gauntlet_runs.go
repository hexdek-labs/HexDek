// Package db — gauntlet_runs persistence.
//
// One row per completed gauntlet captures the rating trajectory for the
// deck under test. Drives the deck-page ELO history chart so players see
// the calibration arc, not just the current rating snapshot.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GauntletRunRecord is one persistent gauntlet-completion entry.
// Fields mirror the schema in schema.sql (gauntlet_runs).
type GauntletRunRecord struct {
	RunID      int64     `json:"run_id"`
	DeckKey    string    `json:"deck_key"`
	Commander  string    `json:"commander"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Games      int       `json:"games"`
	Wins       int       `json:"wins"`
	Losses     int       `json:"losses"`
	WinRate    float64   `json:"win_rate"`
	ELOStart   float64   `json:"elo_start"`
	ELOEnd     float64   `json:"elo_end"`
	ELODelta   float64   `json:"elo_delta"`
	AvgTurns   float64   `json:"avg_turns"`
	// Per-position counts for the deck under test (seat 0).
	// Index meaning: [1st, 2nd, 3rd, 4th].
	Place1st int `json:"place_1st"`
	Place2nd int `json:"place_2nd"`
	Place3rd int `json:"place_3rd"`
	Place4th int `json:"place_4th"`
}

// InsertGauntletRun writes a completed-gauntlet snapshot.
// Idempotent only by autoincrement run_id — caller is responsible for
// not double-inserting on retry. Safe to call from the gauntlet
// finalization path.
func InsertGauntletRun(ctx context.Context, sqlDB *sql.DB, rec GauntletRunRecord) error {
	if sqlDB == nil {
		return nil
	}
	_, err := sqlDB.ExecContext(ctx, `
		INSERT INTO gauntlet_runs (
			deck_key, commander, started_at, finished_at,
			games, wins, losses, win_rate,
			elo_start, elo_end, elo_delta, avg_turns,
			place_1st, place_2nd, place_3rd, place_4th
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.DeckKey, rec.Commander,
		rec.StartedAt.Unix(), rec.FinishedAt.Unix(),
		rec.Games, rec.Wins, rec.Losses, rec.WinRate,
		rec.ELOStart, rec.ELOEnd, rec.ELODelta, rec.AvgTurns,
		rec.Place1st, rec.Place2nd, rec.Place3rd, rec.Place4th,
	)
	return err
}

// InsertPendingGauntletRun reserves a gauntlet_runs row at the
// START of a gauntlet — before any games have been played. The
// caller is responsible for calling FinalizeGauntletRun once the
// run completes to write the final aggregate counters. The row is
// inserted with Games=0 / Wins=0 / Losses=0 / finished_at=started_at
// as placeholders.
//
// Returns the new run_id so the caller can link per-game
// showmatch_game rows back via InsertGauntletRunGame as they're
// persisted.
func InsertPendingGauntletRun(ctx context.Context, sqlDB *sql.DB, deckKey, commander string, startedAt time.Time) (int64, error) {
	if sqlDB == nil {
		return 0, nil
	}
	res, err := sqlDB.ExecContext(ctx, `
		INSERT INTO gauntlet_runs (
			deck_key, commander, started_at, finished_at,
			games, wins, losses, win_rate,
			elo_start, elo_end, elo_delta, avg_turns,
			place_1st, place_2nd, place_3rd, place_4th
		) VALUES (?, ?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)`,
		deckKey, commander, startedAt.Unix(), startedAt.Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinalizeGauntletRun updates a pending row inserted via
// InsertPendingGauntletRun with the final aggregate counters. The
// caller passes the same GauntletRunRecord shape InsertGauntletRun
// expects; RunID identifies which row to overwrite. Useful for the
// gauntlet-replay-archive pattern where the row is reserved at
// start so per-game links have a valid foreign key target.
func FinalizeGauntletRun(ctx context.Context, sqlDB *sql.DB, rec GauntletRunRecord) error {
	if sqlDB == nil {
		return nil
	}
	if rec.RunID <= 0 {
		return fmt.Errorf("FinalizeGauntletRun: RunID must be > 0")
	}
	_, err := sqlDB.ExecContext(ctx, `
		UPDATE gauntlet_runs SET
			deck_key = ?, commander = ?,
			started_at = ?, finished_at = ?,
			games = ?, wins = ?, losses = ?, win_rate = ?,
			elo_start = ?, elo_end = ?, elo_delta = ?, avg_turns = ?,
			place_1st = ?, place_2nd = ?, place_3rd = ?, place_4th = ?
		WHERE run_id = ?`,
		rec.DeckKey, rec.Commander,
		rec.StartedAt.Unix(), rec.FinishedAt.Unix(),
		rec.Games, rec.Wins, rec.Losses, rec.WinRate,
		rec.ELOStart, rec.ELOEnd, rec.ELODelta, rec.AvgTurns,
		rec.Place1st, rec.Place2nd, rec.Place3rd, rec.Place4th,
		rec.RunID,
	)
	return err
}

// InsertGauntletRunGame appends one (gauntlet_id, game_id) link
// to the gauntlet_run_game junction. gameIndex is the 0-based
// ordinal within the gauntlet so consumers can replay games in
// sequence even when database insertion order drifts.
//
// Idempotent on the primary key (gauntlet_id, game_id) — re-runs
// of the same insert silently no-op via INSERT OR IGNORE so
// retries don't blow up.
func InsertGauntletRunGame(ctx context.Context, sqlDB *sql.DB, gauntletID, gameID int64, gameIndex int) error {
	if sqlDB == nil {
		return nil
	}
	_, err := sqlDB.ExecContext(ctx,
		`INSERT OR IGNORE INTO gauntlet_run_game (gauntlet_id, game_id, game_index) VALUES (?, ?, ?)`,
		gauntletID, gameID, gameIndex)
	return err
}

// LoadGauntletRunGames returns the showmatch_game.game_id values
// linked to a gauntlet run, in their original gauntlet sequence
// (game_index ASC). Used by the tournament-stats endpoint when
// the explicit junction is preferred over the time-window
// heuristic, and by future replay-archive consumers that need to
// re-analyze a gauntlet's per-game data.
//
// Returns an empty slice when the gauntlet pre-dates the junction
// table (older runs were only stored as aggregate rows).
func LoadGauntletRunGames(ctx context.Context, sqlDB *sql.DB, gauntletID int64) ([]int64, error) {
	if sqlDB == nil {
		return nil, nil
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT game_id FROM gauntlet_run_game WHERE gauntlet_id = ? ORDER BY game_index ASC, game_id ASC`,
		gauntletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// LoadGauntletRunByID fetches one gauntlet run by its autoincrement
// run_id. Returns sql.ErrNoRows verbatim when the id is unknown so
// callers can map it to a 404. Used by the tournament-stats endpoint
// to anchor the (deck_key, started_at, finished_at) tuple that
// identifies the per-game rows belonging to this tournament.
func LoadGauntletRunByID(ctx context.Context, sqlDB *sql.DB, runID int64) (GauntletRunRecord, error) {
	var r GauntletRunRecord
	if sqlDB == nil {
		return r, sql.ErrNoRows
	}
	var started, finished int64
	err := sqlDB.QueryRowContext(ctx,
		`SELECT run_id, deck_key, commander, started_at, finished_at,
		        games, wins, losses, win_rate,
		        elo_start, elo_end, elo_delta, avg_turns,
		        place_1st, place_2nd, place_3rd, place_4th
		 FROM gauntlet_runs WHERE run_id = ?`, runID).Scan(
		&r.RunID, &r.DeckKey, &r.Commander, &started, &finished,
		&r.Games, &r.Wins, &r.Losses, &r.WinRate,
		&r.ELOStart, &r.ELOEnd, &r.ELODelta, &r.AvgTurns,
		&r.Place1st, &r.Place2nd, &r.Place3rd, &r.Place4th,
	)
	if err != nil {
		return r, err
	}
	r.StartedAt = time.Unix(started, 0)
	r.FinishedAt = time.Unix(finished, 0)
	return r, nil
}

// TournamentGameRow is one game row scoped to a tournament window,
// returned by LoadTournamentGames. Carries the signals heimdall's
// ComputeTournamentStats consumes — game length, win/loss, and
// opponent commanders for the deck-distribution rollup.
type TournamentGameRow struct {
	GameID      int64    `json:"game_id"`
	FinishedAt  int64    `json:"finished_at"`
	Turns       int      `json:"turns"`
	Won         bool     `json:"won"`
	Draw        bool     `json:"draw"`
	Opponents   []string `json:"opponents"`
}

// LoadTournamentGames returns one TournamentGameRow per
// showmatch_game in which the deck under test played AND whose
// finished_at falls in the gauntlet window. The window-based filter
// is necessary because showmatch_game has no tournament_id foreign
// key — the (deck_key, time-window) tuple is the canonical join.
//
// Opponent commanders are loaded per game with a follow-up query
// (one extra round-trip per game, same shape as LoadOwnerGames).
// Bounded by the gauntlet's game count (typically 100-500) so the
// extra round-trips are reasonable.
func LoadTournamentGames(ctx context.Context, sqlDB *sql.DB, deckKey string, startedAt, finishedAt int64) ([]TournamentGameRow, error) {
	if sqlDB == nil || deckKey == "" {
		return []TournamentGameRow{}, nil
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT g.game_id, g.finished_at, g.turns, g.winner, me.seat
		 FROM showmatch_game_seat me
		 JOIN showmatch_game g ON g.game_id = me.game_id
		 WHERE me.deck_key = ?
		   AND g.finished_at >= ?
		   AND g.finished_at <= ?
		 ORDER BY g.finished_at ASC`, deckKey, startedAt, finishedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TournamentGameRow
	for rows.Next() {
		var r TournamentGameRow
		var winner, seat int
		if err := rows.Scan(&r.GameID, &r.FinishedAt, &r.Turns, &winner, &seat); err != nil {
			return nil, err
		}
		r.Won = winner == seat
		r.Draw = winner < 0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		oppRows, qerr := sqlDB.QueryContext(ctx,
			`SELECT commander FROM showmatch_game_seat
			 WHERE game_id = ? AND deck_key != ? ORDER BY seat`,
			out[i].GameID, deckKey)
		if qerr != nil {
			return nil, qerr
		}
		for oppRows.Next() {
			var c string
			if serr := oppRows.Scan(&c); serr != nil {
				oppRows.Close()
				return nil, serr
			}
			out[i].Opponents = append(out[i].Opponents, c)
		}
		oppRows.Close()
	}
	return out, nil
}

// LoadGauntletRuns returns the most recent N runs for a deck, newest
// first. Limit ≤ 0 returns all rows for the deck. Caller renders the
// rating trajectory by reversing this slice (chronological order).
func LoadGauntletRuns(ctx context.Context, sqlDB *sql.DB, deckKey string, limit int) ([]GauntletRunRecord, error) {
	if sqlDB == nil || deckKey == "" {
		return nil, nil
	}
	q := `SELECT run_id, deck_key, commander, started_at, finished_at,
	             games, wins, losses, win_rate,
	             elo_start, elo_end, elo_delta, avg_turns,
	             place_1st, place_2nd, place_3rd, place_4th
	      FROM gauntlet_runs WHERE deck_key = ?
	      ORDER BY finished_at DESC`
	args := []interface{}{deckKey}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GauntletRunRecord
	for rows.Next() {
		var r GauntletRunRecord
		var started, finished int64
		if err := rows.Scan(
			&r.RunID, &r.DeckKey, &r.Commander, &started, &finished,
			&r.Games, &r.Wins, &r.Losses, &r.WinRate,
			&r.ELOStart, &r.ELOEnd, &r.ELODelta, &r.AvgTurns,
			&r.Place1st, &r.Place2nd, &r.Place3rd, &r.Place4th,
		); err != nil {
			return nil, err
		}
		r.StartedAt = time.Unix(started, 0)
		r.FinishedAt = time.Unix(finished, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}
