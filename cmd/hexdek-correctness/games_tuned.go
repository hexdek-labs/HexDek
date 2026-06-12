package main

// games_tuned.go — the TUNED-DECK game sweep (r63): closes the "chaos
// games, not tuned-deck play" caveat on the correctness score.
//
// The chaos sweep (games.go) samples random-deck GreedyHat games; real
// competitive Commander — tuned Moxfield decks piloted by the stronger
// YggdrasilHat — exercises interactions chaos under-samples (combos,
// stax locks, deep stacks, commander-tax loops). This sweep runs the
// SAME game-level Judge surfaces over the real deck pool with a mixed
// hat table (two Yggdrasil pilots + two Greedy pilots per game), so the
// game-level dimensions are scored on both populations.
//
// Tuned-dimension rows are reported as "<dimension>_tuned" alongside
// the chaos rows and participate in the topline mean — the headline
// number now claims correctness for real play, not just chaos.

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/tournament"
)

// yggBudget is the YggdrasilHat decision budget for tuned games:
// evaluator-guided (1-199 band) — strong play without rollout cost.
const yggBudget = 80

// loadTunedDeckPool walks deckDir recursively for Moxfield-format .txt
// decklists and parses them against the corpus. Decks that fail to
// parse, lack a commander, or resolve fewer than 80 real cards are
// skipped (partial decks would distort the sample toward degenerate
// games). Returns the pool sorted by path for determinism.
func loadTunedDeckPool(deckDir string, corpus *astload.Corpus, meta *deckparser.MetaDB) []*deckparser.TournamentDeck {
	var paths []string
	_ = filepath.Walk(deckDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(p) == ".txt" {
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)

	var pool []*deckparser.TournamentDeck
	skipped := 0
	for _, p := range paths {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil || d == nil || len(d.CommanderCards) == 0 || len(d.Library) < 80 {
			skipped++
			continue
		}
		pool = append(pool, d)
	}
	fmt.Fprintf(os.Stderr, "tuned deck pool: %d decks loaded, %d skipped (no commander / <80 resolved / parse error)\n",
		len(pool), skipped)
	return pool
}

// runTunedGamePass mirrors runGamePass over the tuned pool. Same sink,
// same tally semantics, distinct dimension labels.
func runTunedGamePass(pool []*deckparser.TournamentDeck, corpus *astload.Corpus, cfg gameConfig) ([]DimensionScore, *GamePassStats) {
	stats := &GamePassStats{
		Games:            cfg.Games,
		Seats:            cfg.Seats,
		MaxTurns:         cfg.MaxTurns,
		Seed:             cfg.Seed,
		ViolationsByName: map[string]int{},
		GamesAffected:    map[string]int{},
	}
	agg := legalityTally{checkedByKind: map[string]int{}, illegalByKind: map[string]int{}}
	conservationClean, integrityClean := 0, 0

	var cur *gameTally
	unregister := judge.RegisterSink(func(v judge.ValidationViolation) {
		if cur == nil {
			return
		}
		cur.names[v.Surface+"/"+v.Name]++
		if os.Getenv("CORRECTNESS_DEBUG_VIOLATIONS") != "" {
			fmt.Fprintf(os.Stderr, "  TUNED VIOLATION %s\n", v.String())
		}
		if v.Severity == judge.SeverityInfo {
			return
		}
		dim := v.Dimension
		if dim == "" {
			dim = judge.DimensionStateIntegrity
		}
		cur.dims[dim]++
	})
	defer unregister()

	for gameIdx := 0; gameIdx < cfg.Games; gameIdx++ {
		tally := &gameTally{dims: map[string]int{}, names: map[string]int{}}
		cur = tally
		out := runOneTunedGame(gameIdx, pool, corpus, cfg)
		cur = nil

		agg.add(out.legality)
		stats.TotalTurns += out.turns
		stats.Crashes += out.crashes
		if out.crashed {
			stats.CrashedGames++
		}
		for name, n := range tally.names {
			stats.ViolationsByName[name] += n
		}
		seen := map[string]bool{}
		for dim, n := range tally.dims {
			if n > 0 {
				seen[dim] = true
				stats.GamesAffected[dim]++
			}
		}
		if !seen[judge.DimensionConservation] {
			conservationClean++
		}
		if !seen[judge.DimensionStateIntegrity] && !out.crashed {
			integrityClean++
		}
		if (gameIdx+1)%25 == 0 {
			fmt.Fprintf(os.Stderr, "  tuned sweep: %d/%d games done\n", gameIdx+1, cfg.Games)
		}
	}

	actChecked, actIllegal := agg.totals(playerActionKinds, true)

	dims := []DimensionScore{
		{
			Dimension: judge.DimensionLegality + "_tuned",
			Unit:      "actions",
			Checked:   actChecked,
			Passed:    actChecked - actIllegal,
			Pct:       pct(actChecked-actIllegal, actChecked),
			Detail: map[string]interface{}{
				"checked_by_kind": agg.checkedByKind,
				"illegal_by_kind": agg.illegalByKind,
			},
		},
		{
			Dimension: judge.DimensionConservation + "_tuned",
			Unit:      "games",
			Checked:   cfg.Games,
			Passed:    conservationClean,
			Pct:       pct(conservationClean, cfg.Games),
		},
		{
			Dimension: judge.DimensionStateIntegrity + "_tuned",
			Unit:      "games",
			Checked:   cfg.Games,
			Passed:    integrityClean,
			Pct:       pct(integrityClean, cfg.Games),
			Detail: map[string]interface{}{
				"crashed_games": stats.CrashedGames,
			},
		},
	}
	return dims, stats
}

// runOneTunedGame plays one 4-seat game over real decks: the deck RNG
// samples four distinct pool decks; seats 0+2 get YggdrasilHat pilots,
// seats 1+3 GreedyHat (mixed table — strong lines AND greedy chaos in
// the same game). Same turn/invariant/Feynman cadence as the chaos
// sweep.
func runOneTunedGame(gameIdx int, pool []*deckparser.TournamentDeck, corpus *astload.Corpus, cfg gameConfig) (out gameOutcome) {
	out.legality = legalityTally{checkedByKind: map[string]int{}, illegalByKind: map[string]int{}}

	deckSeed := cfg.Seed + int64(gameIdx)*10000 + 5001
	shuffleSeed := deckSeed + 7
	deckRng := rand.New(rand.NewSource(deckSeed))
	gameRng := rand.New(rand.NewSource(shuffleSeed))

	defer func() {
		if r := recover(); r != nil {
			out.crashed = true
			out.crashes++
			fmt.Fprintf(os.Stderr, "  tuned game %d: setup panic: %v\n%s\n", gameIdx, r, truncStack(debug.Stack()))
		}
	}()

	// Sample four distinct decks.
	picked := deckRng.Perm(len(pool))[:cfg.Seats]

	gs := gameengine.NewGameState(cfg.Seats, gameRng, corpus)
	gs.Legality = gameengine.NewLegalityValidator(deckSeed)
	gs.SeatOutcome = gameengine.NewSeatOutcomeChecker()
	attachLegalityCensus(gs.Legality, &out.legality)

	commanderDecks := make([]*gameengine.CommanderDeck, cfg.Seats)
	for i := 0; i < cfg.Seats; i++ {
		tpl := pool[picked[i]]
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		commanderDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: cmdrs,
			Library:        lib,
		}
	}

	gameengine.SetupCommanderGame(gs, commanderDecks)

	// Mixed hat table: even seats Yggdrasil (strong), odd seats Greedy.
	for i := 0; i < cfg.Seats; i++ {
		if i%2 == 0 {
			gs.Seats[i].Hat = hat.NewYggdrasilHat(nil, yggBudget)
		} else {
			gs.Seats[i].Hat = &hat.GreedyHat{}
		}
	}

	// Opening hands via the real London mulligan (tuned decks earn
	// real openers; the chaos sweep's straight-7 is fine for noise
	// decks but would skew tuned games toward mana-screw artifacts).
	for i := 0; i < cfg.Seats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}
	gs.Active = gameRng.Intn(cfg.Seats)
	gs.Turn = 1

	runInvariants := func() { gameengine.RunAllInvariants(gs) }
	runInvariants()

	for turn := 1; turn <= cfg.MaxTurns; turn++ {
		gs.Turn = turn

		func() {
			defer func() {
				if r := recover(); r != nil {
					out.crashed = true
					out.crashes++
					fmt.Fprintf(os.Stderr, "  tuned game %d turn %d: panic: %v\n%s\n", gameIdx, turn, r, truncStack(debug.Stack()))
				}
			}()
			tournament.TakeTurn(gs)
		}()
		runInvariants()

		func() {
			defer func() {
				if r := recover(); r != nil {
					out.crashed = true
					out.crashes++
					fmt.Fprintf(os.Stderr, "  tuned game %d turn %d: SBA panic: %v\n%s\n", gameIdx, turn, r, truncStack(debug.Stack()))
				}
			}()
			gameengine.StateBasedActions(gs)
		}()
		runInvariants()

		if gs.CheckEnd() {
			break
		}
		gs.Active = gameengine.NextLivingSeat(gs)

		if out.crashes > 10 {
			break
		}
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				out.crashed = true
				out.crashes++
				fmt.Fprintf(os.Stderr, "  tuned game %d: feynman panic: %v\n", gameIdx, r)
			}
		}()
		hat.CheckGame(gs)
	}()

	out.turns = gs.Turn
	return out
}
