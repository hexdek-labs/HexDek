package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ----------------------------------------------------------------------
// verification_queue
// ----------------------------------------------------------------------

// VerificationQueueRow is a single pending / completed spot-check.
type VerificationQueueRow struct {
	QueueID         int64
	GameID          int64
	DeckKey         string
	EnqueuedAt      int64
	Status          string // pending|running|passed|failed|error|inconclusive
	StartedAt       sql.NullInt64
	FinishedAt      sql.NullInt64
	Detail          string
	RNGSeed         int64
	NSeats          int
	DeckKeys        []string // canonical seat-indexed keys, decoded from deck_keys_json
	ClaimedWinner   int
	ClaimedTurns    int
	ReplayedWinner  int
	ReplayedTurns   int
}

// VerificationEnqueueParams is the per-row input to EnqueueVerification.
type VerificationEnqueueParams struct {
	GameID        int64
	DeckKey       string
	RNGSeed       int64
	NSeats        int
	DeckKeys      []string
	ClaimedWinner int
	ClaimedTurns  int
}

// EnqueueVerification inserts a single pending row. The scheduler
// calls this for each randomly-selected game in a batch.
func EnqueueVerification(ctx context.Context, db *sql.DB, p VerificationEnqueueParams) (int64, error) {
	keysJSON, err := json.Marshal(p.DeckKeys)
	if err != nil {
		return 0, fmt.Errorf("marshal deck keys: %w", err)
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO verification_queue (
			game_id, deck_key, enqueued_at, status,
			rng_seed, n_seats, deck_keys_json,
			claimed_winner, claimed_turns
		) VALUES (?, ?, ?, 'pending', ?, ?, ?, ?, ?)`,
		p.GameID, p.DeckKey, time.Now().Unix(),
		p.RNGSeed, p.NSeats, string(keysJSON),
		p.ClaimedWinner, p.ClaimedTurns,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ClaimNextVerification atomically claims the oldest pending row for
// processing. Marks it status='running' and returns the row, or nil
// when the queue is empty. The caller MUST eventually call
// FinishVerification to release the row, even on error.
func ClaimNextVerification(ctx context.Context, db *sql.DB) (*VerificationQueueRow, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx,
		`SELECT queue_id, game_id, deck_key, enqueued_at, status,
			started_at, finished_at, detail,
			rng_seed, n_seats, deck_keys_json,
			claimed_winner, claimed_turns,
			replayed_winner, replayed_turns
		 FROM verification_queue
		 WHERE status = 'pending'
		 ORDER BY enqueued_at ASC, queue_id ASC
		 LIMIT 1`)
	r, err := scanVerificationQueueRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx,
		`UPDATE verification_queue SET status='running', started_at=? WHERE queue_id=?`,
		now, r.QueueID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	r.Status = "running"
	r.StartedAt = sql.NullInt64{Int64: now, Valid: true}
	return r, nil
}

// FinishVerification writes the terminal status (passed|failed|error|
// inconclusive) + replayed outcome + detail back to the row.
//
// "inconclusive" is the NON-SANCTIONING terminal verdict (r63 C-H4): the
// verifier could not reproduce the live game's hat, so a replay
// divergence is ambiguous between an honest game replayed with the wrong
// hat and a spoofed game. Acting on it would risk banning an honest
// contributor, so such rows finish "inconclusive" and the worker neither
// cauterizes nor marks the game verified.
func FinishVerification(ctx context.Context, db *sql.DB, queueID int64, status string, replayedWinner, replayedTurns int, detail string) error {
	if status != "passed" && status != "failed" && status != "error" && status != "inconclusive" {
		return fmt.Errorf("invalid terminal status %q", status)
	}
	_, err := db.ExecContext(ctx,
		`UPDATE verification_queue
		 SET status=?, finished_at=?, replayed_winner=?, replayed_turns=?, detail=?
		 WHERE queue_id=?`,
		status, time.Now().Unix(), replayedWinner, replayedTurns, detail, queueID)
	return err
}

// ListVerifications returns recent rows. status="" means any. limit
// is capped at 500.
func ListVerifications(ctx context.Context, db *sql.DB, status string, limit int) ([]VerificationQueueRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT queue_id, game_id, deck_key, enqueued_at, status,
		started_at, finished_at, detail,
		rng_seed, n_seats, deck_keys_json,
		claimed_winner, claimed_turns,
		replayed_winner, replayed_turns
		FROM verification_queue`
	args := []any{}
	if status != "" {
		q += ` WHERE status=?`
		args = append(args, status)
	}
	q += ` ORDER BY enqueued_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VerificationQueueRow
	for rows.Next() {
		r, err := scanVerificationQueueRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// VerificationStats counts rows by status. Convenient for /admin
// dashboards.
type VerificationStats struct {
	Pending      int
	Running      int
	Passed       int
	Failed       int
	Error        int
	Inconclusive int
}

func GetVerificationStats(ctx context.Context, db *sql.DB) (VerificationStats, error) {
	var s VerificationStats
	rows, err := db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM verification_queue GROUP BY status`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return s, err
		}
		switch status {
		case "pending":
			s.Pending = n
		case "running":
			s.Running = n
		case "passed":
			s.Passed = n
		case "failed":
			s.Failed = n
		case "error":
			s.Error = n
		case "inconclusive":
			s.Inconclusive = n
		}
	}
	return s, rows.Err()
}

