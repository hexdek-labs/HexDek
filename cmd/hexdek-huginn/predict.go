package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/huginn"
)

// ---------------------------------------------------------------------------
// Huginn 2.0 — Pre-game predict CLI (R60).
//
// Subcommand: hexdek-huginn --predict <deck.json>
//
// Loads a deck card list and walks the Huginn learned-interactions
// corpus, building a "possibility-space" report for the deck:
//
//   - Direct pairs:     Tier 3 (CONFIRMED) pair interactions where
//                       BOTH cards are present in the deck. The
//                       deck can already fire these interactions.
//   - Indirect chains:  Tier 3 N-tuples (N=3..5) where the deck has
//                       N-1 cards present and is missing exactly 1.
//                       These are "1 card away" combo lines.
//   - Cross-tier:       Tier 1 / Tier 2 (OBSERVED / RECURRING) pair
//                       interactions present in the deck. Lower-
//                       confidence signal — the engine has seen
//                       these fire but doesn't yet trust them.
//   - Tutor reach:      Indirect chains whose missing card is in
//                       the deck's `tutor_targets` set. The chain
//                       is 1 tutor away from firing.
//
// Two output modes: text (default, human-readable) and json
// (machine-consumable for the Decks-screen panel or BlueFrog
// pipeline integration).
// ---------------------------------------------------------------------------

// PredictDeckInput is the JSON shape for `--predict <path>`. Minimal
// by design — cards is the only required field. Commander is used
// for the report header; tutor_targets unlocks the tutor-reach
// section.
type PredictDeckInput struct {
	DeckName     string   `json:"deck_name,omitempty"`
	Commander    string   `json:"commander,omitempty"`
	Cards        []string `json:"cards"`
	TutorTargets []string `json:"tutor_targets,omitempty"`
}

// PredictReport is the structured output. JSON-serialized via the
// --predict-format json flag; the text renderer walks the same
// struct.
type PredictReport struct {
	DeckName        string `json:"deck_name,omitempty"`
	Commander       string `json:"commander,omitempty"`
	DeckCardCount   int    `json:"deck_card_count"`
	TutorTargetCount int   `json:"tutor_target_count,omitempty"`

	DirectPairs    []DirectPair     `json:"direct_pairs"`
	IndirectChains []IndirectChain  `json:"indirect_chains"`
	CrossTier      []CrossTierMatch `json:"cross_tier"`
	TutorReach     []TutorReachable `json:"tutor_reach"`

	DirectPairCount    int `json:"direct_pair_count"`
	IndirectChainCount int `json:"indirect_chain_count"`
	CrossTierCount     int `json:"cross_tier_count"`
	TutorReachCount    int `json:"tutor_reach_count"`
}

// DirectPair is a Tier 3 confirmed pair fully present in the deck.
type DirectPair struct {
	CardA      string  `json:"card_a"`
	CardB      string  `json:"card_b"`
	Pattern    string  `json:"pattern,omitempty"`
	AvgImpact  float64 `json:"avg_impact"`
	Confidence int     `json:"confidence"`
}

// IndirectChain is a Tier 3 N-tuple with N-1 cards present + 1
// missing. PresentCount == len(AllCards) - 1.
type IndirectChain struct {
	AllCards     []string `json:"all_cards"`
	PresentCards []string `json:"present_cards"`
	MissingCard  string   `json:"missing_card"`
	AvgImpact    float64  `json:"avg_impact"`
	Confidence   int      `json:"confidence"`
}

// CrossTierMatch is a Tier 1/2 pair interaction present in the deck.
// Tier carried explicitly so the caller can sort or filter.
type CrossTierMatch struct {
	CardA            string  `json:"card_a"`
	CardB            string  `json:"card_b"`
	Pattern          string  `json:"pattern,omitempty"`
	Tier             int     `json:"tier"`
	ObservationCount int     `json:"observation_count"`
	AvgImpact        float64 `json:"avg_impact"`
}

// TutorReachable is an IndirectChain whose missing card is in the
// deck's tutor_targets set — the chain is 1 tutor away.
type TutorReachable struct {
	AllCards     []string `json:"all_cards"`
	PresentCards []string `json:"present_cards"`
	MissingCard  string   `json:"missing_card"`
	AvgImpact    float64  `json:"avg_impact"`
	Confidence   int      `json:"confidence"`
}

