package hexapi

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
)

// graphql_schema.go — type definitions + schema construction for the
// /graphql surface. The HTTP transport lives in graphql.go; this file
// is intentionally pure schema so the shape of the graph is easy to
// audit in one place.
//
// Schema overview (one source of truth — keep in sync with the
// docstring on GraphQLHandler in graphql.go):
//
//   type Game {
//     id: Int!
//     winner: Int!
//     winnerName: String
//     commanders: [String]
//     deckKeys:   [String]
//     decks:      [Deck]        # cross-reference, resolved lazily
//     turns: Int!
//     endReason: String
//     finishedAt: String
//   }
//
//   type Deck {
//     id:         String!
//     owner:      String!
//     name:       String
//     commander:  String
//     bracket:    String
//     color:      String
//     archetype:  String
//     cardCount:  Int!
//     importedAt: String
//   }
//
//   type Query {
//     game(id: Int!): Game
//     games(limit: Int = 20): [Game]
//     deck(owner: String!, id: String!): Deck
//     decks(owner: String, limit: Int = 50): [Deck]
//   }
//
// Design notes:
//
//   - All resolvers read through existing helpers (sm.GetGame,
//     findDeckFile, resolveDeckMetadata, countCards, enrichDeckSummary)
//     so the graph stays backed by the same in-memory + filesystem
//     stores the REST handlers use. No shadow data layer.
//
//   - The Deck type is the CHEAP DeckSummary subset, mirroring the
//     POC's "Game without Timeline" tradeoff: heavy DB-backed fields
//     (custom name, tags, cloned-from) are omitted so resolvers can
//     answer queries without touching SQLite. Promote them later if
//     a real consumer needs them — the graph can grow additively.
//
//   - Game.decks is the cross-reference that justifies GraphQL over
//     REST here: in one round-trip a client can pull `games { decks
//     { commander archetype } }` instead of N+1 REST GETs. The
//     resolver does a per-game per-deckKey filesystem lookup —
//     bounded by the seat count (4) per game so the cost is
//     constant per game.
//
//   - Owner/id args are validated through validatePathComponent
//     before they touch the filesystem; the same path-traversal
//     guard the REST handlers use. Without it, an attacker could
//     reach `deck(owner: "..", id: "../etc/passwd.txt")` through
//     the graph even though every direct REST path is locked down.

// loadGraphQLDeckSummary builds a DeckSummary for {owner, id} by
// reading from disk. Returns nil when the deck doesn't exist (the
// resolver's null-on-missing contract). Mirrors the per-deck work
// handleListDecks does, minus the SQLite-backed fields (custom
// name, tags, cloned-from) that would require carrying h.DB into
// the resolver — those can be added later under the same Deck
// type without changing the graph shape.
func loadGraphQLDeckSummary(decksDir, owner, id string) *DeckSummary {
	if decksDir == "" {
		return nil
	}
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		return nil
	}
	deckPath := findDeckFile(decksDir, owner, id)
	if deckPath == "" {
		return nil
	}
	commander, bracket, color, cmdrCard := resolveDeckMetadata(decksDir, owner, id, deckPath)
	cards := countCards(deckPath)
	var modTime time.Time
	if info, err := os.Stat(deckPath); err == nil {
		modTime = info.ModTime()
	}
	ds := DeckSummary{
		ID:            id,
		Owner:         owner,
		Name:          commander,
		Commander:     commander,
		CommanderCard: cmdrCard,
		CardCount:     cards,
		Bracket:       bracket,
		Color:         color,
		ImportedAt:    modTime,
	}
	enrichDeckSummary(decksDir, &ds)
	return &ds
}

