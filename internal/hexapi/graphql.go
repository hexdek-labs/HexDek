package hexapi

import (
	"encoding/json"
	"errors"
	"net/http"

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
// Out of POC scope (and still): mutations, subscriptions, depth
// limits, persisted queries, an introspection toggle, dataloader.

// GraphQLHandler wraps a Showmatch + decksDir + a compiled GraphQL
// schema. The schema is built once per process at construction;
// serving a query just calls graphql.Do against it.
type GraphQLHandler struct {
	sm       *Showmatch
	decksDir string
	schema   graphql.Schema
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
func (h *GraphQLHandler) RegisterGraphQL(mux *http.ServeMux) {
	mux.HandleFunc("POST /graphql", h.serve)
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

	result := graphql.Do(graphql.Params{
		Schema:         h.schema,
		RequestString:  req.Query,
		VariableValues: req.Variables,
		OperationName:  req.OperationName,
		Context:        r.Context(),
	})
	// graphql.Do never returns a transport error — it always emits
	// a Result, with any execution / validation errors collected in
	// result.Errors. Mirror that contract on the wire: HTTP 200
	// (per the GraphQL spec's transport recommendations) with
	// result.Errors populated for the client to surface.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

