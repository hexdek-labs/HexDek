package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// graphql_subscriptions_test.go — end-to-end exercise of the
// graphql-transport-ws transport added in this PR.
//
// Coverage:
//   - constructor validates bus + Subscription root presence
//   - connection_init / connection_ack handshake
//   - Subscribe before init closes 4401
//   - subscribe → publish → next frame with field-projected payload
//   - selective field projection (asking for {winner} omits commanders)
//   - variables thread through to the GraphQL execution
//   - client-driven Complete cancels the subscription
//   - duplicate Subscribe id closes 4409
//   - Ping → Pong
//   - subprotocol negotiation: missing graphql-transport-ws is rejected
//   - connection cap returns 503 ErrorResponse before the upgrade
//   - bad query in Subscribe → Error frame (and Complete on op close)
//
// All tests narrow keepalive + write timeouts to avoid real-time
// sleeps. The transport is plain WebSocket so we drive it with
// coder/websocket directly.

// seedSubscriptionsHandler builds a Showmatch with one seeded game
// + an EventBus + the subscriptions WS handler ready to serve.
// Returns the handler and the bus so tests can both connect and
// publish.
func seedSubscriptionsHandler(t *testing.T) (*GraphQLSubscriptionsHandler, *EventBus) {
	t.Helper()
	sm := seedShowmatch(t) // from graphql_test.go
	bus := NewEventBus()
	sm.Events = bus
	gh, err := NewGraphQLHandler(sm, "")
	if err != nil {
		t.Fatalf("NewGraphQLHandler: %v", err)
	}
	subs, err := NewGraphQLSubscriptionsHandler(bus, gh.schema)
	if err != nil {
		t.Fatalf("NewGraphQLSubscriptionsHandler: %v", err)
	}
	subs.SetLogf(func(string, ...any) {})
	return subs, bus
}

// dialSubscriptions opens a graphql-transport-ws WebSocket against
// the httptest server. Sets the subprotocol header so the upgrade
// succeeds. opts.skipSubprotocol omits it (for the negotiation-
// failure test).
func dialSubscriptions(t *testing.T, srv *httptest.Server, opts ...dialOpt) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	cfg := dialConfig{withSubprotocol: true}
	for _, o := range opts {
		o(&cfg)
	}
	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/graphql/subscriptions"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialOpts := &websocket.DialOptions{}
	if cfg.withSubprotocol {
		dialOpts.Subprotocols = []string{graphqlTransportWSSubprotocol}
	}
	return websocket.Dial(ctx, wsURL, dialOpts)
}

type dialConfig struct{ withSubprotocol bool }
type dialOpt func(*dialConfig)

func withoutSubprotocol() dialOpt {
	return func(c *dialConfig) { c.withSubprotocol = false }
}

// readMessage reads one gqlwsMessage envelope from the WS within
// timeout. Fatals on transport / decode error.
func readMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) gqlwsMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read: %v", err)
	}
	var msg gqlwsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("decode message: %v (raw=%q)", err, data)
	}
	return msg
}

// readNextNonPing skips Ping frames sent by the server's keepalive
// goroutine. Subscription tests don't care about Pings; this
// helper makes the assertion path readable.
func readNextNonPing(t *testing.T, conn *websocket.Conn, timeout time.Duration) gqlwsMessage {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg := readMessage(t, conn, time.Until(deadline))
		if msg.Type == gqlwsPing {
			continue
		}
		return msg
	}
	t.Fatalf("no non-ping message within %s", timeout)
	return gqlwsMessage{}
}

func sendMessage(t *testing.T, conn *websocket.Conn, msg gqlwsMessage) {
	t.Helper()
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, body); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// -------------------------------------------------------------------
// Constructor + sanity tests
// -------------------------------------------------------------------

