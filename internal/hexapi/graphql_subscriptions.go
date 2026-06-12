package hexapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
)

// graphql_subscriptions.go — WebSocket transport for GraphQL
// subscriptions. Companion to the /ws/events stream shipped in #338
// (which carries discrete typed JSON events without a schema). This
// surface carries GraphQL subscription documents over the standard
// `graphql-transport-ws` subprotocol used by Apollo Client + urql,
// so clients that already speak GraphQL queries against /graphql
// can re-use the same toolchain for real-time updates.
//
// Wire protocol summary (the load-bearing subset of
// https://github.com/enisdenjo/graphql-ws/blob/master/PROTOCOL.md):
//
//   Client → Server:  ConnectionInit  {"type":"connection_init"}
//   Server → Client:  ConnectionAck   {"type":"connection_ack"}
//   Client → Server:  Subscribe       {"id":"<op>","type":"subscribe","payload":{"query":...,"variables":...}}
//   Server → Client:  Next            {"id":"<op>","type":"next","payload":{...result...}}        — repeated
//   Server → Client:  Error           {"id":"<op>","type":"error","payload":[{...errors...}]}     — once, terminal
//   Server → Client:  Complete        {"id":"<op>","type":"complete"}                              — once, terminal
//   Client → Server:  Complete        {"id":"<op>","type":"complete"}                              — to unsubscribe
//   Either direction: Ping / Pong     {"type":"ping"} / {"type":"pong"}                            — keepalive
//
// Subscribe before ConnectionInit is a 4401 close per spec; a second
// Subscribe with the same id is a 4409 close ("subscriber for <id>
// already exists"). Both are terminal — clients must reconnect.
//
// The only subscription field today is `gameEnded`, which streams
// completed games from the EventBus. The schema's Subscription root
// (in graphql_schema.go) reads the per-request `chan interface{}`
// from the context this handler populates; each value pulled from
// the channel passes through the regular Game-type field resolvers,
// so field projection works exactly like a query.

// graphqlTransportWSSubprotocol is the WebSocket subprotocol string
// clients must request. The handler rejects connections that don't
// negotiate it — matches Apollo's behaviour.
const graphqlTransportWSSubprotocol = "graphql-transport-ws"

// graphqlSubscriptionsMaxConnections caps live subscription
// sessions. Same budget as /ws/events (#338) — these consumers are
// the same class of caller (dashboards, bots).
const graphqlSubscriptionsMaxConnections = 100

// graphqlSubscriptionsKeepalive controls the server-side ping
// interval. Matches /ws/events.
const graphqlSubscriptionsKeepalive = 30 * time.Second

// graphqlSubscriptionsWriteTimeout is the per-message write budget.
const graphqlSubscriptionsWriteTimeout = 5 * time.Second

// graphqlSubscriptionsReadLimit caps incoming control frames.
// A subscription payload includes the full GraphQL query document
// + variables, so 32 KiB is a more generous ceiling than
// /ws/events.
const graphqlSubscriptionsReadLimit = 32 * 1024

// GraphQLSubscriptionsHandler bridges the engine's EventBus to a
// graphql-transport-ws WebSocket endpoint. Construct via
// NewGraphQLSubscriptionsHandler.
type GraphQLSubscriptionsHandler struct {
	bus    *EventBus
	schema graphql.Schema
	logf   func(string, ...any)

	connMu      sync.Mutex
	connections int
}

// NewGraphQLSubscriptionsHandler builds the handler. The schema MUST
// have a Subscription root with `gameEnded` defined and wired to
// pull its source channel from the subscriptionSourceKey context
// value — the schema returned by buildGraphQLSchema(..) does this.
// Returns an error if bus is nil or the schema lacks a Subscription
// root (a sanity check to catch wiring bugs at startup).
func NewGraphQLSubscriptionsHandler(bus *EventBus, schema graphql.Schema) (*GraphQLSubscriptionsHandler, error) {
	if bus == nil {
		return nil, errors.New("graphql subscriptions: nil EventBus")
	}
	if schema.SubscriptionType() == nil {
		return nil, errors.New("graphql subscriptions: schema has no Subscription root — wire one via SchemaConfig.Subscription")
	}
	return &GraphQLSubscriptionsHandler{
		bus:    bus,
		schema: schema,
		logf:   log.Printf,
	}, nil
}

// SetLogf overrides the log sink. Test-only.
func (h *GraphQLSubscriptionsHandler) SetLogf(f func(string, ...any)) {
	if f != nil {
		h.logf = f
	}
}

