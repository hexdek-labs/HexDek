package hexapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
)

// graphql.go — HTTP transport for the /graphql endpoint. The schema
// itself (types + resolvers) lives in graphql_schema.go.
//
// History: started life as the inline POC from PR #321 with one type
// (Game), two queries, and the schema baked into a single closure
// alongside the HTTP serve glue. r60 split the schema out into its
// own file and grew it into a proper graph:
//
//   - Game (unchanged from POC) — completed-game snapshot
//   - Deck (new) — the safe subset of DeckSummary (id / owner /
//     name / commander / bracket / color / archetype / cardCount /
//     importedAt) sourced from disk via the same helpers that back
//     the REST list/get handlers, no shadow data layer
//   - Game.decks: [Deck] cross-reference (new) — each game's
//     deckKeys resolved to populated Deck objects in one round-trip,
//     the canonical "GraphQL pays for itself" shape
//
// Root queries:
//   game(id: Int!)                              → one game or null
//   games(limit: Int = 20)                      → recent games
//   deck(owner: String!, id: String!)           → one deck or null
//   decks(owner: String, limit: Int = 50)       → decks, filterable
//
// Mutations (r60):
//   importDeck(input: DeckImportInput!): Deck   — write a new deck file
//   deleteDeck(owner: String!, id: String!): Boolean
//                                               — caller-owner-checked
//
// Mutations check the caller's identity by reading X-HexDek-Owner from
// the request headers and stashing it in the resolver context. Same
// header-based ownership shape the REST handlers use (checkOwnership);
// mirrored here so the GraphQL surface doesn't become the soft spot.
//
// Out of POC scope (and still): subscriptions, depth limits, persisted
// queries, an introspection toggle, dataloader.

// GraphQLHandler wraps a Showmatch + decksDir + a compiled GraphQL
// schema. The schema is built once per process at construction;
// serving a query just calls graphql.Do against it.
//
// CSRFStore + Limiter are optional protection layers. Both are
// nil-safe — leaving them unset is backwards-compatible with the
// pre-r60 POC wiring. cmd/hexdek-server should stamp the same
// CSRFStore it uses on Handler.CSRFStore and a per-IP RateLimiter
// when constructing the handler for production. Why both:
//   - CSRFStore closes the mutation-bypass gap. Pre-r60 POST /graphql
//     was the only mutation surface NOT wrapped in RequireCSRF — an
//     XSS-derived request could issue `mutation { deleteDeck(...) }`
//     and the CSRF gate that fires on REST DELETE wouldn't see it.
//   - Limiter defends against GraphQL's query-complexity multiplier:
//     one POST can fan out to N resolver calls (game{decks{...}}
//     across 100 games = 100 deck lookups in one request), so a per-
//     request limit at the REST layer underprices the actual work.
type GraphQLHandler struct {
	sm       *Showmatch
	decksDir string
	schema   graphql.Schema

	CSRFStore *CSRFStore
	Limiter   *RateLimiter
}

// NewGraphQLHandler compiles the schema against sm and decksDir.
// Returns an error if the schema fails to validate at startup —
// callers should fail-fast on that rather than silently route
// /graphql to a 500. decksDir may be empty; the deck-resolving
// queries simply return empty / null when so (preserves the "schema
// degrades gracefully on partial wiring" property the POC had).
func NewGraphQLHandler(sm *Showmatch, decksDir string) (*GraphQLHandler, error) {
	if sm == nil {
		return nil, errors.New("graphql: nil Showmatch")
	}
	h := &GraphQLHandler{sm: sm, decksDir: decksDir}
	schema, err := buildGraphQLSchema(h)
	if err != nil {
		return nil, err
	}
	h.schema = schema
	return h, nil
}

// RegisterGraphQL mounts the /graphql route on mux. Kept off the
// default Handler.Register chain so callers opt into the surface
// explicitly (GraphQL is currently a POC, not a production
// dependency).
//
// POST is wrapped in RequireCSRF — mutations on /graphql are
// indistinguishable from queries at the URL layer, so the gate has to
// cover both to defend importDeck / deleteDeck. RequireCSRF is nil-
// safe (pass-through when h.CSRFStore is unset), bypasses OPTIONS for
// CORS preflight, and emits the unified envelope on 403. GET is left
// ungated — only queries can travel over GET per the GraphQL HTTP
// recommendation, and read queries are not the CSRF threat model.
func (h *GraphQLHandler) RegisterGraphQL(mux *http.ServeMux) {
	mux.HandleFunc("POST /graphql", RequireCSRF(h.CSRFStore, h.serve))
	mux.HandleFunc("GET /graphql", h.serve)
}

// graphqlRequest is the JSON body shape POST /graphql accepts.
// Variables are passed through verbatim to graphql.Do.
type graphqlRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]any         `json:"variables"`
	OperationName string                 `json:"operationName"`
}

func (h *GraphQLHandler) serve(w http.ResponseWriter, r *http.Request) {
	// Per-IP rate-limit gate FIRST. A single GraphQL request can fan
	// out to N resolver calls (a deep query with cross-references) —
	// limiting at the request boundary undercharges that work but is
	// still strictly better than no limit. The Showmatch deck-list /
	// game-list resolvers each hit disk + the snapshot cache, so an
	// unthrottled `query { games(limit: 100) { decks { ... } } }`
	// blast can pin those caches. Limiter is nil-safe.
	if enforceRateLimit(h.Limiter, w, r, "graphql") {
		return
	}
	var req graphqlRequest
	switch r.Method {
	case http.MethodGet:
		// Browser-friendly ad-hoc query form: ?query=...&variables=...
		req.Query = r.URL.Query().Get("query")
		if raw := r.URL.Query().Get("variables"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &req.Variables); err != nil {
				writeError(w, http.StatusBadRequest, "variables: "+err.Error())
				return
			}
		}
		req.OperationName = r.URL.Query().Get("operationName")
	default:
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query required")
		return
	}

	// Stash the caller's owner identity in the resolver context so
	// mutation resolvers can do an ownership check without reaching
	// back to the http.Request (which graphql-go doesn't pass through
	// by default). Mirrors REST's checkOwnership(r, owner).
	ctx := r.Context()
	if owner := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner"))); owner != "" {
		ctx = withCallerOwner(ctx, owner)
	}
	result := graphql.Do(graphql.Params{
		Schema:         h.schema,
		RequestString:  req.Query,
		VariableValues: req.Variables,
		OperationName:  req.OperationName,
		Context:        ctx,
	})
	// graphql.Do never returns a transport error — it always emits
	// a Result, with any execution / validation errors collected in
	// result.Errors. Mirror that contract on the wire: HTTP 200
	// (per the GraphQL spec's transport recommendations) with
	// result.Errors populated for the client to surface.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// graphqlCtxKey is the private context-key type for resolver-side
// auth lookups. Unexported so callers outside this file can't collide
// with the key namespace.
type graphqlCtxKey string

const ctxKeyCallerOwner graphqlCtxKey = "caller_owner"

// withCallerOwner returns a derived context carrying the lowercased
// caller-owner slug pulled from the X-HexDek-Owner request header.
// Used by mutation resolvers to gate owner-targeted operations.
func withCallerOwner(ctx context.Context, owner string) context.Context {
	return context.WithValue(ctx, ctxKeyCallerOwner, owner)
}

// callerOwner returns the slug previously stashed via withCallerOwner,
// or "" if absent. Resolvers use the empty return as "unauthenticated"
// — same shape as REST's checkOwnership treating a blank header.
func callerOwner(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyCallerOwner).(string)
	return v
}

