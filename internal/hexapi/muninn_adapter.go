package hexapi

import (
	"github.com/hexdek/hexdek/internal/heimdall"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/muninn"
)

// muninnAdapter implements heimdall.MuninnSink by forwarding observations
// into a muninn.Batcher. The Batcher accumulates writes in memory and
// flushes them periodically (or on Close), eliminating the per-game
// read-modify-write cost of the raw Persist* helpers.
type muninnAdapter struct {
	dataDir string
	batcher *muninn.Batcher
}

var _ heimdall.MuninnSink = (*muninnAdapter)(nil)

func newMuninnAdapter(dataDir string) *muninnAdapter {
	return &muninnAdapter{
		dataDir: dataDir,
		batcher: muninn.NewBatcher(muninn.BatcherConfig{Dir: dataDir}),
	}
}

// r170 — every method on this adapter is nil-safe in BOTH directions:
// a nil *muninnAdapter and a non-nil adapter with a nil batcher.
//
// Three of the six already guarded the batcher; none guarded the
// receiver, and RecordParserGaps/RecordDeadTriggers/RecordCrash guarded
// neither. Found by driving the real grinder loop in the r170 seed
// work: sm.muninnSink is a typed nil in any Showmatch built without
// full init, so EndGame panicked at the batcher check itself.
//
// RecordCrash matters most: it is called from the grinder's recover(),
// where a panic is not recoverable a second time.
func (a *muninnAdapter) Close() error {
	// r170 — nil-safe receiver AND nil batcher. See the family note
	// at the top of this file.
	if a == nil || a.batcher == nil {
		return nil
	}
	return a.batcher.Close()
}

func (a *muninnAdapter) RecordParserGaps(gaps []string, gameID string) {
	// r170 — nil-safe receiver AND nil batcher. See the family note
	// at the top of this file.
	if a == nil || a.batcher == nil {
		return
	}
	if len(gaps) == 0 {
		return
	}
	counts := make(map[string]int, len(gaps))
	for _, g := range gaps {
		counts[g]++
	}
	a.batcher.AddParserGaps(counts)
}

func (a *muninnAdapter) RecordDeadTriggers(triggers []heimdall.DeadTrigger, gameID string) {
	// r170 — nil-safe receiver AND nil batcher. See the family note
	// at the top of this file.
	if a == nil || a.batcher == nil {
		return
	}
	if len(triggers) == 0 {
		return
	}
	for _, t := range triggers {
		a.batcher.AddDeadTrigger(t.TriggerType, t.CardName, 1, 1)
	}
}

func (a *muninnAdapter) RecordCrash(panicMsg string, stackTrace string, deckKeys []string) {
	// r170 — nil-safe receiver AND nil batcher. See the family note
	// at the top of this file.
	// This one runs INSIDE the grinder's recover(), so a panic here
	// turns a recovered crash into a fatal one that takes the worker
	// with it. It had no guard at all.
	if a == nil || a.batcher == nil {
		return
	}
	a.batcher.AddCrash(stackTrace, deckKeys, 0, 0)
}

// AutoArchive buffers Feynman/Odin invariant violations from one game.
// Replaces direct muninn.AutoArchiveViolation calls so the per-game
// invariant check doesn't read-modify-write invariant_violations.json.
func (a *muninnAdapter) AutoArchive(rngSeed int64, deckKeys [4]string, violations []judge.ValidationViolation) {
	// r170 — nil-safe receiver AND nil batcher. See the family note
	// at the top of this file.
	if a == nil || a.batcher == nil {
		return
	}
	a.batcher.AddAutoArchive(rngSeed, deckKeys, violations)
}

// EndGame ticks the batcher's per-game counter. Drives the "100 games"
// half of the flush guarantee — must be called once per completed game.
func (a *muninnAdapter) EndGame() {
	// r170 — nil-safe receiver AND nil batcher. See the family note
	// at the top of this file.
	if a == nil || a.batcher == nil {
		return
	}
	a.batcher.EndGame()
}
