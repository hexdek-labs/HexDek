package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

const (
	showmatchSeats   = 4
	showmatchMaxTurn = 80
	maxLogEntries    = 40
)

var phaseDelays = map[string]time.Duration{
	"untap":           200 * time.Millisecond,
	"upkeep":          400 * time.Millisecond,
	"draw":            300 * time.Millisecond,
	"precombat_main":  800 * time.Millisecond,
	"postcombat_main": 500 * time.Millisecond,
	"end":             400 * time.Millisecond,
	"cleanup":         200 * time.Millisecond,
}

const combatPhaseDelay = 1000 * time.Millisecond

type SeatSnapshot struct {
	Commander   string              `json:"commander"`
	Life        int                 `json:"life"`
	HandSize    int                 `json:"hand_size"`
	LibrarySize int                 `json:"library_size"`
	GYSize      int                 `json:"gy_size"`
	ManaPool    int                 `json:"mana_pool"`
	Lost        bool                `json:"lost"`
	Battlefield []PermanentSnapshot `json:"battlefield"`
	Eval        *EvalSnapshot       `json:"eval,omitempty"`
}

type EvalSnapshot struct {
	Score             float64 `json:"score"`
	BoardPresence     float64 `json:"board_presence"`
	CardAdvantage     float64 `json:"card_advantage"`
	ManaAdvantage     float64 `json:"mana_advantage"`
	LifeResource      float64 `json:"life_resource"`
	ComboProximity    float64 `json:"combo_proximity"`
	ThreatExposure    float64 `json:"threat_exposure"`
	CommanderProgress float64 `json:"commander_progress"`
	GraveyardValue    float64 `json:"graveyard_value"`
}

type PermanentSnapshot struct {
	Name    string `json:"name"`
	Tapped  bool   `json:"tapped"`
	Power   int    `json:"power,omitempty"`
	Tough   int    `json:"toughness,omitempty"`
	IsCmdr  bool   `json:"is_commander,omitempty"`
	IsLand  bool   `json:"is_land,omitempty"`
	Type    string `json:"type,omitempty"`
}

type LogEntry struct {
	Turn   int    `json:"turn"`
	Seat   int    `json:"seat"`
	Action string `json:"action"`
	Detail string `json:"detail,omitempty"`
	Kind   string `json:"kind"`
}

type GameSnapshot struct {
	GameID     int            `json:"game_id"`
	Turn       int            `json:"turn"`
	Phase      string         `json:"phase"`
	Step       string         `json:"step"`
	ActiveSeat int            `json:"active_seat"`
	Seats      []SeatSnapshot `json:"seats"`
	StartedAt  time.Time      `json:"started_at"`
	Finished   bool           `json:"finished"`
	Winner     int            `json:"winner"`
	EndReason  string         `json:"end_reason,omitempty"`
	Log        []LogEntry     `json:"log,omitempty"`
}

type ELOEntry struct {
	DeckID    string  `json:"deck_id"`
	Commander string  `json:"commander"`
	Owner     string  `json:"owner"`
	Rating    float64 `json:"rating"`
	Games     int     `json:"games"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	WinRate   float64 `json:"win_rate"`
	Delta     float64 `json:"delta"`
}

type SessionStats struct {
	GamesPlayed int     `json:"games_played"`
	AvgTurns    float64 `json:"avg_turns"`
	Dominant    string  `json:"dominant"`
	DomWinRate  float64 `json:"dominant_win_rate"`
	Uptime      string  `json:"uptime"`
	Status      string  `json:"status"`
	GamesPerMin float64 `json:"games_per_min"`
}

type CompletedGame struct {
	GameID     int            `json:"game_id"`
	Commanders []string      `json:"commanders"`
	DeckKeys   []string      `json:"deck_keys"`
	Winner     int            `json:"winner"`
	WinnerName string         `json:"winner_name"`
	Turns      int            `json:"turns"`
	EndReason  string         `json:"end_reason"`
	FinishedAt time.Time      `json:"finished_at"`
	FinalSeats []SeatSnapshot `json:"final_seats"`
}

type persistJob struct {
	game CompletedGame
}

type GauntletResult struct {
	DeckKey    string    `json:"deck_key"`
	Commander  string    `json:"commander"`
	Status     string    `json:"status"`
	Games      int       `json:"games"`
	Target     int       `json:"target"`
	Wins       int       `json:"wins"`
	Losses     int       `json:"losses"`
	WinRate    float64   `json:"win_rate"`
	ELOStart   float64   `json:"elo_start"`
	ELOEnd     float64   `json:"elo_end"`
	ELODelta   float64   `json:"elo_delta"`
	AvgTurns   float64   `json:"avg_turns"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	TopBeaten  []string  `json:"top_beaten,omitempty"`
	TopLostTo  []string  `json:"top_lost_to,omitempty"`
}

type Showmatch struct {
	mu              sync.RWMutex
	snap            *GameSnapshot
	elo             map[string]*eloState
	stats           sessionState
	start           time.Time
	corpus          *astload.Corpus
	meta            *deckparser.MetaDB
	deckPool        []*deckparser.TournamentDeck
	ready           bool
	loadErr         string
	gameHistory     []CompletedGame
	eventLog        []LogEntry
	speedMultiplier float64
	sqlDB           *sql.DB
	persistCh       chan persistJob

	specMu     sync.RWMutex
	spectators map[*spectatorConn]struct{}

	gauntletMu sync.RWMutex
	gauntlets  map[string]*GauntletResult
}

type spectatorConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type eloState struct {
	rating    float64
	games     int
	delta     float64
	wins      int
	commander string
	owner     string
}

type sessionState struct {
	gamesPlayed    int // current session only (grinder + showmatch)
	historicGames  int // loaded from DB on startup (all prior sessions)
	historicTurns  int
	totalTurns     int
}

func NewShowmatch(astPath, oraclePath, decksDir string, database *sql.DB) *Showmatch {
	sm := &Showmatch{
		elo:             make(map[string]*eloState),
		start:           time.Now(),
		speedMultiplier: 1.0,
		sqlDB:           database,
		persistCh:       make(chan persistJob, 512),
		spectators:      make(map[*spectatorConn]struct{}),
		gauntlets:       make(map[string]*GauntletResult),
	}
	if database != nil {
		sm.loadPersistedState()
		go sm.persistWorker()
	}
	go sm.loadAndRun(astPath, oraclePath, decksDir)
	return sm
}

func (sm *Showmatch) persistWorker() {
	for job := range sm.persistCh {
		sm.persistGame(job.game)
	}
}

