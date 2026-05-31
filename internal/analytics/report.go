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

	// Win Rate When Cast (per-card correlation with winning).
	r.writeWinRateWhenCast(&b, 15)

	// Mana Efficiency.
	r.writeManaEfficiency(&b)

	// Curve Realization (per-deck cast-CMC distribution per turn).
	r.writeCurveRealization(&b)

	// Tempo Analysis.
	r.writeTempoAnalysis(&b)

	// Per-Commander Breakdown.
	r.writeCommanderBreakdown(&b)

	// Per-Deck Aggregates (winrate, turn-to-finisher, common matchups).
	r.writeDeckAggregates(&b, 5)

	// Missed Combos.
	r.writeMissedCombos(&b)

	// Missed Finishers.
	r.writeMissedFinishers(&b)

	// Co-Trigger Interactions.
	r.writeCoTriggers(&b)

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

	total := max(r.TotalGames, 1)
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
	minGames := max(r.TotalGames/10, 1) // at least 10% of games
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

// writeCoTriggers writes a section listing card pairs that co-triggered
// within the same turn with verified causal resource links.
func (r *AnalyticsReport) writeCoTriggers(b *strings.Builder) {
	// Collect all observations across all analyzed games.
	var allObs []CoTriggerObservation
	for _, ga := range r.Analyses {
		if ga == nil {
			continue
		}
		allObs = append(allObs, ga.CoTriggerObservations...)
	}

	summaries := AggregateCoTriggers(allObs)
	if len(summaries) == 0 {
		return
	}

	b.WriteString("## Co-Trigger Interactions\n\n")
	b.WriteString("_Card pairs that triggered in the same turn with a verified causal resource link._\n\n")

	b.WriteString("| Rank | Card A | Card B | Occurrences | Avg Impact | Pattern |\n")
	b.WriteString("|---:|---|---|---:|---:|---|\n")

	limit := 15
	if limit > len(summaries) {
		limit = len(summaries)
	}
	for i := 0; i < limit; i++ {
		s := &summaries[i]
		fmt.Fprintf(b, "| %d | %s | %s | %d | %.1f | %s |\n",
			i+1, s.CardA, s.CardB, s.Occurrences, s.AvgImpact, s.TopPattern)
	}
	b.WriteString("\n")
}

