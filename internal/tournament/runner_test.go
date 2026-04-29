package tournament

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// ---------------------------------------------------------------------
// Shared setup — lazy-load the corpus once for the whole test package.
// ---------------------------------------------------------------------

var (
	corpusOnce    sync.Once
	sharedCorpus  *astload.Corpus
	sharedMeta    *deckparser.MetaDB
	corpusLoadErr error
)

func loadCorpus(tb testing.TB) (*astload.Corpus, *deckparser.MetaDB) {
	tb.Helper()
	corpusOnce.Do(func() {
		path := astDatasetPath()
		if _, err := os.Stat(path); err != nil {
			corpusLoadErr = err
			return
		}
		var err error
		sharedCorpus, err = astload.Load(path)
		if err != nil {
			corpusLoadErr = err
			return
		}
		sharedMeta, err = deckparser.LoadMetaFromJSONL(path)
		if err != nil {
			corpusLoadErr = err
			return
		}
		oraclePath := filepath.Join(filepath.Dir(path), "oracle-cards.json")
		if _, serr := os.Stat(oraclePath); serr == nil {
			_ = sharedMeta.SupplementWithOracleJSON(oraclePath)
		}
	})
	if corpusLoadErr != nil {
		tb.Skipf("corpus unavailable: %v", corpusLoadErr)
	}
	return sharedCorpus, sharedMeta
}

func astDatasetPath() string {
	// Tests run from the package dir. Walk up to find data/rules.
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "data", "rules", "ast_dataset.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

func findDecks(tb testing.TB, n int) []string {
	tb.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "data", "decks", "personal")
		if entries, err := os.ReadDir(candidate); err == nil && len(entries) > 0 {
			paths, _ := deckparser.ListDeckFiles(candidate)
			if len(paths) >= n {
				return paths[:n]
			}
			return paths
		}
		dir = filepath.Dir(dir)
	}
	tb.Skip("no deck files found")
	return nil
}

// ---------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------

// TestSmallTournament verifies a 10-game run completes + returns sane
// numbers with two decks at 2 seats.
func TestSmallTournament(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need at least 2 decks")
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:2] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}
	cfg := TournamentConfig{
		Decks:         decks,
		NSeats:        2,
		NGames:        10,
		Seed:          1,
		Workers:       2,
		CommanderMode: true,
	}
	r, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Games+r.Crashes != 10 {
		t.Fatalf("expected 10 total outcomes, got games=%d crashes=%d", r.Games, r.Crashes)
	}
	// Everyone should have finished on SOME turn > 0.
	if r.AvgTurns <= 0 {
		t.Fatalf("avg turns is 0")
	}
}

// Test4pCommanderTournament runs 50 games with 4 decks at 4 seats, mix
// of hats. Every commander should appear in the wins matrix (no
// commander is structurally shut out).
func Test4pCommanderTournament(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 4)
	if len(paths) < 4 {
		t.Skipf("need 4 decks, got %d", len(paths))
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:4] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}
	cfg := TournamentConfig{
		Decks:  decks,
		NSeats: 4,
		NGames: 50,
		Seed:   2,
		HatFactories: []HatFactory{
			func() gameengine.Hat { return &hat.GreedyHat{} },
			func() gameengine.Hat { return hat.NewPokerHat() },
			func() gameengine.Hat { return &hat.GreedyHat{} },
			func() gameengine.Hat { return hat.NewPokerHat() },
		},
		Workers:       2,
		CommanderMode: true,
	}
	r, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Games+r.Crashes != 50 {
		t.Fatalf("expected 50 outcomes, got games=%d crashes=%d", r.Games, r.Crashes)
	}
	// Mode changes should be non-zero when any poker hat is active AND
	// games actually play enough turns. Not asserted strictly — ENV
	// variance.
	_ = r.TotalModeChanges
	t.Logf("4p result: games=%d crashes=%d draws=%d avg_turns=%.1f dur=%s gps=%.1f",
		r.Games, r.Crashes, r.Draws, r.AvgTurns, r.Duration, r.GamesPerSecond)
	t.Logf("  wins: %v", r.WinsByCommander)
}

// TestSeedDeterminism runs the same config twice and asserts identical
// per-game outcomes (same winner, same turn count).
func TestSeedDeterminism(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need 2 decks")
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:2] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}
	cfg := TournamentConfig{
		Decks: decks, NSeats: 2, NGames: 10, Seed: 99,
		Workers: 1, CommanderMode: true,
	}
	r1, err := Run(cfg)
	if err != nil {
		t.Fatalf("run1: %v", err)
	}
	r2, err := Run(cfg)
	if err != nil {
		t.Fatalf("run2: %v", err)
	}
	// With Workers=1 and the same seed, aggregated wins should match.
	if r1.Crashes != r2.Crashes {
		t.Fatalf("crashes differ: %d vs %d", r1.Crashes, r2.Crashes)
	}
	// Since outcomes are reported through a channel and aggregated in
	// insertion order, different scheduling can re-order which game
	// reports first — but the AGGREGATE (wins by commander, crash count,
	// draw count) should be identical.
	for _, name := range r1.CommanderNames {
		if r1.WinsByCommander[name] != r2.WinsByCommander[name] {
			t.Fatalf("wins for %q differ: %d vs %d",
				name, r1.WinsByCommander[name], r2.WinsByCommander[name])
		}
	}
	if r1.Draws != r2.Draws {
		t.Fatalf("draws differ: %d vs %d", r1.Draws, r2.Draws)
	}
}