func (sm *Showmatch) loadPersistedState() {
	ctx := context.Background()

	records, err := db.LoadAllELO(ctx, sm.sqlDB)
	if err != nil {
		log.Printf("showmatch: load persisted ELO: %v", err)
		return
	}
	for _, r := range records {
		sm.elo[r.DeckKey] = &eloState{
			rating:    r.Rating,
			games:     r.Games,
			wins:      r.Wins,
			delta:     r.Delta,
			commander: r.Commander,
			owner:     r.Owner,
		}
	}

	kvGames, _ := db.KVGet(ctx, sm.sqlDB, "total_games")
	kvTurns, _ := db.KVGet(ctx, sm.sqlDB, "total_turns")
	if kvGames != "" {
		fmt.Sscanf(kvGames, "%d", &sm.stats.historicGames)
		fmt.Sscanf(kvTurns, "%d", &sm.stats.historicTurns)
	} else {
		sm.stats.historicGames, _ = db.CountGames(ctx, sm.sqlDB)
		sm.stats.historicTurns, _ = db.GetTotalTurns(ctx, sm.sqlDB)
	}

	games, _ := db.LoadRecentGames(ctx, sm.sqlDB, 50)
	for _, g := range games {
		seats, _ := db.LoadGameSeats(ctx, sm.sqlDB, g.GameID)
		var commanders []string
		var finalSeats []SeatSnapshot
		for _, s := range seats {
			commanders = append(commanders, s.Commander)
			finalSeats = append(finalSeats, SeatSnapshot{
				Commander:   s.Commander,
				Life:        s.Life,
				HandSize:    s.HandSize,
				LibrarySize: s.LibrarySize,
				GYSize:      s.GYSize,
				Lost:        s.Lost,
			})
		}
		sm.gameHistory = append(sm.gameHistory, CompletedGame{
			GameID:     int(g.GameID),
			Commanders: commanders,
			Winner:     g.Winner,
			WinnerName: g.WinnerName,
			Turns:      g.Turns,
			EndReason:  g.EndReason,
			FinishedAt: time.Unix(g.FinishedAt, 0),
			FinalSeats: finalSeats,
		})
	}

	if len(records) > 0 || sm.stats.historicGames > 0 {
		log.Printf("showmatch: restored %d ELO records, %d historic games from SQLite", len(records), sm.stats.historicGames)
	}
}

func (sm *Showmatch) loadAndRun(astPath, oraclePath, decksDir string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("showmatch: fatal: %v", r)
			sm.mu.Lock()
			sm.loadErr = fmt.Sprintf("%v", r)
			sm.mu.Unlock()
		}
	}()

	log.Printf("showmatch: loading AST corpus from %s", astPath)
	t0 := time.Now()
	corpus, err := astload.Load(astPath)
	if err != nil {
		log.Printf("showmatch: astload failed: %v", err)
		sm.mu.Lock()
		sm.loadErr = "failed to load card corpus"
		sm.mu.Unlock()
		return
	}
	log.Printf("showmatch: %d cards loaded in %s", corpus.Count(), time.Since(t0))

	meta, err := deckparser.LoadMetaFromJSONL(astPath)
	if err != nil {
		log.Printf("showmatch: meta failed: %v", err)
		sm.mu.Lock()
		sm.loadErr = "failed to load card metadata"
		sm.mu.Unlock()
		return
	}
	if oraclePath != "" {
		if err := meta.SupplementWithOracleJSON(oraclePath); err != nil {
			log.Printf("showmatch: oracle supplement: %v (continuing)", err)
		}
	}

	deckPaths, err := findDeckFiles(decksDir)
	if err != nil {
		log.Printf("showmatch: find decks failed: %v", err)
		sm.mu.Lock()
		sm.loadErr = "failed to load deck files"
		sm.mu.Unlock()
		return
	}

	var decks []*deckparser.TournamentDeck
	for _, p := range deckPaths {
		d, perr := deckparser.ParseDeckFile(p, corpus, meta)
		if perr != nil {
			continue
		}
		totalCards := len(d.Library) + len(d.CommanderCards)
		if len(d.CommanderCards) == 0 || totalCards < 100 {
			continue
		}
		decks = append(decks, d)
	}
	log.Printf("showmatch: %d decks parsed successfully (from %d files)", len(decks), len(deckPaths))

	if len(decks) < showmatchSeats {
		log.Printf("showmatch: only %d valid decks, need %d", len(decks), showmatchSeats)
		sm.mu.Lock()
		sm.loadErr = fmt.Sprintf("only %d valid decks", len(decks))
		sm.mu.Unlock()
		return
	}

	sm.mu.Lock()
	sm.corpus = corpus
	sm.meta = meta
	sm.deckPool = decks
	sm.ready = true
	sm.mu.Unlock()

	log.Printf("showmatch: ready — %d decks in pool, starting fishtank + background grinder", len(decks))
	go sm.runGrinder()
	go sm.runStatsBroadcaster()
	sm.runLoop()
}

func (sm *Showmatch) runLoop() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		sm.runOneGame(rng)
		sm.mu.RLock()
		mult := sm.speedMultiplier
		sm.mu.RUnlock()
		if mult <= 0 {
			mult = 1.0
		}
		time.Sleep(time.Duration(float64(500*time.Millisecond) / mult))
	}
}

func (sm *Showmatch) runGrinder() {
	workers := runtime.NumCPU() / 2
	if workers > 12 {
		workers = 12
	}
	if workers < 2 {
		workers = 2
	}

	debug.SetGCPercent(50)
	debug.SetMemoryLimit(8 * 1024 * 1024 * 1024) // 8 GB hard cap

	var totalGames atomic.Int64
	t0 := time.Now()

	log.Printf("grinder: starting %d parallel workers (GOGC=50, memlimit=8GB)", workers)

	for w := 0; w < workers; w++ {
		go func(id int) {
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)*31))
			for {
				sm.runOneGameFast(rng)
				totalGames.Add(1)
			}
		}(w)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var lastLogged int64
	for range ticker.C {
		n := totalGames.Load()
		elapsed := time.Since(t0)
		gpm := float64(n) / elapsed.Minutes()

		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		log.Printf("grinder: %d games (%.1f g/min), ELO pool: %d, heap=%.0fMB, sys=%.0fMB, gc=%d",
			n, gpm, sm.eloSize(), float64(m.HeapAlloc)/1e6, float64(m.Sys)/1e6, m.NumGC)

		if n-lastLogged >= 50 {
			sm.flushELO()
			lastLogged = n
		}
	}
}

func (sm *Showmatch) runStatsBroadcaster() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		sm.broadcastToSpectators(wsEnvelope{Type: "stats", Payload: sm.GetStats()})
	}
}

func (sm *Showmatch) findDeckInPool(owner, id string) *deckparser.TournamentDeck {
	target := owner + "/" + id
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, d := range sm.deckPool {
		if deckKeyFromPath(d.Path) == target {
			return d
		}
	}
	return nil
}

