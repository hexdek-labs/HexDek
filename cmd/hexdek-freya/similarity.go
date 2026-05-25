package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Deck-vs-deck similarity scoring.
//
// Headline use case: "find decks in the corpus that are similar to mine."
// The composite score combines:
//
//   - Card-set Jaccard (NONLAND ONLY) — the strongest signal. Lands are
//     excluded because basics + standard duals dominate the cardlist and
//     would inflate every cross-archetype pairing.
//   - Archetype affinity — 1.0 same primary, 0.5 primary↔secondary,
//     0.0 disjoint.
//   - Color-identity Jaccard — guards against high card-overlap between
//     two decks that happen to share staples (Sol Ring, Counterspell)
//     but actually run different color combos.
//   - Bracket affinity — same bracket = 1.0, 4 apart = 0.0. A B2 casual
//     and a B5 cEDH deck that share 30 cards are not "similar decks" in
//     a useful sense for the Decks screen.
//
// Composite weighting (sums to 1.0):
//   Overall = 0.55*card + 0.20*archetype + 0.15*color + 0.10*bracket
//
// The Decks screen consumes this to render "decks like yours in the
// corpus" alongside the rank score and a shared-cards sample.
// ---------------------------------------------------------------------------

// DeckFingerprint is the lightweight snapshot extracted from a full
// DeckProfile + FreyaReport. Storing fingerprints means corpus scans
// are O(N²) over small structs instead of full reports — building the
// fingerprint once amortizes the per-card normalization work.
type DeckFingerprint struct {
	Name               string
	Commander          string
	NonlandCards       map[string]bool // lowercased card names (excludes IsLand profiles)
	PrimaryArchetype   string
	SecondaryArchetype string
	Bracket            int
	BracketLabel       string   // display label for the bracket number (e.g. "Optimized")
	ColorIdentity      []string // W/U/B/R/G
	CommanderThemes    []string // for tiebreaker / future weighting
}

// BuildDeckFingerprint extracts the similarity-relevant fields from a
// completed deck analysis. Safe to call with nil dp or report — returns
// a minimally-populated fingerprint so callers don't have to nil-check.
func BuildDeckFingerprint(dp *DeckProfile, report *FreyaReport) *DeckFingerprint {
	fp := &DeckFingerprint{NonlandCards: map[string]bool{}}
	if dp != nil {
		fp.Name = dp.DeckName
		fp.Commander = dp.Commander
		fp.PrimaryArchetype = dp.PrimaryArchetype
		fp.SecondaryArchetype = dp.SecondaryArchetype
		fp.Bracket = dp.Bracket
		fp.BracketLabel = dp.BracketLabel
		fp.ColorIdentity = append([]string(nil), dp.ColorIdentity...)
		fp.CommanderThemes = append([]string(nil), dp.CommanderThemes...)
	}
	if report != nil {
		for _, p := range report.Profiles {
			if p.IsLand {
				continue
			}
			fp.NonlandCards[strings.ToLower(strings.TrimSpace(p.Name))] = true
		}
	}
	return fp
}

// SimilarityScore is the per-pair output. Component fields are exposed
// so the Decks screen can show "60% card overlap, same archetype,
// 1-bracket gap" instead of just an opaque composite number.
type SimilarityScore struct {
	OtherDeck      string   // the name of the deck being compared against
	Overall        float64  // 0.0-1.0 weighted composite
	CardJaccard    float64  // |A∩B| / |A∪B|, nonlands only
	ArchetypeMatch float64  // 1.0 / 0.5 / 0.0 per the affinity ladder
	ColorOverlap   float64  // color identity Jaccard
	BracketAffin   float64  // max(0, 1 - |Δbracket|/4)
	BracketDelta   int      // raw bracket difference (signed not preserved)
	SharedCount    int      // raw |A∩B| nonland count
	SharedSample   []string // up to 8 shared nonland names (alphabetical)
	Verdict        string   // "nearly identical" / "very similar" / "similar" / "loosely related" / "different"
}

