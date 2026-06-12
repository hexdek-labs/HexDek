package muninn

import (
	"github.com/hexdek/hexdek/internal/judge"
	"sync"
	"time"
)

// DefaultBatchSize is the number of accumulated items that triggers a
// synchronous flush. Tuned so that under typical tournament throughput
// flushes happen on the order of seconds, not per game.
const DefaultBatchSize = 256

// DefaultGamesPerFlush is the number of EndGame() ticks that triggers a
// synchronous flush. Combined with FlushInterval, this enforces the
// "every 30s or 100 games, whichever comes first" guarantee.
const DefaultGamesPerFlush = 100

// DefaultFlushInterval is the wall-clock interval at which the background
// flusher writes any pending buffer contents to disk.
const DefaultFlushInterval = 30 * time.Second

// BatcherConfig configures a Batcher. Zero values fall back to the
// Default* constants.
type BatcherConfig struct {
	Dir           string
	BatchSize     int
	GamesPerFlush int
	FlushInterval time.Duration
}

// Batcher accumulates muninn writes in memory and flushes them in batches
// to disk. It removes the per-game read-modify-write hot path that the
// raw Persist* helpers exhibit.
//
// A Batcher is safe for concurrent use. Call Close on graceful shutdown
// to stop the background flusher and persist any remaining buffer.
type Batcher struct {
	dir           string
	batchSize     int
	gamesPerFlush int
	flushInterval time.Duration

	mu           sync.Mutex
	parserGaps   map[string]int
	crashes      []pendingCrash
	deadTriggers map[deadTrigKey]*deadTrigAccum
	concessions  []ConcessionRecord
	invariants   []ArchivedViolation
	regressions  []RegressionFailure
	pending      int
	games        int

	totalGaps       int
	totalCrashes    int
	totalDeadTrigs  int
	totalInvariants int

	stop   chan struct{}
	doneWG sync.WaitGroup
	closed bool
}

type pendingCrash struct {
	StackTrace     string
	CommanderNames []string
	NGames         int
	NSeats         int
}

type deadTrigKey struct {
	triggerName string
	cardName    string
}

type deadTrigAccum struct {
	count     int
	gamesSeen int
}

// NewBatcher creates a Batcher and starts the periodic flush goroutine.
// If FlushInterval is positive, a background goroutine flushes pending
// data every interval. Callers MUST call Close on shutdown.
func NewBatcher(cfg BatcherConfig) *Batcher {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.GamesPerFlush <= 0 {
		cfg.GamesPerFlush = DefaultGamesPerFlush
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = DefaultFlushInterval
	}
	b := &Batcher{
		dir:           cfg.Dir,
		batchSize:     cfg.BatchSize,
		gamesPerFlush: cfg.GamesPerFlush,
		flushInterval: cfg.FlushInterval,
		parserGaps:    make(map[string]int),
		deadTriggers:  make(map[deadTrigKey]*deadTrigAccum),
		stop:          make(chan struct{}),
	}
	b.doneWG.Add(1)
	go b.run()
	return b
}

func (b *Batcher) run() {
	defer b.doneWG.Done()
	t := time.NewTicker(b.flushInterval)
	defer t.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-t.C:
			_ = b.Flush()
		}
	}
}

// AddParserGaps merges per-snippet counts into the in-memory buffer.
func (b *Batcher) AddParserGaps(gaps map[string]int) {
	if len(gaps) == 0 {
		return
	}
	b.mu.Lock()
	for snippet, count := range gaps {
		if count <= 0 {
			continue
		}
		b.parserGaps[snippet] += count
		b.pending++
		b.totalGaps += count
	}
	flush := b.pending >= b.batchSize
	b.mu.Unlock()
	if flush {
		_ = b.Flush()
	}
}

// AddCrash buffers a crash log entry.
func (b *Batcher) AddCrash(stackTrace string, commanderNames []string, nGames, nSeats int) {
	b.mu.Lock()
	b.crashes = append(b.crashes, pendingCrash{
		StackTrace:     stackTrace,
		CommanderNames: append([]string(nil), commanderNames...),
		NGames:         nGames,
		NSeats:         nSeats,
	})
	b.pending++
	b.totalCrashes++
	flush := b.pending >= b.batchSize
	b.mu.Unlock()
	if flush {
		_ = b.Flush()
	}
}

// AddDeadTrigger increments count and gamesSeen for a (triggerName, cardName)
// pair. Mirrors the per-record increment semantics of the legacy adapter.
func (b *Batcher) AddDeadTrigger(triggerName, cardName string, count, gamesSeen int) {
	if count <= 0 && gamesSeen <= 0 {
		return
	}
	k := deadTrigKey{triggerName: triggerName, cardName: cardName}
	b.mu.Lock()
	a := b.deadTriggers[k]
	if a == nil {
		a = &deadTrigAccum{}
		b.deadTriggers[k] = a
	}
	a.count += count
	a.gamesSeen += gamesSeen
	b.pending++
	b.totalDeadTrigs += count
	flush := b.pending >= b.batchSize
	b.mu.Unlock()
	if flush {
		_ = b.Flush()
	}
}

// AddConcessions buffers concession records.
func (b *Batcher) AddConcessions(records []ConcessionRecord) {
	if len(records) == 0 {
		return
	}
	b.mu.Lock()
	b.concessions = append(b.concessions, records...)
	b.pending += len(records)
	flush := b.pending >= b.batchSize
	b.mu.Unlock()
	if flush {
		_ = b.Flush()
	}
}

