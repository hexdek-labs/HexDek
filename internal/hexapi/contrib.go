// BOINC-style distributed-compute dispatcher.
//
// Server flow:
//
//   1. Operator authenticates via Authorization: Bearer <token>.
//      ValidateSession → owner slug.
//   2. WS upgrades. Server emits a "welcome" with a per-session HMAC
//      key (random 32B, hex-encoded). Client uses this key to sign
//      every result it returns; server uses it to sign assignments.
//   3. Client emits "ready" with worker info. Server pulls a
//      ContribAssignment off its work queue, signs it, and sends it
//      down. If the queue is empty, server sends "wait".
//   4. Client runs the chunk locally and emits "result". Server
//      verifies the result signature, optionally re-runs a fraction
//      of chunks for spot-check parity (default 3%), updates
//      Welford stats, awards credits if validated, and replies "ack".
//   5. Repeat.
//
// Validation is intentionally cheap: spot-check a small percentage
// (configurable via HEXDEK_CONTRIB_SPOTCHECK_PCT, default 3) and trust
// the rest. Cheating is dis-incentivized via the 3-sigma anomaly
// detector and a strike-based freeze policy: if a contributor's
// elapsed-ms distribution deviates >3σ from their own running mean,
// or if a spot-check fails, the freeze flag is set and credits stop
// accruing until manual review.
package hexapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	mathrand "math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/hexdek/hexdek/internal/auth"
	"github.com/hexdek/hexdek/internal/db"
)

// ContribDispatcher runs the server-side BOINC dispatcher. One instance
// per process; the work queue is in-memory, so a server restart drops
// pending chunks. Future enhancement: persist chunk_id → assignment in
// SQLite so a reconnecting client can resume mid-run.
type ContribDispatcher struct {
	DB              *sql.DB
	SpotCheckPct    float64 // 0–1, fraction of chunks to re-run for parity
	FreezeAtZScore  float64 // anomaly detector threshold; default 3.0
	WorkerVersion   string  // expected client version (informational)

	// SpotCheck is an optional callback the server uses to re-run a
	// chunk for parity checking. Returning (winners, turns, true) on
	// match; (_, _, false) on mismatch. The dispatcher is engine-
	// agnostic: hexapi imports tournament transitively via showmatch
	// already, but to keep this file dependency-light we accept the
	// runner as a function value injected at server construction.
	SpotCheck func(a *ContribAssignment) (winners []int, turns []int, ok bool)

	queueMu sync.Mutex
	queue   []*ContribAssignment

	rngMu sync.Mutex
	rng   *mathrand.Rand
}

// NewContribDispatcher constructs a dispatcher with sensible defaults.
func NewContribDispatcher(database *sql.DB) *ContribDispatcher {
	return &ContribDispatcher{
		DB:             database,
		SpotCheckPct:   0.03,
		FreezeAtZScore: 3.0,
		WorkerVersion:  "hexdek-contrib/1",
		rng:            mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
	}
}

// Enqueue appends a fresh assignment to the work queue. Caller fills in
// ChunkID, Decks, NSeats, GamesCount, Seed, MaxTurns, Difficulty.
// IssuedAt is stamped here. The caller MUST NOT pre-sign — the
// dispatcher signs at hand-out time using the per-session key.
func (d *ContribDispatcher) Enqueue(a *ContribAssignment) {
	a.IssuedAt = time.Now().Unix()
	d.queueMu.Lock()
	d.queue = append(d.queue, a)
	d.queueMu.Unlock()
}

// PendingCount returns the size of the work queue.
func (d *ContribDispatcher) PendingCount() int {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()
	return len(d.queue)
}

// dequeue returns the next assignment, or nil if empty.
func (d *ContribDispatcher) dequeue() *ContribAssignment {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()
	if len(d.queue) == 0 {
		return nil
	}
	a := d.queue[0]
	d.queue = d.queue[1:]
	return a
}

// requeue puts an assignment back at the FRONT of the queue (used
// when a client disconnects mid-flight).
func (d *ContribDispatcher) requeue(a *ContribAssignment) {
	if a == nil {
		return
	}
	d.queueMu.Lock()
	d.queue = append([]*ContribAssignment{a}, d.queue...)
	d.queueMu.Unlock()
}

