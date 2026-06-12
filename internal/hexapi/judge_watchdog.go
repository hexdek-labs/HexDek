package hexapi

// judge_watchdog.go — the Hex Judge as an always-on sampled watchdog
// over the live grinder (r63, owner direction: "keep looking back over
// games, see what comes up log-wise").
//
// The grinder plays ~127 games/min in production; running every Judge
// dimension on every game would crater that. This watchdog samples:
//
//   - GAME-LEVEL dimensions (conservation / state-integrity / legality)
//     on HEXDEK_JUDGE_SAMPLE of games: the ride-along legality
//     validator is attached for the whole game and RunAllInvariants
//     runs after every turn (not just post-game, which every game
//     already gets) — mid-game violations that self-heal by game end
//     become visible.
//   - CORPUS dimensions (outcome / progression) on
//     HEXDEK_JUDGE_SAMPLE_CORPUS of games: the Judge's interpreters
//     run over the sampled game's deck cards. A process-wide
//     first-seen dedupe means each unique card is interpreted at most
//     once per process, so the steady-state cost decays to ~zero while
//     coverage converges on exactly the cards the live pool plays.
//
// Every violation is written as one JSONL record to a DEDICATED triage
// log (HEXDEK_JUDGE_LOG, default data/judge/grinder-violations.jsonl)
// carrying game seed + deck keys + dimension + rule + detail — enough
// to reproduce any hit (same seed + same decks = same game). A one-line
// summary also goes to the server log per flagged game so "what comes
// up log-wise" stays visible in the normal stream.
//
// Config (all env, read once at construction):
//
//   HEXDEK_JUDGE_SAMPLE         fraction [0,1] of games getting the
//                               game-level dimensions. Unset/0 = OFF:
//                               newJudgeWatchdogFromEnv returns nil and
//                               every hook is a nil-receiver no-op —
//                               zero overhead on the hot path.
//   HEXDEK_JUDGE_SAMPLE_CORPUS  fraction [0, SAMPLE] of games whose
//                               deck cards also get the outcome /
//                               progression interpreters. Default
//                               SAMPLE/10.
//   HEXDEK_JUDGE_LOG            triage JSONL path. Default
//                               data/judge/grinder-violations.jsonl.
//
// Safety: every hook body is panic-isolated — a watchdog bug can never
// take down a grinder worker — and the watchdog only ever READS game
// state (the legality validator is an observation tap by contract).

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/judge/outcome"
	"github.com/hexdek/hexdek/internal/judge/progression"
)

const defaultJudgeLogPath = "data/judge/grinder-violations.jsonl"

// judgeWatchdog is the process-wide sampled-Judge state. nil = off.
type judgeWatchdog struct {
	gameRate   float64 // P(game gets game-level dimensions)
	corpusRate float64 // P(game ALSO gets corpus dimensions); <= gameRate
	logPath    string

	logMu   sync.Mutex
	logFile *os.File
	logErr  bool // open failed once — don't retry every write

	// checkedCards dedupes corpus interpretation process-wide:
	// card name -> struct{}.
	checkedCards sync.Map

	sampledGames atomic.Int64
	flaggedGames atomic.Int64
	violations   atomic.Int64
}

// newJudgeWatchdogFromEnv builds the watchdog from HEXDEK_JUDGE_*
// env vars. Returns nil (fully off) unless HEXDEK_JUDGE_SAMPLE parses
// to a value > 0.
func newJudgeWatchdogFromEnv() *judgeWatchdog {
	rate := parseJudgeRate(os.Getenv("HEXDEK_JUDGE_SAMPLE"), 0)
	if rate <= 0 {
		return nil
	}
	corpus := parseJudgeRate(os.Getenv("HEXDEK_JUDGE_SAMPLE_CORPUS"), rate/10)
	if corpus > rate {
		corpus = rate
	}
	path := os.Getenv("HEXDEK_JUDGE_LOG")
	if path == "" {
		path = defaultJudgeLogPath
	}
	return &judgeWatchdog{gameRate: rate, corpusRate: corpus, logPath: path}
}

// parseJudgeRate parses a sample-rate env value, clamped to [0,1];
// empty or unparseable yields fallback.
func parseJudgeRate(s string, fallback float64) float64 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// judgeViolationRecord is one triage-log line. Seed + deck keys make
// the hit reproducible: replay the same seed with the same decks.
type judgeViolationRecord struct {
	TS        string   `json:"ts"`
	GameSeed  int64    `json:"game_seed"`
	SampledID int64    `json:"sampled_game"`
	DeckKeys  []string `json:"deck_keys,omitempty"`
	Dimension string   `json:"dimension"`
	Surface   string   `json:"surface,omitempty"`
	Rule      string   `json:"rule"`
	Severity  string   `json:"severity,omitempty"`
	Turn      int      `json:"turn,omitempty"`
	Seat      int      `json:"seat,omitempty"`
	Card      string   `json:"card,omitempty"`
	Detail    string   `json:"detail"`
}

