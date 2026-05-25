package hexapi

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"
)

// ArchetypeCorrectionCluster groups every `(freya_archetype →
// user_archetype)` correction pattern in the archetype_feedback_log
// (PR #421) by the number of DISTINCT owners who hit it. Surfaced
// to Freya operators as the training signal: clusters with high
// OwnerCount mean "the archetype fingerprint is consistently
// getting overridden by users — review the fingerprint in
// cmd/hexdek-freya/archetypes.go."
//
// FirstSeen / LastSeen are unix-epoch seconds; OwnerCount is
// distinct-owner count (NOT total corrections, since one
// loud owner clicking the same correction 50 times shouldn't
// dominate the signal). ExampleDecks is up to 3 deck_key
// strings — the operator can spot-check the source decks
// to understand WHY users disagree with Freya.
type ArchetypeCorrectionCluster struct {
	FreyaArchetype   string   `json:"freya_archetype"`
	UserArchetype    string   `json:"user_archetype"`
	OwnerCount       int      `json:"owner_count"`
	TotalCorrections int      `json:"total_corrections"`
	FirstSeen        int64    `json:"first_seen"`
	LastSeen         int64    `json:"last_seen"`
	FirstSeenRFC3339 string   `json:"first_seen_rfc3339,omitempty"`
	LastSeenRFC3339  string   `json:"last_seen_rfc3339,omitempty"`
	ExampleDecks     []string `json:"example_decks"`
}

// DefaultTrainingSignalThreshold is the minimum owner-count below
// which a correction cluster doesn't surface. Five is the threshold
// the user specified: enough independent disagreement to suggest a
// fingerprint issue, low enough that real signals don't take months
// to accumulate.
const DefaultTrainingSignalThreshold = 5

// MaxTrainingSignalThreshold caps the `min` query param to keep
// abusive callers from forcing a `HAVING COUNT(DISTINCT owner) >=
// 1000000`-style query. The log table is small but the GROUP BY +
// DISTINCT subquery is the expensive part.
const MaxTrainingSignalThreshold = 1000

// loadArchetypeCorrectionClusters reads the archetype_feedback_log
// and returns every (freya, user) correction pair where at least
// minOwners distinct owners have hit the pattern. Pairs sorted by
// OwnerCount desc, then LastSeen desc (newest first within same
// owner count), then FreyaArchetype asc for paranoid determinism.
//
// Only "correct" actions are aggregated — "confirm" tells us the
// fingerprint is RIGHT, which is the opposite signal; "clear"
// retracts and shouldn't contribute either direction.
//
// Up to 3 distinct deck_keys per cluster surface as examples so
// the operator can spot-check the source decks.
func loadArchetypeCorrectionClusters(ctx context.Context, db *sql.DB, minOwners int) ([]ArchetypeCorrectionCluster, error) {
	if db == nil {
		return nil, nil
	}
	if minOwners < 1 {
		minOwners = DefaultTrainingSignalThreshold
	}
	rows, err := db.QueryContext(ctx, `
		SELECT
		    freya_archetype,
		    user_archetype,
		    COUNT(DISTINCT owner)   AS owner_count,
		    COUNT(*)                AS total_corrections,
		    MIN(recorded_at)        AS first_seen,
		    MAX(recorded_at)        AS last_seen
		FROM archetype_feedback_log
		WHERE user_action = 'correct'
		  AND freya_archetype != ''
		  AND user_archetype  != ''
		GROUP BY freya_archetype, user_archetype
		HAVING COUNT(DISTINCT owner) >= ?
		ORDER BY owner_count DESC, last_seen DESC, freya_archetype ASC
	`, minOwners)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ArchetypeCorrectionCluster{}
	for rows.Next() {
		var c ArchetypeCorrectionCluster
		if err := rows.Scan(&c.FreyaArchetype, &c.UserArchetype, &c.OwnerCount,
			&c.TotalCorrections, &c.FirstSeen, &c.LastSeen); err != nil {
			return nil, err
		}
		if c.FirstSeen > 0 {
			c.FirstSeenRFC3339 = time.Unix(c.FirstSeen, 0).UTC().Format(time.RFC3339)
		}
		if c.LastSeen > 0 {
			c.LastSeenRFC3339 = time.Unix(c.LastSeen, 0).UTC().Format(time.RFC3339)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Per-cluster example-decks lookup — 3 distinct deck_keys per
	// (freya, user) pair, newest first. Done as a follow-up query
	// rather than a giant CTE so the SQL stays readable; the
	// cluster slice is small (one row per discovered pattern, not
	// per log entry) so the N+1 cost is bounded.
	for i := range out {
		exRows, err := db.QueryContext(ctx, `
			SELECT deck_key FROM (
			    SELECT owner || '/' || deck_id AS deck_key,
			           MAX(recorded_at)        AS latest
			    FROM archetype_feedback_log
			    WHERE user_action = 'correct'
			      AND freya_archetype = ?
			      AND user_archetype  = ?
			    GROUP BY owner, deck_id
			)
			ORDER BY latest DESC
			LIMIT 3
		`, out[i].FreyaArchetype, out[i].UserArchetype)
		if err != nil {
			continue
		}
		for exRows.Next() {
			var dk string
			if err := exRows.Scan(&dk); err == nil && dk != "" {
				out[i].ExampleDecks = append(out[i].ExampleDecks, dk)
			}
		}
		exRows.Close()
	}
	return out, nil
}

// handleArchetypeTrainingSignal serves GET /api/freya/training-signal.
// Public read — clusters are aggregate stats, no per-owner detail
// beyond ExampleDecks which is opt-in (the deck owner already
// chose to publish their archetype correction by submitting it).
//
// Query params:
//   - min=N — threshold for owner_count (default 5, capped at 1000)
//
// Response:
//
//	{"clusters": [...], "threshold": N, "generated_at": "..."}
//
// Empty clusters serialize as [] not null so consumers iterate
// without nil-guarding.
func (h *Handler) handleArchetypeTrainingSignal(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	threshold := DefaultTrainingSignalThreshold
	if v := r.URL.Query().Get("min"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > MaxTrainingSignalThreshold {
				n = MaxTrainingSignalThreshold
			}
			threshold = n
		}
	}

	clusters, err := loadArchetypeCorrectionClusters(r.Context(), h.db, threshold)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load: "+err.Error())
		return
	}
	if clusters == nil {
		clusters = []ArchetypeCorrectionCluster{}
	}

	writeJSON(w, map[string]any{
		"clusters":     clusters,
		"threshold":    threshold,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}
