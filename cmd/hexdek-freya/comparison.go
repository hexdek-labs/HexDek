package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Deck-to-deck comparison report — structured diff between two completed
// deck analyses. Headline use case: "my Iroh vs another Iroh — what's the
// difference?" but works equally for cross-commander shells (e.g. two
// different Reanimator decks).
//
// Where similarity.go scores ONE number ("78% similar"), this produces
// a multi-dimensional diff document the Decks screen can render as a
// side-by-side table:
//
//   - archetype fit (same archetype? primary/secondary crossover?)
//   - bracket gap (which deck is tighter, by how much)
//   - win-line redundancy (count + primary line for each)
//   - card overlap split into shared core / unique-to-A / unique-to-B
//     (lands excluded — see similarity.go on why)
//   - density signals diff (ramp / draw / tutors / interaction /
//     board wipes / win lines / clusters) with signed deltas
//   - mana base grade comparison
//   - narrative verdict synthesizing the headline differences
// ---------------------------------------------------------------------------

// DeckComparison is the structured diff between two deck analyses.
// Fields named …A / …B refer to the first / second deck respectively;
// DensityDiff entries carry a signed Delta = ValueB − ValueA so positive
// means "B has more of this."
type DeckComparison struct {
	DeckA, DeckB string

	// SameCommander is true when both decks share a commander — the
	// natural "your Iroh vs another Iroh" framing. False for
	// cross-commander shells.
	SameCommander bool
	Commander     string // populated only when SameCommander is true

	// Card overlap (NONLAND only — see similarity.go on lands).
	SharedNonlandCount int
	OnlyInACount       int
	OnlyInBCount       int
	SharedSample       []string // up to 12 representative shared cards, alphabetical
	UniqueToA          []string // up to 10 standout cards in A but not B (from A's star/solid tier)
	UniqueToB          []string // up to 10 standout cards in B but not A (from B's star/solid tier)

	// Archetype fit.
	ArchetypeA, ArchetypeB string
	SecondaryA, SecondaryB string
	ArchetypeMatch         float64 // 1.0 same primary / 0.5 crossover / 0.0 disjoint

	// Bracket fit.
	BracketA, BracketB           int
	BracketLabelA, BracketLabelB string
	BracketDelta                 int // signed (B − A)

	// Win lines.
	WinLineCountA, WinLineCountB     int
	PrimaryWinLineA, PrimaryWinLineB string

	// Density signal deltas (ramp / draw / tutors / interaction / etc.)
	DensityDiff []DensityRow

	// Mana base.
	ManaGradeA, ManaGradeB string

	// Narrative verdict — a paragraph synthesizing the diff. Coach
	// surfaces this as the panel headline.
	Verdict string
}

// DensityRow is one comparison axis (e.g. "ramp", "interaction") with
// the signed delta. The Decks screen renders these as a side-by-side
// "A: 8 | B: 12 | +4 more interaction" row.
type DensityRow struct {
	Metric string // "ramp" | "draw" | "tutors" | "interaction" | "board_wipes" | "win_lines" | "clusters"
	ValueA int
	ValueB int
	Delta  int    // B − A
	Note   string // optional one-liner explaining the gap from B's perspective
}

// CompareDecks produces the diff report. Reports the report inputs so
// CompareDecks can pull the card lists (which live on report.Profiles,
// not on DeckProfile). Returns nil if either deck profile is nil.
func CompareDecks(a, b *DeckProfile, repA, repB *FreyaReport) *DeckComparison {
	if a == nil || b == nil {
		return nil
	}
	cmp := &DeckComparison{
		DeckA:          a.DeckName,
		DeckB:          b.DeckName,
		ArchetypeA:     a.PrimaryArchetype,
		ArchetypeB:     b.PrimaryArchetype,
		SecondaryA:     a.SecondaryArchetype,
		SecondaryB:     b.SecondaryArchetype,
		BracketA:       a.Bracket,
		BracketB:       b.Bracket,
		BracketLabelA:  a.BracketLabel,
		BracketLabelB:  b.BracketLabel,
		BracketDelta:   b.Bracket - a.Bracket,
		WinLineCountA:  a.WinLineCount,
		WinLineCountB:  b.WinLineCount,
		PrimaryWinLineA: a.PrimaryWinLine,
		PrimaryWinLineB: b.PrimaryWinLine,
		ManaGradeA:     a.ManaBaseGrade,
		ManaGradeB:     b.ManaBaseGrade,
	}

	if a.Commander != "" && a.Commander == b.Commander {
		cmp.SameCommander = true
		cmp.Commander = a.Commander
	}

	// Archetype affinity — reuse the similarity.go ladder by building
	// lightweight fingerprints. Cheaper than duplicating the logic.
	fpA := BuildDeckFingerprint(a, repA)
	fpB := BuildDeckFingerprint(b, repB)
	cmp.ArchetypeMatch = archetypeAffinity(fpA, fpB)

	// Card overlap from the fingerprints' nonland sets. ⏤ Cards lists
	// are alphabetically sorted for stability across runs.
	populateCardOverlap(cmp, fpA, fpB, a, b)

	// Density signals — pulled from report + profile.
	cmp.DensityDiff = buildDensityDiff(a, b, repA, repB)

	// Verdict last — needs every field above populated.
	cmp.Verdict = buildComparisonVerdict(cmp)
	return cmp
}

