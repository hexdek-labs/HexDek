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

func (a *muninnAdapter) Close() error {
	if a.batcher == nil {
		return nil
	}
	return a.batcher.Close()
}

func (a *muninnAdapter) RecordParserGaps(gaps []string, gameID string) {
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
	if len(triggers) == 0 {
		return
	}
	for _, t := range triggers {
		a.batcher.AddDeadTrigger(t.TriggerType, t.CardName, 1, 1)
	}
}

func (a *muninnAdapter) RecordCrash(panicMsg string, stackTrace string, deckKeys []string) {
	a.batcher.AddCrash(stackTrace, deckKeys, 0, 0)
}

// AutoArchive buffers Feynman/Odin invariant violations from one game.
// Replaces direct muninn.AutoArchiveViolation calls so the per-game
// invariant check doesn't read-modify-write invariant_violations.json.
func (a *muninnAdapter) AutoArchive(rngSeed int64, deckKeys [4]string, violations []judge.ValidationViolation) {
	if a.batcher == nil {
		return
	}
	a.batcher.AddAutoArchive(rngSeed, deckKeys, violations)
}

// EndGame ticks the batcher's per-game counter. Drives the "100 games"
// half of the flush guarantee — must be called once per completed game.
func (a *muninnAdapter) EndGame() {
	if a.batcher == nil {
		return
	}
	a.batcher.EndGame()
}