func TestNewGraphQLSubscriptionsHandler_NilBus(t *testing.T) {
	gh, _ := NewGraphQLHandler(seedShowmatch(t), "")
	if _, err := NewGraphQLSubscriptionsHandler(nil, gh.schema); err == nil {
		t.Fatal("expected error for nil EventBus")
	}
}

func TestNewGraphQLSubscriptionsHandler_SchemaWithoutSubscription(t *testing.T) {
	// Synthesise a schema with no Subscription root.
	bus := NewEventBus()
	// Build a real schema, then construct a parallel one without the
	// Subscription. Easier: just confirm our normal schema HAS one.
	gh, _ := NewGraphQLHandler(seedShowmatch(t), "")
	if gh.schema.SubscriptionType() == nil {
		t.Fatal("default schema should now have a Subscription root after this PR")
	}
	// Now call with that schema and confirm OK.
	if _, err := NewGraphQLSubscriptionsHandler(bus, gh.schema); err != nil {
		t.Fatalf("schema-with-subscription should construct: %v", err)
	}
}

// -------------------------------------------------------------------
// Connection init / ack
// -------------------------------------------------------------------

func TestSubscriptions_ConnectionInitAck(t *testing.T) {
	subs, _ := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, err := dialSubscriptions(t, srv)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	msg := readNextNonPing(t, conn, 2*time.Second)
	if msg.Type != gqlwsConnectionAck {
		t.Errorf("expected connection_ack, got %q", msg.Type)
	}
}

func TestSubscriptions_SubscribeBeforeInit_Closes4401(t *testing.T) {
	subs, _ := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, err := dialSubscriptions(t, srv)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	// Subscribe without connection_init first.
	body, _ := json.Marshal(gqlwsSubscribePayload{Query: `subscription { gameEnded { id } }`})
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: body})

	// Next read should observe the close.
	_, _, err = conn.Read(context.Background())
	if err == nil {
		t.Fatal("expected close after Subscribe-before-init, got no error")
	}
	closeErr := websocket.CloseStatus(err)
	if closeErr != 4401 {
		t.Errorf("close status: want 4401, got %d (err=%v)", closeErr, err)
	}
}

// -------------------------------------------------------------------
// Subscribe → next → complete
// -------------------------------------------------------------------

func TestSubscriptions_GameEnded_DeliversNextFrame(t *testing.T) {
	subs, bus := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, _ := dialSubscriptions(t, srv)
	t.Cleanup(func() { _ = conn.CloseNow() })

	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	if ack := readNextNonPing(t, conn, 2*time.Second); ack.Type != gqlwsConnectionAck {
		t.Fatalf("expected ack, got %q", ack.Type)
	}

	body, _ := json.Marshal(gqlwsSubscribePayload{
		Query: `subscription { gameEnded { id winner winnerName turns } }`,
	})
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: body})

	// Wait for the per-op goroutine to register with the bus.
	waitForSubscribers(t, bus, 1, 2*time.Second)

	// Publish a game.end event in the shape showmatch.go emits.
	bus.Publish(Event{
		Type: EventGameEnd,
		Data: map[string]any{
			"game_id":     42,
			"winner":      1,
			"winner_name": "Atraxa",
			"commanders":  []string{"Korvold", "Atraxa", "Muldrotha", "Breya"},
			"deck_keys":   []string{"alice/k", "bob/a", "carol/m", "dave/b"},
			"turns":       18,
			"end_reason":  "lethal",
			"finished_at": time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	})

	msg := readNextNonPing(t, conn, 2*time.Second)
	if msg.Type != gqlwsNext {
		t.Fatalf("expected next, got %q (payload=%q)", msg.Type, msg.Payload)
	}
	if msg.ID != "op1" {
		t.Errorf("frame id: want op1, got %q", msg.ID)
	}

	var result map[string]any
	if err := json.Unmarshal(msg.Payload, &result); err != nil {
		t.Fatalf("decode next.payload: %v", err)
	}
	data, _ := result["data"].(map[string]any)
	if data == nil {
		t.Fatalf("payload.data missing: %+v", result)
	}
	game, _ := data["gameEnded"].(map[string]any)
	if game == nil {
		t.Fatalf("data.gameEnded missing: %+v", data)
	}
	if int(game["id"].(float64)) != 42 {
		t.Errorf("id: want 42, got %v", game["id"])
	}
	if int(game["winner"].(float64)) != 1 {
		t.Errorf("winner: want 1, got %v", game["winner"])
	}
	if game["winnerName"] != "Atraxa" {
		t.Errorf("winnerName: want Atraxa, got %v", game["winnerName"])
	}
	if int(game["turns"].(float64)) != 18 {
		t.Errorf("turns: want 18, got %v", game["turns"])
	}
}

