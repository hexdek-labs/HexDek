// full_game_smoke.go — Ship 4b/5 end-to-end test: WebSocket game dispatch.
//
// Demonstrates that game actions sent over WebSocket are validated by the
// engine, applied to SQLite, and broadcast back as state_updates to all
// connected players.
//
// Run with: go run scripts/full_game_smoke.go
// Server must be running on :8765.

//go:build ignore

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const baseURL = "http://localhost:8765"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log.SetFlags(0)
	log.Println("=== FULL GAME SMOKE — Ship 4b/5 WebSocket dispatch ===")

	// Setup
	hex := registerDevice(ctx, "Hex")
	josh := registerDevice(ctx, "Josh")
	hexDeckID := importDeck(ctx, hex.Device.ID, "Yuriko (Hex)")
	joshDeckID := importDeck(ctx, josh.Device.ID, "Yuriko (Josh)")
	partyID := createParty(ctx, hex.Device.ID, 2)
	setDeck(ctx, partyID, hex.Device.ID, hexDeckID)
	joinParty(ctx, partyID, josh.Device.ID, joshDeckID)
	log.Printf("Setup complete — party %s, hex=%s, josh=%s", partyID, hex.Device.ID, josh.Device.ID)

	// Connect WS for both
	hexWS := connectWS(ctx, partyID, hex.Session.Token)
	joshWS := connectWS(ctx, partyID, josh.Session.Token)
	defer hexWS.Close(websocket.StatusNormalClosure, "")
	defer joshWS.Close(websocket.StatusNormalClosure, "")

	// Don't drain initial welcomes — we'll skip past them when reading
	// message types we care about. Wait briefly for handshake to settle.
	time.Sleep(500 * time.Millisecond)

	// Start the game
	gameID := startGame(ctx, partyID)
	log.Printf("Game started: %s", gameID)

	// Hex requests snapshot via WS — read messages until we get a state_update
	send(ctx, hexWS, "game.snapshot", nil)
	hexState := readUntilType(ctx, hexWS, "game.state_update", "Hex initial snapshot")

	// Find first Swamp in Hex's hand
	swampInstanceID := findFirstCardInHand(hexState, "Swamp")
	if swampInstanceID == "" {
		// Try Island as fallback
		swampInstanceID = findFirstCardInHand(hexState, "Island")
	}
	if swampInstanceID == "" {
		log.Fatalf("Hex has no Swamp or Island in opening hand — try re-running with new shuffle")
	}
	log.Printf("Hex playing land instance: %s", swampInstanceID)

	// Hex plays the land
	send(ctx, hexWS, "game.play_land", map[string]string{"instance_id": swampInstanceID})
	readUntilType(ctx, hexWS, "game.state_update", "Hex state after play_land")
	readUntilType(ctx, joshWS, "game.state_update", "Josh state after Hex play_land (broadcast verified)")

	// Hex taps the land for mana (let server infer color)
	send(ctx, hexWS, "game.tap_land", map[string]string{"instance_id": swampInstanceID})
	hexAfterTap := readUntilType(ctx, hexWS, "game.state_update", "Hex state after tap_land")
	readUntilType(ctx, joshWS, "game.state_update", "Josh state after Hex tap_land")

	// Verify mana pool now has at least 1 mana of some color
	pool := extractManaPool(hexAfterTap)
	totalMana := pool["W"] + pool["U"] + pool["B"] + pool["R"] + pool["G"] + pool["C"]
	log.Printf("Hex's mana pool after tap: %v (total: %d)", pool, totalMana)
	if totalMana < 1 {
		log.Fatalf("expected at least 1 mana in pool after tap, got 0")
	}

	// Hex advances phase (Main1 → Combat)
	send(ctx, hexWS, "game.advance_phase", nil)
	readUntilType(ctx, hexWS, "game.state_update", "Hex state after advance_phase")
	readUntilType(ctx, joshWS, "game.state_update", "Josh state after advance_phase")

	log.Println("")
	log.Println("=== ALL WEBSOCKET GAME DISPATCH TESTS PASSED ===")
	log.Println("Ship 4b: actions over WS work (play_land, tap_land, advance_phase)")
	log.Println("Ship 5: per-action state broadcast to all connected players verified")
}

// ---------- helpers ----------

type registerResp struct {
	Device  struct{ ID string `json:"ID"` } `json:"device"`
	Session struct{ Token string `json:"Token"` } `json:"session"`
}

func registerDevice(ctx context.Context, name string) registerResp {
	body, _ := json.Marshal(map[string]string{"display_name": name})
	resp := postJSON(ctx, "/api/device/register", body)
	var out registerResp
	if err := json.Unmarshal(resp, &out); err != nil {
		log.Fatalf("register %s: parse: %v", name, err)
	}
	return out
}