// Per-section caps so the rendered report stays readable on
// combo-dense decks. Tunable; tested against 3 fixture decks.
const (
	directPairCap    = 30
	indirectChainCap = 20
	crossTierCap     = 20
	tutorReachCap    = 15
)

// LoadDeckInput parses the predict input JSON from disk.
func LoadDeckInput(path string) (*PredictDeckInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var in PredictDeckInput
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(in.Cards) == 0 {
		return nil, fmt.Errorf("%s: cards array empty or missing", path)
	}
	return &in, nil
}

// Predict computes the possibility-space report from a deck input
// and the Huginn data directory. Reads LearnedInteractions (all
// tiers) and LearnedNTuples (all tiers) from `dir`. Safe to call on
// an empty Huginn corpus — returns a report with zero matches
// rather than an error.
func Predict(in *PredictDeckInput, dir string) (*PredictReport, error) {
	if in == nil {
		return nil, fmt.Errorf("predict: nil input")
	}
	interactions, err := huginn.ReadLearnedInteractions(dir)
	if err != nil {
		return nil, fmt.Errorf("predict: read interactions: %w", err)
	}
	ntuples, err := huginn.ReadLearnedNTuples(dir)
	if err != nil {
		return nil, fmt.Errorf("predict: read ntuples: %w", err)
	}

	deckSet := lowerSet(in.Cards)
	tutorSet := lowerSet(in.TutorTargets)

	out := &PredictReport{
		DeckName:         in.DeckName,
		Commander:        in.Commander,
		DeckCardCount:    len(in.Cards),
		TutorTargetCount: len(in.TutorTargets),
	}
	out.DirectPairs, out.CrossTier = scanPairInteractions(interactions, deckSet)
	out.IndirectChains, out.TutorReach = scanNTupleInteractions(ntuples, deckSet, tutorSet)

	// Apply per-section caps after sorting (sortIndirectChains etc.
	// already sort descending by impact/confidence).
	if len(out.DirectPairs) > directPairCap {
		out.DirectPairs = out.DirectPairs[:directPairCap]
	}
	if len(out.IndirectChains) > indirectChainCap {
		out.IndirectChains = out.IndirectChains[:indirectChainCap]
	}
	if len(out.CrossTier) > crossTierCap {
		out.CrossTier = out.CrossTier[:crossTierCap]
	}
	if len(out.TutorReach) > tutorReachCap {
		out.TutorReach = out.TutorReach[:tutorReachCap]
	}

	out.DirectPairCount = len(out.DirectPairs)
	out.IndirectChainCount = len(out.IndirectChains)
	out.CrossTierCount = len(out.CrossTier)
	out.TutorReachCount = len(out.TutorReach)
	return out, nil
}

// scanPairInteractions walks every LearnedInteraction (all tiers)
// and bins matches into DirectPairs (Tier 3, both present) and
// CrossTier (Tier 1/2, both present). ExampleCards entries are
// "CardA + CardB" — same format Huginn's exportTier3ForFreya
// expands. A single Pattern with multiple example pairs surfaces
// each pair as a separate match so the consumer can see every
// concrete card combination, not just the abstract pattern.
func scanPairInteractions(interactions []huginn.LearnedInteraction, deck map[string]bool) ([]DirectPair, []CrossTierMatch) {
	var direct []DirectPair
	var cross []CrossTierMatch
	seen := map[string]bool{}

	for _, li := range interactions {
		for _, example := range li.ExampleCards {
			parts := strings.SplitN(example, " + ", 2)
			if len(parts) != 2 {
				continue
			}
			a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if a == "" || b == "" {
				continue
			}
			aLower, bLower := strings.ToLower(a), strings.ToLower(b)
			if !deck[aLower] || !deck[bLower] {
				continue
			}
			// Dedup across patterns: the same (cardA, cardB) pair can
			// appear in multiple Patterns (a creature ETB+drain shows
			// up in both "etb→drain" and "creature→drain"). The
			// consumer cares about whether the pair fires, not the
			// pattern taxonomy.
			pairKey := canonicalPairKey(aLower, bLower)
			if seen[pairKey] {
				continue
			}
			seen[pairKey] = true

			if li.Tier >= huginn.TierConfirmed {
				direct = append(direct, DirectPair{
					CardA:      a,
					CardB:      b,
					Pattern:    li.Pattern,
					AvgImpact:  li.AvgImpactScore,
					Confidence: li.ObservationCount,
				})
			} else {
				cross = append(cross, CrossTierMatch{
					CardA:            a,
					CardB:            b,
					Pattern:          li.Pattern,
					Tier:             li.Tier,
					ObservationCount: li.ObservationCount,
					AvgImpact:        li.AvgImpactScore,
				})
			}
		}
	}

	sort.SliceStable(direct, func(i, j int) bool {
		if direct[i].Confidence != direct[j].Confidence {
			return direct[i].Confidence > direct[j].Confidence
		}
		if direct[i].AvgImpact != direct[j].AvgImpact {
			return direct[i].AvgImpact > direct[j].AvgImpact
		}
		return direct[i].CardA < direct[j].CardA
	})
	sort.SliceStable(cross, func(i, j int) bool {
		// Tier desc (so Tier 2 leads Tier 1), then observation count.
		if cross[i].Tier != cross[j].Tier {
			return cross[i].Tier > cross[j].Tier
		}
		if cross[i].ObservationCount != cross[j].ObservationCount {
			return cross[i].ObservationCount > cross[j].ObservationCount
		}
		return cross[i].CardA < cross[j].CardA
	})
	return direct, cross
}