// shouldSpotCheck flips a coin biased by SpotCheckPct.
func (d *ContribDispatcher) shouldSpotCheck() bool {
	if d.SpotCheckPct <= 0 || d.SpotCheck == nil {
		return false
	}
	d.rngMu.Lock()
	v := d.rng.Float64()
	d.rngMu.Unlock()
	return v < d.SpotCheckPct
}

// Register installs HTTP routes on mux.
//   GET /api/contrib/connect     — WebSocket
//   GET /api/contrib/credits/{owner} — JSON credits row
//   GET /api/contrib/queue       — JSON queue stats (admin/debug)
func (d *ContribDispatcher) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/contrib/connect", d.handleConnect)
	mux.HandleFunc("GET /api/contrib/credits/{owner}", d.handleCredits)
	mux.HandleFunc("GET /api/contrib/queue", d.handleQueue)
}

func (d *ContribDispatcher) handleQueue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"pending":         d.PendingCount(),
		"spot_check_pct":  d.SpotCheckPct,
		"worker_version":  d.WorkerVersion,
	})
}

func (d *ContribDispatcher) handleCredits(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	if owner == "" {
		writeError(w, http.StatusBadRequest, "missing owner")
		return
	}
	c, err := db.GetContributorCredits(r.Context(), d.DB, owner)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"owner":             c.Owner,
		"credits_total":     c.CreditsTotal,
		"chunks_completed":  c.ChunksCompleted,
		"chunks_rejected":   c.ChunksRejected,
		"games_simulated":   c.GamesSimulated,
		"frozen":            c.Frozen,
		"frozen_reason":     c.FrozenReason,
		"first_seen_at":     c.FirstSeenAt,
		"last_active_at":    c.LastActiveAt,
		"last_z_score":      c.LastZScore,
	})
}

// --- WebSocket handler --------------------------------------------------

func (d *ContribDispatcher) handleConnect(w http.ResponseWriter, r *http.Request) {
	if d.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "contrib disabled")
		return
	}
	token := contribExtractToken(r)
	session, err := auth.ValidateSession(r.Context(), d.DB, token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	// Resolve the owner slug from the device. Devices store owner in
	// the device table; for this MVP we accept the device_id as the
	// owner if no mapping exists. In production the deviceOwner helper
	// from owner_profile.go would be wired in.
	owner := contribOwnerForSession(r.Context(), d.DB, session.DeviceID)
	if owner == "" {
		owner = session.DeviceID
	}

	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("contrib: ws upgrade: %v", err)
		return
	}
	defer wsConn.Close(websocket.StatusNormalClosure, "")
	wsConn.SetReadLimit(1 << 20) // 1 MB; deck text can be sizable

	// Per-session HMAC key. Random 32 bytes hex-encoded.
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		log.Printf("contrib: rand key: %v", err)
		return
	}
	sessionKey := hex.EncodeToString(keyBytes)

	if err := db.EnsureContributorRow(r.Context(), d.DB, owner, time.Now().Unix()); err != nil {
		log.Printf("contrib: ensure row %q: %v", owner, err)
	}

	// Welcome.
	welcome := ContribWelcome{
		Owner:       owner,
		HMACKey:     sessionKey,
		ServerTime:  time.Now().Unix(),
		ProtocolVer: ProtocolVersion,
	}
	if err := contribSendEnvelope(r.Context(), wsConn, "welcome", welcome); err != nil {
		log.Printf("contrib: send welcome: %v", err)
		return
	}

	// In-flight assignment so we can requeue on disconnect.
	var inFlight *ContribAssignment

	// Dispatcher loop.
	ctx := r.Context()
	for {
		_, data, err := wsConn.Read(ctx)
		if err != nil {
			if inFlight != nil {
				d.requeue(inFlight)
			}
			return
		}

		var env ContribEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			contribSendEnvelope(ctx, wsConn, "error", map[string]string{"message": "bad envelope"})
			continue
		}

		switch env.Type {
		case "ready":
			var ready ContribReady
			_ = json.Unmarshal(env.Payload, &ready)
			a := d.dequeue()
			if a == nil {
				contribSendEnvelope(ctx, wsConn, "wait", map[string]any{"retry_after_seconds": 10})
				continue
			}
			// Sign and send.
			if _, err := SignAssignment(a, keyBytes); err != nil {
				log.Printf("contrib: sign assignment %s: %v", a.ChunkID, err)
				d.requeue(a)
				continue
			}
			if err := db.RecordChunkAssignment(ctx, d.DB, a.ChunkID, owner, a.GamesCount, a.NSeats, a.IssuedAt); err != nil {
				log.Printf("contrib: record chunk %s: %v", a.ChunkID, err)
			}
			inFlight = a
			if err := contribSendEnvelope(ctx, wsConn, "assignment", a); err != nil {
				log.Printf("contrib: send assignment: %v", err)
				d.requeue(a)
				return
			}

		case "result":
			var res ContribResult
			if err := json.Unmarshal(env.Payload, &res); err != nil {
				contribSendEnvelope(ctx, wsConn, "ack", ContribAck{Accepted: false, Reason: "bad result payload"})
				continue
			}
			ack := d.processResult(ctx, owner, &res, keyBytes, inFlight)
			contribSendEnvelope(ctx, wsConn, "ack", ack)
			inFlight = nil

		case "ping":
			contribSendEnvelope(ctx, wsConn, "pong", map[string]int64{"server_time": time.Now().Unix()})

		case "bye":
			if inFlight != nil {
				d.requeue(inFlight)
			}
			return

		default:
			contribSendEnvelope(ctx, wsConn, "error", map[string]string{"message": "unknown type: " + env.Type})
		}
	}
}

