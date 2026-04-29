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

// TestSkillScale runs 1v1 and 4-player pods at three skill levels
// (budget 0 = low, 50 = medium, 200 = nightmare) to evaluate how
// the YggdrasilHat performs at each depth setting after parser fixes.
func TestSkillScale(t *testing.T) {
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
		// 1v1 matchups
		{"Tergrid vs Oloro (1v1)", []string{"tergrid", "oloro"}, 2},
		{"Yuriko vs Eshki (1v1)", []string{"yuriko", "eshki"}, 2},
		{"Obeka vs Moraug (1v1)", []string{"obeka", "moraug"}, 2},
		// 4-player pods
		{"Stax vs Ninjas vs Lifegain vs Tokens", []string{"tergrid", "yuriko", "oloro", "maja"}, 4},
		{"Land Reanimate vs Zombies vs Extra Combats vs Artifacts", []string{"sin_land", "varina", "moraug", "ragost"}, 4},
		{"Battlecruiser Brawl", []string{"coram", "maralen", "kuja", "eshki"}, 4},
		{"Mixed Archetypes", []string{"obeka", "soraya", "ulrich", "golbez"}, 4},
	}

	budgets := []struct {
		label  string
		budget int
	}{
		{"LOW (budget=0)", 0},
		{"MEDIUM (budget=50)", 50},
		{"NIGHTMARE (budget=200)", 200},
	}

	nGames := 10

	var sb strings.Builder
	sb.WriteString("\n╔══════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║ SKILL SCALE BENCHMARK — 1v1 + 4-player × 3 skill levels    ║\n")
	sb.WriteString(fmt.Sprintf("║ %d games per pod per skill level                             ║\n", nGames))
	sb.WriteString(fmt.Sprintf("║ Date: %s                                        ║\n", time.Now().Format("2006-01-02 15:04")))
	sb.WriteString("╚══════════════════════════════════════════════════════════════╝\n")

	for _, bud := range budgets {
		sb.WriteString(fmt.Sprintf("\n\n━━━ %s ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n", bud.label))

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

			factories := make([]HatFactory, nSeats)
			budgetVal := bud.budget
			for i := 0; i < nSeats; i++ {
				idx := i
				factories[i] = func() gameengine.Hat {
					return hat.NewYggdrasilHat(strategies[idx], budgetVal)
				}
			}

			seed := int64(podIdx*1000 + bud.budget*100 + 7)
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
				t.Fatalf("pod %d (%s) budget=%d: %v", podIdx+1, pd.label, bud.budget, err)
			}

			mode := "4-PLAYER"
			if nSeats == 2 {
				mode = "1v1"
			}
			sb.WriteString(fmt.Sprintf("\n┌─ [%s] %s ─────────────────────────\n", mode, pd.label))
			for i, d := range decks {
				arch := "?"
				bracket := 0
				if strategies[i] != nil {
					arch = strategies[i].Archetype
					bracket = strategies[i].Bracket
				}
				sb.WriteString(fmt.Sprintf("│ Seat %d: %-35s [%s b%d]\n",
					i+1, d.CommanderName, arch, bracket))
			}

			sb.WriteString("│ Wins: ")
			type winEntry struct {
				name string
				wins int
			}
			var winList []winEntry
			for name, w := range result.WinsByCommander {
				winList = append(winList, winEntry{name, w})
			}
			sort.Slice(winList, func(a, b int) bool { return winList[a].wins > winList[b].wins })
			for _, we := range winList {
				pct := float64(we.wins) / float64(nGames) * 100
				sb.WriteString(fmt.Sprintf("%s=%d(%.0f%%)  ", we.name, we.wins, pct))
			}
			avgRounds := result.AvgTurns / float64(nSeats)
			sb.WriteString(fmt.Sprintf("\n│ Avg rounds: %.1f | Draws: %d | Crashes: %d | Time: %s\n",
				avgRounds, result.Draws, result.Crashes, elapsed.Round(time.Millisecond)))

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
			}
		}
	}

	t.Log(sb.String())
}
