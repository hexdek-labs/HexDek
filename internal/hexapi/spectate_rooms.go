package hexapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/hexdek/hexdek/internal/cardstats"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/heimdall"
	"github.com/hexdek/hexdek/internal/matchmaking"
	"github.com/hexdek/hexdek/internal/tournament"
)

const (
	maxSpectateRooms    = 50
	roomIdleTimeout     = 5 * time.Minute
	spectateRoomMaxTurn = 80
)

type SpectateRoom struct {
	ID        string    `json:"id"`
	DeckKey   string    `json:"deck_key"`
	Commander string    `json:"commander"`
	CreatedAt time.Time `json:"created_at"`

	sm  *Showmatch
	rng *rand.Rand

	mu              sync.RWMutex
	snap            *GameSnapshot
	eventLog        []LogEntry
	speedMultiplier float64
	gameNumber      int
	running         bool
	stopCh          chan struct{}

	specMu     sync.RWMutex
	spectators map[*spectatorConn]struct{}

	idleTimer *time.Timer
}

type RoomManager struct {
	mu     sync.RWMutex
	rooms  map[string]*SpectateRoom // room_id → room
	byDeck map[string]string        // deck_key → room_id (for reuse)
}

func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:  make(map[string]*SpectateRoom),
		byDeck: make(map[string]string),
	}
}

type SpectateRoomInfo struct {
	ID        string  `json:"id"`
	DeckKey   string  `json:"deck_key"`
	Commander string  `json:"commander"`
	Viewers   int     `json:"viewers"`
	Game      int     `json:"game_number"`
	Speed     float64 `json:"speed"`
}

func (rm *RoomManager) ListRooms() []SpectateRoomInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	out := make([]SpectateRoomInfo, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		r.specMu.RLock()
		viewers := len(r.spectators)
		r.specMu.RUnlock()
		r.mu.RLock()
		out = append(out, SpectateRoomInfo{
			ID:        r.ID,
			DeckKey:   r.DeckKey,
			Commander: r.Commander,
			Viewers:   viewers,
			Game:      r.gameNumber,
			Speed:     r.speedMultiplier,
		})
		r.mu.RUnlock()
	}
	return out
}

func (rm *RoomManager) SpawnOrReuse(sm *Showmatch, deckKey string) (*SpectateRoom, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rid, ok := rm.byDeck[deckKey]; ok {
		if room, ok2 := rm.rooms[rid]; ok2 {
			room.resetIdleTimer()
			return room, nil
		}
		delete(rm.byDeck, deckKey)
	}

	if len(rm.rooms) >= maxSpectateRooms {
		return nil, fmt.Errorf("max %d spectate rooms reached", maxSpectateRooms)
	}

	deck := sm.findDeckInPool2(deckKey)
	if deck == nil {
		return nil, fmt.Errorf("deck %q not in pool", deckKey)
	}

	roomID := fmt.Sprintf("sr-%s-%d", shortHash(deckKey), time.Now().UnixMilli()%100000)

	room := &SpectateRoom{
		ID:              roomID,
		DeckKey:         deckKey,
		Commander:       deck.CommanderName,
		CreatedAt:       time.Now(),
		sm:              sm,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
		speedMultiplier: 1.0,
		running:         true,
		stopCh:          make(chan struct{}),
		spectators:      make(map[*spectatorConn]struct{}),
	}
	room.idleTimer = time.AfterFunc(roomIdleTimeout, func() {
		room.mu.RLock()
		running := room.running
		room.mu.RUnlock()
		if running {
			room.specMu.RLock()
			n := len(room.spectators)
			room.specMu.RUnlock()
			if n == 0 {
				rm.TeardownRoom(roomID)
			}
		}
	})

	rm.rooms[roomID] = room
	rm.byDeck[deckKey] = roomID

	go room.gameLoop()
	log.Printf("spectate-room: spawned %s for %s (%s)", roomID, deckKey, deck.CommanderName)
	return room, nil
}

func (rm *RoomManager) GetRoom(roomID string) *SpectateRoom {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.rooms[roomID]
}