func (h *GraphQLSubscriptionsHandler) reserveSlot() bool {
	h.connMu.Lock()
	defer h.connMu.Unlock()
	if h.connections >= graphqlSubscriptionsMaxConnections {
		return false
	}
	h.connections++
	return true
}

func (h *GraphQLSubscriptionsHandler) releaseSlot() {
	h.connMu.Lock()
	defer h.connMu.Unlock()
	if h.connections > 0 {
		h.connections--
	}
}

// -------------------------------------------------------------------
// Wire message shapes
// -------------------------------------------------------------------

const (
	gqlwsConnectionInit = "connection_init"
	gqlwsConnectionAck  = "connection_ack"
	gqlwsSubscribe      = "subscribe"
	gqlwsNext           = "next"
	gqlwsError          = "error"
	gqlwsComplete       = "complete"
	gqlwsPing           = "ping"
	gqlwsPong           = "pong"
)

// gqlwsMessage is the umbrella envelope. payload is raw JSON so the
// reader doesn't have to know up-front whether it's the
// connection_init Map or the subscribe SubscribePayload.
type gqlwsMessage struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// gqlwsSubscribePayload is the body of a Subscribe message — a full
// GraphQL request document.
type gqlwsSubscribePayload struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
	OperationName string         `json:"operationName,omitempty"`
}

// -------------------------------------------------------------------
// Connection lifecycle
// -------------------------------------------------------------------

func (h *GraphQLSubscriptionsHandler) serve(w http.ResponseWriter, r *http.Request) {
	if !h.reserveSlot() {
		writeError(w, http.StatusServiceUnavailable, "too many subscription clients")
		return
	}
	defer h.releaseSlot()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:       []string{graphqlTransportWSSubprotocol},
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.logf("graphql subscriptions: upgrade error: %v", err)
		return
	}
	defer conn.CloseNow()

	if conn.Subprotocol() != graphqlTransportWSSubprotocol {
		// Client didn't negotiate our subprotocol; close per spec.
		_ = conn.Close(websocket.StatusPolicyViolation,
			"subprotocol "+graphqlTransportWSSubprotocol+" required")
		return
	}
	conn.SetReadLimit(graphqlSubscriptionsReadLimit)

	sess := newSubscriptionSession(h, conn)
	defer sess.shutdown()
	sess.run(r.Context())
}

// subscriptionSession is the per-connection state — initialized
// flag, active subscriptions, lifecycle goroutines.
type subscriptionSession struct {
	h    *GraphQLSubscriptionsHandler
	conn *websocket.Conn

	mu          sync.Mutex
	initialized bool
	subs        map[string]*subscriptionOp
	writeMu     sync.Mutex
}

type subscriptionOp struct {
	id     string
	cancel context.CancelFunc
	donec  chan struct{}
}

func newSubscriptionSession(h *GraphQLSubscriptionsHandler, conn *websocket.Conn) *subscriptionSession {
	return &subscriptionSession{
		h:    h,
		conn: conn,
		subs: map[string]*subscriptionOp{},
	}
}

func (s *subscriptionSession) shutdown() {
	s.mu.Lock()
	ops := make([]*subscriptionOp, 0, len(s.subs))
	for _, op := range s.subs {
		ops = append(ops, op)
	}
	s.subs = nil
	s.mu.Unlock()
	for _, op := range ops {
		op.cancel()
		<-op.donec
	}
}

// run is the connection's read loop. Spawns one subscription
// goroutine per Subscribe message; the connection itself stays
// owned by run. Returns when the client disconnects, the context
// is cancelled, or a protocol violation closes the session.
func (s *subscriptionSession) run(ctx context.Context) {
	// Keepalive ping goroutine; closed when run() returns.
	keepCtx, keepCancel := context.WithCancel(ctx)
	defer keepCancel()
	go s.keepalive(keepCtx)

	for {
		_, data, err := s.conn.Read(ctx)
		if err != nil {
			return
		}
		var msg gqlwsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			s.h.logf("graphql subscriptions: bad frame: %v", err)
			_ = s.conn.Close(websocket.StatusUnsupportedData, "invalid JSON")
			return
		}

		switch msg.Type {
		case gqlwsConnectionInit:
			s.mu.Lock()
			if s.initialized {
				s.mu.Unlock()
				_ = s.conn.Close(4429, "too many initialisation requests")
				return
			}
			s.initialized = true
			s.mu.Unlock()
			s.writeMessage(ctx, gqlwsMessage{Type: gqlwsConnectionAck})

		case gqlwsPing:
			s.writeMessage(ctx, gqlwsMessage{Type: gqlwsPong})

		case gqlwsPong:
			// noop — clients that pong after our keepalive are
			// just confirming liveness.

		case gqlwsSubscribe:
			s.handleSubscribe(ctx, msg)

		case gqlwsComplete:
			s.cancelSubscription(msg.ID)

		default:
			// Unknown verbs are forwards-compat noise; ignore.
		}
	}
}