func populateCardOverlap(cmp *DeckComparison, fpA, fpB *DeckFingerprint, a, b *DeckProfile) {
	// Shared = A ∩ B; OnlyInA = A \ B; OnlyInB = B \ A.
	var shared, onlyA, onlyB []string
	for name := range fpA.NonlandCards {
		if fpB.NonlandCards[name] {
			shared = append(shared, name)
		} else {
			onlyA = append(onlyA, name)
		}
	}
	for name := range fpB.NonlandCards {
		if !fpA.NonlandCards[name] {
			onlyB = append(onlyB, name)
		}
	}
	cmp.SharedNonlandCount = len(shared)
	cmp.OnlyInACount = len(onlyA)
	cmp.OnlyInBCount = len(onlyB)

	sort.Strings(shared)
	if len(shared) > 12 {
		shared = shared[:12]
	}
	cmp.SharedSample = shared

	// For the unique lists, prefer cards from each deck's star/solid
	// tier so we surface MEANINGFUL uniques rather than 60 differences
	// in random goodstuff. Falls back to alphabetical-first when the
	// deck has no star/solid analysis.
	cmp.UniqueToA = selectStandoutUniques(onlyA, a)
	cmp.UniqueToB = selectStandoutUniques(onlyB, b)
}

func selectStandoutUniques(unique []string, dp *DeckProfile) []string {
	// Build a priority set: star cards (highest), then solid cards.
	// Lookup is case-insensitive against the lowercased unique list.
	priority := map[string]int{}
	for _, c := range dp.StarCards {
		priority[strings.ToLower(c.Name)] = 2
	}
	for _, c := range dp.SolidCards {
		k := strings.ToLower(c.Name)
		if priority[k] < 1 {
			priority[k] = 1
		}
	}
	// Sort the unique list: star (2) > solid (1) > other (0); within
	// each tier, alphabetical.
	sort.Slice(unique, func(i, j int) bool {
		pi, pj := priority[unique[i]], priority[unique[j]]
		if pi != pj {
			return pi > pj
		}
		return unique[i] < unique[j]
	})
	if len(unique) > 10 {
		unique = unique[:10]
	}
	return unique
}

func buildDensityDiff(a, b *DeckProfile, repA, repB *FreyaReport) []DensityRow {
	rows := []DensityRow{
		densityRow("ramp", a.RampCount, b.RampCount, "more ramp"),
		densityRow("draw", a.DrawCount, b.DrawCount, "more draw"),
		densityRow("tutors", nonlandTutors(repA), nonlandTutors(repB), "more tutors"),
		densityRow("interaction", interactionCount(repA), interactionCount(repB), "broader interaction suite"),
		densityRow("board_wipes", boardWipeCount(repA), boardWipeCount(repB), "more board wipes"),
		densityRow("win_lines", a.WinLineCount, b.WinLineCount, "more win lines"),
		densityRow("clusters", len(a.SynergyClusters), len(b.SynergyClusters), "more synergy clusters"),
	}
	return rows
}

func densityRow(metric string, valA, valB int, posLabel string) DensityRow {
	delta := valB - valA
	note := ""
	if delta > 0 {
		note = fmt.Sprintf("B has %s (%+d)", posLabel, delta)
	} else if delta < 0 {
		// Invert the framing: if B has fewer ramp pieces, A has more.
		note = fmt.Sprintf("A has %s (%+d for A)", posLabel, -delta)
	}
	return DensityRow{Metric: metric, ValueA: valA, ValueB: valB, Delta: delta, Note: note}
}

func nonlandTutors(r *FreyaReport) int {
	if r == nil {
		return 0
	}
	return r.NonLandTutorCount
}

func interactionCount(r *FreyaReport) int {
	if r == nil {
		return 0
	}
	out := r.RemovalCount
	if r.Roles != nil {
		out += r.Roles.RoleCounts[RoleCounterspell]
	}
	return out
}

func boardWipeCount(r *FreyaReport) int {
	if r == nil || r.Roles == nil {
		return 0
	}
	return r.Roles.RoleCounts[RoleBoardWipe]
}

