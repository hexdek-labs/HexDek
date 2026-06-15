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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/hexdek/hexdek/internal/achievements"
	"github.com/hexdek/hexdek/internal/anticheat"
	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/cardstats"
	"github.com/hexdek/hexdek/internal/credits"
	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/heimdall"
	"github.com/hexdek/hexdek/internal/matchmaking"
	"github.com/hexdek/hexdek/internal/telemetry"
	"github.com/hexdek/hexdek/internal/tournament"
	"github.com/hexdek/hexdek/internal/trueskill"
)

const (
	showmatchSeats   = 4
	showmatchMaxTurn = 80
	maxLogEntries    = 80
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
	LossReason  string              `json:"loss_reason,omitempty"`
	Battlefield []PermanentSnapshot `json:"battlefield"`
	Eval        *EvalSnapshot       `json:"eval,omitempty"`

	// ManaPoolByColor breaks the float-mana total into WUBRG / C / Any
	// buckets so the spectator UI can render per-color pips instead of
	// the legacy single-int total. Keys: "W", "U", "B", "R", "G", "C"
	// (colorless), "Any" (generic / colorless-flex). Restricted mana
	// rolls up into the "Any" bucket — spectators don't need to see
	// the spend-time restriction, just the spendable count. Omitted
	// when every bucket is zero (the common state during priority
	// passes / between turns).
	ManaPoolByColor map[string]int `json:"mana_pool_by_color,omitempty"`

	// CommandZone is the per-seat list of commanders currently sitting
	// in the command zone (CR §903.6). Each entry carries the
	// commander name and the §903.8 cast-count surcharge — the count
	// of prior command-zone casts of THAT commander (next cast costs
	// base_cmc + 2 * CastCount). Omitted when the zone is empty (the
	// common state once the commander has been cast and is on the
	// battlefield).
	CommandZone []CommandZoneEntry `json:"command_zone,omitempty"`
}

// CommandZoneEntry is one commander sitting in the command zone, as
// exposed to the spectator UI. CastCount tracks the §903.8 tax: a
// commander with CastCount=2 will cost base_cmc + 4 to cast again.
type CommandZoneEntry struct {
	Name      string `json:"name"`
	CastCount int    `json:"cast_count,omitempty"`
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
	Archetype         string  `json:"archetype,omitempty"`
	Budget            int     `json:"budget,omitempty"`
	BudgetUsed        int     `json:"budget_used,omitempty"`
}

type PermanentSnapshot struct {
	Name   string `json:"name"`
	Tapped bool   `json:"tapped"`
	Power  int    `json:"power,omitempty"`
	Tough  int    `json:"toughness,omitempty"`
	IsCmdr bool   `json:"is_commander,omitempty"`
	IsLand bool   `json:"is_land,omitempty"`
	Type   string `json:"type,omitempty"`

	// Counters is the per-kind counter map on this permanent. Common
	// keys: "+1/+1", "-1/-1", "loyalty", "charge", "fade", "time",
	// "stun". Omitted when no counters are present (the common state
	// for non-creature non-planeswalker permanents).
	Counters map[string]int `json:"counters,omitempty"`

	// InstanceID Phase 9 lineage fields (per docs/instanceid-system-v2-r60.md §13).
	// Spectator + Heimdall consume these to render lineage trees on hover.
	// Empty when the permanent's Card has no minted InstanceID (legacy mode).
	InstanceID        string   `json:"instance_id,omitempty"`
	Provenance        string   `json:"provenance,omitempty"`
	SourceInstanceID  string   `json:"source_instance_id,omitempty"`
	EnablerInstanceID string   `json:"enabler_instance_id,omitempty"`
	EnablerHistory    []string `json:"enabler_history,omitempty"`
	MergedCardIDs     []string `json:"merged_card_ids,omitempty"`
}

type LogEntry struct {
	Turn    int      `json:"turn"`
	Seat    int      `json:"seat"`
	Action  string   `json:"action"`
	Detail  string   `json:"detail,omitempty"`
	Kind    string   `json:"kind"`
	Source  string   `json:"source,omitempty"`
	Targets []string `json:"targets,omitempty"`
	Amount  int      `json:"amount,omitempty"`
	Count   int      `json:"count,omitempty"`

	// InstanceID Phase 9 — InstanceID of the primary object referenced by
	// this event line (the spell card, the dying permanent, the ability
	// instance pushed onto the stack). Empty when no InstanceID lineage
	// is available for the event. Useful for forensic replay: filter the
	// spectator log to every event touching a specific InstanceID.
	InstanceID string `json:"instance_id,omitempty"`
}

// StackItemSnapshot is one entry on the resolution stack, as exposed
// to the spectator UI. Mirrors a subset of gameengine.StackItem with
// the names resolved for display. The Source field holds the spell or
// ability source (for activated/triggered abilities, the source
// permanent name; for spells, the card name).
type StackItemSnapshot struct {
	Source     string `json:"source"`     // resolved card / permanent name
	Controller int    `json:"controller"` // seat index
	Kind       string `json:"kind"`       // "spell" | "activated" | "triggered" | "" (legacy)
	IsCopy     bool   `json:"is_copy,omitempty"`
	Countered  bool   `json:"countered,omitempty"`
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

	// Stack is the engine's resolution stack at the moment the snapshot
	// was captured. Order matches the engine's bottom-to-top order:
	// Stack[0] is the bottom (oldest item), Stack[len-1] is the top
	// (resolves first). Omitted when the stack is empty (the common
	// state outside of cast/response windows).
	Stack []StackItemSnapshot `json:"stack,omitempty"`

	// Lineage is the InstanceID Phase 9 index — every Card across every
	// zone (battlefield, hand, graveyard, exile, library, command zone,
	// merged-card pointers) with a minted InstanceID gets one entry.
	// Keyed by InstanceID. Spectator + Heimdall consume this to render
	// lineage trees on demand. Empty when no IDs were minted (legacy
	// games).
	Lineage map[string]heimdall.LineageRecord `json:"lineage,omitempty"`
}

