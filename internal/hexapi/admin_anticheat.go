package hexapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/db"
)

// AdminAnticheatHandler exposes read-only admin endpoints for the
// Phase 2 spot-check + cauterize machinery:
//
//	GET /api/admin/verifications?status=&limit= — recent queue rows
//	GET /api/admin/sanctions?active=&deck_key=&limit= — recent
//	    contributor sanctions
//
// Both endpoints are gated by an Authorization: Bearer <token>
// header matched against AdminToken. Empty AdminToken refuses ALL
// requests (fail-closed) to avoid accidentally leaking sanctions
// data in dev / misconfigured deployments.
type AdminAnticheatHandler struct {
	DB         *sql.DB
	AdminToken string
}

// Register wires the handler into mux. Idempotent at the mux level —
// re-registering on a fresh mux is the documented Go-stdlib pattern.
func (h *AdminAnticheatHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/verifications", h.handleListVerifications)
	mux.HandleFunc("GET /api/admin/sanctions", h.handleListSanctions)
	mux.HandleFunc("GET /api/admin/anticheat/stats", h.handleStats)
}

func (h *AdminAnticheatHandler) authorized(r *http.Request) bool {
	if h.AdminToken == "" {
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return got == h.AdminToken
}

func (h *AdminAnticheatHandler) handleListVerifications(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "db not configured")
		return
	}
	status := r.URL.Query().Get("status")
	limit := parseLimit(r.URL.Query().Get("limit"), 100)

	rows, err := db.ListVerifications(r.Context(), h.DB, status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list: "+err.Error())
		return
	}
	stats, _ := db.GetVerificationStats(r.Context(), h.DB)

	out := struct {
		Stats db.VerificationStats        `json:"stats"`
		Rows  []verificationViewRow       `json:"rows"`
	}{
		Stats: stats,
		Rows:  toVerificationViews(rows),
	}
	writeAdminJSON(w, out)
}

func (h *AdminAnticheatHandler) handleListSanctions(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "db not configured")
		return
	}
	q := r.URL.Query()
	deckKey := q.Get("deck_key")
	activeOnly := q.Get("active") == "1" || q.Get("active") == "true"
	limit := parseLimit(q.Get("limit"), 100)

	var rows []db.SanctionRow
	var err error
	if deckKey != "" {
		rows, err = db.ListSanctionsForDeck(r.Context(), h.DB, deckKey)
	} else {
		rows, err = db.ListSanctions(r.Context(), h.DB, activeOnly, limit)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list: "+err.Error())
		return
	}

	out := struct {
		Count int               `json:"count"`
		Rows  []sanctionView    `json:"rows"`
	}{
		Count: len(rows),
		Rows:  toSanctionViews(rows),
	}
	writeAdminJSON(w, out)
}

func (h *AdminAnticheatHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "db not configured")
		return
	}
	queueStats, _ := db.GetVerificationStats(r.Context(), h.DB)
	activeBans, _ := db.ListSanctions(r.Context(), h.DB, true, 500)
	out := struct {
		Queue       db.VerificationStats `json:"queue"`
		ActiveBans  int                  `json:"active_bans"`
	}{
		Queue:      queueStats,
		ActiveBans: len(activeBans),
	}
	writeAdminJSON(w, out)
}

func parseLimit(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 500 {
		return def
	}
	return n
}

func writeAdminJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// verificationViewRow is the JSON shape for /api/admin/verifications.
// We hand-craft it (rather than marshalling db.VerificationQueueRow
// directly) so sql.NullInt64 fields render as nullable JSON
// integers instead of {"Int64":..., "Valid":...} objects.
type verificationViewRow struct {
	QueueID         int64    `json:"queue_id"`
	GameID          int64    `json:"game_id"`
	DeckKey         string   `json:"deck_key"`
	EnqueuedAt      int64    `json:"enqueued_at"`
	Status          string   `json:"status"`
	StartedAt       *int64   `json:"started_at,omitempty"`
	FinishedAt      *int64   `json:"finished_at,omitempty"`
	Detail          string   `json:"detail,omitempty"`
	RNGSeed         int64    `json:"rng_seed"`
	NSeats          int      `json:"n_seats"`
	DeckKeys        []string `json:"deck_keys"`
	ClaimedWinner   int      `json:"claimed_winner"`
	ClaimedTurns    int      `json:"claimed_turns"`
	ReplayedWinner  int      `json:"replayed_winner"`
	ReplayedTurns   int      `json:"replayed_turns"`
}

func toVerificationViews(rows []db.VerificationQueueRow) []verificationViewRow {
	out := make([]verificationViewRow, 0, len(rows))
	for _, r := range rows {
		v := verificationViewRow{
			QueueID:        r.QueueID,
			GameID:         r.GameID,
			DeckKey:        r.DeckKey,
			EnqueuedAt:     r.EnqueuedAt,
			Status:         r.Status,
			Detail:         r.Detail,
			RNGSeed:        r.RNGSeed,
			NSeats:         r.NSeats,
			DeckKeys:       r.DeckKeys,
			ClaimedWinner:  r.ClaimedWinner,
			ClaimedTurns:   r.ClaimedTurns,
			ReplayedWinner: r.ReplayedWinner,
			ReplayedTurns:  r.ReplayedTurns,
		}
		if r.StartedAt.Valid {
			v.StartedAt = &r.StartedAt.Int64
		}
		if r.FinishedAt.Valid {
			v.FinishedAt = &r.FinishedAt.Int64
		}
		out = append(out, v)
	}
	return out
}

type sanctionView struct {
	SanctionID int64  `json:"sanction_id"`
	DeckKey    string `json:"deck_key"`
	Owner      string `json:"owner,omitempty"`
	OffenseNum int    `json:"offense_num"`
	Severity   string `json:"severity"`
	IssuedAt   int64  `json:"issued_at"`
	ExpiresAt  *int64 `json:"expires_at,omitempty"`
	Reason     string `json:"reason,omitempty"`
	QueueID    *int64 `json:"queue_id,omitempty"`
	Reviewed   bool   `json:"reviewed"`
}

func toSanctionViews(rows []db.SanctionRow) []sanctionView {
	out := make([]sanctionView, 0, len(rows))
	for _, r := range rows {
		v := sanctionView{
			SanctionID: r.SanctionID,
			DeckKey:    r.DeckKey,
			Owner:      r.Owner,
			OffenseNum: r.OffenseNum,
			Severity:   r.Severity,
			IssuedAt:   r.IssuedAt,
			Reason:     r.Reason,
			Reviewed:   r.Reviewed,
		}
		if r.ExpiresAt.Valid {
			v.ExpiresAt = &r.ExpiresAt.Int64
		}
		if r.QueueID.Valid {
			v.QueueID = &r.QueueID.Int64
		}
		out = append(out, v)
	}
	return out
}