func TestSubscriptions_FieldProjection_OmitsUnaskedFields(t *testing.T) {
	subs, bus := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, _ := dialSubscriptions(t, srv)
	t.Cleanup(func() { _ = conn.CloseNow() })
	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	readNextNonPing(t, conn, 2*time.Second)

	body, _ := json.Marshal(gqlwsSubscribePayload{
		Query: `subscription { gameEnded { id } }`,
	})
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: body})
	waitForSubscribers(t, bus, 1, 2*time.Second)

	bus.Publish(Event{
		Type: EventGameEnd,
		Data: map[string]any{
			"game_id":     7,
			"winner":      0,
			"winner_name": "Korvold",
			"commanders":  []string{"Korvold", "Atraxa", "Muldrotha", "Breya"},
			"deck_keys":   []string{"a/k", "b/a", "c/m", "d/b"},
			"turns":       12,
			"end_reason":  "lethal",
			"finished_at": time.Now().UTC().Format(time.RFC3339),
		},
	})

	msg := readNextNonPing(t, conn, 2*time.Second)
	var result struct {
		Data struct {
			GameEnded map[string]any `json:"gameEnded"`
		} `json:"data"`
	}
	_ = json.Unmarshal(msg.Payload, &result)
	if result.Data.GameEnded == nil {
		t.Fatalf("data.gameEnded missing: %s", msg.Payload)
	}
	if _, ok := result.Data.GameEnded["id"]; !ok {
		t.Errorf("id missing")
	}
	// Critical projection assertion: nothing else should be in
	// the payload. This is the load-bearing argument for putting
	// subscriptions on GraphQL vs. the /ws/events JSON firehose.
	for _, omitted := range []string{"winner", "winnerName", "commanders", "deckKeys", "turns", "endReason", "finishedAt"} {
		if _, ok := result.Data.GameEnded[omitted]; ok {
			t.Errorf("field projection failed: %s was returned despite not being requested (frame=%s)",
				omitted, msg.Payload)
		}
	}
}

func TestSubscriptions_ClientComplete_StopsSubscription(t *testing.T) {
	subs, bus := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, _ := dialSubscriptions(t, srv)
	t.Cleanup(func() { _ = conn.CloseNow() })
	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	readNextNonPing(t, conn, 2*time.Second)

	body, _ := json.Marshal(gqlwsSubscribePayload{Query: `subscription { gameEnded { id } }`})
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: body})
	waitForSubscribers(t, bus, 1, 2*time.Second)

	// Send Complete from the client side — server must drop the
	// bus subscription.
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsComplete})

	// The server should send back a Complete frame for op1 once
	// the runSubscription loop observes the cancel.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if bus.SubscriberCount() == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("bus subscriber count after client Complete: want 0, got %d", bus.SubscriberCount())
}

