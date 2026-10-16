package hexapi

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/heimdall"
	"github.com/hexdek/hexdek/internal/huginn"
)

const (
	// huginnObsFlushEvery is how many buffered observations the adapter
	// accumulates before it touches disk.
	//
	// r151 — this used to be 1: EVERY game read the entire
	// raw_observations.json back, appended to it and wrote the whole thing
	// out again. On .206 that was a 1.09MB os.ReadFile per game (332MB,
	// 4.61% of a differenced allocation profile, ~305 reads across 325
	// games) for a file that only ever grows, so the per-game cost rose
	// with the length of the run. Buffering amortises the read-modify-write
	// over a batch. The seed sink one function away in heimdall's Observer
	// has used exactly this shape (seedBufSize = 1000) since it was written.
	huginnObsFlushEvery = 500

	// huginnObsCap bounds the on-disk history so the read-modify-write can
	// never grow without limit again. Oldest observations are dropped.
	huginnObsCap = 20000
)

// huginnAdapter implements heimdall.HuginnSink by converting Heimdall's
// lightweight CoTriggerPair observations into Huginn's RawObservation
// format and appending them to the raw_observations.json file for later
// ingestion.
type huginnAdapter struct {
	dataDir string

	mu  sync.Mutex
	buf []huginn.RawObservation
}

var _ heimdall.HuginnSink = (*huginnAdapter)(nil)

// IngestCoTriggers converts Heimdall CoTriggerPairs into Huginn's
// RawObservation format and persists them. The EffectPattern is
// synthesized from the card pair since Heimdall doesn't carry the
// full effect string; Huginn's NormalizePattern handles it gracefully.
func (a *huginnAdapter) IngestCoTriggers(pairs []heimdall.CoTriggerPair, deckNames []string) {
	if len(pairs) == 0 {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	names := append([]string(nil), deckNames...)

	a.mu.Lock()
	for _, p := range pairs {
		// Synthesize an effect pattern from the pair. The format matches
		// what analytics.DetectCoTriggers produces so NormalizePattern
		// can extract the resource flow ("produces X → consumes X").
		// When no resource flow is available, use the card pair directly
		// so Huginn still groups observations by the same card combo.
		effectPattern := fmt.Sprintf("%s produces synergy, %s consumes synergy", p.CardA, p.CardB)

		a.buf = append(a.buf, huginn.RawObservation{
			CardA:         p.CardA,
			CardB:         p.CardB,
			ImpactScore:   p.ImpactScore,
			TurnWindow:    p.TurnWindow,
			EffectPattern: effectPattern,
			DeckNames:     names,
			Timestamp:     now,
		})
	}
	if len(a.buf) < huginnObsFlushEvery {
		a.mu.Unlock()
		return
	}
	pending := a.buf
	a.buf = nil
	a.mu.Unlock()

	a.flush(pending)
}

// flush performs the one read-modify-write for a whole batch.
func (a *huginnAdapter) flush(pending []huginn.RawObservation) {
	if len(pending) == 0 {
		return
	}
	dir := filepath.Join(a.dataDir, "huginn")

	existing, err := huginn.ReadRawObservations(dir)
	if err != nil {
		// r151 — this error used to be swallowed by a bare `return`. The
		// live file on .206 was corrupted by the concurrent-tmp race (since
		// fixed in huginn.atomicWriteJSON) on 2026-06-07 and stayed
		// unparseable for four months: every game paid the read and the
		// pair construction, every game threw the result away, and nothing
		// anywhere said so. Say so, and keep the batch rather than dropping
		// it on the floor — a corrupt history is not a reason to also lose
		// the observations we just made.
		log.Printf("huginn: raw observations unreadable (%v) — starting a fresh history, %d observations kept", err, len(pending))
		existing = nil
	}

	existing = append(existing, pending...)
	if len(existing) > huginnObsCap {
		existing = existing[len(existing)-huginnObsCap:]
	}
	if err := huginn.PersistRawObservationsRaw(dir, existing); err != nil {
		log.Printf("huginn: persist raw observations: %v", err)
	}
}
