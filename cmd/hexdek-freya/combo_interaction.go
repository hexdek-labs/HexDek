package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Combo interaction matrix — pairwise piece-overlap and fragility analysis
// across a deck's detected combos.
//
// Inputs: TrueInfinites + Determined + GraveyardLoops. These are the
// categorical-win combos (or B3-grade graveyard-recurrable combos) that
// matter for deck-resilience analysis. LandCycleSynergies (fixing tools,
// not wincons) and general Synergies (too noisy — would dilute the
// fragility scoring) are intentionally excluded.
//
// What the matrix surfaces:
//
//   1. PIECE OVERLAP: for every pair of combos (i, j), how many pieces
//      do they share? Diagonal = combo size. Symmetric.
//
//   2. PER-PIECE FRAGILITY: every card that appears in at least one
//      combo, ranked by how many combos depend on it. The top entry is
//      the load-bearing card; if its ComboCount equals the total combo
//      count, the deck has a single point of failure (every combo
//      collapses if that one card is removed).
//
//   3. DECK REDUNDANCY: how many combos survive if any single card is
//      removed? Computed as min over every combo piece C of (combos
//      that don't include C). Value 0 means the deck has at least one
//      card whose removal collapses ALL combos.
//
//   4. PER-COMBO INDEPENDENCE: which combo is most resilient to other-
//      combo removals (lowest cross-overlap sum) and which is most
//      entangled (highest cross-overlap sum). IndependentComboCount
//      counts combos that share zero pieces with any sibling.
//
// Wiring: a single field on FreyaReport (ComboInteraction) populated by
// BuildComboInteractionMatrix at the end of AnalyzeDeck, surfaced under
// `combo_interaction` in JSON and as a short "Combo Interaction" section
// in the text report. Both are skipped when there are < 2 combos.
// ---------------------------------------------------------------------------

// ComboMatrixEntry is the canonical representation of one combo in the
// interaction matrix. Cards is sorted ascending for stable Label /
// deduplication keys across reports.
type ComboMatrixEntry struct {
	Label    string   // "card1 + card2 [+ card3...]" for display
	Cards    []string // canonical sorted piece list
	LoopType string   // true_infinite / determined / synergy
	Class    string   // ComboClass* tag
	Source   string   // "true_infinite" / "determined" / "graveyard_loop"
}

// PieceFragility records how many combos a single card participates in.
// ComboIndices points into the parent matrix's Combos slice. The slice
// returned by BuildComboInteractionMatrix is sorted by ComboCount
// descending, then Card ascending — so the first entry is the most
// load-bearing card in the deck.
type PieceFragility struct {
	Card         string
	ComboCount   int
	ComboIndices []int
}

// ComboInteractionMatrix is the per-deck combo-vs-combo analysis. Nil
// when the deck has fewer than 2 combos to compare (a 1-combo deck
// has nothing to interact with itself, and a 0-combo deck has no
// matrix to build).
type ComboInteractionMatrix struct {
	// Combos is the canonical list of combos analyzed. All matrix /
	// fragility indices refer back to this slice. Sorted by Label so
	// the output is deterministic across runs.
	Combos []ComboMatrixEntry

	// Overlap is the symmetric N×N piece-overlap count. Overlap[i][j]
	// is the number of cards combos i and j share. Diagonal equals
	// each combo's piece count.
	Overlap [][]int

	// PieceFragility ranks every card that appears in >= 1 combo by
	// how many combos depend on it. Sorted ComboCount desc, then card
	// name asc for stable ordering.
	PieceFragility []PieceFragility

	// RedundancyOneCardRemoved is the worst-case number of combos
	// remaining after any single card is removed. Computed as
	// min over cards C of (combos not containing C). A value of 0
	// means at least one card is critical to every combo in the deck.
	RedundancyOneCardRemoved int

	// MostFragileComboIndex is the combo with the highest cross-overlap
	// sum (most entangled — most affected by removals targeting shared
	// pieces). Ties broken by index ascending.
	MostFragileComboIndex int

	// MostIndependentComboIndex is the combo with the lowest cross-
	// overlap sum (most resilient — most likely to survive a sibling-
	// combo's piece being removed). -1 when fewer than 2 combos.
	MostIndependentComboIndex int

	// IndependentComboCount is the number of combos with zero piece
	// overlap to any other combo. These are the combos the deck can
	// fall back on when shared pieces are answered.
	IndependentComboCount int
}