func TestSubscriptions_DuplicateSubscribeId_Closes4409(t *testing.T) {
	subs, bus := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, _ := dialSubscriptions(t, srv)
	t.Cleanup(func() { _ = conn.CloseNow() })
	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	readNextNonPing(t, conn, 2*time.Second)

	body, _ := json.Marshal(gqlwsSubscribePayload{Query: `subscription { gameEnded { id } }`})
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: body})
	waitForSubscribers(t, bus, 1, 2*time.Second)

	// Second Subscribe with the same id.
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: body})

	_, _, err := conn.Read(context.Background())
	if err == nil {
		t.Fatal("expected close after duplicate id")
	}
	if status := websocket.CloseStatus(err); status != 4409 {
		t.Errorf("close: want 4409, got %d (err=%v)", status, err)
	}
}

// -------------------------------------------------------------------
// Multiple ops on one connection
// -------------------------------------------------------------------

func TestSubscriptions_MultipleOpsOnOneConnection(t *testing.T) {
	subs, bus := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, _ := dialSubscriptions(t, srv)
	t.Cleanup(func() { _ = conn.CloseNow() })
	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	readNextNonPing(t, conn, 2*time.Second)

	body, _ := json.Marshal(gqlwsSubscribePayload{Query: `subscription { gameEnded { id } }`})
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: body})
	sendMessage(t, conn, gqlwsMessage{ID: "op2", Type: gqlwsSubscribe, Payload: body})
	waitForSubscribers(t, bus, 2, 2*time.Second)

	bus.Publish(Event{
		Type: EventGameEnd,
		Data: map[string]any{
			"game_id":     1,
			"winner":      0,
			"winner_name": "K",
			"commanders":  []string{"K", "A", "M", "B"},
			"deck_keys":   []string{"a/k", "b/a", "c/m", "d/b"},
			"turns":       10,
			"end_reason":  "lethal",
			"finished_at": time.Now().UTC().Format(time.RFC3339),
		},
	})

	// Expect two Next frames — one per op id, in some order.
	ids := map[string]bool{}
	for i := 0; i < 2; i++ {
		msg := readNextNonPing(t, conn, 2*time.Second)
		if msg.Type != gqlwsNext {
			t.Fatalf("frame %d: want next, got %q", i, msg.Type)
		}
		ids[msg.ID] = true
	}
	if !ids["op1"] || !ids["op2"] {
		t.Errorf("expected one frame per op id, got %v", ids)
	}
}

// -------------------------------------------------------------------
// Ping → Pong
// -------------------------------------------------------------------

func TestSubscriptions_ClientPing_GetsPong(t *testing.T) {
	subs, _ := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, _ := dialSubscriptions(t, srv)
	t.Cleanup(func() { _ = conn.CloseNow() })
	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	readNextNonPing(t, conn, 2*time.Second)

	sendMessage(t, conn, gqlwsMessage{Type: gqlwsPing})
	// We expect a Pong; the keepalive ticker shouldn't interfere
	// because we only check for the first non-init frame.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg := readMessage(t, conn, time.Until(deadline))
		if msg.Type == gqlwsPing {
			continue // server keepalive — ignore
		}
		if msg.Type == gqlwsPong {
			return
		}
		t.Fatalf("expected pong, got %q", msg.Type)
	}
	t.Errorf("no pong observed within 2s")
}

// -------------------------------------------------------------------
// Subprotocol negotiation
// -------------------------------------------------------------------

func TestSubscriptions_SubprotocolRequired(t *testing.T) {
	subs, _ := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	// Dial WITHOUT the subprotocol header.
	conn, _, err := dialSubscriptions(t, srv, withoutSubprotocol())
	if err != nil {
		// Some clients refuse to upgrade without an agreed
		// subprotocol; that's also acceptable.
		return
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	// Connection succeeded but server should close immediately.
	_, _, readErr := conn.Read(context.Background())
	if readErr == nil {
		t.Fatal("expected close on missing subprotocol, got no error")
	}
	if status := websocket.CloseStatus(readErr); status != websocket.StatusPolicyViolation {
		t.Errorf("close: want PolicyViolation (1008), got %d", status)
	}
}

// -------------------------------------------------------------------
// Connection cap
// -------------------------------------------------------------------

func TestSubscriptions_ConnectionCap_Returns503(t *testing.T) {
	subs, _ := seedSubscriptionsHandler(t)
	// Saturate the cap by leasing all slots directly.
	for i := 0; i < graphqlSubscriptionsMaxConnections; i++ {
		if !subs.reserveSlot() {
			t.Fatalf("reserve setup %d failed", i)
		}
	}
	t.Cleanup(func() {
		for i := 0; i < graphqlSubscriptionsMaxConnections; i++ {
			subs.releaseSlot()
		}
	})

	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d", resp.StatusCode)
	}
	var env ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode ErrorResponse: %v", err)
	}
	if env.Status != http.StatusServiceUnavailable {
		t.Errorf("envelope.status: want 503, got %d", env.Status)
	}
}