type ELOEntry struct {
	DeckID    string  `json:"deck_id"`
	Commander string  `json:"commander"`
	Owner     string  `json:"owner"`
	Rating    float64 `json:"rating"`
	Mu        float64 `json:"mu"`
	Sigma     float64 `json:"sigma"`
	HexRating float64 `json:"hex_rating"`
	Games     int     `json:"games"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	WinRate   float64 `json:"win_rate"`
	Delta     float64 `json:"delta"`
	HexDelta  float64 `json:"hex_delta"`
	Bracket   int     `json:"bracket"`
	Band      string  `json:"band"`
	Drift     float64 `json:"drift"`
	DriftTag  string  `json:"drift_tag,omitempty"`
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
	GameID     int      `json:"game_id"`
	Commanders []string `json:"commanders"`
	DeckKeys   []string `json:"deck_keys"`
	Winner     int      `json:"winner"`
	WinnerName string   `json:"winner_name"`
	Turns      int      `json:"turns"`
	EndReason  string   `json:"end_reason"`
	// WinReason captures HOW the winner won (heimdall.ClassifyKill,
	// prefixed "win_"). Orthogonal to EndReason (the game-end TYPE).
	// Empty for draws / unclassified games.
	WinReason  string         `json:"win_reason,omitempty"`
	FinishedAt time.Time      `json:"finished_at"`
	FinalSeats []SeatSnapshot `json:"final_seats"`
	// RngSeed is the engine seed for this game. Surfaced for replay /
	// anti-cheat (Phase 1: capture-only). 0 = unknown/not captured.
	RngSeed int64 `json:"rng_seed"`
	// Timeline is the per-turn capture used by the replay viewer.
	// Populated for live games; empty for games rehydrated from
	// SQLite (the persisted schema only stores end-of-game seats).
	// omitempty keeps /api/games payloads lean for callers that don't
	// need the scrubber.
	Timeline []TurnSnapshot `json:"timeline,omitempty"`
}

// TurnSnapshot is the per-turn record attached to CompletedGame so
// the post-game report can scrub through the game. Captured at the
// end of each turn in the live showmatch loop. Drops the per-seat
// Eval telemetry from GameSnapshot to keep payload size sane —
// scrubbing 30 turns × 4 seats × full eval would 3x the response.
type TurnSnapshot struct {
	Turn       int                `json:"turn"`
	ActiveSeat int                `json:"active_seat"`
	Seats      []SeatTurnSnapshot `json:"seats"`
	// Events fired during this turn. Already deduped/coalesced by
	// extractEvents, same shape the live spectator log uses.
	Events []LogEntry `json:"events,omitempty"`
}

type SeatTurnSnapshot struct {
	Life        int                 `json:"life"`
	HandSize    int                 `json:"hand_size"`
	LibrarySize int                 `json:"library_size"`
	GYSize      int                 `json:"gy_size"`
	Lost        bool                `json:"lost,omitempty"`
	LossReason  string              `json:"loss_reason,omitempty"`
	Battlefield []PermanentSnapshot `json:"battlefield,omitempty"`
}

type persistJob struct {
	game       CompletedGame
	perfDeltas []db.CardPerformanceDelta
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
	// Placements is the per-position finish breakdown for the deck under
	// test (seat 0) across all gauntlet games. Index 0 = 1st place count,
	// index 1 = 2nd, index 2 = 3rd, index 3 = 4th. Surfaces the "I'm
	// surviving to top-2 a lot but not closing" pattern that the binary
	// W/L view obscures. By definition Wins == Placements[0] and Losses
	// == Placements[1] + Placements[2] + Placements[3].
	Placements [4]int `json:"placements,omitempty"`

	// RunID is the gauntlet_runs.run_id reserved at gauntlet start.
	// Used by the replay-archive path to link this run's per-game
	// showmatch_game rows back to its aggregate row via the
	// gauntlet_run_game junction table. 0 when the gauntlet started
	// without a SQL backing store (tests / dev runs).
	RunID int64 `json:"run_id,omitempty"`
}

// maintenanceMessage is returned (HTTP 503) by the gauntlet + spectate
// endpoints when HEXDEK_MAINTENANCE=1, while r61 engine-correctness fixes
// are in progress (missed-clause / */* CDA bugs). The grinder is also
// disabled in that mode so no further games corrupt rating/analytics data.
const maintenanceMessage = "Gauntlets & live games are paused for maintenance — engine fixes in progress. Back soon. 🔧"

// inMaintenance reads the maintenance flag under the state lock. Game
// loops poll this between turns (r62, report 06 C3) so a flip mid-game
// halts the game cleanly without recording a rated result.
func (sm *Showmatch) inMaintenance() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.maintenance
}

// SetMaintenance flips maintenance mode at runtime. Construction still
// seeds the flag from HEXDEK_MAINTENANCE; this setter exists so tests —
// and a future admin endpoint — can flip it on a live server and have
// every running rated loop (fishtank, grinder workers, spectate rooms)
// stop at its next between-turns check instead of riding out the game
// and corrupting the rating/analytics data maintenance is meant to
// protect.
func (sm *Showmatch) SetMaintenance(on bool) {
	sm.mu.Lock()
	sm.maintenance = on
	sm.mu.Unlock()
}

type Showmatch struct {
	mu              sync.RWMutex
	snap            *GameSnapshot
	elo             map[string]*eloState
	bracketCache    map[string]int // deck key → Freya bracket (1-5)
	stats           sessionState
	start           time.Time
	maintenance     bool // HEXDEK_MAINTENANCE=1 → grinder off + 503 gauntlet/spectate
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
	heimdall        *heimdall.Observer
	muninnSink      *muninnAdapter
	achievements    *achievements.Tracker
	judge           *judgeWatchdog // sampled Hex Judge over grinder/fishtank games (nil = off)

	curseMu   sync.Mutex
	cursePool map[string]*hat.CursePool // deck key → genetic population
	curseDir  string

	decksDir string // deck source dir, retained so reloadDeckPool can re-scan it live

	trainingDir string                          // neural evaluator training data output
	neuralEval  *hat.NeuralEvaluator            // shared neural model (nil = not trained yet)
	selfPlay    *hat.SelfPlayManager            // Level 6 self-play training loop
	strategies  map[string]*hat.StrategyProfile // deck key → Freya strategy profile

	// compositionPrior is the R60 archetype-pod-conditioned TrueSkill
	// prior (PR #403 + #408). When non-nil, updateELO applies it to
	// the per-game rating update so ratings represent "skill modulo
	// composition" instead of raw winrate. Validation (#411) showed
	// +1.4pp prediction accuracy + +0.036 log-loss vs. vanilla.
	compositionPrior    *trueskill.CompositionPrior
	compositionPriorCfg trueskill.CompositionUpdateConfig

	specMu     sync.RWMutex
	spectators map[*spectatorConn]struct{}

	gauntletMu sync.RWMutex
	gauntlets  map[string]*GauntletResult

	// gauntletSubs holds the active SSE subscribers per deck key. Each
	// subscriber receives a snapshot of GauntletResult after every
	// completed game and on terminal status transitions, then is closed.
	gauntletSubsMu sync.Mutex
	gauntletSubs   map[string]map[*gauntletSubscriber]struct{}

	// MatchBroadcaster fans per-game TournamentMatchEvent deltas to
	// subscribers of /api/tournament/{id}/stream. Distinct from
	// gauntletSubs (which is whole-state snapshots keyed by deckKey) —
	// this is delta-shaped and keyed by run_id so a live-results UI
	// can subscribe by tournament id. Constructed lazily on first use
	// so older test setups that bypass StartGauntlet don't need to
	// pre-init it.
	MatchBroadcaster *MatchBroadcaster

	// credits is the optional credit-economy gate for paid gauntlet
	// runs. When nil (e.g. tests, dev runs without main wiring), the
	// gauntlet handler falls back to "free for everyone" — the
	// economy is a soft layer over the existing endpoint.
	credits *credits.Store

	rooms *RoomManager

	// auditor is the contributor-level statistical anomaly detector.
	// Nil when no SQLite DB is wired (tests, ephemeral runs); the
	// updateELO hook checks for nil before recording.
	auditor *anticheat.StatisticalAuditor

	// Webhooks fan out the game.end event to caller-registered
	// HTTP endpoints. Nil = feature disabled (default for tests
	// and lightweight server boots); cmd/hexdek-server constructs
	// one when HEXDEK_WEBHOOKS_ENABLED is set and the SQLite DB
	// is wired in.
	Webhooks *WebhookDispatcher

	// Events is the in-process pub/sub bus that backs the
	// /ws/events WebSocket stream. Nil = feature disabled; the
	// game-end tail no-ops the Publish call. cmd/hexdek-server
	// constructs the bus + handler at startup when wiring is on.
	Events *EventBus

	// lastComboCursor tracks the highest gs.EventLog index that
	// has already been scanned for `infinite_cycle` events, keyed
	// by gameNum so a new game resets the cursor. Guarded by mu.
	// Used by publishNewCombos to dedupe — the cycle detector can
	// emit the same observation across multiple snapshot ticks
	// while the engine resolves a long stack, so we publish to
	// the event bus exactly once per event-log entry.
	lastComboCursor map[int]int

	// GauntletLimiter rate-limits POST /api/gauntlet/{owner}/{id} per
	// client IP. The endpoint is already protected by a global
	// concurrency semaphore (gauntletSem, cap 2) and a credit-economy
	// gate for authenticated callers, but unauthenticated bursts still
	// burn deck-key lookups + credit-quota checks + can occupy the two
	// free slots indefinitely. Per-IP token-bucket is a layered defense
	// in front of the sem cap. Nil = no limiting (backwards-compat);
	// cmd/hexdek-server sets a default at startup.
	GauntletLimiter *RateLimiter

	// SpectateSpawnLimiter rate-limits POST /api/spectate/spawn per
	// client IP. SpawnOrReuse de-dupes per deck-key but a single caller
	// can still spawn one room per unique deck id they enumerate; each
	// room runs a game-driver goroutine. Nil = no limiting. cmd/hexdek-
	// server sets a default at startup.
	SpectateSpawnLimiter *RateLimiter

	// GauntletOwnerLimiter pairs with GauntletLimiter on the
	// tournament-run endpoint (POST /api/gauntlet/{owner}/{id}) to add
	// the per-owner axis on top of the existing per-IP axis. Without
	// this, an authenticated owner can rotate IPs (mobile + laptop +
	// VPN) to multiplex bursts past the per-IP cap and saturate the
	// gauntletSem concurrency cap (2). enforceRateLimitDual gates
	// both axes. Nil = per-owner axis disabled (per-IP axis still
	// enforced via GauntletLimiter). cmd/hexdek-server sets a default
	// at startup.
	GauntletOwnerLimiter *RateLimiter

	// LineageLimiter rate-limits GET /api/spectator/lineage/{instance_id}
	// per client IP. Each call walks every live spectate room's most-
	// recent snapshot (O(rooms × lineage-entries-per-room) — the
	// snapshots can each hold thousands of lineage records on long
	// games) and renders a tree, so unbounded calls let an attacker
	// pin the GC + serialization budget by spamming format-valid
	// InstanceIDs that miss every room (404 path is just as expensive
	// as the hit path because it scans every room first). Pre-r60 the
	// endpoint had NO rate limit at all — surfaced during the r60
	// rate-limit-coverage audit. Nil = no limiting (backwards-compat);
	// cmd/hexdek-server sets a default at startup.
	LineageLimiter *RateLimiter

	// LineageOwnerLimiter is the per-owner sibling to LineageLimiter —
	// see enforceRateLimitDual for the per-IP-vs-per-owner rationale.
	// Nil = per-owner axis disabled.
	LineageOwnerLimiter *RateLimiter

	// CSRFStore mints / verifies the same HMAC-signed tokens used by
	// Handler.CSRFStore (the two fields point at the SAME store —
	// cmd/hexdek-server constructs one store at startup and stamps it
	// onto both). Needed here so the Showmatch-owned mutating routes
	// (gauntlet start, live-speed override, spectate-room spawn,
	// per-deck curse PATCH) can wrap themselves in RequireCSRF without
	// having to thread the handler-side store through every constructor.
	// Nil = no enforcement (backwards-compat with binaries that don't
	// construct a store yet) — matches Handler.CSRFStore's nil-safe
	// semantics so an upgrade can land in either order.
	CSRFStore *CSRFStore
}

// requireCSRFLate is the late-binding form of RequireCSRF for
// Showmatch-owned routes. The Handler-side store is constructed AFTER
// sm.RegisterShowmatch runs in cmd/hexdek-server/main.go (the
// registration block fires before the CSRF-secret env-var resolution
// later in main), so capturing sm.CSRFStore at registration time
// would always see nil. Resolving the store at request time picks up
// whatever main() later stamps onto sm — and a nil store at request
// time is a pass-through, matching RequireCSRF(nil, ...) semantics.
//
// Per-request overhead is one closure construction (two captured words)
// for the inner RequireCSRF call — negligible for these low-volume
// mutating routes.
func (sm *Showmatch) requireCSRFLate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		RequireCSRF(sm.CSRFStore, next)(w, r)
	}
}

// SetCreditStore attaches a credits.Store to the Showmatch. Called
// once from main() after credit schema setup. Safe to leave unset
// in tests.
func (sm *Showmatch) SetCreditStore(c *credits.Store) {
	sm.credits = c
}

// DeckParserMeta exposes the card-meta DB the Showmatch loads at
// startup so handlers outside this file (currently /api/health for
// the hat-readiness probe; the parse-preview endpoint in flight will
// also consume it) can read it without duplicating the load. Returns
// nil while the background load is in progress (sm.ready) or when
// meta loading failed — callers MUST nil-check and surface the
// "not configured" state instead of dereferencing.
//
// Read-only accessor; the underlying meta is populated once at
// startup and never mutated, so no locking is needed for the pointer
// read.
func (sm *Showmatch) DeckParserMeta() *deckparser.MetaDB { return sm.meta }

type spectatorConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type eloState struct {
	rating     float64 // TrueSkill conservative rating (μ - 3σ), bracket-seeded
	hexRating  float64 // HexELO (bracket-seeded, K-modulated, gravity)
	tsMu       float64 // TrueSkill μ (skill estimate)
	tsSigma    float64 // TrueSkill σ (uncertainty)
	games      int
	delta      float64
	hexDelta   float64
	wins       int
	commander  string
	owner      string
	bracket    int
	lossStreak int // consecutive losses for diminishing penalty + streak break bonus
}

type sessionState struct {
	gamesPlayed   int // current session only (grinder + showmatch)
	historicGames int // loaded from DB on startup (all prior sessions)
	historicTurns int
	totalTurns    int
}

func NewShowmatch(astPath, oraclePath, decksDir string, database *sql.DB) *Showmatch {
	muninnSink := newMuninnAdapter("data/muninn")
	sm := &Showmatch{
		elo:              make(map[string]*eloState),
		bracketCache:     make(map[string]int),
		start:            time.Now(),
		speedMultiplier:  1.0,
		sqlDB:            database,
		persistCh:        make(chan persistJob, 512),
		spectators:       make(map[*spectatorConn]struct{}),
		gauntlets:        make(map[string]*GauntletResult),
		gauntletSubs:     make(map[string]map[*gauntletSubscriber]struct{}),
		MatchBroadcaster: NewMatchBroadcaster(),
		rooms:            NewRoomManager(),
		heimdall:         heimdall.New("data", &huginnAdapter{dataDir: "data"}, muninnSink, newTelemetrySink()),
		muninnSink:       muninnSink,
		cursePool:        make(map[string]*hat.CursePool),
		curseDir:         "data/curse",
		decksDir:         decksDir,
		trainingDir:      "data/training",
	}
	// R60 composition prior — initialized empty; ObserveGame in
	// updateELO populates it as games complete, and after the prior
	// has enough data per (archetype, pod) cell, Confidence rises
	// and the offsets start meaningfully shaping new rating updates.
	// Until then the gate is no-op (Confidence=0 → offset=0).
	sm.compositionPrior = trueskill.NewCompositionPrior(showmatchSeats)
	sm.compositionPriorCfg = trueskill.DefaultCompositionUpdateConfig(sm.compositionPrior)
	if database != nil {
		if err := db.EnsureCardStatsSchema(context.Background(), database); err != nil {
			log.Printf("showmatch: card_stats schema: %v", err)
		}
		if a, err := anticheat.NewStatisticalAuditor(database); err != nil {
			log.Printf("showmatch: anticheat auditor init: %v", err)
		} else {
			sm.auditor = a
		}
		sm.loadPersistedState()
		go sm.persistWorker()
	}
	os.MkdirAll(sm.trainingDir, 0755)
	sm.neuralEval = hat.TryLoadNeuralEvaluator(filepath.Join(sm.trainingDir, "model.json"))
	if sm.neuralEval != nil {
		log.Printf("neural: loaded trained model from %s/model.json", sm.trainingDir)
	}
	spCfg := hat.DefaultSelfPlayConfig(".")
	if _, err := os.Stat("venv/bin/python3"); err == nil {
		spCfg.PythonBin = "venv/bin/python3"
	}
	sm.selfPlay = hat.NewSelfPlayManager(spCfg)
	sm.selfPlay.OnModelLoad = func(ne *hat.NeuralEvaluator) {
		sm.mu.Lock()
		sm.neuralEval = ne
		sm.mu.Unlock()
		log.Printf("selfplay: hot-reloaded neural model (gen %d)", sm.selfPlay.Generation())
	}
	if loaded, err := hat.LoadAllPools(sm.curseDir, rand.New(rand.NewSource(42))); err == nil && len(loaded) > 0 {
		sm.cursePool = loaded
		migrated := 0
		for _, pool := range loaded {
			if pool.TotalGames == 0 && (pool.GenCount > 0 || pool.GameCount > 0) {
				pool.TotalGames = pool.GenCount*hat.CurseEvolveAt + pool.GameCount
				migrated++
			}
		}
		if migrated > 0 {
			log.Printf("curse: seeded TotalGames on %d pools from GenCount*%d+GameCount", migrated, hat.CurseEvolveAt)
		}
		log.Printf("curse: loaded %d deck pools from %s", len(loaded), sm.curseDir)
	}
	if tr, err := achievements.NewTracker("data/achievements"); err != nil {
		log.Printf("achievements: init failed: %v", err)
	} else {
		sm.achievements = tr
	}
	sm.maintenance = os.Getenv("HEXDEK_MAINTENANCE") == "1"
	if sm.maintenance {
		log.Printf("MAINTENANCE MODE ON (HEXDEK_MAINTENANCE=1): grinder disabled; gauntlet + spectate return 503")
	}
	sm.judge = newJudgeWatchdogFromEnv()
	if sm.judge != nil {
		log.Printf("judge-watchdog: ON — sampling %.1f%% of games (game-level dims), %.2f%% (corpus dims), triage log %s",
			sm.judge.gameRate*100, sm.judge.corpusRate*100, sm.judge.logPath)
	}
	go sm.loadAndRun(astPath, oraclePath, decksDir)
	return sm
}

// newTelemetrySink returns a TelemetrySink backed by GA4 if env vars are set,
// or nil if GA4 is not configured.
func newTelemetrySink() heimdall.TelemetrySink {
	ga4 := telemetry.NewGA4Client()
	if ga4 == nil {
		return nil
	}
	return &telemetryAdapter{ga4: ga4}
}

func (sm *Showmatch) persistWorker() {
	for job := range sm.persistCh {
		sm.persistGame(job.game)
		if sm.sqlDB != nil && len(job.perfDeltas) > 0 {
			if err := db.BatchUpsertCardPerformance(context.Background(), sm.sqlDB, job.perfDeltas); err != nil {
				log.Printf("showmatch: card_performance: %v", err)
			}
		}
	}
}

func (sm *Showmatch) loadPersistedState() {
	ctx := context.Background()

	records, err := db.LoadAllELO(ctx, sm.sqlDB)
	if err != nil {
		log.Printf("showmatch: load persisted ELO: %v", err)
		return
	}
	resetCount := 0
	for _, r := range records {
		// Band-aid reset (r60): tsMu/tsSigma were never persisted (so they
		// reset to 0 every boot → degenerate σ=0 updates), and the pre-clamp
		// runaway left some persisted ratings far out of band (±100M). Re-seed
		// any out-of-band / NaN rating to its bracket start, and reconstruct
		// μ/σ from the conservative rating so the next update runs on a sane
		// prior (σ=σ₀) instead of σ=0.
		rating := r.Rating
		if rating != rating || rating < eloRatingMin || rating > eloRatingMax {
			rating = trueSkillStartRating(r.Bracket).Conservative()
			resetCount++
		}
		// r62 (report 09 C-2): prefer the persisted TrueSkill mu/sigma.
		// ts_sigma == 0 marks a row written before the columns existed
		// (or by an older binary) — only those fall back to the legacy
		// reconstruction. Out-of-band ratings (the band-aid reset above)
		// also discard the persisted pair: if the scalar was garbage,
		// the pair that produced it is not trusted either. The PR #996
		// clamps stay as the backstop in both branches.
		sigma := tsDefaultSigma
		mu := clampF(rating+3*sigma, tsMuFloor, tsMuCeil)
		if r.TSSigma > 0 && rating == r.Rating {
			mu = clampF(r.TSMu, tsMuFloor, tsMuCeil)
			sigma = clampF(r.TSSigma, tsSigmaFloor, tsDefaultSigma)
		}
		sm.elo[r.DeckKey] = &eloState{
			rating:    clampF(rating, eloRatingMin, eloRatingMax),
			hexRating: r.HexRating,
			tsMu:      mu,
			tsSigma:   sigma,
			games:     r.Games,
			wins:      r.Wins,
			delta:     r.Delta,
			hexDelta:  r.HexDelta,
			commander: r.Commander,
			owner:     r.Owner,
			bracket:   r.Bracket,
		}
	}
	if resetCount > 0 {
		log.Printf("showmatch: ELO band-aid re-seeded %d out-of-band rating(s) on load", resetCount)
	}

	if affected, err := db.BackfillDeckKeys(ctx, sm.sqlDB); err != nil {
		log.Printf("showmatch: backfill deck_keys: %v", err)
	} else if affected > 0 {
		log.Printf("showmatch: backfilled %d seat rows with deck_key", affected)
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
			RngSeed:    g.Seed,
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

	decks, bracketMap, stratMap, err := buildDeckPool(decksDir, corpus, meta)
	if err != nil {
		log.Printf("showmatch: %v", err)
		sm.mu.Lock()
		sm.loadErr = "failed to load deck files"
		sm.mu.Unlock()
		return
	}

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
	sm.bracketCache = bracketMap
	sm.strategies = stratMap
	for key, e := range sm.elo {
		if e.bracket == 0 {
			if b, ok := bracketMap[key]; ok {
				e.bracket = b
			}
		}
		if e.hexRating == 0 && e.bracket > 0 {
			e.hexRating = hexELOStartRating(e.bracket)
		}
	}
	sm.ready = true
	sm.mu.Unlock()

	log.Printf("showmatch: ready — %d decks in pool, starting fishtank + background grinder", len(decks))
	if sm.inMaintenance() {
		log.Printf("grinder: NOT starting — maintenance mode (HEXDEK_MAINTENANCE=1)")
	} else {
		go sm.runGrinder()
	}
	go sm.runStatsBroadcaster()
	go sm.runHealthPulse()
	sm.runLoop()
}

func (sm *Showmatch) runLoop() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	loggedPause := false
	for {
		// r62 (report 06 C3): the fishtank previously ran rated games
		// unconditionally — one rated game every few minutes kept
		// mutating ELO / history / persistence in maintenance mode.
		// Pause instead; the last snapshot stays on display.
		if sm.inMaintenance() {
			if !loggedPause {
				loggedPause = true
				log.Printf("fishtank: paused — maintenance mode")
			}
			time.Sleep(2 * time.Second)
			continue
		}
		loggedPause = false
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
				// r62 (report 06 C3): honor a runtime maintenance flip
				// even though the grinder never STARTS in maintenance.
				if sm.inMaintenance() {
					time.Sleep(2 * time.Second)
					continue
				}
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

// runHealthPulse sends a periodic health pulse to GA4 via Heimdall every 60s.
func (sm *Showmatch) runHealthPulse() {
	if sm.heimdall == nil {
		return
	}
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		sm.mu.RLock()
		gamesPlayed := sm.stats.historicGames + sm.stats.gamesPlayed
		sm.mu.RUnlock()

		ms := sm.muninnSink.batcher.Stats()

		sm.heimdall.Pulse(heimdall.HealthPulse{
			GamesPlayed:   gamesPlayed,
			ParserGaps:    ms.ParserGaps,
			Crashes:       ms.Crashes,
			DeadTriggers:  ms.DeadTriggers,
			TopGapCards:   nil,
			EngineVersion: "0.1.0",
		})
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
		log.Printf("gauntlet: deck %s not in engine pool — filtered at startup or missing", deckKey)
		errResult := &GauntletResult{
			DeckKey: deckKey, Status: "error", Commander: id,
		}
		sm.gauntletMu.Lock()
		sm.gauntlets[deckKey] = errResult
		sm.gauntletMu.Unlock()
		sm.broadcastGauntlet(deckKey, *errResult)
		sm.closeGauntletSubs(deckKey)
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

	// Reserve the gauntlet_runs row up front so per-game showmatch_game
	// inserts can link back via the gauntlet_run_game junction. Without
	// this, the only post-hoc link is the (deck_key, time-window)
	// heuristic — which mis-attributes games when two gauntlets for
	// the same deck overlap in time. Best-effort: a DB error here leaves
	// result.RunID=0 and the gauntlet still runs (per-game writes will
	// skip the link step).
	if sm.sqlDB != nil {
		runID, err := db.InsertPendingGauntletRun(context.Background(), sm.sqlDB,
			deckKey, targetDeck.CommanderName, result.StartedAt)
		if err != nil {
			log.Printf("gauntlet_runs reserve: %v", err)
		} else {
			result.RunID = runID
		}
	}

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

		gameSeed := time.Now().UnixNano() + int64(g)*37
		gameRng := rand.New(rand.NewSource(gameSeed))
		gs := gameengine.NewGameState(showmatchSeats, gameRng, sm.corpus)
		gs.Seed = gameSeed
		gs.EventPolicy = gameengine.EventLogNone

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

		// Curse: select DNA for gauntlet seats.
		var gauntDnaIdxes [showmatchSeats]int
		var gauntHats [showmatchSeats]*hat.YggdrasilHat
		for i := 0; i < showmatchSeats; i++ {
			pool := sm.getOrCreateCursePool(deckKeys[i], rng)
			sm.curseMu.Lock()
			dna, dnaIdx := pool.SelectForGame()
			dnaCopy := *dna
			dimStats := pool.DimStats
			sm.curseMu.Unlock()
			gauntDnaIdxes[i] = dnaIdx
			h := hat.NewYggdrasilHatWithPool(&dnaCopy, sm.strategies[deckKeys[i]], 50, &dimStats)
			h.NeuralEval = sm.neuralEval
			gs.Seats[i].Hat = h
			gauntHats[i] = h
		}

		for i := 0; i < showmatchSeats; i++ {
			tournament.RunLondonMulligan(gs, i)
		}

		gs.Active = gameRng.Intn(showmatchSeats)
		gs.Turn = 1

		var collectors [showmatchSeats]*hat.TrainingCollector
		for i := 0; i < showmatchSeats; i++ {
			collectors[i] = hat.NewTrainingCollector(5)
		}
		evalCollector := hat.NewEvalSnapshotCollector()

		turnETBs := make(map[int][]string)
		for turn := 1; turn <= showmatchMaxTurn; turn++ {
			gs.Turn = turn
			preBF := heimdall.SnapshotBattlefieldNames(gs)
			tournament.TakeTurn(gs)
			gameengine.StateBasedActions(gs)
			for i := 0; i < showmatchSeats; i++ {
				collectors[i].Snapshot(gs, i)
			}
			if turn%5 == 0 || turn == 1 {
				evalCollector.Record(turn, extractEvalScores(gs))
			}
			if newCards := heimdall.DiffBattlefield(preBF, heimdall.SnapshotBattlefieldNames(gs)); len(newCards) > 0 {
				turnETBs[turn] = newCards
			}
			if gs.CheckEnd() {
				break
			}
			// CR §720 extra turns (LIFO) + skip-your-next-turn via the shared
			// sequencing helper; gs.Turn is re-stamped next iteration.
			gameengine.AdvanceActiveSeat(gs)
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

		// Curse: record results + dimension stats for gauntlet seats.
		sm.curseMu.Lock()
		var gauntEvolved []*hat.CursePool
		for i := 0; i < showmatchSeats; i++ {
			if pool, ok := sm.cursePool[deckKeys[i]]; ok {
				score := hat.PlacementScore(seatPlacement(gs, i, winner), showmatchSeats)
				prevCount := pool.GameCount
				pool.RecordResult(gauntDnaIdxes[i], score)
				if gauntHats[i] != nil {
					pool.DimStats.RecordGame(gauntHats[i].DimensionMeans(), score)
				}
				if pool.GameCount < prevCount {
					gauntEvolved = append(gauntEvolved, pool)
				}
			}
		}
		sm.curseMu.Unlock()
		for _, pool := range gauntEvolved {
			go hat.SavePool(sm.curseDir, pool)
		}

		totalTurns += gs.Turn

		sm.mu.Lock()
		sm.stats.gamesPlayed++
		sm.stats.totalTurns += gs.Turn
		priorEffects := sm.updateELO(deckKeys, commanders, pickedDecks, winner, gs.Turn)
		sm.mu.Unlock()

		// Replay archive: persist per-game outcomes and link them to
		// this gauntlet run so future analysis can recover the original
		// game stream without re-running. Best-effort — a DB error
		// here leaves the row out of the archive but the gauntlet
		// keeps running (in-memory result remains correct).
		if sm.sqlDB != nil && result.RunID > 0 {
			gameID, perr := sm.persistGauntletGame(deckKeys, commanders, gs, gameSeed, winner)
			if perr != nil {
				log.Printf("gauntlet replay: persist game %d: %v", g, perr)
			} else if gameID > 0 {
				if err := db.InsertGauntletRunGame(context.Background(), sm.sqlDB,
					result.RunID, gameID, g); err != nil {
					log.Printf("gauntlet replay: link game %d to run %d: %v", gameID, result.RunID, err)
				}
			}
		}

		gauntSeed := heimdall.GameSeed{
			RNGSeed:    gameSeed,
			DeckKeys:   [4]string{deckKeys[0], deckKeys[1], deckKeys[2], deckKeys[3]},
			Winner:     winner,
			Turns:      gs.Turn,
			KillMethod: heimdall.ClassifyKillWithMaxTurns(gs, winner, showmatchMaxTurn),
		}
		if sm.heimdall != nil {
			sm.heimdall.RecordSeed(gauntSeed)
			sm.heimdall.RecordObservation(heimdall.Observation{
				Seed:                    gauntSeed,
				ParserGaps:              heimdall.ExtractParserGaps(gs),
				CoTriggers:              heimdall.ExtractCoTriggers(turnETBs),
				CardFirstPlayed:         gs.CardFirstPlayed,
				CompositionPriorEffects: priorEffects,
			})
		}

		// Tesla: extract causal pivot and enrich training samples.
		pivot := hat.ExtractPivot(evalCollector.History(), winner, gs.Turn)
		for i := 0; i < showmatchSeats; i++ {
			placement := hat.PlacementScore(seatPlacement(gs, i, winner), showmatchSeats)
			samples := collectors[i].Finalize(placement, gs.Turn)
			if len(samples) > 0 {
				labels := hat.LabelSamplesWithPivot(samples, pivot)
				enriched := hat.EnrichSamples(samples, labels)
				_ = sm.selfPlay.WriteEnrichedSamples(
					filepath.Join(sm.trainingDir, "samples.jsonl"), enriched)
			}
		}

		// Feynman: post-game invariant check.
		oracleResult := hat.CheckGame(gs)
		// Authoritative InstanceID census (control-transfer-robust ZoneConservation).
		if invs := gameengine.RunAllInvariants(gs); len(invs) > 0 {
			for _, v := range invs {
				log.Printf("invariant[%s]: %s", v.Name, v.Message)
			}
		}
		if !oracleResult.Clean() {
			log.Printf("feynman: grinder game violations: %s",
				hat.FormatViolations(oracleResult.Violations))
			sm.muninnSink.AutoArchive(gauntSeed.RNGSeed, gauntSeed.DeckKeys, oracleResult.Violations)
		}
		sm.muninnSink.EndGame()

		// Per-position finish tracking for the deck under test (seat 0).
		// seatPlacement returns 1-based finish (1 = won, 2 = runner-up,
		// 3 = third, 4 = knocked out first). Clamp to [1,4] and write to
		// Placements[0..3].
		seat0Place := seatPlacement(gs, 0, winner)
		if seat0Place >= 1 && seat0Place <= showmatchSeats {
			result.Placements[seat0Place-1]++
		}

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
		result.WinRate = math.Round(float64(result.Wins)/float64(result.Games)*1000) / 10

		sm.gauntletMu.RLock()
		snap := *result
		sm.gauntletMu.RUnlock()
		sm.broadcastGauntlet(deckKey, snap)

		// Fan a per-game delta to /api/tournament/{id}/stream
		// subscribers. Only fires when the run is persisted
		// (RunID > 0) so the stream key (run_id) is meaningful.
		if sm.MatchBroadcaster != nil && result.RunID > 0 {
			sm.MatchBroadcaster.Broadcast(result.RunID, BuildMatchEvent(
				result.RunID, g+1, numGames, winner,
				commanders[:], gs.Turn, result.Wins, result.Losses,
				time.Now(),
			))
		}

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
	finalSnap := *result
	sm.gauntletMu.Unlock()
	sm.broadcastGauntlet(deckKey, finalSnap)
	sm.closeGauntletSubs(deckKey)
	if sm.MatchBroadcaster != nil && result.RunID > 0 {
		sm.MatchBroadcaster.Close(result.RunID)
	}

	// Persist snapshot for ELO-history chart. Best-effort — a DB error
	// here doesn't fail the gauntlet (the in-memory result still serves
	// the API). Logged on failure so we notice trends.
	//
	// Replay-archive path: when result.RunID > 0 we reserved a
	// pending row up front and stamped per-game links along the way,
	// so finalize via UPDATE rather than a fresh INSERT. RunID == 0
	// means InsertPendingGauntletRun failed (or sqlDB was nil at
	// gauntlet start) — fall back to the legacy single-INSERT path
	// so older callers / failure modes still produce a row.
	if sm.sqlDB != nil {
		rec := db.GauntletRunRecord{
			RunID:      result.RunID,
			DeckKey:    result.DeckKey,
			Commander:  result.Commander,
			StartedAt:  result.StartedAt,
			FinishedAt: result.FinishedAt,
			Games:      result.Games,
			Wins:       result.Wins,
			Losses:     result.Losses,
			WinRate:    result.WinRate,
			ELOStart:   result.ELOStart,
			ELOEnd:     result.ELOEnd,
			ELODelta:   result.ELODelta,
			AvgTurns:   result.AvgTurns,
			Place1st:   result.Placements[0],
			Place2nd:   result.Placements[1],
			Place3rd:   result.Placements[2],
			Place4th:   result.Placements[3],
		}
		var err error
		if result.RunID > 0 {
			err = db.FinalizeGauntletRun(context.Background(), sm.sqlDB, rec)
		} else {
			err = db.InsertGauntletRun(context.Background(), sm.sqlDB, rec)
		}
		if err != nil {
			log.Printf("gauntlet_runs persist: %v", err)
		}
	}

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
			HexRating: e.hexRating,
			Games:     e.games,
			Wins:      e.wins,
			Losses:    e.games - e.wins,
			Delta:     e.delta,
			HexDelta:  e.hexDelta,
			Bracket:   e.bracket,
			TSMu:      e.tsMu,
			TSSigma:   e.tsSigma,
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

// Shutdown flushes all buffered state to disk. Call on graceful shutdown.
func (sm *Showmatch) Shutdown() {
	sm.flushELO()
	if sm.heimdall != nil {
		sm.heimdall.Flush()
	}
	if sm.muninnSink != nil {
		if err := sm.muninnSink.Close(); err != nil {
			log.Printf("muninn: close error: %v", err)
		}
	}
	sm.curseMu.Lock()
	pools := sm.cursePool
	sm.curseMu.Unlock()
	if len(pools) > 0 {
		if err := hat.SaveAllPools(sm.curseDir, pools); err != nil {
			log.Printf("curse: save error: %v", err)
		} else {
			log.Printf("curse: saved %d pools to %s", len(pools), sm.curseDir)
		}
	}
}

func (sm *Showmatch) eloSize() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.elo)
}

func convertEngineEvents(gs *gameengine.GameState, fromIdx int, turn int) []hat.GameEvent {
	var out []hat.GameEvent
	for i := fromIdx; i < len(gs.EventLog); i++ {
		ev := gs.EventLog[i]
		switch ev.Kind {
		case "damage":
			out = append(out, hat.GameEvent{
				Turn: turn, Seat: ev.Seat, Kind: "damage",
				Source: ev.Source, Amount: ev.Amount,
			})
		case "seat_eliminated":
			out = append(out, hat.GameEvent{
				Turn: turn, Seat: ev.Seat, Kind: "player_lost",
			})
		case "cast":
			out = append(out, hat.GameEvent{
				Turn: turn, Seat: ev.Seat, Kind: "cast_spell",
				Source: ev.Source,
			})
		case "zone_change":
			if ev.Source != "" && strings.Contains(strings.ToLower(ev.Source), "land") {
				out = append(out, hat.GameEvent{
					Turn: turn, Seat: ev.Seat, Kind: "enter_battlefield",
					Source: ev.Source,
				})
			}
		}
	}
	return out
}

func extractEvalScores(gs *gameengine.GameState) []float64 {
	scores := make([]float64, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		if ygg, ok := s.Hat.(*hat.YggdrasilHat); ok && ygg.Evaluator != nil {
			scores[i] = ygg.Evaluator.Evaluate(gs, i)
		}
	}
	return scores
}

// seatPlacement derives a 1-based finish position for a seat from game state.
// Winner = 1st. Remaining seats ranked by life total (higher = better).
func seatPlacement(gs *gameengine.GameState, seatIdx, winner int) int {
	if seatIdx == winner {
		return 1
	}
	if gs == nil {
		return 4
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost || seat.LeftGame {
		// Lost seats: rank by life (all ≤0). Dead earlier = worse.
		rank := 2 // start at 2nd (winner is 1st)
		for i, s := range gs.Seats {
			if i == seatIdx || i == winner || s == nil {
				continue
			}
			if !s.Lost && !s.LeftGame {
				rank++ // surviving non-winners beat us
			} else if s.Life > seat.Life {
				rank++ // dead players with more life beat us
			}
		}
		if rank > len(gs.Seats) {
			rank = len(gs.Seats)
		}
		return rank
	}
	// Alive non-winner: rank by life among survivors.
	rank := 2
	for i, s := range gs.Seats {
		if i == seatIdx || i == winner || s == nil || s.Lost || s.LeftGame {
			continue
		}
		if s.Life > seat.Life {
			rank++
		}
	}
	return rank
}

// getOrCreateCursePool returns the genetic pool for a deck, creating one lazily.
// Thread-safe; the returned pool's internal rng is set from the provided rng.
//
// New pools are seeded via cross-deck transfer when an evolved pool with
// a similar archetype/colors/themes exists — the new deck's DNA inherits
// from the closest match (with wider-than-mutation gaussian noise) so it
// converges in ~10 games instead of ~100.
func (sm *Showmatch) getOrCreateCursePool(deckKey string, rng *rand.Rand) *hat.CursePool {
	sm.curseMu.Lock()
	defer sm.curseMu.Unlock()
	if pool, ok := sm.cursePool[deckKey]; ok {
		return pool
	}
	poolRng := rand.New(rand.NewSource(rng.Int63()))
	bracket := 0
	if sp := sm.strategies[deckKey]; sp != nil {
		bracket = sp.Bracket
	}
	p := hat.InitPoolWithTransfer(deckKey, bracket, poolRng, sm.cursePool, sm.strategies)
	sm.cursePool[deckKey] = &p
	return &p
}

func (sm *Showmatch) runOneGameFast(rng *rand.Rand) {
	var deckKeys []string // declared here so crash recovery can access it
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			log.Printf("grinder: game crashed: %v\n%s", r, buf[:n])
			if sm.heimdall != nil {
				sm.heimdall.RecordCrash(fmt.Sprintf("%v", r), buf[:n], deckKeys)
			}
		}
	}()

	sm.mu.RLock()
	poolSize := len(sm.deckPool)
	sm.mu.RUnlock()

	if poolSize < showmatchSeats {
		time.Sleep(5 * time.Second)
		return
	}

	// Build matchmaking pool from ELO state for rating-aware pod assembly.
	sm.mu.RLock()
	pool := make([]matchmaking.DeckEntry, poolSize)
	for i := 0; i < poolSize; i++ {
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
		pool[i] = matchmaking.DeckEntry{Index: i, Commander: sm.deckPool[i].CommanderName, Mu: mu, Sigma: sigma, Games: games, Bracket: bracket}
	}
	sm.mu.RUnlock()

	indices := matchmaking.AssembleBracketPod(rng, pool, showmatchSeats)
	pickedDecks := make([]*deckparser.TournamentDeck, showmatchSeats)
	commanders := make([]string, showmatchSeats)
	deckKeys = make([]string, showmatchSeats)
	for i, idx := range indices {
		pickedDecks[i] = sm.deckPool[idx]
		commanders[i] = pickedDecks[i].CommanderName
		deckKeys[i] = deckKeyFromPath(pickedDecks[i].Path)
	}

	gameSeed := time.Now().UnixNano()
	gameRng := rand.New(rand.NewSource(gameSeed))
	gs := gameengine.NewGameState(showmatchSeats, gameRng, sm.corpus)
	gs.Seed = gameSeed
	gs.EventPolicy = gameengine.EventLogNone
	// Hex Judge sampled watchdog (nil-safe no-op when off).
	jr := sm.judge.beginGame(rng, gameSeed, deckKeys)
	jr.Attach(gs)

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

	// Curse: select DNA for each seat's hat.
	var dnaIdxes [showmatchSeats]int
	var bracketHats [showmatchSeats]*hat.YggdrasilHat
	for i := 0; i < showmatchSeats; i++ {
		pool := sm.getOrCreateCursePool(deckKeys[i], rng)
		sm.curseMu.Lock()
		dna, dnaIdx := pool.SelectForGame()
		dnaCopy := *dna
		dimStats := pool.DimStats
		sm.curseMu.Unlock()
		dnaIdxes[i] = dnaIdx
		h := hat.NewYggdrasilHatWithPool(&dnaCopy, sm.strategies[deckKeys[i]], 50, &dimStats)
		h.NeuralEval = sm.neuralEval
		gs.Seats[i].Hat = h
		bracketHats[i] = h
	}

	for i := 0; i < showmatchSeats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}

	gs.Active = gameRng.Intn(showmatchSeats)
	gs.Turn = 1

	var bracketCollectors [showmatchSeats]*hat.TrainingCollector
	for i := 0; i < showmatchSeats; i++ {
		bracketCollectors[i] = hat.NewTrainingCollector(5)
	}
	bracketEvalCollector := hat.NewEvalSnapshotCollector()

	bracketETBs := make(map[int][]string)
	var heapBaseline uint64
	{
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		heapBaseline = m.HeapAlloc
	}
	const maxHeapPerGame = 2 * 1024 * 1024 * 1024 // 2GB per game
	for turn := 1; turn <= showmatchMaxTurn; turn++ {
		// r62 (report 06 C3): abort unrated on a mid-game maintenance flip
		// (same contract as the fishtank loop — see runOneGame).
		if sm.inMaintenance() {
			log.Printf("grinder: game aborted at turn %d — maintenance mode (unrated)", turn)
			return
		}
		gs.Turn = turn
		preBF := heimdall.SnapshotBattlefieldNames(gs)
		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)
		jr.AfterTurn(gs)
		for i := 0; i < showmatchSeats; i++ {
			bracketCollectors[i].Snapshot(gs, i)
		}
		if turn%5 == 0 || turn == 1 {
			bracketEvalCollector.Record(turn, extractEvalScores(gs))
		}
		if newCards := heimdall.DiffBattlefield(preBF, heimdall.SnapshotBattlefieldNames(gs)); len(newCards) > 0 {
			bracketETBs[turn] = newCards
		}

		if gs.CheckEnd() {
			break
		}
		if turn%10 == 0 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > heapBaseline+maxHeapPerGame {
				log.Printf("grinder: aborting game at turn %d — heap delta %.0fMB exceeds budget", turn, float64(m.HeapAlloc-heapBaseline)/1e6)
				break
			}
		}
		// CR §720 extra turns (LIFO) + skip-your-next-turn via the shared
		// sequencing helper; gs.Turn is re-stamped next iteration.
		gameengine.AdvanceActiveSeat(gs)
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

	// Per-card win/loss + first-played telemetry. firstPlayed is built
	// from bracketETBs (battlefield-diff captured every turn) — not
	// perfect (a card recursed back into play counts as "first") but
	// cheap and good enough for empirical card-page analytics. Indexed
	// by lowercased name so it round-trips through cardstats.Record.
	firstPlayed := make(map[string]int, 64)
	turns := make([]int, 0, len(bracketETBs))
	for t := range bracketETBs {
		turns = append(turns, t)
	}
	sort.Ints(turns)
	for _, t := range turns {
		for _, name := range bracketETBs[t] {
			key := strings.ToLower(strings.TrimSpace(name))
			if key == "" {
				continue
			}
			if _, seen := firstPlayed[key]; !seen {
				firstPlayed[key] = t
			}
		}
	}
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
		cardstats.Default.Record(names, i == winner, firstPlayed)
	}

	// Curse: record results + dimension stats for each seat's DNA variant.
	sm.curseMu.Lock()
	var evolved []*hat.CursePool
	for i := 0; i < showmatchSeats; i++ {
		if pool, ok := sm.cursePool[deckKeys[i]]; ok {
			score := hat.PlacementScore(seatPlacement(gs, i, winner), showmatchSeats)
			prevCount := pool.GameCount
			pool.RecordResult(dnaIdxes[i], score)
			if bracketHats[i] != nil {
				pool.DimStats.RecordGame(bracketHats[i].DimensionMeans(), score)
			}
			if pool.GameCount < prevCount {
				evolved = append(evolved, pool)
			}
		}
	}
	sm.curseMu.Unlock()
	for _, pool := range evolved {
		go hat.SavePool(sm.curseDir, pool)
	}

	sm.mu.Lock()
	sm.stats.gamesPlayed++
	sm.stats.totalTurns += gs.Turn
	bracketPriorEffects := sm.updateELO(deckKeys, commanders, pickedDecks, winner, gs.Turn)
	sm.mu.Unlock()

	bracketSeed := heimdall.GameSeed{
		RNGSeed:    gameSeed,
		DeckKeys:   [4]string{deckKeys[0], deckKeys[1], deckKeys[2], deckKeys[3]},
		Winner:     winner,
		Turns:      gs.Turn,
		KillMethod: heimdall.ClassifyKillWithMaxTurns(gs, winner, showmatchMaxTurn),
	}
	if sm.heimdall != nil {
		sm.heimdall.RecordSeed(bracketSeed)
		sm.heimdall.RecordObservation(heimdall.Observation{
			Seed:                    bracketSeed,
			ParserGaps:              heimdall.ExtractParserGaps(gs),
			CoTriggers:              heimdall.ExtractCoTriggers(bracketETBs),
			CardFirstPlayed:         gs.CardFirstPlayed,
			CompositionPriorEffects: bracketPriorEffects,
		})
	}

	// Tesla: extract causal pivot and enrich training samples.
	bracketPivot := hat.ExtractPivot(bracketEvalCollector.History(), winner, gs.Turn)
	for i := 0; i < showmatchSeats; i++ {
		placement := hat.PlacementScore(seatPlacement(gs, i, winner), showmatchSeats)
		samples := bracketCollectors[i].Finalize(placement, gs.Turn)
		if len(samples) > 0 {
			labels := hat.LabelSamplesWithPivot(samples, bracketPivot)
			enriched := hat.EnrichSamples(samples, labels)
			_ = sm.selfPlay.WriteEnrichedSamples(
				filepath.Join(sm.trainingDir, "samples.jsonl"), enriched)
		}
	}

	// Feynman: post-game invariant check.
	bracketOracle := hat.CheckGame(gs)
	// Authoritative InstanceID census (control-transfer-robust ZoneConservation).
	if invs := gameengine.RunAllInvariants(gs); len(invs) > 0 {
		for _, v := range invs {
			log.Printf("invariant[%s]: %s", v.Name, v.Message)
		}
	}
	if !bracketOracle.Clean() {
		log.Printf("feynman: bracket game violations: %s",
			hat.FormatViolations(bracketOracle.Violations))
		sm.muninnSink.AutoArchive(bracketSeed.RNGSeed, bracketSeed.DeckKeys, bracketOracle.Violations)
	}
	jr.Finish(gs, bracketOracle.Violations, pickedDecks)
	sm.muninnSink.EndGame()
}

func (sm *Showmatch) runOneGame(rng *rand.Rand) {
	var deckKeys []string // declared here so crash recovery can access it
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			log.Printf("showmatch: game crashed: %v\n%s", r, buf[:n])
			if sm.heimdall != nil {
				sm.heimdall.RecordCrash(fmt.Sprintf("%v", r), buf[:n], deckKeys)
			}
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
	deckKeys = make([]string, showmatchSeats)
	for i, idx := range indices {
		pickedDecks[i] = sm.deckPool[idx]
		commanders[i] = pickedDecks[i].CommanderName
		deckKeys[i] = deckKeyFromPath(pickedDecks[i].Path)
	}

	gameSeed := time.Now().UnixNano()
	gameRng := rand.New(rand.NewSource(gameSeed))
	gs := gameengine.NewGameState(showmatchSeats, gameRng, sm.corpus)
	gs.Seed = gameSeed
	gs.EventPolicy = gameengine.EventLogFull
	// Hex Judge sampled watchdog (nil-safe no-op when off).
	jr := sm.judge.beginGame(rng, gameSeed, deckKeys)
	jr.Attach(gs)

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

	// Attach Yggdrasil hats with Curse DNA + learned dimension corrections.
	var showDnaIdxes [showmatchSeats]int
	var showHats [showmatchSeats]*hat.YggdrasilHat
	for i := 0; i < showmatchSeats; i++ {
		pool := sm.getOrCreateCursePool(deckKeys[i], rng)
		sm.curseMu.Lock()
		dna, dnaIdx := pool.SelectForGame()
		dnaCopy := *dna
		dimStats := pool.DimStats
		sm.curseMu.Unlock()
		showDnaIdxes[i] = dnaIdx
		h := hat.NewYggdrasilHatWithPool(&dnaCopy, sm.strategies[deckKeys[i]], 50, &dimStats)
		h.NeuralEval = sm.neuralEval
		gs.Seats[i].Hat = h
		showHats[i] = h
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
	var showCollectors [showmatchSeats]*hat.TrainingCollector
	for i := 0; i < showmatchSeats; i++ {
		showCollectors[i] = hat.NewTrainingCollector(5)
	}
	showEvalCollector := hat.NewEvalSnapshotCollector()
	var showGameEvents []hat.GameEvent
	showETBs := make(map[int][]string)
	// timeline accumulates a TurnSnapshot per executed turn for the
	// replay viewer. Attached to CompletedGame below; in-memory only
	// (SQLite game schema doesn't persist per-turn detail).
	var timeline []TurnSnapshot
	for turn := 1; turn <= showmatchMaxTurn; turn++ {
		// r62 (report 06 C3): a maintenance flip mid-game halts the game
		// between turns. Returning here skips the entire end-of-game
		// block — no updateELO, no gameHistory append, no persist
		// enqueue, no curse/training feeds — so an in-flight game can
		// never corrupt the data maintenance is meant to protect.
		if sm.inMaintenance() {
			log.Printf("fishtank: game %d aborted at turn %d — maintenance mode (unrated)", gameNum, turn)
			return
		}
		gs.Turn = turn

		preBF := heimdall.SnapshotBattlefieldNames(gs)
		tournament.TakeTurnWithHook(gs, phaseHook)
		gameengine.StateBasedActions(gs)
		jr.AfterTurn(gs)
		if newCards := heimdall.DiffBattlefield(preBF, heimdall.SnapshotBattlefieldNames(gs)); len(newCards) > 0 {
			showETBs[turn] = newCards
		}
		for i := 0; i < showmatchSeats; i++ {
			showCollectors[i].Snapshot(gs, i)
		}
		if turn%5 == 0 || turn == 1 {
			showEvalCollector.Record(turn, extractEvalScores(gs))
		}

		showGameEvents = append(showGameEvents, convertEngineEvents(gs, lastEventIdx, turn)...)
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

		timeline = append(timeline, buildTurnSnapshot(turn, gs.Active, snap.Seats, newEntries))

		if gs.CheckEnd() {
			break
		}

		// CR §720 extra turns (LIFO) + skip-your-next-turn via the shared
		// sequencing helper; gs.Turn is re-stamped next iteration.
		gameengine.AdvanceActiveSeat(gs)
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

	// win_reason captures HOW the winner won (orthogonal to endReason,
	// the game-end TYPE). Plain ClassifyKill so it's always a kill
	// method; the turn-cap case is already captured by endReason.
	winReason := ""
	if winner >= 0 {
		winReason = "win_" + heimdall.ClassifyKill(gs, winner)
	}

	// Curse: record results + dimension stats for showmatch seats.
	sm.curseMu.Lock()
	var showEvolved []*hat.CursePool
	for i := 0; i < showmatchSeats; i++ {
		if pool, ok := sm.cursePool[deckKeys[i]]; ok {
			score := hat.PlacementScore(seatPlacement(gs, i, winner), showmatchSeats)
			prevCount := pool.GameCount
			pool.RecordResult(showDnaIdxes[i], score)
			if showHats[i] != nil {
				pool.DimStats.RecordGame(showHats[i].DimensionMeans(), score)
			}
			if pool.GameCount < prevCount {
				showEvolved = append(showEvolved, pool)
			}
		}
	}
	sm.curseMu.Unlock()
	for _, pool := range showEvolved {
		go hat.SavePool(sm.curseDir, pool)
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
	showPriorEffects := sm.updateELO(deckKeys, commanders, pickedDecks, winner, gs.Turn)

	completed := CompletedGame{
		GameID:     gameNum,
		Commanders: commanders,
		DeckKeys:   deckKeys,
		Winner:     winner,
		WinnerName: safeCommander(commanders, winner),
		Turns:      gs.Turn,
		EndReason:  endReason,
		WinReason:  winReason,
		FinishedAt: time.Now(),
		FinalSeats: finalSnap.Seats,
		RngSeed:    gs.Seed,
		Timeline:   timeline,
	}
	sm.gameHistory = append(sm.gameHistory, completed)
	if len(sm.gameHistory) > 50 {
		sm.gameHistory = sm.gameHistory[len(sm.gameHistory)-50:]
	}

	sm.mu.Unlock()

	showSeed := heimdall.GameSeed{
		RNGSeed:    gameSeed,
		DeckKeys:   [4]string{deckKeys[0], deckKeys[1], deckKeys[2], deckKeys[3]},
		Winner:     winner,
		Turns:      gs.Turn,
		KillMethod: heimdall.ClassifyKillWithMaxTurns(gs, winner, showmatchMaxTurn),
	}
	if sm.heimdall != nil {
		sm.heimdall.RecordSeed(showSeed)
		sm.heimdall.RecordObservation(heimdall.Observation{
			Seed:                    showSeed,
			ParserGaps:              heimdall.ExtractParserGaps(gs),
			CoTriggers:              heimdall.ExtractCoTriggers(showETBs),
			CardFirstPlayed:         gs.CardFirstPlayed,
			CompositionPriorEffects: showPriorEffects,
		})
	}

	// Tesla: extract causal pivot and enrich training samples.
	showPivot := hat.ExtractPivot(showEvalCollector.History(), winner, gs.Turn)
	for i := 0; i < showmatchSeats; i++ {
		placement := hat.PlacementScore(seatPlacement(gs, i, winner), showmatchSeats)
		samples := showCollectors[i].Finalize(placement, gs.Turn)
		if len(samples) > 0 {
			labels := hat.LabelSamplesWithPivot(samples, showPivot)
			enriched := hat.EnrichSamples(samples, labels)
			_ = sm.selfPlay.WriteEnrichedSamples(
				filepath.Join(sm.trainingDir, "samples.jsonl"), enriched)
		}
	}

	// Feynman: post-game invariant check.
	showOracle := hat.CheckGame(gs)
	// Authoritative InstanceID census (control-transfer-robust ZoneConservation).
	if invs := gameengine.RunAllInvariants(gs); len(invs) > 0 {
		for _, v := range invs {
			log.Printf("invariant[%s]: %s", v.Name, v.Message)
		}
	}
	if !showOracle.Clean() {
		log.Printf("feynman: showmatch game %d violations: %s",
			gameNum, hat.FormatViolations(showOracle.Violations))
		sm.muninnSink.AutoArchive(showSeed.RNGSeed, showSeed.DeckKeys, showOracle.Violations)
	}
	jr.Finish(gs, showOracle.Violations, pickedDecks)
	sm.muninnSink.EndGame()

	// Ive: compose three-act narrative for spectators.
	narrative := hat.ComposeNarrative(showPivot, showGameEvents, commanders, winner, gs.Turn)
	sm.broadcastToSpectators(wsEnvelope{Type: "narrative", Payload: narrative})

	select {
	case sm.persistCh <- persistJob{game: completed, perfDeltas: cardPerformanceDeltas(gs, winner)}:
	default:
	}

	// Fan out the game.end event to caller-registered webhooks (no-op
	// when sm.Webhooks is nil). FireGameEnd spawns one goroutine per
	// subscriber and returns immediately — the persist tail never
	// waits on an external HTTP receiver.
	if sm.Webhooks != nil {
		sm.Webhooks.FireGameEnd(context.Background(), completed)
	}

	// Publish the same shape to the in-process events bus that backs
	// /ws/events. Non-blocking — Publish drops to subscribers whose
	// buffer is full rather than stalling the persist tail.
	if sm.Events != nil {
		sm.Events.Publish(Event{
			Type: EventGameEnd,
			Data: map[string]any{
				"game_id":     completed.GameID,
				"winner":      completed.Winner,
				"winner_name": completed.WinnerName,
				"commanders":  completed.Commanders,
				"deck_keys":   completed.DeckKeys,
				"turns":       completed.Turns,
				"end_reason":  completed.EndReason,
				"finished_at": completed.FinishedAt.UTC().Format(time.RFC3339),
			},
		})
	}

	log.Printf("showmatch: game %d finished — turn %d, winner: %s (%s), pivot: t%d (Δ%.2f)",
		gameNum, gs.Turn, safeCommander(commanders, winner), endReason,
		showPivot.Turn, showPivot.DeltaScore)

	sm.broadcastToSpectators(wsEnvelope{Type: "game", Payload: finalSnap})
	sm.broadcastToSpectators(wsEnvelope{Type: "stats", Payload: sm.GetStats()})
	sm.broadcastToSpectators(wsEnvelope{Type: "elo", Payload: sm.GetELO()})
}

// persistGauntletGame writes one gauntlet game's outcome to
// showmatch_game + showmatch_game_seat in a single transaction.
// Returns the new game_id so the caller can link it to a
// gauntlet_runs row via the gauntlet_run_game junction.
//
// Distinct from persistGame() which consumes a CompletedGame
// summary from the continuous-game pipeline; this variant works
// directly off the live gameengine.GameState at end-of-game so
// the gauntlet path doesn't have to materialize an intermediate
// snapshot. Card-win-stat aggregation is intentionally omitted
// — the continuous-game pipeline keeps that aggregate fresh, and
// duplicating per-card writes here would distort the count.
func (sm *Showmatch) persistGauntletGame(
	deckKeys, commanders []string,
	gs *gameengine.GameState,
	gameSeed int64,
	winner int,
) (int64, error) {
	if sm.sqlDB == nil || gs == nil {
		return 0, nil
	}
	now := time.Now()
	winnerName := ""
	if winner >= 0 && winner < len(commanders) {
		winnerName = commanders[winner]
	} else {
		winnerName = "DRAW"
	}
	endReason := "unknown"
	if gs.Flags != nil {
		if r, ok := gs.Flags["end_reason"]; ok {
			switch r {
			case 1:
				endReason = "last_seat_standing"
			case 2:
				endReason = "turn_cap"
			case 3:
				endReason = "turn_cap_tie"
			}
		}
	}
	winReason := ""
	if winner >= 0 {
		winReason = "win_" + heimdall.ClassifyKill(gs, winner)
	}
	gameRec := db.GameRecord{
		StartedAt:  now.Add(-time.Duration(gs.Turn) * 3600 * time.Millisecond).Unix(),
		FinishedAt: now.Unix(),
		Turns:      gs.Turn,
		Winner:     winner,
		WinnerName: winnerName,
		EndReason:  endReason,
		WinReason:  winReason,
		Seed:       gameSeed,
	}
	seats := make([]db.GameSeatRecord, 0, len(gs.Seats))
	for i, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		cmdr := ""
		if i < len(commanders) {
			cmdr = commanders[i]
		}
		dk := ""
		if i < len(deckKeys) {
			dk = deckKeys[i]
		}
		bfNames := make([]string, 0, len(seat.Battlefield))
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			bfNames = append(bfNames, p.Card.Name)
		}
		bfJSON, _ := json.Marshal(bfNames)
		seats = append(seats, db.GameSeatRecord{
			Seat:             i,
			Commander:        cmdr,
			DeckKey:          dk,
			Life:             seat.Life,
			HandSize:         len(seat.Hand),
			LibrarySize:      len(seat.Library),
			GYSize:           len(seat.Graveyard),
			BFSize:           len(seat.Battlefield),
			Lost:             seat.Lost,
			BattlefieldCards: string(bfJSON),
		})
	}
	return db.PersistGameTx(context.Background(), sm.sqlDB, gameRec, seats)
}

func (sm *Showmatch) persistGame(g CompletedGame) {
	sm.recordAchievements(g)
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
		WinReason:  g.WinReason,
		Seed:       g.RngSeed,
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

	// Deck-list-based per-card aggregate (cross-commander).
	// For every card in each seat's deck list, count one game and tag
	// it as win or loss based on whether that seat won. This drives
	// /api/cards/{name}/stats. Distinct from the battlefield-presence
	// accounting in card_win_stats above.
	var deckCardDeltas []db.CardStatDelta
	for i, dk := range g.DeckKeys {
		if dk == "" {
			continue
		}
		owner, id := splitDeckKey(dk)
		if owner == "" {
			continue
		}
		td := sm.findDeckInPool(owner, id)
		if td == nil {
			continue
		}
		isWinner := i == g.Winner
		// Dedupe within a single deck — duplicates in the deck list
		// (basics, common spells) shouldn't multi-count the same game.
		seen := make(map[string]struct{}, len(td.Library)+len(td.CommanderCards))
		add := func(name string) {
			if name == "" {
				return
			}
			if _, ok := seen[name]; ok {
				return
			}
			seen[name] = struct{}{}
			d := db.CardStatDelta{CardName: name}
			if isWinner {
				d.Win = 1
			} else {
				d.Loss = 1
			}
			deckCardDeltas = append(deckCardDeltas, d)
		}
		for _, c := range td.CommanderCards {
			if c != nil {
				add(c.Name)
			}
		}
		for _, c := range td.Library {
			if c != nil {
				add(c.Name)
			}
		}
	}
	if len(deckCardDeltas) > 0 {
		if err := db.BatchUpsertCardStats(context.Background(), sm.sqlDB, deckCardDeltas); err != nil {
			log.Printf("showmatch: card_stats: %v", err)
		}
	}
}

// cardPerformanceDeltas walks each seat's Battlefield, Hand, and
// Graveyard zones at game end and returns one delta per (seat, card)
// pair: the card was at least drawn during the game. Win is set when
// the holding seat won. TurnPlayed comes from gs.CardFirstPlayed (0
// for cards that were drawn but never cast). BattlefieldTurns
// approximates "time on the battlefield" as gs.Turn - first-played-turn
// for cards still on the battlefield at game end; we don't have
// per-permanent ETB-turn instrumentation yet, so churned permanents
// don't contribute (intentional underestimate, not a wrong number).
func cardPerformanceDeltas(gs *gameengine.GameState, winnerSeat int) []db.CardPerformanceDelta {
	if gs == nil {
		return nil
	}
	var out []db.CardPerformanceDelta
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		seen := map[string]bool{}
		bfNames := map[string]bool{}

		add := func(name string) {
			if name == "" || seen[name] {
				return
			}
			seen[name] = true
			d := db.CardPerformanceDelta{CardName: name}
			if seatIdx == winnerSeat {
				d.Win = 1
			}
			if turn, ok := gs.CardFirstPlayed[name]; ok && turn > 0 {
				d.TurnPlayed = turn
				if bfNames[name] && gs.Turn >= turn {
					d.BattlefieldTurns = gs.Turn - turn
					if d.BattlefieldTurns == 0 {
						d.BattlefieldTurns = 1 // ETB this turn → at least 1
					}
				}
			}
			out = append(out, d)
		}

		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			bfNames[p.Card.DisplayName()] = true
		}
		for name := range bfNames {
			add(name)
		}
		for _, c := range s.Hand {
			if c != nil {
				add(c.DisplayName())
			}
		}
		for _, c := range s.Graveyard {
			if c != nil {
				add(c.DisplayName())
			}
		}
	}
	return out
}

// splitDeckKey splits "owner/id" into its parts. Returns ("","") on
// malformed input rather than panicking — callers can skip silently.
func splitDeckKey(k string) (string, string) {
	for i := 0; i < len(k); i++ {
		if k[i] == '/' {
			return k[:i], k[i+1:]
		}
	}
	return "", ""
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
			LossReason:  s.LossReason,
		}
		// R60 spectator UX: surface per-color mana so the UI can render
		// WUBRG pips instead of the legacy single-int total. Skip when
		// every bucket is zero so the omitempty tag elides the field
		// during the no-floating-mana steady state.
		if mp := buildManaPoolByColor(s.Mana); len(mp) > 0 {
			ss.ManaPoolByColor = mp
		}
		// R60 spectator UX: surface the command zone so the UI can
		// render per-seat commander cards + the §903.8 cast-tax count.
		if cz := buildCommandZone(s.CommandZone, s.CommanderCastCounts); len(cz) > 0 {
			ss.CommandZone = cz
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
			// R60 spectator UX: surface counter-on-permanent state so the
			// UI can overlay +1/+1, loyalty, charge, etc. on perm tiles.
			// Only carry non-zero counters across the wire to keep
			// payloads small (most permanents have zero counters).
			if len(p.Counters) > 0 {
				cs := map[string]int{}
				for k, v := range p.Counters {
					if v != 0 {
						cs[k] = v
					}
				}
				if len(cs) > 0 {
					ps.Counters = cs
				}
			}
			// Phase 9 lineage: pull from the Card (per-instance identity
			// lives on Card, not Permanent — copy/mutate swaps Card while
			// keeping the Permanent slot alive).
			if p.Card.InstanceID != "" {
				ps.InstanceID = p.Card.InstanceID
				ps.Provenance = p.Card.Provenance.String()
				ps.SourceInstanceID = p.Card.SourceInstanceID
				ps.EnablerInstanceID = p.Card.EnablerInstanceID
				if len(p.Card.EnablerHistory) > 0 {
					ps.EnablerHistory = append([]string(nil), p.Card.EnablerHistory...)
				}
				if len(p.MergedCards) > 0 {
					ps.MergedCardIDs = append([]string(nil), p.MergedCards...)
				}
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
			es := &EvalSnapshot{
				Score:             r.Score,
				BoardPresence:     r.BoardPresence,
				CardAdvantage:     r.CardAdvantage,
				ManaAdvantage:     r.ManaAdvantage,
				LifeResource:      r.LifeResource,
				ComboProximity:    r.ComboProximity,
				ThreatExposure:    r.ThreatExposure,
				CommanderProgress: r.CommanderProgress,
				GraveyardValue:    r.GraveyardValue,
				Budget:            ygg.Budget,
				BudgetUsed:        ygg.EvalsSpent(),
			}
			if ygg.Strategy != nil {
				es.Archetype = string(ygg.Strategy.Archetype)
			}
			ss.Eval = es
		}
		snap.Seats[i] = ss
	}
	// Phase 9: build the lineage index covering every card in every zone
	// that has a minted InstanceID. The endpoint /api/spectator/lineage/:id
	// reads this map; Heimdall walks it via BuildLineageTree.
	snap.Lineage = buildLineageIndex(gs)
	// R60 spectator UX: surface the resolution stack so the UI can
	// show what's mid-cast / pending response.
	snap.Stack = buildStackSnapshot(gs.Stack)
	// R60 spectator UX: scan the event log for new `infinite_cycle`
	// entries since the last tick and fan them out as combo_fired
	// events on the bus so the spectator UI can render a toast.
	sm.publishNewCombos(gs, gameNum, commanders)
	return snap
}

// buildManaPoolByColor flattens a ColoredManaPool into the
// JSON-serializable map<color,count> shape the spectator UI consumes.
// Restricted mana rolls into the "Any" bucket — spectators don't need
// the spend-time restriction surfaced. Returns nil when every bucket
// is zero so the SeatSnapshot's omitempty tag elides the field.
func buildManaPoolByColor(p *gameengine.ColoredManaPool) map[string]int {
	if p == nil {
		return nil
	}
	out := map[string]int{}
	if p.W > 0 {
		out["W"] = p.W
	}
	if p.U > 0 {
		out["U"] = p.U
	}
	if p.B > 0 {
		out["B"] = p.B
	}
	if p.R > 0 {
		out["R"] = p.R
	}
	if p.G > 0 {
		out["G"] = p.G
	}
	if p.C > 0 {
		out["C"] = p.C
	}
	anySum := p.Any
	for _, r := range p.Restricted {
		anySum += r.Amount
	}
	if anySum > 0 {
		out["Any"] = anySum
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildCommandZone flattens a seat's command-zone card slice + the
// §903.8 cast-count map into the JSON-serializable shape the
// spectator UI consumes. Returns nil when the zone is empty so the
// SeatSnapshot's omitempty tag elides the field during the
// commander-on-battlefield steady state.
func buildCommandZone(zone []*gameengine.Card, castCounts map[string]int) []CommandZoneEntry {
	if len(zone) == 0 {
		return nil
	}
	out := make([]CommandZoneEntry, 0, len(zone))
	for _, c := range zone {
		if c == nil {
			continue
		}
		name := c.DisplayName()
		entry := CommandZoneEntry{Name: name}
		if castCounts != nil {
			entry.CastCount = castCounts[name]
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildStackSnapshot resolves engine StackItems into display-ready
// snapshots: surfaces the source name (card for spells, source
// permanent for activated/triggered abilities), controller seat,
// stack-item kind, and copy/countered flags. Empty input → nil so
// the GameSnapshot's omitempty tag elides the field during the
// no-stack steady state (between resolutions / outside cast
// windows).
func buildStackSnapshot(stack []*gameengine.StackItem) []StackItemSnapshot {
	if len(stack) == 0 {
		return nil
	}
	out := make([]StackItemSnapshot, 0, len(stack))
	for _, item := range stack {
		if item == nil {
			continue
		}
		name := ""
		if item.Card != nil {
			name = item.Card.DisplayName()
		} else if item.Source != nil && item.Source.Card != nil {
			name = item.Source.Card.DisplayName()
		}
		out = append(out, StackItemSnapshot{
			Source:     name,
			Controller: item.Controller,
			Kind:       item.Kind,
			IsCopy:     item.IsCopy,
			Countered:  item.Countered,
		})
	}
	return out
}

// publishNewCombos scans gs.EventLog for `infinite_cycle` events
// past the per-game scan cursor and publishes one `game.combo_fired`
// event to the EventBus for each. The cursor is keyed by gameNum so
// a new game resets the scan position to 0; within a game, repeat
// snapshot ticks (which can fire as the engine resolves a long
// stack containing the same cycle) skip already-seen events.
//
// No-ops when sm.Events is nil (feature disabled / test path),
// gs is nil, or the engine emits no infinite_cycle events. Safe
// to call from any captureSnapshot caller — the bus drops
// publishes when subscribers' buffers are full so a slow
// spectator can't backpressure the engine.
func (sm *Showmatch) publishNewCombos(gs *gameengine.GameState, gameNum int, commanders []string) {
	if sm == nil || sm.Events == nil || gs == nil {
		return
	}
	sm.mu.Lock()
	if sm.lastComboCursor == nil {
		sm.lastComboCursor = make(map[int]int)
	}
	cursor := sm.lastComboCursor[gameNum]
	logLen := len(gs.EventLog)
	if cursor > logLen {
		// Defensive: event log was truncated (shouldn't happen mid-game
		// but if it does, restart the scan from 0 for this game).
		cursor = 0
	}
	sm.mu.Unlock()

	for i := cursor; i < logLen; i++ {
		ev := gs.EventLog[i]
		if ev.Kind != "infinite_cycle" {
			continue
		}
		data := map[string]any{
			"game_id": gameNum,
			"turn":    gs.Turn,
		}
		if ev.Seat >= 0 {
			data["seat"] = ev.Seat
			if ev.Seat < len(commanders) {
				data["controller_commander"] = commanders[ev.Seat]
			}
		}
		if ev.Details != nil {
			if v, ok := ev.Details["cycle_length"].(int); ok {
				data["cycle_length"] = v
			} else if ev.Amount > 0 {
				data["cycle_length"] = ev.Amount
			}
			if v, ok := ev.Details["participating_names"].([]string); ok && len(v) > 0 {
				data["participating_cards"] = append([]string(nil), v...)
			}
			if v, ok := ev.Details["participating_iids"].([]string); ok && len(v) > 0 {
				data["participating_iids"] = append([]string(nil), v...)
			}
			if v, ok := ev.Details["detected_by"].(string); ok && v != "" {
				data["detected_by"] = v
			}
		}
		sm.Events.Publish(Event{
			Type:      EventComboFired,
			Data:      data,
			Timestamp: time.Now().UTC(),
		})
	}

	sm.mu.Lock()
	sm.lastComboCursor[gameNum] = logLen
	sm.mu.Unlock()
}

// buildLineageIndex scans every zone on every seat (and the unified
// Mutate/Meld merged-card pointers on each Permanent) and records a
// heimdall.LineageRecord for each Card with a minted InstanceID. The
// resulting map is empty when no IDs have been minted yet (legacy
// games) — that's the backwards-compat path.
func buildLineageIndex(gs *gameengine.GameState) map[string]heimdall.LineageRecord {
	if gs == nil {
		return nil
	}
	out := map[string]heimdall.LineageRecord{}
	add := func(c *gameengine.Card) {
		if c == nil || c.InstanceID == "" {
			return
		}
		if _, ok := out[c.InstanceID]; ok {
			return
		}
		out[c.InstanceID] = heimdall.LineageRecord{
			InstanceID:        c.InstanceID,
			Name:              c.DisplayName(),
			Provenance:        c.Provenance.String(),
			SourceInstanceID:  c.SourceInstanceID,
			EnablerInstanceID: c.EnablerInstanceID,
			EnablerHistory:    append([]string(nil), c.EnablerHistory...),
		}
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Hand {
			add(c)
		}
		for _, c := range s.Library {
			add(c)
		}
		for _, c := range s.Graveyard {
			add(c)
		}
		for _, c := range s.Exile {
			add(c)
		}
		for _, c := range s.CommandZone {
			add(c)
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			add(p.Card)
			// Mutate / Meld absorbed cards live in MergedCardPtrs.
			for _, mc := range p.MergedCardPtrs {
				add(mc)
			}
			// Attach merged-card lineage onto the top card's record so
			// the tree walker can recurse via MergedCardIDs.
			if p.Card != nil && p.Card.InstanceID != "" && len(p.MergedCards) > 0 {
				rec := out[p.Card.InstanceID]
				rec.MergedCardIDs = append([]string(nil), p.MergedCards...)
				out[p.Card.InstanceID] = rec
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// instanceIDFromDetails extracts the most-relevant InstanceID from an
// engine event's Details map. Phase 8/5 writers stamp under either
// "instance_id" (canonical) or one of the lineage-specific keys.
// Returns the empty string when none are present.
func instanceIDFromDetails(details map[string]interface{}) string {
	if len(details) == 0 {
		return ""
	}
	for _, key := range []string{"instance_id", "card_instance_id", "source_instance_id", "ability_instance_id"} {
		if v, ok := details[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// buildTurnSnapshot converts the per-seat data from a fully-captured
// GameSnapshot into the compact TurnSnapshot stored on CompletedGame.
// Strips the Eval block (carried only for the live spectator) and
// duplicated Commander labels (already on CompletedGame.Commanders).
func buildTurnSnapshot(turn, active int, seats []SeatSnapshot, events []LogEntry) TurnSnapshot {
	out := TurnSnapshot{
		Turn:       turn,
		ActiveSeat: active,
		Seats:      make([]SeatTurnSnapshot, len(seats)),
		Events:     events,
	}
	for i, s := range seats {
		out.Seats[i] = SeatTurnSnapshot{
			Life:        s.Life,
			HandSize:    s.HandSize,
			LibrarySize: s.LibrarySize,
			GYSize:      s.GYSize,
			Lost:        s.Lost,
			LossReason:  s.LossReason,
			Battlefield: s.Battlefield,
		}
	}
	return out
}

// hexELOStartRating returns the bracket-seeded starting ELO.
// B1=400, B2=1100, B3=1800, B4=2450, B5=3000.
func hexELOStartRating(bracket int) float64 {
	switch bracket {
	case 1:
		return 400
	case 2:
		return 1100
	case 3:
		return 1800
	case 4:
		return 2450
	case 5:
		return 3000
	default:
		return 1500
	}
}

// trueSkillConfig returns the TrueSkill config scaled for our rating range.
// Default TrueSkill uses μ=25, σ=8.33 for a 0-50 range. We scale to 0-4000.
// Scale factor ~80: β=333, τ=6.7, σ₀=400.
var tsConfig = trueskill.Config{
	Beta:            333.0,
	Tau:             6.7,
	DrawProbability: 0.01,
}

const tsDefaultSigma = 400.0

func trueSkillStartRating(bracket int) trueskill.Rating {
	return trueskill.Rating{
		Mu:    hexELOStartRating(bracket),
		Sigma: tsDefaultSigma,
	}
}

// hexELOBandLabel returns the human-readable band for a rating.
func hexELOBandLabel(rating float64) string {
	switch {
	case rating >= 3000:
		return "B5"
	case rating >= 2100:
		return "B4"
	case rating >= 1400:
		return "B3"
	case rating >= 700:
		return "B2"
	default:
		return "B1"
	}
}

// hexELOKFactor computes the bracket-aware K-factor.
// Same bracket: full K. Cross-bracket expected results: dampened.
// Cross-bracket upsets: less dampened (interesting signal).
func hexELOKFactor(baseK float64, winnerBracket, loserBracket int) float64 {
	if winnerBracket <= 0 || loserBracket <= 0 {
		return baseK
	}
	dist := winnerBracket - loserBracket
	if dist < 0 {
		dist = -dist
	}
	if dist == 0 {
		return baseK
	}
	if winnerBracket > loserBracket {
		return baseK * math.Max(0.3, 1.0-float64(dist)*0.25)
	}
	return baseK * math.Max(0.5, 1.0-float64(dist)*0.15)
}

// hexELOBracketFloor returns the floor and basement for a bracket.
// Between floor and basement gravity ramps hard; at basement it's a wall.
func hexELOBracketFloor(bracket int) (floor, basement float64) {
	switch bracket {
	case 1:
		return 100, 0
	case 2:
		return 600, 500
	case 3:
		return 1300, 1200
	case 4:
		return 2000, 1900
	case 5:
		return 2800, 2700
	default:
		return 600, 500
	}
}

// hexELOGravity applies a pull toward the deck's bracket center.
// Above the floor: mild pull (capped ±5). Below: ramps hard toward
// the basement, making it nearly impossible to sink further.
func hexELOGravity(rating float64, bracket int) float64 {
	if bracket <= 0 {
		return 0
	}
	center := hexELOStartRating(bracket)
	floor, basement := hexELOBracketFloor(bracket)

	if rating >= floor {
		pull := 0.02 * (center - rating)
		if pull > 5.0 {
			pull = 5.0
		}
		if pull < -5.0 {
			pull = -5.0
		}
		return pull
	}

	// Below floor: gravity ramps from 5 → 40 as rating approaches basement.
	span := floor - basement
	if span <= 0 {
		span = 1
	}
	t := (floor - rating) / span // 0 at floor, 1 at basement, >1 below
	if t > 1 {
		t = 1
	}
	return 5.0 + t*35.0
}

// hexELOLossDamper returns a multiplier [0.1, 1.0] for loss deltas.
// Below the bracket floor losses barely count — the deck is already dead.
// Loss streaks also diminish: after 3 consecutive losses each additional
// loss is worth less.
func hexELOLossDamper(rating float64, bracket, lossStreak int) float64 {
	damper := 1.0

	// Below-floor damping: losses fade as rating approaches basement.
	floor, basement := hexELOBracketFloor(bracket)
	if rating < floor {
		span := floor - basement
		if span <= 0 {
			span = 1
		}
		t := (floor - rating) / span // 0 at floor, 1 at basement
		if t > 1 {
			t = 1
		}
		damper *= math.Max(0.1, 1.0-0.9*t)
	}

	// Loss streak damping: after 3 consecutive losses, each additional
	// loss is worth 15% less, down to 20% minimum.
	if lossStreak > 3 {
		streakDamp := math.Max(0.2, 1.0-0.15*float64(lossStreak-3))
		damper *= streakDamp
	}

	return damper
}

// hexELOStreakBreakBonus returns bonus ELO for winning after a loss streak.
// Rewards breaking out: +3 per streak game, up to +30.
func hexELOStreakBreakBonus(lossStreak int) float64 {
	if lossStreak < 3 {
		return 0
	}
	return math.Min(float64(lossStreak)*3.0, 30.0)
}

// --- ELO corruption band-aid (r60, 2026-06-10) ---------------------------
// The TrueSkill update path had no bound on μ/σ, so over millions of grinder
// games the conservative rating (μ−3σ) drifted into the ±100M range seen in
// the 2026-06-10 gauntlet reports (-483M start / +471M delta). Two compounding
// causes: (1) μ/σ accumulate with no clamp, and (2) tsMu/tsSigma are NOT
// persisted, so every restart reset them to 0 and the next updates ran on a
// degenerate σ=0 rating. These constants + clampedTS bound the rating to a
// sane band on every update and on load; loadPersistedState re-seeds entries
// whose persisted rating is already out of band. Reversible safety bound — the
// proper 4-player-FFA rating model fix lives on dev/trueskill-4p-ffa-fix-r60.
const (
	tsMuFloor    = 0.0
	tsMuCeil     = 6000.0 // headroom so a reconstructed μ (rating+3σ) isn't clipped
	tsSigmaFloor = 40.0   // ~σ₀/10: prevents pathological over-certainty / σ→0
	eloRatingMin = 0.0
	eloRatingMax = 4000.0
)

func clampF(v, lo, hi float64) float64 {
	switch {
	case v != v: // NaN
		return lo
	case v < lo:
		return lo
	case v > hi:
		return hi
	default:
		return v
	}
}

// clampedTS bounds a freshly-updated TrueSkill rating and returns the clamped
// (μ, σ) plus the clamped conservative rating (μ−3σ) to store.
func clampedTS(r trueskill.Rating) (mu, sigma, rating float64) {
	mu = clampF(r.Mu, tsMuFloor, tsMuCeil)
	sigma = clampF(r.Sigma, tsSigmaFloor, tsDefaultSigma)
	rating = clampF(mu-3*sigma, eloRatingMin, eloRatingMax)
	return
}

func (sm *Showmatch) updateELO(deckKeys, commanders []string, decks []*deckparser.TournamentDeck, winner int, turns int) []heimdall.CompositionPriorEffect {
	// Rating-integrity chokepoint (r63): NEVER move ratings during
	// maintenance. The grinder / fishtank / spectate loops already abort a
	// game at their between-turns maintenance check, so they normally never
	// reach here in maintenance — but a flip in the window between the final
	// turn's check and this call (or any future caller that forgets the loop
	// guard) must not corrupt or persist real player ratings. Gating the one
	// rating-mutation function is the can't-miss guarantee: no games++, no
	// μ/σ movement, no HexELO change, nothing to flush.
	//
	// Read sm.maintenance DIRECTLY, not via sm.inMaintenance(): every
	// updateELO caller already holds sm.mu.Lock() (grinder/bracket/show in
	// showmatch.go, plus spectate_rooms.go), and inMaintenance() takes
	// sm.mu.RLock() — re-acquiring a non-reentrant sync.RWMutex would
	// deadlock. Under the held write lock this field read is race-free.
	if sm.maintenance {
		return nil
	}
	const baseK = 32.0
	n := len(deckKeys)
	if n < 2 {
		return nil
	}
	kScaled := baseK / float64(n-1)

	for i, key := range deckKeys {
		if _, ok := sm.elo[key]; !ok {
			owner, _ := deckOwnerFromKey(key)
			bracket := sm.bracketCache[key]
			tsStart := trueSkillStartRating(bracket)
			sm.elo[key] = &eloState{
				rating:    tsStart.Conservative(),
				hexRating: hexELOStartRating(bracket),
				tsMu:      tsStart.Mu,
				tsSigma:   tsStart.Sigma,
				commander: commanders[i],
				owner:     owner,
				bracket:   bracket,
			}
		}
		sm.elo[key].games++
	}

	if winner >= 0 && winner < n {
		sm.elo[deckKeys[winner]].wins++
	}

	// --- R60: Composition prior offsets (PR #408 wiring) ---
	// Each deck's rating update will be performed in offset-shifted
	// μ-space so the prior's pod-conditioned expectations correctly
	// modulate how much "credit" a win in a favored composition
	// receives. Confidence=0 cells produce 0 offset → no behavioral
	// change when the prior has no data, which is true at cold-start
	// and remains true for any (archetype, pod) cell the prior has
	// never seen.
	podArchetypes := make([]string, n)
	for i, key := range deckKeys {
		if sp := sm.strategies[key]; sp != nil {
			podArchetypes[i] = sp.Archetype
		}
	}
	offsets := make([]float64, n)
	priorConfidence := make([]float64, n)
	priorExpected := make([]float64, n)
	for i, arch := range podArchetypes {
		offsets[i] = trueskill.ComputeCompositionOffset(
			sm.compositionPrior, arch, podArchetypes, sm.compositionPriorCfg)
		// Snapshot Confidence + ExpectedWinrate at the SAME moment
		// the offset is computed so monitoring records reflect the
		// state that produced the offset, not the post-ObserveGame
		// state at the end of updateELO.
		if sm.compositionPrior != nil {
			priorConfidence[i] = sm.compositionPrior.Confidence(arch, podArchetypes)
			priorExpected[i] = sm.compositionPrior.ExpectedWinrate(arch, podArchetypes)
		}
	}

	// --- R60 monitoring (PR #418): per-seat μ-before snapshot + a
	// shadow vanilla update for the "what would have happened without
	// the prior" baseline. Cheap (~one extra UpdateMultiplayer call)
	// and gives Heimdall the MuDeltaVsBaseline signal without doing
	// it offline. Uses the SAME algorithm as the real update below
	// (all-pairs decomposition) so the delta isolates the prior's
	// effect rather than mixing in any algorithm-shape difference.
	muBefore := make([]float64, n)
	for i, key := range deckKeys {
		muBefore[i] = sm.elo[key].tsMu
	}
	vanillaAfter := make([]float64, n)
	{
		shadowRatings := make([]trueskill.Rating, n)
		ranks := make([]int, n)
		for i, key := range deckKeys {
			shadowRatings[i] = trueskill.Rating{Mu: sm.elo[key].tsMu, Sigma: sm.elo[key].tsSigma}
			if winner >= 0 && winner < n {
				if i == winner {
					ranks[i] = 0
				} else {
					ranks[i] = 1
				}
			}
		}
		shadowRatings = trueskill.UpdateMultiplayer(tsConfig, shadowRatings, ranks)
		for i := range deckKeys {
			vanillaAfter[i] = shadowRatings[i].Mu
		}
	}

	// --- TrueSkill update (primary rating) ---
	// All-pairs decomposition via UpdateMultiplayerWithOffsets — winner
	// gets credit for beating EACH loser (not chained-and-double-counted),
	// and each loser eats roughly Δ_winner/3 of the signal mass (the
	// three losers tie among themselves, washing out as ~zero draws).
	// This replaces the pre-R60 pairwise Update2Player loop that gave
	// each loser a FULL 1v1 decisive-loss against a chained winner —
	// triple-penalizing losers vs the actual 25% per-seat win expectation.
	if winner >= 0 && winner < n {
		tsRatings := make([]trueskill.Rating, n)
		ranks := make([]int, n)
		for i, key := range deckKeys {
			tsRatings[i] = trueskill.Rating{Mu: sm.elo[key].tsMu, Sigma: sm.elo[key].tsSigma}
			if i == winner {
				ranks[i] = 0
			} else {
				ranks[i] = 1
			}
		}
		updated := trueskill.UpdateMultiplayerWithOffsets(tsConfig, tsRatings, ranks, offsets)
		for i, key := range deckKeys {
			oldRating := sm.elo[key].rating
			mu, sigma, rating := clampedTS(updated[i])
			sm.elo[key].tsMu = mu
			sm.elo[key].tsSigma = sigma
			sm.elo[key].rating = rating
			sm.elo[key].delta = sm.elo[key].rating - oldRating
		}
	} else {
		// No winner (draw/timeout) — all players draw pairwise.
		tsRatings := make([]trueskill.Rating, n)
		ranks := make([]int, n)
		for i, key := range deckKeys {
			tsRatings[i] = trueskill.Rating{Mu: sm.elo[key].tsMu, Sigma: sm.elo[key].tsSigma}
		}
		updated := trueskill.UpdateMultiplayer(tsConfig, tsRatings, ranks)
		for i, key := range deckKeys {
			oldRating := sm.elo[key].rating
			mu, sigma, rating := clampedTS(updated[i])
			sm.elo[key].tsMu = mu
			sm.elo[key].tsSigma = sigma
			sm.elo[key].rating = rating
			sm.elo[key].delta = sm.elo[key].rating - oldRating
		}
	}

	// --- R60: Feed the composition prior with this game's outcome ---
	// Done AFTER the TrueSkill update so the offsets the prior would
	// compute for THIS game (which were already applied above) reflect
	// what the prior knew BEFORE the game. Future games of the same
	// archetype-pod composition will benefit from the new observation.
	// Both decisive games and draws update the prior (draws via empty
	// winnerArchetype, which credits participation only).
	if sm.compositionPrior != nil {
		winnerArch := ""
		if winner >= 0 && winner < n {
			winnerArch = podArchetypes[winner]
		}
		_ = sm.compositionPrior.ObserveGame(podArchetypes, winnerArch)
	}

	// --- HexELO update (bracket-aware K + gravity + floor/streak) ---
	winnerKey := ""
	if winner >= 0 && winner < n {
		winnerKey = deckKeys[winner]
	}
	if winnerKey == "" {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				a, b := deckKeys[i], deckKeys[j]
				hexEa := 1.0 / (1.0 + math.Pow(10, (sm.elo[b].hexRating-sm.elo[a].hexRating)/400.0))
				hexEb := 1.0 - hexEa
				hk := hexELOKFactor(kScaled, sm.elo[a].bracket, sm.elo[b].bracket)
				sm.elo[a].hexDelta = hk * (0.5 - hexEa)
				sm.elo[b].hexDelta = hk * (0.5 - hexEb)
				sm.elo[a].hexRating += sm.elo[a].hexDelta + hexELOGravity(sm.elo[a].hexRating, sm.elo[a].bracket)
				sm.elo[b].hexRating += sm.elo[b].hexDelta + hexELOGravity(sm.elo[b].hexRating, sm.elo[b].bracket)
			}
		}
		for _, key := range deckKeys {
			sm.elo[key].lossStreak++
		}
		return sm.buildCompositionPriorEffects(deckKeys, podArchetypes, offsets, priorConfidence, priorExpected, muBefore, vanillaAfter)
	}

	// Winner: streak break bonus + reset streak.
	wStreak := sm.elo[winnerKey].lossStreak
	streakBonus := hexELOStreakBreakBonus(wStreak)
	sm.elo[winnerKey].lossStreak = 0

	// Losers: increment streak.
	for _, key := range deckKeys {
		if key != winnerKey {
			sm.elo[key].lossStreak++
		}
	}

	for _, loserKey := range deckKeys {
		if loserKey == winnerKey {
			continue
		}
		wBracket := sm.elo[winnerKey].bracket
		lBracket := sm.elo[loserKey].bracket
		hk := hexELOKFactor(kScaled, wBracket, lBracket)
		hexEW := 1.0 / (1.0 + math.Pow(10, (sm.elo[loserKey].hexRating-sm.elo[winnerKey].hexRating)/400.0))
		hexEL := 1.0 - hexEW
		hwDelta := hk * (1.0 - hexEW)
		hlDelta := hk * (0.0 - hexEL)

		// Dampen loser's loss based on floor proximity + streak length.
		hlDelta *= hexELOLossDamper(sm.elo[loserKey].hexRating, lBracket, sm.elo[loserKey].lossStreak)

		sm.elo[winnerKey].hexDelta = hwDelta
		sm.elo[loserKey].hexDelta = hlDelta
		sm.elo[winnerKey].hexRating += hwDelta + hexELOGravity(sm.elo[winnerKey].hexRating, wBracket)
		sm.elo[loserKey].hexRating += hlDelta + hexELOGravity(sm.elo[loserKey].hexRating, lBracket)
	}

	// Streak break bonus applied once after all pairwise updates.
	sm.elo[winnerKey].hexRating += streakBonus

	// Hard basement clamp — absolute floor.
	for _, key := range deckKeys {
		_, basement := hexELOBracketFloor(sm.elo[key].bracket)
		if sm.elo[key].hexRating < basement {
			sm.elo[key].hexRating = basement
		}
	}

	// Anti-cheat Phase 1 — feed each contributor's per-game result into
	// the statistical anomaly detector and log any flagged outliers.
	// Detection-only: no action is taken on a flag here. The detector
	// has its own mutex so it doesn't interact with sm.mu.
	for i, key := range deckKeys {
		won := i == winner
		if flag := hat.DefaultAnomalyDetector.Record(key, won); flag != nil {
			hat.LogAnomaly(flag)
		}
	}

	// Anti-cheat Phase 2 — same idea at the contributor (owner)
	// granularity, persisted to SQLite and audited across multiple
	// dimensions (win rate, average game length, turn variance). Today
	// the contributor is the deck owner; when BOINC lands the key will
	// shift to a per-machine identifier without changing the math.
	// One result per owner per pod: multiple decks from the same owner
	// would otherwise pin their win/loss ratio to the pod-share they
	// brought, which is a deck-construction artifact, not a signal.
	if sm.auditor != nil {
		seen := map[string]bool{}
		for i, key := range deckKeys {
			owner, _ := deckOwnerFromKey(key)
			if owner == "" || seen[owner] {
				continue
			}
			seen[owner] = true
			won := i == winner
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			flags, err := sm.auditor.RecordGame(ctx, anticheat.Game{
				ContributorID: owner,
				Won:           won,
				Turns:         turns,
			})
			cancel()
			if err != nil {
				log.Printf("anticheat: record %s: %v", owner, err)
				continue
			}
			for _, f := range flags {
				log.Printf("anticheat flag: contributor=%s metric=%s value=%.4f z=%+.2fσ severity=%d (mean=%.4f stddev=%.4f)",
					f.ContributorID, f.Metric, f.MetricValue, f.ZScore, f.Severity, f.PopMean, f.PopStdDev)
			}
		}
	}

	return sm.buildCompositionPriorEffects(deckKeys, podArchetypes, offsets, priorConfidence, priorExpected, muBefore, vanillaAfter)
}

// buildCompositionPriorEffects assembles the per-seat monitoring
// records (PR #418) used by Heimdall analytics. priorConfidence /
// priorExpected are pre-update snapshots — they reflect the state
// that produced the offset, NOT the post-ObserveGame state. Reads
// each deck's post-update μ from sm.elo to compute MuDeltaVsBaseline.
// MuDeltaVsBaseline = priorAfter - vanillaAfter (same μBefore for
// both → simplifies the formal definition).
func (sm *Showmatch) buildCompositionPriorEffects(
	deckKeys, podArchetypes []string,
	offsets, priorConfidence, priorExpected, muBefore, vanillaAfter []float64,
) []heimdall.CompositionPriorEffect {
	effects := make([]heimdall.CompositionPriorEffect, len(deckKeys))
	for i, key := range deckKeys {
		muAfterPrior := sm.elo[key].tsMu
		effects[i] = heimdall.CompositionPriorEffect{
			Seat:              i,
			Archetype:         podArchetypes[i],
			Offset:            offsets[i],
			Confidence:        priorConfidence[i],
			ExpectedWinrate:   priorExpected[i],
			MuDeltaVsBaseline: muAfterPrior - vanillaAfter[i],
		}
	}
	return effects
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
			Mu:        math.Round(e.tsMu*10) / 10,
			Sigma:     math.Round(e.tsSigma*10) / 10,
			HexRating: math.Round(e.hexRating*10) / 10,
			Games:     e.games,
			Wins:      e.wins,
			Losses:    losses,
			WinRate:   winRate,
			Delta:     math.Round(e.delta*10) / 10,
			HexDelta:  math.Round(e.hexDelta*10) / 10,
			Bracket:   e.bracket,
			Band:      hexELOBandLabel(e.hexRating),
		})
	}
	// Sort by standard ELO for primary ranking.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Rating != entries[j].Rating {
			return entries[i].Rating > entries[j].Rating
		}
		if entries[i].Games != entries[j].Games {
			return entries[i].Games > entries[j].Games
		}
		return entries[i].Commander < entries[j].Commander
	})
	// Build HexELO rank index for drift computation.
	hexOrder := make([]int, len(entries))
	for i := range hexOrder {
		hexOrder[i] = i
	}
	sort.SliceStable(hexOrder, func(a, b int) bool {
		return entries[hexOrder[a]].HexRating > entries[hexOrder[b]].HexRating
	})
	hexRank := make(map[int]int, len(entries))
	for rank, idx := range hexOrder {
		hexRank[idx] = rank
	}
	// Compute drift: standard rank minus hex rank, normalized to [-100, +100].
	// Positive drift = deck ranks higher in standard ELO than HexELO (overperforming bracket).
	// Negative drift = deck ranks higher in HexELO than standard (underperforming bracket).
	n := float64(len(entries))
	for i := range entries {
		if n <= 1 {
			continue
		}
		stdPct := float64(i) / (n - 1) * 100.0
		hexPct := float64(hexRank[i]) / (n - 1) * 100.0
		drift := math.Round((hexPct-stdPct)*10) / 10
		entries[i].Drift = drift
		switch {
		case drift >= 15:
			entries[i].DriftTag = "outlier_above"
		case drift >= 5:
			entries[i].DriftTag = "above_bracket"
		case drift <= -15:
			entries[i].DriftTag = "outlier_below"
		case drift <= -5:
			entries[i].DriftTag = "below_bracket"
		}
	}
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
	mux.HandleFunc("GET /api/live/elo/drift", sm.handleLiveDrift)
	mux.HandleFunc("GET /api/live/stats", sm.handleLiveStatsReal)
	mux.HandleFunc("GET /api/games", sm.handleGames)
	mux.HandleFunc("GET /api/games/{id}", sm.handleGameByID)
	mux.HandleFunc("GET /api/games/{id}/report", sm.handleGameReport)
	mux.HandleFunc("GET /api/live/speed", sm.handleGetSpeed)
	mux.HandleFunc("POST /api/live/speed", sm.requireCSRFLate(sm.handleSetSpeed))
	mux.HandleFunc("GET /ws/live", sm.handleSpectatorWS)
	mux.HandleFunc("POST /api/gauntlet/{owner}/{id}", sm.requireCSRFLate(sm.handleStartGauntlet))
	mux.HandleFunc("GET /api/gauntlet/{owner}/{id}", sm.handleGetGauntlet)
	mux.HandleFunc("GET /api/tournaments/{owner}/{id}/events", sm.handleTournamentEvents)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/curse", sm.handleDeckCurse)
	mux.HandleFunc("PATCH /api/decks/{owner}/{id}/curse", sm.requireCSRFLate(sm.handlePatchCurse))
	mux.HandleFunc("GET /api/achievements/{owner}", sm.handleAchievements)
	mux.HandleFunc("GET /api/owner/{owner}/stats", sm.handleOwnerStats)
	mux.HandleFunc("GET /api/owner/{owner}/games", sm.handleOwnerGames)

	mux.HandleFunc("POST /api/spectate/spawn", sm.requireCSRFLate(sm.handleSpawnSpectateRoom))
	mux.HandleFunc("POST /api/showmatch/reload-pool", sm.requireCSRFLate(sm.handleReloadPool))
	mux.HandleFunc("GET /api/spectate/rooms", sm.handleListSpectateRooms)
	mux.HandleFunc("GET /api/spectate/rooms/{room_id}", sm.handleGetSpectateRoom)
	mux.HandleFunc("GET /ws/spectate/{room_id}", sm.handleSpectateRoomWS)
	// InstanceID Phase 9 — lineage tree endpoint. Returns the walked
	// LineageNode JSON for any InstanceID currently in any live room.
	mux.HandleFunc("GET /api/spectator/lineage/{instance_id}", sm.handleSpectatorLineage)

	RegisterAdminAnomalies(mux, sm.auditor)
}

var gauntletSem = make(chan struct{}, 2)

func (sm *Showmatch) handleStartGauntlet(w http.ResponseWriter, r *http.Request) {
	if sm.inMaintenance() {
		writeError(w, http.StatusServiceUnavailable, maintenanceMessage)
		return
	}
	// Per-IP rate-limit gate, layered in FRONT of the global semaphore +
	// credit-economy checks. The sem cap already throttles concurrent
	// gauntlets; the limiter prevents an attacker from rapidly cycling
	// through the cap (start-fail-start-fail) or burning deck-pool
	// lookups across many invalid deck ids. Limiter is nil-safe.
	if enforceRateLimitDual(sm.GauntletLimiter, sm.GauntletOwnerLimiter, w, r, "gauntlet start") {
		return
	}
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	// Reject path components containing "..", slashes, NULs, or any
	// character outside the deck-id alphabet before they flow into the
	// deckKey string. deckKey is used as a credit-ledger reference, a
	// log-line interpolation, and an SSE broadcast topic — every other
	// deck-targeting handler in hexapi already runs this guard
	// (handleDeckSharePage, handleDeckUpgrade, etc.).
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	deckKey := owner + "/" + id

	sm.gauntletMu.RLock()
	existing := sm.gauntlets[deckKey]
	sm.gauntletMu.RUnlock()
	if existing != nil && existing.Status == "running" {
		writeJSON(w, existing)
		return
	}

	numGames := 500
	if n := parseInt(r.URL.Query().Get("games")); n > 0 && n <= 50000 {
		numGames = n
	}

	// Verify the deck is actually in the engine pool BEFORE charging
	// credits. RunGauntlet's findDeckInPool would otherwise just
	// register an error result and return — leaving paying users with
	// no refund (charging is atomic via credits.Spend, but the launch
	// it pays for never executed). Cheap RLock-protected map walk.
	if sm.findDeckInPool(owner, id) == nil {
		writeError(w, http.StatusNotFound,
			"deck not in engine pool — re-import or check the deck id")
		return
	}

	// Acquire the global gauntlet semaphore BEFORE the credit charge
	// for the same reason: if we're at the concurrency cap, return 429
	// without debiting. Non-blocking select — either we get the slot
	// or we bail.
	select {
	case gauntletSem <- struct{}{}:
	default:
		writeError(w, http.StatusTooManyRequests, "too many gauntlets running — try again later")
		return
	}
	// From this point on, every error path must release gauntletSem
	// before returning. Track release responsibility so the launching
	// goroutine knows whether to own it.
	semOwnedByGoroutine := false
	defer func() {
		if !semOwnedByGoroutine {
			<-gauntletSem
		}
	}()

	// Credit-economy gate. Caller identity comes from X-HexDek-Owner;
	// if absent we fall through to free behaviour for backwards
	// compatibility with the (already-public) gauntlet endpoint.
	caller := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner")))
	chargedFree := true
	var chargedAmount int64
	if sm.credits != nil && caller != "" {
		quota, err := sm.credits.QuotaState(r.Context(), caller)
		if err != nil {
			log.Printf("gauntlet: quota check failed: %v", err)
		} else if quota.CanRunFree {
			// Within the daily allowance — no charge, just log usage.
			chargedFree = true
		} else if quota.CanRunPaid {
			// Past the free quota; debit credits before launching.
			// Spend is atomic — either it succeeds (and we proceed) or
			// it fails (and nothing is debited), so we never end up in
			// the "charged but no launch" state that the previous
			// ordering allowed.
			if _, err := sm.credits.Spend(r.Context(), caller,
				credits.CreditsPerGauntlet, credits.ReasonGauntletRun, deckKey); err != nil {
				if err == credits.ErrInsufficientCredits {
					writeErrorWithDetails(w, http.StatusPaymentRequired, "insufficient_credits", map[string]any{
						"balance": quota.Balance,
						"needed":  credits.CreditsPerGauntlet,
						"quota":   quota,
					})
					return
				}
				writeError(w, http.StatusInternalServerError, "credit charge failed: "+err.Error())
				return
			}
			chargedFree = false
			chargedAmount = credits.CreditsPerGauntlet
		} else {
			// Out of free runs and short on credits — surface the
			// quota state so the frontend can render an actionable
			// "earn or wait" message.
			writeErrorWithDetails(w, http.StatusPaymentRequired, "free_quota_exhausted", map[string]any{
				"balance": quota.Balance,
				"needed":  credits.CreditsPerGauntlet,
				"quota":   quota,
			})
			return
		}
		// Best-effort log of the gauntlet usage for the audit trail
		// and the next quota check.
		if err := sm.credits.LogGauntlet(r.Context(), caller, deckKey,
			numGames, chargedFree, chargedAmount); err != nil {
			log.Printf("gauntlet: usage log failed: %v", err)
		}
	}

	semOwnedByGoroutine = true
	go func() {
		defer func() { <-gauntletSem }()
		sm.RunGauntlet(owner, id, numGames)
	}()
	resp := map[string]any{
		"status":   "started",
		"deck_key": deckKey,
		"games":    numGames,
	}
	if !chargedFree {
		resp["credits_charged"] = chargedAmount
	}
	writeJSON(w, resp)
}

func (sm *Showmatch) handleGetGauntlet(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	deckKey := owner + "/" + id

	result := sm.GetGauntlet(deckKey)
	if result == nil {
		writeJSON(w, map[string]any{"status": "none"})
		return
	}
	writeJSON(w, result)
}

// gauntletSubscriber is one SSE listener for a gauntlet's progress.
type gauntletSubscriber struct {
	ch chan GauntletResult
}

func (sm *Showmatch) subscribeGauntlet(deckKey string) *gauntletSubscriber {
	sub := &gauntletSubscriber{ch: make(chan GauntletResult, 16)}
	sm.gauntletSubsMu.Lock()
	if _, ok := sm.gauntletSubs[deckKey]; !ok {
		sm.gauntletSubs[deckKey] = make(map[*gauntletSubscriber]struct{})
	}
	sm.gauntletSubs[deckKey][sub] = struct{}{}
	sm.gauntletSubsMu.Unlock()
	return sub
}

func (sm *Showmatch) unsubscribeGauntlet(deckKey string, sub *gauntletSubscriber) {
	sm.gauntletSubsMu.Lock()
	if subs, ok := sm.gauntletSubs[deckKey]; ok {
		if _, ok := subs[sub]; ok {
			delete(subs, sub)
			close(sub.ch)
		}
		if len(subs) == 0 {
			delete(sm.gauntletSubs, deckKey)
		}
	}
	sm.gauntletSubsMu.Unlock()
}

func (sm *Showmatch) broadcastGauntlet(deckKey string, snap GauntletResult) {
	// Lock is held across the non-blocking sends so a concurrent
	// unsubscribe cannot close(ch) between our membership check and
	// the send (which would panic). Each send is a single select with
	// default — bounded constant time.
	sm.gauntletSubsMu.Lock()
	defer sm.gauntletSubsMu.Unlock()
	for s := range sm.gauntletSubs[deckKey] {
		select {
		case s.ch <- snap:
		default:
			// Subscriber buffer full — drop this delta rather than
			// stall the gauntlet loop. The next event catches them up.
		}
	}
}

func (sm *Showmatch) closeGauntletSubs(deckKey string) {
	sm.gauntletSubsMu.Lock()
	subs := sm.gauntletSubs[deckKey]
	delete(sm.gauntletSubs, deckKey)
	sm.gauntletSubsMu.Unlock()
	for s := range subs {
		close(s.ch)
	}
}

// handleTournamentEvents streams gauntlet progress as Server-Sent Events.
// One `snapshot` event is sent immediately with the current state, then
// further `snapshot` events fire after every completed game until the
// gauntlet reaches a terminal status (complete | error) or the client
// disconnects.
func (sm *Showmatch) handleTournamentEvents(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	// Mirror the validatePathComponent guard the two gauntlet handlers
	// already run on owner / id. deckKey is the SSE subscriber-map key
	// and gets baked into broadcast snapshots — without validation, a
	// "../foo" owner or an id with a slash could pollute the topic map
	// or smuggle controlled bytes into snapshot payloads.
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	deckKey := owner + "/" + id

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := sm.subscribeGauntlet(deckKey)
	defer sm.unsubscribeGauntlet(deckKey, sub)

	if init := sm.GetGauntlet(deckKey); init != nil {
		writeSSE(w, flusher, "snapshot", init)
		if init.Status == "complete" || init.Status == "error" {
			return
		}
	} else {
		writeSSE(w, flusher, "snapshot", map[string]any{
			"status":   "none",
			"deck_key": deckKey,
		})
	}

	ctx := r.Context()
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case snap, ok := <-sub.ch:
			if !ok {
				return
			}
			writeSSE(w, flusher, "snapshot", snap)
			if snap.Status == "complete" || snap.Status == "error" {
				return
			}
		case <-keepalive.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	flusher.Flush()
}

// CurseResponse is the JSON payload for GET /api/decks/{owner}/{id}/curse.
type CurseResponse struct {
	DeckKey    string           `json:"deck_key"`
	GameCount  int              `json:"game_count"`
	TotalGames int              `json:"total_games"`
	Population []CurseMemberDTO `json:"population"`

	// DimStatsN is the number of games observed by the dimension-stats
	// EMA. The frontend treats DimCorrections as cold (== 1.0) until N
	// crosses the engine's dimStatsMinN threshold.
	DimStatsN int `json:"dim_stats_n"`

	// DimCorrections are the 20 outcome-correlated multiplicative
	// corrections to the deck's eval weights (1.0 = no correction;
	// >1 means the dimension predicts wins for this deck and is
	// boosted; <1 means it predicts losses and is suppressed). Index
	// matches DimLabels.
	DimCorrections []float64 `json:"dim_corrections"`
	DimLabels      []string  `json:"dim_labels"`

	// Constraints maps trait key → owner-locked target value in [0,1].
	// Keys not present mean the trait is free to evolve. See
	// hat.CurseTraitKeys for the canonical key list.
	Constraints map[string]float64 `json:"constraints,omitempty"`
}

// CurseMemberDTO is one member of the Curse genetic population.
type CurseMemberDTO struct {
	Generation       int     `json:"generation"`
	GamesPlayed      int     `json:"games_played"`
	Fitness          float64 `json:"fitness"`
	Aggression       float64 `json:"aggression"`
	ComboPatience    float64 `json:"combo_patience"`
	ThreatParanoia   float64 `json:"threat_paranoia"`
	ResourceGreed    float64 `json:"resource_greed"`
	PoliticalMemory  float64 `json:"political_memory"`
	DrainAffinity    float64 `json:"drain_affinity"`
	ArtifactAffinity float64 `json:"artifact_affinity"`
}

// curseDimLabels are the human-readable names for the 20 evaluator
// dimensions in the same order as hat.EvalWeights.AsArray() and
// hat.DimensionStats.WeightCorrections(). Kept in sync with
// internal/hat/eval_weights.go.
var curseDimLabels = []string{
	"BOARD", "CARDS", "MANA",
	"LIFE", "COMBO", "THREAT",
	"COMMANDER", "GRAVEYARD", "DRAIN",
	"ARTIFACT", "ENCHANT", "OPP GY",
	"PARTNER", "TEMPO", "TOOLBOX",
	"THR TRAJ", "STACK", "PLANESWALKER",
	"EXILE", "STAX",
}

func (sm *Showmatch) handleDeckCurse(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	deckKey := owner + "/" + id

	sm.curseMu.Lock()
	pool := sm.cursePool[deckKey]
	if pool == nil {
		sm.curseMu.Unlock()
		writeError(w, http.StatusNotFound, "no curse pool for deck")
		return
	}
	// Snapshot under lock to avoid data races.
	resp := CurseResponse{
		DeckKey:    pool.DeckKey,
		GameCount:  pool.GameCount,
		TotalGames: pool.TotalGames,
		DimStatsN:  pool.DimStats.N,
		DimLabels:  curseDimLabels,
	}
	corr := pool.DimStats.WeightCorrections()
	resp.DimCorrections = make([]float64, len(corr))
	copy(resp.DimCorrections, corr[:])
	if len(pool.Constraints) > 0 {
		resp.Constraints = make(map[string]float64, len(pool.Constraints))
		for k, v := range pool.Constraints {
			resp.Constraints[k] = v
		}
	}
	for _, dna := range pool.Population {
		resp.Population = append(resp.Population, CurseMemberDTO{
			Generation:       dna.Generation,
			GamesPlayed:      dna.GamesPlayed,
			Fitness:          dna.Fitness,
			Aggression:       dna.Aggression,
			ComboPatience:    dna.ComboPat,
			ThreatParanoia:   dna.ThreatParanoia,
			ResourceGreed:    dna.ResourceGreed,
			PoliticalMemory:  dna.PoliticalMemory,
			DrainAffinity:    dna.DrainAffinity,
			ArtifactAffinity: dna.ArtifactAffinity,
		})
	}
	sm.curseMu.Unlock()

	writeJSON(w, resp)
}

// handlePatchCurse updates the deck owner's curse override map. Body is
// {"constraints": {"aggression": 0.9, ...}} — keys must be valid trait
// names (see hat.CurseTraitKeys), values must be in [0,1]. The existing
// pool's constraint map is replaced with the validated payload, the
// population is re-clamped into the new bands immediately, and the pool
// is persisted to disk so the override survives restarts.
func (sm *Showmatch) handlePatchCurse(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	if !checkOwnership(r, owner) {
		writeError(w, http.StatusForbidden, "forbidden: not deck owner")
		return
	}

	var body struct {
		Constraints map[string]float64 `json:"constraints"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	clean := make(map[string]float64, len(body.Constraints))
	for k, v := range body.Constraints {
		if !hat.IsValidCurseTrait(k) {
			writeError(w, http.StatusBadRequest, "invalid trait key: "+k)
			return
		}
		if v < 0 || v > 1 || math.IsNaN(v) {
			writeError(w, http.StatusBadRequest, "constraint value out of range [0,1]: "+k)
			return
		}
		clean[k] = v
	}

	deckKey := owner + "/" + id

	sm.curseMu.Lock()
	pool := sm.cursePool[deckKey]
	if pool == nil {
		sm.curseMu.Unlock()
		writeError(w, http.StatusNotFound, "no curse pool for deck")
		return
	}
	if len(clean) == 0 {
		pool.Constraints = nil
	} else {
		pool.Constraints = clean
	}
	pool.ApplyConstraintsToAll()

	// Snapshot for response under the same lock to avoid races.
	respConstraints := make(map[string]float64, len(pool.Constraints))
	for k, v := range pool.Constraints {
		respConstraints[k] = v
	}
	poolCopy := pool
	sm.curseMu.Unlock()

	if err := hat.SavePool(sm.curseDir, poolCopy); err != nil {
		log.Printf("curse: save after PATCH failed: %v", err)
		writeError(w, http.StatusInternalServerError, "save failed")
		return
	}

	writeJSON(w, map[string]any{
		"deck_key":    deckKey,
		"constraints": respConstraints,
	})
}