// keepalive sends a Ping every graphqlSubscriptionsKeepalive
// interval. Pongs are observed in the read loop and dropped.
func (s *subscriptionSession) keepalive(ctx context.Context) {
	tick := time.NewTicker(graphqlSubscriptionsKeepalive)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.writeMessage(ctx, gqlwsMessage{Type: gqlwsPing})
		}
	}
}

func (s *subscriptionSession) handleSubscribe(ctx context.Context, msg gqlwsMessage) {
	s.mu.Lock()
	if !s.initialized {
		s.mu.Unlock()
		_ = s.conn.Close(4401, "Unauthorized — Subscribe before ConnectionInit")
		return
	}
	if msg.ID == "" {
		s.mu.Unlock()
		_ = s.conn.Close(websocket.StatusUnsupportedData, "Subscribe.id required")
		return
	}
	if _, exists := s.subs[msg.ID]; exists {
		s.mu.Unlock()
		_ = s.conn.Close(4409, "Subscriber for "+msg.ID+" already exists")
		return
	}

	opCtx, cancel := context.WithCancel(ctx)
	op := &subscriptionOp{id: msg.ID, cancel: cancel, donec: make(chan struct{})}
	s.subs[msg.ID] = op
	s.mu.Unlock()

	var payload gqlwsSubscribePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.emitError(ctx, msg.ID, errCodeBadRequest, "invalid Subscribe payload: "+err.Error())
		s.removeSub(msg.ID)
		cancel()
		close(op.donec)
		return
	}

	go func() {
		defer close(op.donec)
		s.runSubscription(opCtx, msg.ID, payload)
		s.removeSub(msg.ID)
	}()
}

func (s *subscriptionSession) removeSub(id string) {
	s.mu.Lock()
	delete(s.subs, id)
	s.mu.Unlock()
}

func (s *subscriptionSession) cancelSubscription(id string) {
	s.mu.Lock()
	op, ok := s.subs[id]
	s.mu.Unlock()
	if !ok {
		return
	}
	op.cancel()
	// Don't wait on donec here — the client just told us to stop;
	// the per-op goroutine cleans itself up.
}