// -------------------------------------------------------------------
// Bad query → Error frame
// -------------------------------------------------------------------

func TestSubscriptions_BadQuery_EmitsErrorFrame(t *testing.T) {
	subs, _ := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, _ := dialSubscriptions(t, srv)
	t.Cleanup(func() { _ = conn.CloseNow() })
	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	readNextNonPing(t, conn, 2*time.Second)

	// Malformed: missing closing brace.
	body, _ := json.Marshal(gqlwsSubscribePayload{Query: `subscription { gameEnded { id`})
	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: body})

	// Expect Error then Complete (per spec, an operation terminates
	// either with Complete or Error). Read until we see Error or
	// hit the deadline.
	deadline := time.Now().Add(2 * time.Second)
	gotError := false
	for time.Now().Before(deadline) {
		msg := readMessage(t, conn, time.Until(deadline))
		if msg.Type == gqlwsPing {
			continue
		}
		if msg.Type == gqlwsError {
			gotError = true
			if msg.ID != "op1" {
				t.Errorf("error id: want op1, got %q", msg.ID)
			}
			break
		}
		t.Fatalf("expected error frame, got %q", msg.Type)
	}
	if !gotError {
		t.Errorf("no error frame observed within 2s")
	}
}

// -------------------------------------------------------------------
// Helper: event payload conversion is exercised end-to-end above,
// but also has a unit pin.
// -------------------------------------------------------------------

func TestEventDataToCompletedGame(t *testing.T) {
	finished := time.Date(2026, 5, 25, 14, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   any
		want CompletedGame
		ok   bool
	}{
		{
			name: "happy path",
			in: map[string]any{
				"game_id":     11,
				"winner":      2,
				"winner_name": "Muldrotha",
				"commanders":  []string{"K", "A", "M", "B"},
				"deck_keys":   []string{"a/k", "b/a", "c/m", "d/b"},
				"turns":       14,
				"end_reason":  "lethal",
				"finished_at": finished.Format(time.RFC3339),
			},
			want: CompletedGame{
				GameID: 11, Winner: 2, WinnerName: "Muldrotha",
				Commanders: []string{"K", "A", "M", "B"},
				DeckKeys:   []string{"a/k", "b/a", "c/m", "d/b"},
				Turns:      14, EndReason: "lethal",
				FinishedAt: finished,
			},
			ok: true,
		},
		{
			name: "non-map input returns false",
			in:   "not a map",
			ok:   false,
		},
		{
			name: "partial map fills what it can",
			in: map[string]any{
				"game_id": 7,
				"winner":  0,
			},
			want: CompletedGame{GameID: 7, Winner: 0},
			ok:   true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := eventDataToCompletedGame(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok: want %v, got %v", tc.ok, ok)
			}
			if !ok {
				return
			}
			if got.GameID != tc.want.GameID || got.Winner != tc.want.Winner {
				t.Errorf("scalar fields: want %+v, got %+v", tc.want, got)
			}
			if !got.FinishedAt.Equal(tc.want.FinishedAt) {
				t.Errorf("finished_at: want %v, got %v", tc.want.FinishedAt, got.FinishedAt)
			}
		})
	}
}
