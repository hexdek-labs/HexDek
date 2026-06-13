package main

// determinism_test.go — seed-reproducibility regression (r63 audit).
//
// The entire repro/baseline system (loki --seed reproduction, the
// issue-log repro lines, the correctness seed=42 sweeps, parity replays)
// depends on a game being EXACTLY reproducible from its seed. This test
// proves it empirically: it plays the SAME seed twice IN ONE PROCESS and
// asserts the two runs produce byte-identical fingerprints (full event
// log + per-seat zone digests + InstanceID census + final life/turn +
// any panic text).
//
// Why an in-process double-run is a valid nondeterminism probe: Go
// randomizes map iteration order per range-loop (the runtime advances a
// per-thread fastrand seed), so two separate game runs iterate their maps
// in different orders. If ANY game-affecting decision — target choice,
// trigger ordering, card selection, mint order — were driven by map
// iteration order (the classic Go determinism leak), or by an un-seeded
// global math/rand source, or by wall-clock time, the two fingerprints
// would diverge. Identical fingerprints across many random decks is
// strong evidence the engine is seed-deterministic.
//
// The game setup mirrors cmd/hexdek-loki's runChaosGame (the canonical
// repro path) minus the fuzz-targeting / invariant-census extras, which
// do not affect game state. Requires the oracle + AST corpora; skips
// cleanly (like a fresh CI checkout) when they are absent.

import (
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

// detCorpus bundles the loaded corpora so the test loads them once.
type detCorpus struct {
	chaos *gameengine.ChaosCorpus
	ast   *astload.Corpus
	meta  *deckparser.MetaDB
}

// loadDetCorpus loads the corpora from HEXDEK_AST / HEXDEK_ORACLE (or the
// default repo-relative paths). Returns nil when the data files are
// absent — the caller skips.
func loadDetCorpus(t *testing.T) *detCorpus {
	t.Helper()
	astPath := os.Getenv("HEXDEK_AST")
	if astPath == "" {
		astPath = "../../data/rules/ast_dataset.jsonl"
	}
	oraclePath := os.Getenv("HEXDEK_ORACLE")
	if oraclePath == "" {
		oraclePath = "../../data/rules/oracle-cards.json"
	}
	if _, err := os.Stat(astPath); err != nil {
		t.Skipf("AST corpus not found at %s (set HEXDEK_AST); skipping determinism soak", astPath)
		return nil
	}
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle corpus not found at %s (set HEXDEK_ORACLE); skipping determinism soak", oraclePath)
		return nil
	}
	ast, err := astload.Load(astPath)
	if err != nil {
		t.Fatalf("astload: %v", err)
	}
	meta, err := deckparser.LoadMetaFromJSONL(astPath)
	if err != nil {
		t.Fatalf("deckparser meta: %v", err)
	}
	if err := meta.SupplementWithOracleJSON(oraclePath); err != nil {
		t.Logf("oracle P/T supplement: %v (continuing)", err)
	}
	chaos, err := loadOracleCorpus(oraclePath)
	if err != nil {
		t.Fatalf("oracle corpus: %v", err)
	}
	return &detCorpus{chaos: chaos, ast: ast, meta: meta}
}

