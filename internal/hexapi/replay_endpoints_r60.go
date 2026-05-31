package hexapi

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// replay_endpoints_r60.go — three structured replay endpoints
// complementing the existing GET /api/games/{id}/replay (which streams
// NDJSON). These all return a single JSON document for clients that
// prefer object-level parsing over line-by-line consumption (React
// frontend, sharing tools, third-party integrations).
//
// All three use the same snapshot-cache + load path as
// /api/games/{id}/replay → /api/games/{id}/summary → /api/games/{id}/summary.pdf,
// so a cached snapshot from any one path is reused on the others.
//
// Auth model: read-only public, matching the existing replay/summary
// endpoints. Game replays are a public record by the time they're
// queryable (the game has ended; the snapshot is persisted). A future
// PR may add an opt-in "private games" gate.
//
// Routes (registered in handler.go):
//   GET /api/replay/{game_id}            → full replay JSON
//   GET /api/replay/{game_id}/turn/{n}   → state-at-turn-N projection
//   GET /api/replay/{game_id}/forensics  → multi-signal analysis bundle
//
// All error paths use writeError → unified ErrorResponse envelope
// (PR #778). The error contract mirrors the NDJSON endpoint:
//   400 — non-numeric / non-positive id or turn
//   404 — game id unknown OR no snapshot persisted
//   500 — DB error or malformed snapshot
//   503 — DB unavailable

// FullReplayResponse is the GET /api/replay/{game_id} body shape.
// `events` is the same chronological list the NDJSON endpoint
// streams, packaged as a single document with a `meta` envelope so
// consumers can read total_turns / winner without scanning all events.
type FullReplayResponse struct {
	GameID     int64                  `json:"game_id"`
	TotalTurns int                    `json:"total_turns"`
	Winner     int                    `json:"winner"`
	Events     []heimdall.ReplayEvent `json:"events"`
}

// TurnStateResponse is the GET /api/replay/{game_id}/turn/{n} body.
// The snapshot is end-of-game, NOT per-turn, so this endpoint
// projects what was true "by turn N" — events through turn N, names
// known by turn N, MVP/regret cards that emerged by then. Reflects
// the snapshot's information-theoretic limit, not a per-turn replay
// (which would need every-turn snapshots, a separate plumbing pass).
type TurnStateResponse struct {
	GameID         int64                  `json:"game_id"`
	Turn           int                    `json:"turn"`
	TotalTurns     int                    `json:"total_turns"`
	EventsThroughN []heimdall.ReplayEvent `json:"events_through_n"`
	MVPsEmerged    []heimdall.MVPCard     `json:"mvps_emerged"`
}

// ForensicsResponse is the GET /api/replay/{game_id}/forensics body.
// Bundles every analysis signal heimdall surfaces off the snapshot —
// regret cards, mulligan stats, commander zone visits, MVPs, plus
// turning points. Single round-trip to assemble what would otherwise
// be 5 separate hits.
type ForensicsResponse struct {
	GameID              int64                          `json:"game_id"`
	Winner              int                            `json:"winner"`
	TotalTurns          int                            `json:"total_turns"`
	TurningPoints       []heimdall.TurningPoint        `json:"turning_points"`
	MVPCards            []heimdall.MVPCard             `json:"mvp_cards"`
	RegretCards         []heimdall.RegretCard          `json:"regret_cards"`
	MulliganStats       []heimdall.MulliganStat        `json:"mulligan_stats"`
	CommanderZoneVisits []heimdall.CommanderZoneVisit  `json:"commander_zone_visits"`
}

// handleReplayFull serves GET /api/replay/{game_id}. Returns the
// same chronological event list the NDJSON endpoint streams, in a
// single JSON envelope so JS clients can `await fetch().then(r =>
// r.json())` instead of line-buffering.
func (h *Handler) handleReplayFull(w http.ResponseWriter, r *http.Request) {
	id, snap, ok := h.loadReplaySnapshot(w, r, "game_id")
	if !ok {
		return
	}
	events := heimdall.BuildReplayLog(snap)
	totalTurns := snap.Seed.Turns
	if snap.FinalTurn > totalTurns {
		totalTurns = snap.FinalTurn
	}
	writeJSON(w, FullReplayResponse{
		GameID:     id,
		TotalTurns: totalTurns,
		Winner:     snap.Seed.Winner,
		Events:     events,
	})
}

