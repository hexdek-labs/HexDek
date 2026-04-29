package tournament

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

func loadNDecks(t *testing.T, paths []string, corpus *astload.Corpus, meta *deckparser.MetaDB) []*deckparser.TournamentDeck {
	t.Helper()
	decks := make([]*deckparser.TournamentDeck, 0, len(paths))
	for _, p := range paths {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}
	return decks
}

func TestMCTSDiagnostic_EvalMode(t *testing.T) {
	runMCTSDiag(t, "EVALUATOR 1v1 (Budget=50)", 50, false, 2)
}

func TestMCTSDiagnostic_RolloutMode(t *testing.T) {
	runMCTSDiag(t, "ROLLOUT 1v1 (Budget=200)", 200, true, 2)
}

func TestMCTSDiagnostic_4Player_EvalMode(t *testing.T) {
	runMCTSDiag(t, "EVALUATOR 4-PLAYER (Budget=50)", 50, false, 4)
}

func TestMCTSDiagnostic_4Player_RolloutMode(t *testing.T) {
	runMCTSDiag(t, "ROLLOUT 4-PLAYER (Budget=200)", 200, true, 4)
}

func TestMCTSDiagnostic_4Player_HighPower(t *testing.T) {
	runMCTSDiag(t, "ROLLOUT 4-PLAYER HIGH-POWER (Budget=500)", 500, true, 4)
}

func TestYggdrasil_4Player_EvalMode(t *testing.T) {
	runYggdrasilDiag(t, "YGGDRASIL 4-PLAYER (Budget=50)", 50, false, 4)
}

func TestYggdrasil_4Player_RolloutMode(t *testing.T) {
	runYggdrasilDiag(t, "YGGDRASIL 4-PLAYER ROLLOUT (Budget=200)", 200, true, 4)
}

func TestYggdrasil_1v1(t *testing.T) {
	runYggdrasilDiag(t, "YGGDRASIL 1v1 (Budget=50)", 50, false, 2)
}

func runYggdrasilDiag(t *testing.T, label string, budget int, wantRollout bool, nSeats int) {
	corpus, meta := loadCorpus(t)
	paths := findAllDecks(t, nSeats)
	if len(paths) < nSeats {
		t.Skipf("need at least %d decks, found %d", nSeats, len(paths))
	}
	decks := loadNDecks(t, paths[:nSeats], corpus, meta)

	logs := make([]*[]string, nSeats)
	for i := range logs {
		log := make([]string, 0, 256)
		logs[i] = &log
	}

	strategies := make([]*hat.StrategyProfile, nSeats)
	for i := 0; i < nSeats; i++ {
		strategies[i] = hat.LoadStrategyFromFreya(paths[i])
		if strategies[i] != nil {
			t.Logf("  Seat %d: loaded strategy — %s (bracket %d, %d win lines, %d value keys)",
				i, strategies[i].Archetype, strategies[i].Bracket,
				len(strategies[i].ComboPieces), len(strategies[i].ValueEngineKeys))
		}
	}

	turnRunner := TurnRunnerForRollout()
	factories := make([]HatFactory, nSeats)
	for i := 0; i < nSeats; i++ {
		idx := i
		factories[i] = func() gameengine.Hat {
			yh := hat.NewYggdrasilHat(strategies[idx], budget)
			yh.DecisionLog = logs[idx]
			if wantRollout {
				yh.TurnRunner = turnRunner
			}
			return yh
		}
	}

	cfg := TournamentConfig{
		Decks:            decks,
		NSeats:           nSeats,
		NGames:           3,
		Seed:             42,
		HatFactories:     factories,
		Workers:          1,
		CommanderMode:    true,
		AnalyticsEnabled: true,
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n═══════════════════════════════════════\n"))
	sb.WriteString(fmt.Sprintf(" %s — %d games, %d crashes\n", label, result.Games, result.Crashes))
	sb.WriteString(fmt.Sprintf("═══════════════════════════════════════\n"))

	sb.WriteString(fmt.Sprintf("\nCommanders:\n"))
	for i, d := range decks {
		sb.WriteString(fmt.Sprintf("  Seat %d: %s\n", i, d.CommanderName))
	}
	sb.WriteString(fmt.Sprintf("\nWins: "))
	for name, w := range result.WinsByCommander {
		sb.WriteString(fmt.Sprintf("%s=%d  ", name, w))
	}
	avgRounds := result.AvgTurns / float64(nSeats)
	sb.WriteString(fmt.Sprintf("\nAvg turns: %.1f (≈%.1f rounds)\n", result.AvgTurns, avgRounds))

	if len(result.ELO) > 0 {
		sb.WriteString(fmt.Sprintf("\nELO:\n"))
		for _, e := range result.ELO {
			sb.WriteString(fmt.Sprintf("  %-35s %.0f (%d games)\n", e.Commander, e.Rating, e.Games))
		}
	}

	// Heimdall analytics report.
	if len(result.Analyses) > 0 {
		report := &analytics.AnalyticsReport{
			Analyses:       result.Analyses,
			CardRankings:   result.CardRankings,
			MatchupDetails: result.MatchupDetails,
			CommanderNames: result.CommanderNames,
			TotalGames:     result.Games,
			Duration:       result.Duration,
		}
		sb.WriteString("\n")
		report.WriteWinConditionsTo(&sb)
		report.WriteMVPCardsTo(&sb, 10)
		report.WriteDeadCardsTo(&sb, 10)
		report.WriteKillShotCardsTo(&sb, 5)
		report.WriteManaEfficiencyTo(&sb)
		report.WriteTempoAnalysisTo(&sb)
		report.WriteCommanderBreakdownTo(&sb)
		report.WriteMissedCombosTo(&sb)
	}

	for i, log := range logs {
		entries := *log
		if len(entries) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n--- Seat %d (%s) decisions [%d total] ---\n",
			i, decks[i%len(decks)].CommanderName, len(entries)))
		cap := 80
		if len(entries) < cap {
			cap = len(entries)
		}
		for _, line := range entries[:cap] {
			sb.WriteString(line + "\n")
		}
		if len(entries) > cap {
			sb.WriteString(fmt.Sprintf("  ... +%d more decisions\n", len(entries)-cap))
		}
	}

	t.Log(sb.String())
}