// recordAchievements maps a completed showmatch game into the
// achievements package's seat outcome shape and feeds it to the tracker.
// No-op when the tracker failed to initialize.
func (sm *Showmatch) recordAchievements(g CompletedGame) {
	if sm.achievements == nil {
		return
	}
	seats := make([]achievements.SeatOutcome, 0, len(g.FinalSeats))
	for i, seat := range g.FinalSeats {
		if i >= len(g.DeckKeys) {
			continue
		}
		deckKey := g.DeckKeys[i]
		owner, _ := deckOwnerFromKey(deckKey)
		if owner == "" {
			continue
		}
		seats = append(seats, achievements.SeatOutcome{
			Owner:     owner,
			DeckKey:   deckKey,
			Won:       i == g.Winner,
			FinalLife: seat.Life,
		})
	}
	if len(seats) == 0 {
		return
	}
	if err := sm.achievements.OnGameComplete(achievements.GameOutcome{
		Turns:      g.Turns,
		Seats:      seats,
		FinishedAt: g.FinishedAt,
	}); err != nil {
		log.Printf("achievements: record game: %v", err)
	}
}

func (sm *Showmatch) handleAchievements(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	if sm.achievements == nil {
		writeJSON(w, achievements.Snapshot{Owner: owner, Badges: []achievements.EarnedDetail{}})
		return
	}
	writeJSON(w, sm.achievements.Snapshot(owner))
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

type DriftReport struct {
	TotalDecks    int        `json:"total_decks"`
	OutliersAbove []ELOEntry `json:"outliers_above"`
	OutliersBelow []ELOEntry `json:"outliers_below"`
	AboveBracket  []ELOEntry `json:"above_bracket"`
	BelowBracket  []ELOEntry `json:"below_bracket"`
}

func (sm *Showmatch) handleLiveDrift(w http.ResponseWriter, r *http.Request) {
	all := sm.GetELO()
	report := DriftReport{TotalDecks: len(all)}
	for _, e := range all {
		if e.Games < 50 {
			continue
		}
		switch e.DriftTag {
		case "outlier_above":
			report.OutliersAbove = append(report.OutliersAbove, e)
		case "outlier_below":
			report.OutliersBelow = append(report.OutliersBelow, e)
		case "above_bracket":
			report.AboveBracket = append(report.AboveBracket, e)
		case "below_bracket":
			report.BelowBracket = append(report.BelowBracket, e)
		}
	}
	writeJSON(w, report)
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
		writeError(w, http.StatusBadRequest, "invalid game id")
		return
	}
	game := sm.GetGame(id)
	if game == nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	// Strip Timeline from the lightweight endpoint — callers on
	// /api/games and /api/games/{id} (history lists, dashboards) only
	// need summary fields. /api/games/{id}/report returns the full
	// timeline for the replay viewer.
	lite := *game
	lite.Timeline = nil
	writeJSON(w, lite)
}

