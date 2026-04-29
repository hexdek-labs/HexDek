// mtgsquad-heimdall — Heimdall game analyst + spectator for the Go rules engine.
//
// TWO MODES:
//
//   1. SPECTATOR (default): Runs a single game with full event streaming,
//      invariant checking, and pause-on-anomaly. Every game event prints
//      to stdout with full context. On anomaly (invariant violation or
//      crash): PAUSE, print full game state, wait for user input.
//
//   2. ANALYTICS (--analyze): Runs N games and produces a deep analytics
//      report. Per-card, per-player, per-game analysis answering "WHY did
//      this deck win?" Output as markdown report.
//
// Usage:
//
//	# Spectator mode:
//	go run ./cmd/mtgsquad-heimdall/ \
//	    --decks data/decks/cage_match \
//	    --seed 42 \
//	    [--pause-on-anomaly] [--seats 4] [--max-turns 80] [--verbose]
//
//	# Analytics mode:
//	go run ./cmd/mtgsquad-heimdall/ \
//	    --analyze --games 50 --decks data/decks/cage_match \
//	    --hat poker --report data/rules/HEIMDALL_ANALYSIS.md \
//	    [--top-cards 20] [--seats 4]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

var (
	pauseOnAnomaly bool
	verbose        bool
	aborted        bool
)

// colorize helpers for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[37m"
	colorBold   = "\033[1m"
)

func main() {
	var (
		decksFlag  = flag.String("decks", "", "comma-separated deck paths or directory (required)")
		seedFlag   = flag.Int64("seed", 42, "game RNG seed")
		seatsFlag  = flag.Int("seats", 4, "seats per game")
		maxTurns   = flag.Int("max-turns", 80, "max turns")
		astPath    = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset path")
		oraclePath = flag.String("oracle", "data/rules/oracle-cards.json", "oracle-cards.json path")
		pauseFlag  = flag.Bool("pause-on-anomaly", true, "pause on invariant violations")
		verboseFlg = flag.Bool("verbose", false, "show all events (not just key ones)")

		// Analytics mode flags.
		analyzeFlag = flag.Bool("analyze", false, "run analytics mode (not spectator)")
		gamesFlag   = flag.Int("games", 50, "number of games to analyze (analytics mode)")
		hatFlag     = flag.String("hat", "greedy", "hat type: greedy, poker, octo")
		topCardsN   = flag.Int("top-cards", 10, "show top N cards by win contribution")
		reportFlag  = flag.String("report", "", "write analytics report to this path")
		workersFlag = flag.Int("workers", 0, "worker goroutines (0 = NumCPU)")
	)
	flag.Parse()

	if *decksFlag == "" {
		log.Fatal("--decks is required")
	}

	if *analyzeFlag {
		runAnalytics(*decksFlag, *astPath, *oraclePath, *seedFlag, *seatsFlag,
			*maxTurns, *gamesFlag, *hatFlag, *topCardsN, *reportFlag, *workersFlag)
	} else {
		pauseOnAnomaly = *pauseFlag
		verbose = *verboseFlg
		runSpectator(*decksFlag, *astPath, *oraclePath, *seedFlag, *seatsFlag, *maxTurns)
	}
}

// ---------------------------------------------------------------------------
// ANALYTICS MODE
// ---------------------------------------------------------------------------

