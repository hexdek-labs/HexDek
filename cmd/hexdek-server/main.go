// hexdek-server is the HTTP entry point for the HexDek MTG platform.
//
// Ship 1 scope: load Hex's Yuriko v1.1 deck from disk, shuffle it via
// Fisher-Yates with crypto/rand entropy, and expose a single endpoint that
// reveals the top N cards of the shuffled library. No WebSockets, no
// multi-player, no SQLite yet — that's Ship 2.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hexdek/hexdek/internal/ai"
	"github.com/hexdek/hexdek/internal/auth"
	"github.com/hexdek/hexdek/internal/credits"
	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/friends"
	"github.com/hexdek/hexdek/internal/hexapi"
	"github.com/hexdek/hexdek/internal/hub"
	"github.com/hexdek/hexdek/internal/moxfield"
	"github.com/hexdek/hexdek/internal/oracle"
	"github.com/hexdek/hexdek/internal/party"
	"github.com/hexdek/hexdek/internal/pincer"
	"github.com/hexdek/hexdek/internal/preferences"
	"github.com/hexdek/hexdek/internal/shuffle"
	"github.com/hexdek/hexdek/internal/userprofile"
	"github.com/hexdek/hexdek/internal/ws"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:8090", "HTTP listen address")
	deckPath := flag.String("deck", "data/decks/yuriko_v1.json", "Path to test deck JSON file")
	dbPath := flag.String("db", "data/hexdek.db", "SQLite path (use :memory: for ephemeral)")
	indexHTML := flag.String("index-html", "", "Path to built SPA index.html for share-page OG injection (optional)")
	pincerAdminToken := flag.String("pincer-admin-token", os.Getenv("PINCER_ADMIN_TOKEN"), "Bearer token gating /api/analytics/sessions (env: PINCER_ADMIN_TOKEN)")
	pincerSecure := flag.Bool("pincer-secure-cookie", false, "Set Secure flag on hexdek_session cookie (true behind HTTPS)")
	flag.Parse()

	// Sanity check: warn if the configured deck file is missing so the
	// operator gets a clear hint before LoadDeckFromFile fatals a few lines
	// down. (The historical init()-time check ran with a hardcoded path
	// before flags were parsed and only warned on non-IsNotExist errors —
	// i.e. it never fired for the case operators actually care about.)
	if _, statErr := os.Stat(*deckPath); statErr != nil {
		log.Printf("warning: deck file %s not accessible: %v", *deckPath, statErr)
	}

	// Open SQLite
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()
	log.Printf("sqlite ready at %s", *dbPath)

	// Background context for long-lived workers (session GC, etc.). The
	// SIGINT/SIGTERM handler below cancels this so workers shut down
	// cleanly alongside the http server. Defined here at top level so
	// each subsystem that wants graceful shutdown can plug into it.
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// r60: wire the long-documented session-expiry reaper that was
	// never actually scheduled. Runs one synchronous pass now (clears
	// rows that expired during downtime), then ticks hourly until
	// bgCancel fires at shutdown. Without this, expired session rows
	// accumulated in the SQLite session table indefinitely — privacy
	// leak (device_id + last_used_at history persists past intended
	// lifetime), audit-trail noise, and slow row bloat.
	auth.RunSessionGC(bgCtx, database, time.Hour)

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
	// r60 round 2: per-IP token buckets on the two mutating Showmatch
	// endpoints. Gauntlet start is already protected by a global cap-2
	// semaphore + credit gate, but the limiter blocks rapid-cycle abuse
	// against the cap. Spectate spawn has no upstream protection and
	// each room runs a game-driver goroutine. Limiters are nil-safe in
	// tests / ephemeral builds; here we wire defaults for the
	// production server.
	sm.GauntletLimiter = hexapi.NewRateLimiter(3, 1.0/60.0)      // 3-burst, 1/min
	sm.SpectateSpawnLimiter = hexapi.NewRateLimiter(5, 1.0/30.0) // 5-burst, 1 per 30s
	sm.RegisterShowmatch(mux)

	// HexDek API: deck listing, Freya analysis, live stats
	hexAPI := &hexapi.Handler{
		DecksDir:      "data/decks",
		Showmatch:     sm,
		IndexHTMLPath: *indexHTML,
		// r60: per-IP token bucket on POST /api/feedback. 5-token
		// burst + 1-token-per-minute refill — enough for a real
		// user opening multiple bug reports in a session, tight
		// enough to throttle a bot blasting the disk.
		FeedbackLimiter: hexapi.NewRateLimiter(5, 1.0/60.0),
		// r60 round 2: per-IP token bucket shared across the deck-write
		// endpoints (POST /api/decks, POST /api/decks/import, POST
		// /api/import/moxfield, POST /api/decks/{owner}/{id}/analyze).
		// 10-burst + 1 per 30s — sized for a legitimate "import 5
		// decks back-to-back, then analyze a couple" session, tight
		// enough to stop a bot from filling DecksDir or pinning the
		// Freya subprocess queue.
		DeckImportLimiter: hexapi.NewRateLimiter(10, 1.0/30.0),
		// r60: FIRST per-user (not per-IP) rate limit. Buckets by
		// X-HexDek-Owner so a household sharing one IP doesn't
		// cross-throttle and one user's edit session on phone + laptop
		// shares a single budget. Applied to PUT/PATCH/DELETE on
		// /api/decks/{owner}/{id} — the owner-authenticated deck
		// mutation surface that previously had no rate limit at all.
		// 30-burst + 1 per 10s — sized for a real "tidy up my deck
		// collection" session (batch rename + delete a handful of old
		// versions) without letting a runaway script churn the
		// versioning DAG.
		DeckMutationLimiter: hexapi.NewRateLimiter(30, 1.0/10.0),
	}
	// r60: stateless CSRF tokens for destructive endpoints (DELETE
	// /api/decks/{owner}/{id} + POST /api/decks/{owner}/{id}/clone).
	// Wiring is opt-in via HEXDEK_CSRF_ENFORCE=1 so the existing React
	// SPA keeps working until it's been updated to call GET /api/csrf
	// and echo the token in X-CSRF-Token. When unset (default):
	//   - CSRFStore is nil
	//   - GET /api/csrf returns 503 (clear signal that the server
	//     isn't participating in tokens yet)
	//   - RequireCSRF wrappers on DELETE/clone pass through unchanged
	// When HEXDEK_CSRF_ENFORCE=1: both issuance and enforcement go live.
	// Secret source: HEXDEK_CSRF_SECRET env (preferred for multi-replica
	// deployments where tokens must survive a single node's restart);
	// falls back to per-boot random bytes.
	if os.Getenv("HEXDEK_CSRF_ENFORCE") == "1" {
		csrfSecret := []byte(os.Getenv("HEXDEK_CSRF_SECRET"))
		if len(csrfSecret) == 0 {
			var err error
			csrfSecret, err = hexapi.RandomCSRFSecret()
			if err != nil {
				log.Fatalf("csrf: random secret: %v", err)
			}
			log.Printf("csrf: HEXDEK_CSRF_SECRET unset, using per-boot random secret (tokens won't survive restart)")
		}
		hexAPI.CSRFStore = hexapi.NewCSRFStore(csrfSecret, 0) // default 1-hour TTL
		// Share the same store with the Showmatch routes so the
		// late-binding requireCSRFLate wrapper picks it up on the
		// gauntlet / live-speed / curse-PATCH / spectate-spawn paths.
		// (sm.RegisterShowmatch ran earlier — see requireCSRFLate doc.)
		sm.CSRFStore = hexAPI.CSRFStore
		log.Printf("csrf: enforcement ENABLED on DELETE + clone endpoints")
	} else {
		log.Printf("csrf: disabled (set HEXDEK_CSRF_ENFORCE=1 to enable token issuance + enforcement)")
	}
	hexAPI.SetDB(database)
	if err := hexapi.EnsureDeckMetaSchema(context.Background(), database); err != nil {
		log.Fatalf("deck_meta schema: %v", err)
	}

	// Credit economy: tracks per-owner balance + ledger; gates
	// gauntlet runs past the free-tier daily quota. Wired into the
	// Showmatch gauntlet handler so a paid run automatically debits
	// the caller's balance and logs to the audit trail.
	if err := credits.EnsureSchema(context.Background(), database); err != nil {
		log.Fatalf("credits schema: %v", err)
	}
	creditStore := credits.New(database)
	creditStore.Register(mux)
	sm.SetCreditStore(creditStore)
	hexAPI.LoadCardDB("data/rules/oracle-cards.json")
	hexAPI.LoadOwnerAliases("data/owner-aliases.json")
	hexAPI.Register(mux)
	(&hexapi.AdminConvictionHandler{}).Register(mux)
	log.Printf("showmatch: loading in background — live games at /api/live/game")

	// pprof debug endpoints — localhost only, gated behind env var
	if os.Getenv("ENABLE_PPROF") == "1" {
		go func() {
			const pprofAddr = "127.0.0.1:6060"
			log.Printf("pprof: listening on %s", pprofAddr)
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				log.Printf("pprof: endpoint disabled, ListenAndServe(%s): %v", pprofAddr, err)
			}
		}()
	}

	// Ship 6: Web UI (static files served from web/ if it exists)
	mux.Handle("GET /ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir("web"))))

	// Convenience: serve the configured test deck JSON so the UI can fetch
	// it directly. Route slug stays `yuriko-deck-json` for client back-compat,
	// but the served file follows --deck so operators who point at a
	// non-Yuriko deck don't get a silent UI/data mismatch.
	testDeckPath := *deckPath
	mux.HandleFunc("GET /api/test/yuriko-deck-json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, testDeckPath)
	})

	// Temporal Pincer: anonymous UUID session tracking + auth stitching.
	// Registers /api/analytics/sessions (admin-gated) and /api/pincer/stitch.
	pincerTracker, err := pincer.New(database, pincer.Options{
		AdminToken:   *pincerAdminToken,
		SecureCookie: *pincerSecure,
	})
	if err != nil {
		log.Fatalf("pincer init: %v", err)
	}
	pincerTracker.Register(mux)

	// User profile: per-owner country detection from Accept-Language.
	if err := userprofile.EnsureSchema(context.Background(), database); err != nil {
		log.Fatalf("user_profile schema: %v", err)
	}
	userprofile.Register(mux, database)

	// User preferences: per-device GET/PATCH /api/me/preferences
	// (display_name + owner_name). Auth-gated via the session token
	// path on internal/auth. Closes docs/half-finished-features-r48.md
	// #8 — Profile.jsx now syncs to this endpoint with localStorage
	// as an offline fallback.
	if err := preferences.EnsureSchema(context.Background(), database); err != nil {
		log.Fatalf("user_preferences schema: %v", err)
	}
	preferences.Register(mux, database)

	// Friends: owner-keyed bidirectional friend list.
	friendTracker, err := friends.New(database)
	if err != nil {
		log.Fatalf("friends init: %v", err)
	}
	friendTracker.Register(mux)

	// BOINC contributor dispatcher: distributed-compute clients
	// (cmd/hexdek-contrib) connect over WS, receive signed work
	// chunks, and earn credits visible on the operator profile.
	contribDispatcher := hexapi.NewContribDispatcher(database)
	contribDispatcher.SpotCheckPct = hexapi.ParseSpotCheckPct(os.Getenv("HEXDEK_CONTRIB_SPOTCHECK_PCT"), 0.03)
	// Spot-check callback re-runs a fraction of returned chunks on the
	// same engine the client used. Corpus + meta are loaded lazily on
	// first spot-check so server startup isn't blocked.
	contribAST := envOr("HEXDEK_CONTRIB_AST", "data/rules/ast_dataset.jsonl")
	contribOracle := os.Getenv("HEXDEK_CONTRIB_ORACLE")
	contribDispatcher.SpotCheck = hexapi.NewSpotCheckRunner(contribAST, contribAST, contribOracle).Run
	contribDispatcher.Register(mux)

	// r60 observability: in-process Prometheus metrics + structured
	// per-request log lines. Registry is constructed before route
	// registration so the /metrics endpoint always serves an
	// initialized registry (the metrics_endpoint.go handler returns
	// 503 on nil). Stdout logger writes greppable key=value lines —
	// journald / file redirect picks them up at deploy time.
	metricsRegistry := hexapi.NewMetricsRegistry(nil)
	mux.HandleFunc("GET /metrics", hexapi.HandleMetrics(metricsRegistry))

	log.Printf("listening on %s", *addr)
	log.Printf("Ship 1: curl http://%s/game/test/library/top/3", *addr)
	log.Printf("Ship 2: curl -XPOST http://%s/api/device/register -d '{\"display_name\":\"Hex\"}'", *addr)
	log.Printf("Ship 3: ws://%s/ws/party/{id}?token={token}", *addr)

	// Observability middleware sits OUTSIDE RequestIDMiddleware so
	// each log line can pull the request ID via RequestIDFromContext
	// (the ID is stamped before the wrapped handler runs). Metrics
	// recording uses r.Pattern as the endpoint label, so cardinality
	// stays bounded by route count.
	handler := hexapi.RequestIDMiddleware(
		hexapi.ObservabilityMiddleware(metricsRegistry, hexapi.StdoutLogger(os.Stdout),
			corsMiddleware(pincerTracker.Middleware(userprofile.LocaleMiddleware(mux)))))
	httpSrv := &http.Server{
		Addr:    *addr,
		Handler: handler,
		// Slow-loris hardening. WebSocket upgrades hijack the connection
		// so these per-request timeouts no longer apply once the WS
		// handshake completes.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown: run ListenAndServe in a goroutine, block main on
	// SIGINT/SIGTERM, then Shutdown with a 15s drain. Defers (including
	// database.Close) fire on return — the previous log.Fatalf path bypassed
	// them via os.Exit.
	serverErr := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("received %s, draining (15s timeout)", sig)
	case err, ok := <-serverErr:
		if ok && err != nil {
			log.Printf("server: %v", err)
		}
	}

	// Cancel bgCtx FIRST so background workers (session GC, etc.) stop
	// touching the DB before Shutdown drains in-flight requests — keeps
	// the shutdown log readable and avoids a racing reaper firing
	// against a closing connection pool.
	bgCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown: %v", err)
	}
}