// scanNTupleInteractions walks every LearnedNTuple at Tier 3 and
// bins it as an IndirectChain when exactly 1 card is missing from
// the deck. Chains 2+ cards missing are skipped — the gap is too
// wide to call "1 tutor away" without overstating the deck's
// proximity to the combo. The missing-card check ALSO drives
// the TutorReach section: same N-1-present condition, plus the
// missing card must be in `tutorSet`.
func scanNTupleInteractions(ntuples []huginn.LearnedNTuple, deck, tutorSet map[string]bool) ([]IndirectChain, []TutorReachable) {
	var chains []IndirectChain
	var reach []TutorReachable

	for _, nt := range ntuples {
		if nt.Tier < huginn.TierConfirmed {
			continue
		}
		if len(nt.Cards) < 3 {
			continue
		}
		var present []string
		var missing []string
		for _, c := range nt.Cards {
			if deck[strings.ToLower(c)] {
				present = append(present, c)
			} else {
				missing = append(missing, c)
			}
		}
		if len(missing) != 1 {
			continue
		}
		chains = append(chains, IndirectChain{
			AllCards:     append([]string(nil), nt.Cards...),
			PresentCards: present,
			MissingCard:  missing[0],
			AvgImpact:    nt.AvgImpactScore,
			Confidence:   nt.ObservationCount,
		})
		if tutorSet[strings.ToLower(missing[0])] {
			reach = append(reach, TutorReachable{
				AllCards:     append([]string(nil), nt.Cards...),
				PresentCards: present,
				MissingCard:  missing[0],
				AvgImpact:    nt.AvgImpactScore,
				Confidence:   nt.ObservationCount,
			})
		}
	}

	sort.SliceStable(chains, func(i, j int) bool {
		if chains[i].Confidence != chains[j].Confidence {
			return chains[i].Confidence > chains[j].Confidence
		}
		if chains[i].AvgImpact != chains[j].AvgImpact {
			return chains[i].AvgImpact > chains[j].AvgImpact
		}
		return chains[i].MissingCard < chains[j].MissingCard
	})
	sort.SliceStable(reach, func(i, j int) bool {
		if reach[i].Confidence != reach[j].Confidence {
			return reach[i].Confidence > reach[j].Confidence
		}
		return reach[i].MissingCard < reach[j].MissingCard
	})
	return chains, reach
}

