package analytics

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// AnalyticsReport holds the full analytics output ready for rendering.
type AnalyticsReport struct {
	Analyses       []*GameAnalysis
	CardRankings   []CardRanking
	MatchupDetails []MatchupDetail
	CommanderNames []string
	TotalGames     int
	Duration       time.Duration
}

// WriteMarkdown renders the analytics report as a markdown file.
func (r *AnalyticsReport) WriteMarkdown(path string) error {
	if r == nil {
		return fmt.Errorf("analytics: nil report")
	}
	var b strings.Builder

	b.WriteString("# Heimdall Analytics Report\n\n")
	b.WriteString(fmt.Sprintf("_Generated: %s_\n\n", time.Now().UTC().Format(time.RFC3339)))
	fmt.Fprintf(&b, "- **Games analyzed:** %d\n", r.TotalGames)
	fmt.Fprintf(&b, "- **Duration:** %s\n", r.Duration.Round(time.Millisecond))
	fmt.Fprintf(&b, "- **Decks:** %s\n\n", strings.Join(r.CommanderNames, ", "))

	// Win Conditions breakdown.
	r.writeWinConditions(&b)

	// MVP Cards.
	r.writeMVPCards(&b, 10)

	// Dead Cards.
	r.writeDeadCards(&b, 10)

	// Kill Shot Cards.
	r.writeKillShotCards(&b, 10)

	// Mana Efficiency.
	r.writeManaEfficiency(&b)

	// Tempo Analysis.
	r.writeTempoAnalysis(&b)

	// Per-Commander Breakdown.
	r.writeCommanderBreakdown(&b)

	// Missed Combos.
	r.writeMissedCombos(&b)

	// Missed Finishers.
	r.writeMissedFinishers(&b)

	// Matchup Details.
	r.writeMatchupDetails(&b)

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeWinConditions adds the win condition pie-chart section.
func (r *AnalyticsReport) writeWinConditions(b *strings.Builder) {
	b.WriteString("## Win Conditions\n\n")

	conditionCounts := make(map[string]int)
	for _, ga := range r.Analyses {
		if ga != nil {
			conditionCounts[ga.WinCondition]++
		}
	}

	if len(conditionCounts) == 0 {
		b.WriteString("_No games analyzed._\n\n")
		return
	}

	b.WriteString("| Win Condition | Count | Percentage |\n")
	b.WriteString("|---|---:|---:|\n")

	// Sort by count desc.
	type kv struct {
		k string
		v int
	}
	sorted := make([]kv, 0, len(conditionCounts))
	for k, v := range conditionCounts {
		sorted = append(sorted, kv{k, v})
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })

	total := max1(r.TotalGames)
	for _, x := range sorted {
		pct := 100.0 * float64(x.v) / float64(total)
		fmt.Fprintf(b, "| %s | %d | %.1f%% |\n", x.k, x.v, pct)
	}
	b.WriteString("\n")
}

// writeMVPCards writes the top N cards by win association.
func (r *AnalyticsReport) writeMVPCards(b *strings.Builder, n int) {
	b.WriteString("## MVP Cards (Top by Wins)\n\n")

	if len(r.CardRankings) == 0 {
		b.WriteString("_No card data._\n\n")
		return
	}

	b.WriteString("| Rank | Card | Games Won | Times Cast | Avg Turn Cast | Avg Damage | Kill Shot Rate |\n")
	b.WriteString("|---:|---|---:|---:|---:|---:|---:|\n")

	limit := n
	if limit > len(r.CardRankings) {
		limit = len(r.CardRankings)
	}
	for i := 0; i < limit; i++ {
		cr := &r.CardRankings[i]
		fmt.Fprintf(b, "| %d | %s | %d | %d | %.1f | %.1f | %.0f%% |\n",
			i+1, cr.Name, cr.GamesWon, cr.TimesCast, cr.AvgTurnCast, cr.AvgDamageDealt,
			cr.KillShotRate*100)
	}
	b.WriteString("\n")
}