func runAnalytics(decksPath, astPath, oraclePath string, seed int64, seats, maxTurns, nGames int, hatKind string, topCards int, reportPath string, workers int) {
	fmt.Printf("%s%s=== HEIMDALL ANALYTICS ENGINE ===%s\n", colorBold, colorCyan, colorReset)
	fmt.Println("Deep per-card, per-player, per-game analysis")
	fmt.Println()

	// Load corpus.
	fmt.Print("Loading card corpus... ")
	t0 := time.Now()
	corpus, err := astload.Load(astPath)
	if err != nil {
		log.Fatalf("astload: %v", err)
	}
	meta, err := deckparser.LoadMetaFromJSONL(astPath)
	if err != nil {
		log.Fatalf("meta: %v", err)
	}
	if oraclePath != "" {
		meta.SupplementWithOracleJSON(oraclePath)
	}
	fmt.Printf("%d cards in %s\n", corpus.Count(), time.Since(t0).Round(time.Millisecond))

	// Load decks.
	deckPaths := resolveDeckPaths(decksPath)
	if len(deckPaths) < seats {
		log.Fatalf("need %d decks for %d seats (found %d)", seats, seats, len(deckPaths))
	}
	decks := make([]*deckparser.TournamentDeck, len(deckPaths))
	for i, p := range deckPaths {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			log.Fatalf("deck %s: %v", p, err)
		}
		decks[i] = d
		fmt.Printf("  Deck %d: %s%s%s (%d cards)\n",
			i, colorCyan, d.CommanderName, colorReset, len(d.Library)+len(d.CommanderCards))
	}
	fmt.Println()

	// Hat factory.
	var factory tournament.HatFactory
	switch strings.ToLower(hatKind) {
	case "greedy":
		factory = func() gameengine.Hat { return &hat.GreedyHat{} }
	case "poker":
		factory = func() gameengine.Hat { return hat.NewPokerHat() }
	case "octo":
		factory = func() gameengine.Hat { return &hat.OctoHat{} }
	default:
		log.Fatalf("unknown hat %q (want greedy|poker|octo)", hatKind)
	}

	fmt.Printf("Running %d games with %s hat, %d seats...\n", nGames, hatKind, seats)

	cfg := tournament.TournamentConfig{
		Decks:            decks,
		NSeats:           seats,
		NGames:           nGames,
		Seed:             seed,
		HatFactories:     []tournament.HatFactory{factory},
		Workers:          workers,
		CommanderMode:    true,
		AnalyticsEnabled: true,
		MaxTurnsPerGame:  maxTurns,
	}

	result, err := tournament.Run(cfg)
	if err != nil {
		log.Fatalf("tournament: %v", err)
	}

	// Print summary dashboard.
	result.PrintDashboard(true)

	// Print analytics dashboard.
	fmt.Printf("\n%s%s=== HEIMDALL DEEP ANALYTICS ===%s\n", colorBold, colorCyan, colorReset)

	// Win conditions.
	printWinConditions(result)

	// Top cards.
	printTopCards(result, topCards)

	// Dead cards.
	printDeadCards(result, topCards)

	// Kill shot cards.
	printKillShots(result, topCards)

	// Per-commander summary.
	printCommanderSummary(result)

	// Write report.
	if reportPath != "" {
		ar := &analytics.AnalyticsReport{
			Analyses:       result.Analyses,
			CardRankings:   result.CardRankings,
			MatchupDetails: result.MatchupDetails,
			CommanderNames: result.CommanderNames,
			TotalGames:     result.Games,
			Duration:       result.Duration,
		}
		if err := ar.WriteMarkdown(reportPath); err != nil {
			log.Fatalf("write report: %v", err)
		}
		fmt.Printf("\n%sAnalytics report written to %s%s\n", colorGreen, reportPath, colorReset)
	}
}

func printWinConditions(r *tournament.TournamentResult) {
	if len(r.Analyses) == 0 {
		return
	}

	condCounts := make(map[string]int)
	for _, ga := range r.Analyses {
		if ga != nil {
			condCounts[ga.WinCondition]++
		}
	}

	fmt.Println()
	fmt.Println("WIN CONDITIONS:")
	total := max1(r.Games)
	for cond, count := range condCounts {
		pct := 100.0 * float64(count) / float64(total)
		fmt.Printf("  %-20s %3.0f%%  (%d/%d)\n", cond+":", pct, count, total)
	}
}

func printTopCards(r *tournament.TournamentResult, n int) {
	if len(r.CardRankings) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("MVP CARDS (by games won):")
	limit := n
	if limit > len(r.CardRankings) {
		limit = len(r.CardRankings)
	}
	for i := 0; i < limit; i++ {
		cr := &r.CardRankings[i]
		fmt.Printf("  %2d. %-35s won=%d  cast=%d  avg-turn=%.1f  avg-dmg=%.1f  kill-shot=%.0f%%\n",
			i+1, cr.Name, cr.GamesWon, cr.TimesCast, cr.AvgTurnCast, cr.AvgDamageDealt, cr.KillShotRate*100)
	}
}