type server struct {
	deck *moxfield.Deck

	mu      sync.RWMutex
	library []*moxfield.Card
}

// envOr returns os.Getenv(key) when set, else def.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
	fmt.Fprintf(w, "hexdek-server (Ship 1)\n\n")
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
	payload := map[string]any{
		"deck_name":    s.deck.Name,
		"library_size": len(s.library),
		"revealed_top": view,
	}
	if len(s.library) > 0 {
		payload["yuriko_damage_if_top_revealed"] = s.library[0].CMC
	}
	if err := enc.Encode(payload); err != nil {
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

// isOriginAllowed returns true when origin matches the production
// allowlist OR a dev-environment host (localhost / 127.0.0.1 / the
// 192.168.1.0/24 LAN range). Uses url.Parse + Hostname() rather
// than string-prefix matching — a HasPrefix check on
// "http://localhost:" would also match
// "http://localhost:5173.evil.example", letting an attacker who
// controls *.evil.example bypass the allowlist (the literal string
// DOES start with "http://localhost:"). Parsing decomposes the
// origin into scheme+host so we compare against the host component
// only.
//
// Returns false on any parse error so a malformed Origin header
// cannot match.
func isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	if allowedOrigins[origin] {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	// Only http/https are valid web Origins. Rejects custom schemes
	// like "data:" / "file:" / "chrome-extension:" that could be
	// abused if we left scheme unchecked.
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return true
	}
	// LAN dev: any 192.168.1.X address. Parses as IPv4 + checks the
	// /24 — defends against tricks like "192.168.1.1.evil.example"
	// (would have Hostname() = "192.168.1.1.evil.example", IP parse
	// fails, returns false).
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil && v4[0] == 192 && v4[1] == 168 && v4[2] == 1 {
			return true
		}
	}
	return false
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Vary: Origin is required whenever the Access-Control-Allow-Origin
		// header varies by request. Without it, CDNs and browser caches
		// serve a response cached for one allowed origin to a different
		// origin, silently breaking CORS or worse.
		w.Header().Add("Vary", "Origin")
		if isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			// Allow-Credentials lets frontend fetches with
			// credentials:'include' carry the pincer hexdek_session cookie.
			// Required by spec to be paired with a specific (non-wildcard)
			// Allow-Origin, which we already echo.
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
		// Expose X-Request-Id so browser clients can read it off
		// the response and surface it in error reports — the
		// header is non-CORS-safelisted, so without this it's
		// hidden from fetch().
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