func TestMCTSDiagnostic_4Player_TokenTrace(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findAllDecks(t, 4)
	if len(paths) < 4 {
		t.Skip("need 4 decks")
	}
	decks := loadNDecks(t, paths[:4], corpus, meta)

	nSeats := 4
	rng := rand.New(rand.NewSource(42))
	gs := gameengine.NewGameState(nSeats, rng, nil)

	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		lib := deckparser.CloneLibrary(decks[i].Library)
		cmdrs := deckparser.CloneCards(decks[i].CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		rng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		commanderDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: cmdrs,
			Library:        lib,
		}
	}
	gameengine.SetupCommanderGame(gs, commanderDecks)
	for i := 0; i < nSeats; i++ {
		gs.Seats[i].Hat = hat.NewPokerHat()
		RunLondonMulligan(gs, i)
	}
	gs.Active = 0
	gs.Turn = 1

	var sb strings.Builder
	sb.WriteString("\n4-PLAYER TOKEN TRACE\n")
	for i, d := range decks {
		sb.WriteString(fmt.Sprintf("  Seat %d: %s\n", i, d.CommanderName))
	}

	for turn := 1; turn <= 20 && !gs.CheckEnd(); turn++ {
		gs.Turn = turn
		takeTurn(gs)
		gameengine.StateBasedActions(gs)

		active := gs.Active
		bf := gs.Seats[active].Battlefield
		tokens := 0
		for _, p := range bf {
			if p != nil && p.Card != nil {
				for _, tp := range p.Card.Types {
					if tp == "token" {
						tokens++
						break
					}
				}
			}
		}
		if tokens > 0 || len(bf) > 5 {
			sb.WriteString(fmt.Sprintf("  [T%d] Seat %d: %d permanents, %d tokens\n", turn, active, len(bf), tokens))
		}

		// Advance active.
		for offset := 1; offset <= nSeats; offset++ {
			next := (gs.Active + offset) % nSeats
			if s := gs.Seats[next]; s != nil && !s.Lost {
				gs.Active = next
				break
			}
		}
	}

	// Dump create_token events by source.
	tokenSources := map[string]int{}
	triggerSources := map[string]int{}
	for _, ev := range gs.EventLog {
		if ev.Kind == "create_token" {
			key := fmt.Sprintf("seat=%d src=%s amount=%d", ev.Seat, ev.Source, ev.Amount)
			tokenSources[key]++
		}
		if ev.Kind == "trigger_fires" {
			src := ev.Source
			event := ""
			if ev.Details != nil {
				if e, ok := ev.Details["event"].(string); ok {
					event = e
				}
			}
			key := fmt.Sprintf("seat=%d %s [%s]", ev.Seat, src, event)
			triggerSources[key]++
		}
	}

	sb.WriteString("\n--- Token Creation Events ---\n")
	for k, n := range tokenSources {
		sb.WriteString(fmt.Sprintf("  [%dx] %s\n", n, k))
	}
	sb.WriteString("\n--- Top Trigger Fires (3+) ---\n")
	for k, n := range triggerSources {
		if n >= 3 {
			sb.WriteString(fmt.Sprintf("  [%dx] %s\n", n, k))
		}
	}

	t.Log(sb.String())
}