// handleGameReport returns the CompletedGame with its per-turn
// Timeline populated, used by the post-game replay viewer.
// Returns the same struct as /api/games/{id} but does not strip
// Timeline. Older games rehydrated from SQLite will have an empty
// Timeline (per-turn data isn't persisted) — the UI handles that
// fallback by collapsing the scrubber.
func (sm *Showmatch) handleGameReport(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id := parseInt(idStr)
	if id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid game id")
		return
	}
	game := sm.GetGame(id)
	if game == nil {
		writeError(w, http.StatusNotFound, "game not found")
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
			// Phase 9: stamp InstanceID from event Details when present.
			// Engine writers store the affected object's ID under one of
			// "instance_id", "card_instance_id", or "source_instance_id"
			// depending on event kind — read in priority order.
			if id := instanceIDFromDetails(ev.Details); id != "" {
				entry.InstanceID = id
			}
			entries = append(entries, entry)
		}
	}
	return coalesceEntries(dedupEntries(entries))
}

func commanderLabel(seat int, commanders []string) string {
	if seat >= 0 && seat < len(commanders) {
		parts := strings.Split(commanders[seat], ",")
		cmdr := strings.TrimSpace(parts[0])
		if len(cmdr) > 20 {
			cmdr = cmdr[:20]
		}
		return strings.ToUpper(cmdr)
	}
	return "SYSTEM"
}

