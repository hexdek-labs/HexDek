package huginn

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Recommender — partner & extend queries (HUGINN 2.0 Worker B)
//
// Two query surfaces over the learned-interactions corpus:
//
//   - Partners(dir, query, minTier):  "What does the corpus think pairs
//     with card X?" — scan every Tier ≥ minTier LearnedInteraction and
//     surface every partner that's been observed pairing with X, ranked
//     by tier × cumulative impact.
//
//   - Extend(dir, deck, minTier):  "What card outside this deck would
//     extend its interaction density?" — scan every Tier ≥ minTier
//     pattern where ONE card is in the deck and ONE card is NOT; rank
//     candidates by sum(tier × impact) across all pairings.
//
// Both queries are pure reads over learned_interactions.json. They do
// NOT depend on the Worker A reverse index (dev-4) — when that lands,
// the lookup path can be swapped in via the ReverseIndex interface
// below. Until then, the queries scan the full corpus linearly (O(N)
// over ~few-thousand entries — well under 10ms for realistic corpora).
//
// Pattern→card resolution:
//
//   LearnedInteraction.ExampleCards holds the canonical "CardA + CardB"
//   strings (sorted alphabetically; see huginn.go:271-274). Each entry
//   is a concrete card pair the pattern was observed across; we parse
//   the " + " split to extract individual cards.
// ---------------------------------------------------------------------------

// Worker A's reverse-index loader (dev-4) shipped as the package-level
// function `ReverseIndex(oracleID) []string` in reverse_index.go. The
// recommender's anticipatory interface stub from PR #927 was retired
// to resolve the name collision; Partners / Extend will swap in
// reverse-index lookups when the integration PR lands.

// PartnerHit is a single partner candidate returned by Partners().
type PartnerHit struct {
	Partner   string   `json:"partner"`
	Tier      int      `json:"tier"`       // best (highest) tier across all shared patterns
	Score     float64  `json:"score"`      // sum of tier × avg_impact across shared patterns
	Patterns  []string `json:"patterns"`   // ordered: highest-impact pattern first
	PairCount int      `json:"pair_count"` // number of distinct patterns the pair shares
}

// ExtendHit is a single card-to-add candidate returned by Extend().
type ExtendHit struct {
	Card       string   `json:"card"`
	Score      float64  `json:"score"`        // sum of tier × avg_impact across all pairings with deck cards
	Tier       int      `json:"tier"`         // best tier of any shared pattern
	Pairs      int      `json:"pairs"`        // number of deck cards this would pair with
	WithCards  []string `json:"with_cards"`   // deck cards this card would pair with (capped at 5 for readability)
	Patterns   []string `json:"patterns"`     // top patterns this card participates in with deck cards (capped at 3)
}

// Partners returns every card the corpus has observed pairing with the
// queried card, ranked by sum(tier × avg_impact) across shared patterns.
//
// The query is case-insensitive and accepts the canonical display name as
// it appears in LearnedInteraction.ExampleCards. minTier filters out
// noise (default usage: minTier=TierRecurring which means "at least
// observed across 2+ decks with 3+ observations").
func Partners(dir, query string, minTier int) ([]PartnerHit, error) {
	if query == "" {
		return nil, fmt.Errorf("huginn.Partners: empty query")
	}
	interactions, err := ReadLearnedInteractions(dir)
	if err != nil {
		return nil, fmt.Errorf("huginn.Partners: read learned: %w", err)
	}
	return partnersFromInteractions(interactions, query, minTier), nil
}

