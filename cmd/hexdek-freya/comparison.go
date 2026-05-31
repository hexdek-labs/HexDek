package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Deck-to-deck comparison report — structured diff between two completed
// deck analyses. Headline use case: "my Atraxa vs another Atraxa — what's
// the difference?" but works equally for cross-commander shells of the
// same archetype.
//
// Where similarity.go reports a single Jaccard-style overlap %, this
// produces a multi-dimensional diff document the Decks-screen panel can
// render as a side-by-side table:
//
//   - power tier distribution (S/A/B/C/D mix per deck)
//   - win conditions (count + primary line for each)
//   - mana base (grade + land/ramp/tapland/fetch breakdown)
//   - tech card differences (what each deck runs that the other doesn't)
//   - card overlap (shared / unique-to-A / unique-to-B with star-tier sort)
//   - archetype + bracket framing
//
// Implementation re-uses BuildDeckFingerprint for card-set extraction
// and archetypeAffinity for the archetype-match ladder — single source
// of truth across the similarity stack.
// ---------------------------------------------------------------------------

// DeckComparison is the structured diff between two deck analyses.
// Fields named …A / …B refer to the first / second deck respectively;
// signed Delta fields use B−A so positive means "B has more."
type DeckComparison struct {
	DeckA, DeckB string

	// SameCommander is true when both decks share a commander — the
	// natural "your Atraxa vs another Atraxa" framing. False for
	// cross-commander shells.
	SameCommander bool
	Commander     string // populated only when SameCommander is true

	CommanderA, CommanderB string

	// Archetype framing.
	ArchetypeA, ArchetypeB string
	SecondaryA, SecondaryB string
	ArchetypeMatch         float64 // 1.0 same primary / 0.5 crossover / 0.0 disjoint

	// Bracket framing.
	BracketA, BracketB           int
	BracketLabelA, BracketLabelB string
	BracketDelta                 int // signed (B − A)

	// Power tier distribution. Each map carries S/A/B/C/D counts;
	// PowerTierDelta is the signed B−A per tier so the panel can render
	// "B has 4 more S-tier cards" inline.
	PowerTierA     map[string]int
	PowerTierB     map[string]int
	PowerTierDelta map[string]int

	// Win conditions.
	WinLineCountA, WinLineCountB     int
	PrimaryWinLineA, PrimaryWinLineB string
	WeightedWinLineA                 int
	WeightedWinLineB                 int

	// Mana base.
	ManaGradeA, ManaGradeB                     string
	LandCountA, LandCountB                     int
	RampCountA, RampCountB                     int
	TaplandCountA, TaplandCountB               int
	FetchCountA, FetchCountB                   int

	// Tech-card differences. CardName here is the card; the entries are
	// pulled from each deck's TechSuggestions list (only NEW suggestions
	// — AlreadyInDeck=true items are surfaced separately as confirmed
	// picks).
	TechOnlyInA []TechCardSuggestion // suggestions that A makes for its worst matchup that B doesn't
	TechOnlyInB []TechCardSuggestion

	// Card overlap (nonland only — lands dilute the signal, mirroring
	// similarity.go's rationale).
	SharedNonlandCount int
	OnlyInACount       int
	OnlyInBCount       int
	SharedSample       []string // up to 12 representative shared cards, alphabetical
	UniqueToA          []string // up to 10 standout cards in A but not B (star tier leads)
	UniqueToB          []string // up to 10 standout cards in B but not A

	// Verdict is the narrative paragraph synthesizing the diff.
	Verdict string
}

// Comparison output sizing constants. Tuned for terminal side-by-side
// render — UniqueTo lists capped at 10 cards each, SharedSample at 12.
const (
	comparisonUniqueToCap    = 10
	comparisonSharedCap      = 12
	comparisonStarCardWeight = 2 // star-tier cards sort to front in UniqueTo lists
)