func (sm *Showmatch) RunGauntlet(owner, id string, numGames int) {
	deckKey := owner + "/" + id
	targetDeck := sm.findDeckInPool(owner, id)
	if targetDeck == nil {
		sm.gauntletMu.Lock()
		sm.gauntlets[deckKey] = &GauntletResult{
			DeckKey: deckKey, Status: "error", Commander: id,
		}
		sm.gauntletMu.Unlock()
		return
	}

	result := &GauntletResult{
		DeckKey:   deckKey,
		Commander: targetDeck.CommanderName,
		Status:    "running",
		Target:    numGames,
		StartedAt: time.Now(),
	}

	sm.mu.RLock()
	if e, ok := sm.elo[deckKey]; ok {
		result.ELOStart = math.Round(e.rating*10) / 10
	}
	sm.mu.RUnlock()

	sm.gauntletMu.Lock()
	sm.gauntlets[deckKey] = result
	sm.gauntletMu.Unlock()

	log.Printf("gauntlet: starting %d games for %s (%s)", numGames, deckKey, targetDeck.CommanderName)

	beaten := map[string]int{}
	lostTo := map[string]int{}
	totalTurns := 0

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for g := 0; g < numGames; g++ {
		sm.mu.RLock()
		poolSize := len(sm.deckPool)
		sm.mu.RUnlock()
		if poolSize < showmatchSeats {
			break
		}

		pickedDecks := make([]*deckparser.TournamentDeck, showmatchSeats)
		commanders := make([]string, showmatchSeats)
		deckKeys := make([]string, showmatchSeats)

		pickedDecks[0] = targetDeck
		commanders[0] = targetDeck.CommanderName
		deckKeys[0] = deckKey

		sm.mu.RLock()
		perm := rng.Perm(poolSize)
		sm.mu.RUnlock()
		oi := 0
		for seat := 1; seat < showmatchSeats; seat++ {
			for oi < len(perm) {
				d := sm.deckPool[perm[oi]]
				oi++
				dk := deckKeyFromPath(d.Path)
				if dk != deckKey {
					pickedDecks[seat] = d
					commanders[seat] = d.CommanderName
					deckKeys[seat] = dk
					break
				}
			}
		}

		gameRng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(g)*37))
		gs := gameengine.NewGameState(showmatchSeats, gameRng, sm.corpus)
		gs.RetainEvents = false

		cmdDecks := make([]*gameengine.CommanderDeck, showmatchSeats)
		for i := 0; i < showmatchSeats; i++ {
			tpl := pickedDecks[i]
			lib := deckparser.CloneLibrary(tpl.Library)
			cmdrs := deckparser.CloneCards(tpl.CommanderCards)
			for _, c := range cmdrs {
				c.Owner = i
			}
			for _, c := range lib {
				c.Owner = i
			}
			gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
			cmdDecks[i] = &gameengine.CommanderDeck{
				CommanderCards: cmdrs,
				Library:        lib,
			}
		}

		gameengine.SetupCommanderGame(gs, cmdDecks)
		for i := 0; i < showmatchSeats; i++ {
			gs.Seats[i].Hat = hat.NewYggdrasilHatWithNoise(nil, 50, 0.2)
		}
		for i := 0; i < showmatchSeats; i++ {
			tournament.RunLondonMulligan(gs, i)
		}

		gs.Active = gameRng.Intn(showmatchSeats)
		gs.Turn = 1

		for turn := 1; turn <= showmatchMaxTurn; turn++ {
			gs.Turn = turn
			tournament.TakeTurn(gs)
			gameengine.StateBasedActions(gs)
			if gs.CheckEnd() {
				break
			}
			gs.Active = nextLiving(gs)
		}

		winner := -1
		if gs.Flags != nil && gs.Flags["ended"] == 1 {
			if w, ok := gs.Flags["winner"]; ok && w >= 0 && w < showmatchSeats {
				winner = w
			}
		}
		if winner < 0 {
			bestLife := -999
			for i, s := range gs.Seats {
				if s != nil && !s.Lost && s.Life > bestLife {
					bestLife = s.Life
					winner = i
				}
			}
		}

		totalTurns += gs.Turn

		sm.mu.Lock()
		sm.stats.gamesPlayed++
		sm.stats.totalTurns += gs.Turn
		sm.updateELO(deckKeys, commanders, pickedDecks, winner)
		sm.mu.Unlock()

		if winner == 0 {
			result.Wins++
			for s := 1; s < showmatchSeats; s++ {
				beaten[commanders[s]]++
			}
		} else {
			result.Losses++
			if winner >= 0 && winner < showmatchSeats {
				lostTo[commanders[winner]]++
			}
		}
		result.Games = g + 1

		if (g+1)%1000 == 0 {
			sm.gauntletMu.Lock()
			wr := 0.0
			if result.Games > 0 {
				wr = math.Round(float64(result.Wins)/float64(result.Games)*1000) / 10
			}
			result.WinRate = wr
			sm.gauntletMu.Unlock()
			log.Printf("gauntlet: %s — %d/%d games, %d wins (%.1f%%)", deckKey, g+1, numGames, result.Wins, wr)
		}
	}

	sm.mu.RLock()
	if e, ok := sm.elo[deckKey]; ok {
		result.ELOEnd = math.Round(e.rating*10) / 10
	}
	sm.mu.RUnlock()
	result.ELODelta = result.ELOEnd - result.ELOStart
	result.AvgTurns = math.Round(float64(totalTurns)/float64(max(result.Games, 1))*10) / 10
	result.WinRate = math.Round(float64(result.Wins)/float64(max(result.Games, 1))*1000) / 10
	result.Status = "complete"
	result.FinishedAt = time.Now()

	type ranked struct {
		name  string
		count int
	}
	topN := func(m map[string]int, n int) []string {
		var rs []ranked
		for k, v := range m {
			rs = append(rs, ranked{k, v})
		}
		sort.Slice(rs, func(i, j int) bool { return rs[i].count > rs[j].count })
		var out []string
		for i := 0; i < n && i < len(rs); i++ {
			out = append(out, fmt.Sprintf("%s (%d)", rs[i].name, rs[i].count))
		}
		return out
	}
	result.TopBeaten = topN(beaten, 5)
	result.TopLostTo = topN(lostTo, 5)

	sm.gauntletMu.Lock()
	sm.gauntlets[deckKey] = result
	sm.gauntletMu.Unlock()

	sm.flushELO()
	log.Printf("gauntlet: complete %s — %d games, %d wins (%.1f%%), ELO %+.0f (%0.f → %.0f)",
		deckKey, result.Games, result.Wins, result.WinRate, result.ELODelta, result.ELOStart, result.ELOEnd)
}

func (sm *Showmatch) GetGauntlet(deckKey string) *GauntletResult {
	sm.gauntletMu.RLock()
	defer sm.gauntletMu.RUnlock()
	if r, ok := sm.gauntlets[deckKey]; ok {
		cp := *r
		return &cp
	}
	return nil
}