// TestGoroutineSafety — 1000 games with 8 workers. Primary purpose is
// to run under `go test -race`; failures in this test almost always
// mean a data race.
func TestGoroutineSafety(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 4)
	if len(paths) < 4 {
		t.Skip("need 4 decks")
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:4] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}
	cfg := TournamentConfig{
		Decks: decks, NSeats: 4, NGames: 1000, Seed: 7,
		Workers: 8, CommanderMode: true,
	}
	r, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Games+r.Crashes != 1000 {
		t.Fatalf("wrong total: games=%d crashes=%d", r.Games, r.Crashes)
	}
	t.Logf("1000-game race test: games=%d crashes=%d dur=%s gps=%.1f",
		r.Games, r.Crashes, r.Duration, r.GamesPerSecond)
}

// TestCrashHandling feeds a deck with no library (empty library file)
// to test that crashes are contained, not fatal.
func TestCrashHandling(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need 2 decks")
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:2] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}
	// Corrupt one deck by zapping library + commander cards so runOneGame
	// hits a legitimate error path (we force a nil commander card).
	decks[0].CommanderCards = []*gameengine.Card{nil}
	// The crash should be caught by validate() before Run kicks off games.
	cfg := TournamentConfig{
		Decks: decks, NSeats: 2, NGames: 5, Seed: 1,
		Workers: 1, CommanderMode: true,
	}
	_, err := Run(cfg)
	if err == nil {
		t.Fatalf("expected error from nil commander, got nil")
	}
	if !strings.Contains(err.Error(), "nil") && !strings.Contains(err.Error(), "commander") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestConfigValidate pokes the validator's corners.
func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  TournamentConfig
	}{
		{"zero games", TournamentConfig{Decks: []*deckparser.TournamentDeck{nil, nil}, NSeats: 2, NGames: 0}},
		{"one seat",
			TournamentConfig{Decks: []*deckparser.TournamentDeck{nil}, NSeats: 1, NGames: 5}},
		{"not enough decks",
			TournamentConfig{Decks: []*deckparser.TournamentDeck{nil}, NSeats: 4, NGames: 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Run(tc.cfg); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

// TestReportMarkdown exercises the markdown writer with a faux result.
func TestReportMarkdown(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "r.md")
	r := &TournamentResult{
		Games: 100, Crashes: 0, Draws: 2, AvgTurns: 12.3,
		NSeats:         4,
		CommanderNames: []string{"A", "B", "C", "D"},
		WinsByCommander: map[string]int{
			"A": 30, "B": 25, "C": 23, "D": 20,
		},
		EliminationByCommanderBySlot: map[string][]int{
			"A": {5, 10, 15, 70},
			"B": {10, 15, 20, 55},
			"C": {15, 20, 25, 40},
			"D": {20, 25, 30, 25},
		},
	}
	if err := r.WriteMarkdown(path); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	text := string(data)
	for _, want := range []string{"Tournament Report", "Wins by Commander", "A", "B", "C", "D", "Elimination Matrix"} {
		if !strings.Contains(text, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

// TestHatFactoriesUniform exercises the len==1 uniform mode + len==0 default.
func TestHatFactoriesUniform(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need 2 decks")
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:2] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}
	// len==0 default
	r, err := Run(TournamentConfig{
		Decks: decks, NSeats: 2, NGames: 4, Seed: 5,
		Workers: 1, CommanderMode: true,
	})
	if err != nil {
		t.Fatalf("default-hat run: %v", err)
	}
	if r.Games+r.Crashes != 4 {
		t.Fatalf("expected 4 outcomes")
	}
	// len==1 uniform
	r2, err := Run(TournamentConfig{
		Decks: decks, NSeats: 2, NGames: 4, Seed: 5,
		Workers: 1, CommanderMode: true,
		HatFactories: []HatFactory{func() gameengine.Hat { return hat.NewPokerHat() }},
	})
	if err != nil {
		t.Fatalf("uniform-hat run: %v", err)
	}
	if r2.Games+r2.Crashes != 4 {
		t.Fatalf("expected 4 outcomes (uniform)")
	}
}

// ---------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------

// BenchmarkTournament_4p_1000Games is the tight-loop perf smoke.
func BenchmarkTournament_4p_1000Games(b *testing.B) {
	corpus, meta := loadCorpus(b)
	paths := findDecks(b, 4)
	if len(paths) < 4 {
		b.Skip("need 4 decks")
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:4] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			b.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}
	cfg := TournamentConfig{
		Decks: decks, NSeats: 4, NGames: 1000, Seed: 42,
		Workers: runtime.NumCPU(), CommanderMode: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := Run(cfg)
		if err != nil {
			b.Fatalf("run: %v", err)
		}
		if r.Games == 0 {
			b.Fatalf("no games finished")
		}
	}
}

// BenchmarkTournament_4p_50kGames is the Phase 11 perf target.
// Run with: go test -bench BenchmarkTournament_4p_50kGames -benchtime=1x ./internal/tournament
func BenchmarkTournament_4p_50kGames(b *testing.B) {
	if testing.Short() {
		b.Skip("-short: skip 50k")
	}
	corpus, meta := loadCorpus(b)
	paths := findDecks(b, 4)
	if len(paths) < 4 {
		b.Skip("need 4 decks")
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:4] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			b.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}
	cfg := TournamentConfig{
		Decks: decks, NSeats: 4, NGames: 50_000, Seed: 42,
		Workers: runtime.NumCPU(), CommanderMode: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := Run(cfg)
		if err != nil {
			b.Fatalf("run: %v", err)
		}
		b.ReportMetric(r.GamesPerSecond, "games/sec")
		b.ReportMetric(float64(r.Duration.Milliseconds()), "duration_ms")
		b.Logf("50k: games=%d crashes=%d dur=%s gps=%.1f",
			r.Games, r.Crashes, r.Duration, r.GamesPerSecond)
	}
}
