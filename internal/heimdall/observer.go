package heimdall

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

const (
	seedBufSize = 1000
)

// Observer is the singleton that receives game results from ALL game paths
// (grinder, fishtank, gauntlet). It buffers seeds in memory and flushes to
// disk in batches, then routes observations to downstream sinks (Huginn,
// Muninn, telemetry).
type Observer struct {
	seedBuf   []GameSeed
	huginn    HuginnSink
	muninn    MuninnSink
	telemetry TelemetrySink
	snapshot  SnapshotSink // optional; wired via SetSnapshotSink
	dataDir   string
	mu        sync.Mutex
}

// New creates an Observer. Sinks can be nil (observations are silently dropped).
func New(dataDir string, huginn HuginnSink, muninn MuninnSink, telemetry TelemetrySink) *Observer {
	o := &Observer{
		seedBuf:   make([]GameSeed, 0, seedBufSize),
		huginn:    huginn,
		muninn:    muninn,
		telemetry: telemetry,
		dataDir:   dataDir,
	}
	os.MkdirAll(filepath.Join(dataDir, "heimdall"), 0755)
	return o
}

// SetSnapshotSink wires (or rewires) the optional SnapshotSink for
// per-game observation persistence. Pass nil to disable. Kept as a
// setter rather than a New() argument so callers that don't have a
// DB-backed adapter at startup (test harnesses, ad-hoc replays) can
// continue to construct an Observer with the existing four-arg
// New(); production code paths attach the sink after the DB
// connection is ready.
func (o *Observer) SetSnapshotSink(s SnapshotSink) {
	o.mu.Lock()
	o.snapshot = s
	o.mu.Unlock()
}

// HasSnapshotSink reports whether a sink is currently wired. Used
// by callers that want to skip the snapshot-building work when no
// sink will receive it. Not strictly necessary — RecordSnapshot
// short-circuits on nil — but useful for skipping the GameState
// walk in SnapshotFromObservation when nobody's listening.
func (o *Observer) HasSnapshotSink() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.snapshot != nil
}

// RecordSnapshot builds a GameObservationSnapshot from the current
// observation + game state and persists it via the wired
// SnapshotSink. Designed for incremental use: live game paths can
// invoke this multiple times during a game (e.g., once per turn,
// or on each major event) and each call upserts the latest state.
// No-op when no SnapshotSink is wired.
//
// Errors from the sink are logged rather than returned — the live
// game must not stall on a persistence failure. Operators read the
// log to spot persistence breakage; clients that need to know
// whether a snapshot landed should query the read-side
// /api/games/{id}/summary directly.
//
// gs may be nil: the resulting snapshot will carry just the obs
// signals + Seed (turning-point reconstruction at read time
// degrades to game_end only). Callers that have a live GameState
// should pass it for the richer reconstruction.
func (o *Observer) RecordSnapshot(ctx context.Context, gameID int64, obs Observation, gs *gameengine.GameState) {
	o.mu.Lock()
	sink := o.snapshot
	o.mu.Unlock()
	if sink == nil {
		return
	}
	snap := SnapshotFromObservation(obs, gs)
	if err := sink.PersistGameObservation(ctx, gameID, snap); err != nil {
		log.Printf("heimdall: persist snapshot for game %d: %v", gameID, err)
	}
}

// RecordSeed is called after EVERY game. Zero-allocation fast path -- appends
// to ring buffer, flushes to disk when full.
func (o *Observer) RecordSeed(seed GameSeed) {
	o.mu.Lock()
	o.seedBuf = append(o.seedBuf, seed)
	if len(o.seedBuf) >= seedBufSize {
		seeds := make([]GameSeed, len(o.seedBuf))
		copy(seeds, o.seedBuf)
		o.seedBuf = o.seedBuf[:0]
		o.mu.Unlock()
		o.flushSeeds(seeds)
		return
	}
	o.mu.Unlock()
}

// RecordObservation is called during batch replay or live observation mode.
// Routes co-triggers + zone-cast + exile-link events to Huginn, and parser
// gaps + dead triggers + (optionally) exile-link audit data to Muninn.
func (o *Observer) RecordObservation(obs Observation) {
	gameID := fmt.Sprintf("%d", obs.Seed.RNGSeed)
	names := make([]string, len(obs.Seed.DeckKeys))
	copy(names, obs.Seed.DeckKeys[:])

	if o.huginn != nil {
		if len(obs.CoTriggers) > 0 {
			o.huginn.IngestCoTriggers(obs.CoTriggers, names)
		}
		if len(obs.ZoneCastEvents) > 0 {
			if zc, ok := o.huginn.(HuginnZoneCastSink); ok {
				zc.IngestZoneCastEvents(obs.ZoneCastEvents, names, gameID)
			}
		}
		if len(obs.ExileLinkEvents) > 0 {
			if el, ok := o.huginn.(HuginnExileLinkSink); ok {
				el.IngestExileLinkEvents(obs.ExileLinkEvents, names, gameID)
			}
		}
	}
	if o.muninn != nil {
		if len(obs.ParserGaps) > 0 {
			o.muninn.RecordParserGaps(obs.ParserGaps, gameID)
		}
		if len(obs.DeadTriggers) > 0 {
			o.muninn.RecordDeadTriggers(obs.DeadTriggers, gameID)
		}
		if len(obs.ExileLinkEvents) > 0 {
			if el, ok := o.muninn.(MuninnExileLinkSink); ok {
				el.RecordExileLinkEvents(obs.ExileLinkEvents, gameID)
			}
		}
	}
	// R60: persist per-game composition prior effects for the replay
	// CLI (PR #422). Best-effort, no-op when the obs carries no
	// effects (pre-#420 paths or games that ran with a nil prior).
	if len(obs.CompositionPriorEffects) > 0 {
		writeCompositionPriorRecord(o.dataDir, CompositionPriorReplayRecord{
			GameSeed: obs.Seed,
			Effects:  obs.CompositionPriorEffects,
		})
	}
}

// RecordCrash is called from panic recovery. Routes to Muninn immediately.
func (o *Observer) RecordCrash(panicMsg string, stack []byte, deckKeys []string) {
	if o.muninn != nil {
		o.muninn.RecordCrash(panicMsg, string(stack), deckKeys)
	}
}

// Pulse forwards a health pulse to the telemetry sink (GA4).
func (o *Observer) Pulse(stats HealthPulse) {
	if o.telemetry != nil {
		o.telemetry.Pulse(stats)
	}
}

// Flush writes any buffered seeds to disk. Call on graceful shutdown.
func (o *Observer) Flush() {
	o.mu.Lock()
	if len(o.seedBuf) > 0 {
		seeds := make([]GameSeed, len(o.seedBuf))
		copy(seeds, o.seedBuf)
		o.seedBuf = o.seedBuf[:0]
		o.mu.Unlock()
		o.flushSeeds(seeds)
		return
	}
	o.mu.Unlock()
}

func (o *Observer) flushSeeds(seeds []GameSeed) {
	fname := filepath.Join(o.dataDir, "heimdall", "seeds.jsonl")
	if info, err := os.Stat(fname); err == nil && info.Size() > maxSeedFileSize {
		os.Rename(fname, fname+".prev")
	}
	f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, s := range seeds {
		enc.Encode(s)
	}
}
