package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SummaryArchiveRow is the per-game record returned by
// SearchGameSummaries — enough to render an archive list view
// (game id, when, who won, what they played, end reason) without
// needing the full GameSummary blob. Commanders + DeckKeys are
// joined from showmatch_game_seat.
type SummaryArchiveRow struct {
	GameID     int64    `json:"game_id"`
	FinishedAt int64    `json:"finished_at"`
	Turns      int      `json:"turns"`
	Winner     int      `json:"winner"`
	WinnerName string   `json:"winner_name"`
	EndReason  string   `json:"end_reason"`
	Commanders []string `json:"commanders,omitempty"`
	DeckKeys   []string `json:"deck_keys,omitempty"`
}

// SummaryArchiveFilters narrows the result set for the archive
// query. Zero-value fields are treated as unset.
//
//   - Since / Until: unix-epoch-seconds bounds on finished_at.
//     Inclusive at both ends. Zero means unbounded on that side.
//   - Commander: substring (case-sensitive on SQLite by default)
//     matched against showmatch_game_seat.commander. Matches the
//     game when ANY seat carries a matching commander.
//   - Deck: substring matched against showmatch_game_seat.deck_key.
//     Same any-seat semantics as Commander.
//   - Winner: substring matched against showmatch_game.winner_name.
//
// Limit defaults to 50 when ≤ 0; capped at 500 to bound the
// response payload. Offset enables simple paging.
type SummaryArchiveFilters struct {
	Since     int64
	Until     int64
	Commander string
	Deck      string
	Winner    string
	Limit     int
	Offset    int
}

// SearchGameSummaries returns games that have a persisted
// observation snapshot in showmatch_game_observation, narrowed by
// the supplied filters. INNER JOIN against the observation table
// is intentional — the archive view is for the "rich path" only,
// not the db_only fallback (those games are reachable via
// LoadRecentGames if needed).
//
// Result ordering: finished_at DESC, game_id DESC tie-break —
// newest first, deterministic when two games share a timestamp.
//
// Commanders + DeckKeys are filled in via a per-row follow-up
// query rather than a single composite join to keep the SQL
// readable; the result set is bounded to Limit (≤500) so the
// fan-out is small.
func SearchGameSummaries(ctx context.Context, sqlDB *sql.DB, f SummaryArchiveFilters) ([]SummaryArchiveRow, error) {
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
	if f.Since > 0 {
		conds = append(conds, "g.finished_at >= ?")
		args = append(args, f.Since)
	}
	if f.Until > 0 {
		conds = append(conds, "g.finished_at <= ?")
		args = append(args, f.Until)
	}
	if f.Winner != "" {
		conds = append(conds, "g.winner_name LIKE ?")
		args = append(args, "%"+f.Winner+"%")
	}
	if f.Commander != "" {
		conds = append(conds, `EXISTS (
			SELECT 1 FROM showmatch_game_seat s
			WHERE s.game_id = g.game_id AND s.commander LIKE ?
		)`)
		args = append(args, "%"+f.Commander+"%")
	}
	if f.Deck != "" {
		conds = append(conds, `EXISTS (
			SELECT 1 FROM showmatch_game_seat s
			WHERE s.game_id = g.game_id AND s.deck_key LIKE ?
		)`)
		args = append(args, "%"+f.Deck+"%")
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	q := fmt.Sprintf(`
		SELECT g.game_id, g.finished_at, g.turns, g.winner, g.winner_name, g.end_reason
		FROM showmatch_game g
		JOIN showmatch_game_observation o ON o.game_id = g.game_id
		%s
		ORDER BY g.finished_at DESC, g.game_id DESC
		LIMIT ? OFFSET ?
	`, where)
	args = append(args, limit, offset)

	rows, err := sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SummaryArchiveRow
	for rows.Next() {
		var r SummaryArchiveRow
		if err := rows.Scan(&r.GameID, &r.FinishedAt, &r.Turns, &r.Winner, &r.WinnerName, &r.EndReason); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Per-row seat lookup — commanders + deck keys ordered by seat.
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
