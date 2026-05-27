package hexapi

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

// TournamentMatchEvent is a per-game delta fired by Showmatch's
// gauntlet loop into subscribers of /api/tournament/{id}/stream.
// Unlike GauntletResult (the whole-state snapshot streamed by the
// existing /api/tournaments/{owner}/{id}/events SSE), this struct
// carries ONLY the just-completed game plus running totals — the
// payload a live-results UI needs to render "match #14 finished:
// Krenko beat Atraxa in 12 turns; deck now 9W-5L (64.3%)" without
// re-reading the full snapshot every tick.
type TournamentMatchEvent struct {
	RunID           int64    `json:"run_id"`
	GameIndex       int      `json:"game_index"`  // 1-based — game number within the run
	TotalGames      int      `json:"total_games"` // expected total for the run
	WinnerSeat      int      `json:"winner_seat"` // -1 = draw, 0 = deck under test, 1..3 = opponents
	WinnerCommander string   `json:"winner_commander"`
	Opponents       []string `json:"opponents"` // seat 1..3 commanders
	Turns           int      `json:"turns"`
	RunningWins     int      `json:"running_wins"`
	RunningLosses   int      `json:"running_losses"`
	RunningWinRate  float64  `json:"running_winrate"` // 0-100, rounded to 1 decimal
	FinishedAt      int64    `json:"finished_at"`     // unix seconds
}

// matchSubscriber is one SSE listener for per-match events on a
// specific tournament run. Buffered ch=32 lets a slow client lag a
// bit before we drop events on the floor (broadcasts are non-blocking
// — see broadcastMatch). 32 is chosen so a typical 100-game gauntlet
// won't drop unless the client is genuinely stalled.
type matchSubscriber struct {
	ch chan TournamentMatchEvent
}

// MatchBroadcaster is the per-run subscriber registry that backs
// /api/tournament/{id}/stream. Decoupled from Showmatch so the stream
// can be tested without spinning up the whole gauntlet engine — the
// Showmatch hook just calls Broadcast on the run's id, and tests can
// drive Broadcast directly.
type MatchBroadcaster struct {
	mu     sync.Mutex
	subs   map[int64]map[*matchSubscriber]struct{}
	closed map[int64]bool // runs that have been finalized — new subs return immediately
}

// NewMatchBroadcaster returns an empty broadcaster ready to accept
// subscribers and broadcasts. Safe for concurrent use.
func NewMatchBroadcaster() *MatchBroadcaster {
	return &MatchBroadcaster{
		subs:   map[int64]map[*matchSubscriber]struct{}{},
		closed: map[int64]bool{},
	}
}

// Subscribe registers a listener for runID. Returns the subscriber and
// a bool indicating whether the run is already closed. When closed,
// the returned subscriber's channel is ALREADY CLOSED — callers can
// safely `for range sub.ch` and the loop will exit immediately,
// matching the active-then-Close lifecycle path so callers don't need
// to branch on the bool to avoid hanging.
func (mb *MatchBroadcaster) Subscribe(runID int64) (*matchSubscriber, bool) {
	sub := &matchSubscriber{ch: make(chan TournamentMatchEvent, 32)}
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if mb.closed[runID] {
		close(sub.ch)
		return sub, true
	}
	if _, ok := mb.subs[runID]; !ok {
		mb.subs[runID] = map[*matchSubscriber]struct{}{}
	}
	mb.subs[runID][sub] = struct{}{}
	return sub, false
}

// Unsubscribe removes a listener. Safe to call on a sub already
// removed by Close — the map-delete is idempotent and a nil-map check
// guards against the post-close state.
func (mb *MatchBroadcaster) Unsubscribe(runID int64, sub *matchSubscriber) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if subs, ok := mb.subs[runID]; ok {
		delete(subs, sub)
		if len(subs) == 0 {
			delete(mb.subs, runID)
		}
	}
}

// Broadcast fans an event out to every active subscriber of runID.
// Non-blocking — a subscriber whose 32-deep buffer is full drops the
// event. The gauntlet engine cannot stall on a slow client, so we
// accept lossy delivery as the cost of liveness.
func (mb *MatchBroadcaster) Broadcast(runID int64, evt TournamentMatchEvent) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	for s := range mb.subs[runID] {
		select {
		case s.ch <- evt:
		default:
			// Buffer full — drop. See struct doc for the design rationale.
		}
	}
}

// Close marks runID as terminal and closes every subscriber's channel
// so SSE handlers can return cleanly via the range-from-closed-channel
// pattern. Idempotent — repeated Close on the same run is a no-op
// (the second call finds no subs to close and the closed-flag is
// already set).
func (mb *MatchBroadcaster) Close(runID int64) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.closed[runID] = true
	for s := range mb.subs[runID] {
		close(s.ch)
	}
	delete(mb.subs, runID)
}

// SubscriberCount reports the number of active subscribers for runID.
// Mostly for tests + future observability — production code shouldn't
// branch on this (TOCTOU between read + use).
func (mb *MatchBroadcaster) SubscriberCount(runID int64) int {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	return len(mb.subs[runID])
}

