package tournament

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
)

// WriteMarkdown renders the tournament result as a Markdown report and
// writes it to `path`. Output shape mirrors Python gauntlet_poker.py's
// terminal output so downstream tooling / humans can diff the two.
func (r *TournamentResult) WriteMarkdown(path string) error {
	if r == nil {
		return fmt.Errorf("tournament: nil result")
	}
	var b strings.Builder
	b.WriteString("# Tournament Report\n\n")
	b.WriteString(fmt.Sprintf("_Generated: %s_\n\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "- Games completed: %d\n", r.Games)
	fmt.Fprintf(&b, "- Crashes: %d\n", r.Crashes)
	fmt.Fprintf(&b, "- Draws: %d\n", r.Draws)
	fmt.Fprintf(&b, "- Avg turns / game: %.1f\n", r.AvgTurns)
	fmt.Fprintf(&b, "- Seats: %d\n", r.NSeats)
	fmt.Fprintf(&b, "- Total mode-change events: %d\n", r.TotalModeChanges)
	if r.Games > 0 {
		fmt.Fprintf(&b, "- Avg mode-changes / game: %.2f\n",
			float64(r.TotalModeChanges)/float64(r.Games))
	}
	fmt.Fprintf(&b, "- Wall time: %s\n", r.Duration)
	fmt.Fprintf(&b, "- Throughput: %.1f games/sec\n\n", r.GamesPerSecond)

	b.WriteString("## Wins by Commander\n\n")
	hasPerCmdr := len(r.GamesPlayedByCommander) > 0
	if hasPerCmdr {
		b.WriteString("| Commander | Wins | Games | Winrate |\n")
		b.WriteString("|---|---:|---:|---:|\n")
	} else {
		b.WriteString("| Commander | Wins | Winrate |\n")
		b.WriteString("|---|---:|---:|\n")
	}
	games := r.Games
	if games == 0 {
		games = 1
	}
	// Sort by winrate desc (per-commander when available).
	type kv struct {
		name    string
		wins    int
		played  int
		winrate float64
	}
	ranked := []kv{}
	for _, name := range r.CommanderNames {
		w := r.WinsByCommander[name]
		played := games
		if hasPerCmdr {
			played = r.GamesPlayedByCommander[name]
		}
		if played == 0 {
			played = 1
		}
		ranked = append(ranked, kv{name, w, played, float64(w) / float64(played)})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].winrate > ranked[j].winrate })
	for _, x := range ranked {
		pct := 100 * x.winrate
		if hasPerCmdr {
			fmt.Fprintf(&b, "| %s | %d | %d | %.1f%% |\n", x.name, x.wins, x.played, pct)
		} else {
			fmt.Fprintf(&b, "| %s | %d | %.1f%% |\n", x.name, x.wins, pct)
		}
	}
	if r.Draws > 0 {
		fmt.Fprintf(&b, "| _draws_ | %d | %.1f%% |\n", r.Draws, 100*float64(r.Draws)/float64(games))
	}

	// Avg turn to win.
	if len(r.AvgTurnToWin) > 0 {
		b.WriteString("\n## Avg Turn to Win\n\n")
		b.WriteString("| Commander | Avg Turn |\n")
		b.WriteString("|---|---:|\n")
		for _, x := range ranked {
			if avg, ok := r.AvgTurnToWin[x.name]; ok {
				fmt.Fprintf(&b, "| %s | %.1f |\n", x.name, avg)
			}
		}
	}

	// Game length distribution.
	b.WriteString("\n## Game Length Distribution\n\n")
	b.WriteString("| Turn Range | Games | Pct |\n")
	b.WriteString("|---|---:|---:|\n")
	labels := [4]string{"1-5", "6-10", "11-20", "21+"}
	for i, label := range labels {
		count := r.TurnDistribution[i]
		pct := 100 * float64(count) / float64(max1i(games))
		fmt.Fprintf(&b, "| %s | %d | %.0f%% |\n", label, count, pct)
	}

	// Matchup matrix.
	if len(r.MatchupMatrix) > 0 && len(ranked) > 1 {
		b.WriteString("\n## Matchup Matrix\n\n")
		b.WriteString("Head-to-head win percentage when both decks are in the same game.\n\n")
		// Header.
		b.WriteString("| |")
		for _, x := range ranked {
			// Truncate long names for the header.
			short := x.name
			if len(short) > 15 {
				short = short[:12] + "..."
			}
			fmt.Fprintf(&b, " %s |", short)
		}
		b.WriteString("\n|---|")
		for range ranked {
			b.WriteString("---:|")
		}
		b.WriteString("\n")
		for _, row := range ranked {
			fmt.Fprintf(&b, "| %s |", row.name)
			for _, col := range ranked {
				if row.name == col.name {
					b.WriteString(" --- |")
					continue
				}
				totalGames := r.MatchupGames[row.name][col.name]
				wins := r.MatchupMatrix[row.name][col.name]
				if totalGames > 0 {
					pct := 100 * float64(wins) / float64(totalGames)
					fmt.Fprintf(&b, " %.0f%% |", pct)
				} else {
					b.WriteString(" n/a |")
				}
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n## Elimination Matrix\n\n")
	b.WriteString("Rows = commander, columns = elimination slot (0 = first out, N-1 = winner).\n\n")
	b.WriteString("| Commander |")
	for i := 0; i < r.NSeats; i++ {
		fmt.Fprintf(&b, " slot %d |", i)
	}
	b.WriteString("\n|---|")
	for i := 0; i < r.NSeats; i++ {
		b.WriteString("---:|")
	}
	b.WriteString("\n")
	for _, name := range r.CommanderNames {
		fmt.Fprintf(&b, "| %s |", name)
		slots := r.EliminationByCommanderBySlot[name]
		for i := 0; i < r.NSeats; i++ {
			v := 0
			if i < len(slots) {
				v = slots[i]
			}
			fmt.Fprintf(&b, " %d |", v)
		}
		b.WriteString("\n")
	}

	// Analytics sections (only when AnalyticsEnabled).
	if len(r.Analyses) > 0 {
		ar := &analytics.AnalyticsReport{
			Analyses:       r.Analyses,
			CardRankings:   r.CardRankings,
			MatchupDetails: r.MatchupDetails,
			CommanderNames: r.CommanderNames,
			TotalGames:     r.Games,
			Duration:       r.Duration,
		}
		writeAnalyticsSections(&b, ar)
	}

	if len(r.TrueSkill) > 0 {
		b.WriteString("\n## TrueSkill Ratings\n\n")
		b.WriteString("| Commander | μ | σ | Conservative (μ-3σ) | Games |\n")
		b.WriteString("|---|---:|---:|---:|---:|\n")
		for _, e := range r.TrueSkill {
			fmt.Fprintf(&b, "| %s | %.1f | %.1f | %.1f | %d |\n",
				e.Commander, e.Mu, e.Sigma, e.Conservative, e.Games)
		}
	}

	if len(r.ParserGapSnippets) > 0 {
		b.WriteString("\n## Parser Gap Snippets (top 20)\n\n")
		type snip struct {
			k string
			v int
		}
		ss := []snip{}
		for k, v := range r.ParserGapSnippets {
			ss = append(ss, snip{k, v})
		}
		sort.SliceStable(ss, func(i, j int) bool { return ss[i].v > ss[j].v })
		limit := len(ss)
		if limit > 20 {
			limit = 20
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&b, "- `%s` (%d)\n", ss[i].k, ss[i].v)
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeAnalyticsSections appends Heimdall analytics sections to the
// tournament report.
func writeAnalyticsSections(b *strings.Builder, ar *analytics.AnalyticsReport) {
	b.WriteString("\n---\n\n# Heimdall Analytics\n\n")

	// Win Conditions.
	ar.WriteWinConditionsTo(b)

	// MVP Cards.
	ar.WriteMVPCardsTo(b, 10)

	// Dead Cards.
	ar.WriteDeadCardsTo(b, 10)

	// Kill Shot Cards.
	ar.WriteKillShotCardsTo(b, 10)

	// Mana Efficiency.
	ar.WriteManaEfficiencyTo(b)

	// Tempo Analysis.
	ar.WriteTempoAnalysisTo(b)

	// Per-Commander Breakdown.
	ar.WriteCommanderBreakdownTo(b)
}

func max1i(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

// PrintDashboard writes the winrate dashboard to stdout in the
// requested plaintext format.
func (r *TournamentResult) PrintDashboard(showMatchup bool) {
	if r == nil {
		return
	}
	games := r.Games
	if games == 0 {
		games = 1
	}

	fmt.Printf("\n=== TOURNAMENT RESULTS (%d games) ===\n\n", r.Games)

	// Sort by wins desc.
	type kv struct {
		name string
		wins int
	}
	ranked := []kv{}
	for _, name := range r.CommanderNames {
		ranked = append(ranked, kv{name, r.WinsByCommander[name]})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].wins > ranked[j].wins })

	hasPerCmdr := len(r.GamesPlayedByCommander) > 0
	fmt.Println("WINRATE RANKINGS:")
	for i, x := range ranked {
		played := games
		if hasPerCmdr {
			if p, ok := r.GamesPlayedByCommander[x.name]; ok && p > 0 {
				played = p
			}
		}
		pct := 100 * float64(x.wins) / float64(played)
		fmt.Printf("  %d. %-40s %5.1f%%  (%d/%d)\n", i+1, x.name, pct, x.wins, played)
	}
	if r.Draws > 0 {
		fmt.Printf("     %-40s %5.1f%%  (%d/%d)\n", "(draws)", 100*float64(r.Draws)/float64(games), r.Draws, r.Games)
	}

	// Matchup matrix.
	if showMatchup && len(r.MatchupMatrix) > 0 && len(ranked) > 1 {
		fmt.Println()
		fmt.Println("MATCHUP MATRIX:")

		// Compute column width.
		colW := 8
		// Print header.
		fmt.Printf("  %-30s", "")
		for _, col := range ranked {
			short := col.name
			if len(short) > colW {
				short = short[:colW-1] + "."
			}
			fmt.Printf(" %*s", colW, short)
		}
		fmt.Println()

		for _, row := range ranked {
			short := row.name
			if len(short) > 30 {
				short = short[:27] + "..."
			}
			fmt.Printf("  %-30s", short)
			for _, col := range ranked {
				if row.name == col.name {
					fmt.Printf(" %*s", colW, "---")
					continue
				}
				totalGames := r.MatchupGames[row.name][col.name]
				wins := r.MatchupMatrix[row.name][col.name]
				if totalGames > 0 {
					pct := 100 * float64(wins) / float64(totalGames)
					fmt.Printf(" %*s", colW, fmt.Sprintf("%.0f%%", pct))
				} else {
					fmt.Printf(" %*s", colW, "n/a")
				}
			}
			fmt.Println()
		}
	}

	// Avg turn to win.
	if len(r.AvgTurnToWin) > 0 {
		fmt.Println()
		fmt.Println("AVG TURN TO WIN:")
		for _, x := range ranked {
			if avg, ok := r.AvgTurnToWin[x.name]; ok {
				fmt.Printf("  %-40s turn %.1f\n", x.name+":", avg)
			}
		}
	}

	// Game length distribution.
	fmt.Println()
	fmt.Println("GAME LENGTH DISTRIBUTION:")
	labels := [4]string{"Turns 1-5", "Turns 6-10", "Turns 11-20", "Turns 21+"}
	for i, label := range labels {
		count := r.TurnDistribution[i]
		pct := 100 * float64(count) / float64(max1i(games))
		fmt.Printf("  %-16s %3.0f%% of games (%d)\n", label+":", pct, count)
	}

	// TrueSkill ratings.
	if len(r.TrueSkill) > 0 {
		fmt.Println()
		fmt.Println("TRUESKILL RATINGS:")
		for i, e := range r.TrueSkill {
			fmt.Printf("  %d. %-40s μ=%.1f  σ=%.1f  conservative=%.1f  (%d games)\n",
				i+1, e.Commander, e.Mu, e.Sigma, e.Conservative, e.Games)
		}
	}

	// Summary line.
	fmt.Println()
	fmt.Printf("Duration: %s  |  Throughput: %.1f games/sec  |  Crashes: %d  |  Concessions: %d  |  Avg turns: %.1f\n",
		r.Duration.Round(time.Millisecond), r.GamesPerSecond, r.Crashes, r.TotalConcessions, r.AvgTurns)
}
