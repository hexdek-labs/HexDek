package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// PowerTierAggregate is a cross-deck rollup of the S/A/B/C/D power-tier
// distribution produced by computeCardPower. It exists to answer the
// calibration question: are the S/A/B/C/D thresholds (75/60/40/25)
// producing a sensible distribution across a real-world deck corpus?
//
// If 60% of cards land in S, the threshold is too generous; if 60%
// land in D, the threshold is too harsh. A roughly normal distribution
// peaked at B/C with thin tails on S/D is the calibration target.
//
// The aggregate also slices the distribution by bracket (B1-B5) and
// primary archetype so the calibrator can spot-check: cEDH decks
// should skew S-heavy, casual precons should skew D-heavy.
type PowerTierAggregate struct {
	DeckCount      int
	TotalCards     int
	TierCounts     map[string]int     // S/A/B/C/D total card counts across all decks
	TierPercents   map[string]float64 // same, as fraction of TotalCards
	MeanPower      float64
	MedianPower    int
	MinPower       int
	MaxPower       int
	ScoreHistogram []ScoreBin         // 5-point bins from 0-100 (21 bins)
	ByBracket      []BracketTierRow   // one row per bracket present in the corpus (sorted 1-5)
	ByArchetype    []ArchetypeTierRow // one row per primary archetype (sorted by deck count desc)
}

// ScoreBin is one 5-point histogram bucket. RangeStart is the inclusive
// lower bound; RangeEnd is the inclusive upper bound (the topmost bin
// is [95, 100], not [95, 100)).
type ScoreBin struct {
	RangeStart int
	RangeEnd   int
	Count      int
	Percent    float64
}

// BracketTierRow is the S/A/B/C/D distribution for a single bracket
// (B1 through B5). DeckCount is how many decks in the bracket;
// CardCount is the total cards summed across those decks.
type BracketTierRow struct {
	Bracket    int
	Label      string
	DeckCount  int
	CardCount  int
	TierCounts map[string]int
}

// ArchetypeTierRow is the S/A/B/C/D distribution for a primary
// archetype. Sorted by DeckCount desc in the parent aggregate so the
// most-represented archetypes lead the calibration table.
type ArchetypeTierRow struct {
	Archetype  string
	DeckCount  int
	CardCount  int
	TierCounts map[string]int
}

// ComputePowerTierAggregate rolls up the power-tier distribution
// across the supplied report set. Skips reports without a Profile or
// without populated CardPowerLevels (defensive — earlier pipeline
// failures shouldn't crash the aggregator).
func ComputePowerTierAggregate(reports []*FreyaReport) *PowerTierAggregate {
	agg := &PowerTierAggregate{
		TierCounts:   map[string]int{"S": 0, "A": 0, "B": 0, "C": 0, "D": 0},
		TierPercents: map[string]float64{"S": 0, "A": 0, "B": 0, "C": 0, "D": 0},
	}

	// 21 5-point bins covering [0, 100]: [0,4], [5,9], ..., [95,100].
	const binWidth = 5
	const binCount = 21
	bins := make([]ScoreBin, binCount)
	for i := range bins {
		bins[i].RangeStart = i * binWidth
		bins[i].RangeEnd = i*binWidth + binWidth - 1
	}
	bins[binCount-1].RangeEnd = 100 // top bin is inclusive of 100

	// Per-bracket and per-archetype tallies built as maps for easy
	// accumulation, then flattened + sorted into the final slice shape.
	bracketMap := map[int]*BracketTierRow{}
	archMap := map[string]*ArchetypeTierRow{}

	var allPowers []int
	for _, r := range reports {
		if r == nil || r.Profile == nil || len(r.Profile.CardPowerLevels) == 0 {
			continue
		}
		agg.DeckCount++

		dp := r.Profile
		// Lazy-init bracket and archetype rows so a missing bracket /
		// archetype field doesn't pollute the rollup with empty keys.
		var br *BracketTierRow
		if dp.Bracket > 0 {
			br = bracketMap[dp.Bracket]
			if br == nil {
				br = &BracketTierRow{
					Bracket:    dp.Bracket,
					Label:      dp.BracketLabel,
					TierCounts: map[string]int{"S": 0, "A": 0, "B": 0, "C": 0, "D": 0},
				}
				bracketMap[dp.Bracket] = br
			}
			br.DeckCount++
		}
		var ar *ArchetypeTierRow
		if dp.PrimaryArchetype != "" {
			ar = archMap[dp.PrimaryArchetype]
			if ar == nil {
				ar = &ArchetypeTierRow{
					Archetype:  dp.PrimaryArchetype,
					TierCounts: map[string]int{"S": 0, "A": 0, "B": 0, "C": 0, "D": 0},
				}
				archMap[dp.PrimaryArchetype] = ar
			}
			ar.DeckCount++
		}

		for _, pl := range dp.CardPowerLevels {
			agg.TierCounts[pl.PowerTier]++
			agg.TotalCards++
			allPowers = append(allPowers, pl.Power)

			binIdx := pl.Power / binWidth
			if binIdx >= binCount {
				binIdx = binCount - 1
			}
			bins[binIdx].Count++

			if br != nil {
				br.TierCounts[pl.PowerTier]++
				br.CardCount++
			}
			if ar != nil {
				ar.TierCounts[pl.PowerTier]++
				ar.CardCount++
			}
		}
	}

	if agg.TotalCards == 0 {
		return agg
	}

	// Tier percentages.
	for tier, count := range agg.TierCounts {
		agg.TierPercents[tier] = float64(count) / float64(agg.TotalCards)
	}

	// Histogram percentages.
	for i := range bins {
		bins[i].Percent = float64(bins[i].Count) / float64(agg.TotalCards)
	}
	agg.ScoreHistogram = bins

	// Mean / median / min / max.
	sort.Ints(allPowers)
	agg.MinPower = allPowers[0]
	agg.MaxPower = allPowers[len(allPowers)-1]
	sum := 0
	for _, p := range allPowers {
		sum += p
	}
	agg.MeanPower = float64(sum) / float64(len(allPowers))
	agg.MedianPower = allPowers[len(allPowers)/2]

	// Flatten + sort the per-bracket rows (ascending bracket).
	for _, row := range bracketMap {
		agg.ByBracket = append(agg.ByBracket, *row)
	}
	sort.Slice(agg.ByBracket, func(i, j int) bool {
		return agg.ByBracket[i].Bracket < agg.ByBracket[j].Bracket
	})

	// Flatten + sort the per-archetype rows (descending deck count;
	// tiebreaker on archetype name for stable output).
	for _, row := range archMap {
		agg.ByArchetype = append(agg.ByArchetype, *row)
	}
	sort.Slice(agg.ByArchetype, func(i, j int) bool {
		if agg.ByArchetype[i].DeckCount != agg.ByArchetype[j].DeckCount {
			return agg.ByArchetype[i].DeckCount > agg.ByArchetype[j].DeckCount
		}
		return agg.ByArchetype[i].Archetype < agg.ByArchetype[j].Archetype
	})

	return agg
}