// CompareDecks builds the structured diff. Safe to call with nil reports
// — the comparison degrades gracefully (no card-overlap section when
// either report is nil; no tech diff when either profile lacks
// TechSuggestions). Both DeckProfiles required.
func CompareDecks(a, b *DeckProfile, repA, repB *FreyaReport) *DeckComparison {
	if a == nil || b == nil {
		return nil
	}

	cmp := &DeckComparison{
		DeckA: a.DeckName,
		DeckB: b.DeckName,

		CommanderA: a.Commander,
		CommanderB: b.Commander,

		ArchetypeA: a.PrimaryArchetype,
		ArchetypeB: b.PrimaryArchetype,
		SecondaryA: a.SecondaryArchetype,
		SecondaryB: b.SecondaryArchetype,

		BracketA:      a.Bracket,
		BracketB:      b.Bracket,
		BracketLabelA: a.BracketLabel,
		BracketLabelB: b.BracketLabel,
		BracketDelta:  b.Bracket - a.Bracket,

		WinLineCountA:    a.WinLineCount,
		WinLineCountB:    b.WinLineCount,
		PrimaryWinLineA:  a.PrimaryWinLine,
		PrimaryWinLineB:  b.PrimaryWinLine,
		WeightedWinLineA: a.WeightedWinLineScore,
		WeightedWinLineB: b.WeightedWinLineScore,

		ManaGradeA:    a.ManaBaseGrade,
		ManaGradeB:    b.ManaBaseGrade,
		LandCountA:    a.LandCount,
		LandCountB:    b.LandCount,
		RampCountA:    a.RampCount,
		RampCountB:    b.RampCount,
		TaplandCountA: a.TaplandCount,
		TaplandCountB: b.TaplandCount,
		FetchCountA:   a.FetchCount,
		FetchCountB:   b.FetchCount,
	}

	if a.Commander != "" && strings.EqualFold(a.Commander, b.Commander) {
		cmp.SameCommander = true
		cmp.Commander = a.Commander
	}

	cmp.PowerTierA, cmp.PowerTierB, cmp.PowerTierDelta = comparePowerTiers(a, b)
	cmp.TechOnlyInA, cmp.TechOnlyInB = compareTechSuggestions(a, b)

	if repA != nil && repB != nil {
		fpA := BuildDeckFingerprint(a, repA)
		fpB := BuildDeckFingerprint(b, repB)
		cmp.ArchetypeMatch = archetypeAffinity(fpA, fpB)
		populateCardOverlap(cmp, a, b, fpA, fpB)
	} else {
		// Without reports we still want a meaningful affinity number —
		// fall back to a synthetic comparison built from the profiles
		// alone (archetypeAffinity inspects fingerprint fields the
		// profile can supply directly).
		cmp.ArchetypeMatch = archetypeAffinity(
			&DeckFingerprint{PrimaryArchetype: a.PrimaryArchetype, SecondaryArchetype: a.SecondaryArchetype},
			&DeckFingerprint{PrimaryArchetype: b.PrimaryArchetype, SecondaryArchetype: b.SecondaryArchetype},
		)
	}

	cmp.Verdict = buildComparisonVerdict(cmp)
	return cmp
}

// comparePowerTiers builds the per-tier S/A/B/C/D maps + signed deltas.
// Always returns a fully-keyed PowerTierDelta (all 5 tiers present,
// even if both decks have 0 of one) so callers can iterate without
// nil checks.
func comparePowerTiers(a, b *DeckProfile) (mapA, mapB, delta map[string]int) {
	mapA = map[string]int{"S": 0, "A": 0, "B": 0, "C": 0, "D": 0}
	mapB = map[string]int{"S": 0, "A": 0, "B": 0, "C": 0, "D": 0}
	for k, v := range a.PowerTierCounts {
		mapA[k] = v
	}
	for k, v := range b.PowerTierCounts {
		mapB[k] = v
	}
	delta = make(map[string]int, 5)
	for _, tier := range PowerTierOrder {
		delta[tier] = mapB[tier] - mapA[tier]
	}
	return
}

// compareTechSuggestions partitions each deck's tech suggestions into
// "only in A's list" and "only in B's list", keyed by card name
// (case-insensitive). Suggestions present in both — the natural
// overlap when both decks share a worst matchup — drop out of both
// returned lists since they're not a difference. AlreadyInDeck status
// flows through unchanged.
func compareTechSuggestions(a, b *DeckProfile) (onlyA, onlyB []TechCardSuggestion) {
	byCardB := map[string]bool{}
	for _, s := range b.TechSuggestions {
		byCardB[strings.ToLower(s.Card)] = true
	}
	byCardA := map[string]bool{}
	for _, s := range a.TechSuggestions {
		byCardA[strings.ToLower(s.Card)] = true
	}
	for _, s := range a.TechSuggestions {
		if !byCardB[strings.ToLower(s.Card)] {
			onlyA = append(onlyA, s)
		}
	}
	for _, s := range b.TechSuggestions {
		if !byCardA[strings.ToLower(s.Card)] {
			onlyB = append(onlyB, s)
		}
	}
	return
}