// handleReplayTurn serves GET /api/replay/{game_id}/turn/{n}.
// Filters BuildReplayLog by turn ≤ n + the MVP subset that emerged
// by turn n. game_end at total_turns is excluded when n < total_turns
// (the game hadn't ended yet from turn n's perspective).
func (h *Handler) handleReplayTurn(w http.ResponseWriter, r *http.Request) {
	id, snap, ok := h.loadReplaySnapshot(w, r, "game_id")
	if !ok {
		return
	}
	turnStr := r.PathValue("n")
	turn, err := strconv.Atoi(turnStr)
	if err != nil || turn < 1 {
		writeError(w, http.StatusBadRequest, "invalid turn")
		return
	}

	all := heimdall.BuildReplayLog(snap)
	totalTurns := snap.Seed.Turns
	if snap.FinalTurn > totalTurns {
		totalTurns = snap.FinalTurn
	}

	through := make([]heimdall.ReplayEvent, 0, len(all))
	for _, ev := range all {
		if ev.Turn > turn {
			continue
		}
		// game_end at the final turn is part of the snapshot but
		// suppress it when the caller asked for an earlier turn —
		// from turn N's perspective, the game hasn't ended yet.
		if ev.Kind == "game_end" && turn < totalTurns {
			continue
		}
		through = append(through, ev)
	}

	mvps := make([]heimdall.MVPCard, 0, len(snap.MVPCards))
	for _, mvp := range snap.MVPCards {
		if mvp.TurnPlayed > 0 && mvp.TurnPlayed <= turn {
			mvps = append(mvps, mvp)
		}
	}

	writeJSON(w, TurnStateResponse{
		GameID:         id,
		Turn:           turn,
		TotalTurns:     totalTurns,
		EventsThroughN: through,
		MVPsEmerged:    mvps,
	})
}

// handleReplayForensics serves GET /api/replay/{game_id}/forensics.
// One round-trip assembly of every analysis surface heimdall already
// computes from a snapshot — what would otherwise require 5 separate
// hits to summary + replay + regret/MVP/zone-visit extractors.
func (h *Handler) handleReplayForensics(w http.ResponseWriter, r *http.Request) {
	id, snap, ok := h.loadReplaySnapshot(w, r, "game_id")
	if !ok {
		return
	}
	// Re-use the snapshot-summary builder for turning_points — it
	// owns the commander_first_cast + game_end derivation logic, no
	// reason to fork it here.
	summary := heimdall.BuildGameSummaryFromSnapshot(snap, "")
	totalTurns := snap.Seed.Turns
	if snap.FinalTurn > totalTurns {
		totalTurns = snap.FinalTurn
	}
	writeJSON(w, ForensicsResponse{
		GameID:              id,
		Winner:              snap.Seed.Winner,
		TotalTurns:          totalTurns,
		TurningPoints:       summary.TurningPoints,
		MVPCards:            nonNilMVPs(snap.MVPCards),
		RegretCards:         nonNilRegrets(snap.RegretCards),
		MulliganStats:       nonNilMulligans(snap.MulliganStats),
		CommanderZoneVisits: nonNilZoneVisits(snap.CommanderZoneVisits),
	})
}

// loadReplaySnapshot is the shared load + error-shape helper for the
// three replay endpoints. Returns (id, snap, true) on success;
// writes an error response and returns (_, _, false) on failure.
// All four error paths (DB unavailable, bad id, game not found,
// missing/malformed snapshot, DB error) emit the unified envelope.
func (h *Handler) loadReplaySnapshot(w http.ResponseWriter, r *http.Request, idPathKey string) (int64, heimdall.GameObservationSnapshot, bool) {
	var zero heimdall.GameObservationSnapshot
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return 0, zero, false
	}
	idStr := r.PathValue(idPathKey)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid game id")
		return 0, zero, false
	}
	ctx := r.Context()
	if _, err := db.LoadGameByID(ctx, h.db, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "game not found")
			return 0, zero, false
		}
		writeError(w, http.StatusInternalServerError, "load game: "+err.Error())
		return 0, zero, false
	}
	h.ensureSnapshotCache()
	snap, _, err := h.loadCachedSnapshot(ctx, id)
	switch {
	case err == nil:
		return id, snap, true
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, "no observation snapshot for game")
		return 0, zero, false
	case IsMalformedSnapshot(err):
		log.Printf("replay_endpoints: malformed snapshot for game %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "malformed snapshot")
		return 0, zero, false
	default:
		writeError(w, http.StatusInternalServerError, "load observation: "+err.Error())
		return 0, zero, false
	}
}

// nonNil* helpers normalize nil slices to empty `[]` so the JSON
// response never carries `"mvp_cards": null` — a JS array consumer
// would have to nil-check otherwise. Same shape every time.
func nonNilMVPs(s []heimdall.MVPCard) []heimdall.MVPCard {
	if s == nil {
		return []heimdall.MVPCard{}
	}
	return s
}

func nonNilRegrets(s []heimdall.RegretCard) []heimdall.RegretCard {
	if s == nil {
		return []heimdall.RegretCard{}
	}
	return s
}

func nonNilMulligans(s []heimdall.MulliganStat) []heimdall.MulliganStat {
	if s == nil {
		return []heimdall.MulliganStat{}
	}
	return s
}

func nonNilZoneVisits(s []heimdall.CommanderZoneVisit) []heimdall.CommanderZoneVisit {
	if s == nil {
		return []heimdall.CommanderZoneVisit{}
	}
	return s
}