func (rm *RoomManager) TeardownRoom(roomID string) {
	rm.mu.Lock()
	room, ok := rm.rooms[roomID]
	if !ok {
		rm.mu.Unlock()
		return
	}
	delete(rm.rooms, roomID)
	delete(rm.byDeck, room.DeckKey)
	rm.mu.Unlock()

	room.mu.Lock()
	if room.running {
		room.running = false
		close(room.stopCh)
	}
	room.mu.Unlock()
	if room.idleTimer != nil {
		room.idleTimer.Stop()
	}

	room.specMu.Lock()
	for sc := range room.spectators {
		sc.conn.Close(websocket.StatusGoingAway, "room closed")
	}
	room.spectators = make(map[*spectatorConn]struct{})
	room.specMu.Unlock()

	log.Printf("spectate-room: teardown %s (%s)", roomID, room.DeckKey)
}

func (room *SpectateRoom) resetIdleTimer() {
	if room.idleTimer != nil {
		room.idleTimer.Reset(roomIdleTimeout)
	}
}

func (room *SpectateRoom) viewerCount() int {
	room.specMu.RLock()
	defer room.specMu.RUnlock()
	return len(room.spectators)
}

func (room *SpectateRoom) gameLoop() {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			log.Printf("spectate-room %s: panic: %v\n%s", room.ID, r, buf[:n])
		}
	}()

	for {
		select {
		case <-room.stopCh:
			return
		default:
		}
		room.runOneSpectateGame()

		room.mu.RLock()
		mult := room.speedMultiplier
		room.mu.RUnlock()
		if mult <= 0 {
			mult = 1.0
		}
		time.Sleep(time.Duration(float64(500*time.Millisecond) / mult))
	}
}

