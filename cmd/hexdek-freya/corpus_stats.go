package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Corpus-wide statistics — cross-deck rollup that answers the headline
// "across all N analyzed decks, average bracket=3.2, most common
// archetype=Midrange (28%), average curve=3.4."
//
// Complements PowerTierAggregate (which calibrates the S/A/B/C/D card
// thresholds): CorpusStats summarizes the DECK-LEVEL signals so a user
// running --all-decks gets context for their own deck against the
// corpus mean ("your deck is bracket 3 in a corpus that averages 3.2 —
// you're typical").
// ---------------------------------------------------------------------------

// CorpusStats is the rollup. Slices are pre-sorted (distributions by
// DeckCount desc; bracket histogram by Bracket asc).
type CorpusStats struct {
	DeckCount int
	CardCount int // sum of TotalCards (sanity check, also lets consumers compute corpus-wide averages)

	// Headline averages.
	AvgBracket       float64 // mean across decks with Bracket > 0
	MedianBracket    int
	AvgCMC           float64
	AvgManaBaseScore float64 // A=5, B=4, C=3, D=2, F=1; "" decks excluded from the mean

	// Density averages (per deck).
	AvgRamp         float64
	AvgDraw         float64
	AvgTutors       float64 // nonland tutors
	AvgInteraction  float64 // removal + counterspells
	AvgWinLines     float64
	AvgClusters     float64 // synergy clusters
	AvgGameChangers float64

	// % of decks carrying these properties.
	PctWithInfiniteCombo   float64 // len(TrueInfinites) > 0
	PctWithDeterminedCombo float64 // len(Determined) > 0
	PctWithFinisher        float64 // len(Finishers) > 0
	PctWithBoardWipe       float64 // ≥1 board-wipe role
	PctWithTutors          float64 // ≥1 nonland tutor

	// Distributions.
	BracketDistribution    []BracketCount
	ArchetypeDistribution  []ArchetypeCount
	ManaGradeDistribution  []ManaGradeCount
	CurveShapeDistribution []CurveShapeCount

	// Most-common entries — readymade for the headline string.
	MostCommonArchetype  string
	MostCommonBracket    int
	MostCommonCurveShape string
}

// BracketCount is one row of the bracket histogram.
type BracketCount struct {
	Bracket   int
	Label     string // empty when no deck supplied one
	DeckCount int
	Percent   float64 // 0-100
}

// ArchetypeCount is one row of the archetype distribution, sorted
// by DeckCount desc in the parent aggregate.
type ArchetypeCount struct {
	Archetype string
	DeckCount int
	Percent   float64
}

// ManaGradeCount is one row of the mana-base-grade distribution.
type ManaGradeCount struct {
	Grade     string // "A" | "B" | "C" | "D" | "F" | "" (unscored)
	DeckCount int
	Percent   float64
}

// CurveShapeCount is one row of the curve-shape distribution
// (aggro / midrange / control / bimodal).
type CurveShapeCount struct {
	Shape     string
	DeckCount int
	Percent   float64
}

