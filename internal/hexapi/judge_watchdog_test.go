package hexapi

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/tournament"
)

func TestJudgeWatchdog_OffByDefault(t *testing.T) {
	t.Setenv("HEXDEK_JUDGE_SAMPLE", "")
	if w := newJudgeWatchdogFromEnv(); w != nil {
		t.Fatalf("watchdog should be nil when HEXDEK_JUDGE_SAMPLE is unset, got %+v", w)
	}
	t.Setenv("HEXDEK_JUDGE_SAMPLE", "0")
	if w := newJudgeWatchdogFromEnv(); w != nil {
		t.Fatal("watchdog should be nil at rate 0")
	}
	t.Setenv("HEXDEK_JUDGE_SAMPLE", "garbage")
	if w := newJudgeWatchdogFromEnv(); w != nil {
		t.Fatal("watchdog should be nil on unparseable rate")
	}

	// The entire nil-safety chain a disabled server exercises.
	var w *judgeWatchdog
	jr := w.beginGame(rand.New(rand.NewSource(1)), 42, nil)
	if jr != nil {
		t.Fatal("nil watchdog must yield nil game run")
	}
	jr.Attach(nil)
	jr.AfterTurn(nil)
	jr.Finish(nil, nil, nil) // must not panic
}

func TestJudgeWatchdog_EnvParsing(t *testing.T) {
	t.Setenv("HEXDEK_JUDGE_SAMPLE", "0.05")
	t.Setenv("HEXDEK_JUDGE_SAMPLE_CORPUS", "")
	t.Setenv("HEXDEK_JUDGE_LOG", "")
	w := newJudgeWatchdogFromEnv()
	if w == nil {
		t.Fatal("expected watchdog on")
	}
	if w.gameRate != 0.05 {
		t.Errorf("gameRate = %v, want 0.05", w.gameRate)
	}
	if w.corpusRate != 0.005 {
		t.Errorf("corpusRate = %v, want 0.005 (SAMPLE/10 default)", w.corpusRate)
	}
	if w.logPath != defaultJudgeLogPath {
		t.Errorf("logPath = %q, want default", w.logPath)
	}

	// Corpus rate is clamped to the game rate (corpus games are a
	// subset of sampled games).
	t.Setenv("HEXDEK_JUDGE_SAMPLE_CORPUS", "0.5")
	w = newJudgeWatchdogFromEnv()
	if w.corpusRate != 0.05 {
		t.Errorf("corpusRate = %v, want clamp to gameRate 0.05", w.corpusRate)
	}

	// Rates clamp to [0,1].
	t.Setenv("HEXDEK_JUDGE_SAMPLE", "7")
	w = newJudgeWatchdogFromEnv()
	if w.gameRate != 1 {
		t.Errorf("gameRate = %v, want clamp to 1", w.gameRate)
	}
}

func TestJudgeWatchdog_SamplingRates(t *testing.T) {
	w := &judgeWatchdog{gameRate: 1.0, corpusRate: 1.0, logPath: filepath.Join(t.TempDir(), "v.jsonl")}
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 20; i++ {
		jr := w.beginGame(rng, int64(i), []string{"a", "b"})
		if jr == nil {
			t.Fatal("rate 1.0 must sample every game")
		}
		if !jr.corpus {
			t.Fatal("corpusRate == gameRate must corpus-sample every sampled game")
		}
		if jr.legality == nil {
			t.Fatal("sampled game must carry a legality validator")
		}
	}

	w = &judgeWatchdog{gameRate: 0.5, corpusRate: 0.0, logPath: filepath.Join(t.TempDir(), "v.jsonl")}
	sampled := 0
	for i := 0; i < 2000; i++ {
		if jr := w.beginGame(rng, int64(i), nil); jr != nil {
			sampled++
			if jr.corpus {
				t.Fatal("corpusRate 0 must never corpus-sample")
			}
		}
	}
	if sampled < 850 || sampled > 1150 {
		t.Errorf("rate 0.5 sampled %d/2000 — outside sanity band", sampled)
	}
}