// processResult validates a returned chunk and, if accepted, accrues
// credits. inFlight is the assignment we last handed this client; it's
// the only chunk we accept right now (one-at-a-time per connection).
func (d *ContribDispatcher) processResult(ctx context.Context, owner string, res *ContribResult, key []byte, inFlight *ContribAssignment) ContribAck {
	now := time.Now().Unix()
	ack := ContribAck{ChunkID: res.ChunkID}

	if inFlight == nil || inFlight.ChunkID != res.ChunkID {
		ack.Accepted = false
		ack.Reason = "no matching in-flight assignment"
		_ = db.RejectChunk(ctx, d.DB, owner, now)
		_ = db.FinalizeChunkRow(ctx, d.DB, res.ChunkID, -1, false, false, res.ElapsedMS, res.OutcomeHash, 0, ack.Reason, now)
		return ack
	}

	if err := VerifyResult(res, key); err != nil {
		ack.Accepted = false
		ack.Reason = "result signature invalid: " + err.Error()
		_ = db.RejectChunk(ctx, d.DB, owner, now)
		_ = db.FinalizeChunkRow(ctx, d.DB, res.ChunkID, -1, false, false, res.ElapsedMS, res.OutcomeHash, 0, ack.Reason, now)
		return ack
	}

	if len(res.Winners) != inFlight.GamesCount {
		ack.Accepted = false
		ack.Reason = "winners length mismatch"
		_ = db.RejectChunk(ctx, d.DB, owner, now)
		_ = db.FinalizeChunkRow(ctx, d.DB, res.ChunkID, -1, false, false, res.ElapsedMS, res.OutcomeHash, 0, ack.Reason, now)
		return ack
	}

	// Hash check (client-claimed).
	expectedHash := HashOutcomes(res.Winners, res.TurnCounts)
	if res.OutcomeHash != expectedHash {
		ack.Accepted = false
		ack.Reason = "outcome hash mismatch"
		_ = db.RejectChunk(ctx, d.DB, owner, now)
		_ = db.FinalizeChunkRow(ctx, d.DB, res.ChunkID, -1, false, false, res.ElapsedMS, res.OutcomeHash, 0, ack.Reason, now)
		return ack
	}

	// Optional spot-check: re-run locally and compare hashes.
	spotChecked := false
	spotPassed := false
	if d.shouldSpotCheck() {
		spotChecked = true
		ack.SpotChecked = true
		winners, turns, ok := d.SpotCheck(inFlight)
		if !ok {
			ack.Accepted = false
			ack.Reason = "spot-check runner failed"
			_ = db.RejectChunk(ctx, d.DB, owner, now)
			_ = db.FinalizeChunkRow(ctx, d.DB, res.ChunkID, -1, true, false, res.ElapsedMS, res.OutcomeHash, 0, ack.Reason, now)
			return ack
		}
		localHash := HashOutcomes(winners, turns)
		if localHash != res.OutcomeHash {
			ack.Accepted = false
			ack.Reason = "spot-check parity failure"
			_ = db.RejectChunk(ctx, d.DB, owner, now)
			_ = db.FinalizeChunkRow(ctx, d.DB, res.ChunkID, -1, true, false, res.ElapsedMS, res.OutcomeHash, 0, ack.Reason, now)
			return ack
		}
		spotPassed = true
	}

	credits := CreditsForChunk(inFlight.GamesCount, inFlight.NSeats)
	c, err := db.AwardCredits(ctx, d.DB, owner, credits, int64(inFlight.GamesCount), res.ElapsedMS, d.FreezeAtZScore, now)
	if err != nil {
		ack.Accepted = false
		ack.Reason = "credit accrual failed: " + err.Error()
		_ = db.FinalizeChunkRow(ctx, d.DB, res.ChunkID, -1, spotChecked, spotPassed, res.ElapsedMS, res.OutcomeHash, 0, ack.Reason, now)
		return ack
	}

	awarded := credits
	reason := ""
	if c.Frozen {
		// Welford-detected anomaly tripped the freeze on THIS chunk.
		// Credit award was suppressed inside AwardCredits.
		awarded = 0
		reason = "frozen by anomaly: " + c.FrozenReason
	}
	_ = db.FinalizeChunkRow(ctx, d.DB, res.ChunkID, 1, spotChecked, spotPassed, res.ElapsedMS, res.OutcomeHash, awarded, reason, now)

	ack.Accepted = true
	ack.CreditsAwarded = awarded
	ack.SpotChecked = spotChecked
	ack.AnomalyZScore = c.LastZScore
	if c.Frozen && reason != "" {
		ack.Reason = reason
	}
	return ack
}