func (sm *Showmatch) flushELO() {
	if sm.sqlDB == nil {
		return
	}
	sm.mu.RLock()
	eloCopy := make(map[string]*eloState, len(sm.elo))
	for k, v := range sm.elo {
		eCopy := *v
		eloCopy[k] = &eCopy
	}
	sm.mu.RUnlock()

	records := make([]db.ELORecord, 0, len(eloCopy))
	for key, e := range eloCopy {
		records = append(records, db.ELORecord{
			DeckKey:   key,
			Commander: e.commander,
			Owner:     e.owner,
			Rating:    e.rating,
			Games:     e.games,
			Wins:      e.wins,
			Losses:    e.games - e.wins,
			Delta:     e.delta,
		})
	}
	if err := db.BatchUpsertELO(context.Background(), sm.sqlDB, records); err != nil {
		log.Printf("showmatch: batch ELO flush: %v", err)
	}

	sm.mu.RLock()
	totalGames := sm.stats.historicGames + sm.stats.gamesPlayed
	totalTurns := sm.stats.historicTurns + sm.stats.totalTurns
	sm.mu.RUnlock()
	ctx := context.Background()
	db.KVSet(ctx, sm.sqlDB, "total_games", fmt.Sprintf("%d", totalGames))
	db.KVSet(ctx, sm.sqlDB, "total_turns", fmt.Sprintf("%d", totalTurns))

	sm.sqlDB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
}

func (sm *Showmatch) eloSize() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.elo)
}

func (sm *Showmatch) runOneGameFast(rng *rand.Rand) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("grinder: game crashed: %v", r)
		}
	}()

	sm.mu.RLock()
	poolSize := len(sm.deckPool)
	sm.mu.RUnlock()

	if poolSize < showmatchSeats {
		time.Sleep(5 * time.Second)
		return
	}

	indices := rng.Perm(poolSize)[:showmatchSeats]
	pickedDecks := make([]*deckparser.TournamentDeck, showmatchSeats)
	commanders := make([]string, showmatchSeats)
	deckKeys := make([]string, showmatchSeats)
	for i, idx := range indices {
		pickedDecks[i] = sm.deckPool[idx]
		commanders[i] = pickedDecks[i].CommanderName
		deckKeys[i] = deckKeyFromPath(pickedDecks[i].Path)
	}

	gameSeed := time.Now().UnixNano()
	gameRng := rand.New(rand.NewSource(gameSeed))
	gs := gameengine.NewGameState(showmatchSeats, gameRng, sm.corpus)
	gs.RetainEvents = false

	cmdDecks := make([]*gameengine.CommanderDeck, showmatchSeats)
	for i := 0; i < showmatchSeats; i++ {
		tpl := pickedDecks[i]
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		cmdDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: cmdrs,
			Library:        lib,
		}
	}

	gameengine.SetupCommanderGame(gs, cmdDecks)
	for i := 0; i < showmatchSeats; i++ {
		gs.Seats[i].Hat = hat.NewYggdrasilHatWithNoise(nil, 50, 0.2)
	}
	for i := 0; i < showmatchSeats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}

	gs.Active = gameRng.Intn(showmatchSeats)
	gs.Turn = 1

	for turn := 1; turn <= showmatchMaxTurn; turn++ {
		gs.Turn = turn
		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)

		if gs.CheckEnd() {
			break
		}
		gs.Active = nextLiving(gs)
	}

	winner := -1
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		if w, ok := gs.Flags["winner"]; ok && w >= 0 && w < showmatchSeats {
			winner = w
		}
	}
	if winner < 0 {
		bestLife := -999
		for i, s := range gs.Seats {
			if s != nil && !s.Lost && s.Life > bestLife {
				bestLife = s.Life
				winner = i
			}
		}
	}

	sm.mu.Lock()
	sm.stats.gamesPlayed++
	sm.stats.totalTurns += gs.Turn
	sm.updateELO(deckKeys, commanders, pickedDecks, winner)
	sm.mu.Unlock()
}