// FormatPredictReport renders the report as human-readable text.
// JSON output is handled by EmitPredictReportJSON (callers
// dispatch on --predict-format).
func FormatPredictReport(r *PredictReport) string {
	if r == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("HUGINN -- Pre-Game Predict\n")
	sb.WriteString("============================\n")
	if r.DeckName != "" {
		sb.WriteString(fmt.Sprintf("Deck:      %s\n", r.DeckName))
	}
	if r.Commander != "" {
		sb.WriteString(fmt.Sprintf("Commander: %s\n", r.Commander))
	}
	sb.WriteString(fmt.Sprintf("Cards:     %d", r.DeckCardCount))
	if r.TutorTargetCount > 0 {
		sb.WriteString(fmt.Sprintf("   Tutor targets: %d", r.TutorTargetCount))
	}
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("Possibility-space summary:\n"))
	sb.WriteString(fmt.Sprintf("  Direct pairs (Tier 3 CONFIRMED, both cards present): %d\n", r.DirectPairCount))
	sb.WriteString(fmt.Sprintf("  Indirect chains (Tier 3 N-tuples, 1 card away):      %d\n", r.IndirectChainCount))
	sb.WriteString(fmt.Sprintf("  Cross-tier exposure (Tier 1-2, both cards present):  %d\n", r.CrossTierCount))
	if r.TutorTargetCount > 0 {
		sb.WriteString(fmt.Sprintf("  Tutor-reachable indirect chains:                     %d\n", r.TutorReachCount))
	}
	sb.WriteString("\n")

	if len(r.DirectPairs) > 0 {
		sb.WriteString("[Direct pairs] (Tier 3 CONFIRMED)\n")
		for _, p := range r.DirectPairs {
			patternTag := ""
			if p.Pattern != "" {
				patternTag = "   pattern: " + p.Pattern
			}
			sb.WriteString(fmt.Sprintf("  ◆ %s + %s   (confidence %d, avg impact %.2f)%s\n",
				p.CardA, p.CardB, p.Confidence, p.AvgImpact, patternTag))
		}
		sb.WriteString("\n")
	}

	if len(r.IndirectChains) > 0 {
		sb.WriteString("[Indirect chains] (Tier 3 N-tuples, 1 card away)\n")
		for _, c := range r.IndirectChains {
			sb.WriteString(fmt.Sprintf("  ○ %d/%d cards: %s | missing: %s   (confidence %d, avg impact %.2f)\n",
				len(c.PresentCards), len(c.AllCards),
				strings.Join(c.PresentCards, " + "), c.MissingCard,
				c.Confidence, c.AvgImpact))
		}
		sb.WriteString("\n")
	}

	if len(r.CrossTier) > 0 {
		sb.WriteString("[Cross-tier exposure] (Tier 1-2 OBSERVED / RECURRING)\n")
		for _, m := range r.CrossTier {
			tierLabel := "OBSERVED"
			if m.Tier == huginn.TierRecurring {
				tierLabel = "RECURRING"
			}
			sb.WriteString(fmt.Sprintf("  · [T%d %s] %s + %s   (%d obs, avg impact %.2f)\n",
				m.Tier, tierLabel, m.CardA, m.CardB, m.ObservationCount, m.AvgImpact))
		}
		sb.WriteString("\n")
	}

	if len(r.TutorReach) > 0 {
		sb.WriteString("[Tutor-reachable] (indirect chains whose missing card is in tutor_targets)\n")
		for _, t := range r.TutorReach {
			sb.WriteString(fmt.Sprintf("  → %s | tutor for: %s   (confidence %d, avg impact %.2f)\n",
				strings.Join(t.PresentCards, " + "), t.MissingCard,
				t.Confidence, t.AvgImpact))
		}
	}

	return sb.String()
}

// EmitPredictReportJSON writes the report as JSON to w. Used when
// the user passes --predict-format json. Pretty-printed for human
// inspection; downstream consumers can still json.Decode it
// normally.
func EmitPredictReportJSON(w io.Writer, r *PredictReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// runPredict is the CLI entry point. Loads the deck JSON, builds
// the report, and writes it in the requested format.
func runPredict(deckPath, dir, format string) {
	in, err := LoadDeckInput(deckPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "predict:", err)
		os.Exit(1)
	}
	report, err := Predict(in, dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "predict:", err)
		os.Exit(1)
	}
	if format == "json" {
		if err := EmitPredictReportJSON(os.Stdout, report); err != nil {
			fmt.Fprintln(os.Stderr, "predict json:", err)
			os.Exit(1)
		}
		return
	}
	fmt.Print(FormatPredictReport(report))
}

// lowerSet builds a lowercase membership set from a slice of card
// names — case-insensitive matching against the Huginn corpus
// (whose card names follow Scryfall canonical casing).
func lowerSet(names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		out[strings.ToLower(n)] = true
	}
	return out
}

func canonicalPairKey(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}
