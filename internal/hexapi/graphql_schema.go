package hexapi

import (
	"errors"
	"fmt"
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
//   input DeckImportInput {
//     deckList: String!
//     owner:    String
//     name:     String
//   }
//
//   type Mutation {
//     importDeck(input: DeckImportInput!): Deck
//     deleteDeck(owner: String!, id: String!): Boolean
//   }
//
// Mutations notes:
//
//   - importDeck mirrors POST /api/decks: writes a deck file under
//     decksDir/{owner}/{id}.txt with the same sanitization,
//     unique-filename collision avoidance, and default owner/name
//     rules. Anonymous-friendly (no caller-owner check) because
//     "import a deck" is a self-creation action.
//
//   - deleteDeck mirrors DELETE /api/decks/{owner}/{id}: gated on
//     X-HexDek-Owner via callerOwner(ctx) — caller MUST match the
//     target owner. Returns true on successful deletion, false on
//     missing file; ownership failure / path-traversal raise a
//     GraphQL error so the client sees it under `errors` rather
//     than as a silent false.
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

	// ---- Mutations ----
	deckImportInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:        "DeckImportInput",
		Description: "Body for importDeck. deckList is the raw deck text (parsed lazily on read); owner + name fall back to defaults when blank.",
		Fields: graphql.InputObjectConfigFieldMap{
			"deckList": &graphql.InputObjectFieldConfig{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Deck list body — same format the REST POST /api/decks accepts.",
			},
			"owner": &graphql.InputObjectFieldConfig{
				Type:        graphql.String,
				Description: "Owner slug. Sanitized to [a-z0-9_-]; falls back to \"imported\" when blank.",
			},
			"name": &graphql.InputObjectFieldConfig{
				Type:        graphql.String,
				Description: "Display name. Sanitized to [a-z0-9_-]; falls back to \"imported_deck\" when blank.",
			},
		},
	})

	mutationRoot := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"importDeck": &graphql.Field{
				Type:        deckType,
				Description: "Write a new deck file under decksDir/{owner}/{id}.txt and return the resulting Deck. Anonymous — no caller-owner check (importing a deck is a self-creation action).",
				Args: graphql.FieldConfigArgument{
					"input": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(deckImportInput),
					},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					input, _ := p.Args["input"].(map[string]any)
					if input == nil {
						return nil, errors.New("importDeck: missing input")
					}
					deckList, _ := input["deckList"].(string)
					if strings.TrimSpace(deckList) == "" {
						return nil, errors.New("importDeck: deckList is required")
					}
					owner, _ := input["owner"].(string)
					name, _ := input["name"].(string)
					resolvedOwner, resolvedID, err := writeImportedDeckFile(h.decksDir, owner, name, deckList)
					if err != nil {
						return nil, err
					}
					ds := loadGraphQLDeckSummary(h.decksDir, resolvedOwner, resolvedID)
					if ds == nil {
						// Should be impossible — the write just succeeded
						// — but if loadDeckSummary fails the client
						// shouldn't get a misleading null without context.
						return nil, fmt.Errorf("importDeck: file written but reload failed (owner=%s id=%s)",
							resolvedOwner, resolvedID)
					}
					return *ds, nil
				},
			},
			"deleteDeck": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "Remove a deck file. Caller's X-HexDek-Owner header MUST equal the target owner; mismatched or missing header raises an error rather than silently returning false (so the client can't misread it as \"deck didn't exist\").",
				Args: graphql.FieldConfigArgument{
					"owner": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					owner, _ := p.Args["owner"].(string)
					id, _ := p.Args["id"].(string)
					if !validatePathComponent(owner) || !validatePathComponent(id) {
						return nil, errors.New("deleteDeck: invalid owner or id")
					}
					caller := callerOwner(p.Context)
					if caller == "" {
						return nil, errors.New("deleteDeck: caller identity missing (set X-HexDek-Owner)")
					}
					if caller != strings.ToLower(owner) {
						return nil, errors.New("deleteDeck: forbidden — caller is not the deck owner")
					}
					if h.decksDir == "" {
						return false, nil
					}
					deckPath := findDeckFile(h.decksDir, owner, id)
					if deckPath == "" {
						// File didn't exist; the operation is a no-op,
						// not an error. Mirrors the REST handler's
						// 404 → false (here) distinction.
						return false, nil
					}
					if err := os.Remove(deckPath); err != nil {
						return nil, fmt.Errorf("deleteDeck: remove: %w", err)
					}
					// Clean up Freya strategy file alongside, same as the
					// REST handler does — keeps the on-disk state
					// consistent across mutation paths.
					_ = os.Remove(filepath.Join(h.decksDir, owner, "freya", id+".strategy.json"))
					return true, nil
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryRoot,
		Mutation: mutationRoot,
	})
}

// writeImportedDeckFile mirrors handleImportDeck's disk-write logic.
// Sanitizes owner + name, creates the owner dir, finds a unique
// filename (appends _2.._100 on collision), writes the deck list.
// Returns the resolved owner + id so the caller can reload a Deck
// summary off the same path.
//
// Kept here rather than refactoring handleImportDeck to call into it
// because the REST handler does several response-specific bits
// (request-body limit reader, DeckSummary marshal) that aren't worth
// generalizing for one extra consumer. Behavior MUST stay in sync —
// covered by the graphql_mutations test that exercises sanitization
// + collision rules end-to-end.
func writeImportedDeckFile(decksDir, ownerArg, nameArg, deckList string) (owner, id string, err error) {
	if decksDir == "" {
		return "", "", errors.New("decksDir not configured")
	}
	owner = strings.TrimSpace(ownerArg)
	if owner == "" {
		owner = "imported"
	}
	owner = sanitizeFilename(owner)
	if owner == "" {
		return "", "", errors.New("owner sanitized to empty")
	}
	name := strings.TrimSpace(nameArg)
	if name == "" {
		name = "imported_deck"
	}
	fileID := sanitizeFilename(strings.ToLower(name))
	if fileID == "" {
		fileID = "deck"
	}
	ownerDir := filepath.Join(decksDir, owner)
	if err := os.MkdirAll(ownerDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create owner dir: %w", err)
	}
	deckPath := filepath.Join(ownerDir, fileID+".txt")
	finalID := fileID
	for i := 2; ; i++ {
		if _, statErr := os.Stat(deckPath); os.IsNotExist(statErr) {
			break
		}
		finalID = fmt.Sprintf("%s_%d", fileID, i)
		deckPath = filepath.Join(ownerDir, finalID+".txt")
		if i > 100 {
			return "", "", errors.New("too many decks with the same name")
		}
	}
	if err := os.WriteFile(deckPath, []byte(deckList), 0o644); err != nil {
		return "", "", fmt.Errorf("write deck: %w", err)
	}
	return owner, finalID, nil
}