// BuildMatchEvent assembles a TournamentMatchEvent from the per-game
// state available inside Showmatch's gauntlet loop. Centralized here
// so the loop integration is a single function call and the wire
// shape stays in one file.
//
// winnerCommander is "" on draws (winnerSeat == -1) per the
// MatchEvent contract.
func BuildMatchEvent(runID int64, gameIndex, totalGames, winnerSeat int,
	commanders []string, turns, wins, losses int, finishedAt time.Time) TournamentMatchEvent {

	winnerName := ""
	if winnerSeat >= 0 && winnerSeat < len(commanders) {
		winnerName = commanders[winnerSeat]
	}
	opps := make([]string, 0, 3)
	for i := 1; i < len(commanders); i++ {
		opps = append(opps, commanders[i])
	}
	played := wins + losses
	wr := 0.0
	if played > 0 {
		wr = math.Round(float64(wins)/float64(played)*1000) / 10
	}
	return TournamentMatchEvent{
		RunID:           runID,
		GameIndex:       gameIndex,
		TotalGames:      totalGames,
		WinnerSeat:      winnerSeat,
		WinnerCommander: winnerName,
		Opponents:       opps,
		Turns:           turns,
		RunningWins:     wins,
		RunningLosses:   losses,
		RunningWinRate:  wr,
		FinishedAt:      finishedAt.Unix(),
	}
}

// handleTournamentStream implements GET /api/tournament/{id}/stream.
//
// SSE stream keyed by gauntlet_runs.run_id (the same id accepted by
// /api/tournament/{id}/stats). Emits one `status` event immediately
// describing the run, then a `match` event per completed game until
// the run reaches terminal status (complete | error) or the client
// disconnects. Heartbeats every 15s keep the connection alive through
// idle proxies (Caddy on MISTY, browser EventSource defaults).
//
// Run states:
//   - active (in-memory MatchBroadcaster has the run): subscribe +
//     stream live match events. Final `complete` event closes the
//     stream when the gauntlet finishes.
//   - completed (DB row exists, run isn't streaming): emit a single
//     `complete` status event with the run summary and close. Clients
//     should fall back to /api/tournament/{id}/stats for full details.
//   - unknown (no DB row, no in-memory entry): 404.
func (h *Handler) handleTournamentStream(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	runID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || runID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid tournament id")
		return
	}
	if h.Showmatch == nil {
		writeError(w, http.StatusServiceUnavailable, "tournament stream unavailable")
		return
	}
	mb := h.Showmatch.MatchBroadcaster
	if mb == nil {
		writeError(w, http.StatusServiceUnavailable, "tournament stream unavailable")
		return
	}

	// Resolve the run. If it exists in the DB we know it's a real
	// tournament; otherwise 404 so clients can't probe arbitrary ids
	// to fingerprint future allocations.
	var (
		runMeta    db.GauntletRunRecord
		runMetaOK  bool
	)
	if h.Showmatch.sqlDB != nil {
		rec, runErr := db.LoadGauntletRunByID(r.Context(), h.Showmatch.sqlDB, runID)
		switch {
		case runErr == nil:
			runMeta, runMetaOK = rec, true
		case errors.Is(runErr, sql.ErrNoRows):
			// fall through to in-memory check
		default:
			writeError(w, http.StatusInternalServerError, "tournament lookup failed")
			return
		}
	}
	inMemory := h.Showmatch.findGauntletByRunID(runID)
	if !runMetaOK && inMemory == nil {
		writeError(w, http.StatusNotFound, "tournament not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Subscribe BEFORE writing the initial status event so we don't
	// race against the gauntlet loop finishing a game in the window
	// between status emission and Subscribe.
	sub, alreadyClosed := mb.Subscribe(runID)
	defer mb.Unsubscribe(runID, sub)

	// Initial status event tells the client which mode they're in.
	statusName := "active"
	if alreadyClosed || (inMemory == nil && runMetaOK) {
		statusName = "complete"
	}
	var runMetaPtr *db.GauntletRunRecord
	if runMetaOK {
		runMetaPtr = &runMeta
	}
	writeSSE(w, flusher, "status", map[string]any{
		"run_id":       runID,
		"status":       statusName,
		"games_so_far": gauntletGamesSoFar(inMemory, runMetaPtr),
	})
	if statusName == "complete" {
		return
	}

	ctx := r.Context()
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-sub.ch:
			if !ok {
				// Broadcaster closed the channel — gauntlet finished.
				writeSSE(w, flusher, "complete", map[string]any{
					"run_id": runID,
				})
				return
			}
			writeSSE(w, flusher, "match", evt)
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// gauntletGamesSoFar returns the best-effort count of games completed
// at the moment of subscribe. Prefers the in-memory snapshot (live
// truth) over the DB row (stale after each game-write batch).
func gauntletGamesSoFar(inMem *GauntletResult, dbRow *db.GauntletRunRecord) int {
	if inMem != nil {
		return inMem.Games
	}
	if dbRow != nil {
		return dbRow.Games
	}
	return 0
}

// findGauntletByRunID looks up the in-memory GauntletResult whose
// RunID matches. O(n) over active gauntlets — n is small (one per
// active deck), and this only fires at SSE-connect time, not per
// match. Returns nil when no active gauntlet has that RunID
// (already-completed runs live only in the DB).
func (sm *Showmatch) findGauntletByRunID(runID int64) *GauntletResult {
	sm.gauntletMu.RLock()
	defer sm.gauntletMu.RUnlock()
	for _, g := range sm.gauntlets {
		if g != nil && g.RunID == runID {
			return g
		}
	}
	return nil
}