// scanVerificationQueueRow accepts either a *sql.Row or *sql.Rows.
func scanVerificationQueueRow(s interface {
	Scan(...any) error
}) (*VerificationQueueRow, error) {
	var r VerificationQueueRow
	var keysJSON string
	if err := s.Scan(
		&r.QueueID, &r.GameID, &r.DeckKey, &r.EnqueuedAt, &r.Status,
		&r.StartedAt, &r.FinishedAt, &r.Detail,
		&r.RNGSeed, &r.NSeats, &keysJSON,
		&r.ClaimedWinner, &r.ClaimedTurns,
		&r.ReplayedWinner, &r.ReplayedTurns,
	); err != nil {
		return nil, err
	}
	if keysJSON != "" {
		_ = json.Unmarshal([]byte(keysJSON), &r.DeckKeys)
	}
	return &r, nil
}

// ----------------------------------------------------------------------
// contributor_sanctions
// ----------------------------------------------------------------------

// Sanction severities.
const (
	SeverityWarning      = "warning"
	SeverityTempBan      = "temp_ban"
	SeverityPermanentBan = "permanent_ban"
)

// SanctionRow is a single warning / ban issued against a deck_key.
type SanctionRow struct {
	SanctionID int64
	DeckKey    string
	Owner      string
	OffenseNum int
	Severity   string
	IssuedAt   int64
	ExpiresAt  sql.NullInt64
	Reason     string
	QueueID    sql.NullInt64
	Reviewed   bool
}

// SanctionInsertParams is what the cauterize service hands to the DB
// layer when applying a new sanction.
type SanctionInsertParams struct {
	DeckKey   string
	Owner     string
	Severity  string
	Reason    string
	QueueID   int64 // 0 means "no triggering verification"
	IssuedAt  int64 // 0 → time.Now()
	ExpiresAt int64 // 0 → no expiry
}

// CountOffenses returns the number of prior sanctions issued against
// deck_key. Used by cauterize to determine the next escalation tier.
func CountOffenses(ctx context.Context, db *sql.DB, deckKey string) (int, error) {
	row := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM contributor_sanctions WHERE deck_key=?`, deckKey)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// InsertSanction writes the row and returns its sanction_id. Caller
// is responsible for picking the right severity for the offense
// number — see anticheat.CauterizeService.
func InsertSanction(ctx context.Context, db *sql.DB, p SanctionInsertParams) (int64, error) {
	if p.Severity != SeverityWarning && p.Severity != SeverityTempBan && p.Severity != SeverityPermanentBan {
		return 0, fmt.Errorf("invalid severity %q", p.Severity)
	}
	issuedAt := p.IssuedAt
	if issuedAt == 0 {
		issuedAt = time.Now().Unix()
	}
	offenseNum, err := CountOffenses(ctx, db, p.DeckKey)
	if err != nil {
		return 0, err
	}
	offenseNum++ // this row IS the next offense
	var expires sql.NullInt64
	if p.ExpiresAt > 0 {
		expires = sql.NullInt64{Int64: p.ExpiresAt, Valid: true}
	}
	var queueRef sql.NullInt64
	if p.QueueID > 0 {
		queueRef = sql.NullInt64{Int64: p.QueueID, Valid: true}
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO contributor_sanctions (
			deck_key, owner, offense_num, severity, issued_at, expires_at, reason, queue_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.DeckKey, p.Owner, offenseNum, p.Severity, issuedAt, expires, p.Reason, queueRef,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListSanctionsForDeck returns every sanction row for a deck_key,
// newest first.
func ListSanctionsForDeck(ctx context.Context, db *sql.DB, deckKey string) ([]SanctionRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT sanction_id, deck_key, owner, offense_num, severity,
			issued_at, expires_at, reason, queue_id, reviewed
		 FROM contributor_sanctions
		 WHERE deck_key=?
		 ORDER BY issued_at DESC, sanction_id DESC`, deckKey)
	if err != nil {
		return nil, err
	}
	return scanSanctionRows(rows)
}