// writeDeadCards writes cards with the highest "never cast" rate.
func (r *AnalyticsReport) writeDeadCards(b *strings.Builder, n int) {
	b.WriteString("## Dead Cards (Highest Never-Cast Rate)\n\n")

	if len(r.CardRankings) == 0 {
		b.WriteString("_No card data._\n\n")
		return
	}

	// Filter to cards that appeared in at least a few games and sort by dead rate.
	minGames := max1(r.TotalGames / 10) // at least 10% of games
	deadSorted := make([]CardRanking, 0)
	for _, cr := range r.CardRankings {
		if cr.GamesPlayed >= minGames && cr.DeadInHandRate > 0 {
			deadSorted = append(deadSorted, cr)
		}
	}
	sort.SliceStable(deadSorted, func(i, j int) bool {
		return deadSorted[i].DeadInHandRate > deadSorted[j].DeadInHandRate
	})

	if len(deadSorted) == 0 {
		b.WriteString("_No dead cards detected._\n\n")
		return
	}

	b.WriteString("| Rank | Card | Never Cast Rate | Games Played | Games Won |\n")
	b.WriteString("|---:|---|---:|---:|---:|\n")

	limit := n
	if limit > len(deadSorted) {
		limit = len(deadSorted)
	}
	for i := 0; i < limit; i++ {
		cr := &deadSorted[i]
		fmt.Fprintf(b, "| %d | %s | %.0f%% | %d | %d |\n",
			i+1, cr.Name, cr.DeadInHandRate*100, cr.GamesPlayed, cr.GamesWon)
	}
	b.WriteString("\n")
}

// writeKillShotCards writes cards that most often delivered the winning blow.
func (r *AnalyticsReport) writeKillShotCards(b *strings.Builder, n int) {
	b.WriteString("## Kill Shot Cards\n\n")

	if len(r.CardRankings) == 0 {
		b.WriteString("_No card data._\n\n")
		return
	}

	// Filter to cards with any kill shot and sort by kill shot rate.
	killSorted := make([]CardRanking, 0)
	for _, cr := range r.CardRankings {
		if cr.KillShotRate > 0 {
			killSorted = append(killSorted, cr)
		}
	}
	sort.SliceStable(killSorted, func(i, j int) bool {
		return killSorted[i].KillShotRate > killSorted[j].KillShotRate
	})

	if len(killSorted) == 0 {
		b.WriteString("_No kill shot cards detected._\n\n")
		return
	}

	b.WriteString("| Rank | Card | Kill Shot Rate | Games Won | Avg Damage |\n")
	b.WriteString("|---:|---|---:|---:|---:|\n")

	limit := n
	if limit > len(killSorted) {
		limit = len(killSorted)
	}
	for i := 0; i < limit; i++ {
		cr := &killSorted[i]
		fmt.Fprintf(b, "| %d | %s | %.0f%% | %d | %.1f |\n",
			i+1, cr.Name, cr.KillShotRate*100, cr.GamesWon, cr.AvgDamageDealt)
	}
	b.WriteString("\n")
}