// playDetGame plays one chaos game and returns a deterministic fingerprint.
// Setup mirrors runChaosGame (same seed derivation) so a divergence here is
// a divergence in the real repro path. A panic is captured into the
// fingerprint so a DETERMINISTIC panic still compares equal across runs.
func playDetGame(dc *detCorpus, gameIdx, nSeats, maxTurns int, masterSeed int64) (fp string) {
	deckSeed := masterSeed + int64(gameIdx)*10000 + 1
	shuffleSeed := deckSeed + 7
	deckRng := rand.New(rand.NewSource(deckSeed))
	gameRng := rand.New(rand.NewSource(shuffleSeed))

	var panicText string
	defer func() {
		if r := recover(); r != nil {
			panicText = fmt.Sprintf("PANIC:%v", r)
			_ = debug.Stack()
		}
		fp = panicText + "\n" + fp
	}()

	chaosDecks := make([]*gameengine.ChaosDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		chaosDecks[i] = gameengine.GenerateChaosDeck(dc.chaos, deckRng)
		if chaosDecks[i] == nil {
			return "nil-deck"
		}
	}

	gs := gameengine.NewGameState(nSeats, gameRng, dc.ast)
	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i, cd := range chaosDecks {
		cmdrCard := buildCardFromName(cd.Commander.Name, dc.ast, dc.meta)
		if cmdrCard == nil {
			cmdrCard = &gameengine.Card{
				Name: cd.Commander.Name, Owner: i,
				Types:     []string{"legendary", "creature"},
				BasePower: cd.Commander.Power, BaseToughness: cd.Commander.Toughness,
				CMC: cd.Commander.CMC, Colors: cd.Commander.Colors,
			}
			if cmdrCard.BaseToughness == 0 {
				cmdrCard.BaseToughness = 1
			}
		} else {
			cmdrCard.Owner = i
		}
		lib := make([]*gameengine.Card, 0, len(cd.Cards))
		for _, name := range cd.Cards {
			c := buildCardFromName(name, dc.ast, dc.meta)
			if c == nil {
				c = &gameengine.Card{Name: name, Owner: i}
				for _, cc := range dc.chaos.All {
					if cc.Name == name {
						c.Types = cc.Types
						c.BasePower = cc.Power
						c.BaseToughness = cc.Toughness
						c.CMC = cc.CMC
						c.Colors = cc.Colors
						break
					}
				}
			} else {
				c.Owner = i
			}
			lib = append(lib, c)
		}
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		commanderDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: []*gameengine.Card{cmdrCard},
			Library:        lib,
		}
	}

	gameengine.SetupCommanderGame(gs, commanderDecks)
	for i := 0; i < nSeats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}
	for i := 0; i < nSeats; i++ {
		for j := 0; j < 7; j++ {
			if len(gs.Seats[i].Library) == 0 {
				break
			}
			c := gs.Seats[i].Library[0]
			gs.Seats[i].Library = gs.Seats[i].Library[1:]
			gs.Seats[i].Hand = append(gs.Seats[i].Hand, c)
		}
	}
	gs.Active = gameRng.Intn(nSeats)
	gs.Turn = 1

	for turn := 1; turn <= maxTurns; turn++ {
		gs.Turn = turn
		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)
		if gs.CheckEnd() {
			break
		}
		gs.Active = gameengine.NextLivingSeat(gs)
	}

	fp = fingerprintGame(gs)
	return fp
}

// fingerprintGame serializes the post-game state into a stable string:
// final turn, per-seat life/poison + ordered zone digests (card name +
// InstanceID, plus tapped + sorted counters for battlefield permanents),
// and the full event-log stream. Map-typed payloads (counters, event
// Details) are key-sorted so the FINGERPRINT itself never introduces
// map-order noise — any divergence reflects the GAME, not the digest.
func fingerprintGame(gs *gameengine.GameState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "turn=%d active=%d\n", gs.Turn, gs.Active)
	for si, s := range gs.Seats {
		if s == nil {
			fmt.Fprintf(&b, "seat%d=nil\n", si)
			continue
		}
		fmt.Fprintf(&b, "seat%d life=%d poison=%d left=%v\n", si, s.Life, s.PoisonCounters, s.LeftGame)
		fmt.Fprintf(&b, "  lib=%s\n", cardsDigest(s.Library))
		fmt.Fprintf(&b, "  hand=%s\n", cardsDigest(s.Hand))
		fmt.Fprintf(&b, "  gy=%s\n", cardsDigest(s.Graveyard))
		fmt.Fprintf(&b, "  exile=%s\n", cardsDigest(s.Exile))
		fmt.Fprintf(&b, "  cmd=%s\n", cardsDigest(s.CommandZone))
		fmt.Fprintf(&b, "  bf=%s\n", permsDigest(s.Battlefield))
	}
	b.WriteString("events:\n")
	for i := range gs.EventLog {
		e := &gs.EventLog[i]
		fmt.Fprintf(&b, "  %s|s%d|t%d|%s|%d\n", e.Kind, e.Seat, e.Target, e.Source, e.Amount)
	}
	return b.String()
}

