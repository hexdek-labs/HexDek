package main

import "sort"

// ---------------------------------------------------------------------------
// Tier list export — per-archetype card ranking by inclusion rate ×
// win-impact correlation. Answers the deckbuilder question:
//
//   "I'm building a Voltron deck — what cards should I auto-include?"
//
// For each archetype represented in the corpus, ranks every card that
// shows up in ≥ tierListMinDeckCount of that archetype's decks by:
//
//   InclusionRate = decks-in-archetype-containing-card / decks-in-archetype
//
//   WinImpact = mean Bracket of decks containing the card -
//               mean Bracket of decks NOT containing the card
//
// (Bracket is the win-impact proxy here — real win-rate data isn't
// available per deck. Bracket-correlation is the next-best signal:
// cards that show up disproportionately in higher-bracket decks of an
// archetype are the cards that distinguish the optimized end of the
// archetype.)
//
//   TierScore = InclusionRate * (1 + WinImpact)
//
// The (1 + WinImpact) multiplier biases toward cards that are common
// AND associated with higher-bracket builds; WinImpact == 0 (no
// bracket discrimination) falls back to pure inclusion ranking.
// Negative WinImpact cards (associated with LOWER-bracket builds)
// still get represented since 1 + (small negative) > 0, but they sort
// lower than equivalently-included neutral cards.
//
// Cohort + inclusion gates filter noise:
//   - Archetypes with < tierListMinArchetypeDecks decks are skipped
//     entirely (too few samples to make per-card claims).
//   - Cards in < tierListMinDeckCount decks within an archetype are
//     skipped (single-deck inclusions are pet-card noise).
//   - Top tierListTopN per archetype emitted by default.
//
// CLI: `--tier-list-out <path>` (only meaningful with `--all-decks`).
// Output is structured JSON suitable for Decks-screen consumption,
// CommanderBracket-style auto-include lookups, or hat training
// signals (high-inclusion + high-WinImpact cards become priors for
// archetype-pinned MCTS weight profiles).
// ---------------------------------------------------------------------------

const (
	tierListMinArchetypeDecks = 5  // skip archetypes with fewer decks
	tierListMinDeckCount      = 2  // skip cards in fewer decks within an archetype
	tierListTopN              = 50 // cards per archetype in the output
)

// TierListExport is the top-level JSON shape. GeneratedFrom names the
// source corpus for downstream consumers that want to attribute the
// rankings (e.g. "moxfield_300").
type TierListExport struct {
	CorpusSize     int                `json:"corpus_size"`
	ArchetypeCount int                `json:"archetype_count"`
	Archetypes     []ArchetypeTierList `json:"archetypes"`
}

// ArchetypeTierList ranks the top-N cards for a single archetype.
// DeckCount is the cohort size; cards are sorted by TierScore desc.
type ArchetypeTierList struct {
	Archetype  string       `json:"archetype"`
	DeckCount  int          `json:"deck_count"`
	AvgBracket float64      `json:"avg_bracket"`
	TopCards   []TierCard   `json:"top_cards"`
}

// TierCard is one row of the per-archetype ranking. All averages
// computed over the archetype's deck cohort only.
type TierCard struct {
	Name               string  `json:"name"`
	InclusionCount     int     `json:"inclusion_count"`
	InclusionRate      float64 `json:"inclusion_rate"`
	AvgBracketWith     float64 `json:"avg_bracket_with"`
	AvgBracketWithout  float64 `json:"avg_bracket_without"`
	WinImpact          float64 `json:"win_impact"`
	TierScore          float64 `json:"tier_score"`
}

// ComputeTierListExport rolls the corpus into the per-archetype tier
// lists. Empty / nil input returns a zero-value export so JSON callers
// always get a parseable shape.
func ComputeTierListExport(reports []*FreyaReport) *TierListExport {
	out := &TierListExport{}
	if len(reports) == 0 {
		return out
	}
	out.CorpusSize = len(reports)

	// Group reports by primary archetype. Lands without an archetype
	// classification are skipped — they can't be ranked into a cohort.
	byArchetype := map[string][]*FreyaReport{}
	for _, r := range reports {
		if r == nil || r.Archetype == nil {
			continue
		}
		arch := r.Archetype.Primary
		if arch == "" {
			continue
		}
		byArchetype[arch] = append(byArchetype[arch], r)
	}

	// Iterate archetypes in alphabetical order for deterministic JSON.
	archetypeNames := make([]string, 0, len(byArchetype))
	for name := range byArchetype {
		archetypeNames = append(archetypeNames, name)
	}
	sort.Strings(archetypeNames)

	for _, arch := range archetypeNames {
		cohort := byArchetype[arch]
		if len(cohort) < tierListMinArchetypeDecks {
			continue
		}
		list := computeArchetypeTierList(arch, cohort)
		out.Archetypes = append(out.Archetypes, list)
	}
	out.ArchetypeCount = len(out.Archetypes)
	return out
}