// readTriageLog parses every JSONL record in the watchdog's log.
func readTriageLog(t *testing.T, path string) []judgeViolationRecord {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read triage log: %v", err)
	}
	var out []judgeViolationRecord
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var rec judgeViolationRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("bad JSONL line %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

func TestJudgeWatchdog_AfterTurnWritesInvariantViolations(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "violations.jsonl")
	w := &judgeWatchdog{gameRate: 1.0, logPath: logPath}
	rng := rand.New(rand.NewSource(1))
	jr := w.beginGame(rng, 12345, []string{"deckA", "deckB"})

	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(2)), nil)
	gs.Turn = 7
	// LifeConsistency violation: negative life on a seat that has not
	// lost (no Platinum-Angel-class shield registered).
	gs.Seats[0].Life = -5
	gs.Seats[0].Lost = false

	jr.AfterTurn(gs)
	recs := readTriageLog(t, logPath)
	if len(recs) == 0 {
		t.Fatal("expected at least one violation record")
	}
	found := false
	for _, r := range recs {
		if r.Rule == "LifeConsistency" {
			found = true
			if r.GameSeed != 12345 {
				t.Errorf("seed = %d, want 12345", r.GameSeed)
			}
			if r.Dimension != judge.DimensionStateIntegrity {
				t.Errorf("dimension = %q, want state_integrity (untagged default)", r.Dimension)
			}
			if r.Turn != 7 {
				t.Errorf("turn = %d, want 7", r.Turn)
			}
			if len(r.DeckKeys) != 2 || r.DeckKeys[0] != "deckA" {
				t.Errorf("deck keys = %v", r.DeckKeys)
			}
		}
	}
	if !found {
		t.Fatalf("no LifeConsistency record in %+v", recs)
	}

	// Per-game dedupe: the same persistent violation must not re-log
	// on the next turn.
	before := len(recs)
	gs.Turn = 8
	jr.AfterTurn(gs)
	if after := len(readTriageLog(t, logPath)); after != before {
		t.Errorf("persistent violation re-logged: %d -> %d records", before, after)
	}
}

func TestJudgeWatchdog_FinishDrainsLegalityAndFeynman(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "violations.jsonl")
	w := &judgeWatchdog{gameRate: 1.0, logPath: logPath}
	jr := w.beginGame(rand.New(rand.NewSource(1)), 99, []string{"d1"})

	jr.legality.Violations = append(jr.legality.Violations, gameengine.LegalityViolation{
		Seed: 99, Turn: 3, Seat: 2, Action: "cast:Lightning Bolt", Rule: "307.1", Detail: "sorcery-speed cast at instant timing",
	})
	feynman := []judge.ValidationViolation{{
		Surface: judge.SurfaceFeynman, Name: "704.5a", Severity: judge.SeverityCritical,
		Dimension: judge.DimensionStateIntegrity, Seat: 1, Message: "seat 1 has -3 life but is not marked lost",
	}}
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(2)), nil)
	gs.Turn = 11
	jr.Finish(gs, feynman, nil)

	recs := readTriageLog(t, logPath)
	var sawLegality, sawFeynman bool
	for _, r := range recs {
		if r.Dimension == judge.DimensionLegality && r.Rule == "307.1" && r.Turn == 3 && r.Seat == 2 {
			sawLegality = true
		}
		if r.Surface == judge.SurfaceFeynman && r.Rule == "704.5a" && r.Turn == 11 {
			sawFeynman = true
		}
	}
	if !sawLegality {
		t.Errorf("legality violation not drained: %+v", recs)
	}
	if !sawFeynman {
		t.Errorf("feynman violation not recorded: %+v", recs)
	}
	if w.flaggedGames.Load() != 1 {
		t.Errorf("flaggedGames = %d, want 1", w.flaggedGames.Load())
	}
}

