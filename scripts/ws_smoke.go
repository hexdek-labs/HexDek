// ws_smoke_test.go — manual integration smoke test for the WebSocket layer.
//
// Run with: go run scripts/ws_smoke_test.go
// Requires the server to be running on :8765.
//
// This is intentionally NOT a Go test (//go:build ignore) so it doesn't
// run as part of `go test ./...`. It's a one-shot diagnostic tool for
// verifying the full Ship 1+2+3 stack against a live server.

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.SetFlags(0)
	log.Println("=== WS SMOKE TEST — Ship 1+2+3 end-to-end ===")

	// 1. Register 2 devices
	hexDev := registerDevice(ctx, "Hex")
	joshDev := registerDevice(ctx, "Josh")
	log.Printf("Hex device:  %s (token: %s...)", hexDev.Device.ID, hexDev.Session.Token[:16])
	log.Printf("Josh device: %s (token: %s...)", joshDev.Device.ID, joshDev.Session.Token[:16])

	// 2. Create a party with Hex as host
	partyID := createParty(ctx, hexDev.Device.ID, 4)
	log.Printf("Party created: %s", partyID)

	// 3. Josh joins the party
	joinParty(ctx, partyID, joshDev.Device.ID)
	log.Printf("Josh joined party %s", partyID)

	// 4. Both connect to WebSocket
	hexWS := connectWS(ctx, partyID, hexDev.Session.Token)
	joshWS := connectWS(ctx, partyID, joshDev.Session.Token)
	defer hexWS.Close(websocket.StatusNormalClosure, "")
	defer joshWS.Close(websocket.StatusNormalClosure, "")
	log.Println("Both clients connected to WebSocket")

	// Drain initial welcome messages
	drainOne(ctx, hexWS, "Hex receives welcome")
	drainOne(ctx, joshWS, "Josh receives welcome")

	// 5. Hex sends a ping, expects pong
	send(ctx, hexWS, "ping", nil)
	drainOne(ctx, hexWS, "Hex receives pong")

	// 6. Hex sends a chat, both should receive
	send(ctx, hexWS, "chat", map[string]string{"text": "Doomsday inbound."})
	drainOne(ctx, hexWS, "Hex receives chat broadcast (own message)")
	drainOne(ctx, joshWS, "Josh receives chat broadcast (Hex's message)")

	// 7. Josh sends state_update, both should receive
	send(ctx, joshWS, "state_update", map[string]any{
		"action":     "play_card",
		"card_name":  "Counterspell",
		"target":     "Doomsday",
	})
	drainOne(ctx, joshWS, "Josh receives state_update broadcast (own)")
	drainOne(ctx, hexWS, "Hex receives state_update broadcast (Josh's)")

	log.Println("=== ALL SMOKE TESTS PASSED ===")
}

type registerResp struct {
	Device  struct{ ID string `json:"ID"` } `json:"device"`
	Session struct{ Token string `json:"Token"` } `json:"session"`
}

func registerDevice(ctx context.Context, name string) registerResp {
	body, _ := json.Marshal(map[string]string{"display_name": name})
	resp := postJSON(ctx, "/api/device/register", body)
	var out registerResp
	if err := json.Unmarshal(resp, &out); err != nil {
		log.Fatalf("register %s: parse: %v\nbody: %s", name, err, resp)
	}
	return out
}

func createParty(ctx context.Context, hostDeviceID string, max int) string {
	body, _ := json.Marshal(map[string]any{"host_device_id": hostDeviceID, "max_players": max})
	resp := postJSON(ctx, "/api/party/create", body)
	var out struct {
		ID string `json:"ID"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		log.Fatalf("create party: parse: %v\nbody: %s", err, resp)
	}
	return out.ID
}

func joinParty(ctx context.Context, partyID, deviceID string) {
	body, _ := json.Marshal(map[string]string{"device_id": deviceID})
	postJSON(ctx, fmt.Sprintf("/api/party/%s/join", partyID), body)
}

func postJSON(ctx context.Context, path string, body []byte) []byte {
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
		log.Fatalf("ws dial %s: %v", url, err)
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

func drainOne(ctx context.Context, c *websocket.Conn, label string) {
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, data, err := c.Read(rctx)
	if err != nil {
		log.Fatalf("%s: read error: %v", label, err)
	}
	log.Printf("  → %s: %s", label, string(data))
}
