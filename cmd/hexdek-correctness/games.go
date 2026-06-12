package main

// games.go — the game-level sweep feeding the LEGALITY, CONSERVATION,
// and STATE-INTEGRITY dimensions. Runs loki-style chaos games (same
// deck generator, seed derivation, turn loop, and invariant cadence as
// cmd/hexdek-loki's runChaosGame, minus the fuzz-targeting extras) with
// every game-level Judge surface enabled:
//
//   - ride-along legality validator (gs.Legality)
//   - per-seat win/loss self-checker (gs.SeatOutcome)
//   - RunAllInvariants after setup, after every turn, after every SBA
//   - Feynman post-game oracle (hat.CheckGame) at game end
//
// All violations are consumed through ONE judge.RegisterSink — this
// driver never re-derives correctness, it only counts the Judge's
// canonical ValidationViolation stream.
//
// Dimension semantics:
//
//	LEGALITY         per ACTION: a counting check registered on the
//	                 validator (running after the built-in checks for
//	                 each observation) marks the observation illegal
//	                 when the validator's violation list grew while
//	                 checking it. The headline counts player actions
//	                 (cast / activate / attack / block); the passive
//	                 etb / zone_change replacement observations are
//	                 tallied separately so their volume can't dilute
//	                 the action pass rate.
//	CONSERVATION     per GAME: clean = zero non-info conservation-
//	                 dimension violations across the whole game.
//	STATE-INTEGRITY  per GAME: clean = zero non-info state-integrity-
//	                 dimension violations AND no panic (a crash is an
//	                 integrity failure by definition).

import (
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"strings"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/tournament"
)

// playerActionKinds are the observation kinds that count as player
// actions for the headline legality rate.
var playerActionKinds = map[string]bool{
	"cast": true, "activate": true, "attack": true, "block": true,
}

type gameConfig struct {
	Games    int
	Seats    int
	MaxTurns int
	Seed     int64
}

// gameTally collects one game's violation stream by dimension.
type gameTally struct {
	dims  map[string]int // dimension -> non-info violation count
	names map[string]int // "surface/Name" -> count (raw stream)
}

type legalityTally struct {
	checkedByKind map[string]int
	illegalByKind map[string]int
}

func (lt *legalityTally) add(other legalityTally) {
	for k, v := range other.checkedByKind {
		lt.checkedByKind[k] += v
	}
	for k, v := range other.illegalByKind {
		lt.illegalByKind[k] += v
	}
}

func (lt legalityTally) totals(kinds map[string]bool, want bool) (checked, illegal int) {
	for k, v := range lt.checkedByKind {
		if kinds[k] == want {
			checked += v
		}
	}
	for k, v := range lt.illegalByKind {
		if kinds[k] == want {
			illegal += v
		}
	}
	return checked, illegal
}

type gameOutcome struct {
	crashed  bool
	crashes  int
	turns    int
	legality legalityTally
}

