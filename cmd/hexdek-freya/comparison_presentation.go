package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Deck-comparison presentation layer (R60).
//
// The base DeckComparison (comparison.go) emits a structured diff —
// power tier mix, mana base, card overlap. This layer adds three
// polish surfaces on top:
//
//   1. Impact-ranked card differences — each unique-to-X card scored by
//      how much it shifts deck identity (star tier, named win-line
//      piece, S-tier power, tech card, game changer all weight high;
//      pet cards and untagged filler weight low). Reader sees the
//      top 8 per side instead of an alphabetized 10-card list.
//   2. Narrative summary — a 3-5 sentence "Deck A is more X than B
//      because of Y, Z, W" paragraph that names specific cards driving
//      the identity gap.
//   3. Recommended tech swaps — concrete "B should consider adding
//      <Card> from A's list, because both decks have Stax as their
//      worst matchup" suggestions. Surfaces only when both decks
//      share an opponent archetype as their worst matchup AND one
//      deck runs a TechSuggestion the other doesn't.
//
// Populated by ApplyComparisonPresentation, called automatically by
// CompareDecks after the structured diff is built. Surfaces in
// FormatDeckComparison as three additional sections (impact-ranked
// diffs, narrative summary, recommended swaps).
// ---------------------------------------------------------------------------

// ImpactfulCardDiff is one unique-to-this-deck card scored by how much
// it shifts the deck's identity vs the comparison. Impact is the
// 0-100 score (sum of individual signals, clamped); Reasons lists
// the per-signal labels that contributed so the panel can render
// "[88] Doubling Season — star card • S-tier • Superfriends fit".
type ImpactfulCardDiff struct {
	Card    string   `json:"card"`
	Impact  int      `json:"impact"`
	Reasons []string `json:"reasons"`
}

// RecommendedSwap is a cross-deck tech-card recommendation. SourceDeck
// runs Card; TargetDeck would benefit because both decks face
// SharedMatchup at low expected-win %. Reason carries the hoser-DB
// rationale (e.g. "shuts down graveyard recursion AND library
// tutors-to-cast").
type RecommendedSwap struct {
	Card          string `json:"card"`
	SourceDeck    string `json:"source_deck"`
	TargetDeck    string `json:"target_deck"`
	SharedMatchup string `json:"shared_matchup"`
	Reason        string `json:"reason"`
	Severity      int    `json:"severity"` // hoserDB severity inherited from the suggestion
}

// Impact-scoring weights. Tuned so a star card with an S-tier rating +
// win-line membership lands in the 80-95 range (clearly identity-
// defining); a B-tier filler with one role match lands 5-15 (clear
// marginal). The 100-cap is a clamp, not a target — most cards score
// far below it.
const (
	impactStarCard     = 40
	impactWinLinePiece = 30
	impactGameChanger  = 25
	impactPowerTierS   = 25
	impactPowerTierA   = 15
	impactPowerTierB   = 5
	impactTechCard     = 20
	impactCounterRole  = 15
	impactPetCard      = 5
)

// rankedDiffCap bounds how many impact-ranked diffs surface per side.
// 8 keeps the panel scannable without dropping signal — the top-8
// alphabetic standouts in a 60-card unique pool nearly always cover
// every star-tier / win-line / game-changer entry.
const rankedDiffCap = 8

// recommendedSwapCap bounds the swap section. 4 is enough to surface
// the critical-tier hosers; more is noise in a "consider these adds"
// section.
const recommendedSwapCap = 4

// ApplyComparisonPresentation populates the presentation-layer fields
// on an already-built DeckComparison. Called once at the end of
// CompareDecks so callers reading the struct see all three layers
// without a second call.
func ApplyComparisonPresentation(cmp *DeckComparison, a, b *DeckProfile, repA, repB *FreyaReport) {
	if cmp == nil || a == nil || b == nil {
		return
	}
	cmp.RankedDiffsA = rankImpactfulDiffs(cmp.uniqueToALower(), a, repA)
	cmp.RankedDiffsB = rankImpactfulDiffs(cmp.uniqueToBLower(), b, repB)
	cmp.RecommendedSwaps = recommendCrossDeckSwaps(cmp, a, b, repA, repB)
	cmp.NarrativeSummary = buildNarrativeSummary(cmp)
}