func (room *SpectateRoom) runOneSpectateGame() {
	sm := room.sm

	sm.mu.RLock()
	if !sm.ready {
		sm.mu.RUnlock()
		time.Sleep(2 * time.Second)
		return
	}
	poolSize := len(sm.deckPool)
	sm.mu.RUnlock()

	if poolSize < showmatchSeats {
		time.Sleep(2 * time.Second)
		return
	}

	targetDeck := sm.findDeckInPool2(room.DeckKey)
	if targetDeck == nil {
		time.Sleep(5 * time.Second)
		return
	}

	targetIdx := -1
	sm.mu.RLock()
	for i, d := range sm.deckPool {
		if deckKeyFromPath(d.Path) == room.DeckKey {
			targetIdx = i
			break
		}
	}
	sm.mu.RUnlock()
	if targetIdx < 0 {
		return
	}

	// Build matchmaking pool excluding target.
	sm.mu.RLock()
	pool := make([]matchmaking.DeckEntry, 0, poolSize-1)
	for i := 0; i < poolSize; i++ {
		if i == targetIdx {
			continue
		}
		dk := deckKeyFromPath(sm.deckPool[i].Path)
		mu := 25.0
		sigma := 8.33
		games := 0
		if e, ok := sm.elo[dk]; ok {
			mu = e.rating / 40.0 * 25.0
			games = e.games
			sigma = 8.33 / math.Max(1.0, math.Sqrt(float64(games)))
		}
		bracket := 0
		if e, ok := sm.elo[dk]; ok {
			bracket = e.bracket
		}
		pool = append(pool, matchmaking.DeckEntry{
			Index: i, Commander: sm.deckPool[i].CommanderName,
			Mu: mu, Sigma: sigma, Games: games, Bracket: bracket,
		})
	}
	sm.mu.RUnlock()

	oppIndices := matchmaking.AssembleBracketPod(room.rng, pool, showmatchSeats-1)

	pickedDecks := make([]*deckparser.TournamentDeck, showmatchSeats)
	commanders := make([]string, showmatchSeats)
	deckKeys := make([]string, showmatchSeats)

	// Seat 0 = spectated deck.
	pickedDecks[0] = targetDeck
	commanders[0] = targetDeck.CommanderName
	deckKeys[0] = room.DeckKey

	for i, idx := range oppIndices {
		if i+1 >= showmatchSeats {
			break
		}
		pickedDecks[i+1] = sm.deckPool[idx]
		commanders[i+1] = pickedDecks[i+1].CommanderName
		deckKeys[i+1] = deckKeyFromPath(pickedDecks[i+1].Path)
	}

	for i, d := range pickedDecks {
		if d == nil {
			sm.mu.RLock()
			pickedDecks[i] = sm.deckPool[room.rng.Intn(poolSize)]
			commanders[i] = pickedDecks[i].CommanderName
			deckKeys[i] = deckKeyFromPath(pickedDecks[i].Path)
			sm.mu.RUnlock()
		}
	}

	room.mu.Lock()
	room.gameNumber++
	gameNum := room.gameNumber
	room.eventLog = nil
	room.mu.Unlock()

	gameSeed := time.Now().UnixNano()
	gameRng := rand.New(rand.NewSource(gameSeed))

	sm.mu.RLock()
	corpus := sm.corpus
	sm.mu.RUnlock()

	gs := gameengine.NewGameState(showmatchSeats, gameRng, corpus)
	gs.Seed = gameSeed
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

	// Attach Yggdrasil hats with Curse DNA.
	var dnaIdxes [showmatchSeats]int
	var roomHats [showmatchSeats]*hat.YggdrasilHat
	for i := 0; i < showmatchSeats; i++ {
		pool := sm.getOrCreateCursePool(deckKeys[i], room.rng)
		sm.curseMu.Lock()
		dna, dnaIdx := pool.SelectForGame()
		dnaCopy := *dna
		dimStats := pool.DimStats
		sm.curseMu.Unlock()
		dnaIdxes[i] = dnaIdx
		h := hat.NewYggdrasilHatWithPool(&dnaCopy, sm.strategies[deckKeys[i]], 50, &dimStats)
		h.NeuralEval = sm.neuralEval
		gs.Seats[i].Hat = h
		roomHats[i] = h
	}

	for i := 0; i < showmatchSeats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}

	gs.Active = gameRng.Intn(showmatchSeats)
	gs.Turn = 1
	gs.LogEvent(gameengine.Event{Kind: "game_start", Seat: gs.Active, Target: -1})

	startedAt := time.Now()

	snap := sm.captureSnapshot(gs, commanders, gameNum, startedAt)
	room.mu.Lock()
	room.snap = snap
	room.mu.Unlock()

	lastEventIdx := len(gs.EventLog)

	var roomCollectors [showmatchSeats]*hat.TrainingCollector
	for i := 0; i < showmatchSeats; i++ {
		roomCollectors[i] = hat.NewTrainingCollector(5)
	}
	roomEvalCollector := hat.NewEvalSnapshotCollector()
	roomETBs := make(map[int][]string)

	phaseHook := func(hookGS *gameengine.GameState) {
		select {
		case <-room.stopCh:
			return
		default:
		}

		room.mu.RLock()
		mult := room.speedMultiplier
		room.mu.RUnlock()
		if mult <= 0 {
			mult = 1.0
		}

		phSnap := sm.captureSnapshot(hookGS, commanders, gameNum, startedAt)
		room.mu.Lock()
		phSnap.Log = make([]LogEntry, len(room.eventLog))
		copy(phSnap.Log, room.eventLog)
		room.snap = phSnap
		room.mu.Unlock()
		room.broadcast(wsEnvelope{Type: "game", Payload: phSnap})

		delay, ok := phaseDelays[hookGS.Step]
		if !ok && hookGS.Phase == "combat" {
			delay = combatPhaseDelay
		}
		if delay > 0 {
			time.Sleep(time.Duration(float64(delay) / mult))
		}
	}

	for turn := 1; turn <= spectateRoomMaxTurn; turn++ {
		select {
		case <-room.stopCh:
			return
		default:
		}

		gs.Turn = turn
		preBF := heimdall.SnapshotBattlefieldNames(gs)
		tournament.TakeTurnWithHook(gs, phaseHook)
		gameengine.StateBasedActions(gs)
		if newCards := heimdall.DiffBattlefield(preBF, heimdall.SnapshotBattlefieldNames(gs)); len(newCards) > 0 {
			roomETBs[turn] = newCards
		}
		for i := 0; i < showmatchSeats; i++ {
			roomCollectors[i].Snapshot(gs, i)
		}
		if turn%5 == 0 || turn == 1 {
			roomEvalCollector.Record(turn, extractEvalScores(gs))
		}

		newEntries := sm.extractEvents(gs, lastEventIdx, commanders, turn)
		lastEventIdx = len(gs.EventLog)

		room.mu.Lock()
		room.eventLog = append(room.eventLog, newEntries...)
		if len(room.eventLog) > maxLogEntries {
			room.eventLog = room.eventLog[len(room.eventLog)-maxLogEntries:]
		}
		room.mu.Unlock()

		if len(gs.EventLog) > 500 {
			gs.EventLog = gs.EventLog[len(gs.EventLog)-200:]
			lastEventIdx = len(gs.EventLog)
		}

		snap = sm.captureSnapshot(gs, commanders, gameNum, startedAt)
		room.mu.Lock()
		snap.Log = make([]LogEntry, len(room.eventLog))
		copy(snap.Log, room.eventLog)
		room.snap = snap
		room.mu.Unlock()
		room.broadcast(wsEnvelope{Type: "game", Payload: snap})

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

	// Curse: record results.
	sm.curseMu.Lock()
	var roomEvolved []*hat.CursePool
	for i := 0; i < showmatchSeats; i++ {
		if cursePool, ok := sm.cursePool[deckKeys[i]]; ok {
			score := hat.PlacementScore(seatPlacement(gs, i, winner), showmatchSeats)
			prevCount := cursePool.GameCount
			cursePool.RecordResult(dnaIdxes[i], score)
			if roomHats[i] != nil {
				cursePool.DimStats.RecordGame(roomHats[i].DimensionMeans(), score)
			}
			if cursePool.GameCount < prevCount {
				roomEvolved = append(roomEvolved, cursePool)
			}
		}
	}
	sm.curseMu.Unlock()
	for _, p := range roomEvolved {
		go hat.SavePool(sm.curseDir, p)
	}

	finalSnap := sm.captureSnapshot(gs, commanders, gameNum, startedAt)
	finalSnap.Finished = true
	finalSnap.Winner = winner
	finalSnap.EndReason = endReason

	room.mu.Lock()
	room.snap = finalSnap
	room.mu.Unlock()

	// Update ELO (games are rated).
	sm.mu.Lock()
	sm.stats.gamesPlayed++
	sm.stats.totalTurns += gs.Turn
	sm.updateELO(deckKeys, commanders, pickedDecks, winner, gs.Turn)

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
		RngSeed:    gs.Seed,
	}
	sm.gameHistory = append(sm.gameHistory, completed)
	if len(sm.gameHistory) > 50 {
		sm.gameHistory = sm.gameHistory[len(sm.gameHistory)-50:]
	}
	sm.mu.Unlock()

	// Cardstats.
	for i := 0; i < showmatchSeats; i++ {
		tpl := pickedDecks[i]
		if tpl == nil {
			continue
		}
		names := make([]string, 0, len(tpl.Library)+len(tpl.CommanderCards))
		for _, c := range tpl.CommanderCards {
			if c != nil && c.Name != "" {
				names = append(names, c.Name)
			}
		}
		for _, c := range tpl.Library {
			if c != nil && c.Name != "" {
				names = append(names, c.Name)
			}
		}
		cardstats.Default.Record(names, i == winner, nil)
	}

	// Heimdall.
	roomSeed := heimdall.GameSeed{
		RNGSeed:    gameSeed,
		DeckKeys:   [4]string{deckKeys[0], deckKeys[1], deckKeys[2], deckKeys[3]},
		Winner:     winner,
		Turns:      gs.Turn,
		KillMethod: heimdall.ClassifyKillWithMaxTurns(gs, winner, spectateRoomMaxTurn),
	}
	if sm.heimdall != nil {
		sm.heimdall.RecordSeed(roomSeed)
		sm.heimdall.RecordObservation(heimdall.Observation{
			Seed:            roomSeed,
			ParserGaps:      heimdall.ExtractParserGaps(gs),
			CoTriggers:      heimdall.ExtractCoTriggers(roomETBs),
			CardFirstPlayed: gs.CardFirstPlayed,
		})
	}

	// Tesla: training samples.
	roomPivot := hat.ExtractPivot(roomEvalCollector.History(), winner, gs.Turn)
	for i := 0; i < showmatchSeats; i++ {
		placement := hat.PlacementScore(seatPlacement(gs, i, winner), showmatchSeats)
		samples := roomCollectors[i].Finalize(placement, gs.Turn)
		if len(samples) > 0 {
			labels := hat.LabelSamplesWithPivot(samples, roomPivot)
			enriched := hat.EnrichSamples(samples, labels)
			_ = sm.selfPlay.WriteEnrichedSamples(
				filepath.Join(sm.trainingDir, "samples.jsonl"), enriched)
		}
	}

	// Feynman: post-game invariant check.
	roomOracle := hat.CheckGame(gs)
	// Authoritative InstanceID census (control-transfer-robust ZoneConservation).
	if invs := gameengine.RunAllInvariants(gs); len(invs) > 0 {
		for _, v := range invs {
			log.Printf("invariant[%s]: %s", v.Name, v.Message)
		}
	}
	if !roomOracle.Clean() {
		log.Printf("feynman: spectate-room %s game %d violations: %s",
			room.ID, gameNum, hat.FormatViolations(roomOracle.Violations))
		msgs := make([]string, 0, len(roomOracle.Violations))
		for _, v := range roomOracle.Violations {
			msgs = append(msgs, v.String())
		}
		sm.muninnSink.AutoArchive(roomSeed.RNGSeed, roomSeed.DeckKeys, msgs)
	}
	sm.muninnSink.EndGame()

	// Narrative.
	var roomGameEvents []hat.GameEvent
	narrative := hat.ComposeNarrative(roomPivot, roomGameEvents, commanders, winner, gs.Turn)
	room.broadcast(wsEnvelope{Type: "narrative", Payload: narrative})

	// Persist.
	select {
	case sm.persistCh <- persistJob{game: completed, perfDeltas: cardPerformanceDeltas(gs, winner)}:
	default:
	}

	log.Printf("spectate-room %s: game %d — turn %d, winner: %s (%s)",
		room.ID, gameNum, gs.Turn, safeCommander(commanders, winner), endReason)

	room.broadcast(wsEnvelope{Type: "game", Payload: finalSnap})

	// Check if room should tear down (no viewers, game finished).
	room.specMu.RLock()
	viewers := len(room.spectators)
	room.specMu.RUnlock()
	if viewers == 0 {
		room.resetIdleTimer()
	}
}

