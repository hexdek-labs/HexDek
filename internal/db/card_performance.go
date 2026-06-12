package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// card_performance — per-card "did the card actually do anything this
// game?" aggregate. Distinct from card_stats (which counts every game
// the card was in any deck list — drawn or not) and from card_win_stats
// (which is keyed (card_name, commander) and only flags battlefield
// presence at game end).
//
// A row is updated for every game in which a seat had the card in any
// non-library/non-command zone at game end (battlefield, hand,
// graveyard) — i.e. the card was at least drawn. games_included counts
// those occurrences; wins_when_included tags the winner's contribution.
//
// avg_turn_played is the running mean of the turn the card first
// resolved as a spell (driven by GameState.CardFirstPlayed). 0 means
// "no turn data captured for this card yet" — observers should
// distinguish 0 from "not measured" with avg_turn_played > 0.
//
// avg_battlefield_time is the running mean of how many turns the card
// spent on the battlefield. For now it's an end-of-game approximation:
// game_turn − card_first_played for cards still on the battlefield at
// game end, which underestimates churned permanents but doesn't require
// per-permanent ETB-turn instrumentation.
const cardPerformanceSchema = `
CREATE TABLE IF NOT EXISTS card_performance (
    card_name             TEXT PRIMARY KEY,
    games_included        INTEGER NOT NULL DEFAULT 0,
    wins_when_included    INTEGER NOT NULL DEFAULT 0,
    avg_turn_played       REAL    NOT NULL DEFAULT 0,
    avg_battlefield_time  REAL    NOT NULL DEFAULT 0,
    -- Internal denominators so running means stay correct across upserts.
    -- Not exposed in the API; consumers read avg_* directly.
    turn_play_count       INTEGER NOT NULL DEFAULT 0,
    bf_obs_count          INTEGER NOT NULL DEFAULT 0,
    updated_at            INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_card_performance_winrate
    ON card_performance(wins_when_included, games_included);
`

// CardPerformance is the API view of a single row.
type CardPerformance struct {
	CardName           string
	GamesIncluded      int
	WinsWhenIncluded   int
	AvgTurnPlayed      float64
	AvgBattlefieldTime float64
}

// CardPerformanceDelta is the per-game increment.
//
//	Win                 1 if the seat that included this card won, 0 otherwise.
//	TurnPlayed          turn the card first resolved as a spell (>0). 0 = card
//	                    was in a zone but never cast (drawn but not played).
//	                    Negative = caller doesn't know; the average is left
//	                    untouched.
//	BattlefieldTurns    turns the card spent on the battlefield. Same
//	                    semantics as TurnPlayed for "no observation".
type CardPerformanceDelta struct {
	CardName         string
	Win              int
	TurnPlayed       int
	BattlefieldTurns int
}

// BatchUpsertCardPerformance applies deltas in a single transaction.
// The running mean update uses ((old_avg * old_n) + sample) / (old_n + 1)
// guarded by the stored count column, which is the only correct way to
// fold per-game samples into a long-running average without losing
// precision across thousands of upserts.
func BatchUpsertCardPerformance(ctx context.Context, sqlDB *sql.DB, deltas []CardPerformanceDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	if sqlDB == nil {
		return errors.New("db: nil database")
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	read, err := tx.PrepareContext(ctx, `
		SELECT games_included, wins_when_included,
		       avg_turn_played, avg_battlefield_time,
		       turn_play_count, bf_obs_count
		FROM card_performance WHERE card_name = ?`)
	if err != nil {
		return err
	}
	defer read.Close()

	write, err := tx.PrepareContext(ctx, `
		INSERT INTO card_performance (card_name, games_included, wins_when_included,
		                              avg_turn_played, avg_battlefield_time,
		                              turn_play_count, bf_obs_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(card_name) DO UPDATE SET
		  games_included        = excluded.games_included,
		  wins_when_included    = excluded.wins_when_included,
		  avg_turn_played       = excluded.avg_turn_played,
		  avg_battlefield_time  = excluded.avg_battlefield_time,
		  turn_play_count       = excluded.turn_play_count,
		  bf_obs_count          = excluded.bf_obs_count,
		  updated_at            = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer write.Close()

	now := time.Now().Unix()
	for _, d := range deltas {
		if d.CardName == "" {
			continue
		}
		var (
			games, wins, turnN, bfN int
			avgTurn, avgBf          float64
		)
		err := read.QueryRowContext(ctx, d.CardName).Scan(
			&games, &wins, &avgTurn, &avgBf, &turnN, &bfN)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		games++
		if d.Win == 1 {
			wins++
		}
		if d.TurnPlayed > 0 {
			avgTurn = (avgTurn*float64(turnN) + float64(d.TurnPlayed)) / float64(turnN+1)
			turnN++
		}
		if d.BattlefieldTurns > 0 {
			avgBf = (avgBf*float64(bfN) + float64(d.BattlefieldTurns)) / float64(bfN+1)
			bfN++
		}
		if _, err := write.ExecContext(ctx, d.CardName, games, wins, avgTurn, avgBf, turnN, bfN, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadCardPerformance reads a single row. Returns a zero-valued
// CardPerformance with GamesIncluded == 0 when the card has no rows
// rather than sql.ErrNoRows so handlers can render a "no data" view
// without branching.
func LoadCardPerformance(ctx context.Context, sqlDB *sql.DB, cardName string) (CardPerformance, error) {
	var p CardPerformance
	p.CardName = cardName
	if sqlDB == nil {
		return p, errors.New("db: nil database")
	}
	err := sqlDB.QueryRowContext(ctx, `
		SELECT card_name, games_included, wins_when_included,
		       avg_turn_played, avg_battlefield_time
		FROM card_performance WHERE card_name = ?`, cardName,
	).Scan(&p.CardName, &p.GamesIncluded, &p.WinsWhenIncluded,
		&p.AvgTurnPlayed, &p.AvgBattlefieldTime)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	return p, nil
}
