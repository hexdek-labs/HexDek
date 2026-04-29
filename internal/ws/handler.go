// Package ws implements the WebSocket transport for mtgsquad. It bridges
// the coder/websocket library to the package-internal hub for connection
// tracking and broadcast.
//
// Wire format: JSON-encoded messages with a {type, payload} envelope.
package ws

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/hexdek/hexdek/internal/auth"
	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/game"
	"github.com/hexdek/hexdek/internal/hub"
)

// Handler exposes the WebSocket endpoint and message router.
type Handler struct {
	DB  *sql.DB
	Hub *hub.Hub
}

// Register adds the WS routes to a mux.
func (h *Handler) Register(mux *http.ServeMux) {
	// Note: we authenticate inside the handler rather than via middleware
	// because WebSocket clients often can't easily set Authorization headers
	// pre-handshake. Token can be passed via ?token= query param too.
	mux.HandleFunc("GET /ws/party/{id}", h.handleWS)
}

// envelope is the on-the-wire message format.
type envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// connection wraps a coder/websocket.Conn to satisfy the hub.Conn interface.
type connection struct {
	wsConn   *websocket.Conn
	deviceID string
	mu       sync.Mutex // serializes writes
}

func (c *connection) Send(ctx context.Context, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.wsConn.Write(wctx, websocket.MessageText, payload)
}

func (c *connection) Close() error {
	return c.wsConn.Close(websocket.StatusNormalClosure, "")
}

func (c *connection) DeviceID() string { return c.deviceID }

// handleWS upgrades the connection, authenticates, registers with hub,
// and runs the per-connection read loop.
func (h *Handler) handleWS(w http.ResponseWriter, r *http.Request) {
	partyID := r.PathValue("id")
	if partyID == "" {
		http.Error(w, "missing party id", http.StatusBadRequest)
		return
	}

	// Authenticate via Authorization header or ?token= query param
	token := extractToken(r)
	session, err := auth.ValidateSession(r.Context(), h.DB, token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify the device is a member of this party
	if err := h.verifyMembership(r.Context(), partyID, session.DeviceID); err != nil {
		http.Error(w, "not a member of this party", http.StatusForbidden)
		return
	}

	// Upgrade
	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	wsConn.SetReadLimit(8192)
	conn := &connection{wsConn: wsConn, deviceID: session.DeviceID}

	h.Hub.Register(partyID, conn)
	defer h.Hub.Unregister(partyID, conn)
	defer conn.Close()

	// Send a welcome state-update so client knows it's connected
	welcome, _ := json.Marshal(envelope{
		Type: "welcome",
		Payload: mustJSON(map[string]any{
			"device_id":         session.DeviceID,
			"party_id":          partyID,
			"connected_devices": h.Hub.PartyDeviceIDs(partyID),
		}),
	})
	_ = conn.Send(r.Context(), welcome)

	// Notify other party members that this device joined
	joinNotice, _ := json.Marshal(envelope{
		Type: "device_joined",
		Payload: mustJSON(map[string]string{
			"device_id": session.DeviceID,
			"party_id":  partyID,
		}),
	})
	for _, sendErr := range h.Hub.Broadcast(r.Context(), partyID, joinNotice) {
		log.Printf("ws broadcast send error: %v", sendErr)
	}

	// Read loop
	ctx := r.Context()
	for {
		_, data, err := wsConn.Read(ctx)
		if err != nil {
			// Closed or read failure — exit loop, defer triggers cleanup
			return
		}
		h.dispatch(ctx, partyID, conn, data)
	}
}

func (h *Handler) dispatch(ctx context.Context, partyID string, conn *connection, data []byte) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		errMsg, _ := json.Marshal(envelope{
			Type:    "error",
			Payload: mustJSON(map[string]string{"message": "invalid JSON envelope"}),
		})
		_ = conn.Send(ctx, errMsg)
		return
	}

	switch env.Type {
	case "ping":
		pong, _ := json.Marshal(envelope{
			Type:    "pong",
			Payload: mustJSON(map[string]int64{"server_time": db.Now()}),
		})
		_ = conn.Send(ctx, pong)

	case "chat":
		// Re-broadcast chat messages to all party members
		var msg struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(env.Payload, &msg)
		out, _ := json.Marshal(envelope{
			Type: "chat",
			Payload: mustJSON(map[string]any{
				"from_device": conn.DeviceID(),
				"text":        msg.Text,
				"timestamp":   db.Now(),
			}),
		})
		for _, err := range h.Hub.Broadcast(ctx, partyID, out) {
			log.Printf("chat broadcast error: %v", err)
		}

	case "state_update":
		// Re-broadcast state updates as-is (Ship 4 will validate game state)
		out, _ := json.Marshal(envelope{
			Type: "state_update",
			Payload: mustJSON(map[string]any{
				"from_device": conn.DeviceID(),
				"payload":     env.Payload,
				"timestamp":   db.Now(),
			}),
		})
		for _, err := range h.Hub.Broadcast(ctx, partyID, out) {
			log.Printf("state_update broadcast error: %v", err)
		}

	case "game.play_land":
		h.handlePlayLand(ctx, partyID, conn, env.Payload)
	case "game.tap_land":
		h.handleTapLand(ctx, partyID, conn, env.Payload)
	case "game.cast_spell":
		h.handleCastSpell(ctx, partyID, conn, env.Payload)
	case "game.draw":
		h.handleDraw(ctx, partyID, conn, env.Payload)
	case "game.advance_phase":
		h.handleAdvancePhase(ctx, partyID, conn)
	case "game.yuriko_reveal":
		h.handleYurikoReveal(ctx, partyID, conn, env.Payload)
	case "game.snapshot":
		h.handleSnapshot(ctx, partyID, conn)
	case "game.declare_attackers":
		h.handleDeclareAttackers(ctx, partyID, conn, env.Payload)
	case "game.declare_blockers":
		h.handleDeclareBlockers(ctx, partyID, conn, env.Payload)
	case "game.resolve_combat":
		h.handleResolveCombat(ctx, partyID, conn)
	case "game.tap_card":
		h.handleSetTapped(ctx, partyID, conn, env.Payload, true)
	case "game.untap_card":
		h.handleSetTapped(ctx, partyID, conn, env.Payload, false)
	case "game.untap_all":
		h.handleUntapAll(ctx, partyID, conn)
	case "game.adjust_life":
		h.handleAdjustLife(ctx, partyID, conn, env.Payload)

	default:
		errMsg, _ := json.Marshal(envelope{
			Type:    "error",
			Payload: mustJSON(map[string]string{"message": "unknown message type: " + env.Type}),
		})
		_ = conn.Send(ctx, errMsg)
	}
}