func targetLabel(seat int, commanders []string) string {
	if seat >= 0 && seat < len(commanders) {
		return strings.ToUpper(strings.TrimSpace(strings.Split(commanders[seat], ",")[0]))
	}
	return ""
}

func formatEvent(ev gameengine.Event, commanders []string, turn int) (LogEntry, bool) {
	seat := ev.Seat
	cmdr := commanderLabel(seat, commanders)

	switch ev.Kind {
	case "cast":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " CASTS " + strings.ToUpper(ev.Source), Kind: "cast", Source: ev.Source, Amount: 1}, true
	case "play_land":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " PLAYS LAND: " + strings.ToUpper(ev.Source), Kind: "land", Source: ev.Source}, true
	case "declare_attackers":
		n := ev.Amount
		if n <= 0 {
			n = 1
		}
		target := ""
		var targets []string
		if ev.Target >= 0 && ev.Target < len(commanders) {
			target = " → " + targetLabel(ev.Target, commanders)
			targets = []string{targetLabel(ev.Target, commanders)}
		}
		return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s ATTACKS WITH %d CREATURE(S)%s", cmdr, n, target), Kind: "combat", Amount: n, Targets: targets}, true
	case "damage":
		tgt := targetLabel(ev.Target, commanders)
		if ev.Amount > 0 && tgt != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s DEALS %d DAMAGE TO %s", cmdr, ev.Amount, tgt), Detail: ev.Source, Kind: "damage", Source: ev.Source, Targets: []string{tgt}, Amount: ev.Amount}, true
		}
	case "counter_spell":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " COUNTERS " + strings.ToUpper(ev.Source), Kind: "counter", Source: ev.Source}, true
	case "counter_spell_fizzle":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " — SPELL FIZZLES (no legal targets)", Kind: "counter", Source: ev.Source}, true
	case "create_token":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " CREATES TOKEN: " + strings.ToUpper(ev.Source), Kind: "token", Source: ev.Source, Amount: 1}, true
	case "destroy":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " DESTROYS " + strings.ToUpper(ev.Source), Kind: "removal", Source: ev.Source}, true
	case "sacrifice":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " SACRIFICES " + strings.ToUpper(ev.Source), Kind: "removal", Source: ev.Source}, true
	case "exile":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " EXILES " + strings.ToUpper(ev.Source), Kind: "removal", Source: ev.Source}, true
		}
	case "bounce":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " BOUNCES " + strings.ToUpper(ev.Source), Kind: "removal", Source: ev.Source}, true
		}
	case "flicker":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " FLICKERS " + strings.ToUpper(ev.Source), Kind: "removal", Source: ev.Source}, true
		}
	case "gain_life":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s GAINS %d LIFE", cmdr, ev.Amount), Kind: "life", Amount: ev.Amount}, true
		}
	case "lose_life":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s LOSES %d LIFE", cmdr, ev.Amount), Kind: "life", Amount: ev.Amount}, true
		}
	case "pay_life":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s PAYS %d LIFE", cmdr, ev.Amount), Kind: "life", Source: ev.Source, Amount: ev.Amount}, true
		}
	case "draw":
		if ev.Amount > 1 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s DRAWS %d CARDS", cmdr, ev.Amount), Kind: "draw", Amount: ev.Amount}, true
		}
		if ev.Amount == 1 {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " DRAWS A CARD", Kind: "draw", Amount: 1}, true
		}
	case "discard":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " DISCARDS " + strings.ToUpper(ev.Source), Kind: "discard", Source: ev.Source, Amount: ev.Amount}, true
		}
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s DISCARDS %d CARDS", cmdr, ev.Amount), Kind: "discard", Amount: ev.Amount}, true
		}
	case "scry":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s SCRIES %d", cmdr, ev.Amount), Kind: "scry", Amount: ev.Amount}, true
		}
	case "surveil":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s SURVEILS %d", cmdr, ev.Amount), Kind: "surveil", Amount: ev.Amount}, true
		}
	case "tutor":
		dest := ""
		if ev.Details != nil {
			if d, ok := ev.Details["destination"].(string); ok {
				dest = " TO " + strings.ToUpper(d)
			}
		}
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " TUTORS" + dest, Kind: "search", Source: ev.Source, Amount: ev.Amount}, true
	case "shuffle":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " SHUFFLES LIBRARY", Kind: "shuffle"}, true
	case "untap_done":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " UNTAPS " + strings.ToUpper(ev.Source), Kind: "untap", Source: ev.Source, Amount: 1}, true
	case "untap":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " UNTAPS " + strings.ToUpper(ev.Source), Kind: "untap", Source: ev.Source, Amount: 1}, true
		}
	case "tap":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " TAPS " + strings.ToUpper(ev.Source), Kind: "tap", Source: ev.Source, Amount: 1}, true
		}
	case "equip", "attach":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " EQUIPS " + strings.ToUpper(ev.Source), Kind: "equip", Source: ev.Source}, true
		}
	case "seat_eliminated", "lose_game":
		reason := ""
		if ev.Details != nil {
			if r, ok := ev.Details["reason"].(string); ok {
				reason = r
			}
		}
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " IS ELIMINATED", Detail: reason, Kind: "elimination"}, true
	case "enter_battlefield":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " → ETB: " + strings.ToUpper(ev.Source), Kind: "etb", Source: ev.Source}, true
		}
	case "triggered_ability", "triggered":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " TRIGGERS " + strings.ToUpper(ev.Source), Kind: "trigger", Source: ev.Source}, true
		}
	case "activate_ability", "activated":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " ACTIVATES " + strings.ToUpper(ev.Source), Kind: "activate", Source: ev.Source}, true
		}
	case "mill":
		if ev.Amount > 0 {
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s MILLS %d CARDS", cmdr, ev.Amount), Kind: "mill", Amount: ev.Amount}, true
		}
	case "search_library", "search":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " SEARCHES LIBRARY", Kind: "search"}, true
	case "reanimate":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " REANIMATES " + strings.ToUpper(ev.Source), Kind: "reanimate", Source: ev.Source}, true
	case "return_to_hand":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " RETURNS " + strings.ToUpper(ev.Source) + " TO HAND", Kind: "bounce", Source: ev.Source}, true
		}
	case "extra_turn":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " TAKES AN EXTRA TURN", Kind: "extra_turn"}, true
	case "commander_cast_from_command_zone":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " CASTS COMMANDER FROM COMMAND ZONE", Kind: "cast", Source: ev.Source}, true
	case "become_monarch":
		return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " BECOMES THE MONARCH", Kind: "monarch"}, true
	case "cascade_hit":
		if ev.Source != "" {
			return LogEntry{Turn: turn, Seat: seat, Action: cmdr + " CASCADE → " + strings.ToUpper(ev.Source), Kind: "cast", Source: ev.Source}, true
		}
	case "annihilator":
		if ev.Amount > 0 {
			tgt := targetLabel(ev.Target, commanders)
			return LogEntry{Turn: turn, Seat: seat, Action: fmt.Sprintf("%s ANNIHILATOR %d → %s", cmdr, ev.Amount, tgt), Kind: "combat", Amount: ev.Amount, Targets: []string{tgt}}, true
		}
	}
	return LogEntry{}, false
}