// BuildComboInteractionMatrix computes the interaction matrix from a
// report. Returns nil when the report has fewer than 2 combos.
func BuildComboInteractionMatrix(report *FreyaReport) *ComboInteractionMatrix {
	if report == nil {
		return nil
	}

	var entries []ComboMatrixEntry
	addAll := func(combos []ComboResult, source string) {
		for _, c := range combos {
			if len(c.Cards) == 0 {
				continue
			}
			cards := append([]string(nil), c.Cards...)
			sort.Strings(cards)
			entries = append(entries, ComboMatrixEntry{
				Label:    strings.Join(cards, " + "),
				Cards:    cards,
				LoopType: c.LoopType,
				Class:    c.Class,
				Source:   source,
			})
		}
	}
	addAll(report.TrueInfinites, "true_infinite")
	addAll(report.Determined, "determined")
	addAll(report.GraveyardLoops, "graveyard_loop")

	if len(entries) < 2 {
		return nil
	}

	// Dedup by (Source, Label). Two combos with identical card sets but
	// different sources (e.g. a Determined combo and its graveyard-loop
	// variant) stay distinct — that's the deck-design fact the matrix
	// should preserve. Sort first so dedup picks the lex-min entry per
	// key and downstream output is deterministic.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Source != entries[j].Source {
			return entries[i].Source < entries[j].Source
		}
		return entries[i].Label < entries[j].Label
	})
	deduped := entries[:0]
	seen := map[string]bool{}
	for _, e := range entries {
		key := e.Source + "|" + e.Label
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, e)
	}

	if len(deduped) < 2 {
		return nil
	}

	n := len(deduped)
	overlap := make([][]int, n)
	for i := range overlap {
		overlap[i] = make([]int, n)
	}

	// Build a card → combo-index lookup, then derive the overlap matrix
	// from it. For each card, every pair of combos that share the card
	// gets +1 in the matrix (and the diagonal increments by 1 for each
	// containing combo).
	cardToCombos := map[string][]int{}
	for i, e := range deduped {
		for _, card := range e.Cards {
			cardToCombos[card] = append(cardToCombos[card], i)
		}
	}
	for _, idxs := range cardToCombos {
		for i := 0; i < len(idxs); i++ {
			for j := 0; j < len(idxs); j++ {
				overlap[idxs[i]][idxs[j]]++
			}
		}
	}

	// PieceFragility: every card in cardToCombos with ComboCount >= 1,
	// sorted by ComboCount desc, then card name asc.
	var fragility []PieceFragility
	for card, idxs := range cardToCombos {
		// Each (card, combo) appears exactly once in idxs because every
		// combo's Cards slice was canonical-sorted (no dup within a
		// combo) and addAll appends each card position once.
		sort.Ints(idxs)
		fragility = append(fragility, PieceFragility{
			Card:         card,
			ComboCount:   len(idxs),
			ComboIndices: idxs,
		})
	}
	sort.Slice(fragility, func(i, j int) bool {
		if fragility[i].ComboCount != fragility[j].ComboCount {
			return fragility[i].ComboCount > fragility[j].ComboCount
		}
		return fragility[i].Card < fragility[j].Card
	})

	// RedundancyOneCardRemoved: worst case across all candidate
	// removals. The default is n (no card touched any combo), then we
	// take the min over every combo-touching card.
	minRedundancy := n
	for _, pf := range fragility {
		remaining := n - pf.ComboCount
		if remaining < minRedundancy {
			minRedundancy = remaining
		}
	}

	// Cross-overlap sums per combo (used for fragility / independence
	// ranking, and IndependentComboCount).
	mostFragile := 0
	mostFragileScore := -1
	mostIndep := 0
	mostIndepScore := -1
	independentCount := 0
	for i := 0; i < n; i++ {
		sum := 0
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			sum += overlap[i][j]
		}
		if sum > mostFragileScore {
			mostFragileScore = sum
			mostFragile = i
		}
		if mostIndepScore == -1 || sum < mostIndepScore {
			mostIndepScore = sum
			mostIndep = i
		}
		if sum == 0 {
			independentCount++
		}
	}

	return &ComboInteractionMatrix{
		Combos:                    deduped,
		Overlap:                   overlap,
		PieceFragility:            fragility,
		RedundancyOneCardRemoved:  minRedundancy,
		MostFragileComboIndex:     mostFragile,
		MostIndependentComboIndex: mostIndep,
		IndependentComboCount:     independentCount,
	}
}

