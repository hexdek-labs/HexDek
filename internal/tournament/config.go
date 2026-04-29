// Package tournament is the mtgsquad Go parallel tournament runner.
//
// Runs N games across a goroutine pool, with per-seat Hat factories for
// decision policy, collects winrate + elimination + crash stats, and
// optionally emits a markdown report matching
// scripts/gauntlet_poker.py's output layout.
//
// Concurrency contract:
//
//   - The AST corpus + deckparser MetaDB + deck templates are shared
//     READ-ONLY across all goroutines.
//   - Each goroutine builds its own *gameengine.GameState + own Hat
//     instances (one per seat, per game, via HatFactory) and owns that
//     state for the duration of the game. Mutable state NEVER crosses
//     goroutine boundaries except through the buffered results channel.
//   - Per-game crashes are contained by a recover() on the worker
//     goroutine — a bad card / panicky effect registers as a crash
//     without killing the rest of the tournament.
//
// Determinism: each game uses RNG seeded `config.Seed + gameIdx*1000+1`
// (matches the Python reference); running the same config twice
// yields the same winner per game.
package tournament

import (
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// HatFactory produces a fresh Hat instance for a single seat in a
// single game. Factories MUST return new values every call — no shared
// state between games. Stateless hats (GreedyHat) may return the same
// pointer per factory invocation, but adaptive hats (PokerHat) MUST
// allocate so per-game observations stay isolated.
type HatFactory func() gameengine.Hat

// TournamentConfig defines a single tournament run.
type TournamentConfig struct {
	// Decks are the pre-parsed deck templates. len(Decks) MUST be >= 2;
	// the runner uses the first NSeats decks and rotates them.
	Decks []*deckparser.TournamentDeck

	// NSeats is the number of seats per game. 4 for standard EDH, 6
	// for 6p, etc. The runner takes the first NSeats entries of
	// Decks and rotates them each game so every deck plays every seat.
	NSeats int

	// NGames is the total number of games to simulate.
	NGames int

	// Seed is the master RNG seed. Per-game seed = Seed + gameIdx*1000 + 1.
	Seed int64

	// HatFactories is a per-seat slice of HatFactory funcs. len==NSeats
	// makes each seat pick its own Hat; len==1 means uniform (every
	// seat gets the same factory); len==0 defaults to GreedyHat for all.
	HatFactories []HatFactory

	// Workers is the goroutine pool size. 0 = runtime.NumCPU().
	Workers int

	// CommanderMode enables CR §903: 40 life, command zone. Always true
	// for the standard gauntlet; set false to test vanilla 20-life
	// format parity.
	CommanderMode bool

	// AuditEnabled captures the full event stream per game for
	// post-run rule auditing. WARNING: ~10x the memory footprint of
	// a 50k-game run; only enable for smaller diagnostic runs.
	AuditEnabled bool

	// AnalyticsEnabled runs per-game deep analytics (Heimdall mode).
	// After each game, the event log is processed to produce per-card,
	// per-player stats. The compact GameAnalysis is attached to the
	// outcome. Memory overhead is modest (one GameAnalysis per game,
	// ~1-5KB each) compared to AuditEnabled's full event log retention.
	AnalyticsEnabled bool

	// ReportPath is the optional markdown output path. When non-empty,
	// Run writes a TournamentResult.WriteMarkdown report after the
	// tournament completes.
	ReportPath string

	// MaxTurnsPerGame caps the turn count to prevent runaway games.
	// 0 means the default (80 for multiplayer commander).
	MaxTurnsPerGame int

	// ProgressLogEvery toggles progress logging every N games.
	// 0 = use default (every 1000 games or every 5% whichever is larger).
	ProgressLogEvery int

	// ProgressLogger is an optional callback that receives progress
	// updates. If nil AND ProgressLogEvery > 0, progress is logged to
	// stderr.
	ProgressLogger func(done, total int, gps float64)

	// PoolMode enables random pod sampling: each game picks NSeats
	// random decks from the full Decks slice instead of always using
	// the first NSeats. Used for bug-hunting across large deck pools.
	PoolMode bool

	// LazyPool enables on-demand deck loading for large pools. Instead
	// of holding all decks in memory, only active game decks are loaded.
	// Requires DeckPaths, Corpus, and Meta to be set. Decks field is ignored.
	LazyPool bool

	// DeckPaths holds file paths for lazy loading. Used with LazyPool.
	DeckPaths []string

	// Corpus is the shared AST corpus for lazy deck loading.
	Corpus *astload.Corpus

	// Meta is the shared card metadata for lazy deck loading.
	Meta *deckparser.MetaDB

	// GameTimeout overrides the per-game wall-clock timeout.
	// 0 means the default (3 minutes).
	GameTimeout time.Duration

	// PprofEnabled triggers a heap profile dump after the first game
	// completes, for diagnosing memory leaks.
	PprofEnabled bool
}

// defaultMaxTurns mirrors playloop.MAX_TURNS_MULTIPLAYER.
const defaultMaxTurns = 80

// DefaultMaxTurns is the exported turn cap so downstream callers / tests
// can reference the same constant.
const DefaultMaxTurns = defaultMaxTurns

// defaultBufferMult is the size of the results channel (N games
// cushion before back-pressure). 4x Workers is enough for typical
// workloads.
const defaultBufferMult = 4