func (sm *Showmatch) runOneGame(rng *rand.Rand) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("showmatch: game crashed: %v", r)
		}
	}()

	sm.mu.RLock()
	poolSize := len(sm.deckPool)
	gameNum := sm.stats.historicGames + sm.stats.gamesPlayed + 1
	sm.mu.RUnlock()

	if poolSize < showmatchSeats {
		time.Sleep(5 * time.Second)
		return
	}

	// Pick 4 random decks.
	indices := rng.Perm(poolSize)[:showmatchSeats]
	pickedDecks := make([]*deckparser.TournamentDeck, showmatchSeats)
	commanders := make([]string, showmatchSeats)
	deckKeys := make([]string, showmatchSeats)
	for i, idx := range indices {
		pickedDecks[i] = sm.deckPool[idx]
		commanders[i] = pickedDecks[i].CommanderName
		deckKeys[i] = deckKeyFromPath(pickedDecks[i].Path)
	}

	gameSeed := time.Now().UnixNano()
	gameRng := rand.New(rand.NewSource(gameSeed))
	gs := gameengine.NewGameState(showmatchSeats, gameRng, sm.corpus)
	gs.RetainEvents = true

	cmdDecks := make([]*gameengine.CommanderDeck, showmatchSeats)
	for i := 0; i < showmatchSeats; i++ {
		tpl := pickedDecks[i]
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		cmdDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: cmdrs,
			Library:        lib,
		}
	}

	gameengine.SetupCommanderGame(gs, cmdDecks)

	// Attach Yggdrasil hats.
	for i := 0; i < showmatchSeats; i++ {
		gs.Seats[i].Hat = hat.NewYggdrasilHatWithNoise(nil, 50, 0.2)
	}

	// Mulligan.
	for i := 0; i < showmatchSeats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}

	gs.Active = gameRng.Intn(showmatchSeats)
	gs.Turn = 1
	gs.LogEvent(gameengine.Event{
		Kind: "game_start", Seat: gs.Active, Target: -1,
	})

	startedAt := time.Now()

	sm.mu.Lock()
	sm.eventLog = nil
	sm.mu.Unlock()

	snap := sm.captureSnapshot(gs, commanders, gameNum, startedAt)
	sm.mu.Lock()
	sm.snap = snap
	sm.mu.Unlock()

	lastEventIdx := len(gs.EventLog)

	// Phase hook: captures snapshot, broadcasts, and sleeps per-phase delay.
	phaseHook := func(hookGS *gameengine.GameState) {
		sm.mu.RLock()
		mult := sm.speedMultiplier
		sm.mu.RUnlock()
		if mult <= 0 {
			mult = 1.0
		}

		phSnap := sm.captureSnapshot(hookGS, commanders, gameNum, startedAt)
		sm.mu.Lock()
		phSnap.Log = make([]LogEntry, len(sm.eventLog))
		copy(phSnap.Log, sm.eventLog)
		sm.snap = phSnap
		sm.mu.Unlock()
		sm.broadcastToSpectators(wsEnvelope{Type: "game", Payload: phSnap})

		delay, ok := phaseDelays[hookGS.Step]
		if !ok && hookGS.Phase == "combat" {
			delay = combatPhaseDelay
		}
		if delay > 0 {
			time.Sleep(time.Duration(float64(delay) / mult))
		}
	}

	// Turn loop with per-phase pacing via hook.
	for turn := 1; turn <= showmatchMaxTurn; turn++ {
		gs.Turn = turn

		tournament.TakeTurnWithHook(gs, phaseHook)
		gameengine.StateBasedActions(gs)

		newEntries := sm.extractEvents(gs, lastEventIdx, commanders, turn)
		lastEventIdx = len(gs.EventLog)

		sm.mu.Lock()
		sm.eventLog = append(sm.eventLog, newEntries...)
		if len(sm.eventLog) > maxLogEntries {
			sm.eventLog = sm.eventLog[len(sm.eventLog)-maxLogEntries:]
		}
		sm.mu.Unlock()

		if len(gs.EventLog) > 500 {
			gs.EventLog = gs.EventLog[len(gs.EventLog)-200:]
			lastEventIdx = len(gs.EventLog)
		}

		snap = sm.captureSnapshot(gs, commanders, gameNum, startedAt)
		sm.mu.Lock()
		snap.Log = make([]LogEntry, len(sm.eventLog))
		copy(snap.Log, sm.eventLog)
		sm.snap = snap
		sm.mu.Unlock()

		sm.broadcastToSpectators(wsEnvelope{Type: "game", Payload: snap})

		if gs.CheckEnd() {
			break
		}

		gs.Active = nextLiving(gs)
	}

	// Determine winner.
	winner := -1
	endReason := "turn_cap"
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		if w, ok := gs.Flags["winner"]; ok && w >= 0 && w < showmatchSeats {
			winner = w
			endReason = "last_seat_standing"
		} else {
			endReason = "draw"
		}
	}
	if winner < 0 {
		// Highest life wins on turn cap.
		bestLife := -999
		for i, s := range gs.Seats {
			if s != nil && !s.Lost && s.Life > bestLife {
				bestLife = s.Life
				winner = i
			}
		}
		if winner >= 0 {
			endReason = "turn_cap_leader"
		}
	}

	// Final snapshot.
	finalSnap := sm.captureSnapshot(gs, commanders, gameNum, startedAt)
	finalSnap.Finished = true
	finalSnap.Winner = winner
	finalSnap.EndReason = endReason

	sm.mu.Lock()
	sm.snap = finalSnap
	sm.stats.gamesPlayed++
	sm.stats.totalTurns += gs.Turn
	sm.updateELO(deckKeys, commanders, pickedDecks, winner)

	completed := CompletedGame{
		GameID:     gameNum,
		Commanders: commanders,
		DeckKeys:   deckKeys,
		Winner:     winner,
		WinnerName: safeCommander(commanders, winner),
		Turns:      gs.Turn,
		EndReason:  endReason,
		FinishedAt: time.Now(),
		FinalSeats: finalSnap.Seats,
	}
	sm.gameHistory = append(sm.gameHistory, completed)
	if len(sm.gameHistory) > 50 {
		sm.gameHistory = sm.gameHistory[len(sm.gameHistory)-50:]
	}

	sm.mu.Unlock()

	select {
	case sm.persistCh <- persistJob{game: completed}:
	default:
	}

	log.Printf("showmatch: game %d finished — turn %d, winner: %s (%s)",
		gameNum, gs.Turn, safeCommander(commanders, winner), endReason)

	sm.broadcastToSpectators(wsEnvelope{Type: "game", Payload: finalSnap})
	sm.broadcastToSpectators(wsEnvelope{Type: "stats", Payload: sm.GetStats()})
	sm.broadcastToSpectators(wsEnvelope{Type: "elo", Payload: sm.GetELO()})
}

func (sm *Showmatch) persistGame(g CompletedGame) {
	if sm.sqlDB == nil {
		return
	}
	gameRec := db.GameRecord{
		StartedAt:  g.FinishedAt.Add(-time.Duration(g.Turns) * 3600 * time.Millisecond).Unix(),
		FinishedAt: g.FinishedAt.Unix(),
		Turns:      g.Turns,
		Winner:     g.Winner,
		WinnerName: g.WinnerName,
		EndReason:  g.EndReason,
	}
	seats := make([]db.GameSeatRecord, 0, len(g.FinalSeats))
	var cardStats []db.CardWinStat
	for i, seat := range g.FinalSeats {
		cmdr := ""
		if i < len(g.Commanders) {
			cmdr = g.Commanders[i]
		}
		dk := ""
		if i < len(g.DeckKeys) {
			dk = g.DeckKeys[i]
		}
		bfNames := make([]string, 0, len(seat.Battlefield))
		for _, p := range seat.Battlefield {
			bfNames = append(bfNames, p.Name)
		}
		bfJSON, _ := json.Marshal(bfNames)
		seats = append(seats, db.GameSeatRecord{
			Seat:             i,
			Commander:        cmdr,
			DeckKey:          dk,
			Life:             seat.Life,
			HandSize:         seat.HandSize,
			LibrarySize:      seat.LibrarySize,
			GYSize:           seat.GYSize,
			BFSize:           len(seat.Battlefield),
			Lost:             seat.Lost,
			BattlefieldCards: string(bfJSON),
		})
		isWinner := i == g.Winner
		bfSet := map[string]bool{}
		for _, name := range bfNames {
			bfSet[name] = true
		}
		for name := range bfSet {
			win := 0
			onBoard := 0
			if isWinner {
				win = 1
				onBoard = 1
			}
			cardStats = append(cardStats, db.CardWinStat{
				CardName:     name,
				Commander:    cmdr,
				Wins:         win,
				OnBoardAtWin: onBoard,
			})
		}
	}
	if _, err := db.PersistGameTx(context.Background(), sm.sqlDB, gameRec, seats); err != nil {
		log.Printf("showmatch: persist game: %v", err)
	}
	if len(cardStats) > 0 {
		if err := db.BatchUpsertCardWinStats(context.Background(), sm.sqlDB, cardStats); err != nil {
			log.Printf("showmatch: card win stats: %v", err)
		}
	}
}

func safeCommander(commanders []string, idx int) string {
	if idx < 0 || idx >= len(commanders) {
		return "DRAW"
	}
	return commanders[idx]
}