// partnersFromInteractions is the pure query path — exposed for tests so
// they don't need to materialize an on-disk corpus.
func partnersFromInteractions(interactions []LearnedInteraction, query string, minTier int) []PartnerHit {
	queryNorm := strings.ToLower(strings.TrimSpace(query))
	if queryNorm == "" {
		return nil
	}

	// partnerName → aggregated state.
	type agg struct {
		bestTier  int
		score     float64
		patterns  map[string]float64 // pattern → cumulative impact contribution
		pairCount int
	}
	byPartner := map[string]*agg{}

	for _, li := range interactions {
		if li.Tier < minTier {
			continue
		}
		for _, pair := range li.ExampleCards {
			cardA, cardB, ok := splitPair(pair)
			if !ok {
				continue
			}
			var partner string
			switch {
			case strings.EqualFold(cardA, queryNorm) || strings.ToLower(cardA) == queryNorm:
				partner = cardB
			case strings.EqualFold(cardB, queryNorm) || strings.ToLower(cardB) == queryNorm:
				partner = cardA
			default:
				continue
			}
			a, ok := byPartner[partner]
			if !ok {
				a = &agg{patterns: map[string]float64{}}
				byPartner[partner] = a
			}
			contribution := float64(li.Tier) * li.AvgImpactScore
			a.score += contribution
			if li.Tier > a.bestTier {
				a.bestTier = li.Tier
			}
			if _, seen := a.patterns[li.Pattern]; !seen {
				a.pairCount++
			}
			a.patterns[li.Pattern] += contribution
		}
	}

	out := make([]PartnerHit, 0, len(byPartner))
	for partner, a := range byPartner {
		// Order partner's patterns by contribution desc for stable display.
		type pat struct {
			name  string
			score float64
		}
		ps := make([]pat, 0, len(a.patterns))
		for p, s := range a.patterns {
			ps = append(ps, pat{p, s})
		}
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].score > ps[j].score })
		names := make([]string, len(ps))
		for i, p := range ps {
			names[i] = p.name
		}
		out = append(out, PartnerHit{
			Partner:   partner,
			Tier:      a.bestTier,
			Score:     a.score,
			Patterns:  names,
			PairCount: a.pairCount,
		})
	}

	// Primary sort: tier desc, score desc, partner name asc (stable tiebreak).
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier > out[j].Tier
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Partner < out[j].Partner
	})
	return out
}

// DeckJSON is the minimal deck shape the --extend subcommand accepts.
// Either Cards (a flat name list) OR Library + Commander (mirroring
// TournamentDeck) is parsed; one of the two must be populated.
type DeckJSON struct {
	Name      string   `json:"name,omitempty"`
	Cards     []string `json:"cards,omitempty"`
	Library   []string `json:"library,omitempty"`
	Commander string   `json:"commander,omitempty"`
}

// LoadDeckJSON parses a deck JSON file from path. Returns a normalized
// name list (Commander + Library OR Cards, deduped, lowercased).
func LoadDeckJSON(path string) (*DeckJSON, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("huginn.LoadDeckJSON: read %s: %w", path, err)
	}
	var d DeckJSON
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, nil, fmt.Errorf("huginn.LoadDeckJSON: parse %s: %w", path, err)
	}
	names := d.normalizedNames()
	if len(names) == 0 {
		return &d, nil, fmt.Errorf("huginn.LoadDeckJSON: deck %s has no cards (need Cards, Library, or Commander populated)", path)
	}
	return &d, names, nil
}

// normalizedNames returns the canonical lowercased deduped card list
// across all the deck's name surfaces.
func (d *DeckJSON) normalizedNames() []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		n := strings.ToLower(strings.TrimSpace(s))
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	if d.Commander != "" {
		add(d.Commander)
	}
	for _, c := range d.Cards {
		add(c)
	}
	for _, c := range d.Library {
		add(c)
	}
	return out
}

// Extend returns cards NOT in the deck that would extend its interaction
// density, ranked by sum(tier × avg_impact) across all pairings with
// deck cards. Each ExtendHit's Pairs field counts the number of distinct
// deck cards the candidate would pair with.
func Extend(dir string, deckCards []string, minTier int) ([]ExtendHit, error) {
	interactions, err := ReadLearnedInteractions(dir)
	if err != nil {
		return nil, fmt.Errorf("huginn.Extend: read learned: %w", err)
	}
	return extendFromInteractions(interactions, deckCards, minTier), nil
}