// populateCardOverlap fills SharedNonlandCount / OnlyInACount /
// OnlyInBCount / SharedSample / UniqueToA / UniqueToB from the
// fingerprints. Star-tier cards lead the UniqueTo lists so the most
// signal-bearing differences surface first.
func populateCardOverlap(cmp *DeckComparison, a, b *DeckProfile, fpA, fpB *DeckFingerprint) {
	shared := []string{}
	onlyA := []string{}
	onlyB := []string{}

	for nameLower := range fpA.NonlandCards {
		if fpB.NonlandCards[nameLower] {
			shared = append(shared, nameLower)
		} else {
			onlyA = append(onlyA, nameLower)
		}
	}
	for nameLower := range fpB.NonlandCards {
		if !fpA.NonlandCards[nameLower] {
			onlyB = append(onlyB, nameLower)
		}
	}

	cmp.SharedNonlandCount = len(shared)
	cmp.OnlyInACount = len(onlyA)
	cmp.OnlyInBCount = len(onlyB)

	sort.Strings(shared)
	if len(shared) > comparisonSharedCap {
		shared = shared[:comparisonSharedCap]
	}
	cmp.SharedSample = titleCase(shared)

	cmp.UniqueToA = standoutUniques(onlyA, a)
	cmp.UniqueToB = standoutUniques(onlyB, b)
}

// standoutUniques sorts unique-to-this-deck cards with star-tier
// cards leading, then alphabetical. Returns the original-cased card
// names (looked up via dp.StarCards / dp.CardPowerLevels for casing;
// falls back to title-case if not found). Capped at the configured
// limit.
func standoutUniques(lowerNames []string, dp *DeckProfile) []string {
	// Build a star-tier lookup so we can sort star-tier cards first.
	starSet := map[string]bool{}
	for _, c := range dp.StarCards {
		starSet[strings.ToLower(c.Name)] = true
	}
	// Original-case lookup from profiles' card lists (StarCards +
	// CardPowerLevels cover the names we want to surface).
	originalCase := map[string]string{}
	for _, c := range dp.StarCards {
		originalCase[strings.ToLower(c.Name)] = c.Name
	}
	for _, c := range dp.CardPowerLevels {
		originalCase[strings.ToLower(c.Name)] = c.Name
	}

	sort.SliceStable(lowerNames, func(i, j int) bool {
		aStar := starSet[lowerNames[i]]
		bStar := starSet[lowerNames[j]]
		if aStar != bStar {
			return aStar
		}
		return lowerNames[i] < lowerNames[j]
	})

	if len(lowerNames) > comparisonUniqueToCap {
		lowerNames = lowerNames[:comparisonUniqueToCap]
	}
	out := make([]string, 0, len(lowerNames))
	for _, n := range lowerNames {
		if orig, ok := originalCase[n]; ok {
			out = append(out, orig)
			continue
		}
		out = append(out, titleCaseWord(n))
	}
	return out
}

func titleCase(lower []string) []string {
	out := make([]string, 0, len(lower))
	for _, n := range lower {
		out = append(out, titleCaseWord(n))
	}
	return out
}