// ComputeCorpusStats rolls the deck reports into the headline summary.
// Empty / nil input returns a zero-value CorpusStats so JSON callers
// always get a parseable shape.
func ComputeCorpusStats(reports []*FreyaReport) *CorpusStats {
	cs := &CorpusStats{}
	if len(reports) == 0 {
		return cs
	}
	cs.DeckCount = len(reports)

	// Accumulators.
	var bracketSum, cmcSum, manaScoreSum float64
	var bracketSamples, cmcSamples, manaScoreSamples int
	var rampSum, drawSum, tutorSum, interactionSum, winlineSum, clusterSum, gcSum int

	bracketCounts := map[int]int{}
	bracketLabel := map[int]string{}
	archetypeCounts := map[string]int{}
	gradeCounts := map[string]int{}
	shapeCounts := map[string]int{}
	var bracketValues []int

	withInfinite, withDetermined, withFinisher, withWipe, withTutor := 0, 0, 0, 0, 0

	for _, r := range reports {
		if r == nil {
			continue
		}
		cs.CardCount += r.TotalCards
		if r.AvgCMC > 0 {
			cmcSum += r.AvgCMC
			cmcSamples++
		}
		shapeCounts[r.CurveShape]++

		// Combo presence (off the raw report, before Profile may be nil).
		if len(r.TrueInfinites) > 0 {
			withInfinite++
		}
		if len(r.Determined) > 0 {
			withDetermined++
		}
		if len(r.Finishers) > 0 {
			withFinisher++
		}
		if r.NonLandTutorCount > 0 {
			withTutor++
		}
		// Board wipe presence — role-driven (the Profile-level signal
		// comes through r.Roles, which exists even if Profile doesn't).
		if r.Roles != nil && r.Roles.RoleCounts[RoleBoardWipe] > 0 {
			withWipe++
		}

		tutorSum += r.NonLandTutorCount
		interactionSum += r.RemovalCount
		if r.Roles != nil {
			interactionSum += r.Roles.RoleCounts[RoleCounterspell]
		}

		dp := r.Profile
		if dp == nil {
			continue
		}
		if dp.Bracket > 0 {
			bracketSum += float64(dp.Bracket)
			bracketSamples++
			bracketCounts[dp.Bracket]++
			bracketValues = append(bracketValues, dp.Bracket)
			if dp.BracketLabel != "" && bracketLabel[dp.Bracket] == "" {
				bracketLabel[dp.Bracket] = dp.BracketLabel
			}
		}
		if dp.PrimaryArchetype != "" {
			archetypeCounts[dp.PrimaryArchetype]++
		}
		gradeCounts[dp.ManaBaseGrade]++
		if score, ok := manaGradeScore(dp.ManaBaseGrade); ok {
			manaScoreSum += float64(score)
			manaScoreSamples++
		}
		rampSum += dp.RampCount
		drawSum += dp.DrawCount
		winlineSum += dp.WinLineCount
		clusterSum += len(dp.SynergyClusters)
		gcSum += dp.GameChangerCount
	}

	// Averages — guard against div-by-zero by checking sample counts.
	if bracketSamples > 0 {
		cs.AvgBracket = bracketSum / float64(bracketSamples)
	}
	if cmcSamples > 0 {
		cs.AvgCMC = cmcSum / float64(cmcSamples)
	}
	if manaScoreSamples > 0 {
		cs.AvgManaBaseScore = manaScoreSum / float64(manaScoreSamples)
	}
	n := float64(cs.DeckCount)
	cs.AvgRamp = float64(rampSum) / n
	cs.AvgDraw = float64(drawSum) / n
	cs.AvgTutors = float64(tutorSum) / n
	cs.AvgInteraction = float64(interactionSum) / n
	cs.AvgWinLines = float64(winlineSum) / n
	cs.AvgClusters = float64(clusterSum) / n
	cs.AvgGameChangers = float64(gcSum) / n

	cs.PctWithInfiniteCombo = pct(withInfinite, cs.DeckCount)
	cs.PctWithDeterminedCombo = pct(withDetermined, cs.DeckCount)
	cs.PctWithFinisher = pct(withFinisher, cs.DeckCount)
	cs.PctWithBoardWipe = pct(withWipe, cs.DeckCount)
	cs.PctWithTutors = pct(withTutor, cs.DeckCount)

	if len(bracketValues) > 0 {
		sort.Ints(bracketValues)
		cs.MedianBracket = bracketValues[len(bracketValues)/2]
	}

	cs.BracketDistribution = buildBracketDistribution(bracketCounts, bracketLabel, cs.DeckCount)
	cs.ArchetypeDistribution = buildArchetypeDistribution(archetypeCounts, cs.DeckCount)
	cs.ManaGradeDistribution = buildGradeDistribution(gradeCounts, cs.DeckCount)
	cs.CurveShapeDistribution = buildShapeDistribution(shapeCounts, cs.DeckCount)

	if len(cs.ArchetypeDistribution) > 0 {
		cs.MostCommonArchetype = cs.ArchetypeDistribution[0].Archetype
	}
	if len(cs.BracketDistribution) > 0 {
		topBracket := cs.BracketDistribution[0]
		topCount := topBracket.DeckCount
		for _, b := range cs.BracketDistribution {
			if b.DeckCount > topCount {
				topBracket = b
				topCount = b.DeckCount
			}
		}
		cs.MostCommonBracket = topBracket.Bracket
	}
	if len(cs.CurveShapeDistribution) > 0 {
		cs.MostCommonCurveShape = cs.CurveShapeDistribution[0].Shape
	}
	return cs
}