func (sm *Showmatch) captureSnapshot(gs *gameengine.GameState, commanders []string, gameNum int, startedAt time.Time) *GameSnapshot {
	snap := &GameSnapshot{
		GameID:     gameNum,
		Turn:       gs.Turn,
		Phase:      gs.Phase,
		Step:       gs.Step,
		ActiveSeat: gs.Active,
		Seats:      make([]SeatSnapshot, len(gs.Seats)),
		StartedAt:  startedAt,
		Winner:     -1,
	}
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		cmdr := ""
		if i < len(commanders) {
			cmdr = commanders[i]
		}
		ss := SeatSnapshot{
			Commander:   cmdr,
			Life:        s.Life,
			HandSize:    len(s.Hand),
			LibrarySize: len(s.Library),
			GYSize:      len(s.Graveyard),
			ManaPool:    s.ManaPool,
			Lost:        s.Lost,
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			ps := PermanentSnapshot{
				Name:   p.Card.DisplayName(),
				Tapped: p.Tapped,
				Power:  p.Power(),
				Tough:  p.Toughness(),
				IsLand: p.IsLand(),
			}
			// Check if this is a commander.
			for _, cn := range s.CommanderNames {
				if cn == p.Card.DisplayName() {
					ps.IsCmdr = true
					break
				}
			}
			// Determine primary type.
			if p.IsCreature() {
				ps.Type = "CREATURE"
			} else if p.IsLand() {
				ps.Type = "LAND"
			} else if p.IsArtifact() {
				ps.Type = "ARTIFACT"
			} else if p.IsEnchantment() {
				ps.Type = "ENCHANTMENT"
			} else if p.IsPlaneswalker() {
				ps.Type = "PLANESWALKER"
			}
			ss.Battlefield = append(ss.Battlefield, ps)
		}
		if ygg, ok := s.Hat.(*hat.YggdrasilHat); ok && ygg.Evaluator != nil {
			r := ygg.Evaluator.EvaluateDetailed(gs, i)
			ss.Eval = &EvalSnapshot{
				Score:             r.Score,
				BoardPresence:     r.BoardPresence,
				CardAdvantage:     r.CardAdvantage,
				ManaAdvantage:     r.ManaAdvantage,
				LifeResource:      r.LifeResource,
				ComboProximity:    r.ComboProximity,
				ThreatExposure:    r.ThreatExposure,
				CommanderProgress: r.CommanderProgress,
				GraveyardValue:    r.GraveyardValue,
			}
		}
		snap.Seats[i] = ss
	}
	return snap
}

func (sm *Showmatch) updateELO(deckKeys, commanders []string, decks []*deckparser.TournamentDeck, winner int) {
	const k = 32.0
	n := len(deckKeys)
	if n < 2 {
		return
	}
	kScaled := k / float64(n-1)

	for i, key := range deckKeys {
		if _, ok := sm.elo[key]; !ok {
			owner, _ := deckOwnerFromKey(key)
			sm.elo[key] = &eloState{rating: 1500, commander: commanders[i], owner: owner}
		}
		sm.elo[key].games++
	}

	winnerKey := ""
	if winner >= 0 && winner < n {
		winnerKey = deckKeys[winner]
		sm.elo[winnerKey].wins++
	}

	if winnerKey == "" {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				a, b := deckKeys[i], deckKeys[j]
				ea := 1.0 / (1.0 + math.Pow(10, (sm.elo[b].rating-sm.elo[a].rating)/400.0))
				eb := 1.0 - ea
				sm.elo[a].delta = kScaled * (0.5 - ea)
				sm.elo[b].delta = kScaled * (0.5 - eb)
				sm.elo[a].rating += sm.elo[a].delta
				sm.elo[b].rating += sm.elo[b].delta
			}
		}
		return
	}

	for _, loserKey := range deckKeys {
		if loserKey == winnerKey {
			continue
		}
		eW := 1.0 / (1.0 + math.Pow(10, (sm.elo[loserKey].rating-sm.elo[winnerKey].rating)/400.0))
		eL := 1.0 - eW
		wDelta := kScaled * (1.0 - eW)
		lDelta := kScaled * (0.0 - eL)
		sm.elo[winnerKey].delta = wDelta
		sm.elo[loserKey].delta = lDelta
		sm.elo[winnerKey].rating += wDelta
		sm.elo[loserKey].rating += lDelta
	}
}

func (sm *Showmatch) GetSnapshot() *GameSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.snap
}

func (sm *Showmatch) GetELO() []ELOEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	entries := make([]ELOEntry, 0, len(sm.elo))
	for key, e := range sm.elo {
		losses := e.games - e.wins
		winRate := 0.0
		if e.games > 0 {
			winRate = math.Round(float64(e.wins)/float64(e.games)*1000) / 10
		}
		owner, deckID := deckOwnerFromKey(key)
		entries = append(entries, ELOEntry{
			DeckID:    deckID,
			Commander: e.commander,
			Owner:     owner,
			Rating:    math.Round(e.rating*10) / 10,
			Games:     e.games,
			Wins:      e.wins,
			Losses:    losses,
			WinRate:   winRate,
			Delta:     math.Round(e.delta*10) / 10,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Rating != entries[j].Rating {
			return entries[i].Rating > entries[j].Rating
		}
		if entries[i].Games != entries[j].Games {
			return entries[i].Games > entries[j].Games
		}
		return entries[i].Commander < entries[j].Commander
	})
	return entries
}

func (sm *Showmatch) GetStats() SessionStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	uptime := time.Since(sm.start)
	totalGames := sm.stats.historicGames + sm.stats.gamesPlayed
	totalTurns := sm.stats.historicTurns + sm.stats.totalTurns
	avgTurns := 0.0
	if totalGames > 0 {
		avgTurns = float64(totalTurns) / float64(totalGames)
	}
	gpm := 0.0
	if uptime.Minutes() > 0 {
		gpm = float64(sm.stats.gamesPlayed) / uptime.Minutes()
	}

	dominant := ""
	domWR := 0.0
	domGames := 0
	domRating := 0.0
	domKey := ""
	for key, e := range sm.elo {
		if e.games > 0 {
			wr := float64(e.wins) / float64(e.games)
			better := wr > domWR ||
				(wr == domWR && e.games > domGames) ||
				(wr == domWR && e.games == domGames && e.rating > domRating) ||
				(wr == domWR && e.games == domGames && e.rating == domRating && key < domKey)
			if better {
				domWR = wr
				dominant = e.commander
				domGames = e.games
				domRating = e.rating
				domKey = key
			}
		}
	}
	if dominant == "" && domKey != "" {
		dominant = domKey
	}

	status := "running"
	if sm.snap != nil && sm.snap.Finished {
		status = "between_games"
	}

	return SessionStats{
		GamesPlayed: totalGames,
		AvgTurns:    math.Round(avgTurns*10) / 10,
		Dominant:    dominant,
		DomWinRate:  math.Round(domWR*1000) / 10,
		Uptime:      formatDuration(uptime),
		Status:      status,
		GamesPerMin: math.Round(gpm*10) / 10,
	}
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dH %dM", h, m)
}

