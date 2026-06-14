package main

// response_behavior_ab_test.go — r63 HAT-RESPONSE empirical A/B.
//
// Plays the SAME set of real (chaos-deck, full-corpus) 4-player Commander
// games twice with real YggdrasilHat seats — once with the r63 response-mana
// funding DISABLED (pre-fix behavior: a defender pays only from an already-
// floated pool, which on an opponent's turn is empty) and once ENABLED — and
// tallies how often the AIs actually cast instant-speed responses (counters /
// reactive removal) on the stack.
//
// This is the ground-truth confirmation that the "hats don't interact"
// behavior gap is real and that the fix closes it. Run:
//
//   HEXDEK_AB=1 go test ./cmd/hexdek-loki/ -run TestResponseBehaviorAB \
//       -count=1 -timeout 600s -v
//
// Skipped by default (needs the gitignored corpora + is a measurement, not a
// regression gate).

import (
	"math/rand"
	"os"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

type respStats struct {
	games        int
	totalCasts   int // Kind=="cast" (any)
	responseCast int // Kind=="cast" with Details["in_response_to"]
	countered    int // stack_resolve with countered==true
	priorityPass int // explicit priority_pass events
}

func tallyResponseEvents(gs *gameengine.GameState, s *respStats) {
	s.games++
	for i := range gs.EventLog {
		e := &gs.EventLog[i]
		switch e.Kind {
		case "cast":
			s.totalCasts++
			if e.Details != nil {
				if _, ok := e.Details["in_response_to"]; ok {
					s.responseCast++
				}
			}
		case "stack_resolve":
			if e.Details != nil {
				if c, ok := e.Details["countered"].(bool); ok && c {
					s.countered++
				}
			}
		case "priority_pass":
			s.priorityPass++
		}
	}
}

// playABGame mirrors playDetGame's setup but seats real YggdrasilHats
// (budget 0 = heuristic, noise 0 = deterministic) and returns the final
// GameState so the caller can read its EventLog. RetainEvents is enabled so
// the full per-decision stream is captured.
func playABGame(dc *detCorpus, gameIdx, nSeats, maxTurns int, masterSeed int64) (gs *gameengine.GameState) {
	deckSeed := masterSeed + int64(gameIdx)*10000 + 1
	shuffleSeed := deckSeed + 7
	deckRng := rand.New(rand.NewSource(deckSeed))
	gameRng := rand.New(rand.NewSource(shuffleSeed))

	defer func() { _ = recover() }()

	chaosDecks := make([]*gameengine.ChaosDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		chaosDecks[i] = gameengine.GenerateChaosDeck(dc.chaos, deckRng)
		if chaosDecks[i] == nil {
			return nil
		}
	}

	gs = gameengine.NewGameState(nSeats, gameRng, dc.ast)
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
		gs.Seats[i].Hat = hat.NewYggdrasilHatWithNoise(nil, 0, 0)
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
	return gs
}

func TestResponseBehaviorAB(t *testing.T) {
	if os.Getenv("HEXDEK_AB") == "" {
		t.Skip("set HEXDEK_AB=1 to run the response-behavior A/B measurement")
	}
	dc := loadDetCorpus(t)
	if dc == nil {
		return
	}
	const nGames = 60
	const nSeats = 4
	const maxTurns = 40
	const masterSeed = 424242

	gameengine.SetResponseTelemetry(true)
	run := func(funding bool) (respStats, gameengine.ResponseTelemetryT) {
		gameengine.ResponseManaFundingEnabled = funding
		gameengine.ResponseTelemetry = gameengine.ResponseTelemetryT{}
		var s respStats
		for g := 0; g < nGames; g++ {
			gs := playABGame(dc, g, nSeats, maxTurns, masterSeed)
			if gs == nil {
				continue
			}
			tallyResponseEvents(gs, &s)
		}
		return s, gameengine.ResponseTelemetry
	}

	before, beforeT := run(false) // pre-r63: defenders can't fund responses
	after, afterT := run(true)    // r63 fix active
	gameengine.ResponseManaFundingEnabled = true

	report := func(label string, s respStats) {
		perGame := 0.0
		pct := 0.0
		if s.games > 0 {
			perGame = float64(s.responseCast) / float64(s.games)
		}
		if s.totalCasts > 0 {
			pct = 100 * float64(s.responseCast) / float64(s.totalCasts)
		}
		t.Logf("%-7s | games=%d totalCasts=%d responseCasts=%d (%.2f/game, %.2f%% of casts) countered=%d",
			label, s.games, s.totalCasts, s.responseCast, perGame, pct, s.countered)
	}
	t.Logf("=== r63 HAT-RESPONSE empirical A/B (%d games × %d seats, %d turns) ===", nGames, nSeats, maxTurns)
	report("BEFORE", before)
	report("AFTER", after)
	dump := func(label string, tm gameengine.ResponseTelemetryT) {
		t.Logf("%s polls=%d counterAvail=%d chosen=%d | rejSorcery=%d rejCounterTL=%d rejTargetIll=%d fundFail=%d CAST=%d",
			label, tm.Polls, tm.CounterAvail, tm.Chosen,
			tm.RejSorcery, tm.RejCounterTL, tm.RejTargetIll, tm.FundFail, tm.Cast)
	}
	dump("OPP(BEFORE):", beforeT)
	dump("OPP(AFTER): ", afterT)
	if after.responseCast > before.responseCast {
		t.Logf("RESULT: response casts rose %d → %d (×%.1f) — fix activates the reactive layer",
			before.responseCast, after.responseCast,
			float64(after.responseCast+1)/float64(before.responseCast+1))
	} else {
		t.Logf("RESULT: no increase (before=%d after=%d) — investigate", before.responseCast, after.responseCast)
	}
}