// dedupEntries removes redundant entries where a cast is immediately
// followed by an ETB for the same card from the same seat (the cast
// already implies the ETB for narration purposes).
func dedupEntries(entries []LogEntry) []LogEntry {
	if len(entries) <= 1 {
		return entries
	}
	result := make([]LogEntry, 0, len(entries))
	for i := 0; i < len(entries); i++ {
		if entries[i].Kind == "etb" && i > 0 {
			prev := entries[i-1]
			if prev.Seat == entries[i].Seat && prev.Kind == "cast" &&
				strings.EqualFold(prev.Source, entries[i].Source) {
				continue
			}
		}
		result = append(result, entries[i])
	}
	return result
}

// coalesceEntries merges consecutive same-seat same-kind entries into
// grouped entries with Count > 1 and aggregated Targets.
func coalesceEntries(entries []LogEntry) []LogEntry {
	if len(entries) <= 1 {
		return entries
	}

	coalescible := map[string]bool{
		"untap":   true,
		"tap":     true,
		"trigger": true,
		"draw":    true,
		"token":   true,
	}

	result := make([]LogEntry, 0, len(entries))
	i := 0
	for i < len(entries) {
		e := entries[i]
		if !coalescible[e.Kind] {
			result = append(result, e)
			i++
			continue
		}

		// Collect consecutive same-seat same-kind entries.
		j := i + 1
		for j < len(entries) && entries[j].Seat == e.Seat && entries[j].Kind == e.Kind {
			j++
		}
		count := j - i
		if count == 1 {
			result = append(result, e)
			i++
			continue
		}

		// Merge: aggregate targets and amounts.
		var targets []string
		totalAmount := 0
		seen := map[string]bool{}
		for k := i; k < j; k++ {
			totalAmount += entries[k].Amount
			if entries[k].Source != "" && !seen[entries[k].Source] {
				targets = append(targets, entries[k].Source)
				seen[entries[k].Source] = true
			}
		}

		// Build coalesced action string.
		cmdr := ""
		if idx := strings.Index(e.Action, " "); idx > 0 {
			cmdr = e.Action[:idx]
		}
		var action string
		switch e.Kind {
		case "untap":
			action = fmt.Sprintf("%s UNTAPS %d PERMANENTS", cmdr, count)
		case "tap":
			action = fmt.Sprintf("%s TAPS %d PERMANENTS", cmdr, count)
		case "trigger":
			action = fmt.Sprintf("%s TRIGGERS %d ABILITIES", cmdr, count)
		case "draw":
			action = fmt.Sprintf("%s DRAWS %d CARDS", cmdr, totalAmount)
		case "token":
			action = fmt.Sprintf("%s CREATES %d TOKENS", cmdr, count)
		default:
			action = fmt.Sprintf("%s ×%d", e.Action, count)
		}

		result = append(result, LogEntry{
			Turn:    e.Turn,
			Seat:    e.Seat,
			Action:  action,
			Kind:    e.Kind,
			Targets: targets,
			Amount:  totalAmount,
			Count:   count,
		})
		i = j
	}
	return result
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

var bannedCommanders = map[string]bool{
	"braids, cabal minion":        true,
	"emrakul, the aeons torn":     true,
	"erayo, soratami ascendant":   true,
	"golos, tireless pilgrim":     true,
	"griselbrand":                 true,
	"iona, shield of emeria":      true,
	"leovold, emissary of trest":  true,
	"lutri, the spellchaser":      true,
	"nadu, winged wisdom":         true,
	"rofellos, llanowar emissary": true,
}

func commanderBanned(name string) bool {
	return bannedCommanders[strings.ToLower(strings.TrimSpace(name))]
}

// buildDeckPool discovers and parses every deck under decksDir using
// the supplied corpus + meta, returning the valid pool plus the Freya
// strategy and bracket caches. Pure (no Showmatch state mutation), so
// startup (loadAndRun) and live reload (reloadDeckPool) share one
// implementation and can't drift in their filtering/keying rules.
func buildDeckPool(decksDir string, corpus *astload.Corpus, meta *deckparser.MetaDB) ([]*deckparser.TournamentDeck, map[string]int, map[string]*hat.StrategyProfile, error) {
	deckPaths, err := findDeckFiles(decksDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("find decks failed: %w", err)
	}

	var decks []*deckparser.TournamentDeck
	var bannedSkipped, parseSkipped, tooSmallSkipped, noCmdrSkipped int
	for _, p := range deckPaths {
		d, perr := deckparser.ParseDeckFile(p, corpus, meta)
		if perr != nil {
			parseSkipped++
			log.Printf("showmatch: parse skip %s: %v", filepath.Base(p), perr)
			continue
		}
		totalCards := len(d.Library) + len(d.CommanderCards)
		if len(d.CommanderCards) == 0 {
			noCmdrSkipped++
			log.Printf("showmatch: no commander %s (%d cards)", deckKeyFromPath(p), totalCards)
			continue
		}
		if totalCards < 80 {
			tooSmallSkipped++
			log.Printf("showmatch: too small %s (%d cards)", deckKeyFromPath(p), totalCards)
			continue
		}
		if commanderBanned(d.CommanderName) {
			bannedSkipped++
			continue
		}
		decks = append(decks, d)
	}
	if parseSkipped+tooSmallSkipped+noCmdrSkipped+bannedSkipped > 0 {
		log.Printf("showmatch: filtered %d decks (parse=%d noCmd=%d small=%d banned=%d)",
			parseSkipped+tooSmallSkipped+noCmdrSkipped+bannedSkipped,
			parseSkipped, noCmdrSkipped, tooSmallSkipped, bannedSkipped)
	}
	log.Printf("showmatch: %d decks parsed successfully (from %d files)", len(decks), len(deckPaths))

	bracketMap := make(map[string]int, len(decks))
	stratMap := make(map[string]*hat.StrategyProfile, len(decks))
	bracketCounts := [6]int{}
	stratLoaded := 0
	for _, d := range decks {
		key := deckKeyFromPath(d.Path)
		sp := hat.LoadStrategyFromFreya(d.Path)
		if sp != nil {
			stratMap[key] = sp
			stratLoaded++
			if sp.Bracket >= 1 && sp.Bracket <= 5 {
				bracketMap[key] = sp.Bracket
				bracketCounts[sp.Bracket]++
			}
		}
	}
	log.Printf("showmatch: loaded %d/%d Freya strategy profiles", stratLoaded, len(decks))
	log.Printf("showmatch: HexELO bracket cache: B1=%d B2=%d B3=%d B4=%d B5=%d (%d unclassified)",
		bracketCounts[1], bracketCounts[2], bracketCounts[3], bracketCounts[4], bracketCounts[5],
		len(decks)-bracketCounts[1]-bracketCounts[2]-bracketCounts[3]-bracketCounts[4]-bracketCounts[5])

	return decks, bracketMap, stratMap, nil
}

// reloadDeckPool re-discovers and re-parses the deck pool from sm.decksDir
// using the already-loaded corpus + meta, then atomically swaps the pool,
// bracket, and strategy caches. Safe to call while the grinder runs —
// every deckPool reader takes sm.mu.RLock and this swaps under Lock.
// Lets newly imported decks enter the pool without a server restart.
func (sm *Showmatch) reloadDeckPool() (int, error) {
	sm.mu.RLock()
	corpus, meta, decksDir := sm.corpus, sm.meta, sm.decksDir
	sm.mu.RUnlock()
	if corpus == nil || meta == nil {
		return 0, fmt.Errorf("corpus/meta not loaded yet")
	}
	if decksDir == "" {
		return 0, fmt.Errorf("decks directory not configured")
	}
	decks, bracketMap, stratMap, err := buildDeckPool(decksDir, corpus, meta)
	if err != nil {
		return 0, err
	}
	if len(decks) < showmatchSeats {
		return 0, fmt.Errorf("only %d valid decks (need %d)", len(decks), showmatchSeats)
	}
	sm.mu.Lock()
	sm.deckPool = decks
	sm.bracketCache = bracketMap
	sm.strategies = stratMap
	sm.mu.Unlock()
	log.Printf("showmatch: pool reloaded — %d decks", len(decks))
	return len(decks), nil
}

func findDeckFiles(dir string) ([]string, error) {
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "freya" || base == "benched" || base == "test" || base == "moxfield_300" || base == ".versions" {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(info.Name())
		switch {
		case strings.HasSuffix(name, ".txt"):
			paths = append(paths, path)
		case strings.HasSuffix(name, ".strategy.json"), strings.HasSuffix(name, ".profile.json"):
			// Freya/import sidecars, not decks — skip.
		case strings.HasSuffix(name, ".json"):
			// Structured deck format (UI/import-served). ParseDeckFile
			// detects + converts these so the pool, deck list, and deck
			// read all share one source of truth.
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
		writeError(w, http.StatusServiceUnavailable, "too many spectators")
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
		var env struct {
			Type string `json:"type"`
		}
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

func (sm *Showmatch) handleOwnerStats(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	if owner == "" || sm.sqlDB == nil {
		writeJSON(w, map[string]any{"games": 0, "wins": 0, "losses": 0, "win_rate": 0})
		return
	}
	stats, err := db.LoadOwnerStats(r.Context(), sm.sqlDB, strings.ToLower(owner))
	if err != nil {
		writeJSON(w, map[string]any{"games": 0, "wins": 0, "losses": 0, "win_rate": 0})
		return
	}
	writeJSON(w, stats)
}

func (sm *Showmatch) handleOwnerGames(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if owner == "" || sm.sqlDB == nil {
		writeJSON(w, []any{})
		return
	}
	games, err := db.LoadOwnerGames(r.Context(), sm.sqlDB, strings.ToLower(owner), limit)
	if err != nil {
		writeJSON(w, []any{})
		return
	}
	if games == nil {
		games = []db.OwnerGameRow{}
	}
	writeJSON(w, games)
}