func cardsDigest(cards []*gameengine.Card) string {
	parts := make([]string, 0, len(cards))
	for _, c := range cards {
		if c == nil {
			parts = append(parts, "<nil>")
			continue
		}
		parts = append(parts, c.Name+"#"+c.InstanceID)
	}
	return fmt.Sprintf("[%d]%s", len(cards), strings.Join(parts, ","))
}

func permsDigest(perms []*gameengine.Permanent) string {
	parts := make([]string, 0, len(perms))
	for _, p := range perms {
		if p == nil || p.Card == nil {
			parts = append(parts, "<nil>")
			continue
		}
		ctr := ""
		if len(p.Counters) > 0 {
			keys := make([]string, 0, len(p.Counters))
			for k := range p.Counters {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			cs := make([]string, 0, len(keys))
			for _, k := range keys {
				cs = append(cs, fmt.Sprintf("%s=%d", k, p.Counters[k]))
			}
			ctr = "{" + strings.Join(cs, ",") + "}"
		}
		tap := ""
		if p.Tapped {
			tap = "T"
		}
		parts = append(parts, p.Card.Name+"#"+p.Card.InstanceID+tap+ctr)
	}
	return fmt.Sprintf("[%d]%s", len(perms), strings.Join(parts, ","))
}

// firstDiff returns a short window around the first line that differs
// between two fingerprints, for actionable failure output.
func firstDiff(a, b string) string {
	la := strings.Split(a, "\n")
	lb := strings.Split(b, "\n")
	n := len(la)
	if len(lb) < n {
		n = len(lb)
	}
	for i := 0; i < n; i++ {
		if la[i] != lb[i] {
			lo := i - 2
			if lo < 0 {
				lo = 0
			}
			var sb strings.Builder
			for j := lo; j <= i; j++ {
				fmt.Fprintf(&sb, "  line %d:\n    run1: %s\n    run2: %s\n", j, la[j], lb[j])
			}
			return sb.String()
		}
	}
	return fmt.Sprintf("(no line diff; len run1=%d run2=%d)", len(la), len(lb))
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// TestSeedDeterminism_DoubleRun plays many distinct random decks twice
// each from the same master seed and asserts byte-identical fingerprints.
func TestSeedDeterminism_DoubleRun(t *testing.T) {
	if testing.Short() {
		t.Skip("determinism soak skipped under -short")
	}
	dc := loadDetCorpus(t)
	if dc == nil {
		return // skipped
	}
	const (
		masterSeed = 42
		nSeats     = 4
	)
	// Defaults give solid CI coverage (~17s); env overrides let a heavier
	// local soak crank the sweep without touching the committed defaults.
	maxTurns := envInt("HEXDEK_DET_TURNS", 40)
	nGames := envInt("HEXDEK_DET_GAMES", 16)
	for g := 0; g < nGames; g++ {
		fp1 := playDetGame(dc, g, nSeats, maxTurns, masterSeed)
		fp2 := playDetGame(dc, g, nSeats, maxTurns, masterSeed)
		if fp1 != fp2 {
			t.Fatalf("NONDETERMINISM at gameIdx=%d (seed %d): same seed produced different games.\n%s",
				g, masterSeed, firstDiff(fp1, fp2))
		}
	}
	t.Logf("determinism confirmed: %d distinct decks, each identical across a double run (seed %d, %d seats, %d-turn cap)",
		nGames, masterSeed, nSeats, maxTurns)
}