func (sm *Showmatch) GetGameHistory(limit int) []CompletedGame {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	n := len(sm.gameHistory)
	if limit <= 0 || limit > n {
		limit = n
	}
	result := make([]CompletedGame, limit)
	for i := 0; i < limit; i++ {
		result[i] = sm.gameHistory[n-1-i]
	}
	return result
}

func (sm *Showmatch) GetGame(id int) *CompletedGame {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for i := range sm.gameHistory {
		if sm.gameHistory[i].GameID == id {
			g := sm.gameHistory[i]
			return &g
		}
	}
	return nil
}

// RegisterShowmatch adds live game endpoints to the mux.
func (sm *Showmatch) RegisterShowmatch(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/live/game", sm.handleLiveGame)
	mux.HandleFunc("GET /api/live/elo", sm.handleLiveELO)
	mux.HandleFunc("GET /api/live/stats", sm.handleLiveStatsReal)
	mux.HandleFunc("GET /api/games", sm.handleGames)
	mux.HandleFunc("GET /api/games/{id}", sm.handleGameByID)
	mux.HandleFunc("GET /api/live/speed", sm.handleGetSpeed)
	mux.HandleFunc("POST /api/live/speed", sm.handleSetSpeed)
	mux.HandleFunc("GET /ws/live", sm.handleSpectatorWS)
	mux.HandleFunc("POST /api/gauntlet/{owner}/{id}", sm.handleStartGauntlet)
	mux.HandleFunc("GET /api/gauntlet/{owner}/{id}", sm.handleGetGauntlet)
}

var gauntletSem = make(chan struct{}, 2)

func (sm *Showmatch) handleStartGauntlet(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	deckKey := owner + "/" + id

	sm.gauntletMu.RLock()
	existing := sm.gauntlets[deckKey]
	sm.gauntletMu.RUnlock()
	if existing != nil && existing.Status == "running" {
		writeJSON(w, existing)
		return
	}

	select {
	case gauntletSem <- struct{}{}:
	default:
		http.Error(w, "too many gauntlets running — try again later", http.StatusTooManyRequests)
		return
	}

	numGames := 10000
	if n := parseInt(r.URL.Query().Get("games")); n > 0 && n <= 50000 {
		numGames = n
	}

	go func() {
		defer func() { <-gauntletSem }()
		sm.RunGauntlet(owner, id, numGames)
	}()
	writeJSON(w, map[string]any{
		"status":   "started",
		"deck_key": deckKey,
		"games":    numGames,
	})
}

func (sm *Showmatch) handleGetGauntlet(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	deckKey := owner + "/" + id

	result := sm.GetGauntlet(deckKey)
	if result == nil {
		writeJSON(w, map[string]any{"status": "none"})
		return
	}
	writeJSON(w, result)
}

func (sm *Showmatch) handleLiveGame(w http.ResponseWriter, r *http.Request) {
	sm.mu.RLock()
	ready := sm.ready
	loadErr := sm.loadErr
	sm.mu.RUnlock()

	if !ready {
		msg := "loading AST corpus and decks..."
		if loadErr != "" {
			msg = "load error: " + loadErr
		}
		writeJSON(w, map[string]any{"status": "starting", "message": msg})
		return
	}

	snap := sm.GetSnapshot()
	if snap == nil {
		writeJSON(w, map[string]any{"status": "starting", "message": "first game loading..."})
		return
	}
	writeJSON(w, snap)
}

func (sm *Showmatch) handleLiveELO(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, sm.GetELO())
}

func (sm *Showmatch) handleLiveStatsReal(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, sm.GetStats())
}

func (sm *Showmatch) handleGames(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if n := parseInt(limitStr); n > 0 {
		limit = n
	}
	games := sm.GetGameHistory(limit)
	writeJSON(w, games)
}

func (sm *Showmatch) handleGameByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id := parseInt(idStr)
	if id <= 0 {
		http.Error(w, "invalid game id", http.StatusBadRequest)
		return
	}
	game := sm.GetGame(id)
	if game == nil {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}
	writeJSON(w, game)
}

func (sm *Showmatch) handleGetSpeed(w http.ResponseWriter, r *http.Request) {
	sm.mu.RLock()
	mult := sm.speedMultiplier
	sm.mu.RUnlock()
	writeJSON(w, map[string]any{"multiplier": mult})
}

func (sm *Showmatch) handleSetSpeed(w http.ResponseWriter, r *http.Request) {
	multStr := r.URL.Query().Get("multiplier")
	mult := 1.0
	if v := parseFloat(multStr); v > 0 {
		mult = v
	}
	if mult < 0.2 {
		mult = 0.2
	}
	if mult > 200.0 {
		mult = 200.0
	}
	sm.mu.Lock()
	sm.speedMultiplier = mult
	sm.mu.Unlock()
	log.Printf("showmatch: speed multiplier set to %.1fx (total_phase_delay=%.0fms)", mult, 3600.0/mult)
	sm.broadcastToSpectators(wsEnvelope{Type: "speed", Payload: map[string]any{"multiplier": mult}})
	writeJSON(w, map[string]any{"multiplier": mult})
}

func parseFloat(s string) float64 {
	var result float64
	var decimal float64
	var inDecimal bool
	for _, c := range s {
		if c == '.' {
			inDecimal = true
			decimal = 0.1
			continue
		}
		if c < '0' || c > '9' {
			return 0
		}
		if inDecimal {
			result += float64(c-'0') * decimal
			decimal *= 0.1
		} else {
			result = result*10 + float64(c-'0')
		}
	}
	return result
}