func printDeadCards(r *tournament.TournamentResult, n int) {
	if len(r.CardRankings) == 0 {
		return
	}

	// Filter + sort by dead rate.
	minGames := max1(r.Games / 10)
	type deadEntry struct {
		name       string
		deadRate   float64
		gamesPlayed int
	}
	var dead []deadEntry
	for _, cr := range r.CardRankings {
		if cr.GamesPlayed >= minGames && cr.DeadInHandRate > 0.1 {
			dead = append(dead, deadEntry{cr.Name, cr.DeadInHandRate, cr.GamesPlayed})
		}
	}
	if len(dead) == 0 {
		return
	}
	// Sort by dead rate desc.
	for i := 0; i < len(dead); i++ {
		for j := i + 1; j < len(dead); j++ {
			if dead[j].deadRate > dead[i].deadRate {
				dead[i], dead[j] = dead[j], dead[i]
			}
		}
	}

	fmt.Println()
	fmt.Println("DEAD CARDS (highest never-cast rate):")
	limit := n
	if limit > len(dead) {
		limit = len(dead)
	}
	for i := 0; i < limit; i++ {
		d := &dead[i]
		fmt.Printf("  %2d. %-35s never-cast=%.0f%%  (in %d games)\n",
			i+1, d.name, d.deadRate*100, d.gamesPlayed)
	}
}

func printKillShots(r *tournament.TournamentResult, n int) {
	if len(r.CardRankings) == 0 {
		return
	}

	type ksEntry struct {
		name     string
		rate     float64
		gamesWon int
	}
	var ks []ksEntry
	for _, cr := range r.CardRankings {
		if cr.KillShotRate > 0 {
			ks = append(ks, ksEntry{cr.Name, cr.KillShotRate, cr.GamesWon})
		}
	}
	if len(ks) == 0 {
		return
	}
	// Sort by rate desc.
	for i := 0; i < len(ks); i++ {
		for j := i + 1; j < len(ks); j++ {
			if ks[j].rate > ks[i].rate {
				ks[i], ks[j] = ks[j], ks[i]
			}
		}
	}

	fmt.Println()
	fmt.Println("KILL SHOT CARDS (delivered the winning blow):")
	limit := n
	if limit > len(ks) {
		limit = len(ks)
	}
	for i := 0; i < limit; i++ {
		k := &ks[i]
		fmt.Printf("  %2d. %-35s kill-shot=%.0f%%  (won %d games)\n",
			i+1, k.name, k.rate*100, k.gamesWon)
	}
}