// listGraphQLDeckSummaries walks the decks directory the same way
// handleListDecks does and returns DeckSummary values, optionally
// filtered to one owner. Excluded reserved dirs match the REST
// handler's list. limit caps result size; <=0 means "use default".
func listGraphQLDeckSummaries(decksDir, ownerFilter string, limit int) []DeckSummary {
	if decksDir == "" {
		return nil
	}
	if ownerFilter != "" && !validatePathComponent(ownerFilter) {
		return nil
	}
	out := []DeckSummary{}
	owners, err := os.ReadDir(decksDir)
	if err != nil {
		return out
	}
	for _, ownerEntry := range owners {
		if !ownerEntry.IsDir() {
			continue
		}
		owner := ownerEntry.Name()
		if ownerFilter != "" && owner != ownerFilter {
			continue
		}
		// Keep the same reserved-dir exclusions as handleListDecks.
		if owner == "freya" || owner == "benched" || owner == "test" || owner == "moxfield_300" || owner == ".versions" {
			continue
		}
		ownerDir := filepath.Join(decksDir, owner)
		files, err := os.ReadDir(ownerDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || (!strings.HasSuffix(f.Name(), ".txt") && !strings.HasSuffix(f.Name(), ".json")) {
				continue
			}
			id := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			if ds := loadGraphQLDeckSummary(decksDir, owner, id); ds != nil {
				out = append(out, *ds)
			}
		}
	}
	// Newest first, matching the REST list endpoint's sort.
	sort.Slice(out, func(i, j int) bool { return out[i].ImportedAt.After(out[j].ImportedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// buildGraphQLSchema wires the Game + Deck types and the Query root.
// Resolvers close over h so they can reach sm + decksDir without a
// separate dispatch table.
func buildGraphQLSchema(h *GraphQLHandler) (graphql.Schema, error) {
	// Deck type — built first so Game.decks can reference it. The
	// fields mirror the safe (no-SQLite) subset of DeckSummary.
	deckType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "Deck",
		Description: "A registered Commander deck (filesystem-backed summary).",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Deck slug as stored on disk (matches the REST owner/{id} path component).",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(DeckSummary).ID, nil
				},
			},
			"owner": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Owner slug.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(DeckSummary).Owner, nil
				},
			},
			"name": &graphql.Field{
				Type:        graphql.String,
				Description: "Display name. Derived from commander when no custom name has been set.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(DeckSummary).Name, nil
				},
			},
			"commander": &graphql.Field{
				Type:        graphql.String,
				Description: "Primary commander name parsed from the deck file's COMMANDER: line.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(DeckSummary).Commander, nil
				},
			},
			"bracket": &graphql.Field{
				Type:        graphql.String,
				Description: "Deck bracket label (B1..B5 / ?), derived from the filename marker or Freya strategy.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(DeckSummary).Bracket, nil
				},
			},
			"color": &graphql.Field{
				Type:        graphql.String,
				Description: "Color identity (WUBRG combo).",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(DeckSummary).Color, nil
				},
			},
			"archetype": &graphql.Field{
				Type:        graphql.String,
				Description: "Freya-detected archetype (Aggro/Combo/Control/...). Null when no strategy file present.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(DeckSummary).Archetype, nil
				},
			},
			"cardCount": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.Int),
				Description: "Total card count (sum of quantities, not unique names).",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return p.Source.(DeckSummary).CardCount, nil
				},
			},
			"importedAt": &graphql.Field{
				Type:        graphql.String,
				Description: "RFC3339 timestamp of last on-disk mtime.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					t := p.Source.(DeckSummary).ImportedAt
					if t.IsZero() {
						return nil, nil
					}
					return t.UTC().Format("2006-01-02T15:04:05Z"), nil
				},
			},
		},
	})

	// Game type — POC fields preserved verbatim, plus the new
	// `decks: [Deck]` cross-reference that resolves each deckKey
	// into a populated Deck object lazily (only fired when the
	// client asks for the field, per GraphQL's projection model).
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
			"decks": &graphql.Field{
				Type:        graphql.NewList(deckType),
				Description: "DeckKeys resolved into populated Deck objects (one filesystem read per seat). The canonical GraphQL win over the REST surface: one round-trip vs N+1 GETs. Entries are null where the deck file is missing or the key isn't shaped as owner/id.",
				Resolve: func(p graphql.ResolveParams) (any, error) {
					g := p.Source.(CompletedGame)
					out := make([]any, 0, len(g.DeckKeys))
					for _, key := range g.DeckKeys {
						owner, id := splitDeckKey(key)
						if owner == "" || id == "" {
							out = append(out, nil)
							continue
						}
						if ds := loadGraphQLDeckSummary(h.decksDir, owner, id); ds != nil {
							out = append(out, *ds)
						} else {
							out = append(out, nil)
						}
					}
					return out, nil
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
			"deck": &graphql.Field{
				Type:        deckType,
				Description: "Look up one deck by owner + id. Returns null when the file isn't present or the path components fail validation (no traversal allowed).",
				Args: graphql.FieldConfigArgument{
					"owner": &graphql.ArgumentConfig{
						Type:        graphql.NewNonNull(graphql.String),
						Description: "Owner slug.",
					},
					"id": &graphql.ArgumentConfig{
						Type:        graphql.NewNonNull(graphql.String),
						Description: "Deck slug.",
					},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					owner, _ := p.Args["owner"].(string)
					id, _ := p.Args["id"].(string)
					ds := loadGraphQLDeckSummary(h.decksDir, owner, id)
					if ds == nil {
						return nil, nil
					}
					return *ds, nil
				},
			},
			"decks": &graphql.Field{
				Type:        graphql.NewList(deckType),
				Description: "Decks, newest first. Optional owner filter restricts to one owner. Reserved dirs (freya/benched/test/moxfield_300/.versions) are excluded, matching the REST list endpoint.",
				Args: graphql.FieldConfigArgument{
					"owner": &graphql.ArgumentConfig{
						Type:        graphql.String,
						Description: "Optional owner filter. When absent, returns decks across all owners.",
					},
					"limit": &graphql.ArgumentConfig{
						Type:         graphql.Int,
						DefaultValue: 50,
						Description:  "Max rows to return after the newest-first sort.",
					},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					owner, _ := p.Args["owner"].(string)
					limit, _ := p.Args["limit"].(int)
					if limit < 0 {
						return nil, errors.New("limit must be non-negative")
					}
					return listGraphQLDeckSummaries(h.decksDir, owner, limit), nil
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{Query: queryRoot})
}

