package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// GameSearchFilters narrows the result set for SearchGames. Distinct
// from SummaryArchiveFilters: this search runs against the FULL
// game history (not just observation-snapshot rows) and adds the
// "my games" filter shape required by the "find all my games where
// I lost on turn 5" use case.
//
// Zero-value fields are treated as unset. Won is *bool so callers can
// distinguish "no filter" from "owner lost" (false) — requires Owner.
type GameSearchFilters struct {
	Owner     string // games where this owner had a seat (joined via showmatch_elo.owner)
	Q         string // free-text substring against winner_name | end_reason | any seat's commander
	Won       *bool  // requires Owner; nil = no filter, true = owner-won, false = owner-lost
	MinTurn   int    // inclusive; 0 = unbounded
	MaxTurn   int    // inclusive; 0 = unbounded
	EndReason string // case-insensitive exact match (e.g. "combat", "wipe", "decking")
	Since     int64  // unix epoch; finished_at lower bound
	Until     int64  // unix epoch; finished_at upper bound
	Limit     int    // default 50, capped at 500
	Offset    int    // pagination offset
}

// GameSearchRow is the per-game record returned by SearchGames.
// Commanders + DeckKeys are seat-ordered (index = seat). OwnerSeat is
// -1 when no Owner filter was supplied; OwnerWon is meaningless in
// that case.
type GameSearchRow struct {
	GameID     int64    `json:"game_id"`
	FinishedAt int64    `json:"finished_at"`
	Turns      int      `json:"turns"`
	Winner     int      `json:"winner"`
	WinnerName string   `json:"winner_name"`
	EndReason  string   `json:"end_reason"`
	Commanders []string `json:"commanders,omitempty"`
	DeckKeys   []string `json:"deck_keys,omitempty"`
	OwnerSeat  int      `json:"owner_seat"`
	OwnerWon   bool     `json:"owner_won"`
}