func printCommanderSummary(r *tournament.TournamentResult) {
	if len(r.Analyses) == 0 {
		return
	}

	type cmdSummary struct {
		name       string
		wins       int
		games      int
		avgDmg     float64
		avgTaken   float64
		avgSpells  float64
		avgRemoval float64
		avgBoard   float64
	}

	byCmd := make(map[string]*cmdSummary)
	for _, ga := range r.Analyses {
		if ga == nil {
			continue
		}
		for i := range ga.Players {
			pa := &ga.Players[i]
			cs, ok := byCmd[pa.CommanderName]
			if !ok {
				cs = &cmdSummary{name: pa.CommanderName}
				byCmd[pa.CommanderName] = cs
			}
			cs.games++
			if pa.Won {
				cs.wins++
			}
			cs.avgDmg += float64(pa.DamageDealt)
			cs.avgTaken += float64(pa.DamageTaken)
			cs.avgSpells += float64(pa.SpellsCast)
			cs.avgRemoval += float64(pa.RemovalCast)
			cs.avgBoard += float64(pa.PeakBoardSize)
		}
	}

	fmt.Println()
	fmt.Println("PER-COMMANDER STATS:")
	for _, name := range r.CommanderNames {
		cs, ok := byCmd[name]
		if !ok || cs.games == 0 {
			continue
		}
		g := float64(cs.games)
		fmt.Printf("  %s:\n", name)
		fmt.Printf("    Win%%=%.1f%%  AvgDmg=%.1f  AvgTaken=%.1f  AvgSpells=%.1f  AvgRemoval=%.1f  PeakBoard=%.1f\n",
			100*float64(cs.wins)/g, cs.avgDmg/g, cs.avgTaken/g, cs.avgSpells/g, cs.avgRemoval/g, cs.avgBoard/g)
	}
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

// ---------------------------------------------------------------------------
// SPECTATOR MODE (original Eye of Sauron)
// ---------------------------------------------------------------------------

func runSpectator(decksPath, astPath, oraclePath string, seed int64, seats, maxTurns int) {
	fmt.Printf("%s%s=== EYE OF SAURON ===%s\n", colorBold, colorRed, colorReset)
	fmt.Println("Full-spectrum game spectator with invariant enforcement")
	fmt.Println()

	// Load corpus.
	fmt.Print("Loading card corpus... ")
	t0 := time.Now()
	corpus, err := astload.Load(astPath)
	if err != nil {
		log.Fatalf("astload: %v", err)
	}
	meta, err := deckparser.LoadMetaFromJSONL(astPath)
	if err != nil {
		log.Fatalf("meta: %v", err)
	}
	if oraclePath != "" {
		meta.SupplementWithOracleJSON(oraclePath)
	}
	fmt.Printf("%d cards in %s\n", corpus.Count(), time.Since(t0).Round(time.Millisecond))

	// Load decks.
	deckPaths := resolveDeckPaths(decksPath)
	if len(deckPaths) < seats {
		log.Fatalf("need %d decks for %d seats (found %d)", seats, seats, len(deckPaths))
	}
	decks := make([]*deckparser.TournamentDeck, len(deckPaths))
	for i, p := range deckPaths {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			log.Fatalf("deck %s: %v", p, err)
		}
		decks[i] = d
		fmt.Printf("  Deck %d: %s%s%s (%d cards)\n",
			i, colorCyan, d.CommanderName, colorReset, len(d.Library)+len(d.CommanderCards))
	}
	fmt.Println()

	nSeats := seats

	// Build game state (mirrors tournament runner logic).
	gameSeed := seed*1000 + 1
	rng := rand.New(rand.NewSource(gameSeed))
	gs := gameengine.NewGameState(nSeats, rng, corpus)

	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		tpl := decks[i%len(decks)]
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
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

	// Attach hats.
	for i := 0; i < nSeats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}

	// Opening hands.
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

	gs.Active = rng.Intn(nSeats)
	gs.Turn = 1

	// Print initial state.
	fmt.Printf("%s--- GAME START (seed %d) ---%s\n", colorBold, seed, colorReset)
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		cmdrNames := make([]string, 0, len(s.CommanderNames))
		cmdrNames = append(cmdrNames, s.CommanderNames...)
		fmt.Printf("  Seat %d: %s%s%s (%d life, %d cards in library)\n",
			i, colorCyan, strings.Join(cmdrNames, " + "), colorReset, s.Life, len(s.Library))
	}
	fmt.Printf("  Active player: Seat %d\n\n", gs.Active)

	// Run invariants on initial state.
	runInvariantsWithPause(gs, "initial state")

	// Track event log position for streaming.
	lastEventIdx := 0

	// Turn loop with round tracking.
	startingSeat := gs.Active
	round := 1
	stallWarned := false
	if gs.Flags == nil {
		gs.Flags = make(map[string]int)
	}
	gs.Flags["round"] = round
	for turn := 1; turn <= maxTurns && !aborted; turn++ {
		gs.Turn = turn
		gs.Flags["round"] = round

		// Print turn header with round notation.
		seat := gs.Seats[gs.Active]
		cmdrStr := ""
		if seat != nil && len(seat.CommanderNames) > 0 {
			cmdrStr = " (" + strings.Join(seat.CommanderNames, " + ") + ")"
		}
		fmt.Printf("\n%s[R%d.%d] Seat %d%s%s\n",
			colorBold, round, gs.Active+1, gs.Active, cmdrStr, colorReset)

		// Take the turn.
		tournament.TakeTurn(gs)

		// Stream new events.
		lastEventIdx = streamEvents(gs, lastEventIdx)

		// Run invariants after the turn.
		runInvariantsWithPause(gs, fmt.Sprintf("after R%d.%d", round, gs.Active+1))

		// Run SBAs.
		gameengine.StateBasedActions(gs)
		lastEventIdx = streamEvents(gs, lastEventIdx)
		runInvariantsWithPause(gs, fmt.Sprintf("after R%d.%d SBA", round, gs.Active+1))

		// Stall warning: like an aircraft stall horn. Fires when the game
		// is past 75% of the turn cap with 2+ players still alive.
		if turn >= maxTurns*3/4 && !stallWarned {
			survivors := 0
			highLife, lowLife := 0, 999999
			for _, s := range gs.Seats {
				if s != nil && !s.Lost {
					survivors++
					if s.Life > highLife {
						highLife = s.Life
					}
					if s.Life < lowLife {
						lowLife = s.Life
					}
				}
			}
			if survivors >= 2 {
				stallWarned = true
				fmt.Printf("\n%s%s⚠ STALL WARNING — Turn %d/%d, %d players alive (life spread %d-%d)%s\n",
					colorBold, colorYellow, turn, maxTurns, survivors, lowLife, highLife, colorReset)
			}
		}

		if gs.CheckEnd() {
			fmt.Printf("\n%s=== GAME OVER ===%s\n", colorBold, colorReset)
			printGameResult(gs)
			break
		}

		prev := gs.Active
		gs.Active = nextLivingSeat(gs)
		if gs.Active <= prev || gs.Active == startingSeat {
			round++
		}
	}

	if !aborted {
		fmt.Printf("\n%s=== FINAL STATE ===%s\n", colorBold, colorReset)
		fmt.Println(gameengine.GameStateSummary(gs))
	}
}