func (room *SpectateRoom) broadcast(env wsEnvelope) {
	room.specMu.RLock()
	conns := make([]*spectatorConn, 0, len(room.spectators))
	for sc := range room.spectators {
		conns = append(conns, sc)
	}
	room.specMu.RUnlock()

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
			room.specMu.Lock()
			delete(room.spectators, sc)
			room.specMu.Unlock()
			sc.conn.CloseNow()
		}
	}
}

func (room *SpectateRoom) sendFullState(sc *spectatorConn) {
	room.mu.RLock()
	snap := room.snap
	mult := room.speedMultiplier
	gameNum := room.gameNumber
	room.mu.RUnlock()

	if snap != nil {
		sc.send(wsEnvelope{Type: "game", Payload: snap})
	}
	sc.send(wsEnvelope{Type: "room_info", Payload: map[string]any{
		"room_id":     room.ID,
		"deck_key":    room.DeckKey,
		"commander":   room.Commander,
		"game_number": gameNum,
		"viewers":     room.viewerCount(),
		"speed":       mult,
	}})
	sc.send(wsEnvelope{Type: "elo", Payload: room.sm.GetELO()})
	sc.send(wsEnvelope{Type: "speed", Payload: map[string]any{"multiplier": mult}})
}

// findDeckInPool2 is like findDeckInPool but takes a full deck key string.
func (sm *Showmatch) findDeckInPool2(deckKey string) *deckparser.TournamentDeck {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, d := range sm.deckPool {
		if deckKeyFromPath(d.Path) == deckKey {
			return d
		}
	}
	return nil
}

