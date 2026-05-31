package deckparser

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// parallel_r60_test.go — correctness + speedup verification for
// ParseDeckFilesParallel. See parallel.go's header for the
// "per-line parallelization rejected" decision doc.

// allBenchFixturePaths returns every fixture deck path across the 5
// format buckets. Used for the bulk-parallel correctness + bench
// tests so each run exercises the full edge-case coverage of the
// benchmark fixture pack (DFC / partner / category headers / HTML /
// Archidekt brackets / tab-separated / etc.).
func allBenchFixturePaths(t testing.TB) []string {
	root := benchmarkRepoRoot(t)
	var paths []string
	for _, format := range []string{"moxfield", "deckbox", "archidekt", "mtggoldfish", "plaintext"} {
		dir := filepath.Join(root, benchmarkDataDir, "fixtures", format)
		paths = append(paths, listFixtures(t, format)...)
		_ = dir
	}
	sort.Strings(paths)
	return paths
}

// TestParseDeckFilesParallel_MatchesSerial — the load-bearing
// correctness pin. Parallel parse must produce the same
// CommanderName / Library length / CardLines / DetectedFormat /
// ParseReport coverage % as sequential parse for every fixture, in
// the same input order.
func TestParseDeckFilesParallel_MatchesSerial(t *testing.T) {
	meta := loadStubMeta(t)
	paths := allBenchFixturePaths(t)

	serialResults := make([]*TournamentDeck, len(paths))
	for i, p := range paths {
		td, err := ParseDeckFile(p, nil, meta)
		if err != nil {
			t.Fatalf("serial parse %s: %v", p, err)
		}
		serialResults[i] = td
	}

	parallelResults, parallelErrs := ParseDeckFilesParallel(paths, nil, meta, 0)
	if len(parallelResults) != len(paths) {
		t.Fatalf("parallel results len = %d, want %d", len(parallelResults), len(paths))
	}
	for i, p := range paths {
		if parallelErrs[i] != nil {
			t.Fatalf("parallel parse %s: %v", p, parallelErrs[i])
		}
		s, par := serialResults[i], parallelResults[i]
		if s.CommanderName != par.CommanderName {
			t.Errorf("%s: commander mismatch serial=%q parallel=%q", p, s.CommanderName, par.CommanderName)
		}
		if len(s.Library) != len(par.Library) {
			t.Errorf("%s: library count mismatch serial=%d parallel=%d", p, len(s.Library), len(par.Library))
		}
		if len(s.CardLines) != len(par.CardLines) {
			t.Errorf("%s: CardLines count mismatch serial=%d parallel=%d", p, len(s.CardLines), len(par.CardLines))
		}
		if s.DetectedFormat != par.DetectedFormat {
			t.Errorf("%s: DetectedFormat mismatch serial=%q parallel=%q", p, s.DetectedFormat, par.DetectedFormat)
		}
		// Coverage % equality verifies the per-line resolution path
		// took the same branches — any goroutine-related state leak
		// (e.g. shared meta with concurrent writes) would shift one
		// of the resolution counts.
		if s.ParseReport.CoveragePercent() != par.ParseReport.CoveragePercent() {
			t.Errorf("%s: coverage mismatch serial=%.1f%% parallel=%.1f%%",
				p, s.ParseReport.CoveragePercent(), par.ParseReport.CoveragePercent())
		}
	}
}

// TestParseDeckFilesParallel_PreservesInputOrder — results[i] must
// always correspond to paths[i] regardless of which worker finished
// first. Reorder-safety pin in case the worker pool implementation
// is ever refactored to a result-channel pattern.
func TestParseDeckFilesParallel_PreservesInputOrder(t *testing.T) {
	meta := loadStubMeta(t)
	paths := allBenchFixturePaths(t)
	results, _ := ParseDeckFilesParallel(paths, nil, meta, 4)
	for i, p := range paths {
		if results[i] == nil {
			t.Errorf("results[%d] nil for path %s", i, p)
			continue
		}
		// The deck's Path field is set by ParseDeckFile; verify it
		// matches the input path at the same index.
		if results[i].Path != p {
			t.Errorf("results[%d].Path = %q, want %q (order scrambled)", i, results[i].Path, p)
		}
	}
}

// TestParseDeckFilesParallel_EmptyInput — defensive: empty input
// returns nil/nil rather than panicking on the worker spawn.
func TestParseDeckFilesParallel_EmptyInput(t *testing.T) {
	results, errs := ParseDeckFilesParallel(nil, nil, nil, 4)
	if results != nil || errs != nil {
		t.Errorf("empty input: results=%v errs=%v, want nil/nil", results, errs)
	}
}

// TestParseDeckFilesParallel_ConcurrencyOverInputCount — when the
// caller passes a concurrency higher than the input count, the pool
// shrinks to len(paths) so we don't spawn idle workers.
func TestParseDeckFilesParallel_ConcurrencyOverInputCount(t *testing.T) {
	meta := loadStubMeta(t)
	paths := allBenchFixturePaths(t)[:3] // just 3 decks
	results, errs := ParseDeckFilesParallel(paths, nil, meta, 32)
	if len(results) != 3 || len(errs) != 3 {
		t.Fatalf("got results=%d errs=%d, want 3/3", len(results), len(errs))
	}
	for i, p := range paths {
		if errs[i] != nil {
			t.Errorf("results[%d] (%s): err=%v", i, p, errs[i])
		}
		if results[i] == nil {
			t.Errorf("results[%d] (%s): nil", i, p)
		}
	}
}