func TestJudgeWatchdog_CorpusDedupe(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "violations.jsonl")
	w := &judgeWatchdog{gameRate: 1.0, corpusRate: 1.0, logPath: logPath}
	rng := rand.New(rand.NewSource(1))

	card := &gameengine.Card{
		Name:  "Watchdog Test Bear",
		Types: []string{"creature"},
		AST:   &gameast.CardAST{Name: "Watchdog Test Bear"},
	}
	deck := &deckparser.TournamentDeck{Library: []*gameengine.Card{card}}

	jr := w.beginGame(rng, 1, nil)
	jr.Finish(nil, nil, []*deckparser.TournamentDeck{deck})
	if _, ok := w.checkedCards.Load("Watchdog Test Bear"); !ok {
		t.Fatal("card not marked checked after corpus pass")
	}

	// Second sampled game with the same card: LoadOrStore must dedupe.
	// (Behavioral pin: re-checking would re-run the interpreters; we
	// assert via the dup return path leaving the map size stable.)
	count := 0
	w.checkedCards.Range(func(_, _ any) bool { count++; return true })
	jr2 := w.beginGame(rng, 2, nil)
	jr2.Finish(nil, nil, []*deckparser.TournamentDeck{deck})
	count2 := 0
	w.checkedCards.Range(func(_, _ any) bool { count2++; return true })
	if count2 != count {
		t.Errorf("checkedCards grew on duplicate deck: %d -> %d", count, count2)
	}
}

// ---------------------------------------------------------------------------
// Throughput harness — measures the per-game cost of each sampling
// tier on real-deck games. Not part of the normal test run: requires
// HEXDEK_JUDGE_PERF=1 plus the gitignored data files. Re-run with:
//
//	HEXDEK_JUDGE_PERF=1 go test ./internal/hexapi/ -run JudgeWatchdog_GrinderThroughput -v -count=1 -timeout 1200s
//
// Pilots are GreedyHat — far cheaper per turn than the production
// YggdrasilHat (MCTS budget 50) — so the measured overhead RATIO is a
// conservative upper bound on the production impact.
// ---------------------------------------------------------------------------