// writeManaEfficiency writes per-deck average mana spent vs wasted.
func (r *AnalyticsReport) writeManaEfficiency(b *strings.Builder) {
	b.WriteString("## Mana Efficiency\n\n")

	// Aggregate per commander.
	type manaStats struct {
		totalSpent  int
		totalWasted int
		gameCount   int
	}
	byCommander := make(map[string]*manaStats)

	for _, ga := range r.Analyses {
		if ga == nil {
			continue
		}
		for i := range ga.Players {
			pa := &ga.Players[i]
			ms, ok := byCommander[pa.CommanderName]
			if !ok {
				ms = &manaStats{}
				byCommander[pa.CommanderName] = ms
			}
			ms.totalSpent += pa.ManaSpent
			ms.totalWasted += pa.ManaWasted
			ms.gameCount++
		}
	}

	if len(byCommander) == 0 {
		b.WriteString("_No mana data._\n\n")
		return
	}

	b.WriteString("| Commander | Avg Mana Spent | Avg Mana Wasted | Efficiency |\n")
	b.WriteString("|---|---:|---:|---:|\n")

	for _, name := range r.CommanderNames {
		ms, ok := byCommander[name]
		if !ok || ms.gameCount == 0 {
			continue
		}
		avgSpent := float64(ms.totalSpent) / float64(ms.gameCount)
		avgWasted := float64(ms.totalWasted) / float64(ms.gameCount)
		total := avgSpent + avgWasted
		efficiency := 0.0
		if total > 0 {
			efficiency = avgSpent / total * 100
		}
		fmt.Fprintf(b, "| %s | %.1f | %.1f | %.0f%% |\n", name, avgSpent, avgWasted, efficiency)
	}
	b.WriteString("\n")
}

// writeTempoAnalysis writes the average turn of first meaningful play per deck.
func (r *AnalyticsReport) writeTempoAnalysis(b *strings.Builder) {
	b.WriteString("## Tempo Analysis\n\n")

	type tempoStats struct {
		firstCastSum   int
		firstCastCount int
		firstBloodSum  int
		firstBloodCnt  int
		landsPlayedSum int
		gameCount      int
	}
	byCommander := make(map[string]*tempoStats)

	for _, ga := range r.Analyses {
		if ga == nil {
			continue
		}
		for i := range ga.Players {
			pa := &ga.Players[i]
			ts, ok := byCommander[pa.CommanderName]
			if !ok {
				ts = &tempoStats{}
				byCommander[pa.CommanderName] = ts
			}
			ts.gameCount++
			ts.landsPlayedSum += pa.LandsPlayed

			// Find earliest cast in this player's card list.
			earliestCast := 999
			for j := range pa.CardsPlayed {
				if pa.CardsPlayed[j].TurnCast > 0 && pa.CardsPlayed[j].TurnCast < earliestCast {
					earliestCast = pa.CardsPlayed[j].TurnCast
				}
			}
			if earliestCast < 999 {
				ts.firstCastSum += earliestCast
				ts.firstCastCount++
			}
		}

		// Track first blood at game level per winner.
		if ga.FirstBlood > 0 && ga.WinnerSeat >= 0 && ga.WinnerSeat < len(ga.Players) {
			winnerName := ga.Players[ga.WinnerSeat].CommanderName
			ts, ok := byCommander[winnerName]
			if ok {
				ts.firstBloodSum += ga.FirstBlood
				ts.firstBloodCnt++
			}
		}
	}

	if len(byCommander) == 0 {
		b.WriteString("_No tempo data._\n\n")
		return
	}

	b.WriteString("| Commander | Avg First Cast | Avg Lands Played | Avg First Blood (as winner) |\n")
	b.WriteString("|---|---:|---:|---:|\n")

	for _, name := range r.CommanderNames {
		ts, ok := byCommander[name]
		if !ok || ts.gameCount == 0 {
			continue
		}
		avgFirstCast := 0.0
		if ts.firstCastCount > 0 {
			avgFirstCast = float64(ts.firstCastSum) / float64(ts.firstCastCount)
		}
		avgLands := float64(ts.landsPlayedSum) / float64(ts.gameCount)
		avgFirstBlood := 0.0
		if ts.firstBloodCnt > 0 {
			avgFirstBlood = float64(ts.firstBloodSum) / float64(ts.firstBloodCnt)
		}
		firstBloodStr := "n/a"
		if ts.firstBloodCnt > 0 {
			firstBloodStr = fmt.Sprintf("%.1f", avgFirstBlood)
		}
		fmt.Fprintf(b, "| %s | %.1f | %.1f | %s |\n", name, avgFirstCast, avgLands, firstBloodStr)
	}
	b.WriteString("\n")
}