// manaGradeScore maps the letter grade to a 1-5 numeric so the mean
// is meaningful. Unscored ("") returns ok=false so it's excluded from
// the mean rather than treated as a zero.
func manaGradeScore(grade string) (int, bool) {
	switch grade {
	case "A":
		return 5, true
	case "B":
		return 4, true
	case "C":
		return 3, true
	case "D":
		return 2, true
	case "F":
		return 1, true
	}
	return 0, false
}

func pct(n, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func buildBracketDistribution(counts map[int]int, labels map[int]string, deckCount int) []BracketCount {
	if len(counts) == 0 {
		return nil
	}
	out := make([]BracketCount, 0, len(counts))
	for b, c := range counts {
		out = append(out, BracketCount{
			Bracket:   b,
			Label:     labels[b],
			DeckCount: c,
			Percent:   pct(c, deckCount),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bracket < out[j].Bracket })
	return out
}

func buildArchetypeDistribution(counts map[string]int, deckCount int) []ArchetypeCount {
	if len(counts) == 0 {
		return nil
	}
	out := make([]ArchetypeCount, 0, len(counts))
	for a, c := range counts {
		out = append(out, ArchetypeCount{
			Archetype: a,
			DeckCount: c,
			Percent:   pct(c, deckCount),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DeckCount != out[j].DeckCount {
			return out[i].DeckCount > out[j].DeckCount
		}
		return out[i].Archetype < out[j].Archetype // alphabetical tiebreak for determinism
	})
	return out
}

func buildGradeDistribution(counts map[string]int, deckCount int) []ManaGradeCount {
	if len(counts) == 0 {
		return nil
	}
	// Canonical order so the text render shows A→F left-to-right with
	// "" (unscored) at the end.
	order := []string{"A", "B", "C", "D", "F", ""}
	out := make([]ManaGradeCount, 0, len(order))
	for _, grade := range order {
		if c, ok := counts[grade]; ok && c > 0 {
			out = append(out, ManaGradeCount{
				Grade:     grade,
				DeckCount: c,
				Percent:   pct(c, deckCount),
			})
		}
	}
	return out
}

func buildShapeDistribution(counts map[string]int, deckCount int) []CurveShapeCount {
	if len(counts) == 0 {
		return nil
	}
	out := make([]CurveShapeCount, 0, len(counts))
	for shape, c := range counts {
		if shape == "" {
			continue
		}
		out = append(out, CurveShapeCount{
			Shape:     shape,
			DeckCount: c,
			Percent:   pct(c, deckCount),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DeckCount != out[j].DeckCount {
			return out[i].DeckCount > out[j].DeckCount
		}
		return out[i].Shape < out[j].Shape
	})
	return out
}

// PrintCorpusStats renders the rollup as a multi-section text block,
// suitable for emission after PrintAllDecksSummary or as a standalone
// --all-decks summary.
func PrintCorpusStats(w io.Writer, cs *CorpusStats) {
	if cs == nil || cs.DeckCount == 0 {
		fmt.Fprintln(w, "Corpus Stats: no decks analyzed.")
		return
	}
	fmt.Fprintf(w, "\nCorpus Stats (across %d decks, %d total cards)\n", cs.DeckCount, cs.CardCount)
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 60))
	fmt.Fprintf(w, "  Headline:\n")
	fmt.Fprintf(w, "    Avg bracket    : %.1f (median %d, most common B%d)\n",
		cs.AvgBracket, cs.MedianBracket, cs.MostCommonBracket)
	fmt.Fprintf(w, "    Avg curve      : %.1f CMC (most common shape: %s)\n",
		cs.AvgCMC, cs.MostCommonCurveShape)
	fmt.Fprintf(w, "    Avg mana grade : %s (%.2f / 5.0)\n",
		manaGradeFromScore(cs.AvgManaBaseScore), cs.AvgManaBaseScore)
	fmt.Fprintf(w, "    Most common    : %s\n", cs.MostCommonArchetype)
	fmt.Fprintf(w, "\n  Density (per deck):\n")
	fmt.Fprintf(w, "    ramp         %.1f   draw           %.1f\n", cs.AvgRamp, cs.AvgDraw)
	fmt.Fprintf(w, "    tutors       %.1f   interaction    %.1f\n", cs.AvgTutors, cs.AvgInteraction)
	fmt.Fprintf(w, "    win lines    %.1f   clusters       %.1f\n", cs.AvgWinLines, cs.AvgClusters)
	fmt.Fprintf(w, "    game changers %.1f\n", cs.AvgGameChangers)

	fmt.Fprintf(w, "\n  Presence (%% of decks):\n")
	fmt.Fprintf(w, "    infinite combo %.0f%%   determined combo %.0f%%   finisher %.0f%%\n",
		cs.PctWithInfiniteCombo, cs.PctWithDeterminedCombo, cs.PctWithFinisher)
	fmt.Fprintf(w, "    board wipe %.0f%%       tutors %.0f%%\n",
		cs.PctWithBoardWipe, cs.PctWithTutors)

	if len(cs.BracketDistribution) > 0 {
		fmt.Fprintf(w, "\n  Bracket distribution:\n")
		for _, b := range cs.BracketDistribution {
			label := b.Label
			if label != "" {
				label = " " + label
			}
			fmt.Fprintf(w, "    B%d%-12s %4d (%.1f%%)\n", b.Bracket, label, b.DeckCount, b.Percent)
		}
	}
	if len(cs.ArchetypeDistribution) > 0 {
		fmt.Fprintf(w, "\n  Archetype distribution (top 10):\n")
		limit := 10
		if len(cs.ArchetypeDistribution) < limit {
			limit = len(cs.ArchetypeDistribution)
		}
		for i := 0; i < limit; i++ {
			a := cs.ArchetypeDistribution[i]
			fmt.Fprintf(w, "    %-22s %4d (%.1f%%)\n", a.Archetype, a.DeckCount, a.Percent)
		}
	}
	if len(cs.ManaGradeDistribution) > 0 {
		fmt.Fprintf(w, "\n  Mana grade distribution:\n")
		for _, g := range cs.ManaGradeDistribution {
			label := g.Grade
			if label == "" {
				label = "—" // unscored
			}
			fmt.Fprintf(w, "    %s  %4d (%.1f%%)\n", label, g.DeckCount, g.Percent)
		}
	}
	if len(cs.CurveShapeDistribution) > 0 {
		fmt.Fprintf(w, "\n  Curve shape distribution:\n")
		for _, c := range cs.CurveShapeDistribution {
			fmt.Fprintf(w, "    %-10s %4d (%.1f%%)\n", c.Shape, c.DeckCount, c.Percent)
		}
	}
	fmt.Fprintln(w)
}

// manaGradeFromScore inverts manaGradeScore for the headline render —
// "the corpus averages a B-grade mana base" reads better than "the
// corpus averages 3.8/5.0" alone.
func manaGradeFromScore(score float64) string {
	switch {
	case score >= 4.5:
		return "A"
	case score >= 3.5:
		return "B"
	case score >= 2.5:
		return "C"
	case score >= 1.5:
		return "D"
	case score > 0:
		return "F"
	}
	return "—"
}