// computeArchetypeTierList ranks cards within a single archetype's
// deck cohort. Returns the structured list (deck count + cohort avg
// bracket + sorted TopCards).
func computeArchetypeTierList(arch string, cohort []*FreyaReport) ArchetypeTierList {
	// Inclusion + bracket-with accumulators per card name.
	type cardStats struct {
		inclusionCount int
		bracketSum     int
		bracketSamples int
	}
	cards := map[string]*cardStats{}

	// Corpus-wide bracket stats for the cohort.
	cohortBracketSum := 0
	cohortBracketSamples := 0
	for _, r := range cohort {
		bracket := bracketOfReport(r)
		if bracket > 0 {
			cohortBracketSum += bracket
			cohortBracketSamples++
		}
		// Dedup within a deck — a card listed twice (which shouldn't
		// happen, but defensive) counts once for the inclusion metric.
		seen := map[string]bool{}
		for _, p := range r.Profiles {
			if p.IsLand {
				continue // skip lands — too generic to rank per archetype
			}
			name := p.Name
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			cs, ok := cards[name]
			if !ok {
				cs = &cardStats{}
				cards[name] = cs
			}
			cs.inclusionCount++
			if bracket > 0 {
				cs.bracketSum += bracket
				cs.bracketSamples++
			}
		}
	}

	avgCohortBracket := 0.0
	if cohortBracketSamples > 0 {
		avgCohortBracket = float64(cohortBracketSum) / float64(cohortBracketSamples)
	}

	// Score every card that passes the inclusion-count gate.
	var tier []TierCard
	for name, cs := range cards {
		if cs.inclusionCount < tierListMinDeckCount {
			continue
		}
		inclusionRate := float64(cs.inclusionCount) / float64(len(cohort))
		avgWith := 0.0
		if cs.bracketSamples > 0 {
			avgWith = float64(cs.bracketSum) / float64(cs.bracketSamples)
		}
		// Avg bracket of decks NOT containing the card. Derived by
		// subtraction from the cohort total to avoid a second pass.
		withoutBracketSum := cohortBracketSum - cs.bracketSum
		withoutBracketSamples := cohortBracketSamples - cs.bracketSamples
		avgWithout := 0.0
		if withoutBracketSamples > 0 {
			avgWithout = float64(withoutBracketSum) / float64(withoutBracketSamples)
		}
		winImpact := avgWith - avgWithout
		tierScore := inclusionRate * (1 + winImpact)
		tier = append(tier, TierCard{
			Name:              name,
			InclusionCount:    cs.inclusionCount,
			InclusionRate:     inclusionRate,
			AvgBracketWith:    avgWith,
			AvgBracketWithout: avgWithout,
			WinImpact:         winImpact,
			TierScore:         tierScore,
		})
	}

	// Sort by TierScore desc, tie-break by InclusionCount desc, then
	// by card name asc (deterministic).
	sort.Slice(tier, func(i, j int) bool {
		if tier[i].TierScore != tier[j].TierScore {
			return tier[i].TierScore > tier[j].TierScore
		}
		if tier[i].InclusionCount != tier[j].InclusionCount {
			return tier[i].InclusionCount > tier[j].InclusionCount
		}
		return tier[i].Name < tier[j].Name
	})
	if len(tier) > tierListTopN {
		tier = tier[:tierListTopN]
	}

	return ArchetypeTierList{
		Archetype:  arch,
		DeckCount:  len(cohort),
		AvgBracket: avgCohortBracket,
		TopCards:   tier,
	}
}

// bracketOfReport reads the effective bracket from a report, preferring
// the Profile (which has the post-BuildDeckProfile timing+floor gate
// applied) and falling back to the raw Archetype value. Returns 0 for
// reports with no usable bracket signal.
func bracketOfReport(r *FreyaReport) int {
	if r == nil {
		return 0
	}
	if r.Profile != nil && r.Profile.Bracket > 0 {
		return r.Profile.Bracket
	}
	if r.Profile != nil && r.Profile.MeasuredBracket > 0 {
		return r.Profile.MeasuredBracket
	}
	if r.Archetype != nil && r.Archetype.Bracket > 0 {
		return r.Archetype.Bracket
	}
	if r.Archetype != nil && r.Archetype.MeasuredBracket > 0 {
		return r.Archetype.MeasuredBracket
	}
	return 0
}