// writeCommanderBreakdown writes per-commander aggregate stats.
func (r *AnalyticsReport) writeCommanderBreakdown(b *strings.Builder) {
	b.WriteString("## Per-Commander Breakdown\n\n")

	type cmdStats struct {
		wins         int
		games        int
		totalDmg     int
		totalTaken   int
		totalSpells  int
		totalCounter int
		totalRemoval int
		avgPeakBoard float64
		peakBoardSum int
	}
	byCommander := make(map[string]*cmdStats)

	for _, ga := range r.Analyses {
		if ga == nil {
			continue
		}
		for i := range ga.Players {
			pa := &ga.Players[i]
			cs, ok := byCommander[pa.CommanderName]
			if !ok {
				cs = &cmdStats{}
				byCommander[pa.CommanderName] = cs
			}
			cs.games++
			if pa.Won {
				cs.wins++
			}
			cs.totalDmg += pa.DamageDealt
			cs.totalTaken += pa.DamageTaken
			cs.totalSpells += pa.SpellsCast
			cs.totalCounter += pa.CountersCast
			cs.totalRemoval += pa.RemovalCast
			cs.peakBoardSum += pa.PeakBoardSize
		}
	}

	if len(byCommander) == 0 {
		b.WriteString("_No data._\n\n")
		return
	}

	b.WriteString("| Commander | Win% | Avg Dmg Dealt | Avg Dmg Taken | Avg Spells | Avg Removal | Avg Peak Board |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")

	for _, name := range r.CommanderNames {
		cs, ok := byCommander[name]
		if !ok || cs.games == 0 {
			continue
		}
		winPct := 100.0 * float64(cs.wins) / float64(cs.games)
		avgDmg := float64(cs.totalDmg) / float64(cs.games)
		avgTaken := float64(cs.totalTaken) / float64(cs.games)
		avgSpells := float64(cs.totalSpells) / float64(cs.games)
		avgRemoval := float64(cs.totalRemoval) / float64(cs.games)
		avgBoard := float64(cs.peakBoardSum) / float64(cs.games)
		fmt.Fprintf(b, "| %s | %.1f%% | %.1f | %.1f | %.1f | %.1f | %.1f |\n",
			name, winPct, avgDmg, avgTaken, avgSpells, avgRemoval, avgBoard)
	}
	b.WriteString("\n")
}

// writeMissedCombos writes a section listing combos that were available on
// the battlefield (all pieces present, mana sufficient) but were never
// executed. These represent Hat intelligence gaps.
func (r *AnalyticsReport) writeMissedCombos(b *strings.Builder) {
	// Collect all missed combos across all analyzed games.
	var allMissed []MissedCombo
	for _, ga := range r.Analyses {
		if ga == nil {
			continue
		}
		allMissed = append(allMissed, ga.MissedCombos...)
	}

	if len(allMissed) == 0 {
		return
	}

	b.WriteString("## Missed Combos\n\n")
	b.WriteString("_Known combos that were live on the battlefield but not executed (Hat intelligence gaps)._\n\n")

	for _, mc := range allMissed {
		commander := ""
		// Try to find the commander name for this seat from the analyses.
		for _, ga := range r.Analyses {
			if ga == nil {
				continue
			}
			if mc.Seat >= 0 && mc.Seat < len(ga.Players) {
				commander = ga.Players[mc.Seat].CommanderName
				break
			}
		}
		pieceStr := strings.Join(mc.Pieces, " + ")
		if commander != "" {
			fmt.Fprintf(b, "- **%s** (%s, seat %d, turn %d): had %s with %d mana available but didn't execute\n",
				mc.ComboName, commander, mc.Seat, mc.Turn, pieceStr, mc.ManaAvail)
		} else {
			fmt.Fprintf(b, "- **%s** (seat %d, turn %d): had %s with %d mana available but didn't execute\n",
				mc.ComboName, mc.Seat, mc.Turn, pieceStr, mc.ManaAvail)
		}
	}
	b.WriteString("\n")
}