func TestJudgeWatchdog_GrinderThroughput(t *testing.T) {
	if os.Getenv("HEXDEK_JUDGE_PERF") != "1" {
		t.Skip("perf harness — set HEXDEK_JUDGE_PERF=1 to run")
	}
	astPath := os.Getenv("HEXDEK_PERF_AST")
	if astPath == "" {
		astPath = "../../data/rules/ast_dataset.jsonl"
	}
	decksDir := os.Getenv("HEXDEK_PERF_DECKS")
	if decksDir == "" {
		decksDir = "../../data/decks/moxfield"
	}
	if _, err := os.Stat(astPath); err != nil {
		t.Skipf("AST dataset missing: %v", err)
	}
	corpus, err := astload.Load(astPath)
	if err != nil {
		t.Fatalf("astload: %v", err)
	}
	meta, err := deckparser.LoadMetaFromJSONL(astPath)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	entries, err := os.ReadDir(decksDir)
	if err != nil {
		t.Skipf("decks dir missing: %v", err)
	}
	var decks []*deckparser.TournamentDeck
	for _, e := range entries {
		if len(decks) >= 8 || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		d, perr := deckparser.ParseDeckFile(filepath.Join(decksDir, e.Name()), corpus, meta)
		if perr == nil && d != nil && len(d.Library) > 60 && len(d.CommanderCards) > 0 {
			decks = append(decks, d)
		}
	}
	if len(decks) < 4 {
		t.Skipf("only %d usable decks in %s", len(decks), decksDir)
	}

	const gamesPerTier = 30
	runOne := func(g int, w *judgeWatchdog, forceCorpus bool) (time.Duration, int) {
		seed := int64(1000 + g)
		gameRng := rand.New(rand.NewSource(seed))
		gs := gameengine.NewGameState(4, gameRng, corpus)
		gs.Seed = seed
		gs.EventPolicy = gameengine.EventLogNone
		jr := w.beginGame(rand.New(rand.NewSource(seed)), seed, nil)
		if jr != nil && !forceCorpus {
			jr.corpus = false
		}
		jr.Attach(gs)
		picked := make([]*deckparser.TournamentDeck, 4)
		cmdDecks := make([]*gameengine.CommanderDeck, 4)
		for i := 0; i < 4; i++ {
			picked[i] = decks[(g+i)%len(decks)]
			lib := deckparser.CloneLibrary(picked[i].Library)
			cmdrs := deckparser.CloneCards(picked[i].CommanderCards)
			for _, c := range cmdrs {
				c.Owner = i
			}
			for _, c := range lib {
				c.Owner = i
			}
			gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
			cmdDecks[i] = &gameengine.CommanderDeck{CommanderCards: cmdrs, Library: lib}
		}
		start := time.Now()
		gameengine.SetupCommanderGame(gs, cmdDecks)
		for i := 0; i < 4; i++ {
			gs.Seats[i].Hat = &hat.GreedyHat{}
			tournament.RunLondonMulligan(gs, i)
		}
		gs.Active = gameRng.Intn(4)
		for turn := 1; turn <= 60; turn++ {
			gs.Turn = turn
			tournament.TakeTurn(gs)
			gameengine.StateBasedActions(gs)
			jr.AfterTurn(gs)
			if gs.CheckEnd() {
				break
			}
			gs.Active = nextLiving(gs)
		}
		oracle := hat.CheckGame(gs)
		if invs := gameengine.RunAllInvariants(gs); len(invs) > 0 {
			_ = invs // every tier pays the existing post-game pass
		}
		jr.Finish(gs, oracle.Violations, picked)
		return time.Since(start), gs.Turn
	}

	dir := t.TempDir()
	wGame := &judgeWatchdog{gameRate: 1.0, logPath: filepath.Join(dir, "g.jsonl")}
	wCorpus := &judgeWatchdog{gameRate: 1.0, corpusRate: 1.0, logPath: filepath.Join(dir, "c.jsonl")}

	// Warmup: equalize page cache / allocator state before timing.
	for g := 0; g < 4; g++ {
		_, _ = runOne(g, nil, false)
	}
	// Interleave the three tiers game-by-game (identical seeds per
	// tier) so drift in machine state spreads evenly across tiers
	// instead of penalizing whichever tier runs first.
	var baseline, gameTier, corpusTier time.Duration
	turns := 0
	for g := 0; g < gamesPerTier; g++ {
		d0, n := runOne(g, nil, false)
		d1, _ := runOne(g, wGame, false)
		d2, _ := runOne(g, wCorpus, true)
		baseline += d0
		gameTier += d1
		corpusTier += d2
		turns += n
	}
	t.Logf("%-28s %d games, %d turns, %.1f ms/game", "off (nil watchdog)", gamesPerTier, turns, float64(baseline.Milliseconds())/gamesPerTier)
	t.Logf("%-28s %d games, %.1f ms/game", "game-level dims (rate=1)", gamesPerTier, float64(gameTier.Milliseconds())/gamesPerTier)
	t.Logf("%-28s %d games, %.1f ms/game", "game+corpus dims (rate=1)", gamesPerTier, float64(corpusTier.Milliseconds())/gamesPerTier)

	overheadGame := float64(gameTier-baseline) / float64(baseline) * 100
	overheadCorpus := float64(corpusTier-baseline) / float64(baseline) * 100
	t.Logf("overhead: game-level=%.1f%%/sampled-game, +corpus=%.1f%%/sampled-game (corpus is first-seen amortized)", overheadGame, overheadCorpus)
	for _, rate := range []float64{0.01, 0.05, 0.10, 0.25} {
		eff := rate * overheadGame
		t.Logf("projected fleet overhead at HEXDEK_JUDGE_SAMPLE=%.2f: %.2f%% → ~%.1f g/min on a 127 g/min baseline",
			rate, eff, 127/(1+eff/100))
	}
}

// ---------------------------------------------------------------------------
// LIVENESS wiring (r63 follow-up: production hang self-report)
// ---------------------------------------------------------------------------

// readTriageLogIfAny is readTriageLog tolerant of a not-yet-created
// file (the lazy-open writer creates it on first record; no records =
// no file, which several liveness tests treat as success).
func readTriageLogIfAny(t *testing.T, path string) []judgeViolationRecord {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return readTriageLog(t, path)
}


