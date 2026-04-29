// mtgsquad-server is the HTTP entry point for the mtgsquad MTG platform.
//
// Ship 1 scope: load Hex's Yuriko v1.1 deck from disk, shuffle it via
// Fisher-Yates with crypto/rand entropy, and expose a single endpoint that
// reveals the top N cards of the shuffled library. No WebSockets, no
// multi-player, no SQLite yet — that's Ship 2.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"strconv"
	"sync"

	"github.com/hexdek/hexdek/internal/ai"
	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/hexapi"
	"github.com/hexdek/hexdek/internal/hub"
	"github.com/hexdek/hexdek/internal/moxfield"
	"github.com/hexdek/hexdek/internal/oracle"
	"github.com/hexdek/hexdek/internal/party"
	"github.com/hexdek/hexdek/internal/shuffle"
	"github.com/hexdek/hexdek/internal/ws"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:8090", "HTTP listen address")
	deckPath := flag.String("deck", "data/decks/yuriko_v1.json", "Path to test deck JSON file")
	dbPath := flag.String("db", "data/mtgsquad.db", "SQLite path (use :memory: for ephemeral)")
	flag.Parse()

	// Open SQLite
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()
	log.Printf("sqlite ready at %s", *dbPath)

	// Load test deck (Ship 1 demo)
	deck, err := moxfield.LoadDeckFromFile(*deckPath)
	if err != nil {
		log.Fatalf("load deck: %v", err)
	}
	log.Printf("loaded test deck %q by %q (%d cards + commander)",
		deck.Name, deck.Author, deck.CardCount())

	srv := &server{
		deck: deck,
	}
	if err := srv.shuffleLibrary(); err != nil {
		log.Fatalf("initial shuffle: %v", err)
	}

	// Build mux with both Ship 1 and Ship 2 endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleRoot)

	// Ship 1: test deck shuffle/reveal
	mux.HandleFunc("GET /game/test/library/top/{n}", srv.handleTopN)
	mux.HandleFunc("GET /game/test/library/reshuffle", srv.handleReshuffle)
	mux.HandleFunc("GET /game/test/deck-info", srv.handleDeckInfo)
	mux.HandleFunc("GET /health", srv.handleHealth)

	// Ship 2: device + deck + party endpoints
	partyHandler := &party.Handler{DB: database}

	// Ship 3: WebSocket transport + connection hub
	connHub := hub.New()
	wsHandler := &ws.Handler{DB: database, Hub: connHub}

	// Wire AI autopilot: when a game starts, spin up the autopilot goroutine
	// if any seats are AI-controlled. Broadcasting is done through the WS
	// handler's public fan-out method so AI-driven state changes reach every
	// connected device.
	partyHandler.OnGameStart = func(gameID, partyID string, numPlayers int) {
		ai.Start(context.Background(), database, gameID, numPlayers, func(gid string) {
			wsHandler.BroadcastSnapshots(context.Background(), partyID, gid)
		})
	}

	partyHandler.Register(mux)
	wsHandler.Register(mux)

	// Ship 7: Card oracle (Scryfall integration with SQLite cache)
	oracleHandler := &oracle.Handler{DB: database}
	oracleHandler.Register(mux)

	// Showmatch: continuous AI vs AI games for the fishtank/spectator.
	// Loads AST corpus + decks in background so the server starts immediately.
	sm := hexapi.NewShowmatch(
		"data/rules/ast_dataset.jsonl",
		"data/rules/oracle-cards.json",
		"data/decks",
		database,
	)
	sm.RegisterShowmatch(mux)

	// HexDek API: deck listing, Freya analysis, live stats
	hexAPI := &hexapi.Handler{DecksDir: "data/decks", Showmatch: sm}
	hexAPI.Register(mux)
	log.Printf("showmatch: loading in background — live games at /api/live/game")

	// pprof debug endpoints — localhost only, gated behind env var
	if os.Getenv("ENABLE_PPROF") == "1" {
		go func() {
			log.Printf("pprof: listening on 127.0.0.1:6060")
			http.ListenAndServe("127.0.0.1:6060", nil)
		}()
	}

	// Ship 6: Web UI (static files served from web/ if it exists)
	mux.Handle("GET /ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir("web"))))

	// Convenience: serve the test deck JSON so the UI can fetch it directly.
	mux.HandleFunc("GET /api/test/yuriko-deck-json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "data/decks/yuriko_v1.json")
	})

	log.Printf("listening on %s", *addr)
	log.Printf("Ship 1: curl http://%s/game/test/library/top/3", *addr)
	log.Printf("Ship 2: curl -XPOST http://%s/api/device/register -d '{\"display_name\":\"Hex\"}'", *addr)
	log.Printf("Ship 3: ws://%s/ws/party/{id}?token={token}", *addr)

	handler := corsMiddleware(mux)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