// buildComparisonVerdict synthesizes the headline narrative — what
// makes A different from B. Tries to characterize each deck's edge
// rather than just listing every metric.
func buildComparisonVerdict(cmp *DeckComparison) string {
	// Opening clause — same-commander vs cross-commander framing.
	var open string
	if cmp.SameCommander {
		open = fmt.Sprintf("Two %s builds.", cmp.Commander)
	} else if cmp.ArchetypeMatch == 1.0 {
		open = fmt.Sprintf("Two %s decks across different commanders.", cmp.ArchetypeA)
	} else {
		open = fmt.Sprintf("%s vs %s — different archetypes entirely.", cmp.ArchetypeA, cmp.ArchetypeB)
	}

	// Bracket framing.
	var brk string
	switch {
	case cmp.BracketDelta == 0:
		brk = fmt.Sprintf("Both target bracket %d.", cmp.BracketA)
	case cmp.BracketDelta > 0:
		brk = fmt.Sprintf("%s is the higher-power build (B%d vs B%d).", cmp.DeckB, cmp.BracketB, cmp.BracketA)
	default:
		brk = fmt.Sprintf("%s is the higher-power build (B%d vs B%d).", cmp.DeckA, cmp.BracketA, cmp.BracketB)
	}

	// Edge framing — count significant deltas in each direction
	// (|Δ| ≥ 2 on density signals). The deck with more "wins" is
	// described as the tighter build.
	aWins, bWins := 0, 0
	var aEdges, bEdges []string
	for _, row := range cmp.DensityDiff {
		if row.Delta >= 2 {
			bWins++
			bEdges = append(bEdges, fmt.Sprintf("%+d %s", row.Delta, row.Metric))
		} else if row.Delta <= -2 {
			aWins++
			aEdges = append(aEdges, fmt.Sprintf("%+d %s", -row.Delta, row.Metric))
		}
	}

	var edges string
	switch {
	case aWins > 0 && bWins == 0:
		edges = fmt.Sprintf(" %s leads on %s.", cmp.DeckA, strings.Join(aEdges, ", "))
	case bWins > 0 && aWins == 0:
		edges = fmt.Sprintf(" %s leads on %s.", cmp.DeckB, strings.Join(bEdges, ", "))
	case aWins > 0 && bWins > 0:
		edges = fmt.Sprintf(" %s wins on %s; %s wins on %s.",
			cmp.DeckA, strings.Join(aEdges, ", "),
			cmp.DeckB, strings.Join(bEdges, ", "))
	default:
		edges = " Density signals are close."
	}

	overlap := fmt.Sprintf(" They share %d nonland cards (%s exclusive: %d, %s exclusive: %d).",
		cmp.SharedNonlandCount, cmp.DeckA, cmp.OnlyInACount, cmp.DeckB, cmp.OnlyInBCount)

	return fmt.Sprintf("%s %s%s%s", open, brk, edges, overlap)
}

// FormatDeckComparison renders the diff as a side-by-side text block
// suitable for the Decks-screen panel or CLI output.
func FormatDeckComparison(cmp *DeckComparison) string {
	if cmp == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Deck Comparison: %s  vs  %s\n", cmp.DeckA, cmp.DeckB)
	if cmp.SameCommander {
		fmt.Fprintf(&b, "  Commander: %s (both)\n", cmp.Commander)
	}
	fmt.Fprintf(&b, "\n  %s\n\n", cmp.Verdict)

	fmt.Fprintf(&b, "  Archetype:  %s%s  vs  %s%s  (match=%.1f)\n",
		cmp.ArchetypeA, secondaryClause(cmp.SecondaryA),
		cmp.ArchetypeB, secondaryClause(cmp.SecondaryB),
		cmp.ArchetypeMatch)
	fmt.Fprintf(&b, "  Bracket:    B%d %s  vs  B%d %s  (Δ %+d)\n",
		cmp.BracketA, cmp.BracketLabelA, cmp.BracketB, cmp.BracketLabelB, cmp.BracketDelta)
	fmt.Fprintf(&b, "  Mana Base:  %s  vs  %s\n", cmp.ManaGradeA, cmp.ManaGradeB)
	fmt.Fprintf(&b, "  Win Lines:  %d (%s)  vs  %d (%s)\n",
		cmp.WinLineCountA, truncateForLine(cmp.PrimaryWinLineA, 40),
		cmp.WinLineCountB, truncateForLine(cmp.PrimaryWinLineB, 40))

	fmt.Fprintf(&b, "\n  Density:\n")
	for _, row := range cmp.DensityDiff {
		marker := "  "
		if row.Delta > 0 {
			marker = "▲ "
		} else if row.Delta < 0 {
			marker = "▼ "
		}
		fmt.Fprintf(&b, "    %s%-13s A:%3d  B:%3d  (Δ %+d)\n",
			marker, row.Metric, row.ValueA, row.ValueB, row.Delta)
	}

	fmt.Fprintf(&b, "\n  Card Overlap: %d shared, %d only-A, %d only-B\n",
		cmp.SharedNonlandCount, cmp.OnlyInACount, cmp.OnlyInBCount)
	if len(cmp.UniqueToA) > 0 {
		fmt.Fprintf(&b, "    Only in %s: %s\n", cmp.DeckA, strings.Join(cmp.UniqueToA, ", "))
	}
	if len(cmp.UniqueToB) > 0 {
		fmt.Fprintf(&b, "    Only in %s: %s\n", cmp.DeckB, strings.Join(cmp.UniqueToB, ", "))
	}
	return b.String()
}

func secondaryClause(secondary string) string {
	if secondary == "" {
		return ""
	}
	return " / " + secondary
}

func truncateForLine(s string, max int) string {
	if s == "" {
		return "—"
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