func streamEvents(gs *gameengine.GameState, fromIdx int) int {
	if gs == nil || fromIdx >= len(gs.EventLog) {
		return fromIdx
	}
	for i := fromIdx; i < len(gs.EventLog); i++ {
		ev := &gs.EventLog[i]
		printEvent(gs, ev, i)
	}
	return len(gs.EventLog)
}

func printEvent(gs *gameengine.GameState, ev *gameengine.Event, idx int) {
	// Filter: in non-verbose mode, only show interesting events.
	if !verbose {
		switch ev.Kind {
		case "stack_push", "stack_resolve", "stack_pop",
			"cast_spell", "spell_resolved",
			"combat_damage", "creature_dies", "destroy", "sacrifice",
			"sba_704_5a", "sba_704_5f", "sba_704_5g",
			"turn_start", "game_start",
			"delayed_trigger_fires", "cleanup_loop",
			"create_token", "draw_card",
			"zone_change", "enter_battlefield", "leave_battlefield",
			"player_loss", "loss_prevented",
			"infinite_loop_draw", "sba_cap_hit",
			"sba_cycle_complete":
			// Show these.
		default:
			return
		}
	}

	// Format based on event kind.
	prefix := fmt.Sprintf("  %s[%d]%s ", colorGray, idx, colorReset)

	switch ev.Kind {
	case "turn_start":
		fmt.Printf("%s%s--- Turn %v (Seat %d) ---%s\n",
			prefix, colorBold, detailVal(ev, "turn"), ev.Seat, colorReset)

	case "stack_push":
		fmt.Printf("%s%s-> stack_push:%s %s (stack size: %v)\n",
			prefix, colorBlue, colorReset, ev.Source, detailVal(ev, "stack_size"))

	case "stack_resolve", "stack_pop", "spell_resolved":
		fmt.Printf("%s%s<- %s:%s %s\n",
			prefix, colorGreen, ev.Kind, colorReset, ev.Source)

	case "cast_spell":
		fmt.Printf("%s%sCAST:%s %s (seat %d)\n",
			prefix, colorPurple, colorReset, ev.Source, ev.Seat)

	case "combat_damage":
		fmt.Printf("%s%sCOMBAT DMG:%s %s deals %d to seat %d\n",
			prefix, colorRed, colorReset, ev.Source, ev.Amount, ev.Target)

	case "destroy":
		fmt.Printf("%s%sDESTROY:%s %v (by %s)\n",
			prefix, colorRed, colorReset, detailVal(ev, "target_card"), ev.Source)

	case "sacrifice":
		fmt.Printf("%s%sSACRIFICE:%s %v\n",
			prefix, colorRed, colorReset, detailVal(ev, "target_card"))

	case "creature_dies":
		fmt.Printf("%s%sDIES:%s %s\n",
			prefix, colorRed, colorReset, ev.Source)

	case "sba_704_5a":
		fmt.Printf("%s%sSBA 704.5a:%s seat %d loses (life=%d)\n",
			prefix, colorRed, colorReset, ev.Seat, ev.Amount)

	case "sba_704_5f", "sba_704_5g":
		fmt.Printf("%s%sSBA %s:%s creature dies\n",
			prefix, colorYellow, ev.Kind, colorReset)

	case "create_token":
		fmt.Printf("%s%sTOKEN:%s %s (seat %d)\n",
			prefix, colorCyan, colorReset, ev.Source, ev.Seat)

	case "draw_card":
		fmt.Printf("%s  draw: seat %d draws %s\n", prefix, ev.Seat, ev.Source)

	case "enter_battlefield":
		fmt.Printf("%s%sETB:%s %s (seat %d)\n",
			prefix, colorGreen, colorReset, ev.Source, ev.Seat)

	case "zone_change":
		fmt.Printf("%s  zone: %s -> %v (seat %d)\n",
			prefix, ev.Source, detailVal(ev, "to_zone"), ev.Seat)

	case "player_loss":
		fmt.Printf("%s%sPLAYER LOSS:%s seat %d\n",
			prefix, colorRed, colorReset, ev.Seat)

	case "loss_prevented":
		fmt.Printf("%s%sLOSS PREVENTED:%s seat %d\n",
			prefix, colorYellow, colorReset, ev.Seat)

	case "infinite_loop_draw":
		fmt.Printf("%s%sINFINITE LOOP DRAW%s (rule %v)\n",
			prefix, colorRed, colorReset, detailVal(ev, "rule"))

	case "sba_cap_hit":
		fmt.Printf("%s%sSBA CAP HIT%s (%v passes)\n",
			prefix, colorRed, colorReset, detailVal(ev, "passes"))

	case "sba_cycle_complete":
		fmt.Printf("%s  SBA cycle: %v passes\n", prefix, detailVal(ev, "passes"))

	case "delayed_trigger_fires":
		fmt.Printf("%s%sDELAYED TRIGGER:%s %s\n",
			prefix, colorYellow, colorReset, ev.Source)

	default:
		fmt.Printf("%s  %s: %s (seat %d)\n", prefix, ev.Kind, ev.Source, ev.Seat)
	}
}