// SearchGames returns games matching the supplied filters, ordered
// newest first. Result set is bounded to f.Limit (capped at 500) with
// a follow-up per-row seat fetch for commander + deck-key context.
//
// Owner filter joins through showmatch_elo to find seats where the
// piloting owner matches. When multiple of the owner's decks were
// seated in the same game (mirror), the FIRST matching seat is
// returned (lowest seat index); OwnerWon is computed against that
// seat.
//
// Q is a single substring matched case-insensitively against
// winner_name, end_reason, OR any seat's commander name. Matches the
// game if ANY of those three fields contains q (after lowercasing).
//
// EndReason is an exact case-insensitive match. The two end-reason
// filters (Q and EndReason) compose with AND when both are set.
func SearchGames(ctx context.Context, sqlDB *sql.DB, f GameSearchFilters) ([]GameSearchRow, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var (
		conds []string
		args  []any
	)

	// Owner filter: select the lowest-seat row whose deck_key belongs
	// to the requested owner. A correlated subquery returns NULL when
	// the owner isn't seated; the surrounding IS NOT NULL gate drops
	// those rows.
	selectOwnerSeat := "(NULL) AS owner_seat"
	if f.Owner != "" {
		selectOwnerSeat = `(SELECT MIN(me.seat)
		                    FROM showmatch_game_seat me
		                    JOIN showmatch_elo e ON e.deck_key = me.deck_key AND e.owner = ?
		                    WHERE me.game_id = g.game_id) AS owner_seat`
	}

	q := `SELECT g.game_id, g.finished_at, g.turns, g.winner, g.winner_name, g.end_reason, ` + selectOwnerSeat + `
	      FROM showmatch_game g`
	if f.Owner != "" {
		// The SELECT-list subquery already references ? for the
		// owner; we need to keep the parameter ordering consistent.
		args = append(args, f.Owner)
		conds = append(conds, `EXISTS (
			SELECT 1 FROM showmatch_game_seat me
			JOIN showmatch_elo e ON e.deck_key = me.deck_key AND e.owner = ?
			WHERE me.game_id = g.game_id)`)
		args = append(args, f.Owner)
	}

	if f.Since > 0 {
		conds = append(conds, "g.finished_at >= ?")
		args = append(args, f.Since)
	}
	if f.Until > 0 {
		conds = append(conds, "g.finished_at <= ?")
		args = append(args, f.Until)
	}
	if f.MinTurn > 0 {
		conds = append(conds, "g.turns >= ?")
		args = append(args, f.MinTurn)
	}
	if f.MaxTurn > 0 {
		conds = append(conds, "g.turns <= ?")
		args = append(args, f.MaxTurn)
	}
	if f.EndReason != "" {
		conds = append(conds, "lower(g.end_reason) = lower(?)")
		args = append(args, f.EndReason)
	}
	if f.Q != "" {
		// Three-way OR: winner_name | end_reason | any seat's
		// commander. The commander branch is the costly leg (EXISTS
		// scan), but it's the one that catches free-text queries
		// like "atraxa" or "krenko" hitting opponent decks.
		conds = append(conds, `(
			lower(g.winner_name) LIKE lower(?) OR
			lower(g.end_reason) LIKE lower(?) OR
			EXISTS (SELECT 1 FROM showmatch_game_seat sq
			        WHERE sq.game_id = g.game_id AND lower(sq.commander) LIKE lower(?))
		)`)
		needle := "%" + f.Q + "%"
		args = append(args, needle, needle, needle)
	}

	// Won filter: owner_seat must equal the game's winner (true) or
	// differ from it (false). Routed as a post-WHERE filter on the
	// derived owner_seat to avoid replicating the subquery.
	if f.Owner != "" && f.Won != nil {
		if *f.Won {
			conds = append(conds, `(SELECT MIN(me.seat) FROM showmatch_game_seat me
			                        JOIN showmatch_elo e ON e.deck_key = me.deck_key AND e.owner = ?
			                        WHERE me.game_id = g.game_id) = g.winner`)
		} else {
			conds = append(conds, `(SELECT MIN(me.seat) FROM showmatch_game_seat me
			                        JOIN showmatch_elo e ON e.deck_key = me.deck_key AND e.owner = ?
			                        WHERE me.game_id = g.game_id) != g.winner`)
		}
		args = append(args, f.Owner)
	}

	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY g.finished_at DESC, g.game_id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("game search query: %w", err)
	}
	defer rows.Close()

	var out []GameSearchRow
	for rows.Next() {
		var r GameSearchRow
		var ownerSeat sql.NullInt64
		if err := rows.Scan(&r.GameID, &r.FinishedAt, &r.Turns, &r.Winner, &r.WinnerName, &r.EndReason, &ownerSeat); err != nil {
			return nil, err
		}
		if ownerSeat.Valid {
			r.OwnerSeat = int(ownerSeat.Int64)
			r.OwnerWon = r.OwnerSeat == r.Winner
		} else {
			r.OwnerSeat = -1
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Per-row seat lookup so the response includes opponent
	// commanders + deck keys. Bounded fan-out (≤500 games × ≤4 seats
	// = ≤2000 extra rows).
	for i := range out {
		sRows, qerr := sqlDB.QueryContext(ctx,
			`SELECT commander, deck_key FROM showmatch_game_seat
			 WHERE game_id = ? ORDER BY seat`, out[i].GameID)
		if qerr != nil {
			return nil, fmt.Errorf("load seats for game %d: %w", out[i].GameID, qerr)
		}
		for sRows.Next() {
			var cmd, dk string
			if serr := sRows.Scan(&cmd, &dk); serr != nil {
				sRows.Close()
				return nil, fmt.Errorf("scan seat for game %d: %w", out[i].GameID, serr)
			}
			out[i].Commanders = append(out[i].Commanders, cmd)
			out[i].DeckKeys = append(out[i].DeckKeys, dk)
		}
		if rerr := sRows.Err(); rerr != nil {
			sRows.Close()
			return nil, fmt.Errorf("seat rows.Err for game %d: %w", out[i].GameID, rerr)
		}
		sRows.Close()
	}

	return out, nil
}