// AddInvariantViolations buffers Odin invariant violations.
func (b *Batcher) AddInvariantViolations(violations []ArchivedViolation) {
	if len(violations) == 0 {
		return
	}
	b.mu.Lock()
	b.invariants = append(b.invariants, violations...)
	b.pending += len(violations)
	b.totalInvariants += len(violations)
	flush := b.pending >= b.batchSize
	b.mu.Unlock()
	if flush {
		_ = b.Flush()
	}
}

// AddAutoArchive buffers Feynman/Odin invariant violations from one game,
// tagged with the RNG seed and deck keys so the regression runner can
// replay it. Mirrors AutoArchiveViolation but goes through the batch.
func (b *Batcher) AddAutoArchive(rngSeed int64, deckKeys [4]string, violations []judge.ValidationViolation) {
	if len(violations) == 0 {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	records := make([]ArchivedViolation, 0, len(violations))
	for _, v := range violations {
		records = append(records, NewArchivedViolation(rngSeed, deckKeys, now, v))
	}
	b.AddInvariantViolations(records)
}

// EndGame ticks the per-game counter. When the counter reaches
// GamesPerFlush, a synchronous Flush is triggered. Callers should invoke
// this once per completed game even if no buffered entries were added —
// the counter drives the "100 games" leg of the flush guarantee.
func (b *Batcher) EndGame() {
	b.mu.Lock()
	b.games++
	flush := b.games >= b.gamesPerFlush
	b.mu.Unlock()
	if flush {
		_ = b.Flush()
	}
}

// AddRegressionFailures buffers parity regression failures.
func (b *Batcher) AddRegressionFailures(failures []RegressionFailure) {
	if len(failures) == 0 {
		return
	}
	b.mu.Lock()
	b.regressions = append(b.regressions, failures...)
	b.pending += len(failures)
	flush := b.pending >= b.batchSize
	b.mu.Unlock()
	if flush {
		_ = b.Flush()
	}
}

// Flush writes all buffered data to disk in a single read-modify-write
// per file. Safe to call concurrently with Add* and the background ticker.
func (b *Batcher) Flush() error {
	b.mu.Lock()
	b.games = 0
	if b.pending == 0 {
		b.mu.Unlock()
		return nil
	}
	gaps := b.parserGaps
	crashes := b.crashes
	dead := b.deadTriggers
	concessions := b.concessions
	invariants := b.invariants
	regressions := b.regressions

	b.parserGaps = make(map[string]int)
	b.crashes = nil
	b.deadTriggers = make(map[deadTrigKey]*deadTrigAccum)
	b.concessions = nil
	b.invariants = nil
	b.regressions = nil
	b.pending = 0
	b.mu.Unlock()

	var firstErr error
	if len(gaps) > 0 {
		if err := PersistParserGaps(b.dir, gaps); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(crashes) > 0 {
		if err := flushCrashes(b.dir, crashes); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(dead) > 0 {
		if err := flushDeadTriggers(b.dir, dead); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(concessions) > 0 {
		if err := PersistConcessions(b.dir, concessions); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(invariants) > 0 {
		if err := PersistInvariantViolations(b.dir, invariants); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(regressions) > 0 {
		if err := PersistRegressionFailures(b.dir, regressions); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// BatcherStats holds lightweight counters for health telemetry.
type BatcherStats struct {
	ParserGaps   int
	Crashes      int
	DeadTriggers int
	Invariants   int
}

// Stats returns cumulative counters without reading any files from disk.
func (b *Batcher) Stats() BatcherStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return BatcherStats{
		ParserGaps:   b.totalGaps,
		Crashes:      b.totalCrashes,
		DeadTriggers: b.totalDeadTrigs,
		Invariants:   b.totalInvariants,
	}
}

// Close stops the background flusher and writes any remaining buffer to
// disk. Idempotent.
func (b *Batcher) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	close(b.stop)
	b.doneWG.Wait()
	return b.Flush()
}

// flushCrashes appends a slice of pending crash entries to crashes.json.
// Mirrors PersistCrashLogs but takes already-grouped pendingCrash entries
// so each entry can carry its own commanderNames/nGames/nSeats.
func flushCrashes(dir string, crashes []pendingCrash) error {
	if len(crashes) == 0 {
		return nil
	}
	for _, c := range crashes {
		if err := PersistCrashLogs(dir, []string{c.StackTrace}, c.CommanderNames, c.NGames, c.NSeats); err != nil {
			return err
		}
	}
	return nil
}

// flushDeadTriggers merges accumulated dead-trigger counts into the
// existing dead_triggers.json. Mirrors the legacy adapter merge logic
// but operates on the accumulated batch.
func flushDeadTriggers(dir string, batch map[deadTrigKey]*deadTrigAccum) error {
	existing, err := ReadDeadTriggers(dir)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	idx := make(map[deadTrigKey]int, len(existing))
	for i, dt := range existing {
		idx[deadTrigKey{dt.TriggerName, dt.CardName}] = i
	}
	for k, a := range batch {
		if i, ok := idx[k]; ok {
			existing[i].Count += a.count
			existing[i].GamesSeen += a.gamesSeen
			existing[i].LastSeen = now
		} else {
			existing = append(existing, DeadTrigger{
				TriggerName: k.triggerName,
				CardName:    k.cardName,
				Count:       a.count,
				GamesSeen:   a.gamesSeen,
				LastSeen:    now,
			})
			idx[k] = len(existing) - 1
		}
	}
	return PersistDeadTriggersRaw(dir, existing)
}
