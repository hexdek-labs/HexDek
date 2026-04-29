package tournament

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// TestHatDecisionReview runs pods at medium budget (50) with full decision
// logging, then analyzes whether each deck is executing its intended
// archetype strategy. Checks: cast priorities, pass rates, attack behavior,
// combo piece usage, value engine deployment, and missed combos.
func TestHatDecisionReview(t *testing.T) {
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

	pods := []podDef{
		{"Tergrid vs Oloro (1v1)", []string{"tergrid", "oloro"}, 2},
		{"Yuriko vs Eshki (1v1)", []string{"yuriko", "eshki"}, 2},
		{"Obeka vs Moraug (1v1)", []string{"obeka", "moraug"}, 2},
		{"Stax vs Ninjas vs Lifegain vs Tokens", []string{"tergrid", "yuriko", "oloro", "maja"}, 4},
		{"Land Reanimate vs Zombies vs Extra Combats vs Artifacts", []string{"sin_land", "varina", "moraug", "ragost"}, 4},
		{"Battlecruiser Brawl", []string{"coram", "maralen", "kuja", "eshki"}, 4},
		{"Mixed Archetypes", []string{"obeka", "soraya", "ulrich", "golbez"}, 4},
	}

	nGames := 10
	budget := 50

	var sb strings.Builder
	sb.WriteString("\n╔══════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║ HAT DECISION QUALITY REVIEW — Archetype Strategy Execution     ║\n")
	sb.WriteString(fmt.Sprintf("║ %d games per pod | budget=%d (medium)                            ║\n", nGames, budget))
	sb.WriteString(fmt.Sprintf("║ Date: %s                                            ║\n", time.Now().Format("2006-01-02 15:04")))
	sb.WriteString("╚══════════════════════════════════════════════════════════════════╝\n")

	type deckProfile struct {
		name      string
		arch      string
		bracket   int
		gameplan  string
		strategy  *hat.StrategyProfile
		log       *[]string
		wins      int
		totalPods int
	}

	allProfiles := make(map[string]*deckProfile)

	for podIdx, pd := range pods {
		nSeats := pd.seats
		paths := make([]string, 0, nSeats)
		for _, key := range pd.keys {
			path := find(key)
			if path == "" {
				t.Logf("skipping pod %q: deck %q not found", pd.label, key)
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
			log := make([]string, 0, 4096)
			logs[i] = &log
		}

		factories := make([]HatFactory, nSeats)
		for i := 0; i < nSeats; i++ {
			idx := i
			factories[i] = func() gameengine.Hat {
				yh := hat.NewYggdrasilHat(strategies[idx], budget)
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

		start := time.Now()
		result, err := Run(cfg)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("pod %d (%s): %v", podIdx+1, pd.label, err)
		}

		mode := "4-PLAYER"
		if nSeats == 2 {
			mode = "1v1"
		}
		sb.WriteString(fmt.Sprintf("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"))
		sb.WriteString(fmt.Sprintf("  [%s] %s  (%s)\n", mode, pd.label, elapsed.Round(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"))

		for i, d := range decks {
			cmdrName := d.CommanderName
			arch := "?"
			bracket := 0
			gameplan := ""
			if strategies[i] != nil {
				arch = strategies[i].Archetype
				bracket = strategies[i].Bracket
				gameplan = strategies[i].GameplanSummary
			}

			entries := *logs[i]
			castEvals, passes, holds, attacks := 0, 0, 0, 0
			comboBoosts, passBoosts := 0, 0
			topCasts := make(map[string]int)
			topCandidates := make(map[string]int)

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
				if strings.Contains(line, "candidate:") {
					parts := strings.SplitN(line, "candidate:", 2)
					if len(parts) == 2 {
						rest := strings.TrimSpace(parts[1])
						fields := strings.Fields(rest)
						if len(fields) > 0 {
							topCandidates[fields[0]]++
						}
					}
				}
			}

			passRate := 0.0
			if castEvals > 0 {
				passRate = float64(passes) / float64(castEvals) * 100
			}
			attackRate := 0.0
			if attacks > 0 && holds > 0 {
				attackRate = float64(attacks-holds) / float64(attacks) * 100
			}

			sb.WriteString(fmt.Sprintf("\n┌─ Seat %d: %s ─────────────────────────\n", i+1, cmdrName))
			sb.WriteString(fmt.Sprintf("│ Archetype: %s | Bracket: %d\n", arch, bracket))
			if gameplan != "" {
				sb.WriteString(fmt.Sprintf("│ Gameplan: %s\n", gameplan))
			}
			sb.WriteString(fmt.Sprintf("│ Wins: %d/%d\n", result.WinsByCommander[cmdrName], nGames))
			sb.WriteString("│\n")

			sb.WriteString(fmt.Sprintf("│ DECISIONS: %d total entries\n", len(entries)))
			sb.WriteString(fmt.Sprintf("│   Cast evaluations: %d | Passes: %d (%.0f%% pass rate)\n",
				castEvals, passes, passRate))
			sb.WriteString(fmt.Sprintf("│   Attack phases: %d | Holds: %d", attacks, holds))
			if attacks > 0 {
				sb.WriteString(fmt.Sprintf(" (%.0f%% aggression)", attackRate))
			}
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("│   Combo-boosts: %d | Pass-boosts: %d\n", comboBoosts, passBoosts))

			// Top 8 cards cast — are they on-strategy?
			type castCount struct {
				name  string
				count int
			}
			var sortedCasts []castCount
			for n, c := range topCasts {
				sortedCasts = append(sortedCasts, castCount{n, c})
			}
			sort.Slice(sortedCasts, func(a, b int) bool { return sortedCasts[a].count > sortedCasts[b].count })
			if len(sortedCasts) > 8 {
				sortedCasts = sortedCasts[:8]
			}
			sb.WriteString("│\n│ TOP CASTS:\n")
			for rank, cc := range sortedCasts {
				marker := ""
				if strategies[i] != nil {
					for _, ve := range strategies[i].ValueEngineKeys {
						if strings.EqualFold(cc.name, ve) {
							marker = " ★ value-engine"
							break
						}
					}
					for _, tt := range strategies[i].TutorTargets {
						if strings.EqualFold(cc.name, tt) {
							marker = " ◆ tutor-target"
							break
						}
					}
					for _, cp := range strategies[i].ComboPieces {
						for _, piece := range cp.Pieces {
							if strings.EqualFold(cc.name, piece) {
								marker = fmt.Sprintf(" ⚡ combo-piece (%s)", cp.Type)
								break
							}
						}
					}
				}
				sb.WriteString(fmt.Sprintf("│   %d. %-35s ×%d%s\n", rank+1, cc.name, cc.count, marker))
			}

			// Strategy alignment assessment
			sb.WriteString("│\n│ STRATEGY ALIGNMENT:\n")
			switch strings.ToLower(arch) {
			case "stax":
				if passRate > 60 {
					sb.WriteString("│   ✓ High pass rate fits stax (hold resources, tax opponents)\n")
				} else {
					sb.WriteString(fmt.Sprintf("│   ⚠ Low pass rate (%.0f%%) — stax should hold more, play reactively\n", passRate))
				}
				if passBoosts > 0 {
					sb.WriteString(fmt.Sprintf("│   ✓ Pass-boosts detected (%d) — archetype-aware hold decisions\n", passBoosts))
				}
			case "aggro", "tribal", "voltron":
				if passRate < 40 {
					sb.WriteString("│   ✓ Low pass rate fits aggro/tribal (deploy threats fast)\n")
				} else {
					sb.WriteString(fmt.Sprintf("│   ⚠ High pass rate (%.0f%%) — aggro should deploy more aggressively\n", passRate))
				}
				if attacks > 0 && attackRate > 50 {
					sb.WriteString(fmt.Sprintf("│   ✓ High aggression (%.0f%%) — attacking as expected\n", attackRate))
				} else if attacks > 0 {
					sb.WriteString(fmt.Sprintf("│   ⚠ Low aggression (%.0f%%) — tribal/aggro should attack more\n", attackRate))
				}
			case "combo":
				if comboBoosts > 0 {
					sb.WriteString(fmt.Sprintf("│   ✓ Combo-boosts detected (%d) — prioritizing combo pieces\n", comboBoosts))
				} else {
					sb.WriteString("│   ⚠ No combo-boosts — may not recognize combo pieces\n")
				}
			case "midrange":
				if passRate >= 30 && passRate <= 60 {
					sb.WriteString("│   ✓ Balanced pass rate fits midrange\n")
				} else if passRate < 30 {
					sb.WriteString(fmt.Sprintf("│   ⚠ Very low pass rate (%.0f%%) — midrange should be more selective\n", passRate))
				} else {
					sb.WriteString(fmt.Sprintf("│   ⚠ High pass rate (%.0f%%) — midrange should be more proactive\n", passRate))
				}
			case "control":
				if passRate > 50 {
					sb.WriteString("│   ✓ High pass rate fits control\n")
				} else {
					sb.WriteString(fmt.Sprintf("│   ⚠ Low pass rate (%.0f%%) — control should hold interaction\n", passRate))
				}
			case "reanimator":
				sb.WriteString("│   Reanimator archetype — checking for graveyard-focused casts\n")
				if passRate > 50 {
					sb.WriteString("│   ✓ Conservative play fits reanimator (set up, then reanimate)\n")
				}
			case "spellslinger":
				if passRate < 30 {
					sb.WriteString("│   ✓ Low pass rate — casting lots of spells as expected\n")
				} else {
					sb.WriteString(fmt.Sprintf("│   ⚠ High pass rate (%.0f%%) — spellslinger should chain spells\n", passRate))
				}
			default:
				sb.WriteString(fmt.Sprintf("│   Archetype '%s' — no specific alignment check\n", arch))
			}

			// Value engine deployment check
			if strategies[i] != nil && len(strategies[i].ValueEngineKeys) > 0 {
				deployed := 0
				for _, ve := range strategies[i].ValueEngineKeys {
					if topCasts[ve] > 0 {
						deployed++
					}
				}
				pct := float64(deployed) / float64(len(strategies[i].ValueEngineKeys)) * 100
				sb.WriteString(fmt.Sprintf("│   Value engines deployed: %d/%d (%.0f%%)\n",
					deployed, len(strategies[i].ValueEngineKeys), pct))
			}

			// Combo piece casting check
			if strategies[i] != nil && len(strategies[i].ComboPieces) > 0 {
				piecesFound := 0
				totalPieces := 0
				for _, cp := range strategies[i].ComboPieces {
					for _, piece := range cp.Pieces {
						totalPieces++
						if topCasts[piece] > 0 {
							piecesFound++
						}
					}
				}
				if totalPieces > 0 {
					sb.WriteString(fmt.Sprintf("│   Combo pieces cast: %d/%d\n", piecesFound, totalPieces))
				}
			}

			// Track for global summary
			key := cmdrName
			if _, ok := allProfiles[key]; !ok {
				allProfiles[key] = &deckProfile{
					name:     cmdrName,
					arch:     arch,
					bracket:  bracket,
					gameplan: gameplan,
					strategy: strategies[i],
				}
			}
			allProfiles[key].wins += result.WinsByCommander[cmdrName]
			allProfiles[key].totalPods++

			sb.WriteString("└──────────────────────────────────────────\n")
		}

		// Pod-level analytics
		if len(result.Analyses) > 0 {
			report := &analytics.AnalyticsReport{
				Analyses:       result.Analyses,
				CardRankings:   result.CardRankings,
				MatchupDetails: result.MatchupDetails,
				CommanderNames: result.CommanderNames,
				TotalGames:     result.Games,
				Duration:       result.Duration,
			}
			sb.WriteString("\n  POD ANALYTICS:\n")
			report.WriteWinConditionsTo(&sb)
			report.WriteMVPCardsTo(&sb, 5)
			report.WriteMissedCombosTo(&sb)
			cmdrNames := make([]string, len(decks))
			for ci, d := range decks {
				cmdrNames[ci] = d.CommanderName
			}
			report.WriteStallWarningsTo(&sb, cmdrNames)
		}

		// Per-game results table
		if len(result.Analyses) > 0 {
			sb.WriteString("\n  PER-GAME:\n")
			sb.WriteString("  | Game | Winner | Win Condition | Killing Card | Turns |\n")
			sb.WriteString("  |---:|---|---|---|---:|\n")
			for gi, ga := range result.Analyses {
				winName := "draw"
				if ga.WinnerSeat >= 0 && ga.WinnerSeat < len(decks) {
					winName = decks[ga.WinnerSeat].CommanderName
				}
				sb.WriteString(fmt.Sprintf("  | %d | %s | %s | %s | %d |\n",
					gi+1, winName, ga.WinCondition, ga.WinningCard, ga.TotalTurns))
			}
		}
	}

	// Global summary — cross-pod archetype pattern recognition
	sb.WriteString("\n\n╔══════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║ GLOBAL SUMMARY — Cross-Pod Archetype Observations              ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════════╝\n")

	type summaryEntry struct {
		name     string
		arch     string
		wins     int
		pods     int
		bracket  int
		gameplan string
	}
	var summaries []summaryEntry
	for _, dp := range allProfiles {
		summaries = append(summaries, summaryEntry{
			name:     dp.name,
			arch:     dp.arch,
			wins:     dp.wins,
			pods:     dp.totalPods,
			bracket:  dp.bracket,
			gameplan: dp.gameplan,
		})
	}
	sort.Slice(summaries, func(a, b int) bool {
		return float64(summaries[a].wins)/float64(summaries[a].pods) >
			float64(summaries[b].wins)/float64(summaries[b].pods)
	})

	sb.WriteString("\n  Commander                          | Arch        | Wins | Pods | Win Rate | Bracket\n")
	sb.WriteString("  -----------------------------------|-------------|------|------|----------|--------\n")
	for _, s := range summaries {
		rate := float64(s.wins) / float64(s.pods*nGames) * 100
		sb.WriteString(fmt.Sprintf("  %-36s | %-11s | %4d | %4d | %5.0f%%   | %d\n",
			s.name, s.arch, s.wins, s.pods, rate, s.bracket))
	}

	t.Log(sb.String())
}