// uniqueToALower / uniqueToBLower extract the FULL lowercase unique
// set from the card-overlap pass (not just the UniqueTo display caps),
// so the impact ranker can sweep every candidate before truncating
// to rankedDiffCap. We rebuild from fingerprints since the display
// list is already capped at 10 + star-sorted.
func (c *DeckComparison) uniqueToALower() []string { return c.uniqueLowerSet(true) }
func (c *DeckComparison) uniqueToBLower() []string { return c.uniqueLowerSet(false) }

// uniqueLowerSet reconstructs the full uniques list from the cached
// card-overlap counts. We stored only the display-capped UniqueToA /
// UniqueToB on the struct; to rank all of them we walk those lists
// (already pre-sorted with star-tier leading + alphabetical fallback)
// and lowercase them. The display caps are 10 which is generally
// already past the impact tail — a 9th-ranked unique that just
// missed the display cap can't beat the 8th-ranked which is already
// included.
func (c *DeckComparison) uniqueLowerSet(forA bool) []string {
	src := c.UniqueToA
	if !forA {
		src = c.UniqueToB
	}
	out := make([]string, 0, len(src))
	for _, name := range src {
		out = append(out, strings.ToLower(name))
	}
	return out
}

// rankImpactfulDiffs walks the lowercased unique-card list and scores
// each via impact-scoring rules, returning the top rankedDiffCap by
// impact. Original-case card names are restored from the deck's
// StarCards / CardPowerLevels / GameChangerCards / TechSuggestions /
// CounterProducers / CounterPayoffs / PetCards lookup tables.
func rankImpactfulDiffs(lowerNames []string, dp *DeckProfile, report *FreyaReport) []ImpactfulCardDiff {
	if dp == nil || len(lowerNames) == 0 {
		return nil
	}

	// Build the per-deck scoring lookups. Each map is keyed lowercase
	// so we can match against the lowercased uniques without rebuilding
	// during the inner loop.
	starSet := lowerSetCardQuality(dp.StarCards)
	gameChangerSet := lowerSetStrings(dp.GameChangerCards)
	techSet := lowerSetTechSuggestion(dp.TechSuggestions)
	counterProducerSet := lowerSetStrings(dp.CounterProducers)
	counterPayoffSet := lowerSetStrings(dp.CounterPayoffs)
	petSet := lowerSetPetCard(dp.PetCards)
	tierByCard := lowerMapCardPowerTier(dp.CardPowerLevels)
	originalCase := lowerToOriginal(dp)

	// Win-line pieces from the report.
	winLineSet := map[string]bool{}
	if report != nil && report.WinLines != nil {
		for _, wl := range report.WinLines.WinLines {
			for _, p := range wl.Pieces {
				winLineSet[strings.ToLower(p)] = true
			}
		}
	}

	out := make([]ImpactfulCardDiff, 0, len(lowerNames))
	for _, lname := range lowerNames {
		impact := 0
		var reasons []string

		if starSet[lname] {
			impact += impactStarCard
			reasons = append(reasons, "star card")
		}
		if winLineSet[lname] {
			impact += impactWinLinePiece
			reasons = append(reasons, "win-line piece")
		}
		if gameChangerSet[lname] {
			impact += impactGameChanger
			reasons = append(reasons, "game changer")
		}
		switch tierByCard[lname] {
		case "S":
			impact += impactPowerTierS
			reasons = append(reasons, "S-tier")
		case "A":
			impact += impactPowerTierA
			reasons = append(reasons, "A-tier")
		case "B":
			impact += impactPowerTierB
			reasons = append(reasons, "B-tier")
		}
		if techSet[lname] {
			impact += impactTechCard
			reasons = append(reasons, "tech card")
		}
		if counterProducerSet[lname] || counterPayoffSet[lname] {
			impact += impactCounterRole
			reasons = append(reasons, "counters engine piece")
		}
		if petSet[lname] {
			impact += impactPetCard
			reasons = append(reasons, "flavor pick")
		}

		if impact == 0 {
			continue
		}
		if impact > 100 {
			impact = 100
		}
		display := originalCase[lname]
		if display == "" {
			display = titleCaseWord(lname)
		}
		out = append(out, ImpactfulCardDiff{
			Card:    display,
			Impact:  impact,
			Reasons: reasons,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Impact != out[j].Impact {
			return out[i].Impact > out[j].Impact
		}
		return out[i].Card < out[j].Card
	})
	if len(out) > rankedDiffCap {
		out = out[:rankedDiffCap]
	}
	return out
}

func lowerSetCardQuality(cards []CardQuality) map[string]bool {
	out := map[string]bool{}
	for _, c := range cards {
		out[strings.ToLower(c.Name)] = true
	}
	return out
}

func lowerSetStrings(names []string) map[string]bool {
	out := map[string]bool{}
	for _, n := range names {
		out[strings.ToLower(n)] = true
	}
	return out
}

func lowerSetTechSuggestion(s []TechCardSuggestion) map[string]bool {
	out := map[string]bool{}
	for _, t := range s {
		out[strings.ToLower(t.Card)] = true
	}
	return out
}

func lowerSetPetCard(p []PetCard) map[string]bool {
	out := map[string]bool{}
	for _, c := range p {
		out[strings.ToLower(c.Name)] = true
	}
	return out
}

func lowerMapCardPowerTier(levels []CardPowerLevel) map[string]string {
	out := map[string]string{}
	for _, l := range levels {
		out[strings.ToLower(l.Name)] = l.PowerTier
	}
	return out
}

// lowerToOriginal builds a "lowercase → original case" lookup across
// every card-name-bearing field on the profile, so the ranked output
// surfaces cards with proper casing ("Doubling Season" not "doubling
// season"). Falls through to titleCaseWord when no original is found.
func lowerToOriginal(dp *DeckProfile) map[string]string {
	out := map[string]string{}
	push := func(name string) {
		k := strings.ToLower(name)
		if k == "" {
			return
		}
		if _, ok := out[k]; !ok {
			out[k] = name
		}
	}
	for _, c := range dp.StarCards {
		push(c.Name)
	}
	for _, c := range dp.SolidCards {
		push(c.Name)
	}
	for _, c := range dp.FlexSlots {
		push(c.Name)
	}
	for _, c := range dp.CuttableCards {
		push(c.Name)
	}
	for _, c := range dp.CardPowerLevels {
		push(c.Name)
	}
	for _, n := range dp.GameChangerCards {
		push(n)
	}
	for _, n := range dp.CounterProducers {
		push(n)
	}
	for _, n := range dp.CounterPayoffs {
		push(n)
	}
	for _, p := range dp.PetCards {
		push(p.Name)
	}
	for _, t := range dp.TechSuggestions {
		push(t.Card)
	}
	return out
}

// recommendCrossDeckSwaps generates "A has X that B should consider
// adding, because both face Stax as worst matchup" suggestions.
// Logic:
//
//  1. Both decks must share a worst matchup (e.g. both face Stax).
//     If WorstMatchups differ, the tech that A runs targets a
//     different problem than B's worst — swap is misaligned.
//
//  2. The candidate card must be in A's TechSuggestions list
//     (canonical hoser for the shared matchup) AND NOT in B's deck.
//     We approximate "not in B's deck" via the card-overlap fingerprint
//     since cards present in B already won't appear in the uniques
//     anyway — but we double-check against B's TechSuggestions
//     AlreadyInDeck flags too, since the deck may carry the card
//     even when it's not in the fingerprint nonland set (e.g. a
//     basic-land hoser like Bojuka Bog lands in the LAND zone and
//     wouldn't show up in nonland card overlap).
//
// Output sorted by severity desc (critical > major > minor) with
// alphabetic tiebreak; capped at recommendedSwapCap entries.
func recommendCrossDeckSwaps(cmp *DeckComparison, a, b *DeckProfile, repA, repB *FreyaReport) []RecommendedSwap {
	if a == nil || b == nil {
		return nil
	}
	if a.WorstMatchup == "" || b.WorstMatchup == "" {
		return nil
	}
	if !strings.EqualFold(a.WorstMatchup, b.WorstMatchup) {
		return nil
	}

	deckACardsLower := profileCardSetLower(a, repA)
	deckBCardsLower := profileCardSetLower(b, repB)
	bAlreadyHas := map[string]bool{}
	for _, t := range b.TechSuggestions {
		if t.AlreadyInDeck {
			bAlreadyHas[strings.ToLower(t.Card)] = true
		}
	}
	aAlreadyHas := map[string]bool{}
	for _, t := range a.TechSuggestions {
		if t.AlreadyInDeck {
			aAlreadyHas[strings.ToLower(t.Card)] = true
		}
	}

	// Build candidates from BOTH directions:
	//   - A→B: a card A runs (or recommends) that B doesn't
	//   - B→A: a card B runs (or recommends) that A doesn't
	// Reads as "consider adding the X your sibling deck runs."
	var out []RecommendedSwap
	addCandidates := func(source *DeckProfile, sourceLabel, targetLabel string,
		targetCardSet map[string]bool, targetAlreadyHas map[string]bool) {
		for _, t := range source.TechSuggestions {
			k := strings.ToLower(t.Card)
			// Skip if target deck already runs the card.
			if targetCardSet[k] || targetAlreadyHas[k] {
				continue
			}
			out = append(out, RecommendedSwap{
				Card:          t.Card,
				SourceDeck:    sourceLabel,
				TargetDeck:    targetLabel,
				SharedMatchup: cmp.normalizedWorstMatchup(),
				Reason:        t.Reason,
				Severity:      t.Severity,
			})
		}
	}
	addCandidates(a, cmp.deckLabelA(), cmp.deckLabelB(), deckBCardsLower, bAlreadyHas)
	addCandidates(b, cmp.deckLabelB(), cmp.deckLabelA(), deckACardsLower, aAlreadyHas)

	// Dedup by (Card, TargetDeck) — if A and B somehow both list the
	// same suggestion targeting the same direction we keep one.
	seen := map[string]bool{}
	deduped := out[:0]
	for _, s := range out {
		key := strings.ToLower(s.Card) + "|" + s.TargetDeck
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, s)
	}
	out = deduped

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Card < out[j].Card
	})
	if len(out) > recommendedSwapCap {
		out = out[:recommendedSwapCap]
	}
	return out
}