func extendFromInteractions(interactions []LearnedInteraction, deckCards []string, minTier int) []ExtendHit {
	if len(deckCards) == 0 {
		return nil
	}
	deck := map[string]bool{}
	for _, c := range deckCards {
		deck[strings.ToLower(strings.TrimSpace(c))] = true
	}

	type agg struct {
		bestTier   int
		score      float64
		pairs      map[string]bool        // distinct deck cards this candidate pairs with
		patterns   map[string]float64     // pattern → cumulative contribution
		withCards  []string               // ordered insertion for display (matches first-seen deck card)
	}
	byCard := map[string]*agg{}

	for _, li := range interactions {
		if li.Tier < minTier {
			continue
		}
		contribution := float64(li.Tier) * li.AvgImpactScore
		for _, pair := range li.ExampleCards {
			cardA, cardB, ok := splitPair(pair)
			if !ok {
				continue
			}
			a := strings.ToLower(cardA)
			b := strings.ToLower(cardB)
			aIn := deck[a]
			bIn := deck[b]
			// We want exactly one in-deck card per pair.
			if aIn == bIn {
				continue
			}
			var inDeck, candidate string
			if aIn {
				inDeck, candidate = cardA, cardB
			} else {
				inDeck, candidate = cardB, cardA
			}
			// Reject if the candidate is itself in the deck (lowercase guard
			// in case of casing drift between ExampleCards).
			if deck[strings.ToLower(candidate)] {
				continue
			}
			ag, ok := byCard[candidate]
			if !ok {
				ag = &agg{
					pairs:    map[string]bool{},
					patterns: map[string]float64{},
				}
				byCard[candidate] = ag
			}
			ag.score += contribution
			if li.Tier > ag.bestTier {
				ag.bestTier = li.Tier
			}
			if !ag.pairs[inDeck] {
				ag.pairs[inDeck] = true
				ag.withCards = append(ag.withCards, inDeck)
			}
			ag.patterns[li.Pattern] += contribution
		}
	}

	out := make([]ExtendHit, 0, len(byCard))
	for card, a := range byCard {
		// Top patterns by contribution.
		type pat struct {
			name  string
			score float64
		}
		ps := make([]pat, 0, len(a.patterns))
		for p, s := range a.patterns {
			ps = append(ps, pat{p, s})
		}
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].score > ps[j].score })
		topPatterns := make([]string, 0, 3)
		for i := 0; i < len(ps) && i < 3; i++ {
			topPatterns = append(topPatterns, ps[i].name)
		}
		// Top with-cards capped at 5 for human display (full list available
		// from the score-by-deck-card map for callers that want it).
		with := a.withCards
		if len(with) > 5 {
			with = append([]string{}, with[:5]...)
		}
		out = append(out, ExtendHit{
			Card:      card,
			Score:     a.score,
			Tier:      a.bestTier,
			Pairs:     len(a.pairs),
			WithCards: with,
			Patterns:  topPatterns,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Tier != out[j].Tier {
			return out[i].Tier > out[j].Tier
		}
		if out[i].Pairs != out[j].Pairs {
			return out[i].Pairs > out[j].Pairs
		}
		return out[i].Card < out[j].Card
	})
	return out
}

// splitPair parses an "CardA + CardB" ExampleCards entry. Returns the two
// names trimmed; ok=false when the entry isn't a 2-card pair (e.g. a
// legacy format or an n-tuple bleed-through).
func splitPair(s string) (string, string, bool) {
	idx := strings.Index(s, " + ")
	if idx <= 0 {
		return "", "", false
	}
	a := strings.TrimSpace(s[:idx])
	b := strings.TrimSpace(s[idx+3:])
	if a == "" || b == "" {
		return "", "", false
	}
	return a, b, true
}