// WriteCoTriggersTo writes the co-trigger interactions section to an external builder.
func (r *AnalyticsReport) WriteCoTriggersTo(b *strings.Builder) {
	r.writeCoTriggers(b)
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

// writeDeckAggregates writes a deck-level aggregate roll-up: winrate,
// turn-to-finisher, win-condition mix, and the deck's top opponents
// with per-matchup records. Sits one layer above
// writeCommanderBreakdown (which is per-game-instance averages); this
// section answers "how did each DECK perform across the tournament,
// and who did it face."
func (r *AnalyticsReport) writeDeckAggregates(b *strings.Builder, topMatchups int) {
	aggs := AggregateDecks(r.Analyses, r.CommanderNames)
	if len(aggs) == 0 {
		return
	}

	b.WriteString("## Per-Deck Aggregates\n\n")
	b.WriteString("_Multi-game rollup per deck: winrate, average turn the winning card hit the table (turn-to-finisher), and most-faced opponents._\n\n")

	b.WriteString("| Deck | Games | Wins | Win% | Avg Turn-to-Finisher | Avg Game Length |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|\n")
	for i := range aggs {
		d := &aggs[i]
		ttf := "n/a"
		if d.FinisherSampleSize > 0 {
			ttf = fmt.Sprintf("%.1f (n=%d)", d.AvgTurnToFinisher, d.FinisherSampleSize)
		}
		fmt.Fprintf(b, "| %s | %d | %d | %.1f%% | %s | %.1f |\n",
			d.CommanderName, d.GamesPlayed, d.GamesWon, d.WinRate*100, ttf, d.AvgGameLength)
	}
	b.WriteString("\n")

	// Per-deck matchup breakdowns + win-condition mix.
	for i := range aggs {
		d := &aggs[i]
		if d.GamesPlayed == 0 {
			continue
		}
		fmt.Fprintf(b, "### %s\n\n", d.CommanderName)

		if len(d.WinConditions) > 0 {
			b.WriteString("**Wins by condition:** ")
			type kv struct {
				k string
				v int
			}
			conds := make([]kv, 0, len(d.WinConditions))
			for k, v := range d.WinConditions {
				conds = append(conds, kv{k, v})
			}
			sort.SliceStable(conds, func(i, j int) bool { return conds[i].v > conds[j].v })
			parts := make([]string, 0, len(conds))
			for _, c := range conds {
				parts = append(parts, fmt.Sprintf("%s=%d", c.k, c.v))
			}
			b.WriteString(strings.Join(parts, ", "))
			b.WriteString("\n\n")
		}

		if len(d.Matchups) == 0 {
			continue
		}
		b.WriteString("**Common matchups:**\n\n")
		b.WriteString("| Opponent | Games | Wins | Losses | Win% |\n")
		b.WriteString("|---|---:|---:|---:|---:|\n")
		limit := topMatchups
		if limit <= 0 || limit > len(d.Matchups) {
			limit = len(d.Matchups)
		}
		for j := 0; j < limit; j++ {
			m := &d.Matchups[j]
			fmt.Fprintf(b, "| %s | %d | %d | %d | %.1f%% |\n",
				m.Opponent, m.Games, m.Wins, m.Losses, m.WinRate*100)
		}
		b.WriteString("\n")
	}
}

// WriteDeckAggregatesTo writes the per-deck aggregates section to an
// external builder. topMatchups caps the matchup rows shown per deck
// (0 or negative = all).
func (r *AnalyticsReport) WriteDeckAggregatesTo(b *strings.Builder, topMatchups int) {
	r.writeDeckAggregates(b, topMatchups)
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

// writeWinRateWhenCast surfaces the top-N cards whose presence in a
// game correlates most strongly with winning, gated on a meaningful
// cast-sample size. Distinct from MVP Cards (which sorts by absolute
// GamesWon) and Kill Shot Cards (which scores the killing blow only).
//
// Sort key: WinRateWhenCast desc, tiebreak by GamesCast desc so a card
// at 100% on 50 games ranks above a card at 100% on 3. Filter:
// GamesCast >= max(TotalGames/10, 5) so single-game flukes don't crowd
// out the signal.
//
// Reports "%w / sample N" so the reader sees the sample backing the
// rate. A 67% rate on 3 games is barely-signal; the renderer surfaces
// the denominator so deckbuilders can apply their own confidence
// threshold without re-running with a higher game count.
func (r *AnalyticsReport) writeWinRateWhenCast(b *strings.Builder, n int) {
	b.WriteString("## Win Rate When Cast (Per-Card Correlation)\n\n")
	b.WriteString("_Top cards by win % in games where they were actually cast._ ")
	b.WriteString("_Distinct from MVP (absolute wins) and Kill Shot Rate (final blow only) — this metric asks \"when I drew and cast this card, did I win?\"_\n\n")

	if len(r.CardRankings) == 0 {
		b.WriteString("_No card data._\n\n")
		return
	}

	minSample := r.TotalGames / 10
	if minSample < 5 {
		minSample = 5
	}

	eligible := make([]CardRanking, 0)
	for _, cr := range r.CardRankings {
		if cr.GamesCast >= minSample {
			eligible = append(eligible, cr)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].WinRateWhenCast != eligible[j].WinRateWhenCast {
			return eligible[i].WinRateWhenCast > eligible[j].WinRateWhenCast
		}
		return eligible[i].GamesCast > eligible[j].GamesCast
	})

	if len(eligible) == 0 {
		fmt.Fprintf(b, "_No cards reached the minimum sample size of %d games cast._\n\n", minSample)
		return
	}

	b.WriteString("| Rank | Card | Win % When Cast | Games Cast | Games Won When Cast | Avg Turn Cast |\n")
	b.WriteString("|---:|---|---:|---:|---:|---:|\n")

	limit := n
	if limit > len(eligible) {
		limit = len(eligible)
	}
	for i := 0; i < limit; i++ {
		cr := &eligible[i]
		fmt.Fprintf(b, "| %d | %s | %.0f%% | %d | %d | %.1f |\n",
			i+1, cr.Name, cr.WinRateWhenCast*100, cr.GamesCast, cr.GamesCastAndWon, cr.AvgTurnCast)
	}
	fmt.Fprintf(b, "\n_Minimum sample: %d games cast._\n\n", minSample)
}

// writeCurveRealization surfaces, per commander, how their actual cast
// curve breaks down across turns. Answers "did you cast the curve you
// built?" — surfaces deckbuilding-vs-execution gap. The table shows
// average CMC cast on each of the first 10 turns (aggregated across
// all games the deck played). A column showing "0.0" means the deck
// cast nothing that turn on average (mana flood, mana screw, etc.).
//
// Sample size and total cast count per commander are shown alongside
// so readers can gate on the underlying sample. Tail turns (11+) are
// folded into a single AvgT11+ bucket — the late-game distribution
// is rarely curve-shaped and showing 60+ columns is noise.
func (r *AnalyticsReport) writeCurveRealization(b *strings.Builder) {
	b.WriteString("## Curve Realization (Did You Cast the Curve You Built?)\n\n")
	b.WriteString("_Per-commander average CMC cast per turn, aggregated across all games. Tail turns folded into T11+._\n\n")

	if len(r.Analyses) == 0 {
		b.WriteString("_No game data._\n\n")
		return
	}

	const earlyTurns = 10

	type curveAcc struct {
		gameCount int
		// Sum and count per turn bucket (1..earlyTurns). Bucket 0 is
		// unused; bucket earlyTurns+1 is "tail" (T11+).
		sumByTurn   []int
		countByTurn []int
		totalSpells int
		totalCMC    int
	}
	byCmdr := make(map[string]*curveAcc)

	for _, ga := range r.Analyses {
		if ga == nil {
			continue
		}
		for i := range ga.Players {
			pa := &ga.Players[i]
			if pa.CommanderName == "" {
				continue
			}
			acc, ok := byCmdr[pa.CommanderName]
			if !ok {
				acc = &curveAcc{
					sumByTurn:   make([]int, earlyTurns+2),
					countByTurn: make([]int, earlyTurns+2),
				}
				byCmdr[pa.CommanderName] = acc
			}
			acc.gameCount++
			for turn, casts := range pa.CastsByTurn {
				if turn == 0 || len(casts) == 0 {
					continue
				}
				bucket := turn
				if bucket > earlyTurns {
					bucket = earlyTurns + 1
				}
				for _, c := range casts {
					acc.sumByTurn[bucket] += c
					acc.countByTurn[bucket]++
					acc.totalSpells++
					acc.totalCMC += c
				}
			}
		}
	}

	if len(byCmdr) == 0 {
		b.WriteString("_No curve data._\n\n")
		return
	}

	// Header. T1..T10 + T11+ + AvgAll + Casts.
	b.WriteString("| Commander |")
	for t := 1; t <= earlyTurns; t++ {
		fmt.Fprintf(b, " T%d |", t)
	}
	b.WriteString(" T11+ | AvgAll | Casts |\n")
	b.WriteString("|---|")
	for t := 0; t < earlyTurns+1; t++ {
		b.WriteString("---:|")
	}
	b.WriteString("---:|---:|\n")

	// Render rows in CommanderNames order, then any leftover alphabetically.
	rendered := make(map[string]bool)
	render := func(name string) {
		acc, ok := byCmdr[name]
		if !ok || acc.totalSpells == 0 {
			return
		}
		rendered[name] = true
		fmt.Fprintf(b, "| %s |", name)
		for t := 1; t <= earlyTurns+1; t++ {
			if acc.countByTurn[t] == 0 {
				b.WriteString(" 0.0 |")
				continue
			}
			avg := float64(acc.sumByTurn[t]) / float64(acc.countByTurn[t])
			fmt.Fprintf(b, " %.1f |", avg)
		}
		avgAll := float64(acc.totalCMC) / float64(acc.totalSpells)
		fmt.Fprintf(b, " %.2f | %d |\n", avgAll, acc.totalSpells)
	}

	for _, name := range r.CommanderNames {
		render(name)
	}
	leftover := make([]string, 0)
	for name := range byCmdr {
		if !rendered[name] {
			leftover = append(leftover, name)
		}
	}
	sort.Strings(leftover)
	for _, name := range leftover {
		render(name)
	}
	b.WriteString("\n")
}