// --- Helpers ----------------------------------------------------------

func contribExtractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		const p = "Bearer "
		if len(h) > len(p) && h[:len(p)] == p {
			return h[len(p):]
		}
	}
	return r.URL.Query().Get("token")
}

func contribOwnerForSession(ctx context.Context, database *sql.DB, deviceID string) string {
	// Best-effort lookup. The device table's display_name is the
	// closest stable per-account string we have at the engine layer
	// (the deck/profile layer adds its own email→owner mapping that
	// the dispatcher doesn't need to depend on). Lowercased so it
	// matches the slug convention used by deck owner fields.
	var displayName sql.NullString
	err := database.QueryRowContext(ctx,
		`SELECT display_name FROM device WHERE id = ?`, deviceID,
	).Scan(&displayName)
	if err != nil || !displayName.Valid || displayName.String == "" {
		return ""
	}
	return strLower(displayName.String)
}

// strLower lowercases ASCII without importing strings into this file —
// avoids one more import for a single call site.
func strLower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}

func contribSendEnvelope(ctx context.Context, c *websocket.Conn, kind string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env := ContribEnvelope{Type: kind, Payload: json.RawMessage(data)}
	out, err := json.Marshal(env)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.Write(wctx, websocket.MessageText, out)
}

// ParseSpotCheckPct reads HEXDEK_CONTRIB_SPOTCHECK_PCT (a number 0–100)
// from the env. Returns the default if missing or unparseable.
func ParseSpotCheckPct(envVal string, def float64) float64 {
	if envVal == "" {
		return def
	}
	f, err := strconv.ParseFloat(envVal, 64)
	if err != nil || f < 0 || f > 100 {
		return def
	}
	return f / 100.0
}

// ErrNoChunk is returned when the queue is empty; clients should retry.
var ErrNoChunk = errors.New("contrib: queue empty")