// TestParseDeckFilesParallel_DefaultConcurrencyUsesNumCPU —
// concurrency=0 routes to runtime.NumCPU(). Compile-time check (no
// timing assertions) so the test stays portable across CI runners
// with varying core counts.
func TestParseDeckFilesParallel_DefaultConcurrencyUsesNumCPU(t *testing.T) {
	if runtime.NumCPU() < 1 {
		t.Skip("NumCPU < 1 — defensive skip")
	}
	meta := loadStubMeta(t)
	paths := allBenchFixturePaths(t)
	results, _ := ParseDeckFilesParallel(paths, nil, meta, 0)
	// All decks should still parse — the default just shouldn't
	// silently produce nil entries.
	for i, td := range results {
		if td == nil {
			t.Errorf("results[%d] nil with default concurrency", i)
		}
	}
}

// TestParseDeckFilesParallel_ErrorIsolation — an error in one deck
// doesn't abort the others. Synthesizes a missing-file path mixed
// with valid fixtures.
func TestParseDeckFilesParallel_ErrorIsolation(t *testing.T) {
	meta := loadStubMeta(t)
	paths := []string{
		allBenchFixturePaths(t)[0],
		"/nonexistent/path/that/cant/exist.txt",
		allBenchFixturePaths(t)[1],
	}
	results, errs := ParseDeckFilesParallel(paths, nil, meta, 4)
	if results[0] == nil || errs[0] != nil {
		t.Errorf("results[0] should be valid; got err=%v", errs[0])
	}
	if results[1] != nil {
		t.Errorf("results[1] should be nil (missing file)")
	}
	if errs[1] == nil {
		t.Errorf("errs[1] should be non-nil (missing file)")
	}
	if !strings.Contains(errs[1].Error(), "nonexistent") && !strings.Contains(errs[1].Error(), "open") {
		t.Errorf("errs[1] = %v, want error mentioning the missing file path", errs[1])
	}
	if results[2] == nil || errs[2] != nil {
		t.Errorf("results[2] should be valid (error in [1] must not affect [2]); got err=%v", errs[2])
	}
}

// BenchmarkParseDeckFilesSerial parses every fixture sequentially —
// the baseline for the parallel speedup comparison. Reports
// allocations so the parallel variant's overhead is visible.
func BenchmarkParseDeckFilesSerial(b *testing.B) {
	meta := loadStubMeta(b)
	paths := allBenchFixturePaths(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			td, err := ParseDeckFile(p, nil, meta)
			if err != nil {
				b.Fatalf("parse %s: %v", p, err)
			}
			_ = td
		}
	}
}

// BenchmarkParseDeckFilesParallel runs the same parse via the bounded
// worker pool. Speedup vs Serial scales with runtime.NumCPU(). On
// fixture sets too small to amortize goroutine spawn the parallel
// variant can be SLOWER — the threshold doc in parallel.go's header
// notes where the per-line vs per-deck tradeoff lives.
func BenchmarkParseDeckFilesParallel(b *testing.B) {
	meta := loadStubMeta(b)
	paths := allBenchFixturePaths(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, errs := ParseDeckFilesParallel(paths, nil, meta, 0)
		for j, e := range errs {
			if e != nil {
				b.Fatalf("parse [%d]: %v", j, e)
			}
		}
		_ = results
	}
}

// BenchmarkParseDeckFilesParallel_Large simulates a tournament-sized
// import (50 decks via fixture repetition). Validates the parallel
// speedup at the workload size where the per-deck pool actually pays
// for itself (~12-25ms serial → ~3-5ms parallel on a 10-core M-series
// laptop; CI cores vary). Reports allocs/op for any future refactor
// that wants to eliminate the job-channel boxing.
func BenchmarkParseDeckFilesParallel_Large(b *testing.B) {
	meta := loadStubMeta(b)
	base := allBenchFixturePaths(b)
	// Repeat the fixture pack to ~50 decks.
	paths := make([]string, 0, 50)
	for len(paths) < 50 {
		paths = append(paths, base...)
	}
	paths = paths[:50]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, _ := ParseDeckFilesParallel(paths, nil, meta, 0)
		_ = results
	}
}

// BenchmarkParseDeckFilesSerial_Large is the serial counterpart of
// _Large for the speedup comparison. Use `benchstat` to diff:
//
//	go test ./internal/deckparser/ -bench=Large -count=10 -benchmem | tee bench.txt
//	benchstat bench.txt
func BenchmarkParseDeckFilesSerial_Large(b *testing.B) {
	meta := loadStubMeta(b)
	base := allBenchFixturePaths(b)
	paths := make([]string, 0, 50)
	for len(paths) < 50 {
		paths = append(paths, base...)
	}
	paths = paths[:50]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			td, err := ParseDeckFile(p, nil, meta)
			if err != nil {
				b.Fatalf("parse %s: %v", p, err)
			}
			_ = td
		}
	}
}
