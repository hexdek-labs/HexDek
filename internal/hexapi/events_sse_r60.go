package hexapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// events_sse_r60.go — per-game Server-Sent-Events stream for
// spectator UIs. Companion to the existing /ws/events WebSocket
// (which is global + bidirectional) but scoped to a single game_id
// for the spectator's-eye-view path:
//
//   GET /api/events/{game_id}/stream
//
// Why SSE alongside WebSocket: SSE is one-way (server → client) over
// plain HTTP/1.1, so it slips through corporate proxies that block
// WS upgrades, requires zero client-side library (browsers have
// EventSource), and reconnects automatically. The WebSocket path
// stays the canonical bidirectional surface; SSE is the lower-
// friction read-only stream for embedded widgets / dashboards /
// share-page live tickers.
//
// Filter strategy: the EventBus is global. The handler subscribes
// with no type-filter (so it sees every published event) and then
// filters per-frame by extracting "game_id" from event.Data. This
// keeps the bus simple and lets a future "subscribe to multiple
// games" surface layer on a different per-request mux without
// changing the bus contract.
//
// Significant-event taxonomy (event types this stream surfaces —
// engine emit sites are wired in follow-up PRs):
//   - game.turn_transition   one per advance_phase + turn cycle
//   - game.combo_fired       fired by per_card combo detectors
//   - game.player_loss       SBA-driven elimination
//   - game.end               existing terminal event
//
// New constants added below; the bus accepts any string so the
// engine can publish these incrementally.

// Spectator-stream event-type constants. New events go here so the
// publisher side (gameengine + per_card handlers) can reference
// them by name without typos. Wire format leaves them as plain
// strings — no enum gating — to keep room for future event types
// the SSE handler doesn't yet know about.
const (
	EventTurnTransition = "game.turn_transition"
	EventComboFired     = "game.combo_fired"
	EventPlayerLoss     = "game.player_loss"
)

// SSEKeepaliveInterval is how often the per-game stream emits a
// `: ping` comment to keep proxies (Caddy / nginx default 60s idle
// close) from severing the connection. 15s matches the existing
// gauntlet SSE keepalive in showmatch.go.
const SSEKeepaliveInterval = 15 * time.Second

// EventStreamHandler serves the per-game SSE endpoint. Carries a
// pointer to the EventBus + an optional per-IP RateLimiter. Both
// nil-safe — a binary that doesn't wire the bus 503s the endpoint;
// a binary without a limiter doesn't throttle (matches every other
// hexapi rate-limit field's nil-safe semantics).
type EventStreamHandler struct {
	Bus     *EventBus
	Limiter *RateLimiter
}

// RegisterEventStream mounts GET /api/events/{game_id}/stream.
// Idempotent — safe to call from any registration block.
func (h *EventStreamHandler) RegisterEventStream(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/events/{game_id}/stream", h.serve)
}

// serve handles one SSE session: validate game_id, throttle, set
// SSE headers, subscribe to the bus, filter per-frame by game_id,
// fan to the client until the client disconnects or the bus closes
// our channel.
func (h *EventStreamHandler) serve(w http.ResponseWriter, r *http.Request) {
	// Per-IP rate limit FIRST — defends the upgrade against burst
	// reconnects (every reconnect would otherwise consume a
	// subscriber slot + a goroutine).
	if enforceRateLimit(h.Limiter, w, r, "event stream") {
		return
	}

	if h.Bus == nil {
		writeError(w, http.StatusServiceUnavailable, "event bus not configured")
		return
	}

	gameIDStr := r.PathValue("game_id")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
	if err != nil || gameID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid game_id")
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
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	// Emit an opening comment + a `ready` event so the client knows
	// the stream is live even before the first significant engine
	// event arrives. The EventSource API doesn't surface successful
	// connection separately from the first message, so this empty
	// frame is the "you're connected" signal.
	writeSSE(w, flusher, "ready", map[string]any{
		"game_id":    gameID,
		"server_now": time.Now().UTC().Format(time.RFC3339),
	})

	sub := h.Bus.Subscribe(64)
	defer h.Bus.Unsubscribe(sub)

	ctx := r.Context()
	keepalive := time.NewTicker(SSEKeepaliveInterval)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub.Ch:
			if !ok {
				return
			}
			if !eventMatchesGame(ev, gameID) {
				continue
			}
			writeSSE(w, flusher, ev.Type, sseFramePayload(ev))
		case <-keepalive.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// eventMatchesGame returns true when ev.Data has a game_id field
// that matches the requested game. Handles three shapes:
//   - map[string]any with a "game_id" key (numeric or string)
//   - struct with a "GameID" / "game_id" field (via marshal→unmarshal
//     fallback — costs one round-trip per event but only fires when
//     the type assertion misses)
//   - nil Data → never matches (defensive — a typed event without
//     game_id shouldn't pollute a per-game stream)
//
// Events that match no shape are dropped from the per-game stream,
// which is the right default: a future event type that doesn't
// carry game_id (e.g. cluster-wide elo.recalc) should NOT surface
// to spectators of one specific game.
func eventMatchesGame(ev Event, gameID int64) bool {
	if ev.Data == nil {
		return false
	}
	if m, ok := ev.Data.(map[string]any); ok {
		return mapHasMatchingGameID(m, gameID)
	}
	// Struct fallback: marshal then unmarshal to map. Costs one round
	// per event whose Data isn't already a map, which in practice
	// applies to typed publisher events. Acceptable for per-game-
	// scoped streams (call rate is low; the bus already imposes a
	// 64-entry buffer per subscriber, so the worst-case throughput
	// is bounded).
	b, err := json.Marshal(ev.Data)
	if err != nil {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return false
	}
	return mapHasMatchingGameID(m, gameID)
}

// mapHasMatchingGameID reads game_id from a normalized event-data
// map. JSON unmarshals integers as float64 by default, so accept
// both numeric and string forms.
func mapHasMatchingGameID(m map[string]any, gameID int64) bool {
	v, ok := m["game_id"]
	if !ok {
		return false
	}
	switch n := v.(type) {
	case float64:
		return int64(n) == gameID
	case int:
		return int64(n) == gameID
	case int64:
		return n == gameID
	case string:
		parsed, err := strconv.ParseInt(n, 10, 64)
		return err == nil && parsed == gameID
	default:
		return false
	}
}

// sseFramePayload normalizes ev for SSE emission. The wire shape is
// the same as the WebSocket bus output (event + data + timestamp)
// so a client can switch transports without re-parsing — JS code
// that handles a /ws/events message handles a /api/events/.../stream
// frame identically once it's been unboxed from the SSE envelope.
func sseFramePayload(ev Event) map[string]any {
	ts := ev.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return map[string]any{
		"event":     ev.Type,
		"data":      ev.Data,
		"timestamp": ts.Format(time.RFC3339Nano),
	}
}