// ---------- game action handlers ----------

func (h *Handler) gameAndSeat(ctx context.Context, partyID, deviceID string) (gameID string, seat int, err error) {
	gameID, err = db.GetActiveGameForParty(ctx, h.DB, partyID)
	if err != nil {
		return "", 0, fmt.Errorf("no active game for party: %w", err)
	}
	members, err := db.ListPartyMembers(ctx, h.DB, partyID)
	if err != nil {
		return "", 0, err
	}
	for _, m := range members {
		if m.DeviceID == deviceID {
			return gameID, m.SeatPosition, nil
		}
	}
	return "", 0, fmt.Errorf("device %s not in party %s", deviceID, partyID)
}

// BroadcastSnapshots is the exported entry point for non-WS callers (e.g.
// the AI autopilot) who drive game state and need to fan out updates.
func (h *Handler) BroadcastSnapshots(ctx context.Context, partyID, gameID string) {
	h.broadcastSnapshots(ctx, partyID, gameID)
}

// broadcastSnapshots fetches a per-seat snapshot for every connected device
// and sends each one their personalized view. This is the fan-out after any
// state-changing action.
func (h *Handler) broadcastSnapshots(ctx context.Context, partyID, gameID string) {
	members, err := db.ListPartyMembers(ctx, h.DB, partyID)
	if err != nil {
		log.Printf("broadcast: list members: %v", err)
		return
	}
	for _, m := range members {
		snap, err := game.Snapshot(ctx, h.DB, gameID, m.SeatPosition)
		if err != nil {
			log.Printf("broadcast: snapshot for seat %d: %v", m.SeatPosition, err)
			continue
		}
		msg, err := json.Marshal(envelope{
			Type:    "game.state_update",
			Payload: mustJSON(snap),
		})
		if err != nil {
			continue
		}
		_ = h.Hub.SendToDevice(ctx, partyID, m.DeviceID, msg)
	}
}

func (h *Handler) sendErr(ctx context.Context, conn *connection, msg string) {
	errMsg, _ := json.Marshal(envelope{
		Type:    "error",
		Payload: mustJSON(map[string]string{"message": msg}),
	})
	_ = conn.Send(ctx, errMsg)
}

