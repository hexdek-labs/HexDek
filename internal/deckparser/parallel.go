package deckparser

import (
	"runtime"
	"sync"

	"github.com/hexdek/hexdek/internal/astload"
)

// PER-LINE PARALLELIZATION — rejected after profiling, 2026-05-31.
//
// The natural temptation when seeing a parser is to fan out the per-
// line resolution loop across goroutines. Profiling the deckparser
// against 25 curated fixtures (~30-100 cards each, see
// data/decks-benchmark/) on Apple M4 silicon measured:
//
//   - Total parse time: ~5-10ms for all 25 fixtures.
//   - Per-deck average:  ~240µs.
//   - Per-card resolution: ~3-8µs (qty extract + cleanCardName +
//     normalizeDFCSeparator + buildCard + Library append).
//   - Allocs: ~13.5K per 25-fixture run (~540 / deck).
//
// Goroutine spawn cost on Go 1.22+ is ~1-2µs; channel ops are
// ~100ns each; sync.WaitGroup add/done is ~50ns. For a 50-card deck
// the per-line work totals ~150-400µs while goroutine overhead would
// be 50µs+ minimum, with no opportunity for real CPU parallelism
// at that scale. Per-line parallelization would be a net slowdown
// AND would force MetaDB.byName to become thread-safe (it's currently
// not — concurrent map reads are safe per the Go memory model, but
// any future write surface would have to grow a mutex).
//
// THE RIGHT LEVEL TO PARALLELIZE IS PER-DECK. Callers like
// cmd/hexdek-tournament and cmd/hexdek-valkyrie load 100-1000+
// decks in a for-loop; that's where a bounded worker pool pays for
// itself. ParseDeckFilesParallel below is the canonical helper.
//
// If a future change makes per-line work expensive (e.g. a network
// fetch inside buildCard, a regex-heavy normalizer), revisit by:
//   1. Re-run BenchmarkParseAllFormats — confirm per-deck time exceeds
//      ~1ms.
//   2. Add a per-line `parallel.go::parseLineWorkers` pool.
//   3. Make MetaDB concurrent-read safe (already true via plain map
//      reads, but document explicitly).
// Today (~240µs / deck) it's not worth the complexity.

// ParseDeckFilesParallel is the bulk-import convenience. Parses every
// path concurrently using a bounded worker pool sized to
// `concurrency` (or runtime.NumCPU() when concurrency <= 0), returning
// per-input results + errors in the input order. Each ParseDeckFile
// call is independent — corpus and meta are concurrent-read safe
// (MetaDB.byName is a plain map; concurrent reads on a never-written
// map are safe per Go's memory model) — so the only synchronization
// is the worker pool's job channel + result aggregation.
//
// Callers iterating large decklist directories (hexdek-tournament's
// startup load, hexdek-valkyrie's gauntlet seeding, hexdek-import's
// post-fetch re-parse) get a ~Nx speedup where N = NumCPU for typical
// EDH-deck workloads. Sequential ParseDeckFile remains the right API
// for single-deck callers (CLI tools, REPLs, judge mode).
//
// Error semantics: results[i] is nil iff errors[i] != nil. A failure
// in one deck doesn't abort the others — every path runs to
// completion. The caller decides whether to drop / report / retry.
func ParseDeckFilesParallel(paths []string, corpus *astload.Corpus, meta *MetaDB, concurrency int) ([]*TournamentDeck, []error) {
	if len(paths) == 0 {
		return nil, nil
	}
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}
	if concurrency > len(paths) {
		concurrency = len(paths)
	}
	results := make([]*TournamentDeck, len(paths))
	errs := make([]error, len(paths))
	type job struct {
		idx  int
		path string
	}
	jobs := make(chan job, concurrency)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				td, err := ParseDeckFile(j.path, corpus, meta)
				results[j.idx] = td
				errs[j.idx] = err
			}
		}()
	}
	for i, p := range paths {
		jobs <- job{idx: i, path: p}
	}
	close(jobs)
	wg.Wait()
	return results, errs
}