// runGamePass runs the chaos-game sweep and converts the per-game
// tallies into the three game-level dimension scores.
func runGamePass(chaosCorpus *gameengine.ChaosCorpus, corpus *astload.Corpus, meta *deckparser.MetaDB, cfg gameConfig) ([]DimensionScore, *GamePassStats) {
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

	// One sink for the whole sweep; games run sequentially so the
	// current game's tally is unambiguous.
	var cur *gameTally
	unregister := judge.RegisterSink(func(v judge.ValidationViolation) {
		if cur == nil {
			return
		}
		cur.names[v.Surface+"/"+v.Name]++
		if os.Getenv("CORRECTNESS_DEBUG_VIOLATIONS") != "" {
			fmt.Fprintf(os.Stderr, "  VIOLATION %s\n", v.String())
		}
		if v.Severity == judge.SeverityInfo {
			return
		}
		dim := v.Dimension
		if dim == "" {
			// Pre-fold emissions without a dimension tag default to
			// state_integrity, matching RunAllInvariants' own default.
			dim = judge.DimensionStateIntegrity
		}
		cur.dims[dim]++
	})
	defer unregister()

	for gameIdx := 0; gameIdx < cfg.Games; gameIdx++ {
		tally := &gameTally{dims: map[string]int{}, names: map[string]int{}}
		cur = tally
		out := runOneGame(gameIdx, chaosCorpus, corpus, meta, cfg)
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
		if (gameIdx+1)%50 == 0 {
			fmt.Fprintf(os.Stderr, "  game sweep: %d/%d games done\n", gameIdx+1, cfg.Games)
		}
	}

	actChecked, actIllegal := agg.totals(playerActionKinds, true)
	repChecked, repIllegal := agg.totals(playerActionKinds, false)

	dims := []DimensionScore{
		{
			Dimension: judge.DimensionLegality,
			Unit:      "actions",
			Checked:   actChecked,
			Passed:    actChecked - actIllegal,
			Pct:       pct(actChecked-actIllegal, actChecked),
			Detail: map[string]interface{}{
				"checked_by_kind":     agg.checkedByKind,
				"illegal_by_kind":     agg.illegalByKind,
				"replacement_checked": repChecked,
				"replacement_illegal": repIllegal,
			},
		},
		{
			Dimension: judge.DimensionConservation,
			Unit:      "games",
			Checked:   cfg.Games,
			Passed:    conservationClean,
			Pct:       pct(conservationClean, cfg.Games),
		},
		{
			Dimension: judge.DimensionStateIntegrity,
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

// runOneGame mirrors loki's runChaosGame (same seed derivation so a
// game here is reproducible under loki with the same master seed).
func runOneGame(gameIdx int, chaosCorpus *gameengine.ChaosCorpus, corpus *astload.Corpus, meta *deckparser.MetaDB, cfg gameConfig) (out gameOutcome) {
	out.legality = legalityTally{checkedByKind: map[string]int{}, illegalByKind: map[string]int{}}

	deckSeed := cfg.Seed + int64(gameIdx)*10000 + 1
	shuffleSeed := deckSeed + 7
	deckRng := rand.New(rand.NewSource(deckSeed))
	gameRng := rand.New(rand.NewSource(shuffleSeed))

	defer func() {
		if r := recover(); r != nil {
			out.crashed = true
			out.crashes++
			fmt.Fprintf(os.Stderr, "  game %d: setup panic: %v\n%s\n", gameIdx, r, truncStack(debug.Stack()))
		}
	}()

	chaosDecks := make([]*gameengine.ChaosDeck, cfg.Seats)
	for i := 0; i < cfg.Seats; i++ {
		chaosDecks[i] = gameengine.GenerateChaosDeck(chaosCorpus, deckRng)
		if chaosDecks[i] == nil {
			out.crashed = true
			out.crashes++
			return out
		}
	}

	gs := gameengine.NewGameState(cfg.Seats, gameRng, corpus)
	gs.Legality = gameengine.NewLegalityValidator(deckSeed)
	gs.SeatOutcome = gameengine.NewSeatOutcomeChecker()
	attachLegalityCensus(gs.Legality, &out.legality)

	commanderDecks := make([]*gameengine.CommanderDeck, cfg.Seats)
	for i, cd := range chaosDecks {
		cmdrCard := buildCardFromName(cd.Commander.Name, corpus, meta)
		if cmdrCard == nil {
			cmdrCard = &gameengine.Card{
				Name:          cd.Commander.Name,
				Owner:         i,
				Types:         []string{"legendary", "creature"},
				BasePower:     cd.Commander.Power,
				BaseToughness: cd.Commander.Toughness,
				CMC:           cd.Commander.CMC,
				Colors:        cd.Commander.Colors,
			}
			if cmdrCard.BaseToughness == 0 {
				cmdrCard.BaseToughness = 1
			}
		} else {
			cmdrCard.Owner = i
		}

		lib := make([]*gameengine.Card, 0, len(cd.Cards))
		for _, name := range cd.Cards {
			c := buildCardFromName(name, corpus, meta)
			if c == nil {
				c = &gameengine.Card{Name: name, Owner: i}
				for _, cc := range chaosCorpus.All {
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

	for i := 0; i < cfg.Seats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}
	for i := 0; i < cfg.Seats; i++ {
		for j := 0; j < 7; j++ {
			if len(gs.Seats[i].Library) == 0 {
				break
			}
			c := gs.Seats[i].Library[0]
			gs.Seats[i].Library = gs.Seats[i].Library[1:]
			gs.Seats[i].Hand = append(gs.Seats[i].Hand, c)
		}
	}
	gs.Active = gameRng.Intn(cfg.Seats)
	gs.Turn = 1

	// RunAllInvariants routes every violation through the sink at
	// origin — this driver only triggers the cadence, loki-style.
	runInvariants := func() { gameengine.RunAllInvariants(gs) }
	runInvariants()

	for turn := 1; turn <= cfg.MaxTurns; turn++ {
		gs.Turn = turn

		func() {
			defer func() {
				if r := recover(); r != nil {
					out.crashed = true
					out.crashes++
					fmt.Fprintf(os.Stderr, "  game %d turn %d: panic: %v\n%s\n", gameIdx, turn, r, truncStack(debug.Stack()))
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
					fmt.Fprintf(os.Stderr, "  game %d turn %d: SBA panic: %v\n%s\n", gameIdx, turn, r, truncStack(debug.Stack()))
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

	// Feynman post-game oracle — routes its violations at origin.
	func() {
		defer func() {
			if r := recover(); r != nil {
				out.crashed = true
				out.crashes++
				fmt.Fprintf(os.Stderr, "  game %d: feynman panic: %v\n", gameIdx, r)
			}
		}()
		hat.CheckGame(gs)
	}()

	out.turns = gs.Turn
	return out
}

// attachLegalityCensus registers a passive counting check on the
// validator. NewLegalityValidator installs the built-in checks first,
// so by the time this check runs for an observation every built-in
// violation for that observation has been recorded — the violation
// list growing since the previous observation marks THIS one illegal.
// (Saturation caveat: once the validator's MaxViolations cap stops the
// list growing, later illegal actions are undercounted; the cap is
// 2000/game, far above observed densities.)
func attachLegalityCensus(v *gameengine.LegalityValidator, tally *legalityTally) {
	last := 0
	v.Register(gameengine.LegalityCheck{
		Name: "correctness-census",
		Fn: func(_ *gameengine.GameState, obs *gameengine.LegalityObservation) []gameengine.LegalityViolation {
			tally.checkedByKind[obs.Kind]++
			if n := len(v.Violations); n != last {
				tally.illegalByKind[obs.Kind]++
				last = n
			}
			return nil
		},
	})
}

// ---------------------------------------------------------------------------
// Oracle corpus → ChaosCorpus + card building (mirrors cmd/hexdek-loki,
// which keeps these in package main; the semantics must stay identical
// so this sweep samples the same card population as the loki baselines).
// ---------------------------------------------------------------------------

func truncStack(b []byte) string {
	s := string(b)
	if len(s) > 1200 {
		s = s[:1200] + "…"
	}
	return s
}

func buildCardFromName(name string, corpus *astload.Corpus, meta *deckparser.MetaDB) *gameengine.Card {
	var cardAST *gameast.CardAST
	if corpus != nil {
		cardAST, _ = corpus.Get(name)
	}
	md := meta.Get(name)

	if cardAST == nil && md == nil {
		// DFC fallback: retry per face (front first) before giving up.
		if strings.Contains(name, " // ") {
			for _, face := range strings.Split(name, " // ") {
				face = strings.TrimSpace(face)
				if face == "" || face == name {
					continue
				}
				if got := buildCardFromName(face, corpus, meta); got != nil {
					return got
				}
			}
		}
		return nil
	}

	c := &gameengine.Card{
		AST:   cardAST,
		Name:  name,
		Owner: -1,
	}
	if md != nil {
		c.Name = md.Name
		if len(md.Types) > 0 {
			c.Types = append([]string(nil), md.Types...)
		}
		c.BasePower = md.Power
		c.BaseToughness = md.Toughness
		if len(md.Colors) > 0 {
			c.Colors = append([]string(nil), md.Colors...)
		}
		c.CMC = md.CMC
		c.TypeLine = md.TypeLine
	}

	// ETB-choice P/T default so 0/0 "as ~ enters, choose" creatures
	// survive SBA §704.5f (Primal Plasma class).
	isCreature := false
	for _, t := range c.Types {
		if t == "creature" {
			isCreature = true
			break
		}
	}
	if isCreature && c.BasePower == 0 && c.BaseToughness == 0 {
		ot := gameengine.OracleTextLower(c)
		if gameengine.HasETBChoicePatternExported(ot) {
			c.BasePower = 3
			c.BaseToughness = 3
		}
	}

	return c
}