func (h *Handler) handlePlayLand(ctx context.Context, partyID string, conn *connection, payload json.RawMessage) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Override   bool   `json:"override"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if _, err := game.PlayLand(ctx, h.DB, gameID, seat, req.InstanceID, req.Override); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleTapLand(ctx context.Context, partyID string, conn *connection, payload json.RawMessage) {
	var req struct {
		InstanceID   string `json:"instance_id"`
		ChosenColor  string `json:"chosen_color"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if _, err := game.TapLandForMana(ctx, h.DB, gameID, seat, req.InstanceID, req.ChosenColor); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleCastSpell(ctx context.Context, partyID string, conn *connection, payload json.RawMessage) {
	var req struct {
		InstanceID string `json:"instance_id"`
		XValue     int    `json:"x_value"`
		Override   bool   `json:"override"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if _, err := game.CastSpell(ctx, h.DB, gameID, seat, req.InstanceID, req.XValue, req.Override); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleDraw(ctx context.Context, partyID string, conn *connection, payload json.RawMessage) {
	var req struct {
		Count    int  `json:"count"`
		Override bool `json:"override"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	if req.Count < 1 {
		req.Count = 1
	}
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if _, err := game.DrawCards(ctx, h.DB, gameID, seat, req.Count, req.Override); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleSetTapped(ctx context.Context, partyID string, conn *connection, payload json.RawMessage, tapped bool) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if err := game.SetTapped(ctx, h.DB, gameID, seat, req.InstanceID, tapped); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleUntapAll(ctx context.Context, partyID string, conn *connection) {
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if err := game.UntapAllForSeat(ctx, h.DB, gameID, seat); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleAdjustLife(ctx context.Context, partyID string, conn *connection, payload json.RawMessage) {
	var req struct {
		Delta int `json:"delta"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if err := game.AdjustLife(ctx, h.DB, gameID, seat, req.Delta); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleAdvancePhase(ctx context.Context, partyID string, conn *connection) {
	gameID, _, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	members, err := db.ListPartyMembers(ctx, h.DB, partyID)
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if _, err := game.AdvancePhase(ctx, h.DB, gameID, len(members)); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleYurikoReveal(ctx context.Context, partyID string, conn *connection, payload json.RawMessage) {
	var req struct {
		TargetSeat int `json:"target_seat"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	gameID, attackerSeat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	revealed, damage, err := game.YurikoReveal(ctx, h.DB, gameID, attackerSeat, req.TargetSeat)
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	// Broadcast a special yuriko_trigger event with the revealed card visible
	// to ALL players (it's a public reveal).
	notice, _ := json.Marshal(envelope{
		Type: "game.yuriko_triggered",
		Payload: mustJSON(map[string]any{
			"attacker_seat":  attackerSeat,
			"target_seat":    req.TargetSeat,
			"revealed_card":  revealed.Name,
			"revealed_cmc":   revealed.CMC,
			"damage_dealt":   damage,
		}),
	})
	for _, sendErr := range h.Hub.Broadcast(ctx, partyID, notice) {
		log.Printf("yuriko broadcast: %v", sendErr)
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleDeclareAttackers(ctx context.Context, partyID string, conn *connection, payload json.RawMessage) {
	var req struct {
		Attackers []game.AttackerSpec `json:"attackers"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if err := game.DeclareAttackers(ctx, h.DB, gameID, seat, req.Attackers); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleDeclareBlockers(ctx context.Context, partyID string, conn *connection, payload json.RawMessage) {
	var req struct {
		Blockers []game.BlockerSpec `json:"blockers"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendErr(ctx, conn, "invalid payload: "+err.Error())
		return
	}
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	if err := game.DeclareBlockers(ctx, h.DB, gameID, seat, req.Blockers); err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleResolveCombat(ctx context.Context, partyID string, conn *connection) {
	gameID, _, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	report, err := game.ResolveCombat(ctx, h.DB, gameID)
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	// Broadcast a combat-resolved event with the damage report
	notice, _ := json.Marshal(envelope{
		Type:    "game.combat_resolved",
		Payload: mustJSON(report),
	})
	for _, sendErr := range h.Hub.Broadcast(ctx, partyID, notice) {
		log.Printf("combat resolved broadcast: %v", sendErr)
	}
	h.broadcastSnapshots(ctx, partyID, gameID)
}

func (h *Handler) handleSnapshot(ctx context.Context, partyID string, conn *connection) {
	gameID, seat, err := h.gameAndSeat(ctx, partyID, conn.DeviceID())
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	snap, err := game.Snapshot(ctx, h.DB, gameID, seat)
	if err != nil {
		h.sendErr(ctx, conn, err.Error())
		return
	}
	msg, _ := json.Marshal(envelope{
		Type:    "game.state_update",
		Payload: mustJSON(snap),
	})
	_ = conn.Send(ctx, msg)
}

func (h *Handler) verifyMembership(ctx context.Context, partyID, deviceID string) error {
	members, err := db.ListPartyMembers(ctx, h.DB, partyID)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.DeviceID == deviceID {
			return nil
		}
	}
	return errors.New("device is not a member of this party")
}

func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		const prefix = "Bearer "
		if len(h) > len(prefix) && h[:len(prefix)] == prefix {
			return h[len(prefix):]
		}
	}
	return r.URL.Query().Get("token")
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("mustJSON: " + err.Error())
	}
	return b
}