func titleCaseWord(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Fields(s)
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// buildComparisonVerdict synthesizes a narrative paragraph from the
// structured diff. Opens with a shared-context frame (same commander
// vs. cross-commander same-archetype vs. different archetypes),
// states the bracket leader, names the headline diff axes, and
// closes with the card-overlap summary.
func buildComparisonVerdict(cmp *DeckComparison) string {
	var sb strings.Builder

	// Opening frame.
	switch {
	case cmp.SameCommander:
		sb.WriteString(fmt.Sprintf("Two %s builds. ", cmp.Commander))
	case strings.EqualFold(cmp.ArchetypeA, cmp.ArchetypeB) && cmp.ArchetypeA != "":
		sb.WriteString(fmt.Sprintf("Two %s decks across different commanders. ", cmp.ArchetypeA))
	default:
		archA := cmp.ArchetypeA
		archB := cmp.ArchetypeB
		if archA == "" {
			archA = "Unclassified"
		}
		if archB == "" {
			archB = "Unclassified"
		}
		sb.WriteString(fmt.Sprintf("%s vs %s — different archetypes entirely. ", archA, archB))
	}

	// Bracket framing.
	switch {
	case cmp.BracketDelta > 0:
		sb.WriteString(fmt.Sprintf("%s is the higher-power build (bracket %d vs %d). ",
			cmp.deckLabelB(), cmp.BracketB, cmp.BracketA))
	case cmp.BracketDelta < 0:
		sb.WriteString(fmt.Sprintf("%s is the higher-power build (bracket %d vs %d). ",
			cmp.deckLabelA(), cmp.BracketA, cmp.BracketB))
	default:
		sb.WriteString(fmt.Sprintf("Both target bracket %d. ", cmp.BracketA))
	}

	// Power-tier diff: which side has more S+A combined? Strong signal
	// of which build is more optimized at the card level.
	sDeltaA := cmp.PowerTierA["S"] + cmp.PowerTierA["A"]
	sDeltaB := cmp.PowerTierB["S"] + cmp.PowerTierB["A"]
	if d := sDeltaB - sDeltaA; intAbs(d) >= 3 {
		if d > 0 {
			sb.WriteString(fmt.Sprintf("%s runs %d more S+A-tier cards. ", cmp.deckLabelB(), d))
		} else {
			sb.WriteString(fmt.Sprintf("%s runs %d more S+A-tier cards. ", cmp.deckLabelA(), -d))
		}
	}

	// Win-line framing.
	if d := cmp.WinLineCountB - cmp.WinLineCountA; intAbs(d) >= 2 {
		if d > 0 {
			sb.WriteString(fmt.Sprintf("%s has %d more win lines. ", cmp.deckLabelB(), d))
		} else {
			sb.WriteString(fmt.Sprintf("%s has %d more win lines. ", cmp.deckLabelA(), -d))
		}
	}

	// Tech-card framing.
	if len(cmp.TechOnlyInA) > 0 && len(cmp.TechOnlyInB) > 0 {
		sb.WriteString(fmt.Sprintf("Tech picks diverge: %d unique adds on each side. ",
			intMin(len(cmp.TechOnlyInA), len(cmp.TechOnlyInB))))
	} else if len(cmp.TechOnlyInA) > 0 {
		sb.WriteString(fmt.Sprintf("%s runs %d unique tech adds the other build skips. ",
			cmp.deckLabelA(), len(cmp.TechOnlyInA)))
	} else if len(cmp.TechOnlyInB) > 0 {
		sb.WriteString(fmt.Sprintf("%s runs %d unique tech adds the other build skips. ",
			cmp.deckLabelB(), len(cmp.TechOnlyInB)))
	}

	// Card-overlap close.
	if cmp.SharedNonlandCount > 0 || cmp.OnlyInACount > 0 || cmp.OnlyInBCount > 0 {
		sb.WriteString(fmt.Sprintf("%d shared nonlands, %d unique to %s, %d unique to %s.",
			cmp.SharedNonlandCount,
			cmp.OnlyInACount, cmp.deckLabelA(),
			cmp.OnlyInBCount, cmp.deckLabelB()))
	}

	return strings.TrimSpace(sb.String())
}

// deckLabelA / deckLabelB produce a short, reader-friendly label for
// the verdict text. Prefers the deck name; falls back to "Deck A" /
// "Deck B" for partially-populated fixtures.
func (c *DeckComparison) deckLabelA() string {
	if c.DeckA != "" {
		return c.DeckA
	}
	return "Deck A"
}

func (c *DeckComparison) deckLabelB() string {
	if c.DeckB != "" {
		return c.DeckB
	}
	return "Deck B"
}

// FormatDeckComparison renders the structured diff as side-by-side
// text suitable for terminal output. Width-budgeted to ~80 columns:
// label (16) + " | " (3) + 30 + " | " (3) + 30 = 82. Each numeric
// metric carries a Δ marker (▲/▼/=) so scanning the table reveals
// the direction of each axis at a glance.
func FormatDeckComparison(cmp *DeckComparison) string {
	if cmp == nil {
		return ""
	}
	var sb strings.Builder

	sb.WriteString("FREYA -- Deck Comparison\n")
	sb.WriteString("==========================\n")
	if cmp.Verdict != "" {
		sb.WriteString(cmp.Verdict)
		sb.WriteString("\n\n")
	}

	row := func(label, a, b string) {
		sb.WriteString(fmt.Sprintf("  %-18s %-30s | %-30s\n",
			label+":", trimTo(a, 30), trimTo(b, 30)))
	}
	deltaInt := func(label string, a, b int) {
		delta := b - a
		marker := "="
		if delta > 0 {
			marker = fmt.Sprintf("▲ +%d (to B)", delta)
		} else if delta < 0 {
			marker = fmt.Sprintf("▼ %d (to A)", delta)
		}
		sb.WriteString(fmt.Sprintf("  %-18s %-30s | %-30s  %s\n",
			label+":", fmt.Sprintf("%d", a), fmt.Sprintf("%d", b), marker))
	}

	row("Deck", cmp.deckLabelA(), cmp.deckLabelB())
	row("Commander", cmp.CommanderA, cmp.CommanderB)
	row("Archetype", cmp.ArchetypeA, cmp.ArchetypeB)
	row("Bracket", fmt.Sprintf("%d %s", cmp.BracketA, cmp.BracketLabelA),
		fmt.Sprintf("%d %s", cmp.BracketB, cmp.BracketLabelB))
	sb.WriteString("\n")

	sb.WriteString("  Power Tier mix\n")
	for _, tier := range PowerTierOrder {
		deltaInt("    "+tier, cmp.PowerTierA[tier], cmp.PowerTierB[tier])
	}
	sb.WriteString("\n")

	sb.WriteString("  Win Conditions\n")
	deltaInt("    count", cmp.WinLineCountA, cmp.WinLineCountB)
	deltaInt("    weighted", cmp.WeightedWinLineA, cmp.WeightedWinLineB)
	if cmp.PrimaryWinLineA != "" || cmp.PrimaryWinLineB != "" {
		row("    primary", cmp.PrimaryWinLineA, cmp.PrimaryWinLineB)
	}
	sb.WriteString("\n")

	sb.WriteString("  Mana Base\n")
	row("    grade", cmp.ManaGradeA, cmp.ManaGradeB)
	deltaInt("    lands", cmp.LandCountA, cmp.LandCountB)
	deltaInt("    ramp", cmp.RampCountA, cmp.RampCountB)
	deltaInt("    taplands", cmp.TaplandCountA, cmp.TaplandCountB)
	deltaInt("    fetches", cmp.FetchCountA, cmp.FetchCountB)
	sb.WriteString("\n")

	if len(cmp.TechOnlyInA) > 0 || len(cmp.TechOnlyInB) > 0 {
		sb.WriteString("  Tech Card Differences\n")
		if len(cmp.TechOnlyInA) > 0 {
			sb.WriteString(fmt.Sprintf("    only in %s: ", cmp.deckLabelA()))
			sb.WriteString(strings.Join(techCardNames(cmp.TechOnlyInA), ", "))
			sb.WriteString("\n")
		}
		if len(cmp.TechOnlyInB) > 0 {
			sb.WriteString(fmt.Sprintf("    only in %s: ", cmp.deckLabelB()))
			sb.WriteString(strings.Join(techCardNames(cmp.TechOnlyInB), ", "))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if cmp.SharedNonlandCount > 0 || cmp.OnlyInACount > 0 || cmp.OnlyInBCount > 0 {
		sb.WriteString("  Card Overlap (nonland)\n")
		sb.WriteString(fmt.Sprintf("    shared:        %d cards\n", cmp.SharedNonlandCount))
		sb.WriteString(fmt.Sprintf("    only in %s: %d cards\n", cmp.deckLabelA(), cmp.OnlyInACount))
		sb.WriteString(fmt.Sprintf("    only in %s: %d cards\n", cmp.deckLabelB(), cmp.OnlyInBCount))
		if len(cmp.UniqueToA) > 0 {
			sb.WriteString(fmt.Sprintf("    standouts in %s: %s\n", cmp.deckLabelA(),
				strings.Join(cmp.UniqueToA, ", ")))
		}
		if len(cmp.UniqueToB) > 0 {
			sb.WriteString(fmt.Sprintf("    standouts in %s: %s\n", cmp.deckLabelB(),
				strings.Join(cmp.UniqueToB, ", ")))
		}
	}

	return sb.String()
}

func techCardNames(suggestions []TechCardSuggestion) []string {
	out := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		name := s.Card
		if s.AlreadyInDeck {
			name += " ✓"
		}
		out = append(out, name)
	}
	return out
}

func trimTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func intAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