// runSubscription bridges one EventBus subscriber to one
// graphql.Subscribe execution. The bus subscriber feeds a
// `chan interface{}` of CompletedGame values; graphql.Subscribe
// reads that channel and produces *Result values for each event,
// which we serialise as `next` frames.
func (s *subscriptionSession) runSubscription(ctx context.Context, opID string, payload gqlwsSubscribePayload) {
	busSub := s.h.bus.Subscribe(64)
	defer s.h.bus.Unsubscribe(busSub)

	// graphql-go's Subscribe machinery expects `chan interface{}`.
	// Adapt the bus channel and translate the event payload into a
	// CompletedGame the Game-type field resolvers can read.
	gqlCh := make(chan interface{}, 64)
	go func() {
		defer close(gqlCh)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-busSub.Ch:
				if !ok {
					return
				}
				if ev.Type != EventGameEnd {
					continue
				}
				game, ok := eventDataToCompletedGame(ev.Data)
				if !ok {
					continue
				}
				select {
				case gqlCh <- game:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	subCtx := context.WithValue(ctx, subscriptionSourceKey, gqlCh)
	results := graphql.Subscribe(graphql.Params{
		Schema:         s.h.schema,
		RequestString:  payload.Query,
		VariableValues: payload.Variables,
		OperationName:  payload.OperationName,
		Context:        subCtx,
	})

	for result := range results {
		if result == nil {
			continue
		}
		if len(result.Errors) > 0 && result.Data == nil {
			// Pre-execution validation / parse failure — single
			// terminal Error frame, then end the op. Re-stamp each
			// graphql FormattedError with extensions.code so the
			// wire shape matches the emitError-built envelope and
			// downstream consumers never see a code-less error.
			body, _ := json.Marshal(stampErrorCodes(result.Errors, errCodeGraphQLValidation))
			s.writeMessage(ctx, gqlwsMessage{ID: opID, Type: gqlwsError, Payload: body})
			return
		}
		body, err := json.Marshal(result)
		if err != nil {
			s.h.logf("graphql subscriptions %s: marshal: %v", opID, err)
			s.emitError(ctx, opID, errCodeInternal, "internal marshal error")
			return
		}
		if err := s.writeMessage(ctx, gqlwsMessage{ID: opID, Type: gqlwsNext, Payload: body}); err != nil {
			return
		}
	}
	// Source channel closed (client cancel, context done, or the
	// engine shutting down). Send Complete so the client knows
	// the op is terminated.
	_ = s.writeMessage(ctx, gqlwsMessage{ID: opID, Type: gqlwsComplete})
}

// Structured error codes carried in the `extensions.code` field of
// every emitted Error-frame payload entry. Apollo / urql clients
// surface these via `error.extensions.code`; consumers should branch
// on these constants, not on the free-text message (which is for logs
// and humans, not protocol). Mirrors the Apollo-Server convention.
const (
	errCodeBadRequest        = "BAD_REQUEST"
	errCodeInternal          = "INTERNAL"
	errCodeGraphQLValidation = "GRAPHQL_VALIDATION"
)

// emitError sends a single terminal Error frame for opID with one
// formatted-error entry. Wraps the message into a gqlerrors.FormattedError
// so the wire shape is byte-identical to the graphql-go validation-error
// path — every Error frame's payload is `[{message, locations?, path?,
// extensions: {code}}]` regardless of whether the failure originated
// inside the GraphQL executor or in our own wrapping code.
func (s *subscriptionSession) emitError(ctx context.Context, opID, code, message string) {
	fe := gqlerrors.FormattedError{
		Message:    message,
		Extensions: map[string]interface{}{"code": code},
	}
	body, _ := json.Marshal([]gqlerrors.FormattedError{fe})
	_ = s.writeMessage(ctx, gqlwsMessage{ID: opID, Type: gqlwsError, Payload: body})
}

// stampErrorCodes ensures every FormattedError in errs carries an
// extensions.code field, defaulting to fallback when missing. graphql-go
// emits validation/parse errors with no extensions, so unified
// downstream handling needs a stamping pass before serialisation.
// Existing codes are preserved — a resolver-supplied "code" in
// extensions wins over the fallback.
func stampErrorCodes(errs []gqlerrors.FormattedError, fallback string) []gqlerrors.FormattedError {
	out := make([]gqlerrors.FormattedError, len(errs))
	for i, e := range errs {
		ext := e.Extensions
		if ext == nil {
			ext = map[string]interface{}{}
		} else {
			// Copy so we don't mutate the caller's map.
			cp := make(map[string]interface{}, len(ext)+1)
			for k, v := range ext {
				cp[k] = v
			}
			ext = cp
		}
		if _, ok := ext["code"]; !ok {
			ext["code"] = fallback
		}
		e.Extensions = ext
		out[i] = e
	}
	return out
}

// writeMessage serialises + sends one envelope. Mutex-guards
// concurrent writes (subscription goroutines and the keepalive
// ticker can both call this).
func (s *subscriptionSession) writeMessage(ctx context.Context, msg gqlwsMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	wctx, cancel := context.WithTimeout(ctx, graphqlSubscriptionsWriteTimeout)
	defer cancel()
	return s.conn.Write(wctx, websocket.MessageText, body)
}

// -------------------------------------------------------------------
// EventBus payload → CompletedGame conversion
// -------------------------------------------------------------------

// eventDataToCompletedGame undoes the map flattening that
// showmatch.go's game-end tail applies before publishing to the
// EventBus. Returns false when the payload doesn't match the
// documented shape (so the subscription handler can skip it
// rather than emit a broken Next frame).
//
// Kept here (not in event_bus.go) so the conversion lives next to
// the only consumer that needs the typed struct. The /ws/events
// transport works with the raw map and doesn't pay for this.
func eventDataToCompletedGame(data any) (CompletedGame, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return CompletedGame{}, false
	}
	var g CompletedGame
	if v, ok := m["game_id"].(int); ok {
		g.GameID = v
	}
	if v, ok := m["winner"].(int); ok {
		g.Winner = v
	}
	if v, ok := m["winner_name"].(string); ok {
		g.WinnerName = v
	}
	if v, ok := m["commanders"].([]string); ok {
		g.Commanders = v
	}
	if v, ok := m["deck_keys"].([]string); ok {
		g.DeckKeys = v
	}
	if v, ok := m["turns"].(int); ok {
		g.Turns = v
	}
	if v, ok := m["end_reason"].(string); ok {
		g.EndReason = v
	}
	if v, ok := m["finished_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			g.FinishedAt = t
		}
	}
	return g, true
}

// Compile-time pin: strconv must stay imported (used by future
// failure-code messages); silence "imported and not used" if the
// 4401 / 4429 / 4409 close codes ever lose their callers.
var _ = strconv.Itoa