// PrintPowerTierAggregate renders the rollup as a multi-section text
// report: headline distribution, per-bracket table, per-archetype
// table, and a 5-point histogram with an inline bar chart so the
// calibrator can spot the distribution shape at a glance.
func PrintPowerTierAggregate(w io.Writer, agg *PowerTierAggregate) {
	if agg == nil || agg.TotalCards == 0 {
		return
	}

	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "POWER-TIER DISTRIBUTION (%d decks, %d cards)\n", agg.DeckCount, agg.TotalCards)
	fmt.Fprintf(w, "===========================================\n")
	fmt.Fprintf(w, "Mean power: %.1f   Median: %d   Min: %d   Max: %d\n\n",
		agg.MeanPower, agg.MedianPower, agg.MinPower, agg.MaxPower)

	// Overall tier line.
	fmt.Fprintf(w, "Overall tier mix:\n")
	for _, tier := range PowerTierOrder {
		fmt.Fprintf(w, "  %s: %5d cards (%5.1f%%)\n",
			tier, agg.TierCounts[tier], agg.TierPercents[tier]*100)
	}
	fmt.Fprintf(w, "\n")

	// Per-bracket breakdown.
	if len(agg.ByBracket) > 0 {
		fmt.Fprintf(w, "By bracket:\n")
		fmt.Fprintf(w, "  %-15s %5s %5s %5s %5s %5s %5s %5s\n",
			"BRACKET", "DECKS", "CARDS", "S%", "A%", "B%", "C%", "D%")
		fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 58))
		for _, br := range agg.ByBracket {
			pct := tierPercents(br.TierCounts, br.CardCount)
			fmt.Fprintf(w, "  B%d %-12s %5d %5d %5.1f %5.1f %5.1f %5.1f %5.1f\n",
				br.Bracket, br.Label, br.DeckCount, br.CardCount,
				pct["S"]*100, pct["A"]*100, pct["B"]*100, pct["C"]*100, pct["D"]*100)
		}
		fmt.Fprintf(w, "\n")
	}

	// Per-archetype breakdown.
	if len(agg.ByArchetype) > 0 {
		fmt.Fprintf(w, "By primary archetype:\n")
		fmt.Fprintf(w, "  %-20s %5s %5s %5s %5s %5s %5s %5s\n",
			"ARCHETYPE", "DECKS", "CARDS", "S%", "A%", "B%", "C%", "D%")
		fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 60))
		for _, ar := range agg.ByArchetype {
			pct := tierPercents(ar.TierCounts, ar.CardCount)
			fmt.Fprintf(w, "  %-20s %5d %5d %5.1f %5.1f %5.1f %5.1f %5.1f\n",
				ar.Archetype, ar.DeckCount, ar.CardCount,
				pct["S"]*100, pct["A"]*100, pct["B"]*100, pct["C"]*100, pct["D"]*100)
		}
		fmt.Fprintf(w, "\n")
	}

	// 5-point histogram with bar chart. Bar width capped at 40 chars
	// so a single dominant bin doesn't blow out the terminal width.
	fmt.Fprintf(w, "Score histogram (5-point bins):\n")
	maxBin := 0
	for _, b := range agg.ScoreHistogram {
		if b.Count > maxBin {
			maxBin = b.Count
		}
	}
	const barMax = 40
	for _, b := range agg.ScoreHistogram {
		barLen := 0
		if maxBin > 0 {
			barLen = b.Count * barMax / maxBin
		}
		fmt.Fprintf(w, "  [%3d-%3d] %5d (%4.1f%%) %s\n",
			b.RangeStart, b.RangeEnd, b.Count, b.Percent*100,
			strings.Repeat("#", barLen))
	}
	fmt.Fprintf(w, "\n")
}

// tierPercents is a small helper for converting a tier-count map +
// total into a percent-of-total map, with the same {S,A,B,C,D} keys.
func tierPercents(counts map[string]int, total int) map[string]float64 {
	out := map[string]float64{"S": 0, "A": 0, "B": 0, "C": 0, "D": 0}
	if total == 0 {
		return out
	}
	for tier, count := range counts {
		out[tier] = float64(count) / float64(total)
	}
	return out
}