type server struct {
	deck *moxfield.Deck

	mu      sync.RWMutex
	library []*moxfield.Card
}

// shuffleLibrary builds a fresh expanded library from the deck's mainboard
// and shuffles it in-place.
func (s *server) shuffleLibrary() error {
	library := s.deck.ExpandLibrary()
	if err := shuffle.Shuffle(library); err != nil {
		return err
	}
	s.mu.Lock()
	s.library = library
	s.mu.Unlock()
	return nil
}

func (s *server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprintf(w, "mtgsquad-server (Ship 1)\n\n")
	fmt.Fprintf(w, "Loaded deck: %s\n", s.deck.Name)
	fmt.Fprintf(w, "Commander: %s\n", s.deck.Commander)
	fmt.Fprintf(w, "Library size: %d\n\n", s.deck.CardCount())
	fmt.Fprintf(w, "Endpoints:\n")
	fmt.Fprintf(w, "  GET /game/test/library/top/N     -- reveal top N cards\n")
	fmt.Fprintf(w, "  GET /game/test/library/reshuffle -- new shuffle, returns library size\n")
	fmt.Fprintf(w, "  GET /game/test/deck-info         -- deck metadata\n")
	fmt.Fprintf(w, "  GET /health                      -- server health\n")
}

func (s *server) handleTopN(w http.ResponseWriter, r *http.Request) {
	nStr := r.PathValue("n")
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 1 {
		http.Error(w, "invalid N (must be positive integer)", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if n > len(s.library) {
		n = len(s.library)
	}

	type cardView struct {
		Position int    `json:"position"`
		Name     string `json:"name"`
		ManaCost string `json:"mana_cost"`
		CMC      int    `json:"cmc"`
		Types    []string `json:"types,omitempty"`
	}

	view := make([]cardView, n)
	for i := 0; i < n; i++ {
		c := s.library[i]
		view[i] = cardView{
			Position: i,
			Name:     c.Name,
			ManaCost: c.ManaCost,
			CMC:      c.CMC,
			Types:    c.Types,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"deck_name":     s.deck.Name,
		"library_size":  len(s.library),
		"revealed_top":  view,
		"yuriko_damage_if_top_revealed": s.library[0].CMC,
	}); err != nil {
		log.Printf("encode error: %v", err)
	}
}

func (s *server) handleReshuffle(w http.ResponseWriter, r *http.Request) {
	if err := s.shuffleLibrary(); err != nil {
		http.Error(w, fmt.Sprintf("shuffle failed: %v", err), http.StatusInternalServerError)
		return
	}
	s.mu.RLock()
	size := len(s.library)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"shuffled":     true,
		"library_size": size,
	})
}

func (s *server) handleDeckInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"name":        s.deck.Name,
		"author":      s.deck.Author,
		"format":      s.deck.Format,
		"commander":   s.deck.Commander,
		"card_count":  s.deck.CardCount(),
	})
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

var allowedOrigins = map[string]bool{
	"https://hexdek.bluefroganalytics.com": true,
	"http://localhost:5173":                true,
	"http://localhost:4173":                true,
	"http://127.0.0.1:5173":               true,
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] || strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://192.168.1.") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Sanity check at startup that the deck file exists before binding to a port.
func init() {
	if _, err := os.Stat("data/decks/yuriko_v1.json"); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: deck file pre-check failed: %v", err)
	}
}