// WriteMissedCombosTo writes the missed combos section to an external builder.
func (r *AnalyticsReport) WriteMissedCombosTo(b *strings.Builder) {
	r.writeMissedCombos(b)
}

func (r *AnalyticsReport) writeMissedFinishers(b *strings.Builder) {
	var allMissed []MissedFinisher
	for _, ga := range r.Analyses {
		if ga == nil {
			continue
		}
		allMissed = append(allMissed, ga.MissedFinishers...)
	}
	if len(allMissed) == 0 {
		return
	}

	b.WriteString("## Missed Finishers\n\n")
	b.WriteString("_Freya-classified finishers on board at game end without winning._\n\n")

	counts := map[string]int{}
	for _, mf := range allMissed {
		counts[mf.FinisherName]++
	}

	type kv struct {
		name  string
		count int
	}
	sorted := make([]kv, 0, len(counts))
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	b.WriteString("| Finisher | Times Missed | Avg Board Power | Avg Opp Life |\n")
	b.WriteString("|---|---:|---:|---:|\n")

	for _, entry := range sorted {
		totalPow, totalLife, n := 0, 0, 0
		for _, mf := range allMissed {
			if mf.FinisherName == entry.name {
				totalPow += mf.BoardPower
				totalLife += mf.OppLifeMin
				n++
			}
		}
		avgPow := float64(totalPow) / float64(n)
		avgLife := float64(totalLife) / float64(n)
		fmt.Fprintf(b, "| %s | %d | %.1f | %.1f |\n", entry.name, entry.count, avgPow, avgLife)
	}
	b.WriteString("\n")
}

// WriteMissedFinishersTo writes the missed finishers section to an external builder.
func (r *AnalyticsReport) WriteMissedFinishersTo(b *strings.Builder) {
	r.writeMissedFinishers(b)
}

// writeMatchupDetails writes deep matchup stats.
func (r *AnalyticsReport) writeMatchupDetails(b *strings.Builder) {
	if len(r.MatchupDetails) == 0 {
		return
	}

	b.WriteString("## Matchup Details\n\n")

	for _, md := range r.MatchupDetails {
		fmt.Fprintf(b, "### %s vs %s (%d games)\n\n", md.Deck1, md.Deck2, md.TotalGames)
		fmt.Fprintf(b, "- **%s:** %d wins (%.1f%%)\n", md.Deck1, md.Deck1Wins, md.Deck1WinRate*100)
		deck2WinRate := 0.0
		if md.TotalGames > 0 {
			deck2WinRate = float64(md.Deck2Wins) / float64(md.TotalGames)
		}
		fmt.Fprintf(b, "- **%s:** %d wins (%.1f%%)\n", md.Deck2, md.Deck2Wins, deck2WinRate*100)

		if md.AvgTurnToWin1 > 0 {
			fmt.Fprintf(b, "- %s avg turn to win: %.1f\n", md.Deck1, md.AvgTurnToWin1)
		}
		if md.AvgTurnToWin2 > 0 {
			fmt.Fprintf(b, "- %s avg turn to win: %.1f\n", md.Deck2, md.AvgTurnToWin2)
		}

		// Win condition breakdown.
		if len(md.Deck1WinsByType) > 0 {
			fmt.Fprintf(b, "- %s wins by type: ", md.Deck1)
			parts := make([]string, 0)
			for k, v := range md.Deck1WinsByType {
				parts = append(parts, fmt.Sprintf("%s=%d", k, v))
			}
			b.WriteString(strings.Join(parts, ", "))
			b.WriteString("\n")
		}
		if len(md.Deck2WinsByType) > 0 {
			fmt.Fprintf(b, "- %s wins by type: ", md.Deck2)
			parts := make([]string, 0)
			for k, v := range md.Deck2WinsByType {
				parts = append(parts, fmt.Sprintf("%s=%d", k, v))
			}
			b.WriteString(strings.Join(parts, ", "))
			b.WriteString("\n")
		}

		// Key cards.
		if len(md.KeyCards1) > 0 {
			fmt.Fprintf(b, "- %s key cards: %s\n", md.Deck1, strings.Join(md.KeyCards1, ", "))
		}
		if len(md.KeyCards2) > 0 {
			fmt.Fprintf(b, "- %s key cards: %s\n", md.Deck2, strings.Join(md.KeyCards2, ", "))
		}
		b.WriteString("\n")
	}
}