func shortHash(s string) string {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%x", h)
}

// --- HTTP handlers ---

// handleReloadPool re-scans the deck directory and atomically swaps the
// engine pool, so decks imported since startup become gauntlet/spectate
// eligible without a server restart. CSRF-gated POST. Returns the new
// pool size. The re-scan re-parses every deck (seconds at ~1.4k decks)
// but runs off the request goroutine's caller — the grinder keeps
// running against the old pool until the atomic swap lands.
func (sm *Showmatch) handleReloadPool(w http.ResponseWriter, r *http.Request) {
	n, err := sm.reloadDeckPool()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "pool reload failed: "+err.Error())
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "decks": n})
}

func (sm *Showmatch) handleSpawnSpectateRoom(w http.ResponseWriter, r *http.Request) {
	if sm.maintenance {
		writeError(w, http.StatusServiceUnavailable, maintenanceMessage)
		return
	}
	// Per-IP rate-limit. SpawnOrReuse de-dupes per deck-key, but a single
	// caller can still spawn one room per unique deck id they probe;
	// each room runs a game-driver goroutine. Limiter is nil-safe.
	if enforceRateLimit(sm.SpectateSpawnLimiter, w, r, "spectate spawn") {
		return
	}
	var body struct {
		DeckID string `json:"deck_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DeckID == "" {
		writeError(w, http.StatusBadRequest, "deck_id required")
		return
	}

	room, err := sm.rooms.SpawnOrReuse(sm, body.DeckID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	room.specMu.RLock()
	viewers := len(room.spectators)
	room.specMu.RUnlock()

	writeJSON(w, map[string]any{
		"room_id":   room.ID,
		"deck_key":  room.DeckKey,
		"commander": room.Commander,
		"viewers":   viewers,
		"reused":    room.gameNumber > 0,
	})
}

func (sm *Showmatch) handleGetSpectateRoom(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("room_id")
	room := sm.rooms.GetRoom(roomID)
	if room == nil {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}

	room.specMu.RLock()
	viewers := len(room.spectators)
	room.specMu.RUnlock()

	room.mu.RLock()
	writeJSON(w, map[string]any{
		"room_id":     room.ID,
		"deck_key":    room.DeckKey,
		"commander":   room.Commander,
		"viewers":     viewers,
		"game_number": room.gameNumber,
		"speed":       room.speedMultiplier,
	})
	room.mu.RUnlock()
}

func (sm *Showmatch) handleListSpectateRooms(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, sm.rooms.ListRooms())
}

func (sm *Showmatch) handleSpectateRoomWS(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("room_id")
	room := sm.rooms.GetRoom(roomID)
	if room == nil {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}

	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("spectate-room ws: upgrade error: %v", err)
		return
	}
	wsConn.SetReadLimit(512)

	sc := &spectatorConn{conn: wsConn}

	room.specMu.Lock()
	room.spectators[sc] = struct{}{}
	count := len(room.spectators)
	room.specMu.Unlock()
	room.resetIdleTimer()

	log.Printf("spectate-room %s: viewer connected (%d total)", roomID, count)

	room.sendFullState(sc)

	// Broadcast updated viewer count.
	room.broadcast(wsEnvelope{Type: "viewers", Payload: map[string]int{"count": count}})

	ctx := r.Context()
	for {
		_, data, err := wsConn.Read(ctx)
		if err != nil {
			break
		}
		var env struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(data, &env) != nil {
			continue
		}
		switch env.Type {
		case "ping":
			sc.send(wsEnvelope{Type: "pong", Payload: map[string]int64{"server_time": time.Now().Unix()}})
		case "speed":
			var sp struct {
				Multiplier float64 `json:"multiplier"`
			}
			if json.Unmarshal(env.Payload, &sp) == nil && sp.Multiplier >= 0.1 && sp.Multiplier <= 5.0 {
				room.mu.Lock()
				room.speedMultiplier = sp.Multiplier
				room.mu.Unlock()
				room.broadcast(wsEnvelope{Type: "speed", Payload: map[string]any{"multiplier": sp.Multiplier}})
			}
		}
	}

	room.specMu.Lock()
	delete(room.spectators, sc)
	count = len(room.spectators)
	room.specMu.Unlock()

	log.Printf("spectate-room %s: viewer disconnected (%d remaining)", roomID, count)
	room.broadcast(wsEnvelope{Type: "viewers", Payload: map[string]int{"count": count}})
	wsConn.CloseNow()

	if count == 0 {
		room.resetIdleTimer()
	}
}