// printComboInteraction emits the human-readable combo interaction
// section. No-op when m is nil (deck has < 2 combos to compare).
func printComboInteraction(w io.Writer, m *ComboInteractionMatrix) {
	if m == nil {
		return
	}
	fmt.Fprintf(w, "[CYA] COMBO INTERACTION -- %d combos, redundancy %d/%d after any 1 card removed\n",
		len(m.Combos), m.RedundancyOneCardRemoved, len(m.Combos))

	// Per-combo cross-overlap summary line.
	for i, c := range m.Combos {
		sum := 0
		for j := 0; j < len(m.Combos); j++ {
			if i == j {
				continue
			}
			sum += m.Overlap[i][j]
		}
		tag := ""
		if i == m.MostFragileComboIndex {
			tag = " (most entangled)"
		}
		if i == m.MostIndependentComboIndex && i != m.MostFragileComboIndex {
			tag = " (most independent)"
		}
		fmt.Fprintf(w, "  [%d] %s -- shares %d piece(s) with sibling combos%s\n",
			i, c.Label, sum, tag)
	}

	// Independent combos line.
	if m.IndependentComboCount > 0 {
		fmt.Fprintf(w, "  %d combo(s) share zero pieces with any sibling.\n",
			m.IndependentComboCount)
	}

	// Load-bearing pieces (cap at top 5 for readability — fuller list
	// available in JSON).
	loadBearing := []PieceFragility{}
	for _, p := range m.PieceFragility {
		if p.ComboCount >= 2 {
			loadBearing = append(loadBearing, p)
		}
	}
	if len(loadBearing) > 0 {
		fmt.Fprintf(w, "  Load-bearing pieces (used in 2+ combos):\n")
		max := 5
		if len(loadBearing) < max {
			max = len(loadBearing)
		}
		for _, p := range loadBearing[:max] {
			fmt.Fprintf(w, "    - %s (in %d combos)\n", p.Card, p.ComboCount)
		}
		if len(loadBearing) > max {
			fmt.Fprintf(w, "    ... and %d more\n", len(loadBearing)-max)
		}
	}
	if m.RedundancyOneCardRemoved == 0 {
		fmt.Fprintf(w, "  \xe2\x9a\xa0\xef\xb8\x8f  At least one card is critical to every combo (single point of failure).\n")
	}
	fmt.Fprintf(w, "\n")
}

// comboInteractionToJSON projects the matrix into the JSON shape (defined
// in report.go). Returns nil when the input is nil so the JSON field is
// omitted via the omitempty tag.
func comboInteractionToJSON(m *ComboInteractionMatrix) *jsonComboInteraction {
	if m == nil {
		return nil
	}
	entries := make([]jsonComboMatrixEntry, len(m.Combos))
	for i, e := range m.Combos {
		entries[i] = jsonComboMatrixEntry{
			Label:    e.Label,
			Cards:    e.Cards,
			LoopType: e.LoopType,
			Class:    e.Class,
			Source:   e.Source,
		}
	}
	fragility := make([]jsonPieceFragility, len(m.PieceFragility))
	for i, p := range m.PieceFragility {
		fragility[i] = jsonPieceFragility{
			Card:         p.Card,
			ComboCount:   p.ComboCount,
			ComboIndices: p.ComboIndices,
		}
	}
	return &jsonComboInteraction{
		Combos:                    entries,
		Overlap:                   m.Overlap,
		PieceFragility:            fragility,
		RedundancyOneCardRemoved:  m.RedundancyOneCardRemoved,
		MostFragileComboIndex:     m.MostFragileComboIndex,
		MostIndependentComboIndex: m.MostIndependentComboIndex,
		IndependentComboCount:     m.IndependentComboCount,
	}
}