func findAllDecks(t *testing.T, minN int) []string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 6; i++ {
		base := filepath.Join(dir, "data", "decks")
		if _, err := os.Stat(base); err == nil {
			var all []string
			for _, sub := range []string{"personal", "hex", "7174n1c", "josh", "lyon", "rudd", "underwood"} {
				if paths, err := deckparser.ListDeckFiles(filepath.Join(base, sub)); err == nil {
					all = append(all, paths...)
				}
			}
			if len(all) >= minN {
				return all
			}
		}
		dir = filepath.Dir(dir)
	}
	t.Skipf("need at least %d decks", minN)
	return nil
}

func runMCTSDiag(t *testing.T, label string, budget int, wantRollout bool, nSeats int) {
	corpus, meta := loadCorpus(t)
	paths := findAllDecks(t, nSeats)
	if len(paths) < nSeats {
		t.Skipf("need at least %d decks, found %d", nSeats, len(paths))
	}
	decks := loadNDecks(t, paths[:nSeats], corpus, meta)

	// Collect decision logs per seat.
	logs := make([]*[]string, nSeats)
	for i := range logs {
		log := make([]string, 0, 256)
		logs[i] = &log
	}

	turnRunner := TurnRunnerForRollout()
	factories := make([]HatFactory, nSeats)
	for i := 0; i < nSeats; i++ {
		idx := i
		factories[i] = func() gameengine.Hat {
			inner := hat.NewPokerHat()
			mh := hat.NewMCTSHat(inner, nil, budget)
			mh.DecisionLog = logs[idx]
			if wantRollout {
				mh.TurnRunner = turnRunner
			}
			return mh
		}
	}

	cfg := TournamentConfig{
		Decks:         decks,
		NSeats:        nSeats,
		NGames:        3,
		Seed:          42,
		HatFactories:  factories,
		Workers:       1,
		CommanderMode: true,
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Build the report.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n═══════════════════════════════════════\n"))
	sb.WriteString(fmt.Sprintf(" %s — %d games, %d crashes\n", label, result.Games, result.Crashes))
	sb.WriteString(fmt.Sprintf("═══════════════════════════════════════\n"))

	sb.WriteString(fmt.Sprintf("\nCommanders:\n"))
	for i, d := range decks {
		sb.WriteString(fmt.Sprintf("  Seat %d: %s\n", i, d.CommanderName))
	}
	sb.WriteString(fmt.Sprintf("\nWins: "))
	for name, w := range result.WinsByCommander {
		sb.WriteString(fmt.Sprintf("%s=%d  ", name, w))
	}
	avgRounds := result.AvgTurns / float64(nSeats)
	sb.WriteString(fmt.Sprintf("\nAvg turns: %.1f (≈%.1f rounds)\n", result.AvgTurns, avgRounds))

	if len(result.ELO) > 0 {
		sb.WriteString(fmt.Sprintf("\nELO:\n"))
		for _, e := range result.ELO {
			sb.WriteString(fmt.Sprintf("  %-35s %.0f (%d games)\n", e.Commander, e.Rating, e.Games))
		}
	}

	// Print decision logs (capped).
	for i, log := range logs {
		entries := *log
		if len(entries) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n--- Seat %d (%s) decisions [%d total] ---\n",
			i, decks[i%len(decks)].CommanderName, len(entries)))
		cap := 80
		if len(entries) < cap {
			cap = len(entries)
		}
		for _, line := range entries[:cap] {
			sb.WriteString(line + "\n")
		}
		if len(entries) > cap {
			sb.WriteString(fmt.Sprintf("  ... +%d more decisions\n", len(entries)-cap))
		}
	}

	t.Log(sb.String())
}