// write appends one record to the triage log. Open is lazy so a server
// that never samples a violation never touches the file.
func (w *judgeWatchdog) write(rec judgeViolationRecord) {
	w.violations.Add(1)
	rec.TS = time.Now().UTC().Format(time.RFC3339)
	w.logMu.Lock()
	defer w.logMu.Unlock()
	if w.logFile == nil {
		if w.logErr {
			return
		}
		if err := os.MkdirAll(filepath.Dir(w.logPath), 0755); err != nil {
			log.Printf("judge-watchdog: mkdir %s: %v (triage log disabled)", filepath.Dir(w.logPath), err)
			w.logErr = true
			return
		}
		f, err := os.OpenFile(w.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("judge-watchdog: open %s: %v (triage log disabled)", w.logPath, err)
			w.logErr = true
			return
		}
		w.logFile = f
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	b = append(b, '\n')
	_, _ = w.logFile.Write(b)
}

// ---------------------------------------------------------------------------
// Per-game run
// ---------------------------------------------------------------------------

// judgeGameRun is one sampled game's watchdog state. nil = this game
// was not sampled (or the watchdog is off); every method is
// nil-receiver safe so call sites need no branching.
type judgeGameRun struct {
	w        *judgeWatchdog
	id       int64
	seed     int64
	deckKeys []string
	legality *gameengine.LegalityValidator
	corpus   bool
	flagged  int
	// seen dedupes repeated invariant reports within the game — a
	// persistent violation (e.g. a broken census) would otherwise
	// re-log every turn.
	seen map[string]bool
}

// beginGame rolls the sampling dice for one game. Returns nil when the
// watchdog is off or the game is not sampled — the single
// rng.Float64() comparison is the entire hot-path cost when sampling
// is enabled, and a nil-pointer check when it is not.
func (w *judgeWatchdog) beginGame(rng *rand.Rand, seed int64, deckKeys []string) *judgeGameRun {
	if w == nil {
		return nil
	}
	roll := rng.Float64()
	if roll >= w.gameRate {
		return nil
	}
	id := w.sampledGames.Add(1)
	return &judgeGameRun{
		w:        w,
		id:       id,
		seed:     seed,
		deckKeys: deckKeys,
		legality: gameengine.NewLegalityValidator(seed),
		// Corpus games are a subset of sampled games: the same roll
		// that passed gameRate also decides corpus membership, so
		// P(corpus) == corpusRate overall.
		corpus: roll < w.corpusRate,
		seen:   make(map[string]bool),
	}
}

// Attach installs the ride-along legality validator on the game state.
func (jr *judgeGameRun) Attach(gs *gameengine.GameState) {
	if jr == nil || gs == nil {
		return
	}
	gs.Legality = jr.legality
}

// AfterTurn runs the inline invariant table (conservation +
// state-integrity dimensions) and logs anything new this game.
func (jr *judgeGameRun) AfterTurn(gs *gameengine.GameState) {
	if jr == nil || gs == nil {
		return
	}
	defer func() { _ = recover() }()
	for _, v := range gameengine.RunAllInvariants(gs) {
		key := v.Name + "|" + v.Message
		if jr.seen[key] {
			continue
		}
		jr.seen[key] = true
		jr.flagged++
		dim := v.Dimension
		if dim == "" {
			dim = judge.DimensionStateIntegrity
		}
		jr.w.write(judgeViolationRecord{
			GameSeed:  jr.seed,
			SampledID: jr.id,
			DeckKeys:  jr.deckKeys,
			Dimension: dim,
			Surface:   v.Surface,
			Rule:      v.Name,
			Severity:  v.Severity,
			Turn:      gs.Turn,
			Seat:      v.Seat,
			Detail:    v.Message,
		})
	}
}

// Finish drains the legality validator, records the caller's post-game
// Feynman violations (computed once by the game loop for every game —
// not recomputed here), runs the corpus dimensions when this game drew
// the corpus sample, and emits the server-log summary line.
func (jr *judgeGameRun) Finish(gs *gameengine.GameState, feynman []judge.ValidationViolation, decks []*deckparser.TournamentDeck) {
	if jr == nil {
		return
	}
	defer func() { _ = recover() }()

	finalTurn := 0
	if gs != nil {
		finalTurn = gs.Turn
	}

	// LEGALITY — drain the ride-along validator.
	for _, lv := range jr.legality.Violations {
		jr.flagged++
		jr.w.write(judgeViolationRecord{
			GameSeed:  jr.seed,
			SampledID: jr.id,
			DeckKeys:  jr.deckKeys,
			Dimension: judge.DimensionLegality,
			Surface:   judge.SurfaceLegality,
			Rule:      lv.Rule,
			Severity:  judge.SeverityCritical,
			Turn:      lv.Turn,
			Seat:      lv.Seat,
			Detail:    lv.Action + ": " + lv.Detail,
		})
	}

	// Post-game Feynman (state-integrity / conservation) — structured
	// copies of what the game loop already prints to the server log.
	for _, v := range feynman {
		key := v.Name + "|" + v.Message
		if jr.seen[key] {
			continue
		}
		jr.seen[key] = true
		jr.flagged++
		dim := v.Dimension
		if dim == "" {
			dim = judge.DimensionStateIntegrity
		}
		jr.w.write(judgeViolationRecord{
			GameSeed:  jr.seed,
			SampledID: jr.id,
			DeckKeys:  jr.deckKeys,
			Dimension: dim,
			Surface:   v.Surface,
			Rule:      v.Name,
			Severity:  v.Severity,
			Turn:      finalTurn,
			Seat:      v.Seat,
			Detail:    v.Message,
		})
	}

	// OUTCOME / PROGRESSION — interpret this game's deck cards
	// (first-seen process-wide).
	if jr.corpus {
		for _, d := range decks {
			if d == nil {
				continue
			}
			for _, c := range d.CommanderCards {
				jr.checkCard(c)
			}
			for _, c := range d.Library {
				jr.checkCard(c)
			}
		}
	}

	if jr.flagged > 0 {
		jr.w.flaggedGames.Add(1)
		log.Printf("judge-watchdog: game seed=%d decks=%v flagged %d violation(s) → %s",
			jr.seed, jr.deckKeys, jr.flagged, jr.w.logPath)
	}
	if jr.id%200 == 0 {
		log.Printf("judge-watchdog: %d games sampled, %d flagged, %d violations logged (rates: game=%.3f corpus=%.3f)",
			jr.id, jr.w.flaggedGames.Load(), jr.w.violations.Load(), jr.w.gameRate, jr.w.corpusRate)
	}
}

// checkCard runs the outcome + progression interpreters over one card,
// once per process per card name. Panic-isolated per card.
func (jr *judgeGameRun) checkCard(c *gameengine.Card) {
	if c == nil || c.Name == "" || c.AST == nil {
		return
	}
	if _, dup := jr.w.checkedCards.LoadOrStore(c.Name, struct{}{}); dup {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			jr.w.write(judgeViolationRecord{
				GameSeed:  jr.seed,
				SampledID: jr.id,
				Dimension: judge.DimensionOutcome,
				Surface:   judge.SurfaceGoldilocks,
				Rule:      "harness_panic",
				Severity:  judge.SeverityCritical,
				Card:      c.Name,
				Detail:    fmt.Sprintf("interpreter panic: %v", r),
			})
		}
	}()

	// OUTCOME: enters-with-counters statics + resolvable effect trees.
	for _, ab := range c.AST.Abilities {
		if st, ok := ab.(*gameast.Static); ok && st.Modification != nil {
			if fd, ran := outcome.CheckETBCounters(c.Name, st.Modification); ran && fd != nil {
				jr.writeCorpusFinding(judge.DimensionOutcome, c.Name, "etb_with_counters",
					fmt.Sprintf("expected %d %s, got %d", fd.Want, fd.Kind, fd.Got))
			}
		}
	}
	for _, ex := range outcome.ExtractEffects(c.AST) {
		if fd, ran := outcome.RunEffect(c.Name, ex.Raw, ex.Effect); ran && fd != nil {
			jr.writeCorpusFinding(judge.DimensionOutcome, c.Name, fd.Kind,
				fmt.Sprintf("expected %s, got %s (raw: %s)", fd.Expected, fd.Actual, fd.Raw))
		}
	}

	// PROGRESSION: trigger-correctness over the in-scope vocabulary.
	for _, ab := range c.AST.Abilities {
		tr, ok := ab.(*gameast.Triggered)
		if !ok {
			continue
		}
		var findings []*progression.Finding
		var ran bool
		if findings, ran = progression.CheckTrigger(c.Name, tr); !ran {
			if findings, ran = progression.CheckPhaseTrigger(c.Name, tr); !ran {
				findings, ran = progression.CheckLTBTrigger(c.Name, tr)
			}
		}
		if !ran {
			continue
		}
		for _, fd := range findings {
			jr.writeCorpusFinding(judge.DimensionProgression, c.Name, fd.Event+"/"+fd.Check,
				fmt.Sprintf("expected %s, got %s (raw: %s)", fd.Expected, fd.Actual, fd.Raw))
		}
	}
}

func (jr *judgeGameRun) writeCorpusFinding(dimension, card, rule, detail string) {
	jr.flagged++
	jr.w.write(judgeViolationRecord{
		GameSeed:  jr.seed,
		SampledID: jr.id,
		Dimension: dimension,
		Surface:   judge.SurfaceGoldilocks,
		Rule:      rule,
		Severity:  judge.SeverityCritical,
		Card:      card,
		Detail:    detail,
	})
}