func detailVal(ev *gameengine.Event, key string) interface{} {
	if ev.Details == nil {
		return ""
	}
	v, ok := ev.Details[key]
	if !ok {
		return ""
	}
	return v
}

func runInvariantsWithPause(gs *gameengine.GameState, context string) {
	violations := gameengine.RunAllInvariants(gs)
	if len(violations) == 0 {
		return
	}

	for _, v := range violations {
		fmt.Printf("\n%s%s!!! INVARIANT VIOLATION: %s%s\n", colorBold, colorRed, v.Name, colorReset)
		fmt.Printf("  Context: %s\n", context)
		fmt.Printf("  Message: %s\n", v.Message)
	}

	if pauseOnAnomaly {
		fmt.Printf("\n%s--- GAME STATE AT VIOLATION ---%s\n", colorBold, colorReset)
		fmt.Println(gameengine.GameStateSummary(gs))

		fmt.Printf("\n%sRecent events:%s\n", colorBold, colorReset)
		events := gameengine.RecentEvents(gs, 30)
		for _, e := range events {
			fmt.Printf("  %s\n", e)
		}

		fmt.Printf("\n%s[PAUSED] Press Enter to continue, or type 'abort' to stop: %s", colorYellow, colorReset)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if strings.ToLower(input) == "abort" {
				fmt.Println("Aborted by user.")
				aborted = true
			}
		}
	}
}

func printGameResult(gs *gameengine.GameState) {
	if gs == nil {
		return
	}
	winner := -1
	if gs.Flags != nil {
		if w, ok := gs.Flags["winner"]; ok {
			winner = w
		}
	}
	if winner >= 0 && winner < len(gs.Seats) {
		seat := gs.Seats[winner]
		cmdrStr := ""
		if seat != nil && len(seat.CommanderNames) > 0 {
			cmdrStr = " (" + strings.Join(seat.CommanderNames, " + ") + ")"
		}
		fmt.Printf("  Winner: %sSeat %d%s%s (life=%d)\n",
			colorGreen, winner, cmdrStr, colorReset, seat.Life)
	} else {
		fmt.Println("  Result: Draw or no winner determined")
	}
	fmt.Printf("  Turns: %d\n", gs.Turn)
	fmt.Printf("  Events: %d\n", len(gs.EventLog))
}

func nextLivingSeat(gs *gameengine.GameState) int {
	n := len(gs.Seats)
	for offset := 1; offset <= n; offset++ {
		idx := (gs.Active + offset) % n
		if gs.Seats[idx] != nil && !gs.Seats[idx].Lost {
			return idx
		}
	}
	return gs.Active
}

func resolveDeckPaths(input string) []string {
	// Single directory: list all .txt files.
	info, err := os.Stat(input)
	if err == nil && info.IsDir() {
		return listDeckFilesInDir(input)
	}
	// Comma-separated list: each part can be a directory or a file.
	parts := strings.Split(input, ",")
	var paths []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pi, err := os.Stat(p)
		if err == nil && pi.IsDir() {
			paths = append(paths, listDeckFilesInDir(p)...)
		} else {
			paths = append(paths, p)
		}
	}
	return paths
}

func listDeckFilesInDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("read deck dir %s: %v", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths
}