// SimilarityScoreOf computes the score between two fingerprints. Safe
// for nil inputs (returns a zero-Overall result so callers can sort
// without nil-checking every comparison).
func SimilarityScoreOf(a, b *DeckFingerprint) *SimilarityScore {
	if a == nil || b == nil {
		return &SimilarityScore{}
	}

	score := &SimilarityScore{OtherDeck: b.Name}

	// Card Jaccard (nonlands only).
	inter, union := 0, 0
	var shared []string
	seenUnion := map[string]bool{}
	for name := range a.NonlandCards {
		seenUnion[name] = true
		if b.NonlandCards[name] {
			inter++
			shared = append(shared, name)
		}
	}
	for name := range b.NonlandCards {
		if !seenUnion[name] {
			seenUnion[name] = true
		}
	}
	union = len(seenUnion)
	if union > 0 {
		score.CardJaccard = float64(inter) / float64(union)
	}
	score.SharedCount = inter
	sort.Strings(shared)
	if len(shared) > 8 {
		shared = shared[:8]
	}
	score.SharedSample = shared

	// Archetype affinity.
	score.ArchetypeMatch = archetypeAffinity(a, b)

	// Color overlap (Jaccard on the W/U/B/R/G set).
	score.ColorOverlap = setJaccard(a.ColorIdentity, b.ColorIdentity)

	// Bracket affinity.
	delta := a.Bracket - b.Bracket
	if delta < 0 {
		delta = -delta
	}
	score.BracketDelta = delta
	score.BracketAffin = 1.0 - float64(delta)/4.0
	if score.BracketAffin < 0 {
		score.BracketAffin = 0
	}

	// Composite weighting — card overlap dominates, archetype is the
	// second strongest, color + bracket are the guardrails.
	score.Overall = 0.55*score.CardJaccard +
		0.20*score.ArchetypeMatch +
		0.15*score.ColorOverlap +
		0.10*score.BracketAffin
	score.Verdict = similarityVerdict(score.Overall)
	return score
}

// archetypeAffinity returns 1.0 when the primary archetypes match,
// 0.5 when one deck's primary equals the other's secondary (a hybrid
// disguise), and 0.0 when the archetype labels don't intersect at all.
// Empty archetypes count as no-match — a deck without classification
// data shouldn't pretend to align with anything.
func archetypeAffinity(a, b *DeckFingerprint) float64 {
	if a.PrimaryArchetype == "" || b.PrimaryArchetype == "" {
		return 0
	}
	if strings.EqualFold(a.PrimaryArchetype, b.PrimaryArchetype) {
		return 1.0
	}
	// Primary ↔ secondary crossover counts as a half-match — the deck
	// looks like that archetype when it pivots.
	if a.SecondaryArchetype != "" &&
		strings.EqualFold(a.SecondaryArchetype, b.PrimaryArchetype) {
		return 0.5
	}
	if b.SecondaryArchetype != "" &&
		strings.EqualFold(a.PrimaryArchetype, b.SecondaryArchetype) {
		return 0.5
	}
	if a.SecondaryArchetype != "" && b.SecondaryArchetype != "" &&
		strings.EqualFold(a.SecondaryArchetype, b.SecondaryArchetype) {
		return 0.5
	}
	return 0
}

// setJaccard returns |A∩B| / |A∪B| over two string sets. Used for
// color identity overlap — values are normalized to lowercase to be
// resilient against W/u/B mixed-case inputs from different sources.
func setJaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		// Two colorless decks share the colorless mana base. Return
		// 1.0 so they aren't penalized for the absence.
		return 1.0
	}
	setA := map[string]bool{}
	for _, s := range a {
		setA[strings.ToLower(s)] = true
	}
	inter, union := 0, 0
	union += len(setA)
	for _, s := range b {
		key := strings.ToLower(s)
		if setA[key] {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// similarityVerdict maps the composite score onto a human-readable
// label. The Decks screen renders this alongside the percentage so
// users get an immediate qualitative read.
func similarityVerdict(overall float64) string {
	switch {
	case overall >= 0.80:
		return "nearly identical"
	case overall >= 0.60:
		return "very similar"
	case overall >= 0.40:
		return "similar"
	case overall >= 0.20:
		return "loosely related"
	default:
		return "different"
	}
}

// FindSimilarDecks scores `target` against every fingerprint in
// `corpus`, returns the top N matches sorted by Overall descending.
// Skips entries with the same Name as the target so a corpus scan
// doesn't return the target itself as its own #1 match. Returns at
// most `limit` results; pass limit ≤0 to return everything.
func FindSimilarDecks(target *DeckFingerprint, corpus []*DeckFingerprint, limit int) []*SimilarityScore {
	if target == nil {
		return nil
	}
	out := make([]*SimilarityScore, 0, len(corpus))
	for _, other := range corpus {
		if other == nil {
			continue
		}
		if other.Name != "" && other.Name == target.Name {
			continue
		}
		out = append(out, SimilarityScoreOf(target, other))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Overall != out[j].Overall {
			return out[i].Overall > out[j].Overall
		}
		// Stable tiebreak by name for deterministic ordering.
		return out[i].OtherDeck < out[j].OtherDeck
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// FormatSimilarityScore renders a single score as a compact one-line
// summary suitable for the text-mode Decks screen.
func FormatSimilarityScore(s *SimilarityScore) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%s — %.0f%% overall (%s): %.0f%% cards (%d shared), arch %.1f, colors %.0f%%, bracket Δ%d",
		s.OtherDeck, s.Overall*100, s.Verdict,
		s.CardJaccard*100, s.SharedCount,
		s.ArchetypeMatch, s.ColorOverlap*100, s.BracketDelta)
}