// TestJudgeWatchdog_LivenessHangTimerFires pins the live wall_clock
// self-report: a sampled game that never reaches Finish writes a
// liveness record with the repro seed once the budget elapses.
func TestJudgeWatchdog_LivenessHangTimerFires(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "violations.jsonl")
	w := &judgeWatchdog{gameRate: 1.0, logPath: logPath, livenessBudget: 50 * time.Millisecond}
	rng := rand.New(rand.NewSource(1))
	jr := w.beginGame(rng, 60606, []string{"deckA"})
	if jr == nil || jr.livenessTimer == nil {
		t.Fatal("sampled game with a liveness budget must arm the hang timer")
	}
	// Simulate the hang: never call Finish.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		recs := readTriageLogIfAny(t, logPath)
		if len(recs) > 0 {
			r := recs[0]
			if r.Dimension != judge.DimensionLiveness || r.Rule != "wall_clock" {
				t.Fatalf("hang record = %+v, want liveness/wall_clock", r)
			}
			if r.GameSeed != 60606 {
				t.Errorf("seed = %d, want 60606", r.GameSeed)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("hang timer never wrote the liveness record")
}

// TestJudgeWatchdog_LivenessFinishDisarmsTimer: a game that finishes
// inside the budget must NOT self-report.
func TestJudgeWatchdog_LivenessFinishDisarmsTimer(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "violations.jsonl")
	w := &judgeWatchdog{gameRate: 1.0, logPath: logPath, livenessBudget: 80 * time.Millisecond}
	rng := rand.New(rand.NewSource(1))
	jr := w.beginGame(rng, 777, nil)

	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(2)), nil)
	gs.Flags["ended"] = 1
	jr.Finish(gs, nil, nil)

	time.Sleep(200 * time.Millisecond) // past the budget
	for _, r := range readTriageLogIfAny(t, logPath) {
		if r.Dimension == judge.DimensionLiveness && r.Rule == "wall_clock" {
			t.Fatalf("finished game self-reported a hang: %+v", r)
		}
	}
}

// TestJudgeWatchdog_LivenessCapContractViaUniformEvent: a game whose
// event log carries a loop_guard_fired but no decided end must write a
// liveness/cap_contract record — scanning the UNIFORM event kind, not
// per-guard vocabulary.
func TestJudgeWatchdog_LivenessCapContractViaUniformEvent(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "violations.jsonl")
	w := &judgeWatchdog{gameRate: 1.0, logPath: logPath}
	rng := rand.New(rand.NewSource(1))
	jr := w.beginGame(rng, 314159, nil)

	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(2)), nil)
	gameengine.LogLoopGuardFired(gs, "drain_iteration_cap", map[string]interface{}{"stack_remaining": 3})
	// Game did NOT end: the guard's abort-as-draw contract is broken.

	jr.Finish(gs, nil, nil)
	found := false
	for _, r := range readTriageLog(t, logPath) {
		if r.Dimension == judge.DimensionLiveness && r.Rule == "cap_contract" {
			found = true
			if r.GameSeed != 314159 {
				t.Errorf("seed = %d, want 314159", r.GameSeed)
			}
		}
	}
	if !found {
		t.Fatal("cap_contract record not written for guard-fired-without-end")
	}
}

// TestLoopGuardFired_UniformEventShape pins the engine helper: every
// guard emission carries Kind=loop_guard_fired + the guard name in
// both Source and Details.
func TestLoopGuardFired_UniformEventShape(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(3)), nil)
	gameengine.LogLoopGuardFired(gs, "sba_max_passes", map[string]interface{}{"passes": 40})
	var ev *gameengine.Event
	for i := range gs.EventLog {
		if gs.EventLog[i].Kind == gameengine.EventLoopGuardFired {
			ev = &gs.EventLog[i]
		}
	}
	if ev == nil {
		t.Fatal("no loop_guard_fired event")
	}
	if ev.Source != "sba_max_passes" || ev.Details["guard"] != "sba_max_passes" || ev.Details["passes"] != 40 {
		t.Errorf("malformed uniform event: %+v", ev)
	}
}