// --- Exported section writers for embedding in other reports ---

// WriteWinConditionsTo writes the win conditions section to an external builder.
func (r *AnalyticsReport) WriteWinConditionsTo(b *strings.Builder) {
	r.writeWinConditions(b)
}

// WriteMVPCardsTo writes the MVP cards section.
func (r *AnalyticsReport) WriteMVPCardsTo(b *strings.Builder, n int) {
	r.writeMVPCards(b, n)
}

// WriteDeadCardsTo writes the dead cards section.
func (r *AnalyticsReport) WriteDeadCardsTo(b *strings.Builder, n int) {
	r.writeDeadCards(b, n)
}

// WriteKillShotCardsTo writes the kill shot cards section.
func (r *AnalyticsReport) WriteKillShotCardsTo(b *strings.Builder, n int) {
	r.writeKillShotCards(b, n)
}

// WriteManaEfficiencyTo writes the mana efficiency section.
func (r *AnalyticsReport) WriteManaEfficiencyTo(b *strings.Builder) {
	r.writeManaEfficiency(b)
}

// WriteTempoAnalysisTo writes the tempo analysis section.
func (r *AnalyticsReport) WriteTempoAnalysisTo(b *strings.Builder) {
	r.writeTempoAnalysis(b)
}

// WriteCommanderBreakdownTo writes the per-commander breakdown section.
func (r *AnalyticsReport) WriteCommanderBreakdownTo(b *strings.Builder) {
	r.writeCommanderBreakdown(b)
}

// WriteStallWarningsTo writes the stall warning section to an external builder.
func (r *AnalyticsReport) WriteStallWarningsTo(b *strings.Builder, commanderNames []string) {
	r.writeStallWarnings(b, commanderNames)
}

func (r *AnalyticsReport) writeStallWarnings(b *strings.Builder, commanderNames []string) {
	var stalled []*StallReport
	var stalledGames []int
	for i, ga := range r.Analyses {
		if ga != nil && ga.StallIndicators != nil {
			stalled = append(stalled, ga.StallIndicators)
			stalledGames = append(stalledGames, i+1)
		}
	}
	if len(stalled) == 0 {
		return
	}

	b.WriteString(fmt.Sprintf("\n## Stall Warnings (%d/%d games)\n\n", len(stalled), r.TotalGames))
	b.WriteString("| Game | Survivors | Turns Since Kill | Life Leader | Life | Spread | Cause |\n")
	b.WriteString("|---:|---:|---:|---|---:|---:|---|\n")

	for i, sr := range stalled {
		leaderName := fmt.Sprintf("Seat %d", sr.LifeLeader+1)
		if sr.LifeLeader >= 0 && sr.LifeLeader < len(commanderNames) {
			leaderName = commanderNames[sr.LifeLeader]
		}
		b.WriteString(fmt.Sprintf("| %d | %d | %d | %s | %d | %d | %s |\n",
			stalledGames[i], sr.SurvivorsAtEnd, sr.TurnsSinceLastKill,
			leaderName, sr.LifeLeaderTotal, sr.LifeSpread, sr.Cause))
	}

	// Aggregate cause breakdown.
	causes := map[string]int{}
	for _, sr := range stalled {
		causes[sr.Cause]++
	}
	b.WriteString("\nStall causes: ")
	first := true
	for cause, count := range causes {
		if !first {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%s=%d", cause, count))
		first = false
	}
	b.WriteString("\n")
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}