func importDeck(ctx context.Context, deviceID, name string) string {
	deckBytes, _ := io.ReadAll(mustOpen("data/decks/yuriko_v1.json"))
	body, _ := json.Marshal(map[string]any{
		"owner_device_id": deviceID,
		"name":            name,
		"deck":            json.RawMessage(deckBytes),
	})
	resp := postJSON(ctx, "/api/deck/import", body)
	var out struct {
		ID string `json:"ID"`
	}
	json.Unmarshal(resp, &out)
	return out.ID
}

func createParty(ctx context.Context, hostID string, max int) string {
	body, _ := json.Marshal(map[string]any{"host_device_id": hostID, "max_players": max})
	resp := postJSON(ctx, "/api/party/create", body)
	var out struct{ ID string `json:"ID"` }
	json.Unmarshal(resp, &out)
	return out.ID
}

func setDeck(ctx context.Context, partyID, deviceID, deckID string) {
	body, _ := json.Marshal(map[string]string{"device_id": deviceID, "deck_id": deckID})
	postJSON(ctx, fmt.Sprintf("/api/party/%s/set_deck", partyID), body)
}

func joinParty(ctx context.Context, partyID, deviceID, deckID string) {
	body, _ := json.Marshal(map[string]string{"device_id": deviceID, "deck_id": deckID})
	postJSON(ctx, fmt.Sprintf("/api/party/%s/join", partyID), body)
}

func startGame(ctx context.Context, partyID string) string {
	resp := postJSON(ctx, fmt.Sprintf("/api/party/%s/start_game", partyID), nil)
	var out struct{ ID string `json:"id"` }
	json.Unmarshal(resp, &out)
	return out.ID
}

func postJSON(ctx context.Context, path string, body []byte) []byte {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		log.Fatalf("POST %s: status %d: %s", path, resp.StatusCode, data)
	}
	return data
}

func connectWS(ctx context.Context, partyID, token string) *websocket.Conn {
	url := fmt.Sprintf("ws://localhost:8765/ws/party/%s?token=%s", partyID, token)
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		log.Fatalf("ws dial: %v", err)
	}
	return c
}

func send(ctx context.Context, c *websocket.Conn, msgType string, payload any) {
	env := map[string]any{"type": msgType}
	if payload != nil {
		env["payload"] = payload
	}
	b, _ := json.Marshal(env)
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		log.Fatalf("ws write: %v", err)
	}
}

func readMsg(ctx context.Context, c *websocket.Conn, label string) map[string]any {
	rctx := timed(ctx, 5*time.Second)
	_, data, err := c.Read(rctx)
	if err != nil {
		log.Fatalf("%s: %v", label, err)
	}
	var env map[string]any
	json.Unmarshal(data, &env)
	preview := string(data)
	if len(preview) > 180 {
		preview = preview[:180] + "..."
	}
	log.Printf("  → %s: type=%v %s", label, env["type"], preview[:min(120, len(preview))])
	return env
}

// readUntilType reads messages until one of the given type arrives,
// dropping intermediate messages (welcome, device_joined, chat, etc.)
func readUntilType(ctx context.Context, c *websocket.Conn, wantType, label string) map[string]any {
	for i := 0; i < 10; i++ {
		rctx := timed(ctx, 5*time.Second)
		_, data, err := c.Read(rctx)
		if err != nil {
			log.Fatalf("%s: read error: %v", label, err)
		}
		var env map[string]any
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env["type"] == wantType {
			log.Printf("  → %s (received %s)", label, wantType)
			return env
		}
		// drop other types silently (welcome, device_joined, etc.)
	}
	log.Fatalf("%s: did not receive %s within 10 messages", label, wantType)
	return nil
}

func findFirstCardInHand(state map[string]any, cardName string) string {
	payload, ok := state["payload"].(map[string]any)
	if !ok {
		return ""
	}
	hand, ok := payload["your_hand"].([]any)
	if !ok {
		return ""
	}
	for _, c := range hand {
		card := c.(map[string]any)
		if name, _ := card["name"].(string); name == cardName {
			if id, _ := card["instance_id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}

func extractManaPool(state map[string]any) map[string]int {
	out := map[string]int{}
	payload, _ := state["payload"].(map[string]any)
	you, _ := payload["you"].(map[string]any)
	for _, color := range []string{"W", "U", "B", "R", "G", "C"} {
		key := "mana_pool_" + map[string]string{"W": "w", "U": "u", "B": "b", "R": "r", "G": "g", "C": "c"}[color]
		if v, ok := you[key].(float64); ok {
			out[color] = int(v)
		}
	}
	return out
}

func timed(parent context.Context, d time.Duration) context.Context {
	ctx, _ := context.WithTimeout(parent, d)
	return ctx
}

func mustOpen(path string) io.ReadCloser {
	f, err := http.Dir(".").Open(path)
	if err != nil {
		log.Fatalf("open %s: %v", path, err)
	}
	return f
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
