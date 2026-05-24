package hexapi

import (
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/hat"
)

// AdminConvictionHandler exposes a live debug view of the conviction
// diagnostic ring buffer maintained in internal/hat. Round 16 made
// conviction non-acting and started emitting "conviction_diagnostic"
// events on the game log for post-game correlation. This endpoint is
// the live counterpart: operators can hit it mid-tournament to see
// what the conviction triggers *would* have decided across the most
// recent samples.
//
//	GET /api/admin/conviction-events?since=<seq>&limit=<n>&triggered=1
//
// Auth uses the same shape as admin_anomalies: an HEXDEK_ADMIN_OWNER
// env var matched against the X-HexDek-Owner header, falling back to
// localhost-only when no admin owner is configured.
type AdminConvictionHandler struct{}

// Register wires the handler into mux.
func (h *AdminConvictionHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/conviction-events", h.handleList)
}

func (h *AdminConvictionHandler) handleList(w http.ResponseWriter, r *http.Request) {
	// Reuse the localhost-or-admin-owner check from admin_anomalies so
	// every conviction-related admin surface gates the same way.
	if !adminAnomalyAuth(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	q := r.URL.Query()
	var sinceSeq uint64
	if v := q.Get("since"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			sinceSeq = n
		}
	}
	// Default cap of 200 keeps responses small for live polling; clients
	// that want the full buffer can pass limit=0.
	limit := 200
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	triggeredOnly := q.Get("triggered") == "1" || q.Get("triggered") == "true"

	events, totalSeen := hat.SnapshotConvictionEvents(sinceSeq, limit)
	if triggeredOnly {
		filtered := events[:0:0]
		for _, ev := range events {
			if ev.AnyTriggered {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	// latest_seq lets the client trivially resume polling with
	// ?since=<latest_seq>. total_seen is cumulative across the process
	// (not capped at buffer size), useful for spotting eviction.
	var latestSeq uint64
	if n := len(events); n > 0 {
		latestSeq = events[n-1].Sequence
	}

	writeJSON(w, map[string]any{
		"count":          len(events),
		"latest_seq":     latestSeq,
		"total_seen":     totalSeen,
		"buffer_capacity": 1024,
		"events":         events,
	})
}