// ListSanctions returns recent rows across all deck_keys. activeOnly
// filters to currently-effective bans (severity != warning AND
// (expires_at IS NULL OR expires_at > now)).
func ListSanctions(ctx context.Context, db *sql.DB, activeOnly bool, limit int) ([]SanctionRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT sanction_id, deck_key, owner, offense_num, severity,
		issued_at, expires_at, reason, queue_id, reviewed
		FROM contributor_sanctions`
	args := []any{}
	if activeOnly {
		q += ` WHERE severity != 'warning' AND (expires_at IS NULL OR expires_at > ?)`
		args = append(args, time.Now().Unix())
	}
	q += ` ORDER BY issued_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return scanSanctionRows(rows)
}

// ActiveBan returns the currently-effective ban (temp or permanent)
// for deck_key, or nil. Warnings never count as bans.
func ActiveBan(ctx context.Context, db *sql.DB, deckKey string) (*SanctionRow, error) {
	now := time.Now().Unix()
	row := db.QueryRowContext(ctx,
		`SELECT sanction_id, deck_key, owner, offense_num, severity,
			issued_at, expires_at, reason, queue_id, reviewed
		 FROM contributor_sanctions
		 WHERE deck_key=? AND severity IN ('temp_ban','permanent_ban')
			AND (expires_at IS NULL OR expires_at > ?)
		 ORDER BY issued_at DESC LIMIT 1`,
		deckKey, now)
	r := SanctionRow{}
	if err := row.Scan(&r.SanctionID, &r.DeckKey, &r.Owner, &r.OffenseNum, &r.Severity,
		&r.IssuedAt, &r.ExpiresAt, &r.Reason, &r.QueueID, &r.Reviewed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// QuarantineRecentGames marks every showmatch_game row in which this
// deck_key appears (via showmatch_game_seat) and that finished after
// `since` as verified=-1. Used by cauterize on a failed verification.
// Returns the number of game rows touched.
func QuarantineRecentGames(ctx context.Context, db *sql.DB, deckKey string, since int64) (int64, error) {
	res, err := db.ExecContext(ctx,
		`UPDATE showmatch_game
		 SET verified=-1
		 WHERE finished_at >= ?
		   AND game_id IN (
			   SELECT game_id FROM showmatch_game_seat WHERE deck_key=?
		   )`,
		since, deckKey)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// MarkGameVerified sets verified=1 on a single game row. Called by
// the verification worker after a passing replay.
func MarkGameVerified(ctx context.Context, db *sql.DB, gameID int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE showmatch_game SET verified=1 WHERE game_id=? AND verified=0`,
		gameID)
	return err
}

// MarkGameUnverified sets verified=-1 on a single game row. Called by
// the verification worker after a failed replay.
func MarkGameUnverified(ctx context.Context, db *sql.DB, gameID int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE showmatch_game SET verified=-1 WHERE game_id=?`, gameID)
	return err
}

// LoadGameForVerification returns the inputs needed to enqueue a
// spot-check. Reads showmatch_game + showmatch_game_seat in one shot.
type GameForVerification struct {
	GameID   int64
	RNGSeed  int64
	Winner   int
	Turns    int
	NSeats   int
	DeckKeys []string
}

func LoadGameForVerification(ctx context.Context, db *sql.DB, gameID int64) (*GameForVerification, error) {
	g := &GameForVerification{GameID: gameID}
	row := db.QueryRowContext(ctx,
		`SELECT rng_seed, winner, turns FROM showmatch_game WHERE game_id=?`, gameID)
	if err := row.Scan(&g.RNGSeed, &g.Winner, &g.Turns); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT seat, deck_key FROM showmatch_game_seat WHERE game_id=? ORDER BY seat ASC`,
		gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type seatKey struct {
		seat int
		key  string
	}
	var seats []seatKey
	for rows.Next() {
		var sk seatKey
		if err := rows.Scan(&sk.seat, &sk.key); err != nil {
			return nil, err
		}
		seats = append(seats, sk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	g.NSeats = len(seats)
	g.DeckKeys = make([]string, len(seats))
	for _, sk := range seats {
		if sk.seat < 0 || sk.seat >= len(g.DeckKeys) {
			return nil, fmt.Errorf("seat index %d out of range for n_seats=%d", sk.seat, g.NSeats)
		}
		g.DeckKeys[sk.seat] = strings.TrimSpace(sk.key)
	}
	return g, nil
}

func scanSanctionRows(rows *sql.Rows) ([]SanctionRow, error) {
	defer rows.Close()
	var out []SanctionRow
	for rows.Next() {
		var r SanctionRow
		var reviewed int
		if err := rows.Scan(&r.SanctionID, &r.DeckKey, &r.Owner, &r.OffenseNum, &r.Severity,
			&r.IssuedAt, &r.ExpiresAt, &r.Reason, &r.QueueID, &reviewed); err != nil {
			return nil, err
		}
		r.Reviewed = reviewed != 0
		out = append(out, r)
	}
	return out, rows.Err()
}
