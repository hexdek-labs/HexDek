package hexapi

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/versioning"
)

// ArchetypeHistoryEvent is one entry on a deck's archetype timeline.
// Kinds (small fixed enum):
//
//   - "version_archetype": a version was created and Freya later
//     classified it as this Archetype. Carries Version + Hash. The
//     example "v2.5 was Combo" surfaces as this kind.
//   - "user_confirmed":    user confirmed Freya's call (no Archetype
//     change, just an agreement signal).
//   - "user_corrected":    user replaced Freya's call with their own
//     Archetype value. The previous-tier "FREYA" archetype on this
//     row carries the original; UserArchetype carries the override.
//   - "user_cleared":      user retracted prior feedback.
//
// Time is unix-epoch seconds for sortability; the wire format
// includes both Time and TimeRFC3339 so clients can choose.
type ArchetypeHistoryEvent struct {
	Kind            string `json:"kind"`
	Time            int64  `json:"time"`
	TimeRFC3339     string `json:"time_rfc3339,omitempty"`
	Version         int    `json:"version,omitempty"`
	Hash            string `json:"hash,omitempty"`
	Archetype       string `json:"archetype,omitempty"`
	UserArchetype   string `json:"user_archetype,omitempty"`
	ChangedFrom     string `json:"changed_from,omitempty"`
}

// handleArchetypeHistory returns a chronologically-sorted timeline
// of a deck's archetype state — both Freya's per-version
// classifications AND the user's confirm/correct/clear actions
// since PR #421. Routed at GET /api/decks/{owner}/{id}/archetype-history.
//
// Public read (no auth required), since the archetype timeline is
// part of the deck's public face and the underlying data
// (versioning DAG + feedback log) doesn't carry sensitive context.
//
// Response shape:
//
//	{"events": [ArchetypeHistoryEvent, ...]}
//
// Events are sorted by Time ascending. The first version's
// version_archetype event acts as the timeline's anchor; subsequent
// version_archetype events with different Archetype values
// represent reclassifications and carry ChangedFrom for the
// "v2.5 was 'Combo', v2.6 reclassified to 'Storm'" narrative.
func (h *Handler) handleArchetypeHistory(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	if findDeckFile(h.DecksDir, owner, id) == "" {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	events := buildArchetypeHistory(h.DecksDir, owner, id, h.db, r.Context())
	writeJSON(w, map[string]any{"events": events})
}

// buildArchetypeHistory is the shared assembler used by the
// endpoint and by tests. Splitting it out keeps the HTTP wiring
// trivially testable without httptest setup.
func buildArchetypeHistory(decksDir, owner, id string, db *sql.DB, ctx ctxLike) []ArchetypeHistoryEvent {
	// Non-nil-empty so a deck with no version/log events still
	// serializes as `"events": []` (not `null`) — frontends iterate
	// without a nil guard.
	events := []ArchetypeHistoryEvent{}

	// 1. Version-archetype events from the DAG. Skip versions where
	//    Freya hasn't classified yet (Archetype == ""); they appear
	//    on the timeline once a future runFreya stamps them.
	dagDir := filepath.Join(decksDir, ".versions")
	if dag, err := versioning.LoadDAG(dagDir); err == nil {
		// GetLineage returns NEWEST-FIRST per its docstring; walk
		// it oldest→newest so ChangedFrom captures the prior
		// version's archetype rather than the next one's.
		lineage := dag.GetLineage(owner, id)
		prev := ""
		for i := len(lineage) - 1; i >= 0; i-- {
			node := lineage[i]
			if node.Archetype == "" {
				continue
			}
			ev := ArchetypeHistoryEvent{
				Kind:      "version_archetype",
				Time:      parseRFC3339OrZero(node.CreatedAt),
				Version:   node.Version,
				Hash:      node.Hash,
				Archetype: node.Archetype,
			}
			if ev.Time > 0 {
				ev.TimeRFC3339 = time.Unix(ev.Time, 0).UTC().Format(time.RFC3339)
			}
			if prev != "" && !strings.EqualFold(prev, node.Archetype) {
				ev.ChangedFrom = prev
			}
			events = append(events, ev)
			prev = node.Archetype
		}
	}

	// 2. User-feedback events from the archetype_feedback_log.
	if db != nil {
		// rowid secondary keeps insertion order stable when two
		// log rows share the same unix-epoch-second recorded_at
		// (e.g. a user clicks Confirm then Disagree in the same
		// wall-clock second). SQLite's implicit ROWID monotonically
		// increases on INSERT, so this gives us the exact click
		// order without an explicit PK on the table.
		rows, err := db.QueryContext(ctx,
			`SELECT freya_archetype, user_action, user_archetype, recorded_at
			 FROM archetype_feedback_log
			 WHERE owner = ? AND deck_id = ?
			 ORDER BY recorded_at ASC, rowid ASC`, owner, id)
		if err == nil {
			for rows.Next() {
				var freyaAt, action, userAt string
				var when int64
				if err := rows.Scan(&freyaAt, &action, &userAt, &when); err != nil {
					continue
				}
				kind := "user_" + action
				ev := ArchetypeHistoryEvent{
					Kind:          kind,
					Time:          when,
					Archetype:     freyaAt,
					UserArchetype: userAt,
				}
				if when > 0 {
					ev.TimeRFC3339 = time.Unix(when, 0).UTC().Format(time.RFC3339)
				}
				events = append(events, ev)
			}
			rows.Close()
		}
	}

	// 3. Sort chronologically with a deterministic tie-break: ascending
	//    Time, then version_archetype before user_* (so a version
	//    reclassification reads before a same-second user action on
	//    that new version), then Version asc for paranoid ordering.
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Time != events[j].Time {
			return events[i].Time < events[j].Time
		}
		iVer := events[i].Kind == "version_archetype"
		jVer := events[j].Kind == "version_archetype"
		if iVer != jVer {
			return iVer
		}
		return events[i].Version < events[j].Version
	})
	return events
}

// parseRFC3339OrZero converts a stored CreatedAt RFC3339 string to
// unix-epoch seconds. Returns 0 on malformed input so the sort
// pushes broken rows to the start of the timeline rather than
// crashing.
func parseRFC3339OrZero(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// ctxLike is the minimal context interface buildArchetypeHistory
// uses, so tests can hand it context.Background() or any timed
// variant without dragging the full context import shape into
// every caller.
type ctxLike interface {
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}
