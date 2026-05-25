package hexapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/graphql-go/graphql"
)

// graphql.go — minimal viable proof-of-concept of a GraphQL endpoint
// over the game data already exposed by the REST surface
// (/api/games + /api/games/{id}). The intent is to evaluate whether
// GraphQL's selective-field projection + cross-resource query shape
// would buy enough in the React SPA to justify maintaining a schema
// alongside the REST handlers.
//
// What this POC ships:
//
//   - One object type, Game, mirroring CompletedGame's stable fields
//     (winner / commanders / deck_keys / turns / end_reason /
//     finished_at). Heavy nested fields (timeline, final_seats) are
//     intentionally absent — the POC's premise is that callers ask
//     for the cheap projection most of the time.
//
//   - Two root queries:
//       game(id: Int!)         → one game by id, or null
//       games(limit: Int = 20) → recent games, newest first
//
//   - One HTTP route: POST /graphql, body `{query, variables}`.
//     Also handles GET /graphql?query=... for ad-hoc poking from
//     a browser, with the same JSON response shape.
//
// What this POC does NOT ship: mutations, subscriptions, depth
// limits, persisted queries, an introspection toggle, dataloader,
// schema documentation. Those are reasonable next steps once the
// team decides GraphQL is worth carrying.
//
// The POC sources its data from sm.GetGame / sm.GetGameHistory so
// there's no separate data layer to keep in sync with the REST
// handlers — schema + REST stay backed by the same in-memory store.

// GraphQLHandler wraps a Showmatch + a compiled GraphQL schema. The
// schema is built once per process at construction; serving a query
// just calls graphql.Do against it.
type GraphQLHandler struct {
	sm     *Showmatch
	schema graphql.Schema
}

// NewGraphQLHandler compiles the POC schema against sm. Returns an
// error if the schema fails to validate at startup — callers should
// fail-fast on that rather than silently route /graphql to a 500.
func NewGraphQLHandler(sm *Showmatch) (*GraphQLHandler, error) {
	if sm == nil {
		return nil, errors.New("graphql: nil Showmatch")
	}
	h := &GraphQLHandler{sm: sm}
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

// -------------------------------------------------------------------
// Schema
// -------------------------------------------------------------------

// buildGraphQLSchema wires the Game type + Query root. Resolvers
// pull from h.sm — no shadow data layer. Closure capture on h keeps
// the resolvers reachable from tests without exporting the schema.
func buildGraphQLSchema(h *GraphQLHandler) (graphql.Schema, error) {
	// Game object: mirrors the safe subset of CompletedGame.
	// Avoid Timeline + FinalSeats here — those are big nested
	// structures and the whole point of the POC is selective
	// projection of the *cheap* fields.
	gameType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "Game",
		Description: "A completed Commander game.",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.Int),
				Description: "Sequential game id assigned by the engine.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(CompletedGame).GameID, nil
				},
			},
			"winner": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.Int),
				Description: "Seat number (0..3) of the winning player.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(CompletedGame).Winner, nil
				},
			},
			"winnerName": &graphql.Field{
				Type:        graphql.String,
				Description: "Commander name of the winner.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(CompletedGame).WinnerName, nil
				},
			},
			"commanders": &graphql.Field{
				Type:        graphql.NewList(graphql.String),
				Description: "Commander names per seat, index = seat.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(CompletedGame).Commanders, nil
				},
			},
			"deckKeys": &graphql.Field{
				Type:        graphql.NewList(graphql.String),
				Description: "Deck keys (owner/id) per seat.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(CompletedGame).DeckKeys, nil
				},
			},
			"turns": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.Int),
				Description: "Turn count at game end.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(CompletedGame).Turns, nil
				},
			},
			"endReason": &graphql.Field{
				Type:        graphql.String,
				Description: "How the game ended (lethal, decking, concede, ...).",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(CompletedGame).EndReason, nil
				},
			},
			"finishedAt": &graphql.Field{
				Type:        graphql.String,
				Description: "RFC3339 timestamp of game completion.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(CompletedGame).FinishedAt.UTC().Format("2006-01-02T15:04:05Z"), nil
				},
			},
		},
	})

	queryRoot := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"game": &graphql.Field{
				Type:        gameType,
				Description: "Look up one game by id; null when missing.",
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type:        graphql.NewNonNull(graphql.Int),
						Description: "Engine-assigned game id.",
					},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					id, _ := p.Args["id"].(int)
					g := h.sm.GetGame(id)
					if g == nil {
						return nil, nil
					}
					return *g, nil
				},
			},
			"games": &graphql.Field{
				Type:        graphql.NewList(gameType),
				Description: "Recent games, newest first. Defaults to 20; caps at the engine's gameHistory window.",
				Args: graphql.FieldConfigArgument{
					"limit": &graphql.ArgumentConfig{
						Type:         graphql.Int,
						DefaultValue: 20,
						Description:  "Max rows to return.",
					},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					limit, _ := p.Args["limit"].(int)
					if limit < 0 {
						return nil, errors.New("limit must be non-negative")
					}
					return h.sm.GetGameHistory(limit), nil
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{Query: queryRoot})
}