func (sm *Showmatch) extractEvents(gs *gameengine.GameState, fromIdx int, commanders []string, turn int) []LogEntry {
	if fromIdx >= len(gs.EventLog) {
		return nil
	}
	var entries []LogEntry
	for i := fromIdx; i < len(gs.EventLog); i++ {
		ev := gs.EventLog[i]
		entry, ok := formatEvent(ev, commanders, turn)
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func formatEvent(ev gameengine.Event, commanders []string, turn int) (LogEntry, bool) {
	seat := ev.Seat
	cmdr := "SYSTEM"
	if seat >= 0 && seat < len(commanders) {
		parts := strings.Split(commanders[seat], ",")
		cmdr = strings.TrimSpace(parts[0])
		if len(cmdr) > 20 {
			cmdr = cmdr[:20]
		}
		cmdr = strings.ToUpper(cmdr)
	}

	switch ev.Kind {
	case "cast":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " CASTS " + strings.ToUpper(ev.Source), Kind: "cast"}, true
	case "play_land":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " PLAYS LAND: " + strings.ToUpper(ev.Source), Kind: "land"}, true
	case "declare_attackers":
		n := ev.Amount
		if n <= 0 {
			n = 1
		}
		target := ""
		if ev.Target >= 0 && ev.Target < len(commanders) {
			target = " → " + strings.ToUpper(strings.Split(commanders[ev.Target], ",")[0])
		}
		return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s ATTACKS WITH %d CREATURE(S)%s", cmdr, n, target), Kind: "combat"}, true
	case "damage":
		target := ""
		if ev.Target >= 0 && ev.Target < len(commanders) {
			target = strings.ToUpper(strings.Split(commanders[ev.Target], ",")[0])
		}
		if ev.Amount > 0 && target != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s DEALS %d DAMAGE TO %s", cmdr, ev.Amount, target), Detail: ev.Source, Kind: "damage"}, true
		}
	case "counter_spell":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " COUNTERS " + strings.ToUpper(ev.Source), Kind: "counter"}, true
	case "create_token":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " CREATES TOKEN: " + strings.ToUpper(ev.Source), Kind: "token"}, true
	case "destroy":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " DESTROYS " + strings.ToUpper(ev.Source), Kind: "removal"}, true
	case "sacrifice":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " SACRIFICES " + strings.ToUpper(ev.Source), Kind: "removal"}, true
	case "gain_life":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s GAINS %d LIFE", cmdr, ev.Amount), Kind: "life"}, true
		}
	case "lose_life":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s LOSES %d LIFE", cmdr, ev.Amount), Kind: "life"}, true
		}
	case "draw":
		if ev.Amount > 1 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s DRAWS %d CARDS", cmdr, ev.Amount), Kind: "draw"}, true
		}
	case "seat_eliminated", "lose_game":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " IS ELIMINATED", Kind: "elimination"}, true
	case "enter_battlefield":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " → ETB: " + strings.ToUpper(ev.Source), Kind: "etb"}, true
		}
	case "triggered_ability", "triggered":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " TRIGGERS " + strings.ToUpper(ev.Source), Kind: "trigger"}, true
		}
	case "activate_ability":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " ACTIVATES " + strings.ToUpper(ev.Source), Kind: "activate"}, true
		}
	case "exile":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " EXILES " + strings.ToUpper(ev.Source), Kind: "removal"}, true
		}
	case "mill":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s MILLS %d CARDS", cmdr, ev.Amount), Kind: "mill"}, true
		}
	case "search_library", "search":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " SEARCHES LIBRARY", Kind: "search"}, true
	case "reanimate":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " REANIMATES " + strings.ToUpper(ev.Source), Kind: "reanimate"}, true
	case "extra_turn":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " TAKES AN EXTRA TURN", Kind: "extra_turn"}, true
	case "commander_cast_from_command_zone":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " CASTS COMMANDER FROM COMMAND ZONE", Kind: "cast"}, true
	}
	return LogEntry{}, false
}

func nextLiving(gs *gameengine.GameState) int {
	n := len(gs.Seats)
	for k := 1; k <= n; k++ {
		cand := (gs.Active + k) % n
		s := gs.Seats[cand]
		if s != nil && !s.Lost {
			return cand
		}
	}
	return gs.Active
}

func deckKeyFromPath(path string) string {
	dir, file := filepath.Split(path)
	ext := filepath.Ext(file)
	name := strings.TrimSuffix(file, ext)
	owner := filepath.Base(filepath.Clean(dir))
	return owner + "/" + name
}

func deckOwnerFromKey(key string) (owner, deckID string) {
	idx := strings.IndexByte(key, '/')
	if idx < 0 {
		return "", key
	}
	return key[:idx], key[idx+1:]
}

func findDeckFiles(dir string) ([]string, error) {
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "freya" || base == "benched" || base == "test" || base == "moxfield_300" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// ---------------------------------------------------------------------------
// Spectator WebSocket — unauthenticated live data feed
// ---------------------------------------------------------------------------

type wsEnvelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

const maxSpectators = 100

func (sm *Showmatch) handleSpectatorWS(w http.ResponseWriter, r *http.Request) {
	sm.specMu.RLock()
	count := len(sm.spectators)
	sm.specMu.RUnlock()
	if count >= maxSpectators {
		http.Error(w, "too many spectators", http.StatusServiceUnavailable)
		return
	}

	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("spectator ws: upgrade error: %v", err)
		return
	}
	wsConn.SetReadLimit(512)

	sc := &spectatorConn{conn: wsConn}

	sm.specMu.Lock()
	sm.spectators[sc] = struct{}{}
	count = len(sm.spectators)
	sm.specMu.Unlock()
	log.Printf("spectator ws: connected (%d total)", count)

	sm.sendFullState(sc)

	ctx := r.Context()
	for {
		_, data, err := wsConn.Read(ctx)
		if err != nil {
			break
		}
		var env struct{ Type string `json:"type"` }
		if json.Unmarshal(data, &env) == nil && env.Type == "ping" {
			sc.send(wsEnvelope{Type: "pong", Payload: map[string]int64{"server_time": time.Now().Unix()}})
		}
	}

	sm.specMu.Lock()
	delete(sm.spectators, sc)
	count = len(sm.spectators)
	sm.specMu.Unlock()
	log.Printf("spectator ws: disconnected (%d total)", count)
	wsConn.CloseNow()
}

func (sc *spectatorConn) send(env wsEnvelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return sc.conn.Write(ctx, websocket.MessageText, data)
}

func (sm *Showmatch) sendFullState(sc *spectatorConn) {
	snap := sm.GetSnapshot()
	if snap != nil {
		sc.send(wsEnvelope{Type: "game", Payload: snap})
	}
	sc.send(wsEnvelope{Type: "elo", Payload: sm.GetELO()})
	sc.send(wsEnvelope{Type: "stats", Payload: sm.GetStats()})

	sm.mu.RLock()
	history := make([]CompletedGame, len(sm.gameHistory))
	copy(history, sm.gameHistory)
	mult := sm.speedMultiplier
	sm.mu.RUnlock()
	sc.send(wsEnvelope{Type: "history", Payload: history})
	sc.send(wsEnvelope{Type: "speed", Payload: map[string]any{"multiplier": mult}})
}

func (sm *Showmatch) broadcastToSpectators(env wsEnvelope) {
	sm.specMu.RLock()
	conns := make([]*spectatorConn, 0, len(sm.spectators))
	for sc := range sm.spectators {
		conns = append(conns, sc)
	}
	sm.specMu.RUnlock()

	if len(conns) == 0 {
		return
	}

	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	for _, sc := range conns {
		sc.mu.Lock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		writeErr := sc.conn.Write(ctx, websocket.MessageText, data)
		cancel()
		sc.mu.Unlock()
		if writeErr != nil {
			sm.specMu.Lock()
			delete(sm.spectators, sc)
			sm.specMu.Unlock()
			sc.conn.CloseNow()
		}
	}
}