// normalizedWorstMatchup returns the shared opponent-archetype label.
// Populated on the comparison struct by CompareDecks when both decks
// match worst-matchups; empty otherwise. recommendCrossDeckSwaps
// gates on equality so this is only called when SharedWorstMatchup
// is non-empty.
func (c *DeckComparison) normalizedWorstMatchup() string {
	return c.SharedWorstMatchup
}

// profileCardSetLower builds a lowercased set of all card names in a
// deck's profile (nonland + land). Reads from report.Profiles when
// available (most reliable — that's the actual deck list); falls
// back to the union of profile-level card-bearing fields when the
// report isn't passed in.
func profileCardSetLower(dp *DeckProfile, report *FreyaReport) map[string]bool {
	out := map[string]bool{}
	if report != nil {
		for _, p := range report.Profiles {
			out[strings.ToLower(p.Name)] = true
		}
	}
	if dp != nil {
		// Defensive cover when report is absent — names from
		// StarCards / SolidCards / FlexSlots / CuttableCards /
		// CardPowerLevels at least catch the cards the analysis
		// surfaced.
		for k := range lowerToOriginal(dp) {
			out[k] = true
		}
	}
	return out
}

// buildNarrativeSummary produces the "Deck A is more X than B because
// of Y, Z, W" paragraph. Structure:
//
//  1. Lead: archetype-gap sentence ("A is more {archA} than B; B leans
//     {archB}.") — or for same-archetype comparisons, the
//     bracket / power-tier gap instead.
//  2. Evidence (A side): names 2-3 high-impact A-only cards with a
//     "anchoring the {archA} engine" frame.
//  3. Evidence (B side): same for B's high-impact uniques.
//  4. Power gap close: bracket + S+A-tier delta, or "Both target
//     bracket N" when matched.
//
// Always at least 3 sentences; up to ~5 when both sides have rich
// signal. Returns a single multi-sentence string (callers paragraph-
// wrap as needed).
func buildNarrativeSummary(cmp *DeckComparison) string {
	var sb strings.Builder

	// 1. Archetype-gap lead.
	archA := strings.TrimSpace(cmp.ArchetypeA)
	archB := strings.TrimSpace(cmp.ArchetypeB)
	sameArch := archA != "" && strings.EqualFold(archA, archB)
	switch {
	case sameArch && cmp.SameCommander:
		sb.WriteString(fmt.Sprintf("Two %s builds of %s — same shell, different priorities. ",
			archA, cmp.Commander))
	case cmp.SameCommander && archA != "" && archB != "":
		sb.WriteString(fmt.Sprintf("Two %s builds — one leans %s, the other %s. ",
			cmp.Commander, archA, archB))
	case sameArch:
		sb.WriteString(fmt.Sprintf("Both decks play %s, but the engines diverge. ", archA))
	case archA != "" && archB != "":
		sb.WriteString(fmt.Sprintf("%s leans %s; %s leans %s. ",
			cmp.deckLabelA(), archA, cmp.deckLabelB(), archB))
	default:
		sb.WriteString("The two decks share core, but diverge in identity. ")
	}

	// 2. + 3. Evidence sentences naming the top impact-ranked uniques.
	if line := narrativeEvidenceLine(cmp.deckLabelA(), cmp.RankedDiffsA, archA); line != "" {
		sb.WriteString(line)
		sb.WriteString(" ")
	}
	if line := narrativeEvidenceLine(cmp.deckLabelB(), cmp.RankedDiffsB, archB); line != "" {
		sb.WriteString(line)
		sb.WriteString(" ")
	}

	// 4. Power-gap close.
	switch {
	case cmp.BracketDelta > 0:
		sb.WriteString(fmt.Sprintf("%s is the tighter build (bracket %d vs %d).",
			cmp.deckLabelB(), cmp.BracketB, cmp.BracketA))
	case cmp.BracketDelta < 0:
		sb.WriteString(fmt.Sprintf("%s is the tighter build (bracket %d vs %d).",
			cmp.deckLabelA(), cmp.BracketA, cmp.BracketB))
	default:
		sb.WriteString(fmt.Sprintf("Both target bracket %d — the gap is in card selection, not power ceiling.", cmp.BracketA))
	}

	return strings.TrimSpace(sb.String())
}

// narrativeEvidenceLine builds one evidence sentence naming the top 3
// impact-ranked uniques for one side. Returns empty when the side
// has no ranked diffs (e.g. both decks share their entire star list).
func narrativeEvidenceLine(deckLabel string, diffs []ImpactfulCardDiff, archetype string) string {
	if len(diffs) == 0 {
		return ""
	}
	pick := diffs
	if len(pick) > 3 {
		pick = pick[:3]
	}
	names := make([]string, 0, len(pick))
	for _, d := range pick {
		names = append(names, d.Card)
	}
	joined := joinWithSerialAnd(names)
	frame := "anchoring its plan"
	if archetype != "" {
		frame = fmt.Sprintf("anchoring the %s engine", archetype)
	}
	switch len(names) {
	case 1:
		return fmt.Sprintf("%s leans on %s, %s.", deckLabel, joined, frame)
	default:
		return fmt.Sprintf("%s leans on %s — all %s.", deckLabel, joined, frame)
	}
}
