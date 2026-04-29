package tournament

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// findAllDecksExtended is like findAllDecks but also includes test/,
// benched/, cage_match/, and imported/ subdirectories.
func findAllDecksExtended(t *testing.T, minN int) []string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 6; i++ {
		base := filepath.Join(dir, "data", "decks")
		if _, err := os.Stat(base); err == nil {
			var all []string
			for _, sub := range []string{
				"personal", "hex", "7174n1c", "josh", "lyon", "rudd", "underwood",
				"test", "benched", "cage_match", "imported",
			} {
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

func TestStress_MultiPod(t *testing.T) {
	corpus, meta := loadCorpus(t)
	allPaths := findAllDecks(t, 4)
	if len(allPaths) < 8 {
		t.Skipf("need at least 8 decks, found %d", len(allPaths))
	}

	seeds := []int64{11, 22, 33, 44, 55}
	nSeats := 4
	nGames := 10

	var sb strings.Builder

	for podIdx, seed := range seeds {
		rng := rand.New(rand.NewSource(seed))
		shuffled := make([]string, len(allPaths))
		copy(shuffled, allPaths)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		paths := shuffled[:nSeats]
		decks := loadNDecks(t, paths, corpus, meta)

		logs := make([]*[]string, nSeats)
		for i := range logs {
			log := make([]string, 0, 512)
			logs[i] = &log
		}

		strategies := make([]*hat.StrategyProfile, nSeats)
		for i := 0; i < nSeats; i++ {
			strategies[i] = hat.LoadStrategyFromFreya(paths[i])
		}

		factories := make([]HatFactory, nSeats)
		for i := 0; i < nSeats; i++ {
			idx := i
			factories[i] = func() gameengine.Hat {
				yh := hat.NewYggdrasilHat(strategies[idx], 50)
				yh.DecisionLog = logs[idx]
				return yh
			}
		}

		cfg := TournamentConfig{
			Decks:            decks,
			NSeats:           nSeats,
			NGames:           nGames,
			Seed:             seed,
			HatFactories:     factories,
			Workers:          1,
			CommanderMode:    true,
			AnalyticsEnabled: true,
		}

		result, err := Run(cfg)
		if err != nil {
			t.Fatalf("pod %d: run: %v", podIdx, err)
		}

		sb.WriteString(fmt.Sprintf("\n╔═══════════════════════════════════════════════════════╗\n"))
		sb.WriteString(fmt.Sprintf("║ POD %d — %d games, %d crashes, seed=%d\n", podIdx+1, result.Games, result.Crashes, seed))
		sb.WriteString(fmt.Sprintf("╚═══════════════════════════════════════════════════════╝\n"))

		for i, d := range decks {
			arch := "unknown"
			bracket := 0
			combos := 0
			valKeys := 0
			if strategies[i] != nil {
				arch = strategies[i].Archetype
				bracket = strategies[i].Bracket
				combos = len(strategies[i].ComboPieces)
				valKeys = len(strategies[i].ValueEngineKeys)
			}
			sb.WriteString(fmt.Sprintf("  Seat %d: %-35s [%s b%d] %d combos, %d value keys — %s\n",
				i+1, d.CommanderName, arch, bracket, combos, valKeys, filepath.Base(paths[i])))
		}

		sb.WriteString(fmt.Sprintf("\nWins: "))
		for name, w := range result.WinsByCommander {
			sb.WriteString(fmt.Sprintf("%s=%d  ", name, w))
		}
		avgRounds := result.AvgTurns / float64(nSeats)
		sb.WriteString(fmt.Sprintf("\nAvg turns: %.1f (≈%.1f rounds)\n", result.AvgTurns, avgRounds))

		if len(result.ELO) > 0 {
			sb.WriteString("ELO: ")
			for _, e := range result.ELO {
				sb.WriteString(fmt.Sprintf("%s=%.0f  ", e.Commander, e.Rating))
			}
			sb.WriteString("\n")
		}

		// Heimdall report.
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
			report.WriteMVPCardsTo(&sb, 8)
			report.WriteKillShotCardsTo(&sb, 5)
			report.WriteManaEfficiencyTo(&sb)
			report.WriteCommanderBreakdownTo(&sb)
			report.WriteMissedCombosTo(&sb)
		}

		// Per-seat decision analysis.
		for i, log := range logs {
			entries := *log
			if len(entries) == 0 {
				continue
			}

			castEvals := 0
			attacks := 0
			passes := 0
			holds := 0
			castNames := make(map[string]int)
			aheadStance := 0
			behindStance := 0
			neutralStance := 0

			for _, line := range entries {
				if strings.Contains(line, "] CAST eval") {
					castEvals++
				}
				if strings.Contains(line, "] ATTACK") {
					attacks++
				}
				if strings.Contains(line, "→ PASS") {
					passes++
				}
				if strings.Contains(line, "HOLD (below threshold)") {
					holds++
				}
				if strings.Contains(line, "AHEAD→aggressive") {
					aheadStance++
				}
				if strings.Contains(line, "BEHIND→selective") {
					behindStance++
				}
				if strings.Contains(line, "stance=neutral") {
					neutralStance++
				}
				if idx := strings.Index(line, "→ CAST "); idx >= 0 {
					rest := line[idx+len("→ CAST "):]
					if paren := strings.Index(rest, " ("); paren > 0 {
						castNames[rest[:paren]]++
					}
				}
			}

			passRate := 0.0
			if castEvals > 0 {
				passRate = float64(passes) / float64(castEvals) * 100
			}
			holdRate := 0.0
			totalAttackCreatures := holds
			for _, line := range entries {
				if strings.Contains(line, "ATTACK") && !strings.Contains(line, "] ATTACK") {
					totalAttackCreatures++
				}
			}

			sb.WriteString(fmt.Sprintf("\n  [Seat %d] %s — %d decisions\n", i+1, decks[i].CommanderName, len(entries)))
			sb.WriteString(fmt.Sprintf("    Casts: %d evals, %d passes (%.0f%%) | Attacks: %d phases, %d holds\n",
				castEvals, passes, passRate, attacks, holds))
			sb.WriteString(fmt.Sprintf("    Stance: AHEAD=%d BEHIND=%d neutral=%d\n",
				aheadStance, behindStance, neutralStance))

			// Repeated casts (potential bugs).
			type cc struct {
				name  string
				count int
			}
			var sorted []cc
			for name, count := range castNames {
				sorted = append(sorted, cc{name, count})
			}
			sort.Slice(sorted, func(a, b int) bool { return sorted[a].count > sorted[b].count })

			// Flag suspicious repeats: >2x per game average.
			for _, c := range sorted {
				perGame := float64(c.count) / float64(nGames)
				if perGame > 2.0 {
					sb.WriteString(fmt.Sprintf("    ⚠ %dx %s (%.1f/game)\n", c.count, c.name, perGame))
				}
			}

			_ = holdRate
		}
	}

	t.Log(sb.String())
}

func TestStress_ArchetypeDiversity(t *testing.T) {
	corpus, meta := loadCorpus(t)

	// Hand-picked decks: one per archetype.
	deckNames := []string{
		"golbez_crystal_collector",     // combo
		"tergrid_cleo_b4_god_of_fright", // stax
		"hex_ninjas_b3_yuriko",         // tribal
		"sin_landreanimator_b3_spiras_punishment", // reanimator
	}
	allPaths := findAllDecks(t, 4)
	paths := make([]string, 0, len(deckNames))
	for _, want := range deckNames {
		for _, p := range allPaths {
			if strings.Contains(p, want) {
				paths = append(paths, p)
				break
			}
		}
	}
	if len(paths) < 4 {
		t.Skipf("need 4 specific decks, found %d", len(paths))
	}

	decks := loadNDecks(t, paths, corpus, meta)
	nSeats := 4
	nGames := 15

	strategies := make([]*hat.StrategyProfile, nSeats)
	for i := 0; i < nSeats; i++ {
		strategies[i] = hat.LoadStrategyFromFreya(paths[i])
	}
	factories := make([]HatFactory, nSeats)

	seeds := []int64{99, 42, 777}
	for _, seed := range seeds {
	logs := make([]*[]string, nSeats)
	for i := range logs {
		log := make([]string, 0, 1024)
		logs[i] = &log
	}
	for i := 0; i < nSeats; i++ {
		idx := i
		factories[i] = func() gameengine.Hat {
			yh := hat.NewYggdrasilHat(strategies[idx], 50)
			yh.DecisionLog = logs[idx]
			return yh
		}
	}

	cfg := TournamentConfig{
		Decks:            decks,
		NSeats:           nSeats,
		NGames:           nGames,
		Seed:             seed,
		HatFactories:     factories,
		Workers:          1,
		CommanderMode:    true,
		AnalyticsEnabled: true,
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("seed %d: run: %v", seed, err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n╔═══════════════════════════════════════════════════════════╗\n"))
	sb.WriteString(fmt.Sprintf("║ ARCHETYPE DIVERSITY — %d games, %d crashes, seed=%d\n", result.Games, result.Crashes, seed))
	sb.WriteString(fmt.Sprintf("╚═══════════════════════════════════════════════════════════╝\n"))

	for i, d := range decks {
		arch := "unknown"
		bracket := 0
		combos := 0
		valKeys := 0
		if strategies[i] != nil {
			arch = strategies[i].Archetype
			bracket = strategies[i].Bracket
			combos = len(strategies[i].ComboPieces)
			valKeys = len(strategies[i].ValueEngineKeys)
		}
		sb.WriteString(fmt.Sprintf("  Seat %d: %-35s [%s b%d] %d combos, %d value keys\n",
			i+1, d.CommanderName, arch, bracket, combos, valKeys))
	}

	sb.WriteString(fmt.Sprintf("\nWins: "))
	for name, w := range result.WinsByCommander {
		sb.WriteString(fmt.Sprintf("%s=%d  ", name, w))
	}
	avgRounds := result.AvgTurns / float64(nSeats)
	sb.WriteString(fmt.Sprintf("\nAvg turns: %.1f (≈%.1f rounds)\n", result.AvgTurns, avgRounds))

	if len(result.ELO) > 0 {
		sb.WriteString("ELO: ")
		for _, e := range result.ELO {
			sb.WriteString(fmt.Sprintf("%s=%.0f  ", e.Commander, e.Rating))
		}
		sb.WriteString("\n")
	}

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
		report.WriteMVPCardsTo(&sb, 8)
		report.WriteKillShotCardsTo(&sb, 5)
		report.WriteManaEfficiencyTo(&sb)
		report.WriteCommanderBreakdownTo(&sb)
		report.WriteMissedCombosTo(&sb)
	}

	for i, log := range logs {
		entries := *log
		if len(entries) == 0 {
			continue
		}
		castEvals := 0
		passes := 0
		holds := 0
		attacks := 0
		comboBoosts := 0
		passBoosts := 0
		for _, line := range entries {
			if strings.Contains(line, "] CAST eval") {
				castEvals++
			}
			if strings.Contains(line, "→ PASS") {
				passes++
			}
			if strings.Contains(line, "HOLD (below threshold)") {
				holds++
			}
			if strings.Contains(line, "] ATTACK") {
				attacks++
			}
			if strings.Contains(line, "boost=0.") && !strings.Contains(line, "boost=0.00)") {
				passBoosts++
			}
			if strings.Contains(line, "heuristic=1.") || strings.Contains(line, "heuristic=0.8") {
				comboBoosts++
			}
		}
		passRate := 0.0
		if castEvals > 0 {
			passRate = float64(passes) / float64(castEvals) * 100
		}
		sb.WriteString(fmt.Sprintf("\n  [Seat %d] %s — %d decisions\n", i+1, decks[i].CommanderName, len(entries)))
		sb.WriteString(fmt.Sprintf("    Casts: %d evals, %d passes (%.0f%%) | Attacks: %d, Holds: %d\n",
			castEvals, passes, passRate, attacks, holds))
		sb.WriteString(fmt.Sprintf("    Combo-boosted candidates: %d | Pass-boosted turns: %d\n",
			comboBoosts, passBoosts))
	}

	t.Log(sb.String())
	} // end seed loop
}

func TestStress_TergridRotation(t *testing.T) {
	corpus, meta := loadCorpus(t)
	allPaths := findAllDecks(t, 4)

	var tergridPath string
	var otherPaths []string
	for _, p := range allPaths {
		if strings.Contains(p, "tergrid") {
			tergridPath = p
		} else {
			otherPaths = append(otherPaths, p)
		}
	}
	if tergridPath == "" {
		t.Skip("tergrid deck not found")
	}
	if len(otherPaths) < 3 {
		t.Skipf("need at least 3 other decks, found %d", len(otherPaths))
	}

	nSeats := 4
	nGames := 10
	seeds := []int64{100, 200, 300, 400, 500}
	tergridWins := 0
	totalGames := 0

	var sb strings.Builder
	for podIdx, seed := range seeds {
		rng := rand.New(rand.NewSource(seed))
		shuffled := make([]string, len(otherPaths))
		copy(shuffled, otherPaths)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		paths := []string{tergridPath}
		paths = append(paths, shuffled[:3]...)
		decks := loadNDecks(t, paths, corpus, meta)

		strategies := make([]*hat.StrategyProfile, nSeats)
		for i := 0; i < nSeats; i++ {
			strategies[i] = hat.LoadStrategyFromFreya(paths[i])
		}
		factories := make([]HatFactory, nSeats)
		for i := 0; i < nSeats; i++ {
			idx := i
			factories[i] = func() gameengine.Hat {
				return hat.NewYggdrasilHat(strategies[idx], 50)
			}
		}

		cfg := TournamentConfig{
			Decks:         decks,
			NSeats:        nSeats,
			NGames:        nGames,
			Seed:          seed,
			HatFactories:  factories,
			Workers:       1,
			CommanderMode: true,
		}
		result, err := Run(cfg)
		if err != nil {
			t.Fatalf("pod %d: %v", podIdx, err)
		}
		totalGames += result.Games

		sb.WriteString(fmt.Sprintf("\n── Pod %d (seed=%d) ──\n", podIdx+1, seed))
		for i, d := range decks {
			arch := "?"
			if strategies[i] != nil {
				arch = strategies[i].Archetype
			}
			sb.WriteString(fmt.Sprintf("  Seat %d: %-35s [%s] — %s\n",
				i+1, d.CommanderName, arch, filepath.Base(paths[i])))
		}
		sb.WriteString("  Wins: ")
		for name, w := range result.WinsByCommander {
			sb.WriteString(fmt.Sprintf("%s=%d  ", name, w))
			if strings.Contains(name, "Tergrid") {
				tergridWins += w
			}
		}
		avgRounds := result.AvgTurns / float64(nSeats)
		sb.WriteString(fmt.Sprintf("\n  Avg rounds: %.1f | Crashes: %d\n", avgRounds, result.Crashes))
		for _, crashLog := range result.CrashLogs {
			sb.WriteString(fmt.Sprintf("  ⚠ CRASH: %s\n", crashLog))
		}
	}

	sb.WriteString(fmt.Sprintf("\n═══ TERGRID OVERALL: %d/%d wins (%.1f%%) across %d pods ═══\n",
		tergridWins, totalGames, float64(tergridWins)/float64(totalGames)*100, len(seeds)))
	t.Log(sb.String())
}

// TestStress_SteelManDiversity runs 6 hand-picked pods that maximise
// archetype diversity and deliberately include decks with the newly
// implemented stax lock pieces (Defense Grid, Notion Thief, Trinisphere).
// Each pod runs 20 games with full analytics so we can evaluate whether
// the hat is executing each deck's intended strategy.
func TestStress_SteelManDiversity(t *testing.T) {
	corpus, meta := loadCorpus(t)
	allPaths := findAllDecksExtended(t, 4)

	find := func(substr string) string {
		for _, p := range allPaths {
			if strings.Contains(strings.ToLower(filepath.Base(p)), strings.ToLower(substr)) {
				return p
			}
		}
		return ""
	}

	type podDef struct {
		label string
		keys  []string
	}
	podDefs := []podDef{
		{"Stax vs Ninjas vs Lifegain vs Tokens",
			[]string{"tergrid", "yuriko", "oloro", "maja"}},
		{"Land Reanimate vs Zombies vs Extra Combats vs Artifacts",
			[]string{"sin_land", "varina", "moraug", "ragost"}},
		{"Battlecruiser Brawl",
			[]string{"coram", "maralen", "kuja", "eshki"}},
		{"Mixed Archetypes",
			[]string{"obeka", "soraya", "ulrich", "golbez"}},
		// cEDH pod disabled — individual games exceed 90s wall-clock.
		// Re-enable after combo win resolution + evaluator fast-path.
		// {"cEDH Combo Showdown",
		//	[]string{"cedh_turbo_b5_kinnan", "cedh_combo_partner_b5_kraum", "cedh_stormoff_b5_ral", "cedh_mullie_b5_muldrotha"}},
	}

	nSeats := 4
	nGames := 10

	var sb strings.Builder
	sb.WriteString("\n╔══════════════════════════════════════════════════╗\n")
	sb.WriteString("║ STEEL MAN ANALYSIS — 4 pods × 10 games          ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════╝\n")

	for podIdx, pd := range podDefs {
		paths := make([]string, 0, nSeats)
		for _, key := range pd.keys {
			path := find(key)
			if path == "" {
				t.Logf("skipping pod %d (%s): deck %q not found", podIdx+1, pd.label, key)
				break
			}
			paths = append(paths, path)
		}
		if len(paths) < nSeats {
			continue
		}
		decks := loadNDecks(t, paths, corpus, meta)

		logs := make([]*[]string, nSeats)
		for i := range logs {
			log := make([]string, 0, 1024)
			logs[i] = &log
		}
		strategies := make([]*hat.StrategyProfile, nSeats)
		for i := 0; i < nSeats; i++ {
			strategies[i] = hat.LoadStrategyFromFreya(paths[i])
		}
		factories := make([]HatFactory, nSeats)
		for i := 0; i < nSeats; i++ {
			idx := i
			factories[i] = func() gameengine.Hat {
				yh := hat.NewYggdrasilHat(strategies[idx], 50)
				yh.DecisionLog = logs[idx]
				return yh
			}
		}

		seed := int64(podIdx*1000 + 7)
		cfg := TournamentConfig{
			Decks:            decks,
			NSeats:           nSeats,
			NGames:           nGames,
			Seed:             seed,
			HatFactories:     factories,
			Workers:          1,
			CommanderMode:    true,
			AnalyticsEnabled: true,
			MaxTurnsPerGame:  60,
		}

		result, err := Run(cfg)
		if err != nil {
			t.Fatalf("pod %d (%s): %v", podIdx+1, pd.label, err)
		}

		sb.WriteString(fmt.Sprintf("\n┌─ Pod %d: %s ─────────────────────────\n", podIdx+1, pd.label))
		for i, d := range decks {
			arch := "?"
			bracket := 0
			if strategies[i] != nil {
				arch = strategies[i].Archetype
				bracket = strategies[i].Bracket
			}
			sb.WriteString(fmt.Sprintf("│ Seat %d: %-35s [%s b%d] — %s\n",
				i+1, d.CommanderName, arch, bracket, filepath.Base(paths[i])))
		}

		sb.WriteString("│ Wins: ")
		for name, w := range result.WinsByCommander {
			sb.WriteString(fmt.Sprintf("%s=%d  ", name, w))
		}
		avgRounds := result.AvgTurns / float64(nSeats)
		sb.WriteString(fmt.Sprintf("\n│ Avg rounds: %.1f | Draws: %d | Crashes: %d\n", avgRounds, result.Draws, result.Crashes))

		if len(result.ELO) > 0 {
			sb.WriteString("│ ELO: ")
			for _, e := range result.ELO {
				sb.WriteString(fmt.Sprintf("%s=%.0f  ", e.Commander, e.Rating))
			}
			sb.WriteString("\n")
		}

		if len(result.Analyses) > 0 {
			report := &analytics.AnalyticsReport{
				Analyses:       result.Analyses,
				CardRankings:   result.CardRankings,
				MatchupDetails: result.MatchupDetails,
				CommanderNames: result.CommanderNames,
				TotalGames:     result.Games,
				Duration:       result.Duration,
			}
			report.WriteWinConditionsTo(&sb)
			report.WriteMVPCardsTo(&sb, 5)
			report.WriteMissedCombosTo(&sb)
			cmdrNames := make([]string, len(decks))
			for ci, d := range decks {
				cmdrNames[ci] = d.CommanderName
			}
			report.WriteStallWarningsTo(&sb, cmdrNames)

			sb.WriteString("\n## Per-Game Detail\n\n")
			sb.WriteString("| Game | Winner | Win Condition | Killing Card | Turns | Final Life |\n")
			sb.WriteString("|---:|---|---|---|---:|---|\n")
			for gi, ga := range result.Analyses {
				winName := "draw"
				if ga.WinnerSeat >= 0 && ga.WinnerSeat < len(decks) {
					winName = decks[ga.WinnerSeat].CommanderName
				}
				lifeStr := ""
				for si, p := range ga.Players {
					if si > 0 {
						lifeStr += " / "
					}
					lifeStr += fmt.Sprintf("S%d:%d", si+1, p.FinalLife)
				}
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %d | %s |\n",
					gi+1, winName, ga.WinCondition, ga.WinningCard, ga.TotalTurns, lifeStr))
			}
		}

		for i, log := range logs {
			entries := *log
			castEvals, passes, holds, attacks := 0, 0, 0, 0
			comboBoosts, passBoosts := 0, 0
			topCasts := make(map[string]int)
			for _, line := range entries {
				if strings.Contains(line, "] CAST eval") {
					castEvals++
				}
				if strings.Contains(line, "→ PASS") {
					passes++
				}
				if strings.Contains(line, "HOLD (below threshold)") {
					holds++
				}
				if strings.Contains(line, "] ATTACK") {
					attacks++
				}
				if strings.Contains(line, "boost=0.") && !strings.Contains(line, "boost=0.00)") {
					passBoosts++
				}
				if strings.Contains(line, "heuristic=1.") || strings.Contains(line, "heuristic=0.8") {
					comboBoosts++
				}
				if idx := strings.Index(line, "→ CAST "); idx >= 0 {
					rest := line[idx+len("→ CAST "):]
					if paren := strings.Index(rest, " ("); paren > 0 {
						topCasts[rest[:paren]]++
					}
				}
			}
			passRate := 0.0
			if castEvals > 0 {
				passRate = float64(passes) / float64(castEvals) * 100
			}
			sb.WriteString(fmt.Sprintf("│ [Seat %d] %s — %d decisions\n", i+1, decks[i].CommanderName, len(entries)))
			sb.WriteString(fmt.Sprintf("│   Casts: %d evals, %d passes (%.0f%%) | Attacks: %d, Holds: %d\n",
				castEvals, passes, passRate, attacks, holds))
			sb.WriteString(fmt.Sprintf("│   Combo-boosts: %d | Pass-boosts: %d\n", comboBoosts, passBoosts))

			type castCount struct {
				name  string
				count int
			}
			var sorted []castCount
			for n, c := range topCasts {
				sorted = append(sorted, castCount{n, c})
			}
			sort.Slice(sorted, func(a, b int) bool { return sorted[a].count > sorted[b].count })
			if len(sorted) > 5 {
				sorted = sorted[:5]
			}
			sb.WriteString("│   Top casts: ")
			for _, cc := range sorted {
				sb.WriteString(fmt.Sprintf("%s×%d  ", cc.name, cc.count))
			}
			sb.WriteString("\n")
		}

		for _, crashLog := range result.CrashLogs {
			sb.WriteString(fmt.Sprintf("│ ⚠ CRASH: %s\n", crashLog))
		}
		sb.WriteString("└──────────────────────────────────────────\n")
	}

	t.Log(sb.String())
}

// TestStress_OloroMatchups runs focused matchups against Oloro to probe
// what breaks the lifegain stall. Pod 1: 7174n1c heavy hitters vs Oloro.
// Pod 2: Obeka vs Oloro 1v1 (2-seat).
// Pod 3: aggro-heavy pod stacked against Oloro.
func TestStress_OloroMatchups(t *testing.T) {
	corpus, meta := loadCorpus(t)
	allPaths := findAllDecksExtended(t, 4)

	find := func(substr string) string {
		for _, p := range allPaths {
			if strings.Contains(strings.ToLower(filepath.Base(p)), strings.ToLower(substr)) {
				return p
			}
		}
		return ""
	}

	type podDef struct {
		label string
		keys  []string
		seats int
	}
	podDefs := []podDef{
		{"7174n1c Heavy Hitters vs Oloro",
			[]string{"sin_land", "coram", "varina", "oloro"}, 4},
		{"Obeka vs Oloro (1v1)",
			[]string{"obeka", "oloro"}, 2},
		{"Aggro Rush vs Oloro",
			[]string{"eshki", "yuriko", "moraug", "oloro"}, 4},
	}

	nGames := 20

	var sb strings.Builder
	sb.WriteString("\n╔══════════════════════════════════════════════════╗\n")
	sb.WriteString("║ OLORO MATCHUP PROBE — 3 pods × 20 games         ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════╝\n")

	for podIdx, pd := range podDefs {
		nSeats := pd.seats
		paths := make([]string, 0, nSeats)
		for _, key := range pd.keys {
			path := find(key)
			if path == "" {
				t.Logf("skipping pod %d (%s): deck %q not found", podIdx+1, pd.label, key)
				break
			}
			paths = append(paths, path)
		}
		if len(paths) < nSeats {
			continue
		}

		decks := loadNDecks(t, paths, corpus, meta)
		strategies := make([]*hat.StrategyProfile, nSeats)
		for i := 0; i < nSeats; i++ {
			strategies[i] = hat.LoadStrategyFromFreya(paths[i])
		}

		logs := make([]*[]string, nSeats)
		for i := range logs {
			log := make([]string, 0, 1024)
			logs[i] = &log
		}
		factories := make([]HatFactory, nSeats)
		for i := 0; i < nSeats; i++ {
			idx := i
			factories[i] = func() gameengine.Hat {
				yh := hat.NewYggdrasilHat(strategies[idx], 50)
				yh.DecisionLog = logs[idx]
				return yh
			}
		}

		seed := int64(podIdx*1000 + 42)
		cfg := TournamentConfig{
			Decks:            decks,
			NSeats:           nSeats,
			NGames:           nGames,
			Seed:             seed,
			HatFactories:     factories,
			Workers:          1,
			CommanderMode:    true,
			AnalyticsEnabled: true,
			MaxTurnsPerGame:  60,
		}

		result, err := Run(cfg)
		if err != nil {
			t.Fatalf("pod %d (%s): %v", podIdx+1, pd.label, err)
		}

		sb.WriteString(fmt.Sprintf("\n┌─ Pod %d: %s ─────────────────────────\n", podIdx+1, pd.label))
		for i, d := range decks {
			arch := "?"
			bracket := 0
			if strategies[i] != nil {
				arch = strategies[i].Archetype
				bracket = strategies[i].Bracket
			}
			sb.WriteString(fmt.Sprintf("│ Seat %d: %-35s [%s b%d] — %s\n",
				i+1, d.CommanderName, arch, bracket, filepath.Base(paths[i])))
		}

		sb.WriteString("│ Wins: ")
		for name, w := range result.WinsByCommander {
			sb.WriteString(fmt.Sprintf("%s=%d  ", name, w))
		}
		avgRounds := result.AvgTurns / float64(nSeats)
		sb.WriteString(fmt.Sprintf("\n│ Avg rounds: %.1f | Draws: %d | Crashes: %d\n", avgRounds, result.Draws, result.Crashes))

		if len(result.ELO) > 0 {
			sb.WriteString("│ ELO: ")
			for _, e := range result.ELO {
				sb.WriteString(fmt.Sprintf("%s=%.0f  ", e.Commander, e.Rating))
			}
			sb.WriteString("\n")
		}

		if len(result.Analyses) > 0 {
			report := &analytics.AnalyticsReport{
				Analyses:       result.Analyses,
				CardRankings:   result.CardRankings,
				MatchupDetails: result.MatchupDetails,
				CommanderNames: result.CommanderNames,
				TotalGames:     result.Games,
				Duration:       result.Duration,
			}
			report.WriteWinConditionsTo(&sb)
			report.WriteMVPCardsTo(&sb, 5)
			report.WriteMissedCombosTo(&sb)
			cmdrNames := make([]string, len(decks))
			for ci, d := range decks {
				cmdrNames[ci] = d.CommanderName
			}
			report.WriteStallWarningsTo(&sb, cmdrNames)

			sb.WriteString("\n## Per-Game Detail\n\n")
			sb.WriteString("| Game | Winner | Win Condition | Killing Card | Turns | Final Life |\n")
			sb.WriteString("|---:|---|---|---|---:|---|\n")
			for gi, ga := range result.Analyses {
				winName := "draw"
				if ga.WinnerSeat >= 0 && ga.WinnerSeat < len(decks) {
					winName = decks[ga.WinnerSeat].CommanderName
				}
				lifeStr := ""
				for si, p := range ga.Players {
					if si > 0 {
						lifeStr += " / "
					}
					lifeStr += fmt.Sprintf("S%d:%d", si+1, p.FinalLife)
				}
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %d | %s |\n",
					gi+1, winName, ga.WinCondition, ga.WinningCard, ga.TotalTurns, lifeStr))
			}
		}

		for i, log := range logs {
			entries := *log
			castEvals, passes, holds, attacks := 0, 0, 0, 0
			topCasts := make(map[string]int)
			for _, line := range entries {
				if strings.Contains(line, "] CAST eval") {
					castEvals++
				}
				if strings.Contains(line, "→ PASS") {
					passes++
				}
				if strings.Contains(line, "HOLD (below threshold)") {
					holds++
				}
				if strings.Contains(line, "] ATTACK") {
					attacks++
				}
				if idx := strings.Index(line, "CAST eval → play "); idx >= 0 {
					name := line[idx+len("CAST eval → play "):]
					if paren := strings.Index(name, " ("); paren >= 0 {
						name = name[:paren]
					}
					topCasts[name]++
				}
			}
			passRate := 0.0
			if castEvals > 0 {
				passRate = float64(passes) / float64(castEvals) * 100
			}
			sb.WriteString(fmt.Sprintf("│ [Seat %d] %s — %d decisions\n", i+1, decks[i].CommanderName, len(entries)))
			sb.WriteString(fmt.Sprintf("│   Casts: %d evals, %d passes (%.0f%%) | Attacks: %d, Holds: %d\n",
				castEvals, passes, passRate, attacks, holds))

			type castCount struct {
				name  string
				count int
			}
			var sorted []castCount
			for n, c := range topCasts {
				sorted = append(sorted, castCount{n, c})
			}
			sort.Slice(sorted, func(a, b int) bool { return sorted[a].count > sorted[b].count })
			if len(sorted) > 5 {
				sorted = sorted[:5]
			}
			sb.WriteString("│   Top casts: ")
			for _, cc := range sorted {
				sb.WriteString(fmt.Sprintf("%s×%d  ", cc.name, cc.count))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("└──────────────────────────────────────────\n")
	}

	t.Log(sb.String())
}
